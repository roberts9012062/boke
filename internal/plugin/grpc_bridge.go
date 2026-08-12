// internal/plugin/grpc_bridge.go
// 主进程侧 go-plugin gRPC 桥接（M3.3）：coreGRPCPlugin 实现 client 侧，
// 插件进程拉起后 Dispense("core") 得到 pluginClient（三服务客户端）。
// 同时提供钩子适配器：把插件声明的钩子包装为 Dispatcher 的 Handler 注册进 Registry。
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/roberts9012062/boke/pkg/plugin-sdk/proto"
)

// coreGRPCPlugin go-plugin 插件封装（主进程侧：仅 client 能力）。
type coreGRPCPlugin struct {
	plugin.NetRPCUnsupportedPlugin // 仅支持 gRPC 协议
	dataProvider  DataProvider     // 只读数据服务回调（M3.8；nil=不提供数据服务）
}

// GRPCServer 主进程侧不提供服务（报错）。
func (p *coreGRPCPlugin) GRPCServer(_ *plugin.GRPCBroker, _ *grpc.Server) error {
	return fmt.Errorf("主进程不提供插件 gRPC 服务")
}

// GRPCClient 返回插件进程的客户端（三服务 + 数据服务 broker 注册）。
// 数据服务（M3.8）：provider 非 nil 时经 broker.AcceptAndServe 注册 DataService，
// 返回的 brokerID 随 Activate 下发给插件（授权 data.read 的插件 Dial 连接）。
func (p *coreGRPCPlugin) GRPCClient(_ context.Context, broker *plugin.GRPCBroker, conn *grpc.ClientConn) (interface{}, error) {
	pc := &pluginClient{
		info:  proto.NewPluginServiceClient(conn),
		hooks: proto.NewHookServiceClient(conn),
		api:   proto.NewPluginAPIClient(conn),
	}
	if p.dataProvider != nil {
		// 数据服务注册（M3.8）：AcceptAndServe 会阻塞等待插件 Dial（broker 机制——
		// 插件在 Activate 收到 brokerID 后才 Dial），必须在 goroutine 中异步执行，
		// 否则与「GRPCClient → Dispense → Activate → Dial」形成死锁。
		pc.dataBrokerID = int64(broker.NextId())
		go broker.AcceptAndServe(uint32(pc.dataBrokerID), func(opts []grpc.ServerOption) *grpc.Server {
			s := grpc.NewServer(opts...)
			proto.RegisterDataServiceServer(s, &dataServiceServer{provider: p.dataProvider})
			return s
		})
	}
	return pc, nil
}

// pluginClient 插件进程客户端（gRPC 三服务封装 + 流式钩子通道）。
type pluginClient struct {
	info         proto.PluginServiceClient // 生命周期（Info/Activate/Deactivate/SetConfig）
	hooks        proto.HookServiceClient   // 钩子执行
	api          proto.PluginAPIClient     // 自定义 API
	dataBrokerID int64                     // 主进程数据服务 brokerID（M3.8；0=未授权）
	streamMu     sync.Mutex                // 流式通道并发保护（M3.9）
	stream       proto.HookService_StreamClient // 流式钩子通道（nil=不可用，回退 Execute）
	streamOK     bool                      // 通道可用标志（Send 失败置 false 降级）
}

// openStream 建立流式钩子通道（M3.9：异步事件经流推送；失败回退 Execute——兼容旧插件）。
func (c *pluginClient) openStream(ctx context.Context) error {
	stream, err := c.hooks.Stream(ctx)
	if err != nil {
		return err
	}
	c.streamMu.Lock()
	c.stream = stream
	c.streamOK = true
	c.streamMu.Unlock()
	return nil
}

// sendStream 推送异步事件到流式通道；返回是否成功（通道不可用/断连标记降级）。
func (c *pluginClient) sendStream(hookName string, ev Event) bool {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()
	if !c.streamOK || c.stream == nil {
		return false
	}
	payload, err := json.Marshal(ev.Payload)
	if err != nil {
		return false
	}
	if err := c.stream.Send(&proto.StreamEvent{
		Hook: hookName, TraceId: ev.TraceID, ActorId: ev.ActorID, Payload: payload,
	}); err != nil {
		// 断连：标记不可用降级 Execute（进程重启时 Start 重建通道）
		c.streamOK = false
		return false
	}
	return true
}

// ---------- 钩子适配器（进程外插件 → Dispatcher.Handler） ----------

// adapterHandler 生成钩子适配器：主进程事件 JSON 序列化 → gRPC Execute → 响应还原。
// 说明：Payload/Modify 经 JSON 往返（search.query 的 string、结构体均兼容）；
//       gRPC 调用携带 dispatcher 传入的 ctx（已有 2s 超时 + panic 隔离，进程外同样兜底）；
//       M3.9：异步钩子优先走流式通道（sendStream），通道不可用回退 Execute。
func (c *pluginClient) adapterHandler(hookName string) Handler {
	return func(ctx context.Context, ev Event) (Result, error) {
		if !IsSyncHook(hookName) && c.sendStream(hookName, ev) {
			return Result{OK: true}, nil // 流式推送成功：异步语义，立即返回
		}
		payload, err := json.Marshal(ev.Payload)
		if err != nil {
			return Result{OK: true}, fmt.Errorf("插件钩子载荷序列化失败：%w", err)
		}
		resp, err := c.hooks.Execute(ctx, &proto.HookRequest{
			Hook: hookName, TraceId: ev.TraceID, ActorId: ev.ActorID, Payload: payload,
		})
		if err != nil {
			// 进程外调用失败（进程崩溃/超时）：故障隔离，跳过该插件
			return Result{OK: true}, fmt.Errorf("插件钩子调用失败（%s）：%w", hookName, err)
		}
		if resp.Error != "" {
			// 插件内部错误（如未订阅）：记录但不阻断核心
			return Result{OK: true}, fmt.Errorf("插件钩子内部错误（%s）：%s", hookName, resp.Error)
		}
		// 改写结果还原（search.query 返回 string；JSON 反序列化回 any）
		var modify any
		if len(resp.Modify) > 0 {
			if err := json.Unmarshal(resp.Modify, &modify); err != nil {
				return Result{OK: true}, fmt.Errorf("插件钩子改写结果解析失败（%s）：%w", hookName, err)
			}
		}
		return Result{OK: resp.Ok, Reason: resp.Reason, Modify: modify}, nil
	}
}

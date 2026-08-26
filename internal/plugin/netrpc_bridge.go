// internal/plugin/netrpc_bridge.go
// 宿主侧插件客户端封装：childProc（进程桥）→ pluginClient（Core net/rpc 调用 +
// 流式钩子推送），并提供钩子适配器——把插件声明的钩子包装为 Dispatcher 的
// Handler 注册进 Registry。
package plugin

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/rpc"
	"sync"

	"github.com/roberts9012062/boke/pkg/plugin-sdk/contract"
	"github.com/roberts9012062/boke/pkg/plugin-sdk/process"
)

// pluginClient 插件进程客户端（Core net/rpc 服务封装 + 流式钩子通道）。
type pluginClient struct {
	rpc       *rpc.Client   // Core 服务客户端（生命周期/钩子/自定义 API）
	streamMu  sync.Mutex    // 流式通道并发保护
	streamEnc *gob.Encoder  // 流编码器（启动时即就绪；断连后置 nil 降级）
}

// newPluginClient 包装子进程为插件客户端（握手完成即流通道就绪）。
func newPluginClient(proc *childProc) *pluginClient {
	return &pluginClient{
		rpc:       proc.CoreClient(),
		streamEnc: gob.NewEncoder(proc.StreamConn()),
	}
}

// callCore 执行一次 Core 服务调用（带超时：net/rpc 无 ctx，Go 异步 + select 兜底；
// 超时后挂起的调用随连接关闭（Kill/断连）自然回收）。
func (c *pluginClient) callCore(ctx context.Context, method string, args any, reply any) error {
	call := c.rpc.Go(process.CoreServiceName+"."+method, args, reply, make(chan *rpc.Call, 1))
	select {
	case done := <-call.Done:
		return done.Error
	case <-ctx.Done():
		return ctx.Err()
	}
}

// closeStream 关闭流式通道（停用/断连清理；幂等）。
func (c *pluginClient) closeStream() {
	c.streamMu.Lock()
	c.streamEnc = nil
	c.streamMu.Unlock()
}

// sendStream 推送异步事件到流式通道；返回是否成功（通道不可用/断连返回 false 降级 ExecuteHook）。
func (c *pluginClient) sendStream(hookName string, ev Event) bool {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()
	if c.streamEnc == nil {
		return false
	}
	payload, err := json.Marshal(ev.Payload)
	if err != nil {
		return false
	}
	if err := c.streamEnc.Encode(&contract.StreamEvent{
		Hook: hookName, TraceID: ev.TraceID, ActorID: ev.ActorID, Payload: payload,
	}); err != nil {
		// 断连：标记不可用降级 ExecuteHook（进程重启时 Start 重建通道）
		c.streamEnc = nil
		return false
	}
	return true
}

// ---------- 钩子适配器（进程外插件 → Dispatcher.Handler） ----------

// adapterHandler 生成钩子适配器：主进程事件 JSON 序列化 → Core.ExecuteHook → 响应还原。
// 说明：Payload/Modify 经 JSON 往返（search.query 的 string、结构体均兼容）；
//       超时由 dispatcher 传入的 ctx 控制（dispatchOne 已有 2s 超时 + panic 隔离）；
//       异步钩子优先走流式通道（sendStream），通道不可用回退 ExecuteHook。
func (c *pluginClient) adapterHandler(hookName string) Handler {
	return func(ctx context.Context, ev Event) (Result, error) {
		if !IsSyncHook(hookName) && c.sendStream(hookName, ev) {
			return Result{OK: true}, nil // 流式推送成功：异步语义，立即返回
		}
		payload, err := json.Marshal(ev.Payload)
		if err != nil {
			return Result{OK: true}, fmt.Errorf("插件钩子载荷序列化失败：%w", err)
		}
		var resp contract.HookResponse
		if err := c.callCore(ctx, "ExecuteHook", &contract.HookRequest{
			Hook: hookName, TraceID: ev.TraceID, ActorID: ev.ActorID, Payload: payload,
		}, &resp); err != nil {
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
		return Result{OK: resp.OK, Reason: resp.Reason, Modify: modify}, nil
	}
}

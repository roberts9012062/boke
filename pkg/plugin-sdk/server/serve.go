// pkg/plugin-sdk/server/serve.go
// 插件侧入口：插件作者 main 中调用 server.Serve(&Plugin{})，
// 内部完成进程桥握手（监听回环端口 + stdout 握手行 + token 鉴权）、
// net/rpc Core 服务注册、流式钩子接收、stdin 关闭优雅退出。
// 契约类型：pkg/plugin-sdk/contract（gob 序列化，与主进程 internal/plugin 共用）。
// 传输：pkg/plugin-sdk/process 自研进程桥（标准库 TCP + net/rpc + gob）——
// 无 grpc/protobuf/hashicorp 依赖，插件二进制体积为 Go runtime 基线（~3MB）。
package server

import (
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"os"

	"github.com/roberts9012062/boke/pkg/plugin-sdk"
	"github.com/roberts9012062/boke/pkg/plugin-sdk/contract"
	"github.com/roberts9012062/boke/pkg/plugin-sdk/process"
)

// coreServer 插件核心服务（生命周期 + 钩子 + 自定义 API 三服务合一）。
type coreServer struct {
	impl   sdk.Plugin          // 插件业务实现
	hooks  map[string]sdk.Hook // 已声明钩子（按名称索引）
	apiMux *sdk.APIMux         // 自定义 API 路由表（可空）
	token  string              // 连接凭据（数据服务回连宿主时鉴权）
}

// Serve 插件进程入口（阻塞运行；宿主关闭 stdin 或服务异常时退出）。
func Serve(impl sdk.Plugin) {
	// 防误启动：非宿主拉起（无握手环境变量）直接退出
	if os.Getenv(process.EnvCookie) != process.CookieValue {
		fmt.Fprintln(os.Stderr, "[plugin] 缺少宿主握手环境变量——插件进程需由主站拉起，直接运行无意义")
		os.Exit(1)
	}
	token := os.Getenv(process.EnvToken)

	listener, err := process.NewListener()
	if err != nil {
		fmt.Fprintln(os.Stderr, "[plugin] 监听失败：", err)
		os.Exit(1)
	}

	// 自定义 API（可选接口：插件实现 RegisterAPI 则挂载）
	var apiMux *sdk.APIMux
	if p, ok := impl.(sdk.APIProvider); ok {
		apiMux = sdk.NewAPIMux()
		p.RegisterAPI(apiMux)
	}
	core := &coreServer{impl: impl, hooks: collectHooks(impl), apiMux: apiMux, token: token}

	// stdout 握手行（宿主阻塞等待此行后建立通道连接）
	fmt.Println(process.BuildHandshakeLine(listener.Addr().String()))
	_ = os.Stdout.Sync()

	// 服务循环（异常才返回；正常退出由 stdin 关闭触发）
	serveDone := make(chan error, 1)
	go func() { serveDone <- acceptLoop(listener, token, core) }()

	// stdin EOF = 宿主退出信号（优雅停用 Deactivate 已先行 RPC 调用）
	stdinDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		close(stdinDone)
	}()

	select {
	case <-stdinDone:
	case err := <-serveDone:
		fmt.Fprintln(os.Stderr, "[plugin] 服务循环退出：", err)
	}
	_ = listener.Close()
}

// acceptLoop 连接接受循环：每条连接首行鉴权后按通道分发（core → net/rpc 服务；
// stream → 异步钩子接收循环）。
func acceptLoop(listener net.Listener, token string, core *coreServer) error {
	rpcServer := rpc.NewServer()
	if err := rpcServer.RegisterName(process.CoreServiceName, core); err != nil {
		return fmt.Errorf("Core 服务注册失败：%w", err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err // listener 关闭/损坏：进程退出（宿主重启重建）
		}
		go func(c net.Conn) {
			defer func() { _ = c.Close() }()
			channel, reader, err := process.ReadChannelHeader(c, token)
			if err != nil {
				fmt.Fprintln(os.Stderr, "[plugin] 连接鉴权失败：", err)
				return
			}
			switch channel {
			case process.ChannelCore:
				rpcServer.ServeConn(&process.BufferedConn{Reader: reader, Conn: c})
			case process.ChannelStream:
				core.recvStream(reader)
			}
		}(conn)
	}
}

// collectHooks 汇总插件声明的钩子（按名称建索引，供 ExecuteHook 分发）。
func collectHooks(impl sdk.Plugin) map[string]sdk.Hook {
	hooks := make(map[string]sdk.Hook, len(impl.Hooks()))
	for _, h := range impl.Hooks() {
		hooks[h.Name] = h
	}
	return hooks
}

// ---------- Core net/rpc 服务（契约 contract；方法名即宿主调用路径 "Core.XXX"） ----------

// Info 返回插件信息（声明钩子/设置项/能力，主进程校验与安装清单一致）。
func (s *coreServer) Info(_ contract.Empty, reply *contract.PluginInfo) error {
	info := s.impl.Info()
	hooks := make([]string, 0, len(s.hooks))
	for name := range s.hooks {
		hooks = append(hooks, name)
	}
	settings := make([]contract.SettingField, 0, len(info.Settings))
	for _, f := range info.Settings {
		settings = append(settings, contract.SettingField{
			Key: f.Key, Label: f.Label, Type: f.Type,
			Default: f.Default, Options: f.Options,
		})
	}
	*reply = contract.PluginInfo{
		ID: info.ID, Name: info.Name, Version: info.Version,
		Author: info.Author, Description: info.Description,
		Hooks: hooks, Settings: settings, Capabilities: info.Capabilities,
	}
	return nil
}

// Activate 启用回调（许可证 + 数据服务地址下发；更新许可、建数据回连、初始化资源）。
func (s *coreServer) Activate(args contract.ActivateRequest, reply *contract.Status) error {
	// 更新插件许可（主进程唯一数据源；插件只读，降级/宽限期全由主站处理）
	sdk.SetLicense(sdk.LicenseInfo{
		Edition: args.License.Edition, Features: args.License.Features,
		ExpiresAt: args.License.ExpiresAt, Degraded: args.License.Degraded,
	})
	// 数据服务回连（data.read 授权时宿主下发其监听地址与凭据；Dial 失败按无数据服务降级）
	if args.DataAddr != "" && args.DataToken != "" {
		if conn, err := net.Dial("tcp", args.DataAddr); err == nil {
			if err := process.WriteChannelHeader(conn, args.DataToken, process.ChannelData); err == nil {
				sdk.SetDataClient(&sdkDataBridge{client: rpc.NewClient(conn)})
			} else {
				_ = conn.Close()
			}
		}
	}
	if err := s.impl.OnActivate(context.Background()); err != nil {
		*reply = contract.Status{OK: false, Error: err.Error()}
		return nil
	}
	*reply = contract.Status{OK: true}
	return nil
}

// recvStream 流式通道接收循环（宿主持续 gob 编码 StreamEvent；断连退出——
// 宿主重启插件进程时重建通道）。入参为通道头之后的预读缓冲（防丢首包）。
func (s *coreServer) recvStream(reader io.Reader) {
	decoder := gob.NewDecoder(reader)
	for {
		var ev contract.StreamEvent
		if err := decoder.Decode(&ev); err != nil {
			return // 对端关闭/断连：由进程生命周期管理（宿主重建）
		}
		hook, ok := s.hooks[ev.Hook]
		if !ok {
			continue // 未订阅：静默跳过
		}
		go func() {
			defer func() { _ = recover() }() // handler panic 不拖垮流
			_, _ = hook.Handler(context.Background(), sdk.Event{
				TraceID: ev.TraceID, ActorID: ev.ActorID, Payload: ev.Payload,
			})
		}()
	}
}

// Deactivate 停用回调（保存状态/释放资源）。
func (s *coreServer) Deactivate(_ contract.Empty, reply *contract.Status) error {
	if err := s.impl.OnDeactivate(context.Background()); err != nil {
		*reply = contract.Status{OK: false, Error: err.Error()}
		return nil
	}
	*reply = contract.Status{OK: true}
	return nil
}

// SetConfig 配置下发回调（宿主：启动激活后 + 保存配置时推送；插件更新内存）。
func (s *coreServer) SetConfig(args contract.ConfigInfo, reply *contract.Status) error {
	sdk.SetConfig(args.Values)
	*reply = contract.Status{OK: true}
	return nil
}

// ExecuteHook 执行同步钩子（未订阅返回放行；插件内部错误记录不阻断核心）。
func (s *coreServer) ExecuteHook(args contract.HookRequest, reply *contract.HookResponse) (err error) {
	hook, ok := s.hooks[args.Hook]
	if !ok {
		*reply = contract.HookResponse{OK: true, Error: "插件未订阅钩子 " + args.Hook}
		return nil
	}
	// panic 恢复：插件 handler 崩溃不拖垮 net/rpc 服务（宿主侧也会检测进程存活）
	defer func() {
		if r := recover(); r != nil {
			*reply = contract.HookResponse{OK: true, Error: "插件钩子 panic"}
		}
	}()
	res, herr := hook.Handler(context.Background(), sdk.Event{TraceID: args.TraceID, ActorID: args.ActorID, Payload: args.Payload})
	if herr != nil {
		*reply = contract.HookResponse{OK: true, Error: herr.Error()}
		return nil
	}
	*reply = contract.HookResponse{OK: res.OK, Reason: res.Reason, Modify: res.Modify}
	return nil
}

// CallAPI 分发自定义 API 调用（404 表示未注册路由）。
// 调用者身份随契约字段内联传输，注入 handler ctx（插件侧经 sdk.CallerID/
// CallerRole/TrustedCaller 查询，做 per-endpoint 鉴权）。
// panic 恢复：插件 API 单次 panic 不拖垮插件进程（转 500）。
func (s *coreServer) CallAPI(args contract.APICall, reply *contract.APICallResult) (err error) {
	defer func() {
		if r := recover(); r != nil {
			*reply = contract.APICallResult{Status: 500, Error: fmt.Sprintf("插件 API panic：%v", r)}
		}
	}()
	if s.apiMux == nil {
		*reply = contract.APICallResult{Status: 404, Body: []byte(`{"error":"not_found"}`)}
		return nil
	}
	handler := s.apiMux.Find(args.Method, args.Path)
	if handler == nil {
		*reply = contract.APICallResult{Status: 404, Body: []byte(`{"error":"not_found"}`)}
		return nil
	}
	callerCtx := sdk.WithCallerIdentity(context.Background(), sdk.CallerIdentity{
		UserID: args.CallerID, Role: args.CallerRole, System: args.CallerSystem,
	})
	status, body, herr := handler(callerCtx, args.Method, args.Path, args.Body)
	if herr != nil {
		*reply = contract.APICallResult{Status: 500, Error: herr.Error()}
		return nil
	}
	*reply = contract.APICallResult{Status: int32(status), Body: body}
	return nil
}

// ---------- 数据服务桥（回连宿主的 net/rpc 通道 → sdk.DataService） ----------

// sdkDataBridge net/rpc 客户端 → sdk.DataService 适配。
type sdkDataBridge struct {
	client *rpc.Client // 宿主数据服务连接（Activate 时回连建立）
}

// dataCall 执行一次数据服务调用（统一服务名前缀）。
func dataCall[T any](b *sdkDataBridge, method string, args any, reply *T) error {
	return b.client.Call(process.DataServiceName+"."+method, args, reply)
}

// GetUser 查询用户脱敏信息。
func (b *sdkDataBridge) GetUser(_ context.Context, userID int64) (*sdk.DataUser, error) {
	var u contract.UserInfo
	if err := dataCall(b, "GetUser", contract.UserRequest{UserID: userID}, &u); err != nil {
		return nil, err
	}
	return &sdk.DataUser{ID: u.ID, Nickname: u.Nickname, AvatarURL: u.AvatarURL, Role: u.Role, Bio: u.Bio}, nil
}

// GetPost 查询帖子脱敏信息。
func (b *sdkDataBridge) GetPost(_ context.Context, postID int64) (*sdk.DataPost, error) {
	var p contract.PostInfo
	if err := dataCall(b, "GetPost", contract.PostRequest{PostID: postID}, &p); err != nil {
		return nil, err
	}
	return &sdk.DataPost{ID: p.ID, Title: p.Title, Status: p.Status, AuthorID: p.AuthorID, AuthorName: p.AuthorName}, nil
}

// GetSettings 查询站点公开设置（白名单键）。
func (b *sdkDataBridge) GetSettings(_ context.Context) (map[string]string, error) {
	var snapshot contract.SettingsSnapshot
	if err := dataCall(b, "GetSettings", contract.Empty{}, &snapshot); err != nil {
		return nil, err
	}
	return snapshot.Values, nil
}

// GetAIModels 查询可用 AI 模型（脱敏；空=未配置 AI——面板提示跳转配置）。
func (b *sdkDataBridge) GetAIModels(_ context.Context) ([]sdk.DataAIModel, error) {
	var list contract.AIModelList
	if err := dataCall(b, "GetAIModels", contract.Empty{}, &list); err != nil {
		return nil, err
	}
	models := make([]sdk.DataAIModel, 0, len(list.Models))
	for _, m := range list.Models {
		models = append(models, sdk.DataAIModel{Name: m.Name, Models: m.Models})
	}
	return models, nil
}

// GenerateAI 调用宿主 AI 生成文本（按模型路由供应商）。
func (b *sdkDataBridge) GenerateAI(_ context.Context, model string, prompt string, content string) (string, error) {
	var result contract.GenerateResult
	if err := dataCall(b, "GenerateAI", contract.GenerateRequest{Model: model, Prompt: prompt, Content: content}, &result); err != nil {
		return "", err
	}
	return result.Text, nil
}

// GetOpenAPIKeys 查询开放接口 API Key 清单（含明文 Key；浏览器插件联动远传验证用）。
func (b *sdkDataBridge) GetOpenAPIKeys(_ context.Context) ([]sdk.DataOpenAPIKey, error) {
	var list contract.OpenAPIKeyList
	if err := dataCall(b, "GetOpenAPIKeys", contract.Empty{}, &list); err != nil {
		return nil, err
	}
	keys := make([]sdk.DataOpenAPIKey, 0, len(list.Keys))
	for _, k := range list.Keys {
		keys = append(keys, sdk.DataOpenAPIKey{
			ID: k.ID, Name: k.Name, Key: k.Key, Endpoints: k.Endpoints,
			ExpiresAt: k.ExpiresAt, LastUsedAt: k.LastUsedAt, CreatedAt: k.CreatedAt,
		})
	}
	return keys, nil
}

// pkg/plugin-sdk/server/serve.go
// 插件侧入口（M3.3）：插件作者 main 中调用 server.Serve(&Plugin{})，
// 内部完成 go-plugin 握手（MagicCookie 校验）、gRPC 三服务注册、优雅退出。
// 契约 proto：pkg/plugin-sdk/proto/plugin.proto（与主进程 internal/plugin 共用）。
package server

import (
	"context"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/roberts9012062/boke/pkg/plugin-sdk"
	"github.com/roberts9012062/boke/pkg/plugin-sdk/proto"
)

// Handshake 插件握手配置（主进程 internal/plugin 引用同一份，修改需两侧同步）。
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  3,                       // 协议版本（升级不兼容时协商）
	MagicCookieKey:   "YUEYAN_PLUGIN_COOKIE",  // 防误启动（校验子进程环境变量）
	MagicCookieValue: "yueyan-blog-plugin-v1", // 主进程启动子进程时注入
}

// coreGRPCPlugin go-plugin gRPC 插件封装（插件侧：注册 gRPC 服务；client 侧不支持）。
type coreGRPCPlugin struct {
	plugin.NetRPCUnsupportedPlugin // 仅支持 gRPC 协议（net/rpc 误用即报错）
	impl   sdk.Plugin              // 插件业务实现
	apiMux *sdk.APIMux             // 自定义 API 路由表（实现 APIProvider 时非空）
	logger hclog.Logger
}

// GRPCServer 注册三个契约服务（生命周期/钩子/自定义 API）。
func (p *coreGRPCPlugin) GRPCServer(_ *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterPluginServiceServer(s, &pluginServiceServer{impl: p.impl, hooks: collectHooks(p.impl)})
	proto.RegisterHookServiceServer(s, &hookServiceServer{hooks: collectHooks(p.impl)})
	proto.RegisterPluginAPIServer(s, &apiServiceServer{mux: p.apiMux})
	return nil
}

// GRPCClient 插件侧不消费主进程服务（返回 nil；握手协议要求实现）。
func (p *coreGRPCPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, _ *grpc.ClientConn) (interface{}, error) {
	return nil, nil
}

// Serve 插件进程入口（阻塞运行；握手失败/子进程被杀时退出）。
func Serve(impl sdk.Plugin) {
	// 插件日志走 stderr（主进程重定向到 logs/plugins/{id}.log）
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "plugin-" + impl.Info().ID,
		Level:  hclog.Warn,
		Output: os.Stderr,
	})

	// 自定义 API（可选接口：插件实现 RegisterAPI 则挂载）
	var apiMux *sdk.APIMux
	if p, ok := impl.(sdk.APIProvider); ok {
		apiMux = sdk.NewAPIMux()
		p.RegisterAPI(apiMux)
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         map[string]plugin.Plugin{"core": &coreGRPCPlugin{impl: impl, apiMux: apiMux, logger: logger}},
		GRPCServer:      plugin.DefaultGRPCServer,
		Logger:          logger,
	})
}

// ---------- gRPC 服务实现（契约 plugin.proto） ----------

// collectHooks 汇总插件声明的钩子（按名称建索引，供 Execute 分发）。
func collectHooks(impl sdk.Plugin) map[string]sdk.Hook {
	hooks := make(map[string]sdk.Hook, len(impl.Hooks()))
	for _, h := range impl.Hooks() {
		hooks[h.Name] = h
	}
	return hooks
}

// pluginServiceServer PluginService 实现（生命周期）。
type pluginServiceServer struct {
	proto.UnimplementedPluginServiceServer
	impl  sdk.Plugin
	hooks map[string]sdk.Hook
}

// Info 返回插件信息。
func (s *pluginServiceServer) Info(context.Context, *proto.Empty) (*proto.PluginInfo, error) {
	info := s.impl.Info()
	hooks := make([]string, 0, len(s.hooks))
	for name := range s.hooks {
		hooks = append(hooks, name)
	}
	// 设置项声明（schema 驱动设置页；插件作者在 Info() 中填写）
	settings := make([]*proto.SettingField, 0, len(info.Settings))
	for _, f := range info.Settings {
		settings = append(settings, &proto.SettingField{
			Key: f.Key, Label: f.Label, Type: f.Type,
			Default: f.Default, Options: f.Options,
		})
	}
	return &proto.PluginInfo{
		Id: info.ID, Name: info.Name, Version: info.Version,
		Author: info.Author, Description: info.Description,
		Hooks: hooks, Settings: settings,
	}, nil
}

// Activate 启用回调（携带主进程下发的许可证信息；插件侧更新许可并初始化资源）。
func (s *pluginServiceServer) Activate(ctx context.Context, req *proto.LicenseInfo) (*proto.Status, error) {
	// 更新插件许可（主进程唯一数据源；插件只读，demo 降级/宽限期全由主站处理）
	sdk.SetLicense(sdk.LicenseInfo{
		Edition: req.Edition, Features: req.Features,
		ExpiresAt: req.ExpiresAt, Degraded: req.Degraded,
	})
	if err := s.impl.OnActivate(ctx); err != nil {
		return &proto.Status{Ok: false, Error: err.Error()}, nil
	}
	return &proto.Status{Ok: true}, nil
}

// Deactivate 停用回调（保存状态/释放资源）。
func (s *pluginServiceServer) Deactivate(ctx context.Context, _ *proto.Empty) (*proto.Status, error) {
	if err := s.impl.OnDeactivate(ctx); err != nil {
		return &proto.Status{Ok: false, Error: err.Error()}, nil
	}
	return &proto.Status{Ok: true}, nil
}

// SetConfig 配置下发回调（主进程：启动激活后 + 保存配置时推送；插件更新内存供 handler 读取）。
func (s *pluginServiceServer) SetConfig(_ context.Context, req *proto.ConfigInfo) (*proto.Status, error) {
	sdk.SetConfig(req.GetValues())
	return &proto.Status{Ok: true}, nil
}

// hookServiceServer HookService 实现（同步钩子执行）。
type hookServiceServer struct {
	proto.UnimplementedHookServiceServer
	hooks map[string]sdk.Hook
}

// Execute 执行钩子（未订阅返回放行；插件内部错误记录不阻断核心）。
func (s *hookServiceServer) Execute(ctx context.Context, req *proto.HookRequest) (resp *proto.HookResponse, err error) {
	hook, ok := s.hooks[req.Hook]
	if !ok {
		return &proto.HookResponse{Ok: true, Error: "插件未订阅钩子 " + req.Hook}, nil
	}
	// panic 恢复：插件 handler 崩溃不拖垮 gRPC 服务（主进程侧也会检测进程存活）
	defer func() {
		if r := recover(); r != nil {
			resp = &proto.HookResponse{Ok: true, Error: "插件钩子 panic"}
		}
	}()
	res, err := hook.Handler(ctx, sdk.Event{TraceID: req.TraceId, ActorID: req.ActorId, Payload: req.Payload})
	if err != nil {
		return &proto.HookResponse{Ok: true, Error: err.Error()}, nil
	}
	return &proto.HookResponse{Ok: res.OK, Reason: res.Reason, Modify: res.Modify}, nil
}

// apiServiceServer PluginAPI 实现（自定义 API 分发，精确匹配 method+path）。
type apiServiceServer struct {
	proto.UnimplementedPluginAPIServer
	mux *sdk.APIMux
}

// Call 分发自定义 API 调用（404 表示未注册路由）。
func (s *apiServiceServer) Call(ctx context.Context, req *proto.APICall) (*proto.APICallResult, error) {
	if s.mux == nil {
		return &proto.APICallResult{Status: 404, Body: []byte(`{"error":"not_found"}`)}, nil
	}
	handler := s.mux.Find(req.Method, req.Path)
	if handler == nil {
		return &proto.APICallResult{Status: 404, Body: []byte(`{"error":"not_found"}`)}, nil
	}
	status, body, err := handler(ctx, req.Method, req.Path, req.Body)
	if err != nil {
		return &proto.APICallResult{Status: 500, Error: err.Error()}, nil
	}
	return &proto.APICallResult{Status: int32(status), Body: body}, nil
}

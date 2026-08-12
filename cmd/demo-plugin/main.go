// cmd/demo-plugin/main.go
// 演示插件（M3.3 进程外验证）：订阅 post.before_publish（同步拦截）、
// post.after_publish（异步通知）、search.query（同步改写观察），暴露 GET /ping 自定义 API。
// 编译产物：data/plugins/demo-plugin/plugin(.exe)（./scripts/build-demo-plugin.sh）。
//
// 验证入口（冒烟）：
//   - 发帖标题含 [demo] → 400 拦截（同步钩子链路）
//   - 正常发帖 → 插件日志记录 after_publish（异步链路）
//   - 搜索触发 → 插件日志记录 search.query（同步链路）
//   - GET /api/v1/plugins/demo-plugin/ping → {"pong":true}（API 代理链路）
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/roberts9012062/boke/pkg/plugin-sdk"
	"github.com/roberts9012062/boke/pkg/plugin-sdk/server"
)

// DemoPlugin 演示插件实现（进程外）。
type DemoPlugin struct{}

// Info 插件信息（与安装清单一致；主进程校验 ID）。
func (p *DemoPlugin) Info() sdk.Info {
	return sdk.Info{
		ID:          "demo-plugin",
		Name:        "演示插件",
		Version:     "0.1.0",
		Author:      "月言官方",
		Description: "M3.3 进程外插件演示：钩子 + 自定义 API",
	}
}

// OnActivate 启用回调（初始化资源）。
func (p *DemoPlugin) OnActivate(ctx context.Context) error {
	logf("已激活")
	return nil
}

// OnDeactivate 停用回调（释放资源）。
func (p *DemoPlugin) OnDeactivate(ctx context.Context) error {
	logf("已停用")
	return nil
}

// postPayload 帖子事件载荷（宽松解析：仅取标题/状态字段，字段缺失不报错）。
type postPayload struct {
	Title  string `json:"title"`
	Status string `json:"status"`
}

// Hooks 订阅钩子（同步拦截 + 异步通知 + 同步改写观察）。
func (p *DemoPlugin) Hooks() []sdk.Hook {
	return []sdk.Hook{
		{
			Name: "post.before_publish", Sync: true, Priority: 100,
			Handler: func(ctx context.Context, ev sdk.Event) (sdk.Result, error) {
				var payload postPayload
				if err := json.Unmarshal(ev.Payload, &payload); err != nil {
					return sdk.Result{OK: true}, nil
				}
				// 标题含 [demo] → 拒绝（验证同步拦截链路）
				if strings.Contains(payload.Title, "[demo]") {
					logf("拦截发布：%s", payload.Title)
					return sdk.Result{OK: false, Reason: "演示插件拦截：[demo] 标题禁止发布"}, nil
				}
				return sdk.Result{OK: true}, nil
			},
		},
		{
			Name: "post.after_publish", Sync: false, Priority: 100,
			Handler: func(ctx context.Context, ev sdk.Event) (sdk.Result, error) {
				var payload postPayload
				_ = json.Unmarshal(ev.Payload, &payload)
				logf("异步钩子 after_publish：标题「%s」状态「%s」", payload.Title, payload.Status)
				return sdk.Result{OK: true}, nil
			},
		},
		{
			Name: "search.query", Sync: true, Priority: 100,
			Handler: func(ctx context.Context, ev sdk.Event) (sdk.Result, error) {
				logf("同步钩子 search.query：关键词「%s」", string(ev.Payload))
				return sdk.Result{OK: true}, nil
			},
		},
	}
}

// RegisterAPI 自定义 API（主进程挂 /api/plugins/demo-plugin/** 代理）。
func (p *DemoPlugin) RegisterAPI(api *sdk.APIMux) {
	api.Handle("GET", "/ping", func(ctx context.Context, method string, path string, body []byte) (int, []byte, error) {
		logf("自定义 API：%s %s", method, path)
		return 200, []byte(`{"pong":true,"plugin":"demo-plugin"}`), nil
	})
}

// logf 插件日志（stderr → 主进程重定向到 logs/plugins/demo-plugin.log）。
func logf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "[demo-plugin] "+format+"\n", args...)
}

// main 插件进程入口（server.Serve 完成握手与 gRPC 服务注册）。
func main() {
	// 启动探针（握手前写 stderr，验证子进程 stderr 管道链路）
	fmt.Fprintln(os.Stderr, "[demo-plugin] 进程启动")
	server.Serve(&DemoPlugin{})
}

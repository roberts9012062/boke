// internal/plugin/proxy.go
// 插件自定义 API 代理（M3.3）：主进程统一挂载 /api/plugins/{id}/**，
// 按插件 ID 转发到对应子进程的 PluginAPI.Call（docs/architecture.md 6.4 API 代理）。
package plugin

import (
	"context"
	"fmt"
	"time"

	"github.com/roberts9012062/boke/pkg/plugin-sdk/proto"
)

// 插件 API 调用超时（子进程处理自定义 API 的上限）。
const apiCallTimeout = 10 * time.Second

// Call 转发自定义 API 到插件子进程（未运行返回错误）。
// 参数：pluginID 插件 ID；method/path/body 原始请求（body 为 JSON bytes，可空）。
// 返回：status HTTP 状态码、resp 响应体 JSON、err 转发错误（非 HTTP 错误）。
func (m *PluginManager) Call(ctx context.Context, pluginID string, method string, path string, body []byte) (int, []byte, error) {
	m.mu.Lock()
	mp, ok := m.managed[pluginID]
	if !ok || mp.state != stateRunning {
		m.mu.Unlock()
		return 0, nil, fmt.Errorf("插件「%s」未在运行", pluginID)
	}
	rpc := mp.rpc
	m.mu.Unlock()

	callCtx, cancel := context.WithTimeout(ctx, apiCallTimeout)
	defer cancel()
	resp, err := rpc.api.Call(callCtx, &proto.APICall{Method: method, Path: path, Body: body})
	if err != nil {
		return 0, nil, fmt.Errorf("插件「%s」API 调用失败：%w", pluginID, err)
	}
	if resp.Error != "" {
		return int(resp.Status), resp.Body, fmt.Errorf("插件「%s」API 内部错误：%s", pluginID, resp.Error)
	}
	return int(resp.Status), resp.Body, nil
}

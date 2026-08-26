// internal/plugin/proxy.go
// 插件自定义 API 代理：主进程统一挂载 /api/plugins/{id}/**，
// 按插件 ID 转发到对应子进程的 Core.CallAPI（docs/architecture.md 6.4 API 代理）。
package plugin

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/roberts9012062/boke/pkg/plugin-sdk"
	"github.com/roberts9012062/boke/pkg/plugin-sdk/contract"
)

// 插件 API 调用超时（子进程处理自定义 API 的上限）。
const apiCallTimeout = 10 * time.Second

// PushConfig 推送配置到运行中的插件（保存配置时即时生效；未运行/失败不阻断调用方）。
func (m *PluginManager) PushConfig(pluginID string, values map[string]string) error {
	m.mu.Lock()
	mp, ok := m.managed[pluginID]
	if !ok || mp.state != stateRunning {
		m.mu.Unlock()
		return nil // 未运行：启动时 Start 会下发，此处静默跳过
	}
	rpc := mp.rpc
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
	defer cancel()
	var status contract.Status
	if err := rpc.callCore(ctx, "SetConfig", &contract.ConfigInfo{Values: values}, &status); err != nil {
		m.logWarn("插件配置推送失败", zap.String("plugin", pluginID), zap.Error(err))
		return fmt.Errorf("插件「%s」配置推送失败：%w", pluginID, err)
	}
	return nil
}

// PluginInfo 拉取运行中插件的 Info（设置页 schema 聚合——进程上报优先；未运行返回错误）。
func (m *PluginManager) PluginInfo(pluginID string) (*contract.PluginInfo, error) {
	m.mu.Lock()
	mp, ok := m.managed[pluginID]
	if !ok || mp.state != stateRunning {
		m.mu.Unlock()
		return nil, fmt.Errorf("插件「%s」未在运行", pluginID)
	}
	rpc := mp.rpc
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
	defer cancel()
	var info contract.PluginInfo
	if err := rpc.callCore(ctx, "Info", &contract.Empty{}, &info); err != nil {
		m.logWarn("PluginInfo 拉取失败（schema 聚合走清单兜底）", zap.String("plugin", pluginID), zap.Error(err))
		return nil, err
	}
	return &info, nil
}

// Call 转发自定义 API 到插件子进程（未运行返回错误）。
// 参数：pluginID 插件 ID；method/path/body 原始请求（body 为 JSON bytes，可空）；
//      caller 调用者身份（经契约字段内联透传，插件侧可做 per-endpoint 鉴权）。
// 返回：status HTTP 状态码、resp 响应体 JSON、err 转发错误（非 HTTP 错误）。
func (m *PluginManager) Call(ctx context.Context, pluginID string, method string, path string, body []byte, caller sdk.CallerIdentity) (int, []byte, error) {
	m.mu.Lock()
	mp, ok := m.managed[pluginID]
	if !ok || mp.state != stateRunning {
		m.logWarn("Call 拒绝：插件未在运行", zap.String("plugin", pluginID), zap.Bool("managed", ok))
		m.mu.Unlock()
		return 0, nil, fmt.Errorf("插件「%s」未在运行", pluginID)
	}
	rpc := mp.rpc
	m.mu.Unlock()

	callCtx, cancel := context.WithTimeout(ctx, apiCallTimeout)
	defer cancel()
	var resp contract.APICallResult
	if err := rpc.callCore(callCtx, "CallAPI", &contract.APICall{
		Method: method, Path: path, Body: body,
		CallerID: caller.UserID, CallerRole: caller.Role, CallerSystem: caller.System,
	}, &resp); err != nil {
		m.logWarn("Call 失败", zap.String("plugin", pluginID), zap.String("method", method), zap.String("path", path), zap.Error(err))
		return 0, nil, fmt.Errorf("插件「%s」API 调用失败：%w", pluginID, err)
	}
	if resp.Error != "" {
		return int(resp.Status), resp.Body, fmt.Errorf("插件「%s」API 内部错误：%s", pluginID, resp.Error)
	}
	return int(resp.Status), resp.Body, nil
}

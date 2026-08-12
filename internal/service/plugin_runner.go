// internal/service/plugin_runner.go
// 插件进程外激活（M3.3）：内置插件走进程内钩子注册；其余插件由 PluginManager 拉起子进程。
// 职责：activate/deactivate 统一入口 + ManagerEvents 回调（崩溃熔断/退避重启落库）。
// 说明：进程外插件启用前校验二进制存在（缺失报「Release 安装后置 M3.4」）。
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/roberts9012062/boke/internal/plugin"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// isProcessPlugin 判断是否为进程外插件（无内置实现即进程外）。
func (s *PluginService) isProcessPlugin(pluginID string) bool {
	return plugin.BuiltinHookRegistrations(pluginID) == nil
}

// activate 激活插件（内置：注册进程内钩子；进程外：go-plugin 拉起子进程 + 注册适配器）。
func (s *PluginService) activate(ctx context.Context, pluginID string) error {
	if !s.isProcessPlugin(pluginID) {
		s.registerHooks(pluginID) // 内置插件（幂等）
		return nil
	}
	if s.manager == nil {
		return errors.New("插件进程管理器未配置")
	}
	return s.manager.Start(ctx, pluginID)
}

// deactivate 停用插件（内置：注销钩子；进程外：停子进程 + 注销适配器）。
func (s *PluginService) deactivate(pluginID string) error {
	if !s.isProcessPlugin(pluginID) {
		s.unregisterHooks(pluginID)
		return nil
	}
	if s.manager == nil {
		return nil
	}
	return s.manager.Stop(pluginID)
}

// CallAPI 转发插件自定义 API（handler 层代理调用 /api/plugins/{id}/**）。
func (s *PluginService) CallAPI(ctx context.Context, pluginID string, method string, path string, body []byte) (int, []byte, error) {
	if s.manager == nil {
		return 0, nil, errors.New("插件进程管理器未配置")
	}
	return s.manager.Call(ctx, pluginID, method, path, body)
}

// IsRunning 插件进程是否在运行（M3.5 激活后重启判断）。
func (s *PluginService) IsRunning(pluginID string) bool {
	return s.manager != nil && s.manager.IsRunning(pluginID)
}

// Restart 重启插件进程（停用再启用；M3.5 激活许可证后让 SDK 许可缓存生效）。
func (s *PluginService) Restart(ctx context.Context, pluginID string) error {
	if s.manager == nil {
		return errors.New("插件进程管理器未配置")
	}
	if err := s.manager.Stop(pluginID); err != nil {
		return err
	}
	return s.manager.Start(ctx, pluginID)
}

// PluginIDByInstance 按实例 ID 查插件 ID（许可证激活/状态等 :id 路由用）。
func (s *PluginService) PluginIDByInstance(ctx context.Context, instanceID int64) (string, error) {
	inst, err := s.plugs.FindByID(ctx, instanceID)
	if err != nil {
		return "", errs.ErrNotFound
	}
	return inst.PluginID, nil
}

// AssetDir 插件前端资源根目录（M3.6：/plugin-assets 静态服务用；ID 不合法返回空）。
func (s *PluginService) AssetDir(pluginID string) string {
	if s.store == nil {
		return ""
	}
	return s.store.Dir(pluginID)
}

// FrontendExtensionDTO 前台插件扩展项（公开接口返回：running 且含前端资产的插件）。
type FrontendExtensionDTO struct {
	PluginID string `json:"plugin_id"` // 插件 ID
	Name     string `json:"name"`      // 插件名称
}

// FrontendExtensions 前台插件扩展清单（公开：页面槽位加载插件扩展用）。
// 说明：仅返回 running 且解包目录含 frontend/manifest.json 的插件（扩展点声明入口）。
func (s *PluginService) FrontendExtensions(ctx context.Context) ([]FrontendExtensionDTO, error) {
	if s.store == nil {
		return []FrontendExtensionDTO{}, nil
	}
	installed, err := s.plugs.ListInstalled(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]FrontendExtensionDTO, 0, len(installed))
	for _, inst := range installed {
		if inst.State != PluginRunning {
			continue
		}
		// 前端扩展点声明存在（安装解包落盘，checksums 已校验）
		manifestPath := filepath.Join(s.store.Dir(inst.PluginID), "frontend", "manifest.json")
		if _, err := os.Stat(manifestPath); err == nil {
			items = append(items, FrontendExtensionDTO{PluginID: inst.PluginID, Name: inst.Name})
		}
	}
	return items, nil
}

// PluginManagerEvents 进程管理器事件落库实现（独立类型避免 service↔manager 装配循环）。
type PluginManagerEvents struct {
	plugs *repository.PluginRepo // 插件实例数据访问
}

// NewPluginManagerEvents 创建事件回调（装配方注入 pluginRepo）。
func NewPluginManagerEvents(plugs *repository.PluginRepo) *PluginManagerEvents {
	return &PluginManagerEvents{plugs: plugs}
}

// OnCrashed 崩溃熔断回调（进程管理器 → DB：crashed + last_error，供后台展示）。
func (e *PluginManagerEvents) OnCrashed(pluginID string, errMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := e.plugs.SetStateByPluginID(ctx, pluginID, PluginCrashed, errMsg); err != nil {
		_ = err // 落库失败静默（DB 抖动场景不阻塞进程管理）
	}
}

// OnRestarting 崩溃退避重启回调（DB 更新 last_error，状态保持 running——自愈中）。
func (e *PluginManagerEvents) OnRestarting(pluginID string, attempt int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msg := fmt.Sprintf("进程崩溃，第 %d 次退避重启中...", attempt)
	if err := e.plugs.SetStateByPluginID(ctx, pluginID, PluginRunning, msg); err != nil {
		_ = err
	}
}

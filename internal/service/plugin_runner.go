// internal/service/plugin_runner.go
// 插件进程外激活（M3.3）：内置插件走进程内钩子注册；其余插件由 PluginManager 拉起子进程。
// 职责：activate/deactivate 统一入口 + ManagerEvents 回调（崩溃熔断/退避重启落库）。
// 说明：进程外插件启用前校验二进制存在（缺失报「Release 安装后置 M3.4」）。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/roberts9012062/boke/internal/plugin"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/plugin-sdk"
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
// B2：统一清理该插件贡献的 seam 服务（注册可逆——卸载即回滚）。
func (s *PluginService) deactivate(pluginID string) error {
	if s.services != nil {
		s.services.UnregisterAll(pluginID)
	}
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
// caller 调用者身份（P1 加固：透传到插件进程供 per-endpoint 鉴权；宿主内部桥接传 System=true）。
func (s *PluginService) CallAPI(ctx context.Context, pluginID string, method string, path string, body []byte, caller sdk.CallerIdentity) (int, []byte, error) {
	if s.manager == nil {
		return 0, nil, errors.New("插件进程管理器未配置")
	}
	return s.manager.Call(ctx, pluginID, method, path, body, caller)
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

// PublicAssetDir 插件前端资源根目录（M3.6：/plugin-assets 静态服务专用）。
// 安全（P0 加固）：仅 running 状态插件返回目录——停用/卸载/未安装插件的资产不再公开可读；
// 与 FrontendExtensions 的「running 且有前端资产」语义对齐。ID 不合法或状态不符返回空。
func (s *PluginService) PublicAssetDir(ctx context.Context, pluginID string) string {
	if s.store == nil {
		return ""
	}
	if !plugin.ValidPluginID(pluginID) {
		return ""
	}
	inst, err := s.plugs.FindByPluginID(ctx, pluginID)
	if err != nil || inst.State != PluginRunning {
		return ""
	}
	return s.store.Dir(pluginID)
}

// FrontendExtensionDTO 前台插件扩展项（公开接口返回：running 且含前端资产的插件）。
type FrontendExtensionDTO struct {
	PluginID  string              `json:"plugin_id"`          // 插件 ID
	Name      string              `json:"name"`               // 插件名称
	SiteNav   []PluginSiteNavItem `json:"site_nav,omitempty"` // 插件声明的前台导航项（manifest siteNav，校验后）
	SitePages []string            `json:"site_pages,omitempty"` // 插件声明的前台公开页面路由（pages 中 scope=site）
}

// PluginSiteNavItem 插件前台导航项（manifest siteNav 数组元素）。
type PluginSiteNavItem struct {
	Label string `json:"label"` // 显示文案
	Path  string `json:"path"`  // 站内路径（仅允许 / 开头）
	Icon  string `json:"icon,omitempty"` // 图标 key（nav-icons 注册表；前台导航暂不渲染，管理端展示用）
}

// frontendManifestDecl 前端资产清单声明（宽松解析：仅取扩展接口所需字段，未知字段忽略）。
type frontendManifestDecl struct {
	Pages   []frontendPageDecl `json:"pages"`
	SiteNav []struct {
		Label string `json:"label"`
		Path  string `json:"path"`
		Icon  string `json:"icon"`
	} `json:"siteNav"`
}

// frontendPageDecl 清单页面声明（route + scope）。
type frontendPageDecl struct {
	Route string `json:"route"`
	Scope string `json:"scope"` // admin（默认）/ site
}

// parseFrontendManifest 读取并解析插件前端资产清单（宽松；文件缺失/损坏返回零值不报错，
// 与「无前端资产的插件不出现在扩展清单」语义一致）。
func parseFrontendManifest(dir string) frontendManifestDecl {
	var decl frontendManifestDecl
	raw, err := os.ReadFile(filepath.Join(dir, "frontend", "manifest.json"))
	if err != nil {
		return decl
	}
	_ = json.Unmarshal(raw, &decl)
	return decl
}

// sanitizeSiteNav 过滤插件导航项（纯函数）：label 非空 ≤30 字符、path 以 / 开头且非 // 开头
// （防协议注入前台头部，与 nav_links 校验同规则）；每插件最多 5 项。
func sanitizeSiteNav(items []struct {
	Label string `json:"label"`
	Path  string `json:"path"`
	Icon  string `json:"icon"`
}) []PluginSiteNavItem {
	cleaned := make([]PluginSiteNavItem, 0, len(items))
	for _, it := range items {
		if it.Label == "" || len(it.Label) > 30 {
			continue
		}
		if !strings.HasPrefix(it.Path, "/") || strings.HasPrefix(it.Path, "//") || len(it.Path) > 500 {
			continue
		}
		cleaned = append(cleaned, PluginSiteNavItem{Label: it.Label, Path: it.Path, Icon: it.Icon})
		if len(cleaned) >= 5 {
			break
		}
	}
	return cleaned
}

// sitePageRoutes 提取 scope=site 的页面路由（route 非空；去重保序）。
func sitePageRoutes(pages []frontendPageDecl) []string {
	seen := make(map[string]bool, len(pages))
	routes := make([]string, 0, len(pages))
	for _, p := range pages {
		if p.Scope != "site" || p.Route == "" || seen[p.Route] {
			continue
		}
		seen[p.Route] = true
		routes = append(routes, p.Route)
	}
	return routes
}

// FrontendExtensions 前台插件扩展清单（公开：页面槽位加载插件扩展用）。
// 说明：仅返回 running 且解包目录含 frontend/manifest.json 的插件（扩展点声明入口）；
// 同时解析清单返回插件声明的前台导航项（siteNav）与公开页面路由（pages scope=site），
// 供前台头部导航合并与公开壳路由直链校验。
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
		dir := s.store.Dir(inst.PluginID)
		// 前端扩展点声明存在（安装解包落盘，checksums 已校验）
		manifestPath := filepath.Join(dir, "frontend", "manifest.json")
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}
		decl := parseFrontendManifest(dir)
		items = append(items, FrontendExtensionDTO{
			PluginID:  inst.PluginID,
			Name:      inst.Name,
			SiteNav:   sanitizeSiteNav(decl.SiteNav),
			SitePages: sitePageRoutes(decl.Pages),
		})
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

// OnRecovered 退避重启成功回调（清除「退避重启中」提示——插件已恢复正常，
// 历史崩溃信息不再展示；再次崩溃会重新写入）。
func (e *PluginManagerEvents) OnRecovered(pluginID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := e.plugs.SetStateByPluginID(ctx, pluginID, PluginRunning, ""); err != nil {
		_ = err
	}
}

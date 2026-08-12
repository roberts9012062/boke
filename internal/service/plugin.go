// internal/service/plugin.go
// 插件服务（M3.1）：插件商城（GitHub 仓库清单驱动）+ 插件管理（安装/启用禁用/卸载）。
// 设计稿《插件商城》《插件安装·免费/付费/Loading/成功》《插件卸载·SEO/成功》。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/roberts9012062/boke/internal/ghclient"
	"github.com/roberts9012062/boke/internal/plugin"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// 默认插件源（独立插件仓库 yueyan-plugins，清单 plugins.json 在仓库根目录）。
const defaultPluginSource = "roberts9012062/yueyan-plugins"

// 核心版本（插件清单 core_version 兼容校验基准，如 ">=0.1.0"）。
const coreVersion = "0.1.0"

// 清单缓存时长（5 分钟，避免每次拉取 GitHub）。
const manifestCacheTTL = 5 * time.Minute

// 插件状态（plugin_instances.state，架构附录 B 状态字典）。
const (
	PluginInstalled   = "installed"   // 已安装（默认）
	PluginRunning     = "running"     // 已启用
	PluginDisabled    = "disabled"    // 已禁用
	PluginCrashed     = "crashed"     // 已熔断（连续崩溃，M3.3 进程外插件）
	PluginUninstalled = "uninstalled" // 已卸载（软删标记）
)

// ---------- 清单模型（GitHub plugins.json 结构） ----------

// PluginManifest 插件清单（仓库根目录 plugins.json）。
type PluginManifest struct {
	Name        string       `json:"name"`        // 插件库名称
	Description string       `json:"description"` // 插件库描述
	Plugins     []PluginInfo `json:"plugins"`     // 插件列表
}

// PluginInfo 插件信息（清单项，含兼容性契约字段）。
type PluginInfo struct {
	ID           string   `json:"id"`           // 插件 ID（唯一）
	Name         string   `json:"name"`         // 插件名称
	Version      string   `json:"version"`      // 版本
	Category     string   `json:"category"`     // 类别：seo/security/performance/analytics/writing/ops/enhancement
	Price        int      `json:"price"`        // 价格（0=免费，>0 为 ¥）
	Installs     int      `json:"installs"`     // 安装量
	Official     bool     `json:"official"`     // 官方标签
	Description  string   `json:"description"`  // 一句话描述
	Capabilities []string `json:"capabilities"` // 能力清单（安装弹层展示）
	RepoURL      string   `json:"repo_url"`     // 来源仓库
	CoreVersion  string   `json:"core_version"` // 兼容核心版本（如 ">=0.1.0"；空=不限制，M3.2 兼容性）
	Requires     []string `json:"requires"`     // 依赖插件 ID（需已安装，M3.2）
	Conflicts    []string `json:"conflicts"`    // 冲突插件 ID（不可同时安装，M3.2）
	Platforms    []string `json:"platforms,omitempty"` // 支持平台（linux/darwin/windows，M3.4）
	Assets       *PluginAssets `json:"assets,omitempty"` // Release 资产声明（M3.4）
	Nav          *PluginNav `json:"nav,omitempty"` // 侧栏入口声明（安装启用后注册，前端扩展点）
	SettingsSchema []PluginSettingField `json:"settings_schema,omitempty"` // 设置项 schema（schema 驱动设置页）
}

// PluginNav 插件侧栏入口声明（前端数据驱动扩展）。
type PluginNav struct {
	Href  string `json:"href"`  // 后台路径
	Label string `json:"label"` // 菜单名
	Icon  string `json:"icon"`  // 图标 key（前端 nav-icons 注册表）
}

// PluginAssets 插件 Release 资产声明（M3.4：.bpk 下载安装匹配）。
type PluginAssets struct {
	Pattern string `json:"pattern"` // 资产名模式（如 {id}-{version}-{os}-{arch}.bpk）
	SHA256  string `json:"sha256"`  // 包 SHA-256（清单声明，可选；下载后实算比对）
}

// PluginSettingField 插件设置项（schema 驱动通用设置页）。
type PluginSettingField struct {
	Key     string `json:"key"`     // 设置键（存 settings：plugin_{id}_{key}）
	Label   string `json:"label"`   // 标签
	Type    string `json:"type"`    // text / switch / select
	Default string `json:"default"` // 默认值
	Options []string `json:"options"` // select 选项
}

// MarketPluginDTO 商城插件 DTO（清单项 + 已安装状态）。
type MarketPluginDTO struct {
	PluginInfo
	Installed bool   `json:"installed"`  // 是否已安装
	State     string `json:"state"`      // 已安装时的状态（running/disabled/installed）
	InstanceID int64 `json:"instance_id"` // 已安装时的实例 ID（0=未安装）
}

// InstalledPluginDTO 已安装插件 DTO（我的插件页）。
type InstalledPluginDTO struct {
	ID        int64     `json:"id"`          // 实例 ID
	PluginID  string    `json:"plugin_id"`   // 插件 ID
	Name      string    `json:"name"`        // 名称
	Version   string    `json:"version"`     // 版本
	RepoURL   string    `json:"repo_url"`    // 来源仓库
	State     string    `json:"state"`       // 状态
	LastError string    `json:"last_error,omitempty"` // 最近错误（M3.3 崩溃/缺失展示）
	CreatedAt time.Time `json:"created_at"`  // 安装时间
	Nav       *PluginNav `json:"nav,omitempty"` // 侧栏入口声明（前端动态扩展）
	SettingsSchema []PluginSettingField `json:"settings_schema,omitempty"` // 设置项 schema（设置页）
}

// manifestCache 清单缓存（source → 内容 + 时间；并发安全）。
type manifestCache struct {
	mu       sync.Mutex
	content  []byte
	fetched  time.Time
	source   string
}

// PluginService 插件服务（连接器类）。
type PluginService struct {
	gh         *ghclient.Client      // GitHub 客户端（拉清单/Release 下载）
	plugs      *repository.PluginRepo // 插件实例数据访问
	settings   *repository.SettingRepo // 插件源设置
	cache      manifestCache         // 清单缓存（5 分钟）
	dispatcher plugin.Dispatcher     // 钩子调度器（M3.2 扩展框架；生命周期联动注册/注销钩子）
	manager    *plugin.PluginManager // 进程管理器（M3.3 进程外插件；可空=纯内置模式）
	store      *plugin.BinStore      // 二进制存储（M3.4 .bpk 解包落点/临时区）
}

// NewPluginService 创建插件服务。
// 参数：dispatcher 钩子调度器（业务扩展点，可空则插件钩子不生效）；
//      manager 进程管理器（进程外插件，可空则进程外插件仅记录安装不拉起）；
//      store 二进制存储（.bpk 解包/临时文件，可空则上传安装不可用）。
func NewPluginService(gh *ghclient.Client, plugs *repository.PluginRepo, settings *repository.SettingRepo, dispatcher plugin.Dispatcher, manager *plugin.PluginManager, store *plugin.BinStore) *PluginService {
	return &PluginService{gh: gh, plugs: plugs, settings: settings, cache: manifestCache{}, dispatcher: dispatcher, manager: manager, store: store}
}

// pluginSource 读取插件源仓库（settings.plugin_source，默认 roberts9012062/boke）。
func (s *PluginService) pluginSource(ctx context.Context) string {
	if v, ok, err := s.settings.Get(ctx, "plugin_source"); err == nil && ok && v != "" {
		return v
	}
	return defaultPluginSource
}

// fetchManifest 拉取并解析清单（缓存 5 分钟；source 空则用设置值）。
func (s *PluginService) fetchManifest(ctx context.Context, source string) (*PluginManifest, error) {
	if source == "" {
		source = s.pluginSource(ctx)
	}
	source = strings.TrimSpace(source)

	// 缓存命中（同源 + 未过期）
	s.cache.mu.Lock()
	if s.cache.source == source && len(s.cache.content) > 0 && time.Since(s.cache.fetched) < manifestCacheTTL {
		content := s.cache.content
		s.cache.mu.Unlock()
		return parseManifest(content)
	}
	s.cache.mu.Unlock()

	// 拉取（GitHub Contents API，仓库根目录 plugins.json）
	parts := strings.SplitN(source, "/", 2)
	if len(parts) != 2 {
		return nil, errs.New(errs.CodeBadRequest, "插件源格式应为 owner/repo")
	}
	raw, err := s.gh.FetchManifest(ctx, parts[0], parts[1])
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, err.Error())
	}

	// 写缓存
	s.cache.mu.Lock()
	s.cache.content = raw
	s.cache.fetched = time.Now()
	s.cache.source = source
	s.cache.mu.Unlock()

	return parseManifest(raw)
}

// parseManifest 解析清单 JSON。
func parseManifest(raw []byte) (*PluginManifest, error) {
	var manifest PluginManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, errs.New(errs.CodeUpstream, "插件清单格式不正确")
	}
	if len(manifest.Plugins) == 0 {
		return nil, errs.New(errs.CodeUpstream, "插件清单为空")
	}
	return &manifest, nil
}

// ---------- 商城 ----------

// Market 拉取插件商城（清单 + 已安装状态合并）。
// 参数：source 插件源仓库（空 = 设置值）。
// 返回：清单、插件列表、实际生效源（前端展示用）。
func (s *PluginService) Market(ctx context.Context, source string) (*PluginManifest, []MarketPluginDTO, string, error) {
	actual := source
	if actual == "" {
		actual = s.pluginSource(ctx)
	}
	manifest, err := s.fetchManifest(ctx, source)
	if err != nil {
		return nil, nil, actual, err
	}
	// 已安装实例（plugin_id → 实例）
	installed, err := s.plugs.ListInstalled(ctx)
	if err != nil {
		return nil, nil, actual, err
	}
	byID := make(map[string]repository.PluginInstance, len(installed))
	for _, inst := range installed {
		byID[inst.PluginID] = inst
	}

	items := make([]MarketPluginDTO, 0, len(manifest.Plugins))
	for _, p := range manifest.Plugins {
		dto := MarketPluginDTO{PluginInfo: p}
		if inst, ok := byID[p.ID]; ok {
			dto.Installed = true
			dto.State = inst.State
			dto.InstanceID = inst.ID
		}
		items = append(items, dto)
	}
	return manifest, items, actual, nil
}

// ---------- 插件管理 ----------

// ListInstalled 已安装插件列表（我的插件页；从清单补充 nav/settings_schema——缓存命中不额外请求）。
func (s *PluginService) ListInstalled(ctx context.Context) ([]InstalledPluginDTO, error) {
	installed, err := s.plugs.ListInstalled(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]InstalledPluginDTO, 0, len(installed))
	for _, inst := range installed {
		items = append(items, InstalledPluginDTO{
			ID: inst.ID, PluginID: inst.PluginID, Name: inst.Name,
			Version: inst.Version, RepoURL: inst.RepoURL, State: inst.State,
			LastError: inst.LastError, CreatedAt: inst.CreatedAt,
		})
	}
	// 清单补充（nav/settings_schema；拉取失败静默——列表仍可用）
	if manifest, err := s.fetchManifest(ctx, ""); err == nil {
		byID := make(map[string]PluginInfo, len(manifest.Plugins))
		for _, p := range manifest.Plugins {
			byID[p.ID] = p
		}
		for i := range items {
			if info, ok := byID[items[i].PluginID]; ok {
				items[i].Nav = info.Nav
				items[i].SettingsSchema = info.SettingsSchema
			}
		}
	}
	return items, nil
}

// Install 安装插件（从清单取信息 → 写 plugin_instances；重复安装返回冲突）。
// 参数：pluginID 清单插件 ID。
func (s *PluginService) Install(ctx context.Context, pluginID string) error {
	manifest, err := s.fetchManifest(ctx, "")
	if err != nil {
		return err
	}
	var info *PluginInfo
	for i := range manifest.Plugins {
		if manifest.Plugins[i].ID == pluginID {
			info = &manifest.Plugins[i]
			break
		}
	}
	if info == nil {
		return errs.New(errs.CodeNotFound, "插件不存在")
	}
	// ---------- 兼容性校验（M3.2：core_version / requires / conflicts） ----------
	if err := s.checkCompatibility(ctx, info); err != nil {
		return err
	}

	// ---------- 安装来源（M3.4）：清单声明 Release 资产且无本地二进制 → 下载安装 ----------
	// 说明：本地预置二进制优先（开发/调试）；Release 下载内部完成解包校验与实例注册激活。
	if s.isProcessPlugin(info.ID) && info.Assets != nil && info.Assets.Pattern != "" &&
		s.store != nil && !s.store.Exists(info.ID) && s.gh != nil {
		return s.installFromRelease(ctx, info)
	}

	// 重复安装检查（已安装返回冲突；已卸载记录复用重装——plugin_id 唯一约束）
	existing, err := s.plugs.FindByPluginID(ctx, pluginID)
	if err == nil {
		if existing.State != PluginUninstalled {
			return errs.New(errs.CodeConflict, "插件「"+info.Name+"」已安装")
		}
		// 重新安装：复用记录（恢复 installed + 更新版本/来源），再尝试激活
		if err := s.plugs.Reinstall(ctx, existing.ID, info.Version, info.RepoURL); err != nil {
			return fmt.Errorf("重新安装插件失败：%w", err)
		}
		s.activateInstalled(ctx, existing.ID, info.ID)
		return nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	// 新建实例（默认 installed；激活成功转 running——M3.3 进程外插件需二进制）
	instanceID, err := s.plugs.Create(ctx, repository.PluginInstance{
		PluginID: info.ID,
		Name:     info.Name,
		Version:  info.Version,
		RepoURL:  info.RepoURL,
		State:    PluginInstalled,
	})
	if err != nil {
		return fmt.Errorf("安装插件失败：%w", err)
	}
	// 安装即启用（内置注册钩子 / 进程外拉起进程；二进制缺失时保持 installed）
	s.activateInstalled(ctx, instanceID, info.ID)
	return nil
}

// activateInstalled 安装后的激活尝试：成功转 running；失败保持 installed（last_error 提示）。
// 说明（M3.3）：进程外插件二进制缺失不阻断安装（Release 安装功能后置 M3.4）。
func (s *PluginService) activateInstalled(ctx context.Context, instanceID int64, pluginID string) {
	if err := s.activate(ctx, pluginID); err != nil {
		_ = s.plugs.SetStateByPluginID(ctx, pluginID, PluginInstalled, err.Error())
		return
	}
	if err := s.plugs.SetState(ctx, instanceID, PluginRunning); err != nil {
		_ = err
	}
}

// checkCompatibility 插件兼容性校验（core_version 匹配 / requires 已装 / conflicts 未装）。
// 参数：info 清单插件信息（含兼容字段）。
func (s *PluginService) checkCompatibility(ctx context.Context, info *PluginInfo) error {
	// core_version：如 ">=0.1.0"（MVP 支持 >= 前缀；空=不限制）
	if info.CoreVersion != "" {
		expect := strings.TrimPrefix(info.CoreVersion, ">=")
		if !strings.HasPrefix(info.CoreVersion, ">=") {
			return errs.New(errs.CodeBadRequest, "插件兼容声明格式不正确（应为 >=x.y.z）")
		}
		if versionLess(coreVersion, expect) {
			return errs.New(errs.CodeConflict, "插件「"+info.Name+"」要求核心版本 "+info.CoreVersion+"，当前 "+coreVersion+"，请先升级核心")
		}
	}
	// requires：依赖插件必须已安装（未卸载）
	for _, dep := range info.Requires {
		inst, err := s.plugs.FindByPluginID(ctx, dep)
		if err != nil || inst.State == PluginUninstalled {
			return errs.New(errs.CodeConflict, "插件「"+info.Name+"」依赖插件「"+dep+"」，请先安装")
		}
	}
	// conflicts：冲突插件不可同时安装
	for _, conflict := range info.Conflicts {
		inst, err := s.plugs.FindByPluginID(ctx, conflict)
		if err == nil && inst.State != PluginUninstalled {
			return errs.New(errs.CodeConflict, "插件「"+info.Name+"」与「"+conflict+"」冲突，请先卸载")
		}
	}
	return nil
}

// versionLess 版本号比较（x.y.z 三元组；a < b 返回 true）。
func versionLess(a string, b string) bool {
	pa := parseVersion(a)
	pb := parseVersion(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

// parseVersion 解析版本号（"0.1.0" → [0,1,0]；非法段按 0）。
func parseVersion(v string) [3]int {
	parts := strings.SplitN(v, ".", 3)
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err == nil {
			out[i] = n
		}
	}
	return out
}

// SetState 启用/禁用插件（running / disabled；先激活/停用成功再落库）。
// 说明（M3.3）：启用失败（如进程外插件二进制缺失）返回错误，DB 状态不变。
func (s *PluginService) SetState(ctx context.Context, instanceID int64, state string) error {
	if state != PluginRunning && state != PluginDisabled {
		return errs.New(errs.CodeBadRequest, "状态仅支持 running / disabled")
	}
	inst, err := s.plugs.FindByID(ctx, instanceID)
	if err != nil {
		return errs.ErrNotFound
	}
	// 先执行激活/停用（失败不改 DB），成功后再落库
	if state == PluginRunning {
		if err := s.activate(ctx, inst.PluginID); err != nil {
			return errs.New(errs.CodeUpstream, err.Error())
		}
	} else if err := s.deactivate(inst.PluginID); err != nil {
		return errs.New(errs.CodeUpstream, err.Error())
	}
	return s.plugs.SetState(ctx, instanceID, state)
}

// Uninstall 卸载插件（软删；生命周期联动停进程/注销钩子）。
func (s *PluginService) Uninstall(ctx context.Context, instanceID int64) error {
	inst, err := s.plugs.FindByID(ctx, instanceID)
	if err != nil {
		return errs.ErrNotFound
	}
	if err := s.deactivate(inst.PluginID); err != nil {
		return errs.New(errs.CodeUpstream, err.Error())
	}
	if err := s.plugs.Delete(ctx, instanceID); err != nil {
		return err
	}
	return nil
}

// ---------- 钩子生命周期联动（M3.2 扩展框架） ----------

// registeredHooks 已注册的钩子项（pluginID → 注册项，注销时精确移除）。
type hookRegistrations struct {
	items []plugin.HookRegistration
}

// pluginHooks 当前已注册的插件钩子（并发安全）。
var pluginHooks = struct {
	sync.RWMutex
	byPlugin map[string]*hookRegistrations
}{byPlugin: make(map[string]*hookRegistrations)}

// SyncActivePlugins 启动时恢复已启用插件（重启后：内置注册钩子 / 进程外拉起子进程）。
// 说明（M3.2 修复）：钩子注册在安装/启用时进行，重启后需按 DB 中 running 状态恢复；
//              M3.3 扩展：进程外插件同时拉起子进程（二进制缺失置回 installed 提示）。
func (s *PluginService) SyncActivePlugins(ctx context.Context) error {
	installed, err := s.plugs.ListInstalled(ctx)
	if err != nil {
		return err
	}
	for _, inst := range installed {
		if inst.State != PluginRunning {
			continue
		}
		if err := s.activate(ctx, inst.PluginID); err != nil {
			// 恢复失败（如二进制缺失）：置回 installed + last_error，避免僵在 running
			_ = s.plugs.SetStateByPluginID(ctx, inst.PluginID, PluginInstalled, err.Error())
		}
	}
	return nil
}

// registerHooks 注册插件钩子（内置注册表；Dispatcher 为空则跳过）。
func (s *PluginService) registerHooks(pluginID string) {
	if s.dispatcher == nil {
		return
	}
	regs := plugin.BuiltinHookRegistrations(pluginID)
	if len(regs) == 0 {
		return
	}
	pluginHooks.Lock()
	defer pluginHooks.Unlock()
	// 已注册则跳过（幂等）
	if _, ok := pluginHooks.byPlugin[pluginID]; ok {
		return
	}
	for _, reg := range regs {
		s.dispatcher.Register(reg.Hook, reg.Handler)
	}
	pluginHooks.byPlugin[pluginID] = &hookRegistrations{items: regs}
}

// unregisterHooks 注销插件钩子（精确按注册项移除）。
func (s *PluginService) unregisterHooks(pluginID string) {
	if s.dispatcher == nil {
		return
	}
	pluginHooks.Lock()
	defer pluginHooks.Unlock()
	regs, ok := pluginHooks.byPlugin[pluginID]
	if !ok {
		return
	}
	for _, reg := range regs.items {
		s.dispatcher.Unregister(reg.Hook, reg.Handler)
	}
	delete(pluginHooks.byPlugin, pluginID)
}

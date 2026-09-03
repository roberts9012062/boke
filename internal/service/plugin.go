// internal/service/plugin.go
// 插件服务（M3.1）：插件管理（安装/启用禁用/卸载）+ 钩子生命周期联动。
// 商城清单与插件介绍见 plugin_market.go（M5 文件夹结构）；Release 下载安装见 plugin_bpk.go。
// 设计稿《插件商城》《插件安装·免费/付费/Loading/成功》《插件卸载·SEO/成功》。
package service

import (
	"context"
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

// 核心版本（插件清单 core_version 兼容校验基准，如 ">=0.1.0"）。
const coreVersion = "0.1.0"

// 插件状态（plugin_instances.state，架构附录 B 状态字典）。
const (
	PluginInstalled   = "installed"   // 已安装（默认）
	PluginRunning     = "running"     // 已启用
	PluginDisabled    = "disabled"    // 已禁用
	PluginCrashed     = "crashed"     // 已熔断（连续崩溃，M3.3 进程外插件）
	PluginUninstalled = "uninstalled" // 已卸载（软删标记）
)

// ---------- 清单模型与缓存（见 plugin_market.go：文件夹结构） ----------

// InstalledPluginDTO 已安装插件 DTO（我的插件页）。
type InstalledPluginDTO struct {
	ID        int64     `json:"id"`          // 实例 ID
	PluginID  string    `json:"plugin_id"`   // 插件 ID
	Name      string    `json:"name"`        // 名称
	Version   string    `json:"version"`     // 版本
	RepoURL   string    `json:"repo_url"`    // 来源仓库
	State     string    `json:"state"`       // 状态
	LastError string    `json:"last_error,omitempty"` // 最近错误（M3.3 崩溃/缺失展示）
	License   *LicenseStatusDTO `json:"license,omitempty"` // 许可证状态（M3.5；免费插件无）
	CreatedAt time.Time `json:"created_at"`  // 安装时间
	Nav       *PluginNav `json:"nav,omitempty"` // 侧栏入口声明（前端动态扩展）
	SettingsSchema []PluginSettingField `json:"settings_schema,omitempty"` // 设置项 schema（设置页）
	StorageProvider bool `json:"storage_provider,omitempty"` // 图床声明（media.storage seam 候选；后台设置页图床插件下拉）
	StorageRawUpload bool `json:"storage_raw_upload,omitempty"` // 图床直传保留原图（发帖直传通道跳过前端压缩）
}

// manifestCache/readmeCache 类型定义见 plugin_market.go（商城缓存）。

// PluginService 插件服务（连接器类）。
type PluginService struct {
	gh         *ghclient.Client      // GitHub 客户端（拉清单/Release 下载）
	plugs      *repository.PluginRepo // 插件实例数据访问
	licenses   *repository.LicenseRepo // 插件许可证数据访问（M3.5）
	settings   *repository.SettingRepo // 插件源设置
	orders     *repository.PluginOrderRepo // 购买订单（M3.9 支付渠道）
	keySecret  string                 // AES 加密种子（M3.9 签发私钥加密存储）
	cache      manifestCache         // 商城清单缓存（5 分钟，plugin_market.go）
	readme     readmeCache           // 插件介绍缓存（5 分钟，plugin_market.go）
	dispatcher plugin.Dispatcher     // 钩子调度器（M3.2 扩展框架；生命周期联动注册/注销钩子）
	manager    *plugin.PluginManager // 进程管理器（M3.3 进程外插件；可空=纯内置模式）
	store      *plugin.BinStore      // 二进制存储（M3.4 .bpk 解包落点/临时区）
	servicesOnce sync.Once           // seam 注册表懒初始化保护（B2）
	services   *plugin.ServiceRegistry // seam 服务注册表（懒初始化，见 plugin_seam.go）
}

// NewPluginService 创建插件服务。
// 参数：dispatcher 钩子调度器（业务扩展点，可空则插件钩子不生效）；
//      manager 进程管理器（进程外插件，可空则进程外插件仅记录安装不拉起）；
//      store 二进制存储（.bpk 解包/临时文件，可空则上传安装不可用）；
//      licenses 许可证仓库（M3.5，可空则激活接口不可用）；
//      orders 购买订单仓库（M3.9 支付渠道）；keySecret AES 加密种子（签发私钥加密存储）。
func NewPluginService(gh *ghclient.Client, plugs *repository.PluginRepo, settings *repository.SettingRepo, dispatcher plugin.Dispatcher, manager *plugin.PluginManager, store *plugin.BinStore, licenses *repository.LicenseRepo, orders *repository.PluginOrderRepo, keySecret string) *PluginService {
	return &PluginService{gh: gh, plugs: plugs, licenses: licenses, settings: settings, orders: orders, keySecret: keySecret, cache: manifestCache{}, readme: readmeCache{items: make(map[string]readmeEntry)}, dispatcher: dispatcher, manager: manager, store: store}
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
		item := InstalledPluginDTO{
			ID: inst.ID, PluginID: inst.PluginID, Name: inst.Name,
			Version: inst.Version, RepoURL: inst.RepoURL, State: inst.State,
			LastError: inst.LastError, CreatedAt: inst.CreatedAt,
		}
		// 许可证状态（M3.5：仅已登记公钥的付费插件展示；查询失败静默）
		if inst.Pubkey != "" && s.licenses != nil {
			if status, err := s.LicenseStatus(ctx, inst.PluginID); err == nil {
				item.License = status
			}
		}
		items = append(items, item)
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
				items[i].StorageProvider = info.StorageProvider
				items[i].StorageRawUpload = info.StorageRawUpload
			}
		}
	}
	// 本地镜像兜底（2026-08-19 修复）：清单拉取失败/插件不在清单时 nav 补空——
	// 运行中插件的侧栏入口不依赖远程网络（见 plugin_manifest_local.go）
	fillNavFromLocalRepo(items)
	// 进程上报聚合（M3.7：运行中插件 schema 以 Info RPC 优先——本地安装插件也可配置；
	// 仅补空——列表已有清单 schema 的不覆盖）
	if s.manager != nil {
		for i := range items {
			if len(items[i].SettingsSchema) > 0 {
				continue
			}
			if info, err := s.manager.PluginInfo(items[i].PluginID); err == nil && len(info.Settings) > 0 {
				items[i].SettingsSchema = settingFieldsFromContract(info.Settings)
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
		return s.installFromRelease(ctx, info, false)
	}

	// 重复安装检查（已安装返回冲突；已卸载记录复用重装——plugin_id 唯一约束）
	existing, err := s.plugs.FindByPluginID(ctx, pluginID)
	if err == nil {
		if existing.State != PluginUninstalled {
			return errs.New(errs.CodeConflict, "插件「"+info.Name+"」已安装")
		}
		// 重新安装：复用记录（恢复 installed + 更新版本/来源/能力登记），再尝试激活
		if err := s.plugs.Reinstall(ctx, existing.ID, info.Name, info.Version, info.RepoURL, info.Capabilities); err != nil {
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
		PluginID:     info.ID,
		Name:         info.Name,
		Version:      info.Version,
		RepoURL:      info.RepoURL,
		State:        PluginInstalled,
		Capabilities: info.Capabilities,
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
	// capabilities（M3.8 授权模型）：声明未知能力 → 拒绝安装（防越权行为声明）
	if unknown := unknownCapabilities(info.Capabilities); len(unknown) > 0 {
		return errs.New(errs.CodeBadRequest,
			"插件「"+info.Name+"」声明了未知能力："+strings.Join(unknown, "、")+
				"（支持："+strings.Join(knownCapabilitiesList(), "、")+"）")
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

// CheckAPIMiddleware 插件 api.middleware 拦截（M3.9：同步钩子；写请求经 router 中间件调用，
// 无插件订阅时空跑——Registry 空快照直接放行；拒绝返回原因供 403 响应）。
func (s *PluginService) CheckAPIMiddleware(ctx context.Context, method string, path string, userID int64) (bool, string) {
	if s.dispatcher == nil {
		return true, ""
	}
	res := s.dispatcher.Dispatch(ctx, plugin.HookAPIMiddleware, plugin.Event{
		ActorID: userID,
		Payload: map[string]any{"method": method, "path": path, "user_id": userID},
	})
	return res.OK, res.Reason
}

// SetState 启用/禁用插件（running / disabled；先激活/停用成功再落库）。// 说明（M3.3）：启用失败（如进程外插件二进制缺失）返回错误，DB 状态不变。
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
		// D3：优先级注册（小值先执行；同值按注册顺序）
		s.dispatcher.RegisterRanked(reg.Hook, "builtin/"+pluginID+"/"+reg.Hook, reg.Priority, reg.Handler)
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
		// 与注册对称：按 builtin 唯一标识精确移除
		s.dispatcher.UnregisterWithID(reg.Hook, "builtin/"+pluginID+"/"+reg.Hook)
	}
	delete(pluginHooks.byPlugin, pluginID)
}

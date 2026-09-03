// internal/service/plugin_market.go
// 插件商城（M5 文件夹结构）：插件源仓库按「每个插件一个文件夹」组织，文件夹内含
// plugin.json（元数据）+ README.md（介绍）。商城从 GitHub 文件树枚举文件夹组装清单，
// 详情接口按需拉取插件 README 原文（前端渲染 Markdown 展示）。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/roberts9012062/boke/internal/ghclient"
	"github.com/roberts9012062/boke/internal/plugin"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// 默认插件源（独立插件仓库 yueyan-plugins，文件夹结构）。
const defaultPluginSource = "roberts9012062/yueyan-plugins"

// 清单缓存时长（15 分钟——市场清单低频变化；GitHub 匿名额度仅 60 次/时，
// 每次刷新全量构建要打 10 次上下 API，缓存过短会快速烧尽额度触发 403）。
const manifestCacheTTL = 15 * time.Minute

// 商城文件大小上限（plugin.json / market.json / README.md 均为小文件）。
const marketFileLimit = 512 * 1024

// 逐文件拉取的连续失败上限（达到即判定系统性故障中止构建）。
// 背景（2026-08-19 线上问题）：网络抖动/限流时逐个拉取从某处起连续失败，旧逻辑
// 逐个跳过产出「缺插件」的不完整清单并被缓存固化，导致已装插件侧栏入口整批消失直到重启。
const manifestConsecutiveFailLimit = 3

// ---------- 清单模型（插件仓库文件夹结构） ----------

// PluginManifest 插件商城清单（由仓库文件夹结构组装）。
type PluginManifest struct {
	Name        string       `json:"name"`        // 商城名称（根 market.json，缺省=仓库名）
	Description string       `json:"description"` // 商城描述（根 market.json，可为空）
	Plugins     []PluginInfo `json:"plugins"`     // 插件列表（各文件夹 plugin.json）
}

// PluginInfo 插件信息（插件文件夹 plugin.json，含兼容性契约字段）。
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
	MusicProvider string   `json:"music_provider,omitempty"` // 音乐源声明（E7：provider 名如 qq/netease；宿主 /music/:provider/* 桥接动态发现）
	StorageProvider bool  `json:"storage_provider,omitempty"` // 图床声明（media.storage seam 提供方候选：发帖上传直达外部存储；宿主按设置项/清单发现选取）
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
	SHA256  string `json:"sha256"`  // 包 SHA-256（单值声明，可选；下载后实算比对）
	// SHA256ByPlatform 按平台声明的包哈希（键为 "{os}-{arch}"，如 linux-arm64）。
	// 多平台分发时各平台包内容不同、哈希不同——优先按平台匹配，无匹配回退 SHA256 单值。
	SHA256ByPlatform map[string]string `json:"sha256_by_platform,omitempty"`
}

// HashForPlatform 取目标平台应校验的包哈希（平台专属优先，回退单值声明；均无返回空串=跳过校验）。
func (a *PluginAssets) HashForPlatform(goos string, goarch string) string {
	if a == nil {
		return ""
	}
	if a.SHA256ByPlatform != nil {
		if h, ok := a.SHA256ByPlatform[goos+"-"+goarch]; ok {
			return h
		}
	}
	return a.SHA256
}

// PluginSettingField 插件设置项（schema 驱动通用设置页）。
type PluginSettingField struct {
	Key     string   `json:"key"`     // 设置键（存 settings：plugin_{id}_{key}）
	Label   string   `json:"label"`   // 标签
	Type    string   `json:"type"`    // text / switch / select
	Default string   `json:"default"` // 默认值
	Options []string `json:"options"` // select 选项
}

// MarketPluginDTO 商城插件 DTO（清单项 + 已安装状态）。
type MarketPluginDTO struct {
	PluginInfo
	Installed  bool   `json:"installed"`   // 是否已安装
	State      string `json:"state"`       // 已安装时的状态（running/disabled/installed）
	InstanceID int64  `json:"instance_id"` // 已安装时的实例 ID（0=未安装）
}

// marketMeta 商城元信息（仓库根 market.json，可选）。
type marketMeta struct {
	Name        string `json:"name"`        // 商城名称
	Description string `json:"description"` // 商城描述
}

// manifestCache 清单缓存（source → 组装后的清单；并发安全）。
type manifestCache struct {
	mu       sync.Mutex
	source   string
	manifest *PluginManifest
	fetched  time.Time
}

// readmeEntry readme 缓存条目。
type readmeEntry struct {
	content string
	fetched time.Time
}

// readmeCache 插件介绍缓存（source/pluginID → 内容；并发安全）。
type readmeCache struct {
	mu    sync.Mutex
	items map[string]readmeEntry
}

// pluginSource 读取插件源仓库（settings.plugin_source，默认 yueyan-plugins）。
func (s *PluginService) pluginSource(ctx context.Context) string {
	if v, ok, err := s.settings.Get(ctx, "plugin_source"); err == nil && ok && v != "" {
		return v
	}
	return defaultPluginSource
}

// pluginProxy 读取 GitHub 加速代理（settings.plugin_proxy，空 = 直连）。
// 面向国内网络：api.github.com DNS 不可达时，管理员在商城页选择代理地址加速拉取。
func (s *PluginService) pluginProxy(ctx context.Context) string {
	if v, ok, err := s.settings.Get(ctx, "plugin_proxy"); err == nil && ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// applyGHProxy 把代理设置应用到 GitHub 客户端（每次网络访问前刷新，切换代理即时生效无需重启）。
func (s *PluginService) applyGHProxy(ctx context.Context) {
	s.gh.SetProxy(s.pluginProxy(ctx))
}

// splitSource 解析插件源为 owner/repo（纯函数）。
func splitSource(source string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(source), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errs.New(errs.CodeBadRequest, "插件源格式应为 owner/repo")
	}
	return parts[0], parts[1], nil
}

// fetchManifest 拉取并组装商城清单（缓存 5 分钟；source 空则用设置值）。
func (s *PluginService) fetchManifest(ctx context.Context, source string) (*PluginManifest, error) {
	if source == "" {
		source = s.pluginSource(ctx)
	}
	source = strings.TrimSpace(source)
	s.applyGHProxy(ctx) // 刷新代理设置（切换代理后重试即时生效）

	// 缓存命中（同源 + 未过期）
	s.cache.mu.Lock()
	if s.cache.source == source && s.cache.manifest != nil && time.Since(s.cache.fetched) < manifestCacheTTL {
		manifest := s.cache.manifest
		s.cache.mu.Unlock()
		return manifest, nil
	}
	s.cache.mu.Unlock()

	owner, repo, err := splitSource(source)
	if err != nil {
		return nil, err
	}
	manifest, err := s.buildManifest(ctx, owner, repo)
	if err != nil {
		// 失效降级：同源旧缓存兜底（哪怕已过期）——限流/网络故障时商城保持可用，
		// 清单结构低频变化，过期数据远好于整页报错；无缓存才透传错误。
		s.cache.mu.Lock()
		stale, cachedSource := s.cache.manifest, s.cache.source
		s.cache.mu.Unlock()
		if cachedSource == source && stale != nil {
			fmt.Fprintf(os.Stderr, "[plugin-market] 清单拉取失败，降级使用旧缓存：%v\n", err)
			return stale, nil
		}
		return nil, err
	}

	// 写缓存
	s.cache.mu.Lock()
	s.cache.source = source
	s.cache.manifest = manifest
	s.cache.fetched = time.Now()
	s.cache.mu.Unlock()
	return manifest, nil
}

// buildManifest 从插件仓库文件夹结构组装清单（文件树 → 各插件 plugin.json → 商城元信息）。
func (s *PluginService) buildManifest(ctx context.Context, owner string, repo string) (*PluginManifest, error) {
	paths, err := s.gh.FetchTree(ctx, owner, repo)
	if err != nil {
		// GitHub 匿名额度用尽（未认证 60 次/时，按出口 IP 计）：给可操作引导
		// ——认证后 5000 次/时，是根治途径（市场设置 OAuth 连接或 .env 配置）。
		if strings.Contains(strings.ToLower(err.Error()), "rate limit") {
			return nil, errs.New(errs.CodeUpstream, "GitHub 匿名请求额度已用尽（未认证限 60 次/小时）。请在后台「插件市场设置」连接 GitHub 账号，或在服务端 .env 配置 GITHUB_TOKEN 后重启")
		}
		// DNS 解析失败/连接不通时按代理状态给引导（国内网络直连 GitHub 常见 + 公共代理单点故障）
		if strings.Contains(err.Error(), "no such host") || strings.Contains(err.Error(), "dial tcp") {
			hint := "当前网络无法直连 GitHub，请在商城页选择加速代理地址后重试"
			if s.pluginProxy(ctx) != "" {
				hint = "当前代理地址不可达，请在商城页切换其他代理后重试"
			}
			return nil, errs.New(errs.CodeUpstream, err.Error()+"。"+hint)
		}
		return nil, errs.New(errs.CodeUpstream, err.Error())
	}
	folders := pluginFolders(paths)
	if len(folders) == 0 {
		return nil, errs.New(errs.CodeUpstream,
			"插件源仓库需按文件夹结构组织：每个插件一个文件夹，内含 plugin.json（元数据）与 README.md（介绍）")
	}
	// 逐个文件夹拉取解析；单个损坏跳过（留痕），不拖垮整个商城。
	// 例外：限流错误必须中止构建——否则会产出「缺插件」的不完整清单并被缓存固化。
	plugins := make([]PluginInfo, 0, len(folders))
	consecutiveFails := 0 // 连续失败计数（成功清零；单点损坏跳过，连续失败判系统性故障）
	for _, folder := range folders {
		raw, err := s.gh.FetchFile(ctx, owner, repo, folder+"/plugin.json", marketFileLimit)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "rate limit") {
				return nil, errs.New(errs.CodeUpstream, "GitHub 匿名请求额度已用尽（未认证限 60 次/小时）。请在后台「插件市场设置」连接 GitHub 账号，或在服务端 .env 配置 GITHUB_TOKEN 后重启")
			}
			// 连续失败达上限：判定系统性故障（网络中断/持续限流），中止构建——
			// 避免产出「缺插件」的不完整清单被缓存固化（fetchManifest 过期降级会延续到重启）
			consecutiveFails++
			if consecutiveFails >= manifestConsecutiveFailLimit {
				return nil, errs.New(errs.CodeUpstream, "清单文件连续拉取失败（疑似网络故障或被限流），已中止构建，请稍后重试："+err.Error())
			}
			fmt.Fprintf(os.Stderr, "[plugin-market] 跳过插件（plugin.json 拉取失败，%s）：%v\n", folder, err)
			continue
		}
		consecutiveFails = 0
		var info PluginInfo
		if err := json.Unmarshal(raw, &info); err != nil || info.ID == "" {
			fmt.Fprintf(os.Stderr, "[plugin-market] 跳过插件（plugin.json 解析失败或缺少 id，%s）：%v\n", folder, err)
			continue
		}
		plugins = append(plugins, info)
	}
	if len(plugins) == 0 {
		return nil, errs.New(errs.CodeUpstream, "插件源仓库未解析到有效插件（plugin.json 缺失或格式错误）")
	}
	meta := s.fetchMarketMeta(ctx, owner, repo)
	if meta.Name == "" {
		meta.Name = repo
	}
	return &PluginManifest{Name: meta.Name, Description: meta.Description, Plugins: plugins}, nil
}

// pluginFolders 从文件树路径中提取插件文件夹（顶层目录且含 plugin.json；纯函数）。
func pluginFolders(paths []string) []string {
	seen := make(map[string]bool)
	for _, p := range paths {
		if !strings.HasSuffix(p, "/plugin.json") {
			continue
		}
		folder := strings.TrimSuffix(p, "/plugin.json")
		// 仅顶层目录（不含子路径）视为插件文件夹
		if folder != "" && !strings.Contains(folder, "/") {
			seen[folder] = true
		}
	}
	folders := make([]string, 0, len(seen))
	for folder := range seen {
		folders = append(folders, folder)
	}
	sort.Strings(folders) // 稳定输出（按文件夹名排序）
	return folders
}

// fetchMarketMeta 拉取仓库根 market.json（可选）；失败返回空（商城名用仓库名兜底）。
func (s *PluginService) fetchMarketMeta(ctx context.Context, owner string, repo string) marketMeta {
	var meta marketMeta
	raw, err := s.gh.FetchFile(ctx, owner, repo, "market.json", marketFileLimit)
	if err != nil {
		return meta
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return marketMeta{}
	}
	return meta
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

// Readme 拉取插件介绍（{pluginID}/README.md 原文，5 分钟缓存）。
// 参数：source 插件源仓库（空 = 设置值）；pluginID 插件 ID（白名单校验 + 需在清单中）。
// 返回：Markdown 原文（前端渲染展示）；未提供 README 返回 404 语义错误。
func (s *PluginService) Readme(ctx context.Context, source string, pluginID string) (string, error) {
	if !plugin.ValidPluginID(pluginID) {
		return "", errs.ErrNotFound
	}
	if source == "" {
		source = s.pluginSource(ctx)
	}
	source = strings.TrimSpace(source)

	// 插件需在清单中（防任意路径读取）
	manifest, err := s.fetchManifest(ctx, source)
	if err != nil {
		return "", err
	}
	found := false
	for _, p := range manifest.Plugins {
		if p.ID == pluginID {
			found = true
			break
		}
	}
	if !found {
		return "", errs.ErrNotFound
	}

	// readme 缓存
	key := source + "/" + pluginID
	s.readme.mu.Lock()
	if entry, ok := s.readme.items[key]; ok && time.Since(entry.fetched) < manifestCacheTTL {
		s.readme.mu.Unlock()
		return entry.content, nil
	}
	s.readme.mu.Unlock()

	owner, repo, err := splitSource(source)
	if err != nil {
		return "", err
	}
	raw, err := s.gh.FetchFile(ctx, owner, repo, pluginID+"/README.md", marketFileLimit)
	if err != nil {
		if errors.Is(err, ghclient.ErrFileNotFound) {
			return "", errs.New(errs.CodeNotFound, "该插件暂无介绍（README.md 未提供）")
		}
		return "", errs.New(errs.CodeUpstream, err.Error())
	}
	content := string(raw)

	s.readme.mu.Lock()
	s.readme.items[key] = readmeEntry{content: content, fetched: time.Now()}
	s.readme.mu.Unlock()
	return content, nil
}

// MusicProviderPlugin 按音乐源 provider 名发现运行中插件（E7 可插拔桥接）。
// 查询市场清单 music_provider 声明 → 校验已安装且 running → 返回插件 ID；
// 未声明/未安装/未运行/清单拉取失败返回空串（调用方回退静态注册表）。
func (s *PluginService) MusicProviderPlugin(ctx context.Context, provider string) (string, error) {
	if provider == "" {
		return "", nil
	}
	manifest, err := s.fetchManifest(ctx, "")
	if err != nil {
		return "", err
	}
	var pluginID string
	for i := range manifest.Plugins {
		if manifest.Plugins[i].MusicProvider == provider {
			pluginID = manifest.Plugins[i].ID
			break
		}
	}
	if pluginID == "" {
		return "", nil
	}
	inst, err := s.plugs.FindByPluginID(ctx, pluginID)
	if err != nil || inst.State != PluginRunning {
		return "", nil
	}
	return pluginID, nil
}

// StorageProviderPlugins 返回全部图床声明插件中「已安装且 running」的插件 ID（字典序稳定）。
// 供 media.storage seam 自动发现（多图床并存时取首个；管理员可用设置项 media_storage_plugin 显式指定）。
// 清单拉取失败返回空列表（调用方回退静态兜底 image-cdn）。
func (s *PluginService) StorageProviderPlugins(ctx context.Context) []string {
	manifest, err := s.fetchManifest(ctx, "")
	if err != nil {
		return nil
	}
	ids := make([]string, 0, 2)
	for i := range manifest.Plugins {
		if !manifest.Plugins[i].StorageProvider {
			continue
		}
		id := manifest.Plugins[i].ID
		inst, err := s.plugs.FindByPluginID(ctx, id)
		if err != nil || inst.State != PluginRunning {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

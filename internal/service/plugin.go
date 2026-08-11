// internal/service/plugin.go
// 插件服务（M3.1）：插件商城（GitHub 仓库清单驱动）+ 插件管理（安装/启用禁用/卸载）。
// 设计稿《插件商城》《插件安装·免费/付费/Loading/成功》《插件卸载·SEO/成功》。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/roberts9012062/boke/internal/ghclient"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// 默认插件源（用户 GitHub 仓库，plugins.json 清单模拟插件库）。
const defaultPluginSource = "roberts9012062/boke"

// 清单缓存时长（5 分钟，避免每次拉取 GitHub）。
const manifestCacheTTL = 5 * time.Minute

// 插件状态（plugin_instances.state，架构附录 B 状态字典）。
const (
	PluginInstalled  = "installed"  // 已安装（默认）
	PluginRunning    = "running"    // 已启用
	PluginDisabled   = "disabled"   // 已禁用
	PluginUninstalled = "uninstalled" // 已卸载（软删标记）
)

// ---------- 清单模型（GitHub plugins.json 结构） ----------

// PluginManifest 插件清单（仓库根目录 plugins.json）。
type PluginManifest struct {
	Name        string       `json:"name"`        // 插件库名称
	Description string       `json:"description"` // 插件库描述
	Plugins     []PluginInfo `json:"plugins"`     // 插件列表
}

// PluginInfo 插件信息（清单项）。
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
	ID        int64     `json:"id"`         // 实例 ID
	PluginID  string    `json:"plugin_id"`  // 插件 ID
	Name      string    `json:"name"`       // 名称
	Version   string    `json:"version"`    // 版本
	RepoURL   string    `json:"repo_url"`   // 来源仓库
	State     string    `json:"state"`      // 状态
	CreatedAt time.Time `json:"created_at"` // 安装时间
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
	gh     *ghclient.Client   // GitHub 客户端（拉清单）
	plugs  *repository.PluginRepo // 插件实例数据访问
	settings *repository.SettingRepo // 插件源设置
	cache  manifestCache      // 清单缓存（5 分钟）
}

// NewPluginService 创建插件服务。
func NewPluginService(gh *ghclient.Client, plugs *repository.PluginRepo, settings *repository.SettingRepo) *PluginService {
	return &PluginService{gh: gh, plugs: plugs, settings: settings, cache: manifestCache{}}
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
func (s *PluginService) Market(ctx context.Context, source string) (*PluginManifest, []MarketPluginDTO, error) {
	manifest, err := s.fetchManifest(ctx, source)
	if err != nil {
		return nil, nil, err
	}
	// 已安装实例（plugin_id → 实例）
	installed, err := s.plugs.ListInstalled(ctx)
	if err != nil {
		return nil, nil, err
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
	return manifest, items, nil
}

// ---------- 插件管理 ----------

// ListInstalled 已安装插件列表（我的插件页）。
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
			CreatedAt: inst.CreatedAt,
		})
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
	// 重复安装检查（已安装返回冲突；已卸载记录复用重装——plugin_id 唯一约束）
	existing, err := s.plugs.FindByPluginID(ctx, pluginID)
	if err == nil {
		if existing.State != PluginUninstalled {
			return errs.New(errs.CodeConflict, "插件「"+info.Name+"」已安装")
		}
		// 重新安装：复用记录（恢复 running + 更新版本/来源）
		if err := s.plugs.Reinstall(ctx, existing.ID, info.Version, info.RepoURL); err != nil {
			return fmt.Errorf("重新安装插件失败：%w", err)
		}
		return nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	_, err = s.plugs.Create(ctx, repository.PluginInstance{
		PluginID: info.ID,
		Name:     info.Name,
		Version:  info.Version,
		RepoURL:  info.RepoURL,
		State:    PluginRunning,
	})
	if err != nil {
		return fmt.Errorf("安装插件失败：%w", err)
	}
	return nil
}

// SetState 启用/禁用插件（running / disabled）。
func (s *PluginService) SetState(ctx context.Context, instanceID int64, state string) error {
	if state != PluginRunning && state != PluginDisabled {
		return errs.New(errs.CodeBadRequest, "状态仅支持 running / disabled")
	}
	return s.plugs.SetState(ctx, instanceID, state)
}

// Uninstall 卸载插件（删除实例记录）。
func (s *PluginService) Uninstall(ctx context.Context, instanceID int64) error {
	return s.plugs.Delete(ctx, instanceID)
}

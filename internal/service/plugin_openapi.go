// internal/service/plugin_openapi.go
// 插件开放目录聚合器：扫描 data/plugins/{id}/manifest.json 的 open_endpoints 声明，
// 产出「接口开放」目录条目与网关路由索引——插件发版即可上新开放接口，主程序免发版。
//
// 单一事实源 = 插件安装目录内的包清单（安装/升级/卸载随目录文件自然增删）。
// 聚合结果带 TTL 缓存（15s）：安装后短暂延迟可接受，避免在 PluginService 各变更点
// 挂失效钩子；扫描仅为若干小 JSON 文件，重建开销可忽略。
// 护栏：命名空间校验（endpoint 须 {pluginID}. 前缀、path 须泛化路由段前缀）——
// 打包期（cmd/bp）与安装期（ParseManifest）已拦截，此处对存量/手工文件防御性跳过。
package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/plugin"
	"github.com/roberts9012062/boke/internal/plugin/bpkg"
)

// catalogCacheTTL 聚合缓存有效期（安装/卸载后最迟此时延内反映到目录）。
const catalogCacheTTL = 15 * time.Second

// staticCatalogIndex 宿主静态目录标识索引（进程内单次构建；插件条目与之冲突即跳过）。
var staticCatalogIndex = model.CatalogIndex()

// PluginRouteTarget 泛化网关路由命中结果（method+path → 插件端转发目标）。
type PluginRouteTarget struct {
	Endpoint     string         // 接口标识（{pluginID}.xxx）
	PluginID     string         // 目标插件 ID
	PluginPath   string         // 插件端处理路径（如 /private/links）
	PluginMethod string         // 插件端方法（声明归一：缺省=对外方法；对外 GET 可调插件 POST）
	TrustedBody  map[string]any // 受信 body 合并（转发时注入并覆盖外部同名键，防伪造身份字段）
}

// PluginOpenCatalog 插件开放目录聚合器（进程内单例；连接器类）。
type PluginOpenCatalog struct {
	dataDir string        // 数据目录（data/plugins 为其子目录）
	logger  *zap.Logger   // 跳过违规声明时的告警日志（可空）
	mu      sync.Mutex    // 保护缓存三件套
	entries []model.CatalogEntry
	targets map[string]PluginRouteTarget // "GET /api/v1/open/plugins/x/y" → 转发目标
	expire  time.Time
}

// NewPluginOpenCatalog 创建聚合器（dataDir 为宿主数据目录）。
func NewPluginOpenCatalog(dataDir string, logger *zap.Logger) *PluginOpenCatalog {
	return &PluginOpenCatalog{dataDir: dataDir, logger: logger}
}

// Entries 返回插件贡献的目录条目（按插件 ID 与接口标识稳定排序）。
func (a *PluginOpenCatalog) Entries() []model.CatalogEntry {
	a.refresh()
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]model.CatalogEntry, len(a.entries))
	copy(out, a.entries)
	return out
}

// RouteIndex 返回「Method + 完整路径 → 接口标识」索引（ApiKeyAuth 中间件反查用）。
func (a *PluginOpenCatalog) RouteIndex() map[string]string {
	a.refresh()
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make(map[string]string, len(a.targets))
	for k, v := range a.targets {
		out[k] = v.Endpoint
	}
	return out
}

// FindRoute 按 method+完整路径查转发目标（泛化网关 handler 用；未声明返回 false）。
func (a *PluginOpenCatalog) FindRoute(method string, openPath string) (PluginRouteTarget, bool) {
	a.refresh()
	a.mu.Lock()
	defer a.mu.Unlock()
	t, ok := a.targets[method+" "+openPath]
	return t, ok
}

// refresh 重建缓存（TTL 过期才扫描；持锁双检避免并发重复扫描）。
func (a *PluginOpenCatalog) refresh() {
	a.mu.Lock()
	if time.Now().Before(a.expire) {
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()

	entries, targets := a.scan()
	sort.Slice(entries, func(i int, j int) bool {
		if entries[i].PluginName != entries[j].PluginName {
			return entries[i].PluginName < entries[j].PluginName
		}
		return entries[i].Endpoint < entries[j].Endpoint
	})

	a.mu.Lock()
	a.entries = entries
	a.targets = targets
	a.expire = time.Now().Add(catalogCacheTTL)
	a.mu.Unlock()
}

// scan 扫描插件目录清单并聚合（纯读；违规声明跳过并告警）。
func (a *PluginOpenCatalog) scan() ([]model.CatalogEntry, map[string]PluginRouteTarget) {
	entries := make([]model.CatalogEntry, 0)
	targets := make(map[string]PluginRouteTarget)
	root := filepath.Join(a.dataDir, "plugins")
	dirs, err := os.ReadDir(root)
	if err != nil {
		return entries, targets // 目录不存在（未安装任何插件）：空结果
	}
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		pluginID := dir.Name()
		if !plugin.ValidPluginID(pluginID) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, pluginID, "manifest.json"))
		if err != nil {
			continue // 无清单（老版本插件/数据文件）：无开放声明
		}
		// 宽容解析（区别于安装期 ParseManifest 的整体拒绝）：聚合期逐条校验、
		// 违规单条跳过——手工改坏一条不至于废掉该插件全部开放端点；
		// 另校验清单 ID 与目录名一致（防目录名伪装其他插件身份）
		var manifest struct {
			ID            string                  `json:"id"`
			Name          string                  `json:"name"`
			OpenEndpoints []bpkg.OpenEndpointDecl `json:"open_endpoints"`
		}
		if json.Unmarshal(raw, &manifest) != nil || manifest.ID != pluginID || len(manifest.OpenEndpoints) == 0 {
			continue
		}
		for _, decl := range manifest.OpenEndpoints {
			// 护栏一：与宿主静态目录标识冲突即跳过（插件永不伪装/覆盖宿主原生接口——
			// 否则已勾选旧标识的 Key 会被静默扩权到插件泛化路径）
			if _, exists := staticCatalogIndex[decl.Endpoint]; exists {
				if a.logger != nil {
					a.logger.Warn("插件开放端点与宿主目录标识冲突，已跳过",
						zap.String("plugin", pluginID), zap.String("endpoint", decl.Endpoint))
				}
				continue
			}
			// 护栏二：命名空间校验（endpoint 须 {pluginID}. 前缀、path 须泛化路由段前缀）
			if reason := bpkg.ValidateOpenEndpoints(pluginID, []bpkg.OpenEndpointDecl{decl}); reason != "" {
				if a.logger != nil {
					a.logger.Warn("插件开放端点声明违规，已跳过",
						zap.String("plugin", pluginID), zap.String("endpoint", decl.Endpoint), zap.String("reason", reason))
				}
				continue
			}
			entries = append(entries, model.CatalogEntry{
				Endpoint:    decl.Endpoint,
				Method:      decl.Method,
				Path:        decl.Path,
				Name:        decl.Name,
				Description: decl.Description,
				Params:      convertDeclParams(decl.Params),
				Source:      model.CatalogSourcePlugin,
				PluginName:  manifest.Name,
			})
			targets[decl.Method+" "+decl.Path] = PluginRouteTarget{
				Endpoint:     decl.Endpoint,
				PluginID:     pluginID,
				PluginPath:   bpkg.PluginPathFromOpenPath(pluginID, decl.Path),
				PluginMethod: decl.EffectivePluginMethod(),
				TrustedBody:  decl.TrustedBody,
			}
		}
	}
	return entries, targets
}

// convertDeclParams 声明参数 → 目录参数（同构转换；纯函数）。
func convertDeclParams(params []bpkg.OpenEndpointParam) []model.CatalogParam {
	out := make([]model.CatalogParam, 0, len(params))
	for _, p := range params {
		out = append(out, model.CatalogParam{
			Name: p.Name, Type: p.Type, Location: p.Location,
			Required: p.Required, Description: p.Description,
		})
	}
	return out
}

// MarshalOpenEndpoints 供调试：序列化声明（保留 JSON 语义；纯函数）。
func MarshalOpenEndpoints(decls []bpkg.OpenEndpointDecl) string {
	raw, err := json.Marshal(decls)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

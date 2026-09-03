// internal/plugin/bpkg/manifest.go
// .bpk 包内清单（M3.4）：manifest.json 定义与校验。
// 对齐 docs/architecture.md 6.5.6 + docs/plugin-dev-guide.md 10.1：
//   包内 manifest.json 与插件仓库 yueyan-plugin.json、代码 Info() 三处 id/version 必须一致。
package bpkg

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Manifest 包内清单（.bpk 内 manifest.json）。
type Manifest struct {
	ID            string             `json:"id"`                      // 插件 ID（唯一，与安装实例一致）
	Name          string             `json:"name"`                    // 插件名称
	Version       string             `json:"version"`                 // 版本号（与包文件名/Release tag 一致）
	Author        string             `json:"author,omitempty"`        // 作者
	Description   string             `json:"description,omitempty"`   // 一句话描述
	SDK           string             `json:"sdk,omitempty"`           // 兼容 SDK 版本范围（如 ">=1.0.0"；空=不限制）
	Capabilities  []string           `json:"capabilities,omitempty"`  // 能力声明（P0 加固：上传通道校验 + 安装落库供运行时门控取交集）
	OpenEndpoints []OpenEndpointDecl `json:"open_endpoints,omitempty"` // 开放端点声明（声明式接口开放：安装后自动进「接口开放」目录）
}

// OpenEndpointParam 开放端点的参数说明（后台目录展示用，与宿主 CatalogParam 同构）。
type OpenEndpointParam struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Location    string `json:"location"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// OpenEndpointDecl 插件贡献的开放端点声明。
// 命名空间约束（ValidateOpenEndpoints 强制）：
//   - endpoint 必须以 "{pluginID}." 前缀（防插件伪装宿主原生接口标识）；
//   - path 必须以 "/api/v1/open/plugins/{pluginID}/" 开头（泛化网关路由段），
//     插件端处理路径由 path 去掉该前缀推导（如 /private/links）。
type OpenEndpointDecl struct {
	Endpoint      string              `json:"endpoint"`                // 接口标识（{pluginID}.xxx）
	Method        string              `json:"method"`                  // 对外 HTTP 方法（GET / POST）
	PluginMethod  string              `json:"plugin_method,omitempty"` // 插件端方法（缺省=Method；外部语义与插件实现解耦，如对外 GET 调插件 POST）
	Path          string              `json:"path"`                    // 宿主侧完整路径（/api/v1/open/plugins/{id}/...）
	Name          string              `json:"name"`                    // 展示名
	Description   string              `json:"description,omitempty"`   // 一句话描述
	Params        []OpenEndpointParam `json:"params,omitempty"`        // 参数说明
	TrustedBody   map[string]any      `json:"trusted_body,omitempty"`  // 受信 body 合并（网关转发时注入并覆盖外部同名键——插件声明网关凭 Key 调用应携带的身份语义，如 {"admin":true}；覆盖而非合并优先，防外部伪造身份字段）
}

// EffectivePluginMethod 归一插件端方法（空=对外方法；纯函数）。
func (d OpenEndpointDecl) EffectivePluginMethod() string {
	if d.PluginMethod == "GET" || d.PluginMethod == "POST" {
		return d.PluginMethod
	}
	return d.Method
}

// openPluginsPrefix 泛化网关路由前缀（含插件 ID 段；纯函数）。
func openPluginsPrefix(pluginID string) string {
	return "/api/v1/open/plugins/" + pluginID + "/"
}

// PluginPathFromOpenPath 由宿主侧开放路径推导插件端处理路径（纯函数）。
// 前缀不匹配（非本插件命名空间）返回空串。
func PluginPathFromOpenPath(pluginID string, openPath string) string {
	prefix := openPluginsPrefix(pluginID)
	if !strings.HasPrefix(openPath, prefix) {
		return ""
	}
	return "/" + strings.TrimPrefix(openPath, prefix)
}

// ValidateOpenEndpoints 校验开放端点声明的命名空间与格式（纯函数）。
// 返回首个违规原因（空串=全部合法）。
func ValidateOpenEndpoints(pluginID string, decls []OpenEndpointDecl) string {
	prefix := openPluginsPrefix(pluginID)
	for _, d := range decls {
		if !strings.HasPrefix(d.Endpoint, pluginID+".") {
			return fmt.Sprintf("接口标识 %q 必须以 %q 前缀命名", d.Endpoint, pluginID+".")
		}
		if d.Method != "GET" && d.Method != "POST" {
			return fmt.Sprintf("接口 %q 的 method 仅支持 GET/POST", d.Endpoint)
		}
		if d.PluginMethod != "" && d.PluginMethod != "GET" && d.PluginMethod != "POST" {
			return fmt.Sprintf("接口 %q 的 plugin_method 仅支持 GET/POST", d.Endpoint)
		}
		if !strings.HasPrefix(d.Path, prefix) {
			return fmt.Sprintf("接口 %q 的 path 必须以 %q 开头", d.Endpoint, prefix)
		}
		if PluginPathFromOpenPath(pluginID, d.Path) == "" || PluginPathFromOpenPath(pluginID, d.Path) == "/" {
			return fmt.Sprintf("接口 %q 的 path 缺少插件端子路径", d.Endpoint)
		}
		if d.Name == "" {
			return fmt.Sprintf("接口 %q 缺少展示名 name", d.Endpoint)
		}
	}
	return ""
}

// ParseManifest 解析包内 manifest.json。
func ParseManifest(raw []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("manifest.json 解析失败：%w", err)
	}
	if m.ID == "" || m.Name == "" || m.Version == "" {
		return nil, fmt.Errorf("manifest.json 缺少必填字段（id/name/version）")
	}
	if reason := ValidateOpenEndpoints(m.ID, m.OpenEndpoints); reason != "" {
		return nil, fmt.Errorf("开放端点声明不合法：%s", reason)
	}
	return &m, nil
}

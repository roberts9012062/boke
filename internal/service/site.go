// internal/service/site.go
// 站点信息服务（公开接口）：站点元信息从 settings 表实时读取（M1.7 技术债修复，
// 此前 /api/v1/meta 为 router 内硬编码常量）。
package service

import (
	"context"
	"encoding/json"

	"github.com/roberts9012062/boke/internal/repository"
)

// 站点元信息默认值（settings 表缺失或读取失败时兜底，与 seed.sql 一致）。
const (
	defaultSiteName        = "月言"
	defaultSiteDescription = "月言 - 月色微博客：写短句，收声音，偶尔录一点夜色。"
	defaultDefaultTheme    = "cool-moon"
)

// NavLinkDTO 头部导航项（settings.nav_links JSON 数组的元素，支持两级）。
// URL 约定：以 / 开头为站内路径，http(s):// 开头为外部链接（new_tab 控制新窗口打开）；
// 一级项 URL 可为空（纯分组，hover 展开二级下拉，不可点击）；二级项 URL 必填。
type NavLinkDTO struct {
	Label    string        `json:"label"`             // 显示文案（≤30 字符）
	URL      string        `json:"url"`               // 跳转地址（站内路径或外链；一级纯分组可空）
	NewTab   bool          `json:"new_tab"`           // 外链是否新窗口打开
	Children []NavLinkDTO  `json:"children,omitempty"` // 二级菜单（最多两级，二级不可再嵌套）
}

// SiteMetaDTO 站点元信息（GET /api/v1/meta 响应）。
type SiteMetaDTO struct {
	SiteName        string       `json:"site_name"`        // 站点名称
	SiteDescription string       `json:"site_description"` // 站点描述
	DefaultTheme    string       `json:"default_theme"`    // 默认主题（冷月/薄雾）
	MaintenanceMode string       `json:"maintenance_mode"` // 维护开关（on/off，M2 前端拦截用）
	Nav             []NavLinkDTO `json:"nav"`              // 头部导航项（空=前端回退默认导航）
}

// SiteService 站点信息服务（连接器类，仅依赖 settings 仓库）。
type SiteService struct {
	settings *repository.SettingRepo // 站点设置数据访问
}

// NewSiteService 创建站点信息服务。
func NewSiteService(settings *repository.SettingRepo) *SiteService {
	return &SiteService{settings: settings}
}

// Meta 读取站点元信息（settings 表实时读取；读取失败时回退默认值，保证服务可用性）。
func (s *SiteService) Meta(ctx context.Context) SiteMetaDTO {
	all, err := s.settings.All(ctx)
	if err != nil {
		// 读取失败（如数据库抖动）：回退默认值，避免公开接口报错
		return SiteMetaDTO{
			SiteName:        defaultSiteName,
			SiteDescription: defaultSiteDescription,
			DefaultTheme:    defaultDefaultTheme,
		}
	}
	return SiteMetaDTO{
		SiteName:        valueOr(all, "site_name", defaultSiteName),
		SiteDescription: valueOr(all, "site_description", defaultSiteDescription),
		DefaultTheme:    valueOr(all, "theme", defaultDefaultTheme),
		MaintenanceMode: maintenanceValue(all),
		Nav:             ParseNavLinks(all["nav_links"]),
	}
}

// ParseNavLinks 解析导航配置（settings.nav_links JSON 数组文本）。
// 解析失败或未配置时返回 nil（前台回退默认导航「首页/话题」，保证头部始终可用）。
func ParseNavLinks(raw string) []NavLinkDTO {
	if raw == "" {
		return nil
	}
	var links []NavLinkDTO
	if err := json.Unmarshal([]byte(raw), &links); err != nil {
		return nil
	}
	return links
}

// maintenanceValue 维护开关值（on/off，缺失默认 off）。
func maintenanceValue(all map[string]string) string {
	if v, ok := all["maintenance_mode"]; ok && v == "on" {
		return "on"
	}
	return "off"
}

// MaintenanceMode 判断全站维护开关是否开启（M2 中间件拦截用，直接查库实时生效）。
func (s *SiteService) MaintenanceMode(ctx context.Context) bool {
	value, ok, err := s.settings.Get(ctx, "maintenance_mode")
	if err != nil || !ok {
		return false
	}
	return value == "on"
}

// valueOr 从设置映射取值，缺失时返回默认值。
func valueOr(all map[string]string, key string, fallback string) string {
	if v, ok := all[key]; ok && v != "" {
		return v
	}
	return fallback
}

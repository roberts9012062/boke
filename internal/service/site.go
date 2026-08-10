// internal/service/site.go
// 站点信息服务（公开接口）：站点元信息从 settings 表实时读取（M1.7 技术债修复，
// 此前 /api/v1/meta 为 router 内硬编码常量）。
package service

import (
	"context"

	"github.com/yueyan/boke/internal/repository"
)

// 站点元信息默认值（settings 表缺失或读取失败时兜底，与 seed.sql 一致）。
const (
	defaultSiteName        = "月言"
	defaultSiteDescription = "月言 - 月色微博客：写短句，收声音，偶尔录一点夜色。"
	defaultDefaultTheme    = "cool-moon"
)

// SiteMetaDTO 站点元信息（GET /api/v1/meta 响应）。
type SiteMetaDTO struct {
	SiteName        string `json:"site_name"`        // 站点名称
	SiteDescription string `json:"site_description"` // 站点描述
	DefaultTheme    string `json:"default_theme"`    // 默认主题（冷月/薄雾）
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
	}
}

// valueOr 从设置映射取值，缺失时返回默认值。
func valueOr(all map[string]string, key string, fallback string) string {
	if v, ok := all[key]; ok && v != "" {
		return v
	}
	return fallback
}

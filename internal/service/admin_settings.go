// internal/service/admin_settings.go
// 站点设置读写（后台设置页）：白名单保存 + 敏感键掩码回显（P2 加固）。
// 从 admin.go 拆出（文件行数硬性指标）。
package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/roberts9012062/boke/pkg/errs"
)

// ---------- 站点设置 ----------

// sensitiveSettingKeys 敏感设置键（P2 加固：密文/凭证类，后台读取一律掩码，不回显原值）。
var sensitiveSettingKeys = map[string]bool{
	"gh_oauth_token":             true, // GitHub OAuth token（AES 密文）
	"plugin_license_private_key": true, // 插件许可证签发私钥（AES 密文）
}

// secretMask 敏感值掩码（回显占位；保存时收到掩码原样值的键跳过不覆盖）。
const secretMask = "__SECRET__"

// maxNavLinks 头部导航项数量上限（一级项数量；头部空间有限，防配置滥用）。
const maxNavLinks = 20

// maxNavChildren 单个一级项的二级菜单数量上限。
const maxNavChildren = 10

// Settings 站点设置读取（P2 加固：敏感键以掩码回显——密文外发扩大离线爆破面）。
func (s *AdminService) Settings(ctx context.Context) (map[string]string, error) {
	all, err := s.settings.All(ctx)
	if err != nil {
		return nil, err
	}
	for key, value := range all {
		if sensitiveSettingKeys[key] && value != "" {
			all[key] = secretMask
		}
	}
	return all, nil
}

// SaveSettings 站点设置保存（站点名/描述/注册开关/评论开关/默认主题/维护开关/头部导航）。
// nav_links 为 JSON 数组文本：不能走 SetMany（其值拼接方式遇内部引号会产生非法 JSONB），
// 校验后分流到 SetJSON 单独保存。
func (s *AdminService) SaveSettings(ctx context.Context, updates map[string]string) error {
	// 白名单校验（防注入任意键；plugin_ 前缀为插件配置键，M3.2 schema 驱动设置页）
	allowed := map[string]bool{
		"site_name": true, "site_description": true,
		"allow_register": true, "comment_open": true,
		"theme": true, "maintenance_mode": true, // 维护开关（M2）
		"plugin_source": true, // 插件源仓库（M3.1，owner/repo）
		"plugin_proxy":  true, // GitHub 加速代理（国内网络直连失败时选择，空=直连）
		"media_storage_plugin": true, // 图床接管插件（media.storage seam 提供方；空=自动发现，值=插件 ID）
		"nav_links": true, // 头部导航配置（JSON 数组，前台头部导航自定义）
	}
	filtered := make(map[string]string, len(updates))
	navRaw, hasNav := "", false // nav_links 原文（SetJSON 单独保存）
	for key, value := range updates {
		if !allowed[key] && !strings.HasPrefix(key, "plugin_") {
			return errs.New(errs.CodeBadRequest, "不支持的设置项："+key)
		}
		// 掩码原样回传的敏感键跳过（前端整表单回存时不覆盖真实密文）
		if sensitiveSettingKeys[key] && value == secretMask {
			continue
		}
		if key == "nav_links" {
			// 导航配置格式校验（JSON 数组 + 每项 label/url 规则）
			if err := ValidateNavLinks(value); err != nil {
				return err
			}
			navRaw, hasNav = value, true
			continue
		}
		filtered[key] = value
	}
	if hasNav {
		// 空串（清空配置、恢复默认导航）以空数组落库：空字符串不是合法 JSONB，
		// 直接绑定会让 $2::jsonb 转换失败（历史 bug：清理导航报 500）
		stored := navRaw
		if strings.TrimSpace(stored) == "" {
			stored = "[]"
		}
		if err := s.settings.SetJSON(ctx, "nav_links", stored); err != nil {
			return err
		}
	}
	return s.settings.SetMany(ctx, filtered)
}

// ValidateNavLinks 校验导航配置格式（后台保存前，两级结构递归校验）。
// 规则：空串=清空配置（回退默认导航）；JSON 数组一级项 ≤20；每项 label 非空 ≤30 字符；
// 一级项 URL 可空（纯分组，hover 展开二级）或以 / 开头（站内）或 http(s):// 开头（外链）；
// 二级项 URL 必填且同样走协议白名单；二级不可再嵌套（最多两级）；单项二级 ≤10。
func ValidateNavLinks(raw string) error {
	if raw == "" {
		return nil
	}
	var links []NavLinkDTO
	if err := json.Unmarshal([]byte(raw), &links); err != nil {
		return errs.New(errs.CodeValidation, "导航配置格式不正确（须为 JSON 数组）")
	}
	if len(links) > maxNavLinks {
		return errs.New(errs.CodeValidation, "一级导航项最多 20 个")
	}
	for _, link := range links {
		if err := validateNavNode(link, true); err != nil {
			return err
		}
	}
	return nil
}

// validateNavNode 校验单个导航节点（纯函数递归；top=一级项，二级项递归时 top=false）。
func validateNavNode(link NavLinkDTO, top bool) error {
	label := strings.TrimSpace(link.Label)
	if label == "" || len(label) > 30 {
		return errs.New(errs.CodeValidation, "导航名称不能为空，且不超过 30 字符")
	}
	// URL：一级项可空（纯分组）；二级项必填；非空时一律走协议白名单
	if top {
		if link.URL != "" && !isNavURLAllowed(link.URL) {
			return errs.New(errs.CodeValidation, "导航地址须以 / 开头（站内）或 http(s):// 开头（外链）："+label)
		}
	} else {
		if !isNavURLAllowed(link.URL) {
			return errs.New(errs.CodeValidation, "二级导航地址不能为空，且须以 / 开头（站内）或 http(s):// 开头（外链）："+label)
		}
		if len(link.Children) > 0 {
			return errs.New(errs.CodeValidation, "导航最多两级，不支持更深层级："+label)
		}
	}
	if len(link.Children) > maxNavChildren {
		return errs.New(errs.CodeValidation, "单个一级导航的二级菜单最多 10 个："+label)
	}
	for _, child := range link.Children {
		if err := validateNavNode(child, false); err != nil {
			return err
		}
	}
	return nil
}

// isNavURLAllowed 导航地址白名单校验（纯函数）：站内路径或 http(s) 外链。
// 防 javascript: 等危险协议注入头部导航；"//" 开头的协议相对地址视同外站，拒绝。
func isNavURLAllowed(url string) bool {
	if url == "" || len(url) > 500 || strings.HasPrefix(url, "//") {
		return false
	}
	return strings.HasPrefix(url, "/") ||
		strings.HasPrefix(url, "http://") ||
		strings.HasPrefix(url, "https://")
}

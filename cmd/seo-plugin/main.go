// cmd/seo-plugin/main.go
// SEO 优化插件（M4.1 插件化：发帖 SEO 面板 + 校验钩子）。
// 能力划分：
//   - 本插件（进程外）：compose.seo 槽位渲染发帖 SEO 表单（前端扩展）+ post.before_publish
//     校验钩子（SEO 标题/描述/别名格式）
//   - 主进程通道：seo 字段随发帖落库 seo_meta、/p/{alias} 短链、收录策略输出（卸载后通道闲置）
// 验证链路：上传安装 → 发帖页出现 SEO 面板 → 填写发帖 → seo_meta 落库 → /p/{alias} 可访问
//           → 卸载 → 面板消失（功能还原）。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"unicode/utf8"

	"github.com/roberts9012062/boke/pkg/plugin-sdk"
	"github.com/roberts9012062/boke/pkg/plugin-sdk/server"
)

// SEO 优化插件实现（进程外）。
type SeoPlugin struct{}

// seoPayload 发帖请求中的 SEO 输入（与主进程 CreatePostReq.Seo 对齐）。
type seoPayload struct {
	SEOTitle       string `json:"seo_title"`
	SEODescription string `json:"seo_description"`
	URLAlias       string `json:"url_alias"`
	Robots         string `json:"robots"`
}

// createPayload 发帖请求载荷（仅解析 SEO 相关字段）。
type createPayload struct {
	Seo *seoPayload `json:"seo"`
}

// 别名格式（小写字母数字连字符，1-64 字符）。
var aliasPattern = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

// Info 插件信息（与商城清单一致；能力声明 + 设置项）。
func (p *SeoPlugin) Info() sdk.Info {
	return sdk.Info{
		ID:          "seo-optimizer",
		Name:        "SEO 优化",
		Version:     "1.2.0",
		Author:      "月言官方",
		Description: "发帖 SEO 面板（标题/描述/别名/收录）+ 元信息校验，一键提升收录。",
		Capabilities: []string{"hooks", "frontend", "settings"},
		Settings: []sdk.SettingField{
			{Key: "site_title_suffix", Label: "站点标题后缀", Type: "text", Default: "· 月言"},
			{Key: "auto_sitemap", Label: "自动生成 sitemap", Type: "switch", Default: "on"},
			{Key: "og_image", Label: "默认 OG 图", Type: "text", Default: ""},
		},
	}
}

// OnActivate 启用回调。
func (p *SeoPlugin) OnActivate(ctx context.Context) error { return nil }

// OnDeactivate 停用回调。
func (p *SeoPlugin) OnDeactivate(ctx context.Context) error { return nil }

// Hooks 订阅钩子：发帖前校验 SEO 输入（格式/长度，非法拒绝）。
func (p *SeoPlugin) Hooks() []sdk.Hook {
	return []sdk.Hook{
		{
			Name: "post.before_publish", Sync: true, Priority: 100,
			Handler: func(ctx context.Context, ev sdk.Event) (sdk.Result, error) {
				var payload createPayload
				if err := json.Unmarshal(ev.Payload, &payload); err != nil || payload.Seo == nil {
					return sdk.Result{OK: true}, nil // 无 SEO 输入：放行（不拦截普通发帖）
				}
				seo := payload.Seo
				// SEO 标题 ≤60 字（画板：标题 12/60）
				if utf8.RuneCountInString(seo.SEOTitle) > 60 {
					return sdk.Result{OK: false, Reason: "SEO 标题不能超过 60 字"}, nil
				}
				// SEO 描述 ≤160 字（画板：描述 18/160）
				if utf8.RuneCountInString(seo.SEODescription) > 160 {
					return sdk.Result{OK: false, Reason: "SEO 描述不能超过 160 字"}, nil
				}
				// URL 别名格式（小写字母数字连字符）
				if seo.URLAlias != "" && !aliasPattern.MatchString(seo.URLAlias) {
					return sdk.Result{OK: false, Reason: "URL 别名仅支持小写字母、数字与连字符（≤64 字符）"}, nil
				}
				return sdk.Result{OK: true}, nil
			},
		},
	}
}

// main 插件进程入口。
func main() {
	fmt.Fprintln(os.Stderr, "[seo-plugin] 进程启动")
	server.Serve(&SeoPlugin{})
}

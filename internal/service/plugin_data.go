// internal/service/plugin_data.go
// 插件只读数据服务实现（M3.8 能力授权 data.read）：
//   PluginDataProvider 实现 plugin.DataProvider 接口，主进程经 MuxBroker 注册后
//   授权插件查询脱敏数据（用户/帖子/站点公开设置）。
//   安全：只读 + 脱敏（不含邮箱/正文/密钥）；写操作仍走钩子与主进程 API。
package service

import (
	"context"
	"time"

	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/plugin-sdk/contract"
)

// 数据服务设置白名单键（站点公开信息；密钥类设置不入白名单）。
var dataSettingsWhitelist = map[string]bool{
	"site_name":         true, // 站点名称
	"site_description":  true, // 站点描述
	"site_keywords":     true, // 默认关键词
	"site_logo":         true, // 站点 Logo
	"site_icp":          true, // ICP 备案号
	"site_announcement": true, // 站点公告
}

// PluginDataProvider 插件只读数据服务（连接器类；独立于 PluginService，避免参数膨胀）。
type PluginDataProvider struct {
	users       *repository.UserRepo        // 用户查询
	posts       *repository.PostRepo        // 帖子查询
	settings    *repository.SettingRepo     // 站点设置
	aiSvc       *AiService                  // AI 服务（M4.1 插件 AI 辅助：模型列表/文本生成）
	openAPIKeys *repository.OpenAPIKeyRepo  // 开放接口凭证（浏览器插件联动：Key 读取）
}

// NewPluginDataProvider 创建数据服务提供者。
func NewPluginDataProvider(users *repository.UserRepo, posts *repository.PostRepo, settings *repository.SettingRepo, aiSvc *AiService, openAPIKeys *repository.OpenAPIKeyRepo) *PluginDataProvider {
	return &PluginDataProvider{users: users, posts: posts, settings: settings, aiSvc: aiSvc, openAPIKeys: openAPIKeys}
}

// GetUser 查询用户脱敏信息（数据服务；不存在返回空结构——插件按 nil 场景兜底）。
func (p *PluginDataProvider) GetUser(ctx context.Context, userID int64) (*contract.UserInfo, error) {
	user, err := p.users.FindByID(ctx, userID)
	if err != nil {
		return &contract.UserInfo{ID: userID}, nil // 不存在/查询失败：返回占位（不向插件暴露错误细节）
	}
	role, _ := p.users.FindRoleByID(ctx, userID) // 角色查询失败按空处理（脱敏优先）
	return &contract.UserInfo{
		ID: user.ID, Nickname: user.Nickname,
		AvatarURL: user.AvatarURL, Role: role, Bio: user.Bio,
	}, nil
}

// GetPost 查询帖子脱敏信息（数据服务；不含正文全文）。
func (p *PluginDataProvider) GetPost(ctx context.Context, postID int64) (*contract.PostInfo, error) {
	post, err := p.posts.FindByID(ctx, postID)
	if err != nil {
		return &contract.PostInfo{ID: postID}, nil // 不存在/查询失败：返回占位
	}
	authorName := ""
	if user, err := p.users.FindByID(ctx, post.AuthorID); err == nil {
		authorName = user.Nickname
	}
	return &contract.PostInfo{
		ID: post.ID, Title: post.Title, Status: post.Status,
		AuthorID: post.AuthorID, AuthorName: authorName,
	}, nil
}

// GetSettings 查询站点公开设置（白名单键；不暴露 AI 密钥/插件源等敏感键）。
func (p *PluginDataProvider) GetSettings(ctx context.Context) (*contract.SettingsSnapshot, error) {
	all, err := p.settings.All(ctx)
	if err != nil {
		return &contract.SettingsSnapshot{Values: map[string]string{}}, nil
	}
	values := make(map[string]string, len(all))
	for key, value := range all {
		if dataSettingsWhitelist[key] {
			values[key] = value
		}
	}
	return &contract.SettingsSnapshot{Values: values}, nil
}

// GetAIModels 查询可用 AI 模型（M4.1 插件 AI 辅助：脱敏——仅启用供应商名 + 模型列表）。
func (p *PluginDataProvider) GetAIModels(ctx context.Context) (*contract.AIModelList, error) {
	if p.aiSvc == nil {
		return &contract.AIModelList{Models: []contract.AIModel{}}, nil
	}
	items, err := p.aiSvc.AIModels(ctx)
	if err != nil {
		return &contract.AIModelList{Models: []contract.AIModel{}}, nil // 查询失败按无模型（面板提示配置）
	}
	models := make([]contract.AIModel, 0, len(items))
	for _, pv := range items {
		models = append(models, contract.AIModel{Name: pv.Name, Models: pv.Models})
	}
	return &contract.AIModelList{Models: models}, nil
}

// GenerateAI 调用主进程 AI 生成文本（M4.1 插件 AI 辅助：按模型路由供应商）。
func (p *PluginDataProvider) GenerateAI(ctx context.Context, model string, prompt string, content string) (*contract.GenerateResult, error) {
	if p.aiSvc == nil {
		return &contract.GenerateResult{Text: ""}, nil
	}
	text, err := p.aiSvc.Generate(ctx, model, prompt, content)
	if err != nil {
		return &contract.GenerateResult{Text: ""}, err
	}
	return &contract.GenerateResult{Text: text}, nil
}

// formatContractTime 契约时间格式化（nil → 空串；纯函数）。
func formatContractTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// GetOpenAPIKeys 查询开放接口 API Key 清单（含明文 Key；查询失败返回空列表——不向插件暴露错误细节）。
// 场景：插件与浏览器插件联动——读取 Key 远传给浏览器插件，凭 X-Api-Key 调用 /api/v1/open/* 验证重要接口。
func (p *PluginDataProvider) GetOpenAPIKeys(ctx context.Context) (*contract.OpenAPIKeyList, error) {
	if p.openAPIKeys == nil {
		return &contract.OpenAPIKeyList{Keys: []contract.OpenAPIKeyInfo{}}, nil
	}
	keys, err := p.openAPIKeys.List(ctx)
	if err != nil {
		return &contract.OpenAPIKeyList{Keys: []contract.OpenAPIKeyInfo{}}, nil
	}
	infos := make([]contract.OpenAPIKeyInfo, 0, len(keys))
	for _, k := range keys {
		infos = append(infos, contract.OpenAPIKeyInfo{
			ID:         k.ID,
			Name:       k.Name,
			Key:        k.Key,
			Endpoints:  k.Endpoints,
			ExpiresAt:  formatContractTime(k.ExpiresAt),
			LastUsedAt: formatContractTime(k.LastUsedAt),
			CreatedAt:  formatContractTime(&k.CreatedAt),
		})
	}
	return &contract.OpenAPIKeyList{Keys: infos}, nil
}

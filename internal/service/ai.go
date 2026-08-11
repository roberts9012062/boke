// internal/service/ai.go
// AI 业务（M4）：供应商管理 / 任务配置 / 用量统计。
//
// 设计：连接器类，注入 AI 域三个仓库 + AI 调用所需辅助仓库；
//       场景执行（摘要/标签/评论审核）见 ai_scenes.go（统一 runTask 流程）。
package service

import (
	"context"
	"strings"
	"time"

	"github.com/roberts9012062/boke/internal/ai"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// AI 常量（M4）。
const (
	aiSystemUserID   = 1          // 系统账号 ID（admin 种子账号；AI 标记工单的举报人）
	aiRequestTimeout = 60         // AI 请求超时（秒）
	aiKeyMask        = "sk-***"   // API Key 掩码回显
	maxPromptLen     = 4000       // 场景输入文本上限（字符，控制 token 成本）
	aiMaxSummaryLen  = 300        // AI 摘要上限（字符）
	aiMaxTagCount    = 5          // AI 标签上限（个）
)

// AiProviderDTO 供应商 DTO（后台列表；API Key 不回显明文，仅掩码标记）。
type AiProviderDTO struct {
	ID        int64    `json:"id"`         // 供应商 ID
	Name      string   `json:"name"`       // 名称
	BaseURL   string   `json:"base_url"`   // 接口地址
	APIKeySet bool     `json:"api_key_set"` // 是否已配置 API Key
	Models    []string `json:"models"`     // 模型列表
	Enabled   bool     `json:"enabled"`    // 是否启用
	Priority  int      `json:"priority"`   // 路由优先级
}

// AiProviderInput 供应商新增/编辑输入（编辑时 api_key 留空 = 保持原值）。
type AiProviderInput struct {
	Name     string   `json:"name"`     // 名称（必填）
	BaseURL  string   `json:"base_url"` // 接口地址（必填）
	APIKey   string   `json:"api_key"`  // API Key（新增必填；编辑可留空）
	Models   []string `json:"models"`   // 模型列表（必填至少 1 个）
	Enabled  bool     `json:"enabled"`  // 是否启用
	Priority int      `json:"priority"` // 路由优先级（1-100）
}

// AiTaskDTO 任务 DTO（后台任务配置列表）。
type AiTaskDTO struct {
	TaskName       string  `json:"task_name"`       // 任务名
	ProviderID     *int64  `json:"provider_id"`     // 绑定供应商（NULL = 自动路由）
	ProviderName   string  `json:"provider_name"`   // 供应商名（List JOIN 回填）
	Model          string  `json:"model"`           // 模型名（空 = 供应商默认）
	PromptTemplate string  `json:"prompt_template"` // 提示词模板
	MaxTokens      int     `json:"max_tokens"`      // 最大输出 token
	Enabled        bool    `json:"enabled"`         // 是否启用
}

// AiTaskInput 任务配置更新输入。
type AiTaskInput struct {
	ProviderID     *int64 `json:"provider_id"`     // 绑定供应商（NULL = 自动路由）
	Model          string `json:"model"`           // 模型名
	PromptTemplate string `json:"prompt_template"` // 提示词模板
	MaxTokens      int    `json:"max_tokens"`      // 最大输出 token
}

// AiUsageDTO 用量统计（汇总 + 近 7 日明细，图表直用）。
type AiUsageDTO struct {
	Summary *repository.AiUsageSummary `json:"summary"` // 汇总（今日/累计）
	Days    []repository.AiDayStat     `json:"days"`    // 近 7 日按日聚合
}

// AiService AI 服务（连接器类）。
type AiService struct {
	providers *repository.AiProviderRepo // 供应商
	tasks     *repository.AiTaskRepo     // 任务路由
	usage     *repository.AiUsageRepo    // 用量统计
	seo       *repository.SeoRepo        // SEO 元数据（摘要落库）
	posts     *repository.PostRepo       // 帖子（场景输入）
	comments  *repository.CommentRepo    // 评论（审核场景）
	reports   *repository.ReportRepo     // 举报工单（高风险标记）
	keySecret string                     // API Key 加密密钥（config.AIKeySecret）
}

// NewAiService 创建 AI 服务。
func NewAiService(
	providers *repository.AiProviderRepo,
	tasks *repository.AiTaskRepo,
	usage *repository.AiUsageRepo,
	seo *repository.SeoRepo,
	posts *repository.PostRepo,
	comments *repository.CommentRepo,
	reports *repository.ReportRepo,
	keySecret string,
) *AiService {
	return &AiService{providers: providers, tasks: tasks, usage: usage, seo: seo, posts: posts, comments: comments, reports: reports, keySecret: keySecret}
}

// ---------- 供应商管理 ----------

// ListProviders 供应商列表（API Key 仅掩码标记，不回显明文）。
func (s *AiService) ListProviders(ctx context.Context) ([]AiProviderDTO, error) {
	providers, err := s.providers.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]AiProviderDTO, 0, len(providers))
	for _, p := range providers {
		items = append(items, AiProviderDTO{
			ID: p.ID, Name: p.Name, BaseURL: p.BaseURL,
			APIKeySet: p.APIKeyEncrypted != "", Models: p.Models,
			Enabled: p.Enabled, Priority: p.Priority,
		})
	}
	return items, nil
}

// CreateProvider 新增供应商（API Key AES 加密后存储）。
func (s *AiService) CreateProvider(ctx context.Context, input AiProviderInput) (int64, error) {
	if err := validateProviderInput(input, true); err != nil {
		return 0, err
	}
	encrypted, err := ai.EncryptSecret(input.APIKey, s.keySecret)
	if err != nil {
		return 0, errs.New(errs.CodeInternal, "API Key 加密失败")
	}
	return s.providers.Create(ctx, repository.AiProvider{
		Name: input.Name, BaseURL: input.BaseURL,
		APIKeyEncrypted: encrypted, Models: input.Models,
		Enabled: input.Enabled, Priority: input.Priority,
	})
}

// UpdateProvider 更新供应商（API Key 留空 = 保持原值，支持只改配置）。
func (s *AiService) UpdateProvider(ctx context.Context, id int64, input AiProviderInput) error {
	exist, found, err := s.providers.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return errs.ErrNotFound
	}
	if err := validateProviderInput(input, false); err != nil {
		return err
	}
	// 加密新 Key（留空时传空串，仓库层保持原值不动）
	encrypted := ""
	if strings.TrimSpace(input.APIKey) != "" {
		encrypted, err = ai.EncryptSecret(input.APIKey, s.keySecret)
		if err != nil {
			return errs.New(errs.CodeInternal, "API Key 加密失败")
		}
	}
	// 未传 Key 时保留原密文（COALESCE 分支依赖仓库层空串判断）
	_ = exist
	return s.providers.Update(ctx, repository.AiProvider{
		ID: id, Name: input.Name, BaseURL: input.BaseURL,
		APIKeyEncrypted: encrypted, Models: input.Models,
		Enabled: input.Enabled, Priority: input.Priority,
	})
}

// DeleteProvider 删除供应商（任务引用自动置空）。
func (s *AiService) DeleteProvider(ctx context.Context, id int64) error {
	return s.providers.Delete(ctx, id)
}

// TestProvider 测试供应商连通性（最小请求验证 API Key 与接口）。
// 返回：成功输出（如「连通正常」）；失败返回带原因的错误。
func (s *AiService) TestProvider(ctx context.Context, id int64) error {
	provider, found, err := s.providers.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return errs.ErrNotFound
	}
	apiKey, err := decryptAPIKey(provider.APIKeyEncrypted, s.keySecret)
	if err != nil {
		return errs.New(errs.CodeUpstream, "API Key 解密失败，请重新保存")
	}
	if apiKey == "" {
		return errs.New(errs.CodeBadRequest, "请先配置该供应商的 API Key")
	}
	model := firstModel(provider.Models)
	if model == "" {
		return errs.New(errs.CodeBadRequest, "该供应商未配置模型")
	}
	client := ai.NewClient(provider.BaseURL, apiKey, model, aiRequestTimeout*time.Second)
	_, err = client.Chat(ctx, "你是一个连通性测试助手，只回复「OK」。", "ping", 8)
	if err != nil {
		return errs.New(errs.CodeUpstream, "连接失败："+err.Error())
	}
	return nil
}

// validateProviderInput 供应商输入校验（新增必填 Key，编辑可留空）。
func validateProviderInput(input AiProviderInput, requireKey bool) error {
	if strings.TrimSpace(input.Name) == "" || len([]rune(input.Name)) > 50 {
		return errs.New(errs.CodeBadRequest, "供应商名称需为 1-50 字符")
	}
	if !strings.HasPrefix(input.BaseURL, "http://") && !strings.HasPrefix(input.BaseURL, "https://") {
		return errs.New(errs.CodeBadRequest, "接口地址需以 http(s):// 开头")
	}
	if requireKey && strings.TrimSpace(input.APIKey) == "" {
		return errs.New(errs.CodeBadRequest, "API Key 必填")
	}
	if len(input.Models) == 0 {
		return errs.New(errs.CodeBadRequest, "请至少配置一个模型")
	}
	if input.Priority < 1 || input.Priority > 100 {
		return errs.New(errs.CodeBadRequest, "路由优先级需为 1-100")
	}
	return nil
}

// ---------- 任务配置 ----------

// ListTasks 任务列表（含供应商名，JOIN 回填）。
func (s *AiService) ListTasks(ctx context.Context) ([]AiTaskDTO, error) {
	tasks, err := s.tasks.List(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]AiTaskDTO, 0, len(tasks))
	for _, t := range tasks {
		items = append(items, AiTaskDTO{
			TaskName: t.TaskName, ProviderID: t.ProviderID, ProviderName: t.ProviderName,
			Model: t.Model, PromptTemplate: t.PromptTemplate,
			MaxTokens: t.MaxTokens, Enabled: t.Enabled,
		})
	}
	return items, nil
}

// UpdateTask 更新任务配置（模型/提示词/最大 token/绑定供应商）。
func (s *AiService) UpdateTask(ctx context.Context, taskName string, input AiTaskInput) error {
	if input.MaxTokens < 1 || input.MaxTokens > 8192 {
		return errs.New(errs.CodeBadRequest, "最大输出 token 需为 1-8192")
	}
	if strings.TrimSpace(input.PromptTemplate) == "" {
		return errs.New(errs.CodeBadRequest, "提示词模板不能为空")
	}
	return s.tasks.Update(ctx, repository.AiTask{
		TaskName: taskName, ProviderID: input.ProviderID,
		Model: strings.TrimSpace(input.Model), PromptTemplate: input.PromptTemplate,
		MaxTokens: input.MaxTokens,
	})
}

// SetTaskEnabled 启停任务。
func (s *AiService) SetTaskEnabled(ctx context.Context, taskName string, enabled bool) error {
	return s.tasks.SetEnabled(ctx, taskName, enabled)
}

// ---------- 用量统计 ----------

// UsageStats 用量统计（汇总 + 近 7 日按日聚合）。
func (s *AiService) UsageStats(ctx context.Context) (*AiUsageDTO, error) {
	summary, err := s.usage.Summary(ctx)
	if err != nil {
		return nil, err
	}
	days, err := s.usage.StatsByDay(ctx, 7)
	if err != nil {
		return nil, err
	}
	return &AiUsageDTO{Summary: summary, Days: days}, nil
}

// ---------- 内部辅助 ----------

// decryptAPIKey 解密供应商 API Key（空密文返回空串，纯函数）。
func decryptAPIKey(encrypted string, keySecret string) (string, error) {
	if encrypted == "" {
		return "", nil
	}
	return ai.DecryptSecret(encrypted, keySecret)
}

// firstModel 取供应商模型列表首个（纯函数；空列表返回空串）。
func firstModel(models []string) string {
	if len(models) == 0 {
		return ""
	}
	return models[0]
}

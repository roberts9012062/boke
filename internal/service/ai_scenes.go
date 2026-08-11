// internal/service/ai_scenes.go
// AI 内置场景（M4）：帖子摘要 / 自动标签 / 评论审核。
//
// 统一执行流程（runTask）：查任务配置 → 校验启用 → 路由供应商（任务绑定或按优先级）
// → 解密 API Key → 调用 OpenAI 兼容接口 → 用量落库 → 解析输出并落库。
// 设计：AI 输出均为结构化 JSON，解析采用容错策略（失败不误伤业务主流程）。
package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/roberts9012062/boke/internal/ai"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// 任务名常量（与迁移 010 种子一致）。
const (
	TaskPostSummary  = "post.summary"  // 帖子摘要
	TaskPostTags     = "post.tags"     // 自动标签
	TaskCommentReview = "comment.review" // 评论审核
)

// AI 高风险评论工单原因（审核队列展示）。
const aiReportReason = "AI 高风险审核"

// GenSummary 生成帖子摘要（写入 seo_meta.summary，M4-AI 场景 1）。
// 返回：AI 生成的摘要文本。
func (s *AiService) GenSummary(ctx context.Context, postID int64) (string, error) {
	post, err := s.posts.FindByID(ctx, postID)
	if err != nil {
		return "", err
	}
	result, err := s.runTask(ctx, TaskPostSummary, buildPostInput(post.Title, post.Content))
	if err != nil {
		return "", err
	}
	summary := truncateRunes(result.Text, aiMaxSummaryLen)
	if err := s.seo.UpdateSummary(ctx, postID, summary); err != nil {
		return "", err
	}
	return summary, nil
}

// GenTags 生成标签建议（M4-AI 场景 2）。
// 返回：AI 建议标签数组（3-5 个；前端确认后由帖子更新接口写入，AI 不直接改帖）。
func (s *AiService) GenTags(ctx context.Context, postID int64) ([]string, error) {
	post, err := s.posts.FindByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	result, err := s.runTask(ctx, TaskPostTags, buildPostInput(post.Title, post.Content))
	if err != nil {
		return nil, err
	}
	return parseTagArray(result.Text)
}

// ReviewComment 审核单条评论（M4-AI 场景 3：异步预审 + 手动批量共用）。
// 规则：AI 判定 risk=high → 评论隐藏 + 写入审核队列（source=ai 工单，待处理+1）；
//       人工复核见 ModerationService.VerdictReport（放行/删除）。
// 说明：解析失败/调用失败静默返回 nil（预审是增强能力，不影响评论主流程）。
func (s *AiService) ReviewComment(ctx context.Context, commentID int64) error {
	comment, err := s.comments.FindByID(ctx, commentID)
	if err != nil {
		return err
	}
	// 已删除/已隐藏的评论跳过（避免重复工单）
	if comment.Status != "visible" {
		return nil
	}
	result, err := s.runTask(ctx, TaskCommentReview, "评论内容："+truncateRunes(comment.Content, 2000))
	if err != nil {
		return err
	}
	verdict, ok := parseReviewVerdict(result.Text)
	if !ok || verdict.Risk != "high" {
		return nil // 低风险或解析失败：放行
	}
	// 高风险：隐藏评论 + 写 AI 来源工单（待人工复核）
	if err := s.comments.SetStatus(ctx, commentID, "hidden"); err != nil {
		return err
	}
	_, err = s.reports.Create(ctx, repository.Report{
		ReporterID: aiSystemUserID, TargetType: "comment", TargetID: commentID,
		Reason: aiReportReason, Detail: verdict.Reason, Source: repository.ReportSourceAI,
	})
	return err
}

// ReviewComments 批量审核评论（后台手动兜底；逐条独立，单条失败不阻断其余）。
// 返回：处理结果说明（成功数/失败数）。
func (s *AiService) ReviewComments(ctx context.Context, commentIDs []int64) (map[string]int64, error) {
	okCount, failCount := int64(0), int64(0)
	for _, id := range commentIDs {
		if err := s.ReviewComment(ctx, id); err != nil {
			failCount++
		} else {
			okCount++
		}
	}
	return map[string]int64{"ok": okCount, "failed": failCount}, nil
}

// ---------- 统一执行流程 ----------

// runTask 执行一次 AI 任务（配置查询 → 路由 → 调用 → 用量落库）。
// 返回：模型输出文本与 token 用量（已落 ai_usage）。
func (s *AiService) runTask(ctx context.Context, taskName string, input string) (*ai.Result, error) {
	// 1. 任务配置（不存在视为任务未配置，种子数据缺失时给出明确提示）
	task, found, err := s.tasks.FindByName(ctx, taskName)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errs.New(errs.CodeNotFound, "AI 任务「"+taskName+"」未配置，请检查迁移 010 种子数据")
	}
	if !task.Enabled {
		return nil, errs.New(errs.CodeStateConflict, "AI 任务「"+taskName+"」已停用，请在 AI 设置中启用")
	}

	// 2. 路由供应商（任务绑定优先，否则按优先级自动路由）
	provider, err := s.resolveProvider(ctx, *task)
	if err != nil {
		return nil, err
	}
	apiKey, err := decryptAPIKey(provider.APIKeyEncrypted, s.keySecret)
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, "API Key 解密失败，请重新保存")
	}
	if apiKey == "" {
		return nil, errs.New(errs.CodeUpstream, "供应商「"+provider.Name+"」未配置 API Key，请先在 AI 设置中填写")
	}

	// 3. 模型：任务指定优先，否则取供应商默认模型
	model := strings.TrimSpace(task.Model)
	if model == "" {
		model = firstModel(provider.Models)
	}
	if model == "" {
		return nil, errs.New(errs.CodeBadRequest, "供应商「"+provider.Name+"」未配置模型")
	}

	// 4. 调用（超时保护；错误透出上游原因）
	client := ai.NewClient(provider.BaseURL, apiKey, model, aiRequestTimeout*time.Second)
	result, err := client.Chat(ctx, task.PromptTemplate, input, task.MaxTokens)
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, "AI 服务不可用："+err.Error())
	}

	// 5. 用量落库（失败静默：统计是观测数据，不影响场景结果）
	_ = s.usage.Record(ctx, repository.AiUsage{
		TaskName: task.TaskName, ProviderID: provider.ID,
		TokensIn: result.InTokens, TokensOut: result.OutTokens,
	})
	return result, nil
}

// resolveProvider 解析任务路由目标（纯逻辑：任务绑定 → 自动路由）。
func (s *AiService) resolveProvider(ctx context.Context, task repository.AiTask) (*repository.AiProvider, error) {
	// 任务绑定指定供应商（无论是否启用都按绑定执行——管理员显式选择）
	if task.ProviderID != nil {
		provider, found, err := s.providers.FindByID(ctx, *task.ProviderID)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, errs.New(errs.CodeNotFound, "任务绑定的供应商不存在，请重新选择")
		}
		return provider, nil
	}
	// 自动路由：已启用供应商中 priority 最小
	enabled, err := s.providers.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	candidates := make([]ai.Provider, 0, len(enabled))
	for _, p := range enabled {
		candidates = append(candidates, ai.Provider{ID: p.ID, Enabled: p.Enabled, Priority: p.Priority})
	}
	selected, err := ai.RouteProvider(candidates)
	if err != nil {
		return nil, errs.New(errs.CodeUpstream, err.Error())
	}
	provider, found, err := s.providers.FindByID(ctx, selected.ID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errs.New(errs.CodeUpstream, "路由到的供应商不存在")
	}
	return provider, nil
}

// ---------- 输入组装与输出解析 ----------

// buildPostInput 组装帖子场景输入（标题 + 正文前 maxPromptLen 字）。
func buildPostInput(title string, content string) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(title) != "" {
		parts = append(parts, "标题："+title)
	}
	parts = append(parts, "正文："+truncateRunes(content, maxPromptLen))
	return strings.Join(parts, "\n\n")
}

// parseTagArray 解析标签 JSON 数组（容错：剥离代码块/前后缀）。
// 返回：过滤后的标签（≤5 个，每个 ≤20 字符，去空去重）；解析失败返回空。
func parseTagArray(text string) ([]string, error) {
	raw := extractJSON(text)
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil, errs.New(errs.CodeUpstream, "AI 返回的标签格式不正确，请重试")
	}
	seen := make(map[string]bool, len(tags))
	items := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || len([]rune(tag)) > 20 || seen[tag] {
			continue
		}
		seen[tag] = true
		items = append(items, tag)
		if len(items) >= aiMaxTagCount {
			break
		}
	}
	return items, nil
}

// reviewVerdict AI 评论审核判定（结构化输出）。
type reviewVerdict struct {
	Risk   string `json:"risk"`   // high / low
	Reason string `json:"reason"` // 原因（中文）
}

// parseReviewVerdict 解析审核判定 JSON（容错：提取失败视为低风险放行）。
func parseReviewVerdict(text string) (reviewVerdict, bool) {
	raw := extractJSON(text)
	var verdict reviewVerdict
	if err := json.Unmarshal([]byte(raw), &verdict); err != nil {
		return reviewVerdict{}, false
	}
	if verdict.Risk != "high" && verdict.Risk != "low" {
		return reviewVerdict{}, false
	}
	return verdict, true
}

// extractJSON 从模型输出中提取 JSON 片段（纯函数：取首个 { 或 [ 到匹配的收尾）。
// 说明：兼容模型在 JSON 外套 markdown 代码块或说明文字的情况。
func extractJSON(text string) string {
	start := strings.IndexAny(text, "{[")
	if start < 0 {
		return text
	}
	end := strings.LastIndexAny(text, "}]")
	if end <= start {
		return text[start:]
	}
	return text[start : end+1]
}

// truncateRunes 按字符数截断（中英文安全，纯函数）。
func truncateRunes(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max])
}

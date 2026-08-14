// internal/service/ai_scenes.go
// AI 内置场景（M4）：帖子摘要 / 自动标签 / 评论审核 / 智能回复助手 / SEO 建议。
//
// 统一执行流程（runTask）：查任务配置 → 校验启用 → 路由供应商（任务绑定或按优先级）
// → 钩子改写 → 统一推理（chatProvider，含费用落库）→ 解析输出并落库。
// 设计：AI 输出均为结构化 JSON，解析采用容错策略（失败不误伤业务主流程）。
package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/roberts9012062/boke/internal/ai"
	"github.com/roberts9012062/boke/internal/plugin"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// 任务名常量（与迁移 010/011 种子一致）。
const (
	TaskPostSummary    = "post.summary"    // 帖子摘要
	TaskPostTags       = "post.tags"       // 自动标签
	TaskCommentReview  = "comment.review"  // 评论审核
	TaskReplyAssistant = "reply.assistant" // 智能回复助手（续写/润色/翻译）
	TaskSeoAdvice      = "seo.advice"      // SEO 建议
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

// replyActionLabel 智能回复助手操作类型的中文描述（提示词 {action} 占位符用）。
func replyActionLabel(action string) (string, error) {
	switch action {
	case "continue":
		return "续写：在保持原文风格与主题的前提下，自然衔接并补全后续内容", nil
	case "polish":
		return "润色：优化措辞与表达，使语句更通顺优美，保持原意不变", nil
	case "translate":
		return "翻译：将内容翻译为中文（若已是中文则翻译为英文）", nil
	default:
		return "", errs.New(errs.CodeBadRequest, "操作类型需为 continue（续写）/ polish（润色）/ translate（翻译）")
	}
}

// GenReplyAssistant 智能回复助手（M4 蓝图场景：AI 续写/润色/翻译）。
// 参数：action ∈ continue / polish / translate；content 前端当前编辑正文（空则回退查库）。
// 说明：续写/润色/翻译必须对「编辑框内正在编辑的正文」操作——若仅按 postID 查库，
//       用户在编辑框的未保存改动会被忽略，AI 拿到旧正文甚至空正文。故正文优先取传入值。
// 返回：处理后的文本（非流式；前端编辑态触发）。
func (s *AiService) GenReplyAssistant(ctx context.Context, postID int64, action string, content string) (string, error) {
	label, err := replyActionLabel(action)
	if err != nil {
		return "", err
	}
	// 正文来源：优先前端传入的当前编辑正文；为空（旧调用/未传）回退按 postID 查库
	body := strings.TrimSpace(content)
	if body == "" {
		post, err := s.posts.FindByID(ctx, postID)
		if err != nil {
			return "", err
		}
		body = post.Content
	}
	body = truncateRunes(body, maxPromptLen)
	if body == "" {
		return "", errs.New(errs.CodeBadRequest, "帖子正文为空，无法执行 AI 操作")
	}
	// {action} 与 {content} 占位符由任务提示词模板承载
	input := "操作类型：" + label + "\n\n" + body
	result, err := s.runTask(ctx, TaskReplyAssistant, input)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// seoAdvice AI SEO 建议（标题/描述/关键词）。
type seoAdvice struct {
	Title       string   `json:"title"`       // SEO 标题
	Description string   `json:"description"` // SEO 描述
	Keywords    []string `json:"keywords"`    // 关键词（≤3 个）
}

// GenSeoAdvice 生成 SEO 建议（M4 蓝图场景：对文章给出标题/描述/关键词建议）。
// 返回：结构化建议（前端回填到 SEO 面板）；不直接落库（作者确认后经 SEO 接口保存）。
func (s *AiService) GenSeoAdvice(ctx context.Context, postID int64) (*seoAdvice, error) {
	post, err := s.posts.FindByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	result, err := s.runTask(ctx, TaskSeoAdvice, buildPostInput(post.Title, post.Content))
	if err != nil {
		return nil, err
	}
	raw := extractJSON(result.Text)
	var advice seoAdvice
	if err := json.Unmarshal([]byte(raw), &advice); err != nil {
		return nil, errs.New(errs.CodeUpstream, "AI 返回的 SEO 建议格式不正确，请重试")
	}
	return &advice, nil
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

// runTask 执行一次 AI 任务（配置查询 → 路由 → 钩子改写 → 统一推理 → 钩子通知）。
// 返回：模型输出文本与 token 用量（已落 ai_usage，含费用折算）。
func (s *AiService) runTask(ctx context.Context, taskName string, input string) (*ai.Result, error) {
	// 1. 任务配置（不存在视为任务未配置，种子数据缺失时给出明确提示）
	task, found, err := s.tasks.FindByName(ctx, taskName)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errs.New(errs.CodeNotFound, "AI 任务「"+taskName+"」未配置，请检查迁移种子数据")
	}
	if !task.Enabled {
		return nil, errs.New(errs.CodeStateConflict, "AI 任务「"+taskName+"」已停用，请在 AI 设置中启用")
	}

	// 2. 路由供应商（任务绑定优先，否则按优先级自动路由）
	provider, err := s.resolveProvider(ctx, *task)
	if err != nil {
		return nil, err
	}

	// 3. 插件钩子：ai.before_generate（M3.9 同步，可改写输入）
	if s.hooks != nil {
		if res := s.hooks.Dispatch(ctx, plugin.HookAIBeforeGenerate, plugin.Event{
			Payload: map[string]any{"task": taskName, "input": input, "model": task.Model},
		}); res.OK {
			if modified, ok := res.Modify.(map[string]any); ok {
				if v, ok := modified["input"].(string); ok && v != "" {
					input = v // 插件改写输入（如注入上下文/过滤敏感词）
				}
			}
		}
	}

	// 4. 统一推理（供应商解密 → Provider.Chat → 用量/费用落库）
	result, err := s.chatProvider(ctx, provider, task.TaskName, ai.ChatRequest{
		Model: task.Model,
		Messages: []ai.Message{
			{Role: "system", Content: task.PromptTemplate},
			{Role: "user", Content: input},
		},
		MaxTokens: task.MaxTokens,
	})
	if err != nil {
		return nil, err
	}

	// 5. 插件钩子：ai.after_generate（M3.9 异步通知）
	if s.hooks != nil {
		s.hooks.Dispatch(ctx, plugin.HookAIAfterGenerate, plugin.Event{
			Payload: map[string]any{"task": taskName, "result": result.Text},
		})
	}
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
	candidates := make([]ai.ProviderCandidate, 0, len(enabled))
	for _, p := range enabled {
		candidates = append(candidates, ai.ProviderCandidate{ID: p.ID, Enabled: p.Enabled, Priority: p.Priority})
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

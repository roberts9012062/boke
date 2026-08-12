// internal/service/moderation.go
// 内容治理业务（M2）：举报提交/工单处理、敏感词库 + 命中检测、封禁记录。
// 设计稿：前台《举报》表单（6 原因 + 补充说明）+ 后台《审核队列》《敏感词》《封禁管理》。
package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// 举报原因预置选项（设计稿：垃圾广告/骚扰辱骂/色情低俗/违法违规/侵犯版权/其他）。
var reportReasons = map[string]bool{
	"垃圾广告": true, "骚扰辱骂": true, "色情低俗": true,
	"违法违规": true, "侵犯版权": true, "其他": true,
}

// ModerationService 内容治理服务（连接器类）。
type ModerationService struct {
	reports   *repository.ReportRepo     // 举报工单
	sensitive *repository.SensitiveRepo  // 敏感词库
	bans      *repository.BanRepo        // 封禁记录
	users     *repository.UserRepo       // 用户（举报人/封禁昵称）
	posts     *repository.PostRepo       // 帖子（举报目标摘要）
	comments  *repository.CommentRepo    // 评论（举报目标摘要）
}

// NewModerationService 创建内容治理服务。
func NewModerationService(
	reports *repository.ReportRepo,
	sensitive *repository.SensitiveRepo,
	bans *repository.BanRepo,
	users *repository.UserRepo,
	posts *repository.PostRepo,
	comments *repository.CommentRepo,
) *ModerationService {
	return &ModerationService{reports: reports, sensitive: sensitive, bans: bans, users: users, posts: posts, comments: comments}
}

// ---------- 举报 ----------

// ReportDTO 举报工单 DTO（后台列表，含目标摘要与举报人）。
type ReportDTO struct {
	ID           int64  `json:"id"`            // 工单 ID
	Reporter     string `json:"reporter"`      // 举报人昵称
	TargetType   string `json:"target_type"`   // 对象类型
	TargetID     int64  `json:"target_id"`     // 对象 ID
	TargetBrief  string `json:"target_brief"`  // 目标内容摘要
	Reason       string `json:"reason"`        // 原因
	Detail       string `json:"detail"`        // 补充说明
	Status       string `json:"status"`        // 状态
	Source       string `json:"source"`        // 来源：user 人工举报 / ai AI 审核标记（M4）
	CreatedAt    string `json:"created_at"`    // 提交时间
	CostSeconds  *int64 `json:"cost_seconds"`  // 处理耗时（秒，已处理工单；P1 审核耗时）
}

// SubmitReport 提交举报（校验对象存在性与原因选项）。
// 参数：reporterID 举报人；targetType post/comment/user；targetID；reason；detail 补充说明。
func (s *ModerationService) SubmitReport(ctx context.Context, reporterID int64, targetType string, targetID int64, reason string, detail string) error {
	if reporterID == 0 {
		return errs.ErrUnauthorized
	}
	if targetType != "post" && targetType != "comment" && targetType != "user" {
		return errs.New(errs.CodeBadRequest, "举报对象类型不正确")
	}
	if targetID <= 0 {
		return errs.New(errs.CodeBadRequest, "举报对象不存在")
	}
	if !reportReasons[reason] {
		return errs.New(errs.CodeBadRequest, "请选择举报原因")
	}
	if len([]rune(detail)) > 500 {
		return errs.New(errs.CodeBadRequest, "补充说明不能超过 500 字")
	}
	_, err := s.reports.Create(ctx, repository.Report{
		ReporterID: reporterID,
		TargetType: targetType,
		TargetID:   targetID,
		Reason:     reason,
		Detail:     detail,
	})
	return err
}

// ListReports 工单列表（状态过滤 + 组装举报人/目标摘要）。
func (s *ModerationService) ListReports(ctx context.Context, status string, page int, pageSize int) ([]ReportDTO, int64, error) {
	reports, total, err := s.reports.List(ctx, status, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	items := make([]ReportDTO, 0, len(reports))
	for _, rep := range reports {
		items = append(items, s.assembleReport(ctx, rep))
	}
	return items, total, nil
}

// SetReportStatus 处理工单（resolved 已解决 / rejected 已驳回）。
func (s *ModerationService) SetReportStatus(ctx context.Context, reportID int64, status string) error {
	if status != repository.ReportResolved && status != repository.ReportRejected {
		return errs.New(errs.CodeBadRequest, "状态仅支持 resolved / rejected")
	}
	return s.reports.SetStatus(ctx, reportID, status)
}

// VerdictReport 复核 AI 标记工单（M4：审核队列「放行/删除」操作）。
// 规则：仅限 AI 来源（source=ai）且目标为评论的待处理工单；
//       allow=放行（评论恢复可见）+ 工单已解决；delete=删除评论 + 工单已解决。
// 设计：人工举报工单沿用「解决/驳回」，AI 工单用「放行/删除」，两套互不干扰。
func (s *ModerationService) VerdictReport(ctx context.Context, reportID int64, action string) error {
	if action != "allow" && action != "delete" {
		return errs.New(errs.CodeBadRequest, "操作仅支持 allow（放行）/ delete（删除）")
	}
	report, found, err := s.reports.FindByID(ctx, reportID)
	if err != nil {
		return err
	}
	if !found {
		return errs.ErrNotFound
	}
	if report.Source != repository.ReportSourceAI {
		return errs.New(errs.CodeStateConflict, "该工单非 AI 标记，请使用「解决/驳回」处理")
	}
	if report.TargetType != "comment" {
		return errs.New(errs.CodeStateConflict, "仅支持对评论工单执行放行/删除")
	}
	if report.Status != repository.ReportPending {
		return errs.New(errs.CodeStateConflict, "该工单已处理")
	}
	// 评论处置：放行 → 恢复可见；删除 → 标记删除
	targetStatus := "visible"
	if action == "delete" {
		targetStatus = "deleted"
	}
	if err := s.comments.SetStatus(ctx, report.TargetID, targetStatus); err != nil {
		return err
	}
	return s.reports.SetStatus(ctx, reportID, repository.ReportResolved)
}

// ReportStats 工单统计（设计稿审核队列统计条：待处理/高风险/今日已审/平均耗时）。
type ReportStats struct {
	Pending        int64 `json:"pending"`         // 待处理（全量）
	HighRisk       int64 `json:"high_risk"`       // 高风险（M4：AI 审核标记且待复核）
	ResolvedToday  int64 `json:"resolved_today"`  // 今日已审
	AvgCostSeconds int64 `json:"avg_cost_seconds"` // 平均处理耗时（秒，P1 审核耗时埋点）
}

// ReportStats 审核队列统计。
func (s *ModerationService) ReportStats(ctx context.Context) (*ReportStats, error) {
	pending, err := s.reports.CountPending(ctx)
	if err != nil {
		return nil, err
	}
	highRisk, err := s.reports.CountHighRisk(ctx)
	if err != nil {
		return nil, err
	}
	resolvedToday, err := s.reports.CountResolvedToday(ctx)
	if err != nil {
		return nil, err
	}
	avgCost, err := s.reports.AvgResolveCost(ctx)
	if err != nil {
		return nil, err
	}
	return &ReportStats{Pending: pending, HighRisk: highRisk, ResolvedToday: resolvedToday, AvgCostSeconds: avgCost}, nil
}

// SensitiveStats 敏感词统计（设计稿统计条：全部/拦截/审核）。
type SensitiveStats struct {
	Total     int64 `json:"total"`     // 全部
	Forbidden int64 `json:"forbidden"` // 拦截
	Review    int64 `json:"review"`    // 审核
}

// SensitiveStats 敏感词级别统计。
func (s *ModerationService) SensitiveStats(ctx context.Context) (*SensitiveStats, error) {
	forbidden, review, err := s.sensitive.CountByLevel(ctx)
	if err != nil {
		return nil, err
	}
	return &SensitiveStats{Total: forbidden + review, Forbidden: forbidden, Review: review}, nil
}

// ---------- 敏感词 ----------

// SensitiveWordDTO 敏感词 DTO。
type SensitiveWordDTO struct {
	ID        int64  `json:"id"`         // 词 ID
	Word      string `json:"word"`       // 词内容
	Level     string `json:"level"`      // forbidden / review
	HitCount  int64  `json:"hit_count"`  // 命中次数（P1 命中统计）
	CreatedAt string `json:"created_at"` // 添加时间
}

// ListSensitiveWords 词库列表（关键词搜索 + 分页）。
func (s *ModerationService) ListSensitiveWords(ctx context.Context, keyword string, page int, pageSize int) ([]SensitiveWordDTO, int64, error) {
	words, total, err := s.sensitive.List(ctx, keyword, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	items := make([]SensitiveWordDTO, 0, len(words))
	for _, w := range words {
		items = append(items, SensitiveWordDTO{ID: w.ID, Word: w.Word, Level: w.Level, HitCount: w.HitCount, CreatedAt: w.CreatedAt.Format(time.RFC3339)})
	}
	return items, total, nil
}

// IncrHit 敏感词命中 +1（P1 命中统计；发帖/评论拦截命中时由调用方触发）。
// 说明：失败静默（命中统计是观测数据，不影响拦截主流程）。
func (s *ModerationService) IncrHit(ctx context.Context, word string) {
	_ = s.sensitive.IncrHit(ctx, word)
}

// AddSensitiveWord 添加敏感词（forbidden/review 两级）。
func (s *ModerationService) AddSensitiveWord(ctx context.Context, word string, level string) error {
	word = strings.TrimSpace(word)
	if word == "" || len([]rune(word)) > 100 {
		return errs.New(errs.CodeBadRequest, "敏感词需为 1-100 字符")
	}
	if level != repository.WordForbidden && level != repository.WordReview {
		return errs.New(errs.CodeBadRequest, "级别仅支持 forbidden / review")
	}
	added, err := s.sensitive.Create(ctx, word, level)
	if err != nil {
		return err
	}
	if !added {
		return errs.New(errs.CodeConflict, "该敏感词已存在")
	}
	return nil
}

// DeleteSensitiveWord 删除敏感词。
func (s *ModerationService) DeleteSensitiveWord(ctx context.Context, word string) error {
	return s.sensitive.Delete(ctx, word)
}

// AddWords 批量添加敏感词（后台站点设置「敏感词（逗号分隔）」入口，forbidden 级别）。
// 参数：words 原始列表（自动去空格/去空项；已存在的跳过不报错）。
// 返回：成功添加数与跳过数（重复/空项）。
func (s *ModerationService) AddWords(ctx context.Context, words []string) (added int, skipped int, err error) {
	for _, raw := range words {
		word := strings.TrimSpace(raw)
		if word == "" {
			skipped++
			continue
		}
		ok, err := s.sensitive.Create(ctx, word, repository.WordForbidden)
		if err != nil {
			return added, skipped, err
		}
		if ok {
			added++
		} else {
			skipped++
		}
	}
	// 有新增时刷新内存词表（后台变更后即时生效）
	if added > 0 {
		if err := s.ReloadForbidden(ctx); err != nil {
			return added, skipped, err
		}
	}
	return added, skipped, nil
}

// ---------- 封禁 ----------

// BanRecordDTO 封禁记录 DTO（含用户昵称）。
type BanRecordDTO struct {
	ID        int64   `json:"id"`         // 记录 ID
	UserID    int64   `json:"user_id"`    // 被封禁用户
	Nickname  string  `json:"nickname"`   // 用户昵称
	Reason    string  `json:"reason"`     // 原因
	Until     *string `json:"until"`      // 解封时间（NULL = 永久）
	CreatedBy int64   `json:"created_by"` // 操作者
	CreatedAt string  `json:"created_at"` // 封禁时间
}

// ListBans 封禁记录列表。
func (s *ModerationService) ListBans(ctx context.Context, page int, pageSize int) ([]BanRecordDTO, int64, error) {
	records, total, err := s.bans.List(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	items := make([]BanRecordDTO, 0, len(records))
	for _, b := range records {
		dto := BanRecordDTO{
			ID: b.ID, UserID: b.UserID, Reason: b.Reason,
			CreatedBy: b.CreatedBy, CreatedAt: b.CreatedAt.Format(time.RFC3339),
		}
		if b.Until != nil {
			until := b.Until.Format(time.RFC3339)
			dto.Until = &until
		}
		// 用户昵称（失败降级为空）
		if u, err := s.users.FindByID(ctx, b.UserID); err == nil {
			dto.Nickname = u.Nickname
		}
		items = append(items, dto)
	}
	return items, total, nil
}

// ---------- 敏感词命中检测（发帖/评论拦截钩子） ----------

// forbiddenWords 内存词表缓存（启动加载，后台增删后刷新）。
// 说明：数据量小（MVP），启动与后台变更时全量加载；词表大时换 Aho-Corasick（架构 9.3）。
// 并发安全：发帖/评论高频读（CheckForbidden）与后台增删触发整体替换（ReloadForbidden）
//           并发执行，须读写锁保护（此前无锁存在数据竞争）。
var (
	forbiddenWords []string
	forbiddenMu    sync.RWMutex
)

// ReloadForbidden 重新加载 forbidden 词表（启动与后台变更后调用）。
func (s *ModerationService) ReloadForbidden(ctx context.Context) error {
	words, err := s.sensitive.AllForbidden(ctx)
	if err != nil {
		return err
	}
	forbiddenMu.Lock()
	forbiddenWords = words
	forbiddenMu.Unlock()
	return nil
}

// CheckForbidden 校验文本是否命中 forbidden 敏感词。
// 返回：命中的敏感词（未命中返回空串）。
func (s *ModerationService) CheckForbidden(content string) string {
	forbiddenMu.RLock()
	words := forbiddenWords
	forbiddenMu.RUnlock()
	for _, word := range words {
		if strings.Contains(content, word) {
			return word
		}
	}
	return ""
}

// ---------- 内部辅助 ----------

// assembleReport 组装工单 DTO（举报人昵称 + 目标摘要 + 处理耗时）。
func (s *ModerationService) assembleReport(ctx context.Context, rep repository.Report) ReportDTO {
	dto := ReportDTO{
		ID: rep.ID, TargetType: rep.TargetType, TargetID: rep.TargetID,
		Reason: rep.Reason, Detail: rep.Detail, Status: rep.Status,
		Source: rep.Source, CreatedAt: rep.CreatedAt.Format(time.RFC3339),
	}
	// 处理耗时（秒；已处理工单才展示，P1 审核耗时）
	if rep.ResolvedAt != nil {
		cost := int64(rep.ResolvedAt.Sub(rep.CreatedAt).Seconds())
		dto.CostSeconds = &cost
	}
	// 举报人昵称（失败降级为空）
	if u, err := s.users.FindByID(ctx, rep.ReporterID); err == nil {
		dto.Reporter = u.Nickname
	}
	// 目标摘要：帖子取标题/正文，评论取内容，用户取昵称（简化展示）
	switch rep.TargetType {
	case "post":
		if p, err := s.posts.FindByID(ctx, rep.TargetID); err == nil {
			dto.TargetBrief = p.Title
			if dto.TargetBrief == "" {
				dto.TargetBrief = briefPreview(p.Content)
			}
		}
	case "comment":
		if c, err := s.comments.FindByID(ctx, rep.TargetID); err == nil {
			dto.TargetBrief = briefPreview(c.Content)
		}
	case "user":
		if u, err := s.users.FindByID(ctx, rep.TargetID); err == nil {
			dto.TargetBrief = "@" + u.Username + " " + u.Nickname
		}
	}
	return dto
}

// briefPreview 目标摘要截断（60 字符）。
func briefPreview(content string) string {
	runes := []rune(strings.Join(strings.Fields(content), " "))
	if len(runes) > 60 {
		return string(runes[:60]) + "…"
	}
	return string(runes)
}

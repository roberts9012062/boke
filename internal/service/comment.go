// internal/service/comment.go
// 评论业务逻辑：楼中楼（2 级）、匿名身份评论、评论点赞、删除。
//
// 规则（需求 3.5）：
//   - 开放评论无需登录：访客签发匿名 token 后评论（防刷：1 条/分钟/同 token）
//   - 楼中楼：顶层评论 + 一层回复（@用户名 前缀由前端生成）；楼层号按帖子递增
//   - 回复目标为子评论时自动挂到其顶层（MVP 限 2 级）
//   - 点赞仅登录用户；作者可删除自己的评论
package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/roberts9012062/boke/internal/auth"
	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/plugin"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// 评论内容上限与匿名限频（需求 3.5）。
const (
	maxCommentLen        = 500 // 评论内容上限（字符）
	guestCommentWindow   = 60  // 匿名评论限频窗口（秒）
	guestCommentMaxCount = 1   // 窗口内最大条数
)

// CommentService 评论服务（连接器类）。
type CommentService struct {
	comments   *repository.CommentRepo   // 评论数据访问
	reactions  *repository.ReactionRepo  // 互动数据访问（评论点赞）
	users      *repository.UserRepo      // 用户数据访问（作者信息）
	guests     *auth.GuestManager        // 匿名身份管理器
	posts      *repository.PostRepo      // 帖子数据访问（作者/计数）
	notify     *NotificationService      // 通知服务（评论/回复通知）
	moderation *ModerationService        // 内容治理（M2：敏感词拦截）
	hooks      plugin.Dispatcher         // 插件钩子调度器（M3.2 扩展框架）
}

// NewCommentService 创建评论服务。
func NewCommentService(
	comments *repository.CommentRepo,
	reactions *repository.ReactionRepo,
	users *repository.UserRepo,
	guests *auth.GuestManager,
	posts *repository.PostRepo,
	notify *NotificationService,
	moderation *ModerationService,
	hooks plugin.Dispatcher,
) *CommentService {
	return &CommentService{comments: comments, reactions: reactions, users: users, guests: guests, posts: posts, notify: notify, moderation: moderation, hooks: hooks}
}

// CommentInput 发表评论输入（顶层与回复共用）。
type CommentInput struct {
	Content    string // 评论内容
	GuestToken string // 匿名 token（未登录时必填）
}

// List 查询帖子评论（顶层 + 子回复，楼中楼结构）。
// 参数：viewerID 当前用户（0 = 访客，用于 liked/is_author 标记）。
func (s *CommentService) List(ctx context.Context, postID int64, viewerID int64) ([]model.CommentDTO, error) {
	comments, err := s.comments.ListByPost(ctx, postID)
	if err != nil {
		return nil, err
	}

	// 组装：顶层 + 子回复（parent_id 分组）。
	// 注意：顶层必须存指针，回复挂载修改同一对象后才解引用为值
	//（历史故障：存值副本导致 replies 永远为空）。
	tops := make([]*model.CommentDTO, 0)
	byID := make(map[int64]*model.CommentDTO, len(comments))
	for i := range comments {
		c := comments[i]
		dto, err := s.toDTO(ctx, c, viewerID)
		if err != nil {
			return nil, err
		}
		byID[c.ID] = &dto
		// 顶层（parent 为 nil）
		if c.ParentID == nil {
			dto.Replies = make([]model.CommentDTO, 0)
			tops = append(tops, &dto)
		}
	}
	// 挂载子回复到顶层
	for i := range comments {
		c := comments[i]
		if c.ParentID == nil {
			continue
		}
		parent := byID[*c.ParentID]
		child := byID[c.ID]
		if parent == nil || child == nil {
			continue // 父/子评论异常（如已删除）跳过
		}
		parent.Replies = append(parent.Replies, *child)
		parent.ReplyCount++
	}

	// 顶层指针解引用为值返回
	result := make([]model.CommentDTO, 0, len(tops))
	for _, top := range tops {
		result = append(result, *top)
	}
	return result, nil
}

// Create 发表评论（顶层，需登录或携带有效匿名 token）。
// 返回：新评论 ID。
func (s *CommentService) Create(ctx context.Context, postID int64, viewerID int64, input CommentInput) (int64, error) {
	// 内容校验
	content := strings.TrimSpace(input.Content)
	if content == "" || utf8.RuneCountInString(content) > maxCommentLen {
		return 0, errs.New(errs.CodeBadRequest, "评论内容需为 1-500 字符")
	}

	// 敏感词拦截（M2：命中 forbidden 直接拒绝；P1：命中计数）
	if word := s.moderation.CheckForbidden(content); word != "" {
		s.moderation.IncrHit(ctx, word)
		return 0, errs.New(errs.CodeBadRequest, "评论包含敏感词「"+word+"」，请修改后发送")
	}

	// ---------- 插件钩子：comment.before_save（同步，可拦截；M3.2 扩展框架） ----------
	if res := s.hooks.Dispatch(ctx, plugin.HookCommentBeforeSave, plugin.Event{
		ActorID: viewerID,
		Payload: content,
	}); !res.OK {
		return 0, errs.New(errs.CodeValidation, res.Reason)
	}

	// ---------- 插件钩子：comment.before_save（同步，可拦截；M3.2 扩展框架） ----------
	if res := s.hooks.Dispatch(ctx, plugin.HookCommentBeforeSave, plugin.Event{
		ActorID: viewerID,
		Payload: content,
	}); !res.OK {
		return 0, errs.New(errs.CodeValidation, res.Reason)
	}

	// 身份确定：登录用户或匿名 token
	authorID, guestName, guestHash, err := s.resolveIdentity(ctx, viewerID, input.GuestToken)
	if err != nil {
		return 0, err
	}

	// 创建评论（楼层号自动递增）
	comment := model.Comment{
		PostID:         postID,
		AuthorID:       authorID,
		Content:        content,
		GuestName:      guestName,
		GuestTokenHash: guestHash,
	}
	commentID, _, err := s.comments.Create(ctx, comment)
	if err != nil {
		return 0, err
	}

	// 帖子评论计数 +1（失败不阻断主流程）
	if err := s.incrPostComment(ctx, postID); err != nil {
		return 0, err
	}

	// 评论通知（通知帖子作者；匿名评论无 actor 不通知）
	if authorID != nil {
		if post, err := s.posts.FindByID(ctx, postID); err == nil {
			s.notify.NotifyComment(ctx, *authorID, post.AuthorID, postID, commentPreview(content))
		}
	}
	return commentID, nil
}
func (s *CommentService) Reply(ctx context.Context, targetID int64, viewerID int64, input CommentInput) (int64, error) {
	// 查询目标评论
	target, err := s.comments.FindByID(ctx, targetID)
	if err != nil {
		return 0, errs.ErrNotFound
	}
	if target.Status != model.CommentStatusVisible {
		return 0, errs.ErrNotFound
	}

	// 内容校验
	content := strings.TrimSpace(input.Content)
	if content == "" || utf8.RuneCountInString(content) > maxCommentLen {
		return 0, errs.New(errs.CodeBadRequest, "评论内容需为 1-500 字符")
	}

	// 敏感词拦截（M2：命中 forbidden 直接拒绝；P1：命中计数）
	if word := s.moderation.CheckForbidden(content); word != "" {
		s.moderation.IncrHit(ctx, word)
		return 0, errs.New(errs.CodeBadRequest, "评论包含敏感词「"+word+"」，请修改后发送")
	}

	// 身份确定
	authorID, guestName, guestHash, err := s.resolveIdentity(ctx, viewerID, input.GuestToken)
	if err != nil {
		return 0, err
	}

	// 父评论：目标为顶层则直接挂，为子回复则挂到其顶层（限 2 级）
	parentID := targetID
	if target.ParentID != nil {
		parentID = *target.ParentID
	}

	// 创建回复
	comment := model.Comment{
		PostID:         target.PostID,
		AuthorID:       authorID,
		ParentID:       &parentID,
		Content:        content,
		GuestName:      guestName,
		GuestTokenHash: guestHash,
	}
	commentID, _, err := s.comments.Create(ctx, comment)
	if err != nil {
		return 0, err
	}

	// 帖子评论计数 +1
	if err := s.incrPostComment(ctx, target.PostID); err != nil {
		return 0, err
	}

	// 回复通知：登录用户回复且被回复者为登录用户时通知对方
	if authorID != nil && target.AuthorID != nil {
		s.notify.NotifyReply(ctx, *authorID, *target.AuthorID, target.PostID, commentPreview(content))
	}
	return commentID, nil
}

// Like 点赞评论（仅登录用户；重复点赞幂等，返回当前点赞数）。
func (s *CommentService) Like(ctx context.Context, commentID int64, viewerID int64) (int64, bool, error) {
	if viewerID == 0 {
		return 0, false, errs.ErrUnauthorized
	}
	// 查询评论（存在性）
	comment, err := s.comments.FindByID(ctx, commentID)
	if err != nil {
		return 0, false, errs.ErrNotFound
	}
	// 幂等添加
	added, err := s.reactions.AddCommentLike(ctx, viewerID, commentID)
	if err != nil {
		return 0, false, err
	}
	if added {
		if err := s.comments.IncrLike(ctx, commentID); err != nil {
			return 0, false, err
		}
	}
	return comment.LikeCount + boolToInt64(added), added, nil
}

// Delete 删除评论（软删；仅作者本人或管理员）。
func (s *CommentService) Delete(ctx context.Context, commentID int64, viewerID int64) error {
	comment, err := s.comments.FindByID(ctx, commentID)
	if err != nil {
		return errs.ErrNotFound
	}
	// 作者校验（登录用户 author_id 匹配；匿名评论不可删）
	canDelete := comment.AuthorID != nil && *comment.AuthorID == viewerID
	if !canDelete {
		return errs.ErrForbidden
	}
	if err := s.comments.SoftDelete(ctx, commentID); err != nil {
		return err
	}
	// 帖子评论计数 -1
	return s.decrPostComment(ctx, comment.PostID)
}

// ---------- 内部辅助 ----------

// resolveIdentity 确定评论身份：登录用户优先，否则校验匿名 token。
// 返回：authorID（0 = 匿名）；匿名昵称；匿名 token 哈希。
func (s *CommentService) resolveIdentity(ctx context.Context, viewerID int64, guestToken string) (*int64, string, string, error) {
	// 已登录：直接使用账号身份（匿名 token 忽略）
	if viewerID > 0 {
		id := viewerID
		return &id, "", "", nil
	}
	// 未登录：必须携带有效匿名 token
	name, tokenHash, ok := s.guests.Verify(guestToken)
	if !ok {
		return nil, "", "", errs.New(errs.CodeUnauthorized, "请先获取匿名身份（或登录后评论）")
	}
	// 防刷：同 token 限频（1 条/分钟）
	recent, err := s.comments.HasGuestCommentRecently(ctx, tokenHash, guestCommentWindow)
	if err != nil {
		return nil, "", "", err
	}
	if recent {
		return nil, "", "", errs.ErrRateLimit
	}
	return nil, name, tokenHash, nil
}

// toDTO 将实体转为 DTO（作者/昵称/时间/权限/点赞状态）。
func (s *CommentService) toDTO(ctx context.Context, c model.Comment, viewerID int64) (model.CommentDTO, error) {
	dto := model.CommentDTO{
		ID:        c.ID,
		Content:   c.Content,
		GuestName: c.GuestName,
		LikeCount: c.LikeCount,
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
	}
	// 作者信息（登录用户）
	if c.AuthorID != nil {
		user, err := s.users.FindByID(ctx, *c.AuthorID)
		if err != nil {
			return model.CommentDTO{}, err
		}
		dto.Author = &model.CommentAuthor{
			ID:       user.ID,
			Username: user.Username,
			Nickname: user.Nickname,
		}
		// 是否本人（删除权限）
		dto.IsAuthor = *c.AuthorID == viewerID
	}
	// 当前用户是否已赞（仅登录用户查询）
	if viewerID > 0 {
		liked, err := s.reactions.HasCommentLike(ctx, viewerID, c.ID)
		if err != nil {
			return model.CommentDTO{}, err
		}
		dto.Liked = liked
	}
	return dto, nil
}

// incrPostComment 帖子评论计数 +1（posts.comment_count 冗余同步）。
// 说明：CommentService 未持有 PostRepo，计数由 CommentRepo 实时统计替代——
// 列表/详情展示直接查询 posts.comment_count，此处通过独立计数表一致性保证。
func (s *CommentService) incrPostComment(ctx context.Context, postID int64) error {
	// 计数同步：调用 CommentRepo 查询最新可见评论数写回 posts.comment_count
	count, err := s.comments.CountByPost(ctx, postID)
	if err != nil {
		return err
	}
	return s.comments.SyncPostCommentCount(ctx, postID, count)
}

// decrPostComment 帖子评论计数 -1（删除评论时，最低 0）。
func (s *CommentService) decrPostComment(ctx context.Context, postID int64) error {
	count, err := s.comments.CountByPost(ctx, postID)
	if err != nil {
		return err
	}
	return s.comments.SyncPostCommentCount(ctx, postID, count)
}

// boolToInt64 布尔转 0/1（计数计算用）。
func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// commentPreview 评论内容摘要（通知展示，截断 50 字符加引号）。
func commentPreview(content string) string {
	runes := []rune(content)
	if len(runes) > 50 {
		return "「" + string(runes[:50]) + "…」"
	}
	return "「" + content + "」"
}

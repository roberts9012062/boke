// internal/service/post.go
// 帖子业务逻辑：发帖/草稿/更新/发布/删除/时间线/详情/媒体上传。
//
// 规则（需求 3.2-3.4）：
//   - 正文 ≤2000 字；标签 ≤5 个（每个 ≤20 字符）；可见性 公开/私密
//   - 私密帖仅作者可见（其他人视为不存在）
//   - 时间线仅展示 published，最新发布在前，支持类型过滤（全部/图/音/影）
//   - 标签：帖子带 # 标签即进入话题聚合（tags 表 + post_tags 关联 + 计数）
package service

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yueyan/boke/internal/media"
	"github.com/yueyan/boke/internal/model"
	"github.com/yueyan/boke/internal/repository"
	"github.com/yueyan/boke/pkg/errs"
)

// 帖子正文与标签限制（需求 3.4）。
const (
	maxContentLen  = 2000 // 正文上限（字）
	maxTags        = 5    // 标签上限（个）
	maxTagLen      = 20   // 单个标签上限（字符）
	maxTitleLen    = 100  // 标题上限
	summaryLen     = 100  // 摘要截断长度（字符）
	summaryPreview = 60   // 列表摘要预览长度（字符）
)

// PostService 帖子服务（连接器类，聚合依赖）。
type PostService struct {
	posts      *repository.PostRepo     // 帖子数据访问
	tags       *repository.TagRepo      // 标签数据访问
	medias     *repository.MediaRepo    // 媒体数据访问
	users      *repository.UserRepo     // 用户数据访问（作者信息）
	store      *media.Store             // 媒体存储（本地磁盘）
	moderation *ModerationService       // 内容治理（M2：敏感词拦截）
	relations  *repository.RelationRepo // 用户关系（M2：仅关注者帖互关判断）
}

// NewPostService 创建帖子服务。
func NewPostService(
	posts *repository.PostRepo,
	tags *repository.TagRepo,
	medias *repository.MediaRepo,
	users *repository.UserRepo,
	store *media.Store,
	moderation *ModerationService,
	relations *repository.RelationRepo,
) *PostService {
	return &PostService{posts: posts, tags: tags, medias: medias, users: users, store: store, moderation: moderation, relations: relations}
}

// ---------- 发帖 / 草稿 ----------

// Create 创建帖子（status=draft 存草稿；published 发布）。
// 返回：新帖子 ID。
func (s *PostService) Create(ctx context.Context, userID int64, req model.CreatePostReq) (int64, error) {
	// ---------- 参数校验（服务端二次校验） ----------
	if err := validatePostReq(req); err != nil {
		return 0, err
	}

	// ---------- 敏感词拦截（M2：发布时命中 forbidden 直接拒绝） ----------
	if word := s.moderation.CheckForbidden(req.Content); word != "" {
		return 0, errs.New(errs.CodeBadRequest, "内容包含敏感词「"+word+"」，请修改后发布")
	}

	// 状态校验：仅允许 draft / published
	if req.Status != model.PostStatusDraft && req.Status != model.PostStatusPublished {
		return 0, errs.New(errs.CodeBadRequest, "帖子状态不正确")
	}

	// ---------- 组装实体 ----------
	post := model.Post{
		AuthorID:    userID,
		Title:       strings.TrimSpace(req.Title),
		Summary:     buildSummary(req.Content),
		Content:     req.Content,
		ContentType: req.ContentType,
		Status:      req.Status,
		Visibility:  req.Visibility,
		MediaIDs:    req.MediaIDs,
	}
	// 发布时写入发布时间
	if req.Status == model.PostStatusPublished {
		now := time.Now()
		post.PublishedAt = &now
	}
	// 封面：图片帖取第一张媒体
	if len(req.MediaIDs) > 0 {
		post.CoverURL = s.firstMediaURL(ctx, req.MediaIDs)
	}

	// ---------- 写入帖子 ----------
	postID, err := s.posts.Create(ctx, post)
	if err != nil {
		return 0, fmt.Errorf("创建帖子失败：%w", err)
	}

	// ---------- 处理标签（查/建/关联/计数） ----------
	if err := s.syncTags(ctx, postID, req.Tags); err != nil {
		return 0, err
	}
	return postID, nil
}

// Update 更新帖子（草稿继续编辑/已发布编辑）。
// 参数：userID 操作者（校验作者）；req 更新内容（nil 字段表示不修改）。
func (s *PostService) Update(ctx context.Context, userID int64, postID int64, req model.UpdatePostReq) error {
	// ---------- 查询并校验作者 ----------
	post, err := s.posts.FindByID(ctx, postID)
	if err != nil {
		return errs.ErrNotFound
	}
	if post.AuthorID != userID {
		return errs.ErrForbidden
	}
	if post.Status == model.PostStatusDeleted {
		return errs.ErrNotFound
	}

	// ---------- 应用更新 ----------
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if utf8.RuneCountInString(title) > maxTitleLen {
			return errs.New(errs.CodeBadRequest, "标题过长")
		}
		post.Title = title
	}
	if req.Content != nil {
		if utf8.RuneCountInString(*req.Content) > maxContentLen {
			return errs.New(errs.CodeBadRequest, "正文不能超过 2000 字")
		}
		post.Content = *req.Content
		post.Summary = buildSummary(*req.Content)
	}
	if req.Visibility != nil {
		if *req.Visibility != model.VisibilityPublic &&
			*req.Visibility != model.VisibilityFollowers &&
			*req.Visibility != model.VisibilityPrivate {
			return errs.New(errs.CodeBadRequest, "可见性不正确")
		}
		post.Visibility = *req.Visibility
	}
	if req.MediaIDs != nil {
		post.MediaIDs = req.MediaIDs
		post.CoverURL = s.firstMediaURL(ctx, req.MediaIDs)
	}

	// ---------- 落库 ----------
	found, err := s.posts.Update(ctx, post)
	if err != nil {
		return fmt.Errorf("更新帖子失败：%w", err)
	}
	if !found {
		return errs.ErrNotFound
	}

	// ---------- 标签重建（仅当显式传入） ----------
	if req.Tags != nil {
		if err := s.syncTags(ctx, postID, req.Tags); err != nil {
			return err
		}
	}
	return nil
}

// Publish 发布草稿（draft → published）。
func (s *PostService) Publish(ctx context.Context, userID int64, postID int64) error {
	post, err := s.posts.FindByID(ctx, postID)
	if err != nil {
		return errs.ErrNotFound
	}
	if post.AuthorID != userID {
		return errs.ErrForbidden
	}
	if post.Status != model.PostStatusDraft {
		return errs.New(errs.CodeStateConflict, "仅草稿可发布")
	}
	now := time.Now()
	return s.posts.SetStatus(ctx, postID, model.PostStatusPublished, now)
}

// Delete 删除帖子（软删，draft/published 均可）。
func (s *PostService) Delete(ctx context.Context, userID int64, postID int64) error {
	post, err := s.posts.FindByID(ctx, postID)
	if err != nil {
		return errs.ErrNotFound
	}
	if post.AuthorID != userID {
		return errs.ErrForbidden
	}
	if post.Status == model.PostStatusDeleted {
		return errs.ErrNotFound
	}
	return s.posts.SetStatus(ctx, postID, model.PostStatusDeleted, nil)
}

// ---------- 时间线 / 草稿箱 ----------

// ListTimeline 时间线分页（全部/图/音/影过滤，最新发布在前）。
// 参数：contentType 类型过滤（空 = 全部）；page/pageSize 分页；viewerID 当前用户（私密帖过滤）。
func (s *PostService) ListTimeline(ctx context.Context, contentType string, page int, pageSize int, viewerID int64) ([]model.PostSummary, int64, error) {
	posts, total, err := s.posts.List(ctx, repository.ListParams{
		ContentType: contentType,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		return nil, 0, err
	}
	// 组装摘要（作者/标签/媒体 + 私密过滤）
	summaries, err := s.assembleSummaries(ctx, posts, viewerID)
	return summaries, total, err
}

// ListDrafts 草稿箱（本人草稿，按更新时间倒序）。
func (s *PostService) ListDrafts(ctx context.Context, userID int64) ([]model.PostSummary, error) {
	posts, err := s.posts.ListDrafts(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.assembleSummaries(ctx, posts, userID)
}

// GetDetail 帖子详情（可见性校验：私密帖仅作者可见）。
// 参数：viewerID 当前用户（0 = 访客）。
func (s *PostService) GetDetail(ctx context.Context, postID int64, viewerID int64) (*model.PostDetail, error) {
	post, err := s.posts.FindByID(ctx, postID)
	if err != nil {
		return nil, errs.ErrNotFound
	}

	// 不可见场景：已删除 / 下架 / 草稿（非作者）/ 私密（非作者）
	visible := post.Status == model.PostStatusPublished ||
		(post.Status == model.PostStatusDraft && post.AuthorID == viewerID)
	if post.Status == model.PostStatusTakenDown || post.Status == model.PostStatusDeleted {
		visible = false
	}
	if post.Visibility == model.VisibilityPrivate && post.AuthorID != viewerID {
		visible = false
	}
	// 仅关注者帖：需登录且与作者互相关注（设计稿：互相关注的人可见）
	if visible && post.Visibility == model.VisibilityFollowers && post.AuthorID != viewerID {
		if viewerID == 0 || !s.isMutual(ctx, viewerID, post.AuthorID) {
			visible = false
		}
	}
	if !visible {
		return nil, errs.ErrNotFound
	}

	// 浏览量 +1（忽略失败）
	_ = s.posts.IncrView(ctx, postID)

	// 组装详情
	detail := &model.PostDetail{
		PostSummary: model.PostSummary{
			ID:           post.ID,
			Title:        post.Title,
			Summary:      post.Summary,
			ContentType:  post.ContentType,
			Visibility:   post.Visibility,
			LikeCount:    post.LikeCount,
			CommentCount: post.CommentCount,
			ViewCount:    post.ViewCount + 1,
		},
		Content:  post.Content,
		IsAuthor: post.AuthorID == viewerID,
		CanView:  true,
	}
	if post.PublishedAt != nil {
		detail.PublishedAt = post.PublishedAt.Format(time.RFC3339)
	}

	// 作者 / 标签 / 媒体
	if err := s.fillAuthor(ctx, &detail.PostSummary, post.AuthorID); err != nil {
		return nil, err
	}
	if err := s.fillTags(ctx, &detail.PostSummary, post.ID); err != nil {
		return nil, err
	}
	if err := s.fillMedia(ctx, &detail.PostSummary, post.MediaIDs); err != nil {
		return nil, err
	}
	return detail, nil
}

// ---------- 媒体上传 ----------

// UploadMedia 媒体上传：本地磁盘保存 + media_assets 记录。
// 参数：userID 上传者；header 文件头；reader 文件内容。
// 返回：上传结果（含访问地址）。
func (s *PostService) UploadMedia(ctx context.Context, userID int64, header *multipart.FileHeader, reader multipart.File) (model.UploadResult, error) {
	// 保存到本地磁盘（类型/大小校验在 store 内完成）
	result, err := s.store.Save(header, reader)
	if err != nil {
		if errors.Is(err, media.ErrUnsupportedType) {
			return model.UploadResult{}, errs.New(errs.CodeBadRequest, "不支持的文件类型，请上传图片（jpg/png/gif/webp）、音频（mp3/m4a/wav）或视频（mp4/mov/webm）")
		}
		if errors.Is(err, media.ErrFileTooLarge) {
			return model.UploadResult{}, errs.New(errs.CodeBadRequest, "文件超出大小限制（图片 10MB / 音频 20MB / 视频 200MB）")
		}
		return model.UploadResult{}, err
	}

	// 写入 media_assets 记录
	mediaID, err := s.medias.Create(ctx, repository.MediaAsset{
		OwnerID:    userID,
		Type:       result.Type,
		StorageKey: result.StorageKey,
		URL:        result.URL,
		MimeType:   result.MimeType,
		SizeBytes:  result.SizeBytes,
		Status:     "ready",
	})
	if err != nil {
		return model.UploadResult{}, fmt.Errorf("记录媒体失败：%w", err)
	}

	return model.UploadResult{
		ID:        mediaID,
		Type:      result.Type,
		URL:       result.URL,
		MimeType:  result.MimeType,
		SizeBytes: result.SizeBytes,
	}, nil
}

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
	"io"
	"mime/multipart"
	"os"
	"time"

	"github.com/roberts9012062/boke/internal/media"
	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/plugin"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// 帖子正文与标签限制（需求 3.4；文章形态放宽正文上限）。
const (
	maxContentLen       = 2000  // 说说正文上限（字）
	maxArticleContentLen = 20000 // 文章正文上限（字）
	maxTags             = 5     // 标签上限（个）
	maxTagLen           = 20    // 单个标签上限（字符）
	maxTitleLen         = 100   // 标题上限
	summaryLen          = 100   // 摘要落库截断长度（字符）
	summaryPreview      = 60    // 说说列表摘要预览长度（字符）
	articlePreviewLen   = 200   // 文章列表摘要预览长度（字符；时间轴展示 200 字，过长省略）
)

// PostService 帖子服务（连接器类，聚合依赖）。
type PostService struct {
	posts       *repository.PostRepo               // 帖子数据访问
	tags        *repository.TagRepo                // 标签数据访问
	medias      *repository.MediaRepo              // 媒体数据访问
	users       *repository.UserRepo               // 用户数据访问（作者信息）
	store       *media.Store                       // 媒体存储（本地磁盘）
	moderation  *ModerationService                 // 内容治理（M2：敏感词拦截）
	relations   *repository.RelationRepo           // 用户关系（M2：仅关注者帖互关判断）
	hooks       plugin.Dispatcher                  // 插件钩子调度器（M3.2 扩展框架）
	seo         *repository.SeoRepo                // SEO 元数据（M4.1 插件通道：发帖/编辑落库）
	storageSeam func() (plugin.MediaStorage, bool) // 媒体存储 seam（图床插件接管上传；可空=始终本地）
	relayHook   func(postID int64)                 // 中继站发布钩子（大世界推送；可空=未启用）
	relayDeleteHook func(postID int64)             // 中继站删除钩子（删帖下架；可空=未启用）
	relayChoiceCheck func(ctx context.Context, input model.RelayPublishInput) error // 中继站发布选择校验（可空=跳过校验）
}

// SetRelayHook 注入中继站发布钩子（装配期调用；发帖成功后异步推送大世界）。
func (s *PostService) SetRelayHook(fn func(postID int64)) { s.relayHook = fn }

// SetRelayChoiceCheck 注入中继站发布选择校验（装配期调用；勾选公开发送时校验分类/标签）。
// PostService 不直接依赖 RelayRepo（分层解耦，与 relayHook 同模式）；校验查库需随请求 ctx。
func (s *PostService) SetRelayChoiceCheck(fn func(ctx context.Context, input model.RelayPublishInput) error) {
	s.relayChoiceCheck = fn
}

// SetRelayDeleteHook 注入中继站删除钩子（装配期调用；删帖后异步下架大世界内容）。
func (s *PostService) SetRelayDeleteHook(fn func(postID int64)) { s.relayDeleteHook = fn }

// NewPostService 创建帖子服务。
// 参数：storageSeam 媒体存储 seam 查找闭包（可空——图床插件运行时上传直达外部对象存储）。
func NewPostService(
	posts *repository.PostRepo,
	tags *repository.TagRepo,
	medias *repository.MediaRepo,
	users *repository.UserRepo,
	store *media.Store,
	moderation *ModerationService,
	relations *repository.RelationRepo,
	hooks plugin.Dispatcher,
	seo *repository.SeoRepo,
	storageSeam func() (plugin.MediaStorage, bool),
) *PostService {
	return &PostService{posts: posts, tags: tags, medias: medias, users: users, store: store, moderation: moderation, relations: relations, hooks: hooks, seo: seo, storageSeam: storageSeam}
}

// ---------- 发帖 / 草稿 ----------

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
	if err := s.posts.SetStatus(ctx, postID, model.PostStatusPublished, now); err != nil {
		return err
	}
	// ---------- 中继站（大世界）：草稿发布同样触发推送（修复既有缺口：此前仅直接发帖推送） ----------
	if s.relayHook != nil && post.RelayPublish && post.Visibility == "public" {
		s.relayHook(postID)
	}
	// ---------- 插件钩子：post.after_publish（异步，M3.2） ----------
	s.hooks.Dispatch(ctx, plugin.HookPostAfterPublish, plugin.Event{
		ActorID: userID,
		Payload: post,
	})
	return nil
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
	if err := s.posts.SetStatus(ctx, postID, model.PostStatusDeleted, nil); err != nil {
		return err
	}
	// 中继站（大世界）：曾公开发布的帖子删除后同步下架（失败仅日志，不影响本地删除）
	if s.relayDeleteHook != nil && post.Status == model.PostStatusPublished && post.Visibility == "public" {
		s.relayDeleteHook(postID)
	}
	return nil
}

// ---------- 时间线 / 草稿箱 ----------

// ListTimeline 时间线分页（全部/图/音/影 + 文章形态过滤，最新发布在前）。
// 参数：contentType 类型过滤（空 = 全部）；kind 帖子形态过滤（空 = 全部形态）；
// page/pageSize 分页；viewerID 当前用户（私密帖过滤）。
func (s *PostService) ListTimeline(ctx context.Context, contentType string, kind string, page int, pageSize int, viewerID int64) ([]model.PostSummary, int64, error) {
	posts, total, err := s.posts.List(ctx, repository.ListParams{
		ContentType: contentType,
		Kind:        kind,
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
func (s *PostService) GetDetail(ctx context.Context, postID int64, viewerID int64, guestToken string) (*model.PostDetail, error) {
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

	// 浏览量 +1 + 浏览埋点（P1 真实日浏览统计；失败均忽略不影响详情展示）
	_ = s.posts.IncrView(ctx, postID)
	_ = s.posts.RecordView(ctx, postID, s.viewerHash(viewerID, guestToken))

	// 组装详情
	detail := &model.PostDetail{
		PostSummary: model.PostSummary{
			ID:           post.ID,
			Title:        post.Title,
			Summary:      post.Summary,
			ContentType:  post.ContentType,
			PostKind:     post.PostKind,
			Visibility:   post.Visibility,
			GalleryStyle: post.GalleryStyle,
			LikeCount:    post.LikeCount,
			CommentCount: post.CommentCount,
			ViewCount:    post.ViewCount + 1,
		},
		Content:       post.Content,
		ContentFormat: post.ContentFormat,
		IsAuthor:      post.AuthorID == viewerID,
		CanView:       true,
		Relay: model.RelayPublishInput{
			Publish:  post.RelayPublish,
			Category: post.RelayCategory,
			Tags:     post.RelayTags,
		},
	}

	// ---------- 插件钩子：content.render（M3.9 同步，可改写正文；失败/拒绝不影响展示） ----------
	if s.hooks != nil {
		if res := s.hooks.Dispatch(ctx, plugin.HookContentRender, plugin.Event{
			ActorID: viewerID,
			Payload: map[string]any{"post_id": post.ID, "content": post.Content},
		}); res.OK {
			if modified, ok := res.Modify.(map[string]any); ok {
				if content, ok := modified["content"].(string); ok {
					detail.Content = content
				}
			}
		}
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

	// SEO 输出（M4.1 插件通道：详情页 robots 收录策略/自定义标题描述；查询失败静默）
	if s.seo != nil {
		if meta, err := s.seo.GetMeta(ctx, post.ID); err == nil {
			detail.Seo = &model.PostSeoOutput{
				Title: meta.Title, Description: meta.Description,
				URLAlias: meta.URLAlias, Robots: meta.Robots,
			}
		}
	}
	return detail, nil
}

// ---------- 媒体上传 ----------

// UploadMedia 媒体上传：存储保存 + media_assets 记录。
// 存储优先级：图床插件 seam（media.storage，上传直达外部对象存储）→ 本地磁盘兜底；
// 类型/大小校验两条路径同规则前置（上传白名单是安全边界，外部存储不豁免）。
// 参数：userID 上传者；header 文件头；reader 文件内容。
// 返回：上传结果（含访问地址）。
func (s *PostService) UploadMedia(ctx context.Context, userID int64, header *multipart.FileHeader, reader multipart.File) (model.UploadResult, error) {
	var result media.StorageResult
	var err error
	if s.storageSeam != nil {
		if storage, ok := s.storageSeam(); ok {
			result, err = saveViaSeam(ctx, storage, header, reader)
			if err != nil {
				// seam 失败回退本地（图床不可达/未配对不阻断上传），留痕排查
				fmt.Fprintf(os.Stderr, "[media] 图床上传失败，回退本地：%v\n", err)
			}
		}
	}
	if result.URL == "" {
		// 本地磁盘保存（类型/大小校验在 store 内完成）
		if seeker, ok := reader.(io.Seeker); ok {
			_, _ = seeker.Seek(0, io.SeekStart) // seam 路径已读过内容：回卷供本地保存
		}
		result, err = s.store.Save(header, reader)
	}
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

// saveViaSeam 经图床插件保存：读全量内容 → 类型/大小前置校验 → seam 存储转发。
// 仅图片类型走图床（Worker 契约白名单 jpg/png/gif/webp；音频/视频体积大仍走本地）；
// 非图片返回零值结果（调用方按 URL 为空走本地分支），不视为错误。
func saveViaSeam(ctx context.Context, storage plugin.MediaStorage, header *multipart.FileHeader, reader multipart.File) (media.StorageResult, error) {
	mediaType, err := media.DetectType(header.Filename, header.Header.Get("Content-Type"))
	if err != nil {
		return media.StorageResult{}, err
	}
	if mediaType != media.TypeImage {
		return media.StorageResult{}, nil
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		return media.StorageResult{}, err
	}
	if int64(len(content)) > media.MaxSizeFor(mediaType) {
		return media.StorageResult{}, media.ErrFileTooLarge
	}
	return storage.Save(ctx, header.Filename, header.Header.Get("Content-Type"), content)
}

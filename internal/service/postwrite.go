// 帖子创建与更新（从 post.go 拆出）：草稿/直发编排、媒体关联与中继站发布选择落地。
package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/plugin"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// Create 创建帖子（status=draft 存草稿；published 发布）。
// 返回：新帖子 ID。
func (s *PostService) Create(ctx context.Context, userID int64, req model.CreatePostReq) (int64, error) {
	// ---------- 参数校验（服务端二次校验） ----------
	if err := validatePostReq(req); err != nil {
		return 0, err
	}

	// ---------- 中继站发布选择校验（勾选公开发送时：分类必填 + 标签规范） ----------
	if req.Relay != nil && s.relayChoiceCheck != nil {
		if err := s.relayChoiceCheck(ctx, *req.Relay); err != nil {
			return 0, err
		}
	}

	// ---------- 敏感词拦截（M2：发布时命中 forbidden 直接拒绝；P1：命中计数） ----------
	if word := s.moderation.CheckForbidden(req.Content); word != "" {
		s.moderation.IncrHit(ctx, word)
		return 0, errs.New(errs.CodeBadRequest, "内容包含敏感词「"+word+"」，请修改后发布")
	}

	// ---------- 插件钩子：post.before_publish（同步，可拦截；M3.2 扩展框架） ----------
	if res := s.hooks.Dispatch(ctx, plugin.HookPostBeforePublish, plugin.Event{
		ActorID: userID,
		Payload: req,
	}); !res.OK {
		return 0, errs.New(errs.CodeValidation, res.Reason)
	}

	// 状态校验：仅允许 draft / published
	if req.Status != model.PostStatusDraft && req.Status != model.PostStatusPublished {
		return 0, errs.New(errs.CodeBadRequest, "帖子状态不正确")
	}

	// ---------- 组装实体 ----------
	post := model.Post{
		AuthorID:      userID,
		Title:         strings.TrimSpace(req.Title),
		Summary:       buildSummary(req.Content),
		Content:       req.Content,
		ContentFormat: normalizeContentFormat(req.ContentFormat),
		ContentType:   req.ContentType,
		PostKind:      normalizePostKind(req.PostKind),
		Status:        req.Status,
		Visibility:    req.Visibility,
		GalleryStyle:  req.GalleryStyle,
		MediaIDs:      req.MediaIDs,
	}
	// 中继站发布选择（用户显式勾选；nil=不发送）
	if req.Relay != nil && req.Relay.Publish {
		post.RelayPublish = true
		post.RelayCategory = strings.TrimSpace(req.Relay.Category)
		post.RelayTags = req.Relay.Tags
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

	// ---------- SEO 元数据（M4.1 插件通道：发帖 SEO 面板提交；别名冲突返回友好错误） ----------
	if req.Seo != nil && s.seo != nil {
		if err := s.seo.UpsertMeta(ctx, repository.SeoMeta{
			PostID: postID, Title: req.Seo.SEOTitle, Description: req.Seo.SEODescription,
			URLAlias: req.Seo.URLAlias, Robots: req.Seo.Robots,
		}); err != nil {
			return 0, errs.New(errs.CodeConflict, err.Error())
		}
	}

	// ---------- 处理标签（查/建/关联/计数） ----------
	if err := s.syncTags(ctx, postID, req.Tags); err != nil {
		return 0, err
	}
	// ---------- 中继站（大世界）：用户勾选公开发送 + 公开已发布 → 异步推送（失败仅日志，幂等可重试） ----------
	if s.relayHook != nil && post.RelayPublish && post.Status == "published" && post.Visibility == "public" {
		s.relayHook(postID)
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
		if err := validateContentByKind(post.PostKind, *req.Content); err != nil {
			return err
		}
		post.Content = *req.Content
		post.Summary = buildSummary(*req.Content)
	}
	if req.ContentFormat != nil {
		post.ContentFormat = normalizeContentFormat(*req.ContentFormat)
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
	if req.GalleryStyle != nil {
		post.GalleryStyle = *req.GalleryStyle
	}
	// 中继站发布选择（编辑时可改；仅当显式传入）
	if req.Relay != nil {
		if s.relayChoiceCheck != nil {
			if err := s.relayChoiceCheck(ctx, *req.Relay); err != nil {
				return err
			}
		}
		post.RelayPublish = req.Relay.Publish
		post.RelayCategory = strings.TrimSpace(req.Relay.Category)
		post.RelayTags = req.Relay.Tags
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

	// ---------- SEO 元数据（M4.1 插件通道：编辑 SEO 面板提交；别名冲突返回友好错误） ----------
	if req.Seo != nil && s.seo != nil {
		if err := s.seo.UpsertMeta(ctx, repository.SeoMeta{
			PostID: postID, Title: req.Seo.SEOTitle, Description: req.Seo.SEODescription,
			URLAlias: req.Seo.URLAlias, Robots: req.Seo.Robots,
		}); err != nil {
			return errs.New(errs.CodeConflict, err.Error())
		}
	}
	return nil
}


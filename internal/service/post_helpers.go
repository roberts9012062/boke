// internal/service/post_helpers.go
// 帖子「写路径」内部辅助：参数校验、摘要生成、封面取址、标签同步。
// 说明（M1.7 拆分）：post.go 超 400 行规范，将纯函数与标签同步逻辑独立成文件。
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/plugin"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// viewerHash 生成浏览埋点的访客标识（登录用户 = ID 的 SHA256；匿名 = 带 guest token 的 SHA256）。
// 说明：不落原始用户 ID，日 UV 聚合用哈希去重；匿名访客带本地 guest token（P1 完善：
//       此前匿名统一空串无法区分访客，现可区分且同人同日去重）；两者皆无时返回空串（降级）。
func (s *PostService) viewerHash(viewerID int64, guestToken string) string {
	if viewerID != 0 {
		sum := sha256.Sum256([]byte(strconv.FormatInt(viewerID, 10)))
		return hex.EncodeToString(sum[:])
	}
	if guestToken != "" {
		sum := sha256.Sum256([]byte("g:" + guestToken))
		return hex.EncodeToString(sum[:])
	}
	return ""
}

// normalizeContentFormat 归一化正文格式（空/未知 → markdown；html 保留）。
// 说明：旧客户端不传时默认 markdown，兼容存量帖子；纯函数。
func normalizeContentFormat(format string) string {
	if format == "html" {
		return "html"
	}
	return "markdown"
}

// normalizePostKind 归一化帖子形态（空 → moment 说说；article 保留；纯函数）。
// 说明：旧客户端不传时默认说说，兼容存量帖子。
func normalizePostKind(kind string) string {
	if kind == model.PostKindArticle {
		return model.PostKindArticle
	}
	return model.PostKindMoment
}

// validateContentByKind 按帖子形态校验正文字数（纯函数）。
// 说明：说说 ≤2000 字；文章放宽到 ≤20000 字（按纯文本计——HTML 标签不占字数）。
func validateContentByKind(kind string, content string) error {
	limit := maxContentLen
	if kind == model.PostKindArticle {
		limit = maxArticleContentLen
	}
	if utf8.RuneCountInString(plainText(content)) > limit {
		return errs.New(errs.CodeBadRequest, fmt.Sprintf("正文不能超过 %d 字", limit))
	}
	return nil
}

// validatePostReq 校验发帖参数（形态/类型/字数/标签/可见性）。
func validatePostReq(req model.CreatePostReq) error {
	// 帖子形态：说说/文章（归一化后校验，article 有额外约束）
	kind := normalizePostKind(req.PostKind)
	// 内容类型：文字/图片/音频/视频（视频 M2 已开放）；文章固定 text（图片走图集/正文内嵌）
	switch req.ContentType {
	case model.PostTypeText, model.PostTypeImage, model.PostTypeAudio, model.PostTypeVideo:
	default:
		return errs.New(errs.CodeBadRequest, "帖子类型不正确")
	}
	if kind == model.PostKindArticle && req.ContentType != model.PostTypeText {
		return errs.New(errs.CodeBadRequest, "文章类型不正确")
	}
	// 文章：标题必填（说说标题可选）；标题统一 ≤100 字
	if kind == model.PostKindArticle && strings.TrimSpace(req.Title) == "" {
		return errs.New(errs.CodeBadRequest, "文章标题不能为空")
	}
	if utf8.RuneCountInString(strings.TrimSpace(req.Title)) > maxTitleLen {
		return errs.New(errs.CodeBadRequest, "标题过长")
	}
	// 正文按形态限长（说说 ≤2000 字 / 文章 ≤20000 字，按纯文本计）
	if err := validateContentByKind(kind, req.Content); err != nil {
		return err
	}
	// 标签 ≤5 个，每个 ≤20 字符
	if len(req.Tags) > maxTags {
		return errs.New(errs.CodeBadRequest, "标签最多 5 个")
	}
	for _, tag := range req.Tags {
		if utf8.RuneCountInString(tag) > maxTagLen {
			return errs.New(errs.CodeBadRequest, "单个标签不能超过 20 个字符")
		}
	}
	// 可见性：公开/仅关注者/仅自己（设计稿《可见性》弹层三选项）
	if req.Visibility != model.VisibilityPublic &&
		req.Visibility != model.VisibilityFollowers &&
		req.Visibility != model.VisibilityPrivate {
		return errs.New(errs.CodeBadRequest, "可见性不正确")
	}
	return nil
}

// buildSummary 生成摘要：剥离 HTML/Markdown 标记后取正文前 100 字符（去除换行）。
func buildSummary(content string) string {
	flat := strings.Join(strings.Fields(plainText(content)), " ")
	runes := []rune(flat)
	if len(runes) > summaryLen {
		return string(runes[:summaryLen])
	}
	return flat
}

// firstMediaURL 取媒体列表第一项的访问地址（封面用）。
func (s *PostService) firstMediaURL(ctx context.Context, mediaIDs []int64) string {
	if len(mediaIDs) == 0 {
		return ""
	}
	assets, err := s.medias.FindByIDs(ctx, mediaIDs)
	if err != nil || len(assets) == 0 {
		return ""
	}
	return assets[0].URL
}

// UpdateByAdmin 后台编辑帖子（管理员权限，不校验作者；设计稿《后台编辑》四画板）。
// 参数：postID 目标帖子；req 更新内容（status 决定按钮语义：draft=保存草稿 / published=更新发布）。
func (s *PostService) UpdateByAdmin(ctx context.Context, postID int64, req model.AdminUpdatePostReq) error {
	// ---------- 查询（已删除帖不可编辑） ----------
	post, err := s.posts.FindByID(ctx, postID)
	if err != nil {
		return errs.ErrNotFound
	}
	if post.Status == model.PostStatusDeleted {
		return errs.ErrNotFound
	}

	// ---------- 参数校验（与前台发帖规则一致） ----------
	if utf8.RuneCountInString(req.Title) > maxTitleLen {
		return errs.New(errs.CodeBadRequest, "标题过长")
	}
	// 正文按原帖形态限长（说说 ≤2000 字 / 文章 ≤20000 字；形态不可变，沿用创建时值）
	if err := validateContentByKind(post.PostKind, req.Content); err != nil {
		return err
	}
	if len(req.Tags) > maxTags {
		return errs.New(errs.CodeBadRequest, "标签最多 5 个")
	}
	for _, tag := range req.Tags {
		if utf8.RuneCountInString(tag) > maxTagLen {
			return errs.New(errs.CodeBadRequest, "单个标签不能超过 20 个字符")
		}
	}
	if req.Visibility != model.VisibilityPublic &&
		req.Visibility != model.VisibilityFollowers &&
		req.Visibility != model.VisibilityPrivate {
		return errs.New(errs.CodeBadRequest, "可见性不正确")
	}
	if req.Status != model.PostStatusDraft && req.Status != model.PostStatusPublished {
		return errs.New(errs.CodeBadRequest, "状态仅支持 draft / published")
	}

	// ---------- 敏感词拦截（与前台发帖一致，防后台绕过发布违规内容；P1：命中计数） ----------
	if word := s.moderation.CheckForbidden(req.Content); word != "" {
		s.moderation.IncrHit(ctx, word)
		return errs.New(errs.CodeBadRequest, "内容包含敏感词「"+word+"」，请修改后保存")
	}

	// ---------- 插件钩子：post.before_publish（同步，可拦截；M3.2 扩展框架） ----------
	if res := s.hooks.Dispatch(ctx, plugin.HookPostBeforePublish, plugin.Event{
		ActorID: 0, // 后台操作者（管理员）
		Payload: post,
	}); !res.OK {
		return errs.New(errs.CodeValidation, res.Reason)
	}

	// ---------- 应用更新（内容类型不允许修改，保留原值） ----------
	post.Title = strings.TrimSpace(req.Title)
	post.Summary = buildSummary(req.Content)
	post.Content = req.Content
	post.ContentFormat = normalizeContentFormat(req.ContentFormat)
	post.Visibility = req.Visibility
	post.MediaIDs = req.MediaIDs
	// 封面策略：显式传 cover_url 优先（视频帖「更换封面」）；否则图片帖取第一张，其余保留原值
	if req.CoverURL != nil {
		post.CoverURL = *req.CoverURL
	} else if post.ContentType == model.PostTypeImage {
		post.CoverURL = s.firstMediaURL(ctx, req.MediaIDs)
	}

	found, err := s.posts.Update(ctx, post)
	if err != nil {
		return fmt.Errorf("更新帖子失败：%w", err)
	}
	if !found {
		return errs.ErrNotFound
	}

	// ---------- 标签重建 ----------
	if err := s.syncTags(ctx, postID, req.Tags); err != nil {
		return err
	}

	// ---------- 状态变更（保存草稿 → draft；更新发布 → published） ----------
	if req.Status == model.PostStatusPublished && post.Status != model.PostStatusPublished {
		now := time.Now()
		if err := s.posts.SetStatus(ctx, postID, model.PostStatusPublished, now); err != nil {
			return err
		}
	} else if req.Status == model.PostStatusDraft && post.Status != model.PostStatusDraft {
		if err := s.posts.SetStatus(ctx, postID, model.PostStatusDraft, nil); err != nil {
			return err
		}
	}
	return nil
}

// syncTags 同步帖子标签（先解除旧关联再关联新标签，计数相应增减）。
func (s *PostService) syncTags(ctx context.Context, postID int64, tags []string) error {
	// 解除旧关联（计数 -1）
	oldTags, err := s.tags.ListByPost(ctx, postID)
	if err != nil {
		return err
	}
	if len(oldTags) > 0 {
		if err := s.tags.UnlinkPost(ctx, postID); err != nil {
			return err
		}
		for _, old := range oldTags {
			if err := s.tags.DecrPostCount(ctx, old.ID); err != nil {
				return err
			}
		}
	}

	// 关联新标签（去重）
	seen := make(map[string]bool)
	for _, raw := range tags {
		name := strings.TrimPrefix(strings.TrimSpace(raw), "#")
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true

		// 查或建标签
		tagID, err := s.tags.FindByName(ctx, name)
		if errors.Is(err, repository.ErrNotFound) {
			tagID, err = s.tags.Create(ctx, name, name)
		}
		if err != nil {
			return err
		}
		// 关联 + 计数
		if err := s.tags.LinkPost(ctx, postID, tagID); err != nil {
			return err
		}
		if err := s.tags.IncrPostCount(ctx, tagID); err != nil {
			return err
		}
	}
	return nil
}

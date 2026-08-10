// internal/service/post_helpers.go
// 帖子「写路径」内部辅助：参数校验、摘要生成、封面取址、标签同步。
// 说明（M1.7 拆分）：post.go 超 400 行规范，将纯函数与标签同步逻辑独立成文件。
package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// validatePostReq 校验发帖参数（类型/字数/标签/可见性）。
func validatePostReq(req model.CreatePostReq) error {
	// 内容类型：文字/图片/音频/视频（视频 M2 已开放）
	switch req.ContentType {
	case model.PostTypeText, model.PostTypeImage, model.PostTypeAudio, model.PostTypeVideo:
	default:
		return errs.New(errs.CodeBadRequest, "帖子类型不正确")
	}
	// 正文 ≤2000 字
	if utf8.RuneCountInString(req.Content) > maxContentLen {
		return errs.New(errs.CodeBadRequest, "正文不能超过 2000 字")
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

// buildSummary 生成摘要：正文前 100 字符（去除换行）。
func buildSummary(content string) string {
	flat := strings.Join(strings.Fields(content), " ")
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

// internal/service/post_assembly.go
// 帖子「读路径」组装：摘要/详情组装（作者、标签、媒体、收藏数）。
// 说明（M1.7 拆分）：post.go 超 400 行规范，将组装逻辑独立成文件，职责单一：
//   - post.go：写路径（CRUD/时间线/详情可见性/媒体上传/标签同步）
//   - post_assembly.go：读路径（PostSummary/PostDetail 的字段组装）
package service

import (
	"context"
	"strings"
	"time"

	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/pkg/errs"
)

// assembleSummaries 批量组装帖子摘要（作者/标签/媒体 + 私密帖过滤 + 收藏数聚合）。
func (s *PostService) assembleSummaries(ctx context.Context, posts []model.Post, viewerID int64) ([]model.PostSummary, error) {
	summaries := make([]model.PostSummary, 0, len(posts))
	for _, post := range posts {
		// 私密帖过滤：非作者不展示
		if post.Visibility == model.VisibilityPrivate && post.AuthorID != viewerID {
			continue
		}
		// 仅关注者帖过滤：需与作者互相关注（设计稿：互相关注的人可见）
		if post.Visibility == model.VisibilityFollowers && post.AuthorID != viewerID {
			if viewerID == 0 || !s.isMutual(ctx, viewerID, post.AuthorID) {
				continue
			}
		}
		summary := model.PostSummary{
			ID:           post.ID,
			Title:        post.Title,
			Summary:      summaryPreviewText(post.Content),
			ContentType:  post.ContentType,
			Visibility:   post.Visibility,
			LikeCount:    post.LikeCount,
			CommentCount: post.CommentCount,
			ViewCount:    post.ViewCount,
		}
		if post.PublishedAt != nil {
			summary.PublishedAt = post.PublishedAt.Format(time.RFC3339)
		}
		if err := s.fillAuthor(ctx, &summary, post.AuthorID); err != nil {
			return nil, err
		}
		if err := s.fillTags(ctx, &summary, post.ID); err != nil {
			return nil, err
		}
		if err := s.fillMedia(ctx, &summary, post.MediaIDs); err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	// 批量补齐收藏数（M1.7 技术债修复：一次查询填充全部摘要，替代逐条 N+1）
	if err := s.fillFavoriteCounts(ctx, summaries); err != nil {
		return nil, err
	}
	return summaries, nil
}

// fillFavoriteCounts 批量填充摘要收藏数（post_reactions 一次聚合）。
func (s *PostService) fillFavoriteCounts(ctx context.Context, summaries []model.PostSummary) error {
	ids := make([]int64, 0, len(summaries))
	for _, summary := range summaries {
		ids = append(ids, summary.ID)
	}
	counts, err := s.posts.CountFavoritesByPosts(ctx, ids)
	if err != nil {
		return err
	}
	for i := range summaries {
		summaries[i].FavoriteCount = counts[summaries[i].ID]
	}
	return nil
}

// fillAuthor 填充作者信息（复用用户仓库）。
func (s *PostService) fillAuthor(ctx context.Context, summary *model.PostSummary, authorID int64) error {
	user, err := s.users.FindByID(ctx, authorID)
	if err != nil {
		return err
	}
	summary.Author = model.AuthorDTO{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		AvatarURL: user.AvatarURL,
	}
	return nil
}

// fillTags 填充标签列表。
func (s *PostService) fillTags(ctx context.Context, summary *model.PostSummary, postID int64) error {
	tags, err := s.tags.ListByPost(ctx, postID)
	if err != nil {
		return err
	}
	summary.Tags = make([]model.TagDTO, 0, len(tags))
	for _, tag := range tags {
		summary.Tags = append(summary.Tags, model.TagDTO{Name: "#" + tag.Name, Slug: tag.Slug})
	}
	return nil
}

// fillMedia 填充媒体列表（按 media_ids 顺序，灯箱多图有序）。
func (s *PostService) fillMedia(ctx context.Context, summary *model.PostSummary, mediaIDs []int64) error {
	assets, err := s.medias.FindByIDs(ctx, mediaIDs)
	if err != nil {
		return err
	}
	summary.Media = make([]model.MediaDTO, 0, len(assets))
	for _, asset := range assets {
		summary.Media = append(summary.Media, model.MediaDTO{
			ID:        asset.ID,
			Type:      asset.Type,
			URL:       asset.URL,
			MimeType:  asset.MimeType,
			SizeBytes: asset.SizeBytes,
			Width:     asset.Width,
			Height:    asset.Height,
		})
	}
	return nil
}

// GetAdminDetail 后台编辑详情（设计稿《后台编辑》四画板：编辑区/发布信息/互动数据/操作）。
// 与前台 GetDetail 的区别：不校验可见性（管理员可查看下架/私密帖）、不增加浏览量。
func (s *PostService) GetAdminDetail(ctx context.Context, postID int64) (*model.AdminPostDetail, error) {
	post, err := s.posts.FindByID(ctx, postID)
	if err != nil {
		return nil, errs.ErrNotFound
	}
	if post.Status == model.PostStatusDeleted {
		return nil, errs.ErrNotFound
	}

	// 作者 / 标签 / 媒体（复用前台组装，避免重复实现）
	var summary model.PostSummary
	if err := s.fillAuthor(ctx, &summary, post.AuthorID); err != nil {
		return nil, err
	}
	if err := s.fillTags(ctx, &summary, post.ID); err != nil {
		return nil, err
	}
	if err := s.fillMedia(ctx, &summary, post.MediaIDs); err != nil {
		return nil, err
	}
	// 标签去掉 # 前缀（表单直接编辑，保存时再补 #）
	tagNames := make([]string, 0, len(summary.Tags))
	for _, tag := range summary.Tags {
		tagNames = append(tagNames, strings.TrimPrefix(tag.Name, "#"))
	}

	detail := &model.AdminPostDetail{
		ID:           post.ID,
		Title:        post.Title,
		Content:      post.Content,
		ContentType:  post.ContentType,
		Status:       post.Status,
		Visibility:   post.Visibility,
		CoverURL:     post.CoverURL,
		Tags:         tagNames,
		Media:        summary.Media,
		ViewCount:    post.ViewCount,
		LikeCount:    post.LikeCount,
		CommentCount: post.CommentCount,
		Author:       summary.Author,
		CreatedAt:    post.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    post.UpdatedAt.Format(time.RFC3339),
	}
	if post.PublishedAt != nil {
		detail.PublishedAt = post.PublishedAt.Format(time.RFC3339)
	}
	return detail, nil
}

// isMutual 判断 viewer 与 author 是否互相关注（仅关注者帖可见性，设计稿「互相关注的人可见」）。
func (s *PostService) isMutual(ctx context.Context, viewerID int64, authorID int64) bool {
	following, err := s.relations.IsFollowing(ctx, viewerID, authorID)
	if err != nil || !following {
		return false
	}
	followedBack, err := s.relations.IsFollowing(ctx, authorID, viewerID)
	return err == nil && followedBack
}

// summaryPreviewText 列表摘要：正文前 60 字符（替换换行为空格，省略号结尾）。
func summaryPreviewText(content string) string {
	flat := strings.Join(strings.Fields(content), " ")
	runes := []rune(flat)
	if len(runes) > summaryPreview {
		return string(runes[:summaryPreview]) + "…"
	}
	return flat
}

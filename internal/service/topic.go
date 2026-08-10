// internal/service/topic.go
// 话题业务逻辑：话题列表/详情/帖子流/关注（需求 3.6）。
//
// 语义：帖子带 # 标签即进入话题聚合（tags + post_tags）；话题 = 标签。
// 话题详情统计：帖子数（post_count 冗余）、关注数（topic_follows COUNT）、
// 浏览数（该话题帖子 view_count 求和，MVP 简化）。
package service

import (
	"context"
	"errors"

	"github.com/yueyan/boke/internal/model"
	"github.com/yueyan/boke/internal/repository"
	"github.com/yueyan/boke/pkg/errs"
)

// TopicDTO 话题信息（列表/详情展示）。
type TopicDTO struct {
	Name        string `json:"name"`         // 话题名（含 #）
	Slug        string `json:"slug"`         // URL 别名
	Description string `json:"description"`  // 描述
	PostCount   int64  `json:"post_count"`   // 帖子数
	FollowCount int64  `json:"follow_count"` // 关注数
	BrowseCount int64  `json:"browse_count"` // 浏览数（帖子浏览量求和）
	Following   bool   `json:"following"`    // 当前用户是否已关注
}

// TopicService 话题服务（连接器类）。
type TopicService struct {
	tags    *repository.TagRepo    // 标签/话题数据访问
	posts   *repository.PostRepo   // 帖子数据访问（帖子流）
	postSvc *PostService           // 帖子服务（摘要组装）
}

// NewTopicService 创建话题服务。
func NewTopicService(tags *repository.TagRepo, posts *repository.PostRepo, postSvc *PostService) *TopicService {
	return &TopicService{tags: tags, posts: posts, postSvc: postSvc}
}

// List 话题列表（按帖数降序，热门在前）。
func (s *TopicService) List(ctx context.Context, viewerID int64) ([]TopicDTO, error) {
	rows, err := s.tags.ListHot(ctx, 30)
	if err != nil {
		return nil, err
	}
	topics := make([]TopicDTO, 0, len(rows))
	for _, t := range rows {
		topics = append(topics, s.toDTO(ctx, t, viewerID))
	}
	return topics, nil
}

// Detail 话题详情（含关注状态）。
func (s *TopicService) Detail(ctx context.Context, name string, viewerID int64) (*TopicDTO, error) {
	tag, err := s.tags.FindWithStats(ctx, name)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}
	dto := s.toDTO(ctx, tag, viewerID)
	return &dto, nil
}

// Posts 话题帖子流（分页；sort=latest 最新 / hot 热门，设计稿 Tab）。
func (s *TopicService) Posts(ctx context.Context, name string, sort string, page int, pageSize int, viewerID int64) ([]model.PostSummary, int64, error) {
	posts, total, err := s.tags.PostsByTopic(ctx, name, sort, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	summaries, err := s.postSvc.assembleSummaries(ctx, posts, viewerID)
	return summaries, total, err
}

// ListByAuthor 用户主页帖子流（按作者 + 类型过滤，分页）。
func (s *TopicService) ListByAuthor(ctx context.Context, authorID int64, contentType string, page int, pageSize int, viewerID int64) ([]model.PostSummary, int64, error) {
	posts, total, err := s.posts.List(ctx, repository.ListParams{
		ContentType: contentType,
		AuthorID:    authorID,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		return nil, 0, err
	}
	summaries, err := s.postSvc.assembleSummaries(ctx, posts, viewerID)
	return summaries, total, err
}

// Follow 关注话题（幂等）。
func (s *TopicService) Follow(ctx context.Context, name string, viewerID int64) error {
	if viewerID == 0 {
		return errs.ErrUnauthorized
	}
	tagID, err := s.tags.FindByName(ctx, name)
	if err != nil {
		return errs.ErrNotFound
	}
	return s.tags.FollowTopic(ctx, viewerID, tagID)
}

// Unfollow 取消关注话题（幂等）。
func (s *TopicService) Unfollow(ctx context.Context, name string, viewerID int64) error {
	if viewerID == 0 {
		return errs.ErrUnauthorized
	}
	tagID, err := s.tags.FindByName(ctx, name)
	if err != nil {
		return errs.ErrNotFound
	}
	return s.tags.UnfollowTopic(ctx, viewerID, tagID)
}

// toDTO 组装话题 DTO（关注状态/关注数/浏览数）。
func (s *TopicService) toDTO(ctx context.Context, t repository.TagRow, viewerID int64) TopicDTO {
	dto := TopicDTO{
		Name:        "#" + t.Name,
		Slug:        t.Slug,
		Description: t.Description,
		PostCount:   t.PostCount,
	}
	// 关注数
	if count, err := s.tags.CountFollowers(ctx, t.ID); err == nil {
		dto.FollowCount = count
	}
	// 浏览数（该话题帖子浏览量求和）
	if count, err := s.tags.BrowseCount(ctx, t.ID); err == nil {
		dto.BrowseCount = count
	}
	// 当前用户关注状态
	if viewerID > 0 {
		if following, err := s.tags.IsFollowingTopic(ctx, viewerID, t.ID); err == nil {
			dto.Following = following
		}
	}
	return dto
}

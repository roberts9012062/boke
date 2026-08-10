// internal/service/reaction.go
// 互动业务逻辑：帖子点赞/收藏（需求 3.10）。
//
// 规则：
//   - 点赞/收藏需登录（未登录返回 1001，前端提示登录）
//   - 幂等：重复点赞/收藏不重复计数；取消走 DELETE
//   - posts.like_count 冗余同步（+1/-1）；收藏数实时 COUNT
//   - 点赞/收藏状态随详情/列表返回（liked/favorited 标记）
package service

import (
	"context"

	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// ReactionService 互动服务（连接器类）。
type ReactionService struct {
	reactions *repository.ReactionRepo // 互动数据访问
	posts     *repository.PostRepo     // 帖子数据访问
	notify    *NotificationService     // 通知服务（点赞通知）
}

// NewReactionService 创建互动服务。
func NewReactionService(reactions *repository.ReactionRepo, posts *repository.PostRepo, notify *NotificationService) *ReactionService {
	return &ReactionService{reactions: reactions, posts: posts, notify: notify}
}

// LikePost 点赞帖子（幂等）。
// 返回：最新点赞数；本次是否新增。
func (s *ReactionService) LikePost(ctx context.Context, postID int64, viewerID int64) (int64, bool, error) {
	if viewerID == 0 {
		return 0, false, errs.ErrUnauthorized
	}
	// 帖子存在性
	post, err := s.posts.FindByID(ctx, postID)
	if err != nil {
		return 0, false, errs.ErrNotFound
	}
	// 幂等添加互动
	added, err := s.reactions.Add(ctx, viewerID, postID, repository.ReactionLike)
	if err != nil {
		return 0, false, err
	}
	if added {
		if err := s.reactions.IncrPostLike(ctx, postID); err != nil {
			return 0, false, err
		}
		// 点赞通知（通知帖子作者，不给自己发）
		s.notify.NotifyLike(ctx, viewerID, post.AuthorID, postID, summaryPreviewText(post.Content))
	}
	return post.LikeCount + boolToInt64(added), added, nil
}

// UnlikePost 取消点赞（幂等）。
// 返回：最新点赞数。
func (s *ReactionService) UnlikePost(ctx context.Context, postID int64, viewerID int64) (int64, error) {
	if viewerID == 0 {
		return 0, errs.ErrUnauthorized
	}
	post, err := s.posts.FindByID(ctx, postID)
	if err != nil {
		return 0, errs.ErrNotFound
	}
	removed, err := s.reactions.Remove(ctx, viewerID, postID, repository.ReactionLike)
	if err != nil {
		return 0, err
	}
	if removed {
		if err := s.reactions.DecrPostLike(ctx, postID); err != nil {
			return 0, err
		}
		return post.LikeCount - 1, nil
	}
	return post.LikeCount, nil
}

// FavoritePost 收藏帖子（幂等）。
// 返回：最新收藏数；本次是否新增。
func (s *ReactionService) FavoritePost(ctx context.Context, postID int64, viewerID int64) (int64, bool, error) {
	if viewerID == 0 {
		return 0, false, errs.ErrUnauthorized
	}
	if _, err := s.posts.FindByID(ctx, postID); err != nil {
		return 0, false, errs.ErrNotFound
	}
	added, err := s.reactions.Add(ctx, viewerID, postID, repository.ReactionFavorite)
	if err != nil {
		return 0, false, err
	}
	count, err := s.reactions.CountFavorite(ctx, postID)
	return count, added, err
}

// UnfavoritePost 取消收藏（幂等）。
// 返回：最新收藏数。
func (s *ReactionService) UnfavoritePost(ctx context.Context, postID int64, viewerID int64) (int64, error) {
	if viewerID == 0 {
		return 0, errs.ErrUnauthorized
	}
	if _, err := s.reactions.Remove(ctx, viewerID, postID, repository.ReactionFavorite); err != nil {
		return 0, err
	}
	return s.reactions.CountFavorite(ctx, postID)
}

// PostReactionState 帖子互动状态（详情页展示）。
type PostReactionState struct {
	Liked     bool `json:"liked"`     // 当前用户是否已赞
	Favorited bool `json:"favorited"` // 当前用户是否已收藏
	FavoriteCount int64 `json:"favorite_count"` // 收藏数
}

// GetPostState 查询帖子互动状态（详情页展示；未登录返回空状态）。
func (s *ReactionService) GetPostState(ctx context.Context, postID int64, viewerID int64) (PostReactionState, error) {
	state := PostReactionState{}
	if viewerID == 0 {
		// 未登录：仅返回收藏数
		count, err := s.reactions.CountFavorite(ctx, postID)
		state.FavoriteCount = count
		return state, err
	}
	var err error
	if state.Liked, err = s.reactions.Has(ctx, viewerID, postID, repository.ReactionLike); err != nil {
		return state, err
	}
	if state.Favorited, err = s.reactions.Has(ctx, viewerID, postID, repository.ReactionFavorite); err != nil {
		return state, err
	}
	if state.FavoriteCount, err = s.reactions.CountFavorite(ctx, postID); err != nil {
		return state, err
	}
	return state, nil
}

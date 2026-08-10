// internal/service/relation.go
// 用户关系业务逻辑（需求 3.9/3.10）：关注/取关、粉丝/关注列表、编辑资料、收藏/赞过列表。
package service

import (
	"context"
	"strings"
	"time"

	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// FollowService 用户关系服务（连接器类）。
type FollowService struct {
	relations *repository.RelationRepo // 关系数据访问
	users     *repository.UserRepo     // 用户数据访问
	posts     *repository.PostRepo     // 帖子数据访问（收藏/赞过）
	postSvc   *PostService             // 帖子服务（摘要组装）
	notify    *NotificationService     // 通知服务（关注通知）
}

// NewFollowService 创建关系服务。
func NewFollowService(
	relations *repository.RelationRepo,
	users *repository.UserRepo,
	posts *repository.PostRepo,
	postSvc *PostService,
	notify *NotificationService,
) *FollowService {
	return &FollowService{relations: relations, users: users, posts: posts, postSvc: postSvc, notify: notify}
}

// Follow 关注用户（幂等；触发关注通知）。
func (s *FollowService) Follow(ctx context.Context, viewerID int64, targetID int64) (bool, error) {
	if viewerID == 0 {
		return false, errs.ErrUnauthorized
	}
	if viewerID == targetID {
		return false, errs.New(errs.CodeBadRequest, "不能关注自己")
	}
	// 目标用户存在性
	if _, err := s.users.FindByID(ctx, targetID); err != nil {
		return false, errs.ErrNotFound
	}
	added, err := s.relations.Follow(ctx, viewerID, targetID)
	if err != nil {
		return false, err
	}
	// 关注通知（不给自己发）
	if added {
		s.notify.NotifyFollow(ctx, viewerID, targetID)
	}
	return added, nil
}

// Unfollow 取消关注（幂等）。
func (s *FollowService) Unfollow(ctx context.Context, viewerID int64, targetID int64) (bool, error) {
	if viewerID == 0 {
		return false, errs.ErrUnauthorized
	}
	return s.relations.Unfollow(ctx, viewerID, targetID)
}

// Followers 粉丝列表（谁关注了我，分页）。
func (s *FollowService) Followers(ctx context.Context, userID int64, viewerID int64, page int, pageSize int) ([]UserRelationDTO, int64, error) {
	ids, total, err := s.relations.ListRelation(ctx, userID, true, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	return s.assembleUsers(ctx, ids, viewerID), total, nil
}

// Following 关注列表（我关注了谁，分页）。
func (s *FollowService) Following(ctx context.Context, userID int64, viewerID int64, page int, pageSize int) ([]UserRelationDTO, int64, error) {
	ids, total, err := s.relations.ListRelation(ctx, userID, false, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	return s.assembleUsers(ctx, ids, viewerID), total, nil
}

// UpdateProfile 编辑资料（昵称 ≤20 字符、简介 ≤100 字符，需求 3.9）。
func (s *FollowService) UpdateProfile(ctx context.Context, viewerID int64, nickname string, bio string) error {
	nickname = strings.TrimSpace(nickname)
	bio = strings.TrimSpace(bio)
	if nickname == "" || len([]rune(nickname)) > 20 {
		return errs.New(errs.CodeBadRequest, "昵称需为 1-20 个字符")
	}
	if len([]rune(bio)) > 100 {
		return errs.New(errs.CodeBadRequest, "简介不能超过 100 个字符")
	}
	return s.users.UpdateProfile(ctx, viewerID, nickname, bio)
}

// UpdateAvatar 更新头像（上传媒体后记录地址；空值表示移除头像，M1.7）。
func (s *FollowService) UpdateAvatar(ctx context.Context, viewerID int64, avatarURL string) error {
	if viewerID == 0 {
		return errs.ErrUnauthorized
	}
	return s.users.UpdateAvatar(ctx, viewerID, avatarURL)
}

// Favorites 我的收藏列表（帖子，分页；含收藏时间 favorited_at）。
func (s *FollowService) Favorites(ctx context.Context, viewerID int64, page int, pageSize int) ([]model.PostSummary, int64, error) {
	if viewerID == 0 {
		return nil, 0, errs.ErrUnauthorized
	}
	// 查询收藏的帖子（按收藏时间倒序）
	rows, total, err := s.posts.ListFavorites(ctx, viewerID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	// 组装摘要（postID → 收藏时间映射，供收藏页「收藏于」展示）
	posts := make([]model.Post, 0, len(rows))
	favTimes := make(map[int64]string, len(rows))
	for _, row := range rows {
		posts = append(posts, row.Post)
		favTimes[row.Post.ID] = row.FavoritedAt.Format(time.RFC3339)
	}
	summaries, err := s.postSvc.assembleSummaries(ctx, posts, viewerID)
	if err != nil {
		return nil, 0, err
	}
	// 填充收藏时间（设计稿：文字 · 收藏于 2 天前）
	for i := range summaries {
		if t, ok := favTimes[summaries[i].ID]; ok {
			summaries[i].FavoritedAt = t
		}
	}
	return summaries, total, nil
}

// LikedPosts 我赞过的帖子（个人主页「赞过」Tab）。
func (s *FollowService) LikedPosts(ctx context.Context, userID int64, viewerID int64, page int, pageSize int) ([]model.PostSummary, int64, error) {
	// 查询该用户赞过的已发布帖子
	posts, total, err := s.likedPosts(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	summaries, err := s.postSvc.assembleSummaries(ctx, posts, viewerID)
	return summaries, total, err
}

// ---------- 内部辅助 ----------

// likedPosts 查询赞过的帖子（post_reactions type=like）。
func (s *FollowService) likedPosts(ctx context.Context, userID int64, page int, pageSize int) ([]model.Post, int64, error) {
	where := `WHERE p.status = 'published' AND p.id IN (
		SELECT post_id FROM post_reactions WHERE user_id = $1 AND type = 'like'
	)`
	var total int64
	if err := s.posts.CountWhere(ctx, where, userID, &total); err != nil {
		return nil, 0, err
	}
	posts, err := s.posts.ListWhere(ctx, where, userID, page, pageSize)
	return posts, total, err
}

// assembleUsers 组装用户列表（关注状态）。
func (s *FollowService) assembleUsers(ctx context.Context, ids []int64, viewerID int64) []UserRelationDTO {
	result := make([]UserRelationDTO, 0, len(ids))
	for _, id := range ids {
		user, err := s.users.FindByID(ctx, id)
		if err != nil {
			continue
		}
		dto := UserRelationDTO{
			ID:        user.ID,
			Username:  user.Username,
			Nickname:  user.Nickname,
			AvatarURL: user.AvatarURL,
			Bio:       user.Bio,
		}
		// 当前用户是否已关注（互关显示「回关」）
		if viewerID > 0 && viewerID != id {
			if following, err := s.relations.IsFollowing(ctx, viewerID, id); err == nil {
				dto.Following = following
			}
		}
		result = append(result, dto)
	}
	return result
}

// UserRelationDTO 用户关系列表项（粉丝/关注）。
type UserRelationDTO struct {
	ID        int64  `json:"id"`         // 用户 ID
	Username  string `json:"username"`   // 账号名
	Nickname  string `json:"nickname"`   // 昵称
	AvatarURL string `json:"avatar_url"` // 头像
	Bio       string `json:"bio"`        // 简介
	Following bool   `json:"following"`  // 当前用户是否已关注（互关显示「回关」）
}

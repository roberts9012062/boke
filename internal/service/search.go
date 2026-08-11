// internal/service/search.go
// 搜索业务逻辑（需求 3.7）：帖子/话题/用户 关键词检索。
package service

import (
	"context"

	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/plugin"
	"github.com/roberts9012062/boke/internal/repository"
)

// SearchResult 搜索结果（按类型分组返回，前端 Tab 切换展示）。
type SearchResult struct {
	Posts  []model.PostSummary `json:"posts"`   // 帖子结果
	Total  int64               `json:"total"`   // 帖子总数（分页用）
	Topics []TopicDTO          `json:"topics"`  // 话题结果（标签名匹配）
	Users  []UserHit           `json:"users"`   // 用户结果
}

// UserHit 用户搜索结果。
type UserHit struct {
	ID        int64  `json:"id"`        // 用户 ID
	Username  string `json:"username"`  // 账号名
	Nickname  string `json:"nickname"`  // 昵称
	AvatarURL string `json:"avatar_url"` // 头像
	Bio       string `json:"bio"`       // 简介
}

// SearchService 搜索服务（连接器类）。
type SearchService struct {
	posts   *repository.PostRepo // 帖子数据访问（关键词检索）
	tags    *repository.TagRepo  // 标签数据访问（话题检索）
	users   *repository.UserRepo // 用户数据访问（用户检索）
	postSvc *PostService         // 帖子服务（摘要组装）
	hooks   plugin.Dispatcher    // 插件钩子调度器（M3.2 扩展框架）
}

// NewSearchService 创建搜索服务。
func NewSearchService(
	posts *repository.PostRepo,
	tags *repository.TagRepo,
	users *repository.UserRepo,
	postSvc *PostService,
	hooks plugin.Dispatcher,
) *SearchService {
	return &SearchService{posts: posts, tags: tags, users: users, postSvc: postSvc, hooks: hooks}
}

// Search 执行搜索（帖子 + 话题 + 用户三组结果）。
// 参数：keyword 关键词；page/pageSize 帖子分页；viewerID 当前用户。
func (s *SearchService) Search(ctx context.Context, keyword string, page int, pageSize int, viewerID int64) (*SearchResult, error) {
	// ---------- 插件钩子：search.query（同步，可改写关键词；M3.2 扩展框架） ----------
	if res := s.hooks.Dispatch(ctx, plugin.HookSearchQuery, plugin.Event{
		ActorID: viewerID,
		Payload: keyword,
	}); !res.OK {
		// 拒绝则按空结果返回（插件可整体禁用搜索扩展）
		return &SearchResult{Posts: []model.PostSummary{}, Topics: []TopicDTO{}, Users: []UserHit{}}, nil
	} else if kw, ok := res.Modify.(string); ok && kw != "" {
		keyword = kw
	}
	result := &SearchResult{
		Posts:  make([]model.PostSummary, 0),
		Topics: make([]TopicDTO, 0),
		Users:  make([]UserHit, 0),
	}

	// ---------- 帖子检索（标题/正文/标签） ----------
	posts, total, err := s.posts.Search(ctx, keyword, page, pageSize)
	if err != nil {
		return nil, err
	}
	result.Total = total
	if result.Posts, err = s.postSvc.assembleSummaries(ctx, posts, viewerID); err != nil {
		return nil, err
	}

	// ---------- 话题检索（标签名匹配，最多 10 条） ----------
	if topicRows, err := s.tags.Search(ctx, keyword, 10); err == nil {
		for _, t := range topicRows {
			result.Topics = append(result.Topics, TopicDTO{
				Name:        "#" + t.Name,
				Slug:        t.Slug,
				Description: t.Description,
				PostCount:   t.PostCount,
			})
		}
	}

	// ---------- 用户检索（昵称/账号名匹配，最多 10 条） ----------
	if userRows, err := s.users.Search(ctx, keyword, 10); err == nil {
		for _, u := range userRows {
			result.Users = append(result.Users, UserHit{
				ID:        u.ID,
				Username:  u.Username,
				Nickname:  u.Nickname,
				AvatarURL: u.AvatarURL,
				Bio:       u.Bio,
			})
		}
	}
	return result, nil
}

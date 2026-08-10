// internal/service/notification.go
// 通知业务逻辑（需求 3.8）：互动触发（赞/评论/回复/关注）+ 列表 + 已读。
//
// 触发点：点赞帖子 → 通知作者；评论帖子 → 通知作者；回复评论 → 通知被回复者；
//         关注用户 → 通知被关注者。不给自己发通知。
package service

import (
	"context"
	"strconv"

	"github.com/yueyan/boke/internal/model"
	"github.com/yueyan/boke/internal/repository"
)

// NotificationDTO 通知条目（前端展示：触发者 + 动作文案 + 帖子摘要 + 时间）。
type NotificationDTO struct {
	ID        int64  `json:"id"`         // 通知 ID
	Type      string `json:"type"`       // 类型：like/comment/reply/follow/system
	Title     string `json:"title"`      // 标题（动作文案）
	Content   string `json:"content"`    // 内容（帖子摘要/附加信息）
	Link      string `json:"link"`       // 跳转链接
	PostID    int64  `json:"post_id"`    // 相关帖子 ID
	Read      bool   `json:"read"`       // 是否已读
	CreatedAt string `json:"created_at"` // 创建时间（ISO8601）
	Actor     *model.CommentAuthor `json:"actor"` // 触发者（系统通知为 nil）
}

// NotificationService 通知服务（连接器类）。
type NotificationService struct {
	notifications *repository.NotificationRepo // 通知数据访问
	users         *repository.UserRepo         // 用户数据访问（触发者信息）
}

// NewNotificationService 创建通知服务。
func NewNotificationService(
	notifications *repository.NotificationRepo,
	users *repository.UserRepo,
) *NotificationService {
	return &NotificationService{notifications: notifications, users: users}
}

// ---------- 触发（供互动/评论/关注 service 调用） ----------

// NotifyLike 点赞通知（dedup：同源未读不重复）。
func (s *NotificationService) NotifyLike(ctx context.Context, actorID int64, targetUserID int64, postID int64, postSummary string) {
	s.send(ctx, targetUserID, actorID, repository.NotifyLike, "赞了你的帖子", postSummary, postID, true)
}

// NotifyComment 评论通知（评论我的帖子）。
func (s *NotificationService) NotifyComment(ctx context.Context, actorID int64, targetUserID int64, postID int64, commentPreview string) {
	s.send(ctx, targetUserID, actorID, repository.NotifyComment, "评论了你", commentPreview, postID, false)
}

// NotifyReply 回复通知（回复我的评论）。
func (s *NotificationService) NotifyReply(ctx context.Context, actorID int64, targetUserID int64, postID int64, replyPreview string) {
	s.send(ctx, targetUserID, actorID, repository.NotifyReply, "回复了你的评论", replyPreview, postID, false)
}

// NotifyFollow 关注通知（dedup：同源未读不重复）。
func (s *NotificationService) NotifyFollow(ctx context.Context, actorID int64, targetUserID int64) {
	s.send(ctx, targetUserID, actorID, repository.NotifyFollow, "开始关注你", "现在你们可以互相收到更新", 0, true)
}

// NotifyMessage 私信通知（M2：收到私信，内容为消息预览，跳转消息中心）。
// 说明：链路为 /messages，与帖子类通知不同，直接写入通知（不走 send 的帖子链接逻辑）。
func (s *NotificationService) NotifyMessage(ctx context.Context, actorID int64, targetUserID int64, preview string) {
	// 不给自己发通知
	if targetUserID <= 0 || targetUserID == actorID {
		return
	}
	// 预览截断 60 字符（通知列表展示）
	runes := []rune(preview)
	if len(runes) > 60 {
		preview = string(runes[:60]) + "…"
	}
	_ = s.notifications.Create(ctx, repository.Notification{
		UserID:  targetUserID,
		ActorID: actorID,
		Type:    repository.NotifyMessage,
		Title:   "给你发来私信",
		Content: preview,
		Link:    "/messages",
	}, false)
}

// send 写入通知（不给本人发；失败静默，不影响主流程）。
func (s *NotificationService) send(ctx context.Context, targetUserID int64, actorID int64, notifyType string, title string, content string, postID int64, dedup bool) {
	// 不给自己发通知
	if targetUserID <= 0 || targetUserID == actorID {
		return
	}
	link := ""
	if postID > 0 {
		link = "/posts/" + strconv.FormatInt(postID, 10)
	}
	_ = s.notifications.Create(ctx, repository.Notification{
		UserID:  targetUserID,
		ActorID: actorID,
		Type:    notifyType,
		Title:   title,
		Content: content,
		Link:    link,
		PostID:  postID,
	}, dedup)
}

// ---------- 列表 / 已读 ----------

// List 通知列表（按类型过滤 + 分页 + 触发者信息）。
func (s *NotificationService) List(ctx context.Context, userID int64, notifyType string, page int, pageSize int) ([]NotificationDTO, int64, error) {
	items, total, err := s.notifications.List(ctx, repository.NotificationListParams{
		UserID: userID, Type: notifyType, Page: page, PageSize: pageSize,
	})
	if err != nil {
		return nil, 0, err
	}

	result := make([]NotificationDTO, 0, len(items))
	for _, n := range items {
		dto := NotificationDTO{
			ID:        n.ID,
			Type:      n.Type,
			Title:     n.Title,
			Content:   n.Content,
			Link:      n.Link,
			PostID:    n.PostID,
			Read:      n.ReadAt != nil,
			CreatedAt: n.CreatedAt.Format("2006-01-02T15:04:05-07:00"),
		}
		// 触发者信息（系统通知 nil）
		if n.ActorID > 0 {
			if user, err := s.users.FindByID(ctx, n.ActorID); err == nil {
				dto.Actor = &model.CommentAuthor{
					ID:       user.ID,
					Username: user.Username,
					Nickname: user.Nickname,
				}
			}
		}
		result = append(result, dto)
	}
	return result, total, nil
}

// CountUnread 未读通知数（角标轮询）。
func (s *NotificationService) CountUnread(ctx context.Context, userID int64) (int64, error) {
	return s.notifications.CountUnread(ctx, userID)
}

// MarkRead 单条已读（校验归属）。
func (s *NotificationService) MarkRead(ctx context.Context, userID int64, notificationID int64) error {
	return s.notifications.MarkRead(ctx, userID, notificationID)
}

// MarkAllRead 全部已读。
func (s *NotificationService) MarkAllRead(ctx context.Context, userID int64) error {
	return s.notifications.MarkAllRead(ctx, userID)
}

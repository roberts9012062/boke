// internal/service/message.go
// 私信业务逻辑（M2，需求 3.10 私信/消息）：
// 会话列表/发起、消息分页（打开即已读）、发送（未读累加 + 通知）、未读总数。
// 设计稿《消息》画板：会话列表（全部/未读 Tab + 未读徽标）+ 对话详情（气泡 + 写消息…发送）。
package service

import (
	"context"
	"strings"
	"time"

	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// 消息内容上限（与设计稿输入框「写消息…」一致，防滥用）。
const maxMessageLen = 1000

// MessageService 私信服务（连接器类）。
type MessageService struct {
	messages *repository.MessageRepo // 会话/消息数据访问
	users    *repository.UserRepo    // 用户数据访问（对方信息）
	notify   *NotificationService    // 通知服务（收到私信通知）
}

// NewMessageService 创建私信服务。
func NewMessageService(messages *repository.MessageRepo, users *repository.UserRepo, notify *NotificationService) *MessageService {
	return &MessageService{messages: messages, users: users, notify: notify}
}

// ConversationDTO 会话列表项（含对方信息与最后消息摘要）。
type ConversationDTO struct {
	ID            int64  `json:"id"`             // 会话 ID
	Peer          PeerDTO `json:"peer"`          // 对方信息
	LastMessage   string `json:"last_message"`   // 最后一条消息摘要
	LastMessageAt string `json:"last_message_at"` // 最后消息时间（ISO8601）
	Unread        int    `json:"unread"`         // 我的未读数（列表徽标）
}

// PeerDTO 会话对方信息。
type PeerDTO struct {
	ID        int64  `json:"id"`         // 用户 ID
	Username  string `json:"username"`   // 账号
	Nickname  string `json:"nickname"`   // 昵称
	AvatarURL string `json:"avatar_url"` // 头像
	Online    bool   `json:"online"`     // 在线状态（最后登录 5 分钟内，设计稿「· 在线」）
}

// 在线判定窗口（最后登录 5 分钟内视为在线）。
const onlineWindow = 5 * time.Minute

// MessageDTO 消息（气泡）。
type MessageDTO struct {
	ID        int64  `json:"id"`              // 消息 ID
	SenderID  int64  `json:"sender_id"`       // 发送者
	Content   string `json:"content"`         // 内容
	CreatedAt string `json:"created_at"`      // 发送时间（ISO8601）
	IsMine    bool   `json:"is_mine"`         // 是否我发的（前端气泡左右）
}

// ListConversations 会话列表（全部/未读过滤，分页）。
func (s *MessageService) ListConversations(ctx context.Context, userID int64, onlyUnread bool, page int, pageSize int) ([]ConversationDTO, int64, error) {
	if userID == 0 {
		return nil, 0, errs.ErrUnauthorized
	}
	convs, total, err := s.messages.ListConversations(ctx, userID, onlyUnread, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	items := make([]ConversationDTO, 0, len(convs))
	for _, c := range convs {
		items = append(items, s.assembleConversation(ctx, userID, c))
	}
	return items, total, nil
}

// Open 发起或打开与某用户的会话（从他人主页「私信」按钮进入）。
// 返回：会话 DTO（含未读）。
func (s *MessageService) Open(ctx context.Context, userID int64, otherUserID int64) (*ConversationDTO, error) {
	if userID == 0 {
		return nil, errs.ErrUnauthorized
	}
	if userID == otherUserID {
		return nil, errs.New(errs.CodeBadRequest, "不能与自己发起会话")
	}
	// 目标用户存在性
	if _, err := s.users.FindByID(ctx, otherUserID); err != nil {
		return nil, errs.ErrNotFound
	}
	conversationID, _, err := s.messages.GetOrCreate(ctx, userID, otherUserID)
	if err != nil {
		return nil, err
	}
	conversation, err := s.messages.FindByID(ctx, conversationID)
	if err != nil {
		return nil, errs.ErrNotFound
	}
	dto := s.assembleConversation(ctx, userID, conversation)
	return &dto, nil
}

// ListMessages 会话消息列表（时间正序返回，打开即标记已读）。
// 校验：仅会话成员可查看。
func (s *MessageService) ListMessages(ctx context.Context, userID int64, conversationID int64, page int, pageSize int) ([]MessageDTO, int64, error) {
	conversation, err := s.messages.FindByID(ctx, conversationID)
	if err != nil {
		return nil, 0, errs.ErrNotFound
	}
	if userID != conversation.UserA && userID != conversation.UserB {
		return nil, 0, errs.New(errs.CodeForbidden, "无权查看该会话")
	}
	// 打开会话 → 标记已读（未读清零 + read_at 落库；失败不阻断列表）
	_ = s.messages.MarkRead(ctx, conversationID, userID)

	// 按时间倒序分页查询后翻转（列表按时间正序展示）
	messages, total, err := s.messages.ListMessages(ctx, conversationID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	items := make([]MessageDTO, 0, len(messages))
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		items = append(items, MessageDTO{
			ID:        m.ID,
			SenderID:  m.SenderID,
			Content:   m.Content,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
			IsMine:    m.SenderID == userID,
		})
	}
	return items, total, nil
}

// Send 发送消息（校验成员 → 写入 → 对方未读 +1 → 私信通知）。
func (s *MessageService) Send(ctx context.Context, userID int64, conversationID int64, content string) (*MessageDTO, error) {
	conversation, err := s.messages.FindByID(ctx, conversationID)
	if err != nil {
		return nil, errs.ErrNotFound
	}
	if userID != conversation.UserA && userID != conversation.UserB {
		return nil, errs.New(errs.CodeForbidden, "无权在该会话发消息")
	}
	content = strings.TrimSpace(content)
	if content == "" || len([]rune(content)) > maxMessageLen {
		return nil, errs.New(errs.CodeBadRequest, "消息不能为空且不超过 1000 字")
	}

	messageID, err := s.messages.CreateMessage(ctx, conversationID, userID, content)
	if err != nil {
		return nil, err
	}
	// 接收方 = 对方（未读累加）
	receiverID := conversation.UserA
	if userID == conversation.UserA {
		receiverID = conversation.UserB
	}
	if err := s.messages.AfterSend(ctx, conversationID, messageID, receiverID); err != nil {
		return nil, err
	}
	// 私信通知（不打扰自己；消息预览作内容）
	s.notify.NotifyMessage(ctx, userID, receiverID, content)

	return &MessageDTO{
		ID:        messageID,
		SenderID:  userID,
		Content:   content,
		CreatedAt: time.Now().Format(time.RFC3339),
		IsMine:    true,
	}, nil
}

// UnreadTotal 全部会话未读总数（消息中心角标）。
func (s *MessageService) UnreadTotal(ctx context.Context, userID int64) (int64, error) {
	if userID == 0 {
		return 0, nil
	}
	return s.messages.CountUnreadTotal(ctx, userID)
}

// ---------- 内部辅助 ----------

// assembleConversation 组装会话 DTO（对方信息 + 最后消息摘要 + 我的未读）。
func (s *MessageService) assembleConversation(ctx context.Context, userID int64, c repository.Conversation) ConversationDTO {
	// 对方 ID 与我的未读（A/B 判定）
	peerID := c.UserA
	unread := c.UnreadA
	if userID == c.UserA {
		peerID = c.UserB
		unread = c.UnreadB
	}

	// 对方信息（失败降级为空昵称，不阻断列表；在线 = 最后登录 5 分钟内）
	peer := PeerDTO{ID: peerID}
	if u, err := s.users.FindByID(ctx, peerID); err == nil {
		peer.Username = u.Username
		peer.Nickname = u.Nickname
		peer.AvatarURL = u.AvatarURL
		peer.Online = u.LastLoginAt != nil && time.Since(*u.LastLoginAt) < onlineWindow
	}

	dto := ConversationDTO{ID: c.ID, Peer: peer, Unread: unread}
	// 最后一条消息摘要（无消息时显示「打个招呼吧」）
	if c.LastMessageID > 0 {
		if last, err := s.messages.LastMessage(ctx, c.ID); err == nil {
			dto.LastMessage = last.Content
			dto.LastMessageAt = last.CreatedAt.Format(time.RFC3339)
		}
	}
	return dto
}

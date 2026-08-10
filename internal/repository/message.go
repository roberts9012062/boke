// internal/repository/message.go
// 私信数据访问（conversations / messages 表，M2 私信/消息）：
// 会话列表/创建/消息分页/未读计数/已读标记。
// 约定（schema）：两人之间唯一会话，user_a < user_b；未读冗余在 conversations（unread_a/unread_b）。
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Conversation 私信会话实体（conversations 表）。
type Conversation struct {
	ID            int64     // 会话 ID
	UserA         int64     // 会话方 A（约定 user_a < user_b）
	UserB         int64     // 会话方 B
	LastMessageID int64     // 最后一条消息 ID（列表排序冗余）
	UnreadA       int       // A 的未读数
	UnreadB       int       // B 的未读数
	CreatedAt     time.Time // 创建时间
	UpdatedAt     time.Time // 更新时间
}

// Message 私信消息实体（messages 表）。
type Message struct {
	ID             int64      // 消息 ID
	ConversationID int64      // 所属会话
	SenderID       int64      // 发送者
	Content        string     // 内容
	ReadAt         *time.Time // 已读时间（NULL = 未读）
	CreatedAt      time.Time  // 发送时间
}

// MessageRepo 私信数据访问（连接器类）。
type MessageRepo struct {
	pool *pgxpool.Pool
}

// NewMessageRepo 创建私信仓库。
func NewMessageRepo(pool *pgxpool.Pool) *MessageRepo {
	return &MessageRepo{pool: pool}
}

// conversationColumns 会话查询列清单。
const conversationColumns = `id, user_a, user_b, last_message_id, unread_a, unread_b, created_at, updated_at`

// scanConversation 将查询行扫描为会话实体。
func scanConversation(row pgx.Row) (Conversation, error) {
	var c Conversation
	err := row.Scan(&c.ID, &c.UserA, &c.UserB, &c.LastMessageID, &c.UnreadA, &c.UnreadB, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

// ListConversations 会话列表（按最后消息时间倒序，分页）。
// 参数：userID 当前用户；onlyUnread 仅未读会话；返回会话与总数。
func (r *MessageRepo) ListConversations(ctx context.Context, userID int64, onlyUnread bool, page int, pageSize int) ([]Conversation, int64, error) {
	// 会话方条件：我是 A 或 B
	where := "WHERE (user_a = $1 OR user_b = $1)"
	if onlyUnread {
		where += " AND ((user_a = $1 AND unread_a > 0) OR (user_b = $1 AND unread_b > 0))"
	}

	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM conversations `+where, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+conversationColumns+` FROM conversations `+where+`
		ORDER BY updated_at DESC
		LIMIT $2 OFFSET $3`, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]Conversation, 0)
	for rows.Next() {
		c, err := scanConversation(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, c)
	}
	return items, total, rows.Err()
}

// FindByID 查询会话（不存在返回 ErrConversationNotFound）。
func (r *MessageRepo) FindByID(ctx context.Context, conversationID int64) (Conversation, error) {
	c, err := scanConversation(r.pool.QueryRow(ctx,
		`SELECT `+conversationColumns+` FROM conversations WHERE id = $1`, conversationID))
	if err != nil {
		return Conversation{}, wrapMessageNotFound(err)
	}
	return c, nil
}

// GetOrCreate 获取或创建两人会话（幂等；user_a < user_b 归一化）。
// 返回：会话 ID；是否新建（xmax=0 判定插入行，标准 PostgreSQL 技巧）。
func (r *MessageRepo) GetOrCreate(ctx context.Context, userID1 int64, userID2 int64) (int64, bool, error) {
	if userID1 == userID2 {
		return 0, false, errors.New("不能与自己发起会话")
	}
	// 归一化：小者为 A
	a, b := userID1, userID2
	if a > b {
		a, b = b, a
	}
	var id int64
	var inserted bool
	err := r.pool.QueryRow(ctx, `
		INSERT INTO conversations (user_a, user_b)
		VALUES ($1, $2)
		ON CONFLICT (user_a, user_b) DO UPDATE SET updated_at = conversations.updated_at
		RETURNING id, (xmax = 0) AS inserted`, a, b).Scan(&id, &inserted)
	return id, inserted, err
}

// ListMessages 会话消息列表（时间倒序分页，前端翻转展示；按 created_at 分页）。
func (r *MessageRepo) ListMessages(ctx context.Context, conversationID int64, page int, pageSize int) ([]Message, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM messages WHERE conversation_id = $1`, conversationID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, conversation_id, sender_id, content, read_at, created_at
		FROM messages WHERE conversation_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, conversationID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]Message, 0)
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Content, &m.ReadAt, &m.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, m)
	}
	return items, total, rows.Err()
}

// LastMessage 会话最后一条消息（列表摘要）。
func (r *MessageRepo) LastMessage(ctx context.Context, conversationID int64) (Message, error) {
	var m Message
	err := r.pool.QueryRow(ctx, `
		SELECT id, conversation_id, sender_id, content, read_at, created_at
		FROM messages WHERE conversation_id = $1
		ORDER BY created_at DESC LIMIT 1`, conversationID).Scan(
		&m.ID, &m.ConversationID, &m.SenderID, &m.Content, &m.ReadAt, &m.CreatedAt)
	return m, err
}

// CreateMessage 写入消息（返回消息 ID）。
func (r *MessageRepo) CreateMessage(ctx context.Context, conversationID int64, senderID int64, content string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, sender_id, content)
		VALUES ($1, $2, $3)
		RETURNING id`, conversationID, senderID, content).Scan(&id)
	return id, err
}

// AfterSend 发送后更新会话：最后消息 ID + 更新时间 + 接收方未读 +1。
// 参数：conversationID 会话；receiverID 接收方（未读累加对象）。
func (r *MessageRepo) AfterSend(ctx context.Context, conversationID int64, messageID int64, receiverID int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE conversations SET
			last_message_id = $2,
			updated_at = now(),
			unread_a = CASE WHEN user_a = $3 THEN unread_a + 1 ELSE unread_a END,
			unread_b = CASE WHEN user_b = $3 THEN unread_b + 1 ELSE unread_b END
		WHERE id = $1`, conversationID, messageID, receiverID)
	return err
}

// MarkRead 打开会话标记已读：本人未读清零 + 本人收到的消息 read_at 落库。
func (r *MessageRepo) MarkRead(ctx context.Context, conversationID int64, userID int64) error {
	// 本人未读清零（A/B 判定）
	_, err := r.pool.Exec(ctx, `
		UPDATE conversations SET
			unread_a = CASE WHEN user_a = $2 THEN 0 ELSE unread_a END,
			unread_b = CASE WHEN user_b = $2 THEN 0 ELSE unread_b END
		WHERE id = $1`, conversationID, userID)
	if err != nil {
		return err
	}
	// 本人收到的未读消息标记已读
	_, err = r.pool.Exec(ctx, `
		UPDATE messages SET read_at = now()
		WHERE conversation_id = $1 AND sender_id != $2 AND read_at IS NULL`, conversationID, userID)
	return err
}

// CountUnreadTotal 当前用户全部会话未读总数（角标）。
func (r *MessageRepo) CountUnreadTotal(ctx context.Context, userID int64) (int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(sum(CASE WHEN user_a = $1 THEN unread_a ELSE unread_b END), 0)
		FROM conversations WHERE user_a = $1 OR user_b = $1`, userID).Scan(&total)
	return total, err
}

// ErrConversationNotFound 会话不存在（pgx.ErrNoRows 包装，供 service 层判错）。
var ErrConversationNotFound = errors.New("会话不存在")

// wrapNotFound 将 pgx.ErrNoRows 转为业务未找到错误。
func wrapMessageNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrConversationNotFound
	}
	return err
}

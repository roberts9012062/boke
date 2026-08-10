// internal/model/comment.go
// 评论模块数据模型：Comment 实体 + 评论/匿名身份 DTO（与前端 src/types 手工同步）。
package model

import "time"

// 评论状态（附录 B 状态字典）。
const (
	CommentStatusVisible = "visible" // 可见
	CommentStatusHidden  = "hidden"  // 隐藏（后台）
	CommentStatusDeleted = "deleted" // 已删除（软删）
)

// ---------- 实体 ----------

// Comment 评论实体（comments 表结构，含 001 迁移的匿名字段）。
type Comment struct {
	ID             int64      // 评论 ID
	PostID         int64      // 所属帖子
	AuthorID       *int64     // 评论者（NULL = 匿名访客）
	ParentID       *int64     // 父评论（NULL = 顶层；MVP 限 2 级）
	Content        string     // 评论内容
	Floor          int        // 楼层号（按帖子内递增）
	Status         string     // 状态：visible/hidden/deleted
	LikeCount      int64      // 点赞数
	GuestName      string     // 匿名昵称（guest_name）
	GuestTokenHash string     // 匿名 token 哈希（防刷）
	CreatedAt      time.Time  // 创建时间
	UpdatedAt      time.Time  // 更新时间
}

// ---------- DTO ----------

// CommentAuthor 评论作者（登录用户；匿名为空）。
type CommentAuthor struct {
	ID       int64  `json:"id"`        // 用户 ID
	Username string `json:"username"`  // 账号名
	Nickname string `json:"nickname"`  // 昵称
}

// CommentDTO 评论（顶层含 replies 子回复列表，楼中楼）。
type CommentDTO struct {
	ID         int64           `json:"id"`          // 评论 ID
	Content    string          `json:"content"`     // 评论内容
	Author     *CommentAuthor  `json:"author"`      // 作者（匿名为 null）
	GuestName  string          `json:"guest_name"`  // 匿名昵称（匿名评论时）
	LikeCount  int64           `json:"like_count"`  // 点赞数
	CreatedAt  string          `json:"created_at"`  // 创建时间（ISO8601）
	IsAuthor   bool            `json:"is_author"`   // 是否本人（删除权限）
	Liked      bool            `json:"liked"`       // 当前用户是否已赞
	ReplyCount int             `json:"reply_count"` // 回复数
	Replies    []CommentDTO    `json:"replies"`     // 子回复（楼中楼，仅顶层含）
}

// ---------- 匿名身份 DTO ----------

// GuestIdentity 匿名身份（POST /guest-identity 签发）。
type GuestIdentity struct {
	GuestToken string `json:"guest_token"` // 匿名 token（短期有效，防刷）
	GuestName  string `json:"guest_name"`  // 匿名昵称
}

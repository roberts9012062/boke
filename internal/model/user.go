// internal/model/user.go
// 用户模块数据模型：User 实体 + 认证/资料 DTO（与前端 src/types 手工同步）。
package model

import "time"

// ---------- 用户实体（对应 users 表） ----------

// User 用户实体（users 表结构，不含任何敏感派生字段）。
type User struct {
	ID              int64     // 用户 ID
	Email           string    // 登录邮箱（唯一）
	Username        string    // 用户名（唯一，@账号）
	PasswordHash    string    // 密码哈希（bcrypt）
	Nickname        string    // 昵称
	AvatarURL       string    // 头像地址
	Bio             string    // 个人简介
	Status          string    // 状态：active=正常 / banned=封禁 / disabled=停用
	PasswordVersion int       // 密码版本号（P1：重置密码自增，JWT 校验使旧会话失效）
	LastLoginAt     *time.Time // 最后登录时间
	CreatedAt       time.Time // 注册时间
	UpdatedAt       time.Time // 更新时间
}

// ---------- 认证 DTO ----------

// RegisterReq 注册请求（需求 3.1：昵称 + 邮箱 + 密码；用户名由服务端自动生成）。
type RegisterReq struct {
	Nickname string // 昵称（1-20 字符）
	Email    string // 登录邮箱（格式校验）
	Password string // 密码（≥8 位，含字母与数字）
}

// LoginReq 登录请求（邮箱或用户名 + 密码，需求 3.1「邮箱或用户名 + 密码」）。
type LoginReq struct {
	Account  string // 邮箱或用户名
	Password string // 密码
}

// TokenPair 令牌对（access 15min + refresh 7d，架构文档 9.3）。
type TokenPair struct {
	AccessToken  string `json:"access_token"`  // 访问令牌（15 分钟）
	RefreshToken string `json:"refresh_token"` // 刷新令牌（7 天）
	ExpiresIn    int64  `json:"expires_in"`    // access 有效期（秒）
}

// ---------- 资料 DTO ----------

// UserProfile 用户资料（对外返回，不含密码哈希）。
type UserProfile struct {
	ID             int64  `json:"id"`            // 用户 ID
	Email          string `json:"email"`         // 邮箱（仅本人可见完整值）
	Username       string `json:"username"`      // 用户名（@账号）
	Nickname       string `json:"nickname"`      // 昵称
	AvatarURL      string `json:"avatar_url"`    // 头像地址
	Bio            string `json:"bio"`           // 个人简介
	Role           string `json:"role"`          // 角色：admin / user
	Status         string `json:"status"`        // 用户状态
	PostCount      int64  `json:"post_count"`    // 帖子数
	LikeCount      int64  `json:"like_count"`    // 获赞数
	TopicCount     int64  `json:"topic_count"`   // 话题数
	ViewCount      int64  `json:"view_count"`    // 浏览数（帖子浏览量求和，设计稿个人主页统计）
	FollowerCount  int64  `json:"follower_count"` // 粉丝数（M1.7：主页/粉丝页统计）
	FollowingCount int64  `json:"following_count"` // 关注数（M1.7）
	CreatedAt      string `json:"created_at"`    // 注册时间（ISO8601）
}

// ToProfile 将用户实体转为对外资料（不含密码）。
// 返回：资料对象；默认角色与统计由调用方补充。
func (u User) ToProfile() UserProfile {
	return UserProfile{
		ID:        u.ID,
		Email:     u.Email,
		Username:  u.Username,
		Nickname:  u.Nickname,
		AvatarURL: u.AvatarURL,
		Bio:       u.Bio,
		Status:    u.Status,
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}

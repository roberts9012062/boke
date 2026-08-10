// internal/auth/jwt.go
// JWT 签发与解析：access（15min）+ refresh（7d）（架构文档 9.3）。
//
// Claims 设计：
//   - token_type=access：仅用于接口鉴权，过期即失效
//   - token_type=refresh：换取新令牌对；登出时写入黑名单（Redis）
//   - 角色（role）随 access 签发，鉴权中间件直接读取
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// 令牌有效期（需求 3.1 会话约定：access 15min + refresh 7d）。
const (
	AccessTTL  = 15 * time.Minute
	RefreshTTL = 7 * 24 * time.Hour
)

// ErrTokenExpired 令牌已过期（鉴权中间件据此返回 1002，触发前端静默刷新）。
var ErrTokenExpired = errors.New("token 已过期")

// tokenType 令牌类型（区分 access / refresh）。
type tokenType string

const (
	tokenAccess  tokenType = "access"
	tokenRefresh tokenType = "refresh"
)

// Claims 自定义声明（标准 JWT + 业务字段）。
type Claims struct {
	UserID    int64     `json:"uid"`    // 用户 ID
	Role      string    `json:"role"`   // 角色：admin / user
	TokenType tokenType `json:"ttp"`    // 令牌类型
	TokenID   string    `json:"jti"`    // 令牌 ID（refresh 撤销用）
	jwt.RegisteredClaims
}

// Manager JWT 管理器（连接器类，持有签名密钥）。
type Manager struct {
	secret []byte // HMAC 签名密钥
}

// NewManager 创建 JWT 管理器。
func NewManager(secret string) *Manager {
	return &Manager{secret: []byte(secret)}
}

// sign 签发指定类型令牌（共用签名与声明组装）。
func (m *Manager) sign(userID int64, role string, tType tokenType, tokenID string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID:    userID,
		Role:      role,
		TokenType: tType,
		TokenID:   tokenID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "yueyan-blog",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// GenerateTokenPair 签发令牌对（access + refresh）。
// 参数：userID 用户 ID；role 角色；refreshTokenID 刷新令牌 ID（随机，撤销用）。
func (m *Manager) GenerateTokenPair(userID int64, role string, refreshTokenID string) (access string, refresh string, err error) {
	access, err = m.sign(userID, role, tokenAccess, "", AccessTTL)
	if err != nil {
		return "", "", fmt.Errorf("签发 access token 失败：%w", err)
	}
	refresh, err = m.sign(userID, role, tokenRefresh, refreshTokenID, RefreshTTL)
	if err != nil {
		return "", "", fmt.Errorf("签发 refresh token 失败：%w", err)
	}
	return access, refresh, nil
}

// parse 解析并校验令牌签名（返回 Claims）。
func (m *Manager) parse(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		// 仅接受 HS256 签名算法（防算法混淆攻击）
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("不支持的签名算法")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("令牌无效")
	}
	return claims, nil
}

// ParseAccess 解析 access 令牌（鉴权中间件使用）。
// 返回：Claims；令牌类型不符或过期时返回错误。
func (m *Manager) ParseAccess(tokenString string) (*Claims, error) {
	claims, err := m.parse(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != tokenAccess {
		return nil, errors.New("令牌类型错误：非 access 令牌")
	}
	return claims, nil
}

// ParseRefresh 解析 refresh 令牌（刷新接口使用）。
// 返回：Claims；令牌类型不符或过期时返回错误。
func (m *Manager) ParseRefresh(tokenString string) (*Claims, error) {
	claims, err := m.parse(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != tokenRefresh {
		return nil, errors.New("令牌类型错误：非 refresh 令牌")
	}
	return claims, nil
}

// ExpiresInSeconds 返回 access 剩余有效期（秒），供前端定时刷新。
func ExpiresInSeconds() int64 {
	return int64(AccessTTL.Seconds())
}

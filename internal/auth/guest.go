// internal/auth/guest.go
// 匿名评论身份：签发短期匿名 token（需求 3.5「开放，无需登录」+ mvp-plan 决策 2）。
//
// 设计：
//   - POST /guest-identity 签发：昵称自填（可选），默认「匿名访客」+ 随机后缀
//   - token 内存存储（单机开发够用；TTL 7 天），哈希落库防刷（1 条/分钟/同 token）
//   - 已登录用户直接使用账号身份，不走匿名通道
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// 匿名身份有效期（7 天）。
const guestTokenTTL = 7 * 24 * time.Hour

// guestEntry 内存中的匿名身份记录。
type guestEntry struct {
	name    string    // 匿名昵称
	expires time.Time // 过期时间
}

// GuestManager 匿名身份管理器（连接器类，内存 map + 互斥锁）。
type GuestManager struct {
	mu     sync.Mutex             // 保护 entries
	entries map[string]guestEntry // token → 身份记录
}

// NewGuestManager 创建匿名身份管理器。
func NewGuestManager() *GuestManager {
	return &GuestManager{entries: make(map[string]guestEntry)}
}

// randomToken 生成匿名 token（24 位 hex）。
func randomToken() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("匿名 token 生成失败：%w", err)
	}
	return hex.EncodeToString(buf), nil
}

// randomSuffix 生成 4 位随机后缀（匿名昵称默认名用）。
func randomSuffix() string {
	buf := make([]byte, 2)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// Issue 签发匿名身份。
// 参数：nickname 自填昵称（空则默认「匿名访客 + 随机后缀」，设计稿「匿名访客 · 昨天」）。
// 返回：token 与昵称。
func (m *GuestManager) Issue(nickname string) (string, string, error) {
	// 昵称处理：自填（截断 20 字符）或默认「匿名访客 + 4 位后缀」
	name := nickname
	if name == "" {
		name = "匿名访客" + randomSuffix()
	} else if runes := []rune(name); len(runes) > 20 {
		name = string(runes[:20])
	}

	token, err := randomToken()
	if err != nil {
		return "", "", err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// 清理过期条目（防内存膨胀）
	now := time.Now()
	for k, e := range m.entries {
		if now.After(e.expires) {
			delete(m.entries, k)
		}
	}
	m.entries[token] = guestEntry{name: name, expires: now.Add(guestTokenTTL)}
	return token, name, nil
}

// Verify 校验匿名 token 是否有效。
// 返回：有效时返回昵称与 token 哈希（哈希用于落库防刷）。
func (m *GuestManager) Verify(token string) (name string, tokenHash string, ok bool) {
	if token == "" {
		return "", "", false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, exists := m.entries[token]
	if !exists || time.Now().After(entry.expires) {
		if exists {
			delete(m.entries, token)
		}
		return "", "", false
	}
	// 哈希：SHA256(token)（存 comments.guest_token_hash）
	sum := sha256.Sum256([]byte(token))
	return entry.name, hex.EncodeToString(sum[:]), true
}

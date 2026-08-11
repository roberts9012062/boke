// internal/auth/guest.go
// 匿名评论身份：签发短期匿名 token（需求 3.5「开放，无需登录」+ mvp-plan 决策 2）。
//
// 设计（M2：内存 → Redis 化，跨实例共享）：
//   - POST /guest-identity 签发：昵称自填（可选），默认「匿名访客」+ 随机后缀
//   - 存储 Redis 优先（key guest:identity:{token} → 昵称，TTL 7 天），内存兜底（单机开发）
//   - 哈希落库防刷（1 条/分钟/同 token，SHA256 在本地计算，不依赖存储）
//   - 已登录用户直接使用账号身份，不走匿名通道
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// 匿名身份有效期（7 天）。
const guestTokenTTL = 7 * 24 * time.Hour

// guestKeyPrefix Redis 键前缀（匿名身份）。
const guestKeyPrefix = "guest:identity:"

// guestEntry 内存兜底中的匿名身份记录。
type guestEntry struct {
	name    string    // 匿名昵称
	expires time.Time // 过期时间
}

// memoryGuestStore 内存兜底存储（map + 互斥锁，单机语义）。
type memoryGuestStore struct {
	mu      sync.Mutex             // 保护 entries
	entries map[string]guestEntry  // token → 身份记录
}

// newMemoryGuestStore 创建内存兜底存储。
func newMemoryGuestStore() *memoryGuestStore {
	return &memoryGuestStore{entries: make(map[string]guestEntry)}
}

// set 写入身份记录（顺带清理过期条目，防内存膨胀）。
func (m *memoryGuestStore) set(token string, name string, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, e := range m.entries {
		if now.After(e.expires) {
			delete(m.entries, k)
		}
	}
	m.entries[token] = guestEntry{name: name, expires: now.Add(ttl)}
}

// get 读取身份昵称（过期即删，返回空表示无效）。
func (m *memoryGuestStore) get(token string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, exists := m.entries[token]
	if !exists || time.Now().After(entry.expires) {
		if exists {
			delete(m.entries, token)
		}
		return ""
	}
	return entry.name
}

// GuestManager 匿名身份管理器（连接器类，Redis 优先 + 内存兜底）。
type GuestManager struct {
	client *redis.Client     // Redis 客户端（可为 nil，此时走内存兜底）
	memory *memoryGuestStore // 内存兜底
}

// NewGuestManager 创建匿名身份管理器。
// 参数：client Redis 客户端（Redis 不可用时传 nil，自动降级内存）。
func NewGuestManager(client *redis.Client) *GuestManager {
	return &GuestManager{client: client, memory: newMemoryGuestStore()}
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

	// Redis 路径（失败降级内存兜底）
	if m.client != nil {
		if err := m.client.Set(context.Background(), guestKeyPrefix+token, name, guestTokenTTL).Err(); err == nil {
			return token, name, nil
		}
	}
	// 内存兜底路径
	m.memory.set(token, name, guestTokenTTL)
	return token, name, nil
}

// Verify 校验匿名 token 是否有效。
// 返回：有效时返回昵称与 token 哈希（哈希用于落库防刷）。
func (m *GuestManager) Verify(token string) (name string, tokenHash string, ok bool) {
	if token == "" {
		return "", "", false
	}
	// Redis 路径
	if m.client != nil {
		value, err := m.client.Get(context.Background(), guestKeyPrefix+token).Result()
		if err == nil && value != "" {
			return value, hashToken(token), true
		}
	}
	// 内存兜底路径
	name = m.memory.get(token)
	if name == "" {
		return "", "", false
	}
	return name, hashToken(token), true
}

// hashToken 计算 token 的 SHA256 哈希（存 comments.guest_token_hash 防刷）。
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// internal/auth/reset.go
// 密码重置令牌管理（M2 找回密码）：
//   - 内存存储（重启失效；跨实例需 Redis，规划 P1）
//   - token 有效期 30 分钟（设计稿：链接 30 分钟内有效）
//   - 同邮箱 60 秒重发限制（设计稿：60 秒后重新发送）
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// 重置令牌有效期与重发间隔（设计稿：30 分钟有效 / 60 秒重发）。
const (
	resetTTL        = 30 * time.Minute
	resendInterval  = 60 * time.Second
)

// resetEntry 重置令牌条目。
type resetEntry struct {
	email     string    // 目标邮箱
	createdAt time.Time // 签发时间
}

// ResetManager 密码重置令牌管理器（内存，连接器类）。
type ResetManager struct {
	mu      sync.Mutex
	tokens  map[string]resetEntry // token → 条目
	lastReq map[string]time.Time  // email → 上次请求时间（60s 重发限制）
}

// NewResetManager 创建重置令牌管理器。
func NewResetManager() *ResetManager {
	return &ResetManager{tokens: make(map[string]resetEntry), lastReq: make(map[string]time.Time)}
}

// Issue 签发重置令牌（同邮箱 60 秒内重复请求返回错误）。
// 返回：令牌；错误（重发间隔未到）。
func (m *ResetManager) Issue(email string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 60 秒重发限制（设计稿：60 秒后重新发送）
	if last, ok := m.lastReq[email]; ok && time.Since(last) < resendInterval {
		return "", ErrResendTooSoon
	}
	m.lastReq[email] = time.Now()

	// 生成随机令牌（32 位 hex）
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	m.tokens[token] = resetEntry{email: email, createdAt: time.Now()}
	return token, nil
}

// Consume 校验并消费令牌（30 分钟有效；成功后删除）。
// 返回：令牌对应邮箱；无效/过期返回 ErrInvalidResetToken。
func (m *ResetManager) Consume(token string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.tokens[token]
	if !ok {
		return "", ErrInvalidResetToken
	}
	// 过期清理
	if time.Since(entry.createdAt) > resetTTL {
		delete(m.tokens, token)
		return "", ErrInvalidResetToken
	}
	delete(m.tokens, token)
	return entry.email, nil
}

// 重置令牌错误（供 service 层映射业务提示）。
var (
	ErrResendTooSoon    = errInvalid("请 60 秒后重新发送")
	ErrInvalidResetToken = errInvalid("重置链接无效或已过期")
)

// errInvalid 简单错误类型（携带提示文案）。
type errInvalid string

// Error 实现 error 接口。
func (e errInvalid) Error() string {
	return string(e)
}

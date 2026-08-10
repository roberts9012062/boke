// internal/redis/ratelimit.go
// 登录限流与令牌黑名单：Redis 优先，内存兜底（开发环境 Redis 不可用时不降级安全能力）。
//
// 语义（需求 6 安全）：
//   - 登录限流：5 次/分钟/账号
//   - 登出撤销：refresh token 加入黑名单（持有至令牌过期）
// 说明：Redis 与内存实现语义一致（计数窗口 / 黑名单 TTL），
//       生产多实例部署时 Redis 生效，单机开发时内存兜底。
package redis

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// LoginLimitWindow 限流窗口时长（1 分钟）。
const LoginLimitWindow = time.Minute

// LoginLimitMax 窗口内最大尝试次数（5 次/分/账号）。
const LoginLimitMax = 5

// memoryRateLimiter 内存兜底实现（map + 互斥锁，单机语义）。
type memoryRateLimiter struct {
	mu      sync.Mutex                // 保护以下两个 map
	loginAt map[string][]time.Time    // 账号 → 登录尝试时间戳（窗口内）
	blackAt map[string]time.Time      // 令牌 ID → 撤销时间（持有至过期）
}

// newMemoryRateLimiter 创建内存兜底限流器。
func newMemoryRateLimiter() *memoryRateLimiter {
	return &memoryRateLimiter{
		loginAt: make(map[string][]time.Time),
		blackAt: make(map[string]time.Time),
	}
}

// cleanup 清理过期数据（登录窗口外的时间戳 / 已过期的黑名单）。
func (m *memoryRateLimiter) cleanup(now time.Time) {
	for account, times := range m.loginAt {
		kept := make([]time.Time, 0, len(times))
		for _, t := range times {
			if now.Sub(t) < LoginLimitWindow {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(m.loginAt, account)
		} else {
			m.loginAt[account] = kept
		}
	}
	for tokenID, until := range m.blackAt {
		if now.After(until) {
			delete(m.blackAt, tokenID)
		}
	}
}

// RateLimiter 登录限流器（连接器类，Redis 优先 + 内存兜底）。
type RateLimiter struct {
	client *redis.Client // Redis 客户端（可为 nil，此时走内存兜底）
	memory *memoryRateLimiter // 内存兜底
}

// NewRateLimiter 创建限流器。
func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client, memory: newMemoryRateLimiter()}
}

// loginKey 登录限流键（按账号维度）。
func loginKey(account string) string {
	return fmt.Sprintf("account:rate:limit:%s", account)
}

// AllowLogin 判断账号是否允许本次登录尝试（Redis 优先，失败走内存）。
// 返回：允许；已超限返回 false。
func (r *RateLimiter) AllowLogin(ctx context.Context, account string) bool {
	// ---------- Redis 路径 ----------
	if r.client != nil {
		key := loginKey(account)
		count, err := r.client.Incr(ctx, key).Result()
		if err == nil {
			if count == 1 {
				r.client.Expire(ctx, key, LoginLimitWindow)
			}
			return count <= LoginLimitMax
		}
		// Redis 出错：降级内存兜底
	}

	// ---------- 内存兜底路径 ----------
	now := time.Now()
	r.memory.mu.Lock()
	defer r.memory.mu.Unlock()
	r.memory.cleanup(now)

	// 过滤窗口内的时间戳
	times := r.memory.loginAt[account]
	kept := make([]time.Time, 0, len(times)+1)
	for _, t := range times {
		if now.Sub(t) < LoginLimitWindow {
			kept = append(kept, t)
		}
	}
	// 超限拒绝（不记录本次）
	if len(kept) >= LoginLimitMax {
		r.memory.loginAt[account] = kept
		return false
	}
	// 允许并记录本次尝试
	r.memory.loginAt[account] = append(kept, now)
	return true
}

// RevokeToken 将令牌 ID 加入黑名单（登出时撤销 refresh token）。
// 参数：tokenID 令牌 ID；ttl 剩余有效期（黑名单持有时长）。
func (r *RateLimiter) RevokeToken(ctx context.Context, tokenID string, ttl time.Duration) {
	if tokenID == "" {
		return
	}
	// Redis 路径
	if r.client != nil {
		if err := r.client.Set(ctx, "token:blacklist:"+tokenID, "1", ttl).Err(); err == nil {
			return
		}
	}
	// 内存兜底
	r.memory.mu.Lock()
	defer r.memory.mu.Unlock()
	r.memory.blackAt[tokenID] = time.Now().Add(ttl)
}

// IsTokenRevoked 判断令牌是否已被撤销（刷新接口校验）。
func (r *RateLimiter) IsTokenRevoked(ctx context.Context, tokenID string) bool {
	if tokenID == "" {
		return false
	}
	// Redis 路径
	if r.client != nil {
		exists, err := r.client.Exists(ctx, "token:blacklist:"+tokenID).Result()
		if err == nil {
			return exists > 0
		}
	}
	// 内存兜底
	r.memory.mu.Lock()
	defer r.memory.mu.Unlock()
	until, ok := r.memory.blackAt[tokenID]
	return ok && time.Now().Before(until)
}

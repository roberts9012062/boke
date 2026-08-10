// internal/redis/ratelimit_test.go
// 登录限流与令牌黑名单单元测试（内存兜底路径，Redis 不可用场景）。
package redis

import (
	"context"
	"testing"
	"time"
)

// TestRevokeTokenMemory 验证登出撤销与查询（内存兜底）。
func TestRevokeTokenMemory(t *testing.T) {
	limiter := NewRateLimiter(nil) // nil client = 内存兜底
	ctx := context.Background()

	// 未撤销时返回 false
	if limiter.IsTokenRevoked(ctx, "token-1") {
		t.Fatal("未撤销的令牌不应命中黑名单")
	}

	// 撤销后命中
	limiter.RevokeToken(ctx, "token-1", time.Hour)
	if !limiter.IsTokenRevoked(ctx, "token-1") {
		t.Fatal("撤销后的令牌应命中黑名单")
	}
}

// TestAllowLoginLimit 验证登录限流（5 次/分钟上限）。
func TestAllowLoginLimit(t *testing.T) {
	limiter := NewRateLimiter(nil)
	ctx := context.Background()

	// 前 5 次允许
	for i := 1; i <= LoginLimitMax; i++ {
		if !limiter.AllowLogin(ctx, "user@moon.light") {
			t.Fatalf("第 %d 次尝试应被允许", i)
		}
	}
	// 第 6 次拒绝
	if limiter.AllowLogin(ctx, "user@moon.light") {
		t.Fatal("第 6 次尝试应被拒绝（超限）")
	}

	// 其他账号不受影响
	if !limiter.AllowLogin(ctx, "other@moon.light") {
		t.Fatal("其他账号不应受影响")
	}
}

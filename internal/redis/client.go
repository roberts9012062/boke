// internal/redis/client.go
// Redis 客户端创建（MVP 用于登录限流与令牌黑名单；P1 上页面缓存）。
// 说明：Redis 不可用时返回 nil 客户端（调用方降级放行，见 ratelimit.go）。
package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config Redis 连接参数。
type Config struct {
	Host string // 主机
	Port string // 端口
	DB   int    // 数据库编号
}

// NewClient 创建 Redis 客户端并做连通性检查。
// 返回：客户端；连接失败返回 nil（调用方降级处理，不阻断服务启动）。
func NewClient(ctx context.Context, cfg Config) *redis.Client {
	if cfg.Host == "" {
		return nil
	}
	addr := cfg.Host
	if cfg.Port != "" {
		addr += ":" + cfg.Port
	}
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		DB:       cfg.DB,
		Password: "",
	})

	// 连通性检查（失败返回 nil，限流降级放行）
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		client.Close()
		return nil
	}
	return client
}

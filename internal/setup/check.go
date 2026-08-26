// internal/setup/check.go
// 安装向导环境检查与自动配置：检测数据目录、数据库、Redis 依赖，
// 提供一键修复（创建缺失目录、等待容器内数据库就绪）。
package setup

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
)

// 检查项状态常量。
const (
	StatusOK      = "ok"      // 通过
	StatusFail    = "fail"    // 失败（阻断安装，可尝试自动配置）
	StatusWarn    = "warn"    // 警告（不阻断安装，功能降级）
	StatusPending = "pending" // 待用户操作（如裸机模式尚未填写数据库）
)

// CheckItem 单项检查结果。
type CheckItem struct {
	ID      string `json:"id"`       // 检查项标识（data_dir / database / redis）
	Name    string `json:"name"`     // 展示名称
	Status  string `json:"status"`   // ok / fail / warn / pending
	Detail  string `json:"detail"`   // 结果说明
	Fixable bool   `json:"fixable"`  // 是否支持自动配置
}

// CheckResult 环境检查汇总。
type CheckResult struct {
	Mode    string      `json:"mode"`     // docker / manual
	Checks  []CheckItem `json:"checks"`
	Pass    bool        `json:"pass"`     // 是否允许进入下一步（无 fail 且数据库 ok/warn）
}

// Mode 安装模式：SETUP_MODE=docker 表示数据库已由编排自动绑定，其余为裸机手动模式。
func Mode() string {
	if os.Getenv("SETUP_MODE") == "docker" {
		return "docker"
	}
	return "manual"
}

// DBConfigFromEnv 从环境变量读取数据库配置（Docker 模式由 compose 注入）。
func DBConfigFromEnv() DBConfig {
	return DBConfig{
		Host:     os.Getenv("POSTGRES_HOST"),
		Port:     envOr("POSTGRES_PORT", "5432"),
		User:     os.Getenv("POSTGRES_USER"),
		Password: os.Getenv("POSTGRES_PASSWORD"),
		Database: os.Getenv("POSTGRES_DB"),
	}
}

// envOr 读取环境变量，未设置时返回兜底值。
func envOr(key string, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// pingDatabase 探测数据库连通性：连接系统库 postgres 执行 SELECT 1。
// 返回：错误信息（nil = 连通）。
func pingDatabase(ctx context.Context, cfg DBConfig) error {
	connCfg, err := pgx.ParseConfig(cfg.WithDatabase("postgres").ConnString())
	if err != nil {
		return fmt.Errorf("连接串非法：%w", err)
	}
	conn, err := pgx.ConnectConfig(ctx, connCfg)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	var one int
	return conn.QueryRow(ctx, "SELECT 1").Scan(&one)
}

// WithDatabase 返回指向指定数据库的配置副本（不修改原配置）。
func (c DBConfig) WithDatabase(database string) DBConfig {
	clone := c
	clone.Database = database
	return clone
}

// checkDataDir 检查数据目录可写（不存在时视为可修复）。
func checkDataDir(dataDir string) CheckItem {
	item := CheckItem{ID: "data_dir", Name: "数据目录", Fixable: true}
	if err := os.MkdirAll(dataDir, 0o755); err == nil {
		if probe := filepath.Join(dataDir, ".write-test"); os.WriteFile(probe, []byte("1"), 0o600) == nil {
			_ = os.Remove(probe)
			item.Status, item.Detail = StatusOK, fmt.Sprintf("%s 可写", dataDir)
			return item
		}
	}
	item.Status, item.Detail = StatusFail, fmt.Sprintf("数据目录 %s 不可写或无法创建", dataDir)
	return item
}

// checkLogsDir 检查日志目录可写。
func checkLogsDir() CheckItem {
	item := CheckItem{ID: "logs_dir", Name: "日志目录", Fixable: true}
	if err := os.MkdirAll("logs", 0o755); err == nil {
		item.Status, item.Detail = StatusOK, "logs/ 可写"
		return item
	}
	item.Status, item.Detail = StatusFail, "日志目录 logs/ 不可写或无法创建"
	return item
}

// resolveCheckDB 解析当前模式下的待检数据库配置。
// 返回：配置；是否已具备检查条件（裸机模式未填写时为 false）。
func resolveCheckDB(dataDir string, mode string) (DBConfig, bool) {
	if mode == "docker" {
		cfg := DBConfigFromEnv()
		if cfg.Host == "" || cfg.User == "" || cfg.Database == "" {
			return cfg, false
		}
		return cfg, true
	}
	cfg, stashed, _ := LoadStashedDBConfig(dataDir)
	return cfg, stashed
}

// checkDatabase 检查数据库连通性（Docker 模式连编排注入的实例；裸机连用户暂存的配置）。
func checkDatabase(ctx context.Context, dataDir string, mode string) CheckItem {
	item := CheckItem{ID: "database", Name: "PostgreSQL 数据库", Fixable: mode == "docker"}
	cfg, ready := resolveCheckDB(dataDir, mode)
	if !ready {
		if mode == "docker" {
			item.Status, item.Detail = StatusFail, "数据库连接参数未注入（缺少 POSTGRES_* 环境变量）"
			return item
		}
		item.Status, item.Detail = StatusPending, "尚未填写数据库连接信息，请在下一步配置"
		return item
	}
	if err := pingDatabase(ctx, cfg); err != nil {
		item.Status, item.Detail = StatusFail, fmt.Sprintf("无法连接 %s:%s（%v）", cfg.Host, cfg.Port, err)
		return item
	}
	item.Status, item.Detail = StatusOK, fmt.Sprintf("%s:%s/%s 连接正常", cfg.Host, cfg.Port, cfg.Database)
	return item
}

// checkRedis 检查 Redis 连通性（不可用仅告警：登录限流等自动降级）。
func checkRedis(ctx context.Context) CheckItem {
	item := CheckItem{ID: "redis", Name: "Redis 缓存"}
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		item.Status, item.Detail = StatusWarn, "未配置 Redis（限流/黑名单功能自动降级放行）"
		return item
	}
	port := envOr("REDIS_PORT", "6379")
	if err := probeTCP(ctx, host, port); err != nil {
		item.Status, item.Detail = StatusWarn, fmt.Sprintf("%s:%s 不可达（%v），相关功能降级", host, port, err)
		return item
	}
	item.Status, item.Detail = StatusOK, fmt.Sprintf("%s:%s 可达", host, port)
	return item
}

// probeTCP TCP 拨号探测端口连通性（Redis 无 Ping 依赖的轻量探测方式）。
func probeTCP(ctx context.Context, host string, port string) error {
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return err
	}
	return conn.Close()
}

// RunChecks 执行全部环境检查并汇总。
// 通过标准：无 fail，且数据库检查为 ok（pending/warn 不阻断但标记 pass=false 等待补齐）。
func RunChecks(ctx context.Context, dataDir string, mode string) CheckResult {
	checks := []CheckItem{
		checkDataDir(dataDir),
		checkLogsDir(),
		checkDatabase(ctx, dataDir, mode),
		checkRedis(ctx),
	}
	pass := true
	dbReady := false
	for _, item := range checks {
		if item.Status == StatusFail {
			pass = false
		}
		if item.ID == "database" && item.Status == StatusOK {
			dbReady = true
		}
	}
	return CheckResult{Mode: mode, Checks: checks, Pass: pass && dbReady}
}

// RunFix 自动配置缺失依赖：创建数据/日志目录；Docker 模式循环等待数据库就绪。
// 返回：修复过程中数据库等待的错误（目录创建失败直接返回）。
func RunFix(ctx context.Context, dataDir string, mode string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("创建数据目录失败：%w", err)
	}
	if err := os.MkdirAll("logs", 0o755); err != nil {
		return fmt.Errorf("创建日志目录失败：%w", err)
	}
	if mode != "docker" {
		return nil
	}
	// Docker 模式：等待编排注入的数据库就绪（最长 60 秒，每 2 秒重试）
	cfg := DBConfigFromEnv()
	deadline := time.Now().Add(60 * time.Second)
	for {
		if err := pingDatabase(ctx, cfg); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("等待数据库就绪超时（60 秒），请检查 postgres 容器状态")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}


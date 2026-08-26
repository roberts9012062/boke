// internal/config/config.go
// 服务配置加载：从环境变量（.env 由脚本加载）读取全部运行参数。
//
// 职责：集中管理服务端口、JWT 密钥、CORS 来源、数据库连接等配置，
//       供 server / middleware / service 各层读取，避免散落硬编码。
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/roberts9012062/boke/pkg/dbcfg"
)

// Config 服务运行配置（值类型，方法按值接收，不修改原配置）。
type Config struct {
	ServerPort  string       // HTTP 监听端口
	JWTSecret   string       // JWT 签名密钥
	CORSOrigin  string       // 允许的跨域来源（前端地址）
	DB          dbcfg.Config // 数据库连接配置
	Redis       RedisConfig  // Redis 连接配置（限流/黑名单）
	DataDir     string       // 本地数据目录（媒体存储等）
	Mail        MailConfig   // 邮件配置（M2 找回密码；未配置时降级日志输出）
	SiteBaseURL string       // 站点访问地址（生成重置链接）
	GitHubToken string       // GitHub Token（M3.1 插件商城清单拉取；可为空仅公开仓库）
	AIKeySecret string       // AI 供应商 API Key 加密密钥（M4：AES 派生；未配置回退 JWT_SECRET）
	GitHubOAuthClientID string // GitHub OAuth App Client ID（M3.5；空=不启用 OAuth 入口）
	GitHubOAuthSecret   string // GitHub OAuth App Client Secret（M3.5）
}

// MailConfig 邮件发送参数（SMTP，M2 找回密码）。
type MailConfig struct {
	Host     string // SMTP 主机
	Port     string // SMTP 端口
	Username string // 账号
	Password string // 密码
	From     string // 发件人
}

// RedisConfig Redis 连接参数（从环境变量读取）。
type RedisConfig struct {
	Host     string // 主机
	Port     string // 端口
	Password string // 认证密码（Redis 未设密码时为空）
	DB       int    // 数据库编号
}

// Load 从环境变量读取全部运行配置，并校验必填项。
// 返回：配置；缺少必填项时返回错误。
func Load() (Config, error) {
	cfg := Config{
		ServerPort:  os.Getenv("SERVER_PORT"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		CORSOrigin:  os.Getenv("CORS_ORIGIN"),
		DataDir:     os.Getenv("DATA_DIR"),
		GitHubToken: os.Getenv("GITHUB_TOKEN"),
		GitHubOAuthClientID: os.Getenv("GITHUB_OAUTH_CLIENT_ID"),
		GitHubOAuthSecret:   os.Getenv("GITHUB_OAUTH_CLIENT_SECRET"),
	}

	// 默认端口：未配置时使用 8080
	if cfg.ServerPort == "" {
		cfg.ServerPort = "8080"
	}
	// 默认数据目录：项目 data/ 目录
	if cfg.DataDir == "" {
		cfg.DataDir = "data"
	}
	// 默认跨域来源：开发环境前端 3000 端口
	if cfg.CORSOrigin == "" {
		cfg.CORSOrigin = "http://localhost:3000"
	}

	// JWT 密钥为必填项（缺失时服务拒绝启动）
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("环境变量缺失：需要 JWT_SECRET")
	}

	// 加载数据库连接配置（复用 pkg/dbcfg，DRY）
	dbCfg, err := dbcfg.Load()
	if err != nil {
		return Config{}, err
	}
	cfg.DB = dbCfg

	// 加载 Redis 连接配置（未配置时 Host 为空，限流降级放行）
	cfg.Redis = RedisConfig{
		Host:     os.Getenv("REDIS_HOST"),
		Port:     os.Getenv("REDIS_PORT"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	}
	if db := os.Getenv("REDIS_DB"); db != "" {
		if n, err := strconv.Atoi(db); err == nil {
			cfg.Redis.DB = n
		}
	}

	// 加载邮件配置（M2 找回密码；SMTP 未配置时发送降级为日志输出）
	cfg.Mail = MailConfig{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     os.Getenv("SMTP_PORT"),
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
	}
	// 站点访问地址（重置链接前缀；默认开发地址）
	cfg.SiteBaseURL = os.Getenv("SITE_BASE_URL")
	if cfg.SiteBaseURL == "" {
		cfg.SiteBaseURL = "http://localhost:3000"
	}
	// AI 供应商 API Key 加密密钥（M4：AES-256-GCM 派生；未配置时回退 JWT_SECRET
	// 保证开箱即用——E6 安全告警：同源意味着 JWT 泄露等价于机密加密密钥泄露，
	// 且已有密文按该种子加密，直接更换密钥会导致解密失败（轮换需迁移密文，另行设计）。
	// 生产环境务必单独配置 AI_KEY_SECRET，消除此警告）
	cfg.AIKeySecret = os.Getenv("AI_KEY_SECRET")
	if cfg.AIKeySecret == "" {
		cfg.AIKeySecret = cfg.JWTSecret
		fmt.Fprintln(os.Stderr, "[安全告警] AI_KEY_SECRET 未配置，机密加密密钥回退 JWT_SECRET（同源风险）；生产环境请单独配置 AI_KEY_SECRET 并迁移密文")
	}
	return cfg, nil
}

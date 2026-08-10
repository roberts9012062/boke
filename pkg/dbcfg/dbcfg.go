// pkg/dbcfg/dbcfg.go
// 数据库连接配置：从环境变量读取，供各命令行工具（dbcheck / dbinit 等）复用。
package dbcfg

import (
	"fmt"
	"os"
)

// Config 数据库连接参数（值类型，方法均按值接收，不修改原配置）。
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

// Load 从环境变量读取数据库连接参数，并校验必填项。
// 返回：配置信息；缺少必填项时返回错误。
func Load() (Config, error) {
	cfg := Config{
		Host:     os.Getenv("POSTGRES_HOST"),
		Port:     os.Getenv("POSTGRES_PORT"),
		User:     os.Getenv("POSTGRES_USER"),
		Password: os.Getenv("POSTGRES_PASSWORD"),
		Database: os.Getenv("POSTGRES_DB"),
	}
	// 校验必填项：主机、用户、数据库名缺一不可
	if cfg.Host == "" || cfg.User == "" || cfg.Database == "" {
		return Config{}, fmt.Errorf("环境变量缺失：需要 POSTGRES_HOST / POSTGRES_USER / POSTGRES_DB")
	}
	return cfg, nil
}

// ConnString 构造 PostgreSQL 连接串。
// 端口未配置时使用默认端口 5432；开发环境使用 sslmode=disable。
func (c Config) ConnString() string {
	port := c.Port
	if port == "" {
		port = "5432"
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.User, c.Password, c.Host, port, c.Database,
	)
}

// WithDatabase 返回指向指定数据库的配置副本（不修改原配置）。
func (c Config) WithDatabase(database string) Config {
	clone := c
	clone.Database = database
	return clone
}

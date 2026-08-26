// internal/setup/state.go
// 安装向导状态管理：安装锁文件、setup.env 运行配置、向导中间态（数据库配置暂存）。
//
// 约定：
//   - data/install.lock 存在 = 已完成安装，服务以正常模式启动
//   - data/setup.env     安装时生成的运行配置（JWT_SECRET、POSTGRES_* 等），
//                        启动时与进程环境变量合并（环境变量优先）
//   - data/setup-db.json 裸机模式下用户在向导中填写并验证通过的数据库配置（中间态）
package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DBConfig 数据库连接参数（向导内部流转的值类型）。
// 与 pkg/dbcfg.Config 字段一致，独立定义避免包间耦合（setup 不依赖 dbcfg 的环境变量读取）。
type DBConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

// ConnString 构造 PostgreSQL 连接串（端口缺省 5432，本地部署 sslmode=disable）。
func (c DBConfig) ConnString() string {
	port := c.Port
	if port == "" {
		port = "5432"
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.User, c.Password, c.Host, port, c.Database,
	)
}

// LockInfo 安装锁文件内容（记录安装时间与管理员账号，便于运维排查）。
type LockInfo struct {
	InstalledAt   string `json:"installed_at"`
	AdminUsername string `json:"admin_username"`
	Mode          string `json:"mode"`
}

// DataDir 读取数据目录环境变量（未配置时默认 data）。
func DataDir() string {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		return "data"
	}
	return dir
}

// LockPath 安装锁文件路径。
func LockPath(dataDir string) string {
	return filepath.Join(dataDir, "install.lock")
}

// Installed 判断是否已完成安装（锁文件存在即已安装）。
func Installed(dataDir string) bool {
	_, err := os.Stat(LockPath(dataDir))
	return err == nil
}

// WriteLock 写入安装锁文件（内容为 JSON 化的 LockInfo）。
func WriteLock(dataDir string, info LockInfo) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("创建数据目录失败：%w", err)
	}
	raw, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(LockPath(dataDir), raw, 0o600)
}

// ReadLock 读取安装锁文件信息（文件缺失或损坏时返回错误）。
func ReadLock(dataDir string) (LockInfo, error) {
	raw, err := os.ReadFile(LockPath(dataDir))
	if err != nil {
		return LockInfo{}, err
	}
	var info LockInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return LockInfo{}, fmt.Errorf("解析安装锁文件失败：%w", err)
	}
	return info, nil
}

// SetupEnvPath 安装生成的运行配置文件路径。
func SetupEnvPath(dataDir string) string {
	return filepath.Join(dataDir, "setup.env")
}

// WriteSetupEnv 将安装生成的配置写入 data/setup.env（KEY=VALUE 逐行）。
// 值含特殊字符时整体加双引号，避免 shell 解析歧义。
func WriteSetupEnv(dataDir string, kv map[string]string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("创建数据目录失败：%w", err)
	}
	var b strings.Builder
	for key, value := range kv {
		if strings.ContainsAny(value, " \t\"'") {
			b.WriteString(fmt.Sprintf("%s=%q\n", key, value))
		} else {
			b.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		}
	}
	return os.WriteFile(SetupEnvPath(dataDir), []byte(b.String()), 0o600)
}

// ApplySetupEnv 将 setup.env 中的配置合并进进程环境变量。
// 仅当同名环境变量未设置时注入（显式环境变量优先级更高），返回注入的键集合。
func ApplySetupEnv(dataDir string) ([]string, error) {
	raw, err := os.ReadFile(SetupEnvPath(dataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 未生成过配置：全新环境，正常返回
		}
		return nil, err
	}
	applied := make([]string, 0)
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"`)
		if key == "" {
			continue
		}
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, value); err != nil {
				return applied, err
			}
			applied = append(applied, key)
		}
	}
	return applied, nil
}

// dbStashPath 向导中间态文件路径（已验证的数据库配置）。
func dbStashPath(dataDir string) string {
	return filepath.Join(dataDir, "setup-db.json")
}

// StashDBConfig 暂存用户填写并验证通过的数据库配置（向导中间态）。
func StashDBConfig(dataDir string, cfg DBConfig) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(dbStashPath(dataDir), raw, 0o600)
}

// LoadStashedDBConfig 读取暂存的数据库配置；未填写时第二个返回值为 false。
func LoadStashedDBConfig(dataDir string) (DBConfig, bool, error) {
	raw, err := os.ReadFile(dbStashPath(dataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return DBConfig{}, false, nil
		}
		return DBConfig{}, false, err
	}
	var cfg DBConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return DBConfig{}, false, fmt.Errorf("解析暂存数据库配置失败：%w", err)
	}
	return cfg, true, nil
}

// NowISO 当前时间的 ISO8601 格式（锁文件记录用）。
func NowISO() string {
	return time.Now().Format(time.RFC3339)
}

// internal/setup/install.go
// 安装执行器：建库 → 建表（schema.sql）→ 种子（seed.sql）→ 增量迁移 →
// 创建管理员 → 生成 JWT_SECRET 与运行配置（setup.env）→ 写安装锁。
//
// SQL 全部来自 db 内嵌包（go:embed），Docker 环境无需挂载源码目录。
// 建库/建表/迁移逻辑与 cmd/dbinit、cmd/dbmigrate 保持一致（幂等可重复执行）。
package setup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	sitedb "github.com/roberts9012062/boke/db"
)

// AdminAccount 向导收集的管理员账号。
type AdminAccount struct {
	Username string `json:"username"` // 登录用户名（唯一）
	Email    string `json:"email"`    // 登录邮箱（唯一；缺省自动生成）
	Password string `json:"password"` // 登录密码（bcrypt 落库）
	Nickname string `json:"nickname"` // 昵称（缺省同用户名）
}

// InstallRequest 安装请求体（数据库配置仅裸机模式需要；Docker 模式从环境变量读取）。
type InstallRequest struct {
	Admin    AdminAccount `json:"admin"`
	Database *DBConfig    `json:"database,omitempty"`
	// SiteURL 用户当前访问地址（浏览器端上报，如 http://1.2.3.4:3000）。
	// 经前端代理转发会丢失原始 Host 头，以此保证安装完成提示的地址准确。
	SiteURL string `json:"site_url,omitempty"`
}

// InstallResult 安装结果（前后台访问地址与重启方式提示）。
type InstallResult struct {
	FrontendURL   string `json:"frontend_url"`   // 前台首页地址
	AdminURL      string `json:"admin_url"`      // 后台登录地址
	AdminUsername string `json:"admin_username"` // 管理员用户名（回显确认）
	Restart       string `json:"restart"`        // auto=Docker 自动重启生效 / manual=需手动重启服务
}

// schemaMigrationsTable 迁移记录表 DDL（与 cmd/dbmigrate 一致，保证幂等）。
const schemaMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    filename   VARCHAR(255) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`

// connectSimple 以简单查询协议建立连接（多语句 SQL 文件执行依赖此模式）。
func connectSimple(ctx context.Context, connString string) (*pgx.Conn, error) {
	cfg, err := pgx.ParseConfig(connString)
	if err != nil {
		return nil, err
	}
	cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	return pgx.ConnectConfig(ctx, cfg)
}

// ensureDatabase 连接系统库 postgres，目标库不存在时创建（CREATE DATABASE 不能在事务内）。
func ensureDatabase(ctx context.Context, cfg DBConfig) error {
	sysConn, err := connectSimple(ctx, cfg.WithDatabase("postgres").ConnString())
	if err != nil {
		return fmt.Errorf("连接数据库服务器失败（%s:%s）：%w", cfg.Host, cfg.Port, err)
	}
	defer sysConn.Close(context.Background())

	var exists bool
	if err := sysConn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", cfg.Database,
	).Scan(&exists); err != nil {
		return fmt.Errorf("查询数据库是否存在失败：%w", err)
	}
	if exists {
		return nil
	}
	if _, err := sysConn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, cfg.Database)); err != nil {
		return fmt.Errorf("创建数据库 %s 失败：%w", cfg.Database, err)
	}
	return nil
}

// applyMigrations 按文件名字典序执行内嵌迁移（事务 + schema_migrations 记录，幂等）。
func applyMigrations(ctx context.Context, conn *pgx.Conn) error {
	if _, err := conn.Exec(ctx, schemaMigrationsTable); err != nil {
		return fmt.Errorf("创建迁移记录表失败：%w", err)
	}
	entries, err := sitedb.MigrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("读取内嵌迁移目录失败：%w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	applied := make(map[string]bool)
	rows, err := conn.Query(ctx, "SELECT filename FROM schema_migrations")
	if err == nil {
		for rows.Next() {
			var filename string
			if rows.Scan(&filename) == nil {
				applied[filename] = true
			}
		}
		rows.Close()
	}

	for _, name := range names {
		if applied[name] {
			continue
		}
		sql, err := sitedb.MigrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("读取迁移 %s 失败：%w", name, err)
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("开启事务失败（%s）：%w", name, err)
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("执行迁移失败（%s）：%w", name, err)
		}
		if _, err := tx.Exec(ctx,
			"INSERT INTO schema_migrations (filename) VALUES ($1) ON CONFLICT DO NOTHING", name,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("记录迁移失败（%s）：%w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("提交迁移失败（%s）：%w", name, err)
		}
	}
	return nil
}

// createAdmin 创建管理员账号（users 表；用户名或邮箱已存在时报错，不静默覆盖）。
func createAdmin(ctx context.Context, conn *pgx.Conn, admin AdminAccount) error {
	email := admin.Email
	if email == "" {
		email = admin.Username + "@yueyan.site"
	}
	nickname := admin.Nickname
	if nickname == "" {
		nickname = admin.Username
	}

	var takenUsername, takenEmail bool
	if err := conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM users WHERE username = $1), EXISTS(SELECT 1 FROM users WHERE email = $2)",
		admin.Username, email,
	).Scan(&takenUsername, &takenEmail); err != nil {
		return fmt.Errorf("查询管理员账号失败：%w", err)
	}
	if takenUsername {
		return fmt.Errorf("用户名 %s 已存在，请更换", admin.Username)
	}
	if takenEmail {
		return fmt.Errorf("邮箱 %s 已被占用，请更换", email)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(admin.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码哈希失败：%w", err)
	}
	if _, err := conn.Exec(ctx, `
		INSERT INTO users (email, username, password_hash, nickname, status, role)
		VALUES ($1, $2, $3, $4, 'active', 'superadmin')`,
		email, admin.Username, string(hash), nickname); err != nil {
		return fmt.Errorf("创建管理员失败：%w", err)
	}
	// 说明：role 显式写 superadmin——迁移 005/011 只映射存量数据，
	// 新装库默认值 'user' 不属于五级角色，遗漏会导致管理员登录后无后台权限。
	return nil
}

// generateSecret 生成随机密钥（32 字节 hex，用作 JWT_SECRET / AI_KEY_SECRET）。
func generateSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// validateInstallInput 校验安装入参（用户名 3-30 位；密码 ≥ 8 位；数据库名合法）。
func validateInstallInput(req InstallRequest) error {
	name := req.Admin.Username
	if len(name) < 3 || len(name) > 30 {
		return fmt.Errorf("管理员用户名需 3-30 个字符")
	}
	if len(req.Admin.Password) < 8 {
		return fmt.Errorf("管理员密码至少 8 位")
	}
	return nil
}

// RunInstall 执行安装全流程。
// 参数：dataDir 数据目录；mode 安装模式（docker 数据库取环境变量）；baseURL 兜底站点地址（请求头推断）。
func RunInstall(ctx context.Context, dataDir string, mode string, req InstallRequest, fallbackBaseURL string) (InstallResult, error) {
	if err := validateInstallInput(req); err != nil {
		return InstallResult{}, err
	}
	// 站点地址：优先浏览器上报（经代理转发 Host 头不可靠），其次请求头推断
	baseURL := req.SiteURL
	if baseURL == "" {
		baseURL = fallbackBaseURL
	}

	// ---------- 确定数据库配置（Docker 注入优先，其次请求体，最后向导暂存值） ----------
	var dbCfg DBConfig
	switch {
	case mode == "docker":
		dbCfg = DBConfigFromEnv()
		if dbCfg.Host == "" || dbCfg.User == "" || dbCfg.Database == "" {
			return InstallResult{}, fmt.Errorf("数据库环境变量缺失（POSTGRES_HOST/POSTGRES_USER/POSTGRES_DB）")
		}
	case req.Database != nil:
		dbCfg = *req.Database
	default:
		stashed, ok, err := LoadStashedDBConfig(dataDir)
		if err != nil || !ok {
			return InstallResult{}, fmt.Errorf("缺少数据库配置：请填写数据库信息后重试")
		}
		dbCfg = stashed
	}
	if dbCfg.Port == "" {
		dbCfg.Port = "5432"
	}

	// ---------- 建库 → 建表 → 种子 → 迁移 ----------
	if err := ensureDatabase(ctx, dbCfg); err != nil {
		return InstallResult{}, err
	}
	conn, err := connectSimple(ctx, dbCfg.ConnString())
	if err != nil {
		return InstallResult{}, fmt.Errorf("连接目标数据库失败：%w", err)
	}
	defer conn.Close(context.Background())

	if _, err := conn.Exec(ctx, sitedb.SchemaSQL); err != nil {
		return InstallResult{}, fmt.Errorf("执行建表脚本失败：%w", err)
	}
	if _, err := conn.Exec(ctx, sitedb.SeedSQL); err != nil {
		return InstallResult{}, fmt.Errorf("执行种子数据失败：%w", err)
	}
	if err := applyMigrations(ctx, conn); err != nil {
		return InstallResult{}, err
	}

	// ---------- 管理员账号 ----------
	if err := createAdmin(ctx, conn, req.Admin); err != nil {
		return InstallResult{}, err
	}

	// ---------- 生成运行配置与安装锁 ----------
	jwtSecret, err := generateSecret()
	if err != nil {
		return InstallResult{}, fmt.Errorf("生成密钥失败：%w", err)
	}
	if err := WriteSetupEnv(dataDir, map[string]string{
		"POSTGRES_HOST":     dbCfg.Host,
		"POSTGRES_PORT":     dbCfg.Port,
		"POSTGRES_USER":     dbCfg.User,
		"POSTGRES_PASSWORD": dbCfg.Password,
		"POSTGRES_DB":       dbCfg.Database,
		"JWT_SECRET":        jwtSecret,
		"AI_KEY_SECRET":     jwtSecret,
		"SITE_BASE_URL":     baseURL,
	}); err != nil {
		return InstallResult{}, fmt.Errorf("写入运行配置失败：%w", err)
	}
	if err := WriteLock(dataDir, LockInfo{
		InstalledAt:   time.Now().Format(time.RFC3339),
		AdminUsername: req.Admin.Username,
		Mode:          mode,
	}); err != nil {
		return InstallResult{}, fmt.Errorf("写入安装锁失败：%w", err)
	}

	restart := "manual"
	if mode == "docker" {
		restart = "auto"
	}
	return InstallResult{
		FrontendURL:   baseURL,
		AdminURL:      baseURL + "/admin-login",
		AdminUsername: req.Admin.Username,
		Restart:       restart,
	}, nil
}

// cmd/dbmigrate/main.go
// 数据库增量迁移工具：按文件名顺序执行 db/migrations/ 下的全部 .sql 迁移。
//
// 用途：开发流程文档第 5 章「数据库开发流程」——增量变更一律新增
//      db/migrations/00N_描述.sql，由本工具统一执行（幂等，可重复运行）。
// 由 scripts/migrate.sh 调用，配置从环境变量读取。
// 迁移 SQL 从 db.MigrationsFS（go:embed）读取（v1.5.6 起二进制自包含）：
//   容器内无需携带 db/ 目录即可执行（update-agent 部署后自动迁移依赖此特性），
//   且与安装向导（internal/setup）使用同一份嵌入资源，杜绝双来源漂移。
// 注意：本工具绝不打印密码明文。
package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/roberts9012062/boke/db"
	"github.com/roberts9012062/boke/pkg/dbcfg"
)

// schemaMigrationsTable 迁移记录表 DDL（记录已执行的迁移文件名，保证幂等）。
const schemaMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    filename   VARCHAR(255) PRIMARY KEY,     -- 迁移文件名（唯一）
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()  -- 执行时间
)`

// connect 建立数据库连接（使用简单查询协议，以支持多语句 SQL 文件执行）。
func connect(ctx context.Context, connString string) (*pgx.Conn, error) {
	config, err := pgx.ParseConfig(connString)
	if err != nil {
		return nil, err
	}
	// 简单查询协议允许一次执行多条 SQL 语句（迁移文件依赖此特性）
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	return pgx.ConnectConfig(ctx, config)
}

// migrationName 从嵌入路径（migrations/xxx.sql）提取纯文件名。
// schema_migrations 历史记录均为纯文件名，必须保持同名语义（换名会触发唯一键冲突）。
func migrationName(embedPath string) string {
	return path.Base(embedPath)
}

// listMigrationNames 列出嵌入的全部迁移文件名（按文件名升序，纯文件名）。
func listMigrationNames() ([]string, error) {
	embedPaths, err := fs.Glob(db.MigrationsFS, "migrations/*.sql")
	if err != nil {
		return nil, fmt.Errorf("枚举嵌入迁移失败：%w", err)
	}
	names := make([]string, 0, len(embedPaths))
	for _, p := range embedPaths {
		names = append(names, migrationName(p))
	}
	// 按文件名升序（迁移顺序即文件名字典序：001 → 002 → …）
	sort.Strings(names)
	return names, nil
}

// readMigrationSQL 读取指定迁移文件的 SQL 内容（嵌入资源）。
func readMigrationSQL(name string) (string, error) {
	raw, err := fs.ReadFile(db.MigrationsFS, "migrations/"+name)
	if err != nil {
		return "", fmt.Errorf("读取迁移文件失败（%s）：%w", name, err)
	}
	return string(raw), nil
}

// listApplied 查询已执行的迁移文件名集合。
func listApplied(ctx context.Context, conn *pgx.Conn) (map[string]bool, error) {
	rows, err := conn.Query(ctx, "SELECT filename FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return nil, err
		}
		applied[filename] = true
	}
	return applied, rows.Err()
}

// applyMigration 在单个事务中执行迁移 SQL 并记录迁移文件名。
// 任一步失败则整体回滚，保证迁移原子性。
func applyMigration(ctx context.Context, conn *pgx.Conn, name string, sqlText string) error {
	// 开启事务
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("开启事务失败：%w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // 提交成功后回滚无副作用

	// 执行迁移 SQL
	if _, err := tx.Exec(ctx, sqlText); err != nil {
		return fmt.Errorf("执行迁移失败（%s）：%w", name, err)
	}
	// 记录已执行的迁移文件名
	if _, err := tx.Exec(ctx,
		"INSERT INTO schema_migrations (filename) VALUES ($1)", name); err != nil {
		return fmt.Errorf("记录迁移状态失败（%s）：%w", name, err)
	}

	// 提交事务
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("提交事务失败：%w", err)
	}
	return nil
}

// runMigrations 执行全部未应用的迁移。
// 返回：本次执行的迁移数；错误信息。
func runMigrations(ctx context.Context, cfg dbcfg.Config) (int, error) {
	// 连接目标数据库
	conn, err := connect(ctx, cfg.ConnString())
	if err != nil {
		return 0, fmt.Errorf("连接数据库失败：%w", err)
	}
	defer conn.Close(ctx)

	// 确保迁移记录表存在
	if _, err := conn.Exec(ctx, schemaMigrationsTable); err != nil {
		return 0, fmt.Errorf("创建迁移记录表失败：%w", err)
	}

	// 列出全部迁移文件与已执行集合
	names, err := listMigrationNames()
	if err != nil {
		return 0, err
	}
	applied, err := listApplied(ctx, conn)
	if err != nil {
		return 0, fmt.Errorf("查询已执行迁移失败：%w", err)
	}

	// 逐个执行未应用的迁移
	appliedCount := 0
	for _, name := range names {
		if applied[name] {
			fmt.Printf("[跳过] %s（已执行）\n", name)
			continue
		}
		sqlText, err := readMigrationSQL(name)
		if err != nil {
			return appliedCount, err
		}
		fmt.Printf("[执行] %s ...\n", name)
		if err := applyMigration(ctx, conn, name, sqlText); err != nil {
			return appliedCount, err
		}
		appliedCount++
	}
	return appliedCount, nil
}

func main() {
	// 读取数据库配置
	cfg, err := dbcfg.Load()
	if err != nil {
		fmt.Println("[失败] 配置错误：", err)
		os.Exit(1)
	}

	fmt.Println("=== 数据库迁移 ===")
	fmt.Printf("目标：%s:%s，数据库 %s\n", cfg.Host, cfg.Port, cfg.Database)

	// 迁移整体超时 60 秒
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	count, err := runMigrations(ctx, cfg)
	if err != nil {
		fmt.Println("[失败]", err)
		os.Exit(1)
	}
	if count == 0 {
		fmt.Println("[完成] 无待执行的迁移")
	} else {
		fmt.Printf("[完成] 成功执行 %d 个迁移\n", count)
	}
}

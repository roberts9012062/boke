// cmd/dbinit/main.go
// 数据库初始化工具：创建目标数据库 → 执行 schema.sql（建表）→ 执行 seed.sql（种子数据）→ 验证。
//
// 用途：在数据库连接检查通过后，完成「新建数据」的全部工作。
// 由 scripts/init-db.sh 调用，配置从环境变量读取。
// 注意：本工具绝不打印密码明文。
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/roberts9012062/boke/pkg/dbcfg"
)

// connect 建立数据库连接（使用简单查询协议，以支持多语句 SQL 文件执行）。
func connect(ctx context.Context, connString string) (*pgx.Conn, error) {
	config, err := pgx.ParseConfig(connString)
	if err != nil {
		return nil, err
	}
	// 简单查询协议允许一次执行多条 SQL 语句（schema.sql / seed.sql 依赖此特性）
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	return pgx.ConnectConfig(ctx, config)
}

// databaseExists 判断指定数据库是否已存在。
func databaseExists(ctx context.Context, conn *pgx.Conn, database string) (bool, error) {
	var exists bool
	err := conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", database).Scan(&exists)
	return exists, err
}

// createDatabase 创建数据库（库名带引号，保留大小写）。
// 注意：CREATE DATABASE 不能在事务块中执行，必须在非事务连接上直接调用。
func createDatabase(ctx context.Context, conn *pgx.Conn, database string) error {
	_, err := conn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, database))
	return err
}

// execSQLFile 执行单个 SQL 文件（依赖简单查询协议支持多语句）。
func execSQLFile(ctx context.Context, conn *pgx.Conn, path string) error {
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取 SQL 文件失败：%w", err)
	}
	if _, err := conn.Exec(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("执行 SQL 失败（文件 %s）：%w", path, err)
	}
	return nil
}

// listTables 列出目标库 public schema 下的全部业务表。
func listTables(ctx context.Context, conn *pgx.Conn) ([]string, error) {
	rows, err := conn.Query(ctx,
		"SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// initDatabase 执行数据库初始化全流程。
// 返回：错误信息；成功返回 nil。
func initDatabase(ctx context.Context, cfg dbcfg.Config) error {
	// ---------- 第一步：连接系统库，确保目标数据库存在 ----------
	sysConn, err := connect(ctx, cfg.WithDatabase("postgres").ConnString())
	if err != nil {
		return fmt.Errorf("连接系统库失败：%w", err)
	}
	defer sysConn.Close(ctx)

	exists, err := databaseExists(ctx, sysConn, cfg.Database)
	if err != nil {
		return fmt.Errorf("查询数据库是否存在失败：%w", err)
	}
	if exists {
		fmt.Printf("[跳过] 数据库 %s 已存在\n", cfg.Database)
	} else {
		if err := createDatabase(ctx, sysConn, cfg.Database); err != nil {
			return fmt.Errorf("创建数据库失败：%w", err)
		}
		fmt.Printf("[成功] 数据库 %s 创建完成\n", cfg.Database)
	}

	// ---------- 第二步：连接目标库，执行表结构 ----------
	dbConn, err := connect(ctx, cfg.ConnString())
	if err != nil {
		return fmt.Errorf("连接目标数据库失败：%w", err)
	}
	defer dbConn.Close(ctx)

	fmt.Println("[进行] 执行 db/schema.sql（创建表结构）...")
	if err := execSQLFile(ctx, dbConn, "db/schema.sql"); err != nil {
		return err
	}
	fmt.Println("[成功] 表结构创建完成")

	// ---------- 第三步：写入种子数据 ----------
	fmt.Println("[进行] 执行 db/seed.sql（写入种子数据）...")
	if err := execSQLFile(ctx, dbConn, "db/seed.sql"); err != nil {
		return err
	}
	fmt.Println("[成功] 种子数据写入完成")

	// ---------- 第四步：验证结果 ----------
	tables, err := listTables(ctx, dbConn)
	if err != nil {
		return fmt.Errorf("验证表结构失败：%w", err)
	}
	fmt.Printf("[成功] 初始化完成，共创建 %d 张表：\n", len(tables))
	for _, table := range tables {
		fmt.Printf("  - %s\n", table)
	}

	// 验证种子数据是否写入
	var settingsCount int
	if err := dbConn.QueryRow(ctx, "SELECT count(*) FROM settings").Scan(&settingsCount); err != nil {
		return fmt.Errorf("验证种子数据失败：%w", err)
	}
	fmt.Printf("[成功] settings 种子数据 %d 条\n", settingsCount)

	return nil
}

func main() {
	// 读取数据库配置
	cfg, err := dbcfg.Load()
	if err != nil {
		fmt.Println("[失败] 配置错误：", err)
		os.Exit(1)
	}

	fmt.Println("=== 数据库初始化 ===")
	fmt.Printf("目标：%s:%s，数据库 %s\n", cfg.Host, cfg.Port, cfg.Database)

	// 初始化整体超时 60 秒（建表较多）
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := initDatabase(ctx, cfg); err != nil {
		fmt.Println("[失败]", err)
		os.Exit(1)
	}
	fmt.Println("[完成] 数据库初始化成功")
}

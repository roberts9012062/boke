// cmd/dbcheck/main.go
// 数据库与 GitHub 连接检查工具。
//
// 用途：在初始化数据库（建库建表）之前，先验证 PostgreSQL 连接是否可用、
//       GitHub Token 是否有效。由 scripts/check-db.sh 调用，配置从环境变量读取。
//
// 注意：本工具绝不打印密码与 Token 明文。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/roberts9012062/boke/pkg/dbcfg"
)

// 数据库检查结果状态码
const (
	dbStatusOK       = 0 // 连接正常
	dbStatusFail     = 1 // 连接失败（认证/网络问题）
	dbStatusNotExist = 2 // 认证成功，但目标数据库不存在（首次初始化场景）
)

// checkDatabase 尝试连接目标数据库。
// 返回：状态码（见常量）与说明信息。
func checkDatabase(ctx context.Context, connString string) (int, string) {
	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		// 区分「认证成功但库不存在」（3D000）与真正的连接失败
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "3D000" {
			return dbStatusNotExist, "认证成功，但目标数据库不存在（首次初始化时属正常）"
		}
		return dbStatusFail, err.Error()
	}
	defer conn.Close(ctx)

	// 查询 PostgreSQL 版本
	var version string
	if err := conn.QueryRow(ctx, "SELECT version()").Scan(&version); err != nil {
		return dbStatusFail, err.Error()
	}
	// 查询当前连接的数据库名
	var dbName string
	if err := conn.QueryRow(ctx, "SELECT current_database()").Scan(&dbName); err != nil {
		return dbStatusFail, err.Error()
	}
	return dbStatusOK, fmt.Sprintf("PostgreSQL 版本：%s；当前数据库：%s", version, dbName)
}

// listDatabases 连接系统库 postgres，列出服务器上全部非模板数据库。
// 用途：目标数据库连接失败时，帮助排查目标库是否存在。
func listDatabases(ctx context.Context, cfg dbcfg.Config) ([]string, error) {
	// 使用系统库 postgres 建立连接（目标库可能不存在）
	sysCfg := cfg.WithDatabase("postgres")
	conn, err := pgx.Connect(ctx, sysCfg.ConnString())
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx,
		"SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// checkGitHub 验证 GitHub Token 是否有效（调用 /user 接口）。
// 返回：是否有效；有效时附带账号信息，失败时附带原因。绝不打印 Token。
func checkGitHub(ctx context.Context, token string) (bool, string) {
	if token == "" {
		return false, "未配置 GITHUB_TOKEN（跳过）"
	}

	// 构造 GitHub API 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return false, err.Error()
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()

	// 非 200 视为无效（401 为 token 无效，403 常为限流）
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("GitHub API 返回 HTTP %d", resp.StatusCode)
	}

	// 解析账号信息
	var body struct {
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, err.Error()
	}
	return true, fmt.Sprintf("GitHub 账号：%s（%s）", body.Login, body.Name)
}

// runCheck 执行全部检查并输出结果。
// 返回：退出码（0 全部通过 / 1 数据库连接失败 / 2 仅 GitHub 警告）。
func runCheck() int {
	// 读取数据库配置
	cfg, err := dbcfg.Load()
	if err != nil {
		fmt.Println("[失败] 配置错误：", err)
		return dbStatusFail
	}

	// ---------- 第一步：数据库连接检查 ----------
	fmt.Println("=== PostgreSQL 连接检查 ===")
	fmt.Printf("目标：%s:%s 数据库 %s\n", cfg.Host, cfg.Port, cfg.Database)

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	status, msg := checkDatabase(dbCtx, cfg.ConnString())
	dbCancel()

	switch status {
	case dbStatusOK:
		fmt.Println("[成功] 数据库连接正常")
		fmt.Println("       ", msg)
	case dbStatusNotExist:
		fmt.Println("[提示]", msg)
	default:
		fmt.Println("[失败] 目标数据库连接失败：", msg)
		fmt.Println("尝试连接系统库，列出服务器上已有数据库...")

		listCtx, listCancel := context.WithTimeout(context.Background(), 10*time.Second)
		names, listErr := listDatabases(listCtx, cfg)
		listCancel()

		if listErr != nil {
			fmt.Println("[失败] 无法列出数据库：", listErr)
			return dbStatusFail
		}
		fmt.Println("服务器上已有数据库：")
		for _, name := range names {
			fmt.Println("  -", name)
		}
		return dbStatusFail
	}

	// ---------- 第二步：GitHub Token 检查 ----------
	fmt.Println("=== GitHub Token 检查 ===")
	ghCtx, ghCancel := context.WithTimeout(context.Background(), 10*time.Second)
	ghOK, ghMsg := checkGitHub(ghCtx, os.Getenv("GITHUB_TOKEN"))
	ghCancel()

	if ghOK {
		fmt.Println("[成功]", ghMsg)
	} else {
		fmt.Println("[警告]", ghMsg)
	}

	return status
}

func main() {
	os.Exit(runCheck())
}

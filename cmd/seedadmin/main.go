// cmd/seedadmin/main.go
// 管理员账号种子工具：创建/更新站点管理员（username=admin，角色由 Casbin 策略绑定）。
//
// 用途：M1.2 认证阶段写入管理员账号（开发流程文档第 7 章 M1.2 种子）。
// 由 scripts/seed-admin.sh 调用，配置从环境变量读取。
// 说明：
//   - 幂等：admin 已存在时跳过（不重置密码）
//   - 初始密码：优先 ADMIN_PASSWORD 环境变量；缺省自动生成随机密码并打印提示
//   - 首次登录后请在「账号安全」修改密码（MVP 简化：文档记录默认密码，首登提示修改）
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/roberts9012062/boke/pkg/dbcfg"
)

// 管理员账号常量（与 Casbin 策略 g, admin, admin 对应）。
const (
	adminUsername = "admin"              // 用户名（唯一）
	adminEmail    = "admin@yueyan.site"  // 登录邮箱
	adminNickname = "站长"                // 昵称
)

// connect 建立数据库连接（简单查询协议，多语句兼容）。
func connect(ctx context.Context, connString string) (*pgx.Conn, error) {
	config, err := pgx.ParseConfig(connString)
	if err != nil {
		return nil, err
	}
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	return pgx.ConnectConfig(ctx, config)
}

// adminExists 判断管理员账号是否已存在。
func adminExists(ctx context.Context, conn *pgx.Conn) (bool, error) {
	var exists bool
	err := conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)", adminUsername).Scan(&exists)
	return exists, err
}

// randomPassword 生成随机初始密码（12 位，含字母与数字）。
func randomPassword() (string, error) {
	// 用 6 字节随机数 hex 编码（12 位，字母数字组成，满足密码强度校验）
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// seedAdmin 写入管理员账号（幂等：已存在则跳过）。
// 返回：是否新建；初始密码（仅新建时非空）；错误。
func seedAdmin(ctx context.Context, conn *pgx.Conn, password string) (bool, string, error) {
	// 已存在则跳过
	exists, err := adminExists(ctx, conn)
	if err != nil {
		return false, "", err
	}
	if exists {
		return false, "", nil
	}

	// 密码哈希（bcrypt）
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false, "", fmt.Errorf("密码哈希失败：%w", err)
	}

	// 插入管理员（status=active）
	_, err = conn.Exec(ctx, `
		INSERT INTO users (email, username, password_hash, nickname, status)
		VALUES ($1, $2, $3, $4, 'active')`,
		adminEmail, adminUsername, string(hash), adminNickname)
	if err != nil {
		return false, "", fmt.Errorf("创建管理员失败：%w", err)
	}
	return true, password, nil
}

func main() {
	// 读取数据库配置
	cfg, err := dbcfg.Load()
	if err != nil {
		fmt.Println("[失败] 配置错误：", err)
		os.Exit(1)
	}

	fmt.Println("=== 管理员账号种子 ===")
	fmt.Printf("目标：%s:%s，数据库 %s\n", cfg.Host, cfg.Port, cfg.Database)

	// 初始密码：环境变量 ADMIN_PASSWORD 优先，缺省随机生成
	password := os.Getenv("ADMIN_PASSWORD")
	generated := false
	if password == "" {
		password, err = randomPassword()
		if err != nil {
			fmt.Println("[失败] 生成初始密码失败：", err)
			os.Exit(1)
		}
		generated = true
	}

	// 连接数据库并写入
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := connect(ctx, cfg.ConnString())
	if err != nil {
		fmt.Println("[失败] 连接数据库失败：", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	created, finalPassword, err := seedAdmin(ctx, conn, password)
	if err != nil {
		fmt.Println("[失败]", err)
		os.Exit(1)
	}

	if !created {
		fmt.Println("[跳过] 管理员账号已存在（不重置密码）")
		return
	}

	// 打印初始密码（仅新建时；环境变量提供时也打印便于登录）
	fmt.Println("[成功] 管理员账号已创建")
	fmt.Printf("  邮箱：%s\n", adminEmail)
	fmt.Printf("  用户名：%s\n", adminUsername)
	if generated {
		fmt.Printf("  初始密码（自动生成）：%s\n", finalPassword)
	} else {
		fmt.Println("  初始密码：已通过 ADMIN_PASSWORD 环境变量设置")
	}
	fmt.Println("  提示：请登录后台后尽快修改初始密码")
}

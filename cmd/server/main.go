// cmd/server/main.go
// 月言博客主服务入口：加载配置 → 初始化日志 → 启动 HTTP 服务。
//
// 启停约定（AGENTS.md 规则）：由 scripts/dev-server.sh 启动、
// scripts/stop-all.sh 停止，禁止直接 go run 裸命令。
package main

import (
	"fmt"
	"os"

	"go.uber.org/zap"

	"github.com/roberts9012062/boke/internal/config"
	"github.com/roberts9012062/boke/internal/server"
)

func main() {
	// ---------- 第一步：加载运行配置 ----------
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "[失败] 配置加载错误：", err)
		os.Exit(1)
	}

	// ---------- 第二步：初始化日志（logs/ 目录） ----------
	logger, err := server.NewLogger("logs")
	if err != nil {
		fmt.Fprintln(os.Stderr, "[失败] 日志初始化错误：", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck // 退出时刷盘

	// ---------- 第三步：启动 HTTP 服务（阻塞，直至优雅退出） ----------
	if err := server.Run(cfg, logger); err != nil {
		logger.Error("服务运行异常退出", zap.Error(err))
		os.Exit(1)
	}
}

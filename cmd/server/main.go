// cmd/server/main.go
// 月言博客主服务入口：判断安装状态 → 初始化日志 → 安装模式 / 正常模式启动。
//
// 启动流程：
//   1. 合并安装向导生成的配置（data/setup.env，环境变量优先）
//   2. 未安装（无 data/install.lock）→ 安装模式：仅提供 /api/setup/* 向导接口
//   3. 已安装 → 加载完整配置，启动全部业务服务
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
	"github.com/roberts9012062/boke/internal/setup"
)

// serverPort 读取监听端口（未配置时默认 8080）。
func serverPort() string {
	if port := os.Getenv("SERVER_PORT"); port != "" {
		return port
	}
	return "8080"
}

func main() {
	// ---------- 第一步：初始化日志（logs/ 目录；安装模式同样需要日志） ----------
	logger, err := server.NewLogger("logs")
	if err != nil {
		fmt.Fprintln(os.Stderr, "[失败] 日志初始化错误：", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck // 退出时刷盘

	dataDir := setup.DataDir()

	// ---------- 第二步：合并安装向导生成的配置（环境变量优先，幂等） ----------
	if _, err := setup.ApplySetupEnv(dataDir); err != nil {
		logger.Warn("读取安装配置 setup.env 失败，忽略", zap.Error(err))
	}

	// ---------- 第三步：未安装 → 安装模式（仅 /api/setup/* 向导接口） ----------
	if !setup.Installed(dataDir) {
		if err := server.RunSetupMode(serverPort(), dataDir, logger); err != nil {
			logger.Error("安装模式服务异常退出", zap.Error(err))
			os.Exit(1)
		}
		return
	}

	// ---------- 第四步：已安装 → 正常模式（加载完整配置，启动全部业务） ----------
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "[失败] 配置加载错误：", err)
		os.Exit(1)
	}
	if err := server.Run(cfg, logger); err != nil {
		logger.Error("服务运行异常退出", zap.Error(err))
		os.Exit(1)
	}
}

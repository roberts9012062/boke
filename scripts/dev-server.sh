#!/usr/bin/env bash
# scripts/dev-server.sh
# 启动月言博客后端服务（Gin）。
#
# 用法：
#   ./scripts/dev-server.sh            # 启动（前台，Ctrl+C 退出）
#   ./scripts/dev-server.sh --daemon   # 后台启动（PID 写入 .pids/server.pid）
#
# 说明：
#   1. 从项目根目录 .env 加载配置（不打印任何敏感值）
#   2. 启动前自动编译校验（go build ./cmd/server）
#   3. 日志统一输出到 logs/server.log（滚动由 lumberjack 处理）
set -euo pipefail

# 定位项目根目录（脚本所在目录的上一级）
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 日志目录
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"

# PID 文件目录
PID_DIR="$PROJECT_ROOT/.pids"
mkdir -p "$PID_DIR"
PID_FILE="$PID_DIR/server.pid"

# 加载 .env 配置（仅导出环境变量，不回显内容）
if [ ! -f .env ]; then
  echo "[错误] 缺少 .env 配置文件，请先创建"
  exit 1
fi
set -a
# shellcheck disable=SC1091
source .env
set +a

# 若已启动则直接提示退出
if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
  echo "[提示] 后端服务已在运行（PID $(cat "$PID_FILE")），无需重复启动"
  exit 0
fi

# 端口占用预检：8080 已被监听说明存在残留进程（可能是等待中的旧实例）
PORT_IN_USE=$(netstat -ano 2>/dev/null | grep "LISTENING" | grep ":8080 " | head -1 || true)
if [ -n "$PORT_IN_USE" ]; then
  echo "[错误] 端口 8080 已被占用，疑似残留后端进程。请先执行 ./scripts/stop-all.sh 清理后再启动"
  exit 1
fi

# 启动前编译校验（保证 go run 不会因编译错误反复失败）
echo "[检查] 编译校验 cmd/server ..."
go build -o /dev/null ./cmd/server

# 前台启动（终端直接观察日志）
if [ "${1:-}" != "--daemon" ]; then
  echo "[启动] 后端服务前台运行中（日志 logs/server.log），Ctrl+C 停止..."
  go run ./cmd/server 2>&1 | tee -a "$LOG_DIR/server.log"
  exit 0
fi

# 后台启动（日志重定向到 logs/server.log）
echo "[启动] 后端服务后台启动中（日志 logs/server.log）..."
nohup go run ./cmd/server >>"$LOG_DIR/server.log" 2>&1 &
SERVER_PID=$!
echo "$SERVER_PID" > "$PID_FILE"
echo "[完成] 后端服务已后台启动（PID $SERVER_PID），停止请执行 ./scripts/stop-all.sh"

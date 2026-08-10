#!/usr/bin/env bash
# scripts/dev-frontend.sh
# 启动月言博客前端（Next.js dev server）。
#
# 用法：
#   ./scripts/dev-frontend.sh            # 启动（前台，Ctrl+C 退出）
#   ./scripts/dev-frontend.sh --daemon   # 后台启动（PID 写入 .pids/frontend.pid）
#
# 说明：
#   1. 依赖未安装时自动执行 scripts/setup-frontend.sh
#   2. 日志统一输出到 logs/frontend.log
#   3. /api 请求由 next.config.ts 代理到后端 :8080
set -euo pipefail

# 定位项目根目录（脚本所在目录的上一级）
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 日志目录与 PID 目录
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
PID_DIR="$PROJECT_ROOT/.pids"
mkdir -p "$PID_DIR"
PID_FILE="$PID_DIR/frontend.pid"

# 依赖未安装时自动安装（node_modules 不存在）
if [ ! -d "$PROJECT_ROOT/frontend/node_modules" ]; then
  echo "[检查] 前端依赖未安装，执行 scripts/setup-frontend.sh ..."
  bash "$PROJECT_ROOT/scripts/setup-frontend.sh"
fi

# 若已启动则直接提示退出
if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
  echo "[提示] 前端服务已在运行（PID $(cat "$PID_FILE")），无需重复启动"
  exit 0
fi

# 端口占用预检：3000 已被监听说明存在残留前端实例（等待中的旧实例会在端口释放后抢占）
PORT_IN_USE=$(netstat -ano 2>/dev/null | grep "LISTENING" | grep ":3000 " | head -1 || true)
if [ -n "$PORT_IN_USE" ]; then
  echo "[错误] 端口 3000 已被占用，疑似残留前端进程。请先执行 ./scripts/stop-all.sh 清理后再启动"
  exit 1
fi

# 切换到前端目录
cd "$PROJECT_ROOT/frontend"

# 前台启动（终端直接观察日志）
if [ "${1:-}" != "--daemon" ]; then
  echo "[启动] 前端服务前台运行中（http://localhost:3000，日志 logs/frontend.log），Ctrl+C 停止..."
  npm run dev 2>&1 | tee -a "$LOG_DIR/frontend.log"
  exit 0
fi

# 后台启动（日志重定向到 logs/frontend.log）
echo "[启动] 前端服务后台启动中（http://localhost:3000，日志 logs/frontend.log）..."
nohup npm run dev >>"$LOG_DIR/frontend.log" 2>&1 &
FRONTEND_PID=$!
echo "$FRONTEND_PID" > "$PID_FILE"
echo "[完成] 前端服务已后台启动（PID $FRONTEND_PID），停止请执行 ./scripts/stop-all.sh"

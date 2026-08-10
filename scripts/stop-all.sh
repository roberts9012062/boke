#!/usr/bin/env bash
# scripts/stop-all.sh
# 停止全部开发进程（后端服务 + 前端服务）。
#
# 用法：
#   ./scripts/stop-all.sh
#
# 说明：
#   1. 依次读取 .pids/ 下的 PID 文件并发送 SIGTERM（优雅退出）
#   2. 进程停止后清理 PID 文件
#   3. 不存在 PID 文件时静默跳过
set -euo pipefail

# 定位项目根目录（脚本所在目录的上一级）
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 检测是否为 Windows（Git Bash / MSYS / MINGW）：
# npm 派生的 node 子进程不会随父进程 SIGTERM 退出，需用 taskkill /T 终止整个进程树
IS_WINDOWS=0
if [ "$(uname -s 2>/dev/null | grep -icE 'mingw|msys|cygwin')" -gt 0 ]; then
  IS_WINDOWS=1
fi

# PID 文件目录
PID_DIR="$PROJECT_ROOT/.pids"

# 目录不存在说明从未启动过
if [ ! -d "$PID_DIR" ]; then
  echo "[提示] 没有正在运行的开发进程（.pids 目录不存在）"
  exit 0
fi

# 逐个停止：server.pid（后端）/ frontend.pid（前端）
STOPPED=0
for PID_FILE in "$PID_DIR"/*.pid; do
  # 无 PID 文件时跳过
  [ -e "$PID_FILE" ] || continue
  NAME="$(basename "$PID_FILE" .pid)"
  PID="$(cat "$PID_FILE")"

  # PID 无效（非数字）时清理文件并跳过
  if ! [[ "$PID" =~ ^[0-9]+$ ]]; then
    echo "[警告] $NAME PID 文件内容无效（$PID），清理文件"
    rm -f "$PID_FILE"
    continue
  fi

  # 进程已不存在时仅清理文件
  if ! kill -0 "$PID" 2>/dev/null; then
    echo "[提示] $NAME（PID $PID）已不在运行，清理 PID 文件"
    rm -f "$PID_FILE"
    continue
  fi

  # 发送 SIGTERM 优雅退出（等待最多 5 秒）
  echo "[停止] 正在停止 $NAME（PID $PID）..."
  if [ "$IS_WINDOWS" -eq 1 ]; then
    # Windows：taskkill /T 终止整个进程树（npm 子进程一并退出）
    taskkill //PID "$PID" //T //F >/dev/null 2>&1 || true
  else
    kill "$PID" 2>/dev/null || true
    for _ in $(seq 1 50); do
      if ! kill -0 "$PID" 2>/dev/null; then
        break
      fi
      sleep 0.1
    done
    # 5 秒未退出则强制终止
    if kill -0 "$PID" 2>/dev/null; then
      echo "[警告] $NAME（PID $PID）未在 5 秒内退出，强制终止"
      kill -9 "$PID" 2>/dev/null || true
    fi
  fi
  rm -f "$PID_FILE"
  STOPPED=$((STOPPED + 1))
done

# ---------- Windows 补充：按端口反查监听进程终止 ----------
# 说明：nohup 记录的 PID 可能是 bash/npm 包装进程，真实 node/server 进程
#       是其孙进程，taskkill /T 有时无法覆盖；按端口反查最可靠。
if [ "$IS_WINDOWS" -eq 1 ]; then
  for PORT in 3000 8080; do
    # 最多重试 3 轮：等待中的 npm/next 实例会在端口释放后抢占监听
    for _ in 1 2 3; do
      # 注意：grep 无匹配时管道返回 1，pipefail 下会触发 set -e 提前退出，
      # 因此必须加 || true 吞掉管道失败（历史故障：8080 轮被跳过）
      PID=$(netstat -ano 2>/dev/null | grep "LISTENING" | grep ":$PORT " | awk '{print $NF}' | head -1 || true)
      if [ -z "${PID:-}" ]; then
        break
      fi
      echo "[停止] 按端口 $PORT 反查监听进程（PID $PID）..."
      taskkill //PID "$PID" //T //F >/dev/null 2>&1 || true
      STOPPED=$((STOPPED + 1))
      sleep 1
    done
  done
fi

if [ "$STOPPED" -eq 0 ]; then
  echo "[提示] 没有正在运行的开发进程"
else
  echo "[完成] 已停止 $STOPPED 个开发进程"
fi

#!/usr/bin/env bash
# scripts/dev-browser-extension.sh
# 插件开发模式：Vite watch 构建，产物实时输出 dist/browser-extension/。
# 改动后到浏览器扩展页点「重新加载」即可看到效果（手册 §11.2）。
#
# 用法：
#   ./scripts/dev-browser-extension.sh [Ctrl+C 停止]
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EXT_DIR="$PROJECT_ROOT/browser-extension"
VITE_BIN="$EXT_DIR/node_modules/.bin/vite"

if [[ ! -x "$VITE_BIN" ]]; then
  echo "[错误] 未找到 vite，请先执行 ./scripts/setup-browser-extension.sh" >&2
  exit 1
fi

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/browser-extension-dev.log"

echo "[开发] watch 构建启动，产物目录 dist/browser-extension/" | tee -a "$LOG_FILE"
cd "$EXT_DIR"
"$VITE_BIN" build --watch --mode development 2>&1 | tee -a "$LOG_FILE"

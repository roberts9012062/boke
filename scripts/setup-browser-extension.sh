#!/usr/bin/env bash
# scripts/setup-browser-extension.sh
# 浏览器插件首次依赖安装（npm install；存在 lock 时用 npm ci 保证可复现）。
#
# 用法：
#   ./scripts/setup-browser-extension.sh
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EXT_DIR="$PROJECT_ROOT/browser-extension"

# 日志统一输出到 logs/
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/browser-extension-setup-$(date +%Y%m%d-%H%M%S).log"

echo "[安装] browser-extension npm 依赖" | tee -a "$LOG_FILE"

if [[ -f "$EXT_DIR/package-lock.json" ]]; then
  (cd "$EXT_DIR" && npm ci) 2>&1 | tee -a "$LOG_FILE"
else
  (cd "$EXT_DIR" && npm install) 2>&1 | tee -a "$LOG_FILE"
fi

echo "[完成] 依赖就绪，可运行 ./scripts/dev-browser-extension.sh 或 ./scripts/build-browser-extension.sh" | tee -a "$LOG_FILE"

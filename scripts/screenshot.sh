#!/usr/bin/env bash
# scripts/screenshot.sh
# Playwright 截图视觉比对（M1.7）：对关键页面 × 双主题截图到 screenshots/，
# 供与 UI设计/ 导出 PNG（设计稿）人工比对。
#
# 用法：
#   ./scripts/screenshot.sh           # 桌面 + 移动双端
#   ./scripts/screenshot.sh --mobile  # 仅移动端（390px）
#
# 前置：
#   1. 双端已启动（./scripts/dev-server.sh --daemon + ./scripts/dev-frontend.sh --daemon）
#   2. playwright 以 --no-save 装入 frontend/node_modules（首次自动安装）
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 日志目录
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/screenshot-$(date +%Y%m%d-%H%M%S).log"

# 端口预检：双端需已启动
if ! curl -s -o /dev/null http://localhost:8080/healthz; then
  echo "[错误] 后端未启动，请先执行 ./scripts/dev-server.sh --daemon" | tee -a "$LOG_FILE"
  exit 1
fi
if ! curl -s -o /dev/null http://localhost:3000/; then
  echo "[错误] 前端未启动，请先执行 ./scripts/dev-frontend.sh --daemon" | tee -a "$LOG_FILE"
  exit 1
fi

# playwright 依赖：装入 frontend/node_modules（--no-save，不污染 package.json）
if [ ! -d "$PROJECT_ROOT/frontend/node_modules/playwright" ]; then
  echo "[安装] 安装 playwright（仅装入 node_modules，不写入 package.json）..." | tee -a "$LOG_FILE"
  (cd "$PROJECT_ROOT/frontend" && npm i --no-save playwright@1.62.1 2>&1 | tee -a "$LOG_FILE")
fi

# 浏览器：优先复用系统 Chrome（channel:chrome，免下载）；无 Chrome 时提示安装 chromium
if [ ! -f "/c/Program Files/Google/Chrome/Application/chrome.exe" ] && [ ! -f "/c/Program Files (x86)/Google/Chrome/Application/chrome.exe" ]; then
  echo "[安装] 未检测到系统 Chrome，下载 playwright chromium（约 150MB，请耐心等待）..." | tee -a "$LOG_FILE"
  (cd "$PROJECT_ROOT/frontend" && npx playwright install chromium 2>&1 | tee -a "$LOG_FILE")
fi

# 执行截图脚本（NODE_PATH 指向 frontend/node_modules 供 import 解析）
echo "[进行] 截图开始（${1:-双端}）..." | tee -a "$LOG_FILE"
NODE_PATH="$PROJECT_ROOT/frontend/node_modules" node "$PROJECT_ROOT/scripts/screenshot.mjs" ${1:-} 2>&1 | tee -a "$LOG_FILE"

echo "[完成] 截图完成，输出目录：$PROJECT_ROOT/screenshots/（日志 $LOG_FILE）"

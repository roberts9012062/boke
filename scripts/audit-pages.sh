#!/usr/bin/env bash
# scripts/audit-pages.sh
# 全页面巡检（Playwright）：遍历前台/用户/后台全部页面，捕获 console 错误、
# 未捕获异常、4xx/5xx 响应、请求失败，输出巡检报告（重点：React key 警告排查）。
#
# 用法：
#   ./scripts/audit-pages.sh            # 默认巡检 :3000（dev）
#   AUDIT_FRONT_PORT=3100 ./scripts/audit-pages.sh   # 生产模式（next start）巡检
#
# 前置：
#   1. 双端已启动（./scripts/dev-server.sh --daemon + ./scripts/dev-frontend.sh --daemon）
#   2. playwright 已装入 frontend/node_modules（首次自动安装）
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 日志目录
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/audit-pages-$(date +%Y%m%d-%H%M%S).log"

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

# 执行巡检脚本（NODE_PATH 指向 frontend/node_modules 供 import 解析）
echo "[进行] 全页面巡检开始..." | tee -a "$LOG_FILE"
NODE_PATH="$PROJECT_ROOT/frontend/node_modules" node "$PROJECT_ROOT/scripts/audit-pages.mjs" 2>&1 | tee -a "$LOG_FILE"

echo "[完成] 巡检结束（日志 $LOG_FILE）"

#!/usr/bin/env bash
# scripts/setup-frontend.sh
# 安装前端依赖（frontend/ 内 npm install）。
#
# 用法：
#   ./scripts/setup-frontend.sh
#
# 说明：
#   1. 依赖版本固定于 frontend/package.json（Next 15.4 / React 19 / Tailwind v4）
#   2. 日志输出到 logs/ 目录（终端同步显示）
#   3. 安装完成后校验 TypeScript 类型（npx tsc --noEmit）
set -euo pipefail

# 定位项目根目录（脚本所在目录的上一级）
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT/frontend"

# 日志目录与日志文件
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/setup-frontend-$(date +%Y%m%d-%H%M%S).log"

# 校验 package.json 存在
if [ ! -f package.json ]; then
  echo "[错误] frontend/package.json 不存在，无法安装依赖"
  exit 1
fi

echo "[步骤 1/2] 安装 npm 依赖..." | tee -a "$LOG_FILE"
npm install 2>&1 | tee -a "$LOG_FILE" || true
NPM_CODE=${PIPESTATUS[0]}
if [ "$NPM_CODE" -ne 0 ]; then
  echo "[错误] npm install 失败（退出码 $NPM_CODE），详见日志：$LOG_FILE"
  exit 1
fi

echo "[步骤 2/2] TypeScript 类型校验..." | tee -a "$LOG_FILE"
npx tsc --noEmit 2>&1 | tee -a "$LOG_FILE" || true
TSC_CODE=${PIPESTATUS[0]}
if [ "$TSC_CODE" -ne 0 ]; then
  echo "[错误] TypeScript 类型校验失败（退出码 $TSC_CODE），详见日志：$LOG_FILE"
  exit 1
fi

echo "[完成] 前端依赖就绪，日志：$LOG_FILE"

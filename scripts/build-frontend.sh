#!/usr/bin/env bash
# scripts/build-frontend.sh
# 前端生产构建校验（next build）。
#
# 用法：
#   ./scripts/build-frontend.sh
#
# 说明：
#   1. 依赖未安装时自动执行 scripts/setup-frontend.sh
#   2. 构建产物输出到 frontend/.next（gitignore）
#   3. 日志输出到 logs/ 目录
set -euo pipefail

# 定位项目根目录（脚本所在目录的上一级）
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 日志目录与日志文件
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/build-frontend-$(date +%Y%m%d-%H%M%S).log"

# 依赖未安装时自动安装
if [ ! -d "$PROJECT_ROOT/frontend/node_modules" ]; then
  echo "[检查] 前端依赖未安装，执行 scripts/setup-frontend.sh ..."
  bash "$PROJECT_ROOT/scripts/setup-frontend.sh"
fi

# 关键防护：next build 会重写 .next 目录，与 dev server 混跑会损坏 dev 缓存
# （历史故障：M1.1/M1.2 两次 build 后首页 500）。构建前自动停止 dev 服务。
echo "[检查] 构建前停止开发服务（防止 .next 缓存损坏）..."
bash "$PROJECT_ROOT/scripts/stop-all.sh" || true

cd "$PROJECT_ROOT/frontend"
echo "[进行] next build 生产构建..." | tee -a "$LOG_FILE"
npm run build 2>&1 | tee -a "$LOG_FILE" || true
BUILD_CODE=${PIPESTATUS[0]}
if [ "$BUILD_CODE" -ne 0 ]; then
  echo "[错误] 前端构建失败（退出码 $BUILD_CODE），详见日志：$LOG_FILE"
  exit 1
fi

echo "[完成] 前端生产构建成功，日志：$LOG_FILE"

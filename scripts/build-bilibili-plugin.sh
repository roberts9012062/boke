#!/usr/bin/env bash
# scripts/build-bilibili-plugin.sh
# 编译 B站视频插件（cmd/bilibili-video-plugin）到 data/plugins/bilibili-video/，
# 并同步前端扩展资产（frontend/）供本地开发验证。
#
# 用法：
#   ./scripts/build-bilibili-plugin.sh
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

OUT_DIR="$PROJECT_ROOT/data/plugins/bilibili-video"
BIN_NAME="plugin"
UNAME_S="$(uname -s)"
if [[ "$UNAME_S" == MINGW* || "$UNAME_S" == MSYS* || "$UNAME_S" == CYGWIN* ]]; then
  BIN_NAME="plugin.exe"
fi

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/build-bilibili-plugin-$(date +%Y%m%d-%H%M%S).log"

echo "[构建] cmd/bilibili-video-plugin → $OUT_DIR/$BIN_NAME" | tee -a "$LOG_FILE"
mkdir -p "$OUT_DIR"
go build -C marketplace-repo -ldflags "-s -w" -trimpath -o "$OUT_DIR/$BIN_NAME" ./bilibili-video 2>&1 | tee -a "$LOG_FILE"

echo "[同步] 清单 → $OUT_DIR/" | tee -a "$LOG_FILE"
cp "$PROJECT_ROOT/marketplace-repo/bilibili-video/yueyan-plugin.json" "$OUT_DIR/yueyan-plugin.json" 2>&1 | tee -a "$LOG_FILE"

echo "[同步] 前端资产 → $OUT_DIR/frontend/" | tee -a "$LOG_FILE"
mkdir -p "$OUT_DIR/frontend"
cp -r "$PROJECT_ROOT/marketplace-repo/bilibili-video/frontend/." "$OUT_DIR/frontend/" 2>&1 | tee -a "$LOG_FILE"

echo "[完成] B站视频插件就绪：$OUT_DIR/（二进制 + 清单 + 前端资产），日志：$LOG_FILE" | tee -a "$LOG_FILE"

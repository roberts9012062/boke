#!/usr/bin/env bash
# scripts/build-qq-music-plugin.sh
# 编译 QQ 音乐插件（cmd/qq-music-plugin）到 data/plugins/qq-music/，
# 并同步前端扩展资产（frontend/）供本地开发验证。
#
# 用法：
#   ./scripts/build-qq-music-plugin.sh
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

OUT_DIR="$PROJECT_ROOT/data/plugins/qq-music"
BIN_NAME="plugin"
UNAME_S="$(uname -s)"
if [[ "$UNAME_S" == MINGW* || "$UNAME_S" == MSYS* || "$UNAME_S" == CYGWIN* ]]; then
  BIN_NAME="plugin.exe"
fi

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/build-qq-music-plugin-$(date +%Y%m%d-%H%M%S).log"

echo "[构建] cmd/qq-music-plugin → $OUT_DIR/$BIN_NAME" | tee -a "$LOG_FILE"
mkdir -p "$OUT_DIR"
go build -o "$OUT_DIR/$BIN_NAME" ./cmd/qq-music-plugin 2>&1 | tee -a "$LOG_FILE"

echo "[同步] 前端资产 → $OUT_DIR/frontend/" | tee -a "$LOG_FILE"
mkdir -p "$OUT_DIR/frontend"
cp -r "$PROJECT_ROOT/cmd/qq-music-plugin/frontend/." "$OUT_DIR/frontend/" 2>&1 | tee -a "$LOG_FILE"

echo "[完成] QQ 音乐插件就绪：$OUT_DIR/（二进制 + 前端资产），日志：$LOG_FILE" | tee -a "$LOG_FILE"

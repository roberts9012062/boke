#!/usr/bin/env bash
# scripts/build-netease-music-plugin.sh
# 编译网易云音乐插件（cmd/netease-music-plugin）到 data/plugins/netease-music/，
# 并同步前端扩展资产（frontend/）供本地开发验证。
#
# 用法：
#   ./scripts/build-netease-music-plugin.sh
#
# 说明：
#   - 产物为当前平台二进制（Windows .exe / Linux/macOS 无后缀）
#   - 前端资产复制到 data/plugins/netease-music/frontend/（与 .bpk 解包落点一致）
#   - 替换二进制后需重启插件进程（后台禁用再启用，或重启后端）
set -euo pipefail

# 定位项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

OUT_DIR="$PROJECT_ROOT/data/plugins/netease-music"
BIN_NAME="plugin"
UNAME_S="$(uname -s)"
if [[ "$UNAME_S" == MINGW* || "$UNAME_S" == MSYS* || "$UNAME_S" == CYGWIN* ]]; then
  BIN_NAME="plugin.exe"
fi

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/build-netease-music-plugin-$(date +%Y%m%d-%H%M%S).log"

echo "[构建] cmd/netease-music-plugin → $OUT_DIR/$BIN_NAME" | tee -a "$LOG_FILE"
mkdir -p "$OUT_DIR"
go build -o "$OUT_DIR/$BIN_NAME" ./cmd/netease-music-plugin 2>&1 | tee -a "$LOG_FILE"

echo "[同步] 前端资产 → $OUT_DIR/frontend/" | tee -a "$LOG_FILE"
mkdir -p "$OUT_DIR/frontend"
cp -r "$PROJECT_ROOT/cmd/netease-music-plugin/frontend/." "$OUT_DIR/frontend/" 2>&1 | tee -a "$LOG_FILE"

echo "[完成] 网易云音乐插件就绪：$OUT_DIR/（二进制 + 前端资产），日志：$LOG_FILE" | tee -a "$LOG_FILE"

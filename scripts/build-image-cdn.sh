#!/usr/bin/env bash
# scripts/build-image-cdn.sh
# 编译 Cloudflare 图床插件（marketplace-repo/image-cdn）到 data/plugins/image-cdn/plugin(.exe)。
#
# 用法：
#   ./scripts/build-image-cdn.sh
#
# 说明：
#   - 产物为当前平台二进制（Windows .exe / Linux/macOS 无后缀），供进程外插件验证
#   - 首次 Build 前自动执行 go mod tidy（同步 marketplace-repo 模块依赖）
#   - 打包 .bpk 安装包另用：./scripts/pack-plugin.sh image-cdn
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

OUT_DIR="$PROJECT_ROOT/data/plugins/image-cdn"
BIN_NAME="plugin"
UNAME_S="$(uname -s)"
if [[ "$UNAME_S" == MINGW* || "$UNAME_S" == MSYS* || "$UNAME_S" == CYGWIN* ]]; then
  BIN_NAME="plugin.exe"
fi

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/build-image-cdn-$(date +%Y%m%d-%H%M%S).log"

echo "[构建] 同步依赖（marketplace-repo go.mod/go.sum）..." | tee -a "$LOG_FILE"
go mod tidy -C marketplace-repo 2>&1 | tee -a "$LOG_FILE"

echo "[构建] marketplace-repo/image-cdn → $OUT_DIR/$BIN_NAME" | tee -a "$LOG_FILE"
mkdir -p "$OUT_DIR"
go build -C marketplace-repo -ldflags "-s -w" -trimpath -o "$OUT_DIR/$BIN_NAME" ./image-cdn 2>&1 | tee -a "$LOG_FILE"

echo "[完成] Cloudflare 图床插件二进制就绪：$OUT_DIR/$BIN_NAME，日志：$LOG_FILE" | tee -a "$LOG_FILE"

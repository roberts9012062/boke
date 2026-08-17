#!/usr/bin/env bash
# scripts/build-netease-music-bpk.sh
# 构建网易云音乐插件 .bpk 安装包（编译 → cmd/bp 打包，含前端扩展）。
#
# 用法：
#   ./scripts/build-netease-music-bpk.sh
#
# 产物：dist/netease-music-{version}-{os}-{arch}.bpk（后台「我的插件 → 上传 .bpk」安装）
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/build-netease-music-bpk-$(date +%Y%m%d-%H%M%S).log"

BUILD_DIR="$PROJECT_ROOT/dist/netease-music"
mkdir -p "$BUILD_DIR"

echo "[步骤 1/2] 编译插件二进制（当前平台）..." | tee -a "$LOG_FILE"
GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"
go build -o "$BUILD_DIR/plugin.bin" -C marketplace-repo ./netease-music 2>&1 | tee -a "$LOG_FILE"

echo "[步骤 2/2] 打包 .bpk（免费插件 + 前端扩展）..." | tee -a "$LOG_FILE"
go run ./cmd/bp pack \
  -plugin "marketplace-repo/netease-music/yueyan-plugin.json" \
  -bin "$BUILD_DIR/plugin.bin" \
  -frontend "marketplace-repo/netease-music/frontend" \
  -os "$GOOS" -arch "$GOARCH" \
  -version 0.1.0 \
  -out "dist" 2>&1 | tee -a "$LOG_FILE"

echo "[完成] .bpk 安装包就绪：$PROJECT_ROOT/dist/netease-music-0.1.0-$GOOS-$GOARCH.bpk，日志：$LOG_FILE"

#!/usr/bin/env bash
# scripts/build-demo-bpk.sh
# 构建 M3.4 演示插件 .bpk 安装包（编译 demo → cmd/bp 打包）。
#
# 用法：
#   ./scripts/build-demo-bpk.sh
#
# 说明：
#   - 产物：dist/demo-plugin-{version}-{os}-{arch}.bpk（冒烟本地上传安装验证用）
#   - 平台取当前构建环境（GOOS/GOARCH）
set -euo pipefail

# 定位项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 日志
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/build-demo-bpk-$(date +%Y%m%d-%H%M%S).log"

# 临时构建目录
BUILD_DIR="$PROJECT_ROOT/dist/demo-plugin"
mkdir -p "$BUILD_DIR"

echo "[步骤 1/2] 编译 demo 插件二进制（当前平台）..." | tee -a "$LOG_FILE"
GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"
BIN_NAME="plugin.bin"
go build -o "$BUILD_DIR/$BIN_NAME" ./cmd/demo-plugin 2>&1 | tee -a "$LOG_FILE"

echo "[步骤 2/2] 打包 .bpk（cmd/bp pack，含许可证公钥 + 前端扩展）..." | tee -a "$LOG_FILE"
go run ./cmd/bp pack \
  -plugin "cmd/demo-plugin/yueyan-plugin.json" \
  -bin "$BUILD_DIR/$BIN_NAME" \
  -pubkey "data/demo-keys/public.pem" \
  -frontend "cmd/demo-plugin/frontend" \
  -os "$GOOS" -arch "$GOARCH" \
  -version 0.2.0 \
  -out "dist" 2>&1 | tee -a "$LOG_FILE"

echo "[完成] .bpk 安装包就绪：$PROJECT_ROOT/dist/demo-plugin-0.2.0-$GOOS-$GOARCH.bpk，日志：$LOG_FILE"

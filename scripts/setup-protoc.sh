#!/usr/bin/env bash
# scripts/setup-protoc.sh
# 安装 proto 工具链（M3.3 插件 gRPC 契约生成用）：
#   1. go install buf（纯 Go 的 protoc 前端编译器，走 GOPROXY 无需下载 GitHub 二进制）
#   2. go install protoc-gen-go / protoc-gen-go-grpc 生成插件
#
# 用法：
#   ./scripts/setup-protoc.sh
#
# 说明：
#   - 工具链为本地开发产物，不随仓库分发
#   - 生成的 .pb.go 桩代码提交仓库，第三方插件作者无需安装任何工具
set -euo pipefail

# 定位项目根目录（脚本所在目录的上一级）
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

GOBIN="$(go env GOPATH)/bin"

# 日志
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/setup-protoc-$(date +%Y%m%d-%H%M%S).log"

echo "[步骤 1/2] 安装 buf 编译器..." | tee -a "$LOG_FILE"
if [ -f "$GOBIN/buf.exe" ] || [ -f "$GOBIN/buf" ]; then
  echo "  buf 已存在：$GOBIN/buf(.exe)（跳过）" | tee -a "$LOG_FILE"
else
  go install github.com/bufbuild/buf/cmd/buf@latest 2>&1 | tee -a "$LOG_FILE"
fi
"$GOBIN/buf.exe" --version 2>/dev/null || "$GOBIN/buf" --version | tee -a "$LOG_FILE"

echo "[步骤 2/2] 安装 Go 生成插件（protoc-gen-go / protoc-gen-go-grpc）..." | tee -a "$LOG_FILE"
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest 2>&1 | tee -a "$LOG_FILE"
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest 2>&1 | tee -a "$LOG_FILE"
echo "  生成插件就绪：$GOBIN/protoc-gen-go(.exe)、protoc-gen-go-grpc(.exe)" | tee -a "$LOG_FILE"

echo "[完成] proto 工具链就绪，日志：$LOG_FILE"

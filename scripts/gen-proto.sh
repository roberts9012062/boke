#!/usr/bin/env bash
# scripts/gen-proto.sh
# 生成插件 gRPC 契约桩代码（M3.3）：pkg/plugin-sdk/proto/plugin.pb.go + plugin_grpc.pb.go
#
# 用法：
#   ./scripts/gen-proto.sh        # 首次需先运行 ./scripts/setup-protoc.sh
#
# 说明：
#   - 生成的 .pb.go 提交仓库（版本随契约变更更新）
#   - 契约 proto 文件：pkg/plugin-sdk/proto/plugin.proto（buf.gen.yaml 生成配置）
set -euo pipefail

# 定位项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT/pkg/plugin-sdk/proto"

GOBIN="$(go env GOPATH)/bin"

# buf 可执行文件（Windows .exe / 其他平台无后缀）
BUF_BIN="$GOBIN/buf.exe"
if [ ! -f "$BUF_BIN" ]; then
  BUF_BIN="$GOBIN/buf"
fi
if [ ! -f "$BUF_BIN" ]; then
  echo "[错误] buf 未安装，请先运行 ./scripts/setup-protoc.sh"
  exit 1
fi

# 日志
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/gen-proto-$(date +%Y%m%d-%H%M%S).log"

echo "[生成] plugin.proto → plugin.pb.go + plugin_grpc.pb.go" | tee -a "$LOG_FILE"
PATH="$GOBIN:$PATH" "$BUF_BIN" generate 2>&1 | tee -a "$LOG_FILE"

echo "[完成] 桩代码已生成，日志：$LOG_FILE"

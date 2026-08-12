#!/usr/bin/env bash
# scripts/setup-backend.sh
# 安装后端 Go 依赖（go get + go mod tidy），并编译校验全部 cmd/ 工具。
#
# 用法：
#   ./scripts/setup-backend.sh
#
# 说明：
#   1. go get 安装 go.mod 声明与新增依赖（幂等，可重复执行）
#   2. go mod tidy 整理依赖
#   3. go build ./... 校验代码可编译
#   4. 日志输出到 logs/ 目录
set -euo pipefail

# 定位项目根目录（脚本所在目录的上一级）
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 日志目录与日志文件
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/setup-backend-$(date +%Y%m%d-%H%M%S).log"

echo "[步骤 1/3] 安装 Go 依赖（go get + tidy）..." | tee -a "$LOG_FILE"
# 项目依赖清单（新增依赖在此声明，统一走脚本安装）
DEPS=(
  "github.com/gin-gonic/gin"
  "go.uber.org/zap"
  "gopkg.in/natefinch/lumberjack.v2"
  "github.com/golang-jwt/jwt/v5"
  "golang.org/x/crypto"
  "github.com/casbin/casbin/v2"
  "github.com/redis/go-redis/v9"
  # M3.3 插件进程外化：go-plugin 握手 + gRPC 通信 + protobuf 消息
  "github.com/hashicorp/go-plugin"
  "google.golang.org/grpc"
  "google.golang.org/protobuf"
)
for dep in "${DEPS[@]}"; do
  go get "$dep" 2>&1 | tee -a "$LOG_FILE" || true
done
go mod tidy 2>&1 | tee -a "$LOG_FILE" || true
TIDY_CODE=${PIPESTATUS[0]}
if [ "$TIDY_CODE" -ne 0 ]; then
  echo "[错误] go mod tidy 失败（退出码 $TIDY_CODE），详见日志：$LOG_FILE"
  exit 1
fi

echo "[步骤 2/3] 编译校验全部 Go 工具..." | tee -a "$LOG_FILE"
go build ./... 2>&1 | tee -a "$LOG_FILE" || true
BUILD_CODE=${PIPESTATUS[0]}
if [ "$BUILD_CODE" -ne 0 ]; then
  echo "[错误] go build 失败（退出码 $BUILD_CODE），详见日志：$LOG_FILE"
  exit 1
fi

echo "[步骤 3/3] go vet 静态检查..." | tee -a "$LOG_FILE"
go vet ./... 2>&1 | tee -a "$LOG_FILE" || true
VET_CODE=${PIPESTATUS[0]}
if [ "$VET_CODE" -ne 0 ]; then
  echo "[错误] go vet 失败（退出码 $VET_CODE），详见日志：$LOG_FILE"
  exit 1
fi

echo "[完成] 后端依赖就绪，日志：$LOG_FILE"

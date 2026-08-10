#!/usr/bin/env bash
# scripts/test.sh
# 运行后端单元测试（go test）。
#
# 用法：
#   ./scripts/test.sh [包路径]   # 默认 ./...
#
# 说明：
#   1. 日志输出到 logs/ 目录
#   2. 退出码非 0 表示存在失败用例
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/test-$(date +%Y%m%d-%H%M%S).log"

echo "[进行] go test ${1:-./...} ..." | tee -a "$LOG_FILE"
go test "${1:-./...}" -count=1 -v 2>&1 | tee -a "$LOG_FILE" || true
TEST_CODE=${PIPESTATUS[0]}
if [ "$TEST_CODE" -ne 0 ]; then
  echo "[错误] 测试失败（退出码 $TEST_CODE），详见日志：$LOG_FILE"
  exit 1
fi
echo "[完成] 测试通过，日志：$LOG_FILE"

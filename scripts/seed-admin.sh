#!/usr/bin/env bash
# scripts/seed-admin.sh
# 写入管理员账号（幂等：已存在则跳过）。
#
# 用法：
#   ./scripts/seed-admin.sh
#   ADMIN_PASSWORD=自定义密码 ./scripts/seed-admin.sh   # 指定初始密码
#
# 说明：
#   1. 从项目根目录 .env 加载配置（不打印任何敏感值）
#   2. 运行 cmd/seedadmin 创建/跳过管理员账号
#   3. 日志输出到 logs/ 目录
set -euo pipefail

# 定位项目根目录（脚本所在目录的上一级）
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 日志目录与日志文件
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/seed-admin-$(date +%Y%m%d-%H%M%S).log"

# 加载 .env 配置（仅导出环境变量，不回显内容）
if [ ! -f .env ]; then
  echo "[错误] 缺少 .env 配置文件，请先创建"
  exit 1
fi
set -a
# shellcheck disable=SC1091
source .env
set +a

# 执行种子工具
go run ./cmd/seedadmin 2>&1 | tee -a "$LOG_FILE" || true
SEED_CODE=${PIPESTATUS[0]}
if [ "$SEED_CODE" -ne 0 ]; then
  echo "[错误] 管理员账号写入失败（退出码 $SEED_CODE），详见日志：$LOG_FILE"
  exit 1
fi

echo "[完成] 管理员种子执行成功，日志：$LOG_FILE"

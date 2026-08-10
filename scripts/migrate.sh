#!/usr/bin/env bash
# scripts/migrate.sh
# 执行数据库增量迁移（db/migrations/ 下全部 .sql，幂等可重复执行）。
#
# 用法：
#   ./scripts/migrate.sh
#
# 说明：
#   1. 从项目根目录 .env 加载配置（不打印任何敏感值）
#   2. 运行 cmd/dbmigrate 按文件名顺序执行未应用的迁移
#   3. 日志输出到 logs/ 目录（终端同步显示）
set -euo pipefail

# 定位项目根目录（脚本所在目录的上一级）
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 日志目录与日志文件
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/migrate-$(date +%Y%m%d-%H%M%S).log"

# 加载 .env 配置（仅导出环境变量，不回显内容）
if [ ! -f .env ]; then
  echo "[错误] 缺少 .env 配置文件，请先创建"
  exit 1
fi
set -a
# shellcheck disable=SC1091
source .env
set +a

# 执行迁移工具
go run ./cmd/dbmigrate 2>&1 | tee -a "$LOG_FILE" || true
MIGRATE_CODE=${PIPESTATUS[0]}
if [ "$MIGRATE_CODE" -ne 0 ]; then
  echo "[错误] 数据库迁移失败（退出码 $MIGRATE_CODE），详见日志：$LOG_FILE"
  exit 1
fi

echo "[完成] 数据库迁移成功，日志：$LOG_FILE"

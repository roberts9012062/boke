#!/usr/bin/env bash
# scripts/init-db.sh
# 初始化数据库：创建数据库 → 建表 → 种子数据。
#
# 用法：
#   ./scripts/init-db.sh
#
# 说明：
#   1. 先执行连接检查（scripts/check-db.sh），连接通过后才允许初始化
#   2. 全程日志输出到 logs/ 目录（终端同步显示）
#   3. 脚本幂等：数据库/表已存在时自动跳过，可重复执行
set -euo pipefail

# 定位项目根目录（脚本所在目录的上一级）
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 日志目录与日志文件
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/init-db-$(date +%Y%m%d-%H%M%S).log"

# 加载 .env 配置（仅导出环境变量，不回显内容）
if [ ! -f .env ]; then
  echo "[错误] 缺少 .env 配置文件，请先创建"
  exit 1
fi
set -a
# shellcheck disable=SC1091
source .env
set +a

# ---------- 第一步：连接检查 ----------
# 退出码：0 = 连接正常；2 = 认证成功但库不存在（首次初始化属正常）；其余 = 失败
echo "[步骤 1/2] 连接检查（先测试链接，通过后才新建数据）..." | tee -a "$LOG_FILE"
# 管道配合 pipefail 会提前退出，故用 || true 抑制，真实退出码从 PIPESTATUS 读取
bash scripts/check-db.sh 2>&1 | tee -a "$LOG_FILE" || true
CHECK_CODE=${PIPESTATUS[0]}
if [ "$CHECK_CODE" -ne 0 ] && [ "$CHECK_CODE" -ne 2 ]; then
  echo "[错误] 连接检查未通过（退出码 $CHECK_CODE），终止初始化" | tee -a "$LOG_FILE"
  exit 1
fi
echo "[通过] 连接检查通过，继续初始化" | tee -a "$LOG_FILE"

# ---------- 第二步：初始化数据库 ----------
echo "[步骤 2/2] 初始化数据库（建库 → 建表 → 种子数据）..." | tee -a "$LOG_FILE"
go run ./cmd/dbinit 2>&1 | tee -a "$LOG_FILE" || true
INIT_CODE=${PIPESTATUS[0]}
if [ "$INIT_CODE" -ne 0 ]; then
  echo "[错误] 数据库初始化失败（退出码 $INIT_CODE），详见日志：$LOG_FILE"
  exit 1
fi

echo "[完成] 数据库初始化成功，日志：$LOG_FILE"

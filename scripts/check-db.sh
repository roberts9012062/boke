#!/usr/bin/env bash
# scripts/check-db.sh
# 检查 PostgreSQL 连接与 GitHub Token 有效性。
#
# 用法：
#   ./scripts/check-db.sh
#
# 说明：
#   1. 从项目根目录 .env 加载配置（不打印任何敏感值）
#   2. 运行 cmd/dbcheck 连接测试程序
#   3. 退出码：0 = 数据库与 GitHub 均正常；非 0 = 存在失败项
set -euo pipefail

# 定位项目根目录（脚本所在目录的上一级）
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 加载 .env 配置（仅导出环境变量，不回显内容）
if [ ! -f .env ]; then
  echo "[错误] 缺少 .env 配置文件，请先创建"
  exit 1
fi
set -a
# shellcheck disable=SC1091
source .env
set +a

# 运行连接检查程序
# 退出码语义：0 = 全部正常；2 = 认证成功但目标库不存在（首次初始化场景）；其他 = 失败
# 说明：set -e 下直接 go run 会吞掉退出码 2，因此先关闭 errexit 捕获真实退出码
set +e
go run ./cmd/dbcheck
CHECK_CODE=$?
set -e
exit "$CHECK_CODE"

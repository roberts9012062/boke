#!/usr/bin/env bash
# scripts/smoke-role.sh
# 运行 M5 权限体系后端冒烟（smoke_role.py）。
# 前置：双端已启动（dev-server.sh / dev-frontend.sh）；数据库可达。
#
# 用法：
#   ./scripts/smoke-role.sh
#
# 说明：
#   1. 输出写入 logs/smoke-role-*.log
#   2. 冒烟脚本会调整测试账号角色并还原（dm_test），需注意登录限流 5 次/分
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/smoke-role-$(date +%Y%m%d-%H%M%S).log"

echo "[进行] 权限体系冒烟（smoke_role.py）..." | tee "$LOG_FILE"
python "$PROJECT_ROOT/scripts/smoke_role.py" 2>&1 | tee -a "$LOG_FILE"
echo "[完成] 冒烟日志：$LOG_FILE"

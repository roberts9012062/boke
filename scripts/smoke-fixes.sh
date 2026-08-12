#!/usr/bin/env bash
# scripts/smoke-fixes.sh
# 运行修复批次回归冒烟（smoke_fixes.py）：注册/评论开关、注销账号、超管保护。
# 前置：双端已启动；数据库可达。
# 注意：登录限流 5 次/分，重跑间隔 1 分钟以上。
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/smoke-fixes-$(date +%Y%m%d-%H%M%S).log"

echo "[进行] 修复批次冒烟（smoke_fixes.py）..." | tee "$LOG_FILE"
python "$PROJECT_ROOT/scripts/smoke_fixes.py" 2>&1 | tee -a "$LOG_FILE"
echo "[完成] 冒烟日志：$LOG_FILE"

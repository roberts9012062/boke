#!/usr/bin/env bash
# scripts/update-agent.sh
# 站点更新代理（宿主机执行）：轮询数据目录的更新任务 → 拉取代码 → 重建镜像 →
# 重启服务 → 写回进度状态。由 systemd timer 每分钟调度（install-update-agent.sh 安装）。
#
# 通信协议（与 internal/update/task.go 对应，经 data/ 挂载卷共享）：
#   data/update-task.json    后端写入的更新请求（本脚本处理后删除）
#   data/update-status.json  本脚本写入的进度（state/stage/percent/version）
#   data/app-version.txt     部署完成后的当前版本（git tag）
#
# 用法：
#   ./scripts/update-agent.sh <项目根目录>          # 处理一次任务（systemd 调度形态）
#   ./scripts/update-agent.sh <项目根目录> --init   # 仅初始化版本文件（首次安装）
set -euo pipefail

ROOT="${1:?用法: update-agent.sh <项目根目录> [--init]}"
MODE="${2:-}"
DATA_DIR="$ROOT/data"
STATUS_FILE="$DATA_DIR/update-status.json"
VERSION_FILE="$DATA_DIR/app-version.txt"
LOG_DIR="$ROOT/logs"
LOG_FILE="$LOG_DIR/update-agent-$(date +%Y%m%d).log"

mkdir -p "$LOG_DIR" "$DATA_DIR"

# systemd 服务不继承登录 shell 的 HOME（git 找不到全局配置 → dubious ownership 拒绝操作）
export HOME="${HOME:-/root}"

# log 统一日志（终端 + 文件）
log() { echo "[$(date '+%F %T')] $*" | tee -a "$LOG_FILE"; }

# write_status 写进度状态文件（JSON 转义交给 python，避免 shell 引号问题）
write_status() {
  local state="$1" stage="$2" percent="$3" version="$4" message="${5:-}"
  python3 - "$STATUS_FILE" "$state" "$stage" "$percent" "$version" "$message" <<'PYEOF'
import json, sys, datetime
path, state, stage, percent, version, message = sys.argv[1:7]
with open(path, "w", encoding="utf-8") as f:
    json.dump({
        "state": state, "stage": stage, "percent": int(percent),
        "version": version, "message": message,
        "updated_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    }, f, ensure_ascii=False)
PYEOF
}

# current_version 读取仓库当前版本（最近 tag；无 tag 用 dev-<短SHA>）
current_version() {
  cd "$ROOT"
  local tag
  tag="$(git describe --tags --exact-match HEAD 2>/dev/null || true)"
  if [[ -n "$tag" ]]; then echo "$tag"; else echo "dev-$(git rev-parse --short HEAD)"; fi
}

# compose 兼容：优先 docker compose v2 插件
compose() {
  if docker compose version >/dev/null 2>&1; then docker compose "$@"; else docker-compose "$@"; fi
}

# ---------- 初始化模式：仅写版本文件后退出 ----------
if [[ "$MODE" == "--init" ]]; then
  current_version > "$VERSION_FILE"
  log "[初始化] 当前版本：$(cat "$VERSION_FILE")"
  exit 0
fi

# ---------- 读取任务（无任务静默退出，systemd 每分钟调度） ----------
TASK_FILE="$DATA_DIR/update-task.json"
if [[ ! -f "$TASK_FILE" ]]; then
  exit 0
fi
VERSION="$(python3 -c "import json; print(json.load(open('$TASK_FILE'))['version'])" 2>/dev/null || true)"
if [[ -z "$VERSION" ]]; then
  log "[错误] 任务文件损坏，删除重启" | tee -a "$LOG_FILE"
  rm -f "$TASK_FILE"
  exit 1
fi
log "[任务] 开始更新到 $VERSION"

# ---------- 阶段 1：拉取目标版本代码（5% → 10%）----------
write_status running "正在拉取 $VERSION 代码" 5 "$VERSION"
cd "$ROOT"
if ! git fetch --tags origin main >> "$LOG_FILE" 2>&1; then
  write_status failed "拉取远程仓库失败（检查服务器网络）" 0 "$VERSION"
  rm -f "$TASK_FILE"; exit 1
fi
# -f 强制切换：更新以仓库为准（丢弃本地未提交修改；data/logs 为挂载卷不受影响）
if ! git checkout -qf "$VERSION" >> "$LOG_FILE" 2>&1; then
  write_status failed "切换到版本 $VERSION 失败（tag 不存在）" 0 "$VERSION"
  rm -f "$TASK_FILE"; exit 1
fi
write_status running "代码已更新到 $VERSION" 10 "$VERSION"

# ---------- 阶段 2：重建后端镜像（15% → 45%）----------
write_status running "正在构建后端镜像" 15 "$VERSION"
if ! compose build backend >> "$LOG_FILE" 2>&1; then
  write_status failed "后端镜像构建失败（详见 logs/update-agent 日志）" 15 "$VERSION"
  rm -f "$TASK_FILE"; exit 1
fi
write_status running "后端镜像构建完成" 45 "$VERSION"

# ---------- 阶段 3：重建前端镜像（50% → 85%）----------
write_status running "正在构建前端镜像" 50 "$VERSION"
if ! compose build frontend >> "$LOG_FILE" 2>&1; then
  write_status failed "前端镜像构建失败（详见 logs/update-agent 日志）" 50 "$VERSION"
  rm -f "$TASK_FILE"; exit 1
fi
write_status running "前端镜像构建完成" 85 "$VERSION"

# ---------- 阶段 4：重启服务（90%）----------
write_status running "正在重启服务" 90 "$VERSION"
if ! compose up -d backend frontend >> "$LOG_FILE" 2>&1; then
  write_status failed "服务重启失败（详见 logs/update-agent 日志）" 90 "$VERSION"
  rm -f "$TASK_FILE"; exit 1
fi

# ---------- 阶段 5：数据库迁移（93%，幂等；v1.5.6 起容器自带嵌入迁移）----------
# dbmigrate 的迁移 SQL 已 go:embed 进二进制，容器内自包含；schema_migrations
# 记录保证幂等（已应用的自动跳过）。迁移失败视为更新失败，避免新代码跑在旧表结构上。
write_status running "正在执行数据库迁移" 93 "$VERSION"
if ! compose exec -T backend /app/dbmigrate >> "$LOG_FILE" 2>&1; then
  write_status failed "数据库迁移失败（详见 logs/update-agent 日志）" 93 "$VERSION"
  rm -f "$TASK_FILE"; exit 1
fi

# ---------- 阶段 6：完成（100%，写版本文件，清任务）----------
echo "$VERSION" > "$VERSION_FILE"
write_status done "已更新到 $VERSION" 100 "$VERSION"
rm -f "$TASK_FILE"
log "[完成] 站点已更新到 $VERSION"

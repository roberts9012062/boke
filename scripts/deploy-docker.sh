#!/usr/bin/env bash
# scripts/deploy-docker.sh
# Docker Compose 部署启停脚本：构建镜像、启停服务、查看日志。
#
# 用法：
#   ./scripts/deploy-docker.sh up      # 构建并启动全部服务（首次部署入口）
#   ./scripts/deploy-docker.sh down    # 停止并移除容器（数据卷保留）
#   ./scripts/deploy-docker.sh restart # 重启全部服务
#   ./scripts/deploy-docker.sh ps      # 查看服务状态
#   ./scripts/deploy-docker.sh logs [服务名]  # 跟踪日志（默认全部）
#   ./scripts/deploy-docker.sh rebuild  # 代码更新后重新构建并滚动启动
#
# 说明：
#   1. 首次启动后访问 http://<主机>:3000 进入安装向导（Docker 模式自动绑定数据库）
#   2. 重装：删除 data/install.lock 后 ./scripts/deploy-docker.sh restart backend
#   3. 日志统一输出到 logs/ 目录（bind mount）与 docker 日志双通道
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/deploy-docker-$(date +%Y%m%d-%H%M%S).log"

# log 统一输出（终端 + 日志文件）
log() {
  echo "$*" | tee -a "$LOG_FILE"
}

# compose 兼容：优先 docker compose（v2 插件），回退 docker-compose
compose() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
  else
    docker-compose "$@"
  fi
}

case "${1:-}" in
  up)
    log "[部署] 构建并启动全部服务..."
    compose up -d --build 2>&1 | tee -a "$LOG_FILE"
    log "[完成] 服务已启动，访问 http://<主机IP>:3000 完成安装向导"
    ;;
  down)
    log "[停止] 移除全部容器（数据卷保留）..."
    compose down 2>&1 | tee -a "$LOG_FILE"
    ;;
  restart)
    log "[重启] 重启全部服务..."
    compose restart 2>&1 | tee -a "$LOG_FILE"
    ;;
  rebuild)
    log "[重建] 重新构建镜像并启动..."
    compose up -d --build 2>&1 | tee -a "$LOG_FILE"
    log "[完成] 已按最新代码重建服务"
    ;;
  ps)
    compose ps
    ;;
  logs)
    shift || true
    compose logs -f "${1:-}" 2>&1 | tee -a "$LOG_FILE"
    ;;
  *)
    echo "用法：$0 {up|down|restart|rebuild|ps|logs [服务名]}"
    exit 1
    ;;
esac

#!/usr/bin/env bash
# scripts/deploy-tg-image-worker.sh
# 一键部署 TG图床反代 Worker（wrangler CLI，本机命令行登录 Cloudflare）。
#
# 用法：
#   TG_BOT_TOKEN="123456:AAxxx" ./scripts/deploy-tg-image-worker.sh
#
# 前置：
#   1. 已执行 npx wrangler login（首次会开浏览器授权）
#   2. Worker 源码 marketplace-repo/tg-image-bed/worker/（index.js + wrangler.example.toml）
#   3. 环境变量 TG_BOT_TOKEN（@BotFather 的 Token；绝不写入任何文件，仅注入 Cloudflare secret）
#
# 产物：
#   - Cloudflare Worker（https://tg-image-bed-worker.<账号>.workers.dev）
#   - secret TG_BOT_TOKEN（持 token 反代读图；博客插件设置只需填 Worker 地址 + Token + Chat ID）
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

WORKER_DIR="marketplace-repo/tg-image-bed/worker"

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/deploy-tg-image-worker-$(date +%Y%m%d-%H%M%S).log"

echo "[检查] wrangler 登录状态..." | tee -a "$LOG_FILE"
# 说明：输出先捕获再判定——grep -q 命中即提前退出会让 tee 收到 SIGPIPE，
# pipefail 下整管道误判失败（与 deploy-image-worker.sh 同源经验）
WHOAMI_OUT="$(npx -y wrangler whoami 2>&1 || true)"
echo "$WHOAMI_OUT" >> "$LOG_FILE"
if ! echo "$WHOAMI_OUT" | grep -q "logged in"; then
  echo "[错误] 未登录 Cloudflare：请先执行 npx wrangler login（浏览器授权）后重试" | tee -a "$LOG_FILE"
  exit 1
fi

echo "[准备] 生成 wrangler.toml（from example；不含任何 secret）..." | tee -a "$LOG_FILE"
cp "$WORKER_DIR/wrangler.example.toml" "$WORKER_DIR/wrangler.toml"

echo "[步骤1] 部署 Worker（首次创建 tg-image-bed-worker）..." | tee -a "$LOG_FILE"
npx -y wrangler deploy --config "$WORKER_DIR/wrangler.toml" 2>&1 | tee -a "$LOG_FILE"

echo "[步骤2] 注入 secret TG_BOT_TOKEN（来自环境变量，不落盘）..." | tee -a "$LOG_FILE"
if [[ -z "${TG_BOT_TOKEN:-}" ]]; then
  echo "[警告] 未提供 TG_BOT_TOKEN 环境变量——Worker 已部署但 /health 将报未配置" | tee -a "$LOG_FILE"
  echo "[提示] 稍后补注：TG_BOT_TOKEN=xxx ./scripts/deploy-tg-image-worker.sh" | tee -a "$LOG_FILE"
else
  printf '%s' "$TG_BOT_TOKEN" | npx -y wrangler secret put TG_BOT_TOKEN --config "$WORKER_DIR/wrangler.toml" 2>&1 | tee -a "$LOG_FILE"
fi

echo "[完成] TG图床反代 Worker 部署结束，日志：$LOG_FILE" | tee -a "$LOG_FILE"

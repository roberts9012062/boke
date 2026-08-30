#!/usr/bin/env bash
# scripts/deploy-image-worker.sh
# 一键部署 Cloudflare 图床 Worker（wrangler CLI，本机命令行登录 Cloudflare）。
#
# 用法：
#   ./scripts/deploy-image-worker.sh            # 全流程：建桶 → 密钥 → 部署 → 输出配对信息
#
# 前置：
#   1. 已执行 npx wrangler login（首次会开浏览器授权）
#   2. Worker 源码 marketplace-repo/image-cdn/worker/（index.js + wrangler.example.toml）
#
# 产物：
#   - Cloudflare Worker（https://<name>.<account>.workers.dev）
#   - R2 桶 yueyan-media
#   - API_KEY（同时写入 data/image-worker-credentials.txt 供 boke 插件配置回填）
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

WORKER_DIR="marketplace-repo/image-cdn/worker"
BUCKET_NAME="yueyan-media"
CRED_FILE="$PROJECT_ROOT/data/image-worker-credentials.txt"

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR" "$PROJECT_ROOT/data"
LOG_FILE="$LOG_DIR/deploy-image-worker-$(date +%Y%m%d-%H%M%S).log"

echo "[检查] wrangler 登录状态..." | tee -a "$LOG_FILE"
# 说明：输出先捕获再判定——grep -q 命中即提前退出会让 tee 收到 SIGPIPE，
# pipefail 下整管道误判失败（脚本判定"未登录"的根因）
WHOAMI_OUT="$(npx -y wrangler whoami 2>&1 || true)"
echo "$WHOAMI_OUT" >> "$LOG_FILE"
if ! echo "$WHOAMI_OUT" | grep -q "logged in"; then
  echo "[错误] 未登录 Cloudflare：请先执行 npx wrangler login（浏览器授权）后重试" | tee -a "$LOG_FILE"
  exit 1
fi

echo "[准备] 生成 wrangler.toml（R2 绑定 $BUCKET_NAME）..." | tee -a "$LOG_FILE"
sed "s/bucket = \"yueyan-media\"/bucket = \"$BUCKET_NAME\"/" \
  "$WORKER_DIR/wrangler.example.toml" > "$WORKER_DIR/wrangler.toml"

echo "[步骤1] 创建 R2 桶 $BUCKET_NAME（已存在则跳过）..." | tee -a "$LOG_FILE"
npx -y wrangler r2 bucket create "$BUCKET_NAME" 2>&1 | tee -a "$LOG_FILE" || \
  echo "[提示] 桶已存在或创建失败，继续部署..." | tee -a "$LOG_FILE"

echo "[步骤2] 生成并配置 API_KEY..." | tee -a "$LOG_FILE"
API_KEY="$(node -e "console.log(require('crypto').randomBytes(32).toString('hex'))")"
printf '%s' "$API_KEY" | npx -y wrangler secret put API_KEY --config "$WORKER_DIR/wrangler.toml" 2>&1 | tee -a "$LOG_FILE"

echo "[步骤3] 部署 Worker..." | tee -a "$LOG_FILE"
DEPLOY_OUT="$(cd "$WORKER_DIR" && npx -y wrangler deploy 2>&1 | tee -a "$LOG_FILE")"
WORKER_URL="$(echo "$DEPLOY_OUT" | grep -oE 'https://[a-z0-9-]+\.[a-z0-9-]+\.workers\.dev' | head -1)"

if [ -z "$WORKER_URL" ]; then
  echo "[错误] 未能从部署输出解析 Worker URL，请查看日志：$LOG_FILE" | tee -a "$LOG_FILE"
  exit 1
fi

echo "[步骤4] 验证配对（/health）..." | tee -a "$LOG_FILE"
sleep 3
if curl -sf "$WORKER_URL/health" -H "Authorization: Bearer $API_KEY" | grep -q '"ok":true'; then
  echo "[通过] Worker 配对验证成功" | tee -a "$LOG_FILE"
else
  echo "[警告] /health 未就绪（新部署传播可能延迟几十秒），稍后可手动重试" | tee -a "$LOG_FILE"
fi

cat > "$CRED_FILE" <<EOF
# Cloudflare 图床配对信息（boke 插件设置回填用）生成于 $(date '+%F %T')
workers_url = $WORKER_URL
api_key = $API_KEY
EOF

echo "[完成] Worker 已部署：$WORKER_URL" | tee -a "$LOG_FILE"
echo "[完成] 配对信息已写入：$CRED_FILE（回填 boke 插件设置即可启用图床）" | tee -a "$LOG_FILE"

#!/usr/bin/env bash
# scripts/push-image-worker-repo.sh
# 创建并同步 CF图床 Worker 源码仓库（roberts9012062/yueyan-image-worker）。
# boke 插件库存放博客插件本体；Cloudflare 侧源码与部署教程独立成仓——
# 商城「CF图床」详情页 README 指引用户到该仓库部署 Worker。
#
# 用法：
#   ./scripts/push-image-worker-repo.sh [owner/repo]
#
# 说明：
#   1. 使用 .env 的 GITHUB_TOKEN（repo 权限）调用 GitHub API（不回显 token）
#   2. 仓库不存在时自动创建（公开仓库，含描述与主题标签）
#   3. 幂等：目标文件已存在时携带 sha 更新
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

TARGET="${1:-roberts9012062/yueyan-image-worker}"
OWNER="${TARGET%/*}"
REPO="${TARGET#*/}"
API="https://api.github.com"
SRC_DIR="$PROJECT_ROOT/marketplace-repo/image-cdn/worker"

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/push-image-worker-$(date +%Y%m%d-%H%M%S).log"

# 读取 GITHUB_TOKEN（不打印敏感值）
TOKEN="${GITHUB_TOKEN:-}"
if [ -z "$TOKEN" ] && grep -q "^GITHUB_TOKEN=" .env 2>/dev/null; then
  TOKEN="$(grep "^GITHUB_TOKEN=" .env | head -1 | sed 's/^GITHUB_TOKEN=//; s/"//g; s/\r//')"
fi
if [ -z "$TOKEN" ]; then
  echo "[错误] 缺少 GITHUB_TOKEN（.env 或环境变量）" | tee -a "$LOG_FILE"
  exit 1
fi

gh_curl() {
  curl -sS -H "Authorization: Bearer $TOKEN" -H "Accept: application/vnd.github+json" "$@"
}

echo "[1/4] 确认/创建仓库 $TARGET ..." | tee -a "$LOG_FILE"
CODE="$(gh_curl -o logs/iw-repo.json -w '%{http_code}' "$API/repos/$OWNER/$REPO")"
if [ "$CODE" = "404" ]; then
  # JSON 体经文件传递（含中文 description 直接内联会被 Git Bash 编码损坏 → 400）
  node -e "require('fs').writeFileSync('logs/iw-create-req.json', JSON.stringify({name: process.argv[1], description: '月言 boke · CF图床插件的服务端（Cloudflare Workers + R2）：部署即得图床 API', private: false, has_issues: true}))" "$REPO"
  CREATE_CODE="$(gh_curl -o logs/iw-create.json -w '%{http_code}' -X POST "$API/user/repos" -d @logs/iw-create-req.json)"
  if [ "$CREATE_CODE" != "201" ]; then
    echo "[错误] 仓库创建失败（HTTP $CREATE_CODE）：$(head -c 300 logs/iw-create.json)" | tee -a "$LOG_FILE"
    exit 1
  fi
  echo "      已创建（公开）：$TARGET" | tee -a "$LOG_FILE"
elif [ "$CODE" = "200" ]; then
  echo "      仓库已存在" | tee -a "$LOG_FILE"
else
  echo "[错误] 仓库查询失败（HTTP $CODE）：$(head -c 200 logs/iw-repo.json)" | tee -a "$LOG_FILE"
  exit 1
fi

echo "[2/4] 上传源码与教程 ..." | tee -a "$LOG_FILE"
for FILE in README.md index.js wrangler.example.toml; do
  if [ ! -f "$SRC_DIR/$FILE" ]; then
    echo "[错误] 缺少源文件：$SRC_DIR/$FILE" | tee -a "$LOG_FILE"
    exit 1
  fi
  SHA="$(gh_curl "$API/repos/$OWNER/$REPO/contents/$FILE" | node -e "let d='';process.stdin.on('data',c=>d+=c).on('end',()=>{try{console.log(JSON.parse(d).sha||'')}catch(e){console.log('')}})")"
  # 请求体经文件传递（base64 内容大且含特殊字符，避免命令行长度与转义问题）
  node -e "const fs=require('fs');const sha=process.argv[2];const body={message:(sha?'update ':'add ')+process.argv[1],content:fs.readFileSync('marketplace-repo/image-cdn/worker/'+process.argv[1]).toString('base64')};if(sha){body.sha=sha}fs.writeFileSync('logs/iw-put-req.json',JSON.stringify(body))" "$FILE" "$SHA"
  PUT_CODE="$(gh_curl -o logs/iw-put.json -w '%{http_code}' -X PUT "$API/repos/$OWNER/$REPO/contents/$FILE" -d @logs/iw-put-req.json)"
  if [ "$PUT_CODE" != "200" ] && [ "$PUT_CODE" != "201" ]; then
    echo "[错误] $FILE 上传失败（HTTP $PUT_CODE）：$(head -c 300 logs/iw-put.json)" | tee -a "$LOG_FILE"
    exit 1
  fi
  echo "      已上传 $FILE" | tee -a "$LOG_FILE"
done

echo "[3/4] 设置仓库主题标签 ..." | tee -a "$LOG_FILE"
gh_curl -X PUT "$API/repos/$OWNER/$REPO/topics" -H "Accept: application/vnd.github.mercy-preview+json" \
  -d '{"names":["cloudflare-workers","r2","image-hosting","boke","chinese"]}' > /dev/null || true

echo "[4/4] 完成：https://github.com/$TARGET" | tee -a "$LOG_FILE"

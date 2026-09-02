#!/usr/bin/env bash
# scripts/push-marketplace-repo.sh
# 将 marketplace-repo/ 内容同步到插件源 GitHub 仓库（默认 roberts9012062/yueyan-plugins）。
#
# 用法：
#   ./scripts/push-marketplace-repo.sh [owner/repo]
#
# 说明：
#   1. 使用 .env 的 GITHUB_TOKEN（需 repo 权限）调用 GitHub Contents API（不回显 token）
#   2. 幂等：目标文件已存在时携带 sha 更新；同步删除旧版总清单 plugins.json
#   3. 本地目录结构约定：marketplace-repo/{插件ID}/plugin.json + README.md（见 docs/plugin-development.md 第 3 章）
#   4. 日志输出到 logs/ 目录
set -euo pipefail

# 定位项目根目录（脚本所在目录的上一级）
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

TARGET="${1:-roberts9012062/yueyan-plugins}"
OWNER="${TARGET%/*}"
REPO="${TARGET#*/}"
LOCAL_DIR="$PROJECT_ROOT/marketplace-repo"
API="https://api.github.com"

# 日志目录与日志文件
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/push-marketplace-$(date +%Y%m%d-%H%M%S).log"

# 读取 GITHUB_TOKEN（不打印任何敏感值）
if [ ! -f .env ]; then
  echo "[错误] 缺少 .env 配置文件" | tee -a "$LOG_FILE"
  exit 1
fi
set -a
# shellcheck disable=SC1091
source .env
set +a

if [ -z "${GITHUB_TOKEN:-}" ]; then
  echo "[错误] .env 未配置 GITHUB_TOKEN" | tee -a "$LOG_FILE"
  exit 1
fi
if [ ! -d "$LOCAL_DIR" ]; then
  echo "[错误] 本地目录 $LOCAL_DIR 不存在" | tee -a "$LOG_FILE"
  exit 1
fi
if [ "$OWNER" = "$TARGET" ] || [ -z "$REPO" ]; then
  echo "[错误] 目标仓库格式应为 owner/repo" | tee -a "$LOG_FILE"
  exit 1
fi

# gh_api GitHub API 请求封装。
# 参数：$1 方法（GET/PUT/DELETE）；$2 路径（如 /repos/o/r/contents/x）；$3 可选 JSON body。
gh_api() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local args=(-sS -X "$method" "$API$path" -H "Authorization: Bearer $GITHUB_TOKEN" -H "Accept: application/vnd.github+json")
  if [ -n "$body" ]; then
    # Windows 命令行长度有限（~32KB，源码/go.sum 的 base64 JSON 会超限）——
    # body 落临时文件经 --data-binary 传递，规避 Argument list too long
    local tmp
    tmp=$(mktemp)
    printf '%s' "$body" > "$tmp"
    local out
    out=$(curl "${args[@]}" -H "Content-Type: application/json" --data-binary "@$tmp")
    local rc=$?
    rm -f "$tmp"
    printf '%s' "$out"
    return $rc
  fi
  curl "${args[@]}"
}

echo "[1/5] 校验 token 与目标仓库..." | tee -a "$LOG_FILE"
USER_RESP="$(gh_api GET "/user")"
if ! echo "$USER_RESP" | python -c "import sys,json; json.load(sys.stdin)['login']" >/dev/null 2>&1; then
  echo "[错误] GITHUB_TOKEN 无效或无权限" | tee -a "$LOG_FILE"
  exit 1
fi
# 注意：Windows 下 python 输出带 \r，统一 tr 剥离（避免污染 sha/URL/JSON）
BRANCH="$(gh_api GET "/repos/$OWNER/$REPO" | python -c "import sys,json; print(json.load(sys.stdin).get('default_branch','main'))" 2>/dev/null | tr -d '\r')"
BRANCH="${BRANCH:-main}"
echo "      目标：$OWNER/$REPO@$BRANCH" | tee -a "$LOG_FILE"

# 远程文件 sha 索引（path → sha；上传时幂等更新、删除旧清单用）
echo "[2/5] 拉取远程文件索引..." | tee -a "$LOG_FILE"
declare -A REMOTE_SHA
while IFS=$'\t' read -r p sha; do
  p="${p%$'\r'}"
  sha="${sha%$'\r'}"
  if [ -n "$p" ]; then
    REMOTE_SHA["$p"]="$sha"
  fi
done < <(gh_api GET "/repos/$OWNER/$REPO/git/trees/$BRANCH?recursive=1" | python -c "
import sys, json
try:
    data = json.load(sys.stdin)
    for item in data.get('tree', []):
        if item.get('type') == 'blob':
            print(item['path'] + '\t' + item.get('sha', ''))
except Exception:
    pass
")

# 清理旧版总清单 plugins.json（文件夹结构不再使用）
# 注意：提交信息用 ASCII——Windows 下中文经 shell 传给 curl 会损坏 JSON 编码
echo "[3/5] 清理旧版总清单..." | tee -a "$LOG_FILE"
if [ -n "${REMOTE_SHA[plugins.json]:-}" ]; then
  DEL_BODY="{\"message\":\"feat: remove legacy plugins.json (folder-based marketplace)\",\"sha\":\"${REMOTE_SHA[plugins.json]}\",\"branch\":\"$BRANCH\"}"
  DEL_RESP="$(gh_api DELETE "/repos/$OWNER/$REPO/contents/plugins.json" "$DEL_BODY")"
  if echo "$DEL_RESP" | python -c "import sys,json; json.load(sys.stdin)['commit']" >/dev/null 2>&1; then
    echo "      已删除 plugins.json" | tee -a "$LOG_FILE"
  else
    echo "      [警告] 删除 plugins.json 失败：$(echo "$DEL_RESP" | head -c 300)" | tee -a "$LOG_FILE"
    exit 1
  fi
else
  echo "      无旧清单，跳过" | tee -a "$LOG_FILE"
fi

# 上传本地全部文件（幂等：远程已存在则携带 sha 更新）
# 409 重试：经代理或 GitHub API 主从延迟时，第 2 步索引里的 sha 可能已陈旧——
# 重新 GET 该文件当前 sha 后重 PUT 一次，仍失败才报错退出
echo "[4/5] 上传本地文件..." | tee -a "$LOG_FILE"
COUNT=0
while IFS= read -r file; do
  rel="${file#"$LOCAL_DIR"/}"
  content_b64="$(base64 -w0 "$file")"
  sha_json=""
  if [ -n "${REMOTE_SHA[$rel]:-}" ]; then
    sha_json=",\"sha\":\"${REMOTE_SHA[$rel]}\""
  fi
  body="{\"message\":\"feat: update plugin marketplace $rel\",\"content\":\"$content_b64\"$sha_json,\"branch\":\"$BRANCH\"}"
  resp="$(gh_api PUT "/repos/$OWNER/$REPO/contents/$rel" "$body")"
  if echo "$resp" | grep -q '"status": *"409"'; then
    cur_sha="$(gh_api GET "/repos/$OWNER/$REPO/contents/$rel" | python -c "import sys,json; print(json.load(sys.stdin).get('sha',''))" 2>/dev/null | tr -d '\r')"
    if [ -n "$cur_sha" ]; then
      echo "      [重试] $rel 索引 sha 陈旧，按当前 sha 重传" | tee -a "$LOG_FILE"
      body="{\"message\":\"feat: update plugin marketplace $rel\",\"content\":\"$content_b64\",\"sha\":\"$cur_sha\",\"branch\":\"$BRANCH\"}"
      resp="$(gh_api PUT "/repos/$OWNER/$REPO/contents/$rel" "$body")"
    fi
  fi
  if echo "$resp" | python -c "import sys,json; json.load(sys.stdin)['content']" >/dev/null 2>&1; then
    COUNT=$((COUNT + 1))
  else
    echo "      [错误] 上传失败 $rel：$(echo "$resp" | head -c 300)" | tee -a "$LOG_FILE"
    exit 1
  fi
done < <(find "$LOCAL_DIR" -type f | sort)
echo "      已上传 $COUNT 个文件" | tee -a "$LOG_FILE"

# 校验最终仓库结构（插件文件夹统计）
echo "[5/5] 校验仓库结构..." | tee -a "$LOG_FILE"
gh_api GET "/repos/$OWNER/$REPO/git/trees/$BRANCH?recursive=1" | python -c "
import sys, json
data = json.load(sys.stdin)
paths = [i['path'] for i in data.get('tree', [])]
plugins = sorted({p.split('/')[0] for p in paths if p.endswith('/plugin.json')})
print('      插件文件夹：' + ', '.join(plugins))
print('      插件数：' + str(len(plugins)))
"
echo "[完成] 同步完成，日志：$LOG_FILE" | tee -a "$LOG_FILE"

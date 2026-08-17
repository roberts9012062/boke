#!/usr/bin/env bash
# scripts/publish-plugin-release.sh
# 发布插件 .bpk 到 GitHub Release（正式分发通道）：
#   {id}-{version}-{os}-{arch}.bpk → boke 仓库 Release v{version} 资产。
#
# 用法：
#   ./scripts/publish-plugin-release.sh dist/qq-music-0.1.0-windows-amd64.bpk
#
# 说明：
#   1. 使用 .env 的 GITHUB_TOKEN（需 repo 权限）；幂等：Release 已存在则替换同名资产
#   2. tag 自动创建（v{version}，指向默认分支 HEAD）
#   3. 市场清单 assets.sha256 与本资产对应——先 pack-plugin.sh 出包再发布
#   4. 查询/创建/清理经 python 解析响应（避免 shell 嵌套引号问题）
set -euo pipefail

BPK_PATH="${1:?用法: ./scripts/publish-plugin-release.sh <xxx.bpk>}"
[[ -f "$BPK_PATH" ]] || { echo "[错误] 文件不存在：$BPK_PATH" >&2; exit 1; }

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

set -a
# shellcheck disable=SC1091
source .env
set +a
: "${GITHUB_TOKEN:?缺少 GITHUB_TOKEN}"
export GITHUB_TOKEN
# Release 发布到插件库仓库（源码/清单/资产统一在 yueyan-plugins；主仓库不再承载插件分发）
REPO="${PLUGIN_RELEASE_REPO:-roberts9012062/yueyan-plugins}"
BASENAME="$(basename "$BPK_PATH")"
VERSION="$(echo "$BASENAME" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)"
TAG="v$VERSION"
[[ -n "$VERSION" ]] || { echo "[错误] 文件名无法解析版本：$BASENAME" >&2; exit 1; }

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/publish-release-$(date +%Y%m%d-%H%M%S).log"
echo "[发布] $BASENAME → $REPO Release $TAG" | tee -a "$LOG_FILE"

# 步骤 1-3：查找/创建 Release + 清理旧资产 + 上传（统一经 python urllib，
# 避免 Windows 原生 curl 不识别 MSYS 路径的问题）；输出下载直链
URL="$(python scripts/publish-release-helper.py "$REPO" "$TAG" "$BASENAME" "$BPK_PATH" 2>>"$LOG_FILE")"
if [[ -n "$URL" ]]; then
  echo "[完成] 资产已上传：$URL" | tee -a "$LOG_FILE"
else
  echo "[失败] 上传失败（详见日志 $LOG_FILE）" | tee -a "$LOG_FILE"
  exit 1
fi

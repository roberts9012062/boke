#!/usr/bin/env bash
# scripts/build-browser-extension.sh
# 插件生产构建：
#   ① Vite 构建 → dist/browser-extension/（浏览器「加载已解压」目录）
#   ② 打包 dist/browser-extension.zip（提交 Chrome Web Store / Edge Add-ons）
#   ③ 校验 manifest 与四尺寸图标齐全
# 用法：
#   ./scripts/build-browser-extension.sh
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EXT_DIR="$PROJECT_ROOT/browser-extension"
OUT_DIR="$PROJECT_ROOT/dist/browser-extension"
ZIP_FILE="$PROJECT_ROOT/dist/browser-extension.zip"
VITE_BIN="$EXT_DIR/node_modules/.bin/vite"

if [[ ! -x "$VITE_BIN" ]]; then
  echo "[错误] 未找到 vite，请先执行 ./scripts/setup-browser-extension.sh" >&2
  exit 1
fi

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/browser-extension-build.log"

# ---------- 构建 ----------
echo "[检查] TypeScript 严格类型" | tee -a "$LOG_FILE"
(cd "$EXT_DIR" && "$EXT_DIR/node_modules/.bin/tsc" --noEmit 2>&1 | tee -a "$LOG_FILE")

echo "[构建] vite build → dist/browser-extension/" | tee -a "$LOG_FILE"
(cd "$EXT_DIR" && "$VITE_BIN" build 2>&1 | tee -a "$LOG_FILE")

# ---------- 完整性校验 ----------
echo "[校验] manifest 与图标完整性" | tee -a "$LOG_FILE"
for f in manifest.json icons/icon-16.png icons/icon-32.png icons/icon-48.png icons/icon-128.png background.js sidepanel.js content-ball.js content-dock.js src/sidepanel/index.html; do
  if [[ ! -f "$OUT_DIR/$f" ]]; then
    echo "[失败] 产物缺失：$f" | tee -a "$LOG_FILE" >&2
    exit 1
  fi
done
# 用环境变量向 node 传路径（命令行内嵌展开会被 Git Bash 路径转换破坏）
MANIFEST_PATH="$(cygpath -m "$OUT_DIR/manifest.json")"
VERSION="$(MANIFEST_FILE="$MANIFEST_PATH" node -e "console.log(JSON.parse(require('fs').readFileSync(process.env.MANIFEST_FILE,'utf8')).version)")"
echo "[校验] 通过，manifest 版本 $VERSION" | tee -a "$LOG_FILE"

# ---------- 打包 zip ----------
if command -v powershell.exe >/dev/null 2>&1; then
  WIN_OUT="$(cygpath -w "$OUT_DIR")"
  WIN_ZIP="$(cygpath -w "$ZIP_FILE")"
  rm -f "$ZIP_FILE"
  powershell.exe -NoProfile -Command "Compress-Archive -Path '$WIN_OUT/*' -DestinationPath '$WIN_ZIP' -Force" >> "$LOG_FILE" 2>&1 \
    && echo "[打包] dist/browser-extension.zip 完成" | tee -a "$LOG_FILE" \
    || echo "[警告] zip 打包失败（产物目录仍可用）" | tee -a "$LOG_FILE"
else
  echo "[警告] 未找到 powershell，跳过 zip 打包" | tee -a "$LOG_FILE"
fi

echo "[完成] 加载方式见 docs/browser-extension-guide.md §11.2" | tee -a "$LOG_FILE"

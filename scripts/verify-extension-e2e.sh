#!/usr/bin/env bash
# scripts/verify-extension-e2e.sh
# 右键任务「真实发布」E2E（mock 站点承载开放网关契约，插件侧全链路真实执行）：
#   连接站点 → 球旁执行框总结发布（SSE 流式）→ 说说草稿篮文字+图片组合发布。
# 依赖：frontend/ 已 npm install（复用其 playwright）；系统安装 Google Chrome。
set -euo pipefail

BOKE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [ ! -f "$BOKE_ROOT/dist/browser-extension/manifest.json" ]; then
  echo "[缺失] dist/browser-extension 不存在，先执行构建" >&2
  exit 1
fi

cd "$BOKE_ROOT"
node scripts/verify-extension-e2e.mjs 2>&1 | tee -a "$BOKE_ROOT/logs/verify-extension-e2e.log"

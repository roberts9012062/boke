#!/usr/bin/env bash
# scripts/verify-extension-edge.sh
# Edge 兜底链路程序化验证（v0.31.0 右键任务 → 球隐藏 → 页内停靠 + 模态执行卡）。
# 依赖：frontend/ 已 npm install（复用其 playwright）；系统安装 Microsoft Edge。
set -euo pipefail

BOKE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# 前置：插件产物必须存在（否则先跑 build-browser-extension.sh）
if [ ! -f "$BOKE_ROOT/dist/browser-extension/manifest.json" ]; then
  echo "[缺失] dist/browser-extension 不存在，先执行构建" >&2
  exit 1
fi

# 复用 frontend 的 playwright 依赖执行验证脚本，日志落 logs/
cd "$BOKE_ROOT"
node scripts/verify-extension-edge.mjs 2>&1 | tee -a "$BOKE_ROOT/logs/verify-extension-edge.log"

#!/usr/bin/env bash
# scripts/build-seo-plugin.sh
# 编译 SEO 优化插件（cmd/seo-plugin）到 data/plugins/seo-optimizer/plugin(.exe)。
#
# 用法：
#   ./scripts/build-seo-plugin.sh
#
# 说明：
#   - 产物为当前平台二进制（Windows .exe / Linux/macOS 无后缀），供进程外插件验证
#   - data/ 不跟踪，产物不入库（Release 资产分发走 M3.4 .bpk 通道）
#   - 替换二进制后需重启插件进程（后台「我的插件」禁用再启用，或重启后端）
set -euo pipefail

# 定位项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 输出目录与文件名（与 binstore.go 约定一致：data/plugins/{id}/plugin[.exe]）
OUT_DIR="$PROJECT_ROOT/data/plugins/seo-optimizer"
BIN_NAME="plugin"
UNAME_S="$(uname -s)"
if [[ "$UNAME_S" == MINGW* || "$UNAME_S" == MSYS* || "$UNAME_S" == CYGWIN* ]]; then
  BIN_NAME="plugin.exe"
fi

# 日志
LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/build-seo-plugin-$(date +%Y%m%d-%H%M%S).log"

echo "[构建] cmd/seo-plugin → $OUT_DIR/$BIN_NAME" | tee -a "$LOG_FILE"
mkdir -p "$OUT_DIR"
go build -o "$OUT_DIR/$BIN_NAME" -C marketplace-repo ./seo-optimizer 2>&1 | tee -a "$LOG_FILE"
echo "[完成] SEO 插件二进制就绪：$OUT_DIR/$BIN_NAME" | tee -a "$LOG_FILE"

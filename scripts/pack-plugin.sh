#!/usr/bin/env bash
# scripts/pack-plugin.sh
# 统一签名打包脚本（P1 包签名体系）：编译插件二进制 → cmd/bp 打包（市场根私钥签名）。
#
# 用法：
#   ./scripts/pack-plugin.sh <插件目录名> [-version x.y.z] [-goos linux] [-goarch arm64]
#   例：./scripts/pack-plugin.sh qq-music-plugin
#       ./scripts/pack-plugin.sh demo-plugin -version 0.2.1
#       ./scripts/pack-plugin.sh bilibili-video -goos linux -goarch arm64
#
# 说明：
#   1. 插件源码位于 marketplace-repo/<插件 ID>/（yueyan-plugin.json 同目录；插件库独立 go module）
#   2. 自动探测：frontend/ 前端资产、demo 插件许可证公钥、市场根私钥（存在即签名）
#   3. 版本缺省取 yueyan-plugin.json 的 version
#   4. -goos/-goarch 缺省取本机平台；显式指定时交叉编译（CGO_ENABLED=0 纯静态）
#      —— 服务器为 ARM64 Linux 时，本机（windows/amd64）必须显式指定才能产出可安装资产
#   5. 产物：dist/{id}-{version}-{os}-{arch}.bpk（上传 GitHub Release 供市场安装）
set -euo pipefail

PLUGIN_DIR_NAME="${1:?用法: ./scripts/pack-plugin.sh <插件目录名> [-version x.y.z] [-goos linux] [-goarch arm64]}"
shift || true
VERSION_ARG=""
TARGET_GOOS=""
TARGET_GOARCH=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -version)
      [[ -n "${2:-}" ]] || { echo "[错误] -version 缺少参数" >&2; exit 1; }
      VERSION_ARG="$2"; shift 2 ;;
    -goos)
      [[ -n "${2:-}" ]] || { echo "[错误] -goos 缺少参数" >&2; exit 1; }
      TARGET_GOOS="$2"; shift 2 ;;
    -goarch)
      [[ -n "${2:-}" ]] || { echo "[错误] -goarch 缺少参数" >&2; exit 1; }
      TARGET_GOARCH="$2"; shift 2 ;;
    *)
      echo "[错误] 未知参数：$1" >&2; exit 1 ;;
  esac
done

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

SRC_DIR="marketplace-repo/$PLUGIN_DIR_NAME"
if [[ ! -f "$SRC_DIR/yueyan-plugin.json" ]]; then
  echo "[错误] 未找到插件清单：$SRC_DIR/yueyan-plugin.json" >&2
  exit 1
fi

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/pack-$PLUGIN_DIR_NAME-$(date +%Y%m%d-%H%M%S).log"

# 目标平台：显式指定优先，缺省本机（交叉编译需 CGO_ENABLED=0 保证纯静态可移植）
GOOS="${TARGET_GOOS:-$(go env GOOS)}"
GOARCH="${TARGET_GOARCH:-$(go env GOARCH)}"
CROSS_ARGS=()
if [[ -n "$TARGET_GOOS" || -n "$TARGET_GOARCH" ]]; then
  CROSS_ARGS=(env "CGO_ENABLED=0" "GOOS=$GOOS" "GOARCH=$GOARCH")
  echo "[交叉编译] 目标平台 $GOOS/$GOARCH（本机 $(go env GOOS)/$(go env GOARCH)）" | tee -a "$LOG_FILE"
fi
BUILD_DIR="$PROJECT_ROOT/dist/$PLUGIN_DIR_NAME"
mkdir -p "$BUILD_DIR"

echo "[步骤 1/3] 编译插件二进制（$GOOS/$GOARCH）..." | tee -a "$LOG_FILE"
if [[ ${#CROSS_ARGS[@]} -gt 0 ]]; then
  "${CROSS_ARGS[@]}" go build -C marketplace-repo -ldflags "-s -w" -trimpath -o "$BUILD_DIR/plugin.bin" "./$PLUGIN_DIR_NAME" 2>&1 | tee -a "$LOG_FILE"
else
  go build -C marketplace-repo -ldflags "-s -w" -trimpath -o "$BUILD_DIR/plugin.bin" "./$PLUGIN_DIR_NAME" 2>&1 | tee -a "$LOG_FILE"
fi

# 打包参数组装（探测式：公钥/前端/签名密钥存在才附加）
PACK_ARGS=(-plugin "$SRC_DIR/yueyan-plugin.json" -bin "$BUILD_DIR/plugin.bin" -os "$GOOS" -arch "$GOARCH" -out dist)
if [[ -d "$SRC_DIR/frontend" ]]; then
  PACK_ARGS+=(-frontend "$SRC_DIR/frontend")
fi
if [[ "$PLUGIN_DIR_NAME" == "demo-plugin" && -f "data/demo-keys/public.pem" ]]; then
  PACK_ARGS+=(-pubkey "data/demo-keys/public.pem")
fi
SIGNED=""
if [[ -f "data/keys/market-private.pem" ]]; then
  PACK_ARGS+=(-key "data/keys/market-private.pem")
  SIGNED=" + 市场签名"
else
  echo "[警告] 未找到市场根私钥 data/keys/market-private.pem，产出未签名包（配置信任公钥的主站将拒绝安装）" | tee -a "$LOG_FILE"
fi
if [[ -n "$VERSION_ARG" ]]; then
  PACK_ARGS+=(-version "$VERSION_ARG")
fi

echo "[步骤 2/3] 打包 .bpk（capabilities 随清单写入$SIGNED）..." | tee -a "$LOG_FILE"
go run ./cmd/bp pack "${PACK_ARGS[@]}" 2>&1 | tee -a "$LOG_FILE"

echo "[步骤 3/3] 输出 SHA-256（填写市场清单 assets.sha256 用）..." | tee -a "$LOG_FILE"
# 按目标平台过滤最新产物（避免误取 dist 中其他平台的旧包）
BPK_FILE="$(ls -t dist/*-"$GOOS"-"$GOARCH".bpk 2>/dev/null | head -1)"
[[ -n "$BPK_FILE" ]] || { echo "[错误] 未找到 $GOOS-$GOARCH 产物，打包可能失败" >&2; exit 1; }
sha256sum "$BPK_FILE" | tee -a "$LOG_FILE"

echo "[完成] 安装包就绪：$PROJECT_ROOT/$BPK_FILE，日志：$LOG_FILE" | tee -a "$LOG_FILE"

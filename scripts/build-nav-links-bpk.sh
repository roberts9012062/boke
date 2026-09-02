#!/usr/bin/env bash
# scripts/build-nav-links-bpk.sh
# 构建精品导航插件双平台 .bpk 安装包（交叉编译 → cmd/bp 打包，含前端扩展，
# 经市场根私钥 data/keys/market-private.pem 签名——站点配置信任公钥后仅接受签名包）。
#
# 用法：
#   ./scripts/build-nav-links-bpk.sh
#
# 产物：dist/nav-links-{version}-{os}-{arch}.bpk（windows-amd64 + linux-arm64，
#       与历版分发平台一致）；构建后输出各包 SHA-256，供 plugin.json 的
#       assets.sha256 / sha256_by_platform 登记（发布前务必同步更新清单）。
# 版本：取 marketplace-repo/nav-links/yueyan-plugin.json（与 plugin.json、
#       main.go Info() 三处保持一致，发版前先统一递增）。
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

LOG_DIR="$PROJECT_ROOT/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/build-nav-links-bpk-$(date +%Y%m%d-%H%M%S).log"

# 版本号读自市场清单（Windows 下 python 输出带 \r，统一剥离）
VERSION="$(python -c "import json; print(json.load(open('marketplace-repo/nav-links/yueyan-plugin.json', encoding='utf-8'))['version'])" | tr -d '\r')"
[[ -n "$VERSION" ]] || { echo "[错误] 未能解析插件版本号" | tee -a "$LOG_FILE"; exit 1; }
echo "[开始] 精品导航 v$VERSION 双平台构建..." | tee -a "$LOG_FILE"

BUILD_DIR="$PROJECT_ROOT/dist/nav-links"
mkdir -p "$BUILD_DIR"

# 平台清单（空格分隔 GOOS/GOARCH；历版分发仅此两平台）
PLATFORMS=("windows amd64" "linux arm64")

for p in "${PLATFORMS[@]}"; do
  read -r GOOS GOARCH <<< "$p"
  echo "[编译] $GOOS-$GOARCH..." | tee -a "$LOG_FILE"
  GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -C marketplace-repo -ldflags "-s -w" -trimpath \
    -o "$BUILD_DIR/plugin.bin-$GOOS-$GOARCH" ./nav-links 2>&1 | tee -a "$LOG_FILE"

  echo "[打包] nav-links-$VERSION-$GOOS-$GOARCH.bpk..." | tee -a "$LOG_FILE"
  go run ./cmd/bp pack \
    -plugin "marketplace-repo/nav-links/yueyan-plugin.json" \
    -bin "$BUILD_DIR/plugin.bin-$GOOS-$GOARCH" \
    -frontend "marketplace-repo/nav-links/frontend" \
    -key "data/keys/market-private.pem" \
    -os "$GOOS" -arch "$GOARCH" \
    -version "$VERSION" \
    -out "dist" 2>&1 | tee -a "$LOG_FILE"
done

# 输出产物哈希（供 plugin.json 登记：sha256 惯例取 windows-amd64 值，by_platform 各自登记）
echo "" | tee -a "$LOG_FILE"
echo "[哈希] 供 plugin.json 的 assets 登记：" | tee -a "$LOG_FILE"
for f in "dist/nav-links-$VERSION-windows-amd64.bpk" "dist/nav-links-$VERSION-linux-arm64.bpk"; do
  if [[ ! -f "$f" ]]; then
    echo "[错误] 产物缺失：$f" | tee -a "$LOG_FILE"
    exit 1
  fi
  echo "$f  $(sha256sum "$f" | cut -d' ' -f1)" | tee -a "$LOG_FILE"
done
echo "[完成] 日志：$LOG_FILE" | tee -a "$LOG_FILE"

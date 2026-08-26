#!/usr/bin/env bash
# scripts/install-update-agent.sh
# 安装站点更新代理的 systemd 单元（服务 + 每分钟定时器）到宿主机。
#
# 用法（在宿主机项目根目录执行）：
#   ./scripts/install-update-agent.sh
#
# 说明：
#   1. 安装 boke-update-agent.service / .timer（每分钟调度 scripts/update-agent.sh）
#   2. 首次安装自动执行 --init 初始化 data/app-version.txt（当前 git 版本）
#   3. 卸载：systemctl disable --now boke-update-agent.timer &&
#            rm /etc/systemd/system/boke-update-agent.{service,timer}
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AGENT="$ROOT/scripts/update-agent.sh"

[[ -f "$AGENT" ]] || { echo "[错误] 未找到 $AGENT" >&2; exit 1; }
[[ "$(id -u)" == "0" ]] || { echo "[错误] 需要 root 权限安装 systemd 单元" >&2; exit 1; }
chmod +x "$AGENT"

# systemd 单元（WorkingDirectory 固定项目根；进度写 data/ 共享卷）
cat > /etc/systemd/system/boke-update-agent.service <<EOF
[Unit]
Description=boke site update agent (one-shot task processor)
After=docker.service network-online.target

[Service]
Type=oneshot
WorkingDirectory=$ROOT
ExecStart=$AGENT $ROOT
EOF

cat > /etc/systemd/system/boke-update-agent.timer <<EOF
[Unit]
Description=Run boke update agent every minute

[Timer]
OnBootSec=1min
OnUnitActiveSec=1min
AccuracySec=10s
Unit=boke-update-agent.service

[Install]
WantedBy=timers.target
EOF

systemctl daemon-reload
systemctl enable --now boke-update-agent.timer

# 初始化当前版本文件（幂等：--init 只写 app-version.txt）
"$AGENT" "$ROOT" --init

echo "[完成] 更新代理已安装并启动（每分钟检查更新任务）"
echo "       当前版本：$(cat "$ROOT/data/app-version.txt")"
systemctl status boke-update-agent.timer --no-pager | head -4

#!/usr/bin/env bash
# 拉取代码 -> 编译 -> 重启 wecom-http（仅后端）
#
# 若同时要更新网页 + Nginx，请用一键脚本：
#   bash ~/my-backend-services/deploy/update-all.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
UNIT="wecom-http"

cd "$ROOT"

if git rev-parse --git-dir >/dev/null 2>&1; then
  echo "==> git pull"
  git pull --ff-only
else
  echo "警告: 当前目录不是 git 仓库，跳过 git pull"
fi

echo "==> 编译"
bash "$ROOT/deploy/build.sh"

echo "==> 重启 systemd 服务 $UNIT"
sudo systemctl restart "$UNIT"

echo "==> 状态"
sudo systemctl status "$UNIT" --no-pager || true

echo ""
echo "后端更新完成。测试: curl -s http://127.0.0.1:8081/ping"
echo "提示: 网页/Nginx 请用 deploy/update-all.sh 一键更新"
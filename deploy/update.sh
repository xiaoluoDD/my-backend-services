#!/usr/bin/env bash
# 拉取代码 -> 编译 -> 重启 wecom-http（日常更新用）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
UNIT="wecom-http"

cd "$ROOT"

if git rev-parse --git-dir >/dev/null 2>&1; then
  echo "==> git pull"
  git pull
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
echo "更新完成。测试: curl -s http://127.0.0.1:8081/ping"

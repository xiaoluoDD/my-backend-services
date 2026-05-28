#!/usr/bin/env bash
# 安装/更新 systemd 单元 wecom-http.service（需要 sudo）
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
USER_NAME="${SUDO_USER:-$(whoami)}"
UNIT_NAME="wecom-http.service"
TEMPLATE="$ROOT/deploy/wecom-http.service"
TARGET="/etc/systemd/system/$UNIT_NAME"

if [[ ! -f "$ROOT/wecom/.env" ]]; then
  echo "错误: 未找到 $ROOT/wecom/.env"
  echo "请先: cp wecom/.env.example wecom/.env 并填写企业微信配置"
  exit 1
fi

if [[ ! -f "$ROOT/bin/wecom-http" ]]; then
  echo "未找到二进制，先执行编译 ..."
  bash "$ROOT/deploy/build.sh"
fi

echo "==> 生成 systemd 单元 -> $TARGET"
sed -e "s|@REPO_ROOT@|$ROOT|g" -e "s|@USER@|$USER_NAME|g" "$TEMPLATE" | sudo tee "$TARGET" > /dev/null

echo "==> 重载 systemd 并启用服务"
sudo systemctl daemon-reload
sudo systemctl enable "$UNIT_NAME"
sudo systemctl restart "$UNIT_NAME"

echo ""
echo "==> 服务状态:"
sudo systemctl status "$UNIT_NAME" --no-pager || true

echo ""
echo "安装完成。查看日志: journalctl -u wecom-http -f"
echo "更新代码后: bash deploy/update.sh"

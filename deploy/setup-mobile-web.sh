#!/usr/bin/env bash
# 在 Ubuntu 云服务器上配置 Nginx，托管 ProjectShow/web 静态页。
# 用法：
#   bash deploy/setup-mobile-web.sh /home/ubuntu/project-show-web
#
# 前提：
#   1. 已把 Windows 上 ProjectShow/web 目录上传到 WEB_ROOT
#   2. wecom-http 已在 8081 运行（systemctl status wecom-http）
#   3. 已安装 nginx（sudo apt install -y nginx）

set -euo pipefail

WEB_ROOT="${1:-}"
if [[ -z "$WEB_ROOT" ]]; then
  echo "用法: bash deploy/setup-mobile-web.sh /path/to/web"
  echo "示例: bash deploy/setup-mobile-web.sh /home/ubuntu/project-show-web"
  exit 1
fi

if [[ ! -f "$WEB_ROOT/index.html" ]]; then
  echo "错误: 未找到 $WEB_ROOT/index.html"
  echo "请先将 ProjectShow/web 目录上传到 $WEB_ROOT"
  exit 1
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SITE="/etc/nginx/sites-available/project-mobile"

if ! command -v nginx >/dev/null 2>&1; then
  echo "未安装 nginx，正在安装 ..."
  sudo apt update
  sudo apt install -y nginx
fi

echo "==> 写入 Nginx 配置 $SITE"
sed "s|@WEB_ROOT@|$WEB_ROOT|g" "$ROOT/deploy/nginx-mobile.conf" | sudo tee "$SITE" > /dev/null

sudo ln -sf "$SITE" /etc/nginx/sites-enabled/project-mobile

# 禁用 default，避免 server_name "_" 冲突导致 /mobile/ 404
if [[ -L /etc/nginx/sites-enabled/default ]] || [[ -f /etc/nginx/sites-enabled/default ]]; then
  echo "==> 禁用 default 站点（避免 80 端口 server_name 冲突）"
  sudo rm -f /etc/nginx/sites-enabled/default
fi

echo "==> 检查配置"
sudo nginx -t

echo "==> 重载 Nginx"
sudo systemctl enable nginx
sudo systemctl reload nginx

echo ""
echo "完成。请在手机浏览器打开："
echo "  http://<本机公网IP>:8080/mobile/index.html"
echo ""
echo "确认 js/config.js 中 apiBase 为 ''（空字符串，同域反代）。"
echo "若 80 由 Caddy 占用，请: sudo systemctl enable --now caddy"
echo "并放行防火墙: sudo ufw allow 8080/tcp"
echo "若 8081 已对公网开放，建议安全组限制 8081 仅办公 IP。"

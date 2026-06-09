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

# 避免与 default 站点冲突时可禁用 default（可选）
# sudo rm -f /etc/nginx/sites-enabled/default

echo "==> 检查配置"
sudo nginx -t

echo "==> 重载 Nginx"
sudo systemctl enable nginx
sudo systemctl reload nginx

echo ""
echo "完成。请在手机浏览器打开："
echo "  http://<本机公网IP>/mobile/index.html"
echo ""
echo "确认 js/config.js 中 apiBase 为 ''（空字符串，同域反代）。"
echo "若 8081 已对公网开放，建议云安全组限制 8081 仅内网/办公 IP，公网只开 80。"

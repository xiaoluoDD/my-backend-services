#!/usr/bin/env bash
# 在 Ubuntu 云服务器上配置 Nginx，托管 ProjectShow/web 静态页。
# 用法：
#   bash deploy/setup-mobile-web.sh /home/ubuntu/projectshow-web
#
# 前提：
#   1. 已把 Windows 上 ProjectShow/web 目录上传到 WEB_BASE/releases/<版本号>
#   2. wecom-http 已在 8081 运行（systemctl status wecom-http）
#   3. 已安装 nginx（sudo apt install -y nginx）

set -euo pipefail

WEB_BASE="${1:-}"
if [[ -z "$WEB_BASE" ]]; then
  echo "用法: bash deploy/setup-mobile-web.sh /path/to/web-base"
  echo "示例: bash deploy/setup-mobile-web.sh /home/ubuntu/projectshow-web"
  exit 1
fi

WEB_CURRENT="$WEB_BASE/current"
WEB_PREVIEW="$WEB_BASE/preview"
WEB_LEGACY="/var/www/project-show-web"

if [[ ! -d "$WEB_BASE" ]]; then
  echo "错误: 未找到 $WEB_BASE"
  echo "请先创建网页基目录并放入 releases/current/preview 结构"
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
sed \
  -e "s|@WEB_CURRENT@|$WEB_CURRENT|g" \
  -e "s|@WEB_PREVIEW@|$WEB_PREVIEW|g" \
  "$ROOT/deploy/nginx-mobile.conf" | sudo tee "$SITE" > /dev/null

# 兼容旧结构：如果正式版还没切到 current，但旧目录还在，就先把 current 指过去。
if [[ ! -e "$WEB_CURRENT" && -d "$WEB_LEGACY" ]]; then
  echo "==> 检测到旧网页目录，先让 current 指向旧正式版"
  sudo ln -sfn "$WEB_LEGACY" "$WEB_CURRENT"
fi

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
echo "预览地址："
echo "  http://<本机公网IP>:8080/mobile-preview/index.html"
echo ""
echo "若 80 由 Caddy 占用，请: sudo systemctl enable --now caddy"
echo "并放行防火墙: sudo ufw allow 8080/tcp"
echo "若 8081 已对公网开放，建议安全组限制 8081 仅办公 IP。"

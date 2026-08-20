#!/usr/bin/env bash
# 将预览版切换为正式版，用户访问 /mobile/ 会立即看到新版本。
#
# 用法：
#   bash deploy/promote-mobile-web.sh
#
# 可选环境变量：
#   WEB_BASE=/var/www/projectshow-web
#   WEB_CURRENT=/var/www/projectshow-web/current
#   WEB_PREVIEW=/var/www/projectshow-web/preview
#
# 也可以直接指定一个发布目录：
#   bash deploy/promote-mobile-web.sh /var/www/projectshow-web/releases/20260820_153000

set -euo pipefail

WEB_BASE="${WEB_BASE:-/var/www/projectshow-web}"
WEB_CURRENT="${WEB_CURRENT:-$WEB_BASE/current}"
WEB_PREVIEW="${WEB_PREVIEW:-$WEB_BASE/preview}"
TARGET="${1:-}"
NGINX_SITE="${NGINX_SITE:-/etc/nginx/sites-available/project-mobile}"

if [[ -z "$TARGET" ]]; then
  if [[ -L "$WEB_PREVIEW" ]]; then
    TARGET="$(readlink -f "$WEB_PREVIEW")"
  else
    echo "错误: 未指定目标目录，且找不到预览版 $WEB_PREVIEW"
    echo "请先运行 deploy/update-all.sh 生成预览版，或手动传入 release 目录"
    exit 1
  fi
fi

if [[ ! -d "$TARGET" ]]; then
  echo "错误: 目标目录不存在: $TARGET"
  exit 1
fi

if [[ ! -f "$TARGET/index.html" ]]; then
  echo "错误: 目标目录不是有效网页目录: $TARGET"
  exit 1
fi

echo "==> 切换正式版"
echo "当前正式版: ${WEB_CURRENT}"
echo "新正式版:   ${TARGET}"

sudo mkdir -p "$WEB_BASE"
sudo ln -sfn "$TARGET" "$WEB_CURRENT"

if [[ -f "$NGINX_SITE" ]]; then
  sudo nginx -t
  sudo systemctl reload nginx
fi

echo ""
echo "完成。正式地址："
echo "  http://<服务器IP>:8080/mobile/index.html"
echo "预览地址："
echo "  http://<服务器IP>:8080/mobile-preview/index.html"

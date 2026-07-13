#!/usr/bin/env bash
# 从 Git 仓库拉取 ProjectShow/web，同步到 Nginx 静态目录。
#
# 首次在服务器：
#   git clone <你的 ProjectShow 仓库> ~/ProjectShow
#
# 日常更新（在 Windows push 后，服务器执行）：
#   bash ~/my-backend-services/deploy/sync-mobile-web.sh
#
# 可选环境变量：
#   PROJECT_SHOW=~/ProjectShow     ProjectShow 仓库路径
#   WEB_DEST=/var/www/project-show-web   Nginx 静态目录

set -euo pipefail

PROJECT_SHOW="${PROJECT_SHOW:-$HOME/ProjectShow}"
WEB_SRC="$PROJECT_SHOW/web"
WEB_DEST="${WEB_DEST:-/var/www/project-show-web}"

if [[ ! -d "$PROJECT_SHOW/.git" ]]; then
  echo "错误: 未找到 Git 仓库 $PROJECT_SHOW"
  echo "请先: git clone <ProjectShow 仓库地址> $PROJECT_SHOW"
  exit 1
fi

if [[ ! -f "$WEB_SRC/index.html" ]]; then
  echo "错误: 未找到 $WEB_SRC/index.html"
  echo "请确认 ProjectShow 仓库里已提交 web/ 目录"
  exit 1
fi

echo "==> git pull $PROJECT_SHOW"
git -C "$PROJECT_SHOW" pull --ff-only

echo "==> 同步 $WEB_SRC -> $WEB_DEST"
sudo mkdir -p "$WEB_DEST"
sudo rsync -a --delete "$WEB_SRC/" "$WEB_DEST/"
sudo chown -R www-data:www-data "$WEB_DEST"
sudo chmod -R a+rX "$WEB_DEST"

echo ""
echo "完成。手机刷新: http://<服务器IP>:8080/mobile/index.html"

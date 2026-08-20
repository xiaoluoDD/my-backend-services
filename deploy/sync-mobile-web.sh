#!/usr/bin/env bash
# 从 Git 仓库拉取 ProjectShow/web，先同步到预览版 release 目录。
#
# 首次在服务器：
#   git clone <你的 ProjectShow 仓库> ~/ProjectShow
#
# 日常更新（在 Windows push 后，服务器执行）：
#   bash ~/my-backend-services/deploy/sync-mobile-web.sh
#
# 可选环境变量：
#   PROJECT_SHOW=~/ProjectShow     ProjectShow 仓库路径
#   WEB_BASE=/var/www/projectshow-web
#   WEB_RELEASES=/var/www/projectshow-web/releases
#   WEB_PREVIEW=/var/www/projectshow-web/preview

set -euo pipefail

PROJECT_SHOW="${PROJECT_SHOW:-$HOME/ProjectShow}"
WEB_SRC="$PROJECT_SHOW/web"
WEB_BASE="${WEB_BASE:-/var/www/projectshow-web}"
WEB_RELEASES="${WEB_RELEASES:-$WEB_BASE/releases}"
WEB_PREVIEW="${WEB_PREVIEW:-$WEB_BASE/preview}"
WEB_LEGACY="${WEB_LEGACY:-/var/www/project-show-web}"

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

sudo mkdir -p "$WEB_BASE" "$WEB_RELEASES"
if [[ ! -e "$WEB_PREVIEW" && -d "$WEB_LEGACY" ]]; then
  sudo ln -sfn "$WEB_LEGACY" "$WEB_BASE/current"
fi

RELEASE_NAME="$(date +%Y%m%d_%H%M%S)"
RELEASE_DIR="$WEB_RELEASES/$RELEASE_NAME"

echo "==> 同步 $WEB_SRC -> $RELEASE_DIR"
sudo mkdir -p "$RELEASE_DIR"
sudo rsync -a --delete "$WEB_SRC/" "$RELEASE_DIR/"
sudo chown -R www-data:www-data "$RELEASE_DIR"
sudo chmod -R a+rX "$RELEASE_DIR"
sudo mkdir -p "$WEB_BASE"
sudo ln -sfn "$RELEASE_DIR" "$WEB_PREVIEW"

echo ""
echo "完成。预览地址: http://<服务器IP>:8080/mobile-preview/index.html"
echo "正式地址仍保持不变: http://<服务器IP>:8080/mobile/index.html"

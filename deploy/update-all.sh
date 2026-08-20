#!/usr/bin/env bash
# 一键更新：后端 + 手机网页 + Nginx 反代配置
#
# 服务器日常用法（push 到 GitHub 后）：
#   bash ~/my-backend-services/deploy/update-all.sh
#
# 可选环境变量：
#   PROJECT_SHOW=~/ProjectShow
#   WEB_BASE=/var/www/projectshow-web
#   NGINX_SITE=/etc/nginx/sites-available/project-mobile
#   SKIP_NGINX=1          跳过 Nginx 配置同步
#   SKIP_WEB=1            跳过网页同步
#   SKIP_BACKEND=1        跳过后端编译重启

set -euo pipefail

# 通过软链接调用时，$0 可能是 ~/update-all.sh，需解析真实路径
SCRIPT_PATH="$(readlink -f "${BASH_SOURCE[0]}")"
ROOT="$(cd "$(dirname "$SCRIPT_PATH")/.." && pwd)"
PROJECT_SHOW="${PROJECT_SHOW:-$HOME/ProjectShow}"
WEB_BASE="${WEB_BASE:-/var/www/projectshow-web}"
WEB_RELEASES="${WEB_RELEASES:-$WEB_BASE/releases}"
WEB_PREVIEW="${WEB_PREVIEW:-$WEB_BASE/preview}"
WEB_CURRENT="${WEB_CURRENT:-$WEB_BASE/current}"
WEB_LEGACY="${WEB_LEGACY:-/var/www/project-show-web}"
NGINX_SITE="${NGINX_SITE:-/etc/nginx/sites-available/project-mobile}"
UNIT="wecom-http"

echo "=========================================="
echo " 一键更新 ProjectShow 云端服务"
echo "=========================================="
echo "后端仓库: $ROOT"
echo "网页仓库: $PROJECT_SHOW"
echo "网页基目录: $WEB_BASE"
echo ""

# ---------- 1. 后端 ----------
if [[ "${SKIP_BACKEND:-0}" != "1" ]]; then
  echo "==> [1/3] 更新后端 wecom-http"
  cd "$ROOT"
  if git rev-parse --git-dir >/dev/null 2>&1; then
    git pull --ff-only
  else
    echo "警告: 后端目录不是 git 仓库，跳过 git pull"
  fi
  bash "$ROOT/deploy/build.sh"
  sudo systemctl restart "$UNIT"
  sudo systemctl --no-pager --full status "$UNIT" || true
  echo ""
else
  echo "==> [1/3] 跳过后端（SKIP_BACKEND=1）"
  echo ""
fi

# ---------- 2. 网页 ----------
if [[ "${SKIP_WEB:-0}" != "1" ]]; then
  echo "==> [2/3] 更新手机网页"
  if [[ ! -d "$PROJECT_SHOW/.git" ]]; then
    echo "错误: 未找到 $PROJECT_SHOW"
    echo "请先: git clone <ProjectShow 仓库> $PROJECT_SHOW"
    exit 1
  fi
  if [[ ! -f "$PROJECT_SHOW/web/index.html" ]]; then
    echo "错误: 未找到 $PROJECT_SHOW/web/index.html"
    exit 1
  fi
  git -C "$PROJECT_SHOW" pull --ff-only
  sudo mkdir -p "$WEB_BASE" "$WEB_RELEASES"
  if [[ ! -e "$WEB_CURRENT" && -d "$WEB_LEGACY" ]]; then
    echo "==> 检测到旧正式目录，先让 current 指向旧站点"
    sudo ln -sfn "$WEB_LEGACY" "$WEB_CURRENT"
  fi
  RELEASE_NAME="$(date +%Y%m%d_%H%M%S)"
  RELEASE_DIR="$WEB_RELEASES/$RELEASE_NAME"
  sudo mkdir -p "$RELEASE_DIR"
  sudo rsync -a --delete "$PROJECT_SHOW/web/" "$RELEASE_DIR/"
  sudo chown -R www-data:www-data "$RELEASE_DIR"
  sudo chmod -R a+rX "$RELEASE_DIR"
  sudo ln -sfn "$RELEASE_DIR" "$WEB_PREVIEW"
  echo "预览版已同步到 $RELEASE_DIR"
  echo "预览地址: http://<服务器IP>:8080/mobile-preview/index.html"
  echo ""
else
  echo "==> [2/3] 跳过网页（SKIP_WEB=1）"
  echo ""
fi

# ---------- 3. Nginx ----------
if [[ "${SKIP_NGINX:-0}" != "1" ]]; then
  echo "==> [3/3] 同步 Nginx 配置并 reload"
  CONF_SRC="$ROOT/deploy/nginx-mobile.conf"
  if [[ ! -f "$CONF_SRC" ]]; then
    echo "警告: 未找到 $CONF_SRC，跳过 Nginx"
  else
    TMP="$(mktemp)"
    sed \
      -e "s|@WEB_CURRENT@|$WEB_CURRENT|g" \
      -e "s|@WEB_PREVIEW@|$WEB_PREVIEW|g" \
      "$CONF_SRC" > "$TMP"
    sudo cp "$TMP" "$NGINX_SITE"
    rm -f "$TMP"
    # 确保启用站点
    if [[ -d /etc/nginx/sites-enabled ]]; then
      sudo ln -sfn "$NGINX_SITE" /etc/nginx/sites-enabled/project-mobile
    fi
    sudo nginx -t
    sudo systemctl reload nginx
    echo "Nginx 已更新: $NGINX_SITE"
  fi
  echo ""
else
  echo "==> [3/3] 跳过 Nginx（SKIP_NGINX=1）"
  echo ""
fi

# ---------- 自检 ----------
echo "==> 自检"
curl -s -o /dev/null -w "ping                %{http_code}\n" http://127.0.0.1:8081/ping || true
curl -s -o /dev/null -w "dashboard/summary   %{http_code}\n" http://127.0.0.1:8081/api/dashboard/summary || true
curl -s -o /dev/null -w "auth/login(OPTIONS略) — 用 POST 测登录\n" http://127.0.0.1:8081/api/auth/me || true
curl -s -o /dev/null -w "nginx mobile preview %{http_code}\n" http://127.0.0.1:8080/mobile-preview/index.html || true
curl -s -o /dev/null -w "nginx mobile index  %{http_code}\n" http://127.0.0.1:8080/mobile/index.html || true
curl -s -o /dev/null -w "nginx dashboard api %{http_code}\n" http://127.0.0.1:8080/api/dashboard/summary || true
curl -s -o /dev/null -w "nginx settings api %{http_code}\n" http://127.0.0.1:8080/api/settings || true
curl -s -o /dev/null -w "nginx logs api     %{http_code}\n" http://127.0.0.1:8080/api/logs || true

echo ""
echo "=========================================="
echo " 全部完成。预览地址："
echo " http://<服务器IP>:8080/mobile-preview/index.html"
echo "切正式版请执行 deploy/promote-mobile-web.sh"
echo "手机正式地址："
echo " http://<服务器IP>:8080/mobile/index.html"
echo "=========================================="
echo ""
echo "可选快捷方式（只需执行一次）："
echo "  ln -sf $ROOT/deploy/update-all.sh ~/update-all.sh"
echo "之后直接: bash ~/update-all.sh"

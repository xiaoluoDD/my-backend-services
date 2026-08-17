# 手机只读 Web 看板 — 云服务器部署指南

本文说明如何在 **Ubuntu 云服务器** 上部署 `ProjectShow/web`（手机只读项目看板），与现有 **Go 后端 wecom-http（8081）** 配合使用。

适用场景：新购云服务器、重装系统后，需要从零部署 Web；或迁移到新机器。

---

## 一、架构说明

```
手机浏览器
    ↓  http://<公网IP>/mobile/index.html
Nginx :80
    ├─ /mobile/*              →  /var/www/project-show-web/   （静态 HTML/CSS/JS）
    └─ /api/projects 等       →  127.0.0.1:8081               （Go 后端 wecom-http）
                                        ↓
                                   SQLite (wecom.db)
```

| 组件 | 说明 |
|------|------|
| 源码仓库 | GitHub `xiaoluoDD/ProjectShow`，目录 `web/` |
| 服务器 Git 目录 | `~/ProjectShow` |
| Nginx 发布目录 | `/var/www/project-show-web/`（不要用 `/home/ubuntu/...`，会 403） |
| 后端服务 | `systemctl status wecom-http`，端口 **8081** |
| Web 配置 | `web/js/config.js` 中 `apiBase: ''`（空字符串，走 Nginx 同域） |

---

## 二、前置条件

在新服务器上应先完成 **Go 后端** 部署，详见 [deploy-systemd.md](./deploy-systemd.md)。

确认后端正常：

```bash
curl -s http://127.0.0.1:8081/ping          # 应返回 pong
curl -s http://127.0.0.1:8081/api/projects | head -c 120   # 应返回 JSON
```

还需具备：

- 服务器可 SSH 登录（用户示例：`ubuntu`）
- 服务器已配置 GitHub SSH 密钥（用于 `git clone` ProjectShow）
- 云安全组稍后需放行 **TCP 80**

---

## 三、首次部署（新服务器完整步骤）

以下在 **云服务器** 上执行。

### 步骤 1：克隆 ProjectShow 仓库

```bash
cd ~
git clone git@github.com:xiaoluoDD/ProjectShow.git ProjectShow
ls ~/ProjectShow/web/index.html
```

若 SSH 未配 GitHub，可改用 HTTPS：

```bash
git clone https://github.com/xiaoluoDD/ProjectShow.git ProjectShow
```

### 步骤 2：确认 Web 配置

```bash
grep apiBase ~/ProjectShow/web/js/config.js
```

应为：

```javascript
apiBase: '',
```

**不要**在 Nginx 同域部署时填 `:8081` 地址，否则手机端易出现跨域问题。

### 步骤 3：安装 Nginx

```bash
sudo apt update
sudo apt install -y nginx
```

### 步骤 4：写入 Nginx 站点配置

推荐使用 **`/var/www/project-show-web`** 作为静态目录（避免家目录权限 403）。

用 `nano` 创建（**建议手打或从本仓库 `deploy/nginx-mobile.conf` 复制，勿从聊天富文本粘贴**，以免混入不可见字符）：

```bash
sudo nano /etc/nginx/sites-available/project-mobile
```

粘贴以下内容（路径已写死为 `/var/www/project-show-web`）：

```nginx
server {
    listen 8080;
    server_name _;

    location /mobile/ {
        alias /var/www/project-show-web/;
        index index.html;
    }

    # 全部 API（含设置/提醒/日志/DB 导出）反代到 Go
    location /api/ {
        proxy_pass http://127.0.0.1:8081;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 300s;
        client_max_body_size 50m;
    }
}
```

保存：`Ctrl+O` → 回车 → `Ctrl+X`。

或从 backend 仓库复制模板（需先 `git clone` my-backend-services）：

```bash
sudo cp ~/my-backend-services/deploy/nginx-mobile.conf /etc/nginx/sites-available/project-mobile
sudo sed -i 's|@WEB_ROOT@|/var/www/project-show-web|g' /etc/nginx/sites-available/project-mobile
```

### 步骤 5：启用站点并禁用 default（避免 404/冲突）

```bash
sudo ln -sf /etc/nginx/sites-available/project-mobile /etc/nginx/sites-enabled/project-mobile
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t
sudo systemctl reload nginx
```

`nginx -t` 应显示 `syntax is ok` 且 **无** `conflicting server name` 警告。

### 步骤 6：同步 Web 文件到 /var/www

**方式 A — 使用同步脚本（推荐）**

若已有 `~/sync-mobile-web.sh`（见步骤 7）或 backend 仓库中的 `deploy/sync-mobile-web.sh`：

```bash
bash ~/sync-mobile-web.sh
# 或
bash ~/my-backend-services/deploy/sync-mobile-web.sh
```

**方式 B — 手动首次同步**

```bash
sudo mkdir -p /var/www/project-show-web
sudo rsync -a --delete ~/ProjectShow/web/ /var/www/project-show-web/
sudo chown -R www-data:www-data /var/www/project-show-web
sudo chmod -R a+rX /var/www/project-show-web
```

### 步骤 7：安装日常同步脚本（可选但推荐）

在 **用户 home 目录** 创建（避免 `sudo tee` 导致 root 权限无法 chmod）：

```bash
cat > ~/sync-mobile-web.sh << 'EOF'
#!/usr/bin/env bash
set -euo pipefail
PROJECT_SHOW="${PROJECT_SHOW:-$HOME/ProjectShow}"
WEB_SRC="$PROJECT_SHOW/web"
WEB_DEST="${WEB_DEST:-/var/www/project-show-web}"
if [[ ! -d "$PROJECT_SHOW/.git" ]]; then echo "error: no git repo $PROJECT_SHOW"; exit 1; fi
if [[ ! -f "$WEB_SRC/index.html" ]]; then echo "error: no $WEB_SRC/index.html"; exit 1; fi
echo "==> git pull $PROJECT_SHOW"
git -C "$PROJECT_SHOW" pull --ff-only
echo "==> rsync $WEB_SRC -> $WEB_DEST"
sudo mkdir -p "$WEB_DEST"
sudo rsync -a --delete "$WEB_SRC/" "$WEB_DEST/"
sudo chown -R www-data:www-data "$WEB_DEST"
sudo chmod -R a+rX "$WEB_DEST"
echo "done: http://YOUR_IP/mobile/index.html"
EOF

chmod +x ~/sync-mobile-web.sh
```

backend 仓库内也提供：`deploy/sync-mobile-web.sh`（逻辑相同）。

### 步骤 8：放行防火墙 / 安全组

**云厂商控制台**：入站规则添加 **TCP 80**。

**UFW（若启用）**：

```bash
sudo ufw allow 80/tcp
sudo ufw status
```

**8081 建议**：若仅办公网络使用 Qt 桌面，可在安全组限制 8081 来源 IP；手机 Web 只走 80 端口即可。

### 步骤 9：验证

```bash
curl -s http://127.0.0.1/mobile/index.html | head -c 120
curl -s http://127.0.0.1/api/projects | head -c 120
```

- 第一条应出现 `<!DOCTYPE html>`
- 第二条应出现 `"ok":true`

手机浏览器访问：

```
http://<公网IP>/mobile/index.html
```

---

## 四、日常更新 Web（改代码后发布）

### Windows（开发机）

```bash
cd /d/QTProject/TOYOTAProject/ProjectShow

# 修改 web/ 下文件后
git add web/
git commit -m "update mobile web: 说明"
git push
```

### 云服务器

```bash
bash ~/sync-mobile-web.sh
```

手机浏览器刷新页面。**无需**重启 `wecom-http`，**无需**重新编译。

---

## 五、与后端更新对照

| 改了什么 | Windows | 云服务器 |
|----------|---------|----------|
| 仅 `ProjectShow/web/` | ProjectShow: `git push` | `bash ~/sync-mobile-web.sh` |
| 仅 Go 后端 | my-backend-services: `git push` | `cd ~/my-backend-services && ./deploy/update.sh` |
| Web + 后端 | 两个仓库都 push | 两条命令都执行 |

---

## 六、目录与 URL 备忘

| 项目 | 路径 / 地址 |
|------|-------------|
| Web 源码（Git） | `~/ProjectShow/web/` |
| Nginx 静态发布 | `/var/www/project-show-web/` |
| Nginx 配置 | `/etc/nginx/sites-available/project-mobile` |
| 手机访问 URL | `http://<IP>/mobile/index.html` |
| Qt 桌面后端 | `http://<IP>:8081` |
| 同步脚本 | `~/sync-mobile-web.sh` |

---

## 七、常见问题

### 1. `404 Not Found`（/mobile 或 /api）

- 检查是否删除 `sites-enabled/default`：`ls /etc/nginx/sites-enabled/`
- 确认 `project-mobile` 已启用：`sudo nginx -t`

### 2. `403 Forbidden`（/mobile）

- 静态文件不要放在 `/home/ubuntu/...`，改用 `/var/www/project-show-web/`
- 执行：`sudo chown -R www-data:www-data /var/www/project-show-web`

### 3. 页面打开但列表一直加载失败

- 确认 `config.js` 中 `apiBase: ''`
- 确认 `curl http://127.0.0.1/api/projects` 正常
- 浏览器 F12 看 Network 是否 404/502

### 4. `nginx: conflicting server name "_"`

- 删除 default 站点：`sudo rm -f /etc/nginx/sites-enabled/default`
- 重载：`sudo systemctl reload nginx`

### 5. `git pull` 或 clone 失败

- 检查服务器 SSH：`ssh -T git@github.com`
- 应显示 `Hi xiaoluoDD!`

### 6. 配置粘贴后 `unknown directive "￼"`

- 从富文本/chat 复制 Nginx 配置会带入不可见字符
- 用 `nano` 手打，或从本仓库 `deploy/nginx-mobile.conf` 复制

---

## 八、新服务器最小命令清单（复制用）

假设后端已按 deploy-systemd.md 部署完毕：

```bash
# 1. 克隆 Web 源码
git clone git@github.com:xiaoluoDD/ProjectShow.git ~/ProjectShow

# 2. Nginx
sudo apt install -y nginx
sudo cp ~/my-backend-services/deploy/nginx-mobile.conf /etc/nginx/sites-available/project-mobile
sudo sed -i 's|@WEB_ROOT@|/var/www/project-show-web|g' /etc/nginx/sites-available/project-mobile
sudo ln -sf /etc/nginx/sites-available/project-mobile /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t && sudo systemctl reload nginx

# 3. 发布静态文件
sudo rsync -a --delete ~/ProjectShow/web/ /var/www/project-show-web/
sudo chown -R www-data:www-data /var/www/project-show-web

# 4. 验证
curl -s http://127.0.0.1/mobile/index.html | head -c 80
curl -s http://127.0.0.1/api/projects | head -c 80

# 5. 安装同步脚本（见第三节步骤 7），以后更新只需：
# bash ~/sync-mobile-web.sh
```

---

## 九、相关文件

| 文件 | 仓库 | 说明 |
|------|------|------|
| `web/` | ProjectShow | H5 源码 |
| `deploy/nginx-mobile.conf` | my-backend-services | Nginx 模板 |
| `deploy/sync-mobile-web.sh` | my-backend-services | Git pull + rsync 脚本 |
| `docs/deploy-systemd.md` | my-backend-services | Go 后端部署 |

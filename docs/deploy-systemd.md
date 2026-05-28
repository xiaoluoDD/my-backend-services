# 编译二进制 + systemd 部署指南

本文说明如何在 **Ubuntu 云服务器** 上，将 `http` 服务以 **编译好的二进制 + systemd** 方式长期运行，以及日常 **更新、重启、排错** 操作。

默认监听端口：**8081**（8080 留给 `echohttp`）。  
环境变量文件：**`wecom/.env`**（不要提交到 Git）。

---

## 一、架构说明

```
项目目录 ~/my-backend-services/
├── bin/wecom-http          ← 编译出的可执行文件
├── http/                   ← Go 源码（HTTP API）
├── wecom/.env              ← 密钥与配置（systemd 自动加载）
├── logs/http.log           ← 应用日志（LOG_DIR=logs 时）
└── deploy/
    ├── build.sh            ← 仅编译
    ├── install-systemd.sh  ← 首次安装 systemd
    └── update.sh           ← 日常：pull + 编译 + 重启
```

systemd 负责：

- 开机自启
- 进程崩溃后自动重启
- 从 `wecom/.env` 注入环境变量（无需每次 `source`）

---

## 二、首次部署（只做一次）

以下在 **云服务器** 上执行，假设项目路径为 `~/my-backend-services`，用户为 `ubuntu`。若路径不同，请自行替换。

### 1. 安装 Go（若尚未安装）

```bash
go version
# 需要 Go 1.22+（与 go.mod 一致）
```

未安装时可参考 [https://go.dev/dl/](https://go.dev/dl/) 或使用发行版包管理器安装。

### 2. 拉取代码

```bash
cd ~
git clone <你的仓库地址> my-backend-services
cd my-backend-services
```

若已有目录，则：

```bash
cd ~/my-backend-services
git pull
```

### 3. 配置环境变量

```bash
cp wecom/.env.example wecom/.env
nano wecom/.env   # 或 vim
```

至少填写：

| 变量 | 说明 |
|------|------|
| `WECOM_CORP_ID` | 企业 ID |
| `WECOM_AGENT_ID` | 应用 AgentId |
| `WECOM_SECRET` | 应用 Secret |
| `WECOM_TO_USER` | 接收测试消息的 userid |
| `HTTP_PORT` | 建议 `8081` |

可选日志配置：

```bash
LOG_LEVEL=info
LOG_DIR=logs
```

保存后确认权限（仅本人可读）：

```bash
chmod 600 wecom/.env
```

### 4. 编译二进制

```bash
cd ~/my-backend-services
chmod +x deploy/*.sh
./deploy/build.sh
```

成功后会生成：`~/my-backend-services/bin/wecom-http`

### 5. 安装并启动 systemd 服务

```bash
./deploy/install-systemd.sh
```

脚本会：

1. 检查 `wecom/.env` 是否存在  
2. 若未编译则自动执行 `build.sh`  
3. 生成 `/etc/systemd/system/wecom-http.service`  
4. `systemctl enable` + `systemctl start`

### 6. 放行防火墙 / 安全组

- **云厂商安全组**：入站 TCP **8081**（来源可按需限制为你的办公 IP）  
- 若启用了 UFW：

```bash
sudo ufw allow 8081/tcp
sudo ufw status
```

### 7. 验证服务

```bash
# 服务是否在跑
sudo systemctl status wecom-http

# 本机接口
curl -s http://127.0.0.1:8081/ping
# 应返回: pong

curl -s -X POST http://127.0.0.1:8081/api/wecom/test
# 应返回 JSON，且企业微信收到测试消息
```

从本机 Qt 看板访问：`http://<服务器公网IP>:8081`

---

## 三、日常更新与重启（最常用）

每次在本地改完代码并 **push 到 GitHub** 后，在服务器执行：

```bash
cd ~/my-backend-services
./deploy/update.sh
```

该脚本依次执行：

1. `git pull`  
2. `./deploy/build.sh`（重新编译 `bin/wecom-http`）  
3. `sudo systemctl restart wecom-http`  

### 仅改了 `.env`、未改代码

修改 `wecom/.env` 后需要重启才能生效：

```bash
sudo systemctl restart wecom-http
```

### 仅重新编译、未拉代码

```bash
./deploy/build.sh
sudo systemctl restart wecom-http
```

### 常用 systemctl 命令

| 操作 | 命令 |
|------|------|
| 查看状态 | `sudo systemctl status wecom-http` |
| 启动 | `sudo systemctl start wecom-http` |
| 停止 | `sudo systemctl stop wecom-http` |
| 重启 | `sudo systemctl restart wecom-http` |
| 开机自启 | `sudo systemctl enable wecom-http` |
| 取消自启 | `sudo systemctl disable wecom-http` |

---

## 四、查看日志（排查问题）

### 1. systemd 日志（启动失败、崩溃重启）

```bash
# 最近 100 行
journalctl -u wecom-http -n 100 --no-pager

# 实时跟踪
journalctl -u wecom-http -f
```

### 2. 应用日志文件

默认路径（在项目根目录下）：

```bash
tail -f ~/my-backend-services/logs/http.log
```

临时打开调试级别：在 `wecom/.env` 中设置 `LOG_LEVEL=debug`，然后：

```bash
sudo systemctl restart wecom-http
```

### 3. 确认端口监听

```bash
sudo ss -tlnp | grep 8081
```

应看到 `wecom-http` 在监听，而不是 `echohttp`（8080 是另一个服务）。

---

## 五、手动安装 systemd（不用脚本时）

若不想用 `install-systemd.sh`，可手工操作：

```bash
# 1. 编译
cd ~/my-backend-services
./deploy/build.sh

# 2. 编辑 service 中的路径后安装
sudo cp deploy/wecom-http.service /etc/systemd/system/wecom-http.service
sudo nano /etc/systemd/system/wecom-http.service
```

将文件中三处占位符改为实际值：

- `@REPO_ROOT@` → `/home/ubuntu/my-backend-services`
- `@USER@` → `ubuntu`
- `ExecStart` 指向 `.../bin/wecom-http`

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now wecom-http
```

---

## 六、与旧方式对比

| 旧方式 | systemd 方式 |
|--------|----------------|
| `set -a && source ../wecom/.env && go run .` | 不用 source，env 由 systemd 加载 |
| 关 SSH 可能退出 | 后台常驻 |
| 崩溃需手动再起 | `Restart=on-failure` 自动重启 |
| 每次现场编译 | 固定二进制 `bin/wecom-http`，启动快 |

---

## 七、常见问题

### 1. `address already in use`（8081 被占用）

```bash
sudo ss -tlnp | grep 8081
```

结束占用进程或修改 `wecom/.env` 中 `HTTP_PORT`，并同步修改 Qt 看板地址与安全组。

### 2. 服务启动失败 `status=203/EXEC`

- 二进制不存在：先 `./deploy/build.sh`  
- 无执行权限：`chmod +x bin/wecom-http`

### 3. 企业微信发送失败

- 检查 `wecom/.env` 配置  
- `journalctl -u wecom-http -n 50`  
- `tail -f logs/http.log`

### 4. 外网访问不了，本机 curl 正常

- 检查云安全组是否放行 **8081**  
- 检查服务器防火墙 `ufw`

### 5. 修改代码后没生效

确认已执行：

```bash
./deploy/update.sh
```

或至少：`./deploy/build.sh` + `sudo systemctl restart wecom-http`  
（仅 `git pull` 不会更新正在运行的二进制。）

---

## 八、快速命令备忘

```bash
# 首次
cd ~/my-backend-services
cp wecom/.env.example wecom/.env   # 编辑填真实值
chmod +x deploy/*.sh
./deploy/install-systemd.sh

# 每次发版
./deploy/update.sh

# 看日志
journalctl -u wecom-http -f
tail -f logs/http.log

# 测试
curl http://127.0.0.1:8081/ping
curl -X POST http://127.0.0.1:8081/api/wecom/test
```

---

## 九、本地 Windows 开发说明

在 Windows 上仍可 `go run` 调试；**正式环境只在 Linux 服务器** 使用 `deploy/build.sh` + systemd。  
本地 push 代码后，到服务器执行 `./deploy/update.sh` 即可。

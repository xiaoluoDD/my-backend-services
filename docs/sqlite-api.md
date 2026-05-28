# SQLite 数据持久化与企业微信成员同步

## 数据存什么

| 表 | 内容 |
|----|------|
| `app_users` | 自建应用**可见范围内**的成员（userid、姓名、手机、部门、来源） |
| `sync_runs` | 每次同步的开始/结束时间、状态、人数、错误信息 |
| `corp_info` | 企业 ID、AgentId 等配置快照 |

数据库文件：环境变量 `DB_PATH`，默认 **`data/wecom.db`**（相对项目根目录）。

## HTTP 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST/GET | `/api/wecom/test` | 发送测试消息；POST body: `{"userid":"成员ID"}` |
| POST/GET | `/api/wecom/sync` | 从企业微信拉取成员并写入数据库 |
| GET | `/api/wecom/users` | 返回已保存的成员列表 |
| GET | `/api/wecom/stats` | 人数、上次同步、`corp_info` |

## 权限要求

在企业微信管理后台 → 自建应用 → 权限管理中，需开启例如：

- 通讯录：成员信息只读
- 通讯录：部门信息只读（若可见范围含部门）

否则 `user/list`、`tag/get` 可能返回权限错误，同步失败。

## 同步逻辑简述

1. `agent/get` 读取应用可见范围（成员、部门、标签）
2. 对部门调用 `user/list` 展开成员
3. 对标签调用 `tag/get` 展开成员与部门
4. 按 `userid` 去重后写入 `app_users`
5. 本次未出现的旧成员标记为 `active=0`

## 备份

```bash
cp ~/my-backend-services/data/wecom.db ~/backup/wecom-$(date +%F).db
```

更新程序不会删除数据库；`./deploy/update.sh` 只替换二进制并重启服务。

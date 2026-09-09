# RemoteHelpDesk 私有化部署

此目录是面向交付和长期运维的单机 Docker Compose 方案，适用于 1Panel、
OpenResty 或普通 Linux 服务器。默认包含 Web、API、PostgreSQL、Redis、
Qdrant 和 MinIO。Jitsi、邮件、语音、LLM/Sub2API 都是按需接入的外部服务。

生产站点专用的远程发布仍使用仓库根目录下的
`scripts/deploy-production.sh`。客户私有化环境统一使用本目录的 `rhdctl`，
不要复制生产脚本里的服务器地址、账户或目录约定。

## 交付原则

- 全新安装创建空数据库，不会自动导入演示租户或历史生产数据。
- `.env` 包含密钥且必须保持 `600` 权限，不提交到 Git，也不放进交付文档。
- 数据、备份和发布状态独立于源码目录；正式环境应将它们配置到独立数据盘。
- PostgreSQL、Redis、Qdrant 和 MinIO 控制台默认只监听回环地址。
- 客户入口和 MinIO 文件域名由 1Panel/OpenResty 提供 HTTPS。
- 前端登录密码预填必须为空，因为 `NEXT_PUBLIC_*` 会进入浏览器静态资源。
- 升级前必须成功完成数据库备份和隔离恢复演练，之后才切换容器。

## 主机要求

- Linux x86_64/arm64，Docker Engine 24+，Docker Compose v2。
- 建议至少 4 核 CPU、8 GiB 内存；启用本地模型时按模型另行扩容。
- 初始可用磁盘建议不少于 20 GiB，生产数据和备份分开评估容量。
- 两个域名或 HTTPS 入口：应用域名、MinIO S3 API 域名。
- 服务器时间同步正常，防火墙只开放 80/443 和明确需要的管理入口。

`rhdctl preflight` 会检查 Docker、配置、密钥长度、目录空间、HTTPS、
危险监听地址和 Compose 语法。镜像仍使用 `latest` 时会给出警告；正式交付应
把 Qdrant/MinIO 镜像固定到验证过的版本或 digest。

## 首次安装

```bash
cd deploy/1panel
chmod +x rhdctl preflight.sh doctor.sh
./rhdctl init
```

`init` 只在 `.env` 不存在时执行，自动生成数据库、Redis、MinIO、客户会话、
MCP、加密和初始平台管理员密钥，不会在终端打印密钥。然后至少修改：

```dotenv
RHD_PUBLIC_URL=https://helpdesk.customer.example
MINIO_PUBLIC_ENDPOINT=https://files.customer.example
RHD_DATA_DIR=/data/remotehelpdesk/volumes
RHD_BACKUP_DIR=/data/remotehelpdesk/backups
IMAGE_TAG=customer-2026.09.06-1
```

执行部署：

```bash
./rhdctl preflight --strict
./rhdctl install
```

安装完成后访问 `${RHD_PUBLIC_URL}/platform`。初始平台账号是 `admin`，密码是
`.env` 中随机生成的 `RHD_BOOTSTRAP_ADMIN_PASSWORD`。首次登录后立即在平台侧
修改密码。修改该环境变量不会重置已经存在的管理员密码。

## 1Panel 接入

1. 在 1Panel 安装 Docker/OpenResty，并把完整交付目录放到固定位置。
2. 在服务器终端使用 `rhdctl` 初始化和安装；1Panel 的 Compose 页面可用于查看，
   但升级、备份和恢复仍使用 `rhdctl`，避免操作步骤分叉。
3. 应用域名反向代理到 `127.0.0.1:${APP_PORT}`。
4. 文件域名反向代理到 `127.0.0.1:${MINIO_PORT}`，保留请求 Host、协议和范围请求头。
5. 两个网站都申请并强制使用 HTTPS。确认 `.env` 中的 URL 与外部实际地址完全一致。
6. 若前后端或原生客户端跨域访问 API，在 `remotehelpdesk.yaml` 的
   `server.cors.allowedOrigins` 增加精确 Origin，不使用 `*`。

如果 1Panel/OpenResty 运行在容器中，回环地址不能到达宿主机端口，应改用同一
Docker 网络或宿主机网关。不要为了省事把 PostgreSQL、Redis、Qdrant 管理端口
直接暴露到公网。

## 配置分组

| 分组 | 必填项 | 说明 |
| --- | --- | --- |
| 发布 | `IMAGE_NAME`, `IMAGE_TAG` | 每次发布使用唯一 tag，保证可回退 |
| 入口 | `RHD_PUBLIC_URL`, `MINIO_PUBLIC_ENDPOINT` | 正式环境必须 HTTPS |
| 数据 | `RHD_DATA_DIR`, `RHD_BACKUP_DIR` | 建议绝对路径并置于数据盘 |
| 基础密钥 | PostgreSQL/Redis/MinIO、会话、MCP、加密密钥 | 由 `init` 生成，不复用客户密码 |
| 平台初始化 | `RHD_BOOTSTRAP_ADMIN_PASSWORD` | 仅空库首次创建 `admin` 时使用 |
| AI | Sub2API 或 OpenAI 兼容配置 | 未配置时基础工单能力仍可运行，AI 不可用 |
| 视频 | `JITSI_*` | 不启用时保持空值及 `JITSI_REQUIRE_AUTH=false` |
| 通知 | 邮件/推送配置 | 按客户环境启用并单独验证连通性 |

`ENCRYPTION_KEY` 不能随意更换，否则历史连接器和模型凭据无法解密。迁移旧环境时，
可临时设置 `ENCRYPTION_KEY_FALLBACKS`，同时显式设置
`ALLOW_LEGACY_ENCRYPTION_FALLBACK=1`；迁移完成并重新保存凭据后清空它们。

## 日常运维

```bash
./rhdctl status
./rhdctl doctor
./rhdctl doctor --logs
./rhdctl logs api
./rhdctl backup
./rhdctl restore-drill
```

建议每天调用 `./rhdctl backup`，至少每月调用一次
`./rhdctl restore-drill`。备份包含 `.dump`、`.sha256` 和 `.meta`，并按
`BACKUP_RETENTION_COUNT` 保留。还应把备份异地复制；只保存在同一服务器不算灾备。

## Prometheus 监控

将 `.env` 中 `RHD_MONITORING_ENABLED=1`，并把五个监控镜像固定到已验收版本，
然后执行：

```bash
./rhdctl monitoring-up
./rhdctl monitoring-status
```

监控 profile 包含 PostgreSQL Exporter、Redis Exporter、Node Exporter、
Prometheus 和 Alertmanager，默认只在回环地址开放 `9090/9093`。规则覆盖服务失联、
API 5xx、依赖异常、磁盘不足、备份过期和恢复演练过期。

默认 `alertmanager.yml` 只在控制台保留告警，不向外发送。上线前应通过
`RHD_ALERTMANAGER_CONFIG_FILE` 挂载客户批准的邮件、Webhook 或聊天通知配置，
并真实触发一条告警验证送达。公网和证书到期监控需要在这套服务之外部署
Blackbox Exporter 或接入客户现有监控平台。

## 升级与回退

更新源码并把 `.env` 中 `IMAGE_TAG` 改为新的唯一值后执行：

```bash
./rhdctl upgrade
```

升级顺序为：严格预检 -> 数据库备份 -> 隔离恢复演练 -> 构建镜像 -> API 切换及
就绪检查 -> Web 切换及就绪检查 -> 全量诊断。切换失败时会自动恢复升级前的 API
和 Web 镜像。上一个镜像记录保存在 `RHD_STATE_DIR/rollback.env`，也可手工执行：

```bash
./rhdctl rollback
```

应用镜像回退不会反向修改数据库。涉及删除、重命名、不可逆数据回填的版本，必须有
单独评审的 SQL 迁移和回退方案；不能依赖 GORM AutoMigrate 自动恢复。

## 数据恢复

先用隔离库验证指定备份：

```bash
./rhdctl restore-drill /data/remotehelpdesk/backups/cs_ai_agent_YYYYMMDDTHHMMSSZ.dump
```

替换当前数据库必须同时给出目标库名。命令会先再做一次当前库安全备份，然后才恢复：

```bash
./rhdctl restore /path/to/backup.dump --confirm cs_ai_agent
```

这是破坏性操作，应在维护窗口执行。恢复后 `rhdctl` 会重启应用并运行诊断。

## 故障处理

1. 先运行 `./rhdctl status` 和 `./rhdctl doctor --logs`。
2. API 不健康时先看 PostgreSQL、Redis、Qdrant、MinIO 是否健康，再看 API 日志。
3. Web 健康但外网不可访问时检查 1Panel 反向代理、证书、Host 和转发协议头。
4. 文件链接错误时核对 `MINIO_PUBLIC_ENDPOINT` 与实际 HTTPS 文件域名。
5. 知识解析失败时核对向量库、Embedding 配置和模型配额，不要直接重建数据卷。
6. 升级失败优先保留日志、备份和 rollback state，不执行 `docker compose down -v`。

## 交付清单

交接给其他团队前，双方共同确认：

- 客户域名、证书、DNS、服务器和 1Panel 管理权已经移交。
- `.env` 密钥通过安全渠道移交，仓库和工单中没有明文副本。
- 平台管理员已改密，演示账户和浏览器密码预填均不存在。
- `preflight --strict`、`doctor`、备份和恢复演练都有日期与结果记录。
- 数据目录、备份目录、异地备份、保留周期和恢复责任人明确。
- Jitsi、邮件、LLM、Embedding、Sub2API 等外部服务逐项标明负责人和到期时间。
- 监控系统采集 `/metrics`（仅内网），并对就绪失败、磁盘、备份过期和证书到期告警。

## 文件说明

- `rhdctl`: 统一安装、升级、回退、诊断和备份入口。
- `preflight.sh`: 部署前静态与主机检查。
- `doctor.sh`: 运行中容器及 HTTP 健康诊断。
- `docker-compose.yml`: Web、API 和基础依赖编排。
- `nginx.conf`: Web 静态资源、API/WebSocket 代理及缓存规则。
- `remotehelpdesk.yaml`: API 运行配置。
- `.env.example`: 不含真实客户信息的环境模板。

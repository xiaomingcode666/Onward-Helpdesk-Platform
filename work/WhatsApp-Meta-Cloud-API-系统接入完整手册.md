# WhatsApp Meta Cloud API 系统接入完整手册

> 更新时间：2026-09-29
> 适用范围：RemoteHelpDesk 通过 Meta WhatsApp Cloud API 接收客户消息，并从客服工作台或 AI Agent 回复 WhatsApp 客户。

## 1. 接入结果

完整链路如下：

```text
客户 WhatsApp
  -> Meta WhatsApp Cloud API
  -> RemoteHelpDesk Webhook
  -> 客户身份、会话和消息入库
  -> AI Agent 或人工客服回复
  -> WhatsApp 出站队列
  -> Meta Cloud API
  -> 客户 WhatsApp
  -> sent / delivered / read 状态回传
```

2026-09-29 已完成一次真实端到端验收：

- 手机真实消息成功进入 RemoteHelpDesk；
- 系统创建 WhatsApp 会话 `#48`；
- 同一 Meta 消息只入库一次；
- AI 自动回复成功发送到手机；
- Meta 回传 `delivered` 和 `read`；
- 整个过程未在代码、数据库渠道配置或文档中保存密钥明文。

## 2. 接入前准备

需要准备：

1. 可登录 Meta Developers 的 Facebook 账号；
2. Meta Business Portfolio；
3. Meta Developer App；
4. WhatsApp Business Account，以下简称 WABA；
5. 测试号码或正式 WhatsApp Business 号码；
6. Meta 可访问的公网 HTTPS Webhook；
7. RemoteHelpDesk 租户、已启用的 AI Agent 和接待配置；
8. RemoteHelpDesk 项目密钥存储目录。

测试阶段可以使用 Meta 提供的测试号码。生产阶段必须使用已注册的正式业务号码和稳定域名。

## 3. Meta 侧配置

### 3.1 创建应用和 WhatsApp 用例

1. 进入 [Meta Developers](https://developers.facebook.com/)；
2. 创建应用；
3. 添加“通过 WhatsApp 与客户建立联系”用例；
4. 进入该用例的“第 1 步：试用”；
5. 认领 WhatsApp 测试号码；
6. 添加并验证测试接收手机号；
7. 从测试号码向手机发送一条模板消息，确认手机可以收到。

### 3.2 记录接入标识

需要记录以下非密钥标识：

| Meta 字段 | 系统用途 |
| --- | --- |
| App ID | 标识 Meta 应用 |
| Phone Number ID | 路由具体 WhatsApp 号码 |
| WABA ID | 标识 WhatsApp Business Account |
| Business ID | Meta Business 管理使用，不要误填到系统的 WABA ID 字段 |

系统页面中的“Meta 企业账户 ID”当前应填写 **WABA ID**，不是 Business ID。

### 3.3 准备三项密钥

需要准备：

- Meta Access Token；
- Meta App Secret；
- Webhook Verify Token。

Webhook Verify Token 由项目自行生成，应使用足够长的随机字符串。三项内容都属于密钥，不得直接写进 Git、Markdown、数据库渠道配置或聊天记录。

测试阶段可使用 Meta 临时 Access Token。生产环境应使用具有所需 WhatsApp 权限的长期系统用户令牌。

## 4. RemoteHelpDesk 密钥配置

> 推荐方式（`feat/whatsapp-channel-secrets` 分支，合并后生效）：在渠道编辑页「Meta 密钥」区域直接填写 Access Token、App Secret、Verify Token，保存后立即生效，系统加密保存且不再显示；无需设置 `RHD_PROJECT_SECRET_DIR`，也无需重启后端。下文的服务器密钥文件方式保留给生产部署，在「高级：使用服务器密钥文件」中填写引用。两者同时存在时页面填写的密钥优先。

项目密钥目录格式：

```text
<RHD_PROJECT_SECRET_DIR>/<tenantId>/<environment>/<secret-name>
```

例如渠道配置可以引用：

```text
meta-whatsapp-access-token
meta-whatsapp-app-secret
meta-whatsapp-verify-token
```

文件内容分别保存对应密钥原文。渠道页面只填写上述“密钥引用名称”，不要填写密钥值。

后端启动时需要指定密钥根目录和环境：

```powershell
$env:RHD_PROJECT_SECRET_DIR = "<项目密钥根目录>"
$env:RHD_PROJECT_ENVIRONMENT = "development"
go run ./cmd/server -config config/config.yaml
```

生产环境将 `development` 替换成实际生产环境名称，并使用生产服务器自己的密钥目录。

## 5. RemoteHelpDesk 渠道配置

进入渠道管理，新建或编辑 WhatsApp Cloud API 渠道。

| 页面字段 | 填写内容 |
| --- | --- |
| 渠道名称 | 可识别的业务名称 |
| 接待配置 | 对应的 AI Agent 或人工接待配置 |
| 项目标识 | 项目内部唯一标识 |
| Meta 电话号码 ID | Meta Phone Number ID |
| Meta 企业账户 ID | WABA ID |
| Meta 访问令牌 | 直接填写 Access Token（或在高级选项填写密钥文件引用） |
| Meta App Secret | 直接填写 App Secret（或高级选项引用） |
| 回调验证口令 | 直接填写 Verify Token（或高级选项引用），须与 Meta 后台「验证口令」一致 |
| 备注 | 环境、号码用途和负责人等非敏感说明 |

保存后检查渠道已关联正确租户和已启用的 AI Agent。

## 6. 配置公网 Webhook

RemoteHelpDesk Meta Webhook 路径为：

```text
https://<正式域名>/api/third/meta/whatsapp/webhook
```

本地联调可临时使用 Cloudflare Tunnel、ngrok 等 HTTPS 隧道。临时域名不能用于生产。

在 Meta 的 WhatsApp“生产设置 -> 配置 Webhook”中：

1. 将上述地址填写到“回调网址”；
2. 将项目生成的 Verify Token 填写到“验证口令”；
3. 点击“验证并保存”；
4. 验证成功后，在 Webhook 字段中找到 `messages`；
5. 打开 `messages` 订阅。

验证过程会对 Webhook 发起 GET 请求。系统校验 Verify Token 后，应原样返回 Meta 提供的 challenge。

真实 POST 请求还会使用 `X-Hub-Signature-256`。系统使用 App Secret 校验签名，签名错误的请求不会入库。

## 7. 绑定 WABA 到当前应用

只在界面看到 `messages 已订阅` 还不够。还必须确认 WABA 的 `subscribed_apps` 中包含当前 App ID。

使用具有 `whatsapp_business_management` 等所需权限的令牌调用：

```http
POST https://graph.facebook.com/<GRAPH_API_VERSION>/<WABA_ID>/subscribed_apps
Authorization: Bearer <ACCESS_TOKEN>
```

随后只读检查：

```http
GET https://graph.facebook.com/<GRAPH_API_VERSION>/<WABA_ID>/subscribed_apps
Authorization: Bearer <ACCESS_TOKEN>
```

返回结果必须包含当前 Meta App ID。

本次联调曾出现以下情况：Meta 测试页面可以看到手机消息，但 RemoteHelpDesk 完全收不到 POST。原因就是 WABA 当时只订阅了 Meta 内部测试应用，没有绑定当前应用。

## 8. 发布 Meta 应用

未发布应用只能接收 Meta 应用面板产生的测试 Webhook。手机发送的真实消息不会转发，即使发送者是应用管理员、开发者或测试者。

发布前至少需要：

1. 配置可公开访问的隐私政策 URL；
2. 检查 WhatsApp 用例；
3. 满足 Meta 发布页列出的必要项目；
4. 点击“发布”，确认状态变为“已发布”。

隐私政策必须是真实、稳定、公开可访问的页面，不能使用 Facebook 首页或临时隧道地址代替。

## 9. 入站验收

### 9.1 操作步骤

1. 手机打开与 Meta 测试号码或正式号码的 WhatsApp 对话；
2. 发送一条唯一文本，例如：

   ```text
   HELPDESK-E2E-<日期时间>
   ```

3. 检查后端收到：

   ```text
   POST /api/third/meta/whatsapp/webhook 200
   ```

4. 检查系统创建或复用对应 WhatsApp 会话；
5. 检查客户消息正文正确；
6. 检查消息保存了 Meta 外部消息 ID；
7. 使用相同 Meta 消息 ID 重放请求，确认消息不会重复入库。

### 9.2 通过标准

- Webhook 返回 HTTP 200；
- 创建或复用正确渠道下的会话；
- 客户消息正文一致；
- 同一外部消息 ID 只保存一次；
- 不跨租户创建会话或消息；
- 手机号不出现在普通日志、页面提示或测试报告中。

## 10. AI 和人工回复验收

客户消息入库后，系统可以触发已配置的 AI Agent 自动回复。

人工客服也可以在客服工作台对应会话底部输入框回复：

1. 输入普通文本；
2. 点击“发送”或按 Enter；
3. 系统创建 agent 消息；
4. 消息进入 WhatsApp 出站队列；
5. Meta Dispatcher 调用 Cloud API；
6. 手机收到消息；
7. Meta 回传发送状态。

出站验收应检查：

- Outbox `send_status=sent`；
- 存在 Meta `external_message_id`；
- `provider_status` 最终到达 `delivered`，条件允许时到达 `read`；
- `delivered_at` 和 `read_at` 正确写入；
- 重试不会重复创建业务消息。

人工点击“发送”会向真实 WhatsApp 用户发送消息，不是页面内模拟操作。

### 10.1 图片、文件、语音和视频验收

系统现已支持以下双向媒体链路：

| WhatsApp 消息 | 系统消息类型 | 出站 Meta 类型 |
| --- | --- | --- |
| 图片 | 图片 | `image` |
| 语音或音频 | 语音 | `audio` |
| PDF、Office 等文件 | 附件 | `document` |
| 视频 | 附件 | `video` |

入站时，系统先根据 Meta Media ID 获取媒体元数据和文件流，再放入私有隔离区完成大小限制、文件类型检测、恶意文件扫描和 SHA-256 校验。只有通过检查的文件才会发布为会话资产。重复的 Meta 消息 ID 不会重复下载或重复入库。

出站时，系统读取已扫描的会话资产，先上传到 Meta `/<PHONE_NUMBER_ID>/media` 获得 Media ID，再调用 `/<PHONE_NUMBER_ID>/messages` 发送。系统不会把内部文件 URL 暴露给 Meta。

真实环境验收步骤：

1. 从手机 WhatsApp 分别发送一张图片、一段语音、一个 PDF 文件和一个 MP4 视频；
2. 在 RemoteHelpDesk 中确认四条消息进入同一客户会话，均可预览或下载；
3. 重放其中一条 webhook，确认没有重复消息和重复资产；
4. 在客服工作台分别回复图片、语音、文件和视频；
5. 确认手机收到的类型、文件名和内容正确；
6. 确认对应 Outbox 进入 `sent`，随后收到 `delivered`，条件允许时收到 `read`；
7. 上传伪装扩展名、超限或扫描不通过的文件，确认系统拒绝发送且不会生成可用资产。

中文、英文及其他 Unicode 文本沿用文本消息链路，可正常双向传输；文件名也使用 UTF-8 保存。是否能发送某一种具体文件格式，还受系统上传安全策略和 Meta 支持格式、大小限制共同约束。

## 11. 24 小时窗口和模板消息

客户最后一次主动发送消息后的 24 小时内，可以发送普通客服文本。

超过 24 小时后，商家主动发起消息通常必须使用 Meta 已审核的消息模板。生产上线前需要：

1. 在 WhatsApp Manager 创建模板；
2. 等待 Meta 审核；
3. 在系统中实现或配置模板发送；
4. 验证模板参数、语言和发送状态。

## 12. 生产环境配置

生产环境不能直接依赖本地测试配置，至少需要：

- 稳定、受控的公网 HTTPS 域名；
- 正式 WhatsApp Business 电话号码；
- 正式 Phone Number ID 和 WABA ID；
- 已发布的 Meta 应用；
- WABA 已绑定当前应用；
- `messages` Webhook 已订阅；
- 长期系统用户 Access Token；
- 生产环境独立的 App Secret 和 Verify Token 密钥引用；
- 必要的企业验证、支付方式和消息模板；
- Webhook、Dispatcher、Outbox 和失败重试监控；
- 密钥轮换和访问审计方案。

生产发布后执行一次完整的“手机入站 -> 系统会话 -> 人工回复 -> 手机收到 -> delivered/read”验收。

## 13. 常见问题排查

### 13.1 Webhook 验证失败

检查：

- 公网 URL 是否能访问；
- Verify Token 是否与密钥存储一致；
- 后端是否使用了正确的租户和环境密钥目录；
- 回调路径是否完整；
- HTTPS 证书是否有效。

### 13.2 Meta 页面能看到消息，系统收不到 POST

依次检查：

1. 应用是否已发布；
2. `messages` 是否已订阅；
3. WABA 的 `subscribed_apps` 是否包含当前 App ID；
4. 回调 URL 是否仍然有效；
5. 隧道或反向代理是否仍在运行；
6. Meta 应用是否配置了正确的默认回调。

### 13.3 Webhook 返回 401

检查 App Secret、运行环境密钥引用以及 `X-Hub-Signature-256`。不要关闭签名校验来绕过问题。

### 13.4 Webhook 返回 400

检查：

- Phone Number ID 是否匹配渠道；
- WABA ID 是否匹配渠道；
- AI Agent 是否启用；
- Meta JSON 是否为支持的消息类型；
- 渠道是否处于可用状态。

### 13.5 入站成功但手机收不到回复

检查：

- Outbox 是否生成记录；
- Access Token 是否过期；
- Phone Number ID 是否正确；
- 是否超过 24 小时客服窗口；
- Meta 返回的错误码；
- Dispatcher 是否正在运行；
- WABA 是否需要支付方式或已审核模板。

## 14. 当前能力与限制

当前真实联调已经验证：

- 文本入站；
- 客户身份、会话和消息创建；
- Meta 消息 ID 幂等；
- AI 文本回复；
- 客服工作台文本回复的同一出站路径；
- `sent`、`delivered`、`read` 状态处理。

当前代码和自动化测试已经验证：

- 图片、文档、音频和视频 webhook 解析；
- 入站图片下载、安全扫描、会话绑定、SHA-256 校验和幂等处理；
- 图片、文档、音频和视频上传 Meta 后的消息结构；
- 客服媒体消息进入 Meta WhatsApp Outbox；
- Meta 通道消息不会被 Twilio Dispatcher 误发送。

当前尚未完成或需要继续验收：

- 使用当前真实 Meta 测试号码逐项验收图片、文件、语音和视频双向收发；
- 超过 24 小时后的模板消息工作流；
- 正式号码和生产域名验收；
- 长期令牌轮换和到期告警。

## 15. 安全要求

- 不在 Git、文档、工单或聊天中粘贴 Access Token、App Secret 和 Verify Token；
- 不在渠道 JSON 中保存密钥明文；
- 日志不得输出 Authorization Header、手机号或完整 Webhook Payload；
- 生产和开发使用不同密钥与环境目录；
- Access Token 采用最小权限并定期轮换；
- App Secret 曾被复制或意外暴露时应立即重置；
- 更换密钥后重新验证 Webhook、入站和出站链路。

## 16. 相关文档

- [WhatsApp 本地开发测试](./WhatsApp-本地开发测试.md)
- [WhatsApp Twilio Sandbox 接入说明](./WhatsApp-Twilio-Sandbox-接入说明.md)
- [WhatsApp 项目账号配置说明](./WhatsApp-Chatwoot-Cloud-API-账号配置说明.md)

本手册以 Meta Cloud API 真实联调为准。本地模拟和 Twilio Sandbox 只能验证各自链路，不能替代 Meta 正式接入验收。

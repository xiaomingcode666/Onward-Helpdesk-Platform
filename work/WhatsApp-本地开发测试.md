# WhatsApp 本地开发测试

本地测试不依赖 Meta 账号、WABA 或真实 WhatsApp 号码。它模拟一条 WhatsApp Cloud API 文本入站事件，复用平台现有的会话、消息、幂等和出站队列逻辑。

## 启用本地入口

仅在本机启动服务时设置：

```powershell
$env:RHD_WHATSAPP_LOCAL_TEST = "true"
$env:RHD_WHATSAPP_LOCAL_VERIFY_TOKEN = "local-whatsapp-test-token"
go run ./cmd/server -config config/config.yaml
```

入口默认关闭；没有 `RHD_WHATSAPP_LOCAL_TEST=true` 时不会注册路由。

验证 Meta 风格回调握手：

```powershell
Invoke-WebRequest `
  "http://127.0.0.1:8080/api/third/whatsapp/local/webhook?hub.mode=subscribe&hub.verify_token=local-whatsapp-test-token&hub.challenge=local-challenge" |
  Select-Object -ExpandProperty Content
```

应返回 `local-challenge`。

## 模拟客户入站消息

先在 Dashboard 的渠道详情中取得已启用 WhatsApp 渠道的 `Channel ID`，再发送：

```powershell
$payload = @{
  eventId = "local-event-001"
  channelId = "<Channel ID>"
  tenantId = 0
  from = "8613800000000"
  profileName = "本地访客"
  messageId = "wamid.local.001"
  text = "本地 WhatsApp 测试消息"
  locale = "zh-CN"
} | ConvertTo-Json

Invoke-RestMethod `
  -Method Post `
  -Uri "http://127.0.0.1:8080/api/third/whatsapp/local/webhook" `
  -ContentType "application/json" `
  -Body $payload
```

首次请求会创建或复用该外部访客的会话，并写入一条客户消息。重复发送同一个 `eventId` 和 `messageId` 不会重复建消息。

客服或 AI 回复会以 `channel_type=whatsapp` 的 pending 记录写入 `channel_message_outbox`，供本地检查；本地入口不会调用 Meta，也不会发送真实消息。

## 当前边界

- 只支持本地测试文本消息；不模拟媒体、模板、签名或 Meta 原始 JSON。
- `tenantId=0` 适合本地基础链路测试；真实租户联调需要提供有效租户上下文和已发布的默认 AI Agent。
- 这个入口验证平台内部链路，不代表 Meta 账号审核、Cloud API 凭据或生产 Webhook 已验收。

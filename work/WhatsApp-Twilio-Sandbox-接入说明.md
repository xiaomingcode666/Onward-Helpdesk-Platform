# WhatsApp Twilio Sandbox 接入 Helpdesk

Twilio Sandbox 已验证个人 WhatsApp 的收发。项目接入使用 Twilio 的表单 Webhook 和 Messages API，不需要 Meta 企业账号。

## 服务端环境变量

在运行 Helpdesk 的本地环境设置：

```powershell
$env:RHD_TWILIO_WHATSAPP_ENABLED = "true"
$env:RHD_TWILIO_ACCOUNT_SID = "ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
$env:RHD_TWILIO_AUTH_TOKEN = "<Twilio Auth Token>"
$env:RHD_TWILIO_WHATSAPP_FROM = "whatsapp:+17372508034"
$env:RHD_TWILIO_WHATSAPP_CHANNEL_ID = "<Helpdesk WhatsApp Channel ID>"
$env:RHD_TWILIO_WHATSAPP_TENANT_ID = "0"
$env:RHD_TWILIO_VALIDATE_SIGNATURE = "true"
$env:RHD_TWILIO_WHATSAPP_WEBHOOK_URL = "https://<公网域名>/api/third/twilio/whatsapp/webhook"
```

不要把 Auth Token 写入仓库或发到聊天中。`RHD_TWILIO_WHATSAPP_FROM` 使用 Twilio Sandbox 页面显示的号码；如果沙盒号码变化，以页面为准。

## 公网回调地址

本地服务需要公网 HTTPS 地址。开发时可以使用：

```powershell
ngrok http 8080
```

将生成的 HTTPS 地址拼接为：

```text
https://<公网域名>/api/third/twilio/whatsapp/webhook
```

在 Twilio Sandbox 的自定义 Webhook 设置中，将入站消息的回调方法设为 `POST`，填入该地址。

## 验收路径

1. 个人 WhatsApp 向 Twilio Sandbox 号码发送消息。
2. Twilio POST 到 Helpdesk 回调地址。
3. Helpdesk 按 `MessageSid` 幂等创建 WhatsApp 会话和客户消息。
4. 客服在 Helpdesk 回复，消息进入 WhatsApp 出站队列。
5. 后台每 5 秒调用 Twilio Messages API，手机收到客服回复。

如果 Twilio 收到回调但 Helpdesk 返回 401，检查公网 URL、`RHD_TWILIO_WHATSAPP_WEBHOOK_URL` 和 Auth Token；如果返回 500，检查渠道 ID、已启用的 AI Agent 及服务日志。

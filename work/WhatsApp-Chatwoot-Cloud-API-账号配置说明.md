# WhatsApp 项目账号配置说明

系统现在支持在“Dashboard → Channels → New Channel”创建 `WhatsApp Cloud API` 渠道。渠道表单可直接填写 Meta Access Token、App Secret、Webhook Verify Token，系统使用平台加密密钥（`encryptionKey`）加密保存，不回显、不写入接口响应与日志；也可继续使用服务器密钥文件引用。（2026-10-09 用户确认 DCH-001 口径变更；`feat/whatsapp-channel-secrets` 分支，合并后生效）

每个项目应部署独立的 Chatwoot Community Core，并为该项目创建独立的 Chatwoot Account/Inbox；消息通道使用官方 Meta WhatsApp Cloud API。需要先在 Meta Business/WhatsApp Manager 完成业务号码、WABA、Phone Number ID 和 Cloud API 应用配置，再将对应标识和部署密钥引用填入渠道表单。

新建 WhatsApp 渠道始终为“禁用”。服务端启用接口会再次校验以下条件，前端勾选不能绕过校验：

- Project Configuration 已批准；
- 商业责任已批准；
- Readiness 已批准；
- 项目标识、Chatwoot Core/Account/Inbox、Meta Phone Number ID/WABA ID 已填写，三项 Meta 密钥均已在页面填写或配置了密钥文件引用。

因此，当前代码完成了账号配置和安全启用门禁；真实 Meta 业务账号、号码审核、Chatwoot 实例部署及生产 Webhook 联调仍需在目标环境完成后再启用渠道。

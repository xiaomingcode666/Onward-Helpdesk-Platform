# WhatsApp 渠道密钥页面配置 —— 验证记录

> 日期：2026-10-09
> 关联需求：DCH-001（口径变更：渠道可接收 Meta 密钥并加密存储，2026-10-09 用户确认）
> 需求状态：开发完成，待用户验收；**在线状态待同步**（在线需求表本次无法读取/编辑，未修改）

## 代码

- 分支 `feat/whatsapp-channel-secrets`，已在本地快进合并到 `main`，HEAD `3bcf85a`；**未推送**。
- 提交：95dce6e、5811db9、7e88bd6、dbd6e22、38f0d3c、8a84d4a、bee752e、3bcf85a。
- 设计与计划（本地，不提交）：`docs/superpowers/specs/2026-10-09-whatsapp-channel-secrets-design.md`、`docs/superpowers/plans/2026-10-09-whatsapp-channel-secrets.md`。

## 本地 CI 等价验证（HEAD 3bcf85a，Linux 容器，UTC，git bundle 干净检出）

| 检查 | 结果 |
| --- | --- |
| gofmt / go vet | 通过 |
| 后端测试 core + 0–7 共 9 组（-race、coverage，隔离 PostgreSQL 16 / Redis 7） | 全部通过 |
| govulncheck v1.6.0 | **失败**：GO-2026-6629（golang.org/x/text v0.40.0）、GO-2026-6617（golang.org/x/net v0.57.0）；go.mod 未改动，main 既有问题 |
| 前端 install / typecheck / lint / build:sdk + build（Node 22.23.3，pnpm 10.30.2） | 通过 |
| pnpm audit --prod --audit-level high | **失败**：13 项（3 critical：next <16.3.6、@capacitor/ios、@capacitor/android <8.5.1）；依赖文件未改动，main 既有问题 |
| gitleaks v8.24.3（全部历史） | 17 项，全部位于初始导入提交 c123d06，本分支提交无发现 |
| P0 浏览器验收（test:e2e:p0） | **未执行** |
| 新增 Playwright `channel-whatsapp-secrets.spec.ts`（Windows 本机） | 2 通过；用旧版编辑页验证会失败 |

推送阻塞项：govulncheck、pnpm audit 失败（需单独升级依赖，待用户决定）；P0 验收未执行。

## 真实联调（渠道 #3，未设置 RHD_PROJECT_SECRET_DIR）

- 页面填写三项密钥，`t_channel_secret` 存 `enc:v1:` 密文；审计仅含名称与指纹末 4 位。
- 隧道 GET 验证返回 200。
- 手机「你好」→ 会话 #52（租户 14）→ AI 回复 → Outbox #20 `sent` → `read`。

## 代码审查（独立审查）

- 结论：With fixes；0 Critical。
- 已修复：跨租户可覆盖渠道密钥（测试 `TestChannelSecretsRejectedForOperatorFromAnotherTenant` 先失败后通过）。
- 待后续处理的小问题 11 项：密钥无法解密时仍显示“已设置”；格式错误提示为英文、前端未预校验；零宽字符未拒绝；删除渠道/改类型清除密钥时无审计；数据库读取错误被吞掉；事务内用全局连接读取 Agent；指纹未加盐；未配置 encryptionKey 时仍允许保存；“页面优先”提示不完整；updatedAt 无时区；回滚旧版本需在运维手册说明。

## 发现的既有问题（非本次改动）

- 渠道列表/详情/修改接口未按租户隔离。
- 允许两个启用渠道使用同一 Meta 电话号码 ID，入站按 ID 最小者路由。
- 新会话欢迎语为英文。

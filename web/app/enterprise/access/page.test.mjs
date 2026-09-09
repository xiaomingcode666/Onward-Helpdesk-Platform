import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")

test("enterprise access page keeps terse dialog and empty-state copy", () => {
  assert.match(source, /function AccessState/)
  assert.match(source, /<AccessState title="无通道管理权限" \/>/)
  assert.match(source, /<AccessState title="暂无业务系统连接器"/)
  assert.doesNotMatch(source, /DialogDescription/)
  assert.doesNotMatch(source, /连接器凭据将加密保存/)
  assert.doesNotMatch(source, /接口响应不会返回凭据明文/)
  assert.doesNotMatch(source, /使用企业自建应用验证身份/)
  assert.doesNotMatch(source, /保存后将调用飞书官方租户令牌接口/)
  assert.doesNotMatch(source, /当前角色可以查看系统接入/)
  assert.doesNotMatch(source, /当前租户尚未接入 ERP/)
  assert.doesNotMatch(source, /SMTP · 工单、SLA、会议与系统事件/)
  assert.doesNotMatch(source, /ERP、CRM、设备云、仓储与供应商系统/)
})

test("enterprise access page loading states stay local to modules", () => {
  assert.match(source, /Promise\.allSettled\(\[/)
  assert.match(source, /function MailFormSkeleton/)
  assert.match(source, /function ConnectorSkeleton/)
  assert.match(source, /aria-label="发件通道加载中"/)
  assert.match(source, /aria-label="连接器加载中"/)
  assert.match(source, /role="status"/)
  assert.match(source, /aria-busy="true"/)
  assert.match(source, /mailError \? \(\s*<ErrorState title="邮件通道加载失败"/)
  assert.doesNotMatch(source, /\) : mailError \? \(\s*<ErrorState title="邮件通道加载失败"/)
  assert.doesNotMatch(source, /RouteLoadingPage/)
})

import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")

test("enterprise access page keeps terse dialog and empty-state copy", () => {
  assert.doesNotMatch(source, /\p{Script=Han}/u)
  assert.match(source, /function AccessState/)
  assert.match(source, /<AccessState title=\{ee\("access\.text058"\)\} \/>/)
  assert.match(source, /<AccessState title=\{ee\("access\.text102"\)\}/)
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
  assert.match(source, /aria-label=\{ee\("access\.text167"\)\}/)
  assert.match(source, /aria-label=\{ee\("access\.text168"\)\}/)
  assert.match(source, /role="status"/)
  assert.match(source, /aria-busy="true"/)
  assert.match(source, /mailError \? \(\s*<ErrorState title=\{ee\("access\.text062"\)\}/)
  assert.doesNotMatch(source, /\) : mailError \? \(\s*<ErrorState title=\{ee\("access\.text062"\)\}/)
  assert.doesNotMatch(source, /RouteLoadingPage/)
})

test("enterprise access tabs refresh labels when the locale changes", () => {
  assert.match(source, /const \{ t \} = useAppLocale\(\)/)
  assert.match(source, /t\("enterpriseExtract\.access\.text013"\)/)
  assert.match(source, /\], \[businessConnectors\.length, t\]\)/)
})

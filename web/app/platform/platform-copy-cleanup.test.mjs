import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const pages = {
  overview: await readFile(new URL("./page.tsx", import.meta.url), "utf8"),
  accessGovernance: await readFile(new URL("./access-governance/page.tsx", import.meta.url), "utf8"),
  globalization: await readFile(new URL("./globalization/page.tsx", import.meta.url), "utf8"),
  ops: await readFile(new URL("./ops/page.tsx", import.meta.url), "utf8"),
  tenants: await readFile(new URL("./tenants/page.tsx", import.meta.url), "utf8"),
  models: await readFile(new URL("./models/page.tsx", import.meta.url), "utf8"),
  usage: await readFile(new URL("./usage/page.tsx", import.meta.url), "utf8"),
}
const integrationDialog = await readFile(new URL("../../components/platform/platform-integration-config-dialog.tsx", import.meta.url), "utf8")
const platformLiveUi = await readFile(new URL("../../components/platform/platform-live-ui.tsx", import.meta.url), "utf8")
const platformPageHeaderSource = platformLiveUi.match(/export function PlatformPageHeader[\s\S]*?\n}\n\nconst metricTones/)?.[0] || ""
const localeMessages = await Promise.all(
  ["zh-CN", "en-US", "es-ES"].map(async (locale) => ({
    locale,
    messages: JSON.parse(await readFile(new URL(`../../messages/${locale}.json`, import.meta.url), "utf8")),
  }))
)

test("platform core pages avoid leftover explanatory descriptions", () => {
  for (const [name, source] of Object.entries(pages)) {
    assert.doesNotMatch(source, /DialogDescription/, `${name} should not render dialog descriptions`)
    assert.doesNotMatch(source, /description=""/, `${name} should not pass empty descriptions`)
  }
  assert.doesNotMatch(integrationDialog, /description=\{meta\.description\}/)
  assert.doesNotMatch(platformPageHeaderSource, /description\?: string/)
  assert.doesNotMatch(platformPageHeaderSource, /generatedAt\?: string/)
  assert.doesNotMatch(platformPageHeaderSource, /headerDescription|updateLabel/)
  assert.doesNotMatch(platformLiveUi, /description\?: string/)
  assert.doesNotMatch(platformLiveUi, /\{description \?/)
  assert.doesNotMatch(pages.ops, /function EmptyState\(\{ title, description \}/)
  assert.doesNotMatch(pages.overview, /description\?: string/)
  assert.doesNotMatch(pages.overview, /\{description \?/)
  assert.doesNotMatch(pages.overview, /if \(error && !data\)/)
  assert.doesNotMatch(pages.overview, /if \(error && ops === null\)/)
  assert.match(pages.overview, /PanelError message=\{error\} onRetry=\{onRetry\}/)

  assert.match(pages.overview, /title="租户规模"/)
  assert.match(pages.overview, /title="知识与诊断"/)
  assert.match(pages.overview, /title="近 7 天模型调用"/)
  assert.match(pages.overview, /label="模型请求"/)
  assert.doesNotMatch(pages.overview, /Top 5 租户设备 \/ AI 使用规模/)
  assert.doesNotMatch(pages.overview, /AI 使用规模/)
  assert.doesNotMatch(pages.overview, /知识库与 AI 诊断/)
  assert.doesNotMatch(pages.overview, /近 7 天 AI 服务调用趋势/)
  assert.doesNotMatch(pages.overview, /AI 请求/)
  assert.doesNotMatch(pages.overview, /\} Tokens</)
  assert.doesNotMatch(pages.overview, /按设备量排序/)
  assert.doesNotMatch(pages.overview, /平台工单存量/)
  assert.doesNotMatch(pages.overview, /知识沉淀/)
  assert.doesNotMatch(pages.overview, /按平台计量事件/)
  assert.doesNotMatch(pages.overview, /核心运行时/)
  assert.doesNotMatch(pages.overview, /记录最近发生/)
  assert.doesNotMatch(pages.overview, /集中查看需要处理/)
  assert.doesNotMatch(pages.overview, /需要尽快跟进|当前没有超时事项|等待审核或索引|检查连接器运行状态/)

  assert.doesNotMatch(pages.accessGovernance, /外部服务配置、连接测试/)
  assert.doesNotMatch(pages.accessGovernance, /共享 Provider/)
  assert.doesNotMatch(pages.accessGovernance, /租户连接器和最近调用记录/)
  assert.doesNotMatch(pages.accessGovernance, /租户账号和产品级 API Key/)
  assert.match(integrationDialog, />内置转写</)
  assert.doesNotMatch(integrationDialog, /转写 Provider/)
  assert.doesNotMatch(integrationDialog, />Mock</)

  assert.doesNotMatch(pages.globalization, /platformGlobalization\.pageDescription/)
  assert.doesNotMatch(pages.globalization, /platformGlobalization\.regionsDescription/)
  assert.doesNotMatch(pages.globalization, /platformGlobalization\.emptyRegionDescription/)
  assert.doesNotMatch(pages.globalization, /platformGlobalization\.tenantDetailsDescription/)
  assert.doesNotMatch(pages.globalization, /platformGlobalization\.dialogDescription/)
  for (const { locale, messages } of localeMessages) {
    const globalization = messages.platformGlobalization
    assert.equal(Object.hasOwn(globalization, "pageDescription"), false, `${locale} should not keep platformGlobalization.pageDescription`)
    assert.equal(Object.hasOwn(globalization, "regionsDescription"), false, `${locale} should not keep platformGlobalization.regionsDescription`)
    assert.equal(Object.hasOwn(globalization, "emptyRegionDescription"), false, `${locale} should not keep platformGlobalization.emptyRegionDescription`)
    assert.equal(Object.hasOwn(globalization, "tenantDetailsDescription"), false, `${locale} should not keep platformGlobalization.tenantDetailsDescription`)
    assert.equal(Object.hasOwn(globalization, "dialogDescription"), false, `${locale} should not keep platformGlobalization.dialogDescription`)
  }

  assert.doesNotMatch(pages.ops, /基础设施探测接口未返回数据/)
  assert.doesNotMatch(pages.ops, /数据库未返回物理占用/)
  assert.doesNotMatch(pages.ops, /当前基础设施、队列与接入链路运行正常/)
  assert.match(pages.ops, /title="模型与接入状态"/)
  assert.match(pages.ops, /label="模型与接入状态加载中"/)
  assert.match(pages.ops, /label="模型接入账号"/)
  assert.match(pages.ops, /label="模型账号"/)
  assert.match(pages.ops, /product_ai_usage_events: "模型用量事件"/)
  assert.doesNotMatch(pages.ops, /AI 与接入状态/)
  assert.doesNotMatch(pages.ops, /AI 账号/)
  assert.doesNotMatch(pages.ops, /AI 接入账号/)
  assert.doesNotMatch(pages.ops, /AI 用量事件/)
  assert.doesNotMatch(pages.ops, /BotIcon/)
  assert.doesNotMatch(pages.usage, /BotIcon/)

  assert.doesNotMatch(pages.tenants, /企业租户、订阅套餐与当月计量状态/)
  assert.match(pages.tenants, /本月模型调用/)
  assert.doesNotMatch(pages.tenants, /本月 AI 调用/)
  assert.doesNotMatch(pages.tenants, /创建企业租户、默认角色/)
  assert.doesNotMatch(pages.tenants, /该账号创建后从企业端登录/)
  assert.doesNotMatch(pages.tenants, /当前筛选结果同步中/)
  assert.doesNotMatch(pages.tenants, /调整筛选条件或新建企业租户/)
  assert.doesNotMatch(pages.tenants, /配置了试用到期日/)

  assert.doesNotMatch(pages.models, /租户模型服务、访问凭证与使用计量/)
  assert.doesNotMatch(pages.models, /配置平台模型网关地址/)
  assert.doesNotMatch(pages.models, /当前租户：/)
  assert.match(pages.models, /usageInitialLoading/)
  assert.match(pages.models, /billingInitialLoading/)
  assert.match(pages.models, /label="用量统计加载中"/)
  assert.match(pages.models, /label="模型用量加载中"/)
  assert.match(pages.models, /label="余额加载中"/)
  assert.match(pages.models, /label="充值记录加载中"/)
})

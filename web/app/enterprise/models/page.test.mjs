import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")
const locales = ["zh-CN", "en-US", "es-ES"]
const messagesByLocale = await Promise.all(
  locales.map(async (locale) => [
    locale,
    JSON.parse(await readFile(new URL(`../../../messages/${locale}.json`, import.meta.url), "utf8")),
  ]),
)

test("enterprise models page uses shared i18n copy", () => {
  assert.match(source, /useI18n\(\)/)
  assert.match(source, /enterpriseModels\.pageTitle/)
  assert.match(source, /enterpriseModels\.apiKeys\.title/)
  assert.match(source, /subscriptionDurationLabel\(t, activeSubscription\)/)
  assert.match(source, /statusLabel\(t, key\.status\)/)
  assert.doesNotMatch(source, /企业 AI 配置数据加载失败|AI 模型用量|每日使用趋势|账户状态|API Key 使用情况/)
})

test("enterprise models page prefers product names for product API keys", () => {
  assert.match(source, /getEnterpriseAIKeyDisplayName/)
  assert.match(source, /getEnterpriseAIKeyProductMeta\(key\)/)
  assert.match(source, /getEnterpriseAIKeyDisplayName\(key,/)
  assert.match(source, /getEnterpriseAIKeyDisplayName\(editingKey,/)
})

test("enterprise models empty states stay concise", () => {
  assert.doesNotMatch(source, /enterpriseModels\.(trend|apiKeys|platformStats)\.emptyDescription/)
  assert.doesNotMatch(source, /<PlatformEmpty[^>]+description=\{t\("enterpriseModels\.(trend|apiKeys|platformStats)\.emptyDescription"\)\}/)

  for (const [locale, messages] of messagesByLocale) {
    const enterpriseModels = messages.enterpriseModels
    assert.ok(enterpriseModels, `${locale} should define enterpriseModels copy`)
    assert.equal(Object.hasOwn(enterpriseModels.trend, "emptyDescription"), false, `${locale} trend empty description should be removed`)
    assert.equal(Object.hasOwn(enterpriseModels.apiKeys, "emptyDescription"), false, `${locale} api key empty description should be removed`)
    assert.equal(Object.hasOwn(enterpriseModels.platformStats, "emptyDescription"), false, `${locale} platform stats empty description should be removed`)
  }
})

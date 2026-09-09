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

test("enterprise products page uses shared productCenter copy end to end", () => {
  assert.match(source, /useI18n\(\)/)
  assert.match(source, /productCenter\.pageTitle/)
  assert.doesNotMatch(source, /description=\{t\("productCenter\.pageDescription"\)\}/)
  assert.match(source, /productCenter\.page\.directoryTitle/)
  assert.match(source, /productCenter\.page\.resourceModalTitle/)
  assert.match(source, /productCenter\.page\.errors\.loadProfileFailed/)
  assert.doesNotMatch(source, /productCenter\.page\.resourceStatusDescription/)
  assert.match(source, /productCenter\.product\.createTitle/)
  assert.match(source, /productCenter\.product\.editTitle/)
  assert.match(source, /getStatusZh\(t, product\.status\)/)
  assert.match(source, /getResourceStatusZh\(t, resources\?\.ai_key\?\.status\)/)
  assert.match(source, /getResourceStatusZh\(t, resources\?\.status\)/)
  assert.match(source, /profileLoaded: boolean/)
  assert.match(source, /const knowledgeLoading = loading && !profileLoaded/)
  assert.match(source, /PRODUCT_OWNER_OPTION_PAGE_SIZE = 50/)
  assert.match(source, /function OwnerMemberOptions/)
  assert.match(source, /currentOwnerId=\{product\.owner_member_id\}/)
  assert.doesNotMatch(source, /fetchEnterpriseIAMMembers\(\{ page: 1, limit: 500 \}/)
  assert.doesNotMatch(source, /BotIcon/)
  assert.doesNotMatch(source, /loading\s+&&\s+profileForProduct === null/)
  assert.doesNotMatch(source, /loading\s+&&\s+!profileForProduct/)
  assert.doesNotMatch(source, /新建产品|产品中心|产品目录|售后档案|设备列表|只读/)

  for (const [locale, messages] of messagesByLocale) {
    assert.equal(Object.hasOwn(messages.productCenter.page, "resourceStatusDescription"), false, `${locale} should not keep product resource status descriptions`)
    assert.equal(Object.hasOwn(messages.knowledge, "emptyDescription"), false, `${locale} should not keep knowledge empty descriptions`)
  }
})

test("enterprise products overview omits the selected product focus block", () => {
  assert.match(source, /<ProductPortfolioOverview products=\{products\} \/>/)
  assert.match(source, /product-portfolio-overview rhd-railops-product-overview/)
  assert.match(source, /product-portfolio-metric rhd-railops-product-metric/)
  assert.doesNotMatch(source, /product-portfolio-focus/)
  assert.doesNotMatch(source, /selectedProduct\?\.name \|\| t\("productCenter\.page\.overview\.none"\)/)
})

test("enterprise products page uses the shared product domain layout", () => {
  assert.match(source, /<section className="ent-product-domain-layout">/)
  assert.doesNotMatch(source, /className="grid gap-1\.5 xl:hidden"/)
  assert.doesNotMatch(source, /className="hidden space-y-3 xl:block"/)
})

import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const sourceEntries = [
  ["models", new URL("./[productId]/models-tab.tsx", import.meta.url)],
  ["modules", new URL("./[productId]/modules-tab.tsx", import.meta.url)],
  ["manuals", new URL("./[productId]/manuals-tab.tsx", import.meta.url)],
  ["faultStats", new URL("./[productId]/fault-stats-tab.tsx", import.meta.url)],
  ["resourceTable", new URL("./[productId]/_components/product-resource-table.tsx", import.meta.url)],
  ["knowledgeTab", new URL("./[productId]/_components/product-knowledge-tab.tsx", import.meta.url)],
]
const sources = await Promise.all(sourceEntries.map(async ([name, url]) => [name, await readFile(url, "utf8")]))
const locales = ["zh-CN", "en-US", "es-ES"]
const messagesByLocale = await Promise.all(
  locales.map(async (locale) => [
    locale,
    JSON.parse(await readFile(new URL(`../../../messages/${locale}.json`, import.meta.url), "utf8")),
  ]),
)

test("product center detail empty states stay concise", () => {
  for (const [name, source] of sources) {
    assert.doesNotMatch(source, /description=\{t\("productCenter\.(model|modules|manuals)\./, `${name} should not pass productCenter empty descriptions`)
    assert.doesNotMatch(source, /description=\{canManageManuals \? t\("productCenter\.manuals\.emptyDescriptionManage"\)/, `${name} should not branch empty descriptions`)
    assert.doesNotMatch(source, /description=\{t\("productArchive\.(emptyDescription|faultStats\.emptyDescription)"\)\}/, `${name} should not pass productArchive empty descriptions`)
  }

  for (const [locale, messages] of messagesByLocale) {
    const productArchive = messages.productArchive
    const productCenter = messages.productCenter
    assert.ok(productArchive, `${locale} should define productArchive copy`)
    assert.ok(productCenter, `${locale} should define productCenter copy`)

    assert.equal(Object.hasOwn(productArchive, "emptyDescription"), false, `${locale} should not keep productArchive.emptyDescription`)
    assert.equal(Object.hasOwn(productArchive.faultStats, "emptyDescription"), false, `${locale} should not keep productArchive.faultStats.emptyDescription`)
    assert.equal(Object.hasOwn(productCenter.page, "emptyDescription"), false, `${locale} should not keep productCenter.page.emptyDescription`)
    assert.equal(Object.hasOwn(productCenter.page, "emptyDescriptionCreate"), false, `${locale} should not keep productCenter.page.emptyDescriptionCreate`)
    assert.equal(Object.hasOwn(productCenter.model, "emptyDescription"), false, `${locale} should not keep productCenter.model.emptyDescription`)
    assert.equal(Object.hasOwn(productCenter.manuals, "emptyDescriptionManage"), false, `${locale} should not keep productCenter.manuals.emptyDescriptionManage`)
    assert.equal(Object.hasOwn(productCenter.manuals, "emptyDescriptionView"), false, `${locale} should not keep productCenter.manuals.emptyDescriptionView`)
    assert.equal(Object.hasOwn(productCenter.modules, "emptyDescription"), false, `${locale} should not keep productCenter.modules.emptyDescription`)
    assert.equal(Object.hasOwn(productCenter.modules, "scopeEmptyDescription"), false, `${locale} should not keep productCenter.modules.scopeEmptyDescription`)
  }
})

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

test("enterprise diagnosis page keeps breadcrumbs and empty states concise", () => {
  assert.match(source, /<PageHeader title=\{t\("enterpriseDiagnosis\.pageTitle"\)\} \/>/)
  assert.doesNotMatch(source, /enterpriseDiagnosis\.pageEyebrow/)
  assert.doesNotMatch(source, /enterpriseDiagnosis\.(noMatchNodesDescription|noTreeDescription|noProductsDescription)/)
  assert.doesNotMatch(source, /<EmptyState[^>]+description=\{nodes\.length/)
  assert.doesNotMatch(source, /<EmptyState[^>]+description=\{t\("enterpriseDiagnosis\.noProductsDescription"\)\}/)
  assert.doesNotMatch(source, /DialogDescription/)
  assert.doesNotMatch(source, /enterpriseDiagnosis\.dialog\.description(WithProduct|WithoutProduct)/)

  for (const [locale, messages] of messagesByLocale) {
    const enterpriseDiagnosis = messages.enterpriseDiagnosis
    assert.ok(enterpriseDiagnosis, `${locale} should define enterpriseDiagnosis copy`)
    assert.equal(Object.hasOwn(enterpriseDiagnosis, "pageEyebrow"), false, `${locale} should not keep enterpriseDiagnosis.pageEyebrow`)
    assert.equal(Object.hasOwn(enterpriseDiagnosis, "noMatchNodesDescription"), false, `${locale} should not keep no-match empty description`)
    assert.equal(Object.hasOwn(enterpriseDiagnosis, "noTreeDescription"), false, `${locale} should not keep no-tree empty description`)
    assert.equal(Object.hasOwn(enterpriseDiagnosis, "noProductsDescription"), false, `${locale} should not keep no-products empty description`)
    assert.equal(Object.hasOwn(enterpriseDiagnosis.dialog, "descriptionWithProduct"), false, `${locale} should not keep dialog product description`)
    assert.equal(Object.hasOwn(enterpriseDiagnosis.dialog, "descriptionWithoutProduct"), false, `${locale} should not keep dialog fallback description`)
  }
})

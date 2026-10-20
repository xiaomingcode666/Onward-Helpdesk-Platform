import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import vm from "node:vm"
import ts from "typescript"

function compile(source, dependencies = {}) {
  const compiled = ts.transpileModule(source, {
    compilerOptions: { target: ts.ScriptTarget.ES2017, module: ts.ModuleKind.CommonJS, esModuleInterop: true },
  })
  const sandbox = { exports: {}, require: (name) => {
    assert.ok(name in dependencies, `Unexpected import: ${name}`)
    return dependencies[name]
  } }
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.exports
}

const config = compile(await readFile(new URL("../i18n/config.ts", import.meta.url), "utf8"))
const catalogs = Object.fromEntries(await Promise.all(config.SUPPORTED_LOCALES.map(async (locale) => [
  locale, JSON.parse(await readFile(new URL(`../messages/${locale}.json`, import.meta.url), "utf8")),
])))
const messages = compile(await readFile(new URL("../i18n/messages.ts", import.meta.url), "utf8"), {
  "@/i18n/config": config,
  "@/i18n/extracted": { getExtractedMessage: () => undefined },
  ...Object.fromEntries(Object.entries(catalogs).map(([locale, catalog]) => [`@/messages/${locale}.json`, catalog])),
})
const { localizeTicketQualityEvidence } = compile(await readFile(new URL("./ticket-quality-i18n.ts", import.meta.url), "utf8"), {
  "@/i18n/config": config,
  "@/i18n/messages": messages,
})

const screenshotEvidence = [
  ["identity_confirmation", "identity_complete"],
  ["information_completeness", "information_unverified"],
  ["issue_classification", "classification_complete"],
  ["priority", "priority_complete"],
  ["communication", "communication_partial"],
  ["data_protection", "data_protection_review"],
  ["handoff", "handoff_complete"],
  ["follow_up", "follow_up_partial"],
  ["closure", "closure_complete"],
]
function evidenceLine(locale, criterion, evidence) {
  const review = catalogs[locale].ticketQuality.review
  return `${review.criteria[criterion]}: ${review.evidence[evidence]}`
}

test("historical screenshot evidence switches both ways without reassessing the ticket", () => {
  const chinese = screenshotEvidence.map(([criterion, evidence]) => evidenceLine("zh-CN", criterion, evidence)).join("\n")
  const english = screenshotEvidence.map(([criterion, evidence]) => evidenceLine("en-US", criterion, evidence)).join("\n")
  assert.equal(localizeTicketQualityEvidence(chinese, "en-US"), english)
  assert.equal(localizeTicketQualityEvidence(english, "zh-CN"), chinese)
  assert.equal(localizeTicketQualityEvidence(chinese, "zh-CN"), chinese)
})

test("every built-in evidence variant can be translated from all supported locales", () => {
  const evidenceKeys = Object.keys(catalogs["zh-CN"].ticketQuality.review.evidence)
  const criterionFor = (key) => key.startsWith("identity_") ? "identity_confirmation"
    : key.startsWith("information_") ? "information_completeness"
    : key.startsWith("classification_") ? "issue_classification"
    : key.startsWith("manual_") ? "closure"
    : screenshotEvidence.find(([, evidence]) => evidence.split("_").slice(0, -1).join("_") === key.split("_").slice(0, -1).join("_"))?.[0]
  for (const key of evidenceKeys) {
    const criterion = criterionFor(key)
    assert.ok(criterion, `Missing criterion for ${key}`)
    for (const locale of config.SUPPORTED_LOCALES) {
      const original = evidenceLine(locale, criterion, key)
      assert.equal(localizeTicketQualityEvidence(original, "en-US"), evidenceLine("en-US", criterion, key))
    }
  }
})

test("manual text, added details, unknown rules, and line breaks remain intact", () => {
  const manual = "客户补充说明：重启后恢复\r\n\r\n沟通表达: 人工核实电话记录\r\n关闭情况: 已解决并记录处理结论，附件见报告\r\n自定义项: 已记录标准工单分类\r\n"
  assert.equal(localizeTicketQualityEvidence(manual, "en-US"), manual)
  const generated = evidenceLine("zh-CN", "closure", "closure_complete")
  assert.equal(localizeTicketQualityEvidence(`${generated}\r\n${manual}`, "en-US"), `${evidenceLine("en-US", "closure", "closure_complete")}\r\n${manual}`)
  assert.equal(localizeTicketQualityEvidence("关闭情况：已解决并记录处理结论", "en-US"), evidenceLine("en-US", "closure", "closure_complete"))
  assert.equal(localizeTicketQualityEvidence(undefined, "en-US"), "")
})

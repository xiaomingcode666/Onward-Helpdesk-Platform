import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import vm from "node:vm"
import ts from "typescript"

const source = await readFile(new URL("./customer-portal-metrics-i18n.ts", import.meta.url), "utf8")
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS },
})
const sandbox = { exports: {} }
vm.runInNewContext(compiled.outputText, sandbox)
const { localizeCustomerPortalMetric } = sandbox.exports
const metrics = [
  { key: "devices", label: "我的设备", meta: "已绑定范围" },
  { key: "conversations", label: "当前会话", meta: "继续沟通" },
  { key: "tickets", label: "处理中工单", meta: "待我处理提醒" },
  { key: "meetings", label: "待加入会议", meta: "视频协作" },
].map((metric) => ({ ...metric, value: "3", tone: "green" }))

for (const locale of ["en-US", "zh-CN", "es-ES"]) {
  const messages = JSON.parse(await readFile(new URL(`../messages/${locale}.json`, import.meta.url), "utf8"))
  const t = (key) => {
    const value = key.split(".").reduce((value, segment) => value?.[segment], messages)
    assert.equal(typeof value, "string", `${locale}: missing ${key}`)
    return value
  }
  test(`customer metric labels follow ${locale} while retaining counts and API data`, () => {
    for (const metric of metrics) {
      const localized = localizeCustomerPortalMetric(t, metric)
      assert.equal(localized.value, metric.value)
      assert.equal(localized.tone, metric.tone)
      assert.equal(localized.key, metric.key)
      assert.equal(localized.label, messages.customerMetrics[metric.key].label)
      assert.equal(localized.meta, messages.customerMetrics[metric.key].meta)
      if (locale === "zh-CN") {
        assert.equal(localized.label, metric.label)
        assert.equal(localized.meta, metric.meta)
      } else {
        assert.doesNotMatch(localized.label + localized.meta, /\p{Script=Han}/u)
      }
    }
    assert.equal(metrics[1].label, "当前会话", "localization must not overwrite the API response")
  })
}

test("unknown customer metrics retain their supplied labels and metadata", () => {
  const metric = { key: "custom", label: "Custom KPI", meta: "Custom detail", value: "9", tone: "blue" }
  assert.equal(localizeCustomerPortalMetric(() => assert.fail("unexpected translation"), metric), metric)
})

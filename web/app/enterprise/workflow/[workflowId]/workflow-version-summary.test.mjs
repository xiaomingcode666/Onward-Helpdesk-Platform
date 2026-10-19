import assert from "node:assert/strict"
import { existsSync, readFileSync, readdirSync, statSync } from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"
import test from "node:test"
import vm from "node:vm"
import ts from "typescript"

const webRoot = fileURLToPath(new URL("../../../../", import.meta.url))
const cache = new Map()
function loadModule(filename) {
  if (cache.has(filename)) return cache.get(filename)
  const source = readFileSync(filename, "utf8")
  if (filename.endsWith(".json")) return JSON.parse(source)
  const exports = {}
  cache.set(filename, exports)
  const compiled = ts.transpileModule(source, {
    compilerOptions: { target: ts.ScriptTarget.ES2020, module: ts.ModuleKind.CommonJS, esModuleInterop: true },
  })
  vm.runInNewContext(compiled.outputText, {
    exports,
    require: (id) => {
      const target = id.startsWith("@/") ? path.join(webRoot, id.slice(2)) : path.resolve(path.dirname(filename), id)
      return loadModule([target, `${target}.ts`, path.join(target, "index.ts")].find(candidate => existsSync(candidate) && statSync(candidate).isFile()) ?? target)
    },
  }, { filename })
  return exports
}

const { translateMessage } = loadModule(path.join(webRoot, "i18n/messages.ts"))
const { localizeWorkflowVersionSummary } = loadModule(fileURLToPath(new URL("./workflow-product-utils.ts", import.meta.url)))
const translate = (locale) => (key, values) => translateMessage(locale, key, values)

test("version descriptions follow the current language when switching both ways", () => {
  const stored = "知识问答仅使用已审批且仍有效资料，并禁止动态动作工具"
  const english = localizeWorkflowVersionSummary(stored, translate("en-US"))
  assert.match(english, /approved.*valid/i)
  assert.doesNotMatch(english, /[\p{Script=Han}]/u)
  assert.equal(localizeWorkflowVersionSummary(stored, translate("zh-CN")), stored)
  assert.equal(localizeWorkflowVersionSummary(english, translate("zh-CN")), stored)

  const initial = localizeWorkflowVersionSummary("平台租户级知识问答流程初始稳定版本", translate("en-US"))
  assert.match(initial, /initial stable version/i)
  assert.doesNotMatch(initial, /[\p{Script=Han}]/u)
})

test("all built-in manifest version descriptions have localized display text", () => {
  const manifestRoot = fileURLToPath(new URL("../../../../../internal/ai/workflow/builtin/manifests/", import.meta.url))
  for (const file of readdirSync(manifestRoot).filter(file => file.endsWith(".json"))) {
    const manifest = JSON.parse(readFileSync(path.join(manifestRoot, file), "utf8"))
    for (const summary of [manifest.initialChange, manifest.upgradeChange]) {
      for (const locale of ["en-US", "es-ES"]) {
        const localized = localizeWorkflowVersionSummary(summary, translate(locale))
        assert.ok(localized.trim(), `${file}: ${locale}`)
        assert.doesNotMatch(localized, /[\p{Script=Han}]/u, `${file}: ${locale}`)
        assert.equal(localizeWorkflowVersionSummary(localized, translate("zh-CN")), summary)
      }
    }
  }
})

test("system rollback descriptions retain their version number", () => {
  assert.equal(localizeWorkflowVersionSummary("回滚至 V12", translate("en-US")), "Rolled back to V12")
  assert.equal(localizeWorkflowVersionSummary("Rolled back to V12", translate("zh-CN")), "回滚至 V12")
})

test("custom version notes remain unchanged in either language", () => {
  for (const custom of ["客户自定义版本说明", "Custom rollout notes", "知识问答仅使用已审批且仍有效资料，并禁止动态动作工具；客户另有说明", "", "  Custom notes  "]) {
    assert.equal(localizeWorkflowVersionSummary(custom, translate("en-US")), custom)
    assert.equal(localizeWorkflowVersionSummary(custom, translate("zh-CN")), custom)
  }
})

test("versions without a publisher name show a localized system label", () => {
  const page = readFileSync(fileURLToPath(new URL("./template-page.tsx", import.meta.url)), "utf8")
  assert.doesNotMatch(page, /publishedByName \|\| "system"/)
  assert.equal(translateMessage("en-US", "workflowExtract.enterpriseWorkflowDetail.versionsPanel.systemPublisher"), "System")
  assert.equal(translateMessage("zh-CN", "workflowExtract.enterpriseWorkflowDetail.versionsPanel.systemPublisher"), "系统")
})

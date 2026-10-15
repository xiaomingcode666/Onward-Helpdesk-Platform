import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

function loadTsModule(relativePath, requireModule) {
  return readFile(new URL(relativePath, import.meta.url), "utf8").then((source) => {
    const compiled = ts.transpileModule(source, {
      compilerOptions: { target: ts.ScriptTarget.ES2017, module: ts.ModuleKind.CommonJS, resolveJsonModule: true, esModuleInterop: true },
      fileName: relativePath,
    })
    const sandbox = { exports: {}, module: { exports: {} }, require: requireModule }
    sandbox.exports = sandbox.module.exports
    vm.runInNewContext(compiled.outputText, sandbox)
    return sandbox.module.exports
  })
}

const config = await loadTsModule("../../../i18n/config.ts", () => ({}))
const catalogs = {}
for (const locale of ["zh-CN", "en-US", "es-ES"]) {
  catalogs[`@/messages/${locale}.json`] = JSON.parse(await readFile(new URL(`../../../messages/${locale}.json`, import.meta.url), "utf8"))
}
const messages = await loadTsModule("../../../i18n/messages.ts", (name) => {
  if (name === "@/i18n/config") return config
  if (name === "@/i18n/extracted") return { getExtractedMessage: () => undefined }
  return { __esModule: true, default: catalogs[name] }
})
const { displayTicketWorkflowVersionNote, displayTicketWorkflowProjectName } = await loadTsModule("./ticket-status-workflow-i18n.ts", (name) => {
  if (name === "@/i18n/messages") return messages
  return {}
})

test("translates generated configuration version notes in English", () => {
  assert.equal(
    displayTicketWorkflowVersionNote("基于 V34 调整运营配置", "en-US"),
    "Adjust operational configuration based on V34",
  )
  assert.equal(
    displayTicketWorkflowVersionNote("基于 V15 调整：基于 V13 调整：基于 V12 调整运营配置", "en-US"),
    "Adjusted from V15: Adjusted from V13: Adjust operational configuration based on V12",
  )
  assert.equal(
    displayTicketWorkflowVersionNote("将现有运营设置纳入配置版本管理", "en-US"),
    "Bring existing operational settings under configuration version management",
  )
  assert.equal(
    displayTicketWorkflowVersionNote("工单流程：达到（范围：*）", "en-US"),
    "Ticket workflow: Reach (scope: *)",
  )
  assert.equal(
    displayTicketWorkflowVersionNote("工单流程： Restore ticket workflow version #40 （范围：*）", "en-US"),
    "Ticket workflow: Restore ticket workflow version #40 (scope: *)",
  )
  assert.equal(
    displayTicketWorkflowVersionNote("工单流程：恢复版本 #40 的工单流程（范围：*）", "en-US"),
    "Ticket workflow: Restore ticket workflow version #40 (scope: *)",
  )
})

test("keeps generated configuration version notes in Chinese", () => {
  assert.equal(
    displayTicketWorkflowVersionNote("基于 V34 调整运营配置", "zh-CN"),
    "基于 V34 调整运营配置",
  )
  assert.equal(
    displayTicketWorkflowVersionNote("工单流程：达到（范围：*）", "zh-CN"),
    "工单流程：达到（范围：*）",
  )
})

test("renders the company default project name in the current locale", () => {
  assert.equal(displayTicketWorkflowProjectName("公司默认流程", "en-US"), "Company default workflow")
  assert.equal(displayTicketWorkflowProjectName("Company default workflow", "zh-CN"), "公司默认流程")
  assert.equal(displayTicketWorkflowProjectName("产线 A", "en-US"), "产线 A")
})

test("keeps custom version notes unchanged", () => {
  assert.equal(displayTicketWorkflowVersionNote("调整夜间值班流转", "en-US"), "调整夜间值班流转")
})

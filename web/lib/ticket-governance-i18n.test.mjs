import assert from "node:assert/strict"
import test from "node:test"
import ts from "typescript"
import { readFile } from "node:fs/promises"
import vm from "node:vm"

async function loadModule() {
  const source = await readFile(new URL("./ticket-governance.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: { target: ts.ScriptTarget.ES2017, module: ts.ModuleKind.CommonJS },
    fileName: "ticket-governance.ts",
  })
  const messages = {
    "ticketGovernance.createdReason": { "zh-CN": "已创建工单", "en-US": "Ticket created" },
    "ticketGovernance.duplicateChildReason": {
      "zh-CN": "已确认同一问题，关联到首次工单",
      "en-US": "Confirmed the same issue; linked to the first ticket",
    },
    "ticketGovernance.mergeReason": {
      "zh-CN": "重复工单，合并至主工单",
      "en-US": "Duplicate ticket; merged into the primary ticket",
    },
  }
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require: (id) => {
      if (id === "@/i18n/messages") {
        return {
          matchMessage: (key, text) => Object.values(messages[key] ?? {}).includes(text.trim()) ? {} : null,
          translateCurrentMessage: (key) => messages[key]?.["en-US"] ?? key,
        }
      }
      if (id === "@/lib/api/client") return { apiGet: () => {}, apiPost: () => {} }
      throw new Error(`unexpected import ${id}`)
    },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("localizes generated ticket relation reasons while preserving custom text", async () => {
  const { governanceReasonLabel } = await loadModule()

  assert.equal(governanceReasonLabel("已确认同一问题，关联到首次工单"), "Confirmed the same issue; linked to the first ticket")
  assert.equal(governanceReasonLabel("已创建工单"), "Ticket created")
  assert.equal(governanceReasonLabel("客户补充的关联说明"), "客户补充的关联说明")
})

test("uses the active component translator for generated reasons", async () => {
  const { governanceReasonLabel } = await loadModule()

  assert.equal(
    governanceReasonLabel("已创建工单", (key) => `translated:${key}`),
    "translated:createdReason",
  )
})

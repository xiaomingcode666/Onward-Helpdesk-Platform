import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

async function loadModule() {
  const source = await readFile(new URL("./customer-portal-runtime.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "customer-portal-runtime.ts",
  })
  const sandbox = { exports: {}, module: { exports: {} } }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("customer conversation landing honors an explicit conversation", async () => {
  const { selectInitialCustomerConversationId } = await loadModule()
  const items = [
    { id: 3, status: "ai_serving" },
    { id: 2, status: "closed" },
  ]
  assert.equal(selectInitialCustomerConversationId(items, 2), 2)
})

test("customer conversation landing prefers an active conversation", async () => {
  const { selectInitialCustomerConversationId } = await loadModule()
  const items = [
    { id: 5, status: "closed" },
    { id: 4, status: "human_serving" },
  ]
  assert.equal(selectInitialCustomerConversationId(items, 0), 4)
})

test("customer conversation landing keeps the list open when all are closed", async () => {
  const { selectInitialCustomerConversationId } = await loadModule()
  const items = [
    { id: 5, status: "closed" },
    { id: 4, status: "closed" },
  ]
  assert.equal(selectInitialCustomerConversationId(items, 0), 0)
})

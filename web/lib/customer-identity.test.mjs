import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

async function loadModule() {
  const source = await readFile(new URL("./customer-identity.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "customer-identity.ts",
  })
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require: (specifier) => {
      if (specifier === "@/i18n/messages") {
        return {
          translateCurrentMessage: (key) => (key.endsWith("unlinkedCustomer") ? "未关联客户账号" : key),
        }
      }
      throw new Error(`Unexpected require: ${specifier}`)
    },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("maps missing and legacy visitor identities to the customer-account fallback", async () => {
  const { customerDisplayName } = await loadModule()

  assert.equal(customerDisplayName(""), "未关联客户账号")
  assert.equal(customerDisplayName("访客"), "未关联客户账号")
  assert.equal(customerDisplayName("访客a1b2c3d4"), "未关联客户账号")
  assert.equal(customerDisplayName("访客", "客户账号"), "客户账号")
})

test("preserves formal customer names", async () => {
  const { customerDisplayName } = await loadModule()

  assert.equal(customerDisplayName(" 华东工厂设备主管 "), "华东工厂设备主管")
  assert.equal(customerDisplayName("support@example.com"), "未关联客户账号")
})

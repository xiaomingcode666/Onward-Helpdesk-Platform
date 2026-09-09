import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

async function loadPermissionGuardModule() {
  const source = await readFile(
    new URL("./permission-guard.tsx", import.meta.url),
    "utf8"
  )
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
      jsx: ts.JsxEmit.ReactJSX,
    },
    fileName: "permission-guard.tsx",
  })
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require: (specifier) => {
      if (specifier === "react/jsx-runtime") {
        return {
          jsx: () => ({}),
          Fragment: "Fragment",
        }
      }
      throw new Error(`Unexpected import: ${specifier}`)
    },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("protected menus stay closed until the matching permission is present", async () => {
  const { CanAccessMenu } = await loadPermissionGuardModule()

  assert.equal(CanAccessMenu("tenant.view", undefined), false)
  assert.equal(CanAccessMenu("tenant.view", []), false)
  assert.equal(
    CanAccessMenu("tenant.view", ["tenant.view"]),
    true
  )
})

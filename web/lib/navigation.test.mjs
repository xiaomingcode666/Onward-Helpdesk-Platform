import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

async function loadNavigationModule() {
  const source = await readFile(new URL("./navigation.tsx", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.CommonJS,
      jsx: ts.JsxEmit.ReactJSX,
    },
    fileName: "navigation.tsx",
  })
  const iconModule = new Proxy(
    {},
    {
      get:
        (_target, property) =>
        function IconStub() {
          return property
        },
    }
  )
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require: (specifier) => {
      if (specifier === "lucide-react") {
        return iconModule
      }
      if (specifier === "react/jsx-runtime") {
        return {
          jsx: () => ({}),
          jsxs: () => ({}),
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

test("dashboard breadcrumbs expose the matching menu name only", async () => {
  const { getPageBreadcrumbKeys } = await loadNavigationModule()

  assert.deepEqual(
    JSON.parse(JSON.stringify(getPageBreadcrumbKeys("/dashboard/conversations"))),
    ["nav.conversations"]
  )
  assert.deepEqual(
    JSON.parse(JSON.stringify(getPageBreadcrumbKeys("/dashboard/knowledge/123"))),
    ["nav.knowledge"]
  )
})

test("after-sales compatibility routes use the same concise breadcrumb contract", async () => {
  const { getPageBreadcrumbKeys } = await loadNavigationModule()

  assert.deepEqual(
    JSON.parse(JSON.stringify(getPageBreadcrumbKeys("/ticket-workbench"))),
    ["nav.workbench"]
  )
  assert.deepEqual(
    JSON.parse(JSON.stringify(getPageBreadcrumbKeys("/products"))),
    ["nav.products"]
  )
})

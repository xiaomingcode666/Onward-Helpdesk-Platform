import assert from "node:assert/strict"
import test from "node:test"
import ts from "typescript"
import { readFile } from "node:fs/promises"
import vm from "node:vm"

async function loadWidgetTestNavigation() {
  const source = await readFile(new URL("./widget-test-navigation.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "widget-test-navigation.ts",
  })
  const sandbox = {
    exports: {},
    module: { exports: {} },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("widget test page lives below support so /support remains available", async () => {
  const { getWidgetTestPath } = await loadWidgetTestNavigation()

  assert.equal(getWidgetTestPath(), "/support/widget-test")
})

test("widget test entry avoids demo route and placeholder copy", async () => {
  const sources = await Promise.all([
    readFile(new URL("./widget-test.tsx", import.meta.url), "utf8"),
    readFile(new URL("./widget-test-navigation.ts", import.meta.url), "utf8"),
    readFile(new URL("../../app/dashboard/channels/_components/edit.tsx", import.meta.url), "utf8"),
    readFile(new URL("../../messages/zh-CN.json", import.meta.url), "utf8"),
    readFile(new URL("../../messages/en-US.json", import.meta.url), "utf8"),
    readFile(new URL("../../messages/es-ES.json", import.meta.url), "utf8"),
  ])
  const combined = sources.join("\n")

  assert.doesNotMatch(combined, /\/support\/demo|widgetDemo|SupportWidgetDemo|WidgetDemo/)
  assert.doesNotMatch(combined, /Demo User|Widget Mount Test|local simulation|本地模拟|demo-user-001/)
  assert.match(combined, /\/support\/widget-test/)
  assert.match(combined, /SupportWidgetTest/)
  assert.match(combined, /Web Widget 接入测试/)
  assert.match(combined, /window\.RemoteHelpDeskConfig/)
})

test("channel web access guide shows the RemoteHelpDesk widget config name", async () => {
  const source = await readFile(new URL("../../app/dashboard/channels/_components/edit.tsx", import.meta.url), "utf8")
  const snippetSource = source.match(/const snippet = useMemo[\s\S]*?}, \[channelId, origin\]\)/)?.[0] || ""

  assert.match(snippetSource, /window\.RemoteHelpDeskConfig/)
  assert.doesNotMatch(snippetSource, /window\.AgentDeskConfig/)
})

import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

async function loadModule(env, windowLocation) {
  const source = await readFile(new URL("./websocket.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "websocket.ts",
  })
  const sandbox = {
    URL,
    exports: {},
    module: { exports: {} },
    process: { env },
    window: { location: windowLocation },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("browser websocket base ignores localhost production API base", async () => {
  const { createWebSocketBaseUrl } = await loadModule(
    { NODE_ENV: "production", NEXT_PUBLIC_API_BASE_URL: "http://127.0.0.1:8083" },
    { protocol: "https:", host: "remotehelpdesk.digintelspace.com:8443", href: "https://remotehelpdesk.digintelspace.com:8443/customer/chat" },
  )

  assert.equal(createWebSocketBaseUrl(), "wss://remotehelpdesk.digintelspace.com:8443")
})

test("browser websocket base preserves explicit public API host", async () => {
  const { createWebSocketBaseUrl } = await loadModule(
    { NODE_ENV: "production", NEXT_PUBLIC_API_BASE_URL: "https://api.example.com" },
    { protocol: "https:", host: "remotehelpdesk.digintelspace.com:8443", href: "https://remotehelpdesk.digintelspace.com:8443/customer/chat" },
  )

  assert.equal(createWebSocketBaseUrl(), "wss://api.example.com")
})

test("browser websocket base keeps localhost during development", async () => {
  const { createWebSocketBaseUrl } = await loadModule(
    { NODE_ENV: "development", NEXT_PUBLIC_API_BASE_URL: "" },
    { protocol: "http:", host: "localhost:3000", href: "http://localhost:3000/customer/chat" },
  )

  assert.equal(createWebSocketBaseUrl(), "ws://127.0.0.1:8083")
})

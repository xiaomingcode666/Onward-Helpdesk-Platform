import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

function createElement(tagName) {
  return {
    tagName,
    children: [],
    dataset: {},
    style: {},
    attributes: {},
    listeners: {},
    parentNode: null,
    contentWindow: {},
    setAttribute(name, value) {
      this.attributes[name] = String(value)
    },
    appendChild(child) {
      child.parentNode = this
      this.children.push(child)
      return child
    },
    removeChild(child) {
      this.children = this.children.filter((item) => item !== child)
      child.parentNode = null
      return child
    },
    addEventListener(type, handler) {
      this.listeners[type] = handler
    },
    click() {
      this.listeners.click?.()
    },
  }
}

async function loadSdk(config, options = {}) {
  const source = await readFile(new URL("./remote-helpdesk-sdk.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.ESNext,
      importsNotUsedAsValues: ts.ImportsNotUsedAsValues.Remove,
    },
    fileName: "remote-helpdesk-sdk.ts",
  })
  const body = createElement("body")
  const sandbox = {
    URL,
    console,
    fetch: async (url) => ({
      json: async () =>
        String(url).endsWith("/api/config")
          ? {
              success: true,
              data: {
                language: "en-US",
              },
            }
          : {
              success: true,
              data: {
                themeColor: "#2563eb",
              },
            },
    }),
    document: {
      body,
      currentScript: {
        src: "https://chat.example/sdk/remote-helpdesk-sdk.min.js",
      },
      createElement,
      createElementNS: (_namespace, tagName) => createElement(tagName),
    },
    window: {
      location: {
        origin: "https://host.example",
      },
      addEventListener() {},
      clearTimeout() {},
      setTimeout(handler) {
        handler()
        return 1
      },
    },
  }
  sandbox.window.window = sandbox.window
  if (options.legacyConfig) {
    sandbox.window.AgentDeskConfig = config
  } else {
    sandbox.window.RemoteHelpDeskConfig = config
  }
  sandbox.window.document = sandbox.document
  sandbox.window.fetch = sandbox.fetch
  sandbox.window.console = console
  sandbox.window.URL = URL
  sandbox.window.setTimeout = sandbox.window.setTimeout
  sandbox.window.clearTimeout = sandbox.window.clearTimeout

  const compiledCode = compiled.outputText.replace(/\nexport\s*\{\};?\s*$/, "")
  vm.runInNewContext(compiledCode, sandbox)
  await Promise.resolve()
  await Promise.resolve()
  return sandbox
}

async function flushPromises(count = 10) {
  for (let i = 0; i < count; i += 1) {
    await Promise.resolve()
  }
}

test("getChatUrl resolves a fresh userToken for each call", async () => {
  let calls = 0
  const sandbox = await loadSdk({
    channelId: "ch_1",
    baseUrl: "https://api.example",
    getUserToken: async () => `token_${++calls}`,
  })

  assert.equal(typeof sandbox.window.RemoteHelpDeskWidget.getChatUrl, "function")
  assert.equal(sandbox.window.AgentDeskWidget, sandbox.window.RemoteHelpDeskWidget)

  const first = await sandbox.window.RemoteHelpDeskWidget.getChatUrl()
  const second = await sandbox.window.RemoteHelpDeskWidget.getChatUrl()

  assert.equal(new URL(first).pathname, "/support/chat/")
  assert.equal(new URL(second).pathname, "/support/chat/")
  assert.equal(new URL(first).searchParams.get("userToken"), "token_1")
  assert.equal(new URL(second).searchParams.get("userToken"), "token_2")
  assert.equal(calls, 2)
})

test("legacy AgentDeskConfig remains a compatibility alias", async () => {
  const sandbox = await loadSdk({
    channelId: "legacy_ch",
    baseUrl: "https://api.example",
  }, { legacyConfig: true })

  assert.equal(typeof sandbox.window.RemoteHelpDeskWidget.getChatUrl, "function")
  assert.equal(sandbox.window.AgentDeskWidget, sandbox.window.RemoteHelpDeskWidget)

  const url = await sandbox.window.AgentDeskWidget.getChatUrl()
  assert.equal(new URL(url).searchParams.get("channelId"), "legacy_ch")
})

test("launcher click creates chat iframe with a freshly resolved userToken", async () => {
  const sandbox = await loadSdk({
    channelId: "ch_1",
    baseUrl: "https://api.example",
    getUserToken: async () => "click_token",
  })
  await flushPromises()
  const launcher = sandbox.document.body.children.find(
    (child) => child.dataset.remoteHelpDeskWidget === "launcher"
  )

  assert.ok(launcher)
  assert.equal(launcher.children.at(-1)?.textContent, "Support")

  launcher.click()
  await flushPromises()

  const frame = sandbox.document.body.children.find(
    (child) => child.dataset.remoteHelpDeskWidget === "frame"
  )

  assert.ok(frame)
  assert.equal(new URL(frame.src).pathname, "/support/chat/")
  assert.equal(new URL(frame.src).searchParams.get("userToken"), "click_token")
})

test("chat iframe layout uses dynamic viewport height for iOS browser chrome", async () => {
  const sandbox = await loadSdk({
    channelId: "ch_1",
    baseUrl: "https://api.example",
  })
  await flushPromises()
  const launcher = sandbox.document.body.children.find(
    (child) => child.dataset.remoteHelpDeskWidget === "launcher"
  )

  launcher.click()
  await flushPromises()

  const frame = sandbox.document.body.children.find(
    (child) => child.dataset.remoteHelpDeskWidget === "frame"
  )

  assert.ok(frame)
  assert.match(frame.style.height, /100dvh/)
  assert.match(frame.style.maxWidth, /100vw/)
})

import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import vm from "node:vm"
import ts from "typescript"

// Execute the page's actual loader and selection callback with controlled HTTP
// promises so request ordering can be reproduced without a business backend.
const source = await readFile(new URL("./customer-portal-pages.tsx", import.meta.url), "utf8")
const ast = ts.createSourceFile("customer-portal-pages.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX)
let loader
let selection
function visit(node) {
  if (ts.isFunctionDeclaration(node) && node.name?.text === "loadMessages") loader = node.getText(ast)
  if (ts.isJsxAttribute(node) && node.name.getText(ast) === "onClick" &&
    node.initializer && ts.isJsxExpression(node.initializer) && node.initializer.expression &&
    node.initializer.expression.getText(ast).includes("messageListRequestRef.current += 1")) {
    selection = node.initializer.expression.getText(ast)
  }
  ts.forEachChild(node, visit)
}
visit(ast)
assert.ok(loader, "message loader must exist")
assert.ok(selection, "conversation selection callback must exist")
const compiled = ts.transpileModule(`${loader}\nexports.load = loadMessages; exports.select = item => (${selection})();`, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText

function deferred() {
  let resolve, reject
  const promise = new Promise((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

function setup() {
  const state = { loading: false, messages: [], errors: [], requests: [], receipts: [] }
  const globals = {
    exports: {},
    messageListRequestRef: { current: 0 },
    selectedConversationIdRef: { current: 42 },
    fetchImMessages: ({ conversationId }) => {
      const request = { conversationId, ...deferred() }
      state.requests.push(request)
      return request.promise
    },
    markImMessageRead: () => {
      const receipt = deferred()
      state.receipts.push(receipt)
      return receipt.promise
    },
    setMessageLoading: value => { state.loading = value },
    setMessages: value => { state.messages = typeof value === "function" ? value(state.messages) : value },
    replaceLoadedImMessagesPreservingLocalTransfers: (_current, incoming) => incoming,
    toast: { error: message => state.errors.push(message) },
    t: key => key,
    setSelectedConversationId: value => { state.selectedId = value },
    syncCustomerConversationUrl: () => {},
    setMobileConversationListOpen: () => {},
  }
  vm.runInNewContext(compiled, globals)
  return { state, ...globals.exports }
}

const reply = content => ({ results: [{ id: 1, conversationId: 42, content }] })
const flush = () => new Promise(resolve => setImmediate(resolve))

test("background refresh superseding initial history load releases the loading skeleton", async () => {
  const { state, load } = setup()
  const initial = load(42)
  const refresh = load(42, true)
  assert.equal(state.loading, true)
  state.requests[1].resolve(reply("latest history"))
  await flush()
  state.receipts.forEach(receipt => receipt.resolve())
  await refresh
  assert.equal(state.loading, false, "latest history is loaded but skeleton remains")
  state.requests[0].resolve(reply("stale history"))
  await initial
  assert.equal(state.messages[0].content, "latest history")
})

test("read receipt cannot keep successfully fetched messages in loading state", async () => {
  const { state, load } = setup()
  const initial = load(42)
  state.requests[0].resolve(reply("visible message"))
  await flush()
  assert.equal(state.messages[0].content, "visible message")
  assert.equal(state.loading, false, "loading waits for unrelated read receipt")
  state.receipts.forEach(receipt => receipt.resolve())
  await initial
})

test("selecting the current conversation does not invalidate its in-flight history request", async () => {
  const { state, load, select } = setup()
  const initial = load(42)
  select({ id: 42 })
  state.requests[0].resolve(reply("current history"))
  await flush()
  state.receipts.forEach(receipt => receipt.resolve())
  await initial
  assert.equal(state.loading, false)
  assert.equal(state.messages[0]?.content, "current history")
})

test("switching conversations rejects stale history without clearing the new loading state", async () => {
  const { state, load, select } = setup()
  const previous = load(42)
  select({ id: 99 })
  const current = load(99)
  state.requests[0].resolve(reply("previous conversation"))
  await previous
  assert.equal(state.loading, true)
  assert.equal(state.messages.length, 0)
  state.requests[1].resolve({ results: [{ id: 2, conversationId: 99, content: "new conversation" }] })
  await flush()
  state.receipts.forEach(receipt => receipt.resolve())
  await current
  assert.equal(state.loading, false)
  assert.equal(state.messages[0].conversationId, 99)
})

test("history request failure releases loading and reports the error", async () => {
  const { state, load } = setup()
  const initial = load(42)
  state.requests[0].reject(new Error("history unavailable"))
  await initial
  assert.equal(state.loading, false)
  assert.equal(state.errors.length, 1)
})

test("read receipt failure does not hide loaded messages or report a history failure", async () => {
  const { state, load } = setup()
  const initial = load(42)
  state.requests[0].resolve(reply("visible history"))
  await flush()
  state.receipts[0].reject(new Error("receipt unavailable"))
  await initial
  await flush()
  assert.equal(state.loading, false)
  assert.equal(state.messages[0].content, "visible history")
  assert.equal(state.errors.length, 0)
})

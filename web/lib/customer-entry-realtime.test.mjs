import assert from "node:assert/strict"
import test from "node:test"
import { readFile } from "node:fs/promises"
import vm from "node:vm"
import ts from "typescript"

async function loadModule() {
  const source = await readFile(
    new URL("./customer-entry-realtime.ts", import.meta.url),
    "utf8",
  )
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "customer-entry-realtime.ts",
  })
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require() {
      return {}
    },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

function context(overrides = {}) {
  return {
    meetings: [],
    tickets: [],
    conversations: [],
    ...overrides,
  }
}

test("active meeting selects its ticket conversation", async () => {
  const { selectCustomerRealtimeConversationId } = await loadModule()
  const result = selectCustomerRealtimeConversationId(
    context({
      meetings: [{ id: "m1", ticketId: 7, status: "active" }],
      tickets: [
        { id: 6, conversationId: 60, status: "processing" },
        { id: 7, conversationId: 70, status: "processing" },
      ],
      conversations: [{ id: 60 }, { id: 70 }],
    }),
  )
  assert.equal(result, 70)
})

test("open conversation wins when there is no active meeting", async () => {
  const { selectCustomerRealtimeConversationId } = await loadModule()
  const result = selectCustomerRealtimeConversationId(
    context({
      conversations: [
        { id: 90, endedAt: "2026-07-28T20:00:00Z" },
        { id: 91 },
      ],
    }),
  )
  assert.equal(result, 91)
})

test("active ticket is used when context has no open conversation summary", async () => {
  const { selectCustomerRealtimeConversationId } = await loadModule()
  const result = selectCustomerRealtimeConversationId(
    context({
      tickets: [
        { id: 1, conversationId: 10, status: "closed" },
        { id: 2, conversationId: 20, status: "pending" },
      ],
    }),
  )
  assert.equal(result, 20)
})

test("closed history remains selectable as a last-resort conversation", async () => {
  const { selectCustomerRealtimeConversationId } = await loadModule()
  const result = selectCustomerRealtimeConversationId(
    context({
      conversations: [{ id: 30, endedAt: "2026-07-28T20:00:00Z" }],
      tickets: [{ id: 3, conversationId: 30, status: "closed" }],
    }),
  )
  assert.equal(result, 30)
})

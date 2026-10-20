import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import vm from "node:vm"
import ts from "typescript"

async function load(name, dependencies = {}) {
  const source = await readFile(new URL(name, import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } })
  const sandbox = { exports: {}, require: (id) => { assert.ok(id in dependencies, id); return dependencies[id] } }
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.exports
}
const merge = await load("./im-message-merge.ts")
const { applyAIMessageStream } = await load("./im-message-stream.ts", { "@/lib/im-message-merge": merge })
const event = { conversationId: 42, sourceMessageId: 10, streamId: "attempt-1", clientMsgId: "ai_reply_10", sequence: 1, content: "Hello", startedAt: 100, streamExpiresAt: Date.now() + 60000, sentAt: "2026-10-20 14:00:00", done: false }
const userMessage = { id: 10, conversationId: 42, senderType: "customer", messageType: "text", content: "Question", sendStatus: 2, customerRead: true, agentRead: true }

test("stream snapshots update one visible draft, ignore duplicate frames and keep draft after its question", () => {
  let messages = applyAIMessageStream([userMessage], event, 42)
  assert.equal(messages.length, 2)
  assert.equal(messages[1].content, "Hello")
  messages = applyAIMessageStream(messages, { ...event, sequence: 2, content: "Hello world" }, 42)
  assert.equal(messages.length, 2)
  assert.equal(messages[1].content, "Hello world")
  assert.equal(applyAIMessageStream(messages, event, 42), messages)
  assert.equal(applyAIMessageStream(messages, { ...event, conversationId: 99 }, 42), messages)
})

test("committed reply replaces draft and late frames cannot resurrect it", () => {
  const draft = applyAIMessageStream([userMessage], event, 42)
  const final = { ...userMessage, id: 11, senderType: "ai", clientMsgId: event.clientMsgId, content: "Hello world" }
  const messages = merge.upsertImMessageReplacingClientMessage(draft, final)
  assert.equal(messages.length, 2)
  assert.equal(messages[1].id, 11)
  assert.equal(applyAIMessageStream(messages, { ...event, sequence: 5 }, 42), messages)
})

test("refresh preserves a live draft; completed, expired and replaced attempts are cleaned up", () => {
  const draft = applyAIMessageStream([userMessage], event, 42)
  const refreshed = merge.replaceLoadedImMessagesPreservingLocalTransfers(draft, [userMessage])
  assert.equal(refreshed.length, 2)
  assert.equal(merge.cursorFromLoadedImMessages(refreshed), "10")
  assert.equal(applyAIMessageStream(refreshed, { ...event, sequence: 2, done: true }, 42).length, 1)
  assert.equal(applyAIMessageStream([userMessage], { ...event, streamExpiresAt: 1 }, 42).length, 1)
  const retry = applyAIMessageStream(draft, { ...event, streamId: "attempt-2", startedAt: 200 }, 42)
  assert.equal(applyAIMessageStream(retry, { ...event, done: true }, 42), retry)
  assert.equal(retry[1].content, "Hello")
})

import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

async function loadModule() {
  const source = await readFile(new URL("./ticket-workbench-queue.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "ticket-workbench-queue.ts",
  })
  const sandbox = { exports: {}, module: { exports: {} } }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("ticket workbench queue keeps active work newest first", async () => {
  const { sortWorkbenchQueueItems } = await loadModule()
  const items = [
    { key: "ticket-new", ticketId: 2, updatedAt: "2026-08-07T11:00:00Z", ticket: { id: 2, created_at: "2026-08-07T10:00:00Z", updated_at: "2026-08-07T11:00:00Z" } },
    { key: "conversation-live", updatedAt: "2026-08-07T12:00:00Z" },
    { key: "ticket-old", ticketId: 1, updatedAt: "2026-08-07T09:00:00Z", ticket: { id: 1, created_at: "2026-08-07T09:00:00Z" } },
  ]

  assert.equal(
    sortWorkbenchQueueItems(items).map((item) => item.key).join(","),
    "conversation-live,ticket-new,ticket-old",
  )
})

test("ticket workbench queue puts completed items after active items", async () => {
  const { sortWorkbenchQueueItems } = await loadModule()
  const items = [
    { key: "ticket-done-old", isDone: true, ticketId: 1, ticket: { id: 1, created_at: "2026-08-07T09:00:00Z" } },
    { key: "conversation-active", updatedAt: "2026-08-07T10:00:00Z" },
  ]

  assert.equal(
    sortWorkbenchQueueItems(items).map((item) => item.key).join(","),
    "conversation-active,ticket-done-old",
  )
})

test("items with matching update time prefer ticket-backed work", async () => {
  const { sortWorkbenchQueueItems } = await loadModule()
  const items = [
    { key: "conversation-new", updatedAt: "2026-08-07T11:00:00Z" },
    { key: "ticket-new", ticketId: 9, updatedAt: "2026-08-07T11:00:00Z", ticket: { id: 9, created_at: "2026-08-07T08:00:00Z" } },
  ]

  assert.equal(
    sortWorkbenchQueueItems(items).map((item) => item.key).join(","),
    "ticket-new,conversation-new",
  )
})

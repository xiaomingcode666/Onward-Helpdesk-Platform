import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

async function loadModule() {
  const source = await readFile(new URL("./ticket-workbench-route.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "ticket-workbench-route.ts",
  })
  const sandbox = { exports: {}, module: { exports: {} }, URL }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("conversation deep link resolves a standalone ticket queue row", async () => {
  const { findWorkbenchRouteItem } = await loadModule()
  const items = [
    { key: "conversation-258", conversationId: 258, ticketId: 324 },
    { key: "ticket-327", conversationId: 266, ticketId: 327 },
  ]

  assert.equal(
    findWorkbenchRouteItem(items, { conversationId: 266 })?.key,
    "ticket-327",
  )
})

test("ticket deep link resolves by ticket id", async () => {
  const { findWorkbenchRouteItem } = await loadModule()
  const items = [
    { key: "conversation-266", conversationId: 266, ticketId: 327 },
    { key: "ticket-328", conversationId: 267, ticketId: 328 },
  ]

  assert.equal(
    findWorkbenchRouteItem(items, { ticketId: 327 })?.key,
    "conversation-266",
  )
})

test("ticket conversation index prefers the deep-linked ticket", async () => {
  const { indexWorkbenchTicketsByConversation } = await loadModule()
  const tickets = [
    { id: 327, conversation_id: 266 },
    { id: 328, conversation_id: 266 },
    { id: 329, conversation_id: 267 },
  ]

  const indexed = indexWorkbenchTicketsByConversation(tickets, 328)
  assert.equal(indexed.get(266)?.id, 328)
  assert.equal(indexed.get(267)?.id, 329)
})

test("ticket workbench links reject non numeric ticket ids", async () => {
  const {
    buildEnterpriseTicketWorkbenchPath,
    buildEnterpriseTicketWorkbenchPathFromItem,
    normalizeEnterpriseActionUrl,
    normalizeWorkbenchId,
  } = await loadModule()

  assert.equal(buildEnterpriseTicketWorkbenchPath(345), "/enterprise/ticket-workbench?ticket_id=345")
  assert.equal(buildEnterpriseTicketWorkbenchPath("345"), "/enterprise/ticket-workbench?ticket_id=345")
  assert.equal(buildEnterpriseTicketWorkbenchPath("enterprise"), "/enterprise/ticket-workbench")
  assert.equal(normalizeWorkbenchId("enterprise"), 0)
  assert.equal(normalizeEnterpriseActionUrl("/enterprise/tickets/345"), "/enterprise/ticket-workbench?ticket_id=345")
  assert.equal(
    normalizeEnterpriseActionUrl("https://remotehelpdesk.digintelspace.com:8443/enterprise/tickets/enterprise"),
    "/enterprise/ticket-workbench",
  )
  assert.equal(
    normalizeEnterpriseActionUrl({ action_url: "/enterprise/tickets/enterprise", biz_type: "ticket", biz_id: 345 }),
    "/enterprise/ticket-workbench?ticket_id=345",
  )
  assert.equal(
    normalizeEnterpriseActionUrl({ action_url: "", biz_type: "ticket", biz_id: 345 }),
    "/enterprise/ticket-workbench?ticket_id=345",
  )
  assert.equal(
    normalizeEnterpriseActionUrl({
      action_url: "/enterprise/ticket-workbench?conversationId=999",
      biz_type: "ticket",
      biz_id: 345,
    }),
    "/enterprise/ticket-workbench?ticket_id=345",
  )
  assert.equal(
    normalizeEnterpriseActionUrl({
      action_url: "/dashboard/tickets/345",
      notification_type: "ticket_assigned",
      biz_type: "system",
      biz_id: 345,
    }),
    "/enterprise/ticket-workbench?ticket_id=345",
  )
  assert.equal(
    buildEnterpriseTicketWorkbenchPathFromItem({ ticket_id: 345 }),
    "/enterprise/ticket-workbench?ticket_id=345",
  )
  assert.equal(
    buildEnterpriseTicketWorkbenchPathFromItem({ action_url: "/enterprise/tickets/enterprise", id: 345 }),
    "/enterprise/ticket-workbench?ticket_id=345",
  )
  assert.equal(
    buildEnterpriseTicketWorkbenchPathFromItem({ action_url: "/enterprise/tickets/enterprise", ticket_id: 345 }),
    "/enterprise/ticket-workbench?ticket_id=345",
  )
})

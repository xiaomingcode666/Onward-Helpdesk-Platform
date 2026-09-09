import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

async function loadLoginPortalsModule() {
  const source = await readFile(new URL("./login-portals.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "login-portals.ts",
  })
  const sandbox = { exports: {}, module: { exports: {} }, URL }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("login portal follows an explicit portal and otherwise infers next", async () => {
  const { getPortalFromPath, resolveLoginPortal } = await loadLoginPortalsModule()

  assert.equal(resolveLoginPortal("platform", "/enterprise/tickets"), "platform")
  assert.equal(resolveLoginPortal(null, "/enterprise/notifications"), "enterprise")
  assert.equal(resolveLoginPortal(null, "/partner/tickets"), "partner")
  assert.equal(resolveLoginPortal(null, "/customer/history"), "customer")
  assert.equal(getPortalFromPath("/c/P15-SVC-000001"), "customer")
  assert.equal(getPortalFromPath("/mobile"), "customer")
  assert.equal(getPortalFromPath("/mobile?state=login"), "customer")
  assert.equal(getPortalFromPath("/platform-preview"), "enterprise")
})

test("redirect only keeps an internal next path from the selected portal", async () => {
  const { getPortalRedirectPath } = await loadLoginPortalsModule()

  assert.equal(
    getPortalRedirectPath("enterprise", "/enterprise/notifications"),
    "/enterprise/notifications"
  )
  assert.equal(getPortalRedirectPath("platform", "/enterprise/tickets"), "/platform")
  assert.equal(getPortalRedirectPath("partner", "//example.com"), "/partner")
  assert.equal(getPortalRedirectPath("enterprise", "/\\example.com"), "/enterprise")
})

test("authenticated sessions always land in their own domain", async () => {
  const { resolveSessionDestination } = await loadLoginPortalsModule()

  assert.equal(
    resolveSessionDestination({ domainType: "partner", permissions: ["ticket.view"] }, "/enterprise/tickets"),
    "/partner"
  )
  assert.equal(
    resolveSessionDestination(
      { domainType: "partner", permissions: ["ticket.view"] },
      "/partner/ticket-detail?collaboration_id=88"
    ),
    "/partner/ticket-detail?collaboration_id=88"
  )
  assert.equal(
    resolveSessionDestination(
      { domainType: "partner", permissions: ["meeting.view"] },
      "/partner/meeting-room?meeting_id=room-123&ticket_id=472"
    ),
    "/partner/meeting-room?meeting_id=room-123&ticket_id=472"
  )
  assert.equal(
    resolveSessionDestination({ domainType: "platform", permissions: ["tenant.view"] }, "/platform/tenants"),
    "/platform/tenants"
  )
  assert.equal(
    resolveSessionDestination({ domainType: "enterprise", permissions: ["ticket.view"] }, "/enterprise/tickets"),
    "/enterprise/tickets"
  )
  assert.equal(
    resolveSessionDestination({ domainType: "customer", permissions: ["device.view"] }, "/customer/devices"),
    "/customer/devices"
  )
  assert.equal(
    resolveSessionDestination({ domainType: "customer", permissions: ["conversation.view"] }, "/mobile?state=chat"),
    "/mobile?state=chat"
  )
  assert.equal(
    resolveSessionDestination(
      { domainType: "customer", permissions: ["conversation.view"] },
      "/customer/chat?conversationId=265"
    ),
    "/customer/chat"
  )
  assert.equal(
    resolveSessionDestination(
      { domainType: "customer", permissions: ["conversation.view"] },
      "/customer/chat?conversationId=265&deviceId=9&new=1"
    ),
    "/customer/chat?deviceId=9&new=1"
  )
  assert.equal(
    resolveSessionDestination({ domainType: "customer", permissions: ["conversation.view"] }, "/enterprise/tickets"),
    "/customer"
  )
  assert.equal(
    resolveSessionDestination(
      { domainType: "platform", permissions: ["tenant.view"] },
      "/platform/staff"
    ),
    "/platform"
  )
  assert.equal(
    resolveSessionDestination(
      { domainType: "enterprise", permissions: ["product.view", "knowledgeBase.view"] },
      "/enterprise"
    ),
    "/enterprise/products"
  )
})

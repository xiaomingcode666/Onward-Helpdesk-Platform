import { expect, test, type Locator, type Page } from "@playwright/test"
import { caseStatuses, displayTicketStatus } from "../lib/ticket-lifecycle"

test.use({ video: "off" })

test("case display prefers a recognized canonical phase without changing legacy dispatch state", () => {
  for (const case_status of caseStatuses) {
    const ticket = { status: "pending_assignee_accept", case_status }
    expect(displayTicketStatus(ticket)).toBe(case_status)
    expect(ticket.status).toBe("pending_assignee_accept")
  }
  expect(displayTicketStatus({ status: "video_support", case_status: "unknown" })).toBe("video_support")
  expect(displayTicketStatus({ status: "processing" })).toBe("processing")
})

const profile = {
  accessToken: "ticket-case-ui-fixture", domainType: "enterprise", tenantId: 8001,
  user: { id: 9001, username: "case-fixture", nickname: "客服小张", avatar: "", status: 0, roles: ["service_manager"] },
  roles: ["service_manager"], permissions: ["ticket.view", "ticket.create", "ticket.update", "ticket.assign"], locale: "zh-CN", timezone: "UTC",
}

function sampleAggregate() {
  return {
    ticket: { id: 84001, ticket_no: "CASE-UI-004", title: "隔离验收工单", description: "用于页面交互验证", status: "pending_dispatch", case_status: "new", priority: "medium", source: "manual", channel: "enterprise", product_id: 0, product_module_id: 0, created_at: "2026-09-14T01:00:00Z", updated_at: "2026-09-14T01:00:00Z", sla_deadline: "", category: "" },
    customer: { id: 8002, name: "测试客户", company: "测试公司", contact: "", email: "" },
    device_context: {}, flow: { current_step: "Accept", steps: [] },
    assignment: { assignee_id: 9003, assignee_name: "工程师小李", team_id: 1, team_name: "工程师组", assigned_at: "", accepted_at: "", accept_deadline_at: "", dispatch_attempts: 0, dispatch_deferred_until: "", last_dispatch_failure_reason: "" },
    meeting: {}, repair: { parts: [], resolution: "" }, timeline: [], assets: [], audit_refs: [], actions: {},
    case_lifecycle: { status: "new", revision: 1, owner_id: 0, owner_name: "", acknowledged_at: "", restored_at: "", waiting_reason: "", allowed_actions: ["acknowledge", "cancel"], can_transfer_owner: false, legacy_record: false },
  }
}

async function clickCaseAction(lifecycle: Locator, action: string) {
  const actions = lifecycle.getByTestId("case-status-actions")
  if (await actions.count() === 0) await lifecycle.getByTestId("case-update-status").click()
  await lifecycle.getByTestId(`case-action-${action}`).click()
}

async function isolate(page: Page, handler: (path: string, method: string, body: Record<string, unknown>) => unknown) {
  // Isolate notification sockets as well as HTTP; fixture credentials must not
  // connect to the developer's backend or cause background reconnect requests.
  await page.routeWebSocket(/\/api\/ws\//, () => {})
  await page.addInitScript(value => {
    localStorage.setItem("remote-helpdesk-session:enterprise", JSON.stringify(value))
    localStorage.setItem("remote-helpdesk-session", JSON.stringify(value))
  }, profile)
  await page.route("**/api/**", async route => {
    const path = new URL(route.request().url()).pathname
    let body: Record<string, unknown> = {}
    try { body = route.request().postDataJSON() ?? {} } catch { /* GET */ }
    let data = await handler(path, route.request().method(), body)
    if (path === "/api/auth/profile") data = profile
    else if (path.includes("unread")) data = { count: 0, unread_count: 0 }
    else if (path.includes("capabilities")) data = { service_scene: "knowledge_support", features: {} }
    await route.fulfill({ json: data === undefined ? { success: false, data: null, message: "Not provided by isolated UI fixture" } : { success: true, data } })
  })
}

// Only synthetic API responses are used: these tests never write the user's database.
test("support acknowledgement, waiting evidence and owner handover use separate controls", async ({ page }) => {
  const aggregate = sampleAggregate()
  const requests: Array<Record<string, unknown>> = []
  const errors: string[] = []
  page.on("pageerror", error => errors.push(error.message))
  await isolate(page, (path, method, body) => {
    if (path.endsWith("/tickets/summary")) return { total: 1, pending: 1, processing: 0, awaiting_customer: 0, sla_risk: 0, urgent: 0, done: 0 }
    if (path.endsWith("/tickets")) return { items: [{ ...aggregate.ticket, ...aggregate.assignment, assignee_name: "工程师小李", customer_name: "测试客户" }], total: 1, page: 1, page_size: 10, total_pages: 1 }
    if (path.endsWith("/case-owner-options")) return [{ id: 9001, name: "客服小张" }, { id: 9002, name: "客服小王" }]
    if (path.endsWith("/lifecycle") && method === "POST") {
      requests.push(body)
      if (body.action === "acknowledge") {
        Object.assign(aggregate.case_lifecycle, { status: "acknowledged", revision: 2, owner_id: 9001, owner_name: "客服小张", acknowledged_at: "2026-09-14T02:00:00Z", allowed_actions: ["triage", "wait"], can_transfer_owner: true })
      } else if (body.action === "wait") {
        Object.assign(aggregate.case_lifecycle, { status: "waiting", revision: 3, waiting_reason: body.reason, allowed_actions: ["resume"] })
      }
      aggregate.ticket.case_status = aggregate.case_lifecycle.status
      return { ticket_id: aggregate.ticket.id, operation_key: body.idempotency_key, status: aggregate.case_lifecycle.status, revision: aggregate.case_lifecycle.revision }
    }
    if (path.endsWith("/case-owner") && method === "POST") {
      requests.push(body)
      Object.assign(aggregate.case_lifecycle, { owner_id: 9002, owner_name: "客服小王", revision: 4 })
      return aggregate
    }
    if (path.endsWith("/tickets/84001")) return aggregate
  })
  let detailReads = 0
  await page.route("**/api/enterprise/v1/tickets/84001", async route => {
    detailReads += 1
    if (detailReads === 2) {
      await route.fulfill({ status: 503, json: { success: false, data: null, message: "fixture refresh unavailable" } })
      return
    }
    await route.fallback()
  })
  await page.goto("/enterprise/tickets", { waitUntil: "domcontentloaded" })
  await page.getByRole("button", { name: "CASE-UI-004", exact: true }).click()
  const lifecycle = page.getByTestId("ticket-case-lifecycle")
  await expect(lifecycle.getByTestId("ticket-case-status")).toHaveText("新建")
  await expect(lifecycle.getByTestId("case-action-resolve")).toHaveCount(0)
  await clickCaseAction(lifecycle, "acknowledge")
  let dialog = page.getByRole("dialog", { name: "客服确认受理", exact: true })
  await expect(dialog.getByRole("button", { name: /^确\s*认$/ })).toBeDisabled()
  await dialog.getByRole("combobox").click()
  await page.getByTitle("客服小张", { exact: true }).click()
  await dialog.getByRole("button", { name: /^确\s*认$/ }).click()
  await expect(dialog.getByRole("alert")).toHaveText("fixture refresh unavailable")
  await expect(dialog.getByRole("button", { name: "确认", exact: true })).toBeEnabled()
  await expect(dialog.getByRole("button", { name: "确认", exact: true })).toHaveAttribute("aria-busy", "false")
  // The write succeeded; retrying a failed detail refresh must not repeat it.
  await dialog.getByRole("button", { name: /^确\s*认$/ }).click()
  await expect(lifecycle.getByTestId("ticket-case-owner")).toHaveText("客服小张")
  expect(requests).toHaveLength(1)
  expect(requests[0]).toMatchObject({ action: "acknowledge", owner_id: 9001, expected_status: "new", expected_revision: 1 })
  expect(requests[0].idempotency_key).toMatch(/^[\da-f-]{36}$/)
  await clickCaseAction(lifecycle, "wait")
  dialog = page.getByRole("dialog", { name: "等待中", exact: true })
  await dialog.getByRole("button", { name: /^确\s*认$/ }).click()
  await expect(dialog.getByRole("alert")).toHaveText("请填写操作说明")
  expect(requests).toHaveLength(1)
  await dialog.getByRole("textbox", { name: "操作说明" }).fill("等待客户补充错误发生时间")
  await dialog.getByRole("button", { name: /^确\s*认$/ }).click()
  await expect(lifecycle.getByTestId("ticket-case-status")).toHaveText("等待中")
  await expect(lifecycle.getByText("等待客户补充错误发生时间", { exact: true })).toBeVisible()
  expect(requests[1]).toMatchObject({ expected_status: "acknowledged", expected_revision: 2, reason: "等待客户补充错误发生时间" })
  await lifecycle.getByTestId("case-owner-transfer").click()
  dialog = page.getByRole("dialog", { name: "交接客服负责人", exact: true })
  await dialog.getByRole("combobox").click()
  await page.getByTitle("客服小王", { exact: true }).click()
  await dialog.getByRole("textbox", { name: "操作说明" }).fill("小张下班，小王继续联系客户")
  await dialog.getByRole("button", { name: /^确\s*认$/ }).click()
  await expect(lifecycle.getByTestId("ticket-case-owner")).toHaveText("客服小王")
  await expect(lifecycle.getByText("工程师小李", { exact: true })).toBeVisible()
  expect(requests[2]).toMatchObject({ owner_id: 9002, expected_owner_id: 9001, reason: "小张下班，小王继续联系客户" })
  expect(requests[2].idempotency_key).toMatch(/^[\da-f-]{36}$/)
  expect(errors).toEqual([])
})

test("legacy read-only case does not invent dates or expose internal actions", async ({ page }) => {
  const aggregate = sampleAggregate()
  Object.assign(aggregate.case_lifecycle, { status: "in_triage", legacy_record: true, allowed_actions: [] })
  aggregate.ticket.case_status = "in_triage"
  await isolate(page, path => {
    if (path.endsWith("/tickets/summary")) return { total: 1 }
    if (path.endsWith("/tickets")) return { items: [{ ...aggregate.ticket, ...aggregate.assignment }], total: 1, page: 1, page_size: 10, total_pages: 1 }
    if (path.endsWith("/tickets/84001")) return aggregate
  })
  await page.goto("/enterprise/tickets", { waitUntil: "domcontentloaded" })
  await page.getByRole("button", { name: "CASE-UI-004", exact: true }).click()
  const lifecycle = page.getByTestId("ticket-case-lifecycle")
  await expect(lifecycle.getByTestId("ticket-case-status")).toHaveText("分析中")
  await expect(lifecycle.getByTestId("ticket-case-legacy")).toBeVisible()
  await expect(lifecycle.getByRole("button")).toHaveCount(0)
  await expect(lifecycle.getByText("客服确认受理时间", { exact: true })).toHaveCount(0)
  await expect(lifecycle.getByText("服务恢复时间", { exact: true })).toHaveCount(0)
})

test("customer-support triage and resume keep the engineer acceptance action separate", async ({ page }) => {
  const aggregate = sampleAggregate()
  aggregate.ticket.status = "pending_assignee_accept"
  aggregate.ticket.case_status = "acknowledged"
  Object.assign(aggregate.actions, { can_accept: true, can_takeover: false })
  Object.assign(aggregate.case_lifecycle, { status: "acknowledged", owner_id: 9001, owner_name: "客服小张", allowed_actions: ["triage", "wait"] })
  await isolate(page, (path, method, body) => {
    const ticket = { ...aggregate.ticket, ...aggregate.assignment, current_team_id: 1, customer_name: "测试客户", actions: aggregate.actions }
    if (path.endsWith("/workbench/queue")) return { conversations: [], tickets: [ticket], total: 1, page: 1, page_size: 16, total_pages: 1 }
    if (path.endsWith("/tickets")) return { items: [ticket], total: 1, page: 1, page_size: 10, total_pages: 1 }
    if (path.endsWith("/lifecycle") && method === "POST") {
      if (body.action === "triage" || body.action === "resume") {
        Object.assign(aggregate.case_lifecycle, { status: "in_triage", allowed_actions: ["wait"] })
        aggregate.ticket.status = "pending_assignee_accept"
      } else {
        Object.assign(aggregate.case_lifecycle, { status: "waiting", allowed_actions: ["resume"] })
        aggregate.ticket.status = "waiting_customer"
      }
      aggregate.case_lifecycle.revision++
      aggregate.ticket.case_status = aggregate.case_lifecycle.status
      return { ticket_id: aggregate.ticket.id, operation_key: body.idempotency_key, status: aggregate.case_lifecycle.status, revision: aggregate.case_lifecycle.revision }
    }
    if (path.endsWith("/tickets/84001")) return aggregate
    if (path.includes("knowledge-candidates") || path.includes("supplier")) return []
  })
  await page.goto("/enterprise/ticket-workbench?ticket_id=84001", { waitUntil: "domcontentloaded" })
  const lifecycle = page.getByTestId("ticket-case-lifecycle")
  await expect(lifecycle.getByTestId("ticket-case-status")).toHaveText("已确认受理")
  await expect(page.getByRole("button", { name: "取消工单", exact: true })).toHaveCount(0)
  await expect(page.getByRole("button", { name: "工程师接单", exact: true }).first()).toBeVisible()
  await clickCaseAction(lifecycle, "triage")
  await page.getByRole("dialog", { name: "开始分析", exact: true }).getByRole("button", { name: /^确\s*认$/ }).click()
  await expect(lifecycle.getByTestId("ticket-case-status")).toHaveText("分析中")
  await expect(page.getByRole("button", { name: "工程师接单", exact: true }).first()).toBeVisible()
  await clickCaseAction(lifecycle, "wait")
  let dialog = page.getByRole("dialog", { name: "等待中", exact: true })
  await dialog.getByRole("textbox", { name: "操作说明" }).fill("等客户提供发生时间")
  await dialog.getByRole("button", { name: /^确\s*认$/ }).click()
  await expect(lifecycle.getByTestId("ticket-case-status")).toHaveText("等待中")
  await clickCaseAction(lifecycle, "resume")
  dialog = page.getByRole("dialog", { name: "继续处理", exact: true })
  await dialog.getByRole("textbox", { name: "操作说明" }).fill("客户已补齐时间")
  await dialog.getByRole("button", { name: /^确\s*认$/ }).click()
  await expect(lifecycle.getByTestId("ticket-case-status")).toHaveText("分析中")
  await expect(page.getByRole("button", { name: "工程师接单", exact: true }).first()).toBeVisible()
  expect(aggregate.assignment.accepted_at).toBe("")
})

test("switching workbench tickets removes the previous ticket's lifecycle while loading", async ({ page }) => {
  const first = sampleAggregate()
  const second = sampleAggregate()
  Object.assign(second.ticket, { id: 84002, ticket_no: "CASE-UI-006", title: "另一张隔离工单" })
  Object.assign(second.case_lifecycle, { status: "waiting", owner_id: 9002, owner_name: "客服小王", allowed_actions: ["resume"] })
  second.ticket.case_status = "waiting"
  let releaseSecond!: () => void
  const secondReady = new Promise<void>(resolve => { releaseSecond = resolve })
  let secondRequested = false
  await isolate(page, async path => {
    const tickets = [first, second].map(value => ({ ...value.ticket, ...value.assignment, customer_name: "测试客户", actions: value.actions }))
    if (path.endsWith("/workbench/queue")) return { conversations: [], tickets, total: 2, page: 1, page_size: 16, total_pages: 1 }
    if (path.endsWith("/tickets")) return { items: tickets, total: 2, page: 1, page_size: 10, total_pages: 1 }
    if (path.endsWith("/tickets/84001")) return first
    if (path.endsWith("/tickets/84002")) { secondRequested = true; await secondReady; return second }
    if (path.includes("knowledge-candidates") || path.includes("supplier")) return []
  })
  try {
    await page.goto("/enterprise/ticket-workbench?ticket_id=84001", { waitUntil: "domcontentloaded" })
    await page.getByTestId("case-update-status").click()
    await expect(page.getByTestId("case-action-acknowledge")).toBeVisible()
    await page.getByRole("button").filter({ hasText: "CASE-UI-006" }).click()
    await expect.poll(() => secondRequested).toBe(true)
    await expect(page.getByTestId("ticket-case-lifecycle")).toHaveCount(0)
    releaseSecond()
    await expect(page.getByTestId("ticket-case-owner")).toHaveText("客服小王")
    await page.getByTestId("case-update-status").click()
    await expect(page.getByTestId("case-action-resume")).toBeVisible()
    await expect(page.getByTestId("case-action-acknowledge")).toHaveCount(0)
  } finally { releaseSecond() }
})

test("an open lifecycle form cannot submit after a concurrent owner change", async ({ page }) => {
  const aggregate = sampleAggregate()
  Object.assign(aggregate.case_lifecycle, { status: "acknowledged", owner_id: 9001, owner_name: "客服小张", allowed_actions: ["triage", "wait"] })
  aggregate.ticket.case_status = "acknowledged"
  const writes: Array<Record<string, unknown>> = []
  await isolate(page, (path, method, body) => {
    const ticket = { ...aggregate.ticket, ...aggregate.assignment, customer_name: "测试客户", actions: aggregate.actions }
    if (path.endsWith("/workbench/queue")) return { conversations: [], tickets: [ticket], total: 1, page: 1, page_size: 16, total_pages: 1 }
    if (path.endsWith("/tickets")) return { items: [ticket], total: 1, page: 1, page_size: 10, total_pages: 1 }
    if (path.endsWith("/tickets/84001")) return aggregate
    if (method === "POST") { writes.push(body); return {} }
    if (path.includes("knowledge-candidates") || path.includes("supplier")) return []
  })
  await page.goto("/enterprise/ticket-workbench?ticket_id=84001", { waitUntil: "domcontentloaded" })
  await expect(page.getByTestId("ticket-case-status")).toHaveText("已确认受理")
  await clickCaseAction(page.getByTestId("ticket-case-lifecycle"), "wait")
  const dialog = page.getByRole("dialog", { name: "等待中", exact: true })
  await dialog.getByRole("textbox", { name: "操作说明" }).fill("等待客户补充资料")
  Object.assign(aggregate.case_lifecycle, { revision: 2, owner_id: 9002, owner_name: "客服小王" })
  await page.evaluate(() => window.dispatchEvent(new Event("focus")))
  await expect(page.getByTestId("ticket-case-owner")).toHaveText("客服小王")
  await expect(dialog.getByRole("alert")).toHaveText("工单已更新，请取消后按最新状态重新操作。")
  await expect(dialog.getByRole("button", { name: /^确\s*认$/ })).toBeDisabled()
  expect(writes).toHaveLength(0)
})

test("terminal lifecycle retries retain the original command after a lost response", async ({ page }) => {
  const aggregate = sampleAggregate()
  Object.assign(aggregate.ticket, { case_status: "closure_pending", status: "pending_customer_confirm" })
  Object.assign(aggregate.case_lifecycle, { status: "closure_pending", owner_id: 9001, owner_name: "客服小张", allowed_actions: ["close", "reopen"] })
  const writes: Array<{ path: string; body: Record<string, unknown> }> = []
  await isolate(page, (path, method, body) => {
    if (path.endsWith("/tickets/summary")) return { total: 1 }
    if (path.endsWith("/tickets")) return { items: [{ ...aggregate.ticket, ...aggregate.assignment }], total: 1, page: 1, page_size: 10, total_pages: 1 }
    if (path.endsWith("/tickets/84001")) return aggregate
    if (method === "POST") { writes.push({ path, body }); return {} }
  })
  await page.route("**/api/enterprise/v1/tickets/84001/lifecycle", async route => {
    const body = route.request().postDataJSON()
    writes.push({ path: new URL(route.request().url()).pathname, body })
    Object.assign(aggregate.ticket, { case_status: "closed", status: "closed" })
    Object.assign(aggregate.case_lifecycle, { status: "closed", revision: 2, allowed_actions: ["reopen"] })
    if (writes.length === 1) {
      await route.fulfill({ status: 503, json: { success: false, message: "fixture write response lost", data: null } })
    } else {
      await route.fulfill({ json: { success: true, data: { ticket_id: 84001, operation_key: body.idempotency_key, status: "closed", revision: 2 } } })
    }
  })
  await page.goto("/enterprise/tickets", { waitUntil: "domcontentloaded" })
  await page.getByRole("button", { name: "CASE-UI-004", exact: true }).click()
  await clickCaseAction(page.getByTestId("ticket-case-lifecycle"), "close")
  const dialog = page.getByRole("dialog", { name: "确认关闭", exact: true })
  await dialog.getByRole("textbox", { name: "操作说明" }).fill("客户已确认恢复正常")
  await dialog.getByRole("button", { name: /^确\s*认$/ }).click()
  await expect(dialog.getByRole("alert")).toHaveText("fixture write response lost")
  await expect(dialog.getByRole("textbox", { name: "操作说明" })).toBeDisabled()
  await expect(dialog.getByRole("button", { name: "确认", exact: true })).toBeEnabled()
  await expect(dialog.getByRole("button", { name: "确认", exact: true })).toHaveAttribute("aria-busy", "false")
  await dialog.getByRole("button", { name: /^确\s*认$/ }).click()
  await expect(page.getByTestId("ticket-case-status")).toHaveText("已关闭")
  expect(writes).toHaveLength(2)
  expect(writes[0]).toEqual(writes[1])
  expect(writes[0]).toMatchObject({ path: "/api/enterprise/v1/tickets/84001/lifecycle", body: { action: "close", reason: "客户已确认恢复正常", expected_status: "closure_pending", expected_revision: 1 } })
  expect(writes[0].body.idempotency_key).toMatch(/^[\da-f-]{36}$/)
})

test("a mismatched detail response never exposes another ticket's mutation controls", async ({ page }) => {
  const aggregate = sampleAggregate()
  await isolate(page, path => {
    if (path.endsWith("/tickets/summary")) return { total: 1 }
    if (path.endsWith("/tickets")) return { items: [{ ...aggregate.ticket, ...aggregate.assignment }], total: 1, page: 1, page_size: 10, total_pages: 1 }
    if (path.endsWith("/tickets/84001")) return { ...aggregate, ticket: { ...aggregate.ticket, id: 84002, ticket_no: "WRONG-TICKET" } }
  })
  await page.goto("/enterprise/tickets", { waitUntil: "domcontentloaded" })
  await page.getByRole("button", { name: "CASE-UI-004", exact: true }).click()
  await expect(page.getByText("加载的工单与当前选择不一致，请重新打开工单。", { exact: true })).toBeVisible()
  await expect(page.getByTestId("ticket-case-lifecycle")).toHaveCount(0)
  await expect(page.getByText("WRONG-TICKET", { exact: true })).toHaveCount(0)
})

import { expect, test, type Page } from "@playwright/test"

test.use({ video: "off" })
test.setTimeout(60000)

const profile = {
  accessToken: "ticket-email-ui-fixture", domainType: "enterprise", tenantId: 8001,
  user: { id: 9001, username: "mail-fixture", nickname: "客服小张", avatar: "", status: 0, roles: ["service_manager"] },
  roles: ["service_manager"], permissions: ["ticket.view", "ticket.progress"], locale: "zh-CN", timezone: "UTC",
}

function sampleAggregate() {
  return {
    ticket: { id: 84001, ticket_no: "MAIL-UI-001", title: "登录失败", description: "客户邮件建单", status: "pending_dispatch", case_status: "new", priority: "medium", source: "manual", channel: "email", product_id: 0, created_at: "2026-09-16T01:00:00Z", updated_at: "2026-09-16T01:00:00Z", sla_deadline: "", category: "" },
    customer: { id: 8002, name: "测试客户", company: "测试公司", contact: "", email: "customer@example.test" },
    device_context: {}, flow: { current_step: "Accept", steps: [] },
    assignment: { assignee_id: 0, assignee_name: "", team_id: 0, team_name: "", assigned_at: "", accepted_at: "" },
    meeting: {}, repair: { parts: [], resolution: "" }, timeline: [], assets: [], audit_refs: [], actions: {},
  }
}

async function isolate(page: Page, handler: (path: string, method: string, body: Record<string, unknown>) => unknown) {
  await page.routeWebSocket(/\/api\/ws\//, () => {})
  await page.addInitScript(value => {
    localStorage.setItem("remote-helpdesk-session:enterprise", JSON.stringify(value))
    localStorage.setItem("remote-helpdesk-session", JSON.stringify(value))
  }, profile)
  await page.route("**/api/**", async route => {
    const path = new URL(route.request().url()).pathname
    let body: Record<string, unknown> = {}
    try { body = route.request().postDataJSON() ?? {} } catch { /* GET */ }
    const result = handler(path, route.request().method(), body) as unknown
    if (result && typeof result === "object" && "__fail" in (result as Record<string, unknown>)) {
      await route.fulfill({ status: 503, json: { success: false, data: null, message: String((result as Record<string, unknown>).__fail) } })
      return
    }
    let data = result
    if (path === "/api/auth/profile") data = profile
    else if (path.includes("unread")) data = { count: 0, unread_count: 0 }
    else if (path.includes("capabilities")) data = { service_scene: "knowledge_support", features: {} }
    await route.fulfill({ json: data === undefined ? { success: false, data: null, message: "Not provided by isolated UI fixture" } : { success: true, data } })
  })
}

// Only synthetic API responses are used: this test never writes the user's database.
test("email ticket shows both directions and a queued reply survives a failed refresh without resending a new payload", async ({ page }) => {
  const aggregate = sampleAggregate()
  const queued: Array<Record<string, unknown>> = []
  let replies = [{ id: 7001, recipient: "customer@example.test", body: "已收到，正在排查。", status: "sent", created_at: "2026-09-16T02:00:00Z" }]
  const incoming = [{ id: 6001, from: "customer@example.test", subject: "登录失败", body: "登录页面报错", attachment_count: 1, received_at: "2026-09-16T01:00:00Z" }]
  let failNextRead = false
  const errors: string[] = []
  page.on("pageerror", error => errors.push(error.message))

  await isolate(page, (path, method, body) => {
    if (path.endsWith("/tickets/summary")) return { total: 1, pending: 1, processing: 0, awaiting_customer: 0, sla_risk: 0, urgent: 0, done: 0 }
    if (path.endsWith("/tickets")) return { items: [{ ...aggregate.ticket, assignee_name: "", customer_name: "测试客户" }], total: 1, page: 1, page_size: 10, total_pages: 1 }
    if (path.endsWith("/tickets/84001/emails")) {
      if (failNextRead) { failNextRead = false; return { __fail: "邮件记录读取失败" } }
      return { incoming, replies }
    }
    if (path.endsWith("/tickets/84001/email-replies") && method === "POST") {
      queued.push(body)
      const row = { id: 7002, recipient: "customer@example.test", body: body.body, status: "queued", created_at: "2026-09-16T03:00:00Z" }
      replies = [...replies, row]
      return row
    }
    if (path.endsWith("/tickets/84001")) return aggregate
  })

  await page.goto("/enterprise/tickets", { waitUntil: "domcontentloaded" })
  await page.getByRole("button", { name: "MAIL-UI-001", exact: true }).click()

  const panel = page.getByRole("region", { name: "邮件往来" })
  await expect(panel).toBeVisible()
  await expect(panel.getByText("登录页面报错")).toBeVisible()
  await expect(panel.getByText("有 1 个附件，请到原邮箱查看")).toBeVisible()
  await expect(panel.getByText("客户来信 · customer@example.test")).toBeVisible()
  await expect(panel.getByText("客服回信 · customer@example.test")).toBeVisible()
  await expect(panel.getByText("已提交邮件服务器")).toBeVisible()

  await panel.getByLabel("邮件回复内容").fill("请先重置密码后重试。")
  await panel.getByRole("button", { name: "发送邮件回复" }).click()
  await expect.poll(() => queued.length).toBe(1)
  expect(queued[0]).toMatchObject({ body: "请先重置密码后重试。" })
  expect(String(queued[0].request_key)).toMatch(/^[\da-f-]{36}$/)
  await expect(panel.getByText("等待发送")).toBeVisible()

  // A failed refresh must not lose the queued reply or emit a second request.
  failNextRead = true
  await panel.getByRole("button", { name: /刷\s*新/ }).click()
  await expect(panel.getByRole("alert")).toHaveText("邮件记录读取失败")
  expect(queued).toHaveLength(1)
  expect(errors).toEqual([])
})

test("only email-channel tickets expose the mail panel", async ({ page }) => {
  const aggregate = sampleAggregate()
  aggregate.ticket.channel = "manual"
  await isolate(page, path => {
    if (path.endsWith("/tickets/summary")) return { total: 1, pending: 1, processing: 0, awaiting_customer: 0, sla_risk: 0, urgent: 0, done: 0 }
    if (path.endsWith("/tickets")) return { items: [{ ...aggregate.ticket, assignee_name: "", customer_name: "测试客户" }], total: 1, page: 1, page_size: 10, total_pages: 1 }
    if (path.endsWith("/tickets/84001")) return aggregate
  })
  await page.goto("/enterprise/tickets", { waitUntil: "domcontentloaded" })
  await page.getByRole("button", { name: "MAIL-UI-001", exact: true }).click()
  await expect(page.getByRole("region", { name: "邮件往来" })).toHaveCount(0)
})

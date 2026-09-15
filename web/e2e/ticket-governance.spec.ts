import { expect, test, type Page } from "@playwright/test"
import type { GovernanceView } from "../lib/ticket-governance"

test.use({ video: "off" })
test.setTimeout(60000)
const profile = { accessToken: "governance-ui-fixture", domain: "enterprise", domainType: "enterprise", tenantId: 9100, tenant_id: 9100, user: { id: 9001, username: "governance-fixture", nickname: "管理负责人", avatar: "", status: 0, roles: ["service_manager"] }, roles: ["service_manager"], permissions: ["ticket.view", "ticket.create", "ticket.update", "ticket.assign", "ticket.changeStatus"], locale: "zh-CN", timezone: "UTC" }
const policy = { version: "fixture-v1", defaults: { user_case: "p2", incident: "p1", major_incident: "p1", problem: "p3", known_error: "p3", service_request: "p4" }, matrix: [["p1", "p2", "p2"], ["p2", "p2", "p3"], ["p3", "p3", "p4"]] }

async function fixture(page: Page, manager = true, failRefresh = false) {
  const requests: Array<Record<string, unknown>> = []
  const creationRequests: Array<Record<string, unknown>> = []
  const aggregate = {
    ticket: { id: 84001, ticket_no: "GOV-UI-001", title: "支付服务故障", description: "合成测试工单", status: "pending_dispatch", case_status: "new", case_type: "incident", priority_level: "p1", priority: "critical", source: "manual", channel: "enterprise", product_id: 0, created_at: "2026-09-14T01:00:00Z", updated_at: "2026-09-14T01:00:00Z", sla_deadline: "", category: "" },
    customer: { id: 8002, name: "测试客户", company: "测试公司", contact: "", email: "" }, device_context: {}, flow: { current_step: "Accept", steps: [] }, assignment: { assignee_id: 9003, assignee_name: "工程师", team_id: 1, team_name: "处理组" }, meeting: {}, repair: { parts: [], resolution: "" }, timeline: [], assets: [], audit_refs: [], actions: {},
  }
  const view: GovernanceView = { can_associate_duplicate: manager, merge: { can_merge: false, sources: [] }, ticket_id: 84001, revision: 1, case_type: "incident", priority: "p1", suggested: "p2", explanation: "按分类先以 P1 处理；评估建议 P2，等待人工复核", review_required: true, overridden: false, legacy: false, facts: { impact: "medium", urgency: "medium", reach: "medium", safety: "none", workaround: "none", evidence: "核对影响范围", root_cause: "" }, policy, can_manage: manager, can_propose: true, proposals: [], relations: [], history: [] }
  const actorProfile = manager ? profile : { ...profile, roles: ["engineer"], user: { ...profile.user, roles: ["engineer"] }, permissions: ["ticket.view", "ticket.create"] }
  let shouldFail = false
  await page.addInitScript(value => { localStorage.setItem("remote-helpdesk-session:enterprise", JSON.stringify(value)); localStorage.setItem("remote-helpdesk-session", JSON.stringify(value)) }, actorProfile)
  await page.routeWebSocket("**/api/ws/**", socket => socket.close())
  await page.route("**/api/**", async route => {
    const url = new URL(route.request().url()), path = url.pathname
    let data: unknown
    if (path === "/api/auth/profile") data = actorProfile
    else if (path.includes("unread")) data = { count: 0, unread_count: 0 }
    else if (path.includes("capabilities")) data = { service_scene: "knowledge_support", features: {} }
    else if (path.endsWith("/tickets/summary")) data = { total: 1, pending: 1, processing: 0, awaiting_customer: 0, sla_risk: 0, urgent: 1, done: 0 }
    else if (path.endsWith("/tickets/customer-options")) data = [{ customer_id: 8002, display_name: "测试客户" }]
    else if (path.endsWith("/ticket-settings/intake")) data = { rules: [] }
    else if (path.endsWith("/tickets") && route.request().method() === "POST") {
      const body = route.request().postDataJSON(); creationRequests.push(body)
      data = { ...aggregate.ticket, id: 84002, ticket_no: "GOV-CREATED-002", priority_level: body.priority_level || policy.defaults[body.case_type as keyof typeof policy.defaults] }
    }
    else if (path.endsWith("/tickets")) data = { items: [{ ...aggregate.ticket, ...aggregate.assignment, customer_name: "测试客户" }], total: 1, page: 1, page_size: 10, total_pages: 1 }
    else if (path.endsWith("/ticket-settings/classification")) data = policy
    else if (path.endsWith("/tickets/84001")) data = aggregate
    else if (path.endsWith("/governance") && url.searchParams.has("preview_priority")) data = { priority: url.searchParams.get("preview_priority"), previous_deadline: "2026-09-14T02:00:00Z", deadline: "2026-09-14T05:00:00Z", overdue: true }
    else if (path.endsWith("/governance") && url.searchParams.has("duplicate_candidates")) data = [{ other_id: 84002, ticket_no: "GOV-UI-002", title: "支付服务故障", priority: "p2", case_type: "user_case", revision: 7, match: "same_title", created_at: "2026-09-13T01:00:00Z" }]
    else if (path.endsWith("/governance") && url.searchParams.has("candidates")) data = [{ other_id: 84002, ticket_no: "GOV-UI-002", title: "B 站点支付故障" }]
    else if (path.endsWith("/governance")) {
      if (route.request().method() === "POST") {
        const body = route.request().postDataJSON(); requests.push(body)
        view.revision++
        if (body.action === "override") { view.priority = body.priority; view.overridden = true; view.review_required = false; aggregate.ticket.priority_level = body.priority; aggregate.ticket.priority = "high" }
        if (body.action === "duplicate_child") { view.can_associate_duplicate = false; view.relations = [{ id: 1, kind: "parent", source_id: 84002, target_id: 84001, other_id: 84002, ticket_no: "GOV-UI-002", title: "支付服务故障", case_type: "user_case", priority: "p2", status: "new", owner_id: 9001, sla_breached: false, reason: "同一问题", can_remove: true }] }
        shouldFail = failRefresh
        data = { ticket_id: 84001, revision: view.revision }
      } else {
        if (shouldFail) { shouldFail = false; await route.fulfill({ status: 503, json: { success: false, error: { message: "fixture refresh failed" } } }); return }
        data = view
      }
    }
    await route.fulfill({ json: data === undefined ? { success: false, data: null, message: "Not available in isolated fixture" } : { success: true, data } })
  })
  await page.goto("/enterprise/tickets", { waitUntil: "domcontentloaded" })
  await page.getByRole("button", { name: "GOV-UI-001", exact: true }).click()
  await expect(page.getByTestId("ticket-governance")).toBeVisible()
  return { requests, view, creationRequests, aggregate }
}

test("priority override requires a reason and refresh retry does not repeat the mutation", async ({ page }) => {
  const { requests } = await fixture(page, true, true)
  await page.getByTestId("ticket-governance").getByRole("button", { name: "修改紧急程度", exact: true }).click()
  const dialog = page.getByRole("dialog", { name: "修改紧急程度", exact: true })
  await dialog.getByRole("button", { name: "确认保存" }).click()
  await expect(dialog.getByRole("alert")).toContainText("请填写具体原因")
  expect(requests).toHaveLength(0)
  await expect(dialog.getByRole("combobox")).toHaveCount(1)
  await expect(dialog.getByRole("textbox")).toHaveCount(1)
  await dialog.locator(".ant-select-selector").click()
  await page.getByTitle("P2 高", { exact: true }).click()
  await dialog.getByRole("textbox", { name: "修改原因" }).fill("仅影响一个站点，已经验证备用流程")
  await expect(page.locator(".ant-select-dropdown:visible")).toHaveCount(0)
  await expect(dialog.getByRole("alert")).toBeHidden()
  await dialog.screenshot({ path: "test-results/tkt005-simple-change.png" })
  await dialog.getByRole("button", { name: "确认保存" }).click()
  await expect(dialog.getByRole("alert")).toBeVisible()
  await dialog.getByRole("button", { name: "刷新结果" }).click()
  await expect(dialog).toBeHidden()
  expect(requests).toHaveLength(1)
  expect(requests[0]).toMatchObject({ action: "override", priority: "p2", expected_revision: 1, reason: "仅影响一个站点，已经验证备用流程" })
  expect(requests[0]).not.toHaveProperty("category")
  expect(requests[0]).not.toHaveProperty("facts")
  await expect(page.getByTestId("ticket-governance")).toContainText("人工调整")
  await expect(page.getByTestId("ticket-governance")).not.toContainText("需要复核")
  await expect(page.getByText("Not available in isolated fixture", { exact: true }).first()).toBeHidden({ timeout: 15000 })
  await expect(page.getByText("P0 紧急", { exact: true })).toHaveCount(0)
  await page.screenshot({ path: "test-results/tkt005-simple-detail.png", fullPage: true })
})

test("users without edit permission have no priority mutation or proposal actions", async ({ page }) => {
  const { requests } = await fixture(page, false)
  const panel = page.getByTestId("ticket-governance")
  for (const name of ["修改紧急程度", "提出修改建议", "同意建议", "重新评估", "采纳系统建议"]) {
    await expect(panel.getByRole("button", { name, exact: true })).toHaveCount(0)
  }
  expect(requests).toHaveLength(0)
  await expect(panel).toContainText("P1")
})

test("duplicate association selects first parent and retains child urgency and state", async ({ page }) => {
  const { requests, aggregate, view } = await fixture(page)
  await page.getByTestId("ticket-governance").getByRole("button", { name: "关联父工单" }).click()
  const dialog = page.getByRole("dialog", { name: "关联父工单" })
  await expect(dialog.getByRole("button", { name: "确认关联" })).toBeDisabled()
  expect(requests).toHaveLength(0)
  const searchRequest = page.waitForRequest(r => new URL(r.url()).searchParams.get("search") === "GOV-UI-002")
  await dialog.getByRole("combobox", { name: "父工单", exact: true }).fill("GOV-UI-002")
  await searchRequest
  await page.locator(".ant-select-item-option").filter({ hasText: "GOV-UI-002 · 支付服务故障" }).click()
  await expect(dialog).not.toContainText("当前工单保留自己的内容、状态和处理时限")
  await expect(dialog.locator("textarea")).toHaveCount(0)
  await expect(dialog.getByRole("combobox")).toHaveCount(1)
  await expect(page.locator(".ant-select-dropdown:visible")).toHaveCount(0)
  await expect(dialog).not.toContainText("选择关联工单")
  await dialog.screenshot({ path: "test-results/ticket-duplicate-parent-confirm.png" })
  await dialog.getByRole("button", { name: "确认关联" }).click()
  await expect(dialog).toBeHidden()
  expect(requests).toHaveLength(1)
  expect(requests[0]).toMatchObject({ action: "duplicate_child", target_id: 84002, target_revision: 7, expected_revision: 1, reason: "" })
  expect(aggregate.ticket.case_status).toBe("new")
  expect(view.priority).toBe("p1")
  await expect(page.getByTestId("ticket-governance")).toContainText("父工单")
  await expect(page.getByTestId("ticket-governance").getByRole("link", { name: "GOV-UI-002 · 支付服务故障" })).toHaveAttribute("href", "/enterprise/ticket-workbench?ticket_id=84002")
  await expect(page.getByTestId("ticket-governance").getByRole("button", { name: "关联父工单" })).toHaveCount(0)
})

test("parent search shows empty results and ignores a late previous query", async ({ page }) => {
  await fixture(page)
  let releaseOld!: () => void
  const oldGate = new Promise<void>(resolve => { releaseOld = resolve })
  let oldReturned!: () => void
  const oldDone = new Promise<void>(resolve => { oldReturned = resolve })
  await page.route("**/governance?**", async route => {
    const url = new URL(route.request().url())
    if (!url.searchParams.has("duplicate_candidates")) { await route.fallback(); return }
    const query = url.searchParams.get("search") || ""
    if (query === "OLD") await oldGate
    const data = ["OLD", "NEW"].includes(query) ? [{ other_id: query === "OLD" ? 84002 : 84003, ticket_no: query, title: "父工单搜索结果", revision: 7 }] : []
    await route.fulfill({ json: { success: true, data } })
    if (query === "OLD") oldReturned()
  })
  await page.getByTestId("ticket-governance").getByRole("button", { name: "关联父工单" }).click()
  const dialog = page.getByRole("dialog", { name: "关联父工单" })
  const input = dialog.getByRole("combobox", { name: "父工单", exact: true })
  await input.fill("NONE")
  await expect(page.getByText("未找到可关联的父工单", { exact: true })).toBeVisible()
  await expect(dialog.getByRole("button", { name: "确认关联" })).toBeDisabled()
  const oldStarted = page.waitForRequest(r => new URL(r.url()).searchParams.get("search") === "OLD")
  await input.fill("OLD")
  await oldStarted
  await input.fill("NEW")
  await expect(page.locator(".ant-select-item-option").filter({ hasText: "NEW · 父工单搜索结果" })).toBeVisible()
  releaseOld()
  await oldDone
  await page.locator(".ant-select-item-option").filter({ hasText: "NEW · 父工单搜索结果" }).click()
  await expect(dialog).toContainText("NEW · 父工单搜索结果")
  await expect(dialog.getByRole("button", { name: "确认关联" })).toBeEnabled()
})

test("merged source descriptions, activity and attachment provenance remain queryable", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 1200 })
  const { view } = await fixture(page)
  view.merge = { can_merge: true, sources: [{ ticket_id: 84002, ticket_no: "GOV-SOURCE-002", title: "同一问题再次报障", description: "另一条报告补充了错误码 E42 和复现步骤。", author: "提交人", created_at: "2026-09-14T02:00:00Z", timeline: [{ id: 9, type: "progress", content: "核对复现步骤，确认同一故障", actor: "工程师", timestamp: "2026-09-14T03:00:00Z" }], assets: [{ id: 90, file_name: "错误截图.png", uploaded_by: "提交人", uploaded_at: "2026-09-14T02:01:00Z" }] }] }
  await page.reload()
  await page.getByRole("button", { name: "GOV-UI-001", exact: true }).click()
  const panel = page.getByTestId("ticket-governance")
  await expect(panel).toContainText("合并来源 (1)")
  await expect(panel).toContainText("另一条报告补充了错误码 E42")
  await panel.getByText("原处理记录 (1)", { exact: true }).click()
  await expect(panel).toContainText("核对复现步骤，确认同一故障")
  await expect(panel).toContainText("错误截图.png")
  await expect(panel.getByRole("link", { name: "GOV-SOURCE-002", exact: true })).toHaveAttribute("href", "/enterprise/ticket-workbench?ticket_id=84002")
  await panel.screenshot({ path: "test-results/ticket-merge-sources.png" })
})

test("new ticket classification shows confirmed P1 and P2 defaults", async ({ page }) => {
  await fixture(page)
  await page.goto("/enterprise/tickets?parent_ticket_id=84001", { waitUntil: "domcontentloaded" })
  const fields = page.getByTestId("classification-fields")
  await expect(fields.locator(".ant-select-selection-item").nth(1)).toContainText("P2")
  await expect(fields.getByRole("combobox")).toHaveCount(2)
  await expect(fields.getByRole("textbox")).toHaveCount(0)
  await expect(fields).not.toContainText("待核对")
  await fields.locator(".ant-select-selector").first().click()
  await page.getByTitle("故障事件", { exact: true }).click()
  await expect(fields.locator(".ant-select-selection-item").nth(1)).toContainText("P1")
  await fields.locator(".ant-select-selector").first().click()
  await page.getByTitle("重大事件", { exact: true }).click()
  await expect(fields.locator(".ant-select-selection-item").nth(1)).toContainText("P1")
})

test("all classifications need no assessment and show the correct urgency", async ({ page }) => {
  await fixture(page)
  await page.goto("/enterprise/tickets?parent_ticket_id=84001", { waitUntil: "domcontentloaded" })
  const fields = page.getByTestId("classification-fields")
  for (const [name, level] of [["服务请求", "P4"], ["已知错误", "P3"], ["根因问题", "P3"], ["故障事件", "P1"], ["重大事件", "P1"], ["客户问题", "P2"]]) {
    await fields.locator(".ant-select-selector").first().click()
    await page.getByTitle(name, { exact: true }).click()
    await expect(fields.locator(".ant-select-selection-item").nth(1)).toContainText(level)
    await expect(fields.getByRole("combobox")).toHaveCount(2)
    await expect(fields.getByRole("textbox")).toHaveCount(0)
  }
  await fields.screenshot({ path: "test-results/tkt005-simple-create.png" })
})

test("classification correction keeps the form free of urgency hints", async ({ page }) => {
  const { view } = await fixture(page)
  view.overridden = true
  view.priority = "p2"
  await page.reload()
  await page.getByRole("button", { name: "GOV-UI-001", exact: true }).click()
  await page.getByTestId("ticket-governance").getByRole("button", { name: "确认或更正分类" }).click()
  const dialog = page.getByRole("dialog", { name: "确认或更正分类" })
  await dialog.locator(".ant-select-selector").click()
  await page.getByTitle("服务请求", { exact: true }).click()
  await expect(dialog).not.toContainText("紧急程度:")
  await expect(dialog).not.toContainText("保留人工调整的等级")
  await expect(dialog.getByRole("checkbox")).toHaveCount(0)
})

test("pending historical proposals remain readable without approval actions", async ({ page }) => {
  const { view, requests } = await fixture(page)
  view.proposals.push({ id: 77, revision: 1, priority: "p4", category: "impact", reason: "历史待审批意见", status: "pending", review_reason: "", proposed_by: 9003 })
  await page.reload()
  await page.getByRole("button", { name: "GOV-UI-001", exact: true }).click()
  const panel = page.getByTestId("ticket-governance")
  await panel.getByText("历史修改建议 (1)", { exact: true }).click()
  await expect(panel).toContainText("历史待审批意见")
  await expect(panel).toContainText("未生效")
  for (const name of ["同意建议", "驳回建议", "提出修改建议"]) await expect(panel.getByRole("button", { name })).toHaveCount(0)
  expect(requests).toHaveLength(0)
})

test("creation lets an editor choose urgency with a reason and sends it atomically", async ({ page }) => {
  const { creationRequests } = await fixture(page)
  await page.goto("/enterprise/tickets", { waitUntil: "domcontentloaded" })
  await page.getByRole("button", { name: "新建工单", exact: true }).click()
  const dialog = page.getByRole("dialog", { name: "新建工单", exact: true })
  await dialog.locator(".ant-form-item").filter({ has: page.getByText("客户", { exact: true }) }).locator(".ant-select-selector").click()
  await page.getByTitle("测试客户", { exact: true }).click()
  await dialog.getByPlaceholder("请输入工单标题").fill("手动选择 P1 的新工单")
  await dialog.getByPlaceholder("请描述需要处理的问题或服务请求").fill("关键业务中断，需要立即处理。")
  const fields = dialog.getByTestId("classification-fields")
  await expect(fields.getByRole("textbox")).toHaveCount(0)
  await fields.locator(".ant-select-selector").nth(1).click()
  await page.getByTitle("P1 紧急", { exact: true }).click()
  await expect(fields.getByRole("textbox", { name: "修改原因" })).toBeVisible()
  await dialog.getByRole("button", { name: "创建工单", exact: true }).click()
  await expect(dialog).toContainText("请填写具体原因")
  expect(creationRequests).toHaveLength(0)
  await fields.getByRole("textbox", { name: "修改原因" }).fill("关键业务全部中断，需要立即处理")
  await fields.locator(".ant-select-selector").first().click()
  await page.getByTitle("服务请求", { exact: true }).click()
  await expect(fields.locator(".ant-select-selection-item").nth(1)).toContainText("P1")
  await expect(page.locator(".ant-select-dropdown:visible")).toHaveCount(0)
  await fields.screenshot({ path: "test-results/tkt005-create-editable-urgency.png" })
  await dialog.getByRole("button", { name: "创建工单", exact: true }).click()
  await expect(dialog).toBeHidden()
  expect(creationRequests).toHaveLength(1)
  expect(creationRequests[0]).toMatchObject({ case_type: "service_request", priority_level: "p1", priority_reason: "关键业务全部中断，需要立即处理" })
})

test("new-ticket manual urgency can return to automatic classification defaults", async ({ page }) => {
  await fixture(page)
  await page.goto("/enterprise/tickets?parent_ticket_id=84001", { waitUntil: "domcontentloaded" })
  const fields = page.getByTestId("classification-fields")
  await fields.locator(".ant-select-selector").nth(1).click()
  await page.getByTitle("P4 低", { exact: true }).click()
  await fields.getByRole("textbox", { name: "修改原因" }).fill("可稍后处理")
  await fields.locator(".ant-select-selector").nth(1).click()
  await page.getByTitle("P2 高", { exact: true }).click()
  await expect(fields.locator(".ant-select-selection-item").nth(1)).toContainText("P2")
  await expect(fields.getByRole("textbox")).toHaveCount(0)
  await fields.locator(".ant-select-selector").first().click()
  await page.getByTitle("故障事件", { exact: true }).click()
  await expect(fields.locator(".ant-select-selection-item").nth(1)).toContainText("P1")
})

test("create-only users cannot override urgency in the new-ticket form", async ({ page }) => {
  await fixture(page, false)
  await page.goto("/enterprise/tickets?parent_ticket_id=84001", { waitUntil: "domcontentloaded" })
  const fields = page.getByTestId("classification-fields")
  await expect(fields.getByRole("combobox").nth(1)).toBeDisabled()
  await expect(fields).toContainText("需要工单编辑权限")
})


test("relation history shows original deadlines and preserved overdue evidence", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 1200 })
  const { view } = await fixture(page)
  view.history = [{ id: 1, action: "duplicate_child", reason: "已确认同一问题", actor_id: 9001, created_at: "2026-09-14T10:00:00Z", details: { sla_effect: "unchanged", clock_reset: false, sla_snapshot: { created_at: "2026-09-13T01:00:00Z", previous_sla_deadline: "2026-09-14T01:00:00Z", sla_deadline: "2026-09-14T01:00:00Z", previous_accept_deadline: "2026-09-13T02:00:00Z", accept_deadline: "2026-09-13T02:00:00Z", resolution_overdue: true, accept_overdue: true } } }]
  await page.reload()
  await page.getByRole("button", { name: "GOV-UI-001", exact: true }).click()
  const panel = page.getByTestId("ticket-governance")
  await panel.locator("details").filter({ hasText: "已确认同一问题" }).first().locator("summary").first().click()
  await panel.locator("details details summary").last().click()
  await expect(panel).toContainText("保持原计时，不重新计时")
  await expect(panel).toContainText("处理已超时")
  await expect(panel).toContainText("接单已超时")
  await panel.screenshot({ path: "test-results/ticket-duplicate-sla-audit.png" })
})

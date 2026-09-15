import { expect, test, type Page } from "@playwright/test"

test.use({ video: "off" })
const defaultWorkflow = { transitions: {
  new: ["acknowledged", "cancelled"], acknowledged: ["in_triage", "assigned", "waiting", "resolved", "cancelled"],
  in_triage: ["assigned", "waiting", "restored", "resolved", "cancelled"], assigned: ["in_triage", "waiting", "restored", "resolved", "cancelled"],
  waiting: ["acknowledged", "in_triage", "assigned", "restored", "resolved", "closure_pending", "cancelled"], restored: ["in_triage", "waiting", "resolved", "cancelled"],
  resolved: ["closure_pending", "closed", "in_triage"], closure_pending: ["closed", "in_triage"], closed: ["in_triage"], cancelled: [],
} }
type Workflow = { transitions: Record<string, string[]> }
function fixtureView() {
  return {
    active_version_id: 10, deployment_managed: false, can_manage: true, project_key: "*",
    projects: [{ key: "support", name: "知识支持" }], workflow: structuredClone(defaultWorkflow), allowed: defaultWorkflow,
    states: Object.keys(defaultWorkflow.transitions), required: { new: ["acknowledged", "cancelled"], resolved: ["closure_pending"], closure_pending: ["closed"] },
    versions: [{ id: 10, note: "原流程", published: true, workflow: structuredClone(defaultWorkflow) }], next_before_id: 0,
  }
}
const profile = {
  accessToken: "workflow-ui-fixture", domainType: "enterprise", tenantId: 8001,
  user: { id: 9001, username: "workflow-fixture", nickname: "公司管理员", avatar: "", status: 0, roles: ["admin"] },
  roles: ["admin"], permissions: ["ticket.view", "ticket.update"], featureFlags: { ai: false }, locale: "zh-CN", timezone: "UTC",
}
async function isolate(page: Page, handler: (path: string, method: string, body: Record<string, unknown>) => unknown) {
  await page.routeWebSocket(/\/api\/ws\//, () => {})
  await page.addInitScript(value => {
    localStorage.setItem("remote-helpdesk-session:enterprise", JSON.stringify(value))
    localStorage.setItem("remote-helpdesk-session", JSON.stringify(value))
  }, profile)
  await page.route("**/api/**", async route => {
    const path = new URL(route.request().url()).pathname
    let body = {}
    try { body = route.request().postDataJSON() ?? {} } catch { /* GET */ }
    let data = await handler(path, route.request().method(), body)
    if (path === "/api/auth/profile") data = profile
    else if (path.includes("unread")) data = { count: 0, unread_count: 0 }
    else if (path.includes("capabilities")) data = { service_scene: "knowledge_support", features: { ai: false } }
    await route.fulfill({ json: data === undefined ? { success: false, data: null, message: "隔离测试：接口暂不可用" } : { success: true, data } })
  })
}

test("non-AI tenant edits, saves, publishes and restores ticket workflow", async ({ page }) => {
  const view = fixtureView()
  const submissions: Array<Record<string, unknown>> = []
  let draft: Workflow | undefined
  const errors: string[] = []
  page.on("pageerror", e => errors.push(e.message))
  await isolate(page, (path, method, body) => {
    if (path.endsWith("/status-workflow") && method === "GET") return view
    if (path.endsWith("/status-workflow/drafts")) {
      submissions.push(body)
      draft = body.workflow as Workflow
      return { id: 10 + submissions.length }
    }
    if (path.endsWith("/status-workflow/publish")) {
      view.active_version_id = Number(body.version_id)
      Object.assign(view.workflow, draft)
      return { id: view.active_version_id }
    }
  })
  await page.goto("/enterprise/workflow", { waitUntil: "domcontentloaded" })
  const card = page.getByTestId("ticket-status-workflow")
  await expect(card.getByRole("heading", { name: "工单状态流程" })).toBeVisible()
  await card.getByRole("button", { name: "修改流程", exact: true }).click()
  await card.getByLabel("查看状态", { exact: true }).selectOption("new")
  await expect(card.getByRole("checkbox", { name: "新建 → 已确认受理", exact: true })).toBeDisabled()
  await card.getByLabel("查看状态", { exact: true }).selectOption("resolved")
  await card.getByRole("checkbox", { name: "已解决 → 已关闭", exact: true }).uncheck()
  await expect(card.getByRole("button", { name: "发布流程", exact: true })).toBeDisabled()
  await card.getByRole("textbox", { name: "流程修改原因" }).fill("解决后先进入待关闭")
  await card.getByRole("button", { name: "保存草稿", exact: true }).click()
  await expect(card.getByRole("status")).toContainText("尚未生效")
  expect(view.active_version_id).toBe(10)
  await card.getByRole("button", { name: "发布流程", exact: true }).click()
  await expect(card).toContainText("当前配置版本 #11")
  expect(submissions[0].base_version_id).toBe(10)
  expect(submissions[0].project_key).toBe("*")
  expect((submissions[0].workflow as Workflow).transitions.resolved).not.toContain("closed")
  await page.reload({ waitUntil: "domcontentloaded" })
  await expect(card).toContainText("当前配置版本 #11")
  await card.getByText("历史版本与恢复", { exact: true }).click()
  await card.getByRole("button", { name: "载入此流程", exact: true }).click()
  await card.getByLabel("查看状态", { exact: true }).selectOption("resolved")
  await expect(card.getByRole("checkbox", { name: "已解决 → 已关闭", exact: true })).toBeChecked()
  await card.getByRole("button", { name: "保存草稿", exact: true }).click()
  expect(submissions[1].base_version_id).toBe(11)
  await card.getByRole("button", { name: "发布流程", exact: true }).click()
  await expect(card).toContainText("当前配置版本 #12")
  expect(errors).toEqual([])
})

test("read-only users see rules and deployment environments cannot publish from the page", async ({ page }) => {
  const view = fixtureView()
  view.can_manage = false
  await isolate(page, path => path.endsWith("/status-workflow") ? view : undefined)
  await page.goto("/enterprise/workflow", { waitUntil: "domcontentloaded" })
  const card = page.getByTestId("ticket-status-workflow")
  await expect(card).toContainText("修改和发布请由公司管理员操作")
  await expect(card.getByRole("button", { name: "修改流程", exact: true })).toHaveCount(0)
  view.can_manage = true; view.deployment_managed = true
  await page.reload({ waitUntil: "domcontentloaded" })
  await card.getByRole("button", { name: "修改流程", exact: true }).click()
  await expect(card).toContainText("此环境由部署管理")
  await expect(card.getByRole("button", { name: "发布流程", exact: true })).toBeDisabled()
})

test("failed reads do not pretend the built-in workflow is the published configuration", async ({ page }) => {
  await isolate(page, () => undefined)
  await page.goto("/enterprise/workflow", { waitUntil: "domcontentloaded" })
  const card = page.getByTestId("ticket-status-workflow")
  await expect(card.getByRole("alert")).toContainText("接口暂不可用")
  await expect(card.getByRole("button", { name: "修改流程", exact: true })).toHaveCount(0)
  await expect(card).not.toContainText("当前使用系统默认规则")
})

test("old backend responses show an upgrade message instead of crashing", async ({ page }) => {
  const errors: string[] = []
  page.on("pageerror", e => errors.push(e.message))
  await isolate(page, path => path.endsWith("/status-workflow") ? { Status: "published", Version: 1 } : undefined)
  await page.goto("/enterprise/workflow", { waitUntil: "domcontentloaded" })
  await expect(page.getByTestId("ticket-status-workflow").getByRole("alert")).toContainText("升级并重启后端")
  expect(errors).toEqual([])
})

test("canvas connects and removes transitions, protects required paths and keeps ten nodes", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1600, height: 1200 })
  const view = fixtureView()
  let submitted: Workflow | undefined
  await isolate(page, (path, method, body) => {
    if (path.endsWith("/status-workflow") && method === "GET") return view
    if (path.endsWith("/status-workflow/drafts")) { submitted = body.workflow as Workflow; return { id: 11 } }
  })
  await page.goto("/enterprise/workflow", { waitUntil: "domcontentloaded" })
  const card = page.getByTestId("ticket-status-workflow")
  const graph = page.getByTestId("ticket-status-workflow-graph")
  await expect(graph.locator(".react-flow__node")).toHaveCount(10)
  const configuredCount = Object.values(defaultWorkflow.transitions).reduce((count, next) => count + next.length, 0)
  await expect(graph.locator(".react-flow__edge")).toHaveCount(configuredCount)
  await expect(graph).toContainText(`已显示全部 ${configuredCount} 条流转`)
  await graph.screenshot({ path: testInfo.outputPath("workflow-overview.png") })
  await card.getByRole("button", { name: "修改流程", exact: true }).click()
  await card.getByLabel("查看状态", { exact: true }).selectOption("resolved")
  await expect(graph.locator(".react-flow__edge")).toHaveCount(configuredCount)
  await expect(graph.locator('[data-testid="rf__edge-new-cancelled"] .react-flow__edge-path')).toHaveCSS("opacity", "0.18")
  const shortcut = card.getByRole("checkbox", { name: "已解决 → 已关闭", exact: true })
  await shortcut.uncheck()
  await expect(graph.locator(".react-flow__edge")).toHaveCount(configuredCount - 1)
  async function connect(from: string, to: string) {
    const source = graph.locator(`[data-id="${from}"] [data-handleid="out"]`)
    const target = graph.locator(`[data-id="${to}"] [data-handleid="in"]`)
    await source.scrollIntoViewIfNeeded()
    const a = await source.boundingBox(), b = await target.boundingBox()
    if (!a || !b) throw new Error("连接点不可见")
    await page.mouse.move(a.x + a.width / 2, a.y + a.height / 2)
    await page.mouse.down()
    await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2, { steps: 15 })
    await page.mouse.up()
  }
  await connect("resolved", "closed")
  await expect(shortcut).toBeChecked()
  await expect(graph.getByTestId("selected-workflow-edge")).toContainText("已解决 → 已关闭")
  await graph.getByRole("button", { name: "移除此连线", exact: true }).click()
  await expect(shortcut).not.toBeChecked()
  await card.getByRole("textbox", { name: "流程修改原因" }).fill("画布验证：限制直接关闭")
  await card.getByRole("button", { name: "保存草稿", exact: true }).click()
  await expect(card.getByRole("button", { name: "发布流程", exact: true })).toBeEnabled()
  expect(submitted?.transitions.resolved).not.toContain("closed")
  await shortcut.check()
  await expect(card.getByRole("button", { name: "发布流程", exact: true })).toBeDisabled()
  const requiredPath = graph.locator('[data-testid="rf__edge-resolved-closure_pending"] .react-flow__edge-path')
  await graph.locator('[data-id="resolved"]').scrollIntoViewIfNeeded()
  const midpoint = await requiredPath.evaluate(element => {
    const path = element as SVGPathElement
    const point = path.getPointAtLength(path.getTotalLength() / 2)
    const screen = new DOMPoint(point.x, point.y).matrixTransform(path.getScreenCTM()!)
    return { x: screen.x, y: screen.y }
  })
  await page.mouse.click(midpoint.x, midpoint.y)
  await expect(graph.getByTestId("selected-workflow-edge")).toContainText("必要流转，不能移除")
  await expect(graph.getByRole("button", { name: "移除此连线", exact: true })).toHaveCount(0)
  await graph.getByRole("button", { name: "返回总览", exact: true }).click()
  await connect("new", "closed")
  await expect(graph.locator('[data-testid="rf__edge-new-closed"]')).toHaveCount(0)
  await card.getByLabel("查看状态", { exact: true }).selectOption("cancelled")
  await expect(graph).toContainText("此状态结束流程，没有下一步")
  await expect(graph.locator(".react-flow__node")).toHaveCount(10)
})

test("canvas rearranges nodes without changing workflow rules and resets layout", async ({ page }) => {
  await page.setViewportSize({ width: 1600, height: 1200 })
  await isolate(page, path => path.endsWith("/status-workflow") ? fixtureView() : undefined)
  await page.goto("/enterprise/workflow", { waitUntil: "domcontentloaded" })
  const card = page.getByTestId("ticket-status-workflow")
  const graph = page.getByTestId("ticket-status-workflow-graph")
  await card.getByRole("button", { name: "修改流程", exact: true }).click()
  const node = graph.locator('.react-flow__node[data-id="waiting"]')
  await node.scrollIntoViewIfNeeded()
  const before = await node.getAttribute("style")
  const box = await node.boundingBox()
  if (!box) throw new Error("等待节点不可见")
  await page.mouse.move(box.x + box.width / 2, box.y + 20)
  await page.mouse.down()
  await page.mouse.move(box.x + box.width / 2 + 45, box.y + 60, { steps: 10 })
  await page.mouse.up()
  await expect(node).not.toHaveAttribute("style", before!)
  await graph.getByRole("button", { name: "自动排列节点" }).click()
  await expect(node).toHaveAttribute("style", before!)
  await expect(graph.locator(".react-flow__node")).toHaveCount(10)
})

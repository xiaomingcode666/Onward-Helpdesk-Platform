import { expect, test, type Browser, type Page } from "@playwright/test"

const baseUrl = process.env.E2E_BASE_URL ?? "http://localhost:3000"
const frontendUrl = process.env.E2E_FRONTEND_URL ?? baseUrl
const customerUsername = process.env.E2E_CUSTOMER_USERNAME ?? ""
const customerPassword = process.env.E2E_CUSTOMER_PASSWORD ?? ""
const engineerUsername = process.env.E2E_ENGINEER_USERNAME ?? ""
const engineerPassword = process.env.E2E_ENGINEER_PASSWORD ?? ""
const supplierUsername = process.env.E2E_SUPPLIER_USERNAME ?? ""
const supplierPassword = process.env.E2E_SUPPLIER_PASSWORD ?? ""
const adminUsername = process.env.E2E_ADMIN_USERNAME ?? ""
const adminPassword = process.env.E2E_ADMIN_PASSWORD ?? ""

type Portal = "customer" | "partner" | "enterprise" | "platform"

function requireUIFixture() {
  const required = {
    E2E_CUSTOMER_USERNAME: customerUsername,
    E2E_CUSTOMER_PASSWORD: customerPassword,
    E2E_ENGINEER_USERNAME: engineerUsername,
    E2E_ENGINEER_PASSWORD: engineerPassword,
    E2E_SUPPLIER_USERNAME: supplierUsername,
    E2E_SUPPLIER_PASSWORD: supplierPassword,
    E2E_ADMIN_USERNAME: adminUsername,
    E2E_ADMIN_PASSWORD: adminPassword,
  }
  const missing = Object.entries(required).filter(([, value]) => !value).map(([key]) => key)
  if (missing.length > 0) throw new Error(`UI 质量回归缺少环境变量：${missing.join(", ")}`)
}

async function rolePage(
  browser: Browser,
  viewport: { width: number; height: number },
) {
  const context = await browser.newContext({ viewport })
  const page = await context.newPage()
  return { context, page }
}

async function login(
  page: Page,
  portal: Portal,
  nextPath: string,
  username: string,
  password: string,
) {
  await page.goto(`${frontendUrl}/dashboard/login?portal=${portal}&next=${encodeURIComponent(nextPath)}`)
  await page.locator('input[name="username"]').fill(username)
  await page.locator('input[name="password"]').fill(password)
  await page.locator('form button[type="submit"]').click()
  await page.waitForURL((url) => !url.pathname.startsWith("/dashboard/login"), { timeout: 15_000 })
  await expect(page.getByText("无权访问此页面", { exact: true })).toHaveCount(0)
}

async function routeBrowserTrafficToBackend(page: Page) {
  if (frontendUrl === baseUrl) return
  await page.route(`${frontendUrl}/api/**`, async (route) => {
    const source = new URL(route.request().url())
    const response = await route.fetch({ url: `${baseUrl}${source.pathname}${source.search}` })
    await route.fulfill({ response })
  })
  await page.addInitScript(({ backendUrl }) => {
    const backend = new URL(backendUrl)
    const NativeWebSocket = window.WebSocket
    window.WebSocket = class E2EWebSocket extends NativeWebSocket {
      constructor(url: string | URL, protocols?: string | string[]) {
        const target = new URL(String(url), window.location.href)
        if (target.pathname.startsWith("/api/ws/")) {
          target.protocol = backend.protocol === "https:" ? "wss:" : "ws:"
          target.host = backend.host
        }
        if (protocols === undefined) {
          super(target.toString())
        } else {
          super(target.toString(), protocols)
        }
      }
    }
  }, { backendUrl: baseUrl })
}

function isExpectedLocalBackendWebSocketOriginError(message: string) {
  if (frontendUrl === baseUrl) return false
  const backend = new URL(baseUrl)
  const wsProtocol = backend.protocol === "https:" ? "wss:" : "ws:"
  return message.includes(`WebSocket connection to '${wsProtocol}//${backend.host}/api/ws/`)
    && message.includes("Unexpected response code: 403")
}

async function dismissEngineerBriefingIfPresent(page: Page) {
  const dialog = page.getByRole("dialog", { name: /工程师工作确认/ })
  const appeared = await dialog.waitFor({ state: "visible", timeout: 15_000 }).then(() => true).catch(() => false)
  if (appeared) {
    const confirmButton = dialog.getByRole("button", { name: /恢复可接单并进入工作台|进入工作台/ })
    const keepButton = dialog.getByRole("button", { name: /保持当前状态/ })
    if (await confirmButton.isVisible().catch(() => false)) {
      await confirmButton.click()
    } else {
      await keepButton.click()
    }
  }
  await expect(dialog).toBeHidden({ timeout: 10_000 })
}

async function auditPage(page: Page, pageName: string) {
  await page.waitForLoadState("domcontentloaded")
  await expect(page.locator("h1").first(), `${pageName} 应完成加载并显示页面主标题`).toBeVisible({ timeout: 30_000 })
  await expect(page.getByText("Internal Server Error", { exact: true })).toHaveCount(0)
  await expect(page.getByText("Application error", { exact: true })).toHaveCount(0)

  const audit = await page.evaluate(() => {
    const visible = (element: Element) => {
      const html = element as HTMLElement
      if (html.getAttribute("aria-hidden") === "true") return false
      const style = window.getComputedStyle(html)
      const rect = html.getBoundingClientRect()
      return style.display !== "none" && style.visibility !== "hidden" && rect.width > 0 && rect.height > 0
    }
    const explicitAccessibleName = (element: Element) => {
      const labelledBy = element.getAttribute("aria-labelledby")
      const labelledText = labelledBy
        ? labelledBy.split(/\s+/).map((id) => document.getElementById(id)?.textContent || "").join(" ")
        : ""
      return [
        element.getAttribute("aria-label"),
        element.getAttribute("title"),
        labelledText,
        element.querySelector("img")?.getAttribute("alt"),
      ].some((value) => Boolean(value?.trim()))
    }
    const accessibleName = (element: Element) => (
      explicitAccessibleName(element) || Boolean(element.textContent?.trim())
    )
    const controlHasLabel = (element: Element) => {
      if (explicitAccessibleName(element)) return true
      const id = element.getAttribute("id")
      return Boolean(id && document.querySelector(`label[for="${CSS.escape(id)}"]`))
    }
    const emptyButtons = Array.from(document.querySelectorAll("button"))
      .filter((item) => visible(item) && !accessibleName(item))
      .slice(0, 10)
      .map((item) => item.outerHTML.slice(0, 180))
    const unlabeledControls = Array.from(
      document.querySelectorAll('input:not([type="hidden"]), textarea, select, [contenteditable="true"]'),
    )
      .filter((item) => visible(item) && !controlHasLabel(item))
      .slice(0, 10)
      .map((item) => item.outerHTML.slice(0, 180))
    const imagesWithoutAlt = Array.from(document.querySelectorAll("img"))
      .filter((item) => visible(item) && !item.hasAttribute("alt"))
      .slice(0, 10)
      .map((item) => item.getAttribute("src") || "unknown")
    return {
      horizontalOverflow: Math.max(0, document.documentElement.scrollWidth - document.documentElement.clientWidth),
      emptyButtons,
      unlabeledControls,
      imagesWithoutAlt,
      headingCount: document.querySelectorAll("h1").length,
    }
  })

  expect(audit.horizontalOverflow, `${pageName} 不应整页横向溢出`).toBeLessThanOrEqual(1)
  expect(audit.emptyButtons, `${pageName} 存在无名称按钮`).toEqual([])
  expect(audit.unlabeledControls, `${pageName} 存在无标签输入控件`).toEqual([])
  expect(audit.imagesWithoutAlt, `${pageName} 存在缺少 alt 的图片`).toEqual([])
  expect(audit.headingCount, `${pageName} 应包含页面主标题`).toBeGreaterThan(0)
}

test("四类角色核心页面满足响应式与基础无障碍门槛", async ({ browser }) => {
  test.setTimeout(180_000)
  requireUIFixture()
  const customer = await rolePage(browser, { width: 390, height: 844 })
  const engineer = await rolePage(browser, { width: 1024, height: 768 })
  const supplier = await rolePage(browser, { width: 1024, height: 768 })
  const admin = await rolePage(browser, { width: 1280, height: 800 })
  const apiFailures: string[] = []
  const consoleErrors: string[] = []
  const webSocketUrls: string[] = []

  for (const role of [customer, engineer, supplier, admin]) {
    await routeBrowserTrafficToBackend(role.page)
    role.page.on("response", (response) => {
      if (response.url().includes("/api/") && response.status() >= 500) {
        apiFailures.push(`${response.status()} ${response.request().method()} ${response.url()}`)
      }
    })
    role.page.on("websocket", (socket) => {
      webSocketUrls.push(socket.url())
    })
    role.page.on("console", (message) => {
      if (message.type() === "error") {
        const errorText = `${role.page.url()} ${message.text()}`
        if (!isExpectedLocalBackendWebSocketOriginError(errorText)) {
          consoleErrors.push(errorText)
        }
      }
    })
  }

  try {
    await login(customer.page, "customer", "/customer/chat", customerUsername, customerPassword)
    await expect(customer.page.getByRole("heading", { name: "会话工作台" })).toBeVisible()
    await auditPage(customer.page, "客户会话")

    await login(engineer.page, "enterprise", "/enterprise/ticket-workbench", engineerUsername, engineerPassword)
    await dismissEngineerBriefingIfPresent(engineer.page)
    await expect(engineer.page.getByRole("heading", { name: "会话" })).toBeVisible()
    await auditPage(engineer.page, "工程师会话工作台")

    await login(supplier.page, "partner", "/partner/tickets", supplierUsername, supplierPassword)
    await auditPage(supplier.page, "供应商工单")

    await login(admin.page, "enterprise", "/enterprise", adminUsername, adminPassword)
    await auditPage(admin.page, "企业服务工作台")
    await admin.page.goto(`${frontendUrl}/enterprise/ai`)
    await auditPage(admin.page, "AI 接待机器人")
    await admin.page.goto(`${frontendUrl}/enterprise/workflow`)
    await auditPage(admin.page, "工作流引擎")

    expect(apiFailures, "核心页面不应出现 5xx API").toEqual([])
    expect(consoleErrors, "核心页面不应输出 console.error").toEqual([])
    expect(webSocketUrls.length, "核心页面应建立实时连接").toBeGreaterThan(0)
    expect(
      webSocketUrls.filter((url) => {
        const params = new URL(url).searchParams
        return params.has("accessToken") || params.has("customerSessionToken")
      }),
      "WebSocket URL 不应携带访问令牌或客户会话令牌",
    ).toEqual([])
  } finally {
    await Promise.all([customer.context.close(), engineer.context.close(), supplier.context.close(), admin.context.close()])
  }
})

test("企业详情入口使用可刷新、可恢复的静态路由", async ({ browser }) => {
  test.setTimeout(90_000)
  if (!adminUsername || !adminPassword) {
    throw new Error("企业详情路由回归缺少 E2E_ADMIN_USERNAME 或 E2E_ADMIN_PASSWORD")
  }

  const admin = await rolePage(browser, { width: 1280, height: 800 })
  await routeBrowserTrafficToBackend(admin.page)
  try {
    await login(admin.page, "platform", "/platform/tenants", adminUsername, adminPassword)
    await admin.page.goto(`${frontendUrl}/platform/tenants`)
    const defaultTenantRow = admin.page.getByRole("row", { name: /Default Tenant/ }).first()
    await expect(defaultTenantRow).toBeVisible()
    await defaultTenantRow.getByRole("button", { name: "进入企业端" }).click()
    await admin.page.waitForURL((url) => url.pathname === "/enterprise", { timeout: 15_000 })
    await admin.page.goto(`${frontendUrl}/enterprise/workflow`)
    await expect(admin.page.getByRole("button", { name: "查看方案" }).first()).toBeVisible()
    await admin.page.getByRole("button", { name: "查看方案" }).first().click()
    await expect(admin.page).toHaveURL(/\/enterprise\/workflow\?workflowId=\d+$/)
    await expect(admin.page.locator("h1").first()).toBeVisible()

    await admin.page.goto(`${frontendUrl}/enterprise/ai`)
    await expect(admin.page.getByRole("button", { name: "配置", exact: true }).first()).toBeVisible()
    await admin.page.getByRole("button", { name: "配置", exact: true }).first().click()
    await expect(admin.page).toHaveURL(/\/enterprise\/ai\?agentId=\d+$/)
    await expect(admin.page.locator("h1").first()).toBeVisible()

    const legacyRoutes = [
      ["/enterprise/workflow/8", "/enterprise/workflow?workflowId=8"],
      ["/enterprise/ai/8", "/enterprise/ai?agentId=8"],
      ["/enterprise/products/8", "/enterprise/products?product_id=8"],
      ["/enterprise/tickets/8", "/enterprise/ticket-workbench?ticket_id=8"],
    ] as const
    for (const [source, destination] of legacyRoutes) {
      const response = await admin.page.request.get(`${frontendUrl}${source}`, { maxRedirects: 0 })
      expect(response.status(), `${source} 应由 Web 层恢复`).toBe(302)
      const location = response.headers().location
      expect(location, `${source} 应使用保留当前 origin 的相对重定向`).toMatch(/^\//)
      expect(location).toContain(destination)
    }

    for (const staticResource of [
      "/enterprise/workflow/__next._tree.txt",
      "/enterprise/ai/__next._tree.txt",
      "/enterprise/products/__next._tree.txt",
      "/enterprise/tickets/__next._tree.txt",
    ]) {
      const response = await admin.page.request.get(`${frontendUrl}${staticResource}`, { maxRedirects: 0 })
      expect(response.status(), `${staticResource} 不应被业务 URL 兜底误重定向`).toBe(200)
    }
  } finally {
    await admin.page.unrouteAll({ behavior: "ignoreErrors" })
    await admin.context.close()
  }
})

test("企业报表采购决策指标区分零值和无数据", async ({ browser }) => {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
  const page = await context.newPage()
  const session = {
    accessToken: "reports-ui-test-token",
    user: {
      id: 1,
      username: "reports-ui-test",
      nickname: "报表测试管理员",
      avatar: "",
      status: 0,
      roles: ["admin"],
    },
    permissions: ["report.view"],
    roles: ["admin"],
    tenantId: 1,
    domainType: "enterprise",
    supportMode: "platform_tenant",
  }

  await page.addInitScript((value) => {
    window.localStorage.setItem("remote-helpdesk-session", JSON.stringify(value))
  }, session)
  await page.route(`${frontendUrl}/api/**`, async (route) => {
    const pathname = new URL(route.request().url()).pathname
    let data: unknown = []

    if (pathname === "/api/auth/profile") {
      data = session
    } else if (pathname === "/api/enterprise/v1/reports/overview") {
      data = {
        total_products: 2,
        total_devices: 8,
        total_tickets: 12,
        pending_tickets: 1,
        in_progress_tickets: 2,
        sla_at_risk: 0,
        new_today: 0,
        ai_sessions: 4,
        ai_resolve_rate: 0,
        expert_intervention_rate: 0,
        avoided_trips: 0,
        first_time_fix_rate: null,
        avg_downtime_minutes: null,
        knowledge_reuse_rate: 0,
        remote_resolution_rate: 42.5,
        outcome_metric_coverage_rate: 50,
        outcome_metric_sample_size: 6,
        recent_activities: [],
        queue_tickets: [],
      }
    }

    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ success: true, errorCode: 0, message: "ok", data }),
    })
  })

  try {
    await page.goto(`${frontendUrl}/enterprise/reports`)

    await expect(page.getByRole("heading", { name: "售后服务报表", exact: true })).toBeVisible()
    await expect(page.locator("h1")).toHaveCount(1)

    const metrics = page.getByRole("region", { name: "采购决策指标" })
    await expect(metrics).toBeVisible()
    await expect(metrics.getByRole("group", { name: "专家介入率" })).toContainText("0.0%")
    await expect(metrics.getByRole("group", { name: "避免出差次数" })).toContainText("0 次")
    await expect(metrics.getByRole("group", { name: "知识复用率" })).toContainText("0.0%")
    await expect(metrics.getByRole("group", { name: "首次修复率" })).toContainText("暂无数据")
    await expect(metrics.getByRole("group", { name: "平均停机时长" })).toContainText("暂无数据")
    await expect(metrics.getByRole("group", { name: "远程解决率" })).toContainText("42.5%")
    await expect(metrics).toContainText("样本 6 · 覆盖 50.0%")
    await expect(metrics.locator("article")).toHaveCount(0)

    await page.setViewportSize({ width: 390, height: 844 })
    await expect(metrics).toBeVisible()
    const horizontalOverflow = await page.evaluate(() => (
      Math.max(0, document.documentElement.scrollWidth - document.documentElement.clientWidth)
    ))
    expect(horizontalOverflow, "采购指标在移动端不应造成整页横向溢出").toBeLessThanOrEqual(1)
  } finally {
    await context.close()
  }
})

test("企业报表接口未返回时先展示页面和模块加载状态", async ({ browser }) => {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
  const page = await context.newPage()
  const session = {
    accessToken: "reports-loading-test-token",
    user: {
      id: 1,
      username: "reports-loading-test",
      nickname: "报表加载测试管理员",
      avatar: "",
      status: 0,
      roles: ["admin"],
    },
    permissions: ["report.view"],
    roles: ["admin"],
    tenantId: 1,
    domainType: "enterprise",
    supportMode: "platform_tenant",
  }
  let releaseReports = () => {}
  const reportsGate = new Promise<void>((resolve) => {
    releaseReports = resolve
  })

  await page.addInitScript((value) => {
    window.localStorage.setItem("remote-helpdesk-session", JSON.stringify(value))
  }, session)
  await page.route(`${frontendUrl}/api/**`, async (route) => {
    const pathname = new URL(route.request().url()).pathname
    let data: unknown = []

    if (pathname === "/api/auth/profile") {
      data = session
    } else if (pathname.startsWith("/api/enterprise/v1/reports/")) {
      await reportsGate
      data = pathname.endsWith("/overview") ? {
        total_products: 0,
        total_devices: 0,
        total_tickets: 0,
        pending_tickets: 0,
        in_progress_tickets: 0,
        sla_at_risk: 0,
        new_today: 0,
        ai_sessions: 0,
        ai_resolve_rate: 0,
        expert_intervention_rate: null,
        avoided_trips: null,
        first_time_fix_rate: null,
        avg_downtime_minutes: null,
        knowledge_reuse_rate: null,
        remote_resolution_rate: null,
        outcome_metric_coverage_rate: null,
        outcome_metric_sample_size: 0,
        recent_activities: [],
        queue_tickets: [],
      } : []
    }

    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ success: true, errorCode: 0, message: "ok", data }),
    })
  })

  try {
    await page.goto(`${frontendUrl}/enterprise/reports`)

    await expect(page.getByRole("heading", { name: "售后服务报表", exact: true })).toBeVisible()
    await expect(page.getByRole("status", { name: "关键指标加载中" })).toBeVisible()
    await expect(page.getByRole("status", { name: "采购决策指标加载中" })).toBeVisible()
    await expect(page.getByRole("status", { name: "报表明细加载中" })).toBeVisible()

    releaseReports()
    await expect(page.getByRole("status", { name: "关键指标加载中" })).toBeHidden()
    await expect(page.getByRole("region", { name: "采购决策指标" })).toBeVisible()
  } finally {
    releaseReports()
    await context.close()
  }
})

test("供应商门户接口未返回时保留工作台标题和结构", async ({ browser }) => {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } })
  const page = await context.newPage()
  const session = {
    accessToken: "partner-loading-test-token",
    user: {
      id: 2,
      username: "partner-loading-test",
      nickname: "供应商加载测试",
      avatar: "",
      status: 0,
      roles: ["partner_engineer"],
    },
    permissions: ["ticket.view", "meeting.view"],
    roles: ["partner_engineer"],
    tenantId: 1,
    domainType: "partner",
    supportMode: "partner_portal",
  }
  let releasePartner = () => {}
  const partnerGate = new Promise<void>((resolve) => {
    releasePartner = resolve
  })

  await page.addInitScript((value) => {
    window.localStorage.setItem("remote-helpdesk-session", JSON.stringify(value))
  }, session)
  await page.route(`${frontendUrl}/api/**`, async (route) => {
    const pathname = new URL(route.request().url()).pathname
    let data: unknown = []

    if (pathname === "/api/auth/profile") {
      data = session
    } else if (pathname === "/api/partner/v1/tickets") {
      await partnerGate
    } else if (pathname === "/api/partner/v1/meetings") {
      await partnerGate
      data = {
        summary: { active: 0, waiting: 0, ended: 0, mine: 0, participants_online: 0 },
        items: [],
      }
    }

    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ success: true, errorCode: 0, message: "ok", data }),
    })
  })

  try {
    await page.goto(`${frontendUrl}/partner`)

    await expect(page.getByRole("heading", { name: "供应商工作台", exact: true })).toBeVisible()
    await expect(page.getByRole("status", { name: "供应商工作台概览加载中" })).toBeVisible()
    await expect(page.getByRole("status", { name: "供应商工作台内容加载中" })).toBeVisible()

    releasePartner()
    await expect(page.getByRole("status", { name: "供应商工作台概览加载中" })).toBeHidden()
    await expect(page.getByRole("heading", { name: "供应商工作台", exact: true })).toBeVisible()
  } finally {
    releasePartner()
    await context.close()
  }
})

test("供应商工作台按管理员与工程师身份提供统计和派工能力", async ({ browser }) => {
  const now = Date.now()
  const sessionFor = (role: "partner_admin" | "partner_engineer") => ({
    accessToken: `partner-overview-${role}-token`,
    user: {
      id: role === "partner_admin" ? 31 : 32,
      username: role,
      nickname: role === "partner_admin" ? "供应商管理员" : "供应商工程师",
      avatar: "",
      status: 0,
      roles: [role],
    },
    permissions: ["ticket.view", "meeting.view"],
    roles: [role],
    tenantId: 1,
    domainType: "partner",
    supportMode: "partner_portal",
  })
  const accounts = [
    {
      id: 71,
      username: "partner.admin",
      display_name: "供应商管理员",
      email: "",
      phone: "",
      languages_json: "[]",
      status: 0,
      updated_at: new Date(now).toISOString(),
      is_current: true,
      roles: ["partner_admin"],
    },
    {
      id: 72,
      username: "partner.engineer",
      display_name: "现场协作工程师",
      email: "",
      phone: "",
      languages_json: "[]",
      status: 0,
      updated_at: new Date(now).toISOString(),
      is_current: false,
      roles: ["partner_engineer"],
    },
  ]
  const tickets = [
    {
      id: 801,
      ticket_id: 9001,
      conversation_id: 3001,
      ticket_no: "TK-PARTNER-OVERVIEW-001",
      ticket_title: "控制模块异常升温",
      ticket_status: "supplier_support",
      ticket_updated_at: new Date(now - 10 * 60 * 1000).toISOString(),
      product_id: 1,
      product_module_id: 11,
      product_module_name: "控制模块",
      partner_company_id: 51,
      partner_company_name: "测试供应商",
      partner_account_id: 71,
      partner_account_name: "供应商管理员",
      participant_count: 1,
      status: "invited",
      reason: "需要供应商确认控制器参数",
      resolution: "",
      visibility: ["repair_progress"],
      invited_at: new Date(now - 30 * 60 * 1000).toISOString(),
      authorization_ends_at: new Date(now + 6 * 60 * 60 * 1000).toISOString(),
      authorization_active: true,
    },
    {
      id: 802,
      ticket_id: 9002,
      conversation_id: 3002,
      ticket_no: "TK-PARTNER-OVERVIEW-002",
      ticket_title: "压力采集信号漂移",
      ticket_status: "supplier_support",
      ticket_updated_at: new Date(now - 20 * 60 * 1000).toISOString(),
      product_id: 1,
      product_module_id: 12,
      product_module_name: "压力控制模块",
      partner_company_id: 51,
      partner_company_name: "测试供应商",
      partner_account_id: 72,
      partner_account_name: "现场协作工程师",
      participant_count: 2,
      status: "processing",
      reason: "需要核对传感器批次",
      resolution: "",
      visibility: ["repair_progress"],
      invited_at: new Date(now - 2 * 60 * 60 * 1000).toISOString(),
      authorization_ends_at: new Date(now + 7 * 24 * 60 * 60 * 1000).toISOString(),
      authorization_active: true,
    },
  ]

  const openRolePage = async (role: "partner_admin" | "partner_engineer", viewport: { width: number; height: number }) => {
    const context = await browser.newContext({ viewport })
    const page = await context.newPage()
    const session = sessionFor(role)
    await page.addInitScript((value) => {
      window.localStorage.setItem("remote-helpdesk-session", JSON.stringify(value))
    }, session)
    await page.route(`${frontendUrl}/api/**`, async (route) => {
      const request = route.request()
      const pathname = new URL(request.url()).pathname
      let data: unknown = []
      if (pathname === "/api/auth/profile") {
        data = session
      } else if (pathname === "/api/partner/v1/profile") {
        data = {
          account_id: role === "partner_admin" ? 71 : 72,
          user_id: role === "partner_admin" ? 31 : 32,
          display_name: role === "partner_admin" ? "供应商管理员" : "现场协作工程师",
          email: "",
          phone: "",
          company_id: 51,
          company_name: "测试供应商",
          partner_no: "PARTNER-E2E",
          partner_type: "manufacturer",
          country_region: "CN",
          languages_json: "[]",
          authorization_ok: true,
          roles: [role],
          can_manage_team: role === "partner_admin",
          can_invite_team: true,
        }
      } else if (pathname === "/api/partner/v1/tickets") {
        data = role === "partner_admin" ? tickets : tickets.filter((item) => item.partner_account_id === 72)
      } else if (pathname === "/api/partner/v1/accounts") {
        data = accounts.map((account) => ({
          ...account,
          is_current: account.id === (role === "partner_admin" ? 71 : 72),
        }))
      } else if (pathname === "/api/partner/v1/meetings") {
        data = { summary: { active: 1, waiting: 0, ended: 0, mine: 1, participants_online: 0 }, items: [] }
      }
      await route.fulfill({
        contentType: "application/json",
        body: JSON.stringify({ success: true, errorCode: 0, message: "ok", data }),
      })
    })
    await page.goto(`${frontendUrl}/partner`)
    await expect(page.getByRole("heading", { name: "供应商工作台", exact: true })).toBeVisible()
    return { context, page }
  }

  const admin = await openRolePage("partner_admin", { width: 1440, height: 900 })
  try {
    await expect(admin.page.getByText("供应商管理员", { exact: true }).first()).toBeVisible()
    await expect(admin.page.getByRole("region", { name: "供应商协作统计" })).toBeVisible()
    await expect(admin.page.getByTestId("partner-team-load")).toContainText("现场协作工程师")
    const riskCard = admin.page.getByRole("link", { name: /授权风险/ })
    await expect(riskCard).toContainText("1")
    const pendingTicket = admin.page.getByTestId("partner-overview-ticket-801")
    await expect(pendingTicket.getByRole("button", { name: "接单处理", exact: true })).toBeVisible()
    await pendingTicket.getByRole("button", { name: "指派", exact: true }).click()
    const assignmentDialog = admin.page.getByRole("dialog", { name: "指派协作负责人" })
    await assignmentDialog.getByRole("combobox").click()
    const engineerOption = admin.page.getByRole("option", { name: "现场协作工程师 · 当前 1 条", exact: true })
    await expect(engineerOption).toBeVisible()
    await engineerOption.click()
    await assignmentDialog.getByRole("button", { name: "取消", exact: true }).click()
    await expect(assignmentDialog).toBeHidden()
    expect(await admin.page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
    await admin.page.setViewportSize({ width: 390, height: 844 })
    expect(await admin.page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
  } finally {
    await admin.context.close()
  }

  const engineer = await openRolePage("partner_engineer", { width: 390, height: 844 })
  try {
    await expect(engineer.page.locator("main").getByText("供应商工程师", { exact: true })).toBeVisible()
    await expect(engineer.page.getByTestId("partner-team-load")).toHaveCount(0)
    await expect(engineer.page.getByRole("button", { name: "指派", exact: true })).toHaveCount(0)
    await expect(engineer.page.getByRole("button", { name: "协作人员", exact: true })).toHaveCount(0)
    expect(await engineer.page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
  } finally {
    await engineer.context.close()
  }
})

test("正式供应商管理员可在新版首页读取团队统计", async ({ browser }) => {
  test.skip(!supplierUsername || !supplierPassword, "缺少正式供应商账号")
  const { context, page } = await rolePage(browser, { width: 1440, height: 900 })
  await routeBrowserTrafficToBackend(page)

  try {
    await login(page, "partner", "/partner", supplierUsername, supplierPassword)
    const main = page.locator("main")
    await expect(main.getByRole("heading", { name: "供应商工作台", exact: true })).toBeVisible()
    await expect(main.getByText("供应商管理员", { exact: true })).toBeVisible()
    await expect(main.getByRole("region", { name: "供应商协作统计" })).toBeVisible()
    await expect(main.getByTestId("partner-team-load")).toBeVisible()
    await expect(main.getByTestId("partner-overview-queue")).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
    await page.setViewportSize({ width: 390, height: 844 })
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
  } finally {
    await context.close()
  }
})

test("企业 MQTT 连接器使用订阅配置并正确刷新消息状态", async ({ browser }) => {
  test.setTimeout(60_000)
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  const page = await context.newPage()
  const session = {
    accessToken: "access-mqtt-ui-test-token",
    user: {
      id: 1,
      username: "access-mqtt-ui-test",
      nickname: "接入配置管理员",
      avatar: "",
      status: 0,
      roles: ["admin"],
    },
    permissions: ["tenantIntegrationConfig.view", "tenantIntegrationConfig.create", "tenantIntegrationConfig.update"],
    roles: ["admin"],
    tenantId: 1,
    domainType: "enterprise",
    supportMode: "platform_tenant",
  }
  let statusRequestCount = 0
  let httpTestRequestCount = 0
  let createPayload: Record<string, unknown> | null = null

  await page.addInitScript((value) => {
    window.localStorage.setItem("remote-helpdesk-session", JSON.stringify(value))
  }, session)
  await page.route(`${frontendUrl}/api/**`, async (route) => {
    const request = route.request()
    const pathname = new URL(request.url()).pathname
    let data: unknown = []

    if (pathname === "/api/auth/profile") {
      data = session
    } else if (pathname === "/api/enterprise/v1/access/connectors") {
      data = [{
        id: 41,
        name: "生产设备 MQTT",
        connector_type: "mqtt",
        base_url: "mqtts://broker.example.com:8883",
        auth_type: "basic",
        active: true,
        health_status: "unknown",
        last_health_check_at: "",
        created_at: "2026-08-10T09:00:00Z",
      }]
    } else if (pathname === "/api/enterprise/v1/access/connector/41/status") {
      statusRequestCount += 1
      data = {
        connectorId: 41,
        status: "healthy",
        lastChecked: "2026-08-10T10:05:00Z",
        lastMessageAt: "2026-08-10T10:04:30Z",
        latencyMs: 0,
      }
    } else if (pathname === "/api/enterprise/v1/access/connector/create" && request.method() === "POST") {
      createPayload = request.postDataJSON() as Record<string, unknown>
      data = { id: 42, name: "车间设备遥测", status: "active" }
    } else if (pathname.endsWith("/test") && request.method() === "POST") {
      httpTestRequestCount += 1
      data = { success: true }
    }

    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ success: true, errorCode: 0, message: "ok", data }),
    })
  })

  try {
    await page.goto(`${frontendUrl}/enterprise/access`)
    await page.getByRole("button", { name: "业务系统", exact: true }).click()

    const mqttRow = page.getByText("生产设备 MQTT", { exact: true }).locator("xpath=../../..")
    await expect(page.getByText("生产设备 MQTT", { exact: true })).toBeVisible()
    await expect(page.getByText("最近消息", { exact: true })).toBeVisible()
    await expect(mqttRow).toContainText("2026")
    await page.getByRole("button", { name: "检查 生产设备 MQTT 订阅状态" }).click()
    await expect.poll(() => statusRequestCount).toBeGreaterThanOrEqual(2)
    expect(httpTestRequestCount, "MQTT 状态检查不应调用 HTTP test 接口").toBe(0)

    await page.getByRole("button", { name: "新增连接器" }).click()
    const dialog = page.getByRole("dialog", { name: "新增业务系统连接器" })
    await dialog.getByRole("combobox", { name: "连接类型" }).click()
    await page.getByRole("option", { name: "MQTT", exact: true }).click()

    await expect(dialog.getByText("认证方式", { exact: true })).toHaveCount(0)
    await expect(dialog.getByText("Bearer Token", { exact: true })).toHaveCount(0)
    await expect(dialog.getByText("Token 或 JSON 配置", { exact: true })).toHaveCount(0)
    await expect(dialog.getByRole("combobox", { name: "MQTT 未知设备策略" })).toContainText("进入死信队列")

    await dialog.getByRole("textbox", { name: "连接器名称" }).fill("车间设备遥测")
    await dialog.getByRole("textbox", { name: "Broker 地址" }).fill("mqtts://broker.factory.example:8883")
    await dialog.getByRole("textbox", { name: "MQTT 用户名" }).fill("rhd-reader")
    await dialog.getByLabel("MQTT 密码").fill("mqtt-secret-value")
    await dialog.getByRole("textbox", { name: "MQTT Client ID" }).fill("rhd-tenant-1")
    await dialog.getByRole("textbox", { name: "MQTT 订阅主题" }).fill("devices/+/telemetry\ndevices/+/events\ndevices/+/telemetry")
    await dialog.getByRole("combobox", { name: "MQTT QoS" }).click()
    await page.getByRole("option", { name: "QoS 2" }).click()
    await dialog.getByLabel("MQTT Keepalive 秒数").fill("90")
    await dialog.getByRole("switch", { name: "MQTT Clean Session" }).click()
    await dialog.getByRole("textbox", { name: "设备序列号 JSON 路径" }).fill("$.device.serial")
    await dialog.getByRole("textbox", { name: "故障码 JSON 路径" }).fill("$.alarm.code")
    await dialog.getByRole("textbox", { name: "严重级别 JSON 路径" }).fill("$.alarm.severity")
    await dialog.getByRole("textbox", { name: "告警消息 JSON 路径" }).fill("$.alarm.message")
    await dialog.getByRole("textbox", { name: "采集时间 JSON 路径" }).fill("$.timestamp")
    await dialog.getByRole("textbox", { name: "MQTT 指标键 1" }).fill("temperature")
    await dialog.getByRole("textbox", { name: "MQTT 指标 JSON 路径 1" }).fill("$.metrics.temperature")
    await dialog.getByRole("button", { name: "新增指标" }).click()
    await dialog.getByRole("textbox", { name: "MQTT 指标键 2" }).fill("pressure")
    await dialog.getByRole("textbox", { name: "MQTT 指标 JSON 路径 2" }).fill("$.metrics.pressure")

    await page.setViewportSize({ width: 390, height: 844 })
    await expect(dialog).toBeVisible()
    const horizontalOverflow = await page.evaluate(() => (
      Math.max(0, document.documentElement.scrollWidth - document.documentElement.clientWidth)
    ))
    expect(horizontalOverflow, "MQTT 配置在移动端不应造成整页横向溢出").toBeLessThanOrEqual(1)
    await expect(dialog.locator('[data-slot="card"] [data-slot="card"]')).toHaveCount(0)

    await dialog.getByRole("button", { name: "创建连接器" }).click()
    await expect(dialog).toBeHidden()
    expect(createPayload).not.toBeNull()
    expect(createPayload?.connectorType).toBe("mqtt")
    expect(createPayload?.baseUrl).toBe("mqtts://broker.factory.example:8883")
    expect(createPayload?.authType).toBe("basic")

    const authConfig = JSON.parse(String(createPayload?.authConfig))
    expect(authConfig).toEqual({
      username: "rhd-reader",
      password: "mqtt-secret-value",
      clientId: "rhd-tenant-1",
    })
    const fieldMapping = JSON.parse(String(createPayload?.fieldMapping))
    expect(fieldMapping).toEqual({
      topics: ["devices/+/telemetry", "devices/+/events"],
      qos: 2,
      keepAliveSeconds: 90,
      cleanSession: true,
      unknownDevicePolicy: "dead_letter",
      deviceSerial: "$.device.serial",
      faultCode: "$.alarm.code",
      severity: "$.alarm.severity",
      alarmMessage: "$.alarm.message",
      recordedAt: "$.timestamp",
      metrics: {
        temperature: "$.metrics.temperature",
        pressure: "$.metrics.pressure",
      },
    })
    await expect(page.getByText("mqtt-secret-value", { exact: true })).toHaveCount(0)
  } finally {
    await context.close()
  }
})

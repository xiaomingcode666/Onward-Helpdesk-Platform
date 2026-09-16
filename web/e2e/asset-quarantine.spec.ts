import { expect, test } from "@playwright/test"

test.use({ video: "off" })

test.afterAll(async ({ request }) => {
  if (process.env.TKT007_BROWSER_URL) await request.post(`${process.env.TKT007_BROWSER_URL}/__test/stop`, { headers: { Authorization: "Bearer asset-quarantine-fixture" } })
})

test("quarantine reasons, tenant isolation, history and real rescan", async ({ page }) => {
  const fixture = process.env.TKT007_BROWSER_URL
  test.skip(!fixture, "Start the isolated Go attachment fixture and real ClamAV first")
  const profile = {
    accessToken: "asset-quarantine-fixture", domainType: "enterprise", tenantId: 8001,
    user: { id: 9001, username: "attachment-fixture", nickname: "附件验收", avatar: "", status: 0, roles: ["enterprise_admin"] },
    roles: ["enterprise_admin"], permissions: ["tenant.update", "asset.view", "asset.create", "ticket.view"], locale: "zh-CN", timezone: "UTC",
  }
  await page.addInitScript(value => {
    localStorage.setItem("remote-helpdesk-session:enterprise", JSON.stringify(value))
    localStorage.setItem("remote-helpdesk-session", JSON.stringify(value))
    localStorage.setItem("remote-helpdesk-locale:enterprise:8001", "zh-CN")
  }, profile)
  const errors: string[] = []
  page.on("pageerror", error => errors.push(error.message))
  await page.route("**/api/**", async route => {
    const url = new URL(route.request().url())
    if (url.pathname.startsWith("/api/enterprise/v1/asset-quarantine")) {
      const response = await route.fetch({ url: `${fixture}${url.pathname}${url.search}`, headers: { ...route.request().headers(), Authorization: "Bearer asset-quarantine-fixture" } })
      await route.fulfill({ response }); return
    }
    let data: unknown = {}
    if (url.pathname === "/api/auth/profile") data = profile
    else if (url.pathname === "/api/dashboard/agent/self/briefing") data = {
      isEngineer: false, workStatus: { status: "available", note: "", needsConfirmation: false },
      teams: [], unassignedTickets: [], myOpenTickets: [],
    }
    else if (url.pathname.includes("unread")) data = { count: 0, unread_count: 0 }
    else if (url.pathname.includes("capabilities")) data = { features: {} }
    else if (url.pathname.includes("notifications")) data = { items: [], total: 0 }
    await route.fulfill({ json: { success: true, data } })
  })
  await page.goto("/enterprise/asset-quarantine", { waitUntil: "domcontentloaded" })
  await expect(page.getByRole("heading", { name: "附件隔离", exact: true })).toBeVisible()
  await expect(page.getByRole("cell", { name: "其他公司附件.txt", exact: false })).toHaveCount(0)
  const waiting = page.getByRole("row").filter({ hasText: "等待复扫.txt" })
  const broken = page.getByRole("row").filter({ hasText: "损坏压缩包.zip" })
  await expect(waiting).toContainText("扫描或文件接收失败")
  await expect(broken).toContainText("压缩包损坏")
  await expect(waiting.getByRole("link")).toHaveCount(0)
  await waiting.getByRole("button", { name: "扫描记录" }).click()
  await expect(page.getByRole("region", { name: "扫描记录" })).toContainText("未完成引擎扫描")
  await waiting.getByRole("button", { name: "重新扫描" }).click()
  await expect(waiting).toHaveCount(0)
  await page.getByRole("combobox", { name: "扫描状态" }).selectOption("clean")
  const released = page.getByRole("row").filter({ hasText: "等待复扫.txt" })
  await expect(released).toContainText("扫描通过")
  await released.getByRole("button", { name: "扫描记录" }).click()
  await expect(page.getByRole("region", { name: "扫描记录" })).toContainText("ClamAV 1.4.3")
  await expect(page.getByRole("region", { name: "扫描记录" })).toContainText("扫描或文件接收失败")
  await page.screenshot({ path: "test-results/tkt-007-quarantine.png", fullPage: true })
  expect(errors).toEqual([])
})

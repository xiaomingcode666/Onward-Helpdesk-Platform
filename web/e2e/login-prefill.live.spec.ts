import { expect, test } from "@playwright/test"

const frontendUrl = process.env.E2E_FRONTEND_URL ?? process.env.E2E_BASE_URL ?? "http://localhost:3000"

const portalFixtures = [
  {
    portal: "platform",
    tab: "平台",
    username: "admin",
    password: process.env.NEXT_PUBLIC_LOCAL_LOGIN_PLATFORM_PASSWORD ?? "",
  },
  {
    portal: "enterprise",
    tab: "企业",
    username: "e2e.product1.engineer",
    password: process.env.NEXT_PUBLIC_LOCAL_LOGIN_ENTERPRISE_PASSWORD ?? "",
  },
  {
    portal: "partner",
    tab: "供应商",
    username: "e2e.product1.partner",
    password: process.env.NEXT_PUBLIC_LOCAL_LOGIN_PARTNER_PASSWORD ?? "",
  },
  {
    portal: "customer",
    tab: "用户端",
    username: "e2e.t1.customer.real",
    password: process.env.NEXT_PUBLIC_LOCAL_LOGIN_CUSTOMER_PASSWORD ?? "",
  },
] as const

test("四个正式入口保留验收账号预填", async ({ browser }) => {
  test.skip(
    process.env.E2E_VERIFY_LOGIN_DEFAULTS !== "true",
    "仅在临时验收账号预填开启时执行",
  )

  const missing = portalFixtures.filter((item) => !item.password).map((item) => item.portal)
  expect(missing, "验收账号密码必须通过受保护的构建环境提供").toEqual([])

  const context = await browser.newContext()
  const page = await context.newPage()

  for (const fixture of portalFixtures) {
    await page.goto(`${frontendUrl}/dashboard/login?portal=${fixture.portal}`)
    await expect(page.getByRole("tab", { name: fixture.tab, exact: true })).toHaveAttribute(
      "aria-selected",
      "true",
    )

    const username = page.locator('input[name="username"]')
    const password = page.locator('input[name="password"]')
    await expect(username).toHaveValue(fixture.username)
    await expect(password).toHaveAttribute("type", "password")

    const actualPassword = await password.inputValue()
    expect(actualPassword.length, `${fixture.portal} 入口密码应已预填`).toBeGreaterThan(0)
    expect(actualPassword === fixture.password, `${fixture.portal} 入口密码应匹配验收账号`).toBe(true)
  }

  await context.close()
})

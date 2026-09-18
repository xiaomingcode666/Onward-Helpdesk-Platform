import { expect, test } from "@playwright/test"
import type { ProjectConfiguration } from "../lib/api/project-configuration"
import { editableProjectConfiguration, replaceProjectRuntime, restoreProjectConfiguration } from "../lib/project-configuration-editing"
import runtimeExample from "../../config/project-configuration.runtime.example.json"

test.use({ video: "off" })

test("configuration editing preserves immutable history and only carries runtime dependencies", () => {
  const current = structuredClone(runtimeExample) as ProjectConfiguration
  current.runtime!.mail.password_ref = "secret://config-shared"
  current.runtime!.integrations = [{ provider: "example", enabled: false, base_url: "", app_id: "", secret_ref: "secret://config-shared", key_ref: "secret://config-app-key", metadata_json: "{}" }]
  current.secret_refs = ["secret://config-shared", "secret://config-app-key", "secret://unavailable-unused"]
  const historical: ProjectConfiguration = { schema_version: 1, tenant_id: current.tenant_id, environment: current.environment, projects: [{ key: "support", name: "Original" }], secret_refs: ["secret://historical-only"] }
  const original = structuredClone(historical)
  const restored = restoreProjectConfiguration(historical, current)
  expect(restored.secret_refs).toEqual(["secret://historical-only", "secret://config-shared", "secret://config-app-key"])
  expect(restored.schema_version).toBe(2)
  restored.projects[0].name = "Edited"
  restored.runtime!.timezone = "Asia/Shanghai"
  expect(historical).toEqual(original)
  expect(current.runtime!.timezone).toBe(runtimeExample.runtime.timezone)

  const runtimeHistory = structuredClone(current)
  runtimeHistory.runtime!.mail.password_ref = "secret://old-mail"
  runtimeHistory.secret_refs = ["secret://old-mail", "secret://config-shared", "secret://config-app-key"]
  expect(restoreProjectConfiguration(runtimeHistory, current)).toEqual(runtimeHistory)
  const legacy = { ...historical, projects: null, secret_refs: null } as unknown as ProjectConfiguration
  expect(editableProjectConfiguration(legacy)).toMatchObject({ projects: [], secret_refs: [] })
  expect(legacy.projects).toBeNull()

  const changed = replaceProjectRuntime(current, { ...current.runtime!, mail: { ...current.runtime!.mail, password_ref: "secret://config-mail" } })
  expect(changed.secret_refs).toEqual(["secret://config-shared", "secret://config-app-key", "secret://unavailable-unused", "secret://config-mail"])
  const removed = replaceProjectRuntime(changed, { ...changed.runtime!, integrations: [] })
  expect(removed.secret_refs).toEqual(["secret://unavailable-unused", "secret://config-mail"])
  expect(current.runtime!.mail.password_ref).toBe("secret://config-shared")
  const withIMAP = replaceProjectRuntime(current, { ...current.runtime!, mail: { ...current.runtime!.mail, imap: { enabled: true, host: "imap.example.test", port: 993, username: "mail@example.test", password_ref: "secret://imap", project_key: "", ticket_type: "" } } })
  expect(withIMAP.secret_refs).toContain("secret://imap")
  expect(restoreProjectConfiguration(historical, withIMAP).secret_refs).toContain("secret://imap")
})

test.afterAll(async ({ request }) => {
  if (process.env.E2E_CONFIG_FIXTURE_URL) {
    await request.post(`${process.env.E2E_CONFIG_FIXTURE_URL}/__test/stop`, { headers: { Authorization: "Bearer config-browser-fixture" } })
  }
})

// Configuration writes go only to the opt-in Go fixture's temporary database.
// All other APIs (including authentication) are intercepted with synthetic data.
test("configuration drafts, validation, activation and restore use real handlers", async ({ page }) => {
  const fixture = process.env.E2E_CONFIG_FIXTURE_URL
  test.skip(!fixture, "Start TestProjectConfigurationBrowserFixture against an isolated database first")
  const profile = {
    accessToken: "config-browser-fixture", domainType: "enterprise", tenantId: 8001,
    user: { id: 9001, username: "config-fixture", nickname: "配置验收", avatar: "", status: 0, roles: ["service_manager"] },
    roles: ["service_manager"], permissions: ["ticket.view", "ticket.create", "ticket.update", "ticket.assign"], locale: "zh-CN", timezone: "UTC",
  }
  await page.addInitScript(value => {
    localStorage.setItem("remote-helpdesk-session:enterprise", JSON.stringify(value))
    localStorage.setItem("remote-helpdesk-session", JSON.stringify(value))
  }, profile)
  const errors: string[] = []
  let legacyDrafts = false
  let deploymentManaged = false
  let unrelatedRuntimeReference = false
  page.on("pageerror", e => errors.push(e.message))
  await page.route("**/api/**", async route => {
    const requestUrl = new URL(route.request().url())
    const path = requestUrl.pathname
    if (path.startsWith("/api/enterprise/v1/ticket-settings/")) {
      const response = await route.fetch({ url: `${fixture}${path}${requestUrl.search}`, headers: { ...route.request().headers(), Authorization: "Bearer config-browser-fixture" } })
      if (path.endsWith("/configuration") && route.request().method() === "GET" && (legacyDrafts || unrelatedRuntimeReference)) {
        const result = await response.json()
        result.data.deployment_managed = deploymentManaged
        if (unrelatedRuntimeReference) result.data.document.secret_refs.push("secret://unavailable-unused")
        if (legacyDrafts) result.data.versions.unshift({
          id: 999999, base_version_id: 0, digest: "legacy-test", note: "旧版空字段草稿", created_by: 9001, created_at: new Date().toISOString(),
          document: { schema_version: 1, tenant_id: 8001, environment: "development", projects: null, secret_refs: null },
        }, {
          id: 999998, base_version_id: 0, digest: "legacy-empty-test", note: "旧版空规则草稿", created_by: 9001, created_at: new Date().toISOString(),
          document: { schema_version: 1, tenant_id: 8001, environment: "development", projects: null, secret_refs: null },
        })
        await route.fulfill({ json: result }); return
      }
      await route.fulfill({ response }); return
    }
    let data: unknown = undefined
    if (path === "/api/auth/profile") data = profile
    else if (path === "/api/enterprise/v1/tickets") data = { items: [], total: 0, page: 1, page_size: 20 }
    else if (path.includes("summary")) data = { total: 0, pending: 0, processing: 0, awaiting_customer: 0, sla_risk: 0, urgent: 0, done: 0 }
    else if (path.includes("unread")) data = { count: 0, unread_count: 0 }
    else if (path.includes("capabilities")) data = { service_scene: "knowledge_support", features: {} }
    await route.fulfill({ json: data === undefined ? { success: false, data: null, message: "Not provided by isolated configuration fixture" } : { success: true, data } })
  })
  await page.goto("/enterprise/tickets", { waitUntil: "domcontentloaded" })
  await page.getByRole("button", { name: /新建工单|创建工单|Create ticket/i }).click()
  await page.getByRole("button", { name: "受理规则", exact: true }).click()
  const dialog = page.getByRole("dialog", { name: "受理规则 · 配置版本" })
  const openSection = async (name: RegExp) => {
    const summary = dialog.locator("summary").filter({ hasText: name })
    if (await summary.locator("..").getAttribute("open") === null) await summary.click()
  }
  await expect(dialog.getByText("原有规则（尚未应用版本）", { exact: false })).toBeVisible()
  await dialog.getByRole("button", { name: "添加服务项目" }).click()
  await dialog.getByRole("textbox", { name: "项目标识 1", exact: true }).fill("support")
  await dialog.getByRole("textbox", { name: "项目名称 1", exact: true }).fill("知识服务")
  await dialog.getByRole("button", { name: "添加规则", exact: true }).click()
  await dialog.getByPlaceholder("例如：电话咨询增加联系电话必填").fill("电话资料规则验收")
  await dialog.getByRole("button", { name: "检查配置", exact: true }).click()
  await expect(dialog.getByText("配置结构不符合规范", { exact: false })).toBeVisible()
  // Ticket type is the sole empty single-line text input in the rule row.
  await dialog.locator('input[maxlength="64"]').last().fill("incident")
  await dialog.getByRole("checkbox", { name: "来电号码", exact: true }).check()
  await dialog.getByRole("button", { name: "保存草稿", exact: true }).click()
  await expect(dialog.getByText(/草稿 V\d+ 已保存，尚未生效/)).toBeVisible()
  await expect(dialog.getByRole("button", { name: "应用已检查的草稿" })).toBeDisabled()
  await dialog.getByRole("button", { name: "检查配置", exact: true }).click()
  await expect(dialog.getByText("配置检查通过；应用时还会再次检查。")).toBeVisible()
  await dialog.getByRole("button", { name: "应用已检查的草稿" }).click()
  await expect(dialog.getByText(/当前生效版本为 V\d+/)).toBeVisible()
  await dialog.locator("summary").filter({ hasText: "历史与草稿" }).click()
  await expect(dialog.getByText("迁移基线：", { exact: false })).toBeVisible()
  await dialog.getByRole("button", { name: "载入为新草稿" }).first().click()
  await expect(dialog.getByText(/已载入 V\d+ 的内容/)).toBeVisible()
  await expect(dialog.getByRole("button", { name: "应用已检查的草稿" })).toBeDisabled()
  await page.setViewportSize({ width: 390, height: 844 })
  await expect(dialog.getByRole("button", { name: "保存草稿", exact: true })).toBeVisible()
  expect(await dialog.evaluate(el => el.scrollWidth <= el.clientWidth + 2)).toBe(true)
  // Simulate immutable legacy rows from before structural draft validation.
  legacyDrafts = true
  await dialog.getByRole("button", { name: "重新加载", exact: true }).click()
  await dialog.locator("summary").filter({ hasText: "历史与草稿" }).click()
  await expect(dialog.getByText("旧版空字段草稿", { exact: true })).toBeVisible()
  await dialog.getByRole("button", { name: "载入为新草稿" }).first().click()
  await dialog.getByRole("button", { name: "保存草稿", exact: true }).click()
  await expect(dialog.getByText(/草稿 V\d+ 已保存，尚未生效/)).toBeVisible()
  // Deployment mode must offer export, never an online activation button.
  deploymentManaged = true
  await dialog.getByRole("button", { name: "重新加载", exact: true }).click()
  await expect(dialog.getByText("当前环境由部署管理。", { exact: false })).toBeVisible()
  await expect(dialog.getByRole("button", { name: "应用已检查的草稿" })).toHaveCount(0)
  await dialog.getByPlaceholder("例如：电话咨询增加联系电话必填").fill("导出部署草稿")
  await dialog.getByRole("button", { name: "保存草稿", exact: true }).click()
  await expect(dialog.getByText(/草稿 V\d+ 已保存，尚未生效/)).toBeVisible()
  await dialog.getByRole("button", { name: "检查配置", exact: true }).click()
  await expect(dialog.getByRole("button", { name: "导出已检查的草稿" })).toBeEnabled()
  const download = page.waitForEvent("download")
  await dialog.getByRole("button", { name: "导出已检查的草稿" }).click()
  expect((await download).suggestedFilename()).toMatch(/^project-configuration-v\d+\.json$/)
  // Upgrade real legacy settings into a checked runtime version, then restore.
  legacyDrafts = false
  deploymentManaged = false
  await page.setViewportSize({ width: 1280, height: 1000 })
  await dialog.getByRole("button", { name: "重新加载", exact: true }).click()
  await dialog.getByRole("button", { name: "载入现有运营设置（升级配置）" }).click()
  await expect(dialog.getByRole("textbox", { name: "默认时区", exact: true })).toBeVisible()
  await dialog.getByRole("textbox", { name: "默认时区", exact: true }).fill("Asia/Shanghai")
  await dialog.getByPlaceholder("例如：电话咨询增加联系电话必填").fill("运营设置升级验收")
  await dialog.getByRole("button", { name: "保存草稿", exact: true }).click()
  await expect(dialog.getByText(/草稿 V\d+ 已保存，尚未生效/)).toBeVisible()
  await dialog.getByRole("button", { name: "检查配置", exact: true }).click()
  await expect(dialog.getByText("配置检查通过；应用时还会再次检查。")).toBeVisible()
  await dialog.getByRole("button", { name: "应用已检查的草稿" }).click()
  await expect(dialog.getByText(/当前生效版本为 V\d+/)).toBeVisible()
  await dialog.getByRole("button", { name: "重新加载", exact: true }).click()
  await expect(dialog.getByRole("textbox", { name: "默认时区", exact: true })).toHaveValue("Asia/Shanghai")
  await dialog.getByRole("textbox", { name: "默认时区", exact: true }).fill("UTC")
  await dialog.getByPlaceholder("例如：电话咨询增加联系电话必填").fill("第二版运营设置")
  await dialog.getByRole("button", { name: "保存草稿", exact: true }).click()
  await expect(dialog.getByText(/草稿 V\d+ 已保存，尚未生效/)).toBeVisible()
  await dialog.getByRole("button", { name: "检查配置", exact: true }).click()
  await expect(dialog.getByText("配置检查通过；应用时还会再次检查。")).toBeVisible()
  await dialog.getByRole("button", { name: "应用已检查的草稿" }).click()
  await expect(dialog.getByText(/当前生效版本为 V\d+/)).toBeVisible()
  await dialog.locator("summary").filter({ hasText: "历史与草稿" }).click()
  await dialog.getByText("运营设置升级验收", {exact:true}).locator("..").getByRole("button",{name:"载入为新草稿"}).click()
  await expect(dialog.getByRole("textbox",{name:"默认时区",exact:true})).toHaveValue("Asia/Shanghai")
  // Retaining a current runtime while loading v1 history must also retain its
  // declared secrets. No SMTP/network call occurs; these are fixture files.
  await dialog.getByRole("button", { name: "重新加载", exact: true }).click()
  await openSection(/^邮件发送$/)
  await dialog.getByRole("checkbox", { name: "启用邮件发送", exact: true }).check()
  await dialog.getByRole("textbox", { name: "SMTP 地址", exact: true }).fill("smtp.example.test")
  await dialog.getByRole("spinbutton", { name: "SMTP 端口", exact: true }).fill("465")
  await dialog.getByRole("textbox", { name: "发信账号", exact: true }).fill("fixture@example.test")
  await dialog.getByRole("textbox", { name: "邮件密钥引用", exact: true }).fill("secret://config-shared")
  await dialog.getByRole("textbox", { name: "发信邮箱地址", exact: true }).fill("fixture@example.test")
  await openSection(/^外部系统接入$/)
  await dialog.getByRole("button", { name: "添加接入设置", exact: true }).click()
  await dialog.getByRole("textbox", { name: "接入标识 1", exact: true }).fill("webhook")
  await dialog.getByRole("textbox", { name: "接入地址 1", exact: true }).fill("https://example.test/webhook")
  await dialog.getByRole("textbox", { name: "接入密钥引用 1", exact: true }).fill("secret://config-shared")
  await dialog.getByRole("textbox", { name: "应用 Key 引用 1", exact: true }).fill("secret://config-app-key")
  await dialog.getByPlaceholder("例如：电话咨询增加联系电话必填").fill("邮箱和接入共享引用")
  await dialog.getByRole("button", { name: "保存草稿", exact: true }).click()
  await expect(dialog.getByText(/草稿 V\d+ 已保存，尚未生效/)).toBeVisible()
  await dialog.getByRole("button", { name: "检查配置", exact: true }).click()
  await expect(dialog.getByText("配置检查通过；应用时还会再次检查。")).toBeVisible()
  await dialog.getByRole("button", { name: "应用已检查的草稿" }).click()
  await expect(dialog.getByText(/当前生效版本为 V\d+/)).toBeVisible()

  unrelatedRuntimeReference = true
  await dialog.getByRole("button", { name: "重新加载", exact: true }).click()
  await dialog.locator("summary").filter({ hasText: "历史与草稿" }).click()
  await dialog.getByText("电话资料规则验收", { exact: true }).locator("..").getByRole("button", { name: "载入为新草稿" }).click()
  await openSection(/部署所需密钥引用/)
  await expect(dialog.getByRole("textbox", { name: "密钥引用", exact: true })).toHaveValue("secret://config-shared\nsecret://config-app-key")
  await dialog.getByRole("button", { name: "保存草稿", exact: true }).click()
  await expect(dialog.getByText(/草稿 V\d+ 已保存，尚未生效/)).toBeVisible()
  await dialog.getByRole("button", { name: "检查配置", exact: true }).click()
  await expect(dialog.getByText("配置检查通过；应用时还会再次检查。")).toBeVisible()
  unrelatedRuntimeReference = false
  await dialog.getByRole("button", { name: "应用已检查的草稿" }).click()
  await expect(dialog.getByText(/当前生效版本为 V\d+/)).toBeVisible()

  // Changing the mail reference must not undeclare the integration's shared
  // reference; deleting the integration must remove only its unused refs.
  await openSection(/^邮件发送$/)
  await dialog.getByRole("textbox", { name: "邮件密钥引用", exact: true }).fill("secret://config-mail")
  await openSection(/部署所需密钥引用/)
  await expect(dialog.getByRole("textbox", { name: "密钥引用", exact: true })).toHaveValue("secret://config-shared\nsecret://config-app-key\nsecret://config-mail")
  await dialog.getByRole("button", { name: "检查配置", exact: true }).click()
  await expect(dialog.getByText("配置检查通过；应用时还会再次检查。")).toBeVisible()
  await openSection(/^外部系统接入$/)
  await dialog.getByRole("button", { name: "删除此接入", exact: true }).click()
  await expect(dialog.getByRole("textbox", { name: "密钥引用", exact: true })).toHaveValue("secret://config-mail")
  await dialog.getByRole("button", { name: "检查配置", exact: true }).click()
  await expect(dialog.getByText("配置检查通过；应用时还会再次检查。")).toBeVisible()
  await page.setViewportSize({width:390,height:844})
  expect(await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth+2)).toBe(true)
  expect(errors).toEqual([])
})

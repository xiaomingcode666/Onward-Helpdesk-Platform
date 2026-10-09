import { expect, test } from "@playwright/test"
import { buildWhatsAppSecretsPayload, isWhatsAppSecretSatisfied } from "../lib/whatsapp-channel-secrets"

test.use({ video: "off" })

const empty = { metaAccessToken: "", metaAppSecret: "", webhookVerifyToken: "" }

test("secret payload keeps, sets and clears only what changed", () => {
  expect(buildWhatsAppSecretsPayload(empty, new Set())).toBeUndefined()
  expect(buildWhatsAppSecretsPayload({ ...empty, metaAppSecret: "  app \n" }, new Set())).toEqual({ metaAppSecret: "app" })
  expect(buildWhatsAppSecretsPayload(empty, new Set(["webhookVerifyToken"]))).toEqual({ clear: ["webhookVerifyToken"] })
  expect(buildWhatsAppSecretsPayload({ ...empty, webhookVerifyToken: "new" }, new Set(["webhookVerifyToken"]))).toEqual({ webhookVerifyToken: "new" })
  const stored = { metaAppSecret: { configured: true, fingerprintSuffix: "ab12" } }
  expect(isWhatsAppSecretSatisfied("metaAppSecret", empty, stored, new Set(), "")).toBe(true)
  expect(isWhatsAppSecretSatisfied("metaAppSecret", empty, stored, new Set(["metaAppSecret"]), "")).toBe(false)
  expect(isWhatsAppSecretSatisfied("metaAppSecret", empty, undefined, new Set(), "secret://app")).toBe(true)
})

// All APIs are intercepted with synthetic data; no backend or Meta call occurs.
test("channel editor stores secrets write-only", async ({ page }) => {
  const profile = {
    accessToken: "channel-secret-fixture", domainType: "enterprise", tenantId: 8101,
    user: { id: 9101, username: "channel-admin", nickname: "渠道管理员", avatar: "", status: 0, roles: ["owner"] },
    roles: ["owner"], permissions: ["channel.view", "channel.update", "channel.create"], locale: "zh-CN", timezone: "UTC",
  }
  await page.addInitScript((value) => {
    localStorage.setItem("remote-helpdesk-session:enterprise", JSON.stringify(value))
    localStorage.setItem("remote-helpdesk-session", JSON.stringify(value))
  }, profile)
  const config = JSON.stringify({ projectKey: "p1", metaPhoneNumberId: "phone-1", metaBusinessAccountId: "waba-1", metaAccessTokenSecretRef: "", metaAppSecretSecretRef: "", webhookVerifyTokenSecretRef: "" })
  const channel = {
    id: 31, channelType: "whatsapp", channelId: "wa-31", aiAgentId: 41, aiAgentName: "机器人", name: "WhatsApp配置", configJson: config, status: 0, remark: "",
    whatsappSecrets: { metaAccessToken: { configured: false }, metaAppSecret: { configured: false }, webhookVerifyToken: { configured: false } } as Record<string, { configured: boolean; fingerprintSuffix?: string }>,
  }
  const updates: Array<Record<string, unknown>> = []
  const errors: string[] = []
  page.on("pageerror", (e) => errors.push(e.message))
  await page.route("**/api/**", async (route) => {
    const path = new URL(route.request().url()).pathname
    let data: unknown = undefined
    if (path === "/api/auth/profile") data = profile
    else if (path === "/api/dashboard/channel/list") data = { results: [channel], page: { page: 1, limit: 20, total: 1 } }
    else if (path === "/api/dashboard/channel/31") data = channel
    else if (path === "/api/dashboard/ai-agent/list_all") data = [{ id: 41, name: "机器人", status: 0, workflowVersionId: 2, workflowPublished: true }]
    else if (path === "/api/dashboard/channel/update") {
      const body = route.request().postDataJSON() as Record<string, unknown>
      updates.push(body)
      const secrets = (body.whatsappSecrets ?? {}) as Record<string, unknown>
      for (const key of ["metaAccessToken", "metaAppSecret", "webhookVerifyToken"]) {
        if (typeof secrets[key] === "string") channel.whatsappSecrets[key] = { configured: true, fingerprintSuffix: "9f3c" }
      }
      for (const key of (secrets.clear as string[] | undefined) ?? []) channel.whatsappSecrets[key] = { configured: false }
      data = null
    }
    const stubbed = data !== undefined || path === "/api/dashboard/channel/update"
    await route.fulfill({ json: stubbed ? { success: true, data } : { success: false, data: null, message: "not stubbed" } })
  })

  await page.goto("/dashboard/channels", { waitUntil: "domcontentloaded" })
  await page.getByRole("button", { name: "编辑" }).first().click()
  const token = page.getByLabel("Meta 访问令牌（Access Token）")
  await expect(token).toHaveAttribute("type", "password")
  await expect(token).toHaveAttribute("placeholder", "未设置")
  await token.fill("EAAG-test-token")
  await page.getByLabel("Meta App Secret", { exact: true }).fill("app-secret-test")
  await page.getByLabel("回调验证口令（Verify Token）").fill("verify-test")
  await page.getByRole("button", { name: /保存|确定/ }).click()
  await expect.poll(() => updates.length).toBe(1)
  expect(updates[0].whatsappSecrets).toEqual({ metaAccessToken: "EAAG-test-token", metaAppSecret: "app-secret-test", webhookVerifyToken: "verify-test" })
  expect(String(updates[0].configJson)).not.toContain("EAAG-test-token")

  await page.getByRole("button", { name: "编辑" }).first().click()
  await expect(page.getByLabel("Meta 访问令牌（Access Token）")).toHaveValue("")
  await expect(page.getByLabel("Meta 访问令牌（Access Token）")).toHaveAttribute("placeholder", "已设置 · …9f3c，留空保持不变")
  await page.getByRole("button", { name: /保存|确定/ }).click()
  await expect.poll(() => updates.length).toBe(2)
  expect(updates[1].whatsappSecrets).toBeUndefined()

  await page.getByRole("button", { name: "编辑" }).first().click()
  await page.getByRole("button", { name: "清除" }).nth(2).click()
  await page.getByRole("button", { name: /保存|确定/ }).click()
  await expect(page.getByText("请填写回调验证口令，或在高级选项中填写密钥文件引用")).toBeVisible()
  expect(updates.length).toBe(2)
  expect(errors).toEqual([])
})

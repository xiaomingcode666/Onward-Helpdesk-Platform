import { chromium } from "@playwright/test"
import assert from "node:assert/strict"
import { writeFile } from "node:fs/promises"
import { fileURLToPath } from "node:url"

const baseURL = process.env.INTAKE_TEST_URL || "http://127.0.0.1:3000"
const outputDir = fileURLToPath(new URL("../../outputs/fun-008/", import.meta.url))
const browser = await chromium.launch({ channel: "chrome", headless: true })
const results = []
try {
  for (const viewport of [{ width: 1440, height: 1080 }, { width: 390, height: 844 }]) {
    const context = await browser.newContext({ viewport })
    const session = { accessToken: "synthetic-test-token", tenantId: 8001, domainType: "enterprise", user: { id: 9001, username: "test-agent", nickname: "Test Agent", avatar: "", status: 0, roles: ["enterprise_admin"], locale: "en-US" }, roles: ["enterprise_admin"], permissions: ["ticket.create", "ticket.view", "ticket.update", "product.view"], featureFlags: { device: true }, locale: "en-US" }
    await context.addInitScript((session) => {
      localStorage.setItem("remote-helpdesk-session", JSON.stringify(session))
      localStorage.setItem("remote-helpdesk-session:enterprise", JSON.stringify(session))
      localStorage.setItem("remote-helpdesk-locale:enterprise:8001", "en-US")
    }, session)
    const page = await context.newPage()
    const errors = []
    page.on("pageerror", (error) => errors.push(error.message))
    let created = null
    let submitted = null
    let policy = { rules: [{ project_key: "Project Alpha", channel: "phone", ticket_type: "Incident", required_fields: ["caller_phone"] }] }
    await page.route("**/api/**", async (route) => {
      const request = route.request()
      const url = new URL(request.url())
      let data = null
      if (url.pathname === "/api/auth/profile") data = session
      else if (url.pathname.endsWith("/agent/self/briefing")) data = { isEngineer: false, workStatus: { status: "available", note: "", needsConfirmation: false }, teams: [], unassignedTickets: [], myOpenTickets: [] }
      else if (url.pathname.endsWith("/ticket-settings/intake")) {
        if (request.method() === "PUT") policy = request.postDataJSON()
        data = policy
      } else if (url.pathname.endsWith("/tickets/customer-options")) data = []
      else if (url.pathname.endsWith("/products")) data = []
      else if (url.pathname.endsWith("/tickets/summary")) data = { total: created ? 1 : 0, pending: created ? 1 : 0, processing: 0, awaiting_customer: 0, sla_risk: 0, urgent: 0, done: 0 }
      else if (url.pathname.endsWith("/tickets")) {
        if (request.method() === "POST") {
          submitted = request.postDataJSON()
          created = { ...submitted, id: 88, ticket_no: "TK-TEST-008", source_record_id: "MP-TEST-008", status: "pending", priority: "medium", context_status: "context_incomplete", missing_context: ["caller_phone"], received_at: "2026-09-08T01:00:00Z", created_at: "2026-09-08T01:00:00Z", updated_at: "2026-09-08T01:00:00Z", current_team_id: 0, assignee_id: 0, team_name: "", customer_name: "", device_no: "", product_name: "", assignee_name: null, sla_deadline: "", sla_breached: false, dispatch_attempts: 0, product_id: 0 }
          data = created
        } else data = { items: created ? [created] : [], total: created ? 1 : 0, page: 1, page_size: 10, total_pages: 1 }
      } else if (/\/tickets\/88(?:\/intake)?$/.test(url.pathname)) {
        if (request.method() === "PATCH") { const input = request.postDataJSON(); created = { ...created, ...input, context_status: "complete", missing_context: [] } }
        data = { ticket: { ...created, description: "Recorded by customer support", category: "", product_module_id: 0, conversation_id: 0 }, customer: { id: 0, name: "", company: "", contact: "", email: "" }, device_context: {}, flow: { current_step: "Receive", steps: [] }, assignment: { assignee_name: "", team_name: "", dispatch_attempts: 0 }, meeting: {}, repair: { parts: [] }, timeline: [], assets: [], audit_refs: [], actions: {} }
      }
      await route.fulfill({ json: { success: true, errorCode: 0, message: "", data } })
    })
    await page.goto(`${baseURL}/enterprise/tickets`, { waitUntil: "domcontentloaded", timeout: 120000 })
    await page.getByRole("button", { name: /Create ticket|New ticket/i }).first().click({ timeout: 60000 })
    const dialog = page.getByRole("dialog").last()
    const field = (label) => dialog.locator(".ant-form-item").filter({ has: page.locator("label").filter({ hasText: new RegExp(`^${label}$`) }) })
    await field("Intake channel").locator(".ant-select").click()
    await page.locator(".ant-select-item-option").filter({ hasText: "Manual phone" }).click()
    await dialog.getByRole("button", { name: "Intake policy", exact: true }).click()
    const policyDialog = page.getByRole("dialog").last()
    await policyDialog.getByRole("checkbox", { name: "Caller name", exact: true }).check()
    await policyDialog.getByRole("button", { name: "Save", exact: true }).click()
    assert.ok(policy.rules[0].required_fields.includes("caller_name"))
    await field("Project").locator(".ant-select").click()
    await page.locator(".ant-select-item-option").filter({ hasText: "Project Alpha" }).click()
    await field("Ticket type").locator(".ant-select").click()
    await page.locator(".ant-select-item-option").filter({ hasText: "Incident" }).click()
    await field("Caller name").locator("input").fill("Test Caller")
    await dialog.locator("input[maxlength='200']").fill("Phone intake acceptance")
    await dialog.locator("textarea").fill("Recorded by customer support")
    assert.match(await dialog.innerText(), /Context Incomplete/)
    await dialog.getByText("New ticket", { exact: true }).click()
    await dialog.evaluate((element) => { const body = element.querySelector(".ant-modal-body"); if (body) body.scrollTop = 0 })
    await page.screenshot({ path: `${outputDir}/phone-create-${viewport.width}.png`, fullPage: true, animations: "disabled" })
    const overflow = await dialog.evaluate((element) => element.scrollWidth > element.clientWidth + 2)
    assert.equal(overflow, false, "phone dialog must not overflow horizontally")
    await dialog.getByRole("button", { name: /Create ticket|Create$/i }).click()
    await page.getByText("Phone intake acceptance", { exact: true }).first().waitFor({ timeout: 15000 })
    assert.equal(submitted.source, "manual")
    assert.equal(submitted.channel, "phone")
    assert.equal(submitted.project_key, "Project Alpha")
    assert.equal(submitted.caller_name, "Test Caller")
    assert.equal(submitted.caller_phone, "")
    assert.ok(submitted.idempotency_key)
    assert.equal(submitted.customer_id, undefined)
    await page.getByText("Phone intake acceptance", { exact: true }).first().click()
    await page.getByText("MP-TEST-008", { exact: true }).waitFor()
    await page.getByRole("dialog").last().evaluate(async (element) => { await Promise.all(element.getAnimations({ subtree: true }).map((animation) => animation.finished.catch(() => {}))) })
    await page.screenshot({ path: `${outputDir}/phone-detail-${viewport.width}.png`, fullPage: true, animations: "disabled" })
    await page.getByRole("button", { name: "Complete intake context", exact: true }).click()
    const completion = page.getByRole("dialog").last()
    const callerPhone = completion.locator(".ant-form-item").filter({ has: page.locator("label").filter({ hasText: /^Caller phone$/ }) }).locator("input")
    await callerPhone.fill("+1 0000000000")
    await completion.getByRole("button", { name: "Save", exact: true }).click()
    await page.getByText("Context complete", { exact: true }).waitFor()
    assert.deepEqual(errors, [])
    results.push({ viewport, policyConfiguration: "passed", creation: "passed", completion: "passed", horizontalOverflow: false, pageErrors: errors, submitted })
    await context.close()
  }
  await writeFile(`${outputDir}/browser-results.json`, JSON.stringify({ tester: "Codex automated Playwright", testedAt: new Date().toISOString(), testMode: "intercepted synthetic API responses", results }, null, 2))
  console.log(JSON.stringify(results.map(({ viewport, creation, completion }) => ({ viewport, creation, completion }))))
} finally {
  await browser.close()
}

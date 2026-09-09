import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const accountMenuSource = await readFile(new URL("./account-menu.tsx", import.meta.url), "utf8")
const customerShellSource = await readFile(new URL("./customer-shell.tsx", import.meta.url), "utf8")
const domainShellSource = await readFile(new URL("./domain-shell.tsx", import.meta.url), "utf8")

test("customer shell does not expose internal company profile fields", () => {
  assert.doesNotMatch(customerShellSource, /profile\.company_name/)
})

test("customer shell uses branding from the authenticated tenant session", () => {
  assert.match(domainShellSource, /domain === "enterprise" \|\| domain === "customer" \? session\?\.tenantBranding : undefined/)
  assert.match(domainShellSource, /tenantBranding\?\.brandName/)
  assert.match(domainShellSource, /tenantBranding\?\.logoUrl/)
  assert.match(domainShellSource, /domain === "customer" \? getCustomerThemeClassName\(customerTheme\) : undefined/)
  assert.match(domainShellSource, /data-customer-theme=\{domain === "customer" \? customerTheme : undefined\}/)
})

test("customer account menu hides internal tenancy and authorization fields", () => {
  assert.match(accountMenuSource, /domain === "customer"\s*\? "客户账号"/)
  assert.match(accountMenuSource, /domain === "customer" \? \[\] : \[/)
  assert.match(accountMenuSource, /domain !== "customer" \? \(/)
})

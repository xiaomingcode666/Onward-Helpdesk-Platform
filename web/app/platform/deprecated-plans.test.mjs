import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const [tenantPage, usagePage, modelsPage, platformApi, platformI18n] = await Promise.all([
  readFile(new URL("./tenants/page.tsx", import.meta.url), "utf8"),
  readFile(new URL("./usage/page.tsx", import.meta.url), "utf8"),
  readFile(new URL("./models/page.tsx", import.meta.url), "utf8"),
  readFile(new URL("../../lib/api/platform.ts", import.meta.url), "utf8"),
  readFile(new URL("../../i18n/extracted/platform.ts", import.meta.url), "utf8"),
])

test("platform tenant and model pages do not load the retired plan catalog", () => {
  assert.doesNotMatch(`${tenantPage}\n${usagePage}\n${modelsPage}\n${platformApi}`, /fetchPlatformPlans|savePlatformPlan|PlatformPlan/)
  assert.doesNotMatch(platformI18n, /plansLoadFailed|plansInlineError|loadingPlans|initialPlan|planCount|tenantCountUnit|overLimitTenantUnit/)
})

test("platform tenant page keeps tenant loading failure handling", () => {
  assert.match(tenantPage, /fetchPlatformTenants\(\{ search, page, limit: DEFAULT_PAGE_SIZE \}\)/)
  assert.match(tenantPage, /setError\(err instanceof Error \? err.message : t\("platformExtract\.tenants\.tenantDataUnavailable"\)\)/)
  assert.match(tenantPage, /<PlatformError message=\{error\} onRetry=\{\(\) => void load\(\)\} \/>/)
})

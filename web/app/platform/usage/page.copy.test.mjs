import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")

test("platform usage page has no retired plan management", () => {
  assert.doesNotMatch(source, /DialogDescription/)
  assert.doesNotMatch(source, /fetchPlatformPlans|savePlatformPlan|PlatformUsagePlan|planConfigTitle|planLoading|noPlans|addPlan|editPlan|planCodeNameRequired|planSaveFailed/)
  assert.match(source, /fetchPlatformUsage/)
  assert.match(source, /fetchPlatformSub2APIUsers/)
  assert.match(source, /<ContentModule title=\{t\("platformExtract\.usage\.sharedTranslationTitle"\)\}/)
  assert.match(source, /<ContentModule title=\{t\("platformExtract\.usage\.trendTitle"\)\}/)
})

test("platform Sub2API accounts use unified paged loading", () => {
  assert.match(source, /const SUB2API_ACCOUNT_PAGE_SIZE = 20/)
  assert.match(source, /const \[accountPageSize, setAccountPageSize\] = useState\(SUB2API_ACCOUNT_PAGE_SIZE\)/)
  assert.match(source, /pageSize,\s*search: search\.trim\(\),/)
  assert.match(source, /pageSizeOptions=\{\[10, 20, 50\]\}/)
  assert.match(source, /setAccountPageSize\(pageSize\)/)
  assert.match(source, /loadSub2APIUsers\(1, appliedAccountSearch, appliedAccountStatus, pageSize\)/)
  assert.doesNotMatch(source, /pageSize:\s*100/)
  assert.doesNotMatch(source, /每页 \{formatNumber\(data\.pageSize \|\| 100\)\} 条/)
})

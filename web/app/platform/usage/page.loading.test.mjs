import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")

test("platform usage keeps loading inside stable modules", () => {
  assert.doesNotMatch(source, /loading && data === null/)
  assert.doesNotMatch(source, /const isInitialLoading =/)
  assert.match(source, /const \[hasLoadedUsage, setHasLoadedUsage\] = useState\(false\)/)
  assert.match(source, /const \[sub2APIDataLoaded, setSub2APIDataLoaded\] = useState\(false\)/)
  assert.doesNotMatch(source, /const initialAccountsLoading = data === null && loading/)
  assert.match(source, /<PageShell[\s\S]*title=\{t\("platformExtract\.usage\.pageTitle"\)\}[\s\S]*<div className="space-y-4">/)
})

test("platform usage renders every major block with its own loading branch", () => {
  assert.match(source, /\{summaryInitialLoading \? \([\s\S]*aria-label=\{t\("platformExtract\.usage\.metricsAria"\)\}/)
  assert.match(source, /<Sub2APIAccountsPanel[\s\S]*accountInitialLoading=\{sub2APILoading && !sub2APIDataLoaded\}[\s\S]*loading=\{sub2APILoading\}/)
  assert.match(source, /<ContentModule title=\{t\("platformExtract\.usage\.sharedTranslationTitle"\)\}[\s\S]*sharedTranslationAria/)
  assert.match(source, /<ContentModule title=\{t\("platformExtract\.usage\.trendTitle"\)\}[\s\S]*<ModuleLoading variant="detail" count=\{3\} label=\{t\("platformExtract\.usage\.trendLoading"\)\} \/>/)
})

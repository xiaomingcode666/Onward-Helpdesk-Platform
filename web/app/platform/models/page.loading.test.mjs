import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")

test("platform model workspace keeps the page shell visible while loading", () => {
  assert.doesNotMatch(source, /if \(workspaceLoading && workspace === null\)/)
  assert.doesNotMatch(source, /if \(workspaceError && workspace === null\)/)
  assert.match(source, /const \[workspaceLoaded, setWorkspaceLoaded\] = useState\(false\)/)
  assert.match(source, /const initialWorkspaceLoading = workspaceLoading && !workspaceLoaded/)
  assert.match(source, /<PageShell[\s\S]*title=\{t\("platformExtract\.models\.title"\)\}[\s\S]*\{initialWorkspaceLoading \? \(/)
  assert.match(source, /<ModelWorkspaceLoadingContent t=\{t\} \/>/)
  assert.match(source, /disabled=\{!hasWorkspace\}/)
})

test("platform model workspace loading keeps a full tenant table and tenant detail tabs", () => {
  assert.match(source, /<ContentModule title=\{t\("platformExtract\.models\.modules\.tenants"\)\}[\s\S]*<ModuleLoading variant="table" count=\{7\} label=\{t\("platformExtract\.models\.loading\.modelTenants"\)\} \/>/)
  assert.doesNotMatch(source, /tenantConfig\.modelCredential/)
  assert.match(source, /<DataTable<PlatformAITenantWorkspaceItem>[\s\S]*scroll=\{\{ x: 2012 \}\}[\s\S]*columns=\{tenantColumns\}/)
  assert.doesNotMatch(source, /tenantConfig\.accessToken/)
  assert.match(source, /platformExtract\.models\.groups\.tenant/)
  assert.match(source, /rhd-railops-platform-models-tenant-row-selected/)
  assert.doesNotMatch(source, /navigator\.clipboard\.writeText\(value\)/)
  assert.match(source, /<DetailDrawer[\s\S]*open=\{Boolean\(selectedTenant\)\}[\s\S]*tenantDetailTabs/)
  assert.match(source, /\) : workspace \? \([\s\S]*<div className="min-w-0 space-y-4">/)
  assert.match(source, /value: "model-usage"/)
  assert.match(source, /value: "call-details"/)
  assert.match(source, /value: "recharge"/)
  assert.match(source, /onClick: \(\) => \{[\s\S]*setSelectedTenantId\(item\.tenant\.id\)/)
  assert.match(source, /return null/)
})

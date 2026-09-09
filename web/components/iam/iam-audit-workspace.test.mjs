import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const workspaceSource = await readFile(
  new URL("./iam-workspace-page.tsx", import.meta.url),
  "utf8",
)

const auditColumnsSource = workspaceSource.slice(
  workspaceSource.indexOf("const auditColumns"),
  workspaceSource.indexOf("const baseCopy"),
)
const auditWorkspaceSource = workspaceSource.slice(
  workspaceSource.indexOf("function buildAuditWorkspace"),
  workspaceSource.indexOf("function mergeBase"),
)

test("audit list presents business actions and outcomes without risk grading", () => {
  assert.match(auditColumnsSource, /操作内容/)
  assert.match(auditColumnsSource, /操作对象/)
  assert.match(auditColumnsSource, /结果/)
  assert.doesNotMatch(auditColumnsSource, /风险/)
  assert.doesNotMatch(auditWorkspaceSource, /高风险|风险等级|riskPill/)
})

test("audit rows expose selectable details for technical identifiers", () => {
  assert.match(auditWorkspaceSource, /kind: "audit-log"/)
  assert.match(auditWorkspaceSource, /协助授权/)
  assert.match(auditWorkspaceSource, /未成功操作/)
})

test("platform staff and permission rows expose edit and delete operations", () => {
  assert.match(workspaceSource, /IAMPlatformStaffDialog/)
  assert.match(workspaceSource, /deletePlatformStaff/)
  assert.match(workspaceSource, /deleteIAMRole/)
  assert.match(workspaceSource, /setEditingPlatformStaff/)
  assert.match(workspaceSource, /setEditingRole/)
})

test("platform permission primary action is labeled as creating a role", () => {
  const start = workspaceSource.indexOf('  "platform-permissions": {')
  const end = workspaceSource.indexOf('  "platform-audit": {')
  assert.notEqual(start, -1)
  assert.notEqual(end, -1)

  const platformPermissionCopy = workspaceSource.slice(
    start,
    end,
  )

  assert.match(platformPermissionCopy, /label: "新增角色"/)
  assert.doesNotMatch(platformPermissionCopy, /保存平台角色|保存角色/)
})

test("iam workspace and detail surfaces stay on semantic tokens for primary visual cues", () => {
  assert.doesNotMatch(workspaceSource, /border-blue-200 bg-blue-50 text-blue-700/)
  assert.doesNotMatch(workspaceSource, /text-slate-950|text-slate-700|text-slate-500/)
  assert.doesNotMatch(workspaceSource, /bg-white shadow-sm/)
  assert.doesNotMatch(workspaceSource, /bg-slate-50/)
  assert.doesNotMatch(workspaceSource, /border-slate-200/)
  assert.doesNotMatch(workspaceSource, /hover:bg-slate-50/)
  assert.match(workspaceSource, /border-primary\/20 bg-primary\/10 px-2 text-xs font-medium text-primary/)
})

test("iam workspace empty states stay terse", () => {
  assert.doesNotMatch(workspaceSource, /当前租户暂无/)
  assert.doesNotMatch(workspaceSource, /当前企业暂无/)
  assert.match(workspaceSource, /emptyMessage: "暂无企业成员"/)
  assert.match(workspaceSource, /emptyMessage: "暂无审计记录"/)
})

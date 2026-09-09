import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./engineer-login-briefing-modal.tsx", import.meta.url), "utf8")

test("engineer briefing is shown once per login session and remains manually accessible", () => {
  assert.match(source, /window\.sessionStorage\.getItem/)
  assert.match(source, /window\.sessionStorage\.setItem/)
  assert.match(source, /onClick=\{openStatusEditor\}/)
  assert.match(source, /workStatusButton\.editAria/)
  assert.match(source, /workStatusButton\.label<\/span>/)
})

test("available engineers do not receive a delayed login overlay", () => {
  assert.match(source, /const needsAttention = result\.workStatus\.needsConfirmation \|\| recoveryDueAtLoad/)
  assert.match(source, /autoOpen && shouldOpen && needsAttention/)
  assert.match(source, /const showModalDialog = open/)
  assert.doesNotMatch(source, /fixed right-4 top-16/)
  assert.match(source, /<Dialog open=\{showModalDialog\}/)
  assert.match(source, /queues\.unassignedTitle/)
  assert.match(source, /queues\.myOpenTitle/)
})

test("employee portal keeps the work status affordance visible while loading or unavailable", () => {
  assert.match(source, /StatusPlaceholderButton/)
  assert.match(source, /isEmployeePortalMode = session\.supportMode === "employee_portal"/)
  assert.match(source, /if \(loading\) \{\s*return isEmployeePortalMode \? <StatusPlaceholderButton loading label=\{eb\(t, "workStatusButton\.loadingLabel"\)\} t=\{t\} title=\{eb\(t, "workStatusButton\.loadingTitle"\)\} \/> : null/)
  assert.match(source, /label=\{loadError \? eb\(t, "workStatusButton\.unavailable"\) : eb\(t, "workStatusButton\.notConfigured"\)\}/)
  assert.match(source, /workStatusButton\.notConfiguredTitle/)
})

test("engineer briefing auto-opens only when the shell opts into operational routes", () => {
  assert.match(source, /autoOpen = true/)
  assert.match(source, /autoOpen && shouldOpen && needsAttention/)
  assert.match(source, /if \(!autoOpen\) return/)
})

test("engineer can manually select leave and other non-dispatchable statuses", () => {
  for (const status of ["available", "busy", "leave", "offline", "custom"]) {
    assert.match(source, new RegExp(`value: "${status}"`))
  }
  assert.match(source, /updateEngineerWorkStatus/)
  assert.match(source, /saveManualStatus/)
  assert.match(source, /dialog\.updateStatus/)
  assert.match(source, /statusEditor\.recoveryLabel/)
})

test("login reminders do not force non-available statuses back to available", () => {
  assert.match(source, /confirmAvailableStatus/)
  assert.match(source, /updateEngineerWorkStatus\(\{ status: "available", note: "" \}\)/)
  assert.match(source, /requiresAvailableConfirmation/)
  assert.match(source, /acknowledgeBriefing/)
  assert.match(source, /dialog\.keepCurrentStatus/)
  assert.match(source, /dialog\.recoverAndEnterOverview/)
  assert.match(source, /dialog\.enterOverview/)
})

test("non-available status is polled every thirty minutes", () => {
  assert.match(source, /STATUS_POLL_INTERVAL_MS = 30 \* 60 \* 1000/)
  assert.match(source, /window\.setInterval/)
  assert.match(source, /!result\.workStatus\.needsConfirmation/)
  assert.match(source, /"reminder"/)
  assert.match(source, /dialog\.badgeReminder/)
})

test("work statuses are localized and rendered with explicit icons", () => {
  for (const key of ["available", "busy", "leave", "offline", "custom"]) {
    assert.match(source, new RegExp(`labelKey: "statusOptions\\.${key}"`))
  }
  for (const icon of ["CheckCircle2Icon", "PauseCircleIcon", "CalendarClockIcon", "PowerIcon", "CircleDashedIcon"]) {
    assert.match(source, new RegExp(icon))
  }
  assert.match(source, /layout="vertical"/)
  assert.match(source, /labelRender: \(\) => \(\s*<span className="flex min-w-0 items-center gap-2">\s*<DraftStatusIcon className=\{`size-4 shrink-0 \$\{draftStatusMeta\.tone\}`\}/)
  assert.match(source, /<span className="block truncate font-medium leading-5 text-foreground">\{eb\(t, item\.labelKey\)\}/)
  assert.match(source, /currentStatusMeta\.labelKey/)
})

test("briefing uses reminder hierarchy and localized ticket status badges", () => {
  assert.match(source, /BellRingIcon/)
  assert.match(source, /dialog\.badgeLogin/)
  assert.match(source, /TICKET_STATUS/)
  assert.match(source, /<StatusIcon \/>\{badgeLabel\}/)
})

test("briefing dialog avoids explanatory header description copy", () => {
  assert.doesNotMatch(source, /DialogDescription/)
  assert.doesNotMatch(source, /修改后立即生效；非空闲状态将暂停自动派单/)
  assert.doesNotMatch(source, /恢复为空闲后会重新参与自动派单/)
  assert.doesNotMatch(source, /保留当前工作状态，并显示本次需要处理的工单/)
  assert.doesNotMatch(source, /系统将恢复自动派单/)
  assert.doesNotMatch(source, /关闭提醒不会修改工作状态/)
})

test("briefing renders team unassigned and personal open ticket queues", () => {
  assert.match(source, /briefing\.unassignedTickets/)
  assert.match(source, /briefing\.myOpenTickets/)
  assert.match(source, /briefing\.unassignedTicketCount \?\? briefing\.unassignedTickets\.length/)
  assert.match(source, /briefing\.myOpenTicketCount \?\? briefing\.myOpenTickets\.length/)
  assert.match(source, /queues\.unassignedTitle/)
  assert.match(source, /queues\.myOpenTitle/)
  assert.match(source, /ticketAgeLabel\(item\.created_at, t\)/)
  assert.match(source, /buildEnterpriseTicketWorkbenchPathFromItem\(item\)/)
  assert.doesNotMatch(source, /\/enterprise\/tickets\/\$\{item\.id\}/)
  assert.match(source, /ticketRows\.created/)
})

test("knowledge-support briefing uses support-team terminology", () => {
  assert.match(source, /session\.featureFlags\?\.product !== false/)
  assert.match(source, /hasProductConcept \? "teamLists\.title" : "teamLists\.supportTitle"/)
  assert.match(source, /hasProductConcept \? "queues\.unassignedTitle" : "queues\.supportUnassignedTitle"/)
  assert.match(source, /hasProductConcept \? "ticketRows\.noProductTeam" : "ticketRows\.noSupportTeam"/)
})

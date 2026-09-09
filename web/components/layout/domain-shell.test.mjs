import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./domain-shell.tsx", import.meta.url), "utf8")

test("domain shell hides tenant content as soon as authentication expires", () => {
  assert.match(source, /const \{ ready, session \} = useAuth\(\)/)
  assert.match(source, /if \(!ready \|\| !session\)/)
  assert.match(source, /aria-busy="true"/)
})

test("domain shell protects admin schedules but allows the engineer mine view", () => {
  assert.match(source, /pathname\?\.startsWith\("\/enterprise\/org\/schedules"\)/)
  assert.match(source, /const searchParams = useSearchParams\(\)/)
  assert.match(source, /isMyWorkScheduleRoute = pathname\?\.startsWith\("\/enterprise\/org\/schedules"\) && searchParams\?\.get\("view"\) === "mine"/)
  assert.match(source, /isEmployeePortalMode \|\| isMyWorkScheduleRoute \? null : "agentTeamSchedule\.view"/)
})

test("enterprise shell mounts engineer briefing in the authenticated topbar", () => {
  assert.match(source, /canShowEngineerBriefing \? \(\s*<EngineerLoginBriefingModal autoOpen=\{shouldAutoOpenEngineerBriefing\} session=\{session\} \/>/)
  assert.match(source, /isEmployeePortalMode = session\?\.supportMode === "employee_portal"/)
  assert.match(source, /isEmployeePortalMode \|\| \(!session\?\.supportGrantId && !session\?\.supportMode\)/)
  assert.match(source, /Boolean\(\s*isEmployeePortalMode \|\|/)
  assert.match(source, /pathname\?\.startsWith\("\/enterprise\/ticket-workbench"\)/)
  assert.match(source, /pathname\?\.startsWith\("\/enterprise\/workbench"\)/)
  assert.match(source, /pathname\?\.startsWith\("\/enterprise\/products"\)/)
})

test("enterprise shell shows my work schedule shortcut before engineer status", () => {
  assert.match(source, /isEmployeePortalMode \|\| CanAccessMenu\("agentTeamSchedule\.view", effectivePermissions\)/)
  assert.match(source, /workScheduleHref = "\/enterprise\/org\/schedules\?view=mine"/)
  assert.match(source, /href=\{workScheduleHref\}/)
  assert.match(source, /<span>我的工作时间表<\/span>/)
  assert.match(source, /canShowWorkScheduleShortcut && canShowEngineerBriefing/)
  assert.match(source, /我的工作时间表/)
  assert.match(source, /我的工作时间表[\s\S]*<EngineerLoginBriefingModal/)
})

import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")

test("product schedule page is sourced from member availability and leave", () => {
  assert.match(source, /fetchAgentTeamMemberAvailability/)
  assert.match(source, /setMemberAvailability/)
  assert.match(source, /data-testid="availability-summary"/)
  assert.match(source, /data-testid="product-effective-roster"/)
  assert.match(source, /os\(t, "memberStatus\.title"\)/)
  assert.match(source, /workdayRangeLabel\(member\.workdays\)/)
  assert.match(source, /member\.startTime} - \{member\.endTime/)
  assert.match(source, /member\.activeLeave/)
  assert.match(source, /member\.pendingLeave/)
  assert.match(source, /availabilityReasonLabel/)
  assert.match(source, /outside_personal_dispatch_rule/)
  assert.match(source, /os\(t, "metrics\.available"\)/)
  assert.match(source, /os\(t, "metrics\.hours24"\)/)
  assert.doesNotMatch(source, /共同决定候选池/)
})

test("product schedule page no longer exposes editable per-member schedule board actions", () => {
  assert.doesNotMatch(source, /createAgentTeamSchedule/)
  assert.doesNotMatch(source, /updateAgentTeamSchedule/)
  assert.doesNotMatch(source, /deleteAgentTeamSchedule/)
  assert.doesNotMatch(source, /prepareAgentTeamScheduleDraft/)
  assert.doesNotMatch(source, /publishAgentTeamSchedule/)
  assert.doesNotMatch(source, /rollbackAgentTeamSchedule/)
  assert.doesNotMatch(source, /disableAgentTeamSchedule/)
  assert.doesNotMatch(source, /ShiftEditorDialog/)
  assert.doesNotMatch(source, /setConfirmAction/)
  assert.doesNotMatch(source, /添加特殊覆盖/)
  assert.doesNotMatch(source, /编辑特殊覆盖/)
  assert.doesNotMatch(source, /发布特殊覆盖/)
})

test("historical special coverage is read-only and secondary to personal rules", () => {
  assert.match(source, /fetchAgentTeamSchedules/)
  assert.match(source, /data-testid="historical-special-coverage"/)
  assert.match(source, /os\(t, "coverage\.title"\)/)
  assert.match(source, /StatusTag tone="neutral">\{os\(t, "coverage\.readOnly"\)\}/)
  assert.match(source, /os\(t, "coverage\.noCoverage"\)/)
  assert.match(source, /item\.dayType === "rest"/)
  assert.match(source, /return os\(t, "scheduleTime\.rest"\)/)
  assert.match(source, /historicalCoverage\.map/)
  assert.doesNotMatch(source, /data-testid=\{`weekly-schedule-day-\$\{weekday\.value\}`\}/)
  assert.doesNotMatch(source, /onClick=\{\(\) => setEditor/)
  assert.doesNotMatch(source, /onClick=\{\(\) => setConfirmAction/)
})

test("product schedule page shows compact leave and reason panels", () => {
  assert.match(source, /data-testid="leave-review-panel"/)
  assert.match(source, /os\(t, "leavePanel\.pendingCount", \{ count: pendingLeaves\.length \}\)/)
  assert.match(source, /h2 className="font-semibold">\{os\(t, "metrics\.onLeave"\)\}<\/h2>/)
  assert.doesNotMatch(source, /新派单模型/)
  assert.doesNotMatch(source, /产品组只决定成员范围/)
  assert.doesNotMatch(source, /派单按个人工作日 24 小时规则/)
  assert.doesNotMatch(source, /批准后暂停接单/)
})

test("employee portal keeps personal work schedule and leave self-service", () => {
  assert.match(source, /fetchEngineerWorkSchedule/)
  assert.match(source, /myScheduleMode = isEmployeePortalMode \|\| searchParams\.get\("view"\) === "mine"/)
  assert.match(source, /title=\{myScheduleMode \? os\(t, "pageTitle\.mySchedule"\) : os\(t, "pageTitle\.productGroup"\)\}/)
  assert.match(source, /DEFAULT_PERSONAL_BASE_SCHEDULE/)
  assert.match(source, /startTime: "00:00"/)
  assert.match(source, /endTime: "24:00"/)
  assert.match(source, /os\(t, "actions\.applyLeave"\)/)
  assert.match(source, /os\(t, "toasts\.leaveRequestSubmitted"\)/)
  assert.doesNotMatch(source, /批准后，这段时间内你会从所属全部产品组的派单候选中移除/)
  assert.doesNotMatch(source, /DialogDescription/)
})

test("personal schedule merges base hours and product overrides into one weekly view", () => {
  assert.match(source, /setDraftSchedules\(data\.draftSchedules \|\| \[\]\)/)
  assert.match(source, /setPersonalBaseSchedule\(normalizePersonalBaseSchedule\(data\.baseSchedule\)\)/)
  assert.match(source, /normalizePersonalBaseSchedule\(data\.baseSchedule\)/)
  assert.match(source, /Array\.isArray\(value\?\.workdays\)/)
  assert.match(source, /data-testid="my-unified-weekly-schedule"/)
  assert.match(source, /os\(t, "mySchedule\.unifiedWeek"\)/)
  assert.match(source, /StatusTag tone="neutral">\{os\(t, "scope\.enterpriseBase"\)\}<\/StatusTag>/)
  assert.match(source, /draft \? os\(t, "scope\.draft"\) : os\(t, "scope\.overrideVersion", \{ version: item\.version \}\)/)
  assert.doesNotMatch(source, /data-testid=\{`my-product-schedule-\$\{team\.id\}`\}/)
  assert.doesNotMatch(source, /企业基础工作时间只显示一次/)
  assert.doesNotMatch(source, /负责人正式发布前不会改变你的实际派单范围/)
  assert.match(source, /empty-published-coverage-warning/)
  assert.match(source, /item\.userId === 0 \? os\(t, "scope\.teamWide"\) : os\(t, "scope\.onlyMe"\)/)
})

test("schedule page avoids explanatory helper copy", () => {
  const noisyCopy = [
    /批准前不会影响派单/,
    /工作时间对你支持的全部产品统一生效/,
    /产品成员关系决定你能处理哪些设备/,
    /待审批不影响派单/,
    /个人规则、请假、状态/,
    /仅只读/,
    /创建产品后会自动生成对应维修组/,
    /取消后你会重新进入所属全部产品组/,
    /不会影响你的派单状态/,
  ]

  for (const pattern of noisyCopy) {
    assert.doesNotMatch(source, pattern)
  }
})

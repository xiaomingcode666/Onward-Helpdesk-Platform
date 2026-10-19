import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")
const globalStyles = await readFile(new URL("../../globals.css", import.meta.url), "utf8")
const zhCN = JSON.parse(await readFile(new URL("../../../messages/zh-CN.json", import.meta.url), "utf8"))
const enUS = JSON.parse(await readFile(new URL("../../../messages/en-US.json", import.meta.url), "utf8"))
const esES = JSON.parse(await readFile(new URL("../../../messages/es-ES.json", import.meta.url), "utf8"))

test("quality sampling controls and analysis labels follow the active locale", () => {
  assert.match(source, /useAppLocale\(\)/)
  assert.match(source, /t\("analysis\.filters\.startDate"\)/)
  assert.match(source, /t\("analysis\.groups\.channel"\)/)
  assert.doesNotMatch(source, /<Input type="date"/)
  assert.doesNotMatch(source, />开始日期</)
  assert.doesNotMatch(source, />Project</)
  assert.doesNotMatch(source, />Team ID</)
  assert.doesNotMatch(source, />Agent ID</)
  assert.doesNotMatch(source, /analysisProject|analysisTeamID|analysis\.filters\.(project|team)/)
})

test("quality sampling localizes backend status and priority codes", () => {
  assert.match(source, /priorityLabel\(item\.priority\)/)
  assert.match(source, /ticketStatusLabel\(item\.ticket_status\)/)
  assert.doesNotMatch(source, /dataIndex: "priority"/)
  assert.doesNotMatch(source, /dataIndex: "ticket_status"/)
  assert.equal(zhCN.ticketQuality.ticketStatus.resolved, "已解决")
  assert.equal(enUS.ticketQuality.ticketStatus.resolved, "Resolved")
  assert.equal(zhCN.ticketQuality.priority.p3, "P3 中")
  assert.equal(enUS.ticketQuality.priority.p3, "P3 Medium")
})

test("quality analysis uses one intentional empty state before rendering result cards", () => {
  assert.match(source, /<Empty image=\{Empty\.PRESENTED_IMAGE_SIMPLE\}/)
  assert.match(source, /analysis \? t\("analysis\.empty"\) : t\("analysis\.notRun"\)/)
  assert.match(source, /analysisLists\.filter\(\(\[, , values\]\) => Boolean\(values\?\.length\)\)/)
})

test("sampling filters use equal responsive columns and local calendar date", () => {
  assert.match(source, /dayjs\(\)\.format\("YYYY-MM-DD"\)/)
  assert.match(source, /grid items-end gap-3 sm:grid-cols-2 xl:grid-cols-4/)
  assert.match(source, /DatePicker className="ticket-quality-date-picker h-9 w-full" allowClear=\{false\}/)
  assert.match(source, /Select className="h-9 w-full"/)
  assert.match(source, /Input className="h-9 w-full" value=\{search\}/)
  assert.doesNotMatch(source, /min-w-64 flex-1/)
})

test("quality date pickers render a single visual input frame", () => {
  assert.equal(source.match(/className="ticket-quality-date-picker h-9 w-full"/g)?.length, 3)
  assert.match(globalStyles, /\.ticket-quality-date-picker \.ant-picker-input > input\s*\{[^}]*background:\s*transparent !important;[^}]*border-radius:\s*0 !important;[^}]*box-shadow:\s*none !important;[^}]*outline:\s*none !important;/s)
})

test("quality analysis localizes category, channel, and missing dimension codes", () => {
  assert.match(source, /\["category", t\("analysis\.groups\.category"\)/)
  assert.match(source, /\["channel", t\("analysis\.groups\.channel"\)/)
  assert.match(source, /analysisBucketLabel\(dimension, item\)/)
  assert.match(source, /analysisBucketLabel\(dimension, bucket\)/)
  assert.match(source, /analysis\.values\.categories/)
  assert.match(source, /analysis\.values\.channels/)
  assert.equal(zhCN.ticketQuality.analysis.values.categories.incident, "故障事件")
  assert.equal(zhCN.ticketQuality.analysis.values.categories.problem, "根因问题")
  assert.equal(zhCN.ticketQuality.analysis.values.categories.user_case, "客户问题")
  assert.equal(zhCN.ticketQuality.analysis.values.channels.enterprise, "企业端")
  assert.equal(enUS.ticketQuality.analysis.values.channels.enterprise, "Enterprise portal")
  assert.equal(esES.ticketQuality.analysis.values.channels.enterprise, "Portal empresarial")
  assert.equal(zhCN.ticketQuality.analysis.values.unset, "未设置")
  assert.equal(enUS.ticketQuality.analysis.values.unset, "Not set")
  assert.equal(esES.ticketQuality.analysis.values.unset, "Sin configurar")
})

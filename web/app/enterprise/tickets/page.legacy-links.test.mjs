import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const pageSource = await readFile(new URL("./page.tsx", import.meta.url), "utf8")
const detailSource = await readFile(
  new URL("./_components/ticket-detail-content.tsx", import.meta.url),
  "utf8",
)
const redirectSource = await readFile(
  new URL("./[ticketId]/redirect-client.tsx", import.meta.url),
  "utf8",
)
const zhMessages = await readFile(new URL("../../../messages/zh-CN.json", import.meta.url), "utf8")
const enMessages = await readFile(new URL("../../../messages/en-US.json", import.meta.url), "utf8")
const esMessages = await readFile(new URL("../../../messages/es-ES.json", import.meta.url), "utf8")

test("enterprise ticket detail dialog only keeps the workbench action", () => {
  assert.match(pageSource, /进入处理工作台/)
  assert.doesNotMatch(pageSource, /打开详情页/)
  assert.doesNotMatch(pageSource, /查看原会话/)
  assert.doesNotMatch(pageSource, /detailConversationId/)
})

test("enterprise ticket detail content does not expose a legacy detail shortcut", () => {
  assert.doesNotMatch(detailSource, /showDetailLink/)
  assert.doesNotMatch(detailSource, /打开工单详情/)
  assert.doesNotMatch(detailSource, /ExternalLinkIcon/)
})

test("productless enterprise ticket detail rewrites legacy device and product-team timeline copy", () => {
  assert.match(detailSource, /localizeGeneratedTimelineContent\(content, showDeviceContext\)/)
  assert.match(detailSource, /showDeviceContext \? "ticketCreatedDescriptionWithTicketNo" : "ticketCreatedDescriptionWithTicketNoGeneral"/)
  assert.match(detailSource, /showDeviceContext \? "ticketSameProductTakeoverAccepted" : "ticketSupportTakeoverAccepted"/)
})

test("legacy enterprise ticket route recovers ticket identifiers from query params", () => {
  assert.match(redirectSource, /normalizeEnterpriseActionUrl/)
  assert.match(redirectSource, /searchParams\.get\("biz_id"\)/)
  assert.match(redirectSource, /searchParams\.get\("ticket_id"\)/)
  assert.doesNotMatch(redirectSource, /buildEnterpriseTicketWorkbenchPath\(ticketId\)/)
  assert.doesNotMatch(redirectSource, /enterprise\.ticketRedirect\.description/)
  assert.doesNotMatch(redirectSource, /description=\{/)
  assert.doesNotMatch(zhMessages, /请稍候，系统正在定位对应工单/)
  assert.doesNotMatch(enMessages, /Please wait while we locate the matching ticket/)
  assert.doesNotMatch(esMessages, /Please wait while we locate the matching ticket/)
})

test("enterprise ticket personal filter is URL-backed and applies to list and summary", () => {
  assert.match(pageSource, /searchParams\.get\("mine"\) === "1"/)
  assert.match(pageSource, /mine: mineOnly \|\| undefined/)
  assert.match(pageSource, /fetchTicketSummary\(\{[\s\S]*mine: mineOnly \|\| undefined/)
  assert.match(pageSource, /params\.set\("mine", "1"\)/)
  assert.match(pageSource, /aria-pressed=\{mineOnly\}/)
  assert.match(pageSource, /我的工单/)
  assert.match(pageSource, /router\.replace\("\/enterprise\/tickets", \{ scroll: false \}\)/)
})

test("enterprise tickets use the RailOps table and drawer pattern", () => {
  assert.match(pageSource, /@railops\/ui/)
  assert.match(pageSource, /<ContentModule[\s\S]*className="rhd-railops-ticket-list-module"/)
  assert.match(pageSource, /<FilterTabs[\s\S]*ariaLabel="工单筛选"/)
  assert.match(pageSource, /<AntTable<TicketListItem>/)
  assert.match(pageSource, /<AntPagination/)
  assert.match(pageSource, /<DetailDrawer[\s\S]*rootClassName="rhd-railops-ticket-drawer-root"/)
  assert.doesNotMatch(pageSource, /<table className="ticket-guoqi-table/)
  assert.doesNotMatch(pageSource, /ticket-center-pager/)
  assert.doesNotMatch(pageSource, /<DialogContent/)
})

test("enterprise tickets detail loading keeps context and avoids explanatory copy", () => {
  assert.match(pageSource, /ModuleLoading label="工单详情加载中" variant="detail" count=\{2\}/)
  assert.match(pageSource, /detailListTicket\?\.ticket_no/)
  assert.match(pageSource, /aria-busy=\{summaryLoading\}/)
  assert.doesNotMatch(pageSource, /正在加载工单详情/)
  assert.doesNotMatch(pageSource, /ticket-ops-stat-desc/)
  assert.doesNotMatch(pageSource, /覆盖产品售后全量工单/)
  assert.doesNotMatch(pageSource, /待受理、待派单或待接单/)
  assert.doesNotMatch(pageSource, /工程师、视频或供应商处理中/)
  assert.doesNotMatch(pageSource, /可复盘、可入库/)
  assert.doesNotMatch(pageSource, /只看我个人的工单/)
})

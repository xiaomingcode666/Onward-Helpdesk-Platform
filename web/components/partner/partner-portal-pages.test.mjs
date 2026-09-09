import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import test from "node:test"

const source = readFileSync(new URL("./partner-portal-pages.tsx", import.meta.url), "utf8")
const serviceSource = readFileSync(
  new URL("../../../internal/services/ticket_supplier_collaboration_service.go", import.meta.url),
  "utf8",
)

test("partner ticket detail uses ticket number as primary heading", () => {
  assert.match(
    source,
    /<h1[^>]*>\{ticket\.ticket_no \|\| `协作 #\$\{ticket\.id\}`\}<\/h1>/,
  )
})

test("partner list pages use the shared route breadcrumb header", () => {
  assert.match(source, /import \{ PageHeader \} from "@\/components\/layout\/page-header"/)

  const ranges = [
    ["overview", source.indexOf("export function PartnerOverviewPage()"), source.indexOf("export function PartnerTicketsPage()")],
    ["tickets", source.indexOf("export function PartnerTicketsPage()"), source.indexOf("export function PartnerConversationsPage()")],
    ["conversations", source.indexOf("export function PartnerConversationsPage()"), source.indexOf("export function PartnerVideoCollaborationPage()")],
    ["video", source.indexOf("export function PartnerVideoCollaborationPage()"), source.indexOf("export function PartnerTicketDetailPage()")],
    ["people", source.indexOf("export function PartnerPeoplePage()"), source.length],
  ]

  for (const [name, start, end] of ranges) {
    assert.ok(start >= 0 && end > start, `${name} source range should exist`)
    const pageSource = source.slice(start, end)
    assert.match(pageSource, /<PageHeader[\s\S]*title="/, `${name} should use PageHeader`)
    assert.doesNotMatch(pageSource, /<h1[^>]*>(供应商工作台|协作工单|处理工作台|视频协作|协作人员)<\/h1>/, `${name} should not keep hand-written page h1`)
    assert.doesNotMatch(pageSource, /<section className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">/, `${name} should not keep hand-written header shell`)
  }
})

test("partner portal exposes a conversation workbench route target", () => {
  assert.match(source, /export function PartnerConversationsPage\(\)/)
  assert.match(source, /fetchPartnerConversations\(\)/)
  assert.match(source, /href="\/partner\/conversations"/)
  assert.match(source, /partnerTicketDetailHref\(item\.id\)/)
})

test("productless supplier collaboration uses a generic service scope", () => {
  assert.match(source, /function collaborationScopeText\(/)
  assert.match(source, /if \(item\.product_id <= 0\) return pt\("fallback\.generalSupport"\)/)
  assert.match(source, /ticket\.product_id > 0 \? "partnerExtract\.fields\.productModule" : "partnerExtract\.fields\.collaborationScope"/)
  assert.match(source, /hasProductCollaborations \? "partnerExtract\.table\.productModule" : "partnerExtract\.table\.collaborationScope"/)
})

test("partner portal avoids legacy workbench naming", () => {
  assert.match(source, /title=\{t\("partnerExtract\.overview\.title"\)\}/)
  assert.match(source, /title=\{t\("partnerExtract\.conversations\.title"\)\}/)
  assert.match(source, />\{t\("partnerExtract\.nav\.conversations"\)\}<\/Button>/)
  assert.match(source, /aria-label=\{t\("partnerExtract\.overview\.refreshAria"\)\} title=\{t\("partnerExtract\.overview\.refreshAria"\)\}/)
  assert.match(source, />\{t\("partnerExtract\.detail\.title"\)\}<\/h1>/)
  assert.doesNotMatch(source, /供应商工作台|处理工作台|刷新工作台|加载供应商工作台失败|加载供应商处理工作台失败/)
})

test("partner overview is role aware and exposes administrator dispatch actions", () => {
  const overviewStart = source.indexOf("export function PartnerOverviewPage()")
  const overviewEnd = source.indexOf("export function PartnerTicketsPage()", overviewStart)
  const overviewSource = source.slice(overviewStart, overviewEnd)

  assert.match(overviewSource, /fetchPartnerProfile\(\)/)
  assert.match(overviewSource, /fetchPartnerAccounts\(\)/)
  assert.match(overviewSource, /profile\?\.can_manage_team/)
  assert.match(overviewSource, /acceptPartnerTicket\(item\.id\)/)
  assert.match(overviewSource, /assignPartnerTicket\(assigning\.id, Number\(assigneeID\)\)/)
  assert.match(overviewSource, /t\("partnerExtract\.overview\.teamLoad"\)/)
  assert.match(overviewSource, /t\("partnerExtract\.modals\.assignOwner"\)/)
})

test("partner overview statistics include acceptance, authorization risk, and recent closure", () => {
  const overviewStart = source.indexOf("export function PartnerOverviewPage()")
  const overviewEnd = source.indexOf("export function PartnerTicketsPage()", overviewStart)
  const overviewSource = source.slice(overviewStart, overviewEnd)

  assert.match(overviewSource, /t\("partnerExtract\.metrics\.pending"\)/)
  assert.match(overviewSource, /t\("partnerExtract\.metrics\.authorizationRisk"\)/)
  assert.match(overviewSource, /t\("partnerExtract\.metrics\.resolved7d"\)/)
  assert.match(overviewSource, /t\("partnerExtract\.metrics\.expiresWithin24h"\)/)
})

test("partner list workspaces keep page shell during initial loading", () => {
  const overviewStart = source.indexOf("export function PartnerOverviewPage()")
  const ticketsStart = source.indexOf("export function PartnerTicketsPage()", overviewStart)
  const conversationsStart = source.indexOf("export function PartnerConversationsPage()", ticketsStart)
  const videoStart = source.indexOf("export function PartnerVideoCollaborationPage()", conversationsStart)
  const peopleStart = source.indexOf("export function PartnerPeoplePage()")
  const overviewSource = source.slice(overviewStart, ticketsStart)
  const ticketSource = source.slice(ticketsStart, conversationsStart)
  const conversationsSource = source.slice(conversationsStart, videoStart)
  const videoSource = source.slice(videoStart, source.indexOf("export function PartnerTicketDetailPage()", videoStart))
  const peopleSource = source.slice(peopleStart)
  const pageLoadingPattern = new RegExp("if \\(loading\\) return <" + "Page" + "Loading")

  assert.doesNotMatch(overviewSource, pageLoadingPattern)
  assert.doesNotMatch(ticketSource, pageLoadingPattern)
  assert.doesNotMatch(conversationsSource, pageLoadingPattern)
  assert.doesNotMatch(videoSource, pageLoadingPattern)
  assert.doesNotMatch(peopleSource, pageLoadingPattern)
  assert.match(overviewSource, /initialLoading \? \(/)
  assert.match(ticketSource, /initialLoading \? \(/)
  assert.match(conversationsSource, /initialLoading \? \(/)
  assert.match(videoSource, /initialLoading \? \(/)
  assert.match(peopleSource, /initialLoading \? \(/)
  assert.match(conversationsSource, /const \[itemsLoaded, setItemsLoaded\] = useState\(false\)/)
  assert.match(videoSource, /const \[meetingsLoaded, setMeetingsLoaded\] = useState\(false\)/)
  assert.match(peopleSource, /const \[peopleLoaded, setPeopleLoaded\] = useState\(false\)/)
  assert.match(conversationsSource, /const initialLoading = loading && !itemsLoaded/)
  assert.match(videoSource, /const initialLoading = loading && !meetingsLoaded/)
  assert.match(peopleSource, /const initialLoading = loading && !peopleLoaded/)
  assert.doesNotMatch(videoSource, /meetingsData === null && loading/)
  assert.doesNotMatch(peopleSource, /profile === null && items\.length === 0 && loading/)
  assert.doesNotMatch(videoSource, /loading\s+&&\s+!meetingsData/)
  assert.doesNotMatch(peopleSource, /loading\s+&&\s+!profile/)
  assert.match(ticketSource, /t\("partnerExtract\.loading\.ticketList"\)/)
  assert.match(conversationsSource, /t\("partnerExtract\.loading\.conversationList"\)/)
  assert.match(videoSource, /t\("partnerExtract\.loading\.collaborationList"\)/)
  assert.match(peopleSource, /t\("partnerExtract\.loading\.peopleList"\)/)
})

test("partner portal empty states stay concise", () => {
  assert.doesNotMatch(source, /<EmptyState[\s\S]{0,200}description=/)
  assert.doesNotMatch(source, /DialogDescription/)
  assert.doesNotMatch(source, /当前没有未完成的供应商协作任务|当前筛选下没有供应商协作任务|当前账号没有匹配的供应商协作会话/)
  assert.doesNotMatch(source, />当前账号</)
  assert.doesNotMatch(source, /指派后新负责人会获得当前协作会话权限|原负责人仍保留协作记录|初始密码仅在本次创建后显示/)
  assert.doesNotMatch(source, /请选择左侧会话|请选择左侧会议|未找到可访问的供应商协作工单|当前供应商公司没有匹配的协作账号/)
  assert.match(source, /t\("partnerExtract\.common\.me"\)/)
})

test("partner detail surfaces use inline loading modules", () => {
  const detailStart = source.indexOf("function PartnerTicketDetailView(")
  const meetingRoomStart = source.indexOf("export function PartnerMeetingRoomPage()")
  const detailSource = source.slice(detailStart, meetingRoomStart)
  const meetingRoomSource = source.slice(meetingRoomStart)

  assert.doesNotMatch(source, new RegExp("Page" + "Loading"))
  assert.match(detailSource, /t\("partnerExtract\.loading\.ticketContext"\)/)
  assert.match(detailSource, /t\("partnerExtract\.loading\.messages"\)/)
  assert.match(detailSource, /t\("partnerExtract\.loading\.audit"\)/)
  assert.match(meetingRoomSource, /t\("partnerExtract\.loading\.collaborationAuthorization"\)/)
})

test("partner collaboration audit is rendered in the conversation context rail", () => {
  const detailStart = source.indexOf("function PartnerTicketDetailView(")
  const detailEnd = source.indexOf("function parseLanguages(", detailStart)
  const detailSource = source.slice(detailStart, detailEnd)
  const contextRailStart = detailSource.indexOf("<aside className=\"space-y-3\">")
  const auditStart = detailSource.indexOf('data-testid="partner-collaboration-audit"')

  assert.ok(contextRailStart >= 0)
  assert.ok(auditStart > contextRailStart)
  assert.match(detailSource, /max-h-80[^\n]*overflow-y-auto/)
})

test("partner video page is meeting scoped and does not embed conversation workbench", () => {
  const start = source.indexOf("export function PartnerVideoCollaborationPage()")
  const end = source.indexOf("export function PartnerTicketDetailPage()", start)
  const videoSource = source.slice(start, end)

  assert.match(videoSource, /fetchPartnerMeetings\(status\)/)
  assert.match(videoSource, /setSelectedMeetingID\(meeting\.id\)/)
  assert.match(videoSource, /t\("partnerExtract\.actions\.ticketDetail"\)/)
  assert.doesNotMatch(videoSource, /fetchPartnerTickets\(\)/)
  assert.doesNotMatch(videoSource, /PartnerTicketDetailView/)
  assert.doesNotMatch(videoSource, /可协作会话|协作会话/)
  assert.doesNotMatch(videoSource, /Jitsi 会议|会议加载失败|会议详情加载失败|加入会议/)
  assert.match(videoSource, /t\("partnerExtract\.video\.jitsiCollaboration"\)/)
  assert.match(videoSource, /t\("partnerExtract\.actions\.joinCollaboration"\)/)
})

test("partner video page treats backend ended meetings as ended archive records", () => {
  const start = source.indexOf("export function PartnerVideoCollaborationPage()")
  const end = source.indexOf("export function PartnerTicketDetailPage()", start)
  const videoSource = source.slice(start, end)

  assert.match(source, /case "ended":[\s\S]*case "finished":[\s\S]*pt\("status\.meeting\.ended"\)/)
  assert.match(videoSource, /useState<"all" \| "active" \| "waiting" \| "ended">\("all"\)/)
  assert.match(videoSource, /\["ended", t\("partnerExtract\.status\.meeting\.ended"\)\]/)

  const roomStart = source.indexOf("export function PartnerMeetingRoomPage()")
  const roomSource = source.slice(roomStart)
  assert.match(roomSource, /const \[archiveMode, setArchiveMode\] = useState\(false\)/)
  assert.match(roomSource, /setArchiveMode\(true\)/)
  assert.match(roomSource, /t\("partnerExtract\.meetingArchive\.endedTitle"\)[\s\S]*t\("partnerExtract\.meetingArchive\.transcriptsTitle"\)/)
})

test("partner historical meetings keep read-only status and transcript access", () => {
  assert.match(serviceSource, /GetMeetingStatus[\s\S]*requirePartnerMeetingAccess\(ctx, collaborationID, meetingID, operator, true, true\)/)
  assert.match(serviceSource, /ListMeetingTranscriptPage[\s\S]*requirePartnerMeetingAccess\(context\.Background\(\), collaborationID, meetingID, operator, true, true\)/)
  assert.match(serviceSource, /JoinMeeting[\s\S]*requirePartnerMeetingAccess\(ctx, collaborationID, meetingID, operator, false, false\)/)
})

test("partner account selects map account ids to display names", () => {
  assert.match(source, /function partnerAccountSelectOptions\(accounts: PartnerPortalAccount\[\]\)/)
  assert.match(source, /items=\{assignableAccountOptions\}/)
  assert.match(source, /items=\{availableParticipantOptions\}/)
  assert.match(source, /label: account\.display_name \|\| account\.username/)
})

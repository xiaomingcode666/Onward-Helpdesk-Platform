import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const enterpriseSource = await readFile(new URL("./page.tsx", import.meta.url), "utf8")
const partnerSource = await readFile(
  new URL("../../../components/partner/partner-portal-pages.tsx", import.meta.url),
  "utf8",
)
const agentConversationStoreSource = await readFile(
  new URL("../../../lib/stores/agent-conversations.ts", import.meta.url),
  "utf8",
)
const agentRealtimeHookSource = await readFile(
  new URL("../../../hooks/use-agent-conversation-realtime.ts", import.meta.url),
  "utf8",
)
const serviceEventSource = await readFile(
  new URL("../../../components/remote-helpdesk/conversation-service-event.tsx", import.meta.url),
  "utf8",
)

test("enterprise ticket workbench visible textareas have explicit accessible names", () => {
  for (const label of [
    "回复客户会话",
    "给供应商发送协作消息",
    "供应商处理结论",
    "供应商协作问题",
    "问题描述",
    "填写取消原因",
    "输入处理内容",
    "根因",
    "处理方案与客户注意事项",
  ]) {
    assert.match(enterpriseSource, new RegExp(`aria-label="${label}"`))
  }
})

test("enterprise ticket workbench uses contextual loading skeletons and terse service labels", () => {
  assert.match(enterpriseSource, /import \{ Skeleton \} from "@\/components\/ui\/skeleton"/)
  assert.match(enterpriseSource, /role="status"/)
  assert.match(enterpriseSource, /aria-busy="true"/)
  assert.match(enterpriseSource, /<Skeleton className="h-3 w-4\/5" \/>/)
  for (const label of ["策略加载中", "附件加载中", "供应商协作加载中", "知识候选加载中"]) {
    assert.match(enterpriseSource, new RegExp(label))
  }
  assert.match(enterpriseSource, /label: "在线接待"/)
  assert.doesNotMatch(enterpriseSource, /label: "服务助手"/)
  assert.doesNotMatch(enterpriseSource, /自动回复消息/)
  assert.match(enterpriseSource, /组内成员可接单/)
  assert.doesNotMatch(enterpriseSource, /AI 接待/)
  assert.doesNotMatch(enterpriseSource, /正在加载策略/)
  assert.doesNotMatch(enterpriseSource, /正在加载上下文/)
  assert.doesNotMatch(enterpriseSource, /正在加载供应商协作/)
  assert.doesNotMatch(enterpriseSource, /正在加载知识候选/)
  assert.doesNotMatch(enterpriseSource, /系统按个人可接单规则/)
  assert.doesNotMatch(enterpriseSource, /工作台队列加载失败/)
  assert.doesNotMatch(enterpriseSource, /这里保留会话、维修、供应商、评价和知识沉淀记录/)
  assert.doesNotMatch(enterpriseSource, /客户反馈复发/)
  assert.match(enterpriseSource, /处理队列加载失败/)
})

test("supplier invitation exposes an explicit authorization deadline", () => {
  assert.match(enterpriseSource, /<span>供应商授权截止时间<\/span>/)
  assert.match(enterpriseSource, /authorization_ends_at: authorizationEndsAt/)
  assert.match(enterpriseSource, /collaboration\.authorization_active/)
  assert.match(enterpriseSource, /collaboration\.status === "expired" \? "授权已过期"/)
})

test("expired product-team assignments expose an explicit self-takeover action", () => {
  assert.match(enterpriseSource, /actions\?\.can_takeover === true/)
  assert.match(enterpriseSource, /canTakeOverTicket \? "转到我名下"/)
  assert.match(enterpriseSource, /"工单已转到你名下，可以开始回复客户"/)
  assert.match(enterpriseSource, /refreshTicketSnapshot\(pinnedTicket\.id\)/)
  assert.match(enterpriseSource, /await refreshTicketSnapshot\(ticket\.id\)/)
  assert.match(enterpriseSource, /"pending_assignee_accept", "assigned", "reopened"/)
})

test("reply composer explains non-acceptance disabled states accurately", () => {
  assert.match(enterpriseSource, /const selectedReplyDisabledReason = selectedTicketRequiresAcceptance/)
  assert.match(enterpriseSource, /"当前会话不可回复，请查看工单处理状态"/)
  assert.match(enterpriseSource, /replyDisabledReason=\{selectedReplyDisabledReason\}/)
  assert.match(enterpriseSource, /replyDisabledReason !== ""/)
  assert.doesNotMatch(enterpriseSource, /replyDisabled=\{selectedTicketRequiresAcceptance \|\| selectedItem\?\.conversation\?\.status !== 3\}/)
})

test("selected workbench ticket exposes dispatch evidence for acceptance review", () => {
  assert.match(enterpriseSource, /function dispatchEvidenceParts/)
  assert.match(enterpriseSource, /ee\(hasDeviceConcept \? "ticketWorkbench\.text072" : "ticketWorkbench\.text325"/)
  assert.match(enterpriseSource, /ee\("ticketWorkbench\.text073", \{ value0: ticket\.assignee_name \}\)/)
  assert.match(enterpriseSource, /ee\("ticketWorkbench\.text074", \{ value0: ticket\.dispatch_attempts \}\)/)
  assert.match(enterpriseSource, /data-testid="enterprise-dispatch-evidence"/)
})

test("knowledge-support tickets rewrite legacy device and product-team lifecycle copy", () => {
  assert.match(enterpriseSource, /buildAggregateDetail\(aggregate, hasDeviceConcept\)/)
  assert.match(enterpriseSource, /hasDeviceConcept \? "ticketCreatedDescriptionWithTicketNo" : "ticketCreatedDescriptionWithTicketNoGeneral"/)
  assert.match(enterpriseSource, /hasDeviceConcept \? "ticketSameProductTakeoverAccepted" : "ticketSupportTakeoverAccepted"/)
})

test("partner collaboration textareas have explicit accessible names", () => {
  assert.match(partnerSource, /aria-label=\{t\("partnerExtract\.placeholders\.resolution"\)\}/)
  assert.match(partnerSource, /aria-label=\{t\("partnerExtract\.fields\.replyConversation"\)\}/)
})

test("enterprise ticket workbench keeps chat messages visible during silent refreshes", () => {
  assert.match(enterpriseSource, /syncLatestMessages/)
  assert.doesNotMatch(enterpriseSource, /window\.setInterval\(\(\) => void refresh\(\), 5_000\)/)
  assert.match(enterpriseSource, /loadingMessages && messages\.length === 0/)
  assert.match(enterpriseSource, /messages\.length > 0 \?/)
  assert.match(enterpriseSource, /hasMoreOlderMessages/)
  assert.match(enterpriseSource, /handleLoadOlderMessages/)
  assert.match(enterpriseSource, /newMessageNoticeCount/)
  assert.match(enterpriseSource, /handleJumpToLatestMessages/)
  assert.match(enterpriseSource, /aria-label="查看新消息"/)
  assert.match(enterpriseSource, /lastHandledMessageSignatureRef/)
  assert.doesNotMatch(enterpriseSource, /\{\s*loadingMessages \? \(/)
})

test("enterprise ticket workbench keeps canonical ticket route parameters", () => {
  assert.match(enterpriseSource, /url\.searchParams\.delete\("ticketId"\)/)
  assert.match(enterpriseSource, /url\.searchParams\.set\("ticket_id", String\(item\.ticketId\)\)/)
  assert.doesNotMatch(enterpriseSource, /url\.searchParams\.set\("ticketId"/)
})

test("enterprise ticket workbench visually separates message identities", () => {
  for (const label of ["在线接待", "人工客服", "供应商", "客户"]) {
    assert.match(enterpriseSource, new RegExp(label))
  }
  assert.doesNotMatch(enterpriseSource, /AI客服机器人/)
  assert.match(enterpriseSource, /data-message-tone/)
  assert.match(enterpriseSource, /tone: isCustomer \? "customer"/)
  assert.match(enterpriseSource, /side: isCustomer \|\| isPartner \? "left" : "right"/)
  assert.match(enterpriseSource, /getMessageToneMeta/)
})

test("enterprise ticket workbench normalizes legacy display copy only", () => {
  assert.match(enterpriseSource, /function normalizeEnterpriseWorkbenchMessageCopy/)
  assert.match(enterpriseSource, /LEGACY_CONVERSATION_WORKBENCH_TITLE_PATTERN/)
  assert.match(enterpriseSource, /\.replaceAll\(LEGACY_CONVERSATION_WORKBENCH_TITLE_PATTERN, "会话处理"\)/)
  assert.match(serviceEventSource, /\.replaceAll\(LEGACY_CONVERSATION_WORKBENCH_TITLE_PATTERN, cee\(t, "normalizeConversationHandling"\)\)/)
  assert.doesNotMatch(enterpriseSource, /售后工单["'`]\s*,\s*["'`]指挥台/)
  assert.match(enterpriseSource, /serviceSummary: normalizeEnterpriseWorkbenchMessageCopy/)
  assert.match(enterpriseSource, /const serviceSummary = normalizeEnterpriseWorkbenchMessageCopy/)
  assert.match(serviceEventSource, /function normalizeEnterpriseServiceEventCopy/)
  assert.match(serviceEventSource, /return cee\(t, "videoActionEnterprise"\)/)
  assert.match(serviceEventSource, /LEGACY_VIDEO_MEETING_TITLE_PATTERN/)
  assert.match(serviceEventSource, /\.replaceAll\(LEGACY_VIDEO_MEETING_TITLE_PATTERN, cee\(t, "normalizeVideoCollaboration"\)\)/)
  assert.doesNotMatch(serviceEventSource, /视频["'`]\s*,\s*["'`]会议/)
})

test("agent conversation realtime resync merges latest messages without resetting the thread", () => {
  assert.match(agentRealtimeHookSource, /store\.applyRealtimeMessageCreated\(message\)/)
  assert.match(agentRealtimeHookSource, /store\.resyncRealtimeData\(conversationId \?\? undefined\)/)
  assert.match(agentConversationStoreSource, /await get\(\)\.syncLatestMessages\(targetConversationId\)/)
  assert.match(agentConversationStoreSource, /messagesLoadedConversationId: conversationId/)
  assert.doesNotMatch(agentConversationStoreSource, /forceLoading: false,\s*reset: false/)
})

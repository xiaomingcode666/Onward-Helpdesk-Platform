import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./customer-portal-pages.tsx", import.meta.url), "utf8")
const paginationSource = await readFile(new URL("../list-pagination.tsx", import.meta.url), "utf8")
const localeMessages = await Promise.all(
  ["zh-CN", "en-US", "es-ES"].map(async (locale) => ({
    locale,
    messages: JSON.parse(await readFile(new URL(`../../messages/${locale}.json`, import.meta.url), "utf8")),
  }))
)

test("customer portal page headers use the shared route breadcrumb contract", () => {
  assert.match(source, /import \{ PageHeader as SharedPageHeader \} from "@\/components\/layout\/page-header"/)
  assert.match(source, /function PageHeader\([\s\S]*<SharedPageHeader/)
  assert.doesNotMatch(source, /eyebrow=\{t\("customer(Chat|Tickets|Meeting|Profile)\.pageEyebrow"\)\}/)
  assert.doesNotMatch(source, /eyebrow=\{t\("customerDevices\.sectionLabel"\)\}/)
  assert.doesNotMatch(source, /eyebrow: string/)
  assert.doesNotMatch(source, /<SharedPageHeader[\s\S]{0,180}eyebrow=/)
})

test("customer portal avoids low-value visible description copy", () => {
  assert.doesNotMatch(source, /BotIcon/)
  assert.doesNotMatch(source, /customerChat\.serviceContextDescription/)
  assert.doesNotMatch(source, /customerChat\.dialogGeneralDescription/)
  assert.doesNotMatch(source, /customerChat\.dialogBindDescription/)
  assert.doesNotMatch(source, /customerTickets\.timelineDescription/)
  assert.doesNotMatch(source, /customerTickets\.repairResultDescription/)
  assert.doesNotMatch(source, /customerMeeting\.noRealtimeTranscriptDescription/)
  assert.match(source, /customerTickets\.noRepairResult/)

  for (const { locale, messages } of localeMessages) {
    assert.equal(Object.hasOwn(messages.customerChat, "serviceContextDescription"), false, `${locale} should not keep serviceContextDescription`)
    assert.equal(Object.hasOwn(messages.customerChat, "dialogGeneralTitle"), false, `${locale} should not keep dialogGeneralTitle`)
    assert.equal(Object.hasOwn(messages.customerChat, "dialogGeneralDescription"), false, `${locale} should not keep dialogGeneralDescription`)
    assert.equal(Object.hasOwn(messages.customerChat, "dialogBindDescription"), false, `${locale} should not keep dialogBindDescription`)
    assert.equal(Object.hasOwn(messages.customerChat, "createGeneralConversation"), false, `${locale} should not keep createGeneralConversation`)
    assert.equal(Object.hasOwn(messages.customerTickets, "timelineDescription"), false, `${locale} should not keep timelineDescription`)
    assert.equal(Object.hasOwn(messages.customerTickets, "repairResultDescription"), false, `${locale} should not keep repairResultDescription`)
    assert.equal(Object.hasOwn(messages.customerTickets, "noRepairResult"), true, `${locale} should keep a concise repair empty state`)
    assert.equal(Object.hasOwn(messages.customerMeeting, "noRealtimeTranscriptDescription"), false, `${locale} should not keep noRealtimeTranscriptDescription`)
    assert.equal(Object.hasOwn(messages.customerChat, "systemHelpTitle"), false, `${locale} should not keep systemHelpTitle`)
    assert.equal(Object.hasOwn(messages.customerChat, "systemHelpPrompt"), false, `${locale} should not keep systemHelpPrompt`)
    assert.equal(Object.hasOwn(messages.customerChat, "openSystemHelp"), false, `${locale} should not keep openSystemHelp`)
    assert.equal(Object.hasOwn(messages.customerChat, "systemIntroDocsTitle"), true, `${locale} should keep system intro docs title`)
    assert.equal(Object.hasOwn(messages.customerChat, "systemIntroBack"), true, `${locale} should keep system intro back action`)
    assert.equal(Object.hasOwn(messages.customerChat, "systemIntroOpenOriginal"), true, `${locale} should keep system intro open original action`)
  }
})

test("general consultation: non-guest creates a conversation, guest opens system intro docs", () => {
  assert.match(source, /function handleCreateSystemHelpConversation\(\)[\s\S]*createOrMatchImConversation\(\{\s*general: true,\s*forceNew: true,\s*\}\)/)
  assert.match(source, /const isGuest = hasDeviceConcept && \(profile/)
  assert.match(source, /const generalConsultationTitle = t\("customerChat\.generalConsultation"\)/)
  assert.match(source, /if \(isGuest\) \{\s*closeCreateDialog\(\);\s*setDocsOpen\(true\) \} else \{\s*void handleCreateSystemHelpConversation\(\) \}/)
  assert.match(source, /<CustomerSystemIntroDocsModal open=\{docsOpen\} onClose=\{\(\) => setDocsOpen\(false\)\} \/>/)
  assert.match(source, /if \(requestedDevice\) \{\s*setCreateDeviceId\(String\(requestedDevice\.id\)\)/)
  assert.doesNotMatch(source, /conversationData\[0\]\?\.device_id/)
})

test("customer can bind a new device from the conversation dialog and enter a forced device conversation", () => {
  assert.match(source, /const \[createDialogView, setCreateDialogView\] = useState<"list" \| "bind">\("list"\)/)
  assert.match(source, /function openBindAndConsultDialog\(\)[\s\S]*setCreateDialogView\("bind"\)/)
  assert.match(source, /function closeCreateDialog\(\)[\s\S]*setCreateDialogView\("list"\)/)
  assert.match(source, /function handleBoundDeviceConversation\(binding: BindCustomerDeviceResponse\)[\s\S]*createOrMatchImConversation\(\{\s*deviceId,\s*forceNew: true,\s*\}\)/)
  assert.match(source, /activateCreatedConversation\(created, boundDevice\)/)
  assert.match(source, /successMessage=\{t\("customerOnboarding\.conversationSuccess"\)\}/)
  assert.match(source, /createDialogView === "bind" \? \([\s\S]*<CustomerDeviceBindForm/)
  assert.doesNotMatch(source, /aiAgentId|workflowId/)
})

test("customer device binding dialog is shared by chat and device pages", () => {
  assert.match(source, /function CustomerDeviceBindDialog\(/)
  assert.match(source, /formId="conversation-bind-device-form"/)
  assert.match(source, /formId="bind-device-form"/)
  assert.match(source, /fieldIdPrefix="conversation-bind-device"/)
  assert.match(source, /fieldIdPrefix="bind"/)
  assert.match(source, /onBound=\{handleDeviceBound\}/)
  assert.doesNotMatch(source, /async function submitBinding\(event: React\.FormEvent/)
})

test("new conversations become stable URLs after selection", () => {
  assert.match(source, /new URLSearchParams\(window\.location\.search\)/)
  assert.match(source, /window\.history\.replaceState\(window\.history\.state, "", adaptCustomerPortalPath\(nextUrl\)\)/)
  assert.match(source, /syncCustomerConversationUrl\(created\.id\)/)
  assert.match(source, /syncCustomerConversationUrl\(item\.id\)/)
  assert.match(source, /const conversationQueryParam = searchParams\?\.get\("conversationId"\) \|\| ""/)
  assert.match(source, /const selectedConversationIdRef = useRef\(0\)/)
  assert.match(source, /const messageListRequestRef = useRef\(0\)/)
  assert.match(source, /selectedConversationIdRef\.current = created\.id[\s\S]*?messageListRequestRef\.current \+= 1/)
  assert.match(source, /currentURLId === selectedConversationId/)
  assert.match(source, /requestId !== messageListRequestRef\.current \|\| selectedConversationIdRef\.current !== conversationId/)
})

test("device new query starts a forced device conversation instead of falling back to old chats", () => {
  assert.match(source, /const requestedDeviceIdParam = searchParams\?\.get\("deviceId"\) \|\| ""/)
  assert.match(source, /const newConversationParam = searchParams\?\.get\("new"\) \|\| ""/)
  assert.match(source, /autoCreateDeviceRequestRef/)
  assert.match(source, /autoCreateDeviceRequestRef\.current = ""/)
  assert.match(source, /if \(newConversationParam !== "1"\)/)
  assert.match(source, /if \(deviceId <= 0 \|\| creatingConversation \|\| !hasLoadedConversations\)/)
  assert.match(source, /newConversationParam === "1" && !requestedDevice/)
  assert.match(source, /createOrMatchImConversation\(\{ deviceId, forceNew: true \}\)/)
  assert.match(source, /activateCreatedConversation\(created, requestedDevice\)/)
  assert.match(source, /setCreateOpen\(false\)/)
  assert.doesNotMatch(source, /router\.push\(`\/customer\/chat\\?deviceId=\$\{selectedDevice\.id\}`\)/)
})

test("closed system help conversations do not claim a ticket or repair archive", () => {
  assert.match(source, /const isGeneralConsultation = Boolean\(/)
  assert.match(source, /closedTimelineGeneralTitle/)
  assert.match(source, /closedTimelineDeviceTitle/)
  assert.match(source, /closedTimelineTicketDescription/)
  assert.doesNotMatch(source, /服务结果已同步到设备档案/)
})

test("mobile conversation navigation keeps list and detail as separate views", () => {
  assert.match(source, /mobileConversationListOpen \|\| !selectedConversation \? "flex" : "hidden"/)
  assert.match(source, /mobileConversationListOpen \|\| !selectedConversation \? "hidden" : "flex"/)
  assert.match(source, /aria-label=\{t\("customerChat\.backToList"\)\}/)
})

test("customer chat shows a stable initial loading workspace with spinners", () => {
  assert.match(source, /const \[hasLoadedConversations, setHasLoadedConversations\] = useState\(false\)/)
  assert.match(source, /const initialConversationLoad = !hasLoadedConversations && loading/)
  assert.match(source, /setHasLoadedConversations\(true\)/)
  assert.match(source, /initialConversationLoad \? <CustomerChatWorkspaceLoading \/>/)
  assert.match(source, /function CustomerChatWorkspaceLoading\(\)[\s\S]*role="status"[\s\S]*aria-label=\{t\("customerChat\.loadingWorkspace"\)\}/)
  assert.match(source, /Loader2Icon[\s\S]*t\("customerChat\.loadingConversation"\)/)
  assert.match(source, /aria-label=\{t\("customerChat\.loadingMessages"\)\}[\s\S]*t\("customerChat\.loadingMessages"\)/)
  assert.match(source, /t\("customerChat\.refreshFailed"\)/)
})

test("customer conversation directory stays searchable and compact", () => {
  assert.match(source, /<SearchField[\s\S]*value=\{conversationSearchInput\}[\s\S]*customerChat\.searchPlaceholder/)
  assert.match(source, /setTimeout\(\(\) => \{\s*setConversationPage\(1\)\s*setConversationQuery\(conversationSearchInput\)\s*\}, 250\)/)
  assert.match(source, /const requestId = \+\+conversationListRequestRef\.current/)
  assert.match(source, /if \(requestId !== conversationListRequestRef\.current\) return \[\]/)
  assert.match(source, /loadBaseData\(false, undefined, false\)/)
  assert.match(source, /void loadCustomerChatDevices\(\)[\s\S]*\}, \[t, newConversationParam, requestedDeviceIdParam, hasDeviceConcept\]\)/)
  assert.match(source, /compactCustomerIdentifier\(item\.device_no\)/)
  assert.match(source, /aria-current=\{selected \? "true" : undefined\}/)
  assert.match(source, /item\.customer_unread_count > 0 \? [\s\S]* : null/)
  assert.match(source, /<ListPagination[\s\S]*compact[\s\S]*pageSizeOptions=\{\[10, 20, 50\]\}/)
  assert.match(source, /loadMessages\(selectedConversationId\)\s*\}, \[selectedConversationId\]\)/)
  assert.equal((source.match(/loadBaseDataRef\.current\(true\)/g) ?? []).length, 2)
  assert.match(paginationSource, /compact\?: boolean/)
  assert.match(paginationSource, /!compact \? <span className="tabular-nums">/)
  assert.match(paginationSource, /!compact \|\| totalPages > 1/)
  assert.match(paginationSource, /aria-label=\{compact \? t\("pagination\.previous"\) : undefined\}/)
})

test("conversation actions expose human handoff without a ticket-creation message", () => {
  assert.match(source, /aria-label=\{t\("customerChat\.requestHuman"\)\}/)
  assert.match(source, /const refreshedSelected = nextConversations\.find\(\(item\) => item\.id === selectedConversationIdRef\.current\)/)
  assert.match(source, /setSelectedConversationDetail\(refreshedSelected\)/)
  assert.doesNotMatch(source, />\s*创建工单\s*</)
  assert.doesNotMatch(source, /content: "<p>请帮我创建工单<\/p>"/)
  assert.doesNotMatch(source, /CustomerTicketRequestDialog/)
})

test("knowledge-support ticket search does not mention devices", () => {
  assert.match(source, /hasDeviceConcept \? "customerTickets\.searchLabel" : "customerTickets\.searchLabelGeneral"/)
  for (const { locale, messages } of localeMessages) {
    assert.ok(messages.customerTickets.searchLabelGeneral, `${locale} should provide a general ticket search label`)
    assert.doesNotMatch(messages.customerTickets.searchLabelGeneral, /device|设备/i)
  }
})

test("customer conversation page exposes message translation controls", () => {
  assert.match(source, /translateCustomerConversationMessage/)
  assert.match(source, /const \[translationTargetLanguage, setTranslationTargetLanguage\] = useState\("zh-CN"\)/)
  assert.match(source, /resolveCustomerMessageTranslationTarget\(message\.content, translationTargetLanguage\)/)
  assert.doesNotMatch(source, /data-testid="customer-translation-target-language"/)
  assert.doesNotMatch(source, /aria-label="翻译目标语言"/)
  assert.match(source, /onTranslateMessage=\{handleTranslateMessage\}/)
  assert.match(source, /t\("customerChat\.translationAutoTarget"/)
})

test("customer ticket and device detail pages can return to the source conversation", () => {
  assert.match(source, /function buildCustomerChatReturnPath\(conversationId: unknown\)/)
  assert.match(source, /function buildCustomerTicketPath\(ticketId: unknown, fromConversationId\?: unknown\)/)
  assert.match(source, /function buildCustomerDevicePath\(deviceId: unknown, fromConversationId\?: unknown, tab\?: string\)/)
  assert.match(source, /buildCustomerTicketPath\(selectedTicketId, selectedConversationId\)/)
  assert.match(source, /buildCustomerDevicePath\(selectedDevice\.id, selectedConversationId\)/)
  assert.match(source, /searchParams\?\.get\("fromConversationId"\)/)
  assert.match(source, /buildCustomerChatReturnPath\(returnConversationId\)/)
  assert.match(source, /router\.replace\(buildCustomerTicketPath\(selectedId, returnConversationId\)\)/)
  assert.match(source, /router\.replace\(buildCustomerDevicePath\(selectedId, returnConversationId, deviceTab\)\)/)
  assert.match(source, /t\("customerTickets\.backToConversation"\)/)
  assert.match(source, /t\("customerDevices\.backToConversation"\)/)
})

test("general AI-only conversations do not imply an engineer has joined", () => {
  assert.match(source, /const humanSupportVisible = selectedConversation/)
  assert.match(source, /selectedConversation\.human_handoff_enabled !== false/)
  assert.match(source, /humanSupportVisible && engineerOnline/)
  assert.match(source, /humanSupportVisible && supplierOnlineCount > 0/)
  assert.match(source, /latestMessage\?\.senderType\.toLowerCase\(\) === "customer"/)
  assert.match(source, /assistantResponding[\s\S]*\? t\("customerChat\.realtimeAssistantResponding"\)[\s\S]*: t\("customerChat\.realtimeAssistantOnline"\)/)
})

test("assistant response status is visually prominent and accessible", () => {
  assert.match(source, /data-testid="customer-realtime-status"[\s\S]*role="status"[\s\S]*aria-live="polite"/)
  assert.match(source, /realtimeStatusTone === "assistant-responding" && "border-primary\/20 bg-primary\/10 text-primary shadow-sm"/)
  assert.match(source, /animate-ping[\s\S]*motion-reduce:animate-none/)
  assert.match(source, /motion-safe:animate-bounce/)
  assert.match(source, /animationDelay: `\$\{index \* 160\}ms`/)
  assert.match(source, /realtimeStatusTone === "assistant-responding" \? \([\s\S]*data-testid="customer-assistant-responding-bubble"/)
  assert.match(source, /aria-label=\{t\("customerChat\.assistantResponding"\)\}/)
  assert.match(source, /min-h-10 max-w-\[78%\]/)
})

test("customer meeting join does not require a separate privacy checkbox", () => {
  assert.doesNotMatch(source, /privacyAccepted|privacyAttention|privacyConfirmationRef/)
  assert.doesNotMatch(source, /hasRememberedCustomerMeetingPrivacyConsent|rememberCustomerMeetingPrivacyConsent/)
  assert.doesNotMatch(source, /请先勾选隐私确认|加入前需勾选|勾选后即可加入会议/)
  assert.match(source, /const config = await fetchCustomerMeetingJoinConfig\(selectedId\)/)
  assert.match(source, /joinRequirementText=\{t\("customerMeeting\.joinRequirementText"\)\}/)
  assert.match(source, /order-first space-y-5 lg:order-none/)
  assert.match(source, /t\("customerMeeting\.privacyTitle"\)/)
  assert.match(source, /t\("customerMeeting\.privacyNote"\)/)
  assert.doesNotMatch(source, /joinDisabled=\{!privacyAccepted\}/)
})

test("customer meeting archives separate system closure notes from real transcripts", () => {
  assert.match(source, /function isSystemMeetingTranscript\(item: MeetingTranscriptSegment\)/)
  assert.match(source, /const realtimeTranscripts = transcripts\.filter\(\(item\) => !isSystemMeetingTranscript\(item\)\)/)
  assert.match(source, /const systemArchives = transcripts\.filter\(isSystemMeetingTranscript\)/)
  assert.match(source, /customerMeeting\.transcriptSummarySystemOnly/)
  assert.match(source, /customerMeeting\.noRealtimeTranscriptTitle/)
})

test("customer ticket page refreshes when the selected ticket URL changes", () => {
  assert.match(source, /const ticketQueryParam = searchParams\?\.get\("ticketId"\) \|\| ""/)
  assert.match(source, /Number\(ticketQueryParam \|\| "0"\)/)
  assert.match(source, /\}, \[loadVersion, ticketQueryParam\]\)/)
  assert.match(source, /ticketQueryParam !== String\(selectedId\)/)
  assert.match(source, /window\.addEventListener\("focus", refreshTickets\)/)
  assert.match(source, /window\.addEventListener\("pageshow", refreshTickets\)/)
  assert.match(source, /window\.addEventListener\("popstate", refreshTickets\)/)
  assert.match(source, /document\.addEventListener\("visibilitychange", refreshVisibleTickets\)/)
})

test("customer ticket page preserves a deep-linked ticket selection until it appears", () => {
  assert.match(source, /const \[ticketFilter, setTicketFilter\] = useState<"all" \| "active" \| "action" \| "closed">\("all"\)/)
  assert.match(source, /const missingTicketPollsRef = useRef\(0\)/)
  assert.match(source, /const queryId = Number\(ticketQueryParam \|\| "0"\)/)
  assert.match(source, /fetchCustomerTicketDetail\(selectedId\)/)
  assert.match(source, /t\("customerTickets\.syncingTitle"\)/)
  assert.doesNotMatch(source, /t\("customerTickets\.syncingDescription"\)/)
  assert.match(source, /missingTicketPollsRef\.current >= 5/)
  assert.match(source, /setLoadVersion\(\(value\) => value \+ 1\), 1200\)/)
  assert.match(source, /\['action', t\("customerTickets\.filterAction"\)\]/)
})

test("customer empty cards stay concise", () => {
  assert.doesNotMatch(source, /description\?: string/)
  assert.doesNotMatch(source, /description \? <div/)
  assert.doesNotMatch(source, /<EmptyCard[\s\S]{0,240}description=/)
  assert.doesNotMatch(source, /customer(Chat|Tickets|Meeting|Devices|Profile)\.(myConversationsDescription|emptyDescription|selectConversationDescription|syncingDescription|selectDescription|noUpcomingDescription|noHistoryDescription|selectDeviceDescription|manualsDescription|noManualsDescription|repairArchiveDescription|loadDescription)/)
})

test("customer conversation creation dialog keeps choices concise", async () => {
  const locales = ["zh-CN", "en-US", "es-ES"]
  const messagesByLocale = await Promise.all(
    locales.map(async (locale) => [
      locale,
      JSON.parse(await readFile(new URL(`../../messages/${locale}.json`, import.meta.url), "utf8")),
    ]),
  )

  assert.doesNotMatch(source, /DialogDescription/)
  assert.doesNotMatch(source, /customerChat\.createDialogDescription/)
  assert.doesNotMatch(source, /customerChat\.dialog(General|Bind)Body/)

  for (const [locale, messages] of messagesByLocale) {
    assert.equal(Object.hasOwn(messages.customerChat, "createDialogDescription"), false, `${locale} should not keep create dialog description`)
    assert.equal(Object.hasOwn(messages.customerChat, "dialogGeneralBody"), false, `${locale} should not keep general body copy`)
    assert.equal(Object.hasOwn(messages.customerChat, "dialogBindBody"), false, `${locale} should not keep bind body copy`)
  }
})

test("customer ticket action controls have explicit accessible labels", () => {
  assert.match(source, /aria-label=\{t\(hasDeviceConcept \? "customerTickets\.searchLabel" : "customerTickets\.searchLabelGeneral"\)\}/)
  assert.match(source, /aria-label=\{t\("customerTickets\.feedbackCommentLabel"\)\}/)
  assert.match(source, /aria-label=\{t\("customerTickets\.reopenReasonLabel"\)\}/)
})

test("customer ticket list keeps the status badge beside the ticket number", () => {
  assert.match(source, /data-testid="customer-ticket-list-item-heading"[\s\S]*\{item\.ticket_no\}[\s\S]*<StatusBadge status=\{item\.status\} \/>/)
  assert.match(source, /inline-flex shrink-0 whitespace-nowrap rounded-full/)
})

test("customer tickets show a stable initial loading skeleton before real counts", () => {
  assert.match(source, /const \[hasLoadedTickets, setHasLoadedTickets\] = useState\(false\)/)
  assert.match(source, /ticketInitialRetryRef/)
  assert.match(source, /window\.setTimeout\(\(\) => setLoadVersion\(\(value\) => value \+ 1\), 800\)/)
  assert.match(source, /const initialTicketLoad = !hasLoadedTickets && loading/)
  assert.match(source, /setHasLoadedTickets\(true\)/)
  assert.match(source, /initialTicketLoad \? <CustomerTicketWorkspaceLoading \/>/)
  assert.match(source, /function CustomerTicketWorkspaceLoading\(\)[\s\S]*role="status" aria-busy="true" aria-label=\{t\("customerTickets\.loadingWorkspace"\)\}/)
  assert.match(source, /const hasNoTickets = tickets\.length === 0/)
  assert.match(source, /const ticketListPending = loading && hasNoTickets/)
  assert.match(source, /ticketListPending \? Array\.from\(\{ length: 6 \}\)/)
  assert.match(source, /!hasLoadedTickets && !loading && loadError/)
  assert.match(source, /t\("customerTickets\.refreshFailed"\)/)
})

test("customer meeting detail keeps a loading surface before selection is resolved", () => {
  assert.match(source, /const hasSelectedMeeting = Boolean\(selectedMeeting\)/)
  assert.match(source, /const meetingDetailPending = loading && !hasSelectedMeeting/)
  assert.match(source, /meetingDetailPending \? <CustomerMeetingDetailLoading \/>/)
  assert.match(source, /function CustomerMeetingDetailLoading\(\)[\s\S]*role="status"[\s\S]*aria-label=\{t\("customerMeeting\.loadingDetail"\)\}/)
  for (const { locale, messages } of localeMessages) {
    assert.equal(typeof messages.customerMeeting.loadingDetail, "string", `${locale} should define customerMeeting.loadingDetail`)
    assert.ok(messages.customerMeeting.loadingDetail.length > 0, `${locale} should keep customerMeeting.loadingDetail non-empty`)
  }
})

test("customer meeting record keeps rendered content while transcripts refresh", () => {
  assert.match(source, /const hasRecordContent = realtimeTranscripts\.length > 0 \|\| systemArchives\.length > 0 \|\| Boolean\(error\)/)
  assert.match(source, /const transcriptInitialLoading = loading && !hasRecordContent/)
  assert.match(source, /const recordRefreshing = loading && hasRecordContent/)
  assert.match(source, /recordRefreshing \? \([\s\S]*role="status" aria-busy="true"/)
  assert.match(source, /transcriptInitialLoading \? \([\s\S]*aria-label=\{t\("common\.loading"\)\}/)
  assert.doesNotMatch(source, /\{loading \? \(\s*<div className="space-y-3 p-6 lg:p-8"/)
})

test("customer profile refresh keeps existing form content visible", () => {
  assert.match(source, /const \[profileLoaded, setProfileLoaded\] = useState\(false\)/)
  assert.match(source, /const \[loadVersion, setLoadVersion\] = useState\(0\)/)
  assert.match(source, /const \[loadError, setLoadError\] = useState\(""\)/)
  assert.match(source, /setProfileLoaded\(true\)/)
  assert.match(source, /const initialProfileLoad = !profileLoaded && loading/)
  assert.match(source, /const profileRefreshing = profileLoaded && loading/)
  assert.match(source, /<MetricsStrip home=\{home\} hasDeviceConcept=\{hasDeviceConcept\} \/>/)
  assert.match(source, /profile \? \([\s\S]*<Card className="rounded-md border-border shadow-sm">/)
  assert.match(source, /initialProfileLoad \? \([\s\S]*<Skeleton className="h-16 w-full" \/>/)
  assert.match(source, /aria-busy=\{profileRefreshing\}/)
  assert.match(source, /aria-label=\{t\("common\.loading"\)\}/)
  assert.doesNotMatch(source, /\{loading \? \(\s*<div className="grid gap-4 lg:grid-cols-2">/)
})

test("customer devices distinguish initial loading from a confirmed empty result", () => {
  assert.match(source, /const \[hasLoadedDevices, setHasLoadedDevices\] = useState\(false\)/)
  assert.match(source, /const initialDeviceLoad = !hasLoadedDevices && loading/)
  assert.match(source, /setHasLoadedDevices\(true\)/)
  assert.match(source, /initialDeviceLoad \? <><Skeleton[\s\S]*customerDevices\.loadingCount/)
  assert.match(source, /initialDeviceLoad \? <CustomerDeviceListLoading \/>/)
  assert.match(source, /initialDeviceLoad \? <CustomerDeviceDetailLoading \/>/)
  assert.match(source, /const \[manualsDeviceId, setManualsDeviceId\] = useState\(0\)/)
  assert.match(source, /const hasSelectedManuals = manualsDeviceId === selectedId/)
  assert.match(source, /const selectedManuals = hasSelectedManuals \? manuals : \[\]/)
  assert.match(source, /const manualsInitialLoading = manualsLoading && !hasSelectedManuals/)
  assert.match(source, /setManualsDeviceId\(selectedId\)/)
  assert.match(source, /manualsInitialLoading \? <div className="grid gap-4 md:grid-cols-2">/)
  assert.match(source, /selectedManuals\.map\(\(manual\) => <a/)
  assert.doesNotMatch(source, /manualsLoading \? <div className="grid gap-4 md:grid-cols-2">/)
  assert.match(source, /hasLoadedDevices && devices\.length === 0 \? <EmptyCard/)
  assert.doesNotMatch(source, /<p className="mt-1 text-sm text-slate-500">\{devices\.length\} 台已绑定设备<\/p>/)
})

test("customer device failures do not become a false empty state", () => {
  assert.match(source, /const \[loadError, setLoadError\] = useState\(""\)/)
  assert.match(source, /setLoadError\(message\)/)
  assert.match(source, /!hasLoadedDevices && loadError && !loading \? \(/)
  assert.match(source, /title=\{t\("customerDevices\.loadFailed"\)\}/)
  assert.match(source, /loadError && hasLoadedDevices \? \(/)
  assert.match(source, /t\("customerDevices\.refreshFailed"\)/)
})

test("customer device loading skeletons expose stable accessible status regions", () => {
  assert.match(source, /function CustomerDeviceListLoading\(\)[\s\S]*role="status" aria-label=\{t\("customerDevices\.loadingList"\)\}/)
  assert.match(source, /function CustomerDeviceDetailLoading\(\)[\s\S]*role="status" aria-label=\{t\("customerDevices\.loadingDetail"\)\}/)
  assert.match(source, /<section[\s\S]*aria-busy=\{loading\}/)
})

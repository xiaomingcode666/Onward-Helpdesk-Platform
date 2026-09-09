import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./mobile-customer-portal.tsx", import.meta.url), "utf8")
const pagesSource = await readFile(new URL("./mobile-customer-pages.tsx", import.meta.url), "utf8")
const conversationWorkbenchSource = await readFile(new URL("./mobile-customer-conversation-workbench.tsx", import.meta.url), "utf8")
const loginSource = await readFile(new URL("./mobile-customer-login.tsx", import.meta.url), "utf8")
const clientSource = await readFile(new URL("../../app/c/[serviceCode]/mobile-service-client.tsx", import.meta.url), "utf8")
const customerPortalSource = await readFile(new URL("../customer-portal/customer-portal-pages.tsx", import.meta.url), "utf8")
const buildSource = await readFile(new URL("../../scripts/build-mobile.mjs", import.meta.url), "utf8")
const pushApiSource = await readFile(new URL("../../lib/api/mobile-push.ts", import.meta.url), "utf8")
const pushRegistrationSource = await readFile(new URL("../../lib/mobile/push-registration.ts", import.meta.url), "utf8")
const extractedSource = await readFile(new URL("../../i18n/extracted/ops-components.ts", import.meta.url), "utf8")
const themeSource = await readFile(new URL("../../styles/customer-portal-themes.css", import.meta.url), "utf8")
const ticketProgressSource = await readFile(new URL("../../lib/customer-ticket-progress-i18n.ts", import.meta.url), "utf8")

test("mobile customer portal renders five dedicated mobile pages", () => {
  for (const page of [
    "MobileCustomerChatPage",
    "MobileCustomerTicketsPage",
    "MobileCustomerMeetingsPage",
    "MobileCustomerDevicesPage",
    "MobileCustomerProfilePage",
  ]) {
    assert.match(source, new RegExp(`<${page}`))
  }
  assert.doesNotMatch(source, /customer-portal-pages/)
  for (const tab of ["chat", "tickets", "video", "devices", "my"]) {
    assert.match(source, new RegExp(`label: ml\\(t, "tab\\.${tab}\\.label"\\)`))
    assert.match(source, new RegExp(`title: ml\\(t, "tab\\.${tab}\\.title"\\)`))
  }
  assert.match(source, /label: ml\(t, "tab\.chat\.label"\), title: ml\(t, "tab\.chat\.title"\), icon: MessageCircleIcon/)
  assert.doesNotMatch(source, /[㐀-鿿]/)
  assert.match(source, /<MobileCustomerChatPage navigate=\{navigate\}/)
})

test("mobile customer portal scopes the tenant theme and removes device UI for knowledge tenants", () => {
  assert.match(source, /normalizeCustomerTheme\(session\?\.tenantBranding\?\.customerTheme\)/)
  assert.match(source, /getCustomerThemeClassName\(customerTheme\)/)
  assert.match(source, /data-customer-theme=\{customerTheme\}/)
  assert.match(source, /hasDeviceConcept \? \[\{ value: "devices" as const/)
  assert.match(source, /requestedTab === "devices" && !hasDeviceConcept \? "chat"/)
  assert.match(conversationWorkbenchSource, /if \(!hasDeviceConcept\) \{[\s\S]*createSystemHelpConversation\(\)/)
  assert.match(conversationWorkbenchSource, /hasDeviceConcept \? "portalExtract\.customerMobile\.conversation\.search" : "portalExtract\.customerMobile\.conversation\.searchGeneral"/)
  assert.match(pagesSource, /hasDeviceConcept \? \[\[t\("portalExtract\.customerMobile\.common\.boundDevices"\)/)
  assert.match(themeSource, /\.rhd-customer-theme-odt/)
  assert.match(themeSource, /\.rhd-customer-theme-clinical/)
  assert.match(themeSource, /\.rhd-customer-theme-coral/)
  assert.match(themeSource, /onward-ai-scheduled-intelligence-v1\.jpg/)
})

test("mobile customer portal bottom tab copy is translated in every locale", () => {
  const tabCopy = [
    ["chat", "会话", "Chat", "Chat"],
    ["tickets", "工单", "Tickets", "Tickets"],
    ["video", "视频", "Video", "Video"],
    ["devices", "设备", "Devices", "Dispositivos"],
    ["my", "我的", "Me", "Mi"],
  ]
  const titleCopy = [
    ["chat", "会话", "Chat", "Chat"],
    ["tickets", "服务工单", "Service tickets", "Tickets de servicio"],
    ["video", "远程协作", "Remote collaboration", "Colaboracion remota"],
    ["devices", "我的设备", "My devices", "Mis dispositivos"],
    ["my", "账号", "Account", "Cuenta"],
  ]
  for (const [tab, zh] of tabCopy) {
    assert.match(extractedSource, new RegExp(`${tab}: \\{[\\s\\S]*?label: "${zh}"`))
  }
  for (const [tab, zh] of titleCopy) {
    assert.match(extractedSource, new RegExp(`${tab}: \\{[\\s\\S]*?title: "${zh}"`))
  }
  for (const [tab, , en, es] of tabCopy) {
    assert.match(extractedSource, new RegExp(`${tab}: \\{[\\s\\S]*?label: "${en}"`))
    assert.match(extractedSource, new RegExp(`${tab}: \\{[\\s\\S]*?label: "${es}"`))
  }
  for (const [tab, , en, es] of titleCopy) {
    assert.match(extractedSource, new RegExp(`${tab}: \\{[\\s\\S]*?title: "${en}"`))
    assert.match(extractedSource, new RegExp(`${tab}: \\{[\\s\\S]*?title: "${es}"`))
  }
})

test("dedicated mobile pages keep the formal customer capabilities", () => {
  const mobileSources = `${pagesSource}\n${conversationWorkbenchSource}`
  for (const api of [
    "fetchCustomerConversations",
    "fetchImMessages",
    "sendImMessage",
    "requestImHumanSupport",
    "fetchCustomerTickets",
    "confirmCustomerTicket",
    "reopenCustomerTicket",
    "submitCustomerTicketFeedback",
    "fetchCustomerMeetings",
    "fetchCustomerMeetingJoinConfig",
    "fetchCustomerDevices",
    "bindCustomerDevice",
    "fetchCustomerProfile",
    "updateCustomerProfile",
    "fetchCustomerAccountDeletionStatus",
    "deleteCustomerAccount",
  ]) {
    assert.match(mobileSources, new RegExp(api))
  }
  assert.match(pagesSource, /<MeetingLiveRoom/)
})

test("mobile profile provides a confirmed account deletion flow", () => {
  assert.match(pagesSource, /永久删除账号/)
  assert.match(pagesSource, /current_password: currentPassword/)
  assert.match(pagesSource, /confirmation: "DELETE"/)
  assert.match(pagesSource, /deletionAcknowledged/)
  assert.match(pagesSource, /signOut\("\/mobile\?state=login&accountDeleted=1"\)/)
  assert.match(pagesSource, /href="\/legal\/privacy"/)
})

test("conversation workbench makes every historical session selectable", () => {
  assert.match(conversationWorkbenchSource, /h-full overflow-y-auto overscroll-contain/)
  assert.match(conversationWorkbenchSource, /const conversationPageSize = 30/)
  assert.match(conversationWorkbenchSource, /visibleConversations = filteredConversations\.slice\(0, visibleCount\)/)
  assert.match(conversationWorkbenchSource, /visibleConversations\.map/)
  assert.match(conversationWorkbenchSource, /加载更多/)
  assert.match(conversationWorkbenchSource, /已显示 \{visibleConversations\.length\}\/\{filteredConversations\.length\}/)
  assert.match(conversationWorkbenchSource, /搜索设备、产品、消息或工单/)
  assert.match(conversationWorkbenchSource, /\['all', '全部'\], \['active', '进行中'\], \['closed', '已结束'\]/)
  assert.match(conversationWorkbenchSource, /openConversation\(item\.id\)/)
  assert.match(conversationWorkbenchSource, /conversationId: String\(conversationId\)/)
  assert.match(conversationWorkbenchSource, /返回会话列表/)
})

test("device consultation starts its device conversation directly", () => {
  assert.match(pagesSource, /navigate\("chat", \{ deviceId: String\(selected\.id\) \}\)/)
  assert.match(conversationWorkbenchSource, /createOrMatchImConversation\(\{ deviceId: requestedDeviceId, forceNew: true \}\)/)
  assert.match(conversationWorkbenchSource, /正在发起设备咨询/)
  assert.match(conversationWorkbenchSource, /activateCreatedConversation\(created, requestedDevice\)/)
  assert.match(conversationWorkbenchSource, /navigate\("chat", \{ conversationId: String\(created\.id\) \}\)/)
})

test("conversation-linked ticket and device details return to their source conversation", () => {
  assert.match(source, /<MobileCustomerTicketsPage navigate=\{navigate\}/)
  assert.equal((conversationWorkbenchSource.match(/fromConversationId: String\(selected\.id\)/g) ?? []).length, 3)
  assert.equal((pagesSource.match(/normalizeMobileConversationId\(searchParams\.get\("fromConversationId"\)\)/g) ?? []).length, 2)
  assert.equal((pagesSource.match(/navigate\("chat", \{ conversationId: String\(returnConversationId\) \}\)/g) ?? []).length, 2)
})

test("device conversations expose an explicit human handoff action", () => {
  assert.match(conversationWorkbenchSource, /requestImHumanSupport\(selected\.id, "客户通过移动端请求人工介入"\)/)
  assert.match(conversationWorkbenchSource, /已提交转人工，正在分配工程师/)
  assert.match(conversationWorkbenchSource, /"转人工"/)
})

test("mobile conversations send and render field photos and attachments", () => {
  assert.match(conversationWorkbenchSource, /uploadImImage/)
  assert.match(conversationWorkbenchSource, /uploadImAttachment/)
  assert.match(conversationWorkbenchSource, /messageType: \"image\" \| \"attachment\"/)
  assert.match(conversationWorkbenchSource, /capture=\"environment\"/)
  assert.match(conversationWorkbenchSource, /拍摄现场照片/)
  assert.match(conversationWorkbenchSource, /从相册选择图片/)
  assert.match(conversationWorkbenchSource, /发送附件/)
  assert.match(conversationWorkbenchSource, /buildMediaTransferPayload/)
  assert.match(conversationWorkbenchSource, /markPendingMobileMediaFailed/)
  assert.match(conversationWorkbenchSource, /renderIMMessageHTML\(message\)/)
  assert.match(conversationWorkbenchSource, /<ImMessageHTML/)
  assert.match(conversationWorkbenchSource, /validateMobileMediaSelection/)
  assert.match(conversationWorkbenchSource, /文件不能超过/)
  assert.match(conversationWorkbenchSource, /selectedMedia\.previewUrl/)
  assert.match(conversationWorkbenchSource, /取消待发送文件/)
  assert.match(conversationWorkbenchSource, /retryMediaMessage/)
  assert.match(conversationWorkbenchSource, /uploadedAsset: transfer\.uploadedAsset/)
  assert.match(conversationWorkbenchSource, /upsertImMessageReplacingClientMessage/)
  assert.match(conversationWorkbenchSource, /replaceLoadedImMessagesPreservingLocalTransfers/)
})

test("conversation workbench refreshes safely and pages older messages", () => {
  assert.match(conversationWorkbenchSource, /messageRequestRef/)
  assert.match(conversationWorkbenchSource, /selectedIdRef\.current !== conversationId/)
  assert.match(conversationWorkbenchSource, /fetchCustomerDevices\(\)[\s\S]*\.catch/)
  assert.match(conversationWorkbenchSource, /function createSystemHelpConversation\(\)/)
  assert.match(conversationWorkbenchSource, /还没有绑定设备时，可以先查看系统使用帮助/)
  assert.match(conversationWorkbenchSource, /设备列表暂不可用，当前显示已缓存设备/)
  assert.match(conversationWorkbenchSource, /fetchImMessages\(\{ conversationId: selectedId, cursor: messageCursor, limit: 50 \}\)/)
  assert.match(conversationWorkbenchSource, /加载更早消息/)
  assert.match(conversationWorkbenchSource, /window\.setInterval/)
  assert.match(conversationWorkbenchSource, /document\.visibilityState === "visible"/)
  assert.match(conversationWorkbenchSource, /replaceLoadedImMessagesPreservingLocalTransfers\(current, nextMessages\)/)
  assert.match(conversationWorkbenchSource, /markImMessageRead\(conversationId\)\.catch/)
})

test("new mobile conversations invalidate stale selection and message requests synchronously", () => {
  assert.match(conversationWorkbenchSource, /selectedIdRef\.current = created\.id[\s\S]*?messageRequestRef\.current \+= 1/)
  assert.match(conversationWorkbenchSource, /function openConversation\(conversationId: number\) \{[\s\S]*?selectedIdRef\.current = conversationId[\s\S]*?messageRequestRef\.current \+= 1/)
})

test("knowledge-support ticket progress accepts generic technical support lifecycle records", () => {
  assert.match(ticketProgressSource, /content === "技术支持组成员接管并受理"/)
  assert.match(ticketProgressSource, /content\.startsWith\("工单回到技术支持组待派单"\)/)
})

test("mobile conversations subscribe to realtime events and poll only as fallback", () => {
  assert.match(conversationWorkbenchSource, /createRealtimeConnectionManager/)
  assert.match(conversationWorkbenchSource, /createAccessTokenWebSocket/)
  assert.match(conversationWorkbenchSource, /topics: \[`conversation:\$\{conversationId\}`\]/)
  assert.match(conversationWorkbenchSource, /envelope\.type === "message\.created"/)
  assert.match(conversationWorkbenchSource, /normalizeRealtimeMessage<ImMessage>\(payload\)/)
  assert.match(conversationWorkbenchSource, /envelope\.type === "resyncRequired"/)
  assert.match(conversationWorkbenchSource, /envelope\.type\?\.startsWith\("conversation\."\)/)
  assert.match(conversationWorkbenchSource, /realtimeStatusRef\.current !== "connected"/)
  assert.match(conversationWorkbenchSource, /实时连接正常/)
  assert.match(conversationWorkbenchSource, /输入内容已保留，请重试/)
})

test("long mobile lists render progressively and keep their toolbars available", () => {
  assert.match(pagesSource, /const mobileListPageSize = 30/)
  assert.match(pagesSource, /const visibleTickets = filtered\.slice\(0, visibleCount\)/)
  assert.match(pagesSource, /const visibleMeetings = filtered\.slice\(0, visibleCount\)/)
  assert.match(pagesSource, /visibleTickets\.map/)
  assert.match(pagesSource, /visibleMeetings\.map/)
  assert.match(pagesSource, /sticky top-0 z-20/)
  assert.match(pagesSource, /已显示 \{visibleTickets\.length\}\/\{filtered\.length\}/)
  assert.match(pagesSource, /已显示 \{visibleMeetings\.length\}\/\{filtered\.length\}/)
})

test("mobile customer pages use tab-level skeleton loading instead of full-page loading text", () => {
  const pageLoadingNamePattern = new RegExp("Page" + "Loading")
  assert.doesNotMatch(pagesSource, new RegExp("function " + "Page" + "Loading"))
  assert.doesNotMatch(pagesSource, new RegExp("<" + "Page" + "Loading"))
  assert.doesNotMatch(pagesSource, new RegExp("if \\(state\\.loading\\) return <" + "Page" + "Loading"))
  assert.match(pagesSource, /function MobileListLoading\(/)
  assert.match(pagesSource, /function MobileProfileLoading\(\)/)
  assert.match(conversationWorkbenchSource, /function MetricLoadingCell/)
  assert.match(conversationWorkbenchSource, /function ConversationListLoading\(\)/)
  assert.match(conversationWorkbenchSource, /function MessagesLoading\(\)/)
  assert.doesNotMatch(pagesSource, /if \(state\.loading\) return/)
  assert.match(conversationWorkbenchSource, /const \[hasLoadedConversations, setHasLoadedConversations\] = useState\(false\)/)
  assert.match(conversationWorkbenchSource, /const \[loadedMessageConversationId, setLoadedMessageConversationId\] = useState\(0\)/)
  assert.match(conversationWorkbenchSource, /const hasLoadedMessages = selectedId > 0 && loadedMessageConversationId === selectedId/)
  assert.match(conversationWorkbenchSource, /const conversationInitialLoading = loading && !hasLoadedConversations/)
  assert.match(pagesSource, /const \[hasLoadedTickets, setHasLoadedTickets\] = useState\(false\)/)
  assert.match(pagesSource, /const \[hasLoadedMeetings, setHasLoadedMeetings\] = useState\(false\)/)
  assert.match(pagesSource, /const \[hasLoadedDevices, setHasLoadedDevices\] = useState\(false\)/)
  assert.match(pagesSource, /const ticketInitialLoading = state\.loading && !hasLoadedTickets/)
  assert.match(pagesSource, /const meetingInitialLoading = state\.loading && !hasLoadedMeetings/)
  assert.match(pagesSource, /const deviceInitialLoading = state\.loading && !hasLoadedDevices/)
  assert.match(conversationWorkbenchSource, /const messageInitialLoading = messageLoading && !hasLoadedMessages/)
  assert.doesNotMatch(conversationWorkbenchSource, /const conversationInitialLoading = loading && conversations\.length === 0/)
  assert.doesNotMatch(conversationWorkbenchSource, /const messageInitialLoading = messageLoading && messages\.length === 0/)
  assert.doesNotMatch(conversationWorkbenchSource, /if \(conversationInitialLoading\) return/)
  assert.match(conversationWorkbenchSource, /conversationInitialLoading \? <ConversationListLoading \/>/)
  assert.match(conversationWorkbenchSource, /disabled=\{conversationInitialLoading\}/)
  assert.match(pagesSource, /ticketInitialLoading \? <MobileListLoading label="正在读取工单"/)
  assert.match(pagesSource, /meetingInitialLoading \? <MobileListLoading label="正在读取视频协作"/)
  assert.match(pagesSource, /deviceInitialLoading \? <MobileListLoading label="正在读取设备"/)
  assert.doesNotMatch(pagesSource, /if \(!profile\) return <MobileProfileLoading \/>/)
  assert.match(pagesSource, /!profile \? \(\s*<MobileProfileLoading \/>/)
  assert.match(conversationWorkbenchSource, /messageInitialLoading \? <MessagesLoading \/>/)
  assert.doesNotMatch(pagesSource, /state\.loading \? <MobileListLoading/)
  assert.doesNotMatch(conversationWorkbenchSource, /messageLoading \? <MessagesLoading \/>/)
  assert.doesNotMatch(pagesSource, pageLoadingNamePattern)
})

test("mobile customer empty states and video copy stay concise", () => {
  assert.doesNotMatch(pagesSource, /description\?: string/)
  assert.doesNotMatch(pagesSource, /description \? <p/)
  assert.doesNotMatch(pagesSource, /<EmptyState[\s\S]{0,220}description=/)
  assert.doesNotMatch(pagesSource, /DialogDescription/)
  assert.doesNotMatch(pagesSource, /输入设备铭牌或交付资料中的服务码/)
  assert.doesNotMatch(pagesSource, /智能服务中/)
  assert.doesNotMatch(pagesSource, /暂无消息，描述设备现象即可开始诊断/)
  assert.doesNotMatch(pagesSource, /选择设备后建立专属售后会话|调整筛选条件或搜索关键词后再试|工程师发起视频协作后会显示在这里|已结束的远程协作会保留在这里/)
  assert.doesNotMatch(pagesSource, /正在读取会议|会议暂时无法加入|退出会议页面|会议已归档|暂无待参加会议|暂无历史会议|会议时长/)
  assert.doesNotMatch(conversationWorkbenchSource, /该会话不存在或当前账号无权访问/)
  assert.match(conversationWorkbenchSource, /无权访问该会话/)
  assert.match(pagesSource, /正在读取视频协作/)
  assert.match(pagesSource, /暂无待参加协作/)
  assert.match(pagesSource, /协作时长/)
})

test("mobile login authenticates only against the customer portal and stays in the app", () => {
  assert.match(loginSource, /domainType: "customer"/)
  assert.match(loginSource, /await refreshProfile\(\)/)
  assert.doesNotMatch(loginSource, /dashboard\/login/)
  assert.match(clientSource, /navigateTo\("login"\)/)
  assert.match(clientSource, /<MobileCustomerPortal serviceCode=\{serviceCode\}/)
})

test("mobile registration supports invitation authorization and device binding", () => {
  assert.match(loginSource, /verifyCustomerRegistration/)
  assert.match(loginSource, /registerCustomerAccount/)
  assert.match(loginSource, /changeMethod\("invite"\)/)
  assert.match(loginSource, /changeMethod\("service_code"\)/)
  assert.match(loginSource, /ml\(t, "inviteMethod"\)/)
  assert.match(loginSource, /ml\(t, "bindDeviceMethod"\)/)
  assert.doesNotMatch(loginSource, /使用已有账号继续/)
  assert.doesNotMatch(loginSource, /通过企业授权或设备服务码注册/)
  assert.doesNotMatch(loginSource, /使用企业邀请授权码/)
  assert.doesNotMatch(loginSource, /使用设备服务码注册/)
  assert.match(loginSource, /initialServiceCode/)
  assert.match(clientSource, /initialServiceCode=\{serviceCode\}/)
})

test("formal customer page navigation is adapted back into the mobile shell", () => {
  for (const path of ["chat", "tickets", "meeting", "devices", "my"]) {
    assert.match(customerPortalSource, new RegExp(`"/customer/${path}"`))
  }
  assert.match(customerPortalSource, /return `\/mobile\?\$\{params\.toString\(\)\}`/)
  assert.match(customerPortalSource, /useCustomerPortalRouter/)
  assert.match(customerPortalSource, /useMemo\(\(\) => \(\{/)
})

test("mobile tab changes reset the page scroll position", () => {
  assert.match(source, /window\.scrollTo\(\{ top: 0, left: 0 \}\)/)
  assert.match(source, /\[activeTab\]/)
})

test("customer mobile sessions register and revoke their own native push token", () => {
  assert.match(source, /registerNativeMobilePushNotifications\(pushRegistrationKey\)/)
  assert.match(source, /unregisterNativeMobilePushNotifications\(pushRegistrationKey\)/)
  assert.match(source, /await signOut\("\/mobile\?state=login"\)/)
  assert.match(pushApiSource, /readSession\(\)\?\.domainType === "customer"/)
  assert.match(pushApiSource, /customerPortalRequest<MobilePushToken>\("\/notifications\/push-tokens"/)
  assert.match(pushApiSource, /customerPortalRequest<void>\(`\/notifications\/push-tokens\/\$\{id\}\/\_revoke`/)
  assert.match(pushRegistrationSource, /remotehelpdesk-mobile-push-token-id:/)
  assert.match(pushRegistrationSource, /ANDROID_CONVERSATION_CHANNEL_ID = "conversation_updates"/)
  assert.match(pushRegistrationSource, /PushNotifications\.createChannel\(/)
  assert.match(pushRegistrationSource, /if \(!isNativePushConfigured\(platform\)\) return/)
  assert.doesNotMatch(pushRegistrationSource, /AI 与工程师回复/)
  assert.match(pushRegistrationSource, /PushNotifications\.unregister\(\)/)
})

test("mobile release build disables local login defaults and passwords", () => {
  assert.match(buildSource, /NEXT_PUBLIC_ENABLE_LOCAL_LOGIN_DEFAULTS: "false"/)
  assert.match(buildSource, /NEXT_PUBLIC_ANDROID_PUSH_CONFIGURED: androidPushConfigured \? "true" : "false"/)
  for (const portal of ["PLATFORM", "ENTERPRISE", "PARTNER", "CUSTOMER"]) {
    assert.match(buildSource, new RegExp(`NEXT_PUBLIC_LOCAL_LOGIN_${portal}_USERNAME: ""`))
    assert.match(buildSource, new RegExp(`NEXT_PUBLIC_LOCAL_LOGIN_${portal}_PASSWORD: ""`))
  }
})

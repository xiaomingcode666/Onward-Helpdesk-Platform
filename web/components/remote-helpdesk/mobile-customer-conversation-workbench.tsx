"use client"

import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent, type FormEvent, type ReactNode } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import {
  ArrowLeftIcon,
  BookOpenIcon,
  CameraIcon,
  CheckCircle2Icon,
  ChevronRightIcon,
  CircleAlertIcon,
  ClipboardListIcon,
  Clock3Icon,
  HeadphonesIcon,
  ImageIcon,
  Loader2Icon,
  LanguagesIcon,
  MessageCircleIcon,
  MessageSquarePlusIcon,
  MessagesSquareIcon,
  PlusIcon,
  PaperclipIcon,
  RadioIcon,
  RefreshCwIcon,
  ShieldCheckIcon,
  StarIcon,
  VideoIcon,
  WifiIcon,
  WifiOffIcon,
  WrenchIcon,
  XIcon,
} from "lucide-react"

import { useAuth } from "@/components/auth-provider"
import { VoiceRecorderButton } from "@/components/chat/voice-recorder-button"
import { ImMessageHTML } from "@/components/im-message-html"
import { FilterTabs, SearchField, SelectField, type RailopsTabItem } from "@railops/ui"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Textarea } from "@/components/ui/textarea"
import { MobileLanguageSwitch } from "@/components/remote-helpdesk/mobile-language-switch"
import { KnowledgeMessageCitations } from "@/components/remote-helpdesk/knowledge-message-citations"
import {
  ConversationServiceEvent,
  parseConversationServiceEvent,
  type ConversationServiceEventData,
} from "@/components/remote-helpdesk/conversation-service-event"
import { fetchCustomerConversationsPage, translateCustomerConversationMessage } from "@/lib/api/customer-conversations"
import { fetchCustomerDevicesPage } from "@/lib/api/customer-devices"
import { fetchCustomerProfile } from "@/lib/api/customer-profile"
import {
  confirmCustomerTicket,
  fetchCustomerTicketDetail,
  submitCustomerTicketFeedback,
} from "@/lib/api/customer-tickets"
import type { CustomerPortalConversation, CustomerPortalDevice, CustomerPortalProfile, CustomerPortalTicket } from "@/lib/api/customer-portal-types"
import type { ConversationMessageTranslationDTO } from "@/lib/api/types"
import {
  createOrMatchImConversation,
  fetchImMessages,
  markImMessageRead,
  requestImHumanSupport,
  sendImMessage,
  uploadImAudio,
  uploadImAttachment,
  uploadImImage,
  type ImAsset,
  type ImConversation,
  type ImMessage,
} from "@/lib/api/im"
import { createAccessTokenWebSocket, createWebSocketBaseUrl } from "@/lib/api/websocket"
import {
  normalizeRealtimeMessage,
  type RealtimeMessageCreatedPayload,
} from "@/lib/im-realtime-state"
import {
  createRealtimeConnectionManager,
  type RealtimeConnectionStatus,
} from "@/lib/realtime-connection"
import { buildMediaTransferPayload, createPendingMessageId, renderIMMessageHTML } from "@/lib/im-message"
import {
  mergeImMessagesReplacingClientMessages,
  replaceLoadedImMessagesPreservingLocalTransfers,
  upsertImMessageReplacingClientMessage,
} from "@/lib/im-message-merge"
import {
  MOBILE_MEDIA_MAX_BYTES,
  formatMobileFileSize,
  validateMobileMediaSelection,
  type MobileMediaKind,
} from "@/lib/mobile/media-selection"
import { cn } from "@/lib/utils"
import type { MobileCustomerNavigate } from "@/components/remote-helpdesk/mobile-customer-pages"
import { renderCustomerConversationSummary } from "@/lib/customer-conversation-summary-i18n"
import { useAppLocale, useI18n } from "@/i18n/provider"

type ConversationFilter = "all" | "active" | "closed"

type FailedMobileMediaTransfer = {
  conversationId: number
  file: File
  messageType: MobileMediaKind
  durationSeconds?: number
  uploadedAsset?: ImAsset
}

type SupplierMessageContext = {
  partnerCompanyName: string
  productModuleName: string
}

const closedConversationStatuses = new Set(["cancelled", "closed", "ended", "finished"])
const conversationPageSize = 30

function isConversationClosed(item: CustomerPortalConversation) {
  return closedConversationStatuses.has(item.status)
}

function formatDate(value: string | undefined, locale: string) {
  if (!value) return "--"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(locale, {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date)
}

function parseMobileMessagePayload(value?: string): Record<string, unknown> {
  if (!value?.trim()) return {}
  try {
    const parsed = JSON.parse(value)
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? parsed as Record<string, unknown>
      : {}
  } catch {
    return {}
  }
}

function mobilePayloadText(payload: Record<string, unknown>, key: string) {
  const value = payload[key]
  if (typeof value === "string") return value.trim()
  if (typeof value === "number" && Number.isFinite(value)) return String(value)
  return ""
}

function partnerMessageCollaborationId(message: ImMessage) {
  const payload = parseMobileMessagePayload(message.payload)
  return mobilePayloadText(payload, "collaborationId") || mobilePayloadText(payload, "collaboration_id")
}

function supplierContextFromMessagePayload(message: ImMessage): SupplierMessageContext | undefined {
  const payload = parseMobileMessagePayload(message.payload)
  const partnerCompanyName = mobilePayloadText(payload, "partnerCompanyName") || mobilePayloadText(payload, "partner_company_name")
  const productModuleName = mobilePayloadText(payload, "productModuleName") || mobilePayloadText(payload, "product_module_name")
  if (!partnerCompanyName && !productModuleName) return undefined
  return { partnerCompanyName, productModuleName }
}

function supplierContextBadge(
  context: SupplierMessageContext | undefined,
  locale: string,
  t: I18nT,
) {
  if (!context) return ""
  const supplier = t("customerEntryExtract.event.supplier")
  const moduleName = context.productModuleName.trim()
  if (moduleName) {
    return locale.toLowerCase().startsWith("zh")
      ? `${moduleName}${supplier}`
      : `${supplier} · ${moduleName}`
  }
  const companyName = context.partnerCompanyName.trim()
  if (companyName) {
    return locale.toLowerCase().startsWith("zh")
      ? `${companyName}${supplier}`
      : `${supplier} · ${companyName}`
  }
  return supplier
}

function messageSenderName(message: ImMessage, t: I18nT) {
  if (message.senderType === "customer" || message.senderType === "user") {
    return t("portalExtract.customerMobile.conversation.me")
  }
  if (message.senderType === "partner") {
    return message.senderName || t("customerEntryExtract.event.supplier")
  }
  return message.senderName || t("portalExtract.customerMobile.conversation.onlineSupport")
}

function localizedMobileMessageContent(content: string, t: I18nT) {
  const supplierResolved = content.trim().match(/^供应商处理完成[:：]\s*(.+)$/)
  if (supplierResolved?.[1]) {
    return t("customerEntryExtract.event.supplierResolvedDescriptionWithResolution", {
      resolution: supplierResolved[1].trim(),
    })
  }
  return content
}

function renderLocalizedMobileMessageHTML(message: ImMessage, t: I18nT) {
  const content = message.messageType === "text"
    ? localizedMobileMessageContent(message.content, t)
    : message.content
  return renderIMMessageHTML(content === message.content ? message : { ...message, content })
}

type I18nT = ReturnType<typeof useI18n>

function statusLabel(status: string, t: I18nT) {
  const labels: Record<string, string> = {
    active: t("portalExtract.customerMobile.status.processing"),
    ai_serving: t("portalExtract.customerMobile.status.aiServing"),
    cancelled: t("portalExtract.customerMobile.status.cancelled"),
    closed: t("portalExtract.customerMobile.status.ended"),
    ended: t("portalExtract.customerMobile.status.ended"),
    finished: t("portalExtract.customerMobile.status.ended"),
    human_serving: t("portalExtract.customerMobile.status.humanServing"),
    queued: t("portalExtract.customerMobile.status.queued"),
    waiting_customer: t("portalExtract.customerMobile.status.waitingCustomer"),
  }
  return labels[status] || status || t("portalExtract.customerMobile.common.statusUnknown")
}

function ConversationStatus({ status }: { status: string }) {
  const t = useI18n()
  const closed = closedConversationStatuses.has(status)
  const waiting = status === "queued" || status === "waiting_customer"
  return (
    <span className={cn(
      "inline-flex shrink-0 items-center gap-1.5 text-rhd-xs font-medium",
      closed ? "text-[#737d8e]" : waiting ? "text-[#b55b08]" : "text-[#087f5b]",
    )}>
      <span className={cn("size-1.5 rounded-full", closed ? "bg-[#9aa3b2]" : waiting ? "bg-[#e28a28]" : "bg-[#10a779]")} />
      {statusLabel(status, t)}
    </span>
  )
}

function buildPendingMobileMediaMessage(input: {
  conversationId: number
  clientMsgId: string
  messageType: MobileMediaKind
  file: File
  durationSeconds?: number
}): ImMessage {
  return {
    id: createPendingMessageId(),
    conversationId: input.conversationId,
    clientMsgId: input.clientMsgId,
    senderType: "customer",
    senderId: 0,
    messageType: input.messageType,
    content: input.file.name,
    payload: buildMediaTransferPayload({
      filename: input.file.name,
      fileSize: input.file.size,
      mimeType: input.file.type,
      durationSeconds: input.durationSeconds,
    }),
    sendStatus: 1,
    sentAt: new Date().toISOString(),
    customerRead: true,
    agentRead: false,
  }
}

function markPendingMobileMediaFailed(message: ImMessage, file: File, durationSeconds?: number) {
  return {
    ...message,
    payload: buildMediaTransferPayload({
      filename: file.name,
      fileSize: file.size,
      mimeType: file.type,
      durationSeconds,
    }, "failed"),
    sendStatus: 5,
  }
}

function mobileMediaSelectionErrorMessage(error: ReturnType<typeof validateMobileMediaSelection>, t: I18nT) {
  if (error === "empty") return t("portalExtract.customerMobile.conversation.emptyFile")
  if (error === "too_large") return t("portalExtract.customerMobile.conversation.fileTooLarge", { size: formatMobileFileSize(MOBILE_MEDIA_MAX_BYTES) })
  if (error === "not_image") return t("portalExtract.customerMobile.conversation.selectImageFile")
  return ""
}

function mobileMediaSendingMessage(messageType: MobileMediaKind, t: I18nT) {
  if (messageType === "image") return t("portalExtract.customerMobile.conversation.sendingImage")
  if (messageType === "audio") return t("conversation.voiceSending")
  return t("portalExtract.customerMobile.conversation.sendingAttachment")
}

function mobileMediaSentMessage(messageType: MobileMediaKind, t: I18nT) {
  if (messageType === "image") return t("portalExtract.customerMobile.conversation.imageSent")
  if (messageType === "audio") return t("supportChat.audioSummary")
  return t("portalExtract.customerMobile.conversation.attachmentSent")
}

function mobileMediaFailedMessage(messageType: MobileMediaKind, t: I18nT) {
  if (messageType === "image") return t("portalExtract.customerMobile.conversation.imageSendFailed")
  if (messageType === "audio") return t("conversation.sendVoiceFailed")
  return t("portalExtract.customerMobile.conversation.attachmentSendFailed")
}

function mapCreatedConversationStatus(status: number | string | undefined) {
  if (typeof status === "string") return status
  if (status === 1) return "ai_serving"
  if (status === 2) return "queued"
  if (status === 3) return "human_serving"
  if (status === 4) return "closed"
  return "waiting_customer"
}

function buildCreatedConversation(
  conversation: ImConversation,
  device?: CustomerPortalDevice,
): CustomerPortalConversation {
  const now = new Date().toISOString()
  const lastActiveAt = conversation.lastActiveAt || conversation.lastMessageAt || now
  return {
    id: conversation.id,
    status: mapCreatedConversationStatus(conversation.status),
    service_mode: conversation.serviceMode || 0,
    human_handoff_enabled: conversation.humanHandoffEnabled ?? false,
    ticket_creation_enabled: false,
    current_ticket_id: 0,
    priority: conversation.priority || 0,
    last_message_summary: conversation.lastMessageSummary || "",
    last_message_at: conversation.lastMessageAt || lastActiveAt,
    last_active_at: lastActiveAt,
    customer_unread_count: conversation.customerUnreadCount || 0,
    device_id: device?.id ?? conversation.deviceId ?? 0,
    device_no: device?.device_no ?? "",
    product_name: device?.product_name ?? "",
    current_assignee_name: conversation.currentAssigneeName || "",
    current_ticket_no: "",
    current_meeting_id: "",
    current_meeting_status: "",
  }
}

function upsertConversation(
  conversations: CustomerPortalConversation[],
  next: CustomerPortalConversation,
) {
  const existing = conversations.find((item) => item.id === next.id)
  if (!existing) return [next, ...conversations]
  return conversations.map((item) => item.id === next.id ? { ...next, ...item } : item)
}

function upsertDevice(
  devices: CustomerPortalDevice[],
  next: CustomerPortalDevice,
) {
  if (next.id <= 0) return devices
  if (!devices.some((item) => item.id === next.id)) return [next, ...devices]
  return devices.map((item) => item.id === next.id ? next : item)
}

function MetricLoadingCell({ icon, label }: { icon: ReactNode; label: string }) {
  const t = useI18n()
  return (
    <div className="min-w-0 px-2 text-center" role="status" aria-busy="true" aria-label={t("portalExtract.customerMobile.conversation.metricLoading", { label })}>
      {icon}
      <span className="mx-auto mt-2 block h-6 w-9 animate-pulse rounded-md bg-[#edf0f4]" />
      <span className="mx-auto mt-1 block h-3 w-12 animate-pulse rounded-md bg-[#edf0f4]" />
    </div>
  )
}

function ConversationListLoading() {
  const t = useI18n()
  return (
    <div className="space-y-2" role="status" aria-busy="true" aria-label={t("portalExtract.customerMobile.conversation.listLoading")}>
      {Array.from({ length: 5 }).map((_, index) => (
        <span key={index} className="block h-[104px] animate-pulse rounded-lg border border-[#e3e7ed] bg-white" />
      ))}
    </div>
  )
}

function MessagesLoading() {
  const t = useI18n()
  return (
    <div className="space-y-4" role="status" aria-busy="true" aria-label={t("portalExtract.customerMobile.conversation.readingMessages")}>
      {Array.from({ length: 6 }).map((_, index) => (
        <div key={index} className={cn("flex", index % 3 === 0 ? "justify-end" : "justify-start")}>
          <span className={cn("block h-14 w-[72%] animate-pulse rounded-lg", index % 3 === 0 ? "bg-[#dce8fa]" : "bg-white")} />
        </div>
      ))}
    </div>
  )
}

function DeviceSelectLoading() {
  const t = useI18n()
  return (
    <div className="space-y-2 py-2" role="status" aria-busy="true" aria-label={t("portalExtract.customerMobile.common.deviceListLoading")}>
      <div className="flex items-center gap-2 rounded-lg border border-[#dce2ea] bg-[#f8f9fb] px-3 py-3 text-xs text-[#657084]">
        <Loader2Icon className="size-3.5 animate-spin" />
        <span>{t("portalExtract.customerMobile.common.deviceListLoading")}</span>
      </div>
      {Array.from({ length: 3 }).map((_, index) => (
        <span key={index} className="block h-14 animate-pulse rounded-lg bg-[#edf0f4]" />
      ))}
    </div>
  )
}

export function MobileCustomerConversationWorkbench({ navigate }: { navigate: MobileCustomerNavigate }) {
  const router = useRouter()
  const t = useI18n()
  const { locale } = useAppLocale()
  const searchParams = useSearchParams()
  const { session } = useAuth()
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState("")
  const [conversations, setConversations] = useState<CustomerPortalConversation[]>([])
  const [hasLoadedConversations, setHasLoadedConversations] = useState(false)
  const [devices, setDevices] = useState<CustomerPortalDevice[]>([])
  const [devicesLoading, setDevicesLoading] = useState(true)
  const [profile, setProfile] = useState<CustomerPortalProfile | null>(null)
  const [profileLoading, setProfileLoading] = useState(true)
  const [selectedTicketDetail, setSelectedTicketDetail] = useState<CustomerPortalTicket | null>(null)
  const [ticketDetailLoading, setTicketDetailLoading] = useState(false)
  const [ticketActionLoading, setTicketActionLoading] = useState(false)
  const [ticketResolutionOpen, setTicketResolutionOpen] = useState(false)
  const [ticketRating, setTicketRating] = useState(5)
  const [ticketFeedbackComment, setTicketFeedbackComment] = useState("")
  const [deviceLoadError, setDeviceLoadError] = useState("")
  const [selectedId, setSelectedId] = useState(0)
  const [translatedMessages, setTranslatedMessages] = useState<Record<number, ConversationMessageTranslationDTO>>({})
  const [translatingMessageId, setTranslatingMessageId] = useState<number | null>(null)
  const [filter, setFilter] = useState<ConversationFilter>("all")
  const mobileConversationFilterTabs: RailopsTabItem[] = [
    { value: "all", label: t("portalExtract.customerMobile.common.all") },
    { value: "active", label: t("portalExtract.customerMobile.status.processing") },
    { value: "closed", label: t("portalExtract.customerMobile.common.ended") },
  ]
  const [query, setQuery] = useState("")
  const [conversationPage, setConversationPage] = useState(1)
  const [conversationTotal, setConversationTotal] = useState(0)
  const [messages, setMessages] = useState<ImMessage[]>([])
  const [messageLoading, setMessageLoading] = useState(false)
  const [loadedMessageConversationId, setLoadedMessageConversationId] = useState(0)
  const [olderLoading, setOlderLoading] = useState(false)
  const [messageCursor, setMessageCursor] = useState("")
  const [hasOlderMessages, setHasOlderMessages] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [createDeviceId, setCreateDeviceId] = useState("")
  const [creating, setCreating] = useState(false)
  const [draft, setDraft] = useState("")
  const [sending, setSending] = useState(false)
  const [uploadingAsset, setUploadingAsset] = useState(false)
  const [handoffLoading, setHandoffLoading] = useState(false)
  const [notice, setNotice] = useState("")
  const [createError, setCreateError] = useState("")
  const [realtimeStatus, setRealtimeStatus] = useState<RealtimeConnectionStatus>("disconnected")

  const messagesEndRef = useRef<HTMLDivElement | null>(null)
  const messagesViewportRef = useRef<HTMLDivElement | null>(null)
  const cameraInputRef = useRef<HTMLInputElement | null>(null)
  const imageInputRef = useRef<HTMLInputElement | null>(null)
  const attachmentInputRef = useRef<HTMLInputElement | null>(null)
  const failedMediaTransfersRef = useRef(new Map<string, FailedMobileMediaTransfer>())
  const conversationsRef = useRef<CustomerPortalConversation[]>([])
  const selectedIdRef = useRef(0)
  const selectedClosedRef = useRef(false)
  const selectedTicketIdRef = useRef(0)
  const messageRequestRef = useRef(0)
  const ticketDetailRequestRef = useRef(0)
  const stickToBottomRef = useRef(true)
  const scrollOnMessageUpdateRef = useRef(false)
  const handledDeviceRequestRef = useRef(0)
  const invalidConversationRef = useRef(0)
  const realtimeStatusRef = useRef<RealtimeConnectionStatus>("disconnected")
  const conversationPageRef = useRef(1)

  const selected = conversations.find((item) => item.id === selectedId) ?? null
  const selectedTicketId = selected?.current_ticket_id ?? 0
  const requestedConversationId = Number(searchParams.get("conversationId") || "0")
  const requestedDeviceId = Number(searchParams.get("deviceId") || "0")
  const hasLoadedMessages = selectedId > 0 && loadedMessageConversationId === selectedId

  async function translateMessage(message: ImMessage) {
    if (!selected || !message.id || !message.content.trim() || translatingMessageId !== null) return
    setTranslatingMessageId(message.id)
    try {
      const targetLanguage = locale.toLowerCase().startsWith("zh") ? "en" : "zh-CN"
      const translation = await translateCustomerConversationMessage(selected.id, message.id, targetLanguage)
      setTranslatedMessages((current) => ({ ...current, [message.id]: translation }))
    } catch (error) {
      setNotice(error instanceof Error ? error.message : t("portalExtract.customerMobile.conversation.loadFailed"))
    } finally {
      setTranslatingMessageId(null)
    }
  }
  const conversationInitialLoading = loading && !hasLoadedConversations
  const messageInitialLoading = messageLoading && !hasLoadedMessages
  const tenantAIEnabled = session?.featureFlags?.ai !== false
  const hasDeviceConcept = session?.featureFlags?.device !== false
  const readOnlyGuest = !hasDeviceConcept ? false : profile
    ? profile.bound_device_count === 0 && !profile.company_name.trim()
    : profileLoading || devicesLoading || devices.length === 0

  const handleMessageMediaSettled = useCallback(() => {
    if (stickToBottomRef.current) messagesEndRef.current?.scrollIntoView({ block: "end" })
  }, [])

  const visibleConversations = conversations

  const supplierMessageContexts = useMemo(() => {
    const contexts = new Map<string, SupplierMessageContext>()
    for (const message of messages) {
      const event = parseConversationServiceEvent(message)
      if (event?.kind !== "supplier" || !event.collaborationId) continue
      const existing = contexts.get(event.collaborationId) ?? { partnerCompanyName: "", productModuleName: "" }
      contexts.set(event.collaborationId, {
        partnerCompanyName: event.partnerCompanyName || existing.partnerCompanyName,
        productModuleName: event.productModuleName || existing.productModuleName,
      })
    }
    return contexts
  }, [messages])

  const resolveMobileVideoHref = useCallback((event: ConversationServiceEventData) => {
    const meetingId = event.meetingId?.trim()
    if (
      !meetingId ||
      event.kind !== "video" ||
      (event.eventType !== "meeting_started" && event.eventType !== "meeting_scheduled")
    ) {
      return ""
    }
    const params = new URLSearchParams(searchParams.toString())
    params.set("state", "video")
    params.set("id", meetingId)
    params.delete("conversationId")
    params.delete("deviceId")
    return `/mobile?${params.toString()}`
  }, [searchParams])

  const metrics = useMemo(() => ({
    total: conversationTotal,
    active: conversations.filter((item) => !isConversationClosed(item)).length,
    unread: conversations.reduce((total, item) => total + Math.max(0, item.customer_unread_count || 0), 0),
  }), [conversationTotal, conversations])

  const loadBase = useCallback(async (
    silent = false,
    fallbackConversation?: CustomerPortalConversation,
    pageNumber = conversationPageRef.current,
    append = false,
  ) => {
    if (!silent) {
      setLoading(true)
      setLoadError("")
    }
    try {
      const conversationPageData = await fetchCustomerConversationsPage({
        page: pageNumber,
        limit: conversationPageSize,
        locale,
        keyword: query.trim(),
        filter: filter === "all" ? undefined : filter,
      })
      const nextConversations = conversationPageData.results ?? []
      let displayedConversations = fallbackConversation
        ? upsertConversation(nextConversations, fallbackConversation)
        : nextConversations
      const currentSelected = conversationsRef.current.find((item) => item.id === selectedIdRef.current)
      if (!append && currentSelected && !displayedConversations.some((item) => item.id === currentSelected.id)) {
        displayedConversations = upsertConversation(displayedConversations, currentSelected)
      }
      const mergedConversations = append
        ? (() => {
            const seen = new Set(conversationsRef.current.map((item) => item.id))
            return [
              ...conversationsRef.current,
              ...displayedConversations.filter((item) => !seen.has(item.id)),
            ]
          })()
        : displayedConversations
      conversationsRef.current = mergedConversations
      setConversations(mergedConversations)
      conversationPageRef.current = pageNumber
      setConversationPage(pageNumber)
      setConversationTotal(conversationPageData.page?.total ?? mergedConversations.length)
      setHasLoadedConversations(true)
      setSelectedId((current) => mergedConversations.some((item) => item.id === current) ? current : 0)
      setLoading(false)
      setLoadError("")
    } catch (error) {
      const message = error instanceof Error ? error.message : t("portalExtract.customerMobile.conversation.loadFailed")
      if (silent && conversationsRef.current.length > 0) setNotice(message)
      else {
        setLoading(false)
        setLoadError(message)
      }
    }
  }, [filter, locale, query])

  const loadDevices = useCallback(async (silent = false) => {
    if (!hasDeviceConcept) {
      setDevices([])
      setDeviceLoadError("")
      setDevicesLoading(false)
      return
    }
    if (!silent) {
      setDevicesLoading(true)
      setDeviceLoadError("")
    }
    try {
      const page = await fetchCustomerDevicesPage({ page: 1, limit: 100 })
      let items = page.results ?? []
      if (requestedDeviceId > 0 && !items.some((item) => item.id === requestedDeviceId)) {
        const requestedDevicePage = await fetchCustomerDevicesPage({ deviceId: requestedDeviceId, limit: 1 })
        const requestedDevice = requestedDevicePage.results?.[0]
        if (requestedDevice) items = upsertDevice(items, requestedDevice)
      }
      setDevices(items)
      setDeviceLoadError("")
    } catch (error) {
      setDeviceLoadError(error instanceof Error ? error.message : t("portalExtract.customerMobile.conversation.deviceListFailed"))
    } finally {
      if (!silent) {
        setDevicesLoading(false)
      }
    }
  }, [hasDeviceConcept, requestedDeviceId])

  const loadProfile = useCallback(async (silent = false) => {
    if (!silent) setProfileLoading(true)
    try {
      setProfile(await fetchCustomerProfile())
    } catch {
      setProfile(null)
    } finally {
      if (!silent) setProfileLoading(false)
    }
  }, [])

  const activateCreatedConversation = useCallback((created: ImConversation, device?: CustomerPortalDevice) => {
    const fallback = buildCreatedConversation(created, device)
    const displayedConversations = upsertConversation(conversationsRef.current, fallback)
    conversationsRef.current = displayedConversations
    selectedIdRef.current = created.id
    messageRequestRef.current += 1
    setConversations(displayedConversations)
    setCreateOpen(false)
    setSelectedId(created.id)
    navigate("chat", { conversationId: String(created.id) })
    void loadBase(true, fallback)
  }, [loadBase, navigate])

  const loadMessages = useCallback(async (conversationId: number, silent = false) => {
    if (!conversationId) return
    const requestId = ++messageRequestRef.current
    if (!silent) {
      setMessageLoading(true)
      setMessages([])
      setLoadedMessageConversationId(0)
      setMessageCursor("")
      setHasOlderMessages(false)
    }
    try {
      const page = await fetchImMessages({ conversationId, limit: 50 })
      if (requestId !== messageRequestRef.current || selectedIdRef.current !== conversationId) return
      const nextMessages = page.results ?? []
      scrollOnMessageUpdateRef.current = !silent || stickToBottomRef.current
      setMessages((current) => silent
        ? replaceLoadedImMessagesPreservingLocalTransfers(current, nextMessages)
        : nextMessages)
      setLoadedMessageConversationId(conversationId)
      if (!silent) {
        setMessageCursor(page.cursor || "")
        setHasOlderMessages(Boolean(page.hasMore))
      }
      setConversations((current) => current.map((item) => item.id === conversationId ? { ...item, customer_unread_count: 0 } : item))
      void markImMessageRead(conversationId).catch(() => undefined)
    } catch (error) {
      if (requestId === messageRequestRef.current && selectedIdRef.current === conversationId && !silent) {
        setNotice(error instanceof Error ? error.message : t("portalExtract.customerMobile.conversation.messageLoadFailed"))
      }
    } finally {
      if (requestId === messageRequestRef.current) setMessageLoading(false)
    }
  }, [])

  const loadSelectedTicketDetail = useCallback(async (ticketId: number, silent = false) => {
    if (ticketId <= 0) return
    const requestId = ++ticketDetailRequestRef.current
    if (!silent) setTicketDetailLoading(true)
    try {
      const detail = await fetchCustomerTicketDetail(ticketId)
      if (requestId !== ticketDetailRequestRef.current || selectedTicketIdRef.current !== ticketId) return
      setSelectedTicketDetail(detail)
    } catch (error) {
      if (requestId !== ticketDetailRequestRef.current || selectedTicketIdRef.current !== ticketId || silent) return
      setNotice(error instanceof Error ? error.message : t("portalExtract.customerMobile.ticket.loadFailed"))
    } finally {
      if (requestId === ticketDetailRequestRef.current) setTicketDetailLoading(false)
    }
  }, [t])

  useEffect(() => { void loadBase() }, [loadBase])
  useEffect(() => { void loadDevices() }, [loadDevices])
  useEffect(() => { void loadProfile() }, [loadProfile])
  useEffect(() => () => {
    failedMediaTransfersRef.current.clear()
  }, [])
  useEffect(() => {
    failedMediaTransfersRef.current.clear()
    selectedIdRef.current = selectedId
    stickToBottomRef.current = true
    setNotice("")
    if (selectedId) void loadMessages(selectedId)
    else {
      messageRequestRef.current += 1
      setMessages([])
      setLoadedMessageConversationId(0)
      setMessageLoading(false)
    }
  }, [loadMessages, selectedId])
  useEffect(() => {
    selectedClosedRef.current = selected ? isConversationClosed(selected) : false
  }, [selected])
  useEffect(() => {
    selectedTicketIdRef.current = selectedTicketId
    ticketDetailRequestRef.current += 1
    setSelectedTicketDetail(null)
    setTicketDetailLoading(false)
    setTicketActionLoading(false)
    setTicketRating(5)
    setTicketFeedbackComment("")
    if (selectedTicketId > 0) void loadSelectedTicketDetail(selectedTicketId)
  }, [loadSelectedTicketDetail, selectedTicketId])
  useEffect(() => {
    if (!scrollOnMessageUpdateRef.current) return
    messagesEndRef.current?.scrollIntoView({ block: "end" })
    scrollOnMessageUpdateRef.current = false
  }, [messages])
  useEffect(() => {
    if (requestedConversationId > 0 && conversations.some((item) => item.id === requestedConversationId)) {
      invalidConversationRef.current = 0
      setSelectedId(requestedConversationId)
      return
    }
    if (!requestedConversationId) {
      setSelectedId(0)
      return
    }
    let cancelled = false
    void fetchCustomerConversationsPage({ conversationId: requestedConversationId, limit: 1, locale })
      .then((page) => {
        if (cancelled) return
        const item = page.results?.[0]
        if (item) {
          invalidConversationRef.current = 0
          const nextConversations = upsertConversation(conversationsRef.current, item)
          conversationsRef.current = nextConversations
          setConversations(nextConversations)
          setSelectedId(requestedConversationId)
          return
        }
        if (!loading && invalidConversationRef.current !== requestedConversationId) {
          invalidConversationRef.current = requestedConversationId
          setNotice(t("portalExtract.customerMobile.conversation.forbidden"))
          navigate("chat")
        }
      })
      .catch(() => {
        if (!cancelled && !loading && invalidConversationRef.current !== requestedConversationId) {
          invalidConversationRef.current = requestedConversationId
          setNotice(t("portalExtract.customerMobile.conversation.forbidden"))
          navigate("chat")
        }
      })
    return () => {
      cancelled = true
    }
  }, [conversations, loading, locale, navigate, requestedConversationId])
  useEffect(() => {
    if (!hasDeviceConcept || requestedDeviceId <= 0 || !hasLoadedConversations || devicesLoading || creating || handledDeviceRequestRef.current === requestedDeviceId) return
    handledDeviceRequestRef.current = requestedDeviceId
    setCreateDeviceId(String(requestedDeviceId))
    setCreateError("")
    setCreateOpen(true)
  }, [creating, devicesLoading, hasDeviceConcept, hasLoadedConversations, requestedDeviceId])
  useEffect(() => {
    const accessToken = session?.accessToken?.trim()
    const conversationId = selectedId
    if (!accessToken || session?.domainType !== "customer" || conversationId <= 0) {
      realtimeStatusRef.current = "disconnected"
      setRealtimeStatus("disconnected")
      return
    }

    let active = true
    const updateRealtimeStatus = (status: RealtimeConnectionStatus) => {
      if (!active) return
      realtimeStatusRef.current = status
      setRealtimeStatus(status)
    }
    const syncConversation = () => {
      void loadBase(true)
      void loadDevices(true)
      void loadProfile(true)
      if (selectedTicketId > 0) void loadSelectedTicketDetail(selectedTicketId, true)
      void loadMessages(conversationId, true)
    }
    const realtime = createRealtimeConnectionManager({
      createSocket: () => createAccessTokenWebSocket(
        `${createWebSocketBaseUrl()}/api/ws/open`,
        accessToken,
      ),
      canReconnect: () => active && selectedIdRef.current === conversationId,
      onStatusChange: updateRealtimeStatus,
      onOpen: (socket) => {
        socket.send(JSON.stringify({ type: "subscribe", topics: [`conversation:${conversationId}`] }))
        syncConversation()
      },
      onMessage: (event) => {
        if (!active) return
        let envelope: {
          type?: string
          data?: RealtimeMessageCreatedPayload<ImMessage>
          payload?: RealtimeMessageCreatedPayload<ImMessage>
        }
        try {
          envelope = JSON.parse(String(event.data)) as typeof envelope
        } catch {
          return
        }

        const payload = envelope.data ?? envelope.payload
        if (envelope.type === "resyncRequired") {
          syncConversation()
          return
        }
        if (envelope.type === "message.created") {
          const message = normalizeRealtimeMessage<ImMessage>(payload)
          if (!message || message.conversationId !== conversationId) {
            syncConversation()
            return
          }
          const isOwnMessage = message.senderType === "customer" || message.senderType === "user"
          scrollOnMessageUpdateRef.current = isOwnMessage || stickToBottomRef.current
          setMessages((current) => upsertImMessageReplacingClientMessage(current, message))
          setLoadedMessageConversationId(conversationId)
          void loadBase(true)
          if (selectedTicketId > 0) void loadSelectedTicketDetail(selectedTicketId, true)
          if (!isOwnMessage && document.visibilityState === "visible") {
            void markImMessageRead(conversationId, message.id).catch(() => undefined)
          }
          return
        }
        if (envelope.type?.startsWith("conversation.")) {
          syncConversation()
        }
      },
      reconnectBaseDelayMs: 500,
      reconnectMaxDelayMs: 5_000,
    })

    realtime.connect()
    return () => {
      active = false
      realtime.disconnect()
      realtimeStatusRef.current = "disconnected"
      setRealtimeStatus("disconnected")
    }
  }, [loadBase, loadDevices, loadMessages, loadProfile, loadSelectedTicketDetail, selectedId, selectedTicketId, session?.accessToken, session?.domainType])
  useEffect(() => {
    const refresh = () => {
      if (document.visibilityState !== "visible") return
      void loadBase(true)
      void loadDevices(true)
      void loadProfile(true)
      if (selectedTicketIdRef.current > 0) void loadSelectedTicketDetail(selectedTicketIdRef.current, true)
      if (selectedIdRef.current && !selectedClosedRef.current) void loadMessages(selectedIdRef.current, true)
    }
    const baseTimer = window.setInterval(() => {
      if (document.visibilityState === "visible") {
        void loadBase(true)
        void loadDevices(true)
        void loadProfile(true)
        if (selectedTicketIdRef.current > 0) void loadSelectedTicketDetail(selectedTicketIdRef.current, true)
      }
    }, 15000)
    const messageTimer = window.setInterval(() => {
      if (
        document.visibilityState === "visible" &&
        selectedIdRef.current &&
        !selectedClosedRef.current &&
        realtimeStatusRef.current !== "connected"
      ) {
        void loadMessages(selectedIdRef.current, true)
      }
    }, 5000)
    document.addEventListener("visibilitychange", refresh)
    return () => {
      window.clearInterval(baseTimer)
      window.clearInterval(messageTimer)
      document.removeEventListener("visibilitychange", refresh)
    }
  }, [loadBase, loadDevices, loadMessages, loadProfile, loadSelectedTicketDetail])

  function openConversation(conversationId: number) {
    selectedIdRef.current = conversationId
    messageRequestRef.current += 1
    setSelectedId(conversationId)
    navigate("chat", { conversationId: String(conversationId) })
  }

  function closeConversation() {
    selectedIdRef.current = 0
    messageRequestRef.current += 1
    setSelectedId(0)
    navigate("chat")
  }

  function openSelectedDevice() {
    if (selected?.device_id) {
      navigate("devices", {
        deviceId: String(selected.device_id),
        fromConversationId: String(selected.id),
      })
      return
    }
    navigate("devices")
  }

  function openCreateConversationDialog() {
    if (!hasDeviceConcept) {
      setNotice("")
      void createSystemHelpConversation()
      return
    }
    setCreateDeviceId(devices[0] ? String(devices[0].id) : "")
    setCreateError("")
    setCreateOpen(true)
  }

  function openAddDeviceEntry() {
    setCreateOpen(false)
    setCreateError("")
    router.replace("/mobile?state=scan")
  }

  async function loadOlderMessages() {
    if (!selectedId || !messageCursor || olderLoading) return
    const viewport = messagesViewportRef.current
    const previousHeight = viewport?.scrollHeight || 0
    setOlderLoading(true)
    try {
      const page = await fetchImMessages({ conversationId: selectedId, cursor: messageCursor, limit: 50 })
      if (selectedIdRef.current !== selectedId) return
      scrollOnMessageUpdateRef.current = false
      setMessages((current) => mergeImMessagesReplacingClientMessages(current, page.results ?? []))
      setMessageCursor(page.cursor || "")
      setHasOlderMessages(Boolean(page.hasMore))
      window.requestAnimationFrame(() => {
        if (viewport) viewport.scrollTop += viewport.scrollHeight - previousHeight
      })
    } catch (error) {
      setNotice(error instanceof Error ? error.message : t("portalExtract.customerMobile.conversation.olderMessagesFailed"))
    } finally {
      setOlderLoading(false)
    }
  }

  async function createConversation() {
    if (creating) return
    const deviceId = Number(createDeviceId || "0")
    if (deviceId <= 0) return
    setCreating(true)
    setCreateError("")
    try {
      const created = await createOrMatchImConversation({ deviceId, forceNew: true })
      const selectedDevice = devices.find((item) => item.id === deviceId)
      activateCreatedConversation(created, selectedDevice)
    } catch (error) {
      setCreateError(error instanceof Error ? error.message : t("portalExtract.customerMobile.conversation.createFailed"))
    } finally {
      setCreating(false)
    }
  }

  async function createSystemHelpConversation() {
    if (!tenantAIEnabled) return
    if (readOnlyGuest || creating) return
    setCreating(true)
    setCreateError("")
    try {
      const created = await createOrMatchImConversation({ general: true, forceNew: true })
      activateCreatedConversation(created)
    } catch (error) {
      const message = error instanceof Error ? error.message : t("portalExtract.customerMobile.conversation.systemHelpFailed")
      setCreateError(message)
      if (!hasDeviceConcept) setNotice(message)
    } finally {
      setCreating(false)
    }
  }

  async function sendDraftMessage() {
    const content = draft.trim()
    if (readOnlyGuest || !selected || !content || sending || isConversationClosed(selected)) return
    setSending(true)
    setNotice("")
    try {
      const sent = await sendImMessage({
        conversationId: selected.id,
        messageType: "text",
        content,
        clientMsgId: globalThis.crypto?.randomUUID?.(),
      })
      stickToBottomRef.current = true
      scrollOnMessageUpdateRef.current = true
      setMessages((current) => upsertImMessageReplacingClientMessage(current, sent))
      setDraft("")
      window.setTimeout(() => { void loadMessages(selected.id, true) }, 900)
    } catch (error) {
      const message = error instanceof Error ? error.message : t("portalExtract.customerMobile.conversation.sendFailed")
      setNotice(t("portalExtract.customerMobile.conversation.draftKept", { message }))
    } finally {
      setSending(false)
    }
  }

  async function submitMessage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    await sendDraftMessage()
  }

  async function sendMediaMessage(
    file: File,
    messageType: MobileMediaKind,
    retry?: { clientMsgId: string; uploadedAsset?: ImAsset; durationSeconds?: number },
  ) {
    if (readOnlyGuest || !selected || uploadingAsset || sending || isConversationClosed(selected)) return
    const validationError = validateMobileMediaSelection(file, messageType)
    if (validationError) {
      setNotice(mobileMediaSelectionErrorMessage(validationError, t))
      return
    }
    if (typeof navigator !== "undefined" && navigator.onLine === false) {
      setNotice(t("portalExtract.customerMobile.conversation.offlineRetry"))
      return
    }

    const conversationId = selected.id
    const clientMsgId = retry?.clientMsgId ?? `mobile_customer_${messageType}_${globalThis.crypto?.randomUUID?.() ?? Date.now()}`
    const durationSeconds = retry?.durationSeconds
    const pending = buildPendingMobileMediaMessage({ conversationId, clientMsgId, messageType, file, durationSeconds })
    scrollOnMessageUpdateRef.current = true
    setMessages((current) => retry
      ? current.map((message) => message.clientMsgId === clientMsgId
        ? { ...pending, id: message.id, sentAt: message.sentAt }
        : message)
      : [...current, pending])
    setUploadingAsset(true)
    setNotice(mobileMediaSendingMessage(messageType, t))
    let uploadedAsset = retry?.uploadedAsset
    try {
      uploadedAsset ??= messageType === "image"
        ? await uploadImImage(conversationId, file)
        : messageType === "audio"
          ? await uploadImAudio(conversationId, file)
          : await uploadImAttachment(conversationId, file)
      const sent = await sendImMessage({
        conversationId,
        messageType,
        content: uploadedAsset.filename,
        payload: JSON.stringify({
          assetId: uploadedAsset.assetId,
          ...(durationSeconds ? { durationSeconds } : {}),
        }),
        clientMsgId,
      })
      failedMediaTransfersRef.current.delete(clientMsgId)
      if (selectedIdRef.current === conversationId) {
        scrollOnMessageUpdateRef.current = true
        setMessages((current) => upsertImMessageReplacingClientMessage(current, sent))
        setNotice(mobileMediaSentMessage(messageType, t))
        void loadMessages(conversationId, true)
      }
      void loadBase(true)
    } catch (error) {
      if (selectedIdRef.current === conversationId) {
        failedMediaTransfersRef.current.set(clientMsgId, {
          conversationId,
          file,
          messageType,
          durationSeconds,
          uploadedAsset,
        })
        setMessages((current) => current.map((message) =>
          message.clientMsgId === clientMsgId ? markPendingMobileMediaFailed(message, file, durationSeconds) : message
        ))
        setNotice(error instanceof Error
          ? error.message
          : mobileMediaFailedMessage(messageType, t))
      }
    } finally {
      setUploadingAsset(false)
    }
  }

  function selectMedia(event: ChangeEvent<HTMLInputElement>, messageType: MobileMediaKind) {
    const file = event.currentTarget.files?.[0]
    event.currentTarget.value = ""
    if (!file) return
    const validationError = validateMobileMediaSelection(file, messageType)
    if (validationError) {
      setNotice(mobileMediaSelectionErrorMessage(validationError, t))
      return
    }

    setNotice("")
    void sendMediaMessage(file, messageType)
  }

  async function sendVoiceMessage(file: File, durationSeconds: number) {
    await sendMediaMessage(file, "audio", { clientMsgId: `mobile_customer_audio_${globalThis.crypto?.randomUUID?.() ?? Date.now()}`, durationSeconds })
  }

  async function retryMediaMessage(clientMsgId: string) {
    const transfer = failedMediaTransfersRef.current.get(clientMsgId)
    if (!transfer || transfer.conversationId !== selected?.id) {
      setNotice(t("portalExtract.customerMobile.conversation.originalFileMissing"))
      return
    }
    await sendMediaMessage(transfer.file, transfer.messageType, {
      clientMsgId,
      uploadedAsset: transfer.uploadedAsset,
      durationSeconds: transfer.durationSeconds,
    })
  }

  function dismissFailedMediaMessage(clientMsgId: string) {
    failedMediaTransfersRef.current.delete(clientMsgId)
    setMessages((current) => current.filter((message) => message.clientMsgId !== clientMsgId))
    setNotice("")
  }

  async function requestHuman() {
    if (readOnlyGuest || !selected || handoffLoading) return
    setHandoffLoading(true)
    setNotice("")
    try {
      await requestImHumanSupport(selected.id, t("portalExtract.customerMobile.conversation.handoffReason"))
      setNotice(t("portalExtract.customerMobile.conversation.handoffSubmitted"))
      await loadBase(true)
      await loadMessages(selected.id, true)
    } catch (error) {
      setNotice(error instanceof Error ? error.message : t("portalExtract.customerMobile.conversation.handoffFailed"))
    } finally {
      setHandoffLoading(false)
    }
  }

  async function handleTicketResolutionAction() {
    const ticket = selectedTicketDetail?.id === selectedTicketId ? selectedTicketDetail : null
    if (readOnlyGuest || !selected || !ticket || ticketActionLoading) return
    const shouldSubmitFeedback = ticket.can_rate && !ticket.feedback
    if (shouldSubmitFeedback && ticketRating <= 0) {
      setNotice(t("portalExtract.customerMobile.ticket.selectRatingFirst"))
      return
    }

    setTicketActionLoading(true)
    setNotice("")
    try {
      let nextTicket = ticket
      if (shouldSubmitFeedback) {
        const feedback = await submitCustomerTicketFeedback(ticket.id, {
          rating: ticketRating,
          comment: ticketFeedbackComment.trim(),
        })
        nextTicket = { ...nextTicket, feedback, can_rate: false }
      }
      if (ticket.can_confirm) {
        nextTicket = await confirmCustomerTicket(ticket.id)
      }
      setSelectedTicketDetail(nextTicket)
      setTicketRating(5)
      setTicketFeedbackComment("")
      setTicketResolutionOpen(false)
      setNotice(ticket.can_confirm
        ? t("portalExtract.customerMobile.ticket.ticketEnded")
        : t("portalExtract.customerMobile.ticket.feedbackSubmitted"))
      await Promise.all([
        loadBase(true),
        loadMessages(selected.id, true),
      ])
      void loadSelectedTicketDetail(ticket.id, true)
    } catch (error) {
      setNotice(error instanceof Error ? error.message : t("portalExtract.customerMobile.ticket.submitFailed"))
    } finally {
      setTicketActionLoading(false)
    }
  }

  const createDialog = hasDeviceConcept ? (
    <Dialog open={createOpen} onOpenChange={(open) => { setCreateOpen(open); if (!open) { setCreateError(""); if (requestedDeviceId > 0) navigate("chat") } }}>
      <DialogContent className="rhd-mobile-dialog w-[calc(100vw-32px)] max-w-sm overflow-hidden rounded-lg p-5">
        <DialogHeader><DialogTitle>{t("portalExtract.customerMobile.conversation.newConversation")}</DialogTitle></DialogHeader>
        {devicesLoading ? (
          <DeviceSelectLoading />
        ) : devices.length > 0 ? (
          <div className="min-w-0 space-y-2 py-2">
            <SelectField
              className="rhd-mobile-stacked-field"
              label={t("portalExtract.customerMobile.conversation.target")}
              labelCol={{ span: 24 }}
              wrapperCol={{ span: 24 }}
              style={{ marginBottom: 0 }}
              selectProps={{
                id: "mobile-chat-device",
                className: "w-full min-w-0",
                size: "large",
                placeholder: t("portalExtract.customerMobile.conversation.selectDevice"),
                value: createDeviceId || undefined,
                onChange: (value) => setCreateDeviceId((value as string) ?? ""),
                options: [
                  ...(createDeviceId && !devices.some((device) => String(device.id) === createDeviceId)
                    ? [{
                      value: createDeviceId,
                      label: <span className="block truncate">{t("portalExtract.customerMobile.conversation.specificDevice")}</span>,
                    }]
                    : []),
                  ...devices.map((device) => ({
                    value: String(device.id),
                    label: <span className="block truncate">{device.product_name || device.device_no} · {device.device_no}</span>,
                  })),
                ],
                style: { width: "100%" },
              }}
            />
            {deviceLoadError ? <p className="text-xs leading-5 text-[#9a5a0a]" role="status">{t("portalExtract.customerMobile.conversation.cachedDevices")}</p> : null}
            <Button type="button" variant="outline" className="h-10 w-full border-[#dce2ea] bg-white text-[#465266] shadow-none" onClick={openAddDeviceEntry} disabled={creating}>
              <PlusIcon className="size-4" />
              {t("portalExtract.customerMobile.conversation.addDevice")}
            </Button>
          </div>
        ) : (
          <div className="rounded-lg border border-dashed border-[#dce2ea] bg-[#f8f9fb] px-3 py-3 text-xs leading-5 text-[#657084]">
            <div>
              <span>{t("portalExtract.customerMobile.conversation.noDeviceCreateHint")} </span>
              <button type="button" className="inline-flex items-center gap-1 font-semibold text-[#1769e0] underline-offset-4 hover:underline disabled:opacity-60" onClick={openAddDeviceEntry} disabled={creating}>
                <PlusIcon className="size-3.5" />
                {t("portalExtract.customerMobile.conversation.addDevice")}
              </button>
            </div>
            {!readOnlyGuest && tenantAIEnabled ? (
              <div className="mt-2 border-t border-[#e5e9ef] pt-2">
                <span>{t("portalExtract.customerMobile.conversation.noDeviceHelpHint")} </span>
                <button type="button" className="inline-flex items-center gap-1 font-semibold text-[#1769e0] underline-offset-4 hover:underline disabled:opacity-60" onClick={() => void createSystemHelpConversation()} disabled={creating}>
                  {creating ? <Loader2Icon className="size-3.5 animate-spin" /> : <BookOpenIcon className="size-3.5" />}
                  {t("portalExtract.customerMobile.conversation.openSystemHelp")}
                </button>
              </div>
            ) : null}
          </div>
        )}
        {!devicesLoading && deviceLoadError && devices.length === 0 ? <p className="text-xs leading-5 text-[#9a5a0a]" role="status">{t("portalExtract.customerMobile.conversation.deviceUnavailableUseHelp")}</p> : null}
        {createError ? <p className="text-xs text-red-600" role="alert">{createError}</p> : null}
        <DialogFooter className="min-w-0">
          <Button className="h-11 w-full sm:w-auto" variant="outline" onClick={() => { setCreateOpen(false); setCreateError(""); if (requestedDeviceId > 0) navigate("chat") }}>{t("portalExtract.customerMobile.common.cancel")}</Button>
          {devices.length > 0 ? <Button className="h-11 w-full sm:w-auto" onClick={() => void createConversation()} disabled={creating || !createDeviceId}>{creating ? <Loader2Icon className="size-4 animate-spin" /> : <MessageSquarePlusIcon className="size-4" />}{t("portalExtract.customerMobile.serviceShell.startConsult")}</Button> : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  ) : null

  if (loadError) {
    return (
      <div className="flex h-full flex-col items-center justify-center px-8 text-center">
        <span className="flex size-12 items-center justify-center rounded-full bg-red-50 text-red-600"><CircleAlertIcon className="size-5" /></span>
        <h2 className="mt-4 text-sm font-semibold text-[#172033]">{t("portalExtract.customerMobile.conversation.loadFailedTitle")}</h2>
        <p className="mt-1 text-xs leading-5 text-[#778195]">{loadError}</p>
        <Button type="button" variant="outline" size="sm" className="mt-4" onClick={() => void loadBase()}><RefreshCwIcon className="size-4" />{t("portalExtract.customerMobile.common.reload")}</Button>
      </div>
    )
  }

  if (!selected) {
    return (
      <div className="h-full overflow-y-auto overscroll-contain bg-[#f4f6f8] pb-4">
        <div className="grid grid-cols-3 divide-x divide-[#edf0f4] border-b border-[#e5e9ef] bg-white px-3 py-3">
          {conversationInitialLoading ? (
            <>
              <MetricLoadingCell icon={<MessagesSquareIcon className="mx-auto size-4 text-[#b8c0cc]" />} label={t("portalExtract.customerMobile.common.allConversations")} />
              <MetricLoadingCell icon={<RadioIcon className="mx-auto size-4 text-[#b8c0cc]" />} label={t("portalExtract.customerMobile.status.processing")} />
              <MetricLoadingCell icon={<CircleAlertIcon className="mx-auto size-4 text-[#b8c0cc]" />} label={t("portalExtract.customerMobile.conversation.unreadMessages")} />
            </>
          ) : (
            <>
              <div className="min-w-0 px-2 text-center"><MessagesSquareIcon className="mx-auto size-4 text-[#8290a3]" /><strong className="mt-1 block text-lg leading-6 text-[#172033]">{metrics.total}</strong><span className="block truncate text-rhd-2xs text-[#778195]">{t("portalExtract.customerMobile.common.allConversations")}</span></div>
              <div className="min-w-0 px-2 text-center"><RadioIcon className="mx-auto size-4 text-[#0f9f76]" /><strong className="mt-1 block text-lg leading-6 text-[#172033]">{metrics.active}</strong><span className="block truncate text-rhd-2xs text-[#778195]">{t("portalExtract.customerMobile.status.processing")}</span></div>
              <div className="min-w-0 px-2 text-center"><CircleAlertIcon className="mx-auto size-4 text-[#d97706]" /><strong className="mt-1 block text-lg leading-6 text-[#172033]">{metrics.unread}</strong><span className="block truncate text-rhd-2xs text-[#778195]">{t("portalExtract.customerMobile.conversation.unreadMessages")}</span></div>
            </>
          )}
        </div>

        <div className="sticky top-0 z-20 space-y-3 border-b border-[#e5e9ef] bg-[#f4f6f8]/96 px-4 py-3 backdrop-blur-xl">
          <div className="flex gap-2">
            <div className="min-w-0 flex-1">
              <SearchField allowClear className="rhd-railops-search-mobile" value={query} onChange={(event) => { conversationPageRef.current = 1; setConversationPage(1); setQuery(event.target.value) }} placeholder={t(hasDeviceConcept ? "portalExtract.customerMobile.conversation.search" : "portalExtract.customerMobile.conversation.searchGeneral")} />
            </div>
            <Button type="button" size="icon" className="size-11 shrink-0 bg-[#1769e0] shadow-none hover:bg-[#125fcf]" onClick={openCreateConversationDialog} aria-label={t("portalExtract.customerMobile.conversation.newConversation")} title={t("portalExtract.customerMobile.conversation.newConversation")} disabled={conversationInitialLoading}><PlusIcon className="size-5" /></Button>
          </div>
          <FilterTabs
            ariaLabel={t("portalExtract.customerMobile.conversation.filterAria")}
            items={mobileConversationFilterTabs}
            value={filter}
            onChange={(value) => {
              conversationPageRef.current = 1
              setConversationPage(1)
              setFilter(value as ConversationFilter)
            }}
          />
        </div>

        {notice ? <p className="mx-4 mt-3 rounded-md bg-white px-3 py-2 text-center text-xs text-[#657084]" aria-live="polite">{notice}</p> : null}
        <div className="space-y-2 px-3 pt-3">
          {conversationInitialLoading ? <ConversationListLoading /> : visibleConversations.map((item) => (
            <button key={item.id} type="button" className="flex min-h-[104px] w-full items-center gap-3 rounded-lg border border-[#e3e7ed] bg-white px-4 py-3 text-left shadow-[0_1px_1px_rgba(15,23,42,0.02)] active:bg-[#f8f9fb]" onClick={() => openConversation(item.id)}>
              <span className="min-w-0 flex-1">
                <span className="flex items-center justify-between gap-2">
                  <strong className="truncate text-sm text-[#172033]">{item.product_name || item.device_no || t("portalExtract.customerMobile.conversation.systemHelp")}</strong>
                  <span className="flex shrink-0 items-center gap-2">{item.customer_unread_count > 0 ? <span className="flex min-w-5 items-center justify-center rounded-full bg-[#1769e0] px-1.5 text-rhd-2xs font-semibold leading-5 text-white">{Math.min(item.customer_unread_count, 99)}</span> : null}<ConversationStatus status={item.status} /></span>
                </span>
                <span className="mt-1.5 block truncate text-xs text-[#657084]">{renderCustomerConversationSummary(item.last_message_summary, t, t("portalExtract.customerMobile.conversation.noMessages"))}</span>
                <span className="mt-2 flex items-center justify-between gap-3 text-rhd-2xs text-[#939dab]">
                  <span className="flex min-w-0 items-center gap-2"><span className="truncate font-mono">{item.device_no || t("portalExtract.customerMobile.conversation.systemHelpShort")}</span>{item.current_ticket_no ? <span className="flex shrink-0 items-center gap-0.5"><ClipboardListIcon className="size-3" />{t("portalExtract.customerMobile.tabs.tickets")}</span> : null}{item.current_meeting_id ? <span className="flex shrink-0 items-center gap-0.5"><VideoIcon className="size-3" />{t("portalExtract.customerMobile.tabs.video")}</span> : null}</span>
                  <span className="flex shrink-0 items-center gap-1"><Clock3Icon className="size-3" />{formatDate(item.last_active_at, locale)}</span>
                </span>
              </span>
              <ChevronRightIcon className="size-4 shrink-0 text-[#bdc5d0]" />
            </button>
          ))}
        </div>
        {!conversationInitialLoading && visibleConversations.length < conversationTotal ? (
          <div className="px-3 pb-2 pt-3 text-center">
            <Button type="button" variant="outline" size="sm" className="h-10 w-full border-[#dce2ea] bg-white text-xs text-[#465266] shadow-none" onClick={() => void loadBase(false, undefined, conversationPage + 1, true)} disabled={loading}>
              {t("portalExtract.customerMobile.common.loadMore")}
              <span className="text-rhd-2xs font-normal text-[#939dab]">{t("portalExtract.customerMobile.common.shownCount", { shown: visibleConversations.length, total: conversationTotal })}</span>
            </Button>
          </div>
        ) : null}
        {!conversationInitialLoading && visibleConversations.length === 0 ? <div className="flex min-h-[42vh] flex-col items-center justify-center px-8 text-center"><span className="flex size-12 items-center justify-center rounded-full bg-[#e9edf2] text-[#687386]"><MessageCircleIcon className="size-5" /></span><p className="mt-4 text-sm font-semibold text-[#172033]">{filter !== "all" || query.trim() ? t("portalExtract.customerMobile.conversation.noMatched") : t("portalExtract.customerMobile.conversation.empty")}</p>{filter === "all" && !query.trim() ? <Button size="sm" className="mt-4" onClick={openCreateConversationDialog}>{readOnlyGuest ? t("portalExtract.customerMobile.common.bindDevice") : t("portalExtract.customerMobile.conversation.startConsult")}</Button> : null}</div> : null}
        {createDialog}
      </div>
    )
  }

  const closed = isConversationClosed(selected)
  const realtimeLabel = realtimeStatus === "connected"
    ? t("portalExtract.customerMobile.conversation.realtimeConnected")
    : realtimeStatus === "connecting"
      ? t("portalExtract.customerMobile.conversation.realtimeConnecting")
      : t("portalExtract.customerMobile.conversation.realtimeDisconnected")
  const activeTicketDetail = selectedTicketDetail?.id === selectedTicketId ? selectedTicketDetail : null
  const ticketNeedsFeedback = Boolean(activeTicketDetail?.can_rate && !activeTicketDetail.feedback)
  const ticketCanEnd = Boolean(activeTicketDetail?.can_confirm)
  const latestTicketProgress = activeTicketDetail?.progress?.[activeTicketDetail.progress.length - 1]
  const ticketRepairSummary = activeTicketDetail?.repair_summary || latestTicketProgress?.content || ""
  const ticketResolutionActionLabel = ticketCanEnd
    ? ticketNeedsFeedback
      ? t("portalExtract.customerMobile.ticket.rateAndEndTicket")
      : t("portalExtract.customerMobile.ticket.endTicketNow")
    : t("portalExtract.customerMobile.ticket.submitFeedback")
  const ticketResolutionAvailable = Boolean(activeTicketDetail && (ticketNeedsFeedback || ticketCanEnd))
  const ticketResolutionTitle = ticketCanEnd
    ? t("portalExtract.customerMobile.ticket.endTicketTitle")
    : t("portalExtract.customerMobile.ticket.serviceFeedback")
  const ticketResolutionHint = ticketCanEnd
    ? ticketNeedsFeedback
      ? t("portalExtract.customerMobile.ticket.endTicketHintWithFeedback")
      : t("portalExtract.customerMobile.ticket.endTicketHint")
    : ticketRepairSummary || t("portalExtract.customerMobile.ticket.noProgress")
  const ticketResolutionDialog = activeTicketDetail ? (
    <Dialog
      open={ticketResolutionOpen && ticketResolutionAvailable}
      onOpenChange={(open) => {
        if (!ticketActionLoading) setTicketResolutionOpen(open)
      }}
    >
      <DialogContent
        className="rhd-mobile-dialog bottom-0 top-auto left-1/2 max-h-[85dvh] w-full max-w-lg -translate-x-1/2 translate-y-0 overflow-hidden rounded-b-none rounded-t-2xl p-0"
        showCloseButton={!ticketActionLoading}
      >
        <div className="max-h-[85dvh] overflow-y-auto px-4 pb-[max(18px,env(safe-area-inset-bottom))] pt-4">
          <div className="mx-auto mb-3 h-1 w-10 rounded-full bg-[#d8dee8]" />
          <DialogHeader className="pr-8">
            <div className="flex items-start gap-2.5">
              <span className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-full bg-[#e9f2ff] text-[#1769e0]">
                <CheckCircle2Icon className="size-4" />
              </span>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <DialogTitle className="text-lg text-[#172033]">{ticketResolutionTitle}</DialogTitle>
                  {ticketCanEnd ? (
                    <span className="rounded-full bg-[#fff4df] px-2 py-0.5 text-rhd-2xs font-medium text-[#a75f00]">
                      {t("portalExtract.customerMobile.ticket.endAvailable")}
                    </span>
                  ) : null}
                </div>
                <p className="mt-1.5 text-xs leading-5 text-[#465266]">{ticketResolutionHint}</p>
              </div>
            </div>
          </DialogHeader>

          {ticketNeedsFeedback ? (
            <div className="mt-4 rounded-lg border border-[#edf0f4] bg-[#f8fbff] p-3">
              <div className="mb-1.5 text-rhd-xs font-medium text-[#465266]">{t("portalExtract.customerMobile.ticket.serviceFeedback")}</div>
              <div className="flex h-11 items-center gap-1" role="radiogroup" aria-label={t("portalExtract.customerMobile.ticket.rating")}>
                {[1, 2, 3, 4, 5].map((value) => (
                  <button
                    key={value}
                    type="button"
                    role="radio"
                    aria-checked={ticketRating === value}
                    aria-label={t("portalExtract.customerMobile.common.star", { value })}
                    className="flex size-10 items-center justify-center rounded-md active:bg-white disabled:opacity-60"
                    onClick={() => setTicketRating(value)}
                    disabled={ticketActionLoading}
                  >
                    <StarIcon className={cn("size-7", value <= ticketRating ? "fill-amber-400 text-amber-400" : "text-[#c4ccd8]")} />
                  </button>
                ))}
              </div>
              <Textarea
                value={ticketFeedbackComment}
                onChange={(event) => setTicketFeedbackComment(event.target.value)}
                placeholder={t("portalExtract.customerMobile.ticket.optionalFeedback")}
                className="mt-2 min-h-20 resize-none rounded-lg border-[#dce2ea] bg-white text-sm shadow-none focus-visible:bg-white"
                disabled={ticketActionLoading}
              />
            </div>
          ) : null}

          <div className="mt-4 grid grid-cols-[auto_minmax(0,1fr)] gap-2">
            <Button
              type="button"
              variant="outline"
              className="h-11 shrink-0 border-[#dce2ea] px-3 text-xs text-[#465266] shadow-none"
              onClick={() => {
                setTicketResolutionOpen(false)
                navigate("tickets", {
                  ticketNo: activeTicketDetail.ticket_no || selected.current_ticket_no,
                  fromConversationId: String(selected.id),
                })
              }}
              disabled={ticketActionLoading}
	            >
	              <ClipboardListIcon className="size-3.5" />
	              {t("portalExtract.customerMobile.ticket.viewTicket")}
	            </Button>
            <Button
              type="button"
              className="h-11 min-w-0 bg-[#1769e0] px-3 text-sm shadow-none hover:bg-[#125fcf]"
              onClick={() => void handleTicketResolutionAction()}
              disabled={ticketActionLoading || (ticketNeedsFeedback && ticketRating <= 0)}
            >
              {ticketActionLoading ? (
                <Loader2Icon className="size-3.5 animate-spin" />
              ) : ticketNeedsFeedback ? (
                <StarIcon className="size-3.5" />
              ) : (
                <CheckCircle2Icon className="size-3.5" />
              )}
              <span className="truncate">{ticketResolutionActionLabel}</span>
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  ) : null

  return (
    <div className="flex h-full min-h-[420px] flex-col overflow-hidden bg-white">
      <div className="flex shrink-0 items-center gap-2 border-b border-[#e5e9ef] bg-white px-2 pb-2.5 pt-[max(10px,env(safe-area-inset-top))]">
        <Button type="button" variant="ghost" size="icon" className="size-9 shrink-0 rounded-full" onClick={closeConversation} disabled={uploadingAsset || sending} aria-label={t("portalExtract.customerMobile.conversation.backToList")} title={t("portalExtract.customerMobile.conversation.backToList")}><ArrowLeftIcon className="size-5" /></Button>
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-2">
            <strong className="truncate text-sm text-[#172033]">{selected.product_name || selected.device_no || t("portalExtract.customerMobile.conversation.systemHelp")}</strong>
            <ConversationStatus status={selected.status} />
            <span className={cn("flex size-4 shrink-0 items-center justify-center", realtimeStatus === "connected" ? "text-[#0f9f76]" : realtimeStatus === "connecting" ? "text-[#b56b10]" : "text-[#8b96a8]")} aria-label={realtimeLabel} title={realtimeLabel}>{realtimeStatus === "connected" ? <WifiIcon className="size-3.5" /> : realtimeStatus === "connecting" ? <Loader2Icon className="size-3.5 animate-spin" /> : <WifiOffIcon className="size-3.5" />}</span>
          </div>
          <p className="mt-0.5 truncate text-rhd-2xs text-[#7b8494]">{selected.device_no || t("portalExtract.customerMobile.conversation.systemHelpShort")}{selected.current_assignee_name ? ` · ${selected.current_assignee_name}` : ""}</p>
        </div>
        <MobileLanguageSwitch />
        {!readOnlyGuest && !closed && selected.human_handoff_enabled !== false ? <Button type="button" variant="outline" size="sm" className="h-9 shrink-0 border-[#dce2ea] px-2.5 text-rhd-xs text-[#465266] shadow-none" onClick={() => void requestHuman()} disabled={handoffLoading || selected.status === "human_serving" || selected.status === "queued"}>{handoffLoading ? <Loader2Icon className="size-4 animate-spin" /> : <HeadphonesIcon className="size-4" />}{selected.status === "human_serving" ? t("portalExtract.customerMobile.conversation.humanProcessing") : selected.status === "queued" ? t("portalExtract.customerMobile.conversation.queued") : t("portalExtract.customerMobile.common.requestHuman")}</Button> : null}
      </div>

      {selected.current_ticket_no || selected.current_meeting_id || (hasDeviceConcept && (selected.device_id || selected.device_no)) || ticketResolutionAvailable ? (
        <div className="flex shrink-0 items-center gap-2 overflow-x-auto border-b border-[#e8ecf1] bg-[#f8fafc] px-3 py-2">
          {selected.current_ticket_no ? (
            <button type="button" className="flex h-8 shrink-0 items-center gap-1.5 rounded-md border border-[#dce2ea] bg-white px-2.5 text-rhd-xs font-medium text-[#465266]" onClick={() => navigate("tickets", { ticketNo: selected.current_ticket_no, fromConversationId: String(selected.id) })}>
              <ClipboardListIcon className="size-3.5 text-[#1769e0]" />
              {selected.current_ticket_no}
            </button>
          ) : null}
          {ticketResolutionAvailable ? (
            <button type="button" className="flex h-8 shrink-0 items-center gap-1.5 rounded-md border border-[#bcd5ff] bg-[#eef5ff] px-2.5 text-rhd-xs font-semibold text-[#1769e0]" onClick={() => setTicketResolutionOpen(true)}>
              {ticketNeedsFeedback ? <StarIcon className="size-3.5" /> : <CheckCircle2Icon className="size-3.5" />}
              {ticketResolutionActionLabel}
            </button>
          ) : null}
          {hasDeviceConcept && (selected.device_id || selected.device_no) ? (
            <button type="button" className="flex h-8 shrink-0 items-center gap-1.5 rounded-md border border-[#dce2ea] bg-white px-2.5 text-rhd-xs font-medium text-[#465266]" onClick={openSelectedDevice}>
              <WrenchIcon className="size-3.5 text-[#0f9f76]" />
              {t("portalExtract.customerMobile.common.deviceArchive")}
            </button>
          ) : null}
          {selected.current_meeting_id ? (
            <button type="button" className="flex h-8 shrink-0 items-center gap-1.5 rounded-md border border-[#dce2ea] bg-white px-2.5 text-rhd-xs font-medium text-[#465266]" onClick={() => navigate("video", { id: selected.current_meeting_id })}>
              <VideoIcon className="size-3.5 text-[#6557d9]" />
              {t("portalExtract.customerMobile.conversation.viewVideo")}
            </button>
          ) : null}
        </div>
      ) : null}

      <div ref={messagesViewportRef} className="min-h-0 flex-1 space-y-4 overflow-y-auto overscroll-contain bg-[#f4f6f8] px-4 py-4" onScroll={(event) => { const target = event.currentTarget; stickToBottomRef.current = target.scrollHeight - target.scrollTop - target.clientHeight < 96 }}>
        {hasOlderMessages ? <div className="text-center"><Button type="button" variant="ghost" size="sm" className="h-8 text-rhd-xs text-[#657084]" onClick={() => void loadOlderMessages()} disabled={olderLoading}>{olderLoading ? <Loader2Icon className="size-3.5 animate-spin" /> : <Clock3Icon className="size-3.5" />}{t("portalExtract.customerMobile.conversation.loadOlder")}</Button></div> : null}
        {messageInitialLoading ? <MessagesLoading /> : messages.map((message) => {
          const mine = message.senderType === "customer" || message.senderType === "user"
          if (message.senderType === "system") {
            const serviceEvent = parseConversationServiceEvent(message)
            const messageTime = message.sentAt ? formatDate(message.sentAt, locale) : ""
            if (serviceEvent) {
              return (
                <ConversationServiceEvent
                  key={message.id}
                  audience="customer"
                  density="compact"
                  event={serviceEvent}
                  hasDeviceConcept={hasDeviceConcept}
                  time={messageTime}
                  resolveVideoActionHref={resolveMobileVideoHref}
                />
              )
            }
            return (
              <div key={message.id} className="mx-auto max-w-[88%] rounded-md bg-white/70 px-3 py-1.5 text-center text-rhd-xs leading-5 text-[#7b8494] shadow-[0_1px_1px_rgba(15,23,42,0.03)] ring-1 ring-[#e7ebf0]">
                <ImMessageHTML html={renderLocalizedMobileMessageHTML(message, t)} className="[&_a]:font-medium [&_a]:text-[#1769e0] [&_a]:underline-offset-2" />
              </div>
            )
          }
          const failedClientMsgId = message.sendStatus === 5 ? message.clientMsgId?.trim() : ""
          const retryable = Boolean(failedClientMsgId && failedMediaTransfersRef.current.has(failedClientMsgId))
          const senderName = messageSenderName(message, t)
          const supplierContext = message.senderType === "partner"
            ? supplierContextFromMessagePayload(message) ?? supplierMessageContexts.get(partnerMessageCollaborationId(message))
            : undefined
          const supplierBadge = supplierContextBadge(supplierContext, locale, t)
          return (
            <div key={message.id || message.clientMsgId} className={cn("flex", mine ? "justify-end" : "justify-start")}>
              <div className={cn("max-w-[84%]", mine && "text-right")}>
                <div className={cn("mb-1 flex flex-wrap items-center gap-x-1.5 gap-y-1 px-1 text-rhd-2xs text-[#9aa3b2]", mine ? "justify-end" : "justify-start")}>
                  <span>{senderName}</span>
                  {supplierBadge ? (
                    <span className="inline-flex max-w-[13rem] items-center truncate rounded-md bg-[#eef3ff] px-1.5 py-0.5 font-medium text-[#5265c9]">
                      {supplierBadge}
                    </span>
                  ) : null}
                  <span>· {formatDate(message.sentAt, locale)}</span>
                  {message.messageType === "text" && message.content.trim() && message.id > 0 ? (
                    <button
                      type="button"
                      className="inline-flex size-5 items-center justify-center rounded text-[#7b8494] hover:bg-[#e8edf3] hover:text-[#1769e0] disabled:opacity-50"
                      onClick={() => void translateMessage(message)}
                      disabled={translatingMessageId !== null}
                      aria-label={t("customerEntryExtract.timeline.translate")}
                      title={t("customerEntryExtract.timeline.translate")}
                    >
                      {translatingMessageId === message.id ? <Loader2Icon className="size-3 animate-spin" /> : <LanguagesIcon className="size-3" />}
                    </button>
                  ) : null}
                </div>
                <div className={cn("rounded-lg px-3.5 py-2.5 text-left text-rhd-lg leading-6", mine ? "rounded-br-sm bg-[#1769e0] text-white" : "rounded-bl-sm bg-white text-[#303b4d] shadow-[0_1px_2px_rgba(15,23,42,0.08)] ring-1 ring-[#e7ebf0]")}>
                  <ImMessageHTML html={renderLocalizedMobileMessageHTML(message, t)} onImageSettled={handleMessageMediaSettled} className={mine ? "[&_a]:text-white [&_.im-media-status]:bg-white/15 [&_.im-media-status]:text-white/80 [&_.im-media-transfer]:bg-white/15" : ""} />
                  {!mine ? <KnowledgeMessageCitations payload={message.payload} /> : null}
                  {translatedMessages[message.id] ? (
                    <div className="mt-2 border-t border-current/15 pt-2 text-rhd-sm opacity-90">
                      {translatedMessages[message.id].translated_text}
                    </div>
                  ) : null}
                  {retryable && failedClientMsgId ? (
                    <div className="mt-2 flex justify-end gap-2 border-t border-white/15 pt-2">
                      <button type="button" className="inline-flex h-8 items-center gap-1 rounded-md bg-white/15 px-2.5 text-rhd-xs font-medium text-white disabled:opacity-50" onClick={() => void retryMediaMessage(failedClientMsgId)} disabled={uploadingAsset || sending}>
                        <RefreshCwIcon className="size-3.5" />
                        {t("portalExtract.customerMobile.common.retry")}
                      </button>
                      <button type="button" className="inline-flex size-8 items-center justify-center rounded-md text-white/75 disabled:opacity-50" onClick={() => dismissFailedMediaMessage(failedClientMsgId)} disabled={uploadingAsset || sending} aria-label={t("portalExtract.customerMobile.conversation.removeFailedMessage")} title={t("portalExtract.customerMobile.conversation.removeFailedMessage")}>
                        <XIcon className="size-3.5" />
                      </button>
                    </div>
                  ) : null}
                </div>
              </div>
            </div>
          )
        })}
        {!messageInitialLoading && !messageLoading && messages.length === 0 ? <p className="py-12 text-center text-xs text-[#8b96a8]">{t("portalExtract.customerMobile.conversation.noMessages")}</p> : null}
        <div ref={messagesEndRef} />
      </div>

      {readOnlyGuest ? (
        <div className="shrink-0 border-t border-[#e5e9ef] bg-white px-3 py-3">
          {notice ? <p className="mb-2 text-center text-rhd-xs text-[#657084]">{notice}</p> : null}
          <div className="flex items-center justify-between gap-3 rounded-lg bg-[#f4f6f8] px-3 py-2.5">
            <span className="flex items-center gap-2 text-xs text-[#657084]"><ShieldCheckIcon className="size-4" />{t("portalExtract.customerMobile.device.notFound")}</span>
            <Button type="button" size="sm" className="h-8" onClick={() => navigate("devices")}><WrenchIcon className="size-3.5" />{t("portalExtract.customerMobile.common.bindDevice")}</Button>
          </div>
        </div>
      ) : closed ? (
        <div className="shrink-0 border-t border-[#e5e9ef] bg-white px-3 py-3">
          {notice ? <p className="mb-2 text-center text-rhd-xs text-[#657084]">{notice}</p> : null}
          <div className="flex items-center justify-between gap-3 rounded-lg bg-[#f4f6f8] px-3 py-2.5">
            <span className="flex items-center gap-2 text-xs text-[#657084]"><CheckCircle2Icon className="size-4" />{t("portalExtract.customerMobile.conversation.closed")}</span>
            <Button type="button" size="sm" className="h-8" onClick={openCreateConversationDialog}><PlusIcon className="size-3.5" />{t("portalExtract.customerMobile.conversation.newConversationShort")}</Button>
          </div>
        </div>
      ) : (
        <form className="shrink-0 border-t border-[#e5e9ef] bg-white px-3 pb-[max(10px,env(safe-area-inset-bottom))] pt-2.5" onSubmit={submitMessage}>
          <input ref={cameraInputRef} type="file" accept="image/*" capture="environment" className="hidden" onChange={(event) => selectMedia(event, "image")} />
          <input ref={imageInputRef} type="file" accept="image/*" className="hidden" onChange={(event) => selectMedia(event, "image")} />
          <input ref={attachmentInputRef} type="file" className="hidden" onChange={(event) => selectMedia(event, "attachment")} />
          {notice ? <p className="mb-2 text-center text-rhd-xs text-[#657084]" aria-live="polite">{notice}</p> : null}
          <div className="flex items-end gap-2 rounded-[24px] border border-[#dce2ea] bg-[#f8f9fb] px-2 py-1.5 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-9 shrink-0 rounded-full text-[#657084] hover:bg-[#e8edf3] hover:text-[#172033]"
                    disabled={uploadingAsset || sending}
                    aria-label={t("portalExtract.customerMobile.conversation.sendAttachment")}
                    title={t("portalExtract.customerMobile.conversation.sendAttachment")}
                  />
                }
              >
                <PlusIcon className="size-4" />
              </DropdownMenuTrigger>
              <DropdownMenuContent side="top" align="start" className="w-44">
                <DropdownMenuItem onClick={() => attachmentInputRef.current?.click()}>
                  <PaperclipIcon className="size-4" />
                  {t("portalExtract.customerMobile.conversation.sendAttachment")}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => cameraInputRef.current?.click()}>
                  <CameraIcon className="size-4" />
                  {t("portalExtract.customerMobile.conversation.capturePhoto")}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => imageInputRef.current?.click()}>
                  <ImageIcon className="size-4" />
                  {t("portalExtract.customerMobile.conversation.pickImage")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            <Textarea
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              onKeyDown={(event) => {
                if (event.key !== "Enter" || event.shiftKey || event.nativeEvent.isComposing) return
                event.preventDefault()
                void sendDraftMessage()
              }}
              placeholder={t("portalExtract.customerMobile.conversation.inputPlaceholder")}
              className="max-h-28 min-h-9 flex-1 resize-none border-0 bg-transparent px-1 py-2 text-sm shadow-none focus-visible:ring-0 focus-visible:ring-offset-0"
              disabled={sending}
              enterKeyHint="send"
              rows={1}
            />
            <VoiceRecorderButton
              className="size-9 shrink-0 rounded-full text-[#1769e0] hover:bg-[#e8f1ff]"
              disabled={uploadingAsset || sending}
              holdToRecord
              onRecorded={sendVoiceMessage}
              onError={setNotice}
            />
          </div>
        </form>
      )}

      {ticketResolutionDialog}
      {createDialog}
    </div>
  )
}

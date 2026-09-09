"use client"

import {
  createOrMatchImConversation,
  closeImConversation,
  fetchImMessages,
  markImMessageRead,
  requestImHumanSupport,
  sendImMessage,
  uploadImAttachment,
  uploadImAudio,
  uploadImImage,
  type ImConversation,
  type ImMessage,
} from "@/lib/api/im"
import { fetchCustomerConversationsPage, translateCustomerConversationMessage } from "@/lib/api/customer-conversations"
import { bindCustomerDevice, fetchCustomerDeviceManuals, fetchCustomerDevicesPage, type BindCustomerDeviceResponse } from "@/lib/api/customer-devices"
import { fetchCustomerHome } from "@/lib/api/customer-home"
import {
  confirmCustomerMeetingJoined,
  confirmCustomerMeetingLeft,
  ingestCustomerMeetingTranscript,
  heartbeatCustomerMeeting,
  fetchCustomerMeetingJoinConfig,
  fetchCustomerMeetingsPage,
  fetchCustomerMeetingStatus,
  fetchCustomerMeetingTranscripts,
  fetchCustomerMeetingTranscriptPage,
} from "@/lib/api/customer-meetings"
import { fetchCustomerProfile, updateCustomerProfile } from "@/lib/api/customer-profile"
import { confirmCustomerTicket, fetchCustomerTicketDetail, fetchCustomerTicketsPage, reopenCustomerTicket, submitCustomerTicketFeedback } from "@/lib/api/customer-tickets"
import { ListPagination } from "@/components/list-pagination"
import { CustomerSystemIntroDocsModal } from "@/components/customer-portal/customer-system-intro-docs-modal"
import type {
  CustomerMeetingJoinConfig,
  CustomerPortalConversation,
  CustomerPortalDevice,
  CustomerPortalHome,
  CustomerPortalManualFile,
  CustomerPortalMeeting,
  CustomerPortalProfile,
  CustomerPortalTicket,
} from "@/lib/api/customer-portal-types"
import type { ConversationMessageTranslationDTO, MeetingTranscriptSegment } from "@/lib/api/types"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/components/auth-provider"
import { useI18n } from "@/i18n/provider"
import { useAppLocale } from "@/i18n/provider"
import { PageHeader as SharedPageHeader } from "@/components/layout/page-header"
import { ErrorState } from "@/components/shared/error-states"
import { CustomerConversationTimeline } from "@/components/remote-helpdesk/customer-conversation-timeline"
import { CustomerMessageEditor } from "@/components/support-chat/customer-message-editor"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Skeleton } from "@/components/ui/skeleton"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import {
  buildMediaTransferPayload,
  createPendingMessageId,
} from "@/lib/im-message"
import {
  replaceLoadedImMessagesPreservingLocalTransfers,
  upsertImMessageReplacingClientMessage,
} from "@/lib/im-message-merge"
import { createRealtimeConnectionManager } from "@/lib/realtime-connection"
import { createAccessTokenWebSocket, createWebSocketBaseUrl } from "@/lib/api/websocket"
import { selectInitialCustomerConversationId } from "@/lib/api/customer-portal-runtime"
import { customerTicketProgressContent, customerTicketProgressLabel } from "@/lib/customer-ticket-progress-i18n"
import { renderCustomerConversationSummary } from "@/lib/customer-conversation-summary-i18n"
import {
  TYPING_IDLE_TIMEOUT_MS,
  mergeRealtimePresenceActor,
  updateRealtimeTypingState,
  expireRealtimeTypingActor,
  type RealtimePresencePayload,
  type RealtimePresenceSnapshotPayload,
  type RealtimeTypingPayload,
  type RealtimeTypingState,
} from "@/lib/im-realtime-state"
import { MeetingLobby } from "@/components/meeting/meeting-lobby"
import { MeetingLiveRoom } from "@/components/meeting/meeting-live-room"
import { MeetingParticipantList } from "@/components/meeting/meeting-participant-list"
import { Badge } from "@/components/ui/badge"
import { FilterTabs, IconButton, SearchField, StandardModal, UnderlineTabs, type RailopsTabItem } from "@railops/ui"
import {
  ArrowLeftIcon,
  BookOpenIcon,
  CalendarClockIcon,
  CheckCircle2Icon,
  ChevronRightIcon,
  CircleUserRoundIcon,
  CpuIcon,
  ExternalLinkIcon,
  FileTextIcon,
  HeadphonesIcon,
  MessageSquareTextIcon,
  MessagesSquareIcon,
  PackageSearchIcon,
  PlusIcon,
  RefreshCwIcon,
  ShieldCheckIcon,
  TicketCheckIcon,
  VideoIcon,
  UsersRoundIcon,
  WrenchIcon,
  StarIcon,
  RotateCcwIcon,
  Loader2Icon,
} from "lucide-react"
import { usePathname, useRouter, useSearchParams } from "next/navigation"
import { useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from "react"
import { toast } from "sonner"

type I18nT = ReturnType<typeof useI18n>

const mobileCustomerStateByPath: Record<string, string> = {
  "/customer/chat": "chat",
  "/customer/tickets": "tickets",
  "/customer/meeting": "video",
  "/customer/devices": "devices",
  "/customer/my": "my",
}

function adaptCustomerPortalPath(destination: string) {
  if (typeof window === "undefined" || window.location.pathname !== "/mobile") {
    return destination
  }
  const target = new URL(destination, window.location.origin)
  const state = mobileCustomerStateByPath[target.pathname]
  if (!state) return destination

  const params = new URLSearchParams(target.search)
  const serviceCode = new URLSearchParams(window.location.search).get("serviceCode")
  if (serviceCode) params.set("serviceCode", serviceCode)
  params.set("state", state)
  return `/mobile?${params.toString()}`
}

function useCustomerPortalRouter() {
  const router = useRouter()
  const pathname = usePathname()
  const mobile = pathname === "/mobile"
  return useMemo(() => ({
    push: (destination: string) => router.push(mobile ? adaptCustomerPortalPath(destination) : destination),
    replace: (destination: string) => router.replace(mobile ? adaptCustomerPortalPath(destination) : destination),
  }), [mobile, router])
}

function formatDateTime(value?: string, locale = "zh-CN") {
  if (!value) {
    return "—"
  }
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }
  return new Intl.DateTimeFormat(locale, {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(parsed)
}

function compactCustomerIdentifier(value: string) {
  const identifier = value.trim()
  if (identifier.length <= 22) return identifier
  return `${identifier.slice(0, 12)}...${identifier.slice(-6)}`
}

const customerTranslationLanguageOptions = [
  { value: "zh-CN", label: "中文" },
  { value: "en", label: "English" },
  { value: "de", label: "Deutsch" },
  { value: "es", label: "Español" },
  { value: "fr", label: "Français" },
  { value: "pt", label: "Português" },
  { value: "ja", label: "日本語" },
  { value: "ko", label: "한국어" },
]

function customerTranslationLanguageLabel(language: string) {
  return customerTranslationLanguageOptions.find((item) => item.value === language)?.label || language
}

function detectCustomerMessageTranslationLanguage(content: string) {
  const hanCount = (content.match(/\p{Script=Han}/gu) || []).length
  const latinCount = (content.match(/\p{Script=Latin}/gu) || []).length
  if (hanCount > 0 && hanCount * 2 >= latinCount) {
    return "zh-CN"
  }
  if (hanCount === 0 && latinCount >= 2) {
    const englishMarkers = new Set([
      "a", "an", "and", "are", "can", "could", "device", "equipment", "hello", "help", "how",
      "i", "is", "it", "machine", "my", "please", "reset", "restart", "safe", "should", "support",
      "the", "this", "to", "warranty", "what", "when", "where", "why", "you",
    ])
    const words = content.toLowerCase().match(/\p{Letter}+/gu) || []
    if (words.some((word) => englishMarkers.has(word))) {
      return "en"
    }
  }
  return ""
}

function resolveCustomerMessageTranslationTarget(content: string, selectedTarget: string) {
  const sourceLanguage = detectCustomerMessageTranslationLanguage(content)
  if (sourceLanguage !== selectedTarget) {
    return selectedTarget
  }
  if (sourceLanguage === "zh-CN") {
    return "en"
  }
  if (sourceLanguage === "en") {
    return "zh-CN"
  }
  return selectedTarget
}

function buildPendingPortalMediaMessage(input: {
  conversationId: number
  clientMsgId: string
  messageType: "audio" | "attachment"
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

function markPendingPortalMediaFailed(message: ImMessage, file: File, durationSeconds?: number) {
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

function mapCreatedCustomerConversationStatus(status: number | string | undefined) {
  if (typeof status === "string") {
    return status
  }
  switch (status) {
    case 1:
      return "ai_serving"
    case 2:
      return "queued"
    case 3:
      return "human_serving"
    case 4:
      return "closed"
    default:
      return "waiting_customer"
  }
}

function buildCreatedCustomerPortalConversation(
  conversation: ImConversation,
  device?: CustomerPortalDevice,
): CustomerPortalConversation {
  const now = new Date().toISOString()
  const lastActiveAt = conversation.lastActiveAt || conversation.lastMessageAt || now
  return {
    id: conversation.id,
    status: mapCreatedCustomerConversationStatus(conversation.status),
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

function upsertCustomerPortalConversation(
  conversations: CustomerPortalConversation[],
  next: CustomerPortalConversation,
) {
  const existingIndex = conversations.findIndex((item) => item.id === next.id)
  if (existingIndex < 0) {
    return [next, ...conversations]
  }
  return conversations.map((item) => item.id === next.id ? { ...next, ...item } : item)
}

function buildCustomerPortalDeviceFromBinding(binding: BindCustomerDeviceResponse): CustomerPortalDevice {
  return {
    id: binding.deviceId,
    device_no: binding.deviceNo || "",
    serial_no: "",
    product_name: binding.productName || "",
    product_code: "",
    model_name: "",
    region_code: "",
    status: "active",
    last_service_at: "",
    installed_at: "",
    warranty_end_at: "",
    manual_count: 0,
    repair_history_count: 0,
    open_ticket_count: 0,
    conversation_count: 0,
  }
}

function upsertCustomerPortalDevice(
  devices: CustomerPortalDevice[],
  next: CustomerPortalDevice,
) {
  if (next.id <= 0) {
    return devices
  }
  if (!devices.some((item) => item.id === next.id)) {
    return [next, ...devices]
  }
  return devices.map((item) => item.id === next.id ? { ...next, ...item } : item)
}

function portalStatusTone(status: string) {
  switch (status) {
    case "active":
    case "online":
    case "human_serving":
      return "bg-[var(--railops-primary-bg)] text-[var(--railops-primary-hover)] border-[#bfdbfe]"
    case "done":
    case "finished":
    case "closed":
      return "bg-[var(--railops-surface-muted)] text-[var(--railops-text-secondary)] border-[var(--railops-border-light)]"
    case "queued":
    case "pending_acceptance":
    case "pending_dispatch":
    case "pending_assignee_accept":
    case "waiting":
    case "ai_serving":
    case "maintenance":
    case "action_required":
    case "pending_confirmation":
      return "bg-[var(--railops-warning-bg)] text-[#b45309] border-[#fde68a]"
    case "offline":
    case "cancelled":
      return "bg-[var(--railops-surface-muted)] text-[var(--railops-text-secondary)] border-[var(--railops-border-light)]"
    default:
      return "bg-[var(--railops-primary-bg)] text-[var(--railops-primary-hover)] border-[#bfdbfe]"
  }
}

function customerPortalStatusLabel(t: I18nT, status: string) {
  const labels: Record<string, string> = {
    active: t("customerChat.statusActive"),
    done: t("customerChat.statusDone"),
    finished: t("customerChat.statusFinished"),
    online: t("customerChat.statusOnline"),
    human_serving: t("customerChat.statusHumanServing"),
    queued: t("customerChat.statusQueued"),
    pending_acceptance: t("customerChat.statusPendingAcceptance"),
    pending_dispatch: t("customerChat.statusPendingDispatch"),
    pending_assignee_accept: t("customerChat.statusPendingAssigneeAccept"),
    waiting: t("customerChat.statusWaiting"),
    waiting_customer: t("customerChat.statusWaitingCustomer"),
    ai_serving: t("customerChat.statusAiServing"),
    maintenance: t("customerChat.statusMaintenance"),
    closed: t("customerChat.statusClosed"),
    offline: t("customerChat.statusOffline"),
    submitted: t("customerChat.statusSubmitted"),
    accepted: t("customerChat.statusAccepted"),
    processing: t("customerChat.statusProcessing"),
    action_required: t("customerChat.statusActionRequired"),
    pending_confirmation: t("customerChat.statusPendingConfirmation"),
    reviewing: t("customerChat.statusReviewing"),
    reopened: t("customerChat.statusReopened"),
    cancelled: t("customerChat.statusCancelled"),
  }
  return labels[status] || status || t("customerChat.statusUnknown")
}

function StatusBadge({ status }: { status: string }) {
  const t = useI18n()
  return (
    <span className={cn("inline-flex shrink-0 whitespace-nowrap rounded-full border px-2 py-0.5 text-rhd-xs font-semibold leading-4", portalStatusTone(status))}>
      {customerPortalStatusLabel(t, status)}
    </span>
  )
}

function warrantySummary(t: I18nT, value?: string) {
  if (!value) return t("customerChat.warrantyPending")
  const warrantyEnd = new Date(value)
  if (Number.isNaN(warrantyEnd.getTime())) return t("customerChat.warrantyPending")
  const remainingDays = Math.ceil((warrantyEnd.getTime() - Date.now()) / 86_400_000)
  return remainingDays >= 0
    ? t("customerChat.warrantyRemaining", { days: remainingDays })
    : t("customerChat.warrantyExpired")
}

function canJoinCustomerMeeting(status?: string) {
  return status === "waiting" || status === "scheduled" || status === "active"
}

function normalizeCustomerConversationId(value: unknown) {
  const id = Number(value)
  return Number.isSafeInteger(id) && id > 0 ? id : 0
}

function buildCustomerChatReturnPath(conversationId: unknown) {
  const id = normalizeCustomerConversationId(conversationId)
  return id > 0 ? `/customer/chat?conversationId=${encodeURIComponent(String(id))}` : "/customer/chat"
}

function buildCustomerTicketPath(ticketId: unknown, fromConversationId?: unknown) {
  const params = new URLSearchParams()
  const normalizedTicketId = Number(ticketId)
  if (Number.isSafeInteger(normalizedTicketId) && normalizedTicketId > 0) {
    params.set("ticketId", String(normalizedTicketId))
  }
  const returnConversationId = normalizeCustomerConversationId(fromConversationId)
  if (returnConversationId > 0) {
    params.set("fromConversationId", String(returnConversationId))
  }
  const query = params.toString()
  return `/customer/tickets${query ? `?${query}` : ""}`
}

function buildCustomerDevicePath(deviceId: unknown, fromConversationId?: unknown, tab?: string) {
  const params = new URLSearchParams()
  const normalizedDeviceId = Number(deviceId)
  if (Number.isSafeInteger(normalizedDeviceId) && normalizedDeviceId > 0) {
    params.set("deviceId", String(normalizedDeviceId))
  }
  const returnConversationId = normalizeCustomerConversationId(fromConversationId)
  if (returnConversationId > 0) {
    params.set("fromConversationId", String(returnConversationId))
  }
  if (tab && tab !== "overview") {
    params.set("tab", tab)
  }
  const query = params.toString()
  return `/customer/devices${query ? `?${query}` : ""}`
}

function customerConversationNextAction(
  t: I18nT,
  conversation: CustomerPortalConversation,
  ticket: CustomerPortalTicket | null,
) {
  if (conversation.status === "closed") {
    return t("customerChat.nextActionClosed")
  }
  if (canJoinCustomerMeeting(conversation.current_meeting_status)) {
    return conversation.current_meeting_status === "active"
      ? t("customerChat.nextActionVideoActive")
      : t("customerChat.nextActionVideoPending")
  }
  if (ticket?.can_confirm) {
    return t("customerChat.nextActionTicketConfirm")
  }
  if (ticket?.status === "action_required") {
    return t("customerChat.nextActionTicketAction")
  }
  if (conversation.status === "queued") {
    return t("customerChat.nextActionQueued")
  }
  if (conversation.status === "human_serving") {
    return t("customerChat.nextActionHuman")
  }
  return t("customerChat.nextActionDefault")
}

function PageHeader({
  title,
  action,
  compact = false,
}: {
  title: string
  action?: ReactNode
  compact?: boolean
}) {
  return (
    <SharedPageHeader
      actions={action}
      className={compact ? "gap-2 pb-2" : undefined}
      title={title}
    />
  )
}

function MetricsStrip({ home, hasDeviceConcept = true }: { home: CustomerPortalHome | null; hasDeviceConcept?: boolean }) {
  const metrics = home?.metrics.filter((metric) => hasDeviceConcept || metric.key !== "devices") ?? []
  const columnClass = hasDeviceConcept ? "lg:grid-cols-4" : "lg:grid-cols-3"
  if (!home) {
    return (
      <div className={cn("rhd-railops-customer-metrics grid grid-cols-2 overflow-hidden rounded-md bg-[var(--railops-surface)] shadow-[var(--railops-card-shadow)]", columnClass)}>
        {Array.from({ length: hasDeviceConcept ? 4 : 3 }).map((_, index) => (
          <div key={index} className={cn(
            "border-b border-r border-[var(--railops-border-light)] px-3 py-2.5 lg:border-b-0 lg:last:border-r-0",
            index % 2 === 1 && "border-r-0 lg:border-r",
            index >= 2 && "border-b-0",
          )}>
            <Skeleton className="h-10 rounded-md" />
          </div>
        ))}
      </div>
    )
  }
  return (
    <div className={cn("rhd-railops-customer-metrics grid grid-cols-2 overflow-hidden rounded-md bg-[var(--railops-surface)] shadow-[var(--railops-card-shadow)]", columnClass)}>
      {metrics.map((metric, index) => (
        <div key={metric.key} className={cn(
          "flex min-w-0 items-center gap-3 border-b border-r border-[var(--railops-border-light)] px-3 py-2.5 lg:border-b-0 lg:last:border-r-0",
          index % 2 === 1 && "border-r-0 lg:border-r",
          index >= metrics.length - 2 && "border-b-0",
        )}>
          <div className="min-w-10 text-base font-semibold text-[var(--railops-text)]">{metric.value}</div>
          <div className="min-w-0">
            <div className="truncate text-xs font-medium text-[var(--railops-text)]">{metric.label}</div>
            <div className="mt-0.5 truncate text-rhd-xs text-[var(--railops-text-secondary)]">{metric.meta}</div>
          </div>
        </div>
      ))}
    </div>
  )
}

function EmptyCard({
  icon,
  title,
  action,
}: {
  icon: ReactNode
  title: string
  action?: ReactNode
}) {
  return (
    <div className="flex min-h-40 flex-col items-center justify-center gap-3 rounded-md border border-dashed border-[var(--railops-border)] bg-[var(--railops-surface-muted)] p-5 text-center">
      <div className="grid size-9 place-items-center rounded-md bg-[var(--railops-surface)] text-[var(--railops-text-secondary)] shadow-[var(--railops-card-shadow)]">{icon}</div>
      <div className="space-y-1">
        <div className="text-xs font-semibold text-[var(--railops-text)]">{title}</div>
      </div>
      {action}
    </div>
  )
}

type CustomerDeviceBindFormProps = {
  formId: string
  fieldIdPrefix: string
  onBound?: (binding: BindCustomerDeviceResponse) => Promise<void> | void
  successMessage?: string
  submitLabel?: string
  submittingLabel?: string
  autoFocus?: boolean
  onSuccess?: () => void
  onSubmittingChange?: (submitting: boolean) => void
  onServiceCodeChange?: (serviceCode: string) => void
}

function CustomerDeviceBindForm({
  formId,
  fieldIdPrefix,
  onBound,
  successMessage,
  submitLabel,
  submittingLabel,
  autoFocus = false,
  onSuccess,
  onSubmittingChange,
  onServiceCodeChange,
}: CustomerDeviceBindFormProps) {
  const t = useI18n()
  const [serviceCode, setServiceCode] = useState("")
  const [binding, setBinding] = useState(false)
  const serviceCodeId = `${fieldIdPrefix}-service-code`
  const formSuccessMessage = successMessage ?? t("customerOnboarding.dialogSuccess")

  async function submitBinding(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!serviceCode.trim() || binding) return
    setBinding(true)
    try {
      const result = await bindCustomerDevice({
        serviceCode: serviceCode.trim(),
      })
      await onBound?.(result)
      toast.success(formSuccessMessage)
      setServiceCode("")
      onSuccess?.()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("customerOnboarding.dialogError"))
    } finally {
      setBinding(false)
    }
  }

  function handleServiceCodeChange(value: string) {
    setServiceCode(value)
    onServiceCodeChange?.(value)
  }

  useEffect(() => {
    onSubmittingChange?.(binding)
  }, [binding, onSubmittingChange])

  return (
    <form id={formId} className="grid gap-3" onSubmit={submitBinding}>
      <label className="text-sm font-medium text-foreground" htmlFor={serviceCodeId}>{t("customerOnboarding.dialogServiceCodeLabel")}</label>
      <Input id={serviceCodeId} value={serviceCode} onChange={(event) => handleServiceCodeChange(event.target.value)} autoFocus={autoFocus} className="uppercase" placeholder={t("customerOnboarding.dialogServiceCodePlaceholder")} />
    </form>
  )
}

type CustomerDeviceBindDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  formId: string
  fieldIdPrefix: string
  onBound?: (binding: BindCustomerDeviceResponse) => Promise<void> | void
  successMessage?: string
  submitLabel?: string
  submittingLabel?: string
}

function CustomerDeviceBindDialog({
  open,
  onOpenChange,
  formId,
  fieldIdPrefix,
  onBound,
  successMessage,
  submitLabel,
  submittingLabel,
}: CustomerDeviceBindDialogProps) {
  const t = useI18n()
  const [binding, setBinding] = useState(false)
  const [canSubmit, setCanSubmit] = useState(false)

  return (
    <StandardModal
      open={open}
      onCancel={() => {
        if (!binding) onOpenChange(false)
      }}
      width={448}
      closeIcon={binding ? null : undefined}
      title={t("customerOnboarding.dialogTitle")}
      footer={(
        <>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={binding}>{t("common.close")}</Button>
          <Button type="submit" form={formId} disabled={binding || !canSubmit}>
            {binding ? <Loader2Icon className="mr-2 size-4 animate-spin" /> : null}
            {binding ? (submittingLabel ?? t("customerOnboarding.dialogSubmitting")) : (submitLabel ?? t("customerOnboarding.dialogSubmit"))}
          </Button>
        </>
      )}
    >
      <CustomerDeviceBindForm
        formId={formId}
        fieldIdPrefix={fieldIdPrefix}
        onBound={onBound}
        successMessage={successMessage}
        autoFocus
        onSuccess={() => onOpenChange(false)}
        onSubmittingChange={setBinding}
        onServiceCodeChange={(value) => setCanSubmit(Boolean(value.trim()))}
      />
    </StandardModal>
  )
}

export function CustomerChatPortalPage() {
  const { session } = useAuth()
  const t = useI18n()
  const { locale } = useAppLocale()
  const router = useCustomerPortalRouter()
  const searchParams = useSearchParams()
  const conversationQueryParam = searchParams?.get("conversationId") || ""
  const requestedDeviceIdParam = searchParams?.get("deviceId") || ""
  const newConversationParam = searchParams?.get("new") || ""
  const [conversations, setConversations] = useState<CustomerPortalConversation[]>([])
  const [devices, setDevices] = useState<CustomerPortalDevice[]>([])
  const [conversationTotal, setConversationTotal] = useState(0)
  const [conversationPage, setConversationPage] = useState(1)
  const [conversationLimit, setConversationLimit] = useState(20)
  const [selectedConversationDetail, setSelectedConversationDetail] = useState<CustomerPortalConversation | null>(null)
  const [selectedConversationLoading, setSelectedConversationLoading] = useState(false)
  const [selectedTicketDetail, setSelectedTicketDetail] = useState<CustomerPortalTicket | null>(null)
  const [loading, setLoading] = useState(true)
  const [devicesLoading, setDevicesLoading] = useState(true)
  const [hasLoadedConversations, setHasLoadedConversations] = useState(false)
  const [conversationLoadError, setConversationLoadError] = useState("")
  const [deviceLoadError, setDeviceLoadError] = useState("")
  const [selectedConversationId, setSelectedConversationId] = useState(0)
  const [createDeviceId, setCreateDeviceId] = useState("")
  const [createOpen, setCreateOpen] = useState(false)
  const [createDialogView, setCreateDialogView] = useState<"list" | "bind">("list")
  const [docsOpen, setDocsOpen] = useState(false)
  const [createDeviceQuery, setCreateDeviceQuery] = useState("")
  const [bindSubmitting, setBindSubmitting] = useState(false)
  const [bindCanSubmit, setBindCanSubmit] = useState(false)
  const [profile, setProfile] = useState<CustomerPortalProfile | null>(null)
  const [conversationFilter, setConversationFilter] = useState<"all" | "active" | "closed">("all")
  const [conversationSearchInput, setConversationSearchInput] = useState("")
  const [conversationQuery, setConversationQuery] = useState("")
  const [mobileConversationListOpen, setMobileConversationListOpen] = useState(() => !conversationQueryParam)
  const [messages, setMessages] = useState<ImMessage[]>([])
  const [messageLoading, setMessageLoading] = useState(false)
  const [creatingConversation, setCreatingConversation] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [uploadingAsset, setUploadingAsset] = useState(false)
  const [conversationAction, setConversationAction] = useState<"human" | "close" | "">("")
  const [realtimeConnected, setRealtimeConnected] = useState(false)
  const [presence, setPresence] = useState<Record<string, RealtimePresencePayload>>({})
  const [remoteTyping, setRemoteTyping] = useState<RealtimeTypingState>({})
  const [translationTargetLanguage, setTranslationTargetLanguage] = useState("zh-CN")
  const [translatedMessages, setTranslatedMessages] = useState<Record<number, ConversationMessageTranslationDTO>>({})
  const [translatingMessageId, setTranslatingMessageId] = useState<number | null>(null)
  const realtimeSocketRef = useRef<WebSocket | null>(null)
  const typingExpiryRef = useRef(new Map<string, number>())
  const messagesEndRef = useRef<HTMLDivElement | null>(null)
  const autoCreateDeviceRequestRef = useRef("")
  const conversationListRequestRef = useRef(0)
  const messageListRequestRef = useRef(0)
  const selectedConversationIdRef = useRef(0)
  const loadBaseDataRef = useRef<(silent?: boolean, fallbackConversation?: CustomerPortalConversation, refreshContext?: boolean) => Promise<CustomerPortalConversation[]>>(async () => [])
  const hasDeviceConcept = session?.featureFlags?.device !== false

  const selectedConversationInPage = conversations.find((item) => item.id === selectedConversationId) ?? null
  const selectedConversation = selectedConversationDetail ?? selectedConversationInPage
  const selectedDevice = hasDeviceConcept && selectedConversation
    ? devices.find((item) => item.id === selectedConversation.device_id) ?? null
    : null
  const selectedTicket = selectedConversation
    ? selectedTicketDetail
    : null
  const selectedTicketId = selectedTicket?.id ?? selectedConversation?.current_ticket_id ?? 0
  const initialConversationLoad = !hasLoadedConversations && loading
  const conversationRefreshing = loading && hasLoadedConversations
  const selectedConversationDetailLoading = selectedConversationLoading && selectedConversationId > 0
  const tenantAIEnabled = session?.featureFlags?.ai !== false
  const isGeneralConsultation = Boolean(selectedConversation && (!hasDeviceConcept || selectedConversation.device_id <= 0 && !selectedConversation.current_ticket_no))
  // Guests can view existing conversations, tickets, and video records, but need
  // a bound device or tenant customer relation before asking for support.
  const isGuest = hasDeviceConcept && (profile
    ? profile.bound_device_count === 0 && !profile.company_name.trim()
    : devicesLoading || devices.length === 0)
  const closedTimelineTitle = selectedConversation?.current_ticket_no
    ? t("customerChat.closedTimelineTicketTitle", { ticketNo: selectedConversation.current_ticket_no })
    : isGeneralConsultation
      ? t("customerChat.closedTimelineGeneralTitle")
      : t("customerChat.closedTimelineDeviceTitle")
  const closedTimelineDescription = selectedConversation?.current_ticket_no
    ? selectedTicket?.repair_summary || t("customerChat.closedTimelineTicketDescription")
    : isGeneralConsultation
      ? t("customerChat.closedTimelineGeneralDescription")
      : t("customerChat.closedTimelineDeviceDescription")
  const closedBannerText = selectedConversation?.current_ticket_no
    ? t("customerChat.closedBannerTicket")
    : isGeneralConsultation
      ? t("customerChat.closedBannerGeneral")
      : t("customerChat.closedBannerDevice")

  async function loadCustomerChatDevices(silent = false) {
    if (!hasDeviceConcept) {
      setDevices([])
      setCreateDeviceId("")
      setDeviceLoadError("")
      setDevicesLoading(false)
      return
    }
    if (!silent) {
      setDevicesLoading(true)
      setDeviceLoadError("")
    }
    try {
      const devicePageData = await fetchCustomerDevicesPage({ page: 1, limit: 100 })
      let deviceData = devicePageData.results ?? []
      const queryDeviceId = Number(requestedDeviceIdParam || "0")
      if (queryDeviceId > 0 && !deviceData.some((item) => item.id === queryDeviceId)) {
        const requestedDevicePage = await fetchCustomerDevicesPage({ deviceId: queryDeviceId, limit: 1 })
        const requestedDevice = requestedDevicePage.results?.[0]
        if (requestedDevice) {
          deviceData = upsertCustomerPortalDevice(deviceData, requestedDevice)
        }
      }
      setDevices(deviceData)
      setDeviceLoadError("")
      const requestedDevice = deviceData.find((item) => item.id === queryDeviceId)
      if (requestedDevice) {
        setCreateDeviceId(String(requestedDevice.id))
      }
      if (newConversationParam === "1" && queryDeviceId > 0 && !requestedDevice) {
        setCreateOpen(true)
      }
    } catch (error) {
      setDeviceLoadError(error instanceof Error ? error.message : t("customerDevices.loadFailed"))
    } finally {
      if (!silent) {
        setDevicesLoading(false)
      }
    }
  }

  async function loadBaseData(silent = false, fallbackConversation?: CustomerPortalConversation, refreshContext = true) {
    const requestId = ++conversationListRequestRef.current
    if (!silent) {
      setLoading(true)
      setConversationLoadError("")
    }
    if (refreshContext) {
      if (hasDeviceConcept) void loadCustomerChatDevices(silent)
      void fetchCustomerProfile()
        .then(setProfile)
        .catch(() => undefined)
    }
    try {
      const conversationPageData = await fetchCustomerConversationsPage({
        page: conversationPage,
        limit: conversationLimit,
        locale,
        keyword: conversationQuery.trim(),
        filter: conversationFilter === "all" ? undefined : conversationFilter,
      })
      const conversationData = conversationPageData.results ?? []
      const nextConversations = fallbackConversation
        ? upsertCustomerPortalConversation(conversationData, fallbackConversation)
        : conversationData
      if (requestId !== conversationListRequestRef.current) return []
      setConversations(nextConversations)
	  const refreshedSelected = nextConversations.find((item) => item.id === selectedConversationIdRef.current)
	  if (refreshedSelected) {
	    setSelectedConversationDetail(refreshedSelected)
	  }
      setConversationTotal(conversationPageData.page?.total ?? nextConversations.length)
      const queryId = Number(conversationQueryParam || "0")
      const initial = queryId > 0
        ? queryId
        : selectInitialCustomerConversationId(nextConversations, 0)
      setSelectedConversationId((current) => current > 0 ? current : initial)
      setHasLoadedConversations(true)
      return nextConversations
    } catch (error) {
      if (requestId !== conversationListRequestRef.current) return []
      const message = error instanceof Error ? error.message : t("customerChat.loadFailed")
      setConversationLoadError(message)
      if (!silent) {
        toast.error(message)
      }
      return []
    } finally {
      if (requestId === conversationListRequestRef.current) {
        setLoading(false)
      }
	    }
		  }

		  async function loadMessages(conversationId: number, silent = false) {
    if (!conversationId) {
      setMessages([])
      return
    }
    const requestId = ++messageListRequestRef.current
    if (!silent) {
      setMessageLoading(true)
    }
    try {
      const page = await fetchImMessages({ conversationId, limit: 50 })
      if (requestId !== messageListRequestRef.current || selectedConversationIdRef.current !== conversationId) {
        return
      }
      const incoming = page.results ?? []
      setMessages((current) => replaceLoadedImMessagesPreservingLocalTransfers(
        current.filter((message) => message.conversationId === conversationId),
        incoming,
      ))
      await markImMessageRead(conversationId)
    } catch (error) {
      if (!silent && requestId === messageListRequestRef.current && selectedConversationIdRef.current === conversationId) {
        toast.error(error instanceof Error ? error.message : t("customerChat.messageLoadFailed"))
      }
    } finally {
      if (!silent && requestId === messageListRequestRef.current) {
        setMessageLoading(false)
      }
    }
  }

  function syncCustomerConversationUrl(conversationId: number) {
    if (conversationId <= 0 || typeof window === "undefined") {
      return
    }
    const nextParams = new URLSearchParams(window.location.search)
    nextParams.set("conversationId", String(conversationId))
    nextParams.delete("new")
    const query = nextParams.toString()
    const nextUrl = `/customer/chat${query ? `?${query}` : ""}`
    window.history.replaceState(window.history.state, "", adaptCustomerPortalPath(nextUrl))
  }

  useEffect(() => {
    loadBaseDataRef.current = loadBaseData
  })

  useEffect(() => {
    loadBaseData(false, undefined, false)
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [locale, t, conversationPage, conversationLimit, conversationFilter, conversationQuery])

  useEffect(() => {
    void loadCustomerChatDevices()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [t, newConversationParam, requestedDeviceIdParam, hasDeviceConcept])

  useEffect(() => {
    void fetchCustomerProfile()
      .then(setProfile)
      .catch(() => undefined)
  }, [])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setConversationPage(1)
      setConversationQuery(conversationSearchInput)
    }, 250)
    return () => window.clearTimeout(timer)
  }, [conversationSearchInput])

  useEffect(() => {
    if (selectedConversationId <= 0) {
      setSelectedConversationDetail(null)
      setSelectedConversationLoading(false)
      return
    }
    setSelectedConversationDetail((current) => current?.id === selectedConversationId ? current : null)
    let cancelled = false
    setSelectedConversationLoading(true)
    void fetchCustomerConversationsPage({ conversationId: selectedConversationId, limit: 1, locale })
      .then((page) => {
        if (!cancelled) {
          setSelectedConversationDetail(page.results?.[0] ?? null)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setSelectedConversationDetail(null)
        }
      })
      .finally(() => {
        if (!cancelled) {
          setSelectedConversationLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [locale, selectedConversationId])

  useEffect(() => {
    const ticketId = selectedConversation?.current_ticket_id ?? 0
    if (ticketId <= 0) {
      setSelectedTicketDetail(null)
      return
    }
    if (selectedTicketDetail?.id !== ticketId) {
      setSelectedTicketDetail(null)
    }
    let cancelled = false
    void fetchCustomerTicketsPage({ ticketId, limit: 1 })
      .then((page) => {
        if (!cancelled) {
          setSelectedTicketDetail(page.results?.[0] ?? null)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setSelectedTicketDetail(null)
        }
      })
    return () => {
      cancelled = true
    }
  }, [selectedConversation?.current_ticket_id, selectedConversation?.last_active_at, selectedConversationId])

  useEffect(() => {
    if (!hasDeviceConcept) {
      autoCreateDeviceRequestRef.current = ""
      return
    }
    const deviceId = Number(requestedDeviceIdParam || "0")
    if (newConversationParam !== "1") {
      autoCreateDeviceRequestRef.current = ""
      return
    }
    if (deviceId <= 0 || creatingConversation || !hasLoadedConversations) {
      return
    }
    const requestKey = `${deviceId}:${newConversationParam}`
    if (autoCreateDeviceRequestRef.current === requestKey) {
      return
    }
    const requestedDevice = devices.find((item) => item.id === deviceId)
    if (!requestedDevice) {
      return
    }
    autoCreateDeviceRequestRef.current = requestKey
    setCreateDeviceId(String(deviceId))
    setCreateOpen(false)
    setCreatingConversation(true)
    void createOrMatchImConversation({ deviceId, forceNew: true })
      .then((created) => activateCreatedConversation(created, requestedDevice))
      .then(() => {
        toast.success(t("customerChat.deviceConversationCreated"))
      })
      .catch((error) => {
        setCreateOpen(true)
        toast.error(error instanceof Error ? error.message : t("customerChat.deviceConversationCreateFailed"))
      })
      .finally(() => {
        setCreatingConversation(false)
      })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [creatingConversation, devices, hasDeviceConcept, hasLoadedConversations, newConversationParam, requestedDeviceIdParam])

  useEffect(() => {
    if (!selectedConversationId) {
      return
    }
    selectedConversationIdRef.current = selectedConversationId
    setTranslatedMessages({})
    setTranslatingMessageId(null)
    syncCustomerConversationUrl(selectedConversationId)
    loadMessages(selectedConversationId)
  }, [selectedConversationId])

  useEffect(() => {
    const queryId = Number(conversationQueryParam || "0")
    if (queryId <= 0 || queryId === selectedConversationId) {
      return
    }
    const currentURLId = typeof window === "undefined"
      ? 0
      : Number(new URLSearchParams(window.location.search).get("conversationId") || "0")
    if (currentURLId === selectedConversationId) {
      return
    }
    selectedConversationIdRef.current = queryId
    messageListRequestRef.current += 1
    setSelectedConversationId(queryId)
  }, [conversationQueryParam, selectedConversationId])

  useEffect(() => {
    if (!selectedConversationId || messages.length === 0) {
      return
    }
    messagesEndRef.current?.scrollIntoView({
      behavior: messageLoading ? "auto" : "smooth",
      block: "end",
    })
  }, [messageLoading, messages.length, selectedConversationId])

  useEffect(() => {
    if (!selectedConversationId) {
      return
    }
    const refreshMessages = () => void loadMessages(selectedConversationId, true)
    const refreshBase = () => void loadBaseDataRef.current(true)
    const messageTimer = window.setInterval(refreshMessages, 5_000)
    const baseTimer = window.setInterval(refreshBase, 15_000)
    const refreshAll = () => {
      refreshMessages()
      refreshBase()
    }
    window.addEventListener("focus", refreshAll)
    return () => {
      window.clearInterval(messageTimer)
      window.clearInterval(baseTimer)
      window.removeEventListener("focus", refreshAll)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedConversationId])

	  useEffect(() => {
	    const accessToken = session?.accessToken?.trim()
	    const conversationId = selectedConversationId
    if (!accessToken || session?.domainType !== "customer" || conversationId <= 0) {
      setRealtimeConnected(false)
      setPresence({})
      setRemoteTyping({})
      return
    }

    let active = true
    const typingExpiry = typingExpiryRef.current
    const realtime = createRealtimeConnectionManager({
      createSocket: () => createAccessTokenWebSocket(
        `${createWebSocketBaseUrl()}/api/ws/open`,
        accessToken,
      ),
      canReconnect: () => active,
      onSocketChange: (socket) => {
        realtimeSocketRef.current = socket
      },
      onStatusChange: (status) => {
        if (!active) return
        const connected = status === "connected"
        setRealtimeConnected(connected)
        if (!connected) {
          setPresence({})
          setRemoteTyping({})
        }
      },
      onOpen: (socket) => {
        socket.send(JSON.stringify({ type: "subscribe", topics: [`conversation:${conversationId}`] }))
      },
      onMessage: (event) => {
        if (!active) return
        let envelope: { type?: string; data?: unknown }
        try {
          envelope = JSON.parse(String(event.data)) as typeof envelope
        } catch {
          return
        }
        if (envelope.type === "presence.snapshot") {
          const snapshot = envelope.data as RealtimePresenceSnapshotPayload
          if (snapshot?.conversationId !== conversationId) return
          const participants = (snapshot.participants || []).filter(
            (item) => item.actorId && item.online && item.participantType !== "customer",
          )
          setPresence(Object.fromEntries(participants.map((item) => [item.actorId, item])))
          return
        }
        if (envelope.type === "presence.changed") {
          const payload = envelope.data as RealtimePresencePayload
          if (payload?.conversationId === conversationId && payload.participantType !== "customer") {
            setPresence((current) => mergeRealtimePresenceActor(current, payload))
          }
          return
        }
        if (envelope.type === "typing.changed") {
          const payload = envelope.data as RealtimeTypingPayload
          if (payload?.conversationId !== conversationId || payload.participantType === "customer") return
          setRemoteTyping((current) => updateRealtimeTypingState(current, payload))
          const timer = typingExpiry.get(payload.actorId)
          if (timer) window.clearTimeout(timer)
          typingExpiry.delete(payload.actorId)
          if (payload.typing) {
            typingExpiry.set(payload.actorId, window.setTimeout(() => {
              setRemoteTyping((current) => expireRealtimeTypingActor(current, payload.actorId))
              typingExpiry.delete(payload.actorId)
            }, 5_200))
          }
          return
        }
        if (envelope.type === "message.created" || envelope.type === "conversation.updated") {
          void loadMessages(conversationId, true)
          void loadBaseDataRef.current(true)
        }
      },
      reconnectBaseDelayMs: 500,
      reconnectMaxDelayMs: 5_000,
    })
    realtime.connect()
    return () => {
      active = false
      const socket = realtimeSocketRef.current
      if (socket?.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: "typing", conversationId, typing: false }))
      }
      realtime.disconnect()
      realtimeSocketRef.current = null
      typingExpiry.forEach((timer) => window.clearTimeout(timer))
      typingExpiry.clear()
      setRealtimeConnected(false)
      setPresence({})
      setRemoteTyping({})
    }
	    // eslint-disable-next-line react-hooks/exhaustive-deps
	  }, [selectedConversationId, session?.accessToken, session?.domainType])

	  if (!hasLoadedConversations && !loading && conversationLoadError) {
	    return (
	      <ErrorState
	        title={t("customerChat.loadFailed")}
	        description={conversationLoadError}
	        action={{ label: t("common.retry"), onClick: () => void loadBaseData() }}
	      />
	    )
	  }

	  function handleTypingChange(typing: boolean) {
    const socket = realtimeSocketRef.current
    if (!socket || socket.readyState !== WebSocket.OPEN || selectedConversationId <= 0) return
    socket.send(JSON.stringify({ type: "typing", conversationId: selectedConversationId, typing }))
    if (typing) {
      window.setTimeout(() => {
        const current = realtimeSocketRef.current
        if (current?.readyState === WebSocket.OPEN) {
          current.send(JSON.stringify({ type: "typing", conversationId: selectedConversationId, typing: false }))
        }
      }, TYPING_IDLE_TIMEOUT_MS)
    }
  }

  async function activateCreatedConversation(created: ImConversation, device?: CustomerPortalDevice) {
    const createdConversation = buildCreatedCustomerPortalConversation(created, device)
    selectedConversationIdRef.current = created.id
    messageListRequestRef.current += 1
    setConversationFilter("all")
    setConversationSearchInput("")
    setConversationQuery("")
    if (device) {
      setDevices((current) => upsertCustomerPortalDevice(current, device))
    }
    setConversations((current) => upsertCustomerPortalConversation(current, createdConversation))
    setSelectedConversationId(created.id)
    setMobileConversationListOpen(false)
    setCreateOpen(false)
    await loadBaseData(true, createdConversation)
    setSelectedConversationId(created.id)
    syncCustomerConversationUrl(created.id)
    void loadMessages(created.id, true)
  }

  async function handleCreateConversation() {
    const deviceId = Number(createDeviceId || "0")
    if (deviceId <= 0) {
      return
    }
    if (creatingConversation) return
    setCreatingConversation(true)
    try {
      const created = await createOrMatchImConversation({
        deviceId,
        forceNew: true,
      })
      await activateCreatedConversation(created, createDevice)
      toast.success(t("customerChat.deviceConversationCreated"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("customerChat.conversationCreateFailed"))
    } finally {
      setCreatingConversation(false)
    }
  }

  async function handleCreateSystemHelpConversation() {
    if (!tenantAIEnabled) return
    if (creatingConversation) return
    setCreateDeviceId("general")
    setCreatingConversation(true)
    try {
      const created = await createOrMatchImConversation({
        general: true,
        forceNew: true,
      })
      await activateCreatedConversation(created)
      toast.success(t("customerChat.generalConversationCreated"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("customerChat.conversationCreateFailed"))
    } finally {
      setCreatingConversation(false)
    }
  }

  function openBindAndConsultDialog() {
    setCreateDialogView("bind")
  }

  function closeCreateDialog() {
    setCreateDialogView("list")
    setCreateDeviceQuery("")
    setCreateOpen(false)
  }

  async function handleBoundDeviceConversation(binding: BindCustomerDeviceResponse) {
    const deviceId = Number(binding.deviceId || 0)
    if (deviceId <= 0) {
      throw new Error(t("customerChat.deviceResolvedButUnavailable"))
    }
    const boundDevice = devices.find((item) => item.id === deviceId) ?? buildCustomerPortalDeviceFromBinding(binding)
    try {
      const created = await createOrMatchImConversation({
        deviceId,
        forceNew: true,
      })
      await activateCreatedConversation(created, boundDevice)
    } catch (error) {
      throw new Error(t("customerChat.deviceResolvedButConversationUnavailable", {
        reason: error instanceof Error ? error.message : t("customerChat.conversationCreateFailed"),
      }))
    }
  }

  function openNewConversationDialog() {
    if (!hasDeviceConcept) {
      void handleCreateSystemHelpConversation()
      return
    }
    setCreateDeviceId(devices[0] ? String(devices[0].id) : "")
    setCreateDeviceQuery("")
    setCreateDialogView(isGuest ? "bind" : "list")
    setCreateOpen(true)
  }

  async function handleSendMessage(html: string) {
    if (isGuest) {
      return
    }
    if (!selectedConversationId || !html.trim()) {
      return
    }
    setSubmitting(true)
    try {
      const message = await sendImMessage({
        conversationId: selectedConversationId,
        messageType: "html",
        content: html,
        clientMsgId: globalThis.crypto?.randomUUID?.() ?? `${Date.now()}`,
      })
      setMessages((current) => upsertImMessageReplacingClientMessage(current, message))
      await loadMessages(selectedConversationId, true)
      await loadBaseData(true)
      handleTypingChange(false)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("customerChat.sendFailed"))
    } finally {
      setSubmitting(false)
    }
  }

  const onlineParticipants = Object.values(presence).filter((item) => item.online)
  const engineerOnline = onlineParticipants.some((item) => item.participantType === "agent")
  const supplierOnlineCount = onlineParticipants.filter((item) => item.participantType === "partner").length
  const humanSupportVisible = selectedConversation && selectedConversation.human_handoff_enabled !== false
    ? selectedConversation.status === "queued" ||
      selectedConversation.status === "human_serving" ||
      Boolean(selectedConversation.current_ticket_no) ||
      Boolean(selectedConversation.current_assignee_name)
    : false
  const latestMessage = messages[messages.length - 1]
  const assistantResponding = Boolean(
    selectedConversation?.status !== "closed" &&
    !isGuest &&
    !humanSupportVisible &&
    !messageLoading &&
    latestMessage?.senderType.toLowerCase() === "customer",
  )
  const realtimeStatusText = selectedConversation?.status === "closed"
    ? t("customerChat.realtimeClosed")
    : !realtimeConnected
      ? t("customerChat.realtimeReconnecting")
      : humanSupportVisible && engineerOnline
        ? supplierOnlineCount > 0
          ? t("customerChat.realtimeEngineerAndSupplierOnline", { count: supplierOnlineCount })
          : t("customerChat.realtimeEngineerOnline")
        : humanSupportVisible && supplierOnlineCount > 0
          ? t("customerChat.realtimeSupplierOnline", { count: supplierOnlineCount })
          : humanSupportVisible
            ? selectedConversation?.status === "human_serving"
              ? t("customerChat.realtimeEngineerOffline")
              : t("customerChat.realtimeWaitingHuman")
            : assistantResponding
              ? t("customerChat.realtimeAssistantResponding")
              : t("customerChat.realtimeAssistantOnline")
  const realtimeStatusTone = selectedConversation?.status === "closed"
    ? "closed"
    : !realtimeConnected
      ? "reconnecting"
      : assistantResponding
        ? "assistant-responding"
        : humanSupportVisible && (engineerOnline || supplierOnlineCount > 0)
          ? "human-online"
          : humanSupportVisible
            ? "waiting-human"
            : "assistant-online"
  const activeTypists = Object.values(remoteTyping).filter((item) => item.typing)
  const typingLabel = activeTypists.some((item) => item.participantType === "agent")
    ? t("customerChat.typingEngineer")
    : activeTypists.some((item) => item.participantType === "partner")
      ? t("customerChat.typingSupplier")
      : ""

  const createDevice = devices.find((item) => String(item.id) === createDeviceId)
  const generalConsultationTitle = t("customerChat.generalConsultation")
  const generalConsultationPrompt = t("customerChat.generalConsultationPrompt")
  const normalizedCreateDeviceQuery = createDeviceQuery.trim().toLowerCase()
  const filteredCreateDevices = normalizedCreateDeviceQuery
    ? devices.filter((device) =>
        [device.device_no, device.product_name, device.model_name]
          .filter((text): text is string => Boolean(text))
          .some((text) => text.toLowerCase().includes(normalizedCreateDeviceQuery)))
    : devices
  const hasConversationListFilter = conversationFilter !== "all" || conversationQuery.trim() !== ""
  const filteredConversations = conversations

  async function handleUploadImage(file: File) {
    if (isGuest) {
      return null
    }
    if (!selectedConversationId) {
      return null
    }
    setUploadingAsset(true)
    try {
      return await uploadImImage(selectedConversationId, file)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("customerChat.imageUploadFailed"))
      return null
    } finally {
      setUploadingAsset(false)
    }
  }

  async function handleSendAttachment(file: File) {
    if (isGuest) {
      return
    }
    if (!selectedConversationId) {
      return
    }
    const clientMsgId = `customer_portal_attachment_${globalThis.crypto?.randomUUID?.() ?? Date.now()}`
    const pendingMessage = buildPendingPortalMediaMessage({
      conversationId: selectedConversationId,
      clientMsgId,
      messageType: "attachment",
      file,
    })
    setMessages((current) => [...current, pendingMessage])
    setUploadingAsset(true)
    try {
      const asset = await uploadImAttachment(selectedConversationId, file)
      const message = await sendImMessage({
        conversationId: selectedConversationId,
        messageType: "attachment",
        content: asset.filename,
        payload: JSON.stringify({ assetId: asset.assetId }),
        clientMsgId,
      })
      setMessages((current) => upsertImMessageReplacingClientMessage(current, message))
      await loadBaseData(true)
    } catch (error) {
      setMessages((current) => current.map((message) =>
        message.clientMsgId === clientMsgId
          ? markPendingPortalMediaFailed(message, file)
          : message
      ))
      toast.error(error instanceof Error ? error.message : t("customerChat.attachmentSendFailed"))
    } finally {
      setUploadingAsset(false)
    }
  }

  async function handleSendVoice(file: File, durationSeconds: number) {
    if (isGuest) return
    if (!selectedConversationId) return
    const clientMsgId = `customer_portal_audio_${globalThis.crypto?.randomUUID?.() ?? Date.now()}`
    const pendingMessage = buildPendingPortalMediaMessage({
      conversationId: selectedConversationId,
      clientMsgId,
      messageType: "audio",
      file,
      durationSeconds,
    })
    setMessages((current) => [...current, pendingMessage])
    setUploadingAsset(true)
    try {
      const asset = await uploadImAudio(selectedConversationId, file)
      const message = await sendImMessage({
        conversationId: selectedConversationId,
        messageType: "audio",
        content: asset.filename,
        payload: JSON.stringify({ assetId: asset.assetId, durationSeconds }),
        clientMsgId,
      })
      setMessages((current) => upsertImMessageReplacingClientMessage(current, message))
      await loadBaseData(true)
    } catch (error) {
      setMessages((current) => current.map((message) =>
        message.clientMsgId === clientMsgId
          ? markPendingPortalMediaFailed(message, file, durationSeconds)
          : message
      ))
      toast.error(error instanceof Error ? error.message : t("customerChat.voiceSendFailed"))
      throw error
    } finally {
      setUploadingAsset(false)
    }
  }

  async function handleRequestHuman() {
    if (isGuest || !selectedConversationId || conversationAction) {
      return
    }
    setConversationAction("human")
    try {
      await requestImHumanSupport(selectedConversationId, t("customerChat.humanSupportRequestReason"))
      await loadMessages(selectedConversationId, true)
      await loadBaseData(true)
      toast.success(t("customerChat.humanSupportRequestSuccess"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("customerChat.humanSupportRequestFailed"))
    } finally {
      setConversationAction("")
    }
  }

  async function handleConfirmResolved() {
    if (!selectedConversationId || conversationAction) {
      return
    }
    if (selectedConversation?.current_ticket_no) {
      router.push(buildCustomerTicketPath(selectedTicketId, selectedConversationId))
      return
    }
    setConversationAction("close")
    try {
      await closeImConversation(selectedConversationId)
      await loadBaseData(true)
      toast.success(t("customerChat.confirmResolvedSuccess"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("customerChat.confirmResolvedFailed"))
    } finally {
      setConversationAction("")
    }
  }

  function handleTranslationTargetLanguageChange(language: string) {
    setTranslationTargetLanguage(language)
    setTranslatedMessages({})
  }

  async function handleTranslateMessage(message: ImMessage) {
    if (!message.id || !selectedConversationId || translatingMessageId !== null) {
      return
    }
    if (!message.content.trim()) {
      toast.info(t("customerChat.translationEmpty"))
      return
    }
    const targetLanguage = resolveCustomerMessageTranslationTarget(message.content, translationTargetLanguage)
    if (targetLanguage !== translationTargetLanguage) {
      handleTranslationTargetLanguageChange(targetLanguage)
      toast.info(t("customerChat.translationAutoTarget", {
        language: customerTranslationLanguageLabel(targetLanguage),
      }))
    }
    setTranslatingMessageId(message.id)
    try {
      const translation = await translateCustomerConversationMessage(
        selectedConversationId,
        message.id,
        targetLanguage,
      )
      setTranslatedMessages((current) => ({ ...current, [message.id]: translation }))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("customerChat.translationFailed"))
    } finally {
      setTranslatingMessageId(null)
    }
  }

  return (
    <div className="rhd-railops-customer-page rhd-railops-customer-chat-page space-y-4 text-rhd-md">
      <PageHeader
        title={t("customerChat.pageTitle")}
        compact
        action={(
          <>
            <Button type="button" variant="outline" size="icon" aria-label={t("common.refresh")} title={t("common.refresh")} onClick={() => void loadBaseData()} disabled={loading}>
              <RefreshCwIcon className={cn("size-4", loading && "animate-spin")} />
            </Button>
            <Button type="button" onClick={openNewConversationDialog} disabled={creatingConversation}>
              <PlusIcon className="mr-2 size-4" />
              {hasDeviceConcept && isGuest ? t("customerDevices.addDevice") : t("customerChat.newConversation")}
            </Button>
          </>
        )}
	      />

	      {conversationLoadError && hasLoadedConversations ? (
	        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-muted px-4 py-3 text-sm text-muted-foreground">
	          <span>{t("customerChat.refreshFailed")}</span>
	          <Button variant="outline" size="sm" onClick={() => void loadBaseData()} disabled={loading}>{t("common.retry")}</Button>
	        </div>
	      ) : null}

	      <div className="rhd-railops-customer-chat-workspace grid gap-4 lg:grid-cols-[280px_minmax(0,1fr)] xl:h-[calc(100dvh-190px)] xl:grid-cols-[280px_minmax(420px,1fr)_280px] 2xl:grid-cols-[330px_minmax(560px,1fr)_320px]">
	        <section className={cn(
	          "rhd-railops-customer-chat-list h-[calc(100dvh-190px)] min-h-[520px] min-w-0 flex-col overflow-hidden rounded-md border border-border bg-card lg:flex xl:h-full xl:min-h-0",
	          mobileConversationListOpen || !selectedConversation ? "flex" : "hidden",
	        )}>
	          <div className="space-y-3 border-b border-border p-3">
	            <div className="flex items-center justify-between gap-3">
	              <h2 className="text-base font-semibold text-foreground">{t("customerChat.myConversations")}</h2>
	              <span className="shrink-0 whitespace-nowrap rounded-full bg-muted px-2 py-0.5 text-rhd-xs tabular-nums text-muted-foreground">
	                {t("pagination.total", { total: conversationTotal })}
	              </span>
	            </div>
	            <SearchField
	              allowClear
	              className="rhd-railops-search-compact"
	              aria-label={t(hasDeviceConcept ? "customerChat.searchPlaceholder" : "customerChat.searchPlaceholderGeneral")}
	              value={conversationSearchInput}
	              onChange={(event) => setConversationSearchInput(event.target.value)}
	              placeholder={t(hasDeviceConcept ? "customerChat.searchPlaceholder" : "customerChat.searchPlaceholderGeneral")}
	            />
	          </div>
	          {conversationRefreshing ? <div className="border-b border-border px-4 py-2 text-rhd-xs text-muted-foreground" role="status" aria-busy="true"><Loader2Icon className="mr-2 inline size-3.5 animate-spin" />{t("customerChat.loadingConversation")}</div> : null}
	          <ScrollArea className="min-h-0 flex-1 px-2 py-2">
	            <div className="space-y-1 pr-1">
	              {initialConversationLoad ? Array.from({ length: 5 }).map((_, index) => <Skeleton key={index} className="h-24 rounded-md" />) : null}
              {!initialConversationLoad && conversations.length === 0 && !hasConversationListFilter && conversationTotal === 0 ? (
                <EmptyCard icon={<MessagesSquareIcon className="size-6" />} title={t("customerChat.emptyTitle")} action={<Button onClick={openNewConversationDialog} disabled={creatingConversation}>{hasDeviceConcept && isGuest ? t("customerDevices.addDevice") : t("customerChat.startConsultation")}</Button>} />
              ) : null}
              {!initialConversationLoad && conversations.length === 0 && (hasConversationListFilter || conversationTotal > 0) ? (
                <div className="py-16 text-center text-xs text-muted-foreground">{t("customerChat.noFilteredResults")}</div>
              ) : null}
	              {filteredConversations.map((item) => {
	                const selected = selectedConversationId === item.id
	                const compactDeviceNo = item.device_no ? compactCustomerIdentifier(item.device_no) : ""
	                const itemTitle = item.product_name || compactDeviceNo || generalConsultationTitle
	                const itemSummary = renderCustomerConversationSummary(item.last_message_summary, t, t("customerChat.listSummaryFallback"))
	                return (
	                  <button
	                    key={item.id}
	                    type="button"
	                    aria-current={selected ? "true" : undefined}
	                    onClick={() => {
	                      selectedConversationIdRef.current = item.id
	                      messageListRequestRef.current += 1
	                      setSelectedConversationId(item.id)
	                      syncCustomerConversationUrl(item.id)
	                      setMobileConversationListOpen(false)
	                    }}
	                    className={cn(
	                      "relative h-24 w-full overflow-hidden rounded-md border-l-2 px-3 py-2 text-left transition-colors",
	                      selected ? "border-l-primary bg-primary/10" : "border-l-transparent bg-transparent hover:bg-muted/60",
	                    )}
	                  >
	                    <div className="flex h-5 min-w-0 items-center justify-between gap-2">
	                      <div className="min-w-0 truncate text-rhd-md font-semibold text-foreground" title={item.product_name || undefined}>{itemTitle}</div>
	                      <StatusBadge status={item.status} />
	                    </div>
	                    <div className="mt-0.5 h-4 truncate text-rhd-xs leading-4 text-muted-foreground" title={hasDeviceConcept ? item.device_no || undefined : undefined}>{hasDeviceConcept ? compactDeviceNo || t("customerChat.unlinkedDevice") : generalConsultationTitle}</div>
	                    <p className="mt-1 h-4 truncate text-rhd-md leading-4 text-muted-foreground" title={itemSummary}>{itemSummary}</p>
	                    <div className="mt-1 flex h-4 items-center justify-between text-rhd-xs leading-4 text-muted-foreground">
	                      <span className="tabular-nums">{formatDateTime(item.last_active_at, locale)}</span>
	                      {item.customer_unread_count > 0 ? <span className="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1.5 font-semibold leading-none text-primary-foreground">{item.customer_unread_count}</span> : null}
	                    </div>
	                  </button>
	                )
	              })}
	            </div>
	          </ScrollArea>
	          <div className="border-t border-border px-3 py-2.5">
	            <ListPagination
	              page={conversationPage}
	              total={conversationTotal}
	              limit={conversationLimit}
	              loading={loading}
	              compact
	              pageSizeOptions={[10, 20, 50]}
              onPageChange={(page) => setConversationPage(page)}
              onLimitChange={(limit) => {
                setConversationLimit(limit)
                setConversationPage(1)
              }}
            />
          </div>
        </section>

        <section className={cn(
	          "rhd-railops-customer-chat-thread h-[calc(100dvh-190px)] min-h-[520px] min-w-0 flex-col overflow-hidden rounded-md border border-border bg-card lg:flex xl:h-full xl:min-h-0",
	          mobileConversationListOpen || (!selectedConversation && !selectedConversationDetailLoading && !initialConversationLoad) ? "hidden" : "flex",
	        )}>
          {initialConversationLoad || selectedConversationDetailLoading ? (
            <CustomerChatThreadLoading />
          ) : !selectedConversation ? (
            <EmptyCard icon={<CpuIcon className="size-6" />} title={t("customerChat.selectConversationTitle")} />
          ) : (
            <>
              <div className="flex flex-col gap-4 border-b border-border px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex min-w-0 items-center gap-3">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="shrink-0 lg:hidden"
                    aria-label={t("customerChat.backToList")}
                    onClick={() => setMobileConversationListOpen(true)}
                  >
                    <ArrowLeftIcon className="size-4" />
                  </Button>
                  <span className="grid size-11 shrink-0 place-items-center rounded-lg bg-muted text-foreground">
                    <CpuIcon className="size-5" />
                  </span>
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <h2 className="truncate text-base font-semibold text-foreground">{hasDeviceConcept ? selectedConversation.device_no || generalConsultationTitle : generalConsultationTitle}</h2>
                      <StatusBadge status={selectedConversation.status} />
                    </div>
                    <p className="mt-0.5 truncate text-xs text-muted-foreground">
                      {hasDeviceConcept ? selectedConversation.product_name || t("customerChat.unlinkedDevice") : generalConsultationPrompt}
                      {hasDeviceConcept && selectedDevice?.model_name ? ` · ${selectedDevice.model_name}` : ""}
                    </p>
                    <div
                      className={cn(
                        "mt-1.5 inline-flex min-h-7 max-w-full items-center gap-2 rounded-md border px-2.5 py-1 text-xs font-medium transition-colors",
                        realtimeStatusTone === "assistant-responding" && "border-primary/20 bg-primary/10 text-primary shadow-sm",
                        realtimeStatusTone === "assistant-online" && "border-primary/20 bg-primary/10 text-primary",
                        realtimeStatusTone === "human-online" && "border-primary/20 bg-primary/10 text-primary",
                        realtimeStatusTone === "waiting-human" && "border-border bg-muted/50 text-muted-foreground",
                        realtimeStatusTone === "reconnecting" && "border-border bg-muted/50 text-muted-foreground",
                        realtimeStatusTone === "closed" && "border-border bg-muted/50 text-muted-foreground",
                      )}
                      data-testid="customer-realtime-status"
                      role="status"
                      aria-live="polite"
                      aria-busy={realtimeStatusTone === "assistant-responding" || realtimeStatusTone === "reconnecting"}
                    >
                      {realtimeStatusTone === "assistant-responding" ? (
                        <span className="relative flex size-2.5 shrink-0" aria-hidden="true">
                          <span className="absolute inline-flex size-full animate-ping rounded-full bg-primary/60 opacity-60 motion-reduce:animate-none" />
                          <span className="relative inline-flex size-2.5 rounded-full bg-primary" />
                        </span>
                      ) : realtimeStatusTone === "reconnecting" ? (
                        <Loader2Icon className="size-3.5 shrink-0 animate-spin" aria-hidden="true" />
                      ) : realtimeStatusTone === "waiting-human" ? (
                        <HeadphonesIcon className="size-3.5 shrink-0" aria-hidden="true" />
                      ) : realtimeStatusTone === "closed" ? (
                        <CheckCircle2Icon className="size-3.5 shrink-0" aria-hidden="true" />
                      ) : realtimeStatusTone === "human-online" ? (
                        <CircleUserRoundIcon className="size-3.5 shrink-0" aria-hidden="true" />
                      ) : (
                        <MessageSquareTextIcon className="size-3.5 shrink-0" aria-hidden="true" />
                      )}
                      <span className="truncate">{realtimeStatusText}</span>
                      {realtimeStatusTone === "assistant-responding" ? (
                        <span className="flex shrink-0 items-center gap-0.5" aria-hidden="true">
                          {[0, 1, 2].map((index) => (
                            <span
                              key={index}
                              className="size-1 rounded-full bg-current motion-safe:animate-bounce"
                              style={{ animationDelay: `${index * 160}ms` }}
                            />
                          ))}
                        </span>
                      ) : null}
                    </div>
                    {selectedConversation.status === "queued" && selectedConversation.current_ticket_no ? (
                      <div className="mt-1 text-rhd-xs font-medium text-primary" data-testid="customer-assignment-status">
                        {selectedConversation.current_assignee_name
                          ? t("customerChat.assignedToEngineer", { name: selectedConversation.current_assignee_name })
                          : t("customerChat.waitingForClaim")}
                      </div>
                    ) : null}
                  </div>
                </div>
                <TooltipProvider>
                  <div className="flex flex-wrap items-center justify-start gap-1.5 sm:justify-end" aria-label={t("customerChat.quickActionsLabel")}>
                    {selectedDevice ? (
                      <Tooltip>
                        <TooltipTrigger render={<Button type="button" variant="ghost" size="icon" aria-label={t("customerChat.viewDeviceArchive")} onClick={() => router.push(buildCustomerDevicePath(selectedDevice.id, selectedConversationId))} />}>
                          <FileTextIcon className="size-4" />
                        </TooltipTrigger>
                        <TooltipContent>{t("customerChat.viewDeviceArchive")}</TooltipContent>
                      </Tooltip>
                    ) : null}
                    {selectedConversation.current_ticket_no ? (
                      <Tooltip>
                        <TooltipTrigger render={<Button type="button" variant="ghost" size="icon" aria-label={t("customerChat.viewRelatedTicket")} onClick={() => router.push(buildCustomerTicketPath(selectedTicketId, selectedConversationId))} />}>
                          <TicketCheckIcon className="size-4" />
                        </TooltipTrigger>
                        <TooltipContent>{t("customerChat.viewRelatedTicket")}</TooltipContent>
                      </Tooltip>
                    ) : null}
                    {selectedConversation.current_meeting_id ? (
                      <Tooltip>
                        <TooltipTrigger render={<Button type="button" variant="ghost" size="icon" aria-label={t("customerChat.viewVideoCollaboration")} onClick={() => router.push(`/customer/meeting?meetingId=${selectedConversation.current_meeting_id}`)} />}>
                          <VideoIcon className="size-4" />
                        </TooltipTrigger>
                        <TooltipContent>{t("customerChat.viewVideoCollaboration")}</TooltipContent>
                      </Tooltip>
                    ) : null}
                    {!isGuest && selectedConversation.status !== "closed" && selectedConversation.status !== "human_serving" && selectedConversation.status !== "queued" && selectedConversation.human_handoff_enabled !== false ? (
                      <Button type="button" variant="outline" size="sm" aria-label={t("customerChat.requestHuman")} onClick={handleRequestHuman} disabled={Boolean(conversationAction)}>
                        {conversationAction === "human" ? <Loader2Icon className="size-4 animate-spin" /> : <HeadphonesIcon className="size-4" />}
                        {t("customerChat.requestHuman")}
                      </Button>
                    ) : null}
                  </div>
                </TooltipProvider>
              </div>
              <div className="h-7 shrink-0 px-6 pt-2 text-rhd-xs text-primary" aria-live="polite">{typingLabel}</div>
	                <ScrollArea className="min-h-0 flex-1 bg-card px-5 py-5">
                  <div className="pr-4">
	                    {messageLoading ? (
	                      <div className="mb-3 flex h-7 items-center gap-2 text-rhd-xs font-medium text-muted-foreground" role="status" aria-busy="true" aria-label={t("customerChat.loadingMessages")}>
	                        <Loader2Icon className="size-3.5 animate-spin" aria-hidden="true" />
	                        <span>{t("customerChat.loadingMessages")}</span>
	                      </div>
                    ) : null}
	                    {messageLoading ? Array.from({ length: 4 }).map((_, index) => <Skeleton key={index} className="h-24 rounded-lg" />) : null}
                    {!messageLoading && messages.length === 0 ? <div className="py-20 text-center text-xs text-muted-foreground">{t("customerChat.emptyMessagesPrompt")}</div> : null}
                    <CustomerConversationTimeline
                      messages={messages}
                      hasDeviceConcept={hasDeviceConcept}
                      completion={selectedConversation.status === "closed" ? {
                        title: closedTimelineTitle,
                        description: closedTimelineDescription,
                      } : undefined}
                      translatedMessages={translatedMessages}
                      translatingMessageId={translatingMessageId}
                      onTranslateMessage={handleTranslateMessage}
                    />
                    {realtimeStatusTone === "assistant-responding" ? (
                      <div
                        className="mt-5 flex items-start gap-2.5"
                        data-testid="customer-assistant-responding-bubble"
                        role="status"
                        aria-live="polite"
                        aria-label={t("customerChat.assistantResponding")}
                      >
                        <span className="relative mt-1 flex size-8 shrink-0 items-center justify-center rounded-full border border-primary/20 bg-primary/10 text-primary shadow-sm">
                          <span className="absolute inset-0 animate-ping rounded-full bg-primary/20 opacity-50 motion-reduce:animate-none" aria-hidden="true" />
                          <MessageSquareTextIcon className="relative size-4 motion-safe:animate-pulse" aria-hidden="true" />
                        </span>
                        <div className="inline-flex min-h-10 max-w-[78%] items-center gap-2 rounded-lg border border-primary/20 bg-primary/10 px-3 py-2 text-rhd-md font-medium text-primary shadow-sm">
                          <span>{t("customerChat.assistantResponding")}</span>
                          <span className="flex shrink-0 items-center gap-1" aria-hidden="true">
                            {[0, 1, 2].map((index) => (
                              <span
                                key={index}
                                className="size-1.5 rounded-full bg-primary motion-safe:animate-bounce"
                                style={{ animationDelay: `${index * 160}ms` }}
                              />
                            ))}
                          </span>
                        </div>
                      </div>
                    ) : null}
                    <div ref={messagesEndRef} aria-hidden="true" />
                  </div>
                </ScrollArea>
              <div className="border-t border-border p-4">
                {isGuest ? (
                  <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-muted px-4 py-3 text-sm text-muted-foreground" data-testid="customer-conversation-readonly">
                    <span className="flex items-center gap-2"><ShieldCheckIcon className="size-4 shrink-0" />{t("customerDevices.emptyTitle")}</span>
                    <Button type="button" variant="outline" size="sm" onClick={() => { setCreateOpen(true); setCreateDialogView("bind") }}>{t("customerDevices.addDevice")}</Button>
                  </div>
                ) : selectedConversation.status === "closed" ? (
                  <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-muted px-4 py-3 text-sm text-muted-foreground" data-testid="customer-conversation-closed">
                    <span className="flex items-center gap-2"><CheckCircle2Icon className="size-4 shrink-0" />{closedBannerText}</span>
                    {selectedConversation.current_ticket_no ? <Button type="button" variant="outline" size="sm" onClick={() => router.push(buildCustomerTicketPath(selectedTicketId, selectedConversationId))}>{t("customerChat.viewTicket")}</Button> : null}
                  </div>
                ) : (
                  <CustomerMessageEditor disabled={submitting} uploadingAsset={uploadingAsset} onSend={handleSendMessage} onUploadImage={handleUploadImage} onSendAttachment={handleSendAttachment} onSendVoice={handleSendVoice} onTypingChange={handleTypingChange} />
                )}
              </div>
            </>
          )}
        </section>

        <aside className="rhd-railops-customer-chat-context hidden min-w-0 rounded-lg border border-border bg-card p-5 xl:col-span-1 xl:block xl:h-full xl:overflow-y-auto">
          <div className="mb-5">
            <h2 className="text-base font-semibold text-foreground">{t("customerChat.serviceContextTitle")}</h2>
          </div>
          {initialConversationLoad || selectedConversationDetailLoading ? <CustomerChatContextLoading /> : !selectedConversation ? <div className="rounded-lg border border-dashed border-border p-8 text-center text-sm text-muted-foreground">{t("customerChat.selectConversationForContext")}</div> : (
            <div className="divide-y divide-border">
              <ContextSection icon={hasDeviceConcept ? <PackageSearchIcon className="size-4" /> : <BookOpenIcon className="size-4" />} tone="blue" title={hasDeviceConcept && selectedConversation.device_id ? t("customerChat.contextCurrentDevice") : t("customerChat.contextGeneralConsultation")}>
                <div className="font-medium text-foreground">{hasDeviceConcept ? selectedConversation.device_no || selectedConversation.product_name || t("customerChat.contextUnspecifiedDevice") : generalConsultationTitle}</div>
                <div>{hasDeviceConcept ? selectedDevice?.model_name || selectedConversation.product_name || t("customerChat.contextSupplementProductInfo") : generalConsultationPrompt}</div>
                {hasDeviceConcept && selectedDevice ? <div>{warrantySummary(t, selectedDevice.warranty_end_at)} · {t("customerChat.repairCount", { count: selectedDevice.repair_history_count })}</div> : null}
              </ContextSection>
              <ContextSection icon={<TicketCheckIcon className="size-4" />} tone="amber" title={t("customerChat.contextRelatedTicket")}>
                <div className="flex items-center justify-between gap-2">
                  <span className="font-medium text-foreground">{selectedConversation.current_ticket_no || t("customerChat.contextNoTicket")}</span>
                  {selectedTicket ? <StatusBadge status={selectedTicket.status} /> : null}
                </div>
                <div>{selectedTicket?.title || (selectedConversation.current_ticket_no ? t("customerChat.contextTicketFollowing") : t("customerChat.contextDiagnosisStage"))}</div>
                {selectedTicket?.repair_summary ? <div className="line-clamp-3">{selectedTicket.repair_summary}</div> : null}
              </ContextSection>
              <ContextSection icon={<VideoIcon className="size-4" />} tone="primary" title={t("customerChat.contextVideoCollaboration")}>
                <div className="font-medium text-foreground">{selectedConversation.current_meeting_id ? customerPortalStatusLabel(t, selectedConversation.current_meeting_status) : t("customerChat.contextNoMeeting")}</div>
                <div>{selectedConversation.current_meeting_id || t("customerChat.contextMeetingInvitation")}</div>
              </ContextSection>
              <ContextSection icon={<ShieldCheckIcon className="size-4" />} tone="slate" title={t("customerChat.nextStep")}>
                <div className="text-foreground">{customerConversationNextAction(t, selectedConversation, selectedTicket)}</div>
                {selectedConversation.current_meeting_id && canJoinCustomerMeeting(selectedConversation.current_meeting_status) ? <Button className="mt-2 w-full" onClick={() => router.push(`/customer/meeting?meetingId=${selectedConversation.current_meeting_id}`)}><VideoIcon className="mr-2 size-4" />{t("customerChat.joinVideoCollaboration")}</Button> : null}
                {selectedConversation.status !== "closed" ? <Button type="button" variant="outline" className="mt-2 w-full" onClick={handleConfirmResolved} disabled={Boolean(conversationAction)}><CheckCircle2Icon className="mr-2 size-4" />{selectedConversation.current_ticket_no ? (selectedTicket?.can_confirm ? t("customerChat.confirmRepairResult") : t("customerChat.viewTicketProgress")) : t("customerChat.confirmIssueResolved")}</Button> : null}
              </ContextSection>
            </div>
          )}
	        </aside>
	      </div>

		      {hasDeviceConcept ? <StandardModal
	        open={createOpen}
	        onCancel={closeCreateDialog}
	        width={672}
	        title={t("customerChat.createDialogTitle")}
	        footer={createDialogView === "bind" ? (
	          <>
	            <Button variant="outline" onClick={() => setCreateDialogView("list")} disabled={bindSubmitting}>
	              <ArrowLeftIcon className="mr-2 size-4" />
	              {t("customerChat.dialogBindBack")}
	            </Button>
	            <Button type="submit" form="conversation-bind-device-form" disabled={bindSubmitting || !bindCanSubmit}>
	              {bindSubmitting ? <Loader2Icon className="mr-2 size-4 animate-spin" /> : null}
	              {bindSubmitting ? t("customerOnboarding.dialogSubmitting") : t("customerOnboarding.dialogSubmit")}
	            </Button>
	          </>
	        ) : (
	          <>
	            <Button variant="outline" onClick={closeCreateDialog} disabled={creatingConversation}>{t("common.close")}</Button>
	            {createDevice ? (
	              <Button onClick={handleCreateConversation} disabled={creatingConversation}>
	                {creatingConversation ? <Loader2Icon className="mr-2 size-4 animate-spin" /> : <MessageSquareTextIcon className="mr-2 size-4" />}
	                {creatingConversation ? t("customerChat.creatingConversation") : t("customerChat.createDeviceConversation")}
	              </Button>
	            ) : null}
	          </>
	        )}
	      >
          {createDialogView === "bind" ? (
            <div className="py-2">
              <CustomerDeviceBindForm
                formId="conversation-bind-device-form"
                fieldIdPrefix="conversation-bind-device"
                onBound={handleBoundDeviceConversation}
                successMessage={t("customerOnboarding.conversationSuccess")}
                autoFocus
                onSuccess={closeCreateDialog}
                onSubmittingChange={setBindSubmitting}
                onServiceCodeChange={(value) => setBindCanSubmit(Boolean(value.trim()))}
              />
            </div>
          ) : (
            <>
              <div className="flex items-center gap-2 py-2">
                <SearchField allowClear value={createDeviceQuery} onChange={(event) => setCreateDeviceQuery(event.target.value)} placeholder={t("customerDevices.searchPlaceholder")} className="rhd-railops-search-compact" />
                <Button type="button" variant="outline" onClick={openBindAndConsultDialog} disabled={creatingConversation}>
                  <PlusIcon className="mr-2 size-4" />
                  {t("customerChat.dialogBindTitle")}
                </Button>
              </div>
              <div className="max-h-[44vh] overflow-y-auto py-1">
                {devicesLoading ? (
                  <div className="space-y-2" role="status" aria-busy="true" aria-label={t("customerDevices.loadingList")}>
                    <div className="flex items-center gap-2 rounded-lg border border-border bg-muted/40 px-4 py-3 text-sm text-muted-foreground">
                      <Loader2Icon className="size-4 animate-spin" />
                      <span>{t("customerDevices.loadingList")}</span>
                    </div>
                    {Array.from({ length: 3 }).map((_, index) => <Skeleton key={index} className="h-16 rounded-lg" />)}
                  </div>
                ) : filteredCreateDevices.length === 0 ? (
                  <div className="rounded-lg border border-dashed border-border bg-muted/40 px-4 py-3 text-sm leading-6 text-muted-foreground">
                    {devices.length === 0 ? t("customerDevices.emptyTitle") : t("customerDevices.noSearchResults")}
                  </div>
                ) : (
                  <div className="space-y-2">
                    {filteredCreateDevices.map((device) => (
                      <button key={device.id} type="button" onClick={() => setCreateDeviceId(String(device.id))} className={cn("flex w-full items-center gap-3 rounded-lg border p-3.5 text-left transition", createDeviceId === String(device.id) ? "border-primary bg-primary/10 ring-2 ring-primary/20" : "border-border hover:border-border")}>
                        <span className="grid size-10 shrink-0 place-items-center rounded-lg bg-muted text-foreground"><CpuIcon className="size-5" /></span>
                        <span className="min-w-0 flex-1">
                          <span className="block truncate font-semibold text-foreground">{device.device_no}</span>
                          <span className="mt-0.5 block truncate text-sm text-muted-foreground">{device.product_name || t("customerDevices.productFallback")} · {device.model_name || t("customerDevices.modelFallback")}</span>
                        </span>
                        <StatusBadge status={device.status} />
                      </button>
                    ))}
                  </div>
                )}
              </div>
              {deviceLoadError ? <div className="rounded-lg border border-border bg-muted/40 px-4 py-3 text-sm leading-6 text-muted-foreground" role="status">{deviceLoadError}</div> : null}
              {tenantAIEnabled ? (
                <div className="border-t border-border py-2">
                  <button type="button" onClick={() => { if (isGuest) { closeCreateDialog(); setDocsOpen(true) } else { void handleCreateSystemHelpConversation() } }} disabled={creatingConversation} className="flex w-full items-center gap-3 rounded-lg border border-dashed border-border p-3.5 text-left transition hover:border-primary/30 hover:bg-muted/50 disabled:cursor-not-allowed disabled:opacity-60">
                    <span className="grid size-10 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary"><BookOpenIcon className="size-5" /></span>
                    <span className="min-w-0 flex-1">
                      <span className="block font-semibold text-foreground">{generalConsultationTitle}</span>
                      <span className="mt-0.5 block text-sm leading-5 text-muted-foreground">{generalConsultationPrompt}</span>
                    </span>
                    {creatingConversation ? <Loader2Icon className="size-4 shrink-0 animate-spin text-muted-foreground" /> : <ChevronRightIcon className="size-4 shrink-0 text-muted-foreground" />}
                  </button>
                </div>
              ) : null}
            </>
          )}
	      </StandardModal> : null}
      {hasDeviceConcept ? <CustomerSystemIntroDocsModal open={docsOpen} onClose={() => setDocsOpen(false)} /> : null}
    </div>
	  )
	}

function CustomerChatThreadLoading() {
  const t = useI18n()
  return (
    <>
      <div className="flex flex-col gap-4 border-b border-border px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-center gap-3">
          <Skeleton className="size-11 shrink-0 rounded-lg" />
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className="h-5 w-48 max-w-full" />
            <Skeleton className="h-3 w-64 max-w-full" />
            <Skeleton className="h-3 w-32 max-w-full" />
          </div>
        </div>
        <div className="hidden items-center gap-1.5 sm:flex">
          {Array.from({ length: 3 }).map((_, index) => <Skeleton key={index} className="size-8 rounded-md" />)}
        </div>
      </div>
      <div className="h-7 shrink-0 px-6 pt-2">
        <div className="flex h-7 items-center gap-2 text-xs font-medium text-muted-foreground" role="status" aria-busy="true" aria-label={t("customerChat.loadingConversation")}>
          <Loader2Icon className="size-3.5 animate-spin" aria-hidden="true" />
          <span>{t("customerChat.loadingConversation")}</span>
        </div>
      </div>
      <ScrollArea className="min-h-0 flex-1 bg-card px-5 py-5">
        <div className="space-y-5 pr-4">
          {Array.from({ length: 4 }).map((_, index) => (
            <div key={index} className={cn("flex gap-2.5", index % 2 === 0 ? "justify-start" : "justify-end")}>
              {index % 2 === 0 ? <Skeleton className="mt-5 size-8 shrink-0 rounded-full" /> : null}
              <div className="w-[72%] max-w-[520px] space-y-2">
                <Skeleton className="h-3 w-28" />
                <Skeleton className="h-20 rounded-lg" />
              </div>
              {index % 2 === 1 ? <Skeleton className="mt-5 size-8 shrink-0 rounded-full" /> : null}
            </div>
          ))}
        </div>
      </ScrollArea>
      <div className="border-t border-border p-4">
        <Skeleton className="h-24 rounded-lg" />
      </div>
    </>
  )
}

function CustomerChatContextLoading() {
  const t = useI18n()
  return (
    <div className="divide-y divide-border" role="status" aria-busy="true" aria-label={t("customerChat.loadingConversation")}>
      {Array.from({ length: 4 }).map((_, index) => (
        <section key={index} className="py-4 first:pt-0 last:pb-0">
          <Skeleton className="h-4 w-28" />
          <Skeleton className="mt-3 h-4 w-40 max-w-full" />
          <Skeleton className="mt-2 h-3 w-56 max-w-full" />
          {index < 2 ? <Skeleton className="mt-3 h-16 rounded-lg" /> : null}
        </section>
      ))}
    </div>
  )
}

function ContextSection({
  icon,
  tone,
  title,
  children,
}: {
  icon: ReactNode
  tone: "blue" | "amber" | "primary" | "slate"
  title: string
  children: ReactNode
}) {
  const toneClass = tone === "blue"
    ? "bg-primary/10 text-primary"
    : tone === "amber"
    ? "bg-amber-50 text-foreground"
      : tone === "primary"
        ? "bg-primary/10 text-primary"
        : "bg-muted text-muted-foreground"
  return (
    <section className="py-4 first:pt-0 last:pb-0">
      <div className="mb-2.5 flex items-center gap-2 text-xs font-semibold text-foreground">
        <span className={cn("grid size-6 place-items-center rounded-md", toneClass)}>{icon}</span>
        {title}
      </div>
      <div className="space-y-1 text-rhd-md leading-5 text-muted-foreground">
        {children}
      </div>
    </section>
  )
}

export function CustomerTicketsPortalPage() {
  const { session } = useAuth()
  const t = useI18n()
  const { locale } = useAppLocale()
  const router = useCustomerPortalRouter()
  const searchParams = useSearchParams()
  const [tickets, setTickets] = useState<CustomerPortalTicket[]>([])
  const [ticketTotal, setTicketTotal] = useState(0)
  const [selectedTicketDetail, setSelectedTicketDetail] = useState<CustomerPortalTicket | null>(null)
  const [selectedTicketLoading, setSelectedTicketLoading] = useState(false)
  const [loading, setLoading] = useState(true)
  const [hasLoadedTickets, setHasLoadedTickets] = useState(false)
  const [loadError, setLoadError] = useState("")
  const [loadVersion, setLoadVersion] = useState(0)
  const [selectedId, setSelectedId] = useState(0)
  const [mobileTicketListOpen, setMobileTicketListOpen] = useState(() => !searchParams?.get("ticketId"))
  const [rating, setRating] = useState(0)
  const [feedbackComment, setFeedbackComment] = useState("")
  const [reopenReason, setReopenReason] = useState("")
  const [ticketAction, setTicketAction] = useState("")
  const [ticketFilter, setTicketFilter] = useState<"all" | "active" | "action" | "closed">("all")
  const [ticketQuery, setTicketQuery] = useState("")
  const [ticketPage, setTicketPage] = useState(1)
  const [ticketLimit, setTicketLimit] = useState(20)
  const ticketInitialRetryRef = useRef(false)
  const missingTicketPollsRef = useRef(0)
  const ticketQueryParam = searchParams?.get("ticketId") || ""
  const returnConversationId = normalizeCustomerConversationId(searchParams?.get("fromConversationId") || "")
  const returnConversationHref = buildCustomerChatReturnPath(returnConversationId)
  const hasDeviceConcept = session?.featureFlags?.device !== false

  const selectedTicketSummary = tickets.find((item) => item.id === selectedId) ?? null
  const selectedTicket = selectedTicketDetail?.id === selectedId ? selectedTicketDetail : selectedTicketSummary
  const initialTicketLoad = !hasLoadedTickets && loading

  useEffect(() => {
    async function loadData() {
      setLoading(true)
      setLoadError("")
      try {
        const ticketPageData = await fetchCustomerTicketsPage({
          page: ticketPage,
          limit: ticketLimit,
          keyword: ticketQuery.trim() || undefined,
          filter: ticketFilter === "all" ? undefined : ticketFilter,
        })
        setTickets(ticketPageData.results ?? [])
        setTicketTotal(ticketPageData.page?.total ?? 0)
        ticketInitialRetryRef.current = false
        setHasLoadedTickets(true)
        const queryId = Number(ticketQueryParam || "0")
        if (queryId > 0) {
          setSelectedId((current) => (current === queryId ? current : queryId))
        } else if (ticketPageData.results?.[0]) {
          setSelectedId((current) => (current > 0 ? current : ticketPageData.results[0].id))
        }
      } catch (error) {
        if (!hasLoadedTickets && !ticketInitialRetryRef.current) {
          ticketInitialRetryRef.current = true
          window.setTimeout(() => setLoadVersion((value) => value + 1), 800)
          return
        }
        const message = error instanceof Error ? error.message : t("customerTickets.loadFailed")
        setLoadError(message)
        toast.error(message)
      } finally {
        setLoading(false)
      }
    }
    loadData()
  }, [loadVersion, ticketFilter, ticketLimit, ticketQuery, ticketQueryParam, ticketPage, t])

  useEffect(() => {
    if (selectedId <= 0) {
      setSelectedTicketDetail(null)
      setSelectedTicketLoading(false)
      return
    }
    let cancelled = false
    setSelectedTicketDetail((current) => current?.id === selectedId ? current : null)
    setSelectedTicketLoading(true)
    void fetchCustomerTicketDetail(selectedId)
      .then((detail) => {
        if (!cancelled) {
          setSelectedTicketDetail(detail)
          setTickets((items) => items.map((item) => item.id === detail.id ? detail : item))
          if (detail.id === selectedId) {
            missingTicketPollsRef.current = 0
          }
        }
      })
      .catch((error) => {
        if (!cancelled) {
          toast.error(error instanceof Error ? error.message : t("customerTickets.loadFailed"))
        }
      })
      .finally(() => {
        if (!cancelled) {
          setSelectedTicketLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [loadVersion, selectedId, t])

  useEffect(() => {
    if (selectedId) {
      if (!loading && ticketQueryParam !== String(selectedId)) {
        router.replace(buildCustomerTicketPath(selectedId, returnConversationId))
      }
    }
  }, [loading, returnConversationId, router, selectedId, ticketQueryParam])

  useEffect(() => {
    const refreshTickets = () => setLoadVersion((value) => value + 1)
    const refreshVisibleTickets = () => {
      if (document.visibilityState === "visible") {
        refreshTickets()
      }
    }
    window.addEventListener("focus", refreshTickets)
    window.addEventListener("pageshow", refreshTickets)
    window.addEventListener("popstate", refreshTickets)
    document.addEventListener("visibilitychange", refreshVisibleTickets)
    return () => {
      window.removeEventListener("focus", refreshTickets)
      window.removeEventListener("pageshow", refreshTickets)
      window.removeEventListener("popstate", refreshTickets)
      document.removeEventListener("visibilitychange", refreshVisibleTickets)
    }
  }, [ticketQueryParam])

  useEffect(() => {
    const queryId = Number(ticketQueryParam || "0")
    if (queryId <= 0 || loading || loadError || selectedTicket || selectedTicketLoading || missingTicketPollsRef.current >= 5) {
      return
    }
    missingTicketPollsRef.current += 1
    const timer = window.setTimeout(() => setLoadVersion((value) => value + 1), 1200)
    return () => window.clearTimeout(timer)
  }, [loadError, loading, selectedTicket, selectedTicketLoading, ticketQueryParam])

  useEffect(() => {
    setRating(0)
    setFeedbackComment("")
    setReopenReason("")
  }, [selectedId])

  if (!hasLoadedTickets && !loading && loadError) {
    return (
      <ErrorState
        title={t("customerTickets.loadFailed")}
        description={loadError}
        action={{ label: t("customerTickets.retry"), onClick: () => setLoadVersion((value) => value + 1) }}
      />
    )
  }

  const hasNoTickets = ticketTotal === 0
  const ticketListPending = loading && !hasLoadedTickets

  async function handleConfirmTicket() {
    if (!selectedTicket || ticketAction) return
    if (selectedTicket.can_rate && rating === 0) {
      toast.error(t("customerTickets.selectRatingFirst"))
      return
    }
    setTicketAction("confirm")
    try {
      let feedback = selectedTicket.feedback
      if (selectedTicket.can_rate) {
        feedback = await submitCustomerTicketFeedback(selectedTicket.id, { rating, comment: feedbackComment })
      }
      const updated = selectedTicket.can_confirm
        ? await confirmCustomerTicket(selectedTicket.id)
        : { ...selectedTicket, feedback, can_rate: false }
      setTickets((items) => items.map((item) => item.id === updated.id ? updated : item))
      setSelectedTicketDetail(updated)
      setLoadVersion((value) => value + 1)
      setRating(0)
      setFeedbackComment("")
      toast.success(selectedTicket.can_confirm ? t("customerTickets.confirmResolvedSuccess") : t("customerTickets.feedbackSubmittedSuccess"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("customerTickets.submitFailed"))
    } finally {
      setTicketAction("")
    }
  }

  async function handleReopenTicket() {
    const reason = reopenReason.trim()
    if (!selectedTicket || ticketAction || !reason) return
    setTicketAction("reopen")
    try {
      const updated = await reopenCustomerTicket(selectedTicket.id, reason)
      setTickets((items) => items.map((item) => item.id === updated.id ? updated : item))
      setSelectedTicketDetail(updated)
      setLoadVersion((value) => value + 1)
      setReopenReason("")
      toast.success(t("customerTickets.ticketReopened"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("customerTickets.reopenFailed"))
    } finally {
      setTicketAction("")
    }
  }

  return (
    <div className="rhd-railops-customer-page rhd-railops-customer-tickets-page space-y-4">
      <PageHeader
        title={t("customerTickets.pageTitle")}
        action={(
          <>
            {returnConversationId > 0 ? (
              <Button type="button" variant="outline" onClick={() => router.push(returnConversationHref)}>
                <ArrowLeftIcon className="mr-2 size-4" />
                {t("customerTickets.backToConversation")}
              </Button>
            ) : null}
            <Button type="button" variant="outline" onClick={() => setLoadVersion((value) => value + 1)} disabled={loading}>
              <RefreshCwIcon className={cn("mr-2 size-4", loading && "animate-spin")} />
              {t("customerTickets.refresh")}
            </Button>
          </>
        )}
      />
      {loadError && hasLoadedTickets ? (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-muted px-4 py-3 text-sm text-muted-foreground">
          <span>{t("customerTickets.refreshFailed")}</span>
          <Button variant="outline" size="sm" onClick={() => setLoadVersion((value) => value + 1)} disabled={loading}>{t("customerTickets.retry")}</Button>
        </div>
      ) : null}
      {initialTicketLoad ? <CustomerTicketWorkspaceLoading /> : (
        <div className="grid gap-4 lg:grid-cols-[280px_minmax(0,1fr)] xl:grid-cols-[320px_minmax(0,1fr)]">
          <section className={cn("min-w-0 overflow-hidden rounded-md border border-border bg-card", mobileTicketListOpen ? "block" : "hidden lg:block")} aria-busy={loading}>
            <div className="space-y-2.5 border-b border-border p-3">
              <FilterTabs
                ariaLabel={t("customerTickets.filterLabel")}
                value={ticketFilter}
                items={[
                  { value: "all", label: t("customerTickets.filterAll") },
                  { value: "active", label: t("customerTickets.filterActive") },
                  { value: "action", label: t("customerTickets.filterAction") },
                  { value: "closed", label: t("customerTickets.filterClosed") },
                ]}
                onChange={(value) => {
                  setTicketPage(1)
                  setTicketFilter(value as "all" | "active" | "action" | "closed")
                }}
              />
              <div><SearchField allowClear className="rhd-railops-search-compact" aria-label={t(hasDeviceConcept ? "customerTickets.searchLabel" : "customerTickets.searchLabelGeneral")} value={ticketQuery} onChange={(event) => { setTicketPage(1); setTicketQuery(event.target.value) }} placeholder={t(hasDeviceConcept ? "customerTickets.searchLabel" : "customerTickets.searchLabelGeneral")} /></div>
            </div>
            <ScrollArea className="h-[min(360px,48vh)] p-2.5 lg:h-[calc(100dvh-260px)] lg:min-h-[420px] lg:max-h-[620px]">
              <div className="space-y-1 pr-2.5">
                {ticketListPending ? Array.from({ length: 6 }).map((_, index) => <Skeleton key={index} className="h-20 rounded-lg" />) : null}
                {!loading && hasNoTickets ? <EmptyCard icon={<TicketCheckIcon className="size-6" />} title={t("customerTickets.emptyTitle")} action={<Button onClick={() => router.push('/customer/chat')}>{t("customerTickets.gotoConversation")}</Button>} /> : null}
                {!loading && tickets.length === 0 && ticketTotal > 0 ? <div className="py-16 text-center text-sm text-muted-foreground">{t("customerTickets.filteredEmpty")}</div> : null}
                {tickets.map((item) => (
                  <button key={item.id} type="button" onClick={() => { setSelectedId(item.id); setMobileTicketListOpen(false) }} className={cn("w-full rounded-md border px-2.5 py-2 text-left transition", selectedId === item.id ? "border-primary/20 bg-primary/10 shadow-sm" : "border-transparent hover:border-border hover:bg-muted/50")}>
                    <div className="min-w-0">
                      <div data-testid="customer-ticket-list-item-heading" className="flex min-w-0 items-center gap-1.5">
                        <span className="min-w-0 truncate text-rhd-xs font-semibold leading-4 text-muted-foreground">{item.ticket_no}</span>
                        <StatusBadge status={item.status} />
                      </div>
                      <div className="mt-0.5 line-clamp-2 text-xs font-semibold leading-4 text-foreground">{item.title}</div>
                    </div>
                    <div className="mt-1.5 flex items-center justify-between gap-2 text-rhd-xs leading-4 text-muted-foreground"><span className="truncate">{hasDeviceConcept ? item.device_no || item.product_name || t("customerTickets.generalService") : t("customerTickets.generalService")}</span><span className="shrink-0">{formatDateTime(item.updated_at, locale)}</span></div>
                  </button>
                ))}
              </div>
            </ScrollArea>
            <div className="border-t border-border px-3 py-3">
              <ListPagination
                page={ticketPage}
                total={ticketTotal}
                limit={ticketLimit}
                loading={loading}
                onPageChange={(page) => setTicketPage(page)}
                onLimitChange={(limit) => {
                  setTicketLimit(limit)
                  setTicketPage(1)
                }}
              />
            </div>
          </section>

          <section className={cn("min-w-0 overflow-hidden rounded-md border border-border bg-card", mobileTicketListOpen ? "hidden lg:block" : "block")}>
            {selectedTicketLoading && selectedId > 0 ? (
              <CustomerTicketDetailLoading />
            ) : !selectedTicket ? (
              ticketQueryParam && selectedId > 0 ? (
                <EmptyCard icon={<PackageSearchIcon className="size-6" />} title={t("customerTickets.syncingTitle")} action={<Button variant="outline" onClick={() => setLoadVersion((value) => value + 1)} disabled={loading}>{t("customerTickets.refresh")}</Button>} />
              ) : (
                <EmptyCard icon={<PackageSearchIcon className="size-6" />} title={t("customerTickets.selectTitle")} />
              )
            ) : (
            <div data-testid="customer-selected-ticket">
                <div className="border-b border-border p-4 lg:p-5">
                  <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                    <div className="flex min-w-0 items-start gap-2">
                      <span className="shrink-0 lg:hidden">
                        <IconButton icon={<ArrowLeftIcon className="size-4" />} aria-label={t("common.back")} tooltip={t("common.back")} onClick={() => setMobileTicketListOpen(true)} />
                      </span>
                      <div className="min-w-0 max-w-3xl"><div className="text-xs font-semibold text-primary">{selectedTicket.ticket_no}</div><h2 className="mt-1 text-lg font-semibold leading-6 text-foreground">{selectedTicket.title}</h2><p className="mt-1 text-xs text-muted-foreground">{hasDeviceConcept ? selectedTicket.device_no || selectedTicket.product_name || t("customerTickets.unlinkedDevice") : t("customerTickets.generalService")} · {t("customerTickets.updatedAt")} {formatDateTime(selectedTicket.updated_at, locale)}</p></div>
                    </div>
                    <span data-testid="customer-selected-ticket-status"><StatusBadge status={selectedTicket.status} /></span>
                  </div>
                  <div className="mt-4 grid gap-2.5 sm:grid-cols-3"><InfoTile label={t("customerTickets.priority")} value={selectedTicket.priority} /><InfoTile label={t("customerTickets.currentEngineer")} value={selectedTicket.assignee_name || t("customerTickets.unassigned")} /><InfoTile label={t("customerTickets.createdAt")} value={formatDateTime(selectedTicket.created_at, locale)} /></div>
                </div>

                <div className="grid gap-4 p-4 lg:p-5 xl:grid-cols-[minmax(0,1fr)_300px]">
                  <div className="order-2 xl:order-none">
                    <div className="mb-4 flex items-center gap-2.5"><span className="grid size-8 place-items-center rounded-lg bg-primary/10 text-primary"><WrenchIcon className="size-4" /></span><h3 className="text-sm font-semibold text-foreground">{t("customerTickets.timelineTitle")}</h3></div>
                    {selectedTicket.progress.length === 0 ? <div className="rounded-lg border border-dashed border-border p-6 text-center text-sm text-muted-foreground">{t("customerTickets.noProgress")}</div> : (
                      <div
                        className="max-h-[min(520px,60vh)] overflow-y-auto overscroll-contain pr-3 [scrollbar-gutter:stable] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40"
                        role="region"
                        aria-label={t("customerTickets.timelineTitle")}
                        tabIndex={0}
                      >
                        <div className="relative space-y-0 before:absolute before:bottom-4 before:left-[7px] before:top-4 before:w-px before:bg-muted">
                          {selectedTicket.progress.map((item) => <div key={item.id} className="relative grid grid-cols-[16px_minmax(0,1fr)] gap-3 pb-6 last:pb-0"><span className="relative z-10 mt-1 size-4 rounded-full border-4 border-white bg-primary shadow-sm" /><div><div className="text-sm font-semibold text-foreground">{customerTicketProgressLabel(t, item)}</div><p className="mt-1.5 text-sm leading-5 text-muted-foreground">{customerTicketProgressContent(t, item, { hasDeviceConcept })}</p><div className="mt-1.5 text-xs text-muted-foreground">{formatDateTime(item.created_at, locale)}</div></div></div>)}
                        </div>
                      </div>
                    )}
                  </div>

                  <aside className="order-1 space-y-4 xl:order-none">
                    <div className="rounded-lg border border-border bg-card p-4"><div className="flex items-center gap-2 text-sm font-semibold text-foreground"><CheckCircle2Icon className="size-4" />{t("customerTickets.repairResult")}</div><p className={cn("mt-2 text-sm leading-5", selectedTicket.repair_summary ? "text-foreground" : "text-muted-foreground")}>{selectedTicket.repair_summary || t("customerTickets.noRepairResult")}</p></div>
                    <div className="rounded-lg border border-border p-4">
                      <h3 className="text-sm font-semibold text-foreground">{t("customerTickets.nextStep")}</h3>
                      <div className="mt-3 space-y-3">
                      {selectedTicket.current_meeting_id ? <Button className="w-full" onClick={() => router.push(`/customer/meeting?meetingId=${selectedTicket.current_meeting_id}`)}><VideoIcon className="mr-2 size-4" />{t("customerTickets.joinMeeting")}</Button> : <div className="rounded-lg bg-muted/50 px-4 py-3 text-sm text-muted-foreground">{t("customerTickets.noMeeting")}</div>}
                      {selectedTicket.feedback ? <div className="rounded-lg border border-border p-4"><div className="flex items-center gap-1" aria-label={t("customerTickets.ratingLabel", { rating: selectedTicket.feedback.rating })}>{[1, 2, 3, 4, 5].map((value) => <StarIcon key={value} className={cn("size-4", value <= selectedTicket.feedback!.rating ? "fill-amber-400 text-amber-400" : "text-muted-foreground")} />)}</div>{selectedTicket.feedback.comment ? <p className="mt-3 text-sm leading-6 text-muted-foreground">{selectedTicket.feedback.comment}</p> : null}</div> : null}
                      {selectedTicket.can_rate ? <div><div className="mb-2 text-sm font-medium text-foreground">{t("customerTickets.serviceRating")}</div><div className="flex gap-1">{[1, 2, 3, 4, 5].map((value) => <Button key={value} type="button" size="icon" variant="ghost" className="size-9" aria-label={t("customerTickets.starRatingLabel", { rating: value })} onClick={() => setRating(value)}><StarIcon className={cn("size-5", value <= rating ? "fill-amber-400 text-amber-400" : "text-muted-foreground")} /></Button>)}</div></div> : null}
                      {selectedTicket.can_rate || selectedTicket.can_confirm ? <Input aria-label={t("customerTickets.feedbackCommentLabel")} className="h-11" value={feedbackComment} onChange={(event) => setFeedbackComment(event.target.value)} placeholder={t("customerTickets.feedbackCommentLabel")} /> : null}
                      {selectedTicket.can_reopen ? <Input aria-label={t("customerTickets.reopenReasonLabel")} className="h-11" value={reopenReason} onChange={(event) => setReopenReason(event.target.value)} placeholder={t("customerTickets.reopenReasonLabel")} /> : null}
                      {selectedTicket.can_rate || selectedTicket.can_confirm ? <Button className="w-full" onClick={handleConfirmTicket} disabled={Boolean(ticketAction) || selectedTicket.can_rate && rating === 0}>{ticketAction === "confirm" ? <Loader2Icon className="mr-2 size-4 animate-spin" /> : <StarIcon className="mr-2 size-4" />}{selectedTicket.can_confirm ? t("customerTickets.confirmResolvedAction") : t("customerTickets.submitFeedbackAction")}</Button> : null}
                      {selectedTicket.can_reopen ? <Button className="w-full" variant="outline" onClick={handleReopenTicket} disabled={Boolean(ticketAction) || !reopenReason.trim()}><RotateCcwIcon className="mr-2 size-4" />{t("customerTickets.continueProcessing")}</Button> : null}
                    </div>
                  </div>
                </aside>
              </div>
            </div>
          )}
        </section>
        </div>
      )}
    </div>
  )
}

function CustomerTicketWorkspaceLoading() {
  const t = useI18n()
  return (
    <div className="grid gap-4 lg:grid-cols-[280px_minmax(0,1fr)] xl:grid-cols-[320px_minmax(0,1fr)]" role="status" aria-busy="true" aria-label={t("customerTickets.loadingWorkspace")}>
      <section className="min-w-0 overflow-hidden rounded-lg border border-border bg-card">
        <div className="space-y-2.5 border-b border-border p-3">
          <div className="space-y-2">
            <Skeleton className="h-4 w-20" />
            <Skeleton className="h-3 w-36" />
          </div>
          <div className="grid grid-cols-4 gap-1 rounded-lg bg-muted p-1">
            {Array.from({ length: 4 }).map((_, index) => <Skeleton key={index} className="h-8 rounded-md" />)}
          </div>
          <Skeleton className="h-8 rounded-md" />
        </div>
        <div className="h-[min(360px,48vh)] space-y-1 p-2.5 lg:h-[calc(100dvh-260px)] lg:min-h-[420px] lg:max-h-[620px]">
          {Array.from({ length: 7 }).map((_, index) => <Skeleton key={index} className="h-20 rounded-lg" />)}
        </div>
      </section>
      <section className="hidden min-w-0 overflow-hidden rounded-md border border-border bg-card lg:block">
        <div className="border-b border-border p-4 lg:p-5">
          <Skeleton className="h-3 w-28" />
          <Skeleton className="mt-2 h-6 w-64 max-w-full" />
          <Skeleton className="mt-2 h-3 w-72 max-w-full" />
          <div className="mt-4 grid gap-2.5 sm:grid-cols-3">
            {Array.from({ length: 3 }).map((_, index) => <Skeleton key={index} className="h-16 rounded-lg" />)}
          </div>
        </div>
        <div className="grid gap-4 p-4 lg:p-5 xl:grid-cols-[minmax(0,1fr)_300px]">
          <div className="space-y-4">
            <Skeleton className="h-10 w-48 max-w-full" />
            <Skeleton className="h-24 rounded-lg" />
            <Skeleton className="h-24 rounded-lg" />
            <Skeleton className="h-20 rounded-lg" />
          </div>
          <aside className="space-y-4">
            <Skeleton className="h-28 rounded-lg" />
            <Skeleton className="h-48 rounded-lg" />
          </aside>
        </div>
      </section>
    </div>
  )
}

function CustomerTicketDetailLoading() {
  const t = useI18n()
  return (
    <div className="space-y-5 p-4 lg:p-5" role="status" aria-busy="true" aria-label={t("customerTickets.loadingDetail")}>
      <div className="border-b border-border pb-4">
        <Skeleton className="h-3 w-24" />
        <Skeleton className="mt-2 h-6 w-64 max-w-full" />
        <Skeleton className="mt-2 h-3 w-72 max-w-full" />
        <div className="mt-4 grid gap-2.5 sm:grid-cols-3">
          {Array.from({ length: 3 }).map((_, index) => <Skeleton key={index} className="h-16 rounded-lg" />)}
        </div>
      </div>
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_300px]">
        <div className="space-y-4">
          <Skeleton className="h-10 w-48 max-w-full" />
          <Skeleton className="h-32 rounded-lg" />
          <Skeleton className="h-24 rounded-lg" />
        </div>
        <aside className="space-y-4">
          <Skeleton className="h-28 rounded-lg" />
          <Skeleton className="h-44 rounded-lg" />
        </aside>
      </div>
    </div>
  )
}

function InfoTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-border bg-muted/70 p-2.5">
      <div className="text-xs font-medium text-muted-foreground">{label}</div>
      <div className="mt-0.5 text-sm font-semibold text-foreground">{value || "—"}</div>
    </div>
  )
}

function customerMeetingDuration(t: I18nT, seconds?: number) {
  const total = Math.max(0, Number(seconds || 0))
  if (total < 1) return t("customerMeeting.durationLessThanOneSecond")
  if (total < 60) {
    const count = Math.floor(total)
    return t(count === 1 ? "customerMeeting.durationSecond" : "customerMeeting.durationSeconds", { count })
  }
  const minutes = Math.floor(total / 60)
  if (minutes < 60) {
    return t(minutes === 1 ? "customerMeeting.durationMinute" : "customerMeeting.durationMinutes", { count: minutes })
  }
  const hours = Math.floor(minutes / 60)
  const remainingMinutes = minutes % 60
  const hourLabel = t(hours === 1 ? "customerMeeting.durationHour" : "customerMeeting.durationHours")
  const minuteLabel = t(remainingMinutes === 1 ? "customerMeeting.durationMinute" : "customerMeeting.durationMinutes")
  return `${hours} ${hourLabel} ${remainingMinutes} ${minuteLabel}`
}

function customerMeetingSubject(meeting: CustomerPortalMeeting, fallback: string) {
  return meeting.title || meeting.ticket_no || fallback
}

function customerTranscriptTime(offsetMs: number, startedAt?: string, locale = "zh-CN") {
  if (offsetMs > 0 && offsetMs < 24 * 60 * 60 * 1000) {
    const seconds = Math.floor(offsetMs / 1000)
    return `${String(Math.floor(seconds / 60)).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`
  }
  if (offsetMs > 0) {
    return new Intl.DateTimeFormat(locale, {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    }).format(new Date(offsetMs))
  }
  return formatDateTime(startedAt, locale)
}

function isSystemMeetingTranscript(item: MeetingTranscriptSegment) {
  return item.provider?.toLowerCase() === "system" || item.ingestSource?.toLowerCase() === "system"
}

function CustomerMeetingRecord({
  meeting,
  transcripts,
  loading,
  error,
  onRetry,
  compact = false,
}: {
  meeting: CustomerPortalMeeting
  transcripts: MeetingTranscriptSegment[]
  loading: boolean
  error: string
  onRetry: () => void
  compact?: boolean
}) {
  const t = useI18n()
  const { locale } = useAppLocale()
  const [activeTab, setActiveTab] = useState("summary")
  const archived = !new Set(["active", "waiting"]).has(meeting.status)
  const realtimeTranscripts = transcripts.filter((item) => !isSystemMeetingTranscript(item))
  const systemArchives = transcripts.filter(isSystemMeetingTranscript)
  const transcriptCount = systemArchives.length > 0 ? realtimeTranscripts.length : Math.max(meeting.transcript_count || 0, realtimeTranscripts.length)
  const archiveBadge = archived ? (transcriptCount > 0 ? t("customerMeeting.archiveBadgeArchived") : systemArchives.length > 0 ? t("customerMeeting.archiveBadgeSystemOnly") : t("customerMeeting.archiveBadgeSummary")) : t("customerMeeting.archiveBadgeSyncing")
  const transcriptSummary = transcriptCount > 0
    ? t("customerMeeting.transcriptSummaryArchived", { count: transcriptCount })
    : systemArchives.length > 0
      ? t("customerMeeting.transcriptSummarySystemOnly")
      : t("customerMeeting.transcriptSummaryEmpty")
  const hasRecordContent = realtimeTranscripts.length > 0 || systemArchives.length > 0 || Boolean(error)
  const transcriptInitialLoading = loading && !hasRecordContent
  const recordRefreshing = loading && hasRecordContent

  const recordTabs = useMemo<RailopsTabItem[]>(() => [
    { value: "summary", label: t("customerMeeting.tabsSummary"), icon: <MessageSquareTextIcon className="size-4" /> },
    { value: "participants", label: t("customerMeeting.tabsParticipantsLabel"), count: meeting.participants?.length ?? 0, icon: <UsersRoundIcon className="size-4" /> },
    { value: "transcript", label: t("customerMeeting.tabsTranscriptLabel"), count: transcriptCount, icon: <FileTextIcon className="size-4" /> },
  ], [t, meeting.participants?.length, transcriptCount])

  return (
    <div className={cn("min-w-0", compact ? "xl:border-l xl:border-border xl:pl-5" : "border-t border-border")}>
      <div className={cn(compact ? "px-0" : "px-6 lg:px-8")}>
        <UnderlineTabs
          ariaLabel={t("customerMeeting.recordTabsAria")}
          items={recordTabs}
          value={activeTab}
          onChange={setActiveTab}
        />
      </div>

      {recordRefreshing ? (
        <div className={cn("border-b border-border bg-muted/40 py-2 text-xs text-muted-foreground", compact ? "px-0" : "px-6 lg:px-8")} role="status" aria-busy="true">
          {t("common.loading")}
        </div>
      ) : null}
      <>
        {error ? <ErrorState title={t("customerMeeting.recordLoadFailed")} description={error} action={{ label: t("customerMeeting.retry"), onClick: onRetry }} className={compact ? "my-4" : "m-6"} /> : null}
        {activeTab === "summary" ? (
          <div className={cn(compact ? "space-y-4 py-4" : "space-y-6 p-6 lg:p-8")}>
          <section>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <h3 className="font-semibold text-foreground">{t("customerMeeting.summaryTitle")}</h3>
              <Badge variant={archived && transcriptCount > 0 ? "secondary" : "outline"}>{archiveBadge}</Badge>
            </div>
            <p className={cn("border-l-2 border-primary pl-3 text-sm text-foreground", compact ? "mt-3 leading-6" : "mt-4 leading-7")}>
              {t("customerMeeting.summarySentence", {
                subject: customerMeetingSubject(meeting, t("customerMeeting.subjectFallback")),
                duration: customerMeetingDuration(t, meeting.duration_seconds),
                participants: meeting.participant_count || 0,
                transcriptSummary,
                annotationCount: meeting.annotation_count || 0,
              })}
            </p>
          </section>
          <dl className={cn("grid border-border", compact ? "grid-cols-2 gap-3 border-t pt-4" : "gap-4 border-y py-5 sm:grid-cols-2 xl:grid-cols-4")}>
            <div><dt className="text-xs text-muted-foreground">{t("customerMeeting.durationLabel")}</dt><dd className="mt-1 text-sm font-semibold text-foreground">{customerMeetingDuration(t, meeting.duration_seconds)}</dd></div>
            <div><dt className="text-xs text-muted-foreground">{t("customerMeeting.participantLabel")}</dt><dd className="mt-1 text-sm font-semibold text-foreground">{t("customerMeeting.participantCount", { count: meeting.participant_count || 0 })}</dd></div>
            <div><dt className="text-xs text-muted-foreground">{t("customerMeeting.transcriptLabel")}</dt><dd className="mt-1 text-sm font-semibold text-foreground">{t("customerMeeting.transcriptCount", { count: transcriptCount })}</dd></div>
            <div><dt className="text-xs text-muted-foreground">{t("customerMeeting.endedAtLabel")}</dt><dd className="mt-1 text-sm font-semibold text-foreground">{formatDateTime(meeting.ended_at, locale)}</dd></div>
          </dl>
          </div>
        ) : null}
        {activeTab === "participants" ? (
          <div className={cn(compact ? "py-4" : "p-6 lg:p-8")}>
            <div className="mb-4 flex items-center justify-between gap-3">
              <h3 className="font-semibold text-foreground">{t("customerMeeting.participantsTitle")}</h3>
              <span className="text-xs text-muted-foreground">{t("customerMeeting.sortedByJoinTime")}</span>
            </div>
            <MeetingParticipantList participants={meeting.participants ?? []} meetingStatus={meeting.status} />
          </div>
        ) : null}
        {activeTab === "transcript" ? (
          transcriptInitialLoading ? (
            <div className={cn("space-y-3", compact ? "py-4" : "p-6 lg:p-8")} role="status" aria-busy="true" aria-label={t("common.loading")}>
              <Skeleton className="h-5 w-36" />
              <Skeleton className="h-20 w-full" />
              <Skeleton className="h-28 w-full" />
            </div>
          ) : realtimeTranscripts.length === 0 ? (
            <div className={cn("text-center", compact ? "py-8" : "px-6 py-12 lg:px-8")}>
              <FileTextIcon className="mx-auto size-7 text-muted-foreground" />
              <h3 className="mt-3 text-sm font-semibold text-foreground">{t("customerMeeting.noRealtimeTranscriptTitle")}</h3>
              {systemArchives[0]?.text ? (
                <p className="mx-auto mt-4 max-w-xl rounded-md border border-border bg-muted/50 px-4 py-3 text-left text-sm leading-6 text-muted-foreground">
                  {systemArchives[0].text}
                </p>
              ) : null}
            </div>
          ) : (
            <div className={cn("divide-y divide-border overflow-y-auto", compact ? "max-h-[360px]" : "max-h-[560px]")}>
              {realtimeTranscripts.map((item) => (
                <article key={item.id} className={cn("grid gap-2 py-4 sm:grid-cols-[96px_minmax(0,1fr)]", compact ? "px-0" : "px-6 lg:px-8")}>
                  <div className="text-xs text-muted-foreground">
                    <div className="font-medium text-foreground">{item.speakerName || t("customerMeeting.unknownSpeaker")}</div>
                    <div className="mt-1 tabular-nums">{customerTranscriptTime(item.startedAtMs, meeting.started_at, locale)}</div>
                  </div>
                  <div className="min-w-0">
                    <p className="text-sm leading-6 text-foreground">{item.text}</p>
                    {item.translatedText ? <p className="mt-2 border-l-2 border-primary bg-muted/50 px-3 py-2 text-sm leading-6 text-foreground">{item.translatedText}</p> : null}
                  </div>
                </article>
              ))}
            </div>
          )
        ) : null}
      </>
    </div>
  )
}

export function CustomerMeetingPortalPage() {
  const { session } = useAuth()
  const t = useI18n()
  const { locale } = useAppLocale()
  const router = useCustomerPortalRouter()
  const searchParams = useSearchParams()
  const [meetings, setMeetings] = useState<CustomerPortalMeeting[]>([])
  const [meetingTotal, setMeetingTotal] = useState(0)
  const [selectedMeetingDetail, setSelectedMeetingDetail] = useState<CustomerPortalMeeting | null>(null)
  const [selectedMeetingLoading, setSelectedMeetingLoading] = useState(false)
  const [profile, setProfile] = useState<CustomerPortalProfile | null>(null)
  const [loading, setLoading] = useState(true)
  const [selectedId, setSelectedId] = useState("")
  const [mobileMeetingListOpen, setMobileMeetingListOpen] = useState(() => !searchParams?.get("meetingId"))
  const [joinConfig, setJoinConfig] = useState<CustomerMeetingJoinConfig | null>(null)
  const [meetingTranscripts, setMeetingTranscripts] = useState<MeetingTranscriptSegment[]>([])
  const [meetingTranscriptsLoading, setMeetingTranscriptsLoading] = useState(false)
  const [meetingTranscriptsError, setMeetingTranscriptsError] = useState("")
  const [meetingRecordRevision, setMeetingRecordRevision] = useState(0)
  const [joining, setJoining] = useState(false)
  const [meetingFilter, setMeetingFilter] = useState<"upcoming" | "history">("upcoming")
  const [meetingPage, setMeetingPage] = useState(1)
  const [meetingLimit, setMeetingLimit] = useState(20)
  const [meetingQueryOverride, setMeetingQueryOverride] = useState<string | null>(null)

  const selectedMeeting = selectedMeetingDetail?.id === selectedId
    ? selectedMeetingDetail
    : meetings.find((item) => item.id === selectedId) ?? null
  const upcomingMeetingStatuses = new Set(["active", "waiting"])
  const urlMeetingQueryParam = searchParams?.get("meetingId") || ""
  const meetingQueryParam = meetingQueryOverride ?? urlMeetingQueryParam
  const requestedMeetingId = meetingFilter === "upcoming" ? meetingQueryParam : ""
  const hasDeviceConcept = session?.featureFlags?.device !== false

  useEffect(() => {
    if (meetingQueryOverride !== null && meetingQueryOverride === urlMeetingQueryParam) {
      setMeetingQueryOverride(null)
    }
  }, [meetingQueryOverride, urlMeetingQueryParam])

  useEffect(() => {
    let cancelled = false

    async function loadData() {
      setLoading(true)
      setJoinConfig(null)
      try {
        void fetchCustomerProfile().then(setProfile).catch(() => undefined)
        const meetingData = await fetchCustomerMeetingsPage({
          page: meetingPage,
          limit: meetingLimit,
          filter: meetingFilter,
          meetingId: requestedMeetingId || undefined,
        })
        if (cancelled) return
        setMeetings(meetingData.results ?? [])
        setMeetingTotal(meetingData.page?.total ?? 0)
        const nextSelectedId = meetingData.results?.[0]?.id || ""
        setSelectedId((current) => (current === nextSelectedId ? current : nextSelectedId))
      } catch (error) {
        if (cancelled) return
        toast.error(error instanceof Error ? error.message : t("customerMeeting.loadFailed"))
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void loadData()
    return () => {
      cancelled = true
    }
  }, [meetingFilter, meetingLimit, meetingPage, requestedMeetingId, t])

  useEffect(() => {
    if (!selectedId) {
      setSelectedMeetingDetail(null)
      setSelectedMeetingLoading(false)
      return
    }
    const selectedMeetingInPage = meetings.find((item) => item.id === selectedId)
    if (selectedMeetingInPage) {
      setSelectedMeetingDetail(selectedMeetingInPage)
      setSelectedMeetingLoading(false)
      return
    }
    setSelectedMeetingDetail(null)
    let cancelled = false
    setSelectedMeetingLoading(true)
    void fetchCustomerMeetingsPage({ meetingId: selectedId, limit: 1 })
      .then((page) => {
        if (!cancelled) {
          setSelectedMeetingDetail(page.results?.[0] ?? null)
        }
      })
      .catch((error) => {
        if (!cancelled) {
          toast.error(error instanceof Error ? error.message : t("customerMeeting.loadFailed"))
        }
      })
      .finally(() => {
        if (!cancelled) {
          setSelectedMeetingLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [meetingRecordRevision, meetings, selectedId, t])

  useEffect(() => {
    if (loading) {
      return
    }
    if (selectedId && meetingQueryParam !== selectedId) {
      router.replace(`/customer/meeting?meetingId=${selectedId}`)
    } else if (!selectedId && meetingQueryParam) {
      router.replace("/customer/meeting")
    }
    setJoinConfig(null)
  }, [loading, meetingQueryParam, router, selectedId])

  useEffect(() => {
    let active = true
    if (!selectedId) {
      setMeetingTranscripts([])
      setMeetingTranscriptsError("")
      setMeetingTranscriptsLoading(false)
      return () => {
        active = false
      }
    }
    setMeetingTranscripts([])
    setMeetingTranscriptsError("")
    setMeetingTranscriptsLoading(true)
    fetchCustomerMeetingTranscripts(selectedId)
      .then((items) => {
        if (active) setMeetingTranscripts(items)
      })
      .catch((error) => {
        if (active) setMeetingTranscriptsError(error instanceof Error ? error.message : t("customerMeeting.recordLoadFailed"))
      })
      .finally(() => {
        if (active) setMeetingTranscriptsLoading(false)
      })
    return () => {
      active = false
    }
  }, [meetingRecordRevision, selectedId])

  async function handleJoin() {
    if (!selectedId) {
      return
    }
    setJoining(true)
    try {
      const config = await fetchCustomerMeetingJoinConfig(selectedId)
      setJoinConfig(config)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("customerMeeting.joinFailed"))
    } finally {
      setJoining(false)
    }
  }

  function handleMeetingFilterChange(nextFilter: "upcoming" | "history") {
    setMeetingFilter(nextFilter)
    setMeetingPage(1)
    setSelectedId("")
    setSelectedMeetingDetail(null)
    setMeetings([])
    setMeetingTotal(0)
    setJoinConfig(null)
    setMeetingQueryOverride("")
    setMobileMeetingListOpen(true)
    router.replace("/customer/meeting")
  }

  const hasSelectedMeeting = Boolean(selectedMeeting)
  const meetingDetailPending = loading && !hasSelectedMeeting

  return (
    <div className="rhd-railops-customer-page rhd-railops-customer-meeting-page space-y-4">
      <PageHeader title={t("customerMeeting.pageTitle")} />
      <div className="grid gap-5 lg:grid-cols-[260px_minmax(0,1fr)] xl:grid-cols-[300px_minmax(0,1fr)]">
        <section className={cn("min-w-0 overflow-hidden rounded-md border border-border bg-card", mobileMeetingListOpen ? "block" : "hidden lg:block")}>
          <div className="space-y-3 border-b border-border p-4">
            <div><h2 className="font-semibold text-foreground">{t("customerMeeting.scheduleTitle")}</h2><p className="mt-1 text-sm text-muted-foreground">{t("customerMeeting.listSummary", { count: meetingTotal })}</p></div>
            <UnderlineTabs
              ariaLabel={t("customerMeeting.scheduleTitle")}
              value={meetingFilter}
              items={[
                { value: "upcoming", label: t("customerMeeting.filterUpcoming") },
                { value: "history", label: t("customerMeeting.filterHistory") },
              ]}
              onChange={(value) => handleMeetingFilterChange(value as "upcoming" | "history")}
            />
          </div>
          <ScrollArea className="h-[min(360px,48vh)] p-3 lg:h-[calc(100dvh-260px)] lg:min-h-[420px] lg:max-h-[620px]">
            <div className="space-y-1.5 pr-3">
              {loading ? Array.from({ length: 5 }).map((_, index) => <Skeleton key={index} className="h-24 rounded-lg" />) : null}
              {!loading && meetings.length === 0 ? <EmptyCard icon={<CalendarClockIcon className="size-6" />} title={meetingFilter === "upcoming" ? t("customerMeeting.noUpcomingTitle") : t("customerMeeting.noHistoryTitle")} /> : null}
              {meetings.map((item) => (
                <button key={item.id} type="button" onClick={() => { setSelectedId(item.id); setMobileMeetingListOpen(false) }} className={cn("w-full rounded-md border p-3 text-left transition", selectedId === item.id ? "border-primary/20 bg-primary/10 shadow-sm" : "border-transparent hover:border-border hover:bg-muted/50")}>
                  <div className="flex items-start justify-between gap-3"><div className="min-w-0"><div className="truncate text-sm font-semibold text-foreground">{item.ticket_no || item.title}</div><div className="mt-1 truncate text-sm text-muted-foreground">{hasDeviceConcept ? item.device_no || item.product_name || t("customerMeeting.meetingFallback") : t("customerMeeting.meetingFallback")}</div></div><StatusBadge status={item.status} /></div>
                  <div className="mt-2 flex items-center gap-2 text-xs text-muted-foreground"><CalendarClockIcon className="size-3.5" />{formatDateTime(item.scheduled_at || item.started_at, locale)}</div>
                </button>
              ))}
            </div>
          </ScrollArea>
          <div className="border-t border-border px-3 py-3">
            <ListPagination
              page={meetingPage}
              total={meetingTotal}
              limit={meetingLimit}
              loading={loading}
              onPageChange={(page) => setMeetingPage(page)}
              onLimitChange={(limit) => {
                setMeetingLimit(limit)
                setMeetingPage(1)
              }}
            />
          </div>
        </section>

        <section className={cn("min-w-0 overflow-hidden rounded-md border border-border bg-card", mobileMeetingListOpen ? "hidden lg:block" : "block")}>
          {selectedMeetingLoading && selectedId && selectedMeeting?.id !== selectedId ? <CustomerMeetingDetailLoading /> : meetingDetailPending ? <CustomerMeetingDetailLoading /> : !selectedMeeting ? <EmptyCard icon={<VideoIcon className="size-6" />} title={t("customerMeeting.selectTitle")} /> : joinConfig ? (
            <div className="space-y-5 p-5 lg:p-6">
              <div className="flex items-center justify-between rounded-lg border border-border bg-card px-4 py-3 text-sm text-foreground"><span>{t("customerMeeting.connectionEstablished")}</span><StatusBadge status="active" /></div>
              <MeetingLiveRoom domain={joinConfig.domain} jitsiUrl={joinConfig.jitsiUrl} meetingId={joinConfig.meetingId} roomName={joinConfig.roomName} subject={selectedMeeting.ticket_no || selectedMeeting.title || t("customerMeeting.subjectFallback")} jwt={joinConfig.jwt} transcriptionEnabled={joinConfig.transcriptionEnabled ?? false} transcriptionProvider={joinConfig.transcriptionProvider} transcriptionReady={joinConfig.transcriptionReady ?? false} initialTranscripts={meetingTranscripts} fetchTranscripts={() => fetchCustomerMeetingTranscripts(joinConfig.meetingId)} fetchTranscriptPage={(cursor) => fetchCustomerMeetingTranscriptPage(joinConfig.meetingId, cursor)} transcriptionUnavailableReason={joinConfig.transcriptionError} arEnabled={joinConfig.arDetectionEnabled ?? false} customerInviteEnabled={false} displayName={profile?.name || t("customerMeeting.displayNameFallback")} onConferenceJoined={() => confirmCustomerMeetingJoined(joinConfig.meetingId)} onConferenceLeft={() => confirmCustomerMeetingLeft(joinConfig.meetingId)} onConferenceHeartbeat={() => heartbeatCustomerMeeting(joinConfig.meetingId)} onTranscriptFinal={(event) => ingestCustomerMeetingTranscript(joinConfig.meetingId, event)} fetchMeetingStatus={() => fetchCustomerMeetingStatus(joinConfig.meetingId)} onMeetingEnd={() => setJoinConfig(null)} className="h-[calc(100dvh-230px)] min-h-[520px] overflow-hidden rounded-md lg:min-h-[640px]" />
            </div>
          ) : (
            <div>
              <div className="border-b border-border px-5 py-4 lg:px-6">
                <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between"><div className="flex min-w-0 items-start gap-2"><span className="shrink-0 lg:hidden"><IconButton icon={<ArrowLeftIcon className="size-4" />} aria-label={t("common.back")} tooltip={t("common.back")} onClick={() => setMobileMeetingListOpen(true)} /></span><div className="min-w-0"><div className="text-sm font-semibold text-primary">{selectedMeeting.ticket_no || t("customerMeeting.ticketFallback")}</div><h2 className="mt-1.5 text-xl font-semibold text-foreground">{selectedMeeting.title || t("customerMeeting.titleFallback")}</h2><p className="mt-1.5 text-sm text-muted-foreground">{hasDeviceConcept ? selectedMeeting.device_no || selectedMeeting.product_name || t("customerMeeting.serviceFallback") : t("customerMeeting.serviceFallback")}</p></div></div><StatusBadge status={selectedMeeting.status} /></div>
                <dl className="mt-4 grid gap-3 border-t border-border pt-3 sm:grid-cols-3">
                  <div className="min-w-0"><dt className="text-xs text-muted-foreground">{t("customerMeeting.plannedTime")}</dt><dd className="mt-1 truncate text-sm font-medium text-foreground">{formatDateTime(selectedMeeting.scheduled_at, locale)}</dd></div>
                  <div className="min-w-0"><dt className="text-xs text-muted-foreground">{t("customerMeeting.hostLabel")}</dt><dd className="mt-1 truncate text-sm font-medium text-foreground">{selectedMeeting.created_by || t("customerMeeting.hostFallback")}</dd></div>
                  <div className="min-w-0"><dt className="text-xs text-muted-foreground">{t("customerMeeting.participantsCountLabel")}</dt><dd className="mt-1 text-sm font-medium text-foreground">{selectedMeeting.participant_count || 0}</dd></div>
                </dl>
              </div>
              {upcomingMeetingStatuses.has(selectedMeeting.status) ? (
                <div className="grid gap-5 p-5 xl:grid-cols-[minmax(0,1fr)_280px] 2xl:grid-cols-[minmax(0,1fr)_minmax(300px,360px)] lg:p-6">
                  <MeetingLobby
                    onJoin={handleJoin}
                    isJoining={joining}
                    joinRequirementText={t("customerMeeting.joinRequirementText")}
                  />
                  <CustomerMeetingRecord
                    meeting={selectedMeeting}
                    transcripts={meetingTranscripts}
                    loading={meetingTranscriptsLoading}
                    error={meetingTranscriptsError}
                    onRetry={() => setMeetingRecordRevision((value) => value + 1)}
                    compact
                  />
                </div>
              ) : (
                <CustomerMeetingRecord
                  meeting={selectedMeeting}
                  transcripts={meetingTranscripts}
                  loading={meetingTranscriptsLoading}
                  error={meetingTranscriptsError}
                  onRetry={() => setMeetingRecordRevision((value) => value + 1)}
                />
              )}
            </div>
          )}
        </section>
      </div>
    </div>
  )
}

function CustomerMeetingDetailLoading() {
  const t = useI18n()
  return (
    <div
      className="space-y-5 p-5 lg:p-6"
      role="status"
      aria-busy="true"
      aria-label={t("customerMeeting.loadingDetail")}
    >
      <div className="flex items-center justify-between gap-4 rounded-lg border border-border bg-card px-4 py-3">
        <Skeleton className="h-5 w-36" />
        <Skeleton className="h-6 w-16 rounded-full" />
      </div>
      <div className="grid gap-3 sm:grid-cols-3">
        {Array.from({ length: 3 }).map((_, index) => (
          <Skeleton key={index} className="h-20 rounded-lg" />
        ))}
      </div>
      <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_300px]">
        <div className="space-y-3 rounded-lg border border-border p-5">
          <Skeleton className="h-6 w-44 max-w-full" />
          <Skeleton className="h-4 w-72 max-w-full" />
          <Skeleton className="h-44 w-full" />
        </div>
        <div className="space-y-4">
          <Skeleton className="h-40 rounded-lg" />
          <Skeleton className="h-36 rounded-lg" />
        </div>
      </div>
    </div>
  )
}

export function CustomerDevicesPortalPage() {
  const { locale } = useAppLocale()
  const router = useCustomerPortalRouter()
  const searchParams = useSearchParams()
  const t = useI18n()
  const deviceQueryParam = searchParams?.get("deviceId") || ""
  const [devices, setDevices] = useState<CustomerPortalDevice[]>([])
  const [deviceTotal, setDeviceTotal] = useState(0)
  const [devicePage, setDevicePage] = useState(1)
  const [deviceLimit, setDeviceLimit] = useState(10)
  const [selectedDeviceDetail, setSelectedDeviceDetail] = useState<CustomerPortalDevice | null>(null)
  const [selectedDeviceLoading, setSelectedDeviceLoading] = useState(false)
  const [conversations, setConversations] = useState<CustomerPortalConversation[]>([])
  const [conversationsLoading, setConversationsLoading] = useState(false)
  const [deviceConversationTotal, setDeviceConversationTotal] = useState(0)
  const [tickets, setTickets] = useState<CustomerPortalTicket[]>([])
  const [ticketsLoading, setTicketsLoading] = useState(false)
  const [meetings, setMeetings] = useState<CustomerPortalMeeting[]>([])
  const [meetingsLoading, setMeetingsLoading] = useState(false)
  const [deviceMeetingTotal, setDeviceMeetingTotal] = useState(0)
  const [manuals, setManuals] = useState<CustomerPortalManualFile[]>([])
  const [manualsDeviceId, setManualsDeviceId] = useState(0)
  const [manualsLoading, setManualsLoading] = useState(false)
  const [loading, setLoading] = useState(true)
  const [hasLoadedDevices, setHasLoadedDevices] = useState(false)
  const [loadError, setLoadError] = useState("")
  const [selectedId, setSelectedId] = useState(0)
  const [mobileDeviceListOpen, setMobileDeviceListOpen] = useState(() => !deviceQueryParam)
  const [loadVersion, setLoadVersion] = useState(0)
  const [bindOpen, setBindOpen] = useState(false)
  const [deviceTab, setDeviceTab] = useState<"overview" | "history" | "manuals">(() => {
    const tab = searchParams?.get("tab")
    return tab === "history" || tab === "manuals" ? tab : "overview"
  })
  const returnConversationId = normalizeCustomerConversationId(searchParams?.get("fromConversationId") || "")
  const returnConversationHref = buildCustomerChatReturnPath(returnConversationId)

  const selectedDeviceInPage = devices.find((item) => item.id === selectedId) ?? null
  const selectedDevice = selectedDeviceInPage ?? selectedDeviceDetail
  const selectedConversations = conversations
  const selectedTickets = tickets
  const selectedMeetings = meetings
  const hasSelectedManuals = manualsDeviceId === selectedId
  const selectedManuals = hasSelectedManuals ? manuals : []
  const initialDeviceLoad = !hasLoadedDevices && loading
  const manualsInitialLoading = manualsLoading && !hasSelectedManuals
  const deviceRefreshing = loading && hasLoadedDevices
  const deviceRelationsLoading = conversationsLoading || ticketsLoading || meetingsLoading
  const conversationsInitialLoading = conversationsLoading && conversations.length === 0
  const ticketsInitialLoading = ticketsLoading && tickets.length === 0
  const meetingsInitialLoading = meetingsLoading && meetings.length === 0
  const selectedDeviceDetailLoading = selectedDeviceLoading && selectedId > 0 && !selectedDevice

  useEffect(() => {
    async function loadData() {
      setLoading(true)
      setLoadError("")
      try {
        const devicePageData = await fetchCustomerDevicesPage({
          page: devicePage,
          limit: deviceLimit,
        })
        const deviceData = devicePageData.results ?? []
        setDevices(deviceData)
        setDeviceTotal(devicePageData.page?.total ?? deviceData.length)
        const queryId = Number(deviceQueryParam || "0")
      setSelectedId((current) => {
        if (queryId > 0) return queryId
        if (current > 0) return current
        return deviceData[0]?.id ?? 0
      })
      setHasLoadedDevices(true)
    } catch (error) {
        const message = error instanceof Error ? error.message : t("customerDevices.loadFailed")
        setLoadError(message)
        toast.error(message)
      } finally {
        setLoading(false)
      }
    }
    loadData()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [deviceLimit, devicePage, loadVersion])

  useEffect(() => {
    if (selectedId <= 0) {
      setSelectedDeviceDetail(null)
      setSelectedDeviceLoading(false)
      return
    }
    if (selectedDeviceInPage) {
      setSelectedDeviceDetail(null)
      setSelectedDeviceLoading(false)
      return
    }
    setSelectedDeviceDetail((current) => current?.id === selectedId ? current : null)
    let cancelled = false
    setSelectedDeviceLoading(true)
    void fetchCustomerDevicesPage({ deviceId: selectedId, limit: 1 })
      .then((page) => {
        if (!cancelled) {
          setSelectedDeviceDetail(page.results?.[0] ?? null)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setSelectedDeviceDetail(null)
        }
      })
      .finally(() => {
        if (!cancelled) {
          setSelectedDeviceLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [selectedDeviceInPage?.id, selectedId])

  useEffect(() => {
    if (selectedId <= 0) {
      setConversations([])
      setTickets([])
      setMeetings([])
      setDeviceConversationTotal(0)
      setDeviceMeetingTotal(0)
      setConversationsLoading(false)
      setTicketsLoading(false)
      setMeetingsLoading(false)
      return
    }
    let cancelled = false
    setConversations([])
    setTickets([])
    setMeetings([])
    setDeviceConversationTotal(0)
    setDeviceMeetingTotal(0)

    setConversationsLoading(true)
    void fetchCustomerConversationsPage({ deviceId: selectedId, limit: 20, locale })
      .then((conversationPageData) => {
        if (cancelled) return
        setConversations(conversationPageData.results ?? [])
        setDeviceConversationTotal(conversationPageData.page?.total ?? 0)
      })
      .catch((error) => {
        if (!cancelled) {
          setConversations([])
          setDeviceConversationTotal(0)
          toast.error(error instanceof Error ? error.message : t("customerDevices.loadFailed"))
        }
      })
      .finally(() => {
        if (!cancelled) setConversationsLoading(false)
      })

    setTicketsLoading(true)
    void fetchCustomerTicketsPage({ deviceId: selectedId, limit: 20 })
      .then((ticketPageData) => {
        if (cancelled) return
        setTickets(ticketPageData.results ?? [])
      })
      .catch((error) => {
        if (!cancelled) {
          setTickets([])
          toast.error(error instanceof Error ? error.message : t("customerDevices.loadFailed"))
        }
      })
      .finally(() => {
        if (!cancelled) setTicketsLoading(false)
      })

    setMeetingsLoading(true)
    void fetchCustomerMeetingsPage({ deviceId: selectedId, limit: 20 })
      .then((meetingPageData) => {
        if (cancelled) return
        setMeetings(meetingPageData.results ?? [])
        setDeviceMeetingTotal(meetingPageData.page?.total ?? 0)
      })
      .catch((error) => {
        if (!cancelled) {
          setMeetings([])
          setDeviceMeetingTotal(0)
          toast.error(error instanceof Error ? error.message : t("customerDevices.loadFailed"))
        }
      })
      .finally(() => {
        if (!cancelled) setMeetingsLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [loadVersion, locale, selectedId, t])

  function handleDeviceBound(binding: BindCustomerDeviceResponse) {
    const deviceId = Number(binding.deviceId || 0)
    if (deviceId > 0) {
      setSelectedId(deviceId)
      setMobileDeviceListOpen(false)
      setDevicePage(1)
    }
    setLoadVersion((value) => value + 1)
  }

  useEffect(() => {
    if (selectedId) {
      router.replace(buildCustomerDevicePath(selectedId, returnConversationId, deviceTab))
    }
  }, [deviceTab, returnConversationId, router, selectedId])

  useEffect(() => {
    let cancelled = false
    if (!selectedId) {
      setManuals([])
      setManualsDeviceId(0)
      return
    }
    setManualsLoading(true)
    void fetchCustomerDeviceManuals(selectedId)
      .then((items) => {
        if (!cancelled) {
          setManuals(items)
          setManualsDeviceId(selectedId)
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setManuals([])
          setManualsDeviceId(selectedId)
          toast.error(error instanceof Error ? error.message : t("customerDevices.loadFailed"))
        }
      })
      .finally(() => {
        if (!cancelled) setManualsLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [selectedId])

  return (
    <div className="rhd-railops-customer-page rhd-railops-customer-devices-page space-y-4">
      <PageHeader
        title={t("customerDevices.pageTitle")}
        action={(
        <>
              {returnConversationId > 0 ? (
            <Button type="button" variant="outline" onClick={() => router.push(returnConversationHref)}>
              <ArrowLeftIcon className="mr-2 size-4" />
              {t("customerDevices.backToConversation")}
            </Button>
          ) : null}
          <Button variant="outline" onClick={() => setLoadVersion((value) => value + 1)} disabled={loading}>
            <RefreshCwIcon className={cn("mr-2 size-4", loading && "animate-spin")} />
            {t("customerDevices.refresh")}
          </Button>
          <Button onClick={() => setBindOpen(true)}>
            <PlusIcon className="mr-2 size-4" />
            {t("customerDevices.addDevice")}
          </Button>
          </>
        )}
      />
      {loadError && hasLoadedDevices ? (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-border bg-muted px-4 py-3 text-sm text-muted-foreground" role="alert">
          <span>{t("customerDevices.refreshFailed")}</span>
          <Button variant="outline" size="sm" onClick={() => setLoadVersion((value) => value + 1)} disabled={loading}>{t("customerDevices.retry")}</Button>
        </div>
      ) : null}
      {!hasLoadedDevices && loadError && !loading ? (
        <ErrorState title={t("customerDevices.loadFailed")} description={loadError} action={{ label: t("customerDevices.retry"), onClick: () => setLoadVersion((value) => value + 1) }} />
      ) : (
      <div className="grid gap-5 lg:grid-cols-[280px_minmax(0,1fr)] xl:grid-cols-[340px_minmax(0,1fr)]">
        <section className={cn("min-w-0 overflow-hidden rounded-md border border-border bg-card", mobileDeviceListOpen ? "block" : "hidden lg:block")} aria-busy={loading}>
          <div className="border-b border-border p-4">
            <h2 className="font-semibold text-foreground">{t("customerDevices.deviceListTitle")}</h2>
            <div className="mt-1 min-h-5 text-sm text-muted-foreground" aria-live="polite">
              {initialDeviceLoad ? <><Skeleton className="inline-block h-4 w-28 align-middle" aria-hidden="true" /><span className="sr-only">{t("customerDevices.loadingCount")}</span></> : t("customerDevices.deviceCount", { count: deviceTotal })}
            </div>
          </div>
          {deviceRefreshing ? <div className="border-b border-border px-4 py-2 text-rhd-xs text-muted-foreground" role="status" aria-busy="true"><Loader2Icon className="mr-2 inline size-3.5 animate-spin" />{t("customerDevices.loadingList")}</div> : null}
          <ScrollArea className="h-[min(360px,48vh)] p-3 lg:h-[calc(100dvh-260px)] lg:min-h-[420px] lg:max-h-[620px]">
            <div className="space-y-1.5 pr-3">
              {initialDeviceLoad ? <CustomerDeviceListLoading /> : null}
              {hasLoadedDevices && devices.length === 0 ? <EmptyCard icon={<PackageSearchIcon className="size-6" />} title={t("customerDevices.emptyTitle")} action={<Button onClick={() => setBindOpen(true)}>{t("customerDevices.addDevice")}</Button>} /> : null}
              {devices.map((item) => (
                <button key={item.id} type="button" onClick={() => { setSelectedId(item.id); setMobileDeviceListOpen(false) }} className={cn("w-full rounded-md border p-3 text-left transition", selectedId === item.id ? "border-primary/30 bg-primary/5 shadow-sm" : "border-transparent hover:border-border hover:bg-muted/40")}>
                  <div className="flex items-start justify-between gap-3"><div className="min-w-0"><div className="truncate text-sm font-semibold text-foreground">{item.device_no}</div><div className="mt-1 truncate text-sm text-muted-foreground">{item.product_name || t("customerDevices.productFallback")} · {item.model_name || t("customerDevices.modelFallback")}</div></div><StatusBadge status={item.status} /></div>
                  <div className="mt-3 grid grid-cols-4 gap-1.5 text-center"><DeviceCount label={t("customerDevices.ticketCount")} value={item.open_ticket_count} /><DeviceCount label={t("customerDevices.conversationCount")} value={item.conversation_count} /><DeviceCount label={t("customerDevices.manualCount")} value={item.manual_count} /><DeviceCount label={t("customerDevices.repairCount")} value={item.repair_history_count} /></div>
                </button>
              ))}
            </div>
          </ScrollArea>
          <div className="border-t border-border px-3 py-3">
            <ListPagination
              page={devicePage}
              total={deviceTotal}
              limit={deviceLimit}
              loading={loading}
              pageSizeOptions={[10, 20, 50]}
              onPageChange={(page) => setDevicePage(page)}
              onLimitChange={(limit) => {
                setDeviceLimit(limit)
                setDevicePage(1)
              }}
            />
          </div>
        </section>

        <section className={cn("min-w-0 overflow-hidden rounded-md border border-border bg-card", mobileDeviceListOpen ? "hidden lg:block" : "block")} aria-busy={initialDeviceLoad || deviceRelationsLoading || selectedDeviceLoading}>
          {selectedDeviceDetailLoading ? <CustomerDeviceDetailLoading /> : initialDeviceLoad ? <CustomerDeviceDetailLoading /> : !selectedDevice ? <EmptyCard icon={<PackageSearchIcon className="size-6" />} title={t("customerDevices.selectDeviceTitle")} /> : (
            <div>
              <div className="border-b border-border p-5 lg:p-6">
                <div className="flex flex-col gap-5 sm:flex-row sm:items-start sm:justify-between">
                  <div className="flex min-w-0 items-start gap-2"><span className="shrink-0 lg:hidden"><IconButton icon={<ArrowLeftIcon className="size-4" />} aria-label={t("common.back")} tooltip={t("common.back")} onClick={() => setMobileDeviceListOpen(true)} /></span><div className="min-w-0"><div className="text-sm font-semibold text-primary">{selectedDevice.device_no}</div><h2 className="mt-1.5 text-xl font-semibold text-foreground">{selectedDevice.product_name || t("customerDevices.deviceArchive")}</h2><p className="mt-1.5 text-sm text-muted-foreground">{selectedDevice.model_name || t("customerDevices.modelFallback")} · {t("customerDevices.serialNumber")} {selectedDevice.serial_no || "—"}</p></div></div>
                  <div className="flex flex-wrap items-center gap-2"><StatusBadge status={selectedDevice.status} /><Button onClick={() => router.push(`/customer/chat?deviceId=${selectedDevice.id}&new=1`)}><MessageSquareTextIcon className="mr-2 size-4" />{t("customerDevices.startConsultation")}</Button></div>
                </div>
                <div className="mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4"><InfoTile label={t("customerDevices.region")} value={selectedDevice.region_code || "—"} /><InfoTile label={t("customerDevices.installedAt")} value={formatDateTime(selectedDevice.installed_at, locale)} /><InfoTile label={t("customerDevices.lastServiceAt")} value={formatDateTime(selectedDevice.last_service_at, locale)} /><InfoTile label={t("customerDevices.warrantyEndAt")} value={formatDateTime(selectedDevice.warranty_end_at, locale)} /></div>
              </div>

              <div className="border-b border-border px-5 lg:px-6">
                <UnderlineTabs
                  ariaLabel={t("customerDevices.deviceArchive")}
                  value={deviceTab}
                  items={[
                    { value: "overview", label: t("customerDevices.serviceOverview") },
                    { value: "history", label: t("customerDevices.serviceHistory") },
                    { value: "manuals", label: t("customerDevices.manuals") },
                  ]}
                  onChange={(value) => setDeviceTab(value as "overview" | "history" | "manuals")}
                />
              </div>

              <div className="p-5 lg:p-6">
                {deviceTab === "overview" ? (
                  <div className="space-y-6">
                    <div className="grid gap-4 md:grid-cols-3">
                      <ServiceSummary
                        icon={<MessagesSquareIcon className="size-5" />}
                        tone="blue"
                        label={t("customerDevices.recentConversations")}
                        value={deviceConversationTotal}
                        loading={conversationsInitialLoading}
                      />
                      <ServiceSummary
                        icon={<TicketCheckIcon className="size-5" />}
                        tone="amber"
                        label={t("customerDevices.openTickets")}
                        value={selectedDevice.open_ticket_count}
                      />
                      <ServiceSummary
                        icon={<VideoIcon className="size-5" />}
                        tone="primary"
                        label={t("customerDevices.videoCollaboration")}
                        value={deviceMeetingTotal}
                        loading={meetingsInitialLoading}
                      />
                    </div>
                    <div className="grid gap-8 xl:grid-cols-2">
                      <div>
                        <div className="mb-4 flex items-center justify-between">
                          <h3 className="font-semibold text-foreground">{t("customerDevices.recentConversations")}</h3>
                          <Button variant="ghost" size="sm" onClick={() => router.push(`/customer/chat?deviceId=${selectedDevice.id}`)}>
                            {t("customerDevices.viewAll")}<ChevronRightIcon className="ml-1 size-4" />
                          </Button>
                        </div>
                        {conversationsInitialLoading ? (
                          <DeviceRelationListLoading rows={4} />
                        ) : (
                          <div className="divide-y divide-border rounded-md border border-border">
                            {selectedConversations.length === 0 ? (
                              <div className="p-6 text-sm text-muted-foreground">{t("customerDevices.noConversations")}</div>
                            ) : selectedConversations.slice(0, 4).map((item) => (
                              <ServiceRecordRow
                                key={item.id}
                                icon={<MessagesSquareIcon className="size-4" />}
                                title={item.last_message_summary || t("customerDevices.deviceServiceConversation")}
                                meta={formatDateTime(item.last_active_at, locale)}
                                status={item.status}
                                onClick={() => router.push(`/customer/chat?conversationId=${item.id}`)}
                              />
                            ))}
                          </div>
                        )}
                      </div>
                      <div>
                        <div className="mb-4 flex items-center justify-between">
                          <h3 className="font-semibold text-foreground">{t("customerDevices.recentTickets")}</h3>
                          <Button variant="ghost" size="sm" onClick={() => router.push(buildCustomerTicketPath(0, returnConversationId))}>
                            {t("customerDevices.viewAll")}<ChevronRightIcon className="ml-1 size-4" />
                          </Button>
                        </div>
                        {ticketsInitialLoading ? (
                          <DeviceRelationListLoading rows={4} />
                        ) : (
                          <div className="divide-y divide-border rounded-md border border-border">
                            {selectedTickets.length === 0 ? (
                              <div className="p-6 text-sm text-muted-foreground">{t("customerDevices.noTickets")}</div>
                            ) : selectedTickets.slice(0, 4).map((item) => (
                              <ServiceRecordRow
                                key={item.id}
                                icon={<TicketCheckIcon className="size-4" />}
                                title={item.title}
                                meta={`${item.ticket_no} · ${formatDateTime(item.updated_at, locale)}`}
                                status={item.status}
                                onClick={() => router.push(buildCustomerTicketPath(item.id, returnConversationId))}
                              />
                            ))}
                          </div>
                        )}
                      </div>
                    </div>
                  </div>
                ) : null}

                {deviceTab === "history" ? (
                  <div className="grid gap-8 xl:grid-cols-[minmax(0,1fr)_320px]">
                    <div className="space-y-8">
                      <div>
                        <h3 className="font-semibold text-foreground">{t("customerDevices.serviceHistory")}</h3>
                        <div className="mt-4">
                          {ticketsInitialLoading ? (
                            <DeviceRelationListLoading rows={4} />
                          ) : (
                            <div className="divide-y divide-border rounded-md border border-border">
                              {selectedTickets.length === 0 ? (
                                <div className="p-6 text-sm text-muted-foreground">{t("customerDevices.noCustomerVisibleTickets")}</div>
                              ) : selectedTickets.map((item) => (
                                <ServiceRecordRow
                                  key={item.id}
                                  icon={<WrenchIcon className="size-4" />}
                                  title={item.title}
                                  meta={`${item.ticket_no} · ${formatDateTime(item.updated_at, locale)}`}
                                  status={item.status}
                                  onClick={() => router.push(buildCustomerTicketPath(item.id, returnConversationId))}
                                />
                              ))}
                            </div>
                          )}
                        </div>
                      </div>
                      <div>
                        <h3 className="font-semibold text-foreground">{t("customerDevices.videoCollaboration")}</h3>
                        <div className="mt-4">
                          {meetingsInitialLoading ? (
                            <DeviceRelationListLoading rows={4} />
                          ) : (
                            <div className="divide-y divide-border rounded-md border border-border">
                              {selectedMeetings.length === 0 ? (
                                <div className="p-6 text-sm text-muted-foreground">{t("customerDevices.noVideoRecords")}</div>
                              ) : selectedMeetings.map((item) => (
                                <ServiceRecordRow
                                  key={item.id}
                                  icon={<VideoIcon className="size-4" />}
                                  title={item.ticket_no || item.title}
                                  meta={formatDateTime(item.scheduled_at || item.started_at, locale)}
                                  status={item.status}
                                  onClick={() => router.push(`/customer/meeting?meetingId=${item.id}`)}
                                />
                              ))}
                            </div>
                          )}
                        </div>
                      </div>
                    </div>
                    <aside className="rounded-md border border-border bg-card p-5">
                      <div className="flex items-center gap-2 font-semibold text-foreground">
                        <WrenchIcon className="size-4" />
                        {t("customerDevices.repairArchive")}
                      </div>
                      <div className="mt-5 text-4xl font-semibold text-foreground">{selectedDevice.repair_history_count}</div>
                    </aside>
                  </div>
                ) : null}

                {deviceTab === "manuals" ? (
                  <div><div className="mb-5"><h3 className="font-semibold text-foreground">{t("customerDevices.productDocuments")}</h3></div>{manualsInitialLoading ? <div className="grid gap-4 md:grid-cols-2">{Array.from({ length: 4 }).map((_, index) => <Skeleton key={index} className="h-24 rounded-md" />)}</div> : selectedManuals.length === 0 ? <EmptyCard icon={<BookOpenIcon className="size-6" />} title={t("customerDevices.noManualsTitle")} /> : <div className="grid gap-4 md:grid-cols-2">{selectedManuals.map((manual) => <a key={manual.id} href={manual.url} target="_blank" rel="noreferrer" className="flex items-center gap-4 rounded-md border border-border p-5 transition hover:border-primary/30 hover:bg-primary/5"><span className="grid size-11 shrink-0 place-items-center rounded-md bg-primary/10 text-primary"><FileTextIcon className="size-5" /></span><span className="min-w-0 flex-1"><span className="block truncate font-medium text-foreground">{manual.title || manual.filename}</span><span className="mt-1 block text-sm text-muted-foreground">{formatDateTime(manual.uploaded_at, locale)}</span></span><ExternalLinkIcon className="size-4 shrink-0 text-muted-foreground" /></a>)}</div>}</div>
                ) : null}
              </div>
            </div>
          )}
        </section>
      </div>
      )}
      <CustomerDeviceBindDialog
        open={bindOpen}
        onOpenChange={setBindOpen}
        formId="bind-device-form"
        fieldIdPrefix="bind"
        onBound={handleDeviceBound}
      />
    </div>
  )
}

function CustomerDeviceListLoading() {
  const t = useI18n()
  return (
    <div className="space-y-1.5" role="status" aria-label={t("customerDevices.loadingList")}>
      {Array.from({ length: 5 }).map((_, index) => <Skeleton key={index} className="h-24 rounded-md" />)}
    </div>
  )
}

function CustomerDeviceDetailLoading() {
  const t = useI18n()
  return (
    <div className="space-y-6 p-5 lg:p-6" role="status" aria-label={t("customerDevices.loadingDetail")}>
      <div className="space-y-3">
        <Skeleton className="h-4 w-32 max-w-full" />
        <Skeleton className="h-7 w-56 max-w-full" />
        <Skeleton className="h-4 w-72 max-w-full" />
      </div>
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {Array.from({ length: 4 }).map((_, index) => <Skeleton key={index} className="h-16 rounded-md" />)}
      </div>
      <div className="border-t border-border pt-5">
        <Skeleton className="h-5 w-28" />
        <div className="mt-4 grid gap-4 md:grid-cols-3">
          {Array.from({ length: 3 }).map((_, index) => <Skeleton key={index} className="h-20 rounded-md" />)}
        </div>
      </div>
    </div>
  )
}

function CustomerDeviceRelationsLoading({ variant }: { variant: "overview" | "history" }) {
  const t = useI18n()
  const renderPanel = () => (
    <div className="space-y-4 rounded-md border border-border bg-card p-4">
      <Skeleton className="h-4 w-32" />
      <div className="space-y-2">
        {Array.from({ length: 4 }).map((_, index) => <Skeleton key={index} className="h-12 rounded-md" />)}
      </div>
    </div>
  )

  if (variant === "overview") {
    return (
      <div className="space-y-6" role="status" aria-busy="true" aria-label={t("customerDevices.loadingDetail")}>
        <div className="grid gap-4 md:grid-cols-3">
          {Array.from({ length: 3 }).map((_, index) => <Skeleton key={index} className="h-20 rounded-md" />)}
        </div>
        <div className="grid gap-8 xl:grid-cols-2">
          {renderPanel()}
          {renderPanel()}
        </div>
      </div>
    )
  }

  return (
    <div className="grid gap-8 xl:grid-cols-[minmax(0,1fr)_320px]" role="status" aria-busy="true" aria-label={t("customerDevices.loadingDetail")}>
      <div className="space-y-8">
        <div className="space-y-4">
          <Skeleton className="h-5 w-36" />
          <div className="space-y-2">
            {Array.from({ length: 4 }).map((_, index) => <Skeleton key={index} className="h-12 rounded-md" />)}
          </div>
        </div>
        <div className="space-y-4">
          <Skeleton className="h-5 w-36" />
          <div className="space-y-2">
            {Array.from({ length: 4 }).map((_, index) => <Skeleton key={index} className="h-12 rounded-md" />)}
          </div>
        </div>
      </div>
      <aside className="rounded-md border border-border bg-card p-5">
        <Skeleton className="h-4 w-28" />
        <Skeleton className="mt-5 h-12 w-24" />
      </aside>
    </div>
  )
}

function DeviceRelationListLoading({ rows = 3 }: { rows?: number }) {
  const t = useI18n()
  return (
    <div className="space-y-2 rounded-md border border-border bg-card p-3" role="status" aria-busy="true" aria-label={t("customerMeeting.relationListLoadingAria")}>
      {Array.from({ length: rows }).map((_, index) => (
        <div key={index} className="flex items-center gap-3 rounded-md border border-dashed border-border px-3 py-3">
          <Skeleton className="size-8 shrink-0 rounded-md" />
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className="h-4 w-2/3" />
            <Skeleton className="h-3 w-1/2" />
          </div>
          <Skeleton className="h-6 w-16 rounded-md" />
        </div>
      ))}
    </div>
  )
}

function DeviceCount({ label, value }: { label: string; value: number }) {
  return <div className="rounded-md bg-muted px-2 py-1"><div className="text-sm font-semibold text-foreground">{value}</div><div className="text-rhd-xs text-muted-foreground">{label}</div></div>
}

function ServiceSummary({ icon, tone, label, value, loading }: { icon: ReactNode; tone: "blue" | "amber" | "primary"; label: string; value: number; loading?: boolean }) {
  const toneClass = tone === "blue" ? "bg-primary/10 text-primary" : tone === "amber" ? "bg-amber-50 text-foreground" : "bg-primary/10 text-primary"
  return <div className="flex items-center gap-3 rounded-md border border-border p-4"><span className={cn("grid size-9 place-items-center rounded-md", toneClass)}>{icon}</span><div>{loading ? <Skeleton className="h-8 w-16" /> : <div className="text-xl font-semibold text-foreground">{value}</div>}<div className="mt-0.5 text-sm text-muted-foreground">{label}</div></div></div>
}

function ServiceRecordRow({ icon, title, meta, status, onClick }: { icon: ReactNode; title: string; meta: string; status: string; onClick: () => void }) {
  return <button type="button" onClick={onClick} className="flex w-full items-center gap-3 p-3 text-left transition hover:bg-muted/50"><span className="grid size-8 shrink-0 place-items-center rounded-md bg-muted text-muted-foreground">{icon}</span><span className="min-w-0 flex-1"><span className="block truncate text-rhd-md font-medium text-foreground">{title}</span><span className="mt-0.5 block truncate text-rhd-xs text-muted-foreground">{meta}</span></span><StatusBadge status={status} /><ChevronRightIcon className="size-4 shrink-0 text-muted-foreground" /></button>
}

export function CustomerProfilePortalPage() {
  const { session } = useAuth()
  const t = useI18n()
  const [profile, setProfile] = useState<CustomerPortalProfile | null>(null)
  const [home, setHome] = useState<CustomerPortalHome | null>(null)
  const [loading, setLoading] = useState(true)
  const [profileLoaded, setProfileLoaded] = useState(false)
  const [loadVersion, setLoadVersion] = useState(0)
  const [loadError, setLoadError] = useState("")
  const [saving, setSaving] = useState(false)
  const [bindOpen, setBindOpen] = useState(false)
  const [profileForm, setProfileForm] = useState({
    name: "",
    primaryEmail: "",
    primaryMobile: "",
  })
  const [profileError, setProfileError] = useState("")

  useEffect(() => {
    async function loadData() {
      setLoading(true)
      setLoadError("")
      try {
        const profileData = await fetchCustomerProfile()
        void fetchCustomerHome().then(setHome).catch(() => undefined)
        setProfile(profileData)
        setProfileForm({
          name: profileData.name || "",
          primaryEmail: profileData.primary_email || "",
          primaryMobile: profileData.primary_mobile || "",
        })
        setProfileLoaded(true)
      } catch (error) {
        const message = error instanceof Error ? error.message : t("customerProfile.loadFailed")
        setLoadError(message)
        toast.error(message)
      } finally {
        setLoading(false)
      }
    }
    loadData()
  }, [loadVersion, t])

  const initialProfileLoad = !profileLoaded && loading
  const profileRefreshing = profileLoaded && loading
  const hasDeviceConcept = session?.featureFlags?.device !== false
  const showBindDeviceEntry = hasDeviceConcept && Boolean(profile && profile.bound_device_count === 0)

  async function submitProfile(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const name = profileForm.name.trim()
    if (!name) {
      setProfileError(t("customerProfile.nameRequired"))
      return
    }
    setSaving(true)
    setProfileError("")
    try {
      const updated = await updateCustomerProfile({
        name,
        primary_email: profileForm.primaryEmail.trim(),
        primary_mobile: profileForm.primaryMobile.trim(),
      })
      setProfile(updated)
      setProfileForm({
        name: updated.name || "",
        primaryEmail: updated.primary_email || "",
        primaryMobile: updated.primary_mobile || "",
      })
      toast.success(t("customerProfile.saveSuccess"))
    } catch (error) {
      setProfileError(error instanceof Error ? error.message : t("customerProfile.saveFailed"))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="rhd-railops-customer-page rhd-railops-customer-profile-page space-y-4">
      <PageHeader
        title={t("customerProfile.pageTitle")}
        action={(
          <>
            <Button type="button" variant="outline" onClick={() => setLoadVersion((value) => value + 1)} disabled={loading}>
              <RefreshCwIcon className={cn("mr-2 size-4", loading && "animate-spin")} />
              {t("common.refresh")}
            </Button>
            {hasDeviceConcept ? (
              <Button type="button" onClick={() => setBindOpen(true)}>
                <PlusIcon className="mr-2 size-4" />
                {t("customerDevices.addDevice")}
              </Button>
            ) : null}
          </>
        )}
      />
      <MetricsStrip home={home} hasDeviceConcept={hasDeviceConcept} />
      {loadError && profileLoaded ? (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-muted px-4 py-3 text-sm text-muted-foreground">
          <span>{t("customerProfile.loadFailed")}</span>
          <Button variant="outline" size="sm" onClick={() => setLoadVersion((value) => value + 1)} disabled={loading}>{t("common.retry")}</Button>
        </div>
      ) : null}
      {profile ? (
        <div className="grid gap-4 lg:grid-cols-2" aria-busy={profileRefreshing}>
          <Card className="rounded-md border-border shadow-sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base"><CircleUserRoundIcon className="size-4" /> {t("customerProfile.basicInfo")}</CardTitle>
            </CardHeader>
            <CardContent>
              <form className="space-y-4" onSubmit={submitProfile}>
                <label className="grid gap-1.5 text-sm font-medium text-foreground">
                  {t("customerProfile.name")}
                  <Input
                    value={profileForm.name}
                    onChange={(event) => setProfileForm((current) => ({ ...current, name: event.target.value }))}
                    maxLength={100}
                    autoComplete="name"
                  />
                </label>
                <InfoTile label={t("customerProfile.customerOrg")} value={profile.company_name || t("customerProfile.notBelonged")} />
                <label className="grid gap-1.5 text-sm font-medium text-foreground">
                  {t("customerProfile.email")}
                  <Input
                    type="email"
                    value={profileForm.primaryEmail}
                    onChange={(event) => setProfileForm((current) => ({ ...current, primaryEmail: event.target.value }))}
                    maxLength={100}
                    autoComplete="email"
                    placeholder={t("customerProfile.unfilled")}
                  />
                </label>
                <label className="grid gap-1.5 text-sm font-medium text-foreground">
                  {t("customerProfile.mobile")}
                  <Input
                    value={profileForm.primaryMobile}
                    onChange={(event) => setProfileForm((current) => ({ ...current, primaryMobile: event.target.value }))}
                    maxLength={32}
                    autoComplete="tel"
                    placeholder={t("customerProfile.unfilled")}
                  />
                </label>
                {profileError ? <div className="rounded-lg border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">{profileError}</div> : null}
                <Button type="submit" disabled={saving} className="w-full sm:w-auto">
                  {saving ? <Loader2Icon className="animate-spin" /> : <CheckCircle2Icon />}
                  {saving ? t("customerProfile.saving") : t("customerProfile.save")}
                </Button>
              </form>
            </CardContent>
          </Card>
          <Card className="rounded-md border-border shadow-sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base"><CheckCircle2Icon className="size-4" /> {t("customerProfile.serviceSummary")}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {showBindDeviceEntry ? (
                <EmptyCard
                  icon={<PackageSearchIcon className="size-6" />}
                  title={t("customerDevices.emptyTitle")}
                  action={<Button type="button" onClick={() => setBindOpen(true)}>{t("customerDevices.addDevice")}</Button>}
                />
              ) : null}
              {hasDeviceConcept ? <InfoTile label={t("customerProfile.boundDevices")} value={String(profile.bound_device_count)} /> : null}
              <InfoTile label={t("customerProfile.activeConversations")} value={String(profile.active_conversation_count)} />
              <InfoTile label={t("customerProfile.openTickets")} value={String(profile.open_ticket_count)} />
              <InfoTile label={t("customerProfile.upcomingMeetings")} value={String(profile.upcoming_meeting_count)} />
            </CardContent>
          </Card>
        </div>
      ) : initialProfileLoad ? (
        <div className="grid gap-4 lg:grid-cols-2" aria-busy="true" aria-label={t("common.loading")}>
          <Card className="rounded-md border-border shadow-sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base"><CircleUserRoundIcon className="size-4" /> {t("customerProfile.basicInfo")}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-3">
                <Skeleton className="h-4 w-20" />
                <Skeleton className="h-10 w-full" />
              </div>
              <Skeleton className="h-16 w-full" />
              <div className="space-y-3">
                <Skeleton className="h-4 w-16" />
                <Skeleton className="h-10 w-full" />
              </div>
              <div className="space-y-3">
                <Skeleton className="h-4 w-16" />
                <Skeleton className="h-10 w-full" />
              </div>
              <Skeleton className="h-10 w-32" />
            </CardContent>
          </Card>
          <Card className="rounded-md border-border shadow-sm">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base"><CheckCircle2Icon className="size-4" /> {t("customerProfile.serviceSummary")}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
            </CardContent>
          </Card>
        </div>
      ) : (
        <EmptyCard icon={<CircleUserRoundIcon className="size-6" />} title={t("customerProfile.loadFailed")} action={<Button variant="outline" onClick={() => setLoadVersion((value) => value + 1)} disabled={loading}>{t("common.retry")}</Button>} />
      )}
      {hasDeviceConcept ? (
        <CustomerDeviceBindDialog
          open={bindOpen}
          onOpenChange={setBindOpen}
          formId="profile-bind-device-form"
          fieldIdPrefix="profile-bind-device"
          onBound={() => setLoadVersion((value) => value + 1)}
        />
      ) : null}
    </div>
  )
}

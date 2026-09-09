"use client"

import { translateCurrentMessage } from "@/i18n/messages"
import { useSearchParams } from "next/navigation"
import {
  ArrowDownIcon,
  ArrowRightLeftIcon,
  CalendarClockIcon,
  CircleAlertIcon,
  CheckCircle2Icon,
  ChevronLeftIcon,
  CircleDotIcon,
  CircleXIcon,
  FileTextIcon,
  HandshakeIcon,
  ImageIcon,
  LanguagesIcon,
  Loader2Icon,
  PaperclipIcon,
  RefreshCwIcon,
  SettingsIcon,
  SendIcon,
  ShieldCheckIcon,
  StarIcon,
  BookOpenCheckIcon,
  TicketIcon,
  UserCheckIcon,
  UserPlusIcon,
  VideoIcon,
  WrenchIcon,
  type LucideIcon,
} from "lucide-react"
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from "react"
import { toast } from "sonner"

import { Input, Skeleton } from "antd"
import {
  CheckboxField,
  IconButton,
  PageShell,
  RailopsButton,
  SelectField,
  StandardModal,
  StatusTag,
  UnderlineTabs,
  type RailopsTabItem,
} from "@railops/ui"
import { AgentRealtimeProvider } from "@/components/agent-realtime-provider"
import { useAuth } from "@/components/auth-provider"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { buildEnterpriseMeetingRoomPath } from "@/components/meeting/meeting-room-context"
import { sendAgentConversationTyping } from "@/hooks/use-agent-conversation-realtime"
import { ConversationCloseDialog } from "@/components/conversation-actions/close-dialog"
import { ConversationTransferDialog } from "@/components/conversation-actions/transfer-dialog"
import { ImMessageHTML } from "@/components/im-message-html"
import { VoiceRecorderButton } from "@/components/chat/voice-recorder-button"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  fetchAgentConversationDetail,
  type AgentConversation,
  type AgentMessage,
} from "@/lib/api/agent"
import { fetchAgentProfile, type AdminAgentProfile } from "@/lib/api/admin"
import { fetchCustomer, type AdminCustomer } from "@/lib/api/customer"
import { fetchCustomerContacts, type AdminCustomerContact } from "@/lib/api/customer-contact"
import { translateConversationMessage } from "@/lib/api/enterprise-conversations"
import {
  acceptTicket,
  addTicketSupplierCollaborationProgress,
  cancelTicket,
  createTicketKnowledgeCandidate,
  createTicketProgress,
  createTicketRepair,
  createTicket,
  fetchTicketAutoClosePolicy,
  fetchTicketAggregate,
  fetchTicketKnowledgeCandidates,
  fetchTicketSupplierCollaborations,
  fetchTicketSupplierCollaborationDetail,
  fetchTicketSupplierOptions,
  fetchTickets,
  inviteTicketSupplier,
  resolveTicketSupplierCollaboration,
  takeoverTicket,
  updateTicketAutoClosePolicy,
  type TicketAutoClosePolicy,
  type TicketSupplierCollaboration,
  type TicketSupplierCollaborationDetail,
  type TicketSupplierOption,
} from "@/lib/api/enterprise-tickets"
import { readSession, type AuthSession } from "@/lib/auth"
import { getProductModules, type ProductModule } from "@/lib/api/enterprise-products"
import { fetchEnterpriseWorkbenchQueue } from "@/lib/api/enterprise-workbench"
import { createMeeting, endMeeting, fetchMeetingJoinConfig } from "@/lib/api/meetings"
import type {
  AssetRefDTO,
  ConversationMessageTranslationDTO,
  EnterpriseWorkbenchConversation,
  RepairPartDTO,
  TicketAggregateDTO,
  TicketFlowStepDTO,
  TicketListItem,
  TicketKnowledgeCandidateDTO,
  TicketPriority,
  TicketStatus,
  TicketTimelineItemDTO,
} from "@/lib/api/types"
import { customerDisplayName } from "@/lib/customer-identity"
import { renderIMMessageHTML } from "@/lib/im-message"
import { TYPING_IDLE_TIMEOUT_MS } from "@/lib/im-realtime-state"
import {
  ConversationServiceEvent,
  parseConversationServiceEvent,
  type ConversationServiceEventData,
} from "@/components/remote-helpdesk/conversation-service-event"
import { PersonCard, formatRelativeOnlineText, type PersonCardData } from "@/components/remote-helpdesk/person-card"
import {
  agentConversationSelectors,
  useAgentConversationsStore,
} from "@/lib/stores/agent-conversations"
import { isProcessingTicketStatus, isTerminalTicketStatus } from "@/lib/ticket-lifecycle"
import { sortWorkbenchQueueItems } from "@/lib/ticket-workbench-queue"
import {
  findWorkbenchRouteItem,
  indexWorkbenchTicketsByConversation,
  normalizeWorkbenchId,
} from "@/lib/ticket-workbench-route"
import { cn, formatDateTime } from "@/lib/utils"
function ee(key: string, values?: Record<string, unknown>) {
  if (!values) {
    return translateCurrentMessage(`enterpriseExtract.${key}`)
  }
  return translateCurrentMessage(
    `enterpriseExtract.${key}`,
    Object.fromEntries(
      Object.entries(values).map(([name, value]) => [
        name,
        typeof value === "string" || typeof value === "number" ? value : String(value ?? ""),
      ]),
    ),
  )
}

const CUSTOMER_ENTRY_EVENT_I18N_PREFIX = "customerEntryExtract.event."

function cee(key: string, values?: Record<string, string | number>) {
  return translateCurrentMessage(`${CUSTOMER_ENTRY_EVENT_I18N_PREFIX}${key}`, values)
}


const QUEUE_PAGE_SIZE = 16

type MobilePane = "queue" | "chat" | "context"
type QueueTab = "all" | "active" | "done"
type ContextTab = "summary" | "supplier" | "knowledge" | "progress" | "timeline"
type QueueItemKind = "conversation" | "ticket"
type MeetingCreateMode = "now" | "scheduled"
type MeetingCreateOptions = {
  scheduledAt?: string
  openRoom?: boolean
}
type Tone = "good" | "warn" | "info" | "neutral" | "bad"

function getTicketKnowledgeCandidateStatusLabel(status: string, linked: boolean) {
  if (linked || status === "approved" || status === "merged") return ee("ticketWorkbench.text001")
  if (status === "rejected") return ee("ticketWorkbench.text002")
  if (status === "needs_enrichment") return ee("ticketWorkbench.text003")
  if (status === "low_quality") return ee("ticketWorkbench.text004")
  if (status === "low_value") return ee("ticketWorkbench.text005")
  if (status === "duplicate") return ee("ticketWorkbench.text006")
  return ee("ticketWorkbench.text007")
}

type WorkbenchMessage = {
  id?: number
  role: string
  content: string
  html: string
  time: string
  side: "left" | "right"
  senderType: string
  senderId?: number
  senderName?: string
  senderAvatar?: string
  tone?: "ai" | "customer" | "engineer" | "partner" | "system"
  serviceEvent?: ConversationServiceEventData | null
}

type CustomerCardProfile = {
  name?: string
  company?: string
  phone?: string
  email?: string
  lastActiveAt?: string
}

const translationLanguageLabels: Record<string, string> = {
  "zh-CN": ee("ticketWorkbench.text008"),
  en: "English",
  de: "Deutsch",
  es: "Espanol",
  fr: "Francais",
  pt: "Portugues",
  ja: "Japanese",
  ko: "Korean",
}

function translationLanguageLabel(language: string) {
  return translationLanguageLabels[language] || language
}

function detectMessageTranslationLanguage(content: string) {
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

function resolveMessageTranslationTarget(content: string, selectedTarget: string) {
  const sourceLanguage = detectMessageTranslationLanguage(content)
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

type WorkbenchAttachment = {
  label: string
  desc: string
  status: string
  tone?: Tone
}

type WorkbenchPart = {
  name: string
  desc: string
  quantity?: string
}

type WorkbenchTimeline = {
  key?: string
  time: string
  title: string
  actor?: string
  status?: string
  state?: "done" | "current" | "risk" | "waiting"
}

type WorkbenchDetail = {
  serviceSummary: string
  conclusion: string
  tags: string[]
  attachments: WorkbenchAttachment[]
  parts: WorkbenchPart[]
  rating: string
  feedback: string
  flow: WorkbenchTimeline[]
  timeline: WorkbenchTimeline[]
}

type WorkbenchQueueItem = {
  key: string
  kind: QueueItemKind
  conversationId?: number
  ticketId?: number
  customer: string
  title: string
  summary: string
  ticketNo: string
  priorityLabel: string
  statusLabel: string
  statusTone: Tone
  updatedLabel: string
  updatedAt: string
  owner: string
  device: string
  source: string
  sla: string
  dispatchLabel?: string
  isDone: boolean
  unread?: number
  conversation?: AgentConversation
  ticket?: TicketListItem
}

const conversationStatusMap: Record<
  number,
  { label: string; className: string; icon: LucideIcon; tone: Tone; isDone: boolean }
> = {
  1: {
    label: ee("ticketWorkbench.text009"),
    className: "border-primary/20 bg-primary/10 text-primary",
    icon: ShieldCheckIcon,
    tone: "info",
    isDone: false,
  },
  2: {
    label: ee("ticketWorkbench.text010"),
    className: "border-amber-200 bg-amber-50 text-foreground",
    icon: CircleDotIcon,
    tone: "warn",
    isDone: false,
  },
  3: {
    label: ee("ticketWorkbench.text011"),
    className: "border-primary/20 bg-primary/10 text-primary",
    icon: UserCheckIcon,
    tone: "good",
    isDone: false,
  },
  4: {
    label: ee("ticketWorkbench.text012"),
    className: "border-border bg-muted text-muted-foreground",
    icon: CheckCircle2Icon,
    tone: "neutral",
    isDone: true,
  },
}

function statusMeta(status: number) {
  return (
    conversationStatusMap[status] ?? {
      label: ee("ticketWorkbench.text013", { value0: status }),
      className: "border-border bg-muted text-muted-foreground",
      icon: CircleDotIcon,
      tone: "neutral" as Tone,
      isDone: false,
    }
  )
}

function ticketStatusLabel(status: string) {
  const map: Record<string, string> = {
    draft: ee("ticketWorkbench.text014"),
    pending: ee("ticketWorkbench.text015"),
    pending_acceptance: ee("ticketWorkbench.text015"),
    pending_dispatch: ee("ticketWorkbench.text016"),
		accepted: ee("ticketWorkbench.text017"),
	assigned: ee("ticketWorkbench.text018"),
	pending_assignee_accept: ee("ticketWorkbench.text018"),
    in_progress: ee("ticketWorkbench.text011"),
	processing: ee("ticketWorkbench.text011"),
	video_support: ee("ticketWorkbench.text019"),
	supplier_support: ee("ticketWorkbench.text020"),
	resolved: ee("ticketWorkbench.text021"),
	pending_customer_confirm: ee("ticketWorkbench.text021"),
	closed: ee("ticketWorkbench.text022"),
	quality_review: ee("ticketWorkbench.text023"),
	reopened: ee("ticketWorkbench.text024"),
    done: ee("ticketWorkbench.text012"),
    cancelled: ee("ticketWorkbench.text025"),
  }
  return map[status] ?? status
}

function ticketStatusRequiresManualAccept(status?: string | null) {
  return new Set<string>([
    "pending",
    "pending_acceptance",
    "pending_dispatch",
    "pending_assignee_accept",
    "assigned",
    "reopened",
  ]).has(String(status || "").trim())
}

function canShowManualAcceptFallback(status?: string | null, assigneeId?: number | null) {
  return ticketStatusRequiresManualAccept(status) && !assigneeId
}

function canManageTicketDispatchFromSession(session: AuthSession | null) {
  if (!session) {
    return false
  }
  const roles = new Set([...(session.roles ?? []), ...(session.user?.roles ?? [])])
  if (session.domainType === "platform") {
    return true
  }
  return (
    roles.has("tenant_owner") ||
    roles.has("tenant_admin") ||
    roles.has("service_manager") ||
    roles.has("super_admin") ||
    roles.has("admin") ||
    (session.permissions ?? []).includes("ticket.assign")
  )
}

function ticketPriorityLabel(priority: string) {
  const map: Record<string, string> = {
    critical: "P0",
    high: "P1",
    medium: "P2",
    low: "P3",
  }
  return map[priority] ?? priority
}

function priorityFullLabel(priority: string) {
  const map: Record<string, string> = {
    critical: ee("ticketWorkbench.text026"),
    high: ee("ticketWorkbench.text027"),
    medium: ee("ticketWorkbench.text028"),
    low: ee("ticketWorkbench.text029"),
  }
  return map[priority] ?? priority
}

function ticketSourceLabel(source: string) {
  const map: Record<string, string> = {
    manual: ee("ticketWorkbench.text030"),
    conversation: ee("ticketWorkbench.text031"),
  }
  return map[source] ?? (source || "-")
}

function ticketTone(status: TicketStatus, priority?: TicketPriority, breached?: boolean): Tone {
  if (breached || priority === "critical") {
    return "bad"
  }
  if (status === "closed" || status === "done") {
    return "good"
  }
  if (isProcessingTicketStatus(status)) {
    return "info"
  }
  if (status === "cancelled" || status === "quality_review") {
    return "neutral"
  }
  return "warn"
}

function statusToneClassName(tone: Tone) {
  const map: Record<Tone, string> = {
    good: "border-primary/20 bg-primary/10 text-primary",
    warn: "border-amber-200 bg-amber-50 text-foreground",
    info: "border-primary/20 bg-primary/10 text-primary",
    neutral: "border-border bg-muted text-muted-foreground",
    bad: "border-destructive/20 bg-destructive/10 text-destructive",
  }
  return map[tone]
}

function priorityClassName(priority: string) {
  if (priority === "critical" || priority === "P0") {
    return "border-destructive/20 bg-destructive/10 text-destructive"
  }
  if (priority === "high" || priority === "P1") {
    return "border-orange-200 bg-orange-50 text-orange-700"
  }
  if (priority === "medium" || priority === "P2") {
    return "border-primary/20 bg-primary/10 text-primary"
  }
  return "border-border bg-muted text-muted-foreground"
}

function presencePillClassName(active: boolean, tone: "blue" | "primary" | "slate" = "primary") {
  if (active && tone === "blue") return "border-primary/20 bg-primary/10 text-primary"
  if (active) return "border-primary/20 bg-primary/10 text-primary"
  return "border-border bg-muted text-muted-foreground"
}

function engineerRealtimeLabel(status: "connecting" | "connected" | "disconnected") {
  if (status === "connected") return ee("ticketWorkbench.text032")
  if (status === "connecting") return ee("ticketWorkbench.text033")
  return ee("ticketWorkbench.text034")
}

function conversationDisplayName(conversation: AgentConversation | null) {
  if (!conversation) {
    return ee("ticketWorkbench.text035")
  }
  return customerDisplayName(
    conversation.customerName,
    ee("ticketWorkbench.text036", { value0: conversation.customerId || conversation.id }),
  )
}

function formatShortDateTime(value?: string) {
  if (!value) {
    return ee("ticketWorkbench.text037")
  }
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) {
    return value
  }
  return formatDateTime(value)
}

function toMeetingDatetimeLocalValue(date: Date) {
  const localDate = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return localDate.toISOString().slice(0, 16)
}

function defaultMeetingScheduleLocalValue() {
  return toMeetingDatetimeLocalValue(new Date(Date.now() + 30 * 60_000))
}

function defaultSupplierAuthorizationLocalValue() {
  return toMeetingDatetimeLocalValue(new Date(Date.now() + 30 * 24 * 60 * 60_000))
}

function localDatetimeValueToISOString(value: string) {
  if (!value) return ""
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return ""
  return date.toISOString()
}

function formatMeetingDuration(seconds?: number) {
  const totalSeconds = Math.max(0, Number(seconds) || 0)
  if (totalSeconds < 60) {
    return totalSeconds > 0 ? `${totalSeconds}s` : ee("ticketWorkbench.text038")
  }
  const minutes = Math.floor(totalSeconds / 60)
  if (minutes < 60) {
    return `${minutes}m`
  }
  const hours = Math.floor(minutes / 60)
  const restMinutes = minutes % 60
  return restMinutes > 0 ? `${hours}h ${restMinutes}m` : `${hours}h`
}

function canJoinTicketMeeting(status?: string) {
  return status === "waiting" || status === "scheduled" || status === "active"
}

function ticketMeetingStatusText(status?: string) {
  if (status === "active") return ee("ticketWorkbench.text039")
  if (status === "scheduled") return ee("ticketWorkbench.text040")
  if (status === "waiting") return ee("ticketWorkbench.text041")
  if (status === "finished") return ee("ticketWorkbench.text042")
  return ee("ticketWorkbench.text043")
}

function ticketMeetingBadgeText(status?: string) {
  if (status === "active") return ee("ticketWorkbench.text044")
  if (status === "scheduled") return ee("ticketWorkbench.text045")
  if (status === "waiting") return ee("ticketWorkbench.text046")
  if (status === "finished") return ee("ticketWorkbench.text047")
  return ee("ticketWorkbench.text048")
}

function ticketAgeLabel(value?: string) {
  if (!value) {
    return ee("ticketWorkbench.text049")
  }
  const createdAt = new Date(value).getTime()
  if (!Number.isFinite(createdAt)) {
    return formatShortDateTime(value)
  }
  const minutes = Math.max(0, Math.floor((Date.now() - createdAt) / 60_000))
  if (minutes < 1) {
    return ee("ticketWorkbench.text050")
  }
  if (minutes < 60) {
    return ee("ticketWorkbench.text051", { value0: minutes })
  }
  const hours = Math.floor(minutes / 60)
  if (hours < 48) {
    return ee("ticketWorkbench.text052", { value0: hours })
  }
  return ee("ticketWorkbench.text053", { value0: Math.floor(hours / 24) })
}

function slaLabel(deadline: string, status: string) {
  if (isTerminalTicketStatus(status)) {
    return ticketStatusLabel(status)
  }
  if (!deadline) {
    return ee("ticketWorkbench.text054")
  }
  const target = new Date(deadline).getTime()
  if (!Number.isFinite(target)) {
    return ee("ticketWorkbench.text054")
  }
  const diff = target - Date.now()
  if (diff <= 0) {
    return ee("ticketWorkbench.text055")
  }
  const mins = Math.floor(diff / 60000)
  if (mins < 60) {
    return ee("ticketWorkbench.text056", { value0: mins })
  }
  const hours = Math.floor(mins / 60)
  if (hours < 48) {
    return ee("ticketWorkbench.text057", { value0: hours })
  }
  return formatDateTime(deadline)
}

function fieldValue(value?: string | number | null) {
  if (value === undefined || value === null || value === "") {
    return "-"
  }
  return String(value)
}

function cleanText(value?: string | null) {
  return String(value ?? "").trim()
}

function pickCustomerContactValue(contacts: AdminCustomerContact[], type: "mobile" | "email") {
  const candidates = contacts.filter(
    (contact) =>
      String(contact.contactType).toLowerCase() === type &&
      cleanText(contact.contactValue),
  )
  const selected = candidates.find((contact) => contact.isPrimary) ?? candidates[0]
  return cleanText(selected?.contactValue)
}

function extractEmailFromText(value?: string) {
  return cleanText(value).match(/[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/i)?.[0] ?? ""
}

function buildCustomerCardProfile(
  customer: AdminCustomer | null,
  contacts: AdminCustomerContact[],
): CustomerCardProfile | null {
  if (!customer && contacts.length === 0) {
    return null
  }
  return {
    name: cleanText(customer?.name),
    company: cleanText(customer?.company?.name),
    phone: cleanText(customer?.primaryMobile) || pickCustomerContactValue(contacts, "mobile"),
    email: cleanText(customer?.primaryEmail) || pickCustomerContactValue(contacts, "email"),
    lastActiveAt: cleanText(customer?.lastActiveAt),
  }
}

function dispatchFailureReasonLabel(reason?: string | null) {
  const normalized = String(reason || "").trim()
  const map: Record<string, string> = {
    accept_timeout_redispatching: ee("ticketWorkbench.text058"),
    assignee_unavailable_redispatching: ee("ticketWorkbench.text059"),
    all_candidates_at_capacity: ee("ticketWorkbench.text060"),
    candidate_became_unavailable: ee("ticketWorkbench.text061"),
    missing_supervisor: ee("ticketWorkbench.text062"),
    no_active_schedule_team: ee("ticketWorkbench.text063"),
    no_alternative_after_accept_timeout: ee("ticketWorkbench.text064"),
    no_alternative_after_assignee_unavailable: ee("ticketWorkbench.text065"),
    no_matched_profile: ee("ticketWorkbench.text066"),
    no_product_repair_team: ee("ticketWorkbench.text067"),
    no_profile_for_enabled_user: ee("ticketWorkbench.text068"),
    no_reachable_user: ee("ticketWorkbench.text069"),
  }
  return map[normalized] ?? normalized
}

function ticketDispatchAgingLabel(
  ticket?: Pick<
    TicketListItem,
    "dispatch_attempts" | "dispatch_deferred_until" | "last_dispatch_failure_reason" | "status"
  > | null,
) {
  if (!ticket || isTerminalTicketStatus(ticket.status)) return ""
  const reason = dispatchFailureReasonLabel(ticket.last_dispatch_failure_reason)
  if (reason) return reason
  if (ticket.dispatch_deferred_until) return ee("ticketWorkbench.text070")
  if (ticket.dispatch_attempts > 0) return ee("ticketWorkbench.text071", { value0: ticket.dispatch_attempts })
  return ""
}

function dispatchEvidenceParts(ticket?: TicketListItem | null, hasDeviceConcept = true) {
  if (!ticket) return []
  const parts = [
    ticket.team_name ? ee(hasDeviceConcept ? "ticketWorkbench.text072" : "ticketWorkbench.text325", { value0: ticket.team_name }) : "",
    ticket.assignee_name ? ee("ticketWorkbench.text073", { value0: ticket.assignee_name }) : "",
    ticket.dispatch_attempts > 0 ? ee("ticketWorkbench.text074", { value0: ticket.dispatch_attempts }) : "",
    ticket.last_dispatch_failure_reason ? ee("ticketWorkbench.text075", { value0: dispatchFailureReasonLabel(ticket.last_dispatch_failure_reason) }) : "",
    ticket.dispatch_deferred_until ? ee("ticketWorkbench.text076", { value0: formatDateTime(ticket.dispatch_deferred_until) }) : "",
  ]
  return parts.filter(Boolean)
}

function buildTicketDraft(item: WorkbenchQueueItem | null) {
  if (!item) {
    return {
      title: "",
      description: "",
      priority: "high" as TicketPriority,
    }
  }
  return {
    title: ee("ticketWorkbench.text077", { value0: item.customer }),
    description: item.summary || ee("ticketWorkbench.text078", { value0: item.title, value1: item.source }),
    priority:
      item.priorityLabel === "P0"
        ? ("critical" as TicketPriority)
        : item.priorityLabel === "P1"
          ? ("high" as TicketPriority)
          : ("medium" as TicketPriority),
  }
}

function conversationPriorityLabel(priority: number) {
  if (priority >= 3) return "P0"
  if (priority >= 2) return "P1"
  if (priority >= 1) return "P2"
  return "P3"
}

function toAgentConversation(conversation: EnterpriseWorkbenchConversation): AgentConversation {
  return {
    id: conversation.id,
    channelId: conversation.channel_id,
    customerId: conversation.customer_id,
    customerName: conversation.customer_name,
    status: conversation.status,
    serviceMode: conversation.service_mode,
    priority: conversation.priority,
    currentAssigneeId: conversation.current_assignee_id,
    currentAssigneeName: conversation.current_assignee_name,
    currentTeamId: conversation.current_team_id,
    currentTeamName: conversation.current_team_name,
    lastMessageId: 0,
    lastMessageAt: conversation.last_message_at,
    lastActiveAt: conversation.last_active_at,
    lastMessageSummary: conversation.last_message_summary,
    customerUnreadCount: conversation.customer_unread_count,
    agentUnreadCount: conversation.agent_unread_count,
    customerLastReadMessageId: 0,
    agentLastReadMessageId: 0,
    customerOnline: Boolean(conversation.customer_online),
    customerLastSeenAt: conversation.customer_last_seen_at,
  }
}

function buildConversationQueueItem(
  source: EnterpriseWorkbenchConversation,
  linkedTicket?: TicketListItem,
): WorkbenchQueueItem {
  const conversation = toAgentConversation(source)
  const meta = statusMeta(conversation.status)
  const customer = conversationDisplayName(conversation)
  const linkedTicketTone = linkedTicket
    ? ticketTone(linkedTicket.status, linkedTicket.priority, linkedTicket.sla_breached)
    : meta.tone
  return {
    key: `conversation-${conversation.id}`,
    kind: "conversation",
    conversationId: conversation.id,
    ticketId: linkedTicket?.id,
    customer,
    title: linkedTicket?.title || conversation.lastMessageSummary || ee("ticketWorkbench.text079", { value0: customer }),
    summary: conversation.lastMessageSummary || ee("ticketWorkbench.text080"),
    ticketNo: linkedTicket?.ticket_no || ee("ticketWorkbench.text081", { value0: conversation.id }),
    priorityLabel: linkedTicket
      ? ticketPriorityLabel(linkedTicket.priority)
      : conversationPriorityLabel(conversation.priority),
    statusLabel: linkedTicket ? ticketStatusLabel(linkedTicket.status) : meta.label,
    statusTone: linkedTicketTone,
    updatedLabel: linkedTicket
      ? ticketAgeLabel(linkedTicket.created_at)
      : formatShortDateTime(conversation.lastMessageAt || conversation.lastActiveAt),
    updatedAt:
      linkedTicket?.updated_at || linkedTicket?.created_at || conversation.lastMessageAt || conversation.lastActiveAt || "",
    owner: linkedTicket?.assignee_name || conversation.currentAssigneeName || ee("ticketWorkbench.text082"),
    device: source.device_no || source.product_name || (conversation.channelId ? `Channel #${conversation.channelId}` : ee("ticketWorkbench.text083")),
    source: conversation.serviceMode === 1 ? ee("ticketWorkbench.text009") : ee("ticketWorkbench.text084"),
    sla: linkedTicket
      ? linkedTicket.sla_breached
        ? ee("ticketWorkbench.text055")
        : slaLabel(linkedTicket.sla_deadline, linkedTicket.status)
      : conversation.agentUnreadCount > 0
        ? ee("ticketWorkbench.text085", { value0: conversation.agentUnreadCount })
        : ee("ticketWorkbench.text086"),
    dispatchLabel: ticketDispatchAgingLabel(linkedTicket),
    isDone: linkedTicket
      ? isTerminalTicketStatus(linkedTicket.status)
      : meta.isDone,
    unread: conversation.agentUnreadCount,
    conversation,
    ticket: linkedTicket,
  }
}

function buildPinnedWorkbenchConversation(source: AgentConversation): EnterpriseWorkbenchConversation {
  return {
    id: source.id,
    customer_id: source.customerId ?? 0,
    customer_name: source.customerName,
    status: source.status,
    service_mode: source.serviceMode,
    priority: source.priority,
    current_assignee_id: source.currentAssigneeId,
    current_assignee_name: source.currentAssigneeName ?? "",
    current_team_id: source.currentTeamId,
    current_team_name: source.currentTeamName ?? "",
    channel_id: source.channelId ?? 0,
    product_name: "",
    device_no: "",
    last_message_at: source.lastMessageAt ?? "",
    last_active_at: source.lastActiveAt ?? "",
    last_message_summary: source.lastMessageSummary ?? "",
    agent_unread_count: source.agentUnreadCount,
    customer_unread_count: source.customerUnreadCount,
    customer_online: source.customerOnline,
    customer_last_seen_at: source.customerLastSeenAt ?? "",
  }
}

function buildTicketQueueItem(ticket: TicketListItem): WorkbenchQueueItem {
  const tone = ticketTone(ticket.status, ticket.priority, ticket.sla_breached)
  return {
    key: `ticket-${ticket.id}`,
    kind: "ticket",
    ticketId: ticket.id,
    conversationId: ticket.conversation_id,
    customer: customerDisplayName(ticket.customer_name),
    title: ticket.title,
    summary: `${ticket.product_name || ee("ticketWorkbench.text087")} / ${ticket.device_no || ee("ticketWorkbench.text088")} · ${ticketSourceLabel(ticket.source || "")}`,
    ticketNo: ticket.ticket_no,
    priorityLabel: ticketPriorityLabel(ticket.priority),
    statusLabel: ticketStatusLabel(ticket.status),
    statusTone: tone,
    updatedLabel: ticketAgeLabel(ticket.created_at),
    updatedAt: ticket.updated_at || ticket.created_at,
    owner: ticket.assignee_name || ee("ticketWorkbench.text082"),
    device: ticket.device_no || ticket.product_name || ee("ticketWorkbench.text083"),
    source: ticketSourceLabel(ticket.source || ""),
    sla: ticket.sla_breached ? ee("ticketWorkbench.text055") : slaLabel(ticket.sla_deadline, ticket.status),
    dispatchLabel: ticketDispatchAgingLabel(ticket),
    isDone: isTerminalTicketStatus(ticket.status),
    ticket,
  }
}

function buildTicketListItemFromAggregate(aggregate: TicketAggregateDTO): TicketListItem {
  const { ticket, assignment, customer, device_context: deviceContext } = aggregate
  const deadline = Date.parse(ticket.sla_deadline || "")
  return {
    id: ticket.id,
    ticket_no: ticket.ticket_no,
    title: ticket.title,
    priority: ticket.priority,
    status: ticket.status,
    source: ticket.source,
    channel: ticket.channel,
    conversation_id: ticket.conversation_id,
    product_id: ticket.product_id,
    current_team_id: assignment.team_id,
    team_name: assignment.team_name,
    assignee_id: assignment.assignee_id,
    customer_name: customerDisplayName(customer.name),
    device_no: deviceContext.device_no,
    product_name: deviceContext.product_name,
    assignee_name: assignment.assignee_name || null,
    created_at: ticket.created_at,
    updated_at: ticket.updated_at,
    sla_deadline: ticket.sla_deadline,
    sla_breached:
      !isTerminalTicketStatus(ticket.status) && Number.isFinite(deadline) && deadline < Date.now(),
    dispatch_attempts: assignment.dispatch_attempts,
    dispatch_deferred_until: assignment.dispatch_deferred_until,
    last_dispatch_failure_reason: assignment.last_dispatch_failure_reason,
    actions: aggregate.actions,
  }
}

function queueItemMatchesTab(item: WorkbenchQueueItem, tab: QueueTab) {
  if (tab === "all") {
    return true
  }
  if (tab === "done") {
    return item.isDone
  }
  return !item.isDone
}

function syncWorkbenchRoute(item: WorkbenchQueueItem | null) {
  const url = new URL(window.location.href)
  url.searchParams.delete("ticket_id")
  url.searchParams.delete("ticketId")
  url.searchParams.delete("conversation_id")
  url.searchParams.delete("conversationId")
  if (item?.ticketId) {
    url.searchParams.set("ticket_id", String(item.ticketId))
  } else if (item?.conversationId) {
    url.searchParams.set("conversationId", String(item.conversationId))
  }
  window.history.replaceState(window.history.state, "", url)
}

export default function EnterpriseTicketWorkbenchPage() {
  return (
    <>
      <AgentRealtimeProvider />
      <EnterpriseTicketWorkbench />
    </>
  )
}

function EnterpriseTicketWorkbench() {
  const searchParams = useSearchParams()
  const routeSearch = searchParams.toString()
  const conversation = useAgentConversationsStore(
    agentConversationSelectors.selectedConversation,
  )
  const messages = useAgentConversationsStore((state) => state.messages)
  const messagesLoading = useAgentConversationsStore((state) => state.messagesLoading)
  const messagesLoadingMore = useAgentConversationsStore((state) => state.messagesLoadingMore)
  const messagesHasMore = useAgentConversationsStore((state) => state.messagesHasMore)
  const selectedConversationId = useAgentConversationsStore((state) => state.selectedConversationId)
  const messagesLoadedConversationId = useAgentConversationsStore(
    (state) => state.messagesLoadedConversationId,
  )
  const sending = useAgentConversationsStore((state) => state.sending)
  const uploadingAsset = useAgentConversationsStore((state) => state.uploadingAsset)
  const realtimePresence = useAgentConversationsStore((state) => state.realtimePresence)
  const realtimeTyping = useAgentConversationsStore((state) => state.realtimeTyping)
  const realtimeStatus = useAgentConversationsStore((state) => state.realtimeStatus)
  const selectConversation = useAgentConversationsStore(
    (state) => state.selectConversation,
  )
  const syncLatestMessages = useAgentConversationsStore((state) => state.syncLatestMessages)
  const loadOlderMessages = useAgentConversationsStore((state) => state.loadOlderMessages)
  const sendMessage = useAgentConversationsStore((state) => state.sendMessage)
  const sendImage = useAgentConversationsStore((state) => state.sendImage)
  const sendAttachment = useAgentConversationsStore((state) => state.sendAttachment)
  const sendVoice = useAgentConversationsStore((state) => state.sendVoice)
  const markSelectedConversationRead = useAgentConversationsStore(
    (state) => state.markSelectedConversationRead,
  )

  const [mobilePane, setMobilePane] = useState<MobilePane>("chat")

  const mobilePaneTabs: RailopsTabItem[] = [
    { value: "queue", label: ee("ticketWorkbench.text089") },
    { value: "chat", label: ee("ticketWorkbench.text090") },
    { value: "context", label: ee("ticketWorkbench.text091") },
  ]
  const [routeConversationId, setRouteConversationId] = useState(0)
  const [routeTicketId, setRouteTicketId] = useState(0)
  const [queueTab, setQueueTab] = useState<QueueTab>("active")
  const [conversationQueue, setConversationQueue] = useState<EnterpriseWorkbenchConversation[]>([])
  const [ticketQueue, setTicketQueue] = useState<TicketListItem[]>([])
  const [pinnedConversation, setPinnedConversation] = useState<EnterpriseWorkbenchConversation | null>(null)
  const [pinnedTicket, setPinnedTicket] = useState<TicketListItem | null>(null)
  const [queueLoading, setQueueLoading] = useState(false)
  const [queueLoaded, setQueueLoaded] = useState(false)
  const [queueError, setQueueError] = useState("")
  const [selectedQueueKey, setSelectedQueueKey] = useState("")
  const [conversationTransferMode, setConversationTransferMode] = useState<"assign" | "transfer">("assign")
  const [conversationTransferOpen, setConversationTransferOpen] = useState(false)
  const [conversationCloseOpen, setConversationCloseOpen] = useState(false)
  const [composerText, setComposerText] = useState("")
  const [acceptingTicket, setAcceptingTicket] = useState(false)
  const [translationTargetLanguage, setTranslationTargetLanguage] = useState("zh-CN")
  const [translatedMessages, setTranslatedMessages] = useState<
    Record<number, ConversationMessageTranslationDTO>
  >({})
  const [translatingMessageId, setTranslatingMessageId] = useState<number | null>(null)
	const [autoCloseOpen, setAutoCloseOpen] = useState(false)
	const [autoClosePolicy, setAutoClosePolicy] = useState<TicketAutoClosePolicy>({
		enabled: false,
		days: 7,
	})
	const [loadingAutoClosePolicy, setLoadingAutoClosePolicy] = useState(false)
	const [savingAutoClosePolicy, setSavingAutoClosePolicy] = useState(false)

	useEffect(() => {
		if (!autoCloseOpen) return
		let cancelled = false
		setLoadingAutoClosePolicy(true)
		void fetchTicketAutoClosePolicy()
			.then((res) => {
				if (cancelled) return
				if (res.success && res.data) setAutoClosePolicy(res.data)
				else toast.error(res.error?.message || ee("ticketWorkbench.text092"))
			})
			.catch((error) => {
				if (!cancelled) toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text092"))
			})
			.finally(() => {
				if (!cancelled) setLoadingAutoClosePolicy(false)
			})
		return () => {
			cancelled = true
		}
	}, [autoCloseOpen])

	const handleSaveAutoClosePolicy = async () => {
		setSavingAutoClosePolicy(true)
		try {
			const res = await updateTicketAutoClosePolicy(autoClosePolicy)
			if (!res.success || !res.data) {
				toast.error(res.error?.message || ee("ticketWorkbench.text093"))
				return
			}
			setAutoClosePolicy(res.data)
			setAutoCloseOpen(false)
			toast.success(res.data.enabled ? ee("ticketWorkbench.text094", { value0: res.data.days }) : ee("ticketWorkbench.text095"))
		} finally {
			setSavingAutoClosePolicy(false)
		}
	}

  const loadWorkbenchQueue = useCallback(async (silent = false) => {
    if (!silent) {
      setQueueLoading(true)
    }
    try {
      const res = await fetchEnterpriseWorkbenchQueue({ page: 1, pageSize: QUEUE_PAGE_SIZE })
      if (res.success && res.data) {
        setConversationQueue(res.data.conversations ?? [])
        setTicketQueue(res.data.tickets ?? [])
        setQueueError("")
        setQueueLoaded(true)
      } else {
        const message = res.error?.message || ee("ticketWorkbench.text096")
        setQueueError(message)
        if (!silent) {
          toast.error(message)
        }
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : ee("ticketWorkbench.text096")
      setQueueError(message)
      if (!silent) {
        toast.error(message)
      }
    } finally {
      if (!silent) {
        setQueueLoading(false)
      }
    }
  }, [])

  const refreshTicketSnapshot = useCallback(async (ticketId: number) => {
    if (ticketId <= 0) {
      return
    }
    try {
      const res = await fetchTicketAggregate(ticketId)
      if (!res.success || !res.data) {
        return
      }
      const snapshot = buildTicketListItemFromAggregate(res.data)
      setTicketQueue((current) => current.map((ticket) => ticket.id === ticketId ? snapshot : ticket))
      setPinnedTicket((current) => current?.id === ticketId ? snapshot : current)
    } catch {
      // The queue refresh remains the fallback when a focused snapshot request is interrupted.
    }
  }, [])

  useEffect(() => {
    const params = new URLSearchParams(routeSearch)
    const nextTicketId = normalizeWorkbenchId(params.get("ticket_id") || params.get("ticketId"))
    const nextConversationId = nextTicketId > 0
      ? 0
      : normalizeWorkbenchId(params.get("conversation_id") || params.get("conversationId"))
    setRouteTicketId(nextTicketId)
    setRouteConversationId(nextConversationId)
    setPinnedTicket((current) => current?.id === nextTicketId ? current : null)
    setPinnedConversation((current) => current?.id === nextConversationId ? current : null)
  }, [routeSearch])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void loadWorkbenchQueue()
    }, 120)
    return () => window.clearTimeout(timer)
  }, [loadWorkbenchQueue])

  useEffect(() => {
    const refresh = () => {
      void loadWorkbenchQueue(true)
      if (pinnedTicket?.id) {
        void refreshTicketSnapshot(pinnedTicket.id)
      }
    }
    const timer = window.setInterval(refresh, 15_000)
    window.addEventListener("focus", refresh)
    return () => {
      window.clearInterval(timer)
      window.removeEventListener("focus", refresh)
    }
  }, [loadWorkbenchQueue, pinnedTicket?.id, refreshTicketSnapshot])

  const queueItems = useMemo(() => {
    const effectiveConversationQueue = pinnedConversation && !conversationQueue.some((item) => item.id === pinnedConversation.id)
      ? [...conversationQueue, pinnedConversation]
      : conversationQueue
    const effectiveTicketQueue = pinnedTicket && !ticketQueue.some((ticket) => ticket.id === pinnedTicket.id)
      ? [...ticketQueue, pinnedTicket]
      : ticketQueue
    const ticketByConversationId = indexWorkbenchTicketsByConversation(
      effectiveTicketQueue,
      routeTicketId || pinnedTicket?.id,
    )
    const conversationItems = effectiveConversationQueue.map((item) =>
      buildConversationQueueItem(item, ticketByConversationId.get(item.id)),
    )
    const conversationIds = new Set(effectiveConversationQueue.map((item) => item.id))
    const ticketItems = effectiveTicketQueue
      .filter((ticket) => !ticket.conversation_id || !conversationIds.has(ticket.conversation_id))
      .map(buildTicketQueueItem)
    return sortWorkbenchQueueItems([...conversationItems, ...ticketItems])
  }, [conversationQueue, pinnedConversation, pinnedTicket, routeTicketId, ticketQueue])

  const displayQueueItems = queueItems
  const filteredQueueItems = displayQueueItems.filter((item) =>
    queueItemMatchesTab(item, queueTab),
  )
  const selectedDisplayItem = displayQueueItems.find((item) => item.key === selectedQueueKey)
  const routedTicketItem = routeTicketId > 0
    ? displayQueueItems.find((item) => item.ticketId === routeTicketId) ?? null
    : null
  const selectedItem = routeTicketId > 0
    ? routedTicketItem
    : selectedDisplayItem ?? filteredQueueItems[0] ?? null
  const selectedConversation =
    selectedItem?.conversationId && conversation?.id === selectedItem.conversationId
      ? conversation
      : selectedItem?.conversation ?? null

  const routeConversationLookupRef = useRef(0)

  useEffect(() => {
    if (routeConversationId <= 0) {
      return
    }
    const item = findWorkbenchRouteItem(displayQueueItems, {
      conversationId: routeConversationId,
    })
    if (item) {
      setSelectedQueueKey(item.key)
      setQueueTab((current) => queueItemMatchesTab(item, current) ? current : "all")
      setMobilePane("chat")
      if (conversation?.id !== routeConversationId) {
        void selectConversation(routeConversationId).catch((error) => {
          toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text097"))
        })
      }
      return
    }
    if (routeConversationLookupRef.current === routeConversationId) {
      return
    }
    routeConversationLookupRef.current = routeConversationId
    void fetchAgentConversationDetail(routeConversationId)
      .then((detail) => setPinnedConversation(buildPinnedWorkbenchConversation(detail)))
      .catch((error) => {
        toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text098"))
      })
    void fetchTickets({ conversation_id: routeConversationId, page: 1, page_size: 1 })
      .then((res) => {
        if (res.success && res.data?.items?.[0]) {
          setPinnedTicket(res.data.items[0])
        }
      })
      .catch(() => undefined)
  }, [conversation?.id, displayQueueItems, routeConversationId, selectConversation])

  useEffect(() => {
    if (!selectedConversation?.lastMessageAt) return
    void loadWorkbenchQueue(true)
  }, [loadWorkbenchQueue, selectedConversation?.lastMessageAt])

  const { ready: authReady, session: authSession } = useAuth()
  const initialSession = useMemo(() => readSession(), [])
  const currentSession = authReady ? authSession : authSession ?? initialSession
  const permissionSet = useMemo(() => new Set(currentSession?.permissions ?? []), [currentSession?.permissions])
  const canAssignTicket = permissionSet.has("ticket.assign")
  const canManageTicketDispatch = useMemo(() => canManageTicketDispatchFromSession(currentSession), [currentSession])
  const tenantAIEnabled = Boolean(currentSession) && currentSession?.featureFlags?.ai !== false
  const hasDeviceConcept = currentSession?.featureFlags?.device !== false
  const selectedPresence = useMemo(
    () => Object.values(realtimePresence).filter(
      (item) => item.conversationId === selectedItem?.conversationId,
    ),
    [realtimePresence, selectedItem?.conversationId],
  )
  const customerOnline = selectedConversation?.customerOnline ?? false
  const customerLastSeenAt = selectedConversation?.customerLastSeenAt ?? ""
  const partnerOnlineCount = selectedPresence.filter(
    (item) => item.participantType === "partner" && item.online,
  ).length
  const activeTypists = useMemo(
    () => Object.values(realtimeTyping).filter((item) =>
      item.conversationId === selectedItem?.conversationId &&
      item.typing &&
      item.participantId !== currentSession?.user.id,
    ),
    [currentSession?.user.id, realtimeTyping, selectedItem?.conversationId],
  )
  const typingLabel = activeTypists.some((item) => item.participantType === "customer")
    ? ee("ticketWorkbench.text099")
    : activeTypists.some((item) => item.participantType === "partner")
      ? ee("ticketWorkbench.text100")
      : ""
  const selectedTicketRequiresAcceptance = Boolean(
    selectedItem?.ticket &&
      ticketStatusRequiresManualAccept(selectedItem.ticket.status),
  )
  const managerReplyOverride = Boolean(
    canManageTicketDispatch &&
      selectedItem?.conversationId &&
      selectedItem?.conversation?.status !== 4,
  )
  const selectedReplyDisabledReason = managerReplyOverride
    ? ""
    : selectedTicketRequiresAcceptance
      ? ee("ticketWorkbench.text101")
      : selectedItem?.conversation?.status !== 3
        ? ee("ticketWorkbench.text102")
        : ""
  const managerCanAcceptSelectedTicket = Boolean(
    canManageTicketDispatch &&
      selectedItem?.ticket &&
      !isTerminalTicketStatus(selectedItem.ticket.status) &&
      ticketStatusRequiresManualAccept(selectedItem.ticket.status),
  )
  const canTakeOverSelectedTicket = selectedItem?.ticket?.actions?.can_takeover === true
  const canAcceptSelectedTicket = Boolean(
    selectedItem?.ticket?.actions?.can_accept === true ||
      canTakeOverSelectedTicket ||
      canShowManualAcceptFallback(selectedItem?.ticket?.status, selectedItem?.ticket?.assignee_id) ||
      managerCanAcceptSelectedTicket,
  )
  const canAssignConversation = permissionSet.has("conversation.assign")
  const canTransferConversation = permissionSet.has("conversation.transfer")

  useEffect(() => {
    if (routeTicketId <= 0) {
      return
    }
    const item = displayQueueItems.find((candidate) => candidate.ticketId === routeTicketId)
    if (!item) {
      let cancelled = false
      void fetchTicketAggregate(routeTicketId)
        .then((res) => {
          if (cancelled) {
            return
          }
          if (!res.success || !res.data) {
            toast.error(res.error?.message || ee("ticketWorkbench.text103"))
            return
          }
          const pinnedTicket = buildTicketListItemFromAggregate(res.data)
          const pinnedItem = buildTicketQueueItem(pinnedTicket)
          const linkedConversationItem = pinnedItem.conversationId
            ? displayQueueItems.find(
              (candidate) =>
                candidate.kind === "conversation" &&
                candidate.conversationId === pinnedItem.conversationId,
            )
            : null
          const nextItem = linkedConversationItem
            ? { ...pinnedItem, key: linkedConversationItem.key }
            : pinnedItem
          setPinnedTicket(pinnedTicket)
          setSelectedQueueKey(nextItem.key)
          setQueueTab((current) => queueItemMatchesTab(nextItem, current) ? current : "all")
          setMobilePane("chat")
          if (pinnedItem.conversationId && conversation?.id !== pinnedItem.conversationId) {
            void selectConversation(pinnedItem.conversationId).catch((error) => {
              toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text097"))
            })
          }
        })
        .catch((error) => {
          if (!cancelled) {
            toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text103"))
          }
        })
      return () => {
        cancelled = true
      }
    }
    setSelectedQueueKey(item.key)
    setQueueTab((current) => queueItemMatchesTab(item, current) ? current : "all")
    setMobilePane("chat")
    if (item.conversationId && conversation?.id !== item.conversationId) {
      void selectConversation(item.conversationId).catch((error) => {
        toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text097"))
      })
    }
  }, [conversation?.id, displayQueueItems, routeTicketId, selectConversation])

  useEffect(() => {
    if (selectedDisplayItem && !queueItemMatchesTab(selectedDisplayItem, queueTab)) {
      setQueueTab("all")
    }
  }, [queueTab, selectedDisplayItem])

  useEffect(() => {
    if (!selectedItem) {
      if (selectedQueueKey) {
        setSelectedQueueKey("")
      }
      return
    }
    if (selectedItem && selectedQueueKey !== selectedItem.key) {
      setSelectedQueueKey(selectedItem.key)
    }
  }, [displayQueueItems, selectedItem, selectedQueueKey])

  useEffect(() => {
    setComposerText("")
    setTranslatedMessages({})
    setTranslatingMessageId(null)
  }, [selectedItem?.key])

  useEffect(() => {
    if (!selectedItem?.isDone) return
    setComposerText("")
    if (selectedItem.conversationId) {
      sendAgentConversationTyping(selectedItem.conversationId, false)
    }
  }, [selectedItem?.conversationId, selectedItem?.isDone])

  useEffect(() => {
    const conversationId = selectedItem?.conversationId ?? 0
    if (conversationId <= 0 || selectedItem?.isDone || selectedTicketRequiresAcceptance) return
    const typing = composerText.trim().length > 0
    sendAgentConversationTyping(conversationId, typing)
    if (!typing) return
    const timer = window.setTimeout(() => {
      sendAgentConversationTyping(conversationId, false)
    }, TYPING_IDLE_TIMEOUT_MS)
    return () => window.clearTimeout(timer)
  }, [composerText, selectedItem?.conversationId, selectedItem?.isDone, selectedTicketRequiresAcceptance])

  useEffect(() => () => {
    if (selectedItem?.conversationId) {
      sendAgentConversationTyping(selectedItem.conversationId, false)
    }
  }, [selectedItem?.conversationId])

  useEffect(() => {
    if (!selectedItem?.conversationId || conversation?.id === selectedItem.conversationId) {
      return
    }
    void selectConversation(selectedItem.conversationId).catch((error) => {
      toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text097"))
    })
  }, [conversation?.id, selectConversation, selectedItem?.conversationId])

  useEffect(() => {
    if (!selectedItem?.conversationId || conversation?.id !== selectedItem.conversationId) {
      return
    }
    void markSelectedConversationRead().catch(() => undefined)
  }, [conversation?.id, markSelectedConversationRead, messages.length, selectedItem?.conversationId])

  const queueCounts = useMemo(
    () => ({
      all: displayQueueItems.length,
      active: displayQueueItems.filter((item) => !item.isDone).length,
      done: displayQueueItems.filter((item) => item.isDone).length,
    }),
    [displayQueueItems],
  )

  const chatMessages = useMemo(
    () => buildChatMessages(selectedItem, messagesLoadedConversationId, messages),
    [messages, messagesLoadedConversationId, selectedItem],
  )

  const handleSelectQueueItem = useCallback(
    (item: WorkbenchQueueItem) => {
      setSelectedQueueKey(item.key)
      setPinnedTicket((current) => current?.id === item.ticketId ? current : null)
      setRouteTicketId(item.ticketId ?? 0)
      setRouteConversationId(item.ticketId ? 0 : (item.conversationId ?? 0))
      syncWorkbenchRoute(item)
      setMobilePane("chat")
      if (item.conversationId) {
        void selectConversation(item.conversationId).catch((error) => {
          toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text097"))
        })
      }
    },
    [selectConversation],
  )

  const handleQueueTabChange = useCallback((nextTab: QueueTab) => {
    setQueueTab(nextTab)
    const current = displayQueueItems.find((item) => item.key === selectedQueueKey)
    if (current && queueItemMatchesTab(current, nextTab)) {
      return
    }
    const nextItem = displayQueueItems.find((item) => queueItemMatchesTab(item, nextTab)) ?? null
    setSelectedQueueKey(nextItem?.key ?? "")
    setPinnedTicket((current) => current?.id === nextItem?.ticketId ? current : null)
    setRouteTicketId(nextItem?.ticketId ?? 0)
    setRouteConversationId(nextItem?.ticketId ? 0 : (nextItem?.conversationId ?? 0))
    syncWorkbenchRoute(nextItem)
  }, [displayQueueItems, selectedQueueKey])

  const handleRefresh = useCallback(async () => {
    await Promise.all([
      loadWorkbenchQueue(),
      selectedItem?.ticketId ? refreshTicketSnapshot(selectedItem.ticketId) : Promise.resolve(),
    ])
    if (selectedItem?.conversationId) {
      await syncLatestMessages(selectedItem.conversationId)
    }
  }, [loadWorkbenchQueue, refreshTicketSnapshot, selectedItem?.conversationId, selectedItem?.ticketId, syncLatestMessages])

  const handleConversationActionSuccess = useCallback(async () => {
    await loadWorkbenchQueue(true)
    if (selectedItem?.conversationId) {
      await syncLatestMessages(selectedItem.conversationId)
    }
  }, [loadWorkbenchQueue, selectedItem?.conversationId, syncLatestMessages])

  const handleSendImage = useCallback(async (file: File) => {
    try {
      await sendImage(file)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text104"))
    }
  }, [sendImage])

  const handleSendAttachment = useCallback(async (file: File) => {
    try {
      await sendAttachment(file)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text105"))
    }
  }, [sendAttachment])

  const handleSendVoice = useCallback(async (file: File, durationSeconds: number) => {
    try {
      await sendVoice(file, durationSeconds)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text106"))
    }
  }, [sendVoice])

  const handleSend = useCallback(async () => {
    const content = composerText.trim()
    if (!content) {
      return
    }
    if (
      !selectedItem?.conversationId ||
      selectedItem.isDone ||
      selectedConversationId !== selectedItem.conversationId
    ) {
      toast.info(ee("ticketWorkbench.text107"))
      return
    }
    try {
      const sent = await sendMessage(content)
      if (sent) {
        setComposerText("")
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text108"))
    }
  }, [composerText, selectedConversationId, selectedItem, sendMessage])

  const handleAcceptTicket = useCallback(async (ticketId?: number, conversationId?: number) => {
    const ticket = selectedItem?.ticket
    const targetTicketId = ticketId ?? ticket?.id ?? selectedItem?.ticketId ?? 0
    const targetConversationId = conversationId ?? selectedItem?.conversationId ?? ticket?.conversation_id ?? 0
    if (!targetTicketId || acceptingTicket || (!ticketId && !canAcceptSelectedTicket)) {
      return
    }
    setAcceptingTicket(true)
    try {
      const res = await acceptTicket(targetTicketId)
      if (!res.success) {
        toast.error(res.error?.message || (canTakeOverSelectedTicket ? ee("ticketWorkbench.text109") : ee("ticketWorkbench.text110")))
        return
      }
      await loadWorkbenchQueue(true)
      if (targetConversationId) {
        await selectConversation(targetConversationId)
      }
      toast.success(canTakeOverSelectedTicket ? ee("ticketWorkbench.text111") : ee("ticketWorkbench.text112"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : canTakeOverSelectedTicket ? ee("ticketWorkbench.text109") : ee("ticketWorkbench.text110"))
    } finally {
      await refreshTicketSnapshot(targetTicketId)
      setAcceptingTicket(false)
    }
  }, [acceptingTicket, canAcceptSelectedTicket, canTakeOverSelectedTicket, loadWorkbenchQueue, refreshTicketSnapshot, selectConversation, selectedItem])

  const handleTakeoverTicket = useCallback(async (ticketId?: number, conversationId?: number) => {
    const ticket = selectedItem?.ticket
    const targetTicketId = ticketId ?? ticket?.id ?? selectedItem?.ticketId ?? 0
    const targetConversationId = conversationId ?? selectedItem?.conversationId ?? ticket?.conversation_id ?? 0
    if (!targetTicketId || acceptingTicket || (!ticketId && !canTakeOverSelectedTicket)) {
      return
    }
    setAcceptingTicket(true)
    try {
      const res = await takeoverTicket(targetTicketId)
      if (!res.success) {
        toast.error(res.error?.message || ee("ticketWorkbench.text109"))
        return
      }
      await loadWorkbenchQueue(true)
      if (targetConversationId) {
        await selectConversation(targetConversationId)
      }
      toast.success(ee("ticketWorkbench.text111"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text109"))
    } finally {
      await refreshTicketSnapshot(targetTicketId)
      setAcceptingTicket(false)
    }
  }, [acceptingTicket, canTakeOverSelectedTicket, loadWorkbenchQueue, refreshTicketSnapshot, selectConversation, selectedItem])

  const handleTranslationTargetLanguageChange = useCallback((language: string) => {
    setTranslationTargetLanguage(language)
    setTranslatedMessages({})
  }, [])

  const handleTranslateMessage = useCallback(
    async (message: WorkbenchMessage) => {
      if (!message.id || !selectedItem?.conversationId || translatingMessageId !== null) {
        return
      }
      const targetLanguage = resolveMessageTranslationTarget(message.content, translationTargetLanguage)
      if (targetLanguage !== translationTargetLanguage) {
        handleTranslationTargetLanguageChange(targetLanguage)
        toast.info(ee("ticketWorkbench.text113", { value0: translationLanguageLabel(translationTargetLanguage), value1: translationLanguageLabel(targetLanguage) }))
      }
      setTranslatingMessageId(message.id)
      try {
        const res = await translateConversationMessage(
          selectedItem.conversationId,
          message.id,
          targetLanguage,
        )
        if (!res.success || !res.data) {
          toast.error(res.error?.message || ee("ticketWorkbench.text114"))
          return
        }
        setTranslatedMessages((current) => ({ ...current, [message.id!]: res.data }))
      } catch (error) {
        toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text114"))
      } finally {
        setTranslatingMessageId(null)
      }
    },
    [handleTranslationTargetLanguageChange, selectedItem?.conversationId, translatingMessageId, translationTargetLanguage],
  )

  return (
    <>
      <PageShell
        title={ee("ticketWorkbench.text115")}
        breadcrumb={useRouteBreadcrumbItems()}
        className="rhd-railops-ticket-workbench-page"
        actions={
          <>
            {queueError ? (
              <span className="hidden max-w-48 truncate text-xs text-destructive sm:block" title={queueError}>{ee("ticketWorkbench.text116")}</span>
            ) : null}
            <IconButton
              icon={<SettingsIcon className="size-4" />}
              tooltip={ee("ticketWorkbench.text117")}
              aria-label={ee("ticketWorkbench.text117")}
              className="size-9"
              onClick={() => setAutoCloseOpen(true)}
            />
            <RailopsButton size="small" className="gap-2" onClick={() => void handleRefresh()}>
              <RefreshCwIcon className={cn("size-4", queueLoading && "animate-spin")} />{ee("ticketWorkbench.text118")}</RailopsButton>
          </>
        }
      >

        <div className="rhd-railops-ticket-workbench-shell flex h-[calc(100svh-172px)] min-h-[500px] flex-col overflow-hidden rounded-md border border-border bg-card text-foreground shadow-sm">
          <header className="rhd-railops-ticket-workbench-header flex min-h-12 shrink-0 items-center justify-between gap-3 border-b border-border bg-card px-3 py-2 lg:px-4">
            <div className="min-w-0">
              <h2 className="truncate text-sm font-semibold leading-tight">
                {selectedItem ? selectedItem.ticketNo : ee("ticketWorkbench.text119")}
              </h2>
              <div className="mt-0.5 flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
                <span className="truncate">
                  {selectedItem ? selectedItem.customer : ee("ticketWorkbench.text120")}
                </span>
                {selectedItem ? (
                  <StatusTag
                    tone="neutral"
                    className={cn("h-5 px-1.5 text-rhd-xs", statusToneClassName(selectedItem.statusTone))}
                  >
                    {selectedItem.statusLabel}
                  </StatusTag>
                ) : null}
              </div>
            </div>
          </header>

      <div className="rhd-railops-ticket-workbench-mobile-tabs bg-muted/20 xl:hidden">
        <UnderlineTabs
          ariaLabel={ee("ticketWorkbench.text121")}
          items={mobilePaneTabs}
          value={mobilePane}
          onChange={(value) => setMobilePane(value as MobilePane)}
        />
      </div>

      <div className="rhd-railops-ticket-workbench-grid grid min-h-0 flex-1 grid-cols-1 bg-background xl:grid-cols-[280px_minmax(420px,1fr)_320px]">
        <section
          className={cn(
            "rhd-railops-ticket-workbench-queue-pane min-h-0 bg-card xl:block xl:border-r xl:border-border",
            mobilePane === "queue" ? "block" : "hidden",
          )}
        >
          <WorkbenchQueuePane
            items={filteredQueueItems}
            counts={queueCounts}
            initialLoading={queueLoading && !queueLoaded}
            loading={queueLoading}
            selectedKey={selectedItem?.key ?? ""}
            tab={queueTab}
            onTabChange={handleQueueTabChange}
            onSelect={handleSelectQueueItem}
          />
        </section>

        <section
          className={cn(
            "rhd-railops-ticket-workbench-chat-pane min-h-0 bg-card xl:block",
            mobilePane === "chat" ? "block" : "hidden",
          )}
        >
          <ConversationChatPane
            item={selectedItem}
            messages={chatMessages}
            customerId={selectedConversation?.customerId ?? 0}
            loadingMessages={messagesLoading && Boolean(selectedItem?.conversationId)}
            loadingOlderMessages={messagesLoadingMore}
            hasMoreOlderMessages={messagesHasMore}
            composerText={composerText}
            sending={sending}
            uploadingAsset={uploadingAsset}
            translationTargetLanguage={translationTargetLanguage}
            translatedMessages={translatedMessages}
            translatingMessageId={translatingMessageId}
            acceptingTicket={acceptingTicket}
            canAssignConversation={canAssignConversation}
            canTransferConversation={canTransferConversation}
            canAcceptTicket={canAcceptSelectedTicket}
            canTakeOverTicket={canTakeOverSelectedTicket}
            replyDisabledReason={selectedReplyDisabledReason}
            realtimeStatus={realtimeStatus}
            customerOnline={customerOnline}
            customerLastSeenAt={customerLastSeenAt}
            partnerOnlineCount={partnerOnlineCount}
            typingLabel={typingLabel}
            hasDeviceConcept={hasDeviceConcept}
            onBack={() => setMobilePane("queue")}
            onLoadOlderMessages={loadOlderMessages}
            onComposerChange={setComposerText}
            onSend={() => void handleSend()}
            onSendImage={handleSendImage}
            onSendAttachment={handleSendAttachment}
            onSendVoice={handleSendVoice}
            onTranslationTargetLanguageChange={handleTranslationTargetLanguageChange}
            onTranslateMessage={handleTranslateMessage}
            onAcceptTicket={() => void handleAcceptTicket()}
            onTakeoverTicket={() => void handleTakeoverTicket()}
            onAssign={() => {
              setConversationTransferMode("assign")
              setConversationTransferOpen(true)
            }}
            onTransfer={() => {
              setConversationTransferMode("transfer")
              setConversationTransferOpen(true)
            }}
            onClose={() => setConversationCloseOpen(true)}
          />
        </section>

        <section
          className={cn(
            "rhd-railops-ticket-workbench-context-pane min-h-0 bg-card xl:block xl:border-l xl:border-border",
            mobilePane === "context" ? "block" : "hidden",
          )}
        >
          <WorkbenchContextPane
            item={selectedItem}
            conversation={selectedConversation}
            canAssignTicket={canAssignTicket}
            tenantAIEnabled={tenantAIEnabled}
            hasDeviceConcept={hasDeviceConcept}
            acceptingTicket={acceptingTicket}
            onAcceptTicket={handleAcceptTicket}
            onTakeoverTicket={handleTakeoverTicket}
            onTicketUpdated={async () => {
              await loadWorkbenchQueue(true)
            }}
          />
        </section>
      </div>
        </div>
    </PageShell>
      <ConversationTransferDialog
        open={conversationTransferOpen}
        mode={conversationTransferMode}
        conversationId={selectedItem?.conversationId ?? null}
        onOpenChange={setConversationTransferOpen}
        onSuccess={handleConversationActionSuccess}
      />
      <ConversationCloseDialog
        open={conversationCloseOpen}
        conversationId={selectedItem?.conversationId ?? null}
        onOpenChange={setConversationCloseOpen}
        onSuccess={handleConversationActionSuccess}
      />
		<StandardModal
  open={autoCloseOpen}
  onCancel={ () => setAutoCloseOpen(false) }
  title={ee("ticketWorkbench.text122")}
  width={448}
  footer={
    <>
					<RailopsButton onClick={() => setAutoCloseOpen(false)}>{ee("ticketWorkbench.text123")}</RailopsButton>
					<RailopsButton onClick={() => void handleSaveAutoClosePolicy()} disabled={loadingAutoClosePolicy || savingAutoClosePolicy}>
						{savingAutoClosePolicy ? <Loader2Icon className="size-4 animate-spin" /> : null}{ee("ticketWorkbench.text124")}</RailopsButton>

    </>
  }
>
				{loadingAutoClosePolicy ? (
					<LoadingLine text={ee("ticketWorkbench.text125")} />
				) : (
					<div className="space-y-4 py-1">
						<div className="flex items-center justify-between gap-4 text-sm">
							<span>{ee("ticketWorkbench.text126")}</span>
							<CheckboxField
								style={{ marginBottom: 0 }}
								checkboxProps={{
									checked: autoClosePolicy.enabled,
									onChange: (event) =>
										setAutoClosePolicy((current) => ({ ...current, enabled: event.target.checked })),
								}}
							/>
						</div>
						<label className="grid gap-2 text-sm">
							<span>{ee("ticketWorkbench.text127")}</span>
							<input
								type="number"
								min={1}
								max={90}
								value={autoClosePolicy.days}
								onChange={(event) =>
									setAutoClosePolicy((current) => ({
										...current,
										days: Math.max(1, Math.min(90, Number(event.target.value) || 1)),
									}))
								}
								className="h-9 rounded-md border border-border px-3 outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
							/>
						</label>
					</div>
				)}
				
</StandardModal>
    </>
  )
}

function WorkbenchQueuePane({
  items,
  counts,
  initialLoading,
  loading,
  selectedKey,
  tab,
  onTabChange,
  onSelect,
}: {
  items: WorkbenchQueueItem[]
  counts: Record<QueueTab, number>
  initialLoading: boolean
  loading: boolean
  selectedKey: string
  tab: QueueTab
  onTabChange: (tab: QueueTab) => void
  onSelect: (item: WorkbenchQueueItem) => void
}) {
  const tabs: Array<{ key: QueueTab; label: string; count: number }> = [
    { key: "all", label: ee("ticketWorkbench.text128"), count: counts.all },
    { key: "active", label: ee("ticketWorkbench.text011"), count: counts.active },
    { key: "done", label: ee("ticketWorkbench.text012"), count: counts.done },
  ]

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 gap-1 border-b border-border p-2.5">
        {tabs.map((item) => (
          <button
            key={item.key}
            type="button"
            className={cn(
              "flex min-h-8 flex-1 items-center justify-center gap-1 whitespace-nowrap rounded-md px-2 text-xs font-medium transition-colors",
              tab === item.key
                ? "bg-primary/10 text-primary"
                : "text-muted-foreground hover:bg-muted hover:text-foreground",
            )}
            onClick={() => onTabChange(item.key)}
          >
            {item.label}
            <span
              className={cn(
                "shrink-0 whitespace-nowrap rounded-full px-1.5 text-rhd-xs",
                tab === item.key ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground",
              )}
            >
              {item.count}
            </span>
          </button>
        ))}
      </div>
      <ScrollArea className="min-h-0 flex-1">
        {initialLoading ? (
          <div className="flex items-center justify-center gap-2 p-6 text-sm text-muted-foreground">
            <Loader2Icon className="size-4 animate-spin" />{ee("ticketWorkbench.text129")}</div>
        ) : items.length > 0 ? (
          <div className="grid gap-2 p-2.5">
            {items.map((item) => (
              <WorkbenchQueueCard
                key={item.key}
                item={item}
                selected={item.key === selectedKey}
                onSelect={() => onSelect(item)}
              />
            ))}
          </div>
		) : (
			<div className="flex flex-col items-center gap-2 p-6 text-center text-sm text-muted-foreground">
				<span>
					{tab === "active"
						? ee("ticketWorkbench.text130")
						: tab === "done"
							? ee("ticketWorkbench.text131")
							: ee("ticketWorkbench.text132")}
				</span>
				{tab === "active" && counts.done > 0 ? (
					<RailopsButton variant="text" size="small" style={{ justifyContent: "flex-start" }} onClick={() => onTabChange("done")}>{ee("ticketWorkbench.text133")}</RailopsButton>
				) : null}
			</div>
		)}
      </ScrollArea>
    </div>
  )
}

function WorkbenchQueueCard({
  item,
  selected,
  onSelect,
}: {
  item: WorkbenchQueueItem
  selected: boolean
  onSelect: () => void
}) {
  const initial = item.customer.slice(0, 1).toUpperCase() || "C"
  return (
      <button
        type="button"
        className={cn(
          "grid w-full grid-cols-[2rem_minmax(0,1fr)] gap-2 rounded-md border p-2.5 text-left transition-colors",
          selected
          ? "border-primary/20 bg-primary/10"
          : "border-border bg-card hover:border-primary/20 hover:bg-muted/40",
        )}
        onClick={onSelect}
      >
      <span className="relative flex size-8 items-center justify-center rounded-full bg-muted text-xs font-semibold text-foreground">
        {initial}
        <span
          className={cn(
            "absolute bottom-0 right-0 size-2.5 rounded-full border-2 border-card",
            item.isDone ? "bg-muted-foreground" : "bg-primary",
          )}
        />
      </span>
      <span className="min-w-0">
        <span className="flex min-w-0 items-center gap-2">
          <strong className="min-w-0 flex-1 truncate text-xs font-semibold text-foreground">
            {item.customer}
          </strong>
          <time className="shrink-0 text-rhd-xs text-muted-foreground">{item.updatedLabel}</time>
        </span>
        <span className="mt-1 line-clamp-2 text-xs leading-4 text-muted-foreground">
          {item.title}
        </span>
        {item.dispatchLabel ? (
          <span className="mt-1 inline-flex max-w-full items-center gap-1 rounded border border-amber-200 bg-amber-50 px-1.5 py-0.5 text-rhd-xs font-medium text-foreground">
            <CircleAlertIcon className="size-3 shrink-0" />
            <span className="truncate">{item.dispatchLabel}</span>
          </span>
        ) : null}
        <span className="mt-1.5 flex min-w-0 items-center justify-between gap-2">
          <span className="min-w-0 truncate text-rhd-xs text-muted-foreground">
            {item.ticketNo} · {item.owner}
          </span>
          <span className="flex shrink-0 items-center gap-1">
            <StatusTag
              tone="neutral"
              className={cn("h-5 px-1.5 text-rhd-xs", priorityClassName(item.priorityLabel))}
            >
              {item.priorityLabel}
            </StatusTag>
            <StatusTag
              tone="neutral"
              className={cn("h-5 px-1.5 text-rhd-xs", statusToneClassName(item.statusTone))}
            >
              {item.statusLabel}
            </StatusTag>
          {item.unread ? (
            <span className="flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full bg-primary px-1.5 text-rhd-xs font-semibold text-primary-foreground">
              {item.unread > 99 ? "99+" : item.unread}
            </span>
          ) : null}
          </span>
        </span>
      </span>
    </button>
  )
}

function getWorkbenchMessageSignature(message: WorkbenchMessage | null | undefined) {
  if (!message) return ""
  return `${message.id ?? "local"}:${message.time}:${message.senderType}:${message.html.length}`
}

function ConversationChatPane({
  item,
  messages,
  customerId,
  loadingMessages,
  loadingOlderMessages,
  hasMoreOlderMessages,
  composerText,
  sending,
  uploadingAsset,
  translationTargetLanguage,
  translatedMessages,
  translatingMessageId,
  acceptingTicket,
  canAssignConversation,
  canTransferConversation,
  canAcceptTicket,
  canTakeOverTicket,
  replyDisabledReason,
  realtimeStatus,
  customerOnline,
  customerLastSeenAt,
  partnerOnlineCount,
  typingLabel,
  hasDeviceConcept,
  onBack,
  onLoadOlderMessages,
  onComposerChange,
  onSend,
  onSendImage,
  onSendAttachment,
  onSendVoice,
  onTranslationTargetLanguageChange,
  onTranslateMessage,
  onAcceptTicket,
  onTakeoverTicket,
  onAssign,
  onTransfer,
  onClose,
}: {
  item: WorkbenchQueueItem | null
  messages: WorkbenchMessage[]
  customerId: number
  loadingMessages: boolean
  loadingOlderMessages: boolean
  hasMoreOlderMessages: boolean
  composerText: string
  sending: boolean
  uploadingAsset: boolean
  translationTargetLanguage: string
  translatedMessages: Record<number, ConversationMessageTranslationDTO>
  translatingMessageId: number | null
  acceptingTicket: boolean
  canAssignConversation: boolean
  canTransferConversation: boolean
  canAcceptTicket: boolean
  canTakeOverTicket: boolean
  replyDisabledReason: string
  realtimeStatus: "connecting" | "connected" | "disconnected"
  customerOnline: boolean
  customerLastSeenAt: string
  partnerOnlineCount: number
  typingLabel: string
  hasDeviceConcept: boolean
  onBack: () => void
  onLoadOlderMessages: () => Promise<void>
  onComposerChange: (value: string) => void
  onSend: () => void
  onSendImage: (file: File) => Promise<void>
  onSendAttachment: (file: File) => Promise<void>
  onSendVoice: (file: File, durationSeconds: number) => Promise<void>
  onTranslationTargetLanguageChange: (value: string) => void
  onTranslateMessage: (message: WorkbenchMessage) => Promise<void>
  onAcceptTicket: () => void
  onTakeoverTicket: () => void
  onAssign: () => void
  onTransfer: () => void
  onClose: () => void
}) {
  const replyDisabled = replyDisabledReason !== ""
  const dispatchEvidence = dispatchEvidenceParts(item?.ticket, hasDeviceConcept)
  const imageInputRef = useRef<HTMLInputElement>(null)
  const attachmentInputRef = useRef<HTMLInputElement>(null)
  const scrollAreaRef = useRef<HTMLDivElement>(null)
  const contentRef = useRef<HTMLDivElement>(null)
  const scrollFrameRef = useRef<number | null>(null)
  const noticeFrameRef = useRef<number | null>(null)
  const shouldStickToBottomRef = useRef(true)
  const positionedConversationRef = useRef(0)
  const lastHandledMessageSignatureRef = useRef("")
  const [newMessageNoticeCount, setNewMessageNoticeCount] = useState(0)
  const [senderProfiles, setSenderProfiles] = useState<Record<number, AdminAgentProfile>>({})
  const [customerCardProfiles, setCustomerCardProfiles] = useState<Record<number, CustomerCardProfile | null>>({})
  const lastMessage = messages[messages.length - 1] ?? null
  const lastMessageSignature = getWorkbenchMessageSignature(lastMessage)
  const customerCardProfile = customerId > 0 ? customerCardProfiles[customerId] ?? null : null

  useEffect(() => {
    const missingEngineerIds = Array.from(new Set(
      messages
        .filter((message) => message.tone === "engineer" && message.senderId && !senderProfiles[message.senderId])
        .map((message) => message.senderId as number),
    ))
    if (missingEngineerIds.length === 0) {
      return
    }

    let cancelled = false
    void Promise.all(
      missingEngineerIds.map(async (id) => {
        try {
          return [id, await fetchAgentProfile(id)] as const
        } catch {
          return null
        }
      }),
    ).then((entries) => {
      if (cancelled) {
        return
      }
      const nextEntries = entries.filter((entry): entry is readonly [number, AdminAgentProfile] => Boolean(entry))
      if (nextEntries.length === 0) {
        return
      }
      setSenderProfiles((current) => {
        const next = { ...current }
        for (const [id, profile] of nextEntries) {
          next[id] = profile
        }
        return next
      })
    })

    return () => {
      cancelled = true
    }
  }, [messages, senderProfiles])

  useEffect(() => {
    if (customerId <= 0 || Object.prototype.hasOwnProperty.call(customerCardProfiles, customerId)) {
      return
    }

    let cancelled = false
    void Promise.allSettled([
      fetchCustomer(customerId),
      fetchCustomerContacts(customerId),
    ]).then(([customerResult, contactsResult]) => {
      if (cancelled) {
        return
      }
      const customer = customerResult.status === "fulfilled" ? customerResult.value : null
      const contacts = contactsResult.status === "fulfilled" ? contactsResult.value : []
      setCustomerCardProfiles((current) => ({
        ...current,
        [customerId]: buildCustomerCardProfile(customer, contacts),
      }))
    })

    return () => {
      cancelled = true
    }
  }, [customerCardProfiles, customerId])

  const getScrollViewport = useCallback(() => {
    return scrollAreaRef.current?.querySelector<HTMLElement>('[data-slot="scroll-area-viewport"]') ?? null
  }, [])

  const isNearBottom = useCallback((element: HTMLElement, threshold = 96) => {
    return element.scrollHeight - element.scrollTop - element.clientHeight <= threshold
  }, [])

  const scheduleScrollToBottom = useCallback((behavior: ScrollBehavior = "auto", attempts = 4) => {
    if (scrollFrameRef.current !== null) {
      window.cancelAnimationFrame(scrollFrameRef.current)
    }

    const run = (remaining: number, previousHeight = -1) => {
      scrollFrameRef.current = window.requestAnimationFrame(() => {
        const viewport = getScrollViewport()
        if (!viewport) {
          scrollFrameRef.current = null
          return
        }
        const currentHeight = viewport.scrollHeight
        viewport.scrollTo({ top: currentHeight, behavior })
        if (remaining > 1 && currentHeight !== previousHeight) {
          run(remaining - 1, currentHeight)
          return
        }
        scrollFrameRef.current = null
      })
    }

    run(attempts)
  }, [getScrollViewport])

  const scheduleNewMessageNotice = useCallback((update: number | ((count: number) => number)) => {
    if (noticeFrameRef.current !== null) {
      window.cancelAnimationFrame(noticeFrameRef.current)
    }
    noticeFrameRef.current = window.requestAnimationFrame(() => {
      noticeFrameRef.current = null
      setNewMessageNoticeCount(update)
    })
  }, [])

  useEffect(() => {
    const viewport = getScrollViewport()
    const content = contentRef.current
    if (!viewport || !content) {
      return
    }

    const handleScroll = () => {
      const nearBottom = isNearBottom(viewport)
      shouldStickToBottomRef.current = nearBottom
      if (nearBottom) {
        setNewMessageNoticeCount(0)
      }
    }
    const resizeObserver = new ResizeObserver(() => {
      if (shouldStickToBottomRef.current) {
        scheduleScrollToBottom("auto")
      }
    })

    handleScroll()
    viewport.addEventListener("scroll", handleScroll)
    resizeObserver.observe(viewport)
    resizeObserver.observe(content)

    return () => {
      viewport.removeEventListener("scroll", handleScroll)
      resizeObserver.disconnect()
    }
  }, [getScrollViewport, isNearBottom, scheduleScrollToBottom])

  useLayoutEffect(() => {
    const conversationId = item?.conversationId ?? 0
    if (!conversationId) {
      positionedConversationRef.current = 0
      lastHandledMessageSignatureRef.current = ""
      scheduleNewMessageNotice(0)
      return
    }
    const conversationChanged = positionedConversationRef.current !== conversationId
    positionedConversationRef.current = conversationId
    if (conversationChanged) {
      shouldStickToBottomRef.current = true
      scheduleNewMessageNotice(0)
    }
    if (messages.length === 0 || !lastMessageSignature) {
      lastHandledMessageSignatureRef.current = ""
      return
    }
    if (conversationChanged) {
      lastHandledMessageSignatureRef.current = lastMessageSignature
      scheduleScrollToBottom("auto")
      return
    }
    if (lastHandledMessageSignatureRef.current === lastMessageSignature) {
      return
    }
    const previousSignature = lastHandledMessageSignatureRef.current
    lastHandledMessageSignatureRef.current = lastMessageSignature
    if (shouldStickToBottomRef.current) {
      scheduleNewMessageNotice(0)
      scheduleScrollToBottom("smooth")
      return
    }
    const previousMessageIndex = previousSignature
      ? messages.findIndex((message) => getWorkbenchMessageSignature(message) === previousSignature)
      : -1
    const addedCount = previousMessageIndex >= 0
      ? Math.max(1, messages.length - previousMessageIndex - 1)
      : 1
    scheduleNewMessageNotice((count) => Math.min(count + addedCount, 99))
  }, [item?.conversationId, lastMessageSignature, messages, scheduleNewMessageNotice, scheduleScrollToBottom])

  useEffect(() => {
    return () => {
      if (scrollFrameRef.current !== null) {
        window.cancelAnimationFrame(scrollFrameRef.current)
        scrollFrameRef.current = null
      }
      if (noticeFrameRef.current !== null) {
        window.cancelAnimationFrame(noticeFrameRef.current)
        noticeFrameRef.current = null
      }
    }
  }, [])

  const handleLoadOlderMessages = useCallback(async () => {
    if (!hasMoreOlderMessages || loadingOlderMessages) {
      return
    }
    const viewport = getScrollViewport()
    const anchor = viewport
      ? { height: viewport.scrollHeight, top: viewport.scrollTop }
      : null
    try {
      await onLoadOlderMessages()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text134"))
      return
    }
    if (!anchor) {
      return
    }
    window.requestAnimationFrame(() => {
      window.requestAnimationFrame(() => {
        const current = getScrollViewport()
        if (!current) {
          return
        }
        current.scrollTop = current.scrollHeight - anchor.height + anchor.top
      })
    })
  }, [getScrollViewport, hasMoreOlderMessages, loadingOlderMessages, onLoadOlderMessages])

  const handleJumpToLatestMessages = useCallback(() => {
    shouldStickToBottomRef.current = true
    setNewMessageNoticeCount(0)
    scheduleScrollToBottom("smooth")
  }, [scheduleScrollToBottom])

  const handleSendRequest = useCallback(() => {
    handleJumpToLatestMessages()
    onSend()
  }, [handleJumpToLatestMessages, onSend])

  const handleSendImageRequest = useCallback(async (file: File) => {
    handleJumpToLatestMessages()
    await onSendImage(file)
  }, [handleJumpToLatestMessages, onSendImage])

  const handleSendAttachmentRequest = useCallback(async (file: File) => {
    handleJumpToLatestMessages()
    await onSendAttachment(file)
  }, [handleJumpToLatestMessages, onSendAttachment])

  const handleSendVoiceRequest = useCallback(async (file: File, durationSeconds: number) => {
    handleJumpToLatestMessages()
    await onSendVoice(file, durationSeconds)
  }, [handleJumpToLatestMessages, onSendVoice])

  if (!item) {
    return (
      <div className="flex h-full min-h-0 items-center justify-center text-sm text-muted-foreground">{ee("ticketWorkbench.text135")}</div>
    )
  }

  return (
      <div className="grid h-full min-h-0 grid-rows-[auto_auto_auto_minmax(0,1fr)_auto] bg-card">
      <header className="flex min-h-14 shrink-0 items-center justify-between gap-2.5 border-b border-border px-3 py-2">
        <div className="flex min-w-0 items-center gap-2.5">
          <button
            type="button"
            className="flex size-8 shrink-0 items-center justify-center rounded-md border border-border text-muted-foreground xl:hidden"
            onClick={onBack}
            aria-label={ee("ticketWorkbench.text136")}
          >
            <ChevronLeftIcon className="size-4" />
          </button>
          <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-semibold text-foreground">
            {item.customer.slice(0, 1).toUpperCase()}
          </span>
          <div className="min-w-0">
            <div className="flex min-w-0 items-center gap-2">
              <strong className="truncate text-sm font-semibold text-foreground">
                {item.customer}
              </strong>
              <StatusTag
                tone="neutral"
                className={cn("h-5 px-1.5 text-rhd-xs", statusToneClassName(item.statusTone))}
              >
                {item.statusLabel}
              </StatusTag>
            </div>
            <div className="mt-0.5 flex items-center gap-2 truncate text-xs text-muted-foreground">
              <span className="truncate">{hasDeviceConcept ? `${item.source} / ${item.device}` : item.source}</span>
            </div>
          </div>
        </div>
        {item.conversationId ? (
          <div className="flex shrink-0 items-center gap-1">
            {item.conversation?.status === 2 && canAssignConversation ? (
              <IconButton
                icon={<UserPlusIcon className="size-4" />}
                tooltip={ee("ticketWorkbench.text137")}
                aria-label={ee("ticketWorkbench.text137")}
                className="size-8"
                onClick={onAssign}
              />
            ) : null}
            {item.conversation?.status === 3 && canTransferConversation ? (
              <IconButton
                icon={<ArrowRightLeftIcon className="size-4" />}
                tooltip={ee("ticketWorkbench.text138")}
                aria-label={ee("ticketWorkbench.text138")}
                className="size-8"
                onClick={onTransfer}
              />
            ) : null}
            {!item.isDone && item.conversation?.status !== 4 ? (
              <IconButton
                icon={<CircleXIcon className="size-4" />}
                tooltip={ee("ticketWorkbench.text139")}
                aria-label={ee("ticketWorkbench.text139")}
                className="size-8"
                onClick={onClose}
              />
            ) : null}
          </div>
        ) : null}
      </header>

      <div className="flex min-w-0 flex-wrap items-center gap-1.5 border-b border-border bg-card px-3 py-1.5 text-xs" aria-live="polite">
        <span className={cn("inline-flex h-7 items-center gap-1.5 rounded-full border px-2.5 font-medium", presencePillClassName(customerOnline))}>
          <span className={cn("size-1.5 rounded-full", customerOnline ? "bg-primary" : "bg-muted-foreground/30")} />
          {customerOnline ? ee("ticketWorkbench.text140") : ee("ticketWorkbench.text141")}
        </span>
        <span className={cn("inline-flex h-7 items-center gap-1.5 rounded-full border px-2.5 font-medium", presencePillClassName(realtimeStatus === "connected", "blue"))}>
          <span className={cn(
            "size-1.5 rounded-full",
            realtimeStatus === "connected" ? "bg-primary" : realtimeStatus === "connecting" ? "animate-pulse bg-amber-500" : "bg-muted-foreground/30",
          )} />
          {engineerRealtimeLabel(realtimeStatus)}
        </span>
        {partnerOnlineCount > 0 ? (
          <span className={cn("inline-flex h-7 items-center gap-1.5 rounded-full border px-2.5 font-medium", presencePillClassName(true))}>
            <span className="size-1.5 rounded-full bg-primary" />{ee("ticketWorkbench.text142")}{partnerOnlineCount}
          </span>
        ) : null}
        <span
          data-testid="enterprise-typing-status"
          className={cn(
            "inline-flex h-7 min-w-0 items-center rounded-full border px-2.5 font-medium",
            typingLabel ? "border-amber-200 bg-amber-50 text-foreground" : "border-border bg-muted text-muted-foreground",
          )}
        >
          {typingLabel ? `${typingLabel}...` : ee("ticketWorkbench.text143")}
        </span>
      </div>

      <div className="flex min-w-0 items-center gap-2.5 border-b border-border bg-muted/20 px-3 py-2">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2 text-rhd-xs text-muted-foreground">
            <span className="font-mono">{item.ticketNo}</span>
            <StatusTag
              tone="neutral"
              className={cn("h-5 px-1.5 text-rhd-xs", priorityClassName(item.priorityLabel))}
            >
              {item.priorityLabel}
            </StatusTag>
            <span>{item.sla}</span>
          </div>
          <div className="mt-0.5 truncate text-sm font-medium text-foreground" title={item.title}>
            {item.title}
          </div>
          {dispatchEvidence.length > 0 ? (
            <div className="mt-1 flex min-w-0 flex-wrap gap-x-3 gap-y-1 text-rhd-xs text-muted-foreground" data-testid="enterprise-dispatch-evidence">
              {dispatchEvidence.map((part) => (
                <span key={part} className="min-w-0 truncate">{part}</span>
              ))}
            </div>
          ) : null}
        </div>
        {canAcceptTicket ? (
          <RailopsButton
            variant="primary"
            size="small"
            className="shrink-0 gap-2 shadow-[0_6px_16px_rgba(59,130,246,0.24)]"
            onClick={canTakeOverTicket ? onTakeoverTicket : onAcceptTicket}
            disabled={acceptingTicket}
          >
            {acceptingTicket ? (
              <Loader2Icon className="size-4 animate-spin" />
            ) : (
              <HandshakeIcon className="size-4" />
            )}
            {acceptingTicket
              ? canTakeOverTicket ? ee("ticketWorkbench.text144") : ee("ticketWorkbench.text145")
              : canTakeOverTicket ? item.ticket?.assignee_id ? ee("ticketWorkbench.text314") : ee("ticketWorkbench.text315") : item.ticket?.status === "reopened" ? ee("ticketWorkbench.text147") : ee("ticketWorkbench.text148")}
          </RailopsButton>
        ) : null}
      </div>

      <div ref={scrollAreaRef} className="relative min-h-0 overflow-hidden">
        <ScrollArea className="h-full min-h-0 bg-[#f7f9fc]">
          <div ref={contentRef} className="space-y-3 p-3">
            {hasMoreOlderMessages && item.conversationId ? (
              <div className="flex justify-center">
                <RailopsButton
                  size="small"
                  className="h-7 rounded-full bg-background/90 text-xs text-muted-foreground shadow-sm hover:bg-background hover:text-primary"
                  disabled={loadingOlderMessages}
                  onClick={() => void handleLoadOlderMessages()}
                >
                  {loadingOlderMessages ? (
                    <Loader2Icon className="size-3.5 animate-spin" />
                  ) : null}
                  {loadingOlderMessages ? ee("ticketWorkbench.text149") : ee("ticketWorkbench.text150")}
                </RailopsButton>
              </div>
            ) : null}
            {loadingMessages && messages.length === 0 ? (
              <div className="flex items-center justify-center gap-2 py-8 text-sm text-muted-foreground">
                <Loader2Icon className="size-4 animate-spin" />{ee("ticketWorkbench.text151")}</div>
            ) : messages.length > 0 ? (
              messages.map((message, index) => (
                <MessageBubble
                  key={message.id ?? `${message.time}-${message.role}-${index}`}
                  message={message}
                  senderProfiles={senderProfiles}
                  customerProfile={customerCardProfile}
                  customerLastSeenAt={customerLastSeenAt}
                  translation={message.id ? translatedMessages[message.id] : undefined}
                  translating={message.id === translatingMessageId}
                  onTranslate={onTranslateMessage}
                />
              ))
            ) : !loadingMessages ? (
              <div className="flex items-center justify-center py-10 text-sm text-muted-foreground">
                {item.conversationId ? ee("ticketWorkbench.text152") : ee("ticketWorkbench.text153")}
              </div>
            ) : null}
          </div>
        </ScrollArea>
        {newMessageNoticeCount > 0 ? (
          <div className="pointer-events-none absolute inset-x-0 bottom-3 z-10 flex justify-center px-4">
            <RailopsButton
              size="small"
              className="pointer-events-auto h-8 rounded-full border border-primary/20 bg-background px-3 text-xs font-medium text-primary shadow-md hover:bg-muted"
              onClick={handleJumpToLatestMessages}
              aria-label={ee("ticketWorkbench.text154")}
            >
              <ArrowDownIcon className="size-3.5" />
              {newMessageNoticeCount > 1 ? ee("ticketWorkbench.text155", { value0: newMessageNoticeCount }) : ee("ticketWorkbench.text156")}
            </RailopsButton>
          </div>
        ) : null}
      </div>

      <footer className="shrink-0 border-t border-border bg-card p-2.5">
        <div className="mb-1.5 h-4 text-xs text-muted-foreground" aria-live="polite">
          {typingLabel ? `${typingLabel}...` : ""}
        </div>
        <Input.TextArea
          aria-label={ee("ticketWorkbench.text157")}
          value={composerText}
          onChange={(event) => onComposerChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter" && !event.shiftKey) {
              event.preventDefault()
              handleSendRequest()
            }
          }}
          placeholder={
            !item.conversationId
              ? ee("ticketWorkbench.text153")
              : item.isDone
                ? ee("ticketWorkbench.text158")
	                : replyDisabled
	                  ? replyDisabledReason
                : ee("ticketWorkbench.text159")
          }
          rows={2}
          className="min-h-14 resize-none border-border shadow-none"
          disabled={!item.conversationId || item.isDone || replyDisabled}
        />
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <input
            ref={imageInputRef}
            type="file"
            accept="image/*"
            className="hidden"
            onChange={(event) => {
              const file = event.target.files?.[0]
              event.target.value = ""
              if (file) void handleSendImageRequest(file)
            }}
          />
          <input
            ref={attachmentInputRef}
            type="file"
            className="hidden"
            onChange={(event) => {
              const file = event.target.files?.[0]
              event.target.value = ""
              if (file) void handleSendAttachmentRequest(file)
            }}
          />
          <RailopsButton
            size="small"
            className="gap-2"
            onClick={() => imageInputRef.current?.click()}
            disabled={!item.conversationId || item.isDone || replyDisabled || uploadingAsset}
          >
            <ImageIcon className="size-4" />{ee("ticketWorkbench.text160")}</RailopsButton>
          <RailopsButton
            size="small"
            className="gap-2"
            onClick={() => attachmentInputRef.current?.click()}
            disabled={!item.conversationId || item.isDone || replyDisabled || uploadingAsset}
          >
            <PaperclipIcon className="size-4" />{ee("ticketWorkbench.text161")}</RailopsButton>
          <VoiceRecorderButton
            disabled={!item.conversationId || item.isDone || replyDisabled || uploadingAsset}
            className="border border-border"
            onRecorded={handleSendVoiceRequest}
            onError={(message) => toast.error(message)}
          />
          <span className="ml-auto hidden text-xs text-muted-foreground sm:inline">{ee("ticketWorkbench.text162")}</span>
          <RailopsButton size="small" className="gap-2" onClick={handleSendRequest} disabled={sending || !item.conversationId || item.isDone || replyDisabled}>
            {sending ? <Loader2Icon className="size-4 animate-spin" /> : <SendIcon className="size-4" />}{ee("ticketWorkbench.text163")}</RailopsButton>
        </div>
      </footer>
    </div>
  )
}

function MessageBubble({
  message,
  senderProfiles,
  customerProfile,
  customerLastSeenAt,
  translation,
  translating,
  onTranslate,
}: {
  message: WorkbenchMessage
  senderProfiles: Record<number, AdminAgentProfile>
  customerProfile: CustomerCardProfile | null
  customerLastSeenAt: string
  translation?: ConversationMessageTranslationDTO
  translating: boolean
  onTranslate: (message: WorkbenchMessage) => Promise<void>
}) {
  if (message.serviceEvent) {
    return <ConversationServiceEvent audience="enterprise" event={message.serviceEvent} time={message.time} />
  }
  if (message.tone === "system") {
    return (
      <div className="flex justify-center py-1">
        <div className="max-w-[88%] rounded-md border border-border bg-muted px-3 py-2 text-center text-xs leading-5 text-foreground">
          <ImMessageHTML html={message.html} />
          <time className="mt-1 block text-rhd-2xs text-muted-foreground">{message.time}</time>
        </div>
      </div>
    )
  }
  const toneMeta = getMessageToneMeta(message.tone)
  const right = message.side === "right"
  const roleDetail = message.role && message.role !== toneMeta.label ? message.role : ""
  const person = buildEnterpriseMessagePersonCard(message, senderProfiles, customerProfile, customerLastSeenAt)
  return (
    <div
      className={cn("flex gap-2", right && "justify-end")}
      data-message-tone={message.tone ?? "engineer"}
      data-sender-type={message.senderType}
    >
      {!right ? <MessagePersonAvatar person={person} label={toneMeta.label} className={toneMeta.avatarClassName} /> : null}
      <div className={cn("max-w-[84%] space-y-1 2xl:max-w-[78%]", right && "text-right")}>
        <div className={cn("flex min-w-0 items-center gap-1.5", right && "justify-end")}>
          <span className={cn("inline-flex h-5 shrink-0 items-center rounded-full border px-2 text-rhd-xs font-medium", toneMeta.badgeClassName)}>
            {toneMeta.label}
          </span>
          {roleDetail ? (
            <span className="min-w-0 max-w-40 truncate text-xs text-muted-foreground" title={roleDetail}>
              {roleDetail}
            </span>
          ) : null}
        </div>
        <div
          className={cn(
            "rounded-md border px-3 py-2 text-left text-sm leading-6 shadow-sm",
            toneMeta.bubbleClassName,
          )}
        >
          <ImMessageHTML html={message.html} />
        </div>
        {translation ? (
          <div className="rounded-md border border-primary/20 bg-primary/10 px-3 py-2 text-left text-xs leading-5 text-foreground">
            <div className="mb-1 text-rhd-2xs font-semibold uppercase text-primary">
              {translationLanguageLabel(translation.target_language)}{ee("ticketWorkbench.text164")}</div>
            {translation.translated_text}
          </div>
        ) : null}
        <div className={cn("flex items-center gap-1", right && "justify-end")}>
          <time className="text-rhd-xs text-muted-foreground">{message.time}</time>
          {message.id ? (
            <button
              type="button"
              className="flex size-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-50"
              onClick={() => void onTranslate(message)}
              disabled={translating}
              aria-label={ee("ticketWorkbench.text165")}
              title={ee("ticketWorkbench.text165")}
            >
              {translating ? (
                <Loader2Icon className="size-3.5 animate-spin" />
              ) : (
                <LanguagesIcon className="size-3.5" />
              )}
            </button>
          ) : null}
        </div>
      </div>
      {right ? <MessagePersonAvatar person={person} label={toneMeta.label} className={toneMeta.avatarClassName} /> : null}
    </div>
  )
}

function getMessageToneMeta(tone: WorkbenchMessage["tone"]) {
  switch (tone) {
    case "customer":
      return {
        label: ee("ticketWorkbench.text166"),
        avatarClassName: "bg-muted text-foreground",
        badgeClassName: "border-border bg-muted text-muted-foreground",
        bubbleClassName: "border-border bg-card text-foreground",
      }
    case "ai":
      return {
        label: ee("ticketWorkbench.text009"),
        avatarClassName: "bg-primary/10 text-primary",
        badgeClassName: "border-primary/20 bg-primary/10 text-primary",
        bubbleClassName: "border-primary/20 bg-primary/10 text-foreground",
      }
    case "partner":
      return {
        label: ee("ticketWorkbench.text167"),
        avatarClassName: "bg-amber-50 text-foreground",
        badgeClassName: "border-amber-200 bg-amber-50 text-foreground",
        bubbleClassName: "border-amber-200 bg-amber-50 text-foreground",
      }
    default:
      return {
        label: ee("ticketWorkbench.text168"),
        avatarClassName: "bg-primary text-primary-foreground",
        badgeClassName: "border-primary/20 bg-primary/10 text-primary",
        bubbleClassName: "border-primary bg-primary text-primary-foreground",
      }
  }
}

function MessagePersonAvatar({
  person,
  label,
  className,
}: {
  person: PersonCardData
  label: string
  className?: string
}) {
  const initials = label === ee("ticketWorkbench.text009") ? ee("ticketWorkbench.text169") : label.slice(0, 1)
  return (
    <PersonCard
      person={person}
      side="right"
      trigger={
        <Avatar className={cn("size-8 shrink-0 text-xs font-semibold", className)} title={label}>
          {person.avatar ? <AvatarImage src={person.avatar} alt={person.name} /> : null}
          <AvatarFallback className={cn("text-xs font-semibold", className)}>
            {initials}
          </AvatarFallback>
        </Avatar>
      }
    />
  )
}

function buildEnterpriseMessagePersonCard(
  message: WorkbenchMessage,
  senderProfiles: Record<number, AdminAgentProfile>,
  customerProfile: CustomerCardProfile | null,
  customerLastSeenAt: string,
): PersonCardData {
  const senderName = message.senderName?.trim() || message.role
  const senderType = message.senderType.toLowerCase()
  if (senderType.includes("customer") || senderType.includes("user")) {
    const onlineAt = customerLastSeenAt || customerProfile?.lastActiveAt
    return {
      name: customerProfile?.name || senderName || ee("ticketWorkbench.text166"),
      role: ee("ticketWorkbench.text166"),
      avatar: message.senderAvatar,
      onlineText: onlineAt ? formatRelativeOnlineText(onlineAt) : undefined,
      company: customerProfile?.company,
      phone: customerProfile?.phone,
      email: customerProfile?.email || extractEmailFromText(senderName),
      note: ee("ticketWorkbench.text171"),
    }
  }
  if (senderType.includes("partner")) {
    return {
      name: senderName || ee("ticketWorkbench.text167"),
      role: ee("ticketWorkbench.text167"),
      avatar: message.senderAvatar,
      note: ee("ticketWorkbench.text172"),
    }
  }
  if (senderType.includes("ai") || senderType.includes("bot")) {
    return {
      name: senderName || ee("ticketWorkbench.text009"),
      role: ee("ticketWorkbench.text173"),
      avatar: message.senderAvatar,
      note: ee("ticketWorkbench.text174"),
    }
  }

  const profile = message.senderId ? senderProfiles[message.senderId] : null
  return {
    name: senderName || ee("ticketWorkbench.text168"),
    role: ee("ticketWorkbench.text175"),
    avatar: message.senderAvatar || profile?.avatar,
    onlineText: profile?.lastOnlineAt
      ? formatRelativeOnlineText(profile.lastOnlineAt)
      : (message.senderId ? ee("ticketWorkbench.text176") : undefined),
    company: profile?.teamName ? ee("ticketWorkbench.text177") : undefined,
    department: profile?.teamName,
    position: profile?.displayName || profile?.nickname || profile?.username,
    note: ee("ticketWorkbench.text178"),
  }
}

function WorkbenchContextPane({
  item,
  conversation,
  canAssignTicket,
  tenantAIEnabled,
  hasDeviceConcept,
  acceptingTicket,
  onAcceptTicket,
  onTakeoverTicket,
  onTicketUpdated,
}: {
  item: WorkbenchQueueItem | null
  conversation: AgentConversation | null
  canAssignTicket: boolean
  tenantAIEnabled: boolean
  hasDeviceConcept: boolean
  acceptingTicket: boolean
  onAcceptTicket: (ticketId?: number, conversationId?: number) => Promise<void>
  onTakeoverTicket: (ticketId?: number, conversationId?: number) => Promise<void>
  onTicketUpdated: () => Promise<void>
}) {
  const [linkedTickets, setLinkedTickets] = useState<TicketListItem[]>([])
  const [contextTicketId, setContextTicketId] = useState<number | null>(null)
  const [aggregate, setAggregate] = useState<TicketAggregateDTO | null>(null)
  const [loadingTickets, setLoadingTickets] = useState(false)
  const [loadingAggregate, setLoadingAggregate] = useState(false)
  const [creating, setCreating] = useState(false)
  const [recordOpen, setRecordOpen] = useState(false)
  const [recordText, setRecordText] = useState("")
  const [recordVisibleToCustomer, setRecordVisibleToCustomer] = useState(false)
  const [savingRecord, setSavingRecord] = useState(false)
  const [repairOpen, setRepairOpen] = useState(false)
  const [repairFaultCode, setRepairFaultCode] = useState("")
  const [repairRootCause, setRepairRootCause] = useState("")
  const [repairSolution, setRepairSolution] = useState("")
  const [repairTestResult, setRepairTestResult] = useState("passed")
  const [savingRepair, setSavingRepair] = useState(false)
  const [cancelOpen, setCancelOpen] = useState(false)
  const [cancelReason, setCancelReason] = useState("")
  const [cancellingTicket, setCancellingTicket] = useState(false)
  const [creatingMeeting, setCreatingMeeting] = useState(false)
  const [endingMeeting, setEndingMeeting] = useState(false)
  const [meetingChoiceOpen, setMeetingChoiceOpen] = useState(false)
  const [meetingCreateMode, setMeetingCreateMode] = useState<MeetingCreateMode>("now")
  const [meetingScheduledAt, setMeetingScheduledAt] = useState(defaultMeetingScheduleLocalValue)
  const [knowledgeCandidates, setKnowledgeCandidates] = useState<TicketKnowledgeCandidateDTO[]>([])
  const [loadingKnowledgeCandidates, setLoadingKnowledgeCandidates] = useState(false)
  const [creatingKnowledgeCandidate, setCreatingKnowledgeCandidate] = useState(false)
  const [supplierCollaborations, setSupplierCollaborations] = useState<TicketSupplierCollaboration[]>([])
  const [supplierCollaborationDetails, setSupplierCollaborationDetails] = useState<Record<number, TicketSupplierCollaborationDetail>>({})
  const [productModules, setProductModules] = useState<ProductModule[]>([])
  const [selectedProductModuleId, setSelectedProductModuleId] = useState(0)
  const [partnerCompanies, setPartnerCompanies] = useState<TicketSupplierOption[]>([])
  const [selectedPartnerCompanyId, setSelectedPartnerCompanyId] = useState(0)
  const [supplierReason, setSupplierReason] = useState("")
  const [supplierAuthorizationEndsAt, setSupplierAuthorizationEndsAt] = useState(defaultSupplierAuthorizationLocalValue)
  const [supplierMessages, setSupplierMessages] = useState<Record<number, string>>({})
  const [supplierResolutions, setSupplierResolutions] = useState<Record<number, string>>({})
  const [loadingSupplierContext, setLoadingSupplierContext] = useState(false)
  const [savingSupplierContext, setSavingSupplierContext] = useState(false)
  const [contextTab, setContextTab] = useState<ContextTab>("summary")
  const [draft, setDraft] = useState(buildTicketDraft(item))

  useEffect(() => {
    setDraft(buildTicketDraft(item))
  }, [item])

  useEffect(() => {
    if (!tenantAIEnabled && contextTab === "knowledge") {
      setContextTab("summary")
    }
  }, [contextTab, tenantAIEnabled])

  useEffect(() => {
    let cancelled = false
    async function loadContextTickets() {
      if (!item) {
        setLinkedTickets([])
        setContextTicketId(null)
        return
      }
      if (item.ticketId) {
        setLinkedTickets(item.ticket ? [item.ticket] : [])
        setContextTicketId(item.ticketId)
        return
      }
      if (!conversation) {
        setLinkedTickets([])
        setContextTicketId(null)
        return
      }
      setLoadingTickets(true)
      try {
        const res = await fetchTickets({
          page: 1,
          page_size: 4,
          conversation_id: conversation.id,
          sort: "-updated_at",
        })
        if (cancelled) {
          return
        }
        const nextTickets = res.success && res.data ? res.data.items : []
        setLinkedTickets(nextTickets)
        setContextTicketId(nextTickets[0]?.id ?? null)
        if (!res.success) {
          toast.error(res.error?.message || ee("ticketWorkbench.text179"))
        }
      } catch (error) {
        if (!cancelled) {
          setLinkedTickets([])
          setContextTicketId(null)
          toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text179"))
        }
      } finally {
        if (!cancelled) {
          setLoadingTickets(false)
        }
      }
    }
    void loadContextTickets()
    return () => {
      cancelled = true
    }
  }, [conversation, item])

  useEffect(() => {
    let cancelled = false
    async function loadAggregate() {
      if (!contextTicketId) {
        setAggregate(null)
        return
      }
      setLoadingAggregate(true)
      try {
        const res = await fetchTicketAggregate(contextTicketId)
        if (cancelled) {
          return
        }
        if (res.success && res.data) {
          setAggregate(res.data)
        } else {
          setAggregate(null)
          toast.error(res.error?.message || ee("ticketWorkbench.text180"))
        }
      } catch (error) {
        if (!cancelled) {
          setAggregate(null)
          toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text180"))
        }
      } finally {
        if (!cancelled) {
          setLoadingAggregate(false)
        }
      }
    }
    void loadAggregate()
    return () => {
      cancelled = true
    }
  }, [contextTicketId, conversation?.lastMessageAt, item?.updatedAt])

  useEffect(() => {
    if (!contextTicketId || isTerminalTicketStatus(aggregate?.ticket.status ?? "")) {
      return
    }
    let cancelled = false
    let refreshInFlight = false
    const refreshAggregate = async () => {
      if (cancelled || refreshInFlight || document.visibilityState !== "visible") return
      refreshInFlight = true
      try {
        const res = await fetchTicketAggregate(contextTicketId)
        if (!cancelled && res.success && res.data) {
          setAggregate(res.data)
        }
      } finally {
        refreshInFlight = false
      }
    }
    const timer = window.setInterval(() => void refreshAggregate(), 4_000)
    window.addEventListener("focus", refreshAggregate)
    return () => {
      cancelled = true
      window.clearInterval(timer)
      window.removeEventListener("focus", refreshAggregate)
    }
  }, [aggregate?.ticket.status, contextTicketId])

  useEffect(() => {
    let cancelled = false
    const productId = aggregate?.ticket.product_id ?? 0
    const ticketProductModuleId = aggregate?.ticket.product_module_id ?? 0
    const supportsProductlessSupplier = !hasDeviceConcept && productId <= 0
    if (!contextTicketId || (!productId && !supportsProductlessSupplier)) {
      setSupplierCollaborations([])
      setSupplierCollaborationDetails({})
      setProductModules([])
      setSelectedProductModuleId(0)
      setPartnerCompanies([])
      setSelectedPartnerCompanyId(0)
      return
    }
    setLoadingSupplierContext(true)
    void (async () => {
      try {
        const collaborationRes = await fetchTicketSupplierCollaborations(contextTicketId)
        if (cancelled) return
        const collaborations = collaborationRes.success && collaborationRes.data
          ? collaborationRes.data
          : null
        if (collaborations) {
          setSupplierCollaborations(collaborations)
          const detailResults = await Promise.all(
            collaborations.map((collaboration) =>
              fetchTicketSupplierCollaborationDetail(contextTicketId, collaboration.id),
            ),
          )
          if (cancelled) return
          setSupplierCollaborationDetails(
            Object.fromEntries(
              detailResults
                .filter((result) => result.success && result.data)
                .map((result) => [result.data!.collaboration.id, result.data!]),
            ),
          )
        } else {
          setSupplierCollaborations([])
          setSupplierCollaborationDetails({})
        }
        if (!collaborationRes.success) {
          toast.error(collaborationRes.error?.message || ee("ticketWorkbench.text181"))
        }

        if (productId > 0) {
          const moduleRes = await getProductModules(productId)
          if (cancelled) return
          const modules = moduleRes.success && moduleRes.data
            ? moduleRes.data.filter((module) => module.status !== "disabled")
            : []
          setProductModules(modules)
          setSelectedProductModuleId((current) => {
            const preferred = current || ticketProductModuleId
            return modules.some((module) => module.id === preferred)
              ? preferred
              : modules[0]?.id || 0
          })
          setPartnerCompanies([])
          setSelectedPartnerCompanyId(0)
          if (!moduleRes.success) {
            toast.error(moduleRes.error?.message || ee("ticketWorkbench.text182"))
          }
        } else {
          const partnerRes = await fetchTicketSupplierOptions(contextTicketId)
          if (cancelled) return
          const partners = partnerRes.success && partnerRes.data ? partnerRes.data : []
          setPartnerCompanies(partners)
          setSelectedPartnerCompanyId((current) =>
            partners.some((partner) => partner.partner_company_id === current)
              ? current
              : partners[0]?.partner_company_id || 0,
          )
          setProductModules([])
          setSelectedProductModuleId(0)
          if (!partnerRes.success) {
            toast.error(partnerRes.error?.message || ee("ticketWorkbench.text327"))
          }
        }
      } catch (error) {
        if (!cancelled) {
          toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text181"))
        }
      } finally {
        if (!cancelled) setLoadingSupplierContext(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [
    aggregate?.ticket.product_id,
    aggregate?.ticket.product_module_id,
    aggregate?.ticket.updated_at,
    contextTicketId,
    hasDeviceConcept,
  ])

  useEffect(() => {
    let cancelled = false
    async function loadKnowledgeCandidates() {
      if (!tenantAIEnabled || !contextTicketId) {
        setKnowledgeCandidates([])
        setLoadingKnowledgeCandidates(false)
        return
      }
      setLoadingKnowledgeCandidates(true)
      try {
        const res = await fetchTicketKnowledgeCandidates(contextTicketId)
        if (cancelled) {
          return
        }
        if (res.success && res.data) {
          setKnowledgeCandidates(res.data)
        } else {
          setKnowledgeCandidates([])
          toast.error(res.error?.message || ee("ticketWorkbench.text183"))
        }
      } catch (error) {
        if (!cancelled) {
          setKnowledgeCandidates([])
          toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text183"))
        }
      } finally {
        if (!cancelled) {
          setLoadingKnowledgeCandidates(false)
        }
      }
    }
    void loadKnowledgeCandidates()
    return () => {
      cancelled = true
    }
  }, [aggregate?.ticket.updated_at, contextTicketId, tenantAIEnabled])

  const detail = aggregate
    ? buildAggregateDetail(aggregate, hasDeviceConcept)
    : buildLightDetail(item, conversation, hasDeviceConcept)
  const contextTicket = aggregate?.ticket ?? item?.ticket ?? linkedTickets[0] ?? null
  const contextTicketListActions = contextTicket && "actions" in contextTicket ? contextTicket.actions : undefined
  const contextActions = aggregate?.actions ?? contextTicketListActions
  const contextAssigneeId =
    aggregate?.assignment.assignee_id ??
    (contextTicket && "assignee_id" in contextTicket ? contextTicket.assignee_id : 0)
  const contextTicketClosed = contextTicket ? isTerminalTicketStatus(contextTicket.status) : false
  const contextTicketCancelled = contextTicket?.status === "cancelled"
  const contextCanTakeOverTicket = contextActions?.can_takeover === true
  const contextCanAcceptTicket = Boolean(
    contextActions?.can_accept === true ||
      contextCanTakeOverTicket ||
      canShowManualAcceptFallback(contextTicket?.status, contextAssigneeId),
  )
  const canCreateTicket = item?.kind === "conversation" && !contextTicketId
  const currentMeeting = aggregate?.meeting
  const canJoinCurrentMeeting = Boolean(currentMeeting?.meeting_id && canJoinTicketMeeting(currentMeeting.status))
  const hasFinishedCurrentMeeting = Boolean(currentMeeting?.meeting_id && !canJoinCurrentMeeting)
  const hasUnfinishedCurrentMeeting = canJoinCurrentMeeting
  const canEndCurrentMeeting = Boolean(
    currentMeeting?.meeting_id &&
      hasUnfinishedCurrentMeeting &&
      aggregate?.actions.can_end_meeting === true,
  )

  const handleEndCurrentMeeting = async (options: { continueRepair?: boolean; silent?: boolean } = {}) => {
    if (!currentMeeting?.meeting_id || !hasUnfinishedCurrentMeeting || endingMeeting) {
      return !hasUnfinishedCurrentMeeting
    }
    if (!canEndCurrentMeeting) {
      toast.error(ee("ticketWorkbench.text318"))
      return false
    }
    const externalNames = currentMeeting.external_participant_names ?? []
    const externalCount = currentMeeting.external_participant_count ?? 0
    const confirmMessage = externalCount > 0
      ? ee("ticketWorkbench.text316", {
          value0: externalNames.length > 0 ? externalNames.join("、") : `${externalCount}${ee("ticketWorkbench.text263")}`,
        })
      : options.continueRepair
        ? ee("ticketWorkbench.text317")
        : ee("ticketWorkbench.text320")
    if (!window.confirm(confirmMessage)) {
      return false
    }
    setEndingMeeting(true)
    try {
      const res = await endMeeting(currentMeeting.meeting_id)
      if (!res.success) {
        toast.error(res.error?.message || ee("ticketWorkbench.text318"))
        return false
      }
      const aggregateRes = await fetchTicketAggregate(contextTicket?.id ?? contextTicketId ?? 0)
      if (aggregateRes.success && aggregateRes.data) {
        setAggregate(aggregateRes.data)
      }
      await onTicketUpdated()
      if (!options.silent) {
        toast.success(ee("ticketWorkbench.text319"))
      }
      return true
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text318"))
      return false
    } finally {
      setEndingMeeting(false)
    }
  }

  const handleCreateTicket = async () => {
    if (!conversation || creating) {
      toast.info(ee("ticketWorkbench.text184"))
      return
    }
    const title = draft.title.trim()
    const description = draft.description.trim()
    if (!title || !description) {
      toast.error(ee("ticketWorkbench.text185"))
      return
    }
    setCreating(true)
    try {
      const res = await createTicket({
        source: "conversation",
        channel: "im",
        conversation_id: conversation.id,
        customer_id: conversation.customerId || undefined,
        current_assignee_id: canAssignTicket ? conversation.currentAssigneeId || undefined : undefined,
        title,
        description,
        priority: draft.priority,
      })
      if (!res.success || !res.data) {
        toast.error(res.error?.message || ee("ticketWorkbench.text186"))
        return
      }
      toast.success(ee("ticketWorkbench.text187"))
      setLinkedTickets([res.data])
      setContextTicketId(res.data.id)
    } finally {
      setCreating(false)
    }
  }

  const handleCreateProgress = async () => {
    if (!contextTicket || !recordText.trim() || savingRecord) {
      return
    }
    setSavingRecord(true)
    try {
      const res = await createTicketProgress(
        contextTicket.id,
        recordText.trim(),
        recordVisibleToCustomer,
      )
      if (!res.success || !res.data) {
        toast.error(res.error?.message || ee("ticketWorkbench.text188"))
        return
      }
      setAggregate(res.data)
      setRecordText("")
      setRecordVisibleToCustomer(false)
      setRecordOpen(false)
      toast.success(ee("ticketWorkbench.text189"))
    } finally {
      setSavingRecord(false)
    }
  }

	  const handleCreateRepair = async () => {
	    const resolved = repairTestResult === "passed"
	    if (
	      !contextTicket ||
      !repairFaultCode.trim() ||
      !repairSolution.trim() ||
      (resolved && !repairRootCause.trim()) ||
	      savingRepair
	    ) {
	      return
	    }
	    setSavingRepair(true)
	    try {
	      if (hasUnfinishedCurrentMeeting) {
	        const ended = await handleEndCurrentMeeting({ continueRepair: true, silent: true })
	        if (!ended) {
	          return
	        }
	      }
	      const res = await createTicketRepair(contextTicket.id, {
	        fault_code: repairFaultCode.trim(),
        conclusion: repairSolution.trim(),
        solution: repairSolution.trim(),
        root_cause: repairRootCause.trim(),
        repair_method: ee("ticketWorkbench.text190"),
        service_method: "remote",
        test_result: repairTestResult,
        remote_resolved: resolved,
        visible_to_customer: true,
        mark_resolved: resolved,
      })
      if (!res.success || !res.data) {
        toast.error(res.error?.message || ee("ticketWorkbench.text191"))
        return
      }
      setAggregate(res.data)
      setRepairOpen(false)
      setRepairFaultCode("")
      setRepairRootCause("")
      setRepairSolution("")
      setRepairTestResult("passed")
		await onTicketUpdated()
      toast.success(resolved ? ee("ticketWorkbench.text192") : ee("ticketWorkbench.text193"))
    } finally {
      setSavingRepair(false)
    }
  }

  const handleCancelTicket = async () => {
    const reason = cancelReason.trim()
    if (!contextTicket || !reason || cancellingTicket) {
      return
    }
    setCancellingTicket(true)
    try {
      const res = await cancelTicket(contextTicket.id, reason)
      if (!res.success) {
        toast.error(res.error?.message || ee("ticketWorkbench.text194"))
        return
      }
      const aggregateRes = await fetchTicketAggregate(contextTicket.id)
      if (aggregateRes.success && aggregateRes.data) {
        setAggregate(aggregateRes.data)
      }
      setCancelReason("")
      setCancelOpen(false)
      await onTicketUpdated()
      toast.success(ee("ticketWorkbench.text195"))
    } finally {
      setCancellingTicket(false)
    }
  }

  const handleCreateMeeting = async (options: MeetingCreateOptions = {}) => {
    if (!contextTicket || creatingMeeting) {
      return false
    }
    const shouldOpenRoom = options.openRoom ?? true
    const popup = shouldOpenRoom ? window.open("about:blank", "_blank") : null
    setCreatingMeeting(true)
    try {
			const meetingId = aggregate?.meeting?.meeting_id
			const meetingStatus = aggregate?.meeting?.status
			const res = shouldOpenRoom && meetingId && canJoinTicketMeeting(meetingStatus)
				? await fetchMeetingJoinConfig(meetingId)
				: await createMeeting({
					ticket_id: contextTicket.id,
					title: contextTicket.title,
					scheduled_at: options.scheduledAt,
				})
      if (!res.success || !res.data) {
        popup?.close()
        toast.error(res.error?.message || ee("ticketWorkbench.text196"))
        return false
      }
      const aggregateRes = await fetchTicketAggregate(contextTicket.id)
      if (aggregateRes.success && aggregateRes.data) {
        setAggregate(aggregateRes.data)
      }
      if (!shouldOpenRoom) {
        setMeetingChoiceOpen(false)
        toast.success(ee("ticketWorkbench.text040"))
        return true
      }
      const roomName = res.data.roomName || res.data.room_name
      const base = res.data.jitsiUrl || res.data.jitsi_url || res.data.domain
      if (!popup || !roomName || !base) {
        popup?.close()
        toast.error(ee("ticketWorkbench.text197"))
        return false
      }
      popup.opener = null
      const nextMeetingId = res.data.meetingId || res.data.meeting_id || meetingId
      popup.location.href = nextMeetingId
        ? buildEnterpriseMeetingRoomPath(nextMeetingId, contextTicket.id)
        : `${base.replace(/\/$/, "")}/${roomName}?jwt=${encodeURIComponent(res.data.jwt)}`
      setMeetingChoiceOpen(false)
      return true
    } finally {
      setCreatingMeeting(false)
    }
  }

  const openMeetingChoice = () => {
    if (canJoinCurrentMeeting) {
      void handleCreateMeeting()
      return
    }
    setMeetingCreateMode("now")
    setMeetingScheduledAt(defaultMeetingScheduleLocalValue())
    setMeetingChoiceOpen(true)
  }

  const submitMeetingChoice = async () => {
    if (meetingCreateMode === "now") {
      await handleCreateMeeting({ openRoom: true })
      return
    }
    const scheduledAt = localDatetimeValueToISOString(meetingScheduledAt)
    if (!scheduledAt) {
      toast.error(ee("ticketWorkbench.text198"))
      return
    }
    if (new Date(scheduledAt).getTime() <= Date.now() + 60_000) {
      toast.error(ee("ticketWorkbench.text199"))
      return
    }
    await handleCreateMeeting({ scheduledAt, openRoom: false })
  }

  const handleCreateKnowledgeCandidate = async () => {
    if (!tenantAIEnabled || !contextTicket || creatingKnowledgeCandidate) {
      return
    }
    setCreatingKnowledgeCandidate(true)
    try {
      const res = await createTicketKnowledgeCandidate(contextTicket.id)
      if (!res.success || !res.data) {
        toast.error(res.error?.message || ee("ticketWorkbench.text200"))
        return
      }
      setKnowledgeCandidates((current) => [
        res.data,
        ...current.filter((candidate) => candidate.id !== res.data.id),
      ])
      toast.success(res.data.created ? ee("ticketWorkbench.text201") : ee("ticketWorkbench.text202"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ee("ticketWorkbench.text200"))
    } finally {
      setCreatingKnowledgeCandidate(false)
    }
  }

  const handleInviteSupplier = async () => {
    const usesProductModule = (aggregate?.ticket.product_id ?? 0) > 0
    const hasSupplierTarget = usesProductModule
      ? selectedProductModuleId > 0
      : selectedPartnerCompanyId > 0
    if (!contextTicket || !hasSupplierTarget || !supplierReason.trim() || savingSupplierContext) {
      return
    }
    const authorizationEndsAt = localDatetimeValueToISOString(supplierAuthorizationEndsAt)
    if (!authorizationEndsAt || new Date(authorizationEndsAt).getTime() <= Date.now()) {
      toast.error(ee("ticketWorkbench.text203"))
      return
    }
    setSavingSupplierContext(true)
    try {
      const res = await inviteTicketSupplier(contextTicket.id, {
        ...(usesProductModule
          ? { product_module_id: selectedProductModuleId }
          : { partner_company_id: selectedPartnerCompanyId }),
        reason: supplierReason.trim(),
        authorization_ends_at: authorizationEndsAt,
      })
      if (!res.success || !res.data) {
        toast.error(res.error?.message || ee("ticketWorkbench.text204"))
        return
      }
      setSupplierCollaborations((current) => [
        res.data,
        ...current.filter((item) => item.id !== res.data.id),
      ])
      setSupplierReason("")
      setSupplierAuthorizationEndsAt(defaultSupplierAuthorizationLocalValue())
      const aggregateRes = await fetchTicketAggregate(contextTicket.id)
      if (aggregateRes.success && aggregateRes.data) setAggregate(aggregateRes.data)
      toast.success(ee("ticketWorkbench.text205", {
        value0: res.data.partner_company_name || ee("ticketWorkbench.text313"),
      }))
    } finally {
      setSavingSupplierContext(false)
    }
  }

	const handleResolveSupplier = async (collaborationId: number) => {
		const resolution = supplierResolutions[collaborationId]?.trim() ?? ""
		if (!contextTicket || !resolution || savingSupplierContext) return
		setSavingSupplierContext(true)
		try {
			const res = await resolveTicketSupplierCollaboration(
				contextTicket.id,
				collaborationId,
				resolution,
			)
			if (!res.success || !res.data) {
				toast.error(res.error?.message || ee("ticketWorkbench.text206"))
				return
			}
			setSupplierCollaborations((current) =>
				current.map((item) => (item.id === res.data.id ? res.data : item)),
			)
			setSupplierResolutions((current) => ({ ...current, [collaborationId]: "" }))
			toast.success(ee("ticketWorkbench.text207"))
		} finally {
			setSavingSupplierContext(false)
		}
	}

	const handleSendSupplierMessage = async (collaborationId: number) => {
		const content = supplierMessages[collaborationId]?.trim() ?? ""
		if (!contextTicket || !content || savingSupplierContext) return
		setSavingSupplierContext(true)
		try {
			const res = await addTicketSupplierCollaborationProgress(
				contextTicket.id,
				collaborationId,
				content,
			)
			if (!res.success || !res.data) {
				toast.error(res.error?.message || ee("ticketWorkbench.text208"))
				return
			}
			setSupplierCollaborationDetails((current) => ({
				...current,
				[collaborationId]: res.data!,
			}))
			setSupplierMessages((current) => ({ ...current, [collaborationId]: "" }))
			toast.success(ee("ticketWorkbench.text209"))
		} finally {
			setSavingSupplierContext(false)
		}
	}

  const contextTabs: RailopsTabItem[] = [
    { value: "summary", label: ee("ticketWorkbench.text210") },
    { value: "supplier", label: ee("ticketWorkbench.text227") },
  ]
  if (tenantAIEnabled) {
    contextTabs.push({ value: "knowledge", label: ee("ticketWorkbench.text246") })
  }
  contextTabs.push(
    { value: "progress", label: ee("ticketWorkbench.text258") },
    { value: "timeline", label: ee("ticketWorkbench.text259") },
  )
  const visibleDetailTags = hasDeviceConcept
    ? detail.tags
    : [item?.source, item?.sla].filter((value): value is string => Boolean(value))

  return (
    <div className="flex h-full min-h-0 flex-col bg-card">
      <div className="shrink-0 border-b border-border bg-card px-2 py-2">
        <UnderlineTabs
          ariaLabel={ee("ticketWorkbench.text121")}
          items={contextTabs}
          value={contextTab}
          onChange={(value) => setContextTab(value as ContextTab)}
        />
      </div>
      <ScrollArea className="min-h-0 flex-1">
        <div className="min-h-full bg-card">
        {contextTab === "summary" ? (
          <>
        <ContextSection title={ee("ticketWorkbench.text210")}>
          <p className="text-xs leading-5 text-foreground">{hasDeviceConcept ? detail.serviceSummary : item?.title || detail.serviceSummary}</p>
          {contextTicket ? (
            <div className={cn("mt-3 grid gap-2 text-xs", hasDeviceConcept ? "grid-cols-2" : "grid-cols-1")}>
              <InfoTile label={ee("ticketWorkbench.text211")} value={fieldValue(contextTicket.ticket_no)} />
              {hasDeviceConcept ? (
                <InfoTile
                  label={ee("ticketWorkbench.text088")}
                  value={fieldValue(
                    item?.device ||
                      aggregate?.device_context.device_no ||
                      ("device_no" in contextTicket ? contextTicket.device_no : ""),
                  )}
                />
              ) : null}
            </div>
          ) : null}
        </ContextSection>

        <ContextSection
          title={ee("ticketWorkbench.text212")}
          action={
            <StatusTag tone="neutral" className={cn("h-5 px-1.5 text-rhd-xs", statusToneClassName(item?.statusTone ?? "neutral"))}>
              {item?.statusLabel ?? ee("ticketWorkbench.text048")}
            </StatusTag>
          }
        >
          <p className="text-xs leading-5 text-muted-foreground">{detail.conclusion}</p>
          {visibleDetailTags.length > 0 ? (
            <div className="mt-2 flex flex-wrap gap-1.5">
              {visibleDetailTags.map((tag) => (
                <span
                  key={tag}
                  className="rounded-full border border-border bg-muted px-2 py-1 text-rhd-xs font-medium text-muted-foreground"
                >
                  {tag}
                </span>
              ))}
            </div>
          ) : null}
        </ContextSection>

        {contextTicket ? (
          <ContextSection title={ee("ticketWorkbench.text213")}>
            <div className="space-y-2">
              <ContextRow
                label={ee(hasDeviceConcept ? "ticketWorkbench.text214" : "ticketWorkbench.text323")}
                value={aggregate?.assignment.team_name || ("team_name" in contextTicket ? contextTicket.team_name : "") || ee("ticketWorkbench.text215")}
              />
              <ContextRow
                label={ee("ticketWorkbench.text216")}
                value={aggregate?.assignment.assignee_name || ("assignee_name" in contextTicket ? contextTicket.assignee_name || "" : "") || ee(hasDeviceConcept ? "ticketWorkbench.text217" : "ticketWorkbench.text324")}
                badge={aggregate?.assignment.assignee_id || ("assignee_id" in contextTicket && contextTicket.assignee_id) ? ee("ticketWorkbench.text218") : ee("ticketWorkbench.text219")}
                tone={aggregate?.assignment.assignee_id || ("assignee_id" in contextTicket && contextTicket.assignee_id) ? "good" : "warn"}
              />
              {aggregate?.assignment.dispatch_deferred_until || aggregate?.assignment.last_dispatch_failure_reason ? (
                <ContextRow
                  label={ee("ticketWorkbench.text220")}
                  value={
                    aggregate?.assignment.dispatch_deferred_until
                      ? formatDateTime(aggregate?.assignment.dispatch_deferred_until || "")
                      : ee("ticketWorkbench.text070")
                  }
                  badge={dispatchFailureReasonLabel(aggregate?.assignment.last_dispatch_failure_reason) || ee("ticketWorkbench.text070")}
                  tone="warn"
                />
              ) : null}
              {!aggregate?.assignment.assignee_id && !("assignee_id" in contextTicket && contextTicket.assignee_id) ? (
                <p className="text-rhd-xs leading-5 text-muted-foreground">{ee("ticketWorkbench.text221")}</p>
              ) : null}
            </div>
          </ContextSection>
        ) : null}

        <ContextSection title={ee("ticketWorkbench.text161")}>
          {loadingAggregate || loadingTickets ? (
            <LoadingLine text={ee("ticketWorkbench.text222")} />
          ) : detail.attachments.length > 0 ? (
            <div className="space-y-2">
              {detail.attachments.map((attachment) => (
                <ContextRow
                  key={`${attachment.label}-${attachment.desc}`}
                  label={attachment.label}
                  value={attachment.desc}
                  badge={attachment.status}
                  tone={attachment.tone}
                />
              ))}
            </div>
          ) : (
            <EmptyLine text={ee("ticketWorkbench.text223")} />
          )}
        </ContextSection>

        <ContextSection title={ee("ticketWorkbench.text224")}>
          {detail.parts.length > 0 ? (
            <div className="space-y-2">
              {detail.parts.map((part) => (
                <ContextRow
                  key={`${part.name}-${part.desc}`}
                  label={part.name}
                  value={part.quantity && part.quantity !== "0" ? `${part.desc} · ${part.quantity}` : part.desc}
                />
              ))}
            </div>
          ) : (
            <EmptyLine text={ee("ticketWorkbench.text225")} />
          )}
        </ContextSection>

        <ContextSection title={ee("ticketWorkbench.text226")}>
          <div className="flex items-center gap-2 text-xs text-foreground">
            <StarIcon className="size-4 text-amber-500" />
            <span>{detail.rating}</span>
          </div>
          {detail.feedback ? (
            <p className="mt-1 text-xs leading-5 text-muted-foreground">{detail.feedback}</p>
          ) : null}
        </ContextSection>

        {canCreateTicket ? (
          <ContextSection title={ee("ticketWorkbench.text254")}>
            <div className="space-y-2">
              <input
                value={draft.title}
                onChange={(event) =>
                  setDraft((current) => ({ ...current, title: event.target.value }))
                }
                className="h-9 w-full rounded-md border border-border bg-background px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
                placeholder={ee("ticketWorkbench.text255")}
              />
              <SelectField
                style={{ marginBottom: 0 }}
                selectProps={{
                  value: draft.priority,
                  onChange: (value) =>
                    setDraft((current) => ({
                      ...current,
                      priority: (value as string) as TicketPriority,
                    })),
                  options: [
                    { value: "critical", label: ee("ticketWorkbench.text026") },
                    { value: "high", label: ee("ticketWorkbench.text027") },
                    { value: "medium", label: ee("ticketWorkbench.text028") },
                    { value: "low", label: ee("ticketWorkbench.text029") },
                  ],
                  style: { width: "100%" },
                }}
              />
              <Input.TextArea
                aria-label={ee("ticketWorkbench.text256")}
                value={draft.description}
                onChange={(event) =>
                  setDraft((current) => ({ ...current, description: event.target.value }))
                }
                placeholder={ee("ticketWorkbench.text256")}
                rows={3}
                className="resize-none border-border shadow-none"
              />
              <RailopsButton
                size="small"
                className="w-full gap-2"
                onClick={() => void handleCreateTicket()}
                disabled={creating}
              >
                {creating ? <Loader2Icon className="size-4 animate-spin" /> : <TicketIcon className="size-4" />}{ee("ticketWorkbench.text257")}</RailopsButton>
            </div>
          </ContextSection>
        ) : null}

          </>
        ) : null}

		{contextTab === "supplier" &&
		contextTicket &&
		((aggregate?.ticket.product_id ?? 0) > 0 || !hasDeviceConcept) ? (
					<ContextSection
					title={ee("ticketWorkbench.text227")}
				action={<HandshakeIcon className="size-4 text-muted-foreground" />}
			>
				{loadingSupplierContext ? (
					<LoadingLine text={ee("ticketWorkbench.text228")} />
				) : (
					<div className="space-y-3">
						{supplierCollaborations.length > 0 ? (
							<div className="space-y-2">
								{supplierCollaborations.map((collaboration) => {
									const active = collaboration.authorization_active
									const collaborationDetail = supplierCollaborationDetails[collaboration.id]
									const supplierMessage = supplierMessages[collaboration.id] ?? ""
									const supplierResolution = supplierResolutions[collaboration.id] ?? ""
									return (
										<div key={collaboration.id} className="rounded-md border border-border p-2.5">
											<div className="flex items-start justify-between gap-2">
												<strong className="text-xs text-foreground">{collaboration.partner_company_name}</strong>
												<StatusTag tone="neutral" className="h-5 px-1.5 text-rhd-2xs">
												{active
													? (collaboration.status === "processing" ? ee("ticketWorkbench.text011") : ee("ticketWorkbench.text229"))
													: collaboration.status === "expired" ? ee("ticketWorkbench.text230") : ee("ticketWorkbench.text012")}
												</StatusTag>
											</div>
											<p className="mt-1 text-rhd-xs leading-5 text-muted-foreground">
												{[
													collaboration.product_module_name,
													collaboration.partner_account_name || ee("ticketWorkbench.text231"),
												].filter(Boolean).join(" · ")}
											</p>
											<p className="mt-1 text-rhd-xs leading-5 text-muted-foreground">{ee("ticketWorkbench.text232")}{formatShortDateTime(collaboration.authorization_ends_at)}
											</p>
											<p className="mt-1 text-rhd-xs leading-5 text-foreground">{collaboration.resolution || collaboration.reason}</p>
											{collaborationDetail?.progresses.length ? (
												<div className="mt-2 space-y-1.5 border-l-2 border-border pl-2">
													{collaborationDetail.progresses.map((progress) => (
														<div key={progress.id} className="text-rhd-xs leading-5 text-foreground">
															<div className="flex items-center justify-between gap-2 text-rhd-2xs text-muted-foreground">
																<span>{progress.author_name || ee("ticketWorkbench.text233")}</span>
																<time>{formatShortDateTime(progress.created_at)}</time>
															</div>
															<p className="whitespace-pre-wrap">{progress.content}</p>
														</div>
													))}
												</div>
											) : null}
											{active ? (
												<div className="mt-2 space-y-2">
													<Input.TextArea
														aria-label={ee("ticketWorkbench.text234")}
														value={supplierMessage}
														onChange={(event) => setSupplierMessages((current) => ({
															...current,
															[collaboration.id]: event.target.value,
														}))}
														placeholder={ee("ticketWorkbench.text234")}
														rows={2}
														className="resize-none text-xs"
													/>
													<RailopsButton
														size="small"
														className="w-full"
														onClick={() => void handleSendSupplierMessage(collaboration.id)}
														disabled={!supplierMessage.trim() || savingSupplierContext}
													>{ee("ticketWorkbench.text235")}</RailopsButton>
													<Input.TextArea
														aria-label={ee("ticketWorkbench.text236")}
														value={supplierResolution}
														onChange={(event) => setSupplierResolutions((current) => ({
															...current,
															[collaboration.id]: event.target.value,
														}))}
														placeholder={ee("ticketWorkbench.text236")}
														rows={2}
														className="resize-none text-xs"
													/>
													<RailopsButton
														size="small"
														className="w-full"
														onClick={() => void handleResolveSupplier(collaboration.id)}
														disabled={!supplierResolution.trim() || savingSupplierContext}
													>{ee("ticketWorkbench.text237")}</RailopsButton>
												</div>
											) : null}
										</div>
									)
								})}
							</div>
						) : null}
							{contextTicketClosed ? (
								<div className="rounded-md border border-border bg-muted px-3 py-2 text-xs leading-5 text-muted-foreground">
									{supplierCollaborations.length > 0
										? ee("ticketWorkbench.text238")
										: ee("ticketWorkbench.text239")}
								</div>
							) : (
								<>
									{(aggregate?.ticket.product_id ?? 0) > 0 ? (
										<SelectField
											style={{ marginBottom: 0 }}
											selectProps={{
												"aria-label": ee("ticketWorkbench.text240"),
												placeholder: ee("ticketWorkbench.text240"),
												value: selectedProductModuleId === 0 ? undefined : String(selectedProductModuleId),
												onChange: (value) => setSelectedProductModuleId(Number(value)),
												options: productModules.map((module) => ({
													value: String(module.id),
													label: module.default_supplier_id
														? module.name
														: ee("ticketWorkbench.text241", { value0: module.name }),
												})),
												style: { width: "100%" },
											}}
										/>
									) : (
										<SelectField
											style={{ marginBottom: 0 }}
											selectProps={{
												"aria-label": ee("ticketWorkbench.text326"),
												placeholder: ee("ticketWorkbench.text326"),
												value: selectedPartnerCompanyId === 0 ? undefined : String(selectedPartnerCompanyId),
												onChange: (value) => setSelectedPartnerCompanyId(Number(value)),
												options: partnerCompanies.map((partner) => ({
													value: String(partner.partner_company_id),
													label: partner.name,
												})),
												style: { width: "100%" },
											}}
										/>
									)}
									{(aggregate?.ticket.product_id ?? 0) <= 0 && partnerCompanies.length === 0 ? (
										<p className="text-rhd-xs leading-5 text-muted-foreground">{ee("ticketWorkbench.text327")}</p>
									) : null}
									<Input.TextArea
										aria-label={ee("ticketWorkbench.text242")}
										value={supplierReason}
										onChange={(event) => setSupplierReason(event.target.value)}
										placeholder={ee("ticketWorkbench.text243")}
										rows={2}
										className="resize-none text-xs"
									/>
									<label className="block space-y-1 text-xs font-medium text-foreground">
										<span>{ee("ticketWorkbench.text244")}</span>
										<input
											type="datetime-local"
											value={supplierAuthorizationEndsAt}
												min={toMeetingDatetimeLocalValue(new Date(Date.now() + 60_000))}
												max={toMeetingDatetimeLocalValue(new Date(Date.now() + 90 * 24 * 60 * 60_000))}
											onChange={(event) => setSupplierAuthorizationEndsAt(event.target.value)}
											className="h-9 w-full rounded-md border border-border bg-background px-3 text-xs shadow-sm outline-none transition focus-visible:ring-2 focus-visible:ring-primary/20"
										/>
									</label>
									<RailopsButton
										size="small"
										className="w-full gap-2"
										onClick={() => void handleInviteSupplier()}
										disabled={
											((aggregate?.ticket.product_id ?? 0) > 0
												? !selectedProductModuleId
												: !selectedPartnerCompanyId) ||
											!supplierReason.trim() ||
											savingSupplierContext ||
											aggregate?.actions.can_escalate_supplier === false
										}
									>
										{savingSupplierContext ? <Loader2Icon className="size-4 animate-spin" /> : <HandshakeIcon className="size-4" />}
										{ee((aggregate?.ticket.product_id ?? 0) > 0 ? "ticketWorkbench.text245" : "ticketWorkbench.text328")}
									</RailopsButton>
								</>
							)}
						</div>
					)}
				</ContextSection>
		) : null}

				{tenantAIEnabled && contextTab === "knowledge" && contextTicket ? (
	          <ContextSection
            title={ee("ticketWorkbench.text246")}
            action={
              <RailopsButton
                variant="text"
                size="small"
                className="h-7 gap-1.5 px-2 text-xs"
                onClick={() => void handleCreateKnowledgeCandidate()}
                disabled={
                  creatingKnowledgeCandidate ||
                  knowledgeCandidates.length > 0 ||
                  aggregate?.actions.can_create_knowledge_candidate === false
                }
              >
                {creatingKnowledgeCandidate ? (
                  <Loader2Icon className="size-3.5 animate-spin" />
                ) : (
                  <BookOpenCheckIcon className="size-3.5" />
                )}
                {knowledgeCandidates.length > 0 ? ee("ticketWorkbench.text247") : ee("ticketWorkbench.text248")}
              </RailopsButton>
            }
          >
            {loadingKnowledgeCandidates ? (
              <LoadingLine text={ee("ticketWorkbench.text249")} />
            ) : knowledgeCandidates.length > 0 ? (
              <div className="space-y-2">
                {knowledgeCandidates.map((candidate) => (
                  <div key={candidate.id} className="rounded-md border border-border p-2.5">
                    <div className="flex items-start justify-between gap-2">
                      <strong className="line-clamp-2 text-xs font-semibold text-foreground">
                        {candidate.title}
                      </strong>
                      <StatusTag tone="neutral" className="h-5 shrink-0 px-1.5 text-rhd-2xs">
                        {getTicketKnowledgeCandidateStatusLabel(candidate.review_status, candidate.status === "linked")}
                      </StatusTag>
                    </div>
                    <p className="mt-1 line-clamp-3 text-rhd-xs leading-5 text-muted-foreground">
                      {candidate.suggestion || candidate.solution_summary || candidate.root_cause_summary}
                    </p>
                    <p className="mt-1 text-rhd-2xs font-medium text-muted-foreground">{ee("ticketWorkbench.text250")}{candidate.candidate_score}{ee("ticketWorkbench.text251")}{candidate.quality_score}{ee("ticketWorkbench.text252")}{candidate.value_score}
                    </p>
                  </div>
                ))}
              </div>
            ) : (
              <EmptyLine text={ee("ticketWorkbench.text253")} />
            )}
          </ContextSection>
        ) : null}

        {contextTab === "progress" ? (
        <ContextSection
          title={ee("ticketWorkbench.text258")}
          action={
            <StatusTag tone="neutral" className={cn("h-5 px-1.5 text-rhd-xs", statusToneClassName(item?.statusTone ?? "neutral"))}>
              {item?.sla ?? ee("ticketWorkbench.text048")}
            </StatusTag>
          }
        >
          <ProgressStageTimeline items={detail.flow} />
        </ContextSection>
        ) : null}

        {contextTab === "timeline" ? (
          <ContextSection title={ee("ticketWorkbench.text259")}>
        {detail.timeline.length > 0 ? (
            <RecordPipeline items={detail.timeline} />
        ) : (
          <EmptyLine text={ee("ticketWorkbench.text291")} />
        )}
          </ContextSection>
        ) : null}
        </div>
      </ScrollArea>

        {contextTicket ? (
          contextTicketClosed ? (
            <div className={cn(
              "m-3 rounded-lg border px-3 py-3 text-sm",
              contextTicketCancelled
                ? "border-border bg-muted text-foreground"
                : "border-primary/20 bg-primary/10 text-foreground",
            )}>
              <div className="flex items-center gap-2 font-medium">
                {contextTicketCancelled ? <CircleXIcon className="size-4" /> : <CheckCircle2Icon className="size-4" />}
                {contextTicketCancelled ? ee("ticketWorkbench.text195") : ee("ticketWorkbench.text260")}
              </div>
            </div>
          ) : (
            <div className="grid grid-cols-2 gap-2 p-3">
              {contextCanAcceptTicket ? (
	                <RailopsButton
	                  variant="primary"
	                  className="col-span-2 gap-2 shadow-[0_8px_18px_rgba(59,130,246,0.24)]"
	                  onClick={() => {
	                    const conversationId = contextTicket.conversation_id ?? item?.conversationId
	                    void (contextCanTakeOverTicket
	                      ? onTakeoverTicket(contextTicket.id, conversationId)
	                      : onAcceptTicket(contextTicket.id, conversationId))
	                  }}
	                  disabled={acceptingTicket}
	                >
                  {acceptingTicket ? (
                    <Loader2Icon className="size-4 animate-spin" />
                  ) : (
                    <HandshakeIcon className="size-4" />
                  )}
	                  {acceptingTicket
	                    ? contextCanTakeOverTicket ? ee("ticketWorkbench.text144") : ee("ticketWorkbench.text145")
	                    : contextCanTakeOverTicket ? contextAssigneeId > 0 ? ee("ticketWorkbench.text314") : ee("ticketWorkbench.text315") : contextTicket.status === "reopened" ? ee("ticketWorkbench.text147") : ee("ticketWorkbench.text148")}
	                </RailopsButton>
	              ) : null}
              <RailopsButton
                className="col-span-2 gap-2"
                onClick={() => {
                  setRepairFaultCode("category" in contextTicket ? contextTicket.category.trim() : "")
                  setRepairOpen(true)
                }}
                disabled={aggregate?.actions.can_save_repair === false}
              >
                <WrenchIcon className="size-4" />{ee("ticketWorkbench.text261")}</RailopsButton>
              {canJoinCurrentMeeting ? (
                <div
                  className="col-span-2 rounded-md border border-border bg-card p-3"
                  data-testid="enterprise-ticket-meeting-card"
                >
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex min-w-0 items-center gap-2 text-sm font-semibold text-foreground">
                      <span className="flex size-8 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary shadow-sm">
                        <VideoIcon className="size-4" />
                      </span>
                      <span className="min-w-0 truncate">{ticketMeetingStatusText(currentMeeting?.status)}</span>
                    </div>
                    <StatusTag tone="neutral" className="shrink-0 border-border bg-background text-foreground">
                      {ticketMeetingBadgeText(currentMeeting?.status)}
                    </StatusTag>
                  </div>
                  <div className="mt-3 grid grid-cols-2 gap-2 text-xs text-muted-foreground">
	                    <div className="rounded-md bg-background/80 px-2 py-1.5">
	                      <div className="text-rhd-xs text-muted-foreground">{ee("ticketWorkbench.text262")}</div>
	                      <div className="mt-0.5 font-medium">{currentMeeting?.active_participant_count ?? currentMeeting?.participant_count ?? 0}{ee("ticketWorkbench.text263")}</div>
	                    </div>
                    <div className="rounded-md bg-background/80 px-2 py-1.5">
                      <div className="text-rhd-xs text-muted-foreground">{ee("ticketWorkbench.text264")}</div>
                      <div className="mt-0.5 font-medium">
                        {currentMeeting?.status === "active" ? formatMeetingDuration(currentMeeting?.duration) : ee("ticketWorkbench.text046")}
                      </div>
                    </div>
                    <div className="col-span-2 rounded-md bg-background/80 px-2 py-1.5">
                      <div className="text-rhd-xs text-muted-foreground">
                        {currentMeeting?.status === "scheduled" ? ee("ticketWorkbench.text265") : ee("ticketWorkbench.text266")}
                      </div>
                      <div className="mt-0.5 truncate font-medium">
                        {formatShortDateTime(
                          currentMeeting?.status === "scheduled"
                            ? currentMeeting?.scheduled_at
                            : currentMeeting?.started_at,
                        )}
	                      </div>
	                    </div>
	                  </div>
	                  {(currentMeeting?.external_participant_count ?? 0) > 0 ? (
	                    <div className="mt-2 rounded-md border border-amber-200 bg-amber-50 px-2 py-1.5 text-xs text-amber-900">
	                      {ee("ticketWorkbench.text322")}：{(currentMeeting?.external_participant_names ?? []).join("、") || `${currentMeeting?.external_participant_count ?? 0}${ee("ticketWorkbench.text263")}`}
	                    </div>
	                  ) : null}
	                  <div className="mt-3 grid grid-cols-2 gap-2">
	                    <RailopsButton
	                      className={cn("gap-2", canEndCurrentMeeting ? "" : "col-span-2")}
	                      onClick={() => void handleCreateMeeting()}
	                      disabled={creatingMeeting || endingMeeting}
	                    >
	                      {creatingMeeting ? <Loader2Icon className="size-4 animate-spin" /> : <VideoIcon className="size-4" />}{ee("ticketWorkbench.text267")}</RailopsButton>
	                    {canEndCurrentMeeting ? (
	                      <RailopsButton
	                        className="gap-2 border-destructive/20 text-destructive hover:bg-destructive/10 hover:text-destructive"
	                        onClick={() => void handleEndCurrentMeeting()}
	                        disabled={endingMeeting || creatingMeeting}
	                      >
	                        {endingMeeting ? <Loader2Icon className="size-4 animate-spin" /> : <CircleXIcon className="size-4" />}
	                        {endingMeeting ? ee("ticketWorkbench.text321") : ee("ticketWorkbench.text320")}
	                      </RailopsButton>
	                    ) : null}
	                  </div>
	                </div>
	              ) : null}
              {hasFinishedCurrentMeeting ? (
                <div className="col-span-2 rounded-md border border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
                  {ticketMeetingStatusText(currentMeeting?.status)}，请重新发起视频协作。
                </div>
              ) : null}
              <RailopsButton
                className="gap-2"
                onClick={openMeetingChoice}
						disabled={
							creatingMeeting ||
							endingMeeting ||
							(!(
								canJoinCurrentMeeting
							) && aggregate?.actions.can_start_meeting === false)
					}
              >
                {creatingMeeting ? <Loader2Icon className="size-4 animate-spin" /> : <VideoIcon className="size-4" />}
								{canJoinCurrentMeeting ? ee("ticketWorkbench.text267") : ee("ticketWorkbench.text268")}
              </RailopsButton>
              <RailopsButton
                className="gap-2"
                onClick={() => setRecordOpen(true)}
              >
                <FileTextIcon className="size-4" />{ee("ticketWorkbench.text259")}</RailopsButton>
              <RailopsButton
                className="col-span-2 gap-2 border-destructive/20 text-destructive hover:bg-destructive/10 hover:text-destructive"
                onClick={() => setCancelOpen(true)}
                disabled={aggregate?.actions.can_cancel === false}
              >
                <CircleXIcon className="size-4" />{ee("ticketWorkbench.text269")}</RailopsButton>
            </div>
          )
        ) : null}

        <StandardModal
  open={meetingChoiceOpen}
  onCancel={ () => { if (!creatingMeeting) setMeetingChoiceOpen(false) } }
  title={ee("ticketWorkbench.text268")}
  width={448}
  footer={
    <>
              <RailopsButton onClick={() => setMeetingChoiceOpen(false)} disabled={creatingMeeting}>{ee("ticketWorkbench.text123")}</RailopsButton>
              <RailopsButton className="gap-2" onClick={() => void submitMeetingChoice()} disabled={creatingMeeting}>
                {creatingMeeting ? <Loader2Icon className="size-4 animate-spin" /> : meetingCreateMode === "scheduled" ? <CalendarClockIcon className="size-4" /> : <VideoIcon className="size-4" />}
                {meetingCreateMode === "scheduled" ? ee("ticketWorkbench.text270") : ee("ticketWorkbench.text271")}
              </RailopsButton>
            
    </>
  }
>
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-2">
                <RailopsButton
                  variant={meetingCreateMode === "now" ? "primary" : "default"}
                  className="gap-2"
                  onClick={() => setMeetingCreateMode("now")}
                >
                  <VideoIcon className="size-4" />{ee("ticketWorkbench.text272")}</RailopsButton>
                <RailopsButton
                  variant={meetingCreateMode === "scheduled" ? "primary" : "default"}
                  className="gap-2"
                  onClick={() => setMeetingCreateMode("scheduled")}
                >
                  <CalendarClockIcon className="size-4" />{ee("ticketWorkbench.text270")}</RailopsButton>
              </div>
              {meetingCreateMode === "scheduled" ? (
                <label className="block space-y-2 text-sm font-medium text-foreground">
                  <span>{ee("ticketWorkbench.text265")}</span>
                  <input
                    type="datetime-local"
                    value={meetingScheduledAt}
                    min={toMeetingDatetimeLocalValue(new Date(Date.now() + 60_000))}
                    onChange={(event) => setMeetingScheduledAt(event.target.value)}
                    className="h-10 w-full rounded-md border border-border bg-background px-3 text-sm shadow-sm outline-none transition focus-visible:ring-2 focus-visible:ring-primary/20"
                  />
                </label>
              ) : null}
            </div>
            
</StandardModal>

        <StandardModal
  open={cancelOpen}
  onCancel={ () => setCancelOpen(false) }
  title={ee("ticketWorkbench.text269")}
  width={512}
  footer={
    <>
              <RailopsButton onClick={() => setCancelOpen(false)}>{ee("ticketWorkbench.text273")}</RailopsButton>
              <RailopsButton
                danger
                onClick={() => void handleCancelTicket()}
                disabled={!cancelReason.trim() || cancellingTicket}
              >
                {cancellingTicket ? <Loader2Icon className="size-4 animate-spin" /> : <CircleXIcon className="size-4" />}{ee("ticketWorkbench.text274")}</RailopsButton>
            
    </>
  }
>
            <Input.TextArea
              aria-label={ee("ticketWorkbench.text275")}
              value={cancelReason}
              onChange={(event) => setCancelReason(event.target.value)}
              rows={4}
              placeholder={ee("ticketWorkbench.text275")}
              className="resize-none"
            />
            
</StandardModal>

        <StandardModal
  open={recordOpen}
  onCancel={ () => setRecordOpen(false) }
  title={ee("ticketWorkbench.text276")}
  width={512}
  footer={
    <>
              <RailopsButton onClick={() => setRecordOpen(false)}>{ee("ticketWorkbench.text123")}</RailopsButton>
              <RailopsButton
                onClick={() => void handleCreateProgress()}
                disabled={!recordText.trim() || savingRecord}
              >
                {savingRecord ? <Loader2Icon className="size-4 animate-spin" /> : <FileTextIcon className="size-4" />}{ee("ticketWorkbench.text124")}</RailopsButton>
            
    </>
  }
>
            <Input.TextArea
              aria-label={ee("ticketWorkbench.text277")}
              value={recordText}
              onChange={(event) => setRecordText(event.target.value)}
              rows={5}
              placeholder={ee("ticketWorkbench.text277")}
              className="resize-none"
            />
            <CheckboxField
              style={{ marginBottom: 0 }}
              className="text-sm text-foreground"
              checkboxProps={{
                checked: recordVisibleToCustomer,
                onChange: (event) => setRecordVisibleToCustomer(event.target.checked),
              }}
            >{ee("ticketWorkbench.text278")}</CheckboxField>
            
</StandardModal>

        <StandardModal
  open={repairOpen}
  onCancel={ () => setRepairOpen(false) }
  title={ee("ticketWorkbench.text261")}
  width={512}
  footer={
    <>
              <RailopsButton onClick={() => setRepairOpen(false)}>{ee("ticketWorkbench.text123")}</RailopsButton>
              <RailopsButton
                onClick={() => void handleCreateRepair()}
                disabled={
                  !repairFaultCode.trim() ||
	                  !repairSolution.trim() ||
	                  (repairTestResult === "passed" && !repairRootCause.trim()) ||
	                  savingRepair ||
	                  endingMeeting
	                }
	              >
	                {savingRepair || endingMeeting ? <Loader2Icon className="size-4 animate-spin" /> : <WrenchIcon className="size-4" />}
                {repairTestResult === "passed" ? ee("ticketWorkbench.text279") : ee("ticketWorkbench.text280")}
              </RailopsButton>
            
    </>
  }
>
            <div className="space-y-4">
              <label className="block space-y-2 text-sm font-medium text-foreground">{ee("ticketWorkbench.text281")}<input
                  value={repairFaultCode}
                  onChange={(event) => setRepairFaultCode(event.target.value)}
                  className="h-10 w-full rounded-md border border-border bg-background px-3 text-sm font-normal outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
                  placeholder={ee("ticketWorkbench.text282")}
                />
              </label>
              <label className="block space-y-2 text-sm font-medium text-foreground">{ee("ticketWorkbench.text283")}<Input.TextArea
                  aria-label={ee("ticketWorkbench.text283")}
                  value={repairRootCause}
                  onChange={(event) => setRepairRootCause(event.target.value)}
                  rows={3}
                  placeholder={ee("ticketWorkbench.text284")}
                  className="resize-none font-normal"
                />
              </label>
              <label className="block space-y-2 text-sm font-medium text-foreground">{ee("ticketWorkbench.text285")}<Input.TextArea
                  aria-label={ee("ticketWorkbench.text285")}
                  value={repairSolution}
                  onChange={(event) => setRepairSolution(event.target.value)}
                  rows={4}
                  placeholder={ee("ticketWorkbench.text286")}
                  className="resize-none font-normal"
                />
              </label>
              <SelectField
                label={ee("ticketWorkbench.text287")}
                style={{ marginBottom: 0 }}
                selectProps={{
                  "aria-label": ee("ticketWorkbench.text287"),
                  value: repairTestResult,
                  onChange: (value) => setRepairTestResult((value as string) ?? "failed"),
                  options: [
                    { value: "passed", label: ee("ticketWorkbench.text288") },
                    { value: "partial", label: ee("ticketWorkbench.text289") },
                    { value: "failed", label: ee("ticketWorkbench.text290") },
                  ],
                  style: { width: "100%" },
                }}
              />
            </div>
	            
</StandardModal>
    </div>
  )
}

function ContextSection({
  title,
  action,
  children,
}: {
  title: string
  action?: ReactNode
  children: ReactNode
}) {
  return (
    <section className="border-b border-border p-4">
      <div className="mb-2 flex items-center justify-between gap-3">
        <strong className="text-sm font-semibold text-foreground">{title}</strong>
        {action}
      </div>
      {children}
    </section>
  )
}

function InfoTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-border bg-muted px-2 py-1.5">
      <div className="text-rhd-xs text-muted-foreground">{label}</div>
      <div className="mt-0.5 truncate text-xs font-semibold text-foreground">{value}</div>
    </div>
  )
}

function ContextRow({
  label,
  value,
  badge,
  tone = "neutral",
}: {
  label: string
  value: string
  badge?: string
  tone?: Tone
}) {
  return (
    <div className="flex items-center justify-between gap-3 text-xs">
      <span className="shrink-0 font-medium text-foreground">{label}</span>
      <span className="flex min-w-0 items-center gap-2">
        <span className="min-w-0 truncate text-muted-foreground">{value}</span>
        {badge ? (
          <StatusTag
            tone="neutral"
            className={cn("h-5 shrink-0 px-1.5 text-rhd-xs", statusToneClassName(tone))}
          >
            {badge}
          </StatusTag>
        ) : null}
      </span>
    </div>
  )
}

function EmptyLine({ text }: { text: string }) {
  return <div className="text-xs text-muted-foreground">{text}</div>
}

function LoadingLine({ text }: { text: string }) {
  return (
    <div
      className="space-y-2 rounded-md border border-border bg-card p-2.5"
      role="status"
      aria-busy="true"
      aria-label={text}
    >
      <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
        <Loader2Icon className="size-3.5 animate-spin" aria-hidden="true" />
        <span>{text}</span>
      </div>
      <div className="space-y-2">
        <Skeleton.Node active style={{ width: "80%", height: 12 }} />
        <Skeleton.Node active style={{ width: "66.6667%", height: 12 }} />
      </div>
    </div>
  )
}

function ProgressStageTimeline({ items }: { items: WorkbenchTimeline[] }) {
  if (items.length === 0) {
    return <EmptyLine text={ee("ticketWorkbench.text291")} />
  }
  const completedCount = items.filter((item) => (item.state ?? "waiting") === "done").length
  const activeIndex = items.findIndex((item) => item.state === "current" || item.state === "risk")
  const progressRatio = items.length > 0
    ? Math.max(completedCount, activeIndex >= 0 ? activeIndex + 0.5 : 0) / items.length
    : 0
  return (
    <div className="space-y-4">
      <div className="border-b border-border pb-3">
        <div className="mb-2 flex items-center justify-between gap-2">
          <span className="text-rhd-xs font-medium text-muted-foreground">{completedCount}/{items.length}</span>
          <span className="text-rhd-xs font-semibold text-primary">{Math.round(progressRatio * 100)}%</span>
        </div>
        <div className="grid gap-1" style={{ gridTemplateColumns: `repeat(${items.length}, minmax(0, 1fr))` }}>
          {items.map((item, index) => {
            const meta = progressStageMeta(item.state ?? (index === items.length - 1 ? "current" : "done"))
            return (
              <div key={item.key ?? `${item.title}-${index}`} className="min-w-0">
                <div className={cn("h-1 rounded-full", meta.trackClassName)} />
              </div>
            )
          })}
        </div>
        <div className="mt-1 h-1 overflow-hidden rounded-full bg-muted">
          <div
            className="h-full rounded-full bg-primary transition-all"
            style={{ width: `${Math.min(100, Math.max(0, progressRatio * 100))}%` }}
          />
        </div>
      </div>
      {items.map((item, index) => {
        const state = item.state ?? (index === items.length - 1 ? "current" : "done")
        const meta = progressStageMeta(state)
        const Icon = meta.icon
        return (
          <div
            key={item.key ?? `${item.time}-${item.title}-${index}`}
            className="grid grid-cols-[2rem_minmax(0,1fr)] gap-3"
          >
            <div className="relative flex justify-center">
              <span className={cn("relative z-10 mt-0.5 flex size-6 items-center justify-center rounded-full border", meta.nodeClassName)}>
                <Icon className="size-3.5" />
              </span>
              {index < items.length - 1 ? (
                <span className={cn("absolute bottom-[-1rem] top-7 w-px", meta.lineClassName)} />
              ) : null}
            </div>
            <div className="min-w-0">
              <div className="flex min-h-6 min-w-0 items-start justify-between gap-2">
                <div className={cn("min-w-0 text-xs leading-5", item.state === "current" || item.state === "risk" ? "font-semibold text-foreground" : "font-medium text-muted-foreground")}>
                  {item.title}
                </div>
                {item.status ? (
                  <span className={cn("shrink-0 rounded-full border px-1.5 py-0.5 text-rhd-2xs font-medium", meta.badgeClassName)}>
                    {item.status}
                  </span>
                ) : null}
              </div>
              <div className="mt-1 flex flex-wrap gap-x-2 gap-y-1 text-rhd-xs text-muted-foreground">
                <time>{item.time}</time>
                {item.actor ? <span>{item.actor}</span> : null}
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}

function RecordPipeline({ items }: { items: WorkbenchTimeline[] }) {
  if (items.length === 0) {
    return <EmptyLine text={ee("ticketWorkbench.text291")} />
  }
  return (
    <div className="space-y-0">
      {items.map((item, index) => {
        const latest = index === items.length - 1
        return (
          <div key={item.key ?? `${item.time}-${item.title}-${index}`} className="grid grid-cols-[3.75rem_minmax(0,1fr)] gap-2">
            <time className="pt-1 text-right text-rhd-2xs font-medium leading-5 text-muted-foreground">
              {item.time}
            </time>
            <div className="relative pb-3 pl-4">
              <span
                className={cn(
                  "absolute left-0 top-1 flex size-2.5 -translate-x-1/2 rounded-full border-2 bg-card",
                  latest ? "border-primary" : "border-muted-foreground/50",
                )}
              />
              {index < items.length - 1 ? (
                <span className="absolute bottom-0 left-0 top-4 w-px bg-border" />
              ) : null}
              <div
                className={cn(
                  "rounded-md border px-2.5 py-2",
                  latest ? "border-primary/25 bg-primary/5" : "border-border bg-card",
                )}
              >
                <div className="flex min-w-0 items-start justify-between gap-2">
                  <div className="min-w-0 text-xs font-semibold leading-5 text-foreground">
                    {item.title}
                  </div>
                  {item.status ? (
                    <span className="shrink-0 rounded-full border border-border bg-muted px-1.5 py-0.5 text-rhd-2xs font-medium text-muted-foreground">
                      {item.status}
                    </span>
                  ) : null}
                </div>
                {item.actor ? (
                  <div className="mt-1 text-rhd-xs leading-5 text-muted-foreground">
                    {item.actor}
                  </div>
                ) : null}
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}

function progressStageMeta(state: WorkbenchTimeline["state"]) {
  switch (state) {
    case "done":
      return {
        icon: CheckCircle2Icon,
        trackClassName: "bg-primary",
        cardClassName: "border-primary/20 bg-primary/5",
        nodeClassName: "border-primary/25 bg-primary text-primary-foreground",
        lineClassName: "bg-primary/30",
        badgeClassName: "border-primary/20 bg-primary/10 text-primary",
      }
    case "current":
      return {
        icon: CircleDotIcon,
        trackClassName: "bg-primary",
        cardClassName: "border-primary/40 bg-primary/15 shadow-[0_8px_20px_rgba(59,130,246,0.16)]",
        nodeClassName: "border-primary bg-primary text-primary-foreground ring-4 ring-primary/15",
        lineClassName: "bg-primary/25",
        badgeClassName: "border-primary/20 bg-primary/10 text-primary",
      }
    case "risk":
      return {
        icon: CircleAlertIcon,
        trackClassName: "bg-destructive/70",
        cardClassName: "border-destructive/25 bg-destructive/5",
        nodeClassName: "border-destructive/25 bg-destructive text-destructive-foreground",
        lineClassName: "bg-border",
        badgeClassName: "border-destructive/20 bg-destructive/10 text-destructive",
      }
    default:
      return {
        icon: CircleDotIcon,
        trackClassName: "bg-border",
        cardClassName: "border-dashed border-border bg-muted/25 opacity-75",
        nodeClassName: "border-border bg-muted text-muted-foreground",
        lineClassName: "bg-border",
        badgeClassName: "border-border bg-muted text-muted-foreground",
      }
  }
}

function buildChatMessages(
  item: WorkbenchQueueItem | null,
  loadedConversationId: number | null,
  messages: AgentMessage[],
): WorkbenchMessage[] {
  if (!item?.conversationId || loadedConversationId !== item.conversationId) {
    return []
  }
  if (messages.length > 0) {
    return messages.map((message) => {
      const senderType = message.senderType.toLowerCase()
      const isCustomer = senderType.includes("customer") || senderType.includes("user")
      const isAi = senderType.includes("ai") || senderType.includes("bot")
      const isSystem = senderType.includes("system")
      const isPartner = senderType.includes("partner")
      const displayContent = normalizeEnterpriseWorkbenchMessageCopy(message.content)
      return {
        id: message.id,
        role:
          message.senderName ||
          (isCustomer ? ee("ticketWorkbench.text166") : isAi ? ee("ticketWorkbench.text009") : isSystem ? ee("ticketWorkbench.text233") : isPartner ? ee("ticketWorkbench.text167") : ee("ticketWorkbench.text168")),
        content: message.content,
        html: renderIMMessageHTML({ ...message, content: displayContent }),
        time: formatShortDateTime(message.sentAt),
        side: isCustomer || isPartner ? "left" : "right",
        senderType,
        senderId: message.senderId,
        senderName: message.senderName,
        senderAvatar: message.senderAvatar,
        tone: isCustomer ? "customer" : isAi ? "ai" : isSystem ? "system" : isPartner ? "partner" : "engineer",
        serviceEvent: parseConversationServiceEvent({
          senderType,
          content: message.content,
          payload: message.payload,
        }),
      }
    })
  }
  return []
}

const LEGACY_AFTERSALES_COMMAND_TITLE_PATTERN = /\u552e\u540e\u5de5\u5355\u6307\u6325\u53f0/g
const LEGACY_CONVERSATION_WORKBENCH_TITLE_PATTERN = /\u4f1a\u8bdd\u5de5\u4f5c\u53f0/g
const LEGACY_SERVICE_WORKBENCH_TITLE_PATTERN = /\u670d\u52a1\u5de5\u4f5c\u53f0/g
const CHINESE_DURATION_PATTERN = /^\s*(?:(\d+)\s*分)?\s*(?:(\d+)\s*秒)?\s*$/

function normalizeEnterpriseWorkbenchMessageCopy(value: string, hasDeviceConcept = true) {
  return localizeEnterpriseGeneratedLifecycleCopy(value, hasDeviceConcept)
    .replaceAll(LEGACY_AFTERSALES_COMMAND_TITLE_PATTERN, ee("ticketWorkbench.text292"))
    .replaceAll(LEGACY_CONVERSATION_WORKBENCH_TITLE_PATTERN, ee("ticketWorkbench.text293"))
    .replaceAll(LEGACY_SERVICE_WORKBENCH_TITLE_PATTERN, ee("ticketWorkbench.text294"))
}

function localizeEnterpriseGeneratedLifecycleCopy(value: string, hasDeviceConcept = true) {
  const trimmed = value.trim()
  if (!trimmed) return value

  const ticketCreated = trimmed.match(/^已生成服务工单(?:\s+([^，,]+))?[，,].*同步至工单[。.]?$/)
  if (ticketCreated) {
    const ticketNo = ticketCreated[1]?.trim()
    return ticketNo
      ? cee(hasDeviceConcept ? "ticketCreatedDescriptionWithTicketNo" : "ticketCreatedDescriptionWithTicketNoGeneral", { ticketNo })
      : cee(hasDeviceConcept ? "ticketCreatedDescription" : "ticketCreatedDescriptionGeneral")
  }

  if (trimmed === "技术支持组成员接管并受理") {
    return cee("ticketSupportTakeoverAccepted")
  }
  if (trimmed === "同产品维修组成员接管并受理") {
    return cee(hasDeviceConcept ? "ticketSameProductTakeoverAccepted" : "ticketSupportTakeoverAccepted")
  }
  if (trimmed === "技术支持组成员从待派单池转派给自己并受理") {
    return cee("ticketSupportPoolTakeoverAccepted")
  }
  if (trimmed === "产品组成员从待派单池转派给自己并受理") {
    return cee(hasDeviceConcept ? "ticketPoolTakeoverAccepted" : "ticketSupportPoolTakeoverAccepted")
  }
  if (trimmed === "原处理人接单超时，技术支持组成员接管并受理") {
    return cee("ticketExpiredSupportTakeoverAccepted")
  }
  if (trimmed === "原处理人接单超时，产品组成员接管并受理") {
    return cee(hasDeviceConcept ? "ticketExpiredTakeoverAccepted" : "ticketExpiredSupportTakeoverAccepted")
  }

  if (/^工程师已发起视频协作[，,]/.test(trimmed)) {
    return cee("videoStartedDescription")
  }

  const scheduledVideo = trimmed.match(/^工程师已预定视频协作[，,]\s*计划时间[:：]\s*([^，,]+)[，,]/)
  if (scheduledVideo) {
    return cee("videoScheduledDescriptionWithTime", { time: scheduledVideo[1].trim() })
  }
  if (/^工程师已预定视频协作[，,]/.test(trimmed)) {
    return cee("videoScheduledDescription")
  }

  const videoEnded = trimmed.match(/^视频协作已结束(?:[，,]\s*会议室[:：]\s*([^，,]+))?(?:[，,]\s*时长[:：]\s*(.+))?[。.]?$/)
  if (videoEnded) {
    const roomName = videoEnded[1]?.trim()
    const duration = localizeEnterpriseEventDuration(videoEnded[2])
    if (roomName && duration) {
      return cee("videoEndedDescriptionWithRoomDuration", { roomName, duration })
    }
    if (duration) {
      return cee("videoEndedDescriptionWithDuration", { duration })
    }
    return cee("videoEndedDescription")
  }

  const supplierInvited = trimmed.match(/^(?:已邀请供应商协作|升级供应商协作)[:：]\s*(.+?)(?:\s*[·/]\s*(.+))?$/)
  if (supplierInvited) {
    const company = supplierInvited[1].trim()
    const module = supplierInvited[2]?.trim()
    return module
      ? cee("supplierInvitedDescriptionWithModule", { company, module })
      : cee("supplierInvitedDescriptionWithCompany", { company })
  }

  const supplierJoined = trimmed.match(/^(.+?)已加入协作会话[，,].*供应商可共同沟通[。.]?$/)
  if (supplierJoined) {
    return cee("supplierJoinedDescriptionWithCompany", { company: supplierJoined[1].trim() })
  }
  if (/^供应商已加入协作会话/.test(trimmed)) {
    return cee("supplierJoinedDescription")
  }

  const supplierResolved = trimmed.match(/^供应商处理完成[:：]\s*(.+)$/)
  if (supplierResolved) {
    return cee("supplierResolvedDescriptionWithResolution", { resolution: supplierResolved[1].trim() })
  }

  const feedback = trimmed.match(/^客户已提交服务评价[:：]\s*([1-5])\/5(?:\s*[·,，]\s*(.+))?$/)
  if (feedback) {
    const rating = Number(feedback[1])
    const comment = feedback[2]?.trim()
    return comment
      ? cee("feedbackSubmittedDescriptionWithComment", { rating, comment })
      : cee("feedbackSubmittedDescription", { rating })
  }
  if (/^客户已完成服务评价/.test(trimmed)) {
    return cee("feedbackSubmittedDescriptionNoRating")
  }

  const repairConclusion = trimmed.match(/^维修结论已提交[:：]\s*(.+)$/)
  if (repairConclusion) {
    return cee("repairConclusionDescriptionWithSummary", { summary: repairConclusion[1].trim() })
  }

  return value
}

function localizeEnterpriseEventDuration(value: string | undefined) {
  const duration = value?.trim() ?? ""
  if (!duration) return ""

  const match = duration.match(CHINESE_DURATION_PATTERN)
  if (!match || (!match[1] && !match[2])) {
    return duration
  }

  const minutes = Number(match[1] ?? 0)
  const seconds = Number(match[2] ?? 0)
  if (minutes > 0 && seconds > 0) {
    return cee("durationMinutesSeconds", { minutes, seconds })
  }
  if (minutes > 0) {
    return cee("durationMinutes", { minutes })
  }
  return cee("durationSeconds", { seconds })
}

function buildLightDetail(
  item: WorkbenchQueueItem | null,
  conversation: AgentConversation | null,
  hasDeviceConcept = true,
): WorkbenchDetail {
  if (!item) {
    return {
      serviceSummary: ee("ticketWorkbench.text295"),
      conclusion: ee("ticketWorkbench.text296"),
      tags: [],
      attachments: [],
      parts: [],
      rating: ee("ticketWorkbench.text297"),
      feedback: "",
      flow: [],
      timeline: [],
    }
  }

  return {
    serviceSummary: normalizeEnterpriseWorkbenchMessageCopy(item.summary || conversation?.lastMessageSummary || item.title, hasDeviceConcept),
    conclusion: item.isDone ? ee("ticketWorkbench.text298") : ee("ticketWorkbench.text299"),
    tags: [item.source, item.device, item.sla].filter(Boolean),
    attachments: [],
    parts: [],
    rating: ee("ticketWorkbench.text297"),
    feedback: "",
    flow: [
      { time: item.updatedLabel, title: item.statusLabel, actor: item.owner, state: item.isDone ? "done" : "current" },
    ],
    timeline: [
      { time: item.updatedLabel, title: item.title, actor: item.customer, status: item.statusLabel, state: item.isDone ? "done" : "current" },
    ],
  }
}

function buildAggregateDetail(aggregate: TicketAggregateDTO, hasDeviceConcept = true): WorkbenchDetail {
  const ticket = aggregate.ticket
  const repair = aggregate.repair
  const diagnosis = aggregate.diagnosis_snapshot
  const serviceSummary = normalizeEnterpriseWorkbenchMessageCopy(
    ticket.description || aggregate.conversation_snapshot?.summary || ee("ticketWorkbench.text295"),
    hasDeviceConcept,
  )
  const conclusion =
    repair?.resolution ||
    diagnosis?.recommended_actions?.[0] ||
    (isTerminalTicketStatus(ticket.status) ? ee("ticketWorkbench.text300") : ee("ticketWorkbench.text301"))
  const diagnosisTriage = diagnosis?.triage_level && diagnosis.triage_level !== ticket.priority
    ? ee("ticketWorkbench.text302", { value0: priorityFullLabel(diagnosis.triage_level) })
    : ""
  const tags = Array.from(new Set([
    priorityFullLabel(ticket.priority),
    ticketSourceLabel(ticket.source),
    diagnosis?.fault_category,
    diagnosisTriage,
  ].filter((value): value is string => Boolean(value))))

  return {
    serviceSummary,
    conclusion,
    tags,
    attachments: buildAggregateAttachments(aggregate.assets),
    parts: buildAggregateParts(repair?.parts ?? []),
    rating: aggregate.feedback ? `${aggregate.feedback.rating}/5` : ee("ticketWorkbench.text297"),
    feedback: aggregate.feedback
      ? [aggregate.feedback.comment, ...(aggregate.feedback.tags ?? [])].filter(Boolean).join(" · ")
      : "",
    flow: buildAggregateFlow(aggregate.flow.steps),
    timeline: buildAggregateTimeline(aggregate.timeline, hasDeviceConcept),
  }
}

function buildAggregateAttachments(assets: AssetRefDTO[] | undefined): WorkbenchAttachment[] {
  if (!assets || assets.length === 0) {
    return []
  }
  return assets.map((asset) => ({
    label: asset.file_type || ee("ticketWorkbench.text161"),
    desc: asset.file_name,
    status: ee("ticketWorkbench.text303"),
    tone: "good",
  }))
}

function buildAggregateParts(parts: RepairPartDTO[]): WorkbenchPart[] {
  if (parts.length === 0) {
    return []
  }
  return parts.map((part) => ({
    name: part.name,
    desc: ee("ticketWorkbench.text304"),
    quantity: String(part.quantity),
  }))
}

function buildAggregateTimeline(
  timeline: TicketTimelineItemDTO[] | undefined,
  hasDeviceConcept = true,
): WorkbenchTimeline[] {
  if (timeline && timeline.length > 0) {
    const seen = new Set<string>()
    const items = timeline.flatMap((item) => {
      const title = timelineContent(item.content, hasDeviceConcept)
      const duplicateKey = `${item.timestamp}-${title}`
      if (seen.has(duplicateKey)) return []
      seen.add(duplicateKey)
      return [{
        key: `${item.type}-${item.id}-${item.timestamp}`,
        time: formatShortDateTime(item.timestamp),
        title,
		actor: item.actor,
		status: timelineEventLabel(item.type),
      }]
    })
    return items.map((item, index) => ({
      ...item,
      state: index === items.length - 1 ? "current" : "done",
    }))
  }
  return []
}

function timelineEventLabel(type: string) {
  const normalized = type.trim().toLowerCase()
  const map: Record<string, string> = {
    created: ee("ticketWorkbench.text305"),
    ticket_created: ee("ticketWorkbench.text305"),
    ticket_accepted: ee("ticketWorkbench.text306"),
    ticket_assigned: ee("ticketWorkbench.text213"),
    ticket_processing: ee("ticketWorkbench.text011"),
    ticket_escalated: ee("ticketWorkbench.text227"),
    repair_completed: ee("ticketWorkbench.text307"),
    ticket_closed: ee("ticketWorkbench.text308"),
    ticket_reopened: ee("ticketWorkbench.text309"),
    meeting_ended: ee("ticketWorkbench.text310"),
    progress: ee("ticketWorkbench.text259"),
  }
  if (map[normalized]) return map[normalized]
  return ee("ticketWorkbench.text311")
}

function buildAggregateFlow(steps: TicketFlowStepDTO[] | undefined): WorkbenchTimeline[] {
  if (!steps || steps.length === 0) {
    return []
  }
  return steps.map((step) => ({
    time: step.completed_at ? formatShortDateTime(step.completed_at) : ee("ticketWorkbench.text048"),
    title: flowStepLabel(step.name),
    actor: step.completed_by,
    status:
      step.status === "done"
        ? ee("ticketWorkbench.text012")
        : step.status === "current"
          ? ee("ticketWorkbench.text011")
          : ee("ticketWorkbench.text048"),
    state:
      step.status === "done"
        ? "done"
        : step.status === "current"
          ? "current"
          : "waiting",
  }))
}

function timelineContent(content: string, hasDeviceConcept = true) {
  const localized = localizeEnterpriseGeneratedLifecycleCopy(content, hasDeviceConcept)
  if (localized !== content) return localized

  const map: Record<string, string> = {
    "Ticket created": ee("ticketWorkbench.text305"),
    "Created ticket": ee("ticketWorkbench.text305"),
  }
  return map[content] ?? content
}

function flowStepLabel(name: string) {
  const map: Record<string, string> = {
    Accept: ee("ticketWorkbench.text306"),
    Dispatch: ee("ticketWorkbench.text213"),
    Process: ee("ticketWorkbench.text090"),
    Repair: ee("ticketWorkbench.text312"),
    Close: ee("ticketWorkbench.text308"),
  }
  return map[name] ?? name
}

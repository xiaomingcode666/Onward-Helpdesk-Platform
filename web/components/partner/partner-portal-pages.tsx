"use client"

import {
  AlertTriangleIcon,
  ArrowLeftIcon,
  CalendarCheck2Icon,
  CheckCircle2Icon,
  Clock3Icon,
  CopyIcon,
  FileTextIcon,
  MessageSquareTextIcon,
  ImageIcon,
  PaperclipIcon,
  PencilIcon,
  RefreshCwIcon,
  TicketCheckIcon,
  UsersIcon,
  UserMinusIcon,
  UserPlusIcon,
  VideoIcon,
} from "lucide-react"
import Link from "next/link"
import { useParams, useRouter, useSearchParams } from "next/navigation"
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import { toast } from "sonner"

import { SearchField, SelectField, StandardModal } from "@railops/ui"

import { resolvePartnerMeetingAccess } from "@/components/partner/partner-meeting-access"
import { PageHeader } from "@/components/layout/page-header"
import { EmptyState, ErrorState } from "@/components/shared/error-states"
import { ModuleLoading } from "@/components/shared/loading-states"
import { VoiceRecorderButton } from "@/components/chat/voice-recorder-button"
import { ImMessageHTML } from "@/components/im-message-html"
import { MeetingLiveRoom } from "@/components/meeting/meeting-live-room"
import {
  ConversationServiceEvent,
  parseConversationServiceEvent,
} from "@/components/remote-helpdesk/conversation-service-event"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Textarea } from "@/components/ui/textarea"
import type { TicketSupplierCollaboration } from "@/lib/api/enterprise-tickets"
import type { MeetingJoinConfig, MeetingListItem, MeetingListResponse, MeetingTranscriptSegment } from "@/lib/api/types"
import { createAdminWebSocket } from "@/lib/api/admin"
import {
  expireRealtimePresenceActor,
  expireRealtimeTypingActor,
  mergeRealtimePresenceActor,
  presenceFromRealtimeTyping,
  realtimePresenceExpiryDelay,
  TYPING_IDLE_TIMEOUT_MS,
  type RealtimePresencePayload,
  type RealtimePresenceSnapshotPayload,
  type RealtimeTypingPayload,
  type RealtimeTypingState,
  updateRealtimeTypingState,
} from "@/lib/im-realtime-state"
import { createRealtimeConnectionManager } from "@/lib/realtime-connection"
import {
  acceptPartnerTicket,
  addPartnerTicketParticipant,
  assignPartnerTicket,
  confirmPartnerMeetingJoined,
  confirmPartnerMeetingLeft,
  createPartnerAccount,
  createPartnerTicketProgress,
  createPartnerTicketMessage,
  fetchPartnerAccounts,
  fetchPartnerConversations,
  fetchPartnerMeetingJoinConfig,
  fetchPartnerMeetingStatus,
  fetchPartnerMeetingTranscriptPage,
  fetchPartnerMeetingTranscripts,
  fetchPartnerMeetings,
  fetchPartnerProfile,
  fetchPartnerTicketDetail,
  fetchPartnerTickets,
  heartbeatPartnerMeeting,
  ingestPartnerMeetingTranscript,
  resolvePartnerTicket,
  removePartnerTicketParticipant,
  updatePartnerAccount,
  uploadPartnerTicketAttachment,
  uploadPartnerTicketAudio,
  uploadPartnerTicketImage,
  type PartnerAccountPayload,
  type PartnerPortalAccount,
  type PartnerPortalProfile,
  type PartnerConversationMessage,
  type PartnerTicketDetail,
} from "@/lib/api/partner"
import { customerDisplayName } from "@/lib/customer-identity"
import { cn } from "@/lib/utils"
import { publishMeetingSync } from "@/lib/meeting-sync"
import { isStaticExportRouteParam } from "@/lib/static-export-route"
import { translateCurrentMessage } from "@/i18n/messages"
import { useI18n } from "@/i18n/provider"
import {
  buildMediaTransferPayload,
  createPendingMessageId,
  renderIMMessageHTML,
} from "@/lib/im-message"

const partnerPanelClass = "rhd-railops-partner-panel overflow-hidden rounded-md bg-[var(--railops-surface)] shadow-[var(--railops-card-shadow)]"
const partnerPanelHeaderClass = "rhd-railops-partner-panel-head border-b border-[var(--railops-border-light)] p-3"
const partnerMutedTextClass = "text-[var(--railops-text-secondary)]"
const partnerTitleClass = "text-[var(--railops-text)]"
const partnerDividerClass = "divide-y divide-[var(--railops-border-light)]"
const partnerTableHeadClass = "rhd-railops-partner-table-head border-b border-[var(--railops-border-light)] bg-[var(--railops-surface-muted)] text-left text-xs font-semibold text-[var(--railops-text-secondary)]"
const partnerTableCellClass = "rhd-railops-partner-table-cell border-b border-[var(--railops-border-light)] px-3 py-2"
const partnerFilterBaseClass = "rhd-railops-partner-filter h-8 rounded-md border px-3 text-xs font-medium"
const partnerFilterActiveClass = "border-[#bfdbfe] bg-[var(--railops-primary-bg)] text-[var(--railops-primary-hover)]"
const partnerFilterIdleClass = "border-[var(--railops-border)] text-[var(--railops-text-secondary)] hover:bg-[var(--railops-primary-bg)] hover:text-[var(--railops-primary-hover)]"

function pt(key: string, values?: Record<string, string | number>) {
  return translateCurrentMessage(`partnerExtract.${key}`, values)
}

function dateText(value?: string) {
  if (!value) return "-"
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function collaborationScopeText(item: Pick<TicketSupplierCollaboration, "product_id" | "product_module_name">) {
  if (item.product_id <= 0) return pt("fallback.generalSupport")
  return item.product_module_name || pt("fallback.unspecifiedModule")
}

type PartnerRequestResult<T> = {
  success: boolean
  data?: T
  error?: { message?: string } | null
}

async function safePartnerRequest<T>(
  request: () => Promise<PartnerRequestResult<T>>,
  fallback: string,
): Promise<PartnerRequestResult<T>> {
  try {
    const response = await request()
    if (!response.success && !response.error?.message) {
      return { ...response, error: { message: fallback } }
    }
    return response
  } catch (error) {
    return {
      success: false,
      error: { message: error instanceof Error ? error.message : fallback },
    }
  }
}

function isSystemMeetingTranscript(item: MeetingTranscriptSegment) {
  return item.provider?.toLowerCase() === "system" || item.ingestSource?.toLowerCase() === "system"
}

function collaborationStatus(status: string) {
  switch (status) {
    case "invited":
      return { label: pt("status.collaboration.invited"), className: "border-amber-200 bg-amber-50 text-foreground" }
    case "accepted":
    case "processing":
      return { label: pt("status.collaboration.processing"), className: "border-primary/20 bg-primary/10 text-primary" }
    case "resolved":
      return { label: pt("status.collaboration.resolved"), className: "border-border bg-muted text-muted-foreground" }
    case "timeout":
      return { label: pt("status.collaboration.timeout"), className: "border-destructive/20 bg-destructive/10 text-destructive" }
    case "expired":
      return { label: pt("status.collaboration.expired"), className: "border-border bg-muted text-muted-foreground" }
    default:
      return { label: status || pt("common.unknown"), className: "border-border bg-muted text-muted-foreground" }
  }
}

function isPartnerCollaborationTerminal(status?: string) {
  return status === "resolved" || status === "timeout" || status === "expired"
}

function isPartnerCollaborationWorkable(item: Pick<TicketSupplierCollaboration, "status" | "authorization_active">) {
  return !isPartnerCollaborationTerminal(item.status) && Boolean(item.authorization_active)
}

function partnerTicketDetailHref(collaborationID: number | string | null | undefined) {
  return `/partner/ticket-detail?collaboration_id=${encodeURIComponent(String(collaborationID ?? ""))}`
}

function partnerMeetingRoomHref(collaborationID: number | string, meetingID: string | number) {
  return `/partner/meeting-room?meeting_id=${encodeURIComponent(String(meetingID))}&collaboration_id=${encodeURIComponent(String(collaborationID))}`
}

function partnerAccountSelectOptions(accounts: PartnerPortalAccount[]) {
  return accounts.map((account) => ({
    value: String(account.id),
    label: account.display_name || account.username || pt("fallback.partnerMemberWithId", { id: account.id }),
  }))
}

function ticketStatus(status: string) {
  switch (status) {
    case "pending":
      return { label: pt("status.ticket.pending"), className: "border-amber-200 bg-amber-50 text-foreground" }
    case "accepted":
      return { label: pt("status.ticket.accepted"), className: "border-primary/20 bg-primary/10 text-primary" }
    case "processing":
      return { label: pt("status.ticket.processing"), className: "border-primary/20 bg-primary/10 text-primary" }
    case "supplier_support":
      return { label: pt("status.ticket.supplierSupport"), className: "border-primary/20 bg-primary/10 text-primary" }
    case "video_support":
      return { label: pt("status.ticket.videoSupport"), className: "border-primary/20 bg-primary/10 text-primary" }
    case "resolved":
    case "done":
      return { label: pt("status.ticket.resolved"), className: "border-border bg-muted text-muted-foreground" }
    case "closed":
      return { label: pt("status.ticket.closed"), className: "border-border bg-muted text-muted-foreground" }
    case "cancelled":
      return { label: pt("status.ticket.cancelled"), className: "border-destructive/20 bg-destructive/10 text-destructive" }
    default:
      return { label: status || pt("common.unknown"), className: "border-border bg-muted text-muted-foreground" }
  }
}

function meetingStatus(status: MeetingListItem["status"]) {
  switch (status) {
    case "active":
      return { label: pt("status.meeting.active"), className: "border-primary/20 bg-primary/10 text-primary" }
    case "scheduled":
      return { label: pt("status.meeting.scheduled"), className: "border-primary/20 bg-primary/10 text-primary" }
    case "waiting":
      return { label: pt("status.meeting.waiting"), className: "border-amber-200 bg-amber-50 text-foreground" }
    case "ended":
    case "finished":
      return { label: pt("status.meeting.ended"), className: "border-border bg-muted text-muted-foreground" }
    default:
      return { label: status || pt("common.unknown"), className: "border-border bg-muted text-muted-foreground" }
  }
}

function meetingDurationText(seconds?: number) {
  const value = Number(seconds || 0)
  if (value <= 0) return "-"
  const minutes = Math.floor(value / 60)
  if (minutes < 60) return pt("time.minutes", { count: minutes })
  const hours = Math.floor(minutes / 60)
  return pt("time.hoursMinutes", { hours, minutes: minutes % 60 })
}

function canJoinMeetingStatus(status?: string) {
  return status === "waiting" || status === "scheduled" || status === "active"
}

function SummaryMetric({
  label,
  value,
  icon,
  tone,
}: {
  label: string
  value: number
  icon: ReactNode
  tone: "blue" | "amber" | "green" | "slate"
}) {
  const toneClass = {
    blue: "bg-primary/10 text-primary",
    amber: "bg-amber-50 text-foreground",
    green: "bg-primary/10 text-primary",
    slate: "bg-muted text-muted-foreground",
  }[tone]
  return (
    <div className="rhd-railops-partner-summary-metric rounded-md border border-border bg-card p-4 shadow-sm">
      <div className="flex items-center justify-between gap-3">
        <div>
          <div className="text-xs font-medium text-muted-foreground">{label}</div>
          <div className="mt-2 text-2xl font-semibold text-foreground">{value}</div>
        </div>
        <div className={cn("grid size-9 place-items-center rounded-md", toneClass)}>{icon}</div>
      </div>
    </div>
  )
}

export function PartnerOverviewPage() {
  const t = useI18n()
  const [profile, setProfile] = useState<PartnerPortalProfile | null>(null)
  const [tickets, setTickets] = useState<TicketSupplierCollaboration[]>([])
  const [accounts, setAccounts] = useState<PartnerPortalAccount[]>([])
  const [activeMeetings, setActiveMeetings] = useState(0)
  const [snapshotTime, setSnapshotTime] = useState(0)
  const [profileLoading, setProfileLoading] = useState(true)
  const [profileLoaded, setProfileLoaded] = useState(false)
  const [profileError, setProfileError] = useState("")
  const [ticketsLoading, setTicketsLoading] = useState(true)
  const [ticketsLoaded, setTicketsLoaded] = useState(false)
  const [ticketsError, setTicketsError] = useState("")
  const [accountsLoading, setAccountsLoading] = useState(true)
  const [accountsLoaded, setAccountsLoaded] = useState(false)
  const [accountsError, setAccountsError] = useState("")
  const [meetingsLoading, setMeetingsLoading] = useState(true)
  const [meetingsLoaded, setMeetingsLoaded] = useState(false)
  const [meetingsError, setMeetingsError] = useState("")
  const [busyID, setBusyID] = useState<number | null>(null)
  const [assigning, setAssigning] = useState<TicketSupplierCollaboration | null>(null)
  const [assigneeID, setAssigneeID] = useState("")
  const router = useRouter()

  const load = useCallback((silent = false) => {
    const loadProfile = async () => {
      if (!silent) setProfileLoading(true)
      setProfileError("")
      const profileRes = await safePartnerRequest(() => fetchPartnerProfile(), t("partnerExtract.errors.loadPartnerProfileFailed"))
      if (profileRes.success) {
        setProfile(profileRes.data ?? null)
        setProfileLoaded(true)
      } else {
        setProfileError(profileRes.error?.message || t("partnerExtract.errors.loadPartnerProfileFailed"))
      }
      if (!silent) setProfileLoading(false)
    }
    const loadTickets = async () => {
      if (!silent) setTicketsLoading(true)
      setTicketsError("")
      const ticketRes = await safePartnerRequest(() => fetchPartnerTickets(), t("partnerExtract.errors.loadPartnerTodoFailed"))
      if (ticketRes.success) {
        setTickets(ticketRes.data ?? [])
        setTicketsLoaded(true)
        setSnapshotTime(Date.now())
      } else {
        setTicketsError(ticketRes.error?.message || t("partnerExtract.errors.loadPartnerTodoFailed"))
      }
      if (!silent) setTicketsLoading(false)
    }
    const loadAccounts = async () => {
      if (!silent) setAccountsLoading(true)
      setAccountsError("")
      const accountRes = await safePartnerRequest(() => fetchPartnerAccounts(), t("partnerExtract.errors.loadPartnerMembersFailed"))
      if (accountRes.success) {
        setAccounts(accountRes.data ?? [])
        setAccountsLoaded(true)
      } else {
        setAccountsError(accountRes.error?.message || t("partnerExtract.errors.loadPartnerMembersFailed"))
      }
      if (!silent) setAccountsLoading(false)
    }
    const loadMeetings = async () => {
      if (!silent) setMeetingsLoading(true)
      setMeetingsError("")
      const meetingRes = await safePartnerRequest(() => fetchPartnerMeetings("active"), t("partnerExtract.errors.loadVideoStatsFailed"))
      if (meetingRes.success) {
        setActiveMeetings(meetingRes.data?.summary.active ?? 0)
        setMeetingsLoaded(true)
      } else {
        setMeetingsError(meetingRes.error?.message || t("partnerExtract.errors.loadVideoStatsFailed"))
      }
      if (!silent) setMeetingsLoading(false)
    }
    void loadProfile()
    void loadTickets()
    void loadAccounts()
    void loadMeetings()
  }, [t])

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => { void load() })
    return () => window.cancelAnimationFrame(frame)
  }, [load])

  useEffect(() => {
    const refresh = () => {
      if (document.visibilityState === "visible") void load(true)
    }
    const timer = window.setInterval(refresh, 10_000)
    document.addEventListener("visibilitychange", refresh)
    window.addEventListener("focus", refresh)
    return () => {
      window.clearInterval(timer)
      document.removeEventListener("visibilitychange", refresh)
      window.removeEventListener("focus", refresh)
    }
  }, [load])

  const overview = useMemo(() => {
    const now = snapshotTime
    const riskBoundary = now + 24 * 60 * 60 * 1000
    const sevenDaysAgo = now - 7 * 24 * 60 * 60 * 1000
    const openTickets = tickets.filter((item) => !isPartnerCollaborationTerminal(item.status))
    const isAtRisk = (item: TicketSupplierCollaboration) => {
      if (!item.authorization_active || !item.authorization_ends_at) return false
      const endsAt = new Date(item.authorization_ends_at).getTime()
      return Number.isFinite(endsAt) && endsAt > now && endsAt <= riskBoundary
    }
    const queue = [...openTickets].sort((left, right) => {
      const leftPriority = left.status === "invited" ? 0 : isAtRisk(left) ? 1 : 2
      const rightPriority = right.status === "invited" ? 0 : isAtRisk(right) ? 1 : 2
      if (leftPriority !== rightPriority) return leftPriority - rightPriority
      const leftTime = new Date(left.ticket_updated_at || left.invited_at).getTime() || 0
      const rightTime = new Date(right.ticket_updated_at || right.invited_at).getTime() || 0
      return rightTime - leftTime
    })

    return {
      openTickets,
      queue,
      pending: openTickets.filter((item) => item.status === "invited").length,
      processing: openTickets.filter((item) => item.status === "accepted" || item.status === "processing").length,
      unassigned: openTickets.filter((item) => !item.partner_account_id).length,
      atRisk: openTickets.filter(isAtRisk).length,
      myProcessing: openTickets.filter((item) => (
        (item.status === "accepted" || item.status === "processing") && item.partner_account_id === profile?.account_id
      )).length,
      resolved: tickets.filter((item) => item.status === "resolved").length,
      resolvedThisWeek: tickets.filter((item) => {
        if (item.status !== "resolved" || !item.resolved_at) return false
        const resolvedAt = new Date(item.resolved_at).getTime()
        return Number.isFinite(resolvedAt) && resolvedAt >= sevenDaysAgo
      }).length,
      isAtRisk,
    }
  }, [profile?.account_id, snapshotTime, tickets])

  const activeAccounts = useMemo(() => accounts.filter((account) => account.status === 0), [accounts])
  const teamLoad = useMemo(() => activeAccounts.map((account) => ({
    account,
    open: overview.openTickets.filter((item) => item.partner_account_id === account.id).length,
    pending: overview.openTickets.filter((item) => item.partner_account_id === account.id && item.status === "invited").length,
  })).sort((left, right) => right.open - left.open || left.account.id - right.account.id), [activeAccounts, overview.openTickets])
  const assignmentOptions = useMemo(() => [...teamLoad].sort((left, right) => left.open - right.open || left.account.id - right.account.id).map(({ account, open }) => ({
    value: String(account.id),
    label: t("partnerExtract.fallback.accountCurrentOpen", {
      name: account.display_name || account.username || t("partnerExtract.fallback.partnerMemberWithId", { id: account.id }),
      count: open,
    }),
  })), [teamLoad, t])

  const accept = useCallback(async (item: TicketSupplierCollaboration) => {
    setBusyID(item.id)
    const res = await acceptPartnerTicket(item.id)
    if (res.success && res.data) {
      setTickets((current) => current.map((entry) => entry.id === item.id ? res.data : entry))
      toast.success(t("partnerExtract.toasts.collaborationAccepted"))
      router.push(partnerTicketDetailHref(item.id))
    } else {
      toast.error(res.error?.message || t("partnerExtract.errors.acceptFailed"))
    }
    setBusyID(null)
  }, [router, t])

  const submitAssignment = useCallback(async () => {
    if (!assigning || !assigneeID) return
    setBusyID(assigning.id)
    const res = await assignPartnerTicket(assigning.id, Number(assigneeID))
    if (res.success && res.data) {
      setTickets((current) => current.map((entry) => entry.id === assigning.id ? res.data : entry))
      setAssigning(null)
      setAssigneeID("")
      toast.success(t("partnerExtract.toasts.ownerUpdated"))
    } else {
      toast.error(res.error?.message || t("partnerExtract.errors.assignFailed"))
    }
    setBusyID(null)
  }, [assigneeID, assigning, t])

  const loading = profileLoading || ticketsLoading || accountsLoading || meetingsLoading
  const ticketsInitialLoading = ticketsLoading && !ticketsLoaded
  const profileInitialLoading = profileLoading && !profileLoaded
  const accountsInitialLoading = accountsLoading && !accountsLoaded
  const meetingsInitialLoading = meetingsLoading && !meetingsLoaded
  const metricsInitialLoading = ticketsInitialLoading || profileInitialLoading
  const metricsError = !ticketsLoaded ? ticketsError : !profileLoaded ? profileError : ""
  const sidebarSummaryInitialLoading = ticketsInitialLoading || accountsInitialLoading || meetingsInitialLoading
  const sidebarSummaryError = [
    !ticketsLoaded ? ticketsError : "",
    !accountsLoaded ? accountsError : "",
    !meetingsLoaded ? meetingsError : "",
  ].filter(Boolean).join("；")

  return (
    <div className="rhd-railops-partner-page space-y-4">
      <PageHeader
        title={t("partnerExtract.overview.title")}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            {profileLoaded ? (
              <Badge variant="outline" className={profile?.can_manage_team ? "border-primary/20 bg-primary/10 text-primary" : "border-border bg-muted text-muted-foreground"}>
                {profile?.can_manage_team ? t("partnerExtract.roles.admin") : t("partnerExtract.roles.engineer")}
              </Badge>
            ) : null}
            <Button variant="outline" render={<Link href="/partner/conversations" />}><MessageSquareTextIcon className="size-4" />{t("partnerExtract.nav.conversations")}</Button>
            {profile?.can_manage_team ? <Button variant="outline" render={<Link href="/partner/people" />}><UsersIcon className="size-4" />{t("partnerExtract.nav.people")}</Button> : null}
            <Button variant="outline" size="icon" aria-label={t("partnerExtract.overview.refreshAria")} title={t("partnerExtract.overview.refreshAria")} onClick={() => void load()} disabled={loading}>
              <RefreshCwIcon className={cn("size-4", loading && "animate-spin")} />
            </Button>
          </div>
        }
      />

      {metricsInitialLoading ? (
        <ModuleLoading variant="metrics" count={4} label={t("partnerExtract.loading.partnerStats")} />
      ) : metricsError ? (
        <ErrorState title={t("partnerExtract.errors.collaborationStatsFailed")} description={metricsError} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} />
      ) : (
        <section className="rhd-railops-partner-overview-metrics grid overflow-hidden rounded-md border border-border bg-card sm:grid-cols-2 xl:grid-cols-4" aria-label={t("partnerExtract.overview.statsAria")}>
          {([
            [t("partnerExtract.metrics.pending"), overview.pending, overview.unassigned > 0 ? t("partnerExtract.metrics.unassignedOwners", { count: overview.unassigned }) : t("partnerExtract.metrics.waitingPartnerResponse"), "text-foreground", <Clock3Icon key="pending" className="size-4" />],
            [t("partnerExtract.metrics.processing"), overview.processing, t("partnerExtract.metrics.myOwnerCount", { count: overview.myProcessing }), "text-primary", <TicketCheckIcon key="processing" className="size-4" />],
            [t("partnerExtract.metrics.authorizationRisk"), overview.atRisk, t("partnerExtract.metrics.expiresWithin24h"), "text-destructive", <AlertTriangleIcon key="risk" className="size-4" />],
            [t("partnerExtract.metrics.resolved7d"), overview.resolvedThisWeek, t("partnerExtract.metrics.totalResolved", { count: overview.resolved }), "text-foreground", <CalendarCheck2Icon key="resolved" className="size-4" />],
          ] as const).map(([label, value, detail, tone, icon]) => (
            <Link key={label} href="/partner/tickets" className="rhd-railops-partner-overview-metric flex min-h-24 items-center justify-between gap-4 border-b border-border px-4 py-3 last:border-b-0 hover:bg-muted/50 sm:border-r sm:[&:nth-child(2n)]:border-r-0 xl:border-b-0 xl:[&:nth-child(2n)]:border-r xl:last:border-r-0">
              <div>
                <span className="text-sm font-medium text-muted-foreground">{label}</span>
                <strong className={cn("mt-1 block text-2xl tabular-nums", tone)}>{value}</strong>
                <span className="mt-1 block text-xs text-muted-foreground">{detail}</span>
              </div>
              <span className={cn("grid size-9 shrink-0 place-items-center rounded-md bg-muted", tone)}>{icon}</span>
            </Link>
          ))}
        </section>
      )}

      <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_320px]">
        <div
          className={`min-w-0 ${partnerPanelClass}`}
          data-testid="partner-overview-queue"
        >
          <div className={`flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between ${partnerPanelHeaderClass}`}>
            <div>
              <h2 className="text-sm font-semibold text-foreground">{profile?.can_manage_team ? t("partnerExtract.overview.teamTodo") : t("partnerExtract.overview.myTodo")}</h2>
            </div>
            <div className="flex items-center gap-2">
              <Button size="sm" variant="outline" render={<Link href="/partner/tickets" />}>{t("partnerExtract.nav.allTickets")}</Button>
            </div>
          </div>
          {ticketsInitialLoading ? (
            <ModuleLoading className="p-4" variant="list" count={5} label={t("partnerExtract.loading.todo")} />
          ) : ticketsError && !ticketsLoaded ? (
            <ErrorState title={t("partnerExtract.errors.todoFailed")} description={ticketsError} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} className="rounded-none border-0" />
          ) : overview.queue.length === 0 ? (
            <EmptyState title={t("partnerExtract.empty.noPendingTickets")} className="rounded-none border-0" />
          ) : (
            <>
              {ticketsError ? (
                <div className="border-b border-border bg-destructive/5 px-4 py-2 text-xs text-destructive">
                  {ticketsError}
                </div>
              ) : null}
              <div className={partnerDividerClass}>
                {overview.queue.slice(0, 8).map((ticket) => {
                  const status = collaborationStatus(ticket.status)
                  const atRisk = overview.isAtRisk(ticket)
                  const canManageOrOwn = Boolean(profile?.can_manage_team || ticket.partner_account_id === profile?.account_id)
                  return (
                    <div
                      key={ticket.id}
                      className="grid gap-3 p-4 lg:grid-cols-[minmax(0,1fr)_150px_auto] lg:items-center"
                      data-testid={`partner-overview-ticket-${ticket.id}`}
                    >
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <Link href={partnerTicketDetailHref(ticket.id)} className="font-semibold text-foreground hover:text-primary">
                            {ticket.ticket_no || t("partnerExtract.fallback.collaborationWithId", { id: ticket.id })}
                          </Link>
                          <Badge variant="outline" className={status.className}>{status.label}</Badge>
                          {atRisk ? <Badge variant="outline" className="border-destructive/20 bg-destructive/10 text-destructive">{t("partnerExtract.badges.authorizationExpiring")}</Badge> : null}
                        </div>
                        <div className="mt-1 truncate text-sm text-muted-foreground">{collaborationScopeText(ticket)} · {ticket.ticket_title || ticket.reason || t("partnerExtract.fallback.collaborationTask")}</div>
                        <div className="mt-1 text-xs text-muted-foreground">{t("partnerExtract.detail.updatedAt", { time: dateText(ticket.ticket_updated_at || ticket.invited_at) })} · {t("partnerExtract.detail.authorizedUntil", { time: dateText(ticket.authorization_ends_at) })}</div>
                      </div>
                      <div className="min-w-0">
                        <div className="text-xs text-muted-foreground">{t("partnerExtract.fields.owner")}</div>
                        <div className="mt-1 truncate text-sm font-medium text-foreground">{ticket.partner_account_name || t("partnerExtract.fallback.unassigned")}</div>
                        <div className="mt-1 text-xs text-muted-foreground">{t("partnerExtract.detail.participantCount", { count: ticket.participant_count || 0 })}</div>
                      </div>
                      <div className="flex flex-wrap items-center gap-2 lg:justify-end">
                        {ticket.status === "invited" && ticket.authorization_active && canManageOrOwn ? (
                          <Button size="sm" disabled={busyID === ticket.id} onClick={() => void accept(ticket)}>{t("partnerExtract.actions.acceptAndHandle")}</Button>
                        ) : (
                          <Button size="sm" variant="outline" render={<Link href={partnerTicketDetailHref(ticket.id)} />}>
                            <MessageSquareTextIcon className="size-3.5" />
                            {profile?.can_manage_team && ticket.partner_account_id !== profile.account_id ? t("partnerExtract.actions.joinHandling") : t("partnerExtract.actions.continueHandling")}
                          </Button>
                        )}
                        {profile?.can_manage_team && isPartnerCollaborationWorkable(ticket) ? (
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={accountsInitialLoading}
                            onClick={() => {
                              setAssigning(ticket)
                              setAssigneeID(String(ticket.partner_account_id || ""))
                            }}
                          >
                            <UsersIcon className="size-3.5" />
                            {t("partnerExtract.actions.assign")}
                          </Button>
                        ) : null}
                      </div>
                    </div>
                  )
                })}
              </div>
            </>
          )}
        </div>

        <aside className="space-y-3">
          {profileInitialLoading ? (
            <ModuleLoading variant="list" count={4} label={t("partnerExtract.loading.sidebar")} />
          ) : profileError && !profileLoaded ? (
            <ErrorState title={t("partnerExtract.errors.identityFailed")} description={profileError} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} />
          ) : profile?.can_manage_team ? (
            <div
              className={partnerPanelClass}
              data-testid="partner-team-load"
            >
              <div className={`flex items-start justify-between gap-3 ${partnerPanelHeaderClass}`}>
                <div>
                  <h2 className="text-sm font-semibold text-foreground">{t("partnerExtract.overview.teamLoad")}</h2>
                </div>
                <span className="text-xs text-muted-foreground">{t("partnerExtract.metrics.enabledPeople", { count: activeAccounts.length })}</span>
              </div>
              {accountsInitialLoading ? (
                <ModuleLoading className="p-4" variant="list" count={4} label={t("partnerExtract.loading.teamLoad")} />
              ) : accountsError && !accountsLoaded ? (
                <ErrorState title={t("partnerExtract.errors.teamLoadFailed")} description={accountsError} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} className="rounded-none border-0" />
              ) : teamLoad.length === 0 ? (
                <div className="px-4 py-6 text-center text-sm text-muted-foreground">{t("partnerExtract.empty.noAssignableMembers")}</div>
              ) : (
                <>
                  {accountsError ? (
                    <div className="border-b border-border bg-destructive/5 px-4 py-2 text-xs text-destructive">
                      {accountsError}
                    </div>
                  ) : null}
                  <div className={`max-h-[420px] overflow-y-auto ${partnerDividerClass}`}>
                    {teamLoad.map(({ account, open, pending: accountPending }) => (
                      <div key={account.id} className="flex items-center justify-between gap-3 px-4 py-3">
                        <div className="min-w-0">
                          <div className="flex items-center gap-2">
                            <span className="truncate text-sm font-medium text-foreground">{account.display_name || account.username}</span>
                            {account.is_current ? <span className="shrink-0 text-xs text-primary">{t("partnerExtract.common.me")}</span> : null}
                          </div>
                          <div className="mt-1 text-xs text-muted-foreground">{account.roles.includes("partner_admin") ? t("partnerExtract.roles.admin") : t("partnerExtract.roles.engineer")}{accountPending > 0 ? ` · ${t("partnerExtract.metrics.pendingTickets", { count: accountPending })}` : ""}</div>
                        </div>
                        <div className="shrink-0 text-right">
                          <div className="text-lg font-semibold tabular-nums text-foreground">{open}</div>
                          <div className="text-xs text-muted-foreground">{t("partnerExtract.metrics.unfinished")}</div>
                        </div>
                      </div>
                    ))}
                  </div>
                </>
              )}
            </div>
          ) : null}

          {sidebarSummaryInitialLoading ? (
            <ModuleLoading variant="list" count={3} label={t("partnerExtract.loading.summary")} />
          ) : sidebarSummaryError ? (
            <ErrorState title={t("partnerExtract.errors.summaryFailed")} description={sidebarSummaryError} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} />
          ) : (
            <div className="rounded-md border border-border bg-card p-4 shadow-sm">
              <h2 className="text-sm font-semibold text-foreground">{t("partnerExtract.overview.title")}</h2>
              <dl className={`mt-3 text-sm ${partnerDividerClass}`}>
                <div className="flex items-center justify-between gap-3 py-2.5"><dt className="text-muted-foreground">{t("partnerExtract.metrics.currentOpen")}</dt><dd className="font-medium tabular-nums text-foreground">{overview.openTickets.length}</dd></div>
                <div className="flex items-center justify-between gap-3 py-2.5"><dt className="text-muted-foreground">{t("partnerExtract.metrics.activeMeetings")}</dt><dd className="font-medium tabular-nums text-foreground">{activeMeetings}</dd></div>
                <div className="flex items-center justify-between gap-3 py-2.5"><dt className="text-muted-foreground">{t("partnerExtract.metrics.assignableMembers")}</dt><dd className="font-medium tabular-nums text-foreground">{activeAccounts.length}</dd></div>
              </dl>
              <div className="mt-3 grid grid-cols-2 gap-2 border-t border-border pt-3">
                <Button size="sm" variant="outline" render={<Link href="/partner/video" />}><VideoIcon className="size-3.5" />{t("partnerExtract.nav.video")}</Button>
                <Button size="sm" variant="outline" render={<Link href="/partner/conversations" />}><MessageSquareTextIcon className="size-3.5" />{t("partnerExtract.nav.allConversations")}</Button>
              </div>
            </div>
          )}
        </aside>
      </section>

      <StandardModal
        open={Boolean(assigning)}
        onCancel={() => setAssigning(null)}
        title={t("partnerExtract.modals.assignOwner")}
        width={448}
        footer={
          <>
            <Button variant="outline" onClick={() => setAssigning(null)}>{t("partnerExtract.common.cancel")}</Button>
            <Button disabled={!assigneeID || busyID === assigning?.id} onClick={() => void submitAssignment()}>{t("partnerExtract.actions.confirmAssign")}</Button>
          </>
        }
      >
          <SelectField
            label={t("partnerExtract.fields.primaryEngineer")}
            style={{ marginBottom: 0 }}
            selectProps={{
              "aria-label": t("partnerExtract.fields.primaryEngineer"),
              placeholder: t("partnerExtract.placeholders.selectPartnerMember"),
              value: assigneeID,
              onChange: (value) => setAssigneeID(value ?? ""),
              options: assignmentOptions,
              style: { width: "100%" },
            }}
          />
      </StandardModal>
    </div>
  )
}

export function PartnerTicketsPage() {
  const t = useI18n()
  const [profile, setProfile] = useState<PartnerPortalProfile | null>(null)
  const [items, setItems] = useState<TicketSupplierCollaboration[]>([])
  const [accounts, setAccounts] = useState<PartnerPortalAccount[]>([])
  const [filter, setFilter] = useState<"active" | "resolved" | "all">("active")
  const [profileLoading, setProfileLoading] = useState(true)
  const [profileLoaded, setProfileLoaded] = useState(false)
  const [profileError, setProfileError] = useState("")
  const [ticketsLoading, setTicketsLoading] = useState(true)
  const [ticketsLoaded, setTicketsLoaded] = useState(false)
  const [ticketsError, setTicketsError] = useState("")
  const [accountsLoading, setAccountsLoading] = useState(true)
  const [accountsLoaded, setAccountsLoaded] = useState(false)
  const [accountsError, setAccountsError] = useState("")
  const [busyID, setBusyID] = useState<number | null>(null)
  const [resolving, setResolving] = useState<TicketSupplierCollaboration | null>(null)
  const [resolution, setResolution] = useState("")
  const [assigning, setAssigning] = useState<TicketSupplierCollaboration | null>(null)
  const [assigneeID, setAssigneeID] = useState("")

  const load = useCallback((silent = false) => {
    const loadProfile = async () => {
      if (!silent) setProfileLoading(true)
      setProfileError("")
      const profileRes = await safePartnerRequest(() => fetchPartnerProfile(), t("partnerExtract.errors.loadPartnerProfileFailed"))
      if (profileRes.success) {
        setProfile(profileRes.data ?? null)
        setProfileLoaded(true)
      } else {
        setProfileError(profileRes.error?.message || t("partnerExtract.errors.loadPartnerProfileFailed"))
      }
      if (!silent) setProfileLoading(false)
    }
    const loadTickets = async () => {
      if (!silent) setTicketsLoading(true)
      setTicketsError("")
      const ticketRes = await safePartnerRequest(() => fetchPartnerTickets(), t("partnerExtract.errors.loadPartnerTicketsFailed"))
      if (ticketRes.success) {
        setItems(ticketRes.data ?? [])
        setTicketsLoaded(true)
      } else {
        setTicketsError(ticketRes.error?.message || t("partnerExtract.errors.loadPartnerTicketsFailed"))
      }
      if (!silent) setTicketsLoading(false)
    }
    const loadAccounts = async () => {
      if (!silent) setAccountsLoading(true)
      setAccountsError("")
      const accountRes = await safePartnerRequest(() => fetchPartnerAccounts(), t("partnerExtract.errors.loadPartnerMembersFailed"))
      if (accountRes.success) {
        setAccounts(accountRes.data ?? [])
        setAccountsLoaded(true)
      } else {
        setAccountsError(accountRes.error?.message || t("partnerExtract.errors.loadPartnerMembersFailed"))
      }
      if (!silent) setAccountsLoading(false)
    }
    void loadProfile()
    void loadTickets()
    void loadAccounts()
  }, [t])

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => { void load() })
    return () => window.cancelAnimationFrame(frame)
  }, [load])

  useEffect(() => {
    const refresh = () => {
      if (document.visibilityState === "visible") void load(true)
    }
    const timer = window.setInterval(refresh, 10_000)
    document.addEventListener("visibilitychange", refresh)
    window.addEventListener("focus", refresh)
    return () => {
      window.clearInterval(timer)
      document.removeEventListener("visibilitychange", refresh)
      window.removeEventListener("focus", refresh)
    }
  }, [load])

  const visibleItems = useMemo(() => items.filter((item) => {
    if (filter === "resolved") return isPartnerCollaborationTerminal(item.status)
    if (filter === "active") return !isPartnerCollaborationTerminal(item.status)
    return true
  }), [filter, items])

  const invited = items.filter((item) => item.status === "invited").length
  const processing = items.filter((item) => item.status === "accepted" || item.status === "processing").length
  const resolved = items.filter((item) => isPartnerCollaborationTerminal(item.status)).length
  const hasProductCollaborations = visibleItems.some((item) => item.product_id > 0)
  const assignableAccountOptions = useMemo(
    () => partnerAccountSelectOptions(accounts.filter((account) => account.status === 0)),
    [accounts],
  )

  const accept = useCallback(async (item: TicketSupplierCollaboration) => {
    setBusyID(item.id)
    const res = await acceptPartnerTicket(item.id)
    if (res.success && res.data) {
      setItems((current) => current.map((entry) => entry.id === item.id ? res.data : entry))
      toast.success(t("partnerExtract.toasts.ticketAccepted"))
    } else {
      toast.error(res.error?.message || t("partnerExtract.errors.acceptFailed"))
    }
    setBusyID(null)
  }, [t])

  const submitResolution = useCallback(async () => {
    if (!resolving || !resolution.trim()) return
    setBusyID(resolving.id)
    const res = await resolvePartnerTicket(resolving.id, resolution.trim())
    if (res.success && res.data) {
      setItems((current) => current.map((entry) => entry.id === resolving.id ? res.data : entry))
      setResolving(null)
      setResolution("")
      toast.success(t("partnerExtract.toasts.resolutionSubmitted"))
    } else {
      toast.error(res.error?.message || t("partnerExtract.errors.submitResolutionFailed"))
    }
    setBusyID(null)
  }, [resolution, resolving, t])

  const submitAssignment = useCallback(async () => {
    if (!assigning || !assigneeID) return
    setBusyID(assigning.id)
    const res = await assignPartnerTicket(assigning.id, Number(assigneeID))
    if (res.success && res.data) {
      setItems((current) => current.map((entry) => entry.id === assigning.id ? res.data : entry))
      setAssigning(null)
      setAssigneeID("")
      toast.success(t("partnerExtract.toasts.assignmentUpdated"))
    } else {
      toast.error(res.error?.message || t("partnerExtract.errors.assignFailed"))
    }
    setBusyID(null)
  }, [assigneeID, assigning, t])

  const loading = profileLoading || ticketsLoading || accountsLoading
  const profileInitialLoading = profileLoading && !profileLoaded
  const ticketsInitialLoading = ticketsLoading && !ticketsLoaded
  const accountsInitialLoading = accountsLoading && !accountsLoaded

  return (
    <div className="rhd-railops-partner-page space-y-4">
      <PageHeader
        title={t("partnerExtract.tickets.title")}
        actions={
          <Button variant="outline" onClick={() => void load()} disabled={loading}>
            <RefreshCwIcon className={cn("size-4", loading && "animate-spin")} />
            {t("partnerExtract.common.refresh")}
          </Button>
        }
      />

      {ticketsInitialLoading ? (
        <ModuleLoading variant="metrics" count={3} label={t("partnerExtract.loading.ticketStats")} />
      ) : ticketsError && !ticketsLoaded ? (
        <ErrorState title={t("partnerExtract.errors.ticketStatsFailed")} description={ticketsError} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} />
      ) : (
        <section className="grid gap-3 md:grid-cols-3">
          <SummaryMetric label={t("partnerExtract.metrics.pending")} value={invited} icon={<Clock3Icon className="size-4" />} tone="amber" />
          <SummaryMetric label={t("partnerExtract.metrics.processing")} value={processing} icon={<TicketCheckIcon className="size-4" />} tone="blue" />
          <SummaryMetric label={t("partnerExtract.metrics.resolved")} value={resolved} icon={<CheckCircle2Icon className="size-4" />} tone="green" />
        </section>
      )}

      <section className={partnerPanelClass}>
        <div className={`flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between ${partnerPanelHeaderClass}`}>
          <div className="flex flex-wrap gap-2">
            {([
              ["active", t("partnerExtract.filters.active")],
              ["resolved", t("partnerExtract.filters.resolved")],
              ["all", t("partnerExtract.filters.all")],
            ] as const).map(([value, label]) => (
              <button
                key={value}
                type="button"
                className={cn(
                  partnerFilterBaseClass,
                  filter === value
                    ? partnerFilterActiveClass
                    : partnerFilterIdleClass,
                )}
                onClick={() => setFilter(value)}
              >
                {label}
              </button>
            ))}
          </div>
          <span className="text-xs text-muted-foreground">{ticketsInitialLoading ? t("partnerExtract.common.loading") : t("partnerExtract.common.totalItems", { count: visibleItems.length })}</span>
        </div>

        {ticketsInitialLoading ? (
          <ModuleLoading className="p-4" variant="table" count={6} label={t("partnerExtract.loading.ticketList")} />
        ) : ticketsError && !ticketsLoaded ? (
          <ErrorState title={t("partnerExtract.errors.ticketsFailed")} description={ticketsError} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} className="rounded-none border-0" />
        ) : visibleItems.length === 0 ? (
          <EmptyState title={t("partnerExtract.empty.noTickets")} className="rounded-none border-0" />
        ) : (
          <>
            {ticketsError || profileError || accountsError ? (
              <div className="border-b border-border bg-destructive/5 px-4 py-2 text-xs text-destructive">
                {[ticketsError, profileError, accountsError].filter(Boolean).join("；")}
              </div>
            ) : null}
            <div className="overflow-x-auto">
            <table className="rhd-railops-partner-table w-full min-w-[1180px] border-separate border-spacing-0 text-sm">
              <thead>
                <tr className={partnerTableHeadClass}>
                  <th className="border-b border-border px-4 py-3">{t("partnerExtract.table.ticket")}</th>
                  <th className="border-b border-border px-4 py-3">{t(hasProductCollaborations ? "partnerExtract.table.productModule" : "partnerExtract.table.collaborationScope")}</th>
                  <th className="border-b border-border px-4 py-3">{t("partnerExtract.table.reason")}</th>
                  <th className="border-b border-border px-4 py-3">{t("partnerExtract.table.ticketStatus")}</th>
                  <th className="border-b border-border px-4 py-3">{t("partnerExtract.table.collaborationStatus")}</th>
                  <th className="border-b border-border px-4 py-3">{t("partnerExtract.table.owner")}</th>
                  <th className="border-b border-border px-4 py-3">{t("partnerExtract.table.recentActivity")}</th>
                  <th className="border-b border-border px-4 py-3 text-right">{t("partnerExtract.table.actions")}</th>
                </tr>
              </thead>
              <tbody>
                {visibleItems.map((item) => {
                  const status = collaborationStatus(item.status)
                  const workOrderStatus = ticketStatus(item.ticket_status)
                  const canManageOrOwn = !profileInitialLoading && Boolean(profile?.can_manage_team || item.partner_account_id === profile?.account_id)
                  const canOperate = isPartnerCollaborationWorkable(item)
                  return (
                    <tr key={item.id} className="align-top text-foreground hover:bg-muted/50">
                      <td className={partnerTableCellClass}>
                        <Link href={partnerTicketDetailHref(item.id)} className="font-semibold text-foreground hover:text-primary">
                          {item.ticket_no || t("partnerExtract.fallback.collaborationWithId", { id: item.id })}
                        </Link>
                        <div className="mt-1 max-w-[260px] text-xs text-muted-foreground">{item.ticket_title || item.symptom_summary || "-"}</div>
                      </td>
                      <td className={partnerTableCellClass}>
                        <div className="font-medium">{collaborationScopeText(item)}</div>
                        <div className="mt-1 text-xs text-muted-foreground">{item.fault_code || t("partnerExtract.fallback.noFaultCode")}</div>
                      </td>
                      <td className={partnerTableCellClass}>
                        <div className="max-w-[280px] text-sm">{item.reason || "-"}</div>
                        {item.resolution ? <div className="mt-1 max-w-[280px] text-xs text-muted-foreground">{t("partnerExtract.detail.resolutionPrefix", { value: item.resolution })}</div> : null}
                      </td>
                      <td className={partnerTableCellClass}>
                        <Badge variant="outline" className={workOrderStatus.className}>{workOrderStatus.label}</Badge>
                      </td>
                      <td className={partnerTableCellClass}>
                        <Badge variant="outline" className={status.className}>{status.label}</Badge>
                      </td>
                      <td className={`${partnerTableCellClass} text-sm`}>
                        <div>{item.partner_account_name || t("partnerExtract.fallback.unassigned")}</div>
                        <div className="mt-1 text-xs text-muted-foreground">{t("partnerExtract.detail.participantCount", { count: item.participant_count || 0 })}</div>
                      </td>
                      <td className={`${partnerTableCellClass} text-xs`}>
                        <div>{dateText(item.ticket_updated_at || item.invited_at)}</div>
                        <div className="mt-1 text-muted-foreground">{t("partnerExtract.detail.authorizedUntil", { time: dateText(item.authorization_ends_at) })}</div>
                      </td>
                      <td className={partnerTableCellClass}>
                        <div className="flex flex-wrap justify-end gap-2">
                          <Button size="sm" variant="outline" render={<Link href={partnerTicketDetailHref(item.id)} />}>
                            <MessageSquareTextIcon className="size-3.5" />
                            {t("partnerExtract.actions.conversation")}
                          </Button>
                          {item.status === "invited" && item.authorization_active && canManageOrOwn ? (
                            <Button size="sm" disabled={busyID === item.id} onClick={() => void accept(item)}>{t("partnerExtract.actions.accept")}</Button>
                          ) : null}
                          {canOperate ? (
                            <>
                              {profile?.can_manage_team ? (
                                <Button
                                  size="sm"
                                  variant="outline"
                                  disabled={accountsInitialLoading}
                                  onClick={() => {
                                    setAssigning(item)
                                    setAssigneeID(String(item.partner_account_id || ""))
                                  }}
                                >
                                  {t("partnerExtract.actions.assignWork")}
                                </Button>
                              ) : null}
                              <Button size="sm" variant="outline" render={<Link href="/partner/video" />}>
                                <VideoIcon className="size-3.5" />
                                {t("partnerExtract.actions.video")}
                              </Button>
                              {canManageOrOwn && item.status !== "invited" ? (
                                <Button
                                  size="sm"
                                  variant="outline"
                                  disabled={busyID === item.id}
                                  onClick={() => {
                                    setResolving(item)
                                    setResolution("")
                                  }}
                                >
                                  {t("partnerExtract.actions.resolution")}
                                </Button>
                              ) : null}
                            </>
                          ) : null}
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
            </div>
          </>
        )}
      </section>

      <StandardModal
        open={Boolean(resolving)}
        onCancel={() => setResolving(null)}
        title={t("partnerExtract.modals.submitResolution")}
        width={512}
        footer={
          <>
            <Button variant="outline" onClick={() => setResolving(null)}>{t("partnerExtract.common.cancel")}</Button>
            <Button disabled={!resolution.trim() || busyID === resolving?.id} onClick={() => void submitResolution()}>{t("partnerExtract.actions.submitResolution")}</Button>
          </>
        }
      >
          <Textarea
            aria-label={t("partnerExtract.placeholders.resolution")}
            className="min-h-32"
            value={resolution}
            onChange={(event) => setResolution(event.target.value)}
            placeholder={t("partnerExtract.placeholders.resolution")}
          />
      </StandardModal>

      <StandardModal
        open={Boolean(assigning)}
        onCancel={() => setAssigning(null)}
        title={t("partnerExtract.modals.setOwner")}
        width={448}
        footer={
          <>
            <Button variant="outline" onClick={() => setAssigning(null)}>{t("partnerExtract.common.cancel")}</Button>
            <Button disabled={!assigneeID || busyID === assigning?.id} onClick={() => void submitAssignment()}>{t("partnerExtract.common.confirm")}</Button>
          </>
        }
      >
          <SelectField
            label={t("partnerExtract.fields.primaryEngineer")}
            style={{ marginBottom: 0 }}
            selectProps={{
              "aria-label": t("partnerExtract.fields.primaryEngineer"),
              placeholder: t("partnerExtract.placeholders.selectPartnerEngineer"),
              value: assigneeID,
              onChange: (value) => setAssigneeID(value ?? ""),
              options: assignableAccountOptions,
              style: { width: "100%" },
            }}
          />
      </StandardModal>
    </div>
  )
}

export function PartnerConversationsPage() {
  const t = useI18n()
  const router = useRouter()
  const searchParams = useSearchParams()
  const requestedCollaborationID = Number(searchParams?.get("collaboration_id") || searchParams?.get("collaborationId") || 0)
  const [items, setItems] = useState<TicketSupplierCollaboration[]>([])
  const [query, setQuery] = useState("")
  const [filter, setFilter] = useState<"active" | "waiting" | "history" | "all">("active")
  const [selectedCollaborationID, setSelectedCollaborationID] = useState<number | null>(() => requestedCollaborationID > 0 ? requestedCollaborationID : null)
  const [loading, setLoading] = useState(true)
  const [itemsLoaded, setItemsLoaded] = useState(false)
  const [error, setError] = useState("")

  const load = useCallback(async (silent = false) => {
    if (!silent) setLoading(true)
    setError("")
    const res = await safePartnerRequest(() => fetchPartnerConversations(), t("partnerExtract.errors.loadConversationsFailed"))
    if (res.success) {
      setItems(res.data ?? [])
      setItemsLoaded(true)
    } else {
      setError(res.error?.message || t("partnerExtract.errors.loadConversationsFailed"))
    }
    if (!silent) setLoading(false)
  }, [t])

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => { void load() })
    return () => window.cancelAnimationFrame(frame)
  }, [load])

  useEffect(() => {
    const refresh = () => {
      if (document.visibilityState === "visible") void load(true)
    }
    const timer = window.setInterval(refresh, 10_000)
    document.addEventListener("visibilitychange", refresh)
    window.addEventListener("focus", refresh)
    return () => {
      window.clearInterval(timer)
      document.removeEventListener("visibilitychange", refresh)
      window.removeEventListener("focus", refresh)
    }
  }, [load])

  const visibleItems = useMemo(() => {
    const keyword = query.trim().toLowerCase()
    return items.filter((item) => {
      if (filter === "waiting" && item.status !== "invited") return false
      if (filter === "active" && isPartnerCollaborationTerminal(item.status)) return false
      if (filter === "history" && !isPartnerCollaborationTerminal(item.status)) return false
      if (!keyword) return true
      return [
        item.ticket_no,
        item.ticket_title,
        item.product_module_name,
        item.fault_code,
        item.reason,
        item.symptom_summary,
        item.partner_account_name,
      ].some((value) => String(value ?? "").toLowerCase().includes(keyword))
    })
  }, [filter, items, query])

  useEffect(() => {
    if (requestedCollaborationID > 0) {
      if (requestedCollaborationID === selectedCollaborationID) return
      const frame = window.requestAnimationFrame(() => setSelectedCollaborationID(requestedCollaborationID))
      return () => window.cancelAnimationFrame(frame)
    }
    if (!itemsLoaded) return
    const nextID = selectedCollaborationID && visibleItems.some((item) => item.id === selectedCollaborationID)
      ? selectedCollaborationID
      : visibleItems[0]?.id ?? null
    if (nextID === selectedCollaborationID) return
    const frame = window.requestAnimationFrame(() => setSelectedCollaborationID(nextID))
    return () => window.cancelAnimationFrame(frame)
  }, [itemsLoaded, requestedCollaborationID, selectedCollaborationID, visibleItems])

  const selectCollaboration = useCallback((id: number) => {
    setSelectedCollaborationID(id)
    router.replace(`/partner/conversations?collaboration_id=${encodeURIComponent(String(id))}`)
  }, [router])

  const selectedItem = items.find((item) => item.id === selectedCollaborationID) ?? null
  const active = items.filter((item) => !isPartnerCollaborationTerminal(item.status)).length
  const waiting = items.filter((item) => item.status === "invited").length
  const history = items.filter((item) => isPartnerCollaborationTerminal(item.status)).length
  const itemsInitialLoading = loading && !itemsLoaded

  return (
    <div className="rhd-railops-partner-page rhd-railops-partner-conversations-page space-y-4">
      <PageHeader
        title={t("partnerExtract.conversations.title")}
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" render={<Link href="/partner/tickets" />}>
              <TicketCheckIcon className="size-4" />
              {t("partnerExtract.nav.tickets")}
            </Button>
            <Button variant="outline" onClick={() => void load()} disabled={loading}>
              <RefreshCwIcon className={cn("size-4", loading && "animate-spin")} />
              {t("partnerExtract.common.refresh")}
            </Button>
          </div>
        }
      />

      {itemsInitialLoading ? (
        <ModuleLoading variant="metrics" count={3} label={t("partnerExtract.loading.conversationStats")} />
      ) : error && !itemsLoaded ? (
        <ErrorState title={t("partnerExtract.errors.conversationStatsFailed")} description={error} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} />
      ) : (
        <section className="grid gap-3 md:grid-cols-3">
          <SummaryMetric label={t("partnerExtract.metrics.activeConversations")} value={active} icon={<MessageSquareTextIcon className="size-4" />} tone="slate" />
          <SummaryMetric label={t("partnerExtract.metrics.pending")} value={waiting} icon={<Clock3Icon className="size-4" />} tone="amber" />
          <SummaryMetric label={t("partnerExtract.metrics.historyConversations")} value={history} icon={<FileTextIcon className="size-4" />} tone="green" />
        </section>
      )}

      <section className="rhd-railops-partner-workspace grid gap-4 lg:grid-cols-[320px_minmax(0,1fr)] xl:h-[calc(100vh-190px)] xl:min-h-[640px] xl:max-h-[920px] xl:grid-cols-[340px_minmax(0,1fr)]">
        <aside className={`min-h-[560px] min-w-0 xl:h-full ${partnerPanelClass}`}>
          <div className={`space-y-3 ${partnerPanelHeaderClass}`}>
            <div>
              <h2 className="text-sm font-semibold text-foreground">{t("partnerExtract.conversations.collaborationConversations")}</h2>
              <p className="mt-1 text-xs text-muted-foreground">{itemsInitialLoading ? t("partnerExtract.common.loading") : t("partnerExtract.common.totalItems", { count: visibleItems.length })}</p>
            </div>
            <div className="grid grid-cols-4 gap-1 rounded-md bg-muted p-1" aria-label={t("partnerExtract.conversations.filterAria")}>
              {([
                ["active", t("partnerExtract.filters.inProgress")],
                ["waiting", t("partnerExtract.filters.pending")],
                ["history", t("partnerExtract.filters.history")],
                ["all", t("partnerExtract.filters.all")],
              ] as const).map(([value, label]) => (
                <button
                  key={value}
                  type="button"
                  className={cn(
                    "h-8 rounded-sm px-2 text-xs font-medium transition",
                    filter === value ? "bg-card text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground",
                  )}
                  onClick={() => setFilter(value)}
                >
                  {label}
                </button>
              ))}
            </div>
            <div>
              <SearchField
                allowClear
                aria-label={t("partnerExtract.conversations.searchAria")}
                placeholder={t("partnerExtract.conversations.searchPlaceholder")}
                value={query}
                onChange={(event) => setQuery(event.target.value)}
              />
            </div>
          </div>
          <ScrollArea className="min-h-0 p-3 xl:h-[calc(100%-146px)]">
            {itemsInitialLoading ? (
              <ModuleLoading variant="list" count={5} label={t("partnerExtract.loading.conversationList")} />
            ) : error && !itemsLoaded ? (
              <ErrorState title={t("partnerExtract.errors.conversationsFailed")} description={error} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} className="rounded-none border-0" />
            ) : visibleItems.length === 0 ? (
              <EmptyState title={t("partnerExtract.empty.noConversations")} className="rounded-none border-0" />
            ) : (
              <>
                {error ? (
                  <div className="mb-2 rounded-md border border-destructive/20 bg-destructive/5 px-3 py-2 text-xs text-destructive">
                    {error}
                  </div>
                ) : null}
                <div className="space-y-1.5 pr-3">
                  {visibleItems.map((item) => {
                    const status = collaborationStatus(item.status)
                    const selected = item.id === selectedCollaborationID
                    const title = item.ticket_title || item.reason || t("partnerExtract.fallback.authorizedConversation")
                    return (
                      <button
                        key={item.id}
                        type="button"
                        className={cn(
                          "w-full rounded-md border p-3 text-left transition",
                          selected ? "border-primary/30 bg-primary/5 shadow-sm" : "border-transparent bg-card hover:border-border hover:bg-muted/50",
                        )}
                        onClick={() => selectCollaboration(item.id)}
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <div className="truncate text-sm font-semibold text-foreground">{item.ticket_no || t("partnerExtract.fallback.collaborationWithId", { id: item.id })}</div>
                            <div className="mt-1 truncate text-xs text-muted-foreground">{collaborationScopeText(item)}</div>
                          </div>
                          <Badge variant="outline" className={status.className}>{status.label}</Badge>
                        </div>
                        <p className="mt-2 line-clamp-2 text-sm leading-5 text-muted-foreground">{title}</p>
                        <div className="mt-2 flex items-center justify-between gap-3 text-xs text-muted-foreground">
                          <span className="truncate">{item.partner_account_name || t("partnerExtract.fallback.unassignedWork")}</span>
                          <span className="shrink-0">{dateText(item.ticket_updated_at || item.invited_at)}</span>
                        </div>
                      </button>
                    )
                  })}
                </div>
              </>
            )}
          </ScrollArea>
        </aside>

        <div className="min-h-[640px] min-w-0 xl:h-full xl:overflow-y-auto">
          {selectedCollaborationID ? (
            <PartnerTicketDetailView
              key={selectedCollaborationID}
              collaborationID={selectedCollaborationID}
              backHref="/partner/conversations"
              backLabel={t("partnerExtract.nav.backToConversationList")}
              showBackLink={false}
              headerDescription={selectedItem ? `${collaborationScopeText(selectedItem)} · ${selectedItem.ticket_title || selectedItem.reason || t("partnerExtract.fallback.partnerConversation")}` : undefined}
            />
          ) : itemsInitialLoading ? (
            <ModuleLoading className="min-h-[520px]" variant="detail" count={5} label={t("partnerExtract.loading.conversationDetail")} />
          ) : error && !itemsLoaded ? (
            <ErrorState title={t("partnerExtract.errors.conversationDetailFailed")} description={error} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} />
          ) : (
            <EmptyState title={t("partnerExtract.empty.selectConversation")} />
          )}
        </div>
      </section>
    </div>
  )
}

export function PartnerVideoCollaborationPage() {
  const t = useI18n()
  const [status, setStatus] = useState<"all" | "active" | "waiting" | "ended">("all")
  const [meetingsData, setMeetingsData] = useState<MeetingListResponse | null>(null)
  const [selectedMeetingID, setSelectedMeetingID] = useState("")
  const [preferredMeetingID, setPreferredMeetingID] = useState("")
  const [preferredSelectionApplied, setPreferredSelectionApplied] = useState(false)
  const [joiningMeetingID, setJoiningMeetingID] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [meetingsLoaded, setMeetingsLoaded] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const meetingID = params.get("meeting_id") || params.get("meetingId") || ""
    setPreferredMeetingID(meetingID)
    setPreferredSelectionApplied(false)
  }, [])

  const load = useCallback(async (silent = false) => {
    if (!silent) setLoading(true)
    setError("")
    const meetingRes = await safePartnerRequest(() => fetchPartnerMeetings(status), t("partnerExtract.errors.loadPartnerVideoFailed"))
    if (meetingRes.success && meetingRes.data) {
      setMeetingsData(meetingRes.data)
      setMeetingsLoaded(true)
    } else {
      setError(meetingRes.error?.message || t("partnerExtract.errors.loadPartnerVideoFailed"))
    }
    if (!silent) setLoading(false)
  }, [status, t])

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => { void load() })
    return () => window.cancelAnimationFrame(frame)
  }, [load])

  useEffect(() => {
    const refresh = () => {
      if (document.visibilityState === "visible") void load(true)
    }
    const timer = window.setInterval(refresh, 10_000)
    document.addEventListener("visibilitychange", refresh)
    window.addEventListener("focus", refresh)
    return () => {
      window.clearInterval(timer)
      document.removeEventListener("visibilitychange", refresh)
      window.removeEventListener("focus", refresh)
    }
  }, [load])

  const meetings = useMemo(() => meetingsData?.items ?? [], [meetingsData])

  useEffect(() => {
    if (!preferredSelectionApplied && preferredMeetingID && meetings.some((meeting) => meeting.id === preferredMeetingID)) {
      setSelectedMeetingID(preferredMeetingID)
      setPreferredSelectionApplied(true)
      return
    }
    if (selectedMeetingID && meetings.some((meeting) => meeting.id === selectedMeetingID)) return
    const activeMeetingID = meetings.find((meeting) => meeting.status === "active")?.id
    setSelectedMeetingID(activeMeetingID ?? meetings[0]?.id ?? "")
  }, [
    meetings,
    preferredMeetingID,
    preferredSelectionApplied,
    selectedMeetingID,
  ])

  const joinMeeting = useCallback(async (meeting: MeetingListItem) => {
    if (!meeting.collaboration_id || joiningMeetingID) return
    setJoiningMeetingID(meeting.id)
    try {
      window.open(partnerMeetingRoomHref(meeting.collaboration_id, meeting.id), "_blank", "noopener,noreferrer")
    } catch (joinError) {
      toast.error(joinError instanceof Error ? joinError.message : t("partnerExtract.errors.enterVideoFailed"))
    } finally {
      setJoiningMeetingID(null)
    }
  }, [joiningMeetingID, t])

  const summary = meetingsData?.summary
  const selectedMeeting = meetings.find((meeting) => meeting.id === selectedMeetingID) ?? null
  const meetingsInitialLoading = loading && !meetingsLoaded

  return (
    <div className="rhd-railops-partner-page rhd-railops-partner-video-page space-y-4">
      <PageHeader
        title={t("partnerExtract.video.title")}
        actions={
          <Button variant="outline" onClick={() => void load()} disabled={loading}>
            <RefreshCwIcon className={cn("size-4", loading && "animate-spin")} />
            {t("partnerExtract.common.refresh")}
          </Button>
        }
      />

      {meetingsInitialLoading ? (
        <ModuleLoading variant="metrics" count={4} label={t("partnerExtract.loading.videoStats")} />
      ) : error && !meetingsLoaded ? (
        <ErrorState title={t("partnerExtract.errors.videoStatsFailed")} description={error} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} />
      ) : (
        <section className="grid gap-3 md:grid-cols-4">
          <SummaryMetric label={t("partnerExtract.filters.inProgress")} value={summary?.active ?? 0} icon={<VideoIcon className="size-4" />} tone="green" />
          <SummaryMetric label={t("partnerExtract.status.meeting.waiting")} value={summary?.waiting ?? 0} icon={<Clock3Icon className="size-4" />} tone="amber" />
          <SummaryMetric label={t("partnerExtract.metrics.myMeetings")} value={summary?.mine ?? 0} icon={<UsersIcon className="size-4" />} tone="blue" />
          <SummaryMetric label={t("partnerExtract.status.meeting.ended")} value={summary?.ended ?? 0} icon={<CheckCircle2Icon className="size-4" />} tone="slate" />
        </section>
      )}

      <section className="rhd-railops-partner-video-workspace grid gap-4 xl:grid-cols-[420px_minmax(0,1fr)]">
        <div className={partnerPanelClass}>
          <div className={`flex flex-col gap-3 ${partnerPanelHeaderClass}`}>
            <div className="flex items-center justify-between gap-3">
              <h2 className="text-sm font-semibold text-foreground">{t("partnerExtract.video.jitsiCollaboration")}</h2>
              <span className="text-xs text-muted-foreground">{meetingsInitialLoading ? t("partnerExtract.common.loading") : t("partnerExtract.common.totalMeetings", { count: meetings.length })}</span>
            </div>
            <div className="flex flex-wrap gap-2" aria-label={t("partnerExtract.video.filterAria")}>
              {([
                ["all", t("partnerExtract.filters.all")],
                ["active", t("partnerExtract.filters.inProgress")],
                ["waiting", t("partnerExtract.status.meeting.waiting")],
                ["ended", t("partnerExtract.status.meeting.ended")],
              ] as const).map(([value, label]) => (
                <button
                  key={value}
                  type="button"
                  className={cn(
                    partnerFilterBaseClass,
                    status === value
                      ? partnerFilterActiveClass
                      : partnerFilterIdleClass,
                  )}
                  onClick={() => setStatus(value)}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>
          {meetingsInitialLoading ? (
            <ModuleLoading className="p-4" variant="list" count={5} label={t("partnerExtract.loading.collaborationList")} />
          ) : error && meetingsData === null ? (
            <ErrorState title={t("partnerExtract.errors.collaborationFailed")} description={error} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} className="rounded-none border-0" />
          ) : meetings.length === 0 ? (
            <EmptyState title={t("partnerExtract.empty.noVideo")} className="rounded-none border-0" />
          ) : (
            <>
              {error ? (
                <div className="border-b border-border bg-destructive/5 px-4 py-2 text-xs text-destructive">
                  {error}
                </div>
              ) : null}
              <div className={`max-h-[680px] overflow-y-auto ${partnerDividerClass}`}>
                {meetings.map((meeting) => {
                  const meta = meetingStatus(meeting.status)
                  const selected = meeting.id === selectedMeetingID
                  const meetingMeta = (meeting.status === "scheduled" || meeting.status === "waiting") && meeting.scheduled_at
                    ? t("partnerExtract.detail.scheduledAt", { time: meeting.scheduled_at })
                    : `${t("partnerExtract.detail.participantCount", { count: meeting.participant_count })} · ${meetingDurationText(meeting.duration_seconds)}`
                  return (
                    <button
                      key={meeting.id}
                      type="button"
                      className={cn("block w-full p-4 text-left hover:bg-muted/50", selected && "bg-primary/5")}
                      onClick={() => setSelectedMeetingID(meeting.id)}
                    >
                      <div className="flex flex-wrap items-center gap-2">
                        <strong className="min-w-0 truncate text-sm text-foreground">{meeting.title || meeting.room_name}</strong>
                        <Badge variant="outline" className={meta.className}>{meta.label}</Badge>
                      </div>
                      <div className="mt-1 space-y-1 text-xs text-muted-foreground">
                        <div className="truncate">{meeting.ticket_no || t("partnerExtract.fallback.noLinkedTicket")} · {customerDisplayName(meeting.customer_name)}</div>
                        <div className="truncate">{meeting.device_no || t("partnerExtract.fallback.noBoundDevice")} · {meetingMeta}</div>
                      </div>
                    </button>
                  )
                })}
              </div>
            </>
          )}
        </div>

        <div className="min-w-0">
          {meetingsInitialLoading ? (
            <ModuleLoading variant="detail" count={5} label={t("partnerExtract.loading.collaborationDetail")} />
          ) : error && meetingsData === null ? (
            <ErrorState title={t("partnerExtract.errors.collaborationDetailFailed")} description={error} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} />
          ) : selectedMeeting ? (
            <div className={partnerPanelClass}>
              <div className="flex flex-col gap-4 border-b border-border p-5 lg:flex-row lg:items-start lg:justify-between">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h2 className="text-lg font-semibold text-foreground">{selectedMeeting.title || selectedMeeting.room_name}</h2>
                    <Badge variant="outline" className={meetingStatus(selectedMeeting.status).className}>
                      {meetingStatus(selectedMeeting.status).label}
                    </Badge>
                  </div>
                  <p className="mt-1 text-sm text-muted-foreground">{selectedMeeting.ticket_no || t("partnerExtract.fallback.noLinkedTicket")}</p>
                </div>
                <div className="flex flex-wrap gap-2">
                  {selectedMeeting.collaboration_id ? (
                    <Button variant="outline" render={<Link href={partnerTicketDetailHref(selectedMeeting.collaboration_id)} />}>
                      <TicketCheckIcon className="size-4" />
                      {t("partnerExtract.actions.ticketDetail")}
                    </Button>
                  ) : null}
                  {canJoinMeetingStatus(selectedMeeting.status) && selectedMeeting.collaboration_id ? (
                    <Button disabled={joiningMeetingID === selectedMeeting.id} onClick={() => void joinMeeting(selectedMeeting)}>
                      <VideoIcon className="size-4" />
                      {joiningMeetingID === selectedMeeting.id ? t("partnerExtract.actions.entering") : t("partnerExtract.actions.joinCollaboration")}
                    </Button>
                  ) : null}
                </div>
              </div>

              <div className="grid gap-x-8 gap-y-5 p-5 sm:grid-cols-2 xl:grid-cols-3">
                {([
                  [t("partnerExtract.fields.customer"), customerDisplayName(selectedMeeting.customer_name)],
                  [t("partnerExtract.fields.device"), selectedMeeting.device_no || t("partnerExtract.fallback.noBoundDevice")],
                  [t("partnerExtract.fields.product"), selectedMeeting.product_name || t("partnerExtract.fallback.unspecifiedProduct")],
                  [t("partnerExtract.fields.initiator"), selectedMeeting.created_by || "-"],
                  [t("partnerExtract.fields.scheduledTime"), selectedMeeting.scheduled_at || "-"],
                  [t("partnerExtract.fields.startedAt"), selectedMeeting.started_at || "-"],
                  [t("partnerExtract.fields.endedAt"), selectedMeeting.ended_at || "-"],
                  [t("partnerExtract.fields.duration"), meetingDurationText(selectedMeeting.duration_seconds)],
                  [t("partnerExtract.fields.participants"), t("partnerExtract.detail.peopleCount", { count: selectedMeeting.participant_count || 0 })],
                ] as const).map(([label, value]) => (
                  <div key={label}>
                    <div className="text-xs text-muted-foreground">{label}</div>
                    <div className="mt-1 break-words text-sm font-medium text-foreground">{value}</div>
                  </div>
                ))}
              </div>

              <div className="border-t border-border p-5">
                <div className="flex items-center justify-between gap-3">
                  <h3 className="text-sm font-semibold text-foreground">{t("partnerExtract.video.participants")}</h3>
                  <span className="text-xs text-muted-foreground">{t("partnerExtract.common.recordCount", { count: selectedMeeting.participants?.length ?? 0 })}</span>
                </div>
                {!selectedMeeting.participants?.length ? (
                  <p className="mt-3 text-sm text-muted-foreground">{t("partnerExtract.empty.noMeetingRecords")}</p>
                ) : (
                  <div className={`mt-3 ${partnerDividerClass}`}>
                    {selectedMeeting.participants.map((participant) => (
                      <div key={participant.id} className="flex flex-wrap items-center justify-between gap-3 py-3 text-sm">
                        <div>
                          <div className="font-medium text-foreground">{participant.name || t("partnerExtract.fallback.unnamedParticipant")}</div>
                          <div className="mt-0.5 text-xs text-muted-foreground">{participant.user_type} · {participant.role}</div>
                        </div>
                        <div className="text-xs text-muted-foreground">{participant.left_at ? t("partnerExtract.status.participant.left") : t("partnerExtract.status.participant.inMeeting")}</div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          ) : (
            <EmptyState title={t("partnerExtract.empty.selectMeeting")} />
          )}
        </div>
      </section>
    </div>
  )
}

export function PartnerTicketDetailPage() {
  const t = useI18n()
  const params = useParams<{ collaborationId: string }>()
  const collaborationID = Number(params?.collaborationId)
  return (
    <PartnerTicketDetailView
      collaborationID={collaborationID}
      backHref="/partner/tickets"
      backLabel={t("partnerExtract.nav.backToTickets")}
    />
  )
}

export function PartnerTicketDetailAliasPage() {
  const t = useI18n()
  const searchParams = useSearchParams()
  const collaborationID = Number(searchParams.get("collaboration_id") || searchParams.get("collaborationId"))
  return (
    <PartnerTicketDetailView
      collaborationID={collaborationID}
      backHref="/partner/tickets"
      backLabel={t("partnerExtract.nav.backToTickets")}
    />
  )
}

function PartnerTicketDetailView({
  collaborationID,
  backHref = "/partner/tickets",
  backLabel = pt("nav.backToTickets"),
  showBackLink = true,
  headerDescription,
}: {
  collaborationID: number
  backHref?: string
  backLabel?: string
  showBackLink?: boolean
  headerDescription?: string
}) {
  const t = useI18n()
  const [detail, setDetail] = useState<PartnerTicketDetail | null>(null)
  const [profile, setProfile] = useState<PartnerPortalProfile | null>(null)
  const [accounts, setAccounts] = useState<PartnerPortalAccount[]>([])
  const [participantAccountID, setParticipantAccountID] = useState("")
  const [detailLoading, setDetailLoading] = useState(true)
  const [detailLoaded, setDetailLoaded] = useState(false)
  const [detailError, setDetailError] = useState("")
  const [profileLoading, setProfileLoading] = useState(true)
  const [profileLoaded, setProfileLoaded] = useState(false)
  const [profileError, setProfileError] = useState("")
  const [accountsLoading, setAccountsLoading] = useState(true)
  const [accountsLoaded, setAccountsLoaded] = useState(false)
  const [accountsError, setAccountsError] = useState("")
  const [progress, setProgress] = useState("")
  const [resolution, setResolution] = useState("")
  const [resolving, setResolving] = useState(false)
  const [busy, setBusy] = useState(false)
  const [mediaSending, setMediaSending] = useState(false)
  const [joiningMeeting, setJoiningMeeting] = useState(false)
  const [meetings, setMeetings] = useState<MeetingListItem[] | null>(null)
  const [meetingsLoading, setMeetingsLoading] = useState(true)
  const [meetingsLoaded, setMeetingsLoaded] = useState(false)
  const [meetingLoadError, setMeetingLoadError] = useState("")
  const [presence, setPresence] = useState<Record<string, RealtimePresencePayload>>({})
  const [remoteTyping, setRemoteTyping] = useState<RealtimeTypingState>({})
  const partnerSocketRef = useRef<WebSocket | null>(null)
  const partnerTypingTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const imageInputRef = useRef<HTMLInputElement | null>(null)
  const attachmentInputRef = useRef<HTMLInputElement | null>(null)

  const refreshMeetings = useCallback(async (silent = false) => {
    if (!silent) setMeetingsLoading(true)
    const meetingsRes = await safePartnerRequest(() => fetchPartnerMeetings("all"), t("partnerExtract.errors.loadMeetingStatusFailed"))
    if (meetingsRes.success && meetingsRes.data) {
      setMeetings(meetingsRes.data.items)
      setMeetingsLoaded(true)
      setMeetingLoadError("")
    } else {
      setMeetingLoadError(meetingsRes.error?.message || t("partnerExtract.errors.loadMeetingStatusFailed"))
    }
    if (!silent) setMeetingsLoading(false)
  }, [t])

  const load = useCallback(async () => {
    if (!Number.isFinite(collaborationID) || collaborationID <= 0) {
      setDetailError(t("partnerExtract.errors.invalidCollaborationTicket"))
      setDetailLoading(false)
      setProfileLoading(false)
      setAccountsLoading(false)
      setMeetingsLoading(false)
      return
    }
    const loadDetail = async () => {
      setDetailLoading(true)
      setDetailError("")
      const detailRes = await safePartnerRequest(() => fetchPartnerTicketDetail(collaborationID), t("partnerExtract.errors.loadTicketDetailFailed"))
      if (detailRes.success && detailRes.data) {
        setDetail(detailRes.data)
        setDetailLoaded(true)
      } else {
        setDetailError(detailRes.error?.message || t("partnerExtract.errors.loadTicketDetailFailed"))
      }
      setDetailLoading(false)
    }
    const loadProfile = async () => {
      setProfileLoading(true)
      setProfileError("")
      const profileRes = await safePartnerRequest(() => fetchPartnerProfile(), t("partnerExtract.errors.loadPartnerProfileFailed"))
      if (profileRes.success) {
        setProfile(profileRes.data ?? null)
        setProfileLoaded(true)
      } else {
        setProfileError(profileRes.error?.message || t("partnerExtract.errors.loadPartnerProfileFailed"))
      }
      setProfileLoading(false)
    }
    const loadAccounts = async () => {
      setAccountsLoading(true)
      setAccountsError("")
      const accountRes = await safePartnerRequest(() => fetchPartnerAccounts(), t("partnerExtract.errors.loadPartnerMembersFailed"))
      if (accountRes.success) {
        setAccounts(accountRes.data ?? [])
        setAccountsLoaded(true)
      } else {
        setAccountsError(accountRes.error?.message || t("partnerExtract.errors.loadPartnerMembersFailed"))
      }
      setAccountsLoading(false)
    }
    setMeetingLoadError("")
    const detailTask = loadDetail()
    void loadProfile()
    void loadAccounts()
    void refreshMeetings()
    await detailTask
  }, [collaborationID, refreshMeetings, t])

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => { void load() })
    return () => window.cancelAnimationFrame(frame)
  }, [load])

  useEffect(() => {
    if (!Number.isFinite(collaborationID) || collaborationID <= 0) return
    const timer = window.setInterval(() => {
      void fetchPartnerTicketDetail(collaborationID).then((detailRes) => {
        if (detailRes.success && detailRes.data) setDetail(detailRes.data)
      }).catch(() => undefined)
      void refreshMeetings(true)
    }, 10_000)
    return () => window.clearInterval(timer)
  }, [collaborationID, refreshMeetings])

  useEffect(() => {
    const conversationID = detail?.collaboration.conversation_id ?? 0
    if (conversationID <= 0) return
    const typingExpiry = new Map<string, ReturnType<typeof setTimeout>>()
    const presenceExpiry = new Map<string, {
      expiresAt: string
      timer: ReturnType<typeof setTimeout>
    }>()

    const clearPresenceExpiry = (actorId: string) => {
      const lease = presenceExpiry.get(actorId)
      if (lease) clearTimeout(lease.timer)
      presenceExpiry.delete(actorId)
    }

    const schedulePresenceExpiry = (payload: RealtimePresencePayload) => {
      if (!payload.actorId) return
      if (!payload.online) {
        clearPresenceExpiry(payload.actorId)
        return
      }
      const delay = realtimePresenceExpiryDelay(payload.expiresAt)
      if (delay === null || !payload.expiresAt) return
      const currentLease = presenceExpiry.get(payload.actorId)
      if (
        currentLease &&
        Date.parse(currentLease.expiresAt) >= Date.parse(payload.expiresAt)
      ) {
        return
      }
      clearPresenceExpiry(payload.actorId)
      const expectedExpiresAt = payload.expiresAt
      presenceExpiry.set(payload.actorId, {
        expiresAt: expectedExpiresAt,
        timer: setTimeout(() => {
          setPresence((current) => expireRealtimePresenceActor(
            current,
            payload.actorId,
            expectedExpiresAt,
          ))
          presenceExpiry.delete(payload.actorId)
        }, delay),
      })
    }

    const markRemoteParticipantsOffline = () => {
      presenceExpiry.forEach((lease) => clearTimeout(lease.timer))
      presenceExpiry.clear()
      setPresence((current) => Object.fromEntries(
        Object.entries(current).map(([key, value]) => [key, { ...value, online: false }]),
      ))
      setRemoteTyping({})
    }

    const realtime = createRealtimeConnectionManager({
      createSocket: createAdminWebSocket,
      onSocketChange: (socket) => {
        partnerSocketRef.current = socket
      },
      onOpen: (socket) => {
        socket.send(JSON.stringify({
          type: "subscribe",
          topics: [`conversation:${conversationID}`],
        }))
        void fetchPartnerTicketDetail(collaborationID).then((res) => {
          if (res.success && res.data) setDetail(res.data)
        }).catch(() => undefined)
      },
      onMessage: (event) => {
        let envelope: { type?: string; data?: unknown }
        try {
          envelope = JSON.parse(event.data) as typeof envelope
        } catch {
          return
        }
        if (envelope.type === "presence.changed") {
          const payload = envelope.data as RealtimePresencePayload | undefined
          if (payload?.actorId && payload.actorId !== `user:${profile?.user_id ?? 0}` && payload.conversationId === conversationID) {
            setPresence((current) => mergeRealtimePresenceActor(current, payload))
            schedulePresenceExpiry(payload)
          }
          return
        }
        if (envelope.type === "presence.snapshot") {
          const snapshot = envelope.data as RealtimePresenceSnapshotPayload | undefined
          if (snapshot?.conversationId === conversationID) {
            presenceExpiry.forEach((lease) => clearTimeout(lease.timer))
            presenceExpiry.clear()
            const participants = (snapshot.participants ?? [])
              .filter((item) => item.actorId !== `user:${profile?.user_id ?? 0}` && item.actorId && item.online)
            setPresence(Object.fromEntries(participants.map((item) => [item.actorId, item])))
            participants.forEach(schedulePresenceExpiry)
          }
          return
        }
        if (envelope.type === "typing.changed") {
          const payload = envelope.data as RealtimeTypingPayload | undefined
          if (payload?.actorId && payload.actorId !== `user:${profile?.user_id ?? 0}` && payload.conversationId === conversationID) {
            setRemoteTyping((current) => updateRealtimeTypingState(current, payload))
            const activePresence = presenceFromRealtimeTyping(payload)
            if (activePresence) {
              setPresence((current) => mergeRealtimePresenceActor(current, activePresence))
              schedulePresenceExpiry(activePresence)
            }
            const existingTimer = typingExpiry.get(payload.actorId)
            if (existingTimer) clearTimeout(existingTimer)
            typingExpiry.delete(payload.actorId)
            if (payload.typing) {
              const expectedActorID = payload.actorId
              typingExpiry.set(expectedActorID, setTimeout(() => {
                setRemoteTyping((current) => expireRealtimeTypingActor(current, expectedActorID))
                typingExpiry.delete(expectedActorID)
              }, 5_200))
            }
          }
          return
        }
        if (envelope.type === "message.created" || envelope.type?.startsWith("conversation.")) {
          void fetchPartnerTicketDetail(collaborationID).then((res) => {
            if (res.success && res.data) setDetail(res.data)
          }).catch(() => undefined)
        }
      },
      onClose: markRemoteParticipantsOffline,
      onConnectError: markRemoteParticipantsOffline,
      reconnectBaseDelayMs: 500,
      reconnectMaxDelayMs: 5_000,
    })
    realtime.connect()

    return () => {
      typingExpiry.forEach((timer) => clearTimeout(timer))
      typingExpiry.clear()
      presenceExpiry.forEach((lease) => clearTimeout(lease.timer))
      presenceExpiry.clear()
      realtime.disconnect()
      partnerSocketRef.current = null
    }
  }, [collaborationID, detail?.collaboration.conversation_id, profile?.user_id])

  useEffect(() => {
    const conversationID = detail?.collaboration.conversation_id ?? 0
    const socket = partnerSocketRef.current
    if (conversationID <= 0 || !socket || socket.readyState !== WebSocket.OPEN) return
    const typing = progress.trim().length > 0
    socket.send(JSON.stringify({ type: "typing", conversationId: conversationID, typing }))
    if (partnerTypingTimerRef.current) clearTimeout(partnerTypingTimerRef.current)
    if (typing) {
      partnerTypingTimerRef.current = setTimeout(() => {
        if (socket.readyState === WebSocket.OPEN) {
          socket.send(JSON.stringify({ type: "typing", conversationId: conversationID, typing: false }))
        }
      }, TYPING_IDLE_TIMEOUT_MS)
    }
    return () => {
      if (partnerTypingTimerRef.current) clearTimeout(partnerTypingTimerRef.current)
    }
  }, [detail?.collaboration.conversation_id, progress])

  const accept = async () => {
    if (!detail || busy) return
    setBusy(true)
    const res = await acceptPartnerTicket(detail.collaboration.id)
    if (res.success) {
      toast.success(t("partnerExtract.toasts.ticketAccepted"))
      await load()
      const conversationID = detail.collaboration.conversation_id ?? 0
      const socket = partnerSocketRef.current
      if (conversationID > 0 && socket?.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: "subscribe", topics: [`conversation:${conversationID}`] }))
      }
    } else {
      toast.error(res.error?.message || t("partnerExtract.errors.acceptFailed"))
    }
    setBusy(false)
  }

  const joinActiveMeeting = async () => {
    if (!detail || joiningMeeting) return
    setJoiningMeeting(true)
    try {
      const meetingsRes = await fetchPartnerMeetings("all")
      if (!meetingsRes.success || !meetingsRes.data) {
        const message = meetingsRes.error?.message || t("partnerExtract.errors.loadVideoFailed")
        setMeetingLoadError(message)
        throw new Error(message)
      }
      setMeetings(meetingsRes.data.items)
      setMeetingsLoaded(true)
      setMeetingLoadError("")
      const access = resolvePartnerMeetingAccess({
        collaboration: detail.collaboration,
        meetings: meetingsRes.data.items,
      })
      if (!access.meeting) {
        throw new Error(t("partnerExtract.errors.noActiveCollaboration"))
      }
      window.open(partnerMeetingRoomHref(detail.collaboration.id, access.meeting.id), "_blank", "noopener,noreferrer")
    } catch (joinError) {
      toast.error(joinError instanceof Error ? joinError.message : t("partnerExtract.errors.enterVideoFailed"))
    } finally {
      setJoiningMeeting(false)
    }
  }

  const addProgress = async () => {
    if (!detail || !progress.trim() || busy) return
    setBusy(true)
    const res = await createPartnerTicketProgress(detail.collaboration.id, progress.trim())
    if (res.success && res.data) {
      setDetail(res.data)
      setProgress("")
      toast.success(t("partnerExtract.toasts.replySent"))
    } else {
      toast.error(res.error?.message || t("partnerExtract.errors.submitProgressFailed"))
    }
    setBusy(false)
  }

  const sendMediaMessage = async (
    kind: "image" | "audio" | "attachment",
    file: File,
    durationSeconds = 0,
  ) => {
    if (!detail || mediaSending) return
    const pendingId = createPendingMessageId()
    const pendingMessage: PartnerConversationMessage = {
      id: pendingId,
      sender_id: profile?.user_id ?? 0,
      sender_type: "partner",
      sender_name: profile?.display_name || t("partnerExtract.common.me"),
      message_type: kind,
      content: file.name,
      payload: buildMediaTransferPayload({
        filename: file.name,
        fileSize: file.size,
        mimeType: file.type,
        durationSeconds: durationSeconds || undefined,
      }),
      sent_at: new Date().toISOString(),
    }
    setDetail((current) => current
      ? { ...current, messages: [...(current.messages ?? []), pendingMessage] }
      : current)
    setMediaSending(true)
    try {
      const upload = kind === "image"
        ? await uploadPartnerTicketImage(detail.collaboration.id, file)
        : kind === "audio"
          ? await uploadPartnerTicketAudio(detail.collaboration.id, file)
          : await uploadPartnerTicketAttachment(detail.collaboration.id, file)
      if (!upload.success || !upload.data) {
        throw new Error(upload.error?.message || t("partnerExtract.errors.mediaUploadFailed"))
      }
      const sent = await createPartnerTicketMessage(detail.collaboration.id, {
        message_type: kind,
        asset_id: upload.data.assetId,
        duration_seconds: durationSeconds || undefined,
      })
      if (!sent.success || !sent.data) {
        throw new Error(sent.error?.message || t("partnerExtract.errors.mediaMessageFailed"))
      }
      setDetail(sent.data)
      toast.success(kind === "image" ? t("partnerExtract.toasts.imageSent") : kind === "audio" ? t("partnerExtract.toasts.audioSent") : t("partnerExtract.toasts.attachmentSent"))
    } catch (sendError) {
      setDetail((current) => current
        ? {
            ...current,
            messages: (current.messages ?? []).map((message) => message.id === pendingId
              ? {
                  ...message,
                  payload: buildMediaTransferPayload({
                    filename: file.name,
                    fileSize: file.size,
                    mimeType: file.type,
                    durationSeconds: durationSeconds || undefined,
                  }, "failed"),
                }
              : message),
          }
        : current)
      toast.error(sendError instanceof Error ? sendError.message : t("partnerExtract.errors.mediaMessageFailed"))
    } finally {
      setMediaSending(false)
    }
  }

  const addParticipant = async () => {
    if (!detail || !participantAccountID || busy) return
    setBusy(true)
    const res = await addPartnerTicketParticipant(detail.collaboration.id, Number(participantAccountID))
    if (res.success && res.data) {
      setDetail(res.data)
      setParticipantAccountID("")
      toast.success(t("partnerExtract.toasts.participantAdded"))
    } else {
      toast.error(res.error?.message || t("partnerExtract.errors.addParticipantFailed"))
    }
    setBusy(false)
  }

  const removeParticipant = async (partnerAccountID: number) => {
    if (!detail || busy) return
    setBusy(true)
    const res = await removePartnerTicketParticipant(detail.collaboration.id, partnerAccountID)
    if (res.success && res.data) {
      setDetail(res.data)
      toast.success(t("partnerExtract.toasts.participantRemoved"))
    } else {
      toast.error(res.error?.message || t("partnerExtract.errors.removeParticipantFailed"))
    }
    setBusy(false)
  }

  const submitResolution = async () => {
    if (!detail || !resolution.trim() || busy) return
    setBusy(true)
    const res = await resolvePartnerTicket(detail.collaboration.id, resolution.trim())
    if (res.success) {
      setResolving(false)
      setResolution("")
      toast.success(t("partnerExtract.toasts.resolutionSubmitted"))
      await load()
    } else {
      toast.error(res.error?.message || t("partnerExtract.errors.submitFinalResolutionFailed"))
    }
    setBusy(false)
  }

  const loading = detailLoading || profileLoading || accountsLoading || meetingsLoading
  const detailInitialLoading = detailLoading && !detailLoaded
  const profileInitialLoading = profileLoading && !profileLoaded
  const accountsInitialLoading = accountsLoading && !accountsLoaded
  const meetingsInitialLoading = meetingsLoading && !meetingsLoaded

  if (!detail) {
    return (
      <div className="rhd-railops-partner-page rhd-railops-partner-ticket-detail-page space-y-4">
        <section className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            {showBackLink ? (
              <Button size="sm" variant="ghost" className="mb-2 -ml-2" render={<Link href={backHref} />}>
                <ArrowLeftIcon className="size-4" />
                {backLabel}
              </Button>
            ) : null}
            <h1 className={`text-2xl font-semibold ${partnerTitleClass}`}>{t("partnerExtract.detail.title")}</h1>
          </div>
          <Button variant="outline" onClick={() => void load()} disabled={loading}>
            <RefreshCwIcon className={cn("size-4", loading && "animate-spin")} />
            {t("partnerExtract.common.refresh")}
          </Button>
        </section>

        {detailError && !detailLoaded ? (
          <ErrorState title={t("partnerExtract.errors.loadFailed")} description={detailError} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} />
        ) : null}

        <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_320px]">
          <div className="space-y-4">
            {detailInitialLoading ? (
              <>
                <ModuleLoading variant="detail" count={4} label={t("partnerExtract.loading.ticketContext")} />
                <ModuleLoading variant="list" count={4} label={t("partnerExtract.loading.messages")} />
              </>
            ) : detailError ? null : (
              <EmptyState title={t("partnerExtract.empty.ticketMissing")} />
            )}
          </div>
          <aside className="space-y-3">
            {detailInitialLoading ? (
              <>
                <ModuleLoading variant="list" count={3} label={t("partnerExtract.loading.collaborationInfo")} />
                <ModuleLoading variant="list" count={3} label={t("partnerExtract.loading.audit")} />
              </>
            ) : (
              null
            )}
          </aside>
        </section>
      </div>
    )
  }

  const ticket = detail.collaboration
  const status = collaborationStatus(ticket.status)
  const canWork = ticket.status !== "invited" && isPartnerCollaborationWorkable(ticket)
  const meetingAccess = resolvePartnerMeetingAccess({
    collaboration: ticket,
    loadError: meetingLoadError,
    meetings,
  })
  const customerOnline = Object.values(presence).some(
    (item) => item.participantType === "customer" && item.online,
  )
  const engineerOnline = Object.values(presence).some(
    (item) => item.participantType === "agent" && item.online,
  )
  const supplierCoworkerOnline = Object.values(presence).some(
    (item) => item.participantType === "partner" && item.online,
  )
  const remoteTypists = Object.values(remoteTyping)
  const remoteTypingLabel = remoteTypists.length > 1
    ? t("partnerExtract.typing.multiple", { count: remoteTypists.length })
    : remoteTypists[0]
      ? t("partnerExtract.typing.single", {
          name: remoteTypists[0].displayName || (
            remoteTypists[0].participantType === "customer"
              ? t("partnerExtract.roles.customer")
              : remoteTypists[0].participantType === "partner"
                ? t("partnerExtract.roles.partnerCoworker")
                : t("partnerExtract.roles.enterpriseEngineer")
          ),
        })
      : ""
  const activeParticipantIDs = new Set((detail.participants ?? []).filter((item) => item.status === 0).map((item) => item.partner_account_id))
  const availableParticipantAccounts = accounts.filter((item) => item.status === 0 && !activeParticipantIDs.has(item.id))
  const availableParticipantOptions = partnerAccountSelectOptions(availableParticipantAccounts)
  const currentPartnerAccountID = profile?.account_id ?? 0
  const canPrimaryOrAdmin = !profileInitialLoading && Boolean(profile?.can_manage_team || ticket.partner_account_id === currentPartnerAccountID)
  const canReply = canWork && Boolean(
    canPrimaryOrAdmin ||
    activeParticipantIDs.has(currentPartnerAccountID),
  )

  return (
    <div className="rhd-railops-partner-page rhd-railops-partner-ticket-detail-page space-y-4">
      <section className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <div>
          {showBackLink ? (
            <Button size="sm" variant="ghost" className="mb-2 -ml-2" render={<Link href={backHref} />}>
              <ArrowLeftIcon className="size-4" />
              {backLabel}
            </Button>
          ) : null}
          <div className="flex flex-wrap items-center gap-2">
            <h1 className={`text-2xl font-semibold ${partnerTitleClass}`}>{ticket.ticket_no || t("partnerExtract.fallback.collaborationWithId", { id: ticket.id })}</h1>
            <Badge variant="secondary">{t("partnerExtract.conversations.title")}</Badge>
            <Badge variant="outline" className={status.className}>{status.label}</Badge>
            <Badge variant="outline" className={ticketStatus(ticket.ticket_status).className}>{ticketStatus(ticket.ticket_status).label}</Badge>
            {!ticket.authorization_active && !isPartnerCollaborationTerminal(ticket.status) ? <Badge variant="outline" className="border-border bg-muted text-muted-foreground">{t("partnerExtract.badges.authorizationExpired")}</Badge> : null}
          </div>
          <p className={`mt-1 text-sm ${partnerMutedTextClass}`}>{headerDescription || ticket.ticket_title || t("partnerExtract.fallback.authorizedTask")}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" onClick={load} disabled={loading}>
            <RefreshCwIcon className={cn("size-4", loading && "animate-spin")} />
            {t("partnerExtract.common.refresh")}
          </Button>
          {ticket.status === "invited" && ticket.authorization_active && canPrimaryOrAdmin ? <Button disabled={busy} onClick={() => void accept()}>{t("partnerExtract.actions.accept")}</Button> : null}
          {canReply ? <Button disabled={busy} onClick={() => setResolving(true)}>{t("partnerExtract.actions.submitFinalResolution")}</Button> : null}
        </div>
      </section>

      {detailError || profileError ? (
        <div className="rounded-md border border-destructive/20 bg-destructive/5 px-4 py-2 text-xs text-destructive">
          {[detailError, profileError].filter(Boolean).join("；")}
        </div>
      ) : null}

      <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_320px]">
        <div className="space-y-4">
          <div className={`${partnerPanelClass} p-5`}>
            <h2 className={`text-sm font-semibold ${partnerTitleClass}`}>{t("partnerExtract.detail.problemContext")}</h2>
            <dl className="mt-4 grid gap-4 sm:grid-cols-2">
              <div><dt className={`text-xs ${partnerMutedTextClass}`}>{t("partnerExtract.fields.reason")}</dt><dd className="mt-1 text-sm text-foreground">{ticket.reason || "-"}</dd></div>
              <div><dt className={`text-xs ${partnerMutedTextClass}`}>{t("partnerExtract.fields.faultCode")}</dt><dd className="mt-1 text-sm text-foreground">{ticket.fault_code || t("partnerExtract.fallback.notAuthorizedOrEmpty")}</dd></div>
              <div className="sm:col-span-2"><dt className={`text-xs ${partnerMutedTextClass}`}>{t("partnerExtract.fields.symptom")}</dt><dd className="mt-1 text-sm text-foreground">{ticket.symptom_summary || t("partnerExtract.fallback.notAuthorizedOrEmpty")}</dd></div>
              <div className="sm:col-span-2"><dt className={`text-xs ${partnerMutedTextClass}`}>{t("partnerExtract.fields.diagnosis")}</dt><dd className="mt-1 whitespace-pre-wrap text-sm text-foreground">{ticket.diagnosis_summary || t("partnerExtract.fallback.notAuthorizedOrEmpty")}</dd></div>
              {ticket.resolution ? <div className="sm:col-span-2"><dt className={`text-xs ${partnerMutedTextClass}`}>{t("partnerExtract.fields.finalResolution")}</dt><dd className="mt-1 whitespace-pre-wrap text-sm text-foreground">{ticket.resolution}</dd></div> : null}
            </dl>
          </div>

          <div className={partnerPanelClass}>
            <div className="flex items-start gap-3 border-b border-border p-5">
              <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
                <MessageSquareTextIcon className="size-4" />
              </span>
              <div>
                <h2 className={`text-sm font-semibold ${partnerTitleClass}`}>{t("partnerExtract.conversations.collaborationConversations")}</h2>
                <div className={`mt-2 flex flex-wrap items-center gap-3 text-xs ${partnerMutedTextClass}`}>
                  <span className="inline-flex items-center gap-1.5">
                    <span className={cn("size-1.5 rounded-full", customerOnline ? "bg-primary" : "bg-muted-foreground/30")} />
                    {t("partnerExtract.presence.customer", { status: customerOnline ? t("partnerExtract.presence.online") : t("partnerExtract.presence.offline") })}
                  </span>
                  <span className="inline-flex items-center gap-1.5">
                    <span className={cn("size-1.5 rounded-full", engineerOnline ? "bg-primary" : "bg-muted-foreground/30")} />
                    {t("partnerExtract.presence.enterpriseEngineer", { status: engineerOnline ? t("partnerExtract.presence.online") : t("partnerExtract.presence.offline") })}
                  </span>
                  <span className="inline-flex items-center gap-1.5">
                    <span className={cn("size-1.5 rounded-full", supplierCoworkerOnline ? "bg-primary" : "bg-muted-foreground/30")} />
                    {t("partnerExtract.presence.partnerCoworker", { status: supplierCoworkerOnline ? t("partnerExtract.presence.online") : t("partnerExtract.presence.offline") })}
                  </span>
                </div>
              </div>
            </div>
            {(detail.messages ?? []).length === 0 ? (
              <EmptyState title={t("partnerExtract.empty.noMessages")} className="rounded-none border-0" />
            ) : (
              <div className="max-h-[420px] space-y-3 overflow-y-auto p-5">
                {(detail.messages ?? []).map((message) => {
                  const ownMessage = message.sender_type === "partner" && message.sender_id === profile?.user_id
                  const serviceEvent = parseConversationServiceEvent({
                    senderType: message.sender_type,
                    content: message.content,
                    payload: message.payload,
                  })
                  if (serviceEvent) {
                    return (
                      <ConversationServiceEvent
                        audience="partner"
                        key={message.id}
                        event={serviceEvent}
                        time={dateText(message.sent_at)}
                      />
                    )
                  }
                  return (
                    <div
                      key={message.id}
                      data-sender-type={message.sender_type}
                      className={cn("flex", ownMessage ? "justify-end" : "justify-start")}
                    >
                      <div className="max-w-[85%]">
                        <div className={cn(`mb-1 flex gap-2 text-rhd-xs ${partnerMutedTextClass}`, ownMessage && "justify-end")}>
                          <span>{ownMessage ? t("partnerExtract.common.me") : message.sender_name || t("partnerExtract.fallback.collaborationMember")}</span>
                          <time>{dateText(message.sent_at)}</time>
                        </div>
                        <div className={cn(
                          "whitespace-pre-wrap break-words rounded-md px-3 py-2 text-sm leading-6",
                          ownMessage ? "bg-primary text-primary-foreground" : "border border-border bg-muted text-foreground",
                        )}>
                          <ImMessageHTML
                            html={renderIMMessageHTML({
                              messageType: message.message_type || "text",
                              content: message.content,
                              payload: message.payload,
                            })}
                            className={ownMessage ? "[&_a]:text-primary-foreground" : ""}
                          />
                        </div>
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
            {profileInitialLoading ? (
              <div className={`border-t border-border bg-muted/50 px-5 py-3 text-sm ${partnerMutedTextClass}`}>
                {t("partnerExtract.loading.permissions")}
              </div>
            ) : profileError && !profileLoaded ? (
              <div className="border-t border-border bg-destructive/5 px-5 py-3 text-sm text-destructive">
                {t("partnerExtract.errors.permissionsFailed")}
              </div>
            ) : canReply ? (
              <div className="border-t border-border p-5">
                <input
                  ref={imageInputRef}
                  type="file"
                  accept="image/*"
                  className="hidden"
                  onChange={(event) => {
                    const file = event.target.files?.[0]
                    event.target.value = ""
                    if (file) void sendMediaMessage("image", file)
                  }}
                />
                <input
                  ref={attachmentInputRef}
                  type="file"
                  className="hidden"
                  onChange={(event) => {
                    const file = event.target.files?.[0]
                    event.target.value = ""
                    if (file) void sendMediaMessage("attachment", file)
                  }}
                />
                <Label htmlFor="partner-conversation-reply">{t("partnerExtract.fields.replyConversation")}</Label>
                <div
                  className={`mt-1 h-4 text-xs ${partnerMutedTextClass}`}
                  aria-live="polite"
                  data-testid="partner-typing-status"
                >
                  {remoteTypingLabel}
                </div>
                <Textarea
                  id="partner-conversation-reply"
                  aria-label={t("partnerExtract.fields.replyConversation")}
                  className="mt-2 min-h-24"
                  value={progress}
                  onChange={(event) => setProgress(event.target.value)}
                  placeholder={t("partnerExtract.placeholders.reply")}
                />
                <div className="mt-3 flex items-center gap-1.5">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    onClick={() => imageInputRef.current?.click()}
                    disabled={mediaSending}
                    aria-label={t("partnerExtract.actions.sendImage")}
                    title={t("partnerExtract.actions.sendImage")}
                  >
                    <ImageIcon className="size-4" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    onClick={() => attachmentInputRef.current?.click()}
                    disabled={mediaSending}
                    aria-label={t("partnerExtract.actions.sendAttachment")}
                    title={t("partnerExtract.actions.sendAttachment")}
                  >
                    <PaperclipIcon className="size-4" />
                  </Button>
                  <VoiceRecorderButton
                    disabled={mediaSending}
                    onRecorded={(file, durationSeconds) => sendMediaMessage("audio", file, durationSeconds)}
                    onError={(message) => toast.error(message)}
                  />
                  <Button className="ml-auto" disabled={busy || !progress.trim()} onClick={() => void addProgress()}>{t("partnerExtract.actions.sendReply")}</Button>
                </div>
              </div>
            ) : canWork ? (
              <div className={`border-t border-border bg-muted/50 px-5 py-3 text-sm ${partnerMutedTextClass}`}>
                {t("partnerExtract.detail.readonlyConversation")}
              </div>
            ) : null}
          </div>

        </div>

        <aside className="space-y-3">
          <div className={`${partnerPanelClass} p-4`}>
            <h2 className={`text-sm font-semibold ${partnerTitleClass}`}>{t("partnerExtract.detail.collaborationInfo")}</h2>
            <dl className="mt-4 space-y-3 text-sm">
              <div><dt className={`text-xs ${partnerMutedTextClass}`}>{t(ticket.product_id > 0 ? "partnerExtract.fields.productModule" : "partnerExtract.fields.collaborationScope")}</dt><dd className="mt-1 text-foreground">{collaborationScopeText(ticket)}</dd></div>
              <div><dt className={`text-xs ${partnerMutedTextClass}`}>{t("partnerExtract.fields.internalOwner")}</dt><dd className="mt-1 text-foreground">{ticket.partner_account_name || t("partnerExtract.fallback.unassignedWork")}</dd></div>
              <div><dt className={`text-xs ${partnerMutedTextClass}`}>{t("partnerExtract.fields.invitedAt")}</dt><dd className="mt-1 text-foreground">{dateText(ticket.invited_at)}</dd></div>
              <div><dt className={`text-xs ${partnerMutedTextClass}`}>{t("partnerExtract.fields.authorizationEndsAt")}</dt><dd className="mt-1 text-foreground">{dateText(ticket.authorization_ends_at)}</dd></div>
            </dl>
          </div>
          <div
            className={partnerPanelClass}
            data-testid="partner-collaboration-audit"
          >
            <div className="flex items-start justify-between gap-3 border-b border-border p-4">
              <div>
                <h2 className={`text-sm font-semibold ${partnerTitleClass}`}>{t("partnerExtract.detail.auditTitle")}</h2>
                <p className={`mt-1 text-xs ${partnerMutedTextClass}`}>{t("partnerExtract.detail.auditDescription")}</p>
              </div>
              <span className={`shrink-0 text-xs tabular-nums ${partnerMutedTextClass}`}>{t("partnerExtract.common.totalItems", { count: detail.progresses.length })}</span>
            </div>
            {detail.progresses.length === 0 ? (
              <div className={`px-4 py-6 text-center text-sm ${partnerMutedTextClass}`}>{t("partnerExtract.empty.noProgress")}</div>
            ) : (
              <div className={`max-h-80 ${partnerDividerClass} overflow-y-auto overscroll-contain`}>
                {detail.progresses.map((item) => (
                  <div key={item.id} className="p-4">
                    <div className="flex items-start justify-between gap-3">
                      <span className="min-w-0 truncate text-xs font-medium text-foreground">{item.author_name || t("partnerExtract.fallback.system")}</span>
                      <span className={`shrink-0 text-xs ${partnerMutedTextClass}`}>{dateText(item.created_at)}</span>
                    </div>
                    <p className="mt-2 whitespace-pre-wrap break-words text-sm leading-5 text-foreground">{item.content}</p>
                  </div>
                ))}
              </div>
            )}
          </div>
          <div className={`${partnerPanelClass} p-4`}>
            <div className="flex items-center justify-between gap-3">
              <h2 className={`text-sm font-semibold ${partnerTitleClass}`}>{t("partnerExtract.detail.members")}</h2>
              <span className={`text-xs ${partnerMutedTextClass}`}>{t("partnerExtract.detail.participantCount", { count: ticket.participant_count || 0 })}</span>
            </div>
            <div className={`mt-3 ${partnerDividerClass}`}>
              {(detail.participants ?? []).map((participant) => (
                <div key={participant.partner_account_id} className="flex items-center justify-between gap-2 py-2.5">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium text-foreground">{participant.partner_account_name || t("partnerExtract.fallback.memberWithId", { id: participant.partner_account_id })}</div>
                    <div className={`mt-0.5 text-xs ${partnerMutedTextClass}`}>
                      {participant.role === "owner" ? t("partnerExtract.roles.owner") : participant.status === 0 ? t("partnerExtract.roles.collaborationMember") : t("partnerExtract.status.participant.exited")}
                    </div>
                  </div>
                  {profile?.can_manage_team && canWork && participant.status === 0 && participant.role !== "owner" ? (
                    <Button
                      size="icon-sm"
                      variant="ghost"
                      title={t("partnerExtract.actions.removeFromCollaboration")}
                      disabled={busy}
                      onClick={() => void removeParticipant(participant.partner_account_id)}
                    >
                      <UserMinusIcon className="size-4" />
                    </Button>
                  ) : null}
                </div>
              ))}
            </div>
            {profileInitialLoading || accountsInitialLoading ? (
              <ModuleLoading className="mt-3 border-t border-border pt-3" variant="list" count={2} label={t("partnerExtract.loading.memberPermissions")} />
            ) : profileError && !profileLoaded ? (
              <div className="mt-3 border-t border-border pt-3 text-xs text-destructive">{t("partnerExtract.errors.memberPermissionsFailed")}</div>
            ) : accountsError && !accountsLoaded ? (
              <div className="mt-3 border-t border-border pt-3 text-xs text-destructive">{t("partnerExtract.errors.addableMembersFailed")}</div>
            ) : profile?.can_manage_team && canWork && availableParticipantAccounts.length > 0 ? (
              <div className="mt-3 flex gap-2 border-t border-border pt-3">
                <SelectField
                  style={{ marginBottom: 0 }}
                  selectProps={{
                    placeholder: t("partnerExtract.placeholders.selectCollaborationMember"),
                    value: participantAccountID,
                    onChange: (value) => setParticipantAccountID(value ?? ""),
                    options: availableParticipantOptions,
                    style: { flex: 1, minWidth: 0 },
                  }}
                />
                <Button size="icon" title={t("partnerExtract.actions.addCollaborationMember")} disabled={busy || !participantAccountID} onClick={() => void addParticipant()}>
                  <UserPlusIcon className="size-4" />
                </Button>
              </div>
            ) : accountsError ? (
              <div className="mt-3 border-t border-border pt-3 text-xs text-destructive">{accountsError}</div>
            ) : null}
          </div>
          {meetingAccess.meeting ? (
            <Button
              className="w-full"
              variant="outline"
              disabled={joiningMeeting || !canReply}
              onClick={() => { if (canReply) void joinActiveMeeting() }}
            >
              <VideoIcon className="size-4" />
              {joiningMeeting ? t("partnerExtract.actions.entering") : canReply ? t("partnerExtract.actions.enterVideo") : t("partnerExtract.actions.acceptBeforeVideo")}
            </Button>
          ) : meetingsInitialLoading ? (
            <ModuleLoading variant="list" count={1} label={t("partnerExtract.loading.videoStatus")} />
          ) : meetingAccess.state !== "hidden" ? (
            <div role="status" className={`flex items-center gap-2 rounded-md border border-border bg-muted/50 px-3 py-2.5 text-sm ${partnerMutedTextClass}`}>
              <VideoIcon className="size-4 shrink-0" />
              <span>{meetingAccess.message}</span>
            </div>
          ) : null}
        </aside>
      </section>

      <StandardModal
        open={resolving}
        onCancel={() => setResolving(false)}
        title={t("partnerExtract.modals.submitFinalResolution")}
        width={512}
        footer={
          <>
            <Button variant="outline" onClick={() => setResolving(false)}>{t("partnerExtract.common.cancel")}</Button>
            <Button disabled={busy || !resolution.trim()} onClick={() => void submitResolution()}>{t("partnerExtract.actions.confirmSubmit")}</Button>
          </>
        }
      >
        <Textarea aria-label={t("partnerExtract.placeholders.resolution")} className="min-h-32" value={resolution} onChange={(event) => setResolution(event.target.value)} placeholder={t("partnerExtract.placeholders.resolution")} />
      </StandardModal>
    </div>
  )
}

function parseLanguages(value: string) {
  try {
    const parsed = JSON.parse(value)
    return Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === "string") : []
  } catch {
    return []
  }
}

type PartnerAccountFormState = {
  username: string
  displayName: string
  email: string
  phone: string
  languages: string
  roleCode: "partner_admin" | "partner_engineer"
  status: number
}

function accountFormState(account?: PartnerPortalAccount | null): PartnerAccountFormState {
  return {
    username: "",
    displayName: account?.display_name ?? "",
    email: account?.email ?? "",
    phone: account?.phone ?? "",
    languages: parseLanguages(account?.languages_json ?? "[]").join(", "),
    roleCode: account?.roles?.includes("partner_admin") ? "partner_admin" : "partner_engineer",
    status: account?.status ?? 0,
  }
}

function PartnerAccountEditor({
  account,
  canManageTeam,
  open,
  onOpenChange,
  onSaved,
}: {
  account: PartnerPortalAccount | null
  canManageTeam: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: (account: PartnerPortalAccount, initialPassword?: string) => void
}) {
  const [form, setForm] = useState<PartnerAccountFormState>(() => accountFormState(account))
  const [saving, setSaving] = useState(false)
  const t = useI18n()

  const setField = <K extends keyof PartnerAccountFormState>(field: K, value: PartnerAccountFormState[K]) => {
    setForm((current) => ({ ...current, [field]: value }))
  }

  const submit = async () => {
    if (!form.displayName.trim() || (!account && !form.username.trim())) return
    setSaving(true)
    const payload: PartnerAccountPayload = {
      username: account ? undefined : form.username.trim(),
      display_name: form.displayName.trim(),
      email: form.email.trim(),
      phone: form.phone.trim(),
      languages: form.languages.split(",").map((item) => item.trim()).filter(Boolean),
      role_code: canManageTeam ? form.roleCode : "partner_engineer",
      status: form.status,
    }
    const res = account
      ? await updatePartnerAccount(account.id, payload)
      : await createPartnerAccount(payload)
    if (res.success && res.data) {
      if (account) {
        onSaved(res.data as PartnerPortalAccount)
      } else {
        const created = res.data as { account: PartnerPortalAccount; initial_password: string }
        onSaved(created.account, created.initial_password)
      }
      onOpenChange(false)
      toast.success(account ? t("partnerExtract.toasts.memberUpdated") : t("partnerExtract.toasts.accountCreated"))
    } else {
      toast.error(res.error?.message || t("partnerExtract.errors.saveMemberFailed"))
    }
    setSaving(false)
  }

  return (
    <StandardModal
      open={open}
      onCancel={() => onOpenChange(false)}
      title={account ? t("partnerExtract.modals.editMember") : t("partnerExtract.modals.inviteCoworker")}
      width={512}
      footer={
        <>
          <Button variant="outline" onClick={() => onOpenChange(false)}>{t("partnerExtract.common.cancel")}</Button>
          <Button disabled={saving || !form.displayName.trim() || (!account && !form.username.trim())} onClick={() => void submit()}>
            {saving ? t("partnerExtract.common.saving") : t("partnerExtract.common.save")}
          </Button>
        </>
      }
    >
        <div className="grid gap-4 sm:grid-cols-2">
          {!account ? (
            <div className="space-y-2 sm:col-span-2">
              <Label htmlFor="partner-username">{t("partnerExtract.fields.loginAccount")}</Label>
              <Input id="partner-username" value={form.username} onChange={(event) => setField("username", event.target.value)} />
            </div>
          ) : null}
          <div className="space-y-2">
            <Label htmlFor="partner-display-name">{t("partnerExtract.fields.name")}</Label>
            <Input id="partner-display-name" value={form.displayName} onChange={(event) => setField("displayName", event.target.value)} />
          </div>
          <div className="space-y-2">
            {canManageTeam ? (
              <SelectField
                label={t("partnerExtract.fields.role")}
                style={{ marginBottom: 0 }}
                selectProps={{
                  "aria-label": t("partnerExtract.fields.role"),
                  value: form.roleCode,
                  onChange: (value) => setField("roleCode", value as PartnerAccountFormState["roleCode"]),
                  options: [
                    { value: "partner_admin", label: t("partnerExtract.roles.supplierAdmin") },
                    { value: "partner_engineer", label: t("partnerExtract.roles.supplierEngineer") },
                  ],
                  style: { width: "100%" },
                }}
              />
            ) : (
              <>
                <Label>{t("partnerExtract.fields.role")}</Label>
                <Input value={t("partnerExtract.roles.supplierEngineer")} disabled readOnly />
              </>
            )}
          </div>
          <div className="space-y-2">
            <Label htmlFor="partner-email">{t("partnerExtract.fields.email")}</Label>
            <Input id="partner-email" type="email" value={form.email} onChange={(event) => setField("email", event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="partner-phone">{t("partnerExtract.fields.phone")}</Label>
            <Input id="partner-phone" value={form.phone} onChange={(event) => setField("phone", event.target.value)} />
          </div>
          <div className="space-y-2 sm:col-span-2">
            <Label htmlFor="partner-languages">{t("partnerExtract.fields.serviceLanguages")}</Label>
            <Input id="partner-languages" value={form.languages} onChange={(event) => setField("languages", event.target.value)} placeholder="zh-CN, en-US" />
          </div>
          {account ? (
            <SelectField
              label={t("partnerExtract.fields.accountStatus")}
              className="sm:col-span-2"
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": t("partnerExtract.fields.accountStatus"),
                value: String(form.status),
                onChange: (value) => setField("status", Number(value)),
                disabled: account.is_current,
                options: [
                  { value: "0", label: t("partnerExtract.status.account.normal") },
                  { value: "1", label: t("partnerExtract.status.account.disabled") },
                ],
                style: { width: "100%" },
              }}
            />
          ) : null}
        </div>
    </StandardModal>
  )
}

export function PartnerMeetingRoomPage() {
  const params = useParams<{ meetingId?: string }>()
  const router = useRouter()
  const searchParams = useSearchParams()
  const routeMeetingId = params.meetingId ?? ""
  const queryMeetingId = searchParams.get("meeting_id") || searchParams.get("meetingId") || ""
  const meetingId = routeMeetingId && !isStaticExportRouteParam(routeMeetingId) ? routeMeetingId : queryMeetingId
  const requestedCollaborationId = Number(searchParams.get("collaboration_id") || searchParams.get("collaborationId") || "0")
  const [collaborationId, setCollaborationId] = useState(requestedCollaborationId)
  const [config, setConfig] = useState<MeetingJoinConfig | null>(null)
  const [profile, setProfile] = useState<PartnerPortalProfile | null>(null)
  const [initialTranscripts, setInitialTranscripts] = useState<MeetingTranscriptSegment[]>([])
  const [archiveMode, setArchiveMode] = useState(false)
  const [configLoading, setConfigLoading] = useState(true)
  const [configLoaded, setConfigLoaded] = useState(false)
  const [configError, setConfigError] = useState("")
  const [transcriptsLoading, setTranscriptsLoading] = useState(true)
  const [transcriptsLoaded, setTranscriptsLoaded] = useState(false)
  const [transcriptsError, setTranscriptsError] = useState("")
  const [profileLoading, setProfileLoading] = useState(true)
  const [profileLoaded, setProfileLoaded] = useState(false)
  const [profileError, setProfileError] = useState("")
  const t = useI18n()

  useEffect(() => {
    setCollaborationId(requestedCollaborationId)
  }, [requestedCollaborationId])

  const load = useCallback(async () => {
    setConfigError("")
    if (!meetingId) {
      setConfigError(t("partnerExtract.errors.meetingInfoMissing"))
      setConfigLoading(false)
      setTranscriptsLoading(false)
      setProfileLoading(false)
      return
    }
    let activeCollaborationId = requestedCollaborationId
    if (!activeCollaborationId) {
      setConfigLoading(true)
      const meetingsRes = await safePartnerRequest(() => fetchPartnerMeetings("all"), t("partnerExtract.errors.meetingCollaborationUnavailable"))
      if (!meetingsRes.success || !meetingsRes.data) {
        setConfigError(meetingsRes.error?.message || t("partnerExtract.errors.meetingCollaborationUnavailable"))
        setConfigLoading(false)
        setTranscriptsLoading(false)
        setProfileLoading(false)
        return
      }
      activeCollaborationId = meetingsRes.data.items.find((item) => item.id === meetingId)?.collaboration_id ?? 0
      if (!activeCollaborationId) {
        setConfigError(t("partnerExtract.errors.partnerCannotJoinMeeting"))
        setConfigLoading(false)
        setTranscriptsLoading(false)
        setProfileLoading(false)
        return
      }
      setCollaborationId(activeCollaborationId)
    }
    setArchiveMode(false)
    const loadConfig = async () => {
      setConfigLoading(true)
      setConfigError("")
      const configRes = await safePartnerRequest(
        () => fetchPartnerMeetingJoinConfig(activeCollaborationId, meetingId),
        t("partnerExtract.errors.meetingJoinConfigUnavailable"),
      )
      if (!configRes.success || !configRes.data) {
        const message = configRes.error?.message || t("partnerExtract.errors.meetingJoinConfigUnavailable")
        if (message.includes("会议已结束") || message.toLocaleLowerCase().includes("meeting has ended")) {
          setConfig(null)
          setConfigLoaded(false)
          setConfigError("")
          setArchiveMode(true)
          setCollaborationId(activeCollaborationId)
          setConfigLoading(false)
          return
        }
        setConfigError(message)
        setConfigLoading(false)
        return
      }
      setConfig(configRes.data)
      setConfigLoaded(true)
      setArchiveMode(false)
      setCollaborationId(activeCollaborationId)
      setConfigLoading(false)
    }
    const loadTranscripts = async () => {
      setTranscriptsLoading(true)
      setTranscriptsError("")
      const transcriptRes = await safePartnerRequest(
        () => fetchPartnerMeetingTranscripts(activeCollaborationId, meetingId),
        t("partnerExtract.errors.transcriptLoadFailed"),
      )
      if (transcriptRes.success) {
        setInitialTranscripts(transcriptRes.data ?? [])
        setTranscriptsLoaded(true)
      } else {
        setTranscriptsError(transcriptRes.error?.message || t("partnerExtract.errors.transcriptLoadFailed"))
      }
      setTranscriptsLoading(false)
    }
    const loadProfile = async () => {
      setProfileLoading(true)
      setProfileError("")
      const profileRes = await safePartnerRequest(() => fetchPartnerProfile(), t("partnerExtract.errors.loadPartnerProfileFailed"))
      if (profileRes.success) {
        setProfile(profileRes.data ?? null)
        setProfileLoaded(true)
      } else {
        setProfileError(profileRes.error?.message || t("partnerExtract.errors.loadPartnerProfileFailed"))
      }
      setProfileLoading(false)
    }
    const configTask = loadConfig()
    void loadTranscripts()
    void loadProfile()
    await configTask
  }, [meetingId, requestedCollaborationId, t])

  useEffect(() => {
    const timer = window.setTimeout(() => { void load() }, 0)
    return () => window.clearTimeout(timer)
  }, [load])

  const roomName = config?.roomName || config?.room_name
  const resolvedMeetingId = config?.meetingId || config?.meeting_id || meetingId
  const returnPath = partnerTicketDetailHref(collaborationId)
  const ended = configError.includes("会议已结束") || configError.toLocaleLowerCase().includes("meeting has ended")
  const realtimeTranscripts = initialTranscripts.filter((item) => !isSystemMeetingTranscript(item))
  const systemArchives = initialTranscripts.filter(isSystemMeetingTranscript)
  const profileInitialLoading = profileLoading && !profileLoaded
  const roomSideError = [
    transcriptsError,
    profileError && !profileInitialLoading ? profileError : "",
  ].filter(Boolean).join("；")

  if (archiveMode) {
    return (
      <div className="rhd-railops-partner-page rhd-railops-partner-meeting-archive mx-auto max-w-3xl space-y-4 p-6">
        <div>
          <h1 className="text-lg font-semibold">{t("partnerExtract.meetingArchive.endedTitle")}</h1>
        </div>
        <div className="rounded-md border bg-background">
          <div className="flex items-center justify-between gap-3 border-b px-4 py-3">
            <h2 className="text-sm font-semibold">{t("partnerExtract.meetingArchive.transcriptsTitle")}</h2>
            <span className="text-xs text-muted-foreground">{t("partnerExtract.meetingArchive.recordCount", { count: realtimeTranscripts.length })}</span>
          </div>
          {transcriptsLoading && !transcriptsLoaded ? (
            <ModuleLoading className="p-4" variant="list" count={4} label={t("partnerExtract.loading.transcripts")} />
          ) : transcriptsError && !transcriptsLoaded ? (
            <ErrorState title={t("partnerExtract.errors.transcriptLoadFailed")} description={transcriptsError} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} className="rounded-none border-0" />
          ) : realtimeTranscripts.length === 0 ? (
            <div className="space-y-3 p-6 text-sm text-muted-foreground">
              {transcriptsError ? <p className="text-destructive">{transcriptsError}</p> : null}
              <p>{t("partnerExtract.meetingArchive.noRealtimeTranscript")}</p>
              {systemArchives[0]?.text ? (
                <p className="rounded-md border bg-muted/40 px-3 py-2 leading-6">{systemArchives[0].text}</p>
              ) : null}
            </div>
          ) : (
            <div className="max-h-[60vh] divide-y overflow-y-auto">
              {realtimeTranscripts.map((item) => (
                <div key={item.id} className="p-4">
                  <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
                    <span>{item.speakerName || t("partnerExtract.fallback.systemRecord")}</span>
                    <span>{dateText(item.createdAt)}</span>
                  </div>
                  <p className="mt-2 whitespace-pre-wrap break-words text-sm leading-6">{item.text}</p>
                </div>
              ))}
            </div>
          )}
        </div>
        <Button onClick={() => router.push(returnPath)}>
          <ArrowLeftIcon className="size-4" />
          {t("partnerExtract.actions.backToTicket")}
        </Button>
      </div>
    )
  }

  if (configError && !configLoaded) {
    return (
      <div className="rhd-railops-partner-page rhd-railops-partner-meeting-state flex min-h-[60vh] flex-col items-center justify-center gap-4 px-6 text-center">
        <h1 className="text-lg font-semibold">{ended ? t("partnerExtract.meetingArchive.endedTitle") : t("partnerExtract.meetingArchive.cannotEnterTitle")}</h1>
        <p className="max-w-md text-sm text-muted-foreground">{configError}</p>
        <div className="flex flex-wrap justify-center gap-2">
          <Button onClick={() => router.push(returnPath)}>
            <ArrowLeftIcon className="size-4" />
            {t("partnerExtract.actions.backToTicket")}
          </Button>
          {!ended ? (
            <Button variant="outline" onClick={() => void load()}>
              <RefreshCwIcon className="size-4" />
              {t("partnerExtract.common.retry")}
            </Button>
          ) : null}
        </div>
      </div>
    )
  }

  if (!config || !roomName) {
    return (
      <div className="rhd-railops-partner-page rhd-railops-partner-meeting-loading space-y-4 p-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h1 className="text-lg font-semibold text-foreground">{t("partnerExtract.video.title")}</h1>
          </div>
          {collaborationId ? (
            <Button variant="outline" onClick={() => router.push(returnPath)}>
              <ArrowLeftIcon className="size-4" />
              {t("partnerExtract.actions.backToTicket")}
            </Button>
          ) : null}
        </div>
        {configLoading && !configLoaded ? (
          <ModuleLoading variant="detail" count={5} label={t("partnerExtract.loading.collaborationAuthorization")} />
        ) : (
          <EmptyState title={t("partnerExtract.errors.collaborationUnavailable")} />
        )}
      </div>
    )
  }

  return (
    <>
      {roomSideError ? (
        <div className="border-b border-destructive/20 bg-destructive/5 px-4 py-2 text-xs text-destructive">
          {roomSideError}
        </div>
      ) : null}
      <div className="rhd-railops-partner-live-room h-[calc(100vh-5.5rem)] min-h-[36rem] overflow-hidden border bg-black">
        <MeetingLiveRoom
          domain={config.domain}
          jitsiUrl={config.jitsiUrl || config.jitsi_url}
          meetingId={resolvedMeetingId}
          roomName={roomName}
          subject={config.subject || config.ticketNo || config.ticket_no || roomName}
          returnLabel={t("partnerExtract.actions.backToTicket")}
          jwt={config.jwt}
          canEndMeeting={false}
          transcriptionEnabled={config.transcriptionEnabled ?? config.transcription_enabled ?? false}
          transcriptionProvider={config.transcriptionProvider ?? config.transcription_provider}
          transcriptionReady={config.transcriptionReady ?? config.transcription_ready ?? false}
          transcriptionUnavailableReason={config.transcriptionError ?? config.transcription_error}
          arEnabled={config.arDetectionEnabled ?? config.ar_detection_enabled ?? false}
          customerInviteEnabled={false}
          displayName={profileLoaded && profile?.display_name ? profile.display_name : "Supplier Engineer"}
          initialTranscripts={initialTranscripts}
          fetchTranscripts={async () => {
            const result = await fetchPartnerMeetingTranscripts(collaborationId, resolvedMeetingId)
            if (!result.success) throw new Error(result.error?.message || t("partnerExtract.errors.transcriptLoadFailed"))
            return result.data ?? []
          }}
          fetchTranscriptPage={async (cursor) => {
            const result = await fetchPartnerMeetingTranscriptPage(collaborationId, resolvedMeetingId, cursor)
            if (!result.success || !result.data) throw new Error(result.error?.message || t("partnerExtract.errors.transcriptLoadFailed"))
            return result.data
          }}
          onConferenceJoined={async () => {
            const result = await confirmPartnerMeetingJoined(collaborationId, resolvedMeetingId)
            if (!result.success) throw new Error(result.error?.message || t("partnerExtract.errors.partnerJoinStatusWriteFailed"))
            publishMeetingSync({ type: "meeting-updated", meetingId: resolvedMeetingId, status: "active" })
          }}
          onConferenceLeft={async () => {
            const result = await confirmPartnerMeetingLeft(collaborationId, resolvedMeetingId)
            if (!result.success) throw new Error(result.error?.message || t("partnerExtract.errors.partnerLeaveStatusWriteFailed"))
            publishMeetingSync({ type: "meeting-updated", meetingId: resolvedMeetingId })
          }}
          onConferenceHeartbeat={async () => {
            const result = await heartbeatPartnerMeeting(collaborationId, resolvedMeetingId)
            if (!result.success) throw new Error(result.error?.message || t("partnerExtract.errors.partnerMeetingHeartbeatFailed"))
          }}
          onTranscriptFinal={async (event) => {
            const result = await ingestPartnerMeetingTranscript(collaborationId, resolvedMeetingId, event)
            if (!result.success) throw new Error(result.error?.message || t("partnerExtract.errors.realtimeTranscriptArchiveFailed"))
            return result.data ?? null
          }}
          fetchMeetingStatus={async () => {
            const result = await fetchPartnerMeetingStatus(collaborationId, resolvedMeetingId)
            return result.success ? result.data ?? null : null
          }}
          onMeetingEnd={() => {
            publishMeetingSync({ type: "meeting-updated", meetingId: resolvedMeetingId })
            router.push(returnPath)
          }}
          className="h-full min-h-0"
        />
      </div>
    </>
  )
}

export function PartnerPeoplePage() {
  const [profile, setProfile] = useState<PartnerPortalProfile | null>(null)
  const [items, setItems] = useState<PartnerPortalAccount[]>([])
  const [search, setSearch] = useState("")
  const [profileLoading, setProfileLoading] = useState(true)
  const [profileLoaded, setProfileLoaded] = useState(false)
  const [profileError, setProfileError] = useState("")
  const [accountsLoading, setAccountsLoading] = useState(true)
  const [accountsLoaded, setAccountsLoaded] = useState(false)
  const [accountsError, setAccountsError] = useState("")
  const [editorOpen, setEditorOpen] = useState(false)
  const [editingAccount, setEditingAccount] = useState<PartnerPortalAccount | null>(null)
  const [initialCredential, setInitialCredential] = useState<{ username: string; password: string } | null>(null)
  const t = useI18n()

  const load = useCallback((silent = false) => {
    const loadProfile = async () => {
      if (!silent) setProfileLoading(true)
      setProfileError("")
      const profileRes = await safePartnerRequest(() => fetchPartnerProfile(), t("partnerExtract.errors.loadPartnerProfileFailed"))
      if (profileRes.success) {
        setProfile(profileRes.data ?? null)
        setProfileLoaded(true)
      } else {
        setProfileError(profileRes.error?.message || t("partnerExtract.errors.loadPartnerProfileFailed"))
      }
      if (!silent) setProfileLoading(false)
    }
    const loadAccounts = async () => {
      if (!silent) setAccountsLoading(true)
      setAccountsError("")
      const accountRes = await safePartnerRequest(() => fetchPartnerAccounts(), t("partnerExtract.errors.loadPartnerPeopleFailed"))
      if (accountRes.success) {
        setItems(accountRes.data ?? [])
        setAccountsLoaded(true)
      } else {
        setAccountsError(accountRes.error?.message || t("partnerExtract.errors.loadPartnerPeopleFailed"))
      }
      if (!silent) setAccountsLoading(false)
    }
    void loadProfile()
    void loadAccounts()
  }, [t])

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => { void load() })
    return () => window.cancelAnimationFrame(frame)
  }, [load])

  const filtered = useMemo(() => {
    const keyword = search.trim().toLowerCase()
    if (!keyword) return items
    return items.filter((item) => [item.display_name, item.email, item.phone]
      .some((value) => value?.toLowerCase().includes(keyword)))
  }, [items, search])

  const activeCount = items.filter((item) => item.status === 0).length
  const languageCount = new Set(items.flatMap((item) => parseLanguages(item.languages_json))).size

  const handleSaved = useCallback((saved: PartnerPortalAccount, initialPassword?: string) => {
    setItems((current) => {
      const exists = current.some((item) => item.id === saved.id)
      return exists ? current.map((item) => item.id === saved.id ? saved : item) : [...current, saved]
    })
    if (initialPassword) {
      setInitialCredential({ username: saved.username, password: initialPassword })
    }
  }, [])

  const loading = profileLoading || accountsLoading
  const profileInitialLoading = profileLoading && !profileLoaded
  const accountsInitialLoading = accountsLoading && !accountsLoaded

  return (
    <div className="rhd-railops-partner-page rhd-railops-partner-people-page space-y-4">
      <PageHeader
        title={t("partnerExtract.people.title")}
        actions={
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" onClick={() => void load()} disabled={loading}>
              <RefreshCwIcon className={cn("size-4", loading && "animate-spin")} />
              {t("partnerExtract.common.refresh")}
            </Button>
            {profileLoaded && (profile?.can_invite_team || profile?.can_manage_team) ? (
              <Button onClick={() => { setEditingAccount(null); setEditorOpen(true) }}>
                <UserPlusIcon className="size-4" />
                {t("partnerExtract.actions.inviteCoworker")}
              </Button>
            ) : null}
          </div>
        }
      />

      {accountsInitialLoading ? (
        <ModuleLoading variant="metrics" count={3} label={t("partnerExtract.loading.peopleStats")} />
      ) : accountsError && !accountsLoaded ? (
        <ErrorState title={t("partnerExtract.errors.peopleStatsFailed")} description={accountsError} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} />
      ) : (
        <section className="grid gap-3 md:grid-cols-3">
          <SummaryMetric label={t("partnerExtract.metrics.companyMembers")} value={items.length} icon={<UsersIcon className="size-4" />} tone="blue" />
          <SummaryMetric label={t("partnerExtract.metrics.activeAccounts")} value={activeCount} icon={<CheckCircle2Icon className="size-4" />} tone="green" />
          <SummaryMetric label={t("partnerExtract.metrics.serviceLanguages")} value={languageCount} icon={<TicketCheckIcon className="size-4" />} tone="slate" />
        </section>
      )}

      <section className={partnerPanelClass}>
        <div className="flex flex-col gap-3 border-b border-border p-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="w-full sm:max-w-xs">
            <SearchField allowClear className="rhd-railops-search-standard" value={search} onChange={(event) => setSearch(event.target.value)} placeholder={t("partnerExtract.placeholders.searchPeople")} disabled={accountsInitialLoading} />
          </div>
          <span className={`text-xs ${partnerMutedTextClass}`}>{profileInitialLoading ? t("partnerExtract.loading.profile") : profileError && !profileLoaded ? t("partnerExtract.errors.profileLoadFailed") : t("partnerExtract.people.partnerSummary", { no: profile?.partner_no || "-", region: profile?.country_region || t("partnerExtract.fallback.regionUnset") })}</span>
        </div>

        {accountsInitialLoading ? (
          <ModuleLoading className="p-4" variant="table" count={6} label={t("partnerExtract.loading.peopleList")} />
        ) : accountsError && !accountsLoaded ? (
          <ErrorState title={t("partnerExtract.errors.peopleFailed")} description={accountsError} action={{ label: t("partnerExtract.common.retry"), onClick: () => void load() }} className="rounded-none border-0" />
        ) : filtered.length === 0 ? (
          <EmptyState title={t("partnerExtract.empty.noMembers")} className="rounded-none border-0" />
        ) : (
          <>
            {accountsError || profileError ? (
              <div className="border-b border-border bg-destructive/5 px-4 py-2 text-xs text-destructive">
                {[accountsError, profileError].filter(Boolean).join("；")}
              </div>
            ) : null}
            <div className="overflow-x-auto">
              <table className="rhd-railops-partner-table w-full min-w-[780px] border-separate border-spacing-0 text-sm">
                <thead>
                  <tr className={partnerTableHeadClass}>
                    <th className={partnerTableCellClass}>{t("partnerExtract.table.person")}</th>
                    <th className={partnerTableCellClass}>{t("partnerExtract.table.contact")}</th>
                    <th className={partnerTableCellClass}>{t("partnerExtract.table.language")}</th>
                    <th className={partnerTableCellClass}>{t("partnerExtract.table.role")}</th>
                    <th className={partnerTableCellClass}>{t("partnerExtract.table.recentActive")}</th>
                    <th className={partnerTableCellClass}>{t("partnerExtract.table.status")}</th>
                    {profile?.can_manage_team ? <th className={`${partnerTableCellClass} text-right`}>{t("partnerExtract.table.actions")}</th> : null}
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((item) => (
                    <tr key={item.id} className="text-foreground hover:bg-muted/50">
                      <td className={partnerTableCellClass}>
                        <div className="flex items-center gap-3">
                          <div className="grid size-9 shrink-0 place-items-center rounded-md bg-muted text-xs font-semibold text-foreground">
                            {(item.display_name || "-").slice(0, 1).toUpperCase()}
                          </div>
                          <div>
                            <div className={`font-medium ${partnerTitleClass}`}>{item.display_name || t("partnerExtract.fallback.memberWithId", { id: item.id })}</div>
                            {item.is_current ? <div className="mt-0.5 text-xs text-primary">{t("partnerExtract.common.me")}</div> : null}
                          </div>
                        </div>
                      </td>
                      <td className={partnerTableCellClass}>
                        <div>{item.email || "-"}</div>
                        <div className={`mt-1 text-xs ${partnerMutedTextClass}`}>{item.phone || "-"}</div>
                      </td>
                      <td className={partnerTableCellClass}>
                        <div className="flex flex-wrap gap-1.5">
                          {parseLanguages(item.languages_json).length > 0
                            ? parseLanguages(item.languages_json).map((language) => <Badge key={language} variant="secondary">{language}</Badge>)
                            : <span className={partnerMutedTextClass}>-</span>}
                        </div>
                      </td>
                      <td className={partnerTableCellClass}>
                        <Badge variant="outline" className={item.roles?.includes("partner_admin") ? partnerFilterActiveClass : "border-border bg-muted text-muted-foreground"}>
                          {item.roles?.includes("partner_admin") ? t("partnerExtract.roles.admin") : t("partnerExtract.roles.engineer")}
                        </Badge>
                      </td>
                      <td className={`${partnerTableCellClass} text-xs`}>{dateText(item.last_active_at)}</td>
                      <td className={partnerTableCellClass}>
                        <Badge variant="outline" className={item.status === 0 ? "border-border bg-card text-foreground" : "border-border bg-muted text-muted-foreground"}>
                          {item.status === 0 ? t("partnerExtract.status.account.normal") : t("partnerExtract.status.account.inactive")}
                        </Badge>
                      </td>
                      {profile?.can_manage_team ? (
                        <td className={`${partnerTableCellClass} text-right`}>
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => { setEditingAccount(item); setEditorOpen(true) }}
                          >
                            <PencilIcon className="size-3.5" />
                            {t("partnerExtract.actions.edit")}
                          </Button>
                        </td>
                      ) : null}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}
      </section>

      <PartnerAccountEditor
        key={`${editingAccount?.id ?? "new"}-${editorOpen ? "open" : "closed"}`}
        account={editingAccount}
        canManageTeam={Boolean(profile?.can_manage_team)}
        open={editorOpen}
        onOpenChange={setEditorOpen}
        onSaved={handleSaved}
      />

      <StandardModal
        open={Boolean(initialCredential)}
        onCancel={() => setInitialCredential(null)}
        title={t("partnerExtract.modals.initialCredentials")}
        width={448}
        footer={
          <Button onClick={() => setInitialCredential(null)}>{t("partnerExtract.common.done")}</Button>
        }
      >
          <div className="rounded-md border border-border bg-muted px-3 py-2 text-xs text-muted-foreground">{t("partnerExtract.people.credentialsShownOnce")}</div>
          <div className="rounded-md border border-border bg-muted/50 p-4">
            <div className={`text-xs ${partnerMutedTextClass}`}>{t("partnerExtract.fields.username")}</div>
            <div className={`mt-1 font-medium ${partnerTitleClass}`}>{initialCredential?.username}</div>
            <div className={`mt-3 text-xs ${partnerMutedTextClass}`}>{t("partnerExtract.fields.initialPassword")}</div>
            <div className={`mt-1 flex items-center justify-between gap-3 font-mono text-sm ${partnerTitleClass}`}>
              <span>{initialCredential?.password}</span>
              <Button size="icon-sm" variant="outline" title={t("partnerExtract.actions.copyInitialPassword")} onClick={() => void navigator.clipboard.writeText(initialCredential?.password || "")}>
                <CopyIcon className="size-3.5" />
              </Button>
            </div>
          </div>
      </StandardModal>
    </div>
  )
}

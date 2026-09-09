"use client"

import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import {
  ArrowLeftIcon,
  BadgeCheckIcon,
  CalendarClockIcon,
  CheckCircle2Icon,
  ChevronRightIcon,
  CircleAlertIcon,
  CircleUserRoundIcon,
  ClipboardListIcon,
  FileTextIcon,
  Loader2Icon,
  MessageCircleIcon,
  PackageSearchIcon,
  PlusIcon,
  RefreshCwIcon,
  ShieldCheckIcon,
  StarIcon,
  Trash2Icon,
  VideoIcon,
  WrenchIcon,
  XIcon,
} from "lucide-react"
import { SearchField } from "@railops/ui"

import { useAuth } from "@/components/auth-provider"
import { MeetingLiveRoom } from "@/components/meeting/meeting-live-room"
import { Button } from "@/components/ui/button"
import { Checkbox as AntCheckbox } from "antd"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { fetchCustomerDeviceManuals, fetchCustomerDevicesPage } from "@/lib/api/customer-devices"
import {
  confirmCustomerMeetingJoined,
  confirmCustomerMeetingLeft,
  fetchCustomerMeetingJoinConfig,
  fetchCustomerMeetingStatus,
  fetchCustomerMeetingTranscriptPage,
  fetchCustomerMeetingTranscripts,
  fetchCustomerMeetingsPage,
  heartbeatCustomerMeeting,
  ingestCustomerMeetingTranscript,
} from "@/lib/api/customer-meetings"
import {
  deleteCustomerAccount,
  fetchCustomerAccountDeletionStatus,
  fetchCustomerProfile,
  updateCustomerProfile,
} from "@/lib/api/customer-profile"
import {
  confirmCustomerTicket,
  fetchCustomerTicketDetail,
  fetchCustomerTicketsPage,
  reopenCustomerTicket,
  submitCustomerTicketFeedback,
} from "@/lib/api/customer-tickets"
import type {
  CustomerMeetingJoinConfig,
  CustomerAccountDeletion,
  CustomerPortalDevice,
  CustomerPortalManualFile,
  CustomerPortalMeeting,
  CustomerPortalProfile,
  CustomerPortalTicket,
} from "@/lib/api/customer-portal-types"
import type { MeetingTranscriptSegment } from "@/lib/api/types"
import { customerTicketProgressContent } from "@/lib/customer-ticket-progress-i18n"
import { cn } from "@/lib/utils"
import { useAppLocale, useI18n } from "@/i18n/provider"
import { renderMobileTicketTitle } from "@/components/remote-helpdesk/mobile-ticket-labels"

export type MobileCustomerNavigate = (
  tab: "chat" | "tickets" | "video" | "devices" | "my" | "scan",
  params?: Record<string, string>,
) => void

type AsyncState = {
  loading: boolean
  error: string
}

const mobileListPageSize = 30

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

function normalizeMobileConversationId(value: string | null) {
  const conversationId = Number(value)
  return Number.isSafeInteger(conversationId) && conversationId > 0 ? conversationId : 0
}

function upsertMobileMeeting(items: CustomerPortalMeeting[], meeting: CustomerPortalMeeting) {
  if (!items.some((item) => item.id === meeting.id)) return [meeting, ...items]
  return items.map((item) => item.id === meeting.id ? meeting : item)
}

type I18nT = ReturnType<typeof useI18n>

function formatDuration(seconds: number | undefined, t: I18nT) {
  if (!seconds) return "--"
  if (seconds < 60) return t("portalExtract.customerMobile.time.seconds", { seconds })
  const minutes = Math.floor(seconds / 60)
  const remainder = seconds % 60
  return remainder
    ? t("portalExtract.customerMobile.time.minutesSeconds", { minutes, seconds: remainder })
    : t("portalExtract.customerMobile.time.minutes", { minutes })
}

function formatTranscriptTime(item: MeetingTranscriptSegment, meeting: CustomerPortalMeeting, locale: string) {
  if (item.startedAtMs > 0 && item.startedAtMs < 24 * 60 * 60 * 1000) {
    const seconds = Math.floor(item.startedAtMs / 1000)
    return `${String(Math.floor(seconds / 60)).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`
  }
  return formatDate(item.createdAt || meeting.started_at, locale)
}

function mergeTranscriptItems(current: MeetingTranscriptSegment[], incoming: MeetingTranscriptSegment[]) {
  const merged = new Map(current.map((item) => [item.id || item.providerEventId, item]))
  for (const item of incoming) merged.set(item.id || item.providerEventId, item)
  return Array.from(merged.values())
}

function statusLabel(status: string, t: I18nT) {
  const labels: Record<string, string> = {
    active: t("portalExtract.customerMobile.status.processing"),
    accepted: t("portalExtract.customerMobile.status.accepted"),
    action_required: t("portalExtract.customerMobile.status.actionRequired"),
    ai_serving: t("portalExtract.customerMobile.status.aiServing"),
    cancelled: t("portalExtract.customerMobile.status.cancelled"),
    closed: t("portalExtract.customerMobile.status.closed"),
    done: t("portalExtract.customerMobile.status.done"),
    ended: t("portalExtract.customerMobile.status.ended"),
    finished: t("portalExtract.customerMobile.status.finished"),
    human_serving: t("portalExtract.customerMobile.status.humanServing"),
    idle: t("portalExtract.customerMobile.status.idle"),
    maintenance: t("portalExtract.customerMobile.status.maintenance"),
    offline: t("portalExtract.customerMobile.status.offline"),
    online: t("portalExtract.customerMobile.status.online"),
    pending_acceptance: t("portalExtract.customerMobile.status.pendingAcceptance"),
    pending_assignee_accept: t("portalExtract.customerMobile.status.pendingAssigneeAccept"),
    pending_confirmation: t("portalExtract.customerMobile.status.pendingConfirmation"),
    pending_dispatch: t("portalExtract.customerMobile.status.pendingDispatch"),
    processing: t("portalExtract.customerMobile.status.processing"),
    queued: t("portalExtract.customerMobile.status.queued"),
    reopened: t("portalExtract.customerMobile.status.reopened"),
    reviewing: t("portalExtract.customerMobile.status.reviewing"),
    scheduled: t("portalExtract.customerMobile.status.scheduled"),
    submitted: t("portalExtract.customerMobile.status.submitted"),
    waiting: t("portalExtract.customerMobile.status.waiting"),
    waiting_customer: t("portalExtract.customerMobile.status.waitingCustomer"),
  }
  return labels[status] || status || t("portalExtract.customerMobile.common.statusUnknown")
}

function StatusPill({ status }: { status: string }) {
  const t = useI18n()
  const active = new Set(["active", "online", "human_serving", "processing"]).has(status)
  const pending = new Set([
    "action_required",
    "ai_serving",
    "pending_acceptance",
    "pending_assignee_accept",
    "pending_confirmation",
    "pending_dispatch",
    "queued",
    "scheduled",
    "submitted",
    "waiting",
    "waiting_customer",
  ]).has(status)
  return (
    <span className={cn(
      "inline-flex shrink-0 items-center gap-1.5 text-rhd-xs font-medium",
      active && "text-[#087f5b]",
      pending && "text-[#b55b08]",
      !active && !pending && "text-[#737d8e]",
    )}>
      <span className={cn(
        "size-1.5 rounded-full",
        active && "bg-[#10a779]",
        pending && "bg-[#e28a28]",
        !active && !pending && "bg-[#9aa3b2]",
      )} />
      {statusLabel(status, t)}
    </span>
  )
}

function PageError({ message, onRetry }: { message: string; onRetry: () => void }) {
  const t = useI18n()
  return (
    <div className="mx-4 mt-16 flex flex-col items-center px-6 text-center">
      <span className="flex size-11 items-center justify-center rounded-full bg-[#fff0f0] text-[#c23b3b]">
        <CircleAlertIcon className="size-5" />
      </span>
      <p className="mt-3 text-sm font-semibold text-[#172033]">{t("portalExtract.customerMobile.common.pageLoadFailed")}</p>
      <p className="mt-1 text-xs leading-5 text-[#778195]">{message}</p>
      <Button type="button" variant="outline" size="sm" className="mt-4 border-[#dce2ea] bg-white" onClick={onRetry}>
        <RefreshCwIcon className="size-4" />{t("portalExtract.customerMobile.common.reload")}
      </Button>
    </div>
  )
}

function LoadingBar({ className = "" }: { className?: string }) {
  return <span className={cn("block animate-pulse rounded-full bg-[#e8ecf1]", className)} />
}

function InlineLoadingHint({ label }: { label: string }) {
  return (
    <div className="flex items-center gap-2 px-4 py-2 text-rhd-xs text-[#657084]" role="status" aria-busy="true" aria-label={label}>
      <Loader2Icon className="size-3.5 animate-spin" />
      <span>{label}</span>
    </div>
  )
}

function MobileListLoading({
  label,
  rows = 5,
  tone = "blue",
  toolbar = "search",
}: {
  label: string
  rows?: number
  tone?: "blue" | "indigo" | "violet" | "emerald"
  toolbar?: "search" | "filters" | "none"
}) {
  const toneClass = {
    blue: "bg-blue-50",
    indigo: "bg-indigo-50",
    violet: "bg-violet-50",
    emerald: "bg-emerald-50",
  }[tone]
  const listClassName = cn("space-y-2", toolbar === "none" ? "" : "px-3")
  return (
    <div className="py-3" role="status" aria-busy="true" aria-label={label}>
      {toolbar === "search" ? (
        <div className="space-y-3 px-4 pb-3">
          <LoadingBar className="h-11 rounded-lg bg-white" />
          <LoadingBar className="h-9 rounded-lg bg-[#e3e7ec]" />
        </div>
      ) : null}
      {toolbar === "filters" ? (
        <div className="px-4 pb-3">
          <LoadingBar className="h-9 rounded-lg bg-[#e3e7ec]" />
        </div>
      ) : null}
      <div className={listClassName}>
          {Array.from({ length: rows }).map((_, index) => (
            <div key={index} className="min-h-[92px] rounded-lg border border-[#e3e7ed] bg-white px-4 py-3">
              <span className="flex items-center justify-between gap-4">
                <LoadingBar className="h-3.5 w-40 max-w-full" />
                <span className={cn("h-3 w-14 shrink-0 animate-pulse rounded-full", toneClass)} />
              </span>
              <LoadingBar className="mt-3 h-3 w-56 max-w-full" />
              <LoadingBar className="mt-2 h-2.5 w-32 max-w-full" />
            </div>
          ))}
      </div>
    </div>
  )
}

function MobileProfileLoading() {
  const t = useI18n()
  const { session } = useAuth()
  const hasDeviceConcept = session?.featureFlags?.device !== false
  return (
    <div role="status" aria-busy="true" aria-label={t("portalExtract.customerMobile.common.accountProfileLoading")}>
      <div className="bg-[#172033] px-4 py-5">
        <div className="flex items-center gap-3">
          <span className="size-12 shrink-0 animate-pulse rounded-full bg-white/15" />
          <span className="min-w-0 flex-1 space-y-2">
            <LoadingBar className="h-4 w-28 bg-white/45" />
            <LoadingBar className="h-3 w-40 bg-white/20" />
          </span>
        </div>
      </div>
      <div className="grid divide-x divide-[#edf0f4] bg-white px-2 py-4" style={{ gridTemplateColumns: `repeat(${hasDeviceConcept ? 4 : 3}, minmax(0, 1fr))` }}>
        {Array.from({ length: hasDeviceConcept ? 4 : 3 }).map((_, index) => (
          <div key={index} className="px-2 text-center">
            <LoadingBar className="mx-auto h-5 w-8" />
            <LoadingBar className="mx-auto mt-2 h-2.5 w-12" />
          </div>
        ))}
      </div>
      <div className="mt-3 space-y-3 bg-white px-4 py-5">
        {Array.from({ length: 4 }).map((_, index) => (
          <LoadingBar key={index} className="h-10 rounded-md bg-zinc-100" />
        ))}
      </div>
    </div>
  )
}

function EmptyState({ icon, title, action }: { icon: ReactNode; title: string; action?: ReactNode }) {
  return (
    <div className="flex min-h-[46vh] flex-col items-center justify-center px-8 text-center">
      <span className="flex size-12 items-center justify-center rounded-full bg-[#e9edf2] text-[#687386]">{icon}</span>
      <p className="mt-4 text-sm font-semibold text-[#172033]">{title}</p>
      {action ? <div className="mt-4">{action}</div> : null}
    </div>
  )
}

function SectionLabel({ children, action }: { children: ReactNode; action?: ReactNode }) {
  return (
    <div className="flex items-center justify-between pb-2 pt-1">
      <h2 className="text-xs font-semibold text-[#657084]">{children}</h2>
      {action}
    </div>
  )
}

function BackTitle({ title, subtitle, onBack }: { title: string; subtitle?: string; onBack: () => void }) {
  const t = useI18n()
  return (
    <div className="sticky top-0 z-20 flex min-w-0 items-center gap-2 border-b border-[#e5e9ef] bg-white/96 px-2 py-2.5 backdrop-blur-xl">
      <Button type="button" variant="ghost" size="icon" className="size-11 shrink-0 rounded-full" onClick={onBack} aria-label={t("portalExtract.customerMobile.common.back")}>
        <ArrowLeftIcon className="size-5" />
      </Button>
      <div className="min-w-0">
        <h2 className="truncate text-sm font-semibold text-[#172033]">{title}</h2>
        {subtitle ? <p className="mt-0.5 truncate text-rhd-xs text-[#778195]">{subtitle}</p> : null}
      </div>
    </div>
  )
}

export { MobileCustomerConversationWorkbench as MobileCustomerChatPage } from "@/components/remote-helpdesk/mobile-customer-conversation-workbench"

export function MobileCustomerTicketsPage({ navigate }: { navigate: MobileCustomerNavigate }) {
  const t = useI18n()
  const { locale } = useAppLocale()
  const { session } = useAuth()
  const hasDeviceConcept = session?.featureFlags?.device !== false
  const searchParams = useSearchParams()
  const [state, setState] = useState<AsyncState>({ loading: true, error: "" })
  const [tickets, setTickets] = useState<CustomerPortalTicket[]>([])
  const [hasLoadedTickets, setHasLoadedTickets] = useState(false)
  const [filter, setFilter] = useState<"all" | "open" | "closed">("all")
  const [query, setQuery] = useState("")
  const [ticketPage, setTicketPage] = useState(1)
  const [ticketTotal, setTicketTotal] = useState(0)
  const [selectedId, setSelectedId] = useState(0)
  const [selectedTicketDetail, setSelectedTicketDetail] = useState<CustomerPortalTicket | null>(null)
  const [ticketDetailLoading, setTicketDetailLoading] = useState(false)
  const [ticketDetailError, setTicketDetailError] = useState("")
  const [actionLoading, setActionLoading] = useState(false)
  const [reopenReason, setReopenReason] = useState("")
  const [showReopen, setShowReopen] = useState(false)
  const [rating, setRating] = useState(5)
  const [ratingComment, setRatingComment] = useState("")
  const [notice, setNotice] = useState("")
  const returnConversationId = normalizeMobileConversationId(searchParams.get("fromConversationId"))

  const selectedSummary = tickets.find((item) => item.id === selectedId) ?? null
  const selected = selectedTicketDetail?.id === selectedId ? selectedTicketDetail : selectedSummary

  const load = useCallback(async (nextPage = 1, append = false) => {
    setState({ loading: true, error: "" })
    try {
      const page = await fetchCustomerTicketsPage({
        page: nextPage,
        limit: mobileListPageSize,
        keyword: query.trim(),
        filter: filter === "all" ? undefined : filter,
      })
      const nextItems = page.results ?? []
      setTickets((current) => {
        if (!append) return nextItems
        const seen = new Set(current.map((item) => item.id))
        return [...current, ...nextItems.filter((item) => !seen.has(item.id))]
      })
      setTicketPage(nextPage)
      setTicketTotal(page.page?.total ?? nextItems.length)
      setHasLoadedTickets(true)
      setState({ loading: false, error: "" })
    } catch (error) {
      setState({ loading: false, error: error instanceof Error ? error.message : t("portalExtract.customerMobile.ticket.loadFailed") })
    }
  }, [filter, query])
  useEffect(() => { void load() }, [load])
  useEffect(() => {
    const requestedTicketNo = searchParams.get("ticketNo")?.trim()
    if (!requestedTicketNo) return
    const requested = tickets.find((item) => item.ticket_no === requestedTicketNo)
    if (requested) setSelectedId(requested.id)
  }, [searchParams, tickets])
  useEffect(() => {
    if (selectedId <= 0) {
      setSelectedTicketDetail(null)
      setTicketDetailLoading(false)
      setTicketDetailError("")
      return
    }
    let cancelled = false
    setSelectedTicketDetail(null)
    setTicketDetailLoading(true)
    setTicketDetailError("")
    void fetchCustomerTicketDetail(selectedId)
      .then((detail) => {
        if (cancelled) return
        setSelectedTicketDetail(detail)
        setTickets((current) => current.map((item) => item.id === detail.id ? detail : item))
      })
      .catch((error) => {
        if (cancelled) return
        setTicketDetailError(error instanceof Error ? error.message : t("portalExtract.customerMobile.ticket.loadFailed"))
      })
      .finally(() => {
        if (!cancelled) setTicketDetailLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [selectedId, t])

  const visibleTickets = tickets
  const ticketInitialLoading = state.loading && !hasLoadedTickets
  const ticketHasMore = tickets.length < ticketTotal
  const ticketRefreshing = state.loading && hasLoadedTickets

  const selectedNeedsFeedback = Boolean(selected?.can_rate && !selected.feedback)
  const selectedCanEndTicket = Boolean(selected?.can_confirm)
  const selectedShowsResolutionPanel = Boolean(
    selected && (selectedNeedsFeedback || selectedCanEndTicket || selected.can_reopen || selected.feedback),
  )

  async function runTicketAction(action: () => Promise<unknown>, successMessage: string) {
    if (!selected || actionLoading) return false
    setActionLoading(true)
    setNotice("")
    try {
      await action()
      setNotice(successMessage)
      await load(ticketPage)
      const detail = await fetchCustomerTicketDetail(selected.id)
      setSelectedTicketDetail(detail)
      setTickets((items) => items.map((item) => item.id === detail.id ? detail : item))
      return true
    } catch (error) {
      setNotice(error instanceof Error ? error.message : t("portalExtract.customerMobile.common.actionFailed"))
      return false
    } finally {
      setActionLoading(false)
    }
  }

  async function submitResolutionAction() {
    if (!selected || actionLoading) return
    const shouldSubmitFeedback = selected.can_rate && !selected.feedback
    const shouldEndTicket = selected.can_confirm
    const succeeded = await runTicketAction(async () => {
      if (shouldSubmitFeedback) {
        await submitCustomerTicketFeedback(selected.id, {
          rating,
          comment: ratingComment.trim(),
        })
      }
      if (shouldEndTicket) {
        await confirmCustomerTicket(selected.id)
      }
    }, shouldEndTicket ? t("portalExtract.customerMobile.ticket.ticketEnded") : t("portalExtract.customerMobile.ticket.feedbackSubmitted"))
    if (shouldSubmitFeedback && succeeded) {
      setRating(5)
      setRatingComment("")
    }
  }

  function closeTicketDetail() {
    setSelectedId(0)
    setShowReopen(false)
    setNotice("")
    if (returnConversationId > 0) {
      navigate("chat", { conversationId: String(returnConversationId) })
    }
  }

  if (state.error && !hasLoadedTickets) return <PageError message={state.error} onRetry={() => void load()} />

  if (selected) {
    const selectedTitle = renderMobileTicketTitle(selected.title, t)
    return (
      <div className="min-h-full bg-white">
        <BackTitle title={selected.ticket_no} subtitle={selectedTitle} onBack={closeTicketDetail} />
        <div className="space-y-5 px-4 py-5">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0"><h2 className="text-lg font-semibold leading-7 text-[#172033]">{selectedTitle}</h2>{hasDeviceConcept ? <p className="mt-1 font-mono text-xs text-[#778195]">{selected.device_no || t("portalExtract.customerMobile.common.noLinkedDevice")}</p> : null}</div>
            <StatusPill status={selected.status} />
          </div>
          <dl className="grid grid-cols-2 gap-x-4 gap-y-4 border-y border-[#edf0f4] py-4 text-xs">
            {hasDeviceConcept ? <div><dt className="text-[#939dab]">{t("portalExtract.customerMobile.common.product")}</dt><dd className="mt-1 truncate font-medium text-[#303b4d]">{selected.product_name || "--"}</dd></div> : null}
            <div><dt className="text-[#939dab]">{t("portalExtract.customerMobile.ticket.assignee")}</dt><dd className="mt-1 truncate font-medium text-[#303b4d]">{selected.assignee_name || t("portalExtract.customerMobile.ticket.unassigned")}</dd></div>
            <div><dt className="text-[#939dab]">{t("portalExtract.customerMobile.ticket.createdAt")}</dt><dd className="mt-1 font-medium text-[#303b4d]">{formatDate(selected.created_at, locale)}</dd></div>
            <div><dt className="text-[#939dab]">{t("portalExtract.customerMobile.ticket.updatedAt")}</dt><dd className="mt-1 font-medium text-[#303b4d]">{formatDate(selected.updated_at, locale)}</dd></div>
          </dl>

          {selectedShowsResolutionPanel ? (
            <section className="rounded-lg border border-[#d6e4f7] bg-[#f8fbff] px-3 py-3 shadow-sm">
              <SectionLabel>{selectedCanEndTicket ? t("portalExtract.customerMobile.ticket.endTicketTitle") : t("portalExtract.customerMobile.ticket.serviceFeedback")}</SectionLabel>
              {selectedCanEndTicket ? (
                <p className="mb-3 rounded-md bg-white px-3 py-2 text-xs leading-5 text-[#657084]">
                  {selectedNeedsFeedback ? t("portalExtract.customerMobile.ticket.endTicketHintWithFeedback") : t("portalExtract.customerMobile.ticket.endTicketHint")}
                </p>
              ) : null}
              {selected.feedback ? (
                <div className="rounded-md bg-white px-3 py-2 text-xs text-[#465266]">
                  <div className="flex items-center justify-between gap-3">
                    <span className="font-medium text-[#303b4d]">
                      {selected.can_rate ? t("portalExtract.customerMobile.ticket.previousFeedback") : t("portalExtract.customerMobile.ticket.currentFeedback")}
                    </span>
                    <span className="text-rhd-2xs text-[#939dab]">{formatDate(selected.feedback.submitted_at, locale)}</span>
                  </div>
                  <div className="mt-1.5 flex items-center gap-1" aria-label={t("portalExtract.customerMobile.common.star", { value: selected.feedback.rating })}>
                    {[1, 2, 3, 4, 5].map((value) => (
                      <StarIcon key={value} className={cn("size-4", value <= selected.feedback!.rating ? "fill-amber-400 text-amber-400" : "text-zinc-300")} />
                    ))}
                  </div>
                  {selected.feedback.comment ? <p className="mt-2 leading-5 text-[#657084]">{selected.feedback.comment}</p> : null}
                </div>
              ) : null}
              {selectedNeedsFeedback ? (
                <div className="space-y-2">
                  <div className="flex gap-1" role="radiogroup" aria-label={t("portalExtract.customerMobile.ticket.rating")}>
                    {[1, 2, 3, 4, 5].map((value) => (
                      <button
                        key={value}
                        type="button"
                        role="radio"
                        aria-checked={rating === value}
                        className="flex size-10 items-center justify-center rounded-md active:bg-white"
                        onClick={() => setRating(value)}
                        aria-label={t("portalExtract.customerMobile.common.star", { value })}
                      >
                        <StarIcon className={cn("size-6", value <= rating ? "fill-amber-400 text-amber-400" : "text-zinc-300")} />
                      </button>
                    ))}
                  </div>
                  <Textarea
                    value={ratingComment}
                    onChange={(event) => setRatingComment(event.target.value)}
                    placeholder={t("portalExtract.customerMobile.ticket.optionalFeedback")}
                    className="min-h-20 border-[#dce2ea] bg-white text-sm shadow-none"
                  />
                </div>
              ) : null}
              {selectedCanEndTicket || selectedNeedsFeedback ? (
                <div className="mt-3 grid grid-cols-2 gap-2">
	                  <Button className={cn("h-11 bg-[#1769e0] shadow-none hover:bg-[#125fcf]", !selected.can_reopen && "col-span-2")} onClick={() => void submitResolutionAction()} disabled={actionLoading || (selectedNeedsFeedback && rating <= 0)}>
                    {actionLoading ? <Loader2Icon className="size-4 animate-spin" /> : <CheckCircle2Icon className="size-4" />}
                    {selectedCanEndTicket
                      ? selectedNeedsFeedback
                        ? t("portalExtract.customerMobile.ticket.rateAndEndTicket")
                        : t("portalExtract.customerMobile.ticket.endTicketNow")
                      : t("portalExtract.customerMobile.ticket.submitFeedback")}
                  </Button>
                  {selected.can_reopen ? (
                    <Button type="button" variant="outline" className="h-11 border-[#dce2ea] bg-white text-[#465266] shadow-none" onClick={() => setShowReopen((current) => !current)} disabled={actionLoading}>
                      {t("portalExtract.customerMobile.ticket.unresolved")}
                    </Button>
                  ) : null}
                </div>
              ) : selected.can_reopen ? (
                <Button type="button" variant="outline" className="mt-3 h-11 w-full border-[#dce2ea] bg-white text-[#465266] shadow-none" onClick={() => setShowReopen((current) => !current)} disabled={actionLoading}>
                  {t("portalExtract.customerMobile.ticket.unresolved")}
                </Button>
              ) : null}
              {showReopen ? (
                <form className="mt-3 space-y-2" onSubmit={(event) => { event.preventDefault(); void runTicketAction(() => reopenCustomerTicket(selected.id, reopenReason.trim()), t("portalExtract.customerMobile.ticket.reopened")); setShowReopen(false) }}>
                  <Textarea value={reopenReason} onChange={(event) => setReopenReason(event.target.value)} placeholder={t("portalExtract.customerMobile.ticket.reopenReasonPlaceholder")} className="min-h-20 border-[#dce2ea] bg-white text-sm shadow-none" required />
                  <Button type="submit" className="h-10 w-full" disabled={!reopenReason.trim() || actionLoading}>{t("portalExtract.customerMobile.ticket.submitAndReopen")}</Button>
                </form>
              ) : null}
            </section>
          ) : null}
          {notice ? <p className="rounded-md bg-zinc-50 px-3 py-2 text-center text-xs text-zinc-600">{notice}</p> : null}

          {selected.repair_summary ? <div><SectionLabel>{t("portalExtract.customerMobile.ticket.repairResult")}</SectionLabel><p className="rounded-lg bg-[#f4f6f8] px-3 py-3 text-sm leading-6 text-[#465266]">{selected.repair_summary}</p></div> : null}

          <div>
            <SectionLabel>{t("portalExtract.customerMobile.ticket.progress")}</SectionLabel>
            <div className="relative ml-2 border-l border-[#dce2ea] pl-5">
              {ticketDetailLoading && !selectedTicketDetail ? (
                <p className="flex items-center gap-2 py-4 text-xs text-[#939dab]" role="status" aria-live="polite">
                  <Loader2Icon className="size-3.5 animate-spin" />
                  {t("portalExtract.customerMobile.common.ticketDetailLoading")}
                </p>
              ) : ticketDetailError && !selectedTicketDetail ? (
                <p className="py-4 text-xs text-red-600">{ticketDetailError}</p>
              ) : selected.progress.length === 0 ? <p className="py-4 text-xs text-[#939dab]">{t("portalExtract.customerMobile.ticket.noProgress")}</p> : selected.progress.map((item) => (
                <div key={item.id} className="relative pb-5 last:pb-0">
                  <span className="absolute -left-[25px] top-1 size-2 rounded-full bg-[#1769e0] ring-4 ring-white" />
                  <p className="text-sm leading-5 text-[#303b4d]">{customerTicketProgressContent(t, item, { hasDeviceConcept })}</p>
                  <p className="mt-1 text-rhd-2xs text-[#939dab]">{formatDate(item.created_at, locale)}</p>
                </div>
              ))}
            </div>
          </div>

        </div>
      </div>
    )
  }

  return (
    <div className="min-h-full">
      <div className="sticky top-0 z-20 space-y-3 border-b border-[#e5e9ef] bg-[#f4f6f8]/96 px-4 py-3 backdrop-blur-xl">
        <div><SearchField allowClear className="rhd-railops-search-mobile" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t(hasDeviceConcept ? "portalExtract.customerMobile.common.searchTickets" : "portalExtract.customerMobile.common.searchTicketsGeneral")} /></div>
        <div className="grid grid-cols-3 rounded-lg bg-[#e3e7ec] p-1">{([["all", t("portalExtract.customerMobile.common.all")], ["open", t("portalExtract.customerMobile.status.processing")], ["closed", t("portalExtract.customerMobile.common.ended")]] as const).map(([value, label]) => <button key={value} type="button" className={cn("h-11 rounded-md text-xs font-medium text-[#737d8e]", filter === value && "bg-white text-[#172033] shadow-sm")} onClick={() => setFilter(value)}>{label}</button>)}</div>
      </div>
      {state.error && hasLoadedTickets ? <p className="mx-3 mt-3 rounded-md border border-[#fde2e2] bg-[#fff5f5] px-3 py-2 text-xs text-red-600">{state.error}</p> : null}
      {ticketRefreshing ? <InlineLoadingHint label={t("portalExtract.customerMobile.ticket.loadingTickets")} /> : null}
      <div className="space-y-2 px-3 pb-4">
        {ticketInitialLoading ? <MobileListLoading label={t("portalExtract.customerMobile.ticket.readingTickets")} rows={5} tone="indigo" toolbar="none" /> : visibleTickets.map((item) => (
            <button key={item.id} type="button" className="min-h-[96px] w-full rounded-lg border border-[#e3e7ed] bg-white px-4 py-3 text-left active:bg-[#f8f9fb]" onClick={() => setSelectedId(item.id)}>
              <span className="flex items-center justify-between gap-3"><strong className="truncate text-sm font-semibold text-[#172033]">{item.ticket_no}</strong><StatusPill status={item.status} /></span>
              <span className="mt-1.5 block truncate text-sm text-[#3f4a5d]">{renderMobileTicketTitle(item.title, t)}</span>
              <span className="mt-2 flex items-center justify-between gap-3 text-rhd-2xs text-[#939dab]">{hasDeviceConcept ? <span className="truncate font-mono">{item.device_no || t("portalExtract.customerMobile.common.noLinkedDevice")}</span> : <span />}<span className="shrink-0">{formatDate(item.updated_at, locale)}</span></span>
            </button>
          ))}
        {!ticketInitialLoading && ticketHasMore ? <Button type="button" variant="outline" size="sm" className="h-10 w-full border-[#dce2ea] bg-white text-xs text-[#465266] shadow-none" onClick={() => void load(ticketPage + 1, true)} disabled={state.loading}>{state.loading ? <Loader2Icon className="size-3.5 animate-spin" /> : null}{state.loading ? t("portalExtract.customerMobile.common.loading") : t("portalExtract.customerMobile.common.loadMore")}<span className="text-rhd-2xs font-normal text-[#939dab]">{t("portalExtract.customerMobile.common.shownCount", { shown: visibleTickets.length, total: ticketTotal })}</span></Button> : null}
        {!ticketInitialLoading && ticketTotal === 0 ? <EmptyState icon={<PackageSearchIcon className="size-5" />} title={filter !== "all" || query.trim() ? t("portalExtract.customerMobile.common.noMatchedTickets") : t("portalExtract.customerMobile.common.noTickets")} /> : null}
      </div>
    </div>
  )
}

export function MobileCustomerMeetingsPage() {
  const t = useI18n()
  const { locale } = useAppLocale()
  const { session } = useAuth()
  const searchParams = useSearchParams()
  const [state, setState] = useState<AsyncState>({ loading: true, error: "" })
  const [meetings, setMeetings] = useState<CustomerPortalMeeting[]>([])
  const [hasLoadedMeetings, setHasLoadedMeetings] = useState(false)
  const [filter, setFilter] = useState<"upcoming" | "history">("upcoming")
  const [meetingPage, setMeetingPage] = useState(1)
  const [meetingTotal, setMeetingTotal] = useState(0)
  const [selectedId, setSelectedId] = useState("")
  const [joinConfig, setJoinConfig] = useState<CustomerMeetingJoinConfig | null>(null)
  const [joining, setJoining] = useState(false)
  const [notice, setNotice] = useState("")
  const [transcriptOpen, setTranscriptOpen] = useState(false)
  const [transcriptItems, setTranscriptItems] = useState<MeetingTranscriptSegment[]>([])
  const [transcriptCursor, setTranscriptCursor] = useState("")
  const [transcriptHasMore, setTranscriptHasMore] = useState(false)
  const [transcriptState, setTranscriptState] = useState<AsyncState>({ loading: false, error: "" })

  const selected = meetings.find((item) => item.id === selectedId) ?? null
  const joinable = (status?: string) => status === "active" || status === "waiting" || status === "scheduled"

  const load = useCallback(async (nextPage = 1, append = false) => {
    setState({ loading: true, error: "" })
    try {
      const page = await fetchCustomerMeetingsPage({
        page: nextPage,
        limit: mobileListPageSize,
        filter,
      })
      const nextItems = page.results ?? []
      setMeetings((current) => {
        if (!append) return nextItems
        const seen = new Set(current.map((item) => item.id))
        return [...current, ...nextItems.filter((item) => !seen.has(item.id))]
      })
      setMeetingPage(nextPage)
      setMeetingTotal(page.page?.total ?? nextItems.length)
      setHasLoadedMeetings(true)
      setState({ loading: false, error: "" })
    } catch (error) {
        setState({ loading: false, error: error instanceof Error ? error.message : t("portalExtract.customerMobile.meeting.loadFailed") })
    }
  }, [filter])
  useEffect(() => { void load() }, [load])
  useEffect(() => {
    setTranscriptOpen(false)
    setTranscriptItems([])
    setTranscriptCursor("")
    setTranscriptHasMore(false)
    setTranscriptState({ loading: false, error: "" })
  }, [selectedId])
  useEffect(() => {
    const requestedMeetingId = searchParams.get("id")?.trim()
    if (!requestedMeetingId) return
    if (meetings.some((item) => item.id === requestedMeetingId)) {
      setSelectedId(requestedMeetingId)
      return
    }
    let cancelled = false
    void fetchCustomerMeetingsPage({ meetingId: requestedMeetingId, limit: 1 })
      .then((page) => {
        if (cancelled) return
        const meeting = page.results?.[0]
        if (!meeting) return
        setMeetings((current) => upsertMobileMeeting(current, meeting))
        setSelectedId(meeting.id)
      })
      .catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [meetings, searchParams])

  const visibleMeetings = meetings
  const meetingInitialLoading = state.loading && !hasLoadedMeetings
  const meetingHasMore = meetings.length < meetingTotal
  const meetingRefreshing = state.loading && hasLoadedMeetings

  async function joinMeeting() {
    if (!selected || joining) return
    setJoining(true)
    setNotice("")
    try {
      setJoinConfig(await fetchCustomerMeetingJoinConfig(selected.id))
    } catch (error) {
      setNotice(error instanceof Error ? error.message : t("portalExtract.customerMobile.meeting.joinUnavailable"))
    } finally {
      setJoining(false)
    }
  }

  async function loadMeetingTranscripts(meeting: CustomerPortalMeeting, cursor = "", append = false) {
    setTranscriptState({ loading: true, error: "" })
    try {
      const page = await fetchCustomerMeetingTranscriptPage(meeting.id, cursor)
      const nextItems = page.results ?? []
      setTranscriptItems((current) => append ? mergeTranscriptItems(current, nextItems) : nextItems)
      setTranscriptCursor(page.cursor ?? "")
      setTranscriptHasMore(Boolean(page.hasMore))
      setTranscriptState({ loading: false, error: "" })
    } catch (error) {
      setTranscriptState({
        loading: false,
        error: error instanceof Error ? error.message : t("portalExtract.customerMobile.meeting.transcriptLoadFailed"),
      })
    }
  }

  function openTranscriptRecords(meeting: CustomerPortalMeeting) {
    setTranscriptOpen(true)
    setTranscriptItems([])
    setTranscriptCursor("")
    setTranscriptHasMore(false)
    void loadMeetingTranscripts(meeting)
  }

  if (joinConfig && selected) {
    return (
      <div className="fixed inset-0 z-[80] flex flex-col bg-black text-white">
        <div className="flex h-[calc(52px+env(safe-area-inset-top))] items-end justify-between bg-zinc-950 px-3 pb-2 pt-[env(safe-area-inset-top)]">
          <div className="min-w-0"><p className="truncate text-sm font-medium">{selected.ticket_no || selected.title}</p>{session?.featureFlags?.device !== false ? <p className="truncate text-rhd-2xs text-white/55">{selected.device_no || selected.product_name}</p> : null}</div>
          <Button type="button" variant="ghost" size="icon" className="size-9 text-white hover:bg-white/10 hover:text-white" onClick={() => setJoinConfig(null)} aria-label={t("portalExtract.customerMobile.common.closeCollaborationPage")}><XIcon className="size-5" /></Button>
        </div>
        <MeetingLiveRoom
          domain={joinConfig.domain}
          jitsiUrl={joinConfig.jitsiUrl}
          meetingId={joinConfig.meetingId}
          roomName={joinConfig.roomName}
          subject={selected.ticket_no || selected.title || t("portalExtract.customerMobile.meeting.remoteCollaboration")}
          jwt={joinConfig.jwt}
          transcriptionEnabled={joinConfig.transcriptionEnabled ?? false}
          transcriptionProvider={joinConfig.transcriptionProvider}
          transcriptionReady={joinConfig.transcriptionReady ?? false}
          initialTranscripts={[]}
          fetchTranscripts={() => fetchCustomerMeetingTranscripts(joinConfig.meetingId)}
          fetchTranscriptPage={(cursor) => fetchCustomerMeetingTranscriptPage(joinConfig.meetingId, cursor)}
          transcriptionUnavailableReason={joinConfig.transcriptionError}
          arEnabled={joinConfig.arDetectionEnabled ?? false}
          customerInviteEnabled={false}
          mobileLayout
          displayName={session?.user?.nickname || session?.user?.username || t("portalExtract.customerMobile.common.customer")}
          onConferenceJoined={() => confirmCustomerMeetingJoined(joinConfig.meetingId)}
          onConferenceLeft={() => confirmCustomerMeetingLeft(joinConfig.meetingId)}
          onConferenceHeartbeat={() => heartbeatCustomerMeeting(joinConfig.meetingId)}
          onTranscriptFinal={(event) => ingestCustomerMeetingTranscript(joinConfig.meetingId, event)}
          fetchMeetingStatus={() => fetchCustomerMeetingStatus(joinConfig.meetingId)}
          onMeetingEnd={() => setJoinConfig(null)}
          className="min-h-0 flex-1 rounded-none border-0"
        />
      </div>
    )
  }

  if (state.error && !hasLoadedMeetings) return <PageError message={state.error} onRetry={() => void load()} />

  if (selected) {
    const selectedTitle = renderMobileTicketTitle(selected.title, t, t("portalExtract.customerMobile.meeting.afterSalesVideo"))
    return (
      <div className="min-h-full bg-white">
        <BackTitle title={selected.ticket_no || t("portalExtract.customerMobile.meeting.remoteCollaboration")} subtitle={selectedTitle} onBack={() => { setSelectedId(""); setNotice("") }} />
        <div className="px-4 py-6">
          <div className="flex items-start justify-between gap-3"><span className="flex size-11 items-center justify-center rounded-lg bg-[#eef3ff] text-[#3f60d7]"><VideoIcon className="size-5" /></span><StatusPill status={selected.status} /></div>
          <h2 className="mt-4 text-lg font-semibold leading-7 text-[#172033]">{selectedTitle}</h2>
          {session?.featureFlags?.device !== false ? <p className="mt-1 font-mono text-xs text-[#778195]">{selected.device_no || selected.product_name || t("portalExtract.customerMobile.common.noLinkedDevice")}</p> : null}
          <dl className="mt-5 divide-y divide-[#edf0f4] border-y border-[#edf0f4] text-sm">
            <div className="flex justify-between gap-4 py-3"><dt className="text-[#778195]">{t("portalExtract.customerMobile.meeting.scheduledAt")}</dt><dd className="text-right font-medium text-[#303b4d]">{formatDate(selected.scheduled_at || selected.started_at, locale)}</dd></div>
            <div className="flex justify-between gap-4 py-3"><dt className="text-[#778195]">{t("portalExtract.customerMobile.meeting.initiator")}</dt><dd className="text-right font-medium text-[#303b4d]">{selected.created_by || "--"}</dd></div>
            <div className="flex justify-between gap-4 py-3"><dt className="text-[#778195]">{t("portalExtract.customerMobile.common.participants")}</dt><dd className="text-right font-medium text-[#303b4d]">{t("portalExtract.customerMobile.time.people", { count: selected.participant_count || 0 })}</dd></div>
            <div className="flex justify-between gap-4 py-3"><dt className="text-[#778195]">{t("portalExtract.customerMobile.common.collaborationDuration")}</dt><dd className="text-right font-medium text-[#303b4d]">{formatDuration(selected.duration_seconds, t)}</dd></div>
            <div
              role="button"
              tabIndex={0}
              className="flex cursor-pointer items-center justify-between gap-4 py-3 text-left active:bg-[#f8f9fb]"
              onClick={() => openTranscriptRecords(selected)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault()
                  openTranscriptRecords(selected)
                }
              }}
            >
              <dt className="text-[#778195]">{t("portalExtract.customerMobile.common.textRecords")}</dt>
              <dd className="flex items-center gap-1 text-right font-medium text-[#303b4d]">{t("portalExtract.customerMobile.time.records", { count: selected.transcript_count || 0 })}<ChevronRightIcon className="size-4 text-[#b6bfcb]" /></dd>
            </div>
          </dl>
          {joinable(selected.status) ? <Button className="mt-5 h-11 w-full bg-[#1769e0] shadow-none hover:bg-[#125fcf]" onClick={() => void joinMeeting()} disabled={joining}>{joining ? <Loader2Icon className="size-4 animate-spin" /> : <VideoIcon className="size-4" />}{t("portalExtract.customerMobile.meeting.joinVideo")}</Button> : <div className="mt-5 rounded-lg bg-[#f4f6f8] px-3 py-3 text-center text-xs text-[#778195]">{t("portalExtract.customerMobile.common.archived")}</div>}
          {notice ? <p className="mt-3 text-center text-xs text-red-600">{notice}</p> : null}
          <Dialog open={transcriptOpen} onOpenChange={setTranscriptOpen}>
            <DialogContent
              className="rhd-mobile-dialog bottom-0 top-auto left-1/2 max-h-[82dvh] w-full max-w-lg -translate-x-1/2 translate-y-0 overflow-hidden rounded-b-none rounded-t-2xl p-0"
              showCloseButton
            >
              <DialogHeader className="border-b border-[#edf0f4] px-4 pb-3 pt-4 text-left">
                <div className="mx-auto mb-3 h-1 w-16 rounded-full bg-[#d8dee8]" />
                <DialogTitle className="flex items-center justify-between gap-3 text-base text-[#172033]">
                  <span>{t("portalExtract.customerMobile.meeting.transcriptTitle")}</span>
                  <span className="rounded-full bg-[#eef3ff] px-2 py-1 text-rhd-2xs font-medium text-[#1769e0]">
                    {t("portalExtract.customerMobile.time.records", { count: selected.transcript_count || transcriptItems.length || 0 })}
                  </span>
                </DialogTitle>
              </DialogHeader>
              <div className="max-h-[58dvh] overflow-y-auto px-4 py-3">
                {transcriptState.loading && transcriptItems.length === 0 ? (
                  <div className="flex min-h-32 items-center justify-center gap-2 text-sm text-[#657084]" role="status" aria-busy="true">
                    <Loader2Icon className="size-4 animate-spin" />
                    {t("portalExtract.customerMobile.meeting.transcriptLoading")}
                  </div>
                ) : transcriptState.error ? (
                  <div className="rounded-lg border border-[#fde2e2] bg-[#fff5f5] px-3 py-3 text-sm text-[#b83232]">
                    <p>{transcriptState.error}</p>
                    <Button type="button" variant="outline" size="sm" className="mt-3 h-9 w-full bg-white text-xs" onClick={() => void loadMeetingTranscripts(selected)}>{t("portalExtract.customerMobile.common.retry")}</Button>
                  </div>
                ) : transcriptItems.length === 0 ? (
                  <div className="rounded-lg border border-[#e4e9f0] bg-[#f8fafc] px-3 py-6 text-center text-sm text-[#657084]">
                    {t("portalExtract.customerMobile.meeting.noTranscripts")}
                  </div>
                ) : (
                  <div className="space-y-3">
                    {transcriptItems.map((item) => (
                      <article key={item.id || item.providerEventId} className="rounded-lg border border-[#e4e9f0] bg-white px-3 py-3">
                        <div className="flex items-start justify-between gap-3">
                          <strong className="min-w-0 truncate text-sm font-medium text-[#172033]">{item.speakerName || t("portalExtract.customerMobile.meeting.unknownSpeaker")}</strong>
                          <span className="shrink-0 text-rhd-2xs tabular-nums text-[#939dab]">{formatTranscriptTime(item, selected, locale)}</span>
                        </div>
                        <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-[#303b4d]">{item.text || item.translatedText || "--"}</p>
                        {item.translatedText && item.translatedText !== item.text ? (
                          <p className="mt-2 rounded-md border-l-2 border-[#1769e0]/30 bg-[#eef5ff] px-3 py-2 text-xs leading-5 text-[#465266]">{item.translatedText}</p>
                        ) : null}
                      </article>
                    ))}
                  </div>
                )}
                {transcriptHasMore && !transcriptState.error ? (
                  <Button type="button" variant="outline" size="sm" className="mt-3 h-10 w-full border-[#dce2ea] bg-white text-xs text-[#465266] shadow-none" onClick={() => void loadMeetingTranscripts(selected, transcriptCursor, true)} disabled={transcriptState.loading}>
                    {transcriptState.loading ? <Loader2Icon className="size-3.5 animate-spin" /> : null}
                    {transcriptState.loading ? t("portalExtract.customerMobile.common.loading") : t("portalExtract.customerMobile.common.loadMore")}
                  </Button>
                ) : null}
              </div>
            </DialogContent>
          </Dialog>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-full">
      <div className="sticky top-0 z-20 border-b border-[#e5e9ef] bg-[#f4f6f8]/96 px-4 py-3 backdrop-blur-xl"><div className="grid grid-cols-2 rounded-lg bg-[#e3e7ec] p-1">{([["upcoming", t("portalExtract.customerMobile.common.upcoming")], ["history", t("portalExtract.customerMobile.common.history")]] as const).map(([value, label]) => <button key={value} type="button" className={cn("h-11 rounded-md text-xs font-medium text-[#737d8e]", filter === value && "bg-white text-[#172033] shadow-sm")} onClick={() => setFilter(value)}>{label}</button>)}</div></div>
      {state.error && hasLoadedMeetings ? <p className="mx-3 mt-3 rounded-md border border-[#fde2e2] bg-[#fff5f5] px-3 py-2 text-xs text-red-600">{state.error}</p> : null}
      {meetingRefreshing ? <InlineLoadingHint label={t("portalExtract.customerMobile.meeting.loading")} /> : null}
      <div className="space-y-2 px-3 pb-4">
        {meetingInitialLoading ? <MobileListLoading label={t("portalExtract.customerMobile.meeting.reading")} rows={4} tone="violet" toolbar="none" /> : visibleMeetings.map((item) => <button key={item.id} type="button" className="min-h-[94px] w-full rounded-lg border border-[#e3e7ed] bg-white px-4 py-3 text-left active:bg-[#f8f9fb]" onClick={() => setSelectedId(item.id)}><span className="flex items-center justify-between gap-3"><strong className="truncate text-sm text-[#172033]">{item.ticket_no || t("portalExtract.customerMobile.meeting.remoteCollaboration")}</strong><StatusPill status={item.status} /></span><span className="mt-1.5 block truncate text-sm text-[#465266]">{renderMobileTicketTitle(item.title, t, session?.featureFlags?.device !== false ? item.device_no || item.product_name || t("portalExtract.customerMobile.common.noLinkedDevice") : t("portalExtract.customerMobile.meeting.afterSalesVideo"))}</span><span className="mt-2 flex items-center justify-between gap-3 text-rhd-2xs text-[#939dab]">{session?.featureFlags?.device !== false ? <span className="truncate font-mono">{item.device_no || item.product_name || t("portalExtract.customerMobile.common.noLinkedDevice")}</span> : <span />}<span className="shrink-0">{formatDate(item.scheduled_at || item.started_at, locale)}</span></span></button>)}
        {!meetingInitialLoading && meetingHasMore ? <Button type="button" variant="outline" size="sm" className="h-10 w-full border-[#dce2ea] bg-white text-xs text-[#465266] shadow-none" onClick={() => void load(meetingPage + 1, true)} disabled={state.loading}>{state.loading ? <Loader2Icon className="size-3.5 animate-spin" /> : null}{state.loading ? t("portalExtract.customerMobile.common.loading") : t("portalExtract.customerMobile.common.loadMore")}<span className="text-rhd-2xs font-normal text-[#939dab]">{t("portalExtract.customerMobile.common.shownCount", { shown: visibleMeetings.length, total: meetingTotal })}</span></Button> : null}
        {!meetingInitialLoading && meetingTotal === 0 ? <EmptyState icon={<CalendarClockIcon className="size-5" />} title={filter === "upcoming" ? t("portalExtract.customerMobile.meeting.noUpcoming") : t("portalExtract.customerMobile.meeting.noHistory")} /> : null}
      </div>
    </div>
  )
}

export function MobileCustomerDevicesPage({ navigate }: { navigate: MobileCustomerNavigate }) {
  const t = useI18n()
  const { locale } = useAppLocale()
  const router = useRouter()
  const searchParams = useSearchParams()
  const [state, setState] = useState<AsyncState>({ loading: true, error: "" })
  const [devices, setDevices] = useState<CustomerPortalDevice[]>([])
  const [hasLoadedDevices, setHasLoadedDevices] = useState(false)
  const [query, setQuery] = useState("")
  const [devicePage, setDevicePage] = useState(1)
  const [deviceTotal, setDeviceTotal] = useState(0)
  const [selectedId, setSelectedId] = useState(0)
  const [manuals, setManuals] = useState<CustomerPortalManualFile[]>([])
  const [manualsLoading, setManualsLoading] = useState(false)
  const selected = devices.find((item) => item.id === selectedId) ?? null
  const requestedDeviceId = Number(searchParams.get("deviceId") || "0")
  const returnConversationId = normalizeMobileConversationId(searchParams.get("fromConversationId"))

  useEffect(() => {
    if (requestedDeviceId > 0 && devices.some((item) => item.id === requestedDeviceId)) {
      setSelectedId(requestedDeviceId)
    } else if (requestedDeviceId <= 0) {
      setSelectedId(0)
    }
  }, [devices, requestedDeviceId])

  const load = useCallback(async (nextPage = 1, append = false) => {
    setState({ loading: true, error: "" })
    try {
      const page = await fetchCustomerDevicesPage({
        page: nextPage,
        limit: mobileListPageSize,
        keyword: query.trim(),
      })
      const nextItems = page.results ?? []
      setDevices((current) => {
        if (!append) return nextItems
        const seen = new Set(current.map((item) => item.id))
        return [...current, ...nextItems.filter((item) => !seen.has(item.id))]
      })
      setDevicePage(nextPage)
      setDeviceTotal(page.page?.total ?? nextItems.length)
      setHasLoadedDevices(true)
      setState({ loading: false, error: "" })
    } catch (error) {
      setState({ loading: false, error: error instanceof Error ? error.message : t("portalExtract.customerMobile.device.loadFailed") })
    }
  }, [query])
  useEffect(() => { void load() }, [load])
  useEffect(() => {
    if (!selectedId) { setManuals([]); return }
    setManualsLoading(true)
    fetchCustomerDeviceManuals(selectedId).then(setManuals).catch(() => setManuals([])).finally(() => setManualsLoading(false))
  }, [selectedId])

  const visibleDevices = devices
  const deviceInitialLoading = state.loading && !hasLoadedDevices
  const deviceHasMore = devices.length < deviceTotal
  const deviceRefreshing = state.loading && hasLoadedDevices

  if (state.error && !hasLoadedDevices) return <PageError message={state.error} onRetry={() => void load()} />

  function closeDeviceDetail() {
    setSelectedId(0)
    if (returnConversationId > 0) {
      navigate("chat", { conversationId: String(returnConversationId) })
      return
    }
    router.replace("/mobile?state=devices", { scroll: false })
  }

  if (selected) {
    return (
      <div className="min-h-full bg-white">
        <BackTitle title={selected.product_name || t("portalExtract.customerMobile.common.deviceArchive")} subtitle={selected.device_no} onBack={closeDeviceDetail} />
        <div className="px-4 py-5">
          <div className="flex items-start justify-between gap-3"><span className="flex size-11 items-center justify-center rounded-lg bg-[#e8f7f2] text-[#0f9f76]"><WrenchIcon className="size-5" /></span><StatusPill status={selected.status} /></div>
          <dl className="mt-4 divide-y divide-[#edf0f4] border-y border-[#edf0f4] text-sm">
            <div className="flex justify-between gap-4 py-3"><dt className="text-[#778195]">{t("portalExtract.customerMobile.common.model")}</dt><dd className="text-right font-medium text-[#303b4d]">{selected.model_name || "--"}</dd></div>
            <div className="flex justify-between gap-4 py-3"><dt className="text-[#778195]">{t("portalExtract.customerMobile.common.serialNo")}</dt><dd className="break-all text-right font-mono text-xs font-medium text-[#303b4d]">{selected.serial_no || "--"}</dd></div>
            <div className="flex justify-between gap-4 py-3"><dt className="text-[#778195]">{t("portalExtract.customerMobile.common.region")}</dt><dd className="text-right font-medium text-[#303b4d]">{selected.region_code || "--"}</dd></div>
            <div className="flex justify-between gap-4 py-3"><dt className="text-[#778195]">{t("portalExtract.customerMobile.common.warrantyEnd")}</dt><dd className="text-right font-medium text-[#303b4d]">{selected.warranty_end_at ? formatDate(selected.warranty_end_at, locale) : t("portalExtract.customerMobile.status.pendingConfirmation")}</dd></div>
          </dl>
          <div className="mt-4 grid grid-cols-3 divide-x divide-[#e3e7ed] rounded-lg bg-[#f4f6f8] py-3 text-center"><div><strong className="block text-base text-[#172033]">{selected.open_ticket_count}</strong><span className="text-rhd-2xs text-[#778195]">{t("portalExtract.customerMobile.common.inProgressTickets")}</span></div><div><strong className="block text-base text-[#172033]">{selected.conversation_count}</strong><span className="text-rhd-2xs text-[#778195]">{t("portalExtract.customerMobile.common.consultationRecords")}</span></div><div><strong className="block text-base text-[#172033]">{selected.repair_history_count}</strong><span className="text-rhd-2xs text-[#778195]">{t("portalExtract.customerMobile.common.repairRecords")}</span></div></div>
          <Button className="mt-4 h-11 w-full bg-[#1769e0] shadow-none hover:bg-[#125fcf]" onClick={() => navigate("chat", { deviceId: String(selected.id) })}><MessageCircleIcon className="size-4" />{t("portalExtract.customerMobile.device.consultDevice")}</Button>
          <div className="mt-5"><SectionLabel>{t("portalExtract.customerMobile.common.deviceData")}</SectionLabel>{manualsLoading ? <div className="space-y-2 border-y border-[#edf0f4] py-2" role="status" aria-busy="true" aria-label={t("portalExtract.customerMobile.device.readingFiles")}>{Array.from({ length: 3 }).map((_, index) => <div key={index} className="flex min-h-14 items-center gap-3 py-2"><LoadingBar className="size-4 shrink-0 rounded-md" /><span className="min-w-0 flex-1 space-y-2"><LoadingBar className="h-3 w-40 max-w-full" /><LoadingBar className="h-2.5 w-20" /></span></div>)}</div> : manuals.length ? <div className="divide-y divide-[#edf0f4] border-y border-[#edf0f4]">{manuals.map((item) => <a key={item.id} href={item.url} target="_blank" rel="noreferrer" className="flex min-h-14 items-center gap-3 py-2"><FileTextIcon className="size-4 shrink-0 text-[#1769e0]" /><span className="min-w-0 flex-1"><strong className="block truncate text-xs text-[#303b4d]">{item.title || item.filename}</strong><span className="mt-0.5 block text-rhd-2xs text-[#939dab]">{item.mime_type || t("portalExtract.customerMobile.common.document")}</span></span><ChevronRightIcon className="size-4 text-[#bdc5d0]" /></a>)}</div> : <div className="space-y-3 rounded-lg bg-[#f4f6f8] px-3 py-4 text-center"><p className="text-xs text-[#939dab]">{t("portalExtract.customerMobile.device.noFiles")}</p><Button className="h-11 w-full" variant="outline" onClick={() => navigate("chat", { deviceId: String(selected.id) })}><MessageCircleIcon className="size-4" />{t("portalExtract.customerMobile.device.consultDevice")}</Button></div>}</div>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-full">
      <div className="sticky top-0 z-20 flex gap-2 border-b border-[#e5e9ef] bg-[#f4f6f8]/96 px-4 py-3 backdrop-blur-xl"><div className="min-w-0 flex-1"><SearchField allowClear className="rhd-railops-search-mobile" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t("portalExtract.customerMobile.device.searchDevice")} /></div><Button type="button" size="icon" className="size-11 shrink-0 bg-[#1769e0] shadow-none hover:bg-[#125fcf]" onClick={() => navigate("scan")} aria-label={t("portalExtract.customerMobile.common.bindDevice")}><PlusIcon className="size-5" /></Button></div>
      {state.error && hasLoadedDevices ? <p className="mx-3 mt-3 rounded-md border border-[#fde2e2] bg-[#fff5f5] px-3 py-2 text-xs text-red-600">{state.error}</p> : null}
      {deviceRefreshing ? <InlineLoadingHint label={t("portalExtract.customerMobile.device.loading")} /> : null}
      <div className="space-y-2 px-3 pb-4">
        {deviceInitialLoading ? <MobileListLoading label={t("portalExtract.customerMobile.device.reading")} rows={5} tone="emerald" toolbar="none" /> : visibleDevices.map((item) => <button key={item.id} type="button" className="min-h-[96px] w-full rounded-lg border border-[#e3e7ed] bg-white px-4 py-3 text-left active:bg-[#f8f9fb]" onClick={() => { setSelectedId(item.id); router.replace(`/mobile?state=devices&deviceId=${item.id}`, { scroll: false }) }}><span className="flex items-center justify-between gap-3"><strong className="truncate text-sm text-[#172033]">{item.product_name || t("portalExtract.customerMobile.common.device")}</strong><StatusPill status={item.status} /></span><span className="mt-1.5 block truncate font-mono text-sm text-[#465266]">{item.device_no}</span><span className="mt-2 flex items-center justify-between gap-3 text-rhd-2xs text-[#939dab]"><span className="truncate">{item.model_name || item.serial_no || t("portalExtract.customerMobile.common.deviceArchive")}</span><ChevronRightIcon className="size-4 shrink-0 text-[#bdc5d0]" /></span></button>)}
        {!deviceInitialLoading && deviceHasMore ? <Button type="button" variant="outline" size="sm" className="h-10 w-full border-[#dce2ea] bg-white text-xs text-[#465266] shadow-none" onClick={() => void load(devicePage + 1, true)} disabled={state.loading}>{state.loading ? <Loader2Icon className="size-3.5 animate-spin" /> : null}{state.loading ? t("portalExtract.customerMobile.common.loading") : t("portalExtract.customerMobile.common.loadMore")}<span className="text-rhd-2xs font-normal text-[#939dab]">{t("portalExtract.customerMobile.common.shownCount", { shown: visibleDevices.length, total: deviceTotal })}</span></Button> : null}
        {!deviceInitialLoading && deviceTotal === 0 ? <EmptyState icon={<WrenchIcon className="size-5" />} title={t("portalExtract.customerMobile.device.notFound")} action={<Button size="sm" onClick={() => navigate("scan")}>{t("portalExtract.customerMobile.common.bindDevice")}</Button>} /> : null}
      </div>
    </div>
  )
}

export function MobileCustomerProfilePage() {
  const t = useI18n()
  const { locale } = useAppLocale()
  const { session, signOut } = useAuth()
  const hasDeviceConcept = session?.featureFlags?.device !== false
  const [state, setState] = useState<AsyncState>({ loading: true, error: "" })
  const [profile, setProfile] = useState<CustomerPortalProfile | null>(null)
  const [deletionStatus, setDeletionStatus] = useState<CustomerAccountDeletion | null>(null)
  const [deletionStatusLoading, setDeletionStatusLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [notice, setNotice] = useState("")
  const [deletionOpen, setDeletionOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [deletionError, setDeletionError] = useState("")
  const [currentPassword, setCurrentPassword] = useState("")
  const [confirmation, setConfirmation] = useState("")
  const [deletionAcknowledged, setDeletionAcknowledged] = useState(false)

  const loadDeletionStatus = useCallback(async () => {
    setDeletionStatusLoading(true)
    try {
      setDeletionStatus(await fetchCustomerAccountDeletionStatus())
    } catch {
      setDeletionStatus(null)
    } finally {
      setDeletionStatusLoading(false)
    }
  }, [])

  const load = useCallback(async () => {
    setState({ loading: true, error: "" })
    try {
      const profileResult = await fetchCustomerProfile()
      setProfile(profileResult)
      setState({ loading: false, error: "" })
      void loadDeletionStatus()
    } catch (error) {
      setState({ loading: false, error: error instanceof Error ? error.message : t("portalExtract.customerMobile.profile.loadFailed") })
    }
  }, [loadDeletionStatus])
  useEffect(() => { void load() }, [load])

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!profile || saving) return
    const form = new FormData(event.currentTarget)
    setSaving(true)
    setNotice("")
    try {
      setProfile(await updateCustomerProfile({
        name: form.get("name")?.toString().trim() || profile.name,
        primary_email: form.get("email")?.toString().trim(),
        primary_mobile: form.get("mobile")?.toString().trim(),
      }))
      setNotice(t("portalExtract.customerMobile.common.accountSaved"))
    } catch (error) {
      setNotice(error instanceof Error ? error.message : t("portalExtract.customerMobile.common.saveFailed"))
    } finally {
      setSaving(false)
    }
  }

  async function confirmAccountDeletion(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (deleting || confirmation !== "DELETE" || !deletionAcknowledged || !currentPassword) return
    setDeleting(true)
    setDeletionError("")
    try {
      const result = await deleteCustomerAccount({
        current_password: currentPassword,
        confirmation: "DELETE",
      })
      setDeletionStatus(result)
      setDeletionOpen(false)
      await signOut("/mobile?state=login&accountDeleted=1")
    } catch (error) {
      setDeletionError(error instanceof Error ? error.message : t("portalExtract.customerMobile.common.deleteAccountFailed"))
    } finally {
      setDeleting(false)
    }
  }

  function changeDeletionOpen(open: boolean) {
    if (deleting) return
    setDeletionOpen(open)
    if (!open) {
      setCurrentPassword("")
      setConfirmation("")
      setDeletionAcknowledged(false)
      setDeletionError("")
    }
  }

  if (state.error && !profile) return <PageError message={state.error} onRetry={() => void load()} />

  const metrics = profile ? [
    ...(hasDeviceConcept ? [[t("portalExtract.customerMobile.common.boundDevices"), profile.bound_device_count, WrenchIcon] as const] : []),
    [t("portalExtract.customerMobile.common.activeConversations"), profile.active_conversation_count, MessageCircleIcon],
    [t("portalExtract.customerMobile.common.inProgressTickets"), profile.open_ticket_count, ClipboardListIcon],
    [t("portalExtract.customerMobile.common.pendingMeetings"), profile.upcoming_meeting_count, VideoIcon],
  ] as const : []

  return (
    <div className="min-h-full bg-[#f4f6f8]">
      {!profile ? (
        <MobileProfileLoading />
      ) : (
        <>
          {state.error ? <p className="mx-3 mt-3 rounded-md border border-[#fde2e2] bg-[#fff5f5] px-3 py-2 text-xs text-red-600">{state.error}</p> : null}
          <div className="bg-[#172033] px-4 py-5 text-white">
            <div className="flex items-center gap-3"><span className="flex size-12 items-center justify-center rounded-full bg-white/10 text-white"><CircleUserRoundIcon className="size-7" /></span><div className="min-w-0"><h2 className="truncate text-lg font-semibold">{profile.name}</h2><p className="mt-0.5 truncate text-xs text-white/60">{profile.company_name || t("portalExtract.customerMobile.common.customerAccount")}</p></div></div>
          </div>
          <div className="grid divide-x divide-[#edf0f4] bg-white px-2 py-4" style={{ gridTemplateColumns: `repeat(${metrics.length}, minmax(0, 1fr))` }}>{metrics.map(([label, value, Icon]) => <div key={label} className="min-w-0 px-1 text-center"><Icon className="mx-auto size-4 text-[#8b96a8]" /><strong className="mt-1.5 block text-lg text-[#172033]">{value}</strong><p className="mt-0.5 min-h-8 text-rhd-2xs leading-4 text-[#778195]">{label}</p></div>)}</div>
          <form className="mt-3 bg-white px-4 py-5" onSubmit={save}>
            <SectionLabel>{t("portalExtract.customerMobile.profile.basicInfo")}</SectionLabel>
            <div className="space-y-3">
              <div className="space-y-1.5"><Label htmlFor="mobile-profile-name" className="text-xs text-[#657084]">{t("portalExtract.customerMobile.common.name")}</Label><Input id="mobile-profile-name" name="name" defaultValue={profile.name} className="h-11 border-[#dce2ea] bg-[#f8f9fb] text-sm shadow-none" required /></div>
              <div className="space-y-1.5"><Label htmlFor="mobile-profile-company" className="text-xs text-[#657084]">{t("portalExtract.customerMobile.common.customerOrg")}</Label><Input id="mobile-profile-company" value={profile.company_name || "--"} className="h-11 border-[#e7ebf0] bg-[#f4f6f8] text-sm shadow-none" readOnly /></div>
              <div className="space-y-1.5"><Label htmlFor="mobile-profile-email" className="text-xs text-[#657084]">{t("portalExtract.customerMobile.common.email")}</Label><Input id="mobile-profile-email" name="email" type="email" defaultValue={profile.primary_email} className="h-11 border-[#dce2ea] bg-[#f8f9fb] text-sm shadow-none" /></div>
            <div className="space-y-1.5"><Label htmlFor="mobile-profile-mobile" className="text-xs text-[#657084]">{t("portalExtract.customerMobile.profile.mobile")}</Label><Input id="mobile-profile-mobile" name="mobile" inputMode="tel" defaultValue={profile.primary_mobile} className="h-11 border-[#dce2ea] bg-[#f8f9fb] text-sm shadow-none" /></div>
          </div>
          <Button type="submit" className="mt-4 h-11 w-full bg-[#1769e0] shadow-none hover:bg-[#125fcf]" disabled={saving}>{saving ? <Loader2Icon className="size-4 animate-spin" /> : <BadgeCheckIcon className="size-4" />}{t("portalExtract.customerMobile.common.saveProfile")}</Button>
          {notice ? <p className="mt-2 text-center text-xs text-[#657084]">{notice}</p> : null}
          </form>
          <section className="mt-3 bg-white px-4 py-5">
            <SectionLabel>{t("portalExtract.customerMobile.profile.accountPrivacy")}</SectionLabel>
            <div className="divide-y divide-[#edf0f4] border-y border-[#edf0f4]">
              <a href="/legal/privacy" target="_blank" rel="noreferrer" className="flex min-h-14 items-center gap-3 py-2 text-left">
                <ShieldCheckIcon className="size-4 shrink-0 text-[#1769e0]" />
                <span className="min-w-0 flex-1"><strong className="block text-sm font-medium text-[#303b4d]">{t("portalExtract.customerMobile.common.privacyPolicy")}</strong><span className="mt-0.5 block text-rhd-2xs text-[#8b96a8]">{t("portalExtract.customerMobile.common.privacyPolicyDesc")}</span></span>
                <ChevronRightIcon className="size-4 shrink-0 text-[#bdc5d0]" />
              </a>
              <button type="button" className="flex min-h-14 w-full items-center gap-3 py-2 text-left" onClick={() => changeDeletionOpen(true)}>
                <Trash2Icon className="size-4 shrink-0 text-[#c43d3d]" />
                <span className="min-w-0 flex-1"><strong className="block text-sm font-medium text-[#b83232]">{t("portalExtract.customerMobile.common.deleteAccount")}</strong><span className="mt-0.5 block text-rhd-2xs text-[#8b96a8]">{t("portalExtract.customerMobile.common.deleteAccountDesc")}</span></span>
                <ChevronRightIcon className="size-4 shrink-0 text-[#bdc5d0]" />
              </button>
            </div>
            {deletionStatusLoading ? (
              <p className="mt-2 text-xs text-[#657084]">{t("portalExtract.customerMobile.common.deleteAccountLoading")}</p>
            ) : deletionStatus && deletionStatus.status !== "not_requested" ? (
              <p className="mt-2 text-xs text-[#657084]">{t("portalExtract.customerMobile.common.deleteAccountStatus", { status: statusLabel(deletionStatus.status, t) })}</p>
            ) : null}
          </section>
          <Dialog open={deletionOpen} onOpenChange={changeDeletionOpen}>
            <DialogContent className="rhd-mobile-dialog w-[calc(100%-32px)] max-w-sm rounded-lg p-5" showCloseButton={!deleting}>
              <DialogHeader>
                <DialogTitle className="text-[#a62f2f]">{t("portalExtract.customerMobile.common.deleteForever")}</DialogTitle>
                <p className="text-xs leading-5 text-[#657084]">{t("portalExtract.customerMobile.common.deleteAccountNotice")}</p>
              </DialogHeader>
              <form id="mobile-delete-account" className="space-y-4" onSubmit={confirmAccountDeletion}>
                <div className="space-y-1.5">
                  <Label htmlFor="mobile-delete-password">{t("portalExtract.customerMobile.common.currentPassword")}</Label>
                  <Input id="mobile-delete-password" type="password" autoComplete="current-password" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} disabled={deleting} required />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="mobile-delete-confirmation">{t("portalExtract.customerMobile.common.enterDeleteConfirm")}</Label>
                  <Input id="mobile-delete-confirmation" value={confirmation} onChange={(event) => setConfirmation(event.target.value)} autoCapitalize="characters" autoCorrect="off" spellCheck={false} disabled={deleting} required />
                </div>
                <label className="flex items-start gap-3 text-xs leading-5 text-[#4f5b6d]">
                  <AntCheckbox checked={deletionAcknowledged} onChange={(event) => setDeletionAcknowledged(event.target.checked)} disabled={deleting} aria-label={t("portalExtract.customerMobile.common.confirmDeleteConsequence")} />
                  <span>{t("portalExtract.customerMobile.common.deleteAcknowledge")}</span>
                </label>
                {deletionError ? <p className="text-xs text-[#b83232]" role="alert">{deletionError}</p> : null}
              </form>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => changeDeletionOpen(false)} disabled={deleting}>{t("portalExtract.customerMobile.common.cancel")}</Button>
                <Button type="submit" form="mobile-delete-account" variant="destructive" disabled={deleting || !currentPassword || confirmation !== "DELETE" || !deletionAcknowledged}>{deleting ? <Loader2Icon className="size-4 animate-spin" /> : <Trash2Icon className="size-4" />}{t("portalExtract.customerMobile.common.deleteForever")}</Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </>
      )}
    </div>
  )
}

"use client"

import {
  BellRingIcon,
  BriefcaseBusinessIcon,
  CalendarClockIcon,
  CheckCircle2Icon,
  ChevronDownIcon,
  ChevronRightIcon,
  CircleAlertIcon,
  CircleDashedIcon,
  ClipboardListIcon,
  Clock3Icon,
  Loader2Icon,
  PauseCircleIcon,
  PowerIcon,
  UserCheckIcon,
  UsersIcon,
  VideoIcon,
  WrenchIcon,
  type LucideIcon,
} from "lucide-react"
import Link from "next/link"
import { useCallback, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"

import { SelectField, StandardModal } from "@railops/ui"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useI18n } from "@/i18n/provider"
import { fetchEngineerBriefing, updateEngineerWorkStatus, type EngineerBriefing, type EngineerWorkStatus } from "@/lib/api/engineer-status"
import type { AuthSession } from "@/lib/auth"
import type { TicketListItem } from "@/lib/api/types"
import { buildEnterpriseTicketWorkbenchPathFromItem } from "@/lib/ticket-workbench-route"
import { formatDateTime } from "@/lib/utils"

const engineerBriefingI18nPrefix = "engineerExtract.engineerBriefing."
type EngineerBriefingT = ReturnType<typeof useI18n>
const eb = (t: EngineerBriefingT, key: string, values?: Record<string, string | number>) => t(`${engineerBriefingI18nPrefix}${key}`, values)

type WorkStatusOption = {
  value: EngineerWorkStatus
  labelKey: string
  hintKey: string
  dispatch: boolean
  icon: LucideIcon
  tone: string
  panelTone: string
}

const STATUS_OPTIONS: WorkStatusOption[] = [
  { value: "available", labelKey: "statusOptions.available", hintKey: "statusOptions.availableHint", dispatch: true, icon: CheckCircle2Icon, tone: "text-primary", panelTone: "border-primary/20 bg-primary/10 text-primary" },
  { value: "busy", labelKey: "statusOptions.busy", hintKey: "statusOptions.busyHint", dispatch: false, icon: PauseCircleIcon, tone: "text-muted-foreground", panelTone: "border-border bg-muted text-muted-foreground" },
  { value: "leave", labelKey: "statusOptions.leave", hintKey: "statusOptions.leaveHint", dispatch: false, icon: CalendarClockIcon, tone: "text-primary", panelTone: "border-primary/20 bg-primary/10 text-primary" },
  { value: "offline", labelKey: "statusOptions.offline", hintKey: "statusOptions.offlineHint", dispatch: false, icon: PowerIcon, tone: "text-muted-foreground", panelTone: "border-border bg-muted text-muted-foreground" },
  { value: "custom", labelKey: "statusOptions.custom", hintKey: "statusOptions.customHint", dispatch: false, icon: CircleDashedIcon, tone: "text-muted-foreground", panelTone: "border-border bg-muted text-muted-foreground" },
]

const STATUS_POLL_INTERVAL_MS = 30 * 60 * 1000

type StatusDialogMode = "login" | "reminder" | "edit"

type TicketStatusMeta = {
  labelKey: string
  icon: LucideIcon
  tone: string
}

function StatusPlaceholderButton({
  label,
  loading,
  t,
  title,
}: {
  label: string
  loading?: boolean
  t: EngineerBriefingT
  title: string
}) {
  return (
    <Button
      variant="outline"
      size="sm"
      className="h-8 shrink-0 gap-1.5 border-border bg-background px-2.5 text-muted-foreground"
      disabled
      title={title}
      aria-label={title}
    >
      {loading ? <Loader2Icon className="size-4 animate-spin" /> : <CircleAlertIcon className="size-4 text-muted-foreground" />}
      <span className="hidden sm:inline">{eb(t, "workStatusButton.label")}</span>
      <span className="font-semibold">{label}</span>
    </Button>
  )
}

const DEFAULT_TICKET_STATUS: TicketStatusMeta = {
  labelKey: "badgeLabels.default",
  icon: Clock3Icon,
  tone: "border-border bg-muted text-muted-foreground",
}

const TICKET_STATUS: Record<string, TicketStatusMeta> = {
  pending: DEFAULT_TICKET_STATUS,
  pending_acceptance: { labelKey: "badgeLabels.pendingAcceptance", icon: CircleAlertIcon, tone: "border-border bg-muted text-muted-foreground" },
  pending_dispatch: { labelKey: "badgeLabels.pendingDispatch", icon: UsersIcon, tone: "border-border bg-muted text-muted-foreground" },
  pending_assignee_accept: { labelKey: "badgeLabels.pendingAssigneeAccept", icon: UserCheckIcon, tone: "border-primary/20 bg-primary/10 text-primary" },
  accepted: { labelKey: "badgeLabels.accepted", icon: CheckCircle2Icon, tone: "border-primary/20 bg-primary/10 text-primary" },
  assigned: { labelKey: "badgeLabels.assigned", icon: UserCheckIcon, tone: "border-primary/20 bg-primary/10 text-primary" },
  in_progress: { labelKey: "badgeLabels.inProgress", icon: WrenchIcon, tone: "border-primary/20 bg-primary/10 text-primary" },
  processing: { labelKey: "badgeLabels.processing", icon: WrenchIcon, tone: "border-primary/20 bg-primary/10 text-primary" },
  video_support: { labelKey: "badgeLabels.videoSupport", icon: VideoIcon, tone: "border-primary/20 bg-primary/10 text-primary" },
  supplier_support: { labelKey: "badgeLabels.supplierSupport", icon: UsersIcon, tone: "border-border bg-muted text-muted-foreground" },
  resolved: { labelKey: "badgeLabels.resolved", icon: Clock3Icon, tone: "border-border bg-muted text-muted-foreground" },
  pending_customer_confirm: { labelKey: "badgeLabels.pendingCustomerConfirm", icon: Clock3Icon, tone: "border-border bg-muted text-muted-foreground" },
  reopened: { labelKey: "badgeLabels.reopened", icon: CircleAlertIcon, tone: "border-destructive/10 bg-destructive/10 text-destructive" },
}

function briefingSessionKey(session: AuthSession) {
  return `rhd:engineer-briefing:${session.user?.id || 0}:${session.expiresAt || "session"}`
}

function toLocalDateTime(value?: string) {
  if (!value) return ""
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ""
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}

function ticketAgeLabel(value: string | undefined, t: EngineerBriefingT) {
  if (!value) return eb(t, "waitTime.unknown")
  const createdAt = new Date(value).getTime()
  if (!Number.isFinite(createdAt)) return eb(t, "waitTime.unknown")
  const minutes = Math.max(0, Math.floor((Date.now() - createdAt) / 60_000))
  if (minutes < 1) return eb(t, "waitTime.justCreated")
  if (minutes < 60) return eb(t, "waitTime.minutes", { minutes })
  const hours = Math.floor(minutes / 60)
  if (hours < 48) return eb(t, "waitTime.hours", { hours })
  return eb(t, "waitTime.days", { days: Math.floor(hours / 24) })
}

function dispatchFailureReasonLabel(reason: string | null | undefined, t: EngineerBriefingT) {
  const normalized = String(reason || "").trim()
  const map: Record<string, string> = {
    accept_timeout_redispatching: "dispatchFailReasons.acceptTimeoutRedispatching",
    all_candidates_at_capacity: "dispatchFailReasons.allCandidatesAtCapacity",
    candidate_became_unavailable: "dispatchFailReasons.candidateBecameUnavailable",
    missing_supervisor: "dispatchFailReasons.missingSupervisor",
    no_active_schedule_team: "dispatchFailReasons.noActiveScheduleTeam",
    no_alternative_after_accept_timeout: "dispatchFailReasons.noAlternativeAfterAcceptTimeout",
    no_matched_profile: "dispatchFailReasons.noMatchedProfile",
    no_product_repair_team: "dispatchFailReasons.noProductRepairTeam",
    no_profile_for_enabled_user: "dispatchFailReasons.noProfileForEnabledUser",
    no_reachable_user: "dispatchFailReasons.noReachableUser",
  }
  const key = map[normalized]
  return key ? eb(t, key) : normalized
}

function ticketDispatchAgingLabel(ticket: TicketListItem, t: EngineerBriefingT) {
  const reason = dispatchFailureReasonLabel(ticket.last_dispatch_failure_reason, t)
  if (reason) return reason
  if (ticket.dispatch_deferred_until) return eb(t, "dispatchStatus.waitingRetry")
  if (ticket.dispatch_attempts > 0) return eb(t, "dispatchStatus.redispatchCount", { attempts: ticket.dispatch_attempts })
  return ""
}

function TicketRows({
  emptyText,
  hasProductConcept,
  items,
  onNavigate,
  t,
}: {
  emptyText: string
  hasProductConcept: boolean
  items: TicketListItem[]
  onNavigate?: () => void
  t: EngineerBriefingT
}) {
  if (items.length === 0) {
    return <div className="py-6 text-center text-xs text-muted-foreground">{emptyText}</div>
  }
  return (
    <div className="max-h-[330px] divide-y divide-border overflow-y-auto">
      {items.map((item) => {
        const knownMeta = TICKET_STATUS[item.status]
        const meta = knownMeta || DEFAULT_TICKET_STATUS
        const StatusIcon = meta.icon
        const dispatchLabel = ticketDispatchAgingLabel(item, t)
        const badgeLabel = knownMeta ? eb(t, knownMeta.labelKey) : item.status || eb(t, DEFAULT_TICKET_STATUS.labelKey)
        const ticketHref = buildEnterpriseTicketWorkbenchPathFromItem(item)
        return (
          <Link key={item.id || item.ticket_no} href={ticketHref} onClick={onNavigate} className="group grid grid-cols-[minmax(0,1fr)_auto] gap-3 px-4 py-3 text-sm no-underline transition-colors hover:bg-muted/60">
            <span className="min-w-0">
              <span className="flex min-w-0 items-baseline gap-2">
                <span className="shrink-0 font-mono text-xs font-semibold text-primary">{item.ticket_no}</span>
                <span className="min-w-0 truncate font-medium text-foreground">{item.title}</span>
              </span>
              <span className="mt-1 block truncate text-xs text-muted-foreground">{item.product_name || item.team_name || eb(t, hasProductConcept ? "ticketRows.noProductTeam" : "ticketRows.noSupportTeam")} · {ticketAgeLabel(item.created_at, t)} · {formatDateTime(item.created_at)} {eb(t, "ticketRows.created")}</span>
              {dispatchLabel ? (
                <span className="mt-1 inline-flex max-w-full items-center gap-1 rounded border border-border bg-muted px-1.5 py-0.5 text-rhd-xs font-medium text-foreground">
                  <CircleAlertIcon className="size-3 shrink-0 text-primary" />
                  <span className="truncate">{dispatchLabel}</span>
                </span>
              ) : null}
            </span>
            <span className="flex items-center gap-1.5 self-start">
              <Badge variant="outline" className={meta.tone}><StatusIcon />{badgeLabel}</Badge>
              <ChevronRightIcon className="size-4 text-muted-foreground/60 transition-transform group-hover:translate-x-0.5 group-hover:text-muted-foreground" />
            </span>
          </Link>
        )
      })}
    </div>
  )
}

function TicketQueue({
  count,
  emptyText,
  hasProductConcept,
  icon: Icon,
  items,
  onNavigate,
  t,
  title,
  tone,
}: {
  count: number
  emptyText: string
  hasProductConcept: boolean
  icon: LucideIcon
  items: TicketListItem[]
  onNavigate?: () => void
  t: EngineerBriefingT
  title: string
  tone: string
}) {
  return (
    <section className="overflow-hidden rounded-lg border border-border bg-card">
      <div className="flex min-h-12 items-center justify-between gap-3 border-b border-border bg-muted/60 px-4 py-2.5">
        <div className="flex items-center gap-2 font-semibold text-foreground"><Icon className={`size-4 ${tone}`} />{title}</div>
        <Badge variant={count ? "secondary" : "outline"}>{count}</Badge>
      </div>
      <TicketRows items={items} emptyText={emptyText} hasProductConcept={hasProductConcept} onNavigate={onNavigate} t={t} />
    </section>
  )
}

export function EngineerLoginBriefingModal({ autoOpen = true, session }: { autoOpen?: boolean; session: AuthSession }) {
  const [briefing, setBriefing] = useState<EngineerBriefing | null>(null)
  const [open, setOpen] = useState(false)
  const [dialogMode, setDialogMode] = useState<StatusDialogMode>("login")
  const [loading, setLoading] = useState(false)
  const [loadError, setLoadError] = useState("")
  const [saving, setSaving] = useState(false)
  const [status, setStatus] = useState<EngineerWorkStatus>("available")
  const [note, setNote] = useState("")
  const [availableAt, setAvailableAt] = useState("")
  const [reminderTick, setReminderTick] = useState(0)
  const t = useI18n()
  const isEmployeePortalMode = session.supportMode === "employee_portal"
  const hasProductConcept = session.featureFlags?.product !== false
  const draftStatusMeta = useMemo(() => STATUS_OPTIONS.find((item) => item.value === status) || STATUS_OPTIONS[0], [status])
  const currentStatusMeta = useMemo(
    () => STATUS_OPTIONS.find((item) => item.value === briefing?.workStatus.status) || STATUS_OPTIONS[0],
    [briefing?.workStatus.status],
  )
  const DraftStatusIcon = draftStatusMeta.icon
  const CurrentStatusIcon = currentStatusMeta.icon
  const draftStatusLabel = eb(t, draftStatusMeta.labelKey)
  const currentStatusLabel = eb(t, currentStatusMeta.labelKey)
  const isEditMode = dialogMode === "edit"
  const showModalDialog = open
  const recoveryDue = Boolean(
    briefing?.workStatus.availableAt &&
      briefing.workStatus.status !== "available" &&
      new Date(briefing.workStatus.availableAt).getTime() <= Date.now(),
  )
  const requiresAvailableConfirmation = Boolean(briefing?.workStatus.needsConfirmation || recoveryDue)

  const loadStatusDraft = useCallback((workStatus: EngineerBriefing["workStatus"]) => {
    setStatus(workStatus.status || "available")
    setNote(workStatus.note || "")
    setAvailableAt(toLocalDateTime(workStatus.availableAt))
  }, [])

  useEffect(() => {
    if (session.domainType !== "enterprise") return
    const key = briefingSessionKey(session)
    const shouldOpen = window.sessionStorage.getItem(key) !== "confirmed"
    let active = true
    setLoading(true)
    setLoadError("")
    void fetchEngineerBriefing()
      .then((result) => {
        if (!active) return
        setLoadError("")
        setBriefing(result)
        if (!result.isEngineer) return
        loadStatusDraft(result.workStatus)
        const recoveryDueAtLoad = Boolean(
          result.workStatus.availableAt &&
            result.workStatus.status !== "available" &&
            new Date(result.workStatus.availableAt).getTime() <= Date.now(),
        )
        const needsAttention = result.workStatus.needsConfirmation || recoveryDueAtLoad
        if (autoOpen && shouldOpen && needsAttention) {
          setDialogMode("reminder")
          setOpen(true)
        }
      })
      .catch((error) => {
        const message = error instanceof Error ? error.message : eb(t, "toasts.loadBriefingFailed")
        setLoadError(message)
        toast.error(message)
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => { active = false }
  }, [autoOpen, loadStatusDraft, session])

  useEffect(() => {
    if (session.domainType !== "enterprise") return
    let active = true
    const interval = window.setInterval(() => {
      void fetchEngineerBriefing()
        .then((result) => {
          if (!active) return
          setBriefing(result)
          if (!autoOpen || !result.isEngineer || !result.workStatus.needsConfirmation) return
          setDialogMode((current) => current === "edit" ? current : "reminder")
          setOpen(true)
        })
        .catch(() => undefined)
    }, STATUS_POLL_INTERVAL_MS)
    return () => {
      active = false
      window.clearInterval(interval)
    }
  }, [autoOpen, session.domainType, session.user?.id])

  useEffect(() => {
    if (!autoOpen) return
    const dueAt = briefing?.workStatus.availableAt
    if (!dueAt || briefing.workStatus.status === "available") return
    const delay = new Date(dueAt).getTime() - Date.now()
    if (delay <= 0) {
      setDialogMode((current) => current === "edit" ? current : "reminder")
      setOpen(true)
      return
    }
    const maxDelay = 2_147_000_000
    const timeout = window.setTimeout(() => {
      if (delay > maxDelay) {
        setReminderTick((value) => value + 1)
        return
      }
      setDialogMode((current) => current === "edit" ? current : "reminder")
      setOpen(true)
    }, Math.min(delay, maxDelay))
    return () => window.clearTimeout(timeout)
  }, [autoOpen, briefing, reminderTick])

  function openStatusEditor() {
    if (!briefing) return
    loadStatusDraft(briefing.workStatus)
    setDialogMode("edit")
    setOpen(true)
  }

  function closeStatusEditor() {
    setOpen(false)
    setDialogMode("login")
  }

  function acknowledgeBriefing() {
    window.sessionStorage.setItem(briefingSessionKey(session), "confirmed")
    setOpen(false)
    setDialogMode("login")
  }

  async function confirmAvailableStatus() {
    setSaving(true)
    try {
      const updated = await updateEngineerWorkStatus({ status: "available", note: "" })
      setBriefing((current) => current ? { ...current, workStatus: updated } : current)
      loadStatusDraft(updated)
      window.sessionStorage.setItem(briefingSessionKey(session), "confirmed")
      setOpen(false)
      setDialogMode("login")
      toast.success(eb(t, "toasts.confirmedAvailable"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : eb(t, "toasts.confirmStatusFailed"))
    } finally {
      setSaving(false)
    }
  }

  async function saveManualStatus() {
    if (!draftStatusMeta.dispatch && status === "custom" && !note.trim()) {
      toast.error(eb(t, "toasts.customStatusNeedsNote"))
      return
    }
    setSaving(true)
    try {
      const recovery = availableAt ? new Date(availableAt) : null
      if (recovery && recovery.getTime() <= Date.now()) {
        toast.error(eb(t, "toasts.recoveryMustBeFuture"))
        return
      }
      const updated = await updateEngineerWorkStatus({
        status,
        note: note.trim(),
        availableAt: recovery?.toISOString(),
      })
      setBriefing((current) => current ? { ...current, workStatus: updated } : current)
      loadStatusDraft(updated)
      closeStatusEditor()
      toast.success(draftStatusMeta.dispatch ? eb(t, "toasts.updatedDispatchable") : eb(t, "toasts.updatedPaused"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : eb(t, "toasts.updateFailed"))
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return isEmployeePortalMode ? <StatusPlaceholderButton loading label={eb(t, "workStatusButton.loadingLabel")} t={t} title={eb(t, "workStatusButton.loadingTitle")} /> : null
  }
  if (!briefing?.isEngineer) {
    if (!isEmployeePortalMode) return null
    return (
      <StatusPlaceholderButton
        label={loadError ? eb(t, "workStatusButton.unavailable") : eb(t, "workStatusButton.notConfigured")}
        t={t}
        title={loadError || eb(t, "workStatusButton.notConfiguredTitle")}
      />
    )
  }

  return (
    <>
      <Button
        variant="outline"
        size="sm"
        className={`h-8 shrink-0 gap-1.5 px-2.5 ${currentStatusMeta.panelTone}`}
        aria-label={eb(t, "workStatusButton.editAria", { label: currentStatusLabel })}
        title={eb(t, "workStatusButton.editTitle")}
        onClick={openStatusEditor}
      >
        <CurrentStatusIcon className={currentStatusMeta.tone} />
        <span className="hidden sm:inline">{eb(t, "workStatusButton.label")}</span>
        <span className="font-semibold">{currentStatusLabel}{currentStatusMeta.dispatch ? eb(t, "workStatusButton.acceptingSuffix") : eb(t, "workStatusButton.pausedSuffix")}</span>
        <ChevronDownIcon className="size-3.5 opacity-60" />
      </Button>
      <StandardModal
        open={showModalDialog}
        onCancel={() => {
          if (isEditMode) closeStatusEditor()
          else acknowledgeBriefing()
        }}
        width={1024}
        styles={{
          header: {
            marginBottom: 0,
            padding: "16px 20px",
            borderBottom: "1px solid color-mix(in oklab, var(--primary) 20%, transparent)",
            background: "color-mix(in oklab, var(--primary) 10%, transparent)",
          },
          body: { padding: 0, maxHeight: "92vh", display: "flex", flexDirection: "column", overflow: "hidden" },
          footer: { marginTop: 0, padding: 0 },
        }}
        title={(
          <div className="flex items-start gap-3">
            <div className="flex size-10 shrink-0 items-center justify-center rounded-lg border border-primary/20 bg-background text-primary shadow-sm"><BellRingIcon className="size-5" /></div>
            <div className="min-w-0 space-y-1">
              <div className="flex flex-wrap items-center gap-2">
                <span className="flex items-center gap-2 text-lg text-foreground"><BriefcaseBusinessIcon className="size-5 text-primary" />{isEditMode ? eb(t, "dialog.titleEdit") : dialogMode === "reminder" ? eb(t, "dialog.titleReminder") : eb(t, "dialog.titleLogin")}</span>
                <Badge variant="outline" className="border-primary/20 bg-background text-primary">{isEditMode ? eb(t, "dialog.badgeEdit") : dialogMode === "reminder" ? eb(t, "dialog.badgeReminder") : eb(t, "dialog.badgeLogin")}</Badge>
              </div>
            </div>
          </div>
        )}
        footer={(
          <div className="flex flex-col-reverse items-center gap-2 border-t border-[var(--railops-border-light)] bg-[var(--railops-surface)] px-5 py-4 sm:flex-row sm:justify-end sm:px-6">
            <div className="mr-auto hidden items-center gap-2 text-xs text-muted-foreground sm:flex">
              {isEditMode ? <DraftStatusIcon className={`size-4 ${draftStatusMeta.tone}`} /> : <CurrentStatusIcon className={`size-4 ${currentStatusMeta.tone}`} />}
              {isEditMode ? eb(t, "dialog.footerSavingAs", { label: draftStatusLabel }) : requiresAvailableConfirmation ? eb(t, "dialog.footerRecoverySuggestion") : eb(t, "dialog.footerKeeping", { label: currentStatusLabel })}
            </div>
            {isEditMode ? (
              <>
                <Button variant="outline" disabled={saving} onClick={closeStatusEditor}>{eb(t, "dialog.cancel")}</Button>
                <Button disabled={saving} onClick={() => void saveManualStatus()}>
                  {saving ? <Loader2Icon className="animate-spin" /> : <UserCheckIcon />}
                  {eb(t, "dialog.updateStatus")}
                </Button>
              </>
            ) : requiresAvailableConfirmation ? (
              <>
                <Button variant="outline" disabled={saving} onClick={acknowledgeBriefing}>{eb(t, "dialog.keepCurrentStatus")}</Button>
                <Button disabled={saving} onClick={() => void confirmAvailableStatus()}>
                  {saving ? <Loader2Icon className="animate-spin" /> : <UserCheckIcon />}
                  {eb(t, "dialog.recoverAndEnterOverview")}
                </Button>
              </>
            ) : (
              <Button onClick={acknowledgeBriefing}>
                <UserCheckIcon />
                {eb(t, "dialog.enterOverview")}
              </Button>
            )}
          </div>
        )}
      >

        <div className="grid min-h-0 overflow-y-auto lg:grid-cols-[280px_minmax(0,1fr)]">
          <section className="space-y-5 border-b border-border bg-muted/30 p-5 lg:border-r lg:border-b-0 sm:p-6">
            {isEditMode ? (
              <>
                <div className="grid gap-2">
                  <SelectField
                    label={eb(t, "statusEditor.statusLabel")}
                    layout="vertical"
                    className="w-full"
                    style={{ marginBottom: 0 }}
                    selectProps={{
                      value: status,
                      onChange: (value) => setStatus(value as EngineerWorkStatus),
                      disabled: saving,
                      labelRender: () => (
                        <span className="flex min-w-0 items-center gap-2">
                          <DraftStatusIcon className={`size-4 shrink-0 ${draftStatusMeta.tone}`} />
                          <span className="truncate font-medium text-foreground">{draftStatusLabel}</span>
                          <span className="shrink-0 text-xs text-muted-foreground">{draftStatusMeta.dispatch ? eb(t, "statusEditor.accepting") : eb(t, "statusEditor.paused")}</span>
                        </span>
                      ),
                      options: STATUS_OPTIONS.map((item) => {
                        const Icon = item.icon
                        return {
                          value: item.value,
                          label: (
                            <span className="flex min-w-0 items-center gap-2">
                              <Icon className={`size-4 shrink-0 ${item.tone}`} />
                              <span className="min-w-0 flex-1">
                                <span className="block truncate font-medium leading-5 text-foreground">{eb(t, item.labelKey)}</span>
                                <span className="block truncate text-xs leading-4 text-muted-foreground">{item.dispatch ? eb(t, "statusEditor.accepting") : eb(t, "statusEditor.paused")}</span>
                              </span>
                            </span>
                          ),
                        }
                      }),
                      style: { width: "100%" },
                    }}
                  />
                  <div className={`flex items-start gap-2 rounded-md border px-3 py-2 text-xs ${draftStatusMeta.panelTone}`}>
                    <DraftStatusIcon className="mt-0.5 size-3.5 shrink-0" />
                    <span>{eb(t, draftStatusMeta.hintKey)}</span>
                  </div>
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="engineer-status-note">{eb(t, "statusEditor.noteLabel")}</Label>
                  <Input id="engineer-status-note" value={note} maxLength={255} disabled={saving} placeholder={status === "leave" ? eb(t, "statusEditor.notePlaceholderLeave") : eb(t, "statusEditor.notePlaceholderGeneral")} onChange={(event) => setNote(event.target.value)} />
                </div>
                {!draftStatusMeta.dispatch ? (
                  <div className="grid gap-2">
                    <Label htmlFor="engineer-status-recovery">{eb(t, "statusEditor.recoveryLabel")}</Label>
                    <Input id="engineer-status-recovery" type="datetime-local" value={availableAt} disabled={saving} onChange={(event) => setAvailableAt(event.target.value)} />
                  </div>
                ) : null}
              </>
            ) : (
              <div className="space-y-3">
                <div className="space-y-1.5">
                  <Label>{eb(t, "dialog.currentStatus")}</Label>
                  <div className={`flex items-center gap-2 rounded-md border px-3 py-2.5 text-sm ${currentStatusMeta.panelTone}`}>
                    <CurrentStatusIcon className={`size-4 ${currentStatusMeta.tone}`} />
                    <span className="font-semibold">{currentStatusLabel}</span>
                    <span className="ml-auto text-xs opacity-75">{currentStatusMeta.dispatch ? eb(t, "statusEditor.accepting") : eb(t, "statusEditor.paused")}</span>
                  </div>
                </div>
	                {requiresAvailableConfirmation ? (
	                  <div className="rounded-lg border border-border bg-muted p-4 text-foreground">
	                    <div className="flex items-center gap-2 font-semibold"><CheckCircle2Icon className="size-5 text-primary" />{eb(t, "dialog.recoverySummary")}</div>
	                  </div>
	                ) : (
	                  <div className={`rounded-lg border p-4 ${currentStatusMeta.panelTone}`}>
	                    <div className="flex items-center gap-2 font-semibold"><CurrentStatusIcon className={`size-5 ${currentStatusMeta.tone}`} />{eb(t, "dialog.keepCurrentStatus")}</div>
	                  </div>
	                )}
              </div>
            )}
            <div className="space-y-2">
              <div className="flex items-center gap-2 text-xs font-semibold text-foreground"><UsersIcon className="size-4 text-primary" />{eb(t, hasProductConcept ? "teamLists.title" : "teamLists.supportTitle")}</div>
              <div className="space-y-1.5">
                {briefing.teams.length ? briefing.teams.map((team) => (
                  <div key={team.id} className="flex items-center justify-between gap-2 rounded-md border border-border bg-card px-3 py-2 text-sm">
                    <span className="min-w-0 truncate font-medium text-foreground">{team.name}</span>
                    <span className="shrink-0 text-rhd-xs text-muted-foreground">{team.productId ? eb(t, "teamLists.productTeam") : eb(t, "teamLists.supportTeam")}</span>
                  </div>
                )) : <div className="rounded-md border border-dashed border-border px-3 py-4 text-center text-xs text-muted-foreground">{eb(t, hasProductConcept ? "teamLists.empty" : "teamLists.supportEmpty")}</div>}
              </div>
            </div>
          </section>

          <div className="grid content-start gap-4 p-5 sm:p-6 xl:grid-cols-2">
            <TicketQueue
              count={briefing.unassignedTicketCount ?? briefing.unassignedTickets.length}
              emptyText={eb(t, "queues.unassignedEmpty")}
              hasProductConcept={hasProductConcept}
              icon={UsersIcon}
              items={briefing.unassignedTickets}
              onNavigate={acknowledgeBriefing}
              t={t}
              title={eb(t, hasProductConcept ? "queues.unassignedTitle" : "queues.supportUnassignedTitle")}
              tone="text-primary"
            />
            <TicketQueue
              count={briefing.myOpenTicketCount ?? briefing.myOpenTickets.length}
              emptyText={eb(t, "queues.myOpenEmpty")}
              hasProductConcept={hasProductConcept}
              icon={ClipboardListIcon}
              items={briefing.myOpenTickets}
              onNavigate={acknowledgeBriefing}
              t={t}
              title={eb(t, "queues.myOpenTitle")}
              tone="text-primary"
            />
          </div>
        </div>
      </StandardModal>
    </>
  )
}

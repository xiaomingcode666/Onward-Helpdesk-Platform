"use client"

import { Input, Skeleton } from "antd"
import { PageShell, RailopsButton, StandardModal, StatusTag } from "@railops/ui"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import {
  ActivityIcon,
  ArrowLeftIcon,
  CalendarOffIcon,
  CalendarClockIcon,
  CheckCircle2Icon,
  ListChecksIcon,
  PlusIcon,
  RefreshCwIcon,
  SendIcon,
  ShieldCheckIcon,
  UserCheckIcon,
  UsersRoundIcon,
  XCircleIcon,
} from "lucide-react"
import { type FormEvent, useCallback, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"

import { OptionCombobox } from "@/components/option-combobox"
import { useAuth } from "@/components/auth-provider"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { useI18n } from "@/i18n/provider"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import {
  fetchAgentTeamMemberAvailability,
  fetchAgentProfilesAll,
  fetchAgentTeamScheduleLeaves,
  fetchAgentTeamSchedules,
  fetchAgentTeamsAll,
  reviewAgentTeamScheduleLeave,
  type AdminAgentProfile,
  type AdminAgentScheduleException,
  type AdminAgentTeam,
  type AdminAgentTeamMemberAvailability,
  type AdminAgentTeamSchedule,
  type AdminAgentTeamScheduleTemplate,
} from "@/lib/api/admin"
import { cancelEngineerLeave, createEngineerLeave, fetchEngineerWorkSchedule } from "@/lib/api/engineer-status"
import { cn } from "@/lib/utils"
import { agentMemberName as memberName } from "../member-name"

const orgSchedulesI18nPrefix = "orgExtract.orgSchedules."
type OrgSchedulesT = ReturnType<typeof useI18n>
const os = (t: OrgSchedulesT, key: string, values?: Record<string, string | number>) => t(`${orgSchedulesI18nPrefix}${key}`, values)

const PRODUCT_TEAM_TYPE = "product_repair"
const WEEKDAYS = [
  { value: 1, labelKey: "weekday.1", shortLabelKey: "weekday.short1", weekend: false },
  { value: 2, labelKey: "weekday.2", shortLabelKey: "weekday.short2", weekend: false },
  { value: 3, labelKey: "weekday.3", shortLabelKey: "weekday.short3", weekend: false },
  { value: 4, labelKey: "weekday.4", shortLabelKey: "weekday.short4", weekend: false },
  { value: 5, labelKey: "weekday.5", shortLabelKey: "weekday.short5", weekend: false },
  { value: 6, labelKey: "weekday.6", shortLabelKey: "weekday.short6", weekend: true },
  { value: 7, labelKey: "weekday.7", shortLabelKey: "weekday.short7", weekend: true },
] as const

const ENGINEER_TIMEZONE = "Asia/Shanghai"

const DEFAULT_PERSONAL_BASE_SCHEDULE: AdminAgentTeamScheduleTemplate = {
  tenantId: 0,
  workdays: [1, 2, 3, 4, 5],
  startTime: "00:00",
  endTime: "24:00",
  timezone: ENGINEER_TIMEZONE,
}

function normalizePersonalBaseSchedule(value?: AdminAgentTeamScheduleTemplate): AdminAgentTeamScheduleTemplate {
  const workdays = Array.isArray(value?.workdays)
    ? [...new Set(value.workdays.filter((weekday) => weekday >= 1 && weekday <= 7))].sort((left, right) => left - right)
    : []
  if (!value || workdays.length === 0 || !value.startTime || !value.endTime) {
    return DEFAULT_PERSONAL_BASE_SCHEDULE
  }
  return {
    ...value,
    workdays,
    timezone: value.timezone || ENGINEER_TIMEZONE,
  }
}

function weekdayLabel(value: number, t: OrgSchedulesT) {
  const item = WEEKDAYS.find((entry) => entry.value === value)
  return item ? os(t, item.labelKey) : os(t, "weekday.fallback", { day: value })
}

function currentWeekday() {
  const value = new Date().getDay()
  return value === 0 ? 7 : value
}

function dateTimeInputValue(date: Date) {
  const shifted = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return shifted.toISOString().slice(0, 16)
}

function formatScheduleDateTime(value: string) {
  if (!value) return "-"
  return new Intl.DateTimeFormat("zh-CN", {
    timeZone: ENGINEER_TIMEZONE,
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(new Date(value))
}

function leaveStatusLabel(status: AdminAgentScheduleException["approvalStatus"], t: OrgSchedulesT) {
  if (status === "approved") return os(t, "leaveStatus.approved")
  if (status === "rejected") return os(t, "leaveStatus.rejected")
  if (status === "cancelled") return os(t, "leaveStatus.cancelled")
  return os(t, "leaveStatus.pending")
}

function activeLeaveForUser(items: AdminAgentScheduleException[], userId: number) {
  const now = Date.now()
  return items.find((item) =>
    item.userId === userId &&
    item.approvalStatus === "approved" &&
    new Date(item.startAt).getTime() <= now &&
    new Date(item.endAt).getTime() > now
  )
}

function workdayRangeLabel(workdays: number[], t: OrgSchedulesT) {
  const normalized = [...new Set(workdays)].sort((left, right) => left - right)
  if (normalized.join(",") === "1,2,3,4,5") return os(t, "workdayRange.workdays")
  if (normalized.join(",") === "1,2,3,4,5,6,7") return os(t, "workdayRange.everyDay")
  return normalized.map((weekday) => weekdayLabel(weekday, t)).join(os(t, "listSeparator")) || os(t, "workdayRange.unset")
}

function availabilityReasonLabel(item: AdminAgentTeamMemberAvailability, t: OrgSchedulesT) {
  if (item.activeLeave) return os(t, "reason.leave")
  switch (item.unavailableReason) {
    case "profile_missing":
      return os(t, "reason.profileMissing")
    case "team_dispatch_disabled":
      return os(t, "reason.teamDispatchDisabled")
    case "auto_assign_disabled":
      return os(t, "reason.autoAssignDisabled")
    case "service_busy":
      return os(t, "reason.serviceBusy")
    case "capacity_not_configured":
      return os(t, "reason.capacityNotConfigured")
    case "outside_personal_dispatch_rule":
      return os(t, "reason.outsidePersonalDispatchRule")
    case "approved_leave":
      return os(t, "reason.leave")
    case "work_status_unconfirmed":
      return os(t, "reason.workStatusUnconfirmed")
    case "work_status_unavailable":
      return os(t, "reason.workStatusUnavailable")
    case "not_reachable":
      return os(t, "reason.notReachable")
    default:
      return os(t, "reason.unavailable")
  }
}

function memberInitial(name: string, t: OrgSchedulesT) {
  return name.trim().slice(-2) || os(t, "memberInitialFallback")
}

function scheduleTime(item: AdminAgentTeamSchedule, t: OrgSchedulesT) {
  if (item.dayType === "rest") {
    return os(t, "scheduleTime.rest")
  }
  const overnight = item.endMinute < item.startMinute
  return `${item.startTime || "00:00"} - ${item.endTime || "24:00"}${overnight ? os(t, "scheduleTime.nextDay") : ""}`
}

function memberCapacityLabel(item: AdminAgentTeamMemberAvailability, t: OrgSchedulesT) {
  if (item.maxConcurrentCount <= 0) {
    return os(t, "capacity.noLimit", { weight: item.dispatchWeight || 1 })
  }
  return os(t, "capacity.concurrency", { weight: item.dispatchWeight || 1, count: item.maxConcurrentCount })
}

function leaveWindowLabel(item: AdminAgentScheduleException) {
  return `${formatScheduleDateTime(item.startAt)} - ${formatScheduleDateTime(item.endAt)}`
}

function scheduleScopeLabel(item: AdminAgentTeamSchedule, members: AdminAgentProfile[], t: OrgSchedulesT) {
  if (item.userId <= 0) {
    return os(t, "scope.teamWide")
  }
  const fallbackMember = members.find((member) => member.userId === item.userId)
  if (fallbackMember) {
    return memberName(fallbackMember, item.userName)
  }
  return item.userName || os(t, "scope.memberFallback", { id: item.userId })
}

type UnifiedPersonalScheduleWeekProps = {
  baseSchedule: AdminAgentTeamScheduleTemplate
  teams: AdminAgentTeam[]
  schedules: AdminAgentTeamSchedule[]
  draftSchedules: AdminAgentTeamSchedule[]
  t: OrgSchedulesT
}

type PersonalScheduleOverride = {
  item: AdminAgentTeamSchedule
  draft: boolean
}

function UnifiedPersonalScheduleWeek({
  baseSchedule,
  teams,
  schedules,
  draftSchedules,
  t,
}: UnifiedPersonalScheduleWeekProps) {
  const teamByID = new Map(teams.map((team) => [team.id, team]))
  const overridesByWeekday = new Map<number, PersonalScheduleOverride[]>()
  for (const weekday of WEEKDAYS) {
    overridesByWeekday.set(weekday.value, [])
  }
  for (const item of schedules) {
    if (item.repeatType === "weekly" && teamByID.get(item.teamId)?.scheduleEnforced) {
      overridesByWeekday.get(item.weekday)?.push({ item, draft: false })
    }
  }
  for (const item of draftSchedules) {
    if (item.repeatType === "weekly") {
      overridesByWeekday.get(item.weekday)?.push({ item, draft: true })
    }
  }
  for (const items of overridesByWeekday.values()) {
    items.sort((left, right) => {
      if (left.draft !== right.draft) return left.draft ? 1 : -1
      const teamOrder = (teamByID.get(left.item.teamId)?.productName || teamByID.get(left.item.teamId)?.name || "")
        .localeCompare(teamByID.get(right.item.teamId)?.productName || teamByID.get(right.item.teamId)?.name || "", "zh-CN")
      return teamOrder || (left.item.startMinute || 0) - (right.item.startMinute || 0) || left.item.id - right.item.id
    })
  }

  return (
    <div className="overflow-x-auto border" data-testid="my-unified-weekly-schedule">
      <div className="grid min-w-[980px] grid-cols-7">
        {WEEKDAYS.map((weekday) => {
          const dayOverrides = overridesByWeekday.get(weekday.value) || []
          const isBaseWorkday = baseSchedule.workdays.includes(weekday.value)
          const isToday = weekday.value === currentWeekday()
          return (
            <div key={weekday.value} className="min-h-40 border-r last:border-r-0">
              <div
                  className={cn(
                    "flex h-10 items-center justify-between border-b px-2.5",
                    weekday.weekend && "bg-amber-50/50 dark:bg-amber-950/20",
                    isToday && "bg-primary/10 dark:bg-primary/10",
                  )}
              >
                <span className="text-sm font-medium">{os(t, weekday.labelKey)}</span>
                <span className="text-xs text-muted-foreground">
                  {dayOverrides.length > 0 ? `+${dayOverrides.length}` : ""}
                </span>
              </div>
              <div className="space-y-1.5 p-2">
                <div
                  className={cn(
                    "border-l-2 px-2 py-1.5 text-xs",
                    isBaseWorkday
                      ? "border-primary bg-primary/10 dark:bg-primary/10"
                      : "border-muted-foreground/40 bg-muted/30",
                  )}
                >
                  <div className="font-medium tabular-nums">
                    {isBaseWorkday ? `${baseSchedule.startTime} - ${baseSchedule.endTime}` : os(t, "scheduleTime.rest")}
                  </div>
                  <div className="mt-1 flex flex-wrap items-center gap-1 text-muted-foreground">
                    <StatusTag tone="neutral">{os(t, "scope.enterpriseBase")}</StatusTag>
                    <span>{baseSchedule.timezone}</span>
                  </div>
                </div>
                {dayOverrides.map(({ item, draft }) => {
                  const team = teamByID.get(item.teamId)
                  const teamName = team?.productName || team?.name || item.teamName || os(t, "scope.teamFallback", { id: item.teamId })
                  return (
                    <div
                      key={`${draft ? "draft" : "published"}-${item.id}`}
                      className={cn(
                        "border-l-2 bg-muted/30 px-2 py-1.5 text-xs",
                        draft
                          ? "border-amber-500 bg-amber-50/50 dark:bg-amber-950/20"
                          : item.dayType === "rest"
                            ? "border-muted-foreground/40"
                            : "border-primary",
                      )}
                    >
                      <div className="truncate font-medium" title={teamName}>
                        {teamName}
                      </div>
                      <div className="font-medium tabular-nums">{scheduleTime(item, t)}</div>
                      <div className="mt-1 flex flex-wrap items-center gap-1 text-muted-foreground">
                        <StatusTag tone="neutral">
                          {draft ? os(t, "scope.draft") : os(t, "scope.overrideVersion", { version: item.version })}
                        </StatusTag>
                        <span>{item.userId === 0 ? os(t, "scope.teamWide") : os(t, "scope.onlyMe")}</span>
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

type LeaveRequestDialogProps = {
  open: boolean
  saving: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (payload: { requestKey: string; startAt: string; endAt: string; reason: string }) => Promise<void>
  t: OrgSchedulesT
}

function LeaveRequestDialog({ open, saving, onOpenChange, onSubmit, t }: LeaveRequestDialogProps) {
  const tomorrow = useMemo(() => {
    const value = new Date()
    value.setDate(value.getDate() + 1)
    value.setHours(9, 0, 0, 0)
    return value
  }, [])
  const [startAt, setStartAt] = useState(dateTimeInputValue(tomorrow))
  const [endAt, setEndAt] = useState(dateTimeInputValue(new Date(tomorrow.getTime() + 9 * 60 * 60_000)))
  const [reason, setReason] = useState("")
  const [requestKey] = useState(() => crypto.randomUUID())

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!startAt || !endAt || endAt <= startAt) {
      toast.error(os(t, "leaveDialog.invalidWindowToast"))
      return
    }
    if (reason.trim().length < 2) {
      toast.error(os(t, "leaveDialog.missingReasonToast"))
      return
    }
    await onSubmit({ requestKey, startAt, endAt, reason: reason.trim() })
  }

  return (
    <StandardModal
      open={open}
      onCancel={() => onOpenChange(false)}
      title={os(t, "actions.applyLeave")}
      width={448}
      footer={
        <>
            <RailopsButton disabled={saving} onClick={() => onOpenChange(false)}>{os(t, "actions.cancel")}</RailopsButton>
            <RailopsButton htmlType="submit" form="leave-request-form" disabled={saving}>
              {saving ? <RefreshCwIcon className="animate-spin" /> : <SendIcon />}
              {os(t, "leaveDialog.submit")}
            </RailopsButton>

        </>
      }
    >
        <div className="grid gap-4 py-1">
          <div className="grid gap-2 sm:grid-cols-2">
            <div className="grid gap-2">
              <label htmlFor="leave-start-at" className="text-sm font-medium">{os(t, "leaveDialog.startTime")}</label>
              <Input id="leave-start-at" type="datetime-local" value={startAt} disabled={saving} onChange={(event) => setStartAt(event.target.value)} />
            </div>
            <div className="grid gap-2">
              <label htmlFor="leave-end-at" className="text-sm font-medium">{os(t, "leaveDialog.endTime")}</label>
              <Input id="leave-end-at" type="datetime-local" value={endAt} min={startAt} disabled={saving} onChange={(event) => setEndAt(event.target.value)} />
            </div>
          </div>
          <div className="grid gap-2">
            <label htmlFor="leave-reason" className="text-sm font-medium">{os(t, "leaveDialog.leaveReason")}</label>
            <Input.TextArea id="leave-reason" value={reason} maxLength={255} disabled={saving} placeholder={os(t, "leaveDialog.leaveReasonPlaceholder")} onChange={(event) => setReason(event.target.value)} />
          </div>
          <div className="text-xs text-muted-foreground">{os(t, "leaveDialog.timezoneHint")}</div>
        </div>

    </StandardModal>
  )
}

export default function EnterpriseOrgSchedulesPage() {
  const { session } = useAuth()
  const t = useI18n()
  const router = useRouter()
  const searchParams = useSearchParams()
  const requestedTeamId = Number(searchParams.get("teamId")) || 0
  const isEmployeePortalMode = session?.supportMode === "employee_portal"
  const myScheduleMode = isEmployeePortalMode || searchParams.get("view") === "mine"
  const [teams, setTeams] = useState<AdminAgentTeam[]>([])
  const [selectedTeamId, setSelectedTeamId] = useState(requestedTeamId > 0 ? String(requestedTeamId) : "")
  const [members, setMembers] = useState<AdminAgentProfile[]>([])
  const [memberAvailability, setMemberAvailability] = useState<AdminAgentTeamMemberAvailability[]>([])
  const [schedules, setSchedules] = useState<AdminAgentTeamSchedule[]>([])
  const [draftSchedules, setDraftSchedules] = useState<AdminAgentTeamSchedule[]>([])
  const [leaveExceptions, setLeaveExceptions] = useState<AdminAgentScheduleException[]>([])
  const [hasDraft, setHasDraft] = useState(false)
  const [loadingTeams, setLoadingTeams] = useState(true)
  const [loadingSchedule, setLoadingSchedule] = useState(false)
  const [personalBaseSchedule, setPersonalBaseSchedule] = useState<AdminAgentTeamScheduleTemplate | null>(null)
  const [leaveOpen, setLeaveOpen] = useState(false)
  const [leaveSaving, setLeaveSaving] = useState(false)
  const [leaveActionId, setLeaveActionId] = useState<number | null>(null)
  const [leaveCancelTarget, setLeaveCancelTarget] = useState<AdminAgentScheduleException | null>(null)
  const permissions = session?.permissions || []
  const canUpdate = !myScheduleMode && permissions.includes("agentTeamSchedule.update")

  const productTeams = useMemo(
    () =>
      myScheduleMode
        ? teams
        : teams.filter((item) => item.teamType === PRODUCT_TEAM_TYPE && item.productId > 0),
    [myScheduleMode, teams],
  )
  const teamsWithEmptyPublishedCoverage = useMemo(() => {
    const publishedTeamIDs = new Set(
      schedules.filter((item) => item.repeatType === "weekly").map((item) => item.teamId),
    )
    return productTeams.filter((team) => team.scheduleEnforced && !publishedTeamIDs.has(team.id))
  }, [productTeams, schedules])
  const activePublishedCoverageCount = useMemo(() => {
    const enforcedTeamIDs = new Set(productTeams.filter((team) => team.scheduleEnforced).map((team) => team.id))
    return schedules.filter((item) => item.repeatType === "weekly" && enforcedTeamIDs.has(item.teamId)).length
  }, [productTeams, schedules])
  const availabilitySummary = useMemo(() => {
    const total = memberAvailability.length
    const available = memberAvailability.filter((item) => item.availableNow).length
    const onLeave = memberAvailability.filter((item) => item.activeLeave).length
    const unavailable = Math.max(total - available, 0)
    return { total, available, unavailable, onLeave }
  }, [memberAvailability])
  const sortedMemberAvailability = useMemo(() => {
    return memberAvailability.slice().sort((left, right) => {
      if (left.availableNow !== right.availableNow) return left.availableNow ? -1 : 1
      if (Boolean(left.activeLeave) !== Boolean(right.activeLeave)) return left.activeLeave ? -1 : 1
      return memberName(left).localeCompare(memberName(right), "zh-CN")
    })
  }, [memberAvailability])
  const unavailableGroups = useMemo(() => {
    const grouped = new Map<string, number>()
    for (const member of memberAvailability) {
      if (member.availableNow) continue
      const label = availabilityReasonLabel(member, t)
      grouped.set(label, (grouped.get(label) || 0) + 1)
    }
    return Array.from(grouped.entries())
      .map(([label, count]) => ({ label, count }))
      .sort((left, right) => right.count - left.count || left.label.localeCompare(right.label, "zh-CN"))
  }, [memberAvailability, t])
  const pendingLeaves = useMemo(() => {
    return leaveExceptions
      .filter((item) => item.approvalStatus === "pending")
      .sort((left, right) => new Date(left.startAt).getTime() - new Date(right.startAt).getTime())
  }, [leaveExceptions])
  const historicalCoverage = useMemo(() => {
    return schedules
      .filter((item) => item.repeatType === "weekly")
      .slice()
      .sort((left, right) => left.weekday - right.weekday || (left.startMinute || 0) - (right.startMinute || 0) || left.id - right.id)
  }, [schedules])
  const selectedTeam = useMemo(
    () =>
      myScheduleMode
        ? null
        : productTeams.find((item) => String(item.id) === selectedTeamId) ?? null,
    [myScheduleMode, productTeams, selectedTeamId],
  )
  const loadTeams = useCallback(async () => {
    if (myScheduleMode) {
      setLoadingTeams(false)
      setSelectedTeamId("")
      return
    }
    setLoadingTeams(true)
    try {
      const data = await fetchAgentTeamsAll()
      const productRows = data.filter((item) => item.teamType === PRODUCT_TEAM_TYPE && item.productId > 0)
      setTeams(data)
      setSelectedTeamId((current) => {
        if (requestedTeamId > 0 && productRows.some((item) => item.id === requestedTeamId)) {
          return String(requestedTeamId)
        }
        if (current && productRows.some((item) => String(item.id) === current)) {
          return current
        }
        return productRows[0] ? String(productRows[0].id) : ""
      })
    } catch (error) {
      toast.error(error instanceof Error ? error.message : os(t, "toasts.loadTeamsFailed"))
    } finally {
      setLoadingTeams(false)
    }
  }, [myScheduleMode, requestedTeamId, t])

  const loadSchedule = useCallback(async () => {
    if (myScheduleMode) {
      setLoadingSchedule(true)
      try {
        const data = await fetchEngineerWorkSchedule()
        setTeams(data.teams || [])
        setMembers([])
        setMemberAvailability([])
        setSchedules(data.schedules || [])
        setDraftSchedules(data.draftSchedules || [])
        setPersonalBaseSchedule(normalizePersonalBaseSchedule(data.baseSchedule))
        setLeaveExceptions(data.exceptions || [])
        setHasDraft(false)
      } catch (error) {
        toast.error(error instanceof Error ? error.message : os(t, "toasts.loadMyScheduleFailed"))
      } finally {
        setLoadingSchedule(false)
      }
      return
    }
    if (!selectedTeam) {
      setMembers([])
      setMemberAvailability([])
      setSchedules([])
      setDraftSchedules([])
      setLeaveExceptions([])
      setHasDraft(false)
      return
    }
    setLoadingSchedule(true)
    try {
      const [memberRows, availabilityRows, publishedPage, draftPage, leaveRows] = await Promise.all([
        fetchAgentProfilesAll({ teamId: selectedTeam.id }),
        fetchAgentTeamMemberAvailability(selectedTeam.id),
        fetchAgentTeamSchedules({
          teamId: selectedTeam.id,
          repeatType: "weekly",
          page: 1,
          limit: 1000,
          publishStatus: "published",
        }),
        fetchAgentTeamSchedules({
          teamId: selectedTeam.id,
          publishStatus: "draft",
          page: 1,
          limit: 1000,
        }),
        fetchAgentTeamScheduleLeaves(selectedTeam.id),
      ])
      setMembers(memberRows.slice().sort((left, right) => memberName(left).localeCompare(memberName(right), "zh-CN")))
      setMemberAvailability(availabilityRows.slice().sort((left, right) => memberName(left).localeCompare(memberName(right), "zh-CN")))
      const published = publishedPage.results || []
      const draft = draftPage.results || []
      setHasDraft(draft.length > 0)
      setSchedules(draft.length > 0 ? draft.filter((item) => item.repeatType === "weekly") : published)
      setDraftSchedules(draft.filter((item) => item.repeatType === "weekly"))
      setLeaveExceptions(leaveRows || [])
    } catch (error) {
      toast.error(error instanceof Error ? error.message : os(t, "toasts.loadScheduleFailed"))
    } finally {
      setLoadingSchedule(false)
    }
  }, [myScheduleMode, selectedTeam, t])

  useEffect(() => {
    void loadTeams()
  }, [loadTeams])

  useEffect(() => {
    void loadSchedule()
  }, [loadSchedule])

  function handleTeamChange(value: string) {
    if (myScheduleMode) {
      return
    }
    setSelectedTeamId(value)
    router.replace(`/enterprise/org/schedules?teamId=${value}`)
  }

  async function handleCreateLeave(payload: { requestKey: string; startAt: string; endAt: string; reason: string }) {
    setLeaveSaving(true)
    try {
      await createEngineerLeave(payload)
      setLeaveOpen(false)
      toast.success(os(t, "toasts.leaveRequestSubmitted"))
      await loadSchedule()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : os(t, "toasts.leaveApplyFailed"))
    } finally {
      setLeaveSaving(false)
    }
  }

  async function handleCancelLeave() {
    if (!leaveCancelTarget) return
    setLeaveActionId(leaveCancelTarget.id)
    try {
      await cancelEngineerLeave(leaveCancelTarget.id)
      setLeaveCancelTarget(null)
      toast.success(os(t, "toasts.leaveCancelled"))
      await loadSchedule()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : os(t, "toasts.leaveCancelFailed"))
    } finally {
      setLeaveActionId(null)
    }
  }

  async function handleReviewLeave(id: number, decision: "approved" | "rejected") {
    setLeaveActionId(id)
    try {
      await reviewAgentTeamScheduleLeave({ id, decision })
      toast.success(decision === "approved" ? os(t, "toasts.leaveApproved") : os(t, "toasts.leaveRejected"))
      await loadSchedule()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : os(t, "toasts.leaveReviewFailed"))
    } finally {
      setLeaveActionId(null)
    }
  }

  const myScheduleActions = myScheduleMode ? (
    <RailopsButton onClick={() => void loadSchedule()} disabled={loadingSchedule}>
      <RefreshCwIcon className={loadingSchedule ? "animate-spin" : undefined} />
      {os(t, "nav.refresh")}
    </RailopsButton>
  ) : null

  return (
    <PageShell
      title={myScheduleMode ? os(t, "pageTitle.mySchedule") : os(t, "pageTitle.productGroup")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="rhd-railops-org-schedules-page"
      actions={
        <>
          {myScheduleMode ? (
            <Link href="/enterprise/workbench">
              <RailopsButton size="small">
                <ArrowLeftIcon />
                {os(t, "nav.backToEmployeePortal")}
              </RailopsButton>
            </Link>
          ) : (
            <Link href={selectedTeam ? `/enterprise/org?teamId=${selectedTeam.id}` : "/enterprise/org"}>
              <RailopsButton size="small">
                <ArrowLeftIcon />
                {os(t, "nav.backToOrg")}
              </RailopsButton>
            </Link>
          )}
          {myScheduleMode ? (
            <>
              {myScheduleActions}
              <RailopsButton onClick={() => setLeaveOpen(true)}>
                <CalendarOffIcon />
                {os(t, "actions.applyLeave")}
              </RailopsButton>
            </>
          ) : (
            <>
              <div className="w-full sm:w-72">
                <OptionCombobox
                  value={selectedTeamId}
                  options={productTeams.map((team) => ({ value: String(team.id), label: team.productName || team.name }))}
                  placeholder={loadingTeams ? os(t, "teamPicker.loading") : os(t, "teamPicker.select")}
                  searchPlaceholder={os(t, "teamPicker.search")}
                  emptyText={os(t, "teamPicker.empty")}
                  disabled={loadingTeams}
                  onChange={handleTeamChange}
                />
              </div>
              {selectedTeam ? (
                <Link href={`/enterprise/org/members?teamId=${selectedTeam.id}`}>
                  <RailopsButton size="small" disabled={!selectedTeam}>
                    <UsersRoundIcon />
                    {os(t, "nav.memberManage")}
                  </RailopsButton>
                </Link>
              ) : (
                <RailopsButton size="small" disabled={!selectedTeam}>
                  <UsersRoundIcon />
                  {os(t, "nav.memberManage")}
                </RailopsButton>
              )}
              <RailopsButton onClick={() => void loadSchedule()} disabled={!selectedTeam || loadingSchedule}>
                <RefreshCwIcon className={loadingSchedule ? "animate-spin" : undefined} />
                {os(t, "nav.refresh")}
              </RailopsButton>
            </>
          )}
        </>
      }
    >

      {myScheduleMode ? (
        <section className="rhd-railops-org-schedule-hero">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-medium">{os(t, "scope.enterpriseBaseWorkTime")}</span>
              <StatusTag tone="neutral">{personalBaseSchedule?.timezone || ENGINEER_TIMEZONE}</StatusTag>
            </div>
            <div className="mt-1 text-xs text-muted-foreground">
              {workdayRangeLabel(personalBaseSchedule?.workdays ?? [], t)} {personalBaseSchedule?.startTime ?? "--:--"} - {personalBaseSchedule?.endTime ?? "--:--"}
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-4 text-sm text-muted-foreground">
            <span>{os(t, "hero.supportedProducts", { count: productTeams.length })}</span>
            <span>{os(t, "hero.pendingReview", { count: leaveExceptions.filter((item) => item.approvalStatus === "pending").length })}</span>
          </div>
        </section>
      ) : selectedTeam ? (
        <section className="rhd-railops-org-schedule-hero">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-medium">{selectedTeam.productName || selectedTeam.name}</span>
              {hasDraft ? (
                <StatusTag tone="neutral">{os(t, "scope.draftVersion", { version: schedules[0]?.version || selectedTeam.scheduleVersion + 1 })}</StatusTag>
              ) : selectedTeam.scheduleEnforced ? (
                <StatusTag tone="blue" className="border-primary/20 bg-primary/10 text-primary">
                  <CheckCircle2Icon />
                  {os(t, "scope.specialOverrideVersion", { version: selectedTeam.scheduleVersion })}
                </StatusTag>
              ) : (
                <StatusTag tone="neutral">{os(t, "scope.usingEnterpriseBase")}</StatusTag>
              )}
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-4 text-sm text-muted-foreground">
            <span>{os(t, "hero.teamMemberCount", { count: memberAvailability.length || members.length })}</span>
            <span>{os(t, "hero.currentLeave", { count: memberAvailability.filter((item) => item.activeLeave).length || members.filter((item) => activeLeaveForUser(leaveExceptions, item.userId)).length })}</span>
            <span>{os(t, "hero.pendingApproval", { count: leaveExceptions.filter((item) => item.approvalStatus === "pending").length })}</span>
          </div>
        </section>
      ) : null}

      {myScheduleMode ? (
        loadingSchedule && productTeams.length === 0 ? (
          <div className="space-y-4">
            <Skeleton.Node active className="w-full" style={{ width: "100%", height: 224 }} />
            <Skeleton.Node active className="w-full" style={{ width: "100%", height: 224 }} />
          </div>
        ) : (
          <div className="space-y-4">
            <section className="rhd-railops-org-schedule-panel space-y-3" data-testid="my-product-schedules">
              <div className="flex flex-wrap items-end justify-between gap-3">
                <div>
                  <h2 className="font-semibold">{os(t, "mySchedule.unifiedWeek")}</h2>
                </div>
                <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
                  <span>{os(t, "mySchedule.publishedOverrideCount", { count: activePublishedCoverageCount })}</span>
                  <span>{os(t, "mySchedule.pendingDraftCount", { count: draftSchedules.filter((item) => item.repeatType === "weekly").length })}</span>
                </div>
              </div>
              {productTeams.length === 0 ? (
                <span className="text-sm text-muted-foreground">{os(t, "mySchedule.notJoinedProductGroup")}</span>
              ) : personalBaseSchedule ? (
                <UnifiedPersonalScheduleWeek
                  baseSchedule={personalBaseSchedule}
                  teams={productTeams}
                  schedules={schedules}
                  draftSchedules={draftSchedules}
                  t={t}
                />
              ) : (
                <Skeleton.Node active className="w-full" style={{ width: "100%", height: 224 }} />
              )}
              {teamsWithEmptyPublishedCoverage.length > 0 ? (
                <div className="border-l-2 border-amber-500 bg-amber-50/60 px-3 py-2 text-sm dark:bg-amber-950/20" data-testid="empty-published-coverage-warning">
                  <div className="font-medium">{os(t, "mySchedule.emptyCoverageWarning")}</div>
                  <div className="mt-1 text-xs text-muted-foreground">
                    {os(t, "mySchedule.emptyCoverageWarningDetail", {
                      teams: teamsWithEmptyPublishedCoverage.map((team) => `${team.productName || team.name} v${team.scheduleVersion}`).join(os(t, "listSeparator")),
                    })}
                  </div>
                </div>
              ) : null}
            </section>

            <section className="rhd-railops-org-schedule-panel space-y-3" data-testid="my-supported-products">
              <div>
                <h2 className="font-semibold">{os(t, "mySchedule.supportedProducts")}</h2>
              </div>
              <div className="flex flex-wrap gap-2">
                {productTeams.map((team) => (
                  <StatusTag key={team.id} tone="neutral" className="px-3 py-1.5">{team.productName || team.name}</StatusTag>
                ))}
                {productTeams.length === 0 ? <span className="text-sm text-muted-foreground">{os(t, "mySchedule.notJoinedProductGroup")}</span> : null}
              </div>
            </section>

            <section className="rhd-railops-org-schedule-panel space-y-3" data-testid="my-leave-requests">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h2 className="font-semibold">{os(t, "mySchedule.leaveRecord")}</h2>
                </div>
                <RailopsButton size="small" onClick={() => setLeaveOpen(true)}><PlusIcon />{os(t, "actions.applyLeave")}</RailopsButton>
              </div>
              <div className="divide-y border-y">
                {leaveExceptions.map((item) => (
                  <div key={item.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2 text-sm font-medium">
                        <span>{os(t, "records.leaveWindowRange", { start: formatScheduleDateTime(item.startAt), end: formatScheduleDateTime(item.endAt) })}</span>
                        <StatusTag tone={item.approvalStatus === "approved" ? "blue" : "neutral"}>{leaveStatusLabel(item.approvalStatus, t)}</StatusTag>
                      </div>
                      <div className="mt-1 text-xs text-muted-foreground">{item.reason || os(t, "records.reasonMissing")}{item.reviewNote ? ` · ${os(t, "records.reviewNote", { note: item.reviewNote })}` : ""}</div>
                    </div>
                    {item.approvalStatus === "pending" || item.approvalStatus === "approved" ? (
                      <RailopsButton variant="text" size="small" disabled={leaveActionId === item.id} onClick={() => setLeaveCancelTarget(item)}>
                        {leaveActionId === item.id ? <RefreshCwIcon className="animate-spin" /> : <XCircleIcon />}
                        {os(t, "actions.cancel")}
                      </RailopsButton>
                    ) : null}
                  </div>
                ))}
                {leaveExceptions.length === 0 ? <div className="py-8 text-center text-sm text-muted-foreground">{os(t, "mySchedule.noLeaveRecords")}</div> : null}
              </div>
            </section>
          </div>
        )
      ) : loadingTeams ? (
        <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
          <Skeleton.Node active className="w-full" style={{ width: "100%", height: 384 }} />
          <Skeleton.Node active className="w-full" style={{ width: "100%", height: 384 }} />
        </div>
      ) : selectedTeam ? (
        <div className="space-y-4">
          <section className="rhd-railops-org-schedule-metrics" data-testid="availability-summary">
            {[
              { label: os(t, "metrics.available"), value: availabilitySummary.available, detail: os(t, "metrics.totalPeople", { count: availabilitySummary.total }), icon: UserCheckIcon, tone: "primary" },
              { label: os(t, "metrics.unavailable"), value: availabilitySummary.unavailable, detail: unavailableGroups.length > 0 ? os(t, "metrics.unavailableSummary", { label: unavailableGroups[0].label, count: unavailableGroups[0].count }) : os(t, "metrics.none"), icon: ActivityIcon, tone: "slate" },
              { label: os(t, "metrics.onLeave"), value: availabilitySummary.onLeave, detail: os(t, "metrics.pendingReview", { count: pendingLeaves.length }), icon: CalendarOffIcon, tone: "amber" },
              { label: os(t, "metrics.personalRule"), value: os(t, "workdayRange.workdays"), detail: os(t, "metrics.hours24"), icon: ShieldCheckIcon, tone: "blue" },
            ].map((item) => {
              const Icon = item.icon
              return (
                <div key={item.label} className="rhd-railops-org-schedule-metric">
                  <div className="flex items-center justify-between gap-3">
                    <span className="text-sm text-muted-foreground">{item.label}</span>
                      <span
                      className={cn(
                        "flex size-8 items-center justify-center rounded-md",
                        item.tone === "primary" && "bg-primary/10 text-primary",
                        item.tone === "amber" && "bg-amber-50 text-amber-700 dark:bg-amber-950 dark:text-amber-300",
                        item.tone === "blue" && "bg-primary/10 text-primary dark:bg-primary/20 dark:text-primary",
                        item.tone === "slate" && "bg-muted text-muted-foreground",
                      )}
                    >
                      <Icon className="size-4" />
                    </span>
                  </div>
                  <div className="mt-3 text-2xl font-semibold tabular-nums">{item.value}</div>
                  <div className="mt-1 text-xs text-muted-foreground">{item.detail}</div>
                </div>
              )
            })}
          </section>

          <section className="rhd-railops-org-schedule-workspace">
            <div className="space-y-4">
              <section className="rhd-railops-org-schedule-panel" data-testid="product-effective-roster">
                <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
                  <div>
                    <h2 className="font-semibold">{os(t, "memberStatus.title")}</h2>
                  </div>
                  <StatusTag tone="neutral">
                    {os(t, "memberStatus.availableCount", { available: availabilitySummary.available, total: availabilitySummary.total })}
                  </StatusTag>
                </div>
                <div className="divide-y">
                  {loadingSchedule ? (
                    Array.from({ length: 4 }).map((_, index) => (
                      <div key={index} className="px-4 py-3">
                        <Skeleton.Node active className="w-full" style={{ width: "100%", height: 56 }} />
                      </div>
                    ))
                  ) : (
                    sortedMemberAvailability.map((member) => {
                      const name = memberName(member)
                      const activeLeave = member.activeLeave
                      const pendingLeave = member.pendingLeave
                      const available = member.availableNow
                      return (
                        <div key={member.userId} className="grid gap-3 px-4 py-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
                          <div className="flex min-w-0 items-start gap-3">
                            <Avatar size="sm" className="mt-0.5">
                              {member.avatar ? <AvatarImage src={member.avatar} alt={name} /> : null}
                              <AvatarFallback>{memberInitial(name, t)}</AvatarFallback>
                            </Avatar>
                            <div className="min-w-0 flex-1">
                              <div className="flex flex-wrap items-center gap-2">
                                <span className="truncate text-sm font-medium">{name}</span>
                                {member.agentCode ? <StatusTag tone="neutral">{member.agentCode}</StatusTag> : null}
                                <StatusTag
                                  tone="neutral"
                                  className={available ? "bg-primary/10 text-primary dark:bg-primary/20 dark:text-primary" : undefined}
                                >
                                  {available ? os(t, "metrics.available") : availabilityReasonLabel(member, t)}
                                </StatusTag>
                              </div>
                              <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                                <span>{workdayRangeLabel(member.workdays, t)} {member.startTime} - {member.endTime}</span>
                                <span>{memberCapacityLabel(member, t)}</span>
                                {member.workStatusNote ? <span>{member.workStatusNote}</span> : null}
                              </div>
                              {activeLeave ? (
                                <div className="mt-2 text-xs text-amber-700 dark:text-amber-300">
                                  {os(t, "memberStatus.leaveWindowPrefix", { window: leaveWindowLabel(activeLeave) })}
                                </div>
                              ) : null}
                            </div>
                          </div>
                          <div className="flex flex-wrap items-center gap-2 lg:justify-end">
                            {!member.dispatchEnabled ? <StatusTag tone="neutral">{os(t, "reason.teamDispatchDisabled")}</StatusTag> : null}
                            {!member.autoAssignEnabled ? <StatusTag tone="neutral">{os(t, "reason.autoAssignDisabled")}</StatusTag> : null}
                            {!member.workStatusConfirmed ? <StatusTag tone="neutral">{os(t, "reason.workStatusUnconfirmed")}</StatusTag> : null}
                            {pendingLeave ? <StatusTag tone="neutral">{os(t, "memberStatus.leavePending")}</StatusTag> : null}
                          </div>
                        </div>
                      )
                    })
                  )}
                  {!loadingSchedule && sortedMemberAvailability.length === 0 ? (
                    <div className="py-12 text-center text-sm text-muted-foreground">{os(t, "memberStatus.noEngineers")}</div>
                  ) : null}
                </div>
              </section>

              <section className="rhd-railops-org-schedule-panel" data-testid="historical-special-coverage">
                <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
                  <div>
                    <h2 className="font-semibold">{os(t, "coverage.title")}</h2>
                  </div>
                  <div className="flex items-center gap-2">
                    <StatusTag tone="neutral">{os(t, "coverage.readOnly")}</StatusTag>
                    <StatusTag tone="neutral">{os(t, "coverage.recordCount", { count: historicalCoverage.length })}</StatusTag>
                  </div>
                </div>
                <div className="divide-y">
                  {loadingSchedule ? (
                    <div className="p-4"><Skeleton.Node active className="w-full" style={{ width: "100%", height: 96 }} /></div>
                  ) : historicalCoverage.length > 0 ? (
                    historicalCoverage.map((item) => {
                      const name = scheduleScopeLabel(item, members, t)
                      return (
                        <div key={item.id} className="grid gap-2 px-4 py-3 sm:grid-cols-[88px_minmax(0,1fr)_auto] sm:items-center">
                          <div className="text-sm font-medium">{weekdayLabel(item.weekday, t)}</div>
                          <div className="min-w-0">
                            <div className="truncate text-sm">{name}</div>
                            <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                              <span className="tabular-nums">{scheduleTime(item, t)}</span>
                              <span>v{item.version}</span>
                              {item.effectiveUntil ? <span>{os(t, "coverage.effectiveUntil", { until: item.effectiveUntil })}</span> : null}
                            </div>
                          </div>
                          <StatusTag tone="neutral">
                            {item.dayType === "rest" ? os(t, "scheduleTime.rest") : os(t, "coverage.override")}
                          </StatusTag>
                        </div>
                      )
                    })
                  ) : (
                    <div className="flex min-h-24 items-center justify-center text-sm text-muted-foreground">
                      {os(t, "coverage.noCoverage")}
                    </div>
                  )}
                </div>
              </section>
            </div>

            <aside className="space-y-4">
              <section className="rhd-railops-org-schedule-panel" data-testid="leave-review-panel">
                <div className="flex items-center justify-between gap-3 border-b px-4 py-3">
                  <div>
                    <h2 className="font-semibold">{os(t, "metrics.onLeave")}</h2>
                  </div>
                  <StatusTag tone="neutral">{os(t, "leavePanel.pendingCount", { count: pendingLeaves.length })}</StatusTag>
                </div>
                <div className="divide-y">
                  {pendingLeaves.slice(0, 4).map((item) => (
                    <div key={item.id} className="space-y-2 px-4 py-3">
                      <div className="flex items-start justify-between gap-2">
                        <div className="min-w-0">
                          <div className="truncate text-sm font-medium">{memberName({ userId: item.userId, nickname: item.userName, username: item.userName }, item.userName)}</div>
                          <div className="mt-1 text-xs text-muted-foreground">{leaveWindowLabel(item)}</div>
                          {item.reason ? <div className="mt-1 text-xs text-muted-foreground">{item.reason}</div> : null}
                        </div>
                        <StatusTag tone="neutral">{os(t, "leaveStatus.pending")}</StatusTag>
                      </div>
                      {canUpdate ? (
                        <div className="flex gap-2">
                          <RailopsButton size="small" className="flex-1" disabled={leaveActionId === item.id} onClick={() => void handleReviewLeave(item.id, "approved")}>
                            {leaveActionId === item.id ? <RefreshCwIcon className="animate-spin" /> : <CheckCircle2Icon />}
                            {os(t, "leavePanel.approve")}
                          </RailopsButton>
                          <RailopsButton size="small" className="flex-1" disabled={leaveActionId === item.id} onClick={() => void handleReviewLeave(item.id, "rejected")}>
                            <XCircleIcon />{os(t, "leavePanel.reject")}
                          </RailopsButton>
                        </div>
                      ) : null}
                    </div>
                  ))}
                  {pendingLeaves.length === 0 ? (
                    <div className="px-4 py-6 text-sm text-muted-foreground">{os(t, "leavePanel.noPendingReview")}</div>
                  ) : null}
                </div>
              </section>

              <section className="rhd-railops-org-schedule-panel">
                <div className="flex items-center gap-2 border-b px-4 py-3">
                  <ListChecksIcon className="size-4 text-muted-foreground" />
                  <h2 className="font-semibold">{os(t, "reasonsPanel.title")}</h2>
                </div>
                <div className="space-y-2 p-4">
                  {unavailableGroups.map((item) => (
                    <div key={item.label} className="flex items-center justify-between gap-3 text-sm">
                      <span className="text-muted-foreground">{item.label}</span>
                      <StatusTag tone="neutral">{item.count}</StatusTag>
                    </div>
                  ))}
                  {unavailableGroups.length === 0 ? <div className="text-sm text-muted-foreground">{os(t, "metrics.none")}</div> : null}
                </div>
              </section>
            </aside>
          </section>
        </div>
      ) : (
        <div className="flex min-h-72 flex-col items-center justify-center gap-3 rounded-lg border border-dashed text-center">
          <CalendarClockIcon className="size-8 text-muted-foreground" />
          <div>
            <div className="font-medium">{os(t, "empty.noProductTeams")}</div>
          </div>
        </div>
      )}

      {leaveOpen ? (
        <LeaveRequestDialog
          open
          saving={leaveSaving}
          onOpenChange={(open) => { if (!leaveSaving) setLeaveOpen(open) }}
          onSubmit={handleCreateLeave}
          t={t}
        />
      ) : null}

      <StandardModal
        open={leaveCancelTarget !== null}
        onCancel={() => { if (leaveActionId === null) setLeaveCancelTarget(null) }}
        title={os(t, "actions.cancelLeaveTitle")}
        width={448}
        footer={
          <>
            <RailopsButton disabled={leaveActionId !== null} onClick={() => setLeaveCancelTarget(null)}>{os(t, "actions.back")}</RailopsButton>
            <RailopsButton danger disabled={leaveActionId !== null} onClick={() => void handleCancelLeave()}>
              {leaveActionId !== null ? <RefreshCwIcon className="animate-spin" /> : <XCircleIcon />}
              {os(t, "actions.confirmCancel")}
            </RailopsButton>

          </>
        }
      />
    </PageShell>
  )
}

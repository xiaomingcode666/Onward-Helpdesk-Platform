"use client"

import Link from "next/link"
import { useSearchParams } from "next/navigation"
import {
  Building2Icon,
  ChevronRightIcon,
  FolderTreeIcon,
  LockKeyholeIcon,
  PackageIcon,
  PowerIcon,
  RefreshCwIcon,
  ShieldCheckIcon,
  UserRoundCheckIcon,
  UserMinusIcon,
  UsersRoundIcon,
} from "lucide-react"
import { type ReactNode, useCallback, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"
import { Skeleton } from "antd"
import { DataTable, IconButton, PageShell, RailopsButton, SelectField, StatusTag } from "@railops/ui"

import { useConfirm } from "@/components/confirm-provider"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { ScrollArea } from "@/components/ui/scroll-area"
import { useI18n } from "@/i18n/provider"
import {
  fetchAgentTeamMemberAvailability,
  fetchAgentProfilesAll,
  fetchAgentTeamsAll,
  removeAgentTeamMember,
  updateAgentTeam,
  upsertAgentTeamMember,
  type AdminAgentProfile,
  type AdminAgentTeam,
  type AdminAgentTeamMemberAvailability,
} from "@/lib/api/admin"
import { assignTicket, fetchTickets } from "@/lib/api/enterprise-tickets"
import {
  fetchAllEnterpriseIAMDepartments,
  type EnterpriseIAMDepartment,
} from "@/lib/api/platform-iam"
import type { TicketListItem } from "@/lib/api/types"
import { cn } from "@/lib/utils"
import { agentMemberName } from "./member-name"

const orgProductSupportI18nPrefix = "orgExtract.orgProductSupport."
type OrgProductSupportT = ReturnType<typeof useI18n>
const ops = (t: OrgProductSupportT, key: string, values?: Record<string, string | number>) => t(`${orgProductSupportI18nPrefix}${key}`, values)

const PRODUCT_TEAM_TYPE = "product_repair"
const TECHNICAL_TEAM_TYPE = "technical_repair"
// 该名称与后端部门路径耦合（后端按中文路径存储），路径匹配需保持原样，故使用转义形式
const TECHNICAL_AFTER_SALES_TEAM_NAME = "\u6280\u672f\u552e\u540e\u7ec4"
const TECHNICAL_AFTER_SALES_PATH = `/${TECHNICAL_AFTER_SALES_TEAM_NAME}`
const TECHNICAL_MAINTENANCE_TEAM_NAME = "\u6280\u672f\u7ef4\u62a4\u7ec4"
const TECHNICAL_MAINTENANCE_PATH = `/${TECHNICAL_MAINTENANCE_TEAM_NAME}`
const ASSIGNMENT_MODES = [
  { value: "balanced", labelKey: "assignment.average" },
  { value: "weighted", labelKey: "assignment.weighted" },
] as const
const DISPATCH_WEIGHTS = [1, 2, 3, 4, 5, 8, 10]

type DepartmentTreeNode = {
  department: EnterpriseIAMDepartment
  children: DepartmentTreeNode[]
}

function assignmentModeLabel(value: string | undefined, t: OrgProductSupportT) {
  const mode = ASSIGNMENT_MODES.find((item) => item.value === value)
  return mode ? ops(t, mode.labelKey) : ops(t, "assignment.average")
}

function isRootDepartment(item: EnterpriseIAMDepartment) {
  return item.department_code === "root" || item.depth === 0
}

function isSupportSystemDepartment(item: EnterpriseIAMDepartment, protectedDepartmentIds: Set<number>) {
  return (
    protectedDepartmentIds.has(item.id) ||
    item.department_code === "technical-repair" ||
    item.path === TECHNICAL_AFTER_SALES_PATH ||
    item.path.startsWith(`${TECHNICAL_AFTER_SALES_PATH}/`) ||
    item.path === TECHNICAL_MAINTENANCE_PATH ||
    item.path.startsWith(`${TECHNICAL_MAINTENANCE_PATH}/`)
  )
}

function buildDepartmentTree(items: EnterpriseIAMDepartment[], parentId: number): DepartmentTreeNode[] {
  const children = items
    .filter((item) => item.parent_id === parentId)
    .sort((left, right) => left.name.localeCompare(right.name, "zh-CN"))
  return children.map((department) => ({
    department,
    children: buildDepartmentTree(items, department.id),
  }))
}

function isClosedTicket(status: TicketListItem["status"]) {
  return status === "closed" || status === "done" || status === "cancelled"
}

function statusLabel(status: TicketListItem["status"], t: OrgProductSupportT) {
  switch (status) {
    case "pending_acceptance":
      return ops(t, "status.pendingAcceptance")
    case "pending_dispatch":
      return ops(t, "status.pendingDispatch")
    case "accepted":
      return ops(t, "status.accepted")
    case "pending_assignee_accept":
      return ops(t, "status.pendingAssigneeAccept")
    case "in_progress":
    case "processing":
      return ops(t, "status.processing")
    case "video_support":
      return ops(t, "status.videoSupport")
    case "supplier_support":
      return ops(t, "status.supplierSupport")
    case "resolved":
    case "pending_customer_confirm":
      return ops(t, "status.pendingCustomerConfirm")
    case "closed":
    case "done":
      return ops(t, "status.closed")
    case "reopened":
      return ops(t, "status.reopened")
    case "cancelled":
      return ops(t, "status.cancelled")
    default:
      return status
  }
}

function memberDispatchActionLabel(item: AdminAgentProfile, t: OrgProductSupportT) {
  if (!item.autoAssignEnabled) {
    return ops(t, "memberState.globalAutoDispatchDisabled")
  }
  return ops(t, item.teamDispatchEnabled ? "memberState.disableTeamAutoDispatch" : "memberState.enableTeamAutoDispatch")
}

function availabilityLabel(item: AdminAgentTeamMemberAvailability, t: OrgProductSupportT) {
  const name = agentMemberName(item)
  const capacity = item.maxConcurrentCount > 0 ? `${item.dispatchWeight || 1}/${item.maxConcurrentCount}` : `${item.dispatchWeight || 1}/${ops(t, "reason.unlimited")}`
  const state = item.availableNow ? ops(t, "availability.available") : ops(t, "availability.unavailable")
  return ops(t, "availability.line", {
    name,
    state,
    weight: item.dispatchWeight || 1,
    capacity,
  })
}

function isDispatchableMember(item: AdminAgentTeamMemberAvailability) {
  return item.availableNow && item.dispatchEnabled
}

function isManuallyAssignableMember(item: AdminAgentTeamMemberAvailability) {
  return item.userId > 0 && item.profileId > 0
}

function dispatchMemberOptionLabel(item: AdminAgentTeamMemberAvailability, t: OrgProductSupportT) {
  const label = availabilityLabel(item, t)
  if (isDispatchableMember(item)) {
    return label
  }
  return `${label} · ${availabilityReasonLabel(item, t)}`
}

function availabilityReasonLabel(item: AdminAgentTeamMemberAvailability, t: OrgProductSupportT) {
  if (item.activeLeave) return ops(t, "reason.leave")
  switch (item.unavailableReason) {
    case "team_dispatch_disabled":
      return ops(t, "reason.teamDispatchDisabled")
    case "auto_assign_disabled":
      return ops(t, "reason.autoAssignDisabled")
    case "service_busy":
      return ops(t, "reason.serviceBusy")
    case "capacity_not_configured":
      return ops(t, "reason.capacityNotConfigured")
    case "outside_personal_dispatch_rule":
      return ops(t, "reason.outsideRule")
    case "approved_leave":
      return ops(t, "reason.leave")
    case "work_status_unconfirmed":
      return ops(t, "reason.workStatusUnconfirmed")
    case "work_status_unavailable":
      return ops(t, "reason.workStatusUnavailable")
    case "not_reachable":
      return ops(t, "reason.notReachable")
    default:
      return ops(t, "reason.unavailable")
  }
}

function workdayLabel(workdays: number[], t: OrgProductSupportT) {
  const normalized = [...new Set(workdays)].sort((left, right) => left - right)
  if (normalized.join(",") === "1,2,3,4,5") return ops(t, "workdayRange.workdays")
  if (normalized.join(",") === "1,2,3,4,5,6,7") return ops(t, "workdayRange.everyDay")
  const weekdayName = (weekday: number) => (weekday >= 1 && weekday <= 7 ? ops(t, `weekday.short${weekday}`) : String(weekday))
  return normalized.map((weekday) => ops(t, "weekday.full", { name: weekdayName(weekday) })).join(ops(t, "listSeparator")) || ops(t, "workdayRange.unset")
}

function parseTimeMinute(value?: string) {
  const [hourRaw, minuteRaw] = String(value || "").split(":")
  const hour = Number(hourRaw)
  const minute = Number(minuteRaw)
  if (!Number.isFinite(hour) || !Number.isFinite(minute)) {
    return 0
  }
  return Math.min(24 * 60, Math.max(0, hour * 60 + minute))
}

function dayWindowStyle(startTime?: string, endTime?: string) {
  const start = parseTimeMinute(startTime)
  const end = parseTimeMinute(endTime || "24:00")
  if (end <= start) {
    return { left: "0%", width: "100%" }
  }
  return {
    left: `${(start / 1440) * 100}%`,
    width: `${((end - start) / 1440) * 100}%`,
  }
}

function todayLeaveStyle(item: AdminAgentTeamMemberAvailability) {
  if (!item.activeLeave) {
    return null
  }
  const start = new Date(item.activeLeave.startAt).getTime()
  const end = new Date(item.activeLeave.endAt).getTime()
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) {
    return null
  }
  const dayStart = new Date()
  dayStart.setHours(0, 0, 0, 0)
  const dayEnd = new Date(dayStart)
  dayEnd.setDate(dayEnd.getDate() + 1)
  const visibleStart = Math.max(start, dayStart.getTime())
  const visibleEnd = Math.min(end, dayEnd.getTime())
  if (visibleEnd <= visibleStart) {
    return null
  }
  return {
    left: `${((visibleStart - dayStart.getTime()) / 86_400_000) * 100}%`,
    width: `${((visibleEnd - visibleStart) / 86_400_000) * 100}%`,
  }
}

function activeLeaveLabel(item: AdminAgentTeamMemberAvailability, t: OrgProductSupportT) {
  if (!item.activeLeave) {
    return ""
  }
  const start = new Date(item.activeLeave.startAt)
  const end = new Date(item.activeLeave.endAt)
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) {
    return ops(t, "reason.leave")
  }
  return `${start.getHours().toString().padStart(2, "0")}:${start.getMinutes().toString().padStart(2, "0")} - ${end.getHours().toString().padStart(2, "0")}:${end.getMinutes().toString().padStart(2, "0")}`
}

function buildAssigneeSelection(
  ticketRows: TicketListItem[],
  availabilityRows: AdminAgentTeamMemberAvailability[],
  current: Record<number, string>,
) {
  const candidateUserIds = new Set(
    availabilityRows
      .filter(isManuallyAssignableMember)
      .map((item) => item.userId),
  )
  const next: Record<number, string> = {}
  for (const ticket of ticketRows) {
    const currentAssignee = Number(current[ticket.id])
    if (candidateUserIds.has(currentAssignee)) {
      next[ticket.id] = current[ticket.id]
    } else {
      next[ticket.id] = ticket.assignee_id && candidateUserIds.has(ticket.assignee_id)
        ? String(ticket.assignee_id)
        : ""
    }
  }
  return next
}

export function ProductSupportOrganizationPage() {
  const confirm = useConfirm()
  const t = useI18n()
  const searchParams = useSearchParams()
  const requestedTeamId = Number(searchParams.get("teamId")) || 0
  const [teams, setTeams] = useState<AdminAgentTeam[]>([])
  const [departments, setDepartments] = useState<EnterpriseIAMDepartment[]>([])
  const [selectedTeamId, setSelectedTeamId] = useState<number | null>(null)
  const [members, setMembers] = useState<AdminAgentProfile[]>([])
  const [memberAvailability, setMemberAvailability] = useState<AdminAgentTeamMemberAvailability[]>([])
  const [tickets, setTickets] = useState<TicketListItem[]>([])
  const [assigneeByTicket, setAssigneeByTicket] = useState<Record<number, string>>({})
  const [teamsLoading, setTeamsLoading] = useState(true)
  const [departmentsLoading, setDepartmentsLoading] = useState(true)
  const [membersLoading, setMembersLoading] = useState(false)
  const [availabilityLoading, setAvailabilityLoading] = useState(false)
  const [ticketsLoading, setTicketsLoading] = useState(false)
  const [assigningTicketId, setAssigningTicketId] = useState<number | null>(null)
  const [savingRule, setSavingRule] = useState(false)
  const [savingLeader, setSavingLeader] = useState(false)
  const [savingMemberId, setSavingMemberId] = useState<number | null>(null)

  const technicalTeam = useMemo(
    () => teams.find((item) => item.teamType === TECHNICAL_TEAM_TYPE) ?? null,
    [teams],
  )
  const productTeams = useMemo(
    () => teams.filter((item) => item.teamType === PRODUCT_TEAM_TYPE && item.productId > 0),
    [teams],
  )
  const selectableTeams = useMemo(
    () => productTeams.length > 0 ? productTeams : technicalTeam ? [technicalTeam] : [],
    [productTeams, technicalTeam],
  )
  const protectedDepartmentIds = useMemo(() => {
    const ids = new Set<number>()
    if (technicalTeam?.departmentId) {
      ids.add(technicalTeam.departmentId)
    }
    for (const team of productTeams) {
      if (team.departmentId) {
        ids.add(team.departmentId)
      }
    }
    return ids
  }, [productTeams, technicalTeam])
  const rootDepartment = useMemo(
    () => departments.find((item) => isRootDepartment(item)) ?? null,
    [departments],
  )
  const ordinaryDepartments = useMemo(
    () => departments.filter((item) => !isRootDepartment(item) && !isSupportSystemDepartment(item, protectedDepartmentIds)),
    [departments, protectedDepartmentIds],
  )
  const ordinaryDepartmentTree = useMemo(
    () => buildDepartmentTree(ordinaryDepartments, rootDepartment?.id ?? 0),
    [ordinaryDepartments, rootDepartment?.id],
  )
  const detachedOrdinaryDepartmentTree = useMemo(
    () =>
      rootDepartment
        ? ordinaryDepartments
            .filter((item) => item.parent_id === 0)
            .map((department) => ({ department, children: buildDepartmentTree(ordinaryDepartments, department.id) }))
        : [],
    [ordinaryDepartments, rootDepartment],
  )
  const selectedTeam = useMemo(
    () => selectableTeams.find((item) => item.id === selectedTeamId) ?? null,
    [selectableTeams, selectedTeamId],
  )
  const isTechnicalTeam = selectedTeam?.teamType === TECHNICAL_TEAM_TYPE
  const activeTickets = useMemo(
    () => tickets.filter((item) => !isClosedTicket(item.status)),
    [tickets],
  )
  const unassignedCount = useMemo(
    () => activeTickets.filter((item) => !item.assignee_id).length,
    [activeTickets],
  )
  const unassignedTickets = useMemo(
    () => activeTickets.filter((item) => !item.assignee_id),
    [activeTickets],
  )
  const assignedActiveTickets = useMemo(
    () => activeTickets.filter((item) => item.assignee_id),
    [activeTickets],
  )
  const ticketRows = useMemo(
    () => [...unassignedTickets, ...assignedActiveTickets],
    [assignedActiveTickets, unassignedTickets],
  )
  const availableMembers = useMemo(
    () => memberAvailability.filter(isDispatchableMember),
    [memberAvailability],
  )
  const manuallyAssignableMembers = useMemo(
    () => memberAvailability.filter(isManuallyAssignableMember),
    [memberAvailability],
  )
  const activeTicketCount = activeTickets.length
  const organizationLoading = teamsLoading || departmentsLoading
  const teamTreeInitialLoading = teamsLoading && teams.length === 0
  const departmentTreeInitialLoading = departmentsLoading && departments.length === 0
  const detailRefreshing = membersLoading || availabilityLoading || ticketsLoading

  const loadTeams = useCallback(async () => {
    setTeamsLoading(true)
    try {
      const data = await fetchAgentTeamsAll()
      setTeams(data)
      const productRows = data.filter(
        (item) => item.teamType === PRODUCT_TEAM_TYPE && item.productId > 0,
      )
      const technicalRow = data.find((item) => item.teamType === TECHNICAL_TEAM_TYPE)
      const selectableRows = productRows.length > 0 ? productRows : technicalRow ? [technicalRow] : []
      setSelectedTeamId((current) =>
        selectableRows.some((item) => item.id === requestedTeamId)
          ? requestedTeamId
          : selectableRows.some((item) => item.id === current) ? current : (selectableRows[0]?.id ?? null),
      )
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ops(t, "toasts.loadOrgGroupsFailed"))
    } finally {
      setTeamsLoading(false)
    }
  }, [requestedTeamId, t])

  const loadDepartments = useCallback(async () => {
    setDepartmentsLoading(true)
    try {
      setDepartments(await fetchAllEnterpriseIAMDepartments())
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ops(t, "toasts.loadDepartmentTreeFailed"))
    } finally {
      setDepartmentsLoading(false)
    }
  }, [t])

  const loadMembers = useCallback(async () => {
    if (!selectedTeam) {
      setMembers([])
      return
    }
    setMembersLoading(true)
    try {
      setMembers(await fetchAgentProfilesAll({ teamId: selectedTeam.id }))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ops(t, "toasts.loadGroupMembersFailed"))
    } finally {
      setMembersLoading(false)
    }
  }, [selectedTeam, t])

  const loadAvailability = useCallback(async () => {
    if (!selectedTeam) {
      setMemberAvailability([])
      return
    }
    setAvailabilityLoading(true)
    try {
      const availabilityRows = await fetchAgentTeamMemberAvailability(selectedTeam.id)
      setMemberAvailability(availabilityRows)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ops(t, "toasts.loadAvailabilityFailed"))
    } finally {
      setAvailabilityLoading(false)
    }
  }, [selectedTeam, t])

  const loadTickets = useCallback(async () => {
    if (!selectedTeam) {
      setTickets([])
      return
    }
    setTicketsLoading(true)
    try {
      const ticketResponse = await fetchTickets({
        ...(selectedTeam.teamType === TECHNICAL_TEAM_TYPE
          ? { team_id: selectedTeam.id }
          : { product_id: selectedTeam.productId }),
        page: 1,
        page_size: 50,
      })
      if (!ticketResponse.success) {
        throw new Error(ticketResponse.error?.message || ops(t, selectedTeam.teamType === TECHNICAL_TEAM_TYPE ? "toasts.loadMaintenanceTicketsFailed" : "toasts.loadProductTicketsFailed"))
      }
      setTickets(ticketResponse.data.items)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ops(t, selectedTeam.teamType === TECHNICAL_TEAM_TYPE ? "toasts.loadMaintenanceTicketsFailed" : "toasts.loadProductTicketsFailed"))
    } finally {
      setTicketsLoading(false)
    }
  }, [selectedTeam, t])

  const loadSelectedTeam = useCallback(async () => {
    if (!selectedTeam) {
      setMembers([])
      setMemberAvailability([])
      setTickets([])
      setAssigneeByTicket({})
      return
    }
    await Promise.allSettled([
      loadMembers(),
      loadAvailability(),
      loadTickets(),
    ])
  }, [loadAvailability, loadMembers, loadTickets, selectedTeam])

  const refreshOrganization = useCallback(async () => {
    await Promise.allSettled([
      loadTeams(),
      loadDepartments(),
      loadSelectedTeam(),
    ])
  }, [loadDepartments, loadSelectedTeam, loadTeams])

  useEffect(() => {
    void loadTeams()
  }, [loadTeams])

  useEffect(() => {
    void loadDepartments()
  }, [loadDepartments])

  useEffect(() => {
    void loadSelectedTeam()
  }, [loadSelectedTeam])

  useEffect(() => {
    setAssigneeByTicket((current) => buildAssigneeSelection(tickets, memberAvailability, current))
  }, [memberAvailability, tickets])

  async function handleAssign(ticket: TicketListItem) {
    const assigneeId = Number(assigneeByTicket[ticket.id])
    if (!Number.isFinite(assigneeId) || assigneeId <= 0) {
      toast.error(ops(t, "toasts.selectRepairMemberFirst"))
      return
    }
    if (!manuallyAssignableMembers.some((item) => item.userId === assigneeId)) {
      toast.error(ops(t, "toasts.dispatchUnavailable"))
      return
    }
    setAssigningTicketId(ticket.id)
    try {
      const response = await assignTicket(ticket.id, assigneeId, ops(t, isTechnicalTeam ? "maintenanceAssignNote" : "assignNote"))
      if (!response.success) {
        throw new Error(response.error?.message || ops(t, "toasts.assignFailed"))
      }
      toast.success(ops(t, "toasts.ticketAssigned", { ticketNo: ticket.ticket_no }))
      await loadSelectedTeam()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ops(t, "toasts.assignFailed"))
    } finally {
      setAssigningTicketId(null)
    }
  }

  async function handleAssignmentModeChange(value: string) {
    if (!selectedTeam || selectedTeam.assignmentMode === value) {
      return
    }
    setSavingRule(true)
    try {
      await updateAgentTeam({
        id: selectedTeam.id,
        name: selectedTeam.name,
        leaderUserId: selectedTeam.leaderUserId,
        assignmentMode: value,
        status: selectedTeam.status,
        description: selectedTeam.description,
        remark: selectedTeam.remark,
      })
      setTeams((current) =>
        current.map((item) => (item.id === selectedTeam.id ? { ...item, assignmentMode: value } : item)),
      )
      toast.success(value === "weighted" ? ops(t, "toasts.assignmentWeightedSaved") : ops(t, "toasts.assignmentAverageSaved"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ops(t, "toasts.assignmentRuleSaveFailed"))
    } finally {
      setSavingRule(false)
    }
  }

  async function handleLeaderChange(value: string) {
    if (!selectedTeam) {
      return
    }
    const leaderUserId = Number(value)
    if (!Number.isFinite(leaderUserId) || leaderUserId <= 0 || selectedTeam.leaderUserId === leaderUserId) {
      return
    }
    setSavingLeader(true)
    try {
      await updateAgentTeam({
        id: selectedTeam.id,
        name: selectedTeam.name,
        leaderUserId,
        assignmentMode: selectedTeam.assignmentMode,
        status: selectedTeam.status,
        description: selectedTeam.description,
        remark: selectedTeam.remark,
      })
      const leader = members.find((member) => member.userId === leaderUserId)
      setTeams((current) =>
        current.map((item) => (item.id === selectedTeam.id ? {
          ...item,
          leaderUserId,
          leaderNickname: agentMemberName(leader, item.leaderNickname),
          leaderUsername: leader?.username || item.leaderUsername,
        } : item)),
      )
      toast.success(ops(t, "toasts.leaderChanged", { name: agentMemberName(leader, ops(t, "toasts.leaderFallback")) }))
      await loadSelectedTeam()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ops(t, "toasts.leaderSaveFailed"))
    } finally {
      setSavingLeader(false)
    }
  }

  async function handleMemberDispatchChange(
    member: AdminAgentProfile,
    patch: { dispatchEnabled?: boolean; dispatchWeight?: number },
  ) {
    if (!selectedTeam) {
      return
    }
    const dispatchEnabled = patch.dispatchEnabled ?? member.teamDispatchEnabled
    const dispatchWeight = Math.max(1, patch.dispatchWeight ?? member.dispatchWeight ?? 1)
    setSavingMemberId(member.id)
    try {
      await upsertAgentTeamMember({
        teamId: selectedTeam.id,
        userId: member.userId,
        dispatchEnabled,
        dispatchWeight,
      })
      setMembers((current) =>
        current.map((item) =>
          item.id === member.id
            ? { ...item, teamDispatchEnabled: dispatchEnabled, dispatchWeight }
            : item,
        ),
      )
      toast.success(ops(t, "toasts.memberDispatchSaved"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ops(t, "toasts.memberDispatchSaveFailed"))
    } finally {
      setSavingMemberId(null)
    }
  }

  async function handleRemoveMember(member: AdminAgentProfile) {
    if (!selectedTeam) return
    if (!isTechnicalTeam && selectedTeam.leaderUserId === member.userId) {
      toast.error(ops(t, "toasts.removeLeaderFirst"))
      return
    }
    const name = agentMemberName(member)
    const accepted = await confirm({
      title: ops(t, isTechnicalTeam ? "confirm.removeMaintenanceTitle" : "confirm.removeTitle", { name }),
      confirmText: ops(t, "confirm.removeConfirm"),
      cancelText: ops(t, "confirm.cancel"),
      variant: "destructive",
    })
    if (!accepted) return
    setSavingMemberId(member.id)
    try {
      const result = await removeAgentTeamMember({ teamId: selectedTeam.id, userId: member.userId })
      const recovered = result.pendingTicketsRecovered + result.pendingConversationsRecovered
      toast.success(
        result.alreadyRemoved
          ? ops(t, isTechnicalTeam ? "toasts.removeMaintenanceMemberMissing" : "toasts.removeMemberMissing", { name })
          : recovered > 0
            ? ops(t, isTechnicalTeam ? "toasts.removeMaintenanceMemberDoneRecovered" : "toasts.removeMemberDoneRecovered", { name, count: recovered })
            : ops(t, isTechnicalTeam ? "toasts.removeMaintenanceMemberDone" : "toasts.removeMemberDone", { name }),
      )
      await loadSelectedTeam()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : ops(t, isTechnicalTeam ? "toasts.removeMaintenanceMemberFailed" : "toasts.removeMemberFailed"))
    } finally {
      setSavingMemberId(null)
    }
  }

  function renderOrdinaryDepartmentNode(node: DepartmentTreeNode): ReactNode {
    const department = node.department
    return (
      <div key={department.id} className="ml-4 border-l border-border pl-2">
        <div className="group flex items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent/60">
          <Building2Icon className="size-4 shrink-0 text-muted-foreground" />
          <span className="min-w-0 flex-1 truncate">{department.name}</span>
          <StatusTag tone="neutral" className="hidden shrink-0 group-hover:inline-flex">
            {ops(t, "tree.peopleCount", { count: department.member_count })}
          </StatusTag>
        </div>
        {node.children.length > 0 ? (
          <div className="mt-1 space-y-1">
            {node.children.map((child) => renderOrdinaryDepartmentNode(child))}
          </div>
        ) : null}
      </div>
    )
  }

  return (
    <PageShell
      title={ops(t, "header.orgManagement")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="rhd-railops-organization-page"
      actions={
        <RailopsButton onClick={() => void refreshOrganization()} disabled={organizationLoading || detailRefreshing}>
          <RefreshCwIcon className={organizationLoading || detailRefreshing ? "animate-spin" : undefined} />
          {ops(t, "header.refresh")}
        </RailopsButton>
      }
    >
      <section className="rhd-railops-org-page-workspace grid min-h-[36rem] overflow-hidden rounded-lg bg-[var(--railops-surface)] shadow-[var(--railops-card-shadow)] lg:grid-cols-[22rem_minmax(0,1fr)]">
        <aside className="rhd-railops-org-page-tree flex min-h-0 flex-col border-b border-[var(--railops-border-light)] bg-[var(--railops-surface-muted)] lg:border-r lg:border-b-0">
          <div className="border-b border-[var(--railops-border-light)] px-4 py-3">
            <div className="flex items-center gap-2 text-sm font-medium">
              <FolderTreeIcon className="size-4 text-muted-foreground" />
              {ops(t, "tree.title")}
              <StatusTag tone="neutral" className="ml-auto">{ops(t, "tree.treeView")}</StatusTag>
            </div>
          </div>
          <ScrollArea className="max-h-64 flex-1 lg:max-h-none">
            <div className="space-y-2 p-2">
              {teamTreeInitialLoading ? (
                Array.from({ length: 4 }).map((_, index) => (
                  <Skeleton.Node key={index} active className="w-full" style={{ width: "100%", height: 44 }} />
                ))
              ) : (
                <>
                  <div className="rounded-md bg-[var(--railops-surface)] p-2 shadow-[var(--railops-card-shadow)]">
                    <div className="flex items-center gap-2 rounded-md px-2 py-1.5 text-sm font-medium">
                      <Building2Icon className="size-4 text-muted-foreground" />
                      {rootDepartment?.name || ops(t, "tree.orgFallback")}
                    </div>
                    <div className="mt-1 ml-4 space-y-1 border-l border-[var(--railops-border-light)] pl-2">
                      <div className="rounded-md border border-dashed border-[var(--railops-border)] bg-[var(--railops-surface-muted)]">
                        {productTeams.length === 0 && technicalTeam ? (
                          <button
                            type="button"
                            className={cn(
                              "flex w-full items-center gap-2 rounded-md px-2 py-2 text-left text-sm transition-colors hover:bg-accent",
                              selectedTeamId === technicalTeam.id && "bg-accent text-accent-foreground",
                            )}
                            onClick={() => setSelectedTeamId(technicalTeam.id)}
                          >
                            <ShieldCheckIcon className="size-4 shrink-0 text-muted-foreground" />
                            <span className="min-w-0 flex-1">
                              <span className="block truncate font-medium">{technicalTeam.name}</span>
                              <span className="block truncate text-xs text-muted-foreground">{ops(t, "tree.defaultMaintenanceQueue")}</span>
                            </span>
                            <ChevronRightIcon className="size-4 shrink-0 text-muted-foreground" />
                          </button>
                        ) : (
                          <div className="flex items-center gap-2 px-2 py-1.5 text-sm font-medium">
                            <ShieldCheckIcon className="size-4 shrink-0 text-muted-foreground" />
                            <span className="min-w-0 flex-1 truncate">{technicalTeam?.name || ops(t, "tree.technicalTeamFallback")}</span>
                            <StatusTag tone="neutral" className="shrink-0">
                              <LockKeyholeIcon className="size-3" />
                              {ops(t, "tree.system")}
                            </StatusTag>
                          </div>
                        )}
                        {productTeams.length > 0 ? (
                          <div className="space-y-1 px-2 pb-2">
                            {productTeams.map((team) => (
                              <button
                                key={team.id}
                                type="button"
                                className={cn(
                                  "ml-4 flex w-[calc(100%-1rem)] items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors hover:bg-accent",
                                  selectedTeamId === team.id && "bg-accent text-accent-foreground",
                                )}
                                onClick={() => setSelectedTeamId(team.id)}
                              >
                                <PackageIcon className="size-4 shrink-0 text-muted-foreground" />
                                <span className="min-w-0 flex-1">
                                  <span className="block truncate font-medium">{team.productName || team.name}</span>
                                  <span className="block truncate text-xs text-muted-foreground">{ops(t, "tree.productAfterSalesQueue")}</span>
                                </span>
                                <ChevronRightIcon className="size-4 shrink-0 text-muted-foreground" />
                              </button>
                            ))}
                          </div>
                        ) : null}
                      </div>
                      {departmentTreeInitialLoading ? (
                        <div className="space-y-1">
                          {Array.from({ length: 3 }).map((_, index) => (
                            <Skeleton.Node key={index} active className="w-full" style={{ width: "100%", height: 34 }} />
                          ))}
                        </div>
                      ) : ordinaryDepartmentTree.length > 0 || detachedOrdinaryDepartmentTree.length > 0 ? (
                        <div className="space-y-1">
                          {[...ordinaryDepartmentTree, ...detachedOrdinaryDepartmentTree].map((node) =>
                            renderOrdinaryDepartmentNode(node),
                          )}
                        </div>
                      ) : (
                        <div className="rounded-md border border-dashed border-[var(--railops-border)] px-3 py-4 text-xs text-[var(--railops-text-secondary)]">
                          {ops(t, "tree.noOrdinaryDepartment")}
                        </div>
                      )}
                    </div>
                  </div>
                </>
              )}
            </div>
          </ScrollArea>
        </aside>

        <div className="rhd-railops-org-page-detail min-w-0">
          {selectedTeam ? (
            <div className="divide-y divide-border">
              <div className="flex flex-col gap-3 px-4 py-4 sm:flex-row sm:items-center sm:justify-between lg:px-5">
                <div>
                  <div className="flex flex-wrap items-center gap-2">
                    <h2 className="text-base font-semibold">{selectedTeam.productName || selectedTeam.name}</h2>
                    <StatusTag tone="neutral">{ops(t, isTechnicalTeam ? "header.technicalGroup" : "header.productGroup")}</StatusTag>
                    {selectedTeam.status === 0 ? <StatusTag tone="blue">{ops(t, "header.enabled")}</StatusTag> : <StatusTag tone="neutral">{ops(t, "header.disabled")}</StatusTag>}
                  </div>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  {!isTechnicalTeam ? <div className="min-w-52">
                    <SelectField
                      style={{ marginBottom: 0 }}
                      selectProps={{
                        value: selectedTeam.leaderUserId > 0 ? String(selectedTeam.leaderUserId) : "",
                        onChange: (value) => {
                          if (typeof value === "string") void handleLeaderChange(value)
                        },
                        disabled: savingLeader || membersLoading || members.length === 0,
                        options: [
                          ...(selectedTeam.leaderUserId > 0 && !members.some((member) => member.userId === selectedTeam.leaderUserId)
                            ? [{
                                value: String(selectedTeam.leaderUserId),
                                label: ops(t, "header.leaderPrefix", {
                                  name: selectedTeam.leaderNickname || selectedTeam.leaderUsername || ops(t, "header.selectProductLeader"),
                                }),
                              }]
                            : []),
                          ...members.map((member) => ({
                            value: String(member.userId),
                            label: member.userId === selectedTeam.leaderUserId
                              ? ops(t, "header.leaderPrefix", { name: agentMemberName(member) })
                              : agentMemberName(member),
                          })),
                        ],
                        style: { width: 208 },
                      }}
                    />
                  </div> : null}
                  <div className="min-w-44">
                    <SelectField
                      style={{ marginBottom: 0 }}
                      selectProps={{
                        value: selectedTeam.assignmentMode || "balanced",
                        onChange: (value) => {
                          if (typeof value === "string") void handleAssignmentModeChange(value)
                        },
                        disabled: savingRule,
                        options: [
                          ...(ASSIGNMENT_MODES.some((mode) => mode.value === (selectedTeam.assignmentMode || "balanced"))
                            ? []
                            : [{ value: selectedTeam.assignmentMode || "balanced", label: assignmentModeLabel(selectedTeam.assignmentMode, t) }]),
                          ...ASSIGNMENT_MODES.map((mode) => ({ value: mode.value, label: ops(t, mode.labelKey) })),
                        ],
                        style: { width: 176 },
                      }}
                    />
                  </div>
                  <Link href={`/enterprise/org/members?teamId=${selectedTeam.id}`}>
                    <RailopsButton size="small">
                      <UsersRoundIcon className="size-4" />{ops(t, "header.memberManage")}
                    </RailopsButton>
                  </Link>
                  {!isTechnicalTeam ? <Link href={`/enterprise/org/schedules?teamId=${selectedTeam.id}`}>
                    <RailopsButton size="small">
                      <UserRoundCheckIcon className="size-4" />{ops(t, "header.availabilitySchedule")}
                    </RailopsButton>
                  </Link> : null}
                </div>
              </div>

              <div className="grid gap-0 xl:grid-cols-2 xl:divide-x xl:divide-border">
                <section className="p-4 lg:p-5">
                  <div className="mb-3 flex items-center justify-between gap-3">
                    <div>
                      <h3 className="text-sm font-semibold">{ops(t, "membersPanel.title")}</h3>
                    </div>
                    <StatusTag tone="neutral">{ops(t, "tree.peopleCount", { count: members.length })}</StatusTag>
                  </div>
                  <div className="space-y-2">
                    {membersLoading ? (
                      <Skeleton.Node active className="w-full" style={{ width: "100%", height: 64 }} />
                    ) : members.length > 0 ? members.slice(0, 6).map((member) => (
                      <div key={member.id} className="flex flex-col gap-3 border-b border-border py-3 last:border-0 sm:flex-row sm:items-start sm:justify-between">
                        <div className="flex min-w-0 flex-1 items-start gap-3">
                          <div className="flex size-9 shrink-0 items-center justify-center rounded-md bg-muted">
                            <UserRoundCheckIcon className="size-4 text-muted-foreground" />
                          </div>
                          <div className="min-w-0 flex-1">
                            <div className="truncate text-sm font-medium">{agentMemberName(member)}</div>
                            <div className="truncate text-xs text-muted-foreground">
                              {ops(t, isTechnicalTeam ? "membersPanel.maintenanceMemberCapacityDetail" : "membersPanel.memberCapacityDetail", {
                                agentCode: member.agentCode,
                                capacity: member.maxConcurrentCount,
                                teamCount: member.teamIds?.length || 1,
                              })}
                            </div>
                            <div className="mt-2 flex flex-wrap items-center gap-1.5">
                              <StatusTag tone={member.autoAssignEnabled ? "success" : "neutral"}>
                                {ops(t, member.autoAssignEnabled ? "memberState.globalAutoDispatchEnabled" : "memberState.globalAutoDispatchDisabled")}
                              </StatusTag>
                              <StatusTag tone={member.teamDispatchEnabled ? "blue" : "neutral"}>
                                {ops(t, member.teamDispatchEnabled ? "memberState.teamAutoDispatchEnabled" : "memberState.teamAutoDispatchDisabled")}
                              </StatusTag>
                              <StatusTag tone="neutral">{ops(t, "memberState.manualDispatchAvailable")}</StatusTag>
                              {member.serviceStatus === 1 ? <StatusTag tone="warning">{ops(t, "memberState.busy")}</StatusTag> : null}
                            </div>
                          </div>
                        </div>
                        <div className="flex shrink-0 flex-wrap items-center gap-2 sm:justify-end">
                          <SelectField
                            style={{ marginBottom: 0 }}
                            selectProps={{
                              value: String(member.dispatchWeight || 1),
                              onChange: (value) => void handleMemberDispatchChange(member, { dispatchWeight: Number(value) }),
                              disabled: savingMemberId === member.id,
                              options: [
                                ...(!DISPATCH_WEIGHTS.includes(member.dispatchWeight || 1)
                                  ? [{ value: String(member.dispatchWeight || 1), label: ops(t, "membersPanel.weight", { weight: member.dispatchWeight || 1 }) }]
                                  : []),
                                ...DISPATCH_WEIGHTS.map((weight) => ({ value: String(weight), label: ops(t, "membersPanel.weight", { weight }) })),
                              ],
                              style: { width: 80 },
                            }}
                          />
                          <RailopsButton
                            size="small"
                            variant={member.teamDispatchEnabled ? "default" : "primary"}
                            disabled={savingMemberId === member.id || !member.autoAssignEnabled}
                            title={!member.autoAssignEnabled ? ops(t, "memberState.globalAutoDispatchDisabled") : undefined}
                            icon={<PowerIcon className="size-3.5" />}
                            onClick={() => void handleMemberDispatchChange(member, { dispatchEnabled: !member.teamDispatchEnabled })}
                          >
                            {memberDispatchActionLabel(member, t)}
                          </RailopsButton>
                          <IconButton
                            icon={<UserMinusIcon className="size-4" />}
                            tooltip={!isTechnicalTeam && selectedTeam.leaderUserId === member.userId
                              ? ops(t, "membersPanel.changeLeaderFirstTooltip")
                              : ops(t, isTechnicalTeam ? "membersPanel.removeMaintenanceTooltip" : "membersPanel.removeTooltip")}
                            aria-label={ops(t, isTechnicalTeam ? "membersPanel.removeMaintenanceAria" : "membersPanel.removeAria", { name: agentMemberName(member) })}
                            danger
                            disabled={savingMemberId === member.id}
                            onClick={() => void handleRemoveMember(member)}
                          />
                        </div>
                      </div>
                    )) : (
                      <div className="py-8 text-center text-sm text-muted-foreground">{ops(t, "membersPanel.noTeamMembers")}</div>
                    )}
                  </div>
                </section>

                <section className="border-t border-[var(--railops-border-light)] p-4 xl:border-t-0 lg:p-5">
                  <div className="mb-3 flex items-center justify-between gap-3">
                    <div>
                      <h3 className="text-sm font-semibold">{ops(t, "dutyPanel.title")}</h3>
                      <p className="mt-0.5 text-xs text-muted-foreground">{ops(t, "dutyPanel.hint")}</p>
                    </div>
                    <div className="flex flex-wrap items-center justify-end gap-2">
                      <StatusTag tone="neutral">
                        {ops(t, "dutyPanel.availableCount", { available: availableMembers.length, total: memberAvailability.length })}
                      </StatusTag>
                      <StatusTag tone="neutral">
                        {ops(t, "dutyPanel.manualAssignableCount", { assignable: manuallyAssignableMembers.length, total: memberAvailability.length })}
                      </StatusTag>
                    </div>
                  </div>
                  <div className="space-y-3" data-testid="team-availability-timeline">
                    {availabilityLoading ? (
                      Array.from({ length: 3 }).map((_, index) => (
                        <Skeleton.Node key={index} active className="w-full" style={{ width: "100%", height: 80 }} />
                      ))
                    ) : memberAvailability.length > 0 ? memberAvailability.slice(0, 8).map((member) => {
                      const leaveStyle = todayLeaveStyle(member)
                      return (
                        <div key={member.memberId || member.userId} className="space-y-2 border-b border-border pb-3 last:border-0">
                          <div className="flex items-center justify-between gap-3">
                            <div className="min-w-0">
                              <div className="truncate text-sm font-medium">{agentMemberName(member)}</div>
                              <div className="truncate text-xs text-muted-foreground">
                                {workdayLabel(member.workdays, t)} {member.startTime} - {member.endTime}
                                {member.activeLeave ? ops(t, "dutyPanel.leaveInline", { window: activeLeaveLabel(member, t) }) : ""}
                              </div>
                            </div>
                            <StatusTag tone="neutral">
                              {member.availableNow ? ops(t, "dutyPanel.available") : availabilityReasonLabel(member, t)}
                            </StatusTag>
                          </div>
                          <div className="relative h-9 rounded-md border border-[var(--railops-border-light)] bg-[var(--railops-surface-muted)]">
                            <div
                              className={cn(
                                "absolute top-1 bottom-1 rounded-sm",
                                member.availableNow ? "bg-primary" : "bg-muted-foreground/25",
                              )}
                              style={dayWindowStyle(member.startTime, member.endTime)}
                            />
                            {leaveStyle ? (
                              <div
                                className="absolute top-1 bottom-1 rounded-sm bg-amber-500/90"
                                style={leaveStyle}
                              />
                            ) : null}
                            <div className="absolute inset-x-0 top-full mt-1 flex justify-between text-rhd-2xs text-muted-foreground">
                              <span>00</span>
                              <span>12</span>
                              <span>24</span>
                            </div>
                          </div>
                        </div>
                      )
                    }) : (
                      <div className="py-8 text-center text-sm text-muted-foreground">{ops(t, isTechnicalTeam ? "dutyPanel.noMaintenanceEngineers" : "dutyPanel.noEngineers")}</div>
                    )}
                  </div>
                </section>
              </div>

              <section className="p-4 lg:p-5">
                <div className="mb-3 flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
                  <div>
                    <h3 className="text-sm font-semibold">{ops(t, isTechnicalTeam ? "ticketsPanel.maintenanceTitle" : "ticketsPanel.title")}</h3>
                  </div>
                  <StatusTag tone="neutral">
                    {ops(t, "ticketsPanel.summary", { active: activeTicketCount, unassigned: unassignedCount })}
                  </StatusTag>
                </div>
                {unassignedTickets.length > 0 ? (
                  <div className="mb-3 rounded-lg bg-[var(--railops-warning-bg)] p-3 text-xs text-[#92400e]">
                    <div className="font-medium">{ops(t, isTechnicalTeam ? "ticketsPanel.maintenanceUnassignedAlert" : "ticketsPanel.unassignedAlert", { count: unassignedTickets.length })}</div>
                  </div>
                ) : null}
                <div className="rhd-railops-native-table-wrap overflow-x-auto rounded-md bg-[var(--railops-surface)] shadow-[var(--railops-card-shadow)]">
                  <DataTable
                    className="rhd-railops-native-table rhd-railops-org-page-table"
                    size="small"
                    rowKey={(record: TicketListItem) => record.id}
                    dataSource={ticketRows}
                    loading={ticketsLoading}
                    emptyDescription={ops(t, "ticketsPanel.empty")}
                    columns={[
                      {
                        title: ops(t, "ticketsPanel.colTicket"),
                        dataIndex: "ticket_no",
                        key: "ticket_no",
                        width: 260,
                        render: (_, ticket: TicketListItem) => (
                          <div>
                            <div className="font-medium">{ticket.ticket_no}</div>
                            <div className="max-w-80 truncate text-xs text-muted-foreground">{ticket.title}</div>
                          </div>
                        ),
                      },
                      {
                        title: ops(t, "ticketsPanel.colStatus"),
                        dataIndex: "status",
                        key: "status",
                        width: 150,
                        render: (_, ticket: TicketListItem) => (
                          <div className="flex flex-wrap gap-1.5">
                            <StatusTag tone="neutral">{statusLabel(ticket.status, t)}</StatusTag>
                            {!ticket.assignee_id ? <StatusTag tone="neutral">{ops(t, "ticketsPanel.unassigned")}</StatusTag> : null}
                          </div>
                        ),
                      },
                      {
                        title: ops(t, "ticketsPanel.colOwner"),
                        dataIndex: "assignee_name",
                        key: "assignee_name",
                        width: 190,
                        render: (_, ticket: TicketListItem) => (
                          <div>
                            <div className="text-sm">{ticket.assignee_name || ticket.team_name || selectedTeam.name}</div>
                            {!ticket.assignee_id ? <div className="text-xs text-muted-foreground">{ops(t, isTechnicalTeam ? "ticketsPanel.maintenanceUnclaimed" : "ticketsPanel.unclaimed")}</div> : null}
                          </div>
                        ),
                      },
                      {
                        title: ops(t, "ticketsPanel.colAssignee"),
                        dataIndex: "assignee_id",
                        key: "assign",
                        width: 330,
                        render: (_, ticket: TicketListItem) => {
                          const selectedAssignee = assigneeByTicket[ticket.id] || ""
                          const selectedMember = memberAvailability.find((member) => String(member.userId) === selectedAssignee)
                          const selectedAssignable = Boolean(selectedMember && isManuallyAssignableMember(selectedMember))
                          const selectedMemberMissing = Boolean(selectedAssignee && !selectedMember)
                          return (
                            <div className="flex items-center gap-2">
                              <SelectField
                                style={{ marginBottom: 0 }}
                                selectProps={{
                                  value: selectedAssignee,
                                  placeholder: ops(t, "ticketsPanel.selectAssignableMember"),
                                  onChange: (value) => setAssigneeByTicket((current) => ({ ...current, [ticket.id]: value ?? "" })),
                                  options: [
                                    ...(selectedMemberMissing
                                      ? [{ value: selectedAssignee, label: ops(t, "ticketsPanel.selectAssignableMember"), disabled: true }]
                                      : []),
                                    ...memberAvailability.map((member) => ({
                                      value: String(member.userId),
                                      label: dispatchMemberOptionLabel(member, t),
                                      disabled: !isManuallyAssignableMember(member),
                                    })),
                                  ],
                                  notFoundContent: ops(t, "ticketsPanel.noAssignableMembers"),
                                  style: { width: 220 },
                                }}
                              />
                              <RailopsButton
                                size="small"
                                variant="primary"
                                disabled={!selectedAssignable || assigningTicketId === ticket.id}
                                onClick={() => void handleAssign(ticket)}
                              >
                                {assigningTicketId === ticket.id ? <RefreshCwIcon className="size-4 animate-spin" /> : <UserRoundCheckIcon className="size-4" />}
                                {ops(t, "ticketsPanel.assign")}
                              </RailopsButton>
                            </div>
                          )
                        },
                      },
                    ]}
                  />
                </div>
              </section>
            </div>
          ) : (
            <div className="flex min-h-[30rem] items-center justify-center px-6 text-center text-xs text-[var(--railops-text-secondary)]">
              {ops(t, "empty.noSupportGroups")}
            </div>
          )}
        </div>
      </section>
    </PageShell>
  )
}

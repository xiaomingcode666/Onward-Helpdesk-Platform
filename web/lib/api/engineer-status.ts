import { request } from "@/lib/api/client"
import type { AdminAgentScheduleException, AdminAgentTeam, AdminAgentTeamSchedule, AdminAgentTeamScheduleTemplate } from "@/lib/api/admin"
import type { TicketListItem } from "@/lib/api/types"

export type EngineerWorkStatus = "available" | "busy" | "leave" | "offline" | "custom"

export type EngineerWorkStatusDetail = {
  status: EngineerWorkStatus
  note: string
  availableAt?: string
  confirmedAt?: string
  statusChangedAt?: string
  needsConfirmation: boolean
}

export type EngineerBriefing = {
  isEngineer: boolean
  workStatus: EngineerWorkStatusDetail
  teams: Array<{ id: number; name: string; productId: number }>
  unassignedTicketCount?: number
  myOpenTicketCount?: number
  unassignedTickets: TicketListItem[]
  myOpenTickets: TicketListItem[]
}

export type EngineerWorkSchedule = {
  isEngineer: boolean
  teams: AdminAgentTeam[]
  schedules: AdminAgentTeamSchedule[]
  draftSchedules: AdminAgentTeamSchedule[]
  baseSchedule?: AdminAgentTeamScheduleTemplate
  timezone: string
  exceptions: AdminAgentScheduleException[]
}

export function fetchEngineerBriefing() {
  return request<EngineerBriefing>("/api/dashboard/agent/self/briefing")
}

export function fetchEngineerWorkSchedule() {
  return request<EngineerWorkSchedule>("/api/dashboard/agent/self/work-schedule")
}

export function createEngineerLeave(payload: {
  requestKey: string
  startAt: string
  endAt: string
  reason: string
}) {
  return request<AdminAgentScheduleException>("/api/dashboard/agent/self/leave/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function cancelEngineerLeave(id: number) {
  return request<AdminAgentScheduleException>("/api/dashboard/agent/self/leave/cancel", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}

export function updateEngineerWorkStatus(payload: {
  status: EngineerWorkStatus
  note: string
  availableAt?: string
}) {
  return request<EngineerWorkStatusDetail>("/api/dashboard/agent/self/work-status", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

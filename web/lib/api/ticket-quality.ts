import { apiGet, apiPost } from "@/lib/api/client"
import type { ApiResponse } from "@/lib/api/types"

export type TicketQualityReason = "random" | "high_priority" | "new_agent" | "complaint" | "reopened" | "risk" | string
export type TicketQualityStatus = "pending" | "in_progress" | "completed" | string

export type TicketQualitySample = {
  id: number
  ticket_id: number
  ticket_no: string
  title: string
  priority: string
  ticket_status: string
  assignee_id: number
  assignee_name: string
  period_key: string
  reasons: TicketQualityReason[]
  status: TicketQualityStatus
  reviewer_id: number
  review_note: string
  sampled_at: string
  started_at: string
  completed_at: string
}

export type TicketQualitySummary = {
  total: number
  pending: number
  in_progress: number
  completed: number
}

export type TicketQualityPage = {
  items: TicketQualitySample[]
  total: number
  page: number
  page_size: number
  total_pages: number
  summary: TicketQualitySummary
}

export async function fetchTicketQualitySamples(params?: {
  page?: number
  page_size?: number
  period_key?: string
  status?: string
  reason?: string
  search?: string
}): Promise<ApiResponse<TicketQualityPage>> {
  return apiGet<TicketQualityPage>("/ticket-quality/samples", params)
}

export async function generateTicketQualitySamples(body?: {
  period_key?: string
  random_count?: number
}): Promise<ApiResponse<{ created: number }>> {
  return apiPost<{ created: number }>("/ticket-quality/samples/_generate", body)
}

export async function startTicketQualitySample(id: number): Promise<ApiResponse<{ success: boolean }>> {
  return apiPost<{ success: boolean }>(`/ticket-quality/samples/${id}/_start`, {})
}

export async function completeTicketQualitySample(id: number, note: string): Promise<ApiResponse<{ success: boolean }>> {
  return apiPost<{ success: boolean }>(`/ticket-quality/samples/${id}/_complete`, { note })
}

export type TicketQualityAnalysisBucket = {
  value: string
  label: string
  reviews: number
  average: number
  pass_rate: number
}

export type TicketQualityAnalysis = {
  from: string
  to: string
  can_view_agents: boolean
  summary: TicketQualityAnalysisBucket
  by_project: TicketQualityAnalysisBucket[]
  by_team: TicketQualityAnalysisBucket[]
  by_agent?: TicketQualityAnalysisBucket[]
  by_category: TicketQualityAnalysisBucket[]
  by_channel: TicketQualityAnalysisBucket[]
}

export async function fetchTicketQualityAnalysis(params?: {
  from?: string
  to?: string
  project?: string
  team_id?: number
  agent_id?: number
  category?: string
  channel?: string
}): Promise<ApiResponse<TicketQualityAnalysis>> {
  return apiGet<TicketQualityAnalysis>("/ticket-quality/analysis", params)
}

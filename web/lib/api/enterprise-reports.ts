// ============================================================
// Enterprise Reports API
// ============================================================
// GET /api/enterprise/v1/reports/overview — workbench/report overview data
// GET /api/enterprise/v1/reports/top-failures — product failure ranking
// GET /api/enterprise/v1/reports/team-performance — engineer/team performance
// GET /api/enterprise/v1/reports/supplier-performance — supplier ticket and rating performance
// ============================================================

import { apiGet } from "@/lib/api/client"
import type { ApiResponse } from "@/lib/api/types"

export interface DashboardOverview {
  total_products: number
  total_devices: number
  total_tickets: number
  pending_tickets: number
  in_progress_tickets: number
  sla_at_risk: number
  new_today: number
  ai_sessions: number
  ai_resolve_rate: number
  expert_intervention_rate: number | null
  avoided_trips: number | null
  first_time_fix_rate: number | null
  avg_downtime_minutes: number | null
  knowledge_reuse_rate: number | null
  remote_resolution_rate: number | null
  outcome_metric_coverage_rate: number | null
  outcome_metric_sample_size: number
  recent_activities: Activity[]
  queue_tickets: QueueTicket[]
}

export interface Activity {
  id?: string
  type?: string
  message: string
  created_at?: string
  time?: string
}

export interface QueueTicket {
  ticket_no: string
  customer: string
  summary: string
  level: string
  state: string
  owner: string
  label: string
}

export interface ReportDateRangeParams extends Record<string, unknown> {
  start?: string
  end?: string
}

export interface ProductFailureRank {
  product_id: string
  product_name: string
  failure_count: number
  escalation_rate: number
  avg_resolution_hours: number
  trend?: string
}

export interface AgentPerformance {
  agent_id: string
  agent_name: string
  tickets_resolved: number
  tickets_assigned: number
  avg_response_minutes: number
  avg_handle_minutes: number
  satisfaction: number
  meetings_held: number
}

export interface SupplierPerformance {
  supplier_id: string
  supplier_name: string
  supplier_no: string
  tickets_assigned: number
  tickets_processing: number
  tickets_completed: number
  tickets_responded: number
  avg_response_minutes: number
  completion_rate: number
  average_rating: number
  rating_count: number
}

export interface TrendPoint {
  date: string
  value: number
  label?: string
}

export async function fetchReportsOverview(
  params?: ReportDateRangeParams,
): Promise<ApiResponse<DashboardOverview>> {
  return apiGet<DashboardOverview>("/reports/overview", params)
}

export async function fetchReportTopFailures(params?: {
  limit?: number
  start?: string
  end?: string
}): Promise<ApiResponse<ProductFailureRank[]>> {
  return apiGet<ProductFailureRank[]>("/reports/top-failures", params)
}

export async function fetchReportTeamPerformance(
  params?: ReportDateRangeParams,
): Promise<ApiResponse<AgentPerformance[]>> {
  return apiGet<AgentPerformance[]>("/reports/team-performance", params)
}

export async function fetchReportSupplierPerformance(
  params?: ReportDateRangeParams,
): Promise<ApiResponse<SupplierPerformance[]>> {
  return apiGet<SupplierPerformance[]>("/reports/supplier-performance", params)
}

export async function fetchReportTrends(params?: {
  metric?: "tickets" | "ai_sessions" | "tokens" | "satisfaction"
  period?: string
  productId?: string
}): Promise<ApiResponse<TrendPoint[]>> {
  return apiGet<TrendPoint[]>("/reports/trends", params)
}

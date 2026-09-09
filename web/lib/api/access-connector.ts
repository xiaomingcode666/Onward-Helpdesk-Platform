// ============================================================
// Access Connector API
// ============================================================
// GET    /api/enterprise/v1/access/connectors
// GET    /api/enterprise/v1/access/connector/{id}/status
// GET    /api/enterprise/v1/access/connector/{id}/logs
// POST   /api/enterprise/v1/access/connector/create
// POST   /api/enterprise/v1/access/connector/{id}/test
// ============================================================

import { apiGet, apiPost } from "@/lib/api/client"
import type { ApiResponse, Pagination } from "@/lib/api/types"

// ---- Types ----

export interface ConnectorListItem {
  id: number
  name: string
  connector_type: string
  base_url: string
  auth_type: string
  active: boolean
  health_status: string // "healthy" | "degraded" | "unhealthy" | "unknown"
  last_health_check_at: string
  last_message_at?: string
  created_at: string
  auth_config?: string
}

export interface ConnectorDetail {
  id: number
  name: string
  connector_type: string
  base_url: string
  auth_type: string
  auth_config: string
  field_mapping: string
  template_code: string
  active: boolean
  health_status: string
  last_health_check_at: string
  created_at: string
  updated_at: string
}

export interface ConnectorStatus {
  connectorId: number
  status: string
  lastChecked: string
  lastMessageAt?: string
  latencyMs: number
  errorMessage?: string
}

export interface ConnectorCallLog {
  id: number
  connector_id: number
  method: string
  path: string
  status_code: number
  duration_ms: number
  request_body: string
  response_body: string
  error_message: string
  created_at: string
}

export interface HealthCheckPoint {
  timestamp: string
  status: string
  response_time_ms: number
}

export interface FieldMapping {
  source_field: string
  target_field: string
  transform: string
  required: boolean
}

export interface CreateConnectorPayload {
  name: string
  connectorType: string
  baseUrl?: string
  authType?: string
  authConfig?: string
  fieldMapping?: string
  templateCode?: string
}

export interface ConnectorTestResult {
  success: boolean
  message: string
  response_time_ms: number
  status_code: number
}

// ---- API Functions ----

export async function fetchConnectors(): Promise<ApiResponse<ConnectorListItem[]>> {
  return apiGet<ConnectorListItem[]>("/access/connectors")
}

export async function fetchConnectorStatus(id: number): Promise<ApiResponse<ConnectorStatus>> {
  return apiGet<ConnectorStatus>(`/access/connector/${id}/status`)
}

export async function fetchConnectorLogs(id: number, page?: number, pageSize?: number): Promise<ApiResponse<{ logs: ConnectorCallLog[]; pagination: Pagination }>> {
  const params = new URLSearchParams()
  if (page) params.set("page", String(page))
  if (pageSize) params.set("pageSize", String(pageSize))
  const qs = params.toString()
  return apiGet(`/access/connector/${id}/logs${qs ? `?${qs}` : ""}`)
}

export async function createConnector(payload: CreateConnectorPayload): Promise<ApiResponse<ConnectorDetail>> {
  return apiPost<ConnectorDetail>("/access/connector/create", payload)
}

export async function updateConnector(id: number, payload: CreateConnectorPayload): Promise<ApiResponse<ConnectorDetail>> {
  return apiPost<ConnectorDetail>("/access/connector/update", { id, ...payload })
}

export async function testConnector(id: number): Promise<ApiResponse<ConnectorTestResult>> {
  return apiPost<ConnectorTestResult>(`/access/connector/${id}/test`, {})
}

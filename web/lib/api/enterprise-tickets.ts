// ============================================================
// Enterprise Ticket API
// ============================================================
// GET  /api/enterprise/v1/tickets                         — list tickets
// GET  /api/enterprise/v1/tickets/{id}                    — ticket detail (TicketAggregateDTO)
// POST /api/enterprise/v1/tickets                         — create ticket
// PATCH /api/enterprise/v1/tickets/{id}                   — update ticket
// POST /api/enterprise/v1/tickets/{id}/_assign            — assign ticket
// POST /api/enterprise/v1/tickets/{id}/_close             — close ticket
// POST /api/enterprise/v1/tickets/{id}/_transfer          — transfer ticket
// ============================================================

import { apiGet, apiPost, apiPatch, apiPut, buildFilterQuery, listQueryToParams } from "@/lib/api/client"
import type {
  ApiResponse,
  ListQuery,
  TicketListItem,
  TicketAggregateDTO,
  TicketPriority,
  CreateTicketPayload,
  CreateTicketKnowledgeCandidatePayload,
  TicketActionPermissionsDTO,
  TicketKnowledgeCandidateDTO,
} from "@/lib/api/types"
import type { EnterpriseCustomerInviteResult } from "@/lib/api/platform-iam"
import type { TicketIntakePolicy, IntakeDraft } from "@/lib/ticket-intake"

export function fetchTicketIntakePolicy() {
  return apiGet<TicketIntakePolicy>("/ticket-settings/intake")
}

export function updateTicketIntakePolicy(policy: TicketIntakePolicy) {
  return apiPut<TicketIntakePolicy>("/ticket-settings/intake", policy)
}

export function completeTicketIntake(id: number, input: IntakeDraft, customerId: number) {
  const { project_key, ticket_type, caller_name, caller_phone, product_id, device_id, service_region } = input
  return apiPatch<TicketAggregateDTO>(`/tickets/${id}/intake`, { project_key, ticket_type, caller_name, caller_phone, product_id, device_id, service_region, customer_id: customerId })
}

export interface TicketListQuery extends ListQuery {
  status?: string
  priority?: string
  product_id?: number
  productId?: number
  device_id?: number
  deviceId?: number
  team_id?: number
  teamId?: number
  product_name?: string
  search?: string
  assignee_id?: number
  mine?: boolean
  sla_breached?: boolean
  sla_risk?: boolean
  conversation_id?: number
  conversationId?: number
}

export interface TicketListResponse {
  items: TicketListItem[]
  total: number
  page: number
  page_size: number
  total_pages: number
  has_more: boolean
}

export interface TicketSummaryResponse {
  total: number
  pending: number
  processing: number
  awaiting_customer: number
  sla_risk: number
  urgent: number
  done: number
  generated_at: string
}

export interface TicketCustomerOption {
  customer_id: number
  customer_user_id: number
  customer_org_id: number
  customer_org_name: string
  display_name: string
  email: string
  phone: string
}

export interface TicketCustomerInvitationDraftPayload {
  title: string
  description: string
  priority: TicketPriority
  displayName: string
  email: string
  customerOrg: string
  locale?: string
  timezone?: string
}

export interface TicketCustomerInvitationDraftResult {
  ticket: TicketListItem
  invitation: EnterpriseCustomerInviteResult
}

export interface TicketSupplierCollaboration {
  id: number
  ticket_id: number
  conversation_id?: number
  ticket_no: string
	ticket_title: string
	ticket_status: string
	ticket_updated_at?: string
  product_id: number
  product_module_id: number
  product_module_name: string
  partner_company_id: number
  partner_company_name: string
  partner_account_id: number
	partner_account_name: string
	participant_count: number
  status: "invited" | "processing" | "resolved" | string
  reason: string
  resolution: string
  visibility: string[]
  fault_code?: string
  symptom_summary?: string
  diagnosis_summary?: string
  invited_at: string
  resolved_at?: string
	authorization_ends_at?: string
	authorization_active: boolean
}

export interface TicketSupplierProgress {
  id: number
  event_type: string
  content: string
  author_id: number
  author_name: string
  created_at: string
}

export interface TicketSupplierCollaborationDetail {
  collaboration: TicketSupplierCollaboration
  progresses: TicketSupplierProgress[]
}

export interface TicketAutoClosePolicy {
  enabled: boolean
  days: number
}

export async function fetchTickets(query?: TicketListQuery): Promise<ApiResponse<TicketListResponse>> {
  const params: Record<string, unknown> = {
    ...listQueryToParams(query || {}),
  }

  // Build composite filter string
  const filters: Record<string, string | string[] | undefined> = {}
  if (query?.status) filters["status"] = query.status
  if (query?.priority) filters["priority"] = query.priority
  if (query?.product_name) filters["product_name"] = query.product_name
  if (query?.product_id) params["product_id"] = query.product_id
  if (query?.productId) params["productId"] = query.productId
  if (query?.device_id) params["device_id"] = query.device_id
  if (query?.deviceId) params["deviceId"] = query.deviceId
  if (query?.team_id) params["team_id"] = query.team_id
  if (query?.teamId) params["teamId"] = query.teamId
  if (query?.assignee_id) params["assignee_id"] = query.assignee_id
  if (query?.mine !== undefined) params["mine"] = query.mine
  if (query?.conversation_id) params["conversation_id"] = query.conversation_id
  if (query?.conversationId) params["conversationId"] = query.conversationId
  if (query?.sla_breached !== undefined) params["sla_breached"] = query.sla_breached
  if (query?.sla_risk !== undefined) params["sla_risk"] = query.sla_risk

  // Search as filter: title:~keyword
  if (query?.search) {
    const existingFilter = buildFilterQuery(filters)
    params["filter"] = existingFilter
      ? `${existingFilter},title:~${query.search}`
      : `title:~${query.search}`
  } else {
    const existingFilter = buildFilterQuery(filters)
    if (existingFilter) params["filter"] = existingFilter
  }

  return apiGet<TicketListResponse>("/tickets", params)
}

export async function fetchTicketSummary(query?: Pick<TicketListQuery, "product_id" | "productId" | "device_id" | "deviceId" | "team_id" | "teamId" | "search" | "mine">): Promise<ApiResponse<TicketSummaryResponse>> {
  const params: Record<string, unknown> = {}
  if (query?.product_id) params["product_id"] = query.product_id
  if (query?.productId) params["productId"] = query.productId
  if (query?.device_id) params["device_id"] = query.device_id
  if (query?.deviceId) params["deviceId"] = query.deviceId
  if (query?.team_id) params["team_id"] = query.team_id
  if (query?.teamId) params["teamId"] = query.teamId
  if (query?.search) params["search"] = query.search
  if (query?.mine !== undefined) params["mine"] = query.mine
  return apiGet<TicketSummaryResponse>("/tickets/summary", params)
}

export async function fetchTicketAggregate(id: number): Promise<ApiResponse<TicketAggregateDTO>> {
  return apiGet<TicketAggregateDTO>(`/tickets/${id}`)
}

export async function createTicket(payload: CreateTicketPayload): Promise<ApiResponse<TicketListItem>> {
  return apiPost<TicketListItem>("/tickets", payload)
}

export async function fetchTicketCustomerOptions(search?: string): Promise<ApiResponse<TicketCustomerOption[]>> {
  return apiGet<TicketCustomerOption[]>("/tickets/customer-options", {
    search: search?.trim() || undefined,
    limit: 100,
  })
}

export async function createTicketCustomerInvitationDraft(
  payload: TicketCustomerInvitationDraftPayload,
): Promise<ApiResponse<TicketCustomerInvitationDraftResult>> {
  return apiPost<TicketCustomerInvitationDraftResult>("/tickets/customer-invitation-drafts", payload)
}

export async function updateTicket(id: number, payload: Record<string, unknown>): Promise<ApiResponse<TicketListItem>> {
  return apiPatch<TicketListItem>(`/tickets/${id}`, payload)
}

export async function assignTicket(id: number, assigneeId: number, note?: string): Promise<ApiResponse<TicketListItem>> {
  return apiPost<TicketListItem>(`/tickets/${id}/_assign`, { assignee_id: assigneeId, note })
}

export async function transferTicket(id: number, toUserId: number, reason?: string): Promise<ApiResponse<TicketListItem>> {
  return apiPost<TicketListItem>(`/tickets/${id}/_transfer`, { to_user_id: toUserId, reason })
}

export async function closeTicket(id: number, resolution?: string): Promise<ApiResponse<TicketListItem>> {
  return apiPost<TicketListItem>(`/tickets/${id}/_close`, { resolution })
}

export async function cancelTicket(id: number, reason: string): Promise<ApiResponse<TicketListItem>> {
  return apiPost<TicketListItem>(`/tickets/${id}/_cancel`, { reason })
}

export async function acceptTicket(id: number): Promise<ApiResponse<TicketListItem>> {
  return apiPost<TicketListItem>(`/tickets/${id}/_accept`)
}

export async function takeoverTicket(id: number): Promise<ApiResponse<TicketAggregateDTO>> {
  return apiPost<TicketAggregateDTO>(`/tickets/${id}/_takeover`)
}

export async function reopenTicket(id: number, reason?: string): Promise<ApiResponse<TicketListItem>> {
  return apiPost<TicketListItem>(`/tickets/${id}/_reopen`, { reason })
}

/** Fetch action permissions for the current user on a ticket (returns the actions DTO directly) */
export async function fetchTicketActions(id: number): Promise<ApiResponse<TicketActionPermissionsDTO>> {
  return apiGet<TicketActionPermissionsDTO>(`/tickets/${id}/actions`)
}

export async function createTicketProgress(
  id: number,
  content: string,
  visibleToCustomer = false,
): Promise<ApiResponse<TicketAggregateDTO>> {
  return apiPost<TicketAggregateDTO>(`/tickets/${id}/progress`, {
    content,
    visible_to_customer: visibleToCustomer,
  })
}

export async function createTicketRepair(
  id: number,
  payload: {
		fault_code?: string
    conclusion: string
    solution?: string
    root_cause?: string
    repair_method?: string
    service_method?: string
    test_result?: string
    warranty_covered?: boolean
    remote_resolved?: boolean
    visible_to_customer?: boolean
    parts?: Array<{ name: string; quantity: number }>
    cost_hours?: number
		mark_resolved?: boolean
  },
): Promise<ApiResponse<TicketAggregateDTO>> {
  return apiPost<TicketAggregateDTO>(`/tickets/${id}/repair`, payload)
}

export async function fetchTicketKnowledgeCandidates(
  id: number,
): Promise<ApiResponse<TicketKnowledgeCandidateDTO[]>> {
  return apiGet<TicketKnowledgeCandidateDTO[]>(`/tickets/${id}/knowledge-candidates`)
}

export async function createTicketKnowledgeCandidate(
  id: number,
  payload: CreateTicketKnowledgeCandidatePayload = {},
): Promise<ApiResponse<TicketKnowledgeCandidateDTO>> {
  return apiPost<TicketKnowledgeCandidateDTO>(`/tickets/${id}/knowledge-candidates`, payload)
}

export async function fetchTicketSupplierCollaborations(
  ticketId: number,
): Promise<ApiResponse<TicketSupplierCollaboration[]>> {
  return apiGet<TicketSupplierCollaboration[]>(`/tickets/${ticketId}/supplier-collaborations`)
}

export interface TicketSupplierOption {
  partner_company_id: number
  name: string
}

export async function fetchTicketSupplierOptions(
  ticketId: number,
): Promise<ApiResponse<TicketSupplierOption[]>> {
  return apiGet<TicketSupplierOption[]>(`/tickets/${ticketId}/supplier-options`)
}

export async function inviteTicketSupplier(
  ticketId: number,
  payload: {
    product_module_id?: number
    partner_company_id?: number
    partner_account_id?: number
    reason: string
    access_days?: number
    authorization_ends_at?: string
  },
): Promise<ApiResponse<TicketSupplierCollaboration>> {
  return apiPost<TicketSupplierCollaboration>(`/tickets/${ticketId}/supplier-collaborations`, payload)
}

export async function fetchTicketSupplierCollaborationDetail(
  ticketId: number,
  collaborationId: number,
): Promise<ApiResponse<TicketSupplierCollaborationDetail>> {
  return apiGet<TicketSupplierCollaborationDetail>(
    `/tickets/${ticketId}/supplier-collaborations/${collaborationId}`,
  )
}

export async function addTicketSupplierCollaborationProgress(
  ticketId: number,
  collaborationId: number,
  content: string,
): Promise<ApiResponse<TicketSupplierCollaborationDetail>> {
  return apiPost<TicketSupplierCollaborationDetail>(
    `/tickets/${ticketId}/supplier-collaborations/${collaborationId}/progress`,
    { content },
  )
}

export async function resolveTicketSupplierCollaboration(
  ticketId: number,
  collaborationId: number,
  resolution: string,
): Promise<ApiResponse<TicketSupplierCollaboration>> {
  return apiPost<TicketSupplierCollaboration>(
    `/tickets/${ticketId}/supplier-collaborations/${collaborationId}/_resolve`,
    { resolution },
  )
}

export async function fetchTicketAutoClosePolicy(): Promise<ApiResponse<TicketAutoClosePolicy>> {
  return apiGet<TicketAutoClosePolicy>("/ticket-settings/auto-close")
}

export async function updateTicketAutoClosePolicy(
  policy: TicketAutoClosePolicy,
): Promise<ApiResponse<TicketAutoClosePolicy>> {
  return apiPut<TicketAutoClosePolicy>("/ticket-settings/auto-close", policy)
}

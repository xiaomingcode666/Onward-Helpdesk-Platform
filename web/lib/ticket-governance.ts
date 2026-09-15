import { translateCurrentMessage } from "@/i18n/messages"
import { apiGet, apiPost } from "@/lib/api/client"

export const caseTypes = ["user_case", "incident", "major_incident", "problem", "known_error", "service_request"] as const
export type CaseType = typeof caseTypes[number]
export function governanceLabel(key: string) { return translateCurrentMessage(`ticketGovernance.${key}`) }
export type PriorityFacts = { impact: string; urgency: string; safety: string; reach: string; workaround: string; evidence: string; root_cause: string }
export const emptyPriorityFacts: PriorityFacts = { impact: "unknown", urgency: "unknown", safety: "unknown", reach: "unknown", workaround: "unknown", evidence: "", root_cause: "" }
export interface PriorityPolicy { version: string; defaults: Record<CaseType, string>; matrix: string[][] }
export const fetchClassificationPolicy = () => apiGet<PriorityPolicy>("/ticket-settings/classification")
export interface TicketRelation {
  id: number; kind: string; source_id: number; target_id: number; other_id: number; ticket_no: string; title: string; case_type: string; status: string; priority: string; owner_id: number; sla_breached: boolean; reason: string; can_remove: boolean
}
export interface PriorityProposal { id: number; revision: number; priority: string; category: string; reason: string; status: string; review_reason: string; proposed_by: number }
export interface DuplicateCandidate extends TicketRelation { revision: number; match: string; created_at: string }
export interface MergeContent {
  ticket_id: number; ticket_no: string; title: string; description: string; author: string; created_at: string
  timeline: Array<{ id: number; type: string; content: string; actor: string; timestamp: string }>
  assets: Array<{ id: number; file_name: string; uploaded_by: string; uploaded_at: string }>
}
export const searchDuplicateTickets = (id: number, search: string) => apiGet<DuplicateCandidate[]>(`/tickets/${id}/governance`, { duplicate_candidates: "1", search })
export interface GovernanceView {
  can_associate_duplicate?: boolean
  merge?: { can_merge: boolean; into?: TicketRelation; sources: MergeContent[] }
  ticket_id: number; revision: number; case_type: string; priority: string; suggested: string; explanation: string; review_required: boolean; overridden: boolean; legacy: boolean; facts: PriorityFacts; policy: PriorityPolicy; can_manage: boolean; can_propose: boolean
  proposals: PriorityProposal[]; relations: TicketRelation[]
  history: Array<{ id: number; action: string; reason: string; actor_id: number; created_at: string; details: Record<string, unknown> }>
}
export interface GovernanceCommand {
  target_revision?: number
  action: string; operation_key: string; expected_revision: number; reason: string
  case_type?: string; facts?: PriorityFacts; priority?: string; category?: string; proposal_id?: number; use_latest_rules?: boolean; relation_kind?: string; target_id?: number; relation_id?: number
}
export const fetchGovernance = (id: number) => apiGet<GovernanceView>(`/tickets/${id}/governance`)
export interface PriorityPreview { priority: string; previous_accept_deadline?: string | null; accept_deadline?: string | null; previous_deadline: string | null; deadline: string | null; overdue: boolean }
export const previewPriorityChange = (id: number, priority: string) => apiGet<PriorityPreview>(`/tickets/${id}/governance`, { preview_priority: priority })
export const saveGovernance = (id: number, command: GovernanceCommand) => apiPost<{ ticket_id: number; revision: number }>(`/tickets/${id}/governance`, command)
export const searchRelationTickets = (id: number, search: string) => apiGet<TicketRelation[]>(`/tickets/${id}/governance`, { candidates: "1", search })

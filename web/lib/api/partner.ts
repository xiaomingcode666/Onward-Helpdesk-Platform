import { partnerApiGet, partnerApiPost, partnerApiPut } from "@/lib/api/client"
import type { TicketSupplierCollaboration } from "@/lib/api/enterprise-tickets"
import type {
  ApiResponse,
  CursorPage,
  MeetingJoinConfig,
  MeetingListResponse,
  MeetingRuntimeStatus,
  MeetingTranscriptIngestInput,
  MeetingTranscriptSegment,
} from "@/lib/api/types"

export interface PartnerPortalProfile {
  account_id: number
  user_id: number
  display_name: string
  email: string
  phone: string
  company_id: number
  company_name: string
  partner_no: string
  partner_type: string
  country_region: string
  languages_json: string
  last_active_at?: string
  authorization_ok: boolean
  roles: string[]
  can_manage_team: boolean
  can_invite_team: boolean
}

export interface PartnerPortalAccount {
  id: number
  username: string
  display_name: string
  email: string
  phone: string
  languages_json: string
  status: number
  last_active_at?: string
  updated_at: string
  is_current: boolean
  roles: string[]
}

export interface PartnerAccountPayload {
  username?: string
  display_name: string
  email: string
  phone: string
  languages: string[]
  role_code: "partner_admin" | "partner_engineer"
  status?: number
}

export interface PartnerAccountCreateResult {
  account: PartnerPortalAccount
  initial_password: string
}

export interface PartnerTicketProgress {
  id: number
  event_type: string
  content: string
  author_id: number
  author_name: string
  created_at: string
}

export interface PartnerConversationMessage {
  id: number
  sender_id: number
  sender_type: "customer" | "agent" | "partner" | string
  sender_name: string
  message_type: "text" | "html" | "image" | "audio" | "attachment" | string
  content: string
  payload?: string
  sent_at: string
}

export interface PartnerMessageAsset {
  id: number
  assetId: string
  provider?: string
  storageKey?: string
  filename: string
  fileSize: number
  mimeType: string
  url?: string
}

export interface PartnerTicketParticipant {
  partner_account_id: number
  partner_account_name: string
  role: "owner" | "member" | string
  status: number
  joined_at: string
  left_at?: string
}

export interface PartnerTicketDetail {
  collaboration: TicketSupplierCollaboration
  participants: PartnerTicketParticipant[]
  progresses: PartnerTicketProgress[]
  messages: PartnerConversationMessage[]
}

export function fetchPartnerProfile(): Promise<ApiResponse<PartnerPortalProfile>> {
  return partnerApiGet<PartnerPortalProfile>("/profile")
}

export function fetchPartnerTickets(): Promise<ApiResponse<TicketSupplierCollaboration[]>> {
  return partnerApiGet<TicketSupplierCollaboration[]>("/tickets")
}

export function fetchPartnerConversations(): Promise<ApiResponse<TicketSupplierCollaboration[]>> {
  return partnerApiGet<TicketSupplierCollaboration[]>("/conversations")
}

export function fetchPartnerTicketDetail(collaborationId: number): Promise<ApiResponse<PartnerTicketDetail>> {
  return partnerApiGet<PartnerTicketDetail>(`/tickets/${collaborationId}`)
}

export function createPartnerTicketProgress(
  collaborationId: number,
  content: string,
): Promise<ApiResponse<PartnerTicketDetail>> {
  return partnerApiPost<PartnerTicketDetail>(`/tickets/${collaborationId}/progress`, { content })
}

export function createPartnerTicketMessage(
  collaborationId: number,
  payload: {
    content?: string
    message_type: "text" | "image" | "audio" | "attachment"
    asset_id?: string
    duration_seconds?: number
  },
): Promise<ApiResponse<PartnerTicketDetail>> {
  return partnerApiPost<PartnerTicketDetail>(`/tickets/${collaborationId}/progress`, payload)
}

function uploadPartnerTicketMessageAsset(collaborationId: number, kind: "image" | "audio" | "attachment", file: File) {
  const formData = new FormData()
  formData.set("file", file)
  return partnerApiPost<PartnerMessageAsset>(`/tickets/${collaborationId}/upload_${kind}`, formData)
}

export function uploadPartnerTicketImage(collaborationId: number, file: File) {
  return uploadPartnerTicketMessageAsset(collaborationId, "image", file)
}

export function uploadPartnerTicketAudio(collaborationId: number, file: File) {
  return uploadPartnerTicketMessageAsset(collaborationId, "audio", file)
}

export function uploadPartnerTicketAttachment(collaborationId: number, file: File) {
  return uploadPartnerTicketMessageAsset(collaborationId, "attachment", file)
}

export function acceptPartnerTicket(collaborationId: number): Promise<ApiResponse<TicketSupplierCollaboration>> {
  return partnerApiPost<TicketSupplierCollaboration>(`/tickets/${collaborationId}/_accept`)
}

export function assignPartnerTicket(collaborationId: number, partnerAccountId: number): Promise<ApiResponse<TicketSupplierCollaboration>> {
  return partnerApiPost<TicketSupplierCollaboration>(`/tickets/${collaborationId}/_assign`, {
    partner_account_id: partnerAccountId,
  })
}

export function addPartnerTicketParticipant(collaborationId: number, partnerAccountId: number): Promise<ApiResponse<PartnerTicketDetail>> {
  return partnerApiPost<PartnerTicketDetail>(`/tickets/${collaborationId}/participants`, {
    partner_account_id: partnerAccountId,
  })
}

export function removePartnerTicketParticipant(collaborationId: number, partnerAccountId: number): Promise<ApiResponse<PartnerTicketDetail>> {
  return partnerApiPost<PartnerTicketDetail>(`/tickets/${collaborationId}/participants/${partnerAccountId}/_remove`)
}

export function resolvePartnerTicket(
  collaborationId: number,
  resolution: string,
): Promise<ApiResponse<TicketSupplierCollaboration>> {
  return partnerApiPost<TicketSupplierCollaboration>(`/tickets/${collaborationId}/_resolve`, { resolution })
}

export function fetchPartnerMeetings(status?: string): Promise<ApiResponse<MeetingListResponse>> {
  return partnerApiGet<MeetingListResponse>("/meetings", status ? { status } : undefined)
}

export function fetchPartnerMeetingJoinConfig(
  collaborationId: number,
  meetingId: string | number,
): Promise<ApiResponse<MeetingJoinConfig>> {
  return partnerApiGet<MeetingJoinConfig>(`/supplier-collaborations/${collaborationId}/meetings/${meetingId}/join`)
}

export function confirmPartnerMeetingJoined(collaborationId: number, meetingId: string | number): Promise<ApiResponse<void>> {
  return partnerApiPost<void>(`/supplier-collaborations/${collaborationId}/meetings/${meetingId}/_joined`)
}

export function confirmPartnerMeetingLeft(collaborationId: number, meetingId: string | number): Promise<ApiResponse<void>> {
  return partnerApiPost<void>(`/supplier-collaborations/${collaborationId}/meetings/${meetingId}/_left`)
}

export function heartbeatPartnerMeeting(collaborationId: number, meetingId: string | number): Promise<ApiResponse<void>> {
  return partnerApiPost<void>(`/supplier-collaborations/${collaborationId}/meetings/${meetingId}/_heartbeat`)
}

export function fetchPartnerMeetingStatus(
  collaborationId: number,
  meetingId: string | number,
): Promise<ApiResponse<MeetingRuntimeStatus>> {
  return partnerApiGet<MeetingRuntimeStatus>(`/supplier-collaborations/${collaborationId}/meetings/${meetingId}/status`)
}

export function fetchPartnerMeetingTranscripts(
  collaborationId: number,
  meetingId: string | number,
): Promise<ApiResponse<MeetingTranscriptSegment[]>> {
  return partnerApiGet<MeetingTranscriptSegment[]>(`/supplier-collaborations/${collaborationId}/meetings/${meetingId}/transcripts`)
}

export function fetchPartnerMeetingTranscriptPage(
  collaborationId: number,
  meetingId: string | number,
  cursor = "",
  limit = 100,
): Promise<ApiResponse<CursorPage<MeetingTranscriptSegment>>> {
  return partnerApiGet<CursorPage<MeetingTranscriptSegment>>(
    `/supplier-collaborations/${collaborationId}/meetings/${meetingId}/transcripts/page`,
    {
      ...(cursor ? { cursor } : {}),
      limit,
    },
  )
}

export function ingestPartnerMeetingTranscript(
  collaborationId: number,
  meetingId: string,
  input: MeetingTranscriptIngestInput,
): Promise<ApiResponse<MeetingTranscriptSegment>> {
  return partnerApiPost<MeetingTranscriptSegment>(
    `/supplier-collaborations/${collaborationId}/meetings/${meetingId}/transcripts`,
    input,
  )
}

export function fetchPartnerAccounts(): Promise<ApiResponse<PartnerPortalAccount[]>> {
  return partnerApiGet<PartnerPortalAccount[]>("/accounts")
}

export function createPartnerAccount(payload: PartnerAccountPayload): Promise<ApiResponse<PartnerAccountCreateResult>> {
  return partnerApiPost<PartnerAccountCreateResult>("/accounts", payload)
}

export function updatePartnerAccount(accountId: number, payload: PartnerAccountPayload): Promise<ApiResponse<PartnerPortalAccount>> {
  return partnerApiPut<PartnerPortalAccount>(`/accounts/${accountId}`, payload)
}

// ============================================================
// Enterprise / Customer Meeting API
// ============================================================
// Enterprise:
//   POST /api/enterprise/v1/tickets/{ticketId}/meetings  — create meeting from ticket
//   GET  /api/enterprise/v1/meetings/{id}/join           — get join config
//   POST /api/enterprise/v1/meetings/{id}/_end            — end meeting
//   GET  /api/enterprise/v1/tickets/{ticketId}/meetings   — list meetings for a ticket
// Customer:
//   GET  /api/customer/v1/meetings/{id}/join             — get customer join config
// ============================================================

import { apiGet, apiPost } from "@/lib/api/client"
import type {
  ApiResponse,
	CursorPage,
  CreateMeetingPayload,
  MeetingListResponse,
  MeetingJoinConfig,
  MeetingListItem,
  MeetingTranscriptSegment,
  MeetingARAnnotation,
  MeetingRuntimeStatus,
  MeetingTranscriptIngestInput,
} from "@/lib/api/types"

export async function createMeeting(payload: CreateMeetingPayload): Promise<ApiResponse<MeetingJoinConfig>> {
  return apiPost<MeetingJoinConfig>(`/tickets/${payload.ticket_id}/meetings`, {
    title: payload.title,
    scheduled_at: payload.scheduled_at ?? payload.scheduledAt,
  })
}

export async function fetchMeetingJoinConfig(meetingId: string | number): Promise<ApiResponse<MeetingJoinConfig>> {
  return apiGet<MeetingJoinConfig>(`/meetings/${meetingId}/join`)
}

export async function confirmMeetingJoined(meetingId: string | number): Promise<ApiResponse<void>> {
  return apiPost<void>(`/meetings/${meetingId}/_joined`)
}

export async function confirmMeetingLeft(meetingId: string | number): Promise<ApiResponse<void>> {
  return apiPost<void>(`/meetings/${meetingId}/_left`)
}

export async function heartbeatMeeting(meetingId: string | number): Promise<ApiResponse<void>> {
  return apiPost<void>(`/meetings/${meetingId}/_heartbeat`)
}

export async function fetchMeetingStatus(meetingId: string | number): Promise<ApiResponse<MeetingRuntimeStatus>> {
  return apiGet<MeetingRuntimeStatus>(`/meetings/${meetingId}/status`)
}

export async function endMeeting(meetingId: string | number): Promise<ApiResponse<void>> {
  return apiPost<void>(`/meetings/${meetingId}/_end`)
}

export async function fetchTicketMeetings(ticketId: number): Promise<ApiResponse<MeetingListItem[]>> {
  return apiGet<MeetingListItem[]>(`/tickets/${ticketId}/meetings`)
}

export async function fetchPersonalMeetings(
	status?: string,
	page = 1,
	pageSize = 50,
	query = "",
	deviceId?: number,
): Promise<ApiResponse<MeetingListResponse>> {
	return apiGet<MeetingListResponse>("/meetings", {
		...(status ? { status } : {}),
		...(query.trim() ? { q: query.trim() } : {}),
		...(deviceId && deviceId > 0 ? { device_id: deviceId } : {}),
		page,
		page_size: pageSize,
	})
}

export function fetchMeetingTranscripts(meetingId: string | number) {
  return apiGet<MeetingTranscriptSegment[]>(`/meetings/${meetingId}/transcripts`)
}

export function fetchMeetingTranscriptPage(meetingId: string | number, cursor = "", limit = 100) {
	return apiGet<CursorPage<MeetingTranscriptSegment>>(`/meetings/${meetingId}/transcripts/page`, {
		...(cursor ? { cursor } : {}),
		limit,
	})
}

export function ingestMeetingTranscript(meetingId: string, input: MeetingTranscriptIngestInput) {
  return apiPost<MeetingTranscriptSegment>(`/meetings/${meetingId}/transcripts`, input)
}

export function fetchMeetingAnnotations(meetingId: string) {
  return apiGet<MeetingARAnnotation[]>(`/meetings/${meetingId}/annotations`)
}

export function createMeetingAnnotation(
  meetingId: string,
  input: Omit<MeetingARAnnotation, "id" | "meetingId" | "ticketId" | "createdBy" | "createdAt">,
) {
  return apiPost<MeetingARAnnotation>(`/meetings/${meetingId}/annotations`, input)
}

export function uploadMeetingFrame(meetingId: string, frame: Blob) {
  const body = new FormData()
  body.append("frame", frame, `meeting-frame-${Date.now()}.png`)
  return apiPost<{ assetId: number; url: string }>(`/meetings/${meetingId}/frames`, body)
}

export function detectMeetingFrame(meetingId: string, frameAssetId: number) {
  return apiPost<MeetingARAnnotation[]>(`/meetings/${meetingId}/frames/${frameAssetId}/_detect`)
}

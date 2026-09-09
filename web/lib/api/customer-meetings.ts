"use client"

import { buildCustomerPortalQueryString } from "@/lib/api/customer-portal-query"
import type {
  CustomerMeetingJoinConfig,
  CustomerMeetingRuntimeStatus,
  CustomerPortalMeeting,
  CustomerPortalListQuery,
  CustomerPortalPage,
} from "@/lib/api/customer-portal-types"
import { customerPortalRequest } from "@/lib/api/customer-portal-client"
import type { CursorPage, MeetingTranscriptIngestInput, MeetingTranscriptSegment } from "@/lib/api/types"

export function fetchCustomerMeetings() {
  return customerPortalRequest<CustomerPortalMeeting[]>("/meetings")
}

export function fetchCustomerMeetingsPage(query?: CustomerPortalListQuery) {
  return customerPortalRequest<CustomerPortalPage<CustomerPortalMeeting>>(`/meetings/page${buildCustomerPortalQueryString(query)}`)
}

export function fetchCustomerMeetingJoinConfig(meetingId: string) {
  return customerPortalRequest<CustomerMeetingJoinConfig>(`/meetings/${meetingId}/join`)
}

export function confirmCustomerMeetingJoined(meetingId: string) {
  return customerPortalRequest<void>(`/meetings/${meetingId}/_joined`, { method: "POST" })
}

export function confirmCustomerMeetingLeft(meetingId: string) {
  return customerPortalRequest<void>(`/meetings/${meetingId}/_left`, { method: "POST" })
}

export function heartbeatCustomerMeeting(meetingId: string) {
  return customerPortalRequest<void>(`/meetings/${meetingId}/_heartbeat`, { method: "POST" })
}

export function fetchCustomerMeetingStatus(meetingId: string) {
  return customerPortalRequest<CustomerMeetingRuntimeStatus>(`/meetings/${meetingId}/status`)
}

export function fetchCustomerMeetingTranscripts(meetingId: string) {
  return customerPortalRequest<MeetingTranscriptSegment[]>(`/meetings/${meetingId}/transcripts`)
}

export function fetchCustomerMeetingTranscriptPage(meetingId: string, cursor = "", limit = 100) {
	const query = new URLSearchParams({ limit: String(limit) })
	if (cursor) query.set("cursor", cursor)
	return customerPortalRequest<CursorPage<MeetingTranscriptSegment>>(
		`/meetings/${meetingId}/transcripts/page?${query.toString()}`,
	)
}

export function ingestCustomerMeetingTranscript(meetingId: string, input: MeetingTranscriptIngestInput) {
  return customerPortalRequest<MeetingTranscriptSegment>(`/meetings/${meetingId}/transcripts`, {
    method: "POST",
    body: JSON.stringify(input),
  })
}

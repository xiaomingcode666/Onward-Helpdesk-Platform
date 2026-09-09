import type { MeetingParticipant } from "@/lib/api/types"

export type CustomerPortalPage<T> = {
  results: T[]
  page: {
    page: number
    limit: number
    total: number
  }
}

export type CustomerPortalListQuery = {
  page?: number
  limit?: number
  locale?: string
  keyword?: string
  filter?: string
  deviceId?: number
  conversationId?: number
  ticketId?: number
  meetingId?: string
}

export type CustomerPortalProfile = {
  id: number
  name: string
  company_name: string
  primary_email: string
  primary_mobile: string
  last_active_at: string
  bound_device_count: number
  active_conversation_count: number
  open_ticket_count: number
  upcoming_meeting_count: number
}

export type CustomerPortalProfileUpdatePayload = {
  name: string
  primary_email?: string
  primary_mobile?: string
}

export type CustomerAccountDeletion = {
  request_id: string
  status: "not_requested" | "pending" | "awaiting_verification" | "processing" | "completed" | string
  requested_at: string
  deadline_at: string
  completed_at: string
  account_access_revoked: boolean
}

export type CustomerPortalMetric = {
  key: string
  label: string
  value: string
  meta: string
  tone: string
}

export type CustomerPortalDevice = {
  id: number
  device_no: string
  serial_no: string
  product_name: string
  product_code: string
  model_name: string
  region_code: string
  status: string
  last_service_at: string
  installed_at: string
  warranty_end_at: string
  manual_count: number
  repair_history_count: number
  open_ticket_count: number
  conversation_count: number
}

export type CustomerPortalManualFile = {
  id: number
  title: string
  filename: string
  file_size: number
  mime_type: string
  url: string
  uploaded_at: string
}

export type CustomerPortalSystemIntroDoc = {
  id: number
  title: string
  filename: string
  file_size: number
  mime_type: string
  url: string
  published_at: string
}

export type CustomerPortalConversation = {
  id: number
  status: string
  service_mode: number
  human_handoff_enabled?: boolean
  ticket_creation_enabled?: boolean
  priority: number
  last_message_summary: string
  last_message_at: string
  last_active_at: string
  customer_unread_count: number
  device_id: number
  device_no: string
  product_name: string
  current_assignee_name: string
  current_ticket_id: number
  current_ticket_no: string
  current_meeting_id: string
  current_meeting_status: string
}

export type CustomerPortalTicketProgress = {
  id: number
  event_type: string
  content: string
  created_at: string
  metadata?: Record<string, string>
}

export type CustomerPortalTicket = {
  id: number
  ticket_no: string
  title: string
  status: string
  priority: string
  device_id: number
  device_no: string
  product_name: string
  assignee_name: string
  created_at: string
  updated_at: string
  current_meeting_id: string
	repair_summary: string
	progress: CustomerPortalTicketProgress[]
	feedback?: {
		id: number
		rating: number
		tags: string[]
		comment: string
		submitted_at: string
	}
	can_confirm: boolean
	can_reopen: boolean
	can_rate: boolean
}

export type CustomerPortalMeeting = {
  id: string
  ticket_id: number
  ticket_no: string
  title: string
  status: string
  device_id: number
  device_no: string
  product_name: string
  created_by: string
  scheduled_at: string
  started_at: string
  ended_at: string
  duration_seconds?: number
  participant_count: number
  participants?: MeetingParticipant[]
  transcript_count?: number
  annotation_count?: number
  join_path: string
  room_name: string
}

export type CustomerPortalHome = {
  metrics: CustomerPortalMetric[]
  active_conversation?: CustomerPortalConversation
  upcoming_meeting?: CustomerPortalMeeting
  recent_devices: CustomerPortalDevice[]
  pending_tickets: CustomerPortalTicket[]
}

export type CustomerMeetingJoinConfig = {
  domain: string
  roomName: string
  jwt: string
  jitsiUrl: string
  meetingId: string
  role?: "moderator" | "participant"
  canEnd?: boolean
  transcriptionEnabled?: boolean
  transcriptionProvider?: string
  transcriptionReady?: boolean
  transcriptionErrorCode?: string
  transcriptionError?: string
  arDetectionEnabled?: boolean
  arDetectionProvider?: string
}

export type CustomerMeetingRuntimeStatus = {
  meetingId: string
  roomName: string
  status: "waiting" | "scheduled" | "active" | "ended"
  startedAt?: string
  endedAt?: string
  createdBy: string
  createdAt: string
  participantCount: number
}

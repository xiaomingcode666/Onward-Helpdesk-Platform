// ============================================================
// Shared API types for RemoteHelpDesk enterprise/customer APIs
// ============================================================

/** Unified API response envelope */
export interface ApiResponse<T> {
  success: boolean
  data: T
  error?: ApiError | null
  pagination?: Pagination | null
  requestId?: string
  timestamp?: string
}

export interface CursorPage<T> {
  results: T[]
  cursor: string
  hasMore: boolean
}

export interface EnterpriseListResponse<T> {
  items: T[]
  total: number
  page: number
  page_size: number
  total_pages: number
  has_more: boolean
}

/** Error details inside an ApiResponse */
export interface ApiError {
  code: string
  message: string
  details?: Record<string, unknown>
  requestId?: string
}

/** Pagination info returned by list endpoints */
export interface Pagination {
  total: number
  page: number
  page_size: number
  total_pages: number
  has_more: boolean
}

/** Standard query parameters for a list endpoint */
export interface ListQuery {
  page?: number
  page_size?: number
  sort?: string
  filter?: string
}

/** Generic filter value — used to build the `filter` query string */
export type FilterValue = string | number | boolean | [string, string] // [from, to] for ranges

// ---- Enterprise Product Types ----

export interface ProductListItem {
  id: number
  code: string
  name: string
  product_line: string
  status: "active" | "inactive" | "discontinued"
  category: string
  owner_member_id?: number
  owner_user_id?: number
  owner_name?: string
  device_count: number
  ticket_count: number
  active_session_count: number
  ai_resolve_rate: number
  default_locale: string
  description: string
  created_at: string
  updated_at: string
}

export interface CreateProductPayload {
  code?: string
  name: string
  product_line?: string
  product_line_id?: number
  category?: string
  description?: string
  owner_member_id?: number
  default_locale?: string
  ai_quota_limit?: number
}

export interface UpdateProductPayload {
  code?: string
  name?: string
  product_line?: string
  product_line_id?: number
  category?: string
  description?: string
  owner_member_id?: number
  default_locale?: string
  status?: "active" | "inactive" | "discontinued"
}

// ---- Enterprise Ticket Types ----

export interface TicketIntakeDTO {
  source_record_id?: string
  project_key?: string
  ticket_type?: string
  caller_name?: string
  caller_phone?: string
  received_at?: string
  context_status?: string
  missing_context?: string[]
}

export interface TicketListItem extends TicketIntakeDTO {
  id: number
  ticket_no: string
  title: string
  priority: "critical" | "high" | "medium" | "low"
  status: TicketStatus
  source?: string
  channel?: string
  conversation_id?: number
  product_id: number
  device_id?: number
  current_team_id: number
  team_name: string
  assignee_id: number
  customer_name: string
  device_no: string
  product_name: string
  assignee_name: string | null
  created_at: string
  updated_at: string
  sla_deadline: string
  sla_breached: boolean
  dispatch_attempts: number
  dispatch_deferred_until: string
  last_dispatch_failure_reason: string
  actions?: TicketActionPermissionsDTO
}

export type TicketStatus =
  | "draft"
  | "pending"
  | "pending_acceptance"
	| "accepted"
  | "assigned"
  | "pending_dispatch"
	| "pending_assignee_accept"
  | "in_progress"
	| "processing"
	| "video_support"
	| "supplier_support"
  | "escalated"
  | "waiting_customer"
	| "resolved"
	| "pending_customer_confirm"
	| "closed"
	| "quality_review"
	| "reopened"
  | "done"
  | "cancelled"

export type TicketPriority = "critical" | "high" | "medium" | "low"

export interface TicketHeaderDTO extends TicketIntakeDTO {
  device_id?: number
  service_region?: string
  id: number
	product_id: number
	product_module_id: number
  ticket_no: string
  title: string
  description: string
  status: TicketStatus
  priority: TicketPriority
  source: string
  channel: string
  conversation_id?: number
  created_at: string
  updated_at: string
  sla_deadline: string
  category: string
}

export interface CustomerSummaryDTO {
  id: number
  name: string
  company: string
  contact: string
  email: string
}

export interface DeviceContextSnapshotDTO {
  device_no: string
  serial_no: string
  product_name: string
  product_code: string
  model_name: string
  region_code: string
  install_date: string
  warranty_end: string
  service_code: string
}

export interface ConversationSnapshotDTO {
  conversation_id: number
  summary: string
  message_count: number
  last_message_at: string
  ai_served: boolean
  handoff_reason?: string
}

export interface DiagnosisHandoffSnapshotDTO {
  diagnosis_session_id: string
  fault_category: string
  confidence: number
  recommended_actions: string[]
  triage_level: string
  handoff_reason: string
}

export interface TicketFlowStepDTO {
  name: string
  status: "done" | "current" | "pending"
  completed_at?: string
  completed_by?: string
}

export interface TicketFlowDTO {
  current_step: string
  steps: TicketFlowStepDTO[]
}

export interface TicketAssignmentDTO {
  assignee_id: number
  assignee_name: string
	team_id: number
  team_name: string
  assigned_at: string
  accepted_at: string
  accept_deadline_at: string
  dispatch_attempts: number
  dispatch_deferred_until: string
  last_dispatch_failure_reason: string
  assigned_by: string
  note: string
  can_transfer: boolean
  can_escalate: boolean
}

export interface TicketMeetingDTO {
	meeting_id: string
  title: string
  status: "waiting" | "scheduled" | "active" | "finished" | "ended"
  scheduled_at: string
  started_at: string
  duration: number
  participant_count: number
  active_participant_count?: number
  external_participant_count?: number
  external_participant_names?: string[]
}

export interface RepairPartDTO {
  name: string
  quantity: number
}

export interface TicketRepairDTO {
  repair_id: number
  device_no: string
  fault_type: string
  resolution: string
  parts: RepairPartDTO[]
  cost_hours: number
  completed_at: string
  technician_name: string
}

export interface TicketFeedbackDTO {
	rating: number
	tags: string[]
	comment: string
	submitted_at: string
}

export interface AssetRefDTO {
  id: number
  file_name: string
  file_type: string
  file_size: number
  uploaded_at: string
  uploaded_by: string
}

export interface TicketTimelineItemDTO {
  id: number
  type: string
  content: string
  actor: string
  timestamp: string
}

export interface AuditRefDTO {
  id: number
  action: string
  operator: string
  timestamp: string
  detail: string
}

export interface TicketActionPermissionsDTO {
  can_accept: boolean
  can_takeover: boolean
  can_assign: boolean
  can_transfer: boolean
  can_cancel: boolean
  can_escalate_supplier: boolean
  can_start_meeting: boolean
  can_end_meeting: boolean
  can_save_repair: boolean
  can_close: boolean
  can_reopen: boolean
  can_create_knowledge_candidate: boolean
}

export interface TicketAggregateDTO {
  ticket: TicketHeaderDTO
  customer: CustomerSummaryDTO
  device_context: DeviceContextSnapshotDTO
  conversation_snapshot?: ConversationSnapshotDTO
  diagnosis_snapshot?: DiagnosisHandoffSnapshotDTO
  flow: TicketFlowDTO
  assignment: TicketAssignmentDTO
  meeting: TicketMeetingDTO
	repair: TicketRepairDTO
	feedback?: TicketFeedbackDTO
  timeline: TicketTimelineItemDTO[]
  assets: AssetRefDTO[]
  audit_refs: AuditRefDTO[]
  actions: TicketActionPermissionsDTO
}

export interface EnterpriseWorkbenchConversation {
	id: number
	customer_id: number
	customer_name: string
	status: number
	service_mode: number
	priority: number
	current_assignee_id: number
	current_assignee_name: string
	current_team_id: number
	current_team_name: string
	channel_id: number
	product_name: string
	device_no: string
	last_message_at: string
	last_active_at: string
	last_message_summary: string
	agent_unread_count: number
	customer_unread_count: number
	customer_online?: boolean
	customer_last_seen_at?: string
}

export interface EnterpriseWorkbenchQueue {
	conversations: EnterpriseWorkbenchConversation[]
	conversations_pagination: Pagination
	tickets: TicketListItem[]
	tickets_pagination: Pagination
}

export interface ConversationMessageTranslationDTO {
  id: number
  conversation_id: number
  message_id: number
  source_language: string
  target_language: string
  translated_text: string
  model_name: string
  prompt_tokens: number
  completion_tokens: number
  cached: boolean
  created_at: string
}

export interface TicketKnowledgeCandidateDTO {
  id: number
  ticket_id: number
  source_type: string
  title: string
  suggestion: string
  root_cause_summary: string
  solution_summary: string
  knowledge_base_id: number
  knowledge_entry_id: number
  quality_score: number
  value_score: number
  candidate_score: number
  quality_flags: string[]
  deduplication_override: boolean
  recurrence_count: number
  affected_device_count: number
  requires_reassessment: boolean
  review_eligible: boolean
  status: "pending" | "linked" | "deleted"
  review_status: "pending" | "needs_enrichment" | "low_quality" | "low_value" | "duplicate" | "approved" | "rejected" | "merged" | string
  created_by: string
  created_at: string
  created?: boolean
}

export interface CreateTicketKnowledgeCandidatePayload {
  knowledge_base_id?: number
  title?: string
  suggestion?: string
  root_cause_summary?: string
  solution_summary?: string
}

export interface CreateTicketPayload extends TicketIntakeDTO {
  idempotency_key?: string
  source: string
  channel: string
  product_id?: number
  product_model_id?: number
  device_id?: number
  customer_id?: number
  conversation_id?: number
  current_assignee_id?: number
  title: string
  description: string
  priority: TicketPriority
  customer_org_id?: number
  customer_user_id?: number
  language?: string
  region_code?: string
  attachments?: { asset_id: number; description: string }[]
}

// ---- Meeting Types ----

export interface CreateMeetingPayload {
  ticket_id: number
  title?: string
  scheduled_at?: string
  scheduledAt?: string
}

export interface MeetingJoinConfig {
  meetingId?: string
  meeting_id?: string
  roomName?: string
  room_name?: string
  domain: string
  jwt: string
  jitsiUrl?: string
  jitsi_url?: string
  subject?: string
  ticketId?: number
  ticket_id?: number
  ticketNo?: string
  ticket_no?: string
  role?: "moderator" | "participant"
  canEnd?: boolean
  can_end?: boolean
  transcriptionEnabled?: boolean
  transcription_enabled?: boolean
  transcriptionProvider?: string
  transcription_provider?: string
  transcriptionReady?: boolean
  transcription_ready?: boolean
  transcriptionErrorCode?: string
  transcription_error_code?: string
  transcriptionError?: string
  transcription_error?: string
  arDetectionEnabled?: boolean
  ar_detection_enabled?: boolean
  arDetectionProvider?: string
  ar_detection_provider?: string
}

export interface MeetingRuntimeStatus {
  meetingId: string
  roomName: string
  status: "waiting" | "scheduled" | "active" | "ended"
  startedAt?: string
  endedAt?: string
  createdBy: string
  createdAt: string
  participantCount: number
}

export interface MeetingTranscriptSegment {
  id: string
  meetingId: string
  participantId: string
  speakerName: string
  provider: string
	providerEventId: string
	ingestSource?: string
  language: string
  text: string
  isFinal: boolean
  startedAtMs: number
  endedAtMs: number
  confidence: number
  translatedLanguage: string
  translatedText: string
  translationProvider: string
  translationStatus: "not_configured" | "pending" | "processing" | "completed" | "failed" | "skipped"
  translationError?: string
  createdAt: string
}

export type MeetingTranscriptIngestInput = {
	provider?: "jitsi_caption" | "jitsi_chat"
	providerEventId: string
  participantId: string
  speakerName: string
  language: string
  text: string
  isFinal: boolean
  startedAtMs: number
  endedAtMs: number
  confidence: number
}

export interface MeetingARAnnotation {
  id: string
  meetingId: string
  ticketId: string
  frameAssetId: number
  frameUrl?: string
  detectionProvider: string
  externalDetectionId: string
  label: string
  partCode: string
  confidence: number
  bounds: { x: number; y: number; width: number; height: number }
  color: string
  note: string
  createdBy: number
  metadataJson: string
  createdAt: string
}

export interface MeetingParticipant {
  id: string
  name: string
  user_type: string
  identity_type?: "platform_admin" | "tenant_admin" | "authorized_support" | "repair_engineer" | "customer" | "external" | string
  role: string
  joined_at: string
  left_at?: string
  duration_seconds: number
}

export interface MeetingListItem {
  id: string
	collaboration_id?: number
  ticket_id: number
  ticket_no: string
  title: string
  room_name: string
  status: "waiting" | "scheduled" | "active" | "finished" | "ended"
  product_name: string
  device_no: string
  customer_name: string
  created_by: string
  scheduled_at: string
  started_at: string
  ended_at?: string
  created_at: string
  duration_seconds: number
  participant_count: number
	participants?: MeetingParticipant[]
	transcript_count?: number
	annotation_count?: number
	can_end?: boolean
  join_path: string
  jitsi_url: string
}

export interface MeetingListResponse {
  summary: {
    active: number
	waiting: number
    ended: number
    mine: number
    participants_online: number
  }
  items: MeetingListItem[]
	total?: number
	page?: number
	page_size?: number
	has_more?: boolean
}

export interface EnterpriseNotification {
  id: number
  tenant_id: number
  recipient_user_id: number
  recipient_name: string
  title: string
  content: string
  notification_type: string
  biz_type: string
  biz_id: number
  category: string
  level: string
  channels: string[]
  email_status: "sent" | "pending" | "disabled" | string
  action_url: string
  delivery_status: string
  external_channel_status: string
  read_at: string
  created_at: string
  can_mark_read: boolean
  is_event_summary: boolean
  recipient_count: number
  unread_count: number
  email_pending_count: number
  email_failed_count: number
}

export interface EnterpriseNotificationSummary {
  total: number
  unread: number
  delivery_total: number
  delivery_unread: number
  markable_unread: number
  urgent: number
  email_enabled: number
  pending_email: number
  failed_email: number
  today: number
}

export interface EnterpriseNotificationListResponse {
  summary: EnterpriseNotificationSummary
  items: EnterpriseNotification[]
  total: number
  page: number
  page_size: number
  has_more: boolean
  scope: "personal" | "tenant"
  can_view_tenant_scope: boolean
}

export interface NotificationMailSetting {
  from_address: string
  from_name: string
  smtp_host: string
  smtp_port: number
  username: string
  reply_to: string
  retry_policy: string
  use_tls: boolean
  connected: boolean
  has_password: boolean
}

export interface UpdateNotificationMailSettingPayload {
  fromAddress: string
  fromName: string
  smtpHost: string
  smtpPort: number
  username?: string
  password?: string
  replyTo: string
  retryPolicy: string
  useTls: boolean
}

export interface NotificationRecipientSetting {
  profile_email: string
  override_email: string
  effective_email: string
  use_profile_email: boolean
  email_enabled: boolean
  profile_source: "enterprise_people" | string
  updated_at: string
}

export interface UpdateNotificationRecipientSettingPayload {
  email: string
  useProfileEmail: boolean
  emailEnabled: boolean
}

export interface EnterpriseDeviceHealth {
  status: string
  label: string
  count: number
  tone: string
}

export interface EnterpriseWorkbenchScope {
  mode: string
  label: string
  role_label: string
  restricted: boolean
  user_id: number
  team_id: number
  team_ids?: number[]
  team_name: string
  product_id: number
  product_ids?: number[]
  product_name: string
}

export interface EnterpriseWorkbenchSummary {
  open_tickets: number
  pending_tickets: number
  processing_tickets: number
  suspended_tickets: number
  awaiting_customer_tickets: number
  sla_risk_tickets: number
  sla_breached_tickets: number
  urgent_tickets: number
  unassigned_tickets: number
  closed_today_tickets: number
  active_conversations: number
  active_meetings: number
  unread_notifications: number
  total_products: number
  total_devices: number
}

export interface EnterpriseWorkbenchAlert {
  key: string
  title: string
  description: string
  severity: "critical" | "warning" | "info" | "success" | string
  count: number
  action_label: string
  action_url: string
}

export interface EnterpriseWorkbenchUsage {
  label: string
  account_balance: number
  quota_limit: number
  quota_used: number
  quota_remaining: number
  usage_percent: number
  currency: string
  product_count: number
  risky_product_count: number
  key_count: number
  risky_key_count: number
  critical_key_count: number
  tone: string
  meta: string
  sync_status: "current" | "stale" | "unconfigured" | string
  last_synced_at: string
}

export interface EnterpriseWorkbenchQuotaAlert {
  product_id: number
  product_name: string
  api_key_id: string
  key_name: string
  quota_limit: number
  quota_used: number
  quota_remaining: number
  usage_percent: number
  currency: string
  severity: "critical" | "warning" | string
  last_synced_at: string
}

export interface EnterpriseWorkbenchQueueCard {
  key: string
  title: string
  description: string
  count: number
  tone: string
  action_url: string
  items: TicketListItem[]
}

export interface EnterpriseWorkbenchProductLoad {
  product_id: number
  product_name: string
  team_id: number
  team_name: string
  open_tickets: number
  pending_tickets: number
  processing_tickets: number
  unassigned_tickets: number
  sla_risk_tickets: number
  quota_limit: number
  quota_used: number
  quota_remaining: number
  usage_percent: number
  currency: string
  tone: string
}

export interface EnterpriseWorkbenchCore {
  queue: TicketListItem[]
  scope: EnterpriseWorkbenchScope
  summary: EnterpriseWorkbenchSummary
  alerts: EnterpriseWorkbenchAlert[]
  queues: EnterpriseWorkbenchQueueCard[]
  generated_at: string
}

export interface EnterpriseWorkbenchCollaboration {
  meetings: MeetingListItem[]
  active_conversations: number
  active_meetings: number
}

export interface EnterpriseWorkbenchResources {
  device_health: EnterpriseDeviceHealth[]
  usage: EnterpriseWorkbenchUsage
  quota_alerts: EnterpriseWorkbenchQuotaAlert[]
  products: EnterpriseWorkbenchProductLoad[]
  total_products: number
  total_devices: number
}

// ---- Diagnosis Types ----

export interface StartDiagnosisPayload {
  ticket_id?: string | number
  device_id?: string | number
  product_id: string | number
  symptom: string
  language?: string
}

export interface StepDiagnosisPayload {
  step_id: string
  answer: string
  attachments?: { asset_id: number }[]
}

export interface StartDiagnosisResponseDTO {
  sessionId: string
  status: string
  createdAt: string
  maxRounds: number
  confidence: number
}

export interface DiagnosisStepResponseDTO {
  stepId: string
  outputContent: string
  confidenceScore: number
  sequenceNo: number
  shouldEscalate: boolean
  escalationReason: string
}

export interface DiagnosisSummaryDTO {
  session_id: string
  tenant_id: number
  customer_id: string
  device_id: string
  product_id: string
  service_code_id: string
  conversation_id: string
  status: string
  symptoms: string
  fault_codes: string
  total_rounds: number
  confidence_score: number
  retrieve_count: number
  retrieve_hit_count: number
  retrieve_top_score: number
  created_at: string
  steps: unknown[]
  fault_tree_path: unknown[]
}

// ---- Device Types ----

export interface DeviceListItem {
  id: number
  device_no: string
  serial_no: string
  product_id: number
  product_name: string
  product_code: string
  model_id: number
  model_name: string
  customer_org_id: number
  customer_user_id: number
  customer_name: string
  customer_org: string
  region_code: string
  install_date: string
  status: "active" | "inactive" | "maintenance"
  device_status: string
  warranty_end: string
  warranty_status: string
  service_code: string
  entry_url?: string
  qr_url?: string
  qr_image_url?: string
  last_service_at: string
  open_ticket_count: number
  total_ticket_count: number
  meeting_count: number
  active_meeting_count: number
  binding_count: number
  updated_at: string
}

export interface DeviceDetailDTO {
  id: number
  device_no: string
  serial_no: string
  product_id: number
  product_name: string
  product_code: string
  model_id: number
  model_name: string
  customer_org_id: number
  customer_user_id: number
  region_code: string
  status: "active" | "inactive" | "maintenance"
  device_status: string
  install_date: string
  install_location: string
  warranty_end: string
  warranty_status: string
  service_code: string
  entry_url?: string
  qr_url?: string
  qr_image_url?: string
  customer_name: string
  customer_org: string
  open_ticket_count: number
  total_ticket_count: number
  meeting_count: number
  active_meeting_count: number
  binding_count: number
  description: string
  source: string
  status_reason: string
  software_versions: SoftwareVersionDTO[]
  repair_history: RepairHistoryItemDTO[]
  recent_tickets: TicketListItem[]
  customer_bindings: CustomerBindingDTO[]
  customer_visible: boolean
  can_create_ticket: boolean
  can_start_meeting: boolean
  created_at: string
  updated_at: string
}

export interface SoftwareVersionDTO {
  id: number
  component: string
  component_type: string
  version: string
  installed_at: string
  source: string
}

export interface RepairHistoryItemDTO {
  id: number
  ticket_id: number
  ticket_no: string
  fault_type: string
  resolution: string
  repair_method: string
  technician_name: string
  completed_at: string
  visible_to_customer: boolean
}

export interface CustomerBindingDTO {
  id: number
  customer_org_id: number
  customer_user_id: number
  customer_name: string
  binding_role: string
  source: string
  status: string
  confirmed_at: string
}

export interface CreateDevicePayload {
  service_code: string
  product_id?: number
  model_id?: number
  region_code?: string
  install_date?: string
  warranty_end?: string
  description?: string
}

export interface UpdateDevicePayload {
  product_id?: number
  model_id?: number
  region_code?: string
  install_date?: string
  warranty_end?: string
  description?: string
}

export interface BatchImportDevicePayload {
  service_code: string
  product_id?: number
  model_id?: number
  region_code?: string
  description?: string
}

export interface DeviceBatchImportError {
  row: number
  reason: string
}

export interface DeviceBatchImportResult {
  total: number
  succeeded: number
  failed: number
  errors?: DeviceBatchImportError[]
  created?: DeviceListItem[]
}

export interface TransferDevicePayload {
  device_id: number
  target_customer_org_id: number
  target_customer_user_id?: number
  reason?: string
}

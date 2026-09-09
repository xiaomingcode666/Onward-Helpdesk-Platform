package dto

type EnterpriseListResponse[T any] struct {
	Items      []T   `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
	HasMore    bool  `json:"has_more"`
}

type EnterpriseTicketListItemDTO struct {
	TicketIntakeDTO
	ID                        int64                       `json:"id"`
	TicketNo                  string                      `json:"ticket_no"`
	Title                     string                      `json:"title"`
	Priority                  string                      `json:"priority"`
	Status                    string                      `json:"status"`
	Source                    string                      `json:"source"`
	Channel                   string                      `json:"channel"`
	ConversationID            int64                       `json:"conversation_id"`
	ProductID                 int64                       `json:"product_id"`
	DeviceID                  int64                       `json:"device_id"`
	CurrentTeamID             int64                       `json:"current_team_id"`
	TeamName                  string                      `json:"team_name"`
	AssigneeID                int64                       `json:"assignee_id"`
	CustomerName              string                      `json:"customer_name"`
	DeviceNo                  string                      `json:"device_no"`
	ProductName               string                      `json:"product_name"`
	AssigneeName              *string                     `json:"assignee_name"`
	CreatedAt                 string                      `json:"created_at"`
	UpdatedAt                 string                      `json:"updated_at"`
	SLADeadline               string                      `json:"sla_deadline"`
	SLABreached               bool                        `json:"sla_breached"`
	DispatchAttempts          int                         `json:"dispatch_attempts"`
	DispatchDeferredUntil     string                      `json:"dispatch_deferred_until"`
	LastDispatchFailureReason string                      `json:"last_dispatch_failure_reason"`
	Actions                   *TicketActionPermissionsDTO `json:"actions,omitempty"`
}

type EnterpriseTicketSummaryDTO struct {
	Total            int64  `json:"total"`
	Pending          int64  `json:"pending"`
	Processing       int64  `json:"processing"`
	AwaitingCustomer int64  `json:"awaiting_customer"`
	SLARisk          int64  `json:"sla_risk"`
	Urgent           int64  `json:"urgent"`
	Done             int64  `json:"done"`
	GeneratedAt      string `json:"generated_at"`
}

type EnterpriseTicketCustomerOptionDTO struct {
	CustomerID      int64  `json:"customer_id"`
	CustomerUserID  int64  `json:"customer_user_id"`
	CustomerOrgID   int64  `json:"customer_org_id"`
	CustomerOrgName string `json:"customer_org_name"`
	DisplayName     string `json:"display_name"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
}

type EnterpriseTicketInvitationDraftDTO struct {
	Ticket     EnterpriseTicketListItemDTO     `json:"ticket"`
	Invitation *EnterpriseIAMCustomerInviteDTO `json:"invitation"`
}

type TicketHeaderDTO struct {
	TicketIntakeDTO
	DeviceID        int64  `json:"device_id"`
	ServiceRegion   string `json:"service_region"`
	ID              int64  `json:"id"`
	ProductID       int64  `json:"product_id"`
	ProductModuleID int64  `json:"product_module_id"`
	TicketNo        string `json:"ticket_no"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Status          string `json:"status"`
	Priority        string `json:"priority"`
	Source          string `json:"source"`
	Channel         string `json:"channel"`
	ConversationID  int64  `json:"conversation_id"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	SLADeadline     string `json:"sla_deadline"`
	Category        string `json:"category"`
}

type CustomerSummaryDTO struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Company string `json:"company"`
	Contact string `json:"contact"`
	Email   string `json:"email"`
}

type DeviceContextSnapshotDTO struct {
	DeviceNo    string `json:"device_no"`
	SerialNo    string `json:"serial_no"`
	ProductName string `json:"product_name"`
	ProductCode string `json:"product_code"`
	ModelName   string `json:"model_name"`
	RegionCode  string `json:"region_code"`
	InstallDate string `json:"install_date"`
	WarrantyEnd string `json:"warranty_end"`
	ServiceCode string `json:"service_code"`
}

type ConversationSnapshotDTO struct {
	ConversationID int64  `json:"conversation_id"`
	Summary        string `json:"summary"`
	MessageCount   int64  `json:"message_count"`
	LastMessageAt  string `json:"last_message_at"`
	AIServed       bool   `json:"ai_served"`
	HandoffReason  string `json:"handoff_reason,omitempty"`
}

type DiagnosisHandoffSnapshotDTO struct {
	DiagnosisSessionID string   `json:"diagnosis_session_id"`
	FaultCategory      string   `json:"fault_category"`
	Confidence         float64  `json:"confidence"`
	RecommendedActions []string `json:"recommended_actions"`
	TriageLevel        string   `json:"triage_level"`
	HandoffReason      string   `json:"handoff_reason"`
}

type TicketFlowStepDTO struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	CompletedAt string `json:"completed_at,omitempty"`
	CompletedBy string `json:"completed_by,omitempty"`
}

type TicketFlowDTO struct {
	CurrentStep string              `json:"current_step"`
	Steps       []TicketFlowStepDTO `json:"steps"`
}

type TicketAssignmentDTO struct {
	AssigneeID                int64  `json:"assignee_id"`
	AssigneeName              string `json:"assignee_name"`
	TeamID                    int64  `json:"team_id"`
	TeamName                  string `json:"team_name"`
	AssignedAt                string `json:"assigned_at"`
	AcceptedAt                string `json:"accepted_at"`
	AcceptDeadlineAt          string `json:"accept_deadline_at"`
	DispatchAttempts          int    `json:"dispatch_attempts"`
	DispatchDeferredUntil     string `json:"dispatch_deferred_until"`
	LastDispatchFailureReason string `json:"last_dispatch_failure_reason"`
	AssignedBy                string `json:"assigned_by"`
	Note                      string `json:"note"`
	CanTransfer               bool   `json:"can_transfer"`
	CanEscalate               bool   `json:"can_escalate"`
}

type TicketMeetingDTO struct {
	MeetingID                string   `json:"meeting_id"`
	Title                    string   `json:"title"`
	Status                   string   `json:"status"`
	ScheduledAt              string   `json:"scheduled_at"`
	StartedAt                string   `json:"started_at"`
	Duration                 int64    `json:"duration"`
	ParticipantCount         int64    `json:"participant_count"`
	ActiveParticipantCount   int64    `json:"active_participant_count"`
	ExternalParticipantCount int64    `json:"external_participant_count"`
	ExternalParticipantNames []string `json:"external_participant_names"`
}

type RepairPartDTO struct {
	Name     string `json:"name"`
	Quantity int64  `json:"quantity"`
}

type TicketRepairDTO struct {
	RepairID       int64           `json:"repair_id"`
	DeviceNo       string          `json:"device_no"`
	FaultType      string          `json:"fault_type"`
	Resolution     string          `json:"resolution"`
	Parts          []RepairPartDTO `json:"parts"`
	CostHours      float64         `json:"cost_hours"`
	CompletedAt    string          `json:"completed_at"`
	TechnicianName string          `json:"technician_name"`
}

type TicketFeedbackDTO struct {
	Rating      int      `json:"rating"`
	Tags        []string `json:"tags"`
	Comment     string   `json:"comment"`
	SubmittedAt string   `json:"submitted_at"`
}

type AssetRefDTO struct {
	ID         int64  `json:"id"`
	FileName   string `json:"file_name"`
	FileType   string `json:"file_type"`
	FileSize   int64  `json:"file_size"`
	UploadedAt string `json:"uploaded_at"`
	UploadedBy string `json:"uploaded_by"`
}

type TicketTimelineItemDTO struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Content   string `json:"content"`
	Actor     string `json:"actor"`
	Timestamp string `json:"timestamp"`
}

type AuditRefDTO struct {
	ID        int64  `json:"id"`
	Action    string `json:"action"`
	Operator  string `json:"operator"`
	Timestamp string `json:"timestamp"`
	Detail    string `json:"detail"`
}

type TicketActionPermissionsDTO struct {
	CanAccept                   bool `json:"can_accept"`
	CanTakeover                 bool `json:"can_takeover"`
	CanAssign                   bool `json:"can_assign"`
	CanTransfer                 bool `json:"can_transfer"`
	CanCancel                   bool `json:"can_cancel"`
	CanEscalateSupplier         bool `json:"can_escalate_supplier"`
	CanStartMeeting             bool `json:"can_start_meeting"`
	CanEndMeeting               bool `json:"can_end_meeting"`
	CanSaveRepair               bool `json:"can_save_repair"`
	CanClose                    bool `json:"can_close"`
	CanReopen                   bool `json:"can_reopen"`
	CanCreateKnowledgeCandidate bool `json:"can_create_knowledge_candidate"`
}

type TicketAggregateDTO struct {
	Ticket               TicketHeaderDTO              `json:"ticket"`
	Customer             CustomerSummaryDTO           `json:"customer"`
	DeviceContext        DeviceContextSnapshotDTO     `json:"device_context"`
	ConversationSnapshot *ConversationSnapshotDTO     `json:"conversation_snapshot,omitempty"`
	DiagnosisSnapshot    *DiagnosisHandoffSnapshotDTO `json:"diagnosis_snapshot,omitempty"`
	Flow                 TicketFlowDTO                `json:"flow"`
	Assignment           TicketAssignmentDTO          `json:"assignment"`
	Meeting              TicketMeetingDTO             `json:"meeting"`
	Repair               TicketRepairDTO              `json:"repair"`
	Feedback             *TicketFeedbackDTO           `json:"feedback,omitempty"`
	Timeline             []TicketTimelineItemDTO      `json:"timeline"`
	Assets               []AssetRefDTO                `json:"assets"`
	AuditRefs            []AuditRefDTO                `json:"audit_refs"`
	Actions              TicketActionPermissionsDTO   `json:"actions"`
}

package dto

type CustomerPortalProfileDTO struct {
	ID                      int64  `json:"id"`
	Name                    string `json:"name"`
	CompanyName             string `json:"company_name"`
	PrimaryEmail            string `json:"primary_email"`
	PrimaryMobile           string `json:"primary_mobile"`
	LastActiveAt            string `json:"last_active_at"`
	BoundDeviceCount        int    `json:"bound_device_count"`
	ActiveConversationCount int    `json:"active_conversation_count"`
	OpenTicketCount         int    `json:"open_ticket_count"`
	UpcomingMeetingCount    int    `json:"upcoming_meeting_count"`
}

type CustomerAccountDeletionDTO struct {
	RequestID            string `json:"request_id"`
	Status               string `json:"status"`
	RequestedAt          string `json:"requested_at"`
	DeadlineAt           string `json:"deadline_at"`
	CompletedAt          string `json:"completed_at"`
	AccountAccessRevoked bool   `json:"account_access_revoked"`
}

type CustomerPortalPresenceDTO struct {
	Online     bool   `json:"online"`
	LastSeenAt string `json:"last_seen_at"`
}

type CustomerPortalMetricDTO struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
	Meta  string `json:"meta"`
	Tone  string `json:"tone"`
}

type CustomerPortalHomeDTO struct {
	Metrics            []CustomerPortalMetricDTO      `json:"metrics"`
	ActiveConversation *CustomerPortalConversationDTO `json:"active_conversation,omitempty"`
	UpcomingMeeting    *CustomerPortalMeetingDTO      `json:"upcoming_meeting,omitempty"`
	RecentDevices      []CustomerPortalDeviceDTO      `json:"recent_devices"`
	PendingTickets     []CustomerPortalTicketDTO      `json:"pending_tickets"`
}

type CustomerPortalDeviceDTO struct {
	ID                 int64  `json:"id"`
	DeviceNo           string `json:"device_no"`
	SerialNo           string `json:"serial_no"`
	ProductName        string `json:"product_name"`
	ProductCode        string `json:"product_code"`
	ModelName          string `json:"model_name"`
	RegionCode         string `json:"region_code"`
	Status             string `json:"status"`
	LastServiceAt      string `json:"last_service_at"`
	InstalledAt        string `json:"installed_at"`
	WarrantyEndAt      string `json:"warranty_end_at"`
	ManualCount        int    `json:"manual_count"`
	RepairHistoryCount int    `json:"repair_history_count"`
	OpenTicketCount    int    `json:"open_ticket_count"`
	ConversationCount  int    `json:"conversation_count"`
}

type CustomerPortalDeviceAccessDTO struct {
	DeviceID   int64 `json:"device_id"`
	Accessible bool  `json:"accessible"`
}

type CustomerPortalManualFileDTO struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Filename   string `json:"filename"`
	FileSize   int64  `json:"file_size"`
	MimeType   string `json:"mime_type"`
	URL        string `json:"url"`
	UploadedAt string `json:"uploaded_at"`
}

// CustomerPortalSystemIntroDocDTO 客户门户访客可见的系统介绍文档（仅已上架）。
type CustomerPortalSystemIntroDocDTO struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Filename    string `json:"filename"`
	FileSize    int64  `json:"file_size"`
	MimeType    string `json:"mime_type"`
	URL         string `json:"url"`
	PublishedAt string `json:"published_at"`
}

type CustomerPortalConversationDTO struct {
	ID                    int64  `json:"id"`
	Status                string `json:"status"`
	ServiceMode           int    `json:"service_mode"`
	HumanHandoffEnabled   bool   `json:"human_handoff_enabled"`
	TicketCreationEnabled bool   `json:"ticket_creation_enabled"`
	Priority              int    `json:"priority"`
	LastMessageSummary    string `json:"last_message_summary"`
	LastMessageAt         string `json:"last_message_at"`
	LastActiveAt          string `json:"last_active_at"`
	CustomerUnreadCount   int    `json:"customer_unread_count"`
	DeviceID              int64  `json:"device_id"`
	DeviceNo              string `json:"device_no"`
	ProductName           string `json:"product_name"`
	CurrentAssigneeID     int64  `json:"-"`
	CurrentAssigneeName   string `json:"current_assignee_name"`
	CurrentTicketID       int64  `json:"current_ticket_id"`
	CurrentTicketNo       string `json:"current_ticket_no"`
	CurrentMeetingID      string `json:"current_meeting_id"`
	CurrentMeetingStatus  string `json:"current_meeting_status"`
}

type CustomerPortalTicketProgressDTO struct {
	ID        int64             `json:"id"`
	EventType string            `json:"event_type"`
	Content   string            `json:"content"`
	CreatedAt string            `json:"created_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type CustomerPortalTicketFeedbackDTO struct {
	ID          int64    `json:"id"`
	Rating      int      `json:"rating"`
	Tags        []string `json:"tags"`
	Comment     string   `json:"comment"`
	SubmittedAt string   `json:"submitted_at"`
}

type CustomerPortalTicketDTO struct {
	ID               int64                             `json:"id"`
	TicketNo         string                            `json:"ticket_no"`
	Title            string                            `json:"title"`
	Status           string                            `json:"status"`
	Priority         string                            `json:"priority"`
	DeviceID         int64                             `json:"device_id"`
	DeviceNo         string                            `json:"device_no"`
	ProductName      string                            `json:"product_name"`
	AssigneeName     string                            `json:"assignee_name"`
	CreatedAt        string                            `json:"created_at"`
	UpdatedAt        string                            `json:"updated_at"`
	CurrentMeetingID string                            `json:"current_meeting_id"`
	RepairSummary    string                            `json:"repair_summary"`
	Progress         []CustomerPortalTicketProgressDTO `json:"progress"`
	Feedback         *CustomerPortalTicketFeedbackDTO  `json:"feedback,omitempty"`
	CanConfirm       bool                              `json:"can_confirm"`
	CanReopen        bool                              `json:"can_reopen"`
	CanRate          bool                              `json:"can_rate"`
}

type CustomerPortalMeetingDTO struct {
	ID               string                  `json:"id"`
	TicketID         int64                   `json:"ticket_id"`
	TicketNo         string                  `json:"ticket_no"`
	Title            string                  `json:"title"`
	Status           string                  `json:"status"`
	DeviceID         int64                   `json:"device_id"`
	DeviceNo         string                  `json:"device_no"`
	ProductName      string                  `json:"product_name"`
	CreatedBy        string                  `json:"created_by"`
	ScheduledAt      string                  `json:"scheduled_at"`
	StartedAt        string                  `json:"started_at"`
	EndedAt          string                  `json:"ended_at"`
	DurationSeconds  int64                   `json:"duration_seconds"`
	ParticipantCount int64                   `json:"participant_count"`
	Participants     []MeetingParticipantDTO `json:"participants"`
	TranscriptCount  int64                   `json:"transcript_count"`
	AnnotationCount  int64                   `json:"annotation_count"`
	JoinPath         string                  `json:"join_path"`
	RoomName         string                  `json:"room_name"`
}

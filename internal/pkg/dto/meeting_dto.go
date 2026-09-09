package dto

// MeetingParticipantDTO is the customer-safe attendance record shared by
// enterprise and customer meeting views.
type MeetingParticipantDTO struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	UserType        string `json:"user_type"`
	IdentityType    string `json:"identity_type"`
	Role            string `json:"role"`
	JoinedAt        string `json:"joined_at"`
	LeftAt          string `json:"left_at,omitempty"`
	DurationSeconds int64  `json:"duration_seconds"`
}

// CreateMeetingRequest 创建会议请求 DTO
type CreateMeetingRequest struct {
	TicketID string `json:"ticketId" validate:"required"`
	TenantID string `json:"tenantId,omitempty"`
}

// JoinConfigResponse 入会配置响应 DTO
type JoinConfigResponse struct {
	Domain              string `json:"domain"`
	RoomName            string `json:"roomName"`
	JWT                 string `json:"jwt"`
	JitsiURL            string `json:"jitsiUrl"`
	MeetingID           string `json:"meetingId"`
	TicketID            int64  `json:"ticketId,omitempty"`
	TicketNo            string `json:"ticketNo,omitempty"`
	Subject             string `json:"subject,omitempty"`
	ARDetectionEnabled  bool   `json:"arDetectionEnabled"`
	ARDetectionProvider string `json:"arDetectionProvider"`
}

// MeetingEventDTO Jitsi Webhook 事件 DTO
type MeetingEventDTO struct {
	EventID         string `json:"event_id"`
	Action          string `json:"action"` // meeting.ended / participant.joined / participant.left
	RoomName        string `json:"room,omitempty"`
	MeetingID       string `json:"meeting_id,omitempty"`
	ParticipantID   string `json:"participant_id,omitempty"`
	ParticipantName string `json:"participant_name,omitempty"`
	DurationMs      int64  `json:"duration_ms,omitempty"`
	Timestamp       int64  `json:"timestamp,omitempty"`
}

// MeetingStatusResponse 会议状态响应 DTO
type MeetingStatusResponse struct {
	MeetingID   string `json:"meetingId"`
	RoomName    string `json:"roomName"`
	Status      string `json:"status"`
	TicketID    string `json:"ticketId,omitempty"`
	TenantID    string `json:"tenantId,omitempty"`
	PartCount   int64  `json:"participantCount"`
	CreatedAt   string `json:"createdAt"`
	ScheduledAt string `json:"scheduledAt,omitempty"`
	StartedAt   string `json:"startedAt,omitempty"`
	EndedAt     string `json:"endedAt,omitempty"`
}

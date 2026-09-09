package dto

type EnterpriseMetricDTO struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Value    string `json:"value"`
	Meta     string `json:"meta"`
	Tone     string `json:"tone"`
	Trend    string `json:"trend,omitempty"`
	Progress int    `json:"progress,omitempty"`
}

type EnterpriseWorkbenchOverviewDTO struct {
	Metrics         []EnterpriseMetricDTO               `json:"metrics"`
	Queue           []EnterpriseTicketListItemDTO       `json:"queue"`
	Meetings        []EnterpriseMeetingListItemDTO      `json:"meetings"`
	Notifications   []EnterpriseNotificationDTO         `json:"notifications"`
	DeviceHealth    []EnterpriseDeviceHealthDTO         `json:"device_health"`
	Activity        []EnterpriseActivityDTO             `json:"activity"`
	Scope           EnterpriseWorkbenchScopeDTO         `json:"scope"`
	Summary         EnterpriseWorkbenchSummaryDTO       `json:"summary"`
	Alerts          []EnterpriseWorkbenchAlertDTO       `json:"alerts"`
	Usage           EnterpriseWorkbenchUsageDTO         `json:"usage"`
	QuotaAlerts     []EnterpriseWorkbenchQuotaAlertDTO  `json:"quota_alerts"`
	Queues          []EnterpriseWorkbenchQueueCardDTO   `json:"queues"`
	Products        []EnterpriseWorkbenchProductLoadDTO `json:"products"`
	GeneratedAt     string                              `json:"generated_at"`
	DegradedModules []string                            `json:"degraded_modules,omitempty"`
}

type EnterpriseWorkbenchCoreDTO struct {
	Queue       []EnterpriseTicketListItemDTO     `json:"queue"`
	Scope       EnterpriseWorkbenchScopeDTO       `json:"scope"`
	Summary     EnterpriseWorkbenchSummaryDTO     `json:"summary"`
	Alerts      []EnterpriseWorkbenchAlertDTO     `json:"alerts"`
	Queues      []EnterpriseWorkbenchQueueCardDTO `json:"queues"`
	GeneratedAt string                            `json:"generated_at"`
}

type EnterpriseWorkbenchCollaborationDTO struct {
	Meetings            []EnterpriseMeetingListItemDTO `json:"meetings"`
	ActiveConversations int64                          `json:"active_conversations"`
	ActiveMeetings      int64                          `json:"active_meetings"`
}

type EnterpriseWorkbenchNotificationsDTO struct {
	Notifications       []EnterpriseNotificationDTO `json:"notifications"`
	UnreadNotifications int64                       `json:"unread_notifications"`
}

type EnterpriseWorkbenchResourcesDTO struct {
	DeviceHealth  []EnterpriseDeviceHealthDTO         `json:"device_health"`
	Usage         EnterpriseWorkbenchUsageDTO         `json:"usage"`
	QuotaAlerts   []EnterpriseWorkbenchQuotaAlertDTO  `json:"quota_alerts"`
	Products      []EnterpriseWorkbenchProductLoadDTO `json:"products"`
	TotalProducts int64                               `json:"total_products"`
	TotalDevices  int64                               `json:"total_devices"`
}

type EnterpriseWorkbenchScopeDTO struct {
	Mode        string  `json:"mode"`
	Label       string  `json:"label"`
	RoleLabel   string  `json:"role_label"`
	Restricted  bool    `json:"restricted"`
	UserID      int64   `json:"user_id"`
	TeamID      int64   `json:"team_id"`
	TeamIDs     []int64 `json:"team_ids"`
	TeamName    string  `json:"team_name"`
	ProductID   int64   `json:"product_id"`
	ProductIDs  []int64 `json:"product_ids"`
	ProductName string  `json:"product_name"`
}

type EnterpriseWorkbenchSummaryDTO struct {
	OpenTickets             int64 `json:"open_tickets"`
	PendingTickets          int64 `json:"pending_tickets"`
	ProcessingTickets       int64 `json:"processing_tickets"`
	SuspendedTickets        int64 `json:"suspended_tickets"`
	AwaitingCustomerTickets int64 `json:"awaiting_customer_tickets"`
	SLARiskTickets          int64 `json:"sla_risk_tickets"`
	SLABreachedTickets      int64 `json:"sla_breached_tickets"`
	UrgentTickets           int64 `json:"urgent_tickets"`
	UnassignedTickets       int64 `json:"unassigned_tickets"`
	ClosedTodayTickets      int64 `json:"closed_today_tickets"`
	ActiveConversations     int64 `json:"active_conversations"`
	ActiveMeetings          int64 `json:"active_meetings"`
	UnreadNotifications     int64 `json:"unread_notifications"`
	TotalProducts           int64 `json:"total_products"`
	TotalDevices            int64 `json:"total_devices"`
}

type EnterpriseWorkbenchAlertDTO struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Count       int64  `json:"count"`
	ActionLabel string `json:"action_label"`
	ActionURL   string `json:"action_url"`
}

type EnterpriseWorkbenchUsageDTO struct {
	Label             string  `json:"label"`
	AccountBalance    float64 `json:"account_balance"`
	QuotaLimit        float64 `json:"quota_limit"`
	QuotaUsed         float64 `json:"quota_used"`
	QuotaRemaining    float64 `json:"quota_remaining"`
	UsagePercent      float64 `json:"usage_percent"`
	Currency          string  `json:"currency"`
	ProductCount      int64   `json:"product_count"`
	RiskyProductCount int64   `json:"risky_product_count"`
	KeyCount          int64   `json:"key_count"`
	RiskyKeyCount     int64   `json:"risky_key_count"`
	CriticalKeyCount  int64   `json:"critical_key_count"`
	Tone              string  `json:"tone"`
	Meta              string  `json:"meta"`
	SyncStatus        string  `json:"sync_status"`
	LastSyncedAt      string  `json:"last_synced_at"`
}

type EnterpriseWorkbenchQuotaAlertDTO struct {
	ProductID      int64   `json:"product_id"`
	ProductName    string  `json:"product_name"`
	APIKeyID       string  `json:"api_key_id"`
	KeyName        string  `json:"key_name"`
	QuotaLimit     float64 `json:"quota_limit"`
	QuotaUsed      float64 `json:"quota_used"`
	QuotaRemaining float64 `json:"quota_remaining"`
	UsagePercent   float64 `json:"usage_percent"`
	Currency       string  `json:"currency"`
	Severity       string  `json:"severity"`
	LastSyncedAt   string  `json:"last_synced_at"`
}

type EnterpriseWorkbenchQueueCardDTO struct {
	Key         string                        `json:"key"`
	Title       string                        `json:"title"`
	Description string                        `json:"description"`
	Count       int64                         `json:"count"`
	Tone        string                        `json:"tone"`
	ActionURL   string                        `json:"action_url"`
	Items       []EnterpriseTicketListItemDTO `json:"items"`
}

type EnterpriseWorkbenchProductLoadDTO struct {
	ProductID         int64   `json:"product_id"`
	ProductName       string  `json:"product_name"`
	TeamID            int64   `json:"team_id"`
	TeamName          string  `json:"team_name"`
	OpenTickets       int64   `json:"open_tickets"`
	PendingTickets    int64   `json:"pending_tickets"`
	ProcessingTickets int64   `json:"processing_tickets"`
	UnassignedTickets int64   `json:"unassigned_tickets"`
	SLARiskTickets    int64   `json:"sla_risk_tickets"`
	QuotaLimit        float64 `json:"quota_limit"`
	QuotaUsed         float64 `json:"quota_used"`
	QuotaRemaining    float64 `json:"quota_remaining"`
	UsagePercent      float64 `json:"usage_percent"`
	Currency          string  `json:"currency"`
	Tone              string  `json:"tone"`
}

type EnterpriseWorkbenchQueueDTO struct {
	Conversations           []EnterpriseWorkbenchConversationDTO `json:"conversations"`
	ConversationsPagination EnterpriseWorkbenchPaginationDTO     `json:"conversations_pagination"`
	Tickets                 []EnterpriseTicketListItemDTO        `json:"tickets"`
	TicketsPagination       EnterpriseWorkbenchPaginationDTO     `json:"tickets_pagination"`
}

type EnterpriseWorkbenchPaginationDTO struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
	HasMore    bool  `json:"has_more"`
}

type EnterpriseWorkbenchConversationDTO struct {
	ID                  int64  `json:"id"`
	CustomerID          int64  `json:"customer_id"`
	CustomerName        string `json:"customer_name"`
	Status              int    `json:"status"`
	ServiceMode         int    `json:"service_mode"`
	Priority            int    `json:"priority"`
	CurrentAssigneeID   int64  `json:"current_assignee_id"`
	CurrentAssigneeName string `json:"current_assignee_name"`
	CurrentTeamID       int64  `json:"current_team_id"`
	CurrentTeamName     string `json:"current_team_name"`
	ChannelID           int64  `json:"channel_id"`
	ProductName         string `json:"product_name"`
	DeviceNo            string `json:"device_no"`
	LastMessageAt       string `json:"last_message_at"`
	LastActiveAt        string `json:"last_active_at"`
	LastMessageSummary  string `json:"last_message_summary"`
	AgentUnreadCount    int    `json:"agent_unread_count"`
	CustomerUnreadCount int    `json:"customer_unread_count"`
	CustomerOnline      bool   `json:"customer_online"`
	CustomerLastSeenAt  string `json:"customer_last_seen_at"`
}

type EnterpriseActivityDTO struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Meta      string `json:"meta"`
	Tone      string `json:"tone"`
	CreatedAt string `json:"created_at"`
}

type EnterpriseDeviceHealthDTO struct {
	Status string `json:"status"`
	Label  string `json:"label"`
	Count  int64  `json:"count"`
	Tone   string `json:"tone"`
}

type EnterpriseNotificationDTO struct {
	ID                    int64    `json:"id"`
	TenantID              int64    `json:"tenant_id"`
	RecipientUserID       int64    `json:"recipient_user_id"`
	RecipientName         string   `json:"recipient_name"`
	Title                 string   `json:"title"`
	Content               string   `json:"content"`
	NotificationType      string   `json:"notification_type"`
	BizType               string   `json:"biz_type"`
	BizID                 int64    `json:"biz_id"`
	Category              string   `json:"category"`
	Level                 string   `json:"level"`
	Channels              []string `json:"channels"`
	EmailStatus           string   `json:"email_status"` // sent / pending / failed / skipped / disabled
	ActionURL             string   `json:"action_url"`
	DeliveryStatus        string   `json:"delivery_status"`
	ExternalChannelStatus string   `json:"external_channel_status"`
	ReadAt                string   `json:"read_at"`
	CreatedAt             string   `json:"created_at"`
	CanMarkRead           bool     `json:"can_mark_read"`
	IsEventSummary        bool     `json:"is_event_summary"`
	RecipientCount        int64    `json:"recipient_count"`
	UnreadCount           int64    `json:"unread_count"`
	EmailPendingCount     int64    `json:"email_pending_count"`
	EmailFailedCount      int64    `json:"email_failed_count"`
}

// EnterpriseNotificationSummaryDTO 消息中心概览卡口径:
// 未读消息(其中紧急 urgent 条)、已启用邮件的消息类型数、待发邮件数、今日消息数。
type EnterpriseNotificationSummaryDTO struct {
	Total          int64 `json:"total"`
	Unread         int64 `json:"unread"`
	DeliveryTotal  int64 `json:"delivery_total"`
	DeliveryUnread int64 `json:"delivery_unread"`
	MarkableUnread int64 `json:"markable_unread"`
	Urgent         int64 `json:"urgent"`
	EmailEnabled   int64 `json:"email_enabled"`
	PendingEmail   int64 `json:"pending_email"`
	FailedEmail    int64 `json:"failed_email"`
	Today          int64 `json:"today"`
}

type EnterpriseNotificationListDTO struct {
	Summary            EnterpriseNotificationSummaryDTO `json:"summary"`
	Items              []EnterpriseNotificationDTO      `json:"items"`
	Total              int64                            `json:"total"`
	Page               int                              `json:"page"`
	PageSize           int                              `json:"page_size"`
	HasMore            bool                             `json:"has_more"`
	Scope              string                           `json:"scope"`
	CanViewTenantScope bool                             `json:"can_view_tenant_scope"`
}

// EnterpriseNotificationMailSettingDTO 租户邮箱发送配置。Password 永远不回传明文。
type EnterpriseNotificationMailSettingDTO struct {
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
	SMTPHost    string `json:"smtp_host"`
	SMTPPort    int    `json:"smtp_port"`
	Username    string `json:"username"`
	ReplyTo     string `json:"reply_to"`
	RetryPolicy string `json:"retry_policy"`
	UseTLS      bool   `json:"use_tls"`
	Connected   bool   `json:"connected"`
	HasPassword bool   `json:"has_password"`
}

// EnterpriseNotificationRecipientSettingDTO is the current user's effective
// notification mailbox. ProfileEmail comes from the enterprise people record.
type EnterpriseNotificationRecipientSettingDTO struct {
	ProfileEmail    string `json:"profile_email"`
	OverrideEmail   string `json:"override_email"`
	EffectiveEmail  string `json:"effective_email"`
	UseProfileEmail bool   `json:"use_profile_email"`
	EmailEnabled    bool   `json:"email_enabled"`
	ProfileSource   string `json:"profile_source"`
	UpdatedAt       string `json:"updated_at"`
}

type EnterpriseMeetingListItemDTO struct {
	ID               string                  `json:"id"`
	TenantID         int64                   `json:"tenant_id"`
	TicketID         int64                   `json:"ticket_id"`
	TicketNo         string                  `json:"ticket_no"`
	Title            string                  `json:"title"`
	RoomName         string                  `json:"room_name"`
	Status           string                  `json:"status"`
	ProductName      string                  `json:"product_name"`
	DeviceNo         string                  `json:"device_no"`
	CustomerName     string                  `json:"customer_name"`
	CreatedBy        string                  `json:"created_by"`
	ScheduledAt      string                  `json:"scheduled_at"`
	StartedAt        string                  `json:"started_at"`
	EndedAt          string                  `json:"ended_at,omitempty"`
	CreatedAt        string                  `json:"created_at"`
	DurationSeconds  int64                   `json:"duration_seconds"`
	ParticipantCount int64                   `json:"participant_count"`
	Participants     []MeetingParticipantDTO `json:"participants"`
	TranscriptCount  int64                   `json:"transcript_count"`
	AnnotationCount  int64                   `json:"annotation_count"`
	CanEnd           bool                    `json:"can_end"`
	JoinPath         string                  `json:"join_path"`
	JitsiURL         string                  `json:"jitsi_url"`
}

type EnterpriseMeetingListDTO struct {
	Summary  EnterpriseMeetingSummaryDTO    `json:"summary"`
	Items    []EnterpriseMeetingListItemDTO `json:"items"`
	Total    int64                          `json:"total"`
	Page     int                            `json:"page"`
	PageSize int                            `json:"page_size"`
	HasMore  bool                           `json:"has_more"`
}

type EnterpriseMeetingSummaryDTO struct {
	Active             int64 `json:"active"`
	Waiting            int64 `json:"waiting"`
	Ended              int64 `json:"ended"`
	Mine               int64 `json:"mine"`
	ParticipantsOnline int64 `json:"participants_online"`
}

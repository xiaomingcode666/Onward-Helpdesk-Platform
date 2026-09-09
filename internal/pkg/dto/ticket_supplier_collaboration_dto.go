package dto

type TicketSupplierInviteRequest struct {
	ProductModuleID     int64    `json:"product_module_id"`
	PartnerCompanyID    int64    `json:"partner_company_id"`
	PartnerAccountID    int64    `json:"partner_account_id"`
	Reason              string   `json:"reason"`
	Visibility          []string `json:"visibility"`
	AccessDays          int      `json:"access_days"`
	AuthorizationEndsAt string   `json:"authorization_ends_at"`
}

type TicketSupplierOptionDTO struct {
	PartnerCompanyID int64  `json:"partner_company_id"`
	Name             string `json:"name"`
}

type TicketSupplierResolveRequest struct {
	Resolution string `json:"resolution"`
}

type PartnerTicketAssignRequest struct {
	PartnerAccountID int64 `json:"partner_account_id"`
}

type PartnerTicketParticipantRequest struct {
	PartnerAccountID int64 `json:"partner_account_id"`
}

type PartnerTicketProgressCreateRequest struct {
	Content         string `json:"content"`
	MessageType     string `json:"message_type"`
	AssetID         string `json:"asset_id"`
	DurationSeconds int    `json:"duration_seconds"`
}

type PartnerAccountCreateRequest struct {
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Email       string   `json:"email"`
	Phone       string   `json:"phone"`
	Languages   []string `json:"languages"`
	RoleCode    string   `json:"role_code"`
}

type PartnerAccountUpdateRequest struct {
	DisplayName string   `json:"display_name"`
	Email       string   `json:"email"`
	Phone       string   `json:"phone"`
	Languages   []string `json:"languages"`
	RoleCode    string   `json:"role_code"`
	Status      int      `json:"status"`
}

type PartnerAccountCreateResultDTO struct {
	Account         PartnerPortalAccountDTO `json:"account"`
	InitialPassword string                  `json:"initial_password"`
}

type TicketAutoClosePolicyDTO struct {
	Enabled bool `json:"enabled"`
	Days    int  `json:"days"`
}

type TicketSupplierCollaborationDTO struct {
	ID                  int64    `json:"id"`
	TicketID            int64    `json:"ticket_id"`
	ConversationID      int64    `json:"conversation_id,omitempty"`
	TicketNo            string   `json:"ticket_no"`
	TicketTitle         string   `json:"ticket_title"`
	TicketStatus        string   `json:"ticket_status"`
	TicketUpdatedAt     string   `json:"ticket_updated_at,omitempty"`
	ProductID           int64    `json:"product_id"`
	ProductModuleID     int64    `json:"product_module_id"`
	ProductModuleName   string   `json:"product_module_name"`
	PartnerCompanyID    int64    `json:"partner_company_id"`
	PartnerCompanyName  string   `json:"partner_company_name"`
	PartnerAccountID    int64    `json:"partner_account_id"`
	PartnerAccountName  string   `json:"partner_account_name"`
	ParticipantCount    int      `json:"participant_count"`
	Status              string   `json:"status"`
	Reason              string   `json:"reason"`
	Resolution          string   `json:"resolution"`
	Visibility          []string `json:"visibility"`
	FaultCode           string   `json:"fault_code,omitempty"`
	SymptomSummary      string   `json:"symptom_summary,omitempty"`
	DiagnosisSummary    string   `json:"diagnosis_summary,omitempty"`
	InvitedAt           string   `json:"invited_at"`
	ResolvedAt          string   `json:"resolved_at,omitempty"`
	AuthorizationEndsAt string   `json:"authorization_ends_at,omitempty"`
	AuthorizationActive bool     `json:"authorization_active"`
}

type TicketSupplierParticipantDTO struct {
	PartnerAccountID   int64  `json:"partner_account_id"`
	PartnerAccountName string `json:"partner_account_name"`
	Role               string `json:"role"`
	Status             int    `json:"status"`
	JoinedAt           string `json:"joined_at"`
	LeftAt             string `json:"left_at,omitempty"`
}

type PartnerTicketProgressDTO struct {
	ID         int64  `json:"id"`
	EventType  string `json:"event_type"`
	Content    string `json:"content"`
	AuthorID   int64  `json:"author_id"`
	AuthorName string `json:"author_name"`
	CreatedAt  string `json:"created_at"`
}

type PartnerConversationMessageDTO struct {
	ID          int64  `json:"id"`
	SenderID    int64  `json:"sender_id"`
	SenderType  string `json:"sender_type"`
	SenderName  string `json:"sender_name"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"`
	Payload     string `json:"payload"`
	SentAt      string `json:"sent_at"`
}

type PartnerTicketDetailDTO struct {
	Collaboration TicketSupplierCollaborationDTO  `json:"collaboration"`
	Participants  []TicketSupplierParticipantDTO  `json:"participants"`
	Progresses    []PartnerTicketProgressDTO      `json:"progresses"`
	Messages      []PartnerConversationMessageDTO `json:"messages"`
}

type PartnerPortalProfileDTO struct {
	AccountID       int64    `json:"account_id"`
	UserID          int64    `json:"user_id"`
	DisplayName     string   `json:"display_name"`
	Email           string   `json:"email"`
	Phone           string   `json:"phone"`
	CompanyID       int64    `json:"company_id"`
	CompanyName     string   `json:"company_name"`
	PartnerNo       string   `json:"partner_no"`
	PartnerType     string   `json:"partner_type"`
	CountryRegion   string   `json:"country_region"`
	LanguagesJSON   string   `json:"languages_json"`
	LastActiveAt    string   `json:"last_active_at,omitempty"`
	AuthorizationOK bool     `json:"authorization_ok"`
	Roles           []string `json:"roles"`
	CanManageTeam   bool     `json:"can_manage_team"`
	CanInviteTeam   bool     `json:"can_invite_team"`
}

type PartnerPortalAccountDTO struct {
	ID            int64    `json:"id"`
	Username      string   `json:"username"`
	DisplayName   string   `json:"display_name"`
	Email         string   `json:"email"`
	Phone         string   `json:"phone"`
	LanguagesJSON string   `json:"languages_json"`
	Status        int      `json:"status"`
	LastActiveAt  string   `json:"last_active_at,omitempty"`
	UpdatedAt     string   `json:"updated_at"`
	IsCurrent     bool     `json:"is_current"`
	Roles         []string `json:"roles"`
}

type PartnerPortalMeetingDTO struct {
	EnterpriseMeetingListItemDTO
	CollaborationID int64 `json:"collaboration_id"`
}

type PartnerPortalMeetingListDTO struct {
	Summary EnterpriseMeetingSummaryDTO `json:"summary"`
	Items   []PartnerPortalMeetingDTO   `json:"items"`
}

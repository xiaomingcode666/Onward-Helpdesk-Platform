package dto

type TicketQualitySampleDTO struct {
	ID           int64    `json:"id"`
	TicketID     int64    `json:"ticket_id"`
	TicketNo     string   `json:"ticket_no"`
	Title        string   `json:"title"`
	Priority     string   `json:"priority"`
	TicketStatus string   `json:"ticket_status"`
	AssigneeID   int64    `json:"assignee_id"`
	AssigneeName string   `json:"assignee_name"`
	PeriodKey    string   `json:"period_key"`
	Reasons      []string `json:"reasons"`
	Status       string   `json:"status"`
	ReviewerID   int64    `json:"reviewer_id"`
	ReviewNote   string   `json:"review_note"`
	SampledAt    string   `json:"sampled_at"`
	StartedAt    string   `json:"started_at"`
	CompletedAt  string   `json:"completed_at"`
}

type TicketQualitySampleSummaryDTO struct {
	Total      int64 `json:"total"`
	Pending    int64 `json:"pending"`
	InProgress int64 `json:"in_progress"`
	Completed  int64 `json:"completed"`
}

type TicketQualitySamplePageDTO struct {
	Items      []TicketQualitySampleDTO      `json:"items"`
	Total      int64                         `json:"total"`
	Page       int                           `json:"page"`
	PageSize   int                           `json:"page_size"`
	TotalPages int                           `json:"total_pages"`
	Summary    TicketQualitySampleSummaryDTO `json:"summary"`
}

package dto

// TicketCaseSummaryDTO exposes the case lifecycle alongside legacy work steps.
// Empty timestamps are intentional for historical records without evidence.
type TicketCaseSummaryDTO struct {
	MergedIntoID           int64  `json:"merged_into_id"`
	CaseType               string `json:"case_type"`
	PriorityLevel          string `json:"priority_level"`
	PriorityReviewRequired bool   `json:"priority_review_required"`
	CaseStatus             string `json:"case_status"`
	CaseStatusRecorded     bool   `json:"case_status_recorded"`
	CaseOwnerID            int64  `json:"case_owner_id"`
	CaseOwnerName          string `json:"case_owner_name"`
	AcknowledgedAt         string `json:"acknowledged_at"`
	RestoredAt             string `json:"restored_at"`
}

type TicketCaseLifecycleDTO struct {
	WorkflowVersionID int64    `json:"workflow_version_id"`
	WorkflowError     string   `json:"workflow_error,omitempty"`
	Revision          int64    `json:"revision"`
	Status            string   `json:"status"`
	AcknowledgedAt    string   `json:"acknowledged_at"`
	RestoredAt        string   `json:"restored_at"`
	WaitingReason     string   `json:"waiting_reason"`
	AllowedActions    []string `json:"allowed_actions"`
	LegacyRecord      bool     `json:"legacy_record"`
}

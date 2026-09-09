package response

import (
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
)

type TicketProgressResponse struct {
	ID                int64  `json:"id"`
	TicketID          int64  `json:"ticketId"`
	Content           string `json:"content"`
	VisibleToCustomer bool   `json:"visibleToCustomer"`
	AuthorID          int64  `json:"authorId"`
	AuthorName        string `json:"authorName,omitempty"`
	CreatedAt         string `json:"createdAt,omitempty"`
}

type TicketResponse struct {
	dto.TicketIntakeDTO
	ID                     int64              `json:"id"`
	TicketNo               string             `json:"ticketNo"`
	Title                  string             `json:"title"`
	Description            string             `json:"description"`
	Source                 enums.TicketSource `json:"source"`
	Channel                string             `json:"channel"`
	CustomerID             int64              `json:"customerId"`
	ConversationID         int64              `json:"conversationId"`
	Tags                   []TagResponse      `json:"tags,omitempty"`
	Status                 enums.TicketStatus `json:"status"`
	CurrentAssigneeID      int64              `json:"currentAssigneeId"`
	CurrentAssigneeName    string             `json:"currentAssigneeName,omitempty"`
	CreatedBy              int64              `json:"createdBy"`
	CreatedByName          string             `json:"createdByName,omitempty"`
	HandledAt              string             `json:"handledAt,omitempty"`
	CreatedAt              string             `json:"createdAt,omitempty"`
	UpdatedAt              string             `json:"updatedAt,omitempty"`
	Customer               *CustomerResponse  `json:"customer,omitempty"`
	TenantID               int64              `json:"tenantId"`
	ProductID              int64              `json:"productId"`
	ProductModelID         int64              `json:"productModelId"`
	ProductModuleID        int64              `json:"productModuleId"`
	DeviceID               int64              `json:"deviceId"`
	ServiceCodeID          int64              `json:"serviceCodeId"`
	CustomerEntrySessionID int64              `json:"customerEntrySessionId"`
	ServiceRegion          string             `json:"serviceRegion"`
	FaultCode              string             `json:"faultCode"`
	SymptomSummary         string             `json:"symptomSummary"`
	DiagnosisSummary       string             `json:"diagnosisSummary"`
	SLADueAt               string             `json:"slaDueAt,omitempty"`
	ResolvedAt             string             `json:"resolvedAt,omitempty"`
	AssignedAt             string             `json:"assignedAt,omitempty"`
	AcceptedAt             string             `json:"acceptedAt,omitempty"`
	AcceptDeadlineAt       string             `json:"acceptDeadlineAt,omitempty"`
	DispatchAttempts       int                `json:"dispatchAttempts"`
}

type TicketDetailResponse struct {
	Ticket     TicketResponse           `json:"ticket"`
	Progresses []TicketProgressResponse `json:"progresses,omitempty"`
}

type TicketSummaryResponse struct {
	All        int64 `json:"all"`
	Pending    int64 `json:"pending"`
	InProgress int64 `json:"inProgress"`
	Done       int64 `json:"done"`
	Unassigned int64 `json:"unassigned"`
	Mine       int64 `json:"mine"`
	Stale      int64 `json:"stale"`
}

type TicketViewResponse struct {
	ID      int64          `json:"id"`
	Name    string         `json:"name"`
	Filters map[string]any `json:"filters,omitempty"`
	SortNo  int            `json:"sortNo"`
}

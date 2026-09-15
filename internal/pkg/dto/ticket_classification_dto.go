package dto

import "remotehelpdesk/internal/pkg/ticketpolicy"

type TicketClassificationInput struct {
	PriorityLevel  string             `json:"priority_level"`
	PriorityReason string             `json:"priority_reason"`
	ParentTicketID int64              `json:"parent_ticket_id"`
	RelationReason string             `json:"relation_reason"`
	CaseType       string             `json:"case_type"`
	PriorityFacts  ticketpolicy.Facts `json:"priority_facts"`
}

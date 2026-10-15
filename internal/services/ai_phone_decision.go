package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// AIPhoneDecisionHistoryTurn is one already completed turn sent by an
// external voice adapter. The Go service uses it as LLM context only; it does
// not persist the external adapter's conversation state.
type AIPhoneDecisionHistoryTurn struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// AIPhoneDecisionTicket is the ticket draft exchanged with an external voice
// adapter. Tenant and project authorization remain on the ticket endpoint.
type AIPhoneDecisionTicket struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// AIPhoneDecisionRequest is the authenticated text-turn contract for local or
// third-party voice adapters. LineID is populated from the request header by
// the HTTP handler and is intentionally excluded from JSON input.
type AIPhoneDecisionRequest struct {
	SchemaVersion   int                          `json:"schema_version"`
	Provider        string                       `json:"provider"`
	CallID          string                       `json:"call_id"`
	IntentID        string                       `json:"intent_id"`
	LineID          string                       `json:"-"`
	Language        string                       `json:"language"`
	WorkLanguage    string                       `json:"work_language"`
	ProjectKey      string                       `json:"project_key"`
	CallerPhone     string                       `json:"caller_phone"`
	Text            string                       `json:"text"`
	CollectedFields map[string]string            `json:"collected_fields"`
	PendingTicket   *AIPhoneDecisionTicket       `json:"pending_ticket"`
	History         []AIPhoneDecisionHistoryTurn `json:"history"`
}

// AIPhoneDecisionResponse contains only the validated structured decision.
// Raw model output never crosses the service boundary.
type AIPhoneDecisionResponse struct {
	Action          string                 `json:"action"`
	IntentID        string                 `json:"intent_id"`
	ReplyText       string                 `json:"reply_text"`
	CollectedFields map[string]string      `json:"collected_fields"`
	MissingFields   []string               `json:"missing_fields"`
	ReadyToCreate   bool                   `json:"ready_to_create"`
	Ticket          *AIPhoneDecisionTicket `json:"ticket,omitempty"`
}

var ErrAIPhoneDecisionLineNotConfigured = errors.New("phone line is not bound to a tenant")

// DecideText evaluates one turn with the same tenant-scoped LLM and prompt as
// the Go Telnyx call path. It deliberately does not mutate the Telnyx call
// map, send Call Control commands, or create a ticket.
func (s *aiPhoneCallService) DecideText(ctx context.Context, input AIPhoneDecisionRequest) (*AIPhoneDecisionResponse, error) {
	tenantID, ok := AIPhoneTenantForLine(strings.TrimSpace(input.LineID))
	if !ok {
		return nil, ErrAIPhoneDecisionLineNotConfigured
	}
	call := &aiPhoneCall{
		callID:      strings.TrimSpace(input.CallID),
		callerPhone: strings.TrimSpace(input.CallerPhone),
		line: aiPhoneLine{
			LineID:     strings.TrimSpace(input.LineID),
			ProjectKey: strings.TrimSpace(input.ProjectKey),
			Language:   strings.TrimSpace(input.Language),
			TenantID:   tenantID,
		},
		fields: make(map[string]string, len(input.CollectedFields)),
	}
	for key, value := range input.CollectedFields {
		call.fields[key] = value
	}
	call.history = make([]aiPhoneTurn, 0, len(input.History))
	for _, turn := range input.History {
		call.history = append(call.history, aiPhoneTurn{Role: turn.Role, Text: turn.Text})
		if turn.Role == "customer" {
			call.customerTurns++
		}
	}
	if input.PendingTicket != nil {
		call.pendingTicket = &aiPhoneTicketDraft{
			Title:       input.PendingTicket.Title,
			Description: input.PendingTicket.Description,
		}
	}

	decision, err := s.decide(ctx, call, strings.TrimSpace(input.Text))
	if err != nil {
		return nil, err
	}
	response := &AIPhoneDecisionResponse{
		Action:          decision.Action,
		IntentID:        strings.TrimSpace(input.IntentID),
		ReplyText:       decision.ReplyText,
		CollectedFields: decision.CollectedFields,
		MissingFields:   decision.MissingFields,
		ReadyToCreate:   decision.Action == aiPhoneActionPrepareTicket || decision.Action == aiPhoneActionConfirmTicket,
	}
	if decision.Ticket != nil {
		response.Ticket = &AIPhoneDecisionTicket{
			Title:       decision.Ticket.Title,
			Description: decision.Ticket.Description,
		}
	}
	return response, nil
}

func (r AIPhoneDecisionRequest) validate() error {
	if r.SchemaVersion != 1 {
		return errors.New("schema_version must be 1")
	}
	for name, value := range map[string]string{
		"provider": r.Provider, "call_id": r.CallID, "intent_id": r.IntentID,
		"language": r.Language, "work_language": r.WorkLanguage,
		"project_key": r.ProjectKey, "text": r.Text,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
		if utf8.RuneCountInString(value) > 20000 || strings.ContainsRune(value, '\x00') {
			return fmt.Errorf("%s is too long or contains invalid characters", name)
		}
	}
	if len(r.History) > aiPhoneHistoryTurns {
		return fmt.Errorf("history cannot contain more than %d turns", aiPhoneHistoryTurns)
	}
	if len(r.CollectedFields) > 64 {
		return errors.New("too many collected fields")
	}
	for _, turn := range r.History {
		if turn.Role != "customer" && turn.Role != "agent" {
			return errors.New("history role must be customer or agent")
		}
		if strings.TrimSpace(turn.Text) == "" || utf8.RuneCountInString(turn.Text) > 20000 {
			return errors.New("history text is invalid")
		}
	}
	if r.PendingTicket != nil {
		if strings.TrimSpace(r.PendingTicket.Title) == "" || strings.TrimSpace(r.PendingTicket.Description) == "" {
			return errors.New("pending_ticket title and description are required")
		}
		if utf8.RuneCountInString(r.PendingTicket.Title) > 20000 || utf8.RuneCountInString(r.PendingTicket.Description) > 20000 {
			return errors.New("pending_ticket is too long")
		}
	}
	return nil
}

// ValidateForHTTP is kept small and explicit so the third-party handler can
// apply the same contract checks before invoking the model.
func (r AIPhoneDecisionRequest) ValidateForHTTP() error {
	return r.validate()
}

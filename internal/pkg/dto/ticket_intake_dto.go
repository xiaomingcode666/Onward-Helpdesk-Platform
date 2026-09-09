package dto

import "time"

// TicketIntakeInput is shared by authenticated manual intake and context completion.
type TicketIntakeInput struct {
	SourceRecordID string     `json:"source_record_id"`
	ProjectKey     string     `json:"project_key"`
	TicketType     string     `json:"ticket_type"`
	CallerName     string     `json:"caller_name"`
	CallerPhone    string     `json:"caller_phone"`
	ReceivedAt     *time.Time `json:"received_at"`
}

type TicketIntakeDTO struct {
	TicketIntakeInput
	ContextStatus  string   `json:"context_status"`
	MissingContext []string `json:"missing_context"`
}

type TicketIntakeRule struct {
	ProjectKey     string   `json:"project_key"`
	Channel        string   `json:"channel"`
	TicketType     string   `json:"ticket_type"`
	RequiredFields []string `json:"required_fields"`
}

type TicketIntakePolicy struct {
	Rules []TicketIntakeRule `json:"rules"`
}

type CompleteTicketIntakeRequest struct {
	CustomerID    int64  `json:"customer_id"`
	ProjectKey    string `json:"project_key"`
	TicketType    string `json:"ticket_type"`
	CallerName    string `json:"caller_name"`
	CallerPhone   string `json:"caller_phone"`
	ProductID     int64  `json:"product_id"`
	DeviceID      int64  `json:"device_id"`
	ServiceRegion string `json:"service_region"`
}

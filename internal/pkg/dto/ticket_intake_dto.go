package dto

import "time"

// TicketIntakeInput 携带工单的来源信息和来电/来信人资料。
type TicketIntakeInput struct {
	SourceRecordID string     `json:"source_record_id"`
	ProjectKey     string     `json:"project_key"`
	TicketType     string     `json:"ticket_type"`
	CallerName     string     `json:"caller_name"`
	CallerPhone    string     `json:"caller_phone"`
	ReceivedAt     *time.Time `json:"received_at"`
}

type TicketIntakeDTO struct {
	ProjectConfigVersionID int64 `json:"project_config_version_id"`
	TicketIntakeInput
}

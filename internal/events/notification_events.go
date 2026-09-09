package events

import "time"

const (
	ConversationAssignTypeAssign     = "assign"
	ConversationAssignTypeTransfer   = "transfer"
	ConversationAssignTypeAutoAssign = "auto_assign"
)

// 核心事件类型
const (
	EventProductCreated            = "product.created"
	EventTicketCreated             = "ticket.created"
	EventTicketAssigned            = "ticket.assigned"
	EventConversationAssigned      = "conversation.assigned"
	EventTicketClosed              = "ticket.closed"
	EventDiagnosisCompleted        = "diagnosis.completed"
	EventDiagnosisHandoffDecided   = "diagnosis.handoff.decided"
	EventMeetingEnded              = "meeting.ended"
	EventKnowledgeCandidateCreated = "knowledge.candidate.created"
	EventQuotaExceeded             = "quota.exceeded"
	EventSLAWarning                = "sla.warning"
	EventSecurityAlert             = "security.alert"
	EventAuditLogCreated           = "audit.log.created"
	EventAccessQueryExecuted       = "access.query.executed"
	EventDeviceAlarmRaised         = "device_alarm.raised"
)

type ProductCreatedEvent struct {
	EventID    string
	ProductID  int64
	TenantID   int64
	OperatorID int64
}

type TicketCreatedEvent struct {
	EventID    string
	TicketID   int64
	OperatorID int64
}

type TicketAssignedEvent struct {
	EventID    string
	TicketID   int64
	FromUserID int64
	ToUserID   int64
	OperatorID int64
	Reason     string
}

type ConversationAssignedEvent struct {
	EventID        string
	ConversationID int64
	FromUserID     int64
	ToUserID       int64
	OperatorID     int64
	Reason         string
	AssignType     string
}

// TicketClosedEvent 工单关闭事件
type TicketClosedEvent struct {
	EventID    string
	TicketID   int64
	TenantID   int64
	OperatorID int64
	Resolution string
	OccurredAt time.Time
}

// DiagnosisCompletedEvent 诊断完成事件
type DiagnosisCompletedEvent struct {
	EventID         string
	SessionID       string
	TenantID        string
	CustomerID      string
	Status          string // resolved / escalated
	ConfidenceScore float64
	HandoffReason   string
}

// MeetingEndedEvent 会议结束事件
type MeetingEndedEvent struct {
	MeetingID  string
	TicketID   string
	TenantID   string
	EndedBy    string
	DurationMs int64
}

// KnowledgeCandidateCreatedEvent 知识候选创建事件
type KnowledgeCandidateCreatedEvent struct {
	EventID     string
	CandidateID int64
	SourceType  string // ticket_closed / meeting_ended / low_score
	SourceID    string
	TenantID    int64
	Suggestion  string
}

// DeviceAlarmRaisedEvent is emitted after an MQTT fault alarm and its outbox
// record are committed together.
type DeviceAlarmRaisedEvent struct {
	EventID        string
	AlarmID        string
	TenantID       int64
	ConnectorID    int64
	DeviceID       int64
	ProductID      int64
	ProductModelID int64
	DeviceSerial   string
	FaultCode      string
	Severity       string
	Message        string
	OccurredAt     time.Time
}

// QuotaExceededEvent 配额超限事件
type QuotaExceededEvent struct {
	EventID      string
	TenantID     int64
	ProductID    int64
	ResourceType string
	LimitValue   int64
	CurrentValue int64
	APIKeyID     string
}

// SLAWarningEvent SLA 告警事件
type SLAWarningEvent struct {
	EventID       string
	TicketID      string
	TenantID      string
	ViolationType string // frt / assignment / resolution
	Severity      string // warning / breach / critical
	TargetMinutes int
	ActualMinutes int
}

// SecurityAlertEvent 安全告警事件
type SecurityAlertEvent struct {
	EventID     string
	TenantID    string
	ActorID     string
	ActorType   string
	AlertType   string // unauthorized_access / suspicious_activity / permission_change
	Severity    string // low / medium / high / critical
	Description string
	IPAddress   string
}

// AuditLogCreatedEvent 审计日志创建事件
type AuditLogCreatedEvent struct {
	EventID    string
	AuditLogID int64
	TenantID   int64
	ActorID    string
	Action     string
	Domain     string
}

// AccessQueryExecutedEvent 受控查询执行事件
type AccessQueryExecutedEvent struct {
	ConnectorID string
	TenantID    string
	Query       string
	Duration    int64
	Status      string
	ResultSize  int
}

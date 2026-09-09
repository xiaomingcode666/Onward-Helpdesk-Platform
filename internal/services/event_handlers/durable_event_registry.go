package event_handlers

import (
	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/pkg/eventbus"
)

func init() {
	eventbus.RegisterDurable[events.ProductCreatedEvent](events.EventProductCreated)
	eventbus.RegisterDurable[events.TicketCreatedEvent](events.EventTicketCreated)
	eventbus.RegisterDurable[events.TicketAssignedEvent](events.EventTicketAssigned)
	eventbus.RegisterDurable[events.ConversationAssignedEvent](events.EventConversationAssigned)
	eventbus.RegisterDurable[events.TicketClosedEvent](events.EventTicketClosed)
	eventbus.RegisterDurable[events.TicketReopenedEvent](events.FaultStatsEventTicketReopened)
	eventbus.RegisterDurable[events.DiagnosisCompletedEvent](events.EventDiagnosisCompleted)
	eventbus.RegisterDurable[events.MeetingEndedEvent](events.EventMeetingEnded)
	eventbus.RegisterDurable[events.KnowledgeCandidateCreatedEvent](events.EventKnowledgeCandidateCreated)
	eventbus.RegisterDurable[events.CustomerRatingCreatedEvent](events.FaultStatsEventCustomerRating)
	eventbus.RegisterDurable[events.QuotaExceededEvent](events.EventQuotaExceeded)
	eventbus.RegisterDurable[events.SLAWarningEvent](events.EventSLAWarning)
	eventbus.RegisterDurable[events.SecurityAlertEvent](events.EventSecurityAlert)
	eventbus.RegisterDurable[events.AuditLogCreatedEvent](events.EventAuditLogCreated)
	eventbus.RegisterDurable[events.AccessQueryExecutedEvent](events.EventAccessQueryExecuted)
	eventbus.RegisterDurable[events.DeviceAlarmRaisedEvent](events.EventDeviceAlarmRaised)
}

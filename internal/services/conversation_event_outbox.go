package services

import (
	"strconv"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/pkg/eventbus"

	"gorm.io/gorm"
)

func enqueueConversationAssignedEventTx(tx *gorm.DB, tenantID, assignmentID int64, event *events.ConversationAssignedEvent, occurredAt time.Time) error {
	if tx == nil || event == nil || tenantID <= 0 || event.ConversationID <= 0 || assignmentID <= 0 {
		return nil
	}
	event.EventID = "tenant:" + strconv.FormatInt(tenantID, 10) + ":conversation.assigned:" + strconv.FormatInt(event.ConversationID, 10) + ":" + strconv.FormatInt(assignmentID, 10)
	_, err := eventbus.EnqueueTx(tx, eventbus.DurableEvent{
		TenantID:       tenantID,
		IdempotencyKey: event.EventID,
		EventType:      events.EventConversationAssigned,
		Payload:        *event,
		Source:         "conversation_service",
		AggregateID:    strconv.FormatInt(event.ConversationID, 10),
		ActorID:        strconv.FormatInt(event.OperatorID, 10),
		ActorType:      "user",
		CreatedAt:      occurredAt,
	})
	return err
}

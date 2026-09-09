package services

import (
	"strconv"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/eventbus"

	"gorm.io/gorm"
)

// enqueueTicketAssignedEventTx 在事务内入队 ticket.assigned 持久化事件，
// 触发企微通知（ticket_assigned_event_handler.go）与站内通知（notification_event_handler.go）。
// progressID 必须传本次派单已写入的 TicketProgress.ID——幂等键含 progressID，
// 保证同一 progress 行重试时不会重复通知。
//
// 所有分配入口（人工指派、自动派单、会话派单回写）统一走该函数，消除入口差异。
func enqueueTicketAssignedEventTx(
	tx *gorm.DB,
	ticket *models.Ticket,
	fromUserID, toUserID int64,
	reason string,
	operator *dto.AuthPrincipal,
	progressID int64,
	now time.Time,
) (*events.TicketAssignedEvent, error) {
	eventID := "tenant:" + strconv.FormatInt(ticket.TenantID, 10) +
		":ticket.assigned:" + strconv.FormatInt(ticket.ID, 10) +
		":" + strconv.FormatInt(progressID, 10)
	assignedEvent := &events.TicketAssignedEvent{
		EventID:    eventID,
		TicketID:   ticket.ID,
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		OperatorID: operator.UserID,
		Reason:     reason,
	}
	if _, err := eventbus.EnqueueTx(tx, eventbus.DurableEvent{
		TenantID:       ticket.TenantID,
		IdempotencyKey: eventID,
		EventType:      events.EventTicketAssigned,
		Payload:        *assignedEvent,
		Source:         "ticket_assigned_event",
		AggregateID:    strconv.FormatInt(ticket.ID, 10),
		ActorID:        strconv.FormatInt(operator.UserID, 10),
		ActorType:      "user",
		CreatedAt:      now,
	}); err != nil {
		return nil, err
	}
	return assignedEvent, nil
}

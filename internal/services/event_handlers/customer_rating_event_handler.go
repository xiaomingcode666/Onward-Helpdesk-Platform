package event_handlers

import (
	"context"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	eventbus.Register[events.CustomerRatingCreatedEvent]().Subscribe(handleCustomerRatingConversationEvent)
}

func handleCustomerRatingConversationEvent(_ context.Context, event events.CustomerRatingCreatedEvent) error {
	if event.FeedbackID <= 0 || event.TicketID <= 0 || event.TenantID <= 0 {
		return nil
	}
	feedback := repositories.TicketFeedbackRepository.Get(sqls.DB(), event.FeedbackID)
	ticket := repositories.TicketRepository.Get(sqls.DB(), event.TicketID)
	if feedback == nil || ticket == nil || feedback.TenantID != event.TenantID || ticket.TenantID != event.TenantID || feedback.TicketID != ticket.ID {
		return nil
	}
	return services.CustomerTicketActionService.PublishFeedbackConversationEvent(ticket, feedback)
}

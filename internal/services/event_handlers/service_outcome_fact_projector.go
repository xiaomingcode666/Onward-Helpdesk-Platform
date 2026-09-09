package event_handlers

import (
	"context"
	"errors"
	"strconv"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	eventbus.Register[events.TicketClosedEvent]().Subscribe(projectTicketClosedToServiceOutcome)
	eventbus.Register[events.TicketReopenedEvent]().Subscribe(projectTicketReopenedToServiceOutcome)
	eventbus.Register[events.CustomerRatingCreatedEvent]().Subscribe(projectCustomerRatingToServiceOutcome)
	eventbus.Register[events.MeetingEndedEvent]().Subscribe(projectMeetingEndedToServiceOutcome)
	eventbus.Register[events.KnowledgeCandidateCreatedEvent]().Subscribe(projectKnowledgeCandidateToServiceOutcome)
}

func projectTicketClosedToServiceOutcome(ctx context.Context, event events.TicketClosedEvent) error {
	return rebuildServiceOutcomeFact(ctx, event.TenantID, event.TicketID)
}

func projectTicketReopenedToServiceOutcome(ctx context.Context, event events.TicketReopenedEvent) error {
	return rebuildServiceOutcomeFact(ctx, event.TenantID, event.TicketID)
}

func projectCustomerRatingToServiceOutcome(ctx context.Context, event events.CustomerRatingCreatedEvent) error {
	return rebuildServiceOutcomeFact(ctx, event.TenantID, event.TicketID)
}

func projectMeetingEndedToServiceOutcome(ctx context.Context, event events.MeetingEndedEvent) error {
	tenantID, _ := strconv.ParseInt(event.TenantID, 10, 64)
	ticketID, _ := strconv.ParseInt(event.TicketID, 10, 64)
	return rebuildServiceOutcomeFact(ctx, tenantID, ticketID)
}

func projectKnowledgeCandidateToServiceOutcome(ctx context.Context, event events.KnowledgeCandidateCreatedEvent) error {
	if event.TenantID <= 0 || event.CandidateID <= 0 {
		return nil
	}
	db := sqls.DB()
	if db == nil || !db.Migrator().HasTable(&models.TicketServiceOutcomeFact{}) {
		return nil
	}
	var candidate models.KnowledgeCandidate
	if err := db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", event.TenantID, event.CandidateID).
		First(&candidate).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return rebuildServiceOutcomeFact(ctx, event.TenantID, candidate.TicketID)
}

func rebuildServiceOutcomeFact(ctx context.Context, tenantID, ticketID int64) error {
	db := sqls.DB()
	if db == nil || tenantID <= 0 || ticketID <= 0 || !db.Migrator().HasTable(&models.TicketServiceOutcomeFact{}) {
		return nil
	}
	_, err := services.ServiceOutcomeFactService.RebuildTicket(ctx, tenantID, ticketID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return err
}

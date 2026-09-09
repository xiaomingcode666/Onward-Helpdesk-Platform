package event_handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/routes"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/common/strs"
)

func init() {
	// TicketClosedEvent 的处理职责：
	// 核心副作用（DeviceServiceRecord、KnowledgeCandidate、TicketQualityClue 创建）
	// 已在 TicketLifecycleService.Close 中同步完成。
	// 此处理器仅保留异步通知和审计等非核心职责。
	eventbus.Register[events.TicketClosedEvent]().Subscribe(handleTicketClosedEvent)
}

// handleTicketClosedEvent 工单关闭时的异步通知处理
// 注意：DeviceServiceRecord、KnowledgeCandidate、TicketQualityClue 等核心记录
// 由 TicketLifecycleService.Close 在事务内同步创建，不应在此重复。
func handleTicketClosedEvent(ctx context.Context, event events.TicketClosedEvent) error {
	slog.Info("ticket closed event received",
		"ticketId", event.TicketID,
		"tenantId", event.TenantID,
		"operatorId", event.OperatorID,
		"resolution", event.Resolution)
	ticket := services.TicketService.Get(event.TicketID)
	if ticket == nil || ticket.TenantID <= 0 || ticket.TenantID != event.TenantID {
		return nil
	}
	recipients, err := services.NotificationAudienceService.ResolveTicketRecipients(ticket)
	if err != nil {
		return err
	}
	ticketNo := strs.DefaultIfBlank(ticket.TicketNo, fmt.Sprintf("#%d", ticket.ID))
	content := strings.TrimSpace(event.Resolution)
	if content == "" {
		content = strings.TrimSpace(ticket.Title)
	}
	eventID := strings.TrimSpace(event.EventID)
	if eventID == "" {
		occurredAt := event.OccurredAt
		if occurredAt.IsZero() && ticket.HandledAt != nil {
			occurredAt = *ticket.HandledAt
		}
		eventID = fmt.Sprintf("ticket.closed:%d:%d", ticket.ID, occurredAt.UTC().UnixNano())
	}
	return services.NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
		TenantID:         ticket.TenantID,
		Title:            fmt.Sprintf("工单 %s 已关闭", ticketNo),
		Content:          content,
		NotificationType: "ticket_closed",
		BizType:          "ticket",
		BizID:            ticket.ID,
		ActionURL:        routes.EnterpriseTicketWorkbenchPath(ticket.ID),
		Category:         "ticket",
		Level:            "info",
		Channels:         "in_app",
		IdempotencyKey:   eventID + ":notification",
	})
}

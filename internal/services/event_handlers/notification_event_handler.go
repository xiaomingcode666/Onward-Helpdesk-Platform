package event_handlers

import (
	"context"
	"fmt"
	"strings"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/pkg/routes"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/common/strs"
)

func init() {
	eventbus.Register[events.ProductCreatedEvent]().Subscribe(handleProductCreatedInAppNotification)
	eventbus.Register[events.TicketCreatedEvent]().Subscribe(handleTicketCreatedInAppNotification)
	eventbus.Register[events.TicketAssignedEvent]().Subscribe(handleTicketAssignedInAppNotification)
	eventbus.Register[events.ConversationAssignedEvent]().Subscribe(handleConversationAssignedInAppNotification)
}

func handleProductCreatedInAppNotification(ctx context.Context, event events.ProductCreatedEvent) error {
	if event.ProductID <= 0 || event.TenantID <= 0 {
		return nil
	}
	product := services.ProductService.Get(event.ProductID)
	if product == nil || product.TenantID != event.TenantID {
		return nil
	}
	recipients, err := services.NotificationAudienceService.ResolveProductRecipients(product, event.OperatorID)
	if err != nil {
		return err
	}
	content := fmt.Sprintf("产品 %s（%s）已创建，可进入产品中心检查维修组、知识库和机器人配置。", product.Name, product.Code)
	return services.NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
		TenantID:         product.TenantID,
		Title:            fmt.Sprintf("产品 %s 已创建", product.Name),
		Content:          content,
		NotificationType: "product_created",
		BizType:          "product",
		BizID:            product.ID,
		ActionURL:        fmt.Sprintf("/enterprise/products/%d", product.ID),
		Category:         "system",
		Level:            "info",
		Channels:         "in_app",
		IdempotencyKey:   firstEventID(event.EventID, fmt.Sprintf("product.created:%d", product.ID)),
	})
}

func handleTicketCreatedInAppNotification(ctx context.Context, event events.TicketCreatedEvent) error {
	if event.TicketID <= 0 {
		return nil
	}
	ticket := services.TicketService.Get(event.TicketID)
	if ticket == nil || ticket.TenantID <= 0 {
		return nil
	}
	recipients, err := services.NotificationAudienceService.ResolveTicketRecipients(ticket)
	if err != nil {
		return err
	}
	ticketNo := strs.DefaultIfBlank(ticket.TicketNo, fmt.Sprintf("#%d", ticket.ID))
	title := fmt.Sprintf("新工单 %s 待处理", ticketNo)
	if ticket.ConversationID > 0 {
		title = fmt.Sprintf("转人工工单 %s 待处理", ticketNo)
	}
	content := strings.TrimSpace(ticket.Title)
	if ticket.CurrentAssigneeID <= 0 {
		content += "\n当前未指派处理人，请及时派工。"
	}
	level := "info"
	if ticket.CurrentAssigneeID <= 0 {
		level = "warning"
	}
	if services.DeriveTicketPriority(*ticket) == "critical" {
		level = "urgent"
	}
	return services.NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
		TenantID:         ticket.TenantID,
		Title:            title,
		Content:          strings.TrimSpace(content),
		NotificationType: "ticket_created",
		BizType:          "ticket",
		BizID:            ticket.ID,
		ActionURL:        routes.EnterpriseTicketWorkbenchPath(ticket.ID),
		Category:         "ticket",
		Level:            level,
		Channels:         "in_app",
		IdempotencyKey:   firstEventID(event.EventID, fmt.Sprintf("ticket.created:%d", ticket.ID)),
	})
}

func handleTicketAssignedInAppNotification(ctx context.Context, event events.TicketAssignedEvent) error {
	if event.TicketID <= 0 || event.ToUserID <= 0 {
		return nil
	}
	ticket := services.TicketService.Get(event.TicketID)
	if ticket == nil {
		return nil
	}
	if err := services.NotificationService.ReconcileTicketCreatedAfterAssignment(ticket); err != nil {
		return err
	}
	if err := services.NotificationService.ReconcileTicketAssignedAfterReassignment(ticket, event.FromUserID, event.ToUserID); err != nil {
		return err
	}
	content := i18nx.Getf(i18nx.DefaultLocale, "notification.ticketAssigned.line", strs.DefaultIfBlank(ticket.TicketNo, fmt.Sprintf("#%d", ticket.ID)))
	if title := strings.TrimSpace(ticket.Title); title != "" {
		content = content + "\n" + title
	}
	if reason := strings.TrimSpace(event.Reason); reason != "" {
		content = content + "\n" + i18nx.Getf(i18nx.DefaultLocale, "notification.ticketAssigned.reason", reason)
	}
	recipients, err := services.NotificationAudienceService.ResolveTicketAssignee(ticket, event.ToUserID)
	if err != nil {
		return err
	}
	return services.NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
		TenantID:         ticket.TenantID,
		Title:            i18nx.Get("notification.ticketAssigned.title"),
		Content:          content,
		NotificationType: "ticket_assigned",
		BizType:          "ticket",
		BizID:            ticket.ID,
		ActionURL:        routes.EnterpriseTicketWorkbenchPath(ticket.ID),
		Category:         "ticket",
		Level:            "info",
		Channels:         "in_app",
		IdempotencyKey:   firstEventID(event.EventID, ""),
	})
}

func handleConversationAssignedInAppNotification(ctx context.Context, event events.ConversationAssignedEvent) error {
	if event.ConversationID <= 0 || event.ToUserID <= 0 {
		return nil
	}
	conversation := services.ConversationService.Get(event.ConversationID)
	if conversation == nil {
		return nil
	}
	content := i18nx.Getf(i18nx.DefaultLocale, "notification.conversationAssigned.line", fmt.Sprint(conversation.ID))
	if summary := strings.TrimSpace(services.ConversationService.BuildConversationSummary(conversation)); summary != "" {
		content = content + "\n" + summary
	}
	if reason := strings.TrimSpace(event.Reason); reason != "" {
		reasonKey := "notification.conversationAssigned.reason"
		if strings.TrimSpace(event.AssignType) == events.ConversationAssignTypeTransfer {
			reasonKey = "notification.conversationTransferred.reason"
		}
		content = content + "\n" + i18nx.Getf(i18nx.DefaultLocale, reasonKey, reason)
	}
	recipients, err := services.NotificationAudienceService.ResolveConversationAssignee(conversation, event.ToUserID)
	if err != nil {
		return err
	}
	return services.NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
		TenantID:         conversation.TenantID,
		Title:            conversationAssignedNotifyTitle(event.AssignType),
		Content:          content,
		NotificationType: "conversation_assigned",
		BizType:          "conversation",
		BizID:            conversation.ID,
		ActionURL:        routes.EnterpriseConversationWorkbenchPath(conversation.ID),
		Category:         "ticket",
		Level:            "info",
		Channels:         "in_app",
		IdempotencyKey:   firstEventID(event.EventID, ""),
	})
}

func firstEventID(eventID, fallback string) string {
	if value := strings.TrimSpace(eventID); value != "" {
		return value + ":notification"
	}
	if value := strings.TrimSpace(fallback); value != "" {
		return value + ":notification"
	}
	return ""
}

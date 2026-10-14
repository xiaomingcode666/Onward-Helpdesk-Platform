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
	return services.NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
		TenantID:         product.TenantID,
		TitleText:        i18nx.NewText("notificationText.productCreated.title", i18nx.Params{"product": product.Name}),
		ContentText:      i18nx.NewText("notificationText.productCreated.content", i18nx.Params{"product": product.Name, "code": product.Code}),
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
	title := i18nx.NewText("notificationText.ticketCreated.title", i18nx.Params{"ticketNo": ticketNo})
	if ticket.ConversationID > 0 {
		title = i18nx.NewText("notificationText.ticketCreated.handoffTitle", i18nx.Params{"ticketNo": ticketNo})
	}
	content := i18nx.NewText("notificationText.ticketCreated.content", i18nx.Params{"ticketTitle": strings.TrimSpace(ticket.Title)})
	if ticket.CurrentAssigneeID <= 0 {
		content = i18nx.NewText("notificationText.ticketCreated.contentUnassigned", i18nx.Params{"ticketTitle": strings.TrimSpace(ticket.Title)})
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
		TitleText:        title,
		ContentText:      content,
		NotificationType: "ticket_created",
		BizType:          "ticket",
		BizID:            ticket.ID,
		ActionURL:        routes.EnterpriseTicketWorkbenchPath(ticket.ID),
		Category:         "ticket",
		Level:            level,
		Channels:         "in_app",
		IdempotencyKey:   firstEventID(event.EventID, fmt.Sprintf("ticket.created:%d", ticket.ID)),
		// 模板变量：已批准模板按接收人语言渲染时使用，避免出现未替换的占位符。
		TemplateCode: "ticket_created",
		TemplateVariables: map[string]string{
			"TicketNo":    ticketNo,
			"TicketTitle": strings.TrimSpace(ticket.Title),
		},
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
	contentLines := []any{i18nx.NewText("notificationText.ticketAssigned.line", i18nx.Params{"ticketNo": strs.DefaultIfBlank(ticket.TicketNo, fmt.Sprintf("#%d", ticket.ID))})}
	if title := strings.TrimSpace(ticket.Title); title != "" {
		contentLines = append(contentLines, title)
	}
	if reason := strings.TrimSpace(event.Reason); reason != "" {
		contentLines = append(contentLines, i18nx.NewText("notificationText.ticketAssigned.reason", i18nx.Params{"reason": services.SystemReasonParam(reason)}))
	}
	recipients, err := services.NotificationAudienceService.ResolveTicketAssignee(ticket, event.ToUserID)
	if err != nil {
		return err
	}
	return services.NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
		TenantID:         ticket.TenantID,
		TitleText:        i18nx.NewText("notificationText.ticketAssigned.title", nil),
		ContentText:      services.JoinTextLines(contentLines...),
		NotificationType: "ticket_assigned",
		BizType:          "ticket",
		BizID:            ticket.ID,
		ActionURL:        routes.EnterpriseTicketWorkbenchPath(ticket.ID),
		Category:         "ticket",
		Level:            "info",
		Channels:         "in_app",
		IdempotencyKey:   firstEventID(event.EventID, ""),
		// 模板变量：已批准模板按接收人语言渲染时使用，避免出现未替换的占位符。
		TemplateCode: "ticket_assigned",
		TemplateVariables: map[string]string{
			"TicketNo":    strs.DefaultIfBlank(ticket.TicketNo, fmt.Sprintf("#%d", ticket.ID)),
			"TicketTitle": strings.TrimSpace(ticket.Title),
			"Reason":      strings.TrimSpace(event.Reason),
		},
		TextVariables: services.SystemReasonTextVariable("Reason", event.Reason),
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
	contentLines := []any{i18nx.NewText("notificationText.conversationAssigned.line", i18nx.Params{"conversationId": fmt.Sprint(conversation.ID)})}
	if summary := strings.TrimSpace(services.ConversationService.BuildConversationSummary(conversation)); summary != "" {
		contentLines = append(contentLines, summary)
	}
	if reason := strings.TrimSpace(event.Reason); reason != "" {
		reasonKey := "notificationText.conversationAssigned.reason"
		if strings.TrimSpace(event.AssignType) == events.ConversationAssignTypeTransfer {
			reasonKey = "notificationText.conversationTransferred.reason"
		}
		contentLines = append(contentLines, i18nx.NewText(reasonKey, i18nx.Params{"reason": services.SystemReasonParam(reason)}))
	}
	recipients, err := services.NotificationAudienceService.ResolveConversationAssignee(conversation, event.ToUserID)
	if err != nil {
		return err
	}
	return services.NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
		TenantID:         conversation.TenantID,
		TitleText:        conversationAssignedNotifyTitleText(event.AssignType),
		ContentText:      services.JoinTextLines(contentLines...),
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

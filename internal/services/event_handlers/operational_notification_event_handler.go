package event_handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/pkg/routes"
	"remotehelpdesk/internal/services"
)

func init() {
	eventbus.Register[events.MeetingEndedEvent]().Subscribe(handleMeetingEndedInAppNotification)
	eventbus.Register[events.KnowledgeCandidateCreatedEvent]().Subscribe(handleKnowledgeCandidateCreatedInAppNotification)
	eventbus.Register[events.QuotaExceededEvent]().Subscribe(handleQuotaExceededInAppNotification)
	eventbus.Register[events.SLAWarningEvent]().Subscribe(handleSLAWarningInAppNotification)
	eventbus.Register[events.SecurityAlertEvent]().Subscribe(handleSecurityAlertInAppNotification)
	eventbus.Register[events.DeviceAlarmRaisedEvent]().Subscribe(handleDeviceAlarmRaisedInAppNotification)
}

func handleDeviceAlarmRaisedInAppNotification(ctx context.Context, event events.DeviceAlarmRaisedEvent) error {
	if event.TenantID <= 0 || strings.TrimSpace(event.AlarmID) == "" {
		return nil
	}

	var (
		recipients []services.NotificationRecipient
		err        error
	)
	if event.ProductID > 0 {
		product := services.ProductService.Get(event.ProductID)
		if product == nil || product.TenantID != event.TenantID {
			return nil
		}
		recipients, err = services.NotificationAudienceService.ResolveProductRecipients(product, 0)
	} else {
		recipients, err = services.NotificationAudienceService.ResolveTenantRecipients(event.TenantID, constants.PermissionProductView.Code)
	}
	if err != nil {
		return err
	}

	var deviceLabel any = strings.TrimSpace(event.DeviceSerial)
	if deviceLabel == "" {
		deviceLabel = i18nx.NewText("notificationText.deviceNumber", i18nx.Params{"id": event.DeviceID})
	}
	content := strings.TrimSpace(event.Message)
	contentText := i18nx.Text{}
	if content == "" {
		contentText = i18nx.NewText("notificationText.deviceAlarm.content", i18nx.Params{"device": deviceLabel, "faultCode": strings.TrimSpace(event.FaultCode)})
	}
	bizID := event.DeviceID
	if bizID <= 0 {
		bizID = event.ProductID
	}
	if bizID <= 0 {
		bizID = event.ConnectorID
	}
	actionURL := fmt.Sprintf("/enterprise/access?connectorId=%d", event.ConnectorID)
	if event.ProductID > 0 {
		actionURL = fmt.Sprintf("/enterprise/products/%d?tab=devices&deviceId=%d", event.ProductID, event.DeviceID)
	}
	channels := "in_app"
	if notificationSeverityLevel(event.Severity) == "urgent" {
		channels = "in_app,email"
	}
	return services.NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
		TenantID:         event.TenantID,
		TitleText:        i18nx.NewText("notificationText.deviceAlarm.title", i18nx.Params{"device": deviceLabel}),
		Content:          content,
		ContentText:      contentText,
		NotificationType: "device_alarm_raised",
		BizType:          "device_alarm",
		BizID:            bizID,
		ActionURL:        actionURL,
		Category:         "system",
		Level:            notificationSeverityLevel(event.Severity),
		Channels:         channels,
		IdempotencyKey:   firstEventID(event.EventID, fmt.Sprintf("device_alarm.raised:%s", event.AlarmID)),
	})
}

func handleMeetingEndedInAppNotification(ctx context.Context, event events.MeetingEndedEvent) error {
	ticketID, err := strconv.ParseInt(event.TicketID, 10, 64)
	if err != nil || ticketID <= 0 {
		return nil
	}
	ticket := services.TicketService.Get(ticketID)
	if ticket == nil || fmt.Sprint(ticket.TenantID) != strings.TrimSpace(event.TenantID) {
		return nil
	}
	recipients, err := services.NotificationAudienceService.ResolveTicketRecipients(ticket)
	if err != nil {
		return err
	}
	return services.NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
		TenantID:         ticket.TenantID,
		TitleText:        i18nx.NewText("notificationText.meetingEnded.title", nil),
		ContentText:      i18nx.NewText("notificationText.meetingEnded.content", i18nx.Params{"ticketNo": notificationTicketNo(ticket.ID, ticket.TicketNo), "duration": services.MeetingDurationText(event.DurationMs)}),
		NotificationType: "meeting_ended",
		BizType:          "ticket",
		BizID:            ticket.ID,
		ActionURL:        routes.EnterpriseTicketWorkbenchPath(ticket.ID),
		Category:         "video",
		Level:            "info",
		Channels:         "in_app",
		IdempotencyKey:   fmt.Sprintf("meeting.ended:%s:notification", event.MeetingID),
	})
}

func handleKnowledgeCandidateCreatedInAppNotification(ctx context.Context, event events.KnowledgeCandidateCreatedEvent) error {
	if event.TenantID <= 0 || event.CandidateID <= 0 {
		return nil
	}
	if !services.TenantCapabilityService.AIEnabled(event.TenantID) {
		return nil
	}
	recipients, err := services.NotificationAudienceService.ResolveTenantRecipients(event.TenantID, constants.PermissionKnowledgeBaseView.Code)
	if err != nil {
		return err
	}
	content := strings.TrimSpace(event.Suggestion)
	contentText := i18nx.Text{}
	if content == "" {
		contentText = i18nx.NewText("notificationText.knowledgeCandidate.content", nil)
	}
	return services.NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
		TenantID:         event.TenantID,
		TitleText:        i18nx.NewText("notificationText.knowledgeCandidate.title", nil),
		Content:          content,
		ContentText:      contentText,
		NotificationType: "knowledge_candidate_created",
		BizType:          "knowledge_candidate",
		BizID:            event.CandidateID,
		ActionURL:        fmt.Sprintf("/enterprise/knowledge?candidateId=%d", event.CandidateID),
		Category:         "knowledge",
		Level:            "info",
		Channels:         "in_app",
		IdempotencyKey:   firstEventID(event.EventID, fmt.Sprintf("knowledge.candidate.created:%d", event.CandidateID)),
	})
}

func handleQuotaExceededInAppNotification(ctx context.Context, event events.QuotaExceededEvent) error {
	if event.TenantID <= 0 || event.LimitValue <= 0 {
		return nil
	}
	recipients, err := services.NotificationAudienceService.ResolveTenantRecipients(event.TenantID, constants.PermissionAIConfigView.Code)
	if err != nil {
		return err
	}
	level := "warning"
	title := i18nx.NewText("notificationText.quota.approachingTitle", nil)
	if event.CurrentValue >= event.LimitValue {
		level = "urgent"
		title = i18nx.NewText("notificationText.quota.exceededTitle", nil)
	}
	return services.NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
		TenantID:         event.TenantID,
		TitleText:        title,
		ContentText:      i18nx.NewText("notificationText.quota.content", i18nx.Params{"resource": quotaResourceText(event.ResourceType), "current": event.CurrentValue, "limit": event.LimitValue}),
		NotificationType: "quota_exceeded",
		BizType:          "product",
		BizID:            event.ProductID,
		ActionURL:        "/enterprise/usage",
		Category:         "quota",
		Level:            level,
		Channels:         "in_app,email",
		IdempotencyKey:   firstEventID(event.EventID, ""),
	})
}

func handleSLAWarningInAppNotification(ctx context.Context, event events.SLAWarningEvent) error {
	ticketID, err := strconv.ParseInt(event.TicketID, 10, 64)
	if err != nil || ticketID <= 0 {
		return nil
	}
	ticket := services.TicketService.Get(ticketID)
	if ticket == nil || fmt.Sprint(ticket.TenantID) != strings.TrimSpace(event.TenantID) {
		return nil
	}
	recipients, err := services.NotificationAudienceService.ResolveTicketRecipients(ticket)
	if err != nil {
		return err
	}
	level := notificationSeverityLevel(event.Severity)
	return services.NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
		TenantID:         ticket.TenantID,
		TitleText:        i18nx.NewText("notificationText.sla.title", i18nx.Params{"ticketNo": notificationTicketNo(ticket.ID, ticket.TicketNo)}),
		ContentText:      i18nx.NewText("notificationText.sla.content", i18nx.Params{"target": slaViolationText(event.ViolationType), "targetMinutes": event.TargetMinutes, "actualMinutes": event.ActualMinutes}),
		NotificationType: "sla_warning",
		BizType:          "ticket",
		BizID:            ticket.ID,
		ActionURL:        routes.EnterpriseTicketWorkbenchPath(ticket.ID),
		Category:         "sla",
		Level:            level,
		Channels:         "in_app,email",
		// 模板变量：已批准模板按接收人语言渲染时使用，避免出现未替换的占位符。
		TemplateCode: "sla_warning",
		TemplateVariables: map[string]string{
			"TicketNo":    notificationTicketNo(ticket.ID, ticket.TicketNo),
			"TicketTitle": strings.TrimSpace(ticket.Title),
		},
		IdempotencyKey: firstEventID(event.EventID, fmt.Sprintf("sla:%d:%s:%s:%d", ticket.ID, event.ViolationType, event.Severity, event.ActualMinutes)),
	})
}

func handleSecurityAlertInAppNotification(ctx context.Context, event events.SecurityAlertEvent) error {
	tenantID, err := strconv.ParseInt(event.TenantID, 10, 64)
	if err != nil || tenantID <= 0 {
		return nil
	}
	recipients, err := services.NotificationAudienceService.ResolveTenantRecipients(tenantID, constants.PermissionSessionView.Code)
	if err != nil {
		return err
	}
	return services.NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
		TenantID:         tenantID,
		TitleText:        i18nx.NewText("notificationText.securityAlert.title", nil),
		Content:          strings.TrimSpace(event.Description),
		NotificationType: "security_alert",
		BizType:          "security",
		ActionURL:        "/enterprise/audit",
		Category:         "system",
		Level:            notificationSeverityLevel(event.Severity),
		Channels:         "in_app,email",
		IdempotencyKey:   firstEventID(event.EventID, ""),
	})
}

func notificationTicketNo(ticketID int64, ticketNo string) string {
	if value := strings.TrimSpace(ticketNo); value != "" {
		return value
	}
	return fmt.Sprintf("#%d", ticketID)
}

func quotaResourceText(resourceType string) i18nx.Text {
	switch strings.TrimSpace(resourceType) {
	case "tokens":
		return i18nx.NewText("notificationText.quota.resource.tokens", nil)
	case "requests":
		return i18nx.NewText("notificationText.quota.resource.requests", nil)
	case "cost":
		return i18nx.NewText("notificationText.quota.resource.cost", nil)
	default:
		return i18nx.NewText("notificationText.quota.resource.ai", nil)
	}
}

func slaViolationText(violationType string) i18nx.Text {
	switch strings.TrimSpace(violationType) {
	case "frt":
		return i18nx.NewText("notificationText.sla.target.firstResponse", nil)
	case "assignment":
		return i18nx.NewText("notificationText.sla.target.assignment", nil)
	case "resolution":
		return i18nx.NewText("notificationText.sla.target.resolution", nil)
	default:
		return i18nx.NewText("notificationText.sla.target.handling", nil)
	}
}

func notificationSeverityLevel(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "breach", "critical", "high":
		return "urgent"
	case "warning", "medium":
		return "warning"
	default:
		return "info"
	}
}

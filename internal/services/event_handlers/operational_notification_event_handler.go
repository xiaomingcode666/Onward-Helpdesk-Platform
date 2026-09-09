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

	deviceLabel := strings.TrimSpace(event.DeviceSerial)
	if deviceLabel == "" {
		deviceLabel = fmt.Sprintf("设备 #%d", event.DeviceID)
	}
	content := strings.TrimSpace(event.Message)
	if content == "" {
		content = fmt.Sprintf("设备 %s 上报故障码 %s。", deviceLabel, strings.TrimSpace(event.FaultCode))
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
		Title:            fmt.Sprintf("设备 %s 故障告警", deviceLabel),
		Content:          content,
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
		Title:            "视频协作已结束",
		Content:          fmt.Sprintf("工单 %s 的视频协作已结束，会议时长 %s。", notificationTicketNo(ticket.ID, ticket.TicketNo), formatMeetingDuration(event.DurationMs)),
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
	if content == "" {
		content = "新的维修知识候选已生成，请及时审核。"
	}
	return services.NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
		TenantID:         event.TenantID,
		Title:            "新的知识候选待审核",
		Content:          content,
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
	title := "AI 用量接近额度上限"
	if event.CurrentValue >= event.LimitValue {
		level = "urgent"
		title = "AI 用量已超过额度"
	}
	return services.NotificationAudienceService.Deliver(recipients, request.CreateNotificationRequest{
		TenantID:         event.TenantID,
		Title:            title,
		Content:          fmt.Sprintf("%s 当前用量 %d，额度 %d。", quotaResourceLabel(event.ResourceType), event.CurrentValue, event.LimitValue),
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
		Title:            fmt.Sprintf("工单 %s SLA 告警", notificationTicketNo(ticket.ID, ticket.TicketNo)),
		Content:          fmt.Sprintf("%s目标 %d 分钟，当前已用 %d 分钟。", slaViolationLabel(event.ViolationType), event.TargetMinutes, event.ActualMinutes),
		NotificationType: "sla_warning",
		BizType:          "ticket",
		BizID:            ticket.ID,
		ActionURL:        routes.EnterpriseTicketWorkbenchPath(ticket.ID),
		Category:         "sla",
		Level:            level,
		Channels:         "in_app,email",
		IdempotencyKey:   firstEventID(event.EventID, fmt.Sprintf("sla:%d:%s:%s:%d", ticket.ID, event.ViolationType, event.Severity, event.ActualMinutes)),
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
		Title:            "安全告警",
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

func quotaResourceLabel(resourceType string) string {
	switch strings.TrimSpace(resourceType) {
	case "tokens":
		return "Token"
	case "requests":
		return "请求次数"
	case "cost":
		return "调用费用"
	default:
		return "AI 资源"
	}
}

func slaViolationLabel(violationType string) string {
	switch strings.TrimSpace(violationType) {
	case "frt":
		return "首次响应"
	case "assignment":
		return "派工"
	case "resolution":
		return "解决时长"
	default:
		return "处理时长"
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

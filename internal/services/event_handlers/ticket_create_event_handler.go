package event_handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/common/strs"
)

func init() {
	eventbus.Register[events.TicketCreatedEvent]().Subscribe(handleTicketCreatedNotify)
	eventbus.Register[events.TicketCreatedEvent]().Subscribe(handleTicketCreatedDispatchNotify)
	eventbus.Register[events.TicketCreatedEvent]().Subscribe(handleTicketCreatedConversationEvent)
}

func handleTicketCreatedConversationEvent(ctx context.Context, event events.TicketCreatedEvent) error {
	if event.TicketID <= 0 {
		return nil
	}
	ticket := services.TicketService.Get(event.TicketID)
	if ticket == nil || ticket.ConversationID <= 0 {
		return nil
	}
	conversation := services.ConversationService.Get(ticket.ConversationID)
	if conversation == nil || conversation.Status == enums.IMConversationStatusClosed {
		return nil
	}
	payload, err := json.Marshal(map[string]any{
		"eventType":        "ticket_created",
		"source":           "ticket",
		"ticketId":         ticket.ID,
		"ticketNo":         ticket.TicketNo,
		"hasDeviceContext": ticket.DeviceID > 0,
	})
	if err != nil {
		return err
	}
	ticketNo := strs.DefaultIfBlank(ticket.TicketNo, fmt.Sprintf("#%d", ticket.ID))
	content := fmt.Sprintf("已生成服务工单 %s，会话摘要和已上传资料已同步至工单。", ticketNo)
	if ticket.DeviceID > 0 {
		content = fmt.Sprintf("已生成服务工单 %s，会话摘要、设备上下文和已上传资料已同步至工单。", ticketNo)
	}
	_, err = services.MessageService.SendSystemMessageWithRequestID(
		ticket.ConversationID,
		fmt.Sprintf("ticket_created_%d", ticket.ID),
		content,
		string(payload),
		event.EventID,
	)
	return err
}

func handleTicketCreatedNotify(ctx context.Context, event events.TicketCreatedEvent) error {
	if event.TicketID <= 0 {
		return nil
	}
	ticket := services.TicketService.Get(event.TicketID)
	if ticket == nil {
		return nil
	}
	content := buildTicketCreatedNotifyBody(ticket)
	return services.WxWorkNotifyService.SendTextToAssigneeOrDefault(ticket.CurrentAssigneeID, "工单创建提醒", content)
}

func buildTicketCreatedNotifyBody(ticket *models.Ticket) string {
	if ticket == nil {
		return ""
	}
	lines := []string{
		fmt.Sprintf("工单号: %s", strs.DefaultIfBlank(ticket.TicketNo, fmt.Sprintf("#%d", ticket.ID))),
		fmt.Sprintf("工单标题: %s", strs.DefaultIfBlank(ticket.Title, "-")),
		fmt.Sprintf("工单来源: %s", strs.DefaultIfBlank(string(ticket.Source), "-")),
		fmt.Sprintf("当前状态: %s", enums.GetTicketStatusLabel(ticket.Status)),
	}
	if ticket.CurrentAssigneeID > 0 {
		lines = append(lines, fmt.Sprintf("处理人: %s", resolveNotifyUserLabel(ticket.CurrentAssigneeID)))
	}
	lines = append(lines, fmt.Sprintf("时间: %s", time.Now().Format("2006-01-02 15:04:05")))
	return strings.Join(lines, "\n")
}

// handleTicketCreatedDispatchNotify 工单创建派单通知
func handleTicketCreatedDispatchNotify(ctx context.Context, event events.TicketCreatedEvent) error {
	if event.TicketID <= 0 {
		return nil
	}
	ticket := services.TicketService.Get(event.TicketID)
	if ticket == nil {
		return nil
	}

	// 如果工单已有处理人，不需要额外的派单通知
	if ticket.CurrentAssigneeID > 0 {
		return nil
	}

	// 通知派单（通知未指派处理人的工单需要分配）
	dispatchContent := buildTicketDispatchNotifyBody(ticket)
	return services.WxWorkNotifyService.SendTextToAssigneeOrDefault(0, "工单待派单", dispatchContent)
}

func buildTicketDispatchNotifyBody(ticket *models.Ticket) string {
	if ticket == nil {
		return ""
	}
	lines := []string{
		"【新工单待派单】",
		fmt.Sprintf("工单号: %s", strs.DefaultIfBlank(ticket.TicketNo, fmt.Sprintf("#%d", ticket.ID))),
		fmt.Sprintf("工单标题: %s", strs.DefaultIfBlank(ticket.Title, "-")),
		fmt.Sprintf("工单来源: %s", strs.DefaultIfBlank(string(ticket.Source), "-")),
	}
	if ticket.CustomerID > 0 {
		lines = append(lines, fmt.Sprintf("客户ID: %d", ticket.CustomerID))
	}
	if ticket.ProductID > 0 {
		lines = append(lines, fmt.Sprintf("产品ID: %d", ticket.ProductID))
	}
	lines = append(lines, fmt.Sprintf("创建时间: %s", time.Now().Format("2006-01-02 15:04:05")))
	return strings.Join(lines, "\n")
}

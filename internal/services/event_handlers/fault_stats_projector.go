package event_handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

// 故障统计投影（§6）：
// 业务事务提交后发布领域事件，本投影器以 event_id 幂等消费并更新
// ProductFaultStatsDaily。重新打开等场景产生补偿事件（Delta=-1），
// 而不是直接修改统计表。

func init() {
	eventbus.Register[events.TicketClosedEvent]().Subscribe(projectTicketClosedToFaultStats)
	eventbus.Register[events.TicketReopenedEvent]().Subscribe(projectTicketReopenedToFaultStats)
	eventbus.Register[events.MeetingEndedEvent]().Subscribe(projectMeetingEndedToFaultStats)
	eventbus.Register[events.DiagnosisCompletedEvent]().Subscribe(projectDiagnosisCompletedToFaultStats)
	eventbus.Register[events.CustomerRatingCreatedEvent]().Subscribe(projectCustomerRatingToFaultStats)
}

func projectTicketClosedToFaultStats(ctx context.Context, event events.TicketClosedEvent) error {
	ticket := repositories.TicketRepository.Get(sqls.DB(), event.TicketID)
	if ticket == nil || ticket.ProductID <= 0 {
		return nil
	}
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() && ticket.HandledAt != nil {
		occurredAt = *ticket.HandledAt
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	eventID := strings.TrimSpace(event.EventID)
	if eventID == "" {
		eventID = fmt.Sprintf("ticket.closed:%d:%d", ticket.ID, occurredAt.UTC().UnixNano())
	}
	return services.ProductFaultStatsService.ProjectEvent(events.ProductFaultStatsEvent{
		EventID:        eventID,
		EventType:      events.FaultStatsEventTicketClosed,
		TenantID:       ticket.TenantID,
		ProductID:      ticket.ProductID,
		ProductModelID: ticket.ProductModelID,
		DeviceID:       ticket.DeviceID,
		TicketID:       ticket.ID,
		FaultCode:      ticket.FaultCode,
		ModuleID:       ticket.ProductModuleID,
		OccurredAt:     occurredAt,
		OperatorID:     event.OperatorID,
		Delta:          1,
	})
}

func projectTicketReopenedToFaultStats(ctx context.Context, event events.TicketReopenedEvent) error {
	ticket := repositories.TicketRepository.Get(sqls.DB(), event.TicketID)
	if ticket == nil || ticket.ProductID <= 0 {
		return nil
	}
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	eventID := strings.TrimSpace(event.EventID)
	if eventID == "" {
		eventID = fmt.Sprintf("ticket.reopened:%d:%d", ticket.ID, occurredAt.UTC().UnixNano())
	}
	// 补偿：抵消关闭时累计的工单计数。
	return services.ProductFaultStatsService.ProjectEvent(events.ProductFaultStatsEvent{
		EventID:        eventID,
		EventType:      events.FaultStatsEventTicketReopened,
		TenantID:       ticket.TenantID,
		ProductID:      ticket.ProductID,
		ProductModelID: ticket.ProductModelID,
		DeviceID:       ticket.DeviceID,
		TicketID:       ticket.ID,
		FaultCode:      ticket.FaultCode,
		ModuleID:       ticket.ProductModuleID,
		OccurredAt:     occurredAt,
		OperatorID:     event.OperatorID,
		Delta:          -1,
	})
}

func projectMeetingEndedToFaultStats(ctx context.Context, event events.MeetingEndedEvent) error {
	ticketID, _ := strconv.ParseInt(strings.TrimSpace(event.TicketID), 10, 64)
	if ticketID <= 0 {
		return nil
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil || ticket.ProductID <= 0 {
		return nil
	}
	tenantID, _ := strconv.ParseInt(strings.TrimSpace(event.TenantID), 10, 64)
	if tenantID <= 0 {
		tenantID = ticket.TenantID
	}
	return services.ProductFaultStatsService.ProjectEvent(events.ProductFaultStatsEvent{
		EventID:        fmt.Sprintf("video_meeting.completed:%s", event.MeetingID),
		EventType:      events.FaultStatsEventVideoMeeting,
		TenantID:       tenantID,
		ProductID:      ticket.ProductID,
		ProductModelID: ticket.ProductModelID,
		DeviceID:       ticket.DeviceID,
		TicketID:       ticket.ID,
		FaultCode:      ticket.FaultCode,
		ModuleID:       ticket.ProductModuleID,
		OccurredAt:     time.Now(),
		Delta:          1,
	})
}

func projectDiagnosisCompletedToFaultStats(ctx context.Context, event events.DiagnosisCompletedEvent) error {
	session := repositories.DiagnosisSessionRepository.Get(sqls.DB(), event.SessionID)
	if session == nil {
		return nil
	}
	var faultCodes []string
	_ = json.Unmarshal([]byte(session.FaultCodes), &faultCodes)
	if len(faultCodes) == 0 {
		return nil // 未确认故障代码不计入故障统计
	}
	productID, _ := strconv.ParseInt(strings.TrimSpace(session.ProductID), 10, 64)
	if productID <= 0 {
		return nil
	}
	return services.ProductFaultStatsService.ProjectEvent(events.ProductFaultStatsEvent{
		EventID:    fmt.Sprintf("diagnosis.fault_confirmed:%s", session.ID),
		EventType:  events.FaultStatsEventFaultConfirmed,
		TenantID:   session.TenantID,
		ProductID:  productID,
		FaultCode:  faultCodes[0],
		OccurredAt: time.Now(),
		Delta:      1,
	})
}

func projectCustomerRatingToFaultStats(ctx context.Context, event events.CustomerRatingCreatedEvent) error {
	if event.Score > 2 { // 仅低评分计入
		return nil
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), event.TicketID)
	if ticket == nil || ticket.ProductID <= 0 {
		return nil
	}
	return services.ProductFaultStatsService.ProjectEvent(events.ProductFaultStatsEvent{
		EventID:        fmt.Sprintf("customer_rating.created:%d", ticket.ID),
		EventType:      events.FaultStatsEventCustomerRating,
		TenantID:       ticket.TenantID,
		ProductID:      ticket.ProductID,
		ProductModelID: ticket.ProductModelID,
		DeviceID:       ticket.DeviceID,
		TicketID:       ticket.ID,
		FaultCode:      ticket.FaultCode,
		OccurredAt:     time.Now(),
		OperatorID:     event.OperatorID,
		Delta:          1,
	})
}

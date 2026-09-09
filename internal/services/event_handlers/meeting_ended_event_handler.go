package event_handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	eventbus.Register[events.MeetingEndedEvent]().Subscribe(handleMeetingEndedWriteTicketTimeline)
}

// handleMeetingEndedWriteTicketTimeline 会议结束时回写工单时间线
func handleMeetingEndedWriteTicketTimeline(ctx context.Context, event events.MeetingEndedEvent) error {
	slog.Info("handling meeting ended event - writing ticket timeline",
		"meetingId", event.MeetingID,
		"ticketId", event.TicketID,
		"tenantId", event.TenantID,
		"durationMs", event.DurationMs)

	if event.TicketID == "" {
		slog.Warn("meeting ended event without ticket id, skip timeline write")
		return nil
	}

	// 查询会议详情
	var meeting models.MeetingRoomJitsi
	if err := sqls.DB().Where("id = ?", event.MeetingID).First(&meeting).Error; err != nil {
		slog.Warn("meeting not found, skip timeline write",
			"meetingId", event.MeetingID,
			"error", err)
		return nil
	}

	// 尝试将 string TicketID 转为 int64
	ticketID, parseErr := strconv.ParseInt(event.TicketID, 10, 64)
	if parseErr != nil {
		slog.Info("meeting ticket id is not numeric, skip timeline progress creation",
			"ticketId", event.TicketID)
		return nil
	}

	// 创建工单进展记录（协作纪要回写）
	durationStr := formatMeetingDuration(event.DurationMs)
	progressContent := fmt.Sprintf("视频协作已结束，时长：%s", durationStr)

	if meeting.RoomName != "" {
		progressContent = fmt.Sprintf("视频协作已结束，会议室：%s，时长：%s", meeting.RoomName, durationStr)
	}

	metadata, err := json.Marshal(map[string]any{"meetingId": meeting.ID})
	if err != nil {
		return err
	}
	var existingCount int64
	if err := sqls.DB().Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND event_type = ? AND metadata_json = ?", ticketID, enums.TicketProgressEventMeetingEnded, string(metadata)).
		Count(&existingCount).Error; err != nil {
		return err
	}
	if existingCount == 0 {
		now := time.Now()
		progress := &models.TicketProgress{
			TenantID:          meeting.TenantID,
			TicketID:          ticketID,
			EventType:         enums.TicketProgressEventMeetingEnded,
			Content:           progressContent,
			VisibleToCustomer: true,
			MetadataJSON:      string(metadata),
			CreatedAt:         now,
		}

		if err := sqls.DB().Create(progress).Error; err != nil {
			slog.Error("failed to create ticket progress for meeting end",
				"ticketId", event.TicketID,
				"meetingId", event.MeetingID,
				"error", err)
			return err
		}
	}

	ticket := services.TicketService.Get(ticketID)
	if ticket != nil && ticket.ConversationID > 0 {
		conversation := services.ConversationService.Get(ticket.ConversationID)
		if conversation != nil && conversation.Status != enums.IMConversationStatusClosed {
			payload, marshalErr := json.Marshal(map[string]any{
				"eventType": "meeting_ended",
				"source":    "jitsi_meeting",
				"ticketId":  ticket.ID,
				"meetingId": meeting.ID,
			})
			if marshalErr != nil {
				return marshalErr
			}
			if _, messageErr := services.MessageService.SendSystemMessageWithRequestID(
				ticket.ConversationID,
				"meeting_ended_"+meeting.ID,
				progressContent,
				string(payload),
				"",
			); messageErr != nil {
				return messageErr
			}
		}
	}

	slog.Info("ticket timeline updated with meeting end info",
		"ticketId", event.TicketID,
		"meetingId", event.MeetingID)

	return nil
}

// formatMeetingDuration 格式化会议时长
func formatMeetingDuration(ms int64) string {
	if ms <= 0 {
		return "不足 1 秒"
	}

	seconds := ms / 1000
	if seconds < 60 {
		return fmt.Sprintf("%d 秒", seconds)
	}

	minutes := seconds / 60
	secs := seconds % 60
	if minutes < 60 {
		if secs > 0 {
			return fmt.Sprintf("%d 分 %d 秒", minutes, secs)
		}
		return fmt.Sprintf("%d 分", minutes)
	}

	hours := minutes / 60
	mins := minutes % 60
	if mins > 0 {
		return fmt.Sprintf("%d 小时 %d 分", hours, mins)
	}
	return fmt.Sprintf("%d 小时", hours)
}

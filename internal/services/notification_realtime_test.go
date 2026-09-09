package services

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
)

func TestTicketReassignmentReconcilePublishesNotificationResync(t *testing.T) {
	previousWS := WsService
	WsService = newWsService()
	t.Cleanup(func() {
		WsService = previousWS
	})
	db := setupHumanDispatchRealtimeTestDB(t)

	oldAssigneeID := int64(12)
	newAssigneeID := int64(13)
	session := &ClientSession{
		ID:     "notification-reassign-old-assignee",
		Role:   realtimeRoleNotification,
		Topics: map[string]struct{}{},
		Send:   make(chan []byte, 8),
	}
	WsService.manager.Register(session, []string{WsService.notificationTopic(oldAssigneeID)})

	now := time.Now()
	for _, user := range []models.User{
		{ID: oldAssigneeID, Username: "old-assignee", Nickname: "旧负责人", Status: enums.StatusOk},
		{ID: newAssigneeID, Username: "new-assignee", Nickname: "新负责人", Status: enums.StatusOk},
	} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatalf("create user %d: %v", user.ID, err)
		}
	}
	ticket := &models.Ticket{
		TenantID:          1,
		TicketNo:          "TK-RESYNC-1",
		Title:             "转派实时刷新",
		Status:            enums.TicketStatusPendingAssigneeAccept,
		CurrentAssigneeID: newAssigneeID,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	stale := &models.Notification{
		TenantID:         1,
		RecipientUserID:  oldAssigneeID,
		Title:            "工单 TK-RESYNC-1 已分派",
		Content:          "请尽快接单处理",
		NotificationType: "ticket_assigned",
		BizType:          "ticket",
		BizID:            ticket.ID,
		Level:            "info",
		Category:         "ticket",
		Channels:         "in_app",
		DeliveryStatus:   "sent",
		Status:           int(enums.StatusOk),
		CreatedAt:        now,
	}
	if err := db.Create(stale).Error; err != nil {
		t.Fatalf("create stale notification: %v", err)
	}

	if err := NotificationService.ReconcileTicketAssignedAfterReassignment(ticket, oldAssigneeID, newAssigneeID); err != nil {
		t.Fatalf("reconcile reassignment notification: %v", err)
	}

	select {
	case raw := <-session.Send:
		var event struct {
			Type string `json:"type"`
			Data struct {
				Reason string `json:"reason"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &event); err != nil {
			t.Fatalf("decode realtime event: %v", err)
		}
		if event.Type != enums.IMRealtimeEventResyncRequired || event.Data.Reason != enums.IMRealtimeResyncReasonNotificationUpdated {
			t.Fatalf("unexpected realtime event: %s", raw)
		}
	case <-time.After(time.Second):
		t.Fatal("old assignee did not receive notification resync")
	}

	updated := &models.Notification{}
	if err := db.First(updated, stale.ID).Error; err != nil {
		t.Fatalf("reload notification: %v", err)
	}
	if updated.ReadAt == nil || !strings.Contains(updated.Content, "无需继续处理") {
		t.Fatalf("stale notification was not closed: %+v", updated)
	}
}

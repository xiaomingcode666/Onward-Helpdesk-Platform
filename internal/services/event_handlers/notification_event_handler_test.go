package event_handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestTicketAssignedInAppNotification(t *testing.T) {
	setupNotificationEventHandlerTestDB(t)
	seedNotificationEventRecipient(t, 1, 11, constants.PermissionTicketView.Code)

	ticket := &models.Ticket{
		TenantID:          1,
		TicketNo:          "TK202604280001",
		Title:             "退款处理",
		Source:            enums.TicketSourceManual,
		Status:            enums.TicketStatusPending,
		CurrentAssigneeID: 11,
		AuditFields: models.AuditFields{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	if err := repositories.TicketRepository.Create(sqls.DB(), ticket); err != nil {
		t.Fatalf("create ticket error = %v", err)
	}

	if err := handleTicketAssignedInAppNotification(context.Background(), events.TicketAssignedEvent{
		TicketID:   ticket.ID,
		FromUserID: 0,
		ToUserID:   11,
		OperatorID: 1,
		Reason:     "需要人工跟进",
	}); err != nil {
		t.Fatalf("handler error = %v", err)
	}
	waitForNotificationEvent(t, func() bool {
		return len(repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().Eq("recipient_user_id", 11))) == 1
	})

	list := repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().Eq("recipient_user_id", 11))
	if len(list) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(list))
	}
	got := list[0]
	if got.NotificationType != "ticket_assigned" || got.BizType != "ticket" || got.BizID != ticket.ID {
		t.Fatalf("unexpected notification: %+v", got)
	}
	if got.TenantID != 1 || got.Category != "ticket" {
		t.Fatalf("notification tenant/category mismatch: %+v", got)
	}
	if got.ActionURL != "/enterprise/ticket-workbench?ticket_id=1" {
		t.Fatalf("unexpected action url: %q", got.ActionURL)
	}
}

func TestTicketAssignedReconcilesStaleUnassignedNotification(t *testing.T) {
	setupNotificationEventHandlerTestDB(t)
	seedNotificationEventRecipient(t, 1, 12, constants.PermissionTicketView.Code)

	ticket := &models.Ticket{
		TenantID:          1,
		TicketNo:          "TK202607280002",
		Title:             "设备断电后仍无法恢复",
		Source:            enums.TicketSourceConversation,
		Status:            enums.TicketStatusAssigned,
		CurrentAssigneeID: 12,
		AuditFields: models.AuditFields{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	if err := repositories.TicketRepository.Create(sqls.DB(), ticket); err != nil {
		t.Fatalf("create ticket error = %v", err)
	}
	createdNotification := &models.Notification{
		TenantID:         1,
		RecipientUserID:  12,
		Title:            "转人工工单 TK202607280002 待处理",
		Content:          "设备断电后仍无法恢复\n当前未指派处理人，请及时派工。",
		NotificationType: "ticket_created",
		BizType:          "ticket",
		BizID:            ticket.ID,
		Level:            "warning",
		Category:         "ticket",
		Channels:         "in_app",
		DeliveryStatus:   "sent",
		Status:           int(enums.StatusOk),
		CreatedAt:        time.Now(),
	}
	if err := repositories.NotificationRepository.Create(sqls.DB(), createdNotification); err != nil {
		t.Fatalf("create stale notification error = %v", err)
	}

	if err := handleTicketAssignedInAppNotification(context.Background(), events.TicketAssignedEvent{
		TicketID: ticket.ID,
		ToUserID: 12,
		Reason:   "自动分配",
	}); err != nil {
		t.Fatalf("handler error = %v", err)
	}

	reconciled := repositories.NotificationRepository.Get(sqls.DB(), createdNotification.ID)
	if reconciled == nil {
		t.Fatal("reconciled notification not found")
	}
	if reconciled.Level != "info" || !strings.Contains(reconciled.Title, "已分配") {
		t.Fatalf("stale notification was not reconciled: %+v", reconciled)
	}
	if strings.Contains(reconciled.Content, "未指派") || !strings.Contains(reconciled.Content, "已分配给") {
		t.Fatalf("unexpected reconciled content: %q", reconciled.Content)
	}
}

func TestTicketAssignedReconcilesStaleReassignedNotification(t *testing.T) {
	setupNotificationEventHandlerTestDB(t)
	seedNotificationEventRecipient(t, 1, 12, constants.PermissionTicketView.Code)
	seedNotificationEventRecipient(t, 1, 13, constants.PermissionTicketView.Code)

	ticket := &models.Ticket{
		TenantID:          1,
		TicketNo:          "TK202607280004",
		Title:             "接单超时后重新分派",
		Source:            enums.TicketSourceConversation,
		Status:            enums.TicketStatusPendingAssigneeAccept,
		CurrentAssigneeID: 13,
		AuditFields: models.AuditFields{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	if err := repositories.TicketRepository.Create(sqls.DB(), ticket); err != nil {
		t.Fatalf("create ticket error = %v", err)
	}
	staleNotification := &models.Notification{
		TenantID:         1,
		RecipientUserID:  12,
		Title:            "工单 TK202607280004 已分派",
		Content:          "请尽快接单处理",
		NotificationType: "ticket_assigned",
		BizType:          "ticket",
		BizID:            ticket.ID,
		Level:            "info",
		Category:         "ticket",
		Channels:         "in_app",
		DeliveryStatus:   "sent",
		Status:           int(enums.StatusOk),
		CreatedAt:        time.Now(),
	}
	if err := repositories.NotificationRepository.Create(sqls.DB(), staleNotification); err != nil {
		t.Fatalf("create stale assigned notification error = %v", err)
	}

	if err := handleTicketAssignedInAppNotification(context.Background(), events.TicketAssignedEvent{
		TicketID:   ticket.ID,
		FromUserID: 12,
		ToUserID:   13,
		Reason:     "接单超时重派",
	}); err != nil {
		t.Fatalf("handler error = %v", err)
	}

	reconciled := repositories.NotificationRepository.Get(sqls.DB(), staleNotification.ID)
	if reconciled == nil {
		t.Fatal("reconciled assigned notification not found")
	}
	if reconciled.ReadAt == nil || !strings.Contains(reconciled.Title, "已转派") || !strings.Contains(reconciled.Content, "无需继续处理") {
		t.Fatalf("stale assigned notification was not closed: %+v", reconciled)
	}
	if !strings.Contains(reconciled.Content, "User 13") {
		t.Fatalf("reassigned notification did not mention the new assignee: %q", reconciled.Content)
	}
	waitForNotificationEvent(t, func() bool {
		return len(repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().
			Eq("recipient_user_id", 13).
			Eq("notification_type", "ticket_assigned"))) == 1
	})
	nextNotifications := repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("recipient_user_id", 13).
		Eq("notification_type", "ticket_assigned"))
	if len(nextNotifications) != 1 || nextNotifications[0].ReadAt != nil || nextNotifications[0].BizID != ticket.ID {
		t.Fatalf("new assignee notification mismatch: %+v", nextNotifications)
	}
	if unread := services.NotificationService.CountUnreadForTenant(1, 12); unread != 0 {
		t.Fatalf("old assignee unread count = %d, want 0", unread)
	}
}

func TestConversationAssignedInAppNotification(t *testing.T) {
	setupNotificationEventHandlerTestDB(t)
	seedNotificationEventRecipient(t, 1, 22, constants.PermissionConversationView.Code)

	conversation := &models.Conversation{
		TenantID:          1,
		CustomerName:      "张三",
		Status:            enums.IMConversationStatusActive,
		CurrentAssigneeID: 22,
		AuditFields: models.AuditFields{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	if err := repositories.ConversationRepository.Create(sqls.DB(), conversation); err != nil {
		t.Fatalf("create conversation error = %v", err)
	}

	if err := handleConversationAssignedInAppNotification(context.Background(), events.ConversationAssignedEvent{
		ConversationID: conversation.ID,
		FromUserID:     0,
		ToUserID:       22,
		OperatorID:     1,
		Reason:         "自动分配",
		AssignType:     events.ConversationAssignTypeAutoAssign,
	}); err != nil {
		t.Fatalf("handler error = %v", err)
	}
	waitForNotificationEvent(t, func() bool {
		return len(repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().Eq("recipient_user_id", 22))) == 1
	})

	list := repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().Eq("recipient_user_id", 22))
	if len(list) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(list))
	}
	got := list[0]
	if got.NotificationType != "conversation_assigned" || got.BizType != "conversation" || got.BizID != conversation.ID {
		t.Fatalf("unexpected notification: %+v", got)
	}
	if got.TenantID != 1 || got.Category != "ticket" {
		t.Fatalf("notification tenant/category mismatch: %+v", got)
	}
	if got.ActionURL != "/enterprise/ticket-workbench?conversationId=1" {
		t.Fatalf("unexpected action url: %q", got.ActionURL)
	}
	if strings.Contains(got.Content, "%!") || !strings.Contains(got.Content, "#1") {
		t.Fatalf("conversation notification content was not formatted: %q", got.Content)
	}
}

func TestMeetingEndedNotificationUsesVideoCategory(t *testing.T) {
	setupNotificationEventHandlerTestDB(t)
	seedNotificationEventRecipient(t, 1, 61, constants.PermissionTicketView.Code)
	ticket := &models.Ticket{
		TenantID: 1, TicketNo: "TK-VIDEO-1", Title: "远程协作",
		Status: enums.TicketStatusAssigned, CurrentAssigneeID: 61,
		AuditFields: models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	if err := repositories.TicketRepository.Create(sqls.DB(), ticket); err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := handleMeetingEndedInAppNotification(context.Background(), events.MeetingEndedEvent{
		MeetingID: "meeting-video-1", TicketID: fmt.Sprint(ticket.ID), TenantID: "1", DurationMs: 90_000,
	}); err != nil {
		t.Fatalf("handle meeting ended notification: %v", err)
	}
	waitForNotificationEvent(t, func() bool {
		return len(repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().Eq("notification_type", "meeting_ended"))) == 1
	})
	items := repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().Eq("notification_type", "meeting_ended"))
	if len(items) != 1 || items[0].Category != "video" || items[0].RecipientUserID != 61 {
		t.Fatalf("unexpected meeting notification: %+v", items)
	}
}

func TestKnowledgeAndQuotaEventsCreateOperationalNotifications(t *testing.T) {
	setupNotificationEventHandlerTestDB(t)
	seedNotificationEventRecipient(t, 1, 71, constants.PermissionKnowledgeBaseView.Code, constants.PermissionAIConfigView.Code)
	if err := handleKnowledgeCandidateCreatedInAppNotification(context.Background(), events.KnowledgeCandidateCreatedEvent{
		EventID: "knowledge-event-1", CandidateID: 91, TenantID: 1, Suggestion: "检查液压阀组",
	}); err != nil {
		t.Fatalf("handle knowledge notification: %v", err)
	}
	if err := handleQuotaExceededInAppNotification(context.Background(), events.QuotaExceededEvent{
		EventID: "quota-event-1", TenantID: 1, ResourceType: "requests", LimitValue: 100, CurrentValue: 101,
	}); err != nil {
		t.Fatalf("handle quota notification: %v", err)
	}
	waitForNotificationEvent(t, func() bool {
		return len(repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().Eq("tenant_id", 1))) == 2
	})
	items := repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().Eq("tenant_id", 1).Asc("id"))
	if len(items) != 2 || items[0].Category != "knowledge" || items[1].Category != "quota" || items[1].Level != "urgent" {
		t.Fatalf("unexpected operational notifications: %+v", items)
	}
}

func TestKnowledgeCandidateNotificationSkippedWhenTenantAIDisabled(t *testing.T) {
	setupNotificationEventHandlerTestDB(t)
	seedNotificationEventRecipient(t, 1, 72, constants.PermissionKnowledgeBaseView.Code)
	if err := sqls.DB().Model(&models.Tenant{}).Where("id = ?", 1).Update("ai_enabled", false).Error; err != nil {
		t.Fatalf("disable tenant AI: %v", err)
	}

	if err := handleKnowledgeCandidateCreatedInAppNotification(context.Background(), events.KnowledgeCandidateCreatedEvent{
		EventID: "knowledge-event-disabled-1", CandidateID: 92, TenantID: 1,
	}); err != nil {
		t.Fatalf("handle knowledge notification: %v", err)
	}
	if items := repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().Eq("tenant_id", 1)); len(items) != 0 {
		t.Fatalf("AI-disabled tenant must not receive knowledge candidate notification: %+v", items)
	}
}

func TestProductCreatedNotificationUsesRBACAudienceAndTenantScope(t *testing.T) {
	setupNotificationEventHandlerTestDB(t)
	seedNotificationEventRecipient(t, 1, 31, constants.PermissionProductView.Code, constants.PermissionProductCreate.Code)
	seedNotificationEventRecipient(t, 1, 32, constants.PermissionProductView.Code)

	product := &models.Product{
		TenantID: 1,
		Code:     "HP-300",
		Name:     "Hydraulic Press 300",
		Status:   enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	if err := repositories.ProductRepository.Create(sqls.DB(), product); err != nil {
		t.Fatalf("create product: %v", err)
	}
	if err := handleProductCreatedInAppNotification(context.Background(), events.ProductCreatedEvent{
		ProductID:  product.ID,
		TenantID:   product.TenantID,
		OperatorID: 31,
	}); err != nil {
		t.Fatalf("handler error = %v", err)
	}
	waitForNotificationEvent(t, func() bool {
		return len(repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().Eq("tenant_id", 1))) == 1
	})

	list := repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().Eq("tenant_id", 1))
	if len(list) != 1 || list[0].RecipientUserID != 31 {
		t.Fatalf("product notification audience = %+v, want only product manager", list)
	}
	if list[0].BizType != "product" || list[0].BizID != product.ID || list[0].ActionURL != "/enterprise/products/1" {
		t.Fatalf("unexpected product notification: %+v", list[0])
	}
}

func TestTicketCreatedNotificationRoutesToDispatchPermission(t *testing.T) {
	setupNotificationEventHandlerTestDB(t)
	seedNotificationEventRecipient(t, 1, 41, constants.PermissionTicketView.Code, constants.PermissionTicketAssign.Code)
	seedNotificationEventRecipient(t, 1, 42, constants.PermissionTicketView.Code)

	ticket := &models.Ticket{
		TenantID:       1,
		TicketNo:       "TK202607280001",
		Title:          "客户请求人工维修",
		ConversationID: 88,
		Source:         enums.TicketSourceConversation,
		Status:         enums.TicketStatusPending,
		AuditFields: models.AuditFields{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	if err := repositories.TicketRepository.Create(sqls.DB(), ticket); err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := handleTicketCreatedInAppNotification(context.Background(), events.TicketCreatedEvent{TicketID: ticket.ID}); err != nil {
		t.Fatalf("handler error = %v", err)
	}
	waitForNotificationEvent(t, func() bool {
		return len(repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().Eq("tenant_id", 1))) == 1
	})

	list := repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().Eq("tenant_id", 1))
	if len(list) != 1 || list[0].RecipientUserID != 41 {
		t.Fatalf("ticket notification audience = %+v, want only dispatcher", list)
	}
	if list[0].NotificationType != "ticket_created" || list[0].Level != "warning" || !strings.Contains(list[0].Title, "转人工工单") {
		t.Fatalf("unexpected ticket notification: %+v", list[0])
	}
}

func TestTicketClosedNotificationIsIdempotentPerRecipient(t *testing.T) {
	setupNotificationEventHandlerTestDB(t)
	seedNotificationEventRecipient(t, 1, 51, constants.PermissionTicketView.Code)
	seedNotificationEventRecipient(t, 1, 52, constants.PermissionTicketView.Code, constants.PermissionTicketAssign.Code)

	closedAt := time.Now().UTC().Truncate(time.Microsecond)
	ticket := &models.Ticket{
		TenantID: 1, TicketNo: "TK202607280003", Title: "重复关闭通知测试",
		ProductID: 7, Status: enums.TicketStatusClosed, CurrentAssigneeID: 51, HandledAt: &closedAt,
		AuditFields: models.AuditFields{CreatedAt: closedAt.Add(-time.Hour), UpdatedAt: closedAt},
	}
	if err := repositories.TicketRepository.Create(sqls.DB(), ticket); err != nil {
		t.Fatalf("create closed ticket: %v", err)
	}
	event := events.TicketClosedEvent{
		EventID: "ticket.closed:1:1722123456789000000", TicketID: ticket.ID, TenantID: ticket.TenantID,
		OperatorID: 51, Resolution: "已修复并复测通过", OccurredAt: closedAt,
	}
	if err := handleTicketClosedEvent(context.Background(), event); err != nil {
		t.Fatalf("first ticket closed handler: %v", err)
	}
	if err := handleTicketClosedEvent(context.Background(), event); err != nil {
		t.Fatalf("replayed ticket closed handler: %v", err)
	}
	waitForNotificationEvent(t, func() bool {
		return len(repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().Eq("tenant_id", 1).Eq("notification_type", "ticket_closed"))) == 2
	})
	time.Sleep(100 * time.Millisecond)

	items := repositories.NotificationRepository.Find(sqls.DB(), sqls.NewCnd().Eq("tenant_id", 1).Eq("notification_type", "ticket_closed").Asc("recipient_user_id"))
	if len(items) != 2 || items[0].RecipientUserID != 51 || items[1].RecipientUserID != 52 {
		t.Fatalf("closed notification audience after replay = %+v, want one notification per recipient", items)
	}
	for i := range items {
		wantKey := fmt.Sprintf("%s:notification:recipient:%d", event.EventID, items[i].RecipientUserID)
		if items[i].IdempotencyKey == nil || *items[i].IdempotencyKey != wantKey {
			t.Fatalf("notification idempotency key = %v, want %q", items[i].IdempotencyKey, wantKey)
		}
	}
}

func setupNotificationEventHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("open sqlite error = %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&models.User{},
		&models.Tenant{},
		&models.TenantMember{},
		&models.AuthRole{},
		&models.AuthRoleBinding{},
		&models.AuthRolePermission{},
		&models.Notification{},
		&models.Product{},
		&models.AgentTeam{},
		&models.AgentTeamMember{},
		&models.Ticket{},
		&models.Conversation{},
		&models.Message{},
	); err != nil {
		t.Fatalf("auto migrate error = %v", err)
	}
	if err := db.Create(&models.Tenant{ID: 1, Name: "notification test tenant", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create notification test tenant: %v", err)
	}
	sqls.SetDB(db)
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	queue := services.NewNotificationQueueService(redisClient, config.RedisConfig{
		NotificationStream:        fmt.Sprintf("test:notification-events:%d", time.Now().UnixNano()),
		NotificationConsumerGroup: "test-notification-event-workers",
	})
	queueContext, cancelQueue := context.WithCancel(context.Background())
	previousQueue := services.NotificationQueueService
	services.NotificationQueueService = queue
	if err := queue.Start(queueContext); err != nil {
		t.Fatalf("start notification queue: %v", err)
	}
	t.Cleanup(func() {
		cancelQueue()
		queue.Wait()
		services.NotificationQueueService = previousQueue
		_ = redisClient.Close()
	})
	return db
}

func waitForNotificationEvent(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("queued notification was not persisted")
}

func seedNotificationEventRecipient(t *testing.T, tenantID, userID int64, resourcePermissions ...string) {
	t.Helper()
	now := time.Now()
	user := &models.User{
		ID:       userID,
		Username: fmt.Sprintf("notification-event-user-%d", userID),
		Nickname: fmt.Sprintf("User %d", userID),
		Status:   enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := sqls.DB().Create(user).Error; err != nil {
		t.Fatalf("create notification event user: %v", err)
	}
	member := &models.TenantMember{
		TenantID:    tenantID,
		UserID:      userID,
		DisplayName: user.Nickname,
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := sqls.DB().Create(member).Error; err != nil {
		t.Fatalf("create notification event member: %v", err)
	}
	role := &models.AuthRole{
		TenantID:   tenantID,
		DomainType: models.DomainTypeEnterprise,
		Code:       fmt.Sprintf("notification-event-role-%d", userID),
		Name:       "Notification event role",
		Status:     enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := sqls.DB().Create(role).Error; err != nil {
		t.Fatalf("create notification event role: %v", err)
	}
	binding := &models.AuthRoleBinding{
		TenantID:    tenantID,
		DomainType:  models.DomainTypeEnterprise,
		RoleID:      role.ID,
		SubjectType: models.SubjectTypeTenantMember,
		SubjectID:   member.ID,
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := sqls.DB().Create(binding).Error; err != nil {
		t.Fatalf("create notification event role binding: %v", err)
	}
	permissions := append([]string{constants.PermissionNotificationView.Code}, resourcePermissions...)
	for _, permission := range permissions {
		row := &models.AuthRolePermission{
			TenantID:       tenantID,
			RoleID:         role.ID,
			PermissionCode: permission,
			Effect:         "allow",
			Status:         enums.StatusOk,
			AuditFields: models.AuditFields{
				CreatedAt: now,
				UpdatedAt: now,
			},
		}
		if err := sqls.DB().Create(row).Error; err != nil {
			t.Fatalf("create notification event permission: %v", err)
		}
	}
}

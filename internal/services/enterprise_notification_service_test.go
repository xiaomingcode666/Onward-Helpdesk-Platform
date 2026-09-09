package services

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestEnterpriseNotificationTenantWideVisibilityKeepsPersonalReadBoundary(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Notification{}); err != nil {
		t.Fatalf("migrate notifications: %v", err)
	}
	sqls.SetDB(db)
	now := time.Now()
	items := []models.Notification{
		{TenantID: 1, EventKey: "owner-event", RecipientUserID: 11, RecipientName: "Owner", Title: "owner message", Content: "会话 #%!s(int64=248) 已分配给你", NotificationType: "conversation_assigned", DeliveryStatus: "sent", Status: int(enums.StatusOk), CreatedAt: now},
		{TenantID: 1, EventKey: "engineer-event", RecipientUserID: 12, RecipientName: "Engineer", Title: "engineer message", DeliveryStatus: "sent", Status: int(enums.StatusOk), CreatedAt: now.Add(-time.Second)},
		{TenantID: 2, RecipientUserID: 11, RecipientName: "Other tenant", Title: "cross tenant", Status: int(enums.StatusOk), CreatedAt: now},
	}
	if err := db.Create(&items).Error; err != nil {
		t.Fatalf("seed notifications: %v", err)
	}

	owner := &dto.AuthPrincipal{UserID: 11, TenantID: 1, DomainType: models.DomainTypeEnterprise, Roles: []string{EnterpriseRoleOwner}}
	result, err := EnterpriseNotificationService.ListForOperator(1, owner, EnterpriseNotificationQuery{Limit: 20})
	if err != nil {
		t.Fatalf("list owner notifications: %v", err)
	}
	if len(result.Items) != 1 || result.Summary.Total != 1 || result.Summary.Unread != 1 || result.Summary.MarkableUnread != 1 {
		t.Fatalf("unexpected owner result: %+v", result)
	}
	if result.Scope != "personal" || !result.CanViewTenantScope || result.Total != 1 || result.Page != 1 || result.PageSize != 20 || result.HasMore {
		t.Fatalf("unexpected owner scope/page metadata: %+v", result)
	}
	if !result.Items[0].CanMarkRead {
		t.Fatalf("owner mark-read boundaries are wrong: %+v", result.Items)
	}
	if strings.Contains(result.Items[0].Content, "%!") || !strings.Contains(result.Items[0].Content, "#248") {
		t.Fatalf("legacy notification content was not normalized: %q", result.Items[0].Content)
	}
	firstPage, err := EnterpriseNotificationService.ListForOperator(1, owner, EnterpriseNotificationQuery{Scope: "tenant", Page: 1, PageSize: 1})
	if err != nil {
		t.Fatalf("list owner notification page: %v", err)
	}
	if len(firstPage.Items) != 1 || firstPage.Total != 2 || !firstPage.HasMore || !firstPage.Items[0].IsEventSummary || firstPage.Items[0].CanMarkRead {
		t.Fatalf("owner notification pagination mismatch: %+v", firstPage)
	}
	ownerPersonal, err := EnterpriseNotificationService.ListForOperator(1, owner, EnterpriseNotificationQuery{Scope: "personal", PageSize: 20})
	if err != nil {
		t.Fatalf("list owner personal notifications: %v", err)
	}
	if ownerPersonal.Scope != "personal" || len(ownerPersonal.Items) != 1 || ownerPersonal.Items[0].RecipientUserID != owner.UserID {
		t.Fatalf("owner personal scope mismatch: %+v", ownerPersonal)
	}
	if err := EnterpriseNotificationService.MarkRead(1, owner.UserID, items[1].ID); err == nil {
		t.Fatal("owner must not mark another member's notification read")
	}
	if err := EnterpriseNotificationService.MarkAllRead(1, owner.UserID); err != nil {
		t.Fatalf("mark owner's notifications read: %v", err)
	}
	if got := repositoriesNotificationForTest(t, db, items[0].ID); got.ReadAt == nil {
		t.Fatal("owner's notification was not marked read")
	} else if got.DeliveryStatus != "sent" {
		t.Fatalf("mark read changed delivery status to %q", got.DeliveryStatus)
	}
	if got := repositoriesNotificationForTest(t, db, items[1].ID); got.ReadAt != nil {
		t.Fatal("another member's notification was changed")
	}

	engineer := &dto.AuthPrincipal{UserID: 12, TenantID: 1, DomainType: models.DomainTypeEnterprise, Roles: []string{EnterpriseRoleEngineer}}
	personal, err := EnterpriseNotificationService.ListForOperator(1, engineer, EnterpriseNotificationQuery{Limit: 20})
	if err != nil {
		t.Fatalf("list engineer notifications: %v", err)
	}
	if len(personal.Items) != 1 || personal.Items[0].RecipientUserID != 12 {
		t.Fatalf("engineer visibility leaked: %+v", personal.Items)
	}

	support := &dto.AuthPrincipal{
		UserID: 99, DomainType: models.DomainTypeEnterprise, TargetTenantID: 1,
		SupportGrantID: 85, ImpersonatedBy: "admin",
	}
	assisted, err := EnterpriseNotificationService.ListForOperator(1, support, EnterpriseNotificationQuery{Scope: "tenant", Limit: 20})
	if err != nil {
		t.Fatalf("list assisted notifications: %v", err)
	}
	if len(assisted.Items) != 2 || assisted.Summary.MarkableUnread != 0 {
		t.Fatalf("assisted visibility/read boundary mismatch: %+v", assisted)
	}
	for _, item := range assisted.Items {
		if item.CanMarkRead {
			t.Fatalf("support operator can mark tenant member notification: %+v", item)
		}
	}

	if got := deriveEmailStatus(models.Notification{Channels: "in_app,email", DeliveryStatus: "sent"}, []string{"in_app", "email"}); got != "pending" {
		t.Fatalf("in-app sent status must not be treated as email delivery: %q", got)
	}
	if got := deriveEmailStatus(models.Notification{Channels: "in_app,email", ExternalChannelStatus: "email_failed"}, []string{"in_app", "email"}); got != "failed" {
		t.Fatalf("email failure status mismatch: %q", got)
	}
}

func repositoriesNotificationForTest(t *testing.T, db *gorm.DB, id int64) models.Notification {
	t.Helper()
	var item models.Notification
	if err := db.First(&item, id).Error; err != nil {
		t.Fatalf("load notification %d: %v", id, err)
	}
	return item
}

func TestEnterpriseNotificationTenantViewGroupsRecipientDeliveriesByEvent(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Notification{}); err != nil {
		t.Fatalf("migrate notifications: %v", err)
	}
	sqls.SetDB(db)
	now := time.Now()
	readAt := now.Add(-time.Minute)
	items := []models.Notification{
		{TenantID: 7, EventKey: "ticket.closed:42", RecipientUserID: 71, Title: "工单已关闭", Category: "ticket", Level: "urgent", Channels: "in_app,email", ExternalChannelStatus: "email_pending", Status: int(enums.StatusOk), CreatedAt: now},
		{TenantID: 7, EventKey: "ticket.closed:42", RecipientUserID: 72, Title: "工单已关闭", Category: "ticket", Level: "urgent", Channels: "in_app,email", ExternalChannelStatus: "email_failed", Status: int(enums.StatusOk), CreatedAt: now.Add(-time.Second)},
		{TenantID: 7, EventKey: "ticket.closed:42", RecipientUserID: 73, Title: "工单已关闭", Category: "ticket", Level: "urgent", Channels: "in_app,email", ExternalChannelStatus: "email_sent", ReadAt: &readAt, Status: int(enums.StatusOk), CreatedAt: now.Add(-2 * time.Second)},
	}
	if err := db.Create(&items).Error; err != nil {
		t.Fatalf("seed notifications: %v", err)
	}
	owner := &dto.AuthPrincipal{UserID: 71, TenantID: 7, DomainType: models.DomainTypeEnterprise, Roles: []string{EnterpriseRoleOwner}}
	result, err := EnterpriseNotificationService.ListForOperator(7, owner, EnterpriseNotificationQuery{Scope: "tenant", PageSize: 20})
	if err != nil {
		t.Fatalf("list tenant events: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Summary.Total != 1 || result.Summary.Unread != 1 {
		t.Fatalf("unexpected grouped event result: %+v", result)
	}
	if result.Summary.DeliveryTotal != 3 || result.Summary.DeliveryUnread != 2 || result.Summary.Urgent != 1 {
		t.Fatalf("unexpected grouped summary: %+v", result.Summary)
	}
	item := result.Items[0]
	if !item.IsEventSummary || item.RecipientCount != 3 || item.UnreadCount != 2 || item.EmailPendingCount != 1 || item.EmailFailedCount != 1 {
		t.Fatalf("unexpected event aggregate: %+v", item)
	}
	readResult, err := EnterpriseNotificationService.ListForOperator(7, owner, EnterpriseNotificationQuery{Scope: "tenant", ReadStatus: "read", PageSize: 20})
	if err != nil {
		t.Fatalf("list fully read events: %v", err)
	}
	if readResult.Total != 0 || len(readResult.Items) != 0 {
		t.Fatalf("event with unread deliveries appeared as fully read: %+v", readResult)
	}
}

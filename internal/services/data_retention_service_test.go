package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDataRetentionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := "retention_" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(
		&models.DataRegionPolicy{},
		&models.AuditLog{},
		&models.Conversation{},
		&models.ConversationEventLog{},
		&models.Notification{},
		&models.LoginCredentialLog{},
		&models.CustomerPrivacyConsent{},
	); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	sqls.SetDB(db)
	return db
}

func TestCleanupExpiredDataPreservesTenantIsolation(t *testing.T) {
	db := setupDataRetentionTestDB(t)
	now := time.Now()
	policies := []models.DataRegionPolicy{
		{ID: "policy-1", TenantID: "1", DataRegion: "global", RetentionDays: 30, ArchiveAfterDays: 10, AutoDeleteEnabled: true, Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "policy-2", TenantID: "2", DataRegion: "global", RetentionDays: 30, ArchiveAfterDays: 10, AutoDeleteEnabled: false, Status: "active", CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&policies).Error; err != nil {
		t.Fatalf("create policies: %v", err)
	}

	audits := []models.AuditLog{
		{TenantID: 1, Action: "expired", CreatedAt: now.AddDate(0, 0, -40)},
		{TenantID: 1, Action: "archive", CreatedAt: now.AddDate(0, 0, -20)},
		{TenantID: 1, Action: "current", CreatedAt: now.AddDate(0, 0, -2)},
		{TenantID: 2, Action: "other-tenant", CreatedAt: now.AddDate(0, 0, -40)},
	}
	if err := db.Create(&audits).Error; err != nil {
		t.Fatalf("create audit logs: %v", err)
	}
	conversations := []models.Conversation{{TenantID: 1, Status: enums.IMConversationStatusActive}, {TenantID: 2, Status: enums.IMConversationStatusActive}}
	if err := db.Create(&conversations).Error; err != nil {
		t.Fatalf("create conversations: %v", err)
	}
	events := []models.ConversationEventLog{
		{ConversationID: conversations[0].ID, EventType: enums.IMEventType("message.sent"), CreatedAt: now.AddDate(0, 0, -20)},
		{ConversationID: conversations[1].ID, EventType: enums.IMEventType("message.sent"), CreatedAt: now.AddDate(0, 0, -20)},
	}
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("create conversation event logs: %v", err)
	}
	loginLog := models.LoginCredentialLog{Principal: "user", CreatedAt: now.AddDate(0, 0, -400)}
	if err := db.Create(&loginLog).Error; err != nil {
		t.Fatalf("create login log: %v", err)
	}
	privacyConsents := []models.CustomerPrivacyConsent{
		{TenantID: 1, EntrySessionID: 1, VisitorID: "expired", PolicyVersion: "v1", RequiredAccepted: true, ReceiptHash: "expired", ConsentedAt: now.AddDate(0, 0, -40), CreatedAt: now.AddDate(0, 0, -40)},
		{TenantID: 1, EntrySessionID: 2, VisitorID: "current", PolicyVersion: "v1", RequiredAccepted: true, ReceiptHash: "current", ConsentedAt: now.AddDate(0, 0, -2), CreatedAt: now.AddDate(0, 0, -2)},
		{TenantID: 2, EntrySessionID: 3, VisitorID: "other", PolicyVersion: "v1", RequiredAccepted: true, ReceiptHash: "other", ConsentedAt: now.AddDate(0, 0, -40), CreatedAt: now.AddDate(0, 0, -40)},
	}
	if err := db.Create(&privacyConsents).Error; err != nil {
		t.Fatalf("create privacy consents: %v", err)
	}

	if err := DataRetentionService.CleanupExpiredData(context.Background()); err != nil {
		t.Fatalf("CleanupExpiredData() error = %v", err)
	}

	var remainingTenant1, remainingTenant2 int64
	db.Model(&models.AuditLog{}).Where("tenant_id = ?", 1).Count(&remainingTenant1)
	db.Model(&models.AuditLog{}).Where("tenant_id = ?", 2).Count(&remainingTenant2)
	if remainingTenant1 != 1 || remainingTenant2 != 1 {
		t.Fatalf("remaining audits tenant1=%d tenant2=%d, want 1 and 1", remainingTenant1, remainingTenant2)
	}
	var archivedTenant1, archivedTenant2 int64
	db.Table("audit_logs_archive").Where("tenant_id = ?", 1).Count(&archivedTenant1)
	db.Table("audit_logs_archive").Where("tenant_id = ?", 2).Count(&archivedTenant2)
	if archivedTenant1 != 1 || archivedTenant2 != 0 {
		t.Fatalf("archived audits tenant1=%d tenant2=%d, want 1 and 0", archivedTenant1, archivedTenant2)
	}
	var eventTenant1, eventTenant2 int64
	db.Model(&models.ConversationEventLog{}).Where("conversation_id = ?", conversations[0].ID).Count(&eventTenant1)
	db.Model(&models.ConversationEventLog{}).Where("conversation_id = ?", conversations[1].ID).Count(&eventTenant2)
	if eventTenant1 != 0 || eventTenant2 != 1 {
		t.Fatalf("remaining events tenant1=%d tenant2=%d, want 0 and 1", eventTenant1, eventTenant2)
	}
	var loginCount int64
	db.Model(&models.LoginCredentialLog{}).Count(&loginCount)
	if loginCount != 1 {
		t.Fatalf("login credential logs were deleted by tenant retention: count=%d", loginCount)
	}
	var tenant1ConsentCount, tenant2ConsentCount int64
	db.Model(&models.CustomerPrivacyConsent{}).Where("tenant_id = ?", 1).Count(&tenant1ConsentCount)
	db.Model(&models.CustomerPrivacyConsent{}).Where("tenant_id = ?", 2).Count(&tenant2ConsentCount)
	if tenant1ConsentCount != 1 || tenant2ConsentCount != 1 {
		t.Fatalf("privacy consent retention crossed tenant boundary: tenant1=%d tenant2=%d", tenant1ConsentCount, tenant2ConsentCount)
	}
}

func TestArchiveOldDataRejectsUnapprovedTable(t *testing.T) {
	setupDataRetentionTestDB(t)
	err := DataRetentionService.ArchiveOldData(context.Background(), "users", "1", time.Now())
	if err == nil {
		t.Fatal("ArchiveOldData() expected an error for an unapproved table")
	}
}

func TestCheckDSARDeadlinesIsIdempotent(t *testing.T) {
	db := setupDataRetentionTestDB(t)
	if err := db.AutoMigrate(&models.DSARRequest{}, &models.DSARExecutionLog{}, &models.AuthRole{}, &models.AuthRoleBinding{}, &models.TenantMember{}); err != nil {
		t.Fatalf("migrate DSAR tables: %v", err)
	}
	now := time.Now()
	request := models.DSARRequest{ID: "dsar-overdue", TenantID: "1", SubjectType: "customer", SubjectID: "1", RequestType: "delete", Status: "pending", DeadlineAt: now.Add(-time.Hour), RequestedAt: now.AddDate(0, 0, -30)}
	if err := db.Create(&request).Error; err != nil {
		t.Fatalf("create DSAR request: %v", err)
	}
	if err := DSARService.CheckDSARDeadlines(); err != nil {
		t.Fatalf("first CheckDSARDeadlines() error = %v", err)
	}
	if err := DSARService.CheckDSARDeadlines(); err != nil {
		t.Fatalf("second CheckDSARDeadlines() error = %v", err)
	}
	var count int64
	db.Model(&models.DSARExecutionLog{}).Where("dsar_id = ? AND action = ?", request.ID, "compliance_breach").Count(&count)
	if count != 1 {
		t.Fatalf("compliance breach log count=%d, want 1", count)
	}
}

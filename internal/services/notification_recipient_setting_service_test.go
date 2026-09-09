package services

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestNotificationRecipientSettingUsesProfileAndOverride(t *testing.T) {
	db := setupNotificationRecipientSettingTestDB(t)
	profileEmail := "person@example.com"
	seedNotificationRecipientSettingUser(t, db, 7, 701, &profileEmail)

	initial, err := NotificationRecipientSettingService.Get(7, 701)
	if err != nil {
		t.Fatalf("get default recipient setting: %v", err)
	}
	if !initial.UseProfileEmail || initial.EffectiveEmail != profileEmail || !initial.EmailEnabled {
		t.Fatalf("unexpected default recipient setting: %+v", initial)
	}

	updated, err := NotificationRecipientSettingService.Save(7, 701, request.UpdateNotificationRecipientSettingRequest{
		Email: "alerts@example.com", EmailEnabled: true,
	})
	if err != nil {
		t.Fatalf("save recipient override: %v", err)
	}
	if updated.UseProfileEmail || updated.EffectiveEmail != "alerts@example.com" {
		t.Fatalf("unexpected override setting: %+v", updated)
	}
	if got := NotificationRecipientSettingService.ResolveEmail(7, 701); got != "alerts@example.com" {
		t.Fatalf("resolved email = %q", got)
	}

	disabled, err := NotificationRecipientSettingService.Save(7, 701, request.UpdateNotificationRecipientSettingRequest{
		UseProfileEmail: true, EmailEnabled: false,
	})
	if err != nil {
		t.Fatalf("disable recipient email: %v", err)
	}
	if !disabled.UseProfileEmail || disabled.EmailEnabled {
		t.Fatalf("unexpected disabled setting: %+v", disabled)
	}
	if got := NotificationRecipientSettingService.ResolveEmail(7, 701); got != "" {
		t.Fatalf("disabled email resolved to %q", got)
	}
}

func TestNotificationRecipientSettingRejectsCrossTenantAndInvalidEmail(t *testing.T) {
	db := setupNotificationRecipientSettingTestDB(t)
	profileEmail := "person@example.com"
	seedNotificationRecipientSettingUser(t, db, 8, 801, &profileEmail)

	if _, err := NotificationRecipientSettingService.Get(9, 801); err == nil {
		t.Fatal("cross-tenant recipient setting should be rejected")
	}
	if _, err := NotificationRecipientSettingService.Save(8, 801, request.UpdateNotificationRecipientSettingRequest{
		Email: "not-an-email", EmailEnabled: true,
	}); err == nil {
		t.Fatal("invalid recipient email should be rejected")
	}
}

func setupNotificationRecipientSettingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.TenantMember{}, &models.NotificationRecipientSetting{}); err != nil {
		t.Fatalf("migrate recipient settings: %v", err)
	}
	sqls.SetDB(db)
	return db
}

func seedNotificationRecipientSettingUser(t *testing.T, db *gorm.DB, tenantID, userID int64, email *string) {
	t.Helper()
	now := time.Now()
	if err := db.Create(&models.User{
		ID: userID, Username: fmt.Sprintf("recipient-user-%d", userID), Nickname: "Recipient", Email: email, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create recipient user: %v", err)
	}
	if err := db.Create(&models.TenantMember{
		TenantID: tenantID, UserID: userID, DisplayName: "Recipient", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create recipient member: %v", err)
	}
}

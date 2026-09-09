package migration

import (
	"fmt"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestHardenNotificationWorkflowGroupsEventsAndExpandsBroadcasts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Notification{}, &models.TenantMember{}, &models.TenantMailSetting{}, &models.DeliveryLog{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	now := time.Now()
	key := "ticket.closed:42:notification:recipient:11"
	personal := models.Notification{
		TenantID: 1, IdempotencyKey: &key, RecipientUserID: 11, Title: "ticket closed", Channels: "in_app,email", Status: int(enums.StatusOk), CreatedAt: now,
	}
	broadcast := models.Notification{
		TenantID: 1, RecipientUserID: 0, RecipientName: "服务团队", Title: "broadcast", Channels: "in_app", Status: int(enums.StatusOk), CreatedAt: now,
	}
	if err := db.Create(&personal).Error; err != nil {
		t.Fatalf("create personal notification: %v", err)
	}
	if err := db.Create(&broadcast).Error; err != nil {
		t.Fatalf("create broadcast notification: %v", err)
	}
	if err := db.Create(&models.TenantMember{
		TenantID: 1, UserID: 11, DisplayName: "Owner", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	if err := db.Create(&models.TenantMailSetting{
		TenantID: 1, FromAddress: "service-notice@hdjg.com", FromName: "设备售后平台通知", SMTPHost: "smtp.enterprise-mail.com", SMTPPort: 465,
		ReplyTo: "support@hdjg.com", RetryPolicy: "retry_3_10m", UseTLS: true, Status: int(enums.StatusOk), CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create prototype mail setting: %v", err)
	}
	log := models.DeliveryLog{NotificationID: personal.ID, Channel: "email", RecipientID: "owner@example.com", Status: "sent", CreatedAt: now}
	if err := db.Create(&log).Error; err != nil {
		t.Fatalf("create delivery log: %v", err)
	}

	if err := hardenNotificationWorkflow(db); err != nil {
		t.Fatalf("harden notification workflow: %v", err)
	}
	var updatedPersonal models.Notification
	if err := db.First(&updatedPersonal, personal.ID).Error; err != nil {
		t.Fatalf("load personal notification: %v", err)
	}
	if updatedPersonal.EventKey != "ticket.closed:42:notification" || updatedPersonal.ExternalChannelStatus != "email_skipped" {
		t.Fatalf("unexpected personal backfill: %+v", updatedPersonal)
	}
	var updatedBroadcast models.Notification
	if err := db.First(&updatedBroadcast, broadcast.ID).Error; err != nil {
		t.Fatalf("load broadcast notification: %v", err)
	}
	if updatedBroadcast.Status != int(enums.StatusDisabled) {
		t.Fatalf("broadcast status = %d, want disabled", updatedBroadcast.Status)
	}
	var copies []models.Notification
	if err := db.Where("event_key = ?", "legacy:broadcast:"+formatMigrationInt64(broadcast.ID)).Find(&copies).Error; err != nil {
		t.Fatalf("load broadcast copies: %v", err)
	}
	if len(copies) != 1 || copies[0].RecipientUserID != 11 || copies[0].RecipientName != "Owner" {
		t.Fatalf("unexpected broadcast copies: %+v", copies)
	}
	var setting models.TenantMailSetting
	if err := db.First(&setting, "tenant_id = ?", 1).Error; err != nil {
		t.Fatalf("load mail setting: %v", err)
	}
	if setting.Status != int(enums.StatusDisabled) || setting.SMTPHost != "" || setting.FromAddress != "" {
		t.Fatalf("prototype mail setting remains active: %+v", setting)
	}
	var updatedLog models.DeliveryLog
	if err := db.First(&updatedLog, log.ID).Error; err != nil {
		t.Fatalf("load delivery log: %v", err)
	}
	if updatedLog.TenantID != 1 || updatedLog.UpdatedAt.IsZero() {
		t.Fatalf("delivery scope was not backfilled: %+v", updatedLog)
	}
}

func formatMigrationInt64(value int64) string {
	return fmt.Sprintf("%d", value)
}

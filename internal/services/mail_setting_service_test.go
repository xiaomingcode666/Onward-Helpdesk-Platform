package services

import (
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/secretstore"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestMailSettingEncryptsPasswordAndNeverReturnsIt(t *testing.T) {
	previous := config.CurrentOrDefault()
	t.Cleanup(func() { config.SetCurrent(&previous) })
	config.SetCurrent(&config.Config{EncryptionKey: "mail-setting-encryption-key-0123456789"})

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.TenantMailSetting{}); err != nil {
		t.Fatalf("migrate mail settings: %v", err)
	}
	sqls.SetDB(db)

	result, err := MailSettingService.Save(1, request.UpdateMailSettingRequest{
		FromAddress: "notice@example.com", FromName: "Service", SMTPHost: "smtp.example.com", SMTPPort: 465,
		Username: "notice@example.com", Password: "application-password", ReplyTo: "support@example.com",
		RetryPolicy: "retry_3_10m", UseTLS: true,
	})
	if err != nil {
		t.Fatalf("save mail setting: %v", err)
	}
	if !result.HasPassword || result.Username != "notice@example.com" {
		t.Fatalf("unexpected mail setting response: %+v", result)
	}

	var stored models.TenantMailSetting
	if err := db.Where("tenant_id = ?", 1).First(&stored).Error; err != nil {
		t.Fatalf("load stored mail setting: %v", err)
	}
	if stored.Password == "application-password" || !strings.HasPrefix(stored.Password, "enc:v1:") {
		t.Fatalf("mail password was not encrypted: %q", stored.Password)
	}
	plain, err := secretstore.Decrypt(stored.Password)
	if err != nil || plain != "application-password" {
		t.Fatalf("decrypt stored password = %q, %v", plain, err)
	}
}

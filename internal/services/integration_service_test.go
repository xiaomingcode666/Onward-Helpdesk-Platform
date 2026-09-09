package services

import (
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
)

func TestIntegrationCredentialResponseMasksSecretsAndMeetingPreview(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&models.Tenant{}, &models.Product{}, &models.Department{}, &models.AgentTeam{}, &models.ProductServiceProfile{}, &models.TenantIntegrationConfig{}, &models.ProductAIUsageCredential{}, &models.MeetingRoom{}, &models.DomainEvent{}, &models.OutboxRecord{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	tenant := models.Tenant{Name: "test", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatal(err)
	}
	op := &dto.AuthPrincipal{UserID: 1, Username: "admin", TenantID: tenant.ID, Status: enums.StatusOk}
	p := &models.Product{TenantID: tenant.ID, Code: "p1", Name: "Product", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = ProductServiceProfileService.CreateProductServiceProfile(request.CreateProductServiceProfileRequest{TenantID: tenant.ID, ProductID: p.ID, MeetingEnabled: true}, op); err != nil {
		t.Fatal(err)
	}
	config, err := TenantIntegrationConfigService.Create(request.CreateTenantIntegrationConfigRequest{TenantID: tenant.ID, Provider: "jitsi", BaseURL: "https://meet.example.test", AppSecret: "secret-value", Enabled: true}, op)
	if err != nil {
		t.Fatal(err)
	}
	cred, err := ProductAIUsageCredentialService.Create(request.CreateProductAIUsageCredentialRequest{TenantID: tenant.ID, ProductID: p.ID, APIKey: "sub2api-key"}, op)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(config.AppSecretRef, "secret-value") || strings.Contains(cred.APIKeyRef, "sub2api-key") {
		t.Fatal("secret refs must not contain raw secrets")
	}
	if config.AppSecretFingerprint == "" || cred.APIKeyFingerprint == "" {
		t.Fatal("secret fingerprints were not generated")
	}
	preview, err := MeetingService.Preview(request.MeetingPreviewCreateRequest{TenantID: tenant.ID, ProductID: p.ID, BusinessType: "ticket", BusinessID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(preview.JoinURL, "https://meet.example.test/") {
		t.Fatalf("join url=%q", preview.JoinURL)
	}
	if strings.Contains(preview.RoomName, "ticket") || strings.Contains(preview.RoomName, "-7") {
		t.Fatalf("room name should not expose business identifiers: %q", preview.RoomName)
	}
	var room models.MeetingRoom
	if err := db.First(&room, "room_name = ?", preview.RoomName).Error; err != nil {
		t.Fatalf("meeting room was not persisted: %v", err)
	}
}

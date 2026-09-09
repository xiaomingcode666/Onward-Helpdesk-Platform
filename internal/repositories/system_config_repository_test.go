package repositories

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestSystemConfigSaveByKeyPreservesCreationAuditAndWritesZeroStatus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SystemConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	createdAt := time.Now().Add(-time.Hour).Truncate(time.Millisecond)
	existing := &models.SystemConfig{
		ConfigKey: "test.key", ConfigValue: "before", Status: enums.StatusDeleted,
		AuditFields: models.AuditFields{
			CreatedAt: createdAt, UpdatedAt: createdAt, CreateUserID: 11, CreateUserName: "creator",
			UpdateUserID: 11, UpdateUserName: "creator",
		},
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("create config: %v", err)
	}

	updatedAt := createdAt.Add(30 * time.Minute)
	item := &models.SystemConfig{
		ConfigKey: "test.key", ConfigValue: "after", GroupCode: "runtime", Status: enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: updatedAt, UpdatedAt: updatedAt, CreateUserID: 22, CreateUserName: "updater",
			UpdateUserID: 22, UpdateUserName: "updater",
		},
	}
	if err := SystemConfigRepository.SaveByKey(db, item); err != nil {
		t.Fatalf("save by key: %v", err)
	}

	stored := SystemConfigRepository.FindByKey(db, "test.key")
	if stored == nil {
		t.Fatal("updated config not found")
	}
	if stored.ConfigValue != "after" || stored.Status != enums.StatusOk || stored.GroupCode != "runtime" {
		t.Fatalf("updated config = %+v", stored)
	}
	if !stored.CreatedAt.Equal(createdAt) || stored.CreateUserID != 11 || stored.CreateUserName != "creator" {
		t.Fatalf("creation audit was overwritten: %+v", stored.AuditFields)
	}
	if !stored.UpdatedAt.Equal(updatedAt) || stored.UpdateUserID != 22 || stored.UpdateUserName != "updater" {
		t.Fatalf("update audit was not applied: %+v", stored.AuditFields)
	}
}

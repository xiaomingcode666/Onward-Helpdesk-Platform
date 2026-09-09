package repositories

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMobilePushTokenRepositoryUpsertAndScopedRevoke(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.MobilePushToken{}); err != nil {
		t.Fatalf("AutoMigrate(): %v", err)
	}

	baseTime := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	item := &models.MobilePushToken{
		TenantID:         11,
		UserID:           22,
		Platform:         "android",
		TokenFingerprint: "fingerprint",
		TokenCiphertext:  "ciphertext-v1",
		DeviceID:         "device-1",
		AppVersion:       "1.0.0",
		LastSeenAt:       baseTime,
		CreatedAt:        baseTime,
		UpdatedAt:        baseTime,
	}
	if err := MobilePushTokenRepository.Upsert(db, item); err != nil {
		t.Fatalf("first Upsert(): %v", err)
	}
	if item.ID <= 0 {
		t.Fatal("first Upsert() did not return the persisted ID")
	}

	revokedAt := baseTime.Add(time.Minute)
	if err := MobilePushTokenRepository.Revoke(db, 11, 22, item.ID, revokedAt); err != nil {
		t.Fatalf("Revoke(): %v", err)
	}

	updatedTime := baseTime.Add(2 * time.Minute)
	replacement := &models.MobilePushToken{
		TenantID:         11,
		UserID:           22,
		Platform:         "android",
		TokenFingerprint: "fingerprint",
		TokenCiphertext:  "ciphertext-v2",
		DeviceID:         "device-1",
		AppVersion:       "1.1.0",
		LastSeenAt:       updatedTime,
		CreatedAt:        updatedTime,
		UpdatedAt:        updatedTime,
	}
	if err := MobilePushTokenRepository.Upsert(db, replacement); err != nil {
		t.Fatalf("second Upsert(): %v", err)
	}
	if replacement.ID != item.ID {
		t.Fatalf("second Upsert() ID = %d, want %d", replacement.ID, item.ID)
	}

	var rows []models.MobilePushToken
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("Find(): %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	if rows[0].TokenCiphertext != "ciphertext-v2" || rows[0].AppVersion != "1.1.0" {
		t.Fatalf("upserted row was not refreshed: %+v", rows[0])
	}
	if rows[0].RevokedAt != nil {
		t.Fatalf("upserted row remained revoked at %v", rows[0].RevokedAt)
	}
	active, err := MobilePushTokenRepository.FindActiveByUser(db, 11, 22)
	if err != nil || len(active) != 1 || active[0].ID != item.ID {
		t.Fatalf("FindActiveByUser() = %+v, %v", active, err)
	}
	otherTenant, err := MobilePushTokenRepository.FindActiveByUser(db, 99, 22)
	if err != nil || len(otherTenant) != 0 {
		t.Fatalf("cross-tenant FindActiveByUser() = %+v, %v", otherTenant, err)
	}

	if err := MobilePushTokenRepository.Revoke(db, 99, 22, item.ID, updatedTime); err != nil {
		t.Fatalf("wrong-tenant Revoke(): %v", err)
	}
	var afterWrongTenant models.MobilePushToken
	if err := db.First(&afterWrongTenant, item.ID).Error; err != nil {
		t.Fatalf("reload after wrong-tenant Revoke(): %v", err)
	}
	if afterWrongTenant.RevokedAt != nil {
		t.Fatal("wrong-tenant revoke changed the token")
	}

	if err := MobilePushTokenRepository.Revoke(db, 11, 22, item.ID, updatedTime); err != nil {
		t.Fatalf("scoped Revoke(): %v", err)
	}
	var revoked models.MobilePushToken
	if err := db.First(&revoked, item.ID).Error; err != nil {
		t.Fatalf("reload revoked token: %v", err)
	}
	if revoked.RevokedAt == nil || !revoked.RevokedAt.Equal(updatedTime) {
		t.Fatalf("RevokedAt = %v, want %v", revoked.RevokedAt, updatedTime)
	}
	active, err = MobilePushTokenRepository.FindActiveByUser(db, 11, 22)
	if err != nil || len(active) != 0 {
		t.Fatalf("revoked token remained active: %+v, %v", active, err)
	}
}

package services

import (
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

func TestNormalizeMobilePushPlatform(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{input: " Android ", want: "android"},
		{input: "IOS", want: "ios"},
	} {
		got, err := normalizeMobilePushPlatform(test.input)
		if err != nil || got != test.want {
			t.Fatalf("normalizeMobilePushPlatform(%q) = %q, %v; want %q", test.input, got, err, test.want)
		}
	}
	for _, input := range []string{"", "web", "capacitor"} {
		if _, err := normalizeMobilePushPlatform(input); err == nil {
			t.Fatalf("normalizeMobilePushPlatform(%q) returned nil error", input)
		}
	}
}

func TestValidateMobilePushToken(t *testing.T) {
	if err := validateMobilePushToken("short"); err == nil {
		t.Fatal("short push token was accepted")
	}
	if err := validateMobilePushToken("0123456789abcdef"); err != nil {
		t.Fatalf("valid push token rejected: %v", err)
	}
	if err := validateMobilePushToken(strings.Repeat("x", 4097)); err == nil {
		t.Fatal("oversized push token was accepted")
	}
}

func TestMobilePushTokenServiceRegistersOnlyMatchingCustomerAccount(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.CustomerUser{}, &models.MobilePushToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqls.SetDB(nil)
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})

	now := time.Now()
	customer := models.CustomerUser{
		TenantID: 9,
		UserID:   42,
		Status:   enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}

	item, err := MobilePushTokenService.RegisterCustomer(9, 42, customer.ID, request.RegisterMobilePushTokenRequest{
		Platform:   "ios",
		Token:      "0123456789abcdef-customer-token",
		DeviceID:   "iphone-customer-42",
		AppVersion: "1.0.0",
	})
	if err != nil {
		t.Fatalf("RegisterCustomer(): %v", err)
	}
	if item.ID <= 0 || item.Platform != "ios" || item.DeviceID != "iphone-customer-42" {
		t.Fatalf("unexpected push token DTO: %+v", item)
	}

	var stored models.MobilePushToken
	if err := db.First(&stored, item.ID).Error; err != nil {
		t.Fatalf("load push token: %v", err)
	}
	if stored.UserID != customer.UserID || stored.TenantID != customer.TenantID || stored.TokenCiphertext == "0123456789abcdef-customer-token" {
		t.Fatalf("push token was not scoped and encrypted: %+v", stored)
	}

	if _, err := MobilePushTokenService.RegisterCustomer(9, 42, customer.ID+1, request.RegisterMobilePushTokenRequest{
		Platform: "ios",
		Token:    "0123456789abcdef-other-customer",
	}); err == nil {
		t.Fatal("mismatched customer identity registered a push token")
	}
	if _, err := MobilePushTokenService.RegisterCustomer(10, 42, customer.ID, request.RegisterMobilePushTokenRequest{
		Platform: "ios",
		Token:    "0123456789abcdef-other-tenant",
	}); err == nil {
		t.Fatal("cross-tenant customer registered a push token")
	}
}

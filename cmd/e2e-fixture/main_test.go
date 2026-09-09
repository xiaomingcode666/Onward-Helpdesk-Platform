package main

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestResolveFixtureTenant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:e2e-fixture-tenant?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err = db.AutoMigrate(&models.Tenant{}); err != nil {
		t.Fatalf("migrate tenant: %v", err)
	}
	defaultTenant := &models.Tenant{
		Name: "Default Tenant", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	if err = db.Create(defaultTenant).Error; err != nil {
		t.Fatalf("create default tenant: %v", err)
	}

	operator := &dto.AuthPrincipal{UserID: 1, Username: "e2e"}
	selected, err := resolveFixtureTenant(db, time.Now(), " Default Tenant ", operator)
	if err != nil {
		t.Fatalf("resolve existing tenant: %v", err)
	}
	if selected.ID != defaultTenant.ID || selected.Name != defaultTenant.Name {
		t.Fatalf("selected tenant = %+v, want id=%d name=%q", selected, defaultTenant.ID, defaultTenant.Name)
	}

	now := time.Date(2026, time.August, 4, 14, 30, 0, 0, time.UTC)
	isolated, err := resolveFixtureTenant(db, now, "", operator)
	if err != nil {
		t.Fatalf("create isolated tenant: %v", err)
	}
	if isolated.ID == defaultTenant.ID || isolated.Name != "P0 Acceptance 20260804-143000" {
		t.Fatalf("isolated tenant = %+v", isolated)
	}
}

func TestEnsureE2EProviderHostCanBeSeededRefusesSharedConfig(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		groupCode string
		wantError bool
	}{
		{name: "missing host"},
		{name: "isolated E2E host", groupCode: "e2e"},
		{name: "shared platform host", groupCode: "platform_ai", wantError: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open("file:e2e-provider-"+strings.ReplaceAll(testCase.name, " ", "-")+"?mode=memory&cache=shared"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.AutoMigrate(&models.SystemConfig{}); err != nil {
				t.Fatal(err)
			}
			if testCase.groupCode != "" {
				if err := db.Create(&models.SystemConfig{
					ConfigKey: "platform.sub2api.host", ConfigValue: "http://example.test", GroupCode: testCase.groupCode,
					Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()},
				}).Error; err != nil {
					t.Fatal(err)
				}
			}

			err = ensureE2EProviderHostCanBeSeeded(db)
			if testCase.wantError && err == nil {
				t.Fatal("expected shared provider configuration to be rejected")
			}
			if !testCase.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

package services

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
)

func TestTenantCommercialStateUsesSub2APIBalanceForPaidStatus(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)

	membership := TenantCommercialService.DescribeMembership(db, fixture.Tenant.ID)
	if membership == nil || !membership.Paid {
		t.Fatalf("DescribeMembership() = %+v, want paid tenant for positive Sub2API balance", membership)
	}

	if err := db.Model(&models.Sub2APITenantAccount{}).
		Where("tenant_id = ?", fixture.Tenant.ID).
		Update("balance", 0).Error; err != nil {
		t.Fatalf("clear tenant balance: %v", err)
	}
	membership = TenantCommercialService.DescribeMembership(db, fixture.Tenant.ID)
	if membership == nil || membership.Paid {
		t.Fatalf("DescribeMembership() = %+v, want free tenant for empty Sub2API balance", membership)
	}
}

func TestRequireTenantAIWorkspaceForProductCreateUsesTenantAccount(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)

	if err := TenantCommercialService.RequireTenantAIWorkspaceForProductCreate(fixture.Tenant.ID); err != nil {
		t.Fatalf("RequireTenantAIWorkspaceForProductCreate() error = %v", err)
	}

	account := repositories.PlatformIAMRepository.FindSub2APIAccountByTenantID(db, fixture.Tenant.ID)
	if account == nil {
		t.Fatal("expected tenant AI account")
	}
	if err := db.Model(&models.Sub2APITenantAccount{}).Where("id = ?", account.ID).Updates(map[string]any{
		"balance": 0,
	}).Error; err != nil {
		t.Fatalf("disable tenant AI balance: %v", err)
	}
	if err := TenantCommercialService.RequireTenantAIWorkspaceForProductCreate(fixture.Tenant.ID); err == nil {
		t.Fatal("RequireTenantAIWorkspaceForProductCreate() error = nil with empty balance")
	}
}

func TestTenantAIWorkspaceReady(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	futureToken := now.Add(time.Hour)
	futureKey := now.Add(2 * time.Hour)
	valid := &models.Sub2APITenantAccount{
		Status:                  enums.StatusOk,
		AccountStatus:           "active",
		ProvisionStatus:         "active",
		LoginEmail:              "tenant@example.com",
		LoginPasswordCiphertext: "encrypted-password",
		AccessTokenExpiresAt:    &futureToken,
		Balance:                 21,
		DefaultKeyID:            "default-key",
		DefaultKeyCiphertext:    "encrypted-key",
		DefaultKeyStatus:        "active",
		DefaultKeyExpiresAt:     &futureKey,
	}

	tests := []struct {
		name    string
		account *models.Sub2APITenantAccount
		want    bool
	}{
		{name: "ready tenant account", account: valid, want: true},
		{name: "missing account", account: nil, want: false},
		{name: "tenant account disabled", account: func() *models.Sub2APITenantAccount { item := *valid; item.AccountStatus = "disabled"; return &item }(), want: false},
		{name: "provisioning incomplete", account: func() *models.Sub2APITenantAccount {
			item := *valid
			item.ProvisionStatus = "provisioning_key"
			return &item
		}(), want: false},
		{name: "access token expired", account: func() *models.Sub2APITenantAccount {
			item := *valid
			expired := now.Add(-time.Minute)
			item.AccessTokenExpiresAt = &expired
			return &item
		}(), want: false},
		{name: "empty balance", account: func() *models.Sub2APITenantAccount { item := *valid; item.Balance = 0; return &item }(), want: false},
		{name: "default key unavailable", account: func() *models.Sub2APITenantAccount { item := *valid; item.DefaultKeyCiphertext = ""; return &item }(), want: false},
		{name: "default key expired", account: func() *models.Sub2APITenantAccount {
			item := *valid
			expired := now.Add(-time.Minute)
			item.DefaultKeyExpiresAt = &expired
			return &item
		}(), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tenantAIWorkspaceReady(tt.account, now); got != tt.want {
				t.Fatalf("tenantAIWorkspaceReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTenantAIAccountStatusReady(t *testing.T) {
	for _, status := range []string{"active", "enabled", "启用"} {
		if !tenantAIAccountStatusReady(status) {
			t.Fatalf("tenantAIAccountStatusReady(%q) = false, want true", status)
		}
	}
	for _, status := range []string{"", "unknown", "disabled", "frozen"} {
		if tenantAIAccountStatusReady(status) {
			t.Fatalf("tenantAIAccountStatusReady(%q) = true, want false", status)
		}
	}
}

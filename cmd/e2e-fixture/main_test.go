package main

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestRegisterFixturePartnerCreatesLoginAndConsumesInvitation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	previousDB := sqls.DB()
	sqls.SetDB(db)
	t.Cleanup(func() { sqls.SetDB(previousDB) })
	previousConfig := config.CurrentOrDefault()
	config.SetCurrent(&config.Config{Auth: config.AuthConfig{TokenTTLHours: 2}})
	t.Cleanup(func() { config.SetCurrent(&previousConfig) })
	if err := bootstrap.AutoMigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	tenant := models.Tenant{Name: "Synthetic supplier fixture tenant", Status: enums.StatusOk}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatal(err)
	}
	operator := &dto.AuthPrincipal{TenantID: tenant.ID, UserID: 1, Username: "fixture", DomainType: models.DomainTypeEnterprise}
	username := "e2e.supplier.registration"
	invitation, err := services.EnterpriseIAMService.InvitePartnerAdmin(tenant.ID, request.EnterprisePartnerAdminInviteRequest{
		PartnerNo: "P0-SUPPLIER-REGISTRATION", PartnerName: "Synthetic fixture supplier", PartnerType: "module_supplier",
		Username: username, DisplayName: "P0 Supplier Engineer", Email: username + "@example.test", Password: fixturePassword,
	}, operator)
	if err != nil {
		t.Fatalf("invite supplier: %v", err)
	}
	var userCount int64
	if err := db.Model(&models.User{}).Where("username = ?", username).Count(&userCount).Error; err != nil {
		t.Fatal(err)
	}
	if userCount != 0 {
		t.Fatal("invitation must not create a user before registration")
	}
	if err := registerFixturePartner(invitation, username); err != nil {
		t.Fatal(err)
	}
	login, err := services.AuthService.Login(request.LoginRequest{
		Username: username, Password: fixturePassword, DomainType: models.DomainTypePartner,
	}, config.CurrentOrDefault().Auth, "127.0.0.1", "e2e-fixture-test")
	if err != nil {
		t.Fatalf("supplier cannot log in with fixture credentials: %v", err)
	}
	if login == nil || login.User == nil || login.User.Username != username || login.AccessToken == "" ||
		login.DomainType != models.DomainTypePartner || login.TenantID != tenant.ID || login.PartnerAccountID <= 0 {
		t.Fatal("supplier login did not return the expected tenant and partner identity")
	}
	var account models.PartnerAccount
	if err := db.First(&account, login.PartnerAccountID).Error; err != nil {
		t.Fatal(err)
	}
	if account.UserID != login.User.ID || account.TenantID != tenant.ID || account.PartnerCompanyID != invitation.PartnerCompany.ID {
		t.Fatal("supplier account is not bound to the invited user, tenant, and company")
	}
	hasPartnerAdmin := false
	for _, role := range login.Roles {
		if role == services.PartnerRoleAdmin {
			hasPartnerAdmin = true
		}
	}
	if !hasPartnerAdmin {
		t.Fatal("supplier login is missing the invited partner administrator role")
	}
	var grant models.CustomerRegistrationGrant
	if err := db.First(&grant, invitation.Grant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if grant.Status != models.CustomerRegistrationGrantConsumed || grant.ConsumedUserID != login.User.ID || grant.ConsumedAt == nil {
		t.Fatal("supplier registration did not consume the invitation")
	}
	if err := registerFixturePartner(invitation, username+".reused"); err == nil {
		t.Fatal("consumed supplier invitation must not register another account")
	}
}

func TestSeedAIConfigsProvidesUsableTenantWorkspace(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	previousDB := sqls.DB()
	sqls.SetDB(db)
	t.Cleanup(func() { sqls.SetDB(previousDB) })
	previousConfig := config.CurrentOrDefault()
	config.SetCurrent(&config.Config{EncryptionKey: "fixture-local-encryption"})
	t.Cleanup(func() { config.SetCurrent(&previousConfig) })
	if err := db.AutoMigrate(&models.Tenant{}, &models.SystemConfig{}, &models.Sub2APITenantAccount{}, &models.AIConfig{}); err != nil {
		t.Fatal(err)
	}
	tenant := models.Tenant{ID: 71, Name: "Synthetic CI tenant", Status: enums.StatusOk}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatal(err)
	}
	op := &dto.AuthPrincipal{TenantID: tenant.ID, UserID: 1, Username: "fixture"}
	for pass := 0; pass < 2; pass++ {
		if err := seedAIConfigs(db, tenant.ID, op); err != nil {
			t.Fatal(err)
		}
		if err := services.TenantCommercialService.RequireTenantAIWorkspaceForProductCreate(tenant.ID); err != nil {
			t.Fatalf("seeded tenant cannot enter actual product creation: %v", err)
		}
		// Re-seeding must restore a usable test account as well as insert one.
		expiredAt := time.Now().Add(-time.Hour)
		if err := db.Model(&models.Sub2APITenantAccount{}).Where("tenant_id = ?", tenant.ID).Updates(map[string]any{
			"balance": 0, "access_token_expires_at": expiredAt, "default_key_expires_at": expiredAt,
		}).Error; err != nil {
			t.Fatal(err)
		}
		if err := services.TenantCommercialService.RequireTenantAIWorkspaceForProductCreate(tenant.ID); err == nil {
			t.Fatal("expired tenant credentials must block product creation before re-seeding")
		}
	}
}

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

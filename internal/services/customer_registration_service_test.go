package services

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupCustomerRegistrationTestDB(t *testing.T) (*gorm.DB, models.Tenant, *dto.AuthPrincipal) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() { sqlDB, _ := db.DB(); _ = sqlDB.Close() })
	if err := db.AutoMigrate(
		&models.User{}, &models.Tenant{}, &models.CustomerOrg{}, &models.CustomerUser{},
		&models.CustomerRegistrationGrant{}, &models.Customer{}, &models.CustomerIdentity{},
		&models.AuthRole{}, &models.AuthRoleBinding{}, &models.AuthPermissionCatalog{}, &models.AuthRolePermission{},
		&models.Role{}, &models.Permission{}, &models.UserRole{}, &models.RolePermission{}, &models.UserPermission{},
		&models.LoginSession{}, &models.LoginCredentialLog{}, &models.Product{}, &models.Device{},
		&models.ServiceCode{}, &models.CustomerDeviceBinding{}, &models.AuthAuditLog{}, &models.Ticket{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	tenant := models.Tenant{Name: "Registration Tenant", DefaultLocale: "zh-CN", Timezone: "Asia/Shanghai", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	operator := &dto.AuthPrincipal{UserID: 99, Username: "tenant-admin", TenantID: tenant.ID, DomainType: models.DomainTypeEnterprise}
	return db, tenant, operator
}

func TestCustomerRegistrationInvitationCreatesCustomerIdentityAndConsumesGrant(t *testing.T) {
	db, tenant, operator := setupCustomerRegistrationTestDB(t)
	invitation, err := CustomerRegistrationService.InviteCustomer(tenant.ID, request.EnterpriseCustomerUserInviteRequest{
		DisplayName: "Invited Customer", Email: "invited@example.com", CustomerOrg: "Example Factory",
		RoleCodes: []string{CustomerRoleUser}, Locale: "zh-CN", Timezone: "Asia/Shanghai",
	}, operator)
	if err != nil {
		t.Fatalf("invite customer: %v", err)
	}
	if invitation.InviteCode == "" || invitation.Grant.TokenHash == invitation.InviteCode || !strings.Contains(invitation.RegistrationURL, "invite=") {
		t.Fatalf("unsafe or incomplete invitation: %+v", invitation)
	}
	context, err := CustomerRegistrationService.Verify(request.CustomerRegistrationVerifyRequest{Method: "invite", Credential: invitation.InviteCode})
	if err != nil || context.Tenant.ID != tenant.ID || context.CustomerOrg == nil || context.Email != "invited@example.com" {
		t.Fatalf("verify invitation context=%+v err=%v", context, err)
	}
	login, err := CustomerRegistrationService.Register(request.CustomerRegistrationRequest{
		Method: "invite", Credential: invitation.InviteCode, Username: "invited.customer",
		DisplayName: "Invited Customer", Email: "invited@example.com", Password: "StrongPass123!",
	}, config.AuthConfig{TokenTTLHours: 2}, "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("register invited customer: %v", err)
	}
	if login.DomainType != models.DomainTypeCustomer || login.TenantID != tenant.ID || login.AccessToken == "" {
		t.Fatalf("unexpected login: %+v", login)
	}
	storedGrant := repositories.CustomerRegistrationRepository.FindByTokenHash(db, hashCustomerRegistrationCode(invitation.InviteCode))
	if storedGrant == nil || storedGrant.Status != models.CustomerRegistrationGrantConsumed || storedGrant.ConsumedUserID <= 0 {
		t.Fatalf("grant not consumed: %+v", storedGrant)
	}
	if _, err := CustomerRegistrationService.Verify(request.CustomerRegistrationVerifyRequest{Method: "invite", Credential: invitation.InviteCode}); err == nil {
		t.Fatal("consumed invitation remained valid")
	}
}

func TestCustomerRegistrationDeviceCodeRequiresRealDeviceAndCreatesBinding(t *testing.T) {
	db, tenant, _ := setupCustomerRegistrationTestDB(t)
	now := time.Now()
	product := models.Product{TenantID: tenant.ID, Code: "PRESS", Name: "Hydraulic Press", DefaultLocale: "zh-CN", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&product).Error; err != nil {
		t.Fatal(err)
	}
	device := models.Device{TenantID: tenant.ID, DeviceNo: "DEV-100", ProductID: product.ID, Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&device).Error; err != nil {
		t.Fatal(err)
	}
	code := models.ServiceCode{TenantID: tenant.ID, ServiceCode: "DEVICE-CODE-100", DeviceID: device.ID, ProductID: product.ID, Status: enums.ServiceCodeStatusActive, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&code).Error; err != nil {
		t.Fatal(err)
	}
	context, err := CustomerRegistrationService.Verify(request.CustomerRegistrationVerifyRequest{Method: "service_code", Credential: code.ServiceCode})
	if err != nil || context.Device == nil || context.Product == nil || context.Tenant.ID != tenant.ID {
		t.Fatalf("verify device context=%+v err=%v", context, err)
	}
	login, err := CustomerRegistrationService.Register(request.CustomerRegistrationRequest{
		Method: "service_code", Credential: code.ServiceCode, Username: "device.owner",
		Email: "owner@example.com", Password: "StrongPass123!",
	}, config.AuthConfig{TokenTTLHours: 2}, "127.0.0.1", "go-test")
	if err != nil {
		t.Fatalf("register by device code: %v", err)
	}
	if login.DomainType != models.DomainTypeCustomer || login.TenantID != tenant.ID {
		t.Fatalf("unexpected login: %+v", login)
	}
	customerUser := repositories.CustomerPortalRepository.FindCustomerUserByUserID(db, login.User.ID)
	if customerUser == nil {
		t.Fatal("customer user was not created")
	}
	binding := repositories.CustomerDeviceBindingRepository.FindByCustomer(db, tenant.ID, device.ID, customerUser.ID, customerUser.CustomerOrgID)
	if binding == nil || binding.BindingRole != "owner" {
		t.Fatalf("device binding not created: %+v", binding)
	}
}

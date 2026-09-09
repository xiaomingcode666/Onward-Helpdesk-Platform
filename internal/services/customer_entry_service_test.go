package services

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

type customerEntryFixture struct {
	DB       *gorm.DB
	Tenant   models.Tenant
	Product  models.Product
	Device   models.Device
	Active   models.ServiceCode
	Disabled models.ServiceCode
}

func setupCustomerEntryTestDB(t *testing.T) customerEntryFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() { sqlDB, _ := db.DB(); _ = sqlDB.Close() })
	if err := db.AutoMigrate(&models.Tenant{}, &models.Product{}, &models.Device{}, &models.ServiceCode{}, &models.CustomerEntrySession{}, &models.CustomerPrivacyConsent{}, &models.CustomerDeviceBinding{}, &models.DeviceRegistrationTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	audit := models.AuditFields{CreatedAt: now, UpdatedAt: now}
	fixture := customerEntryFixture{DB: db, Tenant: models.Tenant{Name: "Tenant", DefaultLocale: "en-US", Status: enums.StatusOk, AuditFields: audit}}
	if err := db.Create(&fixture.Tenant).Error; err != nil {
		t.Fatal(err)
	}
	fixture.Product = models.Product{TenantID: fixture.Tenant.ID, Code: "P-1", Name: "Product", DefaultLocale: "en-US", Status: enums.StatusOk, AuditFields: audit}
	if err := db.Create(&fixture.Product).Error; err != nil {
		t.Fatal(err)
	}
	fixture.Device = models.Device{TenantID: fixture.Tenant.ID, DeviceNo: "D-1", ProductID: fixture.Product.ID, Status: enums.StatusOk, AuditFields: audit}
	if err := db.Create(&fixture.Device).Error; err != nil {
		t.Fatal(err)
	}
	fixture.Active = models.ServiceCode{TenantID: fixture.Tenant.ID, ServiceCode: "ACTIVE-1", DeviceID: fixture.Device.ID, ProductID: fixture.Product.ID, Status: enums.ServiceCodeStatusActive, AuditFields: audit}
	fixture.Disabled = models.ServiceCode{TenantID: fixture.Tenant.ID, ServiceCode: "DISABLED-1", DeviceID: fixture.Device.ID, ProductID: fixture.Product.ID, Status: enums.ServiceCodeStatusRevoked, AuditFields: audit}
	if err := db.Create(&fixture.Active).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&fixture.Disabled).Error; err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestCustomerEntryServiceCreatesSessionFromActiveServiceCode(t *testing.T) {
	fixture := setupCustomerEntryTestDB(t)
	resp, err := CustomerEntryService.CreateEntrySession(request.CreateCustomerEntrySessionRequest{ServiceCode: fixture.Active.ServiceCode, VisitorID: "visitor-1", Locale: "zh-CN"})
	if err != nil {
		t.Fatalf("create entry session: %v", err)
	}
	if resp.Session.ID <= 0 || resp.ServiceCodeValue != fixture.Active.ServiceCode || resp.Device == nil || resp.Session.VisitorID != "visitor-1" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Session.ProductID != fixture.Product.ID || resp.Session.TenantID != fixture.Tenant.ID || resp.VisitorToken == "" || resp.Session.VisitorTokenHash == "" {
		t.Fatalf("entry session scope or visitor credential missing: %+v", resp)
	}
	if !CustomerEntryService.VerifyVisitorToken(resp.Session, resp.VisitorToken) {
		t.Fatalf("generated visitor token does not verify")
	}
	item := repositories.CustomerEntrySessionRepository.Get(fixture.DB, resp.Session.ID)
	if item == nil || item.State != "active" || item.ExpiresAt == nil || item.ExpiresAt.Before(time.Now().Add(23*time.Hour)) {
		t.Fatalf("unexpected session: %+v", item)
	}
}

func TestCustomerEntryServiceRejectsDisabledServiceCode(t *testing.T) {
	setupCustomerEntryTestDB(t)
	_, err := CustomerEntryService.CreateEntrySession(request.CreateCustomerEntrySessionRequest{ServiceCode: "DISABLED-1"})
	if err == nil {
		t.Fatal("expected disabled service code error")
	}
}

func TestCustomerEntryServiceCreateSessionIsIdempotentWithVisitorToken(t *testing.T) {
	fixture := setupCustomerEntryTestDB(t)
	req := request.CreateCustomerEntrySessionRequest{
		ServiceCode:  fixture.Active.ServiceCode,
		VisitorID:    "visitor-idempotent",
		VisitorToken: "visitor-token-idempotent-1234567890",
		Locale:       "zh-CN",
	}
	first, err := CustomerEntryService.CreateEntrySession(req)
	if err != nil {
		t.Fatalf("create first entry session: %v", err)
	}
	second, err := CustomerEntryService.CreateEntrySession(req)
	if err != nil {
		t.Fatalf("create duplicate entry session: %v", err)
	}
	if first.Session.ID != second.Session.ID || first.VisitorToken != second.VisitorToken {
		t.Fatalf("idempotent entry session mismatch: first=%d second=%d", first.Session.ID, second.Session.ID)
	}
	var count int64
	fixture.DB.Model(&models.CustomerEntrySession{}).
		Where("visitor_token_hash = ?", first.Session.VisitorTokenHash).
		Count(&count)
	if count != 1 {
		t.Fatalf("entry session count=%d want 1", count)
	}
}

func TestCustomerEntrySessionRepositoryCreateIfAbsentDoesNotRaiseUniqueConflict(t *testing.T) {
	fixture := setupCustomerEntryTestDB(t)
	now := time.Now()
	tokenHash := hashCustomerEntrySecret("visitor-token-conflict-1234567890")
	first := &models.CustomerEntrySession{
		TenantID: fixture.Tenant.ID, ProductID: fixture.Product.ID, DeviceID: fixture.Device.ID,
		ServiceCodeID: fixture.Active.ID, VisitorID: "visitor-conflict", VisitorTokenHash: tokenHash,
		State: "active", EntryContextJSON: "{}", CreatedAt: now, UpdatedAt: now,
	}
	created, err := repositories.CustomerEntrySessionRepository.CreateIfAbsent(fixture.DB, first)
	if err != nil || !created {
		t.Fatalf("create first session: created=%v err=%v", created, err)
	}
	duplicate := *first
	duplicate.ID = 0
	duplicate.CreatedAt = now.Add(time.Second)
	duplicate.UpdatedAt = duplicate.CreatedAt
	created, err = repositories.CustomerEntrySessionRepository.CreateIfAbsent(fixture.DB, &duplicate)
	if err != nil {
		t.Fatalf("duplicate insert should not raise a unique conflict: %v", err)
	}
	if created {
		t.Fatal("duplicate session was unexpectedly inserted")
	}
	var count int64
	fixture.DB.Model(&models.CustomerEntrySession{}).Where("visitor_token_hash = ?", tokenHash).Count(&count)
	if count != 1 {
		t.Fatalf("entry session count=%d want 1", count)
	}
}

func TestCustomerEntryServiceRejectsVisitorTokenScopeReuse(t *testing.T) {
	fixture := setupCustomerEntryTestDB(t)
	token := "visitor-token-scope-reuse-1234567890"
	if _, err := CustomerEntryService.CreateEntrySession(request.CreateCustomerEntrySessionRequest{
		ServiceCode: fixture.Active.ServiceCode, VisitorID: "visitor-first", VisitorToken: token,
	}); err != nil {
		t.Fatalf("create first entry session: %v", err)
	}
	if _, err := CustomerEntryService.CreateEntrySession(request.CreateCustomerEntrySessionRequest{
		ServiceCode: fixture.Active.ServiceCode, VisitorID: "visitor-other", VisitorToken: token,
	}); err == nil {
		t.Fatal("expected visitor token reuse across visitor scopes to fail")
	}
}

func TestCustomerEntryServiceReopensCompletedGeneralCodeRegistrationTask(t *testing.T) {
	fixture := setupCustomerEntryTestDB(t)
	now := time.Now()
	generalCode := models.ServiceCode{
		TenantID: fixture.Tenant.ID, ServiceCode: "GENERAL-1", Mode: enums.ServiceCodeModeGeneral,
		ProductID: fixture.Product.ID, Status: enums.ServiceCodeStatusActive,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := fixture.DB.Create(&generalCode).Error; err != nil {
		t.Fatal(err)
	}
	completedAt := now.Add(-time.Hour)
	task := models.DeviceRegistrationTask{
		TenantID: fixture.Tenant.ID, ServiceCodeID: generalCode.ID, ServiceCodeVal: generalCode.ServiceCode,
		ProductID: fixture.Product.ID, DeviceNo: "OLD-DEVICE", SerialNo: "OLD-SERIAL",
		RegistrationDataJSON: `{"old":true}`, Status: "completed", CompletedAt: &completedAt,
		AuditFields: models.AuditFields{CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)},
	}
	if err := fixture.DB.Create(&task).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := CustomerEntryService.CreateEntrySession(request.CreateCustomerEntrySessionRequest{
		ServiceCode: generalCode.ServiceCode, VisitorID: "general-code-visitor", CustomerUserID: 42,
	}); err != nil {
		t.Fatalf("create general-code entry session: %v", err)
	}
	stored := repositories.DeviceRegistrationTaskRepository.FindByServiceCodeID(fixture.DB, generalCode.ID)
	if stored == nil || stored.ID != task.ID || stored.Status != "pending" || stored.CompletedAt != nil {
		t.Fatalf("registration task was not reopened: %+v", stored)
	}
	if stored.DeviceNo != "" || stored.SerialNo != "" || stored.RegistrationDataJSON != "{}" || stored.CustomerUserID != 42 {
		t.Fatalf("registration task retained stale completion data: %+v", stored)
	}
	var count int64
	fixture.DB.Model(&models.DeviceRegistrationTask{}).Where("service_code_id = ?", generalCode.ID).Count(&count)
	if count != 1 {
		t.Fatalf("registration task count=%d want 1", count)
	}
}

func TestCustomerEntryServiceRegistersGeneralCodeDevice(t *testing.T) {
	fixture := setupCustomerEntryTestDB(t)
	now := time.Now()
	generalCode := models.ServiceCode{
		TenantID: fixture.Tenant.ID, ServiceCode: "GENERAL-REGISTER-1", Mode: enums.ServiceCodeModeGeneral,
		ProductID: fixture.Product.ID, Status: enums.ServiceCodeStatusActive,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := fixture.DB.Create(&generalCode).Error; err != nil {
		t.Fatal(err)
	}

	resp, err := CustomerEntryService.CreateEntrySession(request.CreateCustomerEntrySessionRequest{
		ServiceCode: generalCode.ServiceCode, DeviceNo: " dev-customer-001 ", SerialNo: "SN-001", RegionCode: "CN-SH",
		VisitorID: "registered-customer", VisitorToken: "registered-customer-token-1234567890",
	})
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if resp.Device == nil || resp.Device.DeviceNo != "DEV-CUSTOMER-001" || resp.Device.ProductID != fixture.Product.ID {
		t.Fatalf("registered device mismatch: %+v", resp.Device)
	}
	if resp.Session.DeviceID != resp.Device.ID || resp.Session.ServiceCodeID != generalCode.ID {
		t.Fatalf("entry session lost registered device scope: %+v", resp.Session)
	}
	storedCode := repositories.ServiceCodeRepository.Get(fixture.DB, generalCode.ID)
	if storedCode == nil || storedCode.Status != enums.ServiceCodeStatusBound || storedCode.DeviceID != resp.Device.ID {
		t.Fatalf("service code was not bound: %+v", storedCode)
	}
	task := repositories.DeviceRegistrationTaskRepository.FindByServiceCodeID(fixture.DB, generalCode.ID)
	if task == nil || task.Status != "completed" || task.DeviceNo != resp.Device.DeviceNo || task.CompletedAt == nil {
		t.Fatalf("registration task was not completed: %+v", task)
	}

	second, err := CustomerEntryService.CreateEntrySession(request.CreateCustomerEntrySessionRequest{
		ServiceCode: generalCode.ServiceCode, VisitorID: "registered-customer-second",
	})
	if err != nil {
		t.Fatalf("reuse bound service code: %v", err)
	}
	if second.Device == nil || second.Device.ID != resp.Device.ID {
		t.Fatalf("bound service code resolved another device: %+v", second.Device)
	}
}

func TestCustomerEntryServiceRejectsForeignCustomerDuringDeviceRegistration(t *testing.T) {
	fixture := setupCustomerEntryTestDB(t)
	if err := fixture.DB.AutoMigrate(&models.CustomerUser{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	foreignCustomer := models.CustomerUser{
		TenantID: fixture.Tenant.ID + 1, UserID: 99, DisplayName: "Foreign customer", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := fixture.DB.Create(&foreignCustomer).Error; err != nil {
		t.Fatal(err)
	}
	generalCode := models.ServiceCode{
		TenantID: fixture.Tenant.ID, ServiceCode: "GENERAL-FOREIGN-1", Mode: enums.ServiceCodeModeGeneral,
		ProductID: fixture.Product.ID, Status: enums.ServiceCodeStatusActive,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := fixture.DB.Create(&generalCode).Error; err != nil {
		t.Fatal(err)
	}

	_, err := CustomerEntryService.CreateEntrySession(request.CreateCustomerEntrySessionRequest{
		ServiceCode: generalCode.ServiceCode, DeviceNo: "DEV-FOREIGN-001", CustomerUserID: foreignCustomer.ID,
	})
	if err == nil {
		t.Fatal("expected foreign customer registration to be rejected")
	}
	if repositories.DeviceRepository.GetByTenantDeviceNo(fixture.DB, fixture.Tenant.ID, "DEV-FOREIGN-001") != nil {
		t.Fatal("foreign customer registration created a device")
	}
}

func TestCustomerEntryServiceConfirmBindingIsIdempotent(t *testing.T) {
	fixture := setupCustomerEntryTestDB(t)
	resp, err := CustomerEntryService.CreateEntrySession(request.CreateCustomerEntrySessionRequest{ServiceCode: fixture.Active.ServiceCode, CustomerUserID: 42})
	if err != nil {
		t.Fatal(err)
	}
	req := request.ConfirmCustomerDeviceBindingRequest{EntrySessionID: resp.Session.ID, VisitorID: resp.Session.VisitorID, VisitorToken: resp.VisitorToken, CustomerUserID: 42, BindingRole: "owner"}
	first, err := CustomerEntryService.ConfirmDeviceBinding(req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CustomerEntryService.ConfirmDeviceBinding(req)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("binding was not idempotent: %d != %d", first.ID, second.ID)
	}
	var count int64
	fixture.DB.Model(&models.CustomerDeviceBinding{}).Where("tenant_id = ? and device_id = ? and customer_user_id = ?", fixture.Tenant.ID, fixture.Device.ID, 42).Count(&count)
	if count != 1 {
		t.Fatalf("binding count=%d want 1", count)
	}
}

func TestCustomerEntryServiceRejectsBindingWithoutVisitorCredential(t *testing.T) {
	fixture := setupCustomerEntryTestDB(t)
	resp, err := CustomerEntryService.CreateEntrySession(request.CreateCustomerEntrySessionRequest{ServiceCode: fixture.Active.ServiceCode})
	if err != nil {
		t.Fatal(err)
	}
	_, err = CustomerEntryService.ConfirmDeviceBinding(request.ConfirmCustomerDeviceBindingRequest{
		EntrySessionID: resp.Session.ID,
		CustomerUserID: 42,
		BindingRole:    "owner",
	})
	if err == nil {
		t.Fatal("expected missing visitor credential to be rejected")
	}
}

func TestCustomerPortalDeviceBindingAutoRegistersPendingCustomer(t *testing.T) {
	fixture := setupCustomerEntryTestDB(t)
	if err := fixture.DB.AutoMigrate(
		&models.User{}, &models.CustomerOrg{}, &models.CustomerUser{},
		&models.Customer{}, &models.CustomerIdentity{},
		&models.AuthRole{}, &models.AuthRolePermission{}, &models.AuthRoleBinding{},
	); err != nil {
		t.Fatalf("migrate customer identity tables: %v", err)
	}
	now := time.Now()
	user := &models.User{
		Username: "device-owner", Nickname: "设备业主", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := fixture.DB.Create(user).Error; err != nil {
		t.Fatalf("create pending account: %v", err)
	}
	operator := &dto.AuthPrincipal{
		UserID: user.ID, Username: user.Username, Nickname: user.Nickname,
		DomainType: models.DomainTypeCustomer, SubjectType: models.SubjectTypePendingCustomer, SubjectID: user.ID,
	}

	binding, deviceResult, productResult, err := CustomerPortalService.BindDeviceByServiceCode(request.BindCustomerDeviceByServiceCodeRequest{
		ServiceCode: fixture.Active.ServiceCode,
	}, operator)
	if err != nil {
		t.Fatalf("bind device as pending customer: %v", err)
	}
	if binding == nil || binding.DeviceID != fixture.Device.ID || binding.TenantID != fixture.Tenant.ID {
		t.Fatalf("unexpected binding result: binding=%+v", binding)
	}
	if deviceResult == nil || deviceResult.ID != fixture.Device.ID || productResult == nil || productResult.ID != fixture.Product.ID {
		t.Fatalf("unexpected binding display context: device=%+v product=%+v", deviceResult, productResult)
	}
	customerUser := repositories.CustomerPortalRepository.FindCustomerUserByTenantAndUserID(fixture.DB, fixture.Tenant.ID, user.ID)
	if customerUser == nil || customerUser.ID != binding.CustomerUserID || customerUser.CustomerOrgID != binding.CustomerOrgID {
		t.Fatalf("pending account was not registered in device tenant: %+v", customerUser)
	}
	device := repositories.DeviceRepository.Get(fixture.DB, fixture.Device.ID)
	if device == nil || device.CustomerOrgID != customerUser.CustomerOrgID {
		t.Fatalf("device ownership was not assigned to the registered customer organization: %+v", device)
	}
	if identity := repositories.CustomerIdentityRepository.GetBy(fixture.DB, enums.ExternalSourceUser, strconv.FormatInt(user.ID, 10)); identity == nil {
		t.Fatal("formal customer identity was not created")
	}
	role := repositories.PlatformIAMRepository.FindAuthRoleByCode(fixture.DB, fixture.Tenant.ID, models.DomainTypeCustomer, CustomerRoleUser)
	if role == nil {
		t.Fatal("customer role was not provisioned")
	}
	var roleBinding models.AuthRoleBinding
	if err := fixture.DB.Where("tenant_id = ? AND subject_type = ? AND subject_id = ? AND role_id = ?", fixture.Tenant.ID, models.SubjectTypeCustomerUser, customerUser.ID, role.ID).First(&roleBinding).Error; err != nil {
		t.Fatalf("customer role binding was not created: %v", err)
	}
}

func TestCustomerEntryServicePersistsPrivacyConsentReceipt(t *testing.T) {
	fixture := setupCustomerEntryTestDB(t)
	entry, err := CustomerEntryService.CreateEntrySession(request.CreateCustomerEntrySessionRequest{
		ServiceCode:    fixture.Active.ServiceCode,
		VisitorID:      "privacy-visitor",
		CustomerUserID: 42,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := CustomerEntryService.RequirePrivacyConsent(entry.Session); err == nil {
		t.Fatal("expected consent guard before confirmation")
	}
	receipt, err := CustomerEntryService.ConfirmPrivacyConsent(
		request.ConfirmCustomerPrivacyConsentRequest{
			EntrySessionID: entry.Session.ID, PolicyVersion: constants.CustomerPrivacyPolicyVersion,
			RequiredAccepted: true, AnalyticsAccepted: true,
		},
		entry.Session.VisitorID,
		entry.VisitorToken,
		CustomerPrivacyConsentMetadata{IPAddress: "127.0.0.1", UserAgent: "test", RequestID: "request-1"},
	)
	if err != nil {
		t.Fatalf("confirm privacy consent: %v", err)
	}
	if receipt.EntrySessionID != entry.Session.ID || !receipt.RequiredAccepted || !receipt.AnalyticsAccepted || receipt.MarketingAccepted {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	if len(receipt.ReceiptHash) != 64 || receipt.RequestID != "request-1" {
		t.Fatalf("receipt audit metadata missing: %+v", receipt)
	}
	if err := CustomerEntryService.RequirePrivacyConsent(entry.Session); err != nil {
		t.Fatalf("consent guard rejected confirmed session: %v", err)
	}
}

package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func setupPrivacyTenantTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := "privacy_tenant_" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.DSARRequest{}, &models.DSARExecutionLog{}, &models.DataBreachRecord{},
		&models.Customer{}, &models.CustomerContact{}, &models.CustomerIdentity{},
		&models.Ticket{}, &models.Conversation{}, &models.User{}, &models.UserIdentity{},
		&models.TenantMember{}, &models.CustomerEntrySession{}, &models.CustomerPrivacyConsent{}, &models.QrScanLog{}, &models.AuditLog{},
	); err != nil {
		t.Fatalf("migrate privacy models: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqls.SetDB(db)
	return db
}

func TestDSARRejectsCrossTenantResourceAccessAndDeletion(t *testing.T) {
	db := setupPrivacyTenantTestDB(t)
	now := time.Now()
	customers := []models.Customer{
		{Name: "Tenant One Customer", PrimaryEmail: "one@example.com", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{Name: "Tenant Two Customer", PrimaryEmail: "two@example.com", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&customers).Error; err != nil {
		t.Fatalf("create customers: %v", err)
	}
	tickets := []models.Ticket{
		{TicketNo: "PRIVACY-T1", TenantID: 1, CustomerID: customers[0].ID, Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TicketNo: "PRIVACY-T2", TenantID: 2, CustomerID: customers[1].ID, Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatalf("create tickets: %v", err)
	}

	if _, err := DSARService.SubmitDSAR(context.Background(), "1", "customer", formatID(customers[1].ID), "two@example.com", "delete"); err == nil {
		t.Fatal("expected foreign customer DSAR submission to fail")
	}
	request, err := DSARService.SubmitDSAR(context.Background(), "1", "customer", formatID(customers[0].ID), "one@example.com", "delete")
	if err != nil {
		t.Fatalf("submit owned DSAR: %v", err)
	}
	if _, err := DSARService.GetDSARStatus(context.Background(), "2", request.ID); err == nil {
		t.Fatal("expected foreign DSAR status read to fail")
	}
	if err := DSARService.VerifyIdentity(context.Background(), "2", request.ID, "email"); err == nil {
		t.Fatal("expected foreign DSAR verification to fail")
	}
	if err := DSARService.ExecuteDataDeletion(context.Background(), "2", request.ID); err == nil {
		t.Fatal("expected foreign DSAR deletion to fail")
	}
	var customer models.Customer
	if err := db.First(&customer, customers[0].ID).Error; err != nil {
		t.Fatalf("reload customer: %v", err)
	}
	if customer.Name != "Tenant One Customer" || customer.PrimaryEmail != "one@example.com" {
		t.Fatalf("foreign deletion changed customer: %+v", customer)
	}
}

func TestDSARExportFiltersTenantDataAndSharedCustomerDeletionRequiresReview(t *testing.T) {
	db := setupPrivacyTenantTestDB(t)
	now := time.Now()
	customer := models.Customer{Name: "Shared Customer", PrimaryEmail: "shared@example.com", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	tickets := []models.Ticket{
		{TicketNo: "SHARED-T1", TenantID: 1, CustomerID: customer.ID, Description: "tenant-one-private", Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TicketNo: "SHARED-T2", TenantID: 2, CustomerID: customer.ID, Description: "tenant-two-private", Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatalf("create shared tickets: %v", err)
	}
	request, err := DSARService.SubmitDSAR(context.Background(), "1", "customer", formatID(customer.ID), customer.PrimaryEmail, "access")
	if err != nil {
		t.Fatalf("submit shared customer DSAR: %v", err)
	}
	exported, _, err := DSARService.ExecuteDataExport(context.Background(), "1", request.ID)
	if err != nil {
		t.Fatalf("export DSAR: %v", err)
	}
	if !strings.Contains(string(exported), "tenant-one-private") || strings.Contains(string(exported), "tenant-two-private") {
		t.Fatalf("export crossed tenant boundary: %s", exported)
	}
	if strings.Contains(string(exported), "shared@example.com") {
		t.Fatalf("shared legacy customer PII leaked through tenant export: %s", exported)
	}

	deleteRequest, err := DSARService.SubmitDSAR(context.Background(), "1", "customer", formatID(customer.ID), customer.PrimaryEmail, "delete")
	if err != nil {
		t.Fatalf("submit delete DSAR: %v", err)
	}
	if err := DSARService.ExecuteDataDeletion(context.Background(), "1", deleteRequest.ID); err == nil {
		t.Fatal("expected shared customer deletion to require manual review")
	}
	var persisted models.Customer
	if err := db.First(&persisted, customer.ID).Error; err != nil {
		t.Fatalf("reload shared customer: %v", err)
	}
	if persisted.PrimaryEmail != customer.PrimaryEmail {
		t.Fatalf("shared customer was mutated: %+v", persisted)
	}
}

func TestDSARVisitorExportAndDeletionCoverPrivacyConsentReceipts(t *testing.T) {
	db := setupPrivacyTenantTestDB(t)
	now := time.Now()
	session := models.CustomerEntrySession{
		TenantID: 1, VisitorID: "visitor-private", Locale: "en", State: "active",
		VisitorTokenHash: "visitor-private-token", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create entry session: %v", err)
	}
	consent := models.CustomerPrivacyConsent{
		TenantID: 1, EntrySessionID: session.ID, VisitorID: session.VisitorID,
		PolicyVersion: "2026-08-01", RequiredAccepted: true, AnalyticsAccepted: true,
		Locale: "en", IPAddress: "203.0.113.8", UserAgent: "private-agent",
		RequestID: "private-request", ReceiptHash: "private-receipt", ConsentedAt: now, CreatedAt: now,
	}
	foreignConsent := models.CustomerPrivacyConsent{
		TenantID: 2, EntrySessionID: session.ID + 1, VisitorID: session.VisitorID,
		PolicyVersion: "2026-08-01", RequiredAccepted: true,
		IPAddress: "198.51.100.9", ReceiptHash: "foreign-receipt", ConsentedAt: now, CreatedAt: now,
	}
	if err := db.Create(&[]models.CustomerPrivacyConsent{consent, foreignConsent}).Error; err != nil {
		t.Fatalf("create privacy consents: %v", err)
	}

	exportRequest, err := DSARService.SubmitDSAR(context.Background(), "1", "visitor", session.VisitorID, "", "access")
	if err != nil {
		t.Fatalf("submit visitor export: %v", err)
	}
	exported, _, err := DSARService.ExecuteDataExport(context.Background(), "1", exportRequest.ID)
	if err != nil {
		t.Fatalf("export visitor data: %v", err)
	}
	if !strings.Contains(string(exported), "203.0.113.8") || strings.Contains(string(exported), "198.51.100.9") {
		t.Fatalf("privacy consent export crossed tenant boundary: %s", exported)
	}

	deleteRequest, err := DSARService.SubmitDSAR(context.Background(), "1", "visitor", session.VisitorID, "", "delete")
	if err != nil {
		t.Fatalf("submit visitor deletion: %v", err)
	}
	if err := DSARService.ExecuteDataDeletion(context.Background(), "1", deleteRequest.ID); err != nil {
		t.Fatalf("delete visitor data: %v", err)
	}
	var persisted models.CustomerPrivacyConsent
	if err := db.Where("receipt_hash = ?", consent.ReceiptHash).First(&persisted).Error; err != nil {
		t.Fatalf("reload privacy consent: %v", err)
	}
	if persisted.VisitorID != "ANONYMIZED_"+session.VisitorID || persisted.IPAddress != "" || persisted.UserAgent != "" || persisted.RequestID != "" {
		t.Fatalf("privacy consent was not anonymized: %+v", persisted)
	}
	var persistedForeign models.CustomerPrivacyConsent
	if err := db.Where("receipt_hash = ?", foreignConsent.ReceiptHash).First(&persistedForeign).Error; err != nil {
		t.Fatalf("reload foreign privacy consent: %v", err)
	}
	if persistedForeign.IPAddress != foreignConsent.IPAddress || persistedForeign.VisitorID != foreignConsent.VisitorID {
		t.Fatalf("foreign privacy consent was changed: %+v", persistedForeign)
	}
}

func TestDataBreachOperationsAreTenantScoped(t *testing.T) {
	db := setupPrivacyTenantTestDB(t)
	records := []*models.DataBreachRecord{
		{TenantID: "1", BreachType: "data_leak", Description: "tenant one"},
		{TenantID: "2", BreachType: "data_leak", Description: "tenant two"},
	}
	for _, record := range records {
		if err := DataBreachService.RecordDataBreach(context.Background(), record); err != nil {
			t.Fatalf("create breach: %v", err)
		}
	}
	if err := DataBreachService.NotifySupervisoryAuthority(context.Background(), "1", records[1].ID); err == nil {
		t.Fatal("expected foreign breach notification to fail")
	}
	var foreign models.DataBreachRecord
	if err := db.Where("id = ?", records[1].ID).First(&foreign).Error; err != nil {
		t.Fatalf("reload foreign breach: %v", err)
	}
	if foreign.SupervisoryAuthorityNotifiedAt != nil {
		t.Fatalf("foreign breach was notified: %+v", foreign)
	}
	list, total, err := DataBreachService.ListBreaches(context.Background(), "1", 1, 20)
	if err != nil {
		t.Fatalf("list breaches: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].TenantID != "1" {
		t.Fatalf("breach list crossed tenant boundary: total=%d list=%+v", total, list)
	}
}

func TestDSARDeletionRollsBackAllChangesOnFailure(t *testing.T) {
	db := setupPrivacyTenantTestDB(t)
	now := time.Now()
	customer := models.Customer{
		Name: "Rollback Customer", PrimaryEmail: "rollback@example.com", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	ticket := models.Ticket{
		TicketNo: "PRIVACY-ROLLBACK", TenantID: 1, CustomerID: customer.ID,
		Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	request, err := DSARService.SubmitDSAR(context.Background(), "1", "customer", formatID(customer.ID), customer.PrimaryEmail, "delete")
	if err != nil {
		t.Fatalf("submit DSAR: %v", err)
	}
	if err := db.Migrator().DropTable(&models.CustomerContact{}); err != nil {
		t.Fatalf("drop contact table: %v", err)
	}
	if err := DSARService.ExecuteDataDeletion(context.Background(), "1", request.ID); err == nil {
		t.Fatal("expected deletion failure")
	}
	var persisted models.Customer
	if err := db.First(&persisted, customer.ID).Error; err != nil {
		t.Fatalf("reload customer: %v", err)
	}
	if persisted.Name != customer.Name || persisted.PrimaryEmail != customer.PrimaryEmail {
		t.Fatalf("partial anonymization was not rolled back: %+v", persisted)
	}
	var persistedRequest models.DSARRequest
	if err := db.Where("id = ?", request.ID).First(&persistedRequest).Error; err != nil {
		t.Fatalf("reload request: %v", err)
	}
	if persistedRequest.Status != "pending" || persistedRequest.CompletedAt != nil {
		t.Fatalf("failed deletion changed request state: %+v", persistedRequest)
	}
}

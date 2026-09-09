package services

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

type serviceCodeManagementFixture struct {
	Tenant       models.Tenant
	Product      models.Product
	ProductModel models.ProductModel
	Operator     *dto.AuthPrincipal
}

func setupServiceCodeManagementTestDB(t *testing.T) (*gorm.DB, serviceCodeManagementFixture) {
	t.Helper()

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	if err := db.AutoMigrate(
		&models.Tenant{},
		&models.Product{},
		&models.ProductModel{},
		&models.ServiceCodeBatch{},
		&models.ServiceCode{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Now()
	audit := models.AuditFields{
		CreatedAt: now,
		UpdatedAt: now,
	}

	fixture := serviceCodeManagementFixture{
		Tenant: models.Tenant{
			Name:          "Acme Machinery",
			Industry:      "machinery",
			CountryRegion: "US",
			DefaultLocale: "en-US",
			Timezone:      "America/Los_Angeles",
			Status:        enums.StatusOk,
			AuditFields:   audit,
		},
		Operator: &dto.AuthPrincipal{
			UserID:   1,
			Username: "admin",
			Status:   enums.StatusOk,
		},
	}
	if err := db.Create(&fixture.Tenant).Error; err != nil {
		t.Fatalf("create tenant error = %v", err)
	}

	fixture.Product = models.Product{
		TenantID:      fixture.Tenant.ID,
		Code:          "HP",
		Name:          "Hydraulic Press",
		Category:      "press",
		DefaultLocale: "en-US",
		Status:        enums.StatusOk,
		AuditFields:   audit,
	}
	if err := db.Create(&fixture.Product).Error; err != nil {
		t.Fatalf("create product error = %v", err)
	}

	fixture.ProductModel = models.ProductModel{
		TenantID:    fixture.Tenant.ID,
		ProductID:   fixture.Product.ID,
		ModelCode:   "HP-200",
		Name:        "HP 200",
		Status:      enums.StatusOk,
		AuditFields: audit,
	}
	if err := db.Create(&fixture.ProductModel).Error; err != nil {
		t.Fatalf("create product model error = %v", err)
	}

	return db, fixture
}

func TestServiceCodeManagementGenerateCodesRejectsOverIssue(t *testing.T) {
	_, fixture := setupServiceCodeManagementTestDB(t)
	batch := createServiceCodeBatchForTest(t, fixture, "BATCH-OVER", 2)

	codes, err := ServiceCodeManagementService.GenerateCodes(batch.ID, 2, fixture.Operator)
	if err != nil {
		t.Fatalf("GenerateCodes() error = %v", err)
	}
	if len(codes) != 2 {
		t.Fatalf("len(codes) = %d, want 2", len(codes))
	}

	_, err = ServiceCodeManagementService.GenerateCodes(batch.ID, 1, fixture.Operator)
	if err == nil {
		t.Fatalf("over-issued GenerateCodes() error = nil, want error")
	}

	count := repositories.ServiceCodeRepository.Count(sqls.DB(), sqls.NewCnd().Eq("batch_id", batch.ID))
	if count != 2 {
		t.Fatalf("service code count = %d, want 2", count)
	}
	updated := ServiceCodeBatchService.Get(batch.ID)
	if updated.GeneratedCount != 2 {
		t.Fatalf("GeneratedCount = %d, want 2", updated.GeneratedCount)
	}
}

func TestServiceCodeManagementRevokeCodeIsIdempotent(t *testing.T) {
	_, fixture := setupServiceCodeManagementTestDB(t)
	batch := createServiceCodeBatchForTest(t, fixture, "BATCH-REVOKE", 1)
	codes, err := ServiceCodeManagementService.GenerateCodes(batch.ID, 1, fixture.Operator)
	if err != nil {
		t.Fatalf("GenerateCodes() error = %v", err)
	}

	if err := ServiceCodeManagementService.RevokeCode(codes[0].ID, fixture.Operator); err != nil {
		t.Fatalf("first RevokeCode() error = %v", err)
	}
	revoked := repositories.ServiceCodeRepository.Get(sqls.DB(), codes[0].ID)
	if revoked.Status != enums.ServiceCodeStatusRevoked {
		t.Fatalf("Status = %q, want revoked", revoked.Status)
	}
	if revoked.RevokedAt == nil || revoked.RevokedAt.IsZero() {
		t.Fatalf("RevokedAt = %v, want non-zero", revoked.RevokedAt)
	}

	if err := ServiceCodeManagementService.RevokeCode(codes[0].ID, fixture.Operator); err != nil {
		t.Fatalf("second RevokeCode() error = %v", err)
	}
}

func TestServiceCodeBatchCreateRejectsReusedBatchNo(t *testing.T) {
	_, fixture := setupServiceCodeManagementTestDB(t)

	first := createServiceCodeBatchForTest(t, fixture, " dup-batch ", 1)
	_, err := ServiceCodeBatchService.CreateBatch(request.CreateServiceCodeBatchRequest{
		TenantID:       fixture.Tenant.ID,
		BatchNo:        "DUP-BATCH",
		Mode:           string(enums.ServiceCodeModeGeneral),
		ProductID:      fixture.Product.ID,
		ProductModelID: fixture.ProductModel.ID,
		Quantity:       1,
	}, fixture.Operator)
	if err == nil {
		t.Fatalf("duplicate CreateBatch() error = nil, want error")
	}

	if err := ServiceCodeBatchService.DeleteBatch(first.ID, fixture.Operator); err != nil {
		t.Fatalf("DeleteBatch() error = %v", err)
	}
	_, err = ServiceCodeBatchService.CreateBatch(request.CreateServiceCodeBatchRequest{
		TenantID:       fixture.Tenant.ID,
		BatchNo:        "DUP-BATCH",
		Mode:           string(enums.ServiceCodeModeGeneral),
		ProductID:      fixture.Product.ID,
		ProductModelID: fixture.ProductModel.ID,
		Quantity:       1,
	}, fixture.Operator)
	if err == nil {
		t.Fatalf("CreateBatch() after delete error = nil, want error")
	}
}

func createServiceCodeBatchForTest(t *testing.T, fixture serviceCodeManagementFixture, batchNo string, quantity int64) *models.ServiceCodeBatch {
	t.Helper()

	batch, err := ServiceCodeBatchService.CreateBatch(request.CreateServiceCodeBatchRequest{
		TenantID:       fixture.Tenant.ID,
		BatchNo:        batchNo,
		Mode:           string(enums.ServiceCodeModeTraceable),
		ProductID:      fixture.Product.ID,
		ProductModelID: fixture.ProductModel.ID,
		Quantity:       quantity,
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateBatch() error = %v", err)
	}
	return batch
}

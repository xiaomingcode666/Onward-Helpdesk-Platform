package services

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

type serviceCodeResolveFixture struct {
	Tenant       models.Tenant
	ProductLine  models.ProductLine
	Product      models.Product
	ProductModel models.ProductModel
	Device       models.Device
	Traceable    models.ServiceCode
	General      models.ServiceCode
	Revoked      models.ServiceCode
}

func setupServiceCodeResolveTestDB(t *testing.T) (*gorm.DB, serviceCodeResolveFixture) {
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
		&models.ProductLine{},
		&models.Product{},
		&models.ProductModel{},
		&models.ProductServiceProfile{},
		&models.Device{},
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

	fixture := serviceCodeResolveFixture{
		Tenant: models.Tenant{
			Name:          "Acme Machinery",
			Industry:      "machinery",
			CountryRegion: "US",
			DefaultLocale: "en-US",
			Timezone:      "America/Los_Angeles",
			DataRegion:    "us",
			Status:        enums.StatusOk,
			AuditFields:   audit,
		},
	}
	if err := db.Create(&fixture.Tenant).Error; err != nil {
		t.Fatalf("create tenant error = %v", err)
	}

	fixture.ProductLine = models.ProductLine{
		TenantID:    fixture.Tenant.ID,
		Code:        "PRESS",
		Name:        "Press Line",
		Category:    "hydraulic",
		Status:      enums.StatusOk,
		AuditFields: audit,
	}
	if err := db.Create(&fixture.ProductLine).Error; err != nil {
		t.Fatalf("create product line error = %v", err)
	}

	fixture.Product = models.Product{
		TenantID:      fixture.Tenant.ID,
		ProductLineID: fixture.ProductLine.ID,
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

	fixture.Device = models.Device{
		TenantID:       fixture.Tenant.ID,
		DeviceNo:       "DEV-1001",
		ProductID:      fixture.Product.ID,
		ProductModelID: fixture.ProductModel.ID,
		SerialNo:       "HP-2026-0001",
		RegionCode:     "US-WEST",
		Status:         enums.StatusOk,
		AuditFields:    audit,
	}
	if err := db.Create(&fixture.Device).Error; err != nil {
		t.Fatalf("create device error = %v", err)
	}

	traceableBatch := models.ServiceCodeBatch{
		TenantID:       fixture.Tenant.ID,
		BatchNo:        "BATCH-TRACE",
		Mode:           enums.ServiceCodeModeTraceable,
		ProductID:      fixture.Product.ID,
		ProductModelID: fixture.ProductModel.ID,
		Quantity:       1,
		Status:         enums.StatusOk,
		AuditFields:    audit,
	}
	if err := db.Create(&traceableBatch).Error; err != nil {
		t.Fatalf("create traceable batch error = %v", err)
	}

	generalBatch := models.ServiceCodeBatch{
		TenantID:       fixture.Tenant.ID,
		BatchNo:        "BATCH-GENERAL",
		Mode:           enums.ServiceCodeModeGeneral,
		ProductID:      fixture.Product.ID,
		ProductModelID: fixture.ProductModel.ID,
		Quantity:       1,
		Status:         enums.StatusOk,
		AuditFields:    audit,
	}
	if err := db.Create(&generalBatch).Error; err != nil {
		t.Fatalf("create general batch error = %v", err)
	}

	fixture.Traceable = models.ServiceCode{
		TenantID:       fixture.Tenant.ID,
		BatchID:        traceableBatch.ID,
		ServiceCode:    "SC-1001",
		Mode:           enums.ServiceCodeModeTraceable,
		DeviceID:       fixture.Device.ID,
		ProductID:      fixture.Product.ID,
		ProductModelID: fixture.ProductModel.ID,
		Status:         enums.ServiceCodeStatusActive,
		AuditFields:    audit,
	}
	fixture.General = models.ServiceCode{
		TenantID:       fixture.Tenant.ID,
		BatchID:        generalBatch.ID,
		ServiceCode:    "SC-GENERAL",
		Mode:           enums.ServiceCodeModeGeneral,
		ProductID:      fixture.Product.ID,
		ProductModelID: fixture.ProductModel.ID,
		Status:         enums.ServiceCodeStatusActive,
		AuditFields:    audit,
	}
	fixture.Revoked = models.ServiceCode{
		TenantID:       fixture.Tenant.ID,
		BatchID:        traceableBatch.ID,
		ServiceCode:    "SC-REVOKED",
		Mode:           enums.ServiceCodeModeTraceable,
		DeviceID:       fixture.Device.ID,
		ProductID:      fixture.Product.ID,
		ProductModelID: fixture.ProductModel.ID,
		Status:         enums.ServiceCodeStatusRevoked,
		AuditFields:    audit,
	}
	if err := db.Create(&fixture.Traceable).Error; err != nil {
		t.Fatalf("create traceable service code error = %v", err)
	}
	if err := db.Create(&fixture.General).Error; err != nil {
		t.Fatalf("create general service code error = %v", err)
	}
	if err := db.Create(&fixture.Revoked).Error; err != nil {
		t.Fatalf("create revoked service code error = %v", err)
	}

	return db, fixture
}

func TestServiceCodeResolveTraceableDevice(t *testing.T) {
	_, fixture := setupServiceCodeResolveTestDB(t)

	resp, err := ServiceCodeResolveService.Resolve(" " + fixture.Traceable.ServiceCode + " ")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if !resp.Valid {
		t.Fatalf("Valid = false, want true: %+v", resp)
	}
	if resp.EntryState != enums.CustomerEntryStateNeedRegister {
		t.Fatalf("EntryState = %q", resp.EntryState)
	}
	if resp.Tenant.Name != fixture.Tenant.Name {
		t.Fatalf("Tenant.Name = %q", resp.Tenant.Name)
	}
	if resp.Product.Name != fixture.Product.Name {
		t.Fatalf("Product.Name = %q", resp.Product.Name)
	}
	if resp.Device.SerialNo != fixture.Device.SerialNo {
		t.Fatalf("Device.SerialNo = %q", resp.Device.SerialNo)
	}
}

func TestServiceCodeResolveBoundDeviceRemainsUsable(t *testing.T) {
	db, fixture := setupServiceCodeResolveTestDB(t)
	if err := db.Model(&models.ServiceCode{}).Where("id = ?", fixture.Traceable.ID).
		Update("status", enums.ServiceCodeStatusBound).Error; err != nil {
		t.Fatal(err)
	}

	resp, err := ServiceCodeResolveService.Resolve(fixture.Traceable.ServiceCode)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !resp.Valid || resp.EntryState != enums.CustomerEntryStateNeedRegister || resp.Device == nil {
		t.Fatalf("bound service code should remain usable: %+v", resp)
	}
}

func TestServiceCodeResolveGeneralCodeNeedsRegister(t *testing.T) {
	_, fixture := setupServiceCodeResolveTestDB(t)

	resp, err := ServiceCodeResolveService.Resolve(fixture.General.ServiceCode)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if !resp.Valid {
		t.Fatalf("Valid = false, want true: %+v", resp)
	}
	if resp.EntryState != enums.CustomerEntryStateNeedRegister {
		t.Fatalf("EntryState = %q", resp.EntryState)
	}
	if resp.Device != nil {
		t.Fatalf("Device = %+v, want nil", resp.Device)
	}
}

func TestServiceCodeResolveRevokedCode(t *testing.T) {
	_, fixture := setupServiceCodeResolveTestDB(t)

	resp, err := ServiceCodeResolveService.Resolve(fixture.Revoked.ServiceCode)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if resp.Valid {
		t.Fatalf("Valid = true, want false")
	}
	if resp.EntryState != enums.CustomerEntryStateRevoked {
		t.Fatalf("EntryState = %q", resp.EntryState)
	}
}

func TestServiceCodeResolveDisabledProductIsInvalid(t *testing.T) {
	db, fixture := setupServiceCodeResolveTestDB(t)

	if err := db.Model(&models.Product{}).
		Where("id = ?", fixture.Product.ID).
		Update("status", enums.StatusDisabled).Error; err != nil {
		t.Fatalf("disable product error = %v", err)
	}

	resp, err := ServiceCodeResolveService.Resolve(fixture.Traceable.ServiceCode)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if resp.Valid {
		t.Fatalf("Valid = true, want false")
	}
	if resp.EntryState != enums.CustomerEntryStateInvalid {
		t.Fatalf("EntryState = %q", resp.EntryState)
	}
}

func TestServiceCodeResolveInvalidCode(t *testing.T) {
	setupServiceCodeResolveTestDB(t)

	resp, err := ServiceCodeResolveService.Resolve("missing")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if resp.Valid {
		t.Fatalf("Valid = true, want false")
	}
	if resp.EntryState != enums.CustomerEntryStateInvalid {
		t.Fatalf("EntryState = %q", resp.EntryState)
	}
}

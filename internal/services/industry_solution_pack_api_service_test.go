package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func TestIndustrySolutionPackDraftImportIsIdempotentTenantScopedAndPublishedImmutable(t *testing.T) {
	db := setupIndustrySolutionPackTestDB(t)
	service := DefaultIndustrySolutionPackService(db)
	operator := industrySolutionOperator(101, 9001)
	req := RailLocomotiveStarterManifest("rail-starter-import-101")

	created, err := service.ImportDraftManifest(context.Background(), 101, req, operator)
	if err != nil {
		t.Fatal(err)
	}
	if created.Pack.Status != models.IndustrySolutionPackStatusDraft || created.Reused || len(created.Resources) != 3 {
		t.Fatalf("unexpected draft import: %+v resources=%d", created, len(created.Resources))
	}
	reused, err := service.ImportDraftManifest(context.Background(), 101, req, operator)
	if err != nil {
		t.Fatal(err)
	}
	if !reused.Reused || reused.Pack.ID != created.Pack.ID {
		t.Fatalf("duplicate import must reuse the same draft: %+v", reused)
	}
	otherTenant, err := service.ListPacks(context.Background(), 102, "", "", 1, 20, industrySolutionOperator(102, 9002))
	if err != nil {
		t.Fatal(err)
	}
	if otherTenant.Total != 0 {
		t.Fatalf("cross-tenant list leaked %d packs", otherTenant.Total)
	}

	published, err := service.Publish(context.Background(), 101, created.Pack.ID, operator)
	if err != nil {
		t.Fatal(err)
	}
	if published.Pack.Status != models.IndustrySolutionPackStatusPublished {
		t.Fatalf("status = %s", published.Pack.Status)
	}
	req.IdempotencyKey = "rail-starter-mutated-101"
	req.Description = "attempted mutation"
	if _, err := service.UpdateDraftManifest(context.Background(), 101, created.Pack.ID, req, operator); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("published pack mutation must fail, got %v", err)
	}
}

func TestIndustrySolutionPackDraftWithApplicationCannotBeOverwritten(t *testing.T) {
	db := setupIndustrySolutionPackTestDB(t)
	service := DefaultIndustrySolutionPackService(db)
	operator := industrySolutionOperator(111, 9101)
	req := RailLocomotiveStarterManifest("rail-starter-import-111")
	created, err := service.ImportDraftManifest(context.Background(), 111, req, operator)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	application := &models.IndustrySolutionPackApplication{
		TenantID: 111, PackID: created.Pack.ID, PackCode: created.Pack.PackCode, PackVersion: created.Pack.Version,
		Version: "1.0.0", Mode: models.IndustrySolutionApplicationModeDryRun,
		ConflictStrategy: models.IndustrySolutionConflictStrategyFail, IdempotencyKey: "historical-draft-audit",
		RequestHash: strings.Repeat("a", 64), Status: models.IndustrySolutionApplicationStatusReady,
		TargetContextJSON: "{}", SummaryJSON: "{}",
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now, CreateUserName: "fixture", UpdateUserName: "fixture"},
	}
	if err := db.Create(application).Error; err != nil {
		t.Fatal(err)
	}
	req.IdempotencyKey = "rail-starter-update-111"
	req.Description = "changed after audited application"
	if _, err := service.UpdateDraftManifest(context.Background(), 111, created.Pack.ID, req, operator); err == nil || !strings.Contains(err.Error(), "audit records") {
		t.Fatalf("audited draft overwrite must fail, got %v", err)
	}
}

func TestDefaultIndustrySolutionPackServiceDryRunAndApplyCreatesConcreteResources(t *testing.T) {
	db := setupIndustrySolutionPackTestDB(t)
	if err := db.AutoMigrate(&models.Product{}, &models.ProductModel{}, &models.FaultTreeNode{}); err != nil {
		t.Fatal(err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() { sqls.SetDB(nil) })
	now := time.Now().UTC()
	product := &models.Product{
		TenantID: 121, Code: "TEST-LOCO", Name: "Synthetic locomotive product", DefaultLocale: "zh-CN",
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now, CreateUserName: "fixture", UpdateUserName: "fixture"},
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatal(err)
	}
	productModel := &models.ProductModel{
		TenantID: 121, ProductID: product.ID, ModelCode: "MODEL-TEST", Name: "Synthetic model", RegionScopeJSON: "[]",
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now, CreateUserName: "fixture", UpdateUserName: "fixture"},
	}
	if err := db.Create(productModel).Error; err != nil {
		t.Fatal(err)
	}

	service := DefaultIndustrySolutionPackService(db)
	operator := industrySolutionOperator(121, 9201)
	draft, err := service.ImportRailLocomotiveStarter(context.Background(), 121, "rail-starter-import-121", operator)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(context.Background(), 121, draft.Pack.ID, operator); err != nil {
		t.Fatal(err)
	}
	validation, err := service.Validate(context.Background(), 121, draft.Pack.ID, operator)
	if err != nil || !validation.Valid {
		t.Fatalf("validation = %+v, err=%v", validation, err)
	}
	request := IndustrySolutionPackApplyRequest{
		TenantID: 121, PackID: draft.Pack.ID, TargetProductID: product.ID, TargetProductModelID: productModel.ID,
		ConflictStrategy: models.IndustrySolutionConflictStrategyFail, IdempotencyKey: "rail-starter-dry-run-121",
	}
	dryRun, err := service.DryRun(context.Background(), request, operator)
	if err != nil {
		t.Fatal(err)
	}
	if dryRun.Application.Status != models.IndustrySolutionApplicationStatusReady || len(dryRun.Items) != 3 {
		t.Fatalf("unexpected dry run: %+v items=%d", dryRun.Application, len(dryRun.Items))
	}
	request.IdempotencyKey = "rail-starter-apply-121"
	applied, err := service.Apply(context.Background(), request, operator)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Application.Status != models.IndustrySolutionApplicationStatusSucceeded || applied.Application.SucceededCount != 3 {
		t.Fatalf("unexpected apply result: %+v items=%+v", applied.Application, applied.Items)
	}
	var evalCount, instructionCount, stepCount, faultCount int64
	if err := db.Model(&models.DiagnosisEvalCase{}).Where("tenant_id = ? AND pack_id = ?", 121, draft.Pack.ID).Count(&evalCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.ARWorkInstruction{}).Where("tenant_id = ? AND pack_id = ?", 121, draft.Pack.ID).Count(&instructionCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.ARWorkStep{}).Where("tenant_id = ?", 121).Count(&stepCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.FaultTreeNode{}).Where("tenant_id = ? AND product_id = ?", 121, product.ID).Count(&faultCount).Error; err != nil {
		t.Fatal(err)
	}
	if evalCount != 1 || instructionCount != 1 || stepCount != 3 || faultCount != 1 {
		t.Fatalf("created eval=%d instruction=%d steps=%d fault=%d", evalCount, instructionCount, stepCount, faultCount)
	}
	detail, err := service.GetApplicationDetail(context.Background(), 121, applied.Application.ID, operator)
	if err != nil || len(detail.Items) != 3 {
		t.Fatalf("application detail items=%d err=%v", len(detail.Items), err)
	}
}

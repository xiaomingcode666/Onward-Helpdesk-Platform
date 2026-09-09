package services

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestProductKnowledgeBindingResolvePrefersModelAndLocaleRegion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:product_knowledge_binding_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqls.SetDB(db)
	if err := db.AutoMigrate(&models.Tenant{}, &models.Product{}, &models.ProductModel{}, &models.KnowledgeBase{}, &models.ProductKnowledgeBinding{}); err != nil {
		t.Fatal(err)
	}
	tenant := &models.Tenant{Name: "Tenant", Status: enums.StatusOk}
	product := &models.Product{TenantID: 1, Code: "P", Name: "Product", Status: enums.StatusOk}
	model := &models.ProductModel{TenantID: 1, ProductID: 1, ModelCode: "M", Name: "Model", Status: enums.StatusOk}
	for _, item := range []any{tenant, product, model} {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"product", "model-generic", "model-locale-region"} {
		if err := db.Create(&models.KnowledgeBase{TenantID: tenant.ID, Name: name, Status: enums.StatusOk}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, binding := range []*models.ProductKnowledgeBinding{
		{TenantID: 1, ProductID: 1, KnowledgeBaseID: 1, ScopeType: "product", Locale: "", RegionCode: "", SortNo: 30, Status: enums.StatusOk},
		{TenantID: 1, ProductID: 1, ProductModelID: 1, KnowledgeBaseID: 2, ScopeType: "model", Locale: "", RegionCode: "", SortNo: 20, Status: enums.StatusOk},
		{TenantID: 1, ProductID: 1, ProductModelID: 1, KnowledgeBaseID: 3, ScopeType: "model", Locale: "zh-CN", RegionCode: "CN", SortNo: 10, Status: enums.StatusOk},
	} {
		if err := repositories.ProductKnowledgeBindingRepository.Create(db, binding); err != nil {
			t.Fatal(err)
		}
	}
	result, err := ProductKnowledgeBindingService.Resolve(request.ResolveProductKnowledgeBindingRequest{TenantID: 1, ProductID: 1, ProductModelID: 1, Locale: "zh-CN", RegionCode: "CN"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 || result[0].Name != "model-locale-region" || result[1].Name != "model-generic" || result[2].Name != "product" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProductKnowledgeBindingCreateRejectsCrossTenantKnowledgeBaseAndOperator(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:product_knowledge_binding_tenant_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqls.SetDB(db)
	if err := db.AutoMigrate(&models.Tenant{}, &models.Product{}, &models.ProductModel{}, &models.KnowledgeBase{}, &models.ProductKnowledgeBinding{}); err != nil {
		t.Fatal(err)
	}
	tenantA := &models.Tenant{Name: "Tenant A", Status: enums.StatusOk}
	tenantB := &models.Tenant{Name: "Tenant B", Status: enums.StatusOk}
	if err := db.Create(tenantA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(tenantB).Error; err != nil {
		t.Fatal(err)
	}
	productA := &models.Product{TenantID: tenantA.ID, Code: "A", Name: "Product A", Status: enums.StatusOk}
	productB := &models.Product{TenantID: tenantB.ID, Code: "B", Name: "Product B", Status: enums.StatusOk}
	if err := repositories.ProductRepository.Create(db, productA); err != nil {
		t.Fatal(err)
	}
	if err := repositories.ProductRepository.Create(db, productB); err != nil {
		t.Fatal(err)
	}
	kbA := &models.KnowledgeBase{TenantID: tenantA.ID, Name: "KB A", Status: enums.StatusOk}
	kbB := &models.KnowledgeBase{TenantID: tenantB.ID, Name: "KB B", Status: enums.StatusOk}
	if err := db.Create(kbA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(kbB).Error; err != nil {
		t.Fatal(err)
	}
	operatorA := &dto.AuthPrincipal{TenantID: tenantA.ID, UserID: 1001, Username: "tenant-a-admin"}

	if _, err := ProductKnowledgeBindingService.Create(request.CreateProductKnowledgeBindingRequest{
		TenantID:        tenantA.ID,
		ProductID:       productA.ID,
		KnowledgeBaseID: kbB.ID,
	}, operatorA); err == nil {
		t.Fatal("Create() error = nil, want cross-tenant knowledge base rejection")
	}
	if _, err := ProductKnowledgeBindingService.Create(request.CreateProductKnowledgeBindingRequest{
		TenantID:        tenantB.ID,
		ProductID:       productB.ID,
		KnowledgeBaseID: kbB.ID,
	}, operatorA); err == nil {
		t.Fatal("Create() error = nil, want cross-tenant operator rejection")
	}
	if _, err := ProductKnowledgeBindingService.Create(request.CreateProductKnowledgeBindingRequest{
		TenantID:        tenantA.ID,
		ProductID:       productA.ID,
		KnowledgeBaseID: kbA.ID,
	}, operatorA); err != nil {
		t.Fatalf("Create() same-tenant binding error = %v", err)
	}
}

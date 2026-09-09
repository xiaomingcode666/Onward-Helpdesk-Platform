package services_test

import (
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

func setupKnowledgeResolverTestDB(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "knowledge-resolver-test.db")
	db, err := bootstrap.InitDB(config.DBConfig{
		Type:         "sqlite",
		DSN:          "file:" + dbPath + "?_busy_timeout=5000",
		MaxIdleConns: 1,
		MaxOpenConns: 1,
	})
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
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
		&models.KnowledgeBase{},
		&models.ProductKnowledgeBinding{},
		&models.ProductKnowledgeLink{},
		&models.KnowledgeDocument{},
		&models.KnowledgeFAQ{},
		&models.KnowledgeCandidate{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
}

func TestProductKnowledgeResolverFallsBackToProductDefaultLanguage(t *testing.T) {
	setupKnowledgeResolverTestDB(t)

	tenant := &models.Tenant{Name: "fallback-language-tenant", Status: enums.StatusOk}
	if err := sqls.DB().Create(tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	product := &models.Product{
		TenantID: tenant.ID, Code: "P-ZH", Name: "Chinese Product", DefaultLocale: "zh-CN", Status: enums.StatusOk,
	}
	if err := repositories.ProductRepository.Create(sqls.DB(), product); err != nil {
		t.Fatalf("create product: %v", err)
	}
	kb := &models.KnowledgeBase{TenantID: tenant.ID, Name: "Chinese KB", Status: enums.StatusOk}
	if err := sqls.DB().Create(kb).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	if err := repositories.ProductKnowledgeBindingRepository.Create(sqls.DB(), &models.ProductKnowledgeBinding{
		TenantID: tenant.ID, ProductID: product.ID, KnowledgeBaseID: kb.ID, ScopeType: "product", Status: enums.StatusOk,
	}); err != nil {
		t.Fatalf("create product knowledge binding: %v", err)
	}
	document := &models.KnowledgeDocument{
		TenantID: tenant.ID, KnowledgeBaseID: kb.ID, Title: "中文维修知识", ReviewStatus: "published",
		Language: "zh-CN", Status: enums.StatusOk, PublishedRevisionID: 801,
	}
	if err := repositories.KnowledgeDocumentRepository.Create(sqls.DB(), document); err != nil {
		t.Fatalf("create document: %v", err)
	}
	if err := repositories.ProductKnowledgeLinkRepository.Create(sqls.DB(), &models.ProductKnowledgeLink{
		TenantID: tenant.ID, ProductID: product.ID, KnowledgeBaseID: kb.ID, KnowledgeEntryID: document.ID,
		LinkType: "document", Language: "zh-CN", Visibility: "public", PublishStatus: "published", Status: enums.StatusOk,
	}); err != nil {
		t.Fatalf("create knowledge link: %v", err)
	}

	scope, err := services.ProductKnowledgeResolver.ResolveScope(dto.KnowledgeScopeContext{
		TenantID: tenant.ID, ProductID: product.ID, Locale: "en-US", Audience: "customer",
	})
	if err != nil {
		t.Fatalf("ResolveScope() error = %v", err)
	}
	expectedEntryKey := "document:" + strconv.FormatInt(document.ID, 10)
	if len(scope.EntryKeys) != 1 || scope.EntryKeys[0] != expectedEntryKey {
		t.Fatalf("EntryKeys = %v, want product-default language entry %q", scope.EntryKeys, expectedEntryKey)
	}
	for _, language := range []string{"en-US", "en", "zh-CN", "zh", "default"} {
		if !slices.Contains(scope.Languages, language) {
			t.Fatalf("Languages = %v, want %q", scope.Languages, language)
		}
	}
}

func TestProductKnowledgeResolverTenantScopeExcludesProductKnowledge(t *testing.T) {
	setupKnowledgeResolverTestDB(t)

	tenant := &models.Tenant{Name: "tenant-knowledge", DefaultLocale: "zh-CN", Status: enums.StatusOk}
	if err := sqls.DB().Create(tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	tenantKB := &models.KnowledgeBase{
		TenantID: tenant.ID, Name: "Tenant KB", AccessScope: string(enums.KnowledgeBaseAccessScopeTenant), Status: enums.StatusOk,
	}
	productKB := &models.KnowledgeBase{
		TenantID: tenant.ID, Name: "Product KB", AccessScope: string(enums.KnowledgeBaseAccessScopeProduct), Status: enums.StatusOk,
	}
	if err := sqls.DB().Create(tenantKB).Error; err != nil {
		t.Fatalf("create tenant knowledge base: %v", err)
	}
	if err := sqls.DB().Create(productKB).Error; err != nil {
		t.Fatalf("create product knowledge base: %v", err)
	}
	tenantDocument := &models.KnowledgeDocument{
		TenantID: tenant.ID, KnowledgeBaseID: tenantKB.ID, Title: "General service", ReviewStatus: "published",
		Language: "zh-CN", Status: enums.StatusOk, PublishedRevisionID: 901,
	}
	productDocument := &models.KnowledgeDocument{
		TenantID: tenant.ID, KnowledgeBaseID: productKB.ID, Title: "Product secret", ReviewStatus: "published",
		Language: "zh-CN", Status: enums.StatusOk, PublishedRevisionID: 902,
	}
	if err := repositories.KnowledgeDocumentRepository.Create(sqls.DB(), tenantDocument); err != nil {
		t.Fatalf("create tenant document: %v", err)
	}
	if err := repositories.KnowledgeDocumentRepository.Create(sqls.DB(), productDocument); err != nil {
		t.Fatalf("create product document: %v", err)
	}

	scope, err := services.ProductKnowledgeResolver.ResolveTenantScope(dto.KnowledgeScopeContext{
		TenantID: tenant.ID, ProductID: 99, ProductModelID: 98, DeviceID: 97, ServiceCodeID: 96,
		Locale: "en-US", Audience: "customer",
	})
	if err != nil {
		t.Fatalf("ResolveTenantScope() error = %v", err)
	}
	if scope.Context.ProductID != 0 || scope.Context.ProductModelID != 0 || scope.Context.DeviceID != 0 || scope.Context.ServiceCodeID != 0 {
		t.Fatalf("tenant scope retained product context: %+v", scope.Context)
	}
	if !slices.Equal(scope.KnowledgeBaseIDs, []int64{tenantKB.ID}) || !slices.Equal(scope.TenantSharedKnowledgeBaseIDs, []int64{tenantKB.ID}) {
		t.Fatalf("tenant knowledge bases = %v / %v, want [%d]", scope.KnowledgeBaseIDs, scope.TenantSharedKnowledgeBaseIDs, tenantKB.ID)
	}
	wantEntry := "document:" + strconv.FormatInt(tenantDocument.ID, 10)
	if !slices.Equal(scope.EntryKeys, []string{wantEntry}) || !slices.Equal(scope.RevisionIDs, []int64{901}) {
		t.Fatalf("tenant scope entries = %v revisions = %v", scope.EntryKeys, scope.RevisionIDs)
	}
}

func TestProductKnowledgeResolverTenantScopeDefaultLocaleIncludesChineseKnowledge(t *testing.T) {
	setupKnowledgeResolverTestDB(t)

	tenant := &models.Tenant{Name: "tenant-default-locale", DefaultLocale: "en", Status: enums.StatusOk}
	if err := sqls.DB().Create(tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	tenantKB := &models.KnowledgeBase{
		TenantID: tenant.ID, Name: "Tenant General KB", AccessScope: string(enums.KnowledgeBaseAccessScopeTenant), Status: enums.StatusOk,
	}
	if err := sqls.DB().Create(tenantKB).Error; err != nil {
		t.Fatalf("create tenant knowledge base: %v", err)
	}
	document := &models.KnowledgeDocument{
		TenantID: tenant.ID, KnowledgeBaseID: tenantKB.ID, Title: "海外售后支持时间", ReviewStatus: "published",
		Language: "zh-CN", Status: enums.StatusOk, PublishedRevisionID: 9901,
	}
	if err := repositories.KnowledgeDocumentRepository.Create(sqls.DB(), document); err != nil {
		t.Fatalf("create tenant document: %v", err)
	}

	scope, err := services.ProductKnowledgeResolver.ResolveTenantScope(dto.KnowledgeScopeContext{
		TenantID: tenant.ID,
		Locale:   "default",
		Audience: "customer",
	})
	if err != nil {
		t.Fatalf("ResolveTenantScope() error = %v", err)
	}
	wantEntry := "document:" + strconv.FormatInt(document.ID, 10)
	if !slices.Contains(scope.Languages, "zh-CN") || !slices.Contains(scope.Languages, "zh") {
		t.Fatalf("Languages = %v, want Chinese fallback languages", scope.Languages)
	}
	if !slices.Equal(scope.EntryKeys, []string{wantEntry}) || !slices.Equal(scope.RevisionIDs, []int64{9901}) {
		t.Fatalf("tenant scope entries = %v revisions = %v", scope.EntryKeys, scope.RevisionIDs)
	}
}

func TestProductKnowledgeResolverResolveScopeCustomerUsesPublishedVisibleEntries(t *testing.T) {
	setupKnowledgeResolverTestDB(t)

	tenant := &models.Tenant{Name: "resolver-tenant", Status: enums.StatusOk}
	if err := sqls.DB().Create(tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	product := &models.Product{TenantID: tenant.ID, Code: "P1", Name: "Product 1", Status: enums.StatusOk}
	if err := repositories.ProductRepository.Create(sqls.DB(), product); err != nil {
		t.Fatalf("create product: %v", err)
	}
	model := &models.ProductModel{TenantID: tenant.ID, ProductID: product.ID, ModelCode: "M1", Name: "Model 1", Status: enums.StatusOk}
	if err := repositories.ProductModelRepository.Create(sqls.DB(), model); err != nil {
		t.Fatalf("create model: %v", err)
	}
	kb := &models.KnowledgeBase{TenantID: tenant.ID, Name: "KB", Status: enums.StatusOk}
	if err := sqls.DB().Create(kb).Error; err != nil {
		t.Fatalf("create kb: %v", err)
	}
	if err := repositories.ProductKnowledgeBindingRepository.Create(sqls.DB(), &models.ProductKnowledgeBinding{
		TenantID:        tenant.ID,
		ProductID:       product.ID,
		ProductModelID:  model.ID,
		KnowledgeBaseID: kb.ID,
		ScopeType:       "model",
		Locale:          "en-US",
		Status:          enums.StatusOk,
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	visibleDoc := &models.KnowledgeDocument{
		TenantID:            tenant.ID,
		KnowledgeBaseID:     kb.ID,
		Title:               "Visible",
		Content:             "Visible content",
		ReviewStatus:        "published",
		Language:            "en-US",
		Status:              enums.StatusOk,
		PublishedRevisionID: 501,
	}
	if err := repositories.KnowledgeDocumentRepository.Create(sqls.DB(), visibleDoc); err != nil {
		t.Fatalf("create visible document: %v", err)
	}
	hiddenDoc := &models.KnowledgeDocument{
		TenantID:            tenant.ID,
		KnowledgeBaseID:     kb.ID,
		Title:               "Hidden",
		Content:             "Hidden content",
		ReviewStatus:        "published",
		Language:            "en-US",
		Status:              enums.StatusOk,
		PublishedRevisionID: 502,
	}
	if err := repositories.KnowledgeDocumentRepository.Create(sqls.DB(), hiddenDoc); err != nil {
		t.Fatalf("create hidden document: %v", err)
	}

	if err := repositories.ProductKnowledgeLinkRepository.Create(sqls.DB(), &models.ProductKnowledgeLink{
		TenantID:         tenant.ID,
		ProductID:        product.ID,
		ProductModelID:   model.ID,
		KnowledgeBaseID:  kb.ID,
		KnowledgeEntryID: visibleDoc.ID,
		LinkType:         "document",
		Language:         "en-US",
		Visibility:       "customer",
		PublishStatus:    "published",
		Status:           enums.StatusOk,
	}); err != nil {
		t.Fatalf("create visible link: %v", err)
	}
	if err := repositories.ProductKnowledgeLinkRepository.Create(sqls.DB(), &models.ProductKnowledgeLink{
		TenantID:         tenant.ID,
		ProductID:        product.ID,
		ProductModelID:   model.ID,
		KnowledgeBaseID:  kb.ID,
		KnowledgeEntryID: hiddenDoc.ID,
		LinkType:         "document",
		Language:         "en-US",
		Visibility:       "internal",
		PublishStatus:    "published",
		Status:           enums.StatusOk,
	}); err != nil {
		t.Fatalf("create hidden link: %v", err)
	}

	scope, err := services.ProductKnowledgeResolver.ResolveScope(dto.KnowledgeScopeContext{
		TenantID:       tenant.ID,
		ProductID:      product.ID,
		ProductModelID: model.ID,
		Locale:         "en-US",
		Audience:       "customer",
	})
	if err != nil {
		t.Fatalf("ResolveScope() error = %v", err)
	}
	expectedEntryKey := "document:" + strconv.FormatInt(visibleDoc.ID, 10)
	if len(scope.EntryKeys) != 1 || scope.EntryKeys[0] != expectedEntryKey {
		t.Fatalf("EntryKeys = %#v, want only customer-visible document entry", scope.EntryKeys)
	}
	if len(scope.RevisionIDs) != 1 || scope.RevisionIDs[0] != 501 {
		t.Fatalf("RevisionIDs = %#v, want [501]", scope.RevisionIDs)
	}
}

func TestProductKnowledgeResolverKeepsCandidateModelKnowledgeOutOfProductOnlyScope(t *testing.T) {
	setupKnowledgeResolverTestDB(t)

	tenant := &models.Tenant{Name: "candidate-model-scope-tenant", Status: enums.StatusOk}
	if err := sqls.DB().Create(tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	product := &models.Product{TenantID: tenant.ID, Code: "SCOPE-P1", Name: "Scope Product", Status: enums.StatusOk}
	if err := repositories.ProductRepository.Create(sqls.DB(), product); err != nil {
		t.Fatalf("create product: %v", err)
	}
	model := &models.ProductModel{TenantID: tenant.ID, ProductID: product.ID, ModelCode: "SCOPE-M1", Name: "Scope Model", Status: enums.StatusOk}
	if err := repositories.ProductModelRepository.Create(sqls.DB(), model); err != nil {
		t.Fatalf("create model: %v", err)
	}
	kb := &models.KnowledgeBase{TenantID: tenant.ID, Name: "Scope KB", Status: enums.StatusOk}
	if err := sqls.DB().Create(kb).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	if err := repositories.ProductKnowledgeBindingRepository.Create(sqls.DB(), &models.ProductKnowledgeBinding{
		TenantID: tenant.ID, ProductID: product.ID, KnowledgeBaseID: kb.ID, ScopeType: "product", Status: enums.StatusOk,
	}); err != nil {
		t.Fatalf("create product knowledge binding: %v", err)
	}

	generalDoc := &models.KnowledgeDocument{
		TenantID: tenant.ID, KnowledgeBaseID: kb.ID, Title: "General repair", ReviewStatus: "published",
		Language: "default", Status: enums.StatusOk, PublishedRevisionID: 801,
	}
	if err := repositories.KnowledgeDocumentRepository.Create(sqls.DB(), generalDoc); err != nil {
		t.Fatalf("create general document: %v", err)
	}
	candidate := &models.KnowledgeCandidate{
		TenantID: tenant.ID, ProductID: product.ID, ProductModelID: model.ID,
		ReviewStatus: string(enums.KnowledgeCandidateReviewStatusApproved), Status: enums.StatusOk,
	}
	if err := repositories.KnowledgeCandidateRepository.Create(sqls.DB(), candidate); err != nil {
		t.Fatalf("create knowledge candidate: %v", err)
	}
	modelDoc := &models.KnowledgeDocument{
		TenantID: tenant.ID, KnowledgeBaseID: kb.ID, Title: "Model repair", ReviewStatus: "published",
		Language: "default", Status: enums.StatusOk, PublishedRevisionID: 802,
		SourceType: "knowledge_candidate", SourceReferenceID: candidate.ID,
	}
	if err := repositories.KnowledgeDocumentRepository.Create(sqls.DB(), modelDoc); err != nil {
		t.Fatalf("create model candidate document: %v", err)
	}
	if err := repositories.ProductKnowledgeLinkRepository.Create(sqls.DB(), &models.ProductKnowledgeLink{
		TenantID: tenant.ID, ProductID: product.ID, KnowledgeBaseID: kb.ID, KnowledgeEntryID: modelDoc.ID,
		LinkType: "document", Language: "default", Visibility: "customer", PublishStatus: "published", Status: enums.StatusOk,
	}); err != nil {
		t.Fatalf("create legacy product-wide candidate link: %v", err)
	}

	productOnlyScope, err := services.ProductKnowledgeResolver.ResolveScope(dto.KnowledgeScopeContext{
		TenantID: tenant.ID, ProductID: product.ID, Locale: "default", Audience: "customer",
	})
	if err != nil {
		t.Fatalf("ResolveScope(product only) error = %v", err)
	}
	assertResolvedKnowledgeEntries(t, productOnlyScope, map[string]int64{
		"document:" + strconv.FormatInt(generalDoc.ID, 10): generalDoc.PublishedRevisionID,
	})

	modelScope, err := services.ProductKnowledgeResolver.ResolveScope(dto.KnowledgeScopeContext{
		TenantID: tenant.ID, ProductID: product.ID, ProductModelID: model.ID, Locale: "default", Audience: "customer",
	})
	if err != nil {
		t.Fatalf("ResolveScope(model) error = %v", err)
	}
	assertResolvedKnowledgeEntries(t, modelScope, map[string]int64{
		"document:" + strconv.FormatInt(generalDoc.ID, 10): generalDoc.PublishedRevisionID,
		"document:" + strconv.FormatInt(modelDoc.ID, 10):   modelDoc.PublishedRevisionID,
	})
}

func TestProductKnowledgeResolverExcludesUnapprovedCandidateDocuments(t *testing.T) {
	setupKnowledgeResolverTestDB(t)

	tenant := &models.Tenant{Name: "candidate-review-gate-tenant", Status: enums.StatusOk}
	if err := sqls.DB().Create(tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	product := &models.Product{TenantID: tenant.ID, Code: "REVIEW-P1", Name: "Review Product", Status: enums.StatusOk}
	if err := repositories.ProductRepository.Create(sqls.DB(), product); err != nil {
		t.Fatalf("create product: %v", err)
	}
	kb := &models.KnowledgeBase{TenantID: tenant.ID, Name: "Review KB", Status: enums.StatusOk}
	if err := sqls.DB().Create(kb).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	if err := repositories.ProductKnowledgeBindingRepository.Create(sqls.DB(), &models.ProductKnowledgeBinding{
		TenantID: tenant.ID, ProductID: product.ID, KnowledgeBaseID: kb.ID, ScopeType: "product", Status: enums.StatusOk,
	}); err != nil {
		t.Fatalf("create product knowledge binding: %v", err)
	}

	cases := []struct {
		name                 string
		reviewStatus         enums.KnowledgeCandidateReviewStatus
		requiresReassessment bool
		sourceType           string
		visible              bool
	}{
		{name: "approved", reviewStatus: enums.KnowledgeCandidateReviewStatusApproved, sourceType: "knowledge_candidate", visible: true},
		{name: "pending", reviewStatus: enums.KnowledgeCandidateReviewStatusPending, sourceType: "knowledge_candidate"},
		{name: "rejected legacy alias", reviewStatus: enums.KnowledgeCandidateReviewStatusRejected, sourceType: "knowledge_entry"},
		{name: "requires reassessment", reviewStatus: enums.KnowledgeCandidateReviewStatusApproved, requiresReassessment: true, sourceType: "knowledge_candidate"},
	}
	want := make(map[string]int64, 1)
	for index, tt := range cases {
		candidate := &models.KnowledgeCandidate{
			TenantID: tenant.ID, ProductID: product.ID, ReviewStatus: string(tt.reviewStatus),
			RequiresReassessment: tt.requiresReassessment, Status: enums.StatusOk,
		}
		if err := repositories.KnowledgeCandidateRepository.Create(sqls.DB(), candidate); err != nil {
			t.Fatalf("create %s candidate: %v", tt.name, err)
		}
		document := &models.KnowledgeDocument{
			TenantID: tenant.ID, KnowledgeBaseID: kb.ID, Title: tt.name, ReviewStatus: "published",
			Language: "default", Status: enums.StatusOk, PublishedRevisionID: int64(900 + index),
			SourceType: tt.sourceType, SourceReferenceID: candidate.ID,
		}
		if err := repositories.KnowledgeDocumentRepository.Create(sqls.DB(), document); err != nil {
			t.Fatalf("create %s document: %v", tt.name, err)
		}
		if err := repositories.ProductKnowledgeLinkRepository.Create(sqls.DB(), &models.ProductKnowledgeLink{
			TenantID: tenant.ID, ProductID: product.ID, KnowledgeBaseID: kb.ID, KnowledgeEntryID: document.ID,
			LinkType: "document", Language: "default", Visibility: "customer", PublishStatus: "published", Status: enums.StatusOk,
		}); err != nil {
			t.Fatalf("create %s link: %v", tt.name, err)
		}
		if tt.visible {
			want["document:"+strconv.FormatInt(document.ID, 10)] = document.PublishedRevisionID
		}
	}

	resolved, err := services.ProductKnowledgeResolver.ResolveScope(dto.KnowledgeScopeContext{
		TenantID: tenant.ID, ProductID: product.ID, Locale: "default", Audience: "customer",
	})
	if err != nil {
		t.Fatalf("ResolveScope() error = %v", err)
	}
	assertResolvedKnowledgeEntries(t, resolved, want)
}

func TestProductKnowledgeResolverResolveScopeIsolatesSharedKnowledgeBaseByProductAndTenant(t *testing.T) {
	setupKnowledgeResolverTestDB(t)

	tenantA := &models.Tenant{Name: "tenant-a", Status: enums.StatusOk}
	tenantB := &models.Tenant{Name: "tenant-b", Status: enums.StatusOk}
	if err := sqls.DB().Create(tenantA).Error; err != nil {
		t.Fatalf("create tenant A: %v", err)
	}
	if err := sqls.DB().Create(tenantB).Error; err != nil {
		t.Fatalf("create tenant B: %v", err)
	}

	productA := &models.Product{TenantID: tenantA.ID, Code: "A", Name: "Product A", Status: enums.StatusOk}
	productB := &models.Product{TenantID: tenantA.ID, Code: "B", Name: "Product B", Status: enums.StatusOk}
	otherTenantProduct := &models.Product{TenantID: tenantB.ID, Code: "A", Name: "Other Tenant Product", Status: enums.StatusOk}
	for _, product := range []*models.Product{productA, productB, otherTenantProduct} {
		if err := repositories.ProductRepository.Create(sqls.DB(), product); err != nil {
			t.Fatalf("create product %s: %v", product.Name, err)
		}
	}

	sharedKB := &models.KnowledgeBase{TenantID: tenantA.ID, Name: "Shared Tenant A KB", Status: enums.StatusOk}
	otherTenantKB := &models.KnowledgeBase{TenantID: tenantB.ID, Name: "Tenant B KB", Status: enums.StatusOk}
	if err := sqls.DB().Create(sharedKB).Error; err != nil {
		t.Fatalf("create shared knowledge base: %v", err)
	}
	if err := sqls.DB().Create(otherTenantKB).Error; err != nil {
		t.Fatalf("create other tenant knowledge base: %v", err)
	}

	for _, binding := range []*models.ProductKnowledgeBinding{
		{TenantID: tenantA.ID, ProductID: productA.ID, KnowledgeBaseID: sharedKB.ID, ScopeType: "product", Status: enums.StatusOk},
		{TenantID: tenantA.ID, ProductID: productB.ID, KnowledgeBaseID: sharedKB.ID, ScopeType: "product", Status: enums.StatusOk},
		{TenantID: tenantB.ID, ProductID: otherTenantProduct.ID, KnowledgeBaseID: otherTenantKB.ID, ScopeType: "product", Status: enums.StatusOk},
	} {
		if err := repositories.ProductKnowledgeBindingRepository.Create(sqls.DB(), binding); err != nil {
			t.Fatalf("create product knowledge binding: %v", err)
		}
	}

	docA := &models.KnowledgeDocument{TenantID: tenantA.ID, KnowledgeBaseID: sharedKB.ID, Title: "Product A", ReviewStatus: "published", Language: "en-US", Status: enums.StatusOk, PublishedRevisionID: 701}
	docB := &models.KnowledgeDocument{TenantID: tenantA.ID, KnowledgeBaseID: sharedKB.ID, Title: "Product B", ReviewStatus: "published", Language: "en-US", Status: enums.StatusOk, PublishedRevisionID: 702}
	otherTenantDoc := &models.KnowledgeDocument{TenantID: tenantB.ID, KnowledgeBaseID: otherTenantKB.ID, Title: "Other Tenant", ReviewStatus: "published", Language: "en-US", Status: enums.StatusOk, PublishedRevisionID: 703}
	sharedDoc := &models.KnowledgeDocument{TenantID: tenantA.ID, KnowledgeBaseID: sharedKB.ID, Title: "Shared General Guidance", ReviewStatus: "published", Language: "en-US", Status: enums.StatusOk, PublishedRevisionID: 704}
	for _, document := range []*models.KnowledgeDocument{docA, docB, otherTenantDoc, sharedDoc} {
		if err := repositories.KnowledgeDocumentRepository.Create(sqls.DB(), document); err != nil {
			t.Fatalf("create document %s: %v", document.Title, err)
		}
	}

	for _, link := range []*models.ProductKnowledgeLink{
		{TenantID: tenantA.ID, ProductID: productA.ID, KnowledgeBaseID: sharedKB.ID, KnowledgeEntryID: docA.ID, LinkType: "document", Language: "en-US", Visibility: "customer", PublishStatus: "published", Status: enums.StatusOk},
		{TenantID: tenantA.ID, ProductID: productB.ID, KnowledgeBaseID: sharedKB.ID, KnowledgeEntryID: docB.ID, LinkType: "document", Language: "en-US", Visibility: "customer", PublishStatus: "published", Status: enums.StatusOk},
		{TenantID: tenantB.ID, ProductID: otherTenantProduct.ID, KnowledgeBaseID: otherTenantKB.ID, KnowledgeEntryID: otherTenantDoc.ID, LinkType: "document", Language: "en-US", Visibility: "customer", PublishStatus: "published", Status: enums.StatusOk},
	} {
		if err := repositories.ProductKnowledgeLinkRepository.Create(sqls.DB(), link); err != nil {
			t.Fatalf("create product knowledge link: %v", err)
		}
	}

	scope, err := services.ProductKnowledgeResolver.ResolveScope(dto.KnowledgeScopeContext{
		TenantID:  tenantA.ID,
		ProductID: productA.ID,
		Locale:    "en-US",
		Audience:  "customer",
	})
	if err != nil {
		t.Fatalf("ResolveScope() error = %v", err)
	}
	assertResolvedKnowledgeEntries(t, scope, map[string]int64{
		"document:" + strconv.FormatInt(docA.ID, 10):      docA.PublishedRevisionID,
		"document:" + strconv.FormatInt(sharedDoc.ID, 10): sharedDoc.PublishedRevisionID,
	})

	productBScope, err := services.ProductKnowledgeResolver.ResolveScope(dto.KnowledgeScopeContext{
		TenantID:  tenantA.ID,
		ProductID: productB.ID,
		Locale:    "en-US",
		Audience:  "customer",
	})
	if err != nil {
		t.Fatalf("ResolveScope(product B) error = %v", err)
	}
	assertResolvedKnowledgeEntries(t, productBScope, map[string]int64{
		"document:" + strconv.FormatInt(docB.ID, 10):      docB.PublishedRevisionID,
		"document:" + strconv.FormatInt(sharedDoc.ID, 10): sharedDoc.PublishedRevisionID,
	})

	otherTenantScope, err := services.ProductKnowledgeResolver.ResolveScope(dto.KnowledgeScopeContext{
		TenantID:  tenantB.ID,
		ProductID: otherTenantProduct.ID,
		Locale:    "en-US",
		Audience:  "customer",
	})
	if err != nil {
		t.Fatalf("ResolveScope(other tenant) error = %v", err)
	}
	assertResolvedKnowledgeEntries(t, otherTenantScope, map[string]int64{
		"document:" + strconv.FormatInt(otherTenantDoc.ID, 10): otherTenantDoc.PublishedRevisionID,
	})

	agentScope, err := services.ProductKnowledgeResolver.ResolveScope(dto.KnowledgeScopeContext{
		TenantID:  tenantA.ID,
		ProductID: productA.ID,
		Locale:    "en-US",
		Audience:  "agent",
	})
	if err != nil {
		t.Fatalf("ResolveScope(agent) error = %v", err)
	}
	wantAgentEntries := map[string]bool{
		"document:" + strconv.FormatInt(docA.ID, 10):      true,
		"document:" + strconv.FormatInt(sharedDoc.ID, 10): true,
	}
	if len(agentScope.EntryKeys) != len(wantAgentEntries) {
		t.Fatalf("agent EntryKeys = %#v, want product A plus unlinked shared knowledge", agentScope.EntryKeys)
	}
	for _, entryKey := range agentScope.EntryKeys {
		if !wantAgentEntries[entryKey] {
			t.Fatalf("agent EntryKeys contains product B or other tenant entry: %#v", agentScope.EntryKeys)
		}
	}
}

func TestProductKnowledgeResolverCombinesCurrentProductAndTenantSharedBases(t *testing.T) {
	setupKnowledgeResolverTestDB(t)

	tenant := &models.Tenant{Name: "combined-scope-tenant", Status: enums.StatusOk}
	otherTenant := &models.Tenant{Name: "combined-scope-other-tenant", Status: enums.StatusOk}
	if err := sqls.DB().Create([]*models.Tenant{tenant, otherTenant}).Error; err != nil {
		t.Fatalf("create tenants: %v", err)
	}
	productA := &models.Product{TenantID: tenant.ID, Code: "COMBO-A", Name: "Combo A", Status: enums.StatusOk}
	productB := &models.Product{TenantID: tenant.ID, Code: "COMBO-B", Name: "Combo B", Status: enums.StatusOk}
	if err := sqls.DB().Create([]*models.Product{productA, productB}).Error; err != nil {
		t.Fatalf("create products: %v", err)
	}
	productAKB := &models.KnowledgeBase{TenantID: tenant.ID, Name: "Product A KB", AccessScope: string(enums.KnowledgeBaseAccessScopeProduct), Status: enums.StatusOk}
	productBKB := &models.KnowledgeBase{TenantID: tenant.ID, Name: "Product B KB", AccessScope: string(enums.KnowledgeBaseAccessScopeProduct), Status: enums.StatusOk}
	sharedKB := &models.KnowledgeBase{TenantID: tenant.ID, Name: "Tenant Shared KB", AccessScope: string(enums.KnowledgeBaseAccessScopeTenant), Status: enums.StatusOk}
	foreignKB := &models.KnowledgeBase{TenantID: otherTenant.ID, Name: "Foreign Shared KB", AccessScope: string(enums.KnowledgeBaseAccessScopeTenant), Status: enums.StatusOk}
	if err := sqls.DB().Create([]*models.KnowledgeBase{productAKB, productBKB, sharedKB, foreignKB}).Error; err != nil {
		t.Fatalf("create knowledge bases: %v", err)
	}
	for _, binding := range []*models.ProductKnowledgeBinding{
		{TenantID: tenant.ID, ProductID: productA.ID, KnowledgeBaseID: productAKB.ID, ScopeType: "product", Status: enums.StatusOk},
		{TenantID: tenant.ID, ProductID: productB.ID, KnowledgeBaseID: productBKB.ID, ScopeType: "product", Status: enums.StatusOk},
	} {
		if err := sqls.DB().Create(binding).Error; err != nil {
			t.Fatalf("create product knowledge binding: %v", err)
		}
	}
	productADoc := &models.KnowledgeDocument{TenantID: tenant.ID, KnowledgeBaseID: productAKB.ID, Title: "Product A only", ReviewStatus: "published", Language: "default", Status: enums.StatusOk, PublishedRevisionID: 901}
	productBDoc := &models.KnowledgeDocument{TenantID: tenant.ID, KnowledgeBaseID: productBKB.ID, Title: "Product B only", ReviewStatus: "published", Language: "default", Status: enums.StatusOk, PublishedRevisionID: 902}
	sharedDoc := &models.KnowledgeDocument{TenantID: tenant.ID, KnowledgeBaseID: sharedKB.ID, Title: "Tenant shared", ReviewStatus: "published", Language: "default", Status: enums.StatusOk, PublishedRevisionID: 903}
	foreignDoc := &models.KnowledgeDocument{TenantID: otherTenant.ID, KnowledgeBaseID: foreignKB.ID, Title: "Foreign tenant", ReviewStatus: "published", Language: "default", Status: enums.StatusOk, PublishedRevisionID: 904}
	if err := sqls.DB().Create([]*models.KnowledgeDocument{productADoc, productBDoc, sharedDoc, foreignDoc}).Error; err != nil {
		t.Fatalf("create knowledge documents: %v", err)
	}

	resolved, err := services.ProductKnowledgeResolver.ResolveScope(dto.KnowledgeScopeContext{
		TenantID: tenant.ID, ProductID: productA.ID, Locale: "default", Audience: "customer",
	})
	if err != nil {
		t.Fatalf("ResolveScope() error = %v", err)
	}
	assertResolvedKnowledgeEntries(t, resolved, map[string]int64{
		"document:" + strconv.FormatInt(productADoc.ID, 10): productADoc.PublishedRevisionID,
		"document:" + strconv.FormatInt(sharedDoc.ID, 10):   sharedDoc.PublishedRevisionID,
	})
	if !slices.Equal(resolved.ProductKnowledgeBaseIDs, []int64{productAKB.ID}) {
		t.Fatalf("ProductKnowledgeBaseIDs = %v, want [%d]", resolved.ProductKnowledgeBaseIDs, productAKB.ID)
	}
	if !slices.Equal(resolved.TenantSharedKnowledgeBaseIDs, []int64{sharedKB.ID}) {
		t.Fatalf("TenantSharedKnowledgeBaseIDs = %v, want [%d]", resolved.TenantSharedKnowledgeBaseIDs, sharedKB.ID)
	}
}

func assertResolvedKnowledgeEntries(t *testing.T, scope *dto.ResolvedKnowledgeScope, expected map[string]int64) {
	t.Helper()
	if scope == nil {
		t.Fatal("resolved scope is nil")
	}
	if len(scope.EntryKeys) != len(expected) {
		t.Fatalf("EntryKeys = %#v, want %d entries", scope.EntryKeys, len(expected))
	}
	if len(scope.RevisionIDs) != len(expected) {
		t.Fatalf("RevisionIDs = %#v, want %d revisions", scope.RevisionIDs, len(expected))
	}
	revisionSet := make(map[int64]struct{}, len(scope.RevisionIDs))
	for _, revisionID := range scope.RevisionIDs {
		revisionSet[revisionID] = struct{}{}
	}
	for _, entryKey := range scope.EntryKeys {
		revisionID, ok := expected[entryKey]
		if !ok {
			t.Fatalf("EntryKeys contains an entry outside the expected tenant/product scope: %#v", scope.EntryKeys)
		}
		if _, ok := revisionSet[revisionID]; !ok {
			t.Fatalf("RevisionIDs = %#v, missing revision %d for %s", scope.RevisionIDs, revisionID, entryKey)
		}
	}
}

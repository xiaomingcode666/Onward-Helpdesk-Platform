package services

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

type productCenterFixture struct {
	Tenant   models.Tenant
	Operator *dto.AuthPrincipal
}

func setupProductCenterTestDB(t *testing.T) (*gorm.DB, productCenterFixture) {
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
		&models.Sub2APITenantAccount{},
		&models.ProductLine{},
		&models.Product{},
		&models.ProductModule{},
		&models.PartnerCompany{},
		&models.Department{},
		&models.AgentTeam{},
		&models.ProductServiceProfile{},
		&models.ProductKnowledgeBinding{},
		&models.ProductKnowledgeLink{},
		&models.ProductManualFile{},
		&models.KnowledgeBase{},
		&models.KnowledgeDocument{},
		&models.KnowledgeFAQ{},
		&models.KnowledgeCandidate{},
		&models.AIAgent{},
		&models.SkillDefinition{},
		&models.AIWorkflow{},
		&models.AIWorkflowVersion{},
		&models.AIAgentRelease{},
		&models.KnowledgeRevision{},
		&models.ProductModel{},
		&models.Device{},
		&models.ProductFaultStatsDaily{},
		&models.ProductQualitySignal{},
		&models.Ticket{},
		&models.DomainEvent{},
		&models.OutboxRecord{},
	); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Now()
	tenant := models.Tenant{
		Name:          "Acme Machinery",
		Industry:      "machinery",
		CountryRegion: "US",
		DefaultLocale: "en-US",
		Timezone:      "America/Los_Angeles",
		Status:        enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("create tenant error = %v", err)
	}
	tokenExpiresAt := now.Add(time.Hour)
	if err := db.Create(&models.Sub2APITenantAccount{
		TenantID:                tenant.ID,
		Sub2APIAccountID:        "test-sub2api-account",
		AccountName:             tenant.Name,
		LoginEmail:              "tenant@example.com",
		LoginPasswordCiphertext: "encrypted-password",
		AccessTokenExpiresAt:    &tokenExpiresAt,
		AccountStatus:           "active",
		ProvisionStatus:         "active",
		Balance:                 21,
		DefaultKeyID:            "test-default-key",
		DefaultKeyName:          "RHD-DEFAULT-TEST",
		DefaultKeyCiphertext:    "encrypted-default-key",
		DefaultKeyStatus:        "active",
		Status:                  enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}).Error; err != nil {
		t.Fatalf("create tenant AI account error = %v", err)
	}

	return db, productCenterFixture{
		Tenant: tenant,
		Operator: &dto.AuthPrincipal{
			UserID:   1,
			Username: "admin",
			TenantID: tenant.ID,
			Status:   enums.StatusOk,
		},
	}
}

func createProductCenterRawProduct(t *testing.T, db *gorm.DB, tenantID int64, code, name string) *models.Product {
	t.Helper()
	now := time.Now()
	product := &models.Product{
		TenantID:      tenantID,
		Code:          normalizeCode(code),
		Name:          strings.TrimSpace(name),
		DefaultLocale: "en-US",
		Status:        enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if product.Name == "" {
		product.Name = product.Code
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create raw product error = %v", err)
	}
	return product
}

func createProductCenterWorkflowTemplate(t *testing.T, db *gorm.DB, tenantID int64) *models.AIWorkflow {
	t.Helper()
	definition, err := marshalDefinition(validAIWorkflowDefinition())
	if err != nil {
		t.Fatalf("marshal workflow definition: %v", err)
	}
	workflow := &models.AIWorkflow{
		TenantID: tenantID, Code: fmt.Sprintf("profile_workflow_%d", tenantID), Scope: models.AIWorkflowScopeTenant,
		Name: "Product profile workflow", Status: enums.StatusOk, DraftDefinition: definition,
	}
	if err := db.Create(workflow).Error; err != nil {
		t.Fatalf("create workflow template: %v", err)
	}
	version := &models.AIWorkflowVersion{
		WorkflowID: workflow.ID, Version: 1, Status: enums.StatusOk, Definition: definition,
		DefinitionHash: hashDefinition(definition), ReleaseChannel: models.AIWorkflowReleaseChannelStable, SchemaVersion: 1,
	}
	if err := db.Create(version).Error; err != nil {
		t.Fatalf("create workflow version: %v", err)
	}
	if err := db.Model(workflow).Updates(map[string]any{
		"published_version_id": version.ID, "current_stable_version_id": version.ID,
	}).Error; err != nil {
		t.Fatalf("mark workflow stable version: %v", err)
	}
	workflow.PublishedVersionID = version.ID
	workflow.CurrentStableVersionID = version.ID
	return workflow
}

func TestProductServiceCreateRejectsDuplicateTenantCode(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)

	req := request.CreateProductRequest{
		TenantID:      fixture.Tenant.ID,
		Code:          " HP ",
		Name:          " Hydraulic Press ",
		Category:      "press",
		DefaultLocale: "en-US",
	}
	product, err := ProductService.CreateProduct(req, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}
	if product.Code != "HP" {
		t.Fatalf("Code = %q, want HP", product.Code)
	}
	productTeam := ProductSupportOrganizationService.FindProductRepairTeam(db, fixture.Tenant.ID, product.ID)
	if productTeam == nil {
		t.Fatalf("product repair team was not provisioned")
	}
	if productTeam.TeamType != AgentTeamTypeProductRepair || productTeam.ParentID <= 0 || productTeam.DepartmentID <= 0 || !productTeam.SystemManaged {
		t.Fatalf("unexpected product repair team hierarchy: %+v", productTeam)
	}
	profile := repositories.ProductServiceProfileRepository.GetByProductID(db, product.ID)
	if profile == nil || profile.DefaultKnowledgeBaseID <= 0 || profile.DefaultAIAgentID <= 0 {
		t.Fatalf("product create should provision default knowledge base and AI agent: %+v", profile)
	}
	agent := repositories.AIAgentRepository.Get(db, profile.DefaultAIAgentID)
	if agent == nil || agent.TenantID != fixture.Tenant.ID || agent.ProductID != product.ID {
		t.Fatalf("default product AI agent scope mismatch: %+v", agent)
	}
	if agent.HandoffMode != enums.AIAgentHandoffModeDefaultTeamPool || !slices.Contains(utils.SplitInt64s(agent.TeamIDs), productTeam.ID) {
		t.Fatalf("default product AI agent should hand off to product repair team pool: team=%+v agent=%+v", productTeam, agent)
	}
	if !slices.Contains(utils.SplitInt64s(agent.KnowledgeIDs), profile.DefaultKnowledgeBaseID) {
		t.Fatalf("default product AI agent should bind product knowledge base: profile=%+v agent=%+v", profile, agent)
	}

	autoProduct, err := ProductService.CreateProduct(request.CreateProductRequest{
		TenantID:      fixture.Tenant.ID,
		Name:          "Auto Coded Pump",
		DefaultLocale: "zh-CN",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProduct() auto code error = %v", err)
	}
	if !strings.HasPrefix(autoProduct.Code, "PROD-") {
		t.Fatalf("auto product Code = %q, want PROD- prefix", autoProduct.Code)
	}

	_, err = ProductService.CreateProduct(req, fixture.Operator)
	if err == nil {
		t.Fatalf("duplicate CreateProduct() error = nil, want error")
	}
}

func TestProductModelServiceCreateRequiresExistingProduct(t *testing.T) {
	_, fixture := setupProductCenterTestDB(t)

	_, err := ProductModelService.CreateProductModel(request.CreateProductModelRequest{
		TenantID:  fixture.Tenant.ID,
		ProductID: 999,
		ModelCode: "HP-200",
		Name:      "HP 200",
	}, fixture.Operator)
	if err == nil {
		t.Fatalf("CreateProductModel() error = nil, want invalid product error")
	}
}

func TestProductCenterDeviceListUsesServerPaginationAndSearch(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	product := createProductCenterRawProduct(t, db, fixture.Tenant.ID, "PAGE-1", "Paged Product")
	now := time.Now()
	devices := make([]models.Device, 0, 25)
	for index := 1; index <= 25; index++ {
		devices = append(devices, models.Device{
			TenantID: fixture.Tenant.ID, ProductID: product.ID,
			DeviceNo: fmt.Sprintf("DEVICE-%03d", index), SerialNo: fmt.Sprintf("SERIAL-%03d", index),
			Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now.Add(time.Duration(index) * time.Second)},
		})
	}
	if err := db.Create(&devices).Error; err != nil {
		t.Fatal(err)
	}
	page, err := ProductCenterService.ListProductDevices(fixture.Tenant.ID, product.ID, ProductResourceQuery{Page: 2, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 25 || page.Page != 2 || page.PageSize != 10 || len(page.Items) != 10 || !page.HasMore || page.TotalPages != 3 {
		t.Fatalf("unexpected device page: %+v", page)
	}
	filtered, err := ProductCenterService.ListProductDevices(fixture.Tenant.ID, product.ID, ProductResourceQuery{Search: "serial-025"})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Total != 1 || len(filtered.Items) != 1 || filtered.Items[0].DeviceNo != "DEVICE-025" {
		t.Fatalf("unexpected filtered page: %+v", filtered)
	}
}

func TestProductKnowledgePreviewListsRespectLimit(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	product, err := ProductService.CreateProduct(request.CreateProductRequest{
		TenantID: fixture.Tenant.ID,
		Code:     "KNOWLEDGE-PREVIEW",
		Name:     "Knowledge Preview Product",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}
	profile := repositories.ProductServiceProfileRepository.GetByProductID(db, product.ID)
	if profile == nil || profile.DefaultKnowledgeBaseID <= 0 {
		t.Fatalf("default product knowledge base was not provisioned: %+v", profile)
	}

	now := time.Now()
	documents := make([]models.KnowledgeDocument, 0, 12)
	for index := 1; index <= 12; index++ {
		documents = append(documents, models.KnowledgeDocument{
			TenantID:        fixture.Tenant.ID,
			KnowledgeBaseID: profile.DefaultKnowledgeBaseID,
			Title:           fmt.Sprintf("Preview document %d", index),
			ReviewStatus:    "published",
			IndexStatus:     enums.KnowledgeDocumentIndexStatusIndexed,
			Status:          enums.StatusOk,
			AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now.Add(time.Duration(index) * time.Second)},
		})
	}
	if err := db.Create(&documents).Error; err != nil {
		t.Fatalf("create knowledge documents error = %v", err)
	}
	links := make([]models.ProductKnowledgeLink, 0, len(documents))
	for index, document := range documents {
		links = append(links, models.ProductKnowledgeLink{
			TenantID:         fixture.Tenant.ID,
			ProductID:        product.ID,
			KnowledgeBaseID:  profile.DefaultKnowledgeBaseID,
			KnowledgeEntryID: document.ID,
			LinkType:         "knowledge_entry",
			Language:         "en-US",
			PublishStatus:    "published",
			SortNo:           index,
			Status:           enums.StatusOk,
			AuditFields:      models.AuditFields{CreatedAt: now, UpdatedAt: now},
		})
	}
	if err := db.Create(&links).Error; err != nil {
		t.Fatalf("create product knowledge links error = %v", err)
	}

	manuals, err := ProductCenterService.ListEnterpriseManuals(fixture.Tenant.ID, product.ID, 2)
	if err != nil {
		t.Fatalf("ListEnterpriseManuals() error = %v", err)
	}
	if len(manuals) != 2 {
		t.Fatalf("ListEnterpriseManuals() len = %d, want 2", len(manuals))
	}
	documentPreview, err := KnowledgeDocumentService.ListEnterpriseProductDocuments(fixture.Tenant.ID, product.ID, 1, 2)
	if err != nil {
		t.Fatalf("ListEnterpriseProductDocuments() error = %v", err)
	}
	if len(documentPreview.Items) != 2 {
		t.Fatalf("ListEnterpriseProductDocuments() len = %d, want 2", len(documentPreview.Items))
	}
	coverage, err := ProductCenterService.GetProductKnowledgeCoverageDetail(fixture.Tenant.ID, product.ID)
	if err != nil {
		t.Fatalf("GetProductKnowledgeCoverageDetail() error = %v", err)
	}
	if len(coverage.Links) != productKnowledgeCoveragePreviewLimit || coverage.LinkedEntries != int64(len(documents)) {
		t.Fatalf("knowledge coverage preview = %d/%d, want %d/%d", len(coverage.Links), coverage.LinkedEntries, productKnowledgeCoveragePreviewLimit, len(documents))
	}
}

func TestProductModuleServiceRejectsCrossTenantProductAndSupplier(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	product, err := ProductService.CreateProduct(request.CreateProductRequest{
		TenantID: fixture.Tenant.ID,
		Code:     "MODULE-SCOPE",
		Name:     "Module Scope Product",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}

	now := time.Now()
	otherTenant := models.Tenant{Name: "Other Tenant", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&otherTenant).Error; err != nil {
		t.Fatalf("create other tenant: %v", err)
	}
	otherProduct := models.Product{TenantID: otherTenant.ID, Code: "OTHER", Name: "Other Product", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&otherProduct).Error; err != nil {
		t.Fatalf("create other product: %v", err)
	}
	otherSupplier := models.PartnerCompany{TenantID: otherTenant.ID, PartnerNo: "OTHER-SUP", Name: "Other Supplier", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&otherSupplier).Error; err != nil {
		t.Fatalf("create other supplier: %v", err)
	}

	if _, err := ProductModuleService.CreateModule(CreateProductModuleRequest{
		TenantID: fixture.Tenant.ID, ProductID: otherProduct.ID, ModuleCode: "BAD-PRODUCT", Name: "Bad Product",
	}, fixture.Operator); err == nil {
		t.Fatal("CreateModule() accepted a product from another tenant")
	}
	if _, err := ProductModuleService.CreateModule(CreateProductModuleRequest{
		TenantID: fixture.Tenant.ID, ProductID: product.ID, ModuleCode: "BAD-SUPPLIER", Name: "Bad Supplier", DefaultSupplierID: otherSupplier.ID,
	}, fixture.Operator); err == nil {
		t.Fatal("CreateModule() accepted a supplier from another tenant")
	}
}

func TestDeviceServiceCreateRejectsDuplicateTenantDeviceNo(t *testing.T) {
	_, fixture := setupProductCenterTestDB(t)

	product, err := ProductService.CreateProduct(request.CreateProductRequest{
		TenantID:      fixture.Tenant.ID,
		Code:          "HP",
		Name:          "Hydraulic Press",
		Category:      "press",
		DefaultLocale: "en-US",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}
	model, err := ProductModelService.CreateProductModel(request.CreateProductModelRequest{
		TenantID:  fixture.Tenant.ID,
		ProductID: product.ID,
		ModelCode: "HP-200",
		Name:      "HP 200",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProductModel() error = %v", err)
	}

	req := request.CreateDeviceRequest{
		TenantID:       fixture.Tenant.ID,
		DeviceNo:       " DEV-1001 ",
		ProductID:      product.ID,
		ProductModelID: model.ID,
		SerialNo:       "HP-2026-0001",
		RegionCode:     "US-WEST",
		Source:         "manual",
	}
	device, err := DeviceService.CreateDevice(req, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateDevice() error = %v", err)
	}
	if device.DeviceNo != "DEV-1001" {
		t.Fatalf("DeviceNo = %q, want DEV-1001", device.DeviceNo)
	}

	_, err = DeviceService.CreateDevice(req, fixture.Operator)
	if err == nil {
		t.Fatalf("duplicate CreateDevice() error = nil, want error")
	}
}

func TestProductServiceProfileCreateRejectsDuplicateProductID(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	product := createProductCenterRawProduct(t, db, fixture.Tenant.ID, "HP", "Hydraulic Press")
	workflow := createProductCenterWorkflowTemplate(t, db, fixture.Tenant.ID)

	req := request.CreateProductServiceProfileRequest{
		TenantID:               fixture.Tenant.ID,
		ProductID:              product.ID,
		SupportLocalesJSON:     `["en-US"]`,
		SupportRegionsJSON:     `["US"]`,
		WarrantyPolicyJSON:     `{"months":12}`,
		SafetyLevel:            "standard",
		DefaultFlowTemplateID:  workflow.ID,
		DefaultKnowledgeBaseID: 20,
		MeetingEnabled:         true,
		ServicePolicyJSON:      `{"sla":"next-business-day"}`,
	}
	profile, err := ProductServiceProfileService.CreateProductServiceProfile(req, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProductServiceProfile() error = %v", err)
	}
	if profile.ProductID != product.ID {
		t.Fatalf("ProductID = %d, want %d", profile.ProductID, product.ID)
	}
	req.SafetyLevel = "enhanced"
	if err := ProductServiceProfileService.UpdateProductServiceProfile(request.UpdateProductServiceProfileRequest{
		ID: profile.ID, CreateProductServiceProfileRequest: req,
	}, fixture.Operator); err != nil {
		t.Fatalf("UpdateProductServiceProfile() error = %v", err)
	}
	updated := ProductServiceProfileService.Get(profile.ID)
	if updated == nil || updated.SafetyLevel != "enhanced" || updated.DefaultFlowTemplateID != workflow.ID {
		t.Fatalf("updated product service profile = %+v", updated)
	}

	_, err = ProductServiceProfileService.CreateProductServiceProfile(req, fixture.Operator)
	if err == nil {
		t.Fatalf("duplicate CreateProductServiceProfile() error = nil, want error")
	}
}

func TestProductServiceProfileCreateRejectsCrossTenantDefaultWorkflow(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	product := createProductCenterRawProduct(t, db, fixture.Tenant.ID, "FLOW", "Flow Test")
	workflow := createProductCenterWorkflowTemplate(t, db, fixture.Tenant.ID+1)

	_, err := ProductServiceProfileService.CreateProductServiceProfile(request.CreateProductServiceProfileRequest{
		TenantID: fixture.Tenant.ID, ProductID: product.ID, DefaultFlowTemplateID: workflow.ID,
	}, fixture.Operator)
	if err == nil {
		t.Fatal("CreateProductServiceProfile() error = nil, want cross-tenant workflow rejection")
	}
}

func TestProductServiceProfileCreateRejectsInvalidJSON(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	product := createProductCenterRawProduct(t, db, fixture.Tenant.ID, "CNC", "CNC Mill")

	_, err := ProductServiceProfileService.CreateProductServiceProfile(request.CreateProductServiceProfileRequest{
		TenantID:           fixture.Tenant.ID,
		ProductID:          product.ID,
		SupportLocalesJSON: `["en-US"]`,
		SupportRegionsJSON: `[`,
	}, fixture.Operator)
	if err == nil {
		t.Fatalf("CreateProductServiceProfile() error = nil, want invalid json error")
	}
}

func TestProductCenterServiceCreateProductKnowledgeBase(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	product := createProductCenterRawProduct(t, db, fixture.Tenant.ID, "KB-HP", "Knowledge Press")

	profile, err := ProductCenterService.CreateProductKnowledgeBase(
		fixture.Tenant.ID,
		product.ID,
		dto.EnterpriseProductKnowledgeBaseCreateRequest{
			Name:             "Knowledge Press KB",
			Description:      "Product-specific manuals and diagnostics",
			RagflowDatasetID: "rag-kb-hp",
			SupportLocales:   []string{"en-US", "de-DE"},
			SupportRegions:   []string{"EU", "US"},
		},
		fixture.Operator,
	)
	if err != nil {
		t.Fatalf("CreateProductKnowledgeBase() error = %v", err)
	}
	if profile.ServiceProfile.KnowledgeBaseID == nil || *profile.ServiceProfile.KnowledgeBaseID <= 0 {
		t.Fatalf("KnowledgeBaseID = %v, want created id", profile.ServiceProfile.KnowledgeBaseID)
	}
	if profile.ServiceProfile.KnowledgeBaseName != "Knowledge Press KB" {
		t.Fatalf("KnowledgeBaseName = %q, want Knowledge Press KB", profile.ServiceProfile.KnowledgeBaseName)
	}

	kb := repositories.KnowledgeBaseRepository.Get(sqls.DB(), *profile.ServiceProfile.KnowledgeBaseID)
	if kb == nil {
		t.Fatalf("knowledge base not persisted")
	}
	if kb.TenantID != fixture.Tenant.ID {
		t.Fatalf("KnowledgeBase TenantID = %d, want %d", kb.TenantID, fixture.Tenant.ID)
	}
	if kb.AccessScope != string(enums.KnowledgeBaseAccessScopeProduct) {
		t.Fatalf("KnowledgeBase AccessScope = %q, want product", kb.AccessScope)
	}
	if kb.RagflowDatasetID != "rag-kb-hp" {
		t.Fatalf("RagflowDatasetID = %q, want rag-kb-hp", kb.RagflowDatasetID)
	}

	serviceProfile := repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), product.ID)
	if serviceProfile == nil {
		t.Fatalf("product service profile not created")
	}
	if serviceProfile.DefaultKnowledgeBaseID != kb.ID {
		t.Fatalf("DefaultKnowledgeBaseID = %d, want %d", serviceProfile.DefaultKnowledgeBaseID, kb.ID)
	}
	if !strings.Contains(serviceProfile.SupportLocalesJSON, "de-DE") {
		t.Fatalf("SupportLocalesJSON = %q, want de-DE", serviceProfile.SupportLocalesJSON)
	}
	if !strings.Contains(serviceProfile.SupportRegionsJSON, "EU") {
		t.Fatalf("SupportRegionsJSON = %q, want EU", serviceProfile.SupportRegionsJSON)
	}
	assertDefaultProductKnowledgeBinding(t, product.ID, kb.ID, fixture.Tenant.ID)
}

func TestProductCenterServiceCreateProductKnowledgeBaseBindsExistingKnowledgeBase(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	product := createProductCenterRawProduct(t, db, fixture.Tenant.ID, "KB-BIND-EXISTING", "Existing KB Product")

	now := time.Now()
	existingKB := &models.KnowledgeBase{
		TenantID:              fixture.Tenant.ID,
		Name:                  "Existing KB Product 产品知识库",
		Description:           "Created before product-page binding",
		KnowledgeType:         string(enums.KnowledgeBaseTypeDocument),
		AccessScope:           string(enums.KnowledgeBaseAccessScopeProduct),
		Status:                enums.StatusOk,
		DefaultTopK:           10,
		DefaultScoreThreshold: 0.5,
		DefaultRerankLimit:    5,
		ChunkProvider:         string(enums.KnowledgeChunkProviderStructured),
		ChunkTargetTokens:     300,
		ChunkMaxTokens:        400,
		ChunkOverlapTokens:    40,
		AnswerMode:            1,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(existingKB).Error; err != nil {
		t.Fatalf("create existing knowledge base error = %v", err)
	}

	profile, err := ProductCenterService.CreateProductKnowledgeBase(
		fixture.Tenant.ID,
		product.ID,
		dto.EnterpriseProductKnowledgeBaseCreateRequest{
			Name:           existingKB.Name,
			Description:    "Ignored because existing KB should be reused",
			SupportLocales: []string{"en-US"},
		},
		fixture.Operator,
	)
	if err != nil {
		t.Fatalf("CreateProductKnowledgeBase() error = %v", err)
	}

	if profile.ServiceProfile.KnowledgeBaseID == nil || *profile.ServiceProfile.KnowledgeBaseID != existingKB.ID {
		t.Fatalf("KnowledgeBaseID = %v, want existing id %d", profile.ServiceProfile.KnowledgeBaseID, existingKB.ID)
	}

	totalKnowledgeBases := repositories.KnowledgeBaseRepository.Count(sqls.DB(), sqls.NewCnd().Eq("tenant_id", fixture.Tenant.ID).NotEq("status", enums.StatusDeleted))
	if totalKnowledgeBases != 1 {
		t.Fatalf("knowledge base count = %d, want 1", totalKnowledgeBases)
	}

	serviceProfile := repositories.ProductServiceProfileRepository.GetByProductID(sqls.DB(), product.ID)
	if serviceProfile == nil {
		t.Fatalf("product service profile not created")
	}
	if serviceProfile.DefaultKnowledgeBaseID != existingKB.ID {
		t.Fatalf("DefaultKnowledgeBaseID = %d, want %d", serviceProfile.DefaultKnowledgeBaseID, existingKB.ID)
	}
	assertDefaultProductKnowledgeBinding(t, product.ID, existingKB.ID, fixture.Tenant.ID)
}

func assertDefaultProductKnowledgeBinding(t *testing.T, productID, knowledgeBaseID, tenantID int64) {
	t.Helper()
	bindings := repositories.ProductKnowledgeBindingRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", productID).
		Eq("product_model_id", 0).
		Eq("knowledge_base_id", knowledgeBaseID).
		Eq("scope_type", "product").
		Eq("status", enums.StatusOk))
	if len(bindings) != 1 {
		t.Fatalf("default product knowledge binding count = %d, want 1", len(bindings))
	}
	if bindings[0].Locale != "" || bindings[0].RegionCode != "" {
		t.Fatalf("default product knowledge binding = %#v, want global locale and region fallback", bindings[0])
	}
}

func TestProductFaultStatsRespectsRangeQuery(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	now := time.Now().UTC()

	product, err := ProductService.CreateProduct(request.CreateProductRequest{
		TenantID:      fixture.Tenant.ID,
		Code:          "RANGE-HP",
		Name:          "Range Hydraulic Press",
		Category:      "press",
		DefaultLocale: "en-US",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}
	model, err := ProductModelService.CreateProductModel(request.CreateProductModelRequest{
		TenantID:  fixture.Tenant.ID,
		ProductID: product.ID,
		ModelCode: "HP-RANGE-200",
		Name:      "HP Range 200",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProductModel() error = %v", err)
	}

	recentBucket := now.AddDate(0, 0, -10)
	oldBucket := now.AddDate(0, 0, -45)
	if err := db.Create(&[]models.ProductFaultStatsDaily{
		{
			TenantID:       fixture.Tenant.ID,
			ProductID:      product.ID,
			ProductModelID: model.ID,
			FaultCode:      "E-42",
			FaultPart:      "pressure sensor",
			BucketDate:     recentBucket,
			TicketCount:    3,
			CreatedAt:      recentBucket,
			UpdatedAt:      recentBucket,
		},
		{
			TenantID:       fixture.Tenant.ID,
			ProductID:      product.ID,
			ProductModelID: model.ID,
			FaultCode:      "E-99",
			FaultPart:      "motor",
			BucketDate:     oldBucket,
			TicketCount:    7,
			CreatedAt:      oldBucket,
			UpdatedAt:      oldBucket,
		},
	}).Error; err != nil {
		t.Fatalf("create fault stats error = %v", err)
	}
	if err := db.Create(&models.Ticket{
		TenantID:       fixture.Tenant.ID,
		ProductID:      product.ID,
		ProductModelID: model.ID,
		DeviceID:       1001,
		TicketNo:       "TK-RANGE-1",
		Title:          "Recent pressure sensor fault",
		Source:         enums.TicketSourceManual,
		Status:         enums.TicketStatusClosed,
		AuditFields: models.AuditFields{
			CreatedAt: recentBucket,
			UpdatedAt: recentBucket,
		},
	}).Error; err != nil {
		t.Fatalf("create ticket error = %v", err)
	}

	stats, err := ProductCenterService.GetProductFaultStats(fixture.Tenant.ID, product.ID, "30d")
	if err != nil {
		t.Fatalf("GetProductFaultStats() error = %v", err)
	}
	if stats.Range != "30d" {
		t.Fatalf("Range = %q, want 30d", stats.Range)
	}
	if stats.TotalFaults != 3 {
		t.Fatalf("TotalFaults = %d, want 3", stats.TotalFaults)
	}
	if stats.AffectedDevices != 1 {
		t.Fatalf("AffectedDevices = %d, want 1", stats.AffectedDevices)
	}
	if len(stats.Stats) != 1 {
		t.Fatalf("len(Stats) = %d, want 1", len(stats.Stats))
	}
	if stats.Stats[0].FaultType != "E-42" || stats.Stats[0].Part != "pressure sensor" {
		t.Fatalf("Stats[0] = %#v, want E-42 pressure sensor", stats.Stats[0])
	}
}

func TestProductKnowledgeCoverageCountsOnlyPendingCandidates(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	product := createProductCenterRawProduct(t, db, fixture.Tenant.ID, "KB-COVERAGE", "Knowledge Coverage Product")
	now := time.Now()

	candidates := []models.KnowledgeCandidate{
		{TenantID: fixture.Tenant.ID, ProductID: product.ID, ReviewStatus: "pending", Status: enums.StatusOk},
		{TenantID: fixture.Tenant.ID, ProductID: product.ID, ReviewStatus: "approved", Status: enums.StatusOk},
		{TenantID: fixture.Tenant.ID, ProductID: product.ID, ReviewStatus: "rejected", Status: enums.StatusOk},
		{TenantID: fixture.Tenant.ID, ProductID: product.ID, ReviewStatus: "pending", Status: enums.StatusDeleted},
	}
	for i := range candidates {
		candidates[i].AuditFields = models.AuditFields{CreatedAt: now, UpdatedAt: now}
	}
	if err := db.Create(&candidates).Error; err != nil {
		t.Fatalf("create knowledge candidates error = %v", err)
	}

	coverage, err := ProductCenterService.GetProductKnowledgeCoverageDetail(fixture.Tenant.ID, product.ID)
	if err != nil {
		t.Fatalf("GetProductKnowledgeCoverageDetail() error = %v", err)
	}
	if coverage.PendingCandidates != 1 {
		t.Fatalf("PendingCandidates = %d, want 1", coverage.PendingCandidates)
	}
}

func TestProductQualitySignalsPopulateMetricFields(t *testing.T) {
	db, fixture := setupProductCenterTestDB(t)
	now := time.Now().UTC()

	product, err := ProductService.CreateProduct(request.CreateProductRequest{
		TenantID:      fixture.Tenant.ID,
		Code:          "QS-HP",
		Name:          "Quality Signal Hydraulic Press",
		Category:      "press",
		DefaultLocale: "en-US",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProduct() error = %v", err)
	}
	model, err := ProductModelService.CreateProductModel(request.CreateProductModelRequest{
		TenantID:  fixture.Tenant.ID,
		ProductID: product.ID,
		ModelCode: "HP-QS-200",
		Name:      "HP Quality 200",
	}, fixture.Operator)
	if err != nil {
		t.Fatalf("CreateProductModel() error = %v", err)
	}

	if err := db.Create(&models.ProductQualitySignal{
		TenantID:       fixture.Tenant.ID,
		ProductID:      product.ID,
		ProductModelID: model.ID,
		SignalType:     "repeat_fault",
		Severity:       "warning",
		Title:          "Pressure sensor repeat faults",
		Description:    "Pressure sensor faults appeared repeatedly.",
		MetricValue:    12.5,
		SampleCount:    8,
		SourceType:     "fault_stats",
		SourceID:       "E-42",
		OwnerUserID:    42,
		DetectedAt:     now,
		Status:         enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}).Error; err != nil {
		t.Fatalf("create quality signal error = %v", err)
	}

	items, err := ProductCenterService.ListProductQualitySignals(fixture.Tenant.ID, product.ID)
	if err != nil {
		t.Fatalf("ListProductQualitySignals() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	item := items[0]
	if item.Title != "Pressure sensor repeat faults" {
		t.Fatalf("Title = %q, want explicit title", item.Title)
	}
	if item.MetricValue != 12.5 {
		t.Fatalf("MetricValue = %v, want 12.5", item.MetricValue)
	}
	if item.SampleCount != 8 {
		t.Fatalf("SampleCount = %d, want 8", item.SampleCount)
	}
	if item.Source != "fault_stats" {
		t.Fatalf("Source = %q, want fault_stats", item.Source)
	}
}

func TestEnterpriseTicketStatusAndPriorityPreserveCustomerConfirmationState(t *testing.T) {
	ticket := models.Ticket{
		Status:    enums.TicketStatusResolved,
		FaultCode: "configuration",
	}
	if got := MapTicketStatusForEnterprise(ticket.Status); got != "resolved" {
		t.Fatalf("MapTicketStatusForEnterprise() = %q, want resolved", got)
	}
	if got := DeriveTicketPriority(ticket); got != "low" {
		t.Fatalf("DeriveTicketPriority() = %q, want low for resolved ticket", got)
	}
	actions := EnterpriseTicketService.BuildActions(&ticket)
	if actions.CanSaveRepair || actions.CanStartMeeting || !actions.CanClose || !actions.CanReopen {
		t.Fatalf("resolved ticket actions = %+v", actions)
	}
}

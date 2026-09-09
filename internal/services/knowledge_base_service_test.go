package services

import (
	"fmt"
	"strings"
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

func TestBuildKnowledgeBaseModelUsesLowerDefaultScoreThreshold(t *testing.T) {
	item, err := KnowledgeBaseService.buildKnowledgeBaseModel(request.CreateKnowledgeBaseRequest{
		RagflowDatasetID: " dataset-hp-300 ",
	})
	if err != nil {
		t.Fatalf("build knowledge base model failed: %v", err)
	}
	if item.DefaultScoreThreshold != 0.2 {
		t.Fatalf("expected default score threshold 0.2, got %v", item.DefaultScoreThreshold)
	}
	if item.RagflowDatasetID != "dataset-hp-300" {
		t.Fatalf("expected trimmed ragflow dataset id, got %q", item.RagflowDatasetID)
	}
}

func TestDeleteKnowledgeBaseRejectsAIAgentReference(t *testing.T) {
	setupKnowledgeBaseServiceTestDB(t)
	kb := createKnowledgeBaseServiceTestBase(t, "Referenced KB")
	otherKB := createKnowledgeBaseServiceTestBase(t, "Other KB")
	if err := repositories.AIAgentRepository.Create(sqls.DB(), &models.AIAgent{
		Name:         "Support Agent",
		Status:       enums.StatusOk,
		KnowledgeIDs: "12",
	}); err != nil {
		t.Fatalf("create unrelated ai agent: %v", err)
	}
	if err := repositories.AIAgentRepository.Create(sqls.DB(), &models.AIAgent{
		Name:         "Knowledge Agent",
		Status:       enums.StatusOk,
		KnowledgeIDs: fmt.Sprintf("12,%d,%d", kb.ID, otherKB.ID),
	}); err != nil {
		t.Fatalf("create ai agent: %v", err)
	}

	err := KnowledgeBaseService.DeleteKnowledgeBase(kb.ID)
	if err == nil {
		t.Fatal("DeleteKnowledgeBase() error is nil, want referenced knowledge base error")
	}
	if got := err.Error(); !strings.Contains(got, "Knowledge Agent") {
		t.Fatalf("DeleteKnowledgeBase() error = %q, want agent name", got)
	}
	if repositories.KnowledgeBaseRepository.Get(sqls.DB(), kb.ID) == nil {
		t.Fatal("knowledge base was deleted despite ai agent reference")
	}
}

func TestDeleteKnowledgeBaseCascadesContentWhenNotReferenced(t *testing.T) {
	setupKnowledgeBaseServiceTestDB(t)
	kb := createKnowledgeBaseServiceTestBase(t, "Delete KB")
	document := &models.KnowledgeDocument{
		KnowledgeBaseID: kb.ID,
		Title:           "Doc",
		ContentType:     enums.KnowledgeDocumentContentTypeMarkdown,
		Content:         "content",
		Status:          enums.StatusOk,
		IndexStatus:     enums.KnowledgeDocumentIndexStatusIndexed,
	}
	if err := repositories.KnowledgeDocumentRepository.Create(sqls.DB(), document); err != nil {
		t.Fatalf("create document: %v", err)
	}
	faq := &models.KnowledgeFAQ{
		KnowledgeBaseID: kb.ID,
		Question:        "Question",
		Answer:          "Answer",
		Status:          enums.StatusOk,
		IndexStatus:     enums.KnowledgeDocumentIndexStatusIndexed,
	}
	if err := repositories.KnowledgeFAQRepository.Create(sqls.DB(), faq); err != nil {
		t.Fatalf("create faq: %v", err)
	}
	if err := repositories.KnowledgeChunkRepository.BatchCreate(sqls.DB(), []models.KnowledgeChunk{
		{KnowledgeBaseID: kb.ID, DocumentID: document.ID, ChunkNo: 1, Status: enums.StatusOk},
		{KnowledgeBaseID: kb.ID, FaqID: faq.ID, ChunkNo: 1, Status: enums.StatusOk},
	}); err != nil {
		t.Fatalf("create chunks: %v", err)
	}

	if err := KnowledgeBaseService.DeleteKnowledgeBase(kb.ID); err != nil {
		t.Fatalf("DeleteKnowledgeBase() error = %v", err)
	}

	assertKnowledgeBaseServiceTestCount(t, &models.KnowledgeBase{}, "id = ?", kb.ID, 0)
	assertKnowledgeBaseServiceTestCount(t, &models.KnowledgeDocument{}, "knowledge_base_id = ?", kb.ID, 0)
	assertKnowledgeBaseServiceTestCount(t, &models.KnowledgeFAQ{}, "knowledge_base_id = ?", kb.ID, 0)
	assertKnowledgeBaseServiceTestCount(t, &models.KnowledgeChunk{}, "knowledge_base_id = ?", kb.ID, 0)
}

func TestKnowledgeBaseOperatorScopeRejectsCrossTenantAccess(t *testing.T) {
	setupKnowledgeBaseServiceTestDB(t)
	tenantOneKB := createKnowledgeBaseServiceTestBaseForTenant(t, 1, "Tenant One KB")
	tenantTwoKB := createKnowledgeBaseServiceTestBaseForTenant(t, 2, "Tenant Two KB")
	operator := &dto.AuthPrincipal{UserID: 10, Username: "tenant-one", TenantID: 1, DomainType: "enterprise"}

	if item := KnowledgeBaseService.GetForOperator(tenantOneKB.ID, operator); item == nil {
		t.Fatal("tenant operator cannot read its own knowledge base")
	}
	if item := KnowledgeBaseService.GetForOperator(tenantTwoKB.ID, operator); item != nil {
		t.Fatalf("tenant operator read another tenant knowledge base: %+v", item)
	}
	if err := KnowledgeBaseService.UpdateKnowledgeBase(request.UpdateKnowledgeBaseRequest{
		ID: tenantTwoKB.ID,
		CreateKnowledgeBaseRequest: request.CreateKnowledgeBaseRequest{
			Name:          "Cross tenant update",
			KnowledgeType: string(enums.KnowledgeBaseTypeDocument),
		},
	}, operator); err == nil {
		t.Fatal("cross-tenant knowledge base update unexpectedly succeeded")
	}
	if err := KnowledgeBaseService.DeleteKnowledgeBaseForOperator(tenantTwoKB.ID, operator); err == nil {
		t.Fatal("cross-tenant knowledge base delete unexpectedly succeeded")
	}
	if repositories.KnowledgeBaseRepository.Get(sqls.DB(), tenantTwoKB.ID) == nil {
		t.Fatal("cross-tenant knowledge base was deleted")
	}
}

func TestCreateKnowledgeBaseUsesOperatorTenant(t *testing.T) {
	setupKnowledgeBaseServiceTestDB(t)
	operator := &dto.AuthPrincipal{UserID: 20, Username: "tenant-seven", TenantID: 7, DomainType: "enterprise"}

	item, err := KnowledgeBaseService.CreateKnowledgeBase(request.CreateKnowledgeBaseRequest{
		Name:          "Tenant Seven KB",
		KnowledgeType: string(enums.KnowledgeBaseTypeDocument),
	}, operator)
	if err != nil {
		t.Fatalf("CreateKnowledgeBase() error = %v", err)
	}
	if item.TenantID != 7 {
		t.Fatalf("knowledge base tenant = %d, want 7", item.TenantID)
	}
	if item.AccessScope != string(enums.KnowledgeBaseAccessScopeTenant) {
		t.Fatalf("knowledge base access scope = %q, want tenant", item.AccessScope)
	}
}

func setupKnowledgeBaseServiceTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.KnowledgeBase{}, &models.KnowledgeDocument{}, &models.KnowledgeFAQ{}, &models.KnowledgeChunk{}, &models.AIAgent{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
}

func createKnowledgeBaseServiceTestBase(t *testing.T, name string) *models.KnowledgeBase {
	return createKnowledgeBaseServiceTestBaseForTenant(t, 0, name)
}

func createKnowledgeBaseServiceTestBaseForTenant(t *testing.T, tenantID int64, name string) *models.KnowledgeBase {
	t.Helper()
	item := &models.KnowledgeBase{
		TenantID:      tenantID,
		Name:          name,
		KnowledgeType: string(enums.KnowledgeBaseTypeDocument),
		Status:        enums.StatusOk,
	}
	if err := repositories.KnowledgeBaseRepository.Create(sqls.DB(), item); err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	return item
}

func assertKnowledgeBaseServiceTestCount(t *testing.T, model any, query string, arg any, want int64) {
	t.Helper()
	var count int64
	if err := sqls.DB().Model(model).Where(query, arg).Count(&count).Error; err != nil {
		t.Fatalf("count %T: %v", model, err)
	}
	if count != want {
		t.Fatalf("count %T = %d, want %d", model, count, want)
	}
}

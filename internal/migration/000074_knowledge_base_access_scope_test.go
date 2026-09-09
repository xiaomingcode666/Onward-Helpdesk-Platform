package migration

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestEnsureKnowledgeBaseAccessScopeBackfillsWithLeastPrivilege(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.KnowledgeBase{}, &models.ProductKnowledgeBinding{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	shared := &models.KnowledgeBase{TenantID: 1, Name: "Tenant shared", Status: enums.StatusOk}
	product := &models.KnowledgeBase{TenantID: 1, Name: "Product only", Status: enums.StatusOk}
	if err := db.Create([]*models.KnowledgeBase{shared, product}).Error; err != nil {
		t.Fatalf("create knowledge bases: %v", err)
	}
	if err := db.Create(&models.ProductKnowledgeBinding{
		TenantID: 1, ProductID: 10, KnowledgeBaseID: product.ID, ScopeType: "product", Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create product binding: %v", err)
	}
	if err := db.Model(&models.KnowledgeBase{}).Where("id IN ?", []int64{shared.ID, product.ID}).Update("access_scope", "").Error; err != nil {
		t.Fatalf("clear access scope: %v", err)
	}

	if err := ensureKnowledgeBaseAccessScope(db); err != nil {
		t.Fatalf("ensure access scope: %v", err)
	}
	if err := db.First(shared, shared.ID).Error; err != nil {
		t.Fatalf("reload shared knowledge base: %v", err)
	}
	if err := db.First(product, product.ID).Error; err != nil {
		t.Fatalf("reload product knowledge base: %v", err)
	}
	if shared.AccessScope != string(enums.KnowledgeBaseAccessScopeTenant) {
		t.Fatalf("shared access scope = %q, want tenant", shared.AccessScope)
	}
	if product.AccessScope != string(enums.KnowledgeBaseAccessScopeProduct) {
		t.Fatalf("product access scope = %q, want product", product.AccessScope)
	}
	if err := ensureKnowledgeBaseAccessScope(db); err != nil {
		t.Fatalf("migration should be idempotent: %v", err)
	}
}

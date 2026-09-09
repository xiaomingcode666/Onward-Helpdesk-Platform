package retrievers

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestScopeFromConversationUsesEntrySessionLocale(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.CustomerEntrySession{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})

	session := models.CustomerEntrySession{
		TenantID:  7,
		ProductID: 11,
		VisitorID: "visitor-locale",
		Locale:    "zh-CN",
		State:     "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create entry session: %v", err)
	}

	scope := ScopeFromConversation(models.Conversation{
		TenantID:               session.TenantID,
		ProductID:              session.ProductID,
		CustomerEntrySessionID: session.ID,
	})
	if scope.Locale != session.Locale {
		t.Fatalf("scope locale = %q, want %q", scope.Locale, session.Locale)
	}
}

func TestKnowledgeRetrieverProductScopeExcludesTenantSharedByDefault(t *testing.T) {
	retriever := NewKnowledgeRetriever(models.AIAgent{}, dto.KnowledgeScopeContext{TenantID: 7, ProductID: 11})
	if retriever.includeTenantSharedKnowledge() {
		t.Fatal("product scoped runtime retrieval should not include tenant-shared knowledge by default")
	}

	tenantRetriever := NewKnowledgeRetriever(models.AIAgent{}, dto.KnowledgeScopeContext{TenantID: 7})
	if !tenantRetriever.includeTenantSharedKnowledge() {
		t.Fatal("tenant-only runtime retrieval should keep tenant-shared knowledge")
	}
}

package migration

import (
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestEnsureUniqueCustomerIdentitySchemaMergesDuplicateIdentityReferences(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Customer{}, &models.CustomerIdentity{}, &models.Conversation{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropIndex(&models.CustomerIdentity{}, "uk_customer_external"); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX uk_customer_external ON t_customer_identity (customer_id, external_source, external_id)").Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	first := models.Customer{Name: "first", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	second := models.Customer{Name: "second", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	for _, customerID := range []int64{first.ID, second.ID} {
		if err := db.Create(&models.CustomerIdentity{
			CustomerID: customerID, ExternalSource: enums.ExternalSourceGuest, ExternalID: "same-external-id",
			Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	conversation := models.Conversation{CustomerID: second.ID, Status: enums.IMConversationStatusActive, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}

	if err := ensureUniqueCustomerIdentitySchema(db); err != nil {
		t.Fatalf("migrate customer identities: %v", err)
	}
	var identityCount int64
	db.Model(&models.CustomerIdentity{}).Where("external_source = ? AND external_id = ?", enums.ExternalSourceGuest, "same-external-id").Count(&identityCount)
	if identityCount != 1 {
		t.Fatalf("identity count=%d want 1", identityCount)
	}
	var storedConversation models.Conversation
	if err := db.First(&storedConversation, conversation.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedConversation.CustomerID != first.ID {
		t.Fatalf("conversation customer=%d want canonical %d", storedConversation.CustomerID, first.ID)
	}
	duplicate := models.CustomerIdentity{
		CustomerID: second.ID, ExternalSource: enums.ExternalSourceGuest, ExternalID: "same-external-id",
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("expected source and external id uniqueness to reject duplicate mapping")
	}
}

package migration

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestRepairSupplierCollaborationActiveUniqueness(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.TicketSupplierCollaboration{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := repairSupplierCollaborationActiveUniqueness(db); err != nil {
		t.Fatalf("repairSupplierCollaborationActiveUniqueness() error = %v", err)
	}

	now := time.Now()
	first := supplierCollaborationIndexFixture(now, "invited")
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first active collaboration: %v", err)
	}
	duplicate := supplierCollaborationIndexFixture(now, "processing")
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("partial unique index accepted a second active collaboration")
	}
	if err := db.Model(&first).Update("status", "resolved").Error; err != nil {
		t.Fatalf("resolve first collaboration: %v", err)
	}
	second := supplierCollaborationIndexFixture(now.Add(time.Minute), "invited")
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create repeated collaboration after resolution: %v", err)
	}
	if err := db.Model(&second).Update("status", "resolved").Error; err != nil {
		t.Fatalf("resolve repeated collaboration: %v", err)
	}

	var resolvedCount int64
	if err := db.Model(&models.TicketSupplierCollaboration{}).
		Where("tenant_id = ? AND ticket_id = ? AND product_module_id = ? AND partner_company_id = ? AND status = ?", 1, 9, 3, 4, "resolved").
		Count(&resolvedCount).Error; err != nil {
		t.Fatalf("count resolved collaborations: %v", err)
	}
	if resolvedCount != 2 {
		t.Fatalf("resolved collaboration count = %d, want 2", resolvedCount)
	}
	if !db.Migrator().HasIndex(&models.TicketSupplierCollaboration{}, "uk_ticket_supplier_active") {
		t.Fatal("active supplier collaboration partial unique index was not created")
	}
}

func supplierCollaborationIndexFixture(invitedAt time.Time, status string) models.TicketSupplierCollaboration {
	return models.TicketSupplierCollaboration{
		TenantID:         1,
		TicketID:         9,
		ProductID:        2,
		ProductModuleID:  3,
		PartnerCompanyID: 4,
		PartnerAccountID: 5,
		Status:           status,
		InvitedAt:        invitedAt,
		RecordStatus:     enums.StatusOk,
		AuditFields:      models.AuditFields{CreatedAt: invitedAt, UpdatedAt: invitedAt},
	}
}

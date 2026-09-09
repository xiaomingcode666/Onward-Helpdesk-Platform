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

func TestSupplierCollaborationTimeoutIndexAllowsReinvite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.TicketSupplierCollaboration{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := migrateSupplierCollaborationTimeoutIndex(db); err != nil {
		t.Fatalf("migrateSupplierCollaborationTimeoutIndex() error = %v", err)
	}

	now := time.Now()
	first := supplierCollaborationIndexFixture(now, "invited")
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create active collaboration: %v", err)
	}
	duplicate := supplierCollaborationIndexFixture(now.Add(time.Minute), "processing")
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("partial unique index accepted a duplicate active collaboration")
	}
	if err := db.Model(&first).Updates(map[string]any{
		"status":      "timeout",
		"resolved_at": now.Add(2 * time.Hour),
	}).Error; err != nil {
		t.Fatalf("timeout first collaboration: %v", err)
	}
	reinvite := supplierCollaborationIndexFixture(now.Add(3*time.Hour), "invited")
	if err := db.Create(&reinvite).Error; err != nil {
		t.Fatalf("create reinvited collaboration after timeout: %v", err)
	}

	var timedOutCount int64
	if err := db.Model(&models.TicketSupplierCollaboration{}).
		Where("tenant_id = ? AND ticket_id = ? AND product_module_id = ? AND partner_company_id = ? AND status = ? AND record_status = ?",
			1, 9, 3, 4, "timeout", enums.StatusOk).
		Count(&timedOutCount).Error; err != nil {
		t.Fatalf("count timed out collaborations: %v", err)
	}
	if timedOutCount != 1 {
		t.Fatalf("timed out collaboration count = %d, want 1", timedOutCount)
	}
}

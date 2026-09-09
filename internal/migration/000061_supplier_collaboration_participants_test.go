package migration

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMigrateSupplierCollaborationParticipantsBackfillsOwnerHistory(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.TicketSupplierCollaboration{}); err != nil {
		t.Fatalf("migrate collaboration: %v", err)
	}
	now := time.Now()
	resolvedAt := now.Add(time.Hour)
	items := []models.TicketSupplierCollaboration{
		{TenantID: 1, TicketID: 10, ProductID: 20, ProductModuleID: 30, PartnerCompanyID: 40, PartnerAccountID: 50, Status: "processing", InvitedAt: now, RecordStatus: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 1, TicketID: 11, ProductID: 20, ProductModuleID: 31, PartnerCompanyID: 40, PartnerAccountID: 51, Status: "resolved", InvitedAt: now, ResolvedAt: &resolvedAt, RecordStatus: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&items).Error; err != nil {
		t.Fatalf("seed collaborations: %v", err)
	}
	if err := migrateSupplierCollaborationParticipants(db); err != nil {
		t.Fatalf("migrate participants: %v", err)
	}
	var participants []models.TicketSupplierCollaborationParticipant
	if err := db.Order("collaboration_id ASC").Find(&participants).Error; err != nil {
		t.Fatalf("find participants: %v", err)
	}
	if len(participants) != 2 || participants[0].Status != enums.StatusOk || participants[1].Status != enums.StatusDisabled || participants[1].LeftAt == nil {
		t.Fatalf("unexpected participants: %+v", participants)
	}
	if err := migrateSupplierCollaborationParticipants(db); err != nil {
		t.Fatalf("idempotent migration: %v", err)
	}
	var count int64
	if err := db.Model(&models.TicketSupplierCollaborationParticipant{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("participant count = %d, err = %v", count, err)
	}
}

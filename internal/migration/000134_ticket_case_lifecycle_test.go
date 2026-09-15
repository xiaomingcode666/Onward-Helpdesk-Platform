package migration

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
)

func TestTicketCaseMigrationPreservesLegacyFactsAndCanRerun(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if conn, err := db.DB(); err == nil {
			_ = conn.Close()
		}
	})
	sqls.SetDB(db)
	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(&models.Ticket{}); err != nil {
		t.Fatal(err)
	}
	legacy := struct {
		ID                int64              `gorm:"primaryKey;autoIncrement"`
		TicketNo          string             `gorm:"type:varchar(64);not null;default:'';uniqueIndex"`
		Title             string             `gorm:"type:varchar(255);not null;default:'';index"`
		Status            enums.TicketStatus `gorm:"type:varchar(50);not null;default:'pending';index"`
		TenantID          int64              `gorm:"type:bigint;not null;default:0;index"`
		CurrentAssigneeID int64              `gorm:"type:bigint;not null;default:0;index"`
		AcceptedAt        *time.Time         `gorm:"type:timestamp;index"`
		ResolvedAt        *time.Time         `gorm:"type:timestamp;index"`
		models.AuditFields
	}{TicketNo: "BEFORE-134", Title: "历史工单", TenantID: 12, CurrentAssigneeID: 34, Status: enums.TicketStatusPendingCustomerConfirm}
	acceptedAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	legacy.AcceptedAt = &acceptedAt
	if err := db.Table(statement.Table).AutoMigrate(&legacy); err != nil {
		t.Fatal(err)
	}
	if err := db.Table(statement.Table).Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		if err := migrationFuncs[134].Fn(); err != nil {
			t.Fatalf("migration pass %d: %v", pass, err)
		}
		var stored models.Ticket
		if err := db.First(&stored, legacy.ID).Error; err != nil {
			t.Fatal(err)
		}
		if stored.TicketNo != legacy.TicketNo || stored.Title != legacy.Title || stored.Status != legacy.Status || stored.CurrentAssigneeID != legacy.CurrentAssigneeID || stored.AcceptedAt == nil || !stored.AcceptedAt.Equal(acceptedAt) {
			t.Fatalf("migration changed recorded facts: %+v", stored)
		}
		if stored.CaseOwnerID != 0 || stored.CaseStatus != "" || stored.AcknowledgedAt != nil || stored.RestoredAt != nil || stored.ResolvedAt != nil || models.EffectiveTicketCaseStatus(stored) != "waiting" {
			t.Fatalf("migration fabricated historical lifecycle: %+v", stored)
		}
		if !db.Migrator().HasTable(&models.TicketCaseOperation{}) {
			t.Fatal("operation receipt table missing")
		}
	}
	// Simulate a development deployment that has already run version 134.
	if err := db.Migrator().DropColumn(&models.Ticket{}, "CaseResumeTechnicalStatus"); err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		if err := migrationFuncs[135].Fn(); err != nil {
			t.Fatalf("resume-workflow migration pass %d: %v", pass, err)
		}
		var stored models.Ticket
		if err := db.First(&stored, legacy.ID).Error; err != nil {
			t.Fatal(err)
		}
		if stored.CaseResumeTechnicalStatus != "" || stored.Status != legacy.Status || stored.AcceptedAt == nil || !stored.AcceptedAt.Equal(acceptedAt) || stored.CaseOwnerID != 0 {
			t.Fatalf("resume-workflow upgrade changed historical facts: %+v", stored)
		}
	}
}

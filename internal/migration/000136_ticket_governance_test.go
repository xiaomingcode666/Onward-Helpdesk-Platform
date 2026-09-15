package migration

import (
	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"remotehelpdesk/internal/models"
	"testing"
	"time"
)

func TestTicketGovernanceMigrationPreservesSLAAndLegacyPriority(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c, _ := db.DB(); _ = c.Close() })
	sqls.SetDB(db)
	old := struct {
		models.AuditFields
		ID           int64 `gorm:"primaryKey"`
		PriorityCode string
		SLADueAt     *time.Time
		TicketType   string
	}{ID: 1, PriorityCode: "p4", TicketType: "custom-original-type"}
	due := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	old.SLADueAt = &due
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(&models.Ticket{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Table(stmt.Table).AutoMigrate(&old); err != nil {
		t.Fatal(err)
	}
	if err := db.Table(stmt.Table).Create(&old).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := migrationFuncs[136].Fn(); err != nil {
			t.Fatal(err)
		}
		var got models.Ticket
		if err := db.First(&got, 1).Error; err != nil {
			t.Fatal(err)
		}
		if got.PriorityCode != "p4" || got.PriorityLevel != "" || got.CaseType != "" || got.TicketType != old.TicketType || got.SLADueAt == nil || !got.SLADueAt.Equal(due) {
			t.Fatal("migration rewrote historical evidence")
		}
	}
}

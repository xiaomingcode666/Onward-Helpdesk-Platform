package migration

import (
	"path/filepath"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestEnsureProductFaultStatsUniqueIndexDeduplicatesAndSupportsUpsert(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "fault-stats-index.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.ProductFaultStatsDaily{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	if err := db.Migrator().DropIndex(&models.ProductFaultStatsDaily{}, productFaultStatsUniqueIndex); err != nil {
		t.Fatalf("DropIndex() error = %v", err)
	}

	bucketDate := time.Date(2026, time.July, 26, 0, 0, 0, 0, time.Local)
	rows := []models.ProductFaultStatsDaily{
		{TenantID: 1, ProductID: 2, ProductModelID: 3, FaultCode: "F-1", ModuleID: 4, BucketDate: bucketDate, TicketCount: 1, MeetingCount: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{TenantID: 1, ProductID: 2, ProductModelID: 3, FaultCode: "F-1", ModuleID: 4, BucketDate: bucketDate, TicketCount: 2, LowScoreCount: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create duplicate rows: %v", err)
	}

	if err := ensureProductFaultStatsUniqueIndex(db); err != nil {
		t.Fatalf("ensureProductFaultStatsUniqueIndex() error = %v", err)
	}
	if !db.Migrator().HasIndex(&models.ProductFaultStatsDaily{}, productFaultStatsUniqueIndex) {
		t.Fatal("expected repaired product fault statistics index")
	}

	var result models.ProductFaultStatsDaily
	if err := db.First(&result).Error; err != nil {
		t.Fatalf("load deduplicated row: %v", err)
	}
	if result.TicketCount != 3 || result.MeetingCount != 1 || result.LowScoreCount != 1 {
		t.Fatalf("unexpected deduplicated counters: %+v", result)
	}

	if err := repositories.ProductFaultStatsRepository.UpsertDaily(db, &models.ProductFaultStatsDaily{
		TenantID: 1, ProductID: 2, ProductModelID: 3, FaultCode: "F-1", ModuleID: 4,
		BucketDate: bucketDate, TicketCount: 1, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("UpsertDaily() error = %v", err)
	}
	if err := db.First(&result).Error; err != nil {
		t.Fatalf("reload upserted row: %v", err)
	}
	if result.TicketCount != 4 {
		t.Fatalf("TicketCount = %d, want 4", result.TicketCount)
	}
}

package migration

import (
	"testing"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestEnsureAssetFileSizeConstraintRepairsAndProtectsLegacySQLiteTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	table, err := migrationModelTableName(db, &models.Asset{})
	if err != nil {
		t.Fatalf("resolve asset table: %v", err)
	}
	quotedTable := quoteMigrationIdentifier(table)
	if err := db.Exec("CREATE TABLE " + quotedTable + " (id INTEGER PRIMARY KEY, file_size BIGINT NOT NULL DEFAULT 0)").Error; err != nil {
		t.Fatalf("create legacy asset table: %v", err)
	}
	if err := db.Exec("INSERT INTO " + quotedTable + " (id, file_size) VALUES (1, -42)").Error; err != nil {
		t.Fatalf("seed negative legacy asset: %v", err)
	}
	if err := ensureAssetFileSizeConstraint(db); err != nil {
		t.Fatalf("ensure constraint: %v", err)
	}

	var repaired int64
	if err := db.Raw("SELECT file_size FROM " + quotedTable + " WHERE id = 1").Scan(&repaired).Error; err != nil {
		t.Fatalf("read repaired size: %v", err)
	}
	if repaired != 0 {
		t.Fatalf("repaired file size = %d, want 0", repaired)
	}
	if err := db.Exec("INSERT INTO " + quotedTable + " (id, file_size) VALUES (2, -1)").Error; err == nil {
		t.Fatal("expected negative insert to fail")
	}
	if err := db.Exec("UPDATE " + quotedTable + " SET file_size = -1 WHERE id = 1").Error; err == nil {
		t.Fatal("expected negative update to fail")
	}
	if err := ensureAssetFileSizeConstraint(db); err != nil {
		t.Fatalf("constraint should be idempotent: %v", err)
	}
}

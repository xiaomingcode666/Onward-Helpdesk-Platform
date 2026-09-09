package migration

import (
	"fmt"

	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const assetFileSizeConstraint = "ck_asset_file_size_nonnegative"

func init() {
	register(71, "enforce non-negative asset file size", func() error {
		return ensureAssetFileSizeConstraint(sqls.DB())
	})
}

func ensureAssetFileSizeConstraint(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Asset{}) {
		return nil
	}
	table, err := migrationModelTableName(db, &models.Asset{})
	if err != nil {
		return err
	}
	quotedTable := quoteMigrationIdentifier(table)
	if err := db.Exec(fmt.Sprintf("UPDATE %s SET file_size = 0 WHERE file_size < 0", quotedTable)).Error; err != nil {
		return err
	}

	switch db.Dialector.Name() {
	case "postgres":
		if db.Migrator().HasConstraint(&models.Asset{}, assetFileSizeConstraint) {
			return nil
		}
		return db.Migrator().CreateConstraint(&models.Asset{}, assetFileSizeConstraint)
	case "sqlite":
		// SQLite cannot add a CHECK constraint without rebuilding the table. The
		// model CHECK protects new schemas; these triggers protect legacy schemas.
		insertTrigger := quoteMigrationIdentifier(assetFileSizeConstraint + "_insert")
		updateTrigger := quoteMigrationIdentifier(assetFileSizeConstraint + "_update")
		if err := db.Exec(fmt.Sprintf(
			"CREATE TRIGGER IF NOT EXISTS %s BEFORE INSERT ON %s FOR EACH ROW WHEN NEW.file_size < 0 BEGIN SELECT RAISE(ABORT, 'asset file size must not be negative'); END",
			insertTrigger, quotedTable,
		)).Error; err != nil {
			return err
		}
		return db.Exec(fmt.Sprintf(
			"CREATE TRIGGER IF NOT EXISTS %s BEFORE UPDATE OF file_size ON %s FOR EACH ROW WHEN NEW.file_size < 0 BEGIN SELECT RAISE(ABORT, 'asset file size must not be negative'); END",
			updateTrigger, quotedTable,
		)).Error
	default:
		return fmt.Errorf("unsupported database dialect for asset file size constraint: %s", db.Dialector.Name())
	}
}

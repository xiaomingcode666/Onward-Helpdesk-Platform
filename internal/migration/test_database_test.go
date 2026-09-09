package migration

import (
	"gorm.io/gorm"
	"testing"
)

// Close file-backed SQLite before testing removes its temporary directory on Windows.
func openMigrationTestDB(t *testing.T, dialector gorm.Dialector, config *gorm.Config) (*gorm.DB, error) {
	t.Helper()
	db, err := gorm.Open(dialector, config)
	if err != nil {
		return nil, err
	}
	connection, err := db.DB()
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() {
		if err := connection.Close(); err != nil {
			t.Errorf("close migration test database: %v", err)
		}
	})
	return db, nil
}

package migration

import (
	"path/filepath"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRepairPlatformSub2APIHostConfig(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		existing      *models.SystemConfig
		wantHost      string
		wantGroupCode string
	}{
		{name: "missing config", wantHost: productionSub2APIHost, wantGroupCode: "platform_ai"},
		{
			name: "e2e host", wantHost: productionSub2APIHost, wantGroupCode: "platform_ai",
			existing: &models.SystemConfig{ConfigKey: platformSub2APIHostKey, ConfigValue: "http://127.0.0.1:18099/v1", GroupCode: "e2e", Status: enums.StatusOk},
		},
		{
			name: "placeholder host", wantHost: productionSub2APIHost, wantGroupCode: "platform_ai",
			existing: &models.SystemConfig{ConfigKey: platformSub2APIHostKey, ConfigValue: "https://sub2api.example.com/", GroupCode: "platform_ai", Status: enums.StatusOk},
		},
		{
			name: "custom deployment is preserved", wantHost: "https://sub2api.private.example", wantGroupCode: "platform_ai",
			existing: &models.SystemConfig{ConfigKey: platformSub2APIHostKey, ConfigValue: "https://sub2api.private.example", GroupCode: "platform_ai", Status: enums.StatusOk},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "sub2api-host.db")), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.AutoMigrate(&models.SystemConfig{}); err != nil {
				t.Fatal(err)
			}
			if testCase.existing != nil {
				if err := db.Create(testCase.existing).Error; err != nil {
					t.Fatal(err)
				}
			}

			if err := repairPlatformSub2APIHostConfig(db); err != nil {
				t.Fatal(err)
			}
			if err := repairPlatformSub2APIHostConfig(db); err != nil {
				t.Fatalf("migration must be idempotent: %v", err)
			}

			result := models.SystemConfig{}
			if err := db.Where("config_key = ?", platformSub2APIHostKey).First(&result).Error; err != nil {
				t.Fatal(err)
			}
			if result.ConfigValue != testCase.wantHost || result.GroupCode != testCase.wantGroupCode {
				t.Fatalf("host config = %q/%q, want %q/%q", result.ConfigValue, result.GroupCode, testCase.wantHost, testCase.wantGroupCode)
			}
		})
	}
}

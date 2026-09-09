package migration

import (
	"errors"
	"os"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const (
	platformSub2APIHostKey       = "platform.sub2api.host"
	productionSub2APIHost        = "http://43.160.245.179:8080"
	e2eSeedPermissionEnvironment = "RHD_E2E_ALLOW_SEED"
)

func init() {
	register(104, "repair platform Sub2API host configuration", func() error {
		// The isolated E2E database intentionally uses its deterministic provider.
		if os.Getenv(e2eSeedPermissionEnvironment) == "1" {
			return nil
		}
		return repairPlatformSub2APIHostConfig(sqls.DB())
	})
}

func repairPlatformSub2APIHostConfig(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.SystemConfig{}) {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		current := models.SystemConfig{}
		err := tx.Where("config_key = ?", platformSub2APIHostKey).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			now := time.Now()
			return tx.Create(&models.SystemConfig{
				ConfigKey:   platformSub2APIHostKey,
				ConfigValue: productionSub2APIHost,
				GroupCode:   "platform_ai",
				Title:       "Sub2API Host",
				Description: "Platform Sub2API gateway root URL",
				Status:      enums.StatusOk,
				AuditFields: models.AuditFields{
					CreatedAt:      now,
					CreateUserName: "migration-104",
					UpdatedAt:      now,
					UpdateUserName: "migration-104",
				},
			}).Error
		}
		if err != nil {
			return err
		}
		if !isReplaceableSub2APIHost(current.ConfigValue) {
			return nil
		}
		return tx.Model(&models.SystemConfig{}).
			Where("id = ?", current.ID).
			Updates(map[string]any{
				"config_value":     productionSub2APIHost,
				"group_code":       "platform_ai",
				"title":            "Sub2API Host",
				"description":      "Platform Sub2API gateway root URL",
				"status":           enums.StatusOk,
				"updated_at":       time.Now(),
				"update_user_id":   0,
				"update_user_name": "migration-104",
			}).Error
	})
}

func isReplaceableSub2APIHost(value string) bool {
	normalized := strings.ToLower(strings.TrimRight(strings.TrimSpace(value), "/"))
	switch normalized {
	case "", "https://sub2api.example.com", "http://127.0.0.1:18099", "http://127.0.0.1:18099/v1", "http://localhost:18099", "http://localhost:18099/v1":
		return true
	default:
		return false
	}
}

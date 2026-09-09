package migration

import (
	"errors"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(112, "stabilize E2E product dispatch fixture", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			return stabilizeE2EDispatchFixture(ctx.Tx)
		})
	})
}

func stabilizeE2EDispatchFixture(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.User{}) || !db.Migrator().HasTable(&models.AgentTeamMember{}) {
		return nil
	}
	fixture := map[string]struct {
		dispatchEnabled bool
		dispatchWeight  int
	}{
		"e2e.tenant1.admin":     {dispatchEnabled: false, dispatchWeight: 1},
		"e2e.product1.engineer": {dispatchEnabled: true, dispatchWeight: 10},
	}
	now := time.Now()
	for username, target := range fixture {
		var user models.User
		if err := db.Unscoped().Where("username = ?", username).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		if err := db.Model(&models.AgentTeamMember{}).Where("user_id = ?", user.ID).Updates(map[string]any{
			"dispatch_enabled": target.dispatchEnabled,
			"dispatch_weight":  target.dispatchWeight,
			"updated_at":       now,
		}).Error; err != nil {
			return err
		}
		if db.Migrator().HasTable(&models.AgentProfile{}) {
			if err := db.Model(&models.AgentProfile{}).Where("user_id = ?", user.ID).Updates(map[string]any{
				"auto_assign_enabled": target.dispatchEnabled,
				"updated_at":          now,
			}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

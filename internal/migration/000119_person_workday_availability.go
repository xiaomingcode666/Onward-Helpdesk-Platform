package migration

import (
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(119, "use personal workday availability for product schedules", func() error {
		return migratePersonWorkdayAvailability(sqls.DB())
	})
}

func migratePersonWorkdayAvailability(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if err := db.AutoMigrate(&models.AgentTeamScheduleTemplate{}); err != nil {
		return err
	}
	now := time.Now()
	if db.Migrator().HasTable(&models.AgentTeamScheduleTemplate{}) {
		if err := db.Model(&models.AgentTeamScheduleTemplate{}).
			Where("status = ?", enums.StatusOk).
			Updates(map[string]any{
				"workdays":         "[1,2,3,4,5]",
				"start_minute":     0,
				"end_minute":       24 * 60,
				"timezone":         services.EngineerScheduleTimezone,
				"updated_at":       now,
				"update_user_name": "migration-119",
			}).Error; err != nil {
			return err
		}
	}
	if !db.Migrator().HasTable(&models.AgentTeamSchedule{}) {
		return nil
	}
	if err := db.Model(&models.AgentTeamSchedule{}).
		Where("user_id > 0").
		Where("repeat_type = ?", services.AgentTeamScheduleRepeatWeekly).
		Where("remark LIKE ?", "企业默认模板：%").
		Where("status = ?", enums.StatusOk).
		Updates(map[string]any{
			"status":           enums.StatusDeleted,
			"updated_at":       now,
			"update_user_name": "migration-119",
		}).Error; err != nil {
		return err
	}
	if !db.Migrator().HasTable(&models.AgentTeam{}) {
		return nil
	}
	teamTable, err := schemaTableName(db, &models.AgentTeam{})
	if err != nil {
		return err
	}
	scheduleTable, err := schemaTableName(db, &models.AgentTeamSchedule{})
	if err != nil {
		return err
	}
	return db.Model(&models.AgentTeam{}).
		Where("team_type = ? AND product_id > 0 AND schedule_enforced = ?", services.AgentTeamTypeProductRepair, true).
		Where("NOT EXISTS (?)",
			db.Model(&models.AgentTeamSchedule{}).
				Select("1").
				Where(scheduleTable+".team_id = "+teamTable+".id").
				Where(scheduleTable+".publish_status = ?", services.AgentTeamSchedulePublishPublished).
				Where(scheduleTable+".status = ?", enums.StatusOk),
		).
		Updates(map[string]any{
			"schedule_enforced": false,
			"updated_at":        now,
			"update_user_name":  "migration-119",
		}).Error
}

package bootstrap

import (
	"fmt"

	"remotehelpdesk/internal/migration"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func InitMigrations() error {
	if config.CurrentOrDefault().DB.AutoMigrateEnabled() {
		if err := AutoMigrateSchema(sqls.DB()); err != nil {
			return err
		}
	}
	if err := migration.Migrate(); err != nil {
		return err
	}
	if err := EnsurePlatformBuiltInWorkflows(sqls.DB()); err != nil {
		return err
	}
	return ensurePlatformNotificationTemplates(sqls.DB())
}

func EnsurePlatformBuiltInWorkflows(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is required to materialize platform workflows")
	}
	for _, model := range []any{&models.AIWorkflow{}, &models.AIWorkflowVersion{}} {
		if !db.Migrator().HasTable(model) {
			return fmt.Errorf("platform workflow schema is missing for %T; apply reviewed schema migrations before startup", model)
		}
	}
	if err := services.AIWorkflowService.EnsurePlatformBuiltInWorkflowsDB(db); err != nil {
		return fmt.Errorf("materialize embedded platform workflows: %w", err)
	}
	return nil
}

// ensurePlatformNotificationTemplates 保证平台基线通知模板存在且已批准，
// 这样任何租户的通知都能由「已批准模板」生成。
func ensurePlatformNotificationTemplates(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is required to materialize platform notification templates")
	}
	if _, err := services.NotificationTemplateService.EnsurePlatformDefaultsDB(db); err != nil {
		return fmt.Errorf("materialize platform notification templates: %w", err)
	}
	return nil
}

// serviceModels 是定义在 services 包内、但同样需要建表的模型（SLA / 服务日历 / 升级）。
func serviceModels() []any {
	return []any{
		&services.SLAPolicy{},
		&services.SLAPauseRecord{},
		&services.SLAViolation{},
		&services.ServiceCalendar{},
		&services.EscalationRule{},
		&services.TicketEscalation{},
	}
}

func AutoMigrateSchema(db *gorm.DB) error {
	return db.AutoMigrate(append(append([]any{}, models.Models...), serviceModels()...)...)
}

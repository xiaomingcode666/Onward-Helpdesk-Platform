package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func init() {
	register(146, "approved multilingual notification templates and delivery attempts", migrateNotificationTemplateGovernance)
}

// migrateNotificationTemplateGovernance 补齐通知模板治理需要的持久化结构：
// 模板增加语言与批准状态，通知和投递记录保留实际使用的模板，新增逐次发送尝试记录。
func migrateNotificationTemplateGovernance() error {
	db := sqls.DB()
	if db.Migrator().HasTable(&models.NotificationTemplate{}) {
		for _, column := range []string{"Language", "ApprovalStatus", "ApprovedBy", "ApprovedAt"} {
			if !db.Migrator().HasColumn(&models.NotificationTemplate{}, column) {
				if err := db.Migrator().AddColumn(&models.NotificationTemplate{}, column); err != nil {
					return err
				}
			}
		}
		// 同一个 code 现在按语言区分，旧的单列唯一索引会让多语言模板无法保存。
		for _, name := range []string{"idx_notification_templates_code", "uni_notification_templates_code", "uk_notification_templates_code"} {
			if db.Migrator().HasIndex(&models.NotificationTemplate{}, name) {
				if err := db.Migrator().DropIndex(&models.NotificationTemplate{}, name); err != nil {
					return err
				}
			}
		}
		if err := db.Migrator().AutoMigrate(&models.NotificationTemplate{}); err != nil {
			return err
		}
		// 升级前已经在用的模板继续生效，统一按“已批准”处理。
		if err := db.Model(&models.NotificationTemplate{}).
			Where("approval_status = ? OR approval_status IS NULL", "").
			Update("approval_status", "approved").Error; err != nil {
			return err
		}
	}
	if db.Migrator().HasTable(&models.Notification{}) {
		for _, column := range []string{"TemplateCode", "Language"} {
			if !db.Migrator().HasColumn(&models.Notification{}, column) {
				if err := db.Migrator().AddColumn(&models.Notification{}, column); err != nil {
					return err
				}
			}
		}
	}
	if db.Migrator().HasTable(&models.DeliveryLog{}) {
		for _, column := range []string{"TemplateID", "TemplateCode", "Language"} {
			if !db.Migrator().HasColumn(&models.DeliveryLog{}, column) {
				if err := db.Migrator().AddColumn(&models.DeliveryLog{}, column); err != nil {
					return err
				}
			}
		}
	}
	return db.Migrator().AutoMigrate(&models.NotificationDeliveryAttempt{})
}

package migration

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(106, "add ticket accept deadline dispatch fields", func() error {
		return migrateTicketAcceptDeadline(sqls.DB())
	})
}

// migrateTicketAcceptDeadline 为 tickets 表补齐限时接单与派单重试字段。
// AutoMigrate 会自动为 models.Ticket 新增列（sqlite 测试库走该路径）；此处仅对
// 已有 postgres 存量库做幂等补列，列已存在时跳过。
func migrateTicketAcceptDeadline(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.Ticket{}) {
		return nil
	}
	columns := []struct {
		name string
		typ  string
	}{
		{"assigned_at", "timestamp"},
		{"accept_deadline_at", "timestamp"},
		{"dispatch_attempts", "int"},
	}
	for _, column := range columns {
		if db.Migrator().HasColumn(&models.Ticket{}, column.name) {
			continue
		}
		if err := db.Migrator().AddColumn(&models.Ticket{}, column.name); err != nil {
			return err
		}
		if column.typ == "int" {
			if err := db.Model(&models.Ticket{}).Where("dispatch_attempts IS NULL").Update("dispatch_attempts", 0).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

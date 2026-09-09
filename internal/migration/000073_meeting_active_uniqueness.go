package migration

import (
	"fmt"
	"time"

	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(73, "enforce one active meeting per tenant ticket", func() error {
		return ensureMeetingActiveUniqueness(sqls.DB())
	})
}

func ensureMeetingActiveUniqueness(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&models.MeetingRoomJitsi{}) {
		return nil
	}
	table, err := migrationModelTableName(db, &models.MeetingRoomJitsi{})
	if err != nil {
		return err
	}
	if db.Migrator().HasIndex(&models.MeetingRoomJitsi{}, "uk_meeting_tenant_ticket_active") {
		return nil
	}
	var active []models.MeetingRoomJitsi
	if err := db.Where("status IN ?", []string{"waiting", "scheduled", "active"}).
		Order("tenant_id ASC, ticket_id ASC, created_at DESC, id DESC").
		Find(&active).Error; err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(active))
	duplicateIDs := make([]string, 0)
	for _, meeting := range active {
		key := fmt.Sprintf("%d:%s", meeting.TenantID, meeting.TicketID)
		if _, ok := seen[key]; ok {
			duplicateIDs = append(duplicateIDs, meeting.ID)
			continue
		}
		seen[key] = struct{}{}
	}
	if len(duplicateIDs) > 0 {
		now := time.Now()
		if err := db.Model(&models.MeetingRoomJitsi{}).Where("id IN ?", duplicateIDs).Updates(map[string]any{
			"status":     "ended",
			"ended_at":   now,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
	}
	return db.Exec(fmt.Sprintf(
		"CREATE UNIQUE INDEX uk_meeting_tenant_ticket_active ON %s (tenant_id, ticket_id) WHERE status IN ('waiting', 'scheduled', 'active')",
		quoteMigrationIdentifier(table),
	)).Error
}

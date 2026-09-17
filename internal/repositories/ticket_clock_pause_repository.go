package repositories

import (
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var TicketClockPauseRepository = newTicketClockPauseRepository()

func newTicketClockPauseRepository() *ticketClockPauseRepository {
	return &ticketClockPauseRepository{}
}

type ticketClockPauseRepository struct{}

func (r *ticketClockPauseRepository) Create(db *gorm.DB, item *models.TicketClockPause) (bool, error) {
	result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(item)
	return result.RowsAffected > 0, result.Error
}

func (r *ticketClockPauseRepository) FindBySource(
	db *gorm.DB,
	tenantID int64,
	reasonCode, sourceType string,
	sourceID int64,
) *models.TicketClockPause {
	var item models.TicketClockPause
	if err := db.Where(
		"tenant_id = ? AND reason_code = ? AND source_type = ? AND source_id = ?",
		tenantID,
		reasonCode,
		sourceType,
		sourceID,
	).First(&item).Error; err != nil {
		return nil
	}
	return &item
}

func (r *ticketClockPauseRepository) FindActiveByTicket(db *gorm.DB, tenantID, ticketID int64) []models.TicketClockPause {
	items := make([]models.TicketClockPause, 0)
	db.Where(
		"tenant_id = ? AND ticket_id = ? AND status = ? AND ended_at IS NULL",
		tenantID,
		ticketID,
		enums.StatusOk,
	).Order("started_at ASC, id ASC").Find(&items)
	return items
}

func (r *ticketClockPauseRepository) CloseBySource(
	db *gorm.DB,
	tenantID int64,
	reasonCode, sourceType string,
	sourceID int64,
	endedAt time.Time,
	operatorID int64,
	operatorName string,
) error {
	return db.Model(&models.TicketClockPause{}).
		Where(
			"tenant_id = ? AND reason_code = ? AND source_type = ? AND source_id = ? AND ended_at IS NULL",
			tenantID,
			reasonCode,
			sourceType,
			sourceID,
		).
		Updates(map[string]any{
			"ended_at":         endedAt,
			"updated_at":       endedAt,
			"update_user_id":   operatorID,
			"update_user_name": operatorName,
		}).Error
}

func (r *ticketClockPauseRepository) FindByTicket(db *gorm.DB, tenantID, ticketID int64) []models.TicketClockPause {
	items := make([]models.TicketClockPause, 0)
	db.Where("tenant_id = ? AND ticket_id = ?", tenantID, ticketID).
		Order("started_at ASC, id ASC").
		Find(&items)
	return items
}

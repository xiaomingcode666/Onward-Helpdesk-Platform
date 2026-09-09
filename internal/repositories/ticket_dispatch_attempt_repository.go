package repositories

import (
	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var TicketDispatchAttemptRepository = newTicketDispatchAttemptRepository()

type ticketDispatchAttemptRepository struct{}

func newTicketDispatchAttemptRepository() *ticketDispatchAttemptRepository {
	return &ticketDispatchAttemptRepository{}
}

func (r *ticketDispatchAttemptRepository) Create(db *gorm.DB, item *models.TicketDispatchAttempt) error {
	if db == nil || !db.Migrator().HasTable(&models.TicketDispatchAttempt{}) {
		return nil
	}
	return db.Create(item).Error
}

func (r *ticketDispatchAttemptRepository) CountByTicket(db *gorm.DB, ticketID int64) (int64, error) {
	if db == nil || !db.Migrator().HasTable(&models.TicketDispatchAttempt{}) {
		return 0, nil
	}
	var count int64
	err := db.Model(&models.TicketDispatchAttempt{}).Where("ticket_id = ?", ticketID).Count(&count).Error
	return count, err
}

func (r *ticketDispatchAttemptRepository) FindPendingForUpdate(db *gorm.DB, ticketID int64) (*models.TicketDispatchAttempt, error) {
	if db == nil || !db.Migrator().HasTable(&models.TicketDispatchAttempt{}) {
		return nil, gorm.ErrRecordNotFound
	}
	var item models.TicketDispatchAttempt
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("ticket_id = ? AND outcome = ?", ticketID, "pending").
		Order("id DESC").
		First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ticketDispatchAttemptRepository) FindLatestFailedAssignee(db *gorm.DB, ticketID int64, outcomes []string) (*models.TicketDispatchAttempt, error) {
	if db == nil || ticketID <= 0 || len(outcomes) == 0 || !db.Migrator().HasTable(&models.TicketDispatchAttempt{}) {
		return nil, gorm.ErrRecordNotFound
	}
	var item models.TicketDispatchAttempt
	err := db.Where("ticket_id = ? AND assignee_id > 0 AND outcome IN ?", ticketID, outcomes).
		Order("attempt_no DESC").
		Order("id DESC").
		First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ticketDispatchAttemptRepository) Updates(db *gorm.DB, id int64, values map[string]any) error {
	if db == nil || !db.Migrator().HasTable(&models.TicketDispatchAttempt{}) {
		return nil
	}
	return db.Model(&models.TicketDispatchAttempt{}).Where("id = ?", id).Updates(values).Error
}

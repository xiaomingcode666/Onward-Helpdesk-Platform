package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var TicketFeedbackRepository = newTicketFeedbackRepository()

func newTicketFeedbackRepository() *ticketFeedbackRepository {
	return &ticketFeedbackRepository{}
}

type ticketFeedbackRepository struct{}

func (r *ticketFeedbackRepository) Get(db *gorm.DB, id int64) *models.TicketFeedback {
	ret := &models.TicketFeedback{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *ticketFeedbackRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.TicketFeedback) {
	cnd.Find(db, &list)
	return
}

func (r *ticketFeedbackRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.TicketFeedback {
	ret := &models.TicketFeedback{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *ticketFeedbackRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.TicketFeedback{})
}

func (r *ticketFeedbackRepository) Create(db *gorm.DB, t *models.TicketFeedback) error {
	return db.Create(t).Error
}

func (r *ticketFeedbackRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.TicketFeedback{}).Where("id = ?", id).Updates(columns).Error
}

func (r *ticketFeedbackRepository) FindByTicketID(db *gorm.DB, ticketID int64) []models.TicketFeedback {
	var list []models.TicketFeedback
	db.Where("ticket_id = ?", ticketID).Find(&list)
	return list
}

func (r *ticketFeedbackRepository) FindByTicketIDs(db *gorm.DB, tenantID int64, ticketIDs []int64) ([]models.TicketFeedback, error) {
	list := make([]models.TicketFeedback, 0)
	if tenantID <= 0 || len(ticketIDs) == 0 {
		return list, nil
	}
	err := db.Where("tenant_id = ? AND ticket_id IN ? AND status = ?", tenantID, ticketIDs, "submitted").
		Order("ticket_id ASC, submitted_at DESC, id DESC").
		Find(&list).Error
	return list, err
}

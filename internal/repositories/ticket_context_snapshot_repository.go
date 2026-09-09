package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var TicketContextSnapshotRepository = newTicketContextSnapshotRepository()

func newTicketContextSnapshotRepository() *ticketContextSnapshotRepository {
	return &ticketContextSnapshotRepository{}
}

type ticketContextSnapshotRepository struct{}

func (r *ticketContextSnapshotRepository) Get(db *gorm.DB, id int64) *models.TicketContextSnapshot {
	ret := &models.TicketContextSnapshot{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *ticketContextSnapshotRepository) GetByTicketID(db *gorm.DB, ticketID int64) *models.TicketContextSnapshot {
	ret := &models.TicketContextSnapshot{}
	if err := db.Where("ticket_id = ?", ticketID).First(ret).Error; err != nil {
		return nil
	}
	return ret
}

func (r *ticketContextSnapshotRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.TicketContextSnapshot) {
	cnd.Find(db, &list)
	return
}

func (r *ticketContextSnapshotRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.TicketContextSnapshot {
	ret := &models.TicketContextSnapshot{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *ticketContextSnapshotRepository) Create(db *gorm.DB, t *models.TicketContextSnapshot) error {
	return db.Create(t).Error
}

func (r *ticketContextSnapshotRepository) UpdateCustomerJSON(db *gorm.DB, tenantID, ticketID int64, customerJSON string) error {
	if db == nil || tenantID <= 0 || ticketID <= 0 {
		return nil
	}
	return db.Model(&models.TicketContextSnapshot{}).
		Where("tenant_id = ? AND ticket_id = ?", tenantID, ticketID).
		Update("customer_json", customerJSON).Error
}

func (r *ticketContextSnapshotRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.TicketContextSnapshot{})
}

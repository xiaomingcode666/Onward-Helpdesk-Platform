package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var TicketQualityClueRepository = newTicketQualityClueRepository()

func newTicketQualityClueRepository() *ticketQualityClueRepository {
	return &ticketQualityClueRepository{}
}

type ticketQualityClueRepository struct{}

func (r *ticketQualityClueRepository) Get(db *gorm.DB, id int64) *models.TicketQualityClue {
	ret := &models.TicketQualityClue{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *ticketQualityClueRepository) Take(db *gorm.DB, where ...any) *models.TicketQualityClue {
	ret := &models.TicketQualityClue{}
	if err := db.Take(ret, where...).Error; err != nil {
		return nil
	}
	return ret
}

func (r *ticketQualityClueRepository) Find(db *gorm.DB, cnd *sqls.Cnd) (list []models.TicketQualityClue) {
	cnd.Find(db, &list)
	return
}

func (r *ticketQualityClueRepository) FindOne(db *gorm.DB, cnd *sqls.Cnd) *models.TicketQualityClue {
	ret := &models.TicketQualityClue{}
	if err := cnd.FindOne(db, &ret); err != nil {
		return nil
	}
	return ret
}

func (r *ticketQualityClueRepository) FindPageByCnd(db *gorm.DB, cnd *sqls.Cnd) (list []models.TicketQualityClue, paging *sqls.Paging) {
	cnd.Find(db, &list)
	count := cnd.Count(db, &models.TicketQualityClue{})
	paging = &sqls.Paging{
		Page:  cnd.Paging.Page,
		Limit: cnd.Paging.Limit,
		Total: count,
	}
	return
}

func (r *ticketQualityClueRepository) Create(db *gorm.DB, t *models.TicketQualityClue) error {
	return db.Create(t).Error
}

func (r *ticketQualityClueRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.TicketQualityClue{}).Where("id = ?", id).Updates(columns).Error
}

func (r *ticketQualityClueRepository) Count(db *gorm.DB, cnd *sqls.Cnd) int64 {
	return cnd.Count(db, &models.TicketQualityClue{})
}

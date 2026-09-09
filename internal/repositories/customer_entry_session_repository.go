package repositories

import (
	"remotehelpdesk/internal/models"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var CustomerEntrySessionRepository = newCustomerEntrySessionRepository()

type customerEntrySessionRepository struct{}

func newCustomerEntrySessionRepository() *customerEntrySessionRepository {
	return &customerEntrySessionRepository{}
}

func (r *customerEntrySessionRepository) Get(db *gorm.DB, id int64) *models.CustomerEntrySession {
	item := &models.CustomerEntrySession{}
	if err := db.First(item, "id = ?", id).Error; err != nil {
		return nil
	}
	return item
}

func (r *customerEntrySessionRepository) FindByIDs(db *gorm.DB, ids []int64) ([]models.CustomerEntrySession, error) {
	items := make([]models.CustomerEntrySession, 0)
	ids = uniqueRepositoryInt64s(ids)
	if db == nil || len(ids) == 0 || !db.Migrator().HasTable(&models.CustomerEntrySession{}) {
		return items, nil
	}
	err := db.Where("id IN ?", ids).Find(&items).Error
	return items, err
}

func (r *customerEntrySessionRepository) Create(db *gorm.DB, item *models.CustomerEntrySession) error {
	return db.Create(item).Error
}

func (r *customerEntrySessionRepository) CreateIfAbsent(db *gorm.DB, item *models.CustomerEntrySession) (bool, error) {
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "visitor_token_hash"}},
		DoNothing: true,
	}).Create(item)
	return result.RowsAffected > 0, result.Error
}

func (r *customerEntrySessionRepository) FindActive(db *gorm.DB, id int64, now any) *models.CustomerEntrySession {
	item := &models.CustomerEntrySession{}
	if err := db.Where("id = ? and state = ? and (expires_at is null or expires_at > ?)", id, "active", now).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *customerEntrySessionRepository) FindActiveByVisitorTokenHash(db *gorm.DB, tokenHash string, now any) *models.CustomerEntrySession {
	item := &models.CustomerEntrySession{}
	if err := db.Where(
		"visitor_token_hash = ? and state = ? and (expires_at is null or expires_at > ?)",
		tokenHash,
		"active",
		now,
	).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *customerEntrySessionRepository) FindByCnd(db *gorm.DB, cnd *sqls.Cnd) *models.CustomerEntrySession {
	item := &models.CustomerEntrySession{}
	if err := cnd.FindOne(db, item); err != nil {
		return nil
	}
	return item
}

func (r *customerEntrySessionRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.CustomerEntrySession{}).Where("id = ?", id).Updates(columns).Error
}

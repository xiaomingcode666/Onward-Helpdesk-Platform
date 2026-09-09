package repositories

import (
	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
)

var DomainEventRepository = &domainEventRepository{}
var OutboxRecordRepository = &outboxRecordRepository{}

type domainEventRepository struct{}
type outboxRecordRepository struct{}

func (r *domainEventRepository) GetByIdempotencyKey(db *gorm.DB, key string) *models.DomainEvent {
	ret := &models.DomainEvent{}
	if err := db.First(ret, "idempotency_key = ?", key).Error; err != nil {
		return nil
	}
	return ret
}

func (r *domainEventRepository) Create(db *gorm.DB, item *models.DomainEvent) error {
	return db.Create(item).Error
}

func (r *outboxRecordRepository) Create(db *gorm.DB, item *models.OutboxRecord) error {
	return db.Create(item).Error
}

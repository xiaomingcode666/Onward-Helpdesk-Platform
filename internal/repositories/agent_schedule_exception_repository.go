package repositories

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"time"

	"gorm.io/gorm"
)

var AgentScheduleExceptionRepository = newAgentScheduleExceptionRepository()

func newAgentScheduleExceptionRepository() *agentScheduleExceptionRepository {
	return &agentScheduleExceptionRepository{}
}

type agentScheduleExceptionRepository struct{}

func (r *agentScheduleExceptionRepository) GetForTenant(db *gorm.DB, tenantID, id int64) *models.AgentScheduleException {
	if db == nil || tenantID <= 0 || id <= 0 {
		return nil
	}
	item := &models.AgentScheduleException{}
	if err := db.Where("tenant_id = ? AND id = ? AND status = ?", tenantID, id, enums.StatusOk).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *agentScheduleExceptionRepository) FindByRequestKey(db *gorm.DB, tenantID, userID int64, requestKey string) *models.AgentScheduleException {
	if db == nil || tenantID <= 0 || userID <= 0 || requestKey == "" {
		return nil
	}
	item := &models.AgentScheduleException{}
	if err := db.Where("tenant_id = ? AND user_id = ? AND request_key = ?", tenantID, userID, requestKey).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *agentScheduleExceptionRepository) FindVisible(db *gorm.DB, tenantID int64, userIDs []int64, startAt, endAt time.Time, approvalStatuses []string) ([]models.AgentScheduleException, error) {
	items := make([]models.AgentScheduleException, 0)
	if db == nil || tenantID <= 0 || len(userIDs) == 0 {
		return items, nil
	}
	query := db.Model(&models.AgentScheduleException{}).
		Where("tenant_id = ? AND user_id IN ? AND status = ?", tenantID, uniqueRepositoryInt64s(userIDs), enums.StatusOk)
	if !startAt.IsZero() {
		query = query.Where("end_at > ?", startAt)
	}
	if !endAt.IsZero() {
		query = query.Where("start_at < ?", endAt)
	}
	if len(approvalStatuses) > 0 {
		query = query.Where("approval_status IN ?", approvalStatuses)
	}
	err := query.Order("start_at ASC").Order("id ASC").Find(&items).Error
	return items, err
}

func (r *agentScheduleExceptionRepository) FindOverlapping(db *gorm.DB, tenantID, userID int64, startAt, endAt time.Time, approvalStatuses []string) ([]models.AgentScheduleException, error) {
	return r.FindVisible(db, tenantID, []int64{userID}, startAt, endAt, approvalStatuses)
}

func (r *agentScheduleExceptionRepository) Create(db *gorm.DB, item *models.AgentScheduleException) error {
	return db.Create(item).Error
}

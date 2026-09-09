package repositories

import (
	"strconv"

	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
)

var DiagnosisSessionRepository = &diagnosisSessionRepository{}

type diagnosisSessionRepository struct{}

var DiagnosisStepRepository = &diagnosisStepRepository{}

type diagnosisStepRepository struct{}

// Get 按会话 ID 查询诊断会话。
func (r *diagnosisSessionRepository) Get(db *gorm.DB, id string) *models.DiagnosisSession {
	if id == "" {
		return nil
	}
	ret := &models.DiagnosisSession{}
	if err := db.First(ret, "id = ?", id).Error; err != nil {
		return nil
	}
	return ret
}

func (r *diagnosisSessionRepository) FindLatestForContext(db *gorm.DB, tenantID, conversationID, deviceID, productID int64) *models.DiagnosisSession {
	if tenantID <= 0 {
		return nil
	}
	query := db.Where("tenant_id = ?", tenantID)
	conditions := make([]string, 0, 3)
	values := make([]any, 0, 6)
	if conversationID > 0 {
		conditions = append(conditions, "(conversation_ref_id = ? OR conversation_id = ?)")
		values = append(values, conversationID, strconv.FormatInt(conversationID, 10))
	}
	if deviceID > 0 {
		conditions = append(conditions, "(device_ref_id = ? OR device_id = ?)")
		values = append(values, deviceID, strconv.FormatInt(deviceID, 10))
	}
	if productID > 0 {
		conditions = append(conditions, "(product_ref_id = ? OR product_id = ?)")
		values = append(values, productID, strconv.FormatInt(productID, 10))
	}
	if len(conditions) == 0 {
		return nil
	}
	condition := conditions[0]
	for _, item := range conditions[1:] {
		condition += " OR " + item
	}
	var session models.DiagnosisSession
	if err := query.Where("("+condition+")", values...).Order("created_at DESC").First(&session).Error; err != nil {
		return nil
	}
	return &session
}

func (r *diagnosisStepRepository) ListBySession(db *gorm.DB, sessionID string) ([]models.DiagnosisStep, error) {
	var rows []models.DiagnosisStep
	err := db.Where("session_id = ?", sessionID).
		Order("sequence_no ASC, id ASC").
		Find(&rows).Error
	return rows, err
}

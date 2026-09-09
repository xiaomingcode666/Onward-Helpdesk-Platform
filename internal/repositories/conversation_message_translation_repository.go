package repositories

import (
	"remotehelpdesk/internal/models"

	"gorm.io/gorm"
)

var ConversationMessageTranslationRepository = newConversationMessageTranslationRepository()

type conversationMessageTranslationRepository struct{}

func newConversationMessageTranslationRepository() *conversationMessageTranslationRepository {
	return &conversationMessageTranslationRepository{}
}

func (r *conversationMessageTranslationRepository) FindCached(db *gorm.DB, tenantID, messageID int64, targetLanguage, sourceTextHash string) *models.ConversationMessageTranslation {
	ret := &models.ConversationMessageTranslation{}
	if err := db.Where(
		"tenant_id = ? AND message_id = ? AND target_language = ? AND source_text_hash = ?",
		tenantID,
		messageID,
		targetLanguage,
		sourceTextHash,
	).First(ret).Error; err != nil {
		return nil
	}
	return ret
}

func (r *conversationMessageTranslationRepository) Create(db *gorm.DB, item *models.ConversationMessageTranslation) error {
	return db.Create(item).Error
}

func (r *conversationMessageTranslationRepository) Updates(db *gorm.DB, id int64, columns map[string]any) error {
	return db.Model(&models.ConversationMessageTranslation{}).Where("id = ?", id).Updates(columns).Error
}

package builders

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/utils"
)

func BuildConversationMessageTranslation(item *models.ConversationMessageTranslation, cached bool) *dto.ConversationMessageTranslationDTO {
	if item == nil {
		return nil
	}
	return &dto.ConversationMessageTranslationDTO{
		ID:               item.ID,
		ConversationID:   item.ConversationID,
		MessageID:        item.MessageID,
		SourceLanguage:   item.SourceLanguage,
		TargetLanguage:   item.TargetLanguage,
		TranslatedText:   item.TranslatedText,
		ModelName:        item.ModelName,
		PromptTokens:     item.PromptTokens,
		CompletionTokens: item.CompletionTokens,
		Cached:           cached,
		CreatedAt:        utils.FormatTime(item.CreatedAt),
	}
}

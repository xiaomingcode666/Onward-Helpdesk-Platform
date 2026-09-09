package dto

type ConversationMessageTranslationDTO struct {
	ID               int64  `json:"id"`
	ConversationID   int64  `json:"conversation_id"`
	MessageID        int64  `json:"message_id"`
	SourceLanguage   string `json:"source_language"`
	TargetLanguage   string `json:"target_language"`
	TranslatedText   string `json:"translated_text"`
	ModelName        string `json:"model_name"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	Cached           bool   `json:"cached"`
	CreatedAt        string `json:"created_at"`
}

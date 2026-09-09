package request

type TranslateConversationMessageRequest struct {
	MessageID      int64  `json:"message_id"`
	TargetLanguage string `json:"target_language"`
}

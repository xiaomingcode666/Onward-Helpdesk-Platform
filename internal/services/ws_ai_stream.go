package services

import "remotehelpdesk/internal/models"

// AIMessageStreamPayload contains provisional text, never a persisted message
// or a read receipt. Final messages still use message.created after commit.
type AIMessageStreamPayload struct {
	ConversationID  int64  `json:"conversationId"`
	SourceMessageID int64  `json:"sourceMessageId"`
	StreamID        string `json:"streamId"`
	ClientMsgID     string `json:"clientMsgId"`
	Sequence        int    `json:"sequence"`
	Content         string `json:"content"`
	SentAt          string `json:"sentAt"`
	StartedAt       int64  `json:"startedAt"`
	ExpiresAt       int64  `json:"streamExpiresAt"`
	Done            bool   `json:"done"`
}

func (AIMessageStreamPayload) realtimeEventPayload() {}

type aiMessageStreamEvent struct{ Payload AIMessageStreamPayload }

func (aiMessageStreamEvent) EventType() string                    { return "message.streaming" }
func (e aiMessageStreamEvent) EventPayload() RealtimeEventPayload { return e.Payload }

func (s *wsService) PublishAIMessageStream(conversation *models.Conversation, update AIMessageStreamPayload) {
	if conversation == nil || conversation.ID != update.ConversationID || update.SourceMessageID <= 0 || update.StreamID == "" {
		return
	}
	event := s.newEvent(s.conversationTopic(conversation.ID), aiMessageStreamEvent{Payload: update})
	s.PublishToTopics(s.routeConversationTopics(conversation), event)
}

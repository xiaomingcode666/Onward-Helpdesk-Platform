package runtime

import (
	applicationruntime "remotehelpdesk/internal/ai/application/runtime"
	"remotehelpdesk/internal/models"
)

type aiReplyContext struct {
	Conversation     models.Conversation
	Message          models.Message
	AIAgent          models.AIAgent
	SummaryRef       **applicationruntime.Summary
	PendingInterrupt *models.ConversationInterrupt
}

func (c aiReplyContext) setSummary(summary *applicationruntime.Summary) {
	if c.SummaryRef != nil {
		*c.SummaryRef = summary
	}
}

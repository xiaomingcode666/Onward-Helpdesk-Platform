package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/aistream"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	svc "remotehelpdesk/internal/services"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
)

func (s *aiReplyService) withReplyStream(ctx context.Context, conversation models.Conversation, message models.Message, agent models.AIAgent, prefix string) (context.Context, func()) {
	now := time.Now()
	update := svc.AIMessageStreamPayload{
		ConversationID: conversation.ID, SourceMessageID: message.ID,
		StreamID: uuid.NewString(), ClientMsgID: fmt.Sprintf("%s_%d", prefix, message.ID),
		SentAt: now.Format(time.DateTime), StartedAt: now.UnixMilli(),
		ExpiresAt: now.Add(s.resolveReplyTimeout(agent) + 15*time.Second).UnixMilli(),
	}
	return newReplyStream(ctx, conversation, update, func() *models.Conversation {
		return repositories.ConversationRepository.Get(sqls.DB(), conversation.ID)
	}, svc.WsService.PublishAIMessageStream)
}

func newReplyStream(ctx context.Context, conversation models.Conversation, update svc.AIMessageStreamPayload,
	loadConversation func() *models.Conversation, publish func(*models.Conversation, svc.AIMessageStreamPayload),
) (context.Context, func()) {
	var mu sync.Mutex
	var lastSent time.Time
	var flushTimer *time.Timer
	finished := false
	publishLocked := func() {
		if finished || ctx.Err() != nil {
			return
		}
		current := loadConversation()
		if current == nil || current.TenantID != conversation.TenantID {
			return
		}
		update.Sequence++
		if current.Status == enums.IMConversationStatusClosed || current.CurrentAssigneeID > 0 || current.HandoffAt != nil {
			update.Done = true
			update.Content = ""
			finished = true
		}
		publish(current, update)
		lastSent = time.Now()
	}
	emit := func(text string) {
		mu.Lock()
		defer mu.Unlock()
		if finished || ctx.Err() != nil || text == update.Content {
			return
		}
		update.Content = text
		if text != "" && !lastSent.IsZero() && time.Since(lastSent) < 40*time.Millisecond {
			if flushTimer == nil {
				flushTimer = time.AfterFunc(40*time.Millisecond-time.Since(lastSent), func() {
					mu.Lock()
					defer mu.Unlock()
					flushTimer = nil
					publishLocked()
				})
			}
			return
		}
		if flushTimer != nil {
			flushTimer.Stop()
			flushTimer = nil
		}
		publishLocked()
	}
	closeStream := func() {
		mu.Lock()
		defer mu.Unlock()
		if flushTimer != nil {
			flushTimer.Stop()
			flushTimer = nil
		}
		if finished {
			return
		}
		finished = true
		if lastSent.IsZero() {
			return
		}
		update.Sequence++
		update.Done = true
		update.Content = ""
		publish(&conversation, update)
	}
	return aistream.WithHandler(ctx, emit), closeStream
}

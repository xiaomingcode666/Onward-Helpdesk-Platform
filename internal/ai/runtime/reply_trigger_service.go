package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	applicationruntime "remotehelpdesk/internal/ai/application/runtime"
	"remotehelpdesk/internal/models"
	svc "remotehelpdesk/internal/services"
)

func (s *aiReplyService) resolveReplyTimeout(aiAgent models.AIAgent) time.Duration {
	if aiAgent.ReplyTimeoutSeconds <= 0 {
		return time.Duration(defaultAIReplyAsyncTimeoutSeconds) * time.Second
	}
	if aiAgent.ReplyTimeoutSeconds > maxAIReplyAsyncTimeoutSeconds {
		return time.Duration(maxAIReplyAsyncTimeoutSeconds) * time.Second
	}
	return time.Duration(aiAgent.ReplyTimeoutSeconds) * time.Second
}

func (s *aiReplyService) commitAsyncFailure(conversation models.Conversation, message models.Message, aiAgent models.AIAgent) {
	failureText := buildAIReplyFailureText(conversation, message, aiAgent, aiReplyFailureKindRuntime)
	if _, err := s.commit.SendAIReply(replyCommitInput{
		Conversation: conversation,
		Message:      message,
		AIAgent:      aiAgent,
		ReplyText:    failureText,
		ClientPrefix: "ai_reply_error",
	}); err != nil {
		if shouldCommitLateAIReplyAsSystem(err) {
			if _, lateErr := svc.MessageService.SendSystemMessageWithRequestIDAndWorkflowRunID(
				conversation.ID,
				fmt.Sprintf("ai_reply_error_late_%d", message.ID),
				buildLateAIReplyFailureSystemText(message),
				"",
				message.RequestID,
				0,
			); lateErr == nil {
				return
			}
		}
		slog.Error("failed to commit ai reply failure message",
			"requestId", message.RequestID,
			"conversation_id", conversation.ID,
			"message_id", message.ID,
			"error", err)
	}
}

func (s *aiReplyService) TriggerReply(ctx context.Context, conversation models.Conversation, message models.Message, aiAgent models.AIAgent) (retErr error) {
	var summary *applicationruntime.Summary
	replyCtx := aiReplyContext{
		Conversation: conversation,
		Message:      message,
		AIAgent:      aiAgent,
		SummaryRef:   &summary,
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.eligibility != nil && !s.eligibility.CanReply(conversation, message, aiAgent) {
		return nil
	}
	if pendingInterrupt := svc.ConversationInterruptService.FindLatestPendingByConversationID(conversation.ID); pendingInterrupt != nil {
		replyCtx.PendingInterrupt = pendingInterrupt
		return s.resumePendingInterrupt(ctx, replyCtx)
	}
	return s.executeReply(ctx, replyCtx)
}

func (s *aiReplyService) resumePendingInterrupt(ctx context.Context, replyCtx aiReplyContext) error {
	return s.interrupts.ResumePendingInterrupt(ctx, s, replyCtx)
}

func (s *aiReplyService) executeReply(ctx context.Context, replyCtx aiReplyContext) error {
	summary, err := s.executor.Run(ctx, runtimeReplyRunInput{
		Conversation: replyCtx.Conversation,
		Message:      replyCtx.Message,
		AIAgent:      replyCtx.AIAgent,
	})
	replyCtx.setSummary(summary)
	if usageErr := s.recordAIReplyUsage(ctx, replyCtx, summary); usageErr != nil {
		return usageErr
	}
	if err != nil {
		return err
	}
	if summary != nil && summary.Interrupted {
		return s.interrupts.HandleInterruptedSummary(s, replyCtx, summary)
	}
	if summary != nil && strings.TrimSpace(summary.ReplyText) != "" {
		_, err := s.commit.CommitAIReply(replyCommitInput{
			Conversation:  replyCtx.Conversation,
			Message:       replyCtx.Message,
			AIAgent:       replyCtx.AIAgent,
			ReplyText:     summary.ReplyText,
			Citations:     summary.KnowledgeCitations,
			ClientPrefix:  "ai_reply",
			WorkflowRunID: summary.WorkflowRunID,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *aiReplyService) recordAIReplyUsage(ctx context.Context, replyCtx aiReplyContext, summary *applicationruntime.Summary) error {
	if summary == nil || replyCtx.Conversation.TenantID <= 0 || replyCtx.Conversation.ProductID <= 0 {
		return nil
	}
	if strings.TrimSpace(summary.ModelAPIKeyID) == "" || (summary.PromptTokens <= 0 && summary.CompletionTokens <= 0) {
		return nil
	}
	return svc.MeteringService.RecordUsage(
		ctx,
		replyCtx.Conversation.TenantID,
		replyCtx.Conversation.ProductID,
		summary.ModelAPIKeyID,
		"chat_completion",
		fmt.Sprintf("ai_reply:%d", replyCtx.Message.ID),
		int64(summary.PromptTokens),
		int64(summary.CompletionTokens),
		0,
	)
}

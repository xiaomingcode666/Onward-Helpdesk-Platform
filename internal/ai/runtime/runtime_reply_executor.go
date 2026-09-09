package runtime

import (
	"context"
	"fmt"
	"strings"

	applicationruntime "remotehelpdesk/internal/ai/application/runtime"
	"remotehelpdesk/internal/ai/runtime/graphs"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/tracex"
	svc "remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

type runtimeReplyExecutor struct{}

type runtimeReplyRunInput struct {
	Conversation models.Conversation
	Message      models.Message
	AIAgent      models.AIAgent
}

type runtimeReplyResumeInput struct {
	Conversation     models.Conversation
	Message          models.Message
	AIAgent          models.AIAgent
	PendingInterrupt *models.ConversationInterrupt
}

func newRuntimeReplyExecutor() *runtimeReplyExecutor {
	return &runtimeReplyExecutor{}
}

func (e *runtimeReplyExecutor) Run(ctx context.Context, input runtimeReplyRunInput) (*applicationruntime.Summary, error) {
	runtimeAgent, _, err := svc.AIAgentReleaseService.MaterializeRuntimeAgent(sqls.DB(), input.AIAgent)
	if err != nil {
		return nil, err
	}
	input.AIAgent = runtimeAgent
	ctx, aiConfig, err := resolveAgentAIConfig(ctx, input.AIAgent, input.Conversation.TenantID, input.Conversation.ProductID, nil)
	if err != nil {
		return nil, err
	}
	ctx = tracex.ContextWithRequestID(ctx, input.Message.RequestID)
	summary, err := Service.Run(ctx, applicationruntime.Request{
		Conversation: input.Conversation,
		UserMessage:  input.Message,
		AIAgent:      input.AIAgent,
		AIConfig:     *aiConfig,
	})
	return summary, err
}

func (e *runtimeReplyExecutor) ResumePendingInterrupt(ctx context.Context, input runtimeReplyResumeInput) (*applicationruntime.Summary, error) {
	if input.PendingInterrupt == nil {
		return nil, fmt.Errorf("pending interrupt is required")
	}
	runtimeAgent, _, err := svc.AIAgentReleaseService.MaterializeRuntimeAgent(sqls.DB(), input.AIAgent)
	if err != nil {
		return nil, err
	}
	input.AIAgent = runtimeAgent
	ctx, aiConfig, err := resolveAgentAIConfig(ctx, input.AIAgent, input.Conversation.TenantID, input.Conversation.ProductID, nil)
	if err != nil {
		return nil, err
	}
	ctx = tracex.ContextWithRequestID(ctx, input.Message.RequestID)
	summary, err := Service.Resume(ctx, applicationruntime.ResumeRequest{
		Conversation: input.Conversation,
		UserMessage:  input.Message,
		AIAgent:      input.AIAgent,
		AIConfig:     *aiConfig,
		CheckPointID: strings.TrimSpace(input.PendingInterrupt.CheckPointID),
		ResumeData: map[string]string{
			strings.TrimSpace(input.PendingInterrupt.InterruptID): strings.TrimSpace(input.Message.Content),
		},
	})
	return summary, err
}

func expiredInterruptSummary() *applicationruntime.Summary {
	return &applicationruntime.Summary{
		Status:    "expired",
		ReplyText: graphs.ConfirmationExpiredReply,
	}
}

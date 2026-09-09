package runtime

import (
	"context"
	"strings"
	"time"

	applicationruntime "remotehelpdesk/internal/ai/application/runtime"
	workflowexecutor "remotehelpdesk/internal/ai/runtime/workflow"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	svc "remotehelpdesk/internal/services"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
)

func init() {
	svc.AIWorkflowTestRunHook = runWorkflowDraftTest
}

func runWorkflowDraftTest(ctx context.Context, input svc.AIWorkflowTestRunInput) (*response.AIWorkflowTestRunResponse, error) {
	agent, _, err := svc.AIAgentReleaseService.MaterializeRuntimeAgent(sqls.DB(), input.Agent)
	if err != nil {
		return nil, err
	}
	credentialChain := input.Request.Definition.EffectiveModelCredentialChain(agent.ProductID)
	ctx, aiConfig, err := resolveAgentAIConfig(ctx, agent, agent.TenantID, agent.ProductID, credentialChain)
	if err != nil {
		return nil, err
	}
	conversation := models.Conversation{
		TenantID:               agent.TenantID,
		ProductID:              agent.ProductID,
		ProductModelID:         input.Request.RuntimeContext.ProductModelID,
		DeviceID:               input.Request.RuntimeContext.DeviceID,
		ServiceCodeID:          input.Request.RuntimeContext.ServiceCodeID,
		CustomerEntrySessionID: input.Request.RuntimeContext.CustomerEntrySessionID,
		AIAgentID:              agent.ID,
		ServiceMode:            workflowTestServiceMode(input.Request.RuntimeContext.ServiceMode, agent.ServiceMode),
	}
	message := models.Message{
		RequestID:   "workflow-test-" + uuid.NewString(),
		SenderType:  enums.IMSenderTypeCustomer,
		MessageType: enums.IMMessageTypeText,
		Content:     strings.TrimSpace(input.Request.UserMessage),
	}
	startedAt := time.Now()
	result, runErr := Service.app.RunDraft(ctx, applicationruntime.DraftRunRequest{
		Definition:          input.Request.Definition,
		Conversation:        conversation,
		UserMessage:         message,
		AIAgent:             agent,
		AIConfig:            *aiConfig,
		AutoConfirm:         input.Request.AutoConfirm,
		BranchOverrides:     input.Request.BranchOverrides,
		SimulateDeviceBound: input.Request.RuntimeContext.DeviceBound,
	})
	ret := buildWorkflowTestRunResponse(input, aiConfig, result, runErr, time.Since(startedAt))
	if result != nil {
		return ret, nil
	}
	return nil, runErr
}

func workflowTestServiceMode(value string, fallback enums.IMConversationServiceMode) enums.IMConversationServiceMode {
	switch strings.TrimSpace(value) {
	case "ai_only":
		return enums.IMConversationServiceModeAIOnly
	case "human_only":
		return enums.IMConversationServiceModeHumanOnly
	case "ai_first":
		return enums.IMConversationServiceModeAIFirst
	default:
		return fallback
	}
}

func buildWorkflowTestRunResponse(
	input svc.AIWorkflowTestRunInput,
	aiConfig *models.AIConfig,
	result *workflowexecutor.Result,
	runErr error,
	duration time.Duration,
) *response.AIWorkflowTestRunResponse {
	ret := &response.AIWorkflowTestRunResponse{
		WorkflowID:    input.Workflow.ID,
		AIAgentID:     input.Agent.ID,
		AIAgentName:   input.Agent.Name,
		RuntimeEngine: workflowexecutor.RuntimeEngineEinoGraph,
		Status:        "failed",
		DurationMS:    duration.Milliseconds(),
		SideEffects:   "suppressed",
		Nodes:         make([]response.AIWorkflowTestNodeResultResponse, 0),
	}
	if aiConfig != nil {
		ret.ModelCredentialScope = aiConfig.RuntimeCredentialScope
		ret.ModelAPIKeyID = aiConfig.RuntimeAPIKeyID
		ret.ModelCredentialFallback = aiConfig.RuntimeCredentialFallback
	}
	if runErr != nil {
		ret.ErrorMessage = runErr.Error()
	}
	if result == nil {
		return ret
	}
	ret.Status = result.Status
	ret.ReplyText = result.ReplyText
	ret.NodePath = append([]string(nil), result.NodePath...)
	for _, trace := range result.NodeTraces {
		ret.Nodes = append(ret.Nodes, response.AIWorkflowTestNodeResultResponse{
			NodeID:        trace.NodeID,
			NodeType:      trace.NodeType,
			Status:        trace.Status,
			InputPreview:  trace.InputPreview,
			OutputPreview: trace.OutputPreview,
			ErrorMessage:  trace.ErrorMessage,
			DurationMS:    trace.DurationMS,
		})
		if trace.Status == "failed" {
			ret.FailedNodeID = trace.NodeID
		}
		if trace.Status == "interrupted" {
			ret.InterruptNodeID = trace.NodeID
		}
	}
	return ret
}

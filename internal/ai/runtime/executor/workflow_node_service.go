package executor

import (
	"context"
	"fmt"
	"strings"

	"remotehelpdesk/internal/ai/runtime/internal/impl/adapter"
	"remotehelpdesk/internal/ai/runtime/internal/impl/callbacks"
	"remotehelpdesk/internal/ai/runtime/internal/impl/factory"
	"remotehelpdesk/internal/pkg/utils"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// ExecuteWorkflowNode uses the same AgentFactory, Skill/MCP middleware and Eino
// runner as a standalone Agent run, but deliberately skips retrieval and Eino
// checkpoints because those responsibilities belong to the parent Workflow.
func (s *Service) ExecuteWorkflowNode(ctx context.Context, req WorkflowNodeInput) (*RunResult, error) {
	summary := &RunResult{
		RunID:            uuid.NewString(),
		Status:           "started",
		ModelProvider:    string(req.AIConfig.Provider),
		ModelName:        req.AIConfig.ModelName,
		ToolCodes:        make([]string, 0),
		InvokedToolCodes: make([]string, 0),
	}
	collector := callbacks.NewRuntimeTraceCollector()
	collector.Data.RunID = summary.RunID
	collector.Data.Model.Provider = string(req.AIConfig.Provider)
	collector.Data.Model.Name = req.AIConfig.ModelName
	collector.Data.Input.KnowledgeBaseIDs = utils.SplitInt64s(req.AIAgent.KnowledgeIDs)
	collector.Data.Input.CurrentUserMessagePreview = preview(req.UserPrompt, 120)

	history := adapter.BuildHistoryMessages(req.Conversation.ID, req.UserMessage.ID, 12)
	summary.HistoryMessageCount = len(history.Messages)
	collector.Data.Input.HistoryMessageCount = len(history.Messages)
	messages := make([]*schema.Message, 0, len(history.Messages)+1)
	messages = append(messages, history.Messages...)
	userPrompt := strings.TrimSpace(req.UserPrompt)
	if userPrompt == "" {
		userPrompt = strings.TrimSpace(req.UserMessage.Content)
	}
	if knowledgeContext := strings.TrimSpace(req.KnowledgeContext); knowledgeContext != "" {
		userPrompt = strings.TrimSpace(
			"Customer's original message:\n" + userPrompt +
				"\n\nReference knowledge (not written by the customer; do not copy its language):\n" + knowledgeContext,
		)
	}
	messages = append(messages, schema.UserMessage(userPrompt))

	agentConfig := req.AIAgent
	if systemPrompt := strings.TrimSpace(req.SystemPrompt); systemPrompt != "" {
		agentConfig.SystemPrompt = systemPrompt
	}
	toolDefs, err := factory.NewToolFactory().BuildMCPTools(agentConfig)
	if err != nil {
		return workflowNodePrepareError(summary, collector, err)
	}
	toolSet, err := filterWorkflowNodeToolSet(ctx, req.ToolSet, req.BlockedToolCodes)
	if err != nil {
		return workflowNodePrepareError(summary, collector, err)
	}
	toolDefs = filterWorkflowNodeToolDefinitions(toolDefs, req.BlockedToolCodes)
	tooling := prepareTooling(toolDefs, nil, toolSet, factory.HasVisibleSkills(agentConfig))
	summary.ToolCodes = append(summary.ToolCodes, tooling.toolCodes...)
	collector.Data.Input.ToolCodes = append(collector.Data.Input.ToolCodes, summary.ToolCodes...)
	collector.SetTooling(tooling.staticToolCodes, definitionToolCodes(tooling.definitions), len(tooling.definitions) > 0)

	agent, err := s.agentFactory.BuildCustomerServiceAgent(ctx, factory.BuildCustomerServiceAgentInput{
		AIAgent:                    agentConfig,
		AIConfig:                   req.AIConfig,
		InstructionToolDefinitions: tooling.definitions,
		DynamicMCPToolDefinitions:  tooling.definitions,
		StaticTools:                tooling.staticTools,
		StaticToolCodes:            tooling.staticToolCodeMap,
		StaticToolMetadata:         tooling.staticToolMetadata,
		Collector:                  collector,
	})
	if err != nil {
		return workflowNodePrepareError(summary, collector, err)
	}
	runner := s.runnerFactory.Build(ctx, agent, false, false)
	if runner == nil {
		return workflowNodePrepareError(summary, collector, fmt.Errorf("failed to build workflow node runner"))
	}

	consumeAgentEvents(runner.Run(ctx, messages), summary, collector, tooling.toolDefsByModelName)
	collector.Data.Status = summary.Status
	collector.Data.Output.ReplyText = summary.ReplyText
	collector.Data.Output.FinishReason = summary.Status
	syncSkillSummaryFromCollector(summary, collector)
	summary.TraceData = collector.Marshal()
	if summary.Status == "error" {
		if strings.TrimSpace(summary.ErrorMessage) == "" {
			summary.ErrorMessage = "workflow Eino node execution failed"
		}
		return summary, fmt.Errorf("%s", summary.ErrorMessage)
	}
	return summary, nil
}

func workflowNodePrepareError(summary *RunResult, collector *callbacks.RuntimeTraceCollector, err error) (*RunResult, error) {
	summary.Status = "error"
	summary.ErrorMessage = err.Error()
	collector.Data.Status = summary.Status
	collector.Data.Error.Message = err.Error()
	collector.Data.Error.Stage = "workflow_node_prepare"
	summary.TraceData = collector.Marshal()
	return summary, err
}

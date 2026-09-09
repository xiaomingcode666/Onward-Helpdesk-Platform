package runtime

import (
	"context"
	"strings"

	"remotehelpdesk/internal/ai/runtime/registry"
	workflowexecutor "remotehelpdesk/internal/ai/runtime/workflow"
	workflowcapability "remotehelpdesk/internal/ai/workflow/capability"
	"remotehelpdesk/internal/ai/workflow/compiler"
	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
)

type DraftRunRequest struct {
	Definition          dsl.Definition
	Conversation        models.Conversation
	UserMessage         models.Message
	AIAgent             models.AIAgent
	AIConfig            models.AIConfig
	AutoConfirm         bool
	BranchOverrides     map[string]string
	SimulateDeviceBound bool
}

func (s *Service) RunDraft(ctx context.Context, req DraftRunRequest) (*workflowexecutor.Result, error) {
	agent := req.AIAgent
	agent.AllowedMCPTools = ""
	agent.AllowedGraphTools = ""
	agent.SkillIDs = ""
	capabilities := workflowcapability.FromDefinition(req.Definition)
	if !capabilities.HumanHandoff {
		agent.TeamIDs = ""
	}
	parts := []string{
		strings.TrimSpace(agent.SystemPrompt),
		strings.TrimSpace(compiler.Compile(req.Definition).Appendix),
		workflowRuntimeActionGuardrails,
		workflowRuntimeResponseStyle,
		"当前为工作流试运行。不得调用外部工具，不得声称测试动作已经真实写入业务系统。",
	}
	sections := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			sections = append(sections, part)
		}
	}
	agent.SystemPrompt = strings.Join(sections, "\n\n")

	return workflowexecutor.NewEngine().Execute(ctx, workflowexecutor.Input{
		Definition:   req.Definition,
		Conversation: req.Conversation,
		UserMessage:  req.UserMessage,
		AIAgent:      agent,
		AIConfig:     req.AIConfig,
		ToolSet: &registry.ToolSet{
			StaticTools:        nil,
			StaticToolCodes:    map[string]string{},
			StaticToolMetadata: map[string]registry.ToolMetadata{},
		},
		AgentRuntime:        s.runtime,
		WorkflowResolver:    workflowVersionResolver{},
		DryRun:              true,
		AutoConfirm:         req.AutoConfirm,
		BranchOverrides:     req.BranchOverrides,
		SimulateDeviceBound: req.SimulateDeviceBound,
	})
}

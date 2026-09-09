package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	coreai "remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/ai/runtime/executor"
	workflowexecutor "remotehelpdesk/internal/ai/runtime/workflow"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

type Service struct {
	runtime *executor.Service
	catalog *toolCatalog
	prepare *prepareService
}

const (
	workflowRunStatusCompleted   = 1
	workflowRunStatusInterrupted = 2
	workflowRunStatusFailed      = 3
)

func NewService() *Service {
	catalog := newToolCatalog()
	return &Service{
		runtime: executor.NewService(),
		catalog: catalog,
		prepare: newPrepareService(catalog),
	}
}

func (s *Service) Run(ctx context.Context, req Request) (*Summary, error) {
	req.UserMessage.Content = utils.BuildRuntimeMessageText(req.UserMessage.MessageType, req.UserMessage.Content)
	if err := validateWorkflowRuntimeScope(req.Conversation, req.AIAgent); err != nil {
		_, _ = writeWorkflowPrepareFailedRun(req, err.Error())
		return nil, err
	}
	aiAgent, workflow, err := prepareWorkflowAgent(req.AIAgent)
	if err != nil {
		_, _ = writeWorkflowPrepareFailedRun(req, err.Error())
		return nil, err
	}
	req.AIAgent = aiAgent
	toolSet, err := s.prepare.prepareToolsForRun(req)
	if err != nil {
		_, _ = writeWorkflowPrepareFailedRun(req, err.Error())
		return nil, err
	}
	req.ToolSet = toolSet
	startedAt := time.Now()
	workflowResult, err := workflowexecutor.NewEngine().Execute(ctx, workflowexecutor.Input{
		Definition:           workflow.Definition,
		Conversation:         req.Conversation,
		UserMessage:          req.UserMessage,
		AIAgent:              req.AIAgent,
		AIConfig:             req.AIConfig,
		ToolSet:              req.ToolSet,
		AgentRuntime:         s.runtime,
		WorkflowResolver:     workflowVersionResolver{},
		EffectStore:          workflowEffectStore{},
		WorkflowVersionStack: []int64{workflow.VersionID},
	})
	if err != nil {
		if workflowResult != nil {
			_, _ = writeWorkflowRun(req, workflow, workflowResult, err.Error(), startedAt)
		}
		return nil, err
	}
	workflowRunID, err := writeWorkflowRun(req, workflow, workflowResult, "", startedAt)
	if err != nil {
		return nil, err
	}
	return toWorkflowSummary(workflowResult, req.AIConfig, workflow, workflowRunID), nil
}

func (s *Service) Resume(ctx context.Context, req ResumeRequest) (*Summary, error) {
	if err := validateWorkflowRuntimeScope(req.Conversation, req.AIAgent); err != nil {
		return nil, err
	}
	aiAgent, workflow, err := prepareWorkflowAgent(req.AIAgent)
	if err != nil {
		return nil, err
	}
	req.AIAgent = aiAgent
	toolSet, err := s.prepare.prepareToolsForResume(req)
	if err != nil {
		return nil, err
	}
	req.ToolSet = toolSet
	if interrupt := repositories.ConversationInterruptRepository.GetByCheckPointID(sqls.DB(), req.CheckPointID); interrupt != nil {
		if strings.TrimSpace(interrupt.RequestData) == "" {
			if interrupt.WorkflowRunID > 0 || strings.HasPrefix(strings.TrimSpace(req.CheckPointID), "workflow:") {
				return nil, errorsx.InvalidParam("workflow checkpoint data is required")
			}
		} else {
			startedAt := time.Now()
			workflowResult, err := workflowexecutor.NewEngine().Resume(ctx, workflowexecutor.Input{
				Definition:           workflow.Definition,
				Conversation:         req.Conversation,
				AIAgent:              req.AIAgent,
				AIConfig:             req.AIConfig,
				ToolSet:              req.ToolSet,
				AgentRuntime:         s.runtime,
				WorkflowResolver:     workflowVersionResolver{},
				EffectStore:          workflowEffectStore{},
				WorkflowVersionStack: []int64{workflow.VersionID},
			}, interrupt.RequestData, firstWorkflowResumeText(req.ResumeData))
			if err != nil {
				if workflowResult != nil {
					_, _ = writeWorkflowRunWithExistingID(Request{
						Conversation: req.Conversation,
						UserMessage:  req.UserMessage,
						AIAgent:      req.AIAgent,
						AIConfig:     req.AIConfig,
					}, workflow, workflowResult, err.Error(), interrupt.WorkflowRunID, startedAt)
				}
				return nil, err
			}
			workflowRunID, err := writeWorkflowRunWithExistingID(Request{
				Conversation: req.Conversation,
				UserMessage:  req.UserMessage,
				AIAgent:      req.AIAgent,
				AIConfig:     req.AIConfig,
			}, workflow, workflowResult, "", interrupt.WorkflowRunID, startedAt)
			if err != nil {
				return nil, err
			}
			return toWorkflowSummary(workflowResult, req.AIConfig, workflow, workflowRunID), nil
		}
	}
	summary, err := s.runtime.ExecuteResume(ctx, executor.ResumeInput{
		Conversation: req.Conversation,
		AIAgent:      req.AIAgent,
		AIConfig:     req.AIConfig,
		CheckPointID: req.CheckPointID,
		ResumeData:   req.ResumeData,
		ToolSet:      req.ToolSet,
	})
	if err != nil {
		return toSummary(summary), err
	}
	return toSummary(summary), nil
}

func firstWorkflowResumeText(data map[string]string) string {
	for _, value := range data {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func toWorkflowSummary(result *workflowexecutor.Result, aiConfig models.AIConfig, workflow resolvedWorkflow, workflowRunID int64) *Summary {
	if result == nil {
		return nil
	}
	trace := map[string]any{
		"status":            result.Status,
		"workflowId":        workflow.WorkflowID,
		"workflowVersionId": workflow.VersionID,
		"workflowRunId":     workflowRunID,
		"nodePath":          result.NodePath,
	}
	if strings.TrimSpace(result.TraceData) != "" {
		var agentRuntimeTrace any
		if err := json.Unmarshal([]byte(result.TraceData), &agentRuntimeTrace); err == nil {
			trace["agentRuntime"] = agentRuntimeTrace
		} else {
			trace["agentRuntime"] = result.TraceData
		}
	}
	traceData, _ := json.Marshal(trace)
	actualModelName := strings.TrimSpace(result.ModelName)
	if actualModelName == "" {
		actualModelName = strings.TrimSpace(aiConfig.ModelName)
	}
	return &Summary{
		Status:                  result.Status,
		ReplyText:               result.ReplyText,
		KnowledgeCitations:      append([]dto.KnowledgeCitation(nil), result.ReplyCitations...),
		PlannedSkillID:          result.SelectedSkillID,
		PlannedSkillName:        result.SelectedSkillName,
		PlanReason:              result.SkillRouteReason,
		SkillRouteTrace:         result.SkillRouteTrace,
		SkillAllowedToolCodes:   append([]string(nil), result.SkillAllowedToolCodes...),
		ModelProvider:           strings.TrimSpace(result.ModelProvider),
		ModelName:               actualModelName,
		ModelCredentialScope:    aiConfig.RuntimeCredentialScope,
		ModelAPIKeyID:           aiConfig.RuntimeAPIKeyID,
		ModelCredentialFallback: aiConfig.RuntimeCredentialFallback,
		PromptTokens:            result.PromptTokens,
		CompletionTokens:        result.CompletionTokens,
		HistoryMessageCount:     result.HistoryMessageCount,
		RetrieverCount:          result.RetrieverCount,
		ToolCallCount:           result.ToolCallCount,
		ToolCodes:               append([]string(nil), result.ToolCodes...),
		InvokedToolCodes:        append([]string(nil), result.InvokedToolCodes...),
		WorkflowID:              workflow.WorkflowID,
		WorkflowVersionID:       workflow.VersionID,
		AgentReleaseID:          workflow.AgentReleaseID,
		WorkflowRunID:           workflowRunID,
		WorkflowNodePath:        append([]string(nil), result.NodePath...),
		TraceData:               string(traceData),
		CheckPointID:            result.CheckPointID,
		CheckPointData:          result.CheckPointData,
		Interrupted:             result.Interrupted,
		Interrupts:              toWorkflowInterruptSummaries(result.Interrupts),
	}
}

func toWorkflowInterruptSummaries(items []workflowexecutor.InterruptSummary) []InterruptContextSummary {
	if len(items) == 0 {
		return nil
	}
	ret := make([]InterruptContextSummary, 0, len(items))
	for _, item := range items {
		ret = append(ret, InterruptContextSummary{
			Type:        item.Type,
			ID:          item.ID,
			InfoPreview: item.InfoPreview,
		})
	}
	return ret
}

func writeWorkflowRun(req Request, workflow resolvedWorkflow, result *workflowexecutor.Result, errorMessage string, startedAt time.Time) (int64, error) {
	return writeWorkflowRunWithExistingID(req, workflow, result, errorMessage, 0, startedAt)
}

func writeWorkflowPrepareFailedRun(req Request, errorMessage string) (int64, error) {
	now := time.Now()
	endedAt := now
	workflowID := int64(0)
	workflowVersionID := req.AIAgent.WorkflowVersionID
	if workflowVersionID > 0 {
		if version := repositories.AIWorkflowVersionRepository.Get(sqls.DB(), workflowVersionID); version != nil {
			workflowID = version.WorkflowID
		}
	}
	run := &models.AIWorkflowRun{
		TenantID:          req.Conversation.TenantID,
		ProductID:         req.Conversation.ProductID,
		WorkflowID:        workflowID,
		WorkflowVersionID: workflowVersionID,
		RuntimeEngine:     workflowexecutor.RuntimeEngineEinoGraph,
		ConversationID:    req.Conversation.ID,
		AIAgentID:         req.AIAgent.ID,
		AgentReleaseID:    req.AIAgent.ActiveReleaseID,
		MessageID:         req.UserMessage.ID,
		Status:            workflowRunStatusFailed,
		StartedAt:         now,
		EndedAt:           &endedAt,
		ErrorMessage:      errorMessage,
		ConfigSnapshot:    buildWorkflowConfigSnapshot(req, resolvedWorkflow{WorkflowID: workflowID, VersionID: workflowVersionID}),
	}
	if err := repositories.AIWorkflowRunRepository.Create(sqls.DB(), run); err != nil {
		return 0, err
	}
	return run.ID, nil
}

func writeWorkflowRunWithExistingID(req Request, workflow resolvedWorkflow, result *workflowexecutor.Result, errorMessage string, existingRunID int64, startedAt time.Time) (int64, error) {
	if result == nil {
		return 0, nil
	}
	now := time.Now()
	if startedAt.IsZero() {
		startedAt = now
	}
	endedAt := now
	nodeTypes := make(map[string]string, len(workflow.Definition.Nodes))
	for _, node := range workflow.Definition.Nodes {
		nodeTypes[node.ID] = node.Type
	}
	runStatus := workflowRunStatus(result.Status, errorMessage)
	var runID int64
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		run := repositories.AIWorkflowRunRepository.Get(ctx.Tx, existingRunID)
		if run == nil {
			run = &models.AIWorkflowRun{
				TenantID:          req.Conversation.TenantID,
				ProductID:         req.Conversation.ProductID,
				WorkflowID:        workflow.WorkflowID,
				WorkflowVersionID: workflow.VersionID,
				DefinitionHash:    workflowDefinitionHash(workflow),
				RuntimeEngine:     workflowexecutor.RuntimeEngineEinoGraph,
				ConversationID:    req.Conversation.ID,
				AIAgentID:         req.AIAgent.ID,
				AgentReleaseID:    workflow.AgentReleaseID,
				MessageID:         req.UserMessage.ID,
				Status:            runStatus,
				StartedAt:         startedAt,
				EndedAt:           &endedAt,
				InterruptType:     firstWorkflowInterruptType(result),
				InterruptNodeID:   firstWorkflowInterruptNodeID(result),
				ErrorMessage:      errorMessage,
				ConfigSnapshot:    buildWorkflowConfigSnapshot(req, workflow),
				TraceData:         result.TraceData,
			}
			if err := repositories.AIWorkflowRunRepository.Create(ctx.Tx, run); err != nil {
				return err
			}
		} else if err := repositories.AIWorkflowRunRepository.Updates(ctx.Tx, run.ID, map[string]any{
			"status":            runStatus,
			"ended_at":          &endedAt,
			"interrupt_type":    firstWorkflowInterruptType(result),
			"interrupt_node_id": firstWorkflowInterruptNodeID(result),
			"error_message":     errorMessage,
			"trace_data":        result.TraceData,
			"updated_at":        now,
		}); err != nil {
			return err
		}
		runID = run.ID
		nodeTraces := result.NodeTraces
		if len(nodeTraces) == 0 {
			nodeTraces = fallbackWorkflowNodeTraces(result.NodePath, nodeTypes, result.Status)
		}
		for _, nodeTrace := range nodeTraces {
			attempt := nodeTrace.Attempt
			if attempt <= 0 {
				attempt = 1
			}
			idempotencyKey := strings.TrimSpace(nodeTrace.IdempotencyKey)
			if idempotencyKey == "" {
				idempotencyKey = fmt.Sprintf("workflow-node-run:%d:%s:%d", run.ID, nodeTrace.NodeID, attempt)
			}
			nodeStartedAt := now.Add(-time.Duration(nodeTrace.DurationMS) * time.Millisecond)
			nodeRun := &models.AIWorkflowNodeRun{
				WorkflowRunID:  run.ID,
				NodeID:         nodeTrace.NodeID,
				NodeType:       firstNonEmpty(nodeTrace.NodeType, nodeTypes[nodeTrace.NodeID]),
				Attempt:        attempt,
				IdempotencyKey: idempotencyKey,
				Status:         workflowRunStatus(nodeTrace.Status, nodeTrace.ErrorMessage),
				InputPreview:   nodeTrace.InputPreview,
				OutputPreview:  nodeTrace.OutputPreview,
				ErrorMessage:   nodeTrace.ErrorMessage,
				StartedAt:      nodeStartedAt,
				EndedAt:        &endedAt,
				DurationMS:     nodeTrace.DurationMS,
			}
			if err := repositories.AIWorkflowNodeRunRepository.Create(ctx.Tx, nodeRun); err != nil {
				return err
			}
		}
		if err := syncWorkflowSkillRunLog(ctx.Tx, run.ID, req, result, errorMessage); err != nil {
			return err
		}
		if err := syncWorkflowRunIDToGeneratedMessages(ctx.Tx, req, run.ID, now); err != nil {
			return err
		}
		return nil
	})
	return runID, err
}

func syncWorkflowRunIDToGeneratedMessages(db *gorm.DB, req Request, workflowRunID int64, now time.Time) error {
	if db == nil || workflowRunID <= 0 || req.Conversation.ID <= 0 {
		return nil
	}
	requestID := strings.TrimSpace(req.UserMessage.RequestID)
	if requestID == "" {
		return nil
	}
	return db.Model(&models.Message{}).
		Where("conversation_id = ? AND request_id = ? AND workflow_run_id = ? AND sender_type IN ?",
			req.Conversation.ID,
			requestID,
			int64(0),
			[]enums.IMSenderType{enums.IMSenderTypeAI, enums.IMSenderTypeSystem},
		).
		Updates(map[string]any{
			"workflow_run_id": workflowRunID,
			"updated_at":      now,
		}).Error
}

func syncWorkflowSkillRunLog(db *gorm.DB, workflowRunID int64, req Request, result *workflowexecutor.Result, errorMessage string) error {
	if db == nil || workflowRunID <= 0 || result == nil || result.SelectedSkillID <= 0 {
		return nil
	}

	now := time.Now()
	usedModel := strings.TrimSpace(result.ModelName)
	if usedModel == "" {
		usedModel = strings.TrimSpace(req.AIConfig.ModelName)
	}
	usedProvider := enums.AIProvider(strings.TrimSpace(result.ModelProvider))
	if usedProvider == "" {
		usedProvider = req.AIConfig.Provider
	}
	values := map[string]any{
		"tenant_id":           req.Conversation.TenantID,
		"conversation_id":     req.Conversation.ID,
		"workflow_run_id":     workflowRunID,
		"source_message_id":   req.UserMessage.ID,
		"ai_agent_id":         req.AIAgent.ID,
		"ai_config_id":        req.AIConfig.ID,
		"skill_definition_id": result.SelectedSkillID,
		"user_message":        req.UserMessage.Content,
		"matched":             true,
		"match_reason":        strings.TrimSpace(result.SkillRouteReason),
		"final_selected":      true,
		"used_model":          usedModel,
		"used_provider":       usedProvider,
		"error_message":       strings.TrimSpace(errorMessage),
		"trace_data":          strings.TrimSpace(result.SkillRouteTrace),
	}
	current := repositories.SkillRunLogRepository.Take(db, "workflow_run_id = ?", workflowRunID)
	if current != nil {
		return repositories.SkillRunLogRepository.Updates(db, current.ID, values)
	}

	return repositories.SkillRunLogRepository.Create(db, &models.SkillRunLog{
		TenantID:          req.Conversation.TenantID,
		ConversationID:    req.Conversation.ID,
		WorkflowRunID:     workflowRunID,
		SourceMessageID:   req.UserMessage.ID,
		AIAgentID:         req.AIAgent.ID,
		AIConfigID:        req.AIConfig.ID,
		SkillDefinitionID: result.SelectedSkillID,
		UserMessage:       req.UserMessage.Content,
		Matched:           true,
		MatchReason:       strings.TrimSpace(result.SkillRouteReason),
		FinalSelected:     true,
		UsedModel:         usedModel,
		UsedProvider:      usedProvider,
		ErrorMessage:      strings.TrimSpace(errorMessage),
		TraceData:         strings.TrimSpace(result.SkillRouteTrace),
		CreatedAt:         now,
	})
}

func validateWorkflowRuntimeScope(conversation models.Conversation, agent models.AIAgent) error {
	if agent.TenantID > 0 && conversation.TenantID != agent.TenantID {
		return errorsx.Forbidden("AI Agent does not belong to the conversation tenant")
	}
	if agent.ProductID > 0 && conversation.ProductID != agent.ProductID {
		return errorsx.Forbidden("AI Agent does not belong to the conversation product")
	}
	if conversation.AIAgentID > 0 && agent.ID > 0 && conversation.AIAgentID != agent.ID {
		return errorsx.Forbidden("AI Agent is not bound to the conversation")
	}
	return nil
}

func workflowDefinitionHash(workflow resolvedWorkflow) string {
	if value := strings.TrimSpace(workflow.DefinitionHash); value != "" {
		return value
	}
	raw, err := json.Marshal(workflow.Definition)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(raw)
	return fmt.Sprintf("%x", digest[:])
}

func buildWorkflowConfigSnapshot(req Request, workflow resolvedWorkflow) string {
	promptDigest := sha256.Sum256([]byte(req.AIAgent.SystemPrompt))
	modelSnapshot := map[string]any{
		"configId":           req.AIConfig.ID,
		"provider":           req.AIConfig.Provider,
		"modelType":          req.AIConfig.ModelType,
		"modelName":          req.AIConfig.ModelName,
		"timeoutMs":          req.AIConfig.TimeoutMS,
		"maxRetryCount":      req.AIConfig.MaxRetryCount,
		"maxOutputTokens":    req.AIConfig.MaxOutputTokens,
		"credentialScope":    req.AIConfig.RuntimeCredentialScope,
		"apiKeyId":           req.AIConfig.RuntimeAPIKeyID,
		"credentialFallback": req.AIConfig.RuntimeCredentialFallback,
	}
	if fallback, err := coreai.ResolveEnvironmentLLMFallbackConfig(); err == nil && fallback != nil {
		modelSnapshot["fallback"] = map[string]any{
			"provider":      fallback.Provider,
			"modelName":     fallback.ModelName,
			"timeoutMs":     fallback.TimeoutMS,
			"maxRetryCount": fallback.MaxRetryCount,
		}
	}
	snapshot := map[string]any{
		"schemaVersion": 1,
		"scope": map[string]any{
			"tenantId":       req.Conversation.TenantID,
			"productId":      req.Conversation.ProductID,
			"conversationId": req.Conversation.ID,
			"messageId":      req.UserMessage.ID,
		},
		"workflow": map[string]any{
			"id":             workflow.WorkflowID,
			"versionId":      workflow.VersionID,
			"definitionHash": workflowDefinitionHash(workflow),
			"runtimeEngine":  workflowexecutor.RuntimeEngineEinoGraph,
		},
		"release": map[string]any{
			"id":                 workflow.AgentReleaseID,
			"agentConfigHash":    workflow.AgentConfigHash,
			"knowledgeScopeHash": workflow.KnowledgeScopeHash,
		},
		"agent": map[string]any{
			"id":                 req.AIAgent.ID,
			"tenantId":           req.AIAgent.TenantID,
			"productId":          req.AIAgent.ProductID,
			"aiConfigId":         req.AIAgent.AIConfigID,
			"llmModelName":       req.AIAgent.LLMModelName,
			"workflowVersionId":  req.AIAgent.WorkflowVersionID,
			"skillIds":           utils.SplitInt64s(req.AIAgent.SkillIDs),
			"allowedMcpTools":    req.AIAgent.AllowedMCPTools,
			"allowedGraphTools":  req.AIAgent.AllowedGraphTools,
			"systemPromptSha256": fmt.Sprintf("%x", promptDigest[:]),
		},
		"model": modelSnapshot,
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func workflowRunStatus(status string, errorMessage string) int {
	if strings.TrimSpace(errorMessage) != "" || strings.TrimSpace(status) == "error" {
		return workflowRunStatusFailed
	}
	switch strings.TrimSpace(status) {
	case "interrupted":
		return workflowRunStatusInterrupted
	default:
		return workflowRunStatusCompleted
	}
}

func fallbackWorkflowNodeTraces(nodePath []string, nodeTypes map[string]string, status string) []workflowexecutor.NodeTrace {
	ret := make([]workflowexecutor.NodeTrace, 0, len(nodePath))
	for _, nodeID := range nodePath {
		ret = append(ret, workflowexecutor.NodeTrace{
			NodeID:   nodeID,
			NodeType: nodeTypes[nodeID],
			Status:   status,
		})
	}
	return ret
}

func firstWorkflowInterruptType(result *workflowexecutor.Result) string {
	if result == nil || len(result.Interrupts) == 0 {
		return ""
	}
	return strings.TrimSpace(result.Interrupts[0].Type)
}

func firstWorkflowInterruptNodeID(result *workflowexecutor.Result) string {
	if result == nil || len(result.Interrupts) == 0 {
		return ""
	}
	return strings.TrimSpace(result.Interrupts[0].ID)
}

func firstNonEmpty(items ...string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}

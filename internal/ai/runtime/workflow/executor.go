package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	runtimeexecutor "remotehelpdesk/internal/ai/runtime/executor"
	"remotehelpdesk/internal/ai/runtime/graphs"
	runtimeintent "remotehelpdesk/internal/ai/runtime/intent"
	"remotehelpdesk/internal/ai/runtime/internal/impl/retrievers"
	"remotehelpdesk/internal/ai/runtime/registry"
	workflowcapability "remotehelpdesk/internal/ai/workflow/capability"
	"remotehelpdesk/internal/ai/workflow/dsl"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/toolx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/services"
)

const maxWorkflowSteps = 128

var workflowHTMLTagPattern = regexp.MustCompile(`<[^>]+>`)

type Input struct {
	Definition           dsl.Definition
	Conversation         models.Conversation
	UserMessage          models.Message
	AIAgent              models.AIAgent
	AIConfig             models.AIConfig
	ToolSet              *registry.ToolSet
	AgentRuntime         AgentNodeRuntime
	WorkflowResolver     WorkflowVersionResolver
	EffectStore          WorkflowEffectStore
	WorkflowVersionStack []int64
	WorkflowNodeStack    []string
	DryRun               bool
	AutoConfirm          bool
	BranchOverrides      map[string]string
	SimulateDeviceBound  bool
}

type AgentNodeRuntime interface {
	ExecuteWorkflowNode(context.Context, runtimeexecutor.WorkflowNodeInput) (*runtimeexecutor.RunResult, error)
}

type WorkflowVersionReference struct {
	TenantID          int64
	ProductID         int64
	WorkflowVersionID int64
}

type ResolvedWorkflowVersion struct {
	WorkflowID     int64
	VersionID      int64
	DefinitionHash string
	Definition     dsl.Definition
}

type WorkflowVersionResolver interface {
	ResolveWorkflowVersion(context.Context, WorkflowVersionReference) (ResolvedWorkflowVersion, error)
}

type WorkflowEffectRequest struct {
	TenantID          int64
	ProductID         int64
	WorkflowVersionID int64
	AgentReleaseID    int64
	ConversationID    int64
	MessageID         int64
	NodeID            string
	EffectType        string
	IdempotencyKey    string
	RequestData       string
}

type WorkflowEffectLease struct {
	ID         int64
	Attempt    int
	Acquired   bool
	Succeeded  bool
	ResultData string
}

type WorkflowEffectStore interface {
	Acquire(context.Context, WorkflowEffectRequest) (WorkflowEffectLease, error)
	Complete(context.Context, WorkflowEffectLease, WorkflowEffectRequest, string) error
	Fail(context.Context, WorkflowEffectLease, string) error
}

type Result struct {
	Status                string
	ReplyText             string
	ReplyCitations        []dto.KnowledgeCitation
	NodePath              []string
	NodeTraces            []NodeTrace
	SelectedSkillID       int64
	SelectedSkillName     string
	SkillRouteReason      string
	SkillRouteTrace       string
	SkillAllowedToolCodes []string
	ModelProvider         string
	ModelName             string
	PromptTokens          int
	CompletionTokens      int
	HistoryMessageCount   int
	RetrieverCount        int
	ToolCallCount         int
	ToolCodes             []string
	InvokedToolCodes      []string
	TraceData             string
	CheckPointID          string
	CheckPointData        string
	Interrupted           bool
	Interrupts            []InterruptSummary
}

type NodeTrace struct {
	NodeID         string
	NodeType       string
	Attempt        int
	IdempotencyKey string
	Status         string
	InputPreview   string
	OutputPreview  string
	ErrorMessage   string
	DurationMS     int
}

type InterruptSummary struct {
	Type        string
	ID          string
	InfoPreview string
}

type Executor struct{}

func NewExecutor() *Executor {
	return &Executor{}
}

type runState struct {
	input           Input
	nodesByID       map[string]dsl.Node
	outgoing        map[string][]dsl.Edge
	vars            map[string]map[string]any
	branchDecisions map[string]branchDecision
	result          Result
	stop            bool
}

type workflowCheckPoint struct {
	Definition    dsl.Definition            `json:"definition"`
	ConfirmNodeID string                    `json:"confirmNodeId"`
	Vars          map[string]map[string]any `json:"vars"`
}

type branchDecision struct {
	SelectedEdgeID       string                `json:"selectedEdgeId,omitempty"`
	SelectedBranchID     string                `json:"selectedBranchId,omitempty"`
	SelectedBranchName   string                `json:"selectedBranchName,omitempty"`
	SelectedTargetNodeID string                `json:"selectedTargetNodeId,omitempty"`
	Reason               string                `json:"reason"`
	Evaluations          []conditionEvaluation `json:"evaluations,omitempty"`
}

type conditionEvaluation struct {
	EdgeID       string `json:"edgeId"`
	BranchID     string `json:"branchId,omitempty"`
	BranchName   string `json:"branchName,omitempty"`
	TargetNodeID string `json:"targetNodeId"`
	SourceNodeID string `json:"sourceNodeId,omitempty"`
	SourceField  string `json:"sourceField,omitempty"`
	Operator     string `json:"operator,omitempty"`
	LeftValue    any    `json:"leftValue,omitempty"`
	RightValue   any    `json:"rightValue,omitempty"`
	Matched      bool   `json:"matched"`
}

func (e *Executor) Execute(ctx context.Context, input Input) (*Result, error) {
	state := newRunState(input)
	currentID := strings.TrimSpace(input.Definition.EntryNodeID)
	if currentID == "" {
		return nil, fmt.Errorf("workflow entry node is required")
	}
	return e.executeFrom(ctx, state, currentID)
}

func (e *Executor) Resume(ctx context.Context, input Input, checkPointData string, resumeText string) (*Result, error) {
	var checkpoint workflowCheckPoint
	if err := json.Unmarshal([]byte(strings.TrimSpace(checkPointData)), &checkpoint); err != nil {
		return nil, fmt.Errorf("invalid workflow checkpoint: %w", err)
	}
	if len(checkpoint.Definition.Nodes) > 0 {
		input.Definition = checkpoint.Definition
	}
	state := newRunState(input)
	state.vars = checkpoint.Vars
	if state.vars == nil {
		state.vars = make(map[string]map[string]any)
	}
	confirmNodeID := strings.TrimSpace(checkpoint.ConfirmNodeID)
	if confirmNodeID == "" {
		return nil, fmt.Errorf("workflow checkpoint confirm node is required")
	}
	decision := graphs.ParseConfirmationDecision(resumeText)
	if decision == "" {
		node, ok := state.nodesByID[confirmNodeID]
		if !ok {
			return nil, fmt.Errorf("workflow node does not exist: %s", confirmNodeID)
		}
		if err := e.executeHumanConfirm(state, node); err != nil {
			return nil, err
		}
		state.result.Status = "interrupted"
		return &state.result, nil
	}
	state.setNodeVars(confirmNodeID, map[string]any{
		"confirmed":    decision == graphs.ConfirmationDecisionConfirm,
		"responseText": strings.TrimSpace(resumeText),
	})
	nextID, ok, err := state.nextNodeID(confirmNodeID)
	if err != nil {
		return nil, err
	}
	if !ok {
		state.result.Status = "completed"
		return &state.result, nil
	}
	return e.executeFrom(ctx, state, nextID)
}

func (e *Executor) executeFrom(ctx context.Context, state *runState, currentID string) (*Result, error) {
	for step := 0; step < maxWorkflowSteps; step++ {
		node, ok := state.nodesByID[currentID]
		if !ok {
			err := fmt.Errorf("workflow node does not exist: %s", currentID)
			state.result.Status = "error"
			return &state.result, err
		}
		state.result.NodePath = append(state.result.NodePath, node.ID)
		trace := NodeTrace{
			NodeID:       node.ID,
			NodeType:     node.Type,
			Status:       "running",
			InputPreview: workflowPreviewJSON(state.nodeInputPreview(node)),
		}
		startedAt := time.Now()
		if err := e.executeNode(ctx, state, node); err != nil {
			trace.Status = "failed"
			trace.ErrorMessage = err.Error()
			trace.DurationMS = int(time.Since(startedAt).Milliseconds())
			if errorTargetNodeID := strings.TrimSpace(node.ErrorTargetNodeID); errorTargetNodeID != "" {
				if _, exists := state.nodesByID[errorTargetNodeID]; !exists {
					trace.ErrorMessage = fmt.Sprintf("%s; workflow error target does not exist: %s", err.Error(), errorTargetNodeID)
					state.result.NodeTraces = append(state.result.NodeTraces, trace)
					state.result.Status = "error"
					return &state.result, errors.New(trace.ErrorMessage)
				}
				trace.Status = "recovered"
				state.setNodeVars(node.ID, map[string]any{
					"errorMessage":      err.Error(),
					"errorRecovered":    true,
					"errorTargetNodeId": errorTargetNodeID,
				})
				trace.OutputPreview = workflowPreviewJSON(state.nodeOutputPreview(node.ID))
				state.result.NodeTraces = append(state.result.NodeTraces, trace)
				currentID = errorTargetNodeID
				continue
			}
			state.result.NodeTraces = append(state.result.NodeTraces, trace)
			state.result.Status = "error"
			return &state.result, err
		}
		trace.DurationMS = int(time.Since(startedAt).Milliseconds())
		if state.result.Interrupted {
			trace.OutputPreview = workflowPreviewJSON(state.nodeOutputPreview(node.ID))
			trace.Status = "interrupted"
			state.result.NodeTraces = append(state.result.NodeTraces, trace)
			state.result.Status = "interrupted"
			return &state.result, nil
		}
		if state.stop {
			trace.OutputPreview = workflowPreviewJSON(state.nodeOutputPreview(node.ID))
			trace.Status = "completed"
			state.result.NodeTraces = append(state.result.NodeTraces, trace)
			state.result.Status = "completed"
			return &state.result, nil
		}
		if node.Type == workflowregistry.NodeTypeEnd {
			trace.OutputPreview = workflowPreviewJSON(state.nodeOutputPreview(node.ID))
			trace.Status = "completed"
			state.result.NodeTraces = append(state.result.NodeTraces, trace)
			state.result.Status = "completed"
			return &state.result, nil
		}
		nextID, ok, err := state.nextNodeID(node.ID)
		if err != nil {
			trace.OutputPreview = workflowPreviewJSON(state.nodeOutputPreview(node.ID))
			trace.Status = "failed"
			trace.ErrorMessage = err.Error()
			trace.DurationMS = int(time.Since(startedAt).Milliseconds())
			state.result.NodeTraces = append(state.result.NodeTraces, trace)
			state.result.Status = "error"
			return &state.result, err
		}
		trace.OutputPreview = workflowPreviewJSON(state.nodeOutputPreview(node.ID))
		trace.Status = "completed"
		state.result.NodeTraces = append(state.result.NodeTraces, trace)
		if !ok {
			state.result.Status = "completed"
			return &state.result, nil
		}
		currentID = nextID
	}
	err := fmt.Errorf("workflow exceeded max steps")
	state.result.Status = "error"
	return &state.result, err
}

func newRunState(input Input) *runState {
	state := &runState{
		input:           input,
		nodesByID:       make(map[string]dsl.Node, len(input.Definition.Nodes)),
		outgoing:        make(map[string][]dsl.Edge),
		vars:            make(map[string]map[string]any),
		branchDecisions: make(map[string]branchDecision),
		result: Result{
			Status:     "started",
			NodePath:   make([]string, 0),
			NodeTraces: make([]NodeTrace, 0),
		},
	}
	for _, node := range input.Definition.Nodes {
		node.ID = strings.TrimSpace(node.ID)
		node.Type = strings.TrimSpace(node.Type)
		if node.ID != "" {
			state.nodesByID[node.ID] = node
		}
	}
	for _, edge := range input.Definition.Edges {
		state.outgoing[edge.Source] = append(state.outgoing[edge.Source], edge)
	}
	return state
}

func (e *Executor) executeNode(ctx context.Context, state *runState, node dsl.Node) error {
	if state.input.DryRun && isWorkflowEffectNode(node.Type) {
		return executeDryRunWorkflowEffect(state, node)
	}
	switch node.Type {
	case workflowregistry.NodeTypeStart:
		knowledgeScope := retrievers.ScopeFromConversation(state.input.Conversation)
		state.setNodeVars(node.ID, map[string]any{
			"tenantId":                state.input.Conversation.TenantID,
			"productId":               state.input.Conversation.ProductID,
			"productModelId":          state.input.Conversation.ProductModelID,
			"deviceId":                state.input.Conversation.DeviceID,
			"serviceCodeId":           state.input.Conversation.ServiceCodeID,
			"customerEntrySessionId":  state.input.Conversation.CustomerEntrySessionID,
			"conversationId":          state.input.Conversation.ID,
			"conversationServiceMode": workflowConversationServiceMode(state.input.Conversation.ServiceMode),
			"messageId":               state.input.UserMessage.ID,
			"aiAgentId":               state.input.AIAgent.ID,
			"agentReleaseId":          state.input.AIAgent.ActiveReleaseID,
			"userMessage":             strings.TrimSpace(state.input.UserMessage.Content),
			"locale":                  knowledgeScope.Locale,
			"regionCode":              knowledgeScope.RegionCode,
			"audience":                knowledgeScope.Audience,
			"conversationState":       state.input.Conversation.Status,
		})
	case workflowregistry.NodeTypeEntryContext:
		return e.executeEntryContext(state, node)
	case workflowregistry.NodeTypeServiceAccessPolicy:
		return e.executeServiceAccessPolicy(state, node)
	case workflowregistry.NodeTypeConversationUnderstanding:
		return e.executeConversationUnderstanding(state, node)
	case workflowregistry.NodeTypeReplyPolicy:
		return e.executeReplyPolicy(state, node)
	case workflowregistry.NodeTypeKnowledgeRetrieve:
		return e.executeKnowledgeRetrieve(ctx, state, node)
	case workflowregistry.NodeTypeKnowledgeMerge:
		return e.executeKnowledgeMerge(state, node)
	case workflowregistry.NodeTypeAnswerabilityGate:
		return e.executeAnswerabilityGate(state, node)
	case workflowregistry.NodeTypeCondition:
		state.setNodeVars(node.ID, map[string]any{"matched": true})
	case workflowregistry.NodeTypeAnalyzeConversation:
		return e.executeAnalyzeConversation(ctx, state, node)
	case workflowregistry.NodeTypePrepareTicketDraft:
		return e.executePrepareTicketDraft(ctx, state, node)
	case workflowregistry.NodeTypeHumanConfirm:
		return e.executeHumanConfirm(state, node)
	case workflowregistry.NodeTypeCreateTicket:
		return e.executeCreateTicket(state, node)
	case workflowregistry.NodeTypeCreateVideoMeeting:
		return e.executeCreateVideoMeeting(ctx, state, node)
	case workflowregistry.NodeTypeCreateKnowledgeCandidate:
		return e.executeCreateKnowledgeCandidate(state, node)
	case workflowregistry.NodeTypeLLMReply:
		return e.executeLLMReply(ctx, state, node)
	case workflowregistry.NodeTypeSendReply:
		replyText := strings.TrimSpace(toString(state.resolveInput(node, "replyText")))
		state.result.ReplyText = replyText
		state.result.ReplyCitations = knowledgeCitationsFromValue(state.resolveInput(node, "citations"))
		state.setNodeVars(node.ID, map[string]any{
			"sent":           replyText != "",
			"replyMessageId": int64(0),
		})
	case workflowregistry.NodeTypeHandoffToHuman:
		return e.executeHandoffToHuman(state, node)
	case workflowregistry.NodeTypeEnd:
		state.setNodeVars(node.ID, map[string]any{"status": "completed"})
	default:
		return fmt.Errorf("unsupported workflow node type: %s", node.Type)
	}
	return nil
}

func (e *Executor) executeConversationUnderstanding(state *runState, node dsl.Node) error {
	rawMessage := strings.TrimSpace(toString(state.resolveInput(node, "userMessage")))
	if rawMessage == "" {
		rawMessage = state.input.UserMessage.Content
	}
	understanding := understandConversationMessage(rawMessage)
	state.setNodeVars(node.ID, map[string]any{
		"normalizedMessage": understanding.NormalizedMessage,
		"messageIntent":     understanding.MessageIntent,
		"answerScope":       understanding.AnswerScope,
		"confidence":        understanding.Confidence,
		"riskSignals":       understanding.RiskSignals,
		"reason":            understanding.Reason,
	})
	return nil
}

func (e *Executor) executeEntryContext(state *runState, node dsl.Node) error {
	config := dsl.EntryContextConfig{}
	if len(node.Config) > 0 {
		if err := json.Unmarshal(node.Config, &config); err != nil {
			return fmt.Errorf("entry_context config is invalid: %w", err)
		}
	}
	unboundMode := strings.TrimSpace(config.UnboundMode)
	if unboundMode == "" {
		unboundMode = "quick_ai"
	}
	productID := toInt64(state.resolveInput(node, "productId"))
	productModelID := toInt64(state.resolveInput(node, "productModelId"))
	deviceID := toInt64(state.resolveInput(node, "deviceId"))
	serviceCodeID := toInt64(state.resolveInput(node, "serviceCodeId"))
	entrySessionID := toInt64(state.resolveInput(node, "customerEntrySessionId"))

	entryMode := "quick_ai"
	contextLevel := "general"
	deviceBound := deviceID > 0 || state.input.SimulateDeviceBound
	productBound := productID > 0 || productModelID > 0 || deviceBound
	requiresContextCollection := false
	reason := "conversation has no service-code or device context"
	switch {
	case deviceBound && serviceCodeID > 0:
		entryMode = "device_service_code"
		contextLevel = "device"
		reason = "service code resolved to a bound device"
	case deviceBound:
		entryMode = "bound_device"
		contextLevel = "device"
		reason = "conversation is already bound to a device"
	case serviceCodeID > 0:
		entryMode = "general_service_code"
		contextLevel = "product"
		requiresContextCollection = true
		reason = "general service code requires device or现场 context collection"
	case entrySessionID > 0 || (unboundMode == "product_context" && productBound):
		entryMode = "product_context"
		contextLevel = "product"
		requiresContextCollection = true
		reason = "product context is available but no concrete device is bound"
	case unboundMode == "product_context":
		entryMode = "context_collection"
		requiresContextCollection = true
		reason = "workflow requires product context before service actions"
	}
	state.setNodeVars(node.ID, map[string]any{
		"entryMode":                 entryMode,
		"contextLevel":              contextLevel,
		"deviceBound":               deviceBound,
		"productBound":              productBound,
		"requiresContextCollection": requiresContextCollection,
		"reason":                    reason,
	})
	return nil
}

func (e *Executor) executeServiceAccessPolicy(state *runState, node dsl.Node) error {
	config := dsl.ServiceAccessPolicyConfig{}
	if len(node.Config) > 0 {
		if err := json.Unmarshal(node.Config, &config); err != nil {
			return fmt.Errorf("service_access_policy config is invalid: %w", err)
		}
	}
	handoffMode := strings.TrimSpace(config.HumanHandoffMode)
	if handoffMode == "" {
		handoffMode = "device_only"
	}
	ticketMode := strings.TrimSpace(config.TicketAccessMode)
	if ticketMode == "" {
		ticketMode = "device_only"
	}
	deviceBound := truthy(state.resolveInput(node, "deviceBound"))
	contextLevel := strings.TrimSpace(toString(state.resolveInput(node, "contextLevel")))
	conversationMode := strings.TrimSpace(toString(state.resolveInput(node, "conversationServiceMode")))
	if conversationMode == "" {
		conversationMode = workflowConversationServiceMode(state.input.Conversation.ServiceMode)
	}
	capabilities := workflowcapability.FromDefinition(state.input.Definition)
	inheritedHandoff := conversationMode != "ai_only" && capabilities.HumanHandoff
	allowHumanHandoff := workflowAccessRuleAllows(handoffMode, inheritedHandoff, deviceBound) && capabilities.HumanHandoff
	inheritedTicket := conversationMode != "ai_only" && capabilities.TicketCreation
	allowTicketCreation := workflowAccessRuleAllows(ticketMode, inheritedTicket, deviceBound) && capabilities.TicketCreation
	allowVideoMeeting := deviceBound && capabilities.VideoMeeting
	serviceMode := "ai_only"
	conversationTag := "AI问答"
	if conversationMode == "human_only" && allowHumanHandoff {
		serviceMode = "human_only"
		conversationTag = "待回复"
	} else if allowHumanHandoff {
		serviceMode = "ai_first"
	}
	reason := "AI self-service policy applied"
	if allowHumanHandoff {
		reason = "device and enterprise policy allow AI-first human support"
	} else if !deviceBound && handoffMode == "device_only" {
		reason = "human support requires a bound device"
	} else if conversationMode == "ai_only" {
		reason = "conversation is configured as AI-only"
	}
	state.setNodeVars(node.ID, map[string]any{
		"serviceMode":         serviceMode,
		"allowHumanHandoff":   allowHumanHandoff,
		"allowTicketCreation": allowTicketCreation,
		"allowVideoMeeting":   allowVideoMeeting,
		"showHumanEntry":      allowHumanHandoff,
		"showTicketEntry":     allowTicketCreation && deviceBound,
		"showDeviceEntry":     deviceBound || contextLevel == "product",
		"conversationTag":     conversationTag,
		"reason":              reason,
	})
	return nil
}

func workflowConversationServiceMode(mode enums.IMConversationServiceMode) string {
	switch mode {
	case enums.IMConversationServiceModeAIOnly:
		return "ai_only"
	case enums.IMConversationServiceModeHumanOnly:
		return "human_only"
	default:
		return "ai_first"
	}
}

func workflowAccessRuleAllows(rule string, inherited bool, deviceBound bool) bool {
	switch strings.TrimSpace(rule) {
	case "always":
		return true
	case "never":
		return false
	case "device_only":
		return inherited && deviceBound
	default:
		return inherited
	}
}

func (e *Executor) executeReplyPolicy(state *runState, node dsl.Node) error {
	intent := strings.TrimSpace(toString(state.resolveInput(node, "messageIntent")))
	scope := strings.TrimSpace(toString(state.resolveInput(node, "answerScope")))
	userMessage := normalizeWorkflowUserMessage(toString(state.resolveInput(node, "userMessage")))
	capabilities := workflowcapability.FromDefinition(state.input.Definition)
	if _, configured := node.Inputs["allowHumanHandoff"]; configured {
		capabilities.HumanHandoff = capabilities.HumanHandoff && truthy(state.resolveInput(node, "allowHumanHandoff"))
	}
	if _, configured := node.Inputs["allowTicketCreation"]; configured {
		capabilities.TicketCreation = capabilities.TicketCreation && truthy(state.resolveInput(node, "allowTicketCreation"))
	}
	decision := decideWorkflowReplyPolicy(state.input.AIAgent, capabilities, workflowReplyPolicyInput{
		MessageIntent: intent,
		AnswerScope:   scope,
		UserMessage:   userMessage,
		Answerability: strings.TrimSpace(toString(state.resolveInput(node, "answerability"))),
	})
	state.setNodeVars(node.ID, map[string]any{
		"action":           decision.Action,
		"replyText":        decision.ReplyText,
		"reason":           decision.Reason,
		"requiresFlow":     decision.RequiresFlow,
		"targetFlow":       decision.TargetFlow,
		"finalReplySource": decision.FinalReplySource,
	})
	return nil
}

func (e *Executor) executeCreateTicket(state *runState, node dsl.Node) error {
	confirmed := truthy(state.resolveInput(node, "confirmed"))
	if !confirmed {
		state.setNodeVars(node.ID, map[string]any{
			"ticketId": int64(0),
			"created":  false,
		})
		return nil
	}
	draft := asMap(state.resolveInput(node, "ticketDraft"))
	title := strings.TrimSpace(toString(draft["title"]))
	description := strings.TrimSpace(toString(draft["description"]))
	if !truthy(draft["ready"]) || title == "" || description == "" || !graphs.HasSufficientTicketIssueContext(title+"\n"+description) {
		message := "工单信息仍不完整，请补充故障现象、故障码（如有）或具体问题描述后再确认创建。"
		state.setNodeVars(node.ID, map[string]any{
			"ticketId": int64(0),
			"created":  false,
			"message":  message,
		})
		state.result.ReplyText = message
		state.stop = true
		return nil
	}
	item, err := services.TicketService.CreateFromConversation(request.CreateTicketFromConversationRequest{
		IdempotencyKey: fmt.Sprintf("workflow-ticket:%d:%d", state.input.Conversation.ID, state.input.UserMessage.ID),
		ConversationID: state.input.Conversation.ID,
		Title:          title,
		Description:    description,
	}, workflowAIPrincipal(state.input.AIAgent, state.input.Conversation.TenantID))
	if err != nil {
		return err
	}
	state.setNodeVars(node.ID, map[string]any{
		"ticketId": item.ID,
		"ticketNo": item.TicketNo,
		"created":  true,
		"message":  buildTicketCreatedMessage(item),
	})
	return nil
}

func buildTicketCreatedMessage(item *models.Ticket) string {
	if item == nil {
		return "工单已创建。"
	}
	ticketNo := strings.TrimSpace(item.TicketNo)
	if ticketNo == "" {
		return fmt.Sprintf("工单已创建，工单 ID：%d。", item.ID)
	}
	return "工单已创建，工单号：" + ticketNo + "。"
}

func (e *Executor) executeCreateVideoMeeting(ctx context.Context, state *runState, node dsl.Node) error {
	confirmed := truthy(state.resolveInput(node, "confirmed"))
	if !confirmed {
		state.setNodeVars(node.ID, map[string]any{
			"meetingId": "",
			"roomName":  "",
			"created":   false,
			"message":   "已取消创建视频会议。",
		})
		return nil
	}
	ticketID := toInt64(state.resolveInput(node, "ticketId"))
	if ticketID <= 0 {
		return fmt.Errorf("create_video_meeting requires ticketId")
	}
	joinConfig, err := services.MeetingService.CreateMeetingRoomForWorkflow(ctx, state.input.Conversation.TenantID, ticketID)
	if err != nil {
		return err
	}
	state.setNodeVars(node.ID, map[string]any{
		"meetingId": joinConfig.MeetingID,
		"roomName":  joinConfig.RoomName,
		"created":   true,
		"message":   buildMeetingCreatedMessage(joinConfig),
	})
	return nil
}

func buildMeetingCreatedMessage(item *services.JoinConfig) string {
	if item == nil || strings.TrimSpace(item.MeetingID) == "" {
		return "视频会议已创建。"
	}
	if roomName := strings.TrimSpace(item.RoomName); roomName != "" {
		return "视频会议已创建，会议室：" + roomName + "。"
	}
	return "视频会议已创建，会议 ID：" + strings.TrimSpace(item.MeetingID) + "。"
}

func (e *Executor) executeCreateKnowledgeCandidate(state *runState, node dsl.Node) error {
	confirmed := truthy(state.resolveInput(node, "confirmed"))
	if !confirmed {
		state.setNodeVars(node.ID, map[string]any{
			"candidateId":  int64(0),
			"created":      false,
			"reviewStatus": "",
			"message":      "已取消创建知识候选。",
		})
		return nil
	}
	ticketID := toInt64(state.resolveInput(node, "ticketId"))
	if ticketID <= 0 {
		return fmt.Errorf("create_knowledge_candidate requires ticketId")
	}
	result, err := services.TicketKnowledgeCandidateService.Create(state.input.Conversation.TenantID, ticketID, request.CreateTicketKnowledgeCandidateRequest{
		Title:            strings.TrimSpace(toString(state.resolveInput(node, "title"))),
		Suggestion:       strings.TrimSpace(toString(state.resolveInput(node, "suggestion"))),
		RootCauseSummary: strings.TrimSpace(toString(state.resolveInput(node, "rootCauseSummary"))),
		SolutionSummary:  strings.TrimSpace(toString(state.resolveInput(node, "solutionSummary"))),
		KnowledgeBaseID:  toInt64(state.resolveInput(node, "knowledgeBaseId")),
	}, workflowAIPrincipal(state.input.AIAgent, state.input.Conversation.TenantID))
	if err != nil {
		return err
	}
	var candidateID int64
	reviewStatus := ""
	if result != nil && result.Candidate != nil {
		candidateID = result.Candidate.ID
		reviewStatus = strings.TrimSpace(result.Candidate.ReviewStatus)
	}
	created := result != nil && result.Created
	state.setNodeVars(node.ID, map[string]any{
		"candidateId":  candidateID,
		"created":      created,
		"reviewStatus": reviewStatus,
		"message":      buildKnowledgeCandidateMessage(candidateID, created),
	})
	return nil
}

func buildKnowledgeCandidateMessage(candidateID int64, created bool) string {
	if candidateID <= 0 {
		return "知识候选已准备。"
	}
	if created {
		return fmt.Sprintf("知识候选已创建，待知识负责人审核，候选 ID：%d。", candidateID)
	}
	return fmt.Sprintf("该工单已有知识候选，已复用候选 ID：%d。", candidateID)
}

func workflowAIPrincipal(aiAgent models.AIAgent, tenantID int64) *dto.AuthPrincipal {
	username := strings.TrimSpace(aiAgent.Name)
	if username == "" {
		username = "AI"
	}
	return &dto.AuthPrincipal{
		UserID:      0,
		Username:    username,
		Nickname:    username,
		TenantID:    tenantID,
		DomainType:  "service_account",
		SubjectType: "service_account",
	}
}

type workflowConversationUnderstanding struct {
	NormalizedMessage string
	MessageIntent     string
	AnswerScope       string
	Confidence        float64
	RiskSignals       []string
	Reason            string
}

type workflowReplyPolicyInput struct {
	MessageIntent string
	AnswerScope   string
	UserMessage   string
	Answerability string
}

type workflowReplyPolicyDecision struct {
	Action           string
	ReplyText        string
	Reason           string
	RequiresFlow     bool
	TargetFlow       string
	FinalReplySource string
}

func understandConversationMessage(rawMessage string) workflowConversationUnderstanding {
	message := normalizeWorkflowUserMessage(rawMessage)
	ret := workflowConversationUnderstanding{
		NormalizedMessage: message,
		MessageIntent:     "unknown",
		AnswerScope:       "needs_clarification",
		Confidence:        0.5,
		Reason:            "message intent is unclear",
	}
	if message == "" {
		ret.MessageIntent = "unknown"
		ret.AnswerScope = "needs_clarification"
		ret.Confidence = 0.9
		ret.Reason = "empty message"
		return ret
	}
	lower := strings.ToLower(message)
	switch {
	case isGreetingMessage(lower):
		ret.MessageIntent = "greeting"
		ret.AnswerScope = "direct_reply"
		ret.Confidence = 0.98
		ret.Reason = "matched greeting phrase"
	case containsAnyWorkflowText(lower, "谢谢", "感谢", "多谢", "辛苦了", "thank"):
		ret.MessageIntent = "thanks"
		ret.AnswerScope = "direct_reply"
		ret.Confidence = 0.95
		ret.Reason = "matched thanks phrase"
	case containsAnyWorkflowText(lower, "再见", "拜拜", "不用了", "没事了", "结束"):
		ret.MessageIntent = "end_conversation"
		ret.AnswerScope = "direct_reply"
		ret.Confidence = 0.9
		ret.Reason = "matched ending phrase"
	case runtimeintent.IsExplicitHandoffRequest(lower):
		ret.MessageIntent = "handoff_request"
		ret.AnswerScope = "needs_handoff"
		ret.Confidence = 0.95
		ret.RiskSignals = append(ret.RiskSignals, "handoff_requested")
		ret.Reason = "matched handoff phrase"
	case containsAnyWorkflowText(lower, "投诉", "举报", "差评", "曝光", "起诉", "律师", "12315"):
		ret.MessageIntent = "complaint"
		ret.AnswerScope = "needs_handoff"
		ret.Confidence = 0.92
		ret.RiskSignals = append(ret.RiskSignals, "complaint_escalation")
		ret.Reason = "matched complaint phrase"
	case containsAnyWorkflowText(lower, "生气", "愤怒", "垃圾", "太差", "一直没人", "再不处理", "非常失望", "不满意"):
		ret.MessageIntent = "negative_sentiment"
		ret.AnswerScope = "needs_handoff"
		ret.Confidence = 0.9
		ret.RiskSignals = append(ret.RiskSignals, "negative_sentiment")
		ret.Reason = "matched negative sentiment phrase"
	case runtimeintent.IsExplicitTicketRequest(lower):
		ret.MessageIntent = "ticket_request"
		ret.AnswerScope = "needs_ticket"
		ret.Confidence = 0.9
		ret.RiskSignals = append(ret.RiskSignals, "ticket_expected")
		ret.Reason = "matched ticket phrase"
	case hasAfterSalesDiagnosticSignal(lower):
		ret.MessageIntent = "business_question"
		ret.AnswerScope = "needs_knowledge"
		ret.Confidence = 0.86
		ret.Reason = "matched after-sales diagnostic signal"
	case isWorkflowConfirmationMessage(lower):
		ret.MessageIntent = "confirmation"
		ret.AnswerScope = "direct_reply"
		ret.Confidence = 0.8
		ret.Reason = "matched confirmation phrase"
	case isAmbiguousWorkflowQuestion(lower):
		ret.MessageIntent = "ambiguous_question"
		ret.AnswerScope = "needs_clarification"
		ret.Confidence = 0.82
		ret.Reason = "message lacks a concrete business object"
	default:
		ret.MessageIntent = "business_question"
		ret.AnswerScope = "needs_knowledge"
		ret.Confidence = 0.7
		ret.Reason = "default business question policy"
	}
	return ret
}

func decideWorkflowReplyPolicy(aiAgent models.AIAgent, capabilities workflowcapability.Set, input workflowReplyPolicyInput) workflowReplyPolicyDecision {
	intent := strings.TrimSpace(input.MessageIntent)
	scope := strings.TrimSpace(input.AnswerScope)
	if answerability := strings.TrimSpace(input.Answerability); answerability != "" && answerability != "answerable" {
		return workflowReplyPolicyDecision{
			Action:           "knowledge_fallback",
			ReplyText:        workflowKnowledgeFallbackReplyForMessage(aiAgent, capabilities, input.UserMessage),
			Reason:           "knowledge is not sufficient for business answer",
			FinalReplySource: "knowledge_fallback",
		}
	}
	switch {
	case intent == "greeting":
		return workflowReplyPolicyDecision{Action: "direct_reply", ReplyText: workflowPolicyReply(input.UserMessage, "您好，请问有什么可以帮您？", "Hello. How can I help you?"), Reason: "greeting can be answered directly", FinalReplySource: "direct_reply"}
	case intent == "thanks":
		return workflowReplyPolicyDecision{Action: "direct_reply", ReplyText: workflowPolicyReply(input.UserMessage, "不客气，如有其他问题可以继续告诉我。", "You are welcome. Let me know if you have another question."), Reason: "thanks can be answered directly", FinalReplySource: "direct_reply"}
	case intent == "end_conversation":
		return workflowReplyPolicyDecision{Action: "end_conversation", ReplyText: workflowPolicyReply(input.UserMessage, "好的，如后续还有问题可以随时联系。", "Understood. Contact us again if you need further help."), Reason: "conversation ending phrase", FinalReplySource: "direct_reply"}
	case intent == "confirmation":
		return workflowReplyPolicyDecision{Action: "direct_reply", ReplyText: workflowPolicyReply(input.UserMessage, "好的，请继续补充需要处理的问题。", "Understood. Please provide the issue you need help with."), Reason: "confirmation without pending interrupt", FinalReplySource: "direct_reply"}
	case intent == "handoff_request" || intent == "negative_sentiment" || scope == "needs_handoff":
		if !capabilities.HumanHandoff {
			return workflowReplyPolicyDecision{
				Action:           "direct_reply",
				ReplyText:        workflowPolicyReply(input.UserMessage, "当前产品仅提供 AI 客服，不提供转人工。我会继续协助诊断，请补充故障现象、故障码和已尝试的处理步骤。", "This product currently provides AI support only and does not offer human handoff. I can continue diagnosing the issue; please provide the symptoms, fault code, and troubleshooting already attempted."),
				Reason:           "workflow does not provide human handoff",
				FinalReplySource: "capability_boundary",
			}
		}
		return workflowReplyPolicyDecision{Action: "handoff_to_human", Reason: "user requested human support or risk requires handoff", RequiresFlow: true, TargetFlow: "handoff_to_human", FinalReplySource: "handoff_notice"}
	case intent == "ticket_request" || scope == "needs_ticket":
		if !capabilities.TicketCreation {
			return workflowReplyPolicyDecision{
				Action:           "direct_reply",
				ReplyText:        workflowPolicyReply(input.UserMessage, "当前流程不创建工单。我会继续协助诊断，请补充故障现象、故障码和已尝试的处理步骤。", "This workflow does not create tickets. I can continue diagnosing the issue; please provide the symptoms, fault code, and troubleshooting already attempted."),
				Reason:           "workflow does not provide ticket creation",
				FinalReplySource: "capability_boundary",
			}
		}
		return workflowReplyPolicyDecision{Action: "prepare_ticket", Reason: "user requested ticket handling", RequiresFlow: true, TargetFlow: "prepare_ticket", FinalReplySource: "ticket_result"}
	case intent == "ambiguous_question" || scope == "needs_clarification":
		return workflowReplyPolicyDecision{Action: "clarify", ReplyText: workflowPolicyReply(input.UserMessage, "请补充具体的产品、场景、报错信息或你希望处理的结果，我再继续帮你确认。", "Please provide the product, scenario, error details, or the result you need so I can continue checking."), Reason: "message needs clarification", FinalReplySource: "clarification"}
	case scope == "needs_knowledge":
		return workflowReplyPolicyDecision{Action: "retrieve_knowledge", Reason: "business question should be answered with knowledge evidence", RequiresFlow: true, TargetFlow: "knowledge", FinalReplySource: "knowledge_answer"}
	default:
		return workflowReplyPolicyDecision{Action: "clarify", ReplyText: workflowPolicyReply(input.UserMessage, "请补充更具体的问题，我再继续帮你处理。", "Please provide a more specific question so I can continue helping."), Reason: "fallback to clarification for unclear policy input", FinalReplySource: "clarification"}
	}
}

func workflowPolicyReply(userMessage, chinese, english string) string {
	if workflowMessageLooksEnglish(userMessage) {
		return english
	}
	return chinese
}

func normalizeWorkflowUserMessage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = workflowHTMLTagPattern.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	value = strings.Join(strings.Fields(value), " ")
	return strings.TrimSpace(value)
}

func isGreetingMessage(value string) bool {
	trimmed := strings.Trim(value, " 	\r\n。.!！?？~～")
	switch trimmed {
	case "你好", "您好", "你好呀", "您好呀", "在吗", "在不在", "hello", "hi", "hey":
		return true
	default:
		return false
	}
}

func isAmbiguousWorkflowQuestion(value string) bool {
	trimmed := strings.Trim(value, " 	\r\n。.!！?？~～")
	if len([]rune(trimmed)) <= 3 {
		return true
	}
	for _, phrase := range []string{
		"怎么弄", "怎么办", "怎么处理", "帮我看看", "有问题",
		"这个怎么弄", "这个怎么办", "这个怎么处理", "这个有问题",
		"那个怎么弄", "那个怎么办", "那个怎么处理", "那个有问题",
		"它怎么弄", "它怎么办", "它怎么处理", "它有问题",
	} {
		if trimmed == phrase {
			return true
		}
	}
	return false
}

func isWorkflowConfirmationMessage(value string) bool {
	trimmed := strings.Trim(value, " \t\r\n。.!！?？~～")
	if trimmed == "" || hasAfterSalesDiagnosticSignal(trimmed) {
		return false
	}
	shortConfirmations := map[string]struct{}{
		"确认": {}, "可以": {}, "好的": {}, "好": {}, "是的": {}, "同意": {},
		"取消": {}, "不用": {}, "不要": {}, "先不": {}, "不转": {},
	}
	if _, ok := shortConfirmations[trimmed]; ok {
		return true
	}
	if len([]rune(trimmed)) > 24 {
		return false
	}
	return containsAnyWorkflowText(
		trimmed,
		"确认创建", "确认建单", "确认转人工", "同意创建", "同意建单", "同意转人工",
		"可以创建", "可以建单", "可以转人工", "取消创建", "取消建单", "取消转人工",
	)
}

func hasAfterSalesDiagnosticSignal(value string) bool {
	return containsAnyWorkflowText(
		value,
		"故障", "故障码", "报警", "告警", "红灯", "黄灯", "绿灯",
		"上电", "断电", "复位", "重启", "停机", "无法启动",
		"电压", "母线", "电流", "温度", "过热", "异响",
		"端子", "线束", "压力", "模块", "控制器", "风扇",
		"复测", "安全", "处理建议", "排查", "知识库",
		"pwr-", "e42", "rhd-",
	)
}

func containsAnyWorkflowText(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func (e *Executor) executeHumanConfirm(state *runState, node dsl.Node) error {
	if state.input.DryRun && state.input.AutoConfirm {
		state.setNodeVars(node.ID, map[string]any{
			"confirmed": true,
			"decision":  "confirmed",
			"simulated": true,
		})
		return nil
	}
	prompt := strings.TrimSpace(toString(state.resolveInput(node, "prompt")))
	if prompt == "" {
		prompt = "请确认是否继续。"
	}
	infoPreview, err := json.Marshal(map[string]string{"message": prompt})
	if err != nil {
		return err
	}
	state.result.Interrupted = true
	state.result.CheckPointID = buildWorkflowCheckPointID(state.input, node.ID)
	checkpoint, err := json.Marshal(workflowCheckPoint{
		Definition:    state.input.Definition,
		ConfirmNodeID: node.ID,
		Vars:          state.vars,
	})
	if err != nil {
		return err
	}
	state.result.CheckPointData = string(checkpoint)
	state.result.Interrupts = []InterruptSummary{
		{
			Type:        workflowregistry.NodeTypeHumanConfirm,
			ID:          node.ID,
			InfoPreview: string(infoPreview),
		},
	}
	return nil
}

func isWorkflowEffectNode(nodeType string) bool {
	switch nodeType {
	case workflowregistry.NodeTypeCreateTicket,
		workflowregistry.NodeTypeCreateVideoMeeting,
		workflowregistry.NodeTypeCreateKnowledgeCandidate,
		workflowregistry.NodeTypeHandoffToHuman:
		return true
	default:
		return false
	}
}

func executeDryRunWorkflowEffect(state *runState, node dsl.Node) error {
	output := map[string]any{
		"simulated": true,
		"created":   false,
		"message":   "测试模式已验证该动作，未写入真实业务数据。",
	}
	switch node.Type {
	case workflowregistry.NodeTypeCreateTicket:
		output["ticketId"] = int64(0)
		output["ticketNo"] = "TEST-TICKET"
	case workflowregistry.NodeTypeCreateVideoMeeting:
		output["meetingId"] = "TEST-MEETING"
		output["roomName"] = "test-room"
	case workflowregistry.NodeTypeCreateKnowledgeCandidate:
		output["candidateId"] = int64(0)
		output["reviewStatus"] = "simulated"
	case workflowregistry.NodeTypeHandoffToHuman:
		output["handoffId"] = int64(0)
		output["decision"] = "simulated"
		output["reason"] = strings.TrimSpace(toString(state.resolveInput(node, "reason")))
	}
	state.setNodeVars(node.ID, output)
	return nil
}

func buildWorkflowCheckPointID(input Input, nodeID string) string {
	return fmt.Sprintf("workflow:%d:%d:%s", input.Conversation.ID, input.UserMessage.ID, strings.TrimSpace(nodeID))
}

func (e *Executor) executePrepareTicketDraft(ctx context.Context, state *runState, node dsl.Node) error {
	issue := strings.TrimSpace(toString(state.resolveInput(node, "issue")))
	input := graphs.PrepareTicketDraftInput{
		Issue: issue,
	}
	if title := strings.TrimSpace(readStringConfig(node.Config, "title")); title != "" {
		input.Title = title
	}
	if description := strings.TrimSpace(readStringConfig(node.Config, "description")); description != "" {
		input.Description = description
	}
	if impact := strings.TrimSpace(readStringConfig(node.Config, "impact")); impact != "" {
		input.Impact = impact
	}
	if expectedOutcome := strings.TrimSpace(readStringConfig(node.Config, "expectedOutcome")); expectedOutcome != "" {
		input.ExpectedOutcome = expectedOutcome
	}
	if currentAttempt := strings.TrimSpace(readStringConfig(node.Config, "currentAttempt")); currentAttempt != "" {
		input.CurrentAttempt = currentAttempt
	}
	args, err := json.Marshal(input)
	if err != nil {
		return err
	}
	raw, err := graphs.NewPrepareTicketDraftGraph(state.input.Conversation).Run(ctx, string(args))
	if err != nil {
		return err
	}
	var result graphs.PrepareTicketDraftResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return err
	}
	state.setNodeVars(node.ID, map[string]any{
		"ticketDraft": map[string]any{
			"ready":             result.Ready,
			"title":             strings.TrimSpace(result.Title),
			"description":       strings.TrimSpace(result.Description),
			"missingFields":     result.MissingFields,
			"followUpQuestions": result.FollowUpQuestions,
			"conversationFacts": result.ConversationFacts,
		},
	})
	if !result.Ready {
		state.result.ReplyText = "创建工单前，请先补充故障现象、故障码（如有）或具体问题描述。至少提供一项关键信息，我会整理草稿后再请您确认。"
		state.stop = true
	}
	return nil
}

func (e *Executor) executeAnalyzeConversation(ctx context.Context, state *runState, node dsl.Node) error {
	userMessage := strings.TrimSpace(toString(state.resolveInput(node, "userMessage")))
	input := graphs.AnalyzeConversationInput{
		ObservedIssue: userMessage,
	}
	if strings.TrimSpace(readStringConfig(node.Config, "goal")) != "" {
		input.Goal = strings.TrimSpace(readStringConfig(node.Config, "goal"))
	}
	if readBoolConfig(node.Config, "needTicket") {
		input.NeedTicket = true
	}
	if readBoolConfig(node.Config, "needHumanHandoff") {
		input.NeedHumanHandoff = true
	}
	if readBoolConfig(node.Config, "needQualityCheck") {
		input.NeedQualityCheck = true
	}
	if strings.TrimSpace(readStringConfig(node.Config, "additionalContext")) != "" {
		input.AdditionalContext = strings.TrimSpace(readStringConfig(node.Config, "additionalContext"))
	}
	args, err := json.Marshal(input)
	if err != nil {
		return err
	}
	raw, err := graphs.NewAnalyzeConversationGraph(state.input.Conversation).Run(ctx, string(args))
	if err != nil {
		return err
	}
	var result graphs.AnalyzeConversationResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return err
	}
	nextAction := strings.TrimSpace(result.RecommendedNextAction)
	state.setNodeVars(node.ID, map[string]any{
		"summary":               strings.TrimSpace(result.Summary),
		"intent":                strings.TrimSpace(result.UserIntent),
		"riskLevel":             strings.TrimSpace(result.RiskLevel),
		"riskSignals":           result.RiskSignals,
		"recommendedNextAction": nextAction,
		"recommendedQuestions":  result.RecommendedQuestions,
		"conversationFacts":     result.ConversationFacts,
		"needTicket":            nextAction == "prepare_ticket",
		"needHumanHandoff":      nextAction == "handoff_to_human",
	})
	return nil
}

func (e *Executor) executeHandoffToHuman(state *runState, node dsl.Node) error {
	if _, hasConfirmedInput := node.Inputs["confirmed"]; hasConfirmedInput && !truthy(state.resolveInput(node, "confirmed")) {
		state.setNodeVars(node.ID, map[string]any{
			"handoffId":  int64(0),
			"reason":     strings.TrimSpace(toString(state.resolveInput(node, "reason"))),
			"decision":   "cancelled",
			"teamId":     int64(0),
			"assigneeId": int64(0),
			"message":    "",
			"skipped":    true,
		})
		return nil
	}
	reason := strings.TrimSpace(toString(state.resolveInput(node, "reason")))
	result, err := services.ConversationHumanDispatchService.HandoffByAIWithRequestID(
		state.input.Conversation.ID,
		state.input.AIAgent,
		reason,
		strings.TrimSpace(state.input.UserMessage.RequestID),
	)
	if err != nil {
		return err
	}
	output := map[string]any{
		"handoffId":  int64(0),
		"reason":     reason,
		"decision":   "",
		"teamId":     int64(0),
		"assigneeId": int64(0),
		"message":    "",
	}
	if result != nil {
		output["decision"] = string(result.Decision)
		output["teamId"] = result.TeamID
		output["assigneeId"] = result.AssigneeID
		output["ticketId"] = result.TicketID
		output["ticketNo"] = result.TicketNo
		output["ticketCreated"] = result.TicketCreated
		output["message"] = strings.TrimSpace(result.Message)
	}
	state.setNodeVars(node.ID, output)
	return nil
}

func (e *Executor) executeKnowledgeRetrieve(ctx context.Context, state *runState, node dsl.Node) error {
	query := strings.TrimSpace(toString(state.resolveInput(node, "query")))
	config := dsl.KnowledgeRetrieveConfig{}
	if err := json.Unmarshal(node.Config, &config); err != nil {
		return fmt.Errorf("knowledge_retrieve config is invalid: %w", err)
	}
	scope := retrievers.ScopeFromConversation(state.input.Conversation)
	bindingMethod := strings.TrimSpace(config.EffectiveBindingMethod())
	if state.input.AIAgent.Source == services.TenantDefaultAIAgentSource && scope.ProductID <= 0 && bindingMethod == "product_context" {
		bindingMethod = "agent_default"
	}
	switch bindingMethod {
	case "agent_default", "product_context":
	case "public_product":
		scope.Audience = "customer"
	case "internal_product":
		if scope.Audience != "internal" && scope.Audience != "employee" {
			return errors.New("internal product knowledge is not available in a customer conversation")
		}
	default:
		return fmt.Errorf("unsupported knowledge binding method: %s", bindingMethod)
	}
	retriever := retrievers.NewKnowledgeRetriever(state.input.AIAgent, scope).
		WithConversationID(state.input.Conversation.ID)
	if bindingMethod == "agent_default" && scope.ProductID <= 0 {
		retriever.WithTenantKnowledgeOnly()
	}
	result, err := retriever.RetrieveContextByOptions(ctx, retrievers.KnowledgeRetrieveOptions{
		TopK:             config.TopK,
		ScoreThreshold:   config.ScoreThreshold,
		ContextMaxTokens: config.ContextMaxTokens,
		MaxContextItems:  config.MaxContextItems,
	}, query)
	if err != nil {
		return err
	}
	items := make([]map[string]any, 0, len(result.ContextResults))
	for _, item := range result.ContextResults {
		items = append(items, map[string]any{
			"knowledgeBaseId": item.KnowledgeBaseID,
			"documentId":      item.DocumentID,
			"chunkId":         item.ChunkID,
			"content":         item.Content,
			"score":           item.Score,
			"denseScore":      item.DenseScore,
			"lexicalScore":    item.LexicalScore,
			"fusionScore":     item.FusionScore,
			"rerankScore":     item.RerankScore,
			"retrievalSource": item.RetrievalSource,
		})
	}
	state.result.RetrieverCount = len(result.Hits)
	state.result.TraceData = mergeWorkflowRetrieverTrace(state.result.TraceData, result)
	state.setNodeVars(node.ID, map[string]any{
		"items":            items,
		"citations":        result.Citations,
		"summary":          result.ContextText,
		"knowledgeBaseIds": result.KnowledgeBaseIDs,
	})
	return nil
}

func knowledgeCitationsFromValue(value any) []dto.KnowledgeCitation {
	if value == nil {
		return nil
	}
	if citations, ok := value.([]dto.KnowledgeCitation); ok {
		return append([]dto.KnowledgeCitation(nil), citations...)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var citations []dto.KnowledgeCitation
	if err := json.Unmarshal(raw, &citations); err != nil {
		return nil
	}
	return citations
}

func mergeWorkflowRetrieverTrace(existing string, result *retrievers.KnowledgeRetrieveResult) string {
	if result == nil {
		return existing
	}
	root := make(map[string]any)
	if strings.TrimSpace(existing) != "" {
		_ = json.Unmarshal([]byte(existing), &root)
	}
	root["retriever"] = map[string]any{
		"count":            len(result.Hits),
		"topK":             result.TraceSummary.TopK,
		"scoreThreshold":   result.TraceSummary.ScoreThreshold,
		"contextMaxTokens": result.TraceSummary.ContextMaxTokens,
		"maxContextItems":  result.TraceSummary.MaxContextItems,
		"contextCount":     len(result.ContextResults),
		"embeddingMs":      result.TraceSummary.EmbeddingMs,
		"vectorSearchMs":   result.TraceSummary.VectorSearchMs,
		"hydrateMs":        result.TraceSummary.HydrateMs,
		"policies":         result.TraceSummary.Policies,
		"items":            result.TraceItems,
	}
	raw, err := json.Marshal(root)
	if err != nil {
		return existing
	}
	return string(raw)
}

func (e *Executor) executeKnowledgeMerge(state *runState, node dsl.Node) error {
	config := dsl.KnowledgeMergeConfig{}
	if err := json.Unmarshal(node.Config, &config); err != nil {
		return fmt.Errorf("invalid knowledge_merge config: %w", err)
	}
	maxItems := config.MaxItems
	if maxItems <= 0 {
		maxItems = 10
	}
	items := make([]map[string]any, 0)
	summaries := make([]string, 0, len(config.Sources))
	seen := make(map[string]struct{})
	for _, selector := range config.Sources {
		value := state.resolveSelector(&selector)
		for _, item := range toWorkflowObjectItems(value) {
			key := fmt.Sprintf("%v:%v:%v", item["knowledgeBaseId"], item["documentId"], item["chunkId"])
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			items = append(items, item)
			if content := strings.TrimSpace(toString(item["content"])); content != "" {
				summaries = append(summaries, content)
			}
			if len(items) >= maxItems {
				break
			}
		}
		if len(items) >= maxItems {
			break
		}
	}
	state.setNodeVars(node.ID, map[string]any{
		"items":       items,
		"summary":     strings.Join(summaries, "\n\n"),
		"sourceCount": len(config.Sources),
	})
	return nil
}

func (e *Executor) executeAnswerabilityGate(state *runState, node dsl.Node) error {
	config := dsl.AnswerabilityGateConfig{}
	if len(node.Config) > 0 {
		if err := json.Unmarshal(node.Config, &config); err != nil {
			return fmt.Errorf("answerability_gate config is invalid: %w", err)
		}
	}
	if config.MinScore <= 0 {
		config.MinScore = 0.35
	}
	if config.StrongScore <= 0 {
		config.StrongScore = 0.72
	}
	if config.MinMatchedTerms <= 0 {
		config.MinMatchedTerms = 1
	}

	query := strings.TrimSpace(toString(state.resolveInput(node, "userMessage")))
	items := toWorkflowObjectItems(state.resolveInput(node, "knowledgeItems"))
	queryTerms := workflowAnswerabilityTerms(query)
	bestScore := 0.0
	matchedTerms := make(map[string]struct{})
	matchedItemCount := 0
	evidenceItemCount := 0
	for _, item := range items {
		content := strings.TrimSpace(toString(item["content"]))
		if content == "" {
			continue
		}
		evidenceItemCount++
		if score := workflowAnswerabilityScore(item); score > bestScore {
			bestScore = score
		}
		itemMatched := false
		for term := range workflowAnswerabilityTerms(content) {
			if _, ok := queryTerms[term]; !ok {
				continue
			}
			matchedTerms[term] = struct{}{}
			itemMatched = true
		}
		if itemMatched {
			matchedItemCount++
		}
	}

	answerability := "unanswerable"
	reason := "no usable retrieved evidence"
	confidence := 0.0
	if evidenceItemCount > 0 && query != "" {
		switch {
		case bestScore >= config.StrongScore:
			answerability = "answerable"
			reason = "retrieval score provides strong semantic evidence"
			confidence = bestScore
		case bestScore >= config.MinScore && len(matchedTerms) >= config.MinMatchedTerms:
			answerability = "answerable"
			reason = "retrieved evidence meets score and query-match thresholds"
			confidence = bestScore
		case bestScore == 0 && len(matchedTerms) >= max(2, config.MinMatchedTerms):
			answerability = "answerable"
			reason = "unscored evidence contains sufficient query matches"
			confidence = 0.5
		default:
			reason = "retrieved evidence is not relevant enough to support an answer"
			confidence = bestScore
		}
	}
	state.setNodeVars(node.ID, map[string]any{
		"answerability":     answerability,
		"reason":            reason,
		"confidence":        confidence,
		"bestScore":         bestScore,
		"matchedTermCount":  len(matchedTerms),
		"matchedItemCount":  matchedItemCount,
		"evidenceItemCount": evidenceItemCount,
	})
	return nil
}

func workflowAnswerabilityScore(item map[string]any) float64 {
	best := toFloat(item["score"])
	// Hybrid RRF scores are rank-fusion values (usually around 0.01-0.03),
	// not semantic confidence. Preserve the original normalized signals so the
	// answerability gate remains comparable across dense, hybrid and reranked modes.
	for _, field := range []string{"denseScore", "rerankScore"} {
		if score := toFloat(item[field]); score > best {
			best = score
		}
	}
	return best
}

var workflowAnswerabilityStopTerms = map[string]struct{}{
	"and": {}, "are": {}, "can": {}, "for": {}, "how": {}, "the": {}, "this": {}, "what": {}, "with": {},
	"一下": {}, "什么": {}, "你们": {}, "可以": {}, "如何": {}, "怎么": {}, "是否": {}, "步骤": {},
	"这个": {}, "那个": {}, "问题": {}, "处理": {}, "设备": {},
}

func workflowAnswerabilityTerms(value string) map[string]struct{} {
	terms := make(map[string]struct{})
	var latin []rune
	var han []rune
	flushLatin := func() {
		if len(latin) >= 2 {
			term := strings.ToLower(string(latin))
			if _, stopped := workflowAnswerabilityStopTerms[term]; !stopped {
				terms[term] = struct{}{}
			}
		}
		latin = latin[:0]
	}
	flushHan := func() {
		if len(han) == 1 {
			term := string(han)
			if _, stopped := workflowAnswerabilityStopTerms[term]; !stopped {
				terms[term] = struct{}{}
			}
		}
		for index := 0; index+1 < len(han); index++ {
			term := string(han[index : index+2])
			if _, stopped := workflowAnswerabilityStopTerms[term]; !stopped {
				terms[term] = struct{}{}
			}
		}
		han = han[:0]
	}
	for _, r := range strings.TrimSpace(value) {
		switch {
		case unicode.Is(unicode.Han, r):
			flushLatin()
			han = append(han, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_':
			flushHan()
			latin = append(latin, unicode.ToLower(r))
		default:
			flushLatin()
			flushHan()
		}
	}
	flushLatin()
	flushHan()
	return terms
}

func (e *Executor) executeLLMReply(ctx context.Context, state *runState, node dsl.Node) error {
	capabilities := workflowcapability.FromDefinition(state.input.Definition)
	if staticReply := strings.TrimSpace(readStringConfig(node.Config, "staticReply")); staticReply != "" {
		state.setNodeVars(node.ID, map[string]any{"replyText": workflowStaticReplyForMessage(staticReply, state.input.UserMessage.Content)})
		return nil
	}
	userPrompt := strings.TrimSpace(toString(state.resolveInput(node, "userMessage")))
	if userPrompt == "" {
		userPrompt = strings.TrimSpace(state.input.UserMessage.Content)
	}
	knowledgeItems := toString(state.resolveInput(node, "knowledgeItems"))
	systemPrompt := strings.TrimSpace(state.input.AIAgent.SystemPrompt)
	if prompt := strings.TrimSpace(readStringConfig(node.Config, "prompt")); prompt != "" {
		systemPrompt = strings.TrimSpace(systemPrompt + "\n\n" + prompt)
	}
	if boundary := capabilities.RuntimeInstruction(); boundary != "" {
		systemPrompt = strings.TrimSpace(systemPrompt + "\n\n" + boundary)
	}
	if languageInstruction := workflowReplyLanguageInstruction(state.input.UserMessage.Content); languageInstruction != "" {
		systemPrompt = strings.TrimSpace(systemPrompt + "\n\n" + languageInstruction)
	}
	allowEmptyKnowledge := readBoolConfig(node.Config, "allowEmptyKnowledge")
	if _, declaresKnowledge := node.Inputs["knowledgeItems"]; declaresKnowledge && !allowEmptyKnowledge && (state.input.AIAgent.ProductID > 0 || len(utils.SplitInt64s(state.input.AIAgent.KnowledgeIDs)) > 0) && !hasItems(state.resolveInput(node, "knowledgeItems")) {
		state.setNodeVars(node.ID, map[string]any{"replyText": workflowKnowledgeFallbackReplyForMessage(state.input.AIAgent, capabilities, state.input.UserMessage.Content)})
		return nil
	}
	if state.input.AgentRuntime == nil {
		return fmt.Errorf("workflow llm_reply requires an Eino Agent runtime")
	}
	agentResult, err := state.input.AgentRuntime.ExecuteWorkflowNode(ctx, runtimeexecutor.WorkflowNodeInput{
		Conversation:     state.input.Conversation,
		UserMessage:      state.input.UserMessage,
		AIAgent:          state.input.AIAgent,
		AIConfig:         state.input.AIConfig,
		ToolSet:          state.input.ToolSet,
		SystemPrompt:     systemPrompt,
		UserPrompt:       userPrompt,
		KnowledgeContext: knowledgeItems,
		BlockedToolCodes: workflowAgentBlockedToolCodes(state.input.Definition),
	})
	if agentResult != nil {
		state.mergeAgentResult(node.ID, agentResult)
	}
	if err != nil {
		return err
	}
	if agentResult == nil {
		return fmt.Errorf("workflow Eino Agent returned no result")
	}
	if agentResult.Interrupted {
		return fmt.Errorf("Eino Agent interruption is not supported inside workflow; use a human_confirm node")
	}
	replyText := sanitizeUnverifiedActionClaims(agentResult.ReplyText, capabilities)
	if replyText == "" {
		replyText = workflowKnowledgeFallbackReplyForMessage(state.input.AIAgent, capabilities, state.input.UserMessage.Content)
	}
	state.setNodeVars(node.ID, map[string]any{
		"replyText":             replyText,
		"agentStatus":           agentResult.Status,
		"selectedSkillId":       agentResult.SelectedSkillID,
		"selectedSkillName":     agentResult.SelectedSkillName,
		"skillRouteReason":      agentResult.SkillRouteReason,
		"toolCodes":             append([]string(nil), agentResult.ToolCodes...),
		"invokedToolCodes":      append([]string(nil), agentResult.InvokedToolCodes...),
		"promptTokens":          agentResult.PromptTokens,
		"completionTokens":      agentResult.CompletionTokens,
		"historyMessageCount":   agentResult.HistoryMessageCount,
		"agentRuntimeTraceData": agentResult.TraceData,
	})
	return nil
}

func sanitizeUnverifiedActionClaims(value string, capabilities workflowcapability.Set) string {
	reply := strings.TrimSpace(value)
	if reply == "" {
		return ""
	}
	handoffReplacement := "建议转人工工程师继续处理"
	handoffShortReplacement := "如需转人工，请通过页面确认"
	if !capabilities.HumanHandoff {
		handoffReplacement = "当前产品不提供转人工，我会继续根据产品知识协助诊断"
		handoffShortReplacement = handoffReplacement
	}
	ticketReplacement := "如需创建工单，请确认后由系统执行"
	if !capabilities.TicketCreation {
		ticketReplacement = "当前流程不创建工单，我会继续根据产品知识协助诊断"
	}
	videoReplacement := "如需发起视频，请由工程师确认后执行"
	if !capabilities.VideoMeeting {
		videoReplacement = "当前流程不发起视频会议"
	}
	replacer := strings.NewReplacer(
		"我已为你转人工工程师处理", handoffReplacement,
		"我已经为你转人工工程师处理", handoffReplacement,
		"已为你转人工工程师处理", handoffReplacement,
		"我已为你转人工", handoffShortReplacement,
		"我已经为你转人工", handoffShortReplacement,
		"已为你转人工", handoffShortReplacement,
		"我已为你创建工单", ticketReplacement,
		"我已经为你创建工单", ticketReplacement,
		"已为你创建工单", ticketReplacement,
		"我已为你发起视频", videoReplacement,
		"我已经为你发起视频", videoReplacement,
		"已为你发起视频", videoReplacement,
		"我已邀请供应商", "如需邀请供应商，请由工程师确认后执行",
		"我已经邀请供应商", "如需邀请供应商，请由工程师确认后执行",
		"已邀请供应商", "如需邀请供应商，请由工程师确认后执行",
	)
	return strings.TrimSpace(replacer.Replace(reply))
}

func workflowAgentBlockedToolCodes(definition dsl.Definition) []string {
	// Side effects stay under Workflow control even when an old Agent config or
	// the runtime registry would otherwise expose these tools directly.
	ret := []string{
		toolx.GraphCreateTicketConfirm.Code,
		toolx.GraphHandoffConversation.Code,
		toolx.GraphCreateVideoMeeting.Code,
		toolx.GraphCreateKnowledgeCandidate.Code,
	}
	for _, node := range definition.Nodes {
		var toolCode string
		switch strings.TrimSpace(node.Type) {
		case workflowregistry.NodeTypeAnalyzeConversation:
			toolCode = toolx.GraphAnalyzeConversation.Code
		case workflowregistry.NodeTypePrepareTicketDraft:
			toolCode = toolx.GraphPrepareTicketDraft.Code
		case workflowregistry.NodeTypeCreateTicket:
			toolCode = toolx.GraphCreateTicketConfirm.Code
		case workflowregistry.NodeTypeHandoffToHuman:
			toolCode = toolx.GraphHandoffConversation.Code
		case workflowregistry.NodeTypeCreateVideoMeeting:
			toolCode = toolx.GraphCreateVideoMeeting.Code
		case workflowregistry.NodeTypeCreateKnowledgeCandidate:
			toolCode = toolx.GraphCreateKnowledgeCandidate.Code
		}
		ret = appendWorkflowUnique(ret, toolCode)
	}
	return ret
}

func (s *runState) mergeAgentResult(nodeID string, result *runtimeexecutor.RunResult) {
	if s == nil || result == nil {
		return
	}
	s.result.SelectedSkillID = result.SelectedSkillID
	s.result.SelectedSkillName = result.SelectedSkillName
	s.result.SkillRouteReason = result.SkillRouteReason
	s.result.SkillRouteTrace = result.SkillRouteTrace
	s.result.SkillAllowedToolCodes = append([]string(nil), result.SkillAllowedToolCodes...)
	if strings.TrimSpace(result.ModelProvider) != "" {
		s.result.ModelProvider = strings.TrimSpace(result.ModelProvider)
	}
	if strings.TrimSpace(result.ModelName) != "" {
		s.result.ModelName = strings.TrimSpace(result.ModelName)
	}
	s.result.PromptTokens += result.PromptTokens
	s.result.CompletionTokens += result.CompletionTokens
	s.result.HistoryMessageCount = result.HistoryMessageCount
	s.result.RetrieverCount += result.RetrieverCount
	s.result.ToolCallCount += result.ToolCallCount
	for _, toolCode := range result.ToolCodes {
		s.result.ToolCodes = appendWorkflowUnique(s.result.ToolCodes, toolCode)
	}
	for _, toolCode := range result.InvokedToolCodes {
		s.result.InvokedToolCodes = appendWorkflowUnique(s.result.InvokedToolCodes, toolCode)
	}
	s.result.TraceData = mergeAgentNodeTrace(s.result.TraceData, nodeID, result.TraceData)
}

func mergeAgentNodeTrace(existing string, nodeID string, traceData string) string {
	root := make(map[string]any)
	if strings.TrimSpace(existing) != "" {
		_ = json.Unmarshal([]byte(existing), &root)
	}
	agentNodes, _ := root["agentNodes"].(map[string]any)
	if agentNodes == nil {
		agentNodes = make(map[string]any)
	}
	var trace any = strings.TrimSpace(traceData)
	if strings.TrimSpace(traceData) != "" {
		_ = json.Unmarshal([]byte(traceData), &trace)
	}
	agentNodes[strings.TrimSpace(nodeID)] = trace
	root["agentNodes"] = agentNodes
	raw, err := json.Marshal(root)
	if err != nil {
		return existing
	}
	return string(raw)
}

func appendWorkflowUnique(items []string, item string) []string {
	item = strings.TrimSpace(item)
	if item == "" {
		return items
	}
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

func workflowKnowledgeFallbackReply(aiAgent models.AIAgent, capabilities workflowcapability.Set) string {
	if reply := capabilities.SanitizeFallbackMessage(aiAgent.FallbackMessage); reply != "" {
		return reply
	}
	if aiAgent.FallbackMode == enums.AIAgentFallbackModeSuggestRetry {
		return "当前知识库里没有找到足够明确的信息，你可以换个更具体的问法再试一次。"
	}
	return "当前知识库暂无明确信息。"
}

func workflowKnowledgeFallbackReplyForMessage(aiAgent models.AIAgent, capabilities workflowcapability.Set, userMessage string) string {
	if !workflowMessageLooksEnglish(userMessage) {
		return workflowKnowledgeFallbackReply(aiAgent, capabilities)
	}
	if capabilities.HumanHandoff {
		return "I do not have enough verified information to confirm a reliable answer. Please provide more details, or use the human support option in this conversation."
	}
	return "I do not have enough verified information to confirm a reliable answer. Please provide more details so I can continue checking."
}

func workflowStaticReplyForMessage(staticReply, userMessage string) string {
	if !workflowMessageLooksEnglish(userMessage) {
		return staticReply
	}
	englishReplies := map[string]string{
		"已取消创建工单。你可以继续补充问题，我会继续帮你处理。":                                            "Ticket creation has been cancelled. You can provide more details and I will continue helping.",
		"我已整理工单草稿。请回复“确认”创建工单，或回复“取消”放弃。":                                        "I have prepared the ticket draft. Reply 'confirm' to create it, or 'cancel' to discard it.",
		"暂时无法读取当前可用知识或生成可靠回答。请补充产品名称、设备型号、故障现象和故障码，或稍后再试；当前不会创建工单或转接人工。":         "I cannot access enough verified knowledge to provide a reliable answer right now. Please provide the product name, device model, symptoms, and fault code, or try again later. This workflow will not create a ticket or transfer the conversation to a human.",
		"我暂时无法完成这次回答。你可以换一种方式描述问题，也可以直接请求人工支持或创建工单；如果问题与设备有关，再补充产品、型号或故障现象即可。":   "I cannot complete this answer right now. Please describe the issue another way, request human support, or create a ticket. For a device issue, include the product, model, and symptoms.",
		"本次暂时无法完成转人工或创建工单。请核对服务码与设备信息后重试；你也可以继续描述问题，我会继续提供 AI 协助。":               "Human handoff or ticket creation could not be completed this time. Verify the service code and device information, then try again. You can also continue describing the issue for AI assistance.",
		"现有产品知识不足以给出可靠的设备诊断。请补充产品型号、故障码、现场现象和已尝试步骤；当前流程仅提供 AI 服务，不会转接人工或创建工单。":   "The available product knowledge is not sufficient for a reliable device diagnosis. Please provide the product model, fault code, observed symptoms, and troubleshooting already attempted. This workflow provides AI support only and will not transfer to a human or create a ticket.",
		"AI 暂时无法可靠完成本次诊断。请保持设备停机和安全隔离，并点击会话中的“转人工”按钮，由技术工程师继续处理；系统不会自动转人工或创建工单。": "AI could not complete a reliable diagnosis this time. Keep the device powered off and safely isolated, then use the 'Human support' button in this conversation so a technical engineer can continue. The system has not automatically transferred the conversation or created a ticket.",
	}
	if translated, ok := englishReplies[strings.TrimSpace(staticReply)]; ok {
		return translated
	}
	return staticReply
}

func workflowReplyLanguageInstruction(userMessage string) string {
	value := normalizeWorkflowUserMessage(userMessage)
	if value == "" {
		return "Response language: use the same primary language as the customer's original latest message. Ignore the language used by reference knowledge."
	}
	if workflowMessageLooksEnglish(value) {
		return "Response language for this turn: English. Reply entirely in English because the customer's original latest message is in English. Do not switch to Chinese even when the system prompt, history, workflow instructions, or reference knowledge are in Chinese."
	}
	hanCount := 0
	latinCount := 0
	for _, character := range value {
		switch {
		case unicode.Is(unicode.Han, character):
			hanCount++
		case unicode.Is(unicode.Latin, character):
			latinCount++
		}
	}
	if hanCount > 0 && hanCount*2 >= latinCount {
		return "本轮回复语言：简体中文。以客户原始最新消息为准，不要受其他语言的知识片段影响。"
	}
	return "Response language: identify the primary language of the customer's original latest message and reply entirely in that same language. If it is English, reply entirely in English. Ignore the language used by reference knowledge."
}

func workflowMessageLooksEnglish(value string) bool {
	value = strings.ToLower(normalizeWorkflowUserMessage(value))
	if value == "" {
		return false
	}
	latinCount := 0
	for _, character := range value {
		if unicode.Is(unicode.Han, character) {
			return false
		}
		if unicode.Is(unicode.Latin, character) {
			latinCount++
		}
	}
	if latinCount < 2 {
		return false
	}
	englishMarkers := map[string]struct{}{
		"a": {}, "an": {}, "and": {}, "are": {}, "can": {}, "could": {}, "device": {}, "equipment": {},
		"hello": {}, "help": {}, "how": {}, "i": {}, "is": {}, "it": {}, "machine": {}, "my": {},
		"please": {}, "reset": {}, "restart": {}, "safe": {}, "should": {}, "support": {}, "the": {},
		"this": {}, "to": {}, "warranty": {}, "what": {}, "when": {}, "where": {}, "why": {}, "you": {},
	}
	for _, word := range strings.FieldsFunc(value, func(character rune) bool {
		return !unicode.IsLetter(character)
	}) {
		if _, ok := englishMarkers[word]; ok {
			return true
		}
	}
	return false
}

func (s *runState) nextNodeID(sourceNodeID string) (string, bool, error) {
	edges := s.outgoing[sourceNodeID]
	if len(edges) == 0 {
		return "", false, nil
	}
	node := s.nodesByID[sourceNodeID]
	if strings.TrimSpace(node.Type) != workflowregistry.NodeTypeCondition {
		errorTargetNodeID := strings.TrimSpace(node.ErrorTargetNodeID)
		for _, edge := range edges {
			targetNodeID := strings.TrimSpace(edge.Target)
			if targetNodeID != "" && targetNodeID != errorTargetNodeID {
				return targetNodeID, true, nil
			}
		}
		return "", false, nil
	}
	config := dsl.ConditionConfig{}
	if len(node.Config) > 0 {
		if err := json.Unmarshal(node.Config, &config); err != nil {
			return "", false, fmt.Errorf("invalid condition node config: %w", err)
		}
	}
	if s.input.DryRun && len(s.input.WorkflowNodeStack) == 0 {
		branchID := strings.TrimSpace(s.input.BranchOverrides[sourceNodeID])
		if branchID != "" {
			for _, branch := range config.Branches {
				if strings.TrimSpace(branch.ID) != branchID {
					continue
				}
				targetNodeID := strings.TrimSpace(branch.TargetNodeID)
				s.branchDecisions[sourceNodeID] = branchDecision{
					SelectedEdgeID:       s.edgeIDForTarget(sourceNodeID, targetNodeID),
					SelectedBranchID:     branchID,
					SelectedBranchName:   strings.TrimSpace(branch.Name),
					SelectedTargetNodeID: targetNodeID,
					Reason:               "test run branch override",
				}
				return targetNodeID, true, nil
			}
			return "", false, fmt.Errorf("test branch override does not exist: %s.%s", sourceNodeID, branchID)
		}
	}
	evaluations := make([]conditionEvaluation, 0)
	for _, branch := range config.Branches {
		if branch.Default {
			continue
		}
		matched, evaluation, err := s.evaluateConditionBranch(sourceNodeID, branch)
		if err != nil {
			return "", false, err
		}
		evaluations = append(evaluations, evaluation)
		if matched {
			targetNodeID := strings.TrimSpace(branch.TargetNodeID)
			s.branchDecisions[sourceNodeID] = branchDecision{
				SelectedEdgeID:       s.edgeIDForTarget(sourceNodeID, targetNodeID),
				SelectedBranchID:     strings.TrimSpace(branch.ID),
				SelectedBranchName:   strings.TrimSpace(branch.Name),
				SelectedTargetNodeID: targetNodeID,
				Reason:               "condition branch matched",
				Evaluations:          evaluations,
			}
			return targetNodeID, true, nil
		}
	}
	for _, branch := range config.Branches {
		if !branch.Default {
			continue
		}
		targetNodeID := strings.TrimSpace(branch.TargetNodeID)
		s.branchDecisions[sourceNodeID] = branchDecision{
			SelectedEdgeID:       s.edgeIDForTarget(sourceNodeID, targetNodeID),
			SelectedBranchID:     strings.TrimSpace(branch.ID),
			SelectedBranchName:   strings.TrimSpace(branch.Name),
			SelectedTargetNodeID: targetNodeID,
			Reason:               "no condition branch matched; selected default branch",
			Evaluations:          evaluations,
		}
		return targetNodeID, true, nil
	}
	s.branchDecisions[sourceNodeID] = branchDecision{
		Reason:      "no condition branch matched and no default branch exists",
		Evaluations: evaluations,
	}
	return "", false, nil
}

func (s *runState) evaluateConditionBranch(sourceNodeID string, branch dsl.ConditionBranch) (bool, conditionEvaluation, error) {
	condition := branch.Condition
	targetNodeID := strings.TrimSpace(branch.TargetNodeID)
	evaluation := conditionEvaluation{
		EdgeID:       s.edgeIDForTarget(sourceNodeID, targetNodeID),
		BranchID:     strings.TrimSpace(branch.ID),
		BranchName:   strings.TrimSpace(branch.Name),
		TargetNodeID: targetNodeID,
	}
	if condition == nil {
		evaluation.Matched = true
		return true, evaluation, nil
	}
	left := s.resolveSelector(condition.Left)
	operator := strings.TrimSpace(condition.Operator)
	if condition.Left != nil {
		evaluation.SourceNodeID = strings.TrimSpace(condition.Left.NodeID)
		evaluation.SourceField = strings.TrimSpace(condition.Left.Field)
	}
	evaluation.Operator = operator
	evaluation.LeftValue = left
	evaluation.RightValue = condition.Right
	if operator == "" && strings.TrimSpace(condition.Expression) != "" {
		return false, evaluation, fmt.Errorf("free-form workflow condition expressions are not supported")
	}
	var matched bool
	switch operator {
	case "eq", "equals":
		matched = compareString(left, condition.Right) == 0
	case "neq", "not_equals":
		matched = compareString(left, condition.Right) != 0
	case "contains":
		matched = strings.Contains(toString(left), toString(condition.Right))
	case "exists":
		matched = exists(left)
	case "not_exists":
		matched = !exists(left)
	case "truthy", "is_true":
		matched = truthy(left)
	case "falsy", "is_false":
		matched = !truthy(left)
	case "gt":
		matched = compareNumber(left, condition.Right) > 0
	case "gte":
		matched = compareNumber(left, condition.Right) >= 0
	case "lt":
		matched = compareNumber(left, condition.Right) < 0
	case "lte":
		matched = compareNumber(left, condition.Right) <= 0
	default:
		return false, evaluation, fmt.Errorf("unsupported workflow condition operator: %s", operator)
	}
	evaluation.Matched = matched
	return matched, evaluation, nil
}

func (s *runState) edgeIDForTarget(sourceNodeID string, targetNodeID string) string {
	for _, edge := range s.outgoing[sourceNodeID] {
		if strings.TrimSpace(edge.Target) == targetNodeID {
			return strings.TrimSpace(edge.ID)
		}
	}
	return ""
}

func (s *runState) setNodeVars(nodeID string, values map[string]any) {
	s.vars[nodeID] = values
}

func (s *runState) resolveInput(node dsl.Node, inputName string) any {
	selector, ok := node.Inputs[inputName]
	if !ok {
		return nil
	}
	return s.resolveSelector(&selector)
}

func (s *runState) nodeInputPreview(node dsl.Node) map[string]any {
	inputs := make(map[string]any, len(node.Inputs))
	for name, selector := range node.Inputs {
		inputs[name] = s.resolveSelector(&selector)
	}
	ret := map[string]any{
		"inputs": inputs,
	}
	if len(node.Config) > 0 {
		var cfg any
		if err := json.Unmarshal(node.Config, &cfg); err == nil {
			ret["config"] = cfg
		} else {
			ret["config"] = string(node.Config)
		}
	}
	return ret
}

func (s *runState) nodeOutputPreview(nodeID string) map[string]any {
	ret := map[string]any{
		"outputs": s.vars[nodeID],
	}
	if decision, ok := s.branchDecisions[nodeID]; ok {
		ret["branchDecision"] = decision
	}
	return ret
}

func workflowPreviewJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	const maxPreviewBytes = 2000
	if len(raw) <= maxPreviewBytes {
		return string(raw)
	}
	limit := maxPreviewBytes
	for limit > 0 && !utf8.Valid(raw[:limit]) {
		limit--
	}
	return string(raw[:limit])
}

func (s *runState) resolveSelector(selector *dsl.VariableSelector) any {
	if selector == nil {
		return nil
	}
	fields := s.vars[strings.TrimSpace(selector.NodeID)]
	if fields == nil {
		return nil
	}
	return fields[strings.TrimSpace(selector.Field)]
}

func readStringConfig(raw json.RawMessage, key string) string {
	if len(raw) == 0 {
		return ""
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return ""
	}
	return toString(cfg[key])
}

func readBoolConfig(raw json.RawMessage, key string) bool {
	if len(raw) == 0 {
		return false
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return false
	}
	return truthy(cfg[key])
}

func readInt64SliceConfig(raw json.RawMessage, key string) ([]int64, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("invalid workflow node config: %w", err)
	}
	value, ok := cfg[key]
	if !ok || string(value) == "null" {
		return nil, nil
	}
	var ret []int64
	if err := json.Unmarshal(value, &ret); err != nil {
		return nil, fmt.Errorf("workflow node config %s must be an integer array", key)
	}
	return ret, nil
}

func containsInt64(items []int64, expected int64) bool {
	for _, item := range items {
		if item == expected {
			return true
		}
	}
	return false
}

func toWorkflowObjectItems(value any) []map[string]any {
	switch items := value.(type) {
	case []map[string]any:
		return items
	case []any:
		ret := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if mapped, ok := item.(map[string]any); ok {
				ret = append(ret, mapped)
			}
		}
		return ret
	case string:
		var ret []map[string]any
		_ = json.Unmarshal([]byte(strings.TrimSpace(items)), &ret)
		return ret
	default:
		return nil
	}
}

func compareString(left any, right any) int {
	return strings.Compare(toString(left), toString(right))
}

func compareNumber(left any, right any) int {
	leftNum := toFloat(left)
	rightNum := toFloat(right)
	switch {
	case leftNum > rightNum:
		return 1
	case leftNum < rightNum:
		return -1
	default:
		return 0
	}
}

func toString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case []map[string]any:
		buf, _ := json.Marshal(v)
		return string(buf)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func toFloat(value any) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	case float32:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f
	default:
		return 0
	}
}

func toInt64(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	case uint:
		return int64(v)
	case uint64:
		if v > uint64(^uint64(0)>>1) {
			return 0
		}
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case json.Number:
		i, _ := v.Int64()
		return i
	case string:
		i, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return i
	default:
		return int64(toFloat(value))
	}
}

func asMap(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		return v
	case map[string]string:
		ret := make(map[string]any, len(v))
		for key, item := range v {
			ret[key] = item
		}
		return ret
	case string:
		var ret map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(v)), &ret); err == nil {
			return ret
		}
	}
	return map[string]any{}
}

func truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		normalized := strings.ToLower(strings.TrimSpace(v))
		return normalized != "" && normalized != "false" && normalized != "0"
	default:
		return !reflect.ValueOf(value).IsZero()
	}
}

func exists(value any) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	default:
		return true
	}
}

func hasItems(value any) bool {
	if value == nil {
		return false
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map:
		return rv.Len() > 0
	default:
		return exists(value)
	}
}

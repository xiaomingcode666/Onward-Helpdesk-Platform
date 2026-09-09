package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/compose"

	"remotehelpdesk/internal/ai/runtime/graphs"
	"remotehelpdesk/internal/ai/workflow/dsl"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
	"remotehelpdesk/internal/pkg/utils"
)

const RuntimeEngineEinoGraph = "eino_graph"

// Engine is the stable application-facing workflow runtime contract. The DSL,
// published versions and business checkpoint format stay outside the engine.
type Engine interface {
	Execute(context.Context, Input) (*Result, error)
	Resume(context.Context, Input, string, string) (*Result, error)
}

func NewEngine() Engine {
	return NewGraphExecutor()
}

// GraphExecutor compiles a published Workflow DSL definition into an Eino
// Graph. Node behavior remains centralized in Executor while Eino owns graph
// scheduling, branching and fan-out/fan-in.
type GraphExecutor struct {
	nodes *Executor
}

func NewGraphExecutor() *GraphExecutor {
	registerGraphTokenMerge.Do(func() {
		compose.RegisterValuesMergeFunc[*graphToken](mergeGraphTokens)
	})
	return &GraphExecutor{nodes: NewExecutor()}
}

type graphExecution struct {
	mu    sync.Mutex
	state *runState
}

type graphToken struct {
	execution *graphExecution
}

var registerGraphTokenMerge sync.Once

func mergeGraphTokens(tokens []*graphToken) (*graphToken, error) {
	var selected *graphToken
	for _, token := range tokens {
		if token == nil || token.execution == nil {
			continue
		}
		if selected == nil {
			selected = token
			continue
		}
		if selected.execution != token.execution {
			return nil, fmt.Errorf("workflow graph received values from different executions")
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("workflow graph received no execution state")
	}
	return selected, nil
}

func (e *GraphExecutor) Execute(ctx context.Context, input Input) (*Result, error) {
	entryNodeID := strings.TrimSpace(input.Definition.EntryNodeID)
	if entryNodeID == "" {
		return nil, fmt.Errorf("workflow entry node is required")
	}
	return e.executeFrom(ctx, newRunState(input), entryNodeID)
}

func (e *GraphExecutor) Resume(ctx context.Context, input Input, checkPointData string, resumeText string) (*Result, error) {
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
		if err := e.nodes.executeHumanConfirm(state, node); err != nil {
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

func (e *GraphExecutor) executeFrom(ctx context.Context, state *runState, startNodeID string) (*Result, error) {
	execution := &graphExecution{state: state}
	runner, err := e.compile(ctx, execution, startNodeID)
	if err != nil {
		state.result.Status = "error"
		return &state.result, err
	}
	_, invokeErr := runner.Invoke(ctx, &graphToken{execution: execution})
	execution.mu.Lock()
	result := execution.state.result
	execution.mu.Unlock()
	if result.Interrupted {
		result.Status = "interrupted"
		return &result, nil
	}
	if invokeErr != nil {
		result.Status = "error"
		return &result, invokeErr
	}
	result.Status = "completed"
	return &result, nil
}

func (e *GraphExecutor) compile(ctx context.Context, execution *graphExecution, startNodeID string) (compose.Runnable[*graphToken, *graphToken], error) {
	reachable, err := graphReachableNodes(execution.state, startNodeID)
	if err != nil {
		return nil, err
	}
	graph := compose.NewGraph[*graphToken, *graphToken]()
	for nodeID := range reachable {
		node := execution.state.nodesByID[nodeID]
		current := node
		if err := graph.AddLambdaNode(nodeID, compose.InvokableLambda(func(ctx context.Context, token *graphToken) (*graphToken, error) {
			if token == nil || token.execution == nil {
				return nil, fmt.Errorf("workflow graph execution state is missing")
			}
			if err := e.executeNode(ctx, token.execution, current); err != nil {
				return token, err
			}
			return token, nil
		})); err != nil {
			return nil, err
		}
	}
	if err := graph.AddEdge(compose.START, startNodeID); err != nil {
		return nil, err
	}
	for nodeID := range reachable {
		node := execution.state.nodesByID[nodeID]
		if node.Type == workflowregistry.NodeTypeCondition {
			branch, err := e.buildBranch(execution, node, reachable)
			if err != nil {
				return nil, err
			}
			if err := graph.AddBranch(node.ID, branch); err != nil {
				return nil, err
			}
			continue
		}
		if strings.TrimSpace(node.ErrorTargetNodeID) != "" {
			branch, err := e.buildErrorTransitionBranch(execution, node, reachable)
			if err != nil {
				return nil, err
			}
			if err := graph.AddBranch(node.ID, branch); err != nil {
				return nil, err
			}
			continue
		}
		outgoingCount := 0
		for _, edge := range execution.state.outgoing[nodeID] {
			target := strings.TrimSpace(edge.Target)
			if !reachable[target] {
				continue
			}
			if err := graph.AddEdge(nodeID, target); err != nil {
				return nil, err
			}
			outgoingCount++
		}
		if outgoingCount == 0 {
			if err := graph.AddEdge(nodeID, compose.END); err != nil {
				return nil, err
			}
		}
	}
	return graph.Compile(ctx,
		compose.WithNodeTriggerMode(compose.AllPredecessor),
		compose.WithGraphName(RuntimeEngineEinoGraph),
	)
}

func (e *GraphExecutor) buildErrorTransitionBranch(
	execution *graphExecution,
	node dsl.Node,
	reachable map[string]bool,
) (*compose.GraphBranch, error) {
	errorTargetNodeID := strings.TrimSpace(node.ErrorTargetNodeID)
	if !reachable[errorTargetNodeID] {
		return nil, fmt.Errorf("node %s error target is unreachable: %s", node.ID, errorTargetNodeID)
	}
	successTargetNodeID := ""
	for _, edge := range execution.state.outgoing[node.ID] {
		targetNodeID := strings.TrimSpace(edge.Target)
		if targetNodeID == "" || targetNodeID == errorTargetNodeID || !reachable[targetNodeID] {
			continue
		}
		if successTargetNodeID != "" && successTargetNodeID != targetNodeID {
			return nil, fmt.Errorf("node %s with an error target may have only one normal target", node.ID)
		}
		successTargetNodeID = targetNodeID
	}
	endNodes := map[string]bool{
		compose.END:       true,
		errorTargetNodeID: true,
	}
	if successTargetNodeID != "" {
		endNodes[successTargetNodeID] = true
	}
	return compose.NewGraphBranch(func(_ context.Context, _ *graphToken) (string, error) {
		execution.mu.Lock()
		defer execution.mu.Unlock()
		if recovered, _ := execution.state.vars[node.ID]["errorRecovered"].(bool); recovered {
			return errorTargetNodeID, nil
		}
		if successTargetNodeID == "" {
			return compose.END, nil
		}
		return successTargetNodeID, nil
	}, endNodes), nil
}

func (e *GraphExecutor) buildBranch(execution *graphExecution, node dsl.Node, reachable map[string]bool) (*compose.GraphBranch, error) {
	config := dsl.ConditionConfig{}
	if len(node.Config) > 0 {
		if err := json.Unmarshal(node.Config, &config); err != nil {
			return nil, fmt.Errorf("invalid condition node config: %w", err)
		}
	}
	endNodes := map[string]bool{compose.END: true}
	for _, branch := range config.Branches {
		target := strings.TrimSpace(branch.TargetNodeID)
		if reachable[target] {
			endNodes[target] = true
		}
	}
	return compose.NewGraphBranch(func(_ context.Context, _ *graphToken) (string, error) {
		execution.mu.Lock()
		defer execution.mu.Unlock()
		target, ok, err := execution.state.nextNodeID(node.ID)
		if err != nil {
			return "", err
		}
		if !ok {
			updateGraphConditionTrace(execution.state, node.ID)
			return compose.END, nil
		}
		if !reachable[target] {
			return "", fmt.Errorf("condition node %s selected unreachable target %s", node.ID, target)
		}
		updateGraphConditionTrace(execution.state, node.ID)
		return target, nil
	}, endNodes), nil
}

func updateGraphConditionTrace(state *runState, nodeID string) {
	output := workflowPreviewJSON(state.nodeOutputPreview(nodeID))
	for index := len(state.result.NodeTraces) - 1; index >= 0; index-- {
		if state.result.NodeTraces[index].NodeID == nodeID {
			state.result.NodeTraces[index].OutputPreview = output
			return
		}
	}
}

func (e *GraphExecutor) executeNode(ctx context.Context, execution *graphExecution, node dsl.Node) error {
	execution.mu.Lock()
	if execution.state.result.Interrupted {
		execution.mu.Unlock()
		return errors.New("workflow is already interrupted")
	}
	local := cloneRunStateForNode(execution.state)
	inputPreview := workflowPreviewJSON(local.nodeInputPreview(node))
	execution.mu.Unlock()

	effectRequest, hasEffect := buildWorkflowEffectRequest(local.input, node, inputPreview)
	effectLease := WorkflowEffectLease{Attempt: 1, Acquired: true}
	if hasEffect && local.input.EffectStore != nil {
		var acquireErr error
		effectLease, acquireErr = local.input.EffectStore.Acquire(ctx, effectRequest)
		if acquireErr != nil {
			return acquireErr
		}
		if effectLease.Succeeded {
			var output map[string]any
			if err := json.Unmarshal([]byte(effectLease.ResultData), &output); err != nil {
				return fmt.Errorf("invalid completed workflow effect result: %w", err)
			}
			local.setNodeVars(node.ID, output)
		}
		if !effectLease.Acquired && !effectLease.Succeeded {
			return fmt.Errorf("workflow effect is already running: %s", effectRequest.IdempotencyKey)
		}
	}
	attempt := effectLease.Attempt
	if attempt <= 0 {
		attempt = 1
	}
	idempotencyKey := buildGraphNodeIdempotencyKey(local.input, node.ID, attempt)
	if hasEffect {
		idempotencyKey = effectRequest.IdempotencyKey
	}
	trace := NodeTrace{
		NodeID:         node.ID,
		NodeType:       node.Type,
		Attempt:        attempt,
		IdempotencyKey: idempotencyKey,
		Status:         "running",
		InputPreview:   inputPreview,
	}
	startedAt := time.Now()
	var err error
	if !effectLease.Succeeded {
		switch node.Type {
		case workflowregistry.NodeTypeSubflow:
			err = e.executeSubflow(ctx, local, node)
		case workflowregistry.NodeTypeLoop:
			err = e.executeLoop(ctx, local, node)
		default:
			err = e.nodes.executeNode(ctx, local, node)
		}
		if hasEffect && local.input.EffectStore != nil {
			if err != nil {
				_ = local.input.EffectStore.Fail(ctx, effectLease, err.Error())
			} else {
				resultData, marshalErr := json.Marshal(local.vars[node.ID])
				if marshalErr != nil {
					err = marshalErr
					_ = local.input.EffectStore.Fail(ctx, effectLease, marshalErr.Error())
				} else if completeErr := local.input.EffectStore.Complete(ctx, effectLease, effectRequest, string(resultData)); completeErr != nil {
					err = completeErr
				}
			}
		}
	}
	recoveredError := err != nil && strings.TrimSpace(node.ErrorTargetNodeID) != ""
	if recoveredError {
		local.setNodeVars(node.ID, map[string]any{
			"errorMessage":      err.Error(),
			"errorRecovered":    true,
			"errorTargetNodeId": strings.TrimSpace(node.ErrorTargetNodeID),
		})
	}
	trace.DurationMS = int(time.Since(startedAt).Milliseconds())
	trace.OutputPreview = workflowPreviewJSON(local.nodeOutputPreview(node.ID))
	if recoveredError {
		trace.Status = "recovered"
		trace.ErrorMessage = err.Error()
	} else if err != nil {
		trace.Status = "failed"
		trace.ErrorMessage = err.Error()
	} else if local.result.Interrupted {
		trace.Status = "interrupted"
	} else {
		trace.Status = "completed"
	}

	execution.mu.Lock()
	execution.state.vars[node.ID] = cloneWorkflowVars(local.vars[node.ID])
	mergeGraphNodeResult(&execution.state.result, &local.result, node.ID, trace)
	execution.mu.Unlock()
	if recoveredError {
		return nil
	}
	return err
}

func buildWorkflowEffectRequest(input Input, node dsl.Node, requestData string) (WorkflowEffectRequest, bool) {
	effectType := ""
	switch node.Type {
	case workflowregistry.NodeTypeCreateTicket:
		effectType = workflowregistry.NodeTypeCreateTicket
	case workflowregistry.NodeTypeCreateVideoMeeting:
		effectType = workflowregistry.NodeTypeCreateVideoMeeting
	case workflowregistry.NodeTypeCreateKnowledgeCandidate:
		effectType = workflowregistry.NodeTypeCreateKnowledgeCandidate
	case workflowregistry.NodeTypeHandoffToHuman:
		effectType = workflowregistry.NodeTypeHandoffToHuman
	default:
		return WorkflowEffectRequest{}, false
	}
	workflowVersionID := int64(0)
	if len(input.WorkflowVersionStack) > 0 {
		workflowVersionID = input.WorkflowVersionStack[len(input.WorkflowVersionStack)-1]
	}
	versionPath := utils.JoinInt64s(input.WorkflowVersionStack)
	if versionPath == "" {
		versionPath = "0"
	}
	nodePath := strings.Join(append(append([]string(nil), input.WorkflowNodeStack...), node.ID), "/")
	keyMaterial := fmt.Sprintf("%d:%d:%d:%d:%s:%s", input.Conversation.TenantID, input.Conversation.ID, input.UserMessage.ID, input.AIAgent.ActiveReleaseID, versionPath, nodePath)
	keyDigest := sha256.Sum256([]byte(keyMaterial))
	key := fmt.Sprintf("workflow-effect:%x", keyDigest[:])
	return WorkflowEffectRequest{
		TenantID:          input.Conversation.TenantID,
		ProductID:         input.Conversation.ProductID,
		WorkflowVersionID: workflowVersionID,
		AgentReleaseID:    input.AIAgent.ActiveReleaseID,
		ConversationID:    input.Conversation.ID,
		MessageID:         input.UserMessage.ID,
		NodeID:            node.ID,
		EffectType:        effectType,
		IdempotencyKey:    key,
		RequestData:       requestData,
	}, true
}

func (e *GraphExecutor) executeSubflow(ctx context.Context, state *runState, node dsl.Node) error {
	config := dsl.SubflowConfig{}
	if err := json.Unmarshal(node.Config, &config); err != nil {
		return fmt.Errorf("invalid subflow config: %w", err)
	}
	result, resolved, err := e.executeWorkflowVersion(ctx, state.input, config.WorkflowVersionID, node.ID)
	if err != nil {
		return err
	}
	if result.Interrupted {
		return fmt.Errorf("subflow %d interrupted; place human_confirm in the parent workflow", resolved.VersionID)
	}
	mergeChildWorkflowResult(&state.result, result)
	state.setNodeVars(node.ID, map[string]any{
		"status":            result.Status,
		"replyText":         result.ReplyText,
		"nodePath":          append([]string(nil), result.NodePath...),
		"workflowId":        resolved.WorkflowID,
		"workflowVersionId": resolved.VersionID,
		"definitionHash":    resolved.DefinitionHash,
	})
	return nil
}

func (e *GraphExecutor) executeLoop(ctx context.Context, state *runState, node dsl.Node) error {
	config := dsl.LoopConfig{}
	if err := json.Unmarshal(node.Config, &config); err != nil {
		return fmt.Errorf("invalid loop config: %w", err)
	}
	maxIterations := config.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 3
	}
	if maxIterations > 10 {
		return fmt.Errorf("loop maxIterations must not exceed 10")
	}
	failurePolicy := strings.TrimSpace(config.FailurePolicy)
	if failurePolicy == "" {
		failurePolicy = "fail"
	}
	completed := false
	for iteration := 1; iteration <= maxIterations; iteration++ {
		result, _, err := e.executeWorkflowVersion(ctx, state.input, config.WorkflowVersionID, node.ID)
		if err != nil {
			state.setNodeVars(node.ID, map[string]any{
				"status":    "error",
				"iteration": iteration,
				"completed": false,
				"lastError": err.Error(),
			})
			if failurePolicy == "continue" {
				continue
			}
			return err
		}
		if result.Interrupted {
			return fmt.Errorf("loop subflow interrupted; place human_confirm outside the loop")
		}
		mergeChildWorkflowResult(&state.result, result)
		state.setNodeVars(node.ID, map[string]any{
			"status":    result.Status,
			"replyText": result.ReplyText,
			"iteration": iteration,
			"completed": false,
			"nodePath":  append([]string(nil), result.NodePath...),
		})
		if config.Until != nil {
			matched, _, err := state.evaluateConditionBranch(node.ID, dsl.ConditionBranch{
				ID:           "loop_until",
				TargetNodeID: node.ID,
				Condition:    config.Until,
			})
			if err != nil {
				return err
			}
			if matched {
				completed = true
				state.vars[node.ID]["completed"] = true
				break
			}
		}
	}
	if !completed && config.Until != nil {
		state.vars[node.ID]["status"] = "max_iterations_reached"
	}
	return nil
}

func (e *GraphExecutor) executeWorkflowVersion(ctx context.Context, input Input, versionID int64, callSiteNodeID string) (*Result, ResolvedWorkflowVersion, error) {
	if versionID <= 0 {
		return nil, ResolvedWorkflowVersion{}, fmt.Errorf("workflowVersionId is required")
	}
	if input.WorkflowResolver == nil {
		return nil, ResolvedWorkflowVersion{}, fmt.Errorf("workflow version resolver is not configured")
	}
	if containsInt64(input.WorkflowVersionStack, versionID) {
		return nil, ResolvedWorkflowVersion{}, fmt.Errorf("recursive workflow version reference detected: %d", versionID)
	}
	resolved, err := input.WorkflowResolver.ResolveWorkflowVersion(ctx, WorkflowVersionReference{
		TenantID:          input.Conversation.TenantID,
		ProductID:         input.Conversation.ProductID,
		WorkflowVersionID: versionID,
	})
	if err != nil {
		return nil, ResolvedWorkflowVersion{}, err
	}
	childInput := input
	childInput.Definition = resolved.Definition
	childInput.WorkflowVersionStack = append(append([]int64(nil), input.WorkflowVersionStack...), versionID)
	childInput.WorkflowNodeStack = append(append([]string(nil), input.WorkflowNodeStack...), strings.TrimSpace(callSiteNodeID))
	result, err := NewGraphExecutor().Execute(ctx, childInput)
	return result, resolved, err
}

func mergeChildWorkflowResult(target *Result, child *Result) {
	if target == nil || child == nil {
		return
	}
	target.PromptTokens += child.PromptTokens
	target.CompletionTokens += child.CompletionTokens
	if strings.TrimSpace(child.ModelProvider) != "" {
		target.ModelProvider = strings.TrimSpace(child.ModelProvider)
	}
	if strings.TrimSpace(child.ModelName) != "" {
		target.ModelName = strings.TrimSpace(child.ModelName)
	}
	target.RetrieverCount += child.RetrieverCount
	target.ToolCallCount += child.ToolCallCount
	for _, code := range child.ToolCodes {
		target.ToolCodes = appendWorkflowUnique(target.ToolCodes, code)
	}
	for _, code := range child.InvokedToolCodes {
		target.InvokedToolCodes = appendWorkflowUnique(target.InvokedToolCodes, code)
	}
	if child.TraceData != "" {
		target.TraceData = mergeAgentNodeTrace(target.TraceData, "subflow", child.TraceData)
	}
}

func buildGraphNodeIdempotencyKey(input Input, nodeID string, attempt int) string {
	return fmt.Sprintf("workflow-node:%d:%d:%s:%d", input.Conversation.ID, input.UserMessage.ID, strings.TrimSpace(nodeID), attempt)
}

func cloneRunStateForNode(source *runState) *runState {
	ret := &runState{
		input:           source.input,
		nodesByID:       source.nodesByID,
		outgoing:        source.outgoing,
		vars:            make(map[string]map[string]any, len(source.vars)),
		branchDecisions: make(map[string]branchDecision, len(source.branchDecisions)),
		result: Result{
			Status:     "started",
			NodePath:   make([]string, 0, 1),
			NodeTraces: make([]NodeTrace, 0, 1),
		},
	}
	for nodeID, values := range source.vars {
		ret.vars[nodeID] = cloneWorkflowVars(values)
	}
	for nodeID, decision := range source.branchDecisions {
		ret.branchDecisions[nodeID] = decision
	}
	return ret
}

func cloneWorkflowVars(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	ret := make(map[string]any, len(values))
	for key, value := range values {
		ret[key] = value
	}
	return ret
}

func mergeGraphNodeResult(target *Result, source *Result, nodeID string, trace NodeTrace) {
	target.NodePath = append(target.NodePath, nodeID)
	target.NodeTraces = append(target.NodeTraces, trace)
	if source.ReplyText != "" {
		target.ReplyText = source.ReplyText
	}
	if source.SelectedSkillID > 0 {
		target.SelectedSkillID = source.SelectedSkillID
		target.SelectedSkillName = source.SelectedSkillName
		target.SkillRouteReason = source.SkillRouteReason
		target.SkillRouteTrace = source.SkillRouteTrace
		target.SkillAllowedToolCodes = append([]string(nil), source.SkillAllowedToolCodes...)
	}
	target.PromptTokens += source.PromptTokens
	target.CompletionTokens += source.CompletionTokens
	if strings.TrimSpace(source.ModelProvider) != "" {
		target.ModelProvider = strings.TrimSpace(source.ModelProvider)
	}
	if strings.TrimSpace(source.ModelName) != "" {
		target.ModelName = strings.TrimSpace(source.ModelName)
	}
	if source.HistoryMessageCount > target.HistoryMessageCount {
		target.HistoryMessageCount = source.HistoryMessageCount
	}
	target.RetrieverCount += source.RetrieverCount
	target.ToolCallCount += source.ToolCallCount
	for _, code := range source.ToolCodes {
		target.ToolCodes = appendWorkflowUnique(target.ToolCodes, code)
	}
	for _, code := range source.InvokedToolCodes {
		target.InvokedToolCodes = appendWorkflowUnique(target.InvokedToolCodes, code)
	}
	if source.TraceData != "" {
		target.TraceData = mergeAgentNodeTrace(target.TraceData, nodeID, source.TraceData)
	}
	if source.Interrupted {
		target.Interrupted = true
		target.CheckPointID = source.CheckPointID
		target.CheckPointData = source.CheckPointData
		target.Interrupts = append([]InterruptSummary(nil), source.Interrupts...)
	}
}

func graphReachableNodes(state *runState, startNodeID string) (map[string]bool, error) {
	startNodeID = strings.TrimSpace(startNodeID)
	if _, ok := state.nodesByID[startNodeID]; !ok {
		return nil, fmt.Errorf("workflow node does not exist: %s", startNodeID)
	}
	reachable := make(map[string]bool, len(state.nodesByID))
	queue := []string{startNodeID}
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		if reachable[nodeID] {
			continue
		}
		reachable[nodeID] = true
		for _, edge := range state.outgoing[nodeID] {
			target := strings.TrimSpace(edge.Target)
			if _, ok := state.nodesByID[target]; !ok {
				return nil, fmt.Errorf("workflow edge target does not exist: %s", target)
			}
			if !reachable[target] {
				queue = append(queue, target)
			}
		}
	}
	return reachable, nil
}

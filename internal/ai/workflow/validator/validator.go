package validator

import (
	"encoding/json"
	"fmt"
	"strings"

	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/ai/workflow/registry"
)

type Error struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type Result struct {
	Valid  bool    `json:"valid"`
	Errors []Error `json:"errors"`
}

func ValidateDefinition(def dsl.Definition, reg *registry.Registry) Result {
	if reg == nil {
		reg = registry.DefaultRegistry()
	}
	v := definitionValidator{
		def:          def,
		registry:     reg,
		nodesByID:    make(map[string]dsl.Node, len(def.Nodes)),
		outgoing:     make(map[string][]string),
		incoming:     make(map[string][]string),
		startNodeIDs: make([]string, 0, 1),
		endNodeIDs:   make([]string, 0, 1),
	}
	v.validate()
	return Result{
		Valid:  len(v.errors) == 0,
		Errors: v.errors,
	}
}

type definitionValidator struct {
	def          dsl.Definition
	registry     *registry.Registry
	nodesByID    map[string]dsl.Node
	runtimeVars  map[string]dsl.RuntimeVariable
	outgoing     map[string][]string
	incoming     map[string][]string
	startNodeIDs []string
	endNodeIDs   []string
	errors       []Error
}

func (v *definitionValidator) validate() {
	v.validateModelPolicy()
	v.validateRuntimeVariables()
	v.validateNodes()
	v.validateEdges()
	v.validateErrorTransitions()
	v.validateEntry()
	v.validateReachability()
	v.validateAcyclicGraph()
	v.validateConfirmationGuards()
	v.validateVariableMappings()
	v.validateConditions()
	v.validateRuntimeNodeConfigs()
}

func (v *definitionValidator) validateModelPolicy() {
	if v.def.ModelPolicy == nil || len(v.def.ModelPolicy.CredentialChain) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(v.def.ModelPolicy.CredentialChain))
	for index, rawScope := range v.def.ModelPolicy.CredentialChain {
		scope := strings.TrimSpace(rawScope)
		field := fmt.Sprintf("modelPolicy.credentialChain[%d]", index)
		if !dsl.IsSupportedModelCredentialScope(scope) {
			v.addError(field, "model credential source must be tenant_default or product")
			continue
		}
		if _, exists := seen[scope]; exists {
			v.addError(field, "model credential source must not be repeated")
			continue
		}
		seen[scope] = struct{}{}
	}
}

func (v *definitionValidator) validateErrorTransitions() {
	for id, node := range v.nodesByID {
		targetNodeID := strings.TrimSpace(node.ErrorTargetNodeID)
		if targetNodeID == "" {
			continue
		}
		field := "nodes." + id + ".errorTargetNodeId"
		switch node.Type {
		case registry.NodeTypeStart, registry.NodeTypeCondition, registry.NodeTypeEnd:
			v.addError(field, "this node type does not support an error target")
			continue
		}
		if targetNodeID == id {
			v.addError(field, "error target must not reference the current node")
			continue
		}
		if _, ok := v.nodesByID[targetNodeID]; !ok {
			v.addError(field, "error target node does not exist: "+targetNodeID)
			continue
		}
		if !v.hasEdgeTo(id, targetNodeID) {
			v.addError(field, "error target must have an outgoing edge: "+targetNodeID)
		}
		normalTargets := make(map[string]struct{})
		for _, outgoingTarget := range v.outgoing[id] {
			if outgoingTarget != targetNodeID {
				normalTargets[outgoingTarget] = struct{}{}
			}
		}
		if len(normalTargets) > 1 {
			v.addError(field, "a node with an error target may have only one normal target")
		}
	}
}

func (v *definitionValidator) validateRuntimeVariables() {
	v.runtimeVars = make(map[string]dsl.RuntimeVariable, len(v.def.RuntimeVariables))
	for index, variable := range v.def.RuntimeVariables {
		field := fmt.Sprintf("runtimeVariables[%d]", index)
		variable.Key = strings.TrimSpace(variable.Key)
		variable.Type = strings.TrimSpace(variable.Type)
		variable.Source = strings.TrimSpace(variable.Source)
		if variable.Key == "" {
			v.addError(field+".key", "runtime variable key is required")
			continue
		}
		if _, exists := v.runtimeVars[variable.Key]; exists {
			v.addError(field+".key", "duplicate runtime variable: "+variable.Key)
			continue
		}
		if variable.Type == "" {
			v.addError(field+".type", "runtime variable type is required")
		}
		if variable.Source == "" {
			v.addError(field+".source", "runtime variable source is required")
		}
		v.runtimeVars[variable.Key] = variable
	}
}

func (v *definitionValidator) validateAcyclicGraph() {
	const (
		unvisited = iota
		visiting
		visited
	)
	states := make(map[string]int, len(v.nodesByID))
	var visit func(string) bool
	visit = func(nodeID string) bool {
		switch states[nodeID] {
		case visiting:
			return false
		case visited:
			return true
		}
		states[nodeID] = visiting
		for _, target := range v.outgoing[nodeID] {
			if !visit(target) {
				return false
			}
		}
		states[nodeID] = visited
		return true
	}
	for nodeID := range v.nodesByID {
		if states[nodeID] == unvisited && !visit(nodeID) {
			v.addError("edges", "workflow graph cycles are not allowed; use a bounded loop node")
			return
		}
	}
}

func (v *definitionValidator) validateNodes() {
	for index, node := range v.def.Nodes {
		node.ID = strings.TrimSpace(node.ID)
		node.Type = strings.TrimSpace(node.Type)
		field := fmt.Sprintf("nodes[%d]", index)
		if node.ID == "" {
			v.addError(field+".id", "node id is required")
			continue
		}
		if _, exists := v.nodesByID[node.ID]; exists {
			v.addError(field+".id", "duplicate node id: "+node.ID)
			continue
		}
		v.nodesByID[node.ID] = node
		if node.Type == "" {
			v.addError(field+".type", "node type is required")
			continue
		}
		if _, ok := v.registry.Get(node.Type); !ok {
			v.addError(field+".type", "unknown node type: "+node.Type)
			continue
		}
		switch node.Type {
		case registry.NodeTypeStart:
			v.startNodeIDs = append(v.startNodeIDs, node.ID)
		case registry.NodeTypeEnd:
			v.endNodeIDs = append(v.endNodeIDs, node.ID)
		}
	}
	if len(v.startNodeIDs) != 1 {
		v.addError("nodes", "workflow must contain exactly one start node")
	}
	if len(v.endNodeIDs) == 0 {
		v.addError("nodes", "workflow must contain at least one end node")
	}
}

func (v *definitionValidator) validateEdges() {
	seen := make(map[string]struct{}, len(v.def.Edges))
	for index, edge := range v.def.Edges {
		edge.ID = strings.TrimSpace(edge.ID)
		edge.Source = strings.TrimSpace(edge.Source)
		edge.Target = strings.TrimSpace(edge.Target)
		field := fmt.Sprintf("edges[%d]", index)
		if edge.ID == "" {
			v.addError(field+".id", "edge id is required")
		} else if _, exists := seen[edge.ID]; exists {
			v.addError(field+".id", "duplicate edge id: "+edge.ID)
		}
		seen[edge.ID] = struct{}{}
		if edge.Source == "" {
			v.addError(field+".source", "edge source is required")
		} else if _, ok := v.nodesByID[edge.Source]; !ok {
			v.addError(field+".source", "edge source node does not exist: "+edge.Source)
		}
		if edge.Target == "" {
			v.addError(field+".target", "edge target is required")
		} else if _, ok := v.nodesByID[edge.Target]; !ok {
			v.addError(field+".target", "edge target node does not exist: "+edge.Target)
		}
		if edge.Source != "" && edge.Target != "" {
			v.outgoing[edge.Source] = append(v.outgoing[edge.Source], edge.Target)
			v.incoming[edge.Target] = append(v.incoming[edge.Target], edge.Source)
		}
	}
}

func (v *definitionValidator) validateEntry() {
	entryNodeID := strings.TrimSpace(v.def.EntryNodeID)
	if entryNodeID == "" {
		v.addError("entryNodeId", "entry node id is required")
		return
	}
	entry, ok := v.nodesByID[entryNodeID]
	if !ok {
		v.addError("entryNodeId", "entry node does not exist: "+entryNodeID)
		return
	}
	if entry.Type != registry.NodeTypeStart {
		v.addError("entryNodeId", "entry node must be the start node")
	}
}

func (v *definitionValidator) validateReachability() {
	entryNodeID := strings.TrimSpace(v.def.EntryNodeID)
	if entryNodeID == "" {
		return
	}
	if _, ok := v.nodesByID[entryNodeID]; !ok {
		return
	}
	reachable := make(map[string]struct{}, len(v.nodesByID))
	queue := []string{entryNodeID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if _, exists := reachable[current]; exists {
			continue
		}
		reachable[current] = struct{}{}
		for _, target := range v.outgoing[current] {
			if _, exists := reachable[target]; !exists {
				queue = append(queue, target)
			}
		}
	}
	for id := range v.nodesByID {
		if _, ok := reachable[id]; !ok {
			v.addError("nodes", "node is not reachable from entry node: "+id)
		}
	}
}

func (v *definitionValidator) validateConfirmationGuards() {
	for id, node := range v.nodesByID {
		spec, ok := v.registry.Get(node.Type)
		if !ok || !spec.RequiresConfirmationPredecessor {
			continue
		}
		if !v.hasConfirmationPredecessor(id, make(map[string]struct{})) {
			v.addError("nodes."+id, node.Type+" requires human_confirm before execution")
		}
		v.validateConfirmedInput(id, node)
	}
}

func (v *definitionValidator) validateConfirmedInput(nodeID string, node dsl.Node) {
	selector, ok := node.Inputs["confirmed"]
	if !ok || strings.TrimSpace(selector.NodeID) == "" || strings.TrimSpace(selector.Field) == "" {
		return
	}
	sourceNodeID := strings.TrimSpace(selector.NodeID)
	sourceNode, ok := v.nodesByID[sourceNodeID]
	if !ok {
		return
	}
	if sourceNode.Type != registry.NodeTypeHumanConfirm || strings.TrimSpace(selector.Field) != "confirmed" {
		v.addError("nodes."+nodeID+".inputs.confirmed", "confirmed input must come from human_confirm.confirmed")
	}
}

func (v *definitionValidator) validateVariableMappings() {
	for id, node := range v.nodesByID {
		spec, ok := v.registry.Get(node.Type)
		if !ok {
			continue
		}
		for _, input := range spec.InputSchema {
			if !input.Required {
				continue
			}
			selector, ok := node.Inputs[input.Name]
			if !ok || strings.TrimSpace(selector.NodeID) == "" || strings.TrimSpace(selector.Field) == "" {
				v.addError("nodes."+id+".inputs."+input.Name, "required input mapping is missing: "+input.Name)
				continue
			}
			v.validateInputSelector(id, input, selector)
		}
		for inputName, selector := range node.Inputs {
			if strings.TrimSpace(selector.NodeID) == "" || strings.TrimSpace(selector.Field) == "" {
				v.addError("nodes."+id+".inputs."+inputName, "input mapping source is required")
				continue
			}
			if _, ok := findInputSpec(spec.InputSchema, inputName); ok {
				continue
			}
			sourceNode, sourceOK := v.nodesByID[strings.TrimSpace(selector.NodeID)]
			if !sourceOK {
				v.addError("nodes."+id+".inputs."+inputName, "input source node does not exist: "+selector.NodeID)
				continue
			}
			sourceSpec, sourceSpecOK := v.registry.Get(sourceNode.Type)
			if !sourceSpecOK {
				continue
			}
			if _, ok := findOutputSpec(sourceSpec.OutputSchema, selector.Field); !ok {
				v.addError("nodes."+id+".inputs."+inputName, "input source field does not exist: "+selector.NodeID+"."+selector.Field)
			}
		}
	}
}

func (v *definitionValidator) validateInputSelector(nodeID string, input registry.VariableSpec, selector dsl.VariableSelector) {
	sourceNodeID := strings.TrimSpace(selector.NodeID)
	sourceField := strings.TrimSpace(selector.Field)
	sourceNode, ok := v.nodesByID[sourceNodeID]
	if !ok {
		v.addError("nodes."+nodeID+".inputs."+input.Name, "input source node does not exist: "+sourceNodeID)
		return
	}
	if !v.hasPath(sourceNodeID, nodeID, make(map[string]struct{})) {
		v.addError("nodes."+nodeID+".inputs."+input.Name, "input source node is not available before current node: "+sourceNodeID)
		return
	}
	sourceSpec, ok := v.registry.Get(sourceNode.Type)
	if !ok {
		return
	}
	output, ok := findOutputSpec(sourceSpec.OutputSchema, sourceField)
	if !ok {
		v.addError("nodes."+nodeID+".inputs."+input.Name, "input source field does not exist: "+sourceNodeID+"."+sourceField)
		return
	}
	if !variableTypesCompatible(input.Type, output.Type) {
		v.addError("nodes."+nodeID+".inputs."+input.Name, fmt.Sprintf("input type mismatch: %s expects %s but %s.%s is %s", input.Name, input.Type, sourceNodeID, sourceField, output.Type))
	}
}

func (v *definitionValidator) validateConditions() {
	for index, node := range v.def.Nodes {
		if strings.TrimSpace(node.Type) != registry.NodeTypeCondition {
			continue
		}
		field := fmt.Sprintf("nodes[%d].config.branches", index)
		config := dsl.ConditionConfig{}
		if len(node.Config) > 0 {
			if err := json.Unmarshal(node.Config, &config); err != nil {
				v.addError(field, "condition branches config must be valid JSON")
				continue
			}
		}
		if len(config.Branches) == 0 {
			v.addError(field, "condition node must include at least one branch")
			continue
		}
		defaultCount := 0
		seenBranchIDs := make(map[string]struct{}, len(config.Branches))
		for branchIndex, branch := range config.Branches {
			branchField := fmt.Sprintf("%s[%d]", field, branchIndex)
			branchID := strings.TrimSpace(branch.ID)
			if branchID == "" {
				v.addError(branchField+".id", "condition branch id is required")
			} else if _, exists := seenBranchIDs[branchID]; exists {
				v.addError(branchField+".id", "duplicate condition branch id: "+branchID)
			}
			seenBranchIDs[branchID] = struct{}{}
			targetNodeID := strings.TrimSpace(branch.TargetNodeID)
			if targetNodeID == "" {
				v.addError(branchField+".targetNodeId", "condition branch target node is required")
			} else if _, ok := v.nodesByID[targetNodeID]; !ok {
				v.addError(branchField+".targetNodeId", "condition branch target node does not exist: "+targetNodeID)
			}
			if !v.hasEdgeTo(strings.TrimSpace(node.ID), targetNodeID) {
				v.addError(branchField+".targetNodeId", "condition branch target must have an outgoing edge: "+targetNodeID)
			}
			if branch.Default {
				defaultCount++
				if branch.Condition != nil {
					v.addError(branchField+".condition", "default condition branch must not define a condition")
				}
				continue
			}
			v.validateCondition(branchField+".condition", strings.TrimSpace(node.ID), branch.Condition)
		}
		if defaultCount != 1 {
			v.addError(field, "condition node must include exactly one default branch")
		}
	}
}

func (v *definitionValidator) validateRuntimeNodeConfigs() {
	for index, node := range v.def.Nodes {
		field := fmt.Sprintf("nodes[%d].config", index)
		if forbidden := forbiddenResourceConfigKey(node.Config); forbidden != "" {
			v.addError(field+"."+forbidden, "workflow definition must use logical scope instead of resource identifiers")
		}
		switch strings.TrimSpace(node.Type) {
		case registry.NodeTypeEntryContext:
			config := dsl.EntryContextConfig{}
			if len(node.Config) > 0 {
				if err := json.Unmarshal(node.Config, &config); err != nil {
					v.addError(field, "entry_context config must be valid JSON")
					continue
				}
			}
			switch strings.TrimSpace(config.UnboundMode) {
			case "", "quick_ai", "product_context":
			default:
				v.addError(field+".unboundMode", "entry_context unboundMode must be quick_ai or product_context")
			}
		case registry.NodeTypeServiceAccessPolicy:
			config := dsl.ServiceAccessPolicyConfig{}
			if len(node.Config) > 0 {
				if err := json.Unmarshal(node.Config, &config); err != nil {
					v.addError(field, "service_access_policy config must be valid JSON")
					continue
				}
			}
			if !isSupportedServiceAccessMode(config.HumanHandoffMode) {
				v.addError(field+".humanHandoffMode", "service access mode must be inherit, always, never, or device_only")
			}
			if !isSupportedServiceAccessMode(config.TicketAccessMode) {
				v.addError(field+".ticketAccessMode", "service access mode must be inherit, always, never, or device_only")
			}
		case registry.NodeTypeKnowledgeRetrieve:
			config := dsl.KnowledgeRetrieveConfig{}
			if err := json.Unmarshal(node.Config, &config); err != nil {
				v.addError(field, "knowledge_retrieve config must be valid JSON")
				continue
			}
			bindingMethod := strings.TrimSpace(config.EffectiveBindingMethod())
			switch bindingMethod {
			case "agent_default", "product_context", "public_product", "internal_product":
			default:
				v.addError(field+".bindingMethod", "knowledge_retrieve must declare a supported knowledge binding method")
			}
			if variableKey := strings.TrimSpace(config.KnowledgeBaseIDsVariable); variableKey != "" {
				variable, ok := v.runtimeVars[variableKey]
				if !ok {
					v.addError(field+".knowledgeBaseIdsVariable", "knowledge_retrieve runtime variable is not declared: "+variableKey)
				} else if variable.Type != "array<int64>" {
					v.addError(field+".knowledgeBaseIdsVariable", "knowledge_retrieve runtime variable must use array<int64>")
				}
			}
			if config.TopK < 0 || config.TopK > 50 {
				v.addError(field+".topK", "knowledge_retrieve topK must be between 1 and 50 when configured")
			}
			if config.ScoreThreshold < 0 || config.ScoreThreshold > 1 {
				v.addError(field+".scoreThreshold", "knowledge_retrieve scoreThreshold must be between 0 and 1")
			}
			if config.ContextMaxTokens < 0 || config.ContextMaxTokens > 32000 {
				v.addError(field+".contextMaxTokens", "knowledge_retrieve contextMaxTokens must be between 1 and 32000 when configured")
			}
			if config.MaxContextItems < 0 || config.MaxContextItems > 50 {
				v.addError(field+".maxContextItems", "knowledge_retrieve maxContextItems must be between 1 and 50 when configured")
			}
		case registry.NodeTypeKnowledgeMerge:
			config := dsl.KnowledgeMergeConfig{}
			if err := json.Unmarshal(node.Config, &config); err != nil {
				v.addError(field, "knowledge_merge config must be valid JSON")
				continue
			}
			if len(config.Sources) == 0 {
				v.addError(field+".sources", "knowledge_merge requires at least one source")
			}
			if config.MaxItems < 0 || config.MaxItems > 50 {
				v.addError(field+".maxItems", "knowledge_merge maxItems must be between 1 and 50 when configured")
			}
			for sourceIndex, selector := range config.Sources {
				sourceField := fmt.Sprintf("%s.sources[%d]", field, sourceIndex)
				sourceNode := v.nodesByID[strings.TrimSpace(selector.NodeID)]
				if sourceNode.ID == "" || sourceNode.Type != registry.NodeTypeKnowledgeRetrieve || strings.TrimSpace(selector.Field) != "items" {
					v.addError(sourceField, "knowledge_merge source must reference knowledge_retrieve.items")
					continue
				}
				if !v.hasPath(sourceNode.ID, strings.TrimSpace(node.ID), make(map[string]struct{})) {
					v.addError(sourceField, "knowledge_merge source must execute before the merge node")
				}
			}
		case registry.NodeTypeAnswerabilityGate:
			config := dsl.AnswerabilityGateConfig{}
			if len(node.Config) > 0 {
				if err := json.Unmarshal(node.Config, &config); err != nil {
					v.addError(field, "answerability_gate config must be valid JSON")
					continue
				}
			}
			if config.MinScore < 0 || config.MinScore > 1 {
				v.addError(field+".minScore", "answerability_gate minScore must be between 0 and 1")
			}
			if config.StrongScore < 0 || config.StrongScore > 1 {
				v.addError(field+".strongScore", "answerability_gate strongScore must be between 0 and 1")
			}
			if config.MinScore > 0 && config.StrongScore > 0 && config.StrongScore < config.MinScore {
				v.addError(field+".strongScore", "answerability_gate strongScore must be greater than or equal to minScore")
			}
			if config.MinMatchedTerms < 0 || config.MinMatchedTerms > 20 {
				v.addError(field+".minMatchedTerms", "answerability_gate minMatchedTerms must be between 1 and 20 when configured")
			}
		case registry.NodeTypeSubflow:
			config := dsl.SubflowConfig{}
			if err := json.Unmarshal(node.Config, &config); err != nil {
				v.addError(field, "subflow config must be valid JSON")
				continue
			}
			if config.WorkflowVersionID <= 0 {
				v.addError(field+".workflowVersionId", "subflow must reference a published workflow version")
			}
		case registry.NodeTypeLoop:
			config := dsl.LoopConfig{}
			if err := json.Unmarshal(node.Config, &config); err != nil {
				v.addError(field, "loop config must be valid JSON")
				continue
			}
			if config.WorkflowVersionID <= 0 {
				v.addError(field+".workflowVersionId", "loop must reference a published workflow version")
			}
			if config.MaxIterations <= 0 || config.MaxIterations > 10 {
				v.addError(field+".maxIterations", "loop maxIterations must be between 1 and 10")
			}
			failurePolicy := strings.TrimSpace(config.FailurePolicy)
			if failurePolicy != "" && failurePolicy != "fail" && failurePolicy != "continue" {
				v.addError(field+".failurePolicy", "loop failurePolicy must be fail or continue")
			}
			if config.Until != nil {
				v.validateCondition(field+".until", strings.TrimSpace(node.ID), config.Until)
				if config.Until.Left != nil && strings.TrimSpace(config.Until.Left.NodeID) != strings.TrimSpace(node.ID) {
					v.addError(field+".until.left", "loop exit condition must reference the loop node output")
				}
			}
		}
	}
}

func isSupportedServiceAccessMode(value string) bool {
	switch strings.TrimSpace(value) {
	case "", "inherit", "always", "never", "device_only":
		return true
	default:
		return false
	}
}

func forbiddenResourceConfigKey(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	forbidden := map[string]struct{}{
		"knowledgebaseid": {}, "knowledgebaseids": {}, "productid": {}, "productids": {},
		"teamid": {}, "teamids": {}, "connectorid": {}, "connectorids": {}, "mcpserverid": {},
	}
	var walk func(any) string
	walk = func(current any) string {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(key), "_", ""), "-", ""))
				if _, exists := forbidden[normalized]; exists {
					return key
				}
				if found := walk(child); found != "" {
					return found
				}
			}
		case []any:
			for _, child := range typed {
				if found := walk(child); found != "" {
					return found
				}
			}
		}
		return ""
	}
	return walk(value)
}

func (v *definitionValidator) validateCondition(field string, sourceNodeID string, condition *dsl.Condition) {
	if condition == nil {
		v.addError(field, "condition branch condition is required")
		return
	}
	operator := strings.TrimSpace(condition.Operator)
	if operator == "" && strings.TrimSpace(condition.Expression) != "" {
		v.addError(field+".expression", "free-form condition expressions are not supported")
		return
	}
	if !isSupportedConditionOperator(operator) {
		v.addError(field+".operator", "unsupported condition operator: "+operator)
		return
	}
	if condition.Left == nil {
		v.addError(field+".left", "condition left variable is required")
		return
	}
	sourceSelectorNodeID := strings.TrimSpace(condition.Left.NodeID)
	sourceField := strings.TrimSpace(condition.Left.Field)
	if sourceSelectorNodeID == "" || sourceField == "" {
		v.addError(field+".left", "condition left variable is required")
		return
	}
	sourceNode, ok := v.nodesByID[sourceSelectorNodeID]
	if !ok {
		v.addError(field+".left", "condition source node does not exist: "+sourceSelectorNodeID)
		return
	}
	if sourceNodeID != "" && !v.hasPath(sourceSelectorNodeID, sourceNodeID, make(map[string]struct{})) && sourceSelectorNodeID != sourceNodeID {
		v.addError(field+".left", "condition source node is not available before branch: "+sourceSelectorNodeID)
		return
	}
	sourceSpec, ok := v.registry.Get(sourceNode.Type)
	if !ok {
		return
	}
	if _, ok := findOutputSpec(sourceSpec.OutputSchema, sourceField); !ok {
		v.addError(field+".left", "condition source field does not exist: "+sourceSelectorNodeID+"."+sourceField)
	}
}

func isSupportedConditionOperator(operator string) bool {
	switch strings.TrimSpace(operator) {
	case "eq", "equals", "neq", "not_equals", "contains", "exists", "not_exists", "truthy", "is_true", "falsy", "is_false", "gt", "gte", "lt", "lte":
		return true
	default:
		return false
	}
}

func (v *definitionValidator) hasPath(sourceID string, targetID string, visiting map[string]struct{}) bool {
	if sourceID == targetID {
		return false
	}
	if _, seen := visiting[sourceID]; seen {
		return false
	}
	visiting[sourceID] = struct{}{}
	for _, next := range v.outgoing[sourceID] {
		if next == targetID {
			return true
		}
		if v.hasPath(next, targetID, visiting) {
			return true
		}
	}
	return false
}

func (v *definitionValidator) hasEdgeTo(sourceID string, targetID string) bool {
	if sourceID == "" || targetID == "" {
		return true
	}
	for _, edge := range v.def.Edges {
		if strings.TrimSpace(edge.Source) == sourceID && strings.TrimSpace(edge.Target) == targetID {
			return true
		}
	}
	return false
}

func findInputSpec(items []registry.VariableSpec, name string) (registry.VariableSpec, bool) {
	name = strings.TrimSpace(name)
	for _, item := range items {
		if item.Name == name {
			return item, true
		}
	}
	return registry.VariableSpec{}, false
}

func findOutputSpec(items []registry.VariableSpec, name string) (registry.VariableSpec, bool) {
	name = strings.TrimSpace(name)
	for _, item := range items {
		if item.Name == name {
			return item, true
		}
	}
	return registry.VariableSpec{}, false
}

func variableTypesCompatible(input registry.VariableType, output registry.VariableType) bool {
	return input == registry.VariableTypeAny || output == registry.VariableTypeAny || input == output
}

func (v *definitionValidator) hasConfirmationPredecessor(nodeID string, visiting map[string]struct{}) bool {
	if _, seen := visiting[nodeID]; seen {
		return false
	}
	visiting[nodeID] = struct{}{}
	for _, source := range v.incoming[nodeID] {
		node, ok := v.nodesByID[source]
		if !ok {
			continue
		}
		if node.Type == registry.NodeTypeHumanConfirm {
			return true
		}
		if v.hasConfirmationPredecessor(source, visiting) {
			return true
		}
	}
	return false
}

func (v *definitionValidator) addError(field string, message string) {
	v.errors = append(v.errors, Error{Field: field, Message: message})
}

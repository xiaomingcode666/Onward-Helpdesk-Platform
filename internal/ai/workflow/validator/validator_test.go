package validator_test

import (
	"encoding/json"
	"strings"
	"testing"

	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/ai/workflow/registry"
	"remotehelpdesk/internal/ai/workflow/validator"
)

func TestValidateDefinitionAcceptsMinimalConversationFlow(t *testing.T) {
	result := validator.ValidateDefinition(minimalDefinition(), registry.DefaultRegistry())

	if !result.Valid {
		t.Fatalf("expected valid definition, got errors: %#v", result.Errors)
	}
}

func TestValidateDefinitionAcceptsModelCredentialPolicy(t *testing.T) {
	def := minimalDefinition()
	def.ModelPolicy = &dsl.ModelPolicy{CredentialChain: []string{
		dsl.ModelCredentialScopeProduct,
		dsl.ModelCredentialScopeTenantDefault,
	}}
	result := validator.ValidateDefinition(def, registry.DefaultRegistry())
	if !result.Valid {
		t.Fatalf("expected model credential policy to be valid: %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsUnknownModelCredentialPolicy(t *testing.T) {
	def := minimalDefinition()
	def.ModelPolicy = &dsl.ModelPolicy{CredentialChain: []string{"workflow_code_guess"}}
	result := validator.ValidateDefinition(def, registry.DefaultRegistry())
	if result.Valid || !hasValidationMessage(result, "model credential source must be") {
		t.Fatalf("expected model credential policy validation error: %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsRepeatedModelCredentialSource(t *testing.T) {
	def := minimalDefinition()
	def.ModelPolicy = &dsl.ModelPolicy{CredentialChain: []string{
		dsl.ModelCredentialScopeProduct,
		dsl.ModelCredentialScopeProduct,
	}}
	result := validator.ValidateDefinition(def, registry.DefaultRegistry())
	if result.Valid || !hasValidationMessage(result, "must not be repeated") {
		t.Fatalf("expected duplicate model credential source error: %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsRawGraphCycle(t *testing.T) {
	def := minimalDefinition()
	def.Edges = append(def.Edges, dsl.Edge{ID: "cycle", Source: "end_1", Target: "reply_1"})
	result := validator.ValidateDefinition(def, registry.DefaultRegistry())
	if result.Valid || !hasValidationMessage(result, "cycles are not allowed") {
		t.Fatalf("expected graph cycle rejection, got %#v", result.Errors)
	}
}

func TestValidateDefinitionAcceptsBoundedLoopNode(t *testing.T) {
	def := dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: registry.NodeTypeStart},
			{ID: "loop_1", Type: registry.NodeTypeLoop, Config: json.RawMessage(`{"workflowVersionId":12,"maxIterations":3,"until":{"left":{"nodeId":"loop_1","field":"iteration"},"operator":"gte","right":2}}`)},
			{ID: "end_1", Type: registry.NodeTypeEnd},
		},
		Edges: []dsl.Edge{
			{ID: "e1", Source: "start_1", Target: "loop_1"},
			{ID: "e2", Source: "loop_1", Target: "end_1"},
		},
	}
	result := validator.ValidateDefinition(def, registry.DefaultRegistry())
	if !result.Valid {
		t.Fatalf("expected bounded loop to be valid, got %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsUnboundedLoopNode(t *testing.T) {
	def := minimalDefinition()
	def.Nodes[1] = dsl.Node{ID: "reply_1", Type: registry.NodeTypeLoop, Config: json.RawMessage(`{"workflowVersionId":12,"maxIterations":100}`)}
	result := validator.ValidateDefinition(def, registry.DefaultRegistry())
	if result.Valid || !hasValidationMessage(result, "maxIterations must be between 1 and 10") {
		t.Fatalf("expected unbounded loop rejection, got %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsMissingStart(t *testing.T) {
	def := minimalDefinition()
	def.Nodes = []dsl.Node{
		{ID: "reply_1", Type: "send_reply", Config: json.RawMessage(`{"text":"hello"}`)},
		{ID: "end_1", Type: "end"},
	}

	result := validator.ValidateDefinition(def, registry.DefaultRegistry())

	if result.Valid {
		t.Fatalf("expected missing start to be invalid")
	}
	if !hasValidationMessage(result, "exactly one start node") {
		t.Fatalf("expected start error, got %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsUnknownNodeType(t *testing.T) {
	def := minimalDefinition()
	def.Nodes = append(def.Nodes, dsl.Node{ID: "unknown_1", Type: "unknown_node"})
	def.Edges = append(def.Edges, dsl.Edge{ID: "e3", Source: "reply_1", Target: "unknown_1"})

	result := validator.ValidateDefinition(def, registry.DefaultRegistry())

	if result.Valid {
		t.Fatalf("expected unknown node type to be invalid")
	}
	if !hasValidationMessage(result, "unknown node type") {
		t.Fatalf("expected unknown-node error, got %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsUnguardedCreateTicket(t *testing.T) {
	def := dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: "start"},
			{ID: "draft_1", Type: "prepare_ticket_draft"},
			{ID: "create_1", Type: "create_ticket"},
			{ID: "end_1", Type: "end"},
		},
		Edges: []dsl.Edge{
			{ID: "e1", Source: "start_1", Target: "draft_1"},
			{ID: "e2", Source: "draft_1", Target: "create_1"},
			{ID: "e3", Source: "create_1", Target: "end_1"},
		},
	}

	result := validator.ValidateDefinition(def, registry.DefaultRegistry())

	if result.Valid {
		t.Fatalf("expected unguarded create_ticket to be invalid")
	}
	if !hasValidationMessage(result, "requires human_confirm") {
		t.Fatalf("expected confirmation guard error, got %#v", result.Errors)
	}
}

func TestValidateDefinitionAcceptsConfirmedCreateTicket(t *testing.T) {
	def := dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: "start"},
			{ID: "draft_1", Type: "prepare_ticket_draft", Inputs: map[string]dsl.VariableSelector{
				"issue": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "confirm_1", Type: "human_confirm", Inputs: map[string]dsl.VariableSelector{
				"prompt": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "create_1", Type: "create_ticket", Inputs: map[string]dsl.VariableSelector{
				"ticketDraft": {NodeID: "draft_1", Field: "ticketDraft"},
				"confirmed":   {NodeID: "confirm_1", Field: "confirmed"},
			}},
			{ID: "end_1", Type: "end"},
		},
		Edges: []dsl.Edge{
			{ID: "e1", Source: "start_1", Target: "draft_1"},
			{ID: "e2", Source: "draft_1", Target: "confirm_1"},
			{ID: "e3", Source: "confirm_1", Target: "create_1"},
			{ID: "e4", Source: "create_1", Target: "end_1"},
		},
	}

	result := validator.ValidateDefinition(def, registry.DefaultRegistry())

	if !result.Valid {
		t.Fatalf("expected confirmed create_ticket to be valid, got %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsUnguardedAfterSalesToolNodes(t *testing.T) {
	for _, nodeType := range []string{registry.NodeTypeCreateVideoMeeting, registry.NodeTypeCreateKnowledgeCandidate} {
		def := dsl.Definition{
			SchemaVersion: 1,
			EntryNodeID:   "start_1",
			Nodes: []dsl.Node{
				{ID: "start_1", Type: registry.NodeTypeStart},
				{ID: "tool_1", Type: nodeType, Inputs: map[string]dsl.VariableSelector{
					"ticketId": {NodeID: "start_1", Field: "conversationId"},
				}},
				{ID: "end_1", Type: registry.NodeTypeEnd},
			},
			Edges: []dsl.Edge{
				{ID: "e1", Source: "start_1", Target: "tool_1"},
				{ID: "e2", Source: "tool_1", Target: "end_1"},
			},
		}

		result := validator.ValidateDefinition(def, registry.DefaultRegistry())
		if result.Valid {
			t.Fatalf("expected unguarded %s to be invalid", nodeType)
		}
		if !hasValidationMessage(result, "requires human_confirm") {
			t.Fatalf("expected confirmation guard error for %s, got %#v", nodeType, result.Errors)
		}
	}
}

func TestValidateDefinitionAcceptsConfirmedAfterSalesToolNodes(t *testing.T) {
	def := dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: registry.NodeTypeStart},
			{ID: "draft_1", Type: registry.NodeTypePrepareTicketDraft, Inputs: map[string]dsl.VariableSelector{
				"issue": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "confirm_1", Type: registry.NodeTypeHumanConfirm, Inputs: map[string]dsl.VariableSelector{
				"prompt": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "create_1", Type: registry.NodeTypeCreateTicket, Inputs: map[string]dsl.VariableSelector{
				"ticketDraft": {NodeID: "draft_1", Field: "ticketDraft"},
				"confirmed":   {NodeID: "confirm_1", Field: "confirmed"},
			}},
			{ID: "meeting_1", Type: registry.NodeTypeCreateVideoMeeting, Inputs: map[string]dsl.VariableSelector{
				"ticketId":  {NodeID: "create_1", Field: "ticketId"},
				"confirmed": {NodeID: "confirm_1", Field: "confirmed"},
			}},
			{ID: "knowledge_1", Type: registry.NodeTypeCreateKnowledgeCandidate, Inputs: map[string]dsl.VariableSelector{
				"ticketId":   {NodeID: "create_1", Field: "ticketId"},
				"confirmed":  {NodeID: "confirm_1", Field: "confirmed"},
				"suggestion": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "end_1", Type: registry.NodeTypeEnd},
		},
		Edges: []dsl.Edge{
			{ID: "e1", Source: "start_1", Target: "draft_1"},
			{ID: "e2", Source: "draft_1", Target: "confirm_1"},
			{ID: "e3", Source: "confirm_1", Target: "create_1"},
			{ID: "e4", Source: "create_1", Target: "meeting_1"},
			{ID: "e5", Source: "meeting_1", Target: "knowledge_1"},
			{ID: "e6", Source: "knowledge_1", Target: "end_1"},
		},
	}

	result := validator.ValidateDefinition(def, registry.DefaultRegistry())
	if !result.Valid {
		t.Fatalf("expected confirmed after-sales tool nodes to be valid, got %#v", result.Errors)
	}
}

func TestValidateDefinitionAcceptsDirectHandoffToHuman(t *testing.T) {
	def := dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: "start"},
			{ID: "handoff_1", Type: "handoff_to_human", Inputs: map[string]dsl.VariableSelector{
				"reason": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "end_1", Type: "end"},
		},
		Edges: []dsl.Edge{
			{ID: "e1", Source: "start_1", Target: "handoff_1"},
			{ID: "e2", Source: "handoff_1", Target: "end_1"},
		},
	}

	result := validator.ValidateDefinition(def, registry.DefaultRegistry())

	if !result.Valid {
		t.Fatalf("expected direct handoff_to_human to be valid, got %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsConfirmedInputFromNonConfirmNode(t *testing.T) {
	def := dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: "start"},
			{ID: "analysis_1", Type: "analyze_conversation", Inputs: map[string]dsl.VariableSelector{
				"userMessage": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "draft_1", Type: "prepare_ticket_draft", Inputs: map[string]dsl.VariableSelector{
				"issue": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "confirm_1", Type: "human_confirm", Inputs: map[string]dsl.VariableSelector{
				"prompt": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "create_1", Type: "create_ticket", Inputs: map[string]dsl.VariableSelector{
				"ticketDraft": {NodeID: "draft_1", Field: "ticketDraft"},
				"confirmed":   {NodeID: "analysis_1", Field: "needTicket"},
			}},
			{ID: "end_1", Type: "end"},
		},
		Edges: []dsl.Edge{
			{ID: "e1", Source: "start_1", Target: "analysis_1"},
			{ID: "e2", Source: "analysis_1", Target: "draft_1"},
			{ID: "e3", Source: "draft_1", Target: "confirm_1"},
			{ID: "e4", Source: "confirm_1", Target: "create_1"},
			{ID: "e5", Source: "create_1", Target: "end_1"},
		},
	}

	result := validator.ValidateDefinition(def, registry.DefaultRegistry())

	if result.Valid {
		t.Fatalf("expected confirmed input from non-confirm node to be invalid")
	}
	if !hasValidationMessage(result, "confirmed input must come from human_confirm.confirmed") {
		t.Fatalf("expected confirmed-source error, got %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsMissingRequiredInputMapping(t *testing.T) {
	def := minimalDefinition()
	def.Nodes[1].Inputs = nil

	result := validator.ValidateDefinition(def, registry.DefaultRegistry())

	if result.Valid {
		t.Fatalf("expected missing required input mapping to be invalid")
	}
	if !hasValidationMessage(result, "required input mapping is missing") {
		t.Fatalf("expected required-input error, got %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsUnknownInputSourceNode(t *testing.T) {
	def := mappedReplyDefinition()
	def.Nodes[1].Inputs["replyText"] = dsl.VariableSelector{NodeID: "missing_1", Field: "replyText"}

	result := validator.ValidateDefinition(def, registry.DefaultRegistry())

	if result.Valid {
		t.Fatalf("expected unknown input source node to be invalid")
	}
	if !hasValidationMessage(result, "input source node does not exist") {
		t.Fatalf("expected source-node error, got %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsUnknownInputSourceField(t *testing.T) {
	def := mappedReplyDefinition()
	def.Nodes[1].Inputs["replyText"] = dsl.VariableSelector{NodeID: "start_1", Field: "missing"}

	result := validator.ValidateDefinition(def, registry.DefaultRegistry())

	if result.Valid {
		t.Fatalf("expected unknown input source field to be invalid")
	}
	if !hasValidationMessage(result, "input source field does not exist") {
		t.Fatalf("expected source-field error, got %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsIncompatibleInputType(t *testing.T) {
	def := mappedReplyDefinition()
	def.Nodes[1].Inputs["replyText"] = dsl.VariableSelector{NodeID: "start_1", Field: "conversationId"}

	result := validator.ValidateDefinition(def, registry.DefaultRegistry())

	if result.Valid {
		t.Fatalf("expected incompatible input type to be invalid")
	}
	if !hasValidationMessage(result, "input type mismatch") {
		t.Fatalf("expected type-mismatch error, got %#v", result.Errors)
	}
}

func TestValidateDefinitionAcceptsMappedKnowledgeFlow(t *testing.T) {
	def := dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: "start"},
			{ID: "retrieve_1", Type: "knowledge_retrieve", Config: json.RawMessage(`{"scope":"product_context"}`), Inputs: map[string]dsl.VariableSelector{
				"query": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "reply_1", Type: "send_reply", Inputs: map[string]dsl.VariableSelector{
				"replyText": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "end_1", Type: "end"},
		},
		Edges: []dsl.Edge{
			{ID: "e1", Source: "start_1", Target: "retrieve_1"},
			{ID: "e2", Source: "retrieve_1", Target: "reply_1"},
			{ID: "e3", Source: "reply_1", Target: "end_1"},
		},
	}

	result := validator.ValidateDefinition(def, registry.DefaultRegistry())

	if !result.Valid {
		t.Fatalf("expected mapped knowledge flow to be valid, got %#v", result.Errors)
	}
}

func TestValidateDefinitionAcceptsKnowledgeBindingMethod(t *testing.T) {
	def := dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: "start"},
			{ID: "retrieve_1", Type: "knowledge_retrieve", Config: json.RawMessage(`{"bindingMethod":"public_product","topK":8}`), Inputs: map[string]dsl.VariableSelector{
				"query": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "end_1", Type: "end"},
		},
		Edges: []dsl.Edge{
			{ID: "e1", Source: "start_1", Target: "retrieve_1"},
			{ID: "e2", Source: "retrieve_1", Target: "end_1"},
		},
	}

	result := validator.ValidateDefinition(def, registry.DefaultRegistry())
	if !result.Valid {
		t.Fatalf("expected knowledge binding method to be valid, got %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsUnknownKnowledgeBindingMethod(t *testing.T) {
	def := minimalDefinition()
	def.Nodes[1] = dsl.Node{ID: "retrieve_1", Type: "knowledge_retrieve", Config: json.RawMessage(`{"bindingMethod":"tenant_knowledge_base_12"}`), Inputs: map[string]dsl.VariableSelector{
		"query": {NodeID: "start_1", Field: "userMessage"},
	}}
	def.Edges[0].Target = "retrieve_1"
	def.Edges[1].Source = "retrieve_1"

	result := validator.ValidateDefinition(def, registry.DefaultRegistry())
	if result.Valid || !hasValidationMessage(result, "supported knowledge binding method") {
		t.Fatalf("expected unknown knowledge binding method rejection, got %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsUnknownConditionOperator(t *testing.T) {
	def := conditionDefinition()
	var config dsl.ConditionConfig
	if err := json.Unmarshal(def.Nodes[1].Config, &config); err != nil {
		t.Fatalf("unmarshal condition config: %v", err)
	}
	config.Branches[0].Condition.Operator = "regex"
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal condition config: %v", err)
	}
	def.Nodes[1].Config = raw

	result := validator.ValidateDefinition(def, registry.DefaultRegistry())

	if result.Valid {
		t.Fatalf("expected unknown condition operator to be invalid")
	}
	if !hasValidationMessage(result, "unsupported condition operator") {
		t.Fatalf("expected condition operator error, got %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsUnknownConditionVariable(t *testing.T) {
	def := conditionDefinition()
	var config dsl.ConditionConfig
	if err := json.Unmarshal(def.Nodes[1].Config, &config); err != nil {
		t.Fatalf("unmarshal condition config: %v", err)
	}
	config.Branches[0].Condition.Left.Field = "missing"
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal condition config: %v", err)
	}
	def.Nodes[1].Config = raw

	result := validator.ValidateDefinition(def, registry.DefaultRegistry())

	if result.Valid {
		t.Fatalf("expected unknown condition variable to be invalid")
	}
	if !hasValidationMessage(result, "condition source field does not exist") {
		t.Fatalf("expected condition variable error, got %#v", result.Errors)
	}
}

func minimalDefinition() dsl.Definition {
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: "start"},
			{ID: "reply_1", Type: "send_reply", Config: json.RawMessage(`{"text":"hello"}`), Inputs: map[string]dsl.VariableSelector{
				"replyText": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "end_1", Type: "end"},
		},
		Edges: []dsl.Edge{
			{ID: "e1", Source: "start_1", Target: "reply_1"},
			{ID: "e2", Source: "reply_1", Target: "end_1"},
		},
	}
}

func TestValidateDefinitionRejectsConcreteResourceIDs(t *testing.T) {
	def := dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: "start"},
			{ID: "retrieve_1", Type: "knowledge_retrieve", Config: json.RawMessage(`{"scope":"product_context","knowledgeBaseIds":[12]}`), Inputs: map[string]dsl.VariableSelector{
				"query": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "end_1", Type: "end"},
		},
		Edges: []dsl.Edge{
			{ID: "e1", Source: "start_1", Target: "retrieve_1"},
			{ID: "e2", Source: "retrieve_1", Target: "end_1"},
		},
	}
	result := validator.ValidateDefinition(def, registry.DefaultRegistry())
	if result.Valid || !hasValidationMessage(result, "logical scope instead of resource identifiers") {
		t.Fatalf("expected concrete resource ID rejection, got %#v", result.Errors)
	}
}

func mappedReplyDefinition() dsl.Definition {
	def := minimalDefinition()
	def.Nodes[1].Inputs = map[string]dsl.VariableSelector{
		"replyText": {NodeID: "start_1", Field: "userMessage"},
	}
	return def
}

func conditionDefinition() dsl.Definition {
	conditionConfig, _ := json.Marshal(dsl.ConditionConfig{
		Branches: []dsl.ConditionBranch{
			{
				ID:           "hello",
				Name:         "Hello",
				TargetNodeID: "end_1",
				Condition: &dsl.Condition{
					Left:     &dsl.VariableSelector{NodeID: "start_1", Field: "userMessage"},
					Operator: "eq",
					Right:    "hello",
				},
			},
			{
				ID:           "default",
				Name:         "Default",
				TargetNodeID: "end_1",
				Default:      true,
			},
		},
	})
	return dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: "start"},
			{ID: "condition_1", Type: "condition", Config: conditionConfig},
			{ID: "end_1", Type: "end"},
		},
		Edges: []dsl.Edge{
			{ID: "e1", Source: "start_1", Target: "condition_1"},
			{ID: "e2", Source: "condition_1", Target: "end_1"},
			{ID: "e3", Source: "condition_1", Target: "end_1"},
		},
	}
}

func TestValidateDefinitionRequiresErrorTargetEdge(t *testing.T) {
	def := dsl.Definition{
		SchemaVersion: 1,
		EntryNodeID:   "start_1",
		Nodes: []dsl.Node{
			{ID: "start_1", Type: "start"},
			{ID: "llm_1", Type: "llm_reply", ErrorTargetNodeID: "fallback_end", Inputs: map[string]dsl.VariableSelector{
				"userMessage": {NodeID: "start_1", Field: "userMessage"},
			}},
			{ID: "success_end", Type: "end"},
			{ID: "fallback_end", Type: "end"},
		},
		Edges: []dsl.Edge{
			{ID: "e1", Source: "start_1", Target: "llm_1"},
			{ID: "e2", Source: "llm_1", Target: "success_end"},
		},
	}
	result := validator.ValidateDefinition(def, registry.DefaultRegistry())
	if result.Valid || !hasValidationMessage(result, "error target must have an outgoing edge") {
		t.Fatalf("expected missing error target edge rejection, got %#v", result.Errors)
	}

	def.Edges = append(def.Edges, dsl.Edge{ID: "e3", Source: "llm_1", Target: "fallback_end"})
	result = validator.ValidateDefinition(def, registry.DefaultRegistry())
	if !result.Valid {
		t.Fatalf("expected portable error transition to be valid, got %#v", result.Errors)
	}
}

func TestValidateDefinitionRejectsUnknownServiceAccessMode(t *testing.T) {
	def := minimalDefinition()
	def.Nodes = append(def.Nodes, dsl.Node{
		ID:     "access_1",
		Type:   registry.NodeTypeServiceAccessPolicy,
		Config: json.RawMessage(`{"humanHandoffMode":"product_method"}`),
		Inputs: map[string]dsl.VariableSelector{
			"contextLevel": {NodeID: "start_1", Field: "userMessage"},
			"deviceBound":  {NodeID: "start_1", Field: "userMessage"},
		},
	})
	result := validator.ValidateDefinition(def, registry.DefaultRegistry())
	if result.Valid || !hasValidationMessage(result, "service access mode must be") {
		t.Fatalf("expected unknown service access mode rejection, got %#v", result.Errors)
	}
}

func hasValidationMessage(result validator.Result, want string) bool {
	for _, item := range result.Errors {
		if strings.Contains(item.Message, want) {
			return true
		}
	}
	return false
}

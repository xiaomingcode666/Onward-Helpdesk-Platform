package registry

import "testing"

func TestDefaultRegistryExposesStartOutputs(t *testing.T) {
	spec, ok := DefaultRegistry().Get(NodeTypeStart)
	if !ok {
		t.Fatalf("start node spec not found")
	}
	if !hasVariable(spec.OutputSchema, "userMessage", VariableTypeString) {
		t.Fatalf("expected start output userMessage:string, got %#v", spec.OutputSchema)
	}
	for _, expected := range []struct {
		name     string
		typeName VariableType
	}{
		{name: "tenantId", typeName: VariableTypeInteger},
		{name: "productId", typeName: VariableTypeInteger},
		{name: "agentReleaseId", typeName: VariableTypeInteger},
		{name: "locale", typeName: VariableTypeString},
		{name: "audience", typeName: VariableTypeString},
	} {
		if !hasVariable(spec.OutputSchema, expected.name, expected.typeName) {
			t.Fatalf("expected start output %s:%s, got %#v", expected.name, expected.typeName, spec.OutputSchema)
		}
	}
	if hasVariable(spec.OutputSchema, "knowledgeBaseIds", VariableTypeIntegerArray) {
		t.Fatalf("start output must not expose concrete knowledge base ids: %#v", spec.OutputSchema)
	}
}

func TestDefaultRegistryExposesKnowledgeRetrieveInputsAndOutputs(t *testing.T) {
	spec, ok := DefaultRegistry().Get(NodeTypeKnowledgeRetrieve)
	if !ok {
		t.Fatalf("knowledge_retrieve node spec not found")
	}
	if !hasRequiredVariable(spec.InputSchema, "query", VariableTypeString) {
		t.Fatalf("expected knowledge_retrieve required input query:string, got %#v", spec.InputSchema)
	}
	if !hasVariable(spec.OutputSchema, "items", VariableTypeObjectArray) {
		t.Fatalf("expected knowledge_retrieve output items:array<object>, got %#v", spec.OutputSchema)
	}
}

func TestDefaultRegistryExposesSendReplyRequiredInput(t *testing.T) {
	spec, ok := DefaultRegistry().Get(NodeTypeSendReply)
	if !ok {
		t.Fatalf("send_reply node spec not found")
	}
	if !hasRequiredVariable(spec.InputSchema, "replyText", VariableTypeString) {
		t.Fatalf("expected send_reply required input replyText:string, got %#v", spec.InputSchema)
	}
	if !hasVariable(spec.OutputSchema, "sent", VariableTypeBoolean) {
		t.Fatalf("expected send_reply output sent:boolean, got %#v", spec.OutputSchema)
	}
}

func TestDefaultRegistryExposesAfterSalesToolNodes(t *testing.T) {
	for _, tt := range []struct {
		nodeType string
		inputs   []string
		outputs  []string
	}{
		{
			nodeType: NodeTypeCreateVideoMeeting,
			inputs:   []string{"ticketId", "confirmed"},
			outputs:  []string{"meetingId", "roomName", "created", "message"},
		},
		{
			nodeType: NodeTypeCreateKnowledgeCandidate,
			inputs:   []string{"ticketId", "confirmed"},
			outputs:  []string{"candidateId", "created", "reviewStatus", "message"},
		},
	} {
		spec, ok := DefaultRegistry().Get(tt.nodeType)
		if !ok {
			t.Fatalf("%s node spec not found", tt.nodeType)
		}
		if spec.RiskLevel != NodeRiskLevelHigh || !spec.RequiresConfirmationPredecessor {
			t.Fatalf("%s must be high-risk and confirmation guarded, got %#v", tt.nodeType, spec)
		}
		for _, input := range tt.inputs {
			if !hasVariableName(spec.InputSchema, input) {
				t.Fatalf("expected %s input %s, got %#v", tt.nodeType, input, spec.InputSchema)
			}
		}
		for _, output := range tt.outputs {
			if !hasVariableName(spec.OutputSchema, output) {
				t.Fatalf("expected %s output %s, got %#v", tt.nodeType, output, spec.OutputSchema)
			}
		}
	}
}

func TestDefaultRegistryExposesConversationUnderstandingOutputs(t *testing.T) {
	spec, ok := DefaultRegistry().Get(NodeTypeConversationUnderstanding)
	if !ok {
		t.Fatalf("conversation_understanding node spec not found")
	}
	if !hasRequiredVariable(spec.InputSchema, "userMessage", VariableTypeString) {
		t.Fatalf("expected conversation_understanding required input userMessage:string, got %#v", spec.InputSchema)
	}
	for _, want := range []string{"messageIntent", "answerScope", "riskSignals", "reason"} {
		if !hasVariableName(spec.OutputSchema, want) {
			t.Fatalf("expected conversation_understanding output %s, got %#v", want, spec.OutputSchema)
		}
	}
	if !hasVariable(spec.OutputSchema, "confidence", VariableTypeNumber) {
		t.Fatalf("expected conversation_understanding output confidence:number, got %#v", spec.OutputSchema)
	}
}

func TestDefaultRegistryExposesReplyPolicyOutputs(t *testing.T) {
	spec, ok := DefaultRegistry().Get(NodeTypeReplyPolicy)
	if !ok {
		t.Fatalf("reply_policy node spec not found")
	}
	if !hasRequiredVariable(spec.InputSchema, "messageIntent", VariableTypeString) {
		t.Fatalf("expected reply_policy required input messageIntent:string, got %#v", spec.InputSchema)
	}
	if !hasRequiredVariable(spec.InputSchema, "answerScope", VariableTypeString) {
		t.Fatalf("expected reply_policy required input answerScope:string, got %#v", spec.InputSchema)
	}
	for _, want := range []string{"action", "replyText", "reason", "finalReplySource"} {
		if !hasVariableName(spec.OutputSchema, want) {
			t.Fatalf("expected reply_policy output %s, got %#v", want, spec.OutputSchema)
		}
	}
}

func hasRequiredVariable(items []VariableSpec, name string, variableType VariableType) bool {
	for _, item := range items {
		if item.Name == name && item.Type == variableType && item.Required {
			return true
		}
	}
	return false
}

func hasVariableName(items []VariableSpec, name string) bool {
	for _, item := range items {
		if item.Name == name {
			return true
		}
	}
	return false
}

func hasVariable(items []VariableSpec, name string, variableType VariableType) bool {
	for _, item := range items {
		if item.Name == name && item.Type == variableType {
			return true
		}
	}
	return false
}

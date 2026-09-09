package migration

import (
	"testing"

	"remotehelpdesk/internal/ai/workflow/dsl"
)

func TestKnowledgeSupportDefinitionCarriesCitations(t *testing.T) {
	definition := dsl.Definition{Nodes: []dsl.Node{
		{ID: "tenant_retrieve_1", Type: "knowledge_retrieve"},
		{ID: "send_reply_1", Type: "send_reply", Inputs: map[string]dsl.VariableSelector{
			"citations": {NodeID: "tenant_retrieve_1", Field: "citations"},
		}},
	}}
	if !knowledgeSupportDefinitionCarriesCitations(definition) {
		t.Fatal("expected the knowledge reply to carry citations")
	}
	definition.Nodes[1].Inputs = nil
	if knowledgeSupportDefinitionCarriesCitations(definition) {
		t.Fatal("reply without a citation selector must not pass validation")
	}
}

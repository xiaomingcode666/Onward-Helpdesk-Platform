package compiler

import (
	"testing"

	"remotehelpdesk/internal/ai/workflow/dsl"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
	"remotehelpdesk/internal/pkg/toolx"
)

func TestCompileMapsWorkflowNodesToGraphTools(t *testing.T) {
	result := Compile(dsl.Definition{
		EntryNodeID: "start",
		Nodes: []dsl.Node{
			{ID: "start", Type: workflowregistry.NodeTypeStart, Name: "Start"},
			{ID: "analyze", Type: workflowregistry.NodeTypeAnalyzeConversation, Name: "Analyze"},
			{ID: "draft", Type: workflowregistry.NodeTypePrepareTicketDraft, Name: "Draft"},
			{ID: "create", Type: workflowregistry.NodeTypeCreateTicket, Name: "Create"},
			{ID: "handoff", Type: workflowregistry.NodeTypeHandoffToHuman, Name: "Handoff"},
			{ID: "meeting", Type: workflowregistry.NodeTypeCreateVideoMeeting, Name: "Meeting"},
			{ID: "knowledge", Type: workflowregistry.NodeTypeCreateKnowledgeCandidate, Name: "Knowledge"},
		},
	})
	want := []string{
		toolx.GraphAnalyzeConversation.Code,
		toolx.GraphPrepareTicketDraft.Code,
		toolx.GraphCreateTicketConfirm.Code,
		toolx.GraphHandoffConversation.Code,
		toolx.GraphCreateVideoMeeting.Code,
		toolx.GraphCreateKnowledgeCandidate.Code,
	}
	if len(result.ToolCodes) != len(want) {
		t.Fatalf("expected %d tool codes, got %d: %#v", len(want), len(result.ToolCodes), result.ToolCodes)
	}
	for i, item := range want {
		if result.ToolCodes[i] != item {
			t.Fatalf("tool code[%d] = %s, want %s", i, result.ToolCodes[i], item)
		}
	}
	if result.Appendix == "" {
		t.Fatalf("expected workflow appendix")
	}
}

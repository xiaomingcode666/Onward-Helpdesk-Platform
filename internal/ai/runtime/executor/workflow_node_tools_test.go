package executor

import (
	"context"
	"testing"

	"remotehelpdesk/internal/ai/runtime/registry"
	"remotehelpdesk/internal/ai/runtime/tooling"
	"remotehelpdesk/internal/pkg/toolx"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type workflowNodeStubTool struct {
	name string
}

func (t workflowNodeStubTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: t.name}, nil
}

func TestFilterWorkflowNodeToolSetBlocksWorkflowOwnedSideEffects(t *testing.T) {
	toolSet := &registry.ToolSet{
		StaticTools: []einotool.BaseTool{
			workflowNodeStubTool{name: toolx.GraphCreateTicketConfirm.Name},
			workflowNodeStubTool{name: toolx.GraphHandoffConversation.Name},
			workflowNodeStubTool{name: toolx.GraphTriageServiceRequest.Name},
		},
		StaticToolCodes: map[string]string{
			toolx.GraphCreateTicketConfirm.Name:  toolx.GraphCreateTicketConfirm.Code,
			toolx.GraphHandoffConversation.Name:  toolx.GraphHandoffConversation.Code,
			toolx.GraphTriageServiceRequest.Name: toolx.GraphTriageServiceRequest.Code,
		},
		StaticToolMetadata: map[string]registry.ToolMetadata{
			toolx.GraphCreateTicketConfirm.Name:  {ToolCode: toolx.GraphCreateTicketConfirm.Code},
			toolx.GraphHandoffConversation.Name:  {ToolCode: toolx.GraphHandoffConversation.Code},
			toolx.GraphTriageServiceRequest.Name: {ToolCode: toolx.GraphTriageServiceRequest.Code},
		},
	}

	filtered, err := filterWorkflowNodeToolSet(context.Background(), toolSet, []string{
		toolx.GraphCreateTicketConfirm.Code,
		toolx.GraphHandoffConversation.Code,
	})
	if err != nil {
		t.Fatalf("filter workflow node ToolSet: %v", err)
	}
	if len(filtered.StaticTools) != 1 {
		t.Fatalf("expected one non-side-effect tool, got %d", len(filtered.StaticTools))
	}
	if _, ok := filtered.StaticToolCodes[toolx.GraphTriageServiceRequest.Name]; !ok {
		t.Fatalf("expected triage tool to remain available")
	}
	if _, ok := filtered.StaticToolCodes[toolx.GraphCreateTicketConfirm.Name]; ok {
		t.Fatalf("create-ticket tool must remain workflow-owned")
	}
	if _, ok := filtered.StaticToolCodes[toolx.GraphHandoffConversation.Name]; ok {
		t.Fatalf("handoff tool must remain workflow-owned")
	}
}

func TestFilterWorkflowNodeToolDefinitionsBlocksAliases(t *testing.T) {
	filtered := filterWorkflowNodeToolDefinitions([]tooling.MCPToolDefinition{
		{ToolCode: "builtin/create_ticket_with_confirmation"},
		{ToolCode: "mcp/manual/search"},
	}, []string{toolx.GraphCreateTicketConfirm.Code})
	if len(filtered) != 1 || filtered[0].ToolCode != "mcp/manual/search" {
		t.Fatalf("unexpected filtered definitions: %#v", filtered)
	}
}

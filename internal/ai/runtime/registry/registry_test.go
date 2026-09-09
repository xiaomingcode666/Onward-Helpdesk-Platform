package registry_test

import (
	"context"
	"testing"

	"remotehelpdesk/internal/ai/runtime/registry"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/toolx"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type stubTool struct {
	name string
	code string
}

func (t stubTool) Spec() toolx.ToolSpec {
	return toolx.ToolSpec{
		Code:       t.code,
		Name:       t.name,
		ServerCode: toolx.GraphCreateTicketConfirm.ServerCode,
		SourceType: toolx.GraphCreateTicketConfirm.SourceType,
	}
}

func (t stubTool) Name() string { return t.name }
func (t stubTool) Code() string { return t.code }
func (t stubTool) Enabled(registry.Context) bool {
	return true
}
func (t stubTool) Build(registry.Context) (einotool.BaseTool, error) {
	return stubBaseTool{name: t.name}, nil
}

type stubBaseTool struct {
	name string
}

func (t stubBaseTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: t.name}, nil
}

func TestResolveBuildsStaticToolMetadata(t *testing.T) {
	r := registry.NewRegistry(stubTool{
		name: toolx.GraphCreateTicketConfirm.Name,
		code: toolx.GraphCreateTicketConfirm.Code,
	})
	toolSet, err := r.Resolve(registry.Context{
		Conversation: models.Conversation{ID: 1},
		AIAgent:      models.AIAgent{ID: 1},
	})
	if err != nil {
		t.Fatalf("resolve returned error: %v", err)
	}
	if toolSet == nil {
		t.Fatalf("expected tool set")
	}
	if len(toolSet.StaticToolMetadata) != 1 {
		t.Fatalf("expected 1 metadata item, got %d", len(toolSet.StaticToolMetadata))
	}
	item, ok := toolSet.StaticToolMetadata[toolx.GraphCreateTicketConfirm.Name]
	if !ok {
		t.Fatalf("missing metadata for %s", toolx.GraphCreateTicketConfirm.Name)
	}
	if item.ToolCode != toolx.GraphCreateTicketConfirm.Code {
		t.Fatalf("unexpected tool code: %s", item.ToolCode)
	}
	if item.ServerCode != toolx.GraphCreateTicketConfirm.ServerCode {
		t.Fatalf("unexpected server code: %s", item.ServerCode)
	}
	if item.ToolName != toolx.GraphCreateTicketConfirm.Name {
		t.Fatalf("unexpected tool name: %s", item.ToolName)
	}
	if item.SourceType != toolx.GraphCreateTicketConfirm.SourceType {
		t.Fatalf("unexpected source type: %s", item.SourceType)
	}
}

func TestResolveEnforcedEmptyAllowlistDeniesSideEffectTool(t *testing.T) {
	r := registry.NewRegistry(
		stubTool{name: toolx.GraphCreateTicketConfirm.Name, code: toolx.GraphCreateTicketConfirm.Code},
		stubTool{name: toolx.GraphHandoffConversation.Name, code: toolx.GraphHandoffConversation.Code},
	)
	toolSet, err := r.Resolve(registry.Context{EnforceAllowedToolCodes: true})
	if err != nil {
		t.Fatalf("resolve returned error: %v", err)
	}
	if len(toolSet.StaticTools) != 1 {
		t.Fatalf("expected only always-available handoff tool, got %d", len(toolSet.StaticTools))
	}
	if _, ok := toolSet.StaticToolCodes[toolx.GraphCreateTicketConfirm.Name]; ok {
		t.Fatal("create-ticket tool must not be exposed without an explicit agent allowlist")
	}
	if got := toolSet.StaticToolCodes[toolx.GraphHandoffConversation.Name]; got != toolx.GraphHandoffConversation.Code {
		t.Fatalf("expected handoff tool to remain available, got %q", got)
	}
}

func TestResolveEnforcedAllowlistIncludesRequiredDependencies(t *testing.T) {
	r := registry.NewRegistry(
		stubTool{name: toolx.GraphTriageServiceRequest.Name, code: toolx.GraphTriageServiceRequest.Code},
		stubTool{name: toolx.GraphPrepareTicketDraft.Name, code: toolx.GraphPrepareTicketDraft.Code},
		stubTool{name: toolx.GraphCreateTicketConfirm.Name, code: toolx.GraphCreateTicketConfirm.Code},
		stubTool{name: toolx.GraphHandoffConversation.Name, code: toolx.GraphHandoffConversation.Code},
	)
	toolSet, err := r.Resolve(registry.Context{
		AllowedToolCodes:        []string{toolx.GraphCreateTicketConfirm.Code},
		EnforceAllowedToolCodes: true,
	})
	if err != nil {
		t.Fatalf("resolve returned error: %v", err)
	}
	if len(toolSet.StaticTools) != 4 {
		t.Fatalf("expected ticket dependencies and handoff tool, got %d", len(toolSet.StaticTools))
	}
}

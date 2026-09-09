package factory

import (
	"context"
	"testing"

	runtimetooling "remotehelpdesk/internal/ai/runtime/tooling"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
)

func TestBuildMCPToolsSkipsGraphAndBuiltinTools(t *testing.T) {
	aiAgent := models.AIAgent{
		AllowedMCPTools: `[
			{"toolCode":"graph/create_ticket_with_confirmation","serverCode":"graph","toolName":"create_ticket_with_confirmation"},
			{"toolCode":"builtin/tool_search","serverCode":"builtin","toolName":"tool_search"},
			{"toolCode":"system/list_agents","serverCode":"system","toolName":"list_agents"}
		]`,
	}

	got, err := NewToolFactory().BuildMCPTools(aiAgent)
	if err != nil {
		t.Fatalf("BuildMCPTools returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 dynamic mcp tool, got %d: %#v", len(got), got)
	}
	if got[0].ToolCode != "system/list_agents" {
		t.Fatalf("unexpected tool code: %#v", got[0])
	}
	if got[0].ServerCode != "system" || got[0].ToolName != "list_agents" {
		t.Fatalf("unexpected tool identity: %#v", got[0])
	}
}

func TestBuildAvailableMCPToolsDegradesUnavailableServer(t *testing.T) {
	previous := config.CurrentOrDefault()
	config.SetCurrent(&config.Config{MCP: config.MCPConfig{
		Enabled: true,
		Servers: map[string]config.MCPServerConfig{
			"offline": {
				Enabled:   true,
				Endpoint:  "http://127.0.0.1:1/api/mcp",
				TimeoutMS: 50,
			},
		},
	}})
	t.Cleanup(func() { config.SetCurrent(&previous) })

	definitions := []runtimetooling.MCPToolDefinition{{
		ToolCode:   "offline/inventory",
		ServerCode: "offline",
		ToolName:   "inventory",
		ModelName:  "offline_inventory",
	}}
	result := NewToolFactory().BuildAvailableBaseToolsByDefinitions(context.Background(), definitions)
	if len(result.Tools) != 0 || len(result.Definitions) != 0 {
		t.Fatalf("unavailable MCP tools must not be exposed: %#v", result)
	}
	if len(result.Issues) != 1 || result.Issues[0].Definition.ToolCode != "offline/inventory" {
		t.Fatalf("expected one auditable degradation issue, got %#v", result.Issues)
	}
	if _, err := NewToolFactory().BuildBaseToolsByDefinitions(context.Background(), definitions); err == nil {
		t.Fatal("strict MCP tool build must still report unavailable servers")
	}
}

package factory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"remotehelpdesk/internal/ai/mcps"
	impladapter "remotehelpdesk/internal/ai/runtime/internal/impl/adapter"
	runtimetooling "remotehelpdesk/internal/ai/runtime/tooling"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/toolx"

	einotool "github.com/cloudwego/eino/components/tool"
)

type ToolFactory struct{}

type MCPToolBuildIssue struct {
	Definition runtimetooling.MCPToolDefinition
	Err        error
}

type MCPToolBuildResult struct {
	Tools       []einotool.BaseTool
	Definitions []runtimetooling.MCPToolDefinition
	Issues      []MCPToolBuildIssue
}

func NewToolFactory() *ToolFactory {
	return &ToolFactory{}
}

func (f *ToolFactory) BuildMCPTools(aiAgent models.AIAgent) ([]runtimetooling.MCPToolDefinition, error) {
	raw, err := toolx.ParseAgentMCPToolsJSON(aiAgent.AllowedMCPTools)
	if err != nil {
		return nil, err
	}
	ret := make([]runtimetooling.MCPToolDefinition, 0, len(raw))
	for _, item := range raw {
		toolCode := strings.TrimSpace(item.ToolCode)
		toolCode = toolx.NormalizeToolCodeAlias(toolCode)
		if toolCode == "" {
			toolCode = toolx.BuildMCPToolCode(item.ServerCode, item.ToolName)
		}
		if toolx.ResolveToolSourceType(toolCode) != enums.ToolSourceTypeMCP {
			continue
		}
		serverCode, toolName := toolx.SplitMCPToolCode(toolCode)
		if serverCode == "" || toolName == "" {
			continue
		}
		definition := runtimetooling.MCPToolDefinition{
			ToolCode:    toolCode,
			ServerCode:  serverCode,
			ToolName:    toolName,
			Title:       strings.TrimSpace(item.Title),
			Description: strings.TrimSpace(item.Description),
			FixedArgs:   cloneStringMap(item.Arguments),
		}
		definition.ModelName = runtimetooling.BuildModelToolName(definition)
		ret = append(ret, definition)
	}
	return ret, nil
}

func (f *ToolFactory) BuildBaseTools(ctx context.Context, aiAgent models.AIAgent) ([]einotool.BaseTool, error) {
	definitions, err := f.BuildMCPTools(aiAgent)
	if err != nil {
		return nil, err
	}
	return f.BuildBaseToolsByDefinitions(ctx, definitions)
}

func (f *ToolFactory) BuildBaseToolsByDefinitions(ctx context.Context, definitions []runtimetooling.MCPToolDefinition) ([]einotool.BaseTool, error) {
	result := f.BuildAvailableBaseToolsByDefinitions(ctx, definitions)
	if len(result.Issues) == 0 {
		return result.Tools, nil
	}
	errs := make([]error, 0, len(result.Issues))
	for _, issue := range result.Issues {
		errs = append(errs, issue.Err)
	}
	return nil, errors.Join(errs...)
}

// BuildAvailableBaseToolsByDefinitions isolates MCP failures by server. An
// unavailable optional connector is omitted from the current run instead of
// preventing the customer-service agent from answering with its remaining
// capabilities.
func (f *ToolFactory) BuildAvailableBaseToolsByDefinitions(ctx context.Context, definitions []runtimetooling.MCPToolDefinition) MCPToolBuildResult {
	result := MCPToolBuildResult{
		Tools:       make([]einotool.BaseTool, 0, len(definitions)),
		Definitions: make([]runtimetooling.MCPToolDefinition, 0, len(definitions)),
		Issues:      make([]MCPToolBuildIssue, 0),
	}
	if len(definitions) == 0 {
		return result
	}
	metadataByCode, serverErrors := f.loadAvailableToolMetadata(ctx, definitions)
	for _, item := range definitions {
		if err := serverErrors[item.ServerCode]; err != nil {
			result.Issues = append(result.Issues, MCPToolBuildIssue{
				Definition: item,
				Err:        fmt.Errorf("load MCP tool %s: %w", item.ToolCode, err),
			})
			continue
		}
		metadata, ok := metadataByCode[item.ToolCode]
		if !ok {
			result.Issues = append(result.Issues, MCPToolBuildIssue{
				Definition: item,
				Err:        fmt.Errorf("MCP tool %s is not advertised by server %s", item.ToolCode, item.ServerCode),
			})
			continue
		}
		result.Tools = append(result.Tools, impladapter.NewMCPTool(item, metadata))
		result.Definitions = append(result.Definitions, item)
	}
	return result
}

func (f *ToolFactory) loadAvailableToolMetadata(ctx context.Context, definitions []runtimetooling.MCPToolDefinition) (map[string]*mcps.ToolInfo, map[string]error) {
	toolsByCode := make(map[string]*mcps.ToolInfo, len(definitions))
	serverCodes := make([]string, 0)
	seenServerCodes := make(map[string]struct{})
	for _, item := range definitions {
		if _, ok := seenServerCodes[item.ServerCode]; ok {
			continue
		}
		seenServerCodes[item.ServerCode] = struct{}{}
		serverCodes = append(serverCodes, item.ServerCode)
	}
	serverErrors := make(map[string]error)
	for serverCode := range serverCodes {
		serverCode := serverCodes[serverCode]
		toolInfos, err := mcps.Runtime.ListTools(ctx, serverCode)
		if err != nil {
			serverErrors[serverCode] = err
			continue
		}
		for i := range toolInfos {
			toolInfo := toolInfos[i]
			toolCode := toolx.BuildMCPToolCode(serverCode, toolInfo.Name)
			toolInfoCopy := toolInfo
			toolsByCode[toolCode] = &toolInfoCopy
		}
	}
	return toolsByCode, serverErrors
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	ret := make(map[string]string, len(input))
	for key, value := range input {
		ret[key] = value
	}
	return ret
}

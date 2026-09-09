package executor

import (
	"context"
	"fmt"
	"strings"

	"remotehelpdesk/internal/ai/runtime/registry"
	runtimetooling "remotehelpdesk/internal/ai/runtime/tooling"
	"remotehelpdesk/internal/pkg/toolx"

	einotool "github.com/cloudwego/eino/components/tool"
)

func filterWorkflowNodeToolSet(ctx context.Context, toolSet *registry.ToolSet, blockedToolCodes []string) (*registry.ToolSet, error) {
	if toolSet == nil || len(blockedToolCodes) == 0 {
		return toolSet, nil
	}
	blocked := workflowNodeToolCodeSet(blockedToolCodes)
	ret := &registry.ToolSet{
		StaticTools:        make([]einotool.BaseTool, 0, len(toolSet.StaticTools)),
		StaticToolCodes:    make(map[string]string),
		StaticToolMetadata: make(map[string]registry.ToolMetadata),
	}
	for _, tool := range toolSet.StaticTools {
		if tool == nil {
			continue
		}
		info, err := tool.Info(ctx)
		if err != nil {
			return nil, err
		}
		name := ""
		if info != nil {
			name = strings.TrimSpace(info.Name)
		}
		if name == "" {
			return nil, fmt.Errorf("workflow node tool is missing a model-visible name")
		}
		code := toolx.NormalizeToolCodeAlias(strings.TrimSpace(toolSet.StaticToolCodes[name]))
		if code == "" {
			code = toolx.NormalizeToolCodeAlias(strings.TrimSpace(toolSet.StaticToolMetadata[name].ToolCode))
		}
		if code == "" {
			return nil, fmt.Errorf("workflow node tool %q is missing a registered tool code", name)
		}
		if _, denied := blocked[code]; denied {
			continue
		}
		ret.StaticTools = append(ret.StaticTools, tool)
	}
	for name, code := range toolSet.StaticToolCodes {
		normalized := toolx.NormalizeToolCodeAlias(strings.TrimSpace(code))
		if _, denied := blocked[normalized]; denied {
			continue
		}
		ret.StaticToolCodes[name] = code
	}
	for name, metadata := range toolSet.StaticToolMetadata {
		normalized := toolx.NormalizeToolCodeAlias(strings.TrimSpace(metadata.ToolCode))
		if _, denied := blocked[normalized]; denied {
			continue
		}
		ret.StaticToolMetadata[name] = metadata
	}
	return ret, nil
}

func filterWorkflowNodeToolDefinitions(definitions []runtimetooling.MCPToolDefinition, blockedToolCodes []string) []runtimetooling.MCPToolDefinition {
	if len(definitions) == 0 || len(blockedToolCodes) == 0 {
		return definitions
	}
	blocked := workflowNodeToolCodeSet(blockedToolCodes)
	ret := make([]runtimetooling.MCPToolDefinition, 0, len(definitions))
	for _, definition := range definitions {
		code := toolx.NormalizeToolCodeAlias(strings.TrimSpace(definition.ToolCode))
		if _, denied := blocked[code]; denied {
			continue
		}
		ret = append(ret, definition)
	}
	return ret
}

func workflowNodeToolCodeSet(toolCodes []string) map[string]struct{} {
	ret := make(map[string]struct{}, len(toolCodes))
	for _, toolCode := range toolCodes {
		toolCode = toolx.NormalizeToolCodeAlias(strings.TrimSpace(toolCode))
		if toolCode != "" {
			ret[toolCode] = struct{}{}
		}
	}
	return ret
}

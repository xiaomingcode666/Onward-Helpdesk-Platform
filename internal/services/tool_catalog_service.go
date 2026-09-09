package services

import (
	"context"
	"slices"
	"strings"

	"remotehelpdesk/internal/ai/mcps"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/pkg/toolx"
)

var ToolCatalogService = newToolCatalogService()

func newToolCatalogService() *toolCatalogService {
	return &toolCatalogService{}
}

type toolCatalogService struct{}

type MCPToolCatalogItem struct {
	ToolCode     string
	ServerCode   string
	ToolName     string
	SourceType   enums.ToolSourceType
	AutoInjected bool
	Title        string
	Description  string
	InputSchema  any
	OutputSchema any
}

func (s *toolCatalogService) ListMCPTools(ctx context.Context) ([]MCPToolCatalogItem, error) {
	return s.ListMCPToolsWithLocale(ctx, i18nx.DefaultLocale)
}

func (s *toolCatalogService) ListMCPToolsWithLocale(ctx context.Context, locale string) ([]MCPToolCatalogItem, error) {
	cfg := config.Current()
	ret := make([]MCPToolCatalogItem, 0, 3)
	for _, spec := range toolx.ListAgentDirectToolSpecs() {
		if spec.Code == toolx.BuiltinToolSearch.Code && !cfg.MCP.Enabled {
			continue
		}
		ret = append(ret, MCPToolCatalogItem{
			ToolCode:     spec.Code,
			ServerCode:   spec.ServerCode,
			ToolName:     spec.Name,
			SourceType:   spec.SourceType,
			AutoInjected: spec.AutoInjected,
			Title:        toolx.GetRegisteredToolTitleLocale(spec.Code, locale),
			Description:  toolx.GetRegisteredToolDescriptionLocale(spec.Code, locale),
		})
	}
	if !cfg.MCP.Enabled {
		return ret, nil
	}
	serverCodes := make([]string, 0, len(cfg.MCP.Servers))
	for serverCode, server := range cfg.MCP.Servers {
		if !server.Enabled {
			continue
		}
		serverCodes = append(serverCodes, serverCode)
	}
	slices.Sort(serverCodes)
	for _, serverCode := range serverCodes {
		tools, err := mcps.Runtime.ListTools(ctx, serverCode)
		if err != nil {
			return nil, err
		}
		for _, item := range tools {
			ret = append(ret, MCPToolCatalogItem{
				ToolCode:     toolx.BuildMCPToolCode(serverCode, item.Name),
				ServerCode:   serverCode,
				ToolName:     strings.TrimSpace(item.Name),
				SourceType:   enums.ToolSourceTypeMCP,
				AutoInjected: false,
				Title:        strings.TrimSpace(item.Title),
				Description:  strings.TrimSpace(item.Description),
				InputSchema:  item.InputSchema,
				OutputSchema: item.OutputSchema,
			})
		}
	}
	return ret, nil
}

func (s *toolCatalogService) ValidateMCPToolCode(toolCode string) error {
	return s.ValidateToolCode(toolCode)
}

func (s *toolCatalogService) ValidateToolCode(toolCode string) error {
	cfg := config.Current()
	toolCode = strings.TrimSpace(toolCode)
	if toolCode == "" {
		return errorsx.InvalidParamI18n("error.e0074")
	}
	if toolx.IsAgentDirectToolCode(toolCode) {
		return nil
	}
	serverCode, toolName := toolx.SplitMCPToolCode(toolCode)
	if serverCode == "" || toolName == "" {
		return errorsx.InvalidParamI18n("error.e0075")
	}
	if !cfg.MCP.Enabled {
		return errorsx.InvalidParamI18n("error.e0035")
	}
	server, ok := cfg.MCP.Servers[serverCode]
	if !ok || !server.Enabled {
		return errorsx.InvalidParamI18n("error.e0073")
	}
	return nil
}

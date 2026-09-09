package factory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	einocallbacks "remotehelpdesk/internal/ai/runtime/internal/impl/callbacks"
	"remotehelpdesk/internal/pkg/toolx"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const activeSkillRunLocalKey = "runtime_active_skill_id"

type RuntimeToolFilterMiddleware struct {
	*adk.BaseChatModelAgentMiddleware
	collector          *einocallbacks.RuntimeTraceCollector
	toolMetadataByName map[string]einocallbacks.ToolMetadata
	skillMetadataBy    map[string]einocallbacks.SkillMetadata
	dynamicToolNames   []string
}

func NewRuntimeToolFilterMiddleware(
	collector *einocallbacks.RuntimeTraceCollector,
	toolMetadataByName map[string]einocallbacks.ToolMetadata,
	skillMetadataBy map[string]einocallbacks.SkillMetadata,
	dynamicToolNames []string,
) *RuntimeToolFilterMiddleware {
	return &RuntimeToolFilterMiddleware{
		BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
		collector:                    collector,
		toolMetadataByName:           cloneToolMetadataMap(toolMetadataByName),
		skillMetadataBy:              cloneSkillMetadataMap(skillMetadataBy),
		dynamicToolNames:             append([]string(nil), dynamicToolNames...),
	}
}

func (m *RuntimeToolFilterMiddleware) WrapModel(_ context.Context, cm model.BaseChatModel, mc *adk.ModelContext) (model.BaseChatModel, error) {
	if mc == nil {
		return cm, nil
	}
	return &runtimeToolFilterModelWrapper{
		cm:                 cm,
		allTools:           append([]*schema.ToolInfo(nil), mc.Tools...),
		collector:          m.collector,
		toolMetadataByName: m.toolMetadataByName,
		skillMetadataBy:    m.skillMetadataBy,
		dynamicToolNames:   append([]string(nil), m.dynamicToolNames...),
	}, nil
}

func (m *RuntimeToolFilterMiddleware) WrapInvokableToolCall(_ context.Context, endpoint adk.InvokableToolCallEndpoint, tCtx *adk.ToolContext) (adk.InvokableToolCallEndpoint, error) {
	return func(ctx context.Context, argumentsInJSON string, opts ...einotool.Option) (string, error) {
		toolName := ""
		if tCtx != nil {
			toolName = strings.TrimSpace(tCtx.Name)
		}
		metadata, _ := resolveRuntimeToolMetadata(toolName, m.toolMetadataByName)
		if !isRuntimeToolAlwaysAllowedForSkill(metadata.ToolCode) {
			activeSkill, restricted := m.resolveActiveSkill(ctx)
			if restricted && !isToolCodeAllowedForSkill(metadata.ToolCode, activeSkill.AllowedToolCodes) {
				return "", m.blockToolCall(metadata, argumentsInJSON, activeSkill)
			}
		}
		result, err := endpoint(ctx, argumentsInJSON, opts...)
		if err != nil {
			return result, err
		}
		if strings.TrimSpace(metadata.ToolCode) == toolx.BuiltinSkill.Code {
			_ = m.setActiveSkill(ctx, skillIDFromArguments(argumentsInJSON))
			return result, nil
		}
		if strings.TrimSpace(metadata.ToolCode) == toolx.BuiltinToolSearch.Code {
			activeSkill, restricted := m.resolveActiveSkill(ctx)
			if !restricted {
				return result, nil
			}
			filtered, filterErr := filterToolSearchResult(result, activeSkill.AllowedToolCodes, m.toolMetadataByName)
			if filterErr == nil {
				return filtered, nil
			}
		}
		return result, nil
	}, nil
}

func (m *RuntimeToolFilterMiddleware) blockToolCall(metadata einocallbacks.ToolMetadata, argumentsInJSON string, activeSkill einocallbacks.SkillMetadata) error {
	err := fmt.Errorf("tool %s is not allowed for active skill %d", strings.TrimSpace(metadata.ToolCode), activeSkill.ID)
	if m.collector != nil {
		m.collector.AddToolItem(einocallbacks.ToolTraceItem{
			ToolCode:      strings.TrimSpace(metadata.ToolCode),
			ServerCode:    strings.TrimSpace(metadata.ServerCode),
			ToolName:      strings.TrimSpace(metadata.ToolName),
			Arguments:     parseRuntimeToolArguments(argumentsInJSON),
			Status:        "error",
			ErrorMessage:  err.Error(),
			Blocked:       true,
			BlockedReason: "skill_tool_not_allowed",
		})
	}
	return err
}

func (m *RuntimeToolFilterMiddleware) setActiveSkill(ctx context.Context, skillID string) error {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return nil
	}
	return adk.SetRunLocalValue(ctx, activeSkillRunLocalKey, skillID)
}

func (m *RuntimeToolFilterMiddleware) resolveActiveSkill(ctx context.Context) (einocallbacks.SkillMetadata, bool) {
	if len(m.skillMetadataBy) == 0 {
		return einocallbacks.SkillMetadata{}, false
	}
	value, found, err := adk.GetRunLocalValue(ctx, activeSkillRunLocalKey)
	if err != nil || !found {
		return einocallbacks.SkillMetadata{}, false
	}
	skillID, ok := value.(string)
	if !ok {
		return einocallbacks.SkillMetadata{}, false
	}
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return einocallbacks.SkillMetadata{}, false
	}
	skill, ok := m.skillMetadataBy[skillID]
	if !ok {
		return einocallbacks.SkillMetadata{}, false
	}
	return skill, true
}

type runtimeToolFilterModelWrapper struct {
	cm                 model.BaseChatModel
	allTools           []*schema.ToolInfo
	collector          *einocallbacks.RuntimeTraceCollector
	toolMetadataByName map[string]einocallbacks.ToolMetadata
	skillMetadataBy    map[string]einocallbacks.SkillMetadata
	dynamicToolNames   []string
}

func (w *runtimeToolFilterModelWrapper) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	tools := w.filteredTools(ctx, input)
	return w.cm.Generate(ctx, input, append(opts, model.WithTools(tools))...)
}

func (w *runtimeToolFilterModelWrapper) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	tools := w.filteredTools(ctx, input)
	return w.cm.Stream(ctx, input, append(opts, model.WithTools(tools))...)
}

func (w *runtimeToolFilterModelWrapper) filteredTools(ctx context.Context, input []*schema.Message) []*schema.ToolInfo {
	tools := filterDynamicToolInfos(w.allTools, w.dynamicToolNames, input)
	activeSkill, restricted := resolveActiveSkillMetadata(ctx, w.skillMetadataBy)
	if restricted {
		tools = filterToolInfosBySkill(tools, w.toolMetadataByName, activeSkill.AllowedToolCodes)
	}
	if w.collector != nil {
		w.collector.SetFilteredToolCodes(extractToolCodesFromInfos(tools, w.toolMetadataByName))
	}
	return tools
}

func resolveActiveSkillMetadata(ctx context.Context, skills map[string]einocallbacks.SkillMetadata) (einocallbacks.SkillMetadata, bool) {
	if len(skills) == 0 {
		return einocallbacks.SkillMetadata{}, false
	}
	value, found, err := adk.GetRunLocalValue(ctx, activeSkillRunLocalKey)
	if err != nil || !found {
		return einocallbacks.SkillMetadata{}, false
	}
	skillID, ok := value.(string)
	if !ok {
		return einocallbacks.SkillMetadata{}, false
	}
	skillID = strings.TrimSpace(skillID)
	skill, ok := skills[skillID]
	if !ok {
		return skill, false
	}
	return skill, true
}

func filterDynamicToolInfos(allTools []*schema.ToolInfo, dynamicToolNames []string, messages []*schema.Message) []*schema.ToolInfo {
	if len(allTools) == 0 {
		return nil
	}
	selectedToolNames := extractSelectedDynamicToolNames(messages)
	if len(dynamicToolNames) == 0 {
		return append([]*schema.ToolInfo(nil), allTools...)
	}
	removeMap := invertStringSelection(dynamicToolNames, selectedToolNames)
	ret := make([]*schema.ToolInfo, 0, len(allTools))
	for _, info := range allTools {
		if info == nil {
			continue
		}
		if _, ok := removeMap[strings.TrimSpace(info.Name)]; ok {
			continue
		}
		ret = append(ret, info)
	}
	return ret
}

func extractSelectedDynamicToolNames(messages []*schema.Message) []string {
	if len(messages) == 0 {
		return nil
	}
	selected := make([]string, 0)
	for _, message := range messages {
		if message == nil || message.Role != schema.Tool || strings.TrimSpace(message.ToolName) != toolx.BuiltinToolSearch.Name {
			continue
		}
		var payload struct {
			SelectedTools []string `json:"selectedTools"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(message.Content)), &payload); err != nil {
			continue
		}
		for _, item := range payload.SelectedTools {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			selected = append(selected, item)
		}
	}
	return selected
}

func invertStringSelection(all []string, selected []string) map[string]struct{} {
	selectedSet := make(map[string]struct{}, len(selected))
	for _, item := range selected {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		selectedSet[item] = struct{}{}
	}
	ret := make(map[string]struct{})
	for _, item := range all {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := selectedSet[item]; ok {
			continue
		}
		ret[item] = struct{}{}
	}
	return ret
}

func filterToolInfosBySkill(allTools []*schema.ToolInfo, toolMetadataByName map[string]einocallbacks.ToolMetadata, allowedToolCodes []string) []*schema.ToolInfo {
	if len(allTools) == 0 {
		return nil
	}
	ret := make([]*schema.ToolInfo, 0, len(allTools))
	for _, info := range allTools {
		if info == nil {
			continue
		}
		metadata, _ := resolveRuntimeToolMetadata(strings.TrimSpace(info.Name), toolMetadataByName)
		if isToolCodeAllowedForSkill(metadata.ToolCode, allowedToolCodes) {
			ret = append(ret, info)
		}
	}
	return ret
}

func extractToolCodesFromInfos(infos []*schema.ToolInfo, toolMetadataByName map[string]einocallbacks.ToolMetadata) []string {
	ret := make([]string, 0, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		metadata, ok := resolveRuntimeToolMetadata(strings.TrimSpace(info.Name), toolMetadataByName)
		if !ok || strings.TrimSpace(metadata.ToolCode) == "" {
			continue
		}
		ret = append(ret, metadata.ToolCode)
	}
	return toolx.NormalizeToolCodes(ret)
}

func isToolCodeAllowedForSkill(toolCode string, allowedToolCodes []string) bool {
	toolCode = toolx.NormalizeToolCodeAlias(strings.TrimSpace(toolCode))
	if toolCode == "" {
		return true
	}
	allowed := toolx.NormalizeToolCodes(allowedToolCodes)
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, item := range allowed {
		allowedSet[item] = struct{}{}
		if item == toolCode {
			return true
		}
	}
	if isRuntimeToolAlwaysAllowedForSkill(toolCode) {
		return true
	}
	return toolx.IsImpliedAllowedToolCode(toolCode, allowedSet)
}

func isRuntimeToolAlwaysAllowedForSkill(toolCode string) bool {
	toolCode = toolx.NormalizeToolCodeAlias(strings.TrimSpace(toolCode))
	switch toolCode {
	case toolx.BuiltinSkill.Code,
		toolx.BuiltinToolSearch.Code,
		toolx.GraphTriageServiceRequest.Code,
		toolx.GraphAnalyzeConversation.Code:
		return true
	default:
		return toolx.IsAlwaysAllowedToolCode(toolCode)
	}
}

func resolveRuntimeToolMetadata(toolName string, toolMetadataByName map[string]einocallbacks.ToolMetadata) (einocallbacks.ToolMetadata, bool) {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return einocallbacks.ToolMetadata{}, false
	}
	if spec, ok := toolx.GetRegisteredToolSpecByName(toolName); ok {
		resolved := toolx.ResolveToolMetadata(spec.Code, spec.Name)
		return einocallbacks.ToolMetadata{
			ToolCode:   resolved.ToolCode,
			ServerCode: resolved.ServerCode,
			ToolName:   resolved.ToolName,
			SourceType: resolved.SourceType,
		}, true
	}
	metadata, ok := toolMetadataByName[toolName]
	return metadata, ok
}

func skillIDFromArguments(argumentsInJSON string) string {
	var args struct {
		Skill string `json:"skill"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(argumentsInJSON)), &args); err != nil {
		return ""
	}
	return strings.TrimSpace(args.Skill)
}

func parseRuntimeToolArguments(argumentsInJSON string) map[string]any {
	argumentsInJSON = strings.TrimSpace(argumentsInJSON)
	if argumentsInJSON == "" {
		return nil
	}
	ret := make(map[string]any)
	if err := json.Unmarshal([]byte(argumentsInJSON), &ret); err != nil {
		return nil
	}
	return ret
}

func filterToolSearchResult(result string, allowedToolCodes []string, toolMetadataByName map[string]einocallbacks.ToolMetadata) (string, error) {
	result = strings.TrimSpace(result)
	if result == "" {
		return result, nil
	}
	var payload struct {
		SelectedTools []string `json:"selectedTools"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		return result, err
	}
	filtered := make([]string, 0, len(payload.SelectedTools))
	for _, toolName := range payload.SelectedTools {
		metadata, _ := resolveRuntimeToolMetadata(toolName, toolMetadataByName)
		if isToolCodeAllowedForSkill(metadata.ToolCode, allowedToolCodes) {
			filtered = append(filtered, strings.TrimSpace(toolName))
		}
	}
	payload.SelectedTools = filtered
	buf, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func cloneToolMetadataMap(input map[string]einocallbacks.ToolMetadata) map[string]einocallbacks.ToolMetadata {
	if len(input) == 0 {
		return nil
	}
	ret := make(map[string]einocallbacks.ToolMetadata, len(input))
	for key, value := range input {
		ret[key] = value
	}
	return ret
}

func cloneSkillMetadataMap(input map[string]einocallbacks.SkillMetadata) map[string]einocallbacks.SkillMetadata {
	if len(input) == 0 {
		return nil
	}
	ret := make(map[string]einocallbacks.SkillMetadata, len(input))
	for key, value := range input {
		value.AllowedToolCodes = append([]string(nil), value.AllowedToolCodes...)
		ret[key] = value
	}
	return ret
}

package builders

import (
	"encoding/json"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/pkg/toolx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/services"
)

func BuildAIAgent(item *models.AIAgent) response.AIAgentResponse {
	return BuildAIAgentWithLocale(item, i18nx.DefaultLocale)
}

func BuildAIAgentWithLocale(item *models.AIAgent, locale string) response.AIAgentResponse {
	ret := response.AIAgentResponse{
		ID:                  item.ID,
		TenantID:            item.TenantID,
		ProductID:           item.ProductID,
		Source:              item.Source,
		Name:                item.Name,
		Description:         item.Description,
		Status:              item.Status,
		StatusName:          enums.GetStatusLabel(item.Status),
		AIConfigID:          item.AIConfigID,
		LLMModelName:        item.LLMModelName,
		ServiceMode:         item.ServiceMode,
		ServiceModeName:     enums.GetIMConversationServiceModeLabel(item.ServiceMode),
		SystemPrompt:        item.SystemPrompt,
		WelcomeMessage:      item.WelcomeMessage,
		ReplyTimeoutSeconds: item.ReplyTimeoutSeconds,
		HandoffMode:         item.HandoffMode,
		HandoffModeName:     enums.GetAIAgentHandoffModeLabel(item.HandoffMode),
		FallbackMode:        item.FallbackMode,
		FallbackModeName:    enums.GetAIAgentFallbackModeLabel(item.FallbackMode),
		FallbackMessage:     item.FallbackMessage,
		KnowledgeIDs:        utils.SplitInt64s(item.KnowledgeIDs),
		SkillIDs:            utils.SplitInt64s(item.SkillIDs),
		KnowledgeBaseNames:  make([]string, 0),
		Skills:              make([]response.AIAgentSkillResponse, 0),
		Teams:               make([]response.AIAgentTeamResponse, 0),
		DirectTools:         make([]response.AIAgentMCPToolResponse, 0),
		GraphTools:          make([]string, 0),
		WorkflowVersionID:   item.WorkflowVersionID,
		WorkflowID:          item.WorkflowID,
		ActiveReleaseID:     item.ActiveReleaseID,
		DraftRevision:       item.DraftRevision,
		WorkflowPublished:   item.WorkflowVersionID > 0,
		WorkflowState:       aiAgentWorkflowState(item.WorkflowVersionID),
		WorkflowStateText:   aiAgentWorkflowStateText(item.WorkflowVersionID),
		ReviewStatus:        item.ReviewStatus,
		ReviewStatusName:    enums.GetAIAgentReviewStatusLabel(item.ReviewStatus),
		ReviewComment:       item.ReviewComment,
		ReviewedAt:          utils.FormatTimePtr(item.ReviewedAt),
		ReviewedByName:      item.ReviewedByName,
		SortNo:              item.SortNo,
		CreatedAt:           item.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:           item.UpdatedAt.Format("2006-01-02 15:04:05"),
		CreateUserName:      item.CreateUserName,
		UpdateUserName:      item.UpdateUserName,
	}
	if product := services.ProductService.Get(item.ProductID); product != nil && product.TenantID == item.TenantID {
		ret.ProductName = product.Name
		ret.ProductCode = product.Code
	}
	if profile := services.ProductServiceProfileService.GetByProductID(item.ProductID); profile != nil &&
		profile.TenantID == item.TenantID &&
		profile.Status != enums.StatusDeleted {
		ret.ProductDefaultKnowledgeBaseID = profile.DefaultKnowledgeBaseID
	}
	if aiConfig := services.AIConfigService.Get(item.AIConfigID); aiConfig != nil {
		ret.AIConfigName = aiConfig.Name
	} else if item.AIConfigID == 0 {
		ret.AIConfigName = "平台模型能力"
	}
	for _, id := range utils.SplitInt64s(item.TeamIDs) {
		if team := services.AgentTeamService.Get(id); team != nil {
			ret.Teams = append(ret.Teams, response.AIAgentTeamResponse{
				ID:   team.ID,
				Name: team.Name,
			})
		}
	}
	for _, id := range ret.KnowledgeIDs {
		if knowledgeBase := services.KnowledgeBaseService.Get(id); knowledgeBase != nil {
			ret.KnowledgeBaseNames = append(ret.KnowledgeBaseNames, knowledgeBase.Name)
		}
	}
	for _, id := range ret.SkillIDs {
		if skill := services.SkillDefinitionService.Get(id); skill != nil {
			ret.Skills = append(ret.Skills, response.AIAgentSkillResponse{
				ID:   skill.ID,
				Name: skill.Name,
			})
		}
	}
	buildAIAgentDirectTools(&ret, item, locale)
	buildAIAgentGraphTools(&ret, item)
	return ret
}

func BuildAIAgentListWithLocale(items []models.AIAgent, locale string) []response.AIAgentResponse {
	ret := make([]response.AIAgentResponse, 0, len(items))
	for i := range items {
		ret = append(ret, BuildAIAgentWithLocale(&items[i], locale))
	}
	return ret
}

func buildAIAgentDirectTools(ret *response.AIAgentResponse, item *models.AIAgent, locale string) {
	raw := strings.TrimSpace(item.AllowedMCPTools)
	if raw == "" {
		return
	}
	var directTools []request.AIAgentMCPToolRequest
	if err := json.Unmarshal([]byte(raw), &directTools); err != nil {
		return
	}
	for _, tool := range directTools {
		toolCode := strings.TrimSpace(tool.ToolCode)
		if toolCode == "" {
			toolCode = toolx.BuildMCPToolCode(tool.ServerCode, tool.ToolName)
		}
		toolCode = toolx.NormalizeToolCodeAlias(toolCode)
		if toolx.IsAutoInjectedToolCode(toolCode) {
			continue
		}
		if toolx.IsAgentDirectGraphToolCode(toolCode) {
			ret.GraphTools = appendGraphToolCodeIfMissing(ret.GraphTools, toolCode)
			continue
		}
		serverCode := strings.TrimSpace(tool.ServerCode)
		toolName := strings.TrimSpace(tool.ToolName)
		if registeredServerCode, registeredToolName, ok := toolx.GetRegisteredToolIdentity(toolCode); ok {
			serverCode = registeredServerCode
			toolName = registeredToolName
		} else if parsedServerCode, parsedToolName := toolx.SplitMCPToolCode(toolCode); parsedServerCode != "" && parsedToolName != "" {
			serverCode = parsedServerCode
			toolName = parsedToolName
		}
		title := strings.TrimSpace(tool.Title)
		if title == "" {
			if registeredTitle := toolx.GetRegisteredToolTitleLocale(toolCode, locale); registeredTitle != "" {
				title = registeredTitle
			}
		}
		description := strings.TrimSpace(tool.Description)
		if description == "" {
			if registeredDescription := toolx.GetRegisteredToolDescriptionLocale(toolCode, locale); registeredDescription != "" {
				description = registeredDescription
			}
		}
		ret.DirectTools = append(ret.DirectTools, response.AIAgentMCPToolResponse{
			ToolCode:    toolCode,
			ServerCode:  serverCode,
			ToolName:    toolName,
			Title:       title,
			Description: description,
			Arguments:   tool.Arguments,
		})
	}
}

func buildAIAgentGraphTools(ret *response.AIAgentResponse, item *models.AIAgent) {
	raw := strings.TrimSpace(item.AllowedGraphTools)
	if raw == "" {
		return
	}
	var graphTools []string
	if err := json.Unmarshal([]byte(raw), &graphTools); err != nil {
		return
	}
	for _, toolCode := range graphTools {
		toolCode = toolx.NormalizeToolCodeAlias(strings.TrimSpace(toolCode))
		if !toolx.IsAgentDirectGraphToolCode(toolCode) {
			continue
		}
		ret.GraphTools = appendGraphToolCodeIfMissing(ret.GraphTools, toolCode)
	}
}

func aiAgentWorkflowState(workflowVersionID int64) string {
	if workflowVersionID > 0 {
		return "published"
	}
	return "draft"
}

func aiAgentWorkflowStateText(workflowVersionID int64) string {
	if workflowVersionID > 0 {
		return "已发布"
	}
	return "未发布"
}

func appendGraphToolCodeIfMissing(items []string, toolCode string) []string {
	toolCode = strings.TrimSpace(toolCode)
	if toolCode == "" {
		return items
	}
	for _, item := range items {
		if strings.TrimSpace(item) == toolCode {
			return items
		}
	}
	return append(items, toolCode)
}

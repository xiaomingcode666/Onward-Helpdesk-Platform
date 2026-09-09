package response

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
)

type AIAgentTeamResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type AIAgentSkillResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type AIAgentMCPToolResponse struct {
	ToolCode    string            `json:"toolCode"`
	ServerCode  string            `json:"serverCode"`
	ToolName    string            `json:"toolName"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Arguments   map[string]string `json:"arguments"`
}

type AIConfigResponse struct {
	ID               int64             `json:"id"`
	Name             string            `json:"name"`
	Provider         enums.AIProvider  `json:"provider"`
	BaseURL          string            `json:"baseUrl"`
	HasAPIKey        bool              `json:"hasApiKey"`
	ModelType        enums.AIModelType `json:"modelType"`
	ModelName        string            `json:"modelName"`
	Dimension        int               `json:"dimension"`
	MaxContextTokens int               `json:"maxContextTokens"`
	MaxOutputTokens  int               `json:"maxOutputTokens"`
	TimeoutMS        int               `json:"timeoutMs"`
	MaxRetryCount    int               `json:"maxRetryCount"`
	RPMLimit         int               `json:"rpmLimit"`
	TPMLimit         int               `json:"tpmLimit"`
	Status           enums.Status      `json:"status"`
	SortNo           int               `json:"sortNo"`
	Remark           string            `json:"remark"`
}

func BuildAIConfigResponse(item *models.AIConfig) AIConfigResponse {
	return AIConfigResponse{
		ID:               item.ID,
		Name:             item.Name,
		Provider:         item.Provider,
		BaseURL:          item.BaseURL,
		HasAPIKey:        item.APIKey != "",
		ModelType:        item.ModelType,
		ModelName:        item.ModelName,
		Dimension:        item.Dimension,
		MaxContextTokens: item.MaxContextTokens,
		MaxOutputTokens:  item.MaxOutputTokens,
		TimeoutMS:        item.TimeoutMS,
		MaxRetryCount:    item.MaxRetryCount,
		RPMLimit:         item.RPMLimit,
		TPMLimit:         item.TPMLimit,
		Status:           item.Status,
		SortNo:           item.SortNo,
		Remark:           item.Remark,
	}
}

type AIAgentResponse struct {
	ID                            int64                           `json:"id"`
	TenantID                      int64                           `json:"tenantId"`
	ProductID                     int64                           `json:"productId"`
	ProductName                   string                          `json:"productName"`
	ProductCode                   string                          `json:"productCode"`
	ProductDefaultKnowledgeBaseID int64                           `json:"productDefaultKnowledgeBaseId"`
	Source                        string                          `json:"source"`
	Name                          string                          `json:"name"`
	Description                   string                          `json:"description"`
	Status                        enums.Status                    `json:"status"`
	StatusName                    string                          `json:"statusName"`
	AIConfigID                    int64                           `json:"aiConfigId"`
	AIConfigName                  string                          `json:"aiConfigName"`
	LLMModelName                  string                          `json:"llmModelName"`
	ServiceMode                   enums.IMConversationServiceMode `json:"serviceMode"`
	ServiceModeName               string                          `json:"serviceModeName"`
	SystemPrompt                  string                          `json:"systemPrompt"`
	WelcomeMessage                string                          `json:"welcomeMessage"`
	ReplyTimeoutSeconds           int                             `json:"replyTimeoutSeconds"`
	Teams                         []AIAgentTeamResponse           `json:"teams"`
	HandoffMode                   enums.AIAgentHandoffMode        `json:"handoffMode"`
	HandoffModeName               string                          `json:"handoffModeName"`
	FallbackMode                  enums.AIAgentFallbackMode       `json:"fallbackMode"`
	FallbackModeName              string                          `json:"fallbackModeName"`
	FallbackMessage               string                          `json:"fallbackMessage"`
	KnowledgeIDs                  []int64                         `json:"knowledgeIds"`
	KnowledgeBaseNames            []string                        `json:"knowledgeBaseNames"`
	SkillIDs                      []int64                         `json:"skillIds"`
	Skills                        []AIAgentSkillResponse          `json:"skills"`
	DirectTools                   []AIAgentMCPToolResponse        `json:"directTools"`
	GraphTools                    []string                        `json:"graphTools"`
	WorkflowVersionID             int64                           `json:"workflowVersionId"`
	WorkflowID                    int64                           `json:"workflowId"`
	ActiveReleaseID               int64                           `json:"activeReleaseId"`
	DraftRevision                 int64                           `json:"draftRevision"`
	WorkflowPublished             bool                            `json:"workflowPublished"`
	WorkflowState                 string                          `json:"workflowState"`
	WorkflowStateText             string                          `json:"workflowStateText"`
	ReviewStatus                  enums.AIAgentReviewStatus       `json:"reviewStatus"`
	ReviewStatusName              string                          `json:"reviewStatusName"`
	ReviewComment                 string                          `json:"reviewComment"`
	ReviewedAt                    string                          `json:"reviewedAt"`
	ReviewedByName                string                          `json:"reviewedByName"`
	SortNo                        int                             `json:"sortNo"`
	CreatedAt                     string                          `json:"createdAt"`
	UpdatedAt                     string                          `json:"updatedAt"`
	CreateUserName                string                          `json:"createUserName"`
	UpdateUserName                string                          `json:"updateUserName"`
}

type EnterpriseAIAgentSummaryResponse struct {
	Total                  int64   `json:"total"`
	Active                 int64   `json:"active"`
	NotDeployed            int64   `json:"notDeployed"`
	ProductTotal           int64   `json:"productTotal"`
	ProductAgentTotal      int64   `json:"productAgentTotal"`
	MissingProductAgents   int64   `json:"missingProductAgents"`
	TenantDefaultReady     bool    `json:"tenantDefaultReady"`
	ProductAgentProductIDs []int64 `json:"productAgentProductIds"`
}

type AIAgentReleaseResponse struct {
	ID                     int64                     `json:"id"`
	TenantID               int64                     `json:"tenantId"`
	ProductID              int64                     `json:"productId"`
	AgentID                int64                     `json:"agentId"`
	ReleaseNo              int                       `json:"releaseNo"`
	WorkflowID             int64                     `json:"workflowId"`
	WorkflowVersionID      int64                     `json:"workflowVersionId"`
	WorkflowDefinitionHash string                    `json:"workflowDefinitionHash"`
	AgentConfigHash        string                    `json:"agentConfigHash"`
	KnowledgeScopeHash     string                    `json:"knowledgeScopeHash"`
	ReviewStatus           enums.AIAgentReviewStatus `json:"reviewStatus"`
	ReviewStatusName       string                    `json:"reviewStatusName"`
	ReviewComment          string                    `json:"reviewComment"`
	ReviewedAt             string                    `json:"reviewedAt"`
	ReviewedByName         string                    `json:"reviewedByName"`
	DeploymentStatus       string                    `json:"deploymentStatus"`
	DeployedAt             string                    `json:"deployedAt"`
	DeployedByName         string                    `json:"deployedByName"`
	RollbackFromReleaseID  int64                     `json:"rollbackFromReleaseId"`
	Status                 enums.Status              `json:"status"`
	CreatedAt              string                    `json:"createdAt"`
	UpdatedAt              string                    `json:"updatedAt"`
}

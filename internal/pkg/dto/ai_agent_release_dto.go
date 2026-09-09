package dto

import "remotehelpdesk/internal/pkg/enums"

type AIAgentReleaseConfigSnapshot struct {
	SchemaVersion       int                             `json:"schemaVersion"`
	DraftRevision       int64                           `json:"draftRevision"`
	AIConfigID          int64                           `json:"aiConfigId"`
	LLMModelName        string                          `json:"llmModelName"`
	ServiceMode         enums.IMConversationServiceMode `json:"serviceMode"`
	SystemPrompt        string                          `json:"systemPrompt"`
	WelcomeMessage      string                          `json:"welcomeMessage"`
	ReplyTimeoutSeconds int                             `json:"replyTimeoutSeconds"`
	TeamIDs             []int64                         `json:"teamIds"`
	HandoffMode         enums.AIAgentHandoffMode        `json:"handoffMode"`
	FallbackMode        enums.AIAgentFallbackMode       `json:"fallbackMode"`
	FallbackMessage     string                          `json:"fallbackMessage"`
	KnowledgeIDs        []int64                         `json:"knowledgeIds"`
	SkillIDs            []int64                         `json:"skillIds"`
	AllowedMCPTools     any                             `json:"allowedMcpTools"`
	AllowedGraphTools   []string                        `json:"allowedGraphTools"`
}

type AIAgentReleaseKnowledgeBindingSnapshot struct {
	BindingID       int64  `json:"bindingId"`
	ProductModelID  int64  `json:"productModelId"`
	KnowledgeBaseID int64  `json:"knowledgeBaseId"`
	ScopeType       string `json:"scopeType"`
	Locale          string `json:"locale"`
	RegionCode      string `json:"regionCode"`
}

type AIAgentReleaseKnowledgeRevisionSnapshot struct {
	RevisionID        int64  `json:"revisionId"`
	KnowledgeBaseID   int64  `json:"knowledgeBaseId"`
	ContentHash       string `json:"contentHash"`
	Language          string `json:"language"`
	Visibility        string `json:"visibility"`
	IndexGenerationID int64  `json:"indexGenerationId"`
}

type AIAgentReleaseKnowledgeScopeSnapshot struct {
	SchemaVersion int                                       `json:"schemaVersion"`
	TenantID      int64                                     `json:"tenantId"`
	ProductID     int64                                     `json:"productId"`
	Resolution    string                                    `json:"resolution"`
	Bindings      []AIAgentReleaseKnowledgeBindingSnapshot  `json:"bindings"`
	Revisions     []AIAgentReleaseKnowledgeRevisionSnapshot `json:"revisions"`
}

type AIAgentReleaseReviewRequest struct {
	Comment string `json:"comment"`
}

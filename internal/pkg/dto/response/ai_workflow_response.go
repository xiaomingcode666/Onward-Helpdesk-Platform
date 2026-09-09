package response

import (
	"remotehelpdesk/internal/ai/workflow/dsl"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
	workflowvalidator "remotehelpdesk/internal/ai/workflow/validator"
	"remotehelpdesk/internal/pkg/enums"
)

type AIWorkflowResponse struct {
	ID                     int64          `json:"id"`
	TenantID               int64          `json:"tenantId"`
	Code                   string         `json:"code"`
	Scope                  string         `json:"scope"`
	Name                   string         `json:"name"`
	Description            string         `json:"description"`
	AgentID                int64          `json:"agentId"`
	Status                 enums.Status   `json:"status"`
	DraftDefinition        dsl.Definition `json:"draftDefinition"`
	PublishedVersionID     int64          `json:"publishedVersionId"`
	CurrentStableVersionID int64          `json:"currentStableVersionId"`
	SourceWorkflowID       int64          `json:"sourceWorkflowId"`
	SourceVersionID        int64          `json:"sourceVersionId"`
	Locked                 bool           `json:"locked"`
	HumanHandoffEnabled    bool           `json:"humanHandoffEnabled"`
	SortNo                 int            `json:"sortNo"`
	CreatedAt              string         `json:"createdAt"`
	UpdatedAt              string         `json:"updatedAt"`
	CreateUserName         string         `json:"createUserName"`
	UpdateUserName         string         `json:"updateUserName"`
}

type AIWorkflowVersionResponse struct {
	ID              int64          `json:"id"`
	WorkflowID      int64          `json:"workflowId"`
	Version         int            `json:"version"`
	Status          enums.Status   `json:"status"`
	Definition      dsl.Definition `json:"definition"`
	DefinitionHash  string         `json:"definitionHash"`
	ReleaseChannel  string         `json:"releaseChannel"`
	SchemaVersion   int            `json:"schemaVersion"`
	ChangeSummary   string         `json:"changeSummary"`
	SourceVersionID int64          `json:"sourceVersionId"`
	PublishedAt     string         `json:"publishedAt"`
	PublishedByID   int64          `json:"publishedById"`
	PublishedByName string         `json:"publishedByName"`
	CreatedAt       string         `json:"createdAt"`
	UpdatedAt       string         `json:"updatedAt"`
}

type AIWorkflowSummaryResponse struct {
	Total                   int64                `json:"total"`
	Adopted                 int64                `json:"adopted"`
	Attention               int64                `json:"attention"`
	PlatformTemplates       []AIWorkflowResponse `json:"platformTemplates"`
	StableVersionByWorkflow map[int64]int64      `json:"stableVersionByWorkflow"`
}

type AIWorkflowAdoptionResponse struct {
	Adoption                map[int64]int64 `json:"adoption"`
	StableAdoption          map[int64]int64 `json:"stableAdoption"`
	StableVersionByWorkflow map[int64]int64 `json:"stableVersionByWorkflow"`
}

type AIWorkflowValidationResponse struct {
	Valid  bool                      `json:"valid"`
	Errors []workflowvalidator.Error `json:"errors"`
}

type AIWorkflowNodeSpecResponse struct {
	Type                            string                          `json:"type"`
	Title                           string                          `json:"title"`
	Description                     string                          `json:"description"`
	RiskLevel                       workflowregistry.NodeRiskLevel  `json:"riskLevel"`
	Interruptible                   bool                            `json:"interruptible"`
	RequiresConfirmationPredecessor bool                            `json:"requiresConfirmationPredecessor"`
	ConfigSchema                    any                             `json:"configSchema,omitempty"`
	InputSchema                     []workflowregistry.VariableSpec `json:"inputSchema,omitempty"`
	OutputSchema                    []workflowregistry.VariableSpec `json:"outputSchema,omitempty"`
	DefaultInputs                   map[string]dsl.VariableSelector `json:"defaultInputs,omitempty"`
}

type AIWorkflowRunResponse struct {
	ID                int64                            `json:"id"`
	TenantID          int64                            `json:"tenantId"`
	ProductID         int64                            `json:"productId"`
	WorkflowID        int64                            `json:"workflowId"`
	WorkflowVersionID int64                            `json:"workflowVersionId"`
	DefinitionHash    string                           `json:"definitionHash"`
	RuntimeEngine     string                           `json:"runtimeEngine"`
	WorkflowVersion   int                              `json:"workflowVersion"`
	WorkflowName      string                           `json:"workflowName"`
	ConversationID    int64                            `json:"conversationId"`
	AIAgentID         int64                            `json:"aiAgentId"`
	AgentReleaseID    int64                            `json:"agentReleaseId"`
	AIAgentName       string                           `json:"aiAgentName"`
	MessageID         int64                            `json:"messageId"`
	Status            int                              `json:"status"`
	StatusName        string                           `json:"statusName"`
	StartedAt         string                           `json:"startedAt"`
	EndedAt           string                           `json:"endedAt"`
	DurationMS        int64                            `json:"durationMs"`
	InterruptType     string                           `json:"interruptType"`
	InterruptNodeID   string                           `json:"interruptNodeId"`
	ErrorMessage      string                           `json:"errorMessage"`
	ConfigSnapshot    string                           `json:"configSnapshot"`
	TraceData         string                           `json:"traceData"`
	CreatedAt         string                           `json:"createdAt"`
	UpdatedAt         string                           `json:"updatedAt"`
	Definition        dsl.Definition                   `json:"definition"`
	Nodes             []AIWorkflowNodeRunResponse      `json:"nodes,omitempty"`
	SkillAudit        AIWorkflowSkillAuditResponse     `json:"skillAudit"`
	HumanHandling     *AIWorkflowHumanHandlingResponse `json:"humanHandling,omitempty"`
}

type AIWorkflowHumanHandlingResponse struct {
	HandoffOccurred          bool   `json:"handoffOccurred"`
	HandoffAt                string `json:"handoffAt"`
	HandoffReason            string `json:"handoffReason"`
	HandledByHuman           bool   `json:"handledByHuman"`
	HandlerUserID            int64  `json:"handlerUserId"`
	HandlerName              string `json:"handlerName"`
	FirstHumanReplyMessageID int64  `json:"firstHumanReplyMessageId"`
	FirstHumanReplyAt        string `json:"firstHumanReplyAt"`
	ConversationStatus       int    `json:"conversationStatus"`
	ConversationStatusName   string `json:"conversationStatusName"`
}

type AIWorkflowSkillAuditResponse struct {
	State                    string                             `json:"state"`
	MiddlewareEnabled        bool                               `json:"middlewareEnabled"`
	CandidateSkills          []AIWorkflowSkillCandidateResponse `json:"candidateSkills"`
	SelectedSkillID          int64                              `json:"selectedSkillId"`
	SelectedSkillName        string                             `json:"selectedSkillName"`
	SelectedSkillDescription string                             `json:"selectedSkillDescription"`
	MatchReason              string                             `json:"matchReason"`
	RouteTrace               string                             `json:"routeTrace"`
	ExposedToolCodes         []string                           `json:"exposedToolCodes"`
	InvokedToolCodes         []string                           `json:"invokedToolCodes"`
	SourceMessageID          int64                              `json:"sourceMessageId"`
	CreatedAt                string                             `json:"createdAt"`
}

type AIWorkflowSkillCandidateResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AIWorkflowNodeRunResponse struct {
	ID             int64  `json:"id"`
	WorkflowRunID  int64  `json:"workflowRunId"`
	NodeID         string `json:"nodeId"`
	NodeType       string `json:"nodeType"`
	Attempt        int    `json:"attempt"`
	IdempotencyKey string `json:"idempotencyKey"`
	Status         int    `json:"status"`
	StatusName     string `json:"statusName"`
	InputPreview   string `json:"inputPreview"`
	OutputPreview  string `json:"outputPreview"`
	ErrorMessage   string `json:"errorMessage"`
	StartedAt      string `json:"startedAt"`
	EndedAt        string `json:"endedAt"`
	DurationMS     int    `json:"durationMs"`
}

type AIWorkflowTestRunResponse struct {
	WorkflowID              int64                              `json:"workflowId"`
	AIAgentID               int64                              `json:"aiAgentId"`
	AIAgentName             string                             `json:"aiAgentName"`
	RuntimeEngine           string                             `json:"runtimeEngine"`
	Status                  string                             `json:"status"`
	ReplyText               string                             `json:"replyText"`
	FailedNodeID            string                             `json:"failedNodeId"`
	InterruptNodeID         string                             `json:"interruptNodeId"`
	ErrorMessage            string                             `json:"errorMessage"`
	DurationMS              int64                              `json:"durationMs"`
	SideEffects             string                             `json:"sideEffects"`
	ModelCredentialScope    string                             `json:"modelCredentialScope"`
	ModelAPIKeyID           string                             `json:"modelApiKeyId"`
	ModelCredentialFallback bool                               `json:"modelCredentialFallback"`
	NodePath                []string                           `json:"nodePath"`
	Nodes                   []AIWorkflowTestNodeResultResponse `json:"nodes"`
}

type AIWorkflowTestNodeResultResponse struct {
	NodeID        string `json:"nodeId"`
	NodeType      string `json:"nodeType"`
	Status        string `json:"status"`
	InputPreview  string `json:"inputPreview"`
	OutputPreview string `json:"outputPreview"`
	ErrorMessage  string `json:"errorMessage"`
	DurationMS    int    `json:"durationMs"`
}

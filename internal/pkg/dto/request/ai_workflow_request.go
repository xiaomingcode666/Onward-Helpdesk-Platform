package request

import "remotehelpdesk/internal/ai/workflow/dsl"

type CreateAIWorkflowRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	AgentID     int64          `json:"agentId"`
	Definition  dsl.Definition `json:"definition"`
}

type SaveAIWorkflowRequest = CreateAIWorkflowRequest

type UpdateAIWorkflowRequest struct {
	ID int64 `json:"id"`
	CreateAIWorkflowRequest
}

type DeleteAIWorkflowRequest struct {
	ID int64 `json:"id"`
}

type ValidateAIWorkflowRequest struct {
	Definition dsl.Definition `json:"definition"`
}

type TestAIWorkflowRequest struct {
	AIAgentID       int64                        `json:"aiAgentId"`
	Definition      dsl.Definition               `json:"definition"`
	UserMessage     string                       `json:"userMessage"`
	AutoConfirm     bool                         `json:"autoConfirm"`
	BranchOverrides map[string]string            `json:"branchOverrides"`
	RuntimeContext  AIWorkflowTestRuntimeContext `json:"runtimeContext"`
}

type AIWorkflowTestRuntimeContext struct {
	ProductModelID         int64  `json:"productModelId"`
	DeviceID               int64  `json:"deviceId"`
	ServiceCodeID          int64  `json:"serviceCodeId"`
	CustomerEntrySessionID int64  `json:"customerEntrySessionId"`
	DeviceBound            bool   `json:"deviceBound"`
	ServiceMode            string `json:"serviceMode"`
}

type PublishAIWorkflowRequest struct {
	WorkflowID int64          `json:"workflowId"`
	AgentID    int64          `json:"agentId"`
	Definition dsl.Definition `json:"definition"`
}

type AIWorkflowVersionListRequest struct {
	WorkflowID int64 `json:"workflowId"`
}

type CreateAIWorkflowTemplateRequest struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	SourceVersionID int64  `json:"sourceVersionId"`
}

type UpdateAIWorkflowTemplateDraftRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Definition  dsl.Definition `json:"definition"`
}

type BindAIAgentWorkflowVersionRequest struct {
	WorkflowVersionID int64 `json:"workflowVersionId"`
}

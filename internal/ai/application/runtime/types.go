package runtime

import (
	"remotehelpdesk/internal/ai/runtime/registry"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
)

type Request struct {
	Conversation models.Conversation
	UserMessage  models.Message
	AIAgent      models.AIAgent
	AIConfig     models.AIConfig
	CheckPointID string
	ToolSet      *registry.ToolSet
}

type ResumeRequest struct {
	Conversation models.Conversation
	UserMessage  models.Message
	AIAgent      models.AIAgent
	AIConfig     models.AIConfig
	CheckPointID string
	ResumeData   map[string]string
	ToolSet      *registry.ToolSet
}

type InterruptContextSummary struct {
	Type        string `json:"type,omitempty"`
	ID          string `json:"id"`
	InfoPreview string `json:"infoPreview,omitempty"`
}

type Summary struct {
	RunID                   string
	Status                  string
	ReplyText               string
	KnowledgeCitations      []dto.KnowledgeCitation
	PlannedSkillID          int64
	PlannedSkillName        string
	PlanReason              string
	SkillRouteTrace         string
	SkillAllowedToolCodes   []string
	ModelProvider           string
	ModelName               string
	ModelCredentialScope    string
	ModelAPIKeyID           string
	ModelCredentialFallback bool
	PromptTokens            int
	CompletionTokens        int
	HistoryMessageCount     int
	RetrieverCount          int
	ToolCallCount           int
	ToolCodes               []string
	InvokedToolCodes        []string
	WorkflowID              int64
	WorkflowVersionID       int64
	AgentReleaseID          int64
	WorkflowRunID           int64
	WorkflowNodePath        []string
	CheckPointID            string
	CheckPointData          string
	Interrupted             bool
	Interrupts              []InterruptContextSummary
	TraceData               string
	ErrorMessage            string
}

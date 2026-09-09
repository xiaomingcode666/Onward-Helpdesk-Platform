package executor

import (
	"remotehelpdesk/internal/ai/runtime/registry"
	"remotehelpdesk/internal/models"
)

type RunInput struct {
	Conversation models.Conversation
	UserMessage  models.Message
	AIAgent      models.AIAgent
	AIConfig     models.AIConfig
	CheckPointID string
	ToolSet      *registry.ToolSet
}

// WorkflowNodeInput runs one generative workflow node through the Eino Agent
// stack. Knowledge retrieval and workflow control remain owned by Workflow.
type WorkflowNodeInput struct {
	Conversation     models.Conversation
	UserMessage      models.Message
	AIAgent          models.AIAgent
	AIConfig         models.AIConfig
	ToolSet          *registry.ToolSet
	SystemPrompt     string
	UserPrompt       string
	KnowledgeContext string
	BlockedToolCodes []string
}

type ResumeInput struct {
	Conversation models.Conversation
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

type RunResult struct {
	RunID                 string
	Status                string
	ReplyText             string
	SelectedSkillID       int64
	SelectedSkillName     string
	SkillRouteReason      string
	SkillRouteTrace       string
	SkillAllowedToolCodes []string
	ModelProvider         string
	ModelName             string
	PromptTokens          int
	CompletionTokens      int
	HistoryMessageCount   int
	RetrieverCount        int
	ToolCallCount         int
	ToolCodes             []string
	InvokedToolCodes      []string
	CheckPointID          string
	Interrupted           bool
	Interrupts            []InterruptContextSummary
	TraceData             string
	ErrorMessage          string
}

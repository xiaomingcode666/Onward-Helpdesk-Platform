package dto

type EnterpriseProductResourcesDTO struct {
	ProductID int64                            `json:"product_id"`
	Status    string                           `json:"status"`
	AIKey     EnterpriseProductAIKeyDTO        `json:"ai_key"`
	Knowledge EnterpriseProductKnowledgeDTO    `json:"knowledge"`
	Agent     EnterpriseProductAgentDTO        `json:"agent"`
	Job       *EnterpriseProductResourceJobDTO `json:"job,omitempty"`
}

type EnterpriseProductAIKeyDTO struct {
	Status          string  `json:"status"`
	KeyID           string  `json:"key_id"`
	KeyName         string  `json:"key_name"`
	KeyPreview      string  `json:"key_preview"`
	QuotaLimit      float64 `json:"quota_limit"`
	QuotaUsed       float64 `json:"quota_used"`
	Currency        string  `json:"currency"`
	ProvisionError  string  `json:"provision_error,omitempty"`
	LastSyncedAt    string  `json:"last_synced_at,omitempty"`
	ProvisionStatus string  `json:"provision_status"`
}

type EnterpriseProductKnowledgeDTO struct {
	Status            string `json:"status"`
	KnowledgeBaseID   int64  `json:"knowledge_base_id"`
	KnowledgeBaseName string `json:"knowledge_base_name"`
	DatasetID         string `json:"dataset_id"`
	Provider          string `json:"provider"`
	ProvisionError    string `json:"provision_error,omitempty"`
}

type EnterpriseProductAgentDTO struct {
	Status       string `json:"status"`
	AgentID      int64  `json:"agent_id"`
	AgentName    string `json:"agent_name"`
	ReviewStatus string `json:"review_status"`
	Enabled      bool   `json:"enabled"`
}

type EnterpriseProductAIQuotaUpdateRequest struct {
	QuotaLimit *float64 `json:"quota_limit"`
}

type EnterpriseProductResourceJobDTO struct {
	ID                  int64   `json:"id"`
	Status              string  `json:"status"`
	RequestedQuotaLimit float64 `json:"requested_quota_limit"`
	KnowledgeStatus     string  `json:"knowledge_status"`
	AIKeyStatus         string  `json:"ai_key_status"`
	AgentStatus         string  `json:"agent_status"`
	ErrorSummary        string  `json:"error_summary,omitempty"`
	RetryCount          int     `json:"retry_count"`
	MaxRetries          int     `json:"max_retries"`
	NextAttemptAt       string  `json:"next_attempt_at,omitempty"`
	LastAttemptAt       string  `json:"last_attempt_at,omitempty"`
	StartedAt           string  `json:"started_at,omitempty"`
	FinishedAt          string  `json:"finished_at,omitempty"`
	UpdatedAt           string  `json:"updated_at,omitempty"`
}

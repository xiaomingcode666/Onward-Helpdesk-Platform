package dto

import (
	"encoding/json"
	"time"
)

type IndustrySolutionPackManifestRequest struct {
	IdempotencyKey    string                                        `json:"idempotency_key"`
	PackCode          string                                        `json:"pack_code"`
	Name              string                                        `json:"name"`
	IndustryCode      string                                        `json:"industry_code"`
	ProductFamilyCode string                                        `json:"product_family_code"`
	Version           string                                        `json:"version"`
	SchemaVersion     int                                           `json:"schema_version"`
	DefaultLocale     string                                        `json:"default_locale"`
	Description       string                                        `json:"description"`
	Metadata          json.RawMessage                               `json:"metadata"`
	Resources         []IndustrySolutionPackManifestResourceRequest `json:"resources"`
}

type IndustrySolutionPackManifestResourceRequest struct {
	ResourceType    string          `json:"resource_type"`
	ResourceKey     string          `json:"resource_key"`
	Version         string          `json:"version"`
	Required        *bool           `json:"required"`
	ApplyOrder      int             `json:"apply_order"`
	Dependencies    []string        `json:"dependencies"`
	SourceReference json.RawMessage `json:"source_reference"`
	TargetSelector  json.RawMessage `json:"target_selector"`
	Payload         json.RawMessage `json:"payload"`
	Metadata        json.RawMessage `json:"metadata"`
}

type IndustrySolutionPackApplyDTORequest struct {
	TargetProductID      int64           `json:"target_product_id"`
	TargetProductModelID int64           `json:"target_product_model_id"`
	TargetContext        json.RawMessage `json:"target_context"`
	ConflictStrategy     string          `json:"conflict_strategy"`
	IdempotencyKey       string          `json:"idempotency_key"`
}

type IndustrySolutionPackDTO struct {
	ID                int64           `json:"id"`
	PackCode          string          `json:"pack_code"`
	Name              string          `json:"name"`
	IndustryCode      string          `json:"industry_code"`
	ProductFamilyCode string          `json:"product_family_code"`
	Version           string          `json:"version"`
	SchemaVersion     int             `json:"schema_version"`
	Status            string          `json:"status"`
	DefaultLocale     string          `json:"default_locale"`
	Description       string          `json:"description"`
	ManifestHash      string          `json:"manifest_hash"`
	Metadata          json.RawMessage `json:"metadata"`
	PublishedAt       *time.Time      `json:"published_at,omitempty"`
	PublishedByID     int64           `json:"published_by_id"`
	PublishedByName   string          `json:"published_by_name"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	CreateUserName    string          `json:"create_user_name"`
	UpdateUserName    string          `json:"update_user_name"`
}

type IndustrySolutionPackResourceDTO struct {
	ID              int64           `json:"id"`
	PackID          int64           `json:"pack_id"`
	ResourceType    string          `json:"resource_type"`
	ResourceKey     string          `json:"resource_key"`
	Version         string          `json:"version"`
	Status          string          `json:"status"`
	Required        bool            `json:"required"`
	ApplyOrder      int             `json:"apply_order"`
	Dependencies    json.RawMessage `json:"dependencies"`
	SourceReference json.RawMessage `json:"source_reference"`
	TargetSelector  json.RawMessage `json:"target_selector"`
	Payload         json.RawMessage `json:"payload"`
	Checksum        string          `json:"checksum"`
	Metadata        json.RawMessage `json:"metadata"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	CreateUserName  string          `json:"create_user_name"`
	UpdateUserName  string          `json:"update_user_name"`
}

type IndustrySolutionPackListDTO struct {
	Items    []IndustrySolutionPackDTO `json:"items"`
	Total    int64                     `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
}

type IndustrySolutionPackManifestDTO struct {
	Pack      IndustrySolutionPackDTO           `json:"pack"`
	Resources []IndustrySolutionPackResourceDTO `json:"resources"`
	Reused    bool                              `json:"reused"`
}

type IndustrySolutionPackValidationIssueDTO struct {
	Severity     string `json:"severity"`
	Code         string `json:"code"`
	Message      string `json:"message"`
	ResourceID   int64  `json:"resource_id,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
	ResourceKey  string `json:"resource_key,omitempty"`
}

type IndustrySolutionPackValidationDTO struct {
	Valid                  bool                                     `json:"valid"`
	Pack                   IndustrySolutionPackDTO                  `json:"pack"`
	Resources              []IndustrySolutionPackResourceDTO        `json:"resources"`
	CalculatedManifestHash string                                   `json:"calculated_manifest_hash"`
	Issues                 []IndustrySolutionPackValidationIssueDTO `json:"issues"`
}

type IndustrySolutionPackApplicationDTO struct {
	ID                   int64           `json:"id"`
	PackID               int64           `json:"pack_id"`
	PackCode             string          `json:"pack_code"`
	PackVersion          string          `json:"pack_version"`
	Version              string          `json:"version"`
	TargetProductID      int64           `json:"target_product_id"`
	TargetProductModelID int64           `json:"target_product_model_id"`
	TargetContext        json.RawMessage `json:"target_context"`
	Mode                 string          `json:"mode"`
	ConflictStrategy     string          `json:"conflict_strategy"`
	IdempotencyKey       string          `json:"idempotency_key"`
	RequestHash          string          `json:"request_hash"`
	Status               string          `json:"status"`
	ResourceCount        int             `json:"resource_count"`
	SucceededCount       int             `json:"succeeded_count"`
	SkippedCount         int             `json:"skipped_count"`
	FailedCount          int             `json:"failed_count"`
	Summary              json.RawMessage `json:"summary"`
	LastError            string          `json:"last_error"`
	StartedAt            *time.Time      `json:"started_at,omitempty"`
	CompletedAt          *time.Time      `json:"completed_at,omitempty"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
	CreateUserName       string          `json:"create_user_name"`
	UpdateUserName       string          `json:"update_user_name"`
}

type IndustrySolutionPackApplicationItemDTO struct {
	ID                int64           `json:"id"`
	ApplicationID     int64           `json:"application_id"`
	PackResourceID    int64           `json:"pack_resource_id"`
	ResourceType      string          `json:"resource_type"`
	ResourceKey       string          `json:"resource_key"`
	Version           string          `json:"version"`
	Status            string          `json:"status"`
	Required          bool            `json:"required"`
	ApplyOrder        int             `json:"apply_order"`
	Dependencies      json.RawMessage `json:"dependencies"`
	PlannedAction     string          `json:"planned_action"`
	ConflictReason    string          `json:"conflict_reason"`
	ExistingReference json.RawMessage `json:"existing_reference"`
	AppliedReference  json.RawMessage `json:"applied_reference"`
	Result            json.RawMessage `json:"result"`
	AttemptCount      int             `json:"attempt_count"`
	LastError         string          `json:"last_error"`
	StartedAt         *time.Time      `json:"started_at,omitempty"`
	CompletedAt       *time.Time      `json:"completed_at,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type IndustrySolutionPackExecutionDTO struct {
	Application IndustrySolutionPackApplicationDTO       `json:"application"`
	Items       []IndustrySolutionPackApplicationItemDTO `json:"items"`
	Validation  IndustrySolutionPackValidationDTO        `json:"validation"`
	Reused      bool                                     `json:"reused"`
}

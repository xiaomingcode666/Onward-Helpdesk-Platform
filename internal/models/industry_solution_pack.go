package models

import "time"

const (
	IndustrySolutionPackStatusDraft      = "draft"
	IndustrySolutionPackStatusPublished  = "published"
	IndustrySolutionPackStatusDeprecated = "deprecated"
	IndustrySolutionPackStatusArchived   = "archived"

	IndustrySolutionResourceStatusDraft = "draft"
	IndustrySolutionResourceStatusReady = "ready"
	IndustrySolutionResourceStatusVoid  = "void"

	IndustrySolutionApplicationModeDryRun = "dry_run"
	IndustrySolutionApplicationModeApply  = "apply"

	IndustrySolutionApplicationStatusPending   = "pending"
	IndustrySolutionApplicationStatusRunning   = "running"
	IndustrySolutionApplicationStatusReady     = "ready"
	IndustrySolutionApplicationStatusBlocked   = "blocked"
	IndustrySolutionApplicationStatusSucceeded = "succeeded"
	IndustrySolutionApplicationStatusPartial   = "partial"
	IndustrySolutionApplicationStatusFailed    = "failed"
	IndustrySolutionApplicationStatusNoop      = "noop"

	IndustrySolutionApplicationItemStatusPlanned   = "planned"
	IndustrySolutionApplicationItemStatusRunning   = "running"
	IndustrySolutionApplicationItemStatusSucceeded = "succeeded"
	IndustrySolutionApplicationItemStatusSkipped   = "skipped"
	IndustrySolutionApplicationItemStatusBlocked   = "blocked"
	IndustrySolutionApplicationItemStatusFailed    = "failed"

	IndustrySolutionConflictStrategyFail             = "fail"
	IndustrySolutionConflictStrategySkip             = "skip"
	IndustrySolutionConflictStrategyOverwrite        = "overwrite"
	IndustrySolutionConflictStrategyCreateNewVersion = "create_new_version"

	IndustrySolutionResourceActionCreate           = "create"
	IndustrySolutionResourceActionNoop             = "noop"
	IndustrySolutionResourceActionSkip             = "skip"
	IndustrySolutionResourceActionOverwrite        = "overwrite"
	IndustrySolutionResourceActionCreateNewVersion = "create_new_version"
	IndustrySolutionResourceActionConflict         = "conflict"

	DiagnosisEvalCaseStatusDraft    = "draft"
	DiagnosisEvalCaseStatusActive   = "active"
	DiagnosisEvalCaseStatusArchived = "archived"

	DiagnosisEvalRunStatusPending = "pending"
	DiagnosisEvalRunStatusRunning = "running"
	DiagnosisEvalRunStatusPassed  = "passed"
	DiagnosisEvalRunStatusFailed  = "failed"
	DiagnosisEvalRunStatusError   = "error"

	ARWorkInstructionStatusDraft     = "draft"
	ARWorkInstructionStatusPublished = "published"
	ARWorkInstructionStatusArchived  = "archived"

	ARWorkStepStatusDraft     = "draft"
	ARWorkStepStatusPublished = "published"
	ARWorkStepStatusArchived  = "archived"
)

// IndustrySolutionPack is one immutable, tenant-owned release of an industry
// solution. Existing product, workflow, fault-tree, and knowledge records are
// referenced by resources; their business data is not duplicated here.
type IndustrySolutionPack struct {
	ID                int64      `gorm:"primaryKey;autoIncrement"`
	TenantID          int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_industry_solution_pack_version"`
	PackCode          string     `gorm:"type:varchar(64);not null;index;uniqueIndex:uk_industry_solution_pack_version"`
	Name              string     `gorm:"type:varchar(200);not null"`
	IndustryCode      string     `gorm:"type:varchar(64);not null;index"`
	ProductFamilyCode string     `gorm:"type:varchar(64);not null;default:'';index"`
	Version           string     `gorm:"type:varchar(64);not null;uniqueIndex:uk_industry_solution_pack_version"`
	SchemaVersion     int        `gorm:"type:int;not null;default:1"`
	Status            string     `gorm:"type:varchar(24);not null;default:'draft';index"`
	DefaultLocale     string     `gorm:"type:varchar(16);not null;default:'en'"`
	Description       string     `gorm:"type:text"`
	ManifestHash      string     `gorm:"type:varchar(64);not null;default:''"`
	MetadataJSON      string     `gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	PublishedAt       *time.Time `gorm:"type:timestamp;index"`
	PublishedByID     int64      `gorm:"type:bigint;not null;default:0;index"`
	PublishedByName   string     `gorm:"type:varchar(100);not null;default:''"`
	AuditFields
}

// IndustrySolutionPackResource is a generic manifest entry. PayloadJSON is
// interpreted only by a registered domain handler, which must delegate to the
// existing domain service instead of writing parallel product data.
type IndustrySolutionPackResource struct {
	ID                  int64  `gorm:"primaryKey;autoIncrement"`
	TenantID            int64  `gorm:"type:bigint;not null;index;uniqueIndex:uk_industry_solution_pack_resource"`
	PackID              int64  `gorm:"type:bigint;not null;index;uniqueIndex:uk_industry_solution_pack_resource"`
	ResourceType        string `gorm:"type:varchar(64);not null;index;uniqueIndex:uk_industry_solution_pack_resource"`
	ResourceKey         string `gorm:"type:varchar(128);not null;index;uniqueIndex:uk_industry_solution_pack_resource"`
	Version             string `gorm:"type:varchar(64);not null"`
	Status              string `gorm:"type:varchar(24);not null;default:'draft';index"`
	Required            bool   `gorm:"not null;default:true"`
	ApplyOrder          int    `gorm:"type:int;not null;default:0;index"`
	DependenciesJSON    string `gorm:"column:dependencies_json;type:text;not null;default:'[]'"`
	SourceReferenceJSON string `gorm:"column:source_reference_json;type:text;not null;default:'{}'"`
	TargetSelectorJSON  string `gorm:"column:target_selector_json;type:text;not null;default:'{}'"`
	PayloadJSON         string `gorm:"column:payload_json;type:text;not null;default:'{}'"`
	Checksum            string `gorm:"type:varchar(64);not null"`
	MetadataJSON        string `gorm:"column:metadata_json;type:text;not null;default:'{}'"`
	AuditFields
}

// IndustrySolutionPackApplication is the durable idempotency and audit record
// for both dry-run and apply operations.
type IndustrySolutionPackApplication struct {
	ID                   int64      `gorm:"primaryKey;autoIncrement"`
	TenantID             int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_industry_solution_application_key"`
	PackID               int64      `gorm:"type:bigint;not null;index"`
	PackCode             string     `gorm:"type:varchar(64);not null;index"`
	PackVersion          string     `gorm:"type:varchar(64);not null;index"`
	Version              string     `gorm:"type:varchar(64);not null;default:'1.0.0'"`
	TargetProductID      int64      `gorm:"type:bigint;not null;default:0;index"`
	TargetProductModelID int64      `gorm:"type:bigint;not null;default:0;index"`
	TargetContextJSON    string     `gorm:"column:target_context_json;type:text;not null;default:'{}'"`
	Mode                 string     `gorm:"type:varchar(16);not null;index"`
	ConflictStrategy     string     `gorm:"type:varchar(32);not null;index"`
	IdempotencyKey       string     `gorm:"type:varchar(128);not null;uniqueIndex:uk_industry_solution_application_key"`
	RequestHash          string     `gorm:"type:varchar(64);not null"`
	Status               string     `gorm:"type:varchar(24);not null;default:'pending';index"`
	ResourceCount        int        `gorm:"type:int;not null;default:0"`
	SucceededCount       int        `gorm:"type:int;not null;default:0"`
	SkippedCount         int        `gorm:"type:int;not null;default:0"`
	FailedCount          int        `gorm:"type:int;not null;default:0"`
	SummaryJSON          string     `gorm:"column:summary_json;type:text;not null;default:'{}'"`
	LastError            string     `gorm:"type:text"`
	LeaseToken           string     `gorm:"type:varchar(64);not null;default:'';index"`
	LeaseUntil           *time.Time `gorm:"type:timestamp;index"`
	StartedAt            *time.Time `gorm:"type:timestamp;index"`
	CompletedAt          *time.Time `gorm:"type:timestamp;index"`
	AuditFields
}

// IndustrySolutionPackApplicationItem preserves the plan and result for every
// manifest resource so partial runs can be audited and safely resumed.
type IndustrySolutionPackApplicationItem struct {
	ID                    int64      `gorm:"primaryKey;autoIncrement"`
	TenantID              int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_industry_solution_application_item"`
	ApplicationID         int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_industry_solution_application_item"`
	PackResourceID        int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_industry_solution_application_item"`
	ResourceType          string     `gorm:"type:varchar(64);not null;index"`
	ResourceKey           string     `gorm:"type:varchar(128);not null;index"`
	Version               string     `gorm:"type:varchar(64);not null"`
	Status                string     `gorm:"type:varchar(24);not null;default:'planned';index"`
	Required              bool       `gorm:"not null;default:true"`
	ApplyOrder            int        `gorm:"type:int;not null;default:0;index"`
	DependenciesJSON      string     `gorm:"column:dependencies_json;type:text;not null;default:'[]'"`
	PlannedAction         string     `gorm:"type:varchar(32);not null;index"`
	ConflictReason        string     `gorm:"type:text"`
	ExistingReferenceJSON string     `gorm:"column:existing_reference_json;type:text;not null;default:'{}'"`
	AppliedReferenceJSON  string     `gorm:"column:applied_reference_json;type:text;not null;default:'{}'"`
	ResultJSON            string     `gorm:"column:result_json;type:text;not null;default:'{}'"`
	AttemptCount          int        `gorm:"type:int;not null;default:0"`
	LastError             string     `gorm:"type:text"`
	StartedAt             *time.Time `gorm:"type:timestamp;index"`
	CompletedAt           *time.Time `gorm:"type:timestamp;index"`
	AuditFields
}

// DiagnosisEvalCase captures a versioned, reproducible diagnosis expectation.
type DiagnosisEvalCase struct {
	ID                         int64   `gorm:"primaryKey;autoIncrement"`
	TenantID                   int64   `gorm:"type:bigint;not null;index;uniqueIndex:uk_diagnosis_eval_case_version"`
	PackID                     int64   `gorm:"type:bigint;not null;default:0;index"`
	CaseCode                   string  `gorm:"type:varchar(96);not null;index;uniqueIndex:uk_diagnosis_eval_case_version"`
	Name                       string  `gorm:"type:varchar(200);not null"`
	Description                string  `gorm:"type:text"`
	Version                    string  `gorm:"type:varchar(64);not null;uniqueIndex:uk_diagnosis_eval_case_version"`
	Status                     string  `gorm:"type:varchar(24);not null;default:'draft';index"`
	ProductID                  int64   `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID             int64   `gorm:"type:bigint;not null;default:0;index"`
	DeviceID                   int64   `gorm:"type:bigint;not null;default:0;index"`
	Locale                     string  `gorm:"type:varchar(16);not null;default:'';index"`
	RegionCode                 string  `gorm:"type:varchar(64);not null;default:'';index"`
	ProductContextJSON         string  `gorm:"column:product_context_json;type:text;not null;default:'{}'"`
	DeviceContextJSON          string  `gorm:"column:device_context_json;type:text;not null;default:'{}'"`
	InputMessagesJSON          string  `gorm:"column:input_messages_json;type:text;not null;default:'[]'"`
	ExpectedOutcomeJSON        string  `gorm:"column:expected_outcome_json;type:text;not null;default:'{}'"`
	ExpectedWorkflowBranchJSON string  `gorm:"column:expected_workflow_branch_json;type:text;not null;default:'[]'"`
	ExpectedCitationsJSON      string  `gorm:"column:expected_citations_json;type:text;not null;default:'[]'"`
	SafetyBoundaryJSON         string  `gorm:"column:safety_boundary_json;type:text;not null;default:'{}'"`
	ForbiddenAnswersJSON       string  `gorm:"column:forbidden_answers_json;type:text;not null;default:'[]'"`
	ScoringDimensionsJSON      string  `gorm:"column:scoring_dimensions_json;type:text;not null;default:'[]'"`
	MinimumScore               float64 `gorm:"type:decimal(6,4);not null;default:0"`
	ModelConfigID              int64   `gorm:"type:bigint;not null;default:0;index"`
	ModelName                  string  `gorm:"type:varchar(128);not null;default:''"`
	ModelVersion               string  `gorm:"type:varchar(64);not null;default:''"`
	WorkflowID                 int64   `gorm:"type:bigint;not null;default:0;index"`
	WorkflowVersionID          int64   `gorm:"type:bigint;not null;default:0;index"`
	WorkflowVersion            string  `gorm:"type:varchar(64);not null;default:''"`
	EvaluatorVersion           string  `gorm:"type:varchar(64);not null;default:'1.0.0'"`
	TagsJSON                   string  `gorm:"column:tags_json;type:text;not null;default:'[]'"`
	AuditFields
}

// DiagnosisEvalRun snapshots the model/workflow under test and every scoring
// observation required to reproduce an evaluation result.
type DiagnosisEvalRun struct {
	ID                         int64      `gorm:"primaryKey;autoIncrement"`
	TenantID                   int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_diagnosis_eval_run_key"`
	EvalCaseID                 int64      `gorm:"type:bigint;not null;index"`
	RunKey                     string     `gorm:"type:varchar(128);not null;uniqueIndex:uk_diagnosis_eval_run_key"`
	Version                    string     `gorm:"type:varchar(64);not null;default:'1.0.0'"`
	CaseVersion                string     `gorm:"type:varchar(64);not null"`
	Status                     string     `gorm:"type:varchar(24);not null;default:'pending';index"`
	ProductID                  int64      `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID             int64      `gorm:"type:bigint;not null;default:0;index"`
	DeviceID                   int64      `gorm:"type:bigint;not null;default:0;index"`
	ModelConfigID              int64      `gorm:"type:bigint;not null;default:0;index"`
	ModelName                  string     `gorm:"type:varchar(128);not null;default:''"`
	ModelVersion               string     `gorm:"type:varchar(64);not null;default:''"`
	WorkflowID                 int64      `gorm:"type:bigint;not null;default:0;index"`
	WorkflowVersionID          int64      `gorm:"type:bigint;not null;default:0;index"`
	WorkflowVersion            string     `gorm:"type:varchar(64);not null;default:''"`
	EvaluatorVersion           string     `gorm:"type:varchar(64);not null;default:'1.0.0'"`
	InputSnapshotJSON          string     `gorm:"column:input_snapshot_json;type:text;not null;default:'{}'"`
	OutputJSON                 string     `gorm:"column:output_json;type:text;not null;default:'{}'"`
	ObservedWorkflowBranchJSON string     `gorm:"column:observed_workflow_branch_json;type:text;not null;default:'[]'"`
	ObservedCitationsJSON      string     `gorm:"column:observed_citations_json;type:text;not null;default:'[]'"`
	SafetyViolationsJSON       string     `gorm:"column:safety_violations_json;type:text;not null;default:'[]'"`
	ForbiddenAnswerMatchesJSON string     `gorm:"column:forbidden_answer_matches_json;type:text;not null;default:'[]'"`
	DimensionScoresJSON        string     `gorm:"column:dimension_scores_json;type:text;not null;default:'[]'"`
	TotalScore                 float64    `gorm:"type:decimal(6,4);not null;default:0"`
	Passed                     bool       `gorm:"not null;default:false;index"`
	TraceID                    string     `gorm:"type:varchar(128);not null;default:'';index"`
	LastError                  string     `gorm:"type:text"`
	StartedAt                  *time.Time `gorm:"type:timestamp;index"`
	CompletedAt                *time.Time `gorm:"type:timestamp;index"`
	AuditFields
}

// ARWorkInstruction is a versioned procedure bound to an optional product and
// model. Hardware-specific rendering remains an integration concern.
type ARWorkInstruction struct {
	ID                       int64      `gorm:"primaryKey;autoIncrement"`
	TenantID                 int64      `gorm:"type:bigint;not null;index;uniqueIndex:uk_ar_work_instruction_version"`
	PackID                   int64      `gorm:"type:bigint;not null;default:0;index"`
	InstructionCode          string     `gorm:"type:varchar(96);not null;index;uniqueIndex:uk_ar_work_instruction_version"`
	Version                  string     `gorm:"type:varchar(64);not null;uniqueIndex:uk_ar_work_instruction_version"`
	Status                   string     `gorm:"type:varchar(24);not null;default:'draft';index"`
	ProductID                int64      `gorm:"type:bigint;not null;default:0;index"`
	ProductModelID           int64      `gorm:"type:bigint;not null;default:0;index"`
	Title                    string     `gorm:"type:varchar(200);not null"`
	Description              string     `gorm:"type:text"`
	Locale                   string     `gorm:"type:varchar(16);not null;default:'';index"`
	SafetyLevel              string     `gorm:"type:varchar(24);not null;default:'low';index"`
	ApplicableFaultCodesJSON string     `gorm:"column:applicable_fault_codes_json;type:text;not null;default:'[]'"`
	PrerequisitesJSON        string     `gorm:"column:prerequisites_json;type:text;not null;default:'[]'"`
	RequiredToolsJSON        string     `gorm:"column:required_tools_json;type:text;not null;default:'[]'"`
	MediaReferencesJSON      string     `gorm:"column:media_references_json;type:text;not null;default:'[]'"`
	EstimatedMinutes         int        `gorm:"type:int;not null;default:0"`
	Checksum                 string     `gorm:"type:varchar(64);not null;default:''"`
	PublishedAt              *time.Time `gorm:"type:timestamp;index"`
	PublishedByID            int64      `gorm:"type:bigint;not null;default:0;index"`
	PublishedByName          string     `gorm:"type:varchar(100);not null;default:''"`
	AuditFields
}

// ARWorkStep stores ordered guidance, safety acknowledgement, media, and
// completion evidence requirements independently of any AR device SDK.
type ARWorkStep struct {
	ID                                 int64  `gorm:"primaryKey;autoIncrement"`
	TenantID                           int64  `gorm:"type:bigint;not null;index;uniqueIndex:uk_ar_work_step_code;uniqueIndex:uk_ar_work_step_sequence"`
	InstructionID                      int64  `gorm:"type:bigint;not null;index;uniqueIndex:uk_ar_work_step_code;uniqueIndex:uk_ar_work_step_sequence"`
	StepCode                           string `gorm:"type:varchar(96);not null;uniqueIndex:uk_ar_work_step_code"`
	Version                            string `gorm:"type:varchar(64);not null"`
	Status                             string `gorm:"type:varchar(24);not null;default:'draft';index"`
	SequenceNo                         int    `gorm:"type:int;not null;uniqueIndex:uk_ar_work_step_sequence"`
	Title                              string `gorm:"type:varchar(200);not null"`
	Instruction                        string `gorm:"type:text;not null"`
	SafetyWarning                      string `gorm:"type:text"`
	RiskLevel                          string `gorm:"type:varchar(24);not null;default:'low';index"`
	RequiresSafetyAcknowledgement      bool   `gorm:"not null;default:false"`
	MediaReferencesJSON                string `gorm:"column:media_references_json;type:text;not null;default:'[]'"`
	CompletionEvidenceRequirementsJSON string `gorm:"column:completion_evidence_requirements_json;type:text;not null;default:'[]'"`
	VerificationCriteriaJSON           string `gorm:"column:verification_criteria_json;type:text;not null;default:'[]'"`
	EstimatedSeconds                   int    `gorm:"type:int;not null;default:0"`
	AuditFields
}

// IndustrySolutionPackModels exposes this isolated schema slice to focused
// migrations without changing the central model registry.
func IndustrySolutionPackModels() []any {
	return []any{
		&IndustrySolutionPack{},
		&IndustrySolutionPackResource{},
		&IndustrySolutionPackApplication{},
		&IndustrySolutionPackApplicationItem{},
		&DiagnosisEvalCase{},
		&DiagnosisEvalRun{},
		&ARWorkInstruction{},
		&ARWorkStep{},
	}
}

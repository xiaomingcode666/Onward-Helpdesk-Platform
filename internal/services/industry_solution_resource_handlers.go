package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"gorm.io/gorm"
)

const (
	industrySolutionResourceTypeDiagnosisEvalCase = "diagnosis_eval_case"
	industrySolutionResourceTypeARWorkInstruction = "ar_work_instruction"
	industrySolutionResourceTypeFaultTreeNode     = "fault_tree_node"
)

type industrySolutionResourceMarker struct {
	PackCode       string `json:"pack_code"`
	ResourceKey    string `json:"resource_key"`
	Version        string `json:"version"`
	Checksum       string `json:"checksum"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type diagnosisEvalCaseResourcePayload struct {
	Name                   string          `json:"name"`
	Description            string          `json:"description"`
	Status                 string          `json:"status"`
	Locale                 string          `json:"locale"`
	RegionCode             string          `json:"region_code"`
	ProductContext         json.RawMessage `json:"product_context"`
	DeviceContext          json.RawMessage `json:"device_context"`
	InputMessages          json.RawMessage `json:"input_messages"`
	ExpectedOutcome        json.RawMessage `json:"expected_outcome"`
	ExpectedWorkflowBranch json.RawMessage `json:"expected_workflow_branch"`
	ExpectedCitations      json.RawMessage `json:"expected_citations"`
	SafetyBoundary         json.RawMessage `json:"safety_boundary"`
	ForbiddenAnswers       json.RawMessage `json:"forbidden_answers"`
	ScoringDimensions      json.RawMessage `json:"scoring_dimensions"`
	MinimumScore           float64         `json:"minimum_score"`
	EvaluatorVersion       string          `json:"evaluator_version"`
	Tags                   json.RawMessage `json:"tags"`
}

type arWorkInstructionResourcePayload struct {
	Title                string                      `json:"title"`
	Description          string                      `json:"description"`
	Status               string                      `json:"status"`
	Locale               string                      `json:"locale"`
	SafetyLevel          string                      `json:"safety_level"`
	ApplicableFaultCodes json.RawMessage             `json:"applicable_fault_codes"`
	Prerequisites        json.RawMessage             `json:"prerequisites"`
	RequiredTools        json.RawMessage             `json:"required_tools"`
	MediaReferences      json.RawMessage             `json:"media_references"`
	EstimatedMinutes     int                         `json:"estimated_minutes"`
	Steps                []arWorkStepResourcePayload `json:"steps"`
}

type arWorkStepResourcePayload struct {
	StepCode                      string          `json:"step_code"`
	SequenceNo                    int             `json:"sequence_no"`
	Title                         string          `json:"title"`
	Instruction                   string          `json:"instruction"`
	SafetyWarning                 string          `json:"safety_warning"`
	RiskLevel                     string          `json:"risk_level"`
	RequiresSafetyAcknowledgement bool            `json:"requires_safety_acknowledgement"`
	MediaReferences               json.RawMessage `json:"media_references"`
	CompletionEvidence            json.RawMessage `json:"completion_evidence"`
	VerificationCriteria          json.RawMessage `json:"verification_criteria"`
	EstimatedSeconds              int             `json:"estimated_seconds"`
}

type faultTreeNodeResourcePayload struct {
	ParentResourceKey string          `json:"parent_resource_key"`
	Title             string          `json:"title"`
	Description       string          `json:"description"`
	NodeType          string          `json:"node_type"`
	FaultPattern      string          `json:"fault_pattern"`
	TriggerConditions json.RawMessage `json:"trigger_conditions"`
	IsComposite       bool            `json:"is_composite"`
	RiskLevel         string          `json:"risk_level"`
	OrderIndex        int             `json:"order_index"`
	Status            string          `json:"status"`
}

type diagnosisEvalCaseResourceHandler struct{ db *gorm.DB }
type arWorkInstructionResourceHandler struct{ db *gorm.DB }
type faultTreeNodeResourceHandler struct{ db *gorm.DB }

func DefaultIndustrySolutionPackService(db *gorm.DB) *industrySolutionPackService {
	return NewIndustrySolutionPackService(
		db,
		&diagnosisEvalCaseResourceHandler{db: db},
		&arWorkInstructionResourceHandler{db: db},
		&faultTreeNodeResourceHandler{db: db},
	)
}

func (h *diagnosisEvalCaseResourceHandler) ResourceType() string {
	return industrySolutionResourceTypeDiagnosisEvalCase
}

func (h *diagnosisEvalCaseResourceHandler) ValidateManifestResource(resource models.IndustrySolutionPackResource) error {
	_, err := parseDiagnosisEvalCasePayload(resource.PayloadJSON)
	return err
}

func (h *diagnosisEvalCaseResourceHandler) Inspect(_ context.Context, resourceContext IndustrySolutionResourceContext) (IndustrySolutionResourceInspection, error) {
	if err := requireIndustrySolutionTargetProduct(resourceContext); err != nil {
		return IndustrySolutionResourceInspection{}, err
	}
	code, err := industrySolutionTargetResourceCode(h.db, resourceContext, 96)
	if err != nil {
		return IndustrySolutionResourceInspection{}, err
	}
	item, err := repositories.IndustrySolutionPackRepository.FindDiagnosisEvalCase(
		h.db, resourceContext.TenantID, code, resourceContext.TargetProductID, resourceContext.TargetProductModelID,
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return IndustrySolutionResourceInspection{}, nil
	}
	if err != nil {
		return IndustrySolutionResourceInspection{}, err
	}
	marker := diagnosisEvalCaseMarker(item.ProductContextJSON)
	return IndustrySolutionResourceInspection{
		Exists: true, Version: item.Version, Checksum: marker.Checksum,
		ReferenceJSON: mustIndustrySolutionJSON(map[string]any{"eval_case_id": item.ID, "case_code": item.CaseCode}),
	}, nil
}

func (h *diagnosisEvalCaseResourceHandler) Apply(
	_ context.Context,
	resourceContext IndustrySolutionResourceContext,
	action, idempotencyKey string,
) (IndustrySolutionResourceApplyResult, error) {
	if err := requireIndustrySolutionTargetProduct(resourceContext); err != nil {
		return IndustrySolutionResourceApplyResult{}, err
	}
	payload, err := parseDiagnosisEvalCasePayload(resourceContext.Resource.PayloadJSON)
	if err != nil {
		return IndustrySolutionResourceApplyResult{}, err
	}
	code, err := industrySolutionTargetResourceCode(h.db, resourceContext, 96)
	if err != nil {
		return IndustrySolutionResourceApplyResult{}, err
	}
	version := industrySolutionAppliedVersion(resourceContext.Resource, action)
	marker := industrySolutionResourceMarker{
		PackCode: resourceContext.Pack.PackCode, ResourceKey: resourceContext.Resource.ResourceKey,
		Version: version, Checksum: resourceContext.Resource.Checksum, IdempotencyKey: idempotencyKey,
	}
	productContext, err := industrySolutionContextWithMarker(payload.ProductContext, marker)
	if err != nil {
		return IndustrySolutionResourceApplyResult{}, err
	}
	exact, exactErr := repositories.IndustrySolutionPackRepository.FindDiagnosisEvalCaseVersion(h.db, resourceContext.TenantID, code, version)
	if exactErr == nil {
		existingMarker := diagnosisEvalCaseMarker(exact.ProductContextJSON)
		if existingMarker.Checksum == resourceContext.Resource.Checksum || existingMarker.IdempotencyKey == idempotencyKey {
			return industrySolutionAppliedResult("eval_case_id", exact.ID, map[string]any{"case_code": exact.CaseCode, "version": exact.Version}), nil
		}
	} else if !errors.Is(exactErr, gorm.ErrRecordNotFound) {
		return IndustrySolutionResourceApplyResult{}, exactErr
	}

	now := time.Now().UTC()
	audit := industrySolutionAuditFields(resourceContext.Operator, now)
	fields := diagnosisEvalCaseFields(resourceContext, payload, code, version, productContext, audit)
	if action == models.IndustrySolutionResourceActionOverwrite {
		existing, findErr := repositories.IndustrySolutionPackRepository.FindDiagnosisEvalCase(
			h.db, resourceContext.TenantID, code, resourceContext.TargetProductID, resourceContext.TargetProductModelID,
		)
		if findErr != nil {
			return IndustrySolutionResourceApplyResult{}, findErr
		}
		updates := diagnosisEvalCaseUpdateMap(fields)
		if err := repositories.IndustrySolutionPackRepository.UpdateDiagnosisEvalCase(h.db, resourceContext.TenantID, existing.ID, updates); err != nil {
			return IndustrySolutionResourceApplyResult{}, err
		}
		return industrySolutionAppliedResult("eval_case_id", existing.ID, map[string]any{"case_code": code, "version": version}), nil
	}
	if action != models.IndustrySolutionResourceActionCreate && action != models.IndustrySolutionResourceActionCreateNewVersion {
		return IndustrySolutionResourceApplyResult{}, errorsx.InvalidParam("unsupported diagnosis evaluation resource action")
	}
	if err := repositories.IndustrySolutionPackRepository.CreateDiagnosisEvalCase(h.db, &fields); err != nil {
		return IndustrySolutionResourceApplyResult{}, err
	}
	return industrySolutionAppliedResult("eval_case_id", fields.ID, map[string]any{"case_code": code, "version": version}), nil
}

func (h *arWorkInstructionResourceHandler) ResourceType() string {
	return industrySolutionResourceTypeARWorkInstruction
}

func (h *arWorkInstructionResourceHandler) ValidateManifestResource(resource models.IndustrySolutionPackResource) error {
	_, err := parseARWorkInstructionPayload(resource.PayloadJSON)
	return err
}

func (h *arWorkInstructionResourceHandler) Inspect(_ context.Context, resourceContext IndustrySolutionResourceContext) (IndustrySolutionResourceInspection, error) {
	if err := requireIndustrySolutionTargetProduct(resourceContext); err != nil {
		return IndustrySolutionResourceInspection{}, err
	}
	code, err := industrySolutionTargetResourceCode(h.db, resourceContext, 96)
	if err != nil {
		return IndustrySolutionResourceInspection{}, err
	}
	item, err := repositories.IndustrySolutionPackRepository.FindARWorkInstruction(
		h.db, resourceContext.TenantID, code, resourceContext.TargetProductID, resourceContext.TargetProductModelID,
	)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return IndustrySolutionResourceInspection{}, nil
	}
	if err != nil {
		return IndustrySolutionResourceInspection{}, err
	}
	return IndustrySolutionResourceInspection{
		Exists: true, Version: item.Version, Checksum: item.Checksum,
		ReferenceJSON: mustIndustrySolutionJSON(map[string]any{"instruction_id": item.ID, "instruction_code": item.InstructionCode}),
	}, nil
}

func (h *arWorkInstructionResourceHandler) Apply(
	_ context.Context,
	resourceContext IndustrySolutionResourceContext,
	action, idempotencyKey string,
) (IndustrySolutionResourceApplyResult, error) {
	if err := requireIndustrySolutionTargetProduct(resourceContext); err != nil {
		return IndustrySolutionResourceApplyResult{}, err
	}
	payload, err := parseARWorkInstructionPayload(resourceContext.Resource.PayloadJSON)
	if err != nil {
		return IndustrySolutionResourceApplyResult{}, err
	}
	code, err := industrySolutionTargetResourceCode(h.db, resourceContext, 96)
	if err != nil {
		return IndustrySolutionResourceApplyResult{}, err
	}
	version := industrySolutionAppliedVersion(resourceContext.Resource, action)
	exact, exactErr := repositories.IndustrySolutionPackRepository.FindARWorkInstructionVersion(h.db, resourceContext.TenantID, code, version)
	if exactErr == nil && exact.Checksum == resourceContext.Resource.Checksum {
		return industrySolutionAppliedResult("instruction_id", exact.ID, map[string]any{"instruction_code": code, "version": version, "idempotency_key": idempotencyKey}), nil
	}
	if exactErr != nil && !errors.Is(exactErr, gorm.ErrRecordNotFound) {
		return IndustrySolutionResourceApplyResult{}, exactErr
	}
	now := time.Now().UTC()
	audit := industrySolutionAuditFields(resourceContext.Operator, now)
	instruction, steps := arWorkInstructionFields(resourceContext, payload, code, version, audit)
	if action == models.IndustrySolutionResourceActionOverwrite {
		existing, findErr := repositories.IndustrySolutionPackRepository.FindARWorkInstruction(
			h.db, resourceContext.TenantID, code, resourceContext.TargetProductID, resourceContext.TargetProductModelID,
		)
		if findErr != nil {
			return IndustrySolutionResourceApplyResult{}, findErr
		}
		if err := repositories.IndustrySolutionPackRepository.ReplaceARWorkInstruction(
			h.db, resourceContext.TenantID, existing.ID, arWorkInstructionUpdateMap(instruction), steps,
		); err != nil {
			return IndustrySolutionResourceApplyResult{}, err
		}
		return industrySolutionAppliedResult("instruction_id", existing.ID, map[string]any{"instruction_code": code, "version": version}), nil
	}
	if action != models.IndustrySolutionResourceActionCreate && action != models.IndustrySolutionResourceActionCreateNewVersion {
		return IndustrySolutionResourceApplyResult{}, errorsx.InvalidParam("unsupported AR instruction resource action")
	}
	if err := repositories.IndustrySolutionPackRepository.CreateARWorkInstructionWithSteps(h.db, &instruction, steps); err != nil {
		return IndustrySolutionResourceApplyResult{}, err
	}
	return industrySolutionAppliedResult("instruction_id", instruction.ID, map[string]any{"instruction_code": code, "version": version}), nil
}

func (h *faultTreeNodeResourceHandler) ResourceType() string {
	return industrySolutionResourceTypeFaultTreeNode
}

func (h *faultTreeNodeResourceHandler) ValidateManifestResource(resource models.IndustrySolutionPackResource) error {
	payload, err := parseFaultTreeNodePayload(resource.PayloadJSON)
	if err != nil {
		return err
	}
	if payload.ParentResourceKey != "" {
		dependencies, _ := parseIndustrySolutionDependencies(resource.DependenciesJSON)
		expected := industrySolutionResourceTypeFaultTreeNode + ":" + payload.ParentResourceKey
		found := false
		for _, dependency := range dependencies {
			if dependency == expected {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("parent_resource_key requires dependency %s", expected)
		}
	}
	return nil
}

func (h *faultTreeNodeResourceHandler) Inspect(_ context.Context, resourceContext IndustrySolutionResourceContext) (IndustrySolutionResourceInspection, error) {
	if err := requireIndustrySolutionTargetProduct(resourceContext); err != nil {
		return IndustrySolutionResourceInspection{}, err
	}
	node, marker, err := h.findManagedNode(resourceContext, resourceContext.Resource.ResourceKey, "")
	if err != nil {
		return IndustrySolutionResourceInspection{}, err
	}
	if node == nil {
		return IndustrySolutionResourceInspection{}, nil
	}
	return IndustrySolutionResourceInspection{
		Exists: true, Version: marker.Version, Checksum: marker.Checksum,
		ReferenceJSON: mustIndustrySolutionJSON(map[string]any{"node_id": node.ID}),
	}, nil
}

func (h *faultTreeNodeResourceHandler) Apply(
	_ context.Context,
	resourceContext IndustrySolutionResourceContext,
	action, idempotencyKey string,
) (IndustrySolutionResourceApplyResult, error) {
	if err := requireIndustrySolutionTargetProduct(resourceContext); err != nil {
		return IndustrySolutionResourceApplyResult{}, err
	}
	payload, err := parseFaultTreeNodePayload(resourceContext.Resource.PayloadJSON)
	if err != nil {
		return IndustrySolutionResourceApplyResult{}, err
	}
	if exact, marker, err := h.findManagedNode(resourceContext, resourceContext.Resource.ResourceKey, idempotencyKey); err != nil {
		return IndustrySolutionResourceApplyResult{}, err
	} else if exact != nil && (marker.IdempotencyKey == idempotencyKey || marker.Checksum == resourceContext.Resource.Checksum) {
		return industrySolutionAppliedResult("node_id", exact.ID, map[string]any{"version": marker.Version}), nil
	}
	version := industrySolutionAppliedVersion(resourceContext.Resource, action)
	marker := industrySolutionResourceMarker{
		PackCode: resourceContext.Pack.PackCode, ResourceKey: resourceContext.Resource.ResourceKey,
		Version: version, Checksum: resourceContext.Resource.Checksum, IdempotencyKey: idempotencyKey,
	}
	triggerConditions, err := managedFaultTreeConditions(payload.TriggerConditions, marker)
	if err != nil {
		return IndustrySolutionResourceApplyResult{}, err
	}
	parentID := ""
	if payload.ParentResourceKey != "" {
		parent, _, findErr := h.findManagedNode(resourceContext, payload.ParentResourceKey, "")
		if findErr != nil {
			return IndustrySolutionResourceApplyResult{}, findErr
		}
		if parent == nil {
			return IndustrySolutionResourceApplyResult{}, errorsx.InvalidParam("parent fault tree resource was not applied")
		}
		parentID = parent.ID
	}
	existing, existingMarker, err := h.findManagedNode(resourceContext, resourceContext.Resource.ResourceKey, "")
	if err != nil {
		return IndustrySolutionResourceApplyResult{}, err
	}
	if action == models.IndustrySolutionResourceActionCreate && existing != nil {
		if existingMarker.Checksum == resourceContext.Resource.Checksum {
			return industrySolutionAppliedResult("node_id", existing.ID, map[string]any{"version": existingMarker.Version}), nil
		}
		return IndustrySolutionResourceApplyResult{}, errorsx.InvalidParam("managed fault tree resource already exists")
	}
	if action == models.IndustrySolutionResourceActionOverwrite {
		if existing == nil {
			return IndustrySolutionResourceApplyResult{}, errorsx.InvalidParam("managed fault tree resource to overwrite was not found")
		}
		status := payload.Status
		update := dto.EnterpriseFaultTreeNodeUpdateRequest{
			ParentID: &parentID, Title: &payload.Title, Description: &payload.Description, NodeType: &payload.NodeType,
			FaultPattern: &payload.FaultPattern, TriggerConditions: &triggerConditions, IsComposite: &payload.IsComposite,
			RiskLevel: &payload.RiskLevel, OrderIndex: &payload.OrderIndex, Status: &status,
		}
		updated, err := FaultTreeService.UpdateManagedNode(resourceContext.TenantID, existing.ID, update)
		if err != nil {
			return IndustrySolutionResourceApplyResult{}, err
		}
		return industrySolutionAppliedResult("node_id", updated.ID, map[string]any{"version": version}), nil
	}
	if action != models.IndustrySolutionResourceActionCreate && action != models.IndustrySolutionResourceActionCreateNewVersion {
		return IndustrySolutionResourceApplyResult{}, errorsx.InvalidParam("unsupported fault tree resource action")
	}
	created, err := FaultTreeService.CreateManagedNode(resourceContext.TenantID, dto.EnterpriseFaultTreeNodeCreateRequest{
		ProductID: resourceContext.TargetProductID, ParentID: parentID, Title: payload.Title, Description: payload.Description,
		NodeType: payload.NodeType, FaultPattern: payload.FaultPattern, TriggerConditions: triggerConditions,
		IsComposite: payload.IsComposite, RiskLevel: payload.RiskLevel, OrderIndex: payload.OrderIndex,
	})
	if err != nil {
		return IndustrySolutionResourceApplyResult{}, err
	}
	if payload.Status != models.IndustrySolutionResourceStatusDraft {
		status := payload.Status
		created, err = FaultTreeService.UpdateManagedNode(resourceContext.TenantID, created.ID, dto.EnterpriseFaultTreeNodeUpdateRequest{Status: &status})
		if err != nil {
			return IndustrySolutionResourceApplyResult{}, err
		}
	}
	return industrySolutionAppliedResult("node_id", created.ID, map[string]any{"version": version}), nil
}

func (h *faultTreeNodeResourceHandler) findManagedNode(
	resourceContext IndustrySolutionResourceContext,
	resourceKey, idempotencyKey string,
) (*models.FaultTreeNode, industrySolutionResourceMarker, error) {
	rows, err := repositories.FaultTreeRepository.ListByTenantProduct(
		h.db, resourceContext.TenantID, strconv.FormatInt(resourceContext.TargetProductID, 10),
	)
	if err != nil {
		return nil, industrySolutionResourceMarker{}, err
	}
	for index := len(rows) - 1; index >= 0; index-- {
		marker := faultTreeNodeMarker(rows[index].TriggerConditions)
		if marker.PackCode != resourceContext.Pack.PackCode || marker.ResourceKey != resourceKey {
			continue
		}
		if idempotencyKey != "" && marker.IdempotencyKey != idempotencyKey {
			continue
		}
		return &rows[index], marker, nil
	}
	return nil, industrySolutionResourceMarker{}, nil
}

func parseDiagnosisEvalCasePayload(value string) (diagnosisEvalCaseResourcePayload, error) {
	payload := diagnosisEvalCaseResourcePayload{}
	if err := decodeIndustrySolutionResourcePayload(value, &payload); err != nil {
		return payload, err
	}
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		return payload, errors.New("diagnosis evaluation name is required")
	}
	payload.Status = firstNonBlank(strings.TrimSpace(payload.Status), models.DiagnosisEvalCaseStatusActive)
	if payload.Status != models.DiagnosisEvalCaseStatusDraft && payload.Status != models.DiagnosisEvalCaseStatusActive && payload.Status != models.DiagnosisEvalCaseStatusArchived {
		return payload, errors.New("invalid diagnosis evaluation status")
	}
	if payload.MinimumScore < 0 || payload.MinimumScore > 1 {
		return payload, errors.New("minimum_score must be between 0 and 1")
	}
	var err error
	if payload.ProductContext, err = normalizeIndustrySolutionRawJSON(payload.ProductContext, `{}`, false); err != nil {
		return payload, fmt.Errorf("product_context must be a JSON object: %w", err)
	}
	if payload.DeviceContext, err = normalizeIndustrySolutionRawJSON(payload.DeviceContext, `{}`, false); err != nil {
		return payload, fmt.Errorf("device_context must be a JSON object: %w", err)
	}
	for name, field := range map[string]*json.RawMessage{
		"input_messages": &payload.InputMessages, "expected_workflow_branch": &payload.ExpectedWorkflowBranch,
		"expected_citations": &payload.ExpectedCitations, "forbidden_answers": &payload.ForbiddenAnswers,
		"scoring_dimensions": &payload.ScoringDimensions, "tags": &payload.Tags,
	} {
		if *field, err = normalizeIndustrySolutionRawJSON(*field, `[]`, true); err != nil {
			return payload, fmt.Errorf("%s must be a JSON array: %w", name, err)
		}
	}
	for name, field := range map[string]*json.RawMessage{
		"expected_outcome": &payload.ExpectedOutcome, "safety_boundary": &payload.SafetyBoundary,
	} {
		if *field, err = normalizeIndustrySolutionRawJSON(*field, `{}`, false); err != nil {
			return payload, fmt.Errorf("%s must be a JSON object: %w", name, err)
		}
	}
	payload.EvaluatorVersion = firstNonBlank(strings.TrimSpace(payload.EvaluatorVersion), "1.0.0")
	return payload, nil
}

func parseARWorkInstructionPayload(value string) (arWorkInstructionResourcePayload, error) {
	payload := arWorkInstructionResourcePayload{}
	if err := decodeIndustrySolutionResourcePayload(value, &payload); err != nil {
		return payload, err
	}
	payload.Title = strings.TrimSpace(payload.Title)
	if payload.Title == "" {
		return payload, errors.New("AR instruction title is required")
	}
	payload.Status = firstNonBlank(strings.TrimSpace(payload.Status), models.ARWorkInstructionStatusPublished)
	if payload.Status != models.ARWorkInstructionStatusDraft && payload.Status != models.ARWorkInstructionStatusPublished && payload.Status != models.ARWorkInstructionStatusArchived {
		return payload, errors.New("invalid AR instruction status")
	}
	if payload.EstimatedMinutes < 0 {
		return payload, errors.New("estimated_minutes cannot be negative")
	}
	var err error
	for name, field := range map[string]*json.RawMessage{
		"applicable_fault_codes": &payload.ApplicableFaultCodes, "prerequisites": &payload.Prerequisites,
		"required_tools": &payload.RequiredTools, "media_references": &payload.MediaReferences,
	} {
		if *field, err = normalizeIndustrySolutionRawJSON(*field, `[]`, true); err != nil {
			return payload, fmt.Errorf("%s must be a JSON array: %w", name, err)
		}
	}
	seenCodes := map[string]bool{}
	seenSequence := map[int]bool{}
	for i := range payload.Steps {
		step := &payload.Steps[i]
		step.StepCode = strings.TrimSpace(step.StepCode)
		step.Title = strings.TrimSpace(step.Title)
		step.Instruction = strings.TrimSpace(step.Instruction)
		if !industrySolutionResourceComponentPattern.MatchString(step.StepCode) || step.Title == "" || step.Instruction == "" {
			return payload, errors.New("each AR step requires a stable step_code, title, and instruction")
		}
		if step.SequenceNo <= 0 || seenCodes[step.StepCode] || seenSequence[step.SequenceNo] {
			return payload, errors.New("AR step_code and positive sequence_no must be unique")
		}
		if step.EstimatedSeconds < 0 {
			return payload, errors.New("AR step estimated_seconds cannot be negative")
		}
		seenCodes[step.StepCode] = true
		seenSequence[step.SequenceNo] = true
		step.RiskLevel = firstNonBlank(strings.TrimSpace(step.RiskLevel), "low")
		for name, field := range map[string]*json.RawMessage{
			"media_references": &step.MediaReferences, "completion_evidence": &step.CompletionEvidence,
			"verification_criteria": &step.VerificationCriteria,
		} {
			if *field, err = normalizeIndustrySolutionRawJSON(*field, `[]`, true); err != nil {
				return payload, fmt.Errorf("AR step %s %s must be a JSON array: %w", step.StepCode, name, err)
			}
		}
	}
	if len(payload.Steps) == 0 {
		return payload, errors.New("AR instruction requires at least one step")
	}
	payload.SafetyLevel = firstNonBlank(strings.TrimSpace(payload.SafetyLevel), "low")
	return payload, nil
}

func parseFaultTreeNodePayload(value string) (faultTreeNodeResourcePayload, error) {
	payload := faultTreeNodeResourcePayload{}
	if err := decodeIndustrySolutionResourcePayload(value, &payload); err != nil {
		return payload, err
	}
	payload.ParentResourceKey = strings.TrimSpace(payload.ParentResourceKey)
	if payload.ParentResourceKey != "" && !industrySolutionResourceComponentPattern.MatchString(payload.ParentResourceKey) {
		return payload, errors.New("parent_resource_key must be a stable resource key")
	}
	payload.Title = strings.TrimSpace(payload.Title)
	payload.NodeType = strings.TrimSpace(payload.NodeType)
	if payload.Title == "" || !faultTreeNodeTypes[payload.NodeType] {
		return payload, errors.New("fault tree title and valid node_type are required")
	}
	payload.FaultPattern = firstNonBlank(strings.TrimSpace(payload.FaultPattern), "persistent")
	payload.RiskLevel = firstNonBlank(strings.TrimSpace(payload.RiskLevel), "low")
	payload.Status = firstNonBlank(strings.TrimSpace(payload.Status), "published")
	if !faultTreePatterns[payload.FaultPattern] || !faultTreeRiskLevels[payload.RiskLevel] || !faultTreeStatuses[payload.Status] {
		return payload, errors.New("fault tree pattern, risk level, or status is invalid")
	}
	conditions, err := normalizeIndustrySolutionRawJSONAny(payload.TriggerConditions, `[]`)
	if err != nil {
		return payload, fmt.Errorf("trigger_conditions must be valid JSON: %w", err)
	}
	payload.TriggerConditions = conditions
	return payload, nil
}

func decodeIndustrySolutionResourcePayload(value string, target any) error {
	decoder := json.NewDecoder(bytes.NewBufferString(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("resource payload must contain one JSON object")
	}
	return nil
}

func normalizeIndustrySolutionRawJSON(value json.RawMessage, fallback string, array bool) (json.RawMessage, error) {
	value = bytes.TrimSpace(value)
	if len(value) == 0 || bytes.Equal(value, []byte("null")) {
		value = json.RawMessage(fallback)
	}
	var target any
	if array {
		target = &[]any{}
	} else {
		target = &map[string]any{}
	}
	if err := json.Unmarshal(value, target); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(target)
	return encoded, err
}

func normalizeIndustrySolutionRawJSONAny(value json.RawMessage, fallback string) (json.RawMessage, error) {
	value = bytes.TrimSpace(value)
	if len(value) == 0 || bytes.Equal(value, []byte("null")) {
		value = json.RawMessage(fallback)
	}
	var target any
	if err := json.Unmarshal(value, &target); err != nil {
		return nil, err
	}
	switch target.(type) {
	case map[string]any, []any:
	default:
		return nil, errors.New("expected JSON object or array")
	}
	encoded, err := json.Marshal(target)
	return encoded, err
}

func diagnosisEvalCaseFields(
	resourceContext IndustrySolutionResourceContext,
	payload diagnosisEvalCaseResourcePayload,
	code, version, productContext string,
	audit models.AuditFields,
) models.DiagnosisEvalCase {
	return models.DiagnosisEvalCase{
		TenantID: resourceContext.TenantID, PackID: resourceContext.Pack.ID, CaseCode: code, Name: payload.Name,
		Description: strings.TrimSpace(payload.Description), Version: version, Status: payload.Status,
		ProductID: resourceContext.TargetProductID, ProductModelID: resourceContext.TargetProductModelID,
		Locale: firstNonBlank(strings.TrimSpace(payload.Locale), resourceContext.Pack.DefaultLocale), RegionCode: strings.TrimSpace(payload.RegionCode),
		ProductContextJSON: productContext, DeviceContextJSON: string(payload.DeviceContext),
		InputMessagesJSON: string(payload.InputMessages), ExpectedOutcomeJSON: string(payload.ExpectedOutcome),
		ExpectedWorkflowBranchJSON: string(payload.ExpectedWorkflowBranch), ExpectedCitationsJSON: string(payload.ExpectedCitations),
		SafetyBoundaryJSON: string(payload.SafetyBoundary), ForbiddenAnswersJSON: string(payload.ForbiddenAnswers),
		ScoringDimensionsJSON: string(payload.ScoringDimensions), MinimumScore: payload.MinimumScore,
		EvaluatorVersion: payload.EvaluatorVersion, TagsJSON: string(payload.Tags), AuditFields: audit,
	}
}

func diagnosisEvalCaseUpdateMap(item models.DiagnosisEvalCase) map[string]any {
	return map[string]any{
		"pack_id": item.PackID, "name": item.Name, "description": item.Description, "version": item.Version,
		"status": item.Status, "product_id": item.ProductID, "product_model_id": item.ProductModelID,
		"locale": item.Locale, "region_code": item.RegionCode, "product_context_json": item.ProductContextJSON,
		"device_context_json": item.DeviceContextJSON, "input_messages_json": item.InputMessagesJSON,
		"expected_outcome_json": item.ExpectedOutcomeJSON, "expected_workflow_branch_json": item.ExpectedWorkflowBranchJSON,
		"expected_citations_json": item.ExpectedCitationsJSON, "safety_boundary_json": item.SafetyBoundaryJSON,
		"forbidden_answers_json": item.ForbiddenAnswersJSON, "scoring_dimensions_json": item.ScoringDimensionsJSON,
		"minimum_score": item.MinimumScore, "evaluator_version": item.EvaluatorVersion, "tags_json": item.TagsJSON,
		"update_user_id": item.UpdateUserID, "update_user_name": item.UpdateUserName, "updated_at": item.UpdatedAt,
	}
}

func arWorkInstructionFields(
	resourceContext IndustrySolutionResourceContext,
	payload arWorkInstructionResourcePayload,
	code, version string,
	audit models.AuditFields,
) (models.ARWorkInstruction, []models.ARWorkStep) {
	instruction := models.ARWorkInstruction{
		TenantID: resourceContext.TenantID, PackID: resourceContext.Pack.ID, InstructionCode: code, Version: version,
		Status: payload.Status, ProductID: resourceContext.TargetProductID, ProductModelID: resourceContext.TargetProductModelID,
		Title: payload.Title, Description: strings.TrimSpace(payload.Description),
		Locale:      firstNonBlank(strings.TrimSpace(payload.Locale), resourceContext.Pack.DefaultLocale),
		SafetyLevel: payload.SafetyLevel, ApplicableFaultCodesJSON: string(payload.ApplicableFaultCodes),
		PrerequisitesJSON: string(payload.Prerequisites), RequiredToolsJSON: string(payload.RequiredTools),
		MediaReferencesJSON: string(payload.MediaReferences), EstimatedMinutes: payload.EstimatedMinutes,
		Checksum: resourceContext.Resource.Checksum, AuditFields: audit,
	}
	if payload.Status == models.ARWorkInstructionStatusPublished {
		publishedAt := audit.UpdatedAt
		instruction.PublishedAt = &publishedAt
		instruction.PublishedByID = resourceContext.Operator.UserID
		instruction.PublishedByName = industrySolutionOperatorName(resourceContext.Operator)
	}
	steps := make([]models.ARWorkStep, 0, len(payload.Steps))
	for _, input := range payload.Steps {
		status := models.ARWorkStepStatusDraft
		if payload.Status == models.ARWorkInstructionStatusPublished {
			status = models.ARWorkStepStatusPublished
		} else if payload.Status == models.ARWorkInstructionStatusArchived {
			status = models.ARWorkStepStatusArchived
		}
		steps = append(steps, models.ARWorkStep{
			TenantID: resourceContext.TenantID, StepCode: input.StepCode, Version: version, Status: status,
			SequenceNo: input.SequenceNo, Title: input.Title, Instruction: input.Instruction,
			SafetyWarning: strings.TrimSpace(input.SafetyWarning), RiskLevel: input.RiskLevel,
			RequiresSafetyAcknowledgement:      input.RequiresSafetyAcknowledgement,
			MediaReferencesJSON:                string(input.MediaReferences),
			CompletionEvidenceRequirementsJSON: string(input.CompletionEvidence),
			VerificationCriteriaJSON:           string(input.VerificationCriteria), EstimatedSeconds: input.EstimatedSeconds,
			AuditFields: audit,
		})
	}
	return instruction, steps
}

func arWorkInstructionUpdateMap(item models.ARWorkInstruction) map[string]any {
	return map[string]any{
		"pack_id": item.PackID, "version": item.Version, "status": item.Status, "product_id": item.ProductID,
		"product_model_id": item.ProductModelID, "title": item.Title, "description": item.Description,
		"locale": item.Locale, "safety_level": item.SafetyLevel, "applicable_fault_codes_json": item.ApplicableFaultCodesJSON,
		"prerequisites_json": item.PrerequisitesJSON, "required_tools_json": item.RequiredToolsJSON,
		"media_references_json": item.MediaReferencesJSON, "estimated_minutes": item.EstimatedMinutes,
		"checksum": item.Checksum, "published_at": item.PublishedAt, "published_by_id": item.PublishedByID,
		"published_by_name": item.PublishedByName, "update_user_id": item.UpdateUserID,
		"update_user_name": item.UpdateUserName, "updated_at": item.UpdatedAt,
	}
}

func requireIndustrySolutionTargetProduct(resourceContext IndustrySolutionResourceContext) error {
	if resourceContext.TargetProductID <= 0 {
		return errorsx.InvalidParam("target_product_id is required for industry solution resources")
	}
	return nil
}

func industrySolutionTargetResourceCode(db *gorm.DB, resourceContext IndustrySolutionResourceContext, maxLength int) (string, error) {
	product := repositories.ProductRepository.GetByTenant(db, resourceContext.TargetProductID, resourceContext.TenantID)
	if product == nil {
		return "", errorsx.InvalidParam("target product not found")
	}
	parts := []string{resourceContext.Resource.ResourceKey, product.Code}
	if resourceContext.TargetProductModelID > 0 {
		model := repositories.ProductModelRepository.Get(db, resourceContext.TargetProductModelID)
		if model == nil || model.TenantID != resourceContext.TenantID || model.ProductID != product.ID {
			return "", errorsx.InvalidParam("target product model not found")
		}
		parts = append(parts, model.ModelCode)
	}
	code := strings.Join(parts, "@")
	if len(code) <= maxLength {
		return code, nil
	}
	sum := sha256String(code)
	keep := maxLength - len(sum[:12]) - 1
	if keep < 1 {
		return "", errors.New("managed resource code length is too small")
	}
	return code[:keep] + "@" + sum[:12], nil
}

func industrySolutionAppliedVersion(resource models.IndustrySolutionPackResource, action string) string {
	if action != models.IndustrySolutionResourceActionCreateNewVersion {
		return resource.Version
	}
	short := resource.Checksum
	if len(short) > 12 {
		short = short[:12]
	}
	parts := strings.SplitN(resource.Version, "+", 2)
	if len(parts) == 2 {
		return parts[0] + "+" + parts[1] + ".rhd." + short
	}
	return resource.Version + "+rhd." + short
}

func industrySolutionContextWithMarker(value json.RawMessage, marker industrySolutionResourceMarker) (string, error) {
	contextValue := map[string]any{}
	if err := json.Unmarshal(value, &contextValue); err != nil {
		return "", err
	}
	contextValue["_solution_pack"] = marker
	encoded, err := json.Marshal(contextValue)
	return string(encoded), err
}

func diagnosisEvalCaseMarker(value string) industrySolutionResourceMarker {
	contextValue := map[string]json.RawMessage{}
	if json.Unmarshal([]byte(value), &contextValue) != nil {
		return industrySolutionResourceMarker{}
	}
	var marker industrySolutionResourceMarker
	_ = json.Unmarshal(contextValue["_solution_pack"], &marker)
	return marker
}

func managedFaultTreeConditions(value json.RawMessage, marker industrySolutionResourceMarker) (string, error) {
	encoded, err := json.Marshal(map[string]any{"conditions": value, "_solution_pack": marker})
	return string(encoded), err
}

func faultTreeNodeMarker(value string) industrySolutionResourceMarker {
	container := map[string]json.RawMessage{}
	if json.Unmarshal([]byte(value), &container) != nil {
		return industrySolutionResourceMarker{}
	}
	var marker industrySolutionResourceMarker
	_ = json.Unmarshal(container["_solution_pack"], &marker)
	return marker
}

func industrySolutionAppliedResult(referenceKey string, referenceValue any, result map[string]any) IndustrySolutionResourceApplyResult {
	return IndustrySolutionResourceApplyResult{
		ReferenceJSON: mustIndustrySolutionJSON(map[string]any{referenceKey: referenceValue}),
		ResultJSON:    mustIndustrySolutionJSON(result),
	}
}

func mustIndustrySolutionJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func sha256String(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:])
}

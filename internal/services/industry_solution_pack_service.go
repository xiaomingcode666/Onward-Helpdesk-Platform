package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	industrySolutionApplicationVersion       = "1.0.0"
	industrySolutionApplicationLeaseDuration = 10 * time.Minute
)

var (
	industrySolutionVersionPattern           = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	industrySolutionKeyPattern               = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`)
	industrySolutionResourceComponentPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$`)
)

type IndustrySolutionPackValidationIssue struct {
	Severity     string `json:"severity"`
	Code         string `json:"code"`
	Message      string `json:"message"`
	ResourceID   int64  `json:"resourceId,omitempty"`
	ResourceType string `json:"resourceType,omitempty"`
	ResourceKey  string `json:"resourceKey,omitempty"`
}

type IndustrySolutionPackValidationResult struct {
	Valid                  bool                                  `json:"valid"`
	Pack                   models.IndustrySolutionPack           `json:"pack"`
	Resources              []models.IndustrySolutionPackResource `json:"resources"`
	CalculatedManifestHash string                                `json:"calculatedManifestHash"`
	Issues                 []IndustrySolutionPackValidationIssue `json:"issues"`
}

type IndustrySolutionPackApplyRequest struct {
	TenantID             int64
	PackID               int64
	TargetProductID      int64
	TargetProductModelID int64
	TargetContextJSON    string
	ConflictStrategy     string
	IdempotencyKey       string
}

type IndustrySolutionResourceContext struct {
	TenantID             int64
	Pack                 models.IndustrySolutionPack
	Resource             models.IndustrySolutionPackResource
	TargetProductID      int64
	TargetProductModelID int64
	TargetContextJSON    string
	Operator             *dto.AuthPrincipal
}

type IndustrySolutionResourceInspection struct {
	Exists        bool
	Version       string
	Checksum      string
	ReferenceJSON string
}

type IndustrySolutionResourceApplyResult struct {
	ReferenceJSON string
	ResultJSON    string
}

// IndustrySolutionResourceHandler adapts a manifest resource to an existing
// domain service. Implementations must honor the supplied idempotency key and
// resolve stable resource keys inside context.TenantID; manifest references
// must never be cast directly to tenant-local database IDs.
type IndustrySolutionResourceHandler interface {
	ResourceType() string
	Inspect(context.Context, IndustrySolutionResourceContext) (IndustrySolutionResourceInspection, error)
	Apply(context.Context, IndustrySolutionResourceContext, string, string) (IndustrySolutionResourceApplyResult, error)
}

// IndustrySolutionResourceManifestValidator lets a concrete handler reject a
// structurally valid but domain-invalid payload before a pack is published.
type IndustrySolutionResourceManifestValidator interface {
	ValidateManifestResource(models.IndustrySolutionPackResource) error
}

type IndustrySolutionPackExecutionResult struct {
	Application models.IndustrySolutionPackApplication       `json:"application"`
	Items       []models.IndustrySolutionPackApplicationItem `json:"items"`
	Validation  IndustrySolutionPackValidationResult         `json:"validation"`
	Reused      bool                                         `json:"reused"`
}

type industrySolutionPackService struct {
	db       *gorm.DB
	handlers map[string]IndustrySolutionResourceHandler
	now      func() time.Time
}

func NewIndustrySolutionPackService(db *gorm.DB, handlers ...IndustrySolutionResourceHandler) *industrySolutionPackService {
	service := &industrySolutionPackService{
		db:       db,
		handlers: make(map[string]IndustrySolutionResourceHandler, len(handlers)),
		now:      time.Now,
	}
	for _, handler := range handlers {
		if handler == nil {
			continue
		}
		resourceType := strings.TrimSpace(handler.ResourceType())
		if resourceType != "" {
			service.handlers[resourceType] = handler
		}
	}
	return service
}

func (s *industrySolutionPackService) Validate(
	ctx context.Context,
	tenantID, packID int64,
	operator *dto.AuthPrincipal,
) (*IndustrySolutionPackValidationResult, error) {
	if err := s.authorize(tenantID, operator); err != nil {
		return nil, err
	}
	pack, resources, err := s.loadPack(ctx, tenantID, packID)
	if err != nil {
		return nil, err
	}
	result := s.validatePack(*pack, resources)
	return &result, nil
}

func (s *industrySolutionPackService) DryRun(
	ctx context.Context,
	req IndustrySolutionPackApplyRequest,
	operator *dto.AuthPrincipal,
) (*IndustrySolutionPackExecutionResult, error) {
	return s.execute(ctx, models.IndustrySolutionApplicationModeDryRun, req, operator)
}

func (s *industrySolutionPackService) Apply(
	ctx context.Context,
	req IndustrySolutionPackApplyRequest,
	operator *dto.AuthPrincipal,
) (*IndustrySolutionPackExecutionResult, error) {
	return s.execute(ctx, models.IndustrySolutionApplicationModeApply, req, operator)
}

func (s *industrySolutionPackService) execute(
	ctx context.Context,
	mode string,
	req IndustrySolutionPackApplyRequest,
	operator *dto.AuthPrincipal,
) (*IndustrySolutionPackExecutionResult, error) {
	if err := s.authorize(req.TenantID, operator); err != nil {
		return nil, err
	}
	normalizedReq, err := normalizeIndustrySolutionApplyRequest(req)
	if err != nil {
		return nil, err
	}
	pack, resources, err := s.loadPack(ctx, normalizedReq.TenantID, normalizedReq.PackID)
	if err != nil {
		return nil, err
	}
	validation := s.validatePack(*pack, resources)
	requestHash := industrySolutionApplicationRequestHash(mode, normalizedReq, *pack)

	existing, err := repositories.IndustrySolutionPackRepository.FindApplicationByKey(
		s.db.WithContext(ctx),
		normalizedReq.TenantID,
		normalizedReq.IdempotencyKey,
	)
	if err == nil {
		if existing.RequestHash != requestHash {
			return nil, errorsx.InvalidParam("idempotency key was already used for a different solution pack request")
		}
		return s.resumeOrReturn(ctx, mode, normalizedReq, *pack, resources, validation, existing, operator)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if !validation.Valid {
		return nil, errorsx.InvalidParam(firstIndustrySolutionValidationError(validation))
	}
	if pack.Status != models.IndustrySolutionPackStatusPublished {
		return nil, errorsx.InvalidParam("only published industry solution packs can be applied")
	}
	if err := s.validateTargetScope(normalizedReq); err != nil {
		return nil, err
	}

	items, blocked := s.planResources(ctx, *pack, resources, normalizedReq, operator)
	now := s.now().UTC()
	status := models.IndustrySolutionApplicationStatusPending
	var completedAt *time.Time
	if blocked {
		status = models.IndustrySolutionApplicationStatusBlocked
		completedAt = &now
	} else if mode == models.IndustrySolutionApplicationModeDryRun {
		status = models.IndustrySolutionApplicationStatusReady
		completedAt = &now
	}
	application := models.IndustrySolutionPackApplication{
		TenantID:             normalizedReq.TenantID,
		PackID:               pack.ID,
		PackCode:             pack.PackCode,
		PackVersion:          pack.Version,
		Version:              industrySolutionApplicationVersion,
		TargetProductID:      normalizedReq.TargetProductID,
		TargetProductModelID: normalizedReq.TargetProductModelID,
		TargetContextJSON:    normalizedReq.TargetContextJSON,
		Mode:                 mode,
		ConflictStrategy:     normalizedReq.ConflictStrategy,
		IdempotencyKey:       normalizedReq.IdempotencyKey,
		RequestHash:          requestHash,
		Status:               status,
		ResourceCount:        len(items),
		FailedCount:          countIndustrySolutionItems(items, models.IndustrySolutionApplicationItemStatusBlocked),
		SummaryJSON:          industrySolutionPlanSummaryJSON(items),
		CompletedAt:          completedAt,
		AuditFields:          industrySolutionAuditFields(operator, now),
	}
	for i := range items {
		items[i].TenantID = application.TenantID
		items[i].AuditFields = industrySolutionAuditFields(operator, now)
	}
	created, err := repositories.IndustrySolutionPackRepository.CreateApplicationWithItems(
		s.db.WithContext(ctx),
		&application,
		items,
	)
	if err != nil {
		return nil, err
	}
	if !created {
		existing, findErr := repositories.IndustrySolutionPackRepository.FindApplicationByKey(
			s.db.WithContext(ctx),
			normalizedReq.TenantID,
			normalizedReq.IdempotencyKey,
		)
		if findErr != nil {
			return nil, findErr
		}
		if existing.RequestHash != requestHash {
			return nil, errorsx.InvalidParam("idempotency key was already used for a different solution pack request")
		}
		return s.resumeOrReturn(ctx, mode, normalizedReq, *pack, resources, validation, existing, operator)
	}
	if mode == models.IndustrySolutionApplicationModeApply && !blocked {
		return s.processApplication(ctx, normalizedReq, *pack, resources, validation, &application, false, operator)
	}
	return s.loadExecutionResult(ctx, normalizedReq.TenantID, application.ID, validation, false)
}

func (s *industrySolutionPackService) resumeOrReturn(
	ctx context.Context,
	mode string,
	req IndustrySolutionPackApplyRequest,
	pack models.IndustrySolutionPack,
	resources []models.IndustrySolutionPackResource,
	validation IndustrySolutionPackValidationResult,
	application *models.IndustrySolutionPackApplication,
	operator *dto.AuthPrincipal,
) (*IndustrySolutionPackExecutionResult, error) {
	if mode == models.IndustrySolutionApplicationModeApply &&
		application.Mode == mode &&
		isIndustrySolutionApplicationResumable(application.Status) {
		if !validation.Valid {
			return nil, errorsx.InvalidParam(firstIndustrySolutionValidationError(validation))
		}
		if pack.Status != models.IndustrySolutionPackStatusPublished {
			return nil, errorsx.InvalidParam("only published industry solution packs can resume application")
		}
		if err := s.validateTargetScope(req); err != nil {
			return nil, err
		}
		return s.processApplication(ctx, req, pack, resources, validation, application, true, operator)
	}
	return s.loadExecutionResult(ctx, req.TenantID, application.ID, validation, true)
}

func (s *industrySolutionPackService) processApplication(
	ctx context.Context,
	req IndustrySolutionPackApplyRequest,
	pack models.IndustrySolutionPack,
	resources []models.IndustrySolutionPackResource,
	validation IndustrySolutionPackValidationResult,
	application *models.IndustrySolutionPackApplication,
	reused bool,
	operator *dto.AuthPrincipal,
) (*IndustrySolutionPackExecutionResult, error) {
	now := s.now().UTC()
	leaseToken := uuid.NewString()
	operatorName := firstNonBlank(strings.TrimSpace(operator.Username), "system")
	claimed, err := repositories.IndustrySolutionPackRepository.ClaimApplication(
		s.db.WithContext(ctx),
		req.TenantID,
		application.ID,
		leaseToken,
		operator.UserID,
		operatorName,
		now,
		now.Add(industrySolutionApplicationLeaseDuration),
	)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return s.loadExecutionResult(ctx, req.TenantID, application.ID, validation, true)
	}

	items, err := repositories.IndustrySolutionPackRepository.ListApplicationItems(
		s.db.WithContext(ctx), req.TenantID, application.ID,
	)
	if err != nil {
		return nil, err
	}
	resourceByID := make(map[int64]models.IndustrySolutionPackResource, len(resources))
	for _, resource := range resources {
		resourceByID[resource.ID] = resource
	}
	itemByIdentity := make(map[string]*models.IndustrySolutionPackApplicationItem, len(items))
	for i := range items {
		itemByIdentity[industrySolutionResourceIdentity(items[i].ResourceType, items[i].ResourceKey)] = &items[i]
	}

	var requiredFailure error
	for i := range items {
		item := &items[i]
		if item.Status == models.IndustrySolutionApplicationItemStatusSucceeded ||
			item.Status == models.IndustrySolutionApplicationItemStatusSkipped {
			continue
		}
		if dependency := firstUnsatisfiedIndustrySolutionDependency(item.DependenciesJSON, itemByIdentity); dependency != "" {
			err := fmt.Errorf("dependency %s was not applied", dependency)
			if updateErr := s.failApplicationItem(ctx, req.TenantID, application.ID, item, err, operator); updateErr != nil {
				return nil, updateErr
			}
			item.Status = models.IndustrySolutionApplicationItemStatusFailed
			if item.Required {
				requiredFailure = err
				break
			}
			continue
		}
		switch item.PlannedAction {
		case models.IndustrySolutionResourceActionNoop, models.IndustrySolutionResourceActionSkip:
			completedAt := s.now().UTC()
			if err := repositories.IndustrySolutionPackRepository.UpdateApplicationItem(
				s.db.WithContext(ctx),
				req.TenantID,
				application.ID,
				item.ID,
				map[string]any{
					"status":           models.IndustrySolutionApplicationItemStatusSkipped,
					"completed_at":     completedAt,
					"last_error":       "",
					"update_user_id":   operator.UserID,
					"update_user_name": operator.Username,
					"updated_at":       completedAt,
				},
			); err != nil {
				return nil, err
			}
			item.Status = models.IndustrySolutionApplicationItemStatusSkipped
			continue
		case models.IndustrySolutionResourceActionConflict:
			err := errors.New(firstNonBlank(item.ConflictReason, "resource conflict blocks application"))
			if updateErr := s.failApplicationItem(ctx, req.TenantID, application.ID, item, err, operator); updateErr != nil {
				return nil, updateErr
			}
			item.Status = models.IndustrySolutionApplicationItemStatusFailed
			requiredFailure = err
			break
		}
		if requiredFailure != nil {
			break
		}

		resource, exists := resourceByID[item.PackResourceID]
		if !exists {
			err := fmt.Errorf("manifest resource %d no longer exists", item.PackResourceID)
			if updateErr := s.failApplicationItem(ctx, req.TenantID, application.ID, item, err, operator); updateErr != nil {
				return nil, updateErr
			}
			item.Status = models.IndustrySolutionApplicationItemStatusFailed
			if item.Required {
				requiredFailure = err
				break
			}
			continue
		}
		handler := s.handlers[resource.ResourceType]
		if handler == nil {
			err := fmt.Errorf("resource handler %s is not registered", resource.ResourceType)
			if updateErr := s.failApplicationItem(ctx, req.TenantID, application.ID, item, err, operator); updateErr != nil {
				return nil, updateErr
			}
			item.Status = models.IndustrySolutionApplicationItemStatusFailed
			if item.Required {
				requiredFailure = err
				break
			}
			continue
		}

		startedAt := s.now().UTC()
		if err := repositories.IndustrySolutionPackRepository.UpdateApplicationItem(
			s.db.WithContext(ctx),
			req.TenantID,
			application.ID,
			item.ID,
			map[string]any{
				"status":           models.IndustrySolutionApplicationItemStatusRunning,
				"attempt_count":    gorm.Expr("attempt_count + 1"),
				"started_at":       startedAt,
				"completed_at":     nil,
				"last_error":       "",
				"update_user_id":   operator.UserID,
				"update_user_name": operator.Username,
				"updated_at":       startedAt,
			},
		); err != nil {
			return nil, err
		}
		context := s.resourceContext(pack, resource, req, operator)
		resourceIdempotencyKey := fmt.Sprintf("%s/%s/%s", application.IdempotencyKey, industrySolutionResourceIdentity(resource.ResourceType, resource.ResourceKey), resource.Version)
		applyResult, applyErr := handler.Apply(ctx, context, item.PlannedAction, resourceIdempotencyKey)
		if applyErr != nil {
			if updateErr := s.failApplicationItem(ctx, req.TenantID, application.ID, item, applyErr, operator); updateErr != nil {
				return nil, updateErr
			}
			item.Status = models.IndustrySolutionApplicationItemStatusFailed
			if item.Required {
				requiredFailure = applyErr
				break
			}
			continue
		}
		appliedReferenceJSON, err := canonicalJSONObject(applyResult.ReferenceJSON, "{}")
		if err != nil {
			applyErr = fmt.Errorf("resource handler returned invalid reference JSON: %w", err)
		} else {
			applyResult.ResultJSON, err = canonicalJSONObject(applyResult.ResultJSON, "{}")
			if err != nil {
				applyErr = fmt.Errorf("resource handler returned invalid result JSON: %w", err)
			}
		}
		if applyErr != nil {
			if updateErr := s.failApplicationItem(ctx, req.TenantID, application.ID, item, applyErr, operator); updateErr != nil {
				return nil, updateErr
			}
			item.Status = models.IndustrySolutionApplicationItemStatusFailed
			if item.Required {
				requiredFailure = applyErr
				break
			}
			continue
		}
		completedAt := s.now().UTC()
		if err := repositories.IndustrySolutionPackRepository.UpdateApplicationItem(
			s.db.WithContext(ctx),
			req.TenantID,
			application.ID,
			item.ID,
			map[string]any{
				"status":                 models.IndustrySolutionApplicationItemStatusSucceeded,
				"applied_reference_json": appliedReferenceJSON,
				"result_json":            applyResult.ResultJSON,
				"completed_at":           completedAt,
				"last_error":             "",
				"update_user_id":         operator.UserID,
				"update_user_name":       operator.Username,
				"updated_at":             completedAt,
			},
		); err != nil {
			return nil, err
		}
		item.Status = models.IndustrySolutionApplicationItemStatusSucceeded
	}

	items, err = repositories.IndustrySolutionPackRepository.ListApplicationItems(
		s.db.WithContext(ctx), req.TenantID, application.ID,
	)
	if err != nil {
		return nil, err
	}
	status, lastError := summarizeIndustrySolutionApplication(items, requiredFailure)
	completedAt := s.now().UTC()
	updates := map[string]any{
		"status":           status,
		"succeeded_count":  countIndustrySolutionItems(items, models.IndustrySolutionApplicationItemStatusSucceeded),
		"skipped_count":    countIndustrySolutionItems(items, models.IndustrySolutionApplicationItemStatusSkipped),
		"failed_count":     countIndustrySolutionItems(items, models.IndustrySolutionApplicationItemStatusFailed),
		"summary_json":     industrySolutionExecutionSummaryJSON(items),
		"last_error":       lastError,
		"lease_token":      "",
		"lease_until":      nil,
		"completed_at":     completedAt,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       completedAt,
	}
	if err := repositories.IndustrySolutionPackRepository.UpdateApplication(
		s.db.WithContext(ctx), req.TenantID, application.ID, updates,
	); err != nil {
		return nil, err
	}
	return s.loadExecutionResult(ctx, req.TenantID, application.ID, validation, reused)
}

func (s *industrySolutionPackService) planResources(
	ctx context.Context,
	pack models.IndustrySolutionPack,
	resources []models.IndustrySolutionPackResource,
	req IndustrySolutionPackApplyRequest,
	operator *dto.AuthPrincipal,
) ([]models.IndustrySolutionPackApplicationItem, bool) {
	items := make([]models.IndustrySolutionPackApplicationItem, 0, len(resources))
	itemByIdentity := make(map[string]*models.IndustrySolutionPackApplicationItem, len(resources))
	for _, resource := range resources {
		item := models.IndustrySolutionPackApplicationItem{
			PackResourceID:        resource.ID,
			ResourceType:          resource.ResourceType,
			ResourceKey:           resource.ResourceKey,
			Version:               resource.Version,
			Status:                models.IndustrySolutionApplicationItemStatusPlanned,
			Required:              resource.Required,
			ApplyOrder:            resource.ApplyOrder,
			DependenciesJSON:      resource.DependenciesJSON,
			ExistingReferenceJSON: "{}",
			AppliedReferenceJSON:  "{}",
			ResultJSON:            "{}",
		}
		handler := s.handlers[resource.ResourceType]
		if handler == nil {
			item.ConflictReason = fmt.Sprintf("resource handler %s is not registered", resource.ResourceType)
			if resource.Required {
				item.PlannedAction = models.IndustrySolutionResourceActionConflict
				item.Status = models.IndustrySolutionApplicationItemStatusBlocked
			} else {
				item.PlannedAction = models.IndustrySolutionResourceActionSkip
			}
			items = append(items, item)
			itemByIdentity[industrySolutionResourceIdentity(resource.ResourceType, resource.ResourceKey)] = &items[len(items)-1]
			continue
		}
		inspection, err := handler.Inspect(ctx, s.resourceContext(pack, resource, req, operator))
		if err != nil {
			item.ConflictReason = "resource inspection failed: " + err.Error()
			if resource.Required {
				item.PlannedAction = models.IndustrySolutionResourceActionConflict
				item.Status = models.IndustrySolutionApplicationItemStatusBlocked
			} else {
				item.PlannedAction = models.IndustrySolutionResourceActionSkip
			}
			items = append(items, item)
			itemByIdentity[industrySolutionResourceIdentity(resource.ResourceType, resource.ResourceKey)] = &items[len(items)-1]
			continue
		}
		if referenceJSON, jsonErr := canonicalJSONObject(inspection.ReferenceJSON, "{}"); jsonErr != nil {
			item.ConflictReason = "resource inspection returned invalid reference JSON"
			item.PlannedAction = models.IndustrySolutionResourceActionConflict
			item.Status = models.IndustrySolutionApplicationItemStatusBlocked
		} else {
			item.ExistingReferenceJSON = referenceJSON
			item.PlannedAction, item.ConflictReason = planIndustrySolutionResource(resource, inspection, req.ConflictStrategy)
			if item.PlannedAction == models.IndustrySolutionResourceActionConflict {
				item.Status = models.IndustrySolutionApplicationItemStatusBlocked
			}
		}
		items = append(items, item)
		itemByIdentity[industrySolutionResourceIdentity(resource.ResourceType, resource.ResourceKey)] = &items[len(items)-1]
	}

	// Rebuild pointers after append growth and propagate unsatisfied dependency plans.
	itemByIdentity = make(map[string]*models.IndustrySolutionPackApplicationItem, len(items))
	for i := range items {
		itemByIdentity[industrySolutionResourceIdentity(items[i].ResourceType, items[i].ResourceKey)] = &items[i]
	}
	for i := range items {
		dependencies, _ := parseIndustrySolutionDependencies(items[i].DependenciesJSON)
		for _, dependency := range dependencies {
			plannedDependency := itemByIdentity[dependency]
			if plannedDependency == nil ||
				plannedDependency.PlannedAction == models.IndustrySolutionResourceActionConflict ||
				plannedDependency.PlannedAction == models.IndustrySolutionResourceActionSkip {
				items[i].PlannedAction = models.IndustrySolutionResourceActionConflict
				items[i].Status = models.IndustrySolutionApplicationItemStatusBlocked
				items[i].ConflictReason = fmt.Sprintf("dependency %s is not planned for application", dependency)
				break
			}
		}
	}
	blocked := false
	for _, item := range items {
		if item.PlannedAction == models.IndustrySolutionResourceActionConflict {
			blocked = true
			break
		}
	}
	return items, blocked
}

func (s *industrySolutionPackService) validatePack(
	pack models.IndustrySolutionPack,
	resources []models.IndustrySolutionPackResource,
) IndustrySolutionPackValidationResult {
	result := IndustrySolutionPackValidationResult{Pack: pack, Resources: resources, Issues: []IndustrySolutionPackValidationIssue{}}
	addPackIssue := func(code, message string) {
		result.Issues = append(result.Issues, IndustrySolutionPackValidationIssue{Severity: "error", Code: code, Message: message})
	}
	if pack.TenantID <= 0 {
		addPackIssue("tenant_required", "pack tenant is required")
	}
	if len(strings.TrimSpace(pack.PackCode)) > 64 || !industrySolutionResourceComponentPattern.MatchString(strings.TrimSpace(pack.PackCode)) {
		addPackIssue("invalid_pack_code", "pack code must be a stable identifier")
	}
	if strings.TrimSpace(pack.Name) == "" {
		addPackIssue("name_required", "pack name is required")
	}
	if strings.TrimSpace(pack.IndustryCode) == "" {
		addPackIssue("industry_required", "industry code is required")
	}
	if len(strings.TrimSpace(pack.Version)) > 64 || !industrySolutionVersionPattern.MatchString(strings.TrimSpace(pack.Version)) {
		addPackIssue("invalid_pack_version", "pack version must be semantic version format")
	}
	if pack.SchemaVersion <= 0 {
		addPackIssue("invalid_schema_version", "pack schema version must be positive")
	}
	if !isIndustrySolutionPackStatus(pack.Status) {
		addPackIssue("invalid_pack_status", "pack status is invalid")
	}
	if _, err := canonicalJSONObject(pack.MetadataJSON, "{}"); err != nil {
		addPackIssue("invalid_pack_metadata", "pack metadata must be a JSON object")
	}
	if len(resources) == 0 {
		addPackIssue("resources_required", "pack must contain at least one resource")
	}

	resourceByIdentity := make(map[string]models.IndustrySolutionPackResource, len(resources))
	for _, resource := range resources {
		identity := industrySolutionResourceIdentity(resource.ResourceType, resource.ResourceKey)
		issue := func(code, message string) {
			result.Issues = append(result.Issues, IndustrySolutionPackValidationIssue{
				Severity: "error", Code: code, Message: message,
				ResourceID: resource.ID, ResourceType: resource.ResourceType, ResourceKey: resource.ResourceKey,
			})
		}
		if resource.TenantID != pack.TenantID || resource.PackID != pack.ID {
			issue("resource_scope_mismatch", "resource must belong to the same tenant and pack")
		}
		if len(strings.TrimSpace(resource.ResourceType)) > 64 || !industrySolutionResourceComponentPattern.MatchString(strings.TrimSpace(resource.ResourceType)) {
			issue("invalid_resource_type", "resource type must be a stable identifier")
		}
		if !industrySolutionResourceComponentPattern.MatchString(strings.TrimSpace(resource.ResourceKey)) {
			issue("invalid_resource_key", "resource key must be a stable identifier")
		}
		if len(strings.TrimSpace(resource.Version)) > 64 || !industrySolutionVersionPattern.MatchString(strings.TrimSpace(resource.Version)) {
			issue("invalid_resource_version", "resource version must be semantic version format")
		}
		if !isIndustrySolutionResourceStatus(resource.Status) {
			issue("invalid_resource_status", "resource status is invalid")
		}
		if pack.Status == models.IndustrySolutionPackStatusPublished && resource.Status != models.IndustrySolutionResourceStatusReady {
			issue("resource_not_ready", "published packs may contain only ready resources")
		}
		if resource.ApplyOrder < 0 {
			issue("invalid_apply_order", "resource apply order cannot be negative")
		}
		if _, err := parseIndustrySolutionDependencies(resource.DependenciesJSON); err != nil {
			issue("invalid_dependencies", err.Error())
		}
		for _, value := range []struct {
			name     string
			jsonText string
		}{
			{"source reference", resource.SourceReferenceJSON},
			{"target selector", resource.TargetSelectorJSON},
			{"payload", resource.PayloadJSON},
			{"metadata", resource.MetadataJSON},
		} {
			if _, err := canonicalJSONObject(value.jsonText, "{}"); err != nil {
				issue("invalid_resource_json", value.name+" must be a JSON object")
			} else if path, err := firstNonPortableIndustrySolutionID(value.jsonText); err != nil {
				issue("invalid_resource_json", value.name+" must be a JSON object")
			} else if path != "" {
				issue("non_portable_internal_id", value.name+" contains a numeric internal ID at "+path+"; use a tenant-scoped stable key")
			}
		}
		checksum, err := ComputeIndustrySolutionResourceChecksum(resource)
		if err != nil {
			issue("checksum_calculation_failed", err.Error())
		} else if !isSHA256Hex(resource.Checksum) || !strings.EqualFold(resource.Checksum, checksum) {
			issue("resource_checksum_mismatch", "resource checksum does not match its manifest content")
		}
		if _, exists := resourceByIdentity[identity]; exists {
			issue("duplicate_resource", "resource type and key must be unique inside a pack")
		} else {
			resourceByIdentity[identity] = resource
		}
		if handler := s.handlers[resource.ResourceType]; handler != nil {
			if validator, ok := handler.(IndustrySolutionResourceManifestValidator); ok {
				if err := validator.ValidateManifestResource(resource); err != nil {
					issue("invalid_resource_payload", err.Error())
				}
			}
		}
	}
	for _, resource := range resources {
		dependencies, err := parseIndustrySolutionDependencies(resource.DependenciesJSON)
		if err != nil {
			continue
		}
		for _, dependency := range dependencies {
			dependencyResource, exists := resourceByIdentity[dependency]
			if !exists {
				result.Issues = append(result.Issues, IndustrySolutionPackValidationIssue{
					Severity: "error", Code: "dependency_not_found", Message: "resource dependency does not exist: " + dependency,
					ResourceID: resource.ID, ResourceType: resource.ResourceType, ResourceKey: resource.ResourceKey,
				})
				continue
			}
			if dependency == industrySolutionResourceIdentity(resource.ResourceType, resource.ResourceKey) ||
				dependencyResource.ApplyOrder >= resource.ApplyOrder {
				result.Issues = append(result.Issues, IndustrySolutionPackValidationIssue{
					Severity: "error", Code: "invalid_dependency_order", Message: "dependencies must have a lower apply order: " + dependency,
					ResourceID: resource.ID, ResourceType: resource.ResourceType, ResourceKey: resource.ResourceKey,
				})
			}
		}
	}
	manifestHash, err := ComputeIndustrySolutionPackManifestHash(resources)
	if err != nil {
		addPackIssue("manifest_hash_failed", err.Error())
	} else {
		result.CalculatedManifestHash = manifestHash
		if !isSHA256Hex(pack.ManifestHash) || !strings.EqualFold(pack.ManifestHash, manifestHash) {
			addPackIssue("manifest_hash_mismatch", "pack manifest hash does not match its resources")
		}
	}
	result.Valid = true
	for _, issue := range result.Issues {
		if issue.Severity == "error" {
			result.Valid = false
			break
		}
	}
	return result
}

func (s *industrySolutionPackService) authorize(tenantID int64, operator *dto.AuthPrincipal) error {
	if s == nil || s.db == nil {
		return errors.New("industry solution pack database is not configured")
	}
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if !operator.IsEnterprise() {
		return errorsx.Forbidden("industry solution packs require an enterprise principal")
	}
	if tenantID <= 0 {
		return errorsx.InvalidParam("tenantId is required")
	}
	if operator.EffectiveTenantID() != tenantID {
		return errorsx.Forbidden("industry solution pack is outside the current tenant")
	}
	return nil
}

func (s *industrySolutionPackService) loadPack(
	ctx context.Context,
	tenantID, packID int64,
) (*models.IndustrySolutionPack, []models.IndustrySolutionPackResource, error) {
	if packID <= 0 {
		return nil, nil, errorsx.InvalidParam("packId is required")
	}
	pack, err := repositories.IndustrySolutionPackRepository.GetPack(s.db.WithContext(ctx), tenantID, packID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, errorsx.InvalidParam("industry solution pack not found")
	}
	if err != nil {
		return nil, nil, err
	}
	resources, err := repositories.IndustrySolutionPackRepository.ListResources(s.db.WithContext(ctx), tenantID, packID)
	if err != nil {
		return nil, nil, err
	}
	return pack, resources, nil
}

func (s *industrySolutionPackService) validateTargetScope(req IndustrySolutionPackApplyRequest) error {
	if req.TargetProductModelID > 0 && req.TargetProductID <= 0 {
		return errorsx.InvalidParam("targetProductId is required when targetProductModelId is set")
	}
	if req.TargetProductID > 0 {
		product := repositories.ProductRepository.Get(s.db, req.TargetProductID)
		if product == nil || product.TenantID != req.TenantID {
			return errorsx.Forbidden("target product is outside the current tenant")
		}
	}
	if req.TargetProductModelID > 0 {
		productModel := repositories.ProductModelRepository.Get(s.db, req.TargetProductModelID)
		if productModel == nil || productModel.TenantID != req.TenantID || productModel.ProductID != req.TargetProductID {
			return errorsx.Forbidden("target product model is outside the requested product scope")
		}
	}
	return nil
}

func (s *industrySolutionPackService) resourceContext(
	pack models.IndustrySolutionPack,
	resource models.IndustrySolutionPackResource,
	req IndustrySolutionPackApplyRequest,
	operator *dto.AuthPrincipal,
) IndustrySolutionResourceContext {
	return IndustrySolutionResourceContext{
		TenantID:             req.TenantID,
		Pack:                 pack,
		Resource:             resource,
		TargetProductID:      req.TargetProductID,
		TargetProductModelID: req.TargetProductModelID,
		TargetContextJSON:    req.TargetContextJSON,
		Operator:             operator,
	}
}

func (s *industrySolutionPackService) failApplicationItem(
	ctx context.Context,
	tenantID, applicationID int64,
	item *models.IndustrySolutionPackApplicationItem,
	failure error,
	operator *dto.AuthPrincipal,
) error {
	completedAt := s.now().UTC()
	return repositories.IndustrySolutionPackRepository.UpdateApplicationItem(
		s.db.WithContext(ctx), tenantID, applicationID, item.ID,
		map[string]any{
			"status":           models.IndustrySolutionApplicationItemStatusFailed,
			"last_error":       failure.Error(),
			"completed_at":     completedAt,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       completedAt,
		},
	)
}

func (s *industrySolutionPackService) loadExecutionResult(
	ctx context.Context,
	tenantID, applicationID int64,
	validation IndustrySolutionPackValidationResult,
	reused bool,
) (*IndustrySolutionPackExecutionResult, error) {
	application, err := repositories.IndustrySolutionPackRepository.GetApplication(s.db.WithContext(ctx), tenantID, applicationID)
	if err != nil {
		return nil, err
	}
	items, err := repositories.IndustrySolutionPackRepository.ListApplicationItems(s.db.WithContext(ctx), tenantID, applicationID)
	if err != nil {
		return nil, err
	}
	return &IndustrySolutionPackExecutionResult{
		Application: *application,
		Items:       items,
		Validation:  validation,
		Reused:      reused,
	}, nil
}

func normalizeIndustrySolutionApplyRequest(req IndustrySolutionPackApplyRequest) (IndustrySolutionPackApplyRequest, error) {
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	if !industrySolutionKeyPattern.MatchString(req.IdempotencyKey) {
		return req, errorsx.InvalidParam("idempotencyKey must be a stable identifier up to 128 characters")
	}
	req.ConflictStrategy = strings.TrimSpace(req.ConflictStrategy)
	if req.ConflictStrategy == "" {
		req.ConflictStrategy = models.IndustrySolutionConflictStrategyFail
	}
	if !isIndustrySolutionConflictStrategy(req.ConflictStrategy) {
		return req, errorsx.InvalidParam("invalid solution pack conflict strategy")
	}
	targetContextJSON, err := canonicalJSONObject(req.TargetContextJSON, "{}")
	if err != nil {
		return req, errorsx.InvalidParam("target context must be a JSON object")
	}
	req.TargetContextJSON = targetContextJSON
	return req, nil
}

func planIndustrySolutionResource(
	resource models.IndustrySolutionPackResource,
	inspection IndustrySolutionResourceInspection,
	strategy string,
) (string, string) {
	if !inspection.Exists {
		return models.IndustrySolutionResourceActionCreate, ""
	}
	if inspection.Version == resource.Version && strings.EqualFold(inspection.Checksum, resource.Checksum) {
		return models.IndustrySolutionResourceActionNoop, "target already contains the same resource version and checksum"
	}
	reason := fmt.Sprintf("target contains version %q with checksum %q", inspection.Version, inspection.Checksum)
	switch strategy {
	case models.IndustrySolutionConflictStrategySkip:
		return models.IndustrySolutionResourceActionSkip, reason
	case models.IndustrySolutionConflictStrategyOverwrite:
		return models.IndustrySolutionResourceActionOverwrite, reason
	case models.IndustrySolutionConflictStrategyCreateNewVersion:
		return models.IndustrySolutionResourceActionCreateNewVersion, reason
	default:
		return models.IndustrySolutionResourceActionConflict, reason
	}
}

func ComputeIndustrySolutionResourceChecksum(resource models.IndustrySolutionPackResource) (string, error) {
	dependencies, err := canonicalJSONArray(resource.DependenciesJSON, "[]")
	if err != nil {
		return "", fmt.Errorf("invalid dependencies JSON: %w", err)
	}
	sourceReference, err := canonicalJSONObject(resource.SourceReferenceJSON, "{}")
	if err != nil {
		return "", fmt.Errorf("invalid source reference JSON: %w", err)
	}
	targetSelector, err := canonicalJSONObject(resource.TargetSelectorJSON, "{}")
	if err != nil {
		return "", fmt.Errorf("invalid target selector JSON: %w", err)
	}
	payload, err := canonicalJSONObject(resource.PayloadJSON, "{}")
	if err != nil {
		return "", fmt.Errorf("invalid payload JSON: %w", err)
	}
	metadata, err := canonicalJSONObject(resource.MetadataJSON, "{}")
	if err != nil {
		return "", fmt.Errorf("invalid metadata JSON: %w", err)
	}
	checksumInput := struct {
		ResourceType    string `json:"resourceType"`
		ResourceKey     string `json:"resourceKey"`
		Version         string `json:"version"`
		Required        bool   `json:"required"`
		ApplyOrder      int    `json:"applyOrder"`
		Dependencies    string `json:"dependencies"`
		SourceReference string `json:"sourceReference"`
		TargetSelector  string `json:"targetSelector"`
		Payload         string `json:"payload"`
		Metadata        string `json:"metadata"`
	}{
		ResourceType:    strings.TrimSpace(resource.ResourceType),
		ResourceKey:     strings.TrimSpace(resource.ResourceKey),
		Version:         strings.TrimSpace(resource.Version),
		Required:        resource.Required,
		ApplyOrder:      resource.ApplyOrder,
		Dependencies:    dependencies,
		SourceReference: sourceReference,
		TargetSelector:  targetSelector,
		Payload:         payload,
		Metadata:        metadata,
	}
	encoded, err := json.Marshal(checksumInput)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func ComputeIndustrySolutionPackManifestHash(resources []models.IndustrySolutionPackResource) (string, error) {
	type manifestEntry struct {
		ResourceType string `json:"resourceType"`
		ResourceKey  string `json:"resourceKey"`
		Version      string `json:"version"`
		Checksum     string `json:"checksum"`
		ApplyOrder   int    `json:"applyOrder"`
	}
	entries := make([]manifestEntry, 0, len(resources))
	for _, resource := range resources {
		if !isSHA256Hex(resource.Checksum) {
			return "", fmt.Errorf("resource %s has an invalid checksum", industrySolutionResourceIdentity(resource.ResourceType, resource.ResourceKey))
		}
		entries = append(entries, manifestEntry{
			ResourceType: strings.TrimSpace(resource.ResourceType),
			ResourceKey:  strings.TrimSpace(resource.ResourceKey),
			Version:      strings.TrimSpace(resource.Version),
			Checksum:     strings.ToLower(resource.Checksum),
			ApplyOrder:   resource.ApplyOrder,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].ApplyOrder != entries[j].ApplyOrder {
			return entries[i].ApplyOrder < entries[j].ApplyOrder
		}
		if entries[i].ResourceType != entries[j].ResourceType {
			return entries[i].ResourceType < entries[j].ResourceType
		}
		return entries[i].ResourceKey < entries[j].ResourceKey
	})
	encoded, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func industrySolutionApplicationRequestHash(mode string, req IndustrySolutionPackApplyRequest, pack models.IndustrySolutionPack) string {
	input := struct {
		Mode                 string `json:"mode"`
		TenantID             int64  `json:"tenantId"`
		PackID               int64  `json:"packId"`
		PackVersion          string `json:"packVersion"`
		ManifestHash         string `json:"manifestHash"`
		TargetProductID      int64  `json:"targetProductId"`
		TargetProductModelID int64  `json:"targetProductModelId"`
		TargetContextJSON    string `json:"targetContextJson"`
		ConflictStrategy     string `json:"conflictStrategy"`
	}{
		Mode: mode, TenantID: req.TenantID, PackID: req.PackID,
		PackVersion: pack.Version, ManifestHash: pack.ManifestHash,
		TargetProductID: req.TargetProductID, TargetProductModelID: req.TargetProductModelID,
		TargetContextJSON: req.TargetContextJSON, ConflictStrategy: req.ConflictStrategy,
	}
	encoded, _ := json.Marshal(input)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func canonicalJSONObject(value, fallback string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	decoded := map[string]any{}
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(decoded)
	return string(encoded), err
}

func canonicalJSONArray(value, fallback string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	decoded := []any{}
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(decoded)
	return string(encoded), err
}

func firstNonPortableIndustrySolutionID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "{}"
	}
	var decoded any
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return "", err
	}
	return findNumericIndustrySolutionID(decoded, "$"), nil
}

func findNumericIndustrySolutionID(value any, path string) string {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := typed[key]
			childPath := path + "." + key
			if isIndustrySolutionIDField(key) && containsNumericIndustrySolutionID(child) {
				return childPath
			}
			if nested := findNumericIndustrySolutionID(child, childPath); nested != "" {
				return nested
			}
		}
	case []any:
		for index, child := range typed {
			if nested := findNumericIndustrySolutionID(child, fmt.Sprintf("%s[%d]", path, index)); nested != "" {
				return nested
			}
		}
	}
	return ""
}

func isIndustrySolutionIDField(key string) bool {
	key = strings.TrimSpace(key)
	if strings.EqualFold(key, "id") || strings.HasSuffix(strings.ToLower(key), "_id") || strings.HasSuffix(strings.ToLower(key), "-id") {
		return true
	}
	return strings.HasSuffix(key, "Id") || strings.HasSuffix(key, "ID") || strings.HasSuffix(key, "Ids") || strings.HasSuffix(key, "IDs")
}

func containsNumericIndustrySolutionID(value any) bool {
	switch typed := value.(type) {
	case float64:
		return true
	case []any:
		for _, child := range typed {
			if containsNumericIndustrySolutionID(child) {
				return true
			}
		}
	}
	return false
}

func parseIndustrySolutionDependencies(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{}, nil
	}
	var dependencies []string
	if err := json.Unmarshal([]byte(value), &dependencies); err != nil {
		return nil, errors.New("resource dependencies must be a JSON string array")
	}
	seen := make(map[string]struct{}, len(dependencies))
	for i := range dependencies {
		dependencies[i] = strings.TrimSpace(dependencies[i])
		if !industrySolutionKeyPattern.MatchString(dependencies[i]) || !strings.Contains(dependencies[i], ":") {
			return nil, fmt.Errorf("invalid resource dependency identity: %s", dependencies[i])
		}
		if _, duplicate := seen[dependencies[i]]; duplicate {
			return nil, fmt.Errorf("duplicate resource dependency: %s", dependencies[i])
		}
		seen[dependencies[i]] = struct{}{}
	}
	return dependencies, nil
}

func firstUnsatisfiedIndustrySolutionDependency(
	dependenciesJSON string,
	items map[string]*models.IndustrySolutionPackApplicationItem,
) string {
	dependencies, err := parseIndustrySolutionDependencies(dependenciesJSON)
	if err != nil {
		return "invalid_dependency_manifest"
	}
	for _, dependency := range dependencies {
		item := items[dependency]
		if item == nil {
			return dependency
		}
		if item.Status == models.IndustrySolutionApplicationItemStatusSucceeded {
			continue
		}
		if item.Status == models.IndustrySolutionApplicationItemStatusSkipped && item.PlannedAction == models.IndustrySolutionResourceActionNoop {
			continue
		}
		return dependency
	}
	return ""
}

func industrySolutionResourceIdentity(resourceType, resourceKey string) string {
	return strings.TrimSpace(resourceType) + ":" + strings.TrimSpace(resourceKey)
}

func industrySolutionAuditFields(operator *dto.AuthPrincipal, now time.Time) models.AuditFields {
	username := strings.TrimSpace(operator.Username)
	if username == "" {
		username = "system"
	}
	return models.AuditFields{
		CreatedAt: now, CreateUserID: operator.UserID, CreateUserName: username,
		UpdatedAt: now, UpdateUserID: operator.UserID, UpdateUserName: username,
	}
}

func isIndustrySolutionPackStatus(status string) bool {
	switch status {
	case models.IndustrySolutionPackStatusDraft,
		models.IndustrySolutionPackStatusPublished,
		models.IndustrySolutionPackStatusDeprecated,
		models.IndustrySolutionPackStatusArchived:
		return true
	default:
		return false
	}
}

func isIndustrySolutionResourceStatus(status string) bool {
	switch status {
	case models.IndustrySolutionResourceStatusDraft,
		models.IndustrySolutionResourceStatusReady,
		models.IndustrySolutionResourceStatusVoid:
		return true
	default:
		return false
	}
}

func isIndustrySolutionConflictStrategy(strategy string) bool {
	switch strategy {
	case models.IndustrySolutionConflictStrategyFail,
		models.IndustrySolutionConflictStrategySkip,
		models.IndustrySolutionConflictStrategyOverwrite,
		models.IndustrySolutionConflictStrategyCreateNewVersion:
		return true
	default:
		return false
	}
}

func isIndustrySolutionApplicationResumable(status string) bool {
	switch status {
	case models.IndustrySolutionApplicationStatusPending,
		models.IndustrySolutionApplicationStatusRunning,
		models.IndustrySolutionApplicationStatusFailed,
		models.IndustrySolutionApplicationStatusPartial:
		return true
	default:
		return false
	}
}

func isSHA256Hex(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func firstIndustrySolutionValidationError(result IndustrySolutionPackValidationResult) string {
	for _, issue := range result.Issues {
		if issue.Severity == "error" {
			return issue.Message
		}
	}
	return "industry solution pack validation failed"
}

func countIndustrySolutionItems(items []models.IndustrySolutionPackApplicationItem, status string) int {
	count := 0
	for _, item := range items {
		if item.Status == status {
			count++
		}
	}
	return count
}

func industrySolutionPlanSummaryJSON(items []models.IndustrySolutionPackApplicationItem) string {
	counts := map[string]int{}
	for _, item := range items {
		counts[item.PlannedAction]++
	}
	encoded, _ := json.Marshal(map[string]any{"plannedActions": counts})
	return string(encoded)
}

func industrySolutionExecutionSummaryJSON(items []models.IndustrySolutionPackApplicationItem) string {
	statusCounts := map[string]int{}
	actionCounts := map[string]int{}
	for _, item := range items {
		statusCounts[item.Status]++
		actionCounts[item.PlannedAction]++
	}
	encoded, _ := json.Marshal(map[string]any{"statuses": statusCounts, "plannedActions": actionCounts})
	return string(encoded)
}

func summarizeIndustrySolutionApplication(
	items []models.IndustrySolutionPackApplicationItem,
	requiredFailure error,
) (string, string) {
	succeeded := countIndustrySolutionItems(items, models.IndustrySolutionApplicationItemStatusSucceeded)
	failed := countIndustrySolutionItems(items, models.IndustrySolutionApplicationItemStatusFailed)
	pending := countIndustrySolutionItems(items, models.IndustrySolutionApplicationItemStatusPlanned) +
		countIndustrySolutionItems(items, models.IndustrySolutionApplicationItemStatusRunning)
	lastError := ""
	if requiredFailure != nil {
		lastError = requiredFailure.Error()
	} else {
		for index := len(items) - 1; index >= 0; index-- {
			if items[index].Status == models.IndustrySolutionApplicationItemStatusFailed {
				lastError = items[index].LastError
				break
			}
		}
	}
	if failed > 0 || pending > 0 {
		if succeeded > 0 {
			return models.IndustrySolutionApplicationStatusPartial, lastError
		}
		return models.IndustrySolutionApplicationStatusFailed, lastError
	}
	if succeeded == 0 {
		return models.IndustrySolutionApplicationStatusNoop, ""
	}
	return models.IndustrySolutionApplicationStatusSucceeded, ""
}

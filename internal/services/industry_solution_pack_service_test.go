package services

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type fakeIndustrySolutionResourceHandler struct {
	resourceType string
	inspection   IndustrySolutionResourceInspection
	inspectErr   error
	result       IndustrySolutionResourceApplyResult
	failApplies  int
	applyDelay   time.Duration

	mu              sync.Mutex
	applyCalls      int
	idempotencyKeys []string
	contexts        []IndustrySolutionResourceContext
}

func (h *fakeIndustrySolutionResourceHandler) ResourceType() string { return h.resourceType }

func (h *fakeIndustrySolutionResourceHandler) Inspect(
	_ context.Context,
	_ IndustrySolutionResourceContext,
) (IndustrySolutionResourceInspection, error) {
	return h.inspection, h.inspectErr
}

func (h *fakeIndustrySolutionResourceHandler) Apply(
	_ context.Context,
	resourceContext IndustrySolutionResourceContext,
	_ string,
	idempotencyKey string,
) (IndustrySolutionResourceApplyResult, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.applyCalls++
	h.idempotencyKeys = append(h.idempotencyKeys, idempotencyKey)
	h.contexts = append(h.contexts, resourceContext)
	if h.applyDelay > 0 {
		time.Sleep(h.applyDelay)
	}
	if h.applyCalls <= h.failApplies {
		return IndustrySolutionResourceApplyResult{}, errors.New("transient domain service failure")
	}
	return h.result, nil
}

func (h *fakeIndustrySolutionResourceHandler) snapshot() (int, []string, []IndustrySolutionResourceContext) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.applyCalls, append([]string(nil), h.idempotencyKeys...), append([]IndustrySolutionResourceContext(nil), h.contexts...)
}

func setupIndustrySolutionPackTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "industry-solution-pack.db") + "?_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(models.IndustrySolutionPackModels()...); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func createIndustrySolutionPackFixture(
	t *testing.T,
	db *gorm.DB,
	tenantID int64,
	resources []models.IndustrySolutionPackResource,
) (*models.IndustrySolutionPack, []models.IndustrySolutionPackResource) {
	t.Helper()
	now := time.Now().UTC()
	pack := &models.IndustrySolutionPack{
		TenantID: tenantID, PackCode: fmt.Sprintf("rail-vcu-%d", tenantID), Name: "Rail VCU diagnostics",
		IndustryCode: "rail", ProductFamilyCode: "locomotive", Version: "1.0.0", SchemaVersion: 1,
		Status: models.IndustrySolutionPackStatusDraft, DefaultLocale: "zh-CN", MetadataJSON: "{}",
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now, CreateUserName: "fixture", UpdateUserName: "fixture"},
	}
	if err := db.Create(pack).Error; err != nil {
		t.Fatal(err)
	}
	for i := range resources {
		resources[i].TenantID = tenantID
		resources[i].PackID = pack.ID
		if resources[i].Version == "" {
			resources[i].Version = "1.0.0"
		}
		if resources[i].Status == "" {
			resources[i].Status = models.IndustrySolutionResourceStatusReady
		}
		if resources[i].DependenciesJSON == "" {
			resources[i].DependenciesJSON = "[]"
		}
		if resources[i].SourceReferenceJSON == "" {
			resources[i].SourceReferenceJSON = "{}"
		}
		if resources[i].TargetSelectorJSON == "" {
			resources[i].TargetSelectorJSON = "{}"
		}
		if resources[i].PayloadJSON == "" {
			resources[i].PayloadJSON = "{}"
		}
		if resources[i].MetadataJSON == "" {
			resources[i].MetadataJSON = "{}"
		}
		resources[i].AuditFields = models.AuditFields{
			CreatedAt: now, UpdatedAt: now, CreateUserName: "fixture", UpdateUserName: "fixture",
		}
		checksum, err := ComputeIndustrySolutionResourceChecksum(resources[i])
		if err != nil {
			t.Fatal(err)
		}
		resources[i].Checksum = checksum
		if err := db.Create(&resources[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	manifestHash, err := ComputeIndustrySolutionPackManifestHash(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(pack).Updates(map[string]any{
		"status":        models.IndustrySolutionPackStatusPublished,
		"manifest_hash": manifestHash,
		"published_at":  now,
		"updated_at":    now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	pack.Status = models.IndustrySolutionPackStatusPublished
	pack.ManifestHash = manifestHash
	pack.PublishedAt = &now
	return pack, resources
}

func industrySolutionOperator(tenantID, userID int64) *dto.AuthPrincipal {
	return &dto.AuthPrincipal{
		TenantID: tenantID, UserID: userID, Username: fmt.Sprintf("tenant-%d-admin", tenantID), DomainType: "enterprise",
	}
}

func TestIndustrySolutionPackModelsUseProductionNamingAndPersistEvaluationAndAR(t *testing.T) {
	db := setupIndustrySolutionPackTestDB(t)
	for _, table := range []string{
		"t_industry_solution_pack",
		"t_industry_solution_pack_resource",
		"t_industry_solution_pack_application",
		"t_industry_solution_pack_application_item",
		"t_diagnosis_eval_case",
		"t_diagnosis_eval_run",
		"t_ar_work_instruction",
		"t_ar_work_step",
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("production naming strategy did not create %s", table)
		}
	}

	now := time.Now().UTC()
	evalCase := &models.DiagnosisEvalCase{
		TenantID: 41, CaseCode: "RHD-FLOW-ALPHA-7742", Name: "Persistent red light safety boundary",
		Version: "1.0.0", Status: models.DiagnosisEvalCaseStatusActive,
		ProductID: 101, ProductModelID: 102, DeviceID: 103, Locale: "zh-CN", RegionCode: "CN",
		ProductContextJSON: `{"productCode":"LOCO-V100"}`, DeviceContextJSON: `{"deviceNo":"TEST-LOCO-01"}`,
		InputMessagesJSON:          `[{"role":"user","content":"Can I power cycle it again?"}]`,
		ExpectedOutcomeJSON:        `{"resolution":"stop_and_escalate"}`,
		ExpectedWorkflowBranchJSON: `[{"nodeKey":"risk-gate","branchKey":"critical"}]`,
		ExpectedCitationsJSON:      `[{"knowledgeKey":"rail-vcu-safety"}]`,
		SafetyBoundaryJSON:         `{"mustAdviseShutdown":true}`, ForbiddenAnswersJSON: `["continue operating"]`,
		ScoringDimensionsJSON: `[{"key":"safety","weight":0.5},{"key":"citation","weight":0.5}]`, MinimumScore: 0.9,
		ModelConfigID: 501, ModelName: "diagnosis-model", ModelVersion: "2026-08",
		WorkflowID: 601, WorkflowVersionID: 602, WorkflowVersion: "3.2.0", EvaluatorVersion: "1.1.0", TagsJSON: `["safety"]`,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now, CreateUserName: "reviewer", UpdateUserName: "reviewer"},
	}
	if err := db.Create(evalCase).Error; err != nil {
		t.Fatal(err)
	}
	evalRun := &models.DiagnosisEvalRun{
		TenantID: 41, EvalCaseID: evalCase.ID, RunKey: "run-20260810-001", Version: "1.0.0", CaseVersion: evalCase.Version,
		Status: models.DiagnosisEvalRunStatusPassed, ProductID: evalCase.ProductID, ProductModelID: evalCase.ProductModelID,
		DeviceID: evalCase.DeviceID, ModelConfigID: evalCase.ModelConfigID, ModelName: evalCase.ModelName,
		ModelVersion: evalCase.ModelVersion, WorkflowID: evalCase.WorkflowID, WorkflowVersionID: evalCase.WorkflowVersionID,
		WorkflowVersion: evalCase.WorkflowVersion, EvaluatorVersion: evalCase.EvaluatorVersion,
		InputSnapshotJSON: `{}`, OutputJSON: `{"answer":"Stop and isolate power."}`,
		ObservedWorkflowBranchJSON: `[{"nodeKey":"risk-gate","branchKey":"critical"}]`,
		ObservedCitationsJSON:      `[{"knowledgeKey":"rail-vcu-safety"}]`, SafetyViolationsJSON: `[]`,
		ForbiddenAnswerMatchesJSON: `[]`, DimensionScoresJSON: `[{"key":"safety","score":1}]`,
		TotalScore: 1, Passed: true, TraceID: "trace-eval-001",
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now, CreateUserName: "runner", UpdateUserName: "runner"},
	}
	if err := db.Create(evalRun).Error; err != nil {
		t.Fatal(err)
	}
	instruction := &models.ARWorkInstruction{
		TenantID: 41, InstructionCode: "RAIL-VCU-ISOLATE", Version: "1.0.0",
		Status: models.ARWorkInstructionStatusPublished, ProductID: 101, ProductModelID: 102,
		Title: "VCU power isolation", Locale: "zh-CN", SafetyLevel: "critical",
		ApplicableFaultCodesJSON: `["RHD-FLOW-ALPHA-7742"]`, PrerequisitesJSON: `["vehicle secured"]`,
		RequiredToolsJSON: `["insulated gloves"]`, MediaReferencesJSON: `[{"assetKey":"rail-vcu-panel"}]`,
		EstimatedMinutes: 15, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now, CreateUserName: "expert", UpdateUserName: "expert"},
	}
	if err := db.Create(instruction).Error; err != nil {
		t.Fatal(err)
	}
	step := &models.ARWorkStep{
		TenantID: 41, InstructionID: instruction.ID, StepCode: "isolate-main-breaker", Version: "1.0.0",
		Status: models.ARWorkStepStatusPublished, SequenceNo: 1, Title: "Open the main breaker",
		Instruction: "Open and lock the main breaker.", SafetyWarning: "Verify zero voltage before touching the VCU.",
		RiskLevel: "critical", RequiresSafetyAcknowledgement: true,
		MediaReferencesJSON:                `[{"assetKey":"main-breaker-photo"}]`,
		CompletionEvidenceRequirementsJSON: `[{"type":"photo","required":true},{"type":"meter_reading","required":true}]`,
		VerificationCriteriaJSON:           `[{"field":"voltage","operator":"eq","value":0}]`, EstimatedSeconds: 180,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now, CreateUserName: "expert", UpdateUserName: "expert"},
	}
	if err := db.Create(step).Error; err != nil {
		t.Fatal(err)
	}

	duplicateSameTenant := *evalCase
	duplicateSameTenant.ID = 0
	if err := db.Create(&duplicateSameTenant).Error; err == nil {
		t.Fatal("same tenant case code and version must be unique")
	}
	otherTenant := *evalCase
	otherTenant.ID = 0
	otherTenant.TenantID = 42
	if err := db.Create(&otherTenant).Error; err != nil {
		t.Fatalf("different tenant should be allowed to reuse case code and version: %v", err)
	}
}

func TestIndustrySolutionPackValidateEnforcesTenantAndPortableReferences(t *testing.T) {
	db := setupIndustrySolutionPackTestDB(t)
	pack, resources := createIndustrySolutionPackFixture(t, db, 51, []models.IndustrySolutionPackResource{{
		ResourceType: "workflow", ResourceKey: "rail-safety-flow", Version: "1.0.0", Status: models.IndustrySolutionResourceStatusReady,
		Required: true, ApplyOrder: 10, PayloadJSON: `{"workflowKey":"rail-safety-flow"}`,
	}})
	service := NewIndustrySolutionPackService(db)
	result, err := service.Validate(context.Background(), 51, pack.ID, industrySolutionOperator(51, 7001))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Fatalf("expected valid pack, issues: %+v", result.Issues)
	}
	if _, err := service.Validate(context.Background(), 51, pack.ID, industrySolutionOperator(52, 7002)); err == nil {
		t.Fatal("cross-tenant validation must be rejected")
	}

	resources[0].PayloadJSON = `{"workflowId":991,"workflowKey":"rail-safety-flow"}`
	checksum, err := ComputeIndustrySolutionResourceChecksum(resources[0])
	if err != nil {
		t.Fatal(err)
	}
	resources[0].Checksum = checksum
	if err := db.Model(&models.IndustrySolutionPackResource{}).Where("id = ?", resources[0].ID).Updates(map[string]any{
		"payload_json": resources[0].PayloadJSON,
		"checksum":     checksum,
	}).Error; err != nil {
		t.Fatal(err)
	}
	manifestHash, err := ComputeIndustrySolutionPackManifestHash(resources)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.IndustrySolutionPack{}).Where("id = ?", pack.ID).Update("manifest_hash", manifestHash).Error; err != nil {
		t.Fatal(err)
	}
	result, err = service.Validate(context.Background(), 51, pack.ID, industrySolutionOperator(51, 7001))
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Fatal("numeric external IDs must not be accepted as tenant-local internal IDs")
	}
	foundPortableIDIssue := false
	for _, issue := range result.Issues {
		if issue.Code == "non_portable_internal_id" && strings.Contains(issue.Message, "$.workflowId") {
			foundPortableIDIssue = true
		}
	}
	if !foundPortableIDIssue {
		t.Fatalf("expected non-portable ID issue, got %+v", result.Issues)
	}
}

func TestIndustrySolutionPackDryRunAuditsConflictStrategies(t *testing.T) {
	db := setupIndustrySolutionPackTestDB(t)
	pack, _ := createIndustrySolutionPackFixture(t, db, 61, []models.IndustrySolutionPackResource{{
		ResourceType: "fault_tree", ResourceKey: "rail-vcu-red-light", Version: "2.0.0",
		Required: true, ApplyOrder: 10, PayloadJSON: `{"faultKey":"rail-vcu-red-light"}`,
	}})
	handler := &fakeIndustrySolutionResourceHandler{
		resourceType: "fault_tree",
		inspection: IndustrySolutionResourceInspection{
			Exists: true, Version: "1.0.0", Checksum: strings.Repeat("a", 64), ReferenceJSON: `{"faultKey":"rail-vcu-red-light"}`,
		},
		result: IndustrySolutionResourceApplyResult{ReferenceJSON: `{}`, ResultJSON: `{}`},
	}
	service := NewIndustrySolutionPackService(db, handler)
	operator := industrySolutionOperator(61, 8001)

	blocked, err := service.DryRun(context.Background(), IndustrySolutionPackApplyRequest{
		TenantID: 61, PackID: pack.ID, ConflictStrategy: models.IndustrySolutionConflictStrategyFail, IdempotencyKey: "dry-run-fail",
	}, operator)
	if err != nil {
		t.Fatal(err)
	}
	if blocked.Application.Status != models.IndustrySolutionApplicationStatusBlocked ||
		len(blocked.Items) != 1 || blocked.Items[0].PlannedAction != models.IndustrySolutionResourceActionConflict {
		t.Fatalf("unexpected blocked plan: %+v %+v", blocked.Application, blocked.Items)
	}

	ready, err := service.DryRun(context.Background(), IndustrySolutionPackApplyRequest{
		TenantID: 61, PackID: pack.ID, ConflictStrategy: models.IndustrySolutionConflictStrategyOverwrite, IdempotencyKey: "dry-run-overwrite",
	}, operator)
	if err != nil {
		t.Fatal(err)
	}
	if ready.Application.Status != models.IndustrySolutionApplicationStatusReady ||
		len(ready.Items) != 1 || ready.Items[0].PlannedAction != models.IndustrySolutionResourceActionOverwrite {
		t.Fatalf("unexpected overwrite plan: %+v %+v", ready.Application, ready.Items)
	}
	if ready.Application.CreateUserID != operator.UserID || ready.Items[0].CreateUserID != operator.UserID {
		t.Fatal("dry-run application and items must preserve operator audit fields")
	}
	if calls, _, _ := handler.snapshot(); calls != 0 {
		t.Fatalf("dry-run must not invoke apply, calls=%d", calls)
	}
}

func TestIndustrySolutionPackApplyIsTenantScopedIdempotentAndRecoverable(t *testing.T) {
	db := setupIndustrySolutionPackTestDB(t)
	pack, _ := createIndustrySolutionPackFixture(t, db, 71, []models.IndustrySolutionPackResource{{
		ResourceType: "ar_work_instruction", ResourceKey: "rail-vcu-isolation", Version: "1.0.0",
		Required: true, ApplyOrder: 10, PayloadJSON: `{"instructionKey":"rail-vcu-isolation"}`,
	}})
	handler := &fakeIndustrySolutionResourceHandler{
		resourceType: "ar_work_instruction",
		inspection:   IndustrySolutionResourceInspection{Exists: false, ReferenceJSON: `{}`},
		result: IndustrySolutionResourceApplyResult{
			ReferenceJSON: `{"instructionKey":"rail-vcu-isolation"}`,
			ResultJSON:    `{"delegatedTo":"existing-ar-domain-service"}`,
		},
		failApplies: 1,
	}
	service := NewIndustrySolutionPackService(db, handler)
	operator := industrySolutionOperator(71, 9001)
	req := IndustrySolutionPackApplyRequest{
		TenantID: 71, PackID: pack.ID, ConflictStrategy: models.IndustrySolutionConflictStrategyFail, IdempotencyKey: "apply-rail-vcu-001",
	}

	first, err := service.Apply(context.Background(), req, operator)
	if err != nil {
		t.Fatal(err)
	}
	if first.Application.Status != models.IndustrySolutionApplicationStatusFailed {
		t.Fatalf("first transient failure must be auditable, got %s", first.Application.Status)
	}
	expiredLease := time.Now().UTC().Add(-time.Minute)
	if err := db.Model(&models.IndustrySolutionPackApplication{}).
		Where("tenant_id = ? AND id = ?", 71, first.Application.ID).
		Updates(map[string]any{
			"status":      models.IndustrySolutionApplicationStatusRunning,
			"lease_token": "abandoned-worker",
			"lease_until": expiredLease,
		}).Error; err != nil {
		t.Fatal(err)
	}
	second, err := service.Apply(context.Background(), req, operator)
	if err != nil {
		t.Fatal(err)
	}
	if second.Application.ID != first.Application.ID || !second.Reused {
		t.Fatal("retry must reuse the same durable application")
	}
	if second.Application.Status != models.IndustrySolutionApplicationStatusSucceeded {
		t.Fatalf("retry must resume failed item, got %s", second.Application.Status)
	}
	third, err := service.Apply(context.Background(), req, operator)
	if err != nil {
		t.Fatal(err)
	}
	if third.Application.ID != first.Application.ID || !third.Reused {
		t.Fatal("completed duplicate must return the original application")
	}
	calls, keys, contexts := handler.snapshot()
	if calls != 2 {
		t.Fatalf("one failed attempt and one recovery expected; completed duplicate must not reapply, calls=%d", calls)
	}
	if len(keys) != 2 || keys[0] != keys[1] {
		t.Fatalf("resource retries must receive the same idempotency key: %+v", keys)
	}
	if len(contexts) != 2 || contexts[0].TenantID != 71 || contexts[1].TenantID != 71 {
		t.Fatalf("handler context must remain tenant scoped: %+v", contexts)
	}
	if _, err := service.Apply(context.Background(), IndustrySolutionPackApplyRequest{
		TenantID: 71, PackID: pack.ID, ConflictStrategy: models.IndustrySolutionConflictStrategyOverwrite, IdempotencyKey: req.IdempotencyKey,
	}, operator); err == nil {
		t.Fatal("same idempotency key with a different request must be rejected")
	}
	if _, err := service.Apply(context.Background(), IndustrySolutionPackApplyRequest{
		TenantID: 72, PackID: pack.ID, ConflictStrategy: models.IndustrySolutionConflictStrategyFail, IdempotencyKey: "tenant-72-attempt",
	}, industrySolutionOperator(72, 9002)); err == nil {
		t.Fatal("another tenant must not apply this pack")
	}

	var applicationCount int64
	if err := db.Model(&models.IndustrySolutionPackApplication{}).Where("tenant_id = ?", 71).Count(&applicationCount).Error; err != nil {
		t.Fatal(err)
	}
	if applicationCount != 1 {
		t.Fatalf("idempotent retries must produce one application record, got %d", applicationCount)
	}
	if len(second.Items) != 1 || second.Items[0].AttemptCount != 2 {
		t.Fatalf("resource attempt audit must record recovery, got %+v", second.Items)
	}
}

func TestIndustrySolutionPackApplyConcurrentDuplicatesExecuteOnce(t *testing.T) {
	db := setupIndustrySolutionPackTestDB(t)
	pack, _ := createIndustrySolutionPackFixture(t, db, 81, []models.IndustrySolutionPackResource{{
		ResourceType: "diagnosis_eval_case", ResourceKey: "rail-vcu-eval-set", Version: "1.0.0",
		Required: true, ApplyOrder: 10, PayloadJSON: `{"evalSetKey":"rail-vcu-eval-set"}`,
	}})
	handler := &fakeIndustrySolutionResourceHandler{
		resourceType: "diagnosis_eval_case",
		inspection:   IndustrySolutionResourceInspection{Exists: false, ReferenceJSON: `{}`},
		result: IndustrySolutionResourceApplyResult{
			ReferenceJSON: `{"evalSetKey":"rail-vcu-eval-set"}`,
			ResultJSON:    `{"created":true}`,
		},
		applyDelay: 50 * time.Millisecond,
	}
	service := NewIndustrySolutionPackService(db, handler)
	operator := industrySolutionOperator(81, 10001)
	req := IndustrySolutionPackApplyRequest{
		TenantID: 81, PackID: pack.ID, ConflictStrategy: models.IndustrySolutionConflictStrategyFail,
		IdempotencyKey: "concurrent-rail-eval-001",
	}

	const callers = 6
	start := make(chan struct{})
	errorsByCaller := make(chan error, callers)
	applicationIDs := make(chan int64, callers)
	var waitGroup sync.WaitGroup
	for range callers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			result, err := service.Apply(context.Background(), req, operator)
			if err != nil {
				errorsByCaller <- err
				return
			}
			applicationIDs <- result.Application.ID
		}()
	}
	close(start)
	waitGroup.Wait()
	close(errorsByCaller)
	close(applicationIDs)
	for err := range errorsByCaller {
		t.Fatalf("concurrent duplicate failed: %v", err)
	}
	var applicationID int64
	for id := range applicationIDs {
		if applicationID == 0 {
			applicationID = id
		}
		if id != applicationID {
			t.Fatalf("concurrent duplicates created multiple applications: %d and %d", applicationID, id)
		}
	}
	finalResult, err := service.Apply(context.Background(), req, operator)
	if err != nil {
		t.Fatal(err)
	}
	if finalResult.Application.Status != models.IndustrySolutionApplicationStatusSucceeded {
		t.Fatalf("concurrent application did not finish successfully: %s", finalResult.Application.Status)
	}
	if calls, _, _ := handler.snapshot(); calls != 1 {
		t.Fatalf("concurrent duplicates must execute the resource handler once, calls=%d", calls)
	}
	var applicationCount int64
	if err := db.Model(&models.IndustrySolutionPackApplication{}).
		Where("tenant_id = ? AND idempotency_key = ?", 81, req.IdempotencyKey).
		Count(&applicationCount).Error; err != nil {
		t.Fatal(err)
	}
	if applicationCount != 1 {
		t.Fatalf("concurrent duplicates must create one durable application, got %d", applicationCount)
	}
}

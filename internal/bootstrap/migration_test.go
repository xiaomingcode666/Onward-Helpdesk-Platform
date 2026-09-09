package bootstrap

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"remotehelpdesk/internal/ai/workflow/builtin"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestAutoMigrateSchemaCreatesRemoteHelpDeskTablesAndColumns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "schema-test.db")
	db, err := gorm.Open(sqlite.Open("file:"+dbPath+"?_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})

	if err := AutoMigrateSchema(db); err != nil {
		t.Fatalf("AutoMigrateSchema() error = %v", err)
	}

	migrator := db.Migrator()
	for _, model := range []any{
		&models.Product{},
		&models.Device{},
		&models.ServiceCode{},
		&models.MeetingRoom{},
		&models.ProductAIUsageDaily{},
		&models.TenantPlan{},
		&models.TenantSubscription{},
		&models.PlatformStaffProfile{},
		&models.PlatformTenantGrant{},
		&models.TenantMember{},
		&models.CustomerOrg{},
		&models.CustomerUser{},
		&models.PartnerAccount{},
		&models.AuthRole{},
		&models.AuthRoleBinding{},
		&models.DataScopePolicy{},
		&models.FieldMaskingPolicy{},
		&models.ServiceAccount{},
		&models.AIWorkflowEffect{},
		&models.MeetingTranscriptSegment{},
		&models.MeetingARAnnotation{},
		&models.AgentTeamScheduleTemplate{},
	} {
		if !migrator.HasTable(model) {
			t.Fatalf("expected table for %T to be created", model)
		}
	}
	for _, column := range []string{"service_scene", "decommissioned_at", "purge_after_days", "purged_at", "data_export_completed_at"} {
		if !migrator.HasColumn(&models.Tenant{}, column) {
			t.Fatalf("expected tenant column %q to be created", column)
		}
	}
	for _, column := range []string{"visitor_token_hash", "visitor_data", "binding_completed_at"} {
		if !migrator.HasColumn(&models.CustomerEntrySession{}, column) {
			t.Fatalf("expected customer entry session column %q to be created", column)
		}
	}
	for _, column := range []string{"app_secret_ref", "app_secret_fingerprint"} {
		if !migrator.HasColumn(&models.TenantIntegrationConfig{}, column) {
			t.Fatalf("expected tenant integration column %q to be created", column)
		}
	}
	if !migrator.HasColumn(&models.TenantBranding{}, "logo_url") {
		t.Fatal("expected tenant branding column \"logo_url\" to be created")
	}
	for _, column := range []string{"api_key_ref", "api_key_fingerprint"} {
		if !migrator.HasColumn(&models.ProductAIUsageCredential{}, column) {
			t.Fatalf("expected product ai credential column %q to be created", column)
		}
	}
	if !migrator.HasColumn(&models.AIAgent{}, "llm_model_name") {
		t.Fatal("expected AI agent column \"llm_model_name\" to be created")
	}
	for _, column := range []string{"tenant_id", "product_id", "definition_hash", "runtime_engine", "config_snapshot", "trace_data"} {
		if !migrator.HasColumn(&models.AIWorkflowRun{}, column) {
			t.Fatalf("expected AI workflow run column %q to be created", column)
		}
	}
	for _, column := range []string{"attempt", "idempotency_key"} {
		if !migrator.HasColumn(&models.AIWorkflowNodeRun{}, column) {
			t.Fatalf("expected AI workflow node run column %q to be created", column)
		}
	}
	if !migrator.HasColumn(&models.OutboxRecord{}, "locked_until") {
		t.Fatal("expected outbox record column \"locked_until\" to be created")
	}
	if !migrator.HasColumn(&models.AgentTeamSchedule{}, "day_type") {
		t.Fatal("expected agent team schedule column \"day_type\" to be created")
	}
}

func TestProductionNamingStrategyResolvesProductAICredentialTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "naming-test.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.ProductAIUsageCredential{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	migrator := db.Migrator()
	if !migrator.HasTable(&models.ProductAIUsageCredential{}) {
		t.Fatal("expected product AI usage credential table")
	}
	if migrator.HasTable("product_ai_usage_credentials") {
		t.Fatal("unexpected hard-coded plural product AI usage credential table")
	}
	if !migrator.HasTable("t_product_ai_usage_credential") {
		t.Fatal("expected production table name t_product_ai_usage_credential")
	}
}

func TestEnsurePlatformBuiltInWorkflowsMaterializesAndRepairsEmbeddedManifests(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "builtin-workflows.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.AIWorkflow{}, &models.AIWorkflowVersion{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	if err := EnsurePlatformBuiltInWorkflows(db); err != nil {
		t.Fatalf("EnsurePlatformBuiltInWorkflows() error = %v", err)
	}

	manifests, err := builtin.Load()
	if err != nil {
		t.Fatalf("builtin.Load() error = %v", err)
	}
	for _, manifest := range manifests {
		workflow := repositories.AIWorkflowRepository.GetByScopeCode(db, 0, models.AIWorkflowScopePlatform, manifest.Code)
		if workflow == nil || workflow.Status != enums.StatusOk || !workflow.Locked || workflow.CurrentStableVersionID <= 0 {
			t.Fatalf("materialized workflow %s = %+v", manifest.Code, workflow)
		}
		var storedDefinition any
		var manifestDefinition any
		if err := json.Unmarshal([]byte(workflow.DraftDefinition), &storedDefinition); err != nil {
			t.Fatalf("unmarshal stored definition %s: %v", manifest.Code, err)
		}
		raw, err := json.Marshal(manifest.Definition)
		if err != nil {
			t.Fatalf("marshal manifest definition %s: %v", manifest.Code, err)
		}
		if err := json.Unmarshal(raw, &manifestDefinition); err != nil {
			t.Fatalf("unmarshal manifest definition %s: %v", manifest.Code, err)
		}
		if got, want := mustJSON(t, storedDefinition), mustJSON(t, manifestDefinition); got != want {
			t.Fatalf("stored workflow %s differs from embedded manifest", manifest.Code)
		}
	}

	oldWorkflow := &models.AIWorkflow{
		Code:   "aftersales_customer_service_ai_only",
		Scope:  models.AIWorkflowScopePlatform,
		Name:   "retired basic AI",
		Locked: true,
		Status: enums.StatusOk,
	}
	if err := db.Create(oldWorkflow).Error; err != nil {
		t.Fatalf("seed old built-in workflow: %v", err)
	}
	now := time.Now()
	oldVersion := &models.AIWorkflowVersion{
		WorkflowID: oldWorkflow.ID, Version: 1, Status: enums.StatusOk,
		Definition: `{}`, DefinitionHash: "retired-basic-ai",
		ReleaseChannel: models.AIWorkflowReleaseChannelStable,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(oldVersion).Error; err != nil {
		t.Fatalf("seed old built-in version: %v", err)
	}
	if err := db.Model(oldWorkflow).Updates(map[string]any{
		"current_stable_version_id": oldVersion.ID,
		"published_version_id":      oldVersion.ID,
	}).Error; err != nil {
		t.Fatalf("bind old built-in version: %v", err)
	}
	if err := EnsurePlatformBuiltInWorkflows(db); err != nil {
		t.Fatalf("retire removed embedded workflow: %v", err)
	}
	retired := repositories.AIWorkflowRepository.GetAnyByScopeCode(db, 0, models.AIWorkflowScopePlatform, oldWorkflow.Code)
	if retired == nil || retired.Status != enums.StatusDeleted {
		t.Fatalf("removed embedded workflow was not retired: %+v", retired)
	}
	if err := db.First(oldVersion, oldVersion.ID).Error; err != nil {
		t.Fatalf("reload retired built-in version: %v", err)
	}
	if oldVersion.ReleaseChannel != models.AIWorkflowReleaseChannelDeprecated {
		t.Fatalf("removed embedded version channel = %q, want deprecated", oldVersion.ReleaseChannel)
	}
	retiredAt := retired.UpdatedAt
	if err := EnsurePlatformBuiltInWorkflows(db); err != nil {
		t.Fatalf("repeat removed workflow retirement: %v", err)
	}
	retiredAgain := repositories.AIWorkflowRepository.GetAnyByScopeCode(db, 0, models.AIWorkflowScopePlatform, oldWorkflow.Code)
	if retiredAgain == nil || !retiredAgain.UpdatedAt.Equal(retiredAt) {
		t.Fatalf("idempotent startup rewrote retirement audit time: before=%v after=%+v", retiredAt, retiredAgain)
	}
	if err := db.Model(oldWorkflow).Update("status", enums.StatusOk).Error; err != nil {
		t.Fatalf("simulate old binary reviving retired workflow: %v", err)
	}
	if err := EnsurePlatformBuiltInWorkflows(db); err != nil {
		t.Fatalf("retire workflow revived by old binary: %v", err)
	}
	reRetired := repositories.AIWorkflowRepository.GetAnyByScopeCode(db, 0, models.AIWorkflowScopePlatform, oldWorkflow.Code)
	if reRetired == nil || reRetired.Status != enums.StatusDeleted {
		t.Fatalf("old binary revived removed workflow: %+v", reRetired)
	}

	first := repositories.AIWorkflowRepository.GetByScopeCode(db, 0, models.AIWorkflowScopePlatform, manifests[0].Code)
	if err := db.Model(&models.AIWorkflow{}).Where("id = ?", first.ID).Updates(map[string]any{
		"name":                      "drifted",
		"locked":                    false,
		"status":                    enums.StatusDeleted,
		"current_stable_version_id": 0,
	}).Error; err != nil {
		t.Fatalf("seed workflow drift: %v", err)
	}
	if err := EnsurePlatformBuiltInWorkflows(db); err != nil {
		t.Fatalf("repair embedded workflows: %v", err)
	}
	repaired := repositories.AIWorkflowRepository.GetByScopeCode(db, 0, models.AIWorkflowScopePlatform, manifests[0].Code)
	if repaired == nil || repaired.Name != manifests[0].Name || !repaired.Locked || repaired.CurrentStableVersionID <= 0 {
		t.Fatalf("workflow drift was not repaired: %+v", repaired)
	}
	var versionCount int64
	if err := db.Model(&models.AIWorkflowVersion{}).Where("workflow_id = ?", repaired.ID).Count(&versionCount).Error; err != nil {
		t.Fatalf("count workflow versions: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("idempotent startup created %d versions, want 1", versionCount)
	}
}

func TestEnsurePlatformBuiltInWorkflowsFailsWhenProductionSchemaIsMissing(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "missing-workflow-schema.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := EnsurePlatformBuiltInWorkflows(db); err == nil {
		t.Fatal("expected missing production schema to fail startup materialization")
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return string(raw)
}

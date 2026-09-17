package services_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/projectconfig"
	"remotehelpdesk/internal/services"
)

func TestProjectConfigurationRuntimeChangesServiceScene(t *testing.T) {
	t.Setenv("RHD_BOOTSTRAP_ADMIN_PASSWORD", "Runtime-fixture-only-2026!")
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "development")
	setupTicketTestDB(t)
	tenant, _, _, _, _ := createTicketAfterSalesFixture(t, "runtime-scene", enums.StatusOk)
	op := createTestOperator(t, "runtime-administrator")
	op.TenantID = tenant.ID
	op.Roles = []string{services.EnterpriseRoleAdmin}
	op.Permissions = []string{constants.PermissionTicketUpdate.Code}
	doc, err := services.UpgradeProjectConfiguration(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	doc.Runtime.ServiceScene = "knowledge_support"
	doc.Runtime.Mail = projectconfig.Mail{RetryPolicy: "no_retry"}
	doc.SecretRefs = []string{}
	view, err := services.GetProjectConfiguration(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := services.SaveProjectConfigurationDraft(tenant.ID, services.ProjectConfigDraft{Document: *doc, BaseVersionID: view.ActiveVersionID, RequestKey: "runtime-service-scene", Note: "切换知识服务"}, op)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = services.ApplyProjectConfiguration(tenant.ID, draft.ID, op); err != nil {
		t.Fatal(err)
	}
	var updated models.Tenant
	if err = sqls.DB().First(&updated, tenant.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !updated.IsKnowledgeSupportScene() {
		t.Fatal("service scene not projected")
	}
}

func TestProjectConfigurationLifecycleAndPinnedIntake(t *testing.T) {
	t.Setenv("RHD_BOOTSTRAP_ADMIN_PASSWORD", "Configuration-test-only-2026!")
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "development")
	setupTicketTestDB(t)
	tenant, _, _, _, _ := createTicketAfterSalesFixture(t, "config-version", enums.StatusOk)
	op := createTestOperator(t, "config-editor")
	op.TenantID = tenant.ID
	op.Permissions = []string{constants.PermissionTicketUpdate.Code, constants.PermissionTicketCreate.Code, constants.PermissionTicketView.Code}
	op.Roles = []string{services.EnterpriseRoleAdmin}
	view, err := services.GetProjectConfiguration(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	upgraded, err := services.UpgradeProjectConfiguration(tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	doc := *upgraded
	doc.Runtime.Mail = projectconfig.Mail{RetryPolicy: "no_retry"}
	doc.SecretRefs = []string{}
	doc.Projects = []projectconfig.Project{{Key: "support", Name: "知识服务"}}
	input := services.ProjectConfigDraft{Document: doc, BaseVersionID: 0, RequestKey: "test-create-draft-1", Note: "联系电话必填"}
	v1, err := services.SaveProjectConfigurationDraft(tenant.ID, input, op)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := services.SaveProjectConfigurationDraft(tenant.ID, input, op)
	if err != nil || retry.ID != v1.ID {
		t.Fatalf("duplicate save: %v", err)
	}
	input.Note = "different payload"
	if _, err := services.SaveProjectConfigurationDraft(tenant.ID, input, op); !errors.Is(err, services.ErrProjectConfigConflict) {
		t.Fatalf("payload conflict: %v", err)
	}
	live, err := services.GetProjectConfiguration(tenant.ID)
	if err != nil || live.ActiveVersionID != 0 || len(live.Document.Projects) != 0 {
		t.Fatalf("draft affected live settings: %v", err)
	}
	if _, err := services.ApplyProjectConfiguration(tenant.ID, v1.ID, op); err != nil {
		t.Fatal(err)
	}
	if _, err := services.ApplyProjectConfiguration(tenant.ID, v1.ID, op); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := sqls.DB().Model(&models.ProjectConfigurationActivation{}).Where("version_id = ?", v1.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("duplicate activation: %d %v", count, err)
	}
	ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{Title: "Configuration version test", Description: "Synthetic support request", Source: "manual", Channel: "enterprise", TicketIntakeInput: dto.TicketIntakeInput{ProjectKey: "support", TicketType: "incident"}}, op)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.ProjectConfigVersionID != v1.ID {
		t.Fatalf("ticket not pinned: %+v", ticket)
	}
	doc.Projects[0].Name = "知识服务（改）"
	v2, err := services.SaveProjectConfigurationDraft(tenant.ID, services.ProjectConfigDraft{Document: doc, BaseVersionID: v1.ID, RequestKey: "test-create-draft-2", Note: "来电人必填"}, op)
	if err != nil {
		t.Fatal(err)
	}
	v3, err := services.SaveProjectConfigurationDraft(tenant.ID, services.ProjectConfigDraft{Document: doc, BaseVersionID: v1.ID, RequestKey: "test-create-draft-3", Note: "并发草稿"}, op)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := services.ApplyProjectConfiguration(tenant.ID, v2.ID, op); err != nil {
		t.Fatal(err)
	}
	if _, err := services.ApplyProjectConfiguration(tenant.ID, v3.ID, op); !errors.Is(err, services.ErrProjectConfigConflict) {
		t.Fatalf("stale apply: %v", err)
	}
	if refreshed := services.TicketService.Get(ticket.ID); refreshed == nil || refreshed.ProjectConfigVersionID != v1.ID {
		t.Fatal("old ticket switched to the new version")
	}
	if _, err := services.ApplyProjectConfiguration(tenant.ID, v1.ID, op); err != nil {
		t.Fatal(err)
	}
	view, err = services.GetProjectConfiguration(tenant.ID)
	if err != nil || view.ActiveVersionID != v2.ID {
		t.Fatal("retry reverted active version")
	}
	other := *op
	other.TenantID = tenant.ID + 100
	if _, err := services.ApplyProjectConfiguration(tenant.ID, v2.ID, &other); err == nil {
		t.Fatal("cross tenant write")
	}
	other = *op
	other.Permissions = nil
	if _, err := services.SaveProjectConfigurationDraft(tenant.ID, input, &other); err == nil {
		t.Fatal("missing permission accepted")
	}
	// A changed rule is restored by creating a new version, not altering v1/v2.
	restore, err := services.SaveProjectConfigurationDraft(tenant.ID, services.ProjectConfigDraft{Document: v1.Document, BaseVersionID: v2.ID, RequestKey: "test-restore-version", Note: "恢复原规则"}, op)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := services.ApplyProjectConfiguration(tenant.ID, restore.ID, op); err != nil {
		t.Fatal(err)
	}
	deploymentPath := filepath.Join(t.TempDir(), "current.json")
	bundle := projectconfig.Deployment{VersionID: restore.ID, Digest: restore.Digest, Document: restore.Document}
	deploymentJSON, _ := json.Marshal(bundle)
	if err := os.WriteFile(deploymentPath, deploymentJSON, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RHD_PROJECT_CONFIG_FILE", deploymentPath)
	t.Setenv("RHD_PROJECT_CONFIG_TENANT_ID", fmt.Sprint(tenant.ID))
	t.Setenv("RHD_PROJECT_CONFIG_DIGEST", restore.Digest)
	if err := services.VerifyProjectConfigurationDeployment(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RHD_PROJECT_CONFIG_DIGEST", v2.Digest)
	if err := services.VerifyProjectConfigurationDeployment(); err == nil {
		t.Fatal("deployment accepted wrong content")
	}
	t.Setenv("RHD_PROJECT_CONFIG_DIGEST", restore.Digest)
	// Subsequent checks exercise local activation, not the deployment-only gate.
	t.Setenv("RHD_PROJECT_CONFIG_FILE", "")
	// A secret disappearing after validation must fail a fresh apply.
	root := t.TempDir()
	t.Setenv("RHD_PROJECT_SECRET_DIR", root)
	dir := filepath.Join(root, fmt.Sprint(tenant.ID), "development")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "mail")
	if err := os.WriteFile(path, []byte("synthetic-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	doc.SecretRefs = []string{"secret://mail"}
	if report := projectconfig.Validate(doc, tenant.ID, "development", projectconfig.RuntimeSecretCheck); !report.Valid {
		t.Fatal(report)
	}
	secretDraft, err := services.SaveProjectConfigurationDraft(tenant.ID, services.ProjectConfigDraft{Document: doc, BaseVersionID: restore.ID, RequestKey: "test-secret-disappears", Note: "测试密钥检查"}, op)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := services.ApplyProjectConfiguration(tenant.ID, secretDraft.ID, op); err == nil {
		t.Fatal("missing secret accepted")
	}
	view, err = services.GetProjectConfiguration(tenant.ID)
	if err != nil || view.ActiveVersionID != restore.ID {
		t.Fatal("failed apply changed active version")
	}
	// An audit persistence failure must roll back the active pointer and projection.
	if err := sqls.DB().Callback().Create().Before("gorm:create").Register("test:reject-config-audit", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*models.ProjectConfigurationActivation); ok {
			tx.AddError(errors.New("synthetic audit failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqls.DB().Callback().Create().Remove("test:reject-config-audit") })
	doc.SecretRefs = []string{}
	failDraft, err := services.SaveProjectConfigurationDraft(tenant.ID, services.ProjectConfigDraft{Document: doc, BaseVersionID: restore.ID, RequestKey: "test-audit-rollback", Note: "事务失败回滚"}, op)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := services.ApplyProjectConfiguration(tenant.ID, failDraft.ID, op); err == nil {
		t.Fatal("audit failure ignored")
	}
	view, err = services.GetProjectConfiguration(tenant.ID)
	if err != nil || view.ActiveVersionID != restore.ID {
		t.Fatal("transaction did not roll back")
	}
	if err := sqls.DB().Model(&models.ProjectConfigurationVersion{}).Where("id = ?", v1.ID).Update("document_json", "{}").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := services.GetProjectConfiguration(tenant.ID); err == nil {
		t.Fatal("tampered historical configuration silently used or replaced")
	}
}

func TestProjectConfigurationDeploymentRequiresPinnedProduction(t *testing.T) {
	t.Setenv("RHD_PROJECT_CONFIG_FILE", "")
	t.Setenv("RHD_PROJECT_CONFIG_REQUIRED", "0")
	for _, environment := range []string{"production", "staging"} {
		t.Setenv("RHD_PROJECT_ENVIRONMENT", environment)
		if _, err := services.CheckProjectConfigurationDeploymentFile(); err == nil {
			t.Fatal("production gate bypassed")
		}
	}
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "development")
	if _, err := services.CheckProjectConfigurationDeploymentFile(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectConfigurationDeploymentActivation(t *testing.T) {
	t.Setenv("RHD_BOOTSTRAP_ADMIN_PASSWORD", "Configuration-test-only-2026!")
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "production")
	t.Setenv("RHD_PROJECT_CONFIG_FILE", "")
	t.Setenv("RHD_PROJECT_SECRET_DIR", "")
	setupTicketTestDB(t)
	tenant, _, _, _, _ := createTicketAfterSalesFixture(t, "config-deployment", enums.StatusOk)
	op := createTestOperator(t, "config-deployer")
	op.TenantID = tenant.ID
	op.Permissions = []string{constants.PermissionTicketUpdate.Code}
	view, err := services.GetProjectConfiguration(tenant.ID)
	if err != nil || !view.DeploymentManaged {
		t.Fatalf("production not marked deployment managed: %v", err)
	}
	save := func(base int64, key string) *services.ProjectConfigVersion {
		t.Helper()
		doc := view.Document
		doc.Projects = []projectconfig.Project{{Key: key, Name: key}}
		v, err := services.SaveProjectConfigurationDraft(tenant.ID, services.ProjectConfigDraft{Document: doc, BaseVersionID: base, RequestKey: "deployment-" + key, Note: key}, op)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	v1 := save(0, "first")
	if _, err := services.InitializeProjectConfigurationVersion(tenant.ID, v1.ID); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "current.json")
	t.Setenv("RHD_PROJECT_CONFIG_FILE", path)
	t.Setenv("RHD_PROJECT_CONFIG_TENANT_ID", fmt.Sprint(tenant.ID))
	writeBundle := func(v *services.ProjectConfigVersion) {
		t.Helper()
		b, _ := json.Marshal(projectconfig.Deployment{VersionID: v.ID, Digest: v.Digest, Document: v.Document})
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("RHD_PROJECT_CONFIG_DIGEST", v.Digest)
	}
	writeBundle(v1)
	v2 := save(v1.ID, "second")
	stale := save(v1.ID, "stale")
	if _, err := services.ApplyProjectConfiguration(tenant.ID, v2.ID, op); err == nil {
		t.Fatal("online apply changed a deployment-managed configuration")
	}
	// Normal restart must still validate against the unchanged mounted bundle.
	if err := services.VerifyProjectConfigurationDeployment(); err != nil {
		t.Fatal(err)
	}
	if _, err := services.InitializeProjectConfigurationVersion(tenant.ID, v2.ID); err == nil {
		t.Fatal("initializer replaced existing production configuration")
	}
	writeBundle(v2)
	if err := services.VerifyProjectConfigurationDeployment(); err == nil {
		t.Fatal("startup silently activated a pending deployment")
	}
	// A self-consistent export with the wrong stored content must also fail.
	wrong := *v2
	wrong.Document = v1.Document
	wrong.Digest = v1.Digest
	writeBundle(&wrong)
	if err := services.ApplyProjectConfigurationDeployment(); err == nil {
		t.Fatal("wrong stored digest accepted")
	}
	writeBundle(v2)
	t.Setenv("RHD_PROJECT_CONFIG_TENANT_ID", fmt.Sprint(tenant.ID+100))
	if err := services.ApplyProjectConfigurationDeployment(); err == nil {
		t.Fatal("cross tenant deployment accepted")
	}
	t.Setenv("RHD_PROJECT_CONFIG_TENANT_ID", fmt.Sprint(tenant.ID))
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "staging")
	if err := services.ApplyProjectConfigurationDeployment(); err == nil {
		t.Fatal("wrong environment deployment accepted")
	}
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "production")
	for i := 0; i < 2; i++ {
		if err := services.ApplyProjectConfigurationDeployment(); err != nil {
			t.Fatal(err)
		}
		if err := services.VerifyProjectConfigurationDeployment(); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := sqls.DB().Model(&models.ProjectConfigurationActivation{}).Where("version_id = ?", v2.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("deployment retry duplicated activation: %d %v", count, err)
	}
	for _, v := range []*services.ProjectConfigVersion{stale, v1} {
		writeBundle(v)
		if err := services.ApplyProjectConfigurationDeployment(); !errors.Is(err, services.ErrProjectConfigConflict) {
			t.Fatalf("stale deployment changed active configuration: %v", err)
		}
	}
	writeBundle(v2)
	if err := services.VerifyProjectConfigurationDeployment(); err != nil {
		t.Fatal(err)
	}
}

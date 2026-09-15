package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/deploymentidentity"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/projectconfig"
)

func TestDeploymentIdentityActivationRestartAndMissingMarkers(t *testing.T) {
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "staging")
	t.Setenv("RHD_INSTANCE_ID", "onward-acme-staging")
	t.Setenv("RHD_DEPLOYMENT_PROJECT_ID", "acme")
	t.Setenv("RHD_PROJECT_CONFIG_TENANT_ID", "1")
	t.Setenv("RHD_PROJECT_SECRET_DIR", "")
	t.Setenv("RHD_PROJECT_CONFIG_REQUIRED", "1")
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Tenant{}, &models.ProjectConfigurationState{}, &models.ProjectConfigurationVersion{}, &models.ProjectConfigurationActivation{}, &deploymentidentity.Record{}))
	require.NoError(t, db.Create(&models.Tenant{ID: 1, Name: "Synthetic staging"}).Error)
	doc := projectconfig.Document{SchemaVersion: 1, TenantID: 1, Environment: "staging", Projects: []projectconfig.Project{}, Intake: projectconfig.IntakePolicy{Rules: []projectconfig.Rule{}}, SecretRefs: []string{}}
	op := &dto.AuthPrincipal{TenantID: 1, Username: "fixture", Permissions: []string{constants.PermissionTicketUpdate.Code}}
	draft, err := SaveProjectConfigurationDraft(1, ProjectConfigDraft{Document: doc, RequestKey: "initial-instance", Note: "synthetic"}, op)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "current.json")
	writeBundle := func(v *ProjectConfigVersion) {
		t.Helper()
		data, err := json.Marshal(projectconfig.Deployment{VersionID: v.ID, Digest: v.Digest, Document: v.Document})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, data, 0600))
		t.Setenv("RHD_PROJECT_CONFIG_FILE", path)
		t.Setenv("RHD_PROJECT_CONFIG_DIGEST", v.Digest)
	}
	writeBundle(draft)
	require.NoError(t, CheckDeploymentIdentityBeforeMigrations(db, true))
	require.Error(t, CheckDeploymentIdentityBeforeMigrations(db, false))
	require.Error(t, VerifyProjectConfigurationDeployment(), "ordinary startup initialized identity")
	var count int64
	require.NoError(t, db.Model(&deploymentidentity.Record{}).Count(&count).Error)
	require.Zero(t, count)
	// Failure after the transaction hook must roll back its first identity row.
	require.NoError(t, db.Migrator().DropTable(&models.OutboxRecord{}))
	require.Error(t, ApplyProjectConfigurationDeployment())
	require.NoError(t, db.Model(&deploymentidentity.Record{}).Count(&count).Error)
	require.Zero(t, count)
	state, err := projectState(db, 1)
	require.NoError(t, err)
	require.Zero(t, state.ActiveVersionID)
	require.NoError(t, db.AutoMigrate(&models.OutboxRecord{}))
	require.NoError(t, ApplyProjectConfigurationDeployment())
	require.NoError(t, ApplyProjectConfigurationDeployment(), "same configuration deployment must be retryable")
	require.NoError(t, VerifyProjectConfigurationDeployment())
	t.Setenv("RHD_INSTANCE_ID", "onward-other-staging")
	t.Setenv("RHD_DEPLOYMENT_PROJECT_ID", "other")
	require.Error(t, CheckDeploymentIdentityBeforeMigrations(db, true))
	require.Error(t, ApplyProjectConfigurationDeployment(), "already-applied version skipped identity validation")
	t.Setenv("RHD_INSTANCE_ID", "")
	t.Setenv("RHD_DEPLOYMENT_PROJECT_ID", "")
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "development")
	t.Setenv("RHD_PROJECT_CONFIG_FILE", "")
	t.Setenv("RHD_PROJECT_CONFIG_REQUIRED", "0")
	require.Error(t, CheckDeploymentIdentityBeforeMigrations(db, false))
	require.Error(t, VerifyProjectConfigurationDeployment(), "removing environment markers bypassed bound database")
}

func TestSharedIntegrationInstanceRequiresConfigurationBundle(t *testing.T) {
	t.Setenv("RHD_INSTANCE_ID", "onward-shared-integration")
	t.Setenv("RHD_DEPLOYMENT_PROJECT_ID", "daypop-shared")
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "integration")
	t.Setenv("RHD_PROJECT_CONFIG_TENANT_ID", "7")
	t.Setenv("RHD_PROJECT_CONFIG_FILE", "")
	t.Setenv("RHD_PROJECT_CONFIG_REQUIRED", "0")
	_, err := CheckProjectConfigurationDeploymentFile()
	require.Error(t, err)
}

func TestDeploymentIdentityInitialCompanyAndConfigurationAreAtomic(t *testing.T) {
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "production")
	t.Setenv("RHD_INSTANCE_ID", "onward-acme-production")
	t.Setenv("RHD_DEPLOYMENT_PROJECT_ID", "acme")
	t.Setenv("RHD_PROJECT_CONFIG_TENANT_ID", "27")
	t.Setenv("RHD_PROJECT_SECRET_DIR", "")
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Tenant{}, &models.ProjectConfigurationState{}, &models.ProjectConfigurationVersion{}, &models.ProjectConfigurationActivation{}, &deploymentidentity.Record{}))
	doc := projectconfig.Document{SchemaVersion: 1, TenantID: 27, Environment: "production", Projects: []projectconfig.Project{}, Intake: projectconfig.IntakePolicy{Rules: []projectconfig.Rule{}}, SecretRefs: []string{}}
	require.NoError(t, db.AutoMigrate(models.Models...))
	options := ProjectDeploymentInitialization{TenantName: "Acme fixture", Administrator: ProjectDeploymentAdministrator{Username: "atomic.admin", Password: "Atomic-admin-fixture-2026!"}}
	_, err := InitializeProjectConfigurationDocument(doc, "")
	require.ErrorContains(t, err, "-project-tenant-name")
	require.NoError(t, db.Migrator().DropTable(&models.OutboxRecord{}))
	_, err = InitializeProjectConfigurationDocumentWithOptions(doc, options)
	require.Error(t, err)
	for _, model := range []any{&models.Tenant{}, &models.User{}, &models.TenantMember{}, &models.AuthRoleBinding{}, &models.AuthAuditLog{}, &models.ProjectConfigurationVersion{}, &models.ProjectConfigurationState{}, &deploymentidentity.Record{}} {
		var count int64
		require.NoError(t, db.Model(model).Count(&count).Error)
		require.Zero(t, count, "failed first install left partial rows for %T", model)
	}
	require.NoError(t, db.AutoMigrate(&models.OutboxRecord{}))
	version, err := InitializeProjectConfigurationDocumentWithOptions(doc, options)
	require.NoError(t, err)
	retried, err := InitializeProjectConfigurationDocument(doc, "Must not rename existing company")
	require.NoError(t, err)
	require.Equal(t, version.ID, retried.ID)
	var tenant models.Tenant
	require.NoError(t, db.First(&tenant, 27).Error)
	require.Equal(t, "Acme fixture", tenant.Name)
	require.False(t, tenant.IsAIEnabled(), "bootstrap must not activate external AI integration")
	require.NoError(t, CheckDeploymentIdentityBeforeMigrations(db, false))
	doc.Projects = []projectconfig.Project{{Key: "new", Name: "different"}}
	_, err = InitializeProjectConfigurationDocument(doc, "Acme fixture")
	require.ErrorContains(t, err, "不会替换")
	state, err := projectState(db, 27)
	require.NoError(t, err)
	require.Equal(t, version.ID, state.ActiveVersionID)
}

func TestDeploymentIdentityCannotAdoptLegacyOtherEnvironmentOrCompany(t *testing.T) {
	t.Setenv("RHD_INSTANCE_ID", "")
	t.Setenv("RHD_DEPLOYMENT_PROJECT_ID", "")
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "production")
	t.Setenv("RHD_PROJECT_CONFIG_TENANT_ID", "27")
	t.Setenv("RHD_PROJECT_SECRET_DIR", "")
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Tenant{}, &models.ProjectConfigurationState{}, &models.ProjectConfigurationVersion{}, &models.ProjectConfigurationActivation{}))
	require.NoError(t, db.AutoMigrate(models.Models...))
	doc := projectconfig.Document{SchemaVersion: 1, TenantID: 27, Environment: "production", Projects: []projectconfig.Project{}, Intake: projectconfig.IntakePolicy{Rules: []projectconfig.Rule{}}, SecretRefs: []string{}}
	legacy, err := InitializeProjectConfigurationDocumentWithOptions(doc, ProjectDeploymentInitialization{TenantName: "Legacy production fixture", Administrator: ProjectDeploymentAdministrator{Username: "legacy.admin", Password: "Legacy-admin-fixture-2026!"}})
	require.NoError(t, err)
	// Reproduce upgrading a pre-FND-003 database: no identity table exists yet.
	require.False(t, db.Migrator().HasTable(&deploymentidentity.Record{}))
	t.Setenv("RHD_INSTANCE_ID", "onward-acme-staging")
	t.Setenv("RHD_DEPLOYMENT_PROJECT_ID", "acme")
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "staging")
	require.ErrorContains(t, CheckDeploymentIdentityBeforeMigrations(db, true), "已有其他公司或环境")
	doc.Environment = "staging"
	_, err = InitializeProjectConfigurationDocument(doc, "Must not adopt production")
	require.ErrorContains(t, err, "已有其他公司或环境")
	var count int64
	require.NoError(t, db.Model(&models.ProjectConfigurationState{}).Where("environment = ?", "staging").Count(&count).Error)
	require.Zero(t, count)
	// A multi-company legacy database also needs an explicit data migration.
	t.Setenv("RHD_INSTANCE_ID", "onward-acme-production")
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "production")
	require.NoError(t, db.Create(&models.ProjectConfigurationState{TenantID: 28, Environment: "production", ActiveVersionID: 99}).Error)
	require.ErrorContains(t, CheckDeploymentIdentityBeforeMigrations(db, true), "已有其他公司或环境")
	require.NoError(t, db.Where("tenant_id = ?", 28).Delete(&models.ProjectConfigurationState{}).Error)
	// Same-company/same-environment upgrade remains supported after migration133.
	require.NoError(t, CheckDeploymentIdentityBeforeMigrations(db, true))
	require.NoError(t, db.AutoMigrate(&deploymentidentity.Record{}))
	doc.Environment = "production"
	adopted, err := InitializeProjectConfigurationDocument(doc, "Must not rename")
	require.NoError(t, err)
	require.Equal(t, legacy.ID, adopted.ID)
	require.NoError(t, CheckDeploymentIdentityBeforeMigrations(db, false))
	var tenant models.Tenant
	require.NoError(t, db.First(&tenant, 27).Error)
	require.Equal(t, "Legacy production fixture", tenant.Name)
}

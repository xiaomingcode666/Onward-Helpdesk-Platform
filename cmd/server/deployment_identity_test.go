package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/deploymentidentity"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/projectconfig"
	"remotehelpdesk/internal/services"
)

func TestDeploymentIdentityRejectsWrongDatabaseBeforeMigrations(t *testing.T) {
	dir := t.TempDir()
	previous := config.CurrentOrDefault()
	t.Cleanup(func() { config.SetCurrent(&previous) })
	dbConfig := config.DBConfig{Type: "sqlite", DSN: filepath.ToSlash(filepath.Join(dir, "identity.db"))}
	db, err := bootstrap.InitDB(dbConfig)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&deploymentidentity.Record{}))
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return deploymentidentity.Bind(tx, &deploymentidentity.Identity{InstanceID: "onward-acme-production", ProjectID: "acme", Environment: "production", TenantID: 7})
	}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	settings := fmt.Sprintf("encryptionKey: identity-test-only-long-fixture-key\ndb:\n  type: sqlite\n  dsn: %s\n  autoMigrate: true\n", dbConfig.DSN)
	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(settings), 0600))
	doc := projectconfig.Document{SchemaVersion: 1, TenantID: 7, Environment: "staging", Projects: []projectconfig.Project{}, Intake: projectconfig.IntakePolicy{Rules: []projectconfig.Rule{}}, SecretRefs: []string{}}
	bundle := projectconfig.Deployment{VersionID: 1, Digest: projectconfig.Digest(doc), Document: doc}
	encoded, err := json.Marshal(bundle)
	require.NoError(t, err)
	bundlePath := filepath.Join(dir, "current.json")
	require.NoError(t, os.WriteFile(bundlePath, encoded, 0600))
	t.Setenv("RHD_INSTANCE_ID", "onward-acme-staging")
	t.Setenv("RHD_DEPLOYMENT_PROJECT_ID", "acme")
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "staging")
	t.Setenv("RHD_PROJECT_CONFIG_TENANT_ID", "7")
	t.Setenv("RHD_PROJECT_CONFIG_FILE", bundlePath)
	t.Setenv("RHD_PROJECT_CONFIG_DIGEST", bundle.Digest)
	t.Setenv("RHD_PROJECT_SECRET_DIR", "")
	for _, apply := range []bool{false, true} {
		err := migrateProjectDeployment(configPath, apply)
		require.ErrorContains(t, err, "数据库已绑定其他部署实例")
	}
	input, err := json.Marshal(doc)
	require.NoError(t, err)
	inputPath := filepath.Join(dir, "input.json")
	require.NoError(t, os.WriteFile(inputPath, input, 0600))
	err = initializeProjectConfiguration(configPath, inputPath, filepath.Join(dir, "initial.json"), 7, "staging", "")
	require.ErrorContains(t, err, "数据库已绑定其他部署实例")
	db, err = bootstrap.InitDB(dbConfig)
	require.NoError(t, err)
	sqlDB, err = db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	require.False(t, db.Migrator().HasTable(&models.Tenant{}), "wrong-environment connection performed schema migrations")
	var identity deploymentidentity.Record
	require.NoError(t, db.First(&identity).Error)
	require.Equal(t, "production", identity.Environment)
}

func TestDeploymentIdentityFreshInstallCreatesOnlyExplicitMissingCompany(t *testing.T) {
	t.Setenv("RHD_BOOTSTRAP_ADMIN_PASSWORD", "Identity-fixture-only-2026!")
	t.Setenv("RHD_INSTANCE_ID", "onward-acme-staging")
	t.Setenv("RHD_DEPLOYMENT_PROJECT_ID", "acme")
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "staging")
	t.Setenv("RHD_PROJECT_CONFIG_TENANT_ID", "27")
	t.Setenv("RHD_PROJECT_SECRET_DIR", "")
	previous := config.CurrentOrDefault()
	t.Cleanup(func() { config.SetCurrent(&previous) })
	dir := t.TempDir()
	dbPath := filepath.ToSlash(filepath.Join(dir, "fresh.db"))
	configPath := filepath.Join(dir, "config.yaml")
	settings := fmt.Sprintf("encryptionKey: identity-test-only-long-fixture-key\ndb:\n  type: sqlite\n  dsn: %s\n  autoMigrate: true\n", dbPath)
	require.NoError(t, os.WriteFile(configPath, []byte(settings), 0600))
	doc := projectconfig.Document{SchemaVersion: 1, TenantID: 27, Environment: "staging", Projects: []projectconfig.Project{}, Intake: projectconfig.IntakePolicy{Rules: []projectconfig.Rule{}}, SecretRefs: []string{}}
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	inputPath := filepath.Join(dir, "input.json")
	outputPath := filepath.Join(dir, "current.json")
	require.NoError(t, os.WriteFile(inputPath, raw, 0600))
	options := services.ProjectDeploymentInitialization{TenantName: "Acme fixture", Administrator: services.ProjectDeploymentAdministrator{Username: "acme.admin", Password: "Acme-admin-fixture-2026!"}}
	require.NoError(t, initializeProjectConfigurationWithOptions(configPath, inputPath, outputPath, 27, "staging", "", options))
	bundle, err := projectconfig.ReadDeployment(outputPath, 27, "staging", projectconfig.Digest(doc), "")
	require.NoError(t, err)
	t.Setenv("RHD_PROJECT_CONFIG_FILE", outputPath)
	t.Setenv("RHD_PROJECT_CONFIG_DIGEST", bundle.Digest)
	require.NoError(t, migrateProjectDeployment(configPath, false))
	require.NoError(t, initializeProjectConfiguration(configPath, inputPath, filepath.Join(dir, "retry.json"), 27, "staging", "", "Never rename"))
	db, err := bootstrap.InitDB(config.DBConfig{Type: "sqlite", DSN: dbPath})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	var tenant models.Tenant
	require.NoError(t, db.First(&tenant, 27).Error)
	require.Equal(t, "Acme fixture", tenant.Name)
	var identity deploymentidentity.Record
	require.NoError(t, db.First(&identity).Error)
	require.Equal(t, int64(27), identity.TenantID)
	require.Equal(t, "onward-acme-staging", identity.InstanceID)
	login, err := services.AuthService.Login(request.LoginRequest{Username: options.Administrator.Username, Password: options.Administrator.Password, DomainType: models.DomainTypeEnterprise, PortalChoiceConfirmed: true}, config.CurrentOrDefault().Auth, "127.0.0.1", "deployment-fixture")
	require.NoError(t, err)
	require.Equal(t, int64(27), login.TenantID)
	require.Equal(t, models.DomainTypeEnterprise, login.DomainType)
	require.Contains(t, login.Roles, services.EnterpriseRoleOwner)
	require.Contains(t, login.Permissions, constants.PermissionTicketUpdate.Code)
	var platformAdmin models.User
	require.NoError(t, db.Where("username = ?", constants.BootstrapAdminUsername).First(&platformAdmin).Error)
	entered, err := services.PlatformIAMService.EnterTenant(request.PlatformTenantEnterRequest{TenantID: 27}, &dto.AuthPrincipal{UserID: platformAdmin.ID, Username: platformAdmin.Username}, config.CurrentOrDefault().Auth, "127.0.0.1", "deployment-fixture")
	require.NoError(t, err)
	require.Equal(t, int64(27), entered.TenantID)
}

func TestDeploymentIdentityRawConfigurationPreflight(t *testing.T) {
	doc := projectconfig.Document{SchemaVersion: 1, TenantID: 27, Environment: "integration", Projects: []projectconfig.Project{}, Intake: projectconfig.IntakePolicy{Rules: []projectconfig.Rule{}}, SecretRefs: []string{}}
	path := filepath.Join(t.TempDir(), "input.json")
	write := func() {
		data, err := json.Marshal(doc)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, data, 0600))
	}
	write()
	got, err := readInitialProjectConfiguration(path, 27, "integration", "")
	require.NoError(t, err)
	require.Equal(t, doc, got)
	_, err = readInitialProjectConfiguration(path, 28, "integration", "")
	require.Error(t, err)
	_, err = readInitialProjectConfiguration(path, 27, "production", "")
	require.Error(t, err)
	doc.SecretRefs = []string{"secret://missing-fixture-credential"}
	write()
	_, err = readInitialProjectConfiguration(path, 27, "integration", t.TempDir())
	require.Error(t, err)
}

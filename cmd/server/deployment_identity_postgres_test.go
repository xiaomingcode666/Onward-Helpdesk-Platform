package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/deploymentidentity"
	"remotehelpdesk/internal/pkg/projectconfig"
	"remotehelpdesk/internal/services"
)

// Opt-in only. The supplied connection must be the local server's maintenance
// database; the test creates and drops its own randomly named database, never
// runs migrations against the supplied database, and never uses project .env.
func TestDeploymentIdentityPostgresInitialization(t *testing.T) {
	raw := os.Getenv("RHD_TEST_PG_DSN")
	if raw == "" {
		t.Skip("set RHD_TEST_PG_DSN to an isolated local PostgreSQL maintenance database")
	}
	u, err := url.Parse(raw)
	require.NoError(t, err)
	require.Contains(t, []string{"postgres", "postgresql"}, u.Scheme)
	require.Contains(t, []string{"127.0.0.1", "localhost", "::1"}, u.Hostname(), "integration tests require a local isolated server")
	require.Equal(t, "/postgres", u.Path, "the provided database must be postgres, not a business database")
	admin, err := gorm.Open(postgres.Open(raw), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	name := "fnd003_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	validName := regexp.MustCompile(`^fnd003_test_[a-f0-9]{32}$`)
	require.True(t, validName.MatchString(name))
	require.NoError(t, admin.Exec("CREATE DATABASE "+name).Error)
	t.Cleanup(func() {
		if !validName.MatchString(name) {
			t.Error("refusing to clean an unexpected database name")
			return
		}
		if err := admin.Exec("DROP DATABASE " + name).Error; err != nil {
			t.Errorf("drop isolated test database: %v", err)
		}
		adminSQL.Close()
	})
	u.Path = "/" + name
	dsn := u.String()
	check, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	var current string
	require.NoError(t, check.Raw("SELECT current_database()").Scan(&current).Error)
	require.Equal(t, name, current)
	checkSQL, err := check.DB()
	require.NoError(t, err)
	require.NoError(t, checkSQL.Close())

	t.Setenv("RHD_BOOTSTRAP_ADMIN_PASSWORD", "Fnd003-pg-fixture-only-2026!")
	t.Setenv("RHD_INSTANCE_ID", "onward-acme-staging")
	t.Setenv("RHD_DEPLOYMENT_PROJECT_ID", "acme")
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "staging")
	t.Setenv("RHD_PROJECT_CONFIG_TENANT_ID", "27")
	t.Setenv("RHD_PROJECT_SECRET_DIR", "")
	previous := config.CurrentOrDefault()
	t.Cleanup(func() { config.SetCurrent(&previous) })
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	settings := fmt.Sprintf("encryptionKey: fnd003-postgres-fixture-only-key\ndb:\n  type: postgres\n  dsn: %q\n  autoMigrate: true\n", dsn)
	require.NoError(t, os.WriteFile(configPath, []byte(settings), 0600))
	doc := projectconfig.Document{SchemaVersion: 1, TenantID: 27, Environment: "staging", Projects: []projectconfig.Project{}, Intake: projectconfig.IntakePolicy{Rules: []projectconfig.Rule{}}, SecretRefs: []string{}}
	input, err := json.Marshal(doc)
	require.NoError(t, err)
	inputPath := filepath.Join(dir, "input.json")
	outputPath := filepath.Join(dir, "current.json")
	require.NoError(t, os.WriteFile(inputPath, input, 0600))
	options := services.ProjectDeploymentInitialization{TenantName: "PostgreSQL fixture", Administrator: services.ProjectDeploymentAdministrator{Username: "postgres.fixture.admin", Password: "Postgres-admin-fixture-2026!"}}
	require.NoError(t, initializeProjectConfigurationWithOptions(configPath, inputPath, outputPath, 27, "staging", "", options))
	require.NoError(t, initializeProjectConfiguration(configPath, inputPath, filepath.Join(dir, "retry.json"), 27, "staging", "", "Must not rename"))
	t.Setenv("RHD_PROJECT_CONFIG_FILE", outputPath)
	t.Setenv("RHD_PROJECT_CONFIG_DIGEST", projectconfig.Digest(doc))
	require.NoError(t, migrateProjectDeployment(configPath, false))
	db, err := bootstrap.InitDB(config.DBConfig{Type: "postgres", DSN: dsn})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	var tenant models.Tenant
	require.NoError(t, db.First(&tenant, 27).Error)
	require.Equal(t, "PostgreSQL fixture", tenant.Name)
	// Exercise the PostgreSQL sequence advanced by an explicit tenant ID.
	next := models.Tenant{Name: "Subsequent automatic-ID fixture"}
	require.NoError(t, db.Create(&next).Error)
	require.Greater(t, next.ID, int64(27))
	var binding deploymentidentity.Record
	require.NoError(t, db.First(&binding).Error)
	require.Equal(t, int64(27), binding.TenantID)
	t.Setenv("RHD_INSTANCE_ID", "onward-other-staging")
	t.Setenv("RHD_DEPLOYMENT_PROJECT_ID", "other")
	require.ErrorContains(t, migrateProjectDeployment(configPath, true), "数据库已绑定其他部署实例")
}

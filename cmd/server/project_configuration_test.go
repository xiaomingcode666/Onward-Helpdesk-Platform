package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/projectconfig"
	"remotehelpdesk/internal/services"
)

func TestInitializeProjectConfigurationIsScopedAndNonDestructive(t *testing.T) {
	t.Setenv("RHD_BOOTSTRAP_ADMIN_PASSWORD", "Configuration-test-only-2026!")
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "production")
	t.Setenv("RHD_PROJECT_SECRET_DIR", "")
	previous := config.CurrentOrDefault()
	t.Cleanup(func() { config.SetCurrent(&previous) })
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	inputPath := filepath.Join(dir, "input.json")
	dbPath := filepath.ToSlash(filepath.Join(dir, "config-test.db"))
	settings := fmt.Sprintf("encryptionKey: configuration-test-only-2026-long-key\ndb:\n  type: sqlite\n  dsn: file:%s?_busy_timeout=5000\n  autoMigrate: true\n", dbPath)
	if err := os.WriteFile(configPath, []byte(settings), 0600); err != nil {
		t.Fatal(err)
	}
	doc := projectconfig.Document{SchemaVersion: 1, TenantID: 1, Environment: "production", Projects: []projectconfig.Project{}, Intake: projectconfig.IntakePolicy{Rules: []projectconfig.Rule{}}, SecretRefs: []string{}}
	writeInput := func() {
		b, _ := json.Marshal(doc)
		if err := os.WriteFile(inputPath, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeInput()
	outputPath := filepath.Join(dir, "current.json")
	options := services.ProjectDeploymentInitialization{Administrator: services.ProjectDeploymentAdministrator{Username: "configuration.admin", Password: "Configuration-admin-fixture-2026!"}}
	if err := initializeProjectConfigurationWithOptions(configPath, inputPath, outputPath, 1, "production", "", options); err != nil {
		t.Fatal(err)
	}
	first, err := projectconfig.ReadDeployment(outputPath, 1, "production", projectconfig.Digest(doc), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("RHD_PROJECT_CONFIG_FILE", outputPath)
	t.Setenv("RHD_PROJECT_CONFIG_TENANT_ID", "1")
	t.Setenv("RHD_PROJECT_CONFIG_DIGEST", first.Digest)
	if err := migrateProjectDeployment(configPath, false); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := initializeProjectConfiguration(configPath, inputPath, outputPath, 1, "production", ""); err != nil {
		t.Fatal("same output retry rejected", err)
	}
	after, err := os.ReadFile(outputPath)
	if err != nil || string(before) != string(after) {
		t.Fatal("existing output overwritten", err)
	}
	retryPath := filepath.Join(dir, "retry.json")
	if err := initializeProjectConfiguration(configPath, inputPath, retryPath, 1, "production", ""); err != nil {
		t.Fatal(err)
	}
	retry, err := projectconfig.ReadDeployment(retryPath, 1, "production", projectconfig.Digest(doc), "")
	if err != nil || retry.VersionID != first.VersionID {
		t.Fatalf("retry changed version: %v", err)
	}
	doc.Projects = []projectconfig.Project{{Key: "different", Name: "不同配置"}}
	writeInput()
	if err := initializeProjectConfiguration(configPath, inputPath, filepath.Join(dir, "rejected.json"), 1, "production", ""); err == nil {
		t.Fatal("initializer replaced active configuration")
	}
	if _, err := os.Stat(filepath.Join(dir, "rejected.json")); !os.IsNotExist(err) {
		t.Fatal("failed initialization left output")
	}
	if err := initializeProjectConfiguration(configPath, inputPath, filepath.Join(dir, "wrong-env.json"), 1, "staging", ""); err == nil {
		t.Fatal("wrong environment accepted")
	}
	// Exercise the deployment command against the same persisted database,
	// closing/reopening it as separate CLI invocations do.
	db, err := bootstrap.InitDB(config.Current().DB)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	op := &dto.AuthPrincipal{TenantID: 1, Username: "deployment-test", Permissions: []string{constants.PermissionTicketUpdate.Code}}
	draft, err := services.SaveProjectConfigurationDraft(1, services.ProjectConfigDraft{Document: doc, BaseVersionID: first.VersionID, RequestKey: "cli-next-draft", Note: "next deployment"}, op)
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(projectconfig.Deployment{VersionID: draft.ID, Digest: draft.Digest, Document: draft.Document})
	if err := os.WriteFile(outputPath, b, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RHD_PROJECT_CONFIG_DIGEST", draft.Digest)
	if err := migrateProjectDeployment(configPath, false); err == nil {
		t.Fatal("read-only binding check activated draft")
	}
	if err := migrateProjectDeployment(configPath, true); err != nil {
		t.Fatal(err)
	}
	if err := migrateProjectDeployment(configPath, true); err != nil {
		t.Fatal(err)
	}
	if err := migrateProjectDeployment(configPath, false); err != nil {
		t.Fatalf("restart binding failed: %v", err)
	}
}

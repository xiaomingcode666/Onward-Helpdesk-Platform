package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/projectconfig"
	"remotehelpdesk/internal/services"
)

func main() {
	configPath := flag.String("config", "config/config.yaml", "path to config file")
	migrateOnly := flag.Bool("migrate", false, "apply database migrations and verify deployment binding, then exit")
	applyProject := flag.Bool("apply-project-config", false, "with -migrate, activate the validated deployment bundle before verifying binding")
	projectFile := flag.String("check-project-config", "", "validate an exported project configuration and exit without starting services")
	projectDocument := flag.String("check-project-config-document", "", "validate a raw project configuration document without connecting to a database")
	initializeProject := flag.String("initialize-project-config", "", "initialize an unconfigured tenant/environment from a raw configuration document, then exit")
	projectOutput := flag.String("project-output", "", "output file for the initialized bundle; an existing identical current bundle allows retry and is never overwritten")
	projectTenant := flag.Int64("project-tenant", 0, "expected project configuration tenant")
	projectTenantName := flag.String("project-tenant-name", "", "with -initialize-project-config, create only a missing company with this display name; existing companies are never renamed")
	projectAdminUserID := flag.Int64("project-admin-user-id", 0, "with -initialize-project-config, bind an existing standalone login account as company administrator")
	projectAdminUsername := flag.String("project-admin-username", "", "with -initialize-project-config, create this new company administrator account")
	projectAdminPasswordFile := flag.String("project-admin-password-file", "", "operator-owned bootstrap-admin-password file in the company's environment secret directory; never pass a password on the command line")
	projectEnvironment := flag.String("project-environment", "", "expected project configuration environment")
	projectDigest := flag.String("project-digest", "", "expected project configuration SHA-256")
	secretRoot := flag.String("project-secret-dir", "", "operator-managed tenant/environment secret root")
	flag.Parse()
	if *initializeProject == "" && (*projectAdminUserID != 0 || *projectAdminUsername != "" || *projectAdminPasswordFile != "") {
		fmt.Fprintln(os.Stderr, "管理员初始化参数只能与 -initialize-project-config 一起使用")
		os.Exit(1)
	}
	if *applyProject && (!*migrateOnly || *initializeProject != "" || *projectFile != "" || *projectDocument != "") {
		fmt.Fprintln(os.Stderr, "-apply-project-config 只能与 -migrate 一起使用")
		os.Exit(1)
	}
	if *projectDocument != "" {
		if *migrateOnly || *initializeProject != "" || *projectFile != "" {
			fmt.Fprintln(os.Stderr, "-check-project-config-document 不能与迁移、初始化或配置包检查同时使用")
			os.Exit(1)
		}
		doc, err := readInitialProjectConfiguration(*projectDocument, *projectTenant, *projectEnvironment, *secretRoot)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("Project configuration document verified: %s\n", projectconfig.Digest(doc))
		return
	}
	if *initializeProject != "" {
		admin, err := readProjectDeploymentAdministrator(*projectAdminUserID, *projectAdminUsername, *projectAdminPasswordFile, *secretRoot, *projectTenant, *projectEnvironment)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		options := services.ProjectDeploymentInitialization{TenantName: *projectTenantName, Administrator: admin}
		if err := initializeProjectConfigurationWithOptions(*configPath, *initializeProject, *projectOutput, *projectTenant, *projectEnvironment, *secretRoot, options); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *projectFile != "" {
		bundle, err := projectconfig.ReadDeployment(*projectFile, *projectTenant, *projectEnvironment, *projectDigest, *secretRoot)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("Project configuration V%d verified: %s\n", bundle.VersionID, bundle.Digest)
		return
	}
	if *migrateOnly {
		if err := migrateProjectDeployment(*configPath, *applyProject); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := bootstrap.Init(*configPath); err != nil {
		slog.Error("bootstrap init failed", "error", err)
		os.Exit(1)
	}

	cfg := config.Current()

	app, err := bootstrap.NewServer()
	if err != nil {
		slog.Error("bootstrap server failed", "error", err)
		return
	}

	if err := app.Run(cfg.Server.Address()); err != nil {
		slog.Error("start server failed", "error", err)
		return
	}
}

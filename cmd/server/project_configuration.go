package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/deploymentidentity"
	"remotehelpdesk/internal/pkg/projectconfig"
	"remotehelpdesk/internal/services"
)

// Used by the existing Compose deployment workflow; no listeners or workers.
func migrateProjectDeployment(configPath string, applyConfiguration bool) error {
	if _, err := services.CheckProjectConfigurationDeploymentFile(); err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	config.SetCurrent(cfg)
	db, err := bootstrap.InitDB(cfg.DB)
	if err != nil {
		return err
	}
	if sqlDB, err := db.DB(); err == nil {
		defer sqlDB.Close()
	}
	if err := services.CheckDeploymentIdentityBeforeMigrations(db, applyConfiguration); err != nil {
		return err
	}
	if err := bootstrap.InitMigrations(); err != nil {
		return err
	}
	if applyConfiguration {
		if err := services.ApplyProjectConfigurationDeployment(); err != nil {
			return err
		}
	}
	return services.VerifyProjectConfigurationDeployment()
}

// This is an explicit, local provisioning operation, not an HTTP bypass.
// It requires operator access to the database configuration and secret files.
// It never replaces an already active configuration with different content.
func initializeProjectConfiguration(configPath, inputPath, outputPath string, tenantID int64, environment, secretRoot string, tenantName ...string) (resultErr error) {
	options := services.ProjectDeploymentInitialization{}
	if len(tenantName) > 0 {
		options.TenantName = tenantName[0]
	}
	return initializeProjectConfigurationWithOptions(configPath, inputPath, outputPath, tenantID, environment, secretRoot, options)
}

func initializeProjectConfigurationWithOptions(configPath, inputPath, outputPath string, tenantID int64, environment, secretRoot string, options services.ProjectDeploymentInitialization) (resultErr error) {
	if outputPath == "" {
		return fmt.Errorf("首次配置必须指定 -project-output 文件")
	}
	if err := options.Administrator.Validate(); err != nil {
		return err
	}
	doc, err := readInitialProjectConfiguration(inputPath, tenantID, environment, secretRoot)
	if err != nil {
		return err
	}
	var out *os.File
	if info, statErr := os.Lstat(outputPath); statErr == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("已有输出必须是普通配置文件，不能使用链接")
		}
		bundle, err := projectconfig.ReadDeployment(outputPath, tenantID, environment, projectconfig.Digest(doc), secretRoot)
		if err != nil {
			return fmt.Errorf("已有输出配置与本次初始化不一致，不会覆盖文件")
		}
		options.ExistingVersionID = bundle.VersionID
	} else if os.IsNotExist(statErr) {
		out, err = os.OpenFile(outputPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("输出文件无法创建；不会覆盖原文件")
		}
	} else {
		return fmt.Errorf("输出文件无法检查")
	}
	defer func() {
		if out != nil {
			out.Close()
			if resultErr != nil {
				_ = os.Remove(outputPath)
			}
		}
	}()
	if err := os.Setenv("RHD_PROJECT_ENVIRONMENT", environment); err != nil {
		return err
	}
	if err := os.Setenv("RHD_PROJECT_SECRET_DIR", secretRoot); err != nil {
		return err
	}
	identity, err := deploymentidentity.FromEnvironment()
	if err != nil {
		return err
	}
	if identity != nil && identity.TenantID != tenantID {
		return fmt.Errorf("首次配置的公司编号与部署实例不一致")
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	config.SetCurrent(cfg)
	db, err := bootstrap.InitDB(cfg.DB)
	if err != nil {
		return err
	}
	if sqlDB, err := db.DB(); err == nil {
		defer sqlDB.Close()
	}
	if err := services.CheckDeploymentIdentityBeforeMigrations(db, true); err != nil {
		return err
	}
	if err := bootstrap.InitMigrations(); err != nil {
		return err
	}
	version, err := services.InitializeProjectConfigurationDocumentWithOptions(doc, options)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(projectconfig.Deployment{VersionID: version.ID, Digest: version.Digest, Document: version.Document}, "", "  ")
	if err != nil {
		return err
	}
	if out != nil {
		if _, err := out.Write(encoded); err != nil {
			return err
		}
		if err := out.Sync(); err != nil {
			return err
		}
	}
	fmt.Printf("Initialized project configuration V%d; deployment bundle: %s\n", version.ID, outputPath)
	fmt.Printf("Project configuration digest: %s\n", version.Digest)
	return nil
}

func readProjectDeploymentAdministrator(userID int64, username, passwordFile, secretRoot string, tenantID int64, environment string) (services.ProjectDeploymentAdministrator, error) {
	admin := services.ProjectDeploymentAdministrator{UserID: userID, Username: username}
	if passwordFile != "" {
		if userID != 0 || username == "" {
			return admin, fmt.Errorf("密码文件只能与新管理员账号名称一起使用")
		}
		expected, err := filepath.Abs(filepath.Join(secretRoot, fmt.Sprint(tenantID), environment, "bootstrap-admin-password"))
		actual, actualErr := filepath.Abs(passwordFile)
		if err != nil || actualErr != nil || secretRoot == "" || actual != expected {
			return admin, fmt.Errorf("管理员密码文件必须在当前公司及环境的密钥目录，名称为 bootstrap-admin-password")
		}
		secret, err := projectconfig.ReadSecret(secretRoot, tenantID, environment, "secret://bootstrap-admin-password")
		if err != nil {
			return admin, fmt.Errorf("管理员密码文件不可安全读取")
		}
		info, err := os.Stat(passwordFile)
		if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
			return admin, fmt.Errorf("管理员密码文件必须限制为仅文件所有者可读取（例如 chmod 600）")
		}
		admin.Password = strings.TrimSuffix(strings.TrimSuffix(string(secret), "\n"), "\r")
	}
	return admin, admin.Validate()
}

// Used by first-install preflight; deliberately has no database/config-loader calls.
func readInitialProjectConfiguration(path string, tenantID int64, environment, secretRoot string) (projectconfig.Document, error) {
	var doc projectconfig.Document
	f, err := os.Open(path)
	if err != nil {
		return doc, fmt.Errorf("初始配置文件不可读取")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 256*1024+1))
	if err != nil {
		return doc, fmt.Errorf("初始配置文件读取失败")
	}
	doc, err = projectconfig.Decode(b)
	if err != nil {
		return doc, err
	}
	report := projectconfig.Validate(doc, tenantID, environment, func(t int64, e, ref string) error { return projectconfig.CheckSecret(secretRoot, t, e, ref) })
	if !report.Valid {
		return doc, fmt.Errorf("初始配置校验失败：%s", report.Issues[0].Message)
	}
	return doc, nil
}

package services

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/deploymentidentity"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/projectconfig"
)

func TestDeploymentAdministratorRepairRetryAndConflicts(t *testing.T) {
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "staging")
	t.Setenv("RHD_INSTANCE_ID", "onward-acme-staging")
	t.Setenv("RHD_DEPLOYMENT_PROJECT_ID", "acme")
	t.Setenv("RHD_PROJECT_CONFIG_TENANT_ID", "27")
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(models.Models...))
	require.NoError(t, db.AutoMigrate(&deploymentidentity.Record{}))
	doc := projectconfig.Document{SchemaVersion: 1, TenantID: 27, Environment: "staging", Projects: []projectconfig.Project{}, Intake: projectconfig.IntakePolicy{Rules: []projectconfig.Rule{}}, SecretRefs: []string{}}
	options := ProjectDeploymentInitialization{TenantName: "Repair fixture"}
	_, err := InitializeProjectConfigurationDocumentWithOptions(doc, options)
	require.ErrorContains(t, err, "公司缺少企业管理员")
	var count int64
	require.NoError(t, db.Model(&models.Tenant{}).Count(&count).Error)
	require.Zero(t, count, "missing administrator must not leave a half-created company")
	// A pre-fix install has already created the company. The same explicit
	// initialization can repair it, without creating a replacement company.
	require.NoError(t, db.Create(&models.Tenant{ID: 27, Name: "Original company", Status: enums.StatusOk}).Error)
	draft, err := SaveProjectConfigurationDraft(27, ProjectConfigDraft{Document: doc, RequestKey: "pre-fix-initial-config", Note: "Synthetic pre-fix initialization"}, &dto.AuthPrincipal{TenantID: 27, Permissions: []string{constants.PermissionTicketUpdate.Code}})
	require.NoError(t, err)
	_, err = InitializeProjectConfigurationVersion(27, draft.ID)
	require.NoError(t, err)
	options.ExistingVersionID = draft.ID
	options.Administrator = ProjectDeploymentAdministrator{Username: "repair.admin", Password: "Original-admin-password-2026!"}
	v, err := InitializeProjectConfigurationDocumentWithOptions(doc, options)
	require.NoError(t, err)
	require.Equal(t, draft.ID, v.ID, "repair must keep the original active configuration")
	var originalUser models.User
	require.NoError(t, db.Where("username = ?", options.Administrator.Username).First(&originalUser).Error)
	var originalMember models.TenantMember
	require.NoError(t, db.Where("tenant_id = ?", 27).First(&originalMember).Error)
	options.ExistingVersionID = v.ID
	options.Administrator.Password = "Different-file-password-2026!"
	retried, err := InitializeProjectConfigurationDocumentWithOptions(doc, options)
	require.NoError(t, err)
	require.Equal(t, v.ID, retried.ID)
	var user models.User
	require.NoError(t, db.First(&user, originalUser.ID).Error)
	require.Equal(t, originalUser.Password, user.Password)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("Original-admin-password-2026!")))
	options.Administrator = ProjectDeploymentAdministrator{}
	_, err = InitializeProjectConfigurationDocumentWithOptions(doc, options)
	require.NoError(t, err, "existing administrator allows a retry without credentials")
	options.Administrator = ProjectDeploymentAdministrator{Username: "replacement.admin", Password: "Other-admin-password-2026!"}
	_, err = InitializeProjectConfigurationDocumentWithOptions(doc, options)
	require.ErrorContains(t, err, "已有其他企业管理员")
	options.Administrator = ProjectDeploymentAdministrator{UserID: originalUser.ID}
	_, err = InitializeProjectConfigurationDocumentWithOptions(doc, options)
	require.NoError(t, err)
	options.ExistingVersionID = v.ID + 99
	_, err = InitializeProjectConfigurationDocumentWithOptions(doc, options)
	require.ErrorContains(t, err, "不是此数据库的当前生效版本")
	require.NoError(t, db.Model(&models.TenantMember{}).Where("tenant_id = ?", 27).Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.NoError(t, db.Model(&models.AuthAuditLog{}).Where("action = ?", "tenant.administrator_initialized").Count(&count).Error)
	require.Equal(t, int64(1), count)
	var company models.Tenant
	require.NoError(t, db.First(&company, 27).Error)
	require.Equal(t, "Original company", company.Name)
}

func TestDeploymentAdministratorExplicitExistingAccountAndTenantBoundary(t *testing.T) {
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "integration")
	t.Setenv("RHD_INSTANCE_ID", "onward-shared-integration")
	t.Setenv("RHD_DEPLOYMENT_PROJECT_ID", "daypop-shared")
	t.Setenv("RHD_PROJECT_CONFIG_TENANT_ID", "27")
	db := setupSLATenantTestDB(t)
	require.NoError(t, db.AutoMigrate(models.Models...))
	require.NoError(t, db.AutoMigrate(&deploymentidentity.Record{}))
	standalone, _, err := UserService.CreateUserDB(db, request.CreateUserRequest{Username: "standalone", Password: "Standalone-fixture-2026!"}, nil)
	require.NoError(t, err)
	doc := projectconfig.Document{SchemaVersion: 1, TenantID: 27, Environment: "integration", Projects: []projectconfig.Project{}, Intake: projectconfig.IntakePolicy{Rules: []projectconfig.Rule{}}, SecretRefs: []string{}}
	options := ProjectDeploymentInitialization{TenantName: "Shared fixture", Administrator: ProjectDeploymentAdministrator{Username: standalone.Username, Password: "Must-not-reset-fixture-2026!"}}
	_, err = InitializeProjectConfigurationDocumentWithOptions(doc, options)
	require.ErrorContains(t, err, "账号名称已存在")
	options.Administrator = ProjectDeploymentAdministrator{UserID: standalone.ID}
	// A disabled foreign-company membership is still not transferable by install.
	foreign := models.TenantMember{TenantID: 99, UserID: standalone.ID, Status: enums.StatusDisabled}
	require.NoError(t, db.Create(&foreign).Error)
	_, err = InitializeProjectConfigurationDocumentWithOptions(doc, options)
	require.ErrorContains(t, err, "不会改变其归属")
	var count int64
	require.NoError(t, db.Model(&deploymentidentity.Record{}).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Delete(&foreign).Error)
	_, err = InitializeProjectConfigurationDocumentWithOptions(doc, options)
	require.NoError(t, err)
	var user models.User
	require.NoError(t, db.First(&user, standalone.ID).Error)
	require.Equal(t, standalone.Password, user.Password)
	var member models.TenantMember
	require.NoError(t, db.Where("tenant_id = ?", 27).First(&member).Error)
	require.Equal(t, standalone.ID, member.UserID)
}

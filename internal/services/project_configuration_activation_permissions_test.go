package services

import (
	"testing"

	"github.com/stretchr/testify/require"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
)

func TestProjectRuntimeActivationRequiresAdministratorForExistingDraft(t *testing.T) {
	db, admin, document := setupProjectRuntime(t)
	document.Runtime.Locale = "en-US"
	document.Runtime.Locales = []string{"zh-CN", "en-US"}
	draft, err := SaveProjectConfigurationDraft(1, ProjectConfigDraft{
		Document: document, RequestKey: "administrator-runtime-draft", Note: "Synthetic administrator draft",
	}, admin)
	require.NoError(t, err)
	support := &dto.AuthPrincipal{TenantID: 1, UserID: 8, Username: "support-fixture",
		Permissions: []string{constants.PermissionTicketUpdate.Code}}

	_, err = ApplyProjectConfiguration(1, draft.ID, support)
	require.ErrorContains(t, err, "请由公司管理员操作")
	view, err := GetProjectConfiguration(1)
	require.NoError(t, err)
	require.Zero(t, view.ActiveVersionID, "rejected activation changed the active version")
	var company models.Tenant
	require.NoError(t, db.First(&company, 1).Error)
	require.Equal(t, "zh-CN", company.DefaultLocale, "rejected activation changed live company settings")
	var activationCount, outboxCount int64
	require.NoError(t, db.Model(&models.ProjectConfigurationActivation{}).Count(&activationCount).Error)
	require.NoError(t, db.Model(&models.OutboxRecord{}).Count(&outboxCount).Error)
	require.Zero(t, activationCount)
	require.Zero(t, outboxCount)

	applied, err := ApplyProjectConfiguration(1, draft.ID, admin)
	require.NoError(t, err, "administrator must retain the normal activation path")
	require.Equal(t, draft.ID, applied.ID)
	require.NoError(t, db.First(&company, 1).Error)
	require.Equal(t, "en-US", company.DefaultLocale)
	// The idempotent branch must not bypass authorization for an already active version.
	_, err = ApplyProjectConfiguration(1, draft.ID, support)
	require.ErrorContains(t, err, "请由公司管理员操作")
}

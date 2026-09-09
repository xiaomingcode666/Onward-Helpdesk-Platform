package services

import (
	"slices"
	"testing"

	"remotehelpdesk/internal/pkg/constants"
)

func TestKnowledgeManagerUsesLeastPrivilegeProductPolicy(t *testing.T) {
	var permissions []string
	for _, spec := range tenantDefaultRoleSpecs() {
		if spec.Code == EnterpriseRoleKnowledge {
			permissions = spec.Permissions
			break
		}
	}
	if len(permissions) == 0 {
		t.Fatal("knowledge manager role was not defined")
	}

	for _, required := range []string{"product.view", "productModel.view", "knowledgeBase.create", "knowledgeDocument.update", "productKnowledgeBinding.resolve", "aiAgent.update", "aiConfig.view"} {
		if !slices.Contains(permissions, required) {
			t.Errorf("knowledge manager missing %q", required)
		}
	}
	for _, forbidden := range []string{"product.create", "product.update", "product.delete", "productModel.create", "aiConfig.update", "user.view", "ticket.view"} {
		if slices.Contains(permissions, forbidden) {
			t.Errorf("knowledge manager unexpectedly includes %q", forbidden)
		}
	}
}

func TestTenantAdminCanConfigureAgentMCPTools(t *testing.T) {
	var permissions []string
	for _, spec := range tenantDefaultRoleSpecs() {
		if spec.Code == EnterpriseRoleAdmin {
			permissions = spec.Permissions
			break
		}
	}
	if len(permissions) == 0 {
		t.Fatal("tenant admin role was not defined")
	}

	for _, required := range []string{"aiAgent.view", "aiAgent.update", "skillDefinition.view", "mcp.view", "mcp.call"} {
		if !slices.Contains(permissions, required) {
			t.Errorf("tenant admin missing %q", required)
		}
	}
}

func TestNotificationChannelManagementUsesLeastPrivilege(t *testing.T) {
	permissionsByRole := make(map[string][]string)
	for _, spec := range tenantDefaultRoleSpecs() {
		permissionsByRole[spec.Code] = spec.Permissions
	}

	channelPermission := constants.PermissionNotificationChannelManage.Code
	for _, roleCode := range []string{EnterpriseRoleAdmin, EnterpriseRoleServiceManager} {
		if !slices.Contains(permissionsByRole[roleCode], channelPermission) {
			t.Errorf("role %q missing %q", roleCode, channelPermission)
		}
	}
	if slices.Contains(permissionsByRole[EnterpriseRoleEngineer], channelPermission) {
		t.Errorf("engineer unexpectedly includes %q", channelPermission)
	}
	if !slices.Contains(permissionsByRole[EnterpriseRoleEngineer], constants.PermissionNotificationUpdate.Code) {
		t.Errorf("engineer must retain %q for personal read state", constants.PermissionNotificationUpdate.Code)
	}
}

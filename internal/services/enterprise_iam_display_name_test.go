package services

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

func TestEnterpriseIAMUpdateMemberSynchronizesAgentDisplayName(t *testing.T) {
	db, operator := setupPlatformIAMTestDB(t)
	if err := db.AutoMigrate(&models.EngineerProfile{}, &models.AgentProfile{}); err != nil {
		t.Fatalf("AutoMigrate engineer identities: %v", err)
	}
	tenant, err := PlatformIAMService.CreateTenant(request.PlatformTenantCreateRequest{Name: "Engineer Rename Tenant"}, operator)
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	operator.TenantID = tenant.ID
	operator.DomainType = models.DomainTypeEnterprise

	member, err := EnterpriseIAMService.InviteMember(tenant.ID, request.EnterpriseMemberInviteRequest{
		Username:        "rename.engineer",
		DisplayName:     "Old Engineer Name",
		Password:        "InitialPass123!",
		RoleCodes:       []string{EnterpriseRoleEngineer},
		DispatchEnabled: true,
	}, operator)
	if err != nil {
		t.Fatalf("InviteMember() error = %v", err)
	}

	newName := "Current Engineer Name"
	updated, err := EnterpriseIAMService.UpdateMember(tenant.ID, member.Member.ID, request.EnterpriseMemberUpdateRequest{
		DisplayName: &newName,
	}, operator)
	if err != nil {
		t.Fatalf("UpdateMember() error = %v", err)
	}
	if updated.Member.DisplayName != newName || updated.User.Nickname != newName {
		t.Fatalf("enterprise identity was not renamed: member=%q user=%q", updated.Member.DisplayName, updated.User.Nickname)
	}

	profile := repositories.AgentProfileRepository.FindOne(db, sqls.NewCnd().Eq("tenant_id", tenant.ID).Eq("user_id", member.User.ID))
	if profile == nil {
		t.Fatal("agent profile was not created")
	}
	if profile.DisplayName != newName {
		t.Fatalf("agent display name = %q, want %q", profile.DisplayName, newName)
	}
}

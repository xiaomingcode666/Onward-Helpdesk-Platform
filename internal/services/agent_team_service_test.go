package services

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"

	"gorm.io/gorm"
)

func TestUpdateSystemManagedTechnicalRepairTeamSetsLeader(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	team := models.AgentTeam{
		ID:             40,
		TenantID:       1,
		Name:           "技术维护组",
		TeamType:       AgentTeamTypeTechnicalRepair,
		SystemManaged:  true,
		AssignmentMode: AgentTeamAssignmentModeBalanced,
		Status:         enums.StatusOk,
	}
	if err := db.Create(&team).Error; err != nil {
		t.Fatalf("create technical repair team: %v", err)
	}
	createHumanDispatchRealtimeAgentProfile(t, db, 401, team.ID)
	grantEnterpriseServiceManagerRole(t, db, team.TenantID, 401)

	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager"}
	if err := AgentTeamService.UpdateAgentTeam(request.UpdateAgentTeamRequest{
		ID:             team.ID,
		Name:           team.Name,
		LeaderUserID:   401,
		AssignmentMode: AgentTeamAssignmentModeWeighted,
		Status:         int(enums.StatusOk),
		Description:    team.Description,
		Remark:         team.Remark,
	}, operator); err != nil {
		t.Fatalf("UpdateAgentTeam() error = %v", err)
	}

	var updated models.AgentTeam
	if err := db.First(&updated, team.ID).Error; err != nil {
		t.Fatalf("reload technical repair team: %v", err)
	}
	if updated.LeaderUserID != 401 {
		t.Fatalf("leader user id = %d, want 401", updated.LeaderUserID)
	}
	if updated.AssignmentMode != AgentTeamAssignmentModeWeighted {
		t.Fatalf("assignment mode = %q, want %q", updated.AssignmentMode, AgentTeamAssignmentModeWeighted)
	}

	var membership models.AgentTeamMember
	if err := db.Where("tenant_id = ? AND team_id = ? AND user_id = ?", 1, team.ID, 401).
		First(&membership).Error; err != nil {
		t.Fatalf("load leader membership: %v", err)
	}
	if !membership.DispatchEnabled {
		t.Fatalf("existing leader dispatch setting was overwritten: %+v", membership)
	}

	ticket := &models.Ticket{TenantID: 1, CurrentTeamID: team.ID}
	if supervisorID := ResolveSupervisorOwner(ticket); supervisorID != 401 {
		t.Fatalf("supervisor owner = %d, want 401", supervisorID)
	}
}

func TestUpdateSystemManagedTechnicalRepairTeamRejectsNonServiceManager(t *testing.T) {
	db := setupHumanDispatchRealtimeTestDB(t)
	team := models.AgentTeam{
		ID:             41,
		TenantID:       1,
		Name:           "技术维护组",
		TeamType:       AgentTeamTypeTechnicalRepair,
		SystemManaged:  true,
		AssignmentMode: AgentTeamAssignmentModeBalanced,
		Status:         enums.StatusOk,
	}
	if err := db.Create(&team).Error; err != nil {
		t.Fatalf("create technical repair team: %v", err)
	}
	createHumanDispatchRealtimeAgentProfile(t, db, 402, team.ID)

	operator := &dto.AuthPrincipal{TenantID: 1, UserID: 900, Username: "manager"}
	err := AgentTeamService.UpdateAgentTeam(request.UpdateAgentTeamRequest{
		ID:             team.ID,
		Name:           team.Name,
		LeaderUserID:   402,
		AssignmentMode: AgentTeamAssignmentModeBalanced,
		Status:         int(enums.StatusOk),
		Description:    team.Description,
		Remark:         team.Remark,
	}, operator)
	if err == nil {
		t.Fatal("expected non-service-manager supervisor to be rejected")
	}

	var updated models.AgentTeam
	if err := db.First(&updated, team.ID).Error; err != nil {
		t.Fatalf("reload technical repair team: %v", err)
	}
	if updated.LeaderUserID != 0 {
		t.Fatalf("leader user id = %d, want 0", updated.LeaderUserID)
	}
}

func grantEnterpriseServiceManagerRole(t *testing.T, db *gorm.DB, tenantID, userID int64) {
	t.Helper()
	if err := db.AutoMigrate(&models.AuthRole{}, &models.AuthRoleBinding{}); err != nil {
		t.Fatalf("migrate auth role tables: %v", err)
	}
	var member models.TenantMember
	if err := db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&member).Error; err != nil {
		t.Fatalf("load tenant member: %v", err)
	}
	role := models.AuthRole{
		TenantID:    tenantID,
		DomainType:  models.DomainTypeEnterprise,
		Code:        EnterpriseRoleServiceManager,
		Name:        "服务负责人",
		IsBuiltin:   true,
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create service manager role: %v", err)
	}
	binding := models.AuthRoleBinding{
		TenantID:    tenantID,
		DomainType:  models.DomainTypeEnterprise,
		RoleID:      role.ID,
		SubjectType: models.SubjectTypeTenantMember,
		SubjectID:   member.ID,
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	if err := db.Create(&binding).Error; err != nil {
		t.Fatalf("create service manager role binding: %v", err)
	}
}

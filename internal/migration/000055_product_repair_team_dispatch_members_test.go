package migration

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestProductRepairDispatchMemberMigrationBackfillsAgentProfiles(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.AgentTeam{}, &models.AgentProfile{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	user := models.User{Username: "migration-engineer", Status: enums.StatusOk}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	team := models.AgentTeam{TenantID: 7, ProductID: 11, TeamType: services.AgentTeamTypeProductRepair, Name: "产品维修组", Status: enums.StatusOk}
	if err := db.Create(&team).Error; err != nil {
		t.Fatalf("create team: %v", err)
	}
	profile := models.AgentProfile{
		TenantID: 7, TeamID: team.ID, UserID: user.ID, AgentCode: "MIG-ENG",
		DisplayName: "迁移工程师", AutoAssignEnabled: true, Status: enums.StatusOk,
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}

	if err := ensureProductRepairDispatchMemberSchema(db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if err := backfillAgentTeamMembersFromProfiles(db); err != nil {
		t.Fatalf("backfill members: %v", err)
	}

	var member models.AgentTeamMember
	if err := db.First(&member, "tenant_id = ? AND team_id = ? AND user_id = ?", int64(7), team.ID, user.ID).Error; err != nil {
		t.Fatalf("member was not backfilled: %v", err)
	}
	if !member.DispatchEnabled || member.DispatchWeight != 1 || member.Status != enums.StatusOk {
		t.Fatalf("unexpected member after backfill: %+v", member)
	}
	var persistedTeam models.AgentTeam
	if err := db.First(&persistedTeam, team.ID).Error; err != nil {
		t.Fatalf("reload team: %v", err)
	}
	if persistedTeam.AssignmentMode != services.AgentTeamAssignmentModeBalanced {
		t.Fatalf("assignment mode = %q, want balanced", persistedTeam.AssignmentMode)
	}
}

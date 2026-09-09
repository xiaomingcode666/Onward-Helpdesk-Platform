package migration

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestStabilizeE2EDispatchFixture(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.AgentProfile{}, &models.AgentTeamMember{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	admin := models.User{Username: "e2e.tenant1.admin", Status: enums.StatusOk}
	engineer := models.User{Username: "e2e.product1.engineer", Status: enums.StatusOk}
	other := models.User{Username: "e2e.product1.engineer2", Status: enums.StatusOk}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := db.Create(&engineer).Error; err != nil {
		t.Fatalf("create engineer: %v", err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	members := []models.AgentTeamMember{
		{TenantID: 1, TeamID: 5, UserID: admin.ID, DispatchEnabled: true, DispatchWeight: 9, Status: enums.StatusOk},
		{TenantID: 1, TeamID: 5, UserID: engineer.ID, DispatchEnabled: false, DispatchWeight: 1, Status: enums.StatusOk},
		{TenantID: 1, TeamID: 5, UserID: other.ID, DispatchEnabled: true, DispatchWeight: 3, Status: enums.StatusOk},
	}
	if err := db.Create(&members).Error; err != nil {
		t.Fatalf("create members: %v", err)
	}
	profiles := []models.AgentProfile{
		{TenantID: 1, UserID: admin.ID, AgentCode: "admin", AutoAssignEnabled: true, Status: enums.StatusOk},
		{TenantID: 1, UserID: engineer.ID, AgentCode: "engineer", AutoAssignEnabled: false, Status: enums.StatusOk},
	}
	if err := db.Create(&profiles).Error; err != nil {
		t.Fatalf("create profiles: %v", err)
	}

	if err := stabilizeE2EDispatchFixture(db); err != nil {
		t.Fatalf("stabilize fixture: %v", err)
	}

	var adminMember, engineerMember, otherMember models.AgentTeamMember
	if err := db.First(&adminMember, "user_id = ?", admin.ID).Error; err != nil {
		t.Fatalf("load admin member: %v", err)
	}
	if err := db.First(&engineerMember, "user_id = ?", engineer.ID).Error; err != nil {
		t.Fatalf("load engineer member: %v", err)
	}
	if err := db.First(&otherMember, "user_id = ?", other.ID).Error; err != nil {
		t.Fatalf("load other member: %v", err)
	}
	if adminMember.DispatchEnabled || adminMember.DispatchWeight != 1 {
		t.Fatalf("admin dispatch fixture = %+v, want disabled weight 1", adminMember)
	}
	if !engineerMember.DispatchEnabled || engineerMember.DispatchWeight != 10 {
		t.Fatalf("engineer dispatch fixture = %+v, want enabled weight 10", engineerMember)
	}
	if !otherMember.DispatchEnabled || otherMember.DispatchWeight != 3 {
		t.Fatalf("unrelated member changed: %+v", otherMember)
	}
	var adminProfile, engineerProfile models.AgentProfile
	if err := db.First(&adminProfile, "user_id = ?", admin.ID).Error; err != nil {
		t.Fatalf("load admin profile: %v", err)
	}
	if err := db.First(&engineerProfile, "user_id = ?", engineer.ID).Error; err != nil {
		t.Fatalf("load engineer profile: %v", err)
	}
	if adminProfile.AutoAssignEnabled || !engineerProfile.AutoAssignEnabled {
		t.Fatalf("profile auto-assign not stabilized: admin=%v engineer=%v", adminProfile.AutoAssignEnabled, engineerProfile.AutoAssignEnabled)
	}
}

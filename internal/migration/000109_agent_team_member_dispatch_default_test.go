package migration

import (
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAgentTeamMemberDispatchDefaultsToDisabledForDirectCreates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&models.AgentTeamMember{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID: 1,
		TeamID:   2,
		UserID:   3,
		Status:   enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	var member models.AgentTeamMember
	if err := db.First(&member, "tenant_id = ? AND team_id = ? AND user_id = ?", int64(1), int64(2), int64(3)).Error; err != nil {
		t.Fatalf("reload member: %v", err)
	}
	if member.DispatchEnabled {
		t.Fatalf("direct-created member dispatch_enabled = true, want opt-in false")
	}
}

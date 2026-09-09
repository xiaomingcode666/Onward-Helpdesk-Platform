package migration

import (
	"path/filepath"
	"strings"
	"testing"

	"remotehelpdesk/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestSeedAfterSalesSkillTemplatesIsIdempotentAndLeastPrivilege(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "skill-templates.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.SkillDefinition{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	if err := seedAfterSalesSkillTemplates(db); err != nil {
		t.Fatalf("first seed error = %v", err)
	}
	if err := seedAfterSalesSkillTemplates(db); err != nil {
		t.Fatalf("second seed error = %v", err)
	}

	var items []models.SkillDefinition
	if err := db.Order("id ASC").Find(&items).Error; err != nil {
		t.Fatalf("load templates: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("template count = %d, want 4", len(items))
	}
	for _, item := range items {
		if item.TenantID != 0 {
			t.Errorf("template %q tenant = %d, want platform scope", item.Name, item.TenantID)
		}
	}

	var escalation models.SkillDefinition
	if err := db.Where("remark = ?", builtinSkillTemplateRemarkPrefix+"safe-stop-escalation").First(&escalation).Error; err != nil {
		t.Fatalf("load escalation template: %v", err)
	}
	if escalation.ToolWhitelist != `[]` || !strings.Contains(escalation.Instruction, "会话主流程") {
		t.Fatalf("escalation whitelist = %s", escalation.ToolWhitelist)
	}

	var ticket models.SkillDefinition
	if err := db.Where("remark = ?", builtinSkillTemplateRemarkPrefix+"service-ticket").First(&ticket).Error; err != nil {
		t.Fatalf("load ticket template: %v", err)
	}
	if ticket.ToolWhitelist != `[]` || !strings.Contains(ticket.Instruction, "会话主流程") {
		t.Fatalf("ticket whitelist = %s", ticket.ToolWhitelist)
	}
}

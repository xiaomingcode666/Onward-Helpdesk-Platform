package services_test

import (
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestSkillDefinitionTenantScopeAndGlobalTemplateProtection(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SkillDefinition{}, &models.AIAgent{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)

	global := &models.SkillDefinition{Name: "平台安全模板", Instruction: "只读模板", Status: enums.StatusOk}
	otherTenant := &models.SkillDefinition{TenantID: 22, Name: "其他企业技能", Instruction: "不可见", Status: enums.StatusOk}
	if err := db.Create(global).Error; err != nil {
		t.Fatalf("create global skill: %v", err)
	}
	if err := db.Create(otherTenant).Error; err != nil {
		t.Fatalf("create other tenant skill: %v", err)
	}

	operator := &dto.AuthPrincipal{TenantID: 11, UserID: 101, Username: "tenant-11-admin"}
	created, err := services.SkillDefinitionService.CreateSkillDefinition(request.CreateSkillDefinitionRequest{
		Name:        "本企业故障诊断",
		Instruction: "根据产品知识进行故障诊断。",
	}, operator)
	if err != nil {
		t.Fatalf("create tenant skill: %v", err)
	}
	if created.TenantID != operator.TenantID {
		t.Fatalf("tenant id = %d, want %d", created.TenantID, operator.TenantID)
	}

	visible := services.SkillDefinitionService.Find(
		services.SkillDefinitionService.ScopeCnd(sqls.NewCnd().Asc("id"), operator),
	)
	if len(visible) != 2 || visible[0].ID != global.ID || visible[1].ID != created.ID {
		t.Fatalf("unexpected visible skills: %#v", visible)
	}
	if services.SkillDefinitionService.GetForOperator(otherTenant.ID, operator) != nil {
		t.Fatalf("cross-tenant skill must not be visible")
	}
	if err := services.SkillDefinitionService.UpdateSkillDefinition(request.UpdateSkillDefinitionRequest{
		ID: global.ID,
		CreateSkillDefinitionRequest: request.CreateSkillDefinitionRequest{
			Name:        "尝试修改平台模板",
			Instruction: "should fail",
		},
	}, operator); err == nil {
		t.Fatalf("tenant must not update a global template")
	}
}

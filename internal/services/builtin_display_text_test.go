package services

import (
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/i18nx"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

// 存储列保留改动前的中文规范值；这里锁定逐字相同，避免存量数据被误判为用户改名。
func TestBuiltinStoredValuesStayUnchanged(t *testing.T) {
	knowledgeSupport := &models.Tenant{}
	knowledgeSupport.ServiceScene = models.TenantServiceSceneKnowledgeSupport
	product := &models.Product{Name: "Pump", Code: "P-1"}

	cases := map[string][2]string{
		"default knowledge base name":        {tenantDefaultKnowledgeBaseName(), "企业通用知识库"},
		"default knowledge base description": {tenantDefaultKnowledgeBaseDescription(), "保存企业级服务说明、FAQ 和安全指引，供知识问答统一检索。"},
		"default agent name":                 {tenantDefaultAgentName(), "企业默认服务机器人"},
		"default agent knowledge support":    {tenantDefaultAgentDescription(knowledgeSupport), "基于企业通用知识库提供智能咨询，并支持转接工程师。"},
		"default agent general service":      {tenantDefaultAgentDescription(&models.Tenant{}), "未选择设备时提供通用咨询和企业知识问答；识别设备后由对应产品机器人接管。"},
		"product knowledge base name":        {productKnowledgeBaseNameText("Pump").Render(i18nx.LocaleZhCN), "Pump 知识库"},
		"product knowledge base description": {productKnowledgeBaseDescriptionText("Pump").Render(i18nx.LocaleZhCN), "保存 Pump 的手册、FAQ、维修案例和诊断知识，服务团队和产品机器人都从这里查。"},
		"product agent name":                 {productAgentNameText(product, false).Render(i18nx.LocaleZhCN), "Pump 客服机器人"},
		"product agent name with code":       {productAgentNameText(product, true).Render(i18nx.LocaleZhCN), "Pump 客服机器人 (P-1)"},
		"product agent description":          {productAgentDescriptionText("Pump").Render(i18nx.LocaleZhCN), "Pump 的售后客服机器人，创建后自动部署；后续配置变更需提交审核。"},
	}
	for name, values := range cases {
		if values[0] != values[1] {
			t.Errorf("%s = %q, want %q", name, values[0], values[1])
		}
	}
}

func TestKeepOrReplaceDisplayText(t *testing.T) {
	text := productAgentNameText(&models.Product{Name: "Pump"}, false)
	stored := text.Render(i18nx.LocaleZhCN)

	for _, submitted := range []string{"Pump Service Bot", stored, " Pump Service Bot "} {
		name, kept := keepOrReplaceDisplayText(text, stored, submitted)
		if name != stored || !kept.Equal(text) {
			t.Fatalf("submitted %q: got (%q, %+v), want stored value and text kept", submitted, name, kept)
		}
	}

	name, cleared := keepOrReplaceDisplayText(text, stored, "Pump Concierge")
	if name != "Pump Concierge" || !cleared.IsZero() {
		t.Fatalf("customized name: got (%q, %+v)", name, cleared)
	}
	name, cleared = keepOrReplaceDisplayText(i18nx.Text{}, "Custom", "Renamed")
	if name != "Renamed" || !cleared.IsZero() {
		t.Fatalf("user-defined name: got (%q, %+v)", name, cleared)
	}
}

func setupBuiltinDisplayTextTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Tenant{}, &models.Product{}, &models.ProductServiceProfile{}, &models.KnowledgeBase{}, &models.AIAgent{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestBackfillBuiltinDisplayTextOnlyTouchesSystemGeneratedValues(t *testing.T) {
	db := setupBuiltinDisplayTextTestDB(t)
	tenant := models.Tenant{Name: "Acme", Status: enums.StatusOk, ServiceScene: models.TenantServiceSceneKnowledgeSupport}
	mustCreateBuiltinRecord(t, db, &tenant)
	product := models.Product{TenantID: tenant.ID, Name: "Pump", Code: "P-1", Status: enums.StatusOk}
	mustCreateBuiltinRecord(t, db, &product)

	defaultKB := models.KnowledgeBase{TenantID: tenant.ID, Name: "企业通用知识库", Description: tenantDefaultKnowledgeBaseDescription(), Remark: tenantDefaultKnowledgeBaseRemark, Status: enums.StatusOk}
	productKB := models.KnowledgeBase{TenantID: tenant.ID, Name: "Pump 知识库", Description: "Customized description", Status: enums.StatusOk}
	mustCreateBuiltinRecord(t, db, &defaultKB)
	mustCreateBuiltinRecord(t, db, &productKB)
	mustCreateBuiltinRecord(t, db, &models.ProductServiceProfile{TenantID: tenant.ID, ProductID: product.ID, DefaultKnowledgeBaseID: productKB.ID, Status: enums.StatusOk})

	defaultAgent := models.AIAgent{TenantID: tenant.ID, Source: TenantDefaultAIAgentSource, Name: "企业默认服务机器人", Description: tenantDefaultAgentDescription(&tenant), Status: enums.StatusOk}
	productAgent := models.AIAgent{TenantID: tenant.ID, ProductID: product.ID, Name: "Pump 客服机器人 (P-1)", Description: "Pump 的售后客服机器人，创建后自动部署；后续配置变更需提交审核。", Status: enums.StatusOk}
	renamedAgent := models.AIAgent{TenantID: tenant.ID, ProductID: product.ID, Name: "Pump Concierge", Description: "Custom", Status: enums.StatusOk}
	mustCreateBuiltinRecord(t, db, &defaultAgent)
	mustCreateBuiltinRecord(t, db, &productAgent)
	mustCreateBuiltinRecord(t, db, &renamedAgent)

	if err := BackfillBuiltinDisplayTextDB(db); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	// 重复执行保持幂等。
	if err := BackfillBuiltinDisplayTextDB(db); err != nil {
		t.Fatalf("second backfill: %v", err)
	}

	assertKB := func(id int64, wantName, wantDescription string) {
		t.Helper()
		var got models.KnowledgeBase
		if err := db.First(&got, id).Error; err != nil {
			t.Fatalf("load knowledge base %d: %v", id, err)
		}
		if name := got.DisplayName(i18nx.LocaleEnUS); name != wantName {
			t.Fatalf("knowledge base %d name = %q, want %q", id, name, wantName)
		}
		if description := got.DisplayDescription(i18nx.LocaleEnUS); description != wantDescription {
			t.Fatalf("knowledge base %d description = %q, want %q", id, description, wantDescription)
		}
	}
	assertKB(defaultKB.ID, "Enterprise General Knowledge Base", i18nx.TLocale(i18nx.LocaleEnUS, "builtin.knowledgeBase.tenantDefault.description"))
	assertKB(productKB.ID, "Pump Knowledge Base", "Customized description")

	assertAgent := func(id int64, wantName, wantDescription string) {
		t.Helper()
		var got models.AIAgent
		if err := db.First(&got, id).Error; err != nil {
			t.Fatalf("load agent %d: %v", id, err)
		}
		if name := got.DisplayName(i18nx.LocaleEnUS); name != wantName {
			t.Fatalf("agent %d name = %q, want %q", id, name, wantName)
		}
		if description := got.DisplayDescription(i18nx.LocaleEnUS); description != wantDescription {
			t.Fatalf("agent %d description = %q, want %q", id, description, wantDescription)
		}
	}
	assertAgent(defaultAgent.ID, "Enterprise Default Service Bot", i18nx.TLocale(i18nx.LocaleEnUS, "builtin.aiAgent.tenantDefault.description.knowledgeSupport"))
	assertAgent(productAgent.ID, "Pump Service Bot (P-1)", productAgentDescriptionText("Pump").Render(i18nx.LocaleEnUS))
	assertAgent(renamedAgent.ID, "Pump Concierge", "Custom")
}

func TestSyncProductDerivedDisplayTextFollowsProductRename(t *testing.T) {
	db := setupBuiltinDisplayTextTestDB(t)
	product := models.Product{TenantID: 1, Name: "Pump", Code: "P-1", Status: enums.StatusOk}
	mustCreateBuiltinRecord(t, db, &product)
	kb := models.KnowledgeBase{TenantID: 1, Name: "Pump 知识库", NameText: productKnowledgeBaseNameText("Pump"), DescriptionText: productKnowledgeBaseDescriptionText("Pump"), Status: enums.StatusOk}
	mustCreateBuiltinRecord(t, db, &kb)
	mustCreateBuiltinRecord(t, db, &models.ProductServiceProfile{TenantID: 1, ProductID: product.ID, DefaultKnowledgeBaseID: kb.ID, Status: enums.StatusOk})
	agent := models.AIAgent{TenantID: 1, ProductID: product.ID, Name: "Pump 客服机器人", NameText: productAgentNameText(&product, false), DescriptionText: productAgentDescriptionText("Pump"), Status: enums.StatusOk}
	custom := models.AIAgent{TenantID: 1, ProductID: product.ID, Name: "Pump Concierge", Status: enums.StatusOk}
	mustCreateBuiltinRecord(t, db, &agent)
	mustCreateBuiltinRecord(t, db, &custom)

	product.Name = "Fan"
	if err := syncProductDerivedDisplayTextDB(db, &product); err != nil {
		t.Fatalf("sync: %v", err)
	}

	var gotAgent, gotCustom models.AIAgent
	var gotKB models.KnowledgeBase
	if err := db.First(&gotAgent, agent.ID).Error; err != nil {
		t.Fatalf("load agent: %v", err)
	}
	if err := db.First(&gotCustom, custom.ID).Error; err != nil {
		t.Fatalf("load custom agent: %v", err)
	}
	if err := db.First(&gotKB, kb.ID).Error; err != nil {
		t.Fatalf("load knowledge base: %v", err)
	}
	if got := gotAgent.DisplayName(i18nx.LocaleEnUS); got != "Fan Service Bot" {
		t.Fatalf("agent name = %q", got)
	}
	if got := gotCustom.DisplayName(i18nx.LocaleEnUS); got != "Pump Concierge" {
		t.Fatalf("custom agent name = %q", got)
	}
	if got := gotKB.DisplayName(i18nx.LocaleZhCN); got != "Fan 知识库" {
		t.Fatalf("knowledge base name = %q", got)
	}
}

func TestResolveKnowledgeBaseStoredNameAcceptsDisplayNames(t *testing.T) {
	db := setupBuiltinDisplayTextTestDB(t)
	mustCreateBuiltinRecord(t, db, &models.KnowledgeBase{TenantID: 1, Name: "企业通用知识库", NameText: tenantDefaultKnowledgeBaseNameText, Status: enums.StatusOk})
	mustCreateBuiltinRecord(t, db, &models.KnowledgeBase{TenantID: 2, Name: "Other", Status: enums.StatusOk})

	for _, input := range []string{"Enterprise General Knowledge Base", "企业通用知识库"} {
		if got := resolveKnowledgeBaseStoredName(db, 1, input); got != "企业通用知识库" {
			t.Fatalf("resolve(%q) = %q", input, got)
		}
	}
	if got := resolveKnowledgeBaseStoredName(db, 2, "Enterprise General Knowledge Base"); got != "Enterprise General Knowledge Base" {
		t.Fatalf("other tenant resolve = %q", got)
	}
	if got := resolveKnowledgeBaseStoredName(db, 1, "Spare parts"); got != "Spare parts" {
		t.Fatalf("user-defined resolve = %q", got)
	}
}

func mustCreateBuiltinRecord(t *testing.T, db *gorm.DB, value any) {
	t.Helper()
	if err := db.Create(value).Error; err != nil {
		t.Fatalf("create %T: %v", value, err)
	}
}

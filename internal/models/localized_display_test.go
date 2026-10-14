package models

import (
	"testing"

	"remotehelpdesk/internal/pkg/i18nx"
)

func TestKnowledgeBaseDisplayFollowsLocaleUntilCustomized(t *testing.T) {
	item := &KnowledgeBase{
		Name:            "企业通用知识库",
		Description:     "legacy description",
		NameText:        i18nx.NewText("builtin.knowledgeBase.tenantDefault.name", nil),
		DescriptionText: i18nx.NewText("builtin.knowledgeBase.tenantDefault.description", nil),
	}

	if got := item.DisplayName(i18nx.LocaleEnUS); got != "Enterprise General Knowledge Base" {
		t.Fatalf("DisplayName(en-US) = %q", got)
	}
	if got := item.DisplayName(i18nx.LocaleZhCN); got != "企业通用知识库" {
		t.Fatalf("DisplayName(zh-CN) = %q", got)
	}
	if got := item.DisplayDescription(i18nx.LocaleEnUS); got == "legacy description" || got == "" {
		t.Fatalf("DisplayDescription(en-US) = %q", got)
	}

	custom := &KnowledgeBase{Name: "Spare parts", Description: "Parts catalog"}
	if got := custom.DisplayName(i18nx.LocaleZhCN); got != "Spare parts" {
		t.Fatalf("custom DisplayName = %q", got)
	}
	if got := custom.DisplayDescription(i18nx.LocaleZhCN); got != "Parts catalog" {
		t.Fatalf("custom DisplayDescription = %q", got)
	}
}

func TestProductDerivedNamesUseProductParam(t *testing.T) {
	kb := &KnowledgeBase{NameText: i18nx.NewText("builtin.knowledgeBase.product.name", i18nx.Params{"product": "Pump X"})}
	if got := kb.DisplayName(i18nx.LocaleEnUS); got != "Pump X Knowledge Base" {
		t.Fatalf("product KB en-US = %q", got)
	}
	if got := kb.DisplayName(i18nx.LocaleZhCN); got != "Pump X 知识库" {
		t.Fatalf("product KB zh-CN = %q", got)
	}

	agent := &AIAgent{NameText: i18nx.NewText("builtin.aiAgent.product.name", i18nx.Params{"product": "Pump X"})}
	if got := agent.DisplayName(i18nx.LocaleEnUS); got != "Pump X Service Bot" {
		t.Fatalf("product agent en-US = %q", got)
	}
	if got := agent.DisplayName(i18nx.LocaleEsES); got == "" || got == "builtin.aiAgent.product.name" {
		t.Fatalf("product agent es-ES = %q", got)
	}
}

func TestAIAgentDisplayFallsBackToStoredValues(t *testing.T) {
	agent := &AIAgent{Name: "Sales bot", Description: "Handles sales"}
	if got := agent.DisplayName(i18nx.LocaleEnUS); got != "Sales bot" {
		t.Fatalf("DisplayName = %q", got)
	}
	if got := agent.DisplayDescription(i18nx.LocaleEnUS); got != "Handles sales" {
		t.Fatalf("DisplayDescription = %q", got)
	}
}

func TestAgentTeamAndDepartmentDisplay(t *testing.T) {
	team := &AgentTeam{Name: "技术维护组", NameText: i18nx.NewText("builtin.agentTeam.technicalMaintenance.name", nil), Description: "custom"}
	if got := team.DisplayName(i18nx.LocaleEnUS); got != "Technical Maintenance Team" {
		t.Fatalf("team DisplayName = %q", got)
	}
	if got := team.DisplayDescription(i18nx.LocaleEnUS); got != "custom" {
		t.Fatalf("team DisplayDescription = %q", got)
	}
	department := &Department{Name: "企业组织", NameText: i18nx.NewText("builtin.department.root.name", nil)}
	if got := department.DisplayName(i18nx.LocaleEnUS); got != "Enterprise Organization" {
		t.Fatalf("department DisplayName = %q", got)
	}
	if got := (&Department{Name: "Sales"}).DisplayName(i18nx.LocaleZhCN); got != "Sales" {
		t.Fatalf("custom department DisplayName = %q", got)
	}
}

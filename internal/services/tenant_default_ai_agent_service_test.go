package services

import (
	"strings"
	"testing"

	workflowcapability "remotehelpdesk/internal/ai/workflow/capability"
	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
)

func TestDefaultTenantAgentPromptDefinesGeneralConsultationBoundaries(t *testing.T) {
	prompt := defaultTenantAgentSystemPrompt(&models.Tenant{Name: "Test Tenant"})
	for _, required := range []string{
		"使用用户最新消息的主要语言回答",
		"保修范围、服务时间或服务区域",
		"停止运行、断电隔离、避免重复上电",
		"不提供转人工或创建工单",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("default tenant prompt missing %q: %s", required, prompt)
		}
	}
	if strings.Contains(workflowcapability.SafeKnowledgeFallbackMessage, "故障码") {
		t.Fatalf("general fallback still assumes a device fault: %q", workflowcapability.SafeKnowledgeFallbackMessage)
	}
}

func TestKnowledgeSupportTenantAgentPromptDoesNotRequireEquipmentContext(t *testing.T) {
	prompt := defaultTenantAgentSystemPrompt(&models.Tenant{
		Name:         "Knowledge Tenant",
		ServiceScene: models.TenantServiceSceneKnowledgeSupport,
	})
	for _, required := range []string{"企业知识服务助手", "企业已发布的通用知识库", "不得要求用户绑定产品或设备", "允许进入现有转人工流程"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("knowledge tenant prompt missing %q: %s", required, prompt)
		}
	}
	for _, forbidden := range []string{"请提供设备型号", "请提供序列号", "请提供故障码", "断电隔离"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("knowledge tenant prompt contains equipment instruction %q: %s", forbidden, prompt)
		}
	}
}

func TestWorkflowDefinitionCarriesKnowledgeCitations(t *testing.T) {
	definition := dsl.Definition{Nodes: []dsl.Node{
		{ID: "retrieve", Type: "knowledge_retrieve"},
		{ID: "reply", Type: "send_reply", Inputs: map[string]dsl.VariableSelector{
			"citations": {NodeID: "retrieve", Field: "citations"},
		}},
	}}
	if !workflowDefinitionCarriesKnowledgeCitations(definition) {
		t.Fatal("citation-enabled knowledge definition was not detected")
	}
	definition.Nodes[1].Inputs["citations"] = dsl.VariableSelector{NodeID: "other", Field: "citations"}
	if workflowDefinitionCarriesKnowledgeCitations(definition) {
		t.Fatal("unrelated citation selector was accepted")
	}
}

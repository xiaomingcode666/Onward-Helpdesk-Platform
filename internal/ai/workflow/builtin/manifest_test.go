package builtin_test

import (
	"encoding/json"
	"strings"
	"testing"

	"remotehelpdesk/internal/ai/workflow/builtin"
	"remotehelpdesk/internal/ai/workflow/dsl"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
	workflowvalidator "remotehelpdesk/internal/ai/workflow/validator"
)

func TestEmbeddedPlatformWorkflowManifestsAreUniqueAndValid(t *testing.T) {
	items, err := builtin.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("manifest count = %d, want 4", len(items))
	}
	for _, item := range items {
		result := workflowvalidator.ValidateDefinition(item.Definition, workflowregistry.DefaultRegistry())
		if !result.Valid {
			t.Fatalf("manifest %s is invalid: %#v", item.Code, result.Errors)
		}
	}
}

func TestKnowledgeSupportManifestUsesTenantKnowledgeAndHumanHandoff(t *testing.T) {
	items, err := builtin.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for _, item := range items {
		if item.Code != "knowledge_customer_service_default" {
			continue
		}
		if item.Definition.ModelPolicy == nil || len(item.Definition.ModelPolicy.CredentialChain) != 1 || item.Definition.ModelPolicy.CredentialChain[0] != dsl.ModelCredentialScopeTenantDefault {
			t.Fatalf("knowledge workflow credential policy = %+v", item.Definition.ModelPolicy)
		}
		if item.Definition.HasNodeType(workflowregistry.NodeTypeEntryContext) || item.Definition.HasNodeType(workflowregistry.NodeTypeCreateTicket) {
			t.Fatalf("knowledge workflow contains equipment or direct ticket nodes: %+v", item.Definition.Nodes)
		}
		if !item.Definition.HasNodeType(workflowregistry.NodeTypeHandoffToHuman) {
			t.Fatal("knowledge workflow must allow customer-initiated human handoff")
		}
		analysis := manifestNodeByID(item.Definition, "analysis_1")
		var analysisConfig map[string]any
		if analysis == nil || json.Unmarshal(analysis.Config, &analysisConfig) != nil || len(analysisConfig) != 0 {
			t.Fatalf("knowledge workflow analysis must detect the customer message without forcing handoff: %+v", analysis)
		}
		retrieve := manifestNodeByID(item.Definition, "tenant_retrieve_1")
		var retrieveConfig dsl.KnowledgeRetrieveConfig
		if retrieve == nil || json.Unmarshal(retrieve.Config, &retrieveConfig) != nil || retrieveConfig.EffectiveBindingMethod() != "agent_default" {
			t.Fatalf("knowledge workflow does not use tenant agent knowledge: %+v", retrieve)
		}
		reply := manifestNodeByID(item.Definition, "send_reply_1")
		if reply == nil {
			t.Fatal("knowledge workflow reply node not found")
		}
		citations, ok := reply.Inputs["citations"]
		if !ok || citations.NodeID != "tenant_retrieve_1" || citations.Field != "citations" {
			t.Fatalf("knowledge workflow does not forward retrieval citations to the reply: %+v", reply)
		}
		serialized, err := json.Marshal(item.Definition)
		if err != nil {
			t.Fatalf("marshal knowledge workflow: %v", err)
		}
		content := string(serialized)
		for _, forbidden := range []string{"productId", "deviceId", "serviceCodeId", "product_context", "public_product"} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("knowledge workflow contains equipment context %q", forbidden)
			}
		}
		return
	}
	t.Fatal("knowledge support platform manifest not found")
}

func TestDefaultWorkflowRoutesGeneralConsultationThroughIntentAndTenantKnowledge(t *testing.T) {
	items, err := builtin.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for _, item := range items {
		if item.Code != "aftersales_customer_service_default" {
			continue
		}
		access := manifestNodeByID(item.Definition, "service_access_1")
		var accessConfig dsl.ServiceAccessPolicyConfig
		if access == nil || json.Unmarshal(access.Config, &accessConfig) != nil || accessConfig.HumanHandoffMode != "always" || accessConfig.TicketAccessMode != "always" {
			t.Fatalf("general service access is not enabled: %+v", access)
		}
		route := manifestNodeByID(item.Definition, "service_access_route_1")
		var routeConfig dsl.ConditionConfig
		if route == nil || json.Unmarshal(route.Config, &routeConfig) != nil {
			t.Fatalf("general route config is invalid: %+v", route)
		}
		defaultTarget := ""
		for _, branch := range routeConfig.Branches {
			if branch.Default {
				defaultTarget = branch.TargetNodeID
			}
		}
		if defaultTarget != "understanding_1" {
			t.Fatalf("general consultation bypasses intent recognition: target=%q", defaultTarget)
		}
		retrieve := manifestNodeByID(item.Definition, "quick_retrieve_1")
		var retrieveConfig dsl.KnowledgeRetrieveConfig
		if retrieve == nil || json.Unmarshal(retrieve.Config, &retrieveConfig) != nil || retrieveConfig.EffectiveBindingMethod() != "agent_default" {
			t.Fatalf("general consultation is not using the default agent knowledge scope: %+v", retrieve)
		}
		reply := manifestNodeByID(item.Definition, "quick_reply_1")
		var replyConfig map[string]any
		if reply == nil || json.Unmarshal(reply.Config, &replyConfig) != nil || replyConfig["allowEmptyKnowledge"] != true {
			t.Fatalf("general consultation cannot continue without a knowledge hit: %+v", reply)
		}
		return
	}
	t.Fatal("default platform manifest not found")
}

func TestDefaultWorkflowDiagnosisFailureRequiresCustomerHandoffAction(t *testing.T) {
	items, err := builtin.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for _, item := range items {
		if item.Code != "aftersales_customer_service_default" {
			continue
		}
		route := manifestNodeByID(item.Definition, "diagnosis_failure_route_1")
		var routeConfig dsl.ConditionConfig
		if route == nil || json.Unmarshal(route.Config, &routeConfig) != nil {
			t.Fatalf("diagnosis failure route config is invalid: %+v", route)
		}
		if len(routeConfig.Branches) != 1 || !routeConfig.Branches[0].Default || routeConfig.Branches[0].TargetNodeID != "diagnostic_safe_reply_1" {
			t.Fatalf("diagnosis failure can still trigger an automatic action: %+v", routeConfig.Branches)
		}
		answerabilityRoute := manifestNodeByID(item.Definition, "answerability_route_1")
		var answerabilityConfig dsl.ConditionConfig
		if answerabilityRoute == nil || json.Unmarshal(answerabilityRoute.Config, &answerabilityConfig) != nil {
			t.Fatalf("answerability route config is invalid: %+v", answerabilityRoute)
		}
		for _, branch := range answerabilityConfig.Branches {
			if branch.TargetNodeID == "handoff_1" {
				t.Fatalf("unanswerable diagnosis can still trigger automatic handoff: %+v", answerabilityConfig.Branches)
			}
		}
		for _, edge := range item.Definition.Edges {
			if (edge.Source == "diagnosis_failure_route_1" || edge.Source == "answerability_route_1") && edge.Target == "handoff_1" {
				t.Fatalf("knowledge failure still has an automatic handoff edge: %+v", edge)
			}
		}
		reply := manifestNodeByID(item.Definition, "diagnostic_safe_reply_1")
		var replyConfig map[string]any
		if reply == nil || json.Unmarshal(reply.Config, &replyConfig) != nil {
			t.Fatalf("diagnosis safe reply config is invalid: %+v", reply)
		}
		content := stringValue(replyConfig["staticReply"])
		if !strings.Contains(content, "转人工") || !strings.Contains(content, "不会自动转人工或创建工单") {
			t.Fatalf("diagnosis safe reply is not actionable: %q", content)
		}
		return
	}
	t.Fatal("default platform manifest not found")
}

func TestDeviceDiagnosisManifestSupportsNoDeviceBasicReception(t *testing.T) {
	items, err := builtin.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for _, item := range items {
		if item.Code != "aftersales_device_diagnosis_ai_only" {
			continue
		}
		for _, node := range item.Definition.Nodes {
			if node.ID != "quick_reply_1" && node.ID != "quick_safe_reply_1" {
				continue
			}
			var config map[string]any
			if err := json.Unmarshal(node.Config, &config); err != nil {
				t.Fatalf("unmarshal %s config: %v", node.ID, err)
			}
			content := strings.TrimSpace(stringValue(config["prompt"]) + " " + stringValue(config["staticReply"]))
			if strings.Contains(content, "该产品的专属知识") || !strings.Contains(content, "产品名称") {
				t.Fatalf("no-device node %s still assumes a resolved product: %q", node.ID, content)
			}
			if node.ID == "quick_reply_1" && config["allowEmptyKnowledge"] != true {
				t.Fatalf("no-device quick reply should allow general reception without a knowledge hit: %+v", config)
			}
		}
		return
	}
	t.Fatal("device diagnosis platform manifest not found")
}

func TestDispatchOnlyManifestRoutesTicketsAndHandoffWithoutModelReplies(t *testing.T) {
	items, err := builtin.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for _, item := range items {
		if item.Code != "aftersales_customer_service_dispatch_only" {
			continue
		}
		if item.Definition.HasNodeType(workflowregistry.NodeTypeKnowledgeRetrieve) || item.Definition.HasNodeType(workflowregistry.NodeTypeAnswerabilityGate) {
			t.Fatalf("dispatch-only manifest still contains diagnosis nodes: %+v", item.Definition.Nodes)
		}
		for _, nodeType := range []string{
			workflowregistry.NodeTypeAnalyzeConversation,
			workflowregistry.NodeTypePrepareTicketDraft,
			workflowregistry.NodeTypeHumanConfirm,
			workflowregistry.NodeTypeCreateTicket,
			workflowregistry.NodeTypeHandoffToHuman,
		} {
			if !item.Definition.HasNodeType(nodeType) {
				t.Fatalf("dispatch-only manifest must include %s", nodeType)
			}
		}
		for _, node := range item.Definition.Nodes {
			if node.Type != workflowregistry.NodeTypeLLMReply {
				continue
			}
			var config map[string]any
			if err := json.Unmarshal(node.Config, &config); err != nil {
				t.Fatalf("unmarshal %s config: %v", node.ID, err)
			}
			if strings.TrimSpace(stringValue(config["staticReply"])) == "" {
				t.Fatalf("dispatch-only reply node %s must use static reply text", node.ID)
			}
			if strings.TrimSpace(stringValue(config["prompt"])) != "" {
				t.Fatalf("dispatch-only reply node %s must not use a model prompt", node.ID)
			}
		}
		return
	}
	t.Fatal("dispatch-only platform manifest not found")
}

func stringValue(value any) string {
	ret, _ := value.(string)
	return ret
}

func manifestNodeByID(definition dsl.Definition, id string) *dsl.Node {
	for i := range definition.Nodes {
		if definition.Nodes[i].ID == id {
			return &definition.Nodes[i]
		}
	}
	return nil
}

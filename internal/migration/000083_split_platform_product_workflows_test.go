package migration

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"remotehelpdesk/internal/ai/workflow/dsl"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/services"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestSplitPlatformProductWorkflowsCreatesRegisteredServiceModes(t *testing.T) {
	db, err := openMigrationTestDB(t, sqlite.Open("file:"+filepath.Join(t.TempDir(), "split-platform-product-workflows.db")), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "t_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&models.AIWorkflow{}, &models.AIWorkflowVersion{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	if err := splitPlatformProductWorkflows(db); err != nil {
		t.Fatalf("splitPlatformProductWorkflows() error = %v", err)
	}

	definitions := make(map[string]dsl.Definition)
	for _, code := range services.PlatformBuiltInWorkflowCodes() {
		workflow := repositoriesWorkflowByCodeForTest(t, db, code)
		var definition dsl.Definition
		if err := json.Unmarshal([]byte(workflow.DraftDefinition), &definition); err != nil {
			t.Fatalf("unmarshal %s definition: %v", code, err)
		}
		definitions[code] = definition
	}
	if len(definitions) != 4 {
		t.Fatalf("platform workflow count = %d, want 4", len(definitions))
	}
	deviceAI := definitions[services.PlatformDeviceAIOnlyWorkflowCode]
	productChain := []string{dsl.ModelCredentialScopeProduct, dsl.ModelCredentialScopeTenantDefault}
	if deviceAI.ModelPolicy == nil || !reflect.DeepEqual(deviceAI.ModelPolicy.CredentialChain, productChain) {
		t.Fatalf("device AI workflow credential chain = %#v, want %#v", deviceAI.ModelPolicy, productChain)
	}
	if !deviceAI.HasNodeType(workflowregistry.NodeTypeEntryContext) || !deviceAI.HasNodeType(workflowregistry.NodeTypeAnswerabilityGate) {
		t.Fatal("device AI workflow must include context recognition and answerability gating")
	}
	deviceRoute, ok := findWorkflowNode(deviceAI, "context_route_1")
	if !ok {
		t.Fatal("AI diagnosis workflow must route by device context")
	}
	var deviceRouteConfig dsl.ConditionConfig
	if err := json.Unmarshal(deviceRoute.Config, &deviceRouteConfig); err != nil {
		t.Fatalf("unmarshal AI diagnosis route config: %v", err)
	}
	if len(deviceRouteConfig.Branches) != 2 || deviceRouteConfig.Branches[0].TargetNodeID != "device_retrieve_1" || deviceRouteConfig.Branches[1].TargetNodeID != "quick_retrieve_1" {
		t.Fatalf("AI diagnosis workflow must cover bound and unbound devices: %#v", deviceRouteConfig.Branches)
	}
	for _, nodeType := range []string{workflowregistry.NodeTypeHandoffToHuman, workflowregistry.NodeTypeCreateTicket} {
		if deviceAI.HasNodeType(nodeType) {
			t.Fatalf("device AI workflow must not contain %s", nodeType)
		}
	}

	dispatchOnly := definitions[services.PlatformDispatchOnlyWorkflowCode]
	if dispatchOnly.HasNodeType(workflowregistry.NodeTypeKnowledgeRetrieve) || dispatchOnly.HasNodeType(workflowregistry.NodeTypeAnswerabilityGate) {
		t.Fatal("dispatch-only workflow must not contain diagnosis nodes")
	}
	if !dispatchOnly.HasNodeType(workflowregistry.NodeTypePrepareTicketDraft) || !dispatchOnly.HasNodeType(workflowregistry.NodeTypeHandoffToHuman) {
		t.Fatal("dispatch-only workflow must include ticket and human dispatch actions")
	}
	for _, node := range dispatchOnly.Nodes {
		if node.Type != workflowregistry.NodeTypeLLMReply {
			continue
		}
		var config map[string]any
		if err := json.Unmarshal(node.Config, &config); err != nil {
			t.Fatalf("unmarshal dispatch-only reply node %s config: %v", node.ID, err)
		}
		if strings.TrimSpace(stringValue(config["staticReply"])) == "" {
			t.Fatalf("dispatch-only reply node %s must be static", node.ID)
		}
		if prompt := strings.TrimSpace(stringValue(config["prompt"])); prompt != "" {
			t.Fatalf("dispatch-only reply node %s must not use a prompt: %q", node.ID, prompt)
		}
	}

	collaboration := definitions[services.PlatformDefaultAfterSalesWorkflowCode]
	if collaboration.ModelPolicy == nil || !reflect.DeepEqual(collaboration.ModelPolicy.CredentialChain, productChain) {
		t.Fatalf("collaboration workflow credential chain = %#v, want %#v", collaboration.ModelPolicy, productChain)
	}
	serviceRoute, ok := findWorkflowNode(collaboration, "service_access_route_1")
	if !ok {
		t.Fatal("collaboration workflow service route is missing")
	}
	var routeConfig dsl.ConditionConfig
	if err := json.Unmarshal(serviceRoute.Config, &routeConfig); err != nil {
		t.Fatalf("unmarshal service route config: %v", err)
	}
	deviceBranchFound := false
	for _, branch := range routeConfig.Branches {
		if branch.TargetNodeID == "understanding_1" && branch.Condition != nil && branch.Condition.Left != nil && branch.Condition.Left.NodeID == "entry_context_1" && branch.Condition.Left.Field == "deviceBound" {
			deviceBranchFound = true
		}
	}
	if !deviceBranchFound {
		t.Fatal("complete device diagnosis must depend on device context, not human handoff permission")
	}
	for _, nodeID := range []string{"retrieve_1", "reply_1"} {
		node, ok := findWorkflowNode(collaboration, nodeID)
		if !ok || node.ErrorTargetNodeID != "diagnosis_failure_route_1" {
			t.Fatalf("%s must route failures through product policy, got %q", nodeID, node.ErrorTargetNodeID)
		}
	}
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

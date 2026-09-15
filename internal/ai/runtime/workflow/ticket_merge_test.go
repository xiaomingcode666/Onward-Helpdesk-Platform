package workflow

import (
	"context"
	"encoding/json"
	"testing"

	"remotehelpdesk/internal/ai/workflow/dsl"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
	"remotehelpdesk/internal/models"
)

type completedTicketEffect struct {
	data   string
	writes int
}

func (s *completedTicketEffect) Acquire(context.Context, WorkflowEffectRequest) (WorkflowEffectLease, error) {
	return WorkflowEffectLease{Succeeded: true, Attempt: 1, ResultData: s.data}, nil
}
func (s *completedTicketEffect) Complete(context.Context, WorkflowEffectLease, WorkflowEffectRequest, string) error {
	s.writes++
	return nil
}
func (s *completedTicketEffect) Fail(context.Context, WorkflowEffectLease, string) error {
	s.writes++
	return nil
}

func TestGraphReplaysCreatedTicketAsMergedMain(t *testing.T) {
	db := setupWorkflowExecutorHandoffDB(t)
	main := models.Ticket{TenantID: 1, CustomerID: 5, TicketNo: "MAIN", Title: "Main", TicketGovernance: models.TicketGovernance{CaseType: "user_case", PriorityLevel: "p2"}}
	if err := db.Create(&main).Error; err != nil {
		t.Fatal(err)
	}
	source := models.Ticket{TenantID: 1, CustomerID: 5, TicketNo: "SOURCE", Title: "Duplicate", TicketGovernance: models.TicketGovernance{MergedIntoID: main.ID}}
	if err := db.Create(&source).Error; err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(map[string]any{"ticketId": source.ID, "ticketNo": source.TicketNo, "created": true})
	store := &completedTicketEffect{data: string(encoded)}
	execution := &graphExecution{state: newRunState(Input{Conversation: models.Conversation{ID: 10, TenantID: 1}, EffectStore: store})}
	node := dsl.Node{ID: "create", Type: workflowregistry.NodeTypeCreateTicket}
	if err := NewGraphExecutor().executeNode(context.Background(), execution, node); err != nil {
		t.Fatal(err)
	}
	output := execution.state.vars[node.ID]
	if toInt64(output["ticketId"]) != main.ID || output["ticketNo"] != "MAIN" || output["priority"] != "p2" || output["caseType"] != "user_case" {
		t.Fatalf("stale workflow output: %+v", output)
	}
	if store.writes != 0 {
		t.Fatal("replay reran a completed side effect")
	}
	handoffJSON, _ := json.Marshal(map[string]any{"ticketId": source.ID, "ticketNo": source.TicketNo, "message": "已转人工，等待处理"})
	handoffStore := &completedTicketEffect{data: string(handoffJSON)}
	handoff := &graphExecution{state: newRunState(Input{Conversation: models.Conversation{ID: 10, TenantID: 1}, EffectStore: handoffStore})}
	handoffNode := dsl.Node{ID: "handoff", Type: workflowregistry.NodeTypeHandoffToHuman}
	if err := NewGraphExecutor().executeNode(context.Background(), handoff, handoffNode); err != nil {
		t.Fatal(err)
	}
	handoffOutput := handoff.state.vars[handoffNode.ID]
	if toInt64(handoffOutput["ticketId"]) != main.ID || handoffOutput["message"] != "已转人工，等待处理" || handoffStore.writes != 0 {
		t.Fatal("handoff replay lost main ticket or routing result")
	}
	var count int64
	if err := db.Model(&models.Ticket{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatal("replay created a ticket", err)
	}
	foreign := &graphExecution{state: newRunState(Input{Conversation: models.Conversation{ID: 10, TenantID: 2}, EffectStore: store})}
	if err := NewGraphExecutor().executeNode(context.Background(), foreign, node); err == nil {
		t.Fatal("effect replay crossed tenant")
	}
}

func TestGraphReplaysParentLinkedTicketAsOriginalChild(t *testing.T) {
	db := setupWorkflowExecutorHandoffDB(t)
	if err := db.AutoMigrate(&models.TicketRelation{}); err != nil {
		t.Fatal(err)
	}
	parent := models.Ticket{TenantID: 1, CustomerID: 5, TicketNo: "FIRST", Title: "First report"}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	child := models.Ticket{TenantID: 1, CustomerID: 6, TicketNo: "LATER", Title: "Later report", TicketGovernance: models.TicketGovernance{CaseType: "user_case", PriorityLevel: "p2"}}
	if err := db.Create(&child).Error; err != nil {
		t.Fatal(err)
	}
	key := "parent-child-fixture"
	if err := db.Create(&models.TicketRelation{TenantID: 1, SourceID: parent.ID, TargetID: child.ID, Kind: "parent", ActiveKey: &key, ChildKey: &child.ID}).Error; err != nil {
		t.Fatal(err)
	}
	for _, nodeType := range []string{workflowregistry.NodeTypeCreateTicket, workflowregistry.NodeTypeHandoffToHuman} {
		encoded, _ := json.Marshal(map[string]any{"ticketId": child.ID, "ticketNo": child.TicketNo, "created": true})
		store := &completedTicketEffect{data: string(encoded)}
		execution := &graphExecution{state: newRunState(Input{Conversation: models.Conversation{ID: 10, TenantID: 1}, EffectStore: store})}
		node := dsl.Node{ID: "replay", Type: nodeType}
		if err := NewGraphExecutor().executeNode(context.Background(), execution, node); err != nil {
			t.Fatal(err)
		}
		output := execution.state.vars[node.ID]
		if toInt64(output["ticketId"]) != child.ID || output["ticketNo"] != child.TicketNo || store.writes != 0 {
			t.Fatalf("parent association redirected the child's conversation: %+v", output)
		}
	}
}

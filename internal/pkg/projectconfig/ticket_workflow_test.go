package projectconfig

import (
	"remotehelpdesk/internal/pkg/ticketpolicy"
	"testing"
)

func TestTicketWorkflowConfigurationSchemaAndPolicy(t *testing.T) {
	doc := Document{SchemaVersion: 1, TenantID: 91, Environment: "development", Projects: []Project{}, SecretRefs: []string{}, TicketWorkflows: map[string]ticketpolicy.Workflow{"*": ticketpolicy.DefaultWorkflow()}}
	if report := Validate(doc, 91, "development", nil); !report.Valid {
		t.Fatal(report)
	}
	oldDigest := Digest(doc)
	delete(doc.TicketWorkflows["*"].Transitions, "waiting")
	if report := Validate(doc, 91, "development", nil); report.Valid {
		t.Fatal("missing state passed deployment validation")
	}
	if err := ValidateDraftShape(doc); err == nil {
		t.Fatal("invalid graph entered a draft")
	}
	if Digest(doc) == oldDigest {
		t.Fatal("workflow changes not included in deployment digest")
	}
	doc.TicketWorkflows = map[string]ticketpolicy.Workflow{"missing": ticketpolicy.DefaultWorkflow()}
	if report := Validate(doc, 91, "development", nil); report.Valid {
		t.Fatal("unknown project accepted")
	}
	doc.Projects = []Project{{Key: "missing", Name: "已登记项目"}}
	if report := Validate(doc, 91, "development", nil); !report.Valid {
		t.Fatal(report)
	}
}

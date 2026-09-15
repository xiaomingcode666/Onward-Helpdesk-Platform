package enterprise

import (
	"encoding/json"
	"fmt"
	"net/http"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/services"
	"testing"
	"time"
)

func TestTicketGovernanceHTTPAuthorizationAndStableReceipt(t *testing.T) {
	f := newCaseHTTPFixture(t)
	if err := f.db.AutoMigrate(&models.TicketGovernanceOperation{}, &models.TicketRelation{}, &models.TicketPriorityProposal{}); err != nil {
		t.Fatal(err)
	}
	f.member(t, "manager", 9401, []string{constants.PermissionTicketView.Code, constants.PermissionTicketUpdate.Code})
	f.router.GET("/api/enterprise/v1/tickets/:id/governance", TicketGovernance)
	f.router.POST("/api/enterprise/v1/tickets/:id/governance", TicketGovernance)
	endpoint := fmt.Sprintf("/api/enterprise/v1/tickets/%d/governance", f.ticket.ID)
	revision := int64(0)
	cmd := services.TicketGovernanceCommand{Action: "classify", CaseType: "incident", Reason: "已确认发生故障", ExpectedRevision: &revision, OperationKey: "http-classify-1"}
	rec := f.request(t, "manager", http.MethodPost, endpoint, cmd)
	var receipt services.TicketGovernanceReceipt
	decodeEnterpriseData(t, rec, &receipt)
	if receipt.Revision != 1 {
		t.Fatal(receipt)
	}
	var view services.TicketGovernanceView
	decodeEnterpriseData(t, f.request(t, "manager", http.MethodGet, endpoint, nil), &view)
	if view.Priority != "p1" || view.CaseType != "incident" || !view.CanManage {
		t.Fatal(view)
	}
	cmd.Reason = "changed payload"
	if rec := f.request(t, "manager", http.MethodPost, endpoint, cmd); rec.Code != http.StatusConflict {
		t.Fatalf("expected 409: %d %s", rec.Code, rec.Body.String())
	}
	revision = 1
	cmd.OperationKey = "viewer-change-1"
	for _, actor := range []string{"viewer", "foreign", ""} {
		rec := f.request(t, actor, http.MethodPost, endpoint, cmd)
		if rec.Code == http.StatusOK && rec.Body.String() == "" {
			t.Fatal("missing rejection")
		}
		var envelope struct {
			Success bool `json:"success"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil || envelope.Success {
			t.Fatalf("expected rejection for %s: %s", actor, rec.Body.String())
		}
		// Re-read persistence, independent of this API's business-error HTTP convention.
		var item models.Ticket
		f.db.First(&item, f.ticket.ID)
		if item.GovernanceRevision != 1 {
			t.Fatalf("unauthorized actor %s mutated ticket", actor)
		}
	}
}

func TestTicketGovernanceHTTPSimpleChangeWithoutAssessmentOrCategory(t *testing.T) {
	f := newCaseHTTPFixture(t)
	if err := f.db.AutoMigrate(&models.TicketGovernanceOperation{}, &models.TicketRelation{}, &models.TicketPriorityProposal{}); err != nil {
		t.Fatal(err)
	}
	f.member(t, "editor", 9401, []string{constants.PermissionTicketView.Code, constants.PermissionTicketUpdate.Code})
	f.router.GET("/api/enterprise/v1/tickets/:id/governance", TicketGovernance)
	f.router.POST("/api/enterprise/v1/tickets/:id/governance", TicketGovernance)
	endpoint := fmt.Sprintf("/api/enterprise/v1/tickets/%d/governance", f.ticket.ID)
	revision := int64(0)
	classify := services.TicketGovernanceCommand{Action: "classify", CaseType: "known_error", Reason: "分类已确认", ExpectedRevision: &revision, OperationKey: "simple-classify"}
	var receipt services.TicketGovernanceReceipt
	decodeEnterpriseData(t, f.request(t, "editor", http.MethodPost, endpoint, classify), &receipt)
	var view services.TicketGovernanceView
	decodeEnterpriseData(t, f.request(t, "editor", http.MethodGet, endpoint, nil), &view)
	if view.Priority != "p3" || view.ReviewRequired {
		t.Fatalf("unexpected initial urgency: %+v", view)
	}
	revision = receipt.Revision
	change := services.TicketGovernanceCommand{Action: "override", Priority: "p1", Reason: "实际影响扩大，需要立即处理", ExpectedRevision: &revision, OperationKey: "simple-override"}
	decodeEnterpriseData(t, f.request(t, "editor", http.MethodPost, endpoint, change), &receipt)
	decodeEnterpriseData(t, f.request(t, "editor", http.MethodGet, endpoint, nil), &view)
	if view.Priority != "p1" || !view.Overridden || view.ReviewRequired || len(view.Proposals) != 0 {
		t.Fatalf("direct adjustment failed: %+v", view)
	}
	if len(view.History) != 2 || view.History[0].Reason != change.Reason {
		t.Fatal("reason history missing")
	}
	var again services.TicketGovernanceReceipt
	decodeEnterpriseData(t, f.request(t, "editor", http.MethodPost, endpoint, change), &again)
	if again != receipt {
		t.Fatal("retry changed receipt")
	}
}

func TestTicketGovernanceHTTPDuplicateParentAndQuery(t *testing.T) {
	f := newCaseHTTPFixture(t)
	if err := f.db.AutoMigrate(&models.TicketGovernanceOperation{}, &models.TicketRelation{}, &models.TicketPriorityProposal{}); err != nil {
		t.Fatal(err)
	}
	f.member(t, "merger", 9401, []string{constants.PermissionTicketView.Code, constants.PermissionTicketUpdate.Code, constants.PermissionTicketChangeStatus.Code})
	f.router.GET("/api/enterprise/v1/tickets/:id/governance", TicketGovernance)
	f.router.POST("/api/enterprise/v1/tickets/:id/governance", TicketGovernance)
	f.ticket.CustomerID = 42
	if err := f.db.Save(&f.ticket).Error; err != nil {
		t.Fatal(err)
	}
	main := f.ticket
	main.ID = 0
	main.TicketNo = "HTTP-MAIN"
	main.CreatedAt = f.ticket.CreatedAt.Add(-time.Hour)
	if err := f.db.Create(&main).Error; err != nil {
		t.Fatal(err)
	}
	endpoint := fmt.Sprintf("/api/enterprise/v1/tickets/%d/governance", f.ticket.ID)
	var candidates []services.TicketDuplicateCandidate
	decodeEnterpriseData(t, f.request(t, "merger", http.MethodGet, endpoint+"?duplicate_candidates=1", nil), &candidates)
	if len(candidates) != 1 || candidates[0].OtherID != main.ID {
		t.Fatal(candidates)
	}
	revision := int64(0)
	cmd := services.TicketGovernanceCommand{Action: "duplicate_child", ExpectedRevision: &revision, TargetRevision: &revision, TargetID: main.ID, OperationKey: "http-merge-once"}
	var receipt services.TicketGovernanceReceipt
	decodeEnterpriseData(t, f.request(t, "merger", http.MethodPost, endpoint, cmd), &receipt)
	var again services.TicketGovernanceReceipt
	decodeEnterpriseData(t, f.request(t, "merger", http.MethodPost, endpoint, cmd), &again)
	if again != receipt {
		t.Fatal("HTTP retry duplicated merge")
	}
	var sourceView services.TicketGovernanceView
	decodeEnterpriseData(t, f.request(t, "merger", http.MethodGet, endpoint, nil), &sourceView)
	if len(sourceView.Relations) != 1 || sourceView.Relations[0].SourceID != main.ID || sourceView.Relations[0].TargetID != f.ticket.ID || sourceView.CanAssociateDuplicate || sourceView.Merge.CanMerge {
		t.Fatal("bad merged source query")
	}
	var mainView services.TicketGovernanceView
	decodeEnterpriseData(t, f.request(t, "merger", http.MethodGet, fmt.Sprintf("/api/enterprise/v1/tickets/%d/governance", main.ID), nil), &mainView)
	if len(mainView.Relations) != 1 || mainView.Relations[0].OtherID != f.ticket.ID || mainView.Relations[0].Kind != "parent" {
		t.Fatal("missing source content")
	}
}

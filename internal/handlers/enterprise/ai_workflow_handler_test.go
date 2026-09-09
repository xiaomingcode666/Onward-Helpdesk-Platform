package enterprise

import (
	"fmt"
	"net/http"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
)

func TestAIWorkflowSummaryAndAdoptionAreScopedAndSplit(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	tenantID := int64(270814)
	otherTenantID := int64(270815)
	for _, tenant := range []models.Tenant{
		{ID: tenantID, Name: "Workflow tenant", Status: enums.StatusOk},
		{ID: otherTenantID, Name: "Other workflow tenant", Status: enums.StatusOk},
	} {
		if err := db.Create(&tenant).Error; err != nil {
			t.Fatalf("create tenant: %v", err)
		}
	}

	platformWorkflow := models.AIWorkflow{
		TenantID:               0,
		Code:                   "platform-flow",
		Scope:                  models.AIWorkflowScopePlatform,
		Name:                   "Platform Flow",
		Status:                 enums.StatusOk,
		Locked:                 true,
		CurrentStableVersionID: 11,
	}
	tenantWorkflow := models.AIWorkflow{
		TenantID:               tenantID,
		Code:                   "tenant-flow",
		Scope:                  models.AIWorkflowScopeTenant,
		Name:                   "Tenant Flow",
		Status:                 enums.StatusOk,
		CurrentStableVersionID: 21,
		SourceWorkflowID:       0,
		SourceVersionID:        10,
	}
	deletedWorkflow := models.AIWorkflow{
		TenantID: tenantID,
		Code:     "deleted-flow",
		Scope:    models.AIWorkflowScopeTenant,
		Name:     "Deleted Flow",
		Status:   enums.StatusDeleted,
	}
	otherWorkflow := models.AIWorkflow{
		TenantID:               otherTenantID,
		Code:                   "other-flow",
		Scope:                  models.AIWorkflowScopeTenant,
		Name:                   "Other Flow",
		Status:                 enums.StatusOk,
		CurrentStableVersionID: 31,
	}
	if err := db.Create(&[]*models.AIWorkflow{&platformWorkflow, &tenantWorkflow, &deletedWorkflow, &otherWorkflow}).Error; err != nil {
		t.Fatalf("create workflows: %v", err)
	}
	if err := db.Model(&tenantWorkflow).Update("source_workflow_id", platformWorkflow.ID).Error; err != nil {
		t.Fatalf("set tenant source workflow: %v", err)
	}
	tenantWorkflow.SourceWorkflowID = platformWorkflow.ID

	agents := []*models.AIAgent{
		{
			TenantID:          tenantID,
			ProductID:         101,
			Source:            "product_auto",
			Name:              "Platform adopter",
			Status:            enums.StatusOk,
			WorkflowID:        platformWorkflow.ID,
			WorkflowVersionID: 10,
			ReviewStatus:      enums.AIAgentReviewStatusApproved,
		},
		{
			TenantID:          tenantID,
			ProductID:         102,
			Source:            "product_auto",
			Name:              "Tenant adopter",
			Status:            enums.StatusOk,
			WorkflowID:        tenantWorkflow.ID,
			WorkflowVersionID: tenantWorkflow.CurrentStableVersionID,
			ReviewStatus:      enums.AIAgentReviewStatusApproved,
		},
		{
			TenantID:          tenantID,
			ProductID:         103,
			Source:            "product_auto",
			Name:              "Deleted adopter",
			Status:            enums.StatusDeleted,
			WorkflowID:        tenantWorkflow.ID,
			WorkflowVersionID: tenantWorkflow.CurrentStableVersionID,
			ReviewStatus:      enums.AIAgentReviewStatusApproved,
		},
		{
			TenantID:          otherTenantID,
			ProductID:         201,
			Source:            "product_auto",
			Name:              "Other tenant adopter",
			Status:            enums.StatusOk,
			WorkflowID:        tenantWorkflow.ID,
			WorkflowVersionID: tenantWorkflow.CurrentStableVersionID,
			ReviewStatus:      enums.AIAgentReviewStatusApproved,
		},
	}
	if err := db.Create(&agents).Error; err != nil {
		t.Fatalf("create agents: %v", err)
	}

	summaryCtx, summaryRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/ai-workflows/summary", tenantID)
	AIWorkflowSummary(summaryCtx)
	var summary response.AIWorkflowSummaryResponse
	decodeEnterpriseData(t, summaryRec, &summary)
	if summary.Total != 2 || summary.Adopted != 2 || summary.Attention != 2 {
		t.Fatalf("summary = %+v, want total/adopted/attention = 2/2/2", summary)
	}
	if len(summary.PlatformTemplates) != 1 || summary.PlatformTemplates[0].ID != platformWorkflow.ID {
		t.Fatalf("platform templates = %+v, want platform workflow only", summary.PlatformTemplates)
	}
	if summary.StableVersionByWorkflow[platformWorkflow.ID] != platformWorkflow.CurrentStableVersionID ||
		summary.StableVersionByWorkflow[tenantWorkflow.ID] != tenantWorkflow.CurrentStableVersionID {
		t.Fatalf("stable version map = %+v", summary.StableVersionByWorkflow)
	}

	adoptionCtx, adoptionRec := newEnterpriseContractContext(
		http.MethodGet,
		fmt.Sprintf("/api/enterprise/v1/ai-workflows/adoption?workflow_ids=%d", tenantWorkflow.ID),
		tenantID,
	)
	AIWorkflowAdoption(adoptionCtx)
	var adoption response.AIWorkflowAdoptionResponse
	decodeEnterpriseData(t, adoptionRec, &adoption)
	if adoption.Adoption[tenantWorkflow.ID] != 1 || adoption.StableAdoption[tenantWorkflow.ID] != 1 {
		t.Fatalf("tenant workflow adoption = %+v stable = %+v, want 1/1", adoption.Adoption, adoption.StableAdoption)
	}
	if _, ok := adoption.Adoption[platformWorkflow.ID]; ok {
		t.Fatalf("adoption endpoint leaked unrequested workflow: %+v", adoption.Adoption)
	}
	if adoption.StableVersionByWorkflow[platformWorkflow.ID] != platformWorkflow.CurrentStableVersionID {
		t.Fatalf("source stable version missing: %+v", adoption.StableVersionByWorkflow)
	}
}

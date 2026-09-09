package enterprise

import (
	"fmt"
	"net/http"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"
)

func TestEnterpriseAIAgentSummaryAndListAreScopedAndPaginated(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	tenantID := int64(260814)
	otherTenantID := int64(260815)
	for _, tenant := range []models.Tenant{
		{ID: tenantID, Name: "AI agent tenant", Status: enums.StatusOk},
		{ID: otherTenantID, Name: "Other AI agent tenant", Status: enums.StatusOk},
	} {
		if err := db.Create(&tenant).Error; err != nil {
			t.Fatalf("create tenant: %v", err)
		}
	}

	productA := models.Product{TenantID: tenantID, Code: "AI-PROD-A", Name: "Service Product A", Status: enums.StatusOk}
	productB := models.Product{TenantID: tenantID, Code: "AI-PROD-B", Name: "Service Product B", Status: enums.StatusOk}
	deletedProduct := models.Product{TenantID: tenantID, Code: "AI-PROD-X", Name: "Deleted", Status: enums.StatusDeleted}
	otherProduct := models.Product{TenantID: otherTenantID, Code: "AI-PROD-C", Name: "Other Product", Status: enums.StatusOk}
	if err := db.Create(&[]*models.Product{&productA, &productB, &deletedProduct, &otherProduct}).Error; err != nil {
		t.Fatalf("create products: %v", err)
	}
	workflowA := models.AIWorkflow{TenantID: tenantID, Code: "agent-flow-a", Scope: models.AIWorkflowScopeTenant, Name: "Agent Flow A", Status: enums.StatusOk}
	workflowB := models.AIWorkflow{TenantID: tenantID, Code: "agent-flow-b", Scope: models.AIWorkflowScopeTenant, Name: "Agent Flow B", Status: enums.StatusOk}
	if err := db.Create(&[]*models.AIWorkflow{&workflowA, &workflowB}).Error; err != nil {
		t.Fatalf("create workflows: %v", err)
	}

	agents := []*models.AIAgent{
		{
			TenantID:        tenantID,
			ProductID:       0,
			Source:          services.TenantDefaultAIAgentSource,
			Name:            "Default reception",
			Status:          enums.StatusOk,
			ActiveReleaseID: 9,
			ReviewStatus:    enums.AIAgentReviewStatusApproved,
		},
		{
			TenantID:     tenantID,
			ProductID:    productA.ID,
			Source:       "product_auto",
			Name:         "Product A reception",
			Status:       enums.StatusDisabled,
			WorkflowID:   workflowA.ID,
			ReviewStatus: enums.AIAgentReviewStatusUnreviewed,
		},
		{
			TenantID:     tenantID,
			ProductID:    productB.ID,
			Source:       "product_auto",
			Name:         "Deleted product reception",
			Status:       enums.StatusDeleted,
			WorkflowID:   workflowA.ID,
			ReviewStatus: enums.AIAgentReviewStatusUnreviewed,
		},
		{
			TenantID:     tenantID,
			ProductID:    productB.ID,
			Source:       "product_auto",
			Name:         "Product B reception",
			Status:       enums.StatusOk,
			WorkflowID:   workflowB.ID,
			ReviewStatus: enums.AIAgentReviewStatusApproved,
		},
		{
			TenantID:        otherTenantID,
			ProductID:       otherProduct.ID,
			Source:          "product_auto",
			Name:            "Other tenant reception",
			Status:          enums.StatusOk,
			ActiveReleaseID: 7,
			ReviewStatus:    enums.AIAgentReviewStatusApproved,
		},
	}
	if err := db.Create(&agents).Error; err != nil {
		t.Fatalf("create agents: %v", err)
	}

	summaryCtx, summaryRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/ai-agents/summary", tenantID)
	EnterpriseAIAgentSummary(summaryCtx)
	var summary response.EnterpriseAIAgentSummaryResponse
	decodeEnterpriseData(t, summaryRec, &summary)
	if summary.Total != 3 || summary.Active != 1 || summary.NotDeployed != 2 {
		t.Fatalf("summary core = %+v, want total=3 active=1 notDeployed=2", summary)
	}
	if summary.ProductTotal != 2 || summary.ProductAgentTotal != 2 || summary.MissingProductAgents != 0 {
		t.Fatalf("summary product coverage = %+v, want productTotal=2 productAgentTotal=2 missing=0", summary)
	}
	if !summary.TenantDefaultReady {
		t.Fatalf("tenant default should be ready: %+v", summary)
	}
	if len(summary.ProductAgentProductIDs) != 2 || summary.ProductAgentProductIDs[0] != productA.ID || summary.ProductAgentProductIDs[1] != productB.ID {
		t.Fatalf("summary product ids = %+v, want product A and B", summary.ProductAgentProductIDs)
	}

	listCtx, listRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/ai-agents?page=1&limit=1&productScoped=0", tenantID)
	EnterpriseAIAgentList(listCtx)
	var list struct {
		Results []response.AIAgentResponse `json:"results"`
		Page    struct {
			Page  int   `json:"page"`
			Limit int   `json:"limit"`
			Total int64 `json:"total"`
		} `json:"page"`
	}
	decodeEnterpriseData(t, listRec, &list)
	if list.Page.Total != 2 || list.Page.Page != 1 || list.Page.Limit != 1 {
		t.Fatalf("list page = %+v, want two scoped product agents with one item page", list.Page)
	}
	if len(list.Results) != 1 || list.Results[0].ProductID != productB.ID {
		t.Fatalf("list results = %+v, want latest product scoped agent first", list.Results)
	}

	tenantCtx, tenantRec := newEnterpriseContractContext(
		http.MethodGet,
		fmt.Sprintf("/api/enterprise/v1/ai-agents?productId=0&source=%s", services.TenantDefaultAIAgentSource),
		tenantID,
	)
	EnterpriseAIAgentList(tenantCtx)
	var tenantList struct {
		Results []response.AIAgentResponse `json:"results"`
		Page    struct {
			Total int64 `json:"total"`
		} `json:"page"`
	}
	decodeEnterpriseData(t, tenantRec, &tenantList)
	if tenantList.Page.Total != 1 || len(tenantList.Results) != 1 || tenantList.Results[0].Source != services.TenantDefaultAIAgentSource {
		t.Fatalf("tenant default list = %+v", tenantList)
	}

	workflowCtx, workflowRec := newEnterpriseContractContext(
		http.MethodGet,
		fmt.Sprintf("/api/enterprise/v1/ai-agents?page=1&limit=10&workflowId=%d", workflowA.ID),
		tenantID,
	)
	EnterpriseAIAgentList(workflowCtx)
	var workflowList struct {
		Results []response.AIAgentResponse `json:"results"`
		Page    struct {
			Total int64 `json:"total"`
		} `json:"page"`
	}
	decodeEnterpriseData(t, workflowRec, &workflowList)
	if workflowList.Page.Total != 1 || len(workflowList.Results) != 1 || workflowList.Results[0].WorkflowID != workflowA.ID {
		t.Fatalf("workflow filtered list = %+v", workflowList)
	}
}

package enterprise

import (
	"fmt"
	"net/http"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/gin-gonic/gin"
)

func TestEnterpriseAIWorkflowVersionListFiltersStableVersions(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	if err := db.Create(&models.Tenant{ID: 202601, Name: "Workflow version tenant", Status: enums.StatusOk}).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	workflow := models.AIWorkflow{
		TenantID: 202601,
		Code:     "wf-version-filter",
		Scope:    models.AIWorkflowScopeTenant,
		Name:     "Version filter",
		AgentID:  0,
		Status:   enums.StatusOk,
	}
	if err := db.Create(&workflow).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	versions := []models.AIWorkflowVersion{
		{WorkflowID: workflow.ID, Version: 1, Status: enums.StatusOk, ReleaseChannel: models.AIWorkflowReleaseChannelPreview, Definition: `{"nodes":[],"edges":[]}`, DefinitionHash: "preview-1"},
		{WorkflowID: workflow.ID, Version: 2, Status: enums.StatusOk, ReleaseChannel: models.AIWorkflowReleaseChannelStable, Definition: `{"nodes":[],"edges":[]}`, DefinitionHash: "stable-2"},
		{WorkflowID: workflow.ID, Version: 3, Status: enums.StatusOk, ReleaseChannel: models.AIWorkflowReleaseChannelStable, Definition: `{"nodes":[],"edges":[]}`, DefinitionHash: "stable-3"},
		{WorkflowID: workflow.ID, Version: 4, Status: enums.StatusDeleted, ReleaseChannel: models.AIWorkflowReleaseChannelStable, Definition: `{"nodes":[],"edges":[]}`, DefinitionHash: "deleted-4"},
	}
	if err := db.Create(&versions).Error; err != nil {
		t.Fatalf("create workflow versions: %v", err)
	}

	ctx, rec := newEnterpriseContractContext(
		http.MethodGet,
		fmt.Sprintf("/api/enterprise/v1/ai-workflows/%d/versions?page=1&limit=1&releaseChannel=stable&status=0", workflow.ID),
		202601,
		gin.Param{Key: "id", Value: fmt.Sprint(workflow.ID)},
	)
	AIWorkflowVersionList(ctx)

	var payload struct {
		Results []struct {
			ID             int64  `json:"id"`
			Version        int    `json:"version"`
			Status         int    `json:"status"`
			ReleaseChannel string `json:"releaseChannel"`
		} `json:"results"`
		Page struct {
			Page  int   `json:"page"`
			Limit int   `json:"limit"`
			Total int64 `json:"total"`
		} `json:"page"`
	}
	decodeEnterpriseData(t, rec, &payload)

	if payload.Page.Page != 1 || payload.Page.Limit != 1 || payload.Page.Total != 2 {
		t.Fatalf("page = %+v, want page 1 limit 1 total 2", payload.Page)
	}
	if len(payload.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(payload.Results))
	}
	got := payload.Results[0]
	if got.Version != 3 || got.Status != int(enums.StatusOk) || got.ReleaseChannel != models.AIWorkflowReleaseChannelStable {
		t.Fatalf("first version = %+v, want latest stable ok version", got)
	}
}

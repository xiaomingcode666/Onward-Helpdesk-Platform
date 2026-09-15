package enterprise

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/ticketpolicy"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"
	"testing"
)

func TestTicketStatusWorkflowHTTPPublishAndExecute(t *testing.T) {
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "development")
	t.Setenv("RHD_PROJECT_CONFIG_FILE", "")
	t.Setenv("RHD_PROJECT_CONFIG_REQUIRED", "")
	f := newCaseHTTPFixture(t)
	require.NoError(t, f.db.AutoMigrate(&models.ProjectConfigurationVersion{}, &models.ProjectConfigurationState{}, &models.ProjectConfigurationActivation{}, &models.DomainEvent{}, &models.OutboxRecord{}))
	f.owner.Permissions = append(f.owner.Permissions, constants.PermissionTicketUpdate.Code)
	f.owner.Roles = []string{services.EnterpriseRoleAdmin}
	path := "/api/enterprise/v1/ticket-settings/status-workflow"
	f.router.GET(path, TicketStatusWorkflowGet)
	f.router.POST(path+"/drafts", TicketStatusWorkflowDraft)
	f.router.POST(path+"/publish", TicketStatusWorkflowPublish)
	var view services.TicketWorkflowView
	decodeEnterpriseData(t, f.request(t, "owner", http.MethodGet, path, nil), &view)
	require.Zero(t, view.ActiveVersionID)
	require.True(t, view.CanManage)
	require.Len(t, view.States, 10)
	workflow := ticketpolicy.DefaultWorkflow()
	workflow.Transitions["in_triage"] = []string{"assigned", "waiting", "restored", "cancelled"}
	body := services.TicketWorkflowDraft{ProjectKey: "*", Workflow: workflow, RequestKey: "http-workflow-draft", Note: "解决前确认恢复"}
	assertEnterpriseEnvelopeError(t, f.request(t, "viewer", http.MethodPost, path+"/drafts", body))
	assertEnterpriseEnvelopeError(t, f.request(t, "owner", http.MethodGet, path+"?before_id=invalid", nil))
	assertEnterpriseEnvelopeError(t, f.request(t, "owner", http.MethodPost, path+"/drafts", map[string]any{"tenant_id": 9402}))
	var saved struct {
		ID int64 `json:"id"`
	}
	decodeEnterpriseData(t, f.request(t, "owner", http.MethodPost, path+"/drafts", body), &saved)
	require.Positive(t, saved.ID)
	decodeEnterpriseData(t, f.request(t, "owner", http.MethodPost, path+"/publish", map[string]any{"version_id": saved.ID}), &saved)
	f.actors["foreign"].Permissions = append(f.actors["foreign"].Permissions, constants.PermissionTicketUpdate.Code)
	f.actors["foreign"].Roles = []string{services.EnterpriseRoleAdmin}
	assertEnterpriseEnvelopeError(t, f.request(t, "foreign", http.MethodPost, path+"/publish", map[string]any{"version_id": saved.ID}))
	fresh := f.ticket
	fresh.ID = 0
	fresh.TicketNo = "WORKFLOW-HTTP-PUBLISHED"
	require.NoError(t, repositories.TicketRepository.Create(f.db, &fresh))
	f.ticket = fresh
	f.aggregateAfterCommand(t, f.command(t, "owner", "acknowledge", "new", 0, "wf-ack", "客服受理"), "acknowledged", 1)
	aggregate := f.aggregateAfterCommand(t, f.command(t, "owner", "triage", "acknowledged", 1, "wf-triage", ""), "in_triage", 2)
	require.Equal(t, saved.ID, aggregate.CaseLifecycle.WorkflowVersionID)
	require.NotContains(t, aggregate.CaseLifecycle.AllowedActions, "resolve")
	assertEnterpriseEnvelopeError(t, f.command(t, "owner", "resolve", "in_triage", 2, "wf-block", "尝试跳过恢复"))
	f.aggregateAfterCommand(t, f.command(t, "owner", "restore", "in_triage", 2, "wf-restore", "验证服务可用"), "restored", 3)
	f.aggregateAfterCommand(t, f.command(t, "owner", "resolve", "restored", 3, "wf-resolve", "问题修复验证通过"), "resolved", 4)
	decodeEnterpriseData(t, f.request(t, "owner", http.MethodGet, path, nil), &view)
	require.Equal(t, saved.ID, view.ActiveVersionID)
	// API returns only lifecycle settings, not mail credentials or unrelated config.
	require.NotContains(t, f.request(t, "viewer", http.MethodGet, path, nil).Body.String(), "password_ref")
	require.Equal(t, fmt.Sprint(saved.ID), fmt.Sprint(f.ticket.CaseWorkflowVersionID))
}

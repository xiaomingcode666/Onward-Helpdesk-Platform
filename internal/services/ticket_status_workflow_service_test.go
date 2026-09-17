package services

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/projectconfig"
	"remotehelpdesk/internal/pkg/ticketpolicy"
	"remotehelpdesk/internal/repositories"
	"slices"
	"testing"
)

func setupStatusWorkflow(t *testing.T) (*gorm.DB, *dto.AuthPrincipal) {
	t.Helper()
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "development")
	t.Setenv("RHD_PROJECT_CONFIG_FILE", "")
	t.Setenv("RHD_PROJECT_CONFIG_REQUIRED", "")
	db, op, _ := setupCaseLifecycle(t)
	require.NoError(t, db.AutoMigrate(&models.ProjectConfigurationState{}, &models.ProjectConfigurationVersion{}, &models.ProjectConfigurationActivation{}))
	op.Roles = append(op.Roles, EnterpriseRoleAdmin)
	op.Permissions = append(op.Permissions, constants.PermissionTicketUpdate.Code, constants.PermissionTicketView.Code)
	return db, op
}

func newWorkflowTicket(t *testing.T, db *gorm.DB, project string) models.Ticket {
	t.Helper()
	ticket := models.Ticket{TenantID: 91, ProjectKey: project, Title: "工作流测试工单", TicketNo: fmt.Sprintf("WF-%s-%d", t.Name(), workflowTestSequence), CaseStatus: "new", Status: enums.TicketStatusPendingAcceptance}
	workflowTestSequence++
	require.NoError(t, repositories.TicketRepository.Create(db, &ticket))
	return ticket
}

var workflowTestSequence int

func TestTicketStatusWorkflowPublishRuntimeAndRestore(t *testing.T) {
	db, op := setupStatusWorkflow(t)
	legacy := newWorkflowTicket(t, db, "")
	workflow := ticketpolicy.DefaultWorkflow()
	workflow.Transitions["resolved"] = []string{"closure_pending", "in_triage"}
	workflow.Transitions["closed"] = []string{}
	input := TicketWorkflowDraft{ProjectKey: "*", Workflow: workflow, RequestKey: "workflow-publish-1", Note: "关闭前确认"}
	draft, err := SaveTicketStatusWorkflow(91, input, op)
	require.NoError(t, err)
	retry, err := SaveTicketStatusWorkflow(91, input, op)
	require.NoError(t, err)
	require.Equal(t, draft.ID, retry.ID)
	require.Zero(t, newWorkflowTicket(t, db, "").CaseWorkflowVersionID, "draft cannot affect tickets")
	_, err = PublishTicketStatusWorkflow(91, draft.ID, op)
	require.NoError(t, err)
	_, err = PublishTicketStatusWorkflow(91, draft.ID, op)
	require.NoError(t, err)
	var count int64
	require.NoError(t, db.Model(&models.ProjectConfigurationActivation{}).Where("version_id = ?", draft.ID).Count(&count).Error)
	require.EqualValues(t, 1, count)
	ticket := newWorkflowTicket(t, db, "")
	require.Equal(t, draft.ID, ticket.CaseWorkflowVersionID)
	require.Equal(t, "*", ticket.CaseWorkflowKey)
	require.NoError(t, repositories.TicketRepository.Updates(db, ticket.ID, map[string]any{"current_assignee_id": op.UserID}))
	runCaseAction(t, db, ticket.ID, "triage", "", op)
	secondEngineer, _, _ := seedCaseOwnerMember(t, db, 91, "workflow-second-engineer", constants.PermissionTicketChangeStatus.Code)
	require.NoError(t, repositories.TicketRepository.Updates(db, ticket.ID, map[string]any{"current_assignee_id": secondEngineer.ID, "status": enums.TicketStatusPendingAssigneeAccept}))
	runCaseAction(t, db, ticket.ID, "wait", "等待供应商排查", op)
	resumed := runCaseAction(t, db, ticket.ID, "resume", "供应商已回复", op)
	require.Equal(t, "assigned", resumed.CaseStatus)
	runCaseAction(t, db, ticket.ID, "restore", "服务可以正常使用", op)
	resolved := runCaseAction(t, db, ticket.ID, "resolve", "根因修复已验证", op)
	require.NotContains(t, BuildTicketCaseLifecycle(&resolved, op).AllowedActions, "close")
	require.Error(t, TicketLifecycleService.Close(ticket.ID, "不得跳过待关闭", op))
	require.Error(t, repositories.TicketRepository.Updates(db, ticket.ID, map[string]any{"status": enums.TicketStatusClosed}))
	require.Equal(t, "resolved", repositories.TicketRepository.Get(db, ticket.ID).CaseStatus)
	runCaseAction(t, db, ticket.ID, "request_closure", "客户确认前复核", op)
	closed := runCaseAction(t, db, ticket.ID, "close", "客户确认", op)
	require.Equal(t, secondEngineer.ID, closed.CaseOwnerID)
	require.Empty(t, BuildTicketCaseLifecycle(&closed, op).AllowedActions)
	require.Error(t, TicketLifecycleService.Reopen(ticket.ID, "新流程禁用重新打开", op))
	cancelled := newWorkflowTicket(t, db, "")
	runCaseAction(t, db, cancelled.ID, "cancel", "误提交", op)

	// Restoring copies only workflow content into the CURRENT configuration.
	view, err := GetProjectConfiguration(91)
	require.NoError(t, err)
	doc := view.Document
	doc.Projects = []projectconfig.Project{{Key: "support", Name: "保留的新项目名称"}}
	config, err := SaveProjectConfigurationDraft(91, ProjectConfigDraft{Document: doc, BaseVersionID: draft.ID, RequestKey: "unrelated-config-change", Note: "新增项目"}, op)
	require.NoError(t, err)
	_, err = ApplyProjectConfiguration(91, config.ID, op)
	require.NoError(t, err)
	restored, err := SaveTicketStatusWorkflow(91, TicketWorkflowDraft{ProjectKey: "*", Workflow: ticketpolicy.DefaultWorkflow(), BaseVersionID: config.ID, RequestKey: "workflow-restore-1", Note: "恢复默认流程"}, op)
	require.NoError(t, err)
	require.Equal(t, doc.Projects, restored.Document.Projects)
	_, err = PublishTicketStatusWorkflow(91, restored.ID, op)
	require.NoError(t, err)
	// Retrying the first base-zero submission remains stable after other changes.
	replayed, err := SaveTicketStatusWorkflow(91, input, op)
	require.NoError(t, err)
	require.Equal(t, draft.ID, replayed.ID)
	changedInput := input
	changedInput.Note = "不同内容不能复用编号"
	_, err = SaveTicketStatusWorkflow(91, changedInput, op)
	require.ErrorIs(t, err, ErrProjectConfigConflict)
	require.Zero(t, repositories.TicketRepository.Get(db, legacy.ID).CaseWorkflowVersionID)
	require.Empty(t, BuildTicketCaseLifecycle(repositories.TicketRepository.Get(db, ticket.ID), op).AllowedActions, "old ticket stays pinned")
	fresh := newWorkflowTicket(t, db, "support")
	require.Equal(t, restored.ID, fresh.CaseWorkflowVersionID)
	require.NoError(t, repositories.TicketRepository.Updates(db, fresh.ID, map[string]any{"current_assignee_id": op.UserID}))
	runCaseAction(t, db, fresh.ID, "triage", "", op)
	runCaseAction(t, db, fresh.ID, "resolve", "已处理", op)
	fresh = runCaseAction(t, db, fresh.ID, "close", "确认", op)
	require.Contains(t, BuildTicketCaseLifecycle(&fresh, op).AllowedActions, "reopen")
}

func TestTicketStatusWorkflowScopePermissionsAndConflicts(t *testing.T) {
	db, op := setupStatusWorkflow(t)
	doc, err := GetProjectConfiguration(91)
	require.NoError(t, err)
	doc.Document.Projects = []projectconfig.Project{{Key: "support", Name: "支持服务"}}
	base, err := SaveProjectConfigurationDraft(91, ProjectConfigDraft{Document: doc.Document, RequestKey: "workflow-project-base", Note: "服务项目"}, op)
	require.NoError(t, err)
	_, err = ApplyProjectConfiguration(91, base.ID, op)
	require.NoError(t, err)
	w := ticketpolicy.DefaultWorkflow()
	w.Transitions["closed"] = []string{}
	input := TicketWorkflowDraft{ProjectKey: "support", Workflow: w, BaseVersionID: base.ID, RequestKey: "workflow-specific-project", Note: "特定服务不重开"}
	draft, err := SaveTicketStatusWorkflow(91, input, op)
	require.NoError(t, err)
	input.RequestKey = "workflow-other-draft"
	other, err := SaveTicketStatusWorkflow(91, input, op)
	require.NoError(t, err)
	_, err = PublishTicketStatusWorkflow(91, draft.ID, op)
	require.NoError(t, err)
	_, err = PublishTicketStatusWorkflow(91, other.ID, op)
	require.ErrorIs(t, err, ErrProjectConfigConflict)
	require.Equal(t, draft.ID, newWorkflowTicket(t, db, "support").CaseWorkflowVersionID)
	require.Zero(t, newWorkflowTicket(t, db, "").CaseWorkflowVersionID)
	ticket := newWorkflowTicket(t, db, "support")
	// Completing intake later cannot silently select a different workflow.
	require.NoError(t, repositories.TicketRepository.Updates(db, ticket.ID, map[string]any{"project_key": ""}))
	ticket = *repositories.TicketRepository.Get(db, ticket.ID)
	bound, err := repositories.TicketWorkflowDB(db, &ticket)
	require.NoError(t, err)
	require.Empty(t, bound.Transitions["closed"])
	require.Error(t, repositories.TicketRepository.Updates(db, ticket.ID, map[string]any{"case_workflow_version_id": 0}))
	ticket.TenantID = 92
	_, err = repositories.TicketWorkflowDB(db, &ticket)
	require.Error(t, err, "cannot load another tenant's version")
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "staging")
	ticket.TenantID = 91
	_, err = repositories.TicketWorkflowDB(db, &ticket)
	require.Error(t, err, "cannot use development version in staging")
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "development")

	editor := *op
	editor.Roles = nil
	editor.Permissions = []string{constants.PermissionTicketView.Code, constants.PermissionTicketUpdate.Code}
	input.BaseVersionID = draft.ID
	input.RequestKey = "unauthorized-workflow"
	_, err = SaveTicketStatusWorkflow(91, input, &editor)
	require.Error(t, err)
	view, err := GetProjectConfiguration(91)
	require.NoError(t, err)
	view.Document.TicketWorkflows = nil
	_, err = SaveProjectConfigurationDraft(91, ProjectConfigDraft{Document: view.Document, BaseVersionID: draft.ID, RequestKey: "strip-workflow-config", Note: "不得清空管理员配置"}, &editor)
	require.Error(t, err)
	input.ProjectKey = "missing-project"
	_, err = SaveTicketStatusWorkflow(91, input, op)
	require.Error(t, err)
	_, err = GetTicketStatusWorkflow(91, "missing-project", 0, op)
	require.Error(t, err)
	input.ProjectKey = "support"
	input.RequestKey = "managed-workflow-draft"
	managed, err := SaveTicketStatusWorkflow(91, input, op)
	require.NoError(t, err)
	t.Setenv("RHD_PROJECT_CONFIG_REQUIRED", "1")
	_, err = PublishTicketStatusWorkflow(91, managed.ID, op)
	require.ErrorContains(t, err, "部署")
	require.NoError(t, db.Migrator().DropTable(&models.ProjectConfigurationVersion{}))
	_, err = GetTicketStatusWorkflow(91, "*", 0, op)
	require.Error(t, err, "database failures must not look like default workflow")
	require.NotEmpty(t, BuildTicketCaseLifecycle(&ticket, op).WorkflowError)
	require.Empty(t, BuildTicketCaseLifecycle(&ticket, op).AllowedActions)
}

func TestTicketStatusWorkflowRejectsInvalidAndKeepsImmutableDefaults(t *testing.T) {
	for _, mutate := range []func(*ticketpolicy.Workflow){
		func(w *ticketpolicy.Workflow) { delete(w.Transitions, "waiting") },
		func(w *ticketpolicy.Workflow) { w.Transitions["new"] = []string{"resolved"} },
		func(w *ticketpolicy.Workflow) { w.Transitions["closed"] = []string{"reopened"} },
		func(w *ticketpolicy.Workflow) { w.Transitions["cancelled"] = []string{"in_triage"} },
		func(w *ticketpolicy.Workflow) { w.Transitions["assigned"] = nil },
		func(w *ticketpolicy.Workflow) { w.Transitions["new"] = append(w.Transitions["new"], "cancelled") },
	} {
		w := ticketpolicy.DefaultWorkflow()
		mutate(&w)
		require.Error(t, w.Validate())
	}
	w := ticketpolicy.DefaultWorkflow()
	w.Transitions["new"][0] = "closed"
	require.True(t, slices.Contains(ticketpolicy.DefaultWorkflow().Transitions["new"], "acknowledged"))
}

package services

import (
	"encoding/json"
	"fmt"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/projectconfig"
	"remotehelpdesk/internal/pkg/ticketpolicy"
	"remotehelpdesk/internal/repositories"
	"slices"
	"strings"
)

type TicketWorkflowVersion struct {
	ID        int64                 `json:"id"`
	Note      string                `json:"note"`
	Published bool                  `json:"published"`
	Workflow  ticketpolicy.Workflow `json:"workflow"`
}
type TicketWorkflowView struct {
	ActiveVersionID   int64                   `json:"active_version_id"`
	DeploymentManaged bool                    `json:"deployment_managed"`
	CanManage         bool                    `json:"can_manage"`
	ProjectKey        string                  `json:"project_key"`
	Projects          []projectconfig.Project `json:"projects"`
	Workflow          ticketpolicy.Workflow   `json:"workflow"`
	Allowed           ticketpolicy.Workflow   `json:"allowed"`
	Required          map[string][]string     `json:"required"`
	States            []string                `json:"states"`
	Versions          []TicketWorkflowVersion `json:"versions"`
	NextBeforeID      int64                   `json:"next_before_id"`
}
type TicketWorkflowDraft struct {
	ProjectKey    string                `json:"project_key"`
	Workflow      ticketpolicy.Workflow `json:"workflow"`
	BaseVersionID int64                 `json:"base_version_id"`
	RequestKey    string                `json:"request_key"`
	Note          string                `json:"note"`
}

func workflowForDocument(doc projectconfig.Document, key string) ticketpolicy.Workflow {
	if w, ok := doc.TicketWorkflows[key]; ok {
		return w
	}
	if w, ok := doc.TicketWorkflows["*"]; ok {
		return w
	}
	return ticketpolicy.DefaultWorkflow()
}

func validWorkflowProject(doc projectconfig.Document, key string) bool {
	if key == "*" {
		return true
	}
	for _, p := range doc.Projects {
		if p.Key == key {
			return true
		}
	}
	return false
}

func GetTicketStatusWorkflow(tenantID int64, key string, beforeID int64, op *dto.AuthPrincipal) (*TicketWorkflowView, error) {
	if op == nil || tenantID <= 0 || op.EffectiveTenantID() != tenantID || !op.HasPermission(constants.PermissionTicketView.Code) {
		return nil, errorsx.Forbidden("无权查看此公司的工单流程")
	}
	view, err := GetProjectConfigurationHistory(tenantID, beforeID)
	if err != nil {
		return nil, err
	}
	if !validWorkflowProject(view.Document, key) {
		return nil, fmt.Errorf("服务项目不存在，请重新选择")
	}
	result := &TicketWorkflowView{
		ActiveVersionID: view.ActiveVersionID, DeploymentManaged: view.DeploymentManaged,
		CanManage:  requireProjectConfigOperator(tenantID, op) == nil && RequireProjectRuntimeOperator(op) == nil,
		ProjectKey: key, Projects: view.Document.Projects, Workflow: workflowForDocument(view.Document, key),
		Allowed: ticketpolicy.DefaultWorkflow(), Required: ticketpolicy.RequiredCaseTransitions(), States: ticketpolicy.CaseStates(),
		Versions: []TicketWorkflowVersion{}, NextBeforeID: view.NextBeforeID,
	}
	if !result.CanManage {
		result.NextBeforeID = 0
		return result, nil
	}
	for _, v := range view.Versions {
		if !strings.HasPrefix(v.Note, "工单流程：") && v.Activation == nil {
			continue
		}
		result.Versions = append(result.Versions, TicketWorkflowVersion{v.ID, v.Note, v.Activation != nil, workflowForDocument(v.Document, key)})
	}
	return result, nil
}

// Only workflow rules are replaced. Other project settings are copied unchanged
// from the active version, including when restoring a historical workflow.
func SaveTicketStatusWorkflow(tenantID int64, input TicketWorkflowDraft, op *dto.AuthPrincipal) (*ProjectConfigVersion, error) {
	if err := requireProjectConfigOperator(tenantID, op); err != nil {
		return nil, err
	}
	if err := RequireProjectRuntimeOperator(op); err != nil {
		return nil, err
	}
	if err := input.Workflow.Validate(); err != nil {
		return nil, err
	}
	note := "工单流程：" + input.Note + "（范围：" + input.ProjectKey + "）"
	var previous models.ProjectConfigurationVersion
	if err := sqls.DB().Where("tenant_id = ? AND environment = ? AND request_key = ?", tenantID, projectconfig.Environment(), input.RequestKey).Limit(1).Find(&previous).Error; err != nil {
		return nil, err
	}
	if previous.ID > 0 {
		parsed, err := decodeProjectVersion(previous)
		if err != nil {
			return nil, err
		}
		stored, exists := parsed.Document.TicketWorkflows[input.ProjectKey]
		left, _ := json.Marshal(stored)
		right, _ := json.Marshal(input.Workflow)
		if !exists || previous.BaseVersionID != input.BaseVersionID || previous.Note != note || string(left) != string(right) {
			return nil, ErrProjectConfigConflict
		}
		return &parsed, nil
	}
	// Read the exact base so retries remain stable even after publication.
	var doc projectconfig.Document
	if input.BaseVersionID > 0 {
		var v models.ProjectConfigurationVersion
		if err := sqls.DB().Where("id = ? AND tenant_id = ? AND environment = ?", input.BaseVersionID, tenantID, projectconfig.Environment()).First(&v).Error; err != nil {
			return nil, err
		}
		decoded, err := decodeProjectVersion(v)
		if err != nil {
			return nil, err
		}
		doc = decoded.Document
	} else {
		var tenant models.Tenant
		if err := sqls.DB().First(&tenant, tenantID).Error; err != nil {
			return nil, err
		}
		var err error
		doc, err = legacyProjectDocument(&tenant)
		if err != nil {
			return nil, err
		}
	}
	if !validWorkflowProject(doc, input.ProjectKey) {
		return nil, fmt.Errorf("服务项目不存在，请重新选择")
	}
	if strings.TrimSpace(input.Note) == "" {
		return nil, fmt.Errorf("请填写修改原因")
	}
	if doc.TicketWorkflows == nil {
		doc.TicketWorkflows = map[string]ticketpolicy.Workflow{}
	}
	doc.TicketWorkflows[input.ProjectKey] = input.Workflow
	return SaveProjectConfigurationDraft(tenantID, ProjectConfigDraft{BaseVersionID: input.BaseVersionID, RequestKey: input.RequestKey, Note: note, Document: doc}, op)
}

func PublishTicketStatusWorkflow(tenantID, versionID int64, op *dto.AuthPrincipal) (*ProjectConfigVersion, error) {
	if err := requireProjectConfigOperator(tenantID, op); err != nil {
		return nil, err
	}
	if err := RequireProjectRuntimeOperator(op); err != nil {
		return nil, err
	}
	var version models.ProjectConfigurationVersion
	if err := sqls.DB().Where("id = ? AND tenant_id = ? AND environment = ?", versionID, tenantID, projectconfig.Environment()).First(&version).Error; err != nil {
		return nil, err
	}
	if !strings.HasPrefix(version.Note, "工单流程：") {
		return nil, fmt.Errorf("请选择工单流程草稿")
	}
	return ApplyProjectConfiguration(tenantID, versionID, op)
}

func ticketCaseWorkflowActionsDB(db *gorm.DB, ticket *models.Ticket, op *dto.AuthPrincipal) ([]string, error) {
	actions := ticketCaseActions(ticket, op)
	workflow, err := repositories.TicketWorkflowDB(db, ticket)
	if err != nil {
		return []string{}, err
	}
	if workflow == nil {
		return actions, nil
	}
	from := models.EffectiveTicketCaseStatus(*ticket)
	result := []string{}
	for _, action := range actions {
		target := map[string]string{"acknowledge": "acknowledged", "triage": "in_triage", "wait": "waiting", "restore": "restored", "resolve": "resolved", "request_closure": "closure_pending", "close": "closed", "cancel": "cancelled", "reopen": "in_triage"}[action]
		if action == "acknowledge" && from != "new" {
			target = from
		}
		if action == "resume" {
			target = ticket.CaseResumeStatus
			if !slices.Contains([]string{"acknowledged", "in_triage", "assigned", "restored"}, target) {
				target = "in_triage"
			}
		}
		if workflow.Allows(from, target) {
			result = append(result, action)
		}
	}
	return result, nil
}

// The general configuration API must not let a ticket editor remove an active
// administrator-owned workflow by submitting a document without this field.
func requireExistingWorkflowAdministratorDB(db *gorm.DB, versionID int64, op *dto.AuthPrincipal) error {
	if versionID == 0 {
		return nil
	}
	var v models.ProjectConfigurationVersion
	if err := db.First(&v, versionID).Error; err != nil {
		return err
	}
	parsed, err := decodeProjectVersion(v)
	if err != nil {
		return err
	}
	if len(parsed.Document.TicketWorkflows) > 0 {
		return RequireProjectRuntimeOperator(op)
	}
	return nil
}

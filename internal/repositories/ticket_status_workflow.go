package repositories

import (
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/projectconfig"
	"remotehelpdesk/internal/pkg/ticketpolicy"
)

func ticketWorkflowDocumentDB(db *gorm.DB, tenantID, versionID int64) (projectconfig.Document, error) {
	var v models.ProjectConfigurationVersion
	if err := db.Where("id = ? AND tenant_id = ? AND environment = ?", versionID, tenantID, projectconfig.Environment()).First(&v).Error; err != nil {
		return projectconfig.Document{}, err
	}
	doc, err := projectconfig.Decode([]byte(v.DocumentJSON))
	if err != nil {
		return doc, err
	}
	if projectconfig.Digest(doc) != v.Digest {
		return doc, fmt.Errorf("工单流程版本内容校验失败")
	}
	return doc, nil
}

// Called only on creation. Publishing never changes existing tickets.
func BindTicketWorkflowDB(db *gorm.DB, ticket *models.Ticket) error {
	ticket.CaseWorkflowVersionID = 0
	ticket.CaseWorkflowKey = ""
	if ticket.TenantID <= 0 {
		return nil
	}
	// Isolated legacy fixtures may predate configuration versioning.
	if !db.Migrator().HasTable(&models.ProjectConfigurationState{}) {
		return nil
	}
	id := ticket.ProjectConfigVersionID
	if id == 0 {
		id = ticket.IntakeConfigVersionID
	}
	if id == 0 {
		var state models.ProjectConfigurationState
		if err := db.Where("tenant_id = ? AND environment = ?", ticket.TenantID, projectconfig.Environment()).Limit(1).Find(&state).Error; err != nil {
			return err
		}
		id = state.ActiveVersionID
	}
	if id == 0 {
		return nil
	}
	doc, err := ticketWorkflowDocumentDB(db, ticket.TenantID, id)
	if err != nil {
		return err
	}
	workflow, ok := doc.TicketWorkflows[ticket.ProjectKey]
	key := ticket.ProjectKey
	if !ok {
		workflow, ok = doc.TicketWorkflows["*"]
		key = "*"
	}
	if !ok {
		return nil
	}
	if err := workflow.Validate(); err != nil {
		return err
	}
	ticket.CaseWorkflowVersionID = id
	ticket.CaseWorkflowKey = key
	return nil
}

func TicketWorkflowDB(db *gorm.DB, ticket *models.Ticket) (*ticketpolicy.Workflow, error) {
	if ticket.CaseWorkflowVersionID == 0 {
		return nil, nil
	}
	doc, err := ticketWorkflowDocumentDB(db, ticket.TenantID, ticket.CaseWorkflowVersionID)
	if err != nil {
		return nil, err
	}
	workflow, ok := doc.TicketWorkflows[ticket.CaseWorkflowKey]
	if !ok {
		return nil, fmt.Errorf("工单绑定版本中缺少流程配置")
	}
	return &workflow, workflow.Validate()
}

func GuardTicketWorkflowDB(db *gorm.DB, id int64, columns map[string]any) error {
	if _, ok := columns["case_workflow_key"]; ok {
		return fmt.Errorf("不能修改已有工单的流程范围")
	}
	if _, ok := columns["case_workflow_version_id"]; ok {
		return fmt.Errorf("不能修改已有工单的流程版本")
	}
	_, caseChanged := columns["case_status"]
	_, statusChanged := columns["status"]
	_, projectChanged := columns["project_key"]
	if !caseChanged && !statusChanged && !projectChanged {
		return nil
	}
	var ticket models.Ticket
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&ticket, id).Error; err != nil {
		return err
	}
	if ticket.CaseWorkflowVersionID == 0 {
		return nil
	}
	if !caseChanged {
		return nil
	}
	workflow, err := TicketWorkflowDB(db, &ticket)
	if err != nil {
		return err
	}
	if !workflow.Allows(models.EffectiveTicketCaseStatus(ticket), fmt.Sprint(columns["case_status"])) {
		return fmt.Errorf("当前工单流程不允许这次状态变更，请按工单显示的下一步操作")
	}
	return nil
}

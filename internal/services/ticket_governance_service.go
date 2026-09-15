package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/ticketpolicy"
	"remotehelpdesk/internal/repositories"
)

type TicketGovernanceCommand struct {
	Action           string             `json:"action"`
	ExpectedRevision *int64             `json:"expected_revision"`
	OperationKey     string             `json:"operation_key"`
	CaseType         string             `json:"case_type"`
	Facts            ticketpolicy.Facts `json:"facts"`
	Priority         string             `json:"priority"`
	Category         string             `json:"category"`
	Reason           string             `json:"reason"`
	ProposalID       int64              `json:"proposal_id"`
	UseLatestRules   bool               `json:"use_latest_rules"`
	RelationKind     string             `json:"relation_kind"`
	TargetID         int64              `json:"target_id"`
	TargetRevision   *int64             `json:"target_revision"`
	RelationID       int64              `json:"relation_id"`
}
type TicketGovernanceReceipt struct {
	TicketID     int64  `json:"ticket_id"`
	Revision     int64  `json:"revision"`
	OperationKey string `json:"operation_key"`
}

type TicketPriorityPreview struct {
	Priority               string     `json:"priority"`
	PreviousDeadline       *time.Time `json:"previous_deadline"`
	Deadline               *time.Time `json:"deadline"`
	PreviousAcceptDeadline *time.Time `json:"previous_accept_deadline"`
	AcceptDeadline         *time.Time `json:"accept_deadline"`
	Overdue                bool       `json:"overdue"`
}

func PreviewTicketPriorityChange(ticketID int64, priority string, op *dto.AuthPrincipal) (*TicketPriorityPreview, error) {
	if op == nil || !ticketpolicy.ValidPriority(priority) {
		return nil, errorsx.InvalidParam("请选择 P1–P4")
	}
	db := sqls.DB()
	var t models.Ticket
	if err := db.Where("id = ? AND tenant_id = ?", ticketID, op.EffectiveTenantID()).First(&t).Error; err != nil {
		return nil, err
	}
	if !canManageTicketGovernance(&t, op) {
		return nil, errorsx.Forbidden("无权评估该工单等级调整")
	}
	previous := t.SLADueAt
	previousAccept := t.AcceptDeadlineAt
	if priority != ticketEffectivePriority(t) {
		t.PriorityLevel = priority
		t.PriorityCode = ticketpolicy.LegacyCode(priority)
		if err := refreshGovernanceDeadlineDB(db, &t); err != nil {
			return nil, err
		}
	}
	return &TicketPriorityPreview{Priority: priority, PreviousDeadline: previous, Deadline: t.SLADueAt, PreviousAcceptDeadline: previousAccept, AcceptDeadline: t.AcceptDeadlineAt, Overdue: t.SLADueAt != nil && t.SLADueAt.Before(time.Now()) && !ticketResolutionSLAStopped(t)}, nil
}

type TicketGovernanceView struct {
	CanAssociateDuplicate bool                            `json:"can_associate_duplicate"`
	Merge                 TicketMergeView                 `json:"merge"`
	TicketID              int64                           `json:"ticket_id"`
	Revision              int64                           `json:"revision"`
	CaseType              string                          `json:"case_type"`
	Priority              string                          `json:"priority"`
	Suggested             string                          `json:"suggested"`
	Explanation           string                          `json:"explanation"`
	ReviewRequired        bool                            `json:"review_required"`
	Overridden            bool                            `json:"overridden"`
	Legacy                bool                            `json:"legacy"`
	Facts                 ticketpolicy.Facts              `json:"facts"`
	Policy                ticketpolicy.Policy             `json:"policy"`
	CanManage             bool                            `json:"can_manage"`
	CanPropose            bool                            `json:"can_propose"`
	Proposals             []models.TicketPriorityProposal `json:"proposals"`
	Relations             []TicketRelationView            `json:"relations"`
	History               []TicketGovernanceHistory       `json:"history"`
}
type TicketGovernanceHistory struct {
	ID        int64           `json:"id"`
	Action    string          `json:"action"`
	Reason    string          `json:"reason"`
	ActorID   int64           `json:"actor_id"`
	CreatedAt time.Time       `json:"created_at"`
	Details   json.RawMessage `json:"details"`
}

func ticketGovernanceAccess(t *models.Ticket, op *dto.AuthPrincipal) error {
	if op == nil || op.UserID <= 0 || op.IsCustomer() || op.IsPartner() || op.IsServiceAccount() || op.EffectiveTenantID() != t.TenantID || !op.HasPermission(constants.PermissionTicketView.Code) {
		return errorsx.Forbidden("无权查看工单内部分类与评估")
	}
	return requireTicketTenantAccess(t, op)
}
func canManageTicketGovernance(t *models.Ticket, op *dto.AuthPrincipal) bool {
	return ticketGovernanceAccess(t, op) == nil && (canManageTicketDispatch(op) || op.HasPermission(constants.PermissionTicketUpdate.Code))
}
func loadTicketPriorityPolicyDB(db *gorm.DB, t *models.Ticket, latest bool) (ticketpolicy.Policy, int64, error) {
	p := ticketpolicy.Default()
	if !latest && t.PriorityPolicyJSON != "" {
		if err := json.Unmarshal([]byte(t.PriorityPolicyJSON), &p); err != nil {
			return p, 0, err
		}
		return p, t.PriorityConfigVersionID, p.Validate()
	}
	id := t.ProjectConfigVersionID
	if latest {
		id = 0
	}
	if db.Migrator().HasTable(&models.ProjectConfigurationState{}) {
		if id == 0 {
			state, err := projectState(db, t.TenantID)
			if err != nil {
				return p, 0, err
			}
			id = state.ActiveVersionID
		}
		if id > 0 {
			var v models.ProjectConfigurationVersion
			if err := db.Where("id = ? AND tenant_id = ?", id, t.TenantID).First(&v).Error; err != nil {
				return p, 0, err
			}
			parsed, err := decodeProjectVersion(v)
			if err != nil {
				return p, 0, err
			}
			if parsed.Document.TicketPriority != nil {
				p = *parsed.Document.TicketPriority
			}
		}
	}
	return p, id, p.Validate()
}

func TicketPriorityPolicyForTenant(db *gorm.DB, tenantID int64) (ticketpolicy.Policy, error) {
	p, _, err := loadTicketPriorityPolicyDB(db, &models.Ticket{TenantID: tenantID}, true)
	return p, err
}

// Initial urgency is determined only by classification. Legacy assessment inputs
// remain accepted in the request contract but never change the effective level.
func initializeTicketGovernanceDB(db *gorm.DB, t *models.Ticket, caseType string, _ ticketpolicy.Facts) error {
	if caseType == "" {
		caseType = "user_case"
	}
	if err := applyTicketClassificationDB(db, t, caseType); err != nil {
		return err
	}
	return refreshGovernanceDeadlineDB(db, t)
}

func applyTicketClassificationDB(db *gorm.DB, t *models.Ticket, caseType string) error {
	if !slices.Contains(ticketpolicy.Types, caseType) {
		return errorsx.InvalidParam("请选择六种标准工单分类之一")
	}
	p, id, err := loadTicketPriorityPolicyDB(db, t, false)
	if err != nil {
		return err
	}
	encoded, _ := json.Marshal(p)
	t.CaseType = caseType
	t.PriorityLevel = p.Defaults[caseType]
	t.PrioritySuggested = t.PriorityLevel
	t.PriorityReviewRequired = false
	t.PriorityExplanation = "根据工单分类自动生成"
	// Preserve historical assessment evidence when correcting a classification.
	if t.PriorityFactsJSON == "" {
		t.PriorityFactsJSON = "{}"
	}
	t.PriorityPolicyJSON = string(encoded)
	t.PriorityConfigVersionID = id
	t.PriorityCode = ticketpolicy.LegacyCode(t.PriorityLevel)
	return nil
}
func refreshGovernanceDeadlineDB(db *gorm.DB, t *models.Ticket) error {
	if !ticketCaseClosed(*t) && t.AssignedAt != nil && t.AcceptedAt == nil && t.AcceptDeadlineAt != nil {
		deadline := ticketAssignmentDeadlineDB(db, t, *t.AssignedAt)
		t.AcceptDeadlineAt = &deadline
	}
	if ticketResolutionSLAStopped(*t) {
		return nil
	}
	r, _, err := projectRuntimeDB(db, t.TenantID, t.ProjectConfigVersionID)
	if err != nil {
		return err
	}
	var policy SLAPolicy
	if r != nil {
		target, cal, ok := projectTicketTarget(r, t.ProjectKey, t.PriorityCode)
		if !ok || target.ResolutionMinutes <= 0 {
			t.SLADueAt = nil
			return nil
		}
		policy = SLAPolicy{ResolutionMinutes: target.ResolutionMinutes, runtimeCalendar: &cal}
	} else {
		if !db.Migrator().HasTable(&SLAPolicy{}) {
			return nil
		}
		if err := db.Where("tenant_id = ? AND priority = ? AND status = 'active'", formatID(t.TenantID), t.PriorityCode).Limit(1).Find(&policy).Error; err != nil {
			return err
		}
		if policy.ID == "" || policy.ResolutionMinutes <= 0 {
			// A new legacy ticket may supply its own deadline when no SLA policy
			// exists. Priority changes on saved tickets must still clear stale targets.
			if t.ID != 0 || policy.ID != "" {
				t.SLADueAt = nil
			}
			return nil
		}
	}
	paused := time.Duration(0)
	if db.Migrator().HasTable(&SLAPauseRecord{}) {
		paused, err = policy.pausedDB(db, *t, time.Now())
		if err != nil {
			return err
		}
	}
	deadline := policy.deadline(t.CreatedAt, policy.ResolutionMinutes, paused)
	t.SLADueAt = &deadline
	return nil
}
func ticketEffectivePriority(t models.Ticket) string {
	if ticketpolicy.ValidPriority(t.PriorityLevel) {
		return t.PriorityLevel
	}
	return ticketpolicy.FromLegacy(strings.ToLower(strings.TrimSpace(t.PriorityCode)))
}

func GetTicketGovernance(ticketID int64, op *dto.AuthPrincipal) (*TicketGovernanceView, error) {
	if op == nil {
		return nil, errorsx.Forbidden("请登录后操作")
	}
	db := sqls.DB()
	var t models.Ticket
	if err := db.Where("id = ? AND tenant_id = ?", ticketID, op.EffectiveTenantID()).First(&t).Error; err != nil {
		return nil, err
	}
	if err := ticketGovernanceAccess(&t, op); err != nil {
		return nil, err
	}
	p, _, err := loadTicketPriorityPolicyDB(db, &t, false)
	if err != nil {
		return nil, err
	}
	v := &TicketGovernanceView{TicketID: t.ID, Revision: t.GovernanceRevision, CaseType: t.CaseType, Priority: ticketEffectivePriority(t), Suggested: t.PrioritySuggested, Explanation: t.PriorityExplanation, ReviewRequired: false, Overridden: t.PriorityOverridden, Legacy: t.PriorityLevel == "", Policy: p, CanManage: canManageTicketGovernance(&t, op), CanPropose: false, Proposals: []models.TicketPriorityProposal{}, History: []TicketGovernanceHistory{}}
	if err := json.Unmarshal([]byte(t.PriorityFactsJSON), &v.Facts); t.PriorityFactsJSON != "" && err != nil {
		return nil, err
	}
	if err := db.Where("tenant_id = ? AND ticket_id = ?", t.TenantID, t.ID).Order("id DESC").Find(&v.Proposals).Error; err != nil {
		return nil, err
	}
	v.Relations, err = readTicketRelationsDB(db, &t, op)
	if err != nil {
		return nil, err
	}
	v.CanAssociateDuplicate = v.CanManage && t.MergedIntoID == 0 && t.Status != "draft"
	for _, relation := range v.Relations {
		if relation.Kind == "parent" {
			v.CanAssociateDuplicate = false
		}
	}
	v.Merge, err = readTicketMergeDB(db, &t, op)
	if err != nil {
		return nil, err
	}
	var rows []models.TicketProgress
	if err := db.Where("tenant_id = ? AND ticket_id = ? AND event_type IN ?", t.TenantID, t.ID, []string{"ticket_governance", "ticket_created"}).Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		var data map[string]json.RawMessage
		if json.Unmarshal([]byte(row.MetadataJSON), &data) != nil {
			continue
		}
		if row.EventType == "ticket_created" {
			if len(data["governance"]) == 0 {
				continue
			}
			row.MetadataJSON = string(data["governance"])
			if json.Unmarshal([]byte(row.MetadataJSON), &data) != nil {
				continue
			}
		}
		var relatedID int64
		_ = json.Unmarshal(data["related_ticket_id"], &relatedID)
		if relatedID > 0 {
			var related models.Ticket
			if db.Where("id = ? AND tenant_id = ?", relatedID, t.TenantID).First(&related).Error != nil || ticketGovernanceAccess(&related, op) != nil {
				continue
			}
		}
		var action string
		_ = json.Unmarshal(data["action"], &action)
		v.History = append(v.History, TicketGovernanceHistory{row.ID, action, row.Content, row.AuthorID, row.CreatedAt, json.RawMessage(row.MetadataJSON)})
	}
	return v, nil
}
func recordGovernanceDB(db *gorm.DB, t *models.Ticket, op *dto.AuthPrincipal, action, reason string, details map[string]any) error {
	details["action"] = action
	encoded, err := json.Marshal(details)
	if err != nil {
		return err
	}
	return db.Create(&models.TicketProgress{TenantID: t.TenantID, TicketID: t.ID, EventType: "ticket_governance", Content: reason, MetadataJSON: string(encoded), AuthorID: op.UserID, CreatedAt: time.Now()}).Error
}
func ExecuteTicketGovernance(ticketID int64, cmd TicketGovernanceCommand, op *dto.AuthPrincipal) (*TicketGovernanceReceipt, error) {
	if op == nil || op.UserID <= 0 || op.EffectiveTenantID() <= 0 {
		return nil, errorsx.Forbidden("请登录后操作")
	}
	cmd.Reason = strings.TrimSpace(cmd.Reason)
	if cmd.Action == "duplicate_child" && cmd.Reason == "" {
		cmd.Reason = "已确认同一问题，关联到首次工单"
	}
	if cmd.Action == "merge" && cmd.Reason == "" {
		cmd.Reason = "重复工单，合并至主工单"
	}
	if cmd.ExpectedRevision == nil || len(cmd.OperationKey) < 8 || len(cmd.OperationKey) > 100 || cmd.Reason == "" || len([]rune(cmd.Reason)) > 2000 {
		return nil, errorsx.InvalidParam("请提供工单版本、操作编号和具体原因（最多 2000 字）")
	}
	payload, _ := json.Marshal(struct {
		Command TicketGovernanceCommand
		Actor   int64
	}{cmd, op.UserID})
	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])
	var result TicketGovernanceReceipt
	err := sqls.DB().Transaction(func(db *gorm.DB) error {
		if err := repositories.LockTicketGraphDB(db, op.EffectiveTenantID()); err != nil {
			return err
		}
		var t models.Ticket
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", ticketID, op.EffectiveTenantID()).First(&t).Error; err != nil {
			return err
		}
		if err := ticketGovernanceAccess(&t, op); err != nil {
			return err
		}
		var operation models.TicketGovernanceOperation
		e := db.Where("tenant_id = ? AND ticket_id = ? AND operation_key = ?", t.TenantID, t.ID, cmd.OperationKey).First(&operation).Error
		if e == nil {
			if operation.PayloadHash != hash {
				return ErrTicketCaseConflict
			}
			return json.Unmarshal([]byte(operation.ResultJSON), &result)
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if t.GovernanceRevision != *cmd.ExpectedRevision {
			return ErrTicketCaseConflict
		}
		if !canManageTicketGovernance(&t, op) {
			return errorsx.Forbidden("需要工单管理权限才能执行此操作")
		}
		if cmd.Action == "merge" || (cmd.Action == "link" && cmd.RelationKind == "duplicate") {
			return errorsx.InvalidParam("重复问题已改为父子工单，请刷新页面后关联到首次工单")
		}
		if slices.Contains([]string{"assess", "confirm", "propose", "approve", "reject"}, cmd.Action) {
			return errorsx.InvalidParam("评估和建议审批流程已停用，请刷新页面后直接修改紧急程度并填写原因；历史建议不会自动生效")
		}
		if t.MergedIntoID > 0 {
			return errorsx.InvalidParam("该工单已合并，请在主工单继续处理")
		}
		if ticketCaseClosed(t) && cmd.Action != "unlink" && cmd.Action != "duplicate_child" {
			return errorsx.InvalidParam("已结束工单保留历史分类与优先级，请重新打开后操作")
		}
		before := t.TicketGovernance
		oldCode := t.PriorityCode
		oldDeadline := t.SLADueAt
		oldAcceptDeadline := t.AcceptDeadlineAt
		details := map[string]any{"before": before, "previous_sla_deadline": oldDeadline, "previous_accept_deadline": oldAcceptDeadline, "category": cmd.Category}
		if cmd.Action == "link" || cmd.Action == "unlink" || cmd.Action == "duplicate_child" {
			if err := mutateTicketRelationDB(db, &t, cmd, op); err != nil {
				return err
			}
		} else {
			switch cmd.Action {
			case "classify":
				previous := ticketEffectivePriority(t)
				if err := applyTicketClassificationDB(db, &t, cmd.CaseType); err != nil {
					return err
				}
				if t.PriorityOverridden {
					t.PriorityLevel = previous
					t.PriorityCode = oldCode
					t.PriorityExplanation = "人工调整"
				}
			case "override":
				if !ticketpolicy.ValidPriority(cmd.Priority) {
					return errorsx.InvalidParam("请选择 P1–P4")
				}
				t.PriorityLevel = cmd.Priority
				t.PriorityCode = ticketpolicy.LegacyCode(cmd.Priority)
				t.PriorityOverridden = true
				t.PriorityReviewRequired = false
				t.PriorityExplanation = "人工调整"
			default:
				return errorsx.InvalidParam("未知工单分类或等级操作")
			}
			if t.PriorityLevel == ticketpolicy.FromLegacy(oldCode) {
				t.PriorityCode = oldCode
			}
			if t.PriorityCode != oldCode {
				if err := refreshGovernanceDeadlineDB(db, &t); err != nil {
					return err
				}
			} else {
				t.SLADueAt = oldDeadline
				t.AcceptDeadlineAt = oldAcceptDeadline
			}
			t.GovernanceRevision++
			if err := db.Model(&models.Ticket{}).Where("id = ? AND tenant_id = ?", t.ID, t.TenantID).Updates(map[string]any{"case_type": t.CaseType, "priority_level": t.PriorityLevel, "priority_suggested": t.PrioritySuggested, "priority_review_required": t.PriorityReviewRequired, "priority_overridden": t.PriorityOverridden, "priority_facts_json": t.PriorityFactsJSON, "priority_policy_json": t.PriorityPolicyJSON, "priority_explanation": t.PriorityExplanation, "priority_config_version_id": t.PriorityConfigVersionID, "governance_revision": t.GovernanceRevision, "priority_code": t.PriorityCode, "sla_due_at": t.SLADueAt, "accept_deadline_at": t.AcceptDeadlineAt, "updated_at": time.Now(), "update_user_id": op.UserID, "update_user_name": op.Username}).Error; err != nil {
				return err
			}
			details["after"] = t.TicketGovernance
			details["sla_deadline"] = t.SLADueAt
			details["accept_deadline"] = t.AcceptDeadlineAt
			if err := recordGovernanceDB(db, &t, op, cmd.Action, cmd.Reason, details); err != nil {
				return err
			}
		}
		result = TicketGovernanceReceipt{t.ID, t.GovernanceRevision, cmd.OperationKey}
		encoded, _ := json.Marshal(result)
		return db.Create(&models.TicketGovernanceOperation{TenantID: t.TenantID, TicketID: t.ID, OperationKey: cmd.OperationKey, PayloadHash: hash, ResultJSON: string(encoded), CreatedAt: time.Now()}).Error
	})
	return &result, err
}

package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"
)

var ErrTicketCaseConflict = errors.New("工单已被其他操作修改，请刷新；同一操作编号不能用于不同内容")

type TicketCaseCommand struct {
	Action           string `json:"action"`
	Reason           string `json:"reason"`
	ExpectedStatus   string `json:"expected_status"`
	ExpectedRevision *int64 `json:"expected_revision"`
	OwnerID          int64  `json:"owner_id"`
	IdempotencyKey   string `json:"idempotency_key"`
}

func ticketCaseCanEdit(ticket *models.Ticket, operator *dto.AuthPrincipal) bool {
	if ticket == nil || ticket.MergedIntoID > 0 || operator == nil || operator.UserID <= 0 || operator.EffectiveTenantID() != ticket.TenantID || operator.IsCustomer() || operator.IsPartner() || operator.IsServiceAccount() {
		return false
	}
	if !operator.HasPermission(constants.PermissionTicketChangeStatus.Code) {
		return false
	}
	if requireTicketTenantAccess(ticket, operator) != nil {
		return false
	}
	return canManageTicketDispatch(operator) || ticket.CaseOwnerID == operator.UserID || ticket.CurrentAssigneeID == operator.UserID || (!operator.HasRole(EnterpriseRoleEngineer) && operator.HasPermission(constants.PermissionTicketChangeStatus.Code))
}

func ticketCaseActions(ticket *models.Ticket, operator *dto.AuthPrincipal) []string {
	actions := []string{}
	if !ticketCaseCanEdit(ticket, operator) {
		return actions
	}
	state := models.EffectiveTicketCaseStatus(*ticket)
	if state == "cancelled" {
		return actions
	}
	if state == "closed" {
		return append(actions, "reopen")
	}
	if ticket.CaseOwnerID <= 0 {
		return append(actions, "acknowledge", "cancel")
	}
	switch state {
	case "new":
		actions = append(actions, "acknowledge")
	case "acknowledged":
		actions = append(actions, "triage", "wait")
	case "in_triage", "assigned":
		actions = append(actions, "wait", "restore")
		if ticket.ProductID == 0 && ticket.DeviceID == 0 {
			actions = append(actions, "resolve")
		}
	case "waiting":
		actions = append(actions, "resume")
	case "restored":
		actions = append(actions, "triage", "wait")
		if ticket.ProductID == 0 && ticket.DeviceID == 0 {
			actions = append(actions, "resolve")
		}
	case "resolved":
		actions = append(actions, "request_closure", "close", "reopen")
	case "closure_pending":
		actions = append(actions, "close", "reopen")
	}
	if state != "resolved" && state != "closure_pending" {
		actions = append(actions, "cancel")
	}
	return actions
}

func BuildTicketCaseLifecycle(ticket *models.Ticket, operator *dto.AuthPrincipal) dto.TicketCaseLifecycleDTO {
	if ticket == nil {
		return dto.TicketCaseLifecycleDTO{AllowedActions: []string{}}
	}
	result := dto.TicketCaseLifecycleDTO{Status: models.EffectiveTicketCaseStatus(*ticket), Revision: ticket.CaseRevision, OwnerID: ticket.CaseOwnerID, WaitingReason: ticket.WaitingReason, AllowedActions: ticketCaseActions(ticket, operator), LegacyRecord: ticket.CaseStatus == "", CanTransferOwner: ticket.CaseOwnerID > 0 && canManageTicketCaseOwner(ticket, operator)}
	result.WorkflowVersionID = ticket.CaseWorkflowVersionID
	var workflowErr error
	result.AllowedActions, workflowErr = ticketCaseWorkflowActionsDB(sqls.DB(), ticket, operator)
	if workflowErr != nil {
		result.WorkflowError = "工单流程暂时无法读取，请稍后重试或联系管理员"
	}
	if ticket.AcknowledgedAt != nil {
		result.AcknowledgedAt = ticket.AcknowledgedAt.Format(time.RFC3339)
	}
	if ticket.RestoredAt != nil {
		result.RestoredAt = ticket.RestoredAt.Format(time.RFC3339)
	}
	if ticket.CaseOwnerID > 0 {
		if user := repositories.UserRepository.Get(sqls.DB(), ticket.CaseOwnerID); user != nil {
			result.OwnerName = firstNonBlank(user.Nickname, user.Username)
		}
	}
	return result
}

// Confirming reception is distinct from an engineer's AcceptedAt. Legacy accept
// endpoints explicitly perform both actions; no historical records are backfilled.
func ensureTicketCaseAcknowledgedDB(db *gorm.DB, ticket *models.Ticket, operator *dto.AuthPrincipal) error {
	if ticket.CaseOwnerID > 0 {
		return RequireTicketCaseOwnerDB(db, ticket)
	}
	if operator == nil || operator.UserID <= 0 || operator.IsCustomer() || operator.IsPartner() || operator.IsServiceAccount() {
		return errorsx.Forbidden("请由客服确认受理并指定管理负责人")
	}
	return acknowledgeTicketCaseDB(db, ticket, operator.UserID, operator, "受理并接单：由操作人负责跟进")
}

func acknowledgeTicketCaseDB(db *gorm.DB, ticket *models.Ticket, ownerID int64, operator *dto.AuthPrincipal, reason string) error {
	if err := ValidateCaseOwnerDB(db, ticket.TenantID, ownerID); err != nil {
		return err
	}
	now := time.Now()
	previous := models.EffectiveTicketCaseStatus(*ticket)
	next := previous
	if next == "new" {
		next = "acknowledged"
	}
	columns := map[string]any{"case_owner_id": ownerID, "case_status": next, "acknowledged_at": now, "case_revision": gorm.Expr("case_revision + 1"), "updated_at": now, "update_user_id": operator.UserID, "update_user_name": operator.Username}
	if previous == "new" && ticket.CurrentAssigneeID == 0 {
		columns["status"] = enums.TicketStatusAccepted
	}
	if err := repositories.TicketRepository.Updates(db, ticket.ID, columns); err != nil {
		return err
	}
	ticket.CaseOwnerID = ownerID
	ticket.CaseStatus = next
	ticket.CaseRevision++
	ticket.AcknowledgedAt = &now
	if s, ok := columns["status"].(enums.TicketStatus); ok {
		ticket.Status = s
	}
	return recordTicketCaseChangeDB(db, ticket, previous, next, "acknowledge", reason, operator, now)
}

func recordTicketCaseChangeDB(db *gorm.DB, ticket *models.Ticket, from, to, action, reason string, operator *dto.AuthPrincipal, now time.Time) error {
	metadata, _ := json.Marshal(map[string]any{"from_case_status": from, "to_case_status": to, "action": action, "reason": reason, "case_owner_id": ticket.CaseOwnerID})
	if err := db.Create(&models.TicketProgress{TenantID: ticket.TenantID, TicketID: ticket.ID, EventType: "case_status_changed", Content: "工单主状态：" + from + " → " + to + "；" + reason, MetadataJSON: string(metadata), AuthorID: operator.UserID, VisibleToCustomer: false, CreatedAt: now}).Error; err != nil {
		return err
	}
	labels := map[string]string{"acknowledged": "客服已确认受理", "in_triage": "正在分析问题", "waiting": "正在等待补充资料或处理条件", "assigned": "已分配工程师", "restored": "服务已恢复，问题仍在跟进", "resolved": "问题已解决", "closure_pending": "等待确认关闭"}
	if label := labels[to]; label != "" {
		publicMetadata, _ := json.Marshal(map[string]string{"case_status": to})
		return db.Create(&models.TicketProgress{TenantID: ticket.TenantID, TicketID: ticket.ID, EventType: "case_status_changed", Content: label, MetadataJSON: string(publicMetadata), AuthorID: operator.UserID, VisibleToCustomer: true, CreatedAt: now}).Error
	}
	return nil
}

func ExecuteTicketCaseCommand(ticketID int64, cmd TicketCaseCommand, operator *dto.AuthPrincipal) error {
	cmd.Action = strings.TrimSpace(cmd.Action)
	cmd.Reason = strings.TrimSpace(cmd.Reason)
	cmd.IdempotencyKey = strings.TrimSpace(cmd.IdempotencyKey)
	if cmd.ExpectedRevision == nil || !enums.IsValidTicketCaseStatus(cmd.ExpectedStatus) || cmd.IdempotencyKey == "" || utf8.RuneCountInString(cmd.IdempotencyKey) > 100 || utf8.RuneCountInString(cmd.Reason) > 4000 {
		return errorsx.InvalidParam("请提供当前状态、版本和操作编号；说明不得超过4000字")
	}
	if slices.Contains([]string{"wait", "restore", "resolve", "cancel", "reopen"}, cmd.Action) && cmd.Reason == "" {
		return errorsx.InvalidParam("请填写原因或可核对的处理结果")
	}
	switch cmd.Action {
	case "close":
		return TicketLifecycleService.close(ticketID, cmd.Reason, operator, &cmd)
	case "cancel":
		return TicketLifecycleService.cancel(ticketID, cmd.Reason, operator, &cmd)
	case "reopen":
		return TicketLifecycleService.reopen(ticketID, cmd.Reason, operator, &cmd)
	}
	return sqls.DB().Transaction(func(tx *gorm.DB) error {
		ticket := loadTicketForUpdate(tx, ticketID)
		if replay, err := checkTicketCaseCommandDB(tx, ticket, &cmd, operator); err != nil || replay {
			return err
		}
		if cmd.Action == "acknowledge" {
			ownerID := cmd.OwnerID
			if ownerID <= 0 {
				ownerID = operator.UserID
			}
			if ticket.CaseOwnerID > 0 {
				return errorsx.InvalidParam("已受理，请使用负责人交接操作")
			}
			if err := acknowledgeTicketCaseDB(tx, ticket, ownerID, operator, cmd.Reason); err != nil {
				return err
			}
		} else {
			if err := RequireTicketCaseOwnerDB(tx, ticket); err != nil {
				return err
			}
			from := models.EffectiveTicketCaseStatus(*ticket)
			next := from
			now := time.Now()
			columns := map[string]any{"case_revision": gorm.Expr("case_revision + 1"), "updated_at": now, "update_user_id": operator.UserID, "update_user_name": operator.Username}
			switch cmd.Action {
			case "triage":
				next = "in_triage"
				columns["status"] = ticketCaseActiveTechnicalStatus(ticket)
			case "wait":
				next = "waiting"
				columns["waiting_reason"] = cmd.Reason
				columns["case_resume_status"] = from
				columns["case_resume_technical_status"] = ticket.Status
				columns["status"] = enums.TicketStatusWaitingCustomer
			case "resume":
				next = ticket.CaseResumeStatus
				if !slices.Contains([]string{"acknowledged", "in_triage", "assigned", "restored"}, next) {
					next = "in_triage"
				}
				columns["waiting_reason"] = ""
				columns["case_resume_status"] = ""
				columns["case_resume_technical_status"] = ""
				columns["status"] = ticketCaseActiveTechnicalStatus(ticket)
			case "restore":
				next = "restored"
				columns["restored_at"] = now
			case "resolve":
				if err := validateRepairCompletionReadiness(tx, ticket); err != nil {
					return err
				}
				if ticket.ProductID > 0 || ticket.DeviceID > 0 {
					return errorsx.InvalidParam("设备售后工单请提交处理记录确认解决")
				}
				next = "resolved"
				columns["resolved_at"] = now
				columns["case_resolution"] = cmd.Reason
				columns["status"] = enums.TicketStatusResolved
			case "request_closure":
				if ticket.ResolvedAt == nil {
					return errorsx.InvalidParam("尚无已解决记录，不能进入待关闭")
				}
				next = "closure_pending"
				columns["status"] = enums.TicketStatusPendingCustomerConfirm
			default:
				return errorsx.InvalidParam("不支持的工单操作")
			}
			columns["case_status"] = next
			if err := repositories.TicketRepository.Updates(tx, ticket.ID, columns); err != nil {
				return err
			}
			if err := recordTicketCaseChangeDB(tx, ticket, from, next, cmd.Action, cmd.Reason, operator, now); err != nil {
				return err
			}
			ticket.CaseStatus = next
			ticket.CaseRevision++
		}
		return recordTicketCaseCommandDB(tx, ticket.ID, &cmd, operator)
	})
}

func ticketCaseCommandHash(cmd *TicketCaseCommand, operator *dto.AuthPrincipal) string {
	data, _ := json.Marshal(struct {
		Command *TicketCaseCommand
		ActorID int64
	}{cmd, operatorID(operator)})
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Called with the ticket locked, before any mutation or terminal no-op checks.
// Terminal operations share the same receipt transaction as normal transitions.
func checkTicketCaseCommandDB(tx *gorm.DB, ticket *models.Ticket, cmd *TicketCaseCommand, operator *dto.AuthPrincipal) (bool, error) {
	if cmd == nil {
		return false, nil
	}
	if !ticketCaseCanEdit(ticket, operator) {
		return false, errorsx.Forbidden("无权处理这张工单")
	}
	var operation models.TicketCaseOperation
	err := tx.Where("tenant_id = ? AND ticket_id = ? AND operation_key = ?", ticket.TenantID, ticket.ID, cmd.IdempotencyKey).Take(&operation).Error
	if err == nil {
		if operation.PayloadHash != ticketCaseCommandHash(cmd, operator) {
			return false, ErrTicketCaseConflict
		}
		return true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	if cmd.ExpectedRevision == nil || ticket.CaseRevision != *cmd.ExpectedRevision || models.EffectiveTicketCaseStatus(*ticket) != cmd.ExpectedStatus {
		return false, ErrTicketCaseConflict
	}
	actions, err := ticketCaseWorkflowActionsDB(tx, ticket, operator)
	if err != nil {
		return false, err
	}
	if !slices.Contains(actions, cmd.Action) {
		return false, errorsx.InvalidParam("当前状态不允许此操作")
	}
	return false, nil
}

func recordTicketCaseCommandDB(tx *gorm.DB, ticketID int64, cmd *TicketCaseCommand, operator *dto.AuthPrincipal) error {
	if cmd == nil {
		return nil
	}
	var ticket models.Ticket
	if err := tx.First(&ticket, ticketID).Error; err != nil {
		return err
	}
	return tx.Create(&models.TicketCaseOperation{TenantID: ticket.TenantID, TicketID: ticket.ID, OperationKey: cmd.IdempotencyKey, PayloadHash: ticketCaseCommandHash(cmd, operator), ResultStatus: models.EffectiveTicketCaseStatus(ticket), ResultRevision: ticket.CaseRevision, CreatedAt: time.Now()}).Error
}

func ticketCaseActiveTechnicalStatus(ticket *models.Ticket) enums.TicketStatus {
	status := ticket.Status
	if status == enums.TicketStatusWaitingCustomer {
		status = enums.TicketStatus(ticket.CaseResumeTechnicalStatus)
	}
	// Waiting/triage are case actions, not the end of an engineer's active
	// video or supplier collaboration. A technical change during the wait wins
	// over the saved step, so reassignment or a completed meeting stays current.
	switch status {
	case enums.TicketStatusVideoSupport, enums.TicketStatusSupplierSupport:
		return status
	}
	if ticket.CurrentAssigneeID > 0 && ticket.AcceptedAt == nil {
		return enums.TicketStatusPendingAssigneeAccept
	}
	if ticket.CurrentAssigneeID == 0 && (status == enums.TicketStatusAccepted || status == enums.TicketStatusPendingDispatch || status == enums.TicketStatusReopened) {
		return status
	}
	return enums.TicketStatusProcessing
}

func operatorID(operator *dto.AuthPrincipal) int64 {
	if operator == nil {
		return 0
	}
	return operator.UserID
}

// Guard the old endpoints as well: they cannot bypass ownership or turn a
// restored/waiting case into a closed case without a recorded resolution.
func requireTicketCaseCloseDB(db *gorm.DB, ticket *models.Ticket) error {
	if ticket.CaseStatus == "" {
		return nil
	}
	if !slices.Contains([]string{"resolved", "closure_pending"}, ticket.CaseStatus) || ticket.ResolvedAt == nil {
		return errorsx.InvalidParam("请先确认问题已解决，再关闭工单；不再处理的问题请取消")
	}
	return RequireTicketCaseOwnerDB(db, ticket)
}

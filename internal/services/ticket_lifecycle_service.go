package services

import (
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var TicketLifecycleService = newTicketLifecycleService()

func newTicketLifecycleService() *ticketLifecycleService {
	return &ticketLifecycleService{}
}

type ticketLifecycleService struct{}

type ticketTakeoverKind string

const (
	ticketTakeoverExpiredAcceptance  ticketTakeoverKind = "expired_acceptance"
	ticketTakeoverUnansweredCustomer ticketTakeoverKind = "unanswered_customer"
	ticketTakeoverDispatchPool       ticketTakeoverKind = "dispatch_pool"
	ticketTakeoverTeamReassignment   ticketTakeoverKind = "team_reassignment"
)

func loadTicketForUpdate(db *gorm.DB, ticketID int64) *models.Ticket {
	if db == nil || ticketID <= 0 {
		return nil
	}
	ticket := &models.Ticket{}
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(ticket, ticketID).Error; err != nil {
		return nil
	}
	return ticket
}

// Accept 受理工单
func (s *ticketLifecycleService) Accept(ticketID int64, assigneeID int64, operator *dto.AuthPrincipal) error {
	return s.accept(ticketID, assigneeID, operator, ticketAcceptOptions{})
}

func (s *ticketLifecycleService) AcceptWithAudit(ticketID int64, assigneeID int64, operator *dto.AuthPrincipal, auditInput RecordAuditInput) error {
	return s.accept(ticketID, assigneeID, operator, ticketAcceptOptions{AuditInput: &auditInput})
}

func (s *ticketLifecycleService) Takeover(ticketID int64, operator *dto.AuthPrincipal) error {
	return s.accept(ticketID, 0, operator, ticketAcceptOptions{ForceTakeover: true})
}

func (s *ticketLifecycleService) TakeoverWithAudit(ticketID int64, operator *dto.AuthPrincipal, auditInput RecordAuditInput) error {
	return s.accept(ticketID, 0, operator, ticketAcceptOptions{AuditInput: &auditInput, ForceTakeover: true})
}

type ticketAcceptOptions struct {
	AuditInput    *RecordAuditInput
	ForceTakeover bool
}

func (s *ticketLifecycleService) accept(ticketID int64, assigneeID int64, operator *dto.AuthPrincipal, options ticketAcceptOptions) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	var conversationID int64
	var acceptedAssigneeID int64
	var previousAssigneeID int64
	var takeoverKind ticketTakeoverKind
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		ticket := loadTicketForUpdate(ctx.Tx, ticketID)
		if ticket == nil {
			return errorsx.InvalidParamI18n("error.e0178")
		}
		if err := requireTicketTenantAccess(ticket, operator); err != nil {
			return err
		}
		knowledgeSupport := TenantCapabilityService.KnowledgeSupportDB(ctx.Tx, ticket.TenantID)
		if assigneeID > 0 && assigneeID != operator.UserID {
			return errorsx.Forbidden("operators can only accept tickets for themselves")
		}
		if isTicketAlreadyAcceptedByAssignee(ticket, operator.UserID) {
			return nil
		}
		now := time.Now()
		takingOver := ticket.CurrentAssigneeID > 0 && ticket.CurrentAssigneeID != operator.UserID
		targetStatus, ok := ticketAcceptTargetStatus(ticket)
		if !ok {
			return errorsx.InvalidParamI18n("error.e0182")
		}
		effectiveAssigneeID := assigneeID
		if effectiveAssigneeID <= 0 {
			if ticket.CurrentAssigneeID > 0 && !takingOver {
				effectiveAssigneeID = ticket.CurrentAssigneeID
			} else {
				effectiveAssigneeID = operator.UserID
			}
		}
		if effectiveAssigneeID != operator.UserID {
			return errorsx.Forbidden("operators can only accept tickets for themselves")
		}
		effectiveTeamID := ticket.CurrentTeamID
		if effectiveTeamID <= 0 && ticket.ProductID > 0 {
			if team := ProductSupportOrganizationService.FindProductRepairTeam(ctx.Tx, ticket.TenantID, ticket.ProductID); team != nil {
				effectiveTeamID = team.ID
			}
		}
		if effectiveAssigneeID > 0 {
			assignee := repositories.UserRepository.Get(ctx.Tx, effectiveAssigneeID)
			if assignee == nil || assignee.Status != enums.StatusOk {
				return errorsx.InvalidParamI18n("error.e0334")
			}
			if options.ForceTakeover {
				validatedTeamID, validatedKind, err := validateTicketTakeoverDB(ctx.Tx, ticket, effectiveAssigneeID, now)
				if err != nil {
					return err
				}
				effectiveTeamID = validatedTeamID
				takeoverKind = validatedKind
			} else if canManageTicketDispatch(operator) {
				if effectiveTeamID <= 0 && ticket.ProductID > 0 {
					if team := ProductSupportOrganizationService.FindProductRepairTeam(ctx.Tx, ticket.TenantID, ticket.ProductID); team != nil {
						effectiveTeamID = team.ID
					}
				}
			} else if takingOver {
				validatedTeamID, validatedKind, err := validateTicketTakeoverDB(ctx.Tx, ticket, effectiveAssigneeID, now)
				if err != nil {
					return err
				}
				effectiveTeamID = validatedTeamID
				takeoverKind = validatedKind
			} else if ticket.TenantID > 0 && ticket.CurrentAssigneeID <= 0 {
				assignmentTicket := *ticket
				assignmentTicket.CurrentTeamID = effectiveTeamID
				_, _, validatedTeamID, err := validateManualTicketSelfClaimWithOptionsDB(
					ctx.Tx,
					&assignmentTicket,
					effectiveAssigneeID,
					now,
					manualTicketAssigneeOptions{
						AllowNoActiveSchedule: true,
						AllowRequestPresence:  true,
					},
				)
				if err != nil {
					return err
				}
				effectiveTeamID = validatedTeamID
			} else if err := validateAssignedTicketAcceptanceDB(ctx.Tx, ticket, effectiveAssigneeID, effectiveTeamID, now); err != nil {
				return err
			}
		}
		updates := map[string]any{
			"status":                       targetStatus,
			"current_assignee_id":          effectiveAssigneeID,
			"accepted_at":                  now,
			"accept_deadline_at":           nil,
			"dispatch_deferred_until":      nil,
			"last_dispatch_failure_reason": "",
			"updated_at":                   now,
			"update_user_id":               operator.UserID,
			"update_user_name":             operator.Username,
		}
		if effectiveTeamID > 0 && ticket.CurrentTeamID != effectiveTeamID {
			updates["current_team_id"] = effectiveTeamID
		}
		if ticket.AssignedAt == nil || takingOver || options.ForceTakeover {
			updates["assigned_at"] = now
		}
		if takingOver && takeoverKind == ticketTakeoverExpiredAcceptance {
			updates["dispatch_attempts"] = gorm.Expr("dispatch_attempts + ?", 1)
		}
		if err := repositories.TicketRepository.Updates(ctx.Tx, ticket.ID, updates); err != nil {
			return err
		}
		if takingOver {
			reason := "同产品维修组成员接管已分配工单"
			if knowledgeSupport {
				reason = "技术支持组成员接管已分配工单"
			}
			switch takeoverKind {
			case ticketTakeoverExpiredAcceptance:
				if err := finishPendingTicketDispatchAttemptTx(ctx.Tx, ticket.ID, ticketDispatchOutcomeTimedOut, nil, now); err != nil {
					return err
				}
				reason = "原处理人接单超时，产品组成员接管"
				if knowledgeSupport {
					reason = "原处理人接单超时，技术支持组成员接管"
				}
			case ticketTakeoverUnansweredCustomer:
				reason = "原处理人长期未回复客户，产品组成员接管"
				if knowledgeSupport {
					reason = "原处理人长期未回复客户，技术支持组成员接管"
				}
			}
			if err := createFinishedTicketDispatchAttemptTx(ctx.Tx, ticket, effectiveTeamID, effectiveAssigneeID, ticketDispatchOutcomeAccepted, reason, operator, now); err != nil {
				return err
			}
		} else if options.ForceTakeover {
			reason := "产品组成员从待派单池转派给自己"
			if knowledgeSupport {
				reason = "技术支持组成员从待派单池转派给自己"
			}
			if err := createFinishedTicketDispatchAttemptTx(ctx.Tx, ticket, effectiveTeamID, effectiveAssigneeID, ticketDispatchOutcomeAccepted, reason, operator, now); err != nil {
				return err
			}
		} else if err := finishPendingTicketDispatchAttemptTx(ctx.Tx, ticket.ID, ticketDispatchOutcomeAccepted, &now, now); err != nil {
			return err
		}
		activationTicket := *ticket
		activationTicket.Status = targetStatus
		activationTicket.CurrentTeamID = effectiveTeamID
		activationTicket.CurrentAssigneeID = effectiveAssigneeID
		if err := s.activateLinkedConversation(ctx, &activationTicket, effectiveAssigneeID, operator, now, takeoverKind); err != nil {
			return err
		}
		conversationID = ticket.ConversationID
		acceptedAssigneeID = effectiveAssigneeID
		content := "受理工单"
		if takingOver {
			previousAssigneeID = ticket.CurrentAssigneeID
			content = "同产品维修组成员接管并受理"
			if knowledgeSupport {
				content = "技术支持组成员接管并受理"
			}
			switch takeoverKind {
			case ticketTakeoverExpiredAcceptance:
				content = "原处理人接单超时，产品组成员接管并受理"
				if knowledgeSupport {
					content = "原处理人接单超时，技术支持组成员接管并受理"
				}
			case ticketTakeoverUnansweredCustomer:
				content = "原处理人长期未回复客户，产品组成员接管并受理"
				if knowledgeSupport {
					content = "原处理人长期未回复客户，技术支持组成员接管并受理"
				}
			}
		} else if options.ForceTakeover {
			content = "产品组成员从待派单池转派给自己并受理"
			if knowledgeSupport {
				content = "技术支持组成员从待派单池转派给自己并受理"
			}
		}
		if err := s.addProgress(ctx.Tx, ticket.ID, enums.TicketProgressEventAccepted, content, operator); err != nil {
			return err
		}
		if options.AuditInput != nil {
			input := *options.AuditInput
			input.TenantID = ticket.TenantID
			input.ActorID = strconv.FormatInt(operator.UserID, 10)
			input.ActorType = "user"
			input.Domain = "ticket"
			input.ResourceType = "ticket"
			input.ResourceID = strconv.FormatInt(ticket.ID, 10)
			input.Action = "ticket.accepted"
			input.RiskLevel = models.RiskLevelLow
			input.BeforeState = map[string]any{
				"status":           ticket.Status,
				"assigneeId":       ticket.CurrentAssigneeID,
				"acceptDeadlineAt": ticket.AcceptDeadlineAt,
			}
			afterState := map[string]any{
				"status":     targetStatus,
				"assigneeId": effectiveAssigneeID,
				"acceptedAt": now,
			}
			if takingOver || options.ForceTakeover {
				input.Action = "ticket.taken_over"
				input.RiskLevel = models.RiskLevelMedium
				afterState["takeoverKind"] = takeoverKind
			}
			input.AfterState = afterState
			if err := AuditService.RecordAuditTx(ctx, input); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if acceptedAssigneeID > 0 {
		if ticket := TicketService.Get(ticketID); ticket != nil {
			if previousAssigneeID > 0 {
				if err := NotificationService.ReconcileTicketAssignedAfterReassignment(ticket, previousAssigneeID, acceptedAssigneeID); err != nil {
					slog.Warn("reconcile previous ticket assignment notification after takeover failed", "ticketId", ticketID, "fromAssigneeId", previousAssigneeID, "toAssigneeId", acceptedAssigneeID, "error", err)
				}
			}
			if err := NotificationService.ReconcileTicketAssignedAfterAcceptance(ticket, acceptedAssigneeID); err != nil {
				slog.Warn("reconcile ticket assignment notification after acceptance failed", "ticketId", ticketID, "assigneeId", acceptedAssigneeID, "error", err)
			}
		}
	}
	if conversationID > 0 {
		if conversation := ConversationService.Get(conversationID); conversation != nil {
			WsService.PublishConversationChanged(conversation, enums.IMRealtimeEventConversationAssigned)
		}
	}
	return nil
}

func isTicketAlreadyAcceptedByAssignee(ticket *models.Ticket, assigneeID int64) bool {
	if ticket == nil || assigneeID <= 0 || ticket.CurrentAssigneeID != assigneeID || ticket.AcceptedAt == nil {
		return false
	}
	switch enums.NormalizeTicketStatus(string(ticket.Status)) {
	case enums.TicketStatusAccepted, enums.TicketStatusProcessing:
		return true
	default:
		return false
	}
}

func validateTicketTakeoverDB(db *gorm.DB, ticket *models.Ticket, assigneeID int64, now time.Time) (int64, ticketTakeoverKind, error) {
	if ticket == nil || assigneeID <= 0 {
		return 0, "", errorsx.Forbidden("only an eligible product repair engineer can take over the ticket")
	}
	status := enums.NormalizeTicketStatus(string(ticket.Status))
	kind := ticketTakeoverKind("")
	switch status {
	case enums.TicketStatusPendingDispatch:
		if ticket.CurrentAssigneeID > 0 && ticket.CurrentAssigneeID != assigneeID {
			kind = ticketTakeoverTeamReassignment
		} else if ticket.CurrentAssigneeID == assigneeID {
			return 0, "", errorsx.Forbidden("the ticket is already assigned to the current engineer")
		} else {
			kind = ticketTakeoverDispatchPool
		}
	case enums.TicketStatusPendingAssigneeAccept, enums.TicketStatusAssigned:
		if ticket.CurrentAssigneeID <= 0 || ticket.CurrentAssigneeID == assigneeID {
			return 0, "", errorsx.Forbidden("only another assigned engineer's overdue ticket can be taken over")
		}
		kind = ticketTakeoverTeamReassignment
		if deadline, ok := ticketEffectiveAcceptanceDeadlineDB(db, ticket); ok && !deadline.After(now) {
			kind = ticketTakeoverExpiredAcceptance
		}
	case enums.TicketStatusAccepted, enums.TicketStatusProcessing, enums.TicketStatusInProgress, enums.TicketStatusVideoSupport, enums.TicketStatusSupplierSupport, enums.TicketStatusWaitingCustomer:
		if ticket.CurrentAssigneeID <= 0 || ticket.CurrentAssigneeID == assigneeID {
			return 0, "", errorsx.Forbidden("only another assigned engineer's overdue ticket can be taken over")
		}
		kind = ticketTakeoverTeamReassignment
		if _, overdue := ticketOwnerReplyTakeoverDeadlineDB(db, ticket, now); overdue {
			kind = ticketTakeoverUnansweredCustomer
		}
	default:
		return 0, "", errorsx.InvalidParam("ticket is not eligible for team takeover")
	}
	teamID := ticket.CurrentTeamID
	if teamID <= 0 && ticket.ProductID > 0 {
		if team := ProductSupportOrganizationService.FindProductRepairTeam(db, ticket.TenantID, ticket.ProductID); team != nil {
			teamID = team.ID
		}
	}
	assignmentTicket := *ticket
	assignmentTicket.CurrentTeamID = teamID
	assignmentTicket.CurrentAssigneeID = 0
	_, _, validatedTeamID, err := validateManualTicketAssigneeForTakeoverDB(db, &assignmentTicket, assigneeID, now)
	if err != nil {
		return 0, "", err
	}
	return validatedTeamID, kind, nil
}

func ticketEffectiveAcceptanceDeadlineDB(db *gorm.DB, ticket *models.Ticket) (time.Time, bool) {
	if ticket == nil {
		return time.Time{}, false
	}
	if ticket.AcceptDeadlineAt != nil && !ticket.AcceptDeadlineAt.IsZero() {
		return *ticket.AcceptDeadlineAt, true
	}
	assignedAt := ticket.CreatedAt
	if ticket.AssignedAt != nil && !ticket.AssignedAt.IsZero() {
		assignedAt = *ticket.AssignedAt
	} else if !ticket.UpdatedAt.IsZero() && ticket.UpdatedAt.After(assignedAt) {
		// Legacy assigned tickets may predate assignment tracking. UpdatedAt is
		// the conservative latest bound for deriving their acceptance timeout.
		assignedAt = ticket.UpdatedAt
	}
	if assignedAt.IsZero() {
		return time.Time{}, false
	}
	deadline := assignedAt.Add(time.Duration(acceptDeadlineMinutes()) * time.Minute)
	if slaDeadline, ok := ticketAssignmentSLADeadlineDB(db, ticket); ok && slaDeadline.Before(deadline) {
		deadline = slaDeadline
	}
	return deadline, true
}

func ticketOwnerReplyTakeoverDeadlineDB(db *gorm.DB, ticket *models.Ticket, now time.Time) (time.Time, bool) {
	if db == nil || ticket == nil || ticket.ConversationID <= 0 {
		return time.Time{}, false
	}
	latestCustomer := repositories.MessageRepository.FindLastValidByConversationIDAndSenderType(db, ticket.ConversationID, enums.IMSenderTypeCustomer)
	if latestCustomer == nil {
		return time.Time{}, false
	}
	customerAt := messageActivityTime(latestCustomer)
	if customerAt.IsZero() {
		return time.Time{}, false
	}
	if latestAgent := repositories.MessageRepository.FindLastValidByConversationIDAndSenderType(db, ticket.ConversationID, enums.IMSenderTypeAgent); latestAgent != nil {
		agentAt := messageActivityTime(latestAgent)
		if !agentAt.IsZero() && !agentAt.Before(customerAt) {
			return time.Time{}, false
		}
	}
	waitStartedAt := customerAt
	if ticket.AcceptedAt != nil && ticket.AcceptedAt.After(waitStartedAt) {
		waitStartedAt = *ticket.AcceptedAt
	} else if ticket.AcceptedAt == nil && ticket.AssignedAt != nil && ticket.AssignedAt.After(waitStartedAt) {
		waitStartedAt = *ticket.AssignedAt
	} else if ticket.AcceptedAt == nil && ticket.AssignedAt == nil && ticket.CreatedAt.After(waitStartedAt) {
		waitStartedAt = ticket.CreatedAt
	}
	deadline := waitStartedAt.Add(time.Duration(ticketOwnerReplyTakeoverMinutesDB(db, ticket)) * time.Minute)
	return deadline, !deadline.After(now)
}

func ticketOwnerReplyTakeoverMinutesDB(db *gorm.DB, ticket *models.Ticket) int {
	if db != nil && ticket != nil && ticket.TenantID > 0 && db.Migrator().HasTable(&SLAPolicy{}) {
		var policy SLAPolicy
		if err := db.Where("tenant_id = ? AND priority = ? AND status = ? AND frt_minutes > ?",
			formatID(ticket.TenantID), ticketSLAPriority(*ticket), "active", 0).
			Order("updated_at DESC").
			First(&policy).Error; err == nil {
			return policy.FRTMinutes
		}
	}
	return config.CurrentOrDefault().TicketDispatch.Normalized().OwnerReplyTakeoverMinutes
}

func messageActivityTime(message *models.Message) time.Time {
	if message == nil {
		return time.Time{}
	}
	if message.SentAt != nil && !message.SentAt.IsZero() {
		return *message.SentAt
	}
	return message.CreatedAt
}

func ticketAcceptTargetStatus(ticket *models.Ticket) (enums.TicketStatus, bool) {
	if ticket == nil {
		return "", false
	}
	status := enums.NormalizeTicketStatus(string(ticket.Status))
	switch status {
	case enums.TicketStatusPendingAssigneeAccept:
		return enums.TicketStatusProcessing, enums.IsValidAfterSalesTransition(string(ticket.Status), string(enums.TicketStatusProcessing))
	case enums.TicketStatusPendingDispatch:
		// 待派单池里的自领接单是“认领 + 进入处理”的复合动作，避免工程师看到接单入口却被状态机挡住。
		return enums.TicketStatusProcessing, true
	case enums.TicketStatusAssigned:
		if ticket.CurrentAssigneeID > 0 {
			return enums.TicketStatusProcessing, true
		}
		return enums.TicketStatusAccepted, true
	case enums.TicketStatusAccepted, enums.TicketStatusProcessing:
		if ticket.CurrentAssigneeID > 0 && (ticket.AcceptedAt == nil || status == enums.TicketStatusProcessing) {
			return status, true
		}
		return "", false
	case enums.TicketStatusInProgress, enums.TicketStatusVideoSupport, enums.TicketStatusSupplierSupport, enums.TicketStatusWaitingCustomer:
		if ticket.CurrentAssigneeID > 0 {
			return status, true
		}
		return "", false
	default:
		if enums.IsValidTicketStatusTransition(string(ticket.Status), string(enums.TicketStatusAccepted)) ||
			enums.IsValidAfterSalesTransition(string(ticket.Status), string(enums.TicketStatusAccepted)) {
			return enums.TicketStatusAccepted, true
		}
		return "", false
	}
}

func (s *ticketLifecycleService) activateLinkedConversation(ctx *sqls.TxContext, ticket *models.Ticket, assigneeID int64, operator *dto.AuthPrincipal, now time.Time, takeoverKind ticketTakeoverKind) error {
	if ctx == nil || ticket == nil || ticket.ConversationID <= 0 || assigneeID <= 0 {
		return nil
	}
	conversation := repositories.ConversationRepository.Get(ctx.Tx, ticket.ConversationID)
	if conversation == nil {
		return nil
	}
	if ticket.TenantID > 0 && conversation.TenantID != ticket.TenantID {
		return errorsx.Forbidden("ticket conversation tenant mismatch")
	}
	if conversation.Status == enums.IMConversationStatusActive && conversation.CurrentAssigneeID == assigneeID && conversation.CurrentTeamID == ticket.CurrentTeamID {
		return nil
	}
	if err := ConversationAssignmentService.FinishActiveAssignments(ctx, conversation.ID, now); err != nil {
		return err
	}
	assignmentType := enums.IMAssignmentTypeAssign
	assignmentReason := "受理关联工单"
	eventType := enums.IMEventTypeAssign
	eventContent := "关联工单受理，会话已分配"
	if takeoverKind != "" {
		assignmentType = enums.IMAssignmentTypeTransfer
		assignmentReason = "产品维修组成员接管工单"
		eventType = enums.IMEventTypeTransfer
		eventContent = "关联工单已由产品维修组成员接管，会话已转接"
		if TenantCapabilityService.KnowledgeSupportDB(ctx.Tx, ticket.TenantID) {
			assignmentReason = "技术支持组成员接管工单"
			eventContent = "关联工单已由技术支持组成员接管，会话已转接"
		}
		if takeoverKind == ticketTakeoverExpiredAcceptance {
			assignmentReason = "工单超时接管"
			eventContent = "关联工单超时接管，会话已转接"
		} else if takeoverKind == ticketTakeoverUnansweredCustomer {
			assignmentReason = "客户消息久未回复，工单接管"
			eventContent = "客户消息久未回复，关联工单已接管并转接会话"
		}
	}
	if _, err := ConversationAssignmentService.CreateAssignment(ctx, conversation.ID, conversation.CurrentAssigneeID, assigneeID, assignmentType, assignmentReason, operator, now); err != nil {
		return err
	}
	if err := ConversationService.ensureAgentParticipantTx(ctx.Tx, conversation.ID, assigneeID, now, operator); err != nil {
		return err
	}
	if err := repositories.ConversationRepository.Updates(ctx.Tx, conversation.ID, map[string]any{
		"status":              enums.IMConversationStatusActive,
		"current_team_id":     ticket.CurrentTeamID,
		"current_assignee_id": assigneeID,
		"closed_at":           nil,
		"closed_by":           0,
		"close_reason":        "",
		"updated_at":          now,
		"update_user_id":      operator.UserID,
		"update_user_name":    operator.Username,
	}); err != nil {
		return err
	}
	return ConversationEventLogService.CreateEvent(ctx, conversation.ID, eventType, enums.IMSenderTypeAgent, operator.UserID, eventContent, ConversationService.buildEventPayload(map[string]any{
		"fromStatus":     conversation.Status,
		"toStatus":       enums.IMConversationStatusActive,
		"fromAssigneeId": conversation.CurrentAssigneeID,
		"toAssigneeId":   assigneeID,
		"ticketId":       ticket.ID,
	}))
}

func canManageTicketDispatch(operator *dto.AuthPrincipal) bool {
	if operator == nil {
		return false
	}
	return operator.IsPlatform() || operator.HasRole(EnterpriseRoleOwner) || operator.HasRole(EnterpriseRoleAdmin) || operator.HasRole(EnterpriseRoleServiceManager)
}

func requireManualTicketAssignmentAuthority(ticket *models.Ticket, operator *dto.AuthPrincipal) error {
	if ticket == nil || operator == nil {
		return errorsx.ForbiddenI18n("error.e0225")
	}
	if canManageTicketDispatch(operator) {
		return nil
	}
	if ticket.CurrentAssigneeID > 0 && ticket.CurrentAssigneeID == operator.UserID {
		return nil
	}
	return errorsx.Forbidden("only dispatch managers or the current assignee can assign tickets")
}

func validateManualTicketAssigneeDB(db *gorm.DB, ticket *models.Ticket, assigneeID int64, now time.Time) (*models.User, *models.AgentProfile, int64, error) {
	return validateManualTicketAssigneeWithOptionsDB(db, ticket, assigneeID, now, manualTicketAssigneeOptions{AllowNoActiveSchedule: true})
}

func validateManualTicketSelfClaimDB(db *gorm.DB, ticket *models.Ticket, assigneeID int64, now time.Time) (*models.User, *models.AgentProfile, int64, error) {
	return validateManualTicketAssigneeWithOptionsDB(db, ticket, assigneeID, now, manualTicketAssigneeOptions{AllowNoActiveSchedule: true, AllowRequestPresence: true})
}

func validateManualTicketSelfClaimWithOptionsDB(db *gorm.DB, ticket *models.Ticket, assigneeID int64, now time.Time, options manualTicketAssigneeOptions) (*models.User, *models.AgentProfile, int64, error) {
	return validateManualTicketAssigneeWithOptionsDB(db, ticket, assigneeID, now, options)
}

func validateManualTicketAssigneeForTakeoverDB(db *gorm.DB, ticket *models.Ticket, assigneeID int64, now time.Time) (*models.User, *models.AgentProfile, int64, error) {
	return validateManualTicketAssigneeWithOptionsDB(db, ticket, assigneeID, now, manualTicketAssigneeOptions{AllowNoActiveSchedule: true, AllowRequestPresence: true})
}

type manualTicketAssigneeOptions struct {
	AllowNoActiveSchedule bool
	AllowRequestPresence  bool
}

func validateManualTicketAssigneeWithOptionsDB(db *gorm.DB, ticket *models.Ticket, assigneeID int64, now time.Time, options manualTicketAssigneeOptions) (*models.User, *models.AgentProfile, int64, error) {
	if assigneeID <= 0 {
		return nil, nil, 0, errorsx.InvalidParamI18n("error.e0334")
	}
	user := repositories.UserRepository.Get(db, assigneeID)
	if user == nil || user.Status != enums.StatusOk {
		return nil, nil, 0, errorsx.InvalidParamI18n("error.e0334")
	}
	teamID := int64(0)
	if ticket != nil {
		teamID = ticket.CurrentTeamID
	}
	if ticket == nil || ticket.TenantID <= 0 {
		return user, nil, teamID, nil
	}
	hasAgentTeamTable := db != nil && db.Migrator().HasTable(&models.AgentTeam{})
	if ticket.ProductID > 0 && teamID <= 0 && hasAgentTeamTable {
		team := ProductSupportOrganizationService.FindProductRepairTeam(db, ticket.TenantID, ticket.ProductID)
		if team == nil || team.Status != enums.StatusOk {
			return nil, nil, 0, errorsx.InvalidParam("ticket product repair team is required before dispatch")
		}
		teamID = team.ID
	}
	if db == nil || !db.Migrator().HasTable(&models.AgentProfile{}) {
		return user, nil, teamID, nil
	}
	profile := repositories.AgentProfileRepository.FindActiveByUserForUpdate(db, ticket.TenantID, assigneeID)
	if profile == nil {
		return nil, nil, 0, errorsx.InvalidParam("assignee must have an active engineer profile")
	}
	if teamID <= 0 {
		teamIDs := AgentTeamMemberService.FindTeamIDsByUserID(db, ticket.TenantID, assigneeID)
		if len(teamIDs) > 0 {
			teamID = teamIDs[0]
		}
	}
	if teamID <= 0 {
		return nil, nil, 0, errorsx.InvalidParam("assignee has no product repair team")
	}
	if hasAgentTeamTable {
		team := repositories.AgentTeamRepository.Get(db, teamID)
		if team == nil || team.TenantID != ticket.TenantID || team.Status != enums.StatusOk {
			return nil, nil, 0, errorsx.InvalidParam("ticket product repair team is not active")
		}
		if ticket.ProductID > 0 && (team.TeamType != AgentTeamTypeProductRepair || team.ProductID != ticket.ProductID) {
			return nil, nil, 0, errorsx.InvalidParam("assignee must belong to the ticket product repair team")
		}
		if err := validateTeamRosterScheduleDB(db, team, assigneeID, now, options.AllowNoActiveSchedule); err != nil {
			return nil, nil, 0, err
		}
	}
	if !AgentTeamMemberService.IsUserActiveMemberOfTeamDB(db, ticket.TenantID, teamID, assigneeID) {
		return nil, nil, 0, errorsx.InvalidParam("assignee must be a member of the ticket product repair team")
	}
	if err := validateManualTicketCapabilityDB(db, ticket, assigneeID); err != nil {
		return nil, nil, 0, err
	}
	return user, profile, teamID, nil
}

func validateTeamRosterScheduleDB(db *gorm.DB, team *models.AgentTeam, assigneeID int64, now time.Time, allowNoActiveSchedule bool) error {
	if db == nil || team == nil || !team.ScheduleEnforced || assigneeID <= 0 || !db.Migrator().HasTable(&models.AgentTeamSchedule{}) {
		return nil
	}
	if allowNoActiveSchedule {
		return nil
	}
	schedules := repositories.AgentTeamScheduleRepository.Find(db, sqls.NewCnd().
		Eq("tenant_id", team.TenantID).
		Eq("team_id", team.ID).
		Eq("publish_status", AgentTeamSchedulePublishPublished).
		Eq("status", enums.StatusOk))
	if len(schedules) == 0 {
		if allowNoActiveSchedule {
			return nil
		}
		return errorsx.InvalidParam("assignee has no active product repair schedule")
	}
	for _, schedule := range schedules {
		if isScheduleActiveAt(schedule, now) {
			if schedule.UserID == 0 || schedule.UserID == assigneeID {
				return nil
			}
		}
	}
	return errorsx.InvalidParam("assignee is outside the active product repair roster")
}

func isUserScheduleActiveAt(schedule models.AgentTeamSchedule, userID int64, at time.Time) bool {
	if schedule.UserID != 0 && schedule.UserID != userID {
		return false
	}
	return isScheduleActiveAt(schedule, at)
}

func isScheduleActiveAt(schedule models.AgentTeamSchedule, at time.Time) bool {
	switch NormalizeAgentTeamScheduleRepeatType(schedule.RepeatType) {
	case AgentTeamScheduleRepeatWeekly:
		startAt, endAt, ok := weeklyScheduleOccurrence(schedule, at)
		return ok && !at.Before(startAt) && at.Before(endAt)
	default:
		return !at.Before(schedule.StartAt) && at.Before(schedule.EndAt)
	}
}

func validateAssignedTicketAcceptanceDB(db *gorm.DB, ticket *models.Ticket, assigneeID, teamID int64, now time.Time) error {
	if ticket == nil || ticket.TenantID <= 0 || assigneeID <= 0 {
		return nil
	}
	if deadline, ok := ticketEffectiveAcceptanceDeadlineDB(db, ticket); ok && !deadline.After(now) {
		return errorsx.Forbidden("the ticket accept deadline has expired")
	}
	if teamID > 0 && db != nil && db.Migrator().HasTable(&models.AgentTeam{}) {
		team := repositories.AgentTeamRepository.Get(db, teamID)
		if team == nil || team.TenantID != ticket.TenantID || team.Status != enums.StatusOk {
			return errorsx.InvalidParam("ticket product repair team is not active")
		}
		if ticket.ProductID > 0 && (team.TeamType != AgentTeamTypeProductRepair || team.ProductID != ticket.ProductID) {
			return errorsx.InvalidParam("assignee must belong to the ticket product repair team")
		}
		if !AgentTeamMemberService.IsUserActiveMemberOfTeamDB(db, ticket.TenantID, teamID, assigneeID) {
			return errorsx.InvalidParam("assignee must be an active member of the ticket product repair team")
		}
	}
	return nil
}

func validateDispatchWorkStatusDB(db *gorm.DB, tenantID, userID int64, now time.Time) error {
	if tenantID <= 0 || userID <= 0 || db == nil {
		return nil
	}
	if !AgentScheduleExceptionService.isUserAvailableDB(db, tenantID, userID, now) {
		return errorsx.InvalidParam("assignee is on approved leave")
	}
	return nil
}

func validateAutoAssigneeLeaveAvailabilityDB(db *gorm.DB, tenantID, userID int64, now time.Time) error {
	if tenantID <= 0 || userID <= 0 || db == nil {
		return nil
	}
	if !AgentScheduleExceptionService.isUserAvailableDB(db, tenantID, userID, now) {
		return errorsx.InvalidParam("assignee is on approved leave")
	}
	return nil
}

func validateManualDispatchProfileDB(db *gorm.DB, tenantID int64, profile *models.AgentProfile, now time.Time, allowRequestPresence bool) error {
	if db == nil || tenantID <= 0 || profile == nil {
		return nil
	}
	if profile.TenantID != tenantID || profile.Status != enums.StatusOk {
		return errorsx.InvalidParam("assignee must have an active engineer profile")
	}
	if !profile.AutoAssignEnabled {
		return errorsx.InvalidParam("assignee is not enabled for dispatch")
	}
	if profile.ServiceStatus != enums.ServiceStatusIdle {
		return errorsx.InvalidParam("assignee is not idle for dispatch")
	}
	if profile.MaxConcurrentCount <= 0 {
		return errorsx.InvalidParam("assignee has no dispatch capacity")
	}
	if !allowRequestPresence && !isDispatchReachable(profile, now) {
		return errorsx.InvalidParam("assignee is not reachable for dispatch")
	}
	workload, err := manualDispatchWorkloadForUserDB(db, profile.UserID)
	if err != nil {
		return err
	}
	if workload >= profile.MaxConcurrentCount {
		return errorsx.InvalidParam("assignee has reached dispatch capacity")
	}
	return nil
}

func manualDispatchWorkloadForUserDB(db *gorm.DB, userID int64) (int, error) {
	if db == nil || userID <= 0 {
		return 0, nil
	}
	total := int64(0)
	if db.Migrator().HasTable(&models.Conversation{}) {
		query := db.Model(&models.Conversation{}).
			Where("status IN ? AND current_assignee_id = ?", []enums.IMConversationStatus{
				enums.IMConversationStatusPending,
				enums.IMConversationStatusActive,
			}, userID)
		if db.Migrator().HasTable(&models.Ticket{}) {
			linkedTicket := db.Model(&models.Ticket{}).
				Select("1").
				Where("conversation_id = " + conversationIDColumnRef(db))
			query = query.Where("NOT EXISTS (?)", linkedTicket)
		}
		var activeConversations int64
		if err := query.Count(&activeConversations).Error; err != nil {
			return 0, err
		}
		total += activeConversations
	}
	if db.Migrator().HasTable(&models.Ticket{}) {
		var openTickets int64
		if err := db.Model(&models.Ticket{}).
			Where("current_assignee_id = ?", userID).
			Where("status IN ?", dispatchWorkloadTicketStatuses()).
			Count(&openTickets).Error; err != nil {
			return 0, err
		}
		total += openTickets
	}
	return int(total), nil
}

func validateManualTicketCapabilityDB(db *gorm.DB, ticket *models.Ticket, assigneeID int64) error {
	if err := validateManualTicketDispatchCapabilityDB(db, ticket, assigneeID); err != nil {
		if errors.Is(err, errConversationDispatchCandidateUnavailable) {
			return errorsx.InvalidParam("assignee capability does not match the ticket")
		}
		return err
	}
	return nil
}

func validateManualConversationLinkedTicketCapabilityDB(db *gorm.DB, conversation *models.Conversation, assigneeID int64) error {
	if db == nil || conversation == nil || conversation.ID <= 0 || !db.Migrator().HasTable(&models.Ticket{}) {
		return nil
	}
	var ticket models.Ticket
	err := db.Where("conversation_id = ? AND status NOT IN ?", conversation.ID, []enums.TicketStatus{
		enums.TicketStatusClosed,
		enums.TicketStatusDone,
		enums.TicketStatusCancelled,
	}).Order("id DESC").First(&ticket).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return validateManualTicketCapabilityDB(db, &ticket, assigneeID)
}

// Assign 分配工单
func (s *ticketLifecycleService) Assign(ticketID int64, assigneeID int64, reason string, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if assigneeID <= 0 {
		return errorsx.InvalidParamI18n("error.e0334")
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		ticket := loadTicketForUpdate(ctx.Tx, ticketID)
		if ticket == nil {
			return errorsx.InvalidParamI18n("error.e0178")
		}
		if err := requireTicketTenantAccess(ticket, operator); err != nil {
			return err
		}
		if err := requireManualTicketAssignmentAuthority(ticket, operator); err != nil {
			return err
		}
		if !canAssignTicketStatus(ticket.Status) {
			return errorsx.InvalidParamI18n("error.e0182")
		}
		if ticket.CurrentAssigneeID == assigneeID {
			return nil
		}
		now := time.Now()
		toUser, _, currentTeamID, err := validateManualTicketAssigneeDB(ctx.Tx, ticket, assigneeID, now)
		if err != nil {
			return err
		}
		fromUserID := ticket.CurrentAssigneeID
		updates := map[string]any{
			"status":              ticketAssignmentStatus(ticket.Status),
			"current_team_id":     currentTeamID,
			"current_assignee_id": assigneeID,
			"updated_at":          now,
			"update_user_id":      operator.UserID,
			"update_user_name":    operator.Username,
		}
		addTicketAssignmentTrackingForTicketDB(updates, ctx.Tx, ticket, now)
		if err := repositories.TicketRepository.Updates(ctx.Tx, ticket.ID, updates); err != nil {
			return err
		}
		fromName := ""
		if fromUserID > 0 {
			if fromUser := repositories.UserRepository.Get(ctx.Tx, fromUserID); fromUser != nil {
				fromName = fromUser.Username
			}
		}
		content := "分配工单"
		if fromName != "" {
			content += "，原处理人：" + fromName
		}
		content += " -> " + toUser.Username
		if trimmedReason := strings.TrimSpace(reason); trimmedReason != "" {
			content += "，原因：" + trimmedReason
		}
		progress, err := s.createProgressTx(ctx.Tx, ticket.ID, enums.TicketProgressEventAssigned, content, operator)
		if err != nil {
			return err
		}
		if err := createTicketDispatchAttemptTx(ctx.Tx, ticket, currentTeamID, assigneeID, reason, operator, now); err != nil {
			return err
		}
		// 与 Dashboard 派单、会话自动派单保持一致：发布 ticket.assigned 事件触发通知。
		if _, err := enqueueTicketAssignedEventTx(ctx.Tx, ticket, fromUserID, assigneeID, strings.TrimSpace(reason), operator, progress.ID, now); err != nil {
			return err
		}
		if ctx.RegisterCallback != nil {
			ctx.RegisterCallback(eventbus.WakeDefaultOutboxPublisher)
		}
		return nil
	})
}

func (s *ticketLifecycleService) closeCollaborationResources(db *gorm.DB, ticket *models.Ticket, resolution string, operator *dto.AuthPrincipal, closedAt time.Time) error {
	if ticket == nil || operator == nil {
		return nil
	}
	var activeMeetings []models.MeetingRoomJitsi
	if err := db.Where("tenant_id = ? AND ticket_id = ? AND status IN ?",
		ticket.TenantID,
		strconv.FormatInt(ticket.ID, 10),
		[]string{"waiting", "scheduled", "active"},
	).Find(&activeMeetings).Error; err != nil {
		return err
	}
	actorID := strconv.FormatInt(operator.UserID, 10)
	for i := range activeMeetings {
		if _, _, err := MeetingService.endMeetingAtDB(
			db,
			activeMeetings[i].ID,
			ticket.TenantID,
			closedAt,
			"ticket_lifecycle_service",
			actorID,
			"user",
		); err != nil {
			return err
		}
	}
	supplierResolution := "工单已关闭"
	if value := strings.TrimSpace(resolution); value != "" {
		supplierResolution += "：" + value
	}
	if err := repositories.TicketSupplierCollaborationRepository.ResolveActiveByTicket(
		db, ticket.TenantID, ticket.ID, supplierResolution, closedAt, operator.UserID, operator.Username,
	); err != nil {
		return err
	}
	if err := repositories.TicketSupplierCollaborationRepository.DisableAllTicketAuthorizationScopes(db, ticket.TenantID, ticket.ID, closedAt); err != nil {
		return err
	}
	if err := repositories.TicketSupplierCollaborationRepository.DisableParticipantsByTicket(
		db, ticket.TenantID, ticket.ID, closedAt, operator.UserID, operator.Username,
	); err != nil {
		return err
	}
	if ticket.ConversationID > 0 {
		if err := db.Model(&models.ConversationParticipant{}).
			Where("conversation_id = ? AND participant_type = ? AND status = ?", ticket.ConversationID, enums.IMParticipantTypePartner, enums.StatusOk).
			Updates(map[string]any{
				"left_at":          closedAt,
				"status":           enums.StatusDisabled,
				"updated_at":       closedAt,
				"update_user_id":   operator.UserID,
				"update_user_name": operator.Username,
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

// Transfer 转单
func (s *ticketLifecycleService) Transfer(ticketID int64, toUserID int64, reason string, operator *dto.AuthPrincipal) error {
	// 转单本质上也是分配，重用分配逻辑
	return s.Assign(ticketID, toUserID, reason, operator)
}

// Escalate 升级工单
func (s *ticketLifecycleService) Escalate(ticketID int64, reason string, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		ticket := loadTicketForUpdate(ctx.Tx, ticketID)
		if ticket == nil {
			return errorsx.InvalidParamI18n("error.e0178")
		}
		if err := requireTicketTenantAccess(ticket, operator); err != nil {
			return err
		}
		if err := requireTicketMutationAccess(ticket, operator); err != nil {
			return err
		}
		if !enums.IsValidTicketStatusTransition(string(ticket.Status), string(enums.TicketStatusEscalated)) {
			return errorsx.InvalidParamI18n("error.e0182")
		}
		now := time.Now()
		if err := repositories.TicketRepository.Updates(ctx.Tx, ticket.ID, map[string]any{
			"status":           enums.TicketStatusEscalated,
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		}); err != nil {
			return err
		}
		content := "升级工单"
		if trimmedReason := strings.TrimSpace(reason); trimmedReason != "" {
			content += "，原因：" + trimmedReason
		}
		return s.addProgress(ctx.Tx, ticket.ID, enums.TicketProgressEventEscalated, content, operator)
	})
}

// Repair 执行维修并填写维修记录
func (s *ticketLifecycleService) Repair(record *models.TicketRepairRecord, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if record.TicketID <= 0 {
		return errorsx.InvalidParam("ticketId is required")
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		ticket := loadTicketForUpdate(ctx.Tx, record.TicketID)
		if ticket == nil {
			return errorsx.InvalidParamI18n("error.e0178")
		}
		if err := requireTicketTenantAccess(ticket, operator); err != nil {
			return err
		}
		if err := requireTicketMutationAccess(ticket, operator); err != nil {
			return err
		}
		record.TenantID = ticket.TenantID
		record.AuditFields = utils.BuildAuditFields(operator)
		if record.DeviceID == 0 {
			record.DeviceID = ticket.DeviceID
		}
		if record.ProductID == 0 {
			record.ProductID = ticket.ProductID
		}
		if record.ProductModelID == 0 {
			record.ProductModelID = ticket.ProductModelID
		}
		now := time.Now()
		record.FinishedAt = &now
		if err := repositories.TicketRepairRepository.Create(ctx.Tx, record); err != nil {
			return err
		}
		// 保存维修记录后同步写入设备正式维修历史（设计 §6.7）
		if err := s.writeDeviceServiceRecord(ctx.Tx, ticket, record, operator); err != nil {
			return err
		}
		if enums.IsValidTicketStatusTransition(string(ticket.Status), string(enums.TicketStatusResolved)) {
			if err := repositories.TicketRepository.Updates(ctx.Tx, ticket.ID, map[string]any{
				"status":           enums.TicketStatusResolved,
				"resolved_at":      &now,
				"updated_at":       now,
				"update_user_id":   operator.UserID,
				"update_user_name": operator.Username,
			}); err != nil {
				return err
			}
		}
		return s.addProgress(ctx.Tx, ticket.ID, enums.TicketProgressEventRepairCompleted, "已填写维修记录", operator)
	})
}

// Close 关闭工单：校验维修记录、更新状态与时间线。
// 关闭提交后再生成知识候选/质量线索/事件，这些副作用失败只记日志、不阻塞关闭（设计 §6.7）。
func (s *ticketLifecycleService) Close(ticketID int64, resolution string, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	var closedTicket *models.Ticket
	var repairs []models.TicketRepairRecord
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		ticket := loadTicketForUpdate(ctx.Tx, ticketID)
		if ticket == nil {
			return errorsx.InvalidParamI18n("error.e0178")
		}
		if err := requireTicketTenantAccess(ticket, operator); err != nil {
			return err
		}
		if err := requireTicketMutationAccess(ticket, operator); err != nil {
			return err
		}
		if enums.NormalizeTicketStatus(string(ticket.Status)) == enums.TicketStatusClosed {
			return nil
		}
		customerInitiatedClose := operator.IsCustomer()
		if !customerInitiatedClose && !isValidTicketCloseTransition(ticket.Status) {
			return errorsx.InvalidParamI18n("error.e0182")
		}
		// Internal closure requires a repair record. Customers may end a ticket
		// at any point, so their explicit action is recorded as a separate close
		// reason without inventing a repair conclusion.
		repairs = repositories.TicketRepairRepository.FindByTicketID(ctx.Tx, ticket.ID)
		if !customerInitiatedClose && len(repairs) == 0 {
			return errorsx.InvalidParam("ticket must have at least one repair record before closing")
		}
		if !customerInitiatedClose && ticket.ProductID > 0 && strings.TrimSpace(ticket.FaultCode) == "" {
			return errorsx.InvalidParam("fault type is required before closing a product ticket")
		}

		now := time.Now()
		if err := repositories.TicketRepository.Updates(ctx.Tx, ticket.ID, map[string]any{
			"status":                       enums.TicketStatusClosed,
			"handled_at":                   &now,
			"accept_deadline_at":           nil,
			"dispatch_deferred_until":      nil,
			"last_dispatch_failure_reason": "",
			"updated_at":                   now,
			"update_user_id":               operator.UserID,
			"update_user_name":             operator.Username,
		}); err != nil {
			return err
		}
		if err := finishPendingTicketDispatchAttemptTx(ctx.Tx, ticket.ID, ticketDispatchOutcomeClosed, nil, now); err != nil {
			return err
		}
		ticket.Status = enums.TicketStatusClosed
		ticket.HandledAt = &now
		ticket.UpdatedAt = now
		if err := s.closeCollaborationResources(ctx.Tx, ticket, resolution, operator, now); err != nil {
			return err
		}
		if ctx.RegisterCallback != nil {
			ctx.RegisterCallback(eventbus.WakeDefaultOutboxPublisher)
		}
		if err := s.closeLinkedConversation(ctx, ticket, resolution, operator, now); err != nil {
			return err
		}

		content := "关闭工单"
		if trimmedResolution := strings.TrimSpace(resolution); trimmedResolution != "" {
			content += "，结论：" + trimmedResolution
		}
		if err := s.addProgress(ctx.Tx, ticket.ID, enums.TicketProgressEventClosed, content, operator); err != nil {
			return err
		}
		closedTicket = ticket
		return nil
	})
	if err != nil {
		return err
	}
	s.generateCloseSideEffects(closedTicket, repairs, resolution, operator)
	return nil
}

// Cancel cancels an active ticket without producing repair or knowledge side effects.
func (s *ticketLifecycleService) Cancel(ticketID int64, reason string, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errorsx.InvalidParam("cancellation reason is required")
	}
	var cancelledAssigneeID int64
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		ticket := loadTicketForUpdate(ctx.Tx, ticketID)
		if ticket == nil {
			return errorsx.InvalidParamI18n("error.e0178")
		}
		if err := requireTicketTenantAccess(ticket, operator); err != nil {
			return err
		}
		if err := requireTicketMutationAccess(ticket, operator); err != nil {
			return err
		}
		status := enums.NormalizeTicketStatus(string(ticket.Status))
		if status == enums.TicketStatusCancelled {
			return nil
		}
		if !enums.IsValidAfterSalesTransition(string(status), string(enums.TicketStatusCancelled)) {
			return errorsx.InvalidParamI18n("error.e0182")
		}

		now := time.Now()
		if err := repositories.TicketRepository.Updates(ctx.Tx, ticket.ID, map[string]any{
			"status":                       enums.TicketStatusCancelled,
			"handled_at":                   &now,
			"resolved_at":                  nil,
			"accept_deadline_at":           nil,
			"dispatch_deferred_until":      nil,
			"last_dispatch_failure_reason": "",
			"updated_at":                   now,
			"update_user_id":               operator.UserID,
			"update_user_name":             operator.Username,
		}); err != nil {
			return err
		}
		if err := finishPendingTicketDispatchAttemptTx(ctx.Tx, ticket.ID, ticketDispatchOutcomeCancelled, nil, now); err != nil {
			return err
		}
		ticket.Status = enums.TicketStatusCancelled
		ticket.HandledAt = &now
		if err := s.closeCollaborationResources(ctx.Tx, ticket, "工单已取消："+reason, operator, now); err != nil {
			return err
		}
		if ctx.RegisterCallback != nil {
			ctx.RegisterCallback(eventbus.WakeDefaultOutboxPublisher)
		}
		if err := s.closeLinkedConversation(ctx, ticket, reason, operator, now); err != nil {
			return err
		}
		cancelledAssigneeID = ticket.CurrentAssigneeID
		return s.addProgress(ctx.Tx, ticket.ID, enums.TicketProgressEventClosed, "取消工单，原因："+reason, operator)
	}); err != nil {
		return err
	}
	if cancelledAssigneeID > 0 {
		if ticket := TicketService.Get(ticketID); ticket != nil {
			if err := NotificationService.ReconcileTicketAssignedAfterCancellation(ticket, cancelledAssigneeID); err != nil {
				slog.Warn("reconcile ticket assignment notification after cancellation failed", "ticketId", ticketID, "assigneeId", cancelledAssigneeID, "error", err)
			}
		}
	}
	return nil
}

func (s *ticketLifecycleService) closeLinkedConversation(ctx *sqls.TxContext, ticket *models.Ticket, resolution string, operator *dto.AuthPrincipal, closedAt time.Time) error {
	if ctx == nil || ctx.Tx == nil || ticket == nil || ticket.ConversationID <= 0 || operator == nil {
		return nil
	}
	conversation := repositories.ConversationRepository.Get(ctx.Tx, ticket.ConversationID)
	if conversation == nil {
		return nil
	}
	if ticket.TenantID > 0 && conversation.TenantID != ticket.TenantID {
		return errorsx.Forbidden("ticket conversation tenant mismatch")
	}
	if conversation.Status == enums.IMConversationStatusClosed {
		return nil
	}
	if err := ConversationAssignmentService.FinishActiveAssignments(ctx, conversation.ID, closedAt); err != nil {
		return err
	}
	closeReason := "关联工单已关闭"
	eventDescription := "关联工单关闭，会话已结束"
	if enums.NormalizeTicketStatus(string(ticket.Status)) == enums.TicketStatusCancelled {
		closeReason = "关联工单已取消"
		eventDescription = "关联工单取消，会话已结束"
	}
	if value := strings.TrimSpace(resolution); value != "" {
		closeReason += "：" + value
	}
	if err := repositories.ConversationRepository.Updates(ctx.Tx, conversation.ID, map[string]any{
		"status":           enums.IMConversationStatusClosed,
		"closed_at":        closedAt,
		"closed_by":        operator.UserID,
		"close_reason":     closeReason,
		"updated_at":       closedAt,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
	}); err != nil {
		return err
	}
	senderType := enums.IMSenderTypeAgent
	if operator.IsCustomer() {
		senderType = enums.IMSenderTypeCustomer
		eventDescription = "客户确认解决，关联会话已结束"
	}
	if err := ConversationEventLogService.CreateEvent(ctx, conversation.ID, enums.IMEventTypeClose, senderType, operator.UserID, eventDescription, ConversationService.buildEventPayload(map[string]any{
		"fromStatus":     conversation.Status,
		"toStatus":       enums.IMConversationStatusClosed,
		"fromAssigneeId": conversation.CurrentAssigneeID,
		"toAssigneeId":   conversation.CurrentAssigneeID,
		"closeReason":    closeReason,
		"ticketId":       ticket.ID,
	})); err != nil {
		return err
	}
	if ctx.RegisterCallback != nil {
		conversationID := conversation.ID
		ctx.RegisterCallback(func() {
			if current := ConversationService.Get(conversationID); current != nil {
				WsService.PublishConversationChanged(current, enums.IMRealtimeEventConversationClosed)
			}
		})
	}
	return nil
}

func (s *ticketLifecycleService) BackfillClosedTicketConversations(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	tickets := repositories.TicketRepository.Find(db, sqls.NewCnd().Where("conversation_id > 0").In("status", []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone}))
	for i := range tickets {
		ticket := tickets[i]
		if err := db.Transaction(func(tx *gorm.DB) error {
			operator := &dto.AuthPrincipal{
				TenantID: ticket.TenantID,
				UserID:   ticket.UpdateUserID,
				Username: ticket.UpdateUserName,
			}
			if operator.Username == "" {
				operator.Username = "system"
			}
			closedAt := ticket.UpdatedAt
			if ticket.HandledAt != nil {
				closedAt = *ticket.HandledAt
			}
			return s.closeLinkedConversation(&sqls.TxContext{Tx: tx}, &ticket, "历史关闭工单状态同步", operator, closedAt)
		}); err != nil {
			return err
		}
	}
	return nil
}

// RecoverClosedTicketSideEffects repairs the crash window between committing a
// closed ticket and creating its knowledge/quality side effects or publishing
// its lifecycle event. The bounded sweep is safe to replay because candidates,
// fault projections, and notifications all have stable idempotency identities.
func (s *ticketLifecycleService) RecoverClosedTicketSideEffects(limit int) int {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	db := sqls.DB()
	if db == nil {
		return 0
	}
	ticketTable := db.NamingStrategy.TableName("Ticket")
	candidateTable := db.NamingStrategy.TableName("KnowledgeCandidate")
	faultEventTable := db.NamingStrategy.TableName("FaultStatsEventInbox")
	activeCandidateStatuses := []string{
		string(enums.KnowledgeCandidateReviewStatusPending),
		string(enums.KnowledgeCandidateReviewStatusNeedsEnrichment),
		string(enums.KnowledgeCandidateReviewStatusLowQuality),
		string(enums.KnowledgeCandidateReviewStatusLowValue),
		string(enums.KnowledgeCandidateReviewStatusDuplicate),
		string(enums.KnowledgeCandidateReviewStatusApproved),
		string(enums.KnowledgeCandidateReviewStatusRejected),
		string(enums.KnowledgeCandidateReviewStatusMerged),
	}
	validCandidate := db.Table(candidateTable+" AS recovery_candidate").
		Select("1").
		Where("recovery_candidate.tenant_id = recovery_ticket.tenant_id").
		Where("recovery_candidate.ticket_id = recovery_ticket.id").
		Where("recovery_candidate.status <> ?", enums.StatusDeleted).
		Where("recovery_candidate.review_status IN ?", activeCandidateStatuses).
		Where("NOT (recovery_candidate.review_status = ? AND recovery_candidate.requires_reassessment = ?)", enums.KnowledgeCandidateReviewStatusApproved, true).
		Where("recovery_candidate.review_status <> ? OR recovery_candidate.reviewed_at >= COALESCE(recovery_ticket.handled_at, recovery_ticket.updated_at)", enums.KnowledgeCandidateReviewStatusRejected)
	closedEvent := db.Table(faultEventTable+" AS recovery_event").
		Select("1").
		Where("recovery_event.tenant_id = recovery_ticket.tenant_id").
		Where("recovery_event.ticket_id = recovery_ticket.id").
		Where("recovery_event.event_type = ?", events.FaultStatsEventTicketClosed).
		Where("recovery_event.processed_at >= COALESCE(recovery_ticket.handled_at, recovery_ticket.updated_at)")

	var tickets []models.Ticket
	if err := db.Table(ticketTable+" AS recovery_ticket").
		Where("recovery_ticket.status = ?", enums.TicketStatusClosed).
		Where("NOT EXISTS (?) OR (recovery_ticket.product_id > 0 AND NOT EXISTS (?))", validCandidate, closedEvent).
		Order("recovery_ticket.handled_at ASC, recovery_ticket.id ASC").
		Limit(limit).
		Find(&tickets).Error; err != nil {
		slog.Error("find closed tickets requiring side effect recovery failed", "error", err)
		return 0
	}

	recovered := 0
	for i := range tickets {
		var candidateCount int64
		closedAt := tickets[i].UpdatedAt
		if tickets[i].HandledAt != nil {
			closedAt = *tickets[i].HandledAt
		}
		if err := db.Model(&models.KnowledgeCandidate{}).
			Where("tenant_id = ? AND ticket_id = ? AND status <> ? AND review_status IN ? AND NOT (review_status = ? AND requires_reassessment = ?) AND (review_status <> ? OR reviewed_at >= ?)",
				tickets[i].TenantID, tickets[i].ID, enums.StatusDeleted, activeCandidateStatuses,
				enums.KnowledgeCandidateReviewStatusApproved, true, enums.KnowledgeCandidateReviewStatusRejected, closedAt).
			Count(&candidateCount).Error; err != nil {
			continue
		}
		var closedEventCount int64
		if tickets[i].ProductID > 0 {
			if err := db.Model(&models.FaultStatsEventInbox{}).
				Where("tenant_id = ? AND ticket_id = ? AND event_type = ? AND processed_at >= ?",
					tickets[i].TenantID, tickets[i].ID, events.FaultStatsEventTicketClosed, closedAt).
				Count(&closedEventCount).Error; err != nil {
				continue
			}
		}
		repairs := repositories.TicketRepairRepository.FindByTicketID(db, tickets[i].ID)
		resolution := "恢复已关闭工单的知识沉淀"
		if latest := latestVerifiedKnowledgeRepair(repairs); latest != nil {
			resolution = firstNonBlank(strings.TrimSpace(latest.Conclusion), strings.TrimSpace(latest.Solution), resolution)
		}
		operatorName := strings.TrimSpace(tickets[i].UpdateUserName)
		if operatorName == "" {
			operatorName = "ticket-close-recovery"
		}
		operator := &dto.AuthPrincipal{
			TenantID: tickets[i].TenantID,
			UserID:   tickets[i].UpdateUserID,
			Username: operatorName,
		}
		eventScheduled := false
		if candidateCount == 0 {
			s.generateCloseSideEffects(&tickets[i], repairs, resolution, operator)
			eventScheduled = true
		} else if tickets[i].ProductID > 0 && closedEventCount == 0 {
			if err := s.enqueueTicketClosedEvent(&tickets[i], resolution, operator); err != nil {
				slog.Error("recover ticket closed event failed", "ticket_id", tickets[i].ID, "error", err)
				continue
			}
			eventScheduled = true
		}

		if err := db.Model(&models.KnowledgeCandidate{}).
			Where("tenant_id = ? AND ticket_id = ? AND status <> ? AND review_status IN ? AND NOT (review_status = ? AND requires_reassessment = ?) AND (review_status <> ? OR reviewed_at >= ?)",
				tickets[i].TenantID, tickets[i].ID, enums.StatusDeleted, activeCandidateStatuses,
				enums.KnowledgeCandidateReviewStatusApproved, true, enums.KnowledgeCandidateReviewStatusRejected, closedAt).
			Count(&candidateCount).Error; err == nil && candidateCount > 0 &&
			(tickets[i].ProductID == 0 || closedEventCount > 0 || eventScheduled) {
			recovered++
		}
	}
	return recovered
}

func isValidTicketCloseTransition(status enums.TicketStatus) bool {
	return enums.IsValidAfterSalesTransition(string(status), string(enums.TicketStatusClosed)) ||
		enums.IsValidTicketStatusTransition(string(status), string(enums.TicketStatusClosed))
}

// generateCloseSideEffects 工单关闭后的副作用：知识候选（幂等）、质量线索、事件。
// 失败只记审计/错误日志，不影响关闭结果（设计 §6.7）。
func (s *ticketLifecycleService) generateCloseSideEffects(ticket *models.Ticket, repairs []models.TicketRepairRecord, resolution string, operator *dto.AuthPrincipal) {
	if ticket == nil {
		return
	}
	var candidateEvent *events.KnowledgeCandidateCreatedEvent
	if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		current := loadTicketForUpdate(tx, ticket.ID)
		if current == nil {
			return errorsx.InvalidParamI18n("error.e0178")
		}
		status := enums.NormalizeTicketStatus(string(current.Status))
		if status != enums.TicketStatusClosed {
			return nil
		}
		var err error
		candidateEvent, err = s.ensureTicketRepairKnowledgeCandidateTx(tx, current, repairs, resolution, operator)
		if err != nil {
			return err
		}
		if clue := s.buildQualityClue(current, repairs, operator); clue != nil {
			existingClue := repositories.TicketQualityClueRepository.FindOne(tx, sqls.NewCnd().
				Eq("tenant_id", clue.TenantID).
				Eq("ticket_id", clue.TicketID).
				Eq("clue_type", clue.ClueType).
				Desc("id"))
			if existingClue == nil {
				if err := repositories.TicketQualityClueRepository.Create(tx, clue); err != nil {
					return err
				}
			} else if err := repositories.TicketQualityClueRepository.Updates(tx, existingClue.ID, map[string]any{
				"severity":         clue.Severity,
				"description":      clue.Description,
				"clue_data_json":   clue.ClueDataJSON,
				"detected_at":      clue.DetectedAt,
				"updated_at":       clue.UpdatedAt,
				"update_user_id":   clue.UpdateUserID,
				"update_user_name": clue.UpdateUserName,
			}); err != nil {
				return err
			}
		}
		if candidateEvent != nil {
			if err := s.enqueueKnowledgeCandidateCreatedEventTx(tx, candidateEvent, operator, "ticket_lifecycle_service"); err != nil {
				return err
			}
		}
		return s.enqueueTicketClosedEventTx(tx, current, resolution, operator, false)
	}); err != nil {
		slog.Error("generate ticket close side effects failed", "ticket_id", ticket.ID, "error", err)
		return
	}
	eventbus.WakeDefaultOutboxPublisher()
}

func (s *ticketLifecycleService) ensureTicketRepairKnowledgeCandidateTx(tx *gorm.DB, ticket *models.Ticket, repairs []models.TicketRepairRecord, resolution string, operator *dto.AuthPrincipal) (*events.KnowledgeCandidateCreatedEvent, error) {
	if tx == nil || ticket == nil || ticket.ID <= 0 || ticket.TenantID <= 0 {
		return nil, nil
	}
	existing := repositories.KnowledgeCandidateRepository.FindOne(tx,
		sqls.NewCnd().
			Eq("tenant_id", ticket.TenantID).
			Eq("ticket_id", ticket.ID).
			NotEq("status", enums.StatusDeleted).
			Desc("knowledge_entry_id").
			Desc("id"))
	rootCauseSummary := s.buildRootCauseSummary(repairs)
	solutionSummary := s.buildSolutionSummary(repairs)
	suggestion := firstNonBlank(strings.TrimSpace(resolution), solutionSummary, rootCauseSummary, ticket.DiagnosisSummary, ticket.SymptomSummary, ticket.Description, "维修知识候选")
	candidateTitle := buildTicketKnowledgeCandidateTitle("", ticket, repairs)
	if existing == nil {
		candidate := &models.KnowledgeCandidate{
			TenantID:         ticket.TenantID,
			ProductID:        ticket.ProductID,
			ProductModelID:   ticket.ProductModelID,
			SourceType:       "ticket_repair",
			SourceID:         ticket.TicketNo,
			TicketID:         ticket.ID,
			Title:            candidateTitle,
			Suggestion:       suggestion,
			RootCauseSummary: rootCauseSummary,
			SolutionSummary:  solutionSummary,
			Status:           enums.StatusOk,
			AuditFields:      utils.BuildAuditFields(operator),
		}
		if err := KnowledgeCandidateScoringService.PrepareCandidateDB(tx, candidate, ticket, repairs); err != nil {
			return nil, err
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(candidate)
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 0 {
			existing = repositories.KnowledgeCandidateRepository.FindOne(tx,
				sqls.NewCnd().
					Eq("tenant_id", ticket.TenantID).
					Eq("ticket_id", ticket.ID).
					NotEq("status", enums.StatusDeleted).
					Desc("knowledge_entry_id").
					Desc("id"))
			if existing == nil {
				return nil, errorsx.InvalidParam("knowledge candidate could not be created")
			}
		} else {
			if err := KnowledgeCandidateScoringService.RefreshDuplicateGroupDB(tx, candidate.SimilarityHash); err != nil {
				return nil, err
			}
			return &events.KnowledgeCandidateCreatedEvent{
				CandidateID: candidate.ID,
				SourceType:  candidate.SourceType,
				SourceID:    candidate.SourceID,
				TenantID:    candidate.TenantID,
				Suggestion:  suggestion,
			}, nil
		}
	}
	if shouldRefreshClosedTicketKnowledgeCandidate(existing, ticket) {
		now := time.Now()
		rescored := *existing
		rescored.RequiresReassessment = false
		rescored.Title = candidateTitle
		rescored.Suggestion = suggestion
		rescored.RootCauseSummary = rootCauseSummary
		rescored.SolutionSummary = solutionSummary
		if err := KnowledgeCandidateScoringService.PrepareCandidateDB(tx, &rescored, ticket, repairs); err != nil {
			return nil, err
		}
		updates := knowledgeCandidateScoreUpdates(&rescored)
		for key, value := range map[string]any{
			"title":              candidateTitle,
			"suggestion":         suggestion,
			"root_cause_summary": rootCauseSummary,
			"solution_summary":   solutionSummary,
			"updated_at":         now,
			"update_user_id":     auditUserID(operator),
			"update_user_name":   auditUserName(operator),
		} {
			updates[key] = value
		}
		if err := repositories.KnowledgeCandidateRepository.Updates(tx, existing.ID, updates); err != nil {
			return nil, err
		}
		if err := KnowledgeCandidateScoringService.RefreshDuplicateGroupDB(tx, rescored.SimilarityHash); err != nil {
			return nil, err
		}
		return &events.KnowledgeCandidateCreatedEvent{
			CandidateID: existing.ID,
			SourceType:  existing.SourceType,
			SourceID:    existing.SourceID,
			TenantID:    existing.TenantID,
			Suggestion:  suggestion,
		}, nil
	}
	return nil, nil
}

func shouldRefreshClosedTicketKnowledgeCandidate(candidate *models.KnowledgeCandidate, ticket *models.Ticket) bool {
	if candidate == nil || ticket == nil {
		return false
	}
	if candidate.KnowledgeEntryID > 0 && !candidate.RequiresReassessment {
		return false
	}
	switch candidate.ReviewStatus {
	case string(enums.KnowledgeCandidateReviewStatusApproved):
		return candidate.RequiresReassessment
	case string(enums.KnowledgeCandidateReviewStatusMerged):
		return false
	case string(enums.KnowledgeCandidateReviewStatusRejected):
		closedAt := ticket.UpdatedAt
		if ticket.HandledAt != nil {
			closedAt = *ticket.HandledAt
		}
		return candidate.ReviewedAt == nil || candidate.ReviewedAt.Before(closedAt)
	default:
		return true
	}
}

func (s *ticketLifecycleService) enqueueKnowledgeCandidateCreatedEventTx(tx *gorm.DB, candidateEvent *events.KnowledgeCandidateCreatedEvent, operator *dto.AuthPrincipal, source string) error {
	if tx == nil || candidateEvent == nil || candidateEvent.CandidateID <= 0 || candidateEvent.TenantID <= 0 {
		return nil
	}
	if source == "" {
		source = "ticket_lifecycle_service"
	}
	candidateEvent.EventID = "tenant:" + strconv.FormatInt(candidateEvent.TenantID, 10) + ":knowledge.candidate.created:" + strconv.FormatInt(candidateEvent.CandidateID, 10)
	_, err := eventbus.EnqueueTx(tx, eventbus.DurableEvent{
		TenantID:       candidateEvent.TenantID,
		IdempotencyKey: candidateEvent.EventID,
		EventType:      events.EventKnowledgeCandidateCreated,
		Payload:        *candidateEvent,
		Source:         source,
		AggregateID:    strconv.FormatInt(candidateEvent.CandidateID, 10),
		ActorID:        strconv.FormatInt(auditUserID(operator), 10),
		ActorType:      "user",
		CreatedAt:      time.Now(),
	})
	return err
}

func (s *ticketLifecycleService) enqueueTicketClosedEvent(ticket *models.Ticket, resolution string, operator *dto.AuthPrincipal) error {
	if ticket == nil {
		return nil
	}
	err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		return s.enqueueTicketClosedEventTx(tx, ticket, resolution, operator, true)
	})
	if err == nil {
		eventbus.WakeDefaultOutboxPublisher()
	}
	return err
}

func (s *ticketLifecycleService) enqueueTicketClosedEventTx(tx *gorm.DB, ticket *models.Ticket, resolution string, operator *dto.AuthPrincipal, replayIfExists bool) error {
	if tx == nil || ticket == nil {
		return nil
	}
	closedAt := ticket.UpdatedAt
	if ticket.HandledAt != nil {
		closedAt = *ticket.HandledAt
	}
	operatorID := int64(0)
	if operator != nil {
		operatorID = operator.UserID
	}
	closedEvent := events.TicketClosedEvent{
		EventID:    ticketLifecycleEventID(events.FaultStatsEventTicketClosed, ticket.ID, closedAt),
		TicketID:   ticket.ID,
		TenantID:   ticket.TenantID,
		OperatorID: operatorID,
		Resolution: resolution,
		OccurredAt: closedAt,
	}
	_, err := eventbus.EnqueueTx(tx, eventbus.DurableEvent{
		TenantID:       ticket.TenantID,
		IdempotencyKey: "tenant:" + strconv.FormatInt(ticket.TenantID, 10) + ":" + closedEvent.EventID,
		EventType:      events.EventTicketClosed,
		Payload:        closedEvent,
		Source:         "ticket_lifecycle_service",
		AggregateID:    strconv.FormatInt(ticket.ID, 10),
		ActorID:        strconv.FormatInt(operatorID, 10),
		ActorType:      "user",
		CreatedAt:      closedAt,
		ReplayIfExists: replayIfExists,
	})
	return err
}

func ticketLifecycleEventID(eventType string, ticketID int64, occurredAt time.Time) string {
	return eventType + ":" + strconv.FormatInt(ticketID, 10) + ":" + strconv.FormatInt(occurredAt.UTC().UnixNano(), 10)
}

// writeDeviceServiceRecord 保存维修记录时同步写入设备正式维修历史（设计 §6.7）。
func (s *ticketLifecycleService) writeDeviceServiceRecord(tx *gorm.DB, ticket *models.Ticket, repair *models.TicketRepairRecord, operator *dto.AuthPrincipal) error {
	now := time.Now()
	summary := repair.Solution
	if summary == "" {
		summary = repair.Conclusion
	}
	deviceRecord := &models.DeviceServiceRecord{
		TenantID:          ticket.TenantID,
		DeviceID:          repair.DeviceID,
		ProductID:         repair.ProductID,
		ProductModelID:    repair.ProductModelID,
		SourceType:        "ticket",
		SourceID:          ticket.TicketNo,
		TicketID:          ticket.ID,
		ServiceType:       repair.ServiceMethod,
		Summary:           summary,
		RootCause:         repair.RootCause,
		Solution:          repair.Solution,
		VisibleToCustomer: repair.VisibleToCustomer,
		OccurredAt:        now,
		AuditFields:       utils.BuildAuditFields(operator),
	}
	return repositories.DeviceServiceRecordRepository.Create(tx, deviceRecord)
}

// Reopen 重新打开工单
func (s *ticketLifecycleService) Reopen(ticketID int64, reason string, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	var reopened *models.Ticket
	var reopenedMessage *models.Message
	knowledgeReassessmentRequired := false
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		ticket := loadTicketForUpdate(ctx.Tx, ticketID)
		if ticket == nil {
			return errorsx.InvalidParamI18n("error.e0178")
		}
		if err := requireTicketTenantAccess(ticket, operator); err != nil {
			return err
		}
		if enums.NormalizeTicketStatus(string(ticket.Status)) == enums.TicketStatusReopened {
			return nil
		}
		if !enums.IsValidTicketStatusTransition(string(ticket.Status), string(enums.TicketStatusReopened)) {
			return errorsx.InvalidParamI18n("error.e0182")
		}
		now := time.Now()
		if err := repositories.TicketRepository.Updates(ctx.Tx, ticket.ID, map[string]any{
			"status":                       enums.TicketStatusReopened,
			"current_assignee_id":          0,
			"resolved_at":                  nil,
			"handled_at":                   nil,
			"assigned_at":                  nil,
			"accepted_at":                  nil,
			"accept_deadline_at":           nil,
			"dispatch_attempts":            0,
			"dispatch_deferred_until":      nil,
			"last_dispatch_failure_reason": "",
			"updated_at":                   now,
			"update_user_id":               operator.UserID,
			"update_user_name":             operator.Username,
		}); err != nil {
			return err
		}
		if err := finishPendingTicketDispatchAttemptTx(ctx.Tx, ticket.ID, ticketDispatchOutcomeSuperseded, nil, now); err != nil {
			return err
		}
		ticket.Status = enums.TicketStatusReopened
		ticket.CurrentAssigneeID = 0
		ticket.ResolvedAt = nil
		ticket.HandledAt = nil
		ticket.AssignedAt = nil
		ticket.AcceptedAt = nil
		ticket.AcceptDeadlineAt = nil
		ticket.DispatchAttempts = 0
		ticket.DispatchDeferredUntil = nil
		ticket.LastDispatchFailureReason = ""
		ticket.UpdatedAt = now
		pendingCandidates := repositories.KnowledgeCandidateRepository.Find(ctx.Tx, sqls.NewCnd().
			Eq("tenant_id", ticket.TenantID).
			Eq("ticket_id", ticket.ID).
			In("review_status", []string{
				string(enums.KnowledgeCandidateReviewStatusPending),
				string(enums.KnowledgeCandidateReviewStatusNeedsEnrichment),
				string(enums.KnowledgeCandidateReviewStatusLowQuality),
				string(enums.KnowledgeCandidateReviewStatusLowValue),
				string(enums.KnowledgeCandidateReviewStatusDuplicate),
			}).
			NotEq("status", enums.StatusDeleted))
		for i := range pendingCandidates {
			if err := repositories.KnowledgeCandidateRepository.Updates(ctx.Tx, pendingCandidates[i].ID, map[string]any{
				"review_status":    "rejected",
				"review_remark":    "工单重新打开，原知识候选已失效",
				"reviewed_at":      now,
				"reviewer_id":      operator.UserID,
				"updated_at":       now,
				"update_user_id":   operator.UserID,
				"update_user_name": operator.Username,
			}); err != nil {
				return err
			}
		}
		if ticket.ConversationID > 0 {
			conversation := repositories.ConversationRepository.Get(ctx.Tx, ticket.ConversationID)
			if conversation != nil {
				if ticket.TenantID > 0 && conversation.TenantID != ticket.TenantID {
					return errorsx.Forbidden("ticket conversation tenant mismatch")
				}
				if err := ConversationAssignmentService.FinishActiveAssignments(ctx, conversation.ID, now); err != nil {
					return err
				}
				messageContent := "工程师重新打开工单"
				senderType := enums.IMSenderTypeAgent
				senderID := operator.UserID
				agentUnreadCount := conversation.AgentUnreadCount
				customerUnreadCount := conversation.CustomerUnreadCount + 1
				if operator.IsCustomer() {
					messageContent = "客户重新打开工单"
					senderType = enums.IMSenderTypeCustomer
					senderID = 0
					agentUnreadCount++
					customerUnreadCount = conversation.CustomerUnreadCount
				}
				if trimmedReason := strings.TrimSpace(reason); trimmedReason != "" {
					messageContent += "：" + trimmedReason
				}
				messagePayload, _ := json.Marshal(map[string]any{
					"source":    "ticket_reopen",
					"ticketId":  ticket.ID,
					"eventType": enums.TicketProgressEventReopened,
				})
				reopenedMessage = &models.Message{
					ConversationID: conversation.ID,
					ClientMsgID: "ticket-reopen:" + strconv.FormatInt(ticket.ID, 10) + ":" +
						strconv.FormatInt(now.UnixNano(), 10),
					SenderType:  senderType,
					SenderID:    senderID,
					MessageType: enums.IMMessageTypeText,
					Content:     messageContent,
					Payload:     string(messagePayload),
					SendStatus:  enums.IMMessageStatusSent,
					SentAt:      &now,
					AuditFields: utils.BuildAuditFields(operator),
				}
				if err := repositories.MessageRepository.Create(ctx.Tx, reopenedMessage); err != nil {
					return err
				}
				if err := repositories.ConversationRepository.Updates(ctx.Tx, conversation.ID, map[string]any{
					"status":                enums.IMConversationStatusPending,
					"current_team_id":       ticket.CurrentTeamID,
					"current_assignee_id":   0,
					"last_message_id":       reopenedMessage.ID,
					"last_message_at":       now,
					"last_active_at":        now,
					"last_message_summary":  limitText(messageContent, 255),
					"agent_unread_count":    agentUnreadCount,
					"customer_unread_count": customerUnreadCount,
					"closed_at":             nil,
					"closed_by":             0,
					"close_reason":          "",
					"updated_at":            now,
					"update_user_id":        operator.UserID,
					"update_user_name":      operator.Username,
				}); err != nil {
					return err
				}
			}
		}
		approvedCandidates := repositories.KnowledgeCandidateRepository.Find(ctx.Tx, sqls.NewCnd().
			Eq("tenant_id", ticket.TenantID).
			Eq("ticket_id", ticket.ID).
			Eq("review_status", enums.KnowledgeCandidateReviewStatusApproved).
			NotEq("status", enums.StatusDeleted))
		for i := range approvedCandidates {
			if err := repositories.KnowledgeCandidateRepository.Updates(ctx.Tx, approvedCandidates[i].ID, map[string]any{
				"requires_reassessment": true,
				"review_remark":         "工单已重新打开，关联知识已暂停并等待新一轮维修验证",
				"updated_at":            now,
				"update_user_id":        operator.UserID,
				"update_user_name":      operator.Username,
			}); err != nil {
				return err
			}
			knowledgeReassessmentRequired = true
		}
		if knowledgeReassessmentRequired {
			if err := quarantineReassessmentKnowledgeEntriesTx(ctx.Tx, ticket.TenantID, ticket.ID, now, operator); err != nil {
				return err
			}
		}
		refreshedGroups := make(map[string]struct{})
		for i := range pendingCandidates {
			if pendingCandidates[i].SimilarityHash != "" {
				refreshedGroups[pendingCandidates[i].SimilarityHash] = struct{}{}
			}
		}
		for i := range approvedCandidates {
			if approvedCandidates[i].SimilarityHash != "" {
				refreshedGroups[approvedCandidates[i].SimilarityHash] = struct{}{}
			}
		}
		for similarityHash := range refreshedGroups {
			if err := KnowledgeCandidateScoringService.RefreshDuplicateGroupDB(ctx.Tx, similarityHash); err != nil {
				return err
			}
		}
		content := "重新打开工单"
		if trimmedReason := strings.TrimSpace(reason); trimmedReason != "" {
			content += "，原因：" + trimmedReason
		}
		if err := s.addProgress(ctx.Tx, ticket.ID, enums.TicketProgressEventReopened, content, operator); err != nil {
			return err
		}
		reopenedEvent := events.TicketReopenedEvent{
			EventID:    ticketLifecycleEventID(events.FaultStatsEventTicketReopened, ticket.ID, ticket.UpdatedAt),
			TicketID:   ticket.ID,
			TenantID:   ticket.TenantID,
			OperatorID: operator.UserID,
			Reason:     strings.TrimSpace(reason),
			OccurredAt: ticket.UpdatedAt,
		}
		if _, err := eventbus.EnqueueTx(ctx.Tx, eventbus.DurableEvent{
			TenantID:       ticket.TenantID,
			IdempotencyKey: "tenant:" + strconv.FormatInt(ticket.TenantID, 10) + ":" + reopenedEvent.EventID,
			EventType:      events.FaultStatsEventTicketReopened,
			Payload:        reopenedEvent,
			Source:         "ticket_lifecycle_service",
			AggregateID:    strconv.FormatInt(ticket.ID, 10),
			ActorID:        strconv.FormatInt(operator.UserID, 10),
			ActorType:      "user",
			CreatedAt:      ticket.UpdatedAt,
		}); err != nil {
			return err
		}
		if ctx.RegisterCallback != nil {
			ctx.RegisterCallback(eventbus.WakeDefaultOutboxPublisher)
		}
		reopened = ticket
		return nil
	})
	if err != nil {
		return err
	}
	if reopened == nil {
		return nil
	}
	if knowledgeReassessmentRequired {
		KnowledgeIndexSyncService.ReconcileEntryTasks(20)
	}
	if reopened.ConversationID > 0 {
		if conversation := ConversationService.Get(reopened.ConversationID); conversation != nil {
			if reopenedMessage != nil {
				WsService.PublishMessageCreated(conversation, reopenedMessage)
			}
			WsService.PublishConversationChanged(conversation, enums.IMRealtimeEventConversationUpdated)
		}
		if _, dispatchErr := ConversationDispatchService.DispatchConversation(reopened.ConversationID); dispatchErr != nil {
			slog.Warn("dispatch reopened customer conversation failed",
				"ticket_id", reopened.ID,
				"conversation_id", reopened.ConversationID,
				"error", dispatchErr,
			)
		}
	}
	return nil
}

func (s *ticketLifecycleService) addProgress(tx *gorm.DB, ticketID int64, eventType enums.TicketProgressEventType, content string, operator *dto.AuthPrincipal) error {
	_, err := s.createProgressTx(tx, ticketID, eventType, content, operator)
	return err
}

// createProgressTx 写入工单进度并返回带主键的记录，供需要 progress.ID 作为
// 事件幂等键的调用方（如 Assign 发布 ticket.assigned）使用。
func (s *ticketLifecycleService) createProgressTx(tx *gorm.DB, ticketID int64, eventType enums.TicketProgressEventType, content string, operator *dto.AuthPrincipal) (*models.TicketProgress, error) {
	tenantID := operator.TenantID
	if ticket := repositories.TicketRepository.Get(tx, ticketID); ticket != nil {
		tenantID = ticket.TenantID
	}
	progress := &models.TicketProgress{
		TenantID:     tenantID,
		TicketID:     ticketID,
		EventType:    eventType,
		Content:      content,
		MetadataJSON: "{}",
		AuthorID:     operator.UserID,
		CreatedAt:    time.Now(),
	}
	if err := repositories.TicketProgressRepository.Create(tx, progress); err != nil {
		return nil, err
	}
	return progress, nil
}

func (s *ticketLifecycleService) buildRootCauseSummary(repairs []models.TicketRepairRecord) string {
	repair := latestVerifiedKnowledgeRepair(repairs)
	if repair == nil {
		return ""
	}
	return strings.TrimSpace(repair.RootCause)
}

func (s *ticketLifecycleService) buildSolutionSummary(repairs []models.TicketRepairRecord) string {
	repair := latestVerifiedKnowledgeRepair(repairs)
	if repair == nil {
		return ""
	}
	solutions := make([]string, 0, 2)
	if solution := strings.TrimSpace(repair.Solution); solution != "" {
		solutions = append(solutions, solution)
	}
	conclusion := strings.TrimSpace(repair.Conclusion)
	if conclusion != "" && !knowledgeSummaryContains(solutions, conclusion) {
		solutions = append(solutions, "维修结论："+conclusion)
	}
	return strings.Join(solutions, "; ")
}

func latestVerifiedKnowledgeRepair(repairs []models.TicketRepairRecord) *models.TicketRepairRecord {
	for i := len(repairs) - 1; i >= 0; i-- {
		if strings.EqualFold(strings.TrimSpace(repairs[i].TestResult), "passed") {
			return &repairs[i]
		}
	}
	for i := len(repairs) - 1; i >= 0; i-- {
		if strings.TrimSpace(repairs[i].RootCause) != "" || strings.TrimSpace(repairs[i].Solution) != "" || strings.TrimSpace(repairs[i].Conclusion) != "" {
			return &repairs[i]
		}
	}
	return nil
}

func knowledgeSummaryContains(parts []string, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	for _, part := range parts {
		if strings.Contains(part, value) {
			return true
		}
	}
	return false
}

func (s *ticketLifecycleService) buildQualityClue(ticket *models.Ticket, repairs []models.TicketRepairRecord, operator *dto.AuthPrincipal) *models.TicketQualityClue {
	if ticket == nil {
		return nil
	}
	// 检测质量线索：重复报修、低分诊断、处理时长过长等
	clueType := ""
	severity := "info"
	description := ""

	if ticket.DiagnosisSummary != "" {
		var summary map[string]any
		if err := json.Unmarshal([]byte(ticket.DiagnosisSummary), &summary); err == nil {
			if score, ok := summary["confidenceScore"].(float64); ok && score < 0.3 {
				clueType = "low_score"
				severity = "warning"
				description = "诊段置信度偏低，建议人工审核"
			}
		}
	}

	// 如果有多次维修记录，标记为重复报修
	if len(repairs) > 1 {
		clueType = "repeat_issue"
		severity = "warning"
		description = "工单存在多次维修记录，可能存在根因未彻底解决"
	}

	if clueType == "" {
		// 默认检查工单处理时长
		if ticket.CreatedAt.Add(72 * time.Hour).Before(time.Now()) {
			clueType = "long_duration"
			severity = "info"
			description = "工单处理超时72小时，建议关注"
		}
	}

	if clueType == "" {
		return nil
	}

	return &models.TicketQualityClue{
		TenantID:       ticket.TenantID,
		TicketID:       ticket.ID,
		ProductID:      ticket.ProductID,
		ProductModelID: ticket.ProductModelID,
		ClueType:       clueType,
		Severity:       severity,
		Description:    description,
		DetectedAt:     time.Now(),
		Status:         "open",
		AuditFields:    utils.BuildAuditFields(operator),
	}
}

package services

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TicketDispatchService 负责独立工单（无关联会话）的自动派单与限时接单回收。
// 与会话派单（ConversationDispatchService）按 conversation_id 划分职责：
// - conversation_id > 0 的工单由会话派单循环独占驱动（经 SyncConversationDispatchTx 回写）；
// - conversation_id = 0 的独立工单由本服务驱动。
// 两条路径最终都写入同一售后状态 pending_assignee_accept + 接单截止 + ticket.assigned 事件。
var TicketDispatchService = newTicketDispatchService()

func newTicketDispatchService() *ticketDispatchService {
	return &ticketDispatchService{}
}

type ticketDispatchService struct{}

var ticketDispatchRunning atomic.Bool

const (
	shortDispatchRetryDelay              = 5 * time.Minute
	longDispatchRetryDelay               = 15 * time.Minute
	acceptTimeoutRedispatchingCode       = "accept_timeout_redispatching"
	assigneeUnavailableRedispatchingCode = "assignee_unavailable_redispatching"
	linkedTicketRecoveryGracePeriod      = time.Minute
)

// unassignedTicketStatuses 返回参与自动派单扫描的工单状态集合。
// reopened 工单重新打开后若无负责人，同样需要重新派单。
func unassignedTicketStatuses() []enums.TicketStatus {
	return []enums.TicketStatus{
		enums.TicketStatusPending,
		enums.TicketStatusPendingAcceptance,
		enums.TicketStatusPendingDispatch,
		enums.TicketStatusReopened,
	}
}

func pendingAssigneeAcceptanceTicketStatuses() []enums.TicketStatus {
	return []enums.TicketStatus{
		enums.TicketStatusAssigned,
		enums.TicketStatusPendingAssigneeAccept,
	}
}

func isPendingAssigneeAcceptanceTicketStatus(status enums.TicketStatus) bool {
	return enums.NormalizeTicketStatus(string(status)) == enums.TicketStatusPendingAssigneeAccept
}

func isTicketAwaitingDispatch(ticket *models.Ticket) bool {
	if ticket == nil || ticket.TenantID <= 0 || ticket.CurrentAssigneeID > 0 || ticket.ConversationID > 0 {
		return false
	}
	for _, status := range unassignedTicketStatuses() {
		if ticket.Status == status {
			return true
		}
	}
	return false
}

func acceptDeadlineMinutes() int {
	return config.CurrentOrDefault().TicketDispatch.Normalized().AcceptDeadlineMinutes
}

func maxDispatchAttempts() int {
	return config.CurrentOrDefault().TicketDispatch.Normalized().MaxDispatchAttempts
}

func unassignedEscalationMinutes() int {
	return config.CurrentOrDefault().TicketDispatch.Normalized().UnassignedEscalationMinutes
}

// DispatchPendingTickets 扫描未分配独立工单并按等待时长优先自动派单。
// 这是 cron @every 30s 的入口；limit <= 0 时使用默认批量上限。
func (s *ticketDispatchService) DispatchPendingTickets(limit int) (int, error) {
	if !ticketDispatchRunning.CompareAndSwap(false, true) {
		return 0, nil
	}
	defer ticketDispatchRunning.Store(false)

	return withDispatchScanLock("remotehelpdesk:ticket_dispatch:pending_tickets", func() (int, error) {
		return s.dispatchPendingTicketsOnce(limit)
	})
}

func (s *ticketDispatchService) dispatchPendingTicketsOnce(limit int) (int, error) {
	if limit <= 0 {
		limit = pendingDispatchBatchLimit
	}
	now := time.Now()
	awaiting := unassignedTicketStatuses()
	// 只取扫描窗口内的候选（最老优先），无人可派时也不会逐条扫全表空转。
	tickets := repositories.TicketRepository.Find(sqls.DB(), sqls.NewCnd().
		In("status", awaiting).
		Eq("current_assignee_id", 0).
		Eq("conversation_id", 0).
		Gt("tenant_id", 0).
		Where("(dispatch_deferred_until IS NULL OR dispatch_deferred_until <= ?)", now).
		Asc("created_at").Asc("id").
		Limit(dispatchScanWindow(limit)))
	if len(tickets) == 0 {
		return 0, nil
	}

	dispatchedCount := 0
	var dispatchErrors []error
	for i := range tickets {
		if dispatchedCount >= limit {
			break
		}
		dispatched, err := s.dispatchTicket(&tickets[i], 0, now)
		if err != nil {
			dispatchErrors = append(dispatchErrors, err)
			continue
		}
		if dispatched != nil {
			dispatchedCount++
		}
	}
	if dispatchedCount > 0 {
		slog.Info("pending ticket dispatch scan completed",
			"dispatched_count", dispatchedCount,
			"scanned_count", len(tickets),
			"limit", limit,
		)
	}
	return dispatchedCount, errors.Join(dispatchErrors...)
}

func (s *ticketDispatchService) recoverUnclaimedLinkedConversationTickets(limit int) (int, error) {
	tickets, err := repositories.TicketRepository.FindUnclaimedLinkedConversationTickets(
		sqls.DB(),
		unassignedTicketStatuses(),
		time.Now().Add(-linkedTicketRecoveryGracePeriod),
		limit,
	)
	if err != nil {
		return 0, err
	}
	recovered := 0
	for i := range tickets {
		changed := false
		err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			locked := loadTicketForUpdate(ctx.Tx, tickets[i].ID)
			var err error
			changed, err = reconcileUnclaimedLinkedConversationTicketTx(ctx, locked, systemDispatchPrincipal(), time.Now())
			return err
		})
		if err != nil {
			return recovered, err
		}
		if changed {
			recovered++
		}
	}
	if recovered > 0 {
		slog.Warn("recovered linked ticket conversations into pending dispatch",
			"recovered_count", recovered,
		)
	}
	return recovered, nil
}

// DispatchPendingTicket 为单张独立工单触发一次自动派单（超时回收重派时复用）。
// 已分配/已有会话/非待派单状态时直接返回 nil，不报错。
func (s *ticketDispatchService) DispatchPendingTicket(ticketID int64) (*models.Ticket, error) {
	if ticketID <= 0 {
		return nil, nil
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil || !isTicketAwaitingDispatch(ticket) {
		return nil, nil
	}
	return s.dispatchTicket(ticket, 0, time.Now())
}

// dispatchTicket 为工单挑选候选人并尝试分配。excludeUserID 用于超时回收重派时
// 排除上一任负责人，避免同一人反复接到同一张工单。
// 无人可派时先限频告警；等待超过阈值后自动交由组长或服务经理兜底。
func (s *ticketDispatchService) dispatchTicket(ticket *models.Ticket, excludeUserID int64, now time.Time) (*models.Ticket, error) {
	return s.dispatchTicketWithExclusionReason(ticket, excludeUserID, "no_alternative_after_accept_timeout", now)
}

func (s *ticketDispatchService) dispatchTicketWithExclusionReason(ticket *models.Ticket, excludeUserID int64, noAlternativeReason string, now time.Time) (*models.Ticket, error) {
	teamID := resolveTicketDispatchTeamIDDB(sqls.DB(), ticket)
	if teamID <= 0 {
		slog.Info("ticket has no product repair team for auto dispatch",
			"ticket_id", ticket.ID, "tenant_id", ticket.TenantID, "product_id", ticket.ProductID)
		return s.handleUndispatchableTicket(ticket, "no_product_repair_team", now)
	}

	candidates, report, err := ConversationDispatchService.pickDispatchCandidates(ticket.TenantID, ticket.ProductID, []int64{teamID}, now)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		slog.Info("no dispatch candidate for pending ticket",
			"ticket_id", ticket.ID,
			"tenant_id", ticket.TenantID,
			"team_id", teamID,
			"reason", report.Reason,
			"matched_profiles", report.MatchedProfiles,
			"eligible_profiles", report.EligibleProfiles,
		)
		return s.handleUndispatchableTicket(ticket, report.Reason, now)
	}
	candidates, capabilityReason, err := filterDispatchCandidatesByTicketCapabilityDB(sqls.DB(), ticket, candidates)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		if strings.TrimSpace(capabilityReason) == "" {
			capabilityReason = "no_capability_match"
		}
		slog.Info("no capability matched dispatch candidate for pending ticket",
			"ticket_id", ticket.ID,
			"tenant_id", ticket.TenantID,
			"team_id", teamID,
			"reason", capabilityReason,
			"matched_profiles", report.MatchedProfiles,
			"eligible_profiles", report.EligibleProfiles,
		)
		return s.handleUndispatchableTicket(ticket, capabilityReason, now)
	}

	assignmentReason := dispatchAssignmentReason(report)
	for _, candidate := range candidates {
		if excludeUserID > 0 && candidate.profile.UserID == excludeUserID {
			continue
		}
		dispatched, err := s.tryAssignTicketTx(ticket.ID, candidate, assignmentReason, now)
		if err != nil {
			if errors.Is(err, errConversationDispatchCandidateUnavailable) {
				continue
			}
			if errors.Is(err, errConversationDispatchConflict) {
				return nil, nil
			}
			return nil, err
		}
		if dispatched != nil {
			slog.Info("ticket auto dispatched",
				"ticket_id", dispatched.ID,
				"ticket_no", dispatched.TicketNo,
				"assignee_id", dispatched.CurrentAssigneeID,
				"team_id", dispatched.CurrentTeamID,
				"accept_deadline_at", zeroTime(dispatched.AcceptDeadlineAt),
			)
			return dispatched, nil
		}
	}
	reason := "candidate_became_unavailable"
	if excludeUserID > 0 {
		reason = strings.TrimSpace(noAlternativeReason)
		if reason == "" {
			reason = "no_alternative_after_accept_timeout"
		}
	}
	return s.handleUndispatchableTicket(ticket, reason, now)
}

func resolveTicketDispatchTeamIDDB(db *gorm.DB, ticket *models.Ticket) int64 {
	if ticket == nil || ticket.TenantID <= 0 {
		return 0
	}
	if db == nil {
		db = sqls.DB()
	}
	if ticket.CurrentTeamID > 0 {
		if db == nil || !db.Migrator().HasTable(&models.AgentTeam{}) {
			return ticket.CurrentTeamID
		}
		team := repositories.AgentTeamRepository.Get(db, ticket.CurrentTeamID)
		if team != nil && team.TenantID == ticket.TenantID && team.Status == enums.StatusOk &&
			(ticket.ProductID <= 0 || team.ProductID <= 0 || team.ProductID == ticket.ProductID) {
			return team.ID
		}
	}
	if ticket.ProductID > 0 {
		if team := ProductSupportOrganizationService.FindProductRepairTeam(db, ticket.TenantID, ticket.ProductID); team != nil {
			return team.ID
		}
	}
	return 0
}

func (s *ticketDispatchService) handleUndispatchableTicket(ticket *models.Ticket, reason string, now time.Time) (*models.Ticket, error) {
	if ticket == nil {
		return nil, nil
	}
	reason = normalizeDispatchFailureReason(reason)
	waitStartedAt := ticketDispatchWaitStartedAt(ticket)
	waitDeadline := waitStartedAt.Add(time.Duration(unassignedEscalationMinutes()) * time.Minute)
	if waitStartedAt.IsZero() || now.Before(waitDeadline) {
		if err := s.deferUndispatchableTicket(ticket, reason, now); err != nil {
			return nil, err
		}
		s.notifyUndispatchableTicket(ticket, reason, now)
		return nil, nil
	}
	return s.EscalateToSupervisor(ticket.ID, ticket.CurrentAssigneeID, "持续无人可派，主管兜底接管："+reason, now)
}

func ticketDispatchWaitStartedAt(ticket *models.Ticket) time.Time {
	if ticket == nil {
		return time.Time{}
	}
	waitStartedAt := ticket.CreatedAt
	db := sqls.DB()
	if db == nil || !db.Migrator().HasTable(&models.TicketProgress{}) {
		return waitStartedAt
	}
	latestReopen := repositories.TicketProgressRepository.FindOne(db, sqls.NewCnd().
		Eq("ticket_id", ticket.ID).
		Eq("event_type", enums.TicketProgressEventReopened).
		Desc("created_at").Desc("id"))
	if latestReopen != nil && latestReopen.CreatedAt.After(waitStartedAt) {
		return latestReopen.CreatedAt
	}
	return waitStartedAt
}

func (s *ticketDispatchService) notifyUndispatchableTicket(ticket *models.Ticket, reason string, now time.Time) {
	if ticket == nil || ticket.ID <= 0 {
		return
	}
	reason = normalizeDispatchFailureReason(reason)
	bucket := now.Truncate(15 * time.Minute).Unix()
	key := "ticket.dispatch_failure:" + strconv.FormatInt(ticket.ID, 10) + ":" + strconv.FormatInt(bucket, 10)
	_ = NotifySupervisors(ticket, "工单暂无人可派",
		"工单 "+ticket.TicketNo+" 无可用工程师，可能原因："+reason, "ticket_dispatch_failure", key)
}

func (s *ticketDispatchService) deferUndispatchableTicket(ticket *models.Ticket, reason string, now time.Time) error {
	if ticket == nil || ticket.ID <= 0 {
		return nil
	}
	deferredUntil := now.Add(dispatchRetryDelay(reason))
	operator := systemDispatchPrincipal()
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		locked := loadTicketForUpdate(ctx.Tx, ticket.ID)
		if locked == nil || locked.CurrentAssigneeID != 0 || !isUnassignedTicketStatus(locked.Status) {
			return nil
		}
		teamID := resolveTicketDispatchTeamIDDB(ctx.Tx, locked)
		return s.deferUndispatchableTicketTx(ctx.Tx, locked, teamID, reason, deferredUntil, operator, now)
	})
}

func (s *ticketDispatchService) deferConversationTicketTx(db *gorm.DB, conversationID, teamID int64, reason string, now time.Time) (*models.Ticket, error) {
	if db == nil || conversationID <= 0 {
		return nil, nil
	}
	var ticket models.Ticket
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("conversation_id = ? AND status NOT IN ?", conversationID, []enums.TicketStatus{
		enums.TicketStatusClosed,
		enums.TicketStatusDone,
		enums.TicketStatusCancelled,
	}).Order("id DESC").First(&ticket).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if ticket.CurrentAssigneeID != 0 || !isUnassignedTicketStatus(ticket.Status) {
		return nil, nil
	}
	if teamID <= 0 {
		teamID = resolveTicketDispatchTeamIDDB(db, &ticket)
	}
	reason = normalizeDispatchFailureReason(reason)
	if ticket.Status == enums.TicketStatusPendingDispatch &&
		normalizeDispatchFailureReason(ticket.LastDispatchFailureReason) == reason &&
		ticket.DispatchDeferredUntil != nil && ticket.DispatchDeferredUntil.After(now) {
		return nil, nil
	}
	deferredUntil := now.Add(dispatchRetryDelay(reason))
	operator := systemDispatchPrincipal()
	if err := s.deferUndispatchableTicketTx(db, &ticket, teamID, reason, deferredUntil, operator, now); err != nil {
		return nil, err
	}
	ticket.Status = enums.TicketStatusPendingDispatch
	ticket.CurrentTeamID = teamID
	ticket.CurrentAssigneeID = 0
	ticket.AssignedAt = nil
	ticket.AcceptedAt = nil
	ticket.AcceptDeadlineAt = nil
	ticket.DispatchDeferredUntil = &deferredUntil
	ticket.LastDispatchFailureReason = reason
	return &ticket, nil
}

func (s *ticketDispatchService) deferUndispatchableTicketTx(
	db *gorm.DB,
	ticket *models.Ticket,
	teamID int64,
	reason string,
	deferredUntil time.Time,
	operator *dto.AuthPrincipal,
	now time.Time,
) error {
	if db == nil || ticket == nil || ticket.ID <= 0 {
		return nil
	}
	reason = normalizeDispatchFailureReason(reason)
	updates := map[string]any{
		"status":                       enums.TicketStatusPendingDispatch,
		"current_assignee_id":          0,
		"assigned_at":                  nil,
		"accepted_at":                  nil,
		"accept_deadline_at":           nil,
		"dispatch_deferred_until":      deferredUntil,
		"last_dispatch_failure_reason": reason,
		"updated_at":                   now,
		"update_user_id":               auditOperatorID(operator),
		"update_user_name":             auditOperatorName(operator),
	}
	if teamID > 0 && ticket.CurrentTeamID != teamID {
		updates["current_team_id"] = teamID
	}
	if err := repositories.TicketRepository.Updates(db, ticket.ID, updates); err != nil {
		return err
	}
	return createFailedTicketDispatchAttemptTx(db, ticket, teamID, reason, operator, now)
}

func dispatchRetryDelay(reason string) time.Duration {
	switch normalizeDispatchFailureReason(reason) {
	case "outside_personal_dispatch_rule", "no_product_repair_team", "no_matched_profile", "no_profile_for_enabled_user":
		return longDispatchRetryDelay
	default:
		return shortDispatchRetryDelay
	}
}

func normalizeDispatchFailureReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "unknown"
	}
	runes := []rune(reason)
	if len(runes) > 64 {
		return string(runes[:64])
	}
	return reason
}

// EscalateToSupervisor 将仍处于预期负责人状态的未完成工单交给主管兜底。
// expectedAssigneeID 用于避免扫描期间人工指派或接单后被后台任务覆盖。
func (s *ticketDispatchService) EscalateToSupervisor(ticketID, expectedAssigneeID int64, reason string, now time.Time) (*models.Ticket, error) {
	if ticketID <= 0 {
		return nil, nil
	}
	snapshot := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if snapshot == nil || snapshot.CurrentAssigneeID != expectedAssigneeID || !canAssignTicketStatus(snapshot.Status) {
		return nil, nil
	}
	supervisorScope := *snapshot
	excludedAssigneeID := snapshot.CurrentAssigneeID
	if excludedAssigneeID <= 0 {
		attempt, err := repositories.TicketDispatchAttemptRepository.FindLatestFailedAssignee(sqls.DB(), snapshot.ID, []string{
			ticketDispatchOutcomeTimedOut,
			ticketDispatchOutcomeSuperseded,
		})
		if err == nil {
			excludedAssigneeID = attempt.AssigneeID
			supervisorScope.CurrentAssigneeID = excludedAssigneeID
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	supervisorID := ResolveSupervisorOwner(&supervisorScope)
	if supervisorID <= 0 {
		deferReason := "missing_supervisor"
		if excludedAssigneeID > 0 {
			switch normalizeDispatchFailureReason(snapshot.LastDispatchFailureReason) {
			case assigneeUnavailableRedispatchingCode, "no_alternative_after_assignee_unavailable":
				deferReason = "no_alternative_after_assignee_unavailable"
			default:
				deferReason = "no_alternative_after_accept_timeout"
			}
		}
		if err := s.deferUndispatchableTicket(snapshot, deferReason, now); err != nil {
			return nil, err
		}
		bucket := now.Truncate(15 * time.Minute).Unix()
		key := "ticket.supervisor_missing:" + strconv.FormatInt(ticketID, 10) + ":" + strconv.FormatInt(bucket, 10)
		content := "工单 " + snapshot.TicketNo + " 已达到兜底条件，但未找到可接管负责人"
		if excludedAssigneeID > 0 {
			content = "工单 " + snapshot.TicketNo + " 的原负责人已超时或不可用，且暂无其他可接管负责人"
		}
		_ = NotifySupervisors(snapshot, "工单需要主管兜底", content, "ticket_supervisor_missing", key)
		return nil, nil
	}
	operator := systemDispatchPrincipal()
	escalated := false
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		locked := loadTicketForUpdate(ctx.Tx, ticketID)
		if locked == nil || locked.CurrentAssigneeID != expectedAssigneeID || !canAssignTicketStatus(locked.Status) {
			return nil
		}
		teamID := resolveTicketDispatchTeamIDDB(ctx.Tx, locked)
		updates := map[string]any{
			"status":                       enums.TicketStatusProcessing,
			"current_team_id":              teamID,
			"current_assignee_id":          supervisorID,
			"assigned_at":                  now,
			"accepted_at":                  now,
			"accept_deadline_at":           nil,
			"dispatch_deferred_until":      nil,
			"last_dispatch_failure_reason": "",
			"updated_at":                   now,
			"update_user_id":               operator.UserID,
			"update_user_name":             operator.Username,
		}
		if err := repositories.TicketRepository.Updates(ctx.Tx, locked.ID, updates); err != nil {
			return err
		}
		if locked.ConversationID > 0 {
			conversation := repositories.ConversationRepository.Get(ctx.Tx, locked.ConversationID)
			if conversation != nil && conversation.CurrentAssigneeID == expectedAssigneeID {
				if err := repositories.ConversationRepository.Updates(ctx.Tx, conversation.ID, map[string]any{
					"status":              enums.IMConversationStatusActive,
					"current_assignee_id": supervisorID,
					"updated_at":          now,
					"update_user_id":      operator.UserID,
					"update_user_name":    operator.Username,
				}); err != nil {
					return err
				}
			}
		}
		if err := finishPendingTicketDispatchAttemptTx(ctx.Tx, locked.ID, ticketDispatchOutcomeEscalated, nil, now); err != nil {
			return err
		}
		if err := createFinishedTicketDispatchAttemptTx(ctx.Tx, locked, teamID, supervisorID, ticketDispatchOutcomeEscalated, reason, operator, now); err != nil {
			return err
		}
		progress := &models.TicketProgress{
			TenantID: locked.TenantID, TicketID: locked.ID, EventType: enums.TicketProgressEventEscalated,
			Content: reason, MetadataJSON: "{}", AuthorID: operator.UserID, CreatedAt: now,
		}
		if err := repositories.TicketProgressRepository.Create(ctx.Tx, progress); err != nil {
			return err
		}
		if _, err := enqueueTicketAssignedEventTx(ctx.Tx, locked, expectedAssigneeID, supervisorID, reason, operator, progress.ID, now); err != nil {
			return err
		}
		if ctx.RegisterCallback != nil {
			ctx.RegisterCallback(eventbus.WakeDefaultOutboxPublisher)
		}
		escalated = true
		return nil
	})
	if err != nil || !escalated {
		return nil, err
	}
	key := "ticket.supervisor_takeover:" + strconv.FormatInt(ticketID, 10) + ":" + strconv.FormatInt(supervisorID, 10)
	_ = NotifySupervisors(snapshot, "工单已由主管兜底接管",
		"工单 "+snapshot.TicketNo+" 长时间无人响应，已自动交由主管处理", "ticket_supervisor_takeover", key)
	return repositories.TicketRepository.Get(sqls.DB(), ticketID), nil
}

// tryAssignTicketTx 在事务内把独立工单派给候选人：写入 pending_assignee_accept、
// 接单截止时间，并记录派单进度与 ticket.assigned 事件。
func (s *ticketDispatchService) tryAssignTicketTx(ticketID int64, candidate dispatchCandidate, reason string, now time.Time) (*models.Ticket, error) {
	operator := systemDispatchPrincipal()
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = dispatchReasonAutomatic
	}

	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		ticket := loadTicketForUpdate(ctx.Tx, ticketID)
		if ticket == nil {
			return errConversationDispatchConflict
		}
		if !isTicketAwaitingDispatch(ticket) {
			return errConversationDispatchConflict
		}
		profile, err := ConversationDispatchService.lockAndValidateDispatchCandidateAt(ctx.Tx, candidate, ticket.TenantID, ticket.ProductID, now)
		if err != nil {
			return err
		}
		if err := validateTicketDispatchCapabilityDB(ctx.Tx, ticket, profile.UserID); err != nil {
			return err
		}

		updates := map[string]any{
			"status":              enums.TicketStatusPendingAssigneeAccept,
			"current_team_id":     candidate.teamID,
			"current_assignee_id": profile.UserID,
			"updated_at":          now,
			"update_user_id":      operator.UserID,
			"update_user_name":    operator.Username,
		}
		addTicketAssignmentTrackingForTicketDB(updates, ctx.Tx, ticket, now)
		result := ctx.Tx.Model(&models.Ticket{}).
			Where("id = ? AND current_assignee_id = ? AND conversation_id = ?", ticketID, 0, 0).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errConversationDispatchConflict
		}

		progress := &models.TicketProgress{
			TenantID:     ticket.TenantID,
			TicketID:     ticket.ID,
			EventType:    enums.TicketProgressEventAssigned,
			Content:      "工单" + reason + " -> 等待工程师接单",
			MetadataJSON: "{}",
			AuthorID:     operator.UserID,
			CreatedAt:    now,
		}
		if err := repositories.TicketProgressRepository.Create(ctx.Tx, progress); err != nil {
			return err
		}
		if err := createTicketDispatchAttemptTx(ctx.Tx, ticket, candidate.teamID, profile.UserID, reason, operator, now); err != nil {
			return err
		}
		if _, err := enqueueTicketAssignedEventTx(ctx.Tx, ticket, 0, profile.UserID, reason, operator, progress.ID, now); err != nil {
			return err
		}
		if ctx.RegisterCallback != nil {
			ctx.RegisterCallback(eventbus.WakeDefaultOutboxPublisher)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return repositories.TicketRepository.Get(sqls.DB(), ticketID), nil
}

// RecoverExpiredTicketAcceptances 每分钟扫描接单超时的工单：
// 未达重派上限则回收并立即重派（排除上一任负责人）；达到上限则升级主管并停止派单。
func (s *ticketDispatchService) RecoverExpiredTicketAcceptances(limit int) (int, error) {
	return withDispatchScanLock("remotehelpdesk:ticket_dispatch:acceptance_recovery", func() (int, error) {
		return s.recoverExpiredTicketAcceptancesOnce(limit)
	})
}

func (s *ticketDispatchService) recoverExpiredTicketAcceptancesOnce(limit int) (int, error) {
	if limit <= 0 {
		limit = pendingDispatchBatchLimit
	}
	now := time.Now()
	expired := repositories.TicketRepository.Find(sqls.DB(), sqls.NewCnd().
		Where("status NOT IN ?", []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled}).
		Gt("current_assignee_id", 0).
		Where("accept_deadline_at IS NOT NULL").
		Where("accept_deadline_at <= ?", now).
		Asc("accept_deadline_at").
		Asc("id").
		Limit(dispatchScanWindow(limit)))
	if len(expired) == 0 {
		return 0, nil
	}

	recovered := 0
	for i := range expired {
		if recovered >= limit {
			break
		}
		ticket := &expired[i]
		oldAssigneeID := ticket.CurrentAssigneeID

		nextAttempt := ticket.DispatchAttempts + 1
		if nextAttempt >= maxDispatchAttempts() {
			escalated, err := s.escalateTicketAfterRepeatedTimeout(ticket, oldAssigneeID, nextAttempt, now, false)
			if err != nil {
				slog.Warn("escalate ticket after accept timeout failed", "ticket_id", ticket.ID, "error", err)
				continue
			}
			if escalated {
				recovered++
			}
			continue
		}
		recycled, err := s.recycleTicketAfterAcceptTimeout(ticket, oldAssigneeID, nextAttempt, now, false)
		if err != nil {
			slog.Warn("recycle ticket after accept timeout failed", "ticket_id", ticket.ID, "error", err)
			continue
		}
		if !recycled {
			continue
		}
		recovered++
		if ticket.ConversationID > 0 {
			if _, err := ConversationDispatchService.DispatchConversationExcluding(ticket.ConversationID, oldAssigneeID); err != nil {
				slog.Warn("redispatch conversation after ticket accept timeout failed", "ticket_id", ticket.ID, "conversation_id", ticket.ConversationID, "error", err)
			}
			continue
		}
		refreshed := repositories.TicketRepository.Get(sqls.DB(), ticket.ID)
		if refreshed != nil {
			if _, err := s.dispatchTicket(refreshed, oldAssigneeID, now); err != nil {
				slog.Warn("redispatch ticket after accept timeout failed", "ticket_id", ticket.ID, "error", err)
			}
		}
	}
	return recovered, nil
}

// RecoverTicketAssignmentSLA applies the same recycle/escalation policy to one
// ticket when its assignment SLA is at risk or already breached.
func (s *ticketDispatchService) RecoverTicketAssignmentSLA(ticketID int64, now time.Time) (bool, error) {
	if ticketID <= 0 {
		return false, nil
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil || ticket.AcceptedAt != nil || !canAssignTicketStatus(ticket.Status) {
		return false, nil
	}
	if ticket.CurrentAssigneeID <= 0 {
		if ticket.ConversationID > 0 {
			dispatched, err := ConversationDispatchService.DispatchConversation(ticket.ConversationID)
			return dispatched != nil, err
		}
		dispatched, err := s.DispatchPendingTicket(ticket.ID)
		return dispatched != nil, err
	}

	oldAssigneeID := ticket.CurrentAssigneeID
	nextAttempt := ticket.DispatchAttempts + 1
	if nextAttempt >= maxDispatchAttempts() {
		return s.escalateTicketAfterRepeatedTimeout(ticket, oldAssigneeID, nextAttempt, now, true)
	}
	recycled, err := s.recycleTicketAfterAcceptTimeout(ticket, oldAssigneeID, nextAttempt, now, true)
	if err != nil || !recycled {
		return recycled, err
	}
	if ticket.ConversationID > 0 {
		_, err := ConversationDispatchService.DispatchConversationExcluding(ticket.ConversationID, oldAssigneeID)
		return true, err
	}
	refreshed := repositories.TicketRepository.Get(sqls.DB(), ticket.ID)
	if refreshed == nil {
		return true, nil
	}
	_, err = s.dispatchTicket(refreshed, oldAssigneeID, now)
	return true, err
}

// RecoverUnavailableAssigneeAssignments immediately returns pending acceptance
// tickets to the dispatch pool when their current assignee becomes unavailable.
// It reuses the normal excluded redispatch path so the same engineer is not
// selected again while their work status is busy, leave, offline, or custom.
func (s *ticketDispatchService) RecoverUnavailableAssigneeAssignments(tenantID, assigneeID int64, now time.Time) (int, error) {
	if tenantID <= 0 || assigneeID <= 0 {
		return 0, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	lockName := "remotehelpdesk:ticket_dispatch:assignee_unavailable:" +
		strconv.FormatInt(tenantID, 10) + ":" + strconv.FormatInt(assigneeID, 10)
	return withDispatchScanLock(lockName, func() (int, error) {
		return s.recoverUnavailableAssigneeAssignmentsOnce(tenantID, assigneeID, now)
	})
}

func (s *ticketDispatchService) recoverUnavailableAssigneeAssignmentsOnce(tenantID, assigneeID int64, now time.Time) (int, error) {
	tickets := repositories.TicketRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("current_assignee_id", assigneeID).
		In("status", pendingAssigneeAcceptanceTicketStatuses()).
		Where("accepted_at IS NULL").
		Asc("assigned_at").
		Asc("id"))
	if len(tickets) == 0 {
		return 0, nil
	}

	recovered := 0
	var recoverErrors []error
	for i := range tickets {
		ticket := &tickets[i]
		recycled, err := s.recoverPendingAssigneeTicket(ticket, assigneeID, now, "负责人已切换为不可接单状态，原派单已回到待派池，无需继续处理。", false)
		if err != nil {
			recoverErrors = append(recoverErrors, err)
			slog.Warn("recover ticket after assignee became unavailable failed",
				"ticket_id", ticket.ID,
				"tenant_id", tenantID,
				"assignee_id", assigneeID,
				"error", err,
			)
			continue
		}
		if !recycled {
			continue
		}
		recovered++
	}
	return recovered, errors.Join(recoverErrors...)
}

// RecoverIneligibleTicketAcceptances scans pending automatic assignments whose
// assignee no longer satisfies team membership, team dispatch, or approved-leave rules.
func (s *ticketDispatchService) RecoverIneligibleTicketAcceptances(limit int) (int, error) {
	return withDispatchScanLock("remotehelpdesk:ticket_dispatch:ineligible_acceptance_recovery", func() (int, error) {
		return s.recoverIneligibleTicketAcceptancesOnce(limit)
	})
}

func (s *ticketDispatchService) recoverIneligibleTicketAcceptancesOnce(limit int) (int, error) {
	if limit <= 0 {
		limit = pendingDispatchBatchLimit
	}
	now := time.Now()
	tickets := repositories.TicketRepository.Find(sqls.DB(), sqls.NewCnd().
		In("status", pendingAssigneeAcceptanceTicketStatuses()).
		Gt("current_assignee_id", 0).
		Where("accepted_at IS NULL").
		Asc("accept_deadline_at").
		Asc("id").
		Limit(dispatchScanWindow(limit)))
	if len(tickets) == 0 {
		return 0, nil
	}

	recovered := 0
	var recoverErrors []error
	for i := range tickets {
		if recovered >= limit {
			break
		}
		ticket := &tickets[i]
		oldAssigneeID := ticket.CurrentAssigneeID
		if validateAutomaticTicketAssigneeEligibilityDB(sqls.DB(), ticket, oldAssigneeID, ticket.CurrentTeamID, now) == nil {
			continue
		}
		recycled, err := s.recoverPendingAssigneeTicket(ticket, oldAssigneeID, now, "负责人已不符合当前排班或派单资格，原派单已回到待派池，无需继续处理。", false)
		if err != nil {
			recoverErrors = append(recoverErrors, err)
			slog.Warn("recover ticket after assignee became ineligible failed",
				"ticket_id", ticket.ID,
				"tenant_id", ticket.TenantID,
				"assignee_id", oldAssigneeID,
				"error", err,
			)
			continue
		}
		if !recycled {
			continue
		}
		recovered++
	}
	return recovered, errors.Join(recoverErrors...)
}

func (s *ticketDispatchService) RecoverTeamMemberIneligibleAssignments(tenantID, teamID, assigneeID int64, now time.Time) (int, error) {
	if tenantID <= 0 || teamID <= 0 || assigneeID <= 0 {
		return 0, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	lockName := "remotehelpdesk:ticket_dispatch:team_member_ineligible:" +
		strconv.FormatInt(tenantID, 10) + ":" +
		strconv.FormatInt(teamID, 10) + ":" +
		strconv.FormatInt(assigneeID, 10)
	return withDispatchScanLock(lockName, func() (int, error) {
		tickets := repositories.TicketRepository.Find(sqls.DB(), sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("current_team_id", teamID).
			Eq("current_assignee_id", assigneeID).
			In("status", pendingAssigneeAcceptanceTicketStatuses()).
			Where("accepted_at IS NULL").
			Asc("assigned_at").
			Asc("id"))
		return s.recoverIneligibleTicketAssignments(tickets, now, "负责人已不符合当前团队派单资格，原派单已回到待派池，无需继续处理。")
	})
}

func (s *ticketDispatchService) RecoverTeamIneligibleAssignments(tenantID, teamID int64, now time.Time) (int, error) {
	if tenantID <= 0 || teamID <= 0 {
		return 0, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	lockName := "remotehelpdesk:ticket_dispatch:team_ineligible:" +
		strconv.FormatInt(tenantID, 10) + ":" +
		strconv.FormatInt(teamID, 10)
	return withDispatchScanLock(lockName, func() (int, error) {
		tickets := repositories.TicketRepository.Find(sqls.DB(), sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("current_team_id", teamID).
			In("status", pendingAssigneeAcceptanceTicketStatuses()).
			Gt("current_assignee_id", 0).
			Where("accepted_at IS NULL").
			Asc("assigned_at").
			Asc("id"))
		return s.recoverIneligibleTicketAssignments(tickets, now, "负责人已不符合当前团队派单资格，原派单已回到待派池，无需继续处理。")
	})
}

func (s *ticketDispatchService) RecoverDisabledTeamAssignments(tenantID, teamID int64, now time.Time) (int, error) {
	if tenantID <= 0 || teamID <= 0 {
		return 0, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	lockName := "remotehelpdesk:ticket_dispatch:team_disabled:" +
		strconv.FormatInt(tenantID, 10) + ":" +
		strconv.FormatInt(teamID, 10)
	return withDispatchScanLock(lockName, func() (int, error) {
		tickets := repositories.TicketRepository.Find(sqls.DB(), sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("current_team_id", teamID).
			In("status", pendingAssigneeAcceptanceTicketStatuses()).
			Gt("current_assignee_id", 0).
			Where("accepted_at IS NULL").
			Asc("assigned_at").
			Asc("id"))
		recoveredAssigned, assignedErr := s.recoverIneligibleTicketAssignmentsWithOptions(tickets, now, "负责人所在团队已停用，原派单已回到待派池，无需继续处理。", true)
		releasedPool, poolErr := s.releaseDisabledTeamTicketPool(tenantID, teamID, now)
		return recoveredAssigned + releasedPool, errors.Join(assignedErr, poolErr)
	})
}

func (s *ticketDispatchService) releaseDisabledTeamTicketPool(tenantID, teamID int64, now time.Time) (int, error) {
	tickets := repositories.TicketRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("current_team_id", teamID).
		Eq("current_assignee_id", 0).
		In("status", unassignedTicketStatuses()).
		Asc("created_at").
		Asc("id"))
	if len(tickets) == 0 {
		return 0, nil
	}
	released := 0
	var releaseErrors []error
	for i := range tickets {
		ok, err := s.releaseDisabledTeamTicketPoolItem(tickets[i].ID, teamID, now)
		if err != nil {
			releaseErrors = append(releaseErrors, err)
			slog.Warn("release ticket from disabled team pool failed",
				"ticket_id", tickets[i].ID,
				"tenant_id", tenantID,
				"team_id", teamID,
				"error", err,
			)
			continue
		}
		if ok {
			released++
		}
	}
	return released, errors.Join(releaseErrors...)
}

func (s *ticketDispatchService) releaseDisabledTeamTicketPoolItem(ticketID, teamID int64, now time.Time) (bool, error) {
	operator := systemDispatchPrincipal()
	released := false
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		locked := loadTicketForUpdate(ctx.Tx, ticketID)
		if locked == nil || locked.CurrentTeamID != teamID || locked.CurrentAssigneeID > 0 || !isUnassignedTicketStatus(locked.Status) {
			return nil
		}
		if err := repositories.TicketRepository.Updates(ctx.Tx, locked.ID, map[string]any{
			"current_team_id":              0,
			"dispatch_deferred_until":      nil,
			"last_dispatch_failure_reason": "team_disabled_pool_released",
			"updated_at":                   now,
			"update_user_id":               operator.UserID,
			"update_user_name":             operator.Username,
		}); err != nil {
			return err
		}
		if locked.ConversationID > 0 {
			conversation := repositories.ConversationRepository.Get(ctx.Tx, locked.ConversationID)
			if conversation != nil && conversation.CurrentTeamID == teamID && conversation.CurrentAssigneeID == 0 {
				if err := repositories.ConversationRepository.Updates(ctx.Tx, conversation.ID, map[string]any{
					"current_team_id":  0,
					"updated_at":       now,
					"update_user_id":   operator.UserID,
					"update_user_name": operator.Username,
				}); err != nil {
					return err
				}
			}
		}
		progress := &models.TicketProgress{
			TenantID:     locked.TenantID,
			TicketID:     locked.ID,
			EventType:    enums.TicketProgressEventAssigned,
			Content:      "团队已停用，工单回到全局待派池",
			MetadataJSON: "{}",
			AuthorID:     operator.UserID,
			CreatedAt:    now,
		}
		if err := repositories.TicketProgressRepository.Create(ctx.Tx, progress); err != nil {
			return err
		}
		released = true
		return nil
	})
	return released, err
}

func isUnassignedTicketStatus(status enums.TicketStatus) bool {
	for _, item := range unassignedTicketStatuses() {
		if status == item {
			return true
		}
	}
	return false
}

func (s *ticketDispatchService) recoverIneligibleTicketAssignments(tickets []models.Ticket, now time.Time, notificationReason string) (int, error) {
	return s.recoverIneligibleTicketAssignmentsWithOptions(tickets, now, notificationReason, false)
}

func (s *ticketDispatchService) recoverIneligibleTicketAssignmentsWithOptions(tickets []models.Ticket, now time.Time, notificationReason string, clearTeam bool) (int, error) {
	if len(tickets) == 0 {
		return 0, nil
	}
	recovered := 0
	var recoverErrors []error
	for i := range tickets {
		ticket := &tickets[i]
		oldAssigneeID := ticket.CurrentAssigneeID
		if oldAssigneeID <= 0 {
			continue
		}
		if validateAutomaticTicketAssigneeEligibilityDB(sqls.DB(), ticket, oldAssigneeID, ticket.CurrentTeamID, now) == nil {
			continue
		}
		recycled, err := s.recoverPendingAssigneeTicket(ticket, oldAssigneeID, now, notificationReason, clearTeam)
		if err != nil {
			recoverErrors = append(recoverErrors, err)
			slog.Warn("recover ticket after team assignment became ineligible failed",
				"ticket_id", ticket.ID,
				"tenant_id", ticket.TenantID,
				"team_id", ticket.CurrentTeamID,
				"assignee_id", oldAssigneeID,
				"error", err,
			)
			continue
		}
		if recycled {
			recovered++
		}
	}
	return recovered, errors.Join(recoverErrors...)
}

func validateAutomaticTicketAssigneeEligibilityDB(db *gorm.DB, ticket *models.Ticket, assigneeID, teamID int64, now time.Time) error {
	if err := validateAssignedTicketAcceptanceDB(db, ticket, assigneeID, teamID, now); err != nil {
		return err
	}
	if teamID > 0 && db != nil && db.Migrator().HasTable(&models.AgentTeamMember{}) &&
		!AgentTeamMemberService.IsUserDispatchEnabledMemberOfTeamDB(db, ticket.TenantID, teamID, assigneeID) {
		return errors.New("assignee is not enabled for automatic team dispatch")
	}
	return validateAutoAssigneeLeaveAvailabilityDB(db, ticket.TenantID, assigneeID, now)
}

func (s *ticketDispatchService) recoverPendingAssigneeTicket(ticket *models.Ticket, oldAssigneeID int64, now time.Time, notificationReason string, clearTeam bool) (bool, error) {
	recycled, err := s.recycleTicketAfterAssigneeUnavailableWithOptions(ticket, oldAssigneeID, now, clearTeam)
	if err != nil || !recycled {
		return recycled, err
	}
	var recoverErrors []error
	if err := NotificationService.ReconcileTicketAssignedAfterRecovery(ticket, oldAssigneeID, notificationReason); err != nil {
		recoverErrors = append(recoverErrors, err)
		slog.Warn("reconcile recovered ticket assignment notification failed",
			"ticket_id", ticket.ID,
			"tenant_id", ticket.TenantID,
			"assignee_id", oldAssigneeID,
			"error", err,
		)
	}
	if clearTeam {
		return true, errors.Join(recoverErrors...)
	}
	if ticket.ConversationID > 0 {
		if _, err := ConversationDispatchService.DispatchConversationExcludingForReason(ticket.ConversationID, oldAssigneeID, "no_alternative_after_assignee_unavailable"); err != nil {
			recoverErrors = append(recoverErrors, err)
			slog.Warn("redispatch conversation after assignee became unavailable failed",
				"ticket_id", ticket.ID,
				"conversation_id", ticket.ConversationID,
				"assignee_id", oldAssigneeID,
				"error", err,
			)
		}
		return true, errors.Join(recoverErrors...)
	}
	refreshed := repositories.TicketRepository.Get(sqls.DB(), ticket.ID)
	if refreshed != nil {
		if _, err := s.dispatchTicketWithExclusionReason(refreshed, oldAssigneeID, "no_alternative_after_assignee_unavailable", now); err != nil {
			recoverErrors = append(recoverErrors, err)
			slog.Warn("redispatch ticket after assignee became unavailable failed",
				"ticket_id", ticket.ID,
				"assignee_id", oldAssigneeID,
				"error", err,
			)
		}
	}
	return true, errors.Join(recoverErrors...)
}

func (s *ticketDispatchService) recycleTicketAfterAssigneeUnavailable(ticket *models.Ticket, oldAssigneeID int64, now time.Time) (bool, error) {
	return s.recycleTicketAfterAssigneeUnavailableWithOptions(ticket, oldAssigneeID, now, false)
}

func (s *ticketDispatchService) recycleTicketAfterAssigneeUnavailableWithOptions(ticket *models.Ticket, oldAssigneeID int64, now time.Time, clearTeam bool) (bool, error) {
	if ticket == nil || ticket.ID <= 0 || oldAssigneeID <= 0 {
		return false, nil
	}
	operator := systemDispatchPrincipal()
	recycled := false
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		locked := loadTicketForUpdate(ctx.Tx, ticket.ID)
		if locked == nil || locked.CurrentAssigneeID != oldAssigneeID || locked.AcceptedAt != nil || !isPendingAssigneeAcceptanceTicketStatus(locked.Status) {
			return nil
		}
		redispatchDeferredUntil := now.Add(shortDispatchRetryDelay)
		failureReason := assigneeUnavailableRedispatchingCode
		if clearTeam {
			failureReason = "team_disabled_pool_released"
		}
		updates := map[string]any{
			"status":                       enums.TicketStatusPendingDispatch,
			"current_assignee_id":          0,
			"assigned_at":                  nil,
			"accepted_at":                  nil,
			"accept_deadline_at":           nil,
			"dispatch_deferred_until":      redispatchDeferredUntil,
			"last_dispatch_failure_reason": failureReason,
			"updated_at":                   now,
			"update_user_id":               operator.UserID,
			"update_user_name":             operator.Username,
		}
		if clearTeam {
			updates["current_team_id"] = 0
		}
		if err := repositories.TicketRepository.Updates(ctx.Tx, locked.ID, updates); err != nil {
			return err
		}
		if locked.ConversationID > 0 {
			conversation := repositories.ConversationRepository.Get(ctx.Tx, locked.ConversationID)
			if conversation != nil && conversation.CurrentAssigneeID == oldAssigneeID {
				conversationUpdates := map[string]any{
					"status":              enums.IMConversationStatusPending,
					"current_assignee_id": 0,
					"updated_at":          now,
					"update_user_id":      operator.UserID,
					"update_user_name":    operator.Username,
				}
				if clearTeam && conversation.CurrentTeamID == locked.CurrentTeamID {
					conversationUpdates["current_team_id"] = 0
				}
				if err := repositories.ConversationRepository.Updates(ctx.Tx, conversation.ID, conversationUpdates); err != nil {
					return err
				}
			}
		}
		if err := finishPendingTicketDispatchAttemptTx(ctx.Tx, locked.ID, ticketDispatchOutcomeSuperseded, nil, now); err != nil {
			return err
		}
		progress := &models.TicketProgress{
			TenantID:     locked.TenantID,
			TicketID:     locked.ID,
			EventType:    enums.TicketProgressEventAssigned,
			Content:      "负责人不可接单，重新派单",
			MetadataJSON: "{}",
			AuthorID:     operator.UserID,
			CreatedAt:    now,
		}
		if err := repositories.TicketProgressRepository.Create(ctx.Tx, progress); err != nil {
			return err
		}
		recycled = true
		return nil
	})
	return recycled, err
}

// recycleTicketAfterAcceptTimeout 清空负责人、退回 pending_dispatch 并立即重派。
// 重派排除上一任负责人，避免同一个人反复接到同一张工单。
func (s *ticketDispatchService) recycleTicketAfterAcceptTimeout(ticket *models.Ticket, oldAssigneeID int64, nextAttempt int, now time.Time, force bool) (bool, error) {
	operator := systemDispatchPrincipal()
	recycled := false
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		locked := loadTicketForUpdate(ctx.Tx, ticket.ID)
		if locked == nil || locked.CurrentAssigneeID != oldAssigneeID {
			return nil
		}
		if locked.AcceptedAt != nil {
			return nil
		}
		if !force && (locked.AcceptDeadlineAt == nil || locked.AcceptDeadlineAt.After(now)) {
			return nil
		}
		redispatchDeferredUntil := now.Add(shortDispatchRetryDelay)
		if err := repositories.TicketRepository.Updates(ctx.Tx, locked.ID, map[string]any{
			"status":                       enums.TicketStatusPendingDispatch,
			"current_assignee_id":          0,
			"assigned_at":                  nil,
			"accepted_at":                  nil,
			"accept_deadline_at":           nil,
			"dispatch_attempts":            nextAttempt,
			"dispatch_deferred_until":      redispatchDeferredUntil,
			"last_dispatch_failure_reason": acceptTimeoutRedispatchingCode,
			"updated_at":                   now,
			"update_user_id":               operator.UserID,
			"update_user_name":             operator.Username,
		}); err != nil {
			return err
		}
		if locked.ConversationID > 0 {
			conversation := repositories.ConversationRepository.Get(ctx.Tx, locked.ConversationID)
			if conversation != nil && conversation.CurrentAssigneeID == oldAssigneeID {
				if err := repositories.ConversationRepository.Updates(ctx.Tx, conversation.ID, map[string]any{
					"status":              enums.IMConversationStatusPending,
					"current_assignee_id": 0,
					"updated_at":          now,
					"update_user_id":      operator.UserID,
					"update_user_name":    operator.Username,
				}); err != nil {
					return err
				}
			}
		}
		if err := finishPendingTicketDispatchAttemptTx(ctx.Tx, locked.ID, ticketDispatchOutcomeTimedOut, nil, now); err != nil {
			return err
		}
		progress := &models.TicketProgress{
			TenantID:     locked.TenantID,
			TicketID:     locked.ID,
			EventType:    enums.TicketProgressEventAssigned,
			Content:      "接单超时，重新派单",
			MetadataJSON: "{}",
			AuthorID:     operator.UserID,
			CreatedAt:    now,
		}
		if err := repositories.TicketProgressRepository.Create(ctx.Tx, progress); err != nil {
			return err
		}
		recycled = true
		return nil
	})
	return recycled, err
}

// escalateTicketAfterRepeatedTimeout 接单多次超时后升级主管并停止自动派单。
func (s *ticketDispatchService) escalateTicketAfterRepeatedTimeout(ticket *models.Ticket, oldAssigneeID int64, nextAttempt int, now time.Time, force bool) (bool, error) {
	operator := systemDispatchPrincipal()
	supervisorID := ResolveSupervisorOwner(ticket)
	if supervisorID <= 0 {
		bucket := now.Truncate(15 * time.Minute).Unix()
		key := "ticket.supervisor_missing:" + strconv.FormatInt(ticket.ID, 10) + ":" + strconv.FormatInt(bucket, 10)
		_ = NotifySupervisors(ticket, "工单接单超时且缺少兜底负责人",
			"工单 "+ticket.TicketNo+" 已连续接单超时，但未找到可接管的组长或服务经理", "ticket_supervisor_missing", key)
		return false, nil
	}
	escalated := false
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		locked := loadTicketForUpdate(ctx.Tx, ticket.ID)
		if locked == nil || locked.CurrentAssigneeID != oldAssigneeID {
			return nil
		}
		if locked.AcceptedAt != nil {
			return nil
		}
		if !force && (locked.AcceptDeadlineAt == nil || locked.AcceptDeadlineAt.After(now)) {
			return nil
		}
		teamID := resolveTicketDispatchTeamIDDB(ctx.Tx, locked)
		updates := map[string]any{
			"status":                       enums.TicketStatusProcessing,
			"current_team_id":              teamID,
			"current_assignee_id":          supervisorID,
			"accept_deadline_at":           nil,
			"dispatch_attempts":            nextAttempt,
			"dispatch_deferred_until":      nil,
			"last_dispatch_failure_reason": "",
			"updated_at":                   now,
			"update_user_id":               operator.UserID,
			"update_user_name":             operator.Username,
		}
		updates["assigned_at"] = now
		updates["accepted_at"] = now
		if err := repositories.TicketRepository.Updates(ctx.Tx, locked.ID, updates); err != nil {
			return err
		}
		if locked.ConversationID > 0 {
			conversation := repositories.ConversationRepository.Get(ctx.Tx, locked.ConversationID)
			if conversation != nil && conversation.CurrentAssigneeID == oldAssigneeID {
				if err := repositories.ConversationRepository.Updates(ctx.Tx, conversation.ID, map[string]any{
					"status":              enums.IMConversationStatusActive,
					"current_assignee_id": supervisorID,
					"updated_at":          now,
					"update_user_id":      operator.UserID,
					"update_user_name":    operator.Username,
				}); err != nil {
					return err
				}
			}
		}
		if err := finishPendingTicketDispatchAttemptTx(ctx.Tx, locked.ID, ticketDispatchOutcomeEscalated, nil, now); err != nil {
			return err
		}
		if err := createFinishedTicketDispatchAttemptTx(ctx.Tx, locked, teamID, supervisorID, ticketDispatchOutcomeEscalated, "接单多次超时，主管兜底接管", operator, now); err != nil {
			return err
		}
		progress := &models.TicketProgress{
			TenantID:     locked.TenantID,
			TicketID:     locked.ID,
			EventType:    enums.TicketProgressEventEscalated,
			Content:      "多次接单超时，自动升级主管",
			MetadataJSON: "{}",
			AuthorID:     operator.UserID,
			CreatedAt:    now,
		}
		if err := repositories.TicketProgressRepository.Create(ctx.Tx, progress); err != nil {
			return err
		}
		if supervisorID > 0 {
			if _, err := enqueueTicketAssignedEventTx(ctx.Tx, locked, oldAssigneeID, supervisorID, "接单多次超时，主管兜底接管", operator, progress.ID, now); err != nil {
				return err
			}
			if ctx.RegisterCallback != nil {
				ctx.RegisterCallback(eventbus.WakeDefaultOutboxPublisher)
			}
		}
		escalated = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if !escalated {
		return false, nil
	}
	key := "ticket.escalation:" + strconv.FormatInt(ticket.ID, 10) + ":" + strconv.FormatInt(int64(nextAttempt), 10)
	_ = NotifySupervisors(ticket, "工单多次接单超时已升级",
		"工单 "+ticket.TicketNo+" 连续 "+strconv.FormatInt(int64(nextAttempt), 10)+" 次接单超时，已升级，请及时处理", "ticket_escalation", key)
	return true, nil
}

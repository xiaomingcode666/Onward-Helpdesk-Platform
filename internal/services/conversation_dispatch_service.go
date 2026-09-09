package services

import (
	"errors"
	"log/slog"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ConversationDispatchService = newConversationDispatchService()

func newConversationDispatchService() *conversationDispatchService {
	return &conversationDispatchService{}
}

type conversationDispatchService struct{}

type dispatchCandidate struct {
	profile             models.AgentProfile
	teamID              int64
	assignmentMode      string
	dispatchWeight      int
	teamDispatchEnabled bool
	activeCount         int
	openTicketCount     int
	workload            int
	loadRate            float64
	score               float64
}

type agentActiveConversationCount struct {
	CurrentAssigneeID int64 `gorm:"column:current_assignee_id"`
	ActiveCount       int   `gorm:"column:active_count"`
}

type agentOpenTicketCount struct {
	CurrentAssigneeID int64 `gorm:"column:current_assignee_id"`
	OpenCount         int   `gorm:"column:open_count"`
}

type dispatchPoolReport struct {
	RequestedTeamIDs []int64
	MatchedProfiles  int
	EligibleProfiles int
	CandidateCount   int
	Reason           string
}

type linkedTicketDispatchGuard struct {
	blocked             bool
	deferredUntil       *time.Time
	excludeUserID       int64
	noAlternativeReason string
}

var errConversationDispatchConflict = errors.New("conversation dispatch conflict")
var errConversationDispatchCandidateUnavailable = errors.New("conversation dispatch candidate unavailable")

const pendingDispatchBatchLimit = 50

const (
	dispatchReasonAutomatic = "自动分配"
)

// dispatchScanWindowFactor 限制单次派单扫描窗口为派单目标的若干倍。无人可派时
// 循环只扫描窗口内的候选而不是整表，避免空转扫全表放大 DB 压力；窗口内仍按
// 等待时长优先逐条尝试，派满 limit 条即提前结束。
const dispatchScanWindowFactor = 4

func dispatchScanWindow(limit int) int {
	if limit <= 0 {
		limit = pendingDispatchBatchLimit
	}
	return limit * dispatchScanWindowFactor
}

var pendingDispatchRunning atomic.Bool

func (s *conversationDispatchService) DispatchConversation(conversationID int64) (*models.Conversation, error) {
	return s.DispatchConversationExcluding(conversationID, 0)
}

// DispatchConversationExcluding 自动派单时跳过指定负责人，用于接单超时后的立即重派。
func (s *conversationDispatchService) DispatchConversationExcluding(conversationID, excludeUserID int64) (*models.Conversation, error) {
	return s.DispatchConversationExcludingForReason(conversationID, excludeUserID, "no_alternative_after_accept_timeout")
}

func (s *conversationDispatchService) DispatchConversationExcludingForReason(conversationID, excludeUserID int64, noAlternativeReason string) (*models.Conversation, error) {
	if conversationID <= 0 {
		return nil, nil
	}
	conversation := ConversationService.Get(conversationID)
	if conversation == nil {
		return nil, nil
	}
	if conversation.Status != enums.IMConversationStatusPending || conversation.CurrentAssigneeID > 0 {
		return nil, nil
	}
	aiAgent := AIAgentService.Get(conversation.AIAgentID)
	if aiAgent == nil || aiAgent.Status != enums.StatusOk {
		return nil, nil
	}
	return s.dispatchPendingConversationExcludingWithReason(conversation, aiAgent, excludeUserID, noAlternativeReason)
}

func (s *conversationDispatchService) DispatchPendingConversation(conversation *models.Conversation, aiAgent *models.AIAgent) (*models.Conversation, error) {
	return s.dispatchPendingConversationExcluding(conversation, aiAgent, 0)
}

func (s *conversationDispatchService) dispatchPendingConversationExcluding(conversation *models.Conversation, aiAgent *models.AIAgent, excludeUserID int64) (*models.Conversation, error) {
	return s.dispatchPendingConversationExcludingWithReason(conversation, aiAgent, excludeUserID, "no_alternative_after_accept_timeout")
}

func (s *conversationDispatchService) dispatchPendingConversationExcludingWithReason(conversation *models.Conversation, aiAgent *models.AIAgent, excludeUserID int64, noAlternativeReason string) (*models.Conversation, error) {
	if conversation == nil || aiAgent == nil {
		return nil, nil
	}
	if conversation.Status != enums.IMConversationStatusPending || conversation.CurrentAssigneeID > 0 {
		return nil, nil
	}
	now := time.Now()
	bypassBackoff := excludeUserID > 0
	guard, err := resolveLinkedTicketDispatchGuardDB(sqls.DB(), conversation.ID, now, false)
	if err != nil {
		return nil, err
	}
	if guard.blocked {
		return nil, nil
	}
	if !bypassBackoff && guard.deferredUntil != nil && guard.deferredUntil.After(now) {
		return nil, nil
	}
	if excludeUserID <= 0 && guard.excludeUserID > 0 {
		excludeUserID = guard.excludeUserID
		noAlternativeReason = guard.noAlternativeReason
	}
	tenantID, productID, scopeOK := resolveConversationDispatchScope(conversation, aiAgent)
	if !scopeOK {
		slog.Warn("skip auto dispatch due to conversation and agent scope mismatch",
			"conversation_id", conversation.ID,
			"conversation_tenant_id", conversation.TenantID,
			"conversation_product_id", conversation.ProductID,
			"ai_agent_id", aiAgent.ID,
			"ai_agent_tenant_id", aiAgent.TenantID,
			"ai_agent_product_id", aiAgent.ProductID,
		)
		return nil, nil
	}

	teamIDs := resolveHumanDispatchTeamIDs(tenantID, productID, utils.SplitInt64s(aiAgent.TeamIDs))
	if len(teamIDs) == 0 {
		slog.Debug("skip auto dispatch due to empty ai agent team ids",
			"conversation_id", conversation.ID,
			"ai_agent_id", aiAgent.ID,
		)
		return nil, nil
	}

	candidates, report, err := s.pickDispatchCandidates(tenantID, productID, teamIDs, now)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		slog.Debug("no dispatch candidate available",
			"conversation_id", conversation.ID,
			"ai_agent_id", aiAgent.ID,
			"requested_team_ids", report.RequestedTeamIDs,
			"matched_profiles", report.MatchedProfiles,
			"eligible_profiles", report.EligibleProfiles,
			"reason", report.Reason,
		)
		if escalated, err := s.handleUndispatchableLinkedTicket(conversation, report.Reason, now); err != nil {
			return nil, err
		} else if escalated != nil {
			return ConversationService.Get(conversation.ID), nil
		}
		return nil, nil
	}

	assignmentReason := dispatchAssignmentReason(report)
	for _, candidate := range candidates {
		if excludeUserID > 0 && candidate.profile.UserID == excludeUserID {
			continue
		}
		dispatched, err := s.tryAssignConversationWithBackoff(conversation.ID, candidate, assignmentReason, bypassBackoff)
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
			slog.Info("conversation auto dispatched",
				"conversation_id", dispatched.ID,
				"ai_agent_id", aiAgent.ID,
				"assignee_id", dispatched.CurrentAssigneeID,
				"team_id", dispatched.CurrentTeamID,
				"candidate_count", report.CandidateCount,
				"requested_team_ids", report.RequestedTeamIDs,
			)
			WsService.PublishConversationChanged(dispatched, enums.IMRealtimeEventConversationAssigned)
			return dispatched, nil
		}
	}
	slog.Debug("auto dispatch candidate list exhausted without assignment",
		"conversation_id", conversation.ID,
		"ai_agent_id", aiAgent.ID,
		"candidate_count", report.CandidateCount,
	)
	reason := "candidate_became_unavailable"
	if excludeUserID > 0 {
		reason = strings.TrimSpace(noAlternativeReason)
		if reason == "" {
			reason = "no_alternative_after_accept_timeout"
		}
	}
	if escalated, err := s.handleUndispatchableLinkedTicket(conversation, reason, now); err != nil {
		return nil, err
	} else if escalated != nil {
		return ConversationService.Get(conversation.ID), nil
	}
	return nil, nil
}

func resolveLinkedTicketDispatchGuardDB(db *gorm.DB, conversationID int64, now time.Time, forUpdate bool) (linkedTicketDispatchGuard, error) {
	guard := linkedTicketDispatchGuard{}
	if db == nil || conversationID <= 0 || !db.Migrator().HasTable(&models.Ticket{}) {
		return guard, nil
	}
	query := db.Where("conversation_id = ? AND status NOT IN ?", conversationID, []enums.TicketStatus{
		enums.TicketStatusClosed,
		enums.TicketStatusDone,
		enums.TicketStatusCancelled,
	}).Order("id DESC")
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var ticket models.Ticket
	if err := query.First(&ticket).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return guard, nil
		}
		return guard, err
	}
	if ticket.CurrentAssigneeID > 0 {
		guard.blocked = true
		return guard, nil
	}
	if ticket.DispatchDeferredUntil != nil && ticket.DispatchDeferredUntil.After(now) {
		guard.deferredUntil = ticket.DispatchDeferredUntil
		return guard, nil
	}

	switch normalizeDispatchFailureReason(ticket.LastDispatchFailureReason) {
	case acceptTimeoutRedispatchingCode, "no_alternative_after_accept_timeout":
		guard.noAlternativeReason = "no_alternative_after_accept_timeout"
	case assigneeUnavailableRedispatchingCode, "no_alternative_after_assignee_unavailable":
		guard.noAlternativeReason = "no_alternative_after_assignee_unavailable"
	default:
		return guard, nil
	}
	attempt, err := repositories.TicketDispatchAttemptRepository.FindLatestFailedAssignee(db, ticket.ID, []string{
		ticketDispatchOutcomeTimedOut,
		ticketDispatchOutcomeSuperseded,
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return guard, nil
		}
		return guard, err
	}
	guard.excludeUserID = attempt.AssigneeID
	return guard, nil
}

func (s *conversationDispatchService) handleUndispatchableLinkedTicket(conversation *models.Conversation, reason string, now time.Time) (*models.Ticket, error) {
	if conversation == nil || conversation.ID <= 0 {
		return nil, nil
	}
	ticket := repositories.TicketRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("conversation_id", conversation.ID).
		Where("status NOT IN ?", []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled}).
		Desc("id"))
	if ticket == nil || ticket.CurrentAssigneeID > 0 {
		return nil, nil
	}
	if ticket.DispatchDeferredUntil != nil && ticket.DispatchDeferredUntil.After(now) {
		return nil, nil
	}
	return TicketDispatchService.handleUndispatchableTicket(ticket, reason, now)
}

// RecoverUnavailableAssigneeAssignments immediately returns pure pending
// conversations to the team pool when their current assignee becomes unavailable.
// Conversations with an active linked ticket are recovered by TicketDispatchService
// so ticket attempt audit and assignment notifications stay the source of truth.
func (s *conversationDispatchService) RecoverUnavailableAssigneeAssignments(tenantID, assigneeID int64, now time.Time) (int, error) {
	if tenantID <= 0 || assigneeID <= 0 {
		return 0, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	lockName := "remotehelpdesk:conversation_dispatch:assignee_unavailable:" +
		strconv.FormatInt(tenantID, 10) + ":" + strconv.FormatInt(assigneeID, 10)
	return withDispatchScanLock(lockName, func() (int, error) {
		conversations := s.findPurePendingAssignedConversations(sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("current_assignee_id", assigneeID).
			Asc("updated_at").
			Asc("id"))
		return s.recoverPureConversationAssignments(conversations, now, true, "负责人已切换为不可接入状态，会话已回到待接入池。")
	})
}

func (s *conversationDispatchService) RecoverTeamMemberIneligibleAssignments(tenantID, teamID, assigneeID int64, now time.Time) (int, error) {
	if tenantID <= 0 || teamID <= 0 || assigneeID <= 0 {
		return 0, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	lockName := "remotehelpdesk:conversation_dispatch:team_member_ineligible:" +
		strconv.FormatInt(tenantID, 10) + ":" +
		strconv.FormatInt(teamID, 10) + ":" +
		strconv.FormatInt(assigneeID, 10)
	return withDispatchScanLock(lockName, func() (int, error) {
		conversations := s.findPurePendingAssignedConversations(sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("current_team_id", teamID).
			Eq("current_assignee_id", assigneeID).
			Asc("updated_at").
			Asc("id"))
		return s.recoverPureConversationAssignments(conversations, now, true, "负责人已不符合当前团队派单资格，会话已回到待接入池。")
	})
}

func (s *conversationDispatchService) RecoverTeamIneligibleAssignments(tenantID, teamID int64, now time.Time) (int, error) {
	if tenantID <= 0 || teamID <= 0 {
		return 0, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	lockName := "remotehelpdesk:conversation_dispatch:team_ineligible:" +
		strconv.FormatInt(tenantID, 10) + ":" + strconv.FormatInt(teamID, 10)
	return withDispatchScanLock(lockName, func() (int, error) {
		conversations := s.findPurePendingAssignedConversations(sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("current_team_id", teamID).
			Asc("updated_at").
			Asc("id"))
		return s.recoverPureConversationAssignments(conversations, now, false, "负责人已不符合当前团队排班或派单资格，会话已回到待接入池。")
	})
}

func (s *conversationDispatchService) RecoverDisabledTeamAssignments(tenantID, teamID int64, now time.Time) (int, error) {
	if tenantID <= 0 || teamID <= 0 {
		return 0, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	lockName := "remotehelpdesk:conversation_dispatch:team_disabled:" +
		strconv.FormatInt(tenantID, 10) + ":" + strconv.FormatInt(teamID, 10)
	return withDispatchScanLock(lockName, func() (int, error) {
		assignedConversations := s.findPurePendingAssignedConversations(sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("current_team_id", teamID).
			Asc("updated_at").
			Asc("id"))
		recoveredAssigned, assignedErr := s.recoverPureConversationAssignmentsWithOptions(assignedConversations, now, false, "负责人所在团队已停用，会话已回到待接入池。", true)
		releasedPool, poolErr := s.releaseDisabledTeamConversationPool(tenantID, teamID, now)
		return recoveredAssigned + releasedPool, errors.Join(assignedErr, poolErr)
	})
}

// RecoverIneligibleAssignments scans pure pending assigned conversations whose
// assignee no longer satisfies hard ownership constraints such as active account,
// active team membership, or approved-leave availability.
func (s *conversationDispatchService) RecoverIneligibleAssignments(limit int) (int, error) {
	return withDispatchScanLock("remotehelpdesk:conversation_dispatch:ineligible_assignment_recovery", func() (int, error) {
		if limit <= 0 {
			limit = pendingDispatchBatchLimit
		}
		conversations := s.findPurePendingAssignedConversations(sqls.NewCnd().
			Gt("tenant_id", 0).
			Asc("updated_at").
			Asc("id").
			Limit(limit))
		return s.recoverPureConversationAssignments(conversations, time.Now(), false, "负责人已不符合当前派单资格，会话已回到待接入池。")
	})
}

func conversationIDColumnRef(db *gorm.DB) string {
	if db == nil {
		db = sqls.DB()
	}
	if db == nil {
		return "conversations.id"
	}
	return db.NamingStrategy.TableName("Conversation") + ".id"
}

func (s *conversationDispatchService) findPurePendingAssignedConversations(cnd *sqls.Cnd) []models.Conversation {
	db := sqls.DB()
	if db == nil {
		return nil
	}
	if cnd == nil {
		cnd = sqls.NewCnd()
	}
	activeLinkedTicket := db.Model(&models.Ticket{}).
		Select("1").
		Where("conversation_id = "+conversationIDColumnRef(db)).
		Where("status NOT IN ?", []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled})
	cnd.Eq("status", enums.IMConversationStatusPending).
		Gt("current_assignee_id", 0).
		Where("NOT EXISTS (?)", activeLinkedTicket)
	return ConversationService.Find(cnd)
}

func (s *conversationDispatchService) findPurePendingTeamPoolConversations(cnd *sqls.Cnd) []models.Conversation {
	db := sqls.DB()
	if db == nil {
		return nil
	}
	if cnd == nil {
		cnd = sqls.NewCnd()
	}
	activeLinkedTicket := db.Model(&models.Ticket{}).
		Select("1").
		Where("conversation_id = "+conversationIDColumnRef(db)).
		Where("status NOT IN ?", []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled})
	cnd.Eq("status", enums.IMConversationStatusPending).
		Eq("current_assignee_id", 0).
		Where("NOT EXISTS (?)", activeLinkedTicket)
	return ConversationService.Find(cnd)
}

func (s *conversationDispatchService) releaseDisabledTeamConversationPool(tenantID, teamID int64, now time.Time) (int, error) {
	conversations := s.findPurePendingTeamPoolConversations(sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("current_team_id", teamID).
		Asc("updated_at").
		Asc("id"))
	if len(conversations) == 0 {
		return 0, nil
	}
	released := 0
	var releaseErrors []error
	for i := range conversations {
		ok, err := s.releaseDisabledTeamConversationPoolItem(conversations[i].ID, teamID, now)
		if err != nil {
			releaseErrors = append(releaseErrors, err)
			slog.Warn("release pure conversation from disabled team pool failed",
				"conversation_id", conversations[i].ID,
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

func (s *conversationDispatchService) releaseDisabledTeamConversationPoolItem(conversationID, teamID int64, now time.Time) (bool, error) {
	operator := systemDispatchPrincipal()
	released := false
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		locked, err := loadConversationForUpdate(ctx.Tx, conversationID)
		if err != nil {
			return err
		}
		if locked == nil || locked.Status != enums.IMConversationStatusPending || locked.CurrentTeamID != teamID || locked.CurrentAssigneeID != 0 {
			return nil
		}
		if hasActiveLinkedTicketDB(ctx.Tx, locked.ID) {
			return nil
		}
		if err := repositories.ConversationRepository.Updates(ctx.Tx, locked.ID, map[string]any{
			"current_team_id":  0,
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		}); err != nil {
			return err
		}
		if err := ConversationEventLogService.CreateEvent(ctx, locked.ID, enums.IMEventTypeTransfer, enums.IMSenderTypeSystem, operator.UserID, "会话回到全局待接入池", ConversationService.buildEventPayload(map[string]any{
			"fromStatus":   locked.Status,
			"toStatus":     enums.IMConversationStatusPending,
			"fromTeamId":   teamID,
			"toTeamId":     int64(0),
			"toAssigneeId": int64(0),
			"reason":       "团队已停用",
		})); err != nil {
			return err
		}
		released = true
		return nil
	})
	return released, err
}

func (s *conversationDispatchService) recoverPureConversationAssignments(conversations []models.Conversation, now time.Time, force bool, recoveryReason string) (int, error) {
	return s.recoverPureConversationAssignmentsWithOptions(conversations, now, force, recoveryReason, false)
}

func (s *conversationDispatchService) recoverPureConversationAssignmentsWithOptions(conversations []models.Conversation, now time.Time, force bool, recoveryReason string, clearTeam bool) (int, error) {
	if len(conversations) == 0 {
		return 0, nil
	}
	recovered := 0
	var recoverErrors []error
	for i := range conversations {
		conversation := &conversations[i]
		oldAssigneeID := conversation.CurrentAssigneeID
		if oldAssigneeID <= 0 {
			continue
		}
		if !force && validateAssignedConversationAcceptanceDB(sqls.DB(), conversation, oldAssigneeID, conversation.CurrentTeamID, now) == nil {
			continue
		}
		recycled, err := s.recoverPurePendingAssigneeConversation(conversation, oldAssigneeID, now, recoveryReason, clearTeam)
		if err != nil {
			recoverErrors = append(recoverErrors, err)
			slog.Warn("recover pure conversation after assignee became ineligible failed",
				"conversation_id", conversation.ID,
				"tenant_id", conversation.TenantID,
				"team_id", conversation.CurrentTeamID,
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

func validateAssignedConversationAcceptanceDB(db *gorm.DB, conversation *models.Conversation, assigneeID, teamID int64, now time.Time) error {
	if conversation == nil || conversation.TenantID <= 0 || assigneeID <= 0 || db == nil {
		return nil
	}
	if db.Migrator().HasTable(&models.User{}) {
		var user models.User
		if err := db.Select("id", "status").Where("id = ?", assigneeID).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errConversationDispatchCandidateUnavailable
			}
			return err
		}
		if user.Status != enums.StatusOk {
			return errConversationDispatchCandidateUnavailable
		}
	}
	if db.Migrator().HasTable(&models.AgentProfile{}) {
		var profile models.AgentProfile
		if err := db.Where("tenant_id = ? AND user_id = ?", conversation.TenantID, assigneeID).First(&profile).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errConversationDispatchCandidateUnavailable
			}
			return err
		}
		if profile.Status != enums.StatusOk {
			return errConversationDispatchCandidateUnavailable
		}
	}
	if teamID > 0 && db.Migrator().HasTable(&models.AgentTeam{}) {
		team := repositories.AgentTeamRepository.Get(db, teamID)
		if team == nil || team.TenantID != conversation.TenantID || team.Status != enums.StatusOk {
			return errConversationDispatchCandidateUnavailable
		}
		if conversation.ProductID > 0 && team.ProductID > 0 && team.ProductID != conversation.ProductID {
			return errConversationDispatchCandidateUnavailable
		}
		if !AgentTeamMemberService.IsUserActiveMemberOfTeamDB(db, conversation.TenantID, teamID, assigneeID) {
			return errConversationDispatchCandidateUnavailable
		}
		return validateAutoAssigneeLeaveAvailabilityDB(db, conversation.TenantID, assigneeID, now)
	}
	return validateAutoAssigneeLeaveAvailabilityDB(db, conversation.TenantID, assigneeID, now)
}

func (s *conversationDispatchService) recoverPurePendingAssigneeConversation(conversation *models.Conversation, oldAssigneeID int64, now time.Time, recoveryReason string, clearTeam bool) (bool, error) {
	if conversation == nil || conversation.ID <= 0 || oldAssigneeID <= 0 {
		return false, nil
	}
	operator := systemDispatchPrincipal()
	recycled := false
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		locked, err := loadConversationForUpdate(ctx.Tx, conversation.ID)
		if err != nil {
			return err
		}
		if locked == nil || locked.Status != enums.IMConversationStatusPending || locked.CurrentAssigneeID != oldAssigneeID {
			return nil
		}
		if hasActiveLinkedTicketDB(ctx.Tx, locked.ID) {
			return nil
		}
		if err := ConversationAssignmentService.FinishActiveAssignments(ctx, locked.ID, now); err != nil {
			return err
		}
		updates := map[string]any{
			"status":              enums.IMConversationStatusPending,
			"current_assignee_id": 0,
			"updated_at":          now,
			"update_user_id":      operator.UserID,
			"update_user_name":    operator.Username,
		}
		toTeamID := locked.CurrentTeamID
		if clearTeam {
			updates["current_team_id"] = 0
			toTeamID = 0
		}
		if err := repositories.ConversationRepository.Updates(ctx.Tx, locked.ID, updates); err != nil {
			return err
		}
		if err := ConversationEventLogService.CreateEvent(ctx, locked.ID, enums.IMEventTypeTransfer, enums.IMSenderTypeSystem, operator.UserID, "会话回到待接入池", ConversationService.buildEventPayload(map[string]any{
			"fromStatus":     locked.Status,
			"toStatus":       enums.IMConversationStatusPending,
			"fromAssigneeId": oldAssigneeID,
			"toAssigneeId":   int64(0),
			"toTeamId":       toTeamID,
			"reason":         strings.TrimSpace(recoveryReason),
		})); err != nil {
			return err
		}
		recycled = true
		return nil
	})
	if err != nil || !recycled {
		return recycled, err
	}
	dispatched, dispatchErr := s.DispatchConversationExcludingForReason(conversation.ID, oldAssigneeID, "no_alternative_after_assignee_unavailable")
	if dispatched == nil {
		if current := ConversationService.Get(conversation.ID); current != nil {
			WsService.PublishConversationChanged(current, enums.IMRealtimeEventConversationUpdated)
		}
	}
	return true, dispatchErr
}

func loadConversationForUpdate(db *gorm.DB, conversationID int64) (*models.Conversation, error) {
	if db == nil || conversationID <= 0 {
		return nil, nil
	}
	var conversation models.Conversation
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&conversation, conversationID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conversation, nil
}

func hasActiveLinkedTicketDB(db *gorm.DB, conversationID int64) bool {
	if db == nil || conversationID <= 0 || !db.Migrator().HasTable(&models.Ticket{}) {
		return false
	}
	return repositories.TicketRepository.FindOne(db, sqls.NewCnd().
		Eq("conversation_id", conversationID).
		Where("status NOT IN ?", []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled}).
		Desc("id")) != nil
}

func (s *conversationDispatchService) DispatchPendingConversations(limit int) (int, error) {
	if !pendingDispatchRunning.CompareAndSwap(false, true) {
		return 0, nil
	}
	defer pendingDispatchRunning.Store(false)

	return withDispatchScanLock("remotehelpdesk:conversation_dispatch:pending_conversations", func() (int, error) {
		return s.dispatchPendingConversationsOnce(limit)
	})
}

func (s *conversationDispatchService) dispatchPendingConversationsOnce(limit int) (int, error) {
	if limit <= 0 {
		limit = pendingDispatchBatchLimit
	}
	if _, err := TicketDispatchService.recoverUnclaimedLinkedConversationTickets(dispatchScanWindow(limit)); err != nil {
		return 0, err
	}
	// 按等待时长优先（最老先派），避免新会话持续插队导致老会话饥饿。
	// 只取扫描窗口内的候选，无人可派时也不会逐条扫全表空转。
	conversations := ConversationService.Find(sqls.NewCnd().
		Eq("status", enums.IMConversationStatusPending).
		Eq("current_assignee_id", 0).
		Asc("created_at").
		Asc("id").
		Limit(dispatchScanWindow(limit)))
	if len(conversations) == 0 {
		return 0, nil
	}

	dispatchedCount := 0
	scannedCount := 0
	var dispatchErrors []error
	for _, conversation := range conversations {
		if dispatchedCount >= limit {
			break
		}
		scannedCount++
		dispatched, err := s.DispatchConversation(conversation.ID)
		if err != nil {
			dispatchErrors = append(dispatchErrors, err)
			continue
		}
		if dispatched != nil {
			dispatchedCount++
		}
	}
	if scannedCount > 0 {
		slog.Info("pending conversation dispatch scan completed",
			"scanned_count", scannedCount,
			"dispatched_count", dispatchedCount,
			"limit", limit,
		)
	}
	return dispatchedCount, errors.Join(dispatchErrors...)
}

func (s *conversationDispatchService) RunPendingDispatchLoop(interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		slog.Info("pending conversation dispatch loop started",
			"interval_seconds", int(interval/time.Second),
		)

		for {
			if _, err := s.DispatchPendingConversations(0); err != nil {
				slog.Warn("dispatch pending conversations loop failed", "error", err)
			}
			<-ticker.C
		}
	}()
}

// pickDispatchCandidates returns the eligible dispatch candidates for the given teamIDs at the given time, along with a report for debugging and analysis.
func (s *conversationDispatchService) pickDispatchCandidates(tenantID, productID int64, teamIDs []int64, now time.Time) ([]dispatchCandidate, dispatchPoolReport, error) {
	report := dispatchPoolReport{
		RequestedTeamIDs: append([]int64(nil), teamIDs...),
	}

	dispatchProfiles := AgentTeamMemberService.FindDispatchProfilesByTeamIDs(sqls.DB(), tenantID, teamIDs)
	report.MatchedProfiles = len(dispatchProfiles)
	if len(dispatchProfiles) == 0 {
		report.Reason = "no_matched_profile"
		return nil, report, nil
	}

	enabledProfiles, enabledUserIDs, reason := s.filterEnabledDispatchProfiles(tenantID, dispatchProfiles, now)
	if reason != "" {
		report.Reason = reason
		return nil, report, nil
	}
	report.EligibleProfiles = len(enabledProfiles)

	activeCounts, err := s.findActiveConversationCountMap(enabledUserIDs)
	if err != nil {
		return nil, report, err
	}
	openTicketCounts, err := s.findOpenTicketCountMap(enabledUserIDs)
	if err != nil {
		return nil, report, err
	}

	candidates := make([]dispatchCandidate, 0, len(enabledProfiles))
	for _, dispatchProfile := range enabledProfiles {
		profile := dispatchProfile.Profile
		activeCount := activeCounts[profile.UserID]
		openTicketCount := openTicketCounts[profile.UserID]
		workload := activeCount + openTicketCount
		if profile.MaxConcurrentCount <= 0 || workload >= profile.MaxConcurrentCount {
			continue
		}
		loadRate := float64(workload) / math.Max(float64(profile.MaxConcurrentCount), 1)
		score := float64(workload)
		if dispatchProfile.AssignmentMode == AgentTeamAssignmentModeWeighted {
			score = float64(workload+1) / math.Max(float64(dispatchProfile.DispatchWeight), 1)
		}
		candidates = append(candidates, dispatchCandidate{
			profile:             profile,
			teamID:              dispatchProfile.TeamID,
			assignmentMode:      dispatchProfile.AssignmentMode,
			dispatchWeight:      dispatchProfile.DispatchWeight,
			teamDispatchEnabled: dispatchProfile.DispatchEnabled,
			activeCount:         activeCount,
			openTicketCount:     openTicketCount,
			workload:            workload,
			loadRate:            loadRate,
			score:               score,
		})
	}
	report.CandidateCount = len(candidates)
	if len(candidates) == 0 {
		report.Reason = "all_candidates_at_capacity"
		return nil, report, nil
	}

	slices.SortFunc(candidates, func(a, b dispatchCandidate) int {
		switch {
		case a.score < b.score:
			return -1
		case a.score > b.score:
			return 1
		}
		if a.assignmentMode == AgentTeamAssignmentModeWeighted || b.assignmentMode == AgentTeamAssignmentModeWeighted {
			switch {
			case a.dispatchWeight > b.dispatchWeight:
				return -1
			case a.dispatchWeight < b.dispatchWeight:
				return 1
			}
		}
		switch {
		case a.loadRate < b.loadRate:
			return -1
		case a.loadRate > b.loadRate:
			return 1
		}
		switch {
		case a.workload < b.workload:
			return -1
		case a.workload > b.workload:
			return 1
		}
		switch {
		case a.profile.PriorityLevel > b.profile.PriorityLevel:
			return -1
		case a.profile.PriorityLevel < b.profile.PriorityLevel:
			return 1
		}
		aLastStatusAt := zeroTime(a.profile.LastStatusAt)
		bLastStatusAt := zeroTime(b.profile.LastStatusAt)
		switch {
		case aLastStatusAt.Before(bLastStatusAt):
			return -1
		case aLastStatusAt.After(bLastStatusAt):
			return 1
		}
		switch {
		case a.teamID < b.teamID:
			return -1
		case a.teamID > b.teamID:
			return 1
		case a.profile.UserID < b.profile.UserID:
			return -1
		case a.profile.UserID > b.profile.UserID:
			return 1
		default:
			return 0
		}
	})
	report.Reason = "ok"
	return candidates, report, nil
}

func (s *conversationDispatchService) filterEnabledDispatchProfiles(tenantID int64, profiles []AgentTeamDispatchProfile, now time.Time) ([]AgentTeamDispatchProfile, []int64, string) {
	userIDs := make([]int64, 0, len(profiles))
	for _, profile := range profiles {
		if profile.Profile.UserID > 0 {
			userIDs = append(userIDs, profile.Profile.UserID)
		}
	}
	if len(userIDs) == 0 {
		return nil, nil, "no_profile_with_capacity_config"
	}

	enabledUsers := UserService.Find(sqls.NewCnd().
		In("id", userIDs).
		Eq("status", enums.StatusOk))
	if len(enabledUsers) == 0 {
		return nil, nil, "no_enabled_user"
	}

	enabledUserSet := make(map[int64]struct{}, len(enabledUsers))
	for _, user := range enabledUsers {
		enabledUserSet[user.ID] = struct{}{}
	}
	if !AgentScheduleExceptionService.IsWithinPersonalDispatchRuleDB(sqls.DB(), tenantID, now) {
		return nil, nil, "outside_personal_dispatch_rule"
	}
	scheduleAvailable := AgentScheduleExceptionService.FindAvailableUserSetDB(sqls.DB(), tenantID, userIDs, now)
	hasConfigEligibleProfile := false
	hasScheduleAvailableProfile := false
	hasUnreachableProfile := false

	enabledProfiles := make([]AgentTeamDispatchProfile, 0, len(profiles))
	enabledUserIDs := make([]int64, 0, len(profiles))
	for _, profile := range profiles {
		if _, exists := enabledUserSet[profile.Profile.UserID]; !exists {
			continue
		}
		if !profile.DispatchEnabled || !profile.Profile.AutoAssignEnabled || profile.Profile.ServiceStatus != enums.ServiceStatusIdle || profile.Profile.MaxConcurrentCount <= 0 {
			continue
		}
		hasConfigEligibleProfile = true
		if !scheduleAvailable[profile.Profile.UserID] {
			continue
		}
		hasScheduleAvailableProfile = true
		if !isDispatchReachable(&profile.Profile, now) {
			hasUnreachableProfile = true
			continue
		}
		enabledProfiles = append(enabledProfiles, profile)
		enabledUserIDs = append(enabledUserIDs, profile.Profile.UserID)
	}
	if len(enabledProfiles) == 0 {
		if hasUnreachableProfile || hasScheduleAvailableProfile {
			return nil, nil, "no_reachable_user"
		}
		if hasConfigEligibleProfile {
			return nil, nil, "all_engineers_on_approved_leave"
		}
		return nil, nil, "no_profile_for_enabled_user"
	}
	return enabledProfiles, enabledUserIDs, ""
}

func (s *conversationDispatchService) findEligibleTeamIDs(tenantID, productID int64, teamIDs []int64) []int64 {
	return s.findEligibleTeamIDsDB(sqls.DB(), tenantID, productID, teamIDs)
}

func (s *conversationDispatchService) findEligibleTeamIDsDB(db *gorm.DB, tenantID, productID int64, teamIDs []int64) []int64 {
	if len(teamIDs) == 0 {
		return nil
	}
	if db == nil {
		db = sqls.DB()
	}
	if db == nil || !db.Migrator().HasTable(&models.AgentTeam{}) {
		return uniqueServiceInt64s(teamIDs)
	}
	teams := repositories.AgentTeamRepository.Find(db, sqls.NewCnd().
		In("id", teamIDs).
		Eq("status", enums.StatusOk))
	eligibleSet := make(map[int64]struct{}, len(teams))
	for _, team := range teams {
		if tenantID > 0 && team.TenantID != tenantID {
			continue
		}
		if productID > 0 && team.ProductID > 0 && team.ProductID != productID {
			continue
		}
		eligibleSet[team.ID] = struct{}{}
	}
	ret := make([]int64, 0, len(teamIDs))
	seen := make(map[int64]struct{}, len(teamIDs))
	for _, teamID := range teamIDs {
		if _, eligible := eligibleSet[teamID]; !eligible {
			continue
		}
		if _, exists := seen[teamID]; exists {
			continue
		}
		seen[teamID] = struct{}{}
		ret = append(ret, teamID)
	}
	return ret
}

func dispatchAssignmentReason(report dispatchPoolReport) string {
	return dispatchReasonAutomatic
}

func (s *conversationDispatchService) findActiveConversationCountMap(userIDs []int64) (map[int64]int, error) {
	ret := make(map[int64]int, len(userIDs))
	if len(userIDs) == 0 {
		return ret, nil
	}

	rows := make([]agentActiveConversationCount, 0)
	linkedTicket := sqls.DB().Model(&models.Ticket{}).
		Select("1").
		Where("conversation_id = " + conversationIDColumnRef(sqls.DB()))
	if err := sqls.DB().
		Model(&models.Conversation{}).
		Select("current_assignee_id, COUNT(1) AS active_count").
		Where("status IN ? AND current_assignee_id IN ?", []enums.IMConversationStatus{
			enums.IMConversationStatusPending,
			enums.IMConversationStatusActive,
		}, userIDs).
		Where("NOT EXISTS (?)", linkedTicket).
		Group("current_assignee_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.CurrentAssigneeID <= 0 {
			continue
		}
		ret[row.CurrentAssigneeID] = row.ActiveCount
	}
	return ret, nil
}

func (s *conversationDispatchService) findOpenTicketCountMap(userIDs []int64) (map[int64]int, error) {
	ret := make(map[int64]int, len(userIDs))
	if len(userIDs) == 0 {
		return ret, nil
	}

	rows := make([]agentOpenTicketCount, 0)
	if err := sqls.DB().
		Model(&models.Ticket{}).
		Select("current_assignee_id, COUNT(1) AS open_count").
		Where("current_assignee_id IN ?", userIDs).
		Where("status IN ?", dispatchWorkloadTicketStatuses()).
		Group("current_assignee_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.CurrentAssigneeID <= 0 {
			continue
		}
		ret[row.CurrentAssigneeID] = row.OpenCount
	}
	return ret, nil
}

func dispatchWorkloadTicketStatuses() []enums.TicketStatus {
	return []enums.TicketStatus{
		enums.TicketStatusAccepted,
		enums.TicketStatusAssigned,
		enums.TicketStatusInProgress,
		enums.TicketStatusEscalated,
		enums.TicketStatusReopened,
		enums.TicketStatusPendingAssigneeAccept,
		enums.TicketStatusProcessing,
		enums.TicketStatusVideoSupport,
		enums.TicketStatusSupplierSupport,
	}
}

func (s *conversationDispatchService) tryAssignConversation(conversationID int64, candidate dispatchCandidate, reason string) (*models.Conversation, error) {
	return s.tryAssignConversationWithBackoff(conversationID, candidate, reason, false)
}

func (s *conversationDispatchService) tryAssignConversationWithBackoff(conversationID int64, candidate dispatchCandidate, reason string, bypassBackoff bool) (*models.Conversation, error) {
	now := time.Now()
	operator := systemDispatchPrincipal()

	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		conversation := repositories.ConversationRepository.Get(ctx.Tx, conversationID)
		if conversation == nil {
			return errConversationDispatchConflict
		}
		if conversation.Status != enums.IMConversationStatusPending || conversation.CurrentAssigneeID > 0 {
			return errConversationDispatchConflict
		}
		guard, err := resolveLinkedTicketDispatchGuardDB(ctx.Tx, conversationID, now, true)
		if err != nil {
			return err
		}
		if guard.blocked {
			return errConversationDispatchConflict
		}
		if !bypassBackoff && guard.deferredUntil != nil && guard.deferredUntil.After(now) {
			return errConversationDispatchConflict
		}
		if guard.excludeUserID > 0 && guard.excludeUserID == candidate.profile.UserID {
			return errConversationDispatchCandidateUnavailable
		}
		profile, err := s.lockAndValidateDispatchCandidateAt(ctx.Tx, candidate, conversation.TenantID, conversation.ProductID, now)
		if err != nil {
			return err
		}

		result := ctx.Tx.Model(&models.Conversation{}).
			Where("id = ? AND status = ? AND current_assignee_id = ?", conversationID, enums.IMConversationStatusPending, 0).
			Updates(map[string]any{
				"current_assignee_id": profile.UserID,
				"current_team_id":     candidate.teamID,
				"status":              enums.IMConversationStatusPending,
				"update_user_id":      operator.UserID,
				"update_user_name":    operator.Username,
				"updated_at":          now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errConversationDispatchConflict
		}
		if err := ConversationAssignmentService.FinishActiveAssignments(ctx, conversationID, now); err != nil {
			return err
		}
		assignment, err := ConversationAssignmentService.CreateAssignment(ctx, conversationID, conversation.CurrentAssigneeID, profile.UserID, enums.IMAssignmentTypeAssign, reason, operator, now)
		if err != nil {
			return err
		}
		if err := TicketService.SyncConversationDispatchTx(ctx.Tx, conversationID, candidate.teamID, profile.UserID, reason, operator); err != nil {
			return err
		}

		if err := ConversationEventLogService.CreateEvent(ctx, conversationID, enums.IMEventTypeAssign, enums.IMSenderTypeSystem, operator.UserID, "会话已自动分配", buildDispatchEventPayload(conversation.CurrentAssigneeID, profile.UserID, candidate.teamID, reason)); err != nil {
			return err
		}
		assignedEvent := events.ConversationAssignedEvent{
			ConversationID: conversationID,
			FromUserID:     conversation.CurrentAssigneeID,
			ToUserID:       profile.UserID,
			OperatorID:     operator.UserID,
			Reason:         strings.TrimSpace(reason),
			AssignType:     events.ConversationAssignTypeAutoAssign,
		}
		if err := enqueueConversationAssignedEventTx(ctx.Tx, conversation.TenantID, assignment.ID, &assignedEvent, now); err != nil {
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
	return ConversationService.Get(conversationID), nil
}

func (s *conversationDispatchService) lockAndValidateDispatchCandidate(db *gorm.DB, candidate dispatchCandidate, tenantID, productID int64) (*models.AgentProfile, error) {
	return s.lockAndValidateDispatchCandidateAt(db, candidate, tenantID, productID, time.Now())
}

func (s *conversationDispatchService) lockAndValidateDispatchCandidateAt(db *gorm.DB, candidate dispatchCandidate, tenantID, productID int64, validationNow time.Time) (*models.AgentProfile, error) {
	if db == nil || candidate.profile.ID <= 0 || candidate.profile.UserID <= 0 || candidate.teamID <= 0 {
		return nil, errConversationDispatchCandidateUnavailable
	}
	if validationNow.IsZero() {
		validationNow = time.Now()
	}

	var profile models.AgentProfile
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", candidate.profile.ID, candidate.profile.UserID).
		First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errConversationDispatchCandidateUnavailable
		}
		return nil, err
	}
	if profile.Status != enums.StatusOk || (tenantID > 0 && profile.TenantID != tenantID) {
		return nil, errConversationDispatchCandidateUnavailable
	}
	if !profile.AutoAssignEnabled || profile.ServiceStatus != enums.ServiceStatusIdle || profile.MaxConcurrentCount <= 0 {
		return nil, errConversationDispatchCandidateUnavailable
	}
	if !AgentScheduleExceptionService.isUserAvailableDB(db, profile.TenantID, profile.UserID, validationNow) {
		return nil, errConversationDispatchCandidateUnavailable
	}

	var user models.User
	if err := db.Select("id", "status").Where("id = ?", profile.UserID).First(&user).Error; err != nil || user.Status != enums.StatusOk {
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, errConversationDispatchCandidateUnavailable
	}

	var team models.AgentTeam
	if err := db.Select("id", "tenant_id", "product_id", "status").Where("id = ?", candidate.teamID).First(&team).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errConversationDispatchCandidateUnavailable
		}
		return nil, err
	}
	if team.Status != enums.StatusOk || (profile.TenantID > 0 && team.TenantID != profile.TenantID) {
		return nil, errConversationDispatchCandidateUnavailable
	}
	if tenantID > 0 && team.TenantID != tenantID {
		return nil, errConversationDispatchCandidateUnavailable
	}
	if productID > 0 && team.ProductID > 0 && team.ProductID != productID {
		return nil, errConversationDispatchCandidateUnavailable
	}
	if !db.Migrator().HasTable(&models.AgentTeamMember{}) {
		return nil, errConversationDispatchCandidateUnavailable
	}
	var membership models.AgentTeamMember
	err := db.Where("team_id = ? AND user_id = ?", candidate.teamID, profile.UserID).First(&membership).Error
	switch {
	case err == nil:
		if membership.Status != enums.StatusOk || !membership.DispatchEnabled || (profile.TenantID > 0 && membership.TenantID != profile.TenantID) {
			return nil, errConversationDispatchCandidateUnavailable
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, errConversationDispatchCandidateUnavailable
	default:
		return nil, err
	}
	if !isDispatchReachable(&profile, validationNow) {
		return nil, errConversationDispatchCandidateUnavailable
	}
	workload, err := s.dispatchWorkloadForUserDB(db, profile.UserID)
	if err != nil {
		return nil, err
	}
	if workload >= profile.MaxConcurrentCount {
		return nil, errConversationDispatchCandidateUnavailable
	}

	return &profile, nil
}

func resolveConversationDispatchScope(conversation *models.Conversation, aiAgent *models.AIAgent) (int64, int64, bool) {
	if conversation == nil || aiAgent == nil {
		return 0, 0, false
	}
	if conversation.TenantID > 0 && aiAgent.TenantID > 0 && conversation.TenantID != aiAgent.TenantID {
		return 0, 0, false
	}
	if conversation.ProductID > 0 && aiAgent.ProductID > 0 && conversation.ProductID != aiAgent.ProductID {
		return 0, 0, false
	}
	tenantID := conversation.TenantID
	if tenantID <= 0 {
		tenantID = aiAgent.TenantID
	}
	productID := conversation.ProductID
	if productID <= 0 {
		productID = aiAgent.ProductID
	}
	return tenantID, productID, true
}

func (s *conversationDispatchService) dispatchWorkloadForUserDB(db *gorm.DB, userID int64) (int, error) {
	if db == nil || userID <= 0 {
		return 0, nil
	}
	var activeConversations int64
	linkedTicket := db.Model(&models.Ticket{}).
		Select("1").
		Where("conversation_id = " + conversationIDColumnRef(db))
	if err := db.Model(&models.Conversation{}).
		Where("status IN ? AND current_assignee_id = ?", []enums.IMConversationStatus{
			enums.IMConversationStatusPending,
			enums.IMConversationStatusActive,
		}, userID).
		Where("NOT EXISTS (?)", linkedTicket).
		Count(&activeConversations).Error; err != nil {
		return 0, err
	}
	var openTickets int64
	if err := db.Model(&models.Ticket{}).
		Where("current_assignee_id = ?", userID).
		Where("status IN ?", dispatchWorkloadTicketStatuses()).
		Count(&openTickets).Error; err != nil {
		return 0, err
	}
	return int(activeConversations + openTickets), nil
}

func buildDispatchEventPayload(fromAssigneeID, toAssigneeID, toTeamID int64, reason string) string {
	return ConversationService.buildEventPayload(map[string]any{
		"fromStatus":     enums.IMConversationStatusPending,
		"toStatus":       enums.IMConversationStatusPending,
		"fromAssigneeId": fromAssigneeID,
		"toAssigneeId":   toAssigneeID,
		"toTeamId":       toTeamID,
		"reason":         strings.TrimSpace(reason),
	})
}

func systemDispatchPrincipal() *dto.AuthPrincipal {
	return &dto.AuthPrincipal{
		UserID:   0,
		Username: "system",
		Nickname: "system",
	}
}

func isDispatchReachable(profile *models.AgentProfile, now time.Time) bool {
	if profile == nil || profile.UserID <= 0 {
		return false
	}
	if profile.ReceiveOfflineMessage {
		return true
	}
	if WsService.IsUserOnline(profile.UserID) {
		return true
	}
	if profile.LastOnlineAt == nil || profile.LastOnlineAt.IsZero() || now.Before(*profile.LastOnlineAt) {
		return false
	}
	freshness := time.Duration(config.CurrentOrDefault().TicketDispatch.Normalized().OnlineFreshnessMinutes) * time.Minute
	return now.Sub(*profile.LastOnlineAt) <= freshness
}

func zeroTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ConversationHumanDispatchService = newConversationHumanDispatchService()

var (
	HandoffWaitingMessage        = HandoffWaitingMessageForLocale(i18nx.DefaultLocale)
	HandoffOffHoursMessage       = HandoffOffHoursMessageForLocale(i18nx.DefaultLocale)
	HandoffOffHoursQueuedMessage = HandoffOffHoursQueuedMessageForLocale(i18nx.DefaultLocale)
	HandoffAIHoldMessage         = HandoffAIHoldMessageForLocale(i18nx.DefaultLocale)
)

func HandoffWaitingMessageForLocale(locale string) string {
	return i18nx.Getf(locale, "conversation.handoff.waiting")
}

func HandoffCustomerRequestedMessageForLocale(locale string) string {
	return i18nx.Getf(locale, "conversation.handoff.customerRequested")
}

func HandoffOffHoursMessageForLocale(locale string) string {
	return i18nx.Getf(locale, "conversation.handoff.offHours")
}

func HandoffOffHoursQueuedMessageForLocale(locale string) string {
	return i18nx.Getf(locale, "conversation.handoff.offHoursQueued")
}

func HandoffAIHoldMessageForLocale(locale string) string {
	return i18nx.Getf(locale, "conversation.handoff.aiHold")
}

type HandoffDecisionType string

const (
	HandoffDecisionAssigned   HandoffDecisionType = "assigned"
	HandoffDecisionTeamPool   HandoffDecisionType = "team_pool"
	HandoffDecisionGlobalPool HandoffDecisionType = "global_pool"
	HandoffDecisionOffHours   HandoffDecisionType = "off_hours"
	HandoffDecisionAIHold     HandoffDecisionType = "ai_hold"
)

type HandoffDecisionResult struct {
	Decision      HandoffDecisionType
	TeamID        int64
	AssigneeID    int64
	TicketID      int64
	TicketNo      string
	TicketCreated bool
	Message       string
}

type conversationHumanDispatchService struct{}

func newConversationHumanDispatchService() *conversationHumanDispatchService {
	return &conversationHumanDispatchService{}
}

func (s *conversationHumanDispatchService) TryOffHoursHandoffByAI(conversationID int64, aiAgent models.AIAgent, reason string) (bool, error) {
	return s.TryOffHoursHandoffByAIWithRequestID(conversationID, aiAgent, reason, "")
}

func (s *conversationHumanDispatchService) TryOffHoursHandoffByAIWithRequestID(conversationID int64, aiAgent models.AIAgent, reason string, requestID string) (bool, error) {
	conversation := ConversationService.Get(conversationID)
	if conversation == nil {
		return false, errorsx.InvalidParamI18n("error.e0116")
	}
	if isRestrictedTenantDefaultGeneralConversation(conversation, &aiAgent) {
		return false, errorsx.Forbidden("通用咨询仅支持 AI 自助，不支持转人工或创建工单")
	}
	if err := requireTenantOwnedConversationForHumanDispatch(conversation); err != nil {
		return false, err
	}
	if !AIWorkflowService.AgentAllowsHumanHandoff(&aiAgent) {
		return false, errorsx.Forbidden("当前产品仅提供 AI 客服，不支持转人工")
	}
	tenantID, productID, scopeOK := resolveConversationDispatchScope(conversation, &aiAgent)
	teamIDs := orderedPositiveIDs(aiAgent.TeamIDs)
	if !scopeOK {
		teamIDs = nil
	}
	teamIDs = resolveHumanDispatchTeamIDs(tenantID, productID, teamIDs)
	candidates, _, err := ConversationDispatchService.pickDispatchCandidates(tenantID, productID, teamIDs, time.Now())
	if err != nil {
		return false, err
	}
	if len(candidates) > 0 {
		return false, nil
	}
	if err := s.createEventWithRequestID(conversationID, requestID, enums.IMEventTypeTransfer, enums.IMSenderTypeAI, aiAgent.ID, "转人工失败：非服务时间", strings.TrimSpace(reason)); err != nil {
		return true, err
	}
	if err := s.sendAITextWithRequestID(conversationID, aiAgent.ID, HandoffOffHoursMessage, requestID); err != nil {
		return true, err
	}
	return true, nil
}

func (s *conversationHumanDispatchService) HandoffByAI(conversationID int64, aiAgent models.AIAgent, reason string) (*HandoffDecisionResult, error) {
	return s.HandoffByAIWithRequestID(conversationID, aiAgent, reason, "")
}

func (s *conversationHumanDispatchService) HandoffByAIWithRequestID(conversationID int64, aiAgent models.AIAgent, reason string, requestID string) (*HandoffDecisionResult, error) {
	return s.HandoffByAIWithRequestIDForLocale(conversationID, aiAgent, reason, requestID, i18nx.DefaultLocale)
}

func (s *conversationHumanDispatchService) HandoffByAIWithRequestIDForLocale(conversationID int64, aiAgent models.AIAgent, reason, requestID, locale string) (*HandoffDecisionResult, error) {
	locale = i18nx.NormalizeLocale(locale)
	waitingMessage := HandoffWaitingMessageForLocale(locale)
	aiHoldMessage := HandoffAIHoldMessageForLocale(locale)
	offHoursQueuedMessage := HandoffOffHoursQueuedMessageForLocale(locale)
	conversation := ConversationService.Get(conversationID)
	if conversation == nil {
		return nil, errorsx.InvalidParamI18n("error.e0116")
	}
	if isRestrictedTenantDefaultGeneralConversation(conversation, &aiAgent) {
		return nil, errorsx.Forbidden("通用咨询仅支持 AI 自助，不支持转人工或创建工单")
	}
	if err := requireTenantOwnedConversationForHumanDispatch(conversation); err != nil {
		return nil, err
	}
	if !AIWorkflowService.AgentAllowsHumanHandoff(&aiAgent) {
		return nil, errorsx.Forbidden("当前产品仅提供 AI 客服，不支持转人工")
	}
	if conversation.Status == enums.IMConversationStatusClosed {
		return nil, errorsx.InvalidParam("conversation is closed")
	}
	mode := normalizedHandoffMode(aiAgent.HandoffMode)
	if TenantCapabilityService.KnowledgeSupport(conversation.TenantID) {
		mode = enums.AIAgentHandoffModeDefaultTeamPool
	}
	if conversation.HandoffAt != nil {
		ticket, _, err := s.ensureHandoffTicket(conversation, aiAgent, reason)
		if err != nil {
			return nil, err
		}
		if mode == enums.AIAgentHandoffModeDefaultTeamPool && conversation.Status == enums.IMConversationStatusPending && conversation.CurrentAssigneeID <= 0 && conversation.CurrentTeamID <= 0 {
			teamIDs := resolveHumanDispatchTeamIDs(conversation.TenantID, conversation.ProductID, orderedPositiveIDs(aiAgent.TeamIDs))
			if len(teamIDs) > 0 {
				updated, moveErr := s.moveToTeamPoolWithDispatchFailure(conversation.ID, teamIDs[0], reason, requestID, "no_available_engineer")
				if moveErr != nil && !errors.Is(moveErr, errConversationDispatchConflict) {
					return nil, moveErr
				}
				if updated != nil {
					conversation = updated
				}
			}
		}
		return s.resolveStartedHandoff(conversation, mode, ticket, locale), nil
	}
	ticket, ticketCreated, claimed, err := s.claimHandoffWithTicket(conversation, aiAgent, reason, requestID, mode)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return s.resolveConcurrentHandoff(conversationID, mode, ticket, locale)
	}
	if mode == enums.AIAgentHandoffModeAIHoldAndNotify {
		if err := s.sendAITextWithRequestID(conversationID, aiAgent.ID, aiHoldMessage, requestID); err != nil {
			return nil, err
		}
		return handoffDecisionWithTicket(&HandoffDecisionResult{
			Decision: HandoffDecisionAIHold,
			Message:  aiHoldMessage,
		}, ticket, ticketCreated), nil
	}
	if mode == enums.AIAgentHandoffModeWaitPool {
		if err := s.moveToGlobalPool(conversationID, aiAgent.Name); err != nil {
			return nil, err
		}
		if err := s.sendAITextWithRequestID(conversationID, aiAgent.ID, waitingMessage, requestID); err != nil {
			return nil, err
		}
		return handoffDecisionWithTicket(&HandoffDecisionResult{
			Decision: HandoffDecisionGlobalPool,
			Message:  waitingMessage,
		}, ticket, ticketCreated), nil
	}
	tenantID, productID, scopeOK := resolveConversationDispatchScope(conversation, &aiAgent)
	teamIDs := orderedPositiveIDs(aiAgent.TeamIDs)
	if !scopeOK {
		teamIDs = nil
	}
	teamIDs = resolveHumanDispatchTeamIDs(tenantID, productID, teamIDs)
	candidates, _, err := ConversationDispatchService.pickDispatchCandidates(tenantID, productID, teamIDs, time.Now())
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		decision := HandoffDecisionGlobalPool
		teamID := int64(0)
		if len(teamIDs) > 0 {
			teamID = teamIDs[0]
			decision = HandoffDecisionTeamPool
			if _, err := s.moveToTeamPoolWithDispatchFailure(conversationID, teamID, reason, requestID, "no_available_engineer"); err != nil {
				return nil, err
			}
		} else if err := s.moveToGlobalPool(conversationID, aiAgent.Name); err != nil {
			return nil, err
		}
		if err := s.sendAITextWithRequestID(conversationID, aiAgent.ID, offHoursQueuedMessage, requestID); err != nil {
			return nil, err
		}
		return handoffDecisionWithTicket(&HandoffDecisionResult{
			Decision: decision,
			TeamID:   teamID,
			Message:  offHoursQueuedMessage,
		}, ticket, ticketCreated), nil
	}
	result, err := s.dispatchAfterHandoffWithRequestIDForLocale(conversationID, aiAgent.ID, teamIDs, strings.TrimSpace(reason), true, requestID, locale)
	if err != nil {
		return nil, err
	}
	return handoffDecisionWithTicket(result, ticket, ticketCreated), nil
}

func (s *conversationHumanDispatchService) resolveConcurrentHandoff(conversationID int64, mode enums.AIAgentHandoffMode, ticket *models.Ticket, locale string) (*HandoffDecisionResult, error) {
	conversation := ConversationService.Get(conversationID)
	if conversation == nil || conversation.HandoffAt == nil {
		return nil, errConversationDispatchConflict
	}
	return s.resolveStartedHandoff(conversation, mode, ticket, locale), nil
}

func (s *conversationHumanDispatchService) resolveStartedHandoff(conversation *models.Conversation, mode enums.AIAgentHandoffMode, ticket *models.Ticket, locale string) *HandoffDecisionResult {
	waitingMessage := HandoffWaitingMessageForLocale(locale)
	result := &HandoffDecisionResult{}
	switch {
	case mode == enums.AIAgentHandoffModeAIHoldAndNotify:
		result.Decision = HandoffDecisionAIHold
		result.Message = HandoffAIHoldMessageForLocale(locale)
	case conversation != nil && conversation.CurrentAssigneeID > 0:
		result.Decision = HandoffDecisionAssigned
		result.TeamID = conversation.CurrentTeamID
		result.AssigneeID = conversation.CurrentAssigneeID
		result.Message = waitingMessage
	case conversation != nil && conversation.CurrentTeamID > 0:
		result.Decision = HandoffDecisionTeamPool
		result.TeamID = conversation.CurrentTeamID
		result.Message = waitingMessage
	default:
		result.Decision = HandoffDecisionGlobalPool
		result.Message = waitingMessage
	}
	return handoffDecisionWithTicket(result, ticket, false)
}

func (s *conversationHumanDispatchService) claimHandoffWithTicket(conversation *models.Conversation, aiAgent models.AIAgent, reason, requestID string, mode enums.AIAgentHandoffMode) (*models.Ticket, bool, bool, error) {
	if conversation == nil {
		return nil, false, false, errorsx.InvalidParamI18n("error.e0116")
	}
	if isRestrictedTenantDefaultGeneralConversation(conversation, &aiAgent) {
		return nil, false, false, errorsx.Forbidden("通用咨询仅支持 AI 自助，不支持转人工或创建工单")
	}
	if conversation.TenantID <= 0 {
		return nil, false, false, errorsx.InvalidParam("conversation must belong to a tenant before human handoff")
	}

	var ticket *models.Ticket
	ticketCreated := false
	claimed := false
	db := sqls.DB()
	key := conversationTicketCreateLockKey(conversation.TenantID, conversation.ID)
	run := func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			if key != "" && db != nil && db.Dialector != nil && db.Dialector.Name() == "postgres" {
				if err := ctx.Tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error; err != nil {
					return fmt.Errorf("lock conversation ticket create: %w", err)
				}
			}
			var err error
			ticket, ticketCreated, claimed, err = s.claimHandoffWithTicketTx(ctx, conversation.ID, aiAgent, reason, requestID, mode)
			return err
		})
	}
	if key != "" && (db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres") {
		lock := acquireLocalConversationCreateScopeLock(key)
		defer releaseLocalConversationCreateScopeLock(key, lock)
	}
	if err := run(); err != nil {
		return nil, false, false, err
	}
	return ticket, ticketCreated, claimed, nil
}

func (s *conversationHumanDispatchService) claimHandoffWithTicketTx(ctx *sqls.TxContext, conversationID int64, aiAgent models.AIAgent, reason, requestID string, mode enums.AIAgentHandoffMode) (*models.Ticket, bool, bool, error) {
	if ctx == nil || ctx.Tx == nil {
		return nil, false, false, errorsx.InvalidParam("handoff transaction is not available")
	}
	current := &models.Conversation{}
	if err := ctx.Tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(current, conversationID).Error; err != nil {
		return nil, false, false, err
	}
	if isRestrictedTenantDefaultGeneralConversation(current, &aiAgent) {
		return nil, false, false, errorsx.Forbidden("通用咨询仅支持 AI 自助，不支持转人工或创建工单")
	}
	if current.Status == enums.IMConversationStatusClosed {
		return nil, false, false, errorsx.InvalidParam("conversation is closed")
	}
	operator := handoffTicketOperator(current, aiAgent)
	if current.HandoffAt != nil {
		ticket, created, err := TicketService.createFromConversationTx(ctx, s.buildHandoffTicketRequestDB(ctx.Tx, current, aiAgent, reason), operator, current, current.TenantID)
		return ticket, created, false, err
	}
	if current.Status != enums.IMConversationStatusAIServing {
		return nil, false, false, errConversationDispatchConflict
	}

	now := time.Now()
	trimmedReason := strings.TrimSpace(reason)
	updates := map[string]any{
		"handoff_at":       now,
		"handoff_reason":   trimmedReason,
		"update_user_id":   0,
		"update_user_name": aiAgent.Name,
		"updated_at":       now,
	}
	eventContent := "AI继续接待并通知人工"
	if mode != enums.AIAgentHandoffModeAIHoldAndNotify {
		updates["status"] = enums.IMConversationStatusPending
		updates["current_team_id"] = int64(0)
		updates["current_assignee_id"] = int64(0)
		eventContent = "AI转人工"
	}
	result := ctx.Tx.Model(&models.Conversation{}).
		Where("id = ? AND status = ? AND handoff_at IS NULL", conversationID, enums.IMConversationStatusAIServing).
		Updates(updates)
	if result.Error != nil {
		return nil, false, false, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, false, false, errConversationDispatchConflict
	}

	current.HandoffAt = &now
	current.HandoffReason = trimmedReason
	current.UpdateUserID = 0
	current.UpdateUserName = aiAgent.Name
	current.UpdatedAt = now
	if mode != enums.AIAgentHandoffModeAIHoldAndNotify {
		current.Status = enums.IMConversationStatusPending
		current.CurrentTeamID = 0
		current.CurrentAssigneeID = 0
	}
	ticket, created, err := TicketService.createFromConversationTx(ctx, s.buildHandoffTicketRequestDB(ctx.Tx, current, aiAgent, reason), operator, current, current.TenantID)
	if err != nil {
		return nil, false, false, err
	}
	if err := ConversationEventLogService.CreateEventWithRequestID(ctx, conversationID, requestID, enums.IMEventTypeTransfer, enums.IMSenderTypeAI, aiAgent.ID, eventContent, trimmedReason); err != nil {
		return nil, false, false, err
	}
	return ticket, created, true, nil
}

func (s *conversationHumanDispatchService) ensureHandoffTicket(conversation *models.Conversation, aiAgent models.AIAgent, reason string) (*models.Ticket, bool, error) {
	if conversation == nil {
		return nil, false, nil
	}
	if conversation.TenantID <= 0 {
		return nil, false, errorsx.InvalidParam("conversation must belong to a tenant before human handoff")
	}
	if isRestrictedTenantDefaultGeneralConversation(conversation, &aiAgent) {
		return nil, false, errorsx.Forbidden("通用咨询仅支持 AI 自助，不支持转人工或创建工单")
	}
	if existing := repositories.TicketRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", conversation.TenantID).
		Eq("conversation_id", conversation.ID).
		Where("status NOT IN ?", []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled}).
		Desc("id")); existing != nil {
		return existing, false, nil
	}
	idempotencyKey := fmt.Sprintf("ai-handoff:%d", conversation.ID)
	if existing := repositories.TicketRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", conversation.TenantID).
		Eq("idempotency_key", idempotencyKey)); existing != nil {
		return existing, false, nil
	}
	description := strings.TrimSpace(reason)
	if summary := strings.TrimSpace(ConversationService.BuildConversationSummary(conversation)); summary != "" && summary != description {
		if description != "" {
			description += "\n\n"
		}
		description += summary
	}
	if description == "" {
		description = "AI 无法可靠回答，已转人工处理。"
	}
	ticket, err := TicketService.CreateFromConversation(s.buildHandoffTicketRequestWithDescription(conversation, aiAgent, reason, description), handoffTicketOperator(conversation, aiAgent))
	if err != nil {
		return nil, false, err
	}
	return ticket, true, nil
}

func (s *conversationHumanDispatchService) buildHandoffTicketRequest(conversation *models.Conversation, aiAgent models.AIAgent, reason string) request.CreateTicketFromConversationRequest {
	return s.buildHandoffTicketRequestDB(sqls.DB(), conversation, aiAgent, reason)
}

func (s *conversationHumanDispatchService) buildHandoffTicketRequestDB(db *gorm.DB, conversation *models.Conversation, aiAgent models.AIAgent, reason string) request.CreateTicketFromConversationRequest {
	if db == nil {
		db = sqls.DB()
	}
	description := strings.TrimSpace(reason)
	if summary := strings.TrimSpace(ConversationService.BuildConversationSummaryDB(db, conversation)); summary != "" && summary != description {
		if description != "" {
			description += "\n\n"
		}
		description += summary
	}
	if description == "" {
		description = "AI 无法可靠回答，已转人工处理。"
	}
	return s.buildHandoffTicketRequestWithDescriptionDB(db, conversation, aiAgent, reason, description)
}

func (s *conversationHumanDispatchService) buildHandoffTicketRequestWithDescription(conversation *models.Conversation, _ models.AIAgent, reason, description string) request.CreateTicketFromConversationRequest {
	return s.buildHandoffTicketRequestWithDescriptionDB(sqls.DB(), conversation, models.AIAgent{}, reason, description)
}

func (s *conversationHumanDispatchService) buildHandoffTicketRequestWithDescriptionDB(db *gorm.DB, conversation *models.Conversation, _ models.AIAgent, reason, description string) request.CreateTicketFromConversationRequest {
	if db == nil {
		db = sqls.DB()
	}
	conversationID := int64(0)
	if conversation != nil {
		conversationID = conversation.ID
	}
	return request.CreateTicketFromConversationRequest{
		IdempotencyKey: fmt.Sprintf("ai-handoff:%d", conversationID),
		ConversationID: conversationID,
		Title:          s.buildHandoffTicketTitleDB(db, conversation, reason),
		Description:    description,
	}
}

func handoffTicketOperator(conversation *models.Conversation, aiAgent models.AIAgent) *dto.AuthPrincipal {
	name := strings.TrimSpace(aiAgent.Name)
	if name == "" {
		name = "AI"
	}
	tenantID := int64(0)
	if conversation != nil {
		tenantID = conversation.TenantID
	}
	return &dto.AuthPrincipal{
		Username:    name,
		Nickname:    name,
		TenantID:    tenantID,
		DomainType:  "service_account",
		SubjectType: "service_account",
	}
}

func (s *conversationHumanDispatchService) buildHandoffTicketTitle(conversation *models.Conversation, reason string) string {
	return s.buildHandoffTicketTitleDB(sqls.DB(), conversation, reason)
}

func (s *conversationHumanDispatchService) buildHandoffTicketTitleDB(db *gorm.DB, conversation *models.Conversation, reason string) string {
	if db == nil {
		db = sqls.DB()
	}
	if conversation != nil {
		messages := repositories.MessageRepository.Find(db, sqls.NewCnd().
			Eq("conversation_id", conversation.ID).
			Eq("sender_type", enums.IMSenderTypeCustomer).
			Desc("id").
			Limit(12))
		for i := range messages {
			if title := handoffTicketIssueTitle(buildMessageSummary(messages[i].MessageType, messages[i].Content)); title != "" {
				return limitText(title, 120)
			}
		}
	}
	if title := handoffTicketIssueTitle(reason); title != "" {
		return limitText(title, 120)
	}
	return i18nx.Get("ticket.defaultConversationTitle")
}

func handoffTicketIssueTitle(value string) string {
	for _, sentence := range strings.FieldsFunc(strings.TrimSpace(value), func(r rune) bool {
		switch r {
		case '。', '！', '!', '？', '?', '；', ';', '\n', '\r':
			return true
		default:
			return false
		}
	}) {
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}
		if !containsHandoffTicketActionText(sentence) {
			return sentence
		}
		clauses := strings.FieldsFunc(sentence, func(r rune) bool { return r == '，' || r == ',' })
		kept := make([]string, 0, len(clauses))
		for _, clause := range clauses {
			clause = strings.TrimSpace(clause)
			if clause != "" && !containsHandoffTicketActionText(clause) {
				kept = append(kept, clause)
			}
		}
		if len(kept) > 0 {
			return strings.Join(kept, "，")
		}
	}
	return ""
}

func containsHandoffTicketActionText(value string) bool {
	compact := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "").Replace(strings.ToLower(value))
	for _, phrase := range []string{
		"转人工", "人工客服", "人工服务", "真人客服", "联系人工", "请求人工",
		"创建工单", "新建工单", "提交工单", "发起工单", "建工单", "开工单", "建单", "报障",
		"humanagent", "liveagent", "createticket", "openticket", "submitticket",
	} {
		if strings.Contains(compact, phrase) {
			return true
		}
	}
	return false
}

func handoffDecisionWithTicket(result *HandoffDecisionResult, ticket *models.Ticket, created bool) *HandoffDecisionResult {
	if result == nil {
		result = &HandoffDecisionResult{}
	}
	if ticket != nil {
		result.TicketID = ticket.ID
		result.TicketNo = ticket.TicketNo
		result.TicketCreated = created
	}
	return result
}

func (s *conversationHumanDispatchService) RequestByCustomer(conversationID int64, external openidentity.ExternalUser, reason, requestID string) error {
	return s.RequestByCustomerForLocale(conversationID, external, reason, requestID, i18nx.DefaultLocale)
}

func (s *conversationHumanDispatchService) RequestByCustomerForLocale(conversationID int64, external openidentity.ExternalUser, reason, requestID, locale string) error {
	locale = i18nx.NormalizeLocale(locale)
	conversation := ConversationService.Get(conversationID)
	if conversation == nil {
		return errorsx.InvalidParamI18n("error.e0116")
	}
	if !ConversationService.IsCustomerConversationOwner(conversation, external) {
		return errorsx.ForbiddenI18n("error.e0222")
	}
	if TenantCapabilityService.AIDisabled(conversation.TenantID) {
		if err := requireTenantOwnedConversationForHumanDispatch(conversation); err != nil {
			return err
		}
		if conversation.Status == enums.IMConversationStatusPending || conversation.Status == enums.IMConversationStatusActive {
			agent := ConversationService.ticketOnlyConversationAgent(resolvedConversationCreateContext{
				tenantID:  conversation.TenantID,
				productID: conversation.ProductID,
			})
			if _, _, err := s.ensureHumanOnlyCreateTicket(conversation, *agent); err != nil {
				return err
			}
			return s.syncHumanOnlyConversationTicketDispatch(conversation, "纯工单会话补齐关联工单")
		}
		if conversation.Status == enums.IMConversationStatusClosed {
			return errorsx.InvalidParam("conversation is closed")
		}
		reason = strings.TrimSpace(reason)
		if reason == "" {
			reason = "客户请求人工支持"
		}
		agent := ConversationService.ticketOnlyConversationAgent(resolvedConversationCreateContext{
			tenantID:  conversation.TenantID,
			productID: conversation.ProductID,
		})
		_, err := s.HandoffByAIWithRequestIDForLocale(conversation.ID, *agent, reason, requestID, locale)
		return err
	}
	aiAgent := AIAgentService.Get(conversation.AIAgentID)
	if aiAgent == nil || aiAgent.Status != enums.StatusOk {
		return errorsx.InvalidParamI18n("error.e0002")
	}
	if isRestrictedTenantDefaultGeneralConversation(conversation, aiAgent) {
		return errorsx.Forbidden("通用咨询仅支持 AI 自助，不支持转人工或创建工单")
	}
	if err := requireTenantOwnedConversationForHumanDispatch(conversation); err != nil {
		return err
	}
	if conversation.Status == enums.IMConversationStatusPending || conversation.Status == enums.IMConversationStatusActive {
		return nil
	}
	if conversation.Status == enums.IMConversationStatusClosed {
		return errorsx.InvalidParam("conversation is closed")
	}
	if !AIWorkflowService.AgentAllowsHumanHandoff(aiAgent) {
		return errorsx.Forbidden("当前产品仅提供 AI 客服，不支持转人工")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "客户请求人工支持"
	}
	payload := ConversationService.buildEventPayload(map[string]any{
		"source": "customer_request",
		"reason": reason,
	})
	if _, err := MessageService.SendSystemMessageWithRequestID(
		conversation.ID,
		fmt.Sprintf("customer-handoff-request:%d", conversation.ID),
		HandoffCustomerRequestedMessageForLocale(locale),
		payload,
		requestID,
	); err != nil {
		return err
	}
	_, err := s.HandoffByAIWithRequestIDForLocale(conversation.ID, *aiAgent, reason, requestID, locale)
	return err
}

func CustomerConversationAllowsHumanHandoff(conversation *models.Conversation, aiAgent *models.AIAgent) bool {
	if conversation != nil && TenantCapabilityService.AIDisabled(conversation.TenantID) {
		return true
	}
	return !isRestrictedTenantDefaultGeneralConversation(conversation, aiAgent) && AIWorkflowService.AgentAllowsHumanHandoff(aiAgent)
}

func CustomerConversationAllowsTicketCreation(conversation *models.Conversation, aiAgent *models.AIAgent) bool {
	if conversation != nil && TenantCapabilityService.AIDisabled(conversation.TenantID) {
		return true
	}
	return !isRestrictedTenantDefaultGeneralConversation(conversation, aiAgent) && AIWorkflowService.AgentAllowsTicketCreation(aiAgent)
}

func requireTenantOwnedConversationForHumanDispatch(conversation *models.Conversation) error {
	if conversation == nil || conversation.TenantID <= 0 {
		return errorsx.InvalidParam("conversation must belong to a tenant before human handoff")
	}
	return nil
}

func isTenantDefaultGeneralConversation(conversation *models.Conversation, aiAgent *models.AIAgent) bool {
	if conversation == nil || aiAgent == nil {
		return false
	}
	if aiAgent.Source != TenantDefaultAIAgentSource || aiAgent.ProductID > 0 {
		return false
	}
	return conversation.ProductID <= 0 &&
		conversation.DeviceID <= 0 &&
		conversation.ServiceCodeID <= 0 &&
		conversation.CustomerEntrySessionID <= 0
}

func isRestrictedTenantDefaultGeneralConversation(conversation *models.Conversation, aiAgent *models.AIAgent) bool {
	return isTenantDefaultGeneralConversation(conversation, aiAgent) &&
		!TenantCapabilityService.KnowledgeSupport(conversation.TenantID)
}

func normalizedHandoffMode(mode enums.AIAgentHandoffMode) enums.AIAgentHandoffMode {
	switch mode {
	case enums.AIAgentHandoffModeWaitPool,
		enums.AIAgentHandoffModeDefaultTeamPool,
		enums.AIAgentHandoffModeAIHoldAndNotify:
		return mode
	default:
		return enums.AIAgentHandoffModeWaitPool
	}
}

func resolveHumanDispatchTeamIDs(tenantID, productID int64, configuredTeamIDs []int64) []int64 {
	teamIDs := ConversationDispatchService.findEligibleTeamIDs(tenantID, productID, uniquePositiveInt64s(configuredTeamIDs))
	if tenantID <= 0 || !TenantCapabilityService.KnowledgeSupport(tenantID) {
		return teamIDs
	}
	team := repositories.AgentTeamRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("team_type", AgentTeamTypeTechnicalRepair).
		Eq("status", enums.StatusOk).
		Asc("id"))
	if team == nil || team.ID <= 0 {
		return nil
	}
	return []int64{team.ID}
}

func (s *conversationHumanDispatchService) ApplyHumanOnlyCreate(conversationID int64, aiAgent models.AIAgent) (*HandoffDecisionResult, error) {
	conversation := ConversationService.Get(conversationID)
	if conversation == nil {
		return nil, errorsx.InvalidParamI18n("error.e0116")
	}
	if err := requireTenantOwnedConversationForHumanDispatch(conversation); err != nil {
		return nil, err
	}
	ticket, ticketCreated, err := s.ensureHumanOnlyCreateTicket(conversation, aiAgent)
	if err != nil {
		return nil, err
	}
	tenantID, productID, scopeOK := resolveConversationDispatchScope(conversation, &aiAgent)
	teamIDs := orderedPositiveIDs(aiAgent.TeamIDs)
	if !scopeOK {
		teamIDs = nil
	}
	teamIDs = resolveHumanDispatchTeamIDs(tenantID, productID, teamIDs)
	candidates, _, err := ConversationDispatchService.pickDispatchCandidates(tenantID, productID, teamIDs, time.Now())
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		decision := HandoffDecisionGlobalPool
		teamID := int64(0)
		if len(teamIDs) > 0 {
			teamID = teamIDs[0]
			decision = HandoffDecisionTeamPool
			if _, err := s.moveToTeamPoolWithDispatchFailure(conversationID, teamID, "仅人工模式新会话", "", "no_available_engineer"); err != nil {
				return nil, err
			}
		} else if err := s.moveToGlobalPool(conversationID, aiAgent.Name); err != nil {
			return nil, err
		}
		if err := s.sendAIText(conversationID, aiAgent.ID, HandoffWaitingMessage); err != nil {
			return nil, err
		}
		return handoffDecisionWithTicket(&HandoffDecisionResult{Decision: decision, TeamID: teamID, Message: HandoffWaitingMessage}, ticket, ticketCreated), nil
	}
	result, err := s.dispatchAfterHandoff(conversationID, aiAgent.ID, teamIDs, "仅人工模式新会话", false)
	if err != nil {
		return nil, err
	}
	return handoffDecisionWithTicket(result, ticket, ticketCreated), nil
}

func (s *conversationHumanDispatchService) ensureHumanOnlyCreateTicket(conversation *models.Conversation, aiAgent models.AIAgent) (*models.Ticket, bool, error) {
	if conversation == nil {
		return nil, false, nil
	}
	if conversation.TenantID <= 0 {
		return nil, false, errorsx.InvalidParam("conversation must belong to a tenant before ticket creation")
	}
	if existing := repositories.TicketRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", conversation.TenantID).
		Eq("conversation_id", conversation.ID).
		Where("status NOT IN ?", []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled}).
		Desc("id")); existing != nil {
		return existing, false, nil
	}
	req := request.CreateTicketFromConversationRequest{
		IdempotencyKey:    conversationTicketIdempotencyKey(conversation.ID),
		ConversationID:    conversation.ID,
		Title:             i18nx.Get("ticket.defaultConversationTitle"),
		Description:       "客户发起人工服务会话，已进入工单处理流程。",
		CurrentTeamID:     conversation.CurrentTeamID,
		CurrentAssigneeID: conversation.CurrentAssigneeID,
	}
	ticket, err := TicketService.CreateFromConversation(req, handoffTicketOperator(conversation, aiAgent))
	if err != nil {
		return nil, false, err
	}
	return ticket, true, nil
}

func (s *conversationHumanDispatchService) syncHumanOnlyConversationTicketDispatch(conversation *models.Conversation, reason string) error {
	if conversation == nil || conversation.TenantID <= 0 || conversation.ID <= 0 {
		return nil
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		current := repositories.ConversationRepository.Get(ctx.Tx, conversation.ID)
		if current == nil {
			return errorsx.InvalidParamI18n("error.e0116")
		}
		return TicketService.SyncConversationDispatchTx(ctx.Tx, current.ID, current.CurrentTeamID, current.CurrentAssigneeID, reason, systemDispatchPrincipal())
	})
}

func (s *conversationHumanDispatchService) DispatchPendingConversation(conversationID int64, aiAgent models.AIAgent) (*HandoffDecisionResult, error) {
	conversation := ConversationService.Get(conversationID)
	if conversation == nil {
		return nil, errorsx.InvalidParamI18n("error.e0116")
	}
	if err := requireTenantOwnedConversationForHumanDispatch(conversation); err != nil {
		return nil, err
	}
	if conversation.Status != enums.IMConversationStatusPending || conversation.CurrentAssigneeID > 0 {
		return nil, errorsx.InvalidParamI18n("error.e0137")
	}
	tenantID, productID, scopeOK := resolveConversationDispatchScope(conversation, &aiAgent)
	teamIDs := orderedPositiveIDs(aiAgent.TeamIDs)
	if !scopeOK {
		teamIDs = nil
	}
	teamIDs = resolveHumanDispatchTeamIDs(tenantID, productID, teamIDs)
	if len(teamIDs) == 0 {
		return &HandoffDecisionResult{Decision: HandoffDecisionOffHours}, nil
	}
	candidates, report, err := ConversationDispatchService.pickDispatchCandidates(tenantID, productID, teamIDs, time.Now())
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 && report.Reason == "outside_personal_dispatch_rule" {
		return &HandoffDecisionResult{Decision: HandoffDecisionOffHours}, nil
	}
	if len(candidates) > 0 {
		for _, candidate := range candidates {
			dispatched, err := ConversationDispatchService.tryAssignConversation(conversationID, candidate, dispatchAssignmentReason(report))
			if errors.Is(err, errConversationDispatchCandidateUnavailable) {
				continue
			}
			if err != nil {
				return s.resolveConcurrentDispatch(conversationID, err)
			}
			if dispatched != nil {
				WsService.PublishConversationChanged(dispatched, enums.IMRealtimeEventConversationAssigned)
				return &HandoffDecisionResult{
					Decision:   HandoffDecisionAssigned,
					TeamID:     dispatched.CurrentTeamID,
					AssigneeID: dispatched.CurrentAssigneeID,
				}, nil
			}
		}
	}
	teamID := teamIDs[0]
	teamPoolConversation, err := s.moveToTeamPool(conversationID, teamID, "手动触发自动分配")
	if err != nil {
		return nil, err
	}
	if teamPoolConversation != nil {
		WsService.PublishConversationChanged(teamPoolConversation, enums.IMRealtimeEventConversationUpdated)
	}
	return &HandoffDecisionResult{Decision: HandoffDecisionTeamPool, TeamID: teamID}, nil
}

func (s *conversationHumanDispatchService) dispatchAfterHandoff(conversationID, aiAgentID int64, activeTeamIDs []int64, reason string, publishAssignEvent bool) (*HandoffDecisionResult, error) {
	return s.dispatchAfterHandoffWithRequestID(conversationID, aiAgentID, activeTeamIDs, reason, publishAssignEvent, "")
}

func (s *conversationHumanDispatchService) dispatchAfterHandoffWithRequestID(conversationID, aiAgentID int64, activeTeamIDs []int64, reason string, publishAssignEvent bool, requestID string) (*HandoffDecisionResult, error) {
	return s.dispatchAfterHandoffWithRequestIDForLocale(conversationID, aiAgentID, activeTeamIDs, reason, publishAssignEvent, requestID, i18nx.DefaultLocale)
}

func (s *conversationHumanDispatchService) dispatchAfterHandoffWithRequestIDForLocale(conversationID, aiAgentID int64, activeTeamIDs []int64, reason string, publishAssignEvent bool, requestID, locale string) (*HandoffDecisionResult, error) {
	waitingMessage := HandoffWaitingMessageForLocale(locale)
	if err := s.sendAITextWithRequestID(conversationID, aiAgentID, waitingMessage, requestID); err != nil {
		return nil, err
	}

	conversation := ConversationService.Get(conversationID)
	if conversation == nil {
		return nil, errorsx.InvalidParamI18n("error.e0116")
	}
	candidates, report, err := ConversationDispatchService.pickDispatchCandidates(conversation.TenantID, conversation.ProductID, activeTeamIDs, time.Now())
	if err != nil {
		return nil, err
	}
	if len(candidates) > 0 {
		for _, candidate := range candidates {
			dispatched, err := ConversationDispatchService.tryAssignConversation(conversationID, candidate, dispatchAssignmentReason(report))
			if errors.Is(err, errConversationDispatchCandidateUnavailable) {
				continue
			}
			if err != nil {
				return s.resolveConcurrentDispatch(conversationID, err)
			}
			if dispatched != nil {
				WsService.PublishConversationChanged(dispatched, enums.IMRealtimeEventConversationAssigned)
				return &HandoffDecisionResult{
					Decision:   HandoffDecisionAssigned,
					TeamID:     dispatched.CurrentTeamID,
					AssigneeID: dispatched.CurrentAssigneeID,
					Message:    waitingMessage,
				}, nil
			}
		}
	}

	teamID := activeTeamIDs[0]
	dispatchFailureReason := strings.TrimSpace(report.Reason)
	if dispatchFailureReason == "" {
		dispatchFailureReason = "candidate_became_unavailable"
	}
	teamPoolConversation, err := s.moveToTeamPoolWithDispatchFailure(conversationID, teamID, reason, requestID, dispatchFailureReason)
	if err != nil {
		return nil, err
	}
	if teamPoolConversation != nil {
		WsService.PublishConversationChanged(teamPoolConversation, enums.IMRealtimeEventConversationUpdated)
	}
	return &HandoffDecisionResult{Decision: HandoffDecisionTeamPool, TeamID: teamID, Message: waitingMessage}, nil
}

func (s *conversationHumanDispatchService) moveToTeamPool(conversationID, teamID int64, reason string) (*models.Conversation, error) {
	return s.moveToTeamPoolWithRequestID(conversationID, teamID, reason, "")
}

func (s *conversationHumanDispatchService) moveToTeamPoolWithRequestID(conversationID, teamID int64, reason string, requestID string) (*models.Conversation, error) {
	return s.moveToTeamPoolWithDispatchFailure(conversationID, teamID, reason, requestID, "")
}

func (s *conversationHumanDispatchService) moveToTeamPoolWithDispatchFailure(conversationID, teamID int64, reason, requestID, dispatchFailureReason string) (*models.Conversation, error) {
	now := time.Now()
	var conversation *models.Conversation
	var dispatchFailureTicket *models.Ticket
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		current := repositories.ConversationRepository.Get(ctx.Tx, conversationID)
		if current == nil {
			return errorsx.InvalidParamI18n("error.e0116")
		}
		if current.Status != enums.IMConversationStatusPending || current.CurrentAssigneeID > 0 {
			return errConversationDispatchConflict
		}
		if err := ConversationAssignmentService.FinishActiveAssignments(ctx, conversationID, now); err != nil {
			return err
		}
		result := ctx.Tx.Model(&models.Conversation{}).
			Where("id = ? AND status = ? AND current_assignee_id = ?", conversationID, enums.IMConversationStatusPending, 0).
			Updates(map[string]any{
				"status":              enums.IMConversationStatusPending,
				"current_team_id":     teamID,
				"current_assignee_id": 0,
				"update_user_id":      0,
				"update_user_name":    "system",
				"updated_at":          now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errConversationDispatchConflict
		}
		if err := TicketService.SyncConversationDispatchTx(ctx.Tx, conversationID, teamID, 0, reason, systemDispatchPrincipal()); err != nil {
			return err
		}
		if strings.TrimSpace(dispatchFailureReason) != "" {
			var err error
			dispatchFailureTicket, err = TicketDispatchService.deferConversationTicketTx(ctx.Tx, conversationID, teamID, dispatchFailureReason, now)
			if err != nil {
				return err
			}
		}
		eventPayload := map[string]any{
			"fromStatus":     current.Status,
			"toStatus":       enums.IMConversationStatusPending,
			"fromAssigneeId": current.CurrentAssigneeID,
			"toAssigneeId":   int64(0),
			"toTeamId":       teamID,
			"reason":         strings.TrimSpace(reason),
			"decision":       string(HandoffDecisionTeamPool),
		}
		if strings.TrimSpace(dispatchFailureReason) != "" {
			eventPayload["dispatchFailureReason"] = normalizeDispatchFailureReason(dispatchFailureReason)
		}
		if err := ConversationEventLogService.CreateEventWithRequestID(ctx, conversationID, requestID, enums.IMEventTypeTransfer, enums.IMSenderTypeSystem, 0, "会话进入客服组待接入", ConversationService.buildEventPayload(eventPayload)); err != nil {
			return err
		}
		current.Status = enums.IMConversationStatusPending
		current.CurrentTeamID = teamID
		current.CurrentAssigneeID = 0
		current.UpdateUserID = 0
		current.UpdateUserName = "system"
		current.UpdatedAt = now
		conversation = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	if dispatchFailureTicket != nil {
		TicketDispatchService.notifyUndispatchableTicket(dispatchFailureTicket, dispatchFailureReason, now)
	}
	return conversation, nil
}

func (s *conversationHumanDispatchService) moveToGlobalPool(conversationID int64, operatorName string) error {
	now := time.Now()
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		conversation := repositories.ConversationRepository.Get(ctx.Tx, conversationID)
		if conversation == nil {
			return errorsx.InvalidParamI18n("error.e0116")
		}
		if conversation.Status != enums.IMConversationStatusPending || conversation.CurrentAssigneeID > 0 {
			return errConversationDispatchConflict
		}
		result := ctx.Tx.Model(&models.Conversation{}).
			Where("id = ? AND status = ? AND current_assignee_id = ?", conversationID, enums.IMConversationStatusPending, 0).
			Updates(map[string]any{
				"status":              enums.IMConversationStatusPending,
				"current_team_id":     0,
				"current_assignee_id": 0,
				"update_user_id":      0,
				"update_user_name":    operatorName,
				"updated_at":          now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errConversationDispatchConflict
		}
		dispatchReason := "进入产品维修组待接入"
		if TenantCapabilityService.KnowledgeSupport(conversation.TenantID) {
			dispatchReason = "进入技术支持组待接入"
		}
		if err := TicketService.SyncConversationDispatchTx(ctx.Tx, conversationID, 0, 0, dispatchReason, systemDispatchPrincipal()); err != nil {
			return err
		}
		return ConversationEventLogService.CreateEvent(ctx, conversationID, enums.IMEventTypeTransfer, enums.IMSenderTypeSystem, 0, "会话进入全局待接入", ConversationService.buildEventPayload(map[string]any{
			"fromStatus": conversation.Status,
			"toStatus":   enums.IMConversationStatusPending,
			"decision":   string(HandoffDecisionGlobalPool),
		}))
	})
}

func (s *conversationHumanDispatchService) resolveConcurrentDispatch(conversationID int64, dispatchErr error) (*HandoffDecisionResult, error) {
	if !errors.Is(dispatchErr, errConversationDispatchConflict) {
		return nil, dispatchErr
	}
	current := ConversationService.Get(conversationID)
	if current == nil {
		return nil, dispatchErr
	}
	if current.CurrentAssigneeID > 0 {
		return &HandoffDecisionResult{
			Decision:   HandoffDecisionAssigned,
			TeamID:     current.CurrentTeamID,
			AssigneeID: current.CurrentAssigneeID,
			Message:    HandoffWaitingMessage,
		}, nil
	}
	if current.Status == enums.IMConversationStatusPending && current.CurrentTeamID > 0 {
		return &HandoffDecisionResult{
			Decision: HandoffDecisionTeamPool,
			TeamID:   current.CurrentTeamID,
			Message:  HandoffWaitingMessage,
		}, nil
	}
	return nil, dispatchErr
}

func (s *conversationHumanDispatchService) createEvent(conversationID int64, eventType enums.IMEventType, senderType enums.IMSenderType, senderID int64, content, payload string) error {
	return s.createEventWithRequestID(conversationID, "", eventType, senderType, senderID, content, payload)
}

func (s *conversationHumanDispatchService) createEventWithRequestID(conversationID int64, requestID string, eventType enums.IMEventType, senderType enums.IMSenderType, senderID int64, content, payload string) error {
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		return ConversationEventLogService.CreateEventWithRequestID(ctx, conversationID, requestID, eventType, senderType, senderID, content, payload)
	})
}

func (s *conversationHumanDispatchService) sendAIText(conversationID, aiAgentID int64, content string) error {
	return s.sendAITextWithRequestID(conversationID, aiAgentID, content, "")
}

func (s *conversationHumanDispatchService) sendAITextWithRequestID(conversationID, aiAgentID int64, content string, requestID string) error {
	_, err := MessageService.SendAIServiceNoticeWithRequestID(conversationID, aiAgentID, content, requestID)
	return err
}

func orderedPositiveIDs(value string) []int64 {
	return uniquePositiveInt64sFromStrings(strings.Split(value, ","))
}

func uniquePositiveInt64sFromStrings(values []string) []int64 {
	seen := make(map[int64]struct{}, len(values))
	ret := make([]int64, 0, len(values))
	for _, value := range values {
		var id int64
		_, _ = fmt.Sscan(strings.TrimSpace(value), &id)
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ret = append(ret, id)
	}
	return ret
}

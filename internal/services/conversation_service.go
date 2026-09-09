package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/pkg/tracex"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
	"slices"
	"strings"
	"time"

	"github.com/mlogclub/simple/common/strs"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var ConversationService = newConversationService()

const customerPresenceFreshness = 90 * time.Second

func newConversationService() *conversationService {
	return &conversationService{}
}

type conversationService struct {
}

type resolvedConversationCreateContext struct {
	tenantID               int64
	productID              int64
	productModelID         int64
	deviceID               int64
	serviceCodeID          int64
	customerEntrySessionID int64
	locale                 string
}

type conversationCreateScopeLock struct {
	mu   sync.Mutex
	refs int
}

var conversationCreateScopeLockRegistry = struct {
	sync.Mutex
	locks map[string]*conversationCreateScopeLock
}{
	locks: make(map[string]*conversationCreateScopeLock),
}

func (s *conversationService) Get(id int64) *models.Conversation {
	if id <= 0 {
		return nil
	}
	return repositories.ConversationRepository.Get(sqls.DB(), id)
}

func (s *conversationService) Find(cnd *sqls.Cnd) []models.Conversation {
	return repositories.ConversationRepository.Find(sqls.DB(), cnd)
}

func (s *conversationService) FindOne(cnd *sqls.Cnd) *models.Conversation {
	return repositories.ConversationRepository.FindOne(sqls.DB(), cnd)
}

func (s *conversationService) FindPageByCnd(cnd *sqls.Cnd) (list []models.Conversation, paging *sqls.Paging) {
	return repositories.ConversationRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *conversationService) ListConversations(tenantID, userID int64, filter request.AgentConversationFilter, keyword string, paging *sqls.Paging, operator *dto.AuthPrincipal) ([]models.Conversation, *sqls.Paging, error) {
	cnd := sqls.NewCnd().Eq("tenant_id", tenantID).Page(paging.Page, paging.Limit)
	restricted, _, viewerTeamIDs := enterpriseTicketViewerScope(tenantID, operator)
	viewerTeamIDs = uniqueServiceInt64s(viewerTeamIDs)

	if strs.IsNotBlank(keyword) {
		keyword = strings.TrimSpace(keyword)
		keywordLike := "%" + keyword + "%"
		cnd.Where("customer_name LIKE ? OR last_message_summary LIKE ?", keywordLike, keywordLike)
	}

	switch filter {
	case request.AgentConversationFilterAIServing:
		cnd.Eq("current_assignee_id", 0).Eq("status", enums.IMConversationStatusAIServing).Desc("last_active_at").Desc("id")
		if restricted {
			cnd.Eq("id", -1)
		}
	case request.AgentConversationFilterMine:
		cnd.Eq("current_assignee_id", userID).Desc("last_active_at").Desc("id")
	case request.AgentConversationFilterActive:
		cnd.Eq("current_assignee_id", userID).Eq("status", enums.IMConversationStatusActive).Desc("last_active_at").Desc("id")
	case request.AgentConversationFilterPending:
		cnd.Eq("current_assignee_id", 0).Eq("status", enums.IMConversationStatusPending).Asc("last_active_at").Desc("id")
		if restricted {
			if len(viewerTeamIDs) == 0 {
				cnd.Eq("id", -1)
			} else {
				cnd.In("current_team_id", viewerTeamIDs)
			}
		}
	case request.AgentConversationFilterClosed:
		cnd.Eq("current_assignee_id", userID).Eq("status", enums.IMConversationStatusClosed).Desc("last_active_at").Desc("id")
	default:
		return nil, nil, errorsx.InvalidParamI18n("error.e0121")
	}

	list, paging := repositories.ConversationRepository.FindPageByCnd(sqls.DB(), cnd)
	return list, paging, nil
}

func (s *conversationService) Updates(id int64, columns map[string]interface{}) error {
	return repositories.ConversationRepository.Updates(sqls.DB(), id, columns)
}

func (s *conversationService) getLatestNotFinishedByCustomerID(db *gorm.DB, customerID int64) *models.Conversation {
	if customerID <= 0 {
		return nil
	}
	cnd := sqls.NewCnd()
	cnd.Eq("customer_id", customerID)
	cnd.In("status", []enums.IMConversationStatus{
		enums.IMConversationStatusAIServing,
		enums.IMConversationStatusPending,
		enums.IMConversationStatusActive,
	})
	cnd.Desc("id")
	return repositories.ConversationRepository.FindOne(db, cnd)
}

func (s *conversationService) Create(externalUser openidentity.ExternalUser, channelID, aiAgentID int64) (*models.Conversation, error) {
	return s.CreateWithContext(externalUser, channelID, aiAgentID, request.CreateOrMatchConversationRequest{})
}

func (s *conversationService) CreateWithContext(externalUser openidentity.ExternalUser, channelID, aiAgentID int64, req request.CreateOrMatchConversationRequest) (*models.Conversation, error) {
	var conversation *models.Conversation
	var aiAgent *models.AIAgent
	var welcomeMessage *models.Message
	created := false
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		unlockExternalScope, err := s.lockExternalCreateScopeForTx(ctx.Tx, externalUser)
		if err != nil {
			return err
		}
		defer unlockExternalScope()
		customerID, err := CustomerService.EnsureExternalCustomer(ctx, externalUser)
		if err != nil {
			return err
		}
		createCtx, err := s.resolveCreateContext(ctx.Tx, customerID, externalUser, req)
		if err != nil {
			return err
		}
		unlockCreateScope, err := s.lockCreateScopeForTx(ctx.Tx, customerID, createCtx)
		if err != nil {
			return err
		}
		defer unlockCreateScope()
		customerName := s.getCustomerName(ctx.Tx, customerID)
		existing := s.findConversationByCreateIdempotency(ctx.Tx, customerID, createCtx, externalUser, req.IdempotencyKey)
		if existing == nil && !req.ForceNew {
			existing = s.findExistingConversationForCreate(ctx.Tx, customerID, createCtx)
		}
		if existing != nil {
			matchesParticipant, participantCount := s.customerParticipantAccess(ctx.Tx, existing.ID, externalUser)
			if participantCount > 0 && !matchesParticipant {
				existing = nil
			} else if participantCount == 0 {
				if err := ConversationParticipantService.CreateCustomerParticipant(ctx, existing.ID, externalUser); err != nil {
					return err
				}
			}
		}
		if existing != nil {
			conversation = existing
			if customerName != "" && existing.CustomerName != customerName {
				if err := repositories.ConversationRepository.Updates(ctx.Tx, existing.ID, map[string]any{
					"customer_name": customerName,
					"updated_at":    time.Now(),
				}); err != nil {
					return err
				}
				conversation.CustomerName = customerName
			}
			return nil
		}
		aiAgent, err = s.resolveConversationAIAgent(ctx.Tx, createCtx, aiAgentID)
		if err != nil {
			return err
		}
		if aiAgent.ServiceMode == enums.IMConversationServiceModeHumanOnly && createCtx.tenantID <= 0 {
			return errorsx.InvalidParam("human handoff requires tenant context")
		}
		created = true
		now := time.Now()
		conversation = &models.Conversation{
			AIAgentID:              aiAgent.ID,
			ChannelID:              channelID,
			CustomerID:             customerID,
			CustomerName:           customerName,
			TenantID:               createCtx.tenantID,
			ProductID:              createCtx.productID,
			ProductModelID:         createCtx.productModelID,
			DeviceID:               createCtx.deviceID,
			ServiceCodeID:          createCtx.serviceCodeID,
			CustomerEntrySessionID: createCtx.customerEntrySessionID,
			Status:                 s.resolveInitialStatus(aiAgent.ServiceMode),
			ServiceMode:            aiAgent.ServiceMode,
			Priority:               0,
			CurrentAssigneeID:      0,
			CurrentTeamID:          0,
			LastMessageAt:          now,
			LastActiveAt:           now,
			AuditFields:            utils.BuildAuditFields(nil),
		}
		if err := ctx.Tx.Create(conversation).Error; err != nil {
			return err
		}
		if err := ConversationParticipantService.CreateCustomerParticipant(ctx, conversation.ID, externalUser); err != nil {
			return err
		}
		if err := ConversationEventLogService.CreateEventWithRequestID(ctx, conversation.ID, req.IdempotencyKey, enums.IMEventTypeCreate, enums.IMSenderTypeCustomer, 0, "用户创建会话", ""); err != nil {
			return err
		}
		welcomeMessage, err = MessageService.createAIWelcomeMessage(ctx, conversation, aiAgent, createCtx.locale, now)
		return err
	}); err != nil {
		return nil, err
	}
	if conversation == nil {
		return nil, errorsx.BusinessErrorI18n(1, "error.conversation.createFailed")
	}
	if !created {
		if TenantCapabilityService.AIDisabled(conversation.TenantID) {
			if err := ConversationHumanDispatchService.RequestByCustomer(
				conversation.ID,
				externalUser,
				"租户未启用 AI，直接进入人工服务",
				"",
			); err != nil {
				return nil, err
			}
			conversation = s.Get(conversation.ID)
		}
		return conversation, nil
	}

	// 推送会话创建事件
	WsService.PublishConversationChanged(conversation, enums.IMRealtimeEventConversationCreated)
	if welcomeMessage != nil {
		if updatedConversation := s.Get(conversation.ID); updatedConversation != nil {
			WsService.PublishMessageCreated(updatedConversation, welcomeMessage)
			WsService.PublishConversationChanged(updatedConversation, enums.IMRealtimeEventConversationUpdated)
		}
	}

	if aiAgent != nil && aiAgent.ServiceMode == enums.IMConversationServiceModeHumanOnly {
		if _, err := ConversationHumanDispatchService.ApplyHumanOnlyCreate(conversation.ID, *aiAgent); err != nil {
			return nil, err
		}
	}
	return s.Get(conversation.ID), nil
}

func (s *conversationService) findConversationByCreateIdempotency(
	db *gorm.DB,
	customerID int64,
	createCtx resolvedConversationCreateContext,
	externalUser openidentity.ExternalUser,
	idempotencyKey string,
) *models.Conversation {
	idempotencyKey = tracex.NormalizeRequestID(idempotencyKey)
	if db == nil || customerID <= 0 || idempotencyKey == "" {
		return nil
	}
	events := repositories.ConversationEventLogRepository.Find(db, sqls.NewCnd().
		Eq("request_id", idempotencyKey).
		Eq("event_type", enums.IMEventTypeCreate).
		Desc("id"))
	for i := range events {
		conversation := repositories.ConversationRepository.Get(db, events[i].ConversationID)
		if !conversationMatchesCreateScope(conversation, customerID, createCtx) {
			continue
		}
		matchesParticipant, participantCount := s.customerParticipantAccess(db, conversation.ID, externalUser)
		if participantCount == 0 || matchesParticipant {
			return conversation
		}
	}
	return nil
}

func conversationMatchesCreateScope(conversation *models.Conversation, customerID int64, createCtx resolvedConversationCreateContext) bool {
	if conversation == nil || conversation.CustomerID != customerID ||
		conversation.TenantID != createCtx.tenantID || conversation.ProductID != createCtx.productID ||
		conversation.DeviceID != createCtx.deviceID {
		return false
	}
	return true
}

func (s *conversationService) lockExternalCreateScopeForTx(db *gorm.DB, externalUser openidentity.ExternalUser) (func(), error) {
	if db != nil && db.Dialector != nil && db.Dialector.Name() == "postgres" {
		return func() {}, nil
	}
	externalSource := strings.TrimSpace(string(externalUser.ExternalSource))
	externalID := strings.TrimSpace(externalUser.ExternalID)
	if externalSource == "" || externalID == "" {
		return func() {}, nil
	}
	key := strings.Join([]string{
		"remotehelpdesk:conversation:create:external",
		externalSource,
		externalID,
	}, ":")
	lock := acquireLocalConversationCreateScopeLock(key)
	return func() { releaseLocalConversationCreateScopeLock(key, lock) }, nil
}

func (s *conversationService) lockCreateScopeForTx(db *gorm.DB, customerID int64, createCtx resolvedConversationCreateContext) (func(), error) {
	key := conversationCreateScopeKey(customerID, createCtx)
	if key == "" {
		return func() {}, nil
	}
	if db != nil && db.Dialector != nil && db.Dialector.Name() == "postgres" {
		if err := db.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error; err != nil {
			return nil, fmt.Errorf("lock conversation create scope: %w", err)
		}
		return func() {}, nil
	}

	lock := acquireLocalConversationCreateScopeLock(key)
	return func() { releaseLocalConversationCreateScopeLock(key, lock) }, nil
}

func conversationCreateScopeKey(customerID int64, createCtx resolvedConversationCreateContext) string {
	if customerID <= 0 {
		return ""
	}
	productID := createCtx.productID
	if productID < 0 {
		productID = 0
	}
	deviceID := createCtx.deviceID
	if deviceID < 0 {
		deviceID = 0
	}
	return strings.Join([]string{
		"remotehelpdesk:conversation:create",
		"tenant", strconv.FormatInt(createCtx.tenantID, 10),
		"customer", strconv.FormatInt(customerID, 10),
		"product", strconv.FormatInt(productID, 10),
		"device", strconv.FormatInt(deviceID, 10),
	}, ":")
}

func acquireLocalConversationCreateScopeLock(key string) *conversationCreateScopeLock {
	conversationCreateScopeLockRegistry.Lock()
	lock := conversationCreateScopeLockRegistry.locks[key]
	if lock == nil {
		lock = &conversationCreateScopeLock{}
		conversationCreateScopeLockRegistry.locks[key] = lock
	}
	lock.refs++
	conversationCreateScopeLockRegistry.Unlock()

	lock.mu.Lock()
	return lock
}

func releaseLocalConversationCreateScopeLock(key string, lock *conversationCreateScopeLock) {
	if lock == nil {
		return
	}
	lock.mu.Unlock()

	conversationCreateScopeLockRegistry.Lock()
	if current := conversationCreateScopeLockRegistry.locks[key]; current == lock {
		lock.refs--
		if lock.refs <= 0 {
			delete(conversationCreateScopeLockRegistry.locks, key)
		}
	}
	conversationCreateScopeLockRegistry.Unlock()
}

func (s *conversationService) resolveConversationAIAgent(db *gorm.DB, createCtx resolvedConversationCreateContext, fallbackAgentID int64) (*models.AIAgent, error) {
	if TenantCapabilityService.AIDisabledDB(db, createCtx.tenantID) {
		return s.ticketOnlyConversationAgent(createCtx), nil
	}
	agentID := int64(0)
	if createCtx.deviceID > 0 && createCtx.productID > 0 {
		profile := repositories.ProductServiceProfileRepository.GetByProductID(db, createCtx.productID)
		if profile == nil || profile.Status != enums.StatusOk || profile.TenantID != createCtx.tenantID || profile.DefaultAIAgentID <= 0 {
			return nil, errorsx.InvalidParam("product AI agent is not configured")
		}
		agentID = profile.DefaultAIAgentID
	}
	if agentID <= 0 && createCtx.tenantID > 0 && createCtx.deviceID == 0 {
		if generalAgent := repositories.AIAgentRepository.FindOne(db, sqls.NewCnd().
			Eq("tenant_id", createCtx.tenantID).
			Eq("product_id", 0).
			Eq("source", TenantDefaultAIAgentSource).
			Eq("status", enums.StatusOk).
			Asc("sort_no").
			Asc("id")); generalAgent != nil {
			agentID = generalAgent.ID
		}
	}
	if agentID <= 0 && createCtx.deviceID == 0 {
		if createCtx.tenantID > 0 {
			return nil, errorsx.InvalidParam("tenant default AI agent is not configured")
		}
		agentID = fallbackAgentID
	}
	agent := repositories.AIAgentRepository.Get(db, agentID)
	if agent == nil || agent.Status != enums.StatusOk {
		return nil, errorsx.InvalidParam("AI agent is not enabled")
	}
	if createCtx.tenantID > 0 && agent.TenantID > 0 && agent.TenantID != createCtx.tenantID {
		return nil, errorsx.Forbidden("AI agent does not belong to the customer tenant")
	}
	if createCtx.deviceID > 0 && createCtx.productID > 0 {
		if agent.TenantID != createCtx.tenantID || agent.ProductID != createCtx.productID {
			return nil, errorsx.Forbidden("AI agent does not belong to the customer product")
		}
	}
	if createCtx.tenantID > 0 && createCtx.deviceID == 0 && agent.Source != TenantDefaultAIAgentSource {
		return nil, errorsx.InvalidParam("tenant default AI agent is not configured")
	}
	if createCtx.deviceID > 0 || agent.Source == TenantDefaultAIAgentSource {
		runtimeAgent, release, err := AIAgentReleaseService.MaterializeRuntimeAgent(db, *agent)
		if err != nil || release == nil {
			if createCtx.deviceID > 0 {
				return nil, errorsx.InvalidParam("product AI agent must have an approved active release")
			}
			return nil, errorsx.InvalidParam("tenant default AI agent must have an approved active release")
		}
		if agent.Source == TenantDefaultAIAgentSource {
			workflow := repositories.AIWorkflowRepository.Get(db, release.WorkflowID)
			version := repositories.AIWorkflowVersionRepository.Get(db, release.WorkflowVersionID)
			tenant := repositories.PlatformIAMRepository.GetTenant(db, createCtx.tenantID)
			if workflow == nil || workflow.Code != TenantDefaultWorkflowCode(tenant) || version == nil || version.ReleaseChannel != models.AIWorkflowReleaseChannelStable {
				return nil, errorsx.InvalidParam("tenant default AI agent is not bound to an active AI diagnosis release")
			}
		}
		return &runtimeAgent, nil
	}
	return agent, nil
}

func (s *conversationService) ticketOnlyConversationAgent(createCtx resolvedConversationCreateContext) *models.AIAgent {
	teamIDs := ""
	if createCtx.tenantID > 0 && createCtx.productID > 0 {
		if team := ProductSupportOrganizationService.FindProductRepairTeam(sqls.DB(), createCtx.tenantID, createCtx.productID); team != nil {
			teamIDs = strconv.FormatInt(team.ID, 10)
		}
	}
	return &models.AIAgent{
		TenantID:    createCtx.tenantID,
		ProductID:   createCtx.productID,
		Source:      "ticket_only",
		Name:        "纯工单系统",
		Status:      enums.StatusOk,
		ServiceMode: enums.IMConversationServiceModeHumanOnly,
		TeamIDs:     teamIDs,
	}
}

func (s *conversationService) findExistingConversationForCreate(db *gorm.DB, customerID int64, createCtx resolvedConversationCreateContext) *models.Conversation {
	if customerID <= 0 {
		return nil
	}
	cnd := sqls.NewCnd().Eq("customer_id", customerID)
	cnd.In("status", []enums.IMConversationStatus{
		enums.IMConversationStatusAIServing,
		enums.IMConversationStatusPending,
		enums.IMConversationStatusActive,
	})
	if createCtx.deviceID > 0 {
		cnd.Eq("device_id", createCtx.deviceID)
	} else {
		cnd.Eq("device_id", 0)
	}
	if createCtx.tenantID > 0 {
		cnd.Eq("tenant_id", createCtx.tenantID)
	}
	if createCtx.productID > 0 {
		cnd.Eq("product_id", createCtx.productID)
	}
	cnd.Desc("id")
	return repositories.ConversationRepository.FindOne(db, cnd)
}

func (s *conversationService) resolveCreateContext(db *gorm.DB, customerID int64, externalUser openidentity.ExternalUser, req request.CreateOrMatchConversationRequest) (resolvedConversationCreateContext, error) {
	ctx := resolvedConversationCreateContext{
		customerEntrySessionID: req.CustomerEntrySessionID,
		tenantID:               req.ContextTenantID,
		productID:              req.ContextProductID,
		locale:                 strings.TrimSpace(req.Locale),
	}
	if customerID <= 0 {
		return ctx, errorsx.UnauthorizedI18n("error.e0149")
	}
	var entrySession *models.CustomerEntrySession
	if req.CustomerEntrySessionID > 0 {
		entrySession = repositories.CustomerEntrySessionRepository.FindActive(db, req.CustomerEntrySessionID, time.Now())
		session := entrySession
		if session == nil {
			return ctx, errorsx.InvalidParam("customer entry session is inactive or expired")
		}
		if strings.TrimSpace(session.VisitorID) == "" || entrySessionExternalID(session.ID, session.VisitorID) != strings.TrimSpace(externalUser.ExternalID) {
			return ctx, errorsx.Forbidden("customer entry session does not belong to current visitor")
		}
		if err := CustomerEntryService.RequirePrivacyConsent(session); err != nil {
			return ctx, err
		}
		ctx.customerEntrySessionID = session.ID
		ctx.tenantID = session.TenantID
		if ctx.locale == "" {
			ctx.locale = session.Locale
		}
		if !req.General {
			ctx.productID = session.ProductID
			ctx.productModelID = session.ProductModelID
			var snapshot struct {
				TenantID       int64 `json:"tenantId"`
				ProductID      int64 `json:"productId"`
				ProductModelID int64 `json:"productModelId"`
			}
			if json.Unmarshal([]byte(session.EntryContextJSON), &snapshot) == nil {
				if ctx.tenantID == 0 {
					ctx.tenantID = snapshot.TenantID
				}
				if ctx.productID == 0 {
					ctx.productID = snapshot.ProductID
				}
				if ctx.productModelID == 0 {
					ctx.productModelID = snapshot.ProductModelID
				}
			}
			if req.DeviceID == 0 {
				req.DeviceID = session.DeviceID
			}
			if req.ServiceCodeID == 0 {
				req.ServiceCodeID = session.ServiceCodeID
			}
		}
	}
	ctx.locale = i18nx.NormalizeLocale(ctx.locale)
	if tenant := repositories.PlatformIAMRepository.GetTenant(db, ctx.tenantID); tenant != nil && tenant.IsKnowledgeSupportScene() {
		ctx.productID = 0
		ctx.productModelID = 0
		ctx.deviceID = 0
		ctx.serviceCodeID = 0
		ctx.customerEntrySessionID = 0
		return ctx, nil
	}
	if req.General {
		if ctx.tenantID <= 0 {
			return ctx, errorsx.InvalidParam("general conversation tenant context is required")
		}
		ctx.productID = 0
		ctx.productModelID = 0
		ctx.deviceID = 0
		ctx.serviceCodeID = 0
		ctx.customerEntrySessionID = 0
		return ctx, nil
	}
	if req.DeviceID <= 0 {
		if ctx.customerEntrySessionID <= 0 {
			if ctx.productID > 0 {
				if ctx.tenantID <= 0 {
					return ctx, errorsx.InvalidParam("customer product tenant context is required")
				}
				product := repositories.ProductRepository.Get(db, ctx.productID)
				if product == nil || product.TenantID != ctx.tenantID || product.Status != enums.StatusOk {
					return ctx, errorsx.InvalidParam("customer product is not available")
				}
			}
			return ctx, nil
		}
		if ctx.tenantID <= 0 || ctx.productID <= 0 {
			return ctx, errorsx.InvalidParam("customer entry session product context is required")
		}
		product := repositories.ProductRepository.Get(db, ctx.productID)
		if product == nil || product.TenantID != ctx.tenantID || product.Status != enums.StatusOk {
			return ctx, errorsx.InvalidParam("customer entry product is not available")
		}
		if req.ServiceCodeID > 0 {
			serviceCode := repositories.ServiceCodeRepository.Get(db, req.ServiceCodeID)
			if serviceCode == nil || serviceCode.TenantID != ctx.tenantID || serviceCode.ProductID != ctx.productID {
				return ctx, errorsx.InvalidParam("service code is not valid for product")
			}
			ctx.serviceCodeID = serviceCode.ID
		}
		return ctx, nil
	}
	device := repositories.DeviceRepository.Get(db, req.DeviceID)
	if device == nil || device.Status == enums.StatusDeleted {
		return ctx, errorsx.InvalidParam("device not found")
	}
	customer := repositories.CustomerRepository.Get(db, customerID)
	if customer == nil || customer.Status == enums.StatusDeleted {
		return ctx, errorsx.Unauthorized("customer is not available")
	}
	entrySessionAuthorizesDevice := entrySession != nil && entrySession.DeviceID == device.ID && entrySession.TenantID == device.TenantID
	if !entrySessionAuthorizesDevice {
		bindingCustomerUserID, bindingCustomerOrgID := customerBindingScope(externalUser, customer)
		binding := repositories.CustomerDeviceBindingRepository.FindVisibleForCustomer(db, device.TenantID, device.ID, bindingCustomerUserID, bindingCustomerOrgID)
		if binding == nil {
			return ctx, errorsx.ForbiddenI18n("error.conversation.deviceNotVisible")
		}
	}
	ctx.tenantID = device.TenantID
	ctx.productID = device.ProductID
	ctx.productModelID = device.ProductModelID
	ctx.deviceID = device.ID
	if req.ServiceCodeID > 0 {
		serviceCode := repositories.ServiceCodeRepository.Get(db, req.ServiceCodeID)
		if serviceCode == nil || serviceCode.TenantID != device.TenantID || serviceCode.DeviceID != 0 && serviceCode.DeviceID != device.ID {
			return ctx, errorsx.InvalidParam("service code is not valid for device")
		}
		ctx.serviceCodeID = serviceCode.ID
	} else if serviceCode := repositories.ServiceCodeRepository.GetActiveByDeviceID(db, device.ID); serviceCode != nil && serviceCode.TenantID == device.TenantID {
		ctx.serviceCodeID = serviceCode.ID
	}
	return ctx, nil
}

func (s *conversationService) AssignConversation(req request.AssignConversationRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	targetProfile := AgentProfileService.GetByUserID(req.AssigneeID)
	if targetProfile == nil || targetProfile.Status != enums.StatusOk {
		return errorsx.InvalidParamI18n("error.e0276")
	}
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		conversation := repositories.ConversationRepository.Get(ctx.Tx, req.ConversationID)
		if conversation == nil {
			return errorsx.InvalidParamI18n("error.e0116")
		}
		if !s.CanAccessConversation(conversation, operator) {
			return errorsx.ForbiddenI18n("error.e0225")
		}
		if conversation.Status != enums.IMConversationStatusPending {
			return errorsx.InvalidParamI18n("error.e0135")
		}
		targetTeamID, err := s.resolveAssignmentTeam(ctx.Tx, conversation, targetProfile)
		if err != nil {
			return err
		}
		now := time.Now()
		if err := validateManualConversationAssigneeDB(ctx.Tx, conversation, req.AssigneeID, targetTeamID, now); err != nil {
			return err
		}
		if err := ConversationAssignmentService.FinishActiveAssignments(ctx, req.ConversationID, now); err != nil {
			return err
		}
		assignment, err := ConversationAssignmentService.CreateAssignment(ctx, req.ConversationID, conversation.CurrentAssigneeID, req.AssigneeID, enums.IMAssignmentTypeAssign, req.Reason, operator, now)
		if err != nil {
			return err
		}
		if err := s.ensureAgentParticipantTx(ctx.Tx, req.ConversationID, req.AssigneeID, now, operator); err != nil {
			return err
		}
		if err := repositories.ConversationRepository.Updates(ctx.Tx, req.ConversationID, map[string]any{
			"current_assignee_id": req.AssigneeID,
			"current_team_id":     targetTeamID,
			"status":              enums.IMConversationStatusPending,
			"update_user_id":      operator.UserID,
			"update_user_name":    operator.Username,
			"updated_at":          now,
		}); err != nil {
			return err
		}
		if err := TicketService.SyncConversationDispatchTx(ctx.Tx, req.ConversationID, targetTeamID, req.AssigneeID, req.Reason, operator); err != nil {
			return err
		}
		if err := ConversationEventLogService.CreateEvent(ctx, req.ConversationID, enums.IMEventTypeAssign, enums.IMSenderTypeAgent, operator.UserID, "会话已分配", s.buildEventPayload(map[string]any{
			"fromStatus":     conversation.Status,
			"toStatus":       enums.IMConversationStatusPending,
			"fromTeamId":     conversation.CurrentTeamID,
			"toTeamId":       targetTeamID,
			"fromAssigneeId": conversation.CurrentAssigneeID,
			"toAssigneeId":   req.AssigneeID,
			"reason":         strings.TrimSpace(req.Reason),
		})); err != nil {
			return err
		}
		assignedEvent := events.ConversationAssignedEvent{
			ConversationID: req.ConversationID,
			FromUserID:     conversation.CurrentAssigneeID,
			ToUserID:       req.AssigneeID,
			OperatorID:     operator.UserID,
			Reason:         strings.TrimSpace(req.Reason),
			AssignType:     events.ConversationAssignTypeAssign,
		}
		if err := enqueueConversationAssignedEventTx(ctx.Tx, conversation.TenantID, assignment.ID, &assignedEvent, now); err != nil {
			return err
		}
		if ctx.RegisterCallback != nil {
			ctx.RegisterCallback(eventbus.WakeDefaultOutboxPublisher)
		}
		return nil
	}); err != nil {
		return err
	}
	if conversation := s.Get(req.ConversationID); conversation != nil {
		WsService.PublishConversationChanged(conversation, enums.IMRealtimeEventConversationAssigned)
	}
	return nil
}

func (s *conversationService) resolveAssignmentTeam(db *gorm.DB, conversation *models.Conversation, targetProfile *models.AgentProfile) (int64, error) {
	if db == nil || conversation == nil || targetProfile == nil {
		return 0, errorsx.InvalidParamI18n("error.e0276")
	}
	if conversation.TenantID > 0 && targetProfile.TenantID != conversation.TenantID {
		return 0, errorsx.Forbidden("assignee is outside the conversation tenant")
	}
	if conversation.TenantID <= 0 {
		return conversation.CurrentTeamID, nil
	}
	targetTeamID := conversation.CurrentTeamID
	if targetTeamID <= 0 && conversation.TenantID > 0 && conversation.ProductID > 0 {
		if team := ProductSupportOrganizationService.FindProductRepairTeam(db, conversation.TenantID, conversation.ProductID); team != nil {
			targetTeamID = team.ID
		}
	}
	if targetTeamID <= 0 {
		teamIDs := AgentTeamMemberService.FindTeamIDsByUserID(db, conversation.TenantID, targetProfile.UserID)
		if len(teamIDs) > 0 {
			targetTeamID = teamIDs[0]
		}
	}
	if targetTeamID <= 0 || conversation.TenantID <= 0 {
		return targetTeamID, nil
	}
	team := repositories.AgentTeamRepository.Get(db, targetTeamID)
	if team == nil || team.TenantID != conversation.TenantID || team.Status != enums.StatusOk {
		return 0, errorsx.InvalidParam("conversation repair team is invalid")
	}
	if conversation.ProductID > 0 && team.ProductID > 0 && team.ProductID != conversation.ProductID {
		return 0, errorsx.Forbidden("conversation repair team does not match the product")
	}
	if AgentTeamMemberService.tableReady(db) && !AgentTeamMemberService.IsUserActiveMemberOfTeamDB(db, conversation.TenantID, targetTeamID, targetProfile.UserID) {
		return 0, errorsx.Forbidden("assignee is not a member of the product repair team")
	}
	return targetTeamID, nil
}

func validateManualConversationAssigneeDB(db *gorm.DB, conversation *models.Conversation, assigneeID, teamID int64, now time.Time) error {
	return validateManualConversationAssigneeWithOptionsDB(db, conversation, assigneeID, teamID, now, true)
}

func validateManualConversationTransferAssigneeDB(db *gorm.DB, conversation *models.Conversation, assigneeID, teamID int64, now time.Time) error {
	return validateManualConversationAssigneeWithOptionsDB(db, conversation, assigneeID, teamID, now, false)
}

func validateManualConversationAssigneeWithOptionsDB(
	db *gorm.DB,
	conversation *models.Conversation,
	assigneeID,
	teamID int64,
	now time.Time,
	requireLinkedTicketCapability bool,
) error {
	if assigneeID <= 0 {
		return errorsx.InvalidParamI18n("error.e0334")
	}
	if db == nil {
		return nil
	}
	if db.Migrator().HasTable(&models.User{}) {
		user := repositories.UserRepository.Get(db, assigneeID)
		if user == nil || user.Status != enums.StatusOk {
			return errorsx.InvalidParamI18n("error.e0334")
		}
	}
	if conversation == nil || conversation.TenantID <= 0 {
		return nil
	}
	var profile *models.AgentProfile
	if db.Migrator().HasTable(&models.AgentProfile{}) {
		profile = repositories.AgentProfileRepository.FindOne(db, sqls.NewCnd().
			Eq("tenant_id", conversation.TenantID).
			Eq("user_id", assigneeID).
			Eq("status", enums.StatusOk))
		if profile == nil {
			return errorsx.InvalidParam("assignee must have an active engineer profile")
		}
		if teamID <= 0 {
			teamIDs := AgentTeamMemberService.FindTeamIDsByUserID(db, conversation.TenantID, assigneeID)
			if len(teamIDs) > 0 {
				teamID = teamIDs[0]
			}
		}
	}
	if teamID <= 0 {
		return errorsx.InvalidParam("assignee has no product repair team")
	}
	if db.Migrator().HasTable(&models.AgentTeam{}) {
		team := repositories.AgentTeamRepository.Get(db, teamID)
		if team == nil || team.TenantID != conversation.TenantID || team.Status != enums.StatusOk {
			return errorsx.InvalidParam("conversation repair team is not active")
		}
		if conversation.ProductID > 0 && team.ProductID > 0 && team.ProductID != conversation.ProductID {
			return errorsx.Forbidden("conversation repair team does not match the product")
		}
	}
	if !AgentTeamMemberService.IsUserActiveMemberOfTeamDB(db, conversation.TenantID, teamID, assigneeID) {
		return errorsx.InvalidParam("assignee must be a member of the conversation repair team")
	}
	if requireLinkedTicketCapability {
		if err := validateManualConversationLinkedTicketCapabilityDB(db, conversation, assigneeID); err != nil {
			return err
		}
	}
	return nil
}

func (s *conversationService) AutoAssignConversation(conversationID int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}

	conversation := s.Get(conversationID)
	if conversation == nil {
		return errorsx.InvalidParamI18n("error.e0116")
	}
	if !s.CanAccessConversation(conversation, operator) {
		return errorsx.ForbiddenI18n("error.e0225")
	}
	if conversation.Status != enums.IMConversationStatusPending {
		return errorsx.InvalidParamI18n("error.e0136")
	}
	if conversation.CurrentAssigneeID > 0 {
		return errorsx.InvalidParamI18n("error.e0190")
	}

	aiAgent := AIAgentService.Get(conversation.AIAgentID)
	if aiAgent == nil || aiAgent.Status != enums.StatusOk {
		return errorsx.InvalidParamI18n("error.e0003")
	}
	result, err := ConversationHumanDispatchService.DispatchPendingConversation(conversationID, *aiAgent)
	if err != nil {
		return err
	}
	if result == nil || result.Decision == HandoffDecisionOffHours {
		return errorsx.InvalidParamI18n("error.e0194")
	}
	return nil
}

func (s *conversationService) TransferConversation(conversationID, toUserID int64, reason string, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if toUserID <= 0 {
		return errorsx.InvalidParamI18n("error.e0278")
	}
	targetProfile := AgentProfileService.GetByUserID(toUserID)
	if targetProfile == nil || targetProfile.Status != enums.StatusOk {
		return errorsx.InvalidParamI18n("error.e0276")
	}
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		conversation := repositories.ConversationRepository.Get(ctx.Tx, conversationID)
		if conversation == nil {
			return errorsx.InvalidParamI18n("error.e0116")
		}
		if !s.canTransferConversation(conversation, operator) {
			return errorsx.ForbiddenI18n("error.e0223")
		}
		if conversation.Status != enums.IMConversationStatusActive {
			return errorsx.InvalidParamI18n("error.e0134")
		}
		if conversation.CurrentAssigneeID <= 0 {
			return errorsx.InvalidParamI18n("error.e0193")
		}
		if conversation.CurrentAssigneeID == toUserID {
			return errorsx.InvalidParamI18n("error.e0277")
		}
		targetTeamID, err := s.resolveTransferTeam(ctx.Tx, conversation, targetProfile)
		if err != nil {
			return err
		}
		now := time.Now()
		if err := validateManualConversationTransferAssigneeDB(ctx.Tx, conversation, toUserID, targetTeamID, now); err != nil {
			return err
		}
		if err := ConversationAssignmentService.FinishActiveAssignments(ctx, conversationID, now); err != nil {
			return err
		}
		assignment, err := ConversationAssignmentService.CreateAssignment(ctx, conversationID, conversation.CurrentAssigneeID, toUserID, enums.IMAssignmentTypeTransfer, reason, operator, now)
		if err != nil {
			return err
		}
		if err := s.ensureAgentParticipantTx(ctx.Tx, conversationID, toUserID, now, operator); err != nil {
			return err
		}
		if err := repositories.ConversationRepository.Updates(ctx.Tx, conversationID, map[string]any{
			"current_assignee_id": toUserID,
			"current_team_id":     targetTeamID,
			"status":              enums.IMConversationStatusActive,
			"update_user_id":      operator.UserID,
			"update_user_name":    operator.Username,
			"updated_at":          now,
		}); err != nil {
			return err
		}
		if err := TicketService.SyncConversationDispatchTx(ctx.Tx, conversationID, targetTeamID, toUserID, reason, operator); err != nil {
			return err
		}
		if err := ConversationEventLogService.CreateEvent(ctx, conversationID, enums.IMEventTypeTransfer, enums.IMSenderTypeAgent, operator.UserID, "会话已转接", s.buildEventPayload(map[string]any{
			"fromStatus":     conversation.Status,
			"toStatus":       enums.IMConversationStatusActive,
			"fromTeamId":     conversation.CurrentTeamID,
			"toTeamId":       targetTeamID,
			"fromAssigneeId": conversation.CurrentAssigneeID,
			"toAssigneeId":   toUserID,
			"reason":         strings.TrimSpace(reason),
		})); err != nil {
			return err
		}
		assignedEvent := events.ConversationAssignedEvent{
			ConversationID: conversationID,
			FromUserID:     conversation.CurrentAssigneeID,
			ToUserID:       toUserID,
			OperatorID:     operator.UserID,
			Reason:         strings.TrimSpace(reason),
			AssignType:     events.ConversationAssignTypeTransfer,
		}
		if err := enqueueConversationAssignedEventTx(ctx.Tx, conversation.TenantID, assignment.ID, &assignedEvent, now); err != nil {
			return err
		}
		if ctx.RegisterCallback != nil {
			ctx.RegisterCallback(eventbus.WakeDefaultOutboxPublisher)
		}
		return nil
	}); err != nil {
		return err
	}
	if conversation := s.Get(conversationID); conversation != nil {
		WsService.PublishConversationChanged(conversation, enums.IMRealtimeEventConversationTransferred)
	}
	return nil
}

func (s *conversationService) resolveTransferTeam(db *gorm.DB, conversation *models.Conversation, targetProfile *models.AgentProfile) (int64, error) {
	if db == nil || conversation == nil || targetProfile == nil {
		return 0, errorsx.InvalidParamI18n("error.e0276")
	}
	if conversation.TenantID > 0 && targetProfile.TenantID != conversation.TenantID {
		return 0, errorsx.Forbidden("assignee is outside the conversation tenant")
	}
	if conversation.TenantID > 0 && conversation.ProductID > 0 {
		if team := ProductSupportOrganizationService.FindProductRepairTeam(db, conversation.TenantID, conversation.ProductID); team != nil {
			if AgentTeamMemberService.IsUserActiveMemberOfTeamDB(db, conversation.TenantID, team.ID, targetProfile.UserID) {
				return team.ID, nil
			}
			return 0, errorsx.Forbidden("assignee is not a member of the product repair team")
		}
		return 0, errorsx.InvalidParam("product repair team is not active")
	}
	if conversation.TenantID > 0 && conversation.CurrentTeamID > 0 {
		if AgentTeamMemberService.IsUserActiveMemberOfTeamDB(db, conversation.TenantID, conversation.CurrentTeamID, targetProfile.UserID) {
			return conversation.CurrentTeamID, nil
		}
	}
	if conversation.TenantID > 0 {
		teamIDs := AgentTeamMemberService.FindTeamIDsByUserID(db, conversation.TenantID, targetProfile.UserID)
		if len(teamIDs) > 0 {
			return teamIDs[0], nil
		}
	}
	return s.resolveAssignmentTeam(db, conversation, targetProfile)
}

func (s *conversationService) HandoffByAI(conversationID int64, aiAgent models.AIAgent, reason string) error {
	return s.HandoffByAIWithRequestID(conversationID, aiAgent, reason, "")
}

func (s *conversationService) HandoffByAIWithRequestID(conversationID int64, aiAgent models.AIAgent, reason string, requestID string) error {
	if conversationID <= 0 {
		return errorsx.InvalidParamI18n("error.e0116")
	}
	_, err := ConversationHumanDispatchService.HandoffByAIWithRequestID(conversationID, aiAgent, reason, requestID)
	if err != nil {
		slog.Warn("schedule-aware ai handoff failed",
			"requestId", requestID,
			"conversation_id", conversationID,
			"ai_agent_id", aiAgent.ID,
			"error", err)
	}
	return err
}

func (s *conversationService) TryOffHoursHandoffByAI(conversationID int64, aiAgent models.AIAgent, reason string) (bool, error) {
	return s.TryOffHoursHandoffByAIWithRequestID(conversationID, aiAgent, reason, "")
}

func (s *conversationService) TryOffHoursHandoffByAIWithRequestID(conversationID int64, aiAgent models.AIAgent, reason string, requestID string) (bool, error) {
	if conversationID <= 0 {
		return false, errorsx.InvalidParamI18n("error.e0116")
	}
	handled, err := ConversationHumanDispatchService.TryOffHoursHandoffByAIWithRequestID(conversationID, aiAgent, reason, requestID)
	if err != nil {
		slog.Warn("off-hours ai handoff failed",
			"requestId", requestID,
			"conversation_id", conversationID,
			"ai_agent_id", aiAgent.ID,
			"error", err)
	}
	return handled, err
}

func (s *conversationService) CloseConversation(conversationID int64, closeReason string, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return s.closeConversation(conversationID, enums.IMSenderTypeAgent, closeReason, operator)
}

func (s *conversationService) CloseCustomerConversation(conversationID int64, externalUser openidentity.ExternalUser) error {
	conversation := s.Get(conversationID)
	if conversation == nil {
		return errorsx.InvalidParamI18n("error.e0116")
	}
	if !s.IsCustomerConversationOwner(conversation, externalUser) {
		return errorsx.ForbiddenI18n("error.e0222")
	}
	return s.closeConversation(conversationID, enums.IMSenderTypeCustomer, "", nil)
}

func (s *conversationService) closeConversation(conversationID int64, senderType enums.IMSenderType, closeReason string, operator *dto.AuthPrincipal) error {
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		conversation := repositories.ConversationRepository.Get(ctx.Tx, conversationID)
		if conversation == nil {
			return errorsx.InvalidParamI18n("error.e0116")
		}
		if conversation.Status == enums.IMConversationStatusClosed {
			return nil
		}
		if conversation.Status != enums.IMConversationStatusAIServing &&
			conversation.Status != enums.IMConversationStatusPending &&
			conversation.Status != enums.IMConversationStatusActive {
			return errorsx.InvalidParamI18n("error.e0197")
		}
		var (
			now          = time.Now()
			eventDesc    = "会话已关闭"
			operatorID   int64
			operatorName string
		)
		closeReason = strings.TrimSpace(closeReason)
		if senderType == enums.IMSenderTypeCustomer {
			eventDesc = "客户关闭会话"
		} else {
			if operator == nil {
				return errorsx.InvalidParamI18n("error.e0226")
			}
			if closeReason == "" {
				return errorsx.InvalidParamI18n("error.e0128")
			}
			if !s.canCloseConversation(conversation, operator) {
				return errorsx.ForbiddenI18n("error.e0221")
			}
			operatorID = operator.UserID
			operatorName = operator.Nickname
		}
		if err := ConversationAssignmentService.FinishActiveAssignments(ctx, conversationID, now); err != nil {
			return err
		}
		if err := repositories.ConversationRepository.Updates(ctx.Tx, conversationID, map[string]any{
			"status":           enums.IMConversationStatusClosed,
			"closed_at":        now,
			"closed_by":        operatorID,
			"close_reason":     closeReason,
			"update_user_id":   operatorID,
			"update_user_name": operatorName,
			"updated_at":       now,
		}); err != nil {
			return err
		}
		return ConversationEventLogService.CreateEvent(ctx, conversationID, enums.IMEventTypeClose, senderType, operatorID, eventDesc, s.buildEventPayload(map[string]any{
			"fromStatus":     conversation.Status,
			"toStatus":       enums.IMConversationStatusClosed,
			"fromAssigneeId": conversation.CurrentAssigneeID,
			"toAssigneeId":   conversation.CurrentAssigneeID,
			"closeReason":    closeReason,
		}))
	}); err != nil {
		return err
	}
	if conversation := s.Get(conversationID); conversation != nil {
		WsService.PublishConversationChanged(conversation, enums.IMRealtimeEventConversationClosed)
	}
	return nil
}

// MarkAgentConversationReadToMessage 控制台客服将会话已读推进到指定消息。
func (s *conversationService) MarkAgentConversationReadToMessage(conversationID, messageID int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	conversation := s.Get(conversationID)
	if conversation == nil {
		return errorsx.InvalidParamI18n("error.e0116")
	}
	if !s.CanAccessConversation(conversation, operator) {
		return errorsx.ForbiddenI18n("error.e0225")
	}
	changed, err := s.markConversationReadWithActor(conversation, messageID, agentConversationReadActor{operator: operator})
	if err != nil {
		return err
	}
	if changed {
		if updated := s.Get(conversationID); updated != nil {
			WsService.PublishConversationChanged(updated, enums.IMRealtimeEventConversationRead)
		}
	}
	return nil
}

// MarkCustomerConversationReadToMessage IM 客户将会话已读推进到指定消息（需为会话归属外部身份）。
func (s *conversationService) MarkCustomerConversationReadToMessage(conversationID, messageID int64, external *openidentity.ExternalUser) error {
	if external == nil || strings.TrimSpace(external.ExternalID) == "" {
		return errorsx.UnauthorizedI18n("error.e0149")
	}
	conversation := s.Get(conversationID)
	if conversation == nil {
		return errorsx.InvalidParamI18n("error.e0116")
	}
	if !s.IsCustomerConversationOwner(conversation, *external) {
		return errorsx.ForbiddenI18n("error.e0222")
	}
	changed, err := s.markConversationReadWithActor(conversation, messageID, customerConversationReadActor{external: external})
	if err != nil {
		return err
	}
	if changed {
		if updated := s.Get(conversationID); updated != nil {
			WsService.PublishConversationChanged(updated, enums.IMRealtimeEventConversationRead)
		}
	}
	return nil
}

func displayExternalName(ext *openidentity.ExternalUser) string {
	if ext == nil {
		return ""
	}
	if n := strings.TrimSpace(ext.ExternalName); n != "" {
		return n
	}
	return strings.TrimSpace(ext.ExternalID)
}

// conversationReadActor 抽象「读者身份」，供 markConversationReadWithActor 共用（包内私有）。
type conversationReadActor interface {
	isAgentSide() bool
	getReadState(conversationID int64) *models.ConversationReadState
	markRead(ctx *sqls.TxContext, conversation *models.Conversation, targetMessage *models.Message) error
	conversationUpdateAudit() (userID int64, userName string)
}

type agentConversationReadActor struct {
	operator *dto.AuthPrincipal
}

func (a agentConversationReadActor) isAgentSide() bool { return true }

func (a agentConversationReadActor) getReadState(conversationID int64) *models.ConversationReadState {
	return ConversationReadStateService.GetByAgentReader(conversationID, a.operator)
}

func (a agentConversationReadActor) markRead(ctx *sqls.TxContext, conversation *models.Conversation, targetMessage *models.Message) error {
	_, err := ConversationReadStateService.MarkAgentRead(ctx, conversation, a.operator, targetMessage)
	return err
}

func (a agentConversationReadActor) conversationUpdateAudit() (int64, string) {
	if a.operator == nil {
		return 0, ""
	}
	return a.operator.UserID, a.operator.Username
}

type customerConversationReadActor struct {
	external *openidentity.ExternalUser
}

func (a customerConversationReadActor) isAgentSide() bool { return false }

func (a customerConversationReadActor) getReadState(conversationID int64) *models.ConversationReadState {
	return ConversationReadStateService.GetByCustomerReader(conversationID, a.external)
}

func (a customerConversationReadActor) markRead(ctx *sqls.TxContext, conversation *models.Conversation, targetMessage *models.Message) error {
	_, err := ConversationReadStateService.MarkCustomerRead(ctx, conversation, a.external, targetMessage)
	return err
}

func (a customerConversationReadActor) conversationUpdateAudit() (int64, string) {
	return 0, displayExternalName(a.external)
}

func (s *conversationService) markConversationReadWithActor(conversation *models.Conversation, messageID int64, actor conversationReadActor) (bool, error) {
	if conversation == nil {
		return false, errorsx.InvalidParamI18n("error.e0116")
	}
	targetMessage, err := MessageService.GetConversationReadTarget(conversation.ID, messageID)
	if err != nil {
		return false, err
	}
	if targetMessage == nil {
		if actor.isAgentSide() && conversation.AgentUnreadCount == 0 {
			return false, nil
		}
		if !actor.isAgentSide() && conversation.CustomerUnreadCount == 0 {
			return false, nil
		}
		now := time.Now()
		updateUserID, updateUserName := actor.conversationUpdateAudit()
		updates := map[string]any{
			"update_user_id":   updateUserID,
			"update_user_name": updateUserName,
			"updated_at":       now,
		}
		if actor.isAgentSide() {
			updates["agent_unread_count"] = 0
		} else {
			updates["customer_unread_count"] = 0
		}
		return true, s.Updates(conversation.ID, updates)
	}

	currentReadState := actor.getReadState(conversation.ID)
	if currentReadState != nil && currentReadState.LastReadMessageID >= targetMessage.ID {
		if actor.isAgentSide() && conversation.AgentUnreadCount == 0 {
			return false, nil
		}
		if !actor.isAgentSide() && conversation.CustomerUnreadCount == 0 {
			return false, nil
		}
	}

	err = sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		currentConversation := repositories.ConversationRepository.Get(ctx.Tx, conversation.ID)
		if currentConversation == nil {
			return errorsx.InvalidParamI18n("error.e0116")
		}
		if err := actor.markRead(ctx, currentConversation, targetMessage); err != nil {
			return err
		}
		agentReadState, customerReadState := ConversationReadStateService.getConversationReadStates(ctx.Tx, currentConversation.ID)
		agentUnreadCount, err := s.countUnreadByState(ctx, currentConversation.ID, agentReadState, enums.IMSenderTypeCustomer, enums.IMSenderTypePartner)
		if err != nil {
			return err
		}
		customerUnreadCount, err := s.countUnreadByState(ctx, currentConversation.ID, customerReadState, enums.IMSenderTypeAgent, enums.IMSenderTypeAI, enums.IMSenderTypePartner)
		if err != nil {
			return err
		}
		if actor.isAgentSide() && currentConversation.AgentUnreadCount == agentUnreadCount && currentReadState != nil && currentReadState.LastReadMessageID >= targetMessage.ID {
			return nil
		}
		if !actor.isAgentSide() && currentConversation.CustomerUnreadCount == customerUnreadCount && currentReadState != nil && currentReadState.LastReadMessageID >= targetMessage.ID {
			return nil
		}
		updateUserID, updateUserName := actor.conversationUpdateAudit()
		return repositories.ConversationRepository.Updates(ctx.Tx, currentConversation.ID, map[string]any{
			"agent_unread_count":    agentUnreadCount,
			"customer_unread_count": customerUnreadCount,
			"update_user_id":        updateUserID,
			"update_user_name":      updateUserName,
			"updated_at":            time.Now(),
		})
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *conversationService) countUnreadByState(ctx *sqls.TxContext, conversationID int64, state *models.ConversationReadState, senderTypes ...enums.IMSenderType) (int, error) {
	lastReadMessageID := int64(0)
	if state != nil {
		lastReadMessageID = state.LastReadMessageID
	}
	normalizedSenderTypes := make([]enums.IMSenderType, 0, len(senderTypes))
	for _, senderType := range senderTypes {
		normalizedSenderTypes = append(normalizedSenderTypes, senderType)
	}
	count, err := ConversationReadStateService.CountUnreadMessages(ctx, conversationID, lastReadMessageID, normalizedSenderTypes...)
	return int(count), err
}

func (s *conversationService) IsCustomerConversationOwner(conversation *models.Conversation, externalUser openidentity.ExternalUser) bool {
	if conversation == nil {
		return false
	}
	extID := strings.TrimSpace(externalUser.ExternalID)
	if extID == "" || strings.TrimSpace(string(externalUser.ExternalSource)) == "" || conversation.CustomerID <= 0 {
		return false
	}
	identity := repositories.CustomerIdentityRepository.GetBy(sqls.DB(), externalUser.ExternalSource, extID)
	if identity == nil {
		return false
	}
	if identity.CustomerID != conversation.CustomerID {
		return false
	}
	matchesParticipant, participantCount := s.customerParticipantAccess(sqls.DB(), conversation.ID, externalUser)
	return matchesParticipant || participantCount == 0
}

func (s *conversationService) customerParticipantAccess(db *gorm.DB, conversationID int64, externalUser openidentity.ExternalUser) (bool, int64) {
	participant := repositories.ConversationParticipantRepository.FindOne(db, sqls.NewCnd().
		Eq("conversation_id", conversationID).
		Eq("participant_type", enums.IMParticipantTypeCustomer).
		Eq("status", enums.StatusOk).
		In("external_participant_id", CustomerParticipantExternalIDs(externalUser)))
	if participant != nil {
		return true, 1
	}
	participantCount := repositories.ConversationParticipantRepository.Count(db, sqls.NewCnd().
		Eq("conversation_id", conversationID).
		Eq("participant_type", enums.IMParticipantTypeCustomer).
		Eq("status", enums.StatusOk))
	return false, participantCount
}

func (s *conversationService) BuildConversationSummary(conversation *models.Conversation) string {
	return s.BuildConversationSummaryDB(sqls.DB(), conversation)
}

func (s *conversationService) BuildConversationSummaryDB(db *gorm.DB, conversation *models.Conversation) string {
	if conversation == nil {
		return ""
	}
	if db == nil {
		db = sqls.DB()
	}
	if message := repositories.MessageRepository.FindLastValidByConversationIDAndSenderType(db, conversation.ID, enums.IMSenderTypeCustomer); message != nil {
		if summary := strings.TrimSpace(buildMessageSummary(message.MessageType, message.Content)); summary != "" {
			return limitText(summary, 255)
		}
	}
	if strings.TrimSpace(conversation.LastMessageSummary) != "" {
		return conversation.LastMessageSummary
	}
	return strings.TrimSpace(conversation.CustomerName)
}

func (s *conversationService) getCustomerName(db *gorm.DB, customerID int64) string {
	if customerID <= 0 {
		return ""
	}
	if customer := repositories.CustomerRepository.Get(db, customerID); customer != nil {
		return strings.TrimSpace(customer.Name)
	}
	return ""
}

func (s *conversationService) canCloseConversation(conversation *models.Conversation, operator *dto.AuthPrincipal) bool {
	if conversation == nil || operator == nil {
		return false
	}
	if !s.CanAccessConversation(conversation, operator) {
		return false
	}
	if s.isAdmin(operator) {
		return true
	}
	return conversation.Status == enums.IMConversationStatusActive && conversation.CurrentAssigneeID > 0 && conversation.CurrentAssigneeID == operator.UserID
}

func (s *conversationService) canTransferConversation(conversation *models.Conversation, operator *dto.AuthPrincipal) bool {
	if conversation == nil || operator == nil {
		return false
	}
	if !s.CanAccessConversation(conversation, operator) {
		return false
	}
	if s.isAdmin(operator) {
		return true
	}
	return conversation.Status == enums.IMConversationStatusActive &&
		conversation.CurrentAssigneeID > 0 &&
		conversation.CurrentAssigneeID == operator.UserID
}

func (s *conversationService) CanTransferConversation(conversation *models.Conversation, operator *dto.AuthPrincipal) bool {
	return s.canTransferConversation(conversation, operator)
}

func (s *conversationService) isAdmin(operator *dto.AuthPrincipal) bool {
	if operator == nil {
		return false
	}
	return canManageTicketDispatch(operator) ||
		operator.HasRole(constants.RoleCodeSuperAdmin) ||
		operator.HasRole(constants.RoleCodeAdmin)
}

// CanAccessConversation enforces tenant, product-team and temporary supplier scope.
func (s *conversationService) CanAccessConversation(conversation *models.Conversation, operator *dto.AuthPrincipal) bool {
	if conversation == nil || operator == nil {
		return false
	}
	tenantID := operator.TenantID
	if tenantID <= 0 {
		tenantID = operator.TargetTenantID
	}
	if tenantID > 0 && conversation.TenantID != tenantID {
		return false
	}
	if tenantID <= 0 {
		if operator.IsPlatform() && s.isAdmin(operator) {
			return true
		}
		if conversation.TenantID != 0 {
			return false
		}
	}
	if operator.IsPartner() {
		return s.partnerCanAccessConversation(conversation, operator)
	}
	scope := resolveEnterpriseProductAccessScope(conversation.TenantID, operator)
	if !scope.Restricted {
		return true
	}
	if conversation.CurrentAssigneeID == scope.UserID || slices.Contains(scope.TeamIDs, conversation.CurrentTeamID) {
		return true
	}
	if conversation.ProductID > 0 && slices.Contains(scope.ProductIDs, conversation.ProductID) {
		return true
	}
	if ticket := repositories.TicketRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", conversation.TenantID).
		Eq("conversation_id", conversation.ID).
		Where("status NOT IN ?", []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled}).
		Desc("id")); ticket != nil {
		return scope.canAccessTicket(ticket)
	}
	return false
}

func (s *conversationService) partnerCanAccessConversation(conversation *models.Conversation, operator *dto.AuthPrincipal) bool {
	if conversation == nil || operator == nil || operator.PartnerAccountID <= 0 || operator.UserID <= 0 {
		return false
	}
	participant := repositories.ConversationParticipantRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("conversation_id", conversation.ID).
		Eq("participant_type", enums.IMParticipantTypePartner).
		Eq("participant_id", operator.UserID).
		Eq("status", enums.StatusOk))
	if participant == nil {
		return false
	}
	now := time.Now()
	for _, collaboration := range repositories.TicketSupplierCollaborationRepository.FindByParticipantAccount(sqls.DB(), conversation.TenantID, operator.PartnerAccountID) {
		if collaboration.Status == SupplierCollaborationResolved || collaboration.RecordStatus == enums.StatusDeleted ||
			collaboration.AuthorizationEnds != nil && !collaboration.AuthorizationEnds.After(now) {
			continue
		}
		if repositories.TicketSupplierCollaborationRepository.FindActiveParticipant(sqls.DB(), conversation.TenantID, collaboration.ID, operator.PartnerAccountID) == nil {
			continue
		}
		ticket := repositories.TicketRepository.Get(sqls.DB(), collaboration.TicketID)
		if ticket != nil && ticket.TenantID == conversation.TenantID && ticket.ConversationID == conversation.ID {
			return true
		}
	}
	return false
}

func (s *conversationService) ensureAgentParticipantTx(db *gorm.DB, conversationID, userID int64, joinedAt time.Time, operator *dto.AuthPrincipal) error {
	if db == nil || conversationID <= 0 || userID <= 0 || operator == nil {
		return nil
	}
	existing := repositories.ConversationParticipantRepository.FindOne(db, sqls.NewCnd().
		Eq("conversation_id", conversationID).
		Eq("participant_type", enums.IMParticipantTypeAgent).
		Eq("participant_id", userID))
	if existing != nil {
		return repositories.ConversationParticipantRepository.Updates(db, existing.ID, map[string]any{
			"external_participant_id": "agent-user:" + strconv.FormatInt(userID, 10),
			"joined_at":               joinedAt,
			"left_at":                 nil,
			"status":                  enums.StatusOk,
			"updated_at":              joinedAt,
			"update_user_id":          operator.UserID,
			"update_user_name":        operator.Username,
		})
	}
	return repositories.ConversationParticipantRepository.Create(db, &models.ConversationParticipant{
		ConversationID:        conversationID,
		ParticipantType:       string(enums.IMParticipantTypeAgent),
		ParticipantID:         userID,
		ExternalParticipantID: "agent-user:" + strconv.FormatInt(userID, 10),
		JoinedAt:              &joinedAt,
		Status:                enums.StatusOk,
		AuditFields:           utils.BuildAuditFields(operator),
	})
}

func (s *conversationService) buildEventPayload(payload map[string]any) string {
	if len(payload) == 0 {
		return ""
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

// LinkConversationCustomer 将会话绑定到指定客户。
func (s *conversationService) LinkConversationCustomer(conversationID, customerID int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if conversationID <= 0 || customerID <= 0 {
		return errorsx.InvalidParamI18n("error.e0133")
	}
	cust := CustomerService.Get(customerID)
	if cust == nil || cust.Status == enums.StatusDeleted {
		return errorsx.InvalidParamI18n("error.e0155")
	}
	conv := s.Get(conversationID)
	if conv == nil {
		return errorsx.InvalidParamI18n("error.e0116")
	}
	if conv.Status == enums.IMConversationStatusClosed {
		return errorsx.InvalidParamI18n("error.e0183")
	}
	if !s.canLinkConversationCustomer(conv, operator) {
		return errorsx.ForbiddenI18n("error.e0224")
	}

	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		current := repositories.ConversationRepository.Get(ctx.Tx, conversationID)
		if current == nil {
			return errorsx.InvalidParamI18n("error.e0116")
		}
		now := time.Now()
		return repositories.ConversationRepository.Updates(ctx.Tx, conversationID, map[string]any{
			"customer_id":      customerID,
			"customer_name":    strings.TrimSpace(cust.Name),
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       now,
		})
	})
	if err != nil {
		return err
	}
	if updated := s.Get(conversationID); updated != nil {
		WsService.PublishConversationChanged(updated, enums.IMRealtimeEventConversationUpdated)
	}
	return nil
}

func (s *conversationService) GetConversationExternalIdentity(conversation *models.Conversation) *models.CustomerIdentity {
	if conversation == nil || conversation.CustomerID <= 0 {
		return nil
	}
	identities := repositories.CustomerIdentityRepository.FindByCustomerID(sqls.DB(), conversation.CustomerID)
	if len(identities) == 0 {
		return nil
	}
	if channel := ChannelService.Get(conversation.ChannelID); channel != nil {
		expected := externalSourceForChannelType(channel.ChannelType)
		if strings.TrimSpace(string(expected)) != "" {
			for i := range identities {
				if identities[i].ExternalSource == expected {
					return &identities[i]
				}
			}
		}
	}
	return &identities[0]
}

func (s *conversationService) GetConversationCustomerLastSeenAt(conversation *models.Conversation) *time.Time {
	if conversation == nil {
		return nil
	}
	db := sqls.DB()
	if db == nil {
		return nil
	}
	if conversation.CustomerEntrySessionID > 0 && db.Migrator().HasTable(&models.CustomerEntrySession{}) {
		if session := repositories.CustomerEntrySessionRepository.Get(db, conversation.CustomerEntrySessionID); session != nil && session.CustomerUserID > 0 {
			if customerUser := repositories.EnterpriseIAMRepository.GetCustomerUser(db, session.TenantID, session.CustomerUserID); customerUser != nil && customerUser.Status == enums.StatusOk {
				return customerUser.LastSeenAt
			}
		}
	}
	identities, err := repositories.CustomerIdentityRepository.FindActiveUserIdentitiesByCustomerIDs(db, []int64{conversation.CustomerID})
	if err != nil || len(identities) == 0 {
		return nil
	}
	accountUserID, err := strconv.ParseInt(strings.TrimSpace(identities[0].ExternalID), 10, 64)
	if err != nil || accountUserID <= 0 {
		return nil
	}
	var customerUser *models.CustomerUser
	if conversation.TenantID > 0 {
		customerUser = repositories.CustomerPortalRepository.FindCustomerUserByTenantAndUserID(db, conversation.TenantID, accountUserID)
	}
	if customerUser == nil {
		customerUser = repositories.CustomerPortalRepository.FindCustomerUserByUserID(db, accountUserID)
	}
	if customerUser == nil || customerUser.Status != enums.StatusOk {
		return nil
	}
	return customerUser.LastSeenAt
}

func (s *conversationService) BuildConversationCustomerLastSeenAtMap(conversations []models.Conversation) map[int64]*time.Time {
	ret := make(map[int64]*time.Time, len(conversations))
	if len(conversations) == 0 {
		return ret
	}
	db := sqls.DB()
	if db == nil {
		return ret
	}

	entrySessionIDs := make([]int64, 0, len(conversations))
	customerIDs := make([]int64, 0, len(conversations))
	for i := range conversations {
		conversation := conversations[i]
		if conversation.CustomerEntrySessionID > 0 {
			entrySessionIDs = append(entrySessionIDs, conversation.CustomerEntrySessionID)
		}
		if conversation.CustomerID > 0 {
			customerIDs = append(customerIDs, conversation.CustomerID)
		}
	}

	sessionByID := make(map[int64]models.CustomerEntrySession)
	customerUserIDs := make([]int64, 0)
	if sessions, err := repositories.CustomerEntrySessionRepository.FindByIDs(db, entrySessionIDs); err == nil {
		for i := range sessions {
			session := sessions[i]
			if session.CustomerUserID <= 0 {
				continue
			}
			sessionByID[session.ID] = session
			customerUserIDs = append(customerUserIDs, session.CustomerUserID)
		}
	}
	if len(sessionByID) > 0 {
		customerUserLastSeenByID := make(map[int64]*time.Time, len(customerUserIDs))
		if users, err := repositories.CustomerPortalRepository.FindCustomerUsersByIDs(db, customerUserIDs); err == nil {
			for i := range users {
				user := users[i]
				customerUserLastSeenByID[user.ID] = user.LastSeenAt
			}
		}
		for i := range conversations {
			conversation := conversations[i]
			session, ok := sessionByID[conversation.CustomerEntrySessionID]
			if !ok || conversation.ID <= 0 {
				continue
			}
			if conversation.TenantID > 0 && session.TenantID > 0 && conversation.TenantID != session.TenantID {
				continue
			}
			if lastSeenAt, ok := customerUserLastSeenByID[session.CustomerUserID]; ok {
				ret[conversation.ID] = lastSeenAt
			}
		}
	}

	customerAccountUserIDByCustomerID := make(map[int64]int64)
	if identities, err := repositories.CustomerIdentityRepository.FindActiveUserIdentitiesByCustomerIDs(db, customerIDs); err == nil {
		for i := range identities {
			identity := identities[i]
			if _, exists := customerAccountUserIDByCustomerID[identity.CustomerID]; exists {
				continue
			}
			accountUserID, parseErr := strconv.ParseInt(strings.TrimSpace(identity.ExternalID), 10, 64)
			if parseErr != nil || accountUserID <= 0 {
				continue
			}
			customerAccountUserIDByCustomerID[identity.CustomerID] = accountUserID
		}
	}
	if len(customerAccountUserIDByCustomerID) == 0 {
		return ret
	}

	tenantIDs := make([]int64, 0, len(conversations))
	accountUserIDs := make([]int64, 0, len(customerAccountUserIDByCustomerID))
	for i := range conversations {
		conversation := conversations[i]
		if conversation.ID <= 0 {
			continue
		}
		if _, exists := ret[conversation.ID]; exists || conversation.TenantID <= 0 {
			continue
		}
		accountUserID := customerAccountUserIDByCustomerID[conversation.CustomerID]
		if accountUserID <= 0 {
			continue
		}
		tenantIDs = append(tenantIDs, conversation.TenantID)
		accountUserIDs = append(accountUserIDs, accountUserID)
	}
	customerUserLastSeenByTenantAndUserID := make(map[int64]map[int64]*time.Time)
	if users, err := repositories.CustomerPortalRepository.FindCustomerUsersByTenantIDsAndUserIDs(db, tenantIDs, accountUserIDs); err == nil {
		for i := range users {
			user := users[i]
			if customerUserLastSeenByTenantAndUserID[user.TenantID] == nil {
				customerUserLastSeenByTenantAndUserID[user.TenantID] = make(map[int64]*time.Time)
			}
			if _, exists := customerUserLastSeenByTenantAndUserID[user.TenantID][user.UserID]; exists {
				continue
			}
			customerUserLastSeenByTenantAndUserID[user.TenantID][user.UserID] = user.LastSeenAt
		}
	}
	for i := range conversations {
		conversation := conversations[i]
		if conversation.ID <= 0 {
			continue
		}
		if _, exists := ret[conversation.ID]; exists {
			continue
		}
		accountUserID := customerAccountUserIDByCustomerID[conversation.CustomerID]
		lastSeenByAccountUserID := customerUserLastSeenByTenantAndUserID[conversation.TenantID]
		if accountUserID <= 0 || lastSeenByAccountUserID == nil {
			continue
		}
		if lastSeenAt, ok := lastSeenByAccountUserID[accountUserID]; ok {
			ret[conversation.ID] = lastSeenAt
		}
	}
	return ret
}

func (s *conversationService) IsCustomerLastSeenOnline(lastSeenAt *time.Time, now time.Time) bool {
	if lastSeenAt == nil || lastSeenAt.IsZero() || now.IsZero() || now.Before(*lastSeenAt) {
		return false
	}
	return now.Sub(*lastSeenAt) <= customerPresenceFreshness
}

func externalSourceForChannelType(channelType string) enums.ExternalSource {
	switch strings.TrimSpace(channelType) {
	case enums.ChannelTypeWxWorkKF:
		return enums.ExternalSourceWxWorkKF
	case enums.ChannelTypeWeb:
		return enums.ExternalSourceGuest
	default:
		return ""
	}
}

func (s *conversationService) canLinkConversationCustomer(conv *models.Conversation, operator *dto.AuthPrincipal) bool {
	if conv == nil || operator == nil {
		return false
	}
	if !s.CanAccessConversation(conv, operator) {
		return false
	}
	if s.isAdmin(operator) {
		return true
	}
	switch conv.Status {
	case enums.IMConversationStatusAIServing:
		return true
	case enums.IMConversationStatusPending:
		return true
	case enums.IMConversationStatusActive:
		return conv.CurrentAssigneeID == 0 || conv.CurrentAssigneeID == operator.UserID
	default:
		return false
	}
}

func (s *conversationService) resolveInitialStatus(serviceMode enums.IMConversationServiceMode) enums.IMConversationStatus {
	switch serviceMode {
	case enums.IMConversationServiceModeHumanOnly:
		return enums.IMConversationStatusPending
	case enums.IMConversationServiceModeAIOnly, enums.IMConversationServiceModeAIFirst:
		return enums.IMConversationStatusAIServing
	default:
		return enums.IMConversationStatusAIServing
	}
}

package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var TicketService = newTicketService()

var conversationTicketFaultCodePattern = regexp.MustCompile(`(?i)\b(?:[A-Z][A-Z0-9]{0,15}(?:-[A-Z0-9]{1,16})+|[A-Z]{1,8}[0-9]{2,8})\b`)
var conversationTicketLabeledFaultCodePattern = regexp.MustCompile(`(?i)(?:故障码|错误码|报警码|告警码|报错|报码|报)\s*[:：#]?\s*([A-Z][A-Z0-9]{0,15}(?:-[A-Z0-9]{1,16})+|[A-Z]{1,8}[0-9]{2,8})\b`)

func newTicketService() *ticketService {
	return &ticketService{}
}

func CanCreateTicketWithAssignee(operator *dto.AuthPrincipal, assigneeID int64) bool {
	return assigneeID <= 0 || operator != nil && (assigneeID == operator.UserID || operator.HasPermission(constants.PermissionTicketAssign.Code))
}

type TicketDetailAggregate struct {
	Ticket        *models.Ticket
	Tags          []models.Tag
	Customer      *models.Customer
	Progresses    []models.TicketProgress
	RepairRecords []models.TicketRepairRecord
	Users         map[int64]*models.User
}

type TicketSummaryAggregate struct {
	All        int64
	Pending    int64
	InProgress int64
	Done       int64
	Unassigned int64
	Mine       int64
	Stale      int64
}

type TicketListAggregate struct {
	List           []models.Ticket
	Paging         *sqls.Paging
	TagsByTicketID map[int64][]models.Tag
	Users          map[int64]*models.User
	Customers      map[int64]*models.Customer
}

type ticketService struct {
}

type preparedTicketCreate struct {
	ticket              *models.Ticket
	tagIDs              []int64
	tenantID            int64
	idempotencyKey      *string
	idempotencyKeyValue string
}

func normalizeTicketStaleHours(staleHours int) int {
	switch staleHours {
	case 24, 48, 168:
		return staleHours
	default:
		return 24
	}
}

func (s *ticketService) Get(id int64) *models.Ticket {
	return repositories.TicketRepository.Get(sqls.DB(), id)
}

func (s *ticketService) Take(where ...any) *models.Ticket {
	return repositories.TicketRepository.Take(sqls.DB(), where...)
}

func (s *ticketService) Find(cnd *sqls.Cnd) []models.Ticket {
	return repositories.TicketRepository.Find(sqls.DB(), cnd)
}

func (s *ticketService) FindOne(cnd *sqls.Cnd) *models.Ticket {
	return repositories.TicketRepository.FindOne(sqls.DB(), cnd)
}

func (s *ticketService) FindPageByParams(params *params.QueryParams) (list []models.Ticket, paging *sqls.Paging) {
	return repositories.TicketRepository.FindPageByParams(sqls.DB(), params)
}

func (s *ticketService) FindPageByCnd(cnd *sqls.Cnd) (list []models.Ticket, paging *sqls.Paging) {
	return repositories.TicketRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *ticketService) FindPageAggregateByCnd(cnd *sqls.Cnd, _ int64) (*TicketListAggregate, error) {
	list, paging := repositories.TicketRepository.FindPageByCnd(sqls.DB(), cnd)
	return s.buildTicketListAggregate(sqls.DB(), list, paging), nil
}

func (s *ticketService) FindRepairHistoryPage(filter request.RepairHistoryFilter) ([]models.Ticket, *sqls.Paging) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	cnd := sqls.NewCnd().Desc("id").Page(1, limit)
	if filter.DeviceID > 0 {
		cnd.Eq("device_id", filter.DeviceID)
	}
	if filter.ProductID > 0 {
		cnd.Eq("product_id", filter.ProductID)
	}
	if filter.CustomerID > 0 {
		cnd.Eq("customer_id", filter.CustomerID)
	}
	return repositories.TicketRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *ticketService) ApplyStaleFilter(cnd *sqls.Cnd, staleHours int) *sqls.Cnd {
	if cnd == nil {
		cnd = sqls.NewCnd()
	}
	staleHour := normalizeTicketStaleHours(staleHours)
	return cnd.
		NotEq("status", enums.TicketStatusClosed).
		NotEq("status", enums.TicketStatusDone).
		Where("updated_at < ?", time.Now().Add(-time.Duration(staleHour)*time.Hour))
}

func (s *ticketService) Count(cnd *sqls.Cnd) int64 {
	return repositories.TicketRepository.Count(sqls.DB(), cnd)
}

func (s *ticketService) Create(t *models.Ticket) error {
	return repositories.TicketRepository.Create(sqls.DB(), t)
}

func (s *ticketService) Update(t *models.Ticket) error {
	return repositories.TicketRepository.Update(sqls.DB(), t)
}

func (s *ticketService) Updates(id int64, columns map[string]any) error {
	return repositories.TicketRepository.Updates(sqls.DB(), id, columns)
}

func (s *ticketService) UpdateColumn(id int64, name string, value any) error {
	return repositories.TicketRepository.UpdateColumn(sqls.DB(), id, name, value)
}

func (s *ticketService) Delete(id int64) {
	repositories.TicketRepository.Delete(sqls.DB(), id)
}

func (s *ticketService) GetTags(ticketID int64) []models.Tag {
	if ticketID <= 0 {
		return nil
	}
	relations := TicketTagService.Find(sqls.NewCnd().Eq("ticket_id", ticketID).Asc("id"))
	if len(relations) == 0 {
		return nil
	}
	tagIDs := make([]int64, 0, len(relations))
	for i := range relations {
		tagIDs = append(tagIDs, relations[i].TagID)
	}
	tags := repositories.TagRepository.Find(sqls.DB(), sqls.NewCnd().In("id", tagIDs))
	if len(tags) <= 1 {
		return tags
	}
	tagMap := make(map[int64]models.Tag, len(tags))
	for i := range tags {
		tagMap[tags[i].ID] = tags[i]
	}
	ordered := make([]models.Tag, 0, len(relations))
	for _, tagID := range tagIDs {
		if tag, ok := tagMap[tagID]; ok {
			ordered = append(ordered, tag)
		}
	}
	return ordered
}

func (s *ticketService) CreateTicket(req request.CreateTicketRequest, operator *dto.AuthPrincipal) (*models.Ticket, error) {
	prepared, existing, err := s.prepareTicketCreate(sqls.DB(), req, operator)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	ticketNo, err := TicketNoSequenceService.Next(prepared.ticket.CreatedAt)
	if err != nil {
		return nil, err
	}
	prepared.ticket.TicketNo = ticketNo

	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		return s.createTicketPreparedTx(ctx, prepared, operator)
	}); err != nil {
		// A concurrent retry may have committed the same logical ticket first.
		if prepared.idempotencyKey != nil {
			if existing := repositories.TicketRepository.FindOne(sqls.DB(), sqls.NewCnd().
				Eq("tenant_id", prepared.tenantID).
				Eq("idempotency_key", prepared.idempotencyKeyValue)); existing != nil {
				return existing, nil
			}
		}
		return nil, err
	}

	return s.Get(prepared.ticket.ID), nil
}

func (s *ticketService) prepareTicketCreate(db *gorm.DB, req request.CreateTicketRequest, operator *dto.AuthPrincipal) (*preparedTicketCreate, *models.Ticket, error) {
	if operator == nil {
		return nil, nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if db == nil {
		db = sqls.DB()
	}
	title := strings.TrimSpace(req.Title)
	description := strings.TrimSpace(req.Description)
	if title == "" {
		return nil, nil, errorsx.InvalidParamI18n("error.e0181")
	}
	if description == "" {
		return nil, nil, errorsx.InvalidParamI18n("error.e0179")
	}
	source := enums.TicketSource(strings.TrimSpace(req.Source))
	if source == "" {
		source = enums.TicketSourceManual
	}
	if !enums.IsValidTicketSource(string(source)) {
		return nil, nil, errorsx.InvalidParamI18n("error.e0180")
	}
	// tenant_id 优先取 AuthPrincipal；operator 未携带租户的内部/测试场景回退到请求体
	tenantID := operator.TenantID
	if tenantID <= 0 {
		tenantID = req.TenantID
	}
	tenant := repositories.PlatformIAMRepository.GetTenant(db, tenantID)
	if tenant != nil && tenant.IsKnowledgeSupportScene() {
		req.ProductID = 0
		req.ProductModelID = 0
		req.ProductModuleID = 0
		req.DeviceID = 0
		req.ServiceCodeID = 0
		req.CustomerEntrySessionID = 0
	}
	var registrationGrant *models.CustomerRegistrationGrant
	if req.CustomerRegistrationGrantID > 0 {
		if req.CustomerID > 0 || req.ConversationID > 0 {
			return nil, nil, errorsx.InvalidParam("invited-customer draft cannot already have a customer or conversation")
		}
		registrationGrant = repositories.CustomerRegistrationRepository.GetForTenant(db, tenantID, req.CustomerRegistrationGrantID)
		if registrationGrant == nil || registrationGrant.DomainType != models.DomainTypeCustomer ||
			registrationGrant.Status != models.CustomerRegistrationGrantPending || !registrationGrant.ExpiresAt.After(time.Now()) {
			return nil, nil, errorsx.InvalidParam("customer invitation is not available")
		}
	}
	idempotencyKeyValue := strings.TrimSpace(req.IdempotencyKey)
	var idempotencyKey *string
	if idempotencyKeyValue != "" {
		idempotencyKey = &idempotencyKeyValue
		if existing := repositories.TicketRepository.FindOne(db, sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("idempotency_key", idempotencyKeyValue)); existing != nil {
			return nil, existing, nil
		}
	}
	if err := s.validateTicketRefsDB(db, req.CustomerID, req.ConversationID, 0, tenantID); err != nil {
		return nil, nil, err
	}
	if err := s.validateAfterSalesContextDB(db, tenantID, req.ProductID, req.ProductModelID, req.ProductModuleID, req.DeviceID, req.ServiceCodeID, req.CustomerEntrySessionID); err != nil {
		return nil, nil, err
	}
	tagIDs, err := TicketTagService.ValidateTagIDs(req.TagIDs)
	if err != nil {
		return nil, nil, err
	}
	priorityCode, err := normalizeTicketPriorityCode(req.PriorityCode, "p2")
	if err != nil {
		return nil, nil, err
	}

	currentTeamID := req.CurrentTeamID
	if currentTeamID <= 0 && req.ProductID > 0 {
		if team := ProductSupportOrganizationService.FindProductRepairTeam(db, tenantID, req.ProductID); team != nil {
			currentTeamID = team.ID
		}
	}
	if currentTeamID <= 0 && tenant != nil && tenant.IsKnowledgeSupportScene() {
		if team := repositories.AgentTeamRepository.FindOne(db, sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("team_type", AgentTeamTypeTechnicalRepair).
			Eq("status", enums.StatusOk).
			Asc("id")); team != nil {
			currentTeamID = team.ID
		}
	}
	if req.CurrentAssigneeID > 0 {
		if tenantID > 0 {
			assignmentTicket := &models.Ticket{
				TenantID:      tenantID,
				ProductID:     req.ProductID,
				CurrentTeamID: currentTeamID,
			}
			_, _, validatedTeamID, err := validateManualTicketAssigneeDB(db, assignmentTicket, req.CurrentAssigneeID, time.Now())
			if err != nil {
				return nil, nil, err
			}
			currentTeamID = validatedTeamID
		} else if err := s.validateRequiredAssignee(req.CurrentAssigneeID); err != nil {
			return nil, nil, err
		}
	}
	initialStatus := enums.TicketStatusPending
	if registrationGrant != nil {
		initialStatus = enums.TicketStatusDraft
	} else if tenantID > 0 && req.CurrentAssigneeID > 0 {
		initialStatus = enums.TicketStatusPendingAssigneeAccept
	}
	ticket := &models.Ticket{
		IdempotencyKey:              idempotencyKey,
		Title:                       title,
		Description:                 description,
		Source:                      source,
		Channel:                     strings.TrimSpace(req.Channel),
		CustomerID:                  req.CustomerID,
		CustomerRegistrationGrantID: req.CustomerRegistrationGrantID,
		ConversationID:              req.ConversationID,
		Status:                      initialStatus,
		PriorityCode:                priorityCode,
		CurrentTeamID:               currentTeamID,
		CurrentAssigneeID:           req.CurrentAssigneeID,
		TenantID:                    tenantID,
		ProductID:                   req.ProductID,
		ProductModelID:              req.ProductModelID,
		ProductModuleID:             req.ProductModuleID,
		DeviceID:                    req.DeviceID,
		ServiceCodeID:               req.ServiceCodeID,
		CustomerEntrySessionID:      req.CustomerEntrySessionID,
		ServiceRegion:               strings.TrimSpace(req.ServiceRegion),
		FaultCode:                   strings.TrimSpace(req.FaultCode),
		SymptomSummary:              strings.TrimSpace(req.SymptomSummary),
		DiagnosisSummary:            strings.TrimSpace(req.DiagnosisSummary),
		SLADueAt:                    req.SLADueAt,
		ResolvedAt:                  req.ResolvedAt,
		AuditFields:                 utils.BuildAuditFields(operator),
	}
	if err := prepareTicketIntakeDB(db, ticket, req.TicketIntakeInput); err != nil {
		return nil, nil, err
	}
	if ticket.CurrentAssigneeID > 0 && ticket.Status != enums.TicketStatusDraft {
		assignedAt := ticket.CreatedAt
		if assignedAt.IsZero() {
			assignedAt = time.Now()
		}
		deadline := ticketAssignmentDeadline(ticket, assignedAt)
		ticket.AssignedAt = &assignedAt
		ticket.AcceptDeadlineAt = &deadline
	}
	return &preparedTicketCreate{
		ticket:              ticket,
		tagIDs:              tagIDs,
		tenantID:            tenantID,
		idempotencyKey:      idempotencyKey,
		idempotencyKeyValue: idempotencyKeyValue,
	}, nil, nil
}

func (s *ticketService) createTicketPreparedTx(ctx *sqls.TxContext, prepared *preparedTicketCreate, operator *dto.AuthPrincipal) error {
	if ctx == nil || ctx.Tx == nil || prepared == nil || prepared.ticket == nil {
		return errorsx.InvalidParam("ticket transaction is not available")
	}
	ticket := prepared.ticket
	if strings.TrimSpace(ticket.TicketNo) == "" {
		ticketNo, err := TicketNoSequenceService.NextTx(ctx.Tx, ticket.CreatedAt)
		if err != nil {
			return err
		}
		ticket.TicketNo = ticketNo
	}
	if err := repositories.TicketRepository.Create(ctx.Tx, ticket); err != nil {
		return err
	}
	if err := TicketTagService.ReplaceTicketTags(ctx.Tx, ticket.ID, prepared.tagIDs, operator); err != nil {
		return err
	}
	// 写入上下文快照
	if err := s.createContextSnapshot(ctx.Tx, ticket); err != nil {
		return err
	}
	createdContent := "Created ticket"
	if ticket.Status == enums.TicketStatusDraft && ticket.CustomerRegistrationGrantID > 0 {
		createdContent = "Draft ticket created; awaiting customer invitation acceptance"
	}
	createdProgress := &models.TicketProgress{
		TenantID:     ticket.TenantID,
		TicketID:     ticket.ID,
		EventType:    enums.TicketProgressEventCreated,
		Content:      createdContent,
		MetadataJSON: "{}",
		AuthorID:     operator.UserID,
		CreatedAt:    time.Now(),
	}
	if ticket.SourceRecordID != "" {
		metadata, err := json.Marshal(BuildTicketIntakeDTO(ticket))
		if err != nil {
			return err
		}
		createdProgress.MetadataJSON = string(metadata)
	}
	if err := repositories.TicketProgressRepository.Create(ctx.Tx, createdProgress); err != nil {
		return err
	}
	if ticket.Status == enums.TicketStatusDraft {
		return nil
	}
	if ticket.CurrentAssigneeID > 0 {
		assignedAt := zeroTime(ticket.AssignedAt)
		if err := createTicketDispatchAttemptTx(ctx.Tx, ticket, ticket.CurrentTeamID, ticket.CurrentAssigneeID, "创建工单时指定负责人", operator, assignedAt); err != nil {
			return err
		}
		if _, err := enqueueTicketAssignedEventTx(ctx.Tx, ticket, 0, ticket.CurrentAssigneeID, "创建工单时指定负责人", operator, createdProgress.ID, assignedAt); err != nil {
			return err
		}
		if ctx.RegisterCallback != nil {
			ctx.RegisterCallback(eventbus.WakeDefaultOutboxPublisher)
		}
	}
	return enqueueTicketCreatedEventTx(ctx, ticket, operator.UserID, "ticket_service", "user")
}

func enqueueTicketCreatedEventTx(ctx *sqls.TxContext, ticket *models.Ticket, operatorID int64, source, actorType string) error {
	if ctx == nil || ctx.Tx == nil || ticket == nil || ticket.ID <= 0 {
		return errorsx.InvalidParam("ticket event transaction is not available")
	}
	eventID := "tenant:" + strconv.FormatInt(ticket.TenantID, 10) + ":ticket.created:" + strconv.FormatInt(ticket.ID, 10)
	created, err := eventbus.EnqueueTx(ctx.Tx, eventbus.DurableEvent{
		TenantID:       ticket.TenantID,
		IdempotencyKey: eventID,
		EventType:      events.EventTicketCreated,
		Payload: events.TicketCreatedEvent{
			EventID:    eventID,
			TicketID:   ticket.ID,
			OperatorID: operatorID,
		},
		Source:      strings.TrimSpace(source),
		AggregateID: strconv.FormatInt(ticket.ID, 10),
		ActorID:     strconv.FormatInt(operatorID, 10),
		ActorType:   strings.TrimSpace(actorType),
	})
	if err == nil && created && ctx.RegisterCallback != nil {
		ctx.RegisterCallback(eventbus.WakeDefaultOutboxPublisher)
	}
	return err
}

func (s *ticketService) ActivateCustomerInvitationDrafts(
	ctx *sqls.TxContext,
	grant *models.CustomerRegistrationGrant,
	customerID int64,
) error {
	if ctx == nil || ctx.Tx == nil || grant == nil || grant.ID <= 0 || grant.TenantID <= 0 || customerID <= 0 {
		return errorsx.InvalidParam("customer invitation activation context is invalid")
	}
	tickets, err := repositories.TicketRepository.FindDraftsByCustomerRegistrationGrantID(ctx.Tx, grant.TenantID, grant.ID)
	if err != nil {
		return err
	}
	now := time.Now()
	for i := range tickets {
		ticket := &tickets[i]
		nextStatus := enums.TicketStatusPendingAcceptance
		updates := map[string]any{
			"customer_id":      customerID,
			"status":           nextStatus,
			"update_user_id":   int64(0),
			"update_user_name": "customer-invitation",
			"updated_at":       now,
		}
		if ticket.CurrentAssigneeID > 0 {
			nextStatus = enums.TicketStatusPendingAssigneeAccept
			updates["status"] = nextStatus
			activationTicket := *ticket
			activationTicket.CreatedAt = now
			addTicketAssignmentTrackingForTicketDB(updates, ctx.Tx, &activationTicket, now)
		}
		if err := repositories.TicketRepository.Updates(ctx.Tx, ticket.ID, updates); err != nil {
			return err
		}
		ticket.CustomerID = customerID
		ticket.Status = nextStatus
		if customer := repositories.CustomerRepository.Get(ctx.Tx, customerID); customer != nil {
			customerJSON, marshalErr := json.Marshal(customer)
			if marshalErr != nil {
				return marshalErr
			}
			if err := repositories.TicketContextSnapshotRepository.UpdateCustomerJSON(ctx.Tx, ticket.TenantID, ticket.ID, string(customerJSON)); err != nil {
				return err
			}
		}
		progress := &models.TicketProgress{
			TenantID: ticket.TenantID, TicketID: ticket.ID, EventType: enums.TicketProgressEventProgress,
			Content: "Customer accepted invitation; draft ticket submitted", MetadataJSON: ticketProgressMetadata(enums.TicketStatusDraft, nextStatus),
			AuthorID: 0, CreatedAt: now,
		}
		if err := repositories.TicketProgressRepository.Create(ctx.Tx, progress); err != nil {
			return err
		}
		creator := &dto.AuthPrincipal{
			UserID: ticket.CreateUserID, Username: ticket.CreateUserName, TenantID: ticket.TenantID,
			DomainType: models.DomainTypeEnterprise, Domain: models.DomainTypeEnterprise,
		}
		if ticket.CurrentAssigneeID > 0 {
			if err := createTicketDispatchAttemptTx(ctx.Tx, ticket, ticket.CurrentTeamID, ticket.CurrentAssigneeID, "客户接受邀请后激活草稿工单", creator, now); err != nil {
				return err
			}
			if _, err := enqueueTicketAssignedEventTx(ctx.Tx, ticket, 0, ticket.CurrentAssigneeID, "客户接受邀请后激活草稿工单", creator, progress.ID, now); err != nil {
				return err
			}
		}
		if err := enqueueTicketCreatedEventTx(ctx, ticket, ticket.CreateUserID, "customer_registration", "user"); err != nil {
			return err
		}
	}
	return nil
}

const linkedTicketConversationRecoveryReason = "关联工单已创建，进入人工待接入"

func reconcileUnclaimedLinkedConversationTicketTx(
	ctx *sqls.TxContext,
	ticket *models.Ticket,
	operator *dto.AuthPrincipal,
	now time.Time,
) (bool, error) {
	if ctx == nil || ctx.Tx == nil || ticket == nil || ticket.ConversationID <= 0 ||
		ticket.TenantID <= 0 || ticket.CurrentAssigneeID > 0 || !isUnassignedTicketStatus(ticket.Status) {
		return false, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	conversation := &models.Conversation{}
	if err := ctx.Tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(conversation, ticket.ConversationID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if conversation.TenantID != ticket.TenantID || conversation.Status != enums.IMConversationStatusAIServing ||
		conversation.HandoffAt != nil || conversation.CurrentAssigneeID > 0 {
		return false, nil
	}

	updateUserID := auditOperatorID(operator)
	updateUserName := auditOperatorName(operator)
	result := ctx.Tx.Model(&models.Conversation{}).
		Where("id = ? AND status = ? AND handoff_at IS NULL AND current_assignee_id = 0", conversation.ID, enums.IMConversationStatusAIServing).
		Updates(map[string]any{
			"status":              enums.IMConversationStatusPending,
			"current_team_id":     ticket.CurrentTeamID,
			"current_assignee_id": int64(0),
			"handoff_at":          now,
			"handoff_reason":      linkedTicketConversationRecoveryReason,
			"updated_at":          now,
			"update_user_id":      updateUserID,
			"update_user_name":    updateUserName,
		})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	if err := ConversationEventLogService.CreateEvent(ctx, conversation.ID, enums.IMEventTypeTransfer, enums.IMSenderTypeSystem, updateUserID, linkedTicketConversationRecoveryReason, ConversationService.buildEventPayload(map[string]any{
		"ticketId":       ticket.ID,
		"fromStatus":     conversation.Status,
		"toStatus":       enums.IMConversationStatusPending,
		"fromAssigneeId": conversation.CurrentAssigneeID,
		"toAssigneeId":   int64(0),
		"toTeamId":       ticket.CurrentTeamID,
		"reason":         "linked_ticket_state_recovery",
	})); err != nil {
		return false, err
	}
	progress := &models.TicketProgress{
		TenantID:     ticket.TenantID,
		TicketID:     ticket.ID,
		EventType:    enums.TicketProgressEventAssigned,
		Content:      "关联会话状态已恢复为待接入",
		MetadataJSON: "{}",
		AuthorID:     updateUserID,
		CreatedAt:    now,
	}
	if err := repositories.TicketProgressRepository.Create(ctx.Tx, progress); err != nil {
		return false, err
	}
	return true, nil
}

func (s *ticketService) CreateFromConversation(req request.CreateTicketFromConversationRequest, operator *dto.AuthPrincipal) (*models.Ticket, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	conversation := ConversationService.Get(req.ConversationID)
	if conversation == nil {
		return nil, errorsx.InvalidParamI18n("error.e0116")
	}
	tenantID := operator.TenantID
	if tenantID <= 0 {
		tenantID = req.TenantID
	}
	if tenantID <= 0 || conversation.TenantID != tenantID {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	return s.withConversationTicketCreateLock(tenantID, conversation.ID, func() (*models.Ticket, error) {
		return s.createFromConversationLocked(req, operator, conversation, tenantID)
	})
}

func (s *ticketService) createFromConversationLocked(req request.CreateTicketFromConversationRequest, operator *dto.AuthPrincipal, conversation *models.Conversation, tenantID int64) (*models.Ticket, error) {
	if existing := s.findActiveConversationTicket(sqls.DB(), tenantID, conversation.ID); existing != nil {
		return existing, nil
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		req.IdempotencyKey = conversationTicketIdempotencyKey(conversation.ID)
	}
	return s.CreateTicket(s.buildCreateTicketRequestFromConversation(req, conversation, tenantID), operator)
}

func (s *ticketService) createFromConversationTx(ctx *sqls.TxContext, req request.CreateTicketFromConversationRequest, operator *dto.AuthPrincipal, conversation *models.Conversation, tenantID int64) (*models.Ticket, bool, error) {
	if ctx == nil || ctx.Tx == nil {
		return nil, false, errorsx.InvalidParam("ticket transaction is not available")
	}
	if operator == nil {
		return nil, false, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if conversation == nil {
		return nil, false, errorsx.InvalidParamI18n("error.e0116")
	}
	if tenantID <= 0 {
		tenantID = operator.TenantID
	}
	if tenantID <= 0 {
		tenantID = req.TenantID
	}
	if tenantID <= 0 || conversation.TenantID != tenantID {
		return nil, false, errorsx.ForbiddenI18n("error.e0225")
	}
	if existing := s.findActiveConversationTicket(ctx.Tx, tenantID, conversation.ID); existing != nil {
		return existing, false, nil
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		req.IdempotencyKey = conversationTicketIdempotencyKey(conversation.ID)
	}
	prepared, existing, err := s.prepareTicketCreate(ctx.Tx, s.buildCreateTicketRequestFromConversationDB(ctx.Tx, req, conversation, tenantID), operator)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, false, nil
	}
	if err := s.createTicketPreparedTx(ctx, prepared, operator); err != nil {
		if prepared.idempotencyKey != nil {
			if existing := repositories.TicketRepository.FindOne(ctx.Tx, sqls.NewCnd().
				Eq("tenant_id", prepared.tenantID).
				Eq("idempotency_key", prepared.idempotencyKeyValue)); existing != nil {
				return existing, false, nil
			}
		}
		return nil, false, err
	}
	return prepared.ticket, true, nil
}

func (s *ticketService) buildCreateTicketRequestFromConversation(req request.CreateTicketFromConversationRequest, conversation *models.Conversation, tenantID int64) request.CreateTicketRequest {
	return s.buildCreateTicketRequestFromConversationDB(sqls.DB(), req, conversation, tenantID)
}

func (s *ticketService) buildCreateTicketRequestFromConversationDB(db *gorm.DB, req request.CreateTicketFromConversationRequest, conversation *models.Conversation, tenantID int64) request.CreateTicketRequest {
	if db == nil {
		db = sqls.DB()
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = strings.TrimSpace(ConversationService.BuildConversationSummaryDB(db, conversation))
	}
	if title == "" {
		title = i18nx.Get("ticket.defaultConversationTitle")
	}
	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = strings.TrimSpace(conversation.LastMessageSummary)
	}
	if description == "" {
		description = title
	}
	faultCode := strings.TrimSpace(req.FaultCode)
	symptomSummary := strings.TrimSpace(req.SymptomSummary)
	diagnosisSummary := strings.TrimSpace(req.DiagnosisSummary)
	if faultCode == "" || symptomSummary == "" || diagnosisSummary == "" {
		inferredFaultCode, inferredSymptom, inferredDiagnosis := inferConversationTicketContextDB(db, conversation)
		if faultCode == "" {
			faultCode = inferredFaultCode
		}
		if symptomSummary == "" {
			symptomSummary = inferredSymptom
		}
		if diagnosisSummary == "" {
			diagnosisSummary = inferredDiagnosis
		}
	}
	productID := req.ProductID
	if productID <= 0 {
		productID = conversation.ProductID
	}
	productModelID := req.ProductModelID
	if productModelID <= 0 {
		productModelID = conversation.ProductModelID
	}
	productModuleID := req.ProductModuleID
	deviceID := req.DeviceID
	if deviceID <= 0 {
		deviceID = conversation.DeviceID
	}
	serviceCodeID := req.ServiceCodeID
	if serviceCodeID <= 0 {
		serviceCodeID = conversation.ServiceCodeID
	}
	customerEntrySessionID := req.CustomerEntrySessionID
	if customerEntrySessionID <= 0 {
		customerEntrySessionID = conversation.CustomerEntrySessionID
	}
	serviceRegion := strings.TrimSpace(req.ServiceRegion)
	if serviceRegion == "" && deviceID > 0 {
		if device := repositories.DeviceRepository.Get(db, deviceID); device != nil && device.TenantID == tenantID {
			serviceRegion = strings.TrimSpace(device.RegionCode)
		}
	}
	return request.CreateTicketRequest{
		IdempotencyKey:         req.IdempotencyKey,
		Title:                  title,
		Description:            description,
		PriorityCode:           req.PriorityCode,
		Source:                 string(enums.TicketSourceConversation),
		Channel:                s.resolveConversationChannelDB(db, conversation),
		CustomerID:             conversation.CustomerID,
		ConversationID:         conversation.ID,
		TagIDs:                 req.TagIDs,
		CurrentTeamID:          firstTicketPositiveInt64(req.CurrentTeamID, conversation.CurrentTeamID),
		CurrentAssigneeID:      req.CurrentAssigneeID,
		TenantID:               tenantID,
		ProductID:              productID,
		ProductModelID:         productModelID,
		ProductModuleID:        productModuleID,
		DeviceID:               deviceID,
		ServiceCodeID:          serviceCodeID,
		CustomerEntrySessionID: customerEntrySessionID,
		ServiceRegion:          serviceRegion,
		FaultCode:              faultCode,
		SymptomSummary:         symptomSummary,
		DiagnosisSummary:       diagnosisSummary,
		SLADueAt:               req.SLADueAt,
		ResolvedAt:             req.ResolvedAt,
	}
}

func (s *ticketService) withConversationTicketCreateLock(tenantID, conversationID int64, fn func() (*models.Ticket, error)) (*models.Ticket, error) {
	if fn == nil {
		return nil, nil
	}
	key := conversationTicketCreateLockKey(tenantID, conversationID)
	if key == "" {
		return fn()
	}
	db := sqls.DB()
	if db != nil && db.Dialector != nil && db.Dialector.Name() == "postgres" {
		var ticket *models.Ticket
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error; err != nil {
				return fmt.Errorf("lock conversation ticket create: %w", err)
			}
			var createErr error
			ticket, createErr = fn()
			return createErr
		})
		return ticket, err
	}

	lock := acquireLocalConversationCreateScopeLock(key)
	defer releaseLocalConversationCreateScopeLock(key, lock)
	return fn()
}

func conversationTicketCreateLockKey(tenantID, conversationID int64) string {
	if tenantID <= 0 || conversationID <= 0 {
		return ""
	}
	return strings.Join([]string{
		"remotehelpdesk:conversation-ticket:create",
		"tenant", strconv.FormatInt(tenantID, 10),
		"conversation", strconv.FormatInt(conversationID, 10),
	}, ":")
}

func (s *ticketService) findActiveConversationTicket(db *gorm.DB, tenantID, conversationID int64) *models.Ticket {
	if db == nil || tenantID <= 0 || conversationID <= 0 {
		return nil
	}
	return repositories.TicketRepository.FindOne(db, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("conversation_id", conversationID).
		Where("status NOT IN ?", []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled}).
		Desc("id"))
}

func conversationTicketIdempotencyKey(conversationID int64) string {
	if conversationID <= 0 {
		return ""
	}
	return fmt.Sprintf("conversation-ticket:%d", conversationID)
}

func inferConversationTicketContext(conversation *models.Conversation) (string, string, string) {
	return inferConversationTicketContextDB(sqls.DB(), conversation)
}

func inferConversationTicketContextDB(db *gorm.DB, conversation *models.Conversation) (string, string, string) {
	if conversation == nil || conversation.ID <= 0 {
		return "", "", ""
	}
	if db == nil {
		db = sqls.DB()
	}
	messages := repositories.MessageRepository.Find(db, sqls.NewCnd().
		Eq("conversation_id", conversation.ID).
		Desc("id").
		Limit(40))
	customerIssues := make([]string, 0, 3)
	customerSummaries := make([]string, 0, 8)
	faultCode := ""
	diagnosisSummary := ""
	seenIssues := make(map[string]struct{})
	for _, message := range messages {
		summary := strings.TrimSpace(buildMessageSummary(message.MessageType, message.Content))
		if summary == "" {
			continue
		}
		switch message.SenderType {
		case enums.IMSenderTypeCustomer:
			customerSummaries = append(customerSummaries, summary)
			if faultCode == "" {
				faultCode = extractConversationLabeledFaultCode(summary)
			}
			issue := handoffTicketIssueTitle(summary)
			if issue == "" || len(customerIssues) >= 3 {
				continue
			}
			if _, exists := seenIssues[issue]; exists {
				continue
			}
			seenIssues[issue] = struct{}{}
			customerIssues = append(customerIssues, issue)
		case enums.IMSenderTypeAI:
			if diagnosisSummary == "" && !isConversationQueueNotice(summary) {
				diagnosisSummary = limitText(summary, 2000)
			}
		}
	}
	if faultCode == "" {
		for _, summary := range customerSummaries {
			if faultCode = extractConversationUnlabeledFaultCode(summary); faultCode != "" {
				break
			}
		}
	}
	if faultCode == "" {
		faultCode = extractConversationFaultCode(diagnosisSummary)
	}
	symptomSummary := limitText(strings.Join(customerIssues, "\n"), 1200)
	if symptomSummary == "" {
		symptomSummary = limitText(conversation.LastMessageSummary, 1200)
	}
	return faultCode, symptomSummary, diagnosisSummary
}

func extractConversationFaultCode(value string) string {
	if faultCode := extractConversationLabeledFaultCode(value); faultCode != "" {
		return faultCode
	}
	return extractConversationUnlabeledFaultCode(value)
}

func extractConversationLabeledFaultCode(value string) string {
	if match := conversationTicketLabeledFaultCodePattern.FindStringSubmatch(strings.ToUpper(value)); len(match) > 1 {
		return match[1]
	}
	return ""
}

func extractConversationUnlabeledFaultCode(value string) string {
	for _, match := range conversationTicketFaultCodePattern.FindAllString(strings.ToUpper(value), -1) {
		switch match {
		case "P0", "P1", "P2", "P3", "P4":
			continue
		}
		if strings.HasPrefix(match, "V") && !strings.Contains(match, "-") {
			continue
		}
		return match
	}
	return ""
}

func isConversationQueueNotice(value string) bool {
	value = strings.TrimSpace(value)
	return value == strings.TrimSpace(HandoffWaitingMessage) ||
		value == strings.TrimSpace(HandoffOffHoursMessage) ||
		value == strings.TrimSpace(HandoffOffHoursQueuedMessage) ||
		value == strings.TrimSpace(HandoffAIHoldMessage)
}

// SyncConversationDispatchTx keeps the handoff ticket and its source conversation on
// the same product repair queue and assignee. It is called inside the conversation
// assignment transaction so a partial handoff cannot leave the ticket behind.
func (s *ticketService) SyncConversationDispatchTx(
	tx *gorm.DB,
	conversationID, teamID, assigneeID int64,
	reason string,
	operator *dto.AuthPrincipal,
) error {
	if tx == nil || conversationID <= 0 {
		return nil
	}
	ticket := repositories.TicketRepository.FindOne(tx, sqls.NewCnd().
		Eq("conversation_id", conversationID).
		Where("status NOT IN ?", []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled}).
		Desc("id"))
	if ticket == nil {
		return nil
	}
	if teamID <= 0 && ticket.ProductID > 0 {
		if team := ProductSupportOrganizationService.FindProductRepairTeam(tx, ticket.TenantID, ticket.ProductID); team != nil {
			teamID = team.ID
		}
	}
	if ticket.CurrentTeamID == teamID && ticket.CurrentAssigneeID == assigneeID {
		// Keep an accepted assignment intact, but still normalize newly-created
		// unassigned handoff tickets from the legacy pending status into the
		// dispatch pool. Product tickets are created with their repair team
		// already populated, so assignment equality alone is not enough here.
		if assigneeID > 0 || ticket.Status == enums.TicketStatusPendingDispatch {
			return nil
		}
	}
	now := time.Now()
	fromUserID := ticket.CurrentAssigneeID
	updates := map[string]any{
		"current_team_id":     teamID,
		"current_assignee_id": assigneeID,
		"updated_at":          now,
		"update_user_id":      auditOperatorID(operator),
		"update_user_name":    auditOperatorName(operator),
	}
	if assigneeID > 0 {
		updates["status"] = ticketAssignmentStatus(ticket.Status)
		addTicketAssignmentTrackingForTicketDB(updates, tx, ticket, now)
	} else if canAssignTicketStatus(ticket.Status) {
		updates["status"] = enums.TicketStatusPendingDispatch
		addTicketDispatchPoolUpdates(updates)
	}
	if err := repositories.TicketRepository.Updates(tx, ticket.ID, updates); err != nil {
		return err
	}
	if assigneeID <= 0 {
		if fromUserID <= 0 {
			return nil
		}
		content := "工单回到产品组待派单"
		if TenantCapabilityService.KnowledgeSupport(ticket.TenantID) {
			content = "工单回到技术支持组待派单"
		}
		if trimmed := strings.TrimSpace(reason); trimmed != "" {
			content += "，原因：" + trimmed
		}
		progress := &models.TicketProgress{
			TenantID:     ticket.TenantID,
			TicketID:     ticket.ID,
			EventType:    enums.TicketProgressEventAssigned,
			Content:      content,
			MetadataJSON: "{}",
			AuthorID:     auditOperatorID(operator),
			CreatedAt:    now,
		}
		return repositories.TicketProgressRepository.Create(tx, progress)
	}
	if fromUserID == assigneeID {
		return nil
	}
	content := "工单随会话自动分配"
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		content += "，原因：" + trimmed
	}
	progress := &models.TicketProgress{
		TenantID:     ticket.TenantID,
		TicketID:     ticket.ID,
		EventType:    enums.TicketProgressEventAssigned,
		Content:      content,
		MetadataJSON: "{}",
		AuthorID:     auditOperatorID(operator),
		CreatedAt:    now,
	}
	if err := repositories.TicketProgressRepository.Create(tx, progress); err != nil {
		return err
	}
	if err := createTicketDispatchAttemptTx(tx, ticket, teamID, assigneeID, reason, operator, now); err != nil {
		return err
	}
	// 与 Dashboard 派单、工单自动派单保持一致：发布 ticket.assigned 事件触发通知。
	// outbox 由调用方（tryAssignConversation）统一注册 WakeDefaultOutboxPublisher。
	if _, err := enqueueTicketAssignedEventTx(tx, ticket, fromUserID, assigneeID, strings.TrimSpace(reason), operator, progress.ID, now); err != nil {
		return err
	}
	return nil
}

func firstTicketPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func (s *ticketService) UpdateTicket(req request.UpdateTicketRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	title := strings.TrimSpace(req.Title)
	description := strings.TrimSpace(req.Description)
	if title == "" {
		return errorsx.InvalidParamI18n("error.e0181")
	}
	if description == "" {
		return errorsx.InvalidParamI18n("error.e0179")
	}
	ticket := s.Get(req.TicketID)
	if ticket == nil {
		return errorsx.InvalidParamI18n("error.e0178")
	}
	// 强制 tenant_id 一致：只能操作本租户的工单
	tenantID := operator.TenantID
	if tenantID <= 0 {
		// operator 未携带租户的内部/测试场景，沿用工单现有租户
		tenantID = ticket.TenantID
	}
	if ticket.TenantID != tenantID {
		return errorsx.ForbiddenI18n("error.e0225")
	}
	if req.CurrentAssigneeID > 0 && req.CurrentAssigneeID != ticket.CurrentAssigneeID {
		return errorsx.InvalidParam("ticket assignee must be changed through the assignment workflow")
	}
	// tenant_id 从 AuthPrincipal 派生，不接受客户端传入；其余字段保持原逻辑
	productID := req.ProductID
	productModelID := req.ProductModelID
	productModuleID := req.ProductModuleID
	deviceID := req.DeviceID
	serviceCodeID := req.ServiceCodeID
	customerEntrySessionID := req.CustomerEntrySessionID
	serviceRegion := strings.TrimSpace(req.ServiceRegion)
	faultCode := strings.TrimSpace(req.FaultCode)
	symptomSummary := strings.TrimSpace(req.SymptomSummary)
	diagnosisSummary := strings.TrimSpace(req.DiagnosisSummary)
	priorityCode, err := normalizeTicketPriorityCode(req.PriorityCode, ticket.PriorityCode)
	if err != nil {
		return err
	}
	slaDueAt := req.SLADueAt
	resolvedAt := req.ResolvedAt
	if updateTicketAfterSalesContextOmitted(req) {
		productID = ticket.ProductID
		productModelID = ticket.ProductModelID
		productModuleID = ticket.ProductModuleID
		deviceID = ticket.DeviceID
		serviceCodeID = ticket.ServiceCodeID
		customerEntrySessionID = ticket.CustomerEntrySessionID
		serviceRegion = ticket.ServiceRegion
		faultCode = ticket.FaultCode
		symptomSummary = ticket.SymptomSummary
		diagnosisSummary = ticket.DiagnosisSummary
		slaDueAt = ticket.SLADueAt
		resolvedAt = ticket.ResolvedAt
	} else {
		if productID == 0 {
			productID = ticket.ProductID
		}
		if productModelID == 0 {
			productModelID = ticket.ProductModelID
		}
		if productModuleID == 0 {
			productModuleID = ticket.ProductModuleID
		}
		if deviceID == 0 {
			deviceID = ticket.DeviceID
		}
		if serviceCodeID == 0 {
			serviceCodeID = ticket.ServiceCodeID
		}
		if customerEntrySessionID == 0 {
			customerEntrySessionID = ticket.CustomerEntrySessionID
		}
		if serviceRegion == "" {
			serviceRegion = ticket.ServiceRegion
		}
		if faultCode == "" {
			faultCode = ticket.FaultCode
		}
		if symptomSummary == "" {
			symptomSummary = ticket.SymptomSummary
		}
		if diagnosisSummary == "" {
			diagnosisSummary = ticket.DiagnosisSummary
		}
		if slaDueAt == nil {
			slaDueAt = ticket.SLADueAt
		}
		if resolvedAt == nil {
			resolvedAt = ticket.ResolvedAt
		}
	}
	if err := s.validateAfterSalesContext(tenantID, productID, productModelID, productModuleID, deviceID, serviceCodeID, customerEntrySessionID); err != nil {
		return err
	}
	ticket.ProductID, ticket.DeviceID, ticket.ServiceRegion = productID, deviceID, serviceRegion
	if err := refreshTicketIntakeDB(sqls.DB(), ticket); err != nil {
		return err
	}
	tagIDs, err := TicketTagService.ValidateTagIDs(req.TagIDs)
	if err != nil {
		return err
	}
	now := time.Now()
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.TicketRepository.Updates(ctx.Tx, ticket.ID, map[string]any{
			"title":                     title,
			"description":               description,
			"priority_code":             priorityCode,
			"tenant_id":                 tenantID,
			"product_id":                productID,
			"product_model_id":          productModelID,
			"product_module_id":         productModuleID,
			"device_id":                 deviceID,
			"service_code_id":           serviceCodeID,
			"customer_entry_session_id": customerEntrySessionID,
			"service_region":            serviceRegion,
			"fault_code":                faultCode,
			"symptom_summary":           symptomSummary,
			"diagnosis_summary":         diagnosisSummary,
			"context_status":            ticket.ContextStatus,
			"missing_context_json":      ticket.MissingContextJSON,
			"sla_due_at":                slaDueAt,
			"resolved_at":               resolvedAt,
			"updated_at":                now,
			"update_user_id":            operator.UserID,
			"update_user_name":          operator.Username,
		}); err != nil {
			return err
		}
		return TicketTagService.ReplaceTicketTags(ctx.Tx, ticket.ID, tagIDs, operator)
	})
}

func (s *ticketService) LinkCustomer(ticketID int64, customerID int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	ticket := s.Get(ticketID)
	if ticket == nil {
		return errorsx.InvalidParamI18n("error.e0178")
	}
	if err := requireTicketTenantAccess(ticket, operator); err != nil {
		return err
	}
	if customerID <= 0 || CustomerService.Get(customerID) == nil {
		return errorsx.InvalidParamI18n("error.e0155")
	}
	if err := s.validateCustomerTenant(customerID, ticket.TenantID); err != nil {
		return err
	}
	if ticket.ConversationID > 0 {
		conversation := ConversationService.Get(ticket.ConversationID)
		if conversation == nil {
			return errorsx.InvalidParamI18n("error.e0116")
		}
		if conversation.CustomerID > 0 && conversation.CustomerID != customerID {
			return errorsx.InvalidParamI18n("error.e0118")
		}
	}
	now := time.Now()
	ticket.CustomerID = customerID
	if err := refreshTicketIntakeDB(sqls.DB(), ticket); err != nil {
		return err
	}
	return repositories.TicketRepository.Updates(sqls.DB(), ticket.ID, map[string]any{
		"customer_id":          customerID,
		"context_status":       ticket.ContextStatus,
		"missing_context_json": ticket.MissingContextJSON,
		"updated_at":           now,
		"update_user_id":       operator.UserID,
		"update_user_name":     operator.Username,
	})
}

func (s *ticketService) AssignTicket(req request.AssignTicketRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	return TicketLifecycleService.Assign(req.TicketID, req.ToUserID, req.Reason, operator)
}

func (s *ticketService) ChangeStatus(req request.ChangeTicketStatusRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	status := strings.TrimSpace(req.Status)
	if !enums.IsValidTicketStatus(status) && !enums.IsValidAfterSalesStatus(status) {
		return errorsx.InvalidParamI18n("error.e0182")
	}
	ticket := s.Get(req.TicketID)
	if ticket == nil {
		return errorsx.InvalidParamI18n("error.e0178")
	}
	if err := requireTicketTenantAccess(ticket, operator); err != nil {
		return err
	}
	if err := requireTicketMutationAccess(ticket, operator); err != nil {
		return err
	}
	newStatus := enums.TicketStatus(status)
	if newStatus == enums.TicketStatusCancelled {
		return TicketLifecycleService.Cancel(ticket.ID, req.Resolution, operator)
	}
	// 校验合法状态流转
	if !enums.IsValidTicketStatusTransition(string(ticket.Status), status) && !enums.IsValidAfterSalesTransition(string(ticket.Status), status) {
		return errorsx.InvalidParamI18n("error.e0182")
	}
	// resolved/closed/done must pass through lifecycle or repair-conclusion paths so
	// DeviceServiceRecord, KnowledgeCandidate and quality clues are generated.
	if newStatus == enums.TicketStatusResolved {
		return errorsx.InvalidParam("ticket must be resolved via repair record conclusion")
	}
	if newStatus == enums.TicketStatusClosed || newStatus == enums.TicketStatusDone {
		return errorsx.InvalidParam("ticket must be closed via TicketLifecycleService.Close, use the close endpoint instead")
	}

	now := time.Now()
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		updates := map[string]any{
			"status":           newStatus,
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		}
		dispatchOutcome, acceptedAt := applyTicketStatusDispatchSideEffects(updates, ticket, newStatus, now)
		if err := repositories.TicketRepository.Updates(ctx.Tx, ticket.ID, updates); err != nil {
			return err
		}
		if dispatchOutcome != "" {
			if err := finishPendingTicketDispatchAttemptTx(ctx.Tx, ticket.ID, dispatchOutcome, acceptedAt, now); err != nil {
				return err
			}
		}
		return repositories.TicketProgressRepository.Create(ctx.Tx, &models.TicketProgress{
			TenantID:     ticket.TenantID,
			TicketID:     ticket.ID,
			EventType:    ticketProgressEventForStatus(newStatus),
			Content:      "状态流转：" + string(newStatus),
			MetadataJSON: ticketProgressMetadata(ticket.Status, newStatus),
			AuthorID:     operator.UserID,
			CreatedAt:    now,
		})
	})
}

// Transition 按售后状态机推进工单状态（设计 §6.2）。
// 与 ChangeStatus 不同：Transition 使用售后状态机，允许沿合法路径到达 resolved/closed，
// 并在到达 resolved 时写 resolved_at、到达 closed 时写 handled_at。
func (s *ticketService) Transition(req request.TransitionTicketRequest, operator *dto.AuthPrincipal) (*models.Ticket, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	ticket := s.Get(req.TicketID)
	if ticket == nil {
		return nil, errorsx.InvalidParamI18n("error.e0178")
	}
	if err := requireTicketTenantAccess(ticket, operator); err != nil {
		return nil, err
	}
	if err := requireTicketMutationAccess(ticket, operator); err != nil {
		return nil, err
	}
	target := enums.NormalizeTicketStatus(strings.TrimSpace(req.Status))
	if !enums.IsValidAfterSalesStatus(string(target)) {
		return nil, errorsx.InvalidParamI18n("error.e0182")
	}
	if !enums.IsValidAfterSalesTransition(string(ticket.Status), string(target)) {
		return nil, errorsx.InvalidParamI18n("error.e0182")
	}
	now := time.Now()
	updates := map[string]any{
		"status":           target,
		"updated_at":       now,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
	}
	switch target {
	case enums.TicketStatusResolved:
		updates["resolved_at"] = &now
	case enums.TicketStatusClosed:
		updates["handled_at"] = &now
	case enums.TicketStatusReopened:
		updates["resolved_at"] = nil
		updates["handled_at"] = nil
	}
	dispatchOutcome, acceptedAt := applyTicketStatusDispatchSideEffects(updates, ticket, target, now)
	content := "状态流转：" + string(target)
	if remark := strings.TrimSpace(req.Remark); remark != "" {
		content += "，备注：" + remark
	}
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.TicketRepository.Updates(ctx.Tx, ticket.ID, updates); err != nil {
			return err
		}
		if dispatchOutcome != "" {
			if err := finishPendingTicketDispatchAttemptTx(ctx.Tx, ticket.ID, dispatchOutcome, acceptedAt, now); err != nil {
				return err
			}
		}
		return repositories.TicketProgressRepository.Create(ctx.Tx, &models.TicketProgress{
			TenantID:     ticket.TenantID,
			TicketID:     ticket.ID,
			EventType:    ticketProgressEventForStatus(target),
			Content:      content,
			MetadataJSON: ticketProgressMetadata(ticket.Status, target),
			AuthorID:     operator.UserID,
			CreatedAt:    now,
		})
	}); err != nil {
		return nil, err
	}
	return s.Get(ticket.ID), nil
}

// CreateRepairRecord 创建工单维修记录。产品咨询可以没有设备，此时不会写设备维修履历。
func (s *ticketService) CreateRepairRecord(req request.CreateTicketRepairRecordRequest, operator *dto.AuthPrincipal) (*models.TicketRepairRecord, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	ticket := s.Get(req.TicketID)
	if ticket == nil {
		return nil, errorsx.InvalidParamI18n("error.e0178")
	}
	if err := requireTicketTenantAccess(ticket, operator); err != nil {
		return nil, err
	}
	if err := requireTicketMutationAccess(ticket, operator); err != nil {
		return nil, err
	}
	if ticket.TenantID <= 0 || (ticket.ProductID <= 0 && !TenantCapabilityService.KnowledgeSupport(ticket.TenantID)) {
		return nil, errorsx.InvalidParam("repair record requires tenant and product context")
	}
	faultCode := resolveRepairFaultCode(ticket.FaultCode, req.FaultCode)
	testResult := strings.TrimSpace(req.TestResult)
	rootCause := strings.TrimSpace(req.RootCause)
	solution := strings.TrimSpace(req.Solution)
	if solution == "" {
		solution = strings.TrimSpace(req.Conclusion)
	}
	if req.MarkResolved && faultCode == "" {
		return nil, errorsx.InvalidParam("fault type is required before submitting a repair conclusion")
	}
	if req.MarkResolved && rootCause == "" {
		return nil, errorsx.InvalidParam("root cause is required before submitting a repair conclusion")
	}
	if req.MarkResolved && solution == "" {
		return nil, errorsx.InvalidParam("solution is required before submitting a repair conclusion")
	}
	if req.MarkResolved && testResult != "passed" {
		return nil, errorsx.InvalidParam("only a passed verification can move the ticket to customer confirmation")
	}
	if req.MarkResolved && !canCompleteRepairFromStatus(ticket.Status) && enums.NormalizeTicketStatus(string(ticket.Status)) != enums.TicketStatusResolved && !canManageTicketDispatch(operator) {
		return nil, errorsx.InvalidParam("ticket must be accepted before submitting a repair conclusion")
	}
	now := time.Now()
	partsJSON, err := json.Marshal(req.Parts)
	if err != nil {
		return nil, errorsx.InvalidParam("invalid repair parts")
	}
	record := &models.TicketRepairRecord{
		TenantID:          ticket.TenantID,
		TicketID:          ticket.ID,
		DeviceID:          ticket.DeviceID,
		ProductID:         ticket.ProductID,
		ProductModelID:    ticket.ProductModelID,
		ServiceCodeID:     ticket.ServiceCodeID,
		ServiceMethod:     strings.TrimSpace(req.ServiceMethod),
		Conclusion:        strings.TrimSpace(req.Conclusion),
		RootCause:         rootCause,
		RepairMethod:      strings.TrimSpace(req.RepairMethod),
		Solution:          solution,
		TestResult:        testResult,
		WarrantyCovered:   req.WarrantyCovered,
		RemoteResolved:    req.RemoteResolved,
		VisibleToCustomer: req.VisibleToCustomer,
		PartsJSON:         string(partsJSON),
		CostHours:         req.CostHours,
		StartedAt:         &now,
		FinishedAt:        &now,
		AuditFields:       utils.BuildAuditFields(operator),
	}
	created := false
	knowledgeCandidateChanged := false
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		lockedTicket := &models.Ticket{}
		if err := ctx.Tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(lockedTicket, ticket.ID).Error; err != nil {
			return err
		}
		if err := requireTicketTenantAccess(lockedTicket, operator); err != nil {
			return err
		}
		if err := requireTicketMutationAccess(lockedTicket, operator); err != nil {
			return err
		}
		if req.MarkResolved && enums.NormalizeTicketStatus(string(lockedTicket.Status)) == enums.TicketStatusResolved {
			if resolveRepairFaultCode(lockedTicket.FaultCode, req.FaultCode) != strings.TrimSpace(lockedTicket.FaultCode) {
				return errorsx.InvalidParam("a different repair conclusion has already been submitted for this resolution")
			}
			repairs := repositories.TicketRepairRepository.FindByTicketID(ctx.Tx, lockedTicket.ID)
			if len(repairs) > 0 && isIdempotentRepairRecordRetry(&repairs[len(repairs)-1], record) {
				record = &repairs[len(repairs)-1]
				return nil
			}
			return errorsx.InvalidParam("a different repair conclusion has already been submitted for this resolution")
		}
		if req.MarkResolved && !canCompleteRepairFromStatus(lockedTicket.Status) && !canManageTicketDispatch(operator) {
			return errorsx.InvalidParam("ticket must be accepted before submitting a repair conclusion")
		}
		if req.MarkResolved {
			if err := s.resolveSupplierCollaborationsForRepairConclusionTx(ctx.Tx, lockedTicket, record, operator, now); err != nil {
				return err
			}
			if err := validateRepairCompletionReadiness(ctx.Tx, lockedTicket); err != nil {
				return err
			}
		}
		record.TenantID = lockedTicket.TenantID
		record.TicketID = lockedTicket.ID
		record.DeviceID = lockedTicket.DeviceID
		record.ProductID = lockedTicket.ProductID
		record.ProductModelID = lockedTicket.ProductModelID
		record.ServiceCodeID = lockedTicket.ServiceCodeID
		if err := repositories.TicketRepairRepository.Create(ctx.Tx, record); err != nil {
			return err
		}
		created = true
		if lockedTicket.DeviceID > 0 {
			if err := TicketLifecycleService.writeDeviceServiceRecord(ctx.Tx, lockedTicket, record, operator); err != nil {
				return err
			}
		}
		if err := repositories.TicketProgressRepository.Create(ctx.Tx, &models.TicketProgress{
			TenantID:          ticket.TenantID,
			TicketID:          ticket.ID,
			EventType:         enums.TicketProgressEventRepairCompleted,
			Content:           "已填写维修记录",
			MetadataJSON:      "{}",
			AuthorID:          operator.UserID,
			VisibleToCustomer: req.VisibleToCustomer,
			CreatedAt:         now,
		}); err != nil {
			return err
		}
		updates := map[string]any{
			"fault_code":       faultCode,
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		}
		if req.MarkResolved {
			updates["status"] = enums.TicketStatusResolved
			updates["resolved_at"] = &now
		}
		if err := repositories.TicketRepository.Updates(ctx.Tx, ticket.ID, updates); err != nil {
			return err
		}
		lockedTicket.FaultCode = faultCode
		lockedTicket.UpdatedAt = now
		lockedTicket.UpdateUserID = operator.UserID
		lockedTicket.UpdateUserName = operator.Username
		if req.MarkResolved {
			lockedTicket.Status = enums.TicketStatusResolved
			lockedTicket.ResolvedAt = &now
			repairs := repositories.TicketRepairRepository.FindByTicketID(ctx.Tx, lockedTicket.ID)
			resultSummary := firstNonBlank(strings.TrimSpace(record.Conclusion), strings.TrimSpace(record.Solution), "维修结论已提交")
			candidateEvent, err := TicketLifecycleService.ensureTicketRepairKnowledgeCandidateTx(ctx.Tx, lockedTicket, repairs, resultSummary, operator)
			if err != nil {
				return err
			}
			if candidateEvent != nil {
				if err := TicketLifecycleService.enqueueKnowledgeCandidateCreatedEventTx(ctx.Tx, candidateEvent, operator, "ticket_repair_service"); err != nil {
					return err
				}
				knowledgeCandidateChanged = true
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if knowledgeCandidateChanged {
		eventbus.WakeDefaultOutboxPublisher()
	}
	if created && req.MarkResolved && ticket.ConversationID > 0 {
		if req.VisibleToCustomer {
			payload, _ := json.Marshal(map[string]any{
				"eventType":        "ticket_resolved",
				"source":           "ticket_repair",
				"ticketId":         ticket.ID,
				"ticketNo":         ticket.TicketNo,
				"repairId":         record.ID,
				"hasDeviceContext": ticket.DeviceID > 0,
			})
			resultSummary := strings.TrimSpace(record.Conclusion)
			if resultSummary == "" {
				resultSummary = strings.TrimSpace(record.Solution)
			}
			content := "维修结论已提交，等待客户确认服务结果。"
			if ticket.DeviceID > 0 {
				content = "维修结论已提交，等待客户确认设备状态。"
			}
			if resultSummary != "" {
				content = "维修结论已提交：" + resultSummary
			}
			if _, eventErr := MessageService.SendSystemMessageWithRequestID(
				ticket.ConversationID,
				fmt.Sprintf("ticket_resolved_%d_%d", ticket.ID, record.ID),
				content,
				string(payload),
				"",
			); eventErr != nil {
				slog.Warn("publish repair conclusion conversation event failed", "ticket_id", ticket.ID, "repair_id", record.ID, "error", eventErr)
			}
		}
		if conversation := ConversationService.Get(ticket.ConversationID); conversation != nil {
			WsService.PublishConversationChanged(conversation, enums.IMRealtimeEventConversationUpdated)
		}
	}
	return record, nil
}

func isIdempotentRepairRecordRetry(existing, submitted *models.TicketRepairRecord) bool {
	if existing == nil || submitted == nil {
		return false
	}
	return existing.TenantID == submitted.TenantID &&
		existing.TicketID == submitted.TicketID &&
		existing.DeviceID == submitted.DeviceID &&
		existing.ProductID == submitted.ProductID &&
		existing.ProductModelID == submitted.ProductModelID &&
		existing.ServiceCodeID == submitted.ServiceCodeID &&
		existing.ServiceMethod == submitted.ServiceMethod &&
		existing.Conclusion == submitted.Conclusion &&
		existing.RootCause == submitted.RootCause &&
		existing.RepairMethod == submitted.RepairMethod &&
		existing.Solution == submitted.Solution &&
		existing.TestResult == submitted.TestResult &&
		existing.WarrantyCovered == submitted.WarrantyCovered &&
		existing.RemoteResolved == submitted.RemoteResolved &&
		existing.VisibleToCustomer == submitted.VisibleToCustomer &&
		existing.PartsJSON == submitted.PartsJSON &&
		existing.CostHours == submitted.CostHours
}

func resolveRepairFaultCode(reported, submitted string) string {
	reported = strings.TrimSpace(reported)
	submitted = strings.TrimSpace(submitted)
	if submitted == "" {
		return reported
	}
	upperSubmitted := strings.ToUpper(submitted)
	if match := conversationTicketFaultCodePattern.FindString(upperSubmitted); match == upperSubmitted {
		return match
	}
	if reported != "" {
		return reported
	}
	return submitted
}

func canCompleteRepairFromStatus(status enums.TicketStatus) bool {
	normalized := enums.NormalizeTicketStatus(string(status))
	switch normalized {
	case enums.TicketStatusAccepted,
		enums.TicketStatusProcessing,
		enums.TicketStatusVideoSupport,
		enums.TicketStatusSupplierSupport:
		return true
	default:
		return false
	}
}

func validateRepairCompletionReadiness(db *gorm.DB, ticket *models.Ticket) error {
	if db == nil || ticket == nil {
		return nil
	}
	if repositories.TicketRepository.HasUnfinishedMeeting(db, ticket.ID) {
		return errorsx.InvalidParam("video meeting must be ended before submitting a repair conclusion")
	}
	return nil
}

func (s *ticketService) resolveSupplierCollaborationsForRepairConclusionTx(db *gorm.DB, ticket *models.Ticket, record *models.TicketRepairRecord, operator *dto.AuthPrincipal, resolvedAt time.Time) error {
	if db == nil || ticket == nil || operator == nil {
		return nil
	}
	activeCount := repositories.TicketSupplierCollaborationRepository.CountActiveByTicket(db, ticket.TenantID, ticket.ID)
	if activeCount == 0 {
		return nil
	}
	summary := "维修结论已提交"
	if record != nil {
		summary = firstNonBlank(record.Conclusion, record.Solution, record.RootCause, summary)
	}
	resolution := "供应商协作随维修结论关闭：" + summary
	if err := repositories.TicketSupplierCollaborationRepository.ResolveActiveByTicket(
		db, ticket.TenantID, ticket.ID, resolution, resolvedAt, operator.UserID, operator.Username,
	); err != nil {
		return err
	}
	if err := repositories.TicketSupplierCollaborationRepository.DisableAllTicketAuthorizationScopes(db, ticket.TenantID, ticket.ID, resolvedAt); err != nil {
		return err
	}
	if err := repositories.TicketSupplierCollaborationRepository.DisableParticipantsByTicket(
		db, ticket.TenantID, ticket.ID, resolvedAt, operator.UserID, operator.Username,
	); err != nil {
		return err
	}
	if ticket.ConversationID > 0 {
		if err := db.Model(&models.ConversationParticipant{}).
			Where("conversation_id = ? AND participant_type = ? AND status = ?", ticket.ConversationID, enums.IMParticipantTypePartner, enums.StatusOk).
			Updates(map[string]any{
				"left_at":          resolvedAt,
				"status":           enums.StatusDisabled,
				"updated_at":       resolvedAt,
				"update_user_id":   operator.UserID,
				"update_user_name": operator.Username,
			}).Error; err != nil {
			return err
		}
	}
	metadata, _ := json.Marshal(map[string]any{
		"source":                     "ticket_repair_conclusion",
		"auto_resolved_supplier_cnt": activeCount,
	})
	return repositories.TicketProgressRepository.Create(db, &models.TicketProgress{
		TenantID:     ticket.TenantID,
		TicketID:     ticket.ID,
		EventType:    enums.TicketProgressEventProgress,
		Content:      "供应商协作已随维修结论关闭",
		MetadataJSON: string(metadata),
		AuthorID:     operator.UserID,
		CreatedAt:    resolvedAt,
	})
}

func canStartMeetingFromStatus(status enums.TicketStatus) bool {
	normalized := enums.NormalizeTicketStatus(string(status))
	switch normalized {
	case enums.TicketStatusAccepted,
		enums.TicketStatusProcessing,
		enums.TicketStatusVideoSupport,
		enums.TicketStatusSupplierSupport:
		return true
	default:
		return false
	}
}

func applyTicketStatusDispatchSideEffects(updates map[string]any, ticket *models.Ticket, target enums.TicketStatus, now time.Time) (string, *time.Time) {
	if updates == nil || ticket == nil {
		return "", nil
	}
	switch enums.NormalizeTicketStatus(string(target)) {
	case enums.TicketStatusPendingDispatch:
		addTicketDispatchPoolUpdates(updates)
		return ticketDispatchOutcomeSuperseded, nil
	case enums.TicketStatusAccepted, enums.TicketStatusProcessing:
		addTicketDispatchSettlementUpdates(updates)
		if ticket.CurrentAssigneeID <= 0 {
			return "", nil
		}
		if ticket.AcceptedAt != nil {
			return ticketDispatchOutcomeAccepted, ticket.AcceptedAt
		}
		updates["accepted_at"] = now
		return ticketDispatchOutcomeAccepted, &now
	case enums.TicketStatusVideoSupport,
		enums.TicketStatusSupplierSupport,
		enums.TicketStatusResolved,
		enums.TicketStatusPendingCustomerConfirm,
		enums.TicketStatusQualityReview,
		enums.TicketStatusReopened:
		addTicketDispatchSettlementUpdates(updates)
		return ticketDispatchOutcomeSuperseded, nil
	default:
		return "", nil
	}
}

func (s *ticketService) AddProgress(req request.CreateTicketProgressRequest, operator *dto.AuthPrincipal) (*models.TicketProgress, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, errorsx.InvalidParamI18n("error.e0148")
	}
	ticket := s.Get(req.TicketID)
	if ticket == nil {
		return nil, errorsx.InvalidParamI18n("error.e0178")
	}
	if err := requireTicketTenantAccess(ticket, operator); err != nil {
		return nil, err
	}
	if err := requireTicketMutationAccess(ticket, operator); err != nil {
		return nil, err
	}
	now := time.Now()
	progress := &models.TicketProgress{
		TenantID:          ticket.TenantID,
		TicketID:          ticket.ID,
		EventType:         enums.TicketProgressEventProgress,
		Content:           content,
		VisibleToCustomer: req.VisibleToCustomer,
		MetadataJSON:      "{}",
		AuthorID:          operator.UserID,
		CreatedAt:         now,
	}
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		if err := repositories.TicketProgressRepository.Create(ctx.Tx, progress); err != nil {
			return err
		}
		return repositories.TicketRepository.Updates(ctx.Tx, ticket.ID, map[string]any{
			"updated_at":       now,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
		})
	}); err != nil {
		return nil, err
	}
	return progress, nil
}

func (s *ticketService) GetDetail(id int64) (*TicketDetailAggregate, error) {
	ticket := s.Get(id)
	if ticket == nil {
		return nil, errorsx.InvalidParamI18n("error.e0178")
	}
	aggregate := &TicketDetailAggregate{
		Ticket:     ticket,
		Tags:       s.GetTags(id),
		Progresses: repositories.TicketProgressRepository.Find(sqls.DB(), sqls.NewCnd().Eq("ticket_id", id).Asc("id")),
		Users:      make(map[int64]*models.User),
	}
	if ticket.CustomerID > 0 {
		aggregate.Customer = CustomerService.Get(ticket.CustomerID)
	}
	userIDs := make([]int64, 0)
	seen := make(map[int64]struct{})
	addUserID := func(userID int64) {
		if userID <= 0 {
			return
		}
		if _, ok := seen[userID]; ok {
			return
		}
		seen[userID] = struct{}{}
		userIDs = append(userIDs, userID)
	}
	addUserID(ticket.CurrentAssigneeID)
	for i := range aggregate.Progresses {
		addUserID(aggregate.Progresses[i].AuthorID)
	}
	if len(userIDs) > 0 {
		users := repositories.UserRepository.FindByIds(sqls.DB(), userIDs)
		for i := range users {
			item := users[i]
			aggregate.Users[item.ID] = &item
		}
	}
	aggregate.RepairRecords = repositories.TicketRepairRepository.FindByTicketID(sqls.DB(), id)
	return aggregate, nil
}

func (s *ticketService) GetSummary(operator *dto.AuthPrincipal, staleHours ...int) *TicketSummaryAggregate {
	staleHour := 0
	if len(staleHours) > 0 {
		staleHour = staleHours[0]
	}
	summary := &TicketSummaryAggregate{
		All:        s.Count(sqls.NewCnd()),
		Pending:    s.Count(sqls.NewCnd().Eq("status", enums.TicketStatusPending)),
		InProgress: s.Count(sqls.NewCnd().Eq("status", enums.TicketStatusInProgress)),
		Done:       s.Count(sqls.NewCnd().Eq("status", enums.TicketStatusClosed)) + s.Count(sqls.NewCnd().Eq("status", enums.TicketStatusDone)),
		Unassigned: s.Count(sqls.NewCnd().Eq("current_assignee_id", 0)),
		Stale:      s.Count(s.ApplyStaleFilter(sqls.NewCnd(), staleHour)),
	}
	if operator != nil {
		summary.Mine = s.Count(sqls.NewCnd().Eq("current_assignee_id", operator.UserID))
	}
	return summary
}

func (s *ticketService) buildTicketListAggregate(db *gorm.DB, list []models.Ticket, paging *sqls.Paging) *TicketListAggregate {
	aggregate := &TicketListAggregate{
		List:           list,
		Paging:         paging,
		TagsByTicketID: make(map[int64][]models.Tag),
		Users:          make(map[int64]*models.User),
		Customers:      make(map[int64]*models.Customer),
	}
	if len(list) == 0 {
		return aggregate
	}
	ticketIDs := make([]int64, 0, len(list))
	customerIDs := make([]int64, 0)
	userIDs := make([]int64, 0)
	ticketSeen := make(map[int64]struct{})
	customerSeen := make(map[int64]struct{})
	userSeen := make(map[int64]struct{})
	for i := range list {
		item := &list[i]
		if _, ok := ticketSeen[item.ID]; !ok {
			ticketSeen[item.ID] = struct{}{}
			ticketIDs = append(ticketIDs, item.ID)
		}
		if item.CustomerID > 0 {
			if _, ok := customerSeen[item.CustomerID]; !ok {
				customerSeen[item.CustomerID] = struct{}{}
				customerIDs = append(customerIDs, item.CustomerID)
			}
		}
		if item.CurrentAssigneeID > 0 {
			if _, ok := userSeen[item.CurrentAssigneeID]; !ok {
				userSeen[item.CurrentAssigneeID] = struct{}{}
				userIDs = append(userIDs, item.CurrentAssigneeID)
			}
		}
	}
	s.enrichTicketTags(db, aggregate, ticketIDs)
	if len(userIDs) > 0 {
		users := repositories.UserRepository.FindByIds(db, userIDs)
		for i := range users {
			item := users[i]
			aggregate.Users[item.ID] = &item
		}
	}
	if len(customerIDs) > 0 {
		customers := repositories.CustomerRepository.Find(db, sqls.NewCnd().In("id", customerIDs))
		for i := range customers {
			item := customers[i]
			aggregate.Customers[item.ID] = &item
		}
	}
	return aggregate
}

func (s *ticketService) enrichTicketTags(db *gorm.DB, aggregate *TicketListAggregate, ticketIDs []int64) {
	if len(ticketIDs) == 0 {
		return
	}
	ticketTags := repositories.TicketTagRepository.Find(db, sqls.NewCnd().In("ticket_id", ticketIDs).Asc("id"))
	if len(ticketTags) == 0 {
		return
	}
	tagIDs := make([]int64, 0)
	tagSeen := make(map[int64]struct{})
	ticketTagMap := make(map[int64][]int64, len(ticketIDs))
	for i := range ticketTags {
		relation := ticketTags[i]
		ticketTagMap[relation.TicketID] = append(ticketTagMap[relation.TicketID], relation.TagID)
		if _, ok := tagSeen[relation.TagID]; !ok {
			tagSeen[relation.TagID] = struct{}{}
			tagIDs = append(tagIDs, relation.TagID)
		}
	}
	tags := repositories.TagRepository.Find(db, sqls.NewCnd().In("id", tagIDs))
	tagMap := make(map[int64]models.Tag, len(tags))
	for i := range tags {
		tagMap[tags[i].ID] = tags[i]
	}
	for ticketID, orderedTagIDs := range ticketTagMap {
		orderedTags := make([]models.Tag, 0, len(orderedTagIDs))
		for _, tagID := range orderedTagIDs {
			if tag, ok := tagMap[tagID]; ok {
				orderedTags = append(orderedTags, tag)
			}
		}
		aggregate.TagsByTicketID[ticketID] = orderedTags
	}
}

func (s *ticketService) createContextSnapshot(tx *gorm.DB, ticket *models.Ticket) error {
	if ticket == nil {
		return nil
	}
	productJSON := "{}"
	deviceJSON := "{}"
	customerJSON := "{}"
	conversationJSON := "{}"
	diagnosisJSON := "{}"
	serviceCodeJSON := "{}"

	if ticket.ProductID > 0 {
		if product := repositories.ProductRepository.Get(tx, ticket.ProductID); product != nil {
			if b, err := json.Marshal(product); err == nil {
				productJSON = string(b)
			}
		}
	}
	if ticket.DeviceID > 0 {
		if device := repositories.DeviceRepository.Get(tx, ticket.DeviceID); device != nil {
			if b, err := json.Marshal(device); err == nil {
				deviceJSON = string(b)
			}
		}
	}
	if ticket.CustomerID > 0 {
		if customer := repositories.CustomerRepository.Get(tx, ticket.CustomerID); customer != nil {
			if b, err := json.Marshal(customer); err == nil {
				customerJSON = string(b)
			}
		}
	}
	if ticket.ConversationID > 0 {
		if conversation := repositories.ConversationRepository.Get(tx, ticket.ConversationID); conversation != nil {
			if b, err := json.Marshal(conversation); err == nil {
				conversationJSON = string(b)
			}
		}
	}
	if ticket.DiagnosisSummary != "" {
		diagnosisJSON = ticket.DiagnosisSummary
	}
	if ticket.ServiceCodeID > 0 {
		if sc := repositories.ServiceCodeRepository.Get(tx, ticket.ServiceCodeID); sc != nil {
			if b, err := json.Marshal(sc); err == nil {
				serviceCodeJSON = string(b)
			}
		}
	}

	snapshot := &models.TicketContextSnapshot{
		TicketID:         ticket.ID,
		TenantID:         ticket.TenantID,
		ProductJSON:      productJSON,
		DeviceJSON:       deviceJSON,
		CustomerJSON:     customerJSON,
		ConversationJSON: conversationJSON,
		DiagnosisJSON:    diagnosisJSON,
		ServiceCodeJSON:  serviceCodeJSON,
		CreatedAt:        time.Now(),
	}
	return repositories.TicketContextSnapshotRepository.Create(tx, snapshot)
}

func (s *ticketService) validateTicketRefs(customerID, conversationID, assigneeID, tenantID int64) error {
	return s.validateTicketRefsDB(sqls.DB(), customerID, conversationID, assigneeID, tenantID)
}

func (s *ticketService) validateTicketRefsDB(db *gorm.DB, customerID, conversationID, assigneeID, tenantID int64) error {
	if db == nil {
		db = sqls.DB()
	}
	if customerID > 0 && repositories.CustomerRepository.Get(db, customerID) == nil {
		return errorsx.InvalidParamI18n("error.e0155")
	}
	if customerID > 0 {
		if err := s.validateCustomerTenantDB(db, customerID, tenantID); err != nil {
			return err
		}
	}
	if conversationID > 0 {
		conversation := repositories.ConversationRepository.Get(db, conversationID)
		if conversation == nil {
			return errorsx.InvalidParamI18n("error.e0116")
		}
		if customerID > 0 && conversation.CustomerID != customerID {
			return errorsx.InvalidParamI18n("error.e0118")
		}
		if tenantID <= 0 || conversation.TenantID != tenantID {
			return errorsx.ForbiddenI18n("error.e0225")
		}
	}
	return s.validateAssignee(assigneeID)
}

func (s *ticketService) validateCustomerTenant(customerID, tenantID int64) error {
	return s.validateCustomerTenantDB(sqls.DB(), customerID, tenantID)
}

func (s *ticketService) validateCustomerTenantDB(db *gorm.DB, customerID, tenantID int64) error {
	if db == nil {
		db = sqls.DB()
	}
	if tenantID <= 0 || repositories.CustomerRepository.HasTenantAssociation(db, customerID, tenantID) {
		return nil
	}
	if repositories.CustomerRepository.HasOtherTenantAssociation(db, customerID, tenantID) {
		return errorsx.ForbiddenI18n("error.e0225")
	}
	return nil
}

func (s *ticketService) validateAfterSalesContext(tenantID, productID, productModelID, productModuleID, deviceID, serviceCodeID, sessionID int64) error {
	return s.validateAfterSalesContextDB(sqls.DB(), tenantID, productID, productModelID, productModuleID, deviceID, serviceCodeID, sessionID)
}

func (s *ticketService) validateAfterSalesContextDB(db *gorm.DB, tenantID, productID, productModelID, productModuleID, deviceID, serviceCodeID, sessionID int64) error {
	if tenantID == 0 && productID == 0 && productModelID == 0 && productModuleID == 0 && deviceID == 0 && serviceCodeID == 0 && sessionID == 0 {
		return nil
	}
	if db == nil {
		db = sqls.DB()
	}
	tenant, err := requireActiveTenantDB(db, tenantID)
	if err != nil {
		return err
	}
	var product *models.Product
	if productID > 0 {
		product = repositories.ProductRepository.Get(db, productID)
		if product == nil || product.TenantID != tenant.ID || product.Status != enums.StatusOk {
			return errorsx.InvalidParam("product is invalid for tenant")
		}
	}
	var productModel *models.ProductModel
	if productModelID > 0 {
		productModel = repositories.ProductModelRepository.Get(db, productModelID)
		if productModel == nil || productModel.TenantID != tenant.ID || (product != nil && productModel.ProductID != product.ID) || productModel.Status != enums.StatusOk {
			return errorsx.InvalidParam("productModel is invalid for tenant/product")
		}
	}
	if productModuleID > 0 {
		module := repositories.ProductModuleRepository.Get(db, productModuleID)
		if module == nil || module.TenantID != tenant.ID || (product != nil && module.ProductID != product.ID) || module.Status != enums.StatusOk {
			return errorsx.InvalidParam("productModule is invalid for tenant/product")
		}
	}
	var device *models.Device
	if deviceID > 0 {
		device = repositories.DeviceRepository.Get(db, deviceID)
		if device == nil || device.TenantID != tenant.ID || (product != nil && device.ProductID != product.ID) || (productModel != nil && device.ProductModelID != productModel.ID) || device.Status != enums.StatusOk {
			return errorsx.InvalidParam("device is invalid for tenant/productModel")
		}
	}
	if serviceCodeID > 0 {
		serviceCode := repositories.ServiceCodeRepository.Get(db, serviceCodeID)
		if serviceCode == nil || serviceCode.TenantID != tenant.ID || (product != nil && serviceCode.ProductID != product.ID) || (productModel != nil && serviceCode.ProductModelID != productModel.ID) || (device != nil && serviceCode.DeviceID != 0 && serviceCode.DeviceID != device.ID) || !enums.IsUsableServiceCodeStatus(serviceCode.Status) {
			return errorsx.InvalidParam("serviceCode is invalid for tenant/device")
		}
	}
	if sessionID > 0 {
		session := repositories.CustomerEntrySessionRepository.Get(db, sessionID)
		if session == nil || session.TenantID != tenant.ID || (device != nil && session.DeviceID != 0 && session.DeviceID != device.ID) {
			return errorsx.InvalidParam("customerEntrySession is invalid for tenant/device")
		}
	}
	return nil
}

func updateTicketAfterSalesContextOmitted(req request.UpdateTicketRequest) bool {
	return req.TenantID == 0 &&
		req.ProductID == 0 &&
		req.ProductModelID == 0 &&
		req.ProductModuleID == 0 &&
		req.DeviceID == 0 &&
		req.ServiceCodeID == 0 &&
		req.CustomerEntrySessionID == 0 &&
		strings.TrimSpace(req.ServiceRegion) == "" &&
		strings.TrimSpace(req.FaultCode) == "" &&
		strings.TrimSpace(req.SymptomSummary) == "" &&
		strings.TrimSpace(req.DiagnosisSummary) == "" &&
		req.SLADueAt == nil &&
		req.ResolvedAt == nil
}

func normalizeTicketPriorityCode(value, fallback string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(fallback))
	}
	if value == "" {
		value = "p2"
	}
	switch value {
	case "p0", "p1", "p2", "p3", "p4":
		return value, nil
	default:
		return "", errorsx.InvalidParam("priority_code must be p0/p1/p2/p3/p4")
	}
}

func (s *ticketService) validateAssignee(userID int64) error {
	if userID <= 0 {
		return nil
	}
	return s.validateRequiredAssignee(userID)
}

func (s *ticketService) validateRequiredAssignee(userID int64) error {
	if userID <= 0 {
		return errorsx.InvalidParamI18n("error.e0334")
	}
	user := UserService.Get(userID)
	if user == nil || user.Status != enums.StatusOk {
		return errorsx.InvalidParamI18n("error.e0334")
	}
	return nil
}

func (s *ticketService) resolveConversationChannel(conversation *models.Conversation) string {
	return s.resolveConversationChannelDB(sqls.DB(), conversation)
}

func (s *ticketService) resolveConversationChannelDB(db *gorm.DB, conversation *models.Conversation) string {
	if conversation == nil || conversation.ChannelID <= 0 {
		return ""
	}
	if db == nil {
		db = sqls.DB()
	}
	if channel := repositories.ChannelRepository.Get(db, conversation.ChannelID); channel != nil {
		return channel.ChannelType
	}
	return ""
}

func normalizeInt64IDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func ticketProgressEventForStatus(status enums.TicketStatus) enums.TicketProgressEventType {
	switch enums.NormalizeTicketStatus(string(status)) {
	case enums.TicketStatusAccepted:
		return enums.TicketProgressEventAccepted
	case enums.TicketStatusPendingDispatch, enums.TicketStatusPendingAssigneeAccept:
		return enums.TicketProgressEventAssigned
	case enums.TicketStatusProcessing, enums.TicketStatusVideoSupport, enums.TicketStatusResolved, enums.TicketStatusPendingCustomerConfirm, enums.TicketStatusQualityReview:
		return enums.TicketProgressEventProcessing
	case enums.TicketStatusClosed:
		return enums.TicketProgressEventClosed
	case enums.TicketStatusReopened:
		return enums.TicketProgressEventReopened
	default:
		return enums.TicketProgressEventProgress
	}
}

func ticketProgressMetadata(fromStatus, toStatus enums.TicketStatus) string {
	payload, err := json.Marshal(map[string]string{
		"from_status": string(fromStatus),
		"to_status":   string(toStatus),
	})
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func requireTicketTenantAccess(ticket *models.Ticket, operator *dto.AuthPrincipal) error {
	if ticket == nil || operator == nil {
		return errorsx.ForbiddenI18n("error.e0225")
	}
	tenantID := operator.TenantID
	if tenantID <= 0 {
		tenantID = operator.TargetTenantID
	}
	if tenantID > 0 && ticket.TenantID != tenantID {
		return errorsx.ForbiddenI18n("error.e0225")
	}
	scope := resolveEnterpriseProductAccessScope(ticket.TenantID, operator)
	if !scope.canAccessTicket(ticket) {
		return errorsx.Forbidden("ticket is outside the engineer's product repair team")
	}
	return nil
}

func requireTicketMutationAccess(ticket *models.Ticket, operator *dto.AuthPrincipal) error {
	if err := requireTicketTenantAccess(ticket, operator); err != nil {
		return err
	}
	if operator == nil || operator.IsPlatform() || !operator.HasRole(EnterpriseRoleEngineer) || canManageTicketDispatch(operator) {
		return nil
	}
	if ticket.CurrentAssigneeID <= 0 || ticket.CurrentAssigneeID != operator.UserID {
		return errorsx.Forbidden("只有当前工单负责人可以执行此操作")
	}
	return nil
}

func enterpriseTicketViewerScope(tenantID int64, operator *dto.AuthPrincipal) (restricted bool, userID int64, teamIDs []int64) {
	scope := resolveEnterpriseProductAccessScope(tenantID, operator)
	return scope.Restricted, scope.UserID, scope.TeamIDs
}

package services

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

// workbenchOverviewCache: 工作台概览短缓存,避免高频轮询时重复执行 30+ 条 SQL。
type workbenchOverviewCacheEntry struct {
	at   time.Time
	data *dto.EnterpriseWorkbenchOverviewDTO
}

var workbenchOverviewCache sync.Map // key: "tenantID:userID"

const workbenchOverviewCacheTTL = 5 * time.Second

var EnterpriseWorkbenchService = newEnterpriseWorkbenchService()

func newEnterpriseWorkbenchService() *enterpriseWorkbenchService {
	return &enterpriseWorkbenchService{}
}

type enterpriseWorkbenchService struct{}

type EnterpriseWorkbenchQueueAggregate struct {
	Conversations          []models.Conversation
	ConversationPagination dto.EnterpriseWorkbenchPaginationDTO
	Tickets                []dto.EnterpriseTicketListItemDTO
	TicketPagination       dto.EnterpriseWorkbenchPaginationDTO
}

type EnterpriseWorkbenchQueueQuery struct {
	Page     int
	PageSize int
	Limit    int
	QueueKey string
}

type workbenchUsageByProduct struct {
	Limit     float64
	Used      float64
	Remaining float64
	Percent   float64
	Currency  string
}

func (s *enterpriseWorkbenchService) Queue(tenantID, userID int64, query EnterpriseWorkbenchQueueQuery, operator *dto.AuthPrincipal) (*EnterpriseWorkbenchQueueAggregate, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	query = normalizeEnterpriseWorkbenchQueueQuery(query)

	conversations, conversationPaging := s.visibleConversationPage(tenantID, userID, query.Page, query.PageSize, operator)
	tickets, err := EnterpriseTicketService.ListForOperator(tenantID, enterpriseWorkbenchTicketQuery(query), operator)
	if err != nil {
		return nil, err
	}
	return &EnterpriseWorkbenchQueueAggregate{
		Conversations:          conversations,
		ConversationPagination: workbenchPaginationFromPaging(conversationPaging, query.Page, query.PageSize, len(conversations)),
		Tickets:                tickets.Items,
		TicketPagination: workbenchPaginationFromList(
			tickets.Total,
			tickets.Page,
			tickets.PageSize,
			tickets.TotalPages,
			tickets.HasMore,
		),
	}, nil
}

func enterpriseWorkbenchTicketQuery(query EnterpriseWorkbenchQueueQuery) EnterpriseTicketQuery {
	ret := EnterpriseTicketQuery{
		Page:     query.Page,
		PageSize: query.PageSize,
		Sort:     "-updated_at",
		Status:   "active",
	}
	switch strings.ToLower(strings.TrimSpace(query.QueueKey)) {
	case "sla_risk":
		value := true
		ret.SLARisk = &value
	case "urgent":
		ret.Priority = "critical"
	case "unassigned":
		value := true
		ret.Unassigned = &value
	case "pending":
		ret.Status = "pending"
	case "processing":
		ret.Status = "processing"
	case "awaiting_customer":
		ret.Status = "awaiting_customer"
	}
	return ret
}

func normalizeEnterpriseWorkbenchQueueQuery(query EnterpriseWorkbenchQueueQuery) EnterpriseWorkbenchQueueQuery {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = query.Limit
	}
	if query.PageSize <= 0 {
		query.PageSize = 50
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	return query
}

func workbenchPaginationFromList(total int64, page, pageSize, totalPages int, hasMore bool) dto.EnterpriseWorkbenchPaginationDTO {
	return dto.EnterpriseWorkbenchPaginationDTO{
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
		HasMore:    hasMore,
	}
}

func workbenchPaginationFromPaging(paging *sqls.Paging, page, pageSize, fallbackCount int) dto.EnterpriseWorkbenchPaginationDTO {
	if paging == nil {
		total := int64(fallbackCount)
		totalPages := 0
		if pageSize > 0 && total > 0 {
			totalPages = int(math.Ceil(float64(total) / float64(pageSize)))
		}
		return workbenchPaginationFromList(total, page, pageSize, totalPages, page < totalPages)
	}
	totalPages := paging.TotalPage()
	return workbenchPaginationFromList(paging.Total, paging.Page, paging.Limit, totalPages, paging.Page < totalPages)
}

func (s *enterpriseWorkbenchService) Overview(ctx context.Context, tenantID, userID int64, operator *dto.AuthPrincipal) (*dto.EnterpriseWorkbenchOverviewDTO, error) {
	// 短缓存:工作台高频轮询/刷新时跳过重复的 30+ 条 SQL 计算
	if tenantID > 0 {
		cacheKey := fmt.Sprintf("%d:%d", tenantID, userID)
		if entry, ok := workbenchOverviewCache.Load(cacheKey); ok {
			if cached := entry.(*workbenchOverviewCacheEntry); time.Since(cached.at) < workbenchOverviewCacheTTL {
				return cached.data, nil
			}
		}
	}

	type parts struct {
		core         *dto.EnterpriseWorkbenchCoreDTO
		coreErr      error
		collab       *dto.EnterpriseWorkbenchCollaborationDTO
		collabErr    error
		notifs       *dto.EnterpriseWorkbenchNotificationsDTO
		notifsErr    error
		resources    *dto.EnterpriseWorkbenchResourcesDTO
		resourcesErr error
	}
	var result parts
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		result.core, result.coreErr = s.Core(tenantID, userID, operator)
	}()
	go func() {
		defer wg.Done()
		result.collab, result.collabErr = s.Collaboration(tenantID, userID, operator)
	}()
	go func() {
		defer wg.Done()
		result.notifs, result.notifsErr = s.Notifications(tenantID, userID)
	}()
	go func() {
		defer wg.Done()
		result.resources, result.resourcesErr = s.Resources(tenantID, userID, operator)
	}()
	wg.Wait()

	core := result.core
	if result.coreErr != nil {
		return nil, result.coreErr
	}

	ret := &dto.EnterpriseWorkbenchOverviewDTO{
		Queue:           core.Queue,
		Meetings:        make([]dto.EnterpriseMeetingListItemDTO, 0),
		Notifications:   make([]dto.EnterpriseNotificationDTO, 0),
		DeviceHealth:    make([]dto.EnterpriseDeviceHealthDTO, 0),
		Activity:        make([]dto.EnterpriseActivityDTO, 0),
		Scope:           core.Scope,
		Summary:         core.Summary,
		Alerts:          core.Alerts,
		QuotaAlerts:     make([]dto.EnterpriseWorkbenchQuotaAlertDTO, 0),
		Queues:          core.Queues,
		Products:        make([]dto.EnterpriseWorkbenchProductLoadDTO, 0),
		GeneratedAt:     core.GeneratedAt,
		DegradedModules: make([]string, 0),
	}

	if result.collabErr != nil {
		slog.Warn("enterprise workbench collaboration degraded", "tenant_id", tenantID, "user_id", userID, "error", result.collabErr)
		ret.DegradedModules = append(ret.DegradedModules, "collaboration")
	} else {
		ret.Meetings = result.collab.Meetings
		ret.Summary.ActiveConversations = result.collab.ActiveConversations
		ret.Summary.ActiveMeetings = result.collab.ActiveMeetings
	}

	if result.notifsErr != nil {
		slog.Warn("enterprise workbench notifications degraded", "tenant_id", tenantID, "user_id", userID, "error", result.notifsErr)
		ret.DegradedModules = append(ret.DegradedModules, "notifications")
	} else {
		ret.Notifications = result.notifs.Notifications
		ret.Summary.UnreadNotifications = result.notifs.UnreadNotifications
	}

	if result.resourcesErr != nil {
		slog.Warn("enterprise workbench resources degraded", "tenant_id", tenantID, "user_id", userID, "error", result.resourcesErr)
		ret.DegradedModules = append(ret.DegradedModules, "resources")
	} else {
		ret.DeviceHealth = result.resources.DeviceHealth
		ret.Usage = result.resources.Usage
		ret.QuotaAlerts = result.resources.QuotaAlerts
		ret.Products = result.resources.Products
		ret.Summary.TotalProducts = result.resources.TotalProducts
		ret.Summary.TotalDevices = result.resources.TotalDevices
	}

	ret.Metrics = s.metrics(ret.Summary, ret.Usage)
	ret.Alerts = s.alerts(ret.Summary, ret.Usage)
	ret.Activity = s.activity(s.visibleTicketList(tenantID, operator, false, nil, 5), ret.Notifications)

	if tenantID > 0 {
		workbenchOverviewCache.Store(fmt.Sprintf("%d:%d", tenantID, userID), &workbenchOverviewCacheEntry{at: time.Now(), data: ret})
	}
	return ret, nil
}

func (s *enterpriseWorkbenchService) Core(tenantID, userID int64, operator *dto.AuthPrincipal) (*dto.EnterpriseWorkbenchCoreDTO, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}

	scope := s.scope(tenantID, userID, operator)
	queueTickets := s.visibleTicketList(tenantID, operator, false, func(db *gorm.DB) *gorm.DB {
		return db.Where("status NOT IN ?", []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled})
	}, 8)
	summary := s.ticketSummaryFromDB(tenantID, operator)
	return &dto.EnterpriseWorkbenchCoreDTO{
		Queue:       firstTicketDTOs(queueTickets, 8),
		Scope:       scope,
		Summary:     summary,
		Alerts:      s.alerts(summary, dto.EnterpriseWorkbenchUsageDTO{}),
		Queues:      s.queueCardsFromDB(tenantID, operator),
		GeneratedAt: formatEnterpriseTime(time.Now()),
	}, nil
}

func (s *enterpriseWorkbenchService) Collaboration(tenantID, userID int64, operator *dto.AuthPrincipal) (*dto.EnterpriseWorkbenchCollaborationDTO, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}

	scope := s.scope(tenantID, userID, operator)
	meetingUserID := int64(0)
	if scope.Restricted {
		meetingUserID = userID
	}
	meetings, err := MeetingService.ListPersonalMeetings(nil, tenantID, meetingUserID, "all")
	if err != nil {
		return nil, err
	}
	conversations := s.visibleConversations(tenantID, userID, 100, operator)
	return &dto.EnterpriseWorkbenchCollaborationDTO{
		Meetings:            firstMeetings(meetings.Items, 6),
		ActiveConversations: int64(len(conversations)),
		ActiveMeetings:      meetings.Summary.Active,
	}, nil
}

func (s *enterpriseWorkbenchService) Notifications(tenantID, userID int64) (*dto.EnterpriseWorkbenchNotificationsDTO, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	notifications, err := EnterpriseNotificationService.List(tenantID, userID, EnterpriseNotificationQuery{Limit: 6})
	if err != nil {
		return nil, err
	}
	return &dto.EnterpriseWorkbenchNotificationsDTO{
		Notifications:       notifications.Items,
		UnreadNotifications: notifications.Summary.Unread,
	}, nil
}

func (s *enterpriseWorkbenchService) Resources(tenantID, userID int64, operator *dto.AuthPrincipal) (*dto.EnterpriseWorkbenchResourcesDTO, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	scope := s.scope(tenantID, userID, operator)
	var ticketProductIDs []int64
	if scope.Restricted {
		ticketProductIDs = s.visibleTicketProductIDs(tenantID, operator)
	}
	products := s.visibleProductsFromScope(tenantID, scope, ticketProductIDs)
	productIDs := productIDsFromProducts(products)
	usage, usageByProduct, quotaAlerts := s.usage(tenantID, products)
	return &dto.EnterpriseWorkbenchResourcesDTO{
		DeviceHealth:  s.deviceHealth(tenantID, productIDs, scope.Restricted),
		Usage:         usage,
		QuotaAlerts:   quotaAlerts,
		Products:      s.productLoads(tenantID, products, operator, usageByProduct),
		TotalProducts: int64(len(products)),
		TotalDevices:  s.deviceCount(tenantID, productIDs, scope.Restricted),
	}, nil
}

func (s *enterpriseWorkbenchService) scope(tenantID, userID int64, operator *dto.AuthPrincipal) dto.EnterpriseWorkbenchScopeDTO {
	scope := dto.EnterpriseWorkbenchScopeDTO{
		Mode:      "tenant",
		Label:     "租户队列",
		RoleLabel: "企业管理员",
		UserID:    userID,
	}
	restricted, viewerUserID, viewerTeamIDs := enterpriseTicketViewerScope(tenantID, operator)
	if !restricted {
		if operator != nil {
			switch {
			case operator.HasRole(EnterpriseRoleOwner):
				scope.RoleLabel = "租户所有者"
			case operator.HasRole(EnterpriseRoleServiceManager):
				scope.RoleLabel = "服务负责人"
			case operator.HasRole(EnterpriseRoleAdmin):
				scope.RoleLabel = "企业管理员"
			case operator.IsPlatform():
				scope.RoleLabel = "平台支持"
			}
		}
		return scope
	}

	scope.Restricted = true
	scope.Mode = "product_team"
	scope.RoleLabel = "服务工程师"
	scope.UserID = viewerUserID
	scope.Label = "我的产品维修组"
	scope.TeamIDs = uniqueServiceInt64s(viewerTeamIDs)
	if len(scope.TeamIDs) > 0 {
		teams := AgentTeamService.FindByIds(scope.TeamIDs)
		teamByID := make(map[int64]models.AgentTeam, len(teams))
		for _, item := range teams {
			teamByID[item.ID] = item
		}
		activeTeams := make([]models.AgentTeam, 0, len(scope.TeamIDs))
		productIDs := make([]int64, 0, len(scope.TeamIDs))
		for _, teamID := range scope.TeamIDs {
			team, ok := teamByID[teamID]
			if !ok || team.TenantID != tenantID || team.Status == enums.StatusDeleted {
				continue
			}
			activeTeams = append(activeTeams, team)
			if team.ProductID > 0 {
				productIDs = append(productIDs, team.ProductID)
			}
		}
		scope.TeamIDs = agentTeamIDs(activeTeams)
		scope.ProductIDs = uniqueServiceInt64s(productIDs)
		if len(activeTeams) == 1 {
			scope.TeamID = activeTeams[0].ID
			scope.TeamName = activeTeams[0].Name
			scope.ProductID = activeTeams[0].ProductID
			scope.Label = scope.TeamName
		} else if len(activeTeams) > 1 {
			scope.TeamID = 0
			scope.TeamName = ""
			scope.ProductID = 0
			scope.ProductName = ""
			scope.Label = fmt.Sprintf("我的 %d 个产品维修组", len(activeTeams))
		}
	}
	if scope.ProductID > 0 {
		if product := repositories.ProductRepository.GetByTenant(sqls.DB(), scope.ProductID, tenantID); product != nil {
			scope.ProductName = product.Name
		}
	}
	if len(scope.TeamIDs) == 0 && scope.TeamName == "" && scope.ProductName == "" {
		scope.Label = "我的工单"
	}
	return scope
}

func (s *enterpriseWorkbenchService) visibleTickets(tenantID int64, operator *dto.AuthPrincipal, oldestFirst bool) []models.Ticket {
	cnd := sqls.NewCnd().Eq("tenant_id", tenantID)
	scope := resolveEnterpriseProductAccessScope(tenantID, operator)
	if scope.Restricted {
		viewerTeamIDs := uniqueServiceInt64s(scope.TeamIDs)
		viewerProductIDs := uniqueServiceInt64s(scope.ProductIDs)
		if len(viewerProductIDs) > 0 && len(viewerTeamIDs) > 0 {
			cnd.Where("product_id IN ? OR current_assignee_id = ? OR (product_id = 0 AND current_team_id IN ?)", viewerProductIDs, scope.UserID, viewerTeamIDs)
		} else if len(viewerProductIDs) > 0 {
			cnd.Where("product_id IN ? OR current_assignee_id = ?", viewerProductIDs, scope.UserID)
		} else if len(viewerTeamIDs) > 0 {
			cnd.Where("current_assignee_id = ? OR (product_id = 0 AND current_team_id IN ?)", scope.UserID, viewerTeamIDs)
		} else {
			cnd.Eq("current_assignee_id", scope.UserID)
		}
	}
	if oldestFirst {
		cnd.Asc("created_at").Asc("id")
	} else {
		cnd.Desc("updated_at").Desc("id")
	}
	return repositories.TicketRepository.Find(sqls.DB(), cnd)
}

func (s *enterpriseWorkbenchService) visibleTicketDB(tenantID int64, operator *dto.AuthPrincipal) *gorm.DB {
	db := sqls.DB().Model(&models.Ticket{}).Where("tenant_id = ?", tenantID)
	scope := resolveEnterpriseProductAccessScope(tenantID, operator)
	if !scope.Restricted {
		return db
	}
	viewerTeamIDs := uniqueServiceInt64s(scope.TeamIDs)
	viewerProductIDs := uniqueServiceInt64s(scope.ProductIDs)
	switch {
	case len(viewerProductIDs) > 0 && len(viewerTeamIDs) > 0:
		db = db.Where("product_id IN ? OR current_assignee_id = ? OR (product_id = 0 AND current_team_id IN ?)", viewerProductIDs, scope.UserID, viewerTeamIDs)
	case len(viewerProductIDs) > 0:
		db = db.Where("product_id IN ? OR current_assignee_id = ?", viewerProductIDs, scope.UserID)
	case len(viewerTeamIDs) > 0:
		db = db.Where("current_assignee_id = ? OR (product_id = 0 AND current_team_id IN ?)", scope.UserID, viewerTeamIDs)
	default:
		db = db.Where("current_assignee_id = ?", scope.UserID)
	}
	return db
}

func (s *enterpriseWorkbenchService) visibleTicketList(tenantID int64, operator *dto.AuthPrincipal, oldestFirst bool, apply func(*gorm.DB) *gorm.DB, limit int) []models.Ticket {
	db := s.visibleTicketDB(tenantID, operator)
	if apply != nil {
		db = apply(db)
	}
	if oldestFirst {
		db = db.Order("created_at ASC").Order("id ASC")
	} else {
		db = db.Order("updated_at DESC").Order("id DESC")
	}
	if limit > 0 {
		db = db.Limit(limit)
	}
	var ret []models.Ticket
	if err := db.Find(&ret).Error; err != nil {
		slog.Warn("enterprise workbench ticket list degraded", "tenant_id", tenantID, "error", err)
		return make([]models.Ticket, 0)
	}
	return ret
}

func (s *enterpriseWorkbenchService) visibleTicketCount(tenantID int64, operator *dto.AuthPrincipal, apply func(*gorm.DB) *gorm.DB) int64 {
	db := s.visibleTicketDB(tenantID, operator)
	if apply != nil {
		db = apply(db)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		slog.Warn("enterprise workbench ticket count degraded", "tenant_id", tenantID, "error", err)
		return 0
	}
	return total
}

func (s *enterpriseWorkbenchService) visibleConversations(tenantID, userID int64, limit int, operator *dto.AuthPrincipal) []models.Conversation {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	cnd := s.visibleConversationCnd(tenantID, userID, operator)
	cnd.Desc("last_active_at").Desc("id").Limit(limit)
	return repositories.ConversationRepository.Find(sqls.DB(), cnd)
}

func (s *enterpriseWorkbenchService) visibleConversationPage(tenantID, userID int64, page, pageSize int, operator *dto.AuthPrincipal) ([]models.Conversation, *sqls.Paging) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}
	cnd := s.visibleConversationCnd(tenantID, userID, operator)
	cnd.Desc("last_active_at").Desc("id").Page(page, pageSize)
	return repositories.ConversationRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *enterpriseWorkbenchService) visibleConversationCnd(tenantID, userID int64, operator *dto.AuthPrincipal) *sqls.Cnd {
	cnd := sqls.NewCnd().Eq("tenant_id", tenantID)
	restricted, viewerUserID, viewerTeamIDs := enterpriseTicketViewerScope(tenantID, operator)
	if restricted {
		cnd.In("status", []enums.IMConversationStatus{
			enums.IMConversationStatusPending,
			enums.IMConversationStatusActive,
			enums.IMConversationStatusClosed,
		})
		viewerTeamIDs = uniqueServiceInt64s(viewerTeamIDs)
		if len(viewerTeamIDs) > 0 {
			cnd.Where("current_team_id IN ? OR current_assignee_id = ?", viewerTeamIDs, viewerUserID)
		} else {
			cnd.Eq("current_assignee_id", viewerUserID)
		}
	} else {
		cnd.In("status", []enums.IMConversationStatus{
			enums.IMConversationStatusAIServing,
			enums.IMConversationStatusPending,
			enums.IMConversationStatusActive,
			enums.IMConversationStatusClosed,
		})
	}
	return cnd
}

func (s *enterpriseWorkbenchService) visibleProducts(tenantID int64, scope dto.EnterpriseWorkbenchScopeDTO, tickets []models.Ticket) []models.Product {
	cnd := sqls.NewCnd().Eq("tenant_id", tenantID).NotEq("status", enums.StatusDeleted).Asc("id")
	if scope.Restricted {
		productIDs := positiveTicketProductIDs(tickets)
		if scope.ProductID > 0 {
			productIDs = append(productIDs, scope.ProductID)
		}
		productIDs = append(productIDs, scope.ProductIDs...)
		productIDs = uniqueWorkbenchInt64s(productIDs)
		if len(productIDs) == 0 {
			return make([]models.Product, 0)
		}
		cnd.In("id", productIDs)
	}
	return repositories.ProductRepository.Find(sqls.DB(), cnd)
}

func (s *enterpriseWorkbenchService) visibleTicketProductIDs(tenantID int64, operator *dto.AuthPrincipal) []int64 {
	var productIDs []int64
	err := s.visibleTicketDB(tenantID, operator).
		Where("product_id > 0").
		Distinct("product_id").
		Pluck("product_id", &productIDs).Error
	if err != nil {
		slog.Warn("enterprise workbench ticket product scope degraded", "tenant_id", tenantID, "error", err)
		return nil
	}
	return uniqueWorkbenchInt64s(productIDs)
}

func (s *enterpriseWorkbenchService) visibleProductsFromScope(tenantID int64, scope dto.EnterpriseWorkbenchScopeDTO, ticketProductIDs []int64) []models.Product {
	cnd := sqls.NewCnd().Eq("tenant_id", tenantID).NotEq("status", enums.StatusDeleted).Asc("id")
	if scope.Restricted {
		productIDs := append([]int64(nil), ticketProductIDs...)
		if scope.ProductID > 0 {
			productIDs = append(productIDs, scope.ProductID)
		}
		productIDs = append(productIDs, scope.ProductIDs...)
		productIDs = uniqueWorkbenchInt64s(productIDs)
		if len(productIDs) == 0 {
			return make([]models.Product, 0)
		}
		cnd.In("id", productIDs)
	}
	return repositories.ProductRepository.Find(sqls.DB(), cnd)
}

func (s *enterpriseWorkbenchService) ticketSummary(tickets []models.Ticket) dto.EnterpriseWorkbenchSummaryDTO {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	ret := dto.EnterpriseWorkbenchSummaryDTO{}
	for _, item := range tickets {
		done := ticketSLACompleted(item.Status)
		if !done {
			ret.OpenTickets++
		}
		if enterpriseTicketStatusInFilter(item.Status, "pending") {
			ret.PendingTickets++
		}
		if enterpriseTicketStatusInFilter(item.Status, "processing") {
			ret.ProcessingTickets++
		}
		if enterpriseTicketStatusInFilter(item.Status, "awaiting_customer") {
			ret.AwaitingCustomerTickets++
			ret.SuspendedTickets++
		}
		if enterpriseTicketUnassigned(item) {
			ret.UnassignedTickets++
		}
		if enterpriseTicketSLAAtRisk(item) {
			ret.SLARiskTickets++
		}
		if enterpriseTicketSLABreached(item) {
			ret.SLABreachedTickets++
		}
		if enterpriseWorkbenchQueueTicket(item) && DeriveTicketPriority(item) == "critical" {
			ret.UrgentTickets++
		}
		if done && ticketBusinessTime(item).After(startOfDay) {
			ret.ClosedTodayTickets++
		}
	}
	return ret
}

func (s *enterpriseWorkbenchService) ticketSummaryFromDB(tenantID int64, operator *dto.AuthPrincipal) dto.EnterpriseWorkbenchSummaryDTO {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	completed := enterpriseTicketCompletedStatuses()
	ret := dto.EnterpriseWorkbenchSummaryDTO{}
	ret.OpenTickets = s.visibleTicketCount(tenantID, operator, func(db *gorm.DB) *gorm.DB {
		return db.Where("status NOT IN ?", completed)
	})
	ret.PendingTickets = s.visibleTicketCount(tenantID, operator, func(db *gorm.DB) *gorm.DB {
		return db.Where("status IN ?", enterpriseStatusFilterToDB("pending"))
	})
	ret.ProcessingTickets = s.visibleTicketCount(tenantID, operator, func(db *gorm.DB) *gorm.DB {
		return db.Where("status IN ?", enterpriseStatusFilterToDB("processing"))
	})
	ret.AwaitingCustomerTickets = s.visibleTicketCount(tenantID, operator, func(db *gorm.DB) *gorm.DB {
		return db.Where("status IN ?", enterpriseStatusFilterToDB("awaiting_customer"))
	})
	ret.SuspendedTickets = ret.AwaitingCustomerTickets
	ret.UnassignedTickets = s.visibleTicketCount(tenantID, operator, func(db *gorm.DB) *gorm.DB {
		return db.Where("status NOT IN ? AND current_assignee_id <= 0", completed)
	})
	ret.SLARiskTickets = s.visibleTicketCount(tenantID, operator, func(db *gorm.DB) *gorm.DB {
		return db.Where("status NOT IN ? AND sla_due_at IS NOT NULL AND sla_due_at <= ?", completed, now.Add(2*time.Hour))
	})
	ret.SLABreachedTickets = s.visibleTicketCount(tenantID, operator, func(db *gorm.DB) *gorm.DB {
		return db.Where("status NOT IN ? AND sla_due_at IS NOT NULL AND sla_due_at < ?", completed, now)
	})
	ret.UrgentTickets = s.visibleTicketCount(tenantID, operator, func(db *gorm.DB) *gorm.DB {
		return db.Where("status NOT IN ? AND (LOWER(priority_code) = ? OR (sla_due_at IS NOT NULL AND sla_due_at < ?))", completed, "p0", now)
	})
	ret.ClosedTodayTickets = s.visibleTicketCount(tenantID, operator, func(db *gorm.DB) *gorm.DB {
		return db.Where("status IN ? AND COALESCE(resolved_at, updated_at, created_at) >= ?", completed, startOfDay)
	})
	return ret
}

func (s *enterpriseWorkbenchService) metrics(summary dto.EnterpriseWorkbenchSummaryDTO, usage dto.EnterpriseWorkbenchUsageDTO) []dto.EnterpriseMetricDTO {
	return []dto.EnterpriseMetricDTO{
		{
			Key:   "open_tickets",
			Label: "待处理工单",
			Value: fmt.Sprint(summary.OpenTickets),
			Meta:  fmt.Sprintf("待响应 %d · 处理中 %d", summary.PendingTickets, summary.ProcessingTickets),
			Tone:  riskTone(summary.SLARiskTickets + summary.UrgentTickets),
		},
		{
			Key:   "sla_risk",
			Label: "SLA 风险",
			Value: fmt.Sprint(summary.SLARiskTickets),
			Meta:  fmt.Sprintf("已超时 %d", summary.SLABreachedTickets),
			Tone:  slaTone(summary.SLABreachedTickets, summary.SLARiskTickets),
		},
		{
			Key:      "usage_remaining",
			Label:    "账户余额",
			Value:    formatWorkbenchAmount(usage.AccountBalance, usage.Currency),
			Meta:     usage.Meta,
			Tone:     usage.Tone,
			Progress: int(math.Round(usage.UsagePercent)),
		},
		{
			Key:   "active_work",
			Label: "现场协同",
			Value: fmt.Sprint(summary.ActiveConversations + summary.ActiveMeetings),
			Meta:  fmt.Sprintf("会话 %d · 会议 %d", summary.ActiveConversations, summary.ActiveMeetings),
			Tone:  "blue",
		},
	}
}

func (s *enterpriseWorkbenchService) alerts(summary dto.EnterpriseWorkbenchSummaryDTO, usage dto.EnterpriseWorkbenchUsageDTO) []dto.EnterpriseWorkbenchAlertDTO {
	alerts := make([]dto.EnterpriseWorkbenchAlertDTO, 0, 6)
	if summary.SLABreachedTickets > 0 {
		alerts = append(alerts, dto.EnterpriseWorkbenchAlertDTO{
			Key: "sla_breached", Title: "SLA 已超时", Description: "这些工单已经过了处理截止时间，先处理。",
			Severity: "critical", Count: summary.SLABreachedTickets, ActionLabel: "查看超时工单", ActionURL: "/enterprise/tickets?sla_breached=true",
		})
	} else if summary.SLARiskTickets > 0 {
		alerts = append(alerts, dto.EnterpriseWorkbenchAlertDTO{
			Key: "sla_risk", Title: "SLA 风险", Description: "有工单将在 2 小时内到期，先分给工程师。",
			Severity: "warning", Count: summary.SLARiskTickets, ActionLabel: "查看风险队列", ActionURL: "/enterprise/tickets?sla_risk=true",
		})
	}
	if summary.UrgentTickets > 0 {
		alerts = append(alerts, dto.EnterpriseWorkbenchAlertDTO{
			Key: "urgent", Title: "P0 紧急工单", Description: "P0 工单需要主管盯进度。",
			Severity: "critical", Count: summary.UrgentTickets, ActionLabel: "查看 P0", ActionURL: "/enterprise/tickets?priority=critical",
		})
	}
	if summary.UnassignedTickets > 0 {
		alerts = append(alerts, dto.EnterpriseWorkbenchAlertDTO{
			Key: "unassigned", Title: "待派单", Description: "还有工单没分到工程师，产品组可以先接。",
			Severity: "warning", Count: summary.UnassignedTickets, ActionLabel: "进入产品组派单", ActionURL: "/enterprise/org",
		})
	}
	if summary.SuspendedTickets > 0 {
		alerts = append(alerts, dto.EnterpriseWorkbenchAlertDTO{
			Key: "suspended", Title: "等客户确认", Description: "客户还没确认处理结果，安排人跟进。",
			Severity: "info", Count: summary.SuspendedTickets, ActionLabel: "查看挂起", ActionURL: "/enterprise/tickets?status=awaiting_customer",
		})
	}
	if usage.RiskyKeyCount > 0 {
		severity := "warning"
		description := "有产品 API Key 的额度即将用完，请及时调整额度。"
		if usage.CriticalKeyCount > 0 {
			severity = "critical"
			description = "有产品 API Key 的额度已接近耗尽，请立即处理。"
		}
		alerts = append(alerts, dto.EnterpriseWorkbenchAlertDTO{
			Key: "usage", Title: "API Key 额度预警", Description: description,
			Severity: severity, Count: usage.RiskyKeyCount, ActionLabel: "查看额度", ActionURL: "/enterprise/usage",
		})
	}
	if len(alerts) == 0 {
		alerts = append(alerts, dto.EnterpriseWorkbenchAlertDTO{
			Key: "healthy", Title: "暂无紧急告警", Description: "没有超时、待派单或用量告警。",
			Severity: "success", Count: 0, ActionLabel: "查看工单", ActionURL: "/enterprise/tickets",
		})
	}
	return alerts
}

func (s *enterpriseWorkbenchService) queueCards(tickets []models.Ticket) []dto.EnterpriseWorkbenchQueueCardDTO {
	tickets = append([]models.Ticket(nil), tickets...)
	sort.SliceStable(tickets, func(i, j int) bool {
		if tickets[i].CreatedAt.Equal(tickets[j].CreatedAt) {
			return tickets[i].ID < tickets[j].ID
		}
		return tickets[i].CreatedAt.Before(tickets[j].CreatedAt)
	})
	specs := []struct {
		key         string
		title       string
		description string
		tone        string
		actionURL   string
		match       func(models.Ticket) bool
	}{
		{key: "sla_risk", title: "SLA 风险", description: "已超时，或 2 小时内到期", tone: "red", actionURL: "/enterprise/tickets?sla_risk=true", match: enterpriseTicketSLAAtRisk},
		{key: "unassigned", title: "产品组待分配", description: "还没有指定工程师，产品组成员可见", tone: "amber", actionURL: "/enterprise/org", match: enterpriseTicketUnassigned},
		{key: "pending", title: "待响应", description: "待受理、待派单、待接单", tone: "amber", actionURL: "/enterprise/tickets?status=pending", match: func(t models.Ticket) bool { return enterpriseTicketStatusInFilter(t.Status, "pending") }},
		{key: "processing", title: "处理中", description: "工程师、视频或供应商正在处理", tone: "blue", actionURL: "/enterprise/tickets?status=processing", match: func(t models.Ticket) bool { return enterpriseTicketStatusInFilter(t.Status, "processing") }},
		{key: "awaiting_customer", title: "客户确认", description: "已解决，等客户确认或评价", tone: "green", actionURL: "/enterprise/tickets?status=awaiting_customer", match: func(t models.Ticket) bool { return enterpriseTicketStatusInFilter(t.Status, "awaiting_customer") }},
		{key: "urgent", title: "P0 紧急", description: "主管需要盯进度", tone: "red", actionURL: "/enterprise/tickets?priority=critical", match: func(t models.Ticket) bool { return DeriveTicketPriority(t) == "critical" }},
	}
	ret := make([]dto.EnterpriseWorkbenchQueueCardDTO, 0, len(specs))
	for _, spec := range specs {
		card := dto.EnterpriseWorkbenchQueueCardDTO{
			Key: spec.key, Title: spec.title, Description: spec.description, Tone: spec.tone, ActionURL: spec.actionURL,
			Items: make([]dto.EnterpriseTicketListItemDTO, 0, 4),
		}
		for _, ticket := range tickets {
			if !spec.match(ticket) {
				continue
			}
			card.Count++
			if len(card.Items) < 4 {
				card.Items = append(card.Items, EnterpriseTicketService.buildListItem(ticket))
			}
		}
		ret = append(ret, card)
	}
	return ret
}

func (s *enterpriseWorkbenchService) queueCardsFromDB(tenantID int64, operator *dto.AuthPrincipal) []dto.EnterpriseWorkbenchQueueCardDTO {
	now := time.Now()
	completed := enterpriseTicketCompletedStatuses()
	specs := []struct {
		key         string
		title       string
		description string
		tone        string
		actionURL   string
		apply       func(*gorm.DB) *gorm.DB
	}{
		{
			key: "sla_risk", title: "SLA 风险", description: "已超时，或 2 小时内到期", tone: "red", actionURL: "/enterprise/tickets?sla_risk=true",
			apply: func(db *gorm.DB) *gorm.DB {
				return db.Where("status NOT IN ? AND sla_due_at IS NOT NULL AND sla_due_at <= ?", completed, now.Add(2*time.Hour))
			},
		},
		{
			key: "unassigned", title: "产品组待分配", description: "还没有指定工程师，产品组成员可见", tone: "amber", actionURL: "/enterprise/org",
			apply: func(db *gorm.DB) *gorm.DB {
				return db.Where("status NOT IN ? AND current_assignee_id <= 0", completed)
			},
		},
		{
			key: "pending", title: "待响应", description: "待受理、待派单、待接单", tone: "amber", actionURL: "/enterprise/tickets?status=pending",
			apply: func(db *gorm.DB) *gorm.DB {
				return db.Where("status IN ?", enterpriseStatusFilterToDB("pending"))
			},
		},
		{
			key: "processing", title: "处理中", description: "工程师、视频或供应商正在处理", tone: "blue", actionURL: "/enterprise/tickets?status=processing",
			apply: func(db *gorm.DB) *gorm.DB {
				return db.Where("status IN ?", enterpriseStatusFilterToDB("processing"))
			},
		},
		{
			key: "awaiting_customer", title: "客户确认", description: "已解决，等客户确认或评价", tone: "green", actionURL: "/enterprise/tickets?status=awaiting_customer",
			apply: func(db *gorm.DB) *gorm.DB {
				return db.Where("status IN ?", enterpriseStatusFilterToDB("awaiting_customer"))
			},
		},
		{
			key: "urgent", title: "P0 紧急", description: "主管需要盯进度", tone: "red", actionURL: "/enterprise/tickets?priority=critical",
			apply: func(db *gorm.DB) *gorm.DB {
				return db.Where("status NOT IN ? AND (LOWER(priority_code) = ? OR (sla_due_at IS NOT NULL AND sla_due_at < ?))", completed, "p0", now)
			},
		},
	}
	ret := make([]dto.EnterpriseWorkbenchQueueCardDTO, 0, len(specs))
	for _, spec := range specs {
		card := dto.EnterpriseWorkbenchQueueCardDTO{
			Key: spec.key, Title: spec.title, Description: spec.description, Tone: spec.tone, ActionURL: spec.actionURL,
			Items: make([]dto.EnterpriseTicketListItemDTO, 0, 4),
		}
		card.Count = s.visibleTicketCount(tenantID, operator, spec.apply)
		tickets := s.visibleTicketList(tenantID, operator, true, spec.apply, 4)
		for _, ticket := range tickets {
			card.Items = append(card.Items, EnterpriseTicketService.buildListItem(ticket))
		}
		ret = append(ret, card)
	}
	return ret
}

func (s *enterpriseWorkbenchService) usage(tenantID int64, products []models.Product) (dto.EnterpriseWorkbenchUsageDTO, map[int64]workbenchUsageByProduct, []dto.EnterpriseWorkbenchQuotaAlertDTO) {
	ret := dto.EnterpriseWorkbenchUsageDTO{Label: "账户余额", Currency: "USD", Tone: "slate", Meta: "还没配置产品 API Key", SyncStatus: "unconfigured"}
	if account := repositories.PlatformIAMRepository.FindSub2APIAccountByTenantID(sqls.DB(), tenantID); account != nil {
		ret.AccountBalance = account.Balance
	}
	byProduct := make(map[int64]workbenchUsageByProduct)
	alerts := make([]dto.EnterpriseWorkbenchQuotaAlertDTO, 0)
	var oldestSyncedAt *time.Time
	allSnapshotsSynced := true
	productIDs := productIDsFromProducts(products)
	productIDs = uniqueWorkbenchInt64s(productIDs)
	if len(productIDs) == 0 {
		return ret, byProduct, alerts
	}
	productNames := make(map[int64]string, len(products))
	for _, product := range products {
		productNames[product.ID] = product.Name
	}
	cnd := sqls.NewCnd().Eq("tenant_id", tenantID).NotEq("status", enums.StatusDeleted)
	if len(productIDs) > 0 {
		cnd.In("product_id", productIDs)
	}
	credentials := repositories.ProductAIUsageCredentialRepository.Find(sqls.DB(), cnd)
	for _, credential := range credentials {
		currency := firstNonEmptyString(credential.Currency, "USD")
		remaining := math.Max(0, credential.QuotaLimit-credential.QuotaUsed)
		percent := workbenchPercent(credential.QuotaUsed, credential.QuotaLimit)
		byProduct[credential.ProductID] = workbenchUsageByProduct{
			Limit: credential.QuotaLimit, Used: credential.QuotaUsed, Remaining: remaining, Percent: percent, Currency: currency,
		}
		ret.QuotaLimit += credential.QuotaLimit
		ret.QuotaUsed += credential.QuotaUsed
		ret.ProductCount++
		ret.KeyCount++
		ret.Currency = currency
		if credential.LastSyncedAt == nil {
			allSnapshotsSynced = false
		} else if oldestSyncedAt == nil || credential.LastSyncedAt.Before(*oldestSyncedAt) {
			value := *credential.LastSyncedAt
			oldestSyncedAt = &value
		}
		if credential.QuotaLimit > 0 && percent >= 80 {
			ret.RiskyProductCount++
			ret.RiskyKeyCount++
			severity := "warning"
			if percent >= 95 {
				severity = "critical"
				ret.CriticalKeyCount++
			}
			lastSyncedAt := ""
			if credential.LastSyncedAt != nil {
				lastSyncedAt = formatEnterpriseTime(*credential.LastSyncedAt)
			}
			alerts = append(alerts, dto.EnterpriseWorkbenchQuotaAlertDTO{
				ProductID:      credential.ProductID,
				ProductName:    productNames[credential.ProductID],
				APIKeyID:       credential.Sub2APIKeyID,
				KeyName:        credential.KeyName,
				QuotaLimit:     credential.QuotaLimit,
				QuotaUsed:      credential.QuotaUsed,
				QuotaRemaining: remaining,
				UsagePercent:   percent,
				Currency:       currency,
				Severity:       severity,
				LastSyncedAt:   lastSyncedAt,
			})
		}
	}
	if ret.ProductCount == 0 {
		return ret, byProduct, alerts
	}
	ret.QuotaRemaining = math.Max(0, ret.QuotaLimit-ret.QuotaUsed)
	ret.UsagePercent = workbenchPercent(ret.QuotaUsed, ret.QuotaLimit)
	ret.Tone = usageTone(ret.UsagePercent, ret.QuotaLimit)
	ret.Meta = fmt.Sprintf("已用 %.0f%% · %d 个产品 Key", ret.UsagePercent, ret.KeyCount)
	ret.SyncStatus = "current"
	if oldestSyncedAt != nil {
		ret.LastSyncedAt = formatEnterpriseTime(*oldestSyncedAt)
	}
	if !allSnapshotsSynced || oldestSyncedAt == nil || time.Since(*oldestSyncedAt) >= 5*time.Minute {
		ret.SyncStatus = "stale"
	}
	sort.SliceStable(alerts, func(i, j int) bool {
		if alerts[i].UsagePercent == alerts[j].UsagePercent {
			return alerts[i].ProductID < alerts[j].ProductID
		}
		return alerts[i].UsagePercent > alerts[j].UsagePercent
	})
	if len(alerts) > 6 {
		alerts = alerts[:6]
	}
	return ret, byProduct, alerts
}

func (s *enterpriseWorkbenchService) productLoads(tenantID int64, products []models.Product, operator *dto.AuthPrincipal, usage map[int64]workbenchUsageByProduct) []dto.EnterpriseWorkbenchProductLoadDTO {
	if len(products) == 0 {
		return make([]dto.EnterpriseWorkbenchProductLoadDTO, 0)
	}
	productIDs := productIDsFromProducts(products)
	ticketLoadByProduct := s.productTicketLoads(tenantID, productIDs, operator)
	teams := repositories.AgentTeamRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		NotEq("status", enums.StatusDeleted).
		NotEq("product_id", 0))
	teamByProduct := make(map[int64]models.AgentTeam, len(teams))
	for _, team := range teams {
		if _, exists := teamByProduct[team.ProductID]; !exists {
			teamByProduct[team.ProductID] = team
		}
	}
	ret := make([]dto.EnterpriseWorkbenchProductLoadDTO, 0, len(products))
	for _, product := range products {
		row := dto.EnterpriseWorkbenchProductLoadDTO{
			ProductID: product.ID, ProductName: product.Name, Currency: "USD", Tone: "green",
		}
		if team, ok := teamByProduct[product.ID]; ok {
			row.TeamID = team.ID
			row.TeamName = team.Name
		}
		if item, ok := usage[product.ID]; ok {
			row.QuotaLimit = item.Limit
			row.QuotaUsed = item.Used
			row.QuotaRemaining = item.Remaining
			row.UsagePercent = item.Percent
			row.Currency = item.Currency
		}
		if item, ok := ticketLoadByProduct[product.ID]; ok {
			row.OpenTickets = item.OpenTickets
			row.PendingTickets = item.PendingTickets
			row.ProcessingTickets = item.ProcessingTickets
			row.UnassignedTickets = item.UnassignedTickets
			row.SLARiskTickets = item.SLARiskTickets
		}
		row.Tone = productLoadTone(row)
		ret = append(ret, row)
	}
	sort.SliceStable(ret, func(i, j int) bool {
		left := ret[i].SLARiskTickets*1000 + ret[i].PendingTickets*100 + ret[i].ProcessingTickets*10 + ret[i].OpenTickets
		right := ret[j].SLARiskTickets*1000 + ret[j].PendingTickets*100 + ret[j].ProcessingTickets*10 + ret[j].OpenTickets
		if left == right {
			return ret[i].ProductID < ret[j].ProductID
		}
		return left > right
	})
	if len(ret) > 6 {
		return ret[:6]
	}
	return ret
}

func (s *enterpriseWorkbenchService) productTicketLoads(tenantID int64, productIDs []int64, operator *dto.AuthPrincipal) map[int64]dto.EnterpriseWorkbenchProductLoadDTO {
	ret := make(map[int64]dto.EnterpriseWorkbenchProductLoadDTO)
	productIDs = uniqueWorkbenchInt64s(productIDs)
	if len(productIDs) == 0 {
		return ret
	}
	completed := enterpriseTicketCompletedStatuses()
	now := time.Now()
	type row struct {
		ProductID         int64
		OpenTickets       int64
		PendingTickets    int64
		ProcessingTickets int64
		UnassignedTickets int64
		SLARiskTickets    int64
	}
	var rows []row
	err := s.visibleTicketDB(tenantID, operator).
		Where("product_id IN ?", productIDs).
		Select(
			`product_id,
			SUM(CASE WHEN status NOT IN ? THEN 1 ELSE 0 END) AS open_tickets,
			SUM(CASE WHEN status IN ? THEN 1 ELSE 0 END) AS pending_tickets,
			SUM(CASE WHEN status IN ? THEN 1 ELSE 0 END) AS processing_tickets,
			SUM(CASE WHEN status NOT IN ? AND current_assignee_id <= 0 THEN 1 ELSE 0 END) AS unassigned_tickets,
			SUM(CASE WHEN status NOT IN ? AND sla_due_at IS NOT NULL AND sla_due_at <= ? THEN 1 ELSE 0 END) AS sla_risk_tickets`,
			completed,
			enterpriseStatusFilterToDB("pending"),
			enterpriseStatusFilterToDB("processing"),
			completed,
			completed,
			now.Add(2*time.Hour),
		).
		Group("product_id").
		Find(&rows).Error
	if err != nil {
		slog.Warn("enterprise workbench product ticket load degraded", "tenant_id", tenantID, "error", err)
		return ret
	}
	for _, item := range rows {
		ret[item.ProductID] = dto.EnterpriseWorkbenchProductLoadDTO{
			ProductID:         item.ProductID,
			OpenTickets:       item.OpenTickets,
			PendingTickets:    item.PendingTickets,
			ProcessingTickets: item.ProcessingTickets,
			UnassignedTickets: item.UnassignedTickets,
			SLARiskTickets:    item.SLARiskTickets,
		}
	}
	return ret
}

func (s *enterpriseWorkbenchService) deviceHealth(tenantID int64, productIDs []int64, restricted bool) []dto.EnterpriseDeviceHealthDTO {
	statuses := []struct {
		status string
		label  string
		tone   string
	}{
		{status: string(enums.DeviceStatusOperational), label: "运行中", tone: "green"},
		{status: string(enums.DeviceStatusUnderMaintenance), label: "维护中", tone: "amber"},
		{status: string(enums.DeviceStatusDecommissioned), label: "退役", tone: "slate"},
		{status: "new", label: "新建未绑定", tone: "blue"},
	}
	ret := make([]dto.EnterpriseDeviceHealthDTO, 0, len(statuses))
	for _, item := range statuses {
		cnd := sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Eq("device_status", item.status).
			NotEq("status", enums.StatusDeleted)
		if restricted {
			productIDs = uniqueWorkbenchInt64s(productIDs)
			if len(productIDs) == 0 {
				ret = append(ret, dto.EnterpriseDeviceHealthDTO{Status: item.status, Label: item.label, Count: 0, Tone: item.tone})
				continue
			}
			cnd.In("product_id", productIDs)
		}
		ret = append(ret, dto.EnterpriseDeviceHealthDTO{
			Status: item.status, Label: item.label, Count: repositories.DeviceRepository.Count(sqls.DB(), cnd), Tone: item.tone,
		})
	}
	return ret
}

func (s *enterpriseWorkbenchService) deviceCount(tenantID int64, productIDs []int64, restricted bool) int64 {
	cnd := sqls.NewCnd().Eq("tenant_id", tenantID).NotEq("status", enums.StatusDeleted)
	if restricted {
		productIDs = uniqueWorkbenchInt64s(productIDs)
		if len(productIDs) == 0 {
			return 0
		}
		cnd.In("product_id", productIDs)
	}
	return repositories.DeviceRepository.Count(sqls.DB(), cnd)
}

func (s *enterpriseWorkbenchService) activity(tickets []models.Ticket, notifications []dto.EnterpriseNotificationDTO) []dto.EnterpriseActivityDTO {
	ret := make([]dto.EnterpriseActivityDTO, 0, 10)
	for _, ticket := range tickets {
		ret = append(ret, dto.EnterpriseActivityDTO{
			ID:        fmt.Sprintf("ticket-%d", ticket.ID),
			Type:      "ticket",
			Title:     ticket.Title,
			Meta:      ticket.TicketNo,
			Tone:      ticketTone(ticket.Status),
			CreatedAt: formatEnterpriseTime(ticketBusinessTime(ticket)),
		})
		if len(ret) >= 5 {
			break
		}
	}
	for _, item := range notifications {
		ret = append(ret, dto.EnterpriseActivityDTO{
			ID:        fmt.Sprintf("notification-%d", item.ID),
			Type:      "notification",
			Title:     item.Title,
			Meta:      notificationActivityMeta(item),
			Tone:      "blue",
			CreatedAt: item.CreatedAt,
		})
		if len(ret) >= 10 {
			break
		}
	}
	return ret
}

func notificationActivityMeta(item dto.EnterpriseNotificationDTO) string {
	if item.BizID > 0 {
		switch item.BizType {
		case "ticket":
			return fmt.Sprintf("工单 #%d", item.BizID)
		case "conversation":
			return fmt.Sprintf("会话 #%d", item.BizID)
		case "meeting":
			return fmt.Sprintf("会议 #%d", item.BizID)
		}
	}
	switch item.BizType {
	case "ticket":
		return "工单"
	case "conversation":
		return "会话"
	case "meeting":
		return "视频协作"
	case "sla":
		return "SLA"
	case "approval":
		return "审批"
	case "quota", "usage":
		return "用量"
	case "knowledge":
		return "知识库"
	case "ai", "ai_agent":
		return "AI 机器人"
	case "data_breach":
		return "合规告警"
	case "dsar":
		return "数据请求"
	default:
		if item.Category != "" {
			return notificationCategoryLabel(item.Category)
		}
		return "通知"
	}
}

func notificationCategoryLabel(category string) string {
	switch category {
	case "ticket":
		return "工单"
	case "sla":
		return "SLA"
	case "approval":
		return "审批"
	case "quota", "usage":
		return "用量"
	case "meeting":
		return "视频协作"
	case "knowledge":
		return "知识库"
	case "system":
		return "系统"
	default:
		return "通知"
	}
}

func firstMeetings(items []dto.EnterpriseMeetingListItemDTO, limit int) []dto.EnterpriseMeetingListItemDTO {
	if limit <= 0 || len(items) == 0 {
		return make([]dto.EnterpriseMeetingListItemDTO, 0)
	}
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func firstTicketDTOs(items []models.Ticket, limit int) []dto.EnterpriseTicketListItemDTO {
	if limit <= 0 {
		return make([]dto.EnterpriseTicketListItemDTO, 0)
	}
	if len(items) < limit {
		limit = len(items)
	}
	ret := make([]dto.EnterpriseTicketListItemDTO, 0, limit)
	for _, item := range items[:limit] {
		ret = append(ret, EnterpriseTicketService.buildListItem(item))
	}
	return ret
}

func enterpriseWorkbenchQueueTickets(items []models.Ticket) []models.Ticket {
	ret := make([]models.Ticket, 0, len(items))
	for _, item := range items {
		if enterpriseWorkbenchQueueTicket(item) {
			ret = append(ret, item)
		}
	}
	return ret
}

func enterpriseWorkbenchQueueTicket(ticket models.Ticket) bool {
	switch enums.NormalizeTicketStatus(string(ticket.Status)) {
	case enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled:
		return false
	default:
		return true
	}
}

func productIDsFromProducts(products []models.Product) []int64 {
	ret := make([]int64, 0, len(products))
	for _, product := range products {
		if product.ID > 0 {
			ret = append(ret, product.ID)
		}
	}
	return uniqueWorkbenchInt64s(ret)
}

func agentTeamIDs(teams []models.AgentTeam) []int64 {
	ret := make([]int64, 0, len(teams))
	for _, team := range teams {
		if team.ID > 0 {
			ret = append(ret, team.ID)
		}
	}
	return uniqueWorkbenchInt64s(ret)
}

func positiveTicketProductIDs(tickets []models.Ticket) []int64 {
	ret := make([]int64, 0, len(tickets))
	for _, ticket := range tickets {
		if ticket.ProductID > 0 {
			ret = append(ret, ticket.ProductID)
		}
	}
	return uniqueWorkbenchInt64s(ret)
}

func uniqueWorkbenchInt64s(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int64]bool, len(values))
	ret := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		ret = append(ret, value)
	}
	return ret
}

func ticketBusinessTime(ticket models.Ticket) time.Time {
	if ticket.ResolvedAt != nil && !ticket.ResolvedAt.IsZero() {
		return *ticket.ResolvedAt
	}
	if !ticket.UpdatedAt.IsZero() {
		return ticket.UpdatedAt
	}
	return ticket.CreatedAt
}

func workbenchPercent(used, limit float64) float64 {
	if limit <= 0 {
		return 0
	}
	return math.Max(0, math.Min(100, used/limit*100))
}

func formatWorkbenchAmount(value float64, currency string) string {
	if currency == "" {
		currency = "USD"
	}
	if value >= 1000 {
		return fmt.Sprintf("%.0f %s", value, currency)
	}
	return fmt.Sprintf("%.2f %s", value, currency)
}

func usageTone(percent float64, limit float64) string {
	if limit <= 0 {
		return "slate"
	}
	if percent >= 95 {
		return "red"
	}
	if percent >= 80 {
		return "amber"
	}
	return "green"
}

func productLoadTone(item dto.EnterpriseWorkbenchProductLoadDTO) string {
	if item.SLARiskTickets > 0 || item.UsagePercent >= 95 {
		return "red"
	}
	if item.UnassignedTickets > 0 || item.PendingTickets > 0 || item.UsagePercent >= 80 {
		return "amber"
	}
	if item.ProcessingTickets > 0 || item.OpenTickets > 0 {
		return "blue"
	}
	return "green"
}

func enterpriseTicketUnassigned(ticket models.Ticket) bool {
	return !ticketSLACompleted(ticket.Status) && ticket.CurrentAssigneeID <= 0
}

func slaTone(breached, risk int64) string {
	if breached > 0 {
		return "red"
	}
	if risk > 0 {
		return "amber"
	}
	return "green"
}

func riskTone(value int64) string {
	if value > 0 {
		return "amber"
	}
	return "green"
}

func ticketTone(status enums.TicketStatus) string {
	switch status {
	case enums.TicketStatusClosed, enums.TicketStatusDone:
		return "green"
	case enums.TicketStatusEscalated, enums.TicketStatusVideoSupport:
		return "red"
	case enums.TicketStatusWaitingCustomer, enums.TicketStatusPendingCustomerConfirm:
		return "amber"
	default:
		return "blue"
	}
}

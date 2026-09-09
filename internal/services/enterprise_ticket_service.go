package services

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var EnterpriseTicketService = newEnterpriseTicketService()

func newEnterpriseTicketService() *enterpriseTicketService {
	return &enterpriseTicketService{}
}

type enterpriseTicketService struct{}

type EnterpriseTicketQuery struct {
	Page             int
	PageSize         int
	Status           string
	Priority         string
	Search           string
	Sort             string
	ConversationID   int64
	ProductID        int64
	DeviceID         int64
	TeamID           int64
	SLABreached      *bool
	SLARisk          *bool
	Unassigned       *bool
	Mine             bool
	ViewerUserID     int64
	ViewerTeamID     int64
	ViewerTeamIDs    []int64
	ViewerProductIDs []int64
	RestrictViewer   bool
}

func normalizeEnterprisePage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func (s *enterpriseTicketService) List(tenantID int64, query EnterpriseTicketQuery) (*dto.EnterpriseListResponse[dto.EnterpriseTicketListItemDTO], error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	if query.Mine && query.ViewerUserID <= 0 {
		return nil, errorsx.InvalidParam("current enterprise user is required for personal ticket filtering")
	}
	query.Page, query.PageSize = normalizeEnterprisePage(query.Page, query.PageSize)
	cnd := enterpriseTicketBaseCnd(tenantID, query)
	if statuses := enterpriseStatusFilterToDB(query.Status); len(statuses) > 0 {
		cnd.In("status", statuses)
	}
	now := time.Now()
	applyEnterpriseTicketPriorityCnd(cnd, query.Priority, now)
	applyEnterpriseTicketSLACnd(cnd, query.SLABreached, query.SLARisk, now)
	if query.Unassigned != nil {
		if *query.Unassigned {
			cnd.Eq("current_assignee_id", 0)
		} else {
			cnd.NotEq("current_assignee_id", 0)
		}
	}
	switch query.Sort {
	case "created_at":
		cnd.Asc("created_at")
		cnd.Asc("id")
	case "updated_at":
		cnd.Asc("updated_at")
		cnd.Asc("id")
	case "-updated_at":
		cnd.Desc("updated_at")
		cnd.Desc("id")
	case "-id":
		cnd.Desc("id")
	case "id":
		cnd.Asc("id")
	default:
		cnd.Desc("created_at")
		cnd.Desc("id")
	}
	cnd.Page(query.Page, query.PageSize)

	tickets, paging := repositories.TicketRepository.FindPageByCnd(sqls.DB(), cnd)
	rows := make([]dto.EnterpriseTicketListItemDTO, 0, len(tickets))
	for _, item := range tickets {
		rows = append(rows, s.buildListItem(item))
	}
	if paging == nil {
		paging = &sqls.Paging{Page: query.Page, Limit: query.PageSize, Total: int64(len(rows))}
	}
	totalPages := paging.TotalPage()
	return &dto.EnterpriseListResponse[dto.EnterpriseTicketListItemDTO]{
		Items:      rows,
		Total:      paging.Total,
		Page:       paging.Page,
		PageSize:   paging.Limit,
		TotalPages: totalPages,
		HasMore:    paging.Page < totalPages,
	}, nil
}

func (s *enterpriseTicketService) Summary(tenantID int64, query EnterpriseTicketQuery) (*dto.EnterpriseTicketSummaryDTO, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	if query.Mine && query.ViewerUserID <= 0 {
		return nil, errorsx.InvalidParam("current enterprise user is required for personal ticket filtering")
	}
	now := time.Now()
	baseQuery := query
	ret := &dto.EnterpriseTicketSummaryDTO{GeneratedAt: formatEnterpriseTime(now)}
	ret.Total = repositories.TicketRepository.Count(sqls.DB(), enterpriseTicketBaseCnd(tenantID, baseQuery))
	ret.Pending = repositories.TicketRepository.Count(sqls.DB(), enterpriseTicketStatusCnd(tenantID, baseQuery, "pending"))
	ret.Processing = repositories.TicketRepository.Count(sqls.DB(), enterpriseTicketStatusCnd(tenantID, baseQuery, "processing"))
	ret.AwaitingCustomer = repositories.TicketRepository.Count(sqls.DB(), enterpriseTicketStatusCnd(tenantID, baseQuery, "awaiting_customer"))
	ret.Done = repositories.TicketRepository.Count(sqls.DB(), enterpriseTicketStatusCnd(tenantID, baseQuery, "done"))
	slaRisk := true
	ret.SLARisk = repositories.TicketRepository.Count(sqls.DB(), enterpriseTicketSLASummaryCnd(tenantID, baseQuery, nil, &slaRisk, now))
	urgent := enterpriseTicketBaseCnd(tenantID, baseQuery)
	applyEnterpriseTicketPriorityCnd(urgent, "critical", now)
	ret.Urgent = repositories.TicketRepository.Count(sqls.DB(), urgent)
	return ret, nil
}

func (s *enterpriseTicketService) ListCustomerOptions(tenantID int64, search string, limit int) ([]dto.EnterpriseTicketCustomerOptionDTO, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	items, err := repositories.CustomerRepository.FindTenantCustomerOptions(sqls.DB(), tenantID, search, limit)
	if err != nil {
		return nil, err
	}
	result := make([]dto.EnterpriseTicketCustomerOptionDTO, 0, len(items))
	for _, item := range items {
		result = append(result, dto.EnterpriseTicketCustomerOptionDTO{
			CustomerID: item.CustomerID, CustomerUserID: item.CustomerUserID,
			CustomerOrgID: item.CustomerOrgID, CustomerOrgName: item.CustomerOrgName,
			DisplayName: item.DisplayName, Email: item.Email, Phone: item.Phone,
		})
	}
	return result, nil
}

func enterpriseTicketBaseCnd(tenantID int64, query EnterpriseTicketQuery) *sqls.Cnd {
	cnd := sqls.NewCnd().Eq("tenant_id", tenantID)
	if query.RestrictViewer {
		viewerTeamIDs := enterpriseViewerTeamIDs(query)
		viewerProductIDs := uniqueServiceInt64s(query.ViewerProductIDs)
		if len(viewerProductIDs) > 0 && len(viewerTeamIDs) > 0 {
			cnd.Where("(product_id IN ? OR current_assignee_id = ? OR (product_id = 0 AND current_team_id IN ?))", viewerProductIDs, query.ViewerUserID, viewerTeamIDs)
		} else if len(viewerProductIDs) > 0 {
			cnd.Where("(product_id IN ? OR current_assignee_id = ?)", viewerProductIDs, query.ViewerUserID)
		} else if len(viewerTeamIDs) > 0 {
			cnd.Where("(current_assignee_id = ? OR (product_id = 0 AND current_team_id IN ?))", query.ViewerUserID, viewerTeamIDs)
		} else {
			cnd.Eq("current_assignee_id", query.ViewerUserID)
		}
	}
	if query.Mine {
		cnd.Eq("current_assignee_id", query.ViewerUserID)
	}
	if query.ConversationID > 0 {
		cnd.Eq("conversation_id", query.ConversationID)
	}
	if query.ProductID > 0 {
		cnd.Eq("product_id", query.ProductID)
	}
	if query.DeviceID > 0 {
		cnd.Eq("device_id", query.DeviceID)
	}
	if query.TeamID > 0 {
		cnd.Eq("current_team_id", query.TeamID)
	}
	applyEnterpriseTicketSearchCnd(cnd, query.Search)
	return cnd
}

func enterpriseTicketStatusCnd(tenantID int64, query EnterpriseTicketQuery, status string) *sqls.Cnd {
	cnd := enterpriseTicketBaseCnd(tenantID, query)
	if statuses := enterpriseStatusFilterToDB(status); len(statuses) > 0 {
		cnd.In("status", statuses)
	}
	return cnd
}

func enterpriseTicketSLASummaryCnd(tenantID int64, query EnterpriseTicketQuery, breached, risk *bool, now time.Time) *sqls.Cnd {
	cnd := enterpriseTicketBaseCnd(tenantID, query)
	applyEnterpriseTicketSLACnd(cnd, breached, risk, now)
	return cnd
}

func applyEnterpriseTicketSearchCnd(cnd *sqls.Cnd, search string) {
	search = strings.ToLower(strings.TrimSpace(search))
	if cnd == nil || search == "" {
		return
	}
	pattern := "%" + search + "%"
	db := sqls.DB()
	if db == nil {
		cnd.Where("(LOWER(source_record_id) LIKE ? OR LOWER(ticket_no) LIKE ? OR LOWER(title) LIKE ? OR LOWER(description) LIKE ? OR LOWER(fault_code) LIKE ? OR LOWER(symptom_summary) LIKE ? OR LOWER(diagnosis_summary) LIKE ? OR LOWER(service_region) LIKE ?)",
			pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern)
		return
	}
	ticketTenant := enterpriseTicketColumnRef("tenant_id")
	productMatch := db.Model(&models.Product{}).
		Select("1").
		Where("id = "+enterpriseTicketColumnRef("product_id")).
		Where("tenant_id = "+ticketTenant).
		Where("(LOWER(code) LIKE ? OR LOWER(name) LIKE ? OR LOWER(category) LIKE ? OR LOWER(description) LIKE ?)", pattern, pattern, pattern, pattern)
	deviceMatch := db.Model(&models.Device{}).
		Select("1").
		Where("id = "+enterpriseTicketColumnRef("device_id")).
		Where("tenant_id = "+ticketTenant).
		Where("(LOWER(device_no) LIKE ? OR LOWER(serial_no) LIKE ? OR LOWER(external_device_id) LIKE ? OR LOWER(region_code) LIKE ?)", pattern, pattern, pattern, pattern)
	customerMatch := db.Model(&models.Customer{}).
		Select("1").
		Where("id = "+enterpriseTicketColumnRef("customer_id")).
		Where("(LOWER(name) LIKE ? OR LOWER(primary_mobile) LIKE ? OR LOWER(primary_email) LIKE ? OR LOWER(remark) LIKE ?)", pattern, pattern, pattern, pattern)
	invitationMatch := db.Model(&models.CustomerRegistrationGrant{}).
		Select("1").
		Where("id = "+enterpriseTicketColumnRef("customer_registration_grant_id")).
		Where("tenant_id = "+ticketTenant).
		Where("(LOWER(display_name) LIKE ? OR LOWER(email) LIKE ?)", pattern, pattern)
	assigneeMatch := db.Model(&models.User{}).
		Select("1").
		Where("id = "+enterpriseTicketColumnRef("current_assignee_id")).
		Where("(LOWER(username) LIKE ? OR LOWER(nickname) LIKE ? OR LOWER(email) LIKE ? OR LOWER(mobile) LIKE ?)", pattern, pattern, pattern, pattern)
	cnd.Where(`(
		LOWER(source_record_id) LIKE ? OR LOWER(ticket_no) LIKE ? OR LOWER(title) LIKE ? OR LOWER(description) LIKE ? OR
		LOWER(fault_code) LIKE ? OR LOWER(symptom_summary) LIKE ? OR LOWER(diagnosis_summary) LIKE ? OR LOWER(service_region) LIKE ? OR
		EXISTS (?) OR EXISTS (?) OR EXISTS (?) OR EXISTS (?) OR EXISTS (?)
	)`, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern, productMatch, deviceMatch, customerMatch, invitationMatch, assigneeMatch)
}

func enterpriseTicketColumnRef(column string) string {
	db := sqls.DB()
	if db == nil {
		return "tickets." + column
	}
	return db.NamingStrategy.TableName("Ticket") + "." + column
}

func applyEnterpriseTicketPriorityCnd(cnd *sqls.Cnd, priority string, now time.Time) {
	if cnd == nil {
		return
	}
	switch enterpriseTicketNormalizePriorityFilter(priority) {
	case "":
		return
	case "critical":
		cnd.Where("status NOT IN ? AND (LOWER(TRIM(priority_code)) = ? OR (LOWER(TRIM(priority_code)) NOT IN ? AND sla_due_at IS NOT NULL AND sla_due_at < ?))",
			enterpriseTicketCompletedStatuses(), "p0", enterpriseTicketKnownPriorityCodes(), now)
	case "high":
		cnd.Where("status NOT IN ? AND (LOWER(TRIM(priority_code)) = ? OR (LOWER(TRIM(priority_code)) NOT IN ? AND status = ?))",
			enterpriseTicketCompletedStatuses(), "p1", enterpriseTicketKnownPriorityCodes(), enums.TicketStatusEscalated)
	case "medium":
		cnd.Where("status NOT IN ? AND (LOWER(TRIM(priority_code)) = ? OR (LOWER(TRIM(priority_code)) NOT IN ? AND status <> ? AND sla_due_at IS NOT NULL AND sla_due_at >= ? AND sla_due_at <= ?))",
			enterpriseTicketCompletedStatuses(), "p2", enterpriseTicketKnownPriorityCodes(), enums.TicketStatusEscalated, now, now.Add(24*time.Hour))
	case "low":
		cnd.Where("(status IN ? OR (status NOT IN ? AND (LOWER(TRIM(priority_code)) IN ? OR (LOWER(TRIM(priority_code)) NOT IN ? AND status <> ? AND (sla_due_at IS NULL OR sla_due_at > ?)))))",
			enterpriseTicketCompletedStatuses(), enterpriseTicketCompletedStatuses(), []string{"p3", "p4"}, enterpriseTicketKnownPriorityCodes(), enums.TicketStatusEscalated, now.Add(24*time.Hour))
	}
}

func enterpriseTicketNormalizePriorityFilter(priority string) string {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "", "all":
		return ""
	case "critical", "p0":
		return "critical"
	case "high", "p1":
		return "high"
	case "medium", "p2":
		return "medium"
	case "low", "p3", "p4":
		return "low"
	default:
		return strings.ToLower(strings.TrimSpace(priority))
	}
}

func enterpriseTicketKnownPriorityCodes() []string {
	return []string{"p0", "p1", "p2", "p3", "p4"}
}

func enterpriseTicketCompletedStatuses() []enums.TicketStatus {
	return []enums.TicketStatus{
		enums.TicketStatusWaitingCustomer,
		enums.TicketStatusResolved,
		enums.TicketStatusPendingCustomerConfirm,
		enums.TicketStatusClosed,
		enums.TicketStatusCancelled,
		enums.TicketStatusDone,
	}
}

func applyEnterpriseTicketSLACnd(cnd *sqls.Cnd, breached, risk *bool, now time.Time) {
	if cnd == nil {
		return
	}
	if breached != nil {
		if *breached {
			cnd.Where("status NOT IN ? AND sla_due_at IS NOT NULL AND sla_due_at < ?", enterpriseTicketCompletedStatuses(), now)
		} else {
			cnd.Where("(status IN ? OR sla_due_at IS NULL OR sla_due_at >= ?)", enterpriseTicketCompletedStatuses(), now)
		}
	}
	if risk != nil {
		if *risk {
			cnd.Where("status NOT IN ? AND sla_due_at IS NOT NULL AND sla_due_at <= ?", enterpriseTicketCompletedStatuses(), now.Add(2*time.Hour))
		} else {
			cnd.Where("(status IN ? OR sla_due_at IS NULL OR sla_due_at > ?)", enterpriseTicketCompletedStatuses(), now.Add(2*time.Hour))
		}
	}
}

func (s *enterpriseTicketService) SummaryForOperator(tenantID int64, query EnterpriseTicketQuery, operator *dto.AuthPrincipal) (*dto.EnterpriseTicketSummaryDTO, error) {
	scope := resolveEnterpriseProductAccessScope(tenantID, operator)
	query.RestrictViewer, query.ViewerUserID, query.ViewerTeamIDs, query.ViewerProductIDs = scope.Restricted, scope.UserID, scope.TeamIDs, scope.ProductIDs
	if operator != nil && operator.UserID > 0 {
		query.ViewerUserID = operator.UserID
	}
	if len(query.ViewerTeamIDs) > 0 {
		query.ViewerTeamID = query.ViewerTeamIDs[0]
	}
	return s.Summary(tenantID, query)
}

func enterpriseTicketStatusInFilter(status enums.TicketStatus, filter string) bool {
	for _, item := range enterpriseStatusFilterToDB(filter) {
		if status == item {
			return true
		}
	}
	return false
}

func enterpriseTicketSLABreached(ticket models.Ticket) bool {
	deadline, ok := ticketSLADeadline(ticket)
	if !ok {
		return false
	}
	return time.Now().After(deadline) && !ticketSLACompleted(ticket.Status)
}

func enterpriseTicketSLAAtRisk(ticket models.Ticket) bool {
	if ticketSLACompleted(ticket.Status) {
		return false
	}
	deadline, ok := ticketSLADeadline(ticket)
	if !ok {
		return false
	}
	return !deadline.After(time.Now().Add(2 * time.Hour))
}

func (s *enterpriseTicketService) ListForOperator(tenantID int64, query EnterpriseTicketQuery, operator *dto.AuthPrincipal) (*dto.EnterpriseListResponse[dto.EnterpriseTicketListItemDTO], error) {
	scope := resolveEnterpriseProductAccessScope(tenantID, operator)
	query.RestrictViewer, query.ViewerUserID, query.ViewerTeamIDs, query.ViewerProductIDs = scope.Restricted, scope.UserID, scope.TeamIDs, scope.ProductIDs
	if operator != nil && operator.UserID > 0 {
		query.ViewerUserID = operator.UserID
	}
	if len(query.ViewerTeamIDs) > 0 {
		query.ViewerTeamID = query.ViewerTeamIDs[0]
	}
	result, err := s.List(tenantID, query)
	if err != nil || result == nil {
		return result, err
	}
	for index := range result.Items {
		ticket := repositories.TicketRepository.Get(sqls.DB(), result.Items[index].ID)
		if ticket == nil {
			continue
		}
		actions := s.BuildActionsForOperator(ticket, operator)
		result.Items[index].Actions = &actions
	}
	return result, nil
}

func enterpriseViewerTeamIDs(query EnterpriseTicketQuery) []int64 {
	teamIDs := uniqueServiceInt64s(query.ViewerTeamIDs)
	if len(teamIDs) == 0 && query.ViewerTeamID > 0 {
		teamIDs = []int64{query.ViewerTeamID}
	}
	return teamIDs
}

func (s *enterpriseTicketService) GetAggregate(tenantID int64, ticketID int64) (*dto.TicketAggregateDTO, error) {
	if tenantID <= 0 || ticketID <= 0 {
		return nil, errorsx.InvalidParam("tenant and ticket are required")
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil || ticket.TenantID != tenantID {
		return nil, errorsx.InvalidParam("ticket not found")
	}
	deviceNo, deviceContext := s.buildDeviceContext(ticket)
	repair := s.buildRepair(ticket, deviceNo)
	return &dto.TicketAggregateDTO{
		Ticket:               s.buildHeader(ticket),
		Customer:             s.buildCustomer(ticket),
		DeviceContext:        deviceContext,
		ConversationSnapshot: s.buildConversationSnapshot(ticket),
		DiagnosisSnapshot:    s.buildDiagnosisSnapshot(ticket),
		Flow:                 s.buildFlow(ticket),
		Assignment:           s.buildAssignment(ticket),
		Meeting:              s.buildMeeting(ticket),
		Repair:               repair,
		Feedback:             s.buildFeedback(ticket),
		Timeline:             s.buildTimeline(ticket),
		Assets:               s.buildAssets(ticket),
		AuditRefs:            s.buildAuditRefs(ticket),
		Actions:              s.BuildActions(ticket),
	}, nil
}

func (s *enterpriseTicketService) GetAggregateForOperator(tenantID, ticketID int64, operator *dto.AuthPrincipal) (*dto.TicketAggregateDTO, error) {
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil || ticket.TenantID != tenantID {
		return nil, errorsx.InvalidParam("ticket not found")
	}
	if err := requireTicketTenantAccess(ticket, operator); err != nil {
		return nil, err
	}
	result, err := s.GetAggregate(tenantID, ticketID)
	if err != nil {
		return nil, err
	}
	result.Actions = s.BuildActionsForOperator(ticket, operator)
	return result, nil
}

func (s *enterpriseTicketService) BuildActions(ticket *models.Ticket) dto.TicketActionPermissionsDTO {
	if ticket == nil {
		return dto.TicketActionPermissionsDTO{}
	}
	status := enums.NormalizeTicketStatus(string(ticket.Status))
	if status == enums.TicketStatusDraft {
		return dto.TicketActionPermissionsDTO{CanCancel: true}
	}
	done := status == enums.TicketStatusClosed || status == enums.TicketStatusCancelled
	awaitingCustomer := status == enums.TicketStatusResolved || status == enums.TicketStatusPendingCustomerConfirm
	supplierEscalatable := status == enums.TicketStatusAccepted || status == enums.TicketStatusProcessing || status == enums.TicketStatusVideoSupport || status == enums.TicketStatusSupplierSupport
	return dto.TicketActionPermissionsDTO{
		CanAccept:                   status == enums.TicketStatusPendingAcceptance || status == enums.TicketStatusPendingDispatch || status == enums.TicketStatusPendingAssigneeAccept || status == enums.TicketStatusReopened,
		CanAssign:                   !done && !awaitingCustomer,
		CanTransfer:                 !done && !awaitingCustomer && ticket.CurrentAssigneeID > 0,
		CanCancel:                   !done && !awaitingCustomer,
		CanEscalateSupplier:         supplierEscalatable,
		CanStartMeeting:             canStartMeetingFromStatus(ticket.Status),
		CanSaveRepair:               canCompleteRepairFromStatus(ticket.Status),
		CanClose:                    !done,
		CanReopen:                   done || awaitingCustomer,
		CanCreateKnowledgeCandidate: done && TenantCapabilityService.AIEnabled(ticket.TenantID),
	}
}

func (s *enterpriseTicketService) BuildActionsForOperator(ticket *models.Ticket, operator *dto.AuthPrincipal) dto.TicketActionPermissionsDTO {
	actions := s.BuildActions(ticket)
	if ticket == nil || operator == nil {
		return dto.TicketActionPermissionsDTO{}
	}
	isAssignee := ticket.CurrentAssigneeID > 0 && ticket.CurrentAssigneeID == operator.UserID
	actions.CanTakeover = canOperatorTakeOverTicketNow(ticket, operator)
	if canManageTicketDispatch(operator) {
		actions.CanAccept = actions.CanAccept && canManagerAcceptTicketNow(ticket, operator)
		actions.CanStartMeeting = canManagerOperateOpenTicket(ticket.Status)
		actions.CanEndMeeting = canManagerOperateOpenTicket(ticket.Status)
		actions.CanSaveRepair = canManagerOperateOpenTicket(ticket.Status)
		return actions
	}
	actions.CanStartMeeting = actions.CanStartMeeting && isAssignee
	actions.CanEndMeeting = canOperatorEndTicketMeetingNow(ticket, operator)
	if !operator.HasRole(EnterpriseRoleEngineer) {
		actions.CanAccept = actions.CanAccept && canOperatorAcceptTicketNow(ticket, operator)
		return actions
	}

	actions.CanAccept = actions.CanAccept && (ticket.CurrentAssigneeID == 0 || isAssignee)
	if actions.CanAccept {
		actions.CanAccept = canOperatorAcceptTicketNow(ticket, operator)
	}
	actions.CanAssign = false
	actions.CanTransfer = false
	if !isAssignee {
		actions.CanCancel = false
		actions.CanEscalateSupplier = false
		actions.CanStartMeeting = false
		actions.CanSaveRepair = false
		actions.CanClose = false
		actions.CanReopen = false
		actions.CanCreateKnowledgeCandidate = false
	}
	return actions
}

func canOperatorEndTicketMeetingNow(ticket *models.Ticket, operator *dto.AuthPrincipal) bool {
	if ticket == nil || operator == nil || operator.UserID <= 0 {
		return false
	}
	if err := requireTicketTenantAccess(ticket, operator); err != nil {
		return false
	}
	if canManageTicketDispatch(operator) {
		return canManagerOperateOpenTicket(ticket.Status)
	}
	if ticket.CurrentAssigneeID > 0 && ticket.CurrentAssigneeID == operator.UserID {
		return true
	}
	if !operator.HasRole(EnterpriseRoleEngineer) {
		return false
	}
	teamID := ticket.CurrentTeamID
	if teamID <= 0 && ticket.ProductID > 0 {
		if team := ProductSupportOrganizationService.FindProductRepairTeam(sqls.DB(), ticket.TenantID, ticket.ProductID); team != nil {
			teamID = team.ID
		}
	}
	return teamID > 0 && AgentTeamMemberService.IsUserActiveMemberOfTeamDB(sqls.DB(), ticket.TenantID, teamID, operator.UserID)
}

func canManagerAcceptTicketNow(ticket *models.Ticket, operator *dto.AuthPrincipal) bool {
	if ticket == nil || operator == nil || operator.UserID <= 0 || !canManageTicketDispatch(operator) {
		return false
	}
	_, ok := ticketAcceptTargetStatus(ticket)
	return ok
}

func canManagerOperateOpenTicket(status enums.TicketStatus) bool {
	switch enums.NormalizeTicketStatus(string(status)) {
	case enums.TicketStatusDraft, enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled:
		return false
	default:
		return true
	}
}

func canOperatorAcceptTicketNow(ticket *models.Ticket, operator *dto.AuthPrincipal) bool {
	if ticket == nil || operator == nil || operator.UserID <= 0 {
		return false
	}
	if ticket.CurrentAssigneeID > 0 && ticket.CurrentAssigneeID != operator.UserID {
		return false
	}
	now := time.Now()
	teamID := ticket.CurrentTeamID
	if teamID <= 0 && ticket.ProductID > 0 {
		if team := ProductSupportOrganizationService.FindProductRepairTeam(sqls.DB(), ticket.TenantID, ticket.ProductID); team != nil {
			teamID = team.ID
		}
	}
	if ticket.CurrentAssigneeID == operator.UserID {
		return validateAssignedTicketAcceptanceDB(sqls.DB(), ticket, operator.UserID, teamID, now) == nil
	}
	assignmentTicket := *ticket
	assignmentTicket.CurrentTeamID = teamID
	_, _, _, err := validateManualTicketSelfClaimWithOptionsDB(
		sqls.DB(),
		&assignmentTicket,
		operator.UserID,
		now,
		manualTicketAssigneeOptions{
			AllowNoActiveSchedule: true,
			AllowRequestPresence:  true,
		},
	)
	return err == nil
}

func canOperatorTakeOverTicketNow(ticket *models.Ticket, operator *dto.AuthPrincipal) bool {
	if ticket == nil || operator == nil || operator.UserID <= 0 {
		return false
	}
	_, _, err := validateTicketTakeoverDB(sqls.DB(), ticket, operator.UserID, time.Now())
	return err == nil
}

func enterpriseStatusFilterToDB(status string) []enums.TicketStatus {
	switch strings.TrimSpace(status) {
	case "", "all":
		return nil
	case "pending":
		return []enums.TicketStatus{
			enums.TicketStatusPending,
			enums.TicketStatusPendingAcceptance,
			enums.TicketStatusReopened,
			enums.TicketStatusAccepted,
			enums.TicketStatusPendingDispatch,
			enums.TicketStatusAssigned,
			enums.TicketStatusPendingAssigneeAccept,
		}
	case "pending_acceptance":
		return []enums.TicketStatus{enums.TicketStatusPendingAcceptance, enums.TicketStatusPending, enums.TicketStatusReopened}
	case "pending_dispatch":
		return []enums.TicketStatus{enums.TicketStatusAccepted, enums.TicketStatusPendingDispatch, enums.TicketStatusAssigned, enums.TicketStatusPendingAssigneeAccept}
	case "processing", "in_progress":
		return []enums.TicketStatus{
			enums.TicketStatusInProgress,
			enums.TicketStatusProcessing,
			enums.TicketStatusVideoSupport,
			enums.TicketStatusSupplierSupport,
			enums.TicketStatusEscalated,
			enums.TicketStatusWaitingCustomer,
			enums.TicketStatusResolved,
			enums.TicketStatusPendingCustomerConfirm,
		}
	case "awaiting_customer":
		return []enums.TicketStatus{
			enums.TicketStatusWaitingCustomer,
			enums.TicketStatusResolved,
			enums.TicketStatusPendingCustomerConfirm,
		}
	case "active":
		return []enums.TicketStatus{
			enums.TicketStatusPending,
			enums.TicketStatusPendingAcceptance,
			enums.TicketStatusReopened,
			enums.TicketStatusAccepted,
			enums.TicketStatusPendingDispatch,
			enums.TicketStatusAssigned,
			enums.TicketStatusPendingAssigneeAccept,
			enums.TicketStatusInProgress,
			enums.TicketStatusProcessing,
			enums.TicketStatusVideoSupport,
			enums.TicketStatusSupplierSupport,
			enums.TicketStatusEscalated,
			enums.TicketStatusWaitingCustomer,
			enums.TicketStatusResolved,
			enums.TicketStatusPendingCustomerConfirm,
		}
	case "done":
		return []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled}
	default:
		return []enums.TicketStatus{enums.TicketStatus(status)}
	}
}

func (s *enterpriseTicketService) buildListItem(ticket models.Ticket) dto.EnterpriseTicketListItemDTO {
	productName := ""
	if ticket.ProductID > 0 {
		if product := repositories.ProductRepository.Get(sqls.DB(), ticket.ProductID); product != nil {
			productName = product.Name
		}
	}
	deviceNo := ""
	if ticket.DeviceID > 0 {
		if device := repositories.DeviceRepository.Get(sqls.DB(), ticket.DeviceID); device != nil {
			deviceNo = device.DeviceNo
		}
	}
	customerName := ""
	if ticket.CustomerID > 0 {
		if customer := repositories.CustomerRepository.Get(sqls.DB(), ticket.CustomerID); customer != nil {
			customerName = customer.Name
		}
	}
	if customerName == "" && ticket.CustomerRegistrationGrantID > 0 {
		if grant := repositories.CustomerRegistrationRepository.GetForTenant(sqls.DB(), ticket.TenantID, ticket.CustomerRegistrationGrantID); grant != nil {
			customerName = strings.TrimSpace(grant.DisplayName)
			if customerName == "" {
				customerName = grant.Email
			}
		}
	}
	var assigneeName *string
	if ticket.CurrentAssigneeID > 0 {
		if user := repositories.UserRepository.Get(sqls.DB(), ticket.CurrentAssigneeID); user != nil {
			name := user.Nickname
			if name == "" {
				name = user.Username
			}
			assigneeName = &name
		}
	}
	teamName := ""
	if ticket.CurrentTeamID > 0 {
		if team := AgentTeamService.GetForTenant(ticket.CurrentTeamID, ticket.TenantID); team != nil {
			teamName = team.Name
		}
	}
	deadline, hasSLADeadline := ticketSLADeadline(ticket)
	dispatchAttempts := ticketDispatchAttemptCount(sqls.DB(), ticket.ID, ticket.DispatchAttempts)
	return dto.EnterpriseTicketListItemDTO{
		ID:                        ticket.ID,
		TicketNo:                  ticket.TicketNo,
		Title:                     ticket.Title,
		Priority:                  DeriveTicketPriority(ticket),
		Status:                    MapTicketStatusForEnterprise(ticket.Status),
		Source:                    string(ticket.Source),
		Channel:                   ticket.Channel,
		TicketIntakeDTO:           BuildTicketIntakeDTO(&ticket),
		ConversationID:            ticket.ConversationID,
		ProductID:                 ticket.ProductID,
		DeviceID:                  ticket.DeviceID,
		CurrentTeamID:             ticket.CurrentTeamID,
		TeamName:                  teamName,
		AssigneeID:                ticket.CurrentAssigneeID,
		CustomerName:              customerName,
		DeviceNo:                  deviceNo,
		ProductName:               productName,
		AssigneeName:              assigneeName,
		CreatedAt:                 formatEnterpriseTime(ticket.CreatedAt),
		UpdatedAt:                 formatEnterpriseTime(ticket.UpdatedAt),
		SLADeadline:               formatEnterpriseTime(deadline),
		SLABreached:               hasSLADeadline && time.Now().After(deadline) && !ticketSLACompleted(ticket.Status),
		DispatchAttempts:          dispatchAttempts,
		DispatchDeferredUntil:     formatEnterpriseTimePtr(ticket.DispatchDeferredUntil),
		LastDispatchFailureReason: ticket.LastDispatchFailureReason,
	}
}

// BuildListItem exposes the canonical ticket list mapping for cross-domain aggregates.
func (s *enterpriseTicketService) BuildListItem(ticket models.Ticket) dto.EnterpriseTicketListItemDTO {
	return s.buildListItem(ticket)
}

func (s *enterpriseTicketService) buildHeader(ticket *models.Ticket) dto.TicketHeaderDTO {
	return dto.TicketHeaderDTO{
		ID:              ticket.ID,
		ProductID:       ticket.ProductID,
		ProductModuleID: ticket.ProductModuleID,
		TicketNo:        ticket.TicketNo,
		Title:           ticket.Title,
		Description:     ticket.Description,
		Status:          MapTicketStatusForEnterprise(ticket.Status),
		Priority:        DeriveTicketPriority(*ticket),
		Source:          string(ticket.Source),
		Channel:         ticket.Channel,
		TicketIntakeDTO: BuildTicketIntakeDTO(ticket),
		DeviceID:        ticket.DeviceID,
		ServiceRegion:   ticket.ServiceRegion,
		ConversationID:  ticket.ConversationID,
		CreatedAt:       formatEnterpriseTime(ticket.CreatedAt),
		UpdatedAt:       formatEnterpriseTime(ticket.UpdatedAt),
		SLADeadline:     formatTicketSLADeadline(*ticket),
		Category:        ticket.FaultCode,
	}
}

func (s *enterpriseTicketService) buildCustomer(ticket *models.Ticket) dto.CustomerSummaryDTO {
	if ticket.CustomerID <= 0 {
		if ticket.CustomerRegistrationGrantID > 0 {
			if grant := repositories.CustomerRegistrationRepository.GetForTenant(sqls.DB(), ticket.TenantID, ticket.CustomerRegistrationGrantID); grant != nil {
				name := strings.TrimSpace(grant.DisplayName)
				if name == "" {
					name = grant.Email
				}
				company := ""
				if org := repositories.EnterpriseIAMRepository.GetCustomerOrg(sqls.DB(), ticket.TenantID, grant.CustomerOrgID); org != nil {
					company = org.Name
				}
				return dto.CustomerSummaryDTO{Name: name, Company: company, Email: grant.Email}
			}
		}
		return dto.CustomerSummaryDTO{Name: "Unknown Customer"}
	}
	customer := repositories.CustomerRepository.Get(sqls.DB(), ticket.CustomerID)
	if customer == nil {
		return dto.CustomerSummaryDTO{ID: ticket.CustomerID, Name: "Unknown Customer"}
	}
	result := dto.CustomerSummaryDTO{
		ID:      customer.ID,
		Name:    customer.Name,
		Contact: customer.PrimaryMobile,
		Email:   customer.PrimaryEmail,
	}
	if customer.CompanyID > 0 {
		if company := repositories.CompanyRepository.Get(sqls.DB(), customer.CompanyID); company != nil {
			result.Company = company.Name
		}
	}
	if ticket.CustomerRegistrationGrantID > 0 {
		if grant := repositories.CustomerRegistrationRepository.GetForTenant(sqls.DB(), ticket.TenantID, ticket.CustomerRegistrationGrantID); grant != nil {
			if result.Name == "" {
				result.Name = defaultString(strings.TrimSpace(grant.DisplayName), grant.Email)
			}
			if result.Email == "" {
				result.Email = grant.Email
			}
			if result.Company == "" {
				if org := repositories.EnterpriseIAMRepository.GetCustomerOrg(sqls.DB(), ticket.TenantID, grant.CustomerOrgID); org != nil {
					result.Company = org.Name
				}
			}
		}
	}
	return result
}

func (s *enterpriseTicketService) buildDeviceContext(ticket *models.Ticket) (string, dto.DeviceContextSnapshotDTO) {
	ctx := dto.DeviceContextSnapshotDTO{}
	if ticket.ProductID > 0 {
		if product := repositories.ProductRepository.Get(sqls.DB(), ticket.ProductID); product != nil {
			ctx.ProductName = product.Name
			ctx.ProductCode = product.Code
		}
	}
	if ticket.ProductModelID > 0 {
		if model := repositories.ProductModelRepository.Get(sqls.DB(), ticket.ProductModelID); model != nil {
			ctx.ModelName = model.Name
		}
	}
	if ticket.DeviceID > 0 {
		if device := repositories.DeviceRepository.Get(sqls.DB(), ticket.DeviceID); device != nil {
			ctx.DeviceNo = device.DeviceNo
			ctx.SerialNo = device.SerialNo
			ctx.RegionCode = device.RegionCode
			ctx.InstallDate = formatEnterpriseTimePtr(device.InstalledAt)
		}
		warranties := repositories.DeviceWarrantyRecordRepository.FindByDeviceID(sqls.DB(), ticket.DeviceID)
		for _, warranty := range warranties {
			if warranty.TenantID == ticket.TenantID {
				ctx.WarrantyEnd = formatEnterpriseTime(warranty.EndAt)
				break
			}
		}
	}
	if ticket.ServiceCodeID > 0 {
		if code := repositories.ServiceCodeRepository.Get(sqls.DB(), ticket.ServiceCodeID); code != nil {
			ctx.ServiceCode = code.ServiceCode
		}
	}
	return ctx.DeviceNo, ctx
}

func (s *enterpriseTicketService) buildConversationSnapshot(ticket *models.Ticket) *dto.ConversationSnapshotDTO {
	if ticket.ConversationID <= 0 {
		return nil
	}
	conversation := repositories.ConversationRepository.Get(sqls.DB(), ticket.ConversationID)
	if conversation == nil {
		return nil
	}
	return &dto.ConversationSnapshotDTO{
		ConversationID: conversation.ID,
		Summary:        conversation.LastMessageSummary,
		MessageCount:   repositories.MessageRepository.Count(sqls.DB(), sqls.NewCnd().Eq("conversation_id", conversation.ID)),
		LastMessageAt:  formatEnterpriseTime(conversation.LastMessageAt),
		AIServed:       conversation.AIReplyRounds > 0,
		HandoffReason:  conversation.HandoffReason,
	}
}

func (s *enterpriseTicketService) buildDiagnosisSnapshot(ticket *models.Ticket) *dto.DiagnosisHandoffSnapshotDTO {
	session := repositories.DiagnosisSessionRepository.FindLatestForContext(
		sqls.DB(), ticket.TenantID, ticket.ConversationID, ticket.DeviceID, ticket.ProductID,
	)
	if session == nil && strings.TrimSpace(ticket.DiagnosisSummary) == "" && strings.TrimSpace(ticket.SymptomSummary) == "" {
		return nil
	}
	actions := []string{}
	if session != nil && strings.TrimSpace(session.Resolution) != "" {
		actions = append(actions, strings.TrimSpace(session.Resolution))
	} else if strings.TrimSpace(ticket.DiagnosisSummary) != "" {
		actions = append(actions, ticket.DiagnosisSummary)
	}
	result := &dto.DiagnosisHandoffSnapshotDTO{
		FaultCategory:      ticket.FaultCode,
		RecommendedActions: actions,
		TriageLevel:        DeriveTicketPriority(*ticket),
		HandoffReason:      ticket.SymptomSummary,
	}
	if session != nil {
		result.DiagnosisSessionID = session.ID
		result.Confidence = session.ConfidenceScore
		if result.FaultCategory == "" {
			var faultCodes []string
			if json.Unmarshal([]byte(session.FaultCodes), &faultCodes) == nil && len(faultCodes) > 0 {
				result.FaultCategory = faultCodes[0]
			}
		}
	}
	return result
}

func (s *enterpriseTicketService) buildFlow(ticket *models.Ticket) dto.TicketFlowDTO {
	steps := []string{"Accept", "Dispatch", "Process", "Repair", "Close"}
	currentIndex := ticketFlowCurrentIndex(ticket.Status)
	milestones := s.ticketFlowMilestones(ticket)
	if currentIndex == 2 {
		if _, repairCompleted := milestones["Repair"]; repairCompleted {
			currentIndex = 3
		}
	}
	result := dto.TicketFlowDTO{CurrentStep: steps[minInt(currentIndex, len(steps)-1)]}
	for i, name := range steps {
		stepStatus := "pending"
		if i < currentIndex {
			stepStatus = "done"
		}
		if i == currentIndex && currentIndex < len(steps) {
			stepStatus = "current"
		}
		step := dto.TicketFlowStepDTO{Name: name, Status: stepStatus}
		if milestone, ok := milestones[name]; ok {
			step.CompletedAt = formatEnterpriseTime(milestone.completedAt)
			step.CompletedBy = milestone.completedBy
		}
		result.Steps = append(result.Steps, step)
	}
	return result
}

type ticketFlowMilestone struct {
	completedAt time.Time
	completedBy string
}

func ticketFlowCurrentIndex(status enums.TicketStatus) int {
	switch enums.NormalizeTicketStatus(string(status)) {
	case enums.TicketStatusAccepted, enums.TicketStatusPendingDispatch:
		return 1
	case enums.TicketStatusPendingAssigneeAccept:
		return 0
	case enums.TicketStatusProcessing, enums.TicketStatusVideoSupport, enums.TicketStatusReopened:
		return 2
	case enums.TicketStatusResolved, enums.TicketStatusPendingCustomerConfirm, enums.TicketStatusQualityReview:
		return 4
	case enums.TicketStatusClosed, enums.TicketStatusCancelled:
		return 5
	default:
		return 0
	}
}

func (s *enterpriseTicketService) ticketFlowMilestones(ticket *models.Ticket) map[string]ticketFlowMilestone {
	progresses := repositories.TicketProgressRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", ticket.TenantID).
		Eq("ticket_id", ticket.ID).
		Asc("id"))
	start := 0
	for i := range progresses {
		if ticketProgressMatches(progresses[i], enums.TicketProgressEventReopened, "重新打开", "reopen") {
			start = i + 1
		}
	}
	milestones := make(map[string]ticketFlowMilestone)
	setProgressMilestone := func(step string, progress models.TicketProgress) {
		if _, exists := milestones[step]; exists {
			return
		}
		milestones[step] = ticketFlowMilestone{
			completedAt: progress.CreatedAt,
			completedBy: ticketProgressAuthorName(progress.AuthorID),
		}
	}
	for _, progress := range progresses[start:] {
		switch {
		case ticketProgressMatches(progress, enums.TicketProgressEventAccepted, "受理工单", "accept"):
			setProgressMilestone("Accept", progress)
		case ticketProgressMatches(progress, enums.TicketProgressEventAssigned, "分配工单", "指派工单", "assign"):
			setProgressMilestone("Dispatch", progress)
		case ticketProgressMatches(progress, enums.TicketProgressEventRepairCompleted, "维修记录", "repair"):
			setProgressMilestone("Process", progress)
			setProgressMilestone("Repair", progress)
		case ticketProgressMatches(progress, enums.TicketProgressEventClosed, "关闭工单", "close"):
			setProgressMilestone("Close", progress)
		case progress.EventType == enums.TicketProgressEventProcessing && progressTargetsResolvedStatus(progress.MetadataJSON):
			setProgressMilestone("Process", progress)
		}
	}
	// Claiming a ticket from the product-team pool assigns it directly to the
	// accepting engineer, so there is no separate ticket_assigned progress row.
	// Reuse the acceptance milestone to keep the completed dispatch step auditable.
	if _, ok := milestones["Dispatch"]; !ok && ticket.CurrentAssigneeID > 0 {
		if accepted, acceptedOK := milestones["Accept"]; acceptedOK {
			milestones["Dispatch"] = accepted
		}
	}

	if _, ok := milestones["Process"]; !ok && ticket.ResolvedAt != nil {
		milestones["Process"] = ticketFlowMilestone{completedAt: *ticket.ResolvedAt, completedBy: ticket.UpdateUserName}
	}
	if _, ok := milestones["Repair"]; !ok {
		repairs := repositories.TicketRepairRepository.FindByTicketID(sqls.DB(), ticket.ID)
		if len(repairs) > 0 {
			latest := repairs[len(repairs)-1]
			completedAt := latest.UpdatedAt
			if latest.FinishedAt != nil {
				completedAt = *latest.FinishedAt
			}
			milestones["Repair"] = ticketFlowMilestone{
				completedAt: completedAt,
				completedBy: firstNonEmptyString(latest.UpdateUserName, latest.CreateUserName),
			}
		}
	}
	if _, ok := milestones["Close"]; !ok && ticket.HandledAt != nil {
		milestones["Close"] = ticketFlowMilestone{completedAt: *ticket.HandledAt, completedBy: ticket.UpdateUserName}
	}
	return milestones
}

func ticketProgressMatches(progress models.TicketProgress, eventType enums.TicketProgressEventType, contentParts ...string) bool {
	if progress.EventType == eventType {
		return true
	}
	content := strings.ToLower(progress.Content)
	for _, part := range contentParts {
		if strings.Contains(content, strings.ToLower(part)) {
			return true
		}
	}
	return false
}

func progressTargetsResolvedStatus(metadataJSON string) bool {
	var metadata struct {
		ToStatus string `json:"to_status"`
	}
	if json.Unmarshal([]byte(metadataJSON), &metadata) != nil {
		return false
	}
	target := enums.NormalizeTicketStatus(metadata.ToStatus)
	return target == enums.TicketStatusResolved || target == enums.TicketStatusPendingCustomerConfirm || target == enums.TicketStatusClosed
}

func ticketProgressAuthorName(authorID int64) string {
	if authorID <= 0 {
		return ""
	}
	user := repositories.UserRepository.Get(sqls.DB(), authorID)
	if user == nil {
		return ""
	}
	return firstNonEmptyString(user.Nickname, user.Username)
}

func ticketDispatchAttemptCount(db *gorm.DB, ticketID int64, fallback int) int {
	count, err := repositories.TicketDispatchAttemptRepository.CountByTicket(db, ticketID)
	if err != nil || count <= 0 {
		return fallback
	}
	return int(count)
}

func (s *enterpriseTicketService) buildAssignment(ticket *models.Ticket) dto.TicketAssignmentDTO {
	dispatchAttempts := ticketDispatchAttemptCount(sqls.DB(), ticket.ID, ticket.DispatchAttempts)
	assignment := dto.TicketAssignmentDTO{
		AssigneeID:                ticket.CurrentAssigneeID,
		TeamID:                    ticket.CurrentTeamID,
		AssignedAt:                formatEnterpriseTimePtr(ticket.AssignedAt),
		AcceptedAt:                formatEnterpriseTimePtr(ticket.AcceptedAt),
		AcceptDeadlineAt:          formatEnterpriseTimePtr(ticket.AcceptDeadlineAt),
		DispatchAttempts:          dispatchAttempts,
		DispatchDeferredUntil:     formatEnterpriseTimePtr(ticket.DispatchDeferredUntil),
		LastDispatchFailureReason: ticket.LastDispatchFailureReason,
		CanTransfer:               ticket.CurrentAssigneeID > 0,
		CanEscalate:               ticket.Status != enums.TicketStatusClosed && ticket.Status != enums.TicketStatusDone,
	}
	if ticket.CurrentTeamID > 0 {
		if team := AgentTeamService.GetForTenant(ticket.CurrentTeamID, ticket.TenantID); team != nil {
			assignment.TeamName = team.Name
		}
	}
	if ticket.CurrentAssigneeID > 0 {
		if user := repositories.UserRepository.Get(sqls.DB(), ticket.CurrentAssigneeID); user != nil {
			assignment.AssigneeName = user.Nickname
			if assignment.AssigneeName == "" {
				assignment.AssigneeName = user.Username
			}
		}
	}
	return assignment
}

func (s *enterpriseTicketService) buildMeeting(ticket *models.Ticket) dto.TicketMeetingDTO {
	meeting := repositories.MeetingRoomRepository.FindLatestJitsiByTicketID(sqls.DB(), ticket.TenantID, strconv.FormatInt(ticket.ID, 10))
	if meeting == nil {
		return dto.TicketMeetingDTO{Status: "waiting"}
	}
	attendance := repositories.MeetingRoomRepository.AttendanceStats(sqls.DB(), meeting.ID)
	activeParticipantCount, externalParticipantCount, externalParticipantNames := ticketMeetingActiveParticipantSummary(meeting.ID)
	startedAt := confirmedMeetingStartedAt(meeting, attendance)
	return dto.TicketMeetingDTO{
		MeetingID:                meeting.ID,
		Title:                    ticket.Title,
		Status:                   mapEnterpriseMeetingStatus(meeting.Status),
		ScheduledAt:              formatEnterpriseTimePtr(meeting.ScheduledAt),
		StartedAt:                formatEnterpriseTimePtr(startedAt),
		Duration:                 meetingDurationSeconds(startedAt, meeting.EndedAt),
		ParticipantCount:         attendance.Count,
		ActiveParticipantCount:   activeParticipantCount,
		ExternalParticipantCount: externalParticipantCount,
		ExternalParticipantNames: externalParticipantNames,
	}
}

func ticketMeetingActiveParticipantSummary(meetingID string) (int64, int64, []string) {
	if strings.TrimSpace(meetingID) == "" {
		return 0, 0, []string{}
	}
	var participants []models.MeetingParticipant
	if err := sqls.DB().
		Where("meeting_id = ? AND joined_at IS NOT NULL AND left_at IS NULL", meetingID).
		Order("joined_at ASC, created_at ASC").
		Find(&participants).Error; err != nil {
		return 0, 0, []string{}
	}
	names := make([]string, 0, len(participants))
	externalCount := int64(0)
	for i := range participants {
		switch strings.TrimSpace(participants[i].UserType) {
		case "customer", "partner", "supplier":
			externalCount++
			if name := strings.TrimSpace(participants[i].ParticipantName); name != "" && len(names) < 3 {
				names = append(names, name)
			}
		default:
			continue
		}
	}
	return int64(len(participants)), externalCount, names
}

func (s *enterpriseTicketService) buildRepair(ticket *models.Ticket, deviceNo string) dto.TicketRepairDTO {
	repair := dto.TicketRepairDTO{
		DeviceNo:  deviceNo,
		FaultType: ticket.FaultCode,
		Parts:     []dto.RepairPartDTO{},
	}
	repairs := repositories.TicketRepairRepository.FindByTicketID(sqls.DB(), ticket.ID)
	if len(repairs) == 0 {
		return repair
	}
	latest := repairs[len(repairs)-1]
	repair.RepairID = latest.ID
	repair.Resolution = latest.Solution
	repair.CostHours = latest.CostHours
	repair.CompletedAt = formatEnterpriseTimePtr(latest.FinishedAt)
	repair.TechnicianName = firstNonEmptyString(latest.UpdateUserName, latest.CreateUserName)
	if strings.TrimSpace(latest.PartsJSON) != "" {
		parts := make([]dto.RepairPartDTO, 0)
		if json.Unmarshal([]byte(latest.PartsJSON), &parts) == nil && parts != nil {
			repair.Parts = parts
		}
	}
	if repair.Parts == nil {
		repair.Parts = []dto.RepairPartDTO{}
	}
	return repair
}

func (s *enterpriseTicketService) buildFeedback(ticket *models.Ticket) *dto.TicketFeedbackDTO {
	feedback := repositories.TicketFeedbackRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", ticket.TenantID).
		Eq("ticket_id", ticket.ID).
		Desc("submitted_at").
		Desc("id"))
	if feedback == nil {
		return nil
	}
	result := &dto.TicketFeedbackDTO{
		Rating:      feedback.Rating,
		Comment:     feedback.Comment,
		SubmittedAt: formatEnterpriseTime(feedback.SubmittedAt),
		Tags:        []string{},
	}
	if strings.TrimSpace(feedback.TagsJSON) != "" {
		tags := make([]string, 0)
		if json.Unmarshal([]byte(feedback.TagsJSON), &tags) == nil && tags != nil {
			result.Tags = tags
		}
	}
	return result
}

func (s *enterpriseTicketService) buildAssets(ticket *models.Ticket) []dto.AssetRefDTO {
	if ticket.ConversationID <= 0 {
		return []dto.AssetRefDTO{}
	}
	messages := repositories.MessageRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("conversation_id", ticket.ConversationID).
		In("message_type", []enums.IMMessageType{enums.IMMessageTypeImage, enums.IMMessageTypeAudio, enums.IMMessageTypeAttachment}).
		Asc("id"))
	result := make([]dto.AssetRefDTO, 0, len(messages))
	seen := make(map[string]bool)
	for _, message := range messages {
		var payload struct {
			AssetID      string `json:"assetId"`
			AssetIDSnake string `json:"asset_id"`
		}
		if json.Unmarshal([]byte(message.Payload), &payload) != nil {
			continue
		}
		assetID := firstNonEmptyString(payload.AssetID, payload.AssetIDSnake)
		if assetID == "" || seen[assetID] {
			continue
		}
		asset := repositories.AssetRepository.GetByAssetID(sqls.DB(), assetID)
		if asset == nil || asset.TenantID != ticket.TenantID {
			continue
		}
		seen[assetID] = true
		result = append(result, dto.AssetRefDTO{
			ID:         asset.ID,
			FileName:   asset.Filename,
			FileType:   asset.MimeType,
			FileSize:   asset.FileSize,
			UploadedAt: formatEnterpriseTime(asset.CreatedAt),
			UploadedBy: asset.CreateUserName,
		})
	}
	return result
}

func (s *enterpriseTicketService) buildTimeline(ticket *models.Ticket) []dto.TicketTimelineItemDTO {
	timeline := make([]dto.TicketTimelineItemDTO, 0)
	progresses := repositories.TicketProgressRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", ticket.TenantID).
		Eq("ticket_id", ticket.ID).
		Asc("id"))
	hasCreatedProgress := false
	for _, item := range progresses {
		actor := ""
		eventType := item.EventType
		if eventType == "" {
			eventType = enums.TicketProgressEventProgress
		}
		if eventType == enums.TicketProgressEventCreated {
			hasCreatedProgress = true
		}
		if item.AuthorID > 0 {
			if user := repositories.UserRepository.Get(sqls.DB(), item.AuthorID); user != nil {
				actor = user.Nickname
				if actor == "" {
					actor = user.Username
				}
			}
		}
		timeline = append(timeline, dto.TicketTimelineItemDTO{
			ID:        item.ID,
			Type:      string(eventType),
			Content:   item.Content,
			Actor:     actor,
			Timestamp: formatEnterpriseTime(item.CreatedAt),
		})
	}
	if !hasCreatedProgress {
		timeline = append([]dto.TicketTimelineItemDTO{{
			ID:        ticket.ID,
			Type:      "created",
			Content:   "Ticket created",
			Actor:     ticket.CreateUserName,
			Timestamp: formatEnterpriseTime(ticket.CreatedAt),
		}}, timeline...)
	}
	return timeline
}

func (s *enterpriseTicketService) buildAuditRefs(ticket *models.Ticket) []dto.AuditRefDTO {
	refs := []dto.AuditRefDTO{}
	if ticket.CreateUserName != "" {
		refs = append(refs, dto.AuditRefDTO{
			ID:        ticket.ID,
			Action:    "create",
			Operator:  ticket.CreateUserName,
			Timestamp: formatEnterpriseTime(ticket.CreatedAt),
			Detail:    ticket.Title,
		})
	}
	return refs
}

func ticketSLADeadline(ticket models.Ticket) (time.Time, bool) {
	if ticket.SLADueAt != nil && !ticket.SLADueAt.IsZero() {
		return *ticket.SLADueAt, true
	}
	return time.Time{}, false
}

func formatTicketSLADeadline(ticket models.Ticket) string {
	deadline, ok := ticketSLADeadline(ticket)
	if !ok {
		return ""
	}
	return formatEnterpriseTime(deadline)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

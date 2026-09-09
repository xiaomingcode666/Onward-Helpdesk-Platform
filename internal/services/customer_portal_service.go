package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var CustomerPortalService = newCustomerPortalService()

type customerPortalService struct{}

var customerPortalVisibleProgressEventTypes = []string{
	string(enums.TicketProgressEventCreated),
	string(enums.TicketProgressEventAccepted),
	string(enums.TicketProgressEventAssigned),
	string(enums.TicketProgressEventProcessing),
	string(enums.TicketProgressEventEscalated),
	string(enums.TicketProgressEventRepairCompleted),
	string(enums.TicketProgressEventClosed),
	string(enums.TicketProgressEventReopened),
	string(enums.TicketProgressEventMeetingEnded),
}

func (s *customerPortalService) BindDeviceByServiceCode(req request.BindCustomerDeviceByServiceCodeRequest, operator *dto.AuthPrincipal) (*models.CustomerDeviceBinding, *models.Device, *models.Product, error) {
	if operator == nil || operator.UserID <= 0 || operator.DomainType != models.DomainTypeCustomer {
		return nil, nil, nil, errorsx.Unauthorized("customer account is required")
	}
	resolved, err := ServiceCodeResolveService.Resolve(req.ServiceCode)
	if err != nil {
		return nil, nil, nil, err
	}
	if resolved == nil || !resolved.Valid || resolved.Tenant == nil || resolved.Product == nil {
		return nil, nil, nil, errorsx.InvalidParam("service code is not available")
	}
	if resolved.Tenant.IsKnowledgeSupportScene() {
		return nil, nil, nil, errorsx.Forbidden("device binding is unavailable for this tenant")
	}
	if resolved.Device == nil && strings.TrimSpace(req.DeviceNo) == "" {
		return nil, nil, nil, errorsx.InvalidParam("deviceNo is required for this service code")
	}
	if operator.SubjectType == models.SubjectTypeCustomerUser && operator.TenantID > 0 && operator.TenantID != resolved.Tenant.ID {
		return nil, nil, nil, errorsx.Forbidden("device belongs to another enterprise")
	}

	var customerUser *models.CustomerUser
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		var ensureErr error
		customerUser, ensureErr = s.ensureDeviceCustomerUser(ctx, resolved, operator)
		return ensureErr
	}); err != nil {
		return nil, nil, nil, err
	}

	entry, err := CustomerEntryService.CreateEntrySession(request.CreateCustomerEntrySessionRequest{
		ServiceCode: req.ServiceCode, DeviceNo: req.DeviceNo, SerialNo: req.SerialNo, RegionCode: req.RegionCode,
		VisitorID: fmt.Sprintf("account_%d", operator.UserID), CustomerUserID: customerUser.ID,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	if entry.Session == nil || entry.Device == nil {
		return nil, nil, nil, errorsx.InvalidParam("this service code requires device registration before binding")
	}
	if entry.Session.TenantID != customerUser.TenantID || entry.Session.CustomerUserID != customerUser.ID {
		return nil, nil, nil, errorsx.InvalidParam("device belongs to another tenant")
	}
	binding, err := CustomerEntryService.ConfirmDeviceBinding(request.ConfirmCustomerDeviceBindingRequest{
		EntrySessionID: entry.Session.ID, VisitorID: entry.Session.VisitorID, VisitorToken: entry.VisitorToken,
		CustomerUserID: customerUser.ID, CustomerOrgID: customerUser.CustomerOrgID, BindingRole: "owner",
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return binding, entry.Device, entry.Product, nil
}

func (s *customerPortalService) ensureDeviceCustomerUser(ctx *sqls.TxContext, resolved *ServiceCodeResolveResult, operator *dto.AuthPrincipal) (*models.CustomerUser, error) {
	if ctx == nil || ctx.Tx == nil || resolved == nil || resolved.Tenant == nil || operator == nil {
		return nil, errorsx.InvalidParam("customer registration context is incomplete")
	}
	tenantID := resolved.Tenant.ID
	user := repositories.UserRepository.Get(ctx.Tx, operator.UserID)
	if user == nil || user.Status != enums.StatusOk {
		return nil, errorsx.Unauthorized("customer account is not available")
	}
	customerDefaultLocale := TenantPortalSettingsService.CustomerDefaultLocaleDB(ctx.Tx, resolved.Tenant)

	if existing := repositories.CustomerPortalRepository.FindCustomerUserByTenantAndUserID(ctx.Tx, tenantID, user.ID); existing != nil {
		if resolved.Device != nil && resolved.Device.CustomerOrgID > 0 && existing.CustomerOrgID != resolved.Device.CustomerOrgID {
			return nil, errorsx.Forbidden("device is assigned to another customer organization")
		}
		if _, err := CustomerService.EnsureExternalCustomer(ctx, openidentity.ExternalUser{
			ExternalSource: enums.ExternalSourceUser,
			ExternalID:     strconv.FormatInt(user.ID, 10),
			ExternalName:   existing.DisplayName,
		}); err != nil {
			return nil, err
		}
		return existing, nil
	}

	var customerOrg *models.CustomerOrg
	if resolved.Device != nil && resolved.Device.CustomerOrgID > 0 {
		customerOrg = repositories.EnterpriseIAMRepository.GetCustomerOrg(ctx.Tx, tenantID, resolved.Device.CustomerOrgID)
		if customerOrg == nil || customerOrg.Status != enums.StatusOk {
			return nil, errorsx.InvalidParam("device customer organization is not available")
		}
	} else {
		customerNo := fmt.Sprintf("SELF-%d-%d", tenantID, user.ID)
		customerOrg = repositories.CustomerPortalRepository.FindCustomerOrgByCustomerNo(ctx.Tx, tenantID, customerNo)
		if customerOrg == nil {
			displayName := strings.TrimSpace(user.Nickname)
			if displayName == "" {
				displayName = strings.TrimSpace(user.Username)
			}
			customerOrg = &models.CustomerOrg{
				TenantID: tenantID, CustomerNo: customerNo, Name: displayName + "的设备账户",
				DefaultLocale: customerDefaultLocale, Timezone: "UTC",
				Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(operator),
			}
			if err := repositories.EnterpriseIAMRepository.CreateCustomerOrg(ctx.Tx, customerOrg); err != nil {
				return nil, err
			}
		}
	}

	displayName := strings.TrimSpace(user.Nickname)
	if displayName == "" {
		displayName = strings.TrimSpace(user.Username)
	}
	customerUser := repositories.CustomerPortalRepository.FindCustomerUserByTenantOrgAndUserID(ctx.Tx, tenantID, customerOrg.ID, user.ID)
	if customerUser == nil {
		customerUser = &models.CustomerUser{
			TenantID: tenantID, CustomerOrgID: customerOrg.ID, UserID: user.ID, DisplayName: displayName,
			Email: userEmail(user), Phone: userMobile(user), Locale: customerDefaultLocale, Timezone: "UTC",
			Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(operator),
		}
		if err := repositories.EnterpriseIAMRepository.CreateCustomerUser(ctx.Tx, customerUser); err != nil {
			return nil, err
		}
	}
	if _, err := CustomerService.EnsureExternalCustomer(ctx, openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     strconv.FormatInt(user.ID, 10),
		ExternalName:   displayName,
	}); err != nil {
		return nil, err
	}
	if err := EnsureTenantDefaultIAMRolesDB(ctx.Tx, tenantID, operator); err != nil {
		return nil, err
	}
	if err := replaceIAMRoleBindingsDB(ctx.Tx, tenantID, models.DomainTypeCustomer, models.SubjectTypeCustomerUser, customerUser.ID, []string{CustomerRoleUser}, CustomerRoleUser, operator); err != nil {
		return nil, err
	}
	return customerUser, nil
}

type customerPortalScope struct {
	customer     *models.Customer
	customerUser *models.CustomerUser
	devices      []models.Device
	deviceByID   map[int64]*models.Device
	deviceIDs    []int64
	tenantIDs    []int64
}

func customerBindingScope(external openidentity.ExternalUser, customer *models.Customer) (int64, int64) {
	if external.ExternalSource == enums.ExternalSourceUser {
		accountUserID, err := strconv.ParseInt(strings.TrimSpace(external.ExternalID), 10, 64)
		if err == nil && accountUserID > 0 {
			db := sqls.DB()
			if db != nil && db.Migrator().HasTable(&models.CustomerUser{}) {
				if formalUser := repositories.CustomerPortalRepository.FindCustomerUserByUserID(db, accountUserID); formalUser != nil {
					return formalUser.ID, formalUser.CustomerOrgID
				}
			}
			// Compatibility for customer-session identities created before formal
			// CustomerUser provisioning used the platform user ID directly.
			return accountUserID, 0
		}
	}
	if customer != nil && customer.ID > 0 {
		return 0, customer.ID
	}
	return 0, 0
}

func newCustomerPortalService() *customerPortalService {
	return &customerPortalService{}
}

func (s *customerPortalService) HeartbeatPresence(operator *dto.AuthPrincipal) (*dto.CustomerPortalPresenceDTO, error) {
	if operator == nil || !operator.IsCustomer() || operator.UserID <= 0 {
		return nil, errorsx.Unauthorized("customer account is required")
	}
	now := time.Now()
	if operator.SubjectType == models.SubjectTypePendingCustomer {
		return &dto.CustomerPortalPresenceDTO{
			Online:     true,
			LastSeenAt: formatCustomerTime(now),
		}, nil
	}
	if operator.SubjectType != models.SubjectTypeCustomerUser || operator.SubjectID <= 0 || operator.TenantID <= 0 {
		return nil, errorsx.Unauthorized("customer account is required")
	}
	touched, err := repositories.CustomerPortalRepository.TouchCustomerUserLastSeenAt(sqls.DB(), operator.TenantID, operator.SubjectID, operator.UserID, now)
	if err != nil {
		return nil, err
	}
	if !touched {
		return nil, errorsx.Unauthorized("customer account is not available")
	}
	return &dto.CustomerPortalPresenceDTO{
		Online:     true,
		LastSeenAt: formatCustomerTime(now),
	}, nil
}

func (s *customerPortalService) GetProfile(external openidentity.ExternalUser) (*dto.CustomerPortalProfileDTO, error) {
	scope, err := s.resolveScope(external)
	if err != nil {
		return nil, err
	}
	conversations := s.findVisibleConversations(scope, external)
	tickets := s.findVisibleTickets(scope, conversations)
	meetings := repositories.CustomerPortalRepository.FindVisibleMeetings(sqls.DB(), scope.tenantIDs, collectTicketIDStrings(tickets))
	activeConversations := 0
	for _, item := range conversations {
		if item.ClosedAt == nil {
			activeConversations++
		}
	}
	openTickets := 0
	for _, item := range tickets {
		if isOpenTicketStatus(item.Status) {
			openTickets++
		}
	}
	upcomingMeetings := 0
	for _, item := range meetings {
		if item.Status != "ended" {
			upcomingMeetings++
		}
	}
	companyName := ""
	if scope.customerUser != nil {
		if tenant := repositories.TenantRepository.Get(sqls.DB(), scope.customerUser.TenantID); tenant != nil {
			companyName = tenant.Name
		}
	}
	if companyName == "" && scope.customer.CompanyID > 0 {
		if company := repositories.CompanyRepository.Get(sqls.DB(), scope.customer.CompanyID); company != nil {
			companyName = company.Name
		}
	}
	return &dto.CustomerPortalProfileDTO{
		ID:                      scope.customer.ID,
		Name:                    scope.customer.Name,
		CompanyName:             companyName,
		PrimaryEmail:            scope.customer.PrimaryEmail,
		PrimaryMobile:           scope.customer.PrimaryMobile,
		LastActiveAt:            formatCustomerTimePtr(scope.customer.LastActiveAt),
		BoundDeviceCount:        len(scope.devices),
		ActiveConversationCount: activeConversations,
		OpenTicketCount:         openTickets,
		UpcomingMeetingCount:    upcomingMeetings,
	}, nil
}

func (s *customerPortalService) UpdateProfile(external openidentity.ExternalUser, req request.UpdateCustomerPortalProfileRequest) (*dto.CustomerPortalProfileDTO, error) {
	scope, err := s.resolveScope(external)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errorsx.InvalidParam("customer name is required")
	}
	email := strings.TrimSpace(req.PrimaryEmail)
	mobile := strings.TrimSpace(req.PrimaryMobile)
	accountUserID := int64(0)
	if external.ExternalSource == enums.ExternalSourceUser {
		accountUserID, _ = strconv.ParseInt(strings.TrimSpace(external.ExternalID), 10, 64)
	}
	tenantID := int64(0)
	if scope.customerUser != nil {
		tenantID = scope.customerUser.TenantID
	}
	operator := &dto.AuthPrincipal{
		UserID:      accountUserID,
		Username:    name,
		TenantID:    tenantID,
		DomainType:  models.DomainTypeCustomer,
		SubjectType: models.SubjectTypeCustomerUser,
	}

	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		now := time.Now()
		if accountUserID > 0 {
			if email != "" {
				if existing := repositories.UserRepository.GetByEmail(ctx.Tx, email); existing != nil && existing.ID != accountUserID {
					return errorsx.InvalidParam("email is already in use")
				}
			}
			var userEmail *string
			if email != "" {
				userEmail = &email
			}
			if err := repositories.UserRepository.Updates(ctx.Tx, accountUserID, map[string]any{
				"email":            userEmail,
				"update_user_id":   accountUserID,
				"update_user_name": name,
				"updated_at":       now,
			}); err != nil {
				return err
			}
		}
		if err := repositories.CustomerRepository.Updates(ctx.Tx, scope.customer.ID, map[string]any{
			"name":             name,
			"primary_email":    email,
			"primary_mobile":   mobile,
			"update_user_id":   accountUserID,
			"update_user_name": name,
			"updated_at":       now,
		}); err != nil {
			return err
		}
		if err := CustomerService.syncConversationCustomerName(ctx.Tx, scope.customer.ID, name, operator, now); err != nil {
			return err
		}
		if scope.customerUser != nil {
			if err := ctx.Tx.Model(&models.CustomerUser{}).
				Where("tenant_id = ? AND id = ?", scope.customerUser.TenantID, scope.customerUser.ID).
				Updates(map[string]any{
					"display_name":     name,
					"email":            email,
					"phone":            mobile,
					"update_user_id":   accountUserID,
					"update_user_name": name,
					"updated_at":       now,
				}).Error; err != nil {
				return err
			}
		}
		if accountUserID > 0 && ctx.Tx.Migrator().HasTable(&models.User{}) {
			if err := repositories.UserRepository.Updates(ctx.Tx, accountUserID, map[string]any{
				"nickname":         name,
				"email":            utils.NormalizeNullableString(&email),
				"mobile":           utils.NormalizeNullableString(&mobile),
				"update_user_id":   accountUserID,
				"update_user_name": name,
				"updated_at":       now,
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if accountUserID > 0 {
		customerPortalInvalidateUsers(accountUserID)
	}
	return s.GetProfile(external)
}

func (s *customerPortalService) GetHome(external openidentity.ExternalUser, tenantIDs ...int64) (*dto.CustomerPortalHomeDTO, error) {
	profile, err := s.GetProfile(external)
	if err != nil {
		return nil, err
	}
	meetings, err := s.ListMeetings(external)
	if err != nil {
		return nil, err
	}
	metrics := buildCustomerPortalMetrics(profile)
	for _, tenantID := range tenantIDs {
		if TenantCapabilityService.KnowledgeSupport(tenantID) {
			metrics = filterCustomerPortalDeviceMetrics(metrics)
			break
		}
	}
	result := &dto.CustomerPortalHomeDTO{
		Metrics:        metrics,
		RecentDevices:  []dto.CustomerPortalDeviceDTO{},
		PendingTickets: []dto.CustomerPortalTicketDTO{},
	}
	for i := range meetings {
		if meetings[i].Status == "active" {
			result.UpcomingMeeting = &meetings[i]
			break
		}
		if result.UpcomingMeeting == nil && (meetings[i].Status == "waiting" || meetings[i].Status == "scheduled") {
			result.UpcomingMeeting = &meetings[i]
		}
	}
	return result, nil
}

func (s *customerPortalService) ListDevicesPage(external openidentity.ExternalUser, req request.CustomerPortalListRequest) ([]dto.CustomerPortalDeviceDTO, *sqls.Paging, error) {
	items, err := s.ListDevices(external)
	if err != nil {
		return nil, nil, err
	}
	filtered := filterCustomerPortalDevices(items, req)
	if req.DeviceID > 0 && len(filtered) > 0 {
		if detail, detailErr := s.buildCustomerPortalDeviceDetail(external, filtered[0].ID); detailErr == nil && detail != nil {
			filtered[0] = *detail
		}
	}
	return paginateCustomerPortalItems(filtered, req.GetPage(), req.GetLimit())
}

func (s *customerPortalService) ListDevices(external openidentity.ExternalUser) ([]dto.CustomerPortalDeviceDTO, error) {
	scope, err := s.resolveScope(external)
	if err != nil {
		return nil, err
	}
	conversations := s.findVisibleConversations(scope, external)
	tickets := s.findVisibleTickets(scope, conversations)
	repairs := repositories.CustomerPortalRepository.FindVisibleRepairRecords(sqls.DB(), scope.tenantIDs, scope.deviceIDs)
	productIDs := make([]int64, 0, len(scope.devices))
	modelIDs := make([]int64, 0, len(scope.devices))
	for i := range scope.devices {
		if scope.devices[i].ProductID > 0 {
			productIDs = append(productIDs, scope.devices[i].ProductID)
		}
		if scope.devices[i].ProductModelID > 0 {
			modelIDs = append(modelIDs, scope.devices[i].ProductModelID)
		}
	}
	products := loadCustomerPortalProductsByID(productIDs)
	productModels := loadCustomerPortalProductModelsByID(modelIDs)
	manualCountByProductID := loadCustomerPortalManualCountByProductID(productIDs)
	warrantyByDeviceID := loadCustomerPortalLatestWarrantyByDeviceID(scope.deviceIDs)
	openTicketCount := make(map[int64]int)
	for _, item := range tickets {
		if item.DeviceID > 0 && isOpenTicketStatus(item.Status) {
			openTicketCount[item.DeviceID]++
		}
	}
	conversationCount := make(map[int64]int)
	for _, item := range conversations {
		if item.DeviceID > 0 {
			conversationCount[item.DeviceID]++
		}
	}
	repairCount := make(map[int64]int)
	for _, item := range repairs {
		repairCount[item.DeviceID]++
	}
	result := make([]dto.CustomerPortalDeviceDTO, 0, len(scope.devices))
	for _, item := range scope.devices {
		productName, productCode := "", ""
		if product := products[item.ProductID]; product != nil {
			productName = product.Name
			productCode = product.Code
		}
		modelName := ""
		if model := productModels[item.ProductModelID]; model != nil {
			modelName = model.Name
		}
		warrantyEndAt := ""
		if warranty := warrantyByDeviceID[item.ID]; warranty != nil {
			warrantyEndAt = formatCustomerTime(warranty.EndAt)
		}
		result = append(result, dto.CustomerPortalDeviceDTO{
			ID:                 item.ID,
			DeviceNo:           item.DeviceNo,
			SerialNo:           item.SerialNo,
			ProductName:        productName,
			ProductCode:        productCode,
			ModelName:          modelName,
			RegionCode:         item.RegionCode,
			Status:             mapCustomerDeviceStatus(item),
			LastServiceAt:      formatCustomerTimePtr(item.LastServiceAt),
			InstalledAt:        formatCustomerTimePtr(item.InstalledAt),
			WarrantyEndAt:      warrantyEndAt,
			ManualCount:        manualCountByProductID[item.ProductID],
			RepairHistoryCount: repairCount[item.ID],
			OpenTicketCount:    openTicketCount[item.ID],
			ConversationCount:  conversationCount[item.ID],
		})
	}
	return result, nil
}

func (s *customerPortalService) ListConversationsPage(external openidentity.ExternalUser, req request.CustomerPortalListRequest) ([]dto.CustomerPortalConversationDTO, *sqls.Paging, error) {
	items, err := s.listConversations(external, req.Locale)
	if err != nil {
		return nil, nil, err
	}
	filtered := filterCustomerPortalConversations(items, req)
	if req.ConversationID > 0 && len(filtered) > 0 {
		if detail, detailErr := s.buildCustomerPortalConversationDetail(external, filtered[0].ID, req.Locale); detailErr == nil && detail != nil {
			filtered[0] = *detail
		}
	}
	return paginateCustomerPortalItems(filtered, req.GetPage(), req.GetLimit())
}

func (s *customerPortalService) HasDeviceAccess(external openidentity.ExternalUser, deviceID int64) (bool, error) {
	if deviceID <= 0 {
		return false, errorsx.InvalidParam("device id is required")
	}
	if external.ExternalSource != enums.ExternalSourceUser {
		return false, errorsx.Unauthorized("a signed-in customer account is required")
	}
	identity := repositories.CustomerIdentityRepository.GetBy(
		sqls.DB(),
		external.ExternalSource,
		strings.TrimSpace(external.ExternalID),
	)
	if identity == nil || identity.CustomerID <= 0 {
		return false, nil
	}
	scope, err := s.resolveScope(external)
	if err != nil {
		return false, err
	}
	return scope.deviceByID[deviceID] != nil, nil
}

func (s *customerPortalService) ListDeviceManuals(external openidentity.ExternalUser, deviceID int64) ([]dto.CustomerPortalManualFileDTO, error) {
	scope, err := s.resolveScope(external)
	if err != nil {
		return nil, err
	}
	device := scope.deviceByID[deviceID]
	if device == nil || device.ProductID <= 0 {
		return nil, errorsx.InvalidParam("customer device not found")
	}
	items, err := ProductManualFileService.ListCustomerManualFiles(device.TenantID, device.ProductID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.CustomerPortalManualFileDTO, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.URL) == "" {
			continue
		}
		result = append(result, dto.CustomerPortalManualFileDTO{
			ID:         item.ID,
			Title:      firstNonBlank(item.Title, item.Filename),
			Filename:   item.Filename,
			FileSize:   item.FileSize,
			MimeType:   item.MimeType,
			URL:        item.URL,
			UploadedAt: item.UploadedAt,
		})
	}
	return result, nil
}

func (s *customerPortalService) ListConversations(external openidentity.ExternalUser) ([]dto.CustomerPortalConversationDTO, error) {
	return s.listConversations(external, "")
}

func (s *customerPortalService) listConversations(external openidentity.ExternalUser, locale string) ([]dto.CustomerPortalConversationDTO, error) {
	scope, err := s.resolveScope(external)
	if err != nil {
		return nil, err
	}
	conversations := s.findVisibleConversations(scope, external)
	productIDs := make([]int64, 0, len(conversations))
	for i := range conversations {
		if conversations[i].ProductID > 0 {
			productIDs = append(productIDs, conversations[i].ProductID)
		}
		if device := scope.deviceByID[conversations[i].DeviceID]; device != nil && device.ProductID > 0 {
			productIDs = append(productIDs, device.ProductID)
		}
	}
	tickets := s.findVisibleTickets(scope, conversations)
	ticketByConversationID := make(map[int64]*models.Ticket)
	for i := range tickets {
		if tickets[i].ConversationID > 0 {
			ticketByConversationID[tickets[i].ConversationID] = chooseCustomerPortalConversationTicket(ticketByConversationID[tickets[i].ConversationID], &tickets[i])
		}
	}
	ticketIDs := make([]string, 0, len(ticketByConversationID))
	userIDs := make([]int64, 0, len(conversations)+len(ticketByConversationID))
	agentIDs := make([]int64, 0, len(conversations))
	for i := range conversations {
		if conversations[i].AIAgentID > 0 {
			agentIDs = append(agentIDs, conversations[i].AIAgentID)
		}
	}
	for _, ticket := range ticketByConversationID {
		if ticket == nil {
			continue
		}
		ticketIDs = append(ticketIDs, strconv.FormatInt(ticket.ID, 10))
		if ticket.CurrentAssigneeID > 0 {
			userIDs = append(userIDs, ticket.CurrentAssigneeID)
		}
	}
	for i := range conversations {
		if conversations[i].CurrentAssigneeID > 0 {
			userIDs = append(userIDs, conversations[i].CurrentAssigneeID)
		}
	}
	products := loadCustomerPortalProductsByID(productIDs)
	users := loadCustomerPortalUsersByID(userIDs)
	agents := AIAgentService.FindByIds(agentIDs)
	agentByID := make(map[int64]*models.AIAgent, len(agents))
	for i := range agents {
		agentByID[agents[i].ID] = &agents[i]
	}
	meetings := repositories.CustomerPortalRepository.FindVisibleMeetings(sqls.DB(), scope.tenantIDs, ticketIDs)
	meetingByTicket := make(map[string]*models.MeetingRoomJitsi)
	for i := range meetings {
		if current := meetingByTicket[meetings[i].TicketID]; current == nil || current.CreatedAt.Before(meetings[i].CreatedAt) {
			meetingByTicket[meetings[i].TicketID] = &meetings[i]
		}
	}
	result := make([]dto.CustomerPortalConversationDTO, 0, len(conversations))
	hasDeviceConceptByTenantID := make(map[int64]bool)
	for _, item := range conversations {
		deviceNo := ""
		productName := ""
		if device := scope.deviceByID[item.DeviceID]; device != nil {
			deviceNo = device.DeviceNo
			if product := products[device.ProductID]; product != nil {
				productName = product.Name
			}
		}
		if productName == "" && item.ProductID > 0 {
			if product := products[item.ProductID]; product != nil && product.TenantID == item.TenantID {
				productName = product.Name
			}
		}
		ticket := ticketByConversationID[item.ID]
		ticketID := int64(0)
		ticketNo := ""
		meetingID := ""
		meetingStatus := ""
		assigneeID := item.CurrentAssigneeID
		if ticket != nil {
			ticketID = ticket.ID
			ticketNo = ticket.TicketNo
			if assigneeID == 0 {
				assigneeID = ticket.CurrentAssigneeID
			}
			if meeting := meetingByTicket[strconv.FormatInt(ticket.ID, 10)]; meeting != nil {
				meetingID = meeting.ID
				meetingStatus = mapCustomerMeetingStatus(meeting.Status)
			}
		}
		assigneeName := ""
		if user := users[assigneeID]; user != nil {
			assigneeName = customerPortalFirstNonEmptyString(user.Nickname, user.Username)
		}
		agent := agentByID[item.AIAgentID]
		lastMessageSummary := item.LastMessageSummary
		if strings.TrimSpace(locale) != "" {
			hasDeviceConcept, ok := hasDeviceConceptByTenantID[item.TenantID]
			if !ok {
				hasDeviceConcept = !TenantCapabilityService.KnowledgeSupport(item.TenantID)
				hasDeviceConceptByTenantID[item.TenantID] = hasDeviceConcept
			}
			lastMessageSummary = i18nx.LocalizeCustomerConversationSummary(locale, lastMessageSummary, hasDeviceConcept)
		}
		result = append(result, dto.CustomerPortalConversationDTO{
			ID:                    item.ID,
			Status:                mapCustomerConversationStatus(item),
			ServiceMode:           int(item.ServiceMode),
			HumanHandoffEnabled:   CustomerConversationAllowsHumanHandoff(&item, agent),
			TicketCreationEnabled: CustomerConversationAllowsTicketCreation(&item, agent),
			Priority:              item.Priority,
			LastMessageSummary:    lastMessageSummary,
			LastMessageAt:         formatCustomerTime(item.LastMessageAt),
			LastActiveAt:          formatCustomerTime(item.LastActiveAt),
			CustomerUnreadCount:   item.CustomerUnreadCount,
			DeviceID:              item.DeviceID,
			DeviceNo:              deviceNo,
			ProductName:           productName,
			CurrentAssigneeID:     assigneeID,
			CurrentAssigneeName:   assigneeName,
			CurrentTicketID:       ticketID,
			CurrentTicketNo:       ticketNo,
			CurrentMeetingID:      meetingID,
			CurrentMeetingStatus:  meetingStatus,
		})
	}
	return result, nil
}

func (s *customerPortalService) TranslateConversationMessage(ctx context.Context, external openidentity.ExternalUser, conversationID int64, req request.TranslateConversationMessageRequest) (*ConversationTranslationResult, error) {
	if conversationID <= 0 {
		return nil, errorsx.InvalidParam("conversation id is required")
	}
	scope, err := s.resolveScope(external)
	if err != nil {
		return nil, err
	}
	conversations := s.findVisibleConversations(scope, external)
	for i := range conversations {
		if conversations[i].ID == conversationID {
			return ConversationTranslationService.Translate(
				ctx,
				conversations[i].TenantID,
				conversations[i].ID,
				req,
				customerPortalConversationOperator(external, scope, &conversations[i]),
			)
		}
	}
	return nil, errorsx.Unauthorized("conversation is not visible to current customer")
}

func (s *customerPortalService) ListTickets(external openidentity.ExternalUser) ([]dto.CustomerPortalTicketDTO, error) {
	scope, err := s.resolveScope(external)
	if err != nil {
		return nil, err
	}
	conversations := s.findVisibleConversations(scope, external)
	tickets := s.findVisibleTickets(scope, conversations)
	meetings := repositories.CustomerPortalRepository.FindVisibleMeetings(sqls.DB(), scope.tenantIDs, collectTicketIDStrings(tickets))
	productIDs := make([]int64, 0, len(tickets))
	userIDs := make([]int64, 0, len(tickets))
	for i := range tickets {
		if tickets[i].ProductID > 0 {
			productIDs = append(productIDs, tickets[i].ProductID)
		}
		if device := scope.deviceByID[tickets[i].DeviceID]; device != nil && device.ProductID > 0 {
			productIDs = append(productIDs, device.ProductID)
		}
		if tickets[i].CurrentAssigneeID > 0 {
			userIDs = append(userIDs, tickets[i].CurrentAssigneeID)
		}
	}
	products := loadCustomerPortalProductsByID(productIDs)
	users := loadCustomerPortalUsersByID(userIDs)
	meetingByTicket := make(map[string]*models.MeetingRoomJitsi)
	for i := range meetings {
		if current := meetingByTicket[meetings[i].TicketID]; current == nil || current.CreatedAt.Before(meetings[i].CreatedAt) {
			meetingByTicket[meetings[i].TicketID] = &meetings[i]
		}
	}
	result := make([]dto.CustomerPortalTicketDTO, 0, len(tickets))
	for _, item := range tickets {
		deviceNo := ""
		productName := ""
		if device := scope.deviceByID[item.DeviceID]; device != nil {
			deviceNo = device.DeviceNo
			if product := products[device.ProductID]; product != nil {
				productName = product.Name
			}
		}
		if productName == "" && item.ProductID > 0 {
			if product := products[item.ProductID]; product != nil && product.TenantID == item.TenantID {
				productName = product.Name
			}
		}
		assigneeName := ""
		if item.CurrentAssigneeID > 0 {
			if user := users[item.CurrentAssigneeID]; user != nil {
				assigneeName = customerPortalFirstNonEmptyString(user.Nickname, user.Username)
			}
		}
		meetingID := ""
		if meeting := meetingByTicket[strconv.FormatInt(item.ID, 10)]; meeting != nil {
			meetingID = meeting.ID
		}
		canConfirm, canReopen, canRate := customerTicketActionFlags(&item, nil)
		result = append(result, dto.CustomerPortalTicketDTO{
			ID:               item.ID,
			TicketNo:         item.TicketNo,
			Title:            item.Title,
			Status:           mapCustomerTicketStatus(item.Status),
			Priority:         DeriveTicketPriority(item),
			DeviceID:         item.DeviceID,
			DeviceNo:         deviceNo,
			ProductName:      productName,
			AssigneeName:     assigneeName,
			CreatedAt:        formatCustomerTime(item.CreatedAt),
			UpdatedAt:        formatCustomerTime(item.UpdatedAt),
			CurrentMeetingID: meetingID,
			RepairSummary:    "",
			Progress:         []dto.CustomerPortalTicketProgressDTO{},
			Feedback:         nil,
			CanConfirm:       canConfirm,
			CanReopen:        canReopen,
			CanRate:          canRate,
		})
	}
	return result, nil
}

func (s *customerPortalService) ListTicketsPage(external openidentity.ExternalUser, req request.CustomerPortalListRequest) ([]dto.CustomerPortalTicketDTO, *sqls.Paging, error) {
	items, err := s.ListTickets(external)
	if err != nil {
		return nil, nil, err
	}
	filtered := filterCustomerPortalTickets(items, req)
	if req.TicketID > 0 && len(filtered) > 0 {
		if detail, detailErr := s.buildCustomerPortalTicketDetail(external, filtered[0].ID); detailErr == nil && detail != nil {
			filtered[0] = *detail
		}
	}
	return paginateCustomerPortalItems(filtered, req.GetPage(), req.GetLimit())
}

func (s *customerPortalService) GetTicketDetail(external openidentity.ExternalUser, ticketID int64) (*dto.CustomerPortalTicketDTO, error) {
	return s.buildCustomerPortalTicketDetail(external, ticketID)
}

func (s *customerPortalService) SubmitTicketFeedback(external openidentity.ExternalUser, ticketID int64, req request.SubmitTicketFeedbackRequest) (*dto.CustomerPortalTicketFeedbackDTO, error) {
	scope, ticket, err := s.resolveVisibleTicket(external, ticketID)
	if err != nil {
		return nil, err
	}
	customerUserID, _ := customerBindingScope(external, scope.customer)
	feedback, err := CustomerTicketActionService.SubmitFeedback(ticket, customerUserID, req, customerPortalOperator(external, scope, ticket))
	if err != nil {
		return nil, err
	}
	return buildCustomerPortalFeedback(feedback), nil
}

func (s *customerPortalService) ConfirmTicketResolved(external openidentity.ExternalUser, ticketID int64) (*dto.CustomerPortalTicketDTO, error) {
	scope, ticket, err := s.resolveVisibleTicket(external, ticketID)
	if err != nil {
		return nil, err
	}
	if err := CustomerTicketActionService.ConfirmResolved(ticket, customerPortalOperator(external, scope, ticket)); err != nil {
		return nil, err
	}
	return s.findTicketDTO(external, ticketID)
}

func (s *customerPortalService) ReopenTicket(external openidentity.ExternalUser, ticketID int64, reason string) (*dto.CustomerPortalTicketDTO, error) {
	scope, ticket, err := s.resolveVisibleTicket(external, ticketID)
	if err != nil {
		return nil, err
	}
	if err := CustomerTicketActionService.Reopen(ticket, reason, customerPortalOperator(external, scope, ticket)); err != nil {
		return nil, err
	}
	return s.findTicketDTO(external, ticketID)
}

func (s *customerPortalService) resolveVisibleTicket(external openidentity.ExternalUser, ticketID int64) (*customerPortalScope, *models.Ticket, error) {
	if ticketID <= 0 {
		return nil, nil, errorsx.InvalidParam("ticket id is required")
	}
	scope, err := s.resolveScope(external)
	if err != nil {
		return nil, nil, err
	}
	conversations := s.findVisibleConversations(scope, external)
	items := s.findVisibleTickets(scope, conversations)
	for i := range items {
		if items[i].ID == ticketID {
			return scope, &items[i], nil
		}
	}
	return nil, nil, errorsx.Unauthorized("ticket is not visible to current customer")
}

func (s *customerPortalService) findTicketDTO(external openidentity.ExternalUser, ticketID int64) (*dto.CustomerPortalTicketDTO, error) {
	items, err := s.ListTickets(external)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == ticketID {
			return &items[i], nil
		}
	}
	return nil, errorsx.InvalidParam("ticket not found")
}

func customerPortalOperator(external openidentity.ExternalUser, scope *customerPortalScope, ticket *models.Ticket) *dto.AuthPrincipal {
	tenantID := int64(0)
	if ticket != nil {
		tenantID = ticket.TenantID
	}
	return customerPortalOperatorForTenant(external, scope, tenantID)
}

func customerPortalConversationOperator(external openidentity.ExternalUser, scope *customerPortalScope, conversation *models.Conversation) *dto.AuthPrincipal {
	tenantID := int64(0)
	if conversation != nil {
		tenantID = conversation.TenantID
	}
	return customerPortalOperatorForTenant(external, scope, tenantID)
}

func customerPortalOperatorForTenant(external openidentity.ExternalUser, scope *customerPortalScope, tenantID int64) *dto.AuthPrincipal {
	userID := int64(0)
	if external.ExternalSource == enums.ExternalSourceUser {
		userID, _ = strconv.ParseInt(strings.TrimSpace(external.ExternalID), 10, 64)
	}
	name := "客户"
	if scope != nil && scope.customer != nil && strings.TrimSpace(scope.customer.Name) != "" {
		name = strings.TrimSpace(scope.customer.Name)
	}
	customerUserID := int64(0)
	customerOrgID := int64(0)
	subjectID := userID
	if scope != nil && scope.customerUser != nil {
		customerUserID = scope.customerUser.ID
		customerOrgID = scope.customerUser.CustomerOrgID
		subjectID = scope.customerUser.ID
	}
	return &dto.AuthPrincipal{
		UserID:         userID,
		Username:       name,
		TenantID:       tenantID,
		DomainType:     models.DomainTypeCustomer,
		Domain:         models.DomainTypeCustomer,
		SubjectType:    models.SubjectTypeCustomerUser,
		SubjectID:      subjectID,
		CustomerUserID: customerUserID,
		CustomerOrgID:  customerOrgID,
		Status:         enums.StatusOk,
	}
}

func buildCustomerPortalFeedback(feedback *models.TicketFeedback) *dto.CustomerPortalTicketFeedbackDTO {
	if feedback == nil {
		return nil
	}
	tags := []string{}
	if strings.TrimSpace(feedback.TagsJSON) != "" {
		_ = json.Unmarshal([]byte(feedback.TagsJSON), &tags)
	}
	return &dto.CustomerPortalTicketFeedbackDTO{
		ID: feedback.ID, Rating: feedback.Rating, Tags: tags, Comment: feedback.Comment, SubmittedAt: formatCustomerTime(feedback.SubmittedAt),
	}
}

func (s *customerPortalService) ListMeetings(external openidentity.ExternalUser) ([]dto.CustomerPortalMeetingDTO, error) {
	scope, err := s.resolveScope(external)
	if err != nil {
		return nil, err
	}
	for _, tenantID := range scope.tenantIDs {
		MeetingService.ReconcileStaleMeetingPresenceForTenant(context.Background(), tenantID, 100)
	}
	conversations := s.findVisibleConversations(scope, external)
	tickets := s.findVisibleTickets(scope, conversations)
	meetings := repositories.CustomerPortalRepository.FindVisibleMeetings(sqls.DB(), scope.tenantIDs, collectTicketIDStrings(tickets))
	productIDs := make([]int64, 0, len(tickets))
	userIDs := make([]int64, 0, len(meetings))
	for i := range tickets {
		if tickets[i].ProductID > 0 {
			productIDs = append(productIDs, tickets[i].ProductID)
		}
		if device := scope.deviceByID[tickets[i].DeviceID]; device != nil && device.ProductID > 0 {
			productIDs = append(productIDs, device.ProductID)
		}
	}
	for i := range meetings {
		if userID, parseErr := strconv.ParseInt(strings.TrimSpace(meetings[i].CreatedBy), 10, 64); parseErr == nil && userID > 0 {
			userIDs = append(userIDs, userID)
		}
	}
	attendanceByMeeting := repositories.MeetingRoomRepository.AttendanceStatsByMeetingIDs(sqls.DB(), collectMeetingIDs(meetings))
	products := loadCustomerPortalProductsByID(productIDs)
	users := loadCustomerPortalUsersByID(userIDs)
	ticketByID := make(map[int64]*models.Ticket)
	for i := range tickets {
		ticketByID[tickets[i].ID] = &tickets[i]
	}
	result := make([]dto.CustomerPortalMeetingDTO, 0, len(meetings))
	for _, item := range meetings {
		ticketID, _ := strconv.ParseInt(item.TicketID, 10, 64)
		ticket := ticketByID[ticketID]
		ticketNo := ""
		deviceNo := ""
		productName := ""
		title := "远程视频协作"
		if ticket != nil {
			ticketNo = ticket.TicketNo
			title = customerPortalFirstNonEmptyString(ticket.Title, title)
			if device := scope.deviceByID[ticket.DeviceID]; device != nil {
				deviceNo = device.DeviceNo
				if product := products[device.ProductID]; product != nil {
					productName = product.Name
				}
			}
			if productName == "" && ticket.ProductID > 0 {
				if product := products[ticket.ProductID]; product != nil && product.TenantID == ticket.TenantID {
					productName = product.Name
				}
			}
		}
		createdBy := strings.TrimSpace(item.CreatedBy)
		if userID, parseErr := strconv.ParseInt(item.CreatedBy, 10, 64); parseErr == nil && userID > 0 {
			if user := users[userID]; user != nil {
				createdBy = customerPortalFirstNonEmptyString(user.Nickname, user.Username, createdBy)
			}
		}
		attendance := attendanceByMeeting[item.ID]
		result = append(result, dto.CustomerPortalMeetingDTO{
			ID:       item.ID,
			TicketID: ticketID,
			TicketNo: ticketNo,
			Title:    title,
			Status:   mapCustomerMeetingStatus(item.Status),
			DeviceID: func() int64 {
				if ticket != nil {
					return ticket.DeviceID
				}
				return 0
			}(),
			DeviceNo:         deviceNo,
			ProductName:      productName,
			CreatedBy:        createdBy,
			ScheduledAt:      customerPortalFirstNonEmptyString(formatCustomerTimePtr(item.ScheduledAt), formatCustomerTime(item.CreatedAt)),
			StartedAt:        formatCustomerTimePtr(item.StartedAt),
			EndedAt:          formatCustomerTimePtr(item.EndedAt),
			DurationSeconds:  meetingDurationSeconds(item.StartedAt, item.EndedAt),
			ParticipantCount: attendance.Count,
			Participants:     []dto.MeetingParticipantDTO{},
			TranscriptCount:  0,
			AnnotationCount:  0,
			JoinPath:         fmt.Sprintf("/api/customer/v1/meetings/%s/join", item.ID),
			RoomName:         item.RoomName,
		})
	}
	return result, nil
}

func (s *customerPortalService) ListMeetingsPage(external openidentity.ExternalUser, req request.CustomerPortalListRequest) ([]dto.CustomerPortalMeetingDTO, *sqls.Paging, error) {
	items, err := s.ListMeetings(external)
	if err != nil {
		return nil, nil, err
	}
	filtered := filterCustomerPortalMeetings(items, req)
	if req.MeetingID != "" && len(filtered) > 0 {
		if detail, detailErr := s.buildCustomerPortalMeetingDetail(external, filtered[0].ID); detailErr == nil && detail != nil {
			filtered[0] = *detail
		}
	}
	return paginateCustomerPortalItems(filtered, req.GetPage(), req.GetLimit())
}

func (s *customerPortalService) JoinMeeting(ctx context.Context, external openidentity.ExternalUser, meetingID string) (*JoinConfig, error) {
	scope, meeting, err := s.resolveVisibleMeeting(external, meetingID)
	if err != nil {
		return nil, err
	}
	return MeetingService.JoinMeeting(ctx, meetingID, strconv.FormatInt(scope.customer.ID, 10), scope.customer.Name, "customer", meeting.TenantID)
}

func (s *customerPortalService) ConfirmMeetingJoined(ctx context.Context, external openidentity.ExternalUser, meetingID string) error {
	scope, meeting, err := s.resolveVisibleMeeting(external, meetingID)
	if err != nil {
		return err
	}
	if isMeetingEndedStatus(meeting.Status) {
		return meetingEndedError()
	}
	return MeetingService.confirmParticipantJoin(
		meeting.ID,
		strconv.FormatInt(scope.customer.ID, 10),
		"customer",
		scope.customer.Name,
		time.Now(),
	)
}

func (s *customerPortalService) ConfirmMeetingLeft(ctx context.Context, external openidentity.ExternalUser, meetingID string) error {
	scope, meeting, err := s.resolveVisibleMeeting(external, meetingID)
	if err != nil {
		return err
	}
	return MeetingService.confirmParticipantLeave(
		meeting,
		strconv.FormatInt(scope.customer.ID, 10),
		"customer",
		time.Now(),
	)
}

func (s *customerPortalService) HeartbeatMeeting(ctx context.Context, external openidentity.ExternalUser, meetingID string) error {
	scope, meeting, err := s.resolveVisibleMeeting(external, meetingID)
	if err != nil {
		return err
	}
	return MeetingService.heartbeatParticipant(
		meeting, strconv.FormatInt(scope.customer.ID, 10), "customer", time.Now(),
	)
}

func (s *customerPortalService) GetMeetingStatus(ctx context.Context, external openidentity.ExternalUser, meetingID string) (*MeetingRoomStatus, error) {
	_, meeting, err := s.resolveVisibleMeeting(external, meetingID)
	if err != nil {
		return nil, err
	}
	return MeetingService.GetMeetingStatus(ctx, meeting.ID, meeting.TenantID)
}

func (s *customerPortalService) ListMeetingTranscripts(external openidentity.ExternalUser, meetingID string) ([]models.MeetingTranscriptSegment, error) {
	_, meeting, err := s.resolveVisibleMeeting(external, meetingID)
	if err != nil {
		return nil, err
	}
	return repositories.MeetingIntelligenceRepository.ListTranscripts(sqls.DB(), meeting.TenantID, meeting.ID, 100)
}

func (s *customerPortalService) ListMeetingTranscriptPage(external openidentity.ExternalUser, meetingID, cursor string, limit int) ([]models.MeetingTranscriptSegment, string, bool, error) {
	_, meeting, err := s.resolveVisibleMeeting(external, meetingID)
	if err != nil {
		return nil, "", false, err
	}
	return MeetingIntelligenceService.listTranscriptPage(meeting.TenantID, meeting.ID, cursor, limit)
}

func (s *customerPortalService) IngestMeetingTranscript(ctx context.Context, external openidentity.ExternalUser, meetingID string, input request.MeetingTranscriptIngestRequest) (*models.MeetingTranscriptSegment, error) {
	scope, meeting, err := s.resolveVisibleMeeting(external, meetingID)
	if err != nil {
		return nil, err
	}
	return MeetingIntelligenceService.IngestClientTranscript(ctx, meeting, MeetingTranscriptSpeaker{
		ParticipantID: strconv.FormatInt(scope.customer.ID, 10),
		Name:          firstNonEmptyString(strings.TrimSpace(scope.customer.Name), "Customer"),
		Language:      input.Language,
	}, input)
}

func (s *customerPortalService) resolveVisibleMeeting(external openidentity.ExternalUser, meetingID string) (*customerPortalScope, *models.MeetingRoomJitsi, error) {
	meetingID = strings.TrimSpace(meetingID)
	if meetingID == "" {
		return nil, nil, errorsx.InvalidParam("meeting id is required")
	}
	scope, err := s.resolveScope(external)
	if err != nil {
		return nil, nil, err
	}
	if len(scope.tenantIDs) == 0 {
		return nil, nil, errorsx.Unauthorized("meeting is not visible to current customer")
	}
	var meeting models.MeetingRoomJitsi
	if err := sqls.DB().Where("id = ? AND tenant_id IN ?", meetingID, scope.tenantIDs).First(&meeting).Error; err != nil {
		return nil, nil, errorsx.Unauthorized("meeting is not visible to current customer")
	}
	ticketID, err := strconv.ParseInt(strings.TrimSpace(meeting.TicketID), 10, 64)
	if err != nil || ticketID <= 0 {
		return nil, nil, errorsx.Unauthorized("meeting is not visible to current customer")
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil || ticket.TenantID != meeting.TenantID || ticket.CustomerID != scope.customer.ID {
		return nil, nil, errorsx.Unauthorized("meeting is not visible to current customer")
	}
	visible := ticket.DeviceID > 0 && scope.deviceByID[ticket.DeviceID] != nil
	if !visible && ticket.ConversationID > 0 {
		conversation := repositories.ConversationRepository.Get(sqls.DB(), ticket.ConversationID)
		visible = conversation != nil &&
			conversation.TenantID == ticket.TenantID &&
			conversation.CustomerID == scope.customer.ID &&
			ConversationService.IsCustomerConversationOwner(conversation, external)
	}
	if !visible {
		return nil, nil, errorsx.Unauthorized("meeting is not visible to current customer")
	}
	return scope, &meeting, nil
}

func (s *customerPortalService) resolveScope(external openidentity.ExternalUser) (*customerPortalScope, error) {
	if external.ExternalSource != enums.ExternalSourceUser {
		return nil, errorsx.Unauthorized("a signed-in customer account is required")
	}
	identity := repositories.CustomerIdentityRepository.GetBy(sqls.DB(), external.ExternalSource, strings.TrimSpace(external.ExternalID))
	if identity == nil || identity.CustomerID <= 0 {
		ensured, err := s.ensureAccountCustomerIdentity(external)
		if err != nil {
			return nil, err
		}
		identity = ensured
	}
	if identity == nil || identity.CustomerID <= 0 {
		return nil, errorsx.Unauthorized("customer identity is not available")
	}
	customer := repositories.CustomerRepository.Get(sqls.DB(), identity.CustomerID)
	if customer == nil || customer.Status == enums.StatusDeleted {
		return nil, errorsx.Unauthorized("customer is not available")
	}
	bindingCustomerUserID, bindingCustomerOrgID := customerBindingScope(external, customer)
	var formalUser *models.CustomerUser
	if accountUserID, parseErr := strconv.ParseInt(strings.TrimSpace(external.ExternalID), 10, 64); parseErr == nil && accountUserID > 0 {
		formalUser = repositories.CustomerPortalRepository.FindCustomerUserByUserID(sqls.DB(), accountUserID)
	}
	bindings := make([]models.CustomerDeviceBinding, 0)
	if sqls.DB().Migrator().HasTable(&models.CustomerDeviceBinding{}) {
		bindings = repositories.CustomerPortalRepository.FindActiveBindings(sqls.DB(), bindingCustomerUserID, bindingCustomerOrgID)
	}
	deviceIDs := make([]int64, 0, len(bindings))
	tenantIDs := make([]int64, 0, len(bindings))
	seen := make(map[int64]bool)
	tenantSeen := make(map[int64]bool)
	if formalUser != nil && formalUser.TenantID > 0 {
		tenantSeen[formalUser.TenantID] = true
		tenantIDs = append(tenantIDs, formalUser.TenantID)
	}
	for _, item := range bindings {
		if item.TenantID > 0 && !tenantSeen[item.TenantID] {
			tenantSeen[item.TenantID] = true
			tenantIDs = append(tenantIDs, item.TenantID)
		}
		if item.DeviceID <= 0 || seen[item.DeviceID] {
			continue
		}
		seen[item.DeviceID] = true
		deviceIDs = append(deviceIDs, item.DeviceID)
	}
	devices := repositories.CustomerPortalRepository.FindDevicesByTenantAndIDs(sqls.DB(), tenantIDs, deviceIDs)
	deviceByID := make(map[int64]*models.Device, len(devices))
	for i := range devices {
		deviceByID[devices[i].ID] = &devices[i]
		if devices[i].TenantID > 0 && !tenantSeen[devices[i].TenantID] {
			tenantSeen[devices[i].TenantID] = true
			tenantIDs = append(tenantIDs, devices[i].TenantID)
		}
	}
	return &customerPortalScope{
		customer:     customer,
		customerUser: formalUser,
		devices:      devices,
		deviceByID:   deviceByID,
		deviceIDs:    deviceIDs,
		tenantIDs:    tenantIDs,
	}, nil
}

func (s *customerPortalService) RequireServiceInteractionAccess(external openidentity.ExternalUser, tenantIDs ...int64) error {
	if external.ExternalSource != enums.ExternalSourceUser {
		return nil
	}
	for _, tenantID := range tenantIDs {
		if TenantCapabilityService.KnowledgeSupport(tenantID) {
			return nil
		}
	}
	scope, err := s.resolveScope(external)
	if err != nil {
		return err
	}
	if scope == nil || scope.customer == nil {
		return errorsx.Unauthorized("customer identity is not available")
	}
	if scope.customerUser != nil || len(scope.devices) > 0 || scope.customer.CompanyID > 0 {
		return nil
	}
	return errorsx.Forbidden("bind a device before asking for support")
}

func (s *customerPortalService) ensureAccountCustomerIdentity(external openidentity.ExternalUser) (*models.CustomerIdentity, error) {
	if external.ExternalSource != enums.ExternalSourceUser {
		return nil, nil
	}
	externalID := strings.TrimSpace(external.ExternalID)
	accountUserID, err := strconv.ParseInt(externalID, 10, 64)
	if err != nil || accountUserID <= 0 {
		return nil, nil
	}
	user := repositories.UserRepository.Get(sqls.DB(), accountUserID)
	if user == nil || user.Status != enums.StatusOk {
		return nil, errorsx.Unauthorized("customer account is not available")
	}
	displayName := strings.TrimSpace(user.Nickname)
	if displayName == "" {
		displayName = strings.TrimSpace(user.Username)
	}
	if displayName == "" {
		displayName = external.ExternalName
	}

	var identity *models.CustomerIdentity
	if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		customerID, err := CustomerService.EnsureExternalCustomer(ctx, openidentity.ExternalUser{
			ExternalSource: enums.ExternalSourceUser,
			ExternalID:     externalID,
			ExternalName:   displayName,
		})
		if err != nil {
			return err
		}
		updates := map[string]interface{}{}
		if email := strings.TrimSpace(userEmail(user)); email != "" {
			updates["primary_email"] = email
		}
		if mobile := strings.TrimSpace(userMobile(user)); mobile != "" {
			updates["primary_mobile"] = mobile
		}
		if len(updates) > 0 {
			updates["updated_at"] = time.Now()
			if err := repositories.CustomerRepository.Updates(ctx.Tx, customerID, updates); err != nil {
				return err
			}
		}
		identity = repositories.CustomerIdentityRepository.GetBy(ctx.Tx, enums.ExternalSourceUser, externalID)
		if identity == nil || identity.CustomerID <= 0 {
			return errorsx.Unauthorized("customer identity is not available")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return identity, nil
}

func (s *customerPortalService) findVisibleConversations(scope *customerPortalScope, external openidentity.ExternalUser) []models.Conversation {
	if scope == nil || scope.customer == nil {
		return nil
	}
	items := repositories.CustomerPortalRepository.FindVisibleConversations(
		sqls.DB(),
		scope.tenantIDs,
		scope.customer.ID,
		CustomerParticipantExternalIDs(external),
	)
	result := make([]models.Conversation, 0, len(items))
	for i := range items {
		if ConversationService.IsCustomerConversationOwner(&items[i], external) {
			result = append(result, items[i])
			if items[i].TenantID > 0 && !containsCustomerPortalTenantID(scope.tenantIDs, items[i].TenantID) {
				scope.tenantIDs = append(scope.tenantIDs, items[i].TenantID)
			}
		}
	}
	return result
}

func containsCustomerPortalTenantID(items []int64, target int64) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func (s *customerPortalService) findVisibleTickets(scope *customerPortalScope, conversations []models.Conversation) []models.Ticket {
	if scope == nil || scope.customer == nil {
		return nil
	}
	conversationIDs := make([]int64, 0, len(conversations))
	for _, item := range conversations {
		if item.ID > 0 {
			conversationIDs = append(conversationIDs, item.ID)
		}
	}
	unscopedTenantIDs := make([]int64, 0, len(scope.tenantIDs))
	for _, tenantID := range scope.tenantIDs {
		if TenantCapabilityService.KnowledgeSupport(tenantID) {
			unscopedTenantIDs = append(unscopedTenantIDs, tenantID)
		}
	}
	return repositories.CustomerPortalRepository.FindVisibleTickets(
		sqls.DB(),
		scope.tenantIDs,
		unscopedTenantIDs,
		scope.customer.ID,
		scope.deviceIDs,
		conversationIDs,
	)
}

func chooseCustomerPortalConversationTicket(current *models.Ticket, candidate *models.Ticket) *models.Ticket {
	if candidate == nil {
		return current
	}
	if current == nil {
		return candidate
	}
	currentOpen := isOpenTicketStatus(current.Status)
	candidateOpen := isOpenTicketStatus(candidate.Status)
	if !currentOpen && candidateOpen {
		return candidate
	}
	if currentOpen == candidateOpen && candidate.UpdatedAt.After(current.UpdatedAt) {
		return candidate
	}
	return current
}

func loadCustomerPortalProductsByID(productIDs []int64) map[int64]*models.Product {
	return loadCustomerPortalProductsByIDCached(productIDs)
}

func loadCustomerPortalProductModelsByID(productModelIDs []int64) map[int64]*models.ProductModel {
	return loadCustomerPortalProductModelsByIDCached(productModelIDs)
}

func loadCustomerPortalUsersByID(userIDs []int64) map[int64]*models.User {
	return loadCustomerPortalUsersByIDCached(userIDs)
}

func loadCustomerPortalManualCountByProductID(productIDs []int64) map[int64]int {
	return loadCustomerPortalManualCountByProductIDCached(productIDs)
}

func loadCustomerPortalLatestWarrantyByDeviceID(deviceIDs []int64) map[int64]*models.DeviceWarrantyRecord {
	return loadCustomerPortalLatestWarrantyByDeviceIDCached(deviceIDs)
}

func uniqueCustomerPortalInt64s(values []int64) []int64 {
	if len(values) == 0 {
		return []int64{}
	}
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (s *customerPortalService) buildCustomerPortalDeviceDetail(external openidentity.ExternalUser, deviceID int64) (*dto.CustomerPortalDeviceDTO, error) {
	if deviceID <= 0 {
		return nil, errorsx.InvalidParam("device id is required")
	}
	scope, err := s.resolveScope(external)
	if err != nil {
		return nil, err
	}
	device := scope.deviceByID[deviceID]
	if device == nil {
		return nil, errorsx.InvalidParam("customer device not found")
	}
	products := loadCustomerPortalProductsByID([]int64{device.ProductID})
	productModels := loadCustomerPortalProductModelsByID([]int64{device.ProductModelID})
	manualCountByProductID := loadCustomerPortalManualCountByProductID([]int64{device.ProductID})
	warrantyByDeviceID := loadCustomerPortalLatestWarrantyByDeviceID([]int64{device.ID})
	productName, productCode := "", ""
	if product := products[device.ProductID]; product != nil {
		productName = product.Name
		productCode = product.Code
	}
	modelName := ""
	if model := productModels[device.ProductModelID]; model != nil {
		modelName = model.Name
	}
	warrantyEndAt := ""
	if warranty := warrantyByDeviceID[device.ID]; warranty != nil {
		warrantyEndAt = formatCustomerTime(warranty.EndAt)
	}
	return &dto.CustomerPortalDeviceDTO{
		ID:                 device.ID,
		DeviceNo:           device.DeviceNo,
		SerialNo:           device.SerialNo,
		ProductName:        productName,
		ProductCode:        productCode,
		ModelName:          modelName,
		RegionCode:         device.RegionCode,
		Status:             mapCustomerDeviceStatus(*device),
		LastServiceAt:      formatCustomerTimePtr(device.LastServiceAt),
		InstalledAt:        formatCustomerTimePtr(device.InstalledAt),
		WarrantyEndAt:      warrantyEndAt,
		ManualCount:        manualCountByProductID[device.ProductID],
		RepairHistoryCount: 0,
		OpenTicketCount:    0,
		ConversationCount:  0,
	}, nil
}

func (s *customerPortalService) buildCustomerPortalTicketDetail(external openidentity.ExternalUser, ticketID int64) (*dto.CustomerPortalTicketDTO, error) {
	scope, ticket, err := s.resolveVisibleTicket(external, ticketID)
	if err != nil {
		return nil, err
	}
	productIDs := []int64{ticket.ProductID}
	if device := scope.deviceByID[ticket.DeviceID]; device != nil && device.ProductID > 0 {
		productIDs = append(productIDs, device.ProductID)
	}
	products := loadCustomerPortalProductsByID(productIDs)
	users := loadCustomerPortalUsersByID([]int64{ticket.CurrentAssigneeID})
	deviceNo := ""
	productName := ""
	if device := scope.deviceByID[ticket.DeviceID]; device != nil {
		deviceNo = device.DeviceNo
		if product := products[device.ProductID]; product != nil {
			productName = product.Name
		}
	}
	if productName == "" && ticket.ProductID > 0 {
		if product := products[ticket.ProductID]; product != nil && product.TenantID == ticket.TenantID {
			productName = product.Name
		}
	}
	assigneeName := ""
	if user := users[ticket.CurrentAssigneeID]; user != nil {
		assigneeName = customerPortalFirstNonEmptyString(user.Nickname, user.Username)
	}
	progress := repositories.TicketProgressRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", ticket.TenantID).
		Eq("ticket_id", ticket.ID).
		Where("(visible_to_customer = ? OR event_type IN ? OR metadata_json LIKE ?)", true, customerPortalVisibleProgressEventTypes, "%\"source\":\"partner\"%").
		Asc("created_at").
		Asc("id"))
	progressDTO := make([]dto.CustomerPortalTicketProgressDTO, 0, len(progress))
	for _, row := range progress {
		progressDTO = append(progressDTO, buildCustomerPortalTicketProgressDTO(row))
	}
	progressDTO = buildCustomerTicketProgressFallback(*ticket, assigneeName, progressDTO)
	repairSummary := ""
	if repair := latestTicketRepair(ticket.ID); repair != nil {
		repairSummary = customerPortalFirstNonEmptyString(strings.TrimSpace(repair.Solution), strings.TrimSpace(repair.Conclusion), strings.TrimSpace(repair.RepairMethod))
	}
	feedback := repositories.TicketFeedbackRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", ticket.TenantID).
		Eq("ticket_id", ticket.ID).
		Desc("submitted_at").
		Desc("id"))
	canConfirm, canReopen, canRate := customerTicketActionFlags(ticket, feedback)
	meetingID := ""
	meetings := repositories.CustomerPortalRepository.FindVisibleMeetings(sqls.DB(), scope.tenantIDs, []string{strconv.FormatInt(ticket.ID, 10)})
	var latestMeeting *models.MeetingRoomJitsi
	for i := range meetings {
		if latestMeeting == nil || meetings[i].CreatedAt.After(latestMeeting.CreatedAt) {
			latestMeeting = &meetings[i]
			meetingID = meetings[i].ID
		}
	}
	return &dto.CustomerPortalTicketDTO{
		ID:               ticket.ID,
		TicketNo:         ticket.TicketNo,
		Title:            ticket.Title,
		Status:           mapCustomerTicketStatus(ticket.Status),
		Priority:         DeriveTicketPriority(*ticket),
		DeviceID:         ticket.DeviceID,
		DeviceNo:         deviceNo,
		ProductName:      productName,
		AssigneeName:     assigneeName,
		CreatedAt:        formatCustomerTime(ticket.CreatedAt),
		UpdatedAt:        formatCustomerTime(ticket.UpdatedAt),
		CurrentMeetingID: meetingID,
		RepairSummary:    repairSummary,
		Progress:         progressDTO,
		Feedback:         buildCustomerPortalFeedback(feedback),
		CanConfirm:       canConfirm,
		CanReopen:        canReopen,
		CanRate:          canRate,
	}, nil
}

func (s *customerPortalService) buildCustomerPortalMeetingDetail(external openidentity.ExternalUser, meetingID string) (*dto.CustomerPortalMeetingDTO, error) {
	scope, meeting, err := s.resolveVisibleMeeting(external, meetingID)
	if err != nil {
		return nil, err
	}
	var ticket *models.Ticket
	if ticketID, parseErr := strconv.ParseInt(meeting.TicketID, 10, 64); parseErr == nil && ticketID > 0 {
		_, ticket, _ = s.resolveVisibleTicket(external, ticketID)
	}
	productIDs := make([]int64, 0, 2)
	if ticket != nil && ticket.ProductID > 0 {
		productIDs = append(productIDs, ticket.ProductID)
	}
	if ticket != nil {
		if device := scope.deviceByID[ticket.DeviceID]; device != nil && device.ProductID > 0 {
			productIDs = append(productIDs, device.ProductID)
		}
	}
	products := loadCustomerPortalProductsByID(productIDs)
	var users map[int64]*models.User
	if userID, parseErr := strconv.ParseInt(meeting.CreatedBy, 10, 64); parseErr == nil && userID > 0 {
		users = loadCustomerPortalUsersByID([]int64{userID})
	}
	ticketNo := ""
	deviceNo := ""
	productName := ""
	title := "远程视频协作"
	if ticket != nil {
		ticketNo = ticket.TicketNo
		title = customerPortalFirstNonEmptyString(ticket.Title, title)
		if device := scope.deviceByID[ticket.DeviceID]; device != nil {
			deviceNo = device.DeviceNo
			if product := products[device.ProductID]; product != nil {
				productName = product.Name
			}
		}
		if productName == "" && ticket.ProductID > 0 {
			if product := products[ticket.ProductID]; product != nil && product.TenantID == ticket.TenantID {
				productName = product.Name
			}
		}
	}
	createdBy := strings.TrimSpace(meeting.CreatedBy)
	if userID, parseErr := strconv.ParseInt(meeting.CreatedBy, 10, 64); parseErr == nil && userID > 0 {
		if user := users[userID]; user != nil {
			createdBy = customerPortalFirstNonEmptyString(user.Nickname, user.Username, createdBy)
		}
	}
	attendance := repositories.MeetingRoomRepository.AttendanceStats(sqls.DB(), meeting.ID)
	confirmedStartedAt := confirmedMeetingStartedAt(meeting, attendance)
	participantsByMeeting, archiveStatsByMeeting := loadMeetingArchiveData(sqls.DB(), []string{meeting.ID})
	participants := participantsByMeeting[meeting.ID]
	if participants == nil {
		participants = []dto.MeetingParticipantDTO{}
	}
	archiveStats := archiveStatsByMeeting[meeting.ID]
	return &dto.CustomerPortalMeetingDTO{
		ID: meeting.ID,
		TicketID: func() int64 {
			if ticket != nil {
				return ticket.ID
			}
			return 0
		}(),
		TicketNo: ticketNo,
		Title:    title,
		Status:   mapCustomerMeetingStatus(meeting.Status),
		DeviceID: func() int64 {
			if ticket != nil {
				return ticket.DeviceID
			}
			return 0
		}(),
		DeviceNo:         deviceNo,
		ProductName:      productName,
		CreatedBy:        createdBy,
		ScheduledAt:      customerPortalFirstNonEmptyString(formatCustomerTimePtr(meeting.ScheduledAt), formatCustomerTime(meeting.CreatedAt)),
		StartedAt:        formatCustomerTimePtr(confirmedStartedAt),
		EndedAt:          formatCustomerTimePtr(meeting.EndedAt),
		DurationSeconds:  meetingDurationSeconds(confirmedStartedAt, meeting.EndedAt),
		ParticipantCount: attendance.Count,
		Participants:     participants,
		TranscriptCount:  archiveStats.TranscriptCount,
		AnnotationCount:  archiveStats.AnnotationCount,
		JoinPath:         fmt.Sprintf("/api/customer/v1/meetings/%s/join", meeting.ID),
		RoomName:         meeting.RoomName,
	}, nil
}

func (s *customerPortalService) buildCustomerPortalConversationDetail(external openidentity.ExternalUser, conversationID int64, locale string) (*dto.CustomerPortalConversationDTO, error) {
	if conversationID <= 0 {
		return nil, errorsx.InvalidParam("conversation id is required")
	}
	scope, err := s.resolveScope(external)
	if err != nil {
		return nil, err
	}
	conversations := s.findVisibleConversations(scope, external)
	var conversation *models.Conversation
	for i := range conversations {
		if conversations[i].ID == conversationID {
			conversation = &conversations[i]
			break
		}
	}
	if conversation == nil {
		return nil, errorsx.Unauthorized("conversation is not visible to current customer")
	}
	productIDs := []int64{}
	if conversation.ProductID > 0 {
		productIDs = append(productIDs, conversation.ProductID)
	}
	if device := scope.deviceByID[conversation.DeviceID]; device != nil && device.ProductID > 0 {
		productIDs = append(productIDs, device.ProductID)
	}
	products := loadCustomerPortalProductsByID(productIDs)
	tickets := s.findVisibleTickets(scope, []models.Conversation{*conversation})
	var ticket *models.Ticket
	for i := range tickets {
		if tickets[i].ConversationID != conversation.ID {
			continue
		}
		ticket = chooseCustomerPortalConversationTicket(ticket, &tickets[i])
	}
	ticketID := int64(0)
	ticketNo := ""
	meetingID := ""
	meetingStatus := ""
	assigneeID := conversation.CurrentAssigneeID
	if ticket != nil {
		ticketID = ticket.ID
		ticketNo = ticket.TicketNo
		if assigneeID == 0 {
			assigneeID = ticket.CurrentAssigneeID
		}
		meetings := repositories.CustomerPortalRepository.FindVisibleMeetings(sqls.DB(), scope.tenantIDs, []string{strconv.FormatInt(ticket.ID, 10)})
		var latestMeeting *models.MeetingRoomJitsi
		for i := range meetings {
			if latestMeeting == nil || meetings[i].CreatedAt.After(latestMeeting.CreatedAt) {
				latestMeeting = &meetings[i]
				meetingID = meetings[i].ID
				meetingStatus = mapCustomerMeetingStatus(meetings[i].Status)
			}
		}
	}
	users := loadCustomerPortalUsersByID([]int64{assigneeID})
	deviceNo := ""
	productName := ""
	if device := scope.deviceByID[conversation.DeviceID]; device != nil {
		deviceNo = device.DeviceNo
		if product := products[device.ProductID]; product != nil {
			productName = product.Name
		}
	}
	if productName == "" && conversation.ProductID > 0 {
		if product := products[conversation.ProductID]; product != nil && product.TenantID == conversation.TenantID {
			productName = product.Name
		}
	}
	assigneeName := ""
	if user := users[assigneeID]; user != nil {
		assigneeName = customerPortalFirstNonEmptyString(user.Nickname, user.Username)
	}
	agent := AIAgentService.Get(conversation.AIAgentID)
	lastMessageSummary := conversation.LastMessageSummary
	if strings.TrimSpace(locale) != "" {
		lastMessageSummary = i18nx.LocalizeCustomerConversationSummary(
			locale,
			lastMessageSummary,
			!TenantCapabilityService.KnowledgeSupport(conversation.TenantID),
		)
	}
	return &dto.CustomerPortalConversationDTO{
		ID:                    conversation.ID,
		Status:                mapCustomerConversationStatus(*conversation),
		ServiceMode:           int(conversation.ServiceMode),
		HumanHandoffEnabled:   CustomerConversationAllowsHumanHandoff(conversation, agent),
		TicketCreationEnabled: CustomerConversationAllowsTicketCreation(conversation, agent),
		Priority:              conversation.Priority,
		LastMessageSummary:    lastMessageSummary,
		LastMessageAt:         formatCustomerTime(conversation.LastMessageAt),
		LastActiveAt:          formatCustomerTime(conversation.LastActiveAt),
		CustomerUnreadCount:   conversation.CustomerUnreadCount,
		DeviceID:              conversation.DeviceID,
		DeviceNo:              deviceNo,
		ProductName:           productName,
		CurrentAssigneeID:     assigneeID,
		CurrentAssigneeName:   assigneeName,
		CurrentTicketID:       ticketID,
		CurrentTicketNo:       ticketNo,
		CurrentMeetingID:      meetingID,
		CurrentMeetingStatus:  meetingStatus,
	}, nil
}

func formatCustomerTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func formatCustomerTimePtr(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func isOpenTicketStatus(status enums.TicketStatus) bool {
	return status != enums.TicketStatusClosed && status != enums.TicketStatusDone && status != enums.TicketStatusCancelled
}

func mapCustomerConversationStatus(item models.Conversation) string {
	if item.ClosedAt != nil {
		return "closed"
	}
	switch item.Status {
	case enums.IMConversationStatusPending:
		return "queued"
	case enums.IMConversationStatusActive:
		return "human_serving"
	case enums.IMConversationStatusAIServing:
		return "ai_serving"
	default:
		return "waiting_customer"
	}
}

func mapCustomerTicketStatus(status enums.TicketStatus) string {
	switch enums.NormalizeTicketStatus(string(status)) {
	case enums.TicketStatusDraft, enums.TicketStatusPendingAcceptance:
		return "submitted"
	case enums.TicketStatusPendingDispatch:
		return "pending_dispatch"
	case enums.TicketStatusPendingAssigneeAccept:
		return "pending_assignee_accept"
	case enums.TicketStatusAccepted:
		return "accepted"
	case enums.TicketStatusProcessing, enums.TicketStatusVideoSupport:
		return "processing"
	case enums.TicketStatusPendingCustomerConfirm:
		return "action_required"
	case enums.TicketStatusResolved:
		return "pending_confirmation"
	case enums.TicketStatusClosed:
		return "closed"
	case enums.TicketStatusQualityReview:
		return "reviewing"
	case enums.TicketStatusReopened:
		return "reopened"
	case enums.TicketStatusCancelled:
		return "cancelled"
	default:
		return "processing"
	}
}

func buildCustomerTicketProgressFallback(item models.Ticket, assigneeName string, progress []dto.CustomerPortalTicketProgressDTO) []dto.CustomerPortalTicketProgressDTO {
	if len(progress) > 0 {
		return progress
	}
	content := ""
	eventType := enums.TicketProgressEventCreated
	switch enums.NormalizeTicketStatus(string(item.Status)) {
	case enums.TicketStatusPendingDispatch:
		content = "服务工单已创建，正在安排工程师。"
	case enums.TicketStatusPendingAssigneeAccept:
		if strings.TrimSpace(assigneeName) != "" {
			content = "已分配给" + strings.TrimSpace(assigneeName) + "，等待工程师接单。"
		} else {
			content = "已分配工程师，等待接单。"
		}
		eventType = enums.TicketProgressEventAssigned
	case enums.TicketStatusAccepted:
		content = "工程师已接单，正在准备处理。"
		eventType = enums.TicketProgressEventAccepted
	case enums.TicketStatusProcessing, enums.TicketStatusVideoSupport, enums.TicketStatusSupplierSupport, enums.TicketStatusInProgress:
		content = "工程师正在处理该工单。"
		eventType = enums.TicketProgressEventProcessing
	case enums.TicketStatusResolved, enums.TicketStatusPendingCustomerConfirm, enums.TicketStatusQualityReview:
		content = "维修结论已提交，等待确认。"
		eventType = enums.TicketProgressEventRepairCompleted
	case enums.TicketStatusClosed:
		content = "工单已关闭。"
		eventType = enums.TicketProgressEventClosed
	case enums.TicketStatusCancelled:
		content = "工单已取消。"
		eventType = enums.TicketProgressEventClosed
	case enums.TicketStatusReopened:
		content = "工单已重新打开，正在继续处理。"
		eventType = enums.TicketProgressEventReopened
	default:
		content = "服务工单已创建。"
	}
	return []dto.CustomerPortalTicketProgressDTO{{
		EventType: string(eventType),
		Content:   content,
		CreatedAt: formatCustomerTime(firstNonZeroTime(item.UpdatedAt, item.CreatedAt)),
	}}
}

func buildCustomerPortalTicketProgressDTO(row models.TicketProgress) dto.CustomerPortalTicketProgressDTO {
	return dto.CustomerPortalTicketProgressDTO{
		ID:        row.ID,
		EventType: string(row.EventType),
		Content:   customerPortalTicketProgressContent(row),
		CreatedAt: formatCustomerTime(row.CreatedAt),
		Metadata:  customerPortalTicketProgressMetadata(row.MetadataJSON),
	}
}

func customerPortalTicketProgressMetadata(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	metadata := make(map[string]string, len(payload))
	for key, value := range payload {
		key = strings.TrimSpace(key)
		if key == "" || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				metadata[key] = trimmed
			}
		case float64, bool:
			metadata[key] = fmt.Sprint(typed)
		}
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func customerPortalTicketProgressContent(row models.TicketProgress) string {
	content := strings.TrimSpace(row.Content)
	fallback := customerPortalTicketProgressFallbackContent(row.EventType)
	if content == "" {
		return fallback
	}
	if strings.HasPrefix(content, "状态流转：") {
		if statusContent := customerPortalTicketProgressStatusContent(row); statusContent != "" {
			return statusContent
		}
		if fallback != "" {
			return fallback
		}
	}
	if strings.EqualFold(content, "Created ticket") {
		if fallback != "" {
			return fallback
		}
	}
	return content
}

func customerPortalTicketProgressStatusContent(row models.TicketProgress) string {
	var payload struct {
		ToStatus string `json:"to_status"`
	}
	if err := json.Unmarshal([]byte(row.MetadataJSON), &payload); err != nil {
		return ""
	}
	switch enums.NormalizeTicketStatus(payload.ToStatus) {
	case enums.TicketStatusPendingDispatch:
		return "工单已进入派单流程"
	case enums.TicketStatusPendingAssigneeAccept:
		return "已分配工程师，等待接单"
	case enums.TicketStatusAccepted:
		return "工程师已接单"
	case enums.TicketStatusProcessing, enums.TicketStatusVideoSupport, enums.TicketStatusSupplierSupport, enums.TicketStatusInProgress:
		return "工程师正在处理该工单"
	case enums.TicketStatusResolved, enums.TicketStatusPendingCustomerConfirm, enums.TicketStatusQualityReview:
		return "维修结论已提交，等待确认"
	case enums.TicketStatusClosed:
		return "工单已关闭"
	case enums.TicketStatusCancelled:
		return "工单已取消"
	case enums.TicketStatusReopened:
		return "工单已重新打开，正在继续处理"
	default:
		return ""
	}
}

func customerPortalTicketProgressFallbackContent(eventType enums.TicketProgressEventType) string {
	switch eventType {
	case enums.TicketProgressEventCreated:
		return "已创建工单"
	case enums.TicketProgressEventAccepted:
		return "工程师已接单"
	case enums.TicketProgressEventAssigned:
		return "已分配处理人员"
	case enums.TicketProgressEventProcessing:
		return "工单处理中"
	case enums.TicketProgressEventEscalated:
		return "已升级协作处理"
	case enums.TicketProgressEventRepairCompleted:
		return "已填写维修记录"
	case enums.TicketProgressEventClosed:
		return "工单已关闭"
	case enums.TicketProgressEventReopened:
		return "工单已重新打开"
	case enums.TicketProgressEventMeetingEnded:
		return "视频协作已结束"
	default:
		return "工单进度已更新"
	}
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func mapCustomerMeetingStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "active":
		return "active"
	case "ended":
		return "finished"
	default:
		return "waiting"
	}
}

func mapCustomerDeviceStatus(item models.Device) string {
	switch strings.TrimSpace(item.DeviceStatus) {
	case string(enums.DeviceStatusOperational):
		return "online"
	case string(enums.DeviceStatusUnderMaintenance):
		return "maintenance"
	case string(enums.DeviceStatusDecommissioned):
		return "offline"
	default:
		return "idle"
	}
}

func latestWarranty(deviceID int64) *models.DeviceWarrantyRecord {
	records := repositories.DeviceWarrantyRecordRepository.FindByDeviceID(sqls.DB(), deviceID)
	if len(records) == 0 {
		return nil
	}
	return &records[0]
}

func latestTicketRepair(ticketID int64) *models.TicketRepairRecord {
	if ticketID <= 0 {
		return nil
	}
	return repositories.TicketRepairRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("ticket_id", ticketID).
		Eq("visible_to_customer", true).
		Desc("finished_at").
		Desc("id"))
}

func countCustomerVisibleManuals(productID, productModelID int64) int {
	if productID <= 0 {
		return 0
	}
	items := repositories.ProductManualFileRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("product_id", productID).
		Where("(visibility = ? OR visibility = '')", "public").
		Where("status <> ?", enums.StatusDeleted))
	_ = productModelID
	return len(items)
}

func collectTicketIDStrings(tickets []models.Ticket) []string {
	result := make([]string, 0, len(tickets))
	for _, item := range tickets {
		if item.ID > 0 {
			result = append(result, strconv.FormatInt(item.ID, 10))
		}
	}
	return result
}

func collectMeetingIDs(meetings []models.MeetingRoomJitsi) []string {
	result := make([]string, 0, len(meetings))
	for _, item := range meetings {
		if strings.TrimSpace(item.ID) != "" {
			result = append(result, item.ID)
		}
	}
	return result
}

func buildCustomerPortalMetrics(profile *dto.CustomerPortalProfileDTO) []dto.CustomerPortalMetricDTO {
	if profile == nil {
		return nil
	}
	return []dto.CustomerPortalMetricDTO{
		{Key: "devices", Label: "我的设备", Value: strconv.Itoa(profile.BoundDeviceCount), Meta: "已绑定范围", Tone: "blue"},
		{Key: "conversations", Label: "当前会话", Value: strconv.Itoa(profile.ActiveConversationCount), Meta: "继续沟通", Tone: "green"},
		{Key: "tickets", Label: "处理中工单", Value: strconv.Itoa(profile.OpenTicketCount), Meta: "待我处理提醒", Tone: "amber"},
		{Key: "meetings", Label: "待加入会议", Value: strconv.Itoa(profile.UpcomingMeetingCount), Meta: "视频协作", Tone: "slate"},
	}
}

func filterCustomerPortalDeviceMetrics(metrics []dto.CustomerPortalMetricDTO) []dto.CustomerPortalMetricDTO {
	result := make([]dto.CustomerPortalMetricDTO, 0, len(metrics))
	for _, metric := range metrics {
		if metric.Key != "devices" {
			result = append(result, metric)
		}
	}
	return result
}

func paginateCustomerPortalItems[T any](items []T, page, limit int) ([]T, *sqls.Paging, error) {
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	total := len(items)
	start := (page - 1) * limit
	if start >= total {
		return []T{}, &sqls.Paging{Page: page, Limit: limit, Total: int64(total)}, nil
	}
	end := start + limit
	if end > total {
		end = total
	}
	return items[start:end], &sqls.Paging{Page: page, Limit: limit, Total: int64(total)}, nil
}

func filterCustomerPortalDevices(items []dto.CustomerPortalDeviceDTO, req request.CustomerPortalListRequest) []dto.CustomerPortalDeviceDTO {
	keyword := strings.ToLower(strings.TrimSpace(req.Keyword))
	if req.DeviceID <= 0 && keyword == "" {
		return items
	}
	filtered := make([]dto.CustomerPortalDeviceDTO, 0, len(items))
	for _, item := range items {
		if req.DeviceID > 0 && item.ID != req.DeviceID {
			continue
		}
		if keyword != "" {
			if !strings.Contains(strings.ToLower(item.DeviceNo), keyword) &&
				!strings.Contains(strings.ToLower(item.SerialNo), keyword) &&
				!strings.Contains(strings.ToLower(item.ProductName), keyword) &&
				!strings.Contains(strings.ToLower(item.ProductCode), keyword) &&
				!strings.Contains(strings.ToLower(item.ModelName), keyword) &&
				!strings.Contains(strings.ToLower(item.RegionCode), keyword) {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func filterCustomerPortalConversations(items []dto.CustomerPortalConversationDTO, req request.CustomerPortalListRequest) []dto.CustomerPortalConversationDTO {
	keyword := strings.ToLower(strings.TrimSpace(req.Keyword))
	filter := strings.TrimSpace(req.Filter)
	if req.ConversationID <= 0 && keyword == "" && filter == "" {
		return items
	}
	filtered := make([]dto.CustomerPortalConversationDTO, 0, len(items))
	for _, item := range items {
		if req.ConversationID > 0 && item.ID != req.ConversationID {
			continue
		}
		if filter != "" {
			closed := item.Status == "closed"
			switch filter {
			case "active":
				if closed {
					continue
				}
			case "closed":
				if !closed {
					continue
				}
			}
		}
		if keyword != "" {
			if !strings.Contains(strings.ToLower(item.DeviceNo), keyword) &&
				!strings.Contains(strings.ToLower(item.ProductName), keyword) &&
				!strings.Contains(strings.ToLower(item.LastMessageSummary), keyword) &&
				!strings.Contains(strings.ToLower(item.CurrentAssigneeName), keyword) &&
				!strings.Contains(strings.ToLower(item.CurrentTicketNo), keyword) {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func filterCustomerPortalTickets(items []dto.CustomerPortalTicketDTO, req request.CustomerPortalListRequest) []dto.CustomerPortalTicketDTO {
	keyword := strings.ToLower(strings.TrimSpace(req.Keyword))
	filter := strings.TrimSpace(req.Filter)
	if req.TicketID <= 0 && keyword == "" && filter == "" && req.DeviceID <= 0 && req.ConversationID <= 0 {
		return items
	}
	filtered := make([]dto.CustomerPortalTicketDTO, 0, len(items))
	for _, item := range items {
		if req.TicketID > 0 && item.ID != req.TicketID {
			continue
		}
		if req.DeviceID > 0 && item.DeviceID != req.DeviceID {
			continue
		}
		if filter != "" {
			closed := item.Status == "closed" || item.Status == "done" || item.Status == "cancelled"
			actionRequired := item.Status == "action_required" || item.Status == "pending_confirmation"
			switch filter {
			case "open":
				if closed {
					continue
				}
			case "active":
				if closed || actionRequired {
					continue
				}
			case "action":
				if !actionRequired {
					continue
				}
			case "closed":
				if !closed {
					continue
				}
			}
		}
		if keyword != "" {
			if !strings.Contains(strings.ToLower(item.TicketNo), keyword) &&
				!strings.Contains(strings.ToLower(item.Title), keyword) &&
				!strings.Contains(strings.ToLower(item.DeviceNo), keyword) &&
				!strings.Contains(strings.ToLower(item.ProductName), keyword) &&
				!strings.Contains(strings.ToLower(item.AssigneeName), keyword) {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func filterCustomerPortalMeetings(items []dto.CustomerPortalMeetingDTO, req request.CustomerPortalListRequest) []dto.CustomerPortalMeetingDTO {
	keyword := strings.ToLower(strings.TrimSpace(req.Keyword))
	filter := strings.TrimSpace(req.Filter)
	if req.MeetingID == "" && keyword == "" && filter == "" && req.DeviceID <= 0 && req.TicketID <= 0 {
		return items
	}
	filtered := make([]dto.CustomerPortalMeetingDTO, 0, len(items))
	for _, item := range items {
		if req.MeetingID != "" && item.ID != req.MeetingID {
			continue
		}
		if req.TicketID > 0 && item.TicketID != req.TicketID {
			continue
		}
		if req.DeviceID > 0 && item.DeviceID != req.DeviceID {
			continue
		}
		if filter != "" {
			upcoming := item.Status == "scheduled" || item.Status == "waiting" || item.Status == "active"
			switch filter {
			case "upcoming":
				if !upcoming {
					continue
				}
			case "history":
				if upcoming {
					continue
				}
			}
		}
		if keyword != "" {
			if !strings.Contains(strings.ToLower(item.TicketNo), keyword) &&
				!strings.Contains(strings.ToLower(item.Title), keyword) &&
				!strings.Contains(strings.ToLower(item.DeviceNo), keyword) &&
				!strings.Contains(strings.ToLower(item.ProductName), keyword) &&
				!strings.Contains(strings.ToLower(item.CreatedBy), keyword) {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func firstPortalDevices(items []dto.CustomerPortalDeviceDTO, limit int) []dto.CustomerPortalDeviceDTO {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func firstActiveCustomerConversation(items []dto.CustomerPortalConversationDTO) *dto.CustomerPortalConversationDTO {
	for i := range items {
		if items[i].Status != "closed" {
			return &items[i]
		}
	}
	return nil
}

func firstPendingPortalTickets(items []dto.CustomerPortalTicketDTO, limit int) []dto.CustomerPortalTicketDTO {
	result := make([]dto.CustomerPortalTicketDTO, 0, min(len(items), limit))
	for i := range items {
		if items[i].Status == "closed" || items[i].Status == "cancelled" {
			continue
		}
		result = append(result, items[i])
		if len(result) == limit {
			break
		}
	}
	return result
}

func firstUpcomingCustomerMeeting(items []dto.CustomerPortalMeetingDTO) *dto.CustomerPortalMeetingDTO {
	for i := range items {
		if items[i].Status == "active" || items[i].Status == "waiting" {
			return &items[i]
		}
	}
	return nil
}

func customerPortalFirstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

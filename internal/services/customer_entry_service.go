package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var CustomerEntryService = newCustomerEntryService()

type customerEntryService struct{}

type CustomerEntrySessionAggregate struct {
	Session          *models.CustomerEntrySession
	Tenant           *models.Tenant
	Product          *models.Product
	ProductModel     *models.ProductModel
	Device           *models.Device
	ServiceCodeValue string
	VisitorToken     string
	PrivacyConsent   *models.CustomerPrivacyConsent
}

type CustomerPrivacyConsentMetadata struct {
	IPAddress string
	UserAgent string
	RequestID string
}

func newCustomerEntryService() *customerEntryService { return &customerEntryService{} }

func (s *customerEntryService) CreateEntrySession(req request.CreateCustomerEntrySessionRequest) (*CustomerEntrySessionAggregate, error) {
	serviceCodeValue := strings.TrimSpace(req.ServiceCode)
	deviceNo := strings.TrimSpace(req.DeviceNo)
	if serviceCodeValue == "" && deviceNo == "" {
		return nil, errorsx.InvalidParam("serviceCode or deviceNo is required")
	}

	var tenant *models.Tenant
	var product *models.Product
	var productModel *models.ProductModel
	var device *models.Device
	var serviceCode *models.ServiceCode
	if serviceCodeValue != "" {
		resolved, err := ServiceCodeResolveService.Resolve(serviceCodeValue)
		if err != nil {
			return nil, err
		}
		if !resolved.Valid {
			return nil, errorsx.InvalidParam(resolved.Reason)
		}
		tenant, product, productModel, device, serviceCode = resolved.Tenant, resolved.Product, resolved.ProductModel, resolved.Device, resolved.ServiceCode
	} else {
		device = repositories.DeviceRepository.GetByDeviceNo(sqls.DB(), deviceNo)
		if device == nil || device.Status != enums.StatusOk {
			return nil, errorsx.InvalidParam("device is not available")
		}
		tenant = repositories.TenantRepository.Get(sqls.DB(), device.TenantID)
		product = repositories.ProductRepository.Get(sqls.DB(), device.ProductID)
		if tenant == nil || tenant.Status != enums.StatusOk || product == nil || product.Status != enums.StatusOk || product.TenantID != tenant.ID {
			return nil, errorsx.InvalidParam("device context is not available")
		}
		if device.ProductModelID > 0 {
			productModel = repositories.ProductModelRepository.Get(sqls.DB(), device.ProductModelID)
		}
		if productModel != nil && (productModel.Status != enums.StatusOk || productModel.TenantID != tenant.ID || productModel.ProductID != product.ID) {
			return nil, errorsx.InvalidParam("device model is not available")
		}
		serviceCode = repositories.ServiceCodeRepository.GetActiveByDeviceID(sqls.DB(), device.ID)
	}
	if serviceCode != nil && device == nil && deviceNo != "" {
		registeredDevice, err := s.registerGeneralDevice(serviceCode, deviceNo, req.SerialNo, req.RegionCode, req.CustomerUserID)
		if err != nil {
			return nil, err
		}
		device = registeredDevice
	}

	locale := strings.TrimSpace(req.Locale)
	if locale == "" {
		locale = TenantPortalSettingsService.CustomerDefaultLocaleDB(sqls.DB(), tenant)
		if locale == "" {
			locale = product.DefaultLocale
		}
	}
	visitorID := strings.TrimSpace(req.VisitorID)
	if visitorID == "" {
		generatedVisitorID, err := generateCustomerEntrySecret()
		if err != nil {
			return nil, err
		}
		visitorID = "visitor_" + generatedVisitorID
	}
	visitorToken := strings.TrimSpace(req.VisitorToken)
	if visitorToken != "" && (len(visitorToken) < 32 || len(visitorToken) > 256) {
		return nil, errorsx.InvalidParam("visitorToken must contain 32 to 256 characters")
	}
	if visitorToken == "" {
		var err error
		visitorToken, err = generateCustomerEntrySecret()
		if err != nil {
			return nil, err
		}
	}
	visitorTokenHash := hashCustomerEntrySecret(visitorToken)
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)
	serviceCodeValue = serviceCodeValueOrEmpty(serviceCode, serviceCodeValue)

	if existing := repositories.CustomerEntrySessionRepository.FindActiveByVisitorTokenHash(sqls.DB(), visitorTokenHash, now); existing != nil {
		if !matchesCustomerEntrySession(existing, tenant.ID, product.ID, modelID(productModel), deviceID(device), serviceCodeID(serviceCode), visitorID) {
			return nil, errorsx.InvalidParam("visitor credential is already used by another entry session")
		}
		return buildCustomerEntrySessionAggregate(existing, tenant, product, productModel, device, serviceCodeValue, visitorToken), nil
	}

	context := map[string]any{
		"tenantId": tenant.ID, "productId": product.ID, "productModelId": modelID(productModel), "deviceId": deviceID(device),
		"serviceCodeId": serviceCodeID(serviceCode), "serviceCode": serviceCodeValue, "visitorId": visitorID, "locale": locale,
	}
	contextJSON, err := json.Marshal(context)
	if err != nil {
		return nil, errorsx.BusinessError(1, "failed to build entry context")
	}
	item := &models.CustomerEntrySession{
		TenantID:         tenant.ID,
		ProductID:        product.ID,
		ProductModelID:   modelID(productModel),
		EntryType:        "qr",
		ServiceCodeID:    serviceCodeID(serviceCode),
		DeviceID:         deviceID(device),
		CustomerUserID:   req.CustomerUserID,
		VisitorID:        visitorID,
		Locale:           locale,
		State:            "active",
		EntryContextJSON: string(contextJSON),
		VisitorTokenHash: visitorTokenHash,
		ExpiresAt:        &expiresAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	created := false
	if err := sqls.WithTransaction(func(txCtx *sqls.TxContext) error {
		if serviceCode != nil && serviceCode.DeviceID == 0 && serviceCode.Mode == enums.ServiceCodeModeGeneral {
			if err := s.ensureGeneralRegistrationTask(txCtx.Tx, serviceCode, req.CustomerUserID, now, expiresAt); err != nil {
				return err
			}
		}
		var err error
		created, err = repositories.CustomerEntrySessionRepository.CreateIfAbsent(txCtx.Tx, item)
		return err
	}); err != nil {
		return nil, err
	}
	if !created {
		existing := repositories.CustomerEntrySessionRepository.FindActiveByVisitorTokenHash(sqls.DB(), visitorTokenHash, now)
		if existing == nil {
			return nil, errorsx.InvalidParam("visitor credential is already used by another entry session")
		}
		if !matchesCustomerEntrySession(existing, tenant.ID, product.ID, modelID(productModel), deviceID(device), serviceCodeID(serviceCode), visitorID) {
			return nil, errorsx.InvalidParam("visitor credential is already used by another entry session")
		}
		return buildCustomerEntrySessionAggregate(existing, tenant, product, productModel, device, serviceCodeValue, visitorToken), nil
	}
	return buildCustomerEntrySessionAggregate(item, tenant, product, productModel, device, serviceCodeValue, visitorToken), nil
}

func (s *customerEntryService) registerGeneralDevice(
	serviceCode *models.ServiceCode,
	deviceNo, serialNo, regionCode string,
	customerUserID int64,
) (*models.Device, error) {
	if serviceCode == nil || serviceCode.ID <= 0 || serviceCode.Mode != enums.ServiceCodeModeGeneral {
		return nil, errorsx.InvalidParam("service code cannot register a device")
	}
	deviceNo = normalizeCode(deviceNo)
	if deviceNo == "" {
		return nil, errorsx.InvalidParam("deviceNo is required")
	}

	var registered *models.Device
	err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		var locked models.ServiceCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id = ?", serviceCode.ID).Error; err != nil {
			return errorsx.InvalidParam("service code is not available")
		}
		if locked.TenantID != serviceCode.TenantID || locked.ProductID != serviceCode.ProductID {
			return errorsx.InvalidParam("service code scope changed")
		}
		if locked.DeviceID > 0 {
			device := repositories.DeviceRepository.Get(tx, locked.DeviceID)
			if device == nil || device.Status != enums.StatusOk {
				return errorsx.InvalidParam("registered device is not available")
			}
			if device.DeviceNo != deviceNo {
				return errorsx.InvalidParam("service code is already bound to another device")
			}
			registered = device
			return nil
		}
		if locked.Status != enums.ServiceCodeStatusActive {
			return errorsx.InvalidParam("service code cannot be bound in current status")
		}
		if repositories.DeviceRepository.GetByTenantDeviceNo(tx, locked.TenantID, deviceNo) != nil {
			return errorsx.InvalidParam("deviceNo is already registered; use its assigned service code")
		}

		customerOrgID := int64(0)
		createUserID := int64(0)
		if customerUserID > 0 && tx.Migrator().HasTable(&models.CustomerUser{}) {
			customerUser := repositories.EnterpriseIAMRepository.GetCustomerUser(tx, locked.TenantID, customerUserID)
			if customerUser == nil || customerUser.Status != enums.StatusOk {
				return errorsx.InvalidParam("customer user does not belong to service code tenant")
			}
			customerOrgID = customerUser.CustomerOrgID
			createUserID = customerUser.UserID
		}
		now := time.Now()
		registered = &models.Device{
			TenantID: locked.TenantID, DeviceNo: deviceNo, ProductID: locked.ProductID,
			ProductModelID: locked.ProductModelID, SerialNo: strings.TrimSpace(serialNo),
			CustomerOrgID: customerOrgID, RegionCode: strings.TrimSpace(regionCode),
			InstallLocationJSON: "{}", Source: "customer_registration", Status: enums.StatusOk,
			DeviceStatus: "new", MetadataJSON: "{}",
			AuditFields: models.AuditFields{
				CreatedAt: now, UpdatedAt: now, CreateUserID: createUserID, UpdateUserID: createUserID,
				CreateUserName: "customer_portal", UpdateUserName: "customer_portal",
			},
		}
		if err := repositories.DeviceRepository.Create(tx, registered); err != nil {
			return err
		}
		if err := repositories.ServiceCodeRepository.Updates(tx, locked.ID, map[string]any{
			"status": enums.ServiceCodeStatusBound, "bound_at": &now, "device_id": registered.ID,
			"product_id": registered.ProductID, "product_model_id": registered.ProductModelID,
			"update_user_id": createUserID, "update_user_name": "customer_portal", "updated_at": now,
		}); err != nil {
			return err
		}

		task := repositories.DeviceRegistrationTaskRepository.FindByServiceCodeID(tx, locked.ID)
		if task == nil {
			task = &models.DeviceRegistrationTask{
				TenantID: locked.TenantID, ServiceCodeID: locked.ID, ServiceCodeVal: locked.ServiceCode,
				ProductID: locked.ProductID, ProductModelID: locked.ProductModelID,
				AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
			}
			if err := repositories.DeviceRegistrationTaskRepository.Create(tx, task); err != nil {
				return err
			}
		}
		return repositories.DeviceRegistrationTaskRepository.Updates(tx, task.ID, map[string]any{
			"customer_user_id": customerUserID, "customer_org_id": customerOrgID,
			"device_no": registered.DeviceNo, "serial_no": registered.SerialNo,
			"registration_data_json": "{}", "status": "completed", "completed_at": &now,
			"updated_at": now,
		})
	})
	if err != nil {
		return nil, err
	}
	serviceCode.DeviceID = registered.ID
	serviceCode.Status = enums.ServiceCodeStatusBound
	serviceCode.ProductModelID = registered.ProductModelID
	return registered, nil
}

func (s *customerEntryService) ensureGeneralRegistrationTask(
	db *gorm.DB,
	serviceCode *models.ServiceCode,
	customerUserID int64,
	now, expiresAt time.Time,
) error {
	existing := repositories.DeviceRegistrationTaskRepository.FindByServiceCodeID(db, serviceCode.ID)
	if existing == nil {
		_, err := repositories.DeviceRegistrationTaskRepository.CreateIfAbsent(db, &models.DeviceRegistrationTask{
			TenantID:       serviceCode.TenantID,
			ServiceCodeID:  serviceCode.ID,
			ServiceCodeVal: serviceCode.ServiceCode,
			ProductID:      serviceCode.ProductID,
			ProductModelID: serviceCode.ProductModelID,
			CustomerUserID: customerUserID,
			Status:         "pending",
			ExpiresAt:      &expiresAt,
			AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
		})
		return err
	}
	if existing.Status != "completed" && existing.Status != "expired" {
		return nil
	}
	return repositories.DeviceRegistrationTaskRepository.Updates(db, existing.ID, map[string]any{
		"tenant_id":              serviceCode.TenantID,
		"service_code_val":       serviceCode.ServiceCode,
		"product_id":             serviceCode.ProductID,
		"product_model_id":       serviceCode.ProductModelID,
		"customer_user_id":       customerUserID,
		"customer_org_id":        0,
		"device_no":              "",
		"serial_no":              "",
		"registration_data_json": "{}",
		"status":                 "pending",
		"completed_at":           nil,
		"expires_at":             &expiresAt,
		"updated_at":             now,
	})
}

func buildCustomerEntrySessionAggregate(
	session *models.CustomerEntrySession,
	tenant *models.Tenant,
	product *models.Product,
	productModel *models.ProductModel,
	device *models.Device,
	serviceCodeValue string,
	visitorToken string,
) *CustomerEntrySessionAggregate {
	return &CustomerEntrySessionAggregate{
		Session:          session,
		Tenant:           tenant,
		Product:          product,
		ProductModel:     productModel,
		Device:           device,
		ServiceCodeValue: serviceCodeValue,
		VisitorToken:     visitorToken,
		PrivacyConsent: repositories.CustomerPrivacyConsentRepository.FindLatestAccepted(
			sqls.DB(), session.ID, constants.CustomerPrivacyPolicyVersion,
		),
	}
}

func (s *customerEntryService) GetPrivacyConsent(entrySessionID int64, visitorID, visitorToken string) (*models.CustomerPrivacyConsent, error) {
	session, err := s.requireVisitorEntrySession(sqls.DB(), entrySessionID, visitorID, visitorToken)
	if err != nil {
		return nil, err
	}
	return repositories.CustomerPrivacyConsentRepository.FindLatestAccepted(
		sqls.DB(), session.ID, constants.CustomerPrivacyPolicyVersion,
	), nil
}

func (s *customerEntryService) ConfirmPrivacyConsent(
	req request.ConfirmCustomerPrivacyConsentRequest,
	visitorID, visitorToken string,
	metadata CustomerPrivacyConsentMetadata,
) (*models.CustomerPrivacyConsent, error) {
	if !req.RequiredAccepted {
		return nil, errorsx.InvalidParam("required privacy processing must be accepted")
	}
	if strings.TrimSpace(req.PolicyVersion) != constants.CustomerPrivacyPolicyVersion {
		return nil, errorsx.InvalidParam("privacy notice version is no longer current")
	}
	session, err := s.requireVisitorEntrySession(sqls.DB(), req.EntrySessionID, visitorID, visitorToken)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	receiptSource := fmt.Sprintf(
		"%d|%d|%s|%t|%t|%t|%s|%d",
		session.ID,
		session.TenantID,
		constants.CustomerPrivacyPolicyVersion,
		req.RequiredAccepted,
		req.AnalyticsAccepted,
		req.MarketingAccepted,
		strings.TrimSpace(session.VisitorID),
		now.UnixNano(),
	)
	receiptHash := sha256.Sum256([]byte(receiptSource))
	item := &models.CustomerPrivacyConsent{
		TenantID:          session.TenantID,
		ProductID:         session.ProductID,
		EntrySessionID:    session.ID,
		VisitorID:         session.VisitorID,
		PolicyVersion:     constants.CustomerPrivacyPolicyVersion,
		RequiredAccepted:  true,
		AnalyticsAccepted: req.AnalyticsAccepted,
		MarketingAccepted: req.MarketingAccepted,
		Locale:            session.Locale,
		IPAddress:         strings.TrimSpace(metadata.IPAddress),
		UserAgent:         strings.TrimSpace(metadata.UserAgent),
		RequestID:         strings.TrimSpace(metadata.RequestID),
		ReceiptHash:       hex.EncodeToString(receiptHash[:]),
		ConsentedAt:       now,
		CreatedAt:         now,
	}
	if err := repositories.CustomerPrivacyConsentRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *customerEntryService) RequirePrivacyConsent(session *models.CustomerEntrySession) error {
	return s.requirePrivacyConsent(sqls.DB(), session)
}

func (s *customerEntryService) requirePrivacyConsent(db *gorm.DB, session *models.CustomerEntrySession) error {
	if session == nil || repositories.CustomerPrivacyConsentRepository.FindLatestAccepted(
		db, session.ID, constants.CustomerPrivacyPolicyVersion,
	) == nil {
		return errorsx.Forbidden("privacy consent is required")
	}
	return nil
}

func (s *customerEntryService) requireVisitorEntrySession(db *gorm.DB, entrySessionID int64, visitorID, visitorToken string) (*models.CustomerEntrySession, error) {
	if entrySessionID <= 0 || strings.TrimSpace(visitorID) == "" || strings.TrimSpace(visitorToken) == "" {
		return nil, errorsx.InvalidParam("entrySessionId and visitor credential are required")
	}
	session := repositories.CustomerEntrySessionRepository.FindActive(db, entrySessionID, time.Now())
	if session == nil {
		return nil, errorsx.InvalidParam("entry session is inactive or expired")
	}
	if session.CustomerUserID <= 0 {
		return nil, errorsx.Unauthorized("a signed-in customer account is required")
	}
	if session.VisitorID != strings.TrimSpace(visitorID) || !s.VerifyVisitorToken(session, visitorToken) {
		return nil, errorsx.Unauthorized("customer entry session is invalid")
	}
	return session, nil
}

func matchesCustomerEntrySession(
	session *models.CustomerEntrySession,
	tenantID, productID, productModelID, deviceID, serviceCodeID int64,
	visitorID string,
) bool {
	return session != nil &&
		session.TenantID == tenantID &&
		session.ProductID == productID &&
		session.ProductModelID == productModelID &&
		session.DeviceID == deviceID &&
		session.ServiceCodeID == serviceCodeID &&
		session.VisitorID == visitorID
}

func generateCustomerEntrySecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func hashCustomerEntrySecret(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func (s *customerEntryService) VerifyVisitorToken(session *models.CustomerEntrySession, visitorToken string) bool {
	if session == nil || strings.TrimSpace(session.VisitorTokenHash) == "" || strings.TrimSpace(visitorToken) == "" {
		return false
	}
	expected := []byte(strings.TrimSpace(session.VisitorTokenHash))
	actual := []byte(hashCustomerEntrySecret(visitorToken))
	return len(expected) == len(actual) && subtle.ConstantTimeCompare(expected, actual) == 1
}

func (s *customerEntryService) ConfirmDeviceBinding(req request.ConfirmCustomerDeviceBindingRequest) (*models.CustomerDeviceBinding, error) {
	if req.EntrySessionID <= 0 || strings.TrimSpace(req.VisitorID) == "" || strings.TrimSpace(req.VisitorToken) == "" || (req.CustomerUserID <= 0 && req.CustomerOrgID <= 0) {
		return nil, errorsx.InvalidParam("entrySessionId, visitor credential and customer are required")
	}
	var binding *models.CustomerDeviceBinding
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		now := time.Now()
		session := repositories.CustomerEntrySessionRepository.FindActive(ctx.Tx, req.EntrySessionID, now)
		if session == nil {
			return errorsx.InvalidParam("entry session is inactive or expired")
		}
		if session.VisitorID != strings.TrimSpace(req.VisitorID) || !s.VerifyVisitorToken(session, req.VisitorToken) {
			return errorsx.Unauthorized("customer entry session is invalid")
		}
		if session.CustomerUserID <= 0 || session.CustomerUserID != req.CustomerUserID {
			return errorsx.Unauthorized("a signed-in customer account is required")
		}
		device := repositories.DeviceRepository.Get(ctx.Tx, session.DeviceID)
		if device == nil || device.Status != enums.StatusOk {
			return errorsx.InvalidParam("device is not available")
		}

		// 租户一致性校验
		tenant := repositories.TenantRepository.Get(ctx.Tx, session.TenantID)
		if tenant == nil || tenant.Status != enums.StatusOk {
			return errorsx.InvalidParam("tenant is not available")
		}

		// 校验 device 与 session 中的上下文一致
		if device.TenantID != session.TenantID {
			return errorsx.InvalidParam("device tenant mismatch")
		}
		if req.CustomerUserID > 0 && ctx.Tx.Migrator().HasTable(&models.CustomerUser{}) && repositories.EnterpriseIAMRepository.GetCustomerUser(ctx.Tx, session.TenantID, req.CustomerUserID) == nil {
			return errorsx.InvalidParam("customer user does not belong to device tenant")
		}
		if device.CustomerOrgID > 0 && req.CustomerOrgID > 0 && device.CustomerOrgID != req.CustomerOrgID {
			return errorsx.Forbidden("device is assigned to another customer organization")
		}
		if device.CustomerOrgID == 0 && req.CustomerOrgID > 0 {
			if err := repositories.DeviceRepository.Updates(ctx.Tx, device.ID, map[string]any{
				"customer_org_id": req.CustomerOrgID,
				"updated_at":      now,
			}); err != nil {
				return err
			}
			device.CustomerOrgID = req.CustomerOrgID
		}

		// 校验 session 关联的 serviceCode（如果有）的租户一致性
		if session.ServiceCodeID > 0 {
			serviceCode := repositories.ServiceCodeRepository.Get(ctx.Tx, session.ServiceCodeID)
			if serviceCode != nil {
				if serviceCode.TenantID != session.TenantID {
					return errorsx.InvalidParam("serviceCode tenant mismatch")
				}
				if serviceCode.DeviceID > 0 && serviceCode.DeviceID != device.ID {
					return errorsx.InvalidParam("serviceCode device mismatch")
				}
				// 原子更新 serviceCode 状态为 bound
				if serviceCode.Status != enums.ServiceCodeStatusBound {
					affected, err := repositories.ServiceCodeRepository.UpdateStatusIf(ctx.Tx, serviceCode.ID, string(enums.ServiceCodeStatusActive), map[string]any{
						"status":           enums.ServiceCodeStatusBound,
						"bound_at":         &now,
						"device_id":        device.ID,
						"product_id":       device.ProductID,
						"product_model_id": device.ProductModelID,
						"updated_at":       now,
					})
					if err != nil {
						return err
					}
					if affected == 0 {
						current := repositories.ServiceCodeRepository.Get(ctx.Tx, serviceCode.ID)
						if current != nil && current.Status != enums.ServiceCodeStatusActive {
							return errorsx.InvalidParam("service code cannot be bound in current status")
						}
					}
				}
			}
		}

		binding = repositories.CustomerDeviceBindingRepository.FindByCustomer(ctx.Tx, session.TenantID, device.ID, req.CustomerUserID, req.CustomerOrgID)
		columns := map[string]any{"customer_user_id": req.CustomerUserID, "customer_org_id": req.CustomerOrgID, "binding_role": strings.TrimSpace(req.BindingRole), "status": enums.StatusOk, "confirmed_at": now, "updated_at": now}
		if binding != nil {
			return repositories.CustomerDeviceBindingRepository.Updates(ctx.Tx, binding.ID, columns)
		}
		binding = &models.CustomerDeviceBinding{TenantID: session.TenantID, DeviceID: device.ID, CustomerUserID: req.CustomerUserID, CustomerOrgID: req.CustomerOrgID, BindingRole: strings.TrimSpace(req.BindingRole), Source: "customer_entry", Status: enums.StatusOk, ConfirmedAt: &now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
		return repositories.CustomerDeviceBindingRepository.Create(ctx.Tx, binding)
	})
	if err != nil {
		return nil, err
	}
	return binding, nil
}

func (s *customerEntryService) GetActiveEntryContext(id int64) (map[string]any, error) {
	if id <= 0 {
		return nil, errorsx.InvalidParam("entrySessionId is required")
	}
	item := repositories.CustomerEntrySessionRepository.FindActive(sqls.DB(), id, time.Now())
	if item == nil {
		return nil, errorsx.InvalidParam("entry session is inactive or expired")
	}
	if item.CustomerUserID <= 0 {
		return nil, errorsx.Unauthorized("a signed-in customer account is required")
	}
	context := map[string]any{}
	if err := json.Unmarshal([]byte(item.EntryContextJSON), &context); err != nil {
		return nil, errorsx.BusinessError(1, "entry context is invalid")
	}
	context["entrySessionId"] = item.ID
	return context, nil
}

func (s *customerEntryService) GetEntrySessionContext(entrySessionID int64, visitorID, visitorToken string) (*dto.CustomerEntryContextDTO, error) {
	if entrySessionID <= 0 || strings.TrimSpace(visitorID) == "" || strings.TrimSpace(visitorToken) == "" {
		return nil, errorsx.InvalidParam("entrySessionId and visitor credential are required")
	}
	session := repositories.CustomerEntrySessionRepository.FindActive(sqls.DB(), entrySessionID, time.Now())
	if session == nil {
		return nil, errorsx.InvalidParam("entry session is inactive or expired")
	}
	if session.CustomerUserID <= 0 {
		return nil, errorsx.Unauthorized("a signed-in customer account is required")
	}
	if session.VisitorID != strings.TrimSpace(visitorID) || !s.VerifyVisitorToken(session, visitorToken) {
		return nil, errorsx.Unauthorized("customer entry session is invalid")
	}
	if err := s.RequirePrivacyConsent(session); err != nil {
		return nil, err
	}

	tenant := repositories.TenantRepository.Get(sqls.DB(), session.TenantID)
	if tenant == nil || tenant.Status != enums.StatusOk {
		return nil, errorsx.InvalidParam("tenant is not available")
	}

	serviceCode := repositories.ServiceCodeRepository.Get(sqls.DB(), session.ServiceCodeID)
	device := repositories.DeviceRepository.Get(sqls.DB(), session.DeviceID)
	product := productForEntrySession(session, serviceCode, device)
	model := modelForEntrySession(session, serviceCode, device)

	serviceCodeValue := ""
	if serviceCode != nil {
		serviceCodeValue = serviceCode.ServiceCode
	}
	if serviceCodeValue == "" {
		serviceCodeValue = sessionContextString(session.EntryContextJSON, "serviceCode")
	}

	tickets := s.listContextTickets(session, serviceCode, device)
	ticketIDs := make([]int64, 0, len(tickets))
	for _, ticket := range tickets {
		ticketIDs = append(ticketIDs, ticket.ID)
	}

	result := &dto.CustomerEntryContextDTO{
		EntrySessionID:   session.ID,
		ServiceCode:      serviceCodeValue,
		Tenant:           buildCustomerEntryTenantSummary(tenant),
		Product:          buildCustomerEntryProductSummary(product),
		Model:            buildCustomerEntryModelSummary(model),
		Device:           buildCustomerEntryDeviceSummary(device),
		Devices:          s.buildContextDevices(device, product, model),
		Tickets:          s.buildContextTickets(tickets, device),
		Conversations:    s.buildContextConversations(session, serviceCode, device),
		RepairHistory:    s.buildContextRepairHistory(session, device),
		KnowledgeEntries: s.buildContextKnowledgeEntries(tenant.ID, product, model),
		ManualFiles:      s.buildContextManualFiles(tenant.ID, product),
		Meetings:         s.buildContextMeetings(tenant.ID, ticketIDs),
	}
	return result, nil
}

func (s *customerEntryService) SubmitTicketFeedback(entrySessionID, ticketID int64, visitorID, visitorToken string, req request.SubmitTicketFeedbackRequest) (*dto.CustomerEntryTicketFeedbackDTO, error) {
	session, ticket, err := s.resolveEntryTicket(entrySessionID, ticketID, visitorID, visitorToken)
	if err != nil {
		return nil, err
	}
	feedback, err := CustomerTicketActionService.SubmitFeedback(ticket, session.CustomerUserID, req, customerEntryOperator(session))
	if err != nil {
		return nil, err
	}
	return buildCustomerEntryFeedback(feedback), nil
}

func (s *customerEntryService) ConfirmTicketResolved(entrySessionID, ticketID int64, visitorID, visitorToken string) (*dto.CustomerEntryTicketDTO, error) {
	session, ticket, err := s.resolveEntryTicket(entrySessionID, ticketID, visitorID, visitorToken)
	if err != nil {
		return nil, err
	}
	if err := CustomerTicketActionService.ConfirmResolved(ticket, customerEntryOperator(session)); err != nil {
		return nil, err
	}
	return s.findEntryTicketDTO(session, ticketID), nil
}

func (s *customerEntryService) ReopenTicket(entrySessionID, ticketID int64, visitorID, visitorToken, reason string) (*dto.CustomerEntryTicketDTO, error) {
	session, ticket, err := s.resolveEntryTicket(entrySessionID, ticketID, visitorID, visitorToken)
	if err != nil {
		return nil, err
	}
	if err := CustomerTicketActionService.Reopen(ticket, reason, customerEntryOperator(session)); err != nil {
		return nil, err
	}
	return s.findEntryTicketDTO(session, ticketID), nil
}

func (s *customerEntryService) JoinMeeting(ctx context.Context, entrySessionID int64, meetingID, visitorID, visitorToken string) (*JoinConfig, error) {
	if entrySessionID <= 0 || strings.TrimSpace(meetingID) == "" {
		return nil, errorsx.InvalidParam("entrySessionId and meetingId are required")
	}
	var meeting models.MeetingRoomJitsi
	if err := sqls.DB().Where("id = ?", strings.TrimSpace(meetingID)).First(&meeting).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errorsx.InvalidParam("meeting not found")
		}
		return nil, err
	}
	ticketID, err := strconv.ParseInt(meeting.TicketID, 10, 64)
	if err != nil || ticketID <= 0 {
		return nil, errorsx.InvalidParam("meeting ticket context is invalid")
	}
	session, ticket, err := s.resolveEntryTicket(entrySessionID, ticketID, visitorID, visitorToken)
	if err != nil {
		return nil, err
	}
	if meeting.TenantID != session.TenantID || ticket.TenantID != meeting.TenantID {
		return nil, errorsx.Unauthorized("meeting is not visible to current entry session")
	}
	return MeetingService.JoinMeeting(
		ctx,
		meeting.ID,
		"customer-user-"+strconv.FormatInt(session.CustomerUserID, 10),
		"客户用户",
		"customer",
		session.TenantID,
	)
}

func (s *customerEntryService) resolveEntryTicket(entrySessionID, ticketID int64, visitorID, visitorToken string) (*models.CustomerEntrySession, *models.Ticket, error) {
	if entrySessionID <= 0 || ticketID <= 0 || strings.TrimSpace(visitorID) == "" || strings.TrimSpace(visitorToken) == "" {
		return nil, nil, errorsx.InvalidParam("entrySessionId, ticketId and visitor credential are required")
	}
	session := repositories.CustomerEntrySessionRepository.FindActive(sqls.DB(), entrySessionID, time.Now())
	if session == nil || session.VisitorID != strings.TrimSpace(visitorID) || !s.VerifyVisitorToken(session, visitorToken) {
		return nil, nil, errorsx.Unauthorized("customer entry session is invalid")
	}
	if session.CustomerUserID <= 0 {
		return nil, nil, errorsx.Unauthorized("a signed-in customer account is required")
	}
	if err := s.RequirePrivacyConsent(session); err != nil {
		return nil, nil, err
	}
	ticket := repositories.TicketRepository.Get(sqls.DB(), ticketID)
	if ticket == nil || ticket.TenantID != session.TenantID || ticket.CustomerEntrySessionID != session.ID {
		return nil, nil, errorsx.Unauthorized("ticket is not visible to current entry session")
	}
	return session, ticket, nil
}

func (s *customerEntryService) findEntryTicketDTO(session *models.CustomerEntrySession, ticketID int64) *dto.CustomerEntryTicketDTO {
	items := s.buildContextTickets(s.listContextTickets(session, nil, nil), repositories.DeviceRepository.Get(sqls.DB(), session.DeviceID))
	for i := range items {
		if items[i].ID == ticketID {
			return &items[i]
		}
	}
	return nil
}

func customerEntryOperator(session *models.CustomerEntrySession) *dto.AuthPrincipal {
	return &dto.AuthPrincipal{
		Username: "客户用户", TenantID: session.TenantID, DomainType: models.DomainTypeCustomer,
		SubjectType: models.SubjectTypeCustomerUser, SubjectID: session.CustomerUserID, CustomerUserID: session.CustomerUserID, SessionID: session.ID,
	}
}

func buildCustomerEntryFeedback(feedback *models.TicketFeedback) *dto.CustomerEntryTicketFeedbackDTO {
	if feedback == nil {
		return nil
	}
	tags := []string{}
	if strings.TrimSpace(feedback.TagsJSON) != "" {
		_ = json.Unmarshal([]byte(feedback.TagsJSON), &tags)
	}
	return &dto.CustomerEntryTicketFeedbackDTO{
		ID: feedback.ID, Rating: feedback.Rating, Tags: tags, Comment: feedback.Comment, SubmittedAt: formatEnterpriseTime(feedback.SubmittedAt),
	}
}

func (s *customerEntryService) buildContextManualFiles(tenantID int64, product *models.Product) []dto.CustomerEntryManualFileDTO {
	if product == nil {
		return []dto.CustomerEntryManualFileDTO{}
	}
	items, err := ProductManualFileService.ListCustomerManualFiles(tenantID, product.ID)
	if err != nil {
		return []dto.CustomerEntryManualFileDTO{}
	}
	result := make([]dto.CustomerEntryManualFileDTO, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.URL) == "" {
			continue
		}
		result = append(result, dto.CustomerEntryManualFileDTO{
			ID:         item.ID,
			Title:      firstNonBlank(item.Title, item.Filename),
			Filename:   item.Filename,
			FileSize:   item.FileSize,
			MimeType:   item.MimeType,
			URL:        item.URL,
			UploadedAt: item.UploadedAt,
		})
	}
	return result
}

func productForEntrySession(session *models.CustomerEntrySession, serviceCode *models.ServiceCode, device *models.Device) *models.Product {
	productID := int64(0)
	switch {
	case device != nil && device.ProductID > 0:
		productID = device.ProductID
	case serviceCode != nil && serviceCode.ProductID > 0:
		productID = serviceCode.ProductID
	default:
		productID = int64FromSessionContext(session.EntryContextJSON, "productId")
	}
	if productID <= 0 {
		return nil
	}
	product := repositories.ProductRepository.Get(sqls.DB(), productID)
	if product == nil || product.Status != enums.StatusOk || product.TenantID != session.TenantID {
		return nil
	}
	return product
}

func modelForEntrySession(session *models.CustomerEntrySession, serviceCode *models.ServiceCode, device *models.Device) *models.ProductModel {
	modelID := int64(0)
	switch {
	case device != nil && device.ProductModelID > 0:
		modelID = device.ProductModelID
	case serviceCode != nil && serviceCode.ProductModelID > 0:
		modelID = serviceCode.ProductModelID
	default:
		modelID = int64FromSessionContext(session.EntryContextJSON, "productModelId")
	}
	if modelID <= 0 {
		return nil
	}
	model := repositories.ProductModelRepository.Get(sqls.DB(), modelID)
	if model == nil || model.Status != enums.StatusOk || model.TenantID != session.TenantID {
		return nil
	}
	return model
}

func buildCustomerEntryTenantSummary(item *models.Tenant) *dto.CustomerEntryContextSummaryDTO {
	if item == nil {
		return nil
	}
	return &dto.CustomerEntryContextSummaryDTO{ID: item.ID, Name: item.Name, Status: enterpriseStatusText(item.Status)}
}

func buildCustomerEntryProductSummary(item *models.Product) *dto.CustomerEntryContextSummaryDTO {
	if item == nil {
		return nil
	}
	return &dto.CustomerEntryContextSummaryDTO{ID: item.ID, Code: item.Code, Name: item.Name, Status: enterpriseStatusText(item.Status)}
}

func buildCustomerEntryModelSummary(item *models.ProductModel) *dto.CustomerEntryContextSummaryDTO {
	if item == nil {
		return nil
	}
	return &dto.CustomerEntryContextSummaryDTO{ID: item.ID, Code: item.ModelCode, Name: item.Name, ModelCode: item.ModelCode, Status: enterpriseStatusText(item.Status)}
}

func buildCustomerEntryDeviceSummary(item *models.Device) *dto.CustomerEntryContextSummaryDTO {
	if item == nil {
		return nil
	}
	return &dto.CustomerEntryContextSummaryDTO{
		ID:         item.ID,
		Name:       item.DeviceNo,
		DeviceNo:   item.DeviceNo,
		SerialNo:   item.SerialNo,
		RegionCode: item.RegionCode,
		Status:     mapDeviceStatusForEnterprise(*item),
	}
}

func (s *customerEntryService) buildContextDevices(device *models.Device, product *models.Product, model *models.ProductModel) []dto.CustomerEntryDeviceDTO {
	if device == nil {
		return []dto.CustomerEntryDeviceDTO{}
	}
	productName, productCode := "", ""
	if product != nil {
		productName, productCode = product.Name, product.Code
	}
	modelName := ""
	if model != nil {
		modelName = model.Name
	}
	return []dto.CustomerEntryDeviceDTO{{
		ID:            device.ID,
		DeviceNo:      device.DeviceNo,
		SerialNo:      device.SerialNo,
		ProductName:   productName,
		ProductCode:   productCode,
		ModelName:     modelName,
		RegionCode:    device.RegionCode,
		Status:        mapDeviceStatusForEnterprise(*device),
		LastServiceAt: formatEnterpriseTimePtr(device.LastServiceAt),
	}}
}

func (s *customerEntryService) listContextTickets(session *models.CustomerEntrySession, serviceCode *models.ServiceCode, device *models.Device) []models.Ticket {
	cnd := sqls.NewCnd().
		Eq("tenant_id", session.TenantID).
		Eq("customer_entry_session_id", session.ID)
	cnd.Desc("created_at").Desc("id")
	return repositories.TicketRepository.Find(sqls.DB(), cnd)
}

func (s *customerEntryService) buildContextTickets(tickets []models.Ticket, device *models.Device) []dto.CustomerEntryTicketDTO {
	deviceNo := ""
	if device != nil {
		deviceNo = device.DeviceNo
	}
	items := make([]dto.CustomerEntryTicketDTO, 0, len(tickets))
	for _, ticket := range tickets {
		rowDeviceNo := deviceNo
		if rowDeviceNo == "" && ticket.DeviceID > 0 {
			if ticketDevice := repositories.DeviceRepository.Get(sqls.DB(), ticket.DeviceID); ticketDevice != nil {
				rowDeviceNo = ticketDevice.DeviceNo
			}
		}
		feedback := repositories.TicketFeedbackRepository.FindOne(sqls.DB(), sqls.NewCnd().
			Eq("tenant_id", ticket.TenantID).
			Eq("ticket_id", ticket.ID).
			Desc("submitted_at").
			Desc("id"))
		canConfirm, canReopen, canRate := customerTicketActionFlags(&ticket, feedback)
		repairSummary := ""
		if repair := latestTicketRepair(ticket.ID); repair != nil {
			repairSummary = firstNonBlank(strings.TrimSpace(repair.Solution), strings.TrimSpace(repair.Conclusion), strings.TrimSpace(repair.RepairMethod))
		}
		items = append(items, dto.CustomerEntryTicketDTO{
			ID: ticket.ID, ConversationID: ticket.ConversationID, TicketNo: ticket.TicketNo, Title: ticket.Title,
			Status: MapTicketStatusForEnterprise(ticket.Status), Priority: DeriveTicketPriority(ticket),
			CreatedAt: formatEnterpriseTime(ticket.CreatedAt), DeviceNo: rowDeviceNo,
			RepairSummary: repairSummary, Feedback: buildCustomerEntryFeedback(feedback),
			CanConfirm: canConfirm, CanReopen: canReopen, CanRate: canRate,
		})
	}
	return items
}

func (s *customerEntryService) buildContextConversations(session *models.CustomerEntrySession, serviceCode *models.ServiceCode, device *models.Device) []dto.CustomerEntryConversationDTO {
	cnd := sqls.NewCnd().
		Eq("tenant_id", session.TenantID).
		Eq("customer_entry_session_id", session.ID)
	cnd.Desc("last_active_at").Desc("id")
	conversations := repositories.ConversationRepository.Find(sqls.DB(), cnd)
	items := make([]dto.CustomerEntryConversationDTO, 0, len(conversations))
	for _, item := range conversations {
		deviceNo := ""
		if device != nil && item.DeviceID == device.ID {
			deviceNo = device.DeviceNo
		} else if item.DeviceID > 0 {
			if itemDevice := repositories.DeviceRepository.Get(sqls.DB(), item.DeviceID); itemDevice != nil {
				deviceNo = itemDevice.DeviceNo
			}
		}
		agentType := "human"
		if item.AIReplyRounds > 0 && item.CurrentAssigneeID == 0 {
			agentType = "ai"
		}
		items = append(items, dto.CustomerEntryConversationDTO{
			ID:        item.ID,
			Summary:   firstNonBlank(item.LastMessageSummary, item.HandoffReason, "Service conversation"),
			StartedAt: formatEnterpriseTime(item.CreatedAt),
			EndedAt:   formatEnterpriseTimePtr(item.ClosedAt),
			AgentType: agentType,
			DeviceNo:  deviceNo,
		})
	}
	return items
}

func (s *customerEntryService) buildContextRepairHistory(session *models.CustomerEntrySession, device *models.Device) []dto.CustomerEntryRepairHistoryDTO {
	if device == nil || device.ID <= 0 {
		return []dto.CustomerEntryRepairHistoryDTO{}
	}
	records := repositories.DeviceServiceRecordRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", session.TenantID).
		Eq("device_id", device.ID).
		Eq("visible_to_customer", true).
		Desc("occurred_at").
		Desc("id"))
	items := make([]dto.CustomerEntryRepairHistoryDTO, 0, len(records))
	for _, item := range records {
		ticketNo := ""
		if item.TicketID > 0 {
			if ticket := repositories.TicketRepository.Get(sqls.DB(), item.TicketID); ticket != nil {
				ticketNo = ticket.TicketNo
			}
		}
		items = append(items, dto.CustomerEntryRepairHistoryDTO{
			ID:          item.ID,
			TicketID:    item.TicketID,
			TicketNo:    ticketNo,
			DeviceNo:    device.DeviceNo,
			ServiceType: item.ServiceType,
			Summary:     item.Summary,
			RootCause:   item.RootCause,
			Solution:    item.Solution,
			OccurredAt:  formatEnterpriseTime(item.OccurredAt),
		})
	}
	return items
}

func (s *customerEntryService) buildContextKnowledgeEntries(tenantID int64, product *models.Product, model *models.ProductModel) []dto.CustomerEntryKnowledgeDTO {
	if product == nil {
		return []dto.CustomerEntryKnowledgeDTO{}
	}
	links := repositories.ProductKnowledgeLinkRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("product_id", product.ID).
		Eq("status", enums.StatusOk).
		Eq("publish_status", "published").
		Asc("sort_no").
		Desc("updated_at"))
	items := make([]dto.CustomerEntryKnowledgeDTO, 0, len(links))
	for _, link := range links {
		if !matchesCustomerEntryModel(link, model) || !isCustomerVisibleKnowledgeLink(link) {
			continue
		}
		if row := buildCustomerEntryKnowledgeEntry(link); row != nil {
			items = append(items, *row)
		}
	}
	return items
}

func matchesCustomerEntryModel(link models.ProductKnowledgeLink, model *models.ProductModel) bool {
	if link.ProductModelID <= 0 {
		return true
	}
	return model != nil && link.ProductModelID == model.ID
}

func isCustomerVisibleKnowledgeLink(link models.ProductKnowledgeLink) bool {
	switch strings.ToLower(strings.TrimSpace(link.Visibility)) {
	case "customer", "public", "external":
		return true
	default:
		return false
	}
}

func buildCustomerEntryKnowledgeEntry(link models.ProductKnowledgeLink) *dto.CustomerEntryKnowledgeDTO {
	linkType := strings.ToLower(strings.TrimSpace(link.LinkType))
	if linkType == "faq" {
		if row := buildCustomerEntryFAQKnowledge(link); row != nil {
			return row
		}
	}
	if row := buildCustomerEntryDocumentKnowledge(link); row != nil {
		return row
	}
	return buildCustomerEntryFAQKnowledge(link)
}

func buildCustomerEntryDocumentKnowledge(link models.ProductKnowledgeLink) *dto.CustomerEntryKnowledgeDTO {
	if link.KnowledgeEntryID <= 0 {
		return nil
	}
	doc := repositories.KnowledgeDocumentRepository.Get(sqls.DB(), link.KnowledgeEntryID)
	if doc == nil || doc.TenantID != link.TenantID || doc.Status != enums.StatusOk {
		return nil
	}
	return &dto.CustomerEntryKnowledgeDTO{
		ID:          doc.ID,
		Title:       doc.Title,
		Type:        firstNonBlank(link.LinkType, "document"),
		Language:    link.Language,
		Version:     link.Version,
		Content:     doc.Content,
		ContentType: string(doc.ContentType),
		UpdatedAt:   formatEnterpriseTime(doc.UpdatedAt),
	}
}

func buildCustomerEntryFAQKnowledge(link models.ProductKnowledgeLink) *dto.CustomerEntryKnowledgeDTO {
	if link.KnowledgeEntryID <= 0 {
		return nil
	}
	faq := repositories.KnowledgeFAQRepository.Get(sqls.DB(), link.KnowledgeEntryID)
	if faq == nil || faq.TenantID != link.TenantID || faq.Status != enums.StatusOk {
		return nil
	}
	return &dto.CustomerEntryKnowledgeDTO{
		ID:          faq.ID,
		Title:       faq.Question,
		Type:        "faq",
		Language:    link.Language,
		Version:     link.Version,
		Content:     faq.Answer,
		ContentType: "faq",
		UpdatedAt:   formatEnterpriseTime(faq.UpdatedAt),
	}
}

func (s *customerEntryService) buildContextMeetings(tenantID int64, ticketIDs []int64) []dto.CustomerEntryMeetingDTO {
	if len(ticketIDs) == 0 {
		return []dto.CustomerEntryMeetingDTO{}
	}
	ticketIDStrings := make([]string, 0, len(ticketIDs))
	for _, id := range ticketIDs {
		ticketIDStrings = append(ticketIDStrings, strconv.FormatInt(id, 10))
	}
	meetings := repositories.MeetingRoomRepository.FindJitsiByTicketIDs(sqls.DB(), tenantID, ticketIDStrings)
	items := make([]dto.CustomerEntryMeetingDTO, 0, len(meetings))
	for _, item := range meetings {
		ticketID, _ := strconv.ParseInt(item.TicketID, 10, 64)
		items = append(items, dto.CustomerEntryMeetingDTO{
			ID:          item.ID,
			Title:       "Remote support",
			Status:      item.Status,
			ScheduledAt: firstNonEmptyString(formatEnterpriseTimePtr(item.ScheduledAt), formatEnterpriseTime(item.CreatedAt)),
			StartedAt:   formatEnterpriseTimePtr(item.StartedAt),
			EndedAt:     formatEnterpriseTimePtr(item.EndedAt),
			Initiator:   item.CreatedBy,
			TicketID:    ticketID,
		})
	}
	return items
}

func sessionContextString(raw string, key string) string {
	context := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &context); err != nil {
		return ""
	}
	if value, ok := context[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func int64FromSessionContext(raw string, key string) int64 {
	context := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &context); err != nil {
		return 0
	}
	switch value := context[key].(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case string:
		id, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return id
	default:
		return 0
	}
}

func modelID(v *models.ProductModel) int64 {
	if v == nil {
		return 0
	}
	return v.ID
}
func deviceID(v *models.Device) int64 {
	if v == nil {
		return 0
	}
	return v.ID
}
func serviceCodeID(v *models.ServiceCode) int64 {
	if v == nil {
		return 0
	}
	return v.ID
}
func serviceCodeValueOrEmpty(v *models.ServiceCode, fallback string) string {
	if v != nil {
		return strings.TrimSpace(v.ServiceCode)
	}
	return strings.TrimSpace(fallback)
}

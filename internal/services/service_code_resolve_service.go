package services

import (
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var ServiceCodeResolveService = newServiceCodeResolveService()

func newServiceCodeResolveService() *serviceCodeResolveService {
	return &serviceCodeResolveService{}
}

type serviceCodeResolveService struct {
}

type ServiceCodeResolveResult struct {
	Valid        bool
	EntryState   enums.CustomerEntryState
	Reason       string
	ExpiredAt    *time.Time
	ServiceCode  *models.ServiceCode
	Tenant       *models.Tenant
	Product      *models.Product
	ProductModel *models.ProductModel
	Device       *models.Device
}

type ResolveOptions struct {
	IPAddress string
	UserAgent string
	VisitorID string
}

func (s *serviceCodeResolveService) Resolve(rawServiceCode string, opts ...ResolveOptions) (*ServiceCodeResolveResult, error) {
	serviceCodeValue := strings.TrimSpace(rawServiceCode)
	if serviceCodeValue == "" {
		return nil, errorsx.InvalidParam("serviceCode is required")
	}

	var opt ResolveOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	startTime := time.Now()

	serviceCode := repositories.ServiceCodeRepository.GetByCode(sqls.DB(), serviceCodeValue)
	if serviceCode == nil {
		s.recordScanLog(serviceCodeValue, 0, 0, 0, 0, opt, "invalid", "service code not found", startTime)
		return s.invalidResponse("service code not found"), nil
	}

	// 1. 先检查是否已作废
	if enums.IsRevokedServiceCodeStatus(serviceCode.Status) {
		s.recordScanLog(serviceCodeValue, serviceCode.ID, serviceCode.TenantID, serviceCode.ProductID, serviceCode.ProductModelID, opt, "revoked", "service code has been revoked", startTime)
		return s.revokedResponse(serviceCode), nil
	}

	// 2. 检查是否过期
	if ServiceCodeManagementService.IsServiceCodeExpired(serviceCode) {
		s.recordScanLog(serviceCodeValue, serviceCode.ID, serviceCode.TenantID, serviceCode.ProductID, serviceCode.ProductModelID, opt, "expired", "service code is expired", startTime)
		return s.expiredResponse(serviceCode), nil
	}

	// 3. 已激活和已绑定的服务码都可重复进入设备服务。bound 表示归属已确认，不表示失效。
	if serviceCode.Status != enums.ServiceCodeStatusActive && serviceCode.Status != enums.ServiceCodeStatusBound {
		s.recordScanLog(serviceCodeValue, serviceCode.ID, serviceCode.TenantID, serviceCode.ProductID, serviceCode.ProductModelID, opt, "invalid", "service code is not active", startTime)
		return s.invalidResponse("service code is not active"), nil
	}

	tenant := repositories.TenantRepository.Get(sqls.DB(), serviceCode.TenantID)
	product := repositories.ProductRepository.Get(sqls.DB(), serviceCode.ProductID)
	if tenant == nil || tenant.Status != enums.StatusOk {
		s.recordScanLog(serviceCodeValue, serviceCode.ID, serviceCode.TenantID, serviceCode.ProductID, serviceCode.ProductModelID, opt, "invalid", "tenant is not available", startTime)
		return s.invalidResponse("tenant is not available"), nil
	}
	if product == nil || product.Status != enums.StatusOk {
		s.recordScanLog(serviceCodeValue, serviceCode.ID, serviceCode.TenantID, serviceCode.ProductID, serviceCode.ProductModelID, opt, "invalid", "product is not available", startTime)
		return s.invalidResponse("product is not available"), nil
	}
	if product.TenantID != tenant.ID {
		s.recordScanLog(serviceCodeValue, serviceCode.ID, serviceCode.TenantID, serviceCode.ProductID, serviceCode.ProductModelID, opt, "invalid", "product does not belong to tenant", startTime)
		return s.invalidResponse("product does not belong to tenant"), nil
	}

	var productModel *models.ProductModel
	if serviceCode.ProductModelID > 0 {
		productModel = repositories.ProductModelRepository.Get(sqls.DB(), serviceCode.ProductModelID)
		if productModel == nil || productModel.Status != enums.StatusOk {
			s.recordScanLog(serviceCodeValue, serviceCode.ID, serviceCode.TenantID, serviceCode.ProductID, serviceCode.ProductModelID, opt, "invalid", "product model is not available", startTime)
			return s.invalidResponse("product model is not available"), nil
		}
		if productModel.TenantID != tenant.ID || productModel.ProductID != product.ID {
			s.recordScanLog(serviceCodeValue, serviceCode.ID, serviceCode.TenantID, serviceCode.ProductID, serviceCode.ProductModelID, opt, "invalid", "product model does not belong to product", startTime)
			return s.invalidResponse("product model does not belong to product"), nil
		}
	}

	device := s.resolveDevice(serviceCode)
	if serviceCode.DeviceID > 0 && device == nil {
		s.recordScanLog(serviceCodeValue, serviceCode.ID, serviceCode.TenantID, serviceCode.ProductID, serviceCode.ProductModelID, opt, "invalid", "device is not available", startTime)
		return s.invalidResponse("device is not available"), nil
	}

	s.recordScanLog(serviceCodeValue, serviceCode.ID, serviceCode.TenantID, serviceCode.ProductID, serviceCode.ProductModelID, opt, "success", "", startTime)

	return &ServiceCodeResolveResult{
		Valid:        true,
		EntryState:   enums.CustomerEntryStateNeedRegister,
		ServiceCode:  serviceCode,
		Tenant:       tenant,
		Product:      product,
		ProductModel: productModel,
		Device:       device,
	}, nil
}

func (s *serviceCodeResolveService) recordScanLog(codeVal string, serviceCodeID, tenantID, productID, productModelID int64, opt ResolveOptions, result, errMsg string, startTime time.Time) {
	latency := time.Since(startTime).Milliseconds()
	log := &models.QrScanLog{
		TenantID:       tenantID,
		ServiceCodeID:  serviceCodeID,
		ServiceCodeVal: codeVal,
		DeviceID:       0,
		ProductID:      productID,
		ProductModelID: productModelID,
		IPAddress:      opt.IPAddress,
		UserAgent:      opt.UserAgent,
		VisitorID:      opt.VisitorID,
		Result:         result,
		ErrorMessage:   errMsg,
		LatencyMs:      latency,
		CreatedAt:      time.Now(),
	}
	_ = repositories.QrScanLogRepository.Create(sqls.DB(), log)
}

func (s *serviceCodeResolveService) resolveDevice(serviceCode *models.ServiceCode) *models.Device {
	if serviceCode == nil || serviceCode.DeviceID <= 0 {
		return nil
	}
	device := repositories.DeviceRepository.Get(sqls.DB(), serviceCode.DeviceID)
	if device == nil || device.Status != enums.StatusOk {
		return nil
	}
	return device
}

func (s *serviceCodeResolveService) invalidResponse(reason string) *ServiceCodeResolveResult {
	return &ServiceCodeResolveResult{
		Valid:      false,
		EntryState: enums.CustomerEntryStateInvalid,
		Reason:     reason,
	}
}

func (s *serviceCodeResolveService) revokedResponse(serviceCode *models.ServiceCode) *ServiceCodeResolveResult {
	return &ServiceCodeResolveResult{
		Valid:       false,
		EntryState:  enums.CustomerEntryStateRevoked,
		Reason:      "service code has been revoked",
		ServiceCode: serviceCode,
	}
}

func (s *serviceCodeResolveService) expiredResponse(serviceCode *models.ServiceCode) *ServiceCodeResolveResult {
	expiredDate := ""
	if serviceCode.EffectiveEndAt != nil {
		expiredDate = serviceCode.EffectiveEndAt.Format("2006-01-02")
	}
	return &ServiceCodeResolveResult{
		Valid:       false,
		EntryState:  enums.CustomerEntryStateExpired,
		Reason:      fmt.Sprintf("此服务码已在 %s 过期，请联系客户支持", expiredDate),
		ExpiredAt:   serviceCode.EffectiveEndAt,
		ServiceCode: serviceCode,
	}
}

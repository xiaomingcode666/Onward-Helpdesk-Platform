package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/servicecode"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var EnterpriseDeviceService = newEnterpriseDeviceService()

func newEnterpriseDeviceService() *enterpriseDeviceService {
	return &enterpriseDeviceService{}
}

type enterpriseDeviceService struct{}

func requireTenantDeviceFeature(tenantID int64) error {
	tenant := repositories.PlatformIAMRepository.GetTenant(sqls.DB(), tenantID)
	if tenant != nil && tenant.IsKnowledgeSupportScene() {
		return errorsx.Forbidden("device management is unavailable for this tenant")
	}
	return nil
}

type EnterpriseDeviceQuery struct {
	Search    string
	Status    string
	ProductID int64
	Page      int
	PageSize  int
}

func (s *enterpriseDeviceService) CreateForOperator(tenantID int64, req dto.EnterpriseDeviceCreateRequest, operator *dto.AuthPrincipal) (*dto.EnterpriseDeviceListItemDTO, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	if err := requireTenantDeviceFeature(tenantID); err != nil {
		return nil, err
	}
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	serviceCodeValue := normalizeCode(req.ServiceCode)
	if serviceCodeValue == "" {
		return nil, errorsx.InvalidParam("serviceCode is required")
	}
	existingServiceCode := repositories.ServiceCodeRepository.GetByCode(sqls.DB(), serviceCodeValue)
	productID := req.ProductID
	modelID := req.ModelID
	if existingServiceCode != nil {
		if existingServiceCode.TenantID != tenantID {
			return nil, errorsx.InvalidParam("service code not found")
		}
		productID = existingServiceCode.ProductID
		if productID <= 0 {
			return nil, errorsx.InvalidParam("service code is not linked to a product")
		}
		if req.ProductID > 0 && req.ProductID != productID {
			return nil, errorsx.InvalidParam("selected product does not match service code")
		}
		modelID = existingServiceCode.ProductModelID
		if req.ModelID > 0 {
			if modelID > 0 && req.ModelID != modelID {
				return nil, errorsx.InvalidParam("selected product model does not match service code")
			}
			if modelID == 0 {
				modelID = req.ModelID
			}
		}
	}
	if productID <= 0 {
		return nil, errorsx.InvalidParam("productId is required")
	}
	if !resolveEnterpriseProductAccessScope(tenantID, operator).canAccessProduct(productID) {
		return nil, errorsx.Forbidden("product is outside the engineer's authorized product scope")
	}
	if err := DeviceService.validateReferences(tenantID, productID, modelID); err != nil {
		return nil, err
	}
	installedAt, err := parseOptionalEnterpriseDeviceDate(req.InstallDate, "install_date")
	if err != nil {
		return nil, err
	}
	installLocationJSON, err := normalizeJSON(req.InstallLocationJSON, "{}")
	if err != nil {
		return nil, err
	}
	metadataJSON, err := normalizeEnterpriseDeviceMetadata(req.MetadataJSON, req.Description)
	if err != nil {
		return nil, err
	}
	deviceNo := serviceCodeValue
	source := firstNonEmptyString(req.Source, "manual")
	now := time.Now()
	var item *models.Device
	err = sqls.DB().Transaction(func(tx *gorm.DB) error {
		if existingServiceCode != nil {
			var locked models.ServiceCode
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id = ?", existingServiceCode.ID).Error; err != nil {
				return errorsx.InvalidParam("service code not found")
			}
			if locked.TenantID != tenantID || locked.ServiceCode != serviceCodeValue {
				return errorsx.InvalidParam("service code scope changed")
			}
			if locked.DeviceID > 0 {
				return errorsx.InvalidParam("service code is already bound to a device")
			}
			if locked.Status != enums.ServiceCodeStatusActive {
				return errorsx.InvalidParam("service code cannot create a device in current status")
			}
			if ServiceCodeManagementService.IsServiceCodeExpired(&locked) {
				return errorsx.InvalidParam("service code is expired")
			}
			if enums.IsRevokedServiceCodeStatus(locked.Status) {
				return errorsx.InvalidParam("service code cannot create a device in current status")
			}
			if locked.ProductID != productID || locked.ProductModelID != existingServiceCode.ProductModelID {
				return errorsx.InvalidParam("service code product scope changed")
			}
			existingServiceCode = &locked
		}
		if repositories.DeviceRepository.GetByTenantDeviceNo(tx, tenantID, deviceNo) != nil {
			return errorsx.InvalidParam("deviceNo already exists")
		}
		item = &models.Device{
			TenantID:            tenantID,
			DeviceNo:            deviceNo,
			ProductID:           productID,
			ProductModelID:      modelID,
			ExternalDeviceID:    strings.TrimSpace(req.ExternalDeviceID),
			InstallLocationJSON: installLocationJSON,
			RegionCode:          strings.TrimSpace(req.RegionCode),
			InstalledAt:         installedAt,
			Source:              source,
			Status:              enums.StatusOk,
			DeviceStatus:        string(enums.DeviceStatusOperational),
			MetadataJSON:        metadataJSON,
			AuditFields:         utils.BuildAuditFields(operator),
		}
		if err := repositories.DeviceRepository.Create(tx, item); err != nil {
			return err
		}
		if err := s.createOrBindDeviceServiceCode(tx, tenantID, item, serviceCodeValue, productID, modelID, existingServiceCode, operator, now); err != nil {
			return err
		}
		return s.syncDeviceWarrantyRecord(tx, tenantID, item, req.WarrantyEnd, false, operator, now)
	})
	if err != nil {
		return nil, err
	}
	row := s.buildListItem(*item)
	return &row, nil
}

func (s *enterpriseDeviceService) BatchCreateForOperator(tenantID int64, req dto.EnterpriseDeviceBatchCreateRequest, operator *dto.AuthPrincipal) (*dto.EnterpriseDeviceBatchImportResult, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	if err := requireTenantDeviceFeature(tenantID); err != nil {
		return nil, err
	}
	if len(req.Devices) == 0 {
		return nil, errorsx.InvalidParam("no devices to import")
	}
	if len(req.Devices) > 500 {
		return nil, errorsx.InvalidParam("cannot import more than 500 devices at once")
	}
	result := &dto.EnterpriseDeviceBatchImportResult{
		Total:   len(req.Devices),
		Errors:  make([]dto.EnterpriseDeviceBatchImportError, 0),
		Created: make([]dto.EnterpriseDeviceListItemDTO, 0, len(req.Devices)),
	}
	for index, row := range req.Devices {
		row.Source = firstNonEmptyString(row.Source, "batch_import")
		created, err := s.CreateForOperator(tenantID, row, operator)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, dto.EnterpriseDeviceBatchImportError{
				Row:    index + 1,
				Reason: err.Error(),
			})
			continue
		}
		result.Succeeded++
		result.Created = append(result.Created, *created)
	}
	return result, nil
}

func (s *enterpriseDeviceService) UpdateForOperator(tenantID, deviceID int64, req dto.EnterpriseDeviceUpdateRequest, operator *dto.AuthPrincipal) (*dto.EnterpriseDeviceDetailDTO, error) {
	if tenantID <= 0 || deviceID <= 0 {
		return nil, errorsx.InvalidParam("tenant and device are required")
	}
	if err := requireTenantDeviceFeature(tenantID); err != nil {
		return nil, err
	}
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := repositories.DeviceRepository.Get(sqls.DB(), deviceID)
	if item == nil || item.TenantID != tenantID || item.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("device not found")
	}
	scope := resolveEnterpriseProductAccessScope(tenantID, operator)
	if !scope.canAccessProduct(item.ProductID) {
		return nil, errorsx.Forbidden("device is outside the engineer's authorized product scope")
	}
	productID := req.ProductID
	if productID <= 0 {
		productID = item.ProductID
	}
	if !scope.canAccessProduct(productID) {
		return nil, errorsx.Forbidden("target product is outside the engineer's authorized product scope")
	}
	modelID := req.ModelID
	if err := DeviceService.validateReferences(tenantID, productID, modelID); err != nil {
		return nil, err
	}
	installedAt := item.InstalledAt
	var err error
	if req.InstallDate != nil {
		var parsed *time.Time
		parsed, err = parseOptionalEnterpriseDeviceDate(*req.InstallDate, "install_date")
		if err != nil {
			return nil, err
		}
		installedAt = parsed
	}
	installLocationJSON := strings.TrimSpace(req.InstallLocationJSON)
	if installLocationJSON == "" {
		installLocationJSON = item.InstallLocationJSON
	}
	installLocationJSON, err = normalizeJSON(installLocationJSON, "{}")
	if err != nil {
		return nil, err
	}
	metadataJSON, err := normalizeEnterpriseDeviceMetadata(req.MetadataJSON, req.Description)
	if err != nil {
		return nil, err
	}
	source := firstNonEmptyString(req.Source, item.Source, "manual")
	now := time.Now()
	if err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		if err := repositories.DeviceRepository.Updates(tx, item.ID, map[string]any{
			"product_id":            productID,
			"product_model_id":      modelID,
			"external_device_id":    strings.TrimSpace(req.ExternalDeviceID),
			"install_location_json": installLocationJSON,
			"region_code":           strings.TrimSpace(req.RegionCode),
			"installed_at":          installedAt,
			"source":                source,
			"metadata_json":         metadataJSON,
			"update_user_id":        operator.UserID,
			"update_user_name":      operator.Username,
			"updated_at":            now,
		}); err != nil {
			return err
		}
		if code := s.findDeviceServiceCode(tenantID, item.ID); code != nil {
			if err := repositories.ServiceCodeRepository.Updates(tx, code.ID, map[string]any{
				"product_id":       productID,
				"product_model_id": modelID,
				"update_user_id":   operator.UserID,
				"update_user_name": operator.Username,
				"updated_at":       now,
			}); err != nil {
				return err
			}
		}
		item.ProductID = productID
		item.ProductModelID = modelID
		item.InstalledAt = installedAt
		if req.WarrantyEnd == nil {
			return nil
		}
		return s.syncDeviceWarrantyRecord(tx, tenantID, item, *req.WarrantyEnd, true, operator, now)
	}); err != nil {
		return nil, err
	}
	return s.getDetail(tenantID, deviceID, scope)
}

func (s *enterpriseDeviceService) List(tenantID int64, query EnterpriseDeviceQuery) (*dto.EnterpriseListResponse[dto.EnterpriseDeviceListItemDTO], error) {
	return s.list(tenantID, query, enterpriseProductAccessScope{})
}

func (s *enterpriseDeviceService) ListForOperator(tenantID int64, query EnterpriseDeviceQuery, operator *dto.AuthPrincipal) (*dto.EnterpriseListResponse[dto.EnterpriseDeviceListItemDTO], error) {
	return s.list(tenantID, query, resolveEnterpriseProductAccessScope(tenantID, operator))
}

func (s *enterpriseDeviceService) list(tenantID int64, query EnterpriseDeviceQuery, scope enterpriseProductAccessScope) (*dto.EnterpriseListResponse[dto.EnterpriseDeviceListItemDTO], error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	if err := requireTenantDeviceFeature(tenantID); err != nil {
		return nil, err
	}
	query.Page, query.PageSize = normalizeEnterprisePage(query.Page, query.PageSize)
	cnd := sqls.NewCnd().Eq("tenant_id", tenantID).NotEq("status", enums.StatusDeleted)
	if scope.Restricted {
		if len(scope.ProductIDs) == 0 {
			return emptyEnterprisePage[dto.EnterpriseDeviceListItemDTO](query.Page, query.PageSize), nil
		}
		cnd.In("product_id", scope.ProductIDs)
	}
	if query.ProductID > 0 {
		cnd.Eq("product_id", query.ProductID)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		keyword := "%" + search + "%"
		matchingCodes := repositories.ServiceCodeRepository.Find(sqls.DB(), sqls.NewCnd().
			Eq("tenant_id", tenantID).
			Where("service_code LIKE ?", keyword))
		matchingDeviceIDs := make([]int64, 0, len(matchingCodes))
		for _, code := range matchingCodes {
			if code.DeviceID > 0 {
				matchingDeviceIDs = append(matchingDeviceIDs, code.DeviceID)
			}
		}
		if len(matchingDeviceIDs) > 0 {
			cnd.Where("device_no LIKE ? OR region_code LIKE ? OR id in ?", keyword, keyword, matchingDeviceIDs)
		} else {
			cnd.Where("device_no LIKE ? OR region_code LIKE ?", keyword, keyword)
		}
	}
	applyEnterpriseDeviceStatusCnd(cnd, query.Status)
	cnd.Desc("updated_at").Desc("id")
	cnd.Page(query.Page, query.PageSize)
	devices, paging := repositories.DeviceRepository.FindPageByCnd(sqls.DB(), cnd)
	items := make([]dto.EnterpriseDeviceListItemDTO, 0, len(devices))
	for _, item := range devices {
		row := s.buildListItem(item)
		items = append(items, row)
	}
	return enterprisePage(items, paging, query.Page, query.PageSize), nil
}

func applyEnterpriseDeviceStatusCnd(cnd *sqls.Cnd, status string) {
	switch strings.TrimSpace(status) {
	case "", "all":
		return
	case "active", string(enums.DeviceStatusOperational):
		cnd.Where(
			"status = ? AND (device_status = '' OR device_status not in ?)",
			enums.StatusOk,
			[]string{
				string(enums.DeviceStatusUnderMaintenance),
				string(enums.DeviceStatusArchived),
				string(enums.DeviceStatusDecommissioned),
			},
		)
	case "maintenance":
		cnd.Eq("device_status", string(enums.DeviceStatusUnderMaintenance))
	case "inactive":
		cnd.Where("device_status in ? OR status <> ?", []string{string(enums.DeviceStatusArchived), string(enums.DeviceStatusDecommissioned)}, enums.StatusOk)
	case "archived":
		cnd.Eq("device_status", string(enums.DeviceStatusArchived))
	case "decommissioned":
		cnd.Eq("device_status", string(enums.DeviceStatusDecommissioned))
	default:
		cnd.Eq("device_status", strings.TrimSpace(status))
	}
}

func enterprisePage[T any](items []T, paging *sqls.Paging, page, pageSize int) *dto.EnterpriseListResponse[T] {
	if paging == nil {
		paging = &sqls.Paging{Page: page, Limit: pageSize, Total: int64(len(items))}
	}
	totalPages := paging.TotalPage()
	return &dto.EnterpriseListResponse[T]{
		Items:      items,
		Total:      paging.Total,
		Page:       paging.Page,
		PageSize:   paging.Limit,
		TotalPages: totalPages,
		HasMore:    paging.Page < totalPages,
	}
}

func emptyEnterprisePage[T any](page, pageSize int) *dto.EnterpriseListResponse[T] {
	page, pageSize = normalizeEnterprisePage(page, pageSize)
	return &dto.EnterpriseListResponse[T]{
		Items:      []T{},
		Total:      0,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: 0,
		HasMore:    false,
	}
}

func (s *enterpriseDeviceService) buildListItem(item models.Device) dto.EnterpriseDeviceListItemDTO {
	row := dto.EnterpriseDeviceListItemDTO{
		ID:            item.ID,
		DeviceNo:      item.DeviceNo,
		SerialNo:      item.SerialNo,
		ProductID:     item.ProductID,
		ModelID:       item.ProductModelID,
		CustomerOrgID: item.CustomerOrgID,
		RegionCode:    item.RegionCode,
		InstallDate:   formatEnterpriseTimePtr(item.InstalledAt),
		Status:        mapDeviceStatusForEnterprise(item),
		DeviceStatus:  item.DeviceStatus,
		LastServiceAt: formatEnterpriseTimePtr(item.LastServiceAt),
		UpdatedAt:     formatEnterpriseTime(item.UpdatedAt),
	}
	if row.LastServiceAt == "" {
		row.LastServiceAt = formatEnterpriseTime(item.UpdatedAt)
	}
	if item.ProductID > 0 {
		if product := repositories.ProductRepository.Get(sqls.DB(), item.ProductID); product != nil {
			row.ProductName = product.Name
			row.ProductCode = product.Code
		}
	}
	if item.ProductModelID > 0 {
		if model := repositories.ProductModelRepository.Get(sqls.DB(), item.ProductModelID); model != nil {
			row.ModelName = model.Name
		}
	}
	if item.CustomerOrgID > 0 {
		if customer := repositories.CustomerRepository.Get(sqls.DB(), item.CustomerOrgID); customer != nil {
			row.CustomerName = customer.Name
			row.CustomerOrg = customer.Name
		} else {
			var org models.CustomerOrg
			if err := sqls.DB().First(&org, "id = ? and tenant_id = ?", item.CustomerOrgID, item.TenantID).Error; err == nil {
				row.CustomerName = org.Name
				row.CustomerOrg = org.Name
			}
		}
	}
	if code := s.findDeviceServiceCode(item.TenantID, item.ID); code != nil {
		row.ServiceCode = servicecode.Normalize(code.ServiceCode)
		row.EntryURL = servicecode.BuildEntryURL(code.ServiceCode)
		row.QRURL = servicecode.BuildQRURL(code.ServiceCode)
		row.QRImageURL = servicecode.BuildQRImageURL(code.ServiceCode)
	}
	row.WarrantyEnd, row.WarrantyStatus = s.findWarrantyStatus(item.TenantID, item.ID)
	for _, binding := range repositories.CustomerDeviceBindingRepository.FindByDeviceID(sqls.DB(), item.TenantID, item.ID) {
		if binding.Status == enums.StatusDeleted {
			continue
		}
		row.BindingCount++
		if row.CustomerUserID == 0 && binding.CustomerUserID > 0 {
			row.CustomerUserID = binding.CustomerUserID
		}
		if row.CustomerName != "" {
			continue
		}
		if binding.CustomerUserID > 0 {
			var customerUser models.CustomerUser
			if err := sqls.DB().First(&customerUser, "id = ? and tenant_id = ?", binding.CustomerUserID, item.TenantID).Error; err == nil {
				row.CustomerName = firstNonEmptyString(customerUser.DisplayName, customerUser.Email, customerUser.Phone)
			}
		}
		if row.CustomerName == "" && binding.CustomerOrgID > 0 {
			var customerOrg models.CustomerOrg
			if err := sqls.DB().First(&customerOrg, "id = ? and tenant_id = ?", binding.CustomerOrgID, item.TenantID).Error; err == nil {
				row.CustomerName = customerOrg.Name
			}
		}
	}
	row.OpenTicketCount = s.countDeviceTickets(item.TenantID, item.ID, false)
	row.TotalTicketCount = s.countDeviceTickets(item.TenantID, item.ID, true)
	row.MeetingCount = s.countMeetingsForDevice(item.TenantID, item.ID)
	row.ActiveMeetingCount = s.countActiveMeetingsForDevice(item.TenantID, item.ID)
	return row
}

func (s *enterpriseDeviceService) createOrBindDeviceServiceCode(
	tx *gorm.DB,
	tenantID int64,
	device *models.Device,
	serviceCodeValue string,
	productID, modelID int64,
	existingServiceCode *models.ServiceCode,
	operator *dto.AuthPrincipal,
	now time.Time,
) error {
	if device == nil || device.ID <= 0 {
		return errorsx.InvalidParam("device is required")
	}
	if existingServiceCode != nil {
		locked := *existingServiceCode
		updates := map[string]any{
			"status":           enums.ServiceCodeStatusBound,
			"bound_at":         &now,
			"device_id":        device.ID,
			"product_id":       productID,
			"product_model_id": modelID,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       now,
		}
		if locked.ActivatedAt == nil {
			updates["activated_at"] = &now
		}
		if locked.EffectiveStartAt == nil {
			updates["effective_start_at"] = &now
		}
		return repositories.ServiceCodeRepository.Updates(tx, locked.ID, updates)
	}
	return repositories.ServiceCodeRepository.Create(tx, &models.ServiceCode{
		TenantID:         tenantID,
		ServiceCode:      serviceCodeValue,
		Mode:             enums.ServiceCodeModeTraceable,
		DeviceID:         device.ID,
		ProductID:        productID,
		ProductModelID:   modelID,
		Status:           enums.ServiceCodeStatusBound,
		ActivatedAt:      &now,
		BoundAt:          &now,
		EffectiveStartAt: &now,
		MetadataJSON:     "{}",
		AuditFields:      utils.BuildAuditFields(operator),
	})
}

func (s *enterpriseDeviceService) GetDetail(tenantID, deviceID int64) (*dto.EnterpriseDeviceDetailDTO, error) {
	return s.getDetail(tenantID, deviceID, enterpriseProductAccessScope{})
}

func (s *enterpriseDeviceService) GetDetailForOperator(tenantID, deviceID int64, operator *dto.AuthPrincipal) (*dto.EnterpriseDeviceDetailDTO, error) {
	return s.getDetail(tenantID, deviceID, resolveEnterpriseProductAccessScope(tenantID, operator))
}

func (s *enterpriseDeviceService) getDetail(tenantID, deviceID int64, scope enterpriseProductAccessScope) (*dto.EnterpriseDeviceDetailDTO, error) {
	if tenantID <= 0 || deviceID <= 0 {
		return nil, errorsx.InvalidParam("tenant and device are required")
	}
	if err := requireTenantDeviceFeature(tenantID); err != nil {
		return nil, err
	}
	item := repositories.DeviceRepository.Get(sqls.DB(), deviceID)
	if item == nil || item.TenantID != tenantID || item.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("device not found")
	}
	if !scope.canAccessProduct(item.ProductID) {
		return nil, errorsx.Forbidden("device is outside the engineer's authorized product scope")
	}
	base := s.buildListItem(*item)
	detail := &dto.EnterpriseDeviceDetailDTO{
		EnterpriseDeviceListItemDTO: base,
		InstallDate:                 base.InstallDate,
		InstallLocation:             item.InstallLocationJSON,
		Source:                      item.Source,
		StatusReason:                item.StatusReason,
		Description:                 item.MetadataJSON,
		SoftwareVersions:            s.buildSoftwareVersions(tenantID, deviceID),
		RepairHistory:               s.buildRepairHistory(tenantID, deviceID),
		RecentTickets:               s.buildRecentTickets(tenantID, deviceID),
		CustomerBindings:            s.buildCustomerBindings(tenantID, deviceID),
		CustomerVisible:             base.BindingCount > 0 || base.ServiceCode != "",
		CanCreateTicket:             true,
		CanStartMeeting:             base.OpenTicketCount > 0,
	}
	return detail, nil
}

func (s *enterpriseDeviceService) findDeviceServiceCode(tenantID, deviceID int64) *models.ServiceCode {
	return repositories.ServiceCodeRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("device_id", deviceID).
		Desc("bound_at").
		Desc("id"))
}

func (s *enterpriseDeviceService) findWarrantyStatus(tenantID, deviceID int64) (string, string) {
	record := repositories.DeviceWarrantyRecordRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("device_id", deviceID).
		Desc("end_at").
		Desc("id"))
	if record == nil {
		return "", "unknown"
	}
	end := record.EndAt
	status := "active"
	now := time.Now()
	if end.Before(now) {
		status = "expired"
	} else if end.Before(now.AddDate(0, 0, 60)) {
		status = "expiring"
	}
	return formatEnterpriseTime(end), status
}

func normalizeEnterpriseDeviceMetadata(metadataJSON, description string) (string, error) {
	metadataJSON = strings.TrimSpace(metadataJSON)
	if metadataJSON == "" {
		metadataJSON = "{}"
		if description := strings.TrimSpace(description); description != "" {
			payload, err := json.Marshal(map[string]string{"description": description})
			if err != nil {
				return "", err
			}
			metadataJSON = string(payload)
		}
	}
	return normalizeJSON(metadataJSON, "{}")
}

func parseOptionalEnterpriseDeviceDate(value, field string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range []string{
		time.DateOnly,
		time.RFC3339Nano,
		time.RFC3339,
		time.DateTime,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006/01/02",
		"2006/01/02 15:04:05",
	} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			date := enterpriseDeviceDateOnly(parsed)
			return &date, nil
		}
	}
	return nil, errorsx.InvalidParam(field + " is invalid")
}

func enterpriseDeviceDateOnly(value time.Time) time.Time {
	year, month, day := value.In(time.Local).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.Local)
}

func (s *enterpriseDeviceService) syncDeviceWarrantyRecord(tx *gorm.DB, tenantID int64, device *models.Device, warrantyEndValue string, clearWhenBlank bool, operator *dto.AuthPrincipal, now time.Time) error {
	warrantyEndValue = strings.TrimSpace(warrantyEndValue)
	if warrantyEndValue == "" {
		if clearWhenBlank {
			return s.deleteLatestWarrantyRecord(tx, tenantID, device.ID)
		}
		return nil
	}
	warrantyEnd, err := parseOptionalEnterpriseDeviceDate(warrantyEndValue, "warranty_end")
	if err != nil {
		return err
	}
	if warrantyEnd == nil {
		return nil
	}
	startAt := enterpriseDeviceDateOnly(now)
	if device.InstalledAt != nil && !device.InstalledAt.IsZero() {
		startAt = enterpriseDeviceDateOnly(*device.InstalledAt)
	}
	record := repositories.DeviceWarrantyRecordRepository.FindOne(tx, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("device_id", device.ID).
		Desc("end_at").
		Desc("id"))
	if record != nil && (device.InstalledAt == nil || device.InstalledAt.IsZero()) && !record.StartAt.IsZero() {
		startAt = enterpriseDeviceDateOnly(record.StartAt)
	}
	endAt := enterpriseDeviceDateOnly(*warrantyEnd)
	if endAt.Before(startAt) {
		return errorsx.InvalidParam("warranty_end must not be before install_date")
	}
	if record != nil {
		return repositories.DeviceWarrantyRecordRepository.Updates(tx, record.ID, map[string]any{
			"product_id":           device.ProductID,
			"product_model_id":     device.ProductModelID,
			"start_at":             startAt,
			"end_at":               endAt,
			"source":               "manual",
			"update_user_id":       operator.UserID,
			"update_user_name":     operator.Username,
			"updated_at":           now,
			"warranty_policy_json": firstNonEmptyString(record.WarrantyPolicyJSON, "{}"),
			"metadata_json":        firstNonEmptyString(record.MetadataJSON, "{}"),
		})
	}
	return repositories.DeviceWarrantyRecordRepository.Create(tx, &models.DeviceWarrantyRecord{
		TenantID:           tenantID,
		DeviceID:           device.ID,
		ProductID:          device.ProductID,
		ProductModelID:     device.ProductModelID,
		WarrantyType:       "standard",
		StartAt:            startAt,
		EndAt:              endAt,
		WarrantyPolicyJSON: "{}",
		Source:             "manual",
		MetadataJSON:       "{}",
		AuditFields:        utils.BuildAuditFields(operator),
	})
}

func (s *enterpriseDeviceService) deleteLatestWarrantyRecord(tx *gorm.DB, tenantID, deviceID int64) error {
	record := repositories.DeviceWarrantyRecordRepository.FindOne(tx, sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("device_id", deviceID).
		Desc("end_at").
		Desc("id"))
	if record == nil {
		return nil
	}
	return repositories.DeviceWarrantyRecordRepository.Delete(tx, record.ID)
}

func (s *enterpriseDeviceService) countDeviceTickets(tenantID, deviceID int64, includeClosed bool) int64 {
	cnd := sqls.NewCnd().Eq("tenant_id", tenantID).Eq("device_id", deviceID)
	if !includeClosed {
		cnd.Where("status not in ?", []enums.TicketStatus{enums.TicketStatusClosed, enums.TicketStatusDone, enums.TicketStatusCancelled})
	}
	return repositories.TicketRepository.Count(sqls.DB(), cnd)
}

func (s *enterpriseDeviceService) countActiveMeetingsForDevice(tenantID, deviceID int64) int64 {
	return s.countMeetingsForDeviceByStatus(tenantID, deviceID, "active")
}

func (s *enterpriseDeviceService) countMeetingsForDevice(tenantID, deviceID int64) int64 {
	return s.countMeetingsForDeviceByStatus(tenantID, deviceID)
}

func (s *enterpriseDeviceService) countMeetingsForDeviceByStatus(tenantID, deviceID int64, status ...string) int64 {
	tickets := repositories.TicketRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("device_id", deviceID))
	if len(tickets) == 0 {
		return 0
	}
	ticketIDs := make([]string, 0, len(tickets))
	for _, ticket := range tickets {
		ticketIDs = append(ticketIDs, fmt.Sprint(ticket.ID))
	}
	var count int64
	query := sqls.DB().Model(&models.MeetingRoomJitsi{}).
		Where("tenant_id = ? and ticket_id in ?", tenantID, ticketIDs)
	if len(status) > 0 {
		query = query.Where("status = ?", status[0])
	}
	query.Count(&count)
	return count
}

func (s *enterpriseDeviceService) buildSoftwareVersions(tenantID, deviceID int64) []dto.EnterpriseSoftwareVersionDTO {
	versions := repositories.DeviceSoftwareVersionRepository.GetLatestByDevice(sqls.DB(), tenantID, deviceID)
	ret := make([]dto.EnterpriseSoftwareVersionDTO, 0, len(versions))
	seen := make(map[string]bool)
	for _, item := range versions {
		key := item.ComponentType + "/" + item.ComponentName
		if seen[key] {
			continue
		}
		seen[key] = true
		component := item.ComponentName
		if component == "" {
			component = item.ComponentType
		}
		ret = append(ret, dto.EnterpriseSoftwareVersionDTO{
			ID:            item.ID,
			Component:     component,
			ComponentType: item.ComponentType,
			Version:       item.Version,
			InstalledAt:   formatEnterpriseTime(item.InstalledAt),
			Source:        item.Source,
		})
		if len(ret) >= 6 {
			break
		}
	}
	return ret
}

func (s *enterpriseDeviceService) buildRepairHistory(tenantID, deviceID int64) []dto.EnterpriseRepairHistoryDTO {
	records := repositories.TicketRepairRecordRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("device_id", deviceID).
		Desc("finished_at").
		Desc("updated_at").
		Desc("id"))
	ret := make([]dto.EnterpriseRepairHistoryDTO, 0, len(records))
	for _, item := range records {
		ticketNo := ""
		if ticket := repositories.TicketRepository.Get(sqls.DB(), item.TicketID); ticket != nil {
			ticketNo = ticket.TicketNo
		}
		ret = append(ret, dto.EnterpriseRepairHistoryDTO{
			ID:                item.ID,
			TicketID:          item.TicketID,
			TicketNo:          ticketNo,
			FaultType:         firstNonEmptyString(item.RootCause, item.ServiceMethod),
			Resolution:        firstNonEmptyString(item.Solution, item.Conclusion),
			RepairMethod:      item.RepairMethod,
			TechnicianName:    item.UpdateUserName,
			CompletedAt:       firstNonEmptyString(formatEnterpriseTimePtr(item.FinishedAt), formatEnterpriseTime(item.UpdatedAt)),
			VisibleToCustomer: item.VisibleToCustomer,
		})
		if len(ret) >= 10 {
			break
		}
	}
	return ret
}

func (s *enterpriseDeviceService) buildRecentTickets(tenantID, deviceID int64) []dto.EnterpriseTicketListItemDTO {
	tickets := repositories.TicketRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("device_id", deviceID).
		Desc("updated_at").
		Desc("id"))
	ret := make([]dto.EnterpriseTicketListItemDTO, 0, len(tickets))
	for _, ticket := range tickets {
		ret = append(ret, EnterpriseTicketService.buildListItem(ticket))
		if len(ret) >= 8 {
			break
		}
	}
	return ret
}

func (s *enterpriseDeviceService) buildCustomerBindings(tenantID, deviceID int64) []dto.EnterpriseCustomerBindingDTO {
	bindings := repositories.CustomerDeviceBindingRepository.FindByDeviceID(sqls.DB(), tenantID, deviceID)
	ret := make([]dto.EnterpriseCustomerBindingDTO, 0, len(bindings))
	for _, item := range bindings {
		name := ""
		if item.CustomerUserID > 0 {
			var user models.CustomerUser
			if err := sqls.DB().First(&user, "id = ? and tenant_id = ?", item.CustomerUserID, tenantID).Error; err == nil {
				name = firstNonEmptyString(user.DisplayName, user.Email, user.Phone)
			}
		}
		if name == "" && item.CustomerOrgID > 0 {
			var org models.CustomerOrg
			if err := sqls.DB().First(&org, "id = ? and tenant_id = ?", item.CustomerOrgID, tenantID).Error; err == nil {
				name = org.Name
			}
		}
		ret = append(ret, dto.EnterpriseCustomerBindingDTO{
			ID:             item.ID,
			CustomerOrgID:  item.CustomerOrgID,
			CustomerUserID: item.CustomerUserID,
			CustomerName:   name,
			BindingRole:    item.BindingRole,
			Source:         item.Source,
			Status:         mapStatusInt(item.Status),
			ConfirmedAt:    formatEnterpriseTimePtr(item.ConfirmedAt),
		})
	}
	return ret
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mapStatusInt(status enums.Status) string {
	switch status {
	case enums.StatusOk:
		return "active"
	case enums.StatusDeleted:
		return "deleted"
	default:
		return "inactive"
	}
}

func mapDeviceStatusForEnterprise(item models.Device) string {
	if item.Status == enums.StatusDeleted {
		return "inactive"
	}
	switch enums.DeviceStatus(item.DeviceStatus) {
	case enums.DeviceStatusOperational:
		return "active"
	case enums.DeviceStatusUnderMaintenance:
		return "maintenance"
	case enums.DeviceStatusArchived, enums.DeviceStatusDecommissioned:
		return "inactive"
	default:
		if item.Status == enums.StatusOk {
			return "active"
		}
		return "inactive"
	}
}

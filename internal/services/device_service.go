package services

import (
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

// Device lifecycle status mapping for backward compatibility
const (
	DeviceStatusNew              = "new"
	DeviceStatusOperational      = "operational"
	DeviceStatusUnderMaintenance = "under_maintenance"
	DeviceStatusDecommissioned   = "decommissioned"
	DeviceStatusArchived         = "archived"
)

var DeviceService = newDeviceService()

func newDeviceService() *deviceService {
	return &deviceService{}
}

type deviceService struct {
}

func (s *deviceService) Get(id int64) *models.Device {
	if id <= 0 {
		return nil
	}
	return repositories.DeviceRepository.Get(sqls.DB(), id)
}

func (s *deviceService) FindPageByCnd(cnd *sqls.Cnd) (list []models.Device, paging *sqls.Paging) {
	return repositories.DeviceRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *deviceService) CreateDevice(req request.CreateDeviceRequest, operator *dto.AuthPrincipal) (*models.Device, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if err := s.validateReferences(req.TenantID, req.ProductID, req.ProductModelID); err != nil {
		return nil, err
	}
	deviceNo := normalizeCode(req.DeviceNo)
	if deviceNo == "" {
		return nil, errorsx.InvalidParam("deviceNo is required")
	}
	existing := repositories.DeviceRepository.GetByTenantDeviceNo(sqls.DB(), req.TenantID, deviceNo)
	if existing != nil {
		return nil, errorsx.InvalidParam("deviceNo already exists")
	}
	installLocationJSON, err := normalizeJSON(req.InstallLocationJSON, "{}")
	if err != nil {
		return nil, err
	}
	metadataJSON, err := normalizeJSON(req.MetadataJSON, "{}")
	if err != nil {
		return nil, err
	}

	item := &models.Device{
		TenantID:            req.TenantID,
		DeviceNo:            deviceNo,
		ProductID:           req.ProductID,
		ProductModelID:      req.ProductModelID,
		SerialNo:            strings.TrimSpace(req.SerialNo),
		CustomerOrgID:       req.CustomerOrgID,
		ExternalDeviceID:    strings.TrimSpace(req.ExternalDeviceID),
		InstallLocationJSON: installLocationJSON,
		RegionCode:          strings.TrimSpace(req.RegionCode),
		Source:              strings.TrimSpace(req.Source),
		Status:              enums.StatusOk,
		MetadataJSON:        metadataJSON,
		AuditFields:         utils.BuildAuditFields(operator),
	}
	if item.Source == "" {
		item.Source = "manual"
	}
	if err := repositories.DeviceRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *deviceService) UpdateDevice(req request.UpdateDeviceRequest, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := s.Get(req.ID)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("device not found")
	}
	if err := s.validateReferences(req.TenantID, req.ProductID, req.ProductModelID); err != nil {
		return err
	}
	deviceNo := normalizeCode(req.DeviceNo)
	if deviceNo == "" {
		return errorsx.InvalidParam("deviceNo is required")
	}
	existing := repositories.DeviceRepository.GetByTenantDeviceNo(sqls.DB(), req.TenantID, deviceNo)
	if existing != nil && existing.ID != req.ID {
		return errorsx.InvalidParam("deviceNo already exists")
	}
	installLocationJSON, err := normalizeJSON(req.InstallLocationJSON, "{}")
	if err != nil {
		return err
	}
	metadataJSON, err := normalizeJSON(req.MetadataJSON, "{}")
	if err != nil {
		return err
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "manual"
	}

	return repositories.DeviceRepository.Updates(sqls.DB(), req.ID, map[string]any{
		"tenant_id":             req.TenantID,
		"device_no":             deviceNo,
		"product_id":            req.ProductID,
		"product_model_id":      req.ProductModelID,
		"serial_no":             strings.TrimSpace(req.SerialNo),
		"customer_org_id":       req.CustomerOrgID,
		"external_device_id":    strings.TrimSpace(req.ExternalDeviceID),
		"install_location_json": installLocationJSON,
		"region_code":           strings.TrimSpace(req.RegionCode),
		"source":                source,
		"metadata_json":         metadataJSON,
		"update_user_id":        operator.UserID,
		"update_user_name":      operator.Username,
		"updated_at":            time.Now(),
	})
}

func (s *deviceService) DeleteDevice(id int64, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := s.Get(id)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("device not found")
	}
	return repositories.DeviceRepository.Updates(sqls.DB(), id, map[string]any{
		"status":           enums.StatusDeleted,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	})
}

func (s *deviceService) UpdateStatus(id int64, status int, operator *dto.AuthPrincipal) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if !isMutableStatus(status) {
		return errorsx.InvalidParam("invalid status")
	}
	item := s.Get(id)
	if item == nil || item.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("device not found")
	}
	return repositories.DeviceRepository.Updates(sqls.DB(), id, map[string]any{
		"status":           status,
		"update_user_id":   operator.UserID,
		"update_user_name": operator.Username,
		"updated_at":       time.Now(),
	})
}

func (s *deviceService) validateReferences(tenantID int64, productID int64, productModelID int64) error {
	if _, err := requireActiveTenant(tenantID); err != nil {
		return err
	}
	if productID <= 0 {
		return errorsx.InvalidParam("productId is required")
	}
	product := repositories.ProductRepository.Get(sqls.DB(), productID)
	if product == nil || product.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product not found")
	}
	if product.TenantID != tenantID {
		return errorsx.InvalidParam("product does not belong to tenant")
	}
	if productModelID <= 0 {
		return nil
	}
	model := repositories.ProductModelRepository.Get(sqls.DB(), productModelID)
	if model == nil || model.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("product model not found")
	}
	if model.TenantID != tenantID || model.ProductID != productID {
		return errorsx.InvalidParam("product model does not belong to product")
	}
	return nil
}

// TransitionDeviceStatus 设备生命周期状态转换
func (s *deviceService) TransitionDeviceStatus(deviceId int64, targetStatus string, operator *dto.AuthPrincipal, reason string) error {
	if operator == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if deviceId <= 0 {
		return errorsx.InvalidParam("deviceId is required")
	}

	fromStatus := enums.DeviceStatus(targetStatus)
	device := s.Get(deviceId)
	if device == nil || device.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("device not found")
	}

	currentStatus := enums.DeviceStatus(device.DeviceStatus)
	if currentStatus == "" {
		currentStatus = enums.DeviceStatusNew
	}

	// 校验转换是否合法
	if !enums.IsValidDeviceStatusTransition(currentStatus, fromStatus) {
		return errorsx.BusinessError(3,
			fmt.Sprintf("cannot transition device status from %s to %s", currentStatus, fromStatus))
	}

	now := time.Now()
	updates := map[string]any{
		"device_status":     string(fromStatus),
		"status_changed_at": &now,
		"status_reason":     reason,
		"update_user_id":    operator.UserID,
		"update_user_name":  operator.Username,
		"updated_at":        now,
	}

	// 根据目标状态设置额外时间戳
	switch fromStatus {
	case enums.DeviceStatusDecommissioned:
		updates["decommissioned_at"] = &now
	case enums.DeviceStatusArchived:
		updates["archived_at"] = &now
	}

	return repositories.DeviceRepository.Updates(sqls.DB(), deviceId, updates)
}

// ValidateDeviceForTicketCreation 校验设备是否允许创建工单
func (s *deviceService) ValidateDeviceForTicketCreation(deviceId int64) error {
	if deviceId <= 0 {
		return nil
	}
	device := s.Get(deviceId)
	if device == nil || device.Status == enums.StatusDeleted {
		return errorsx.InvalidParam("device not found")
	}
	deviceStatus := enums.DeviceStatus(device.DeviceStatus)
	if deviceStatus == enums.DeviceStatusDecommissioned || deviceStatus == enums.DeviceStatusArchived {
		return errorsx.BusinessError(4,
			fmt.Sprintf("cannot create ticket for device in %s status", deviceStatus))
	}
	if deviceStatus == enums.DeviceStatusNew {
		return errorsx.BusinessError(5,
			"cannot create ticket for device in new status, please activate the device first")
	}
	return nil
}

// TransferDeviceOwnership 设备所有权转移
func (s *deviceService) TransferDeviceOwnership(deviceId int64, targetCustomerOrgID int64, targetCustomerUserID int64, operator *dto.AuthPrincipal, req request.TransferDeviceOwnershipRequest) (*models.Device, error) {
	if operator == nil {
		return nil, errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if deviceId <= 0 {
		return nil, errorsx.InvalidParam("deviceId is required")
	}
	if targetCustomerOrgID <= 0 {
		return nil, errorsx.InvalidParam("targetCustomerOrgId is required")
	}

	device := s.Get(deviceId)
	if device == nil || device.Status == enums.StatusDeleted {
		return nil, errorsx.InvalidParam("device not found")
	}

	// 校验目标客户组织
	if _, err := requireActiveTenant(device.TenantID); err != nil {
		return nil, err
	}

	// 记录旧所有者
	oldCustomerOrgID := device.CustomerOrgID

	// 在事务中执行转移
	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		now := time.Now()

		// 1. 取消旧的绑定关系
		oldBindings := repositories.CustomerDeviceBindingRepository.FindByDeviceID(ctx.Tx, device.TenantID, deviceId)
		for _, binding := range oldBindings {
			if err := repositories.CustomerDeviceBindingRepository.Updates(ctx.Tx, binding.ID, map[string]any{
				"status":              int(enums.StatusDeleted),
				"previous_owner_id":   binding.CustomerOrgID,
				"previous_owner_type": "customer_org",
				"updated_at":          now,
			}); err != nil {
				return err
			}
		}

		// 2. 创建新的绑定关系
		newBinding := &models.CustomerDeviceBinding{
			TenantID:       device.TenantID,
			CustomerOrgID:  targetCustomerOrgID,
			CustomerUserID: targetCustomerUserID,
			DeviceID:       deviceId,
			BindingRole:    "owner",
			Source:         "transfer",
			Status:         enums.StatusOk,
			ConfirmedAt:    &now,
			AuditFields:    utils.BuildAuditFields(operator),
		}
		if err := repositories.CustomerDeviceBindingRepository.Create(ctx.Tx, newBinding); err != nil {
			return err
		}

		// 3. 更新设备信息
		transferCount := device.TransferCount + 1
		if err := repositories.DeviceRepository.Updates(ctx.Tx, deviceId, map[string]any{
			"customer_org_id":  targetCustomerOrgID,
			"device_status":    DeviceStatusOperational,
			"transfer_count":   transferCount,
			"update_user_id":   operator.UserID,
			"update_user_name": operator.Username,
			"updated_at":       now,
		}); err != nil {
			return err
		}

		// 4. 记录事件（写入 outbox 或直接记录）
		eventPayload := fmt.Sprintf(`{"deviceId":%d,"fromOrgId":%d,"toOrgId":%d,"fromUserId":%d,"toUserId":%d,"transferReason":"%s","preserveServiceHistory":%v}`,
			deviceId, oldCustomerOrgID, targetCustomerOrgID, device.CustomerOrgID, targetCustomerUserID,
			req.TransferReason, req.PreserveServiceHistory)

		// 写入通知：通知旧所有者
		if req.NotifyCurrentOwner && oldCustomerOrgID > 0 {
			// 设备转移通知发送
			_ = eventPayload // 实际项目中通过消息队列发送通知
		}

		// 写入通知：通知新所有者
		if req.NotifyNewOwner {
			// 新所有者通知发送
			_ = eventPayload
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	updated := s.Get(deviceId)
	return updated, nil
}

package dashboard

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
)

// DeviceBatchOperationResult 批量操作结果
type DeviceBatchOperationResult struct {
	Total     int                         `json:"total"`
	Succeeded int                         `json:"succeeded"`
	Failed    int                         `json:"failed"`
	Errors    []DeviceBatchOperationError `json:"errors,omitempty"`
}

// DeviceBatchOperationError 批量操作错误详情
type DeviceBatchOperationError struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

// batchImportDeviceRow CSV/JSON 导入的设备行数据
type batchImportDeviceRow struct {
	DeviceNo            string `json:"deviceNo"`
	ProductID           int64  `json:"productId"`
	ProductModelID      int64  `json:"productModelId"`
	SerialNo            string `json:"serialNo"`
	CustomerOrgID       int64  `json:"customerOrgId"`
	ExternalDeviceID    string `json:"externalDeviceId"`
	InstallLocationJSON string `json:"installLocationJson"`
	RegionCode          string `json:"regionCode"`
	Source              string `json:"source"`
}

// DeviceBatchPostCreate 批量导入设备
// POST /api/enterprise/device/batch-create
func DeviceBatchPostCreate(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionDeviceCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	// tenant_id 从 AuthPrincipal 派生，不接受客户端传入
	tenantID := user.TenantID
	if tenantID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("tenantId is required"))
		return
	}

	contentType := ctx.GetHeader("Content-Type")
	var devices []batchImportDeviceRow

	if strings.Contains(contentType, "text/csv") || strings.Contains(contentType, "application/csv") {
		devices, err = parseCSVImport(ctx.Request.Body)
		if err != nil {
			httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid csv format: "+err.Error()))
			return
		}
	} else {
		// JSON format
		req := struct {
			Devices []batchImportDeviceRow `json:"devices"`
		}{}
		if err := params.ReadJSON(ctx, &req); err != nil {
			httpx.WriteJSON(ctx, err)
			return
		}
		devices = req.Devices
	}

	if len(devices) == 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("no devices to import"))
		return
	}

	result := DeviceBatchOperationResult{
		Total:  len(devices),
		Errors: make([]DeviceBatchOperationError, 0),
	}

	for i, d := range devices {
		createreq := request.CreateDeviceRequest{
			TenantID:            tenantID,
			DeviceNo:            d.DeviceNo,
			ProductID:           d.ProductID,
			ProductModelID:      d.ProductModelID,
			SerialNo:            d.SerialNo,
			CustomerOrgID:       d.CustomerOrgID,
			ExternalDeviceID:    d.ExternalDeviceID,
			InstallLocationJSON: d.InstallLocationJSON,
			RegionCode:          d.RegionCode,
			Source:              d.Source,
		}
		if _, err := services.DeviceService.CreateDevice(createreq, user); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, DeviceBatchOperationError{
				Row:    i + 1,
				Reason: err.Error(),
			})
		} else {
			result.Succeeded++
		}
	}

	httpx.WriteJSON(ctx, result)
}

// DeviceBatchPostUpdateWarranty 批量更新保修信息
// POST /api/enterprise/device/batch-update-warranty
func DeviceBatchPostUpdateWarranty(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionDeviceUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.BatchUpdateDeviceWarrantyRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	if len(req.DeviceIDs) == 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("deviceIds is required"))
		return
	}

	result := DeviceBatchOperationResult{
		Total:  len(req.DeviceIDs),
		Errors: make([]DeviceBatchOperationError, 0),
	}

	for i, deviceID := range req.DeviceIDs {
		device := services.DeviceService.Get(deviceID)
		if device == nil || device.Status == enums.StatusDeleted {
			result.Failed++
			result.Errors = append(result.Errors, DeviceBatchOperationError{
				Row:    i + 1,
				Reason: fmt.Sprintf("device %d not found", deviceID),
			})
			continue
		}

		// 更新设备元数据中的保修信息
		metadata := map[string]any{}
		if device.MetadataJSON != "" && device.MetadataJSON != "{}" {
			if err := json.Unmarshal([]byte(device.MetadataJSON), &metadata); err != nil {
				metadata = map[string]any{}
			}
		}
		metadata["warranty_end_at"] = req.WarrantyEndAt
		metadata["warranty_policy"] = req.WarrantyPolicy
		metadataBytes, _ := json.Marshal(metadata)

		updates := map[string]any{
			"metadata_json":    string(metadataBytes),
			"update_user_id":   user.UserID,
			"update_user_name": user.Username,
			"updated_at":       time.Now(),
		}

		if err := repositories.DeviceRepository.Updates(sqls.DB(), deviceID, updates); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, DeviceBatchOperationError{
				Row:    i + 1,
				Reason: err.Error(),
			})
		} else {
			result.Succeeded++
		}
	}

	httpx.WriteJSON(ctx, result)
}

// DeviceBatchPostTransfer 批量转移设备所有权
// POST /api/enterprise/device/batch-transfer
func DeviceBatchPostTransfer(ctx *gin.Context) {
	user, err := services.AuthService.RequirePermission(ctx, constants.PermissionDeviceUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.BatchTransferDeviceRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	if len(req.DeviceIDs) == 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("deviceIds is required"))
		return
	}

	result := DeviceBatchOperationResult{
		Total:  len(req.DeviceIDs),
		Errors: make([]DeviceBatchOperationError, 0),
	}

	for i, deviceID := range req.DeviceIDs {
		transferReq := request.TransferDeviceOwnershipRequest{
			DeviceID:               deviceID,
			TargetCustomerOrgID:    req.TargetCustomerOrgID,
			TargetCustomerUserID:   req.TargetCustomerUserID,
			TransferReason:         req.TransferReason,
			PreserveServiceHistory: req.PreserveServiceHistory,
			NotifyCurrentOwner:     req.NotifyCurrentOwner,
			NotifyNewOwner:         req.NotifyNewOwner,
		}

		if _, err := services.DeviceService.TransferDeviceOwnership(deviceID, req.TargetCustomerOrgID, req.TargetCustomerUserID, user, transferReq); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, DeviceBatchOperationError{
				Row:    i + 1,
				Reason: err.Error(),
			})
		} else {
			result.Succeeded++
		}
	}

	httpx.WriteJSON(ctx, result)
}

// parseCSVImport 解析 CSV 格式的设备导入数据
func parseCSVImport(reader io.Reader) ([]batchImportDeviceRow, error) {
	r := csv.NewReader(reader)
	r.TrimLeadingSpace = true

	headers, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("cannot read csv headers: %w", err)
	}

	// Build header index
	headerMap := make(map[string]int)
	for i, h := range headers {
		headerMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	var devices []batchImportDeviceRow
	lineNo := 1

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv parse error at line %d: %w", lineNo+1, err)
		}
		lineNo++

		d := batchImportDeviceRow{}
		if idx, ok := headerMap["deviceno"]; ok && idx < len(record) {
			d.DeviceNo = strings.TrimSpace(record[idx])
		}
		if idx, ok := headerMap["productid"]; ok && idx < len(record) {
			d.ProductID, _ = strconv.ParseInt(strings.TrimSpace(record[idx]), 10, 64)
		}
		if idx, ok := headerMap["productmodelid"]; ok && idx < len(record) {
			d.ProductModelID, _ = strconv.ParseInt(strings.TrimSpace(record[idx]), 10, 64)
		}
		if idx, ok := headerMap["serialno"]; ok && idx < len(record) {
			d.SerialNo = strings.TrimSpace(record[idx])
		}
		if idx, ok := headerMap["customerorgid"]; ok && idx < len(record) {
			d.CustomerOrgID, _ = strconv.ParseInt(strings.TrimSpace(record[idx]), 10, 64)
		}
		if idx, ok := headerMap["externaldeviceid"]; ok && idx < len(record) {
			d.ExternalDeviceID = strings.TrimSpace(record[idx])
		}
		if idx, ok := headerMap["regioncode"]; ok && idx < len(record) {
			d.RegionCode = strings.TrimSpace(record[idx])
		}
		if d.Source == "" {
			d.Source = "batch_import"
		}
		devices = append(devices, d)
	}

	return devices, nil
}

package enterprise

import (
	"strconv"
	"strings"

	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func DeviceList(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	filter := parseEnterpriseFilter(ctx.Query("filter"))
	productID, _ := strconv.ParseInt(ctx.Query("product_id"), 10, 64)
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", ctx.DefaultQuery("limit", "20")))
	query := services.EnterpriseDeviceQuery{
		Search:    firstNonEmpty(ctx.Query("search"), strings.TrimPrefix(filter["device_no"], "~")),
		Status:    firstNonEmpty(ctx.Query("status"), filter["status"]),
		ProductID: productID,
		Page:      page,
		PageSize:  pageSize,
	}
	result, err := services.EnterpriseDeviceService.ListForOperator(tenantID, query, enterpriseActionOperator(ctx, tenantID))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func DeviceCreate(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req dto.EnterpriseDeviceCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.EnterpriseDeviceService.CreateForOperator(tenantID, req, enterpriseActionOperator(ctx, tenantID))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

func DeviceUpdate(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	deviceID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || deviceID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid device id"))
		return
	}
	var req dto.EnterpriseDeviceUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.EnterpriseDeviceService.UpdateForOperator(tenantID, deviceID, req, enterpriseActionOperator(ctx, tenantID))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

func DeviceBatchImport(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req dto.EnterpriseDeviceBatchCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.EnterpriseDeviceService.BatchCreateForOperator(tenantID, req, enterpriseActionOperator(ctx, tenantID))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func DeviceGet(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	deviceID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || deviceID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid device id"))
		return
	}
	item, err := services.EnterpriseDeviceService.GetDetailForOperator(tenantID, deviceID, enterpriseActionOperator(ctx, tenantID))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

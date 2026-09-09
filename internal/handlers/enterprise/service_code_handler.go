package enterprise

import (
	"strconv"
	"strings"

	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func ServiceCodeList(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	filter := parseEnterpriseFilter(ctx.Query("filter"))
	productID, _ := strconv.ParseInt(ctx.Query("product_id"), 10, 64)
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", ctx.DefaultQuery("limit", "50")))
	query := services.EnterpriseServiceCodeQuery{
		Search:    firstNonEmpty(ctx.Query("search"), strings.TrimPrefix(filter["service_code"], "~")),
		Status:    firstNonEmpty(ctx.Query("status"), filter["status"]),
		ProductID: productID,
		Page:      page,
		PageSize:  pageSize,
	}
	result, err := services.EnterpriseServiceCodeService.ListCodes(tenantID, query)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func ServiceCodeBatchList(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	filter := parseEnterpriseFilter(ctx.Query("filter"))
	productID, _ := strconv.ParseInt(ctx.Query("product_id"), 10, 64)
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", ctx.DefaultQuery("limit", "50")))
	query := services.EnterpriseServiceCodeQuery{
		Search:    firstNonEmpty(ctx.Query("search"), strings.TrimPrefix(filter["batch_no"], "~")),
		ProductID: productID,
		Page:      page,
		PageSize:  pageSize,
	}
	result, err := services.EnterpriseServiceCodeService.ListBatches(tenantID, query)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

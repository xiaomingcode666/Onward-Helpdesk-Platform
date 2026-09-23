package enterprise

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"
)

func TicketQualitySampleList(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))
	result, err := services.TicketQualitySampleService.List(tenantID, servicesTicketQualityQuery(page, pageSize, ctx.Query("period_key"), ctx.Query("status"), ctx.Query("reason"), ctx.Query("search")))
	if err != nil {
		httpxWriteQualityJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

func TicketQualitySampleGenerate(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	var req struct {
		PeriodKey   string `json:"period_key"`
		RandomCount int    `json:"random_count"`
	}
	_ = ctx.ShouldBindJSON(&req)
	created, err := services.TicketQualitySampleService.Generate(tenantID, strings.TrimSpace(req.PeriodKey), req.RandomCount, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpxWriteQualityJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, gin.H{"created": created})
}

func TicketQualitySampleStart(ctx *gin.Context)    { ticketQualitySampleTransition(ctx, false) }
func TicketQualitySampleComplete(ctx *gin.Context) { ticketQualitySampleTransition(ctx, true) }

func ticketQualitySampleTransition(ctx *gin.Context, complete bool) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpxWriteQualityJSON(ctx, errorsx.InvalidParam("invalid sample id"))
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	_ = ctx.ShouldBindJSON(&req)
	operator := services.AuthService.GetAuthPrincipal(ctx)
	reviewerID := int64(0)
	if operator != nil {
		reviewerID = operator.UserID
	}
	if complete {
		err = services.TicketQualitySampleService.Complete(tenantID, id, reviewerID, req.Note)
	} else {
		err = services.TicketQualitySampleService.Start(tenantID, id, reviewerID)
	}
	if err != nil {
		httpxWriteQualityJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, gin.H{"success": true})
}

func servicesTicketQualityQuery(page, pageSize int, periodKey, status, reason, search string) services.TicketQualitySampleQueryForHandler {
	return services.TicketQualitySampleQueryForHandler{Page: page, PageSize: pageSize, PeriodKey: periodKey, Status: status, Reason: reason, Search: search}
}

func httpxWriteQualityJSON(ctx *gin.Context, err error) {
	httpx.WriteJSON(ctx, err)
}

package enterprise

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

// ReportGetOverview 运营概览
func ReportGetOverview(ctx *gin.Context) {
	tenantID := resolveTenantID(ctx)

	overview, err := services.ReportService.GetDashboardOverview(ctx, tenantID)
	if err != nil {
		slog.Error("failed to get dashboard overview", "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}

	httpx.WriteJSON(ctx, overview)
}

// ReportGetProductReport 产品报表
func ReportGetProductReport(ctx *gin.Context) {
	tenantID := resolveTenantID(ctx)
	productID := ctx.Param("id")
	if productID == "" {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "product id is required"))
		return
	}

	startDate, endDate := parseDateRange(ctx.Query("start"), ctx.Query("end"), 30)

	report, err := services.ReportService.GetProductReport(ctx, tenantID, productID, startDate, endDate)
	if err != nil {
		slog.Error("failed to get product report", "productID", productID, "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}

	httpx.WriteJSON(ctx, report)
}

// ReportGetTrends 趋势数据
func ReportGetTrends(ctx *gin.Context) {
	tenantID := resolveTenantID(ctx)
	productID := ctx.Query("productId")
	metric := ctx.DefaultQuery("metric", "tickets")
	period := ctx.DefaultQuery("period", "30d")

	startDate, endDate := parsePeriod(period)

	trends, err := services.ReportService.GetTrendData(ctx, tenantID, productID, metric, startDate, endDate)
	if err != nil {
		slog.Error("failed to get trend data", "metric", metric, "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}

	httpx.WriteJSON(ctx, trends)
}

// ReportGetTopFailures 故障排行榜
func ReportGetTopFailures(ctx *gin.Context) {
	tenantID := resolveTenantID(ctx)
	limitStr := ctx.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)

	ranks, err := services.ReportService.GetTopFailureProducts(ctx, tenantID, limit)
	if err != nil {
		slog.Error("failed to get top failure products", "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}

	httpx.WriteJSON(ctx, ranks)
}

// ReportGetTeamPerformance 团队绩效
func ReportGetTeamPerformance(ctx *gin.Context) {
	tenantID := resolveTenantID(ctx)

	startDate, endDate := parseDateRange(ctx.Query("start"), ctx.Query("end"), 30)

	performances, err := services.ReportService.GetTeamPerformance(ctx, tenantID, startDate, endDate)
	if err != nil {
		slog.Error("failed to get team performance", "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}

	httpx.WriteJSON(ctx, performances)
}

// ReportGetSupplierPerformance 供应商考评
func ReportGetSupplierPerformance(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	startDate, endDate := parseDateRange(ctx.Query("start"), ctx.Query("end"), 30)
	performances, err := services.ReportService.GetSupplierPerformance(tenantID, startDate, endDate)
	if err != nil {
		slog.Error("failed to get supplier performance", "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildSupplierPerformance(performances))
}

// ReportGetExport 导出报表
func ReportGetExport(ctx *gin.Context) {
	tenantID := resolveTenantID(ctx)
	exportType := ctx.DefaultQuery("type", "csv")
	startDate, endDate := parseDateRange(ctx.Query("start"), ctx.Query("end"), 30)
	productID := ctx.Query("productId")

	switch exportType {
	case "csv":
		exportCSV(ctx, tenantID, productID, startDate, endDate)
	default:
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "unsupported export type"))
	}
}

// WebhookPostRegister 注册 Webhook
func WebhookPostRegister(ctx *gin.Context) {
	var req struct {
		Name   string   `json:"name" binding:"required"`
		URL    string   `json:"url" binding:"required"`
		Events []string `json:"events" binding:"required"`
		Secret string   `json:"secret"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	tenantID := resolveTenantID(ctx)
	eventsJSON := "[" + strings.Join(escapeJSONStrings(req.Events), ",") + "]"

	endpoint, err := services.WebhookService.RegisterWebhook(ctx, tenantID, req.Name, req.URL, eventsJSON, req.Secret)
	if err != nil {
		slog.Error("failed to register webhook", "error", err)
		httpx.WriteJSON(ctx, err)
		return
	}

	httpx.WriteJSON(ctx, gin.H{
		"id":     endpoint.ID,
		"name":   endpoint.Name,
		"url":    endpoint.URL,
		"events": req.Events,
		"active": endpoint.Active,
	})
}

// WebhookGetList Webhook 列表
func WebhookGetList(ctx *gin.Context) {
	endpoints, err := services.WebhookService.ListWebhooks(resolveTenantID(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, endpoints)
}

// WebhookPostToggle 启用/禁用 Webhook
func WebhookPostToggle(ctx *gin.Context) {
	webhookID := ctx.Param("webhookId")
	var req struct {
		Active bool `json:"active"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.WebhookService.SetWebhookActive(resolveTenantID(ctx), webhookID, req.Active); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, gin.H{"success": true})
}

// parsePeriod 解析时间段
func parsePeriod(period string) (time.Time, time.Time) {
	now := time.Now()
	end := now
	days := 30
	switch {
	case strings.HasSuffix(period, "d"):
		days, _ = strconv.Atoi(strings.TrimSuffix(period, "d"))
	case strings.HasSuffix(period, "m"):
		months, _ := strconv.Atoi(strings.TrimSuffix(period, "m"))
		days = months * 30
	case strings.HasSuffix(period, "y"):
		years, _ := strconv.Atoi(strings.TrimSuffix(period, "y"))
		days = years * 365
	}
	if days <= 0 {
		days = 30
	}
	start := now.AddDate(0, 0, -days)
	return start, end
}

// parseDateRange 解析日期范围
func parseDateRange(startStr, endStr string, defaultDays int) (time.Time, time.Time) {
	end := time.Now()
	if endStr != "" {
		if parsed, err := time.Parse("2006-01-02", endStr); err == nil {
			end = parsed.Add(24*time.Hour - time.Second)
		}
	}
	start := end.AddDate(0, 0, -defaultDays)
	if startStr != "" {
		if parsed, err := time.Parse("2006-01-02", startStr); err == nil {
			start = parsed
		}
	}
	return start, end
}

// escapeJSONStrings 转义字符串数组为 JSON 字符串元素
func escapeJSONStrings(items []string) []string {
	result := make([]string, len(items))
	for i, item := range items {
		result[i] = `"` + strings.ReplaceAll(item, `"`, `\"`) + `"`
	}
	return result
}

// exportCSV 导出 CSV 报表
func exportCSV(ctx *gin.Context, tenantID, productID string, startDate, endDate time.Time) {
	report, err := services.ReportService.GetProductReport(ctx, tenantID, productID, startDate, endDate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	ctx.Header("Content-Type", "text/csv; charset=utf-8")
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=report_%s.csv", time.Now().Format("20060102_150405")))

	writer := csv.NewWriter(ctx.Writer)
	writer.Write([]string{"指标", "数值"})
	writer.Write([]string{"产品名称", report.ProductName})
	writer.Write([]string{"总工单数", fmt.Sprintf("%d", report.TotalTickets)})
	writer.Write([]string{"开放工单", fmt.Sprintf("%d", report.OpenTickets)})
	writer.Write([]string{"已关闭工单", fmt.Sprintf("%d", report.ClosedTickets)})
	writer.Write([]string{"平均解决时长(小时)", fmt.Sprintf("%.2f", report.AvgResolutionHours)})
	writer.Write([]string{"AI诊断会话数", fmt.Sprintf("%d", report.AISessions)})
	writer.Write([]string{"AI解决率(%)", fmt.Sprintf("%.2f", report.AIResolveRate)})
	writer.Write([]string{"平均置信度(%)", fmt.Sprintf("%.2f", report.AvgConfidence)})
	writer.Write([]string{"转人工率(%)", fmt.Sprintf("%.2f", report.EscalationRate)})
	writer.Write([]string{"知识条目数", fmt.Sprintf("%d", report.KnowledgeEntries)})
	writer.Write([]string{"知识命中率(%)", fmt.Sprintf("%.2f", report.KnowledgeHitRate)})
	writer.Write([]string{"会议数", fmt.Sprintf("%d", report.TotalMeetings)})
	writer.Write([]string{"会议总时长(分钟)", fmt.Sprintf("%d", report.TotalMeetingMinutes)})
	writer.Write([]string{"输入Token数", fmt.Sprintf("%d", report.TotalTokensIn)})
	writer.Write([]string{"输出Token数", fmt.Sprintf("%d", report.TotalTokensOut)})
	writer.Write([]string{"预估成本", fmt.Sprintf("%.4f", report.EstimatedCost)})
	writer.Flush()
}

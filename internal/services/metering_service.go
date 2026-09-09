package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/eventbus"

	"github.com/mlogclub/simple/sqls"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ---------- 返回数据类型 ----------

// UsageDetail 每日用量明细
type UsageDetail struct {
	Date         string  `json:"date"`
	RequestCount int64   `json:"request_count"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	Cost         float64 `json:"cost"`
}

// UsageSummary 用量汇总
type UsageSummary struct {
	TotalRequests     int64         `json:"total_requests"`
	TotalInputTokens  int64         `json:"total_input_tokens"`
	TotalOutputTokens int64         `json:"total_output_tokens"`
	TotalCost         float64       `json:"total_cost"`
	Details           []UsageDetail `json:"details,omitempty"`
}

// ---------- Service ----------

var MeteringService = newMeteringService()

func newMeteringService() *meteringService {
	return &meteringService{}
}

type meteringService struct{}

// RecordUsage 记录一次 Sub2API 用量事件
//
// 使用 request_id 做幂等去重：相同 request_id 重复调用不会产生重复记录。
func (s *meteringService) RecordUsage(
	_ context.Context,
	tenantID, productID int64,
	apiKeyID, usageType, requestID string,
	tokensIn, tokensOut int64,
	costAmount float64,
) error {
	if tenantID <= 0 {
		return errorsx.InvalidParam("tenant_id is required")
	}
	if productID < 0 {
		return errorsx.InvalidParam("product_id is invalid")
	}
	if requestID == "" {
		return errorsx.InvalidParam("request_id is required for idempotent usage recording")
	}

	usageType = strings.TrimSpace(usageType)
	if usageType == "" {
		usageType = "chat_completion"
	}

	now := time.Now()

	event := &models.ProductAIUsageEvent{
		TenantID:     tenantID,
		ProductID:    productID,
		APIKeyID:     strings.TrimSpace(apiKeyID),
		UsageType:    usageType,
		InputTokens:  tokensIn,
		OutputTokens: tokensOut,
		CostAmount:   costAmount,
		RequestID:    requestID,
		OccurredAt:   now,
		CreatedAt:    now,
	}

	// 先检查是否已存在（幂等）
	var existing models.ProductAIUsageEvent
	result := sqls.DB().Where("request_id = ?", requestID).Take(&existing)
	if result.Error == nil {
		// 已存在，幂等返回
		return nil
	}
	if result.Error != gorm.ErrRecordNotFound {
		return fmt.Errorf("metering: check duplicate failed: %w", result.Error)
	}

	if err := sqls.DB().Create(event).Error; err != nil {
		return fmt.Errorf("metering: record usage failed: %w", err)
	}
	return nil
}

// GetUsageSummary 查询指定时间范围内的用量汇总
func (s *meteringService) GetUsageSummary(
	tenantID, productID int64,
	startDate, endDate time.Time,
) (*UsageSummary, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant_id is required")
	}

	db := sqls.DB()

	// 查询汇总
	type summaryRow struct {
		TotalRequests  int64
		TotalInTokens  int64
		TotalOutTokens int64
		TotalCost      float64
	}
	var summary summaryRow

	query := db.Model(&models.ProductAIUsageEvent{}).
		Select("COUNT(*) AS total_requests, COALESCE(SUM(input_tokens), 0) AS total_in_tokens, COALESCE(SUM(output_tokens), 0) AS total_out_tokens, COALESCE(SUM(cost_amount), 0) AS total_cost").
		Where("tenant_id = ?", tenantID).
		Where("occurred_at >= ?", startDate.Format(time.RFC3339)).
		Where("occurred_at < ?", endDate.Format(time.RFC3339))

	if productID > 0 {
		query = query.Where("product_id = ?", productID)
	}

	if err := query.Scan(&summary).Error; err != nil {
		return nil, fmt.Errorf("metering: query usage summary failed: %w", err)
	}

	// 查询每日明细
	type dailyRow struct {
		Date    string
		Count   int64
		InTok   int64
		OutTok  int64
		CostAmt float64
	}
	var dailyRows []dailyRow

	dailyQuery := db.Model(&models.ProductAIUsageEvent{}).
		Select("DATE(occurred_at) AS date, COUNT(*) AS count, COALESCE(SUM(input_tokens), 0) AS in_tok, COALESCE(SUM(output_tokens), 0) AS out_tok, COALESCE(SUM(cost_amount), 0) AS cost_amt").
		Where("tenant_id = ?", tenantID).
		Where("occurred_at >= ?", startDate.Format(time.RFC3339)).
		Where("occurred_at < ?", endDate.Format(time.RFC3339)).
		Group("DATE(occurred_at)").
		Order("date ASC")

	if productID > 0 {
		dailyQuery = dailyQuery.Where("product_id = ?", productID)
	}

	if err := dailyQuery.Scan(&dailyRows).Error; err != nil {
		return nil, fmt.Errorf("metering: query daily usage failed: %w", err)
	}

	details := make([]UsageDetail, 0, len(dailyRows))
	for _, r := range dailyRows {
		details = append(details, UsageDetail{
			Date:         r.Date,
			RequestCount: r.Count,
			InputTokens:  r.InTok,
			OutputTokens: r.OutTok,
			Cost:         r.CostAmt,
		})
	}

	return &UsageSummary{
		TotalRequests:     summary.TotalRequests,
		TotalInputTokens:  summary.TotalInTokens,
		TotalOutputTokens: summary.TotalOutTokens,
		TotalCost:         summary.TotalCost,
		Details:           details,
	}, nil
}

// RecordUsageEvent 记录一次 Sub2API 用量事件（新 UsageEvent 模型）
//
// 使用 EventID 做幂等去重。
func (s *meteringService) RecordUsageEvent(ctx context.Context, event *models.UsageEvent) error {
	if event.TenantID <= 0 {
		return errorsx.InvalidParam("tenant_id is required")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	if err := sqls.DB().Create(event).Error; err != nil {
		return fmt.Errorf("metering: record usage event failed: %w", err)
	}

	// 触发配额检查
	bus := GetEventBus()
	if bus != nil {
		usageCtx := context.Background()
		evt := bus.NewEvent("metering.usage.recorded", fmt.Sprintf("%d", event.TenantID), "system", "0", map[string]interface{}{
			"tenant_id":  event.TenantID,
			"product_id": event.ProductID,
			"api_key_id": event.APIKeyID,
		}, "")
		_ = bus.Publish(usageCtx, evt)
	}

	return nil
}

// CheckQuotaWithLimit 使用 QuotaLimit 模型检查配额
//
// 从 QuotaLimit 表读取配额限制定义，对比当前周期内实际用量。
// 返回剩余配额和是否超限。
func (s *meteringService) CheckQuotaWithLimit(tenantID, productID int64, resourceType string) (remaining int64, exceeded bool, limitValue int64, err error) {
	if tenantID <= 0 {
		return 0, false, 0, errorsx.InvalidParam("tenant_id is required")
	}

	var limit models.QuotaLimit
	result := sqls.DB().
		Where("tenant_id = ? AND resource_type = ? AND status = ?", tenantID, resourceType, 0).
		Where("product_id IN ?", []int64{0, productID}).
		Order("CASE WHEN product_id = 0 THEN 1 ELSE 0 END ASC").
		Take(&limit)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return -1, false, 0, nil // 未配置限制，不限制
		}
		return 0, false, 0, fmt.Errorf("metering: query quota limit failed: %w", result.Error)
	}

	periodStart := getPeriodStart(limit.Period)
	var currentUsage int64

	usageQuery := sqls.DB().Model(&models.ProductAIUsageEvent{}).
		Where("tenant_id = ?", tenantID).
		Where("occurred_at >= ?", periodStart)

	if limit.ProductID > 0 && productID > 0 {
		usageQuery = usageQuery.Where("product_id = ?", productID)
	}

	switch resourceType {
	case models.QuotaResourceTokens:
		usageQuery.Select("COALESCE(SUM(input_tokens + output_tokens), 0)")
	case models.QuotaResourceRequests:
		usageQuery.Select("COUNT(*)")
	case models.QuotaResourceCost:
		usageQuery.Select("COALESCE(SUM(cost_amount), 0)")
	default:
		usageQuery.Select("COUNT(*)")
	}

	if err := usageQuery.Scan(&currentUsage).Error; err != nil {
		return 0, false, 0, fmt.Errorf("metering: query current usage failed: %w", err)
	}

	remaining = limit.LimitValue - currentUsage
	if remaining < 0 {
		remaining = 0
	}
	exceeded = currentUsage >= limit.LimitValue

	// 如果超过阈值，发布配额超限事件
	if exceeded || (limit.LimitValue > 0 && float64(currentUsage)/float64(limit.LimitValue) >= limit.NotifyThreshold) {
		eventID := fmt.Sprintf("tenant:%d:quota.threshold:%d:%s:%s", tenantID, limit.ID, resourceType, periodStart.UTC().Format("20060102T150405Z"))
		if err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			_, err := eventbus.EnqueueTx(ctx.Tx, eventbus.DurableEvent{
				TenantID:       tenantID,
				IdempotencyKey: eventID,
				EventType:      events.EventQuotaExceeded,
				Payload: events.QuotaExceededEvent{
					EventID:      eventID,
					TenantID:     tenantID,
					ProductID:    productID,
					ResourceType: resourceType,
					LimitValue:   limit.LimitValue,
					CurrentValue: currentUsage,
					APIKeyID:     "",
				},
				Source:      "metering_service",
				AggregateID: fmt.Sprintf("%d", limit.ID),
			})
			if err == nil && ctx.RegisterCallback != nil {
				ctx.RegisterCallback(eventbus.WakeDefaultOutboxPublisher)
			}
			return err
		}); err != nil {
			return 0, false, 0, fmt.Errorf("metering: enqueue quota event: %w", err)
		}
	}

	return remaining, exceeded, limit.LimitValue, nil
}

// getPeriodStart 计算当前周期的开始时间
func getPeriodStart(period string) time.Time {
	now := time.Now()
	switch period {
	case models.QuotaPeriodDaily:
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case models.QuotaPeriodWeekly:
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
	case models.QuotaPeriodMonthly:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	default:
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}
}

// CheckQuota 检查指定租户和 API Key 的配额
//
// 从 ProductAIUsageCredential 读取配额策略，对比当日实际用量。
// 返回剩余配额（token 数）和是否超限。
func (s *meteringService) CheckQuota(tenantID, productID int64, apiKeyID string) (remaining int64, exceeded bool, err error) {
	if tenantID <= 0 {
		return 0, false, errorsx.InvalidParam("tenant_id is required")
	}

	db := sqls.DB()

	// 查询 AI 用量凭证（直接 GORM 查询，避免创建新 repository）
	credential := &models.ProductAIUsageCredential{}
	result := db.Where("tenant_id = ? AND product_id = ? AND status = 0", tenantID, productID).
		Take(credential)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// 未配置配额策略，视为不限
			return -1, false, nil
		}
		return 0, false, fmt.Errorf("metering: query credential failed: %w", result.Error)
	}

	// 解析配额策略（此处使用简化实现：硬编码每日配额上限）
	quotaLimit := extractDailyQuotaLimit(credential.QuotaPolicyJSON)
	if quotaLimit <= 0 {
		return -1, false, nil
	}

	// 查询当日已用 token 数
	today := time.Now().Format("2006-01-02")
	var usedTokens int64
	dailyQuery := db.Model(&models.ProductAIUsageEvent{}).
		Select("COALESCE(SUM(input_tokens + output_tokens), 0)").
		Where("tenant_id = ?", tenantID).
		Where("product_id = ?", productID).
		Where("DATE(occurred_at) = ?", today)

	if apiKeyID != "" {
		dailyQuery = dailyQuery.Where("api_key_id = ?", apiKeyID)
	}

	if err := dailyQuery.Scan(&usedTokens).Error; err != nil {
		return 0, false, fmt.Errorf("metering: query daily usage failed: %w", err)
	}

	remaining = quotaLimit - usedTokens
	if remaining < 0 {
		remaining = 0
	}
	exceeded = usedTokens >= quotaLimit

	return remaining, exceeded, nil
}

// AggregateDailyUsage 每日用量聚合任务
//
// 从 ProductAIUsageEvent 明细表按 tenant + product + date 聚合，
// 写入 ProductAIUsageDaily 汇总表。
// 建议通过定时任务（cron）每天凌晨执行一次。
func (s *meteringService) AggregateDailyUsage() error {
	db := sqls.DB()

	// 聚合到昨天为止的数据
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	type aggRow struct {
		TenantID  int64
		ProductID int64
		InTokens  int64
		OutTokens int64
		Count     int64
		CostSum   float64
	}

	var rows []aggRow

	// 按 tenant + product + date 聚合
	err := db.Model(&models.ProductAIUsageEvent{}).
		Select("tenant_id, product_id, COALESCE(SUM(input_tokens), 0) AS in_tokens, COALESCE(SUM(output_tokens), 0) AS out_tokens, COUNT(*) AS count, COALESCE(SUM(cost_amount), 0) AS cost_sum").
		Where("DATE(occurred_at) = ?", yesterday).
		Group("tenant_id, product_id").
		Scan(&rows).Error

	if err != nil {
		return fmt.Errorf("metering: aggregate query failed: %w", err)
	}

	if len(rows) == 0 {
		logrus.Infof("metering: no usage events to aggregate for date %s", yesterday)
		return nil
	}

	// 逐条 upsert 到 ProductAIUsageDaily
	for _, row := range rows {
		costMicros := int64(row.CostSum * 1000000) // 元转微元

		err := db.Where("tenant_id = ? AND product_id = ? AND usage_date = ?", row.TenantID, row.ProductID, yesterday).
			Assign(&models.ProductAIUsageDaily{
				RequestCount:   row.Count,
				InputTokens:    row.InTokens,
				OutputTokens:   row.OutTokens,
				CostMicros:     costMicros,
				SourceProvider: "sub2api",
			}).
			FirstOrCreate(&models.ProductAIUsageDaily{
				TenantID:       row.TenantID,
				ProductID:      row.ProductID,
				UsageDate:      yesterday,
				SourceProvider: "sub2api",
			}).Error

		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"tenant_id":  row.TenantID,
				"product_id": row.ProductID,
				"date":       yesterday,
			}).Error("metering: aggregate daily usage upsert failed")
		}
	}

	logrus.Infof("metering: aggregated %d tenant-product rows for date %s", len(rows), yesterday)
	return nil
}

// ---------- 包级辅助函数 ----------

// quotaPolicy 配额策略 JSON 结构
type quotaPolicy struct {
	DailyTokenLimit   int64 `json:"daily_token_limit"`
	MonthlyTokenLimit int64 `json:"monthly_token_limit"`
}

// extractDailyQuotaLimit 从 QuotaPolicyJSON 中提取每日 token 上限。
//
// 支持格式：{"daily_token_limit": 1000000, "monthly_token_limit": 30000000}
// 空字符串或 "{}" 返回 0 表示不限。
func extractDailyQuotaLimit(policyJSON string) int64 {
	if policyJSON == "" || policyJSON == "{}" {
		return 0 // 0 表示不限
	}

	var policy quotaPolicy
	if err := json.Unmarshal([]byte(policyJSON), &policy); err != nil {
		logrus.WithError(err).WithField("policy_json", policyJSON).Warn("metering: failed to parse quota policy JSON, falling back to unlimited")
		return 0
	}

	if policy.DailyTokenLimit <= 0 {
		return 0 // 不限
	}

	return policy.DailyTokenLimit
}

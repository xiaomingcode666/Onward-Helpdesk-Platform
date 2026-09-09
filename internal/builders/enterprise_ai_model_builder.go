package builders

import (
	"math"
	"strings"
	"time"

	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/services"
)

func BuildEnterpriseAIModelWorkspace(agg *services.EnterpriseAIModelWorkspaceAggregate) *response.EnterpriseAIModelWorkspaceResponse {
	if agg == nil {
		return nil
	}
	ret := &response.EnterpriseAIModelWorkspaceResponse{
		GeneratedAt: formatTime(agg.GeneratedAt),
	}
	if agg.Account != nil {
		ret.Account = response.EnterpriseAIModelAccountResponse{
			TenantID:             agg.Account.TenantID,
			Sub2APIAccountID:     agg.Account.Sub2APIAccountID,
			AccountName:          agg.Account.AccountName,
			LoginEmail:           agg.Account.LoginEmail,
			AccountStatus:        agg.Account.AccountStatus,
			ProvisionStatus:      agg.Account.ProvisionStatus,
			Balance:              agg.Account.Balance,
			Concurrency:          agg.Account.Concurrency,
			RPMLimit:             agg.Account.RPMLimit,
			DefaultKeyID:         agg.Account.DefaultKeyID,
			DefaultKeyName:       agg.Account.DefaultKeyName,
			DefaultKeyQuota:      agg.Account.DefaultKeyQuota,
			DefaultKeyQuotaUsed:  agg.Account.DefaultKeyQuotaUsed,
			DefaultKeyStatus:     agg.Account.DefaultKeyStatus,
			DefaultLLMModel:      agg.Account.DefaultLLMModel,
			AccessTokenExpiresAt: formatTimePtr(agg.Account.AccessTokenExpiresAt),
			LastSyncedAt:         formatTimePtr(agg.Account.LastSyncedAt),
		}
	}
	if agg.User != nil {
		ret.User = response.EnterpriseAISub2APIUserResponse{
			ID:                 agg.User.ID,
			Email:              agg.User.Email,
			Username:           agg.User.Username,
			Role:               agg.User.Role,
			Balance:            agg.User.Balance,
			FrozenBalance:      agg.User.FrozenBalance,
			Concurrency:        agg.User.Concurrency,
			CurrentConcurrency: agg.User.CurrentConcurrency,
			Status:             agg.User.Status,
			TotalRecharged:     agg.User.TotalRecharged,
			RPMLimit:           agg.User.RPMLimit,
			LastActiveAt:       agg.User.LastActiveAt,
			UpdatedAt:          agg.User.UpdatedAt,
		}
	}
	if agg.Stats != nil {
		stats := agg.Stats.Data
		ret.Stats = response.EnterpriseAIUsageStatsResponse{
			TotalAPIKeys:             stats.TotalAPIKeys,
			ActiveAPIKeys:            stats.ActiveAPIKeys,
			TotalRequests:            stats.TotalRequests,
			TotalInputTokens:         stats.TotalInputTokens,
			TotalOutputTokens:        stats.TotalOutputTokens,
			TotalCacheCreationTokens: stats.TotalCacheCreationTokens,
			TotalCacheReadTokens:     stats.TotalCacheReadTokens,
			TotalTokens:              stats.TotalTokens,
			TotalCost:                stats.TotalCost,
			TotalActualCost:          stats.TotalActualCost,
			TodayRequests:            stats.TodayRequests,
			TodayInputTokens:         stats.TodayInputTokens,
			TodayOutputTokens:        stats.TodayOutputTokens,
			TodayCacheCreationTokens: stats.TodayCacheCreationTokens,
			TodayCacheReadTokens:     stats.TodayCacheReadTokens,
			TodayTokens:              stats.TodayTokens,
			TodayCost:                stats.TodayCost,
			TodayActualCost:          stats.TodayActualCost,
			AverageDurationMS:        stats.AverageDurationMS,
			RPM:                      stats.RPM,
			TPM:                      stats.TPM,
			ByPlatform:               make([]response.EnterpriseAIUsagePlatformStatsResponse, 0, len(stats.ByPlatform)),
		}
		for i := range stats.ByPlatform {
			item := stats.ByPlatform[i]
			ret.Stats.ByPlatform = append(ret.Stats.ByPlatform, response.EnterpriseAIUsagePlatformStatsResponse{
				Platform:        item.Platform,
				TotalRequests:   item.TotalRequests,
				TotalTokens:     item.TotalTokens,
				TotalActualCost: item.TotalActualCost,
				TodayRequests:   item.TodayRequests,
				TodayTokens:     item.TodayTokens,
				TodayActualCost: item.TodayActualCost,
			})
		}
	}
	if agg.Trend != nil {
		ret.Trend = make([]response.EnterpriseAIUsageTrendPointResponse, 0, len(agg.Trend.Data.Trend))
		for i := range agg.Trend.Data.Trend {
			item := agg.Trend.Data.Trend[i]
			ret.Trend = append(ret.Trend, response.EnterpriseAIUsageTrendPointResponse{
				Date:                item.Date,
				Requests:            item.Requests,
				InputTokens:         item.InputTokens,
				OutputTokens:        item.OutputTokens,
				CacheCreationTokens: item.CacheCreationTokens,
				CacheReadTokens:     item.CacheReadTokens,
				TotalTokens:         item.TotalTokens,
				Cost:                item.Cost,
				ActualCost:          item.ActualCost,
			})
		}
	}
	if agg.Keys != nil {
		ret.KeyPaging = response.EnterpriseAIKeyPagingResponse{
			Total:    agg.Keys.Data.Total,
			Page:     agg.Keys.Data.Page,
			PageSize: agg.Keys.Data.PageSize,
			Pages:    agg.Keys.Data.Pages,
		}
		ret.Keys = make([]response.EnterpriseAIKeyResponse, 0, len(agg.Keys.Data.Items))
		for i := range agg.Keys.Data.Items {
			var productBinding *services.EnterpriseAIKeyProductBinding
			if agg.KeyProductBindings != nil {
				if binding, ok := agg.KeyProductBindings[agg.Keys.Data.Items[i].ID]; ok {
					productBinding = &binding
				}
			}
			ret.Keys = append(ret.Keys, buildEnterpriseAIKeyResponse(&agg.Keys.Data.Items[i], productBinding))
		}
	}
	if agg.Subscriptions != nil {
		ret.Subscriptions = make([]response.EnterpriseAISubscriptionResponse, 0, len(agg.Subscriptions.Data))
		for i := range agg.Subscriptions.Data {
			built := buildEnterpriseAISubscriptionResponse(&agg.Subscriptions.Data[i], agg.GeneratedAt)
			ret.Subscriptions = append(ret.Subscriptions, built)
			if ret.ActiveSubscription == nil && strings.EqualFold(strings.TrimSpace(built.Status), "active") {
				ret.ActiveSubscription = &ret.Subscriptions[len(ret.Subscriptions)-1]
			}
		}
		if ret.ActiveSubscription == nil && len(ret.Subscriptions) > 0 {
			ret.ActiveSubscription = &ret.Subscriptions[0]
		}
	}
	return ret
}

func buildEnterpriseAIKeyResponse(item *providers.Sub2APIKey, productBinding *services.EnterpriseAIKeyProductBinding) response.EnterpriseAIKeyResponse {
	if item == nil {
		return response.EnterpriseAIKeyResponse{}
	}
	expiresAt := ""
	if item.ExpiresAt != nil {
		expiresAt = *item.ExpiresAt
	}
	lastUsedAt := ""
	if item.LastUsedAt != nil {
		lastUsedAt = *item.LastUsedAt
	}
	quotaRemaining := math.Max(item.Quota-item.QuotaUsed, 0)
	quotaUsagePercent := 0.0
	if item.Quota > 0 {
		quotaUsagePercent = item.QuotaUsed / item.Quota * 100
	}
	ret := response.EnterpriseAIKeyResponse{
		ID:                 item.ID,
		UserID:             item.UserID,
		KeyPreview:         maskSub2APIKey(item.Key),
		Name:               item.Name,
		GroupID:            item.GroupID,
		Status:             item.Status,
		Quota:              item.Quota,
		QuotaUsed:          item.QuotaUsed,
		QuotaRemaining:     quotaRemaining,
		QuotaUsagePercent:  quotaUsagePercent,
		ExpiresAt:          expiresAt,
		LastUsedAt:         lastUsedAt,
		LastUsedIP:         item.LastUsedIP,
		CurrentConcurrency: item.CurrentConcurrency,
		RateLimit5h:        item.RateLimit5h,
		RateLimit1d:        item.RateLimit1d,
		RateLimit7d:        item.RateLimit7d,
		Usage5h:            item.Usage5h,
		Usage1d:            item.Usage1d,
		Usage7d:            item.Usage7d,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
	if productBinding != nil && productBinding.ProductID > 0 {
		ret.ProductID = productBinding.ProductID
		ret.ProductName = productBinding.ProductName
		ret.ProductCode = productBinding.ProductCode
	}
	if item.Group != nil {
		ret.Group = &response.EnterpriseAIKeyGroupResponse{
			ID:             item.Group.ID,
			Name:           item.Group.Name,
			Platform:       item.Group.Platform,
			RateMultiplier: item.Group.RateMultiplier,
			Status:         item.Group.Status,
			RPMLimit:       item.Group.RPMLimit,
		}
	}
	return ret
}

func maskSub2APIKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= 10 {
		return "••••"
	}
	prefixLen := 7
	suffixLen := 6
	if len(runes) < prefixLen+suffixLen {
		prefixLen = 4
		suffixLen = 4
	}
	return string(runes[:prefixLen]) + "…" + string(runes[len(runes)-suffixLen:])
}

func buildEnterpriseAISubscriptionResponse(item *providers.Sub2APISubscription, now time.Time) response.EnterpriseAISubscriptionResponse {
	if item == nil {
		return response.EnterpriseAISubscriptionResponse{}
	}
	dailyWindowStart := ""
	if item.DailyWindowStart != nil {
		dailyWindowStart = *item.DailyWindowStart
	}
	weeklyWindowStart := ""
	if item.WeeklyWindowStart != nil {
		weeklyWindowStart = *item.WeeklyWindowStart
	}
	monthlyWindowStart := ""
	if item.MonthlyWindowStart != nil {
		monthlyWindowStart = *item.MonthlyWindowStart
	}
	durationDays, remainingDays, progressPercent := subscriptionDurationMetrics(item.StartsAt, item.ExpiresAt, now)
	ret := response.EnterpriseAISubscriptionResponse{
		ID:                 item.ID,
		UserID:             item.UserID,
		GroupID:            item.GroupID,
		StartsAt:           item.StartsAt,
		ExpiresAt:          item.ExpiresAt,
		Status:             item.Status,
		DurationDays:       durationDays,
		RemainingDays:      remainingDays,
		ProgressPercent:    progressPercent,
		DailyWindowStart:   dailyWindowStart,
		WeeklyWindowStart:  weeklyWindowStart,
		MonthlyWindowStart: monthlyWindowStart,
		DailyUsageUSD:      item.DailyUsageUSD,
		WeeklyUsageUSD:     item.WeeklyUsageUSD,
		MonthlyUsageUSD:    item.MonthlyUsageUSD,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
	if item.Group != nil {
		ret.Group = &response.EnterpriseAISubscriptionGroupResponse{
			ID:               item.Group.ID,
			Name:             item.Group.Name,
			Description:      item.Group.Description,
			Platform:         item.Group.Platform,
			RateMultiplier:   item.Group.RateMultiplier,
			IsExclusive:      item.Group.IsExclusive,
			Status:           item.Group.Status,
			SubscriptionType: item.Group.SubscriptionType,
			DailyLimitUSD:    item.Group.DailyLimitUSD,
			WeeklyLimitUSD:   item.Group.WeeklyLimitUSD,
			MonthlyLimitUSD:  item.Group.MonthlyLimitUSD,
			RPMLimit:         item.Group.RPMLimit,
		}
	}
	return ret
}

func subscriptionDurationMetrics(startsAt, expiresAt string, now time.Time) (int64, int64, float64) {
	start, startOK := parseSub2APITime(startsAt)
	expires, expiresOK := parseSub2APITime(expiresAt)
	if !startOK || !expiresOK || !expires.After(start) {
		return 0, 0, 0
	}
	duration := expires.Sub(start)
	durationDays := int64(math.Ceil(duration.Hours() / 24))
	remainingDays := int64(0)
	if expires.After(now) {
		remainingDays = int64(math.Ceil(expires.Sub(now).Hours() / 24))
	}
	elapsed := now.Sub(start)
	progressPercent := float64(elapsed) / float64(duration) * 100
	if progressPercent < 0 {
		progressPercent = 0
	}
	if progressPercent > 100 {
		progressPercent = 100
	}
	return durationDays, remainingDays, progressPercent
}

func parseSub2APITime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

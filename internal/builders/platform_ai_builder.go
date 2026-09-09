package builders

import (
	"encoding/json"

	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/services"
)

func BuildPlatformAIModelWorkspace(agg *services.PlatformAIModelWorkspaceAggregate) *response.PlatformAIModelWorkspaceResponse {
	if agg == nil {
		return nil
	}
	ret := &response.PlatformAIModelWorkspaceResponse{
		GeneratedAt: formatTime(agg.GeneratedAt),
		Provider: response.PlatformAIProviderResponse{
			Host:                   agg.Provider.Host,
			DefaultLLMModel:        agg.Provider.DefaultLLMModel,
			AdminAPIKeyConfigured:  agg.Provider.AdminAPIKeyConfigured,
			AdminAPIKeyFingerprint: agg.Provider.AdminAPIKeyFingerprint,
			UpdatedAt:              formatTimePtr(agg.Provider.UpdatedAt),
		},
		Summary: response.PlatformAIModelSummaryResponse{
			TotalTenants:      agg.Summary.TotalTenants,
			ConfiguredTenants: agg.Summary.ConfiguredTenants,
			ActiveTenants:     agg.Summary.ActiveTenants,
			AttentionTenants:  agg.Summary.AttentionTenants,
			ActiveKeys:        agg.Summary.ActiveKeys,
			ModelCount:        agg.Summary.ModelCount,
		},
	}
	ret.Models = make([]response.PlatformAIConfigResponse, 0, len(agg.Models))
	for i := range agg.Models {
		model := agg.Models[i]
		ret.Models = append(ret.Models, response.PlatformAIConfigResponse{
			ID:               model.ID,
			Name:             model.Name,
			Provider:         string(model.Provider),
			BaseURL:          model.BaseURL,
			ModelType:        string(model.ModelType),
			ModelName:        model.ModelName,
			Dimension:        model.Dimension,
			MaxContextTokens: model.MaxContextTokens,
			MaxOutputTokens:  model.MaxOutputTokens,
			TimeoutMS:        model.TimeoutMS,
			MaxRetryCount:    model.MaxRetryCount,
			RPMLimit:         model.RPMLimit,
			TPMLimit:         model.TPMLimit,
			Status:           int(model.Status),
			SortNo:           model.SortNo,
			Remark:           model.Remark,
			CreatedAt:        formatTime(model.CreatedAt),
			UpdatedAt:        formatTime(model.UpdatedAt),
		})
	}
	ret.Tenants = make([]response.PlatformAITenantWorkspaceResponse, 0, len(agg.Tenants))
	for i := range agg.Tenants {
		ret.Tenants = append(ret.Tenants, buildPlatformAITenantWorkspaceResponse(&agg.Tenants[i]))
	}
	return ret
}

func BuildPlatformAIUsage(agg *services.PlatformAIUsageAggregate) *response.PlatformAIUsageDetailResponse {
	if agg == nil {
		return nil
	}
	ret := &response.PlatformAIUsageDetailResponse{
		GeneratedAt: formatTime(agg.GeneratedAt),
		Tenant:      buildPlatformAITenantWorkspaceResponse(&agg.Tenant),
		Stats: response.PlatformAIUsageStatsResponse{
			TotalRequests:            agg.Stats.Data.TotalRequests,
			TotalInputTokens:         agg.Stats.Data.TotalInputTokens,
			TotalOutputTokens:        agg.Stats.Data.TotalOutputTokens,
			TotalCacheTokens:         agg.Stats.Data.TotalCacheTokens,
			TotalCacheCreationTokens: agg.Stats.Data.TotalCacheCreationTokens,
			TotalCacheReadTokens:     agg.Stats.Data.TotalCacheReadTokens,
			TotalTokens:              agg.Stats.Data.TotalTokens,
			TotalCost:                agg.Stats.Data.TotalCost,
			TotalActualCost:          agg.Stats.Data.TotalActualCost,
			AverageDurationMS:        agg.Stats.Data.AverageDurationMS,
			Currency:                 "USD",
			Endpoints:                make([]response.PlatformAIUsageEndpointResponse, 0, len(agg.Stats.Data.Endpoints)),
		},
		Models: make([]response.PlatformAIUsageDashboardModelResponse, 0, len(agg.Models.Data.Models)),
		Usage:  json.RawMessage(agg.RawUsage),
	}
	for i := range agg.Stats.Data.Endpoints {
		endpoint := agg.Stats.Data.Endpoints[i]
		ret.Stats.Endpoints = append(ret.Stats.Endpoints, response.PlatformAIUsageEndpointResponse{
			Endpoint:    endpoint.Endpoint,
			Requests:    endpoint.Requests,
			TotalTokens: endpoint.TotalTokens,
			Cost:        endpoint.Cost,
			ActualCost:  endpoint.ActualCost,
		})
	}
	for i := range agg.Models.Data.Models {
		model := agg.Models.Data.Models[i]
		ret.Models = append(ret.Models, response.PlatformAIUsageDashboardModelResponse{
			Model:               model.Model,
			Requests:            model.Requests,
			InputTokens:         model.InputTokens,
			OutputTokens:        model.OutputTokens,
			CacheCreationTokens: model.CacheCreationTokens,
			CacheReadTokens:     model.CacheReadTokens,
			TotalTokens:         model.TotalTokens,
			Cost:                model.Cost,
			ActualCost:          model.ActualCost,
		})
	}
	return ret
}

func BuildPlatformAITenantBilling(agg *services.PlatformAITenantBillingAggregate) *response.PlatformAITenantBillingResponse {
	if agg == nil {
		return nil
	}
	ret := &response.PlatformAITenantBillingResponse{
		GeneratedAt: formatTime(agg.GeneratedAt),
		Tenant:      buildPlatformAITenantWorkspaceResponse(&agg.Tenant),
	}
	if agg.RemoteUser != nil {
		lastUsedAt := ""
		if agg.RemoteUser.LastUsedAt != nil {
			lastUsedAt = *agg.RemoteUser.LastUsedAt
		}
		ret.RemoteUser = &response.PlatformAISub2APIUserResponse{
			ID:                 agg.RemoteUser.ID,
			Email:              agg.RemoteUser.Email,
			Username:           agg.RemoteUser.Username,
			Role:               agg.RemoteUser.Role,
			Balance:            agg.RemoteUser.Balance,
			FrozenBalance:      agg.RemoteUser.FrozenBalance,
			Concurrency:        agg.RemoteUser.Concurrency,
			CurrentConcurrency: agg.RemoteUser.CurrentConcurrency,
			Status:             agg.RemoteUser.Status,
			TotalRecharged:     agg.RemoteUser.TotalRecharged,
			RPMLimit:           agg.RemoteUser.RPMLimit,
			LastActiveAt:       agg.RemoteUser.LastActiveAt,
			LastUsedAt:         lastUsedAt,
			UpdatedAt:          agg.RemoteUser.UpdatedAt,
		}
	}
	ret.RechargeRecords = make([]response.PlatformAIRechargeRecordResponse, 0, len(agg.Records))
	for i := range agg.Records {
		item := agg.Records[i]
		ret.RechargeRecords = append(ret.RechargeRecords, response.PlatformAIRechargeRecordResponse{
			ID:               item.ID,
			TenantID:         item.TenantID,
			Sub2APIAccountID: item.Sub2APIAccountID,
			RemoteUserID:     item.RemoteUserID,
			Operation:        item.Operation,
			Amount:           item.Amount,
			BalanceBefore:    item.BalanceBefore,
			BalanceAfter:     item.BalanceAfter,
			Notes:            item.Notes,
			Status:           item.Status,
			Message:          item.Message,
			OccurredAt:       formatTime(item.OccurredAt),
			CreatedAt:        formatTime(item.CreatedAt),
			OperatorID:       item.CreateUserID,
			OperatorName:     item.CreateUserName,
		})
	}
	return ret
}

func BuildPlatformSub2APIUserList(agg *services.PlatformSub2APIUserListAggregate) *response.PlatformSub2APIUserListResponse {
	if agg == nil {
		return nil
	}
	ret := &response.PlatformSub2APIUserListResponse{
		GeneratedAt: formatTime(agg.GeneratedAt),
		Items:       make([]response.PlatformSub2APIUserListItemResponse, 0, len(agg.Users)),
		Total:       agg.Total,
		Page:        agg.Page,
		PageSize:    agg.PageSize,
		Pages:       agg.Pages,
	}
	for i := range agg.Users {
		item := agg.Users[i]
		lastUsedAt := ""
		if item.LastUsedAt != nil {
			lastUsedAt = *item.LastUsedAt
		}
		ret.Items = append(ret.Items, response.PlatformSub2APIUserListItemResponse{
			ID:                 item.ID,
			Email:              item.Email,
			Username:           item.Username,
			Role:               item.Role,
			Balance:            item.Balance,
			FrozenBalance:      item.FrozenBalance,
			Concurrency:        item.Concurrency,
			CurrentConcurrency: item.CurrentConcurrency,
			Status:             item.Status,
			LastActiveAt:       item.LastActiveAt,
			LastUsedAt:         lastUsedAt,
			CreatedAt:          item.CreatedAt,
		})
	}
	return ret
}

func BuildPlatformSub2APIUserKeyList(agg *services.PlatformSub2APIUserKeyListAggregate) *response.PlatformSub2APIUserKeyListResponse {
	if agg == nil {
		return nil
	}
	ret := &response.PlatformSub2APIUserKeyListResponse{
		GeneratedAt: formatTime(agg.GeneratedAt),
		UserID:      agg.UserID,
		Items:       make([]response.PlatformSub2APIUserKeyResponse, 0, len(agg.Keys)),
		Total:       agg.Total,
		Page:        agg.Page,
		PageSize:    agg.PageSize,
		Pages:       agg.Pages,
	}
	for i := range agg.Keys {
		item := agg.Keys[i]
		groupName := ""
		if item.Group != nil {
			groupName = item.Group.Name
		}
		lastUsedAt := ""
		if item.LastUsedAt != nil {
			lastUsedAt = *item.LastUsedAt
		}
		expiresAt := ""
		if item.ExpiresAt != nil {
			expiresAt = *item.ExpiresAt
		}
		ret.Items = append(ret.Items, response.PlatformSub2APIUserKeyResponse{
			ID:                 item.ID,
			UserID:             item.UserID,
			KeyPreview:         maskSub2APIKey(item.Key),
			Name:               item.Name,
			GroupID:            item.GroupID,
			GroupName:          groupName,
			Status:             item.Status,
			LastUsedAt:         lastUsedAt,
			LastUsedIP:         item.LastUsedIP,
			Quota:              item.Quota,
			QuotaUsed:          item.QuotaUsed,
			ExpiresAt:          expiresAt,
			CreatedAt:          item.CreatedAt,
			CurrentConcurrency: item.CurrentConcurrency,
		})
	}
	return ret
}

func buildPlatformAITenantWorkspaceResponse(item *services.PlatformAITenantWorkspaceItem) response.PlatformAITenantWorkspaceResponse {
	if item == nil {
		return response.PlatformAITenantWorkspaceResponse{}
	}
	ret := response.PlatformAITenantWorkspaceResponse{
		Tenant:         *BuildPlatformTenant(&item.Tenant, nil, nil, 0, 0),
		HasAccessToken: item.HasAccessToken,
		NeedsAttention: item.NeedsAttention,
	}
	if item.Account != nil {
		ret.Account = &response.PlatformAITenantAccountResponse{
			ID:                   item.Account.ID,
			TenantID:             item.Account.TenantID,
			Sub2APIAccountID:     item.Account.Sub2APIAccountID,
			AccountName:          item.Account.AccountName,
			DashboardURL:         item.Account.DashboardURL,
			LoginEmail:           item.Account.LoginEmail,
			AccountStatus:        item.Account.AccountStatus,
			ProvisionStatus:      item.Account.ProvisionStatus,
			Concurrency:          item.Account.Concurrency,
			Balance:              item.Account.Balance,
			RPMLimit:             item.Account.RPMLimit,
			DefaultKeyID:         item.Account.DefaultKeyID,
			DefaultKeyName:       item.Account.DefaultKeyName,
			DefaultKeyGroupID:    item.Account.DefaultKeyGroupID,
			DefaultKeyQuota:      item.Account.DefaultKeyQuota,
			DefaultKeyQuotaUsed:  item.Account.DefaultKeyQuotaUsed,
			DefaultKeyStatus:     item.Account.DefaultKeyStatus,
			DefaultKeyExpiresAt:  formatTimePtr(item.Account.DefaultKeyExpiresAt),
			DefaultKey:           item.DefaultKey,
			DefaultLLMModel:      item.Account.DefaultLLMModel,
			AccessTokenExpiresAt: formatTimePtr(item.Account.AccessTokenExpiresAt),
			LastSyncedAt:         formatTimePtr(item.Account.LastSyncedAt),
			LastProvisionedAt:    formatTimePtr(item.Account.LastProvisionedAt),
			ProvisionMessage:     item.Account.ProvisionMessage,
			Status:               int(item.Account.Status),
		}
	}
	return ret
}

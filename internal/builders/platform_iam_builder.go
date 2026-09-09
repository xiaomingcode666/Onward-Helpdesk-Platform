package builders

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
)

func BuildPlatformTenant(item *models.Tenant, subscription *models.TenantSubscription, plan *models.TenantPlan, deviceCount, memberCount int64) *response.PlatformTenantResponse {
	if item == nil {
		return nil
	}
	ret := &response.PlatformTenantResponse{
		ID:                    item.ID,
		Code:                  fmt.Sprintf("T-%06d", item.ID),
		Name:                  item.Name,
		ServiceScene:          item.EffectiveServiceScene(),
		Industry:              item.Industry,
		CountryRegion:         item.CountryRegion,
		DefaultLocale:         item.DefaultLocale,
		SupportedLocales:      utils.ParseStringListJSON(item.SupportedLocalesJSON),
		Timezone:              item.Timezone,
		SupportedTimezones:    utils.ParseStringListJSON(item.SupportedTimezonesJSON),
		DataRegion:            item.DataRegion,
		Status:                int(item.Status),
		AIEnabled:             item.IsAIEnabled(),
		TrialEndsAt:           formatTimePtr(item.TrialEndsAt),
		FrozenReason:          item.FrozenReason,
		DecommissionedAt:      formatTimePtr(item.DecommissionedAt),
		DecommissionReason:    item.DecommissionReason,
		PurgeAfterDays:        item.PurgeAfterDays,
		PurgedAt:              formatTimePtr(item.PurgedAt),
		DataExportCompletedAt: formatTimePtr(item.DataExportCompletedAt),
		DeviceCount:           deviceCount,
		MemberCount:           memberCount,
		CreatedAt:             formatTime(item.CreatedAt),
		UpdatedAt:             formatTime(item.UpdatedAt),
	}
	if subscription != nil {
		ret.SubscriptionStatus = int(subscription.Status)
	}
	if plan != nil {
		ret.PlanCode = plan.Code
		ret.PlanName = plan.Name
	}
	return ret
}

func ApplyPlatformTenantPortalSettings(ret *response.PlatformTenantResponse, branding *models.TenantBranding, serverConsole *models.TenantIntegrationConfig) {
	if ret == nil {
		return
	}
	ret.CustomerDefaultLocale = ret.DefaultLocale
	if branding != nil && branding.Status != enums.StatusDeleted {
		ret.BrandName = branding.BrandName
		ret.LogoAssetID = branding.LogoAssetID
		ret.LogoURL = branding.LogoURL
		ret.CustomDomain = branding.CustomDomain
		ret.CustomerTheme = branding.EffectiveCustomerTheme()
		if defaultLocale := strings.TrimSpace(branding.DefaultLocale); defaultLocale != "" {
			ret.CustomerDefaultLocale = defaultLocale
		}
	}
	if ret.CustomerTheme == "" {
		ret.CustomerTheme = models.TenantCustomerThemeDefault
	}
	if serverConsole == nil || serverConsole.Status == enums.StatusDeleted {
		return
	}
	metadata := models.TenantExternalPortalMetadata{DisplayName: "1Panel", EmbedMode: "external"}
	_ = json.Unmarshal([]byte(serverConsole.MetadataJSON), &metadata)
	if strings.TrimSpace(metadata.DisplayName) == "" {
		metadata.DisplayName = "1Panel"
	}
	ret.ServerConsoleName = metadata.DisplayName
	ret.ServerConsoleURL = serverConsole.BaseURL
	ret.ServerConsoleMode = metadata.EmbedMode
	ret.ServerConsoleEnabled = serverConsole.Enabled
}

func ApplyPlatformTenantAdministrators(items []*response.PlatformTenantResponse, administrators map[int64]*models.TenantMember, users map[int64]*models.User) {
	for _, item := range items {
		if item == nil {
			continue
		}
		administrator := administrators[item.ID]
		if administrator == nil {
			continue
		}
		item.AdminUserID = administrator.UserID
		item.AdminDisplayName = strings.TrimSpace(administrator.DisplayName)
		user := users[administrator.UserID]
		if user == nil {
			continue
		}
		item.AdminUsername = user.Username
		if item.AdminDisplayName == "" {
			item.AdminDisplayName = strings.TrimSpace(user.Nickname)
		}
		if user.Email != nil {
			item.AdminEmail = strings.TrimSpace(*user.Email)
		}
	}
}

func BuildPlatformTenantList(items []models.Tenant, subscriptions map[int64]*models.TenantSubscription, plans map[int64]*models.TenantPlan, deviceCounts, memberCounts map[int64]int64, monthlyUsage map[int64]repositories.PlatformAIUsageRow) []*response.PlatformTenantResponse {
	ret := make([]*response.PlatformTenantResponse, 0, len(items))
	for i := range items {
		subscription := subscriptions[items[i].ID]
		var plan *models.TenantPlan
		if subscription != nil {
			plan = plans[subscription.PlanID]
		}
		built := BuildPlatformTenant(&items[i], subscription, plan, deviceCounts[items[i].ID], memberCounts[items[i].ID])
		usage := monthlyUsage[items[i].ID]
		built.MonthlyAIRequests = usage.Requests
		built.MonthlyAITokens = usage.Tokens
		built.MonthlyAICost = usage.CostAmount
		built.LastMeteredAt = formatTimePtr(usage.LastMeteredAt)
		ret = append(ret, built)
	}
	return ret
}

func BuildPlatformPlan(item *models.TenantPlan) *response.PlatformPlanResponse {
	if item == nil {
		return nil
	}
	return &response.PlatformPlanResponse{
		ID:                item.ID,
		Code:              item.Code,
		Name:              item.Name,
		PlanType:          item.PlanType,
		FeatureJSON:       item.FeatureJSON,
		QuotaTemplateJSON: item.QuotaTemplateJSON,
		OverageStrategy:   item.OverageStrategy,
		Status:            int(item.Status),
		SortNo:            item.SortNo,
		CreatedAt:         formatTime(item.CreatedAt),
		UpdatedAt:         formatTime(item.UpdatedAt),
	}
}

func BuildPlatformPlanList(items []models.TenantPlan) []*response.PlatformPlanResponse {
	ret := make([]*response.PlatformPlanResponse, 0, len(items))
	for i := range items {
		ret = append(ret, BuildPlatformPlan(&items[i]))
	}
	return ret
}

func BuildPlatformSubscription(item *models.TenantSubscription) *response.PlatformSubscriptionResponse {
	if item == nil {
		return nil
	}
	return &response.PlatformSubscriptionResponse{
		ID:               item.ID,
		TenantID:         item.TenantID,
		PlanID:           item.PlanID,
		PlanSnapshotJSON: item.PlanSnapshotJSON,
		StartsAt:         formatTime(item.StartsAt),
		EndsAt:           formatTimePtr(item.EndsAt),
		BillingCycle:     item.BillingCycle,
		Status:           int(item.Status),
	}
}

func BuildPlatformQuotaOverride(item *models.TenantQuotaOverride) *response.PlatformQuotaOverrideResponse {
	if item == nil {
		return nil
	}
	return &response.PlatformQuotaOverrideResponse{
		ID:              item.ID,
		TenantID:        item.TenantID,
		ProductID:       item.ProductID,
		QuotaKey:        item.QuotaKey,
		QuotaValue:      item.QuotaValue,
		Period:          item.Period,
		OverageStrategy: item.OverageStrategy,
		Reason:          item.Reason,
		EffectiveFrom:   formatTimePtr(item.EffectiveFrom),
		EffectiveTo:     formatTimePtr(item.EffectiveTo),
		Status:          int(item.Status),
	}
}

func BuildPlatformSub2APIAccount(item *models.Sub2APITenantAccount) *response.PlatformSub2APIAccountResponse {
	if item == nil {
		return nil
	}
	return &response.PlatformSub2APIAccountResponse{
		ID:                item.ID,
		TenantID:          item.TenantID,
		Sub2APIAccountID:  item.Sub2APIAccountID,
		AccountName:       item.AccountName,
		DashboardURL:      item.DashboardURL,
		AccountStatus:     item.AccountStatus,
		QuotaSnapshotJSON: item.QuotaSnapshotJSON,
		LastSyncedAt:      formatTimePtr(item.LastSyncedAt),
		LastTestedAt:      formatTimePtr(item.LastTestedAt),
		Status:            int(item.Status),
	}
}

func BuildPlatformSub2APIAccountList(items []models.Sub2APITenantAccount) []*response.PlatformSub2APIAccountResponse {
	ret := make([]*response.PlatformSub2APIAccountResponse, 0, len(items))
	for i := range items {
		ret = append(ret, BuildPlatformSub2APIAccount(&items[i]))
	}
	return ret
}

func BuildPlatformStaff(item *models.PlatformStaffProfile, user *models.User) *response.PlatformStaffResponse {
	if item == nil {
		return nil
	}
	ret := &response.PlatformStaffResponse{
		ID:             item.ID,
		UserID:         item.UserID,
		TeamCode:       item.TeamCode,
		JobTitle:       item.JobTitle,
		SupportLevel:   item.SupportLevel,
		EmploymentType: item.EmploymentType,
		Status:         int(item.Status),
		CreatedAt:      formatTime(item.CreatedAt),
		UpdatedAt:      formatTime(item.UpdatedAt),
	}
	if user != nil {
		ret.Username = user.Username
		ret.DisplayName = user.Nickname
		ret.Email = stringValue(user.Email)
		ret.Mobile = stringValue(user.Mobile)
	}
	return ret
}

func BuildPlatformStaffList(items []models.PlatformStaffProfile, users map[int64]*models.User) []*response.PlatformStaffResponse {
	ret := make([]*response.PlatformStaffResponse, 0, len(items))
	for i := range items {
		ret = append(ret, BuildPlatformStaff(&items[i], users[items[i].UserID]))
	}
	return ret
}

func BuildPlatformTenantGrant(item *models.PlatformTenantGrant) *response.PlatformTenantGrantResponse {
	if item == nil {
		return nil
	}
	return &response.PlatformTenantGrantResponse{
		ID:              item.ID,
		PlatformStaffID: item.PlatformStaffID,
		TenantID:        item.TenantID,
		GrantScopeJSON:  item.GrantScopeJSON,
		Reason:          item.Reason,
		ApprovedBy:      item.ApprovedBy,
		ApprovedAt:      formatTimePtr(item.ApprovedAt),
		ExpiredAt:       formatTime(item.ExpiredAt),
		RevokedAt:       formatTimePtr(item.RevokedAt),
		Status:          item.Status,
	}
}

func BuildPlatformAuthRole(item *models.AuthRole, permissions []string) *response.PlatformAuthRoleResponse {
	if item == nil {
		return nil
	}
	return &response.PlatformAuthRoleResponse{
		ID:          item.ID,
		TenantID:    item.TenantID,
		DomainType:  item.DomainType,
		Code:        item.Code,
		Name:        item.Name,
		Description: item.Description,
		IsBuiltin:   item.IsBuiltin,
		Status:      int(item.Status),
		SortNo:      item.SortNo,
		Permissions: permissions,
		CreatedAt:   formatTime(item.CreatedAt),
		UpdatedAt:   formatTime(item.UpdatedAt),
	}
}

func BuildPlatformAuthRoleList(items []models.AuthRole, permissions map[int64][]string) []*response.PlatformAuthRoleResponse {
	ret := make([]*response.PlatformAuthRoleResponse, 0, len(items))
	for i := range items {
		ret = append(ret, BuildPlatformAuthRole(&items[i], permissions[items[i].ID]))
	}
	return ret
}

func BuildPlatformAuditLog(item *models.AuthAuditLog) *response.PlatformAuditLogResponse {
	if item == nil {
		return nil
	}
	metadata := buildPlatformAuditMetadata(item)
	return &response.PlatformAuditLogResponse{
		ID:               item.ID,
		TenantID:         item.TenantID,
		DomainType:       item.DomainType,
		ActorUserID:      item.ActorUserID,
		ActorSubjectType: item.ActorSubjectType,
		ActorSubjectID:   item.ActorSubjectID,
		TargetType:       item.TargetType,
		TargetID:         item.TargetID,
		Action:           item.Action,
		Summary:          auditMetadataValue(metadata, "contentSummary", "summary", "title"),
		Metadata:         metadata,
		RequestID:        item.RequestID,
		SupportGrantID:   item.SupportGrantID,
		IPAddress:        item.IPAddress,
		UserAgent:        item.UserAgent,
		RiskLevel:        displayAuthAuditRiskLevel(item),
		Status:           item.Status,
		OccurredAt:       formatTime(item.OccurredAt),
	}
}

func buildPlatformAuditMetadata(item *models.AuthAuditLog) map[string]string {
	if item == nil {
		return nil
	}
	state := parseAuditState(item.AfterStateJSON)
	if len(state) == 0 {
		state = parseAuditState(item.BeforeStateJSON)
	}
	keys := []string{
		"conversationId",
		"messageId",
		"messageType",
		"senderType",
		"senderId",
		"senderName",
		"contentSummary",
		"clientMsgId",
		"workflowRunId",
		"collaborationId",
		"ticketId",
		"ticketNo",
		"ticketProgressId",
		"meetingId",
		"summary",
		"title",
	}
	metadata := make(map[string]string, len(keys))
	for _, key := range keys {
		if value := auditStateString(state[key]); value != "" {
			metadata[key] = value
		}
	}
	switch item.TargetType {
	case "message":
		if metadata["messageId"] == "" {
			metadata["messageId"] = item.TargetID
		}
	case "conversation":
		if metadata["conversationId"] == "" {
			metadata["conversationId"] = item.TargetID
		}
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func parseAuditState(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "{") {
		return nil
	}
	var ret map[string]any
	if err := json.Unmarshal([]byte(raw), &ret); err != nil {
		return nil
	}
	return ret
}

func auditMetadataValue(metadata map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(metadata[key]); value != "" {
			return value
		}
	}
	return ""
}

func auditStateString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return ""
	}
}

func BuildPlatformAuditLogList(items []models.AuthAuditLog) []*response.PlatformAuditLogResponse {
	ret := make([]*response.PlatformAuditLogResponse, 0, len(items))
	for i := range items {
		ret = append(ret, BuildPlatformAuditLog(&items[i]))
	}
	return ret
}

func displayAuthAuditRiskLevel(item *models.AuthAuditLog) string {
	if item == nil {
		return ""
	}
	if item.Status != models.AuditStatusSuccess {
		return item.RiskLevel
	}
	switch item.Action {
	case models.AuditActionTenantCreated,
		"auth_policy.saved",
		"auth_role.created",
		"auth_role.updated",
		"customer_user.authorized",
		"partner_admin.invited",
		"platform_staff.invited",
		"subscription.assigned",
		"tenant_member.invited":
		return models.RiskLevelMedium
	default:
		return item.RiskLevel
	}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.DateTime)
}

func formatTimePtr(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format(time.DateTime)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

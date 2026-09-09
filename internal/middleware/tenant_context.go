package middleware

import (
	"strconv"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

// middlewareAuthPrincipalKey 是本包 AuthPrincipal 在 Gin Context 中的存储键。
// 与 services.AuthService 内部使用不同键，避免类型断言冲突。
const middlewareAuthPrincipalKey = "middlewareAuthPrincipal"

// tenantContextKey 是 TenantContext 在 Gin Context 中的存储键。
const tenantContextKey = "tenantContext"

// TenantContext 存储从认证信息或请求头中提取的租户上下文。
// 所有 handler / service 在需要访问租户信息时应通过 GetTenantContext 获取。
type TenantContext struct {
	// TenantID 当前请求的租户 ID。
	// 平台用户跨租户操作时为目标租户 ID；平台级操作为 0。
	TenantID int64 `json:"tenantId"`
	// UserID 登录用户 ID（users 表主键）。
	UserID int64 `json:"userId"`
	// UserType 用户类型：platform / enterprise / customer / service_account。
	UserType string `json:"userType"`
	// RequestTenantID 从请求头 X-Tenant-Id 中读取的原始值，用于二次校验。
	RequestTenantID string `json:"requestTenantId,omitempty"`
}

// TenantContextMiddleware 从认证后的 AuthPrincipal 中提取租户信息并注入 Gin Context。
//
// 执行顺序：该中间件必须在 AuthMiddleware 之后注册，确保用户已通过认证。
//
// 租户来源（按优先级）：
//  1. AuthPrincipal（已认证用户主体）中的 TenantID。
//     平台用户（platform）可跨租户操作，此时 TenantID 为 ""（或 "*"）；
//     企业/客户用户必须携带有效的 TenantID。
//  2. 请求头 X-Tenant-Id（向后兼容）。
//
// 校验规则：
//   - 平台用户（platform）：允许不携带 TenantID，支持跨租户操作。
//   - 企业用户（enterprise）/ 客户用户（customer）：禁止空 TenantID，否则返回 403。
func TenantContextMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 1. 获取已认证的用户主体（旧版 dto.AuthPrincipal）
		oldPrincipal := services.AuthService.GetAuthPrincipal(ctx)
		if oldPrincipal == nil {
			// 未认证，跳过（应由 AuthMiddleware 拦截）
			ctx.Next()
			return
		}

		// 2. 从请求头读取 X-Tenant-Id（向后兼容 / 跨租户切换时使用）
		headerTenantID := ctx.GetHeader("X-Tenant-Id")

		// 3. 确定用户类型和租户 ID
		//    优先从 AuthPrincipal 的 Domain/SubjectType 派生 user_type；
		//    若 AuthPrincipal 尚未设置 Domain，从角色推断。
		userType := deriveUserType(oldPrincipal)
		tenantID := deriveTenantID(oldPrincipal, headerTenantID, userType)

		// 4. 校验：企业 / 客户用户必须携带有效的 TenantID
		pendingCustomer := userType == UserTypeCustomer && oldPrincipal.SubjectType == models.SubjectTypePendingCustomer
		if userType == UserTypeEnterprise || (userType == UserTypeCustomer && !pendingCustomer) {
			if tenantID <= 0 {
				// 企业/客户域禁止空租户
				ctx.JSON(403, gin.H{"error": "tenant_id is required for enterprise/customer users"})
				ctx.Abort()
				return
			}
		}

		// 5. 构建 TenantContext
		tc := &TenantContext{
			TenantID:        tenantID,
			UserID:          oldPrincipal.UserID,
			UserType:        userType,
			RequestTenantID: headerTenantID,
		}
		ctx.Set(tenantContextKey, tc)

		// 6. 同步更新统一 AuthPrincipal，并用 middleware key 做兼容读取。
		oldPrincipal.TenantID = tenantID
		if userType == UserTypePlatform && tenantID > 0 {
			oldPrincipal.TargetTenantID = tenantID
		}
		oldPrincipal.DomainType = userType
		oldPrincipal.Domain = userType
		if oldPrincipal.SubjectType == "" {
			oldPrincipal.SubjectType = deriveSubjectType(userType)
		}
		if oldPrincipal.SubjectID == 0 {
			oldPrincipal.SubjectID = oldPrincipal.UserID
		}
		oldPrincipal.Timezone = deriveTimezone(oldPrincipal.Timezone)
		oldPrincipal.Locale = deriveLocale(oldPrincipal.Locale)
		ctx.Set(middlewareAuthPrincipalKey, oldPrincipal)

		ctx.Next()
	}
}

// deriveUserType 从 AuthPrincipal 中派生用户类型。
// 优先使用 Domain 字段，若未设置则通过角色推断。
func deriveUserType(p *dto.AuthPrincipal) string {
	if p == nil {
		return UserTypeEnterprise
	}
	if p.EffectiveDomainType() != "" {
		return p.EffectiveDomainType()
	}
	// 通过角色推断
	for _, role := range p.Roles {
		if role == "platform_admin" || role == "platform_staff" || role == "super_admin" {
			return UserTypePlatform
		}
	}
	return UserTypeEnterprise
}

// deriveTenantID 从 AuthPrincipal 和请求头中派生最终 tenant_id。
//
// 优先级：
//  1. AuthPrincipal.TenantID（已登录用户所绑定的租户）
//  2. X-Tenant-Id 请求头（跨租户切换或向后兼容）
func deriveTenantID(p *dto.AuthPrincipal, headerTenantID, userType string) int64 {
	headerID := convertStringTenantToInt64(headerTenantID)
	if p == nil {
		return headerID
	}
	if userType == UserTypePlatform && headerID > 0 {
		return headerID
	}
	if p.TenantID > 0 {
		return p.TenantID
	}
	return headerID
}

// convertStringTenantToInt64 将字符串格式的 tenant_id 转为 int64。
// 如果字符串为空或非数字，返回 0。
func convertStringTenantToInt64(tenantID string) int64 {
	if tenantID == "" || tenantID == "*" || tenantID == "platform" {
		return 0
	}
	id, err := strconv.ParseInt(tenantID, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

func deriveSubjectType(userType string) string {
	switch userType {
	case UserTypePlatform:
		return "platform_staff"
	case UserTypeCustomer:
		return "customer_user"
	case UserTypePartner:
		return "partner_account"
	case UserTypeServiceAccount:
		return "service_account"
	default:
		return "enterprise_member"
	}
}

// deriveLocale 派生用户语言偏好。
func deriveLocale(locale string) string {
	if locale != "" {
		return locale
	}
	return "en-US"
}

// deriveTimezone 派生用户时区。
func deriveTimezone(timezone string) string {
	if timezone != "" {
		return timezone
	}
	return "UTC"
}

// GetTenantContext 从 Gin Context 中获取 TenantContext。
// 如果中间件尚未执行或 context 中不存在，返回 nil。
func GetTenantContext(ctx *gin.Context) *TenantContext {
	if ctx == nil {
		return nil
	}
	v, exists := ctx.Get(tenantContextKey)
	if !exists {
		return nil
	}
	tc, ok := v.(*TenantContext)
	if !ok {
		return nil
	}
	return tc
}

// RequireTenant 要求当前请求必须携带合法的租户 ID。
// 如果不存在，返回 403 错误并终止请求。
func RequireTenant(ctx *gin.Context) error {
	tc := GetTenantContext(ctx)
	if tc == nil || tc.TenantID <= 0 {
		return errorsx.ForbiddenI18n("error.e0225")
	}
	return nil
}

// GetAuthPrincipal 从 Gin Context 中获取扩展的 AuthPrincipal。
// 该对象由 TenantContextMiddleware 设置，包含租户信息和权限数据。
// 如果中间件尚未执行或 context 中不存在，返回 nil。
// 注意：services.AuthService.GetAuthPrincipal 返回的是旧版 dto.AuthPrincipal，
// 与本方法的返回类型不同。
func GetAuthPrincipal(c *gin.Context) *AuthPrincipal {
	if c == nil {
		return nil
	}
	v, exists := c.Get(middlewareAuthPrincipalKey)
	if !exists {
		return nil
	}
	p, ok := v.(*AuthPrincipal)
	if !ok {
		return nil
	}
	return p
}

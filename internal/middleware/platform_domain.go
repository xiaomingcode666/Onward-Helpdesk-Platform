package middleware

import (
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"

	"github.com/gin-gonic/gin"
)

// PlatformOnlyMiddleware prevents tenant-domain identities from accessing
// cross-tenant platform administration APIs.
func PlatformOnlyMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		principal := GetAuthPrincipal(ctx)
		if isPlatformPrincipal(principal) {
			ctx.Next()
			return
		}
		httpx.WriteJSON(ctx, errorsx.ForbiddenI18n("error.e0225"))
		ctx.Abort()
	}
}

// EnterpriseOnlyMiddleware keeps customer and partner identities out of the
// tenant administration surface while preserving explicit platform support access.
func EnterpriseOnlyMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		principal := GetAuthPrincipal(ctx)
		if principal != nil && (principal.IsEnterprise() || isPlatformPrincipal(principal)) {
			ctx.Next()
			return
		}
		httpx.WriteJSON(ctx, errorsx.ForbiddenI18n("error.e0225"))
		ctx.Abort()
	}
}

// PartnerOnlyMiddleware restricts supplier collaboration APIs to resolved
// partner_account principals. Resource-level authorization remains in services.
func PartnerOnlyMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		principal := GetAuthPrincipal(ctx)
		if principal != nil && principal.IsPartner() && principal.PartnerAccountID > 0 {
			ctx.Next()
			return
		}
		httpx.WriteJSON(ctx, errorsx.ForbiddenI18n("error.e0225"))
		ctx.Abort()
	}
}

func isPlatformPrincipal(principal *dto.AuthPrincipal) bool {
	if principal == nil {
		return false
	}
	if principal.IsPlatform() {
		return true
	}
	return principal.HasRole("super_admin") || principal.HasRole("platform_admin") || principal.HasRole("platform_staff")
}

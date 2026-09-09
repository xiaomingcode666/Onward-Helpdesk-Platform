package middleware

import (
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

// RequirePermissionMiddleware keeps route authorization explicit at registration time.
func RequirePermissionMiddleware(permission constants.Permission) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if _, err := services.AuthService.RequirePermission(ctx, permission); err != nil {
			httpx.WriteJSON(ctx, err)
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}

// RequireAnyPermissionMiddleware allows a route to keep legacy permissions while introducing
// narrower domain permissions for newly separated roles.
func RequireAnyPermissionMiddleware(permissions ...constants.Permission) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if _, err := services.AuthService.RequireAnyPermission(ctx, permissions...); err != nil {
			httpx.WriteJSON(ctx, err)
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}

func RequireTenantAICapabilityMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		principal := services.AuthService.GetAuthPrincipal(ctx)
		if principal == nil || principal.EffectiveTenantID() <= 0 || !services.TenantCapabilityService.AIEnabled(principal.EffectiveTenantID()) {
			httpx.WriteJSON(ctx, errorsx.Forbidden("tenant AI capability is disabled"))
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}

package middleware

import (
	"net/http"

	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware(ctx *gin.Context) {
	if !authenticateRequest(ctx) {
		return
	}
	ctx.Next()
}

func authenticateRequest(ctx *gin.Context) bool {
	if _, err := services.AuthService.Authenticate(ctx); err != nil {
		httpx.WriteHttpStatusJSON(ctx, http.StatusUnauthorized, errorsx.UnauthorizedI18n("error.auth.expired"))
		ctx.Abort()
		return false
	}
	return true
}

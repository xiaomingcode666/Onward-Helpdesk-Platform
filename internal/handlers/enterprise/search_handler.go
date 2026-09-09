package enterprise

import (
	"strings"

	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func GlobalSearch(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	scopes := []string{}
	if raw := strings.TrimSpace(ctx.Query("scopes")); raw != "" {
		scopes = strings.Split(raw, ",")
	}
	result, err := services.EnterpriseSearchService.Search(tenantID, ctx.Query("q"), scopes, services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

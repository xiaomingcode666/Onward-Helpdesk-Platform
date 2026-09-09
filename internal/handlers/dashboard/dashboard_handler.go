package dashboard

import (
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/pkg/i18nx"

	"github.com/gin-gonic/gin"
)

func DashboardGetOverview(ctx *gin.Context) {
	rangeValue, _ := params.Get(ctx, "range")
	httpx.WriteJSON(ctx, services.DashboardService.GetOverview(rangeValue, i18nx.Locale(ctx)))
}

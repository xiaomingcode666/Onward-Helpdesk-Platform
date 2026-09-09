package api

import (
	"net/http"

	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
)

func TenantBrandingLogoGet(ctx *gin.Context) {
	asset := services.TenantPortalSettingsService.ResolvePublicLogoAssetDB(sqls.DB(), ctx.Param("assetId"))
	if asset == nil {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}
	streamPublicImmutableAsset(ctx, asset)
}

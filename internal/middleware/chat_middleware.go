package middleware

import (
	"strconv"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func ExternalUserMiddleware(ctx *gin.Context) {
	authorization := strings.TrimSpace(ctx.GetHeader("Authorization"))
	if strings.HasPrefix(authorization, "Bearer ak_") {
		principal, err := services.AuthService.Authenticate(ctx)
		if err != nil || principal == nil || principal.DomainType != models.DomainTypeCustomer {
			if err == nil {
				err = errorsx.Forbidden("customer identity is required")
			}
			httpx.WriteJSON(ctx, err)
			ctx.Abort()
			return
		}
		httpx.SetExternalUser(ctx, &openidentity.ExternalUser{
			ExternalSource: enums.ExternalSourceUser,
			ExternalID:     strconv.FormatInt(principal.UserID, 10),
			ExternalName:   principal.Nickname,
		})
		httpx.SetCustomerSessionAccess(ctx, httpx.CustomerSessionAccess{TenantID: principal.TenantID})
		ctx.Next()
		return
	}
	channel := services.ChannelService.GetEnabledChannel(ctx)
	result, err := services.CustomerSessionService.VerifyRequest(ctx, channel)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		ctx.Abort()
		return
	}
	services.CustomerSessionService.SetRefreshHeaders(ctx, result)
	httpx.SetExternalUser(ctx, result.ExternalUser)
	httpx.SetCustomerSessionAccess(ctx, httpx.CustomerSessionAccess{
		ChannelID:      result.ChannelID,
		EntrySessionID: result.EntrySessionID,
		TenantID:       result.TenantID,
		ProductID:      result.ProductID,
	})
	ctx.Next()
}

func CustomerPortalAccountMiddleware(ctx *gin.Context) {
	principal, err := services.AuthService.Authenticate(ctx)
	if err != nil || principal == nil || !principal.IsCustomer() {
		if err == nil {
			err = errorsx.Unauthorized("customer account is required")
		}
		httpx.WriteJSON(ctx, err)
		ctx.Abort()
		return
	}
	httpx.SetExternalUser(ctx, &openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     strconv.FormatInt(principal.UserID, 10),
		ExternalName:   principal.Nickname,
	})
	httpx.SetCustomerSessionAccess(ctx, httpx.CustomerSessionAccess{TenantID: principal.TenantID})
	ctx.Next()
}

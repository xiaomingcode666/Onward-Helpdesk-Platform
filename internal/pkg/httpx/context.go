package httpx

import (
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/pkg/tracex"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/common/strs"
)

const (
	ctxKeyExternalUser          = "externalUser"
	ctxKeyCustomerSessionAccess = "customerSessionAccess"
)

type CustomerSessionAccess struct {
	ChannelID      int64
	EntrySessionID int64
	TenantID       int64
	ProductID      int64
}

func SetExternalUser(ctx *gin.Context, ext *openidentity.ExternalUser) {
	ctx.Set(ctxKeyExternalUser, ext)
}

func GetExternalUser(ctx *gin.Context) *openidentity.ExternalUser {
	v, _ := ctx.Get(ctxKeyExternalUser)
	ext, _ := v.(*openidentity.ExternalUser)
	return ext
}

func SetCustomerSessionAccess(ctx *gin.Context, access CustomerSessionAccess) {
	ctx.Set(ctxKeyCustomerSessionAccess, access)
}

func GetCustomerSessionAccess(ctx *gin.Context) CustomerSessionAccess {
	value, _ := ctx.Get(ctxKeyCustomerSessionAccess)
	access, _ := value.(CustomerSessionAccess)
	return access
}

func GetChannelID(ctx *gin.Context) string {
	if channelID := ctx.GetHeader("X-Channel-ID"); strs.IsNotBlank(channelID) {
		return channelID
	}
	if channelID, _ := params.Get(ctx, "channelId"); strs.IsNotBlank(channelID) {
		return channelID
	}
	return ""
}

func GetRequestID(ctx *gin.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Get(tracex.GinRequestIDKey); ok {
		if requestID, ok := value.(string); ok {
			return tracex.NormalizeRequestID(requestID)
		}
	}
	return tracex.NormalizeRequestID(ctx.GetHeader(tracex.RequestIDHeader))
}

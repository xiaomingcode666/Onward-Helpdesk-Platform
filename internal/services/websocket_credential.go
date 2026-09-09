package services

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	webSocketCredentialProtocol            = "rhd.credential"
	webSocketAccessTokenProtocolPrefix     = "rhd.access."
	webSocketCustomerSessionProtocolPrefix = "rhd.customer."
)

func webSocketNegotiatedProtocol(ctx *gin.Context) string {
	if ctx == nil {
		return ""
	}
	for _, item := range strings.Split(ctx.GetHeader("Sec-WebSocket-Protocol"), ",") {
		if strings.TrimSpace(item) == webSocketCredentialProtocol {
			return webSocketCredentialProtocol
		}
	}
	return ""
}

func webSocketProtocolCredential(ctx *gin.Context, prefix string) string {
	if ctx == nil || strings.TrimSpace(prefix) == "" {
		return ""
	}
	for _, item := range strings.Split(ctx.GetHeader("Sec-WebSocket-Protocol"), ",") {
		protocol := strings.TrimSpace(item)
		if strings.HasPrefix(protocol, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(protocol, prefix))
		}
	}
	return ""
}

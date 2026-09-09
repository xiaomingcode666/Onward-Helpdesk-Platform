package api

import (
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

// MarketingDemoRequest accepts an unauthenticated request from the public homepage.
func MarketingDemoRequest(ctx *gin.Context) {
	req := request.DemoRequestRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.MarketingDemoService.Submit(req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

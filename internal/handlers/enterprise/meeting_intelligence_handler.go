package enterprise

import (
	"strconv"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func MeetingGetTranscripts(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	items, err := services.MeetingIntelligenceService.ListTranscriptsForOperator(ctx.Param("id"), operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildMeetingTranscriptList(items))
}

func MeetingGetTranscriptPage(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	items, nextCursor, hasMore, err := services.MeetingIntelligenceService.ListTranscriptPageForOperator(
		ctx.Param("id"), ctx.Query("cursor"), limit, operator,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, httpx.CursorData(builders.BuildMeetingTranscriptList(items), nextCursor, hasMore))
}

func MeetingPostTranscript(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var input request.MeetingTranscriptIngestRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid transcript request"))
		return
	}
	item, err := services.MeetingIntelligenceService.IngestClientTranscriptForOperator(
		ctx.Request.Context(), ctx.Param("id"), input, operator,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildMeetingTranscript(item))
}

func MeetingGetAnnotations(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	items, err := services.MeetingIntelligenceService.ListAnnotationsForOperator(ctx.Param("id"), operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	frameURLs := services.MeetingIntelligenceService.ResolveAnnotationFrameURLs(items)
	httpx.WriteJSON(ctx, builders.BuildMeetingARAnnotationList(items, frameURLs))
}

func MeetingPostAnnotation(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var input request.MeetingARAnnotationCreateRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid annotation request"))
		return
	}
	item, err := services.MeetingIntelligenceService.CreateAnnotationForOperator(ctx.Param("id"), input, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	frameURLs := services.MeetingIntelligenceService.ResolveAnnotationFrameURLs([]models.MeetingARAnnotation{*item})
	httpx.WriteJSON(ctx, builders.BuildMeetingARAnnotation(item, frameURLs[item.FrameAssetID]))
}

func MeetingPostFrame(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	file, err := ctx.FormFile("frame")
	if err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("frame image is required"))
		return
	}
	asset, url, err := services.MeetingIntelligenceService.UploadFrameForOperator(ctx.Param("id"), file, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, response.MeetingARFrameResponse{AssetID: asset.ID, URL: url})
}

func MeetingPostFrameDetect(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	frameAssetID, err := strconv.ParseInt(ctx.Param("frameId"), 10, 64)
	if err != nil || frameAssetID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid frame asset id"))
		return
	}
	items, err := services.MeetingIntelligenceService.DetectFrameForOperator(ctx.Request.Context(), ctx.Param("id"), frameAssetID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	frameURLs := services.MeetingIntelligenceService.ResolveAnnotationFrameURLs(items)
	httpx.WriteJSON(ctx, builders.BuildMeetingARAnnotationList(items, frameURLs))
}

package partner

import (
	"strconv"
	"strings"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func Profile(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	item, err := services.TicketSupplierCollaborationService.PartnerProfile(operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPartnerPortalProfile(*item))
}

func AccountList(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	items, _, err := services.TicketSupplierCollaborationService.ListPartnerCompanyAccounts(operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPartnerPortalAccounts(items, operator.PartnerAccountID))
}

func AccountCreate(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	var req dto.PartnerAccountCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	result, err := services.TicketSupplierCollaborationService.CreatePartnerAccount(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, dto.PartnerAccountCreateResultDTO{
		Account:         builders.BuildPartnerPortalAccount(result.Account, operator.PartnerAccountID),
		InitialPassword: result.InitialPassword,
	})
}

func AccountUpdate(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	accountID, err := strconv.ParseInt(ctx.Param("accountId"), 10, 64)
	if err != nil || accountID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid partner account id"))
		return
	}
	var req dto.PartnerAccountUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.TicketSupplierCollaborationService.UpdatePartnerAccount(accountID, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPartnerPortalAccount(*item, operator.PartnerAccountID))
}

func MeetingList(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	items, err := services.TicketSupplierCollaborationService.ListPortalMeetings(ctx.Request.Context(), ctx.Query("status"), operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPartnerPortalMeetings(items))
}

func TicketList(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	items, err := services.TicketSupplierCollaborationService.ListForPartner(operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPartnerTicketSupplierCollaborations(items))
}

func ConversationList(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	items, err := services.TicketSupplierCollaborationService.ListConversationsForPartner(operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPartnerTicketSupplierCollaborations(items))
}

func TicketGet(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	id, ok := collaborationID(ctx)
	if !ok {
		return
	}
	item, err := services.TicketSupplierCollaborationService.PartnerTicketDetail(id, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPartnerTicketDetail(item))
}

func TicketProgressCreate(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	id, ok := collaborationID(ctx)
	if !ok {
		return
	}
	var req dto.PartnerTicketProgressCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.TicketSupplierCollaborationService.AddPartnerMessage(id, req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPartnerTicketDetail(item))
}

func TicketUploadImage(ctx *gin.Context) {
	uploadPartnerMessageAsset(ctx, "images", "image/")
}

func TicketUploadAudio(ctx *gin.Context) {
	uploadPartnerMessageAsset(ctx, "audio", "audio/")
}

func TicketUploadAttachment(ctx *gin.Context) {
	uploadPartnerMessageAsset(ctx, "attachments", "")
}

func uploadPartnerMessageAsset(ctx *gin.Context, prefix, requiredMIMEPrefix string) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	id, ok := collaborationID(ctx)
	if !ok {
		return
	}
	conversation, err := services.TicketSupplierCollaborationService.ResolvePartnerMessageUploadConversation(id, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	header, err := ctx.FormFile("file")
	if err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("message asset file is required"))
		return
	}
	if requiredMIMEPrefix != "" && !strings.HasPrefix(strings.ToLower(header.Header.Get("Content-Type")), requiredMIMEPrefix) {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("message asset MIME type is invalid"))
		return
	}
	item, err := services.AssetService.UploadConversationFile(header, prefix, conversation.ID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildMessageAsset(item))
}

func TicketAccept(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	id, ok := collaborationID(ctx)
	if !ok {
		return
	}
	item, err := services.TicketSupplierCollaborationService.Accept(id, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPartnerTicketSupplierCollaboration(*item))
}

func TicketAssign(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	id, ok := collaborationID(ctx)
	if !ok {
		return
	}
	var req dto.PartnerTicketAssignRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.TicketSupplierCollaborationService.Assign(id, req.PartnerAccountID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPartnerTicketSupplierCollaboration(*item))
}

func TicketParticipantAdd(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	id, ok := collaborationID(ctx)
	if !ok {
		return
	}
	var req dto.PartnerTicketParticipantRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.TicketSupplierCollaborationService.AddParticipant(id, req.PartnerAccountID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPartnerTicketDetail(item))
}

func TicketParticipantRemove(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	id, ok := collaborationID(ctx)
	if !ok {
		return
	}
	accountID, err := strconv.ParseInt(ctx.Param("accountId"), 10, 64)
	if err != nil || accountID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid partner account id"))
		return
	}
	item, err := services.TicketSupplierCollaborationService.RemoveParticipant(id, accountID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPartnerTicketDetail(item))
}

func TicketResolve(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	id, ok := collaborationID(ctx)
	if !ok {
		return
	}
	var req dto.TicketSupplierResolveRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.TicketSupplierCollaborationService.Resolve(id, req.Resolution, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildPartnerTicketSupplierCollaboration(*item))
}

func TicketMeetings(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	ticketID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid ticket id"))
		return
	}
	items, err := services.TicketSupplierCollaborationService.ListMeetingsForPartner(ctx.Request.Context(), ticketID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, items)
}

func MeetingJoin(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	id, ok := collaborationID(ctx)
	if !ok {
		return
	}
	item, err := services.TicketSupplierCollaborationService.JoinMeeting(ctx.Request.Context(), id, ctx.Param("meetingId"), operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

func MeetingJoinByTicket(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	ticketID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid ticket id"))
		return
	}
	item, err := services.TicketSupplierCollaborationService.JoinMeetingForTicket(ctx.Request.Context(), ticketID, ctx.Param("meetingId"), operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, item)
}

func MeetingJoined(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	id, ok := collaborationID(ctx)
	if !ok {
		return
	}
	if err := services.TicketSupplierCollaborationService.ConfirmMeetingJoined(ctx.Request.Context(), id, ctx.Param("meetingId"), operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func MeetingLeft(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	id, ok := collaborationID(ctx)
	if !ok {
		return
	}
	if err := services.TicketSupplierCollaborationService.ConfirmMeetingLeft(ctx.Request.Context(), id, ctx.Param("meetingId"), operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func MeetingHeartbeat(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	id, ok := collaborationID(ctx)
	if !ok {
		return
	}
	if err := services.TicketSupplierCollaborationService.HeartbeatMeeting(ctx.Request.Context(), id, ctx.Param("meetingId"), operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

func MeetingStatus(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	id, ok := collaborationID(ctx)
	if !ok {
		return
	}
	status, err := services.TicketSupplierCollaborationService.GetMeetingStatus(ctx.Request.Context(), id, ctx.Param("meetingId"), operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, status)
}

func MeetingTranscripts(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	id, ok := collaborationID(ctx)
	if !ok {
		return
	}
	items, err := services.TicketSupplierCollaborationService.ListMeetingTranscripts(id, ctx.Param("meetingId"), operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildMeetingTranscriptList(items))
}

func MeetingTranscriptPage(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	id, ok := collaborationID(ctx)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	items, nextCursor, hasMore, err := services.TicketSupplierCollaborationService.ListMeetingTranscriptPage(
		id, ctx.Param("meetingId"), ctx.Query("cursor"), limit, operator,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, httpx.CursorData(builders.BuildMeetingTranscriptList(items), nextCursor, hasMore))
}

func MeetingTranscriptCreate(ctx *gin.Context) {
	operator, ok := requirePartner(ctx)
	if !ok {
		return
	}
	id, ok := collaborationID(ctx)
	if !ok {
		return
	}
	var input request.MeetingTranscriptIngestRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid transcript request"))
		return
	}
	item, err := services.TicketSupplierCollaborationService.IngestMeetingTranscript(ctx.Request.Context(), id, ctx.Param("meetingId"), input, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildMeetingTranscript(item))
}

func requirePartner(ctx *gin.Context) (*dto.AuthPrincipal, bool) {
	operator := services.AuthService.GetAuthPrincipal(ctx)
	if operator == nil || !operator.IsPartner() {
		httpx.WriteJSON(ctx, errorsx.ForbiddenI18n("error.e0225"))
		return nil, false
	}
	return operator, true
}

func collaborationID(ctx *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid supplier collaboration id"))
		return 0, false
	}
	return id, true
}

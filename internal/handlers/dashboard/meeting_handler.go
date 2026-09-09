package dashboard

import (
	"strconv"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
	"github.com/mlogclub/simple/web"
)

// createMeetingRequest 创建会议请求
type createMeetingRequest struct {
	TenantID string `json:"tenantId"` // 废弃字段：不再使用客户端传入的 tenantId
	TicketID string `json:"ticketId" validate:"required"`
}

// MeetingPostCreate 从工单发起会议
//
// 权限校验：
//   - 需要 meeting.create 权限
//   - 从 AuthPrincipal 获取 tenant_id，不使用客户端传入的值
//   - 校验工单存在且当前用户是处理人
func MeetingPostCreate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := createMeetingRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	// 从 AuthPrincipal 获取 tenant_id，不使用客户端传入的值
	// 如果前提权未返回 tenantID，尝试从工单获取
	ticketID := req.TicketID

	// 校验工单存在
	ticketIDInt, parseErr := strconv.ParseInt(ticketID, 10, 64)
	if parseErr != nil {
		httpx.WriteJSON(ctx, web.JsonErrorMsg("invalid ticket id"))
		return
	}

	var ticket models.Ticket
	if err := sqls.DB().First(&ticket, ticketIDInt).Error; err != nil {
		httpx.WriteJSON(ctx, web.JsonErrorMsg("ticket not found"))
		return
	}

	// 校验当前用户是否是工单处理人
	if ticket.CurrentAssigneeID > 0 {
		if operator.UserID != ticket.CurrentAssigneeID {
			httpx.WriteJSON(ctx, web.JsonErrorMsg("only the ticket assignee can create a meeting"))
			return
		}
	}

	joinConfig, err := services.MeetingService.CreateMeetingRoomForOperator(ctx.Request.Context(), ticketIDInt, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	services.AuditService.RecordAudit(ctx.Request.Context(), services.RecordAuditInput{
		TenantID:     ticket.TenantID,
		ActorID:      strconv.FormatInt(operator.UserID, 10),
		ActorType:    "user",
		Domain:       "meeting",
		ResourceType: "meeting",
		ResourceID:   joinConfig.MeetingID,
		Action:       "meeting.created",
		AfterState:   map[string]any{"ticketId": ticketID, "meetingId": joinConfig.MeetingID},
		RiskLevel:    "low",
	})

	httpx.WriteJSON(ctx, web.JsonData(joinConfig))
}

// MeetingPostJoin 获取入会配置
func MeetingPostJoin(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	meetingID := ctx.Param("id")
	if meetingID == "" {
		httpx.WriteJSON(ctx, web.JsonErrorMsg("meeting id is required"))
		return
	}

	joinConfig, err := services.MeetingService.JoinMeetingForOperator(ctx.Request.Context(), meetingID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	services.AuditService.RecordAudit(ctx.Request.Context(), services.RecordAuditInput{
		TenantID:     operator.EffectiveTenantID(),
		ActorID:      strconv.FormatInt(operator.UserID, 10),
		ActorType:    "user",
		Domain:       "meeting",
		ResourceType: "meeting",
		ResourceID:   meetingID,
		Action:       "meeting.joined",
		AfterState:   map[string]any{"meetingId": meetingID},
		RiskLevel:    "low",
	})

	httpx.WriteJSON(ctx, web.JsonData(joinConfig))
}

// MeetingPostEnd 结束会议
func MeetingPostEnd(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	meetingID := ctx.Param("id")
	if meetingID == "" {
		httpx.WriteJSON(ctx, web.JsonErrorMsg("meeting id is required"))
		return
	}

	changed, err := services.MeetingService.EndMeetingForOperatorWithResult(ctx.Request.Context(), meetingID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if changed {
		services.AuditService.RecordAudit(ctx.Request.Context(), services.RecordAuditInput{
			TenantID:     operator.EffectiveTenantID(),
			ActorID:      strconv.FormatInt(operator.UserID, 10),
			ActorType:    "user",
			Domain:       "meeting",
			ResourceType: "meeting",
			ResourceID:   meetingID,
			Action:       "meeting.ended",
			AfterState:   map[string]any{"meetingId": meetingID},
			RiskLevel:    "medium",
		})
	}

	httpx.WriteJSON(ctx, nil)
}

// MeetingGetStatus 获取会议状态
func MeetingGetStatus(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	meetingID := ctx.Param("id")
	if meetingID == "" {
		httpx.WriteJSON(ctx, web.JsonErrorMsg("meeting id is required"))
		return
	}

	status, err := services.MeetingService.GetMeetingStatusForOperator(ctx.Request.Context(), meetingID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	httpx.WriteJSON(ctx, web.JsonData(status))
}

// MeetingGetByTicket 获取工单关联的会议列表
func MeetingGetByTicket(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	ticketID := ctx.Param("ticketId")
	if ticketID == "" {
		httpx.WriteJSON(ctx, web.JsonErrorMsg("ticket id is required"))
		return
	}

	meetings, err := services.MeetingService.ListMeetingsByTicketForOperator(ctx.Request.Context(), ticketID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	httpx.WriteJSON(ctx, web.JsonData(meetings))
}

// 编译时检查
var _ = utils.UUID
var _ = time.Now

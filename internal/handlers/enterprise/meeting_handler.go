package enterprise

import (
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

type createMeetingRequest struct {
	Title            string `json:"title"`
	ScheduledAt      string `json:"scheduled_at"`
	ScheduledAtCamel string `json:"scheduledAt"`
}

// MeetingGetList 获取当前企业成员的个人视频会议清单。
// GET /api/enterprise/v1/meetings
func MeetingGetList(ctx *gin.Context) {
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return
	}
	principal, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "50"))
	deviceID := firstNonZero(queryInt64(ctx, "device_id"), queryInt64(ctx, "deviceId"))
	result, err := services.MeetingService.ListPersonalMeetingsSearchPageForOperator(
		ctx.Request.Context(), tenantID, ctx.Query("status"), ctx.Query("q"), page, pageSize, principal, deviceID,
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}

// MeetingPostCreate 从工单发起会议 (enterprise v1)
// POST /api/enterprise/v1/tickets/:id/meetings
func MeetingPostCreate(ctx *gin.Context) {
	ticketIDStr := ctx.Param("id")
	ticketID, err := strconv.ParseInt(ticketIDStr, 10, 64)
	if err != nil || ticketID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid ticket id"))
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingCreate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var req createMeetingRequest
	if ctx.Request.ContentLength > 0 {
		if err := ctx.ShouldBindJSON(&req); err != nil {
			httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid request body"))
			return
		}
	}
	scheduledAt, err := parseCreateMeetingScheduledAt(firstNonEmptyMeetingString(req.ScheduledAt, req.ScheduledAtCamel))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	joinConfig, err := services.MeetingService.CreateMeetingRoomForOperatorWithOptions(
		ctx.Request.Context(),
		ticketID,
		operator,
		services.CreateMeetingRoomOptions{ScheduledAt: scheduledAt},
	)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	services.AuditService.RecordAudit(ctx.Request.Context(), services.RecordAuditInput{
		TenantID:       operator.TenantID,
		ActorID:        strconv.FormatInt(operator.UserID, 10),
		ActorType:      "user",
		Domain:         "meeting",
		ResourceType:   "meeting",
		ResourceID:     joinConfig.MeetingID,
		Action:         "meeting.created",
		AfterState:     map[string]any{"ticketId": ticketID, "meetingId": joinConfig.MeetingID},
		IPAddress:      ctx.ClientIP(),
		UserAgent:      ctx.Request.UserAgent(),
		RequestID:      httpx.GetRequestID(ctx),
		SupportGrantID: operator.SupportGrantID,
		RiskLevel:      "low",
	})
	httpx.WriteJSON(ctx, joinConfig)
}

func parseCreateMeetingScheduledAt(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, errorsx.InvalidParam("invalid scheduled_at")
	}
	if !parsed.After(time.Now().Add(time.Minute)) {
		return nil, errorsx.InvalidParam("预约时间需晚于当前时间")
	}
	normalized := parsed.UTC()
	return &normalized, nil
}

func firstNonEmptyMeetingString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// MeetingGetByTicket 获取工单关联的会议列表 (enterprise v1)
// GET /api/enterprise/v1/tickets/:id/meetings
func MeetingGetByTicket(ctx *gin.Context) {
	ticketIDStr := ctx.Param("id")
	ticketID, err := strconv.ParseInt(ticketIDStr, 10, 64)
	if err != nil || ticketID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid ticket id"))
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	meetings, err := services.MeetingService.ListTicketMeetingsForOperator(ctx.Request.Context(), ticketID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, meetings)
}

// MeetingGetJoin 获取会议加入配置 (enterprise v1)
// GET /api/enterprise/v1/meetings/:id/join
func MeetingGetJoin(ctx *gin.Context) {
	meetingID := ctx.Param("id")
	if meetingID == "" {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid meeting id"))
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	joinConfig, err := services.MeetingService.JoinMeetingForOperator(ctx.Request.Context(), meetingID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	services.AuditService.RecordAudit(ctx.Request.Context(), services.RecordAuditInput{
		TenantID:       operator.EffectiveTenantID(),
		ActorID:        strconv.FormatInt(operator.UserID, 10),
		ActorType:      "user",
		Domain:         "meeting",
		ResourceType:   "meeting",
		ResourceID:     meetingID,
		Action:         "meeting.join_requested",
		AfterState:     map[string]any{"meetingId": meetingID},
		IPAddress:      ctx.ClientIP(),
		UserAgent:      ctx.Request.UserAgent(),
		RequestID:      httpx.GetRequestID(ctx),
		SupportGrantID: operator.SupportGrantID,
		RiskLevel:      "low",
	})
	httpx.WriteJSON(ctx, joinConfig)
}

// MeetingPostJoined confirms that the embedded Jitsi client actually joined.
// POST /api/enterprise/v1/meetings/:id/_joined
func MeetingPostJoined(ctx *gin.Context) {
	meetingID := ctx.Param("id")
	if meetingID == "" {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid meeting id"))
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.MeetingService.ConfirmJoinForOperator(ctx.Request.Context(), meetingID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	services.AuditService.RecordAudit(ctx.Request.Context(), services.RecordAuditInput{
		TenantID: operator.EffectiveTenantID(), ActorID: strconv.FormatInt(operator.UserID, 10), ActorType: "user",
		Domain: "meeting", ResourceType: "meeting", ResourceID: meetingID, Action: "meeting.joined",
		AfterState: map[string]any{"meetingId": meetingID},
		IPAddress:  ctx.ClientIP(), UserAgent: ctx.Request.UserAgent(), RequestID: httpx.GetRequestID(ctx),
		SupportGrantID: operator.SupportGrantID, RiskLevel: "low",
	})
	httpx.WriteJSON(ctx, nil)
}

// MeetingPostLeft confirms that the embedded Jitsi client left the room.
// POST /api/enterprise/v1/meetings/:id/_left
func MeetingPostLeft(ctx *gin.Context) {
	meetingID := ctx.Param("id")
	if meetingID == "" {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid meeting id"))
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.MeetingService.ConfirmLeaveForOperator(ctx.Request.Context(), meetingID, operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	services.AuditService.RecordAudit(ctx.Request.Context(), services.RecordAuditInput{
		TenantID: operator.EffectiveTenantID(), ActorID: strconv.FormatInt(operator.UserID, 10), ActorType: "user",
		Domain: "meeting", ResourceType: "meeting", ResourceID: meetingID, Action: "meeting.left",
		AfterState: map[string]any{"meetingId": meetingID},
		IPAddress:  ctx.ClientIP(), UserAgent: ctx.Request.UserAgent(), RequestID: httpx.GetRequestID(ctx),
		SupportGrantID: operator.SupportGrantID, RiskLevel: "low",
	})
	httpx.WriteJSON(ctx, nil)
}

func MeetingPostHeartbeat(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if err := services.MeetingService.HeartbeatForOperator(ctx.Request.Context(), ctx.Param("id"), operator); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, nil)
}

// MeetingGetStatus returns the authoritative application meeting state used
// by embedded clients to stop stale Jitsi sessions after a room is ended.
func MeetingGetStatus(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	status, err := services.MeetingService.GetMeetingStatusForOperator(ctx.Request.Context(), ctx.Param("id"), operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, status)
}

// MeetingPostEnd 结束会议 (enterprise v1)
// POST /api/enterprise/v1/meetings/:id/_end
func MeetingPostEnd(ctx *gin.Context) {
	meetingID := ctx.Param("id")
	if meetingID == "" {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid meeting id"))
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionMeetingUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	changed, err := services.MeetingService.EndMeetingForOperatorWithResult(ctx.Request.Context(), meetingID, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if changed {
		services.AuditService.RecordAudit(ctx.Request.Context(), services.RecordAuditInput{
			TenantID:       operator.EffectiveTenantID(),
			ActorID:        strconv.FormatInt(operator.UserID, 10),
			ActorType:      "user",
			Domain:         "meeting",
			ResourceType:   "meeting",
			ResourceID:     meetingID,
			Action:         "meeting.ended",
			AfterState:     map[string]any{"meetingId": meetingID},
			IPAddress:      ctx.ClientIP(),
			UserAgent:      ctx.Request.UserAgent(),
			RequestID:      httpx.GetRequestID(ctx),
			SupportGrantID: operator.SupportGrantID,
			RiskLevel:      "medium",
		})
	}
	httpx.WriteJSON(ctx, nil)
}

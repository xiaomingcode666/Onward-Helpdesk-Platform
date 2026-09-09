package dashboard

import (
	"time"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/pkg/httpx/params"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func AgentSelfGetBriefing(ctx *gin.Context) {
	operator := services.AuthService.GetAuthPrincipal(ctx)
	if operator == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}
	result, err := services.AgentWorkStatusService.GetBriefing(operator, time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEngineerBriefingResponse(result))
}

func AgentSelfGetWorkSchedule(ctx *gin.Context) {
	operator := services.AuthService.GetAuthPrincipal(ctx)
	if operator == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}
	result, err := services.AgentTeamScheduleService.GetMyWeeklySchedule(operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	ret := response.EngineerWorkScheduleResponse{
		IsEngineer:     result.IsEngineer,
		Teams:          make([]response.AgentTeamResponse, 0, len(result.Teams)),
		Schedules:      make([]response.AgentTeamScheduleResponse, 0, len(result.Schedules)),
		DraftSchedules: make([]response.AgentTeamScheduleResponse, 0, len(result.DraftSchedules)),
		BaseSchedule:   buildAgentTeamScheduleTemplateResponse(result.BaseSchedule),
		Timezone:       result.Timezone,
		Exceptions:     make([]response.AgentScheduleExceptionResponse, 0, len(result.Exceptions)),
	}
	for _, item := range result.Teams {
		ret.Teams = append(ret.Teams, buildAgentTeamResponse(&item))
	}
	for _, item := range result.Schedules {
		ret.Schedules = append(ret.Schedules, buildAgentTeamScheduleResponse(&item))
	}
	for _, item := range result.DraftSchedules {
		ret.DraftSchedules = append(ret.DraftSchedules, buildAgentTeamScheduleResponse(&item))
	}
	for _, item := range result.Exceptions {
		ret.Exceptions = append(ret.Exceptions, buildAgentScheduleExceptionResponse(&item))
	}
	httpx.WriteJSON(ctx, ret)
}

func AgentSelfPostLeaveCreate(ctx *gin.Context) {
	operator := services.AuthService.GetAuthPrincipal(ctx)
	if operator == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}
	req := request.CreateAgentScheduleLeaveRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.AgentScheduleExceptionService.CreateMyLeave(req, operator, time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, buildAgentScheduleExceptionResponse(item))
}

func AgentSelfPostLeaveCancel(ctx *gin.Context) {
	operator := services.AuthService.GetAuthPrincipal(ctx)
	if operator == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}
	req := request.CancelAgentScheduleLeaveRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.AgentScheduleExceptionService.CancelMyLeave(req.ID, operator, time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, buildAgentScheduleExceptionResponse(item))
}

func AgentSelfPostWorkStatus(ctx *gin.Context) {
	operator := services.AuthService.GetAuthPrincipal(ctx)
	if operator == nil {
		httpx.WriteJSON(ctx, errorsx.UnauthorizedI18n("error.auth.expired"))
		return
	}
	req := request.UpdateAgentWorkStatusRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.AgentWorkStatusService.UpdateMyStatus(req, operator, time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEngineerWorkStatusResponse(item))
}

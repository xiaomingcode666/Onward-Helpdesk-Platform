package dashboard

import (
	"fmt"
	"net/http"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"
	"time"

	"remotehelpdesk/internal/pkg/httpx/params"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/web"
)

const agentTeamScheduleDeprecatedWriteMessage = "产品组排班已改为成员可接单规则和请假；旧排班写入接口已下线"

func writeAgentTeamScheduleDeprecatedWrite(ctx *gin.Context, permission constants.Permission) {
	if _, err := services.AuthService.RequirePermission(ctx, permission); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteHttpStatusJSON(ctx, http.StatusGone, errorsx.InvalidParam(agentTeamScheduleDeprecatedWriteMessage))
}

func AgentTeamScheduleAnyLeaveList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentTeamScheduleView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	teamID, _ := params.GetInt64(ctx, "teamId")
	approvalStatus, _ := params.Get(ctx, "approvalStatus")
	items, err := services.AgentScheduleExceptionService.ListForTeam(operator.EffectiveTenantID(), teamID, approvalStatus, time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	results := make([]response.AgentScheduleExceptionResponse, 0, len(items))
	for i := range items {
		results = append(results, buildAgentScheduleExceptionResponse(&items[i]))
	}
	httpx.WriteJSON(ctx, results)
}

func AgentTeamScheduleAnyMemberAvailability(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentTeamScheduleView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	teamID, _ := params.GetInt64(ctx, "teamId")
	items, err := services.AgentTeamScheduleService.ListTeamMemberAvailability(teamID, operator, time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	results := make([]response.AgentTeamMemberAvailabilityResponse, 0, len(items))
	for _, item := range items {
		results = append(results, buildAgentTeamMemberAvailabilityResponse(item))
	}
	httpx.WriteJSON(ctx, results)
}

func AgentTeamSchedulePostLeaveReview(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentTeamScheduleUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.ReviewAgentScheduleLeaveRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item, err := services.AgentScheduleExceptionService.ReviewLeave(req, operator, time.Now())
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, buildAgentScheduleExceptionResponse(item))
}

func AgentTeamScheduleAnyList(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentTeamScheduleView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	cnd := params.NewPagedSqlCnd(ctx,
		params.QueryFilter{ParamName: "teamId"},
		params.QueryFilter{ParamName: "userId"},
		params.QueryFilter{ParamName: "repeatType"},
		params.QueryFilter{ParamName: "weekday"},
		params.QueryFilter{ParamName: "publishStatus"},
		params.QueryFilter{ParamName: "version"},
	).Desc("start_at").Desc("id")
	if _, ok := params.Get(ctx, "publishStatus"); !ok {
		cnd.Eq("publish_status", services.AgentTeamSchedulePublishPublished)
	}
	tenantID := operator.EffectiveTenantID()
	if tenantID > 0 {
		cnd.Eq("tenant_id", tenantID)
	}
	list, paging := services.AgentTeamScheduleService.FindPageByCnd(cnd)
	results := make([]response.AgentTeamScheduleResponse, 0, len(list))
	for _, item := range list {
		results = append(results, buildAgentTeamScheduleResponse(&item))
	}
	httpx.WriteJSON(ctx, &web.PageResult{Results: results, Page: paging})
}

func AgentTeamScheduleAnyCalendar(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentTeamScheduleView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	startAt, _ := params.Get(ctx, "startAt")
	endAt, _ := params.Get(ctx, "endAt")
	teamID, _ := params.GetInt64(ctx, "teamId")
	list, err := services.AgentTeamScheduleService.FindCalendarSchedules(request.AgentTeamScheduleCalendarRequest{
		TenantID: operator.EffectiveTenantID(),
		StartAt:  startAt,
		EndAt:    endAt,
		TeamID:   teamID,
	})
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	results := make([]response.AgentTeamScheduleResponse, 0, len(list))
	for _, item := range list {
		results = append(results, buildAgentTeamScheduleResponse(&item))
	}
	httpx.WriteJSON(ctx, results)
}

func AgentTeamScheduleGetTemplate(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentTeamScheduleView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	config, err := services.AgentTeamScheduleService.GetTemplate(operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, buildAgentTeamScheduleTemplateResponse(config))
}

func AgentTeamSchedulePostUpdate_template(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentTeamScheduleUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.UpdateAgentTeamScheduleTemplateRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	config, err := services.AgentTeamScheduleService.UpdateTemplate(req, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, buildAgentTeamScheduleTemplateResponse(config))
}

func AgentTeamSchedulePostBatch_preview(ctx *gin.Context) {
	writeAgentTeamScheduleDeprecatedWrite(ctx, constants.PermissionAgentTeamScheduleUpdate)
}

func AgentTeamSchedulePostBatch_generate(ctx *gin.Context) {
	writeAgentTeamScheduleDeprecatedWrite(ctx, constants.PermissionAgentTeamScheduleUpdate)
}

func AgentTeamScheduleGetBy(ctx *gin.Context) {
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionAgentTeamScheduleView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	item := services.AgentTeamScheduleService.Get(id)
	tenantID := operator.EffectiveTenantID()
	if item == nil || (tenantID > 0 && item.TenantID != tenantID) {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0172"))
		return
	}
	httpx.WriteJSON(ctx, buildAgentTeamScheduleResponse(item))
}

func AgentTeamSchedulePostCreate(ctx *gin.Context) {
	writeAgentTeamScheduleDeprecatedWrite(ctx, constants.PermissionAgentTeamScheduleUpdate)
}

func AgentTeamSchedulePostUpdate(ctx *gin.Context) {
	writeAgentTeamScheduleDeprecatedWrite(ctx, constants.PermissionAgentTeamScheduleUpdate)
}

func AgentTeamSchedulePostDelete(ctx *gin.Context) {
	writeAgentTeamScheduleDeprecatedWrite(ctx, constants.PermissionAgentTeamScheduleUpdate)
}

func AgentTeamSchedulePostPrepare_draft(ctx *gin.Context) {
	writeAgentTeamScheduleDeprecatedWrite(ctx, constants.PermissionAgentTeamScheduleUpdate)
}

func AgentTeamSchedulePostPublish(ctx *gin.Context) {
	writeAgentTeamScheduleDeprecatedWrite(ctx, constants.PermissionAgentTeamScheduleUpdate)
}

func AgentTeamSchedulePostRollback(ctx *gin.Context) {
	writeAgentTeamScheduleDeprecatedWrite(ctx, constants.PermissionAgentTeamScheduleUpdate)
}

func AgentTeamSchedulePostDisable(ctx *gin.Context) {
	writeAgentTeamScheduleDeprecatedWrite(ctx, constants.PermissionAgentTeamScheduleUpdate)
}

func buildAgentTeamMemberAvailabilityResponse(item services.AgentTeamMemberAvailability) response.AgentTeamMemberAvailabilityResponse {
	displayName, avatar := services.AgentProfileService.CurrentDisplayIdentity(item.Profile.TenantID, item.Profile.UserID)
	ret := response.AgentTeamMemberAvailabilityResponse{
		UserID:              item.Profile.UserID,
		MemberID:            item.Member.MemberID,
		ProfileID:           item.Profile.ID,
		AgentCode:           item.Profile.AgentCode,
		DisplayName:         displayName,
		Avatar:              item.Profile.Avatar,
		DispatchEnabled:     item.Member.DispatchEnabled,
		DispatchWeight:      item.Member.DispatchWeight,
		AutoAssignEnabled:   item.Profile.AutoAssignEnabled,
		ServiceStatus:       item.Profile.ServiceStatus,
		MaxConcurrentCount:  item.Profile.MaxConcurrentCount,
		WorkStatus:          services.AgentWorkStatusOffline,
		WorkStatusConfirmed: item.WorkStatusConfirmed,
		AvailableNow:        item.AvailableNow,
		UnavailableReason:   item.UnavailableReason,
		Workdays:            item.Workdays,
		StartTime:           formatScheduleMinute(item.StartMinute),
		EndTime:             formatScheduleMinute(item.EndMinute),
		Timezone:            item.Timezone,
	}
	if avatar != "" {
		ret.Avatar = avatar
	}
	if item.User != nil {
		ret.Username = item.User.Username
		ret.Nickname = item.User.Nickname
		if ret.DisplayName == "" {
			ret.DisplayName = item.User.Nickname
		}
		if ret.DisplayName == "" {
			ret.DisplayName = item.User.Username
		}
		if ret.Avatar == "" {
			ret.Avatar = item.User.Avatar
		}
	}
	if ret.DisplayName == "" {
		ret.DisplayName = item.Profile.DisplayName
	}
	if ret.DisplayName == "" {
		ret.DisplayName = item.Profile.AgentCode
	}
	if ret.DisplayName == "" {
		ret.DisplayName = fmt.Sprintf("成员 #%d", item.Profile.UserID)
	}
	if item.WorkStatus != nil {
		ret.WorkStatus = item.WorkStatus.Status
		ret.WorkStatusNote = item.WorkStatus.Note
	}
	if item.ActiveLeave != nil {
		active := buildAgentScheduleExceptionResponse(item.ActiveLeave)
		ret.ActiveLeave = &active
	}
	if item.PendingLeave != nil {
		pending := buildAgentScheduleExceptionResponse(item.PendingLeave)
		ret.PendingLeave = &pending
	}
	return ret
}

func buildAgentTeamScheduleResponse(item *models.AgentTeamSchedule) response.AgentTeamScheduleResponse {
	repeatType := services.NormalizeAgentTeamScheduleRepeatType(item.RepeatType)
	ret := response.AgentTeamScheduleResponse{
		ID:            item.ID,
		TenantID:      item.TenantID,
		TeamID:        item.TeamID,
		UserID:        item.UserID,
		RepeatType:    repeatType,
		DayType:       services.NormalizeAgentTeamScheduleDayType(item.DayType),
		Weekday:       item.Weekday,
		StartMinute:   item.StartMinute,
		EndMinute:     item.EndMinute,
		Timezone:      item.Timezone,
		PublishStatus: services.NormalizeAgentTeamSchedulePublishStatus(item.PublishStatus),
		Version:       item.Version,
		StartAt:       item.StartAt.Format("2006-01-02 15:04:05"),
		EndAt:         item.EndAt.Format("2006-01-02 15:04:05"),
		Remark:        item.Remark,
	}
	if item.EffectiveFrom != nil {
		ret.EffectiveFrom = item.EffectiveFrom.Format(time.DateOnly)
	}
	if item.EffectiveUntil != nil {
		ret.EffectiveUntil = item.EffectiveUntil.Format(time.DateOnly)
	}
	if repeatType == services.AgentTeamScheduleRepeatWeekly {
		ret.StartTime = formatScheduleMinute(item.StartMinute)
		ret.EndTime = formatScheduleMinute(item.EndMinute)
	}
	if team := services.AgentTeamService.Get(item.TeamID); team != nil {
		ret.TeamName = team.Name
	}
	if item.UserID > 0 {
		ret.UserName, ret.UserAvatar = services.AgentProfileService.CurrentDisplayIdentity(item.TenantID, item.UserID)
	}
	return ret
}

func buildAgentTeamScheduleTemplateResponse(config *services.AgentTeamScheduleTemplateConfig) response.AgentTeamScheduleTemplateResponse {
	if config == nil {
		return response.AgentTeamScheduleTemplateResponse{}
	}
	return response.AgentTeamScheduleTemplateResponse{
		TenantID:  config.TenantID,
		Workdays:  config.Workdays,
		StartTime: formatScheduleMinute(config.StartMinute),
		EndTime:   formatScheduleMinute(config.EndMinute),
		Timezone:  config.Timezone,
	}
}

func buildAgentScheduleExceptionResponse(item *models.AgentScheduleException) response.AgentScheduleExceptionResponse {
	if item == nil {
		return response.AgentScheduleExceptionResponse{}
	}
	location, err := time.LoadLocation(services.EngineerScheduleTimezone)
	if err != nil {
		location = time.FixedZone("CST", 8*60*60)
	}
	ret := response.AgentScheduleExceptionResponse{
		ID:               item.ID,
		TenantID:         item.TenantID,
		UserID:           item.UserID,
		RequestKey:       item.RequestKey,
		ExceptionType:    item.ExceptionType,
		StartAt:          item.StartAt.In(location).Format(time.RFC3339),
		EndAt:            item.EndAt.In(location).Format(time.RFC3339),
		ApprovalStatus:   item.ApprovalStatus,
		Reason:           item.Reason,
		ReviewNote:       item.ReviewNote,
		RequestedAt:      item.RequestedAt.In(location).Format(time.RFC3339),
		ReviewerUserID:   item.ReviewerUserID,
		ReviewerUserName: item.ReviewerUserName,
	}
	if item.ReviewedAt != nil {
		ret.ReviewedAt = item.ReviewedAt.In(location).Format(time.RFC3339)
	}
	if item.UserID > 0 {
		ret.UserName, ret.UserAvatar = services.AgentProfileService.CurrentDisplayIdentity(item.TenantID, item.UserID)
	}
	return ret
}

func formatScheduleMinute(value int) string {
	if value < 0 {
		value = 0
	}
	if value > 24*60 {
		value = 24 * 60
	}
	if value == 24*60 {
		return "24:00"
	}
	return fmt.Sprintf("%02d:%02d", value/60, value%60)
}

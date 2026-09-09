package enterprise

import (
	"strconv"

	"remotehelpdesk/internal/builders"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

// SlaPostPolicyCreate 创建 SLA 策略
func SlaPostPolicyCreate(ctx *gin.Context) {
	tenantID, ok := requireSLATenant(ctx, constants.PermissionTicketUpdate)
	if !ok {
		return
	}
	type req struct {
		Name              string `json:"name"`
		Priority          string `json:"priority"` // p0/p1/p2/p3/p4
		FRTMinutes        int    `json:"frtMinutes"`
		AssignmentMinutes int    `json:"assignmentMinutes"`
		ResolutionMinutes int    `json:"resolutionMinutes"`
		CalendarID        string `json:"calendarId"`
	}
	var r req
	if err := ctx.ShouldBindJSON(&r); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	policy, err := services.SLAService.CreateSLAPolicy(services.CreateSLAPolicyInput{
		TenantID:          tenantID,
		Name:              r.Name,
		Priority:          r.Priority,
		FRTMinutes:        r.FRTMinutes,
		AssignmentMinutes: r.AssignmentMinutes,
		ResolutionMinutes: r.ResolutionMinutes,
		CalendarID:        r.CalendarID,
	})
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseSLAPolicy(policy))
}

// SlaGetPolicies 获取 SLA 策略列表
func SlaGetPolicies(ctx *gin.Context) {
	tenantID, ok := requireSLATenant(ctx, constants.PermissionTicketView)
	if !ok {
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseSLAPolicies(services.SLAService.FindSLAPoliciesByTenant(tenantID)))
}

// SlaPostPause 暂停 SLA
func SlaPostPause(ctx *gin.Context) {
	tenantID, ok := requireSLATenant(ctx, constants.PermissionTicketChangeStatus)
	if !ok {
		return
	}
	type req struct {
		TicketID string `json:"ticketId"`
		Reason   string `json:"reason"` // waiting_customer/waiting_parts/scheduled/on_hold
	}
	var r req
	if err := ctx.ShouldBindJSON(&r); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	record, err := services.SLAService.PauseSLAForTenant(tenantID, r.TicketID, r.Reason)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, record)
}

// SlaPostResume 恢复 SLA
func SlaPostResume(ctx *gin.Context) {
	tenantID, ok := requireSLATenant(ctx, constants.PermissionTicketChangeStatus)
	if !ok {
		return
	}
	type req struct {
		TicketID string `json:"ticketId"`
	}
	var r req
	if err := ctx.ShouldBindJSON(&r); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	record, err := services.SLAService.ResumeSLAForTenant(tenantID, r.TicketID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, record)
}

// SlaGetTicketTimeline 获取工单 SLA 时间线
func SlaGetTicketTimeline(ctx *gin.Context) {
	tenantID, ok := requireSLATenant(ctx, constants.PermissionTicketView)
	if !ok {
		return
	}
	ticketID := ctx.Param("id")
	if ticketID == "" {
		httpx.WriteJSON(ctx, "ticket id is required")
		return
	}
	timeline, err := services.SLAService.GetSLATimelineForTenant(tenantID, ticketID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, timeline)
}

// SlaGetViolations 获取 SLA 违规列表
func SlaGetViolations(ctx *gin.Context) {
	tenantID, ok := requireSLATenant(ctx, constants.PermissionTicketView)
	if !ok {
		return
	}
	httpx.WriteJSON(ctx, services.SLAService.FindViolationsByTenant(tenantID))
}

// SlaPostEscalationRuleCreate 创建升级规则
func SlaPostEscalationRuleCreate(ctx *gin.Context) {
	tenantID, ok := requireSLATenant(ctx, constants.PermissionTicketUpdate)
	if !ok {
		return
	}
	type req struct {
		Name        string   `json:"name"`
		Priority    string   `json:"priority"`
		HoursIdle   int      `json:"hoursIdle"`
		TargetLevel int      `json:"targetLevel"`
		NotifyRoles []string `json:"notifyRoles"`
		AutoAssign  bool     `json:"autoAssign"`
	}
	var r req
	if err := ctx.ShouldBindJSON(&r); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	rule, err := services.EscalationService.CreateEscalationRule(services.CreateEscalationRuleInput{
		TenantID:    tenantID,
		Name:        r.Name,
		Priority:    r.Priority,
		HoursIdle:   r.HoursIdle,
		TargetLevel: r.TargetLevel,
		NotifyRoles: r.NotifyRoles,
		AutoAssign:  r.AutoAssign,
	})
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, rule)
}

// SlaGetEscalationRules 获取升级规则列表
func SlaGetEscalationRules(ctx *gin.Context) {
	tenantID, ok := requireSLATenant(ctx, constants.PermissionTicketView)
	if !ok {
		return
	}
	httpx.WriteJSON(ctx, services.EscalationService.FindEscalationRulesByTenant(tenantID))
}

// SlaPatchPolicyUpdate 更新 SLA 策略
func SlaPatchPolicyUpdate(ctx *gin.Context) {
	tenantID, ok := requireSLATenant(ctx, constants.PermissionTicketUpdate)
	if !ok {
		return
	}
	policyID := ctx.Param("id")
	if policyID == "" {
		httpx.WriteJSON(ctx, "policy id is required")
		return
	}
	type req struct {
		Name              *string `json:"name"`
		Priority          *string `json:"priority"`
		FRTMinutes        *int    `json:"frtMinutes"`
		AssignmentMinutes *int    `json:"assignmentMinutes"`
		ResolutionMinutes *int    `json:"resolutionMinutes"`
		CalendarID        *string `json:"calendarId"`
	}
	var r req
	if err := ctx.ShouldBindJSON(&r); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	policy, err := services.SLAService.UpdateSLAPolicyForTenant(tenantID, policyID, services.UpdateSLAPolicyInput{
		Name:              r.Name,
		Priority:          r.Priority,
		FRTMinutes:        r.FRTMinutes,
		AssignmentMinutes: r.AssignmentMinutes,
		ResolutionMinutes: r.ResolutionMinutes,
		CalendarID:        r.CalendarID,
	})
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseSLAPolicy(policy))
}

// SlaPatchPolicyToggle 切换 SLA 策略启用/停用状态
func SlaPatchPolicyToggle(ctx *gin.Context) {
	tenantID, ok := requireSLATenant(ctx, constants.PermissionTicketUpdate)
	if !ok {
		return
	}
	policyID := ctx.Param("id")
	if policyID == "" {
		httpx.WriteJSON(ctx, "policy id is required")
		return
	}
	policy, err := services.SLAService.ToggleSLAPolicyForTenant(tenantID, policyID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, builders.BuildEnterpriseSLAPolicy(policy))
}

func requireSLATenant(ctx *gin.Context, permission constants.Permission) (string, bool) {
	if _, err := services.AuthService.RequirePermission(ctx, permission); err != nil {
		httpx.WriteJSON(ctx, err)
		return "", false
	}
	tenantID, ok := resolveEnterpriseTenantIDInt(ctx)
	if !ok {
		return "", false
	}
	return strconv.FormatInt(tenantID, 10), true
}

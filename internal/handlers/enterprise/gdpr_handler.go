package enterprise

import (
	"strconv"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

// GdprPostDsarSubmit 提交 DSAR 请求
func GdprPostDsarSubmit(ctx *gin.Context) {
	tenantID, ok := requirePrivacyTenant(ctx, constants.PermissionPrivacyRequestManage)
	if !ok {
		return
	}
	type req struct {
		SubjectType  string `json:"subjectType"`
		SubjectID    string `json:"subjectId"`
		SubjectEmail string `json:"subjectEmail"`
		RequestType  string `json:"requestType"`
	}
	var r req
	if err := ctx.ShouldBindJSON(&r); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	dsar, err := services.DSARService.SubmitDSAR(ctx, tenantID, r.SubjectType, r.SubjectID, r.SubjectEmail, r.RequestType)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, dsar)
}

// GdprPostDsarVerify 身份验证
func GdprPostDsarVerify(ctx *gin.Context) {
	tenantID, ok := requirePrivacyTenant(ctx, constants.PermissionPrivacyRequestManage)
	if !ok {
		return
	}
	type req struct {
		VerificationMethod string `json:"verificationMethod"`
	}
	var r req
	if err := ctx.ShouldBindJSON(&r); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	dsarID := ctx.Param("id")
	if err := services.DSARService.VerifyIdentity(ctx, tenantID, dsarID, r.VerificationMethod); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, "identity verified")
}

// GdprGetDsarStatus 查询 DSAR 请求状态
func GdprGetDsarStatus(ctx *gin.Context) {
	tenantID, ok := requirePrivacyTenant(ctx, constants.PermissionPrivacyRequestView)
	if !ok {
		return
	}
	dsarID := ctx.Param("id")
	if dsarID == "" {
		httpx.WriteJSON(ctx, "dsar id is required")
		return
	}
	status, err := services.DSARService.GetDSARStatus(ctx, tenantID, dsarID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, status)
}

// GdprPostDsarExecute 执行 DSAR 请求（管理员）
func GdprPostDsarExecute(ctx *gin.Context) {
	tenantID, ok := requirePrivacyTenant(ctx, constants.PermissionPrivacyRequestManage)
	if !ok {
		return
	}
	type req struct {
		Action string `json:"action"` // export / delete / restrict
	}
	var r req
	if err := ctx.ShouldBindJSON(&r); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	dsarID := ctx.Param("id")
	if dsarID == "" {
		httpx.WriteJSON(ctx, "dsar id is required")
		return
	}

	var err error
	switch r.Action {
	case "export":
		data, filename, exportErr := services.DSARService.ExecuteDataExport(ctx, tenantID, dsarID)
		if exportErr != nil {
			httpx.WriteJSON(ctx, exportErr)
			return
		}
		ctx.Header("Content-Disposition", "attachment; filename="+filename)
		ctx.Data(200, "application/json", data)
		return
	case "delete":
		err = services.DSARService.ExecuteDataDeletion(ctx, tenantID, dsarID)
	case "restrict":
		err = services.DSARService.ExecuteDataRestriction(ctx, tenantID, dsarID)
	default:
		httpx.WriteJSON(ctx, "invalid action: must be export/delete/restrict")
		return
	}
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, "dsar action executed")
}

// GdprGetDsarList DSAR 请求列表（管理员）
func GdprGetDsarList(ctx *gin.Context) {
	tenantID, ok := requirePrivacyTenant(ctx, constants.PermissionPrivacyRequestView)
	if !ok {
		return
	}
	page := 1
	pageSize := 20
	requests, total, err := services.DSARService.ListDSARRequests(ctx, tenantID, page, pageSize)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, map[string]any{
		"list":  requests,
		"total": total,
	})
}

// GdprPostBreachReport 报告数据泄露
func GdprPostBreachReport(ctx *gin.Context) {
	tenantID, ok := requirePrivacyTenant(ctx, constants.PermissionDataBreachManage)
	if !ok {
		return
	}
	type req struct {
		BreachType             string `json:"breachType"`
		Description            string `json:"description"`
		AffectedDataCategories string `json:"affectedDataCategories"`
		AffectedUsersCount     int    `json:"affectedUsersCount"`
		RemediationSteps       string `json:"remediationSteps"`
	}
	var r req
	if err := ctx.ShouldBindJSON(&r); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	record := &models.DataBreachRecord{
		TenantID:               tenantID,
		BreachType:             r.BreachType,
		Description:            r.Description,
		AffectedDataCategories: r.AffectedDataCategories,
		AffectedUsersCount:     r.AffectedUsersCount,
		RemediationSteps:       r.RemediationSteps,
	}

	if err := services.DataBreachService.RecordDataBreach(ctx, record); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, record)
}

// GdprGetRetentionPolicy 查看数据保留策略
func GdprGetRetentionPolicy(ctx *gin.Context) {
	tenantID, ok := requirePrivacyTenant(ctx, constants.PermissionDataRetentionView)
	if !ok {
		return
	}
	policy, err := services.DataRetentionService.GetRetentionPolicy(ctx, tenantID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, policy)
}

// GdprGetBreachList 数据泄露记录列表
func GdprGetBreachList(ctx *gin.Context) {
	tenantID, ok := requirePrivacyTenant(ctx, constants.PermissionDataBreachView)
	if !ok {
		return
	}
	page := 1
	pageSize := 20
	breaches, total, err := services.DataBreachService.ListBreaches(ctx, tenantID, page, pageSize)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, map[string]any{
		"list":  breaches,
		"total": total,
	})
}

// GdprPostBreachNotifyAuthority 通知监管机构
func GdprPostBreachNotifyAuthority(ctx *gin.Context) {
	tenantID, ok := requirePrivacyTenant(ctx, constants.PermissionDataBreachManage)
	if !ok {
		return
	}
	breachID := ctx.Param("id")
	if breachID == "" {
		httpx.WriteJSON(ctx, "breach id is required")
		return
	}
	if err := services.DataBreachService.NotifySupervisoryAuthority(ctx, tenantID, breachID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, "supervisory authority notified")
}

// GdprPostBreachNotifyUsers 通知受影响用户
func GdprPostBreachNotifyUsers(ctx *gin.Context) {
	tenantID, ok := requirePrivacyTenant(ctx, constants.PermissionDataBreachManage)
	if !ok {
		return
	}
	breachID := ctx.Param("id")
	if breachID == "" {
		httpx.WriteJSON(ctx, "breach id is required")
		return
	}
	if err := services.DataBreachService.NotifyAffectedUsers(ctx, tenantID, breachID); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, "affected users notified")
}

// GdprGetBreachStatus 获取数据泄露通知状态
func GdprGetBreachStatus(ctx *gin.Context) {
	tenantID, ok := requirePrivacyTenant(ctx, constants.PermissionDataBreachView)
	if !ok {
		return
	}
	breachID := ctx.Param("id")
	if breachID == "" {
		httpx.WriteJSON(ctx, "breach id is required")
		return
	}
	status, err := services.DataBreachService.GetBreachNotificationStatus(ctx, tenantID, breachID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, status)
}

func requirePrivacyTenant(ctx *gin.Context, permission constants.Permission) (string, bool) {
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

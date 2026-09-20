package enterprise

import (
	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
	"net/http"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"
)

func TicketEmailHistory(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketView)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	_, id, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	history, err := services.GetTicketEmailHistory(id, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, history)
}
func TicketEmailReply(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketProgress)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	_, id, ok := resolveEnterpriseTicketRoute(ctx)
	if !ok {
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 128*1024)
	var req struct {
		Body       string `json:"body"`
		RequestKey string `json:"request_key"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("邮件内容无效"))
		return
	}
	row, err := services.QueueTicketEmailReply(id, req.Body, req.RequestKey, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	// Durable queued row is visible immediately; the worker performs network I/O.
	recordEnterpriseTicketAudit(ctx, operator, operator.TenantID, id, "ticket.email.queued", models.RiskLevelMedium, map[string]any{"email_reply_id": row.ID})
	httpx.WriteJSON(ctx, row)
}
func MailReceivePoll(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionNotificationChannelManage)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	n, err := services.PollTenantInboundMail(ctx.Request.Context(), operator.TenantID)
	if err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam(err.Error()))
		return
	}
	httpx.WriteJSON(ctx, map[string]any{"processed": n, "message": "连接成功；首次连接建立起点，随后到达的邮件会自动收取"})
}
func MailReceiveStatus(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionNotificationChannelManage)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var states []models.MailboxSyncState
	if sqls.DB().Where("tenant_id = ?", operator.TenantID).Order("id DESC").Limit(5).Find(&states).Error != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("收件状态读取失败"))
		return
	}
	var pending int64
	if sqls.DB().Model(&models.InboundEmail{}).Where("tenant_id = ? AND status = ?", operator.TenantID, services.InboundEmailPendingManual).Count(&pending).Error != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("收件状态读取失败"))
		return
	}
	type summary struct {
		Initialized bool   `json:"initialized"`
		LastError   string `json:"last_error"`
		LastUID     uint32 `json:"last_uid"`
	}
	rows := []summary{}
	for _, s := range states {
		rows = append(rows, summary{s.Initialized, s.LastError, s.LastUID})
	}
	httpx.WriteJSON(ctx, map[string]any{"mailboxes": rows, "pending_count": pending})
}

func MailPendingLinks(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionNotificationChannelManage)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	rows, err := services.ListPendingInboundEmailLinks(operator.TenantID)
	if err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("待人工关联邮件读取失败"))
		return
	}
	httpx.WriteJSON(ctx, rows)
}

func MailPendingLinkResolve(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionTicketProgress)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	inboundEmailID, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	var req struct {
		TicketID int64 `json:"ticket_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || req.TicketID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("请选择目标工单"))
		return
	}
	if err := services.ResolveInboundEmailManualLink(inboundEmailID, req.TicketID, operator); err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam(err.Error()))
		return
	}
	httpx.WriteJSON(ctx, map[string]any{"linked": true, "inbound_email_id": inboundEmailID, "ticket_id": req.TicketID})
}

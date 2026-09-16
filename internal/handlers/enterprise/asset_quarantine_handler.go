package enterprise

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"
)

type quarantineAssetView struct {
	ID              int64  `json:"id"`
	Filename        string `json:"filename"`
	FileSize        int64  `json:"fileSize"`
	Source          string `json:"source"`
	ScanStatus      string `json:"scanStatus"`
	ScanReason      string `json:"scanReason"`
	ReceiveComplete bool   `json:"receiveComplete"`
	CreatedAt       string `json:"createdAt"`
	UploadedBy      string `json:"uploadedBy"`
	SHA256          string `json:"sha256"`
}

func quarantineOperator(ctx *gin.Context) *dto.AuthPrincipal {
	op := services.AuthService.GetAuthPrincipal(ctx)
	if err := services.RequireAssetQuarantineOperator(op); err != nil {
		httpx.WriteJSON(ctx, err)
		return nil
	}
	return op
}

func AssetQuarantineList(ctx *gin.Context) {
	op := quarantineOperator(ctx)
	if op == nil {
		return
	}
	page := queryPositiveInt(ctx, "page", 1)
	limit := min(queryPositiveInt(ctx, "page_size", 20), 100)
	q := sqls.DB().Model(&models.Asset{}).Where("tenant_id = ? AND status <> ?", op.EffectiveTenantID(), enums.AssetStatusDeleted)
	status := ctx.DefaultQuery("status", models.AssetScanQuarantined)
	switch status {
	case "all":
	case models.AssetScanPending, models.AssetScanQuarantined, models.AssetScanClean:
		q = q.Where("scan_status = ?", status)
	case models.AssetScanUnscanned:
		q = q.Where("scan_status IN ?", []string{"", models.AssetScanUnscanned})
	default:
		httpx.WriteJSON(ctx, errorsx.InvalidParam("invalid attachment scan status"))
		return
	}
	if keyword := strings.TrimSpace(ctx.Query("search")); keyword != "" {
		q = q.Where("filename LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	var assets []models.Asset
	if err := q.Order("id DESC").Offset((page - 1) * limit).Limit(limit).Find(&assets).Error; err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	items := make([]quarantineAssetView, 0, len(assets))
	for _, a := range assets {
		items = append(items, quarantineAssetView{a.ID, a.Filename, a.FileSize, a.Source, a.ScanStatus, a.ScanReason, a.ReceiveComplete, a.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"), a.CreateUserName, a.SHA256})
	}
	httpx.WriteJSON(ctx, gin.H{"items": items, "total": total, "page": page, "pageSize": limit})
}

func AssetQuarantineAttempts(ctx *gin.Context) {
	op := quarantineOperator(ctx)
	if op == nil {
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	a := services.AssetService.Get(id)
	if a == nil || a.TenantID != op.EffectiveTenantID() {
		httpx.WriteJSON(ctx, errorsx.ForbiddenI18n("error.e0225"))
		return
	}
	before, _ := strconv.ParseInt(ctx.Query("before"), 10, 64)
	q := sqls.DB().Where("tenant_id = ? AND asset_id = ?", op.EffectiveTenantID(), id)
	if before > 0 {
		q = q.Where("id < ?", before)
	}
	var attempts []models.AssetScanAttempt
	if err := q.Order("id DESC").Limit(50).Find(&attempts).Error; err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, attempts)
}

func AssetQuarantineRescan(ctx *gin.Context) {
	op := quarantineOperator(ctx)
	if op == nil {
		return
	}
	id, ok := httpx.GetPathInt64(ctx, "id")
	if !ok {
		return
	}
	asset, err := services.AssetService.RescanAsset(id, op)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, gin.H{"id": asset.ID, "scanStatus": asset.ScanStatus})
}

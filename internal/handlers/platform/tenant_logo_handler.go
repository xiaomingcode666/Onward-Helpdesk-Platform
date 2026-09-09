package platform

import (
	"strings"

	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

const maxTenantLogoSize = 5 * 1024 * 1024

var allowedTenantLogoMIMETypes = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
}

func PostPlatformTenantLogoUpload(ctx *gin.Context) {
	operator, err := services.AuthService.RequireAnyPermission(ctx, constants.PermissionTenantCreate, constants.PermissionTenantUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	header, err := ctx.FormFile("file")
	if err != nil {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("tenant logo file is required"))
		return
	}
	if header.Size <= 0 || header.Size > maxTenantLogoSize {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("tenant logo must be no larger than 5 MB"))
		return
	}
	if _, ok := allowedTenantLogoMIMETypes[strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))]; !ok {
		httpx.WriteJSON(ctx, errorsx.InvalidParam("tenant logo must be a PNG, JPEG, or WebP image"))
		return
	}
	asset, err := services.AssetService.UploadFile(header, services.TenantBrandLogoStoragePrefix, operator)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if _, ok := allowedTenantLogoMIMETypes[strings.ToLower(strings.TrimSpace(asset.MimeType))]; !ok {
		_ = services.AssetService.DeleteAsset(asset.ID, operator)
		httpx.WriteJSON(ctx, errorsx.InvalidParam("tenant logo content is not a supported image"))
		return
	}
	previewURL, err := services.AssetService.GetSignedURL(asset.ID)
	if err != nil || strings.TrimSpace(previewURL) == "" {
		_ = services.AssetService.DeleteAsset(asset.ID, operator)
		if err == nil {
			err = errorsx.InvalidParam("tenant logo preview is unavailable")
		}
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, response.PlatformTenantLogoUploadResponse{
		AssetID:    asset.ID,
		URL:        services.TenantBrandLogoURL(asset.AssetID),
		PreviewURL: previewURL,
	})
}

package api

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/httpx"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
)

func ConversationMediaGetForCustomer(ctx *gin.Context) {
	asset, err := services.ConversationMediaService.AuthorizeForCustomer(ctx.Param("assetId"), httpx.GetExternalUser(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	streamConversationMedia(ctx, asset)
}

func ConversationMediaGetForOperator(ctx *gin.Context) {
	asset, err := services.ConversationMediaService.AuthorizeForOperator(ctx.Param("assetId"), services.AuthService.GetAuthPrincipal(ctx))
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	streamConversationMedia(ctx, asset)
}

var errInvalidMediaRange = errors.New("invalid media range")

func streamConversationMedia(ctx *gin.Context, asset *models.Asset) {
	streamAsset(ctx, asset, "private, max-age=300")
}

func streamPublicImmutableAsset(ctx *gin.Context, asset *models.Asset) {
	streamAsset(ctx, asset, "public, max-age=31536000, immutable")
}

func streamAsset(ctx *gin.Context, asset *models.Asset, cacheControl string) {
	etag := strconv.Quote(asset.AssetID)
	ctx.Header("Cache-Control", cacheControl)
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("Accept-Ranges", "bytes")
	ctx.Header("ETag", etag)
	if requestETagMatches(ctx.GetHeader("If-None-Match"), etag) {
		ctx.Status(http.StatusNotModified)
		ctx.Writer.WriteHeaderNow()
		return
	}
	start, length, partial, err := parseMediaRange(ctx.GetHeader("Range"), asset.FileSize)
	if err != nil {
		ctx.Header("Content-Range", "bytes */"+strconv.FormatInt(asset.FileSize, 10))
		ctx.AbortWithStatus(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	var reader io.ReadCloser
	if partial {
		reader, err = services.AssetService.OpenRange(asset, start, length)
	} else {
		reader, err = services.AssetService.OpenReader(asset)
	}
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	defer func() { _ = reader.Close() }()
	contentType := strings.TrimSpace(asset.MimeType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	disposition := mime.FormatMediaType("inline", map[string]string{"filename": asset.Filename})
	status := http.StatusOK
	if partial {
		status = http.StatusPartialContent
		ctx.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, start+length-1, asset.FileSize))
	}
	ctx.DataFromReader(status, length, contentType, reader, map[string]string{
		"Content-Disposition": disposition,
	})
}

func requestETagMatches(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == etag || strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}
	return false
}

func parseMediaRange(header string, size int64) (start, length int64, partial bool, err error) {
	if size < 0 {
		return 0, 0, false, errInvalidMediaRange
	}
	header = strings.TrimSpace(header)
	if header == "" {
		return 0, size, false, nil
	}
	if size == 0 || !strings.HasPrefix(strings.ToLower(header), "bytes=") || strings.Contains(header, ",") {
		return 0, 0, false, errInvalidMediaRange
	}
	raw := strings.TrimSpace(header[len("bytes="):])
	left, right, found := strings.Cut(raw, "-")
	if !found {
		return 0, 0, false, errInvalidMediaRange
	}
	if strings.TrimSpace(left) == "" {
		suffix, parseErr := strconv.ParseInt(strings.TrimSpace(right), 10, 64)
		if parseErr != nil || suffix <= 0 {
			return 0, 0, false, errInvalidMediaRange
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, suffix, true, nil
	}
	start, err = strconv.ParseInt(strings.TrimSpace(left), 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, false, errInvalidMediaRange
	}
	end := size - 1
	if strings.TrimSpace(right) != "" {
		end, err = strconv.ParseInt(strings.TrimSpace(right), 10, 64)
		if err != nil || end < start {
			return 0, 0, false, errInvalidMediaRange
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end - start + 1, true, nil
}

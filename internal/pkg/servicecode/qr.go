package servicecode

import (
	"net/url"
	"strings"

	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/errorsx"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	defaultQRCodeImageSize = 320
	minQRCodeImageSize     = 128
	maxQRCodeImageSize     = 1024
	maxServiceCodeLength   = 128
)

func Normalize(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func BuildEntryURL(serviceCode string) string {
	code := Normalize(serviceCode)
	if code == "" {
		return ""
	}
	return buildPublicAppURL("/mobile", url.Values{"serviceCode": []string{code}})
}

func BuildQRURL(serviceCode string) string {
	return BuildEntryURL(serviceCode)
}

func BuildQRImageURL(serviceCode string) string {
	code := Normalize(serviceCode)
	if code == "" {
		return ""
	}
	return buildPublicAppURL("/api/customer/service-code/qr-image", url.Values{"serviceCode": []string{code}})
}

func GenerateQRCodePNG(serviceCode string, size int) ([]byte, error) {
	code := Normalize(serviceCode)
	if code == "" {
		return nil, errorsx.InvalidParam("serviceCode is required")
	}
	if len(code) > maxServiceCodeLength {
		return nil, errorsx.InvalidParam("serviceCode is too long")
	}
	if size <= 0 {
		size = defaultQRCodeImageSize
	}
	if size < minQRCodeImageSize {
		size = minQRCodeImageSize
	}
	if size > maxQRCodeImageSize {
		size = maxQRCodeImageSize
	}
	return qrcode.Encode(BuildEntryURL(code), qrcode.Medium, size)
}

func buildPublicAppURL(path string, params url.Values) string {
	path = "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}
	baseURL := config.CurrentOrDefault().Public.NormalizedBaseURL()
	if baseURL == "" {
		return path
	}
	return baseURL + path
}

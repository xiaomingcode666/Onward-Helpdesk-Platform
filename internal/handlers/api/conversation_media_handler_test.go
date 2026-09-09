package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/gin-gonic/gin"
)

func TestParseMediaRange(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		size        int64
		wantStart   int64
		wantLength  int64
		wantPartial bool
		wantError   bool
	}{
		{name: "full response", size: 10, wantLength: 10},
		{name: "bounded", header: "bytes=2-5", size: 10, wantStart: 2, wantLength: 4, wantPartial: true},
		{name: "open ended", header: "bytes=7-", size: 10, wantStart: 7, wantLength: 3, wantPartial: true},
		{name: "suffix", header: "bytes=-3", size: 10, wantStart: 7, wantLength: 3, wantPartial: true},
		{name: "clamped suffix", header: "bytes=-30", size: 10, wantLength: 10, wantPartial: true},
		{name: "clamped end", header: "bytes=8-30", size: 10, wantStart: 8, wantLength: 2, wantPartial: true},
		{name: "multiple ranges", header: "bytes=0-1,3-4", size: 10, wantError: true},
		{name: "past end", header: "bytes=10-", size: 10, wantError: true},
		{name: "backwards", header: "bytes=8-2", size: 10, wantError: true},
		{name: "zero suffix", header: "bytes=-0", size: 10, wantError: true},
		{name: "range on empty file", header: "bytes=0-0", size: 0, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start, length, partial, err := parseMediaRange(test.header, test.size)
			if (err != nil) != test.wantError {
				t.Fatalf("parseMediaRange(%q, %d) error = %v, wantError=%v", test.header, test.size, err, test.wantError)
			}
			if err == nil && (start != test.wantStart || length != test.wantLength || partial != test.wantPartial) {
				t.Fatalf("parseMediaRange(%q, %d) = (%d, %d, %v), want (%d, %d, %v)",
					test.header, test.size, start, length, partial,
					test.wantStart, test.wantLength, test.wantPartial)
			}
		})
	}
}

func TestStreamConversationMediaSupportsByteRanges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	key := "video/demo.mp4"
	path := filepath.Join(root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("0123456789"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	previous := config.CurrentOrDefault()
	config.SetCurrent(&config.Config{Storage: config.StorageConfig{
		Default: enums.AssetProviderLocal,
		Local:   config.LocalStorageConfig{Root: root},
	}})
	t.Cleanup(func() { config.SetCurrent(&previous) })

	asset := &models.Asset{
		AssetID: "range-asset", Provider: enums.AssetProviderLocal, StorageKey: key,
		Filename: "demo.mp4", FileSize: 10, MimeType: "video/mp4", Status: enums.AssetStatusSuccess,
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/media/range-asset", nil)
	ctx.Request.Header.Set("Range", "bytes=3-6")

	streamConversationMedia(ctx, asset)

	if recorder.Code != http.StatusPartialContent || recorder.Body.String() != "3456" {
		t.Fatalf("partial response status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Range"); got != "bytes 3-6/10" {
		t.Fatalf("Content-Range=%q", got)
	}
	if got := recorder.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("Accept-Ranges=%q", got)
	}
}

func TestStreamConversationMediaServesFullAudioWithPlaybackHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	key := "audio/voice.webm"
	path := filepath.Join(root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	content := []byte("audio-data")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	previous := config.CurrentOrDefault()
	config.SetCurrent(&config.Config{Storage: config.StorageConfig{
		Default: enums.AssetProviderLocal,
		Local:   config.LocalStorageConfig{Root: root},
	}})
	t.Cleanup(func() { config.SetCurrent(&previous) })

	asset := &models.Asset{
		AssetID: "audio-asset", Provider: enums.AssetProviderLocal, StorageKey: key,
		Filename: "voice.webm", FileSize: int64(len(content)), MimeType: "audio/webm", Status: enums.AssetStatusSuccess,
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/media/audio-asset", nil)

	streamConversationMedia(ctx, asset)

	if recorder.Code != http.StatusOK || recorder.Body.String() != string(content) {
		t.Fatalf("full response status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "audio/webm" {
		t.Fatalf("Content-Type=%q", got)
	}
	if got := recorder.Header().Get("Content-Length"); got != "10" {
		t.Fatalf("Content-Length=%q", got)
	}
	if got := recorder.Header().Get("Content-Disposition"); got != `inline; filename=voice.webm` {
		t.Fatalf("Content-Disposition=%q", got)
	}
	if got := recorder.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("Accept-Ranges=%q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "private, max-age=300" {
		t.Fatalf("Cache-Control=%q", got)
	}
}

func TestStreamPublicImmutableAssetSupportsBrowserRevalidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	asset := &models.Asset{AssetID: "tenant-logo-asset", FileSize: 10}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/tenant-branding/logo/tenant-logo-asset", nil)
	ctx.Request.Header.Set("If-None-Match", `W/"tenant-logo-asset"`)

	streamPublicImmutableAsset(ctx, asset)

	if recorder.Code != http.StatusNotModified {
		t.Fatalf("status=%d want %d", recorder.Code, http.StatusNotModified)
	}
	if got := recorder.Header().Get("ETag"); got != `"tenant-logo-asset"` {
		t.Fatalf("ETag=%q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control=%q", got)
	}
}

func TestStreamConversationMediaRejectsInvalidRangeBeforeOpeningStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/media/missing", nil)
	ctx.Request.Header.Set("Range", "bytes=99-")

	streamConversationMedia(ctx, &models.Asset{AssetID: "missing", FileSize: 10})

	if recorder.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("status=%d want %d", recorder.Code, http.StatusRequestedRangeNotSatisfiable)
	}
	if got := recorder.Header().Get("Content-Range"); got != "bytes */10" {
		t.Fatalf("Content-Range=%q", got)
	}
}

package utils

import (
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestBuildIMMessageAssetPayloadForResponseHidesStorageDetails(t *testing.T) {
	config.SetCurrent(&config.Config{
		Storage: config.StorageConfig{
			Default: enums.AssetProviderLocal,
			Local: config.LocalStorageConfig{
				BaseURL: "https://files.example.com",
			},
		},
	})

	payload := `{"assetId":"asset_1","provider":"local","storageKey":"attachments/demo.png","filename":"demo.png"}`
	got := buildIMMessageAssetPayloadForResponse(payload)

	if !strings.Contains(got, `"assetId":"asset_1"`) || !strings.Contains(got, `"filename":"demo.png"`) {
		t.Fatalf("expected stable asset metadata in payload, got: %s", got)
	}
	if strings.Contains(got, `"provider"`) || strings.Contains(got, `"storageKey"`) || strings.Contains(got, `"url"`) {
		t.Fatalf("expected storage details hidden from payload, got: %s", got)
	}
}

func TestSanitizeMessageHTMLStripsStoredSrcForManagedImages(t *testing.T) {
	html := `<p><img src="https://files.example.com/demo.png" data-provider="local" data-storage-key="attachments/demo.png" alt="demo"></p>`

	got := SanitizeMessageHTML(html)

	if strings.Contains(got, `src=`) {
		t.Fatalf("expected src removed from stored html, got: %s", got)
	}
	if !strings.Contains(got, `data-provider="local"`) {
		t.Fatalf("expected data-provider kept, got: %s", got)
	}
	if !strings.Contains(got, `data-storage-key="attachments/demo.png"`) {
		t.Fatalf("expected data-storage-key kept, got: %s", got)
	}
}

func TestBuildMessageHTMLForResponseUsesAssetIDWithoutSignedURL(t *testing.T) {
	setupMessageTestDB(t)
	config.SetCurrent(&config.Config{
		Storage: config.StorageConfig{
			Default: enums.AssetProviderLocal,
			Local: config.LocalStorageConfig{
				BaseURL: "https://files.example.com",
			},
		},
	})

	createTestAsset(t, &models.Asset{
		AssetID:    "asset_html_1",
		Provider:   enums.AssetProviderLocal,
		StorageKey: "attachments/demo.png",
		Filename:   "demo.png",
		MimeType:   "image/png",
		Status:     enums.AssetStatusSuccess,
	})
	html := `<p><img data-asset-id="asset_html_1" data-provider="local" data-storage-key="attachments/demo.png" alt="demo"></p>`
	got := BuildMessageHTMLForResponse(html)

	if !strings.Contains(got, `data-asset-id="asset_html_1"`) {
		t.Fatalf("expected asset id in response html, got: %s", got)
	}
	if strings.Contains(got, `src=`) || strings.Contains(got, `data-provider`) || strings.Contains(got, `data-storage-key`) {
		t.Fatalf("expected storage details hidden from response html, got: %s", got)
	}
}

func TestBuildRuntimeMessageTextForHTML(t *testing.T) {
	got := BuildRuntimeMessageText(enums.IMMessageTypeHTML, `<p>你好</p><p><img data-provider="local" data-storage-key="images/demo.png" alt="demo"></p>`)
	if got != "你好 [图片]" {
		t.Fatalf("expected html converted to plain text summary, got: %q", got)
	}
}

func TestBuildRuntimeMessageTextForAssetMessages(t *testing.T) {
	if got := BuildRuntimeMessageText(enums.IMMessageTypeImage, "demo.png"); got != "[图片] demo.png" {
		t.Fatalf("unexpected image runtime text: %q", got)
	}
	if got := BuildRuntimeMessageText(enums.IMMessageTypeAudio, "voice.webm"); got != "[语音] voice.webm" {
		t.Fatalf("unexpected audio runtime text: %q", got)
	}
	if got := BuildRuntimeMessageText(enums.IMMessageTypeAttachment, "spec.pdf"); got != "[附件] spec.pdf" {
		t.Fatalf("unexpected attachment runtime text: %q", got)
	}
}

func TestBuildMarkdownSummaryRemovesFormattingAndKeepsMeaning(t *testing.T) {
	content := "根据知识库：\n\n1. **先做视频复核**\n2. 建议锁定参数权限"
	got := BuildMarkdownSummary(content)
	if strings.Contains(got, "**") || strings.Contains(got, "1.") {
		t.Fatalf("markdown summary still contains formatting: %q", got)
	}
	if !strings.Contains(got, "先做视频复核") || !strings.Contains(got, "建议锁定参数权限") {
		t.Fatalf("markdown summary lost content: %q", got)
	}
}

func TestBuildRenderableMessageTransformsPayloadAndHTML(t *testing.T) {
	config.SetCurrent(&config.Config{
		Storage: config.StorageConfig{
			Default: enums.AssetProviderLocal,
			Local: config.LocalStorageConfig{
				BaseURL: "https://files.example.com",
			},
		},
	})

	image := &models.Message{
		MessageType: enums.IMMessageTypeImage,
		Payload:     `{"assetId":"asset_1","provider":"local","storageKey":"attachments/demo.png","filename":"demo.png"}`,
	}
	_, imagePayload := BuildRenderableMessage(image)
	if !strings.Contains(imagePayload, `"assetId":"asset_1"`) || strings.Contains(imagePayload, `"url"`) || strings.Contains(imagePayload, `"storageKey"`) {
		t.Fatalf("expected image payload without storage url, got: %s", imagePayload)
	}
	audio := &models.Message{
		MessageType: enums.IMMessageTypeAudio,
		Payload:     `{"assetId":"asset_2","provider":"local","storageKey":"audio/voice.webm","filename":"voice.webm","durationSeconds":12}`,
	}
	_, audioPayload := BuildRenderableMessage(audio)
	if strings.Contains(audioPayload, `"url"`) || strings.Contains(audioPayload, `"storageKey"`) || !strings.Contains(audioPayload, `"durationSeconds":12`) {
		t.Fatalf("expected audio payload without storage url and with duration, got: %s", audioPayload)
	}

	htmlMsg := &models.Message{
		MessageType: enums.IMMessageTypeHTML,
		Content:     `<p><img data-asset-id="asset_1" data-provider="local" data-storage-key="attachments/demo.png"></p>`,
	}
	htmlContent, _ := BuildRenderableMessage(htmlMsg)
	if !strings.Contains(htmlContent, `data-asset-id="asset_1"`) || strings.Contains(htmlContent, `src=`) || strings.Contains(htmlContent, `data-storage-key`) {
		t.Fatalf("expected html content to use asset id only, got: %s", htmlContent)
	}
}

func TestNormalizeMessageHTMLAssetsKeepsValidAttrsAndRemovesSrc(t *testing.T) {
	setupMessageTestDB(t)
	config.SetCurrent(&config.Config{
		Storage: config.StorageConfig{
			Default: enums.AssetProviderLocal,
			Local: config.LocalStorageConfig{
				BaseURL: "https://files.example.com",
			},
		},
	})
	createTestAsset(t, &models.Asset{
		AssetID:    "asset_local_1",
		Provider:   enums.AssetProviderLocal,
		StorageKey: "images/demo.png",
		Filename:   "demo.png",
		FileSize:   123,
		MimeType:   "image/png",
		Status:     enums.AssetStatusSuccess,
	})

	got, err := NormalizeMessageHTMLAssets(`<p><img src="https://files.example.com/images/demo.png" data-asset-id="asset_local_1" data-provider="local" data-storage-key="images/demo.png" alt="demo"></p>`)
	if err != nil {
		t.Fatalf("expected normalization success, got error: %v", err)
	}

	if !strings.Contains(got, `data-asset-id="asset_local_1"`) {
		t.Fatalf("expected data-asset-id added, got: %s", got)
	}
	if !strings.Contains(got, `data-provider="local"`) {
		t.Fatalf("expected data-provider added, got: %s", got)
	}
	if !strings.Contains(got, `data-storage-key="images/demo.png"`) {
		t.Fatalf("expected data-storage-key added, got: %s", got)
	}
	if strings.Contains(got, `src=`) {
		t.Fatalf("expected src removed after asset binding, got: %s", got)
	}
}

func TestNormalizeMessageHTMLAssetsAcceptsAssetIDOnly(t *testing.T) {
	setupMessageTestDB(t)
	createTestAsset(t, &models.Asset{
		AssetID:    "asset_id_only",
		Provider:   enums.AssetProviderLocal,
		StorageKey: "images/id-only.png",
		Filename:   "id-only.png",
		FileSize:   321,
		MimeType:   "image/png",
		Status:     enums.AssetStatusSuccess,
	})

	got, err := NormalizeMessageHTMLAssets(`<p><img data-asset-id="asset_id_only" alt="demo"></p>`)
	if err != nil {
		t.Fatalf("expected asset-id-only image accepted, got error: %v", err)
	}
	if !strings.Contains(got, `data-provider="local"`) || !strings.Contains(got, `data-storage-key="images/id-only.png"`) {
		t.Fatalf("expected storage metadata canonicalized for persistence, got: %s", got)
	}
}

func TestExtractMessageHTMLAssetIDsDeduplicatesAssets(t *testing.T) {
	got := ExtractMessageHTMLAssetIDs(`<p><img data-asset-id="asset-1"><img data-asset-id="asset-1"><img data-asset-id="asset-2"></p>`)
	if len(got) != 2 || got[0] != "asset-1" || got[1] != "asset-2" {
		t.Fatalf("unexpected extracted asset IDs: %#v", got)
	}
}

func TestNormalizeMessageHTMLAssetsRejectsMissingAssetMetadata(t *testing.T) {
	setupMessageTestDB(t)
	config.SetCurrent(&config.Config{
		Storage: config.StorageConfig{
			Default: enums.AssetProviderLocal,
			Local: config.LocalStorageConfig{
				BaseURL: "https://files.example.com",
			},
		},
	})

	_, err := NormalizeMessageHTMLAssets(`<p><img src="https://unknown.example.com/demo.png" alt="demo"></p>`)
	if err == nil {
		t.Fatalf("expected missing image asset metadata rejected")
	}
}

func TestNormalizeMessageHTMLAssetsRejectsIncompleteAttrs(t *testing.T) {
	setupMessageTestDB(t)
	config.SetCurrent(&config.Config{
		Storage: config.StorageConfig{
			Default: enums.AssetProviderLocal,
			Local: config.LocalStorageConfig{
				BaseURL: "https://files.example.com",
			},
		},
	})

	_, err := NormalizeMessageHTMLAssets(`<p><img data-asset-id="asset1" data-provider="local" alt="demo"></p>`)
	if err == nil {
		t.Fatalf("expected incomplete asset attrs rejected")
	}
}

func TestNormalizeMessageHTMLAssetsRejectsMismatchedAttrs(t *testing.T) {
	setupMessageTestDB(t)
	config.SetCurrent(&config.Config{
		Storage: config.StorageConfig{
			Default: enums.AssetProviderLocal,
			Local: config.LocalStorageConfig{
				BaseURL: "https://files.example.com",
			},
		},
	})
	createTestAsset(t, &models.Asset{
		AssetID:    "asset_local_2",
		Provider:   enums.AssetProviderLocal,
		StorageKey: "images/real.png",
		Filename:   "real.png",
		FileSize:   456,
		MimeType:   "image/png",
		Status:     enums.AssetStatusSuccess,
	})

	_, err := NormalizeMessageHTMLAssets(`<p><img data-asset-id="asset_local_2" data-provider="local" data-storage-key="images/wrong.png" alt="demo"></p>`)
	if err == nil {
		t.Fatalf("expected mismatched asset attrs rejected")
	}
}

func setupMessageTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Asset{}); err != nil {
		t.Fatalf("auto migrate asset failed: %v", err)
	}
	sqls.SetDB(db)
}

func createTestAsset(t *testing.T, item *models.Asset) {
	t.Helper()
	now := time.Now()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = now
	}
	if err := sqls.DB().Create(item).Error; err != nil {
		t.Fatalf("create asset failed: %v", err)
	}
}

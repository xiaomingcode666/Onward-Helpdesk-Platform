package services

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/openidentity"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestConversationMediaOperatorAccessAndLegacyReference(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Asset{},
		&models.Conversation{},
		&models.Message{},
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.ConversationParticipant{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)

	storageRoot := t.TempDir()
	config.SetCurrent(&config.Config{Storage: config.StorageConfig{
		Default: enums.AssetProviderLocal,
		Local:   config.LocalStorageConfig{Root: storageRoot, BaseURL: "/storage"},
	}})
	t.Cleanup(func() { config.SetCurrent(&config.Config{}) })

	customer := models.Customer{Name: "media customer", Status: enums.StatusOk}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	identity := models.CustomerIdentity{
		CustomerID: customer.ID, ExternalSource: enums.ExternalSourceUser,
		ExternalID: "customer-media-41", Status: enums.StatusOk,
	}
	if err := db.Create(&identity).Error; err != nil {
		t.Fatalf("create customer identity: %v", err)
	}
	conversations := []models.Conversation{{TenantID: 41, CustomerID: customer.ID}, {TenantID: 42}}
	if err := db.Create(&conversations).Error; err != nil {
		t.Fatalf("create conversations: %v", err)
	}
	writeMediaTestFile(t, storageRoot, "images/current.png", "current")
	writeMediaTestFile(t, storageRoot, "audio/legacy.webm", "legacy")
	assets := []models.Asset{
		{
			TenantID: 41, ConversationID: conversations[0].ID, AssetID: "media-current",
			Provider: enums.AssetProviderLocal, StorageKey: "images/current.png", Filename: "current.png",
			FileSize: 7, MimeType: "image/png", Status: enums.AssetStatusSuccess,
		},
		{
			TenantID: 41, AssetID: "media-legacy", Provider: enums.AssetProviderLocal,
			StorageKey: "audio/legacy.webm", Filename: "legacy.webm", FileSize: 6,
			MimeType: "audio/webm", Status: enums.AssetStatusSuccess,
		},
		{
			TenantID: 41, AssetID: "media-near-match", Provider: enums.AssetProviderLocal,
			StorageKey: "audio/near.webm", Filename: "near.webm", FileSize: 4,
			MimeType: "audio/webm", Status: enums.AssetStatusSuccess,
		},
	}
	if err := db.Create(&assets).Error; err != nil {
		t.Fatalf("create assets: %v", err)
	}
	if err := db.Create(&models.Message{
		ConversationID: conversations[0].ID,
		ClientMsgID:    "legacy-exact-reference",
		MessageType:    enums.IMMessageTypeAudio,
		Content:        "audio/legacy.webm",
		Payload:        `{"assetId":"media-legacy"}`,
	}).Error; err != nil {
		t.Fatalf("create legacy message: %v", err)
	}
	if err := db.Create(&models.Message{
		ConversationID: conversations[0].ID,
		ClientMsgID:    "legacy-near-reference",
		MessageType:    enums.IMMessageTypeAudio,
		Payload:        `{"assetId":"media-near-match-suffix","storageKey":"audio/near.webm.backup"}`,
	}).Error; err != nil {
		t.Fatalf("create near-match message: %v", err)
	}

	operator := &dto.AuthPrincipal{TenantID: 41, DomainType: models.DomainTypeEnterprise}
	for _, assetID := range []string{"media-current", "media-legacy"} {
		asset, reader, err := ConversationMediaService.OpenForOperator(assetID, operator)
		if err != nil {
			t.Fatalf("open %s: %v", assetID, err)
		}
		data, readErr := io.ReadAll(reader)
		_ = reader.Close()
		if readErr != nil || len(data) == 0 || asset.AssetID != assetID {
			t.Fatalf("read %s asset=%+v data=%q err=%v", assetID, asset, data, readErr)
		}
	}
	if asset, err := ConversationMediaService.AuthorizeForOperator("media-near-match", operator); err == nil || asset != nil {
		t.Fatalf("near-match legacy reference unexpectedly authorized asset=%+v err=%v", asset, err)
	}
	customerAsset, customerReader, err := ConversationMediaService.OpenForCustomer("media-current", &openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     identity.ExternalID,
	})
	if err != nil || customerAsset == nil || customerReader == nil {
		t.Fatalf("customer open asset=%+v err=%v", customerAsset, err)
	}
	_ = customerReader.Close()
	if _, reader, err := ConversationMediaService.OpenForCustomer("media-current", &openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceUser,
		ExternalID:     "another-customer",
	}); err == nil {
		if reader != nil {
			_ = reader.Close()
		}
		t.Fatal("another customer unexpectedly opened conversation media")
	}

	if _, reader, err := ConversationMediaService.OpenForOperator("media-current", &dto.AuthPrincipal{
		TenantID: 42, DomainType: models.DomainTypeEnterprise,
	}); err == nil {
		if reader != nil {
			_ = reader.Close()
		}
		t.Fatal("cross-tenant operator unexpectedly opened conversation media")
	}
}

func writeMediaTestFile(t *testing.T, root, key, content string) {
	t.Helper()
	filename := filepath.Join(root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatalf("create media directory: %v", err)
	}
	if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
		t.Fatalf("write media fixture: %v", err)
	}
}

package services

import (
	"bytes"
	"io"
	"mime/multipart"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services/storage"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
)

var AssetService = newAssetService()

func newAssetService() *assetService {
	return &assetService{}
}

type assetService struct {
}

func (s *assetService) Get(id int64) *models.Asset {
	return repositories.AssetRepository.Get(sqls.DB(), id)
}

func (s *assetService) GetByAssetID(assetID string) *models.Asset {
	return repositories.AssetRepository.GetByAssetID(sqls.DB(), strings.TrimSpace(assetID))
}

func (s *assetService) GetByStorageKey(storageKey string) *models.Asset {
	return repositories.AssetRepository.GetByStorageKey(sqls.DB(), strings.TrimSpace(storageKey))
}

func (s *assetService) FindPageByCnd(cnd *sqls.Cnd) (list []models.Asset, paging *sqls.Paging) {
	return repositories.AssetRepository.FindPageByCnd(sqls.DB(), cnd)
}

func (s *assetService) OpenReader(asset *models.Asset) (io.ReadCloser, error) {
	cfg := config.Current()
	if asset == nil {
		return nil, errorsx.InvalidParamI18n("error.e0146")
	}
	switch asset.Provider {
	case "", enums.AssetProviderLocal:
		return storage.NewLocalStorage(cfg.Storage.Local).Read(asset.StorageKey)
	case enums.AssetProviderOSS:
		return storage.NewOSSStorage(cfg.Storage.OSS).Read(asset.StorageKey)
	case enums.AssetProviderMinIO:
		return storage.NewMinIOStorage(cfg.Storage.MinIO).Read(asset.StorageKey)
	default:
		return nil, errorsx.InvalidParamI18n("error.e0195")
	}
}

func (s *assetService) OpenRange(asset *models.Asset, offset, length int64) (io.ReadCloser, error) {
	if asset == nil {
		return nil, errorsx.InvalidParamI18n("error.e0146")
	}
	if offset < 0 || length <= 0 || offset > asset.FileSize || length > asset.FileSize-offset {
		return nil, errorsx.InvalidParam("invalid asset byte range")
	}
	provider, err := storage.NewProvider(asset.Provider)
	if err != nil {
		return nil, err
	}
	return provider.ReadRange(asset.StorageKey, offset, length)
}

func (s *assetService) UploadBytes(data []byte, prefix, filename string, principal *dto.AuthPrincipal) (*models.Asset, error) {
	return s.Upload(bytes.NewReader(data), storage.UploadInfo{
		Prefix:    prefix,
		Filename:  filename,
		FileSize:  int64(len(data)),
		Principal: principal,
	})
}

func (s *assetService) UploadFile(file *multipart.FileHeader, prefix string, principal *dto.AuthPrincipal) (*models.Asset, error) {
	return s.uploadFile(file, prefix, 0, principal)
}

func (s *assetService) UploadConversationFile(file *multipart.FileHeader, prefix string, conversationID int64, principal *dto.AuthPrincipal) (*models.Asset, error) {
	if conversationID <= 0 {
		return nil, errorsx.InvalidParamI18n("error.e0064")
	}
	conversation := ConversationService.Get(conversationID)
	if conversation == nil {
		return nil, errorsx.InvalidParamI18n("error.e0116")
	}
	if principal == nil || principalTenantID(principal) != conversation.TenantID {
		return nil, errorsx.Forbidden("message asset does not belong to the conversation tenant")
	}
	return s.uploadFile(file, prefix, conversationID, principal)
}

func (s *assetService) CloneConversationAsset(source *models.Asset, prefix string, conversationID int64, principal *dto.AuthPrincipal) (*models.Asset, error) {
	if source == nil || source.Status != enums.AssetStatusSuccess {
		return nil, errorsx.InvalidParamI18n("error.e0343")
	}
	if conversationID <= 0 || principal == nil {
		return nil, errorsx.Forbidden("conversation asset clone requires an authenticated target conversation")
	}
	target := ConversationService.Get(conversationID)
	if target == nil || target.TenantID != source.TenantID || principalTenantID(principal) != target.TenantID {
		return nil, errorsx.Forbidden("message asset does not belong to the target conversation tenant")
	}
	reader, err := s.OpenReader(source)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	return s.upload(reader, storage.UploadInfo{
		Prefix:    prefix,
		Filename:  source.Filename,
		FileSize:  source.FileSize,
		MimeType:  source.MimeType,
		Principal: principal,
	}, conversationID)
}

func (s *assetService) claimLegacyConversationAsset(asset *models.Asset, conversationID int64, messageType enums.IMMessageType, principal *dto.AuthPrincipal) (*models.Asset, error) {
	if asset == nil {
		return nil, errorsx.InvalidParamI18n("error.e0342")
	}
	if asset.Status != enums.AssetStatusSuccess {
		return nil, errorsx.InvalidParamI18n("error.e0343")
	}
	conversation := ConversationService.Get(conversationID)
	if conversation == nil {
		return nil, errorsx.InvalidParamI18n("error.e0116")
	}
	if principal == nil || principalTenantID(principal) != conversation.TenantID || asset.TenantID != conversation.TenantID {
		return nil, errorsx.Forbidden("message asset does not belong to the conversation tenant")
	}
	if err := validateMessageAssetType(asset, messageType); err != nil {
		return nil, err
	}
	if asset.ConversationID != 0 || len(exactConversationAssetReferences(asset)) > 0 {
		return asset, validateConversationAsset(asset, conversationID, messageType)
	}

	claimed, err := repositories.AssetRepository.AssignConversationIfUnassigned(
		sqls.DB(), asset.ID, conversationID, principal.UserID, principal.Username, time.Now(),
	)
	if err != nil {
		return nil, err
	}
	if claimed {
		asset.ConversationID = conversationID
		return asset, nil
	}

	// Another sender may have claimed the legacy asset concurrently. Reload it
	// and apply the normal ownership check instead of trusting stale state.
	asset = s.Get(asset.ID)
	if err := validateConversationAsset(asset, conversationID, messageType); err != nil {
		return nil, err
	}
	return asset, nil
}

func (s *assetService) uploadFile(file *multipart.FileHeader, prefix string, conversationID int64, principal *dto.AuthPrincipal) (*models.Asset, error) {
	if file == nil {
		return nil, errorsx.InvalidParamI18n("error.e0323")
	}

	cfg := config.Current()
	if file.Size > cfg.Storage.MaxUploadSizeBytes() {
		return nil, errorsx.InvalidParamI18n("error.e0079")
	}

	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = src.Close() }()

	return s.upload(src, storage.UploadInfo{
		Prefix:    prefix,
		Filename:  file.Filename,
		FileSize:  file.Size,
		MimeType:  file.Header.Get("Content-Type"),
		Principal: principal,
	}, conversationID)
}

func (s *assetService) Upload(reader io.Reader, info storage.UploadInfo) (*models.Asset, error) {
	return s.upload(reader, info, 0)
}

func (s *assetService) upload(reader io.Reader, info storage.UploadInfo, conversationID int64) (*models.Asset, error) {
	data, sanitizedInfo, err := inspectUpload(reader, info, config.Current().Storage)
	if err != nil {
		return nil, err
	}
	info = sanitizedInfo

	provider, err := storage.GetDefault()
	if err != nil {
		return nil, err
	}

	assetID, key := storage.GenerateStorageKey(info)
	item := &models.Asset{
		TenantID:       principalTenantID(info.Principal),
		ConversationID: conversationID,
		AssetID:        assetID,
		Provider:       provider.ProviderType(),
		StorageKey:     key,
		Filename:       info.Filename,
		FileSize:       info.FileSize,
		MimeType:       info.MimeType,
		Status:         enums.AssetStatusPending,
		AuditFields:    utils.BuildAuditFields(info.Principal),
	}
	if err := repositories.AssetRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}

	if _, err := provider.Upload(bytes.NewReader(data), key, storage.UploadInfo{
		Prefix:    info.Prefix,
		Filename:  info.Filename,
		FileSize:  info.FileSize,
		MimeType:  info.MimeType,
		Principal: info.Principal,
	}); err != nil {
		_ = s.markAssetStatus(item.ID, enums.AssetStatusFailed, info.Principal)
		return nil, err
	}

	item.Status = enums.AssetStatusSuccess
	_ = repositories.AssetRepository.UpdateColumn(sqls.DB(), item.ID, "status", enums.AssetStatusSuccess)

	return item, nil
}

func (s *assetService) GetSignedURL(id int64) (string, error) {
	item := s.Get(id)
	if item == nil {
		return "", errorsx.InvalidParamI18n("error.e0214")
	}
	if item.Status != enums.AssetStatusSuccess {
		return "", errorsx.InvalidParamI18n("error.e0213")
	}

	provider, err := storage.NewProvider(item.Provider)
	if err != nil {
		return "", err
	}
	accessURL := provider.GetSignedURL(item.StorageKey)
	return accessURL, nil
}

func (s *assetService) DeleteAsset(id int64, principal *dto.AuthPrincipal) error {
	if principal == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	item := s.Get(id)
	if item == nil {
		return errorsx.InvalidParamI18n("error.e0214")
	}
	if !(principal.IsPlatform() && principal.EffectiveTenantID() == 0) && item.TenantID != principal.EffectiveTenantID() {
		return errorsx.Forbidden("asset does not belong to the current tenant")
	}
	return repositories.AssetRepository.Updates(sqls.DB(), id, map[string]any{
		"status":           enums.AssetStatusDeleted,
		"update_user_id":   principal.UserID,
		"update_user_name": principal.Username,
		"updated_at":       time.Now(),
	})
}

func (s *assetService) discardConversationAsset(item *models.Asset, principal *dto.AuthPrincipal) {
	if item == nil || principal == nil || item.TenantID != principalTenantID(principal) {
		return
	}
	if provider, err := storage.NewProvider(item.Provider); err == nil {
		_ = provider.Delete(item.StorageKey)
	}
	_ = s.markAssetStatus(item.ID, enums.AssetStatusDeleted, principal)
}

func (s *assetService) markAssetStatus(id int64, status enums.AssetStatus, principal *dto.AuthPrincipal) error {
	updates := map[string]any{
		"status":     status,
		"updated_at": time.Now(),
	}
	if principal != nil {
		updates["update_user_id"] = principal.UserID
		updates["update_user_name"] = principal.Username
	}
	return repositories.AssetRepository.Updates(sqls.DB(), id, updates)
}

func (s *assetService) buildFilenameFromMime(mimeType string) string {
	mimeType = strings.TrimSpace(strings.Split(mimeType, ";")[0])
	ext := ".bin"
	switch mimeType {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/gif":
		ext = ".gif"
	case "image/webp":
		ext = ".webp"
	case "application/pdf":
		ext = ".pdf"
	case "text/plain":
		ext = ".txt"
	}
	return "wxwork_" + strings.ReplaceAll(uuid.NewString(), "-", "") + ext
}

func principalTenantID(principal *dto.AuthPrincipal) int64 {
	if principal == nil {
		return 0
	}
	return principal.EffectiveTenantID()
}

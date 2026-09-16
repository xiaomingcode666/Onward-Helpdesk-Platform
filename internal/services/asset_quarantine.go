package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services/storage"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

var quarantineReceiveMu sync.Mutex

func knowledgeDocumentAssetUsable(doc *models.KnowledgeDocument) bool {
	if doc == nil {
		return false
	}
	if doc.SourceAssetID <= 0 {
		return true
	}
	asset := AssetService.Get(doc.SourceAssetID)
	return asset.Usable() && asset.TenantID == doc.TenantID
}

func requireScannedAsset(asset *models.Asset) error {
	if !asset.Usable() {
		return errorsx.InvalidParamI18n("error.upload.notScanned")
	}
	return nil
}

func RequireAssetQuarantineOperator(op *dto.AuthPrincipal) error {
	if op == nil {
		return errorsx.UnauthorizedI18n("error.auth.expired")
	}
	if !op.IsEnterprise() && !op.IsPlatform() {
		return errorsx.ForbiddenI18n("error.e0225")
	}
	if op.HasRole(EnterpriseRoleOwner) || op.HasRole(EnterpriseRoleAdmin) || op.HasRole(PlatformRoleAdmin) || op.HasPermission(constants.PermissionTenantUpdate.Code) {
		return nil
	}
	return errorsx.ForbiddenI18n("error.e0225")
}

// RecordRejectedAttachment records a request stopped before multipart parsing could yield a file.
// It deliberately does not claim that an original file was received or retained.
func (s *assetService) RecordRejectedAttachment(op *dto.AuthPrincipal, source string) error {
	if op == nil {
		return errors.New("rejected upload has no authenticated principal")
	}
	now := time.Now().UTC()
	id := uuid.NewString()
	item := &models.Asset{TenantID: op.EffectiveTenantID(), AssetID: id, StorageKey: "unreceived/" + id, Filename: "unreceived-attachment",
		Source: source, ScanStatus: models.AssetScanPending, ScanToken: id, ScanStartedAt: &now, Status: enums.AssetStatusPending, AuditFields: utils.BuildAuditFields(op)}
	item.CreatedAt, item.UpdatedAt = now, now
	if err := sqls.DB().Create(item).Error; err != nil {
		return err
	}
	err := s.finishAssetScan(item, assetScanVerdict{Reason: ScanLimitExceeded, Detail: "HTTP reception limit exceeded before file parsing; no complete original retained"}, op)
	var q *errorsx.AssetQuarantinedError
	if errors.As(err, &q) {
		return nil
	}
	return err
}

// quarantinePath never accepts a client filename or exposes the quarantine directory through storage URLs.
func quarantinePath(cfg config.StorageConfig, key string) (string, error) {
	if key == "" || filepath.Base(key) != key || strings.ContainsAny(key, "/\\:") {
		return "", errors.New("invalid quarantine key")
	}
	root := strings.TrimSpace(cfg.QuarantineRoot)
	if root == "" {
		local := strings.TrimSpace(cfg.Local.Root)
		if local == "" {
			local = "data/storage"
		}
		root = filepath.Clean(local) + ".quarantine"
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	localRoot := cfg.Local.Root
	if strings.TrimSpace(localRoot) == "" {
		localRoot = "data/storage"
	}
	local, err := filepath.Abs(localRoot)
	if err != nil {
		return "", err
	}
	if rel, e := filepath.Rel(local, root); e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("quarantine must be outside public storage")
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	if resolvedLocal, e := filepath.EvalSymlinks(local); e == nil {
		if rel, e := filepath.Rel(resolvedLocal, resolved); e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", errors.New("quarantine resolves into public storage")
		}
	}
	return filepath.Join(resolved, key), nil
}

func (s *assetService) receiveAndScan(reader io.Reader, info storage.UploadInfo, conversationID int64) (*models.Asset, error) {
	if info.Principal == nil || (info.Principal.EffectiveTenantID() <= 0 && !info.Principal.IsPlatform()) {
		return nil, errorsx.Forbidden("attachment reception requires an explicit tenant or platform principal")
	}
	if reader == nil {
		return nil, errorsx.InvalidParamI18n("error.upload.empty")
	}
	cfg := config.Current().Storage
	filename, filenameErr := sanitizeUploadFilename(info.Filename)
	if len([]rune(filename)) > 200 {
		filenameErr = errorsx.InvalidParamI18n("error.upload.invalidFilename")
	}
	if filenameErr != nil {
		filename = "upload"
	}
	info.Filename = string([]rune(filename)[:min(len([]rune(filename)), 200)])
	assetID, key := storage.GenerateStorageKey(info)
	source := strings.TrimSpace(info.Source)
	if source == "" {
		source = strings.TrimSpace(info.Prefix)
	}
	if source == "" {
		source = "upload"
	}
	now := time.Now().UTC()
	item := &models.Asset{TenantID: info.Principal.EffectiveTenantID(), ConversationID: conversationID, AssetID: assetID,
		Provider: cfg.Default, StorageKey: key, Filename: info.Filename, MimeType: string([]rune(info.MimeType)[:min(len([]rune(info.MimeType)), 100)]),
		Status: enums.AssetStatusPending, ScanStatus: models.AssetScanPending, ScanToken: uuid.NewString(), ScanStartedAt: &now,
		QuarantineKey: assetID + ".bin", Source: string([]rune(source)[:min(len([]rune(source)), 100)]), AuditFields: utils.BuildAuditFields(info.Principal)}
	item.CreatedAt, item.UpdatedAt = now, now
	if err := repositories.AssetRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	verdict := s.receivePrivate(item, reader, info.FileSize, cfg)
	if err := s.saveReceivedMetadata(item); err != nil {
		return nil, err
	}
	if verdict.Reason != "" {
		return nil, s.finishAssetScan(item, verdict, info.Principal)
	}
	if filenameErr != nil {
		return nil, s.finishAssetScan(item, assetScanVerdict{Reason: ScanPolicyBlocked, Detail: "Invalid filename"}, info.Principal)
	}
	return s.scanAndPublish(item, info.Principal)
}

func (s *assetService) saveReceivedMetadata(item *models.Asset) error {
	result := sqls.DB().Model(&models.Asset{}).Where("id = ? AND scan_token = ? AND scan_status = ?", item.ID, item.ScanToken, models.AssetScanPending).
		Updates(map[string]any{"file_size": item.FileSize, "sha256": item.SHA256, "receive_complete": item.ReceiveComplete, "quarantine_key": item.QuarantineKey})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("attachment reception lease expired")
	}
	return nil
}

func (s *assetService) receivePrivate(item *models.Asset, reader io.Reader, expected int64, cfg config.StorageConfig) assetScanVerdict {
	quarantineReceiveMu.Lock()
	defer quarantineReceiveMu.Unlock()
	fail := func(reason, detail string) assetScanVerdict { return assetScanVerdict{Reason: reason, Detail: detail} }
	filePath, err := quarantinePath(cfg, item.QuarantineKey)
	if err != nil {
		return fail(ScanFailed, "Private quarantine storage is unavailable")
	}
	entries, err := os.ReadDir(filepath.Dir(filePath))
	if err != nil {
		return fail(ScanFailed, "Cannot check quarantine capacity")
	}
	var used int64
	for _, e := range entries {
		if stat, err := e.Info(); err == nil && stat.Mode().IsRegular() {
			used += stat.Size()
		}
	}
	budget := min(cfg.MaxReceiveSizeBytes()+1, cfg.QuarantineCapacity()-used)
	if budget <= 0 {
		return fail(ScanFailed, "Quarantine storage is full; file not received")
	}
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fail(ScanFailed, "Cannot create private quarantine file")
	}
	hash := sha256.New()
	n, readErr := io.CopyN(io.MultiWriter(f, hash), reader, budget)
	syncErr := f.Sync()
	closeErr := f.Close()
	item.FileSize = n
	item.SHA256 = hex.EncodeToString(hash.Sum(nil))
	item.ReceiveComplete = readErr == io.EOF && (expected <= 0 || expected == n) && syncErr == nil && closeErr == nil
	if n > cfg.MaxReceiveSizeBytes() {
		item.ReceiveComplete = false
		return fail(ScanLimitExceeded, "Reception hard limit exceeded; partial file retained")
	}
	if !item.ReceiveComplete {
		return fail(ScanFailed, "Attachment reception incomplete or quarantine storage full; cannot release")
	}
	if n > cfg.MaxUploadSizeBytes() {
		return fail(ScanLimitExceeded, "Attachment size exceeds the upload limit")
	}
	if n == 0 {
		return fail(ScanPolicyBlocked, "Empty attachment")
	}
	return assetScanVerdict{}
}

func (s *assetService) scanAndPublish(item *models.Asset, op *dto.AuthPrincipal) (*models.Asset, error) {
	cfg := config.Current().Storage
	fail := func(v assetScanVerdict) (*models.Asset, error) { return nil, s.finishAssetScan(item, v, op) }
	if !item.ReceiveComplete {
		return fail(assetScanVerdict{Reason: ScanFailed, Detail: "Incomplete reception cannot be rescanned"})
	}
	if item.FileSize > cfg.MaxUploadSizeBytes() {
		return fail(assetScanVerdict{Reason: ScanLimitExceeded, Detail: "Attachment size exceeds the upload limit"})
	}
	p, err := quarantinePath(cfg, item.QuarantineKey)
	if err != nil {
		return fail(assetScanVerdict{Reason: ScanFailed, Detail: "Quarantine file unavailable"})
	}
	f, err := os.Open(p)
	if err != nil {
		return fail(assetScanVerdict{Reason: ScanFailed, Detail: "Quarantine file unavailable"})
	}
	data, readErr := io.ReadAll(io.LimitReader(f, cfg.MaxUploadSizeBytes()+1))
	_ = f.Close()
	digest := sha256.Sum256(data)
	if readErr != nil || int64(len(data)) != item.FileSize || hex.EncodeToString(digest[:]) != item.SHA256 {
		return fail(assetScanVerdict{Reason: ScanFailed, Detail: "Quarantine file is incomplete or changed"})
	}
	v := scanAttachmentWithClamAV(data, cfg.UploadSecurity.ClamAV)
	if v.Reason != "" {
		return fail(v)
	}
	if !cfg.UploadSecurity.EnabledOrDefault() {
		v.Reason = ScanFailed
		v.Detail = "Upload security checks are disabled"
		return fail(v)
	}
	_, info, inspectErr := inspectUpload(bytes.NewReader(data), storage.UploadInfo{Filename: item.Filename, MimeType: item.MimeType}, cfg)
	if inspectErr != nil {
		v.Reason, v.Detail = ScanPolicyBlocked, "Attachment violates file security policy"
		var archiveErr *uploadInspectionError
		if errors.As(inspectErr, &archiveErr) {
			v.Reason, v.Detail = archiveErr.reason, archiveErr.detail
		}
		return fail(v)
	}
	item.MimeType = info.MimeType
	if (item.Provider == enums.AssetProviderMinIO && !cfg.MinIO.Private) || (item.Provider == enums.AssetProviderOSS && !cfg.OSS.Private) {
		v.Reason, v.Detail = ScanFailed, "Attachment object storage must be private"
		return fail(v)
	}
	provider, err := storage.NewProvider(item.Provider)
	if err != nil {
		v.Reason, v.Detail = ScanFailed, "Attachment storage unavailable"
		return fail(v)
	}
	if _, err = provider.Upload(bytes.NewReader(data), item.StorageKey, info); err != nil {
		_ = provider.Delete(item.StorageKey)
		v.Reason, v.Detail = ScanFailed, "Scanned attachment could not be published"
		return fail(v)
	}
	if err = s.finishAssetScan(item, v, op); err != nil {
		_ = provider.Delete(item.StorageKey)
		return nil, err
	}
	// The published bytes are exactly the buffer scanned above. Only clean assets lose their private copy.
	if err = os.Remove(p); err != nil {
		slog.Warn("clean attachment staging cleanup failed", "asset_id", item.ID)
	}
	return item, nil
}

func (s *assetService) finishAssetScan(item *models.Asset, v assetScanVerdict, op *dto.AuthPrincipal) error {
	if v.Reason == "" && (!v.Performed || v.EngineVersion == "" || v.DatabaseVersion == "") {
		v.Reason, v.Detail = ScanFailed, "A completed antivirus verdict is required"
	}
	status, uploadStatus := models.AssetScanClean, enums.AssetStatusSuccess
	if v.Reason != "" {
		status, uploadStatus = models.AssetScanQuarantined, enums.AssetStatusFailed
	}
	attempt := models.AssetScanAttempt{TenantID: item.TenantID, AssetID: item.ID, Status: status, Reason: v.Reason,
		Detail: string([]rune(v.Detail)[:min(len([]rune(v.Detail)), 500)]), SHA256: item.SHA256, EngineVersion: v.EngineVersion, DatabaseVersion: v.DatabaseVersion, ScanPerformed: v.Performed, CreatedAt: time.Now().UTC()}
	if op != nil {
		attempt.OperatorID = op.UserID
	}
	err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"scan_status": status, "scan_reason": v.Reason, "status": uploadStatus, "mime_type": item.MimeType, "scan_token": "", "updated_at": attempt.CreatedAt}
		if status == models.AssetScanClean {
			updates["quarantine_key"] = ""
		}
		result := tx.Model(&models.Asset{}).Where("id = ? AND scan_token = ? AND scan_status = ? AND status <> ?", item.ID, item.ScanToken, models.AssetScanPending, enums.AssetStatusDeleted).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("attachment scan lease expired")
		}
		return tx.Create(&attempt).Error
	})
	if err != nil {
		return err
	}
	item.ScanStatus, item.ScanReason, item.Status = status, v.Reason, uploadStatus
	if v.Reason != "" {
		return &errorsx.AssetQuarantinedError{AssetID: item.ID, Reason: v.Reason}
	}
	item.QuarantineKey = ""
	return nil
}

func (s *assetService) RescanAsset(id int64, op *dto.AuthPrincipal) (*models.Asset, error) {
	if err := RequireAssetQuarantineOperator(op); err != nil {
		return nil, err
	}
	item := s.Get(id)
	if item == nil || item.TenantID != op.EffectiveTenantID() {
		return nil, errorsx.ForbiddenI18n("error.e0225")
	}
	if item.Status == enums.AssetStatusDeleted {
		return nil, errorsx.InvalidParamI18n("error.upload.notScanned")
	}
	now := time.Now().UTC()
	item.ScanToken = uuid.NewString()
	result := sqls.DB().Model(&models.Asset{}).Where("id = ? AND tenant_id = ? AND scan_status IN ? AND status <> ?", id, item.TenantID, []string{"", models.AssetScanUnscanned, models.AssetScanQuarantined}, enums.AssetStatusDeleted).
		Updates(map[string]any{"scan_status": models.AssetScanPending, "scan_token": item.ScanToken, "scan_started_at": now, "updated_at": now})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, errorsx.InvalidParamI18n("error.upload.scanBusy")
	}
	item.ScanStatus = models.AssetScanPending
	if item.QuarantineKey == "" {
		// Legacy files are copied through bounded private reception before any scan/parse operation.
		reader, err := s.openStoredReader(item)
		if err != nil {
			return nil, s.finishAssetScan(item, assetScanVerdict{Reason: ScanFailed, Detail: "Legacy attachment cannot be read"}, op)
		}
		item.QuarantineKey = item.AssetID + "-" + item.ScanToken + ".bin"
		v := s.receivePrivate(item, reader, item.FileSize, config.Current().Storage)
		_ = reader.Close()
		if err = s.saveReceivedMetadata(item); err != nil {
			return nil, err
		}
		if item.ReceiveComplete {
			provider, providerErr := storage.NewProvider(item.Provider)
			if providerErr != nil {
				return nil, s.finishAssetScan(item, assetScanVerdict{Reason: ScanFailed, Detail: "Legacy storage unavailable"}, op)
			}
			if err = provider.Delete(item.StorageKey); err != nil {
				return nil, s.finishAssetScan(item, assetScanVerdict{Reason: ScanFailed, Detail: "Legacy public copy could not be withdrawn"}, op)
			}
		}
		if v.Reason != "" {
			return nil, s.finishAssetScan(item, v, op)
		}
	}
	return s.scanAndPublish(item, op)
}

func (s *assetService) RecoverInterruptedScans(before time.Time) error {
	var items []models.Asset
	if err := sqls.DB().Where("scan_status = ? AND scan_started_at < ?", models.AssetScanPending, before).Limit(100).Find(&items).Error; err != nil {
		return err
	}
	for i := range items {
		err := s.finishAssetScan(&items[i], assetScanVerdict{Reason: ScanFailed, Detail: "Attachment scan interrupted; rescan required"}, nil)
		var isolated *errorsx.AssetQuarantinedError
		if err != nil && !errors.As(err, &isolated) {
			return err
		}
	}
	return nil
}

func StartAssetScanRecovery(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := AssetService.RecoverInterruptedScans(time.Now().UTC().Add(-15 * time.Minute)); err != nil {
					slog.Error("attachment scan recovery failed", "error", fmt.Sprint(err))
				}
			}
		}
	}()
}

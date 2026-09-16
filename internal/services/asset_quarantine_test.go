package services

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"mime/multipart"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/services/storage"
)

func setupAssetQuarantineTest(t *testing.T) (*gorm.DB, *dto.AuthPrincipal) {
	t.Helper()
	oldCfg := config.CurrentOrDefault()
	oldDB := sqls.DB()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "assets.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Asset{}, &models.AssetScanAttempt{}, &models.Conversation{}))
	sqls.SetDB(db)
	root := t.TempDir()
	config.SetCurrent(&config.Config{Storage: config.StorageConfig{Default: enums.AssetProviderLocal, MaxUploadSizeMB: 1, MaxReceiveSizeMB: 2,
		Local: config.LocalStorageConfig{Root: filepath.Join(root, "public"), BaseURL: "/storage"}, QuarantineRoot: filepath.Join(root, "quarantine")}})
	t.Cleanup(func() { config.SetCurrent(&oldCfg); sqls.SetDB(oldDB); conn, _ := db.DB(); _ = conn.Close() })
	return db, &dto.AuthPrincipal{TenantID: 14, UserID: 4, Username: "scan-test", DomainType: "enterprise", Roles: []string{EnterpriseRoleAdmin}}
}

func fakeAssetClamAV(t *testing.T, reply *atomic.Value) string {
	t.Helper()
	return fakeAssetClamAVVersion(t, reply, "ClamAV 1.4.3/99999/"+time.Now().UTC().Format("Mon Jan _2 15:04:05 2006"))
}

func fakeAssetClamAVVersion(t *testing.T, reply *atomic.Value, version string) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				r := bufio.NewReader(conn)
				command, err := r.ReadString(0)
				if err != nil {
					return
				}
				if command == "zVERSION\x00" {
					_, _ = io.WriteString(conn, version+"\x00")
					return
				}
				for {
					var n uint32
					if binary.Read(r, binary.BigEndian, &n) != nil {
						return
					}
					if n == 0 {
						break
					}
					if _, err := io.CopyN(io.Discard, r, int64(n)); err != nil {
						return
					}
				}
				_, _ = io.WriteString(conn, reply.Load().(string))
			}()
		}
	}()
	return l.Addr().String()
}

func setAssetTestClam(cfg config.ClamAVSecurityConfig) {
	current := config.Current()
	current.Storage.UploadSecurity.ClamAV = cfg
	config.SetCurrent(&current)
}

func lastQuarantinedAsset(t *testing.T, db *gorm.DB, err error) *models.Asset {
	t.Helper()
	var q *errorsx.AssetQuarantinedError
	require.ErrorAs(t, err, &q)
	var asset models.Asset
	require.NoError(t, db.First(&asset, q.AssetID).Error)
	require.Equal(t, models.AssetScanQuarantined, asset.ScanStatus)
	require.False(t, asset.Usable())
	var attempt models.AssetScanAttempt
	if db.Where("asset_id = ?", asset.ID).Order("id DESC").First(&attempt).Error == nil {
		t.Logf("quarantined %s: %s", asset.ScanReason, attempt.Detail)
	}
	_, readErr := AssetService.OpenReader(&asset)
	require.Error(t, readErr)
	_, rangeErr := AssetService.OpenRange(&asset, 0, 1)
	require.Error(t, rangeErr)
	_, urlErr := AssetService.GetSignedURL(asset.ID)
	require.Error(t, urlErr)
	return &asset
}

func TestAssetQuarantineScanAndRescan(t *testing.T) {
	db, op := setupAssetQuarantineTest(t)
	var reply atomic.Value
	reply.Store("stream: Eicar-Signature FOUND\x00")
	failOpen := false
	setAssetTestClam(config.ClamAVSecurityConfig{Enabled: true, Address: fakeAssetClamAV(t, &reply), FailClosed: &failOpen})
	data := []byte("ordinary test content")
	asset, err := AssetService.UploadBytes(data, "tickets", "test.txt", op)
	require.Nil(t, asset)
	isolated := lastQuarantinedAsset(t, db, err)
	require.Equal(t, ScanInfected, isolated.ScanReason)
	p, err := quarantinePath(config.Current().Storage, isolated.QuarantineKey)
	require.NoError(t, err)
	actual, err := os.ReadFile(p)
	require.NoError(t, err)
	require.Equal(t, data, actual)
	require.NoFileExists(t, filepath.Join(config.Current().Storage.Local.Root, isolated.StorageKey))
	foreign := *op
	foreign.TenantID++
	_, err = AssetService.RescanAsset(isolated.ID, &foreign)
	require.Error(t, err)
	reply.Store("stream: OK\x00")
	released, err := AssetService.RescanAsset(isolated.ID, op)
	require.NoError(t, err)
	require.True(t, released.Usable())
	require.NoFileExists(t, p)
	r, err := AssetService.OpenReader(released)
	require.NoError(t, err)
	published, err := io.ReadAll(r)
	_ = r.Close()
	require.NoError(t, err)
	require.Equal(t, data, published)
	var attempts []models.AssetScanAttempt
	require.NoError(t, db.Where("asset_id = ?", isolated.ID).Order("id").Find(&attempts).Error)
	require.Len(t, attempts, 2)
	require.Equal(t, ScanInfected, attempts[0].Reason)
	require.Equal(t, models.AssetScanClean, attempts[1].Status)
	require.True(t, attempts[0].ScanPerformed)
	require.NotEmpty(t, attempts[1].DatabaseVersion)
	_, err = AssetService.RescanAsset(isolated.ID, op)
	require.Error(t, err)
}

func TestAssetQuarantineMissingMultipartFile(t *testing.T) {
	db, op := setupAssetQuarantineTest(t)
	_, err := AssetService.UploadFile(&multipart.FileHeader{Filename: "missing.txt", Size: 12}, "tickets", op)
	item := lastQuarantinedAsset(t, db, err)
	require.Equal(t, "missing.txt", item.Filename)
	require.Equal(t, ScanFailed, item.ScanReason)
	require.False(t, item.ReceiveComplete)
	_, err = AssetService.RescanAsset(item.ID, op)
	require.Error(t, err)
}

func TestAssetQuarantineScannerUnavailableOrUnready(t *testing.T) {
	for _, mode := range []string{"unavailable", "timeout", "stale database", "missing database"} {
		t.Run(mode, func(t *testing.T) {
			db, op := setupAssetQuarantineTest(t)
			var address string
			if mode == "unavailable" || mode == "timeout" {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				address = listener.Addr().String()
				if mode == "unavailable" {
					require.NoError(t, listener.Close())
				} else {
					t.Cleanup(func() { _ = listener.Close() })
					go func() {
						conn, err := listener.Accept()
						if err != nil {
							return
						}
						defer conn.Close()
						_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
						_, _ = io.Copy(io.Discard, conn) // Accept but never return a verdict.
					}()
				}
			} else {
				var reply atomic.Value
				reply.Store("stream: OK\x00")
				version := "ClamAV 1.4.3"
				if mode == "stale database" {
					version += "/99999/" + time.Now().UTC().Add(-8*24*time.Hour).Format("Mon Jan _2 15:04:05 2006")
				}
				address = fakeAssetClamAVVersion(t, &reply, version)
			}
			setAssetTestClam(config.ClamAVSecurityConfig{Enabled: true, Address: address, TimeoutSeconds: 1})
			_, err := AssetService.UploadBytes([]byte("safe content"), "tickets", "safe.txt", op)
			item := lastQuarantinedAsset(t, db, err)
			require.Equal(t, ScanFailed, item.ScanReason)
			require.True(t, item.ReceiveComplete)
		})
	}
}

func TestAssetQuarantineArchiveExtractionFailures(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.CreateHeader(&zip.FileHeader{Name: "inside.txt", Method: zip.Store})
	require.NoError(t, err)
	_, err = entry.Write([]byte("checksum sample"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	corrupt := bytes.Clone(buffer.Bytes())
	offset := bytes.Index(corrupt, []byte("checksum sample"))
	require.GreaterOrEqual(t, offset, 0)
	corrupt[offset] ^= 1
	encrypted := bytes.Clone(buffer.Bytes())
	encrypted[6] |= 1
	central := bytes.Index(encrypted, []byte("PK\x01\x02"))
	require.GreaterOrEqual(t, central, 0)
	encrypted[central+8] |= 1
	nested := buildTestZip(t, map[string][]byte{"nested.zip": buffer.Bytes()})
	for _, tc := range []struct {
		name, filename, reason string
		data                   []byte
	}{
		{"checksum error", "corrupt.zip", ScanArchiveError, corrupt},
		{"encrypted", "encrypted.zip", ScanArchiveError, encrypted},
		{"nested limit", "nested.zip", ScanLimitExceeded, nested},
		{"unsupported archive", "file.rar", ScanArchiveError, []byte("Rar!unsupported container")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, op := setupAssetQuarantineTest(t)
			var reply atomic.Value
			reply.Store("stream: OK\x00")
			setAssetTestClam(config.ClamAVSecurityConfig{Enabled: true, Address: fakeAssetClamAV(t, &reply)})
			cfg := config.Current()
			cfg.Storage.UploadSecurity.MaxArchiveDepth = 1
			config.SetCurrent(&cfg)
			_, err := AssetService.UploadBytes(tc.data, "tickets", tc.filename, op)
			item := lastQuarantinedAsset(t, db, err)
			require.Equal(t, tc.reason, item.ScanReason)
			require.True(t, item.ReceiveComplete)
		})
	}
}

func TestAssetQuarantineFailuresAndLimits(t *testing.T) {
	for _, tc := range []struct{ name, reply, reason string }{
		{"scanner disabled", "", ScanFailed},
		{"truncated OK", "stream: OK", ScanFailed},
		{"clamd scan limit", "stream: Heuristics.Limits.Exceeded.MaxScanSize FOUND\x00", ScanLimitExceeded},
		{"encrypted", "stream: Heuristics.Encrypted.Zip FOUND\x00", ScanArchiveError},
		{"protocol error", "UNKNOWN COMMAND\x00", ScanFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, op := setupAssetQuarantineTest(t)
			if tc.reply != "" {
				var reply atomic.Value
				reply.Store(tc.reply)
				setAssetTestClam(config.ClamAVSecurityConfig{Enabled: true, Address: fakeAssetClamAV(t, &reply)})
			}
			_, err := AssetService.UploadBytes([]byte("test"), "tickets", "file.txt", op)
			a := lastQuarantinedAsset(t, db, err)
			require.Equal(t, tc.reason, a.ScanReason)
			require.True(t, a.ReceiveComplete)
		})
	}
	t.Run("application and reception limits", func(t *testing.T) {
		db, op := setupAssetQuarantineTest(t)
		_, err := AssetService.UploadBytes(bytes.Repeat([]byte("x"), (1<<20)+1), "tickets", "large.txt", op)
		a := lastQuarantinedAsset(t, db, err)
		require.Equal(t, ScanLimitExceeded, a.ScanReason)
		require.True(t, a.ReceiveComplete)
		_, err = AssetService.UploadBytes(bytes.Repeat([]byte("x"), (2<<20)+100), "tickets", "too-large.txt", op)
		a = lastQuarantinedAsset(t, db, err)
		require.Equal(t, ScanLimitExceeded, a.ScanReason)
		require.False(t, a.ReceiveComplete)
		_, err = AssetService.RescanAsset(a.ID, op)
		require.Error(t, err)
	})
}

func TestAssetQuarantineTamperingAndRecovery(t *testing.T) {
	db, op := setupAssetQuarantineTest(t)
	_, err := AssetService.UploadBytes([]byte("original"), "tickets", "test.txt", op)
	a := lastQuarantinedAsset(t, db, err)
	p, err := quarantinePath(config.Current().Storage, a.QuarantineKey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(p, []byte("replaced"), 0600))
	_, err = AssetService.RescanAsset(a.ID, op)
	lastQuarantinedAsset(t, db, err)
	var latest models.AssetScanAttempt
	require.NoError(t, db.Order("id DESC").First(&latest).Error)
	require.Contains(t, latest.Detail, "changed")
	old := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, db.Model(a).Updates(map[string]any{"scan_status": models.AssetScanPending, "scan_token": "interrupted", "scan_started_at": old}).Error)
	require.NoError(t, AssetService.RecoverInterruptedScans(time.Now().UTC().Add(-15*time.Minute)))
	require.NoError(t, db.First(a, a.ID).Error)
	require.Equal(t, models.AssetScanQuarantined, a.ScanStatus)
	_, err = quarantinePath(config.StorageConfig{Local: config.LocalStorageConfig{Root: t.TempDir()}, QuarantineRoot: ""}, "../bad")
	require.Error(t, err)
}

func TestAssetQuarantineLegacyRequiresRealScan(t *testing.T) {
	db, op := setupAssetQuarantineTest(t)
	var reply atomic.Value
	reply.Store("stream: OK\x00")
	setAssetTestClam(config.ClamAVSecurityConfig{Enabled: true, Address: fakeAssetClamAV(t, &reply)})
	provider := storage.NewLocalStorage(config.Current().Storage.Local)
	_, err := provider.Upload(strings.NewReader("legacy"), "legacy.txt", storage.UploadInfo{})
	require.NoError(t, err)
	a := &models.Asset{TenantID: op.TenantID, AssetID: "legacy", StorageKey: "legacy.txt", Filename: "legacy.txt", FileSize: 6, Status: enums.AssetStatusSuccess}
	require.NoError(t, db.Create(a).Error)
	require.False(t, a.Usable())
	_, err = AssetService.OpenReader(a)
	require.Error(t, err)
	clean, err := AssetService.RescanAsset(a.ID, op)
	require.NoError(t, err)
	require.True(t, clean.Usable())
}

func TestAssetQuarantineConcurrentRescanAndKnowledgeGate(t *testing.T) {
	db, op := setupAssetQuarantineTest(t)
	_, err := AssetService.UploadBytes([]byte("knowledge text"), "knowledge", "file.txt", op)
	a := lastQuarantinedAsset(t, db, err)
	doc := &models.KnowledgeDocument{TenantID: op.TenantID, SourceAssetID: a.ID}
	require.False(t, knowledgeDocumentAssetUsable(doc))
	var reply atomic.Value
	reply.Store("stream: OK\x00")
	setAssetTestClam(config.ClamAVSecurityConfig{Enabled: true, Address: fakeAssetClamAV(t, &reply)})
	var wg sync.WaitGroup
	var successes atomic.Int32
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := AssetService.RescanAsset(a.ID, op); err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), successes.Load())
	var count int64
	require.NoError(t, db.Model(&models.AssetScanAttempt{}).Where("asset_id = ?", a.ID).Count(&count).Error)
	require.Equal(t, int64(2), count)
	require.True(t, knowledgeDocumentAssetUsable(doc))
	doc.TenantID++
	require.False(t, knowledgeDocumentAssetUsable(doc))
}

// Opt-in integration test. Requires the managed clamd profile and real current signatures.
func TestAssetQuarantineRealClamAV(t *testing.T) {
	address := os.Getenv("TKT007_CLAMAV_TEST_ADDRESS")
	if address == "" {
		t.Skip("set TKT007_CLAMAV_TEST_ADDRESS to an isolated ClamAV instance")
	}
	db, op := setupAssetQuarantineTest(t)
	setAssetTestClam(config.ClamAVSecurityConfig{Enabled: true, Address: address, TimeoutSeconds: 30})
	asset, err := AssetService.UploadBytes([]byte("Hello from the isolated scan test"), "test", "clean.txt", op)
	require.NoError(t, err)
	require.True(t, asset.Usable())
	var attempt models.AssetScanAttempt
	require.NoError(t, db.Where("asset_id = ?", asset.ID).First(&attempt).Error)
	require.True(t, attempt.ScanPerformed)
	t.Logf("Real engine: %s; signatures: %s", attempt.EngineVersion, attempt.DatabaseVersion)
	eicar := []byte("X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*")
	verdict := scanAttachmentWithClamAV(eicar, config.Current().Storage.UploadSecurity.ClamAV)
	require.Equal(t, ScanInfected, verdict.Reason, "direct real clamd verdict: %+v", verdict)
	_, err = AssetService.UploadBytes(eicar, "test", "eicar.txt", op)
	infected := lastQuarantinedAsset(t, db, err)
	require.Equal(t, ScanInfected, infected.ScanReason)
	require.NoError(t, db.Where("asset_id = ?", infected.ID).First(&models.AssetScanAttempt{}).Error)
	_, err = AssetService.UploadBytes([]byte("PK\x03\x04invalid"), "test", "broken.zip", op)
	broken := lastQuarantinedAsset(t, db, err)
	require.Equal(t, ScanArchiveError, broken.ScanReason)
	archive := buildTestZip(t, map[string][]byte{"eicar.txt": eicar})
	_, err = AssetService.UploadBytes(archive, "test", "eicar.zip", op)
	lastQuarantinedAsset(t, db, err)
	// Application limit is temporarily higher so the clamd MaxFileSize limit is exercised.
	cfg := config.Current()
	cfg.Storage.MaxUploadSizeMB = 24
	cfg.Storage.MaxReceiveSizeMB = 26
	config.SetCurrent(&cfg)
	_, err = AssetService.UploadBytes(bytes.Repeat([]byte("a"), 21<<20), "test", "engine-limit.txt", op)
	limited := lastQuarantinedAsset(t, db, err)
	require.Equal(t, ScanLimitExceeded, limited.ScanReason)
}

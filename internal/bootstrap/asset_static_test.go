package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/ginx"
)

func TestStaticAttachmentsRequireScanForGETHEADAndRange(t *testing.T) {
	oldDB := sqls.DB()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "static.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Asset{}))
	sqls.SetDB(db)
	t.Cleanup(func() { sqls.SetDB(oldDB); conn, _ := db.DB(); _ = conn.Close() })
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "sample.txt"), []byte("0123456789"), 0600))
	a := &models.Asset{AssetID: "static-test", StorageKey: "sample.txt", FileSize: 10, Status: enums.AssetStatusSuccess}
	require.NoError(t, db.Create(a).Error)
	handler := http.FileServer(scannedAssetFS{fs: ginx.StaticFiles(root)})
	for _, state := range []string{models.AssetScanUnscanned, models.AssetScanPending, models.AssetScanQuarantined, models.AssetScanClean} {
		require.NoError(t, db.Model(a).Update("scan_status", state).Error)
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			for _, withRange := range []bool{false, true} {
				req := httptest.NewRequest(method, "/sample.txt", nil)
				if withRange {
					req.Header.Set("Range", "bytes=0-2")
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				if state == models.AssetScanClean {
					require.Contains(t, []int{200, 206}, rec.Code)
				} else {
					require.Equal(t, 404, rec.Code)
					require.NotContains(t, rec.Body.String(), "012")
				}
			}
		}
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/../quarantine/private.bin", nil))
	require.Equal(t, 404, rec.Code)
}

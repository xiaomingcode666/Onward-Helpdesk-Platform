package enterprise

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"
)

func TestAssetQuarantineHTTPContract(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	require.NoError(t, db.AutoMigrate(&models.Asset{}, &models.AssetScanAttempt{}))
	for _, tenant := range []int64{8001, 8002} {
		a := &models.Asset{TenantID: tenant, AssetID: fmt.Sprint(tenant), StorageKey: fmt.Sprint(tenant), Filename: fmt.Sprintf("tenant-%d.txt", tenant), ScanStatus: models.AssetScanQuarantined, Status: enums.AssetStatusFailed, QuarantineKey: "secret-private-location"}
		require.NoError(t, db.Create(a).Error)
	}
	ctx, rec := newEnterpriseContractContext("GET", "/asset-quarantine", 8001)
	ctx.MustGet("authPrincipal").(*dto.AuthPrincipal).DomainType = "enterprise"
	AssetQuarantineList(ctx)
	require.Contains(t, rec.Body.String(), "tenant-8001.txt")
	require.NotContains(t, rec.Body.String(), "tenant-8002.txt")
	require.NotContains(t, rec.Body.String(), "secret-private-location")
	for _, handler := range []gin.HandlerFunc{AssetQuarantineAttempts, AssetQuarantineRescan} {
		ctx, rec = newEnterpriseContractContext("POST", "/asset-quarantine/2", 8001, gin.Param{Key: "id", Value: "2"})
		ctx.MustGet("authPrincipal").(*dto.AuthPrincipal).DomainType = "enterprise"
		handler(ctx)
		require.False(t, decodeEnterpriseEnvelope(t, rec).Success)
	}
	ctx, rec = newEnterpriseContractContext("GET", "/asset-quarantine", 8001)
	op := ctx.MustGet("authPrincipal").(*dto.AuthPrincipal)
	op.DomainType = "enterprise"
	op.Permissions = nil
	AssetQuarantineList(ctx)
	require.False(t, decodeEnterpriseEnvelope(t, rec).Success)
}

// Opt-in browser fixture. Only synthetic authentication and a temporary database are used.
func TestAssetQuarantineBrowserFixture(t *testing.T) {
	address, scanner := os.Getenv("TKT007_BROWSER_ADDRESS"), os.Getenv("TKT007_CLAMAV_TEST_ADDRESS")
	if address == "" || scanner == "" {
		t.Skip("opt-in isolated attachment browser fixture")
	}
	host, _, err := net.SplitHostPort(address)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", host)
	db := setupEnterpriseContractDB(t)
	require.NoError(t, db.AutoMigrate(&models.Asset{}, &models.AssetScanAttempt{}))
	previous := config.CurrentOrDefault()
	t.Cleanup(func() { config.SetCurrent(&previous) })
	root := t.TempDir()
	cfg := config.Config{Storage: config.StorageConfig{Default: enums.AssetProviderLocal, MaxUploadSizeMB: 1, MaxReceiveSizeMB: 2,
		Local: config.LocalStorageConfig{Root: filepath.Join(root, "public"), BaseURL: "/storage"}, QuarantineRoot: filepath.Join(root, "quarantine")}}
	config.SetCurrent(&cfg)
	op := &dto.AuthPrincipal{TenantID: 8001, DomainType: "enterprise", UserID: 9001, Username: "附件验收", Roles: []string{services.EnterpriseRoleAdmin}}
	_, err = services.AssetService.UploadBytes([]byte("Safe text for the quarantine browser acceptance test"), "ticket-upload", "等待复扫.txt", op)
	require.Error(t, err)
	foreign := *op
	foreign.TenantID = 8002
	_, err = services.AssetService.UploadBytes([]byte("Another tenant"), "ticket-upload", "其他公司附件.txt", &foreign)
	require.Error(t, err)
	cfg.Storage.UploadSecurity.ClamAV = config.ClamAVSecurityConfig{Enabled: true, Address: scanner, TimeoutSeconds: 30}
	config.SetCurrent(&cfg)
	_, err = services.AssetService.UploadBytes([]byte("PK\x03\x04bad zip"), "knowledge-documents", "损坏压缩包.zip", op)
	require.Error(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	done := make(chan struct{}, 1)
	router.Use(func(ctx *gin.Context) {
		if ctx.GetHeader("Authorization") != "Bearer asset-quarantine-fixture" {
			ctx.AbortWithStatus(401)
			return
		}
		ctx.Set("authPrincipal", op)
		ctx.Set("middlewareAuthPrincipal", op)
		ctx.Set("tenantId", "8001")
		ctx.Next()
	})
	router.GET("/api/enterprise/v1/asset-quarantine", AssetQuarantineList)
	router.GET("/api/enterprise/v1/asset-quarantine/:id/attempts", AssetQuarantineAttempts)
	router.POST("/api/enterprise/v1/asset-quarantine/:id/rescan", AssetQuarantineRescan)
	router.POST("/__test/stop", func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
		select {
		case done <- struct{}{}:
		default:
		}
	})
	l, err := net.Listen("tcp", address)
	require.NoError(t, err)
	server := httptest.NewUnstartedServer(router)
	server.Listener = l
	server.Start()
	defer server.Close()
	t.Logf("attachment browser fixture: %s", server.URL)
	select {
	case <-done:
	case <-time.After(10 * time.Minute):
		t.Fatal("browser fixture timed out")
	}
}

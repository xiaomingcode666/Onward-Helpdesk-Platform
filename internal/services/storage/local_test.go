package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"remotehelpdesk/internal/pkg/config"
)

func TestLocalStorageReadRangeAndPathBoundary(t *testing.T) {
	root := t.TempDir()
	provider := NewLocalStorage(config.LocalStorageConfig{Root: root})
	stored, err := provider.Upload(strings.NewReader("0123456789"), "media/sample.bin", UploadInfo{
		Filename: "sample.bin", FileSize: 10, MimeType: "application/octet-stream",
	})
	if err != nil || stored == nil {
		t.Fatalf("upload fixture: stored=%+v err=%v", stored, err)
	}

	reader, err := provider.ReadRange("media/sample.bin", 3, 4)
	if err != nil {
		t.Fatalf("read range: %v", err)
	}
	data, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if readErr != nil || string(data) != "3456" {
		t.Fatalf("range data=%q err=%v", data, readErr)
	}
	if _, err := provider.ReadRange("media/sample.bin", 8, 4); err == nil {
		t.Fatal("out-of-bounds range unexpectedly succeeded")
	}
	if _, err := provider.Read("../outside.bin"); err == nil {
		t.Fatal("path traversal key unexpectedly succeeded")
	}
}

func TestLocalStorageHealthCheck(t *testing.T) {
	root := t.TempDir()
	provider := NewLocalStorage(config.LocalStorageConfig{Root: root})
	if err := provider.HealthCheck(context.Background()); err != nil {
		t.Fatalf("health check writable directory: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read storage root: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("health check left probe files behind: %+v", entries)
	}
}

func TestLocalStorageHealthCheckRejectsMissingRootAndRegularFile(t *testing.T) {
	missing := NewLocalStorage(config.LocalStorageConfig{Root: filepath.Join(t.TempDir(), "missing")})
	if err := missing.HealthCheck(context.Background()); err == nil {
		t.Fatal("missing storage root health check succeeded")
	}

	path := filepath.Join(t.TempDir(), "storage-file")
	if err := os.WriteFile(path, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write regular file: %v", err)
	}
	regularFile := NewLocalStorage(config.LocalStorageConfig{Root: path})
	if err := regularFile.HealthCheck(context.Background()); err == nil {
		t.Fatal("regular file storage root health check succeeded")
	}
}

func TestParseMinIOEndpointUsesPublicScheme(t *testing.T) {
	endpoint, secure := parseMinIOEndpoint("https://files.example.com:9443", false)
	if endpoint != "files.example.com:9443" || !secure {
		t.Fatalf("parse public endpoint = %q, %v", endpoint, secure)
	}
	endpoint, secure = parseMinIOEndpoint("minio:9000", false)
	if endpoint != "minio:9000" || secure {
		t.Fatalf("parse internal endpoint = %q, %v", endpoint, secure)
	}
}

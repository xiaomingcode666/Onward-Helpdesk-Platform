package services

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"

	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/services/storage"
)

func buildTestZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buffer.Bytes()
}

func TestInspectUploadRejectsDangerousPayloads(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		data     string
	}{
		{name: "executable extension", filename: "repair-tool.exe", data: "plain text"},
		{name: "pe signature", filename: "repair-guide.pdf", data: "MZpayload"},
		{name: "html content", filename: "report.txt", data: "<!doctype html><script>alert(1)</script>"},
		{name: "shell script", filename: "steps.txt", data: "#!/bin/sh\necho unsafe"},
		{name: "eicar", filename: "scan.txt", data: "X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := inspectUpload(strings.NewReader(tt.data), storage.UploadInfo{Filename: tt.filename}, config.StorageConfig{})
			var i18nErr *errorsx.I18nError
			if !errors.As(err, &i18nErr) || i18nErr.Key != "error.upload.dangerous" {
				t.Fatalf("inspectUpload() error = %v, want dangerous upload error", err)
			}
		})
	}
}

func TestInspectUploadNormalizesSafeFileMetadata(t *testing.T) {
	data := "%PDF-1.7\nrepair guide"
	got, info, err := inspectUpload(strings.NewReader(data), storage.UploadInfo{
		Filename: "C:\\fakepath\\guide.pdf",
		FileSize: 1,
		MimeType: "application/octet-stream",
	}, config.StorageConfig{})
	if err != nil {
		t.Fatalf("inspectUpload() error = %v", err)
	}
	if string(got) != data {
		t.Fatalf("payload changed: %q", string(got))
	}
	if info.Filename != "guide.pdf" {
		t.Fatalf("filename = %q, want guide.pdf", info.Filename)
	}
	if info.FileSize != int64(len(data)) {
		t.Fatalf("file size = %d, want %d", info.FileSize, len(data))
	}
	if info.MimeType != "application/pdf" {
		t.Fatalf("mime type = %q, want application/pdf", info.MimeType)
	}
}

func TestInspectUploadPreservesBrowserAudioMIMEAfterContainerValidation(t *testing.T) {
	data := append([]byte{0x1a, 0x45, 0xdf, 0xa3}, []byte("browser-recorded-webm-audio")...)
	_, info, err := inspectUpload(strings.NewReader(string(data)), storage.UploadInfo{
		Filename: "voice.webm",
		MimeType: "audio/webm;codecs=opus",
	}, config.StorageConfig{})
	if err != nil {
		t.Fatalf("inspectUpload() error = %v", err)
	}
	if info.MimeType != "audio/webm" {
		t.Fatalf("mime type = %q, want audio/webm", info.MimeType)
	}
}

func TestInspectUploadDoesNotTrustSpoofedBrowserAudioMIME(t *testing.T) {
	_, info, err := inspectUpload(strings.NewReader("not a webm container"), storage.UploadInfo{
		Filename: "voice.webm",
		MimeType: "audio/webm",
	}, config.StorageConfig{})
	if err != nil {
		t.Fatalf("inspectUpload() error = %v", err)
	}
	if info.MimeType == "audio/webm" {
		t.Fatalf("spoofed browser audio MIME was trusted: %q", info.MimeType)
	}
}

func TestInspectUploadEnforcesActualSize(t *testing.T) {
	_, _, err := inspectUpload(strings.NewReader("12345"), storage.UploadInfo{Filename: "note.txt", FileSize: 1}, config.StorageConfig{MaxUploadSizeMB: -1})
	if err != nil {
		t.Fatalf("inspectUpload() unexpected error = %v", err)
	}
}

func TestInspectUploadValidatesArchiveExpansionAndContents(t *testing.T) {
	tests := []struct {
		name  string
		files map[string][]byte
		cfg   config.UploadSecurityConfig
	}{
		{
			name:  "excessive expansion",
			files: map[string][]byte{"large.txt": bytes.Repeat([]byte("0"), 2<<20)},
			cfg:   config.UploadSecurityConfig{MaxArchiveCompressionRatio: 10},
		},
		{
			name:  "too many entries",
			files: map[string][]byte{"a.txt": []byte("a"), "b.txt": []byte("b")},
			cfg:   config.UploadSecurityConfig{MaxArchiveEntries: 1},
		},
		{
			name:  "dangerous nested file",
			files: map[string][]byte{"tools/repair.exe": []byte("plain text")},
		},
		{
			name:  "traversal path",
			files: map[string][]byte{"../outside.txt": []byte("plain text")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := buildTestZip(t, tt.files)
			_, _, err := inspectUpload(bytes.NewReader(data), storage.UploadInfo{Filename: "bundle.zip"}, config.StorageConfig{UploadSecurity: tt.cfg})
			var i18nErr *errorsx.I18nError
			if !errors.As(err, &i18nErr) || i18nErr.Key != "error.upload.dangerous" {
				t.Fatalf("inspectUpload() error = %v, want dangerous upload error", err)
			}
		})
	}
}

func TestInspectUploadAcceptsNormalArchive(t *testing.T) {
	data := buildTestZip(t, map[string][]byte{
		"readme.txt": []byte("repair instructions"),
		"data.csv":   []byte("part,status\npump,ok\n"),
	})
	_, info, err := inspectUpload(bytes.NewReader(data), storage.UploadInfo{Filename: "bundle.zip"}, config.StorageConfig{})
	if err != nil {
		t.Fatalf("inspectUpload() error = %v", err)
	}
	if info.MimeType != "application/zip" {
		t.Fatalf("mime type = %q, want application/zip", info.MimeType)
	}
}

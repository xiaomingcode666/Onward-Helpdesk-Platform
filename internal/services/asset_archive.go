package services

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"path"
	"strings"
	"time"

	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/services/storage"
)

type uploadInspectionError struct{ reason, detail string }

func (e *uploadInspectionError) Error() string { return e.detail }
func (e *uploadInspectionError) Unwrap() error {
	return errorsx.InvalidParamI18n("error.upload.dangerous")
}

type archiveBudget struct {
	security config.UploadSecurityConfig
	entries  int
	expanded int64
	deadline time.Time
}

func checkArchiveContents(data []byte, filename string, security config.UploadSecurityConfig) error {
	b := &archiveBudget{security: security, deadline: time.Now().Add(security.ClamAV.TimeoutOrDefault())}
	return b.check(data, filename, 0)
}

func (b *archiveBudget) check(data []byte, filename string, depth int) error {
	fail := func(reason, detail string) error { return &uploadInspectionError{reason, detail} }
	ext := strings.ToLower(path.Ext(filename))
	isZip := bytes.HasPrefix(data, []byte("PK\x03\x04")) || bytes.HasPrefix(data, []byte("PK\x05\x06")) || bytes.HasPrefix(data, []byte("PK\x07\x08"))
	if !isZip && ext != ".zip" && ext != ".docx" && ext != ".xlsx" && ext != ".pptx" {
		// Formats whose full extraction cannot be checked by this gate must not pass silently.
		for _, magic := range [][]byte{[]byte("Rar!"), {0x37, 0x7a, 0xbc, 0xaf, 0x27, 0x1c}, {0x1f, 0x8b}, []byte("BZh"), {0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}} {
			if bytes.HasPrefix(data, magic) {
				return fail(ScanArchiveError, "Unsupported archive container")
			}
		}
		if len(data) > 262 && string(data[257:262]) == "ustar" {
			return fail(ScanArchiveError, "Unsupported archive container")
		}
		switch ext {
		case ".rar", ".7z", ".gz", ".gzip", ".tar", ".tgz", ".bz2", ".xz", ".cab", ".zst":
			return fail(ScanArchiveError, "Unsupported archive container")
		}
		return nil
	}
	if depth >= b.security.ArchiveDepthLimit() {
		return fail(ScanLimitExceeded, "Archive nesting limit exceeded")
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fail(ScanArchiveError, "Invalid ZIP directory")
	}
	b.entries += len(reader.File)
	if b.entries > b.security.ArchiveEntriesLimit() {
		return fail(ScanLimitExceeded, "Archive entry count exceeded")
	}
	for _, entry := range reader.File {
		if time.Now().After(b.deadline) {
			return fail(ScanLimitExceeded, "Archive inspection timed out")
		}
		if entry.Flags&1 != 0 {
			return fail(ScanArchiveError, "Encrypted archive cannot be inspected")
		}
		name := strings.ReplaceAll(entry.Name, "\\", "/")
		if strings.HasPrefix(name, "/") || strings.Contains(name, ":") || name == ".." || strings.HasPrefix(name, "../") || strings.Contains(name, "/../") || strings.HasSuffix(name, "/..") {
			return fail(ScanArchiveError, "Unsafe archive path")
		}
		if entry.FileInfo().IsDir() {
			continue
		}
		if !entry.Mode().IsRegular() {
			return fail(ScanArchiveError, "Archive contains a link or special file")
		}
		remaining := b.security.ArchiveExpandedSizeLimit() - b.expanded
		if entry.UncompressedSize64 > uint64(remaining) {
			return fail(ScanLimitExceeded, "Archive expanded size exceeded")
		}
		if entry.UncompressedSize64 >= 1<<20 && (entry.CompressedSize64 == 0 || entry.CompressedSize64 <= (entry.UncompressedSize64-1)/uint64(b.security.ArchiveCompressionRatioLimit())) {
			return fail(ScanLimitExceeded, "Archive compression ratio exceeded")
		}
		r, err := entry.Open()
		if err != nil {
			return fail(ScanArchiveError, "Archive entry cannot be opened")
		}
		contents, readErr := io.ReadAll(io.LimitReader(&archiveDeadlineReader{r, b.deadline}, remaining+1))
		closeErr := r.Close()
		if int64(len(contents)) > remaining {
			return fail(ScanLimitExceeded, "Actual archive expanded size exceeded")
		}
		if readErr != nil || closeErr != nil {
			var limitErr *uploadInspectionError
			if errors.As(readErr, &limitErr) {
				return limitErr
			}
			return fail(ScanArchiveError, "Archive decompression or checksum failed")
		}
		b.expanded += int64(len(contents))
		if err := b.check(contents, name, depth+1); err != nil {
			return err
		}
		info := storage.UploadInfo{Filename: name, MimeType: detectedUploadMIME(contents, name, "")}
		if uploadIsDangerous(contents, info, b.security) {
			return fail(ScanPolicyBlocked, "Archive contains a blocked file")
		}
	}
	return nil
}

type archiveDeadlineReader struct {
	io.Reader
	deadline time.Time
}

func (r *archiveDeadlineReader) Read(p []byte) (int, error) {
	if time.Now().After(r.deadline) {
		return 0, &uploadInspectionError{ScanLimitExceeded, "Archive inspection timed out"}
	}
	return r.Reader.Read(p)
}

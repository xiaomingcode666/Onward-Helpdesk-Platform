package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"
)

type LocalStorage struct {
	cfg config.LocalStorageConfig
}

func NewLocalStorage(cfg config.LocalStorageConfig) *LocalStorage {
	return &LocalStorage{cfg: cfg}
}

func (s *LocalStorage) ProviderType() enums.AssetProvider {
	return enums.AssetProviderLocal
}

func (s *LocalStorage) HealthCheck(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root := strings.TrimSpace(s.cfg.Root)
	if root == "" {
		return fmt.Errorf("local storage root is empty")
	}
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("stat local storage root: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("local storage root is not a directory")
	}
	probe, err := os.CreateTemp(root, ".healthcheck-*")
	if err != nil {
		return fmt.Errorf("local storage root is not writable: %w", err)
	}
	probePath := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(probePath)
		return fmt.Errorf("close local storage health probe: %w", err)
	}
	if err := os.Remove(probePath); err != nil {
		return fmt.Errorf("remove local storage health probe: %w", err)
	}
	return nil
}

func (s *LocalStorage) Upload(reader io.Reader, key string, info UploadInfo) (*StoredFile, error) {
	fullPath, err := s.resolvePath(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, err
	}

	dst, err := os.Create(fullPath)
	if err != nil {
		return nil, err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, reader); err != nil {
		return nil, err
	}

	return &StoredFile{
		Provider:   enums.AssetProviderLocal,
		StorageKey: key,
		URL:        strings.TrimRight(s.cfg.BaseURL, "/") + "/" + strings.TrimLeft(key, "/"),
		Filename:   info.Filename,
		FileSize:   info.FileSize,
		MimeType:   info.MimeType,
	}, nil
}

func (s *LocalStorage) GetURL(key string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(s.cfg.BaseURL), "/")
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(key, "/")
}

func (s *LocalStorage) GetSignedURL(key string) string {
	return s.GetURL(key)
}

func (s *LocalStorage) Delete(key string) error {
	fullPath, err := s.resolvePath(key)
	if err != nil {
		return err
	}
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(fullPath)
}

func (s *LocalStorage) Read(key string) (io.ReadCloser, error) {
	fullPath, err := s.resolvePath(key)
	if err != nil {
		return nil, err
	}
	return os.Open(fullPath)
}

func (s *LocalStorage) ReadRange(key string, offset, length int64) (io.ReadCloser, error) {
	fullPath, err := s.resolvePath(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	if offset < 0 || length <= 0 {
		_ = file.Close()
		return nil, errors.New("invalid local storage byte range")
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || offset > info.Size() || length > info.Size()-offset {
		_ = file.Close()
		return nil, errors.New("local storage byte range exceeds file size")
	}
	return &rangeReadCloser{Reader: io.NewSectionReader(file, offset, length), Closer: file}, nil
}

func (s *LocalStorage) resolvePath(key string) (string, error) {
	root, err := filepath.Abs(strings.TrimSpace(s.cfg.Root))
	if err != nil || strings.TrimSpace(s.cfg.Root) == "" {
		return "", errors.New("local storage root is invalid")
	}
	normalized := filepath.Clean(filepath.FromSlash(strings.TrimSpace(key)))
	if normalized == "." || filepath.IsAbs(normalized) || normalized == ".." || strings.HasPrefix(normalized, ".."+string(filepath.Separator)) {
		return "", errors.New("local storage key escapes storage root")
	}
	fullPath := filepath.Join(root, normalized)
	relative, err := filepath.Rel(root, fullPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("local storage key escapes storage root")
	}
	return fullPath, nil
}

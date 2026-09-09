package storage

import (
	"context"
	"io"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
)

type FileStorageProvider interface {
	ProviderType() enums.AssetProvider
	HealthCheck(ctx context.Context) error
	Upload(reader io.Reader, key string, info UploadInfo) (*StoredFile, error)
	GetURL(key string) string
	GetSignedURL(key string) string
	Delete(key string) error
	Read(key string) (io.ReadCloser, error)
	ReadRange(key string, offset, length int64) (io.ReadCloser, error)
}

type rangeReadCloser struct {
	io.Reader
	io.Closer
}

func GetDefault() (FileStorageProvider, error) {
	return NewProvider(config.Current().Storage.Default)
}

func GetProvider(providerType enums.AssetProvider) (FileStorageProvider, error) {
	return NewProvider(providerType)
}

func NewProvider(provider enums.AssetProvider) (FileStorageProvider, error) {
	cfg := config.Current().Storage

	switch provider {
	case "", enums.AssetProviderLocal:
		return NewLocalStorage(cfg.Local), nil
	case enums.AssetProviderOSS:
		return NewOSSStorage(cfg.OSS), nil
	case enums.AssetProviderMinIO:
		return NewMinIOStorage(cfg.MinIO), nil
	default:
		return nil, errorsx.InvalidParamI18n("error.e0082")
	}
}

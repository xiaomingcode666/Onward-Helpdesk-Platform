package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOStorage struct {
	cfg config.MinIOStorageConfig
}

func NewMinIOStorage(cfg config.MinIOStorageConfig) *MinIOStorage {
	return &MinIOStorage{cfg: cfg}
}

func (s *MinIOStorage) ProviderType() enums.AssetProvider {
	return enums.AssetProviderMinIO
}

func (s *MinIOStorage) HealthCheck(ctx context.Context) error {
	client, err := s.getClient()
	if err != nil {
		return err
	}
	exists, err := client.BucketExists(ctx, strings.TrimSpace(s.cfg.Bucket))
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("minio bucket %q does not exist", strings.TrimSpace(s.cfg.Bucket))
	}
	return nil
}

func (s *MinIOStorage) Upload(reader io.Reader, key string, info UploadInfo) (*StoredFile, error) {
	client, err := s.getClient()
	if err != nil {
		return nil, err
	}
	if err := s.ensureBucket(client); err != nil {
		return nil, err
	}
	key = normalizeOSSKey(key)
	putOptions := minio.PutObjectOptions{ContentType: strings.TrimSpace(info.MimeType)}
	if _, err := client.PutObject(context.Background(), strings.TrimSpace(s.cfg.Bucket), key, reader, info.FileSize, putOptions); err != nil {
		return nil, err
	}
	return &StoredFile{
		Provider:   enums.AssetProviderMinIO,
		StorageKey: key,
		URL:        s.GetURL(key),
		Filename:   info.Filename,
		FileSize:   info.FileSize,
		MimeType:   info.MimeType,
	}, nil
}

func (s *MinIOStorage) GetURL(key string) string {
	key = normalizeOSSKey(key)
	if key == "" {
		return ""
	}
	if baseURL := strings.TrimRight(strings.TrimSpace(s.cfg.BaseURL), "/"); baseURL != "" {
		return baseURL + "/" + key
	}
	return s.objectURL(key)
}

func (s *MinIOStorage) GetSignedURL(key string) string {
	key = normalizeOSSKey(key)
	if key == "" {
		return ""
	}
	if !s.cfg.Private {
		return s.GetURL(key)
	}
	client, err := s.getSigningClient()
	if err != nil {
		return ""
	}
	presignedURL, err := client.PresignedGetObject(context.Background(), strings.TrimSpace(s.cfg.Bucket), key, s.signedURLExpire(), nil)
	if err != nil {
		return ""
	}
	return presignedURL.String()
}

func (s *MinIOStorage) Delete(key string) error {
	client, err := s.getClient()
	if err != nil {
		return err
	}
	return client.RemoveObject(context.Background(), strings.TrimSpace(s.cfg.Bucket), normalizeOSSKey(key), minio.RemoveObjectOptions{})
}

func (s *MinIOStorage) Read(key string) (io.ReadCloser, error) {
	client, err := s.getClient()
	if err != nil {
		return nil, err
	}
	object, err := client.GetObject(context.Background(), strings.TrimSpace(s.cfg.Bucket), normalizeOSSKey(key), minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := object.Stat(); err != nil {
		_ = object.Close()
		return nil, err
	}
	return object, nil
}

func (s *MinIOStorage) ReadRange(key string, offset, length int64) (io.ReadCloser, error) {
	if offset < 0 || length <= 0 {
		return nil, errors.New("invalid minio byte range")
	}
	client, err := s.getClient()
	if err != nil {
		return nil, err
	}
	options := minio.GetObjectOptions{}
	if err := options.SetRange(offset, offset+length-1); err != nil {
		return nil, err
	}
	object, err := client.GetObject(context.Background(), strings.TrimSpace(s.cfg.Bucket), normalizeOSSKey(key), options)
	if err != nil {
		return nil, err
	}
	if _, err := object.Stat(); err != nil {
		_ = object.Close()
		return nil, err
	}
	return object, nil
}

func (s *MinIOStorage) getClient() (*minio.Client, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	endpoint, secure := parseMinIOEndpoint(s.cfg.Endpoint, s.cfg.UseSSL)
	return s.newClient(endpoint, secure)
}

func (s *MinIOStorage) getSigningClient() (*minio.Client, error) {
	if strings.TrimSpace(s.cfg.PublicEndpoint) == "" {
		return s.getClient()
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	endpoint, secure := parseMinIOEndpoint(s.cfg.PublicEndpoint, s.cfg.UseSSL)
	return s.newClient(endpoint, secure)
}

func (s *MinIOStorage) newClient(endpoint string, secure bool) (*minio.Client, error) {
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(strings.TrimSpace(s.cfg.AccessKeyID), strings.TrimSpace(s.cfg.AccessKeySecret), ""),
		Secure: secure,
		Region: strings.TrimSpace(s.cfg.Region),
	})
}

func (s *MinIOStorage) ensureBucket(client *minio.Client) error {
	ctx := context.Background()
	bucket := strings.TrimSpace(s.cfg.Bucket)
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: strings.TrimSpace(s.cfg.Region)})
}

func (s *MinIOStorage) validate() error {
	if strings.TrimSpace(s.cfg.Endpoint) == "" {
		return errorsx.InvalidParamI18n("error.e0051")
	}
	if strings.TrimSpace(s.cfg.Bucket) == "" {
		return errorsx.InvalidParamI18n("error.e0050")
	}
	if strings.TrimSpace(s.cfg.AccessKeyID) == "" {
		return errorsx.InvalidParamI18n("error.e0048")
	}
	if strings.TrimSpace(s.cfg.AccessKeySecret) == "" {
		return errorsx.InvalidParamI18n("error.e0049")
	}
	return nil
}

func (s *MinIOStorage) objectURL(key string) string {
	endpoint, secure := parseMinIOEndpoint(s.cfg.PublicEndpoint, s.cfg.UseSSL)
	if endpoint == "" {
		endpoint, secure = parseMinIOEndpoint(s.cfg.Endpoint, s.cfg.UseSSL)
	}
	scheme := "http"
	if secure {
		scheme = "https"
	}
	baseURL, err := url.Parse(fmt.Sprintf("%s://%s", scheme, endpoint))
	if err != nil || baseURL.Host == "" {
		return ""
	}
	return fmt.Sprintf("%s://%s/%s/%s", baseURL.Scheme, baseURL.Host, strings.TrimSpace(s.cfg.Bucket), key)
}

func parseMinIOEndpoint(raw string, fallbackSecure bool) (string, bool) {
	endpoint := strings.TrimSpace(raw)
	if endpoint == "" {
		return "", fallbackSecure
	}
	if parsed, err := url.Parse(endpoint); err == nil && parsed.Host != "" {
		return parsed.Host, strings.EqualFold(parsed.Scheme, "https")
	}
	return strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://"), fallbackSecure
}

func (s *MinIOStorage) signedURLExpire() time.Duration {
	if s.cfg.SignedURLExpire > 0 {
		return time.Duration(s.cfg.SignedURLExpire) * time.Second
	}
	return 10 * time.Minute
}

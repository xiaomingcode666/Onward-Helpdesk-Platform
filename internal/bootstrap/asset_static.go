package bootstrap

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/services"
)

type attachmentRequestBody struct {
	io.ReadCloser
	exceeded bool
}

func (b *attachmentRequestBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	var limit *http.MaxBytesError
	if errors.As(err, &limit) {
		b.exceeded = true
	}
	return n, err
}

// Protect legacy /storage links as well as newly generated URLs, including HEAD and Range.
type scannedAssetFS struct{ fs http.FileSystem }

func (s scannedAssetFS) Open(name string) (http.File, error) {
	asset := services.AssetService.GetByStorageKey(strings.TrimLeft(name, "/"))
	if !asset.Usable() || (asset.Provider != "" && asset.Provider != enums.AssetProviderLocal) {
		return nil, os.ErrNotExist
	}
	return s.fs.Open(name)
}

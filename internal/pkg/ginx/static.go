package ginx

import (
	"net/http"
	"os"
)

type noDirFS struct {
	fs http.FileSystem
}

func StaticFiles(root string) http.FileSystem {
	return noDirFS{fs: http.Dir(root)}
}

func (n noDirFS) Open(name string) (http.File, error) {
	file, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if info.IsDir() {
		_ = file.Close()
		return nil, os.ErrNotExist
	}
	return file, nil
}

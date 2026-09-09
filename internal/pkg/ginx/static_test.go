package ginx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStaticFilesDoesNotOpenDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	fileSystem := StaticFiles(root)
	if file, err := fileSystem.Open("/file.txt"); err != nil {
		t.Fatalf("Open(file.txt) err=%v", err)
	} else {
		_ = file.Close()
	}

	if file, err := fileSystem.Open("/"); err == nil {
		_ = file.Close()
		t.Fatal("Open(/) succeeded, want error")
	} else if !os.IsNotExist(err) {
		t.Fatalf("Open(/) err=%v, want not exist", err)
	}
}

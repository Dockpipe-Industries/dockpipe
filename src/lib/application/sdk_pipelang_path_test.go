package application

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSDKPathSelectsPipeLangLibraryDescriptor(t *testing.T) {
	workdir := t.TempDir()
	path, err := captureStdout(t, func() error {
		return cmdSDK([]string{"path", "--language", "pipelang", "--workdir", workdir})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) || filepath.Base(path) != "library.json" {
		t.Fatalf("sdk path = %q", path)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "src", "Client.pipe")); err != nil {
		t.Fatalf("SDK source facade is unavailable: %v", err)
	}
}

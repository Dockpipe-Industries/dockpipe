package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdInstallCoreIsRetiredWithoutMaterialization(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	err := cmdInstall([]string{"core", "--url", "https://example.invalid/core.tar.gz"})
	if err == nil || !strings.Contains(err.Error(), "self-contained") {
		t.Fatalf("expected self-contained CLI guidance, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "templates")); !os.IsNotExist(err) {
		t.Fatalf("retired install created a templates tree: %v", err)
	}
}

func TestCmdInstallUnknownTarget(t *testing.T) {
	if err := cmdInstall([]string{"unknown"}); err == nil {
		t.Fatal("expected an unknown-target error")
	}
}

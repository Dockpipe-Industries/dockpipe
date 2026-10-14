package infrastructure

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/engineversion"
)

func TestWorkflowCompatibilityBeforeParsingDiskAndTar(t *testing.T) {
	previous := engineversion.Current()
	engineversion.Configure("0.6.13")
	t.Cleanup(func() { engineversion.Configure(previous) })
	root := t.TempDir()
	manifest := []byte("name: future\nmin_dockpipe_version: 0.6.14\n")
	config := []byte("not valid: [workflow YAML")
	for name, data := range map[string][]byte{"package.yml": manifest, "config.yml": config} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	archivePath := filepath.Join(root, "future.tar.gz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(file)
	writer := tar.NewWriter(gz)
	for name, data := range map[string][]byte{"package.yml": manifest, "config.yml": config} {
		if err := writer.WriteHeader(&tar.Header{Name: "workflows/future/" + name, Mode: 0o644, Typeflag: tar.TypeReg, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	for _, close := range []func() error{writer.Close, gz.Close, file.Close} {
		if err := close(); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{filepath.Join(root, "config.yml"), "tar://" + archivePath + "##workflows/future/config.yml"} {
		_, err := LoadWorkflow(path)
		var update *domain.DockpipeUpdateRequiredError
		if !errors.As(err, &update) || update.Current != "0.6.13" || update.Minimum != "0.6.14" {
			t.Fatalf("expected compatibility error before YAML parsing for %s: %v", path, err)
		}
	}
}

func TestUnsupportedWorkflowFieldsHaveUpdateGuidance(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_REPO_ROOT", root)
	config := filepath.Join(root, "config.yml")
	if err := os.WriteFile(config, []byte("name: example\nfuture_field: true\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateResolvedWorkflowYAML(config)
	if err == nil || !strings.Contains(err.Error(), "Update Dockpipe") || !strings.Contains(err.Error(), "future_field") {
		t.Fatalf("missing actionable schema diagnostic: %v", err)
	}
	if err := os.WriteFile(config, []byte("name: example\nsteps: not-an-array\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = ValidateResolvedWorkflowYAML(config)
	if err == nil || strings.Contains(err.Error(), "Update Dockpipe") {
		t.Fatalf("ordinary malformed workflow misclassified as an update: %v", err)
	}
}

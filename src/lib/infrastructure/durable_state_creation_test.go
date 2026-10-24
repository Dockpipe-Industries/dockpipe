package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivateStateCreationIsProtectedAndExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.lock")
	file, err := createPrivateFile(path, os.O_RDWR)
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePrivatePath(path, false); err != nil {
		file.Close()
		t.Fatalf("new file was visible before private ownership and permissions: %v", err)
	}
	if _, err := file.WriteString("preserved identity"); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if duplicate, err := createPrivateFile(path, os.O_WRONLY); !os.IsExist(err) {
		if duplicate != nil {
			duplicate.Close()
		}
		t.Fatalf("existing identity was not rejected exclusively: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "preserved identity" {
		t.Fatalf("exclusive creation changed the existing identity: %q, %v", data, err)
	}
}

func TestPrivateStateTemporaryCreationIsProtected(t *testing.T) {
	root := t.TempDir()
	file, err := createPrivateTempFile(root, "metadata-*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := validatePrivatePath(file.Name(), false); err != nil {
		t.Fatalf("new temporary file lacked private ownership and permissions: %v", err)
	}
	name := filepath.Base(file.Name())
	if !strings.HasPrefix(name, "metadata-") || !strings.HasSuffix(name, ".tmp") {
		t.Fatalf("temporary file pattern was not retained: %q", name)
	}
	directory, err := createPrivateTempDirectory(root, "cohort-")
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePrivatePath(directory, true); err != nil {
		t.Fatalf("new temporary directory lacked private ownership and permissions: %v", err)
	}
	if !strings.HasPrefix(filepath.Base(directory), "cohort-") {
		t.Fatalf("temporary directory prefix was not retained: %q", directory)
	}
	for _, pattern := range []string{"../escape", "nested/name"} {
		if _, err := createPrivateTempFile(root, pattern); err == nil {
			t.Fatalf("temporary file accepted a path-bearing pattern: %q", pattern)
		}
		if _, err := createPrivateTempDirectory(root, pattern); err == nil {
			t.Fatalf("temporary directory accepted a path-bearing pattern: %q", pattern)
		}
	}
}

func TestPrivateStateWritersKeepCreatedFilesPrivate(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	if err := createPrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(root, "state.lock")
	lock, err := openPrivateLockFile(root, lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	metadataPath := filepath.Join(root, "metadata.json")
	if err := writePrivateJSONAtomic(root, metadataPath, map[string]string{"identity": "first"}); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateJSONAtomic(root, metadataPath, map[string]string{"identity": "second"}); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "manifest.json")
	if err := writeDurableImportManifest(root, manifestPath, durableImportManifest{Schema: durableImportSchema, Phase: "copying"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{lockPath, metadataPath, manifestPath} {
		if err := validatePrivatePath(path, false); err != nil {
			t.Fatalf("new durable state file %q failed ownership validation: %v", path, err)
		}
	}
}

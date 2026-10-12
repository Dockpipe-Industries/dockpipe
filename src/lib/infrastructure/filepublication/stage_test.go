package filepublication

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStageLeavesPublicationToCaller(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "record.json")
	if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	staged, err := Stage(directory, ".record-*", []byte("replacement"))
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(staged)
	if filepath.Dir(staged) != directory || staged == target {
		t.Fatalf("staged outside destination directory: %s", staged)
	}
	for path, expected := range map[string]string{target: "original", staged: "replacement"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != expected {
			t.Fatalf("%s: %q, %v", path, data, err)
		}
	}
	info, err := os.Stat(staged)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("staging file mode: %v", info.Mode())
	}
	if err := os.Rename(staged, target); err != nil {
		t.Fatalf("caller could not publish closed staging file: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "replacement" {
		t.Fatalf("published file: %q, %v", data, err)
	}
}

func TestStageCreationFailureLeavesDestinationUntouched(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "existing")
	if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if staged, err := Stage(target, ".record-*", []byte("replacement")); err == nil || staged != "" {
		t.Fatalf("staged under a regular file: %q, %v", staged, err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "original" {
		t.Fatalf("failed staging changed original: %q, %v", data, err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("unexpected staging residue: %v, %v", entries, err)
	}
}

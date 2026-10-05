package orchestrationhelper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicJSONPublicationPreservesOriginalAndCleansFailedStage(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "record.json")
	if err := writeJSONFileAtomic(target, map[string]int{"version": 1}); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFileAtomic(target, make(chan int)); err == nil {
		t.Fatal("accepted unserializable value")
	}
	unchanged, err := os.ReadFile(target)
	if err != nil || string(unchanged) != string(original) {
		t.Fatalf("failed serialization changed target: %q, %v", unchanged, err)
	}
	blocked := filepath.Join(directory, "blocked.json")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFileAtomic(blocked, map[string]int{"version": 2}); err == nil {
		t.Fatal("replaced a directory")
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 2 {
		t.Fatalf("failed publication left staging files: %v, %v", entries, err)
	}
	if err := writeJSONFileAtomic(target, map[string]int{"version": 2}); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(target)
	if err != nil || string(updated) != "{\n  \"version\": 2\n}\n" {
		t.Fatalf("JSON format or replacement changed: %q, %v", updated, err)
	}
}

func TestAtomicJSONPublicationRejectsSymlinkTarget(t *testing.T) {
	directory := t.TempDir()
	original := filepath.Join(directory, "original.json")
	if err := os.WriteFile(original, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "linked.json")
	if err := os.Symlink(original, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := writeJSONFileAtomic(link, map[string]int{"version": 2}); err == nil {
		t.Fatal("accepted symlink target")
	}
	content, err := os.ReadFile(original)
	if err != nil || string(content) != "original" {
		t.Fatalf("symlink target changed: %q, %v", content, err)
	}
}

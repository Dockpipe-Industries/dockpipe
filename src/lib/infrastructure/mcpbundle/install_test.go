package mcpbundle

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallVerifiedArtifactWithoutExecutionOrOverwrite(t *testing.T) {
	file, sum := fixture(t, "", nil)
	cache := t.TempDir()
	installed, err := Install(context.Background(), file, sum, "linux", "amd64", cache)
	if err != nil {
		t.Fatal(err)
	}
	if installed.Started || installed.Inspection.Executed {
		t.Fatal("installation launched a process")
	}
	content, err := os.ReadFile(installed.Executable)
	if err != nil || len(content) != 64 || string(content[1:4]) != "ELF" {
		t.Fatalf("wrong payload: %v", err)
	}
	receipt := filepath.Join(filepath.Dir(installed.Directory), "receipt.json")
	if _, err := os.Stat(receipt); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(installed.Directory, "developer-file")
	if err := os.WriteFile(marker, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(context.Background(), file, sum, "linux", "amd64", cache); err == nil {
		t.Fatal("existing artifact overwritten")
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "preserve" {
		t.Fatal("existing directory changed")
	}
	entries, _ := os.ReadDir(cache)
	if len(entries) != 1 {
		t.Fatalf("temporary staging leaked: %v", entries)
	}
}
func TestInstallRejectsUnsafeBundleAndCleansStage(t *testing.T) {
	file, sum := fixture(t, "../escape", nil)
	cache := t.TempDir()
	if _, err := Install(context.Background(), file, sum, "linux", "amd64", cache); err == nil {
		t.Fatal("unsafe bundle installed")
	}
	entries, _ := os.ReadDir(cache)
	if len(entries) != 0 {
		t.Fatal("failed install left state")
	}
	file, sum = fixture(t, "", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Install(ctx, file, sum, "linux", "amd64", cache); err == nil {
		t.Fatal("cancelled install succeeded")
	}
	entries, _ = os.ReadDir(cache)
	if len(entries) != 0 {
		t.Fatal("cancelled install left state")
	}
}

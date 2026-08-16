//go:build !windows

package infrastructure

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestDurableCohortImportRejectsSpecialLegacyFiles(t *testing.T) {
	workdir, legacy, _ := durableImportFixture(t)
	fifo := filepath.Join(legacy, "tpm-old", "unexpected.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareDurableCohortImport(workdir, durableImportTestSpec(legacy, "guest-a", "run-a")); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("special legacy state was accepted: %v", err)
	}
}

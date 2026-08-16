//go:build !windows

package statepaths

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestLearningAuthorityMigrationPublishesOwnerOnlyTree(t *testing.T) {
	workdir, _, _ := learningAuthorityLegacyFixture(t)
	durable, _, err := PrepareLearningAuthority(workdir)
	if err != nil {
		t.Fatal(err)
	}
	if err := filepath.Walk(durable, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		want := os.FileMode(0o600)
		if info.IsDir() {
			want = 0o700
		}
		if info.Mode().Perm() != want {
			t.Errorf("%s mode = %04o, want %04o", path, info.Mode().Perm(), want)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLearningAuthorityMigrationRejectsSpecialSelectedFile(t *testing.T) {
	workdir, legacy, _ := learningAuthorityLegacyFixture(t)
	path := filepath.Join(legacy, "metrics.jsonl")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("FIFO unavailable: %v", err)
	}
	if _, _, err := PrepareLearningAuthority(workdir); err == nil {
		t.Fatal("special selected file was accepted")
	}
	assertNoLearningAuthority(t, workdir)
}

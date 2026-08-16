//go:build !windows

package statepaths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProviderRecoveryMigrationPublishesOwnerOnlyTree(t *testing.T) {
	workdir, _, _ := providerRecoveryLegacyFixture(t)
	durable, _, err := PrepareProviderRecoveryAuthority(workdir)
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.Walk(durable, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		want := os.FileMode(0o600)
		if info.IsDir() {
			want = 0o700
		}
		if info.Mode().Perm() != want {
			t.Fatalf("%s mode = %04o, want %04o", path, info.Mode().Perm(), want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

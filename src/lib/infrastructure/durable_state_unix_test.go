//go:build !windows

package infrastructure

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestDurableStateCreationForcesOwnerOnlyModes(t *testing.T) {
	oldMask := syscall.Umask(0)
	defer syscall.Umask(oldMask)

	base := t.TempDir()
	project := filepath.Join(base, "project")
	if err := os.Mkdir(project, 0o777); err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(base, "state")
	packageRoot, err := projectPackageStateDirAt(project, "example.tools/provider-pool", stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	projectRoot := filepath.Dir(filepath.Dir(packageRoot))
	for _, path := range []string{stateRoot, filepath.Join(stateRoot, "projects"), projectRoot, filepath.Join(projectRoot, "packages"), packageRoot} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("directory %q mode %o, want 700", path, info.Mode().Perm())
		}
	}
	for _, path := range []string{
		filepath.Join(stateRoot, ".project-identity.lock"),
		filepath.Join(stateRoot, durableProjectIndexFile),
		filepath.Join(projectRoot, durableProjectMetaFile),
		filepath.Join(packageRoot, durablePackageMetaFile),
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("file %q mode %o, want 600", path, info.Mode().Perm())
		}
	}
}

func TestDurableStateRejectsSymlinkBoundaries(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}
	realRoot := filepath.Join(base, "real-state")
	if err := os.Mkdir(realRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	linkedRoot := filepath.Join(base, "linked-state")
	if err := os.Symlink(realRoot, linkedRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := projectStateRootAt(project, linkedRoot); err == nil {
		t.Fatal("linked durable root unexpectedly passed validation")
	}

	selectedRoot := filepath.Join(base, "selected")
	if err := os.Mkdir(selectedRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(base, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(selectedRoot, "sessions")); err != nil {
		t.Fatal(err)
	}
	if _, err := JoinStatePath(selectedRoot, "sessions/one.json"); err == nil {
		t.Fatal("linked suffix boundary unexpectedly passed validation")
	}
}

func TestLegacyDiscoveryRejectsLinkedPackageRoot(t *testing.T) {
	project := t.TempDir()
	stateRoot, err := StateRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	packagesRoot := filepath.Join(stateRoot, "packages")
	if err := os.MkdirAll(packagesRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(packagesRoot, "example-tools")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := DiscoverLegacyPackageState(project, "example.tools"); err == nil {
		t.Fatal("linked legacy package root unexpectedly passed discovery")
	}
}

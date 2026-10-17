//go:build !windows

package infrastructure

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

func TestPreparePrivateDirectoryRefusesExistingSharedDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := PreparePrivateDirectory(root); err == nil {
		t.Fatal("shared directory was accepted or silently repaired")
	}
	info, err := os.Stat(root)
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("existing directory permissions changed: %v, %v", info, err)
	}
}

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

func TestDurableStateAcceptsHomeAliasWithoutFollowingOwnedLinks(t *testing.T) {
	base := t.TempDir()
	realHome := filepath.Join(base, "var", "home", "user")
	if err := os.MkdirAll(realHome, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "home")
	if err := os.Symlink(filepath.Dir(realHome), alias); err != nil {
		t.Fatal(err)
	}
	stateBase := filepath.Join(alias, "user", ".local", "state")
	t.Setenv("XDG_STATE_HOME", stateBase)
	root, err := canonicalDurableStateRoot(filepath.Join(stateBase, "dockpipe"), "linux")
	if err != nil {
		t.Fatal(err)
	}
	canonicalBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(canonicalBase, "var", "home", "user", ".local", "state", "dockpipe")
	if root != want {
		t.Fatalf("root = %q, want %q", root, want)
	}
	if runtime.GOOS == "linux" {
		actual, err := DurableStateRoot()
		if err != nil || actual != want {
			t.Fatalf("OS root = %q, %v; want %q", actual, err, want)
		}
	}
	project := t.TempDir()
	if _, err := projectStateRootAt(project, root); err != nil {
		t.Fatalf("first run through home alias: %v", err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, root); err != nil {
		t.Fatal(err)
	}
	root, err = canonicalDurableStateRoot(filepath.Join(stateBase, "dockpipe"), "linux")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projectStateRootAt(project, root); err == nil {
		t.Fatal("Dockpipe-owned root symlink was followed")
	}
}

func TestCanonicalDurableStateRootRejectsDanglingBase(t *testing.T) {
	base := t.TempDir()
	link := filepath.Join(base, "state")
	if err := os.Symlink(filepath.Join(base, "missing"), link); err != nil {
		t.Fatal(err)
	}
	if _, err := canonicalDurableStateRoot(filepath.Join(link, "dockpipe"), "linux"); err == nil {
		t.Fatal("dangling OS base accepted")
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

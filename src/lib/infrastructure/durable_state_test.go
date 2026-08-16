package infrastructure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

func TestDurableStateRootForSupportedOperatingSystems(t *testing.T) {
	tests := []struct {
		name string
		goos string
		env  map[string]string
		want []string
	}{
		{name: "linux xdg", goos: "linux", env: map[string]string{"HOME": "/home/me", "XDG_STATE_HOME": "/state"}, want: []string{"state", "dockpipe"}},
		{name: "linux fallback", goos: "linux", env: map[string]string{"HOME": "/home/me"}, want: []string{"home", "me", ".local", "state", "dockpipe"}},
		{name: "macos", goos: "darwin", env: map[string]string{"HOME": "/Users/me"}, want: []string{"Users", "me", "Library", "Application Support", "dockpipe", "state"}},
		{name: "windows local app data", goos: "windows", env: map[string]string{"HOME": `C:\Users\me`, "LOCALAPPDATA": `C:\Users\me\AppData\Local`}, want: []string{"AppData", "Local", "dockpipe", "state"}},
		{name: "windows fallback", goos: "windows", env: map[string]string{"HOME": `C:\Users\me`}, want: []string{"AppData", "Local", "dockpipe", "state"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := durableStateRootFor(test.goos, test.env)
			if err != nil {
				t.Fatal(err)
			}
			for _, segment := range test.want {
				if !strings.Contains(got, segment) {
					t.Fatalf("root %q does not contain %q", got, segment)
				}
			}
		})
	}
}

func TestDurableStateRootRejectsRelativeBases(t *testing.T) {
	for _, test := range []struct {
		goos string
		env  map[string]string
	}{
		{goos: "linux", env: map[string]string{"HOME": "/home/me", "XDG_STATE_HOME": "relative/state"}},
		{goos: "darwin", env: map[string]string{"HOME": "relative/home"}},
		{goos: "windows", env: map[string]string{"HOME": `C:\Users\me`, "LOCALAPPDATA": `relative\state`}},
	} {
		if _, err := durableStateRootFor(test.goos, test.env); err == nil {
			t.Fatalf("%s relative durable-state base unexpectedly passed validation", test.goos)
		}
	}
}

func TestProjectStateRootStableAcrossAliasAndRenameButNotCopy(t *testing.T) {
	base := t.TempDir()
	stateRoot := filepath.Join(base, "state")
	project := filepath.Join(base, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}

	first, err := projectStateRootAt(project, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(filepath.Base(first)) != 32 {
		t.Fatalf("project ID %q is not a random 128-bit hex value", filepath.Base(first))
	}

	alias := filepath.Join(base, "project-alias")
	if err := os.Symlink(project, alias); err == nil {
		fromAlias, err := projectStateRootAt(alias, stateRoot)
		if err != nil {
			t.Fatal(err)
		}
		if fromAlias != first {
			t.Fatalf("symlink alias changed project identity: %q != %q", fromAlias, first)
		}
	} else if runtime.GOOS != "windows" {
		t.Fatal(err)
	}

	renamed := filepath.Join(base, "renamed-project")
	if err := os.Rename(project, renamed); err != nil {
		t.Fatal(err)
	}
	afterRename, err := projectStateRootAt(renamed, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if afterRename != first {
		t.Fatalf("same-filesystem rename changed project identity: %q != %q", afterRename, first)
	}

	copyPath := filepath.Join(base, "copied-project")
	if err := os.Mkdir(copyPath, 0o755); err != nil {
		t.Fatal(err)
	}
	copyRoot, err := projectStateRootAt(copyPath, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if copyRoot == first {
		t.Fatal("copied checkout reused the original project identity")
	}

	var index durableProjectIndex
	readTestJSON(t, filepath.Join(stateRoot, durableProjectIndexFile), &index)
	if len(index.Projects) != 2 {
		t.Fatalf("got %d project identities, want 2", len(index.Projects))
	}
	foundRenamed := false
	for _, record := range index.Projects {
		if record.ProjectID == filepath.Base(first) {
			foundRenamed = sameDurablePath(record.CanonicalPath, renamed)
		}
	}
	if !foundRenamed {
		t.Fatal("renamed checkout did not update its canonical path")
	}
}

func TestProjectStateRootConcurrentResolutionUsesOneIdentity(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	stateRoot := filepath.Join(base, "state")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	const workers = 12
	results := make(chan string, workers)
	errors := make(chan error, workers)
	var wait sync.WaitGroup
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			root, err := projectStateRootAt(project, stateRoot)
			if err != nil {
				errors <- err
				return
			}
			results <- root
		}()
	}
	wait.Wait()
	close(results)
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
	want := ""
	for result := range results {
		if want == "" {
			want = result
		}
		if result != want {
			t.Fatalf("concurrent resolution returned %q and %q", want, result)
		}
	}
	var index durableProjectIndex
	readTestJSON(t, filepath.Join(stateRoot, durableProjectIndexFile), &index)
	if len(index.Projects) != 1 {
		t.Fatalf("got %d project identities, want 1", len(index.Projects))
	}
}

func TestProjectPackageStateDirUsesCollisionSafeOwnerIdentity(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(base, "state")
	first, err := projectPackageStateDirAt(project, "Package.One/component/Worker", stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	second, err := projectPackageStateDirAt(project, "package-one/component/worker", stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	legacyCollision, err := projectPackageStateDirAt(project, "package.one-component-worker", stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || second == legacyCollision || first == legacyCollision {
		t.Fatalf("distinct case-preserving owner IDs collided: %q %q %q", first, second, legacyCollision)
	}
	for _, path := range []string{first, second, legacyCollision} {
		base := filepath.Base(path)
		parts := strings.Split(base, "-")
		if len(parts) < 2 || len(parts[len(parts)-1]) != 64 {
			t.Fatalf("package path %q does not include a full SHA-256", path)
		}
	}
}

func TestDurableOwnerIdentityValidation(t *testing.T) {
	invalidUTF8 := string([]byte{0xff})
	for _, owner := range []string{"", "   ", "default", "has\x00nul", invalidUTF8, "decomposed-e\u0301", "caf\u00e9"} {
		if utf8.ValidString(owner) && strings.Contains(owner, `\x00`) {
			owner = strings.ReplaceAll(owner, `\x00`, "\x00")
		}
		if _, _, _, err := durableOwnerStorageIdentity(owner); err == nil {
			t.Fatalf("owner %q unexpectedly passed validation", owner)
		}
	}
	canonical, _, _, err := durableOwnerStorageIdentity("  Acme.Tools/provider-pool/app-server  ")
	if err != nil {
		t.Fatal(err)
	}
	if canonical != "Acme.Tools/provider-pool/app-server" {
		t.Fatalf("canonical owner ID got %q", canonical)
	}
}

func TestPackageRuntimeDirIsDisposableAndSeparate(t *testing.T) {
	project := t.TempDir()
	runtimeDir, err := PackageRuntimeDir(project, "example.tools/provider-pool")
	if err != nil {
		t.Fatal(err)
	}
	stateRoot, err := StateRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	if !sameOrWithinDurablePath(stateRoot, runtimeDir) || !strings.Contains(runtimeDir, "packages-runtime") {
		t.Fatalf("runtime directory %q is not under disposable state root %q", runtimeDir, stateRoot)
	}
	if _, err := os.Lstat(runtimeDir); !os.IsNotExist(err) {
		t.Fatalf("PackageRuntimeDir mutated the checkout: %v", err)
	}
}

func TestPreparePrivateStateSubdirectoryIsBoundedAndOwnerOnly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	destination, err := PreparePrivateStateSubdirectory(root, "one/two")
	if err != nil {
		t.Fatal(err)
	}
	if destination != filepath.Join(root, "one", "two") {
		t.Fatalf("private subdirectory = %q", destination)
	}
	if info, err := os.Stat(destination); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("private subdirectory mode = %v, %v", info, err)
	}
	if _, err := PreparePrivateStateSubdirectory(root, "../escape"); err == nil {
		t.Fatal("private subdirectory accepted traversal")
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := PreparePrivateStateSubdirectory(root, "linked/child"); err == nil {
		t.Fatal("private subdirectory followed a link")
	}
}

func TestDiscoverLegacyPackageStateIsReadOnly(t *testing.T) {
	project := t.TempDir()
	stateRoot, err := StateRoot(project)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(stateRoot, "packages", "example-dev-stack")
	path, found, err := DiscoverLegacyPackageState(project, "Example Dev/Stack")
	if err != nil {
		t.Fatal(err)
	}
	if found || path != want {
		t.Fatalf("absent discovery got path=%q found=%v", path, found)
	}
	if _, err := os.Lstat(stateRoot); !os.IsNotExist(err) {
		t.Fatalf("legacy discovery created state: %v", err)
	}
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}
	path, found, err = DiscoverLegacyPackageState(project, "Example Dev/Stack")
	if err != nil {
		t.Fatal(err)
	}
	if !found || path != want {
		t.Fatalf("present discovery got path=%q found=%v", path, found)
	}
}

func TestJoinStatePathRejectsTraversalAbsoluteAndReservedNames(t *testing.T) {
	root := t.TempDir()
	for _, suffix := range []string{"../escape", "a/../../escape", "/absolute", `C:\absolute`, "a//b", "a/./b", "a/../b", "a:stream", "CON", "aux.txt", "name.", "name ", "a\\b"} {
		if _, err := JoinStatePath(root, suffix); err == nil {
			t.Fatalf("suffix %q unexpectedly passed validation", suffix)
		}
	}
	got, err := JoinStatePath(root, "sessions/one.json")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(root, "sessions", "one.json") {
		t.Fatalf("got %q", got)
	}
}

func TestMalformedProjectIndexFailsClosed(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	stateRoot := filepath.Join(base, "state")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ensurePrivateRoot(stateRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateRoot, durableProjectIndexFile), []byte(`{"schema":1,"projects":[{"project_id":"bad"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := projectStateRootAt(project, stateRoot); err == nil {
		t.Fatal("malformed project index did not fail closed")
	}
}

func TestMalformedProjectMetadataFailsClosed(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "project")
	stateRoot := filepath.Join(base, "state")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	projectRoot, err := projectStateRootAt(project, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	metadataPath := filepath.Join(projectRoot, durableProjectMetaFile)
	if err := os.WriteFile(metadataPath, []byte(`{"schema":1,"project_id":"00000000000000000000000000000000","canonical_path":"/substituted","path_aliases":[],"filesystem_identity":"substituted"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := projectStateRootAt(project, stateRoot); err == nil {
		t.Fatal("substituted project metadata did not fail closed")
	}
}

func readTestJSON(t *testing.T, path string, target any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
}

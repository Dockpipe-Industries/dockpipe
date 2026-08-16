package application

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestCmdCleanDryRunAndRemovalCoverWholeDisposableTreeOnly(t *testing.T) {
	base := t.TempDir()
	workdir := filepath.Join(base, "checkout")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "durable"))
	t.Setenv("DOCKPIPE_PACKAGE_STATE_DIR", "")
	durable, err := infrastructure.PackageStateDir(workdir, "clean-survival")
	if err != nil {
		t.Fatal(err)
	}
	durableMarker := filepath.Join(durable, "authority")
	if err := os.WriteFile(durableMarker, []byte("durable\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	externalStore := filepath.Join(base, "external-compiled-store")
	if err := os.MkdirAll(externalStore, 0o755); err != nil {
		t.Fatal(err)
	}
	externalMarker := filepath.Join(externalStore, "marker")
	if err := os.WriteFile(externalMarker, []byte("external\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", externalStore)
	externalEventLog := filepath.Join(base, "external-events", "events.jsonl")
	t.Setenv(infrastructure.EnvDockpipeEventLog, externalEventLog)
	files := map[string]string{
		filepath.Join(workdir, infrastructure.DockpipeDirRel, "internal", "cache", "cache.txt"):  "cache\n",
		filepath.Join(workdir, infrastructure.DockpipeDirRel, "packages", "legacy", "state.txt"): "legacy\n",
		filepath.Join(workdir, infrastructure.DockpipeDirRel, "runs", "run.txt"):                 "run\n",
	}
	var logicalBytes int
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		logicalBytes += len(content)
	}
	root := filepath.Join(workdir, infrastructure.DockpipeDirRel)
	wantPreview := fmt.Sprintf("dry_run=true target=%q logical_bytes=%d files=3 action=remove", root, logicalBytes)
	first, err := captureStdout(t, func() error { return cmdClean([]string{"--workdir", workdir, "--dry-run"}) })
	if err != nil {
		t.Fatal(err)
	}
	second, err := captureStdout(t, func() error { return cmdClean([]string{"--dry-run", "--workdir", workdir}) })
	if err != nil {
		t.Fatal(err)
	}
	if first != wantPreview || second != wantPreview {
		t.Fatalf("dry-run output is not deterministic:\nfirst:  %q\nsecond: %q\nwant:   %q", first, second, wantPreview)
	}
	for path := range files {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("dry-run mutated %q: %v", path, err)
		}
	}
	if err := cmdClean([]string{"--workdir", workdir}); err != nil {
		t.Fatalf("cmdClean: %v", err)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatalf("expected whole disposable tree removed, stat err=%v", err)
	}
	for _, marker := range []string{durableMarker, externalMarker} {
		if _, err := os.Lstat(marker); err != nil {
			t.Fatalf("ordinary clean touched protected marker %q: %v", marker, err)
		}
	}
	if _, err := os.Lstat(externalEventLog); !os.IsNotExist(err) {
		t.Fatalf("ordinary clean wrote inherited external event log: %v", err)
	}
}

func TestCmdCleanMissingTreeIsReportedNoop(t *testing.T) {
	workdir := t.TempDir()
	t.Setenv("HOME", filepath.Join(t.TempDir(), "home"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "durable"))
	root := filepath.Join(workdir, infrastructure.DockpipeDirRel)
	out, err := captureStdout(t, func() error { return cmdClean([]string{"--workdir", workdir, "--dry-run"}) })
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("dry_run=true target=%q logical_bytes=0 files=0 action=noop", root)
	if out != want {
		t.Fatalf("missing-tree preview = %q want %q", out, want)
	}
	stderr, err := captureResultStderr(t, func() error { return cmdClean([]string{"--workdir", workdir}) })
	if err != nil {
		t.Fatal(err)
	}
	wantResult := fmt.Sprintf("dry_run=false target=%q logical_bytes=0 files=0 action=noop", root)
	if strings.TrimSpace(stderr) != wantResult {
		t.Fatalf("missing-tree clean did not report exact no-op target: %s", stderr)
	}
}

func TestCmdBuildDelegatesToCompileAll(t *testing.T) {
	if err := cmdBuild([]string{"--help"}); err != nil {
		t.Fatalf("cmdBuild --help: %v", err)
	}
}

func TestCmdRebuildHelp(t *testing.T) {
	if err := cmdRebuild([]string{"--help"}); err != nil {
		t.Fatalf("cmdRebuild --help: %v", err)
	}
}

func TestCleanAndRebuildCompiledStoreResetKeepSeparateAuthority(t *testing.T) {
	base := t.TempDir()
	workdir := filepath.Join(base, "checkout")
	defaultRoot := filepath.Join(workdir, infrastructure.DockpipeDirRel)
	overrideRoot := filepath.Join(base, "external-compiled-store")
	for _, root := range []string{defaultRoot, overrideRoot} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "marker"), []byte("compiled\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "durable"))
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", overrideRoot)
	if err := cmdClean([]string{"--workdir", workdir}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(defaultRoot); !os.IsNotExist(err) {
		t.Fatalf("ordinary clean did not remove checkout disposable tree: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(overrideRoot, "marker")); err != nil {
		t.Fatalf("ordinary clean touched external override: %v", err)
	}
	if err := resetCompiledPackagesRoot(workdir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(overrideRoot); !os.IsNotExist(err) {
		t.Fatalf("rebuild reset did not remove isolated override: %v", err)
	}
}

func TestCleanTargetRejectsTraversalRootsAncestorsAndDurableState(t *testing.T) {
	base := t.TempDir()
	workdir := filepath.Join(base, "checkout")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(base, "home"))
	xdg := filepath.Join(base, "xdg")
	t.Setenv("XDG_STATE_HOME", xdg)
	for name, stateRel := range map[string]string{
		"absolute":  string(filepath.Separator),
		"traversal": filepath.Join("..", "escape"),
		"workdir":   ".",
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateCleanStateRelativePath(stateRel); err == nil || !strings.Contains(err.Error(), "refusing") {
				t.Fatalf("unsafe state path %q was accepted: %v", stateRel, err)
			}
		})
	}
	if _, err := validateCheckoutCleanTarget(string(filepath.Separator), filepath.Join(string(filepath.Separator), infrastructure.DockpipeDirRel)); err == nil || !strings.Contains(err.Error(), "filesystem-root") {
		t.Fatalf("filesystem-root workdir was accepted: %v", err)
	}
	for name, target := range map[string]string{
		"workdir":           workdir,
		"workdir-ancestor":  filepath.Dir(workdir),
		"nonstandard-child": filepath.Join(workdir, "other"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := validateCheckoutCleanTarget(workdir, target); err == nil || !strings.Contains(err.Error(), "refusing") {
				t.Fatalf("unsafe clean target %q was accepted: %v", target, err)
			}
		})
	}
	durableWorkdir := filepath.Join(xdg, "dockpipe", "projects", "checkout")
	if err := os.MkdirAll(durableWorkdir, 0o755); err != nil {
		t.Fatal(err)
	}
	durableTarget := filepath.Join(durableWorkdir, infrastructure.DockpipeDirRel)
	if _, err := validateCheckoutCleanTarget(durableWorkdir, durableTarget); err == nil || !strings.Contains(err.Error(), "durable state") {
		t.Fatalf("durable-state target was accepted: %v", err)
	}
	if _, _, err := parseCleanArgs([]string{"--workdir", filepath.Join(base, "checkout") + string(filepath.Separator) + ".." + string(filepath.Separator) + "other"}); err == nil || !strings.Contains(err.Error(), "traversal") {
		t.Fatalf("traversing --workdir was accepted: %v", err)
	}
}

func TestCmdCleanRejectsLinkedTreeWithoutMutation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink setup requires platform privileges on Windows")
	}
	base := t.TempDir()
	workdir := filepath.Join(base, "checkout")
	root := filepath.Join(workdir, infrastructure.DockpipeDirRel)
	external := filepath.Join(base, "external")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(external, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(external, "marker")
	if err := os.WriteFile(marker, []byte("preserve\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "substitution")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "durable"))
	if err := cmdClean([]string{"--workdir", workdir, "--dry-run"}); err == nil || !strings.Contains(err.Error(), "linked") {
		t.Fatalf("linked clean tree was accepted: %v", err)
	}
	if _, err := os.Lstat(marker); err != nil {
		t.Fatalf("linked external marker changed: %v", err)
	}
}

func TestRebuildCompiledStoreResetRejectsDangerousTargets(t *testing.T) {
	base := t.TempDir()
	workdir := filepath.Join(base, "checkout", "project")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "home")
	durable := filepath.Join(base, "durable")
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", durable)
	for name, target := range map[string]string{
		"filesystem-root":  string(filepath.Separator),
		"home":             home,
		"workdir":          workdir,
		"workdir-ancestor": filepath.Dir(workdir),
		"durable-root":     filepath.Join(durable, "dockpipe"),
		"inside-durable":   filepath.Join(durable, "dockpipe", "compiled"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("DOCKPIPE_PACKAGES_ROOT", target)
			if err := resetCompiledPackagesRoot(workdir); err == nil || !strings.Contains(err.Error(), "refusing") {
				t.Fatalf("dangerous reset target %q was accepted: %v", target, err)
			}
		})
	}
}

func TestRebuildCompiledStoreResetRejectsLinkedTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink setup requires platform privileges on Windows")
	}
	base := t.TempDir()
	workdir := filepath.Join(base, "checkout")
	realRoot := filepath.Join(base, "real-store")
	linkedRoot := filepath.Join(base, "linked-store")
	if err := os.MkdirAll(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realRoot, "marker"), []byte("preserve\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realRoot, linkedRoot); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "durable"))
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", linkedRoot)
	if err := resetCompiledPackagesRoot(workdir); err == nil || !strings.Contains(err.Error(), "link") {
		t.Fatalf("linked reset target was accepted: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(realRoot, "marker")); err != nil {
		t.Fatalf("linked target bytes changed: %v", err)
	}
}

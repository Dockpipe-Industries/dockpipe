package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInternalStatePrepareDurableCohort(t *testing.T) {
	base := t.TempDir()
	workdir := filepath.Join(base, "checkout")
	legacy := filepath.Join(workdir, "bin", ".dockpipe", "state", "vmimage")
	if err := os.MkdirAll(filepath.Join(legacy, "identity"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "identity", "disk.uuid"), []byte("stable-uuid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "durable"))
	t.Setenv("HOME", filepath.Join(base, "home"))
	out, err := captureStdout(t, func() error {
		return cmdInternalState([]string{
			"prepare-durable-cohort",
			"--workdir", workdir,
			"--owner", "vm/runtime/vmimage",
			"--cohort", "vm-durable-guest-identity-v1",
			"--instance", filepath.Join(workdir, "disk.qcow2"),
			"--run", "run-1",
			"--legacy-root", legacy,
			"--file", "identity/disk.uuid=identity/machine.uuid",
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Split(strings.TrimSuffix(out, "\n"), "\t")
	if len(fields) != 4 || fields[2] != "true" || fields[3] != "false" {
		t.Fatalf("unexpected internal state result: %q", out)
	}
	if strings.HasPrefix(fields[0], filepath.Join(workdir, "bin", ".dockpipe")) {
		t.Fatalf("durable result stayed under checkout runtime: %s", fields[0])
	}
	if !strings.HasPrefix(fields[1], filepath.Join(workdir, "bin", ".dockpipe", "packages-runtime")) {
		t.Fatalf("runtime result did not use package runtime root: %s", fields[1])
	}
	raw, err := os.ReadFile(filepath.Join(fields[0], "identity", "machine.uuid"))
	if err != nil || string(raw) != "stable-uuid\n" {
		t.Fatalf("imported identity = %q, %v", raw, err)
	}
}

func TestInternalStateIsHiddenAndRejectsMalformedMappings(t *testing.T) {
	if err := cmdInternalState([]string{"--help"}); err == nil || !strings.Contains(err.Error(), "unknown internal") {
		t.Fatalf("internal state unexpectedly exposed help: %v", err)
	}
	_, err := parseInternalDurableCohortArgs([]string{
		"--workdir", t.TempDir(),
		"--owner", "vm/runtime/vmimage",
		"--cohort", "vm-durable-guest-identity-v1",
		"--instance", "guest",
		"--run", "run",
		"--legacy-root", "legacy",
		"--file", "missing-destination",
	})
	if err == nil || !strings.Contains(err.Error(), "source=destination") {
		t.Fatalf("malformed mapping was accepted: %v", err)
	}
}

func TestInternalStatePackageRuntimeIsCollisionSafeAndNonMutating(t *testing.T) {
	workdir := filepath.Join(t.TempDir(), "checkout")
	first, err := captureStdout(t, func() error {
		return cmdInternalState([]string{
			"package-runtime",
			"--workdir", workdir,
			"--owner", "Package.One/component/Worker",
			"--path", "ci/analysis",
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := captureStdout(t, func() error {
		return cmdInternalState([]string{
			"package-runtime",
			"--workdir", workdir,
			"--owner", "package-one-component-worker",
			"--path", "ci/analysis",
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	first = strings.TrimSpace(first)
	second = strings.TrimSpace(second)
	if first == second {
		t.Fatalf("collision-prone owners shared runtime path %q", first)
	}
	wantPrefix := filepath.Join(workdir, "bin", ".dockpipe", "packages-runtime") + string(filepath.Separator)
	if !strings.HasPrefix(first, wantPrefix) || !strings.HasSuffix(first, filepath.Join("ci", "analysis")) {
		t.Fatalf("package runtime path = %q, want prefix %q and CI suffix", first, wantPrefix)
	}
	if _, err := os.Stat(filepath.Join(workdir, "bin", ".dockpipe")); !os.IsNotExist(err) {
		t.Fatalf("package runtime lookup mutated checkout state: %v", err)
	}
	if err := cmdInternalState([]string{
		"package-runtime",
		"--workdir", workdir,
		"--owner", "package-owner",
		"--path", "../durable",
	}); err == nil {
		t.Fatal("package runtime accepted traversal suffix")
	}
}

func TestInternalStatePackageRuntimeCanPreparePrivateRoot(t *testing.T) {
	workdir := filepath.Join(t.TempDir(), "checkout")
	stateRoot := filepath.Join(workdir, "bin", ".dockpipe")
	if err := os.MkdirAll(stateRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdout(t, func() error {
		return cmdInternalState([]string{
			"package-runtime",
			"--workdir", workdir,
			"--owner", "package/dev-stack",
			"--ensure-private",
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	runtimeRoot := strings.TrimSpace(out)
	info, err := os.Stat(runtimeRoot)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("private runtime mode = %04o, want 0700", info.Mode().Perm())
	}
	stateInfo, err := os.Stat(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if stateInfo.Mode().Perm() != 0o755 {
		t.Fatalf("state root mode changed to %04o", stateInfo.Mode().Perm())
	}
}

func TestInternalStatePrivateDirectoryRejectsLinksAndTraversal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdout(t, func() error {
		return cmdInternalState([]string{"private-directory", "--root", root, "--path", "home/config"})
	})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "home", "config")
	if strings.TrimSpace(out) != want {
		t.Fatalf("private directory = %q, want %q", out, want)
	}
	if info, err := os.Stat(want); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("private directory mode = %v, %v", info, err)
	}
	if err := cmdInternalState([]string{"private-directory", "--root", root, "--path", "../escape"}); err == nil {
		t.Fatal("private directory accepted traversal")
	}
	if err := cmdInternalState([]string{"private-directory", "--root", root, "--root", root, "--path", "child"}); err == nil {
		t.Fatal("private directory accepted duplicate roots")
	}
	linkedRoot := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(root, linkedRoot); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := cmdInternalState([]string{"private-directory", "--root", linkedRoot, "--path", "child"}); err == nil {
		t.Fatal("private directory accepted a linked root")
	}
}

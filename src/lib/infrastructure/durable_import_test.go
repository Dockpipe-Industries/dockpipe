package infrastructure

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestDurableCohortImportCopiesSelectedStateAndKeepsRuntimeDisposable(t *testing.T) {
	workdir, legacy, stateRoot := durableImportFixture(t)
	before := durableImportTreeDigest(t, legacy)
	status, err := PrepareDurableCohortImport(workdir, durableImportTestSpec(legacy, "guest-a", "run-a"))
	if err != nil {
		t.Fatal(err)
	}
	if !status.DurableAuthoritative || !status.ImportedLegacy || status.LegacyDiverged {
		t.Fatalf("unexpected migration status: %+v", status)
	}
	if sameOrWithinDurablePath(stateRoot, status.DurableDir) {
		t.Fatalf("durable state remained in checkout runtime root: %s", status.DurableDir)
	}
	if !sameOrWithinDurablePath(stateRoot, status.RuntimeDir) {
		t.Fatalf("runtime state escaped checkout runtime root: %s", status.RuntimeDir)
	}
	assertFileContents(t, filepath.Join(status.DurableDir, "identity", "machine.uuid"), "uuid-a\n")
	assertFileContents(t, filepath.Join(status.DurableDir, "firmware", "vars.fd"), "firmware-a")
	assertFileContents(t, filepath.Join(status.DurableDir, "credentials", "administrator-password.txt"), "secret-a\n")
	assertFileContents(t, filepath.Join(status.DurableDir, "tpm", "tpm2-00.permall"), "tpm-a")
	if _, err := os.Lstat(filepath.Join(status.DurableDir, "tpm", "swtpm.log")); !os.IsNotExist(err) {
		t.Fatalf("disposable TPM log was imported: %v", err)
	}
	if after := durableImportTreeDigest(t, legacy); after != before {
		t.Fatalf("legacy source changed: before=%x after=%x", before, after)
	}
	assertPrivateTree(t, status.DurableDir)
	assertPrivateTree(t, status.RuntimeDir)

	restart, err := PrepareDurableCohortImport(workdir, durableImportTestSpec(legacy, "guest-a", "run-b"))
	if err != nil {
		t.Fatal(err)
	}
	if restart.DurableDir != status.DurableDir || restart.RuntimeDir == status.RuntimeDir || !restart.DurableAuthoritative || restart.ImportedLegacy {
		t.Fatalf("restart did not retain durable identity and rotate runtime: first=%+v restart=%+v", status, restart)
	}
	other, err := PrepareDurableCohortImport(workdir, durableImportTestSpec(legacy, "guest-b", "run-a"))
	if err != nil {
		t.Fatal(err)
	}
	if other.DurableDir == status.DurableDir || other.RuntimeDir == status.RuntimeDir {
		t.Fatal("distinct guest identities collided")
	}
}

func TestDurableCohortImportDurableWinsAndRejectsUnsafeAuthority(t *testing.T) {
	workdir, legacy, _ := durableImportFixture(t)
	spec := durableImportTestSpec(legacy, "guest-a", "run-a")
	status, err := PrepareDurableCohortImport(workdir, spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "identity", "disk.uuid"), []byte("legacy-diverged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := PrepareDurableCohortImport(workdir, spec)
	if err != nil {
		t.Fatal(err)
	}
	if !again.LegacyDiverged {
		t.Fatal("legacy divergence was not reported")
	}
	assertFileContents(t, filepath.Join(status.DurableDir, "identity", "machine.uuid"), "uuid-a\n")

	manifest := filepath.Join(status.DurableDir, durableImportManifestName)
	if err := os.WriteFile(manifest, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareDurableCohortImport(workdir, spec); err == nil || !strings.Contains(err.Error(), "manifest") {
		t.Fatalf("malformed durable provenance did not fail closed: %v", err)
	}
}

func TestDurableCohortImportRejectsLinksSubstitutionAndPermissionDrift(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX link and mode assertions")
	}
	t.Run("legacy-link", func(t *testing.T) {
		workdir, legacy, _ := durableImportFixture(t)
		path := filepath.Join(legacy, "identity", "disk.uuid")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(legacy, "firmware.fd"), path); err != nil {
			t.Fatal(err)
		}
		if _, err := PrepareDurableCohortImport(workdir, durableImportTestSpec(legacy, "guest-a", "run-a")); err == nil {
			t.Fatal("linked legacy source was accepted")
		}
	})
	t.Run("source-substitution", func(t *testing.T) {
		workdir, legacy, _ := durableImportFixture(t)
		path := filepath.Join(legacy, "identity", "disk.uuid")
		detached := path + ".detached"
		durableCohortImportTestHook = func(stage string) error {
			if stage != "after-source-inventory" {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := os.Rename(path, detached); err != nil {
				return err
			}
			return os.WriteFile(path, raw, 0o644)
		}
		defer func() { durableCohortImportTestHook = nil }()
		if _, err := PrepareDurableCohortImport(workdir, durableImportTestSpec(legacy, "guest-a", "run-a")); err == nil || !strings.Contains(err.Error(), "substituted") {
			t.Fatalf("identical-byte source substitution did not fail closed: %v", err)
		}
	})
	t.Run("permission-drift", func(t *testing.T) {
		workdir, legacy, _ := durableImportFixture(t)
		spec := durableImportTestSpec(legacy, "guest-a", "run-a")
		status, err := PrepareDurableCohortImport(workdir, spec)
		if err != nil {
			t.Fatal(err)
		}
		secret := filepath.Join(status.DurableDir, "credentials", "administrator-password.txt")
		if err := os.Chmod(secret, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := PrepareDurableCohortImport(workdir, spec); err == nil || !strings.Contains(err.Error(), "permissions") {
			t.Fatalf("unsafe durable permission was repaired or accepted: %v", err)
		}
		mode := mustLstat(t, secret).Mode().Perm()
		if mode != 0o644 {
			t.Fatalf("fail-closed validation mutated unsafe mode to %04o", mode)
		}
	})
	t.Run("runtime-permission-drift", func(t *testing.T) {
		workdir, legacy, _ := durableImportFixture(t)
		spec := durableImportTestSpec(legacy, "guest-a", "run-a")
		status, err := PrepareDurableCohortImport(workdir, spec)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(status.RuntimeDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := PrepareDurableCohortImport(workdir, spec); err == nil || !strings.Contains(err.Error(), "permissions") {
			t.Fatalf("unsafe runtime permission was repaired or accepted: %v", err)
		}
		if mode := mustLstat(t, status.RuntimeDir).Mode().Perm(); mode != 0o755 {
			t.Fatalf("runtime fail-closed validation mutated mode to %04o", mode)
		}
	})
	t.Run("durable-destination-link", func(t *testing.T) {
		workdir, legacy, _ := durableImportFixture(t)
		spec := durableImportTestSpec(legacy, "guest-a", "run-a")
		status, err := PrepareDurableCohortImport(workdir, spec)
		if err != nil {
			t.Fatal(err)
		}
		detached := status.DurableDir + ".detached"
		if err := os.Rename(status.DurableDir, detached); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(detached, status.DurableDir); err != nil {
			t.Fatal(err)
		}
		if _, err := PrepareDurableCohortImport(workdir, spec); err == nil {
			t.Fatal("linked durable destination was accepted")
		}
		if info := mustLstat(t, status.DurableDir); info.Mode()&os.ModeSymlink == 0 {
			t.Fatal("fail-closed validation changed the linked destination")
		}
	})
}

func TestDurableCohortImportRecoversOnlyByteProvenInterruptions(t *testing.T) {
	for _, test := range []struct {
		name  string
		stage string
	}{
		{name: "incomplete-copy", stage: "after-copy"},
		{name: "ready-before-rename", stage: "before-rename"},
		{name: "lost-acknowledgement", stage: "after-rename"},
	} {
		t.Run(test.name, func(t *testing.T) {
			workdir, legacy, _ := durableImportFixture(t)
			spec := durableImportTestSpec(legacy, "guest-a", "run-a")
			durableCohortImportTestHook = func(stage string) error {
				if stage == test.stage {
					return errors.New("injected interruption")
				}
				return nil
			}
			if _, err := PrepareDurableCohortImport(workdir, spec); err == nil || !strings.Contains(err.Error(), "injected interruption") {
				t.Fatalf("interruption was not observed: %v", err)
			}
			durableCohortImportTestHook = nil
			status, err := PrepareDurableCohortImport(workdir, spec)
			if err != nil {
				t.Fatal(err)
			}
			if !status.DurableAuthoritative {
				t.Fatal("restart did not establish durable authority")
			}
			assertFileContents(t, filepath.Join(status.DurableDir, "identity", "machine.uuid"), "uuid-a\n")
		})
	}
	durableCohortImportTestHook = nil
}

func TestDurableCohortImportConcurrentPreparationIsStable(t *testing.T) {
	workdir, legacy, _ := durableImportFixture(t)
	spec := durableImportTestSpec(legacy, "guest-a", "run-a")
	const workers = 8
	results := make(chan DurableCohortImportStatus, workers)
	errorsCh := make(chan error, workers)
	var wait sync.WaitGroup
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			status, err := PrepareDurableCohortImport(workdir, spec)
			if err != nil {
				errorsCh <- err
				return
			}
			results <- status
		}()
	}
	wait.Wait()
	close(results)
	close(errorsCh)
	for err := range errorsCh {
		t.Fatal(err)
	}
	durableDir := ""
	for result := range results {
		if durableDir == "" {
			durableDir = result.DurableDir
		}
		if result.DurableDir != durableDir {
			t.Fatalf("concurrent durable identities differed: %q != %q", result.DurableDir, durableDir)
		}
	}
}

func durableImportFixture(t *testing.T) (workdir, legacy, stateRoot string) {
	t.Helper()
	base := t.TempDir()
	workdir = filepath.Join(base, "checkout")
	if err := os.Mkdir(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "durable"))
	t.Setenv("HOME", filepath.Join(base, "home"))
	stateRoot = filepath.Join(workdir, DockpipeDirRel)
	legacy = filepath.Join(stateRoot, "state", "vmimage")
	for _, directory := range []string{filepath.Join(legacy, "identity"), filepath.Join(legacy, "tpm-old")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(legacy, "identity", "disk.uuid"):      "uuid-a\n",
		filepath.Join(legacy, "firmware.fd"):                "firmware-a",
		filepath.Join(legacy, "administrator-password.txt"): "secret-a\n",
		filepath.Join(legacy, "tpm-old", "tpm2-00.permall"): "tpm-a",
		filepath.Join(legacy, "tpm-old", "swtpm.log"):       "transient-log",
		filepath.Join(legacy, "overlay-run.qcow2"):          "transient-overlay",
	}
	for path, value := range files {
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return workdir, legacy, stateRoot
}

func durableImportTestSpec(legacy, instance, runID string) DurableCohortImportSpec {
	return DurableCohortImportSpec{
		OwnerID:    "vm/runtime/vmimage",
		Cohort:     "vm-durable-guest-identity-v1",
		InstanceID: instance,
		RunID:      runID,
		LegacyRoot: legacy,
		Mappings: []DurableImportMapping{
			{Source: "identity/disk.uuid", Destination: "identity/machine.uuid"},
			{Source: "firmware.fd", Destination: "firmware/vars.fd"},
			{Source: "administrator-password.txt", Destination: "credentials/administrator-password.txt"},
			{Source: "tpm-old", Destination: "tpm", Tree: true},
		},
		IgnorePaths: []string{"tpm-old/swtpm.log"},
	}
}

func durableImportTreeDigest(t *testing.T, root string) [sha256.Size]byte {
	t.Helper()
	hash := sha256.New()
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		fmt.Fprintf(hash, "%s|%s|%o|%d\n", filepath.ToSlash(rel), info.Mode().Type(), info.Mode().Perm(), info.Size())
		if info.Mode().IsRegular() {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			hash.Write(raw)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return digest
}

func assertFileContents(t *testing.T, path, want string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != want {
		t.Fatalf("%s = %q, want %q", path, raw, want)
	}
}

func assertPrivateTree(t *testing.T, root string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		want := os.FileMode(0o600)
		if info.IsDir() {
			want = 0o700
		}
		if info.Mode().Perm() != want {
			return fmt.Errorf("%s mode=%04o want=%04o", path, info.Mode().Perm(), want)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func mustLstat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

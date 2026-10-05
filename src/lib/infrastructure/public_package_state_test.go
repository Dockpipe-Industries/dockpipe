package infrastructure

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestPackageStateDirConservativelyImportsUnknownLegacyScope(t *testing.T) {
	workdir, legacy := publicPackageStateFixture(t, "Third.Party")
	before := durableImportTreeDigest(t, legacy)
	status, err := PreparePackageStateDir(workdir, "Third.Party")
	if err != nil {
		t.Fatal(err)
	}
	if !status.ImportedLegacy || status.PackageOwnedImport || status.LegacyDiverged {
		t.Fatalf("package state status = %+v", status)
	}
	if strings.HasPrefix(status.Dir, filepath.Join(workdir, DockpipeDirRel)) {
		t.Fatalf("public package state remained disposable: %s", status.Dir)
	}
	assertFileContents(t, filepath.Join(status.Dir, "settings", "value.json"), "legacy-value\n")
	assertFileContents(t, filepath.Join(status.Dir, "cache.bin"), "unknown-classification\n")
	if after := durableImportTreeDigest(t, legacy); after != before {
		t.Fatal("legacy public package state changed during import")
	}
	assertPrivateTree(t, status.Dir)

	if err := os.WriteFile(filepath.Join(legacy, "cache.bin"), []byte("diverged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	restart, err := PreparePackageStateDir(workdir, "Third.Party")
	if err != nil {
		t.Fatal(err)
	}
	if !restart.LegacyDiverged || restart.ImportedLegacy {
		t.Fatalf("durable-wins restart status = %+v", restart)
	}
	assertFileContents(t, filepath.Join(status.Dir, "cache.bin"), "unknown-classification\n")
}

func TestPackageStateDirLeavesDeclaredMixedPackageToExactCohortImporter(t *testing.T) {
	workdir, legacy := publicPackageStateFixture(t, "maintained-tool")
	packages := filepath.Join(workdir, "package-sources")
	if err := os.MkdirAll(filepath.Join(packages, "maintained"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := `{"schema":1,"compile":{"workflows":["package-sources"]}}`
	if err := os.WriteFile(filepath.Join(workdir, domain.DockpipeProjectConfigFileName), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "schema: 1\nname: maintained\npackage_state:\n  compatibility_import: package-owned\n  owner_ids: [maintained-tool]\n"
	if err := os.WriteFile(filepath.Join(packages, "maintained", PackageManifestFilename), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	before := durableImportTreeDigest(t, legacy)
	status, err := PreparePackageStateDir(workdir, "maintained-tool")
	if err != nil {
		t.Fatal(err)
	}
	if !status.PackageOwnedImport || status.ImportedLegacy {
		t.Fatalf("package-owned status = %+v", status)
	}
	if _, err := os.Lstat(filepath.Join(status.Dir, "cache.bin")); !os.IsNotExist(err) {
		t.Fatalf("mixed package was imported whole: %v", err)
	}
	cohort, err := PrepareDurableCohortImport(workdir, DurableCohortImportSpec{
		OwnerID:    "maintained/component",
		Cohort:     "settings-v1",
		InstanceID: "workspace",
		RunID:      "run",
		LegacyRoot: status.Dir,
		Mappings:   []DurableImportMapping{{Source: "settings", Destination: "settings", Tree: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertFileContents(t, filepath.Join(cohort.DurableDir, "settings", "value.json"), "legacy-value\n")
	if _, err := os.Lstat(filepath.Join(cohort.DurableDir, "cache.bin")); !os.IsNotExist(err) {
		t.Fatalf("disposable mixed-package entry was imported: %v", err)
	}
	if after := durableImportTreeDigest(t, legacy); after != before {
		t.Fatal("package-owned compatibility import changed legacy bytes")
	}
}

func TestPackageStateDirAcceptsExplicitPackageOwnedManifestContext(t *testing.T) {
	workdir, legacy := publicPackageStateFixture(t, "maintained-tool")
	packageRoot := filepath.Join(t.TempDir(), "maintained")
	if err := os.MkdirAll(packageRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "schema: 1\nname: maintained\npackage_state:\n  compatibility_import: package-owned\n  owner_ids: [maintained-tool]\n"
	manifestPath := filepath.Join(packageRoot, PackageManifestFilename)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	before := durableImportTreeDigest(t, legacy)
	status, err := PreparePackageStateDirWithManifests(workdir, "maintained-tool", manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !status.PackageOwnedImport || status.ImportedLegacy {
		t.Fatalf("explicit package-owned status = %+v", status)
	}
	if entries, err := collectWholePublicPackagePublished(status.Dir); err != nil || len(entries) != 0 {
		t.Fatalf("explicit mixed policy imported public state whole: entries=%v err=%v", entries, err)
	}
	if after := durableImportTreeDigest(t, legacy); after != before {
		t.Fatal("explicit package policy changed legacy bytes")
	}
}

func TestPackageStateDirIsCollisionSafeAndPackagesRootIndependent(t *testing.T) {
	base := t.TempDir()
	workdir := filepath.Join(base, "checkout")
	if err := os.Mkdir(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "durable"))
	t.Setenv("HOME", filepath.Join(base, "home"))
	t.Setenv(envPackagesRoot, filepath.Join(base, "external-compiled-store"))
	first, err := PackageStateDir(workdir, "Package.One/component/Worker")
	if err != nil {
		t.Fatal(err)
	}
	second, err := PackageStateDir(workdir, "package-one-component-worker")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("collision-prone owners shared durable public state %q", first)
	}
	for _, path := range []string{first, second} {
		if strings.Contains(path, "external-compiled-store") || strings.HasPrefix(path, workdir) {
			t.Fatalf("compiled-store override affected durable package state: %s", path)
		}
	}
}

func TestValidatePackageStateOverrideRequiresResolvedOrIndependentlyPrivateState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX ownership and mode assertions")
	}
	base := t.TempDir()
	workdir := filepath.Join(base, "checkout")
	if err := os.Mkdir(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "durable"))
	t.Setenv("HOME", filepath.Join(base, "home"))
	resolved, err := PackageStateDir(workdir, "third-party")
	if err != nil {
		t.Fatal(err)
	}
	privateOverride := filepath.Join(base, "private-override")
	if err := os.Mkdir(privateOverride, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, candidate := range map[string]string{
		"resolved": resolved,
		"private":  privateOverride,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidatePackageStateOverride(workdir, candidate, resolved); err != nil {
				t.Fatalf("safe override rejected: %v", err)
			}
		})
	}
	broad := filepath.Join(base, "broad")
	if err := os.Mkdir(broad, 0o755); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(base, "linked")
	if err := os.Symlink(privateOverride, linked); err != nil {
		t.Fatal(err)
	}
	for name, candidate := range map[string]string{
		"relative": "relative/state",
		"checkout": filepath.Join(workdir, "state"),
		"broad":    broad,
		"linked":   linked,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidatePackageStateOverride(workdir, candidate, resolved); err == nil {
				t.Fatalf("unsafe override %q was accepted", candidate)
			}
		})
	}
}

func TestPackageStateDirRecoversByteProvenInterruptions(t *testing.T) {
	for _, stage := range []string{"after-copy", "before-rename", "after-rename"} {
		t.Run(stage, func(t *testing.T) {
			workdir, _ := publicPackageStateFixture(t, "third-party")
			publicPackageStateImportTestHook = func(observed string) error {
				if observed == stage {
					return errors.New("injected interruption")
				}
				return nil
			}
			if _, err := PackageStateDir(workdir, "third-party"); err == nil || !strings.Contains(err.Error(), "injected interruption") {
				t.Fatalf("interruption was not observed: %v", err)
			}
			publicPackageStateImportTestHook = nil
			root, err := PackageStateDir(workdir, "third-party")
			if err != nil {
				t.Fatal(err)
			}
			assertFileContents(t, filepath.Join(root, "settings", "value.json"), "legacy-value\n")
		})
	}
	publicPackageStateImportTestHook = nil
}

func TestPackageStateDirRejectsUnsafeLegacyAndDurableState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX link and mode assertions")
	}
	t.Run("legacy-link", func(t *testing.T) {
		workdir, legacy := publicPackageStateFixture(t, "third-party")
		if err := os.Symlink(filepath.Join(legacy, "cache.bin"), filepath.Join(legacy, "linked")); err != nil {
			t.Fatal(err)
		}
		if _, err := PackageStateDir(workdir, "third-party"); err == nil || !strings.Contains(err.Error(), "linked") {
			t.Fatalf("linked legacy state was accepted: %v", err)
		}
	})
	t.Run("durable-permissions", func(t *testing.T) {
		workdir, _ := publicPackageStateFixture(t, "third-party")
		root, err := PackageStateDir(workdir, "third-party")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := PackageStateDir(workdir, "third-party"); err == nil || !strings.Contains(err.Error(), "permissions") {
			t.Fatalf("unsafe durable permissions were accepted: %v", err)
		}
	})
}

func publicPackageStateFixture(t *testing.T, ownerID string) (string, string) {
	t.Helper()
	base := t.TempDir()
	workdir := filepath.Join(base, "checkout")
	if err := os.Mkdir(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "durable"))
	t.Setenv("HOME", filepath.Join(base, "home"))
	legacy := filepath.Join(workdir, DockpipeDirRel, "packages", SanitizePackageStateScope(ownerID))
	if err := os.MkdirAll(filepath.Join(legacy, "settings"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "settings", "value.json"), []byte("legacy-value\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "cache.bin"), []byte("unknown-classification\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return workdir, legacy
}

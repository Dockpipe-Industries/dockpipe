package statepaths

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestLearningAuthorityMigrationCopiesOnlyDurableCohortWithoutMutatingLegacy(t *testing.T) {
	workdir, legacy, expected := learningAuthorityLegacyFixture(t)
	legacyModes := map[string]os.FileMode{}
	for rel := range expected {
		info, err := os.Lstat(filepath.Join(legacy, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		legacyModes[rel] = info.Mode().Perm()
	}

	durable, status, err := PrepareLearningAuthority(workdir)
	if err != nil {
		t.Fatal(err)
	}
	if !status.DurableAuthoritative || !status.ImportedLegacy || status.LegacyDiverged {
		t.Fatalf("unexpected migration status: %+v", status)
	}
	if strings.HasPrefix(filepath.Clean(durable), filepath.Clean(workdir)+string(filepath.Separator)) {
		t.Fatalf("durable learning authority remained inside checkout: %q", durable)
	}
	packageSegment := filepath.Base(filepath.Dir(durable))
	if !strings.HasPrefix(packageSegment, "dorkpipe-") || len(strings.TrimPrefix(packageSegment, "dorkpipe-")) != 64 {
		t.Fatalf("durable package owner is not collision-safe: %q", packageSegment)
	}

	for rel, want := range expected {
		got, err := os.ReadFile(filepath.Join(durable, filepath.FromSlash(rel)))
		if err != nil || string(got) != want {
			t.Fatalf("durable %s = %q, %v; want %q", rel, got, err, want)
		}
		legacyBytes, err := os.ReadFile(filepath.Join(legacy, filepath.FromSlash(rel)))
		if err != nil || string(legacyBytes) != want {
			t.Fatalf("legacy %s was mutated: %q, %v", rel, legacyBytes, err)
		}
		info, err := os.Lstat(filepath.Join(legacy, filepath.FromSlash(rel)))
		if err != nil || info.Mode().Perm() != legacyModes[rel] {
			t.Fatalf("legacy %s mode changed: %v, %v", rel, info, err)
		}
	}
	for _, excluded := range []string{"analysis/by-category/convention.json", "provider-pools/sessions.json", "self-analysis/facts.txt"} {
		if _, err := os.Lstat(filepath.Join(durable, filepath.FromSlash(excluded))); !os.IsNotExist(err) {
			t.Fatalf("disposable or separately owned legacy state %q was imported: %v", excluded, err)
		}
		if _, err := os.Lstat(filepath.Join(legacy, filepath.FromSlash(excluded))); err != nil {
			t.Fatalf("excluded legacy state %q was changed: %v", excluded, err)
		}
	}
	manifest, err := validateLearningAuthorityManifest(filepath.Join(durable, learningAuthorityProvenanceName), "authoritative")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Source != "legacy" || manifest.SourceIdentity == "" || len(manifest.Inventory) == 0 || manifest.InventorySHA256 != providerRecoveryInventoryDigest(manifest.Inventory) {
		t.Fatalf("migration provenance is incomplete: %+v", manifest)
	}
	for _, entry := range manifest.Inventory {
		if entry.Type != "directory" && entry.Type != "file" {
			t.Fatalf("migration inventory contains a special type: %+v", entry)
		}
	}
}

func TestLearningAuthorityMigrationDurableWinsAndSurvivesLegacyDetach(t *testing.T) {
	workdir, legacy, expected := learningAuthorityLegacyFixture(t)
	durable, _, err := PrepareLearningAuthority(workdir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "analysis", "history.jsonl"), []byte("{\"late\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	again, status, err := PrepareLearningAuthority(workdir)
	if err != nil {
		t.Fatal(err)
	}
	if again != durable || !status.DurableAuthoritative || status.ImportedLegacy || !status.LegacyDiverged {
		t.Fatalf("durable learning authority did not win divergence: %q %+v", again, status)
	}
	if err := os.Rename(legacy, legacy+".detached"); err != nil {
		t.Fatal(err)
	}
	restarted, status, err := PrepareLearningAuthority(workdir)
	if err != nil {
		t.Fatal(err)
	}
	if restarted != durable || !status.DurableAuthoritative || status.ImportedLegacy || status.LegacyDiverged {
		t.Fatalf("detached legacy changed durable authority: %q %+v", restarted, status)
	}
	for rel, want := range expected {
		got, err := os.ReadFile(filepath.Join(restarted, filepath.FromSlash(rel)))
		if err != nil || string(got) != want {
			t.Fatalf("restart lost cumulative %s: %q, %v", rel, got, err)
		}
	}
}

func TestLearningAuthorityMigrationFailsClosedOnMalformedOrSubstitutedState(t *testing.T) {
	t.Run("linked selected source", func(t *testing.T) {
		workdir, legacy, _ := learningAuthorityLegacyFixture(t)
		target := filepath.Join(workdir, "outside.json")
		if err := os.WriteFile(target, []byte("outside"), 0o644); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(legacy, "analysis", "insights.json")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, _, err := PrepareLearningAuthority(workdir); err == nil {
			t.Fatal("linked learning source was accepted")
		}
		assertNoLearningAuthority(t, workdir)
	})

	t.Run("unclassified selected subtree", func(t *testing.T) {
		workdir, legacy, _ := learningAuthorityLegacyFixture(t)
		if err := os.WriteFile(filepath.Join(legacy, "training", "unknown.json"), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := PrepareLearningAuthority(workdir); err == nil {
			t.Fatal("unclassified training state was accepted")
		}
		assertNoLearningAuthority(t, workdir)
	})

	t.Run("malformed provenance", func(t *testing.T) {
		workdir := isolatedDurableStateWorkdir(t)
		durable, _, err := PrepareLearningAuthority(workdir)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(durable, learningAuthorityProvenanceName), []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := PrepareLearningAuthority(workdir); err == nil {
			t.Fatal("malformed durable learning provenance was accepted")
		}
	})
}

func TestLearningAuthorityMigrationDetectsSourceAndDestinationSubstitution(t *testing.T) {
	t.Run("source", func(t *testing.T) {
		workdir, legacy, _ := learningAuthorityLegacyFixture(t)
		learningAuthorityMigrationTestHook = func(stage string) error {
			if stage == "after-source-inventory" {
				return os.WriteFile(filepath.Join(legacy, "metrics.jsonl"), []byte("{\"substituted\":true}\n"), 0o644)
			}
			return nil
		}
		t.Cleanup(func() { learningAuthorityMigrationTestHook = nil })
		if _, _, err := PrepareLearningAuthority(workdir); err == nil {
			t.Fatal("source substitution was accepted")
		}
		assertNoLearningAuthority(t, workdir)
	})

	t.Run("destination", func(t *testing.T) {
		workdir, legacy, _ := learningAuthorityLegacyFixture(t)
		packageRoot, err := infrastructure.ProjectPackageStateDir(workdir, dorkpipeScope)
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(packageRoot, learningAuthorityDestinationName)
		learningAuthorityMigrationTestHook = func(stage string) error {
			if stage == "before-rename" {
				return os.Mkdir(destination, 0o700)
			}
			return nil
		}
		t.Cleanup(func() { learningAuthorityMigrationTestHook = nil })
		if _, _, err := PrepareLearningAuthority(workdir); err == nil {
			t.Fatal("substituted durable destination was replaced")
		}
		if info, err := os.Lstat(destination); err != nil || !info.IsDir() {
			t.Fatalf("substituted destination was not preserved: %v, %v", info, err)
		}
		if raw, err := os.ReadFile(filepath.Join(legacy, "metrics.jsonl")); err != nil || len(raw) == 0 {
			t.Fatalf("destination substitution changed legacy authority: %q, %v", raw, err)
		}
	})
}

func TestLearningAuthorityMigrationRecoversInterruptedCopiesWithoutReplay(t *testing.T) {
	t.Run("ready temporary", func(t *testing.T) {
		workdir, _, _ := learningAuthorityLegacyFixture(t)
		interrupted := errors.New("simulated interruption before rename")
		copies := 0
		learningAuthorityMigrationTestHook = func(stage string) error {
			if strings.HasPrefix(stage, "copy-file:") {
				copies++
			}
			if stage == "before-rename" {
				return interrupted
			}
			return nil
		}
		t.Cleanup(func() { learningAuthorityMigrationTestHook = nil })
		if _, _, err := PrepareLearningAuthority(workdir); !errors.Is(err, interrupted) {
			t.Fatalf("first migration error = %v, want interruption", err)
		}
		firstCopies := copies
		learningAuthorityMigrationTestHook = func(stage string) error {
			if strings.HasPrefix(stage, "copy-file:") {
				copies++
			}
			return nil
		}
		durable, status, err := PrepareLearningAuthority(workdir)
		if err != nil {
			t.Fatal(err)
		}
		if copies != firstCopies || !status.DurableAuthoritative || !status.ImportedLegacy {
			t.Fatalf("ready temporary replayed or failed to resume: copies=%d status=%+v", copies, status)
		}
		if _, err := os.Stat(filepath.Join(durable, "training", "metrics.jsonl")); err != nil {
			t.Fatalf("resumed authority lost cumulative training: %v", err)
		}
	})

	t.Run("incomplete temporary", func(t *testing.T) {
		workdir, _, _ := learningAuthorityLegacyFixture(t)
		interrupted := errors.New("simulated interruption during copy")
		filesSeen := 0
		learningAuthorityMigrationTestHook = func(stage string) error {
			if strings.HasPrefix(stage, "copy-file:") {
				filesSeen++
				if filesSeen == 2 {
					return interrupted
				}
			}
			return nil
		}
		t.Cleanup(func() { learningAuthorityMigrationTestHook = nil })
		if _, _, err := PrepareLearningAuthority(workdir); !errors.Is(err, interrupted) {
			t.Fatalf("first migration error = %v, want interruption", err)
		}
		learningAuthorityMigrationTestHook = nil
		durable, status, err := PrepareLearningAuthority(workdir)
		if err != nil {
			t.Fatal(err)
		}
		if !status.DurableAuthoritative || !status.ImportedLegacy {
			t.Fatalf("incomplete temporary did not recover: %+v", status)
		}
		if _, err := os.Stat(filepath.Join(durable, "analysis", "history.jsonl")); err != nil {
			t.Fatalf("restarted migration lost history: %v", err)
		}
	})
}

func TestLearningAuthorityMigrationRejectsIdenticalByteSubstitutionAcrossRestart(t *testing.T) {
	workdir, legacy, _ := learningAuthorityLegacyFixture(t)
	interrupted := errors.New("simulated interruption before rename")
	learningAuthorityMigrationTestHook = func(stage string) error {
		if stage == "before-rename" {
			return interrupted
		}
		return nil
	}
	t.Cleanup(func() { learningAuthorityMigrationTestHook = nil })
	if _, _, err := PrepareLearningAuthority(workdir); !errors.Is(err, interrupted) {
		t.Fatalf("first migration error = %v, want interruption", err)
	}
	path := filepath.Join(legacy, "metrics.jsonl")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	replacement := path + ".replacement"
	if err := os.WriteFile(replacement, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	learningAuthorityMigrationTestHook = nil
	if _, _, err := PrepareLearningAuthority(workdir); err == nil {
		t.Fatal("identical-byte source substitution was accepted across restart")
	}
	assertNoLearningAuthority(t, workdir)
}

func TestLearningAuthorityMigrationLostAcknowledgementDoesNotReplay(t *testing.T) {
	workdir, legacy, expected := learningAuthorityLegacyFixture(t)
	interrupted := errors.New("simulated lost acknowledgement")
	copies := 0
	learningAuthorityMigrationTestHook = func(stage string) error {
		if strings.HasPrefix(stage, "copy-file:") {
			copies++
		}
		if stage == "after-rename" {
			return interrupted
		}
		return nil
	}
	t.Cleanup(func() { learningAuthorityMigrationTestHook = nil })
	if _, _, err := PrepareLearningAuthority(workdir); !errors.Is(err, interrupted) {
		t.Fatalf("first migration error = %v, want lost acknowledgement", err)
	}
	firstCopies := copies
	if err := os.WriteFile(filepath.Join(legacy, "training", "metrics.jsonl"), []byte("{\"late\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	learningAuthorityMigrationTestHook = nil
	durable, status, err := PrepareLearningAuthority(workdir)
	if err != nil {
		t.Fatal(err)
	}
	if copies != firstCopies || !status.DurableAuthoritative || !status.LegacyDiverged {
		t.Fatalf("lost acknowledgement replayed or merged import: copies=%d status=%+v", copies, status)
	}
	got, err := os.ReadFile(filepath.Join(durable, "training", "metrics.jsonl"))
	if err != nil || string(got) != expected["training/metrics.jsonl"] {
		t.Fatalf("lost acknowledgement changed cumulative training: %q, %v", got, err)
	}
}

func learningAuthorityLegacyFixture(t *testing.T) (string, string, map[string]string) {
	t.Helper()
	workdir := isolatedDurableStateWorkdir(t)
	legacy := filepath.Join(workdir, infrastructure.DockpipeDirRel, "packages", dorkpipeScope)
	expected := map[string]string{
		"analysis/queue.json":    "{\"items\":[{\"id\":\"q1\"}]}\n",
		"analysis/insights.json": "{\"insights\":[{\"id\":\"i1\"}]}\n",
		"analysis/history.jsonl": "{\"event\":\"enqueue\"}\n{\"event\":\"process\"}\n",
		"metrics.jsonl":          "{\"run\":1}\n{\"run\":2}\n",
		"training/metrics.jsonl": "{\"task\":1}\n{\"task\":2}\n",
	}
	for rel, raw := range expected {
		path := filepath.Join(legacy, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for rel, raw := range map[string]string{
		"analysis/by-category/convention.json": "derived\n",
		"provider-pools/sessions.json":         "provider\n",
		"self-analysis/facts.txt":              "facts\n",
	} {
		path := filepath.Join(legacy, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return workdir, legacy, expected
}

func assertNoLearningAuthority(t *testing.T, workdir string) {
	t.Helper()
	packageRoot, err := infrastructure.ProjectPackageStateDir(workdir, dorkpipeScope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(packageRoot, learningAuthorityDestinationName)); !os.IsNotExist(err) {
		t.Fatalf("invalid state published a durable learning authority: %v", err)
	}
}

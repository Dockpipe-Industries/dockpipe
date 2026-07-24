package orchestrationhelper

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBacklogValidationExecutionUsesExactPatchedWorkspaceAndSupportsArtifactOnlyRestart(t *testing.T) {
	artifactRoot, consumerRoot := prepareBacklogValidationExecutionArtifacts(t, `[
  "go test ./packages/dorkpipe/lib/orchestrationhelper"
]`, `["AGENTS.md"]`)
	upstream := snapshotBacklogValidationArtifacts(t, artifactRoot)
	consumerBefore := mustReadFile(t, filepath.Join(consumerRoot, "packages", "dorkpipe", "README.md"))
	temporaryParent := t.TempDir()
	created := captureBacklogValidationTemporaryRoots(t, temporaryParent)

	originalRunner := backlogValidationRunCommand
	runs := 0
	backlogValidationRunCommand = func(ctx context.Context, root string, argv, environment []string) (int, bool, error) {
		runs++
		if ctx == nil || !stringSlicesEqual(argv, []string{"go", "test", "./packages/dorkpipe/lib/orchestrationhelper"}) {
			t.Fatalf("validation did not receive direct expected argv: %#v", argv)
		}
		if root == consumerRoot || !environmentContains(environment, "GOPROXY=off") || !environmentContains(environment, "GOVCS=off") || !environmentContains(environment, "GOENV=off") {
			t.Fatalf("validation command did not use an isolated offline environment")
		}
		expected := map[string][]byte{
			"AGENTS.md":                   []byte("# Fixture agent guidance\n"),
			"packages/dorkpipe/README.md": []byte("# Fixture package\nUntrusted remote fixture change.\n"),
		}
		if err := verifyBacklogValidationWorkspace(root, expected); err != nil {
			t.Fatalf("validation workspace is not the exact patched union: %v", err)
		}
		return 0, true, nil
	}
	t.Cleanup(func() { backlogValidationRunCommand = originalRunner })

	if err := executeBacklogValidation(consumerRoot, artifactRoot); err != nil {
		t.Fatal(err)
	}
	if runs != 1 {
		t.Fatalf("validation command runs = %d, want 1", runs)
	}
	assertBacklogValidationTemporaryRootsRemoved(t, *created)
	assertBacklogValidationArtifactsUnchanged(t, artifactRoot, upstream)
	if got := mustReadFile(t, filepath.Join(consumerRoot, "packages", "dorkpipe", "README.md")); !bytesEqual(got, consumerBefore) {
		t.Fatal("validation execution mutated the consumer checkout")
	}

	executionPath := filepath.Join(artifactRoot, "validation-execution.json")
	executionRaw := mustReadFile(t, executionPath)
	execution := readJSONMap(executionPath)
	if stringValue(execution["contract_version"]) != backlogValidationExecutionContract || stringValue(execution["state"]) != "completion_candidate" {
		t.Fatalf("unexpected validation execution contract: %#v", execution)
	}
	if stringValue(mapValue(execution["aggregate"])["status"]) != "passed" || !backlogTestBool(mapValue(execution["actions"])["validation_executed"]) || backlogTestBool(mapValue(execution["actions"])["validation_success_authoritative"]) {
		t.Fatalf("unexpected validation execution outcome: %#v", execution)
	}
	if intFromAny(mapValue(execution["workspace"])["file_count"]) != 2 || stringValue(mapValue(execution["binding"])["task_id"]) != "TASK-015" {
		t.Fatalf("validation execution did not bind the exact workspace and task: %#v", execution)
	}
	if strings.Contains(string(executionRaw), consumerRoot) || strings.Contains(string(executionRaw), temporaryParent) || strings.Contains(string(executionRaw), "dorkpipe-backlog-validation-execution-") {
		t.Fatal("validation execution leaked an absolute consumer or temporary path")
	}

	if err := os.RemoveAll(consumerRoot); err != nil {
		t.Fatal(err)
	}
	backlogValidationRunCommand = func(context.Context, string, []string, []string) (int, bool, error) {
		t.Fatal("artifact-only restart re-executed validation")
		return 0, false, nil
	}
	if err := executeBacklogValidation(filepath.Join(t.TempDir(), "missing-consumer"), artifactRoot); err != nil {
		t.Fatalf("artifact-only restart failed: %v", err)
	}
	if got := mustReadFile(t, executionPath); !bytesEqual(got, executionRaw) {
		t.Fatal("artifact-only restart changed validation-execution.json")
	}
}

func TestBacklogValidationExecutionRecordsNonzeroAndStopsAtFirstFailure(t *testing.T) {
	artifactRoot, consumerRoot := prepareBacklogValidationExecutionArtifacts(t, `[
  "go test ./first",
  "go test ./second",
  "go test ./third"
]`, `["AGENTS.md"]`)
	created := captureBacklogValidationTemporaryRoots(t, t.TempDir())
	originalRunner := backlogValidationRunCommand
	runs := 0
	backlogValidationRunCommand = func(context.Context, string, []string, []string) (int, bool, error) {
		runs++
		if runs == 2 {
			return 7, true, nil
		}
		return 0, true, nil
	}
	t.Cleanup(func() { backlogValidationRunCommand = originalRunner })

	if err := executeBacklogValidation(consumerRoot, artifactRoot); err != nil {
		t.Fatal(err)
	}
	if runs != 2 {
		t.Fatalf("validation executed %d commands; want stop after second", runs)
	}
	assertBacklogValidationTemporaryRootsRemoved(t, *created)
	execution := readJSONMap(filepath.Join(artifactRoot, "validation-execution.json"))
	commands := listValue(execution["commands"])
	aggregate := mapValue(execution["aggregate"])
	if len(commands) != 2 || stringValue(mapValue(commands[1])["status"]) != "failed" || intFromAny(mapValue(commands[1])["exit_code"]) != 7 || stringValue(aggregate["status"]) != "failed" || !backlogTestBool(aggregate["stopped_after_first_failure"]) {
		t.Fatalf("unexpected failed validation evidence: %#v", execution)
	}
	assertNoBacklogLifecycleArtifacts(t, artifactRoot)
}

func TestBacklogValidationExecutionRejectsUnsafeCommandGrammarBeforeExecution(t *testing.T) {
	tests := []string{
		`go test "./packages/dorkpipe/lib/orchestrationhelper"`,
		`go test ./packages/dorkpipe/lib/orchestrationhelper | git status`,
		`go test ./packages/dorkpipe/lib/orchestrationhelper > result.txt`,
		`GOFLAGS=-mod=mod go test ./packages/dorkpipe/lib/orchestrationhelper`,
		`C:/Go/bin/go test ./packages/dorkpipe/lib/orchestrationhelper`,
		`go test ../orchestrationhelper`,
		`go test ./packages/dorkpipe/lib/...`,
		`git test ./packages/dorkpipe/lib/orchestrationhelper`,
		"go\ttest ./packages/dorkpipe/lib/orchestrationhelper",
	}
	for _, declaration := range tests {
		if argv, err := parseBacklogValidationCommand(declaration); err == nil {
			t.Fatalf("unsafe declaration %q unexpectedly parsed as %#v", declaration, argv)
		}
	}
	if argv, err := parseBacklogValidationCommand("go test ./packages/dorkpipe/lib/orchestrationhelper"); err != nil || !stringSlicesEqual(argv, []string{"go", "test", "./packages/dorkpipe/lib/orchestrationhelper"}) {
		t.Fatalf("safe declaration rejected: %#v, %v", argv, err)
	}
}

func TestBacklogValidationExecutionFiltersHostWorkspaceAndTemporaryOverrides(t *testing.T) {
	t.Setenv("GOWORK", "C:/host/go.work")
	t.Setenv("GOTMPDIR", "C:/host/go-tmp")
	environment := backlogValidationCommandEnvironment()
	for _, entry := range environment {
		upper := strings.ToUpper(entry)
		if strings.HasPrefix(upper, "GOWORK=") || strings.HasPrefix(upper, "GOTMPDIR=") {
			t.Fatalf("validation environment retained host workspace or temporary override %q", entry)
		}
	}
}

func TestBacklogValidationExecutionRejectsSourceFailuresWithoutExecutionOrArtifact(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{name: "changed", mutate: func(t *testing.T, root string) {
			writeBacklogTestFile(t, filepath.Join(root, "AGENTS.md"), "tampered\n")
		}},
		{name: "missing", mutate: func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "AGENTS.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "non regular", mutate: func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "AGENTS.md")); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(root, "AGENTS.md"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "oversized", mutate: func(t *testing.T, root string) {
			writeBacklogTestFile(t, filepath.Join(root, "AGENTS.md"), strings.Repeat("x", backlogValidationInputMaxBytes+1))
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifactRoot, consumerRoot := prepareBacklogValidationExecutionArtifacts(t, `["go test ./packages/dorkpipe/lib/orchestrationhelper"]`, `["AGENTS.md"]`)
			test.mutate(t, consumerRoot)
			originalRunner := backlogValidationRunCommand
			backlogValidationRunCommand = func(context.Context, string, []string, []string) (int, bool, error) {
				t.Fatal("source rejection launched validation")
				return 0, false, nil
			}
			t.Cleanup(func() { backlogValidationRunCommand = originalRunner })
			err := executeBacklogValidation(consumerRoot, artifactRoot)
			if err == nil || !strings.HasPrefix(err.Error(), "validation_execution_source_invalid:") {
				t.Fatalf("source rejection returned %v", err)
			}
			assertBacklogValidationReceiptAbsent(t, artifactRoot)
		})
	}

	t.Run("symlink", func(t *testing.T) {
		artifactRoot, consumerRoot := prepareBacklogValidationExecutionArtifacts(t, `["go test ./packages/dorkpipe/lib/orchestrationhelper"]`, `["AGENTS.md"]`)
		target := filepath.Join(consumerRoot, "real-agents.md")
		writeBacklogTestFile(t, target, "# Fixture agent guidance\n")
		if err := os.Remove(filepath.Join(consumerRoot, "AGENTS.md")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(consumerRoot, "AGENTS.md")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if err := executeBacklogValidation(consumerRoot, artifactRoot); err == nil || !strings.HasPrefix(err.Error(), "validation_execution_source_invalid:") {
			t.Fatalf("symlink rejection returned %v", err)
		}
		assertBacklogValidationReceiptAbsent(t, artifactRoot)
	})
}

func TestBacklogValidationExecutionCleansOperationalFailuresAndRejectsTampering(t *testing.T) {
	t.Run("launch failure", func(t *testing.T) {
		artifactRoot, consumerRoot := prepareBacklogValidationExecutionArtifacts(t, `["go test ./packages/dorkpipe/lib/orchestrationhelper"]`, `["AGENTS.md"]`)
		created := captureBacklogValidationTemporaryRoots(t, t.TempDir())
		originalRunner := backlogValidationRunCommand
		backlogValidationRunCommand = func(context.Context, string, []string, []string) (int, bool, error) {
			return 0, false, errors.New("injected launch failure")
		}
		t.Cleanup(func() { backlogValidationRunCommand = originalRunner })
		if err := executeBacklogValidation(consumerRoot, artifactRoot); err == nil || !strings.HasPrefix(err.Error(), "validation_execution_launch_failed:") {
			t.Fatalf("launch failure returned %v", err)
		}
		assertBacklogValidationTemporaryRootsRemoved(t, *created)
		assertBacklogValidationReceiptAbsent(t, artifactRoot)
	})

	t.Run("cleanup failure", func(t *testing.T) {
		artifactRoot, consumerRoot := prepareBacklogValidationExecutionArtifacts(t, `["go test ./packages/dorkpipe/lib/orchestrationhelper"]`, `["AGENTS.md"]`)
		originalRunner := backlogValidationRunCommand
		originalRemoveAll := backlogValidationRemoveAll
		backlogValidationRunCommand = func(context.Context, string, []string, []string) (int, bool, error) { return 0, true, nil }
		backlogValidationRemoveAll = func(path string) error {
			_ = originalRemoveAll(path)
			return errors.New("injected cleanup failure")
		}
		t.Cleanup(func() {
			backlogValidationRunCommand = originalRunner
			backlogValidationRemoveAll = originalRemoveAll
		})
		if err := executeBacklogValidation(consumerRoot, artifactRoot); err == nil || !strings.HasPrefix(err.Error(), "validation_execution_cleanup_failed:") {
			t.Fatalf("cleanup failure returned %v", err)
		}
		assertBacklogValidationReceiptAbsent(t, artifactRoot)
	})

	t.Run("upstream tampering before source access", func(t *testing.T) {
		artifactRoot, consumerRoot := prepareBacklogValidationExecutionArtifacts(t, `["go test ./packages/dorkpipe/lib/orchestrationhelper"]`, `["AGENTS.md"]`)
		mutateBacklogJSONFile(t, filepath.Join(artifactRoot, "patch-boundary.json"), func(payload map[string]any) { payload["state"] = "ready_for_review" })
		if err := os.RemoveAll(consumerRoot); err != nil {
			t.Fatal(err)
		}
		err := executeBacklogValidation(consumerRoot, artifactRoot)
		if err == nil || !strings.HasPrefix(err.Error(), "validation_execution_chain_invalid:") {
			t.Fatalf("upstream tampering returned %v", err)
		}
		assertBacklogValidationReceiptAbsent(t, artifactRoot)
	})

	t.Run("tampered existing artifact is preserved", func(t *testing.T) {
		artifactRoot, consumerRoot := prepareBacklogValidationExecutionArtifacts(t, `["go test ./packages/dorkpipe/lib/orchestrationhelper"]`, `["AGENTS.md"]`)
		originalRunner := backlogValidationRunCommand
		backlogValidationRunCommand = func(context.Context, string, []string, []string) (int, bool, error) { return 0, true, nil }
		t.Cleanup(func() { backlogValidationRunCommand = originalRunner })
		if err := executeBacklogValidation(consumerRoot, artifactRoot); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(artifactRoot, "validation-execution.json")
		mutateBacklogJSONFile(t, path, func(payload map[string]any) { payload["state"] = "ready_for_review" })
		tampered := mustReadFile(t, path)
		if err := os.RemoveAll(consumerRoot); err != nil {
			t.Fatal(err)
		}
		if err := executeBacklogValidation(consumerRoot, artifactRoot); err == nil || !strings.HasPrefix(err.Error(), "validation_execution_artifact_invalid:") {
			t.Fatalf("tampered existing artifact returned %v", err)
		}
		if got := mustReadFile(t, path); !bytesEqual(got, tampered) {
			t.Fatal("tampered validation artifact was overwritten")
		}
	})
}

func prepareBacklogValidationExecutionArtifacts(t *testing.T, requiredValidationJSON, validationInputsJSON string) (string, string) {
	t.Helper()
	consumer := writeBacklogTestRepo(t)
	root := filepath.Join(t.TempDir(), "artifacts")
	if err := inspectBacklogSelection(consumer, backlogIndexPath, "TASK-015", "Implement only bounded validation execution.", backlogTestBaseline, root); err != nil {
		t.Fatal(err)
	}
	if err := compileBacklogRemoteRequest(consumer, root, "fixture-environment", "js/dev", `["packages/dorkpipe"]`, `["No live provider"]`, requiredValidationJSON, validationInputsJSON, `[]`); err != nil {
		t.Fatal(err)
	}
	if err := preflightBacklogRemoteCompatibility(root, writeBacklogCompatibilityFixture(t)); err != nil {
		t.Fatal(err)
	}
	dispatchFixture := filepath.Join(t.TempDir(), "dispatch.json")
	writeBacklogTestFile(t, dispatchFixture, `{
  "contract_version": "dorkpipe.remote-dispatch-fixture/v1",
  "adapter_identity": "codex-cloud-fixture-v1",
  "remote_task_id": "remote_fixture_task_015",
  "submitted_at": "2026-07-19T00:00:00Z"
}`)
	if err := dispatchBacklogFixture(root, dispatchFixture); err != nil {
		t.Fatal(err)
	}
	if err := ingestBacklogCompletionCandidate(root, writeBacklogCompletionFixture(t, root, "completion_fixture_candidate_015", "completion_fixture_replay_015", "2026-07-19T00:01:00Z")); err != nil {
		t.Fatal(err)
	}
	if err := retrieveBacklogRemoteStatusFixture(root, writeBacklogStatusFixture(t, root, "status_fixture_observation_015", "status_fixture_replay_015", "2026-07-19T00:02:00Z")); err != nil {
		t.Fatal(err)
	}
	if err := retrieveBacklogRemoteDiffFixture(root, writeBacklogDiffFixture(t, root, "diff_fixture_observation_015", "diff_fixture_replay_015", "2026-07-19T00:03:00Z", backlogTestPatch)); err != nil {
		t.Fatal(err)
	}
	if err := retrieveBacklogRemoteResultFixture(root, writeBacklogResultFixture(t, root, "result_fixture_observation_015", "result_fixture_replay_015", "2026-07-19T00:04:00Z", "fixture-owned opaque result evidence")); err != nil {
		t.Fatal(err)
	}
	if err := retrieveBacklogValidationReceiptFixture(root, writeBacklogValidationReceiptFixture(t, root, "receipt_fixture_observation_015", "receipt_fixture_replay_015", "2026-07-19T00:05:00Z", "fixture-owned opaque validation receipt evidence")); err != nil {
		t.Fatal(err)
	}
	if err := verifyBacklogPatchBoundary(root); err != nil {
		t.Fatal(err)
	}
	if err := applyBacklogPatchTemporaryCopy(consumer, root); err != nil {
		t.Fatal(err)
	}
	return root, consumer
}

func snapshotBacklogValidationArtifacts(t *testing.T, root string) map[string][]byte {
	t.Helper()
	before := snapshotBacklogApplicationArtifacts(t, root)
	before["patch-application.json"] = mustReadFile(t, filepath.Join(root, "patch-application.json"))
	return before
}

func assertBacklogValidationArtifactsUnchanged(t *testing.T, root string, before map[string][]byte) {
	t.Helper()
	for name, expected := range before {
		if got := mustReadFile(t, filepath.Join(root, name)); !bytesEqual(got, expected) {
			t.Fatalf("validation execution changed upstream artifact %s", name)
		}
	}
}

func captureBacklogValidationTemporaryRoots(t *testing.T, parent string) *[]string {
	t.Helper()
	original := backlogValidationMkdirTemp
	created := []string{}
	backlogValidationMkdirTemp = func(_ string, pattern string) (string, error) {
		path, err := os.MkdirTemp(parent, pattern)
		if err == nil {
			created = append(created, path)
		}
		return path, err
	}
	t.Cleanup(func() { backlogValidationMkdirTemp = original })
	return &created
}

func assertBacklogValidationTemporaryRootsRemoved(t *testing.T, roots []string) {
	t.Helper()
	for _, root := range roots {
		if _, err := os.Lstat(root); !os.IsNotExist(err) {
			t.Fatalf("temporary validation root survived cleanup: %s (%v)", root, err)
		}
	}
}

func assertBacklogValidationReceiptAbsent(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, "validation-execution.json")); !os.IsNotExist(err) {
		t.Fatalf("validation-execution.json exists after rejection: %v", err)
	}
}

func assertNoBacklogLifecycleArtifacts(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"ready-for-review.json", "apply.json", "commit.json", "push.json", "publication.json"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("validation execution created lifecycle artifact %s", name)
		}
	}
}

func environmentContains(environment []string, expected string) bool {
	for _, entry := range environment {
		if entry == expected {
			return true
		}
	}
	return false
}

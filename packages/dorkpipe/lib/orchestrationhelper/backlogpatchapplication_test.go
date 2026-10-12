package orchestrationhelper

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBacklogPatchApplicationIsDeterministicIsolatedAndRestartSafe(t *testing.T) {
	artifactRoot := prepareBacklogPatchApplicationArtifacts(t, backlogTestPatch, `["packages/dorkpipe"]`)
	consumerRoot := writeBacklogTestRepo(t)
	consumerPath := filepath.Join(consumerRoot, "packages", "dorkpipe", "README.md")
	consumerBefore := mustReadFile(t, consumerPath)
	upstreamBefore := snapshotBacklogApplicationArtifacts(t, artifactRoot)

	temporaryParent := t.TempDir()
	created := captureBacklogApplicationTemporaryRoots(t, temporaryParent)
	if err := applyBacklogPatchTemporaryCopy(consumerRoot, artifactRoot); err != nil {
		t.Fatal(err)
	}
	if len(*created) != 1 {
		t.Fatalf("created temporary roots = %#v", *created)
	}
	assertBacklogApplicationTemporaryRootsRemoved(t, *created)
	if got := mustReadFile(t, consumerPath); string(got) != string(consumerBefore) {
		t.Fatalf("consumer source changed: %q", got)
	}
	assertBacklogApplicationArtifactsUnchanged(t, artifactRoot, upstreamBefore)

	applicationPath := filepath.Join(artifactRoot, "patch-application.json")
	applicationRaw := mustReadFile(t, applicationPath)
	application := readJSONMap(applicationPath)
	if stringValue(application["contract_version"]) != backlogPatchApplicationContract || stringValue(application["state"]) != "completion_candidate" {
		t.Fatalf("unexpected application contract or state: %#v", application)
	}
	binding := mapValue(application["binding"])
	if stringValue(binding["baseline_commit"]) != backlogTestBaseline || backlogTestBool(binding["baseline_commit_git_verified"]) {
		t.Fatalf("baseline was not preserved as an unverified request declaration: %#v", binding)
	}
	request := readJSONMap(filepath.Join(artifactRoot, "remote-request.json"))
	if stringValue(binding["validation_inputs_fingerprint"]) != stringValue(mapValue(request["validation_input_manifest"])["fingerprint"]) {
		t.Fatalf("application does not bind the immutable validation inputs: %#v", binding)
	}
	acceptedPatch := mapValue(application["accepted_patch"])
	if stringValue(acceptedPatch["sha256"]) != sha256String([]byte(backlogTestPatch)) || int(acceptedPatch["bytes"].(float64)) != len(backlogTestPatch) {
		t.Fatalf("accepted patch binding is wrong: %#v", acceptedPatch)
	}
	applicationEvidence := mapValue(application["application"])
	if stringValue(applicationEvidence["application_scope"]) != "temporary_copy_only" || !backlogTestBool(applicationEvidence["mechanical_application_succeeded"]) || !backlogTestBool(applicationEvidence["temporary_workspace_cleanup_succeeded"]) || int(applicationEvidence["files_applied"].(float64)) != 1 || int(applicationEvidence["hunks_applied"].(float64)) != 1 {
		t.Fatalf("unexpected mechanical application evidence: %#v", applicationEvidence)
	}
	postimage := mapValue(application["postimage_manifest"])
	postFiles := listValue(postimage["files"])
	if len(postFiles) != 1 || stringValue(mapValue(postFiles[0])["sha256"]) != "sha256:3dc160ac1641f7a12a1ab717c64c90aba3b5da8a7123eba6fb303b5d7dfdcb0a" {
		t.Fatalf("unexpected fixture postimage digest: %#v", postimage)
	}
	for name, value := range mapValue(application["actions"]) {
		if backlogTestBool(value) {
			t.Fatalf("application unexpectedly performed %s", name)
		}
	}
	for name, value := range mapValue(application["lifecycle"]) {
		if backlogTestBool(value) {
			t.Fatalf("application unexpectedly enabled %s", name)
		}
	}
	if strings.Contains(string(applicationRaw), consumerRoot) || strings.Contains(string(applicationRaw), temporaryParent) || strings.Contains(string(applicationRaw), "dorkpipe-backlog-patch-application-") {
		t.Fatalf("application receipt leaked an absolute or temporary path: %s", applicationRaw)
	}

	if err := applyBacklogPatchTemporaryCopy(consumerRoot, artifactRoot); err != nil {
		t.Fatalf("idempotent rerun failed: %v", err)
	}
	if got := mustReadFile(t, applicationPath); string(got) != string(applicationRaw) {
		t.Fatal("idempotent rerun changed patch-application.json")
	}
	if len(*created) != 2 {
		t.Fatalf("rerun did not recreate one isolated temporary copy: %#v", *created)
	}
	assertBacklogApplicationTemporaryRootsRemoved(t, *created)
}

func TestBacklogPatchApplicationSupportsMultipleFilesAndHunks(t *testing.T) {
	patch := "diff --git a/packages/dorkpipe/README.md b/packages/dorkpipe/README.md\n" +
		"index 1111111..2222222 100644\n--- a/packages/dorkpipe/README.md\n+++ b/packages/dorkpipe/README.md\n" +
		"@@ -1,2 +1,3 @@\n one\n+inserted\n two\n@@ -4 +5 @@\n-four\n+FOUR\n" +
		"diff --git a/docs/agents/tasks/backlog-driven-remote-tasks.md b/docs/agents/tasks/backlog-driven-remote-tasks.md\n" +
		"index 3333333..4444444 100644\n--- a/docs/agents/tasks/backlog-driven-remote-tasks.md\n+++ b/docs/agents/tasks/backlog-driven-remote-tasks.md\n" +
		"@@ -1,3 +1,3 @@\n alpha\n-beta\n+BETA\n gamma\n"
	artifactRoot := prepareBacklogPatchApplicationArtifacts(t, patch, `["packages/dorkpipe","docs/agents/tasks/backlog-driven-remote-tasks.md"]`)
	consumerRoot := t.TempDir()
	writeBacklogTestFile(t, filepath.Join(consumerRoot, "packages", "dorkpipe", "README.md"), "one\ntwo\nthree\nfour\n")
	writeBacklogTestFile(t, filepath.Join(consumerRoot, "docs", "agents", "tasks", "backlog-driven-remote-tasks.md"), "alpha\nbeta\ngamma\n")
	beforeReadme := mustReadFile(t, filepath.Join(consumerRoot, "packages", "dorkpipe", "README.md"))
	beforeTask := mustReadFile(t, filepath.Join(consumerRoot, "docs", "agents", "tasks", "backlog-driven-remote-tasks.md"))
	if err := applyBacklogPatchTemporaryCopy(consumerRoot, artifactRoot); err != nil {
		t.Fatal(err)
	}
	application := readJSONMap(filepath.Join(artifactRoot, "patch-application.json"))
	evidence := mapValue(application["application"])
	if int(evidence["files_applied"].(float64)) != 2 || int(evidence["hunks_applied"].(float64)) != 3 {
		t.Fatalf("unexpected file/hunk counts: %#v", evidence)
	}
	if !jsonMapsEqual(map[string]any{"changed_paths": application["changed_paths"]}, map[string]any{"changed_paths": []any{"docs/agents/tasks/backlog-driven-remote-tasks.md", "packages/dorkpipe/README.md"}}) {
		t.Fatalf("changed paths are not sorted: %#v", application["changed_paths"])
	}
	if string(mustReadFile(t, filepath.Join(consumerRoot, "packages", "dorkpipe", "README.md"))) != string(beforeReadme) || string(mustReadFile(t, filepath.Join(consumerRoot, "docs", "agents", "tasks", "backlog-driven-remote-tasks.md"))) != string(beforeTask) {
		t.Fatal("multiple-file application changed consumer sources")
	}
}

func TestBacklogPatchApplicationRejectsSourceAndApplicationFailuresWithoutReceipt(t *testing.T) {
	replacementPatch := strings.Replace(backlogTestPatch, "@@ -1 +1,2 @@\n # Fixture package\n+Untrusted remote fixture change.", "@@ -1 +1 @@\n-# Fixture package\n+# Replaced package", 1)
	tests := []struct {
		name     string
		patch    string
		prepare  func(*testing.T, string)
		wantCode string
	}{
		{name: "context mismatch", patch: backlogTestPatch, prepare: func(t *testing.T, root string) {
			writeBacklogTestFile(t, filepath.Join(root, "packages", "dorkpipe", "README.md"), "# Wrong package\n")
		}, wantCode: "patch_application_preimage_mismatch"},
		{name: "removed mismatch", patch: replacementPatch, prepare: func(t *testing.T, root string) {
			writeBacklogTestFile(t, filepath.Join(root, "packages", "dorkpipe", "README.md"), "# Wrong package\n")
		}, wantCode: "patch_application_preimage_mismatch"},
		{name: "invalid hunk counts", patch: strings.Replace(backlogTestPatch, "@@ -1 +1,2 @@", "@@ -1 +1,3 @@", 1), prepare: writeBacklogApplicationFixtureSource, wantCode: "patch_application_patch_unsupported"},
		{name: "no newline marker", patch: backlogTestPatch + "\\ No newline at end of file\n", prepare: writeBacklogApplicationFixtureSource, wantCode: "patch_application_patch_unsupported"},
		{name: "missing file", patch: backlogTestPatch, prepare: func(*testing.T, string) {}, wantCode: "patch_application_source_invalid"},
		{name: "non regular file", patch: backlogTestPatch, prepare: func(t *testing.T, root string) {
			if err := os.MkdirAll(filepath.Join(root, "packages", "dorkpipe", "README.md"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, wantCode: "patch_application_source_invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifactRoot := prepareBacklogPatchApplicationArtifacts(t, test.patch, `["packages/dorkpipe"]`)
			consumerRoot := t.TempDir()
			test.prepare(t, consumerRoot)
			temporaryParent := t.TempDir()
			created := captureBacklogApplicationTemporaryRoots(t, temporaryParent)
			err := applyBacklogPatchTemporaryCopy(consumerRoot, artifactRoot)
			if err == nil || !strings.HasPrefix(err.Error(), test.wantCode+":") {
				t.Fatalf("error = %v, want %s", err, test.wantCode)
			}
			assertBacklogApplicationTemporaryRootsRemoved(t, *created)
			if (test.name == "context mismatch" || test.name == "removed mismatch") && len(*created) != 1 {
				t.Fatalf("application failure did not create and clean exactly one temporary copy: %#v", *created)
			}
			if _, statErr := os.Stat(filepath.Join(artifactRoot, "patch-application.json")); !os.IsNotExist(statErr) {
				t.Fatalf("rejection created patch-application.json: %v", statErr)
			}
		})
	}
}

func TestBacklogPatchApplicationRejectsEveryTamperedUpstreamBeforeSourceRead(t *testing.T) {
	mutations := map[string]func(*testing.T, string){
		"remote-request.json": func(t *testing.T, path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) { mapValue(value["selection"])["baseline_commit"] = strings.Repeat("a", 40) })
		},
		"remote-request.md": func(t *testing.T, path string) { writeBacklogTestFile(t, path, "tampered request\n") },
		"remote-adapter-compatibility.json": func(t *testing.T, path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) { value["adapter_identity"] = "tampered-adapter" })
		},
		"remote-task.json": func(t *testing.T, path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) { value["remote_task_id"] = "remote_fixture_task_tampered" })
		},
		"completion-candidate.json": func(t *testing.T, path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) {
				mapValue(value["identity"])["candidate_id"] = "completion_fixture_candidate_tampered"
			})
		},
		"remote-status.json": func(t *testing.T, path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) {
				mapValue(value["identity"])["observation_id"] = "status_fixture_observation_tampered"
			})
		},
		"remote-diff.json": func(t *testing.T, path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) { value["observed_at"] = "2026-07-19T00:03:01Z" })
		},
		"remote-diff.patch": func(t *testing.T, path string) { writeBacklogTestFile(t, path, backlogTestPatch+"tampered\n") },
		"remote-result.json": func(t *testing.T, path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) { mapValue(value["evidence"])["opaque_result"] = "tampered result" })
		},
		"validation-receipt.json": func(t *testing.T, path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) {
				mapValue(value["remote_result"])["fingerprint"] = "sha256:" + strings.Repeat("a", 64)
			})
		},
		"patch-boundary.json": func(t *testing.T, path string) {
			mutateBacklogJSONFile(t, path, func(value map[string]any) { value["state"] = "ready_for_review" })
		},
	}
	for artifact, mutate := range mutations {
		t.Run(artifact, func(t *testing.T) {
			root := prepareBacklogPatchApplicationArtifacts(t, backlogTestPatch, `["packages/dorkpipe"]`)
			mutate(t, filepath.Join(root, artifact))
			missingConsumer := filepath.Join(t.TempDir(), "consumer-does-not-exist")
			err := applyBacklogPatchTemporaryCopy(missingConsumer, root)
			if err == nil || !strings.HasPrefix(err.Error(), "patch_application_boundary_invalid:") {
				t.Fatalf("tampered %s error = %v; immutable chain was not rejected before source access", artifact, err)
			}
			assertBacklogApplicationReceiptAbsent(t, root)
		})
	}
}

func TestBacklogPatchApplicationRejectsSymlinkBoundaryTamperingAndCleanupFailure(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		artifactRoot := prepareBacklogPatchApplicationArtifacts(t, backlogTestPatch, `["packages/dorkpipe"]`)
		consumerRoot := t.TempDir()
		target := filepath.Join(consumerRoot, "target.md")
		writeBacklogTestFile(t, target, "# Fixture package\n")
		link := filepath.Join(consumerRoot, "packages", "dorkpipe", "README.md")
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlink creation is unavailable: %v", err)
		}
		if err := applyBacklogPatchTemporaryCopy(consumerRoot, artifactRoot); err == nil || !strings.HasPrefix(err.Error(), "patch_application_source_invalid:") {
			t.Fatalf("symlink error = %v", err)
		}
	})

	t.Run("tampered boundary path escape", func(t *testing.T) {
		artifactRoot := prepareBacklogPatchApplicationArtifacts(t, backlogTestPatch, `["packages/dorkpipe"]`)
		mutateBacklogJSONFile(t, filepath.Join(artifactRoot, "patch-boundary.json"), func(value map[string]any) { value["changed_paths"] = []any{"../escape"} })
		if err := applyBacklogPatchTemporaryCopy(writeBacklogTestRepo(t), artifactRoot); err == nil || !strings.HasPrefix(err.Error(), "patch_application_boundary_invalid:") {
			t.Fatalf("tampered boundary error = %v", err)
		}
		assertBacklogApplicationReceiptAbsent(t, artifactRoot)
	})

	t.Run("tampered accepted patch", func(t *testing.T) {
		artifactRoot := prepareBacklogPatchApplicationArtifacts(t, backlogTestPatch, `["packages/dorkpipe"]`)
		writeBacklogTestFile(t, filepath.Join(artifactRoot, "remote-diff.patch"), backlogTestPatch+"tampered\n")
		if err := applyBacklogPatchTemporaryCopy(writeBacklogTestRepo(t), artifactRoot); err == nil || !strings.HasPrefix(err.Error(), "patch_application_boundary_invalid:") {
			t.Fatalf("tampered patch error = %v", err)
		}
		assertBacklogApplicationReceiptAbsent(t, artifactRoot)
	})

	t.Run("tampered existing receipt", func(t *testing.T) {
		artifactRoot := prepareBacklogPatchApplicationArtifacts(t, backlogTestPatch, `["packages/dorkpipe"]`)
		consumerRoot := writeBacklogTestRepo(t)
		if err := applyBacklogPatchTemporaryCopy(consumerRoot, artifactRoot); err != nil {
			t.Fatal(err)
		}
		applicationPath := filepath.Join(artifactRoot, "patch-application.json")
		mutateBacklogJSONFile(t, applicationPath, func(value map[string]any) { value["state"] = "ready_for_review" })
		tampered := mustReadFile(t, applicationPath)
		if err := applyBacklogPatchTemporaryCopy(consumerRoot, artifactRoot); err == nil || !strings.HasPrefix(err.Error(), "patch_application_artifact_invalid:") {
			t.Fatalf("tampered receipt error = %v", err)
		}
		if string(mustReadFile(t, applicationPath)) != string(tampered) {
			t.Fatal("tampered receipt was overwritten or repaired")
		}
	})

	t.Run("cleanup failure", func(t *testing.T) {
		artifactRoot := prepareBacklogPatchApplicationArtifacts(t, backlogTestPatch, `["packages/dorkpipe"]`)
		consumerRoot := writeBacklogTestRepo(t)
		originalRemoveAll := backlogApplicationRemoveAll
		backlogApplicationRemoveAll = func(path string) error {
			if err := originalRemoveAll(path); err != nil {
				return err
			}
			return errors.New("injected cleanup failure")
		}
		t.Cleanup(func() { backlogApplicationRemoveAll = originalRemoveAll })
		if err := applyBacklogPatchTemporaryCopy(consumerRoot, artifactRoot); err == nil || !strings.HasPrefix(err.Error(), "patch_application_cleanup_failed:") {
			t.Fatalf("cleanup error = %v", err)
		}
		assertBacklogApplicationReceiptAbsent(t, artifactRoot)
	})
}

func prepareBacklogPatchApplicationArtifacts(t *testing.T, patch, allowedPathsJSON string) string {
	t.Helper()
	root := prepareBacklogPatchBoundaryTest(t, patch, allowedPathsJSON)
	if err := verifyBacklogPatchBoundary(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeBacklogApplicationFixtureSource(t *testing.T, root string) {
	t.Helper()
	writeBacklogTestFile(t, filepath.Join(root, "packages", "dorkpipe", "README.md"), "# Fixture package\n")
}

func snapshotBacklogApplicationArtifacts(t *testing.T, root string) map[string][]byte {
	t.Helper()
	before := map[string][]byte{}
	for _, name := range []string{"remote-request.json", "remote-request.md", "remote-adapter-compatibility.json", "remote-task.json", "completion-candidate.json", "remote-status.json", "remote-diff.json", "remote-diff.patch", "remote-result.json", "validation-receipt.json", "patch-boundary.json"} {
		before[name] = mustReadFile(t, filepath.Join(root, name))
	}
	return before
}

func assertBacklogApplicationArtifactsUnchanged(t *testing.T, root string, before map[string][]byte) {
	t.Helper()
	for name, expected := range before {
		if got := mustReadFile(t, filepath.Join(root, name)); string(got) != string(expected) {
			t.Fatalf("application changed upstream artifact %s", name)
		}
	}
}

func captureBacklogApplicationTemporaryRoots(t *testing.T, parent string) *[]string {
	t.Helper()
	original := backlogApplicationMkdirTemp
	created := []string{}
	backlogApplicationMkdirTemp = func(_ string, pattern string) (string, error) {
		path, err := os.MkdirTemp(parent, pattern)
		if err == nil {
			created = append(created, path)
		}
		return path, err
	}
	t.Cleanup(func() { backlogApplicationMkdirTemp = original })
	return &created
}

func assertBacklogApplicationTemporaryRootsRemoved(t *testing.T, roots []string) {
	t.Helper()
	for _, root := range roots {
		if _, err := os.Lstat(root); !os.IsNotExist(err) {
			t.Fatalf("temporary application root survived cleanup: %s (%v)", root, err)
		}
	}
}

func assertBacklogApplicationReceiptAbsent(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, "patch-application.json")); !os.IsNotExist(err) {
		t.Fatalf("patch-application.json exists after rejection: %v", err)
	}
}

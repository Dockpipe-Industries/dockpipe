package application

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvironmentInstanceIsScopedToDefinitionAndStableAcrossRuns(t *testing.T) {
	global := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", global)
	firstWorkdir, secondWorkdir := t.TempDir(), t.TempDir()
	firstPath := filepath.Join(firstWorkdir, "dev.yml")
	secondPath := filepath.Join(secondWorkdir, "dev.yml")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.WriteFile(path, []byte("same definition"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	first, stateWorkdir, err := environmentInstanceForDefinition(firstPath)
	if err != nil || !environmentInstanceID.MatchString(first.ID) {
		t.Fatalf("first instance: %#v, %v", first, err)
	}
	reopened, reopenedWorkdir, err := environmentInstanceForDefinition(firstPath)
	if err != nil || reopened != first || stateWorkdir != reopenedWorkdir {
		t.Fatalf("reopening changed the identity or state root: %#v, %v", reopened, err)
	}
	if stateWorkdir != filepath.Join(global, "environment-instances", "runtime", first.ID) {
		t.Fatalf("environment state was not rooted in private host data: %q", stateWorkdir)
	}
	second, secondStateWorkdir, err := environmentInstanceForDefinition(secondPath)
	if err != nil || second.ID == first.ID || stateWorkdir == secondStateWorkdir {
		t.Fatalf("two definitions shared an identity: %#v, %v", second, err)
	}
}

func TestEnvironmentInstanceConcurrentFirstApplyConverges(t *testing.T) {
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	workdir := t.TempDir()
	path := filepath.Join(workdir, "dev.yml")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	type outcome struct {
		record environmentInstanceRecord
		err    error
	}
	results := make(chan outcome, 8)
	for range cap(results) {
		go func() {
			record, _, err := environmentInstanceForDefinition(path)
			results <- outcome{record: record, err: err}
		}()
	}
	var first environmentInstanceRecord
	for range cap(results) {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		if first.ID == "" {
			first = result.record
		}
		if first != result.record {
			t.Fatalf("concurrent applies published distinct instances: %#v, %#v", first, result.record)
		}
	}
}

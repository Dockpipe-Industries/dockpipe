//go:build linux || windows

package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSourceHostScriptAdmissionCancelsActiveSourceAndAllowsIndependentCleanup(t *testing.T) {
	if _, err := resolveBashExeForEnv(os.Environ()); err != nil {
		t.Skipf("actual Bash is required for sourced-script cancellation qualification: %v", err)
	}

	workdir := t.TempDir()
	script := filepath.Join(workdir, "source.sh")
	// A finite fallback and a separate release witness let the test reap a
	// broken source adapter without accepting that fallback as cancellation.
	body := "printf ready > ready\n" +
		"for ((attempt = 0; attempt < 80; attempt++)); do\n" +
		"    if [ -e release ]; then break; fi\n" +
		"    sleep 0.05\n" +
		"done\n" +
		"printf unexpected > after\n" +
		"export SOURCE_CONTEXT_RESULT=unexpected\n"
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	environment := append(os.Environ(),
		"DOCKPIPE_WORKDIR="+workdir,
		"DOCKPIPE_STEP_CWD="+workdir,
		"DOCKPIPE_HOST_SCRIPT_SPINNER=0",
		"DOCKPIPE_OBSERVABILITY=off",
		"DOCKPIPE_SKIP_HOST_CLEANUP=1",
	)
	type sourceResult struct {
		environment map[string]string
		err         error
	}
	result := make(chan sourceResult, 1)
	completed := make(chan struct{})
	go func() {
		defer close(completed)
		values, err := SourceHostScriptWithContext(ctx, script, environment)
		result <- sourceResult{environment: values, err: err}
	}()
	t.Cleanup(func() {
		cancel()
		if err := os.WriteFile(filepath.Join(workdir, "release"), []byte("test cleanup"), 0600); err != nil {
			t.Errorf("independent fixture release: %v", err)
		}
		select {
		case <-completed:
		case <-time.After(6 * time.Second):
			t.Error("source adapter did not join even after independent fixture release")
		}
	})

	ready := filepath.Join(workdir, "ready")
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if value, err := os.ReadFile(ready); err == nil && string(value) == "ready" {
			break
		}
		select {
		case observed := <-result:
			t.Fatalf("source returned before actual readiness: %v", observed.err)
		case <-ctx.Done():
			t.Fatal("sourced pre-script did not reach its admitted working directory")
		case <-ticker.C:
		}
	}

	cancel()
	select {
	case observed := <-result:
		if !errors.Is(observed.err, context.Canceled) {
			t.Fatalf("active sourced pre-script lost cancellation: %v", observed.err)
		}
		if len(observed.environment) != 0 {
			t.Fatalf("cancelled source published partial environment with %d entries", len(observed.environment))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("admission context did not reach the active sourced pre-script owner")
	}

	if _, err := os.Stat(filepath.Join(workdir, "after")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("sourced follow-on mutation ran after cancellation: %v", err)
	}

	// Cleanup owns a fresh context; reusing the cancelled body context would
	// prevent the cleanup process from starting and leave this witness absent.
	cleanupContext, stopCleanup := context.WithTimeout(context.Background(), 3*time.Second)
	defer stopCleanup()
	if err := RunHostCommandWithContext(cleanupContext, "printf cleanup > cleanup", environment, "bash"); err != nil {
		t.Fatalf("independent cleanup after sourced cancellation: %v", err)
	}
	if value, err := os.ReadFile(filepath.Join(workdir, "cleanup")); err != nil || string(value) != "cleanup" {
		t.Fatalf("independent cleanup witness = %q, %v", value, err)
	}
	entries, err := os.ReadDir(HostRunsDir(workdir))
	if err != nil || len(entries) != 0 {
		t.Fatalf("source/cleanup completion retained registry artifacts: %v, %v", entries, err)
	}
}

func TestSourceHostScriptWithContextPreservesEnvironment(t *testing.T) {
	if _, err := resolveBashExeForEnv(os.Environ()); err != nil {
		t.Skipf("actual Bash is required for context-aware source qualification: %v", err)
	}

	workdir := t.TempDir()
	script := filepath.Join(workdir, "source.sh")
	if err := os.WriteFile(script, []byte("export SOURCE_CONTEXT_EXPORTED=derived\n"), 0600); err != nil {
		t.Fatal(err)
	}
	environment := append(os.Environ(), "DOCKPIPE_WORKDIR="+workdir,
		"DOCKPIPE_RUN_ID=caller-run", "DOCKPIPE_PARENT_EXECUTION_ID=caller-parent",
		"DOCKPIPE_HOST_SCRIPT_SPINNER=0", "DOCKPIPE_OBSERVABILITY=off", "DOCKPIPE_SKIP_HOST_CLEANUP=1")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	values, err := SourceHostScriptWithContext(ctx, script, environment)
	if err != nil {
		t.Fatal(err)
	}
	if values["SOURCE_CONTEXT_EXPORTED"] != "derived" || values["DOCKPIPE_RUN_ID"] != "caller-run" || values["DOCKPIPE_PARENT_EXECUTION_ID"] != "caller-parent" {
		t.Fatal("context-aware source changed exports or injected registry execution identities")
	}
}

func TestSourceHostScriptWithContextPreCancelledDoesNotBeginRun(t *testing.T) {
	workdir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	values, err := SourceHostScriptWithContext(ctx, filepath.Join(workdir, "missing.sh"), []string{"DOCKPIPE_WORKDIR=" + workdir})
	if !errors.Is(err, context.Canceled) || len(values) != 0 {
		t.Fatalf("pre-cancelled source = %d exports, %v", len(values), err)
	}
	entries, err := os.ReadDir(workdir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("pre-cancelled source created run state: %v, %v", entries, err)
	}
}

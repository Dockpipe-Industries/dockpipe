package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain/invocation"
)

func TestNativeExecutionDenialPrecedesTemporaryFilesAndLookup(t *testing.T) {
	temporary := t.TempDir()
	t.Setenv("TMPDIR", temporary)
	t.Setenv("TEMP", temporary)
	t.Setenv("TMP", temporary)
	denied := errors.New("execution denied")
	admission := NativeExecutionAdmission{Authorize: func(_ context.Context, binding invocation.Binding) error {
		if binding.Contract.Action != "run" || binding.Contract.Version != "1" || len(binding.Digest) != 64 {
			t.Fatal("authorization received an unbound invocation")
		}
		return denied
	}}
	previousLookup := hostShellLookPathFn
	hostShellLookPathFn = func(string) (string, error) {
		t.Fatal("denied invocation looked up an executable")
		return "", nil
	}
	t.Cleanup(func() { hostShellLookPathFn = previousLookup })
	if err := AdmitHostCommand(admission, "write-file", nil, "pwsh", runHostCommandWithShell); !errors.Is(err, denied) {
		t.Fatalf("command denial lost: %v", err)
	}
	if err := AdmitHostScript(admission, filepath.Join(temporary, "missing.ps1"), nil, "pwsh", runHostScriptWithShell); !errors.Is(err, denied) {
		t.Fatalf("script denial lost: %v", err)
	}
	if _, err := AdmitSourceHostScript(admission, filepath.Join(temporary, "missing.sh"), nil, sourceHostScript); !errors.Is(err, denied) {
		t.Fatalf("source-script denial lost: %v", err)
	}
	entries, err := os.ReadDir(temporary)
	if err != nil || len(entries) != 0 {
		t.Fatalf("denied calls created temporary artifacts: %v, %v", entries, err)
	}
}

func TestNativeExecutionMalformedInputsPrecedeEffects(t *testing.T) {
	for _, test := range []struct {
		name        string
		command     string
		environment []string
		shell       string
	}{
		{name: "NUL command", command: "echo\x00unsafe"},
		{name: "NUL environment", command: "echo ok", environment: []string{"KEY=\x00"}},
		{name: "invalid environment", command: "echo ok", environment: []string{"NOT_AN_ASSIGNMENT"}},
		{name: "unknown shell", command: "echo ok", shell: "unrecognized"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := AdmitHostCommand(NativeExecutionAdmission{}, test.command, test.environment, test.shell, func(string, []string, string) error {
				t.Fatal("malformed invocation executed")
				return nil
			})
			if err == nil {
				t.Fatal("malformed invocation accepted")
			}
		})
	}
	if err := RunHostScript("bad\x00path", nil); err == nil {
		t.Fatal("public script entrypoint accepted NUL")
	}
	if _, err := SourceHostScript("", nil); err == nil {
		t.Fatal("public source entrypoint accepted an empty path")
	}
	if _, err := RunContainer(RunOpts{Image: "fixture"}, []string{"bad\x00arg"}); err == nil {
		t.Fatal("public container entrypoint accepted NUL")
	}
}

func TestNativeExecutionPreservesEmptyCommandsAndEnvironmentValues(t *testing.T) {
	if err := RunHostCommandWithShell(" \n\t", nil, "unused-invalid-shell"); err != nil {
		t.Fatalf("empty command compatibility changed: %v", err)
	}
	environment := []string{"KEY=first\nsecond", "=C:=C:\\work", "EMPTY="}
	err := AdmitHostCommand(NativeExecutionAdmission{}, "echo ok", environment, "", func(_ string, owned []string, _ string) error {
		if strings.Join(owned, "|") != strings.Join(environment, "|") {
			t.Fatal("environment values were rewritten")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNativeExecutionCancellationAndOwnedInput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := AdmitHostCommand(NativeExecutionAdmission{Context: ctx}, "echo ok", nil, "", func(string, []string, string) error {
		t.Fatal("cancelled command executed")
		return nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	environment := []string{"KEY=reviewed"}
	admission := NativeExecutionAdmission{Authorize: func(context.Context, invocation.Binding) error {
		environment[0] = "KEY=changed"
		return nil
	}}
	if err := AdmitHostCommand(admission, "echo ok", environment, "", func(_ string, owned []string, _ string) error {
		if owned[0] != "KEY=reviewed" {
			t.Fatal("authorizer changed accepted command input")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestNativeContainerAdmissionOwnsRuntimePolicy(t *testing.T) {
	options := RunOpts{Image: "fixture", ExtraEnv: []string{"KEY=reviewed"}, ExtraMounts: []string{"reviewed:/work"}, CapDrop: []string{"ALL"}, ReadOnlyRootFS: true}
	arguments := []string{"reviewed"}
	admission := NativeExecutionAdmission{Authorize: func(context.Context, invocation.Binding) error {
		options.ExtraEnv[0] = "KEY=changed"
		options.ExtraMounts[0] = "/:/work"
		options.CapDrop[0] = ""
		arguments[0] = "changed"
		return nil
	}}
	_, err := AdmitContainer(admission, options, arguments, func(owned RunOpts, args []string) (int, error) {
		if owned.ExtraEnv[0] != "KEY=reviewed" || owned.ExtraMounts[0] != "reviewed:/work" || owned.CapDrop[0] != "ALL" || args[0] != "reviewed" || !owned.ReadOnlyRootFS {
			t.Fatal("accepted container input or compiled policy changed")
		}
		return 0, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNativeExecutionDigestSeparatesOrderedFields(t *testing.T) {
	if NativeExecutionDigest([]string{"ab", "c"}) == NativeExecutionDigest([]string{"a", "bc"}) ||
		NativeExecutionDigest([]string{"a"}, []string{"b"}) == NativeExecutionDigest([]string{"a", "b"}) {
		t.Fatal("distinct parameter boundaries have identical digest inputs")
	}
}

func TestNativeExecutionBindsScriptExecutionMode(t *testing.T) {
	digests := make([]string, 0, 2)
	admission := NativeExecutionAdmission{Authorize: func(_ context.Context, binding invocation.Binding) error {
		digests = append(digests, binding.Digest)
		return nil
	}}
	if err := AdmitHostScript(admission, "fixture.sh", nil, "bash", func(string, []string, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := AdmitSourceHostScript(admission, "fixture.sh", nil, func(string, []string) (map[string]string, error) { return nil, nil }); err != nil {
		t.Fatal(err)
	}
	if len(digests) != 2 || digests[0] == digests[1] {
		t.Fatal("script execution and environment sourcing share an authorization digest")
	}
}

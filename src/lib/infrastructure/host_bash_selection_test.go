package infrastructure

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHostBashExplicitInterpreterRejectsInvalidSelection(t *testing.T) {
	t.Setenv("DOCKPIPE_HOST_BASH_BIN", "")
	directory := t.TempDir()
	ordinaryFile := filepath.Join(directory, "not-executable")
	if err := os.WriteFile(ordinaryFile, []byte("plain file"), 0o600); err != nil {
		t.Fatal(err)
	}
	invalid := []string{"relative/bash", filepath.Join(directory, "missing"), directory}
	if runtime.GOOS != "windows" {
		invalid = append(invalid, ordinaryFile)
	}
	for _, selected := range invalid {
		if _, err := resolveBashExeForEnv([]string{"DOCKPIPE_HOST_BASH_BIN=" + selected}); err == nil {
			t.Fatalf("invalid explicit interpreter %q silently accepted", selected)
		}
	}
}

func TestHostBashExplicitInterpreterIsUsedForScriptAndSource(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX wrapper fixture; Windows path selection has its existing native suite")
	}
	actual, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	actual, err = filepath.Abs(actual)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	selected := filepath.Join(directory, "selected bash")
	marker := filepath.Join(directory, "interpreter-calls")
	wrapper := "#!/bin/sh\nprintf 'selected\\n' >> '" + marker + "'\nexec '" + actual + "' \"$@\"\n"
	if err := os.WriteFile(selected, []byte(wrapper), 0o700); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(directory, "fixture.sh")
	if err := os.WriteFile(script, []byte("export INTERPRETER_FIXTURE=observed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A caller's workflow environment overrides an invalid process selection.
	t.Setenv("DOCKPIPE_HOST_BASH_BIN", "/not/an/interpreter")
	environment := append(os.Environ(), "DOCKPIPE_HOST_BASH_BIN="+selected)
	command, prepared, err := hostScriptCommand(script, environment, "bash")
	if err != nil {
		t.Fatal(err)
	}
	command.Env = prepared
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
	exports, err := SourceHostScript(script, environment)
	if err != nil || exports["INTERPRETER_FIXTURE"] != "observed" {
		t.Fatalf("source result = %v, %v", exports, err)
	}
	calls, err := os.ReadFile(marker)
	if err != nil || strings.Count(string(calls), "selected\n") != 2 {
		t.Fatalf("explicit interpreter calls = %q, %v", calls, err)
	}
}

func TestHostBashExplicitSelectionPrecedence(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_HOST_BASH_BIN", filepath.Join(t.TempDir(), "missing"))
	selected, err := resolveBashExeForEnv([]string{"DOCKPIPE_HOST_BASH_BIN=" + executable})
	if err != nil || selected != executable {
		t.Fatalf("explicit workflow selection = %q, %v", selected, err)
	}
	t.Setenv("DOCKPIPE_HOST_BASH_BIN", executable)
	selected, err = resolveBashExeForEnv(nil)
	if err != nil || selected != executable {
		t.Fatalf("explicit process selection = %q, %v", selected, err)
	}
}

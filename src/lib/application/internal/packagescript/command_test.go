package packagescript

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestUpsertEnvReplacesDuplicates(t *testing.T) {
	got := UpsertEnv([]string{"A=old", "B=keep", "A=duplicate"}, "A", "new")
	want := []string{"A=new", "B=keep"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UpsertEnv() = %#v, want %#v", got, want)
	}
}

func TestBashIsWSLPath(t *testing.T) {
	for _, path := range []string{`C:\Windows\System32\bash.exe`, `C:\Users\me\AppData\Local\Microsoft\WindowsApps\bash.exe`, `/wsl/bash`} {
		if !bashIsWSLPath(path) {
			t.Fatalf("bashIsWSLPath(%q) = false", path)
		}
	}
}

func TestPythonScriptUsesExplicitInterpreterWithoutShell(t *testing.T) {
	interpreter := filepath.Join(t.TempDir(), "python3")
	if runtime.GOOS == "windows" {
		interpreter += ".exe"
	}
	if err := os.WriteFile(interpreter, []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_HOST_PYTHON_BIN", interpreter)
	script := filepath.Join(t.TempDir(), "package with spaces.py")
	command, shell, err := ScriptCommand(script)
	if err != nil {
		t.Fatal(err)
	}
	if command.Path != interpreter || shell != "" || !reflect.DeepEqual(command.Args, []string{interpreter, script}) {
		t.Fatalf("Python script was routed through a shell: %#v", command.Args)
	}
}

func TestPythonScriptExplicitInvalidInterpreterDoesNotFallBack(t *testing.T) {
	for _, selected := range []string{"", "python3", filepath.Join(t.TempDir(), "missing-python")} {
		t.Run(selected, func(t *testing.T) {
			t.Setenv("DOCKPIPE_HOST_PYTHON_BIN", selected)
			if _, _, err := ScriptCommand(filepath.Join(t.TempDir(), "build.py")); err == nil {
				t.Fatal("invalid explicit Python interpreter silently fell back")
			}
		})
	}
	t.Setenv("DOCKPIPE_HOST_PYTHON_BIN", t.TempDir())
	if _, _, err := ScriptCommand(filepath.Join(t.TempDir(), "build.py")); err == nil {
		t.Fatal("directory accepted as a Python interpreter")
	}
}

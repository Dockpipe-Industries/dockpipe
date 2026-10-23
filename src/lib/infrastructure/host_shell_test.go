package infrastructure

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestHostShellSelection(t *testing.T) {
	tests := []struct {
		name      string
		selection string
		script    string
		platform  string
		want      domain.HostShell
	}{
		{name: "Windows inline", platform: "windows", want: domain.HostShellPowerShell},
		{name: "Unix inline", platform: "linux", want: domain.HostShellBash},
		{name: "Windows Bash file", script: "setup.sh", platform: "windows", want: domain.HostShellBash},
		{name: "Windows PowerShell file", script: "setup.ps1", platform: "windows", want: domain.HostShellPowerShell},
		{name: "Unix PowerShell file", script: "setup.PS1", platform: "darwin", want: domain.HostShellPwsh},
		{name: "Windows extensionless", script: "setup", platform: "windows", want: domain.HostShellPowerShell},
		{name: "explicit Bash", selection: "bash", platform: "windows", want: domain.HostShellBash},
		{name: "explicit PowerShell 7", selection: "pwsh", script: "setup.ps1", platform: "windows", want: domain.HostShellPwsh},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := hostShellForScript(test.selection, test.script, test.platform)
			if err != nil || got != test.want {
				t.Fatalf("shell = %q, %v; want %q", got, err, test.want)
			}
		})
	}
	if _, err := hostShellForScript("powershell", "", "linux"); err == nil {
		t.Fatal("Windows PowerShell must not silently fall back to pwsh or Bash")
	}
	if _, err := hostShellForScript("pwsh", "setup.sh", "windows"); err == nil {
		t.Fatal("conflicting script interpreter must fail")
	}
}

func TestHostShellMissingRequestedInterpreterFails(t *testing.T) {
	previous := hostShellLookPathFn
	t.Cleanup(func() { hostShellLookPathFn = previous })
	hostShellLookPathFn = func(name string) (string, error) {
		if name != "pwsh" {
			t.Fatalf("unexpected shell fallback to %q", name)
		}
		return "", exec.ErrNotFound
	}
	if err := RunHostCommandWithShell("Write-Output hello", os.Environ(), "pwsh"); err == nil || !strings.Contains(err.Error(), `requested host shell "pwsh"`) {
		t.Fatalf("missing shell result = %v", err)
	}
}

func TestHostShellNativeDefault(t *testing.T) {
	workdir := t.TempDir()
	env := hostShellTestEnv(workdir)
	command := "printf '%s' \"$DOCKPIPE_LITERAL\" > marker.txt"
	if runtime.GOOS == "windows" {
		command = "[IO.File]::WriteAllText((Join-Path $PWD 'marker.txt'), $env:DOCKPIPE_LITERAL)"
	}
	env = append(env, "DOCKPIPE_LITERAL=literal 'quotes' ; $value & spaces")
	if err := RunHostCommand(command, env); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(workdir, "marker.txt"))
	if err != nil || string(body) != "literal 'quotes' ; $value & spaces" {
		t.Fatalf("native shell output = %q, %v", body, err)
	}
	assertHostShellRunArtifactsRemoved(t, workdir)
}

func TestHostShellPowerShellScriptLiteralPathAndCWD(t *testing.T) {
	shell := availableTestPowerShell(t)
	workdir := t.TempDir()
	scriptDir := filepath.Join(workdir, "space 'quoted' 日本語")
	if err := os.Mkdir(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(scriptDir, "script.ps1")
	body := "[IO.File]::WriteAllText((Join-Path $PWD 'marker.txt'), $env:DOCKPIPE_LITERAL)\n"
	if err := os.WriteFile(script, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	env := append(hostShellTestEnv(workdir), "DOCKPIPE_LITERAL=héllo 'literal' $()")
	if err := RunHostScriptWithShell(script, env, shell); err != nil {
		t.Fatal(err)
	}
	marker, err := os.ReadFile(filepath.Join(workdir, "marker.txt"))
	if err != nil || string(marker) != "héllo 'literal' $()" {
		t.Fatalf("script marker = %q, %v", marker, err)
	}
	assertHostShellRunArtifactsRemoved(t, workdir)
}

func TestHostShellPowerShellInlineUnicode(t *testing.T) {
	shell := availableTestPowerShell(t)
	workdir := t.TempDir()
	command := "[IO.File]::WriteAllText((Join-Path $PWD 'marker.txt'), 'héllo 日本語')"
	if err := RunHostCommandWithShell(command, hostShellTestEnv(workdir), shell); err != nil {
		t.Fatal(err)
	}
	marker, err := os.ReadFile(filepath.Join(workdir, "marker.txt"))
	if err != nil || string(marker) != "héllo 日本語" {
		t.Fatalf("inline Unicode = %q, %v", marker, err)
	}
}

func TestHostShellPowerShellNativeFailureRecorded(t *testing.T) {
	shell := availableTestPowerShell(t)
	workdir := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := "& '" + strings.ReplaceAll(executable, "'", "''") + "' '-test.run=^TestHostShellNativeExitHelper$'"
	env := append(hostShellTestEnv(workdir), "DOCKPIPE_SHELL_TEST_CHILD=1")
	err = RunHostCommandWithShell(command, env, shell)
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 23 {
		t.Fatalf("native exit code was not propagated: %v", err)
	}
	snapshot, err := ReadExecutionSnapshot(context.Background(), workdir, false, true)
	if err != nil || len(snapshot.Executions) != 1 {
		t.Fatalf("execution history = %+v, %v", snapshot, err)
	}
	record := snapshot.Executions[0]
	if record.State != "failed" || record.ExitCode == nil || *record.ExitCode != 23 {
		t.Fatalf("failure not recorded: %+v", record)
	}
	assertHostShellRunArtifactsRemoved(t, workdir)
}

func TestHostShellPowerShellTerminatingError(t *testing.T) {
	shell := availableTestPowerShell(t)
	for _, command := range []string{
		"throw 'expected fixture failure'",
		"Get-Item -LiteralPath './missing-fixture-file'",
	} {
		workdir := t.TempDir()
		if err := RunHostCommandWithShell(command, hostShellTestEnv(workdir), shell); err == nil {
			t.Fatalf("PowerShell exception must fail the step: %s", command)
		}
		assertHostShellRunArtifactsRemoved(t, workdir)
	}
}

func TestHostShellPowerShellScriptExitCode(t *testing.T) {
	shell := availableTestPowerShell(t)
	workdir := t.TempDir()
	script := filepath.Join(workdir, "exit.ps1")
	if err := os.WriteFile(script, []byte("exit 19\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := RunHostScriptWithShell(script, hostShellTestEnv(workdir), shell)
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 19 {
		t.Fatalf("script exit code was not propagated: %v", err)
	}
}

func TestHostShellWorkflowSchema(t *testing.T) {
	tests := []struct {
		name  string
		step  string
		valid bool
	}{
		{name: "native", step: "kind: host\n    cmd: Write-Output hello", valid: true},
		{name: "bash", step: "kind: host\n    shell: bash\n    cmd: echo hello", valid: true},
		{name: "powershell", step: "kind: host\n    shell: powershell\n    cmd: Write-Output hello", valid: true},
		{name: "pwsh", step: "kind: host\n    shell: pwsh\n    cmd: Write-Output hello", valid: true},
		{name: "unknown", step: "kind: host\n    shell: cmd\n    cmd: echo hello"},
		{name: "container", step: "shell: bash\n    cmd: echo hello"},
		{name: "builtin", step: "kind: host\n    shell: bash\n    host_builtin: package_build_store"},
		{name: "package", step: "shell: bash\n    package: example\n    workflow: child"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yml")
			body := "name: fixture\nsteps:\n  - " + test.step + "\n"
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			err := ValidateResolvedWorkflowYAML(path)
			if (err == nil) != test.valid {
				t.Fatalf("valid = %v, want %v; error = %v", err == nil, test.valid, err)
			}
		})
	}
}

func TestHostShellNativeExitHelper(t *testing.T) {
	if os.Getenv("DOCKPIPE_SHELL_TEST_CHILD") == "1" {
		os.Exit(23)
	}
}

func TestHostShellNativeWindowsPATH(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows PATH conversion")
	}
	for _, currentPath := range []string{
		`C:\tools\go\bin;C:\Windows\System32`,
		`C:\tools\go\bin`,
	} {
		env := prepareHostNativeEnv([]string{"PATH=" + currentPath})
		if got := envGet(env, "PATH"); got != currentPath {
			t.Fatalf("native PATH changed to %q from %q", got, currentPath)
		}
	}
	env := prepareHostNativeEnv([]string{"PATH=/c/tools/go/bin:/mnt/d/Program Files/tools/bin:/c/Windows/System32"})
	if got := envGet(env, "PATH"); got != `C:\tools\go\bin;D:\Program Files\tools\bin;C:\Windows\System32` {
		t.Fatalf("Bash PATH was not converted for native child: %q", got)
	}
}

func availableTestPowerShell(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("powershell.exe"); err != nil {
			t.Fatalf("native Windows PowerShell unavailable: %v", err)
		}
		return "powershell"
	}
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("PowerShell 7 is not installed on this Unix host")
	}
	return "pwsh"
}

func hostShellTestEnv(workdir string) []string {
	return append(os.Environ(),
		"DOCKPIPE_WORKDIR="+workdir,
		"DOCKPIPE_STEP_CWD="+workdir,
		"DOCKPIPE_SKIP_HOST_CLEANUP=1",
		"DOCKPIPE_HOST_SCRIPT_SPINNER=false",
	)
}

func assertHostShellRunArtifactsRemoved(t *testing.T, workdir string) {
	t.Helper()
	entries, err := os.ReadDir(HostRunsDir(workdir))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() == "executions" {
			continue
		}
		t.Fatalf("active host run artifact remains after exit: %s", entry.Name())
	}
}

func TestHostShellPowerShellProgressDoesNotPolluteOutput(t *testing.T) {
	shell := availableTestPowerShell(t)
	script := filepath.Join(t.TempDir(), "progress.ps1")
	body := "Write-Progress -Activity 'fixture' -Status 'checking' -PercentComplete 50\nWrite-Output 'fixture-complete'\n"
	if err := os.WriteFile(script, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	command, env, err := hostScriptCommand(script, os.Environ(), shell)
	if err != nil {
		t.Fatal(err)
	}
	command.Env = env
	output, err := command.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "fixture-complete" {
		t.Fatalf("PowerShell progress polluted host output: %q, %v", output, err)
	}
}

func TestHostPythonScriptLifecycleAndLiteralPath(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		python, err = exec.LookPath("python")
	}
	if err != nil {
		t.Fatal("Python 3 is required for the native Python host-script test")
	}
	workdir := t.TempDir()
	script := filepath.Join(workdir, "literal 'quoted' 日本語.py")
	body := "import os\nfrom pathlib import Path\nPath('marker.txt').write_text(os.environ['DOCKPIPE_LITERAL'], encoding='utf-8')\n"
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	environment := append(hostShellTestEnv(workdir), "DOCKPIPE_HOST_PYTHON_BIN="+python, "DOCKPIPE_LITERAL=héllo ; $() 'literal'")
	if err := RunHostScript(script, environment); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(workdir, "marker.txt"))
	if err != nil || string(data) != "héllo ; $() 'literal'" {
		t.Fatalf("Python output = %q, %v", data, err)
	}
	assertHostShellRunArtifactsRemoved(t, workdir)
	if err := os.WriteFile(script, []byte("raise SystemExit(23)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	err = RunHostScript(script, environment)
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 23 {
		t.Fatalf("Python exit status = %v", err)
	}
	assertHostShellRunArtifactsRemoved(t, workdir)
}

func TestHostPythonOverrideAndExplicitShell(t *testing.T) {
	for _, selected := range []string{"", "python3", filepath.Join(t.TempDir(), "missing-python"), t.TempDir()} {
		_, _, err := hostScriptCommand("script.py", []string{"DOCKPIPE_HOST_PYTHON_BIN=" + selected}, "")
		if err == nil {
			t.Fatalf("invalid override %q fell back", selected)
		}
	}
	previous := hostShellLookPathFn
	t.Cleanup(func() { hostShellLookPathFn = previous })
	hostShellLookPathFn = func(name string) (string, error) {
		if name != "pwsh" {
			t.Fatalf("explicit shell changed: %s", name)
		}
		return "/fixture/pwsh", nil
	}
	command, _, err := hostScriptCommand("script.py", []string{"DOCKPIPE_HOST_PYTHON_BIN="}, "pwsh")
	if err != nil || command.Path != "/fixture/pwsh" {
		t.Fatalf("explicit shell semantics changed: %v", err)
	}
}

func TestHostNativeEnvironmentCasing(t *testing.T) {
	environment := []string{"Path=first", "PATH=last", "PATHEXT=.EXE;.CMD"}
	want := "first"
	if runtime.GOOS == "windows" {
		want = "last"
	}
	if value := envGet(environment, "Path"); value != want {
		t.Fatalf("native key lookup = %q, want %q", value, want)
	}
	updated := upsertEnv(environment, "PATH", "selected")
	paths := 0
	for _, entry := range updated {
		key, _, _ := strings.Cut(entry, "=")
		if domain.NativeEnvKey(key) == domain.NativeEnvKey("PATH") {
			paths++
		}
	}
	if paths != 1 || envGet(updated, "PATH") != "selected" {
		t.Fatal("native upsert did not preserve one explicit winning value")
	}
	if runtime.GOOS != "windows" && envGet(updated, "Path") != "first" {
		t.Fatal("Unix distinct environment key changed")
	}
}

package application

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/vscode"
)

func writeVSCodeFixture(t *testing.T, root, name, content string) {
	t.Helper()
	directory := filepath.Join(root, ".vscode")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestVSCodeCatalogProvenanceAndUnsupportedEntries(t *testing.T) {
	root := t.TempDir()
	writeVSCodeFixture(t, root, "tasks.json", `{
 // Comments and trailing commas are part of the source format.
 "tasks": [
   {"label":"Build", "type":"process", "command":"echo", "args":["built"]},
   {"label":"Extension build", "type":"contributed-build", "command":"unknown"},
 ],
}`)
	writeVSCodeFixture(t, root, "launch.json", `{"configurations":[
 {"name":"Server", "type":"coreclr", "request":"launch", "program":"${workspaceFolder}/server.dll"},
 {"name":"Attach", "type":"node", "request":"attach", "processId":"1"}
]}`)
	entries, err := listCatalogWorkflows(root, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected all four editor entries, got %#v", entries)
	}
	for _, entry := range entries {
		if entry.Source != "vscode" || entry.SourcePath == "" || entry.SourceName != entry.DisplayName || entry.ConfigPath != "" {
			t.Fatalf("missing or misleading provenance: %#v", entry)
		}
		if entry.Supported == nil {
			t.Fatalf("support must be explicit: %#v", entry)
		}
		if entry.DisplayName == "Server" && (entry.Category != "app" || len(entry.Warnings) == 0) {
			t.Fatalf("launch entry must be an app with a debugger warning: %#v", entry)
		}
		if entry.DisplayName == "Extension build" || entry.DisplayName == "Attach" {
			if *entry.Supported || entry.Cloneable || entry.UnsupportedReason == "" {
				t.Fatalf("unsupported entry is executable: %#v", entry)
			}
		} else if !*entry.Supported || !entry.Cloneable {
			t.Fatalf("expected supported entry: %#v", entry)
		}
	}
}

func TestVSCodeCloneIsIndependentPortableAndExclusive(t *testing.T) {
	root := t.TempDir()
	source := `{"tasks":[{"label":"Build", "type":"process", "command":"printf", "args":["%s", "${env:DOCKPIPE_IMPORT_FIXTURE_SECRET}"], "options":{"cwd":"${workspaceFolder}/src"}}]}`
	writeVSCodeFixture(t, root, "tasks.json", source)
	t.Setenv("DOCKPIPE_IMPORT_FIXTURE_SECRET", "do-not-copy-this-process-value")
	catalog, _, err := discoverVSCodeWorkflows(root)
	if err != nil {
		t.Fatal(err)
	}
	path, err := cloneVSCodeWorkflow(root, catalog.Entries[0].ID, "local-build")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if strings.Contains(text, root) || strings.Contains(text, "do-not-copy-this-process-value") || !strings.Contains(text, "DOCKPIPE_IMPORT_FIXTURE_SECRET") {
		t.Fatalf("clone expanded workspace/process values: %s", text)
	}
	if _, err := cloneVSCodeWorkflow(root, catalog.Entries[0].ID, "local-build"); err == nil {
		t.Fatal("repeat clone overwrote an authored workflow")
	}
	if _, err := cloneVSCodeWorkflow(root, catalog.Entries[0].ID, "../escape"); err == nil {
		t.Fatal("clone escaped its destination")
	}
	after, err := os.ReadFile(filepath.Join(root, ".vscode", "tasks.json"))
	if err != nil || string(after) != source {
		t.Fatal("clone changed the VS Code source")
	}
	if err := os.RemoveAll(filepath.Join(root, ".vscode")); err != nil {
		t.Fatal(err)
	}
	loaded, err := infrastructure.LoadWorkflow(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := domain.ValidateLoadedWorkflow(loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.Name != "local-build" || len(loaded.Steps) != 1 {
		t.Fatalf("not an independent native workflow: %#v", loaded)
	}
	if err := infrastructure.ValidateResolvedWorkflowYAML(path); err != nil {
		t.Fatalf("clone does not pass the public YAML schema: %v", err)
	}
}

func TestVSCodeCommandArgumentsDoNotBecomeShellCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX child-shell integration; Windows adapter overrides have separate fixtures")
	}
	for _, shell := range []string{"", "/bin/sh", "/bin/bash"} {
		t.Run(shell, func(t *testing.T) {
			root := t.TempDir()
			payload := "one 'quote'\n$(touch " + filepath.Join(root, "injected") + ") ; three"
			command := vscode.Command{
				Name: "literal args", Executable: "printf", Args: []string{"%s", "${input:value}"},
				CWD: "${workspaceFolder}", Shell: shell != "", ShellExecutable: shell,
			}
			if command.Shell {
				command.ShellArgs = []string{"-c"}
			}
			body, err := lowerVSCodeCommand(command)
			if err != nil {
				t.Fatal(err)
			}
			child := exec.Command("bash", "-c", body)
			child.Env = append(os.Environ(), "DOCKPIPE_WORKDIR="+root, vscode.InputEnvName("value")+"="+payload)
			output, err := child.CombinedOutput()
			if err != nil || string(output) != payload {
				t.Fatalf("argv changed: %v\n%s\nbody:\n%s", err, output, body)
			}
			if _, err := os.Stat(filepath.Join(root, "injected")); !os.IsNotExist(err) {
				t.Fatal("input was executed as shell code")
			}
		})
	}
}

func TestVSCodeInputDefaultsPasswordAndChoices(t *testing.T) {
	root := t.TempDir()
	writeVSCodeFixture(t, root, "tasks.json", `{"inputs":[
 {"id":"mode", "type":"pickString", "options":["debug","release"], "default":"debug"},
 {"id":"token", "type":"promptString", "password":true, "default":"never-export-password-default"}
],"tasks":[{"label":"Input", "type":"process", "command":"printf", "args":["%s %s", "${input:mode}", "${input:token}"]}]}`)
	catalog, _, err := discoverVSCodeWorkflows(root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := catalog.Resolve(context.Background(), catalog.Entries[0].ID, vscode.Values{Symbolic: true})
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := lowerVSCodeWorkflow(plan)
	if err != nil {
		t.Fatal(err)
	}
	if workflow.Vars[vscode.InputEnvName("mode")] != "debug" || workflow.Vars[vscode.InputEnvName("token")] != "" {
		t.Fatal("input defaults or passwords mapped incorrectly")
	}
	if strings.Contains(workflow.Steps[0].Cmd, "never-export-password-default") {
		t.Fatal("password default was serialized")
	}
	path, err := cloneVSCodeWorkflow(root, catalog.Entries[0].ID, "input-clone")
	if err != nil {
		t.Fatal(err)
	}
	cloned, err := infrastructure.LoadWorkflow(path)
	if err != nil {
		t.Fatal(err)
	}
	fields := buildCatalogWorkflowInputs(path, cloned)
	if len(fields) != 2 || fields[0].EnvName != vscode.InputEnvName("mode") || fields[1].Attributes["password"] != "true" || fields[1].DefaultValue != "" {
		t.Fatalf("cloned inputs lost their launcher metadata: %#v", fields)
	}
	if runtime.GOOS != "windows" {
		child := exec.Command("bash", "-c", workflow.Steps[0].Cmd)
		child.Env = append(os.Environ(), "DOCKPIPE_WORKDIR="+root, vscode.InputEnvName("mode")+"=invalid", vscode.InputEnvName("token")+"=supplied")
		if err := child.Run(); err == nil {
			t.Fatal("invalid pickString choice was accepted")
		}
	}
}

func TestVSCodeTaskEnvironmentDoesNotChangeSubstitution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture")
	}
	command := vscode.Command{
		Executable: "printf", Args: []string{"%s:%s:%s:%s", "${env:FIXTURE_ENV}", "${workspaceFolderBasename}", "${userHome}", "${workspaceFolder}"},
		CWD: "${workspaceFolder}", Env: map[string]string{"FIXTURE_ENV": "child", "HOME": "/child-home", "DOCKPIPE_WORKDIR": "/child-workdir"},
	}
	body, err := lowerVSCodeCommand(command)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	child := exec.Command("bash", "-c", body)
	child.Env = append(os.Environ(), "DOCKPIPE_WORKDIR="+root, "FIXTURE_ENV=parent", "HOME=/parent-home")
	output, err := child.CombinedOutput()
	if err != nil || string(output) != "parent:"+filepath.Base(root)+":/parent-home:"+root {
		t.Fatalf("substituted child environment: %v\n%s\n%s", err, output, body)
	}
}

func TestVSCodeShellExecutableWithSpaces(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture; Windows native path execution remains unverified")
	}
	for _, shell := range []string{"/bin/sh", "/bin/bash"} {
		t.Run(shell, func(t *testing.T) {
			root := t.TempDir()
			program := filepath.Join(root, "tool with spaces")
			if err := os.WriteFile(program, []byte("#!/bin/sh\nprintf '%s' \"$1\"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			body, err := lowerVSCodeCommand(vscode.Command{
				Executable: program, Args: []string{"literal argument"}, CWD: "${workspaceFolder}",
				Shell: true, ShellExecutable: shell,
			})
			if err != nil {
				t.Fatal(err)
			}
			child := exec.Command("bash", "-c", body)
			child.Env = append(os.Environ(), "DOCKPIPE_WORKDIR="+root)
			output, err := child.CombinedOutput()
			if err != nil || string(output) != "literal argument" {
				t.Fatalf("shell split the executable path: %v\n%s\n%s", err, output, body)
			}
		})
	}
}

func TestVSCodeWorkspaceStopsAtRepositoryBoundaries(t *testing.T) {
	for _, gitFile := range []bool{false, true} {
		root := t.TempDir()
		writeVSCodeFixture(t, root, "tasks.json", `{"tasks":[{"label":"Parent", "type":"process", "command":"echo"}]}`)
		repository := filepath.Join(root, "repository")
		nested := filepath.Join(repository, "nested")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatal(err)
		}
		marker := filepath.Join(repository, ".git")
		if gitFile {
			if err := os.WriteFile(marker, []byte("gitdir: /unused\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		} else if err := os.Mkdir(marker, 0o755); err != nil {
			t.Fatal(err)
		}
		workspace, err := vscodeWorkspace(nested)
		if err != nil || workspace != repository {
			t.Fatalf("workspace crossed the repository boundary: %s, %v", workspace, err)
		}
		writeVSCodeFixture(t, repository, "tasks.json", `{"tasks":[{"label":"Local", "type":"process", "command":"echo"}]}`)
		catalog, workspace, err := discoverVSCodeWorkflows(nested)
		if err != nil || workspace != repository || len(catalog.Entries) != 1 || catalog.Entries[0].Name != "Local" {
			t.Fatalf("nested project discovery failed: %s, %#v, %v", workspace, catalog, err)
		}
	}
}

func TestVSCodePowerShellArgumentsStayLiteral(t *testing.T) {
	powershell, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("PowerShell is not installed")
	}
	if runtime.GOOS == "windows" {
		t.Skip("outer host Bash path mapping needs native Windows qualification")
	}
	root := t.TempDir()
	payload := "one 'quote'\n$(Get-Location); three"
	command := vscode.Command{
		Executable: "Write-Output", Args: []string{"${input:value}"}, Shell: true,
		ShellExecutable: powershell, ShellArgs: []string{"-NoProfile", "-Command"}, CWD: "${workspaceFolder}",
	}
	body, err := lowerVSCodeCommand(command)
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command("bash", "-c", body)
	child.Env = append(os.Environ(), "DOCKPIPE_WORKDIR="+root, vscode.InputEnvName("value")+"="+payload)
	output, err := child.CombinedOutput()
	if err != nil || strings.TrimSuffix(string(output), "\n") != payload {
		t.Fatalf("PowerShell argument changed: %v\n%s\n%s", err, output, body)
	}
}

func TestVSCodeCLIInputWhitespaceAndOverridePrecedence(t *testing.T) {
	key := vscode.InputEnvName("value")
	value := "  exact input\nwith whitespace  "
	environment := make(map[string]string)
	MergeExtraEnvCLIIntoSteps(environment, []string{key + "=" + value, "LEGACY= trimmed "}, nil)
	mergeVSCodeInputEnv(environment, []string{key + "=" + value, "LEGACY= trimmed "}, nil)
	if environment[key] != value || environment["LEGACY"] != "trimmed" {
		t.Fatalf("input whitespace or legacy semantics changed: %#v", environment)
	}
	mergeVSCodeInputEnv(environment, []string{key + "=ignored"}, []string{key + "=from-var"})
	if environment[key] != value {
		t.Fatal("input --env bypassed an explicit --var lock")
	}
	mergeVSCodeInputEnv(environment, []string{key + "="}, nil)
	if environment[key] != "" {
		t.Fatal("explicit empty input did not replace the previous value")
	}
}

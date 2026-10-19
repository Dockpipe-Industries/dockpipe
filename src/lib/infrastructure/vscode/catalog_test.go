package vscode

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, tasks string, launch string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".vscode"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"tasks.json": tasks, "launch.json": launch} {
		if text != "" {
			if err := os.WriteFile(filepath.Join(root, ".vscode", name), []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

func discover(t *testing.T, root string, options Options) *Catalog {
	t.Helper()
	catalog, err := Discover(context.Background(), root, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Diagnostics) > 0 {
		t.Fatalf("diagnostics: %+v", catalog.Diagnostics)
	}
	return catalog
}

func TestJSONCDependenciesVariablesAndPlatform(t *testing.T) {
	root := fixture(t, `{
 // A URL and comment-looking string must remain intact.
 "version":"2.0.0", "options":{"env":{"GLOBAL":"base"}},
 "tasks":[
  {"label":"prepare","type":"process","command":"echo","args":["https://host/a//b","/*literal*/"],},
  {"label":"build","type":"process","command":"base","dependsOn":["prepare"],
   "linux":{"command":"linux-tool","options":{"env":{"PLATFORM":"yes"}}},
   "options":{"cwd":"${workspaceFolder}/sub","env":{"TOKEN":"${env:TOKEN}"}},
   "args":["${input:target}","${config:build.value}","${workspaceFolderBasename}"]},
 ],
 "inputs":[{"id":"target","type":"pickString","options":["debug","release"],"default":"debug"}],
}`, "")
	catalog := discover(t, root, Options{GOOS: "linux", Environment: map[string]string{"TOKEN": "sensitive"}})
	if len(catalog.Entries) != 2 || !catalog.Entries[1].Supported {
		t.Fatalf("entries: %+v", catalog.Entries)
	}
	plan, err := catalog.Resolve(context.Background(), catalog.Entries[1].ID, Values{Inputs: map[string]string{"target": "release"}, Configuration: map[string]string{"build.value": "yes"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Commands) != 2 {
		t.Fatalf("commands: %+v", plan.Commands)
	}
	command := plan.Commands[1]
	if command.Executable != "linux-tool" || command.Shell || command.CWD != filepath.Join(root, "sub") {
		t.Fatalf("command: %+v", command)
	}
	if command.Env["GLOBAL"] != "base" || command.Env["PLATFORM"] != "yes" || command.Env["TOKEN"] != "sensitive" {
		t.Fatalf("env: %+v", command.Env)
	}
	if command.Args[0] != "release" || command.Args[1] != "yes" {
		t.Fatal(command.Args)
	}
	if plan.Commands[0].Args[0] != "https://host/a//b" || plan.Commands[0].Args[1] != "/*literal*/" {
		t.Fatal(plan.Commands[0].Args)
	}
	symbolic, err := catalog.Resolve(context.Background(), catalog.Entries[1].ID, Values{Symbolic: true})
	if err != nil {
		t.Fatal(err)
	}
	if symbolic.Commands[1].Env["TOKEN"] != "${env:TOKEN}" || symbolic.Commands[1].CWD != "${workspaceFolder}/sub" || symbolic.Commands[1].Args[0] != "${input:target}" {
		t.Fatalf("symbolic: %+v", symbolic)
	}
}

func TestUnsupportedEntriesRemainVisible(t *testing.T) {
	cases := []struct{ name, task, reason string }{
		{"extension", `{"label":"x","type":"npm","script":"build"}`, "requires a VS Code extension"},
		{"background", `{"label":"x","command":"watch","isBackground":true}`, "background readiness"},
		{"parallel", `{"label":"x","dependsOn":["a","b"]}`, "parallel task dependencies"},
		{"missing", `{"label":"x","dependsOn":"absent"}`, "was not found"},
		{"cycle", `{"label":"x","dependsOn":"x"}`, "cycle"},
		{"editor", `{"label":"x","command":"${file}"}`, "editor context"},
		{"command input", `{"label":"x","command":"${command:extension.secret}"}`, "editor context"},
		{"argument object", `{"label":"x","command":"echo","args":[{"value":"a","quoting":"weak"}]}`, "argument objects"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			root := fixture(t, `{"version":"2.0.0","tasks":[`+item.task+`]}`, "")
			catalog := discover(t, root, Options{})
			if len(catalog.Entries) != 1 || catalog.Entries[0].Supported || !strings.Contains(catalog.Entries[0].UnsupportedReason, item.reason) {
				t.Fatalf("entries: %+v", catalog.Entries)
			}
			if _, err := catalog.Resolve(context.Background(), catalog.Entries[0].ID, Values{}); err == nil {
				t.Fatal("unsupported entry resolved")
			}
		})
	}
}

func TestLaunchApplicationsAndPreLaunchTask(t *testing.T) {
	root := fixture(t, `{"tasks":[{"label":"build","type":"process","command":"make"}]}`, `{
 "configurations":[
 {"name":"Node","type":"node","request":"launch","program":"${workspaceFolder}/app.js","args":["two words"],"preLaunchTask":"build"},
 {"name":"Python","type":"debugpy","request":"launch","module":"http.server","args":["8000"]},
 {"name":"Dotnet","type":"coreclr","request":"launch","program":"${workspaceFolder}/app.dll"},
 {"name":"Attach","type":"node","request":"attach","processId":"42"},
 {"name":"Browser","type":"chrome","request":"launch","url":"https://example.com"}
 ]}`)
	catalog := discover(t, root, Options{GOOS: "linux"})
	for index := 1; index <= 3; index++ {
		if !catalog.Entries[index].Supported {
			t.Fatalf("entry %+v", catalog.Entries[index])
		}
		plan, err := catalog.Resolve(context.Background(), catalog.Entries[index].ID, Values{})
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Warnings) != 1 {
			t.Fatal(plan.Warnings)
		}
		command := plan.Commands[len(plan.Commands)-1]
		switch index {
		case 1:
			if len(plan.Commands) != 2 || command.Executable != "node" || command.Args[1] != "two words" {
				t.Fatalf("node: %+v", plan)
			}
		case 2:
			if command.Executable != "python3" || strings.Join(command.Args, " ") != "-m http.server 8000" {
				t.Fatal(command)
			}
		case 3:
			if command.Executable != "dotnet" || command.Args[0] != filepath.Join(root, "app.dll") {
				t.Fatal(command)
			}
		}
	}
	if catalog.Entries[4].Supported || catalog.Entries[5].Supported {
		t.Fatal("unsupported debugger features presented as runnable")
	}
}

func TestInputValidationAndNoRecursiveVariableEvaluation(t *testing.T) {
	root := fixture(t, `{"tasks":[{"label":"x","type":"process","command":"echo","args":["${input:choice}","${env:VALUE}"]}],"inputs":[{"id":"choice","type":"pickString","options":["yes"]}]}`, "")
	catalog := discover(t, root, Options{Environment: map[string]string{"VALUE": "${command:never-run}"}})
	entry := catalog.Entries[0]
	if _, err := catalog.Resolve(context.Background(), entry.ID, Values{}); err == nil {
		t.Fatal("missing input accepted")
	}
	if _, err := catalog.Resolve(context.Background(), entry.ID, Values{Inputs: map[string]string{"choice": "no"}}); err == nil {
		t.Fatal("invalid pick accepted")
	}
	plan, err := catalog.Resolve(context.Background(), entry.ID, Values{Inputs: map[string]string{"choice": "yes"}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Commands[0].Args[1] != "${command:never-run}" {
		t.Fatal("recursively evaluated environment")
	}
	if InputEnvName("a-b") == InputEnvName("a_b") {
		t.Fatal("input environment collision")
	}
}

func TestDocumentLimitsCancellationAndMalformedJSONC(t *testing.T) {
	root := fixture(t, `{/* never closed`, "")
	catalog, err := Discover(context.Background(), root, Options{})
	if err != nil || len(catalog.Diagnostics) != 1 {
		t.Fatalf("%+v %v", catalog, err)
	}
	root = fixture(t, strings.Repeat(" ", MaxDocumentBytes+1), "")
	catalog, err = Discover(context.Background(), root, Options{})
	if err != nil || len(catalog.Diagnostics) != 1 || !strings.Contains(catalog.Diagnostics[0].Message, "exceeds") {
		t.Fatalf("%+v %v", catalog, err)
	}
	if _, err := normalizeJSONC([]byte(strings.Repeat("[", 65) + strings.Repeat("]", 65))); err == nil {
		t.Fatal("deep JSON accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Discover(ctx, t.TempDir(), Options{}); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestDuplicateLabelsAndLaunchNames(t *testing.T) {
	root := fixture(t, `{"tasks":[{"label":"x","command":"one"},{"label":"x","command":"two"}]}`, `{"configurations":[{"name":"dup","type":"node","request":"launch","program":"one"},{"name":"dup","type":"node","request":"launch","program":"two"}]}`)
	catalog := discover(t, root, Options{})
	for _, entry := range catalog.Entries {
		if entry.Supported {
			t.Fatal(catalog.Entries)
		}
	}
}

func TestGlobalOptionsAndShellRemainSeparate(t *testing.T) {
	root := fixture(t, `{"type":"shell","options":{"shell":{"executable":"pwsh","args":["-NoProfile","-Command"]}},"tasks":[{"label":"x","command":"Get-Item .","args":["two words"],"windows":{"command":"Get-ChildItem ."}}]}`, "")
	catalog := discover(t, root, Options{GOOS: "windows", Environment: map[string]string{}})
	plan, err := catalog.Resolve(context.Background(), catalog.Entries[0].ID, Values{})
	if err != nil {
		t.Fatal(err)
	}
	command := plan.Commands[0]
	if !command.Shell || command.Executable != "Get-ChildItem ." || command.ShellExecutable != "pwsh" || len(command.ShellArgs) != 2 || command.Args[0] != "two words" {
		t.Fatalf("shell: %+v", command)
	}
}

func TestDiscoveryDoesNotTouchOrRunConfigurations(t *testing.T) {
	root := fixture(t, `{"tasks":[{"label":"x","command":"touch should-not-exist"}]}`, "")
	source := filepath.Join(root, ".vscode", "tasks.json")
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	catalog := discover(t, root, Options{})
	if _, err := catalog.Resolve(context.Background(), catalog.Entries[0].ID, Values{Symbolic: true}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(source)
	if err != nil || string(before) != string(after) {
		t.Fatal("configuration was mutated")
	}
	if _, err := os.Stat(filepath.Join(root, "should-not-exist")); !os.IsNotExist(err) {
		t.Fatal("discovery ran a command")
	}
}

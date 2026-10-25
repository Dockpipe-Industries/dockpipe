package application

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestWorkflowSelectionPreservesFlagErrorOrder(t *testing.T) {
	cases := []struct {
		name    string
		request workflowSelectionRequest
		want    string
	}{
		{
			name: "workflow and file before package",
			request: workflowSelectionRequest{
				workflowName:      "build",
				workflowFile:      "config.yml",
				packageName:       "tools",
				compileDepsWanted: true,
			},
			want: "use only one of --workflow and --workflow-file",
		},
		{
			name: "package requires name before file conflict",
			request: workflowSelectionRequest{
				workflowFile:      "config.yml",
				packageName:       "tools",
				compileDepsWanted: true,
			},
			want: "--package requires --workflow",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := selectWorkflow(test.request, func(string) (*domain.Workflow, error) {
				t.Fatal("invalid selection reached workflow loading")
				return nil, nil
			})
			if err == nil || err.Error() != test.want {
				t.Fatalf("got %v, want %q", err, test.want)
			}
		})
	}
}

func TestWorkflowSelectionFileUsesCapturedLoaderAndAbsoluteConfig(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	path := filepath.Join(root, "config.yml")
	if err := os.WriteFile(path, []byte("name: fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wantWorkflow := &domain.Workflow{Steps: []domain.Step{{Kind: "host", Cmd: "echo fixture"}}}
	calls := 0
	selection, err := selectWorkflow(workflowSelectionRequest{
		effectiveWorkdir:  t.TempDir(),
		workflowFile:      "config.yml",
		compileDepsWanted: true,
	}, func(config string) (*domain.Workflow, error) {
		calls++
		if config != path {
			t.Fatalf("loader received %q, want %q", config, path)
		}
		return wantWorkflow, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || selection.workflow != wantWorkflow || selection.root != root || selection.config != path || !selection.stepsMode {
		t.Fatalf("unexpected file selection: calls=%d, selection=%+v", calls, selection)
	}
	if selection.editorWorkdirOverride != "" || selection.editorLaunchWithoutDebugger {
		t.Fatalf("file selection changed editor behavior: %+v", selection)
	}

	loadErr := errors.New("invalid workflow fixture")
	_, err = selectWorkflow(workflowSelectionRequest{workflowFile: path}, func(string) (*domain.Workflow, error) {
		return nil, loadErr
	})
	if !errors.Is(err, loadErr) || err.Error() != "parse config: invalid workflow fixture" {
		t.Fatalf("loader error lost context or cause: %v", err)
	}
}

func TestWorkflowSelectionWithoutWorkflowKeepsCommandMode(t *testing.T) {
	selection, err := selectWorkflow(workflowSelectionRequest{compileDepsWanted: true}, func(string) (*domain.Workflow, error) {
		t.Fatal("command-only selection reached workflow loading")
		return nil, nil
	})
	if err != nil || selection != (workflowSelection{}) {
		t.Fatalf("unexpected command selection: %+v, %v", selection, err)
	}
}

func TestWorkflowSelectionVSCodeReturnsWorkdirAndLaunchNotice(t *testing.T) {
	cases := []struct {
		name       string
		filename   string
		content    string
		wantNotice bool
	}{
		{
			name: "task", filename: "tasks.json",
			content: `{"tasks":[{"label":"Build","type":"process","command":"echo","args":["built"]}]}`,
		},
		{
			name: "launch", filename: "launch.json", wantNotice: true,
			content: `{"configurations":[{"name":"Server","type":"coreclr","request":"launch","program":"${workspaceFolder}/server.dll"}]}`,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeVSCodeFixture(t, root, test.filename, test.content)
			nested := filepath.Join(root, "src")
			if err := os.Mkdir(nested, 0o755); err != nil {
				t.Fatal(err)
			}
			catalog, _, err := discoverVSCodeWorkflows(nested)
			if err != nil {
				t.Fatal(err)
			}
			if len(catalog.Entries) != 1 {
				t.Fatalf("expected one editor workflow, got %+v", catalog.Entries)
			}
			selection, err := selectWorkflow(workflowSelectionRequest{
				effectiveWorkdir:  nested,
				workflowName:      catalog.Entries[0].ID,
				compileDepsWanted: true,
			}, func(string) (*domain.Workflow, error) {
				t.Fatal("editor workflow attempted YAML loading")
				return nil, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if selection.root != root || selection.editorWorkdirOverride != root || selection.config != "" || !selection.stepsMode {
				t.Fatalf("unexpected editor selection: %+v", selection)
			}
			if selection.editorLaunchWithoutDebugger != test.wantNotice {
				t.Fatalf("launch notice=%t, want %t", selection.editorLaunchWithoutDebugger, test.wantNotice)
			}
		})
	}
}

package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestUnspecifiedContainerUsesLibraryContext(t *testing.T) {
	root := t.TempDir()
	options := &runStepsOpts{repoRoot: root, projectRoot: root, wfRoot: root, wf: &domain.Workflow{}, opts: &CliOpts{}}
	_, run, directory, context, _, err := buildStepContainer(options, 0, 1, domain.Step{Cmd: "echo hello"}, map[string]string{"DOCKPIPE_WORKDIR": root}, map[string]string{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if run.Image != "dockpipe-base-dev" || directory != context {
		t.Fatalf("default container = %q, %q, %q", run.Image, directory, context)
	}
	if _, err := os.Stat(filepath.Join(context, "assets", "entrypoint.sh")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "Dockerfile"), []byte("corrupted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, _, err := buildStepContainer(options, 0, 1, domain.Step{Cmd: "echo hello"}, map[string]string{}, map[string]string{}, nil); err == nil {
		t.Fatal("default container cache corruption was ignored")
	}
}

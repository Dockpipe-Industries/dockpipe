package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestPackagedArchiveAdmissionKeepsExactConfigAndMaterializedAssets(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "dockpipe.config.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	for name, contents := range map[string]string{
		"config.yml":  "name: child\nnamespace: archive-fixture\nvault: none\nsteps:\n  - kind: host\n    run: child.sh\n",
		"package.yml": "schema: 1\nkind: workflow\nname: child\nversion: 1.0.0\n",
		"child.sh":    "printf archive-child\n",
	} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	store, err := infrastructure.PackagesWorkflowsDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(store, 0o700); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(store, "dockpipe-workflow-child-1.0.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(source, archive, "workflows/child"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	o := baseRunStepsOpts()
	o.repoRoot, o.projectRoot = root, root
	// Normal invocation from the project cwd has no explicit --workdir export.
	// Nested package lookup must retain the already-admitted project root.
	o.opts.Workdir = ""
	delete(o.envMap, "DOCKPIPE_WORKDIR")
	prepared, err := preparePackageWorkflow(&o, domain.Step{WorkflowName: "child", Package: "archive-fixture"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(prepared.config, "tar://"+archive+"##") {
		t.Fatalf("exact archive config was lost: %q", prepared.config)
	}
	asset, err := os.ReadFile(filepath.Join(prepared.root, "child.sh"))
	if err != nil || string(asset) != "printf archive-child\n" {
		t.Fatalf("compiled child asset unavailable: %q, %v", asset, err)
	}
}

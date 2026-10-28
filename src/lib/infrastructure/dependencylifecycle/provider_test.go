//go:build linux || darwin

package dependencylifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dockpipe/src/lib/domain/dependency"
)

func workflowProviderFixture(t *testing.T, machine, script string) (WorkflowProvider, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", filepath.Join(root, "global"))
	t.Setenv("DOCKPIPE_SYSTEM_ROOT", filepath.Join(root, "system"))
	t.Setenv("DOCKPIPE_PACKAGES_ROOT", "")
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "new-private-state"))
	providerRoot := filepath.Join(root, "authoring", "fixture.provider")
	if err := os.MkdirAll(providerRoot, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(root, "dockpipe.config.json"): `{"schema":1,"compile":{"workflows":["authoring"]}}`,
		filepath.Join(providerRoot, "profile"):      "DOCKPIPE_DEPENDENCY_LIFECYCLE_PROTOCOL=" + dependency.InventorySchema + "\nDOCKPIPE_DEPENDENCY_LIFECYCLE_WORKFLOW=inspect.yml\n",
		filepath.Join(providerRoot, "package.yml"):  "schema: 1\nkind: resolver\nname: fixture.provider\nversion: 1.0.0\ntitle: Fixture provider\n",
		filepath.Join(providerRoot, "inspect.yml"):  "name: inspection-fixture\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	report := dependency.Inventory{Schema: dependency.InventorySchema,
		Provider: dependency.ProviderIdentity{ID: "fixture.provider", Version: "1.0.0"}, Machine: machine, ObservedAt: time.Now().UTC(),
		Observations: []dependency.Observation{}, Gaps: []string{"fixture only"}}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "fixture-cli")
	body := "#!/bin/sh\nset -eu\nfor argument in \"$@\"; do\n case \"$argument\" in\n DOCKPIPE_DEPENDENCY_OUTPUT=*) output=\"${argument#*=}\";;\n esac\ndone\n" + script + "\nprintf '%s' '" + string(data) + "' >\"$output\"\nchmod 600 \"$output\"\n"
	if err := os.WriteFile(executable, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	return WorkflowProvider{Executable: executable}, root
}

func TestDependencyWorkflowProviderBindsReceiptAndCreatesPrivateFirstRun(t *testing.T) {
	provider, root := workflowProviderFixture(t, "fixture-host", "")
	inventory, err := provider.Inspect(context.Background(), root, "fixture.provider", "fixture-host")
	if err != nil || inventory.Provider.ID != "fixture.provider" {
		t.Fatalf("inspection failed: %+v, %v", inventory, err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "new-private-state", "dockpipe", "dependency-lifecycle"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("private invocation receipt was not cleaned up: %v", err)
	}
}

func TestDependencyWorkflowProviderRejectsDifferentTarget(t *testing.T) {
	provider, root := workflowProviderFixture(t, "different-host", "")
	if _, err := provider.Inspect(context.Background(), root, "fixture.provider", "fixture-host"); err == nil {
		t.Fatal("substituted target receipt accepted")
	}
}

func TestDependencyWorkflowProviderCancelsCommandTree(t *testing.T) {
	provider, root := workflowProviderFixture(t, "fixture-host", "sleep 30")
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := provider.Inspect(ctx, root, "fixture.provider", "fixture-host")
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 3*time.Second {
		t.Fatalf("inspection ignored cancellation: %v", err)
	}
}

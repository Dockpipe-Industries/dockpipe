package application

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestEnvironmentRuntimeObservationUsesRecordedOwnerScope(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX executable resolver hook")
	}
	root := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", root)
	workdir := t.TempDir()
	file := filepath.Join(t.TempDir(), "vm.yaml")
	if err := os.WriteFile(file, []byte("schema: dockpipe.environment/v1\nname: vm\nkind: vm\nprovision:\n  workflow: launch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	instance, state, err := environmentInstanceForDefinition(file)
	if err != nil {
		t.Fatal(err)
	}
	const runID = "00112233"
	environ := []string{
		environmentInstanceKey + "=" + instance.ID,
		environmentStateWorkdirKey + "=" + state,
		"DOCKPIPE_WORKDIR=" + workdir,
		"DOCKPIPE_RUN_ID=" + runID,
	}
	if err := recordEnvironmentRuntime([]string{"fixture"}, append(environ, environmentStateWorkdirKey+"="+workdir)); err == nil {
		t.Fatal("accepted a runtime record outside its instance directory")
	}
	if err := recordEnvironmentRuntime([]string{"fixture"}, environ); err != nil {
		t.Fatal(err)
	}

	resolvers, err := infrastructure.GlobalPackagesResolversDir()
	if err != nil {
		t.Fatal(err)
	}
	provider := filepath.Join(resolvers, "fixture")
	if err := os.MkdirAll(provider, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(provider, "profile"), []byte("DOCKPIPE_ENVIRONMENT_INSTANCE_STATUS=observe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hook := `#!/bin/sh
set -eu
[ "$PWD" = "$DOCKPIPE_WORKDIR" ]
[ "$DOCKPIPE_WORKDIR" = '` + workdir + `' ]
[ "$1" = instance-status ]
[ "$2" = '` + instance.ID + `' ]
[ "$3" = --run ]
[ "$4" = '` + runID + `' ]
printf '{"schema":"dockpipe.vm.instance-status/v1","instance_id":"%s","requested_run_id":"%s","run_id":"%s","matched_records":1,"state":"running"}\n' "$2" "$4" "$4"
`
	executable := filepath.Join(provider, "observe")
	if err := os.WriteFile(executable, []byte(hook), 0o700); err != nil {
		t.Fatal(err)
	}

	status := observeEnvironmentVM(context.Background(), instance.ID)
	if status.State != "running" || status.Reason != "" {
		t.Fatalf("unexpected observation: %+v", status)
	}
	entries, err := collectEnvironmentDefinitions([]string{file})
	if err != nil || len(entries) != 1 {
		t.Fatalf("collect environments: %+v, %v", entries, err)
	}
	observeEnvironmentDefinitions(entries)
	if entries[0].Runtime == nil || entries[0].Runtime.State != "running" {
		t.Fatalf("inventory did not carry observed state: %+v", entries[0].Runtime)
	}
}

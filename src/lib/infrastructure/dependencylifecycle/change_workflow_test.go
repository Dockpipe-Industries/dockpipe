//go:build linux || darwin

package dependencylifecycle

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain/dependency"
)

func TestChangeWorkflowNegotiatesVersionAndRejectsInvalidResults(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		provider, root := workflowProviderFixture(t, "fixture-host", "")
		resolverRoot := filepath.Join(root, "authoring", "fixture.provider")
		profile := "DOCKPIPE_DEPENDENCY_CHANGE_PROTOCOL=" + dependency.ChangeSchema + "\nDOCKPIPE_DEPENDENCY_CHANGE_WORKFLOW=inspect.yml\n"
		if err := os.WriteFile(filepath.Join(resolverRoot, "profile"), []byte(profile), 0600); err != nil {
			t.Fatal(err)
		}
		response := `{"schema":"dockpipe.dependency.change/v1","status":"ready","message":"fixture verified"}`
		if invalid {
			response = `{"schema":"wrong","status":"success","message":"untrusted"}`
		}
		script := "#!/bin/sh\nset -eu\nfor argument in \"$@\"; do\n case \"$argument\" in\n DOCKPIPE_DEPENDENCY_OUTPUT=*) output=\"${argument#*=}\";;\n esac\ndone\nprintf '%s' '" + response + "' >\"$output\"\nchmod 600 \"$output\"\n"
		if err := os.WriteFile(provider.Executable, []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		request := dependency.ChangeRequest{Schema: dependency.ChangeSchema, Resolver: "fixture.provider", ProviderVersion: "1.0.0", Machine: "local", Parameters: map[string]string{"target": "/private/tool"}}
		result, err := provider.Change(context.Background(), root, request, "plan", "")
		if invalid && err == nil {
			t.Fatal("invalid provider receipt accepted")
		}
		if !invalid && (err != nil || result.Status != "ready") {
			t.Fatal(result, err)
		}
		request.ProviderVersion = "2.0.0"
		if _, err := provider.Change(context.Background(), root, request, "plan", ""); err == nil {
			t.Fatal("provider version changed under approval")
		}
	}
}

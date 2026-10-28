package dependencylifecycle

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain/dependency"
)

func TestChangeRejectsMismatchedApprovalBeforeProviderExecution(t *testing.T) {
	request := dependency.ChangeRequest{Schema: dependency.ChangeSchema, Resolver: "example.tool", ProviderVersion: "1.0.0", Machine: "local", Parameters: map[string]string{"target": "/tmp/tool"}}
	_, err := (WorkflowProvider{Executable: "/missing/cli"}).Change(context.Background(), t.TempDir(), request, "apply", request.ApprovalID("recover"))
	if err == nil || !strings.Contains(err.Error(), "approval") {
		t.Fatalf("got %v", err)
	}
}

func TestChangeRequestRejectsUnknownTrailingAndLinkedData(t *testing.T) {
	for _, data := range []string{`{"schema":"dockpipe.dependency.change/v1","unknown":true}`, `{} {}`} {
		path := filepath.Join(t.TempDir(), "request.json")
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadChangeRequest(path); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
}

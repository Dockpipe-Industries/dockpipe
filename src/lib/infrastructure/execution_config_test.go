package infrastructure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestExecutionConfigurationUsesExistingVarsRoundTrip(t *testing.T) {
	source := "name: fixture\nvars:\n  DOCKPIPE_OBSERVABILITY: 'off'\nsteps:\n  - id: example\n    kind: host\n    cmd: echo fixture\n"
	result, err := domain.EditWorkflowDocument(domain.WorkflowDocumentRequest{Source: source, Edits: []domain.WorkflowDocumentEdit{
		{Operation: "set", Path: []string{"steps", "0", "id"}, Value: json.RawMessage(`"renamed"`)},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Source, "DOCKPIPE_OBSERVABILITY") || !strings.Contains(result.Source, "off") {
		t.Fatal(result.Source)
	}
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(result.Source), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateResolvedWorkflowYAML(path); err != nil {
		t.Fatal(err)
	}
}

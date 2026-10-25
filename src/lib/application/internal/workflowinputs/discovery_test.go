package workflowinputs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestDiscoverRetainsUsableFieldsAndSourceDiagnostics(t *testing.T) {
	root := t.TempDir()
	model := `public Class Settings
{
    /// <summary>Message sent to the tool.</summary>
    [EnvName = "TOOL_MESSAGE"]
    public string Message = "default";
    public List<string> Tags;
}
`
	if err := os.WriteFile(filepath.Join(root, "Settings.pipe"), []byte(model), 0o600); err != nil {
		t.Fatal(err)
	}
	workflow := &domain.Workflow{
		Types: []string{"invalid<", "Settings.pipe"},
		Vars:  map[string]string{"TOOL_MESSAGE": "workflow-default"},
		Inputs: map[string]domain.InputBinding{
			"Message": {From: "SUPPLIED_MESSAGE"},
		},
	}
	config := filepath.Join(root, "config.yml")
	discovery := Discover(config, root, workflow, nil)
	if len(discovery.Diagnostics) != 1 || !strings.Contains(discovery.Diagnostics[0].Error(), "invalid type spec") {
		t.Fatalf("expected retained source diagnostic, got %v", discovery.Diagnostics)
	}
	if len(discovery.Fields) != 2 {
		t.Fatalf("usable source fields lost: %#v", discovery.Fields)
	}
	message := discovery.Fields[0]
	if message.EnvName != "TOOL_MESSAGE" || message.DefaultValue != "workflow-default" || message.Description != "Message sent to the tool." {
		t.Fatalf("metadata changed: %#v", message)
	}
	if _, exists := discovery.Fields[1].Attributes["control"]; exists {
		t.Fatal("execution metadata acquired an inferred presentation control")
	}
	resolved, err := ResolveWorkflow(workflow, config, root, map[string]string{"SUPPLIED_MESSAGE": "provided"})
	if err != nil {
		t.Fatal(err)
	}
	if resolved["TOOL_MESSAGE"] != "provided" {
		t.Fatalf("partial discovery changed binding semantics: %#v", resolved)
	}
}

func TestResolveWorkflowRetainsMissingTypesError(t *testing.T) {
	root := t.TempDir()
	workflow := &domain.Workflow{
		Types:  []string{"invalid<"},
		Inputs: map[string]domain.InputBinding{"Message": {Value: "hello"}},
	}
	_, err := ResolveWorkflow(workflow, filepath.Join(root, "config.yml"), root, nil)
	if err == nil || err.Error() != "workflow uses inputs but no typed workflow inputs could be resolved from types" {
		t.Fatalf("execution diagnostic contract changed: %v", err)
	}
}

func TestResolveInputBindingPreservesFallbackOrder(t *testing.T) {
	for _, test := range []struct {
		name     string
		binding  domain.InputBinding
		env      map[string]string
		fallback string
		want     string
	}{
		{name: "environment", binding: domain.InputBinding{From: "INPUT", Value: "literal"}, env: map[string]string{"INPUT": " value "}, fallback: "default", want: " value "},
		{name: "empty environment", binding: domain.InputBinding{From: "INPUT", Value: "literal"}, env: map[string]string{"INPUT": " "}, fallback: "default", want: "literal"},
		{name: "missing environment", binding: domain.InputBinding{From: "INPUT"}, fallback: "default", want: "default"},
		{name: "literal", binding: domain.InputBinding{Value: " literal "}, fallback: "default", want: " literal "},
		{name: "empty literal", binding: domain.InputBinding{Value: " "}, fallback: "default", want: "default"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveInputBinding(test.binding, test.env, test.fallback); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

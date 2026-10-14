package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWorkflowDocumentPreservesSourceAndComments(t *testing.T) {
	source := "# user heading\nname: example\nsteps:\n  - id: first # user note\n    kind: host\n    cmd: echo one\n    agent:\n      startup_prompt: Keep this unmodeled field\n  - id: second\n    kind: host\n    cmd: echo two\n"
	result, err := EditWorkflowDocument(WorkflowDocumentRequest{Source: source})
	if err != nil || result.Source != source {
		t.Fatalf("inspection changed source: %v", err)
	}
	result, err = EditWorkflowDocument(WorkflowDocumentRequest{Source: source, Edits: []WorkflowDocumentEdit{
		{Operation: "set", Path: []string{"steps", "0", "id"}, Value: json.RawMessage(`"renamed"`)},
		{Operation: "move", Path: []string{"steps", "0"}, Destination: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# user heading", "# user note", "Keep this unmodeled field", "renamed"} {
		if !strings.Contains(result.Source, want) {
			t.Errorf("lost %q in %s", want, result.Source)
		}
	}
	if strings.Index(result.Source, "second") > strings.Index(result.Source, "renamed") {
		t.Fatal("move failed")
	}
}

func TestWorkflowDocumentEditsParallelAndEscapedValues(t *testing.T) {
	source := "name: sample\nsteps: []\n"
	result, err := EditWorkflowDocument(WorkflowDocumentRequest{Source: source, Edits: []WorkflowDocumentEdit{
		{Operation: "append", Path: []string{"steps"}, Value: json.RawMessage(`{"group":{"mode":"async","tasks":[{"kind":"host","cmd":"echo \"a: b\"\nprintf '%s' '$HOME'","vars":{"FLAG":"true","COUNT":"012"}}]}}`)},
		{Operation: "set", Path: []string{"steps", "0", "group", "tasks", "0", "cwd"}, Value: json.RawMessage(`"repo"`)},
	}})
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := ParseWorkflowYAML([]byte(result.Source))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateLoadedWorkflow(workflow); err != nil {
		t.Fatal(err)
	}
	reread, err := EditWorkflowDocument(WorkflowDocumentRequest{Source: result.Source})
	if err != nil {
		t.Fatal(err)
	}
	document := reread.Document.(map[string]interface{})
	task := document["steps"].([]interface{})[0].(map[string]interface{})["group"].(map[string]interface{})["tasks"].([]interface{})[0].(map[string]interface{})
	if task["vars"].(map[string]interface{})["FLAG"] != "true" {
		t.Fatal("string coerced to boolean")
	}
}

func TestWorkflowDocumentRejectsUnsafeOrInvalidEdits(t *testing.T) {
	for _, source := range []string{
		"name: first\nname: second\n", "name: x\n---\nname: y\n", "steps: &s [*s]\n", "[]", strings.Repeat(" ", WorkflowDocumentMaxBytes+1),
		"steps: " + strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66),
		"steps: [" + strings.Repeat("0,", workflowDocumentMaxNodes) + "0]",
	} {
		if _, err := EditWorkflowDocument(WorkflowDocumentRequest{Source: source}); err == nil {
			t.Fatal("accepted invalid document")
		}
	}
	for _, edit := range []WorkflowDocumentEdit{
		{Operation: "set", Path: []string{"steps", "-1"}, Value: json.RawMessage(`{}`)},
		{Operation: "move", Path: []string{"name"}, Destination: 0},
		{Operation: "append", Path: []string{"name"}, Value: json.RawMessage(`"bad"`)},
		{Operation: "set", Path: []string{"missing", "field"}, Value: json.RawMessage(`true`)},
		{Operation: "set", Path: []string{"name"}, Value: json.RawMessage(`invalid`)},
		{Operation: "unknown", Path: []string{"name"}},
	} {
		if _, err := EditWorkflowDocument(WorkflowDocumentRequest{Source: "name: x\nsteps: []\n", Edits: []WorkflowDocumentEdit{edit}}); err == nil {
			t.Fatalf("accepted %#v", edit)
		}
	}
}

func TestWorkflowDocumentRemoveAndCapacityBoundary(t *testing.T) {
	source := "name: x\nsteps: [{id: a}, {id: b}]\n"
	result, err := EditWorkflowDocument(WorkflowDocumentRequest{Source: source, Edits: []WorkflowDocumentEdit{
		{Operation: "remove", Path: []string{"steps", "0"}},
		{Operation: "remove", Path: []string{"name"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Source, "id: a") || strings.Contains(result.Source, "name:") {
		t.Fatal(result.Source)
	}
	source = "name: x\n#" + strings.Repeat("a", WorkflowDocumentMaxBytes-len("name: x\n#"))
	if _, err := EditWorkflowDocument(WorkflowDocumentRequest{Source: source}); err != nil {
		t.Fatal(err)
	}
}

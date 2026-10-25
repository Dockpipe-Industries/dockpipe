package domain

import (
	"encoding/json"
	"testing"
)

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

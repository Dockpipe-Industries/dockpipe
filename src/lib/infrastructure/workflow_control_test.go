package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dockpipe/src/lib/domain/workflow"
)

func controlTestWorkflow(t *testing.T) *workflow.Workflow {
	t.Helper()
	value, err := workflow.ParseWorkflowYAML([]byte("name: control-test\ncontrol:\n  schema: 1\n  kind: host\n  source: |\n    try steps.test.Run();\nsteps:\n  - id: test\n    kind: host\n    cmd: echo test\n"))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestWorkflowControlSourceHygiene(t *testing.T) {
	definition := controlTestWorkflow(t)
	definition.Steps[0].ID = "host"
	definition.Steps[0].Control = &workflow.StepControl{Parameters: []workflow.ControlParameter{
		{Name: "host", Type: "string", Env: "VALUE_A"},
		{Name: "controlArguments", Type: "string", Env: "VALUE_B"},
		{Name: "encodedArgument2", Type: "bool", Env: "VALUE_C"},
	}}
	source, _, _ := controlSource(definition)
	for _, expected := range []string{"this.host = new WorkflowStep0(host)", "this.host.Run(\"host\", controlArguments_)", "string encodedArgument2_ ="} {
		if !strings.Contains(source, expected) {
			t.Fatalf("missing hygienic binding %q", expected)
		}
	}
}

func TestWorkflowControlWireRejectsMalformedAndBoundedInputs(t *testing.T) {
	for _, payload := range [][]byte{{}, {9}, {3, 3}, {3, 0, 0, 0, 0, 0, 1}} {
		if _, err := decodeControlMessage(payload); err == nil {
			t.Fatalf("accepted malformed frame %v", payload)
		}
	}
	writer := &controlWireWriter{}
	var output bytes.Buffer
	if err := writer.send(&output); err == nil || output.Len() != 0 {
		t.Fatal("empty frame was not rejected before writing")
	}

	writer.byte(1)
	writer.number(1, 8)
	writer.text("test")
	writer.number(2, 4)
	for index := 0; index < 2; index++ {
		writer.text("same")
		writer.text("value")
	}
	if _, err := decodeControlMessage(writer.Bytes()); err == nil {
		t.Fatal("duplicate argument accepted")
	}
	control := controlTestWorkflow(t).Control
	control.Inputs = map[string]workflow.ControlInput{"flag": {Type: "bool", Env: "FLAG"}}
	if _, err := encodeControlInputs(control, map[string]string{"FLAG": "1"}); err == nil {
		t.Fatal("noncanonical boolean accepted")
	}
	if _, err := encodeControlInputs(control, map[string]string{}); err == nil {
		t.Fatal("missing binding accepted")
	}
	control.Inputs = map[string]workflow.ControlInput{"values": {Type: "List<string>", Value: []string{strings.Repeat("x", 65536)}, HasValue: true}}
	if _, err := encodeControlInputs(control, nil); err != nil {
		t.Fatal(err)
	}
	values := make([]string, 17)
	for index := range values {
		values[index] = strings.Repeat("x", 65536)
	}
	control.Inputs["values"] = workflow.ControlInput{Type: "List<string>", Value: values, HasValue: true}
	if _, err := encodeControlInputs(control, nil); err == nil {
		t.Fatal("aggregate input frame limit ignored")
	}
	if _, err := readControlFrame(bytes.NewReader([]byte{255, 255, 255, 255})); err == nil {
		t.Fatal("oversized frame accepted")
	}
}

func publishControlTestArtifact(t *testing.T, workdir string, definition *workflow.Workflow, program []byte) string {
	t.Helper()
	identity, err := controlDefinitionIdentity(definition)
	if err != nil {
		t.Fatal(err)
	}
	root, err := controlCacheDirectory(workdir, identity)
	if err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(root, "artifact-test")
	if err := os.MkdirAll(artifact, 0o700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(artifact, controlBinaryName())
	if err := os.WriteFile(binary, program, 0o700); err != nil {
		t.Fatal(err)
	}
	hash, size, err := controlFileDigest(binary, 256*1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	source, _, line := controlSource(definition)
	manifest := workflowControlManifest{Schema: 1, Profile: workflowControlProfile, DefinitionSHA256: identity, SDKManifestSHA256: strings.Repeat("a", 64), SourceSHA256: controlTestHash([]byte(source)), BinarySHA256: hash, BinarySize: size, OS: runtime.GOOS, Architecture: runtime.GOARCH, BodyLine: line}
	encoded, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(artifact, "manifest.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "selected.json"), []byte(`{"artifact":"artifact-test"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return binary
}

func controlTestHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func TestWorkflowControlArtifactRefusesStaleAndTampered(t *testing.T) {
	definition := controlTestWorkflow(t)
	root := t.TempDir()
	binary := publishControlTestArtifact(t, root, definition, []byte("test executable"))
	if _, _, err := loadControlArtifact(root, definition); err != nil {
		t.Fatal(err)
	}
	definition.Steps[0].Cmd = "different command"
	if _, _, err := loadControlArtifact(root, definition); err == nil {
		t.Fatal("stale workflow accepted")
	}
	definition.Steps[0].Cmd = "echo test"
	if err := os.WriteFile(binary, []byte("tampered bytes"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadControlArtifact(root, definition); err == nil {
		t.Fatal("tampered executable accepted")
	}
}

func TestWorkflowControlArgumentsRequireDeclaredCanonicalSet(t *testing.T) {
	definition := controlTestWorkflow(t)
	definition.Steps[0].Control = &workflow.StepControl{Parameters: []workflow.ControlParameter{{Name: "count", Type: "int", Env: "COUNT"}}}
	for _, arguments := range []map[string]string{{}, {"extra": "1"}, {"count": "01"}, {"count": "1", "extra": "2"}} {
		if _, err := controlStepArguments(definition, "test", arguments); err == nil {
			t.Fatalf("invalid arguments accepted %v", arguments)
		}
	}
	result, err := controlStepArguments(definition, "test", map[string]string{"count": "-2"})
	if err != nil || result["COUNT"] != "-2" {
		t.Fatalf("canonical argument rejected: %v", err)
	}
	if _, err := controlStepArguments(definition, "unknown", nil); err == nil {
		t.Fatal("undeclared step accepted")
	}
}

func TestWorkflowControlExportRelocatesWithoutProjectCache(t *testing.T) {
	original := t.TempDir()
	definition := controlTestWorkflow(t)
	publishControlTestArtifact(t, original, definition, []byte("native fixture bytes"))
	bundle := t.TempDir()
	if err := ExportWorkflowControlArtifact(original, definition, bundle); err != nil {
		t.Fatal(err)
	}
	otherProject := t.TempDir()
	path, _, err := loadControlArtifact(otherProject, definition, bundle)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, bundle+string(os.PathSeparator)) {
		t.Fatal("did not select the relocated bundle")
	}
	if err := os.WriteFile(path, []byte("tampered"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadControlArtifact(otherProject, definition, bundle); err == nil {
		t.Fatal("tampered bundle accepted")
	}
}

func TestWorkflowControlSchemaCompilesAndValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	content := []byte("name: test\ncontrol:\n  schema: 1\n  kind: host\n  inputs:\n    flag: {type: bool, value: true}\n  source: |\n    try steps.test.Run();\nsteps:\n  - id: test\n    kind: host\n    cmd: echo test\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateWorkflowYAML(path); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowControlExportRejectsDifferentStagedBuild(t *testing.T) {
	project := t.TempDir()
	definition := controlTestWorkflow(t)
	publishControlTestArtifact(t, project, definition, []byte("first producer"))
	stage := t.TempDir()
	if err := ExportWorkflowControlArtifact(project, definition, stage); err != nil {
		t.Fatal(err)
	}
	publishControlTestArtifact(t, project, definition, []byte("second producer"))
	if err := ExportWorkflowControlArtifact(project, definition, stage); err == nil {
		t.Fatal("old staged producer silently accepted")
	}
}

package workflowdocumentcmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestWorkflowDocumentBridgeDoesNotExecuteOrAcceptTrailingInput(t *testing.T) {
	var output bytes.Buffer
	request := `{"source":"name: sample\nsteps: [{kind: host, cmd: never-execute-this}]\n"}`
	if err := workflowDocumentBridge(strings.NewReader(request), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "never-execute-this") {
		t.Fatal(output.String())
	}
	output.Reset()
	if err := workflowDocumentBridge(strings.NewReader(request+`{}`), &output); err == nil || output.Len() != 0 {
		t.Fatal("accepted trailing request")
	}
	if err := workflowDocumentBridge(strings.NewReader(strings.Repeat(" ", 6*1024*1024+1)), &output); err == nil {
		t.Fatal("accepted oversized transport")
	}
}

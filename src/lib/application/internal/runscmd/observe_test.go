package runscmd

import (
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestObserveJSONAndUnknownArguments(t *testing.T) {
	previous := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	os.Stdout = writer
	defer func() { os.Stdout = previous }()
	err = Run([]string{"observe", "--workdir", t.TempDir(), "--json"})
	writer.Close()
	os.Stdout = previous
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["schema"] != "dockpipe.execution_snapshot.v1" {
		t.Fatal(snapshot)
	}
	if err := Run([]string{"observe", "--pid", "1"}); err == nil {
		t.Fatal("unexpected option accepted")
	}
	if err := Run([]string{"observe", "--workdir"}); err == nil {
		t.Fatal("missing path accepted")
	}
}

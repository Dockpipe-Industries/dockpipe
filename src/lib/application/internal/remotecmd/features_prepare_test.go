package remotecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFeaturePrepareEmitsCanonicalMetadataBeforeAnyRemoteAccess(t *testing.T) {
	request := fileRecoveryRequest(t)
	expectedID := request.ID
	request.ID = ""
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	input := filepath.Join(root, "intent.json")
	if err := os.WriteFile(input, data, 0o600); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(root, "uninitialized-remote-state")
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	previous := os.Stdout
	os.Stdout = writer
	runErr := Run([]string{"feature-prepare", "--request", input, "--state", state, "--auth", "never"}, nil)
	os.Stdout = previous
	writer.Close()
	if runErr != nil {
		t.Fatal("local preparation required operator state or network access", runErr)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := decodeFeatureReceipt(output)
	if err != nil || metadata.ID != expectedID || metadata.Target != request.Target || metadata.Root != request.Root || metadata.Action != request.Action || !metadata.Deadline.Equal(request.Deadline) {
		t.Fatal("preparation changed the canonical reference", err)
	}
	for _, forbidden := range []string{"payload", "nonce", "private-file", request.Nonce} {
		if strings.Contains(string(output), forbidden) {
			t.Fatal("prepared reference retained replayable input", forbidden)
		}
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("local preparation created remote state", err)
	}
	var repeated bytes.Buffer
	if err := prepareFeature(context.Background(), input, &repeated); err != nil || !bytes.Equal(output, repeated.Bytes()) {
		t.Fatal("repeated preparation changed a fixed intent", err)
	}
}

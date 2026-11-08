package workspacefilescmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure/workspacefiles"
)

func TestFilesPublicAdapterUsesBoundedNativeService(t *testing.T) {
	root := t.TempDir()
	content := []byte("source\r\n")
	if err := os.WriteFile(filepath.Join(root, "document.txt"), content, 0600); err != nil {
		t.Fatal(err)
	}
	envelope := Envelope{Root: root, Request: workspacefiles.Request{
		Schema: workspacefiles.Schema, Action: "read", Path: "document.txt"}}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := handle(context.Background(), bytes.NewReader(data), &output); err != nil {
		t.Fatal(err)
	}
	var result workspacefiles.Result
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Schema != workspacefiles.Schema || !bytes.Equal(result.Content, content) || len(result.Revision) != 64 {
		t.Fatalf("invalid native file result: %+v", result)
	}
}

func TestFilesPublicAdapterRejectsAmbiguousAndOversizedInput(t *testing.T) {
	for name, data := range map[string]string{
		"unknown-fields":         `{"root":"/unused","identity":"forged","request":{}}`,
		"unknown-request-fields": `{"root":"/unused","request":{"schema":"dockpipe.files/v1","action":"read","path":"file","principal":"forged"}}`,
		"multiple-objects":       "{} {}",
		"non-utf8":               string([]byte{'{', 0xff, '}'}),
		"oversized":              strings.Repeat(" ", maxRequestBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			if err := handle(context.Background(), strings.NewReader(data), &output); err == nil || output.Len() != 0 {
				t.Fatalf("invalid file request produced output: %v %q", err, output.String())
			}
		})
	}
}

func TestFilesPublicAdapterPreservesCancellation(t *testing.T) {
	data, err := json.Marshal(Envelope{Root: t.TempDir(), Request: workspacefiles.Request{
		Schema: workspacefiles.Schema, Action: "read", Path: "absent"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := handle(ctx, bytes.NewReader(data), &bytes.Buffer{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation changed into another outcome: %v", err)
	}
}

package orchestrationhelper

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestUsageCommandsRejectInvalidLedger(t *testing.T) {
	for _, content := range []string{"", `{"total_estimated_tokens":12345`, `null`, `{}`, `{"total_estimated_tokens":null}`, `{"total_estimated_tokens":"12"}`, `{"total_estimated_tokens":-1}`, `{"total_estimated_tokens":1.5}`, `{"total_estimated_tokens":9223372036854775808}`} {
		t.Run(content, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "usage.json")
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := Run([]string{"usage-number", path, "total_estimated_tokens"}, nil, &output, io.Discard); err == nil || output.Len() != 0 {
				t.Fatalf("invalid accounting returned success or output: %v %q", err, output.String())
			}
		})
	}
	for _, path := range []string{filepath.Join(t.TempDir(), "missing"), t.TempDir()} {
		var output bytes.Buffer
		if err := Run([]string{"usage-number", path, "total_estimated_tokens"}, nil, &output, io.Discard); err == nil || output.Len() != 0 {
			t.Fatalf("read failure returned usage: %v", err)
		}
	}
}

func TestUsageCommandsPropagateReadAndOutputFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	if err := Run([]string{"usage-number", path, "count"}, nil, io.Discard, io.Discard); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read error lost its cause: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"count":1,"providers":{"codex":{"count":1}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	reader, writer := io.Pipe()
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	for _, args := range [][]string{
		{"usage-number", path, "count"},
		{"provider-usage-number", path, "codex", "count"},
	} {
		if err := Run(args, nil, writer, io.Discard); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("output failure became success: %v", err)
		}
	}
}

func TestUsageCommandsPreserveIntegerCounters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	if err := os.WriteFile(path, []byte(`{"total_estimated_tokens":9007199254740993,"providers":{"codex":{"task_count":3}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"usage-number", path, "total_estimated_tokens"}, "9007199254740993\n"},
		{[]string{"provider-usage-number", path, "codex", "task_count"}, "3\n"},
	} {
		var output bytes.Buffer
		if err := Run(tc.args, nil, &output, io.Discard); err != nil || output.String() != tc.want {
			t.Fatalf("usage = %q, %v", output.String(), err)
		}
	}
	for _, provider := range []string{"missing", "codex"} {
		var output bytes.Buffer
		if err := Run([]string{"provider-usage-number", path, provider, "missing"}, nil, &output, io.Discard); err == nil || output.Len() != 0 {
			t.Fatal("missing provider counter accepted")
		}
	}
}

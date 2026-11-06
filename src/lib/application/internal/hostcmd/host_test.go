package hostcmd

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/infrastructure/nativehost"
)

func TestParseHostBoundary(t *testing.T) {
	for _, args := range [][]string{
		{"status"},
		{"worker", "--socket", "/tmp/x"},
		{"status", "--connection", "/tmp/x", "--nonce", "secret"},
		{"snapshot", "--connection", "/tmp/x", "--cwd", "/tmp"},
		{"worker", "--socket", "/tmp/x", "--nonce", "x", "--detach", "--ready-fd", "3"},
		{"docker", "exec", "--connection", "/tmp/x", "--id", "abcdef123456"},
		{"docker", "start", "--connection", "/tmp/x"},
	} {
		if _, err := parse(args); err == nil {
			t.Fatalf("accepted invalid arguments: %v", args)
		}
	}
	if _, err := parse([]string{"snapshot", "--connection", "/tmp/x", "--json", "--docker"}); err != nil {
		t.Fatal(err)
	}
	if _, err := parse([]string{"docker", "stop", "--connection", "/tmp/x", "--id", "abcdef123456", "--json"}); err != nil {
		t.Fatal(err)
	}
}

func TestParseDockerQuery(t *testing.T) {
	for _, kind := range []string{"container", "network", "volume"} {
		if _, err := parse([]string{"docker-query", "--connection", "/tmp/ref", "--kind", kind, "--id", "abcdef123456", "--json"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"docker-query", "--connection", "/tmp/ref", "--id", "abcdef123456"},
		{"docker-query", "--connection", "/tmp/ref", "--kind", "exec", "--id", "abcdef123456"},
		{"docker-query", "--connection", "/tmp/ref", "--kind", "volume", "--id", "../data"},
		{"docker-query", "--connection", "/tmp/ref", "--kind", "volume", "--kind", "container", "--id", "abcdef123456"},
	} {
		if _, err := parse(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

type failingStatusWriter struct {
	err error
}

func (w failingStatusWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestUnavailableResponsesPreserveOutputFailure(t *testing.T) {
	writeErr := errors.New("output unavailable")
	for _, command := range []string{"status", "docker", "docker-query"} {
		t.Run(command, func(t *testing.T) {
			options := arguments{command: command, connection: filepath.Join(t.TempDir(), "missing.json")}
			err := run(context.Background(), options, failingStatusWriter{err: writeErr})
			if !errors.Is(err, writeErr) || !errors.Is(err, nativehost.ErrUnavailable) {
				t.Fatalf("run error = %v, want output and native-host failures", err)
			}
		})
	}
}

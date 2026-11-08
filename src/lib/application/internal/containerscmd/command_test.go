package containerscmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/telemetry"
)

const containerID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func dockerFixture(t *testing.T, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	directory := t.TempDir()
	path := filepath.Join(directory, "docker")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_DOCKER_BIN", path)
	return directory
}

func TestInvalidRequestsCannotInvokeDocker(t *testing.T) {
	directory := dockerFixture(t, `printf invoked > "$CONTAINER_TEST_MARKER"`)
	marker := filepath.Join(directory, "invoked")
	t.Setenv("CONTAINER_TEST_MARKER", marker)
	for _, input := range []string{
		`null`, `[]`, `{}`, `{"action":"exec","container":"` + containerID + `"}`,
		`{"action":"start","container":"short"}`,
		`{"action":"start","container":"--help"}`,
		`{"action":"start","container":"` + containerID + `","args":["--privileged"]}`,
		`{"action":"snapshot","container":"` + containerID + `"}`,
		`{"action":"snapshot","action":"start"}`,
		`{"action":"snapshot","container":null}`,
		`{"action":"snapshot"} {}`, strings.Repeat(" ", maxRequestBytes+1),
		string([]byte{'{', '"', 0xff, '"', ':', '1', '}'}),
	} {
		var output bytes.Buffer
		if err := handle(context.Background(), strings.NewReader(input), &output); err == nil || output.Len() != 0 {
			t.Errorf("invalid request accepted: %q (%v)", input, err)
		}
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid request started Docker: %v", err)
	}
}

func TestControlUsesExactArgumentsAndStructuredReceipt(t *testing.T) {
	directory := dockerFixture(t, `printf '%s\n' "$@" > "$CONTAINER_TEST_ARGUMENTS"`)
	arguments := filepath.Join(directory, "arguments")
	t.Setenv("CONTAINER_TEST_ARGUMENTS", arguments)
	for _, action := range []string{"start", "stop", "restart"} {
		var output bytes.Buffer
		input := `{"action":"` + action + `","container":"` + containerID + `"}`
		if err := handle(context.Background(), strings.NewReader(input), &output); err != nil {
			t.Fatal(err)
		}
		actual, err := os.ReadFile(arguments)
		if err != nil || string(actual) != "container\n"+action+"\n"+containerID+"\n" {
			t.Fatalf("unexpected Docker arguments %q: %v", actual, err)
		}
		var receipt infrastructure.ContainerControlResult
		if err := json.Unmarshal(output.Bytes(), &receipt); err != nil || receipt.Schema != "dockpipe.containers/v1" || receipt.Action != action || receipt.Container != containerID {
			t.Fatalf("invalid control receipt: %s (%v)", output.Bytes(), err)
		}
	}
}

func TestSnapshotUsesBoundedInventory(t *testing.T) {
	dockerFixture(t, `case "$1:$2" in
container:ls) printf '%s\n' '{"ID":"`+containerID+`","Names":"web"}' ;;
network:ls|volume:ls|container:inspect) ;;
*) exit 1 ;;
esac
`)
	var output bytes.Buffer
	if err := handle(context.Background(), strings.NewReader(`{"action":"snapshot"}`), &output); err != nil {
		t.Fatal(err)
	}
	var snapshot telemetry.DockerSnapshot
	if err := json.Unmarshal(output.Bytes(), &snapshot); err != nil || !snapshot.Available || len(snapshot.Containers) != 1 || snapshot.Containers[0].ID != containerID {
		t.Fatalf("invalid inventory: %s (%v)", output.Bytes(), err)
	}
}

func TestControlCancellationAndFailureDoNotProduceReceipt(t *testing.T) {
	for _, script := range []string{"exit 7\n", "exec /bin/sleep 30\n"} {
		t.Run(script, func(t *testing.T) {
			dockerFixture(t, script)
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			started := time.Now()
			var output bytes.Buffer
			err := handle(ctx, strings.NewReader(`{"action":"start","container":"`+containerID+`"}`), &output)
			if err == nil || output.Len() != 0 || time.Since(started) > 2*time.Second {
				t.Fatalf("failed operation was not bounded: %v, %s", err, output.Bytes())
			}
		})
	}
}

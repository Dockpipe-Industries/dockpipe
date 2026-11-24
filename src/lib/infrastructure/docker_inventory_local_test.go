package infrastructure

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLocalDockerInventoryCannotFollowRemoteContext(t *testing.T) {
	t.Setenv("DOCKER_HOST", "tcp://untrusted.invalid:2375")
	t.Setenv("DOCKER_CONTEXT", "remote")
	t.Setenv("DOCKER_CONFIG", "/untrusted/config")
	t.Setenv("DOCKER_TLS_VERIFY", "1")
	t.Setenv("DOCKER_CERT_PATH", "/untrusted/certificates")
	command := dockerInventoryProcess(context.Background(), true, "container", "ls")
	if len(command.Args) < 3 || command.Args[1] != "--host=unix:///var/run/docker.sock" || command.Args[2] != "--tls=false" {
		t.Fatalf("local daemon not pinned: %v", command.Args)
	}
	for _, entry := range command.Env {
		if strings.HasPrefix(entry, "DOCKER_") {
			t.Fatal("Docker endpoint environment reached local inventory")
		}
	}
	ordinary := dockerInventoryProcess(context.Background(), false, "container", "ls")
	if ordinary.Args[1] != "container" {
		t.Fatal("ordinary inventory behavior changed")
	}
	if !strings.Contains(strings.Join(ordinary.Env, "\n"), "DOCKER_CONTEXT=remote") {
		t.Fatal("ordinary user context no longer preserved")
	}
	if runtime.GOOS == "windows" {
		return
	}
	directory := t.TempDir()
	path := filepath.Join(directory, "docker")
	script := `#!/bin/sh
[ "$1" = '--host=unix:///var/run/docker.sock' ] || exit 1
[ "$2" = '--tls=false' ] || exit 1
[ -z "$DOCKER_HOST$DOCKER_CONTEXT$DOCKER_CONFIG$DOCKER_TLS_VERIFY$DOCKER_CERT_PATH" ] || exit 1
shift 2
case "$1:$2" in
container:ls|network:ls|volume:ls) exit 0 ;;
*) exit 1 ;;
esac
`
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_DOCKER_BIN", path)
	snapshot, err := ReadLocalMachineSnapshot(context.Background(), false, true)
	if err != nil || snapshot.Docker == nil || !snapshot.Docker.Available {
		t.Fatalf("local worker snapshot failed: %+v %v", snapshot, err)
	}
}

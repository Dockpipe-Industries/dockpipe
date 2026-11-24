package infrastructure

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dockpipe/src/lib/domain/invocation"
)

func TestContainerRecoveryRequiresExplicitRetirementAndAuthoritativeAbsence(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake provider fixture")
	}
	root := t.TempDir()
	provider := filepath.Join(root, "docker")
	script := `#!/bin/sh
if [ "$1" = info ]; then printf 'fixture-daemon\n'; exit 0; fi
if [ "$1 $2" = 'container rm' ]; then printf '%s\n' "$*" > "$FIXTURE_REMOVE"; exit 0; fi
if [ "$1 $2" = 'container ls' ]; then
  if [ ! -f "$FIXTURE_REMOVE" ]; then printf '%s\n' "$FIXTURE_CONTAINER"; fi
  exit 0
fi
exit 2
`
	if err := os.WriteFile(provider, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	removed := filepath.Join(root, "removed")
	container := strings.Repeat("a", 64)
	t.Setenv("DOCKPIPE_DOCKER_BIN", provider)
	t.Setenv("FIXTURE_REMOVE", removed)
	t.Setenv("FIXTURE_CONTAINER", container)
	receipt := ContainerLifecycleReceipt{Container: container, Provider: ContainerProviderBinding(provider, os.Environ()), Daemon: ContainerDaemonDigest("fixture-daemon")}
	if err := RecoverContainerLifecycle(context.Background(), receipt, false); err == nil {
		t.Fatal("current provider readback was treated as original completion")
	}
	if _, err := os.Stat(removed); !os.IsNotExist(err) {
		t.Fatal("verification implicitly removed the container")
	}
	target := invocation.Target{Kind: invocation.Remote, ID: "worker", Fingerprint: strings.Repeat("b", 64), Generation: strings.Repeat("c", 64)}
	identity := invocation.Identity{Issuer: "fixture", Subject: "operator"}
	if err := RecoverContainerLifecycleWithAuthority(context.Background(), receipt, true, target, identity, nil); err == nil {
		t.Fatal("remote retirement executed without owning authorization")
	}
	if _, err := os.Stat(removed); !os.IsNotExist(err) {
		t.Fatal("missing authorization failed after a native mutation")
	}
	if err := RecoverContainerLifecycle(context.Background(), receipt, true); err != nil {
		t.Fatal("explicit retirement did not confirm exact resource absence", err)
	}
	arguments, err := os.ReadFile(removed)
	if err != nil || strings.TrimSpace(string(arguments)) != "container rm --force -- "+container {
		t.Fatal("retirement selected extra resources or removed volumes/images", err)
	}
	if err := os.Remove(removed); err != nil {
		t.Fatal(err)
	}
	receipt.Completed = true
	if err := RecoverContainerLifecycle(context.Background(), receipt, false); err != nil {
		t.Fatal("durable daemon completion acknowledgment required deletion", err)
	}
	if _, err := os.Stat(removed); !os.IsNotExist(err) {
		t.Fatal("confirmed completion still removed a container")
	}
	t.Setenv("DOCKER_HOST", "unix:///changed-provider")
	if err := RecoverContainerLifecycle(context.Background(), receipt, true); err == nil {
		t.Fatal("recovery followed a changed Docker provider selection")
	}
}

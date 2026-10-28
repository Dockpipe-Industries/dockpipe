package remotecmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestServiceDefinitionReplacementRequiresExplicitIntent(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("remote user services require Linux or macOS directory durability semantics")
	}
	root := t.TempDir()
	definition := serviceDefinition{path: filepath.Join(root, "remote.service"), content: "new service\n"}
	old := []byte("custom service\n")
	if err := os.WriteFile(definition.path, old, 0o600); err != nil {
		t.Fatal(err)
	}
	if state := inspectServiceDefinition(definition).State; state != "update-required" {
		t.Fatalf("stale registration: %s", state)
	}
	if err := publishServiceDefinition(context.Background(), definition, false); err == nil {
		t.Fatal("custom service replaced without explicit intent")
	}
	got, _ := os.ReadFile(definition.path)
	if string(got) != string(old) {
		t.Fatal("refusal changed existing service")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := publishServiceDefinition(cancelled, definition, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled replacement: %v", err)
	}
	got, _ = os.ReadFile(definition.path)
	if string(got) != string(old) {
		t.Fatal("cancellation changed existing service")
	}
	if err := publishServiceDefinition(context.Background(), definition, true); err != nil {
		t.Fatal(err)
	}
	if state := inspectServiceDefinition(definition).State; state != "current" {
		t.Fatalf("replacement registration: %s", state)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatal("temporary service files leaked")
	}
}

func TestServiceDefinitionRejectsLinksAndOversizedFiles(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("preserve me"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "remote.service")
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	definition := serviceDefinition{path: link, content: "replacement"}
	if err := publishServiceDefinition(context.Background(), definition, true); err == nil {
		t.Fatal("linked definition accepted")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "preserve me" {
		t.Fatal("symlink target modified")
	}
	definition.path = filepath.Join(root, "large.service")
	if err := os.WriteFile(definition.path, make([]byte, maxServiceDefinitionBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := publishServiceDefinition(context.Background(), definition, true); err == nil {
		t.Fatal("oversized definition accepted")
	}
}

func TestExecutableFingerprintDetectsUpgradeAtSamePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dockpipe")
	if err := os.WriteFile(path, []byte("old executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	old, err := executableFingerprint(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("new executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	fresh, err := executableFingerprint(path)
	if err != nil || old == fresh {
		t.Fatalf("same-path upgrade not detected: %v", err)
	}
}

func TestServiceUpdateRestartsAndRetainsFailureForRetry(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd fixture")
	}
	root := privateConnectionDirectory(t)
	tools := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", tools)
	t.Setenv("DOCKPIPE_SERVICE_TEST_ROOT", root)
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$DOCKPIPE_SERVICE_TEST_ROOT/calls"
if [ "$2" = restart ] && [ -f "$DOCKPIPE_SERVICE_TEST_ROOT/fail" ]; then
    exit 1
fi
`
	if err := os.WriteFile(filepath.Join(tools, "systemctl"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	credential := []byte("fixture credential must remain unchanged")
	if err := os.WriteFile(filepath.Join(root, "worker.json"), credential, 0o600); err != nil {
		t.Fatal(err)
	}
	definition, err := prepareService(context.Background(), root, "worker")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(definition.path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(definition.path, []byte("old service"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := installService(context.Background(), root, "worker"); err == nil {
		t.Fatal("implicit service replacement accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "calls")); !os.IsNotExist(err) {
		t.Fatal("refused replacement contacted service manager")
	}
	if err := os.WriteFile(filepath.Join(root, "fail"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := installServiceWithReplacement(context.Background(), root, "worker", true); err == nil {
		t.Fatal("restart failure was swallowed")
	}
	if state := describeService(root, "worker").State; state != "update-required" {
		t.Fatalf("failed activation presented as complete: %s", state)
	}
	if err := installService(context.Background(), root, "worker"); err == nil {
		t.Fatal("incomplete replacement retried without restart intent")
	}
	if err := os.Remove(filepath.Join(root, "fail")); err != nil {
		t.Fatal(err)
	}
	if err := installServiceWithReplacement(context.Background(), root, "worker", true); err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(filepath.Join(root, "calls"))
	if strings.Count(string(calls), "--user restart ") != 2 {
		t.Fatalf("replacement did not restart on both attempts: %s", calls)
	}
	if state := describeService(root, "worker").State; state != "current" {
		t.Fatalf("completed update: %s", state)
	}
	got, _ := os.ReadFile(filepath.Join(root, "worker.json"))
	if string(got) != string(credential) {
		t.Fatal("service update changed worker identity")
	}
}

func TestServiceReplaceFlagCannotChangeOtherOperations(t *testing.T) {
	if err := Run([]string{"pairing-open", "--replace"}, nil); err == nil {
		t.Fatal("service replacement flag accepted for pairing")
	}
}

func TestLaunchAgentUpdateReloadsAndPropagatesFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	root := t.TempDir()
	t.Setenv("PATH", root)
	t.Setenv("DOCKPIPE_SERVICE_TEST_ROOT", root)
	script := `#!/bin/sh
printf '%s\n' "$1" >> "$DOCKPIPE_SERVICE_TEST_ROOT/calls"
if [ "$1" = bootout ] && [ -f "$DOCKPIPE_SERVICE_TEST_ROOT/fail" ]; then
    exit 1
fi
`
	if err := os.WriteFile(filepath.Join(root, "launchctl"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	definition := serviceDefinition{name: "test-service", path: filepath.Join(root, "service.plist")}
	if err := activateServiceForPlatform(context.Background(), definition, true, "darwin"); err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(filepath.Join(root, "calls"))
	if string(calls) != "print\nbootout\nbootstrap\n" {
		t.Fatalf("loaded agent was not replaced: %s", calls)
	}
	if err := os.WriteFile(filepath.Join(root, "calls"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "fail"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := activateServiceForPlatform(context.Background(), definition, true, "darwin"); err == nil {
		t.Fatal("agent unload failure was swallowed")
	}
	calls, _ = os.ReadFile(filepath.Join(root, "calls"))
	if string(calls) != "print\nbootout\n" {
		t.Fatalf("bootstrap continued after unload failure: %s", calls)
	}
}

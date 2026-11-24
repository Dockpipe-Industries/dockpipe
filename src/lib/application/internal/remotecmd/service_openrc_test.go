package remotecmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestOpenRCWorkerAvailableRequiresSupportedUserMode(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("OpenRC user worker backend is Linux-only")
	}
	tools := t.TempDir()
	t.Setenv("PATH", tools)
	for _, name := range []string{"rc-update", "openrc-run", "supervise-daemon"} {
		writeOpenRCFixtureTool(t, tools, name, "exit 0\n")
	}
	writeOpenRCFixtureTool(t, tools, "rc-service", `
test "$#" = 1 && test "$1" = --version || exit 1
printf 'rc-service (OpenRC%s) %s\n' "${OPENRC_TEST_SYSTEM_TYPE:+ [$OPENRC_TEST_SYSTEM_TYPE]}" "$OPENRC_TEST_VERSION"
`)
	runtimeDir := t.TempDir()
	if err := os.Chmod(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	ctx := context.Background()
	base := t.TempDir()
	t.Setenv("OPENRC_TEST_VERSION", "0.63.2")
	if openRCWorkerAvailable(ctx, false, base) {
		t.Fatal("uninitialized OpenRC user session was accepted")
	}
	if err := os.MkdirAll(filepath.Join(base, "rc", "runlevels", "default"), 0o700); err != nil {
		t.Fatal(err)
	}
	if openRCWorkerAvailable(ctx, false, base) {
		t.Fatal("configured runlevel without user-session state was accepted")
	}
	controller := filepath.Join(runtimeDir, "openrc")
	if err := os.Mkdir(controller, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(controller, "softlevel"), []byte("default\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !openRCWorkerAvailable(ctx, false, base) {
		t.Fatal("supported OpenRC user worker was rejected")
	}
	t.Setenv("OPENRC_TEST_SYSTEM_TYPE", "lxc")
	if !openRCWorkerAvailable(ctx, false, base) {
		t.Fatal("supported OpenRC user worker with a system type was rejected")
	}
	if openRCWorkerAvailable(ctx, true, base) {
		t.Fatal("OpenRC was selected from Flatpak")
	}
	t.Setenv("OPENRC_TEST_VERSION", "0.61.9")
	if openRCWorkerAvailable(ctx, false, base) {
		t.Fatal("experimental OpenRC user mode was accepted")
	}
	t.Setenv("OPENRC_TEST_VERSION", "0.62")
	if !openRCWorkerAvailable(ctx, false, base) {
		t.Fatal("first stable OpenRC user mode was rejected")
	}
	t.Setenv("XDG_RUNTIME_DIR", "relative-runtime")
	if openRCWorkerAvailable(ctx, false, base) {
		t.Fatal("non-absolute XDG runtime was accepted")
	}
}

func TestOpenRCWorkerScriptPreservesLiteralArguments(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("POSIX service script fixture")
	}
	tools := t.TempDir()
	t.Setenv("PATH", tools)
	writeOpenRCFixtureTool(t, tools, "openrc-run", "exit 0\n")
	writeOpenRCFixtureTool(t, tools, "supervise-daemon", `printf '%s\000' "$@"`+"\n")
	base := t.TempDir()
	name := "com.dockpipe.remote.worker.fixture"
	worker := filepath.Join(base, `Dockpipe's $worker [literal]`)
	state := filepath.Join(base, `Worker's state $(printf unexpected)*`)
	arguments := []string{worker, "remote", "worker", "--state", state}
	path, script := openRCWorkerDefinition(base, name, arguments, strings.Repeat("f", 64))
	if path != filepath.Join(base, "rc", "init.d", name) || !strings.HasPrefix(script, "#!"+filepath.Join(tools, "openrc-run")+"\n") {
		t.Fatalf("worker script resolved to an unexpected path or interpreter: %s", path)
	}
	log := filepath.Join(state, "worker.log")
	want := append([]string{name, "--start", "--respawn-delay", "3", "--respawn-max", "5", "--respawn-period", "60",
		"--umask", "077", "--env", "PATH=" + tools, "--stdout", log, "--stderr", log, "--"}, arguments...)
	for _, operation := range []struct {
		name string
		want []string
	}{
		{name: "start", want: want},
		{name: "stop", want: []string{name, "--stop"}},
	} {
		cmd := exec.Command("/bin/sh")
		cmd.Stdin = strings.NewReader(script + `
ebegin() { :; }
eend() { return "$1"; }
RC_SVCNAME=com.dockpipe.remote.worker.fixture
` + operation.name + "\n")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s failed: %v\n%s", operation.name, err, output)
		}
		if got := strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00"); !reflect.DeepEqual(got, operation.want) {
			t.Fatalf("%s arguments = %q, want %q", operation.name, got, operation.want)
		}
	}
}

func TestOpenRCWorkerActivationScopesRunlevelAndReportsFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("OpenRC activation with POSIX shell fixtures")
	}
	tools := t.TempDir()
	t.Setenv("PATH", tools)
	t.Setenv("XDG_CONFIG_HOME", "")
	base := t.TempDir()
	name := "com.dockpipe.remote.worker.fixture"
	path := filepath.Join(base, "rc", "init.d", name)
	log := filepath.Join(base, "activation-calls")
	t.Setenv("OPENRC_TEST_CALLS", log)
	t.Setenv("OPENRC_TEST_CONFIG_HOME", base)
	t.Setenv("OPENRC_TEST_DEFINITION", path)
	writeOpenRCFixtureTool(t, tools, "rc-service", `
test "$XDG_CONFIG_HOME" = "$OPENRC_TEST_CONFIG_HOME" || exit 10
printf 'rc-service %s\n' "$*" >> "$OPENRC_TEST_CALLS"
if test "$2" = --resolve; then
    printf '%s\n' "$OPENRC_TEST_DEFINITION"
fi
if test "$4" = restart && test "$OPENRC_TEST_FAIL_RESTART" = yes; then
    exit 11
fi
`)
	writeOpenRCFixtureTool(t, tools, "rc-update", `
test "$XDG_CONFIG_HOME" = "$OPENRC_TEST_CONFIG_HOME" || exit 12
printf 'rc-update %s\n' "$*" >> "$OPENRC_TEST_CALLS"
`)
	definition := serviceDefinition{name: name, path: path}
	ctx := context.Background()
	if err := activateOpenRCWorker(ctx, definition, false); err == nil || !strings.Contains(err.Error(), "default runlevel is unavailable") {
		t.Fatalf("expected uninitialized user runlevel to fail before running a manager: %v", err)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatalf("manager called before user runlevel was initialized: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(base, "rc", "runlevels", "default"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := activateOpenRCWorker(ctx, definition, false); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENRC_TEST_FAIL_RESTART", "yes")
	if err := activateOpenRCWorker(ctx, definition, true); err == nil || !strings.Contains(err.Error(), "restart OpenRC user worker") {
		t.Fatalf("expected restart error to propagate: %v", err)
	}
	t.Setenv("OPENRC_TEST_DEFINITION", filepath.Join(base, "other-service"))
	if err := activateOpenRCWorker(ctx, definition, true); err == nil || !strings.Contains(err.Error(), "different user worker definition") {
		t.Fatalf("a different definition was accepted: %v", err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	resolve := "rc-service --user --resolve " + name + "\n"
	register := "rc-update --user add " + name + " default\n"
	wantCalls := resolve + register + "rc-service --user --nodeps " + name + " start\n" +
		resolve + register + "rc-service --user --nodeps " + name + " restart\n" + resolve
	if string(data) != wantCalls {
		t.Fatalf("manager calls:\n%s\nwant:\n%s", data, wantCalls)
	}
}

func writeOpenRCFixtureTool(t *testing.T, directory, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte("#!/bin/sh\n"+contents), 0o700); err != nil {
		t.Fatal(err)
	}
}

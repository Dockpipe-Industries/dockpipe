//go:build linux

package process

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestDelegatedScopeChild(t *testing.T) {
	role := os.Getenv("DOCKPIPE_CGROUP_TEST_ROLE")
	if role == "" {
		return
	}
	root := os.Getenv("DOCKPIPE_CGROUP_TEST_ROOT")
	if role == "escaped-group" {
		os.WriteFile(filepath.Join(root, "ready"), nil, 0600)
		time.Sleep(600 * time.Millisecond)
		os.WriteFile(filepath.Join(root, "survived"), nil, 0600)
		os.Exit(0)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestDelegatedScopeChild$")
	child.Env = append(os.Environ(), "DOCKPIPE_CGROUP_TEST_ROLE=escaped-group")
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := child.Start(); err != nil {
		os.Exit(2)
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "ready")); err == nil {
			os.Exit(0)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestDelegatedScopeCleansSetsidDescendantAfterParentExit(t *testing.T) {
	base := os.Getenv("DOCKPIPE_TEST_CGROUP_ROOT")
	if base == "" {
		t.Skip("requires an explicitly delegated task-owned cgroup v2 fixture")
	}
	t.Setenv("DOCKPIPE_PROCESS_RECOVERY", "cgroup-v2")
	t.Setenv("DOCKPIPE_PROCESS_SCOPE_ROOT", base)
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx, scope := WithScope(ctx, strings.Repeat("c", 64), func(ScopeReference) error { return nil })
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDelegatedScopeChild$")
	command.Env = append(os.Environ(), "DOCKPIPE_CGROUP_TEST_ROLE=parent", "DOCKPIPE_CGROUP_TEST_ROOT="+root)
	if err := RunWithScope(ctx, command); err != nil {
		t.Fatal(err)
	}
	if err := scope.Stop(ctx); err != nil {
		t.Fatal("kernel scope did not prove descendant cleanup", err)
	}
	time.Sleep(750 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(root, "survived")); !os.IsNotExist(err) {
		t.Fatal("a hook-like setsid descendant escaped the operation scope", err)
	}
}

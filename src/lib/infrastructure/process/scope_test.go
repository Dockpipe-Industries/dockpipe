package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestScopedProcessChild(t *testing.T) {
	root := os.Getenv("DOCKPIPE_SCOPE_TEST_ROOT")
	if root == "" {
		return
	}
	if _, err := os.Stat(filepath.Join(root, "persisted")); err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(filepath.Join(root, "executed"), nil, 0600); err != nil {
		os.Exit(3)
	}
	os.Exit(0)
}

func TestBootFencedScopePersistsIdentityBeforeEffectsAndRequiresChangedBoot(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("native boot identity unavailable")
	}
	t.Setenv("DOCKPIPE_PROCESS_RECOVERY", "host-reboot")
	root := t.TempDir()
	var captured ScopeReference
	ctx, scope := WithScope(context.Background(), strings.Repeat("a", 64), func(reference ScopeReference) error {
		captured = reference
		return os.WriteFile(filepath.Join(root, "persisted"), nil, 0600)
	})
	// Later process-environment changes cannot replace the selected worker policy.
	t.Setenv("DOCKPIPE_PROCESS_RECOVERY", "unavailable")
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestScopedProcessChild$")
	command.Env = append(os.Environ(), "DOCKPIPE_SCOPE_TEST_ROOT="+root)
	if err := RunWithScope(ctx, command); err != nil {
		t.Fatal(err)
	}
	if captured.Mode != "host-reboot" || captured.Validate() != nil {
		t.Fatal("lost the exact pre-effect kernel identity")
	}
	if _, err := os.Stat(filepath.Join(root, "executed")); err != nil {
		t.Fatal("process did not execute after receipt publication", err)
	}
	if err := scope.Stop(context.Background()); !errors.Is(err, ErrRebootRequired) {
		t.Fatal("unchanged worker boot was treated as descendant cleanup proof", err)
	}
	priorBoot := captured
	priorBoot.Boot = "00000000-0000-0000-0000-000000000000"
	if err := StopScope(context.Background(), priorBoot); err != nil {
		t.Fatal("a valid prior kernel lifetime did not qualify boot-fenced recovery", err)
	}
}

func TestScopedProcessCannotStartBeforeReceiptOrWithUnavailableDelegation(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("native scoped execution unavailable")
	}
	root := t.TempDir()
	for _, scenario := range []string{"receipt-failed", "delegation-unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("DOCKPIPE_PROCESS_RECOVERY", "host-reboot")
			if scenario == "delegation-unavailable" {
				t.Setenv("DOCKPIPE_PROCESS_RECOVERY", "cgroup-v2")
				t.Setenv("DOCKPIPE_PROCESS_SCOPE_ROOT", root)
			}
			ctx, _ := WithScope(context.Background(), strings.Repeat("b", 64), func(ScopeReference) error {
				return errors.New("fixture receipt publication failed")
			})
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestScopedProcessChild$")
			command.Env = append(os.Environ(), "DOCKPIPE_SCOPE_TEST_ROOT="+root)
			if err := RunWithScope(ctx, command); err == nil {
				t.Fatal("process started without durable ownership")
			}
			if command.Process != nil {
				t.Fatal("failure occurred after the native process had already started")
			}
		})
	}
}

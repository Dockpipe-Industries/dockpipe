//go:build windows

package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Real test processes supply inherited writers or redirected background work.
// Their finite fallback lifetimes are never accepted as owner cancellation.
func TestHostContextOutputDescendantWindows(t *testing.T) {
	mode := os.Getenv("DOCKPIPE_HOST_CONTEXT_CHILD")
	if mode == "writer" || mode == "finite-writer" {
		if ready := os.Getenv("DOCKPIPE_HOST_CONTEXT_READY"); ready != "" {
			if err := os.WriteFile(ready, []byte("ready"), 0600); err != nil {
				os.Exit(83)
			}
		}
		if mode == "finite-writer" {
			time.Sleep(150 * time.Millisecond)
			if err := os.WriteFile(os.Getenv("DOCKPIPE_HOST_CONTEXT_FINISHED"), []byte("finished"), 0600); err != nil {
				os.Exit(84)
			}
			os.Exit(0)
		}
		time.Sleep(10 * time.Second)
		os.Exit(81)
	}
	if mode != "root" && mode != "redirected-root" {
		return
	}

	childMode := "writer"
	if os.Getenv("DOCKPIPE_HOST_CONTEXT_FINISHED") != "" {
		childMode = "finite-writer"
	}
	child := exec.Command(os.Args[0], "-test.run=^TestHostContextOutputDescendantWindows$")
	child.Env = append(os.Environ(), "DOCKPIPE_HOST_CONTEXT_CHILD="+childMode)
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if mode == "redirected-root" {
		null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			os.Exit(85)
		}
		defer null.Close()
		child.Stdout = null
		child.Stderr = null
	}
	if err := child.Start(); err != nil {
		os.Exit(82)
	}

	if mode == "redirected-root" {
		handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(child.Process.Pid))
		if err != nil {
			os.Exit(86)
		}
		var created, exited, kernel, user windows.Filetime
		err = windows.GetProcessTimes(handle, &created, &exited, &kernel, &user)
		windows.CloseHandle(handle)
		if err != nil {
			os.Exit(87)
		}
		identity := fmt.Sprintf("%d:%d:%d", child.Process.Pid, created.HighDateTime, created.LowDateTime)
		if err := os.WriteFile(os.Getenv("DOCKPIPE_HOST_CONTEXT_IDENTITY"), []byte(identity), 0600); err != nil {
			os.Exit(88)
		}
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(os.Getenv("DOCKPIPE_HOST_CONTEXT_ACK")); err == nil {
				code, err := strconv.Atoi(os.Getenv("DOCKPIPE_HOST_CONTEXT_EXIT"))
				if err != nil {
					os.Exit(89)
				}
				os.Exit(code)
			}
			time.Sleep(5 * time.Millisecond)
		}
		os.Exit(90)
	}
	os.Exit(0)
}

func hostContextWindowsEnvironment(mode string) []string {
	return append(os.Environ(),
		"DOCKPIPE_HOST_CONTEXT_CHILD="+mode,
		"DOCKPIPE_HOST_SCRIPT_SPINNER=0",
		"DOCKPIPE_OBSERVABILITY=off",
		"DOCKPIPE_SKIP_HOST_CLEANUP=1",
	)
}

func TestHostContextWindowsNormalExitWaitsForGenuineEOF(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	finished := filepath.Join(t.TempDir(), "finished")
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHostContextOutputDescendantWindows$")
	env := append(hostContextWindowsEnvironment("root"), "DOCKPIPE_HOST_CONTEXT_FINISHED="+finished)
	if err := runContextHostScriptProcess(ctx, "finite-output-descendant", command, env); err != nil {
		t.Fatalf("normal output completion failed: %v", err)
	}
	value, err := os.ReadFile(finished)
	if err != nil || string(value) != "finished" || ctx.Err() != nil {
		t.Fatalf("root completion cut off the actual inherited writer: %q, %v, %v", value, err, ctx.Err())
	}
}

func TestHostContextWindowsCancellationRemainsActiveAfterRootExit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHostContextOutputDescendantWindows$")
	result := make(chan error, 1)
	go func() {
		result <- runContextHostScriptProcess(ctx, "live-output-descendant", command, hostContextWindowsEnvironment("root"))
	}()
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("post-root output cancellation was lost: %v", err)
		}
		if command.ProcessState == nil || command.ProcessState.ExitCode() != 0 {
			t.Fatalf("deadline occurred before the actual root exited normally: %v", command.ProcessState)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("owner stopped supervising while a descendant retained output")
	}
}

func TestHostContextWindowsActiveCancellationPreservesOutcome(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ready := filepath.Join(t.TempDir(), "ready")
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHostContextOutputDescendantWindows$")
	env := append(hostContextWindowsEnvironment("writer"), "DOCKPIPE_HOST_CONTEXT_READY="+ready)
	result := make(chan error, 1)
	go func() { result <- runContextHostScriptProcess(ctx, "active-cancellation", command, env) }()
	waitHostContextWindowsFile(t, ctx, ready, result)
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("active cancellation outcome was lost: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Windows active process owner did not join cancellation")
	}
}

func waitHostContextWindowsFile(t *testing.T, ctx context.Context, path string, result <-chan error) []byte {
	t.Helper()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if value, err := os.ReadFile(path); err == nil && len(value) != 0 {
			return value
		}
		select {
		case err := <-result:
			t.Fatalf("host process returned before actual readiness: %v", err)
		case <-ctx.Done():
			t.Fatal("Windows host child did not become ready")
		case <-ticker.C:
		}
	}
}

func TestHostContextWindowsRedirectedChildSurvivesNormalCompletion(t *testing.T) {
	for _, exitCode := range []int{0, 17} {
		t.Run(strconv.Itoa(exitCode), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			directory := t.TempDir()
			identityPath := filepath.Join(directory, "identity")
			ackPath := filepath.Join(directory, "ack")
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHostContextOutputDescendantWindows$")
			env := append(hostContextWindowsEnvironment("redirected-root"),
				"DOCKPIPE_HOST_CONTEXT_IDENTITY="+identityPath,
				"DOCKPIPE_HOST_CONTEXT_ACK="+ackPath,
				"DOCKPIPE_HOST_CONTEXT_EXIT="+strconv.Itoa(exitCode),
			)
			result := make(chan error, 1)
			go func() { result <- runContextHostScriptProcess(ctx, "redirected-descendant", command, env) }()
			identity := waitHostContextWindowsFile(t, ctx, identityPath, result)
			parts := strings.Split(string(identity), ":")
			if len(parts) != 3 {
				t.Fatalf("invalid actual child identity: %q", identity)
			}
			values := make([]uint64, 3)
			for index, part := range parts {
				value, err := strconv.ParseUint(part, 10, 32)
				if err != nil {
					t.Fatal(err)
				}
				values[index] = value
			}
			handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(values[0]))
			if err != nil {
				t.Fatal(err)
			}
			defer windows.CloseHandle(handle)
			var created, exited, kernel, user windows.Filetime
			if err := windows.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
				t.Fatal(err)
			}
			if uint64(created.HighDateTime) != values[1] || uint64(created.LowDateTime) != values[2] {
				t.Fatal("opened child identity differs from the actual creation witness")
			}
			// Cleanup uses this already validated retained capability, never a
			// PID reopened after normal host completion/release.
			defer func() {
				if err := windows.TerminateProcess(handle, 1); err != nil {
					t.Errorf("retained child cleanup: %v", err)
				}
				if state, err := windows.WaitForSingleObject(handle, 3000); err != nil || state != windows.WAIT_OBJECT_0 {
					t.Errorf("retained child cleanup did not complete: %d, %v", state, err)
				}
			}()
			if state, err := windows.WaitForSingleObject(handle, 0); err != nil || state != uint32(windows.WAIT_TIMEOUT) {
				t.Fatalf("actual redirected child was not live before release: %d, %v", state, err)
			}
			if err := os.WriteFile(ackPath, []byte("retained"), 0600); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-result:
				if exitCode == 0 {
					if err != nil {
						t.Fatalf("normal root exit failed: %v", err)
					}
				} else {
					var exit *exec.ExitError
					if !errors.As(err, &exit) || exit.ExitCode() != exitCode {
						t.Fatalf("nonzero root outcome changed: %v", err)
					}
				}
			case <-ctx.Done():
				t.Fatal("redirected host completion required cancellation")
			}
			if state, err := windows.WaitForSingleObject(handle, 0); err != nil || state != uint32(windows.WAIT_TIMEOUT) {
				t.Fatalf("host normal completion killed its redirected child: %d, %v", state, err)
			}
		})
	}
}

package process

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Run the test executable itself, so the process-tree checks also run natively
// on Windows without depending on Bash, sleep, or an installed provider CLI.
func TestProcessHelper(t *testing.T) {
	role := os.Getenv("DOCKPIPE_PROCESS_TEST_ROLE")
	if role == "" {
		return
	}
	root := os.Getenv("DOCKPIPE_PROCESS_TEST_ROOT")
	if role == "child" {
		if err := os.WriteFile(filepath.Join(root, "ready"), nil, 0600); err != nil {
			os.Exit(2)
		}
		time.Sleep(600 * time.Millisecond)
		if err := os.WriteFile(filepath.Join(root, "survived"), nil, 0600); err != nil {
			os.Exit(3)
		}
		os.Exit(0)
	}
	if role == "nested" {
		child := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestProcessHelper$")
		child.Env = append(os.Environ(), "DOCKPIPE_PROCESS_TEST_ROLE=wait")
		if err := RunNested(child); err != nil {
			os.Exit(6)
		}
		os.Exit(0)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestProcessHelper$")
	child.Env = append(os.Environ(), "DOCKPIPE_PROCESS_TEST_ROLE=child")
	if err := child.Start(); err != nil {
		os.Exit(4)
	}
	if role == "exit" {
		for {
			if _, err := os.Stat(filepath.Join(root, "ready")); err == nil {
				os.Exit(0)
			}
			time.Sleep(time.Millisecond)
		}
	}
	if err := child.Wait(); err != nil {
		os.Exit(5)
	}
	os.Exit(0)
}

func TestRunCleansDescendantsOnCancelAndExit(t *testing.T) {
	for _, role := range []string{"wait", "exit", "nested"} {
		t.Run(role, func(t *testing.T) {
			root := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProcessHelper$")
			command.Env = append(os.Environ(), "DOCKPIPE_PROCESS_TEST_ROLE="+role, "DOCKPIPE_PROCESS_TEST_ROOT="+root, "GORACE=atexit_sleep_ms=0")
			done := make(chan error, 1)
			var completed bool
			var result error
			go func() { done <- Run(command) }()
			for {
				if _, err := os.Stat(filepath.Join(root, "ready")); err == nil {
					break
				}
				select {
				case result = <-done:
					completed = true
					if _, err := os.Stat(filepath.Join(root, "ready")); err != nil {
						t.Fatalf("command exited before child started: %v", result)
					}
				case <-ctx.Done():
					t.Fatal("child did not start")
				case <-time.After(time.Millisecond):
				}
			}
			if role != "exit" {
				cancel()
			}
			if !completed {
				result = <-done
			}
			if role != "exit" && result == nil {
				t.Fatal("cancelled process reported success")
			}
			if role == "exit" && result != nil {
				t.Fatal(result)
			}
			time.Sleep(750 * time.Millisecond)
			if _, err := os.Stat(filepath.Join(root, "survived")); !os.IsNotExist(err) {
				t.Fatalf("descendant survived command lifetime: %v", err)
			}
		})
	}
}

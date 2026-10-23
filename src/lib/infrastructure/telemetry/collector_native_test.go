//go:build windows || darwin

package telemetry

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestNativeCollectorChildFixture(t *testing.T) {
	if os.Getenv("DOCKPIPE_TELEMETRY_CHILD_FIXTURE") != "1" {
		return
	}
	fmt.Println("ready")
	bufio.NewReader(os.Stdin).ReadByte()
}

func TestNativeCollectorKernelIdentityAndLifecycle(t *testing.T) {
	collector := NativeCollector{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	first, err := collector.Inspect(ctx, os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	second, err := collector.Inspect(ctx, os.Getpid())
	if err != nil || !SameProcess(first.ProcessIdentity, second.ProcessIdentity) {
		t.Fatalf("current process identity changed: %v", err)
	}
	if first.ParentPID != os.Getppid() || first.Name == "" || first.StartToken == "" || first.MemoryBytes != nil {
		t.Fatal("native process metadata contract failed")
	}
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNativeCollectorChildFixture$")
	child.Env = append(os.Environ(), "DOCKPIPE_TELEMETRY_CHILD_FIXTURE=1")
	input, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		input.Close()
		if child.ProcessState == nil {
			child.Process.Kill()
			child.Wait()
		}
	}()
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "ready" {
		t.Fatal("native fixture did not become ready")
	}
	process, err := collector.Inspect(ctx, child.Process.Pid)
	if err != nil || process.ParentPID != os.Getpid() || process.StartToken == first.StartToken {
		t.Fatalf("child process admission failed: %v", err)
	}
	input.Close()
	if err := child.Wait(); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.Inspect(ctx, child.Process.Pid); err == nil {
		t.Fatal("exited native process admitted")
	}
}

func TestNativeCollectorBoundedDiscoveryAndCancellation(t *testing.T) {
	collector := NativeCollector{}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	processes, truncated, err := collector.Discover(ctx)
	if err != nil {
		// Protected system processes may make the inventory partial, but the
		// collector must still return admitted records and report that failure.
		t.Logf("native discovery reported partial inventory: %v", err)
	}
	if len(processes) > MaxProcesses {
		t.Fatal("process inventory exceeded its record budget")
	}
	found := false
	for _, process := range processes {
		if process.PID <= 0 || process.StartToken == "" {
			t.Fatal("unverified native process record admitted")
		}
		found = found || process.PID == os.Getpid()
	}
	if !found && !truncated {
		t.Fatal("native discovery omitted its own readable process")
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if _, err := collector.Inspect(cancelled, os.Getpid()); err != context.Canceled {
		t.Fatal("inspection cancellation lost")
	}
	if _, _, err := collector.Discover(cancelled); err != context.Canceled {
		t.Fatal("discovery cancellation lost")
	}
}

//go:build linux

// Package containedexec runs generated-code tests only inside a verified,
// temporary systemd cgroup. It is test infrastructure, never a runtime dependency.
package containedexec

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// CombinedOutput fails closed before starting cmd unless its entire process tree
// inherits the resource budget and an independent systemd service deadline.
// On cancellation it kills the service, including children that changed sessions.
func CombinedOutput(cmd *exec.Cmd) ([]byte, error) {
	output, _, err := Measure(cmd)
	return output, err
}

// Measurement separates wall time and waited-child peak RSS. Direct compiler
// invocations use the latter for the warm compiler-only regression ceiling.
type Measurement struct {
	Elapsed   time.Duration
	MaxRSSKiB int64
}

func Measure(cmd *exec.Cmd) ([]byte, Measurement, error) {
	start := time.Now()

	cgroup, unit, err := containment()
	if err != nil {
		return nil, Measurement{}, err
	}
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "GOMAXPROCS=4")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		return nil, Measurement{}, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-done:
			return output.Bytes(), Measurement{time.Since(start), cmd.ProcessState.SysUsage().(*syscall.Rusage).Maxrss}, err
		case <-deadline.C:
			return nil, Measurement{}, killUnit(unit, "generated compilation exceeded 30s")
		case <-tick.C:
			current, err := number(filepath.Join(cgroup, "memory.current"))
			if err != nil || current >= 800*1024*1024 {
				return nil, Measurement{}, killUnit(unit, "generated compilation reached proactive memory stop or lost accounting")
			}
		}
	}
}

func number(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
}

func containment() (string, string, error) {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", "", err
	}
	entry := strings.TrimSpace(string(data))
	if !strings.HasPrefix(entry, "0::/") || strings.Contains(entry, "\n") {
		return "", "", fmt.Errorf("generated compilation requires cgroup v2")
	}
	group := filepath.Join("/sys/fs/cgroup", strings.TrimPrefix(entry, "0::"))
	unit := filepath.Base(group)
	for name, want := range map[string]int64{"memory.max": 1 << 30, "memory.swap.max": 0, "pids.max": 128} {
		got, err := number(filepath.Join(group, name))
		if err != nil || got != want {
			return "", "", fmt.Errorf("generated compilation refused: %s must be %d (got %d, %v); use tests/containedexec/run.py", name, want, got, err)
		}
	}
	if !strings.HasSuffix(unit, ".service") {
		return "", "", fmt.Errorf("generated compilation requires a temporary systemd service deadline")
	}
	check := exec.Command("systemctl", "--user", "show", unit, "--property=RuntimeMaxUSec", "--property=KillMode")
	var out bytes.Buffer
	check.Stdout = &out
	check.Stderr = &out
	check.WaitDelay = time.Second
	// systemctl is only a bounded read, not a compiler child.
	if err := check.Start(); err != nil {
		return "", "", err
	}
	timer := time.AfterFunc(2*time.Second, func() { _ = check.Process.Kill() })
	err = check.Wait()
	timer.Stop()
	properties := out.String()
	if err != nil || !strings.Contains(properties, "KillMode=control-group\n") || !strings.Contains(properties, "RuntimeMaxUSec=") || strings.Contains(properties, "RuntimeMaxUSec=infinity") || strings.Contains(properties, "RuntimeMaxUSec=0\n") {
		return "", "", fmt.Errorf("generated compilation refused: missing independent unit deadline/whole-tree cleanup: %s (%v)", properties, err)
	}
	return group, unit, nil
}

func killUnit(unit, reason string) error {
	if group, _, err := containment(); err == nil {
		for _, name := range []string{"memory.current", "memory.peak", "memory.events", "memory.swap.current", "memory.swap.events"} {
			data, err := os.ReadFile(filepath.Join(group, name))
			fmt.Fprintf(os.Stderr, "containment stop %s: %s (%v)\n", name, data, err)
		}
	}
	fmt.Fprintln(os.Stderr, reason+"; terminating entire containment unit")
	err := exec.Command("systemctl", "--user", "kill", "--kill-who=all", "--signal=KILL", unit).Run()
	// The normal path kills this process too. If the manager fails, do not continue
	// tests: the independently verified RuntimeMaxSec remains the final tree guard.
	return fmt.Errorf("%s: whole-unit kill returned: %v", reason, err)
}

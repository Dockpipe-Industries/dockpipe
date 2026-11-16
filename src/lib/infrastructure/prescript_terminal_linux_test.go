//go:build linux

package infrastructure

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func openHostSignalPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	descriptor, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	master := os.NewFile(uintptr(descriptor), "host-signal-master")
	t.Cleanup(func() { _ = master.Close() })
	number, err := unix.IoctlGetInt(descriptor, unix.TIOCGPTN)
	if err == nil {
		err = unix.IoctlSetPointerInt(descriptor, unix.TIOCSPTLCK, 0)
	}
	if err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = slave.Close() })
	return master, slave
}

func waitHostSignalPTY(t *testing.T, master *os.File, output *bytes.Buffer, expected string) {
	t.Helper()
	if err := master.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for !strings.Contains(output.String(), expected) {
		var chunk [1024]byte
		count, err := master.Read(chunk[:])
		if count > 0 {
			if output.Len()+count > 16*1024 {
				t.Fatal("host PTY witness exceeded its output bound")
			}
			output.Write(chunk[:count])
		}
		if err != nil && !strings.Contains(output.String(), expected) {
			t.Fatalf("host PTY missing %q: %v; %q", expected, err, output.String())
		}
	}
}

func TestHostSignalTerminalOwnerLinux(t *testing.T) {
	if os.Getenv("DOCKPIPE_SIGNAL_TEST_TERMINAL") == "" {
		return
	}
	original, err := unix.IoctlGetTermios(0, unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	foreground, err := unix.IoctlGetInt(0, unix.TIOCGPGRP)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("bash", "-c", `printf '__HOST_READY__\n'
IFS= read -r line
printf '__HOST_INPUT__:%s\n' "$line"
`)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	setRunHostProcAttrs(command)
	if command.SysProcAttr.Setpgid || command.SysProcAttr.Foreground || command.SysProcAttr.Pdeathsig != syscall.SIGTERM {
		t.Fatal("terminal-attached host launch changed its process topology")
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	group, err := syscall.Getpgid(command.Process.Pid)
	if err != nil || group != syscall.Getpgrp() {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("terminal host did not inherit its owner's group: %d, %v", group, err)
	}
	if err := waitHostScriptWithSignalForward(command); err != nil {
		t.Fatal(err)
	}
	currentGroup, err := unix.IoctlGetInt(0, unix.TIOCGPGRP)
	if err != nil || currentGroup != foreground {
		t.Fatalf("host terminal group was not restored: %d, %v", currentGroup, err)
	}
	current, err := unix.IoctlGetTermios(0, unix.TCGETS)
	if err != nil || current.Lflag != original.Lflag {
		t.Fatalf("host terminal mode was not restored: %v", err)
	}
	fmt.Println("__OWNER_READY__")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil || line != "owner input\n" {
		t.Fatalf("owner terminal input was not restored: %q, %v", line, err)
	}
	fmt.Println("__OWNER_INPUT_RESTORED__")
}

// This small parent shell makes the owner job non-orphaned, so the terminal's
// Ctrl-Z has real job-control semantics. It observes stop without reaping and
// resumes only its private, still-owned job group.
func TestHostSignalTerminalControllerLinux(t *testing.T) {
	if os.Getenv("DOCKPIPE_SIGNAL_TEST_CONTROLLER") == "" {
		return
	}
	owner := exec.Command(os.Args[0], "-test.run=^TestHostSignalTerminalOwnerLinux$")
	owner.Env = append(os.Environ(), "DOCKPIPE_SIGNAL_TEST_TERMINAL=1")
	owner.Stdin, owner.Stdout, owner.Stderr = os.Stdin, os.Stdout, os.Stderr
	owner.SysProcAttr = &syscall.SysProcAttr{Foreground: true, Ctty: 0}
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	joined := false
	defer func() {
		if !joined {
			_ = unix.Kill(-owner.Process.Pid, unix.SIGKILL)
			_ = owner.Wait()
		}
	}()
	stopped := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var information unix.Siginfo
		err := unix.Waitid(unix.P_PID, owner.Process.Pid, &information, unix.WSTOPPED|unix.WNOWAIT|unix.WNOHANG, nil)
		if err != nil && !errors.Is(err, unix.EINTR) {
			t.Fatal(err)
		}
		if information.Signo != 0 {
			stopped = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !stopped {
		t.Fatal("terminal Ctrl-Z did not stop the owning job")
	}
	fmt.Println("__JOB_STOPPED__")
	if err := unix.Kill(-owner.Process.Pid, unix.SIGCONT); err != nil {
		t.Fatal(err)
	}
	fmt.Println("__JOB_RESUMED__")
	signals := make(chan os.Signal, 1)
	forced := make(chan struct{}, 1)
	timer := time.AfterFunc(4*time.Second, func() {
		forced <- struct{}{}
		signals <- syscall.SIGKILL
	})
	err := waitHostProcessGroup(owner, signals)
	joined = true
	timer.Stop()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-forced:
		t.Fatal("terminal job required emergency disposal")
	default:
	}
	if err := restoreDockerExecForeground(0, syscall.Getpgrp()); err != nil {
		t.Fatal(err)
	}
	fmt.Println("__CONTROLLER_DONE__")
}

func TestHostSignalTerminalTopologyAndJobControl(t *testing.T) {
	master, slave := openHostSignalPTY(t)
	controller := exec.Command(os.Args[0], "-test.run=^TestHostSignalTerminalControllerLinux$")
	controller.Env = append(os.Environ(), "DOCKPIPE_SIGNAL_TEST_CONTROLLER=1")
	controller.Stdin, controller.Stdout, controller.Stderr = slave, slave, slave
	controller.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := controller.Start(); err != nil {
		t.Fatal(err)
	}
	joined := false
	defer func() {
		if !joined {
			_ = controller.Process.Kill()
			_ = controller.Wait()
		}
	}()
	var output bytes.Buffer
	waitHostSignalPTY(t, master, &output, "__HOST_READY__")
	if _, err := master.Write([]byte{26}); err != nil {
		t.Fatal(err)
	}
	waitHostSignalPTY(t, master, &output, "__JOB_STOPPED__")
	waitHostSignalPTY(t, master, &output, "__JOB_RESUMED__")
	if _, err := master.WriteString("host input\n"); err != nil {
		t.Fatal(err)
	}
	waitHostSignalPTY(t, master, &output, "__HOST_INPUT__:host input")
	waitHostSignalPTY(t, master, &output, "__OWNER_READY__")
	if _, err := master.WriteString("owner input\n"); err != nil {
		t.Fatal(err)
	}
	waitHostSignalPTY(t, master, &output, "__OWNER_INPUT_RESTORED__")
	waitHostSignalPTY(t, master, &output, "__CONTROLLER_DONE__")
	err := controller.Wait()
	joined = true
	if err != nil {
		t.Fatalf("host PTY helper failed: %v; %s", err, output.String())
	}
}

func TestHostSignalTerminalProbeLinux(t *testing.T) {
	mode := os.Getenv("DOCKPIPE_SIGNAL_TEST_TERMINAL_PROBE")
	if mode == "" {
		return
	}
	if mode == "controlling" {
		for _, descriptor := range []int{0, 1, 2} {
			if _, err := unix.IoctlGetTermios(descriptor, unix.TCGETS); !errors.Is(err, unix.ENOTTY) {
				t.Fatalf("redirected terminal probe descriptor %d: %v", descriptor, err)
			}
		}
	}
	command := exec.Command("bash", "-c", "exit 0")
	setRunHostProcAttrs(command)
	if command.SysProcAttr.Setpgid {
		t.Fatalf("%s terminal was misclassified as headless", mode)
	}
}

func TestHostSignalTerminalDetectionBeyondStdin(t *testing.T) {
	for _, mode := range []string{"controlling", "attached"} {
		t.Run(mode, func(t *testing.T) {
			_, slave := openHostSignalPTY(t)
			command := exec.Command(os.Args[0], "-test.run=^TestHostSignalTerminalProbeLinux$")
			command.Env = append(os.Environ(), "DOCKPIPE_SIGNAL_TEST_TERMINAL_PROBE="+mode)
			command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			var output bytes.Buffer
			command.Stdout, command.Stderr = &output, &output
			if mode == "controlling" {
				command.ExtraFiles = []*os.File{slave}
				command.SysProcAttr.Setctty = true
				command.SysProcAttr.Ctty = 3
			} else {
				command.Stdout = slave
			}
			if err := command.Run(); err != nil {
				t.Fatalf("terminal detection helper failed: %v; %s", err, output.String())
			}
		})
	}
}

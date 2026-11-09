//go:build linux

package containerscmd

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func openContainerExecTestPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	descriptor, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	master := os.NewFile(uintptr(descriptor), "container-exec-master")
	t.Cleanup(func() { _ = master.Close() })
	name, err := unix.IoctlGetInt(descriptor, unix.TIOCGPTN)
	if err == nil {
		err = unix.IoctlSetPointerInt(descriptor, unix.TIOCSPTLCK, 0)
	}
	if err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", name), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = slave.Close() })
	return master, slave
}

func waitContainerExecPTYText(t *testing.T, master *os.File, output *bytes.Buffer, expected string) {
	t.Helper()
	if err := master.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for !strings.Contains(output.String(), expected) {
		var chunk [1024]byte
		count, err := master.Read(chunk[:])
		if count > 0 {
			if output.Len()+count > 64*1024 {
				t.Fatal("container exec PTY fixture exceeded its 64-KiB output budget")
			}
			output.Write(chunk[:count])
		}
		if err != nil && !strings.Contains(output.String(), expected) {
			t.Fatalf("PTY did not produce %q: %v; output=%q", expected, err, output.String())
		}
	}
}

// TestContainerExecPTYProcess is the external-terminal owner. It exercises the
// real containers CLI branch, then reads the owner's post-command prompt input.
func TestContainerExecPTYProcess(t *testing.T) {
	mode := os.Getenv("CONTAINER_EXEC_PTY_HELPER")
	if mode == "" {
		return
	}
	initialState, err := unix.IoctlGetTermios(0, unix.TCGETS)
	if err != nil {
		os.Exit(80)
	}
	foreground, err := unix.IoctlGetInt(0, unix.TIOCGPGRP)
	if err != nil {
		os.Exit(81)
	}
	code := Run([]string{"exec", "--container", containerID, "--shell", "/bin/sh"})
	if (mode == "success" || mode == "mixed") && code != nil || mode == "cancel" && !errors.Is(code, context.Canceled) {
		fmt.Fprintf(os.Stderr, "unexpected exec outcome: %v\n", code)
		os.Exit(82)
	}
	restored, err := unix.IoctlGetInt(0, unix.TIOCGPGRP)
	if err != nil || restored != foreground {
		os.Exit(83)
	}
	restoredState, err := unix.IoctlGetTermios(0, unix.TCGETS)
	if err != nil || restoredState.Lflag != initialState.Lflag {
		os.Exit(85)
	}
	fmt.Fprintln(os.Stdin, "__PARENT_READY__")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil || line != "owner prompt input\n" {
		os.Exit(84)
	}
	fmt.Fprintln(os.Stdin, "__PARENT_INPUT_RESTORED__")
	os.Exit(0)
}

func TestContainerExecCLIRealPTYInputAndForegroundRestoration(t *testing.T) {
	for _, mode := range []string{"success", "cancel", "mixed"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			provider := filepath.Join(directory, "docker")
			script := `#!/bin/sh
set -eu
if [ "$CONTAINER_EXEC_PTY_HELPER" = mixed ]; then
    test "$#" = 5
    test "$1:$2:$3:$4:$5" = 'container:exec:-i:` + containerID + `:/bin/sh'
else
    test "$#" = 6
    test "$1:$2:$3:$4:$5:$6" = 'container:exec:-i:-t:` + containerID + `:/bin/sh'
fi
stty -echo -icanon min 1 time 0 < /dev/tty
printf '__EXEC_READY__\n' > /dev/tty
IFS= read -r line
printf '__EXEC_INPUT__:%s\n' "$line" > /dev/tty
printf '__STDOUT_INHERITED__\n'
if [ "$CONTAINER_EXEC_PTY_HELPER" = cancel ]; then
    IFS= read -r unused
fi
`
			if err := os.WriteFile(provider, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			master, slave := openContainerExecTestPTY(t)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestContainerExecPTYProcess$")
			command.Env = append(os.Environ(), "CONTAINER_EXEC_PTY_HELPER="+mode, "DOCKPIPE_DOCKER_BIN="+provider)
			command.Stdin, command.Stdout, command.Stderr = slave, slave, slave
			var redirected *os.File
			if mode == "mixed" {
				var err error
				redirected, err = os.Create(filepath.Join(directory, "redirected-output"))
				if err != nil {
					t.Fatal(err)
				}
				defer redirected.Close()
				command.Stdout, command.Stderr = redirected, redirected
			}
			command.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if command.ProcessState == nil {
					_ = command.Process.Kill()
				}
			}()
			var output bytes.Buffer
			waitContainerExecPTYText(t, master, &output, "__EXEC_READY__")
			if _, err := master.WriteString("typed container input\n"); err != nil {
				t.Fatal(err)
			}
			waitContainerExecPTYText(t, master, &output, "__EXEC_INPUT__:typed container input")
			if mode == "cancel" {
				if err := command.Process.Signal(syscall.SIGTERM); err != nil {
					t.Fatal(err)
				}
			}
			waitContainerExecPTYText(t, master, &output, "__PARENT_READY__")
			if _, err := master.WriteString("owner prompt input\n"); err != nil {
				t.Fatal(err)
			}
			waitContainerExecPTYText(t, master, &output, "__PARENT_INPUT_RESTORED__")
			if err := command.Wait(); err != nil {
				t.Fatalf("interactive fixture failed: %v; output=%q", err, output.String())
			}
			if mode == "mixed" {
				actual, err := os.ReadFile(redirected.Name())
				if err != nil || !bytes.Contains(actual, []byte("__STDOUT_INHERITED__")) {
					t.Fatalf("mixed-stream session lost redirected stdout: %q (%v)", actual, err)
				}
			}
		})
	}
}

package nativehost

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type ptyProcess struct {
	master     *os.File
	command    *exec.Cmd
	closeOnce  sync.Once
	finished   chan struct{}
	exited     chan struct{}
	cleaned    chan struct{}
	cleanupErr error
	waitErr    error
	exit       int
}

func loginShell(uid uint32) string {
	file, err := os.Open("/etc/passwd")
	if err != nil {
		return "/bin/sh"
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 8192)
	for count := 0; count < 65536 && scanner.Scan(); count++ {
		fields := strings.Split(scanner.Text(), ":")
		if len(fields) == 7 && fields[2] == strconv.FormatUint(uint64(uid), 10) && filepath.IsAbs(fields[6]) && len(fields[6]) <= 4096 {
			return fields[6]
		}
	}
	return "/bin/sh"
}

func startPTY(identity Status, request frame) (*ptyProcess, error) {
	cwd := request.CWD
	if cwd == "" {
		cwd = identity.Home
	}
	if !filepath.IsAbs(cwd) || len(cwd) > 4096 || strings.IndexByte(cwd, 0) >= 0 {
		return nil, errors.New("invalid native host terminal directory")
	}
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	if err := unix.SetNonblock(fd, true); err != nil {
		return nil, errors.Join(err, unix.Close(fd))
	}
	master := os.NewFile(uintptr(fd), "native-host-pty")
	fail := func(err error) (*ptyProcess, error) {
		return nil, errors.Join(err, master.Close())
	}
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		return fail(err)
	}
	number, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		return fail(err)
	}
	slave, err := os.OpenFile("/dev/pts/"+strconv.Itoa(number), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return fail(err)
	}
	defer slave.Close()
	shell := loginShell(identity.UID)
	command := exec.Command(shell, "-l")
	command.Dir = cwd
	command.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin", "HOME=" + identity.Home, "USER=" + identity.Username, "LOGNAME=" + identity.Username, "SHELL=" + shell, "TERM=xterm-256color", "LANG=C.UTF-8"}
	command.Stdin = slave
	command.Stdout = slave
	command.Stderr = slave
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	process := &ptyProcess{master: master, command: command, finished: make(chan struct{}), exited: make(chan struct{}), cleaned: make(chan struct{})}
	if err := process.Resize(request.Rows, request.Columns); err != nil {
		return fail(err)
	}
	if err := command.Start(); err != nil {
		return fail(err)
	}
	go func() {
		// Keep the leader waitable until session cleanup completes. Reaping
		// first would release its numeric session ID for an unrelated shell.
		var information unix.Siginfo
		for {
			process.waitErr = unix.Waitid(unix.P_PID, command.Process.Pid, &information, unix.WEXITED|unix.WNOWAIT, nil)
			if process.waitErr != unix.EINTR {
				break
			}
		}
		close(process.exited)
		<-process.cleaned
		err := command.Wait()
		if err != nil {
			process.exit = 1
			if exit, ok := err.(*exec.ExitError); ok {
				process.exit = exit.ExitCode()
			}
		}
		close(process.finished)
	}()
	return process, nil
}

func (p *ptyProcess) Resize(rows, columns uint16) error {
	if rows == 0 {
		rows = 24
	}
	if columns == 0 {
		columns = 80
	}
	return unix.IoctlSetWinsize(int(p.master.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: rows, Col: columns})
}

func (p *ptyProcess) Close() error {
	p.closeOnce.Do(func() {
		p.cleanupErr = p.master.Close()
		defer close(p.cleaned)
		// This non-reaping check also refuses to signal a numeric ID whose
		// child ownership was unexpectedly lost to an outside reaper.
		var information unix.Siginfo
		if err := unix.Waitid(unix.P_PID, p.command.Process.Pid, &information, unix.WEXITED|unix.WNOWAIT|unix.WNOHANG, nil); err != nil {
			p.cleanupErr = errors.Join(p.cleanupErr, errors.New("native host terminal ownership unavailable"))
			return
		}
		// The shell owns a distinct session. Kill processes still in that
		// session as well as its process group, including job-control groups.
		p.cleanupErr = errors.Join(p.cleanupErr, killSession(p.command.Process.Pid))
	})
	return p.cleanupErr
}

func (p *ptyProcess) Wait() int {
	select {
	case <-p.exited:
	case <-time.After(2 * time.Second):
	}
	cleanupErr := p.Close()
	select {
	case <-p.finished:
		if cleanupErr != nil || p.waitErr != nil {
			return 1
		}
		return p.exit
	case <-time.After(2 * time.Second):
		return 1
	}
}

func killSession(session int) error {
	directory, err := os.Open("/proc")
	if err != nil {
		return errors.New("native host terminal cleanup scan unavailable")
	}
	defer directory.Close()
	deadline := time.Now().Add(time.Second)
	var cleanupErr error
	count := 0
	for {
		entries, readErr := directory.Readdirnames(128)
		for _, name := range entries {
			count++
			if count > 32768 || time.Now().After(deadline) {
				killErr := syscall.Kill(-session, syscall.SIGKILL)
				if errors.Is(killErr, syscall.ESRCH) {
					killErr = nil
				}
				return errors.Join(errors.New("native host terminal cleanup scan limit exceeded"), killErr)
			}
			pid, err := strconv.Atoi(name)
			if err != nil || pid <= 1 {
				continue
			}
			file, err := os.Open("/proc/" + name + "/stat")
			if err != nil {
				continue
			}
			data := make([]byte, 4096)
			n, readErr := file.Read(data)
			closeErr := file.Close()
			if (readErr != nil && readErr != io.EOF) || closeErr != nil {
				cleanupErr = errors.New("native host terminal process metadata unavailable")
				continue
			}
			text := string(data[:n])
			end := strings.LastIndex(text, ")")
			if end < 0 {
				continue
			}
			fields := strings.Fields(text[end+1:])
			if len(fields) >= 4 && fields[3] == strconv.Itoa(session) {
				if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
					cleanupErr = errors.New("native host terminal process cleanup denied")
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			cleanupErr = errors.New("native host terminal cleanup scan failed")
			break
		}
	}
	if err := syscall.Kill(-session, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		cleanupErr = errors.New("native host terminal process group cleanup denied")
	}
	return cleanupErr
}

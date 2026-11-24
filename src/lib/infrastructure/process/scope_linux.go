//go:build linux

package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const cgroupMount = "/sys/fs/cgroup"

func HostBootIdentity() (string, error) {
	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", err
	}
	boot := strings.TrimSpace(string(data))
	if len(boot) != 36 {
		return "", errors.New("kernel boot identity unavailable")
	}
	return boot, nil
}

func createScope(id, base string) (ScopeReference, error) {
	if len(id) != 64 || strings.Trim(id, "0123456789abcdef") != "" {
		return ScopeReference{}, errors.New("invalid durable process scope identity")
	}
	boot, err := HostBootIdentity()
	if err != nil {
		return ScopeReference{}, err
	}
	if base == "" {
		membership, err := os.ReadFile("/proc/self/cgroup")
		if err != nil || !strings.HasPrefix(string(membership), "0::/") {
			return ScopeReference{}, errors.New("process recovery requires delegated cgroup v2")
		}
		base = filepath.Join(cgroupMount, strings.TrimSpace(strings.TrimPrefix(string(membership), "0::")))
	}
	relative, err := filepath.Rel(cgroupMount, base)
	if err != nil || !filepath.IsAbs(base) || relative == "." || relative == ".." || strings.HasPrefix(relative, "../") {
		return ScopeReference{}, errors.New("process scope root must be a delegated cgroup v2 directory")
	}
	root, err := os.OpenRoot(cgroupMount)
	if err != nil {
		return ScopeReference{}, err
	}
	defer root.Close()
	path := filepath.Join(relative, "dockpipe-feature-"+id)
	if err := root.Mkdir(path, 0700); err != nil {
		return ScopeReference{}, fmt.Errorf("process recovery requires a writable delegated cgroup v2 scope: %w", err)
	}
	info, err := root.Stat(path)
	if err != nil {
		return ScopeReference{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ScopeReference{}, errors.New("process scope kernel identity unavailable")
	}
	return ScopeReference{Mode: "cgroup-v2", Boot: boot, Path: filepath.ToSlash(path), Inode: stat.Ino}, nil
}

func openScope(reference ScopeReference) (*os.Root, *os.File, error) {
	if reference.Inode == 0 || !filepath.IsLocal(reference.Path) || len(reference.Path) > 4096 || strings.ContainsAny(reference.Path, "\x00\r\n") {
		return nil, nil, errors.New("invalid process scope reference")
	}
	root, err := os.OpenRoot(cgroupMount)
	if err != nil {
		return nil, nil, err
	}
	file, err := root.Open(reference.Path)
	if err != nil {
		root.Close() // #nosec G104 -- Release the mount handle and preserve the open error for missing-scope recovery.
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		root.Close() // #nosec G104 -- Release the mount handle and preserve the stat error.
		file.Close() // #nosec G104 -- The read-only scope handle is discarded after stat fails.
		return nil, nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || stat.Ino != reference.Inode {
		root.Close() // #nosec G104 -- Release the mount handle after rejecting the scope identity.
		file.Close() // #nosec G104 -- Release the read-only scope handle after rejecting its identity.
		return nil, nil, errors.New("process scope kernel identity changed")
	}
	return root, file, nil
}

func attachScope(command *exec.Cmd, reference ScopeReference) (func(), error) {
	if reference.Mode == "host-reboot" {
		return func() {}, nil
	}
	boot, err := HostBootIdentity()
	if err != nil || boot != reference.Boot {
		return nil, errors.New("process scope boot identity changed")
	}
	root, file, err := openScope(reference)
	if err != nil {
		return nil, err
	}
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.UseCgroupFD = true
	command.SysProcAttr.CgroupFD = int(file.Fd())
	return func() {
		file.Close() // #nosec G104 -- Release the read-only scope handle after command supervision.
		root.Close() // #nosec G104 -- Release the mount handle after command supervision.
	}, nil
}

func StopScope(ctx context.Context, reference ScopeReference) error {
	if err := reference.Validate(); err != nil {
		return err
	}
	if reference.Mode == "host-reboot" {
		return stopHostBootScope(reference)
	}
	boot, err := HostBootIdentity()
	if err != nil {
		return err
	}
	if reference.Boot == "" {
		return errors.New("missing process scope boot identity")
	}
	if boot != reference.Boot {
		// A kernel reboot terminates every process from the old scope.
		return nil
	}
	root, file, err := openScope(reference)
	if os.IsNotExist(err) {
		// The kernel cannot remove a populated cgroup.
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	defer file.Close()
	killed := false
	for {
		events, err := root.ReadFile(filepath.Join(reference.Path, "cgroup.events"))
		if err != nil {
			return err
		}
		if strings.Contains("\n"+string(events), "\npopulated 0\n") {
			// rmdir is an additional kernel check, including nested descendants.
			return root.Remove(reference.Path)
		}
		if !killed {
			if err := root.WriteFile(filepath.Join(reference.Path, "cgroup.kill"), []byte("1"), 0); err != nil {
				return err
			}
			killed = true
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

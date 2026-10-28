//go:build linux

package remote

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const maximumEdgeBinaryBytes = 128 << 20

func signalEdgeTermination(command *exec.Cmd) {
	_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
}

func edgeImageDigest(pid int) (string, error) {
	file, err := os.Open(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return "", err
	}
	defer file.Close()
	return hashEdgeFile(file)
}

func hashEdgeFile(file *os.File) (string, error) {
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maximumEdgeBinaryBytes {
		return "", errors.New("edge image is not a bounded regular executable")
	}
	hash := sha256.New()
	count, err := io.Copy(hash, io.LimitReader(file, maximumEdgeBinaryBytes+1))
	if err != nil || count > maximumEdgeBinaryBytes {
		return "", errors.New("edge image exceeds its read bound")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func snapshotEdgeBinary(source, destination, expected string) error {
	if err := PrivateDirectory(filepath.Dir(destination)); err != nil {
		return err
	}
	if err := validateEdgeSource(source); err != nil {
		return err
	}
	descriptor, err := syscall.Open(source, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	input := os.NewFile(uintptr(descriptor), source)
	defer input.Close()
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("edge input is not regular")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) || stat.Nlink != 1 || info.Mode().Perm()&0077 != 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || info.Size() > maximumEdgeBinaryBytes {
		return errors.New("edge snapshot source must be bounded, private, singly linked and operator owned")
	}
	return publishEdgeSnapshot(input, destination, expected)
}

func publishEdgeSnapshot(input *os.File, destination, expected string) error {
	output, err := os.CreateTemp(filepath.Dir(destination), "image-")
	if err != nil {
		return err
	}
	defer os.Remove(output.Name())
	defer output.Close()
	hash := sha256.New()
	count, err := io.Copy(io.MultiWriter(output, hash), io.LimitReader(input, maximumEdgeBinaryBytes+1))
	if err != nil || count > maximumEdgeBinaryBytes || hex.EncodeToString(hash.Sum(nil)) != expected {
		return errors.New("edge snapshot checksum mismatch")
	}
	if err := output.Chmod(0500); err != nil {
		return err
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	if err := os.Rename(output.Name(), destination); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(destination))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

// Attribute the readiness listener to the supervised process, rather than a
// healthy old connector or an unrelated service using the same port.
func edgeOwnsSocket(pid int, address string) bool {
	parsed, err := url.Parse(address)
	if err != nil {
		return false
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		return false
	}
	file, err := os.Open(fmt.Sprintf("/proc/%d/net/tcp", pid))
	if err != nil {
		return false
	}
	defer file.Close()
	scanner := bufio.NewScanner(io.LimitReader(file, 2<<20))
	inodes := map[string]bool{}
	lines := 0
	for scanner.Scan() {
		lines++
		if lines > 16384 {
			return false
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) > 9 && fields[1] == fmt.Sprintf("0100007F:%04X", port) && fields[3] == "0A" {
			inodes["socket:["+fields[9]+"]"] = true
		}
	}
	if scanner.Err() != nil || len(inodes) == 0 {
		return false
	}
	directory, err := os.Open(fmt.Sprintf("/proc/%d/fd", pid))
	if err != nil {
		return false
	}
	defer directory.Close()
	names, err := directory.Readdirnames(1024)
	if err != nil && err != io.EOF {
		return false
	}
	for _, name := range names {
		target, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%s", pid, name))
		if err == nil && inodes[target] {
			return true
		}
	}
	return false
}

func edgePrivateDigest(path string) (string, error) {
	if err := validateEdgeSource(path); err != nil {
		return "", err
	}
	descriptor, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return "", err
	}
	file := os.NewFile(uintptr(descriptor), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) || stat.Nlink != 1 || info.Mode().Perm()&0077 != 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return "", errors.New("edge input is not privately owned")
	}
	return hashEdgeFile(file)
}

func bindEdgeParent(command *exec.Cmd) {
	command.SysProcAttr.Pdeathsig = syscall.SIGKILL
}

func snapshotEdgeProcessImage(pid int, destination, expected string) error {
	if err := PrivateDirectory(filepath.Dir(destination)); err != nil {
		return err
	}
	input, err := os.Open(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maximumEdgeBinaryBytes {
		return errors.New("invalid baseline process image")
	}
	return publishEdgeSnapshot(input, destination, expected)
}

func edgeDeclaredDigest(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	descriptor, err := syscall.Open(resolved, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return "", err
	}
	file := os.NewFile(uintptr(descriptor), resolved)
	defer file.Close()
	return hashEdgeFile(file)
}

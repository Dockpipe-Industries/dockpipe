//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

const maximumExecutableBytes = 64 << 20
const addSeals = 1033
const getSeals = 1034
const sealSeals = 0x0001
const sealShrink = 0x0002
const sealGrow = 0x0004
const sealWrite = 0x0008
const requiredSeals = sealWrite | sealShrink | sealGrow | sealSeals
const memfdCloseOnExec = 0x0001
const memfdAllowSealing = 0x0002
const inheritedExecutableFD = 3
const inheritedExecutablePath = "/proc/self/fd/3"

type sealedImage struct {
	file *os.File
}

func newMemfd() (*os.File, error) {
	// CLOEXEC prevents accidental inheritance. ExtraFiles explicitly duplicates
	// this descriptor to FD3 and clears close-on-exec before the intended exec.
	descriptor, err := unix.MemfdCreate("dockpipe-identity-qualification", unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
	if err != nil {
		return nil, errors.New("sealed executable memory is unavailable")
	}
	return os.NewFile(uintptr(descriptor), "sealed identity executable"), nil
}

func executableSize(file *os.File) (int64, error) {
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maximumExecutableBytes {
		return 0, errors.New("executable exceeds its fixed regular-file contract")
	}
	var header [4]byte
	if _, err := file.ReadAt(header[:], 0); err != nil || !bytes.Equal(header[:], []byte{0x7f, 'E', 'L', 'F'}) {
		return 0, errors.New("executable is not a native ELF image")
	}
	return info.Size(), nil
}

func executableDigest(file *os.File) ([32]byte, error) {
	var digest [32]byte
	size, err := executableSize(file)
	if err != nil {
		return digest, err
	}
	hash := sha256.New()
	if _, err := io.CopyN(hash, io.NewSectionReader(file, 0, size), size); err != nil {
		return digest, errors.New("cannot read complete executable image")
	}
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}

func sealRunningExecutable() (*sealedImage, error) {
	// This link names the running inode even if its package path is replaced.
	// Linux denies modifying that executing inode with ETXTBSY. Never reopen the
	// package path and never trust a writable manifest or caller-supplied hash.
	running, err := os.Open("/proc/self/exe")
	if err != nil {
		return nil, errors.New("running executable identity is unavailable")
	}
	defer running.Close()
	size, err := executableSize(running)
	if err != nil {
		return nil, err
	}
	image, err := newMemfd()
	if err != nil {
		return nil, err
	}
	valid := false
	defer func() {
		if !valid {
			_ = image.Close()
		}
	}()
	if _, err := io.CopyN(image, io.NewSectionReader(running, 0, size), size); err != nil {
		return nil, errors.New("cannot create complete sealed executable")
	}
	// The unlinked descriptor must remain executable after the target UID
	// transition. It contains public program bytes, never credentials.
	if image.Chmod(0555) != nil {
		return nil, errors.New("cannot prepare sealed executable permissions")
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_FCNTL, image.Fd(), addSeals, requiredSeals); errno != 0 {
		return nil, errors.New("cannot seal executable bytes and size")
	}
	if err := validateImage(image, running); err != nil {
		return nil, err
	}
	valid = true
	return &sealedImage{file: image}, nil
}

func validateImage(image, running *os.File) error {
	seals, _, errno := syscall.Syscall(syscall.SYS_FCNTL, image.Fd(), getSeals, 0)
	if errno != 0 || seals&requiredSeals != requiredSeals {
		return errors.New("inherited executable lacks immutable-image seals")
	}
	actual, err := executableDigest(image)
	if err != nil {
		return err
	}
	expected, err := executableDigest(running)
	if err != nil || actual != expected {
		return errors.New("inherited executable does not match the running image")
	}
	return nil
}

func inheritExecutable() (*sealedImage, error) {
	image := os.NewFile(inheritedExecutableFD, "inherited sealed executable")
	if image == nil {
		return nil, errors.New("sealed executable descriptor is missing")
	}
	// Own this descriptor, and inherit it onward only through ExtraFiles.
	if _, err := executableSize(image); err != nil {
		_ = image.Close()
		return nil, err
	}
	syscall.CloseOnExec(inheritedExecutableFD)
	running, err := os.Open("/proc/self/exe")
	if err != nil {
		_ = image.Close()
		return nil, errors.New("cannot verify inherited executable identity")
	}
	defer running.Close()
	if err := validateImage(image, running); err != nil {
		_ = image.Close()
		return nil, err
	}
	return &sealedImage{file: image}, nil
}

func (image *sealedImage) command(ctx context.Context, role string) (*exec.Cmd, error) {
	if role != "identity-worker" && role != "identity-probe" {
		return nil, errors.New("unsupported sealed executable role")
	}
	if image == nil || image.file == nil {
		return nil, errors.New("sealed executable is unavailable")
	}
	command := exec.CommandContext(ctx, inheritedExecutablePath)
	command.Args[0] = role
	command.ExtraFiles = []*os.File{image.file}
	return command, nil
}

func (image *sealedImage) Close() error {
	return image.file.Close()
}

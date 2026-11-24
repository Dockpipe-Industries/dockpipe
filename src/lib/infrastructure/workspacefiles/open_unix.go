//go:build linux || darwin

package workspacefiles

import (
	"errors"
	"math"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func openRead(root *os.Root, name string) (*os.File, error) {
	// A concurrent replacement with a FIFO cannot hang before the regular-file
	// check. Never read a device or FIFO merely because an earlier stat passed.
	return root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}

func removeFile(root *os.Root, name string) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	descriptor := directory.Fd()
	if descriptor > uintptr(math.MaxInt) {
		return errors.New("directory descriptor exceeds the platform range")
	}
	return unix.Unlinkat(int(descriptor), name, 0)
}

func linkBetween(_ *os.Root, source *os.Root, sourceName string, destination *os.Root,
	destinationName, _, _ string) error {
	sourceDirectory, err := source.Open(".")
	if err != nil {
		return err
	}
	defer sourceDirectory.Close()
	destinationDirectory, err := destination.Open(".")
	if err != nil {
		return err
	}
	defer destinationDirectory.Close()
	sourceFD, destinationFD := sourceDirectory.Fd(), destinationDirectory.Fd()
	if sourceFD > uintptr(math.MaxInt) || destinationFD > uintptr(math.MaxInt) {
		return errors.New("directory descriptor exceeds the platform range")
	}
	return unix.Linkat(int(sourceFD), sourceName, int(destinationFD), destinationName, 0)
}

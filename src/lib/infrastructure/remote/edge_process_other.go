//go:build !linux

package remote

import (
	"errors"
	"os/exec"
)

func signalEdgeTermination(command *exec.Cmd) { CleanupProcess(command) }
func edgeImageDigest(pid int) (string, error) {
	return "", errors.New("active edge image verification is currently supported only on Linux")
}
func snapshotEdgeBinary(source, destination, expected string) error {
	return errors.New("edge activation is currently supported only on Linux")
}
func edgeOwnsSocket(pid int, address string) bool { return false }

func edgePrivateDigest(path string) (string, error) {
	return "", errors.New("edge snapshot verification requires Linux")
}

func bindEdgeParent(command *exec.Cmd) {}
func snapshotEdgeProcessImage(pid int, destination, expected string) error {
	return errors.New("pinned edge activation requires Linux")
}

func edgeDeclaredDigest(path string) (string, error) {
	return "", errors.New("native image pinning unavailable")
}

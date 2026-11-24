//go:build !linux && !darwin

package workspacefiles

import "os"

func openRead(root *os.Root, name string) (*os.File, error) {
	return root.Open(name)
}

func linkBetween(root, _ *os.Root, _ string, _ *os.Root, _, sourcePath, destinationPath string) error {
	return root.Link(sourcePath, destinationPath)
}

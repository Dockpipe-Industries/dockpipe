//go:build !windows && !linux

package sqliteevidence

import "fmt"

func collectAndProtectWindowsHost(string) (windowsHostFacts, error) {
	return windowsHostFacts{}, fmt.Errorf("Windows native evidence is unavailable on this host")
}

func requireWindowsPrivatePath(string) (string, error) {
	return "", fmt.Errorf("Windows DACL evidence is unavailable on this host")
}

func setWindowsPrivateDirectory(string) error {
	return fmt.Errorf("Windows DACL evidence is unavailable on this host")
}

func selectedNativeVFS() string { return "unix" }

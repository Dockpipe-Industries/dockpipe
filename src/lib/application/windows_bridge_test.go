package application

import (
	"os"
	"testing"
)

// TestUseWSLBridge reads DOCKPIPE_USE_WSL_BRIDGE for 0/1/unset.
func TestUseWSLBridge(t *testing.T) {
	t.Setenv(EnvUseWSLBridge, "")
	if UseWSLBridge() {
		t.Fatal("expected false when unset")
	}
	t.Setenv(EnvUseWSLBridge, "0")
	if UseWSLBridge() {
		t.Fatal("expected false for 0")
	}
	t.Setenv(EnvUseWSLBridge, "1")
	if !UseWSLBridge() {
		t.Fatal("expected true for 1")
	}
	_ = os.Unsetenv(EnvUseWSLBridge)
}

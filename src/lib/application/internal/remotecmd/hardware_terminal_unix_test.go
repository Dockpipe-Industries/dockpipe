//go:build linux || darwin

package remotecmd

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestTerminalReadRejectsInvalidDescriptorBeforePolling(t *testing.T) {
	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	terminal := contextTerminalIO{ctx: context.Background(), file: file}
	if _, err := terminal.Read(make([]byte, 1)); err == nil || !strings.Contains(err.Error(), "descriptor") {
		t.Fatalf("invalid descriptor did not fail before native polling: %v", err)
	}
}

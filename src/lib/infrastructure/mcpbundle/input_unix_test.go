//go:build linux || darwin

package mcpbundle

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMCPInputsRejectSpecialFilesWithoutBlocking(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "input")
	if err := syscall.Mkfifo(pipe, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, operation := range []func() error{
		func() error {
			_, err := Inspect(ctx, pipe, strings.Repeat("0", 64), "linux", "amd64")
			return err
		},
		func() error {
			_, err := Install(ctx, pipe, strings.Repeat("0", 64), "linux", "amd64", t.TempDir())
			return err
		},
		func() error {
			_, err := ReadBindings(ctx, pipe)
			return err
		},
	} {
		done := make(chan error, 1)
		go func() { done <- operation() }()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("special input accepted")
			}
		case <-ctx.Done():
			// Unblock a regressed reader before failing so the fixture does not
			// leave its own goroutine behind.
			writer, err := os.OpenFile(pipe, os.O_WRONLY|syscall.O_NONBLOCK, 0)
			if err == nil {
				writer.Close()
			}
			t.Fatal("special-file input blocked")
		}
	}
}

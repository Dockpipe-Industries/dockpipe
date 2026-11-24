//go:build linux || darwin

package workspacefilescmd

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRequestFIFOIsRejectedWithoutWaitingForWriter(t *testing.T) {
	name := filepath.Join(t.TempDir(), "request")
	if err := syscall.Mkfifo(name, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- Run([]string{"--request", name}) }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "regular JSON file") {
			t.Fatalf("FIFO request was not rejected by its opened descriptor: %v", err)
		}
	case <-time.After(time.Second):
		// Release a regressed blocking open before failing, rather than leaving
		// a stuck fixture goroutine behind for the rest of the test process.
		writer, err := os.OpenFile(name, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err == nil {
			writer.Close()
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		t.Fatal("request open waited for a FIFO writer")
	}
}

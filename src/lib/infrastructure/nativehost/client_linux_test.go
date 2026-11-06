package nativehost

import (
	"errors"
	"testing"
)

type failingCloseWire struct {
	memoryWire
	closeErr error
	closes   int
}

func (w *failingCloseWire) Close() error {
	w.closes++
	return w.closeErr
}

func TestTerminalClosePreservesFailureAndIsIdempotent(t *testing.T) {
	closeErr := errors.New("transport cleanup failed")
	connection := &failingCloseWire{closeErr: closeErr}
	terminal := &Terminal{transport: &wire{ReadWriteCloser: connection}, done: make(chan struct{})}
	for range 2 {
		if err := terminal.Close(); !errors.Is(err, closeErr) {
			t.Fatalf("close error = %v, want transport failure", err)
		}
	}
	if connection.closes != 1 {
		t.Fatalf("transport closed %d times, want once", connection.closes)
	}
	select {
	case <-terminal.done:
	default:
		t.Fatal("terminal shutdown was not signalled")
	}
}

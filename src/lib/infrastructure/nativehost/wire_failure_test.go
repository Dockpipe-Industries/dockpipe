package nativehost

import (
	"errors"
	"testing"
	"time"
)

type deadlineRejectingWire struct {
	memoryWire
	deadlineErr error
}

func (w *deadlineRejectingWire) SetWriteDeadline(time.Time) error {
	return w.deadlineErr
}

func TestWireDeadlineFailurePreventsUnboundedWrite(t *testing.T) {
	deadlineErr := errors.New("write deadline unavailable")
	connection := &deadlineRejectingWire{deadlineErr: deadlineErr}
	transport := &wire{ReadWriteCloser: connection}
	if err := transport.send(frame{Kind: "status"}); !errors.Is(err, deadlineErr) {
		t.Fatalf("send error = %v, want deadline error", err)
	}
	if connection.Len() != 0 {
		t.Fatal("frame written after deadline admission failed")
	}
}

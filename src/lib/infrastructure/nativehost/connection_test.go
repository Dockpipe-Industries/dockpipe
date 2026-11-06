package nativehost

import (
	"bytes"
	"encoding/binary"
	"testing"
)

type memoryWire struct{ bytes.Buffer }

func (*memoryWire) Close() error { return nil }

func TestWireRejectsOversizeBeforeAllocation(t *testing.T) {
	var input memoryWire
	binary.Write(&input, binary.BigEndian, uint32(maxFrame+1))
	if _, err := (&wire{ReadWriteCloser: &input}).receive(); err == nil {
		t.Fatal("oversized frame accepted")
	}
}

func TestWireTerminalBound(t *testing.T) {
	var input memoryWire
	transport := &wire{ReadWriteCloser: &input}
	if err := transport.send(frame{Kind: "input", Data: make([]byte, maxTerminalData+1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := transport.receive(); err == nil {
		t.Fatal("oversized terminal input accepted")
	}
}

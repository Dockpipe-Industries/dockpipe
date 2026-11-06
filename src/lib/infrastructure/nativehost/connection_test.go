package nativehost

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
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

func TestConnectionReferenceAndKeyAdmission(t *testing.T) {
	directory := t.TempDir()
	key := filepath.Join(directory, "key")
	config := filepath.Join(directory, "connection.json")
	if err := os.WriteFile(key, []byte(strings.Repeat("a", 64)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte(`{"schema":"dockpipe.native_host_connection.v1","socket":"/tmp/host.sock","key_file":"`+key+`"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadConnection(config); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readKey(key); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(key, 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readKey(key); err == nil {
		t.Fatal("public key file accepted")
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(config, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadConnection(link); err == nil {
		t.Fatal("symlink config accepted")
	}
	if _, err := ReadConnection("relative.json"); err == nil {
		t.Fatal("relative config accepted")
	}
}

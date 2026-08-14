//go:build linux

package executor

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"dockpipe.vm/tools/internal/protocol"
	"dockpipe.vm/tools/internal/provisioning"
)

func TestGate3ConsoleFinishAcceptsItsOwnedConnectionClose(t *testing.T) {
	root := t.TempDir()
	evidencePath := filepath.Join(root, "console.log")
	file, err := os.OpenFile(evidencePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	prefix := []byte("preserved Gate 3 console prefix\n")
	conn := &ownedCloseConsoleConn{prefix: prefix, closed: make(chan struct{})}
	session := &gate3ConsoleSession{conn: conn, file: file, done: make(chan error, 1)}
	go func() {
		_, copyErr := io.Copy(file, conn)
		session.done <- copyErr
	}()
	deadline := time.Now().Add(time.Second)
	for {
		info, statErr := os.Stat(evidencePath)
		if statErr != nil {
			t.Fatal(statErr)
		}
		if info.Size() == int64(len(prefix)) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("console prefix size = %d, want %d", info.Size(), len(prefix))
		}
		time.Sleep(time.Millisecond)
	}
	if err := session.finish(); err != nil {
		t.Fatalf("owned console connection close must not become a preservation error: %v", err)
	}
	got, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(prefix) {
		t.Fatalf("preserved console = %q, want %q", got, prefix)
	}
}

func TestGate3ConsoleFinishStillPropagatesCaptureErrors(t *testing.T) {
	root := t.TempDir()
	evidencePath := filepath.Join(root, "console.log")
	file, err := os.OpenFile(evidencePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("console capture failed")
	conn := &ownedCloseConsoleConn{prefix: []byte("prefix\n"), terminal: want, closed: make(chan struct{})}
	session := &gate3ConsoleSession{conn: conn, file: file, done: make(chan error, 1)}
	go func() {
		_, copyErr := io.Copy(file, conn)
		session.done <- copyErr
	}()
	if got := session.finish(); !errors.Is(got, want) {
		t.Fatalf("finish error = %v, want capture error %v", got, want)
	}
}

func TestGate3ExchangeDurablyRecordsSignedCheckpointResponseDelivery(t *testing.T) {
	controllerPublic, controllerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	guestPublic, guestPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	boot := protocol.SignedFrame{Context: protocol.Context{
		MachineUUID: "11111111-1111-4111-8111-111111111111", DiskSerial: "dockpipe-data-000001",
		BootID: "22222222-2222-4222-8222-222222222222", RunID: "run-001", Scenario: "sqlite-wal",
		DurabilityBoundary: "after-fsync",
	}}
	runner := &Gate3LinuxRunner{
		config: Gate3RunnerConfig{Plan: Gate3Plan{EvidenceRoot: t.TempDir()}, Keys: provisioning.KeyMaterial{ControllerPrivate: controllerPrivate, GuestPublic: guestPublic}, Now: func() time.Time { return now }},
		agent:  client, boot: boot, sequence: protocol.FirstRequestSequence, nonces: map[string]bool{},
	}
	written := make(chan []byte, 1)
	serverError := make(chan error, 1)
	go func() {
		requestJSON, readErr := protocol.ReadFramed(server)
		if readErr != nil {
			serverError <- readErr
			return
		}
		requestFrame, verifyErr := protocol.Verify(requestJSON, controllerPublic, now)
		if verifyErr != nil {
			serverError <- verifyErr
			return
		}
		response, signErr := protocol.Sign(protocol.ResultKind, requestFrame.Capability, requestFrame.Context, map[string]any{"accepted": true}, now, now.Add(time.Minute), guestPrivate)
		if signErr != nil {
			serverError <- signErr
			return
		}
		if writeErr := protocol.WriteFramed(server, response); writeErr != nil {
			serverError <- writeErr
			return
		}
		written <- response
	}()
	deliveryPath := filepath.Join(runner.config.Plan.EvidenceRoot, "checkpoint-response-delivered.json")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	response, err := runner.exchange(ctx, "checkpoint/v1", map[string]any{"trial_id": "after-stage-before-commit-1"}, "gate3-checkpoint", deliveryPath)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-serverError:
		t.Fatal(err)
	default:
	}
	want := <-written
	stored, err := os.ReadFile(deliveryPath)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(deliveryPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(response, want) || !bytes.Equal(stored, want) || info.Mode().Perm() != 0o600 {
		t.Fatalf("signed response delivery changed: response=%t stored=%t mode=%o", bytes.Equal(response, want), bytes.Equal(stored, want), info.Mode().Perm())
	}
	if strings.Contains(string(stored), "ticket_nonce") {
		t.Fatal("response-delivery evidence unexpectedly contains the request nonce")
	}
}

type ownedCloseConsoleConn struct {
	prefix   []byte
	terminal error
	closed   chan struct{}
	mu       sync.Mutex
	read     bool
	once     sync.Once
}

func (c *ownedCloseConsoleConn) Read(buffer []byte) (int, error) {
	c.mu.Lock()
	if !c.read {
		c.read = true
		count := copy(buffer, c.prefix)
		c.mu.Unlock()
		return count, nil
	}
	c.mu.Unlock()
	if c.terminal != nil {
		return 0, c.terminal
	}
	<-c.closed
	return 0, &net.OpError{Op: "read", Net: "unix", Err: net.ErrClosed}
}

func (*ownedCloseConsoleConn) Write([]byte) (int, error) { return 0, net.ErrClosed }
func (c *ownedCloseConsoleConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}
func (*ownedCloseConsoleConn) LocalAddr() net.Addr              { return consoleTestAddr("local") }
func (*ownedCloseConsoleConn) RemoteAddr() net.Addr             { return consoleTestAddr("remote") }
func (*ownedCloseConsoleConn) SetDeadline(time.Time) error      { return nil }
func (*ownedCloseConsoleConn) SetReadDeadline(time.Time) error  { return nil }
func (*ownedCloseConsoleConn) SetWriteDeadline(time.Time) error { return nil }

type consoleTestAddr string

func (a consoleTestAddr) Network() string { return "unix" }
func (a consoleTestAddr) String() string  { return string(a) }

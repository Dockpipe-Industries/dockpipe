package nativehost

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func startTestBroker(t *testing.T) (*Broker, *Client, func(Grant)) {
	t.Helper()
	requireTestWorkerIdentity(t)
	directory, err := os.MkdirTemp("", "nh-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(directory) })
	var mu sync.Mutex
	grant := Grant{UID: uint32(os.Getuid()), Generation: "test-generation"}
	key := strings.Repeat("b", 64)
	broker, err := NewBroker(Options{WorkerSocket: filepath.Join(directory, "worker.sock"), ClientSocket: filepath.Join(directory, "client.sock"), ClientKey: key, ClientUID: uint32(os.Getuid()), Authorize: func() (Grant, error) {
		mu.Lock()
		defer mu.Unlock()
		return grant, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() { broker.Close() })
	result := make(chan error, 1)
	go func() { result <- broker.Serve(ctx) }()
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(broker.options.ClientSocket); err == nil {
			break
		}
		select {
		case err := <-result:
			t.Fatalf("broker listen failed: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("broker did not start")
		}
		time.Sleep(time.Millisecond)
	}
	keyPath := filepath.Join(directory, "client.key")
	os.WriteFile(keyPath, []byte(key), 0600)
	configPath := filepath.Join(directory, "connection.json")
	config, _ := json.Marshal(Connection{Schema: ConnectionSchema, Socket: broker.options.ClientSocket, KeyFile: keyPath})
	os.WriteFile(configPath, config, 0600)
	client, err := NewClient(configPath)
	if err != nil {
		t.Fatal(err)
	}
	return broker, client, func(next Grant) { mu.Lock(); grant = next; mu.Unlock() }
}

func TestBrokerWorkerSnapshotAndRevocation(t *testing.T) {
	broker, client, changeGrant := startTestBroker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if status, err := client.Status(ctx); err == nil || status.Available {
		t.Fatal("missing worker reported available")
	}
	nonce := strings.Repeat("c", 64)
	if err := broker.ExpectWorker(uint32(os.Getuid()), uint32(os.Getgid()), nonce, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- RunWorker(ctx, WorkerOptions{Socket: broker.options.WorkerSocket, Nonce: nonce, Ready: func() error { close(ready); return nil }})
	}()
	select {
	case <-ready:
	case err := <-workerDone:
		t.Fatalf("worker startup: %v", err)
	case <-ctx.Done():
		t.Fatal("worker startup timeout")
	}
	status, err := client.Status(ctx)
	if err != nil || !status.Available || status.UID != uint32(os.Getuid()) || status.Generation != "test-generation" {
		t.Fatalf("unexpected status: %+v %v", status, err)
	}
	hostname, _ := os.Hostname()
	if status.Node != hostname {
		t.Fatal("worker hostname differs")
	}
	snapshot, err := client.Snapshot(ctx, false, false)
	if err != nil || snapshot.Node != hostname {
		t.Fatalf("snapshot: %+v %v", snapshot, err)
	}
	terminal, err := client.OpenTerminal(ctx, TerminalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer terminal.Close()
	pidFile := filepath.Join(filepath.Dir(broker.options.WorkerSocket), "job.pid")
	if _, err := terminal.Write([]byte(fmt.Sprintf("sleep 60 & echo $! > %q\n", pidFile))); err != nil {
		t.Fatal(err)
	}
	var jobPID int
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			jobPID, _ = strconv.Atoi(strings.TrimSpace(string(data)))
			if jobPID > 1 {
				break
			}
		}
		time.Sleep(time.Millisecond)
	}
	if jobPID <= 1 {
		t.Fatal("terminal did not execute user-owned job")
	}
	output := make(chan error, 1)
	go func() { _, err := terminal.CopyOutput(io.Discard); output <- err }()
	changeGrant(Grant{UID: uint32(os.Getuid()), Generation: "replacement"})
	select {
	case <-output:
	case <-time.After(4 * time.Second):
		t.Fatal("revocation did not close terminal")
	}
	select {
	case <-workerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("revocation did not clean up worker")
	}
	if data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", jobPID)); err == nil {
		end := strings.LastIndex(string(data), ")")
		if end < 0 || !strings.HasPrefix(strings.TrimSpace(string(data[end+1:])), "Z ") {
			t.Fatalf("owned shell job still running after revoke: PID %d", jobPID)
		}
	}
	if status, err := client.Status(ctx); err == nil || status.Available {
		t.Fatal("revoked worker still available")
	}
}

func TestSnapshotCancellationReachesWorker(t *testing.T) {
	broker, client, _ := startTestBroker(t)
	nonce := strings.Repeat("f", 64)
	if err := broker.ExpectWorker(uint32(os.Getuid()), uint32(os.Getgid()), nonce, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	identity := requireTestWorkerIdentity(t)
	connection, err := net.Dial("unix", broker.options.WorkerSocket)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	connection.SetDeadline(time.Now().Add(3 * time.Second))
	worker := &wire{ReadWriteCloser: connection}
	if err := worker.send(frame{Kind: "worker", Nonce: nonce, Status: &identity}); err != nil {
		t.Fatal(err)
	}
	if accepted, err := worker.receive(); err != nil || accepted.Kind != "accepted" {
		t.Fatalf("worker admission: %v %v", accepted, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() { _, err := client.Snapshot(ctx, true, false); finished <- err }()
	request, err := worker.receive()
	if err != nil || request.Kind != "snapshot" {
		t.Fatalf("snapshot request: %+v %v", request, err)
	}
	cancel()
	if err := <-finished; err == nil {
		t.Fatal("canceled snapshot succeeded")
	}
	request, err = worker.receive()
	if err != nil || request.Kind != "close" {
		t.Fatalf("worker did not receive cancellation: %+v %v", request, err)
	}
}

func TestBrokerRejectsForgedWorkerAndClient(t *testing.T) {
	broker, _, _ := startTestBroker(t)
	nonce := strings.Repeat("d", 64)
	if err := broker.ExpectWorker(uint32(os.Getuid()), uint32(os.Getgid()), nonce, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	identity := requireTestWorkerIdentity(t)
	for _, forged := range []frame{
		{Kind: "worker", Nonce: strings.Repeat("e", 64), Status: &identity},
		{Kind: "worker", Nonce: nonce, Status: &Status{UID: 1, GID: 1, Node: "forged", Username: "forged"}},
	} {
		connection, err := net.Dial("unix", broker.options.WorkerSocket)
		if err != nil {
			t.Fatal(err)
		}
		connection.SetDeadline(time.Now().Add(time.Second))
		transport := &wire{ReadWriteCloser: connection}
		transport.send(forged)
		if _, err := transport.receive(); err == nil {
			t.Fatal("forged worker accepted")
		}
		transport.Close()
	}
	connection, err := net.Dial("unix", broker.options.ClientSocket)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	connection.SetDeadline(time.Now().Add(time.Second))
	transport := &wire{ReadWriteCloser: connection}
	transport.send(frame{Kind: "status", Key: strings.Repeat("e", 64)})
	if _, err := transport.receive(); err == nil {
		t.Fatal("forged key accepted")
	}
}

func TestSocketRefusesSymlinkAndPublicDirectory(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "socket")
	if err := os.Symlink("/tmp/unrelated", path); err != nil {
		t.Fatal(err)
	}
	if listener, err := listenSocket(path); err == nil {
		listener.Close()
		t.Fatal("symlink accepted")
	}
	os.Remove(path)
	os.Chmod(directory, 0777)
	if listener, err := listenSocket(path); err == nil {
		listener.Close()
		t.Fatal("public directory accepted")
	}
}

func TestSocketPermissionDenialDoesNotUnlinkListener(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "socket")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	if replacement, err := listenSocket(path); err == nil {
		replacement.Close()
		t.Fatal("uncertain listener replaced")
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("permission-denied listener was unlinked")
	}
}

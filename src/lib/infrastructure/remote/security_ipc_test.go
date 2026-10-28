//go:build linux || darwin

package remote

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func startTestSecurityIPC(t *testing.T, broker *Broker) (string, *securityFixturePlatform, func()) {
	t.Helper()
	directory, err := os.MkdirTemp("", "dp-security-")
	if err != nil {
		t.Fatal(err)
	}
	directory, err = filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	_, platform := testSecurityAuthority(t, broker)
	platform.directory = directory
	stop, err := broker.startOperatorSecurityAt(context.Background(), platform, directory)
	if err != nil {
		os.RemoveAll(directory)
		t.Fatal(err)
	}
	t.Cleanup(func() { stop(); os.RemoveAll(directory) })
	return directory, platform, stop
}

func TestSecurityIPCSharesLockAcrossClientsAndPreservesAdmittedWork(t *testing.T) {
	_, broker, admin, server := testBroker(t)
	token, _ := enroll(t, admin, "worker")
	directory, _, _ := startTestSecurityIPC(t, broker)
	ctx := context.Background()
	status, err := callSecurityAt(ctx, directory, SecurityRequest{Action: "status"})
	if err != nil || status.Status == nil || !status.Status.Locked {
		t.Fatalf("authority did not start locked: %v", err)
	}
	_, err = callSecurityAt(ctx, directory, SecurityRequest{Action: "sign", Audience: broker.state.Identity.PublicKey,
		Path: "/v1/nodes", Digest: bodyDigest([]byte("null"))})
	if !errors.Is(err, ErrOperatorLocked) {
		t.Fatalf("IPC did not preserve the typed lock denial: %v", err)
	}
	if _, err := callSecurityAt(ctx, directory, SecurityRequest{Action: "unlock"}); err != nil {
		t.Fatal(err)
	}
	proofProvider := func(ctx context.Context, path string, body []byte) (SecurityProof, error) {
		response, err := callSecurityAt(ctx, directory, SecurityRequest{Action: "sign", Audience: broker.state.Identity.PublicKey, Path: path, Digest: bodyDigest(body)})
		if err != nil {
			return SecurityProof{}, err
		}
		return *response.Proof, nil
	}
	admin.OperatorProof = proofProvider
	second, _ := NewClient(server.URL, admin.Token)
	defer second.HTTP.CloseIdleConnections()
	second.OperatorProof = proofProvider
	if err := second.Call(ctx, "/v1/nodes", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := admin.Call(ctx, "/v1/submit", contract.Submission{ID: "admitted", Node: "worker", Profile: "test"}, nil); err != nil {
		t.Fatal(err)
	}
	worker, _ := NewClient(server.URL, token)
	defer worker.HTTP.CloseIdleConnections()
	session, _ := Secret()
	var job contract.Job
	if err := worker.Call(ctx, "/v1/next", map[string]string{"session": session}, &job); err != nil {
		t.Fatal(err)
	}
	if _, err := callSecurityAt(ctx, directory, SecurityRequest{Action: "lock"}); err != nil {
		t.Fatal(err)
	}
	for _, client := range []*Client{admin, second} {
		if err := client.Call(ctx, "/v1/nodes", nil, nil); err == nil {
			t.Fatal("one client survived the shared lock")
		}
	}
	result := contract.Result{Status: "success", ExitCode: 0}
	if err := worker.Call(ctx, "/v1/result", map[string]any{"id": job.ID, "session": session, "result": result}, nil); err != nil {
		t.Fatalf("locking operator access interrupted admitted work: %v", err)
	}
}

func TestSecurityIPCShutdownClosesSilentClientsAndRejectsSecondAuthority(t *testing.T) {
	_, broker, _, _ := testBroker(t)
	directory, platform, stop := startTestSecurityIPC(t, broker)
	if _, err := broker.startOperatorSecurityAt(context.Background(), platform, directory); err == nil {
		t.Fatal("second authority bypassed global lock ownership")
	}
	connection, err := net.Dial("unix", filepath.Join(directory, "authority.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	done := make(chan struct{})
	go func() { stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("silent IPC client delayed authority shutdown")
	}
}

func TestSecurityIPCDisconnectCancelsOSAuthentication(t *testing.T) {
	_, broker, _, _ := testBroker(t)
	directory, platform, _ := startTestSecurityIPC(t, broker)
	started, cancelled := make(chan struct{}), make(chan struct{})
	platform.mutex.Lock()
	platform.authenticate = func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return ctx.Err()
	}
	platform.mutex.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := callSecurityAt(ctx, directory, SecurityRequest{Action: "unlock"}); done <- err }()
	<-started
	cancel()
	if err := <-done; err == nil {
		t.Fatal("cancelled client returned success")
	}
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("disconnected client left OS authentication running")
	}
	if !broker.security.Status().Locked {
		t.Fatal("disconnected authentication unlocked remote access")
	}
}

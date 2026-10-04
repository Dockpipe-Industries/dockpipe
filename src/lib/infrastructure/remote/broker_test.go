package remote

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func testBroker(t *testing.T) (string, *Broker, *Client, *httptest.Server) {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	admin, err := Secret()
	if err != nil {
		t.Fatal(err)
	}
	state := BrokerState{Schema: contract.Version, AdminHash: Hash(admin), Nodes: map[string]Enrollment{}, Jobs: map[string]contract.Job{}}
	if err := WritePrivate(filepath.Join(root, "broker.json"), state); err != nil {
		t.Fatal(err)
	}
	broker, err := NewBroker(root)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(broker)
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, admin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.HTTP.CloseIdleConnections)
	return root, broker, client, server
}

func enroll(t *testing.T, admin *Client, node string) (string, contract.Invitation) {
	t.Helper()
	var invitation contract.Invitation
	if err := admin.Call(context.Background(), "/v1/invite", map[string]string{"node": node}, &invitation); err != nil {
		t.Fatal(err)
	}
	token, err := Secret()
	if err != nil {
		t.Fatal(err)
	}
	request := map[string]string{"node": node, "secret": invitation.Secret, "token": token}
	if err := admin.Call(context.Background(), "/v1/pair", request, nil); err != nil {
		t.Fatal(err)
	}
	if err := admin.Call(context.Background(), "/v1/pair", request, nil); err != nil {
		t.Fatalf("lost-reply retry: %v", err)
	}
	return token, invitation
}

func TestBrokerIdentityIdempotenceAndRecovery(t *testing.T) {
	root, broker, admin, server := testBroker(t)
	token, invitation := enroll(t, admin, "mac")
	worker, _ := NewClient(server.URL, token)
	defer worker.HTTP.CloseIdleConnections()
	ctx := context.Background()
	otherToken, _ := Secret()
	if err := admin.Call(ctx, "/v1/pair", map[string]string{"node": "mac", "secret": invitation.Secret, "token": otherToken}, nil); err == nil {
		t.Fatal("consumed invitation minted a new token")
	}
	if err := worker.Call(ctx, "/v1/submit", contract.Submission{ID: "unauthorized", Node: "mac", Profile: "bench"}, nil); err == nil {
		t.Fatal("worker submitted work")
	}
	request := contract.Submission{ID: "benchmark-1", Node: "mac", Profile: "bench"}
	for range 2 {
		if err := admin.Call(ctx, "/v1/submit", request, nil); err != nil {
			t.Fatal(err)
		}
	}
	changed := request
	changed.Profile = "other"
	if err := admin.Call(ctx, "/v1/submit", changed, nil); err == nil {
		t.Fatal("accepted different work under same ID")
	}
	var job contract.Job
	session, _ := Secret()
	if err := worker.Call(ctx, "/v1/next", map[string]string{"session": session}, &job); err != nil || job.ID != request.ID {
		t.Fatalf("claim: %v %+v", err, job)
	}
	otherSession, _ := Secret()
	var concurrentClaim contract.Job
	if err := worker.Call(ctx, "/v1/next", map[string]string{"session": otherSession}, &concurrentClaim); err != nil || concurrentClaim.Session != session {
		t.Fatalf("second session took ownership: %v %+v", err, concurrentClaim)
	}
	server.Close()
	reopened, err := NewBroker(root)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.state.Jobs[job.ID].Status != "unknown" {
		t.Fatal("restart requeued claimed work")
	}
	result := contract.Result{Status: "success", ExitCode: 0, Artifacts: map[string][]byte{"results/result.json": []byte(`{"ok":true}`)}}
	if _, err := reopened.worker("mac", "/v1/result", marshal(t, map[string]any{"id": job.ID, "session": session, "result": result})); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.worker("mac", "/v1/result", marshal(t, map[string]any{"id": job.ID, "session": session, "result": result})); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.worker("other", "/v1/result", marshal(t, map[string]any{"id": job.ID, "session": session, "result": result})); err == nil {
		t.Fatal("cross-node result accepted")
	}
	result.Log = "changed"
	if _, err := reopened.worker("mac", "/v1/result", marshal(t, map[string]any{"id": job.ID, "session": session, "result": result})); err == nil {
		t.Fatal("conflicting result accepted")
	}
	if len(broker.state.Jobs) != 1 {
		t.Fatal("duplicate job")
	}
}

func marshal(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestWorkerRunsOnceAndReturnsArtifacts(t *testing.T) {
	_, _, admin, server := testBroker(t)
	token, _ := enroll(t, admin, "mac")
	checkout := t.TempDir()
	executable := filepath.Join(t.TempDir(), "dockpipe")
	// A real child process exercises argv, cwd, output capture, artifact collection,
	// and the network broker. The separate CLI smoke runs the actual DockPipe CLI.
	script := "#!/bin/sh\n[ \"$1\" = --workdir ] || exit 20\n[ \"$3\" = --workflow ] || exit 21\nprintf x >> count\nmkdir -p results\nprintf '{\"score\":42}' > results/result.json\nprintf benchmark-complete\n"
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	config := contract.WorkerConfig{Schema: contract.Version, Endpoint: server.URL, Node: "mac", Token: token, Profiles: map[string]contract.Profile{"bench": {Workdir: checkout, Workflow: "bench", TimeoutSeconds: 30, Artifacts: []string{"results/result.json"}}}}
	request := contract.Submission{ID: "benchmark-2", Node: "mac", Profile: "bench"}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := admin.Call(ctx, "/v1/submit", request, nil); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	workerState := filepath.Join(t.TempDir(), "worker")
	go func() { done <- RunWorker(ctx, workerState, executable, config) }()
	var job contract.Job
	for ctx.Err() == nil {
		if err := admin.Call(ctx, "/v1/job", map[string]string{"id": request.ID}, &job); err != nil {
			t.Fatal(err)
		}
		if job.Result != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job.Result == nil || job.Status != "success" || job.Result.Log != "benchmark-complete" || string(job.Result.Artifacts["results/result.json"]) != `{"score":42}` {
		t.Fatalf("bad result: %+v", job)
	}
	if err := admin.Call(ctx, "/v1/submit", request, nil); err != nil {
		t.Fatal(err)
	}
	cancel()
	<-done
	data, err := os.ReadFile(filepath.Join(checkout, "count"))
	if err != nil || string(data) != "x" {
		t.Fatalf("execution count: %q %v", data, err)
	}
}

func TestBrokerRejectsRevocationExpiredPairingAndUnsafeArtifacts(t *testing.T) {
	_, broker, admin, server := testBroker(t)
	token, _ := enroll(t, admin, "mac")
	worker, _ := NewClient(server.URL, token)
	defer worker.HTTP.CloseIdleConnections()
	if err := admin.Call(context.Background(), "/v1/revoke", map[string]string{"id": "mac"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := worker.Call(context.Background(), "/v1/next", struct{}{}, nil); err == nil {
		t.Fatal("revoked node could poll")
	}
	secret, _ := Secret()
	broker.state.Nodes["expired"] = Enrollment{SecretHash: Hash(secret), ExpiresAt: time.Now().Add(-time.Minute)}
	if _, err := broker.pair(marshal(t, map[string]string{"node": "expired", "secret": secret, "token": token})); err == nil {
		t.Fatal("expired invitation accepted")
	}
	for _, path := range []string{"../secret", "/etc/passwd", "a/../../secret", "a\\b", "a/../b"} {
		if err := (contract.Result{Status: "success", Artifacts: map[string][]byte{path: []byte("bad")}}).Validate(); err == nil {
			t.Fatalf("unsafe path %q", path)
		}
	}
}

func TestExecutorCancellationAndArtifactLinks(t *testing.T) {
	checkout := t.TempDir()
	executable := filepath.Join(t.TempDir(), "dockpipe")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nsleep 30 &\nwait\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	result := Execute(ctx, executable, contract.Profile{Workdir: checkout, Workflow: "bench", TimeoutSeconds: 60})
	if result.Status != "failure" || !strings.Contains(result.Log, "deadline exceeded") || time.Since(started) > 3*time.Second {
		t.Fatalf("deadline failure not reported: %+v", result)
	}
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(checkout, "result")); err != nil {
		t.Fatal(err)
	}
	if _, err := collectArtifacts(contract.Profile{Workdir: checkout, Artifacts: []string{"result"}}); err == nil {
		t.Fatal("collected symlink")
	}
}

func TestClientRejectsUnsafeEndpointsAndBoundsLogs(t *testing.T) {
	for _, endpoint := range []string{"http://example.com", "http://localhost:80", "https://user:password@example.com", "https://example.com/path", "https://example.com?token=x"} {
		if _, err := NewClient(endpoint, "secret"); err == nil {
			t.Fatalf("accepted %s", endpoint)
		}
	}
	if err := Decode([]byte(`{} {}`), &struct{}{}); err == nil {
		t.Fatal("trailing JSON accepted")
	}
	log := &boundedLog{}
	_, _ = log.Write([]byte(strings.Repeat("x", contract.MaxLog+10)))
	if len(log.data) != contract.MaxLog || !log.truncated {
		t.Fatal("log bounds failed")
	}
}

func TestWorkerRecoversResultButRefusesAnotherSession(t *testing.T) {
	for _, recoverResult := range []bool{false, true} {
		name := "foreign-session"
		if recoverResult {
			name = "recover-result"
		}
		t.Run(name, func(t *testing.T) {
			_, _, admin, server := testBroker(t)
			token, _ := enroll(t, admin, "mac")
			workerClient, _ := NewClient(server.URL, token)
			defer workerClient.HTTP.CloseIdleConnections()
			request := contract.Submission{ID: "recovery", Node: "mac", Profile: "bench"}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := admin.Call(ctx, "/v1/submit", request, nil); err != nil {
				t.Fatal(err)
			}
			oldSession, _ := Secret()
			var job contract.Job
			if err := workerClient.Call(ctx, "/v1/next", map[string]string{"session": oldSession}, &job); err != nil {
				t.Fatal(err)
			}
			workerState := filepath.Join(t.TempDir(), "worker")
			if recoverResult {
				journal := WorkerJournal{Job: request, Session: oldSession, Result: &contract.Result{Status: "success", ExitCode: 0, Log: "already executed"}}
				if err := WritePrivate(filepath.Join(workerState, "jobs", request.ID+".json"), journal); err != nil {
					t.Fatal(err)
				}
			}
			config := contract.WorkerConfig{Schema: contract.Version, Endpoint: server.URL, Node: "mac", Token: token, Profiles: map[string]contract.Profile{"bench": {Workdir: t.TempDir(), Workflow: "bench", TimeoutSeconds: 10}}}
			done := make(chan error, 1)
			go func() { done <- RunWorker(ctx, workerState, "/must/not/execute", config) }()
			if !recoverResult {
				if err := <-done; err == nil || !strings.Contains(err.Error(), "another worker session") {
					t.Fatalf("foreign session accepted: %v", err)
				}
				return
			}
			for ctx.Err() == nil {
				if err := admin.Call(ctx, "/v1/job", map[string]string{"id": request.ID}, &job); err != nil {
					t.Fatal(err)
				}
				if job.Result != nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			cancel()
			<-done
			if job.Result == nil || job.Result.Log != "already executed" || job.Status != "success" {
				t.Fatalf("journal not reconciled: %+v", job)
			}
		})
	}
}

func TestWorkerReturnsRevokedIdentityInsteadOfPollingForever(t *testing.T) {
	_, _, admin, server := testBroker(t)
	token, _ := enroll(t, admin, "revoked")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := admin.Call(ctx, "/v1/revoke", map[string]string{"id": "revoked"}, nil); err != nil {
		t.Fatal(err)
	}
	config := contract.WorkerConfig{Schema: contract.Version, Endpoint: server.URL, Node: "revoked", Token: token, Profiles: map[string]contract.Profile{"bench": {Workdir: t.TempDir(), Workflow: "bench", TimeoutSeconds: 10}}}
	err := RunWorker(ctx, filepath.Join(t.TempDir(), "worker"), "/must/not/execute", config)
	var rejected *HTTPError
	if !errors.As(err, &rejected) || rejected.StatusCode != 401 {
		t.Fatalf("revocation did not stop worker: %v", err)
	}
}

func TestHeartbeatFailureDiffersFromOperatorCancellation(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "dockpipe")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nsleep 30 &\nwait\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, lost := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if lost {
				http.Error(w, "unavailable", 503)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"cancel":true}`))
		}))
		client, err := NewClient(server.URL, strings.Repeat("a", 64))
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		result := executeWithHeartbeat(ctx, client, executable, contract.Profile{Workdir: t.TempDir(), Workflow: "bench", TimeoutSeconds: 30}, "job", strings.Repeat("b", 64))
		cancel()
		client.HTTP.CloseIdleConnections()
		server.Close()
		expected := "cancelled"
		if lost {
			expected = "failure"
		}
		if result.Status != expected {
			t.Fatalf("lost=%v: status=%s log=%s", lost, result.Status, result.Log)
		}
		if lost && !strings.Contains(result.Log, "heartbeat failed") {
			t.Fatalf("lost heartbeat cause absent: %s", result.Log)
		}
	}
}

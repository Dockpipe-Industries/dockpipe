//go:build linux || darwin

package remote

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func TestWorkerRunsOnceAndReturnsArtifacts(t *testing.T) {
	_, _, admin, server := testBroker(t)
	token, _ := enroll(t, admin, "mac")
	checkout := t.TempDir()
	executable := remoteTestExecutable(t, checkout, "success")
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

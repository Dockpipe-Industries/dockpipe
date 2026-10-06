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

func TestWorkerDeliveryAuthorityAndJournalRecovery(t *testing.T) {
	for _, scenario := range []string{"disabled", "interrupted", "saved-result"} {
		t.Run(scenario, func(t *testing.T) {
			_, _, admin, server := testBroker(t)
			token, _ := enroll(t, admin, "delivery")
			bundle := contract.Bundle{WorkflowFile: "config.yml", Files: []contract.BundleFile{{Path: "config.yml", Data: []byte("name: delivery\nsteps: []\n")}}}
			digest, _ := bundle.Digest()
			request := contract.SubmitRequest{Submission: contract.Submission{ID: "delivery", Node: "delivery", BundleHash: digest}, Bundle: &bundle}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := admin.Call(ctx, "/v1/submit", request, nil); err != nil {
				t.Fatal(err)
			}
			state := filepath.Join(t.TempDir(), "worker")
			config := contract.WorkerConfig{Schema: contract.Version, Endpoint: server.URL, Node: "delivery", Token: token, Profiles: map[string]contract.Profile{"local": {Workdir: t.TempDir(), Workflow: "local", TimeoutSeconds: 30}}}
			if scenario != "disabled" {
				config.Delivery = &contract.DeliveryPermission{TimeoutSeconds: 30}
				worker, _ := NewClient(server.URL, token)
				defer worker.HTTP.CloseIdleConnections()
				var assigned contract.Job
				session := strings.Repeat("b", 64)
				if err := worker.Call(ctx, "/v1/next", map[string]string{"session": session}, &assigned); err != nil {
					t.Fatal(err)
				}
				journal := WorkerJournal{Job: request.Submission, Session: session}
				if scenario == "saved-result" {
					journal.Result = &contract.Result{Status: "success", ExitCode: 0, Log: "completed before disconnect"}
				}
				if err := WritePrivate(filepath.Join(state, "jobs", request.ID+".json"), journal); err != nil {
					t.Fatal(err)
				}
			}
			done := make(chan error, 1)
			go func() { done <- RunWorker(ctx, state, "/must/not/execute", config) }()
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
			cancel()
			<-done
			want := map[string]string{"disabled": "failure", "interrupted": "unknown", "saved-result": "success"}[scenario]
			if job.Status != want || job.Result == nil {
				t.Fatalf("got %+v, want %s", job, want)
			}
			if scenario == "disabled" && !strings.Contains(job.Result.Log, "disabled") {
				t.Fatal("missing authority diagnostic")
			}
			if _, err := os.Stat(filepath.Join(state, "deliveries")); !os.IsNotExist(err) {
				t.Fatal("unapproved or recovered delivery was staged again")
			}
		})
	}
}

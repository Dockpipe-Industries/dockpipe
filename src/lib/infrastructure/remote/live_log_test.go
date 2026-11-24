package remote

import (
	"context"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func TestWorkflowLiveLogRequiresAssignmentAndClearsOnCompletion(t *testing.T) {
	broker, operator, worker, config, _, _, _ := featureFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	const id = "live-log"
	session := strings.Repeat("a", 64)
	request := map[string]string{"id": id}
	if err := operator.Call(ctx, "/v1/submit", contract.Submission{ID: id, Node: config.Node, Profile: "bench"}, nil); err != nil {
		t.Fatal(err)
	}
	update := contract.LiveLogUpdate{ID: id, Session: session, Log: contract.LiveLog{Data: []byte("workflow-running\n")}}
	if err := worker.Call(ctx, "/v1/job-log-publish", update, nil); err == nil {
		t.Fatal("queued job accepted a live log")
	}
	if err := worker.Call(ctx, "/v1/next", map[string]string{"session": session}, nil); err != nil {
		t.Fatal(err)
	}
	wrongSession := update
	wrongSession.Session = strings.Repeat("b", 64)
	if err := worker.Call(ctx, "/v1/job-log-publish", wrongSession, nil); err == nil {
		t.Fatal("another worker session published to the active job")
	}
	oversized := update
	oversized.Log.Data = make([]byte, contract.MaxLiveLog+1)
	if err := worker.Call(ctx, "/v1/job-log-publish", oversized, nil); err == nil {
		t.Fatal("oversized live log accepted")
	}

	checkout := t.TempDir()
	executable := remoteTestExecutable(t, checkout, "live-log")
	profile := contract.Profile{Workdir: checkout, Workflow: "bench", TimeoutSeconds: 30}
	done := make(chan contract.Result, 1)
	go func() {
		defer close(done)
		done <- executeWithHeartbeat(ctx, worker, executable, profile, id, session)
	}()
	defer func() {
		cancel()
		<-done
	}()

	for {
		var preview contract.LiveLogView
		if err := operator.Call(ctx, "/v1/job-log", request, &preview); err != nil {
			t.Fatal(err)
		}
		if preview.ID == id && preview.Status == "running" && string(preview.Log.Data) == "workflow-running\n" {
			break
		}
		if !pause(ctx, 50*time.Millisecond) {
			t.Fatal("child output was not visible while the workflow was running")
		}
	}

	if err := worker.Call(ctx, "/v1/job-log", request, nil); err == nil {
		t.Fatal("worker could read the operator log view")
	}
	mutated := update
	mutated.Log.Data = []byte("changed")
	if err := worker.Call(ctx, "/v1/job-log-publish", mutated, nil); err == nil {
		t.Fatal("live log accepted a conflicting prefix")
	}
	if err := operator.Call(ctx, "/v1/cancel", request, nil); err != nil {
		t.Fatal(err)
	}
	result := <-done
	if result.Status != "cancelled" || !strings.Contains(result.Log, "workflow-running") {
		t.Fatalf("unexpected terminal result: %+v", result)
	}
	if err := worker.Call(ctx, "/v1/result", map[string]any{"id": id, "session": session, "result": result}, nil); err != nil {
		t.Fatal(err)
	}
	var preview contract.LiveLogView
	if err := operator.Call(ctx, "/v1/job-log", request, &preview); err != nil || preview.Status != "cancelled" || len(preview.Log.Data) != 0 {
		t.Fatalf("terminal preview should contain status only: %+v, %v", preview, err)
	}
	broker.mutex.Lock()
	retained := len(broker.liveLogs)
	broker.mutex.Unlock()
	if retained != 0 {
		t.Fatal("broker retained a finished live log")
	}
}

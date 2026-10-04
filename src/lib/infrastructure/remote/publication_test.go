package remote

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
)

func TestResultMetadataFailurePoisonsBrokerAndRetainsRecoveryPayload(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	job := contract.Job{
		Submission: contract.Submission{ID: "job", Node: "mini", Profile: "bench"},
		Status:     "running", Session: strings.Repeat("b", 64),
	}
	admin := strings.Repeat("a", 64)
	broker := &Broker{
		path: filepath.Join(root, "broker.json"),
		state: BrokerState{
			Schema: contract.Version, AdminHash: Hash(admin),
			Nodes: map[string]Enrollment{}, Jobs: contract.Queue{job.ID: job},
		},
	}
	// Block metadata publication after the result payload can be written.
	if err := os.Mkdir(broker.path, 0o700); err != nil {
		t.Fatal(err)
	}
	result := contract.Result{Status: "success", Log: "completed once"}
	raw := marshal(t, map[string]any{"id": job.ID, "session": job.Session, "result": result})
	if _, err := broker.worker("mini", "/v1/result", raw); err == nil {
		t.Fatal("acknowledged result without durable metadata")
	}
	if !broker.poisoned {
		t.Fatal("broker continued after uncertain publication")
	}
	var stored contract.Result
	if err := ReadPrivate(filepath.Join(root, "results", "job.json"), &stored); err != nil || stored.Log != result.Log {
		t.Fatalf("recovery payload lost: %+v, %v", stored, err)
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/health", strings.NewReader("{}"))
	request.Header.Set("Authorization", "Bearer "+admin)
	response := httptest.NewRecorder()
	broker.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("uncertain state served another request: %d", response.Code)
	}
}

func TestRejectedResultDoesNotPublishOrPoison(t *testing.T) {
	root := t.TempDir()
	job := contract.Job{
		Submission: contract.Submission{ID: "job", Node: "mini", Profile: "bench"},
		Status:     "running", Session: strings.Repeat("b", 64),
	}
	broker := &Broker{path: filepath.Join(root, "broker.json"), state: BrokerState{Jobs: contract.Queue{job.ID: job}}}
	raw := marshal(t, map[string]any{
		"id": job.ID, "session": job.Session,
		"result": contract.Result{Status: "success", ExitCode: 1},
	})
	if _, err := broker.worker("mini", "/v1/result", raw); err == nil {
		t.Fatal("accepted contradictory result")
	}
	if broker.poisoned || broker.state.Jobs[job.ID].Status != "running" || broker.state.Jobs[job.ID].ResultHash != "" {
		t.Fatal("validation rejection changed durable state")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("rejection published files: %v, %v", entries, err)
	}
}

func TestWritePrivatePreservesOriginalOnSerializationOrTargetFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "state.json")
	if err := WritePrivate(path, map[string]string{"value": "original"}); err != nil {
		t.Fatal(err)
	}
	if err := WritePrivate(path, make(chan int)); err == nil {
		t.Fatal("accepted unserializable state")
	}
	var restored map[string]string
	if err := ReadPrivate(path, &restored); err != nil || restored["value"] != "original" {
		t.Fatalf("failed write changed original: %v, %v", restored, err)
	}
	if err := WritePrivate(root, map[string]string{"value": "invalid"}); err == nil {
		t.Fatal("replaced directory with state file")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "state.json" {
		t.Fatalf("failed write left temporary files: %v, %v", entries, err)
	}
}

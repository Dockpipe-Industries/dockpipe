package remote

import (
	contract "dockpipe/src/lib/domain/remote"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaximumResultsDoNotExhaustBrokerMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("a", 64)
	broker := &Broker{path: filepath.Join(root, "broker.json"), state: BrokerState{
		Schema: contract.Version, AdminHash: Hash(token),
		Nodes: map[string]Enrollment{"node": {TokenHash: Hash(token)}}, Jobs: map[string]contract.Job{},
	}}
	session := strings.Repeat("b", 64)
	payload := make([]byte, contract.MaxArtifacts)
	for index := 0; index < 3; index++ {
		id := fmt.Sprintf("job-%d", index)
		submission := contract.Submission{ID: id, Node: "node", Profile: "bench"}
		raw, _ := json.Marshal(submission)
		if _, err := broker.operator("/v1/submit", raw); err != nil {
			t.Fatal(err)
		}
		raw, _ = json.Marshal(map[string]string{"session": session})
		if _, err := broker.worker("node", "/v1/next", raw); err != nil {
			t.Fatal(err)
		}
		result := contract.Result{Status: "success", ExitCode: 0, Log: strings.Repeat("\x00", contract.MaxLog), Artifacts: map[string][]byte{"result.bin": payload}}
		if err := result.Validate(); err != nil {
			t.Fatal(err)
		}
		raw, _ = json.Marshal(map[string]any{"id": id, "session": session, "result": result})
		if len(raw) > contract.MaxBody {
			t.Fatal("test exceeds request limit")
		}
		_, err := broker.worker("node", "/v1/result", raw)
		if err != nil {
			t.Fatal(err)
		}
		if broker.poisoned {
			t.Fatal("valid result poisoned broker")
		}
		broker, err = NewBroker(root)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ = json.Marshal(map[string]string{"id": id})
		restored, err := broker.operator("/v1/job", raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(restored.(contract.Job).Result.Artifacts["result.bin"]) != contract.MaxArtifacts {
			t.Fatal("result lost on restart")
		}

	}
	request := httptest.NewRequest("POST", "/v1/health", strings.NewReader("{}"))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	broker.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("health=%d", response.Code)
	}
	info, err := os.Stat(filepath.Join(root, "broker.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > 64<<10 {
		t.Fatalf("payload retained in metadata: %d", info.Size())
	}
}

func TestBrokerMigratesInlineResultsAndChecksStoredDigest(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	job := contract.Job{Submission: contract.Submission{ID: "legacy", Node: "node", Profile: "bench"}, Status: "success", Result: &contract.Result{Status: "success", Log: "original"}}
	state := BrokerState{Schema: contract.Version, AdminHash: Hash(strings.Repeat("a", 64)), Nodes: map[string]Enrollment{}, Jobs: map[string]contract.Job{job.ID: job}}
	if err := WritePrivate(filepath.Join(root, "broker.json"), state); err != nil {
		t.Fatal(err)
	}
	broker, err := NewBroker(root)
	if err != nil {
		t.Fatal(err)
	}
	stored := broker.state.Jobs[job.ID]
	if stored.Result != nil || stored.ResultHash == "" {
		t.Fatal("inline result was not migrated")
	}
	restored, err := broker.withResult(stored)
	if err != nil || restored.Result.Log != "original" {
		t.Fatalf("migrated result unavailable: %v", err)
	}
	if err := WritePrivate(filepath.Join(root, "results", "legacy.json"), contract.Result{Status: "success", Log: "changed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.withResult(stored); err == nil {
		t.Fatal("modified result accepted")
	}
}

func TestJobCapacityRejectsBeforeAssignmentWithoutPoisoning(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	broker := &Broker{path: filepath.Join(root, "broker.json"), state: BrokerState{Schema: contract.Version, AdminHash: Hash(strings.Repeat("a", 64)), Nodes: map[string]Enrollment{"node": {TokenHash: Hash(strings.Repeat("b", 64))}}, Jobs: map[string]contract.Job{}}}
	for index := 0; index < 128; index++ {
		job := contract.Job{Submission: contract.Submission{ID: fmt.Sprintf("job-%d", index), Node: "node", Profile: "bench"}, Status: "success"}
		broker.state.Jobs[job.ID] = job
	}
	if err := broker.save(); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(contract.Submission{ID: "over-capacity", Node: "node", Profile: "bench"})
	if _, err := broker.operator("/v1/submit", raw); err == nil {
		t.Fatal("over-capacity job admitted")
	}
	if broker.poisoned || len(broker.state.Jobs) != 128 {
		t.Fatal("capacity rejection changed broker state")
	}
	if _, err := broker.operator("/v1/health", nil); err != nil {
		t.Fatal(err)
	}
}

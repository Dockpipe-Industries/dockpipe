package remote

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

func archiveTestBroker(t *testing.T) *Broker {
	t.Helper()
	root := privateTestDirectory(t)
	broker := &Broker{path: filepath.Join(root, "broker.json"), state: BrokerState{
		Schema: contract.Version, AdminHash: Hash(strings.Repeat("a", 64)),
		Nodes: map[string]Enrollment{"node": {TokenHash: Hash(strings.Repeat("b", 64))}},
		Jobs:  map[string]contract.Job{},
	}}
	for index := range contract.MaxJobs {
		id := fmt.Sprintf("job-%03d", index)
		broker.state.Jobs[id] = contract.Job{
			Submission: contract.Submission{ID: id, Node: "node", Profile: "bench"},
			Status:     "success", CreatedAt: time.Unix(1, 0).UTC(), Session: strings.Repeat("c", 64),
		}
	}
	if err := broker.save(); err != nil {
		t.Fatal(err)
	}
	return broker
}

func archiveOperator(broker *Broker, operation string, value any) (any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return broker.operator(operation, raw)
}

func archiveSubmit(broker *Broker, id string) (any, error) {
	return archiveOperator(broker, "/v1/submit", contract.Submission{ID: id, Node: "node", Profile: "bench"})
}

func TestTerminalArchiveAdmitsNewWorkAndPreservesReplayAfterRestart(t *testing.T) {
	broker := archiveTestBroker(t)
	oldest := broker.state.Jobs["job-000"]
	result := &contract.Result{Status: "success", Log: "original evidence"}
	if err := broker.storeResult(&oldest, result); err != nil {
		t.Fatal(err)
	}
	broker.state.Jobs[oldest.ID] = oldest
	if err := broker.save(); err != nil {
		t.Fatal(err)
	}
	if _, err := archiveSubmit(broker, "new-work"); err != nil {
		t.Fatal(err)
	}
	if len(broker.state.Jobs) != contract.MaxJobs || !broker.state.TerminalArchive {
		t.Fatal("new admission did not preserve the hot bound and downgrade guard")
	}
	if _, exists := broker.state.Jobs[oldest.ID]; exists {
		t.Fatal("oldest terminal ID was not retired deterministically")
	}
	var err error
	broker, err = NewBroker(filepath.Dir(broker.path))
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"/v1/job", "/v1/cancel", "/v1/submit"} {
		value := any(map[string]string{"id": oldest.ID})
		if operation == "/v1/submit" {
			value = oldest.Submission
		}
		response, err := archiveOperator(broker, operation, value)
		if err != nil || response.(contract.Job).Result.Log != result.Log {
			t.Fatalf("archived %s: %v", operation, err)
		}
	}
	if _, err := archiveOperator(broker, "/v1/submit", contract.SubmitRequest{
		Submission: oldest.Submission, Bundle: &contract.Bundle{},
	}); err == nil {
		t.Fatal("archived retry bypassed delivery payload validation")
	}
	conflict := oldest.Submission
	conflict.Profile = "different"
	if _, err := archiveOperator(broker, "/v1/submit", conflict); err == nil {
		t.Fatal("archived ID accepted different work")
	}
	workerRequest := map[string]any{"id": oldest.ID, "session": oldest.Session, "result": result}
	raw, _ := json.Marshal(workerRequest)
	if _, err := broker.worker("node", "/v1/result", raw); err != nil {
		t.Fatalf("duplicate terminal result: %v", err)
	}
	workerRequest["result"] = &contract.Result{Status: "failure", ExitCode: 1}
	raw, _ = json.Marshal(workerRequest)
	if _, err := broker.worker("node", "/v1/result", raw); err == nil {
		t.Fatal("conflicting archived result accepted")
	}
	workerRequest["session"] = strings.Repeat("d", 64)
	raw, _ = json.Marshal(workerRequest)
	if _, err := broker.worker("node", "/v1/pulse", raw); err == nil {
		t.Fatal("archived assignment accepted another worker session")
	}
	if _, exists := broker.state.Jobs[oldest.ID]; exists || len(broker.state.Jobs) != contract.MaxJobs {
		t.Fatal("archived replay reinserted terminal work")
	}
	listed, err := broker.operator("/v1/jobs", nil)
	if err != nil || len(listed.([]contract.Job)) != contract.MaxJobs {
		t.Fatalf("recent job listing: %v", err)
	}
}

func TestTerminalArchiveNeverRetiresUnresolvedWork(t *testing.T) {
	broker := archiveTestBroker(t)
	for id, job := range broker.state.Jobs {
		job.Status = "queued"
		broker.state.Jobs[id] = job
	}
	unknown := broker.state.Jobs["job-000"]
	unknown.Status = "unknown"
	unknown.ResultHash = strings.Repeat("a", 64)
	broker.state.Jobs[unknown.ID] = unknown
	running := broker.state.Jobs["job-001"]
	running.Status = "running"
	broker.state.Jobs[running.ID] = running
	if _, err := archiveSubmit(broker, "blocked"); err == nil {
		t.Fatal("unresolved work was evicted")
	}
	if broker.poisoned || len(broker.state.Jobs) != contract.MaxJobs {
		t.Fatal("capacity refusal changed healthy state")
	}
	terminal := broker.state.Jobs["job-127"]
	terminal.Status = "cancelled"
	terminal.CancelRequested = true
	terminal.Session = ""
	broker.state.Jobs[terminal.ID] = terminal
	if _, err := archiveSubmit(broker, "admitted"); err != nil {
		t.Fatal(err)
	}
	if _, err := archiveOperator(broker, "/v1/cancel", map[string]string{"id": terminal.ID}); err != nil {
		t.Fatal(err)
	}
	if len(broker.state.Jobs) != contract.MaxJobs {
		t.Fatal("cancelled archive was reinserted")
	}
	for _, id := range []string{unknown.ID, running.ID} {
		if _, exists := broker.state.Jobs[id]; !exists {
			t.Fatal("claimed/unknown work retired despite terminal alternative")
		}
	}
}

func TestTerminalArchiveCrashOverlapAndDowngradeGuard(t *testing.T) {
	broker := archiveTestBroker(t)
	job := broker.state.Jobs["job-000"]
	if err := broker.storeArchivedJob(job); err != nil {
		t.Fatal(err)
	}
	// Simulate a crash before replacing broker.json: both old hot metadata and
	// its complete immutable archive survive. No new ID has been acknowledged.
	restarted, err := NewBroker(filepath.Dir(broker.path))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := archiveSubmit(restarted, job.ID); err != nil {
		t.Fatalf("identical overlap rejected: %v", err)
	}
	if _, err := archiveSubmit(restarted, "next"); err != nil {
		t.Fatal(err)
	}
	// A binary with the original strict schema must refuse migrated metadata.
	type legacyState struct {
		Pairings     map[string]PairingSession `json:"pairings,omitempty"`
		PairingUntil time.Time                 `json:"pairing_until,omitempty"`
		Schema       string                    `json:"schema"`
		AdminHash    string                    `json:"admin_hash"`
		Nodes        map[string]Enrollment     `json:"nodes"`
		Jobs         map[string]contract.Job   `json:"jobs"`
	}
	var legacy legacyState
	if err := ReadPrivate(restarted.path, &legacy); err == nil || !strings.Contains(err.Error(), "terminal_archive") {
		t.Fatalf("old schema accepted migrated broker: %v", err)
	}
	// An inconsistent hot copy may not override its archive on restart or lookup.
	restarted.state.Jobs[job.ID] = job
	conflict := job
	conflict.CancelRequested = true
	restarted.state.Jobs[job.ID] = conflict
	if _, _, err := restarted.lookupJob(job.ID); err == nil {
		t.Fatal("conflicting overlap accepted during lookup")
	}
	if err := restarted.save(); err != nil {
		t.Fatal(err)
	}
	if _, err := NewBroker(filepath.Dir(restarted.path)); err == nil {
		t.Fatal("conflicting overlap accepted on restart")
	}
}

func TestTerminalArchiveDurableFailureNeverAcknowledgesNewWork(t *testing.T) {
	broker := archiveTestBroker(t)
	// Keep the durable old state, then make only its replacement destination
	// unavailable. The archive still publishes before the failed state commit.
	oldState, err := os.ReadFile(broker.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(broker.path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(broker.path, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := archiveSubmit(broker, "not-acknowledged"); err == nil || !broker.poisoned {
		t.Fatalf("uncertain state save was acknowledged: %v", err)
	}
	if err := os.Remove(broker.path); err != nil {
		t.Fatal(err)
	}
	if err := WritePrivate(broker.path, json.RawMessage(oldState)); err != nil {
		t.Fatal(err)
	}
	restoredState, err := os.ReadFile(broker.path)
	if err != nil {
		t.Fatal(err)
	}
	if string(restoredState) != string(oldState) {
		t.Fatal("private fixture restoration changed the durable old state")
	}
	restarted, err := NewBroker(filepath.Dir(broker.path))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := restarted.lookupJob("not-acknowledged"); !os.IsNotExist(err) {
		t.Fatal("failed admission unexpectedly persisted")
	}
	if _, err := archiveSubmit(restarted, "after-restart"); err != nil {
		t.Fatal(err)
	}
}

func TestTerminalArchiveRejectsMalformedRecordsAndUnsafePaths(t *testing.T) {
	for _, scenario := range []string{"corrupt", "oversized", "wrong-id", "active", "wrong-digest", "linked-directory", "linked-file"} {
		t.Run(scenario, func(t *testing.T) {
			broker := archiveTestBroker(t)
			job := broker.state.Jobs["job-000"]
			if err := broker.storeArchivedJob(job); err != nil {
				t.Fatal(err)
			}
			path, err := broker.archivedJobPath(job.ID)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "corrupt":
				err = os.WriteFile(path, []byte("{"), 0600)
			case "oversized":
				err = os.WriteFile(path, []byte(strings.Repeat(" ", maxArchivedJobBytes+1)), 0600)
			case "wrong-id", "active", "wrong-digest":
				var record archivedJob
				if err := ReadPrivate(path, &record); err != nil {
					t.Fatal(err)
				}
				if scenario == "wrong-id" {
					record.Job.ID = "another"
				} else if scenario == "active" {
					record.Job.Status = "unknown"
				} else {
					record.SHA256 = strings.Repeat("0", 64)
				}
				err = WritePrivate(path, record)
			case "linked-directory":
				directory := filepath.Dir(path)
				moved := directory + "-original"
				if err := os.Rename(directory, moved); err != nil {
					t.Fatal(err)
				}
				err = os.Symlink(moved, directory)
			case "linked-file":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				err = os.Symlink(broker.path, path)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := archiveSubmit(broker, job.ID); err == nil {
				t.Fatal("unsafe archive accepted as retry or new work")
			}
			if _, err := archiveSubmit(broker, "new"); err == nil {
				t.Fatal("invalid archive allowed retirement/admission")
			}
			if len(broker.state.Jobs) != contract.MaxJobs {
				t.Fatal("invalid archive changed hot state")
			}
		})
	}
	broker := archiveTestBroker(t)
	for _, id := range []string{"../escape", "a/b", `a\b`, "", strings.Repeat("a", 65)} {
		if _, err := broker.archivedJobPath(id); err == nil {
			t.Fatalf("unsafe archive ID accepted: %q", id)
		}
	}
}

func TestTerminalArchivePublicationIsImmutableAndIgnoresIncompleteTemporaryFiles(t *testing.T) {
	broker := archiveTestBroker(t)
	job := broker.state.Jobs["job-000"]
	directory := filepath.Join(filepath.Dir(broker.path), "terminal-jobs")
	if err := PrivateDirectory(directory); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, ".terminal-job-interrupted"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewBroker(filepath.Dir(broker.path))
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.storeArchivedJob(job); err != nil {
		t.Fatal(err)
	}
	path, err := restarted.archivedJobPath(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.storeArchivedJob(job); err != nil {
		t.Fatal(err)
	}
	changed := job
	changed.CancelRequested = true
	if err := restarted.storeArchivedJob(changed); err == nil {
		t.Fatal("immutable archive was replaced")
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || string(actual) != string(original) {
		t.Fatal("existing archive changed during exact/conflicting publication")
	}
}

func TestUnresolvedCapacityRefusesBeforePersistingNewDelivery(t *testing.T) {
	broker := archiveTestBroker(t)
	for id, job := range broker.state.Jobs {
		job.Status = "queued"
		broker.state.Jobs[id] = job
	}
	before, err := json.Marshal(broker.state.Jobs)
	if err != nil {
		t.Fatal(err)
	}
	bundle := contract.Bundle{WorkflowFile: "config.yml", Files: []contract.BundleFile{
		{Path: "config.yml", Data: []byte("name: unaccepted\nsteps: []\n")},
	}}
	digest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	request := contract.SubmitRequest{
		Submission: contract.Submission{ID: "no-capacity", Node: "node", BundleHash: digest},
		Bundle:     &bundle,
	}
	if _, err := archiveOperator(broker, "/v1/submit", request); err == nil {
		t.Fatal("unresolved capacity admitted delivery")
	}
	path, err := broker.bundlePath(request.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("unaccepted delivery consumed persistent storage: %v", err)
	}
	after, err := json.Marshal(broker.state.Jobs)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) || broker.poisoned {
		t.Fatal("capacity refusal changed hot state")
	}
}

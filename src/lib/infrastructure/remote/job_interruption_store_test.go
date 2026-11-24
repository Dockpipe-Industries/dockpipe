package remote

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

// Model the exact shared job shape before acknowledgements existed. The
// production operator decodes this file and ordinary job responses strictly.
type priorInterruptionJob struct {
	contract.Submission
	Status          string           `json:"status"`
	Session         string           `json:"session,omitempty"`
	CancelRequested bool             `json:"cancel_requested"`
	CreatedAt       time.Time        `json:"created_at"`
	Result          *contract.Result `json:"result,omitempty"`
	ResultHash      string           `json:"result_hash,omitempty"`
}

type priorInterruptionBrokerState struct {
	BrokerState
	Jobs map[string]priorInterruptionJob `json:"jobs"`
}

func TestJobInterruptionMigratesLegacyAcknowledgement(t *testing.T) {
	for _, scenario := range []string{"inline only", "matching crash overlap"} {
		t.Run(scenario, func(t *testing.T) {
			broker, job, token := legacyInterruptionFixture(t)
			if scenario == "matching crash overlap" {
				if err := broker.storeJobInterruptionAcknowledgement(job.ID, token); err != nil {
					t.Fatal(err)
				}
			}

			var prior priorInterruptionBrokerState
			if err := ReadPrivate(broker.path, &prior); err == nil {
				t.Fatal("legacy inline fixture unexpectedly decoded using the prior schema")
			}
			root := filepath.Dir(broker.path)
			for range 2 {
				current, err := NewBroker(root)
				if err != nil {
					t.Fatal(err)
				}
				if err := ReadPrivate(broker.path, &prior); err != nil {
					t.Fatalf("migrated broker state fails strict prior decoding: %v", err)
				}
				stored := prior.Jobs[job.ID]
				if stored.Submission != job.Submission || stored.Session != job.Session || stored.Status != "unknown" || stored.ResultHash != job.ResultHash {
					t.Fatal("migration changed the shared job identity or outcome")
				}
				if err := current.verifyJobInterruptionAcknowledgement(job.ID, token); err != nil {
					t.Fatal(err)
				}
				request := contract.JobInterruptionRequest{ID: job.ID, Node: job.Node, ReviewToken: token}
				review, err := current.jobInterruption(marshal(t, request), true)
				if err != nil || review.Validate(request, true) != nil || review.ResultHash != job.ResultHash {
					t.Fatalf("exact retry after migration changed the reviewed receipt: %v", err)
				}
				if selected, _, err := contract.Queue(current.state.Jobs).Next(job.Node, strings.Repeat("c", 64)); err != nil || selected != nil {
					t.Fatalf("acknowledged job still holds assignment: %v", err)
				}
				broker = current
			}

			// A stale restore must not free this ID for a fresh execution if its
			// immutable acknowledgement still exists without the job metadata.
			delete(broker.state.Jobs, job.ID)
			if err := broker.save(); err != nil {
				t.Fatal(err)
			}
			if _, err := broker.operator("/v1/submit", marshal(t, job.Submission)); err == nil {
				t.Fatal("accepted a fresh submission for an orphaned acknowledgement")
			}
		})
	}
}

func TestJobInterruptionRejectsConflictingMigration(t *testing.T) {
	for _, scenario := range []string{"wrong token", "wrong job", "changed result"} {
		t.Run(scenario, func(t *testing.T) {
			broker, job, token := legacyInterruptionFixture(t)
			path, err := broker.jobInterruptionAcknowledgementPath(job.ID)
			if err != nil {
				t.Fatal(err)
			}
			record := jobInterruptionAcknowledgement{Schema: 1, ID: job.ID, ReviewToken: token}
			switch scenario {
			case "wrong token":
				record.ReviewToken = strings.Repeat("f", 64)
			case "wrong job":
				record.ID = "another-job"
			case "changed result":
				resultPath, err := broker.resultPath(job.ID)
				if err != nil {
					t.Fatal(err)
				}
				if err := WritePrivate(resultPath, contract.Result{Status: "unknown", ExitCode: -1, Log: "changed"}); err != nil {
					t.Fatal(err)
				}
			}
			if err := WritePrivate(path, record); err != nil {
				t.Fatal(err)
			}
			original, err := os.ReadFile(broker.path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := NewBroker(filepath.Dir(broker.path)); err == nil {
				t.Fatal("broker accepted conflicting interruption evidence")
			}
			current, err := os.ReadFile(broker.path)
			if err != nil || !bytes.Equal(current, original) {
				t.Fatalf("migration modified broker metadata despite the conflict: %v", err)
			}
			var retained jobInterruptionAcknowledgement
			if err := ReadPrivate(path, &retained); err != nil || retained != record {
				t.Fatalf("migration replaced a conflicting sidecar: %v", err)
			}
		})
	}
}

func legacyInterruptionFixture(t *testing.T) (*Broker, contract.Job, string) {
	t.Helper()
	root := privateTestDirectory(t)
	path := filepath.Join(root, "broker.json")
	state := BrokerState{
		Schema:    contract.Version,
		AdminHash: Hash(strings.Repeat("a", 64)),
		Nodes:     map[string]Enrollment{"worker": {TokenHash: Hash(strings.Repeat("b", 64))}},
		Jobs:      map[string]contract.Job{},
	}
	if err := WritePrivate(path, state); err != nil {
		t.Fatal(err)
	}
	broker, err := NewBroker(root)
	if err != nil {
		t.Fatal(err)
	}
	job := contract.Job{
		Submission: contract.Submission{ID: "interrupted", Node: "worker", Profile: "fixture"},
		CreatedAt:  time.Now().UTC(),
		Status:     "unknown",
		Session:    strings.Repeat("c", 64),
	}
	result := &contract.Result{Status: "unknown", ExitCode: -1, Log: "retained interruption evidence"}
	if err := broker.storeResult(&job, result); err != nil {
		t.Fatal(err)
	}
	token, err := broker.jobInterruptionToken(job)
	if err != nil {
		t.Fatal(err)
	}
	type inlineJob struct {
		contract.Job
		Token string `json:"interruption_acknowledgement"`
	}
	legacy := struct {
		BrokerState
		Jobs map[string]inlineJob `json:"jobs"`
	}{
		BrokerState: broker.state,
		Jobs:        map[string]inlineJob{job.ID: {Job: job, Token: token}},
	}
	if err := WritePrivate(path, legacy); err != nil {
		t.Fatal(err)
	}
	return broker, job, token
}

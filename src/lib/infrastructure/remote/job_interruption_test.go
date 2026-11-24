package remote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
)

func TestBrokerAcknowledgesOnlyReviewedInterruptedJob(t *testing.T) {
	ctx := context.Background()
	root := privateTestDirectory(t)
	brokerPath := filepath.Join(root, "broker.json")
	adminToken := strings.Repeat("a", 64)
	workerToken := strings.Repeat("b", 64)
	originalSession := strings.Repeat("c", 64)
	newSession := strings.Repeat("d", 64)
	state := BrokerState{
		Schema:    contract.Version,
		AdminHash: Hash(adminToken),
		Nodes:     map[string]Enrollment{"worker": {TokenHash: Hash(workerToken)}},
		Jobs:      map[string]contract.Job{},
	}
	if err := WritePrivate(brokerPath, state); err != nil {
		t.Fatal(err)
	}
	broker, err := NewBroker(root)
	if err != nil {
		t.Fatal(err)
	}

	client := func(broker *Broker, token string) *Client {
		t.Helper()
		client, err := NewClient("http://127.0.0.1:47831", token)
		if err != nil {
			t.Fatal(err)
		}
		client.HTTP.Transport = machineTestTransport(func(request *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			broker.ServeHTTP(recorder, request)
			return recorder.Result(), nil
		})
		return client
	}
	operator := func(broker *Broker) *Client {
		t.Helper()
		authority, _ := testSecurityAuthority(t, broker)
		admin := client(broker, adminToken)
		admin.OperatorProof = func(ctx context.Context, path string, body []byte) (SecurityProof, error) {
			return authority.Sign(ctx, broker.state.Identity.PublicKey, path, bodyDigest(body))
		}
		return admin
	}
	admin := operator(broker)
	worker := client(broker, workerToken)
	submit := func(id string) contract.Submission {
		t.Helper()
		request := contract.Submission{ID: id, Node: "worker", Profile: "fixture"}
		if err := admin.Call(ctx, "/v1/submit", request, nil); err != nil {
			t.Fatal(err)
		}
		return request
	}
	claim := func(session string) contract.Job {
		t.Helper()
		var job contract.Job
		if err := worker.Call(ctx, "/v1/next", map[string]string{"session": session}, &job); err != nil {
			t.Fatal(err)
		}
		return job
	}

	submission := submit("interrupted")
	if job := claim(originalSession); job.ID != submission.ID || job.Session != originalSession {
		t.Fatal("fixture assignment was not claimed")
	}
	unknown := contract.Result{Status: "unknown", ExitCode: -1, Log: "retained interruption evidence"}
	delivery := map[string]any{"id": submission.ID, "session": originalSession, "result": unknown}
	if err := worker.Call(ctx, "/v1/result", delivery, nil); err != nil {
		t.Fatal(err)
	}
	submit("queued")
	if job := claim(newSession); job.ID != submission.ID || job.Status != "unknown" || job.Result == nil {
		t.Fatal("unacknowledged result no longer holds the node")
	}

	request := contract.JobInterruptionRequest{ID: submission.ID, Node: submission.Node}
	var inspected contract.JobInterruptionReview
	if err := admin.Call(ctx, contract.JobInterruptionInspectPath, request, &inspected); err != nil {
		t.Fatal(err)
	}
	if err := inspected.Validate(request, false); err != nil || inspected.Acknowledged {
		t.Fatalf("invalid inspection: %v", err)
	}
	if inspected.ResultHash != broker.state.Jobs[submission.ID].ResultHash {
		t.Fatal("inspection failed to bind the retained result")
	}
	stored, err := os.ReadFile(brokerPath)
	if err != nil || strings.Contains(string(stored), "interruption_acknowledgement") {
		t.Fatalf("inspection changed the pre-acknowledgement schema: %v", err)
	}

	request.ReviewToken = inspected.ReviewToken
	if err := client(broker, adminToken).Call(ctx, contract.JobInterruptionAcknowledgePath, request, nil); err == nil {
		t.Fatal("administrator bearer bypassed OS proof")
	}
	if err := worker.Call(ctx, contract.JobInterruptionAcknowledgePath, request, nil); err == nil {
		t.Fatal("worker acknowledged its own interruption")
	}

	// A second unknown assignment stands for an independent recovery hold. It
	// has no retained result, so it cannot be cleared by this review.
	other := contract.Job{
		Submission: contract.Submission{ID: "other", Node: "worker", Profile: "fixture"},
		Status:     "unknown",
		Session:    strings.Repeat("e", 64),
	}
	broker.state.Jobs[other.ID] = other
	if err := broker.save(); err != nil {
		t.Fatal(err)
	}
	for _, mismatch := range []contract.JobInterruptionRequest{
		{ID: request.ID, Node: "other", ReviewToken: request.ReviewToken},
		{ID: other.ID, Node: request.Node, ReviewToken: request.ReviewToken},
		{ID: request.ID, Node: request.Node, ReviewToken: strings.Repeat("f", 64)},
	} {
		if err := admin.Call(ctx, contract.JobInterruptionAcknowledgePath, mismatch, nil); err == nil {
			t.Fatal("changed or unproven interruption was acknowledged")
		}
	}
	var acknowledged contract.JobInterruptionReview
	if err := admin.Call(ctx, contract.JobInterruptionAcknowledgePath, request, &acknowledged); err != nil {
		t.Fatal(err)
	}
	inspected.Acknowledged = true
	if acknowledged != inspected {
		t.Fatal("acknowledgement changed the reviewed outcome")
	}
	if err := broker.verifyJobInterruptionAcknowledgement(request.ID, request.ReviewToken); err != nil {
		t.Fatalf("fresh acknowledgement has no verified sidecar: %v", err)
	}
	var sharedState priorInterruptionBrokerState
	if err := ReadPrivate(brokerPath, &sharedState); err != nil {
		t.Fatalf("fresh acknowledgement broke older local clients: %v", err)
	}
	var jobs []priorInterruptionJob
	if err := admin.Call(ctx, "/v1/jobs", nil, &jobs); err != nil {
		t.Fatalf("fresh acknowledgement broke older operator responses: %v", err)
	}
	if job := claim(newSession); job.ID != other.ID || job.Status != "unknown" || job.Result != nil {
		t.Fatal("acknowledgement cleared an unrelated assignment hold")
	}

	broker, err = NewBroker(root)
	if err != nil {
		t.Fatal(err)
	}
	admin = operator(broker)
	worker = client(broker, workerToken)
	var retried contract.JobInterruptionReview
	if err := admin.Call(ctx, contract.JobInterruptionAcknowledgePath, request, &retried); err != nil || retried != acknowledged {
		t.Fatalf("exact retry after restart changed the acknowledgement: %v", err)
	}
	if err := worker.Call(ctx, "/v1/result", delivery, nil); err != nil {
		t.Fatalf("original worker result replay failed: %v", err)
	}
	delivery["result"] = contract.Result{Status: "success"}
	if err := worker.Call(ctx, "/v1/result", delivery, nil); err == nil {
		t.Fatal("acknowledgement allowed the original outcome to change")
	}
	if err := worker.Call(ctx, "/v1/result", map[string]any{"id": other.ID, "session": other.Session, "result": contract.Result{Status: "success"}}, nil); err != nil {
		t.Fatal(err)
	}
	if job := claim(newSession); job.ID != "queued" || job.Session != newSession {
		t.Fatal("retained acknowledged result still holds new work after restart")
	}
	var original contract.Job
	if err := admin.Call(ctx, "/v1/submit", submission, &original); err != nil || original.Status != "unknown" ||
		original.Session != originalSession || original.ResultHash != acknowledged.ResultHash || original.Result == nil || original.Result.Log != unknown.Log {
		t.Fatalf("original job result or evidence changed after acknowledgement: %v", err)
	}
}

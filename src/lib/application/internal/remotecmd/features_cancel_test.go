package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
)

type featureCancellationFixture struct {
	operation contract.FeatureOperation
	err       error
	calls     int
	reference contract.FeatureReference
}

func (client *featureCancellationFixture) CancelFeatureResult(_ context.Context, reference contract.FeatureReference) (contract.FeatureOperation, error) {
	client.calls++
	client.reference = reference
	return client.operation, client.err
}

func TestFeatureCancellationRetainsAuthoritativeTerminalReceiptWithoutContent(t *testing.T) {
	request := fileRecoveryRequest(t)
	for _, status := range []string{"cancelled", "success", "failure"} {
		t.Run(status, func(t *testing.T) {
			client := &featureCancellationFixture{operation: contract.FeatureOperation{
				Request: request, Status: status, CancelRequested: true,
				Result: &contract.FeatureResult{Status: status, Payload: json.RawMessage(`{"private":"private-result-marker"}`), Error: "private-error-marker"},
			}}
			operation, err := cancelFeatureReceipt(context.Background(), client, request)
			if err != nil || operation.Status != status || operation.Result.Status != status || !operation.ReceiptOnly || !operation.CancelRequested {
				t.Fatalf("synchronous cancellation lost its authoritative terminal result: %+v %v", operation, err)
			}
			if client.calls != 1 || client.reference != request.Reference() || operation.Request.Reference() != request.Reference() {
				t.Fatal("cancellation queried another identity or depended on a later status read")
			}
			encoded, err := json.Marshal(operation)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"payload", "nonce", "private-file", "private-result-marker", "private-error-marker", request.Nonce} {
				if strings.Contains(string(encoded), forbidden) {
					t.Fatal("cancellation receipt exposed request/result content", forbidden)
				}
			}
			if !strings.Contains(operation.Result.Error, "does not prove prior native effects are absent") {
				t.Fatal("terminal cancellation was presented as effect-free execution")
			}
		})
	}
}

func TestFeatureCancellationKeepsNonterminalAndUnknownResponsesUncertain(t *testing.T) {
	request := fileRecoveryRequest(t)
	for _, status := range []string{"queued", "running", "unknown"} {
		t.Run(status, func(t *testing.T) {
			original := contract.FeatureOperation{Request: request, Status: status, CancelRequested: true}
			if status == "unknown" {
				original.Result = &contract.FeatureResult{Status: "unknown", Payload: json.RawMessage(`{"private":"private-result-marker"}`)}
			}
			client := &featureCancellationFixture{operation: original}
			operation, err := cancelFeatureReceipt(context.Background(), client, request)
			if err != nil || operation.Status != "unknown" || !operation.CancelRequested || !operation.ReceiptOnly || operation.Reconciled ||
				len(operation.Request.Payload) != 0 || operation.Request.Nonce != "" || len(operation.Result.Payload) != 0 || client.calls != 1 {
				t.Fatalf("nonterminal cancellation was promoted into a known outcome: %+v %v", operation, err)
			}
		})
	}
}

func TestFeatureCancellationRejectsChangedIntentMalformedResponseAndLostAcknowledgment(t *testing.T) {
	request := fileRecoveryRequest(t)
	for _, scenario := range []string{"id", "target", "action", "root", "deadline", "payload", "nonce", "result", "unacknowledged", "lost-ack"} {
		t.Run(scenario, func(t *testing.T) {
			client := &featureCancellationFixture{operation: contract.FeatureOperation{Request: request, Status: "cancelled", CancelRequested: true,
				Result: &contract.FeatureResult{Status: "cancelled"}}}
			switch scenario {
			case "id":
				client.operation.Request.ID = strings.Repeat("d", 64)
			case "target":
				client.operation.Request.Target.Generation = strings.Repeat("d", 64)
			case "action":
				client.operation.Request.Action = "files.create"
			case "root":
				client.operation.Request.Root = "other"
			case "deadline":
				client.operation.Request.Deadline = request.Deadline.Add(time.Second)
			case "payload":
				client.operation.Request.Payload = json.RawMessage(`{"path":"changed-private-marker"}`)
			case "nonce":
				client.operation.Request.Nonce = strings.Repeat("d", 64)
			case "result":
				client.operation.Result.Status = "success"
			case "unacknowledged":
				client.operation.CancelRequested = false
			case "lost-ack":
				client.err = errors.New("lost cancellation response with private-error-marker")
			}
			operation, err := cancelFeatureReceipt(context.Background(), client, request)
			if err == nil || operation.Status != "unknown" || operation.CancelRequested || operation.Request.Reference() != request.Reference() || client.calls != 1 ||
				len(operation.Request.Payload) != 0 || operation.Request.Nonce != "" || len(operation.Result.Payload) != 0 {
				t.Fatalf("unconfirmed cancellation replaced the captured uncertainty: %+v %v", operation, err)
			}
		})
	}
}

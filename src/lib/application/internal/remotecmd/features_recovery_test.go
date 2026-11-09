package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

type featureRecoveryFixture struct {
	operation contract.FeatureOperation
	result    contract.FeatureOperation
	err       error
	reference contract.FeatureReference
	requests  int
	queries   int
}

func (client *featureRecoveryFixture) ReconcileFeature(_ context.Context, reference contract.FeatureReference) (contract.FeatureOperation, error) {
	client.reference = reference
	client.requests++
	return client.operation, client.err
}

func (client *featureRecoveryFixture) FeatureStatusReference(_ context.Context, reference contract.FeatureReference, _ string) (contract.FeatureOperation, error) {
	client.queries++
	if reference != client.reference {
		return contract.FeatureOperation{}, errors.New("recovery changed its captured reference")
	}
	return client.result, client.err
}

func fileRecoveryRequest(t *testing.T) contract.FeatureRequest {
	t.Helper()
	request := featureRequestFixture(t)
	request.Root = "project"
	request.Action = "files.delete"
	request.Payload = json.RawMessage(`{"schema":"dockpipe.files/v1","action":"delete","path":"private-file","revision":"` + strings.Repeat("a", 64) + `"}`)
	request.ID, _ = request.Digest()
	return request
}

func TestFeatureReconciliationWaitsForProofAndKeepsUnknown(t *testing.T) {
	request := fileRecoveryRequest(t)
	pending := unknownFeatureReceipt(request, true)
	pending.RecoveryRequested = true
	reconciled := pending
	reconciled.Reconciled = true
	client := &featureRecoveryFixture{operation: pending, result: reconciled}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	operation, err := awaitFeatureReconciliation(ctx, client, request)
	if err != nil || !operation.Reconciled || operation.Status != "unknown" || client.requests != 1 || client.queries != 1 || client.reference != request.Reference() {
		t.Fatalf("recovery did not await the exact worker acknowledgment: %+v %v", operation, err)
	}
	if len(operation.Request.Payload) != 0 || operation.Request.Nonce != "" || len(operation.Result.Payload) != 0 {
		t.Fatal("recovery exposed replayable operation content")
	}
}

func TestFeatureReconciliationFailureCannotClaimCleanupOrReplay(t *testing.T) {
	request := fileRecoveryRequest(t)
	for _, scenario := range []string{"timeout", "locked", "changed-target", "changed-metadata", "invalid-proof"} {
		t.Run(scenario, func(t *testing.T) {
			pending := unknownFeatureReceipt(request, true)
			pending.RecoveryRequested = true
			client := &featureRecoveryFixture{operation: pending, result: pending}
			switch scenario {
			case "locked":
				client.err = remoteio.ErrOperatorLocked
			case "changed-target":
				client.operation.Request.Target.Generation = strings.Repeat("c", 64)
			case "changed-metadata":
				client.operation.Request.Root = "other"
			case "invalid-proof":
				client.operation.Reconciled = true
				client.operation.CancelRequested = false
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			operation, err := awaitFeatureReconciliation(ctx, client, request)
			if err == nil || operation.Status != "unknown" || operation.Reconciled || operation.RecoveryRequested || operation.CancelRequested || client.requests != 1 {
				t.Fatalf("unconfirmed cleanup was presented as acknowledged: %+v %v", operation, err)
			}
			if operation.Request.Reference() != request.Reference() || len(operation.Request.Payload) != 0 {
				t.Fatal("failed recovery lost the exact metadata-only reference")
			}
		})
	}
}

func TestFeatureReconciliationKnownCompletionStripsPrivateResult(t *testing.T) {
	request := fileRecoveryRequest(t)
	completed := contract.FeatureOperation{Request: request, Status: "success", Result: &contract.FeatureResult{Status: "success", Payload: json.RawMessage(`{"content":"private"}`)}}
	client := &featureRecoveryFixture{operation: completed}
	operation, err := awaitFeatureReconciliation(context.Background(), client, request)
	if err != nil || operation.Status != "success" || !operation.ReceiptOnly || operation.Reconciled || len(operation.Request.Payload) != 0 || len(operation.Result.Payload) != 0 || client.queries != 0 {
		t.Fatal("known completion recovery exposed content or invented cleanup proof", err)
	}
}

func TestFeatureRetirementFlagsRequireExactExplicitReviewBeforeInput(t *testing.T) {
	for _, arguments := range [][]string{
		{"feature-reconcile", "--request", "-", "--recovery", "retire-container"},
		{"feature-reconcile", "--request", "-", "--recovery", "retire-container", "--expected-container", "short"},
		{"feature-reconcile", "--request", "-", "--recovery", "implicit"},
		{"feature-call", "--request", "-", "--expected-container", strings.Repeat("a", 64)},
	} {
		if err := Run(arguments, nil); err == nil {
			t.Fatal("unsafe retirement flags reached input or operator initialization")
		}
	}
}

func TestHostTerminalRecoveryMetadataValidatesLostBodyReferenceAndRoot(t *testing.T) {
	fixture := featureRequestFixture(t)
	original := contract.TerminalReference{Schema: contract.TerminalVersion, ID: strings.Repeat("c", 64), Epoch: strings.Repeat("d", 64), Target: fixture.Target}
	original.Target.Generation = strings.Repeat("e", 64)
	request, err := remoteio.NewFeatureRequest(fixture.Target, "project", "terminal.host-recover", struct {
		Reference contract.TerminalReference `json:"reference"`
	}{original}, fixture.Deadline)
	if err != nil {
		t.Fatal(err)
	}
	proof := contract.HostTerminalRecovery{Schema: contract.HostTerminalReceiptVersion, Reference: original, Root: "project", Reconciled: true}
	operation := unknownFeatureReceipt(request, false)
	operation.Status = "success"
	operation.Result = &contract.FeatureResult{Status: "success"}
	operation.HostRecovery = &proof
	if err := validateFeatureOperation(operation, request); err != nil {
		t.Fatal("lost result body made the typed original host proof unusable", err)
	}
	for _, scenario := range []string{"root", "identity", "unknown"} {
		changed := proof
		invalid := operation
		invalid.HostRecovery = &changed
		switch scenario {
		case "root":
			changed.Root = "other"
		case "identity":
			changed.Reference.Target.Fingerprint = strings.Repeat("f", 64)
		case "unknown":
			invalid.Status = "unknown"
			invalid.Result = &contract.FeatureResult{Status: "unknown"}
		}
		if err := validateFeatureOperation(invalid, request); err == nil {
			t.Fatal("host proof accepted changed metadata or an unconfirmed outcome", scenario)
		}
	}
}

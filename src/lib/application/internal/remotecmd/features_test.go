package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func featureRequestFixture(t *testing.T) contract.FeatureRequest {
	t.Helper()
	target := contract.FeatureTarget{Node: "fixture", Fingerprint: strings.Repeat("a", 64), Generation: strings.Repeat("b", 64)}
	request, err := remoteio.NewFeatureRequest(target, "", "containers.snapshot", map[string]string{}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func TestFeatureInputPreservesFixedIntent(t *testing.T) {
	request := featureRequestFixture(t)
	expected := request
	request.ID = ""
	data, _ := json.Marshal(request)
	first, err := decodeFeatureRequest(data, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	second, err := decodeFeatureRequest(data, time.Now())
	if err != nil || !reflect.DeepEqual(first, second) || !reflect.DeepEqual(first, expected) {
		t.Fatal("repeated fixed intent changed identity", err)
	}
	request.ID = strings.Repeat("c", 64)
	data, _ = json.Marshal(request)
	if _, err := decodeFeatureRequest(data, time.Now()); err == nil {
		t.Fatal("accepted a substituted operation ID")
	}
	for _, deadline := range []time.Time{time.Now().Add(-time.Second), time.Now().Add(3 * time.Minute)} {
		request.ID = ""
		request.Deadline = deadline
		data, _ = json.Marshal(request)
		if _, err := decodeFeatureRequest(data, time.Now()); err == nil {
			t.Fatal("accepted an expired or excessive operation deadline")
		}
	}
}

func TestFeatureInputRejectsAmbiguousJSON(t *testing.T) {
	for _, input := range []string{
		`{"schema":"one","schema":"two"}`,
		`{"schema":"one","SCHEMA":"two"}`,
		`{"payload":{"path":"one","path":"two"}}`,
		`{"payload":[{"path":1,"path":2}]}`,
		`{"unexpected":"private-user-value"}`,
		`{} {}`,
		"{\"schema\":\"\xff\"}",
		strings.Repeat("[", 65) + strings.Repeat("]", 65),
		strings.Repeat(" ", contract.MaxFeaturePayload+1),
	} {
		var request contract.FeatureRequest
		err := decodeFeatureInput([]byte(input), &request)
		if err == nil {
			t.Fatal("accepted ambiguous, oversized or untyped JSON")
		}
		if strings.Contains(err.Error(), "private-user-value") {
			t.Fatal("decoder exposed supplied content")
		}
	}
	var request contract.FeatureRequest
	if err := decodeFeatureInput([]byte(`{"payload":{"path":"ok","nested":{"other":[1,2]}},"root":"checkout"}`), &request); err != nil {
		t.Fatal("rejected unique nested fields", err)
	}
}

func TestFeatureInputReadBoundsAndCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(path, []byte(strings.Repeat(" ", contract.MaxFeaturePayload+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readFeatureInput(context.Background(), path); err == nil {
		t.Fatal("read an oversized request file")
	}
	if _, err := readFeatureInput(context.Background(), filepath.Dir(path)); err == nil {
		t.Fatal("read a directory as a request")
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	previous := os.Stdin
	os.Stdin = reader
	defer func() { os.Stdin = previous }()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := readFeatureInput(ctx, "-"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("unattended stdin did not honor its deadline", err)
	}
}

func TestFeatureReceiptRecoveryRetainsIdentityWithoutReplayableInput(t *testing.T) {
	request := featureRequestFixture(t)
	request.Deadline = time.Now().Add(-time.Hour)
	receipt := unknownFeatureReceipt(request, true)
	data, err := json.Marshal(receipt.Request)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeFeatureReceipt(data)
	if err != nil || decoded.Reference() != request.Reference() || !decoded.Deadline.Equal(request.Deadline) {
		t.Fatal("receipt recovery changed or rejected the original expired identity", err)
	}
	data, _ = json.Marshal(featureRequestFixture(t))
	if _, err := decodeFeatureReceipt(data); err == nil {
		t.Fatal("receipt recovery accepted replayable content")
	}
	decoded.Root = "/private/host/root"
	data, _ = json.Marshal(decoded)
	if _, err := decodeFeatureReceipt(data); err == nil {
		t.Fatal("container receipt accepted a host root")
	}
}

type featureClientFixture struct {
	request           contract.FeatureRequest
	submitErr         error
	statusErr         error
	contextCancel     context.CancelFunc
	submissions       int
	statuses          int
	cancellations     int
	cancelReference   contract.FeatureReference
	cancelContextLive bool
	terminal          bool
}

func (client *featureClientFixture) SubmitFeature(ctx context.Context, request contract.FeatureRequest) (contract.FeatureOperation, error) {
	client.submissions++
	client.request = request
	if client.contextCancel != nil {
		client.contextCancel()
	}
	operation := contract.FeatureOperation{Request: request, Status: "queued"}
	if client.terminal {
		operation.Status = "success"
		operation.Result = &contract.FeatureResult{Status: "success", Payload: json.RawMessage(`{}`)}
	}
	return operation, client.submitErr
}

func (client *featureClientFixture) FeatureStatus(ctx context.Context, request contract.FeatureRequest) (contract.FeatureOperation, error) {
	client.statuses++
	return contract.FeatureOperation{Request: request, Status: "success", Result: &contract.FeatureResult{Status: "success"}}, client.statusErr
}

func (client *featureClientFixture) CancelFeature(ctx context.Context, reference contract.FeatureReference) error {
	client.cancellations++
	client.cancelReference = reference
	deadline, bounded := ctx.Deadline()
	client.cancelContextLive = ctx.Err() == nil && bounded && time.Until(deadline) <= 2*time.Second
	allowed, _ := ctx.Value(remoteAuthenticationKey{}).(bool)
	if allowed {
		return errors.New("cleanup requested interactive authentication")
	}
	return nil
}

func TestFeatureAmbiguityNeverReplaysAndCancelsCapturedIdentity(t *testing.T) {
	request := featureRequestFixture(t)
	for _, scenario := range []string{"lost-submit", "cancelled", "lost-status", "locked-status"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), remoteAuthenticationKey{}, true))
			defer cancel()
			client := &featureClientFixture{}
			switch scenario {
			case "lost-submit":
				client.submitErr = errors.New("lost response containing private-payload")
			case "cancelled":
				client.contextCancel = cancel
			case "lost-status":
				client.statusErr = errors.New("lost response")
			case "locked-status":
				client.statusErr = remoteio.ErrOperatorLocked
			}
			operation, err := awaitFeature(ctx, client, request)
			if err == nil || operation.Status != "unknown" || !operation.ReceiptOnly || client.submissions != 1 || client.cancellations != 1 {
				t.Fatalf("ambiguous operation replayed or lost receipt: %+v %v", client, err)
			}
			if operation.Request.ID != request.ID || operation.Request.Target != request.Target || len(operation.Request.Payload) != 0 || operation.Request.Nonce != "" || strings.Contains(err.Error(), "private-payload") {
				t.Fatal("unknown receipt changed identity or exposed input")
			}
			if client.cancelReference != request.Reference() || !client.cancelContextLive {
				t.Fatal("cancellation lost captured identity or inherited cancelled context")
			}
		})
	}
}

func TestFeatureLockedSubmitDoesNotRetryOrCancel(t *testing.T) {
	client := &featureClientFixture{submitErr: remoteio.ErrOperatorLocked}
	operation, err := awaitFeature(context.Background(), client, featureRequestFixture(t))
	if !errors.Is(err, remoteio.ErrOperatorLocked) || operation.Request.ID != "" || client.submissions != 1 || client.cancellations != 0 || client.statuses != 0 {
		t.Fatal("locked admission retried or emitted a misleading receipt")
	}
}

func TestFeatureCompletionDoesNotCancel(t *testing.T) {
	for _, terminal := range []bool{true, false} {
		client := &featureClientFixture{terminal: terminal}
		operation, err := awaitFeature(context.Background(), client, featureRequestFixture(t))
		if err != nil || operation.Status != "success" || client.submissions != 1 || client.cancellations != 0 {
			t.Fatal("completed feature operation failed", err)
		}
	}
}

func TestFeatureConfigurationRequiresKeysAndPreservesPairingAuthority(t *testing.T) {
	root := filepath.Join(t.TempDir(), "remote")
	if err := remoteio.PrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	permissions := remoteio.FeaturePermissions{Schema: contract.FeatureVersion, Roots: []remoteio.FeatureRootPermission{{FeatureRoot: contract.FeatureRoot{ID: "checkout", Name: "Checkout", FilesRead: true}, Path: t.TempDir()}}}
	input := filepath.Join(root, "input.json")
	if err := remoteio.WritePrivate(input, permissions); err != nil {
		t.Fatal(err)
	}
	worker := contract.WorkerConfig{Schema: contract.Version, Node: "fixture", Token: strings.Repeat("a", 64), Endpoint: "http://127.0.0.1:47831", Delivery: &contract.DeliveryPermission{TimeoutSeconds: 60}}
	workerPath := filepath.Join(root, "worker.json")
	if err := remoteio.WritePrivate(workerPath, worker); err != nil {
		t.Fatal(err)
	}
	if err := configureFeatures(context.Background(), root, input); err == nil {
		t.Fatal("enabled features from legacy delivery pairing without machine keys")
	}
	if _, err := os.Stat(filepath.Join(root, "features.json")); !os.IsNotExist(err) {
		t.Fatal("unkeyed worker wrote feature permissions")
	}
	identity, err := remoteidentity.New()
	if err != nil {
		t.Fatal(err)
	}
	broker, err := remoteidentity.New()
	if err != nil {
		t.Fatal(err)
	}
	worker.MachineKey, worker.MachinePrivateKey, worker.BrokerKey = identity.PublicKey, identity.PrivateKey, broker.PublicKey
	if err := remoteio.WritePrivate(workerPath, worker); err != nil {
		t.Fatal(err)
	}
	if err := configureFeatures(context.Background(), root, input); err != nil {
		t.Fatal(err)
	}
	var saved contract.WorkerConfig
	if err := remoteio.ReadPrivate(workerPath, &saved); err != nil {
		t.Fatal(err)
	}
	worker.Features = true
	if !reflect.DeepEqual(saved, worker) {
		t.Fatal("local feature opt-in changed pairing or workflow authority")
	}
	loaded, err := remoteio.LoadFeaturePermissions(root)
	if err != nil || !reflect.DeepEqual(loaded, permissions) {
		t.Fatal("local feature permissions were not preserved", err)
	}
	if err := configureFeatures(context.Background(), root, ""); err != nil {
		t.Fatal("local inspection failed", err)
	}
}

func TestFeatureFlagsDoNotBroadenEdgeApproval(t *testing.T) {
	for _, command := range []string{"features", "feature-call", "feature-status", "feature-cancel", "features-config", "nodes"} {
		if err := Run([]string{command, "--approve", "activation"}, nil); err == nil || !strings.Contains(err.Error(), "--approve requires edge-activate") {
			t.Fatal("edge approval accepted by", command, err)
		}
	}
	if err := Run([]string{"nodes", "--request", "-"}, nil); err == nil {
		t.Fatal("request accepted by unrelated command")
	}
}

func TestFeatureOnlyPairingNeverGrantsWorkflowDelivery(t *testing.T) {
	root := filepath.Join(t.TempDir(), "remote")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := pairOnlineWithFeatures(ctx, root, "http://127.0.0.1:47831", "fixture", false, true, 3600); err == nil {
		t.Fatal("cancelled pairing unexpectedly succeeded")
	}
	var worker contract.WorkerConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "worker.json"), &worker); err != nil {
		t.Fatal("pairing did not persist the exact new identity before its first request", err)
	}
	if !worker.Features || worker.Delivery != nil || len(worker.Profiles) != 0 || worker.MachineKey == "" || len(worker.MachinePrivateKey) == 0 {
		t.Fatal("feature-only enrollment created delivery authority or omitted machine keys")
	}
	if err := pairOnlineWithFeatures(ctx, root, worker.Endpoint, worker.Node, true, true, 3600); err == nil || !strings.Contains(err.Error(), "saved worker identity or authority differs") {
		t.Fatal("pairing retry silently broadened workflow authority", err)
	}
	if err := Run([]string{"pair", "--allow-features"}, nil); err == nil {
		t.Fatal("feature-only opt-in accepted without online machine-key pairing")
	}
	if err := Run([]string{"nodes", "--allow-features"}, nil); err == nil {
		t.Fatal("worker feature enrollment flag accepted by an operator command")
	}
}

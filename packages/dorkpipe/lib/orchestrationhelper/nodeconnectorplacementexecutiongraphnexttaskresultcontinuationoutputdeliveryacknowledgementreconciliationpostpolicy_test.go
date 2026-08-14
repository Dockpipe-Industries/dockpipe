package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture struct {
	root         string
	expected     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected
	record       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord
	receipt      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt
	fixture      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFixture
	decisionPath string
	requestPath  string
}

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestTemplate struct {
	once    sync.Once
	fixture nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture
	files   map[string][]byte
}

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestTemplates = map[string]*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestTemplate{
	"succeeded\x00graph_continuation\x00approved":            {},
	"succeeded\x00successful_graph_finalization\x00approved": {},
	"failed\x00failed_graph_finalization\x00approved":        {},
	"succeeded\x00graph_continuation\x00rejected":            {},
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExactRoutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		terminal, route string
		authority       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority
	}{
		{"succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{ContinuationHandoffPostReconciliationAttempt: true}},
		{"succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{SuccessfulTerminalGraphResultPostReconciliationAttempt: true}},
		{"failed", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{FailedTerminalGraphResultPostReconciliationAttempt: true}},
	}
	for _, test := range tests {
		t.Run(test.route, func(t *testing.T) {
			t.Parallel()
			value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t, test.terminal, test.route, "approved")
			before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
			decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t, value)
			if request == nil || request.Authority != test.authority || decision.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{}) {
				t.Fatalf("unexpected route authority: decision=%+v request=%+v", decision.Authority, request)
			}
			binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding(value.record, value.receipt)
			if !nodeExecutionEqual(decision.Binding, binding) || !nodeExecutionEqual(request.Binding, binding) || request.ReconciliationRecordID != value.record.ReconciliationRecordID || request.ReconciliationRecordFingerprint != value.record.RecordFingerprint || request.ReconciliationExecutorReceiptID != value.receipt.ExecutorReceiptID || request.ReconciliationExecutorFingerprint != value.receipt.ReceiptFingerprint || request.ConsumerID != binding.ConsumerID || request.ConsumerContractFingerprint != binding.ConsumerContractFingerprint || request.Route != binding.Route || request.PostState != binding.PostState || request.RouteSpecificEffect != binding.RouteSpecificEffect || request.OutputType != binding.OutputType || request.DeliveryType != binding.DeliveryType || request.TerminalResult != binding.TerminalResult || request.TaskOutcome != binding.TaskOutcome {
				t.Fatal("decision/request did not preserve exact reconciliation and predecessor bindings")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyNoAction(t, *request)
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyOnlyOutputsChanged(t, value, before, true)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRejectedProducesNoRequest(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "rejected")
	before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t, value)
	if request != nil || decision.Route != "" || decision.PostState != "" || decision.RouteSpecificEffect != "" || decision.OutputType != "" || decision.DeliveryType != "" || decision.TerminalResult != "" || decision.TaskOutcome != "" || decision.ConsumerID != "" || decision.ConsumerContractFingerprint != "" || decision.PostReconciliationRequestID != "" || decision.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{}) {
		t.Fatalf("rejected decision leaked request or future authority: %+v %+v", decision, request)
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyOnlyOutputsChanged(t, value, before, false)
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRejectsInferenceAuthenticationAndRouteEscalation(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "approved")
	tests := []struct {
		name              string
		normalizeExpected bool
		mutate            func(*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture)
	}{
		{name: "approval inferred", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.ApprovalInferred = true
		}},
		{name: "route inferred", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.RouteInferred = true
		}},
		{name: "consumer inferred", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.ConsumerInferred = true
		}},
		{name: "reconciliation inferred", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.ReconciliationInferred = true
		}},
		{name: "future authority inferred", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.FutureAuthorityInferred = true
		}},
		{name: "adjacent inference source", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.InferenceSource = "receipt_presence"
		}},
		{name: "consumed", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.DecisionConsumed = true
		}},
		{name: "not deterministic", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.Deterministic = false
		}},
		{name: "not one time", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.OneTimeDecision = false
		}},
		{name: "not fixture owned", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.Provenance = "provider"
		}},
		{name: "unauthenticated", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.AuthenticationDigest = valueFingerprintForAcknowledgementReconciliationTest(t, "wrong")
		}},
		{name: "reused prior authentication", normalizeExpected: true, mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.expected.DecisionAuthenticationID = v.record.Binding.DecisionAuthenticationID
		}},
		{name: "mixed authority", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.Authority.Network = true
		}},
		{name: "cross route", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.Route = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute
		}},
		{name: "wrong state", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.PostState = "failed"
		}},
		{name: "wrong effect", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.RouteSpecificEffect = "inferred"
		}},
		{name: "wrong output", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.OutputType = NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationMaterialization
		}},
		{name: "wrong delivery", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.DeliveryType = NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationDelivery
		}},
		{name: "wrong outcome", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.TaskOutcome = "failed"
		}},
		{name: "wrong terminal", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.TerminalResult = "failed"
		}},
		{name: "wrong consumer", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.ConsumerID = "ambiguous-consumer"
		}},
		{name: "wrong contract", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.ConsumerContractFingerprint = valueFingerprintForAcknowledgementReconciliationTest(t, "wrong-contract")
		}},
		{name: "wrong request", mutate: func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
			v.fixture.PostReconciliationRequestID = "other-post-reconciliation-request"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := *value
			candidate.fixture = cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFixture(value.fixture)
			test.mutate(&candidate)
			var err error
			if test.normalizeExpected {
				_, _, err = normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected(candidate.root, candidate.expected)
			} else {
				_, _, err = deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(candidate.expected, candidate.record, candidate.receipt, candidate.fixture)
			}
			if err == nil {
				t.Fatal("invalid, inferred, ambiguous, or authority-escalated evidence was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyArtifactsAbsent(t, &candidate)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequiresExactRecordReceiptAndPredecessors(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "approved")
	tests := []struct {
		name   string
		path   func(*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) string
		mutate func([]byte) []byte
	}{
		{"record missing", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) string {
			return filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecordName)
		}, func([]byte) []byte { return nil }},
		{"record changed", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) string {
			return filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecordName)
		}, func(raw []byte) []byte { return bytes.Replace(raw, []byte(`"version": 1`), []byte(`"version": 2`), 1) }},
		{"receipt missing", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) string {
			return filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceiptName)
		}, func([]byte) []byte { return nil }},
		{"receipt changed", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) string {
			return filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceiptName)
		}, func(raw []byte) []byte {
			return bytes.Replace(raw, []byte(`"authorization_consumed": true`), []byte(`"authorization_consumed": false`), 1)
		}},
		{"acknowledgement changed", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) string {
			return filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementName)
		}, func(raw []byte) []byte {
			return bytes.Replace(raw, []byte(`"accepted": true`), []byte(`"accepted": false`), 1)
		}},
		{"delivery receipt changed", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) string {
			return filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptName)
		}, func(raw []byte) []byte {
			return bytes.Replace(raw, []byte(`"no_duplicate_delivery": true`), []byte(`"no_duplicate_delivery": false`), 1)
		}},
		{"consumer contract changed", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) string {
			return filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryRequestName)
		}, func(raw []byte) []byte {
			return bytes.Replace(raw, []byte(`"consumer_contract_fingerprint": "sha256:`), []byte(`"consumer_contract_fingerprint": "sha256:0`), 1)
		}},
		{"prior policy changed", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) string {
			return filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationDecisionName)
		}, func(raw []byte) []byte {
			return bytes.Replace(raw, []byte(`"independently_authenticated": true`), []byte(`"independently_authenticated": false`), 1)
		}},
		{"transition changed", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) string {
			return filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecordName)
		}, func(raw []byte) []byte { return bytes.Replace(raw, []byte(`"version": 1`), []byte(`"version": 2`), 1) }},
		{"accepted result changed", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) string {
			return filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskAcceptedResultName)
		}, func(raw []byte) []byte {
			return bytes.Replace(raw, []byte(`"fixture_owned": true`), []byte(`"fixture_owned": false`), 1)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := test.path(value)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.WriteFile(path, raw, 0o600); err != nil {
					t.Errorf("restore predecessor: %v", err)
				}
			}()
			changed := test.mutate(raw)
			if changed == nil {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else if bytes.Equal(changed, raw) {
				t.Fatal("test mutation did not change predecessor")
			} else if err := os.WriteFile(path, changed, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies(value.root, value.expected); err == nil {
				t.Fatal("missing or changed record, receipt, acknowledgement, consumer, or predecessor was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyArtifactsAbsent(t, value)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyReplayRestartConcurrencyAndRecovery(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "approved")
	decision1, request1 := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t, value)
	decisionRaw := mustReadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyArtifact(t, value.decisionPath)
	requestRaw := mustReadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyArtifact(t, value.requestPath)
	decision2, request2 := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t, value)
	if !nodeExecutionEqual(decision1, decision2) || !nodeExecutionEqual(request1, request2) || !bytes.Equal(decisionRaw, mustReadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyArtifact(t, value.decisionPath)) || !bytes.Equal(requestRaw, mustReadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyArtifact(t, value.requestPath)) {
		t.Fatal("exact replay/restart was not byte-identical")
	}

	concurrent := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "approved")
	const workers = 8
	results := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			policies, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies(concurrent.root, concurrent.expected)
			if err == nil {
				_, _, err = policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t, concurrent.fixture))
			}
			results <- err
		}()
	}
	for i := 0; i < workers; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}

	conflict := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "approved")
	other := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFixture(conflict.fixture)
	other.DecisionID, other.ReplayIdentity = "post-reconciliation-conflict-decision", "post-reconciliation-conflict-replay"
	conflictResults := make(chan error, 2)
	for _, raw := range [][]byte{mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t, conflict.fixture), mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t, other)} {
		go func(raw []byte) {
			policies, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies(conflict.root, conflict.expected)
			if err == nil {
				_, _, err = policies.Decide(raw)
			}
			conflictResults <- err
		}(raw)
	}
	successes := 0
	for i := 0; i < 2; i++ {
		if err := <-conflictResults; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("conflicting concurrency successes=%d, want 1", successes)
	}

	recovery := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "approved")
	expectedDecision, expectedRequest, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(recovery.expected, recovery.record, recovery.receipt, recovery.fixture)
	if err != nil {
		t.Fatal(err)
	}
	policies := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies(t, recovery)
	policies.writeRequest = func(string, any) error { return errors.New("injected request publication failure") }
	if _, _, err := policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t, recovery.fixture)); err == nil {
		t.Fatal("request publication failure was accepted")
	}
	if _, err := os.Stat(recovery.decisionPath); err != nil {
		t.Fatal("durable decision missing after request publication failure")
	}
	if _, err := os.Stat(recovery.requestPath); !os.IsNotExist(err) {
		t.Fatal("partial request was published")
	}
	recoveredDecision, recoveredRequest := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t, recovery)
	if !nodeExecutionEqual(recoveredDecision, expectedDecision) || !nodeExecutionEqual(recoveredRequest, expectedRequest) {
		t.Fatal("decision-before-request recovery changed evidence")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRejectsMalformedUnsafeOrphanedAndTamperedArtifacts(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "approved")
	canonical := mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t, value.fixture)
	policies := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies(t, value)
	malformed := []struct {
		name string
		raw  []byte
	}{
		{"empty", nil},
		{"malformed", []byte("{\n")},
		{"noncanonical", []byte("{}\n")},
		{"unknown field", append(bytes.TrimSuffix(bytes.Clone(canonical), []byte("}")), []byte(",\"unknown\":true}")...)},
		{"trailing", append(bytes.Clone(canonical), []byte("true")...)},
		{"partial", []byte(`{"schema":"dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-post-policy-decision-fixture/v1"}`)},
		{"oversized", bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionMax+1)},
	}
	for _, test := range malformed {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := policies.Decide(test.raw); err == nil {
				t.Fatal("malformed, noncanonical, partial, or oversized fixture was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyArtifactsAbsent(t, value)
		})
	}

	orphan := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "approved")
	decision, request, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(orphan.expected, orphan.record, orphan.receipt, orphan.fixture)
	if err != nil || request == nil {
		t.Fatal(err)
	}
	if err := writeJSONFileAtomic(orphan.requestPath, *request); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies(orphan.root, orphan.expected); err == nil {
		t.Fatal("request without exact decision was accepted")
	}
	_ = decision

	tampered := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "approved")
	mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t, tampered)
	raw := mustReadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyArtifact(t, tampered.requestPath)
	changed := bytes.Replace(raw, []byte(`"authorization_consumed": false`), []byte(`"authorization_consumed": true`), 1)
	if bytes.Equal(raw, changed) {
		t.Fatal("request tamper did not change bytes")
	}
	if err := os.WriteFile(tampered.requestPath, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies(tampered.root, tampered.expected); err == nil {
		t.Fatal("tampered request was accepted")
	}

	symlinked := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "approved")
	target := filepath.Join(symlinked.root, "target.json")
	if err := os.WriteFile(target, mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t, symlinked.fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, symlinked.decisionPath); err == nil {
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies(symlinked.root, symlinked.expected); err == nil {
			t.Fatal("symlinked decision was accepted")
		}
	}
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyCanonicalArtifact(symlinked.root, filepath.Join(symlinked.root, "..", "unsafe.json"), &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision{}, true); err == nil {
		t.Fatal("unsafe artifact path was accepted")
	}
}

func newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t *testing.T, terminalResult, route, decision string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture {
	t.Helper()
	template, ok := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestTemplates[terminalResult+"\x00"+route+"\x00"+decision]
	if !ok {
		t.Fatalf("unsupported post-reconciliation policy route %q/%q/%q", terminalResult, route, decision)
	}
	template.once.Do(func() {
		value := buildNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t, terminalResult, route, decision)
		template.fixture = *value
		template.files = mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
	})
	root := t.TempDir()
	for relative, raw := range template.files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	value := template.fixture
	value.root = root
	value.decisionPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionName)
	value.requestPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequestName)
	return &value
}

func buildNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t *testing.T, terminalResult, route, decision string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture {
	t.Helper()
	executor := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture(t, terminalResult, route)
	receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(t, executor)
	record := mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord(t, executor)
	expected := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyExpected{Executor: executor.expected, ReconciliationRecordFingerprint: record.RecordFingerprint, ReconciliationReceiptFingerprint: receipt.ReceiptFingerprint, DecisionAuthenticationID: "post-reconciliation-authentication", DecisionAuthenticationDigest: valueFingerprintForAcknowledgementReconciliationTest(t, "post-reconciliation-authentication"), PostReconciliationRequestID: "post-reconciliation-request"}
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyBinding(record, receipt)
	authority, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRouteAuthority(binding)
	fixture := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFixture{Schema: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFixtureSchema, DecisionID: "post-reconciliation-decision", ReplayIdentity: "post-reconciliation-replay", AuthenticationID: expected.DecisionAuthenticationID, AuthenticationDigest: expected.DecisionAuthenticationDigest, Decision: decision, Binding: binding, Deterministic: true, OneTimeDecision: true, Provenance: "fixture_only_post_reconciliation_policy_decision"}
	if decision == "approved" {
		fixture.Route, fixture.PostState, fixture.RouteSpecificEffect = binding.Route, binding.PostState, binding.RouteSpecificEffect
		fixture.OutputType, fixture.DeliveryType = binding.OutputType, binding.DeliveryType
		fixture.TerminalResult, fixture.TaskOutcome = binding.TerminalResult, binding.TaskOutcome
		fixture.ConsumerID, fixture.ConsumerContractFingerprint = binding.ConsumerID, binding.ConsumerContractFingerprint
		fixture.PostReconciliationRequestID, fixture.Authority = expected.PostReconciliationRequestID, authority
	}
	return &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture{root: executor.root, expected: expected, record: record, receipt: receipt, fixture: fixture, decisionPath: filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionName), requestPath: filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequestName)}
}

func mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies {
	t.Helper()
	policies, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return policies
}

func mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) {
	t.Helper()
	decision, request, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicies(t, value).Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t, value.fixture))
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}

func mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t *testing.T, value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFixture) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFixture(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFixture) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFixture {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFixture
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyNoAction(t *testing.T, request NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) {
	t.Helper()
	if request.AuthorizationConsumed || request.LifecycleAdvanced || request.GraphMutated || request.DependencyWorkPerformed || request.SchedulingPerformed || request.ExecutionPerformed || request.DeliveryPerformed || request.ConsumerInvoked || request.CallbackInvoked || request.PublicationPerformed || request.NetworkUsed || request.GitActionPerformed || request.ExternalActionPerformed || !request.OneTimeRequest || !request.FixtureOwned {
		t.Fatalf("post-reconciliation request performed or granted an adjacent action: %+v", request)
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyOnlyOutputsChanged(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture, before map[string][]byte, requestExpected bool) {
	t.Helper()
	after := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
	allowed := map[string]bool{filepath.Base(value.decisionPath): true}
	if requestExpected {
		allowed[filepath.Base(value.requestPath)] = true
	}
	for path, raw := range before {
		if !bytes.Equal(raw, after[path]) {
			t.Fatalf("immutable predecessor changed: %s", path)
		}
	}
	for path := range after {
		if _, existed := before[path]; !existed && !allowed[filepath.ToSlash(path)] && !allowed[filepath.Base(path)] {
			t.Fatalf("unexpected artifact created: %s", path)
		}
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyArtifactsAbsent(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture) {
	t.Helper()
	for _, path := range []string{value.decisionPath, value.requestPath} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("unexpected post-reconciliation artifact %s: %v", path, err)
		}
	}
}

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

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture struct {
	root            string
	expected        NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected
	fixture         NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecisionFixture
	acknowledgement NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgement
	receipt         NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceipt
	decisionPath    string
	requestPath     string
}

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestTemplate struct {
	once    sync.Once
	fixture nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture
	files   map[string][]byte
}

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestTemplates = map[string]*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestTemplate{
	"succeeded\x00" + NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute + "\x00approved":           {},
	"succeeded\x00" + NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute + "\x00rejected":           {},
	"succeeded\x00" + NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute + "\x00approved": {},
	"failed\x00" + NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute + "\x00approved":        {},
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExactRoutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, result, route string
		authority           NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority
	}{
		{"continuation", "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{ContinuationHandoffAcknowledgementReconciliationAttempt: true}},
		{"successful finalization", "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{SuccessfulTerminalGraphResultAcknowledgementReconciliationAttempt: true}},
		{"failed finalization", "failed", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{FailedTerminalGraphResultAcknowledgementReconciliationAttempt: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture(t, test.result, test.route, "approved")
			before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
			decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t, value)
			if request == nil || request.Authority != test.authority || request.Route != value.acknowledgement.Binding.Route || request.PostState != value.acknowledgement.Binding.PostState || request.RouteSpecificEffect != value.acknowledgement.Binding.RouteSpecificEffect || request.OutputType != value.acknowledgement.Binding.OutputType || request.DeliveryType != value.acknowledgement.Binding.DeliveryType || request.TerminalResult != value.acknowledgement.Binding.TerminalResult || request.TaskOutcome != value.acknowledgement.Binding.TaskOutcome {
				t.Fatal("approved decision did not emit the exact mutually exclusive route-compatible reconciliation authority")
			}
			if decision.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{}) || decision.AuthenticationID == value.acknowledgement.Binding.DeliveryAuthenticationID || request.AcknowledgementID != value.acknowledgement.AcknowledgementID || request.AcknowledgementFingerprint != value.acknowledgement.AcknowledgementFingerprint || request.OperationKey != value.acknowledgement.OperationKey || request.Binding.DeliveryExecutorReceiptID != value.receipt.ExecutorReceiptID || request.Binding.DeliveryExecutorReceiptFingerprint != value.receipt.ReceiptFingerprint || request.ConsumerID != value.acknowledgement.Binding.ConsumerID || request.ConsumerContractFingerprint != value.acknowledgement.Binding.ConsumerContractFingerprint {
				t.Fatal("decision or request omitted the exact independent authentication, acknowledgement, receipt, operation, or consumer binding")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyNoAction(t, *request)
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyOnlyOutputsChanged(t, value, before, true)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRejectedProducesNoRequest(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "rejected")
	before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t, value)
	if decision.Decision != "rejected" || request != nil || decision.Route != "" || decision.OutputType != "" || decision.DeliveryType != "" || decision.ConsumerID != "" || decision.ReconciliationRequestID != "" || decision.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{}) {
		t.Fatal("rejected decision named route, consumer, request, or authority evidence")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyOnlyOutputsChanged(t, value, before, false)
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRejectsInferenceAuthenticationAndRouteEscalation(t *testing.T) {
	t.Parallel()
	base := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "approved")
	mutations := []struct {
		name   string
		mutate func(*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture)
	}{
		{"approval inferred", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.ApprovalInferred = true
		}},
		{"route inferred", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.RouteInferred = true
		}},
		{"acknowledgement inferred", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.AcknowledgementInferred = true
		}},
		{"consumer inferred", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.ConsumerInferred = true
		}},
		{"reconciliation inferred", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.ReconciliationInferred = true
		}},
		{"authority inferred", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.AuthorityInferred = true
		}},
		{"inference source", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.InferenceSource = "receipt_presence"
		}},
		{"prior authentication", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.expected.DecisionAuthenticationID = v.acknowledgement.Binding.DeliveryAuthenticationID
			v.fixture.AuthenticationID = v.expected.DecisionAuthenticationID
		}},
		{"prior digest", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.expected.DecisionAuthenticationDigest = v.acknowledgement.Binding.DeliveryAuthenticationDigest
			v.fixture.AuthenticationDigest = v.expected.DecisionAuthenticationDigest
		}},
		{"consumed decision", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.DecisionConsumed = true
		}},
		{"not deterministic", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.Deterministic = false
		}},
		{"wrong consumer", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.ConsumerID = "wrong-consumer"
		}},
		{"wrong contract", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.ConsumerContractFingerprint = valueFingerprintForAcknowledgementReconciliationTest(t, "wrong-contract")
		}},
		{"mixed authority", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.Authority.SuccessfulTerminalGraphResultAcknowledgementReconciliationAttempt = true
		}},
		{"terminal authority for continuation", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.Authority = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{SuccessfulTerminalGraphResultAcknowledgementReconciliationAttempt: true}
		}},
		{"changed output", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.OutputType = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization
		}},
		{"changed delivery", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.DeliveryType = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationDelivery
		}},
		{"changed state", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.PostState = "succeeded"
		}},
		{"changed outcome", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.TaskOutcome = "failed"
		}},
		{"changed result", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.TerminalResult = "failed"
		}},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture(t, base)
			test.mutate(value)
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRejected(t, value, mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t, value.fixture))
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequiresExactAcknowledgementReceiptAndPredecessors(t *testing.T) {
	t.Parallel()
	base := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "approved")
	mutations := []struct {
		name   string
		mutate func(*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture)
	}{
		{"missing acknowledgement", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			if err := os.Remove(filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementName)); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing receipt", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			if err := os.Remove(filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptName)); err != nil {
				t.Fatal(err)
			}
		}},
		{"unaccepted acknowledgement", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.acknowledgement.Accepted = false
			rewriteAcknowledgementReconciliationPolicyAcknowledgement(t, v)
		}},
		{"wrong accepted count", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.acknowledgement.AcceptedLocalConsumerDeliveryCount = 2
			rewriteAcknowledgementReconciliationPolicyAcknowledgement(t, v)
		}},
		{"acknowledgement authority", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.acknowledgement.Authority.Publication = true
			rewriteAcknowledgementReconciliationPolicyAcknowledgement(t, v)
		}},
		{"wrong logical attempt", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.receipt.LogicalDeliveryAttemptCount = 2
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v)
		}},
		{"wrong invocation count", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.receipt.ConsumerInvocationCount = 0
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v)
		}},
		{"wrong acknowledgement count", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.receipt.AcceptedAcknowledgementCount = 0
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v)
		}},
		{"wrong acknowledgement write", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.receipt.AcknowledgementArtifactWriteCount = 0
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v)
		}},
		{"wrong receipt write", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.receipt.ExecutorReceiptWriteCount = 0
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v)
		}},
		{"authorization not consumed", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.receipt.AuthorizationConsumed = false
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v)
		}},
		{"chain not revalidated", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.receipt.CompleteImmutablePredecessorChainRevalidated = false
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v)
		}},
		{"duplicate delivery", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.receipt.NoDuplicateDelivery = false
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v)
		}},
		{"consumer reinvoked", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.receipt.ConsumerReinvoked = true
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v)
		}},
		{"wrong ownership", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.receipt.FixtureOwned = false
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v)
		}},
		{"receipt authority", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.receipt.Authority.Git = true
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v)
		}},
		{"changed predecessor", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
			v.fixture.Binding.DeliveryExecutorBinding.CandidatesFingerprint = valueFingerprintForAcknowledgementReconciliationTest(t, "changed-candidates")
		}},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture(t, base)
			test.mutate(value)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies(value.root, value.expected); err == nil {
				assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRejected(t, value, mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t, value.fixture))
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyReplayRestartConcurrencyAndRecovery(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "approved")
	raw := mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t, value.fixture)
	policies := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies(t, value)
	decision, request, err := policies.Decide(raw)
	if err != nil || request == nil {
		t.Fatal("initial decision failed", err)
	}
	decisionRaw := mustReadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyArtifact(t, value.decisionPath)
	requestRaw := mustReadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyArtifact(t, value.requestPath)
	for _, candidate := range []*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies{policies, mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies(t, value)} {
		replayedDecision, replayedRequest, replayErr := candidate.Decide(raw)
		if replayErr != nil || replayedRequest == nil || !nodeExecutionEqual(replayedDecision, decision) || !nodeExecutionEqual(*replayedRequest, *request) || !bytes.Equal(decisionRaw, mustReadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyArtifact(t, value.decisionPath)) || !bytes.Equal(requestRaw, mustReadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyArtifact(t, value.requestPath)) {
			t.Fatal("exact replay or restart was not byte-identical", replayErr)
		}
	}

	concurrent := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture(t, "failed", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute, "approved")
	concurrentRaw := mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t, concurrent.fixture)
	var wait sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			policy, openErr := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies(concurrent.root, concurrent.expected)
			if openErr != nil {
				errs <- openErr
				return
			}
			_, _, decideErr := policy.Decide(concurrentRaw)
			errs <- decideErr
		}()
	}
	wait.Wait()
	close(errs)
	for concurrentErr := range errs {
		if concurrentErr != nil {
			t.Fatal("identical concurrency did not converge", concurrentErr)
		}
	}

	recovery := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute, "approved")
	recoveryPolicies := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies(t, recovery)
	recoveryPolicies.writeRequest = func(string, any) error { return errors.New("injected request publication failure") }
	if _, _, err := recoveryPolicies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t, recovery.fixture)); err == nil {
		t.Fatal("injected request publication failure unexpectedly succeeded")
	}
	if _, err := os.Lstat(recovery.decisionPath); err != nil {
		t.Fatal("decision was not durable before request failure", err)
	}
	if _, err := os.Lstat(recovery.requestPath); !os.IsNotExist(err) {
		t.Fatal("request unexpectedly survived injected publication failure")
	}
	recoveryPolicies.writeRequest = writeJSONFileAtomic
	if _, recovered, err := recoveryPolicies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t, recovery.fixture)); err != nil || recovered == nil {
		t.Fatal("decision-before-request recovery failed", err)
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRejectsMalformedUnsafeOrphanedAndTamperedArtifacts(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "approved")
	malformed := [][]byte{nil, []byte("{}\n"), append(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t, value.fixture), []byte("{}")...), bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationDecisionMax+1)}
	unknown := map[string]any{}
	if err := json.Unmarshal(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t, value.fixture), &unknown); err != nil {
		t.Fatal(err)
	}
	unknown["unknown"] = true
	unknownRaw, _ := json.MarshalIndent(unknown, "", "  ")
	malformed = append(malformed, append(unknownRaw, '\n'))
	for _, raw := range malformed {
		assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRejected(t, value, raw)
	}

	orphan := value
	decision, request := mustDeriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t, orphan)
	if request == nil {
		t.Fatal("approved fixture produced no request")
	}
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, orphan.requestPath, *request)
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies(orphan.root, orphan.expected); err == nil {
		t.Fatal("request without decision was accepted")
	}
	if err := os.Remove(orphan.requestPath); err != nil {
		t.Fatal(err)
	}

	tampered := value
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, tampered.decisionPath, decision)
	decision.DecisionFingerprint = valueFingerprintForAcknowledgementReconciliationTest(t, "tampered")
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, tampered.decisionPath, decision)
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies(tampered.root, tampered.expected); err == nil {
		t.Fatal("tampered decision was accepted")
	}
	if err := os.Remove(tampered.decisionPath); err != nil {
		t.Fatal(err)
	}

	symlinked := value
	target := filepath.Join(symlinked.root, "outside-decision.json")
	defer os.Remove(target)
	defer os.Remove(symlinked.decisionPath)
	if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, symlinked.decisionPath); err == nil {
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies(symlinked.root, symlinked.expected); err == nil {
			t.Fatal("symlinked decision was accepted")
		}
	}
}

func newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture(t *testing.T, terminalResult, route, decision string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture {
	t.Helper()
	template, ok := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestTemplates[terminalResult+"\x00"+route+"\x00"+decision]
	if !ok {
		t.Fatalf("unsupported acknowledgement-reconciliation policy test route %q/%q/%q", terminalResult, route, decision)
	}
	template.once.Do(func() {
		value := buildNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture(t, terminalResult, route, decision)
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
	value.decisionPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationDecisionName)
	value.requestPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRequestName)
	return &value
}

func buildNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture(t *testing.T, terminalResult, route, decision string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture {
	t.Helper()
	delivery := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorTestFixture(t, terminalResult, route)
	acknowledgement, receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutor(t, delivery)
	expected := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyExpected{Executor: delivery.expected, AcknowledgementFingerprint: acknowledgement.AcknowledgementFingerprint, DeliveryExecutorReceiptFingerprint: receipt.ReceiptFingerprint, DecisionAuthenticationID: "acknowledgement-reconciliation-authentication", DecisionAuthenticationDigest: valueFingerprintForAcknowledgementReconciliationTest(t, "acknowledgement-reconciliation-authentication"), ReconciliationRequestID: "acknowledgement-reconciliation-request"}
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyBinding(acknowledgement, receipt)
	authority, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRouteAuthority(binding)
	fixture := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecisionFixture{Schema: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecisionFixtureSchema, DecisionID: "acknowledgement-reconciliation-decision", ReplayIdentity: "acknowledgement-reconciliation-replay", AuthenticationID: expected.DecisionAuthenticationID, AuthenticationDigest: expected.DecisionAuthenticationDigest, Decision: decision, Binding: binding, Deterministic: true, OneTimeDecision: true, Provenance: "fixture_only_post_delivery_acknowledgement_reconciliation_policy_decision"}
	if decision == "approved" {
		fixture.Route = binding.Route
		fixture.PostState = binding.PostState
		fixture.RouteSpecificEffect = binding.RouteSpecificEffect
		fixture.OutputType = binding.OutputType
		fixture.DeliveryType = binding.DeliveryType
		fixture.TerminalResult = binding.TerminalResult
		fixture.TaskOutcome = binding.TaskOutcome
		fixture.ReconciliationRequestID = expected.ReconciliationRequestID
		fixture.ConsumerID = binding.ConsumerID
		fixture.ConsumerContractFingerprint = binding.ConsumerContractFingerprint
		fixture.Authority = authority
	}
	return &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture{root: delivery.root, expected: expected, fixture: fixture, acknowledgement: acknowledgement, receipt: receipt, decisionPath: filepath.Join(delivery.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationDecisionName), requestPath: filepath.Join(delivery.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRequestName)}
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture {
	t.Helper()
	root := t.TempDir()
	for relative, raw := range mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root) {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cloned := *value
	cloned.root = root
	cloned.decisionPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationDecisionName)
	cloned.requestPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRequestName)
	return &cloned
}

func mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies {
	t.Helper()
	policies, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return policies
}
func mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest) {
	t.Helper()
	decision, request, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies(t, value).Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t, value.fixture))
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}
func mustDeriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest) {
	t.Helper()
	decision, request, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(value.expected, value.acknowledgement, value.receipt, value.fixture)
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}
func mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t *testing.T, value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecisionFixture) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func valueFingerprintForAcknowledgementReconciliationTest(t *testing.T, value string) string {
	t.Helper()
	fingerprint, err := nodeExecutionFingerprintValue(value)
	if err != nil {
		t.Fatal(err)
	}
	return fingerprint
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRejected(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture, raw []byte) {
	t.Helper()
	before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
	policies, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicies(value.root, value.expected)
	if err == nil {
		_, _, err = policies.Decide(raw)
	}
	if err == nil {
		t.Fatal("invalid acknowledgement reconciliation policy evidence was accepted")
	}
	after := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
	if !nodeExecutionEqual(before, after) {
		t.Fatal("failed acknowledgement reconciliation policy mutated durable evidence")
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyOnlyOutputsChanged(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture, before map[string][]byte, requestExpected bool) {
	t.Helper()
	after := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
	decisionRelative, _ := filepath.Rel(value.root, value.decisionPath)
	requestRelative, _ := filepath.Rel(value.root, value.requestPath)
	delete(after, filepath.ToSlash(decisionRelative))
	if requestExpected {
		delete(after, filepath.ToSlash(requestRelative))
	}
	if !nodeExecutionEqual(before, after) {
		t.Fatal("acknowledgement reconciliation policy changed predecessor evidence")
	}
	if _, err := os.Lstat(value.decisionPath); err != nil {
		t.Fatal("decision was not persisted", err)
	}
	if _, err := os.Lstat(value.requestPath); requestExpected && err != nil || !requestExpected && !os.IsNotExist(err) {
		t.Fatal("request persistence did not match the decision")
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyNoAction(t *testing.T, request NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest) {
	t.Helper()
	if request.AuthorizationConsumed || request.AcknowledgementReconciled || request.LifecycleAdvanced || request.GraphMutated || request.DependencyWorkPerformed || request.DependencyReleasePerformed || request.FailurePropagationPerformed || request.CandidateDiscoveryPerformed || request.CandidateSelectionPerformed || request.SchedulingPerformed || request.ExecutionPerformed || request.NodeExecutionPerformed || request.QueueProcessingPerformed || request.RetryPerformed || request.RepairPerformed || request.CancellationPerformed || request.CallbackInvoked || request.PublicationPerformed || request.ProviderInvoked || request.ConnectorInvoked || request.BrokerInvoked || request.ForgePipeInvoked || request.ProcessLaunched || request.NetworkUsed || request.RemoteExecutionPerformed || request.ValidationPerformed || request.CheckoutMutated || request.GitActionPerformed || request.CheckpointPerformed || request.CommitPerformed || request.PushPerformed || request.ExternalActionPerformed {
		t.Fatal("policy request recorded an executor, lifecycle, publication, Git, network, or external action")
	}
}

func rewriteAcknowledgementReconciliationPolicyAcknowledgement(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
	t.Helper()
	value.acknowledgement.AcknowledgementFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementFingerprint(value.acknowledgement)
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementName), value.acknowledgement)
}
func rewriteAcknowledgementReconciliationPolicyReceipt(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture) {
	t.Helper()
	value.receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptFingerprint(value.receipt)
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptName), value.receipt)
}
func mustReadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyArtifact(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

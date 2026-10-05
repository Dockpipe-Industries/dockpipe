package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture struct {
	root        string
	expected    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected
	decision    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision
	request     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest
	attemptPath string
	receiptPath string
	requestPath string
}

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestTemplate struct {
	once    sync.Once
	fixture nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture
	files   map[string][]byte
}

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestTemplates = map[string]*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestTemplate{
	"succeeded\x00graph_continuation":            {},
	"succeeded\x00successful_graph_finalization": {},
	"failed\x00failed_graph_finalization":        {},
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExactRoutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		terminal, route, attemptType string
		authority                    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority
	}{
		{"succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "continuation_handoff_post_reconciliation_attempt", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{ContinuationHandoffPostReconciliationAttempt: true}},
		{"succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute, "successful_terminal_graph_result_post_reconciliation_attempt", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{SuccessfulTerminalGraphResultPostReconciliationAttempt: true}},
		{"failed", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute, "failed_terminal_graph_result_post_reconciliation_attempt", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{FailedTerminalGraphResultPostReconciliationAttempt: true}},
	}
	for _, test := range tests {
		t.Run(test.route, func(t *testing.T) {
			t.Parallel()
			value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture(t, test.terminal, test.route)
			before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
			receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor(t, value)
			attempt := mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttempt(t, value)
			expectedBinding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorBinding(mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs(t, value))
			expectedEvidence := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorEvidence{LocalPostReconciliationAttemptRecorded: true}
			if attempt.Version != 1 || attempt.AttemptType != test.attemptType || receipt.AttemptType != test.attemptType || receipt.ConsumedAuthority != test.authority || attempt.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{}) || receipt.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{}) || !nodeExecutionEqual(attempt.Binding, expectedBinding) || !nodeExecutionEqual(receipt.Binding, expectedBinding) || attempt.Evidence != expectedEvidence || receipt.Evidence != expectedEvidence {
				t.Fatalf("unexpected route-bound attempt/receipt evidence: attempt=%+v receipt=%+v", attempt, receipt)
			}
			if receipt.LogicalLocalPostReconciliationAttemptCount != 1 || receipt.AttemptRecordWriteCount != 1 || receipt.ExecutorReceiptWriteCount != 1 || !receipt.AuthorizationConsumed || !receipt.CompleteImmutablePredecessorChainRevalidated || !receipt.NoDuplicateAttempt || !receipt.RequestUnchanged || !attempt.FixtureOwned || !receipt.FixtureOwned {
				t.Fatalf("unexpected durable consumption counts or flags: %+v", receipt)
			}
			if attempt.Binding.PolicyDecisionID != value.decision.DecisionID || attempt.Binding.PolicyDecisionFingerprint != value.decision.DecisionFingerprint || attempt.Binding.PolicyRequestID != value.request.RequestID || attempt.Binding.PolicyRequestFingerprint != value.request.RequestFingerprint || attempt.Binding.PolicyReplayIdentity != value.decision.ReplayIdentity || attempt.Binding.PolicyAuthenticationID != value.decision.AuthenticationID || attempt.Binding.PolicyAuthenticationDigest != value.decision.AuthenticationDigest || !nodeExecutionEqual(attempt.Binding.PredecessorBinding, value.request.Binding) || attempt.Binding.AcknowledgementID != value.request.Binding.AcknowledgementID || attempt.Binding.AcknowledgementFingerprint != value.request.Binding.AcknowledgementFingerprint || attempt.Binding.OperationKey != value.request.Binding.OperationKey || attempt.Binding.DeliveryExecutorReceiptID != value.request.Binding.DeliveryExecutorReceiptID || attempt.Binding.DeliveryExecutorFingerprint != value.request.Binding.DeliveryExecutorReceiptFingerprint || attempt.Binding.ConsumerID != value.request.ConsumerID || attempt.Binding.ConsumerContractFingerprint != value.request.ConsumerContractFingerprint || attempt.Binding.Route != value.request.Route || attempt.Binding.PostState != value.request.PostState || attempt.Binding.RouteSpecificEffect != value.request.RouteSpecificEffect || attempt.Binding.OutputType != value.request.OutputType || attempt.Binding.DeliveryType != value.request.DeliveryType || attempt.Binding.TerminalResult != value.request.TerminalResult || attempt.Binding.TaskOutcome != value.request.TaskOutcome {
				t.Fatal("attempt did not preserve the exact policy, acknowledgement, delivery, consumer, route, outcome, and predecessor bindings")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorOnlyOutputsChanged(t, value, before)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorRequiresExactIndependentAuthority(t *testing.T) {
	t.Parallel()
	t.Run("rejected decision", func(t *testing.T) {
		policy := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "rejected")
		decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t, policy)
		if request != nil || decision.Decision != "rejected" {
			t.Fatal("rejected policy unexpectedly emitted a request")
		}
		expected := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected{Policy: policy.expected, PolicyDecisionFingerprint: decision.DecisionFingerprint, PolicyRequestFingerprint: valueFingerprintForAcknowledgementReconciliationTest(t, "missing-request")}
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor(policy.root, expected); err == nil {
			t.Fatal("rejected decision reached the executor")
		}
	})
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	inputs := mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs(t, value)
	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest)
	}{
		{"consumed request", func(_ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected, _ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, request *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) {
			request.AuthorizationConsumed = true
		}},
		{"non fixture owned request", func(_ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected, _ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, request *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) {
			request.FixtureOwned = false
		}},
		{"mixed authority", func(_ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected, _ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, request *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) {
			request.Authority.Execution = true
		}},
		{"empty authority", func(_ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected, _ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, request *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) {
			request.Authority = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{}
		}},
		{"unknown authority", func(_ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected, _ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, request *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) {
			request.Authority = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyAuthority{Scheduling: true}
		}},
		{"route mismatch", func(_ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected, _ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, request *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) {
			request.Route = NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute
		}},
		{"inferred decision", func(_ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected, decision *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, _ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) {
			decision.ApprovalInferred = true
		}},
		{"unauthenticated decision", func(_ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected, decision *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, _ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) {
			decision.IndependentlyAuthenticated = false
		}},
		{"non fixture owned decision", func(_ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected, decision *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, _ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) {
			decision.FixtureOwned = false
		}},
		{"replayed decision", func(_ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected, decision *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, _ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) {
			decision.DecisionConsumed = true
		}},
		{"decision fingerprint mismatch", func(expected *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected, _ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, _ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) {
			expected.PolicyDecisionFingerprint = valueFingerprintForAcknowledgementReconciliationTest(t, "wrong-decision")
		}},
		{"request fingerprint mismatch", func(expected *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected, _ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecision, _ *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequest) {
			expected.PolicyRequestFingerprint = valueFingerprintForAcknowledgementReconciliationTest(t, "wrong-request")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			expected, decision, request := value.expected, value.decision, value.request
			test.mutate(&expected, &decision, &request)
			if _, _, err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorPolicyEvidence(expected, inputs.record, inputs.reconReceipt, decision, request); err == nil {
				t.Fatal("invalid, inferred, unauthenticated, consumed, or authority-escalated policy evidence was accepted")
			}
		})
	}
	t.Run("missing request", func(t *testing.T) {
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.requestPath)
		assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorOpenFails(t, value)
	})
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorRevalidatesCompletePredecessorChain(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	for _, test := range []struct {
		name    string
		missing bool
		path    func(string) string
	}{
		{"reconciliation record", true, func(root string) string {
			return filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecordName)
		}},
		{"reconciliation receipt", false, func(root string) string {
			return filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceiptName)
		}},
		{"acknowledgement", true, func(root string) string {
			return filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementName)
		}},
		{"delivery receipt", false, func(root string) string {
			return filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptName)
		}},
		{"accepted result", true, func(root string) string {
			return filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskAcceptedResultName)
		}},
	} {
		mode := "changed "
		if test.missing {
			mode = "missing "
		}
		t.Run(mode+test.name, func(t *testing.T) {
			path := test.path(value.root)
			raw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path)
			if test.missing {
				mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, path)
			} else {
				changed := append([]byte(nil), raw...)
				changed[len(changed)/2] ^= 1
				mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path, changed)
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorOpenFails(t, value)
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path, raw)
		})
	}
	t.Run("changed consumer contract", func(t *testing.T) {
		inputs := mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs(t, value)
		request := value.request
		request.ConsumerContractFingerprint = valueFingerprintForAcknowledgementReconciliationTest(t, "changed-consumer-contract")
		if _, _, err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorPolicyEvidence(value.expected, inputs.record, inputs.reconReceipt, value.decision, request); err == nil {
			t.Fatal("changed consumer contract was accepted")
		}
	})
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReplayRestartConcurrencyAndExistingArtifacts(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	executor := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor(t, value)
	first, err := executor.Execute()
	if err != nil {
		t.Fatal(err)
	}
	attemptRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.attemptPath)
	receiptRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.receiptPath)
	second, err := executor.Execute()
	if err != nil {
		t.Fatal(err)
	}
	restarted := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor(t, value)
	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(first, restarted) || !bytes.Equal(attemptRaw, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.attemptPath)) || !bytes.Equal(receiptRaw, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.receiptPath)) {
		t.Fatal("replay, restart, or pre-existing identical artifacts changed durable evidence")
	}

	fresh := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	const callers = 2
	var wait sync.WaitGroup
	results := make(chan NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt, callers)
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			current := &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor{root: fresh.root, expected: fresh.expected, writeAttemptAtomic: writeJSONFileAtomic, writeReceiptAtomic: writeJSONFileAtomic}
			receipt, executeErr := current.Execute()
			results <- receipt
			errs <- executeErr
		}()
	}
	wait.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var concurrent NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt
	for receipt := range results {
		if concurrent.Schema == "" {
			concurrent = receipt
		} else if !reflect.DeepEqual(concurrent, receipt) {
			t.Fatal("identical concurrency did not converge")
		}
	}

	valid := &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor{root: fresh.root, expected: fresh.expected, writeAttemptAtomic: writeJSONFileAtomic, writeReceiptAtomic: writeJSONFileAtomic}
	conflict := fresh.expected
	conflict.PolicyRequestFingerprint = valueFingerprintForAcknowledgementReconciliationTest(t, "conflicting-concurrency")
	conflicting := &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor{root: fresh.root, expected: conflict, writeAttemptAtomic: writeJSONFileAtomic, writeReceiptAtomic: writeJSONFileAtomic}
	errs = make(chan error, 2)
	wait = sync.WaitGroup{}
	wait.Add(2)
	go func() { defer wait.Done(); _, err := valid.Execute(); errs <- err }()
	go func() {
		defer wait.Done()
		_, err := conflicting.Execute()
		errs <- err
	}()
	wait.Wait()
	close(errs)
	successes, failures := 0, 0
	for err := range errs {
		if err == nil {
			successes++
		} else {
			failures++
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("conflicting concurrency produced successes=%d failures=%d", successes, failures)
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptThenReceiptRecovery(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture(t, "failed", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute)
	executor := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor(t, value)
	attemptWriter, receiptWriter := executor.writeAttemptAtomic, executor.writeReceiptAtomic
	var attemptWrites, receiptWrites atomic.Int32
	executor.writeAttemptAtomic = func(path string, payload any) error { attemptWrites.Add(1); return attemptWriter(path, payload) }
	executor.writeReceiptAtomic = func(string, any) error {
		receiptWrites.Add(1)
		return errors.New("injected receipt publication failure")
	}
	if _, err := executor.Execute(); err == nil {
		t.Fatal("receipt publication failure was accepted")
	}
	if attemptWrites.Load() != 1 || receiptWrites.Load() != 1 {
		t.Fatal("attempt-before-receipt failure did not perform exactly one logical write of each kind")
	}
	attemptRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.attemptPath)
	if _, err := os.Stat(value.receiptPath); !os.IsNotExist(err) {
		t.Fatal("receipt existed after injected publication failure")
	}
	restarted := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor(t, value)
	restarted.writeAttemptAtomic = func(path string, payload any) error { attemptWrites.Add(1); return attemptWriter(path, payload) }
	restarted.writeReceiptAtomic = func(path string, payload any) error { receiptWrites.Add(1); return receiptWriter(path, payload) }
	receipt, err := restarted.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if attemptWrites.Load() != 1 || receiptWrites.Load() != 2 || !bytes.Equal(attemptRaw, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.attemptPath)) || receipt.LogicalLocalPostReconciliationAttemptCount != 1 || receipt.AttemptRecordWriteCount != 1 || receipt.ExecutorReceiptWriteCount != 1 {
		t.Fatal("restart did not recover by writing only the exact missing receipt")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorRejectsOrphanedTamperedMalformedAndUnsafeState(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	inputs := mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs(t, value)
	attempt := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttempt(inputs)
	inputs.attempt, inputs.attemptExists = attempt, true
	receipt := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt(inputs)
	t.Run("receipt without attempt", func(t *testing.T) {
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.receiptPath, receipt)
		assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorOpenFails(t, value)
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.receiptPath)
	})
	t.Run("attempt without request", func(t *testing.T) {
		requestRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.requestPath)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.attemptPath, attempt)
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.requestPath)
		assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorOpenFails(t, value)
		mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.requestPath, requestRaw)
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.attemptPath)
	})

	for _, target := range []string{"attempt", "receipt"} {
		path := value.attemptPath
		raw := mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorArtifact(t, attempt)
		if target == "receipt" {
			path = value.receiptPath
			raw = mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorArtifact(t, receipt)
		}
		for _, malformed := range []struct {
			name string
			raw  func([]byte) []byte
		}{
			{"malformed", func([]byte) []byte { return []byte("{") }},
			{"partial", func([]byte) []byte { return []byte("{\"schema\":") }},
			{"empty", func([]byte) []byte { return nil }},
			{"noncanonical", func(raw []byte) []byte {
				var v any
				_ = json.Unmarshal(raw, &v)
				compact, _ := json.Marshal(v)
				return compact
			}},
			{"unknown field", func(raw []byte) []byte {
				return bytes.Replace(raw, []byte("\n}"), []byte(",\n  \"unknown\": true\n}"), 1)
			}},
			{"trailing", func(raw []byte) []byte { return append(append([]byte(nil), raw...), []byte("{}")...) }},
			{"oversized", func([]byte) []byte {
				return bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorArtifactMaxBytes+1)
			}},
		} {
			t.Run(target+" "+malformed.name, func(t *testing.T) {
				mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path, malformed.raw(raw))
				var decoded any
				if target == "attempt" {
					decoded = &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecord{}
				} else {
					decoded = &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt{}
				}
				if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorCanonicalArtifact(value.root, path, decoded, false); err == nil {
					t.Fatal("malformed, partial, empty, noncanonical, unknown-field, trailing, or oversized artifact was accepted")
				}
				mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, path)
			})
		}
	}

	t.Run("tampered attempt", func(t *testing.T) {
		tampered := attempt
		tampered.AttemptType = "failed_terminal_graph_result_post_reconciliation_attempt"
		tampered.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptFingerprint(tampered)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.attemptPath, tampered)
		assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorOpenFails(t, value)
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.attemptPath)
	})

	t.Run("tampered receipt", func(t *testing.T) {
		tampered := receipt
		tampered.NoDuplicateAttempt = false
		tampered.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceiptFingerprint(tampered)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.attemptPath, attempt)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.receiptPath, tampered)
		assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorOpenFails(t, value)
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.attemptPath)
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.receiptPath)
	})

	t.Run("unsafe path", func(t *testing.T) {
		outside := filepath.Join(value.root, "..", "post-reconciliation-executor-outside.json")
		if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorCanonicalArtifact(value.root, outside, &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecord{}, true); err == nil {
			t.Fatal("outside-root executor artifact path was accepted")
		}
	})

	t.Run("symlinked attempt", func(t *testing.T) {
		target := value.attemptPath + ".target"
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, target, attempt)
		if err := os.Symlink(target, value.attemptPath); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorCanonicalArtifact(value.root, value.attemptPath, &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecord{}, false); err == nil {
			t.Fatal("symlinked attempt was accepted")
		}
	})
}

func newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture(t *testing.T, terminalResult, route string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture {
	t.Helper()
	template, ok := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestTemplates[terminalResult+"\x00"+route]
	if !ok {
		t.Fatalf("unsupported post-reconciliation executor route %q/%q", terminalResult, route)
	}
	template.once.Do(func() {
		value := buildNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture(t, terminalResult, route)
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
	value.attemptPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecordName)
	value.receiptPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceiptName)
	value.requestPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequestName)
	return &value
}

func buildNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture(t *testing.T, terminalResult, route string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture {
	t.Helper()
	policy := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyTestFixture(t, terminalResult, route, "approved")
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicy(t, policy)
	if request == nil {
		t.Fatal("approved post-reconciliation policy emitted no request")
	}
	return &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture{root: policy.root, expected: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorExpected{Policy: policy.expected, PolicyDecisionFingerprint: decision.DecisionFingerprint, PolicyRequestFingerprint: request.RequestFingerprint}, decision: decision, request: *request, attemptPath: filepath.Join(policy.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecordName), receiptPath: filepath.Join(policy.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceiptName), requestPath: policy.requestPath}
}

func mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture) *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor {
	t.Helper()
	executor, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return executor
}

func mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorReceipt {
	t.Helper()
	receipt, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor(t, value).Execute()
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture) nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs {
	t.Helper()
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorInputs(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return inputs
}

func mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttempt(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecord {
	t.Helper()
	var attempt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorAttemptRecord
	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorCanonicalArtifact(value.root, value.attemptPath, &attempt, false); err != nil {
		t.Fatal(err)
	}
	return attempt
}

func mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorArtifact(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func rewriteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorRequest(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture) {
	t.Helper()
	value.request.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyRequestFingerprint(value.request)
	value.expected.PolicyRequestFingerprint = value.request.RequestFingerprint
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.requestPath, value.request)
}

func rewriteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorDecision(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture) {
	t.Helper()
	value.decision.DecisionFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionFingerprint(value.decision)
	value.expected.PolicyDecisionFingerprint = value.decision.DecisionFingerprint
	path := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostPolicyDecisionName)
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, path, value.decision)
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorOpenFails(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture) {
	t.Helper()
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutor(value.root, value.expected); err == nil {
		t.Fatal("invalid, missing, changed, conflicting, orphaned, or unsafe post-reconciliation executor evidence was accepted")
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorOnlyOutputsChanged(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPostExecutorTestFixture, before map[string][]byte) {
	t.Helper()
	after := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
	allowed := map[string]bool{filepath.ToSlash(filepath.Base(value.attemptPath)): true, filepath.ToSlash(filepath.Base(value.receiptPath)): true}
	for path, raw := range before {
		if !bytes.Equal(raw, after[path]) {
			t.Fatalf("immutable predecessor changed: %s", path)
		}
	}
	for path := range after {
		if _, existed := before[path]; !existed && !allowed[path] {
			t.Fatalf("unexpected external or adjacent artifact created: %s", path)
		}
	}
}

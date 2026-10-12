package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
)

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture struct {
	root           string
	policy         *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture
	decision       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision
	request        NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest
	expected       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorExpected
	transitionPath string
	receiptPath    string
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(t *testing.T) {
	t.Parallel()
	continuation := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	continuationBaseline := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, continuation.root)

	t.Run("exact routes", func(t *testing.T) {
		testNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorPerformsExactRoutes(t, continuation)
	})
	resetNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorRoot(t, continuation.root, continuationBaseline)
	t.Run("rejected missing and route mismatch", func(t *testing.T) {
		testNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorRejectsMissingRejectedAndOutcomeRouteMismatch(t, continuation)
	})
	t.Run("request authority", func(t *testing.T) {
		testNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorRequiresExactRequestAuthority(t, continuation)
	})
	t.Run("changed bindings", func(t *testing.T) {
		testNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorRejectsChangedBindings(t, continuation)
	})
	t.Run("replay restart and concurrency", func(t *testing.T) {
		resetNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorRoot(t, continuation.root, continuationBaseline)
		testNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReplayRestartAndConcurrency(t, continuation)
	})
	t.Run("transition recovery", func(t *testing.T) {
		resetNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorRoot(t, continuation.root, continuationBaseline)
		testNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTransitionBeforeReceiptRecovery(t, continuation)
	})
	t.Run("orphaned partial and unsafe artifacts", func(t *testing.T) {
		resetNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorRoot(t, continuation.root, continuationBaseline)
		testNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorRejectsOrphanedPartialAndUnsafeArtifacts(t, continuation, continuationBaseline)
	})
}

func testNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorPerformsExactRoutes(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture) {
	tests := []struct {
		name, result, route, effect, state string
	}{
		{"passed continuation", "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "passed_selected_task_continued_local_graph", "continued"},
		{"passed successful finalization", "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute, "passed_result_finalized_local_graph_successfully", "succeeded"},
		{"failed finalization", "failed", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute, "failed_result_finalized_local_graph_with_failure_propagation", "failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			effect, state, ok := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationRouteEffect(map[string]string{"succeeded": "passed", "failed": "failed"}[test.result], test.route)
			if !ok || effect != test.effect || state != test.state {
				t.Fatal("executor route did not derive the exact route-specific effect and post-state")
			}
			if test.route != NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute {
				return
			}
			before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
			requestBefore := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.requestPath)
			scheduledPath := value.policy.reconciliation.executor.policy.executor.selectedPath
			scheduledBefore := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, scheduledPath)
			receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(t, value)
			transition := mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord(t, value)

			if transition.Route != test.route || transition.Effect != test.effect || transition.PostState != test.state || transition.Version != 1 || !transition.FixtureOwned {
				t.Fatal("executor did not record the exact route-specific local transition")
			}
			if receipt.Route != test.route || receipt.RouteSpecificEffect != test.effect || receipt.ExactPostState != test.state || receipt.TransitionRecordID != transition.TransitionRecordID || receipt.TransitionRecordFingerprint != transition.RecordFingerprint || receipt.TransitionRecordVersion != 1 {
				t.Fatal("receipt did not bind the exact transition record and route-specific post-state")
			}
			if receipt.TransitionCount != 1 || receipt.RecordWriteCount != 1 || !receipt.AuthorizationConsumed || !receipt.FixtureOwned {
				t.Fatal("receipt did not record exactly one transition, one record write, and consumed fixture authorization")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorExactBinding(t, value, transition.Binding, receipt.Binding)
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorNarrowEvidence(t, receipt)
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorOnlyOutputsChanged(t, value, before)
			if !bytes.Equal(requestBefore, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.requestPath)) || !bytes.Equal(scheduledBefore, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, scheduledPath)) {
				t.Fatal("executor mutated the immutable policy request or persisted scheduled record")
			}
		})
	}
}

func testNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorRejectsMissingRejectedAndOutcomeRouteMismatch(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture) {
	t.Run("rejected decision", func(t *testing.T) {
		fixture := value.policy.decision
		fixture.Decision = "rejected"
		fixture.Route = ""
		fixture.ContinuationRequestID = ""
		fixture.Authority = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority{}
		decision, request, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(value.policy.expected, value.policy.accepted, value.policy.receipt, fixture)
		if err != nil {
			t.Fatal(err)
		}
		if request != nil {
			t.Fatal("rejected policy unexpectedly emitted a request")
		}
		if decision.Decision != "rejected" || decision.Route != "" || decision.ContinuationRequestID != "" {
			t.Fatal("rejected policy decision retained route authority")
		}
	})

	t.Run("missing request", func(t *testing.T) {
		request := value.request
		request.RequestID = ""
		if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest(request, value.policy.expected, value.policy.accepted, value.policy.receipt, value.decision); err == nil {
			t.Fatal("missing exact request identity was accepted")
		}
	})

	for _, test := range []struct{ result, route string }{
		{"succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute},
		{"failed", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute},
		{"failed", NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute},
		{"succeeded", "unknown_route"},
	} {
		t.Run(test.result+" "+test.route, func(t *testing.T) {
			outcome := "passed"
			if test.result == "failed" {
				outcome = "failed"
			}
			if _, _, ok := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationRouteEffect(outcome, test.route); ok {
				t.Fatal("executor accepted an outcome/route mismatch")
			}
		})
	}
}

func testNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorRequiresExactRequestAuthority(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture) {
	tests := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest)
	}{
		{"consumed", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.AuthorizationConsumed = true
		}},
		{"inferred invocation", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.GraphContinuationInvoked = true
		}},
		{"callback authority", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.CallbacksInvoked = true
		}},
		{"external authority", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.ExternalActionsInvoked = true
		}},
		{"non fixture owned", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.FixtureOwned = false
		}},
		{"unauthenticated", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.AuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('f')
		}},
		{"authority escalated", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.Authority.Callback = true
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := value.request
			test.mutate(&request)
			request.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequestFingerprint(request)
			if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest(request, value.policy.expected, value.policy.accepted, value.policy.receipt, value.decision); err == nil {
				t.Fatal("consumed, inferred, unauthenticated, non-fixture-owned, or authority-escalated request was accepted")
			}
		})
	}
}

func testNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorRejectsChangedBindings(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture) {
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs(value.root, value.policy.expected.Reconciliation)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest)
	}{
		{"reconciliation", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.Binding.ReconciliationReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')
		}},
		{"accepted result", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.Binding.AcceptedResultFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		}},
		{"observation", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.Binding.ObservationFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('3')
		}},
		{"attempt", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.Binding.AttemptRecordFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('4')
		}},
		{"executor receipt", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.Binding.ExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('5')
		}},
		{"launch authorization", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.Binding.AuthorizationRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('6')
		}},
		{"scheduling", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.Binding.SchedulingReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('7')
		}},
		{"candidate", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.Binding.CandidatesFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('8')
		}},
		{"released postimage", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.Binding.SelectedReleasedDependencyPostimage.ReleasedPostimageVersion++
		}},
		{"scheduled record", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.Binding.ScheduledRecordVersion++
		}},
		{"terminal result", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.Binding.TerminalResult = "failed"
		}},
		{"task outcome", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
			v.Binding.TaskOutcome = "failed"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := value.request
			test.mutate(&request)
			if err := validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorBindings(request, inputs); err == nil {
				t.Fatal("changed immutable binding was accepted")
			}
		})
	}
}

func testNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReplayRestartAndConcurrency(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture) {
	first := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(t, value)
	firstRecord := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.transitionPath)
	firstReceipt := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.receiptPath)
	second := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(t, value)
	if !reflect.DeepEqual(first, second) || !bytes.Equal(firstRecord, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.transitionPath)) || !bytes.Equal(firstReceipt, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.receiptPath)) {
		t.Fatal("exact replay or restart changed transition evidence")
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	results := make(chan NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			executor, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(value.root, value.expected)
			if err == nil {
				var receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt
				receipt, err = executor.Execute()
				if err == nil {
					results <- receipt
				}
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	close(results)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for result := range results {
		if !reflect.DeepEqual(first, result) {
			t.Fatal("concurrent identical execution did not converge")
		}
	}
	conflicting := bytes.Replace(firstRecord, []byte("continued"), []byte("succeeded"), 1)
	if err := os.WriteFile(value.transitionPath, conflicting, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(value.root, value.expected); err == nil {
		t.Fatal("conflicting pre-existing transition record was accepted")
	}
}

func testNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTransitionBeforeReceiptRecovery(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture) {
	executor := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(t, value)
	var transitionWrites atomic.Int32
	executor.writeTransitionAtomic = func(path string, artifact any) error {
		transitionWrites.Add(1)
		return writeJSONFileAtomic(path, artifact)
	}
	executor.writeReceiptAtomic = func(string, any) error { return errors.New("injected receipt failure") }
	if _, err := executor.Execute(); err == nil {
		t.Fatal("receipt publication failure was accepted")
	}
	if transitionWrites.Load() != 1 {
		t.Fatal("transition was not written exactly once before receipt failure")
	}
	if _, err := os.Lstat(value.transitionPath); err != nil {
		t.Fatal("exact transition record was not preserved for recovery")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorArtifactAbsent(t, value.receiptPath)
	recovered := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(t, value)
	if transitionWrites.Load() != 1 || recovered.TransitionCount != 1 || recovered.RecordWriteCount != 1 {
		t.Fatal("restart repeated the transition instead of recovering only its receipt")
	}
}

func testNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorRejectsOrphanedPartialAndUnsafeArtifacts(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture, baseline map[string][]byte) {
	t.Run("receipt without transition", func(t *testing.T) {
		receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(t, value)
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.transitionPath)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.receiptPath, receipt)
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(value.root, value.expected); err == nil {
			t.Fatal("receipt without transition record was accepted")
		}
		resetNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorRoot(t, value.root, baseline)
	})

	for _, test := range []struct {
		name string
		raw  []byte
	}{
		{"malformed", []byte("{")},
		{"unknown field", []byte("{\"unknown\":true}\n")},
		{"trailing", []byte("{}\n{}\n")},
		{"noncanonical", []byte("{}")},
		{"oversized", bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorMaxBytes+1)},
		{"partial", []byte("{\"schema\":\"dorkpipe.node-placement-execution-graph-next-task-result-continuation-transition-record/v1\"}\n")},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(value.transitionPath, test.raw, 0o600); err != nil {
				t.Fatal(err)
			}
			var artifact NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord
			if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorCanonicalArtifact(value.root, value.transitionPath, &artifact, true); err == nil {
				t.Fatal("malformed, noncanonical, trailing, oversized, or partial transition was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorArtifactAbsent(t, value.receiptPath)
			mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.transitionPath)
		})
	}

	t.Run("symlinked transition", func(t *testing.T) {
		target := filepath.Join(value.root, "transition-target.json")
		if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, value.transitionPath); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(value.root, value.expected); err == nil {
			t.Fatal("symlinked transition was accepted")
		}
	})
}

func resetNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorRoot(t *testing.T, root string, baseline map[string][]byte) {
	t.Helper()
	for _, relative := range []string{
		nodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecordName,
		nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceiptName,
		"transition-target.json",
	} {
		path := filepath.Join(root, relative)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	for relative, raw := range baseline {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture(t *testing.T, terminalResult, route string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture {
	t.Helper()
	policy := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture(t, terminalResult, "approved", route)
	decision, requestPointer := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(t, policy)
	if requestPointer == nil {
		t.Fatal("approved continuation/finalization policy did not produce a request")
	}
	request := *requestPointer
	return &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture{
		root: policy.root, policy: policy, decision: decision, request: request,
		expected: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorExpected{
			Policy: policy.expected, PolicyDecisionFingerprint: decision.DecisionFingerprint, PolicyRequestFingerprint: request.RequestFingerprint,
		},
		transitionPath: filepath.Join(policy.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecordName),
		receiptPath:    filepath.Join(policy.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceiptName),
	}
}

func mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture) *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor {
	t.Helper()
	executor, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return executor
}

func mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt {
	t.Helper()
	receipt, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(t, value).Execute()
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord {
	t.Helper()
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorInputs(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	if !inputs.transitionExists {
		t.Fatal("transition record missing")
	}
	return inputs.transition
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorExactBinding(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture, transition, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorBinding) {
	t.Helper()
	if !reflect.DeepEqual(transition, receipt) {
		t.Fatal("transition record and receipt bindings differ")
	}
	policyBinding := value.request.Binding
	executorBinding := value.policy.reconciliation.receipt.Binding
	if transition.PolicyDecisionID != value.decision.DecisionID || transition.PolicyDecisionFingerprint != value.decision.DecisionFingerprint || transition.PolicyRequestID != value.request.RequestID || transition.PolicyRequestFingerprint != value.request.RequestFingerprint || transition.PolicyAuthenticationID != value.request.AuthenticationID || transition.PolicyAuthenticationDigest != value.request.AuthenticationDigest || transition.ReconciliationReceiptID != policyBinding.ReconciliationReceiptID || transition.ReconciliationReceiptFingerprint != policyBinding.ReconciliationReceiptFingerprint || transition.AcceptedResultID != policyBinding.AcceptedResultID || transition.AcceptedResultFingerprint != policyBinding.AcceptedResultFingerprint || transition.ObservationID != policyBinding.ObservationID || transition.ObservationFingerprint != policyBinding.ObservationFingerprint || transition.AttemptID != policyBinding.AttemptID || transition.AttemptRecordFingerprint != policyBinding.AttemptRecordFingerprint || transition.ExecutorReceiptID != policyBinding.ExecutorReceiptID || transition.ExecutorReceiptFingerprint != policyBinding.ExecutorReceiptFingerprint || transition.LaunchAuthorizationDecisionID != policyBinding.AuthorizationDecisionID || transition.LaunchAuthorizationDecisionFingerprint != policyBinding.AuthorizationDecisionFingerprint || transition.LaunchAuthorizationRequestID != policyBinding.AuthorizationRequestID || transition.LaunchAuthorizationRequestFingerprint != policyBinding.AuthorizationRequestFingerprint || transition.SchedulingReceiptID != policyBinding.SchedulingReceiptID || transition.SchedulingReceiptFingerprint != policyBinding.SchedulingReceiptFingerprint || transition.SchedulingPolicyDecisionID != executorBinding.SchedulingPolicyDecisionID || transition.SchedulingPolicyDecisionFingerprint != executorBinding.SchedulingPolicyDecisionFingerprint || transition.SchedulingPolicyRequestID != executorBinding.SchedulingPolicyRequestID || transition.SchedulingPolicyRequestFingerprint != executorBinding.SchedulingPolicyRequestFingerprint || transition.GraphRunID != policyBinding.GraphRunID || transition.TerminalTaskID != policyBinding.TerminalTaskID || transition.SelectedTaskID != policyBinding.SelectedTaskID || transition.CandidatesFingerprint != policyBinding.CandidatesFingerprint || !reflect.DeepEqual(transition.SelectedReleasedDependencyPostimage, policyBinding.SelectedReleasedDependencyPostimage) || transition.ScheduledRecordID != policyBinding.ScheduledRecordID || transition.ScheduledRecordFingerprint != policyBinding.ScheduledRecordFingerprint || transition.ScheduledRecordVersion != policyBinding.ScheduledRecordVersion || transition.TerminalResult != policyBinding.TerminalResult || transition.TaskOutcome != policyBinding.TaskOutcome {
		t.Fatal("transition evidence omitted an exact policy, authentication, reconciliation, result, observation, attempt, launch, scheduling, graph, candidate, record, result, or outcome binding")
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorNarrowEvidence(t *testing.T, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt) {
	t.Helper()
	if receipt.Evidence != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorEvidence{LocalRouteTransitionPerformed: true}) {
		t.Fatal("executor widened or omitted its sole completed local-route evidence")
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"dependency_release":true`, `"next_task_scheduling":true`, `"task_launch":true`, `"node_execution":true`, `"result_collection":true`, `"result_reconciliation":true`, `"placement":true`, `"dispatch":true`, `"connector":true`, `"broker":true`, `"provider":true`, `"forgepipe":true`, `"retry":true`, `"repair":true`, `"cancellation":true`, `"callback":true`, `"general_queue_processing":true`, `"external_action":true`, `"network":true`, `"remote_execution":true`, `"validation":true`, `"checkout_mutation":true`, `"git":true`, `"checkpoint":true`, `"commit":true`, `"push":true`, `"publication":true`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("executor receipt escalated or implied forbidden activity: %s", forbidden)
		}
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorOnlyOutputsChanged(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture, before map[string][]byte) {
	t.Helper()
	after := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
	changed := make([]string, 0, 2)
	for path, raw := range after {
		old, existed := before[path]
		if !existed || !bytes.Equal(old, raw) {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, exists := after[path]; !exists {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	want := []string{nodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecordName, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceiptName}
	sort.Strings(want)
	if !reflect.DeepEqual(changed, want) {
		t.Fatalf("executor changed forbidden predecessor or adjacent state: got %v want %v", changed, want)
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorArtifactAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("executor unexpectedly published %s", path)
	}
}

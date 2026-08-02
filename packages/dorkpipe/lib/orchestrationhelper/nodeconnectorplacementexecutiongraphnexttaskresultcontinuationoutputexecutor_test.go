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

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture struct {
	root        string
	policy      *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture
	decision    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision
	request     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest
	expected    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected
	outputPath  string
	receiptPath string
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExactRoutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, result, route, output string
		evidence                    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence
	}{
		{"continuation handoff", "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffOutput, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence{ContinuationHandoffMaterialized: true}},
		{"successful terminal result", "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence{SuccessfulTerminalGraphResultMaterialized: true}},
		{"failed terminal result", "failed", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationMaterialization, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidence{FailedTerminalGraphResultMaterialized: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t, test.result, test.route)
			before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
			requestBefore := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.requestPath)
			transitionBefore := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.executor.transitionPath)
			transitionReceiptBefore := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.executor.receiptPath)

			receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(t, value)
			record := mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(t, value)
			if record.Version != 1 || !record.FixtureOwned || record.Binding.Route != test.route || record.Binding.OutputType != test.output {
				t.Fatal("executor did not materialize the exact route-compatible output record")
			}
			if receipt.Route != test.route || receipt.OutputType != test.output || receipt.ExactPostState != record.Binding.PostState || receipt.RouteSpecificEffect != record.Binding.RouteSpecificEffect || receipt.OutputRecordID != record.OutputRecordID || receipt.OutputRecordFingerprint != record.RecordFingerprint || receipt.OutputRecordVersion != 1 {
				t.Fatal("receipt did not bind the exact route-compatible output record")
			}
			if receipt.OutputActionCount != 1 || receipt.OutputRecordWriteCount != 1 || !receipt.AuthorizationConsumed || !receipt.FixtureOwned || receipt.Evidence != test.evidence {
				t.Fatal("receipt did not prove exactly one narrow completed output action")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExactBinding(t, value, record, receipt)
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorNarrowEvidence(t, receipt)
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorOnlyOutputsChanged(t, value, before)
			if !bytes.Equal(requestBefore, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.requestPath)) || !bytes.Equal(transitionBefore, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.executor.transitionPath)) || !bytes.Equal(transitionReceiptBefore, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.executor.receiptPath)) {
				t.Fatal("executor mutated the immutable request, transition, or predecessor receipt")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorRequiresExactIndependentAuthority(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)

	t.Run("missing and rejected decisions produce no output", func(t *testing.T) {
		missing := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t, value)
		if err := os.Remove(missing.policy.decisionPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(missing.policy.requestPath); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(missing.root, missing.expected); err == nil {
			t.Fatal("missing output-policy decision and request were accepted")
		}
		assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorArtifactsAbsent(t, missing)

		rejectedPolicy := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture(t, "succeeded", "rejected", "")
		decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t, rejectedPolicy)
		if request != nil {
			t.Fatal("rejected output policy emitted a request")
		}
		expected := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected{Policy: rejectedPolicy.expected, PolicyDecisionFingerprint: decision.DecisionFingerprint}
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(rejectedPolicy.root, expected); err == nil {
			t.Fatal("rejected output-policy decision was accepted by the executor")
		}
		if _, err := os.Lstat(filepath.Join(rejectedPolicy.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordName)); !os.IsNotExist(err) {
			t.Fatal("rejected decision produced an executor output")
		}
	})

	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest)
	}{
		{"consumed or replayed", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest) {
			v.AuthorizationConsumed = true
		}},
		{"handoff already invoked", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest) {
			v.ContinuationHandoffInvoked = true
		}},
		{"terminal output already invoked", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest) {
			v.TerminalGraphResultMaterializationInvoked = true
		}},
		{"callback already invoked", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest) {
			v.CallbacksInvoked = true
		}},
		{"external action already invoked", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest) {
			v.ExternalActionsInvoked = true
		}},
		{"not fixture owned", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest) {
			v.FixtureOwned = false
		}},
		{"unauthenticated", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest) {
			v.AuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('a')
		}},
		{"mixed authority", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest) {
			v.Authority.SuccessfulTerminalGraphResultMaterializationAttempt = true
		}},
		{"escalated authority", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest) {
			v.Authority.Callback = true
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest(value.request)
			test.mutate(&request)
			request.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequestFingerprint(request)
			if validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest(request, value.policy.expected, value.policy.transition, value.policy.receipt, value.decision) == nil {
				t.Fatal("consumed, replayed, invoked, unauthenticated, non-fixture-owned, mixed, or escalated request was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorArtifactsAbsent(t, value)
		})
	}

	for _, source := range []string{"result_presence", "graph_state", "transition_state", "scheduling", "availability", "connection", "lease", "provider", "broker", "forgepipe", "ranking", "cost", "risk", "validation", "receipt"} {
		t.Run("no inference from "+source, func(t *testing.T) {
			decision := value.decision
			decision.ApprovalInferred, decision.RouteInferred, decision.OutputTypeInferred, decision.AuthorityInferred, decision.InferenceSource = true, true, true, true, source
			decision.DecisionFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFingerprint(decision)
			if validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision(decision, value.policy.expected, value.policy.transition, value.policy.receipt) == nil {
				t.Fatal("adjacent evidence inferred output authority")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorRouteCompatibility(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs)
	}{
		{"unknown route", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) {
			v.request.Route = "unknown_route"
		}},
		{"empty output", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) {
			v.request.OutputType = ""
		}},
		{"terminal output on continuation", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) {
			v.request.OutputType = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization
		}},
		{"successful output on failed route", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) {
			v.transition.Route = NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute
		}},
		{"wrong post-state", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) {
			v.transition.PostState = "succeeded"
		}},
		{"wrong effect", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) {
			v.transition.Effect = "result_presence"
		}},
		{"wrong outcome", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) {
			v.transition.Binding.TaskOutcome = "failed"
		}},
		{"wrong terminal result", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) {
			v.transition.Binding.TerminalResult = "failed"
		}},
		{"empty authority", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) {
			v.request.Authority = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority{}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs(inputs)
			test.mutate(&changed)
			if _, ok := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorEvidenceFor(changed.request, changed.transition); ok {
				t.Fatal("mixed, empty, unknown, outcome-, state-, effect-, output-, or authority-incompatible route was accepted")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorRevalidatesCompleteChain(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	raw, _ := json.Marshal(value.expected)
	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected)
	}{
		{"output policy decision", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) {
			e.PolicyDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')
		}},
		{"output policy request", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) {
			e.PolicyRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		}},
		{"transition receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) {
			e.Policy.ExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('3')
		}},
		{"transition record", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) {
			e.Policy.TransitionRecordFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('4')
		}},
		{"prior continuation decision", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) {
			e.Policy.Executor.PolicyDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('5')
		}},
		{"prior continuation request", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) {
			e.Policy.Executor.PolicyRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('6')
		}},
		{"reconciliation", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) {
			e.Policy.Executor.Policy.ReconciliationReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('7')
		}},
		{"accepted result", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) {
			e.Policy.Executor.Policy.AcceptedResultFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('8')
		}},
		{"observation authentication", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) {
			e.Policy.Executor.Policy.Reconciliation.AuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('9')
		}},
		{"launch receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) {
			e.Policy.Executor.Policy.Reconciliation.ExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('a')
		}},
		{"launch authorization", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) {
			e.Policy.Executor.Policy.Reconciliation.Executor.AuthorizationRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('b')
		}},
		{"scheduling", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) {
			e.Policy.Executor.Policy.Reconciliation.Executor.Policy.SchedulingReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('c')
		}},
		{"dependency transition", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) {
			e.Policy.Executor.Policy.Reconciliation.Executor.Policy.Executor.Policy.TransitionReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('d')
		}},
		{"lifecycle", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) {
			e.Policy.Executor.Policy.Reconciliation.Executor.Policy.Executor.Policy.Executor.Policy.AuditReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('e')
		}},
		{"graph outcome", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected) {
			e.Policy.Executor.Policy.Reconciliation.Executor.Policy.Executor.Policy.Executor.Policy.Executor.Policy.Projection.Finalization.Outcomes[0].ArtifactFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('f')
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var changed NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected
			_ = json.Unmarshal(raw, &changed)
			test.mutate(&changed)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(value.root, changed); err == nil {
				t.Fatal("changed immutable predecessor chain was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorArtifactsAbsent(t, value)
		})
	}

	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs){
		func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) {
			v.request.Binding.ExecutorBinding.GraphRunID = "graph-run-wrong-001"
		},
		func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) {
			v.request.Binding.ExecutorBinding.TerminalTaskID = "terminal-task-wrong-001"
		},
		func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) {
			v.request.Binding.ExecutorBinding.SelectedTaskID = "selected-task-wrong-001"
		},
		func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) {
			v.request.Binding.ExecutorBinding.CandidatesFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('0')
		},
		func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) {
			v.request.Binding.ExecutorBinding.AcceptedResultFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')
		},
		func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) {
			v.request.Binding.ExecutorBinding.ReconciliationReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		},
		func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) {
			v.request.Binding.ExecutorBinding.PolicyAuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('3')
		},
	} {
		changed := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs(inputs)
		mutate(&changed)
		if validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorBindings(changed) == nil {
			t.Fatal("changed graph, task, candidate, result, reconciliation, or authentication binding was accepted")
		}
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReplayConcurrencyAndRecovery(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	first := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(t, value)
	firstOutput := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.outputPath)
	firstReceipt := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.receiptPath)
	second := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(t, value)
	if !reflect.DeepEqual(first, second) || !bytes.Equal(firstOutput, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.outputPath)) || !bytes.Equal(firstReceipt, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.receiptPath)) {
		t.Fatal("exact replay, restart, or identical existing artifacts changed output evidence")
	}

	const callers = 6
	var group sync.WaitGroup
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			executor, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(value.root, value.expected)
			if err == nil {
				var result NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt
				result, err = executor.Execute()
				if err == nil && !reflect.DeepEqual(first, result) {
					err = errors.New("identical concurrency changed output")
				}
			}
			errs <- err
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	recovery := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t, "failed", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute)
	executor := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(t, recovery)
	var outputWrites atomic.Int32
	executor.writeOutputAtomic = func(path string, artifact any) error { outputWrites.Add(1); return writeJSONFileAtomic(path, artifact) }
	executor.writeReceiptAtomic = func(string, any) error { return errors.New("injected receipt failure") }
	if _, err := executor.Execute(); err == nil || outputWrites.Load() != 1 {
		t.Fatal("output-before-receipt failure did not preserve exactly one output")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorArtifactAbsent(t, recovery.receiptPath)
	recovered := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(t, recovery)
	if outputWrites.Load() != 1 || recovered.OutputActionCount != 1 || recovered.OutputRecordWriteCount != 1 {
		t.Fatal("restart repeated output materialization instead of publishing only the missing receipt")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorRejectsConflictingPartialAndUnsafeState(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)

	t.Run("receipt without output", func(t *testing.T) {
		clone := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t, value)
		receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(t, clone)
		if err := os.Remove(clone.outputPath); err != nil {
			t.Fatal(err)
		}
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, clone.receiptPath, receipt)
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(clone.root, clone.expected); err == nil {
			t.Fatal("receipt without its exact output record was accepted")
		}
	})

	for _, test := range []struct {
		name string
		raw  []byte
	}{
		{"malformed", []byte("{")},
		{"unknown field", []byte("{\"unknown\":true}\n")},
		{"trailing", []byte("{}\n{}\n")},
		{"noncanonical", []byte("{}")},
		{"oversized", bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorMaxBytes+1)},
		{"partial", []byte("{\"schema\":\"dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-record/v1\"}\n")},
	} {
		t.Run(test.name, func(t *testing.T) {
			clone := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t, value)
			if err := os.WriteFile(clone.outputPath, test.raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(clone.root, clone.expected); err == nil {
				t.Fatal("malformed, unknown-field, trailing, noncanonical, oversized, partial, or ambiguous output was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorArtifactAbsent(t, clone.receiptPath)
		})
	}

	t.Run("conflicting existing output", func(t *testing.T) {
		clone := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t, value)
		_ = mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(t, clone)
		record := mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(t, clone)
		record.Binding.OutputType = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization
		record.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordFingerprint(record)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, clone.outputPath, record)
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(clone.root, clone.expected); err == nil {
			t.Fatal("conflicting route or output record was accepted")
		}
	})

	t.Run("conflicting receipt", func(t *testing.T) {
		clone := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t, value)
		receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(t, clone)
		receipt.OutputActionCount = 2
		receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptFingerprint(receipt)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, clone.receiptPath, receipt)
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(clone.root, clone.expected); err == nil {
			t.Fatal("conflicting count or receipt was accepted")
		}
	})

	t.Run("symlinked output", func(t *testing.T) {
		clone := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t, value)
		target := filepath.Join(clone.root, "output-target.json")
		if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, clone.outputPath); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(clone.root, clone.expected); err == nil {
			t.Fatal("symlinked output was accepted")
		}
	})

	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorCanonicalArtifact(value.root, filepath.Join(value.root, "..", "unsafe-output.json"), &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord{}, true); err == nil {
		t.Fatal("unsafe output path was accepted")
	}

	tampered := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t, value)
	transition := tampered.policy.transition
	transition.PostState = "succeeded"
	transition.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecordFingerprint(transition)
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, tampered.policy.executor.transitionPath, transition)
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(tampered.root, tampered.expected); err == nil {
		t.Fatal("tampered persisted predecessor was accepted")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorArtifactsAbsent(t, tampered)
}

func newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t *testing.T, terminalResult, route string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture {
	t.Helper()
	policy := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture(t, terminalResult, "approved", route)
	decision, requestPointer := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t, policy)
	if requestPointer == nil {
		t.Fatal("approved output policy did not emit a request")
	}
	request := *requestPointer
	return &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture{
		root: policy.root, policy: policy, decision: decision, request: request,
		expected: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExpected{
			Policy: policy.expected, PolicyDecisionFingerprint: decision.DecisionFingerprint, PolicyRequestFingerprint: request.RequestFingerprint,
		},
		outputPath:  filepath.Join(policy.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordName),
		receiptPath: filepath.Join(policy.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptName),
	}
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture {
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
	policy := &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture{
		root: root, transition: value.policy.transition, receipt: value.policy.receipt, expected: value.policy.expected, decision: value.policy.decision,
		decisionPath: filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionName),
		requestPath:  filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequestName),
	}
	policy.executor = &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture{
		root:           root,
		transitionPath: filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecordName),
		receiptPath:    filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceiptName),
	}
	return &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture{
		root: root, policy: policy, decision: value.decision, request: value.request, expected: value.expected,
		outputPath:  filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordName),
		receiptPath: filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptName),
	}
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs(value nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs) nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs {
	raw, _ := json.Marshal(value)
	var cloned nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs
	_ = json.Unmarshal(raw, &cloned)
	cloned.expected = value.expected
	cloned.transition = value.transition
	cloned.transitionReceipt = value.transitionReceipt
	cloned.decision = value.decision
	cloned.request = value.request
	cloned.output = value.output
	cloned.outputExists = value.outputExists
	cloned.receipt = value.receipt
	cloned.receiptExists = value.receiptExists
	return cloned
}

func mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture) *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor {
	t.Helper()
	executor, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return executor
}

func mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt {
	t.Helper()
	receipt, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(t, value).Execute()
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord {
	t.Helper()
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorInputs(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	if !inputs.outputExists {
		t.Fatal("output record missing")
	}
	return inputs.output
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorExactBinding(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture, record NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt) {
	t.Helper()
	if !reflect.DeepEqual(record.Binding, receipt.Binding) {
		t.Fatal("output record and receipt bindings differ")
	}
	b := record.Binding
	pb := value.request.Binding
	eb := pb.ExecutorBinding
	if b.OutputPolicyDecisionID != value.decision.DecisionID || b.OutputPolicyDecisionFingerprint != value.decision.DecisionFingerprint || b.OutputPolicyRequestID != value.request.RequestID || b.OutputPolicyRequestFingerprint != value.request.RequestFingerprint || b.OutputPolicyAuthenticationID != value.request.AuthenticationID || b.OutputPolicyAuthenticationDigest != value.request.AuthenticationDigest || b.TransitionExecutorReceiptID != pb.TransitionExecutorReceiptID || b.TransitionExecutorReceiptFingerprint != pb.TransitionExecutorReceiptFingerprint || b.TransitionRecordID != pb.TransitionRecordID || b.TransitionRecordFingerprint != pb.TransitionRecordFingerprint || b.TransitionRecordVersion != pb.TransitionRecordVersion || b.Route != pb.Route || b.PostState != pb.PostState || b.RouteSpecificEffect != pb.RouteSpecificEffect || b.OutputType != value.request.OutputType || b.GraphRunID != eb.GraphRunID || b.TerminalTaskID != eb.TerminalTaskID || b.SelectedTaskID != eb.SelectedTaskID || b.CandidatesFingerprint != eb.CandidatesFingerprint || b.AcceptedResultID != eb.AcceptedResultID || b.AcceptedResultFingerprint != eb.AcceptedResultFingerprint || b.ReconciliationReceiptID != eb.ReconciliationReceiptID || b.ReconciliationReceiptFingerprint != eb.ReconciliationReceiptFingerprint || b.TerminalResult != eb.TerminalResult || b.TaskOutcome != eb.TaskOutcome || b.PriorPolicyAuthenticationID != eb.PolicyAuthenticationID || b.PriorPolicyAuthenticationDigest != eb.PolicyAuthenticationDigest || !reflect.DeepEqual(b.ExecutorBinding, eb) {
		t.Fatal("output omitted an exact policy, authentication, transition, graph, task, candidate, result, reconciliation, outcome, or predecessor binding")
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorNarrowEvidence(t *testing.T, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt) {
	t.Helper()
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"receiver_invoked":true`, `"terminal_result_published":true`, `"terminal_result_delivered":true`, `"lifecycle_action_triggered":true`, `"graph_mutation":true`, `"dependency_release":true`, `"failure_propagation":true`, `"candidate_discovery":true`, `"candidate_selection":true`, `"next_task_scheduling":true`, `"task_launch":true`, `"node_execution":true`, `"result_collection":true`, `"retry":true`, `"repair":true`, `"cancellation":true`, `"general_queue_processing":true`, `"callback":true`, `"connector":true`, `"broker":true`, `"provider":true`, `"forgepipe":true`, `"process":true`, `"network":true`, `"remote_execution":true`, `"validation":true`, `"checkout_mutation":true`, `"git":true`, `"checkpoint":true`, `"commit":true`, `"push":true`, `"publication":true`, `"external_action":true`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("output receipt escalated or implied forbidden activity: %s", forbidden)
		}
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorOnlyOutputsChanged(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture, before map[string][]byte) {
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
	want := []string{nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordName, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptName}
	sort.Strings(want)
	if !reflect.DeepEqual(changed, want) {
		t.Fatalf("executor changed forbidden predecessor or adjacent state: got %v want %v", changed, want)
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorArtifactsAbsent(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture) {
	t.Helper()
	for _, path := range []string{value.outputPath, value.receiptPath} {
		assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorArtifactAbsent(t, path)
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorArtifactAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("executor unexpectedly published %s", path)
	}
}

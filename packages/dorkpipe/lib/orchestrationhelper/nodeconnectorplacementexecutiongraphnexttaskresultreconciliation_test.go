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

type nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture struct {
	root            string
	executor        *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture
	attempt         NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecord
	receipt         NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt
	expected        NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected
	observation     NodeConnectorPlacementExecutionGraphNextTaskResultObservation
	observationPath string
	acceptedPath    string
	receiptPath     string
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationMapsOnlyAuthenticatedTerminalResults(t *testing.T) {
	for _, test := range []struct{ result, outcome string }{{"succeeded", "passed"}, {"failed", "failed"}} {
		t.Run(test.result, func(t *testing.T) {
			t.Parallel()
			value := newNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture(t, test.result)
			before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
			observationBefore := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.observationPath)
			accepted, receipt := mustReconcileNodeConnectorPlacementExecutionGraphNextTaskResult(t, value)

			if accepted.TerminalResult != test.result || receipt.TerminalResult != test.result || receipt.TaskOutcome != test.outcome {
				t.Fatalf("terminal result mapping = %s -> %s, want %s -> %s", accepted.TerminalResult, receipt.TaskOutcome, test.result, test.outcome)
			}
			if accepted.ResultIngestionCount != 1 || receipt.ResultIngestionCount != 1 || receipt.AcceptedResultWriteCount != 1 || receipt.ReconciliationWriteCount != 1 || !receipt.ObservationConsumed {
				t.Fatal("result reconciliation did not record exactly one ingestion, accepted-result write, reconciliation write, and durable observation consumption")
			}
			if !receipt.CompleteImmutableChainRevalidated || !receipt.TaskLevelResultOutcomeReconciled || !accepted.FixtureOwned || !receipt.FixtureOwned {
				t.Fatal("result reconciliation omitted its complete immutable-chain or fixture-owned task-level proof")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExactBindings(t, value, accepted, receipt)
			assertNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationNarrowAuthority(t, accepted, receipt)
			if !bytes.Equal(observationBefore, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.observationPath)) {
				t.Fatal("result reconciliation rewrote its immutable result observation")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationOnlyOutputsChanged(t, value, before)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationRejectsObservationValueMatrix(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture(t, "succeeded")
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskResultObservation)
	}{
		{"empty result", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) { v.TerminalResult = "" }},
		{"unsupported result", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) { v.TerminalResult = "degraded" }},
		{"ambiguous result", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.TerminalResult = "succeeded|failed"
		}},
		{"unauthenticated identity", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.AuthenticationID = "result-authentication-wrong-001"
		}},
		{"unauthenticated digest", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.AuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')
		}},
		{"wrong attempt", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.AttemptID = "launch-execution-attempt-wrong-001"
		}},
		{"wrong attempt fingerprint", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.AttemptRecordFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		}},
		{"wrong executor receipt", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.ExecutorReceiptID = "launch-execution-receipt-wrong-001"
		}},
		{"wrong executor fingerprint", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.ExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('3')
		}},
		{"wrong graph", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.GraphRunID = "graph-run-wrong-001"
		}},
		{"wrong terminal task", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.TerminalTaskID = "terminal-task-wrong-001"
		}},
		{"wrong selected task", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.SelectedTaskID = "selected-task-wrong-001"
		}},
		{"wrong candidate set", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.CandidatesFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('4')
		}},
		{"wrong released postimage", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.SelectedReleasedDependencyPostimage.ReleasedPostimageVersion++
		}},
		{"wrong scheduled record", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.ScheduledRecordFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('5')
		}},
		{"wrong scheduled version", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) { v.ScheduledRecordVersion++ }},
		{"wrong authorization decision", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.AuthorizationDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('6')
		}},
		{"wrong authorization request", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.AuthorizationRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('7')
		}},
		{"wrong scheduling receipt", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.SchedulingReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('8')
		}},
		{"wrong predecessor binding", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.PredecessorBindingFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('9')
		}},
		{"not deterministic", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) { v.Deterministic = false }},
		{"not one time", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) { v.OneTimeObservation = false }},
		{"consumed", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) { v.ObservationConsumed = true }},
		{"inferred", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.ResultInferred = true
			v.InferenceSource = "attempt_existence"
		}},
		{"non fixture owned", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) { v.FixtureOwned = false }},
		{"authority escalated", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultObservation) {
			v.Authority.GraphCompletion = true
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := cloneNodeConnectorPlacementExecutionGraphNextTaskResultObservation(value.observation)
			test.mutate(&changed)
			changed.ObservationFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultObservationFingerprint(changed)
			if validateNodeConnectorPlacementExecutionGraphNextTaskResultObservation(changed, inputs.source) == nil {
				t.Fatal("invalid result observation was accepted")
			}
		})
	}

	for _, source := range []string{"attempt_existence", "authorization_consumption", "scheduled_state", "dependency_release", "candidate_selection", "candidate_presence", "process_exit", "connector_presence", "connection", "lease", "broker", "provider", "forgepipe", "machine", "capability", "placement", "validation", "availability", "load", "risk", "cost", "ordering", "score", "ranking", "recommendation", "matching", "graph_completion", "terminal_state", "lifecycle", "transition", "receipt"} {
		t.Run("inference source "+source, func(t *testing.T) {
			changed := cloneNodeConnectorPlacementExecutionGraphNextTaskResultObservation(value.observation)
			changed.ResultInferred, changed.InferenceSource = true, source
			changed.ObservationFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultObservationFingerprint(changed)
			if validateNodeConnectorPlacementExecutionGraphNextTaskResultObservation(changed, inputs.source) == nil {
				t.Fatal("inferred result observation was accepted")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationRejectsMissingOrChangedDurableInputs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture)
	}{
		{"missing observation", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture) {
			mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, v.observationPath)
		}},
		{"missing attempt", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture) {
			mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, v.executor.attemptPath)
		}},
		{"missing executor receipt", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture) {
			mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, v.executor.receiptPath)
		}},
		{"missing scheduled record", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture) {
			mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, v.executor.policy.executor.selectedPath)
		}},
		{"stale scheduled record", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture) {
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, v.executor.policy.executor.selectedPath, v.executor.policy.executor.preimages[v.executor.policy.executor.request.SelectedTaskID])
		}},
		{"changed observation outcome", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture) {
			changed := cloneNodeConnectorPlacementExecutionGraphNextTaskResultObservation(v.observation)
			changed.TerminalResult = "failed"
			changed.ObservationFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultObservationFingerprint(changed)
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, v.observationPath, changed)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture(t, "succeeded")
			opened := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(t, value)
			test.mutate(t, value)
			if _, _, err := opened.Reconcile(); err == nil {
				t.Fatal("missing, stale, or changed durable input was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationOutputsAbsent(t, value)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationRevalidatesCompletePredecessorChain(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture(t, "succeeded")
	expectedRaw, _ := json.Marshal(value.expected)
	tests := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected)
	}{
		{"executor receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected) {
			e.ExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('a')
		}},
		{"authorization decision", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected) {
			e.Executor.AuthorizationDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('b')
		}},
		{"authorization request", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected) {
			e.Executor.AuthorizationRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('c')
		}},
		{"scheduling receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected) {
			e.Executor.Policy.SchedulingReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('d')
		}},
		{"scheduling policy", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected) {
			e.Executor.Policy.Executor.PolicyDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('e')
		}},
		{"dependency transition", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected) {
			e.Executor.Policy.Executor.Policy.TransitionReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('f')
		}},
		{"lifecycle receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected) {
			e.Executor.Policy.Executor.Policy.Executor.Policy.AuditReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')
		}},
		{"graph projection", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected) {
			e.Executor.Policy.Executor.Policy.Executor.Policy.Executor.Policy.ProjectionDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		}},
		{"task outcome", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected) {
			e.Executor.Policy.Executor.Policy.Executor.Policy.Executor.Policy.Projection.Finalization.Outcomes[0].ArtifactFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('3')
		}},
		{"result authentication", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected) {
			e.AuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('4')
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var expected NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected
			_ = json.Unmarshal(expectedRaw, &expected)
			test.mutate(&expected)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(value.root, expected); err == nil {
				t.Fatal("mismatched immutable predecessor binding was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationOutputsAbsent(t, value)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationRejectsExecutorAuthorityClaimsAndCounts(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture(t, "succeeded")
	raw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.executor.receiptPath)
	tests := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt)
	}{
		{"attempt count", func(v *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) {
			v.AttemptCount = 2
		}},
		{"write count", func(v *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) {
			v.AttemptRecordWriteCount = 2
		}},
		{"authorization not consumed", func(v *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) {
			v.AuthorizationConsumed = false
		}},
		{"task process", func(v *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) {
			v.Evidence.TaskProcess = true
		}},
		{"task launch", func(v *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) {
			v.Evidence.TaskLaunch = true
		}},
		{"node execution", func(v *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) {
			v.Evidence.NodeExecution = true
		}},
		{"execution receipt", func(v *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) {
			v.Evidence.NodeExecutionReceipt = true
		}},
		{"successful outcome", func(v *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) {
			v.Evidence.SuccessfulTaskOutcome = true
		}},
		{"graph progress", func(v *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) {
			v.Evidence.GraphProgress = true
		}},
		{"callback", func(v *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) {
			v.Evidence.Callback = true
		}},
		{"network", func(v *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) {
			v.Evidence.Network = true
		}},
		{"validation", func(v *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) {
			v.Evidence.Validation = true
		}},
		{"git", func(v *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) {
			v.Evidence.Git = true
		}},
		{"external action", func(v *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) {
			v.Evidence.ExternalAction = true
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := value.receipt
			test.mutate(&changed)
			changed.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceiptFingerprint(changed)
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.executor.receiptPath, changed)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(value.root, value.expected); err == nil {
				t.Fatal("authority-escalating or ambiguous executor receipt was accepted")
			}
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.executor.receiptPath, raw)
			assertNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationOutputsAbsent(t, value)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationRejectsMalformedUnsafeAndTamperedArtifacts(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"malformed", func([]byte) []byte { return []byte("{") }},
		{"noncanonical", func(raw []byte) []byte {
			var value any
			_ = json.Unmarshal(raw, &value)
			compact, _ := json.Marshal(value)
			return compact
		}},
		{"unknown", func(raw []byte) []byte {
			return bytes.Replace(raw, []byte("\n}"), []byte(",\n  \"unknown\": true\n}"), 1)
		}},
		{"trailing", func(raw []byte) []byte { return append(append([]byte(nil), raw...), []byte("{}")...) }},
		{"oversized", func([]byte) []byte {
			return bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationArtifactMaxBytes+1)
		}},
	}
	for _, test := range mutations {
		t.Run("observation "+test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture(t, "succeeded")
			raw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.observationPath)
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.observationPath, test.mutate(raw))
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(value.root, value.expected); err == nil {
				t.Fatal("malformed, noncanonical, trailing, unknown, or oversized observation was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationOutputsAbsent(t, value)
		})
	}

	t.Run("symlinked observation", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture(t, "succeeded")
		target := value.observationPath + ".target"
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, target, value.observation)
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.observationPath)
		if err := os.Symlink(target, value.observationPath); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(value.root, value.expected); err == nil {
			t.Fatal("symlinked observation was accepted")
		}
	})

	t.Run("tampered accepted result", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture(t, "succeeded")
		inputs, _ := loadNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationInputs(value.root, value.expected)
		accepted := deriveNodeConnectorPlacementExecutionGraphNextTaskAcceptedResult(inputs.source)
		accepted.ResultIngestionCount = 2
		accepted.AcceptedResultFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskAcceptedResultFingerprint(accepted)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.acceptedPath, accepted)
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(value.root, value.expected); err == nil {
			t.Fatal("conflicting accepted result was accepted")
		}
	})
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReplayRestartConcurrencyAndRecovery(t *testing.T) {
	t.Run("replay restart and identical concurrency", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture(t, "succeeded")
		reconciler := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(t, value)
		firstAccepted, firstReceipt, err := reconciler.Reconcile()
		if err != nil {
			t.Fatal(err)
		}
		secondAccepted, secondReceipt, err := reconciler.Reconcile()
		if err != nil || !reflect.DeepEqual(firstAccepted, secondAccepted) || !reflect.DeepEqual(firstReceipt, secondReceipt) {
			t.Fatal("exact replay changed result or reconciliation evidence")
		}
		const callers = 12
		var wait sync.WaitGroup
		errs := make(chan error, callers)
		for index := 0; index < callers; index++ {
			wait.Add(1)
			go func() {
				defer wait.Done()
				current, openErr := OpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(value.root, value.expected)
				if openErr == nil {
					accepted, receipt, reconcileErr := current.Reconcile()
					if reconcileErr == nil && (!reflect.DeepEqual(accepted, firstAccepted) || !reflect.DeepEqual(receipt, firstReceipt)) {
						reconcileErr = errors.New("concurrent identical reconciliation changed evidence")
					}
					openErr = reconcileErr
				}
				errs <- openErr
			}()
		}
		wait.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("accepted result then receipt recovery", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture(t, "failed")
		reconciler := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(t, value)
		acceptedWriter := reconciler.writeAcceptedAtomic
		var acceptedWrites atomic.Int32
		reconciler.writeAcceptedAtomic = func(path string, payload any) error { acceptedWrites.Add(1); return acceptedWriter(path, payload) }
		reconciler.writeReceiptAtomic = func(string, any) error { return errors.New("injected receipt publication failure") }
		if _, _, err := reconciler.Reconcile(); err == nil || acceptedWrites.Load() != 1 {
			t.Fatal("receipt failure did not preserve exactly one accepted result")
		}
		restarted := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(t, value)
		restarted.writeAcceptedAtomic = func(path string, payload any) error { acceptedWrites.Add(1); return acceptedWriter(path, payload) }
		accepted, receipt, err := restarted.Reconcile()
		if err != nil || acceptedWrites.Load() != 1 || receipt.TaskOutcome != "failed" || accepted.ResultIngestionCount != 1 {
			t.Fatal("restart repeated ingestion instead of recovering only the exact reconciliation receipt")
		}
	})
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationConcurrentConflictAndOrphansFailClosed(t *testing.T) {
	t.Run("concurrent conflicting observations", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture(t, "succeeded")
		first := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(t, value)
		changed := cloneNodeConnectorPlacementExecutionGraphNextTaskResultObservation(value.observation)
		changed.TerminalResult = "failed"
		changed.ObservationFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultObservationFingerprint(changed)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.observationPath, changed)
		second := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(t, value)
		var wait sync.WaitGroup
		errs := make(chan error, 2)
		wait.Add(2)
		go func() { defer wait.Done(); _, _, err := first.Reconcile(); errs <- err }()
		go func() { defer wait.Done(); _, _, err := second.Reconcile(); errs <- err }()
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
			t.Fatalf("conflicting observations produced successes=%d failures=%d", successes, failures)
		}
	})

	t.Run("receipt without accepted result", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture(t, "succeeded")
		mustReconcileNodeConnectorPlacementExecutionGraphNextTaskResult(t, value)
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.acceptedPath)
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(value.root, value.expected); err == nil {
			t.Fatal("reconciliation receipt without accepted result was accepted")
		}
	})

	for _, test := range []struct {
		name   string
		remove func(*testing.T, *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture)
	}{
		{"observation", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture) {
			mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, v.observationPath)
		}},
		{"executor receipt", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture) {
			mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, v.executor.receiptPath)
		}},
		{"attempt", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture) {
			mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, v.executor.attemptPath)
		}},
	} {
		t.Run("accepted result without "+test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture(t, "succeeded")
			reconciler := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(t, value)
			reconciler.writeReceiptAtomic = func(string, any) error { return errors.New("injected receipt failure") }
			if _, _, err := reconciler.Reconcile(); err == nil {
				t.Fatal("receipt failure was accepted")
			}
			test.remove(t, value)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(value.root, value.expected); err == nil {
				t.Fatal("accepted result without recoverable exact immutable input was accepted")
			}
		})
	}
}

func newNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture(t *testing.T, terminalResult string) *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture {
	t.Helper()
	executor := newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture(t)
	receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(t, executor)
	attempt := mustLoadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttempt(t, executor)
	expected := NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExpected{
		Executor: executor.expected, ExecutorReceiptFingerprint: receipt.ReceiptFingerprint,
		ObservationID: "next-task-result-observation-001", ReplayIdentity: "next-task-result-replay-001",
		AuthenticationID: "next-task-result-authentication-001", AuthenticationDigest: testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('a'),
		AcceptedResultID: "next-task-accepted-result-001", ReconciliationReceiptID: "next-task-result-reconciliation-receipt-001",
	}
	binding := receipt.Binding
	bindingFingerprint, err := nodeExecutionFingerprintValue(binding)
	if err != nil {
		t.Fatal(err)
	}
	observation := NodeConnectorPlacementExecutionGraphNextTaskResultObservation{
		Schema:        NodeConnectorPlacementExecutionGraphNextTaskResultObservationSchema,
		ObservationID: expected.ObservationID, ReplayIdentity: expected.ReplayIdentity, AuthenticationID: expected.AuthenticationID, AuthenticationDigest: expected.AuthenticationDigest,
		ExecutorReceiptID: receipt.ExecutorReceiptID, ExecutorReceiptFingerprint: receipt.ReceiptFingerprint,
		AttemptID: attempt.AttemptID, AttemptRecordFingerprint: attempt.RecordFingerprint,
		AuthorizationDecisionID: binding.AuthorizationDecisionID, AuthorizationDecisionFingerprint: binding.AuthorizationDecisionFingerprint,
		AuthorizationRequestID: binding.AuthorizationRequestID, AuthorizationRequestFingerprint: binding.AuthorizationRequestFingerprint,
		SchedulingReceiptID: binding.SchedulingReceiptID, SchedulingReceiptFingerprint: binding.SchedulingReceiptFingerprint,
		GraphRunID: binding.GraphRunID, TerminalTaskID: binding.TerminalTaskID, SelectedTaskID: binding.SelectedTaskID, CandidatesFingerprint: binding.CandidatesFingerprint,
		SelectedReleasedDependencyPostimage: binding.SelectedReleasedDependencyPostimage, ScheduledRecordPostimage: binding.ScheduledRecordPostimage,
		ScheduledRecordFingerprint: binding.ScheduledRecordPostimageFingerprint, ScheduledRecordVersion: binding.ScheduledRecordPostimageVersion,
		PredecessorBindingFingerprint: bindingFingerprint, TerminalResult: terminalResult, Deterministic: true, OneTimeObservation: true, FixtureOwned: true,
	}
	observation.ObservationFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultObservationFingerprint(observation)
	value := &nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture{
		root: executor.root, executor: executor, attempt: attempt, receipt: receipt, expected: expected, observation: observation,
		observationPath: filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultObservationName),
		acceptedPath:    filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskAcceptedResultName),
		receiptPath:     filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceiptName),
	}
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.observationPath, observation)
	return value
}

func mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture) *NodeConnectorPlacementExecutionGraphNextTaskResultReconciler {
	t.Helper()
	reconciler, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return reconciler
}

func mustReconcileNodeConnectorPlacementExecutionGraphNextTaskResult(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture) (NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult, NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt) {
	t.Helper()
	accepted, receipt, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultReconciler(t, value).Reconcile()
	if err != nil {
		t.Fatal(err)
	}
	return accepted, receipt
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultObservation(value NodeConnectorPlacementExecutionGraphNextTaskResultObservation) NodeConnectorPlacementExecutionGraphNextTaskResultObservation {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultObservation
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationExactBindings(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture, accepted NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult, receipt NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt) {
	t.Helper()
	binding := value.receipt.Binding
	if accepted.ObservationID != value.observation.ObservationID || accepted.ReplayIdentity != value.observation.ReplayIdentity || accepted.AuthenticationID != value.observation.AuthenticationID || accepted.AuthenticationDigest != value.observation.AuthenticationDigest || accepted.ExecutorReceiptID != value.receipt.ExecutorReceiptID || accepted.ExecutorReceiptFingerprint != value.receipt.ReceiptFingerprint || accepted.AttemptID != value.attempt.AttemptID || accepted.AttemptRecordFingerprint != value.attempt.RecordFingerprint || accepted.AuthorizationDecisionFingerprint != binding.AuthorizationDecisionFingerprint || accepted.AuthorizationRequestFingerprint != binding.AuthorizationRequestFingerprint || accepted.GraphRunID != binding.GraphRunID || accepted.TerminalTaskID != binding.TerminalTaskID || accepted.SelectedTaskID != binding.SelectedTaskID || accepted.CandidatesFingerprint != binding.CandidatesFingerprint || !reflect.DeepEqual(accepted.SelectedReleasedDependencyPostimage, binding.SelectedReleasedDependencyPostimage) || !reflect.DeepEqual(accepted.ScheduledRecordPostimage, binding.ScheduledRecordPostimage) || receipt.AcceptedResultFingerprint != accepted.AcceptedResultFingerprint || receipt.ScheduledRecordFingerprint != binding.ScheduledRecordPostimageFingerprint || receipt.ScheduledRecordVersion != binding.ScheduledRecordPostimageVersion {
		t.Fatal("accepted result or reconciliation receipt omitted an exact authentication, attempt, authorization, scheduling, graph, candidate, selected-task, or record binding")
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationNarrowAuthority(t *testing.T, accepted NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult, receipt NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt) {
	t.Helper()
	if accepted.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultAuthority{}) || receipt.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultAuthority{}) || receipt.GraphCompletionClaimed || receipt.GraphFailurePropagated || receipt.GraphProgressClaimed || receipt.DependencyReleased || receipt.NextTaskScheduled || receipt.ExecutionInvoked || receipt.CallbackInvoked || receipt.ExternalActionInvoked {
		t.Fatal("result reconciliation claimed adjacent graph, execution, callback, or external authority")
	}
	raw, _ := json.Marshal(struct {
		Accepted any `json:"accepted"`
		Receipt  any `json:"receipt"`
	}{accepted, receipt})
	for _, forbidden := range []string{`"task_process":true`, `"task_launch":true`, `"node_execution":true`, `"execution_receipt":true`, `"graph_completion":true`, `"graph_failure":true`, `"graph_progress":true`, `"dependency_release":true`, `"next_task_scheduling":true`, `"placement":true`, `"dispatch":true`, `"connector":true`, `"lease":true`, `"broker":true`, `"provider":true`, `"forgepipe":true`, `"callback":true`, `"external_action":true`, `"network":true`, `"remote_execution":true`, `"validation":true`, `"checkout_mutation":true`, `"git":true`, `"retry":true`, `"repair":true`, `"cancellation":true`, `"publication":true`, `"lifecycle_transition":true`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("result reconciliation escalated forbidden authority: %s", forbidden)
		}
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationOnlyOutputsChanged(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture, before map[string][]byte) {
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
	want := []string{nodeConnectorPlacementExecutionGraphNextTaskAcceptedResultName, nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceiptName}
	sort.Strings(want)
	if !reflect.DeepEqual(changed, want) {
		t.Fatalf("result reconciliation changed forbidden immutable artifacts: got %v want %v", changed, want)
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationOutputsAbsent(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture) {
	t.Helper()
	for _, path := range []string{value.acceptedPath, value.receiptPath} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("failed result reconciliation unexpectedly published %s", path)
		}
	}
}

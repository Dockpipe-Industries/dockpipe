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

type nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture struct {
	root        string
	policy      *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture
	decision    NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision
	request     NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest
	expected    NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected
	attemptPath string
	receiptPath string
}

type nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestTemplate struct {
	once       sync.Once
	fixture    nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture
	policy     nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture
	scheduling nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture
	files      map[string][]byte
}

var nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorSharedTestTemplate nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestTemplate

func TestNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorMaterializesExactAttemptAndReceipt(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture(t)
	before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
	requestBefore := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestName))
	scheduledBefore := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.executor.selectedPath)
	receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(t, value)
	attempt := mustLoadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttempt(t, value)

	if attempt.AttemptID != value.request.RequestID+"-attempt" || !attempt.AttemptMaterialized || !attempt.FixtureOwned || receipt.AttemptID != attempt.AttemptID || receipt.AttemptRecordFingerprint != attempt.RecordFingerprint {
		t.Fatal("executor did not materialize the exact deterministic fixture-owned attempt")
	}
	if receipt.AttemptCount != 1 || receipt.AttemptRecordWriteCount != 1 || !receipt.AuthorizationConsumed || !receipt.FixtureOwned {
		t.Fatal("executor receipt did not prove exactly one attempt, one write, and durable authorization consumption")
	}
	expectedBinding := nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorBinding(nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs{decision: value.decision, request: value.request})
	if !reflect.DeepEqual(attempt.Binding, expectedBinding) || !reflect.DeepEqual(receipt.Binding, expectedBinding) {
		t.Fatal("attempt or receipt omitted an exact authorization, authentication, scheduling, candidate, selected-task, released-postimage, or scheduled-record binding")
	}
	if !bytes.Equal(requestBefore, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestName))) || !bytes.Equal(scheduledBefore, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.executor.selectedPath)) {
		t.Fatal("executor mutated the immutable authorization request or scheduled record")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorOnlyOutputsChanged(t, value, before)
	assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorNarrowEvidence(t, receipt)
	testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReplayRestartConcurrencyAndExistingOutput(t, value)
}

func TestNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorRequiresExactApprovedAuthorization(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture(t)
	decisionPath := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionName)
	decisionRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, decisionPath)
	requestPath := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestName)
	requestRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, requestPath)
	t.Run("missing decision", func(t *testing.T) {
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, decisionPath)
		t.Cleanup(func() {
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, decisionPath, decisionRaw)
		})
		assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorOpenFailsWithoutOutputs(t, value)
	})
	t.Run("missing request", func(t *testing.T) {
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, requestPath)
		t.Cleanup(func() {
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, requestPath, requestRaw)
		})
		assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorOpenFailsWithoutOutputs(t, value)
	})
	t.Run("rejected", func(t *testing.T) {
		policy := newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture(t, "rejected")
		decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(t, policy)
		if request != nil {
			t.Fatal("rejected authorization emitted a request")
		}
		expected := NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected{Policy: policy.expected, AuthorizationDecisionFingerprint: decision.DecisionFingerprint}
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(policy.root, expected); err == nil {
			t.Fatal("rejected authorization was accepted")
		}
		assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorArtifactsAbsent(t, policy.root)
	})

	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest)
	}{
		{"consumed", func(r *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
			r.AuthorizationConsumed = true
		}},
		{"task launch already invoked", func(r *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
			r.TaskLaunchInvoked = true
		}},
		{"node execution already invoked", func(r *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
			r.NodeExecutionInvoked = true
		}},
		{"callback already invoked", func(r *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
			r.CallbacksInvoked = true
		}},
		{"external action already invoked", func(r *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
			r.ExternalActionsInvoked = true
		}},
		{"not one time", func(r *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
			r.OneTimeRequest = false
		}},
		{"non fixture owned", func(r *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
			r.FixtureOwned = false
		}},
		{"authority escalated", func(r *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
			r.Authority.Broker = true
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := value.request
			test.mutate(&request)
			request.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestFingerprint(request)
			if err := validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest(request, value.expected.Policy, value.policy.receipt, value.decision); err == nil {
				t.Fatal("invalid or authority-escalating authorization request was accepted")
			}
		})
	}
	t.Run("consumed request on disk", func(t *testing.T) {
		request := value.request
		request.AuthorizationConsumed = true
		request.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestFingerprint(request)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, requestPath, request)
		t.Cleanup(func() {
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, requestPath, requestRaw)
		})
		current := *value
		current.expected.AuthorizationRequestFingerprint = request.RequestFingerprint
		assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorOpenFailsWithoutOutputs(t, &current)
	})
	testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorRejectsInferredUnauthenticatedAndMismatchedAuthority(t, value)
	testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorRevalidatesSchedulingEvidence(t, value)
	testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorRejectsMalformedUnsafeOrConflictingOutputs(t, value)
}

func testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorRejectsInferredUnauthenticatedAndMismatchedAuthority(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture) {
	t.Helper()
	decisionPath := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionName)
	decisionRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, decisionPath)
	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision)
	}{
		{"inferred", func(d *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision) {
			d.ApprovalInferred = true
			d.InferenceSource = "scheduled_state"
		}},
		{"unauthenticated", func(d *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision) {
			d.IndependentlyAuthenticated = false
		}},
		{"non fixture owned", func(d *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision) {
			d.FixtureOwned = false
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			decision := value.decision
			test.mutate(&decision)
			decision.DecisionFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFingerprint(decision)
			if err := validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision(decision, value.expected.Policy, value.policy.receipt); err == nil {
				t.Fatal("inferred, unauthenticated, or non-fixture-owned decision was accepted")
			}
		})
	}

	for _, source := range []string{"scheduled_state", "dependency_release", "candidate_selection", "candidate_presence", "ordering", "readiness", "availability", "load", "risk", "cost", "score", "ranking", "recommendation", "matching", "graph_completion", "terminal_state", "lifecycle", "transition", "receipt", "connection", "lease", "broker", "provider", "forgepipe", "machine", "capability", "placement", "validation", "execution"} {
		t.Run("inference source "+source, func(t *testing.T) {
			decision := value.decision
			decision.ApprovalInferred = true
			decision.InferenceSource = source
			decision.DecisionFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFingerprint(decision)
			if err := validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision(decision, value.expected.Policy, value.policy.receipt); err == nil {
				t.Fatal("inferred authorization source was accepted")
			}
		})
	}
	t.Run("inferred decision on disk", func(t *testing.T) {
		decision := value.decision
		decision.ApprovalInferred = true
		decision.InferenceSource = "scheduled_state"
		decision.DecisionFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFingerprint(decision)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, decisionPath, decision)
		t.Cleanup(func() {
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, decisionPath, decisionRaw)
		})
		current := *value
		current.expected.AuthorizationDecisionFingerprint = decision.DecisionFingerprint
		assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorOpenFailsWithoutOutputs(t, &current)
	})
}

func testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorRevalidatesSchedulingEvidence(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture) {
	t.Helper()
	receiptRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.executor.receiptPath)
	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt)
	}{
		{"empty candidates", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) { r.Candidates = nil }},
		{"selected outside candidates", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.SelectedTaskID = "task-outside-candidates-001"
		}},
		{"duplicate selected candidate", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.Candidates = append(r.Candidates, r.Candidates[0])
		}},
		{"stale released postimage", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.SelectedCandidate.ReleasedPostimageVersion++
		}},
		{"non scheduled postimage", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.Postimage.State = "dependency_released"
		}},
		{"wrong transition count", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) { r.TransitionCount = 2 }},
		{"wrong write count", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) { r.RecordWriteCount = 2 }},
		{"unconsumed scheduling authorization", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.AuthorizationConsumed = false
		}},
		{"forbidden task launch authority", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.Evidence.TaskLaunch = true
		}},
		{"forbidden node execution authority", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.Evidence.NodeExecution = true
		}},
		{"forbidden callback authority", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.Evidence.Callback = true
		}},
		{"forbidden broker authority", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.Evidence.Broker = true
		}},
		{"forbidden provider authority", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.Evidence.Provider = true
		}},
		{"forbidden forgepipe authority", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.Evidence.ForgePipe = true
		}},
		{"forbidden network authority", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.Evidence.Network = true
		}},
		{"forbidden validation authority", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.Evidence.Validation = true
		}},
		{"forbidden checkout authority", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.Evidence.Checkout = true
		}},
		{"forbidden git authority", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) { r.Evidence.Git = true }},
	} {
		t.Run(test.name, func(t *testing.T) {
			receipt := value.policy.receipt
			test.mutate(&receipt)
			receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptFingerprint(receipt)
			if err := validateNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorSchedulingEvidence(value.request, receipt); err == nil {
				t.Fatal("stale, ambiguous, or authority-escalating scheduling evidence was accepted")
			}
		})
	}
	t.Run("authority-escalating scheduling receipt on disk", func(t *testing.T) {
		receipt := value.policy.receipt
		receipt.Evidence.TaskLaunch = true
		receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptFingerprint(receipt)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.policy.executor.receiptPath, receipt)
		t.Cleanup(func() {
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.executor.receiptPath, receiptRaw)
		})
		current := *value
		current.expected.Policy.SchedulingReceiptFingerprint = receipt.ReceiptFingerprint
		assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorOpenFailsWithoutOutputs(t, &current)
	})

	for _, test := range []struct {
		name   string
		mutate func(*nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture)
	}{
		{"missing scheduling receipt", func(v *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture) {
			mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, v.policy.executor.receiptPath)
		}},
		{"missing scheduled record", func(v *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture) {
			mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, v.policy.executor.selectedPath)
		}},
		{"stale scheduled record", func(v *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture) {
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, v.policy.executor.selectedPath, v.policy.executor.preimages[v.policy.executor.request.SelectedTaskID])
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			selectedRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.executor.selectedPath)
			t.Cleanup(func() {
				mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.executor.receiptPath, receiptRaw)
				mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.executor.selectedPath, selectedRaw)
			})
			test.mutate(value)
			assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorOpenFailsWithoutOutputs(t, value)
		})
	}
	testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorRevalidatesCompletePredecessorChainAndBindings(t, value)
}

func testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorRevalidatesCompletePredecessorChainAndBindings(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture) {
	t.Helper()
	expectedRaw, err := json.Marshal(value.expected)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected)
	}{
		{"authorization decision", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected) {
			e.AuthorizationDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')
		}},
		{"authorization request", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected) {
			e.AuthorizationRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		}},
		{"scheduling receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected) {
			e.Policy.SchedulingReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('3')
		}},
		{"scheduling request", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected) {
			e.Policy.Executor.PolicyRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('4')
		}},
		{"scheduling decision", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected) {
			e.Policy.Executor.PolicyDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('5')
		}},
		{"transition receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected) {
			e.Policy.Executor.Policy.TransitionReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('6')
		}},
		{"transition request", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected) {
			e.Policy.Executor.Policy.Executor.PolicyRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('7')
		}},
		{"lifecycle receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected) {
			e.Policy.Executor.Policy.Executor.Policy.AuditReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('8')
		}},
		{"projection", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected) {
			e.Policy.Executor.Policy.Executor.Policy.Executor.Policy.ProjectionDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('9')
		}},
		{"finalization", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected) {
			e.Policy.Executor.Policy.Executor.Policy.Executor.Policy.Projection.FinalizationDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('a')
		}},
		{"outcome", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected) {
			e.Policy.Executor.Policy.Executor.Policy.Executor.Policy.Projection.Finalization.Outcomes[0].ArtifactFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('b')
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			current := *value
			if err := json.Unmarshal(expectedRaw, &current.expected); err != nil {
				t.Fatal(err)
			}
			test.mutate(&current.expected)
			assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorOpenFailsWithoutOutputs(t, &current)
		})
	}
	if err := json.Unmarshal(expectedRaw, &value.expected); err != nil {
		t.Fatal(err)
	}
}

func testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorRejectsMalformedUnsafeOrConflictingOutputs(t *testing.T, attemptValue *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture) {
	t.Helper()
	attemptInputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs(attemptValue.root, attemptValue.expected)
	if err != nil {
		t.Fatal(err)
	}
	attemptRaw := mustMarshalNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorArtifact(t, deriveNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttempt(attemptInputs))
	tests := []struct {
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
			return bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorArtifactMaxBytes+1)
		}},
	}
	for _, test := range tests {
		t.Run("attempt "+test.name, func(t *testing.T) {
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, attemptValue.attemptPath, test.mutate(attemptRaw))
			t.Cleanup(func() {
				mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, attemptValue.attemptPath)
			})
			assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorOpenFailsWithoutReceipt(t, attemptValue)
		})
	}

	t.Run("symlinked attempt", func(t *testing.T) {
		value := attemptValue
		inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs(value.root, value.expected)
		if err != nil {
			t.Fatal(err)
		}
		target := value.attemptPath + ".target"
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, target, deriveNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttempt(inputs))
		if err := os.Symlink(target, value.attemptPath); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		t.Cleanup(func() {
			mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.attemptPath)
			mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, target)
		})
		assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorOpenFailsWithoutReceipt(t, value)
	})

	t.Run("conflicting attempt", func(t *testing.T) {
		value := attemptValue
		inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs(value.root, value.expected)
		if err != nil {
			t.Fatal(err)
		}
		attempt := deriveNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttempt(inputs)
		attempt.AttemptID = "launch-execution-attempt-conflict-001"
		attempt.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptFingerprint(attempt)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.attemptPath, attempt)
		t.Cleanup(func() {
			mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.attemptPath)
		})
		assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorOpenFailsWithoutReceipt(t, value)
	})

	mustExecuteNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(t, attemptValue)
	receiptRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, attemptValue.receiptPath)
	for _, test := range tests {
		t.Run("receipt "+test.name, func(t *testing.T) {
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, attemptValue.receiptPath, test.mutate(receiptRaw))
			t.Cleanup(func() {
				mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, attemptValue.receiptPath, receiptRaw)
			})
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(attemptValue.root, attemptValue.expected); err == nil {
				t.Fatal("malformed, noncanonical, oversized, or conflicting receipt was accepted")
			}
		})
	}
}

func testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReplayRestartConcurrencyAndExistingOutput(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture) {
	t.Helper()
	executor := mustOpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(t, value)
	first, err := executor.Execute()
	if err != nil {
		t.Fatal(err)
	}
	second, err := executor.Execute()
	if err != nil {
		t.Fatal(err)
	}
	restarted := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(t, value)
	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(first, restarted) {
		t.Fatal("exact replay, restart, or pre-existing identical output changed attempt evidence")
	}

	const callers = 12
	var wait sync.WaitGroup
	results := make(chan NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt, callers)
	errs := make(chan error, callers)
	for index := 0; index < callers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			current, openErr := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(value.root, value.expected)
			if openErr != nil {
				errs <- openErr
				return
			}
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
	for receipt := range results {
		if !reflect.DeepEqual(receipt, first) {
			t.Fatal("concurrent identical execution produced different evidence")
		}
	}

	valid := mustOpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(t, value)
	conflict := value.expected
	conflict.AuthorizationRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('c')
	wait = sync.WaitGroup{}
	errs = make(chan error, 2)
	wait.Add(2)
	go func() { defer wait.Done(); _, err := valid.Execute(); errs <- err }()
	go func() {
		defer wait.Done()
		executor, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(value.root, conflict)
		if err == nil {
			_, err = executor.Execute()
		}
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
		t.Fatalf("concurrent conflicting attempts produced successes=%d failures=%d", successes, failures)
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptThenReceiptRecovery(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture(t)
	executor := mustOpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(t, value)
	attemptWriter := executor.writeAttemptAtomic
	receiptWriter := executor.writeReceiptAtomic
	var attemptWrites atomic.Int32
	executor.writeAttemptAtomic = func(path string, payload any) error { attemptWrites.Add(1); return attemptWriter(path, payload) }
	executor.writeReceiptAtomic = func(string, any) error { return errors.New("injected receipt publication failure") }
	if _, err := executor.Execute(); err == nil {
		t.Fatal("receipt publication failure was accepted")
	}
	if attemptWrites.Load() != 1 {
		t.Fatal("attempt was not materialized exactly once before receipt failure")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorArtifactAbsent(t, value.receiptPath)
	restarted := mustOpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(t, value)
	restarted.writeAttemptAtomic = func(path string, payload any) error { attemptWrites.Add(1); return attemptWriter(path, payload) }
	restarted.writeReceiptAtomic = receiptWriter
	receipt, err := restarted.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if attemptWrites.Load() != 1 || receipt.AttemptCount != 1 || receipt.AttemptRecordWriteCount != 1 {
		t.Fatal("restart repeated the attempt instead of recovering the exact receipt")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorRejectsOrphanedAndAmbiguousPartialState(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture(t)
	t.Run("receipt without attempt", func(t *testing.T) {
		receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(t, value)
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.attemptPath)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.receiptPath, receipt)
		t.Cleanup(func() {
			mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.receiptPath)
		})
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(value.root, value.expected); err == nil {
			t.Fatal("receipt without its exact attempt record was accepted")
		}
	})
	t.Run("attempt without recoverable authorization", func(t *testing.T) {
		executor := mustOpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(t, value)
		executor.writeReceiptAtomic = func(string, any) error { return errors.New("injected receipt publication failure") }
		if _, err := executor.Execute(); err == nil {
			t.Fatal("receipt failure was accepted")
		}
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestName))
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(value.root, value.expected); err == nil {
			t.Fatal("attempt without recoverable exact authorization evidence was accepted")
		}
	})
}

func newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture(t *testing.T) *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture {
	t.Helper()
	template := &nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorSharedTestTemplate
	template.once.Do(func() {
		value := buildNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture(t)
		template.scheduling = *value.policy.executor
		template.policy = *value.policy
		template.policy.executor = &template.scheduling
		template.fixture = *value
		template.fixture.policy = &template.policy
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
	scheduling := template.scheduling
	scheduling.root = root
	scheduling.recordPaths = make(map[string]string, len(template.scheduling.recordPaths))
	for taskID, path := range template.scheduling.recordPaths {
		relative, err := filepath.Rel(template.fixture.root, path)
		if err != nil {
			t.Fatal(err)
		}
		scheduling.recordPaths[taskID] = filepath.Join(root, relative)
	}
	selectedRelative, err := filepath.Rel(template.fixture.root, template.scheduling.selectedPath)
	if err != nil {
		t.Fatal(err)
	}
	scheduling.selectedPath = filepath.Join(root, selectedRelative)
	scheduling.receiptPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptName)
	policy := template.policy
	policy.root, policy.executor = root, &scheduling
	value := template.fixture
	value.root, value.policy = root, &policy
	value.attemptPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecordName)
	value.receiptPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceiptName)
	return &value
}

func buildNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture(t *testing.T) *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture {
	t.Helper()
	policy := newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture(t, "approved")
	decision, requestPointer := mustDecideNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(t, policy)
	if requestPointer == nil {
		t.Fatal("approved launch/execution authorization did not produce a request")
	}
	request := *requestPointer
	return &nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture{
		root: policy.root, policy: policy, decision: decision, request: request,
		expected:    NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorExpected{Policy: policy.expected, AuthorizationDecisionFingerprint: decision.DecisionFingerprint, AuthorizationRequestFingerprint: request.RequestFingerprint},
		attemptPath: filepath.Join(policy.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecordName),
		receiptPath: filepath.Join(policy.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceiptName),
	}
}

func mustOpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture) *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor {
	t.Helper()
	executor, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return executor
}

func mustExecuteNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture) NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt {
	t.Helper()
	receipt, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(t, value).Execute()
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func mustLoadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttempt(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture) NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecord {
	t.Helper()
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorInputs(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	if !inputs.attemptExists {
		t.Fatal("attempt record missing")
	}
	return inputs.attempt
}

func mustMarshalNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorArtifact(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorOpenFailsWithoutOutputs(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture) {
	t.Helper()
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(value.root, value.expected); err == nil {
		t.Fatal("invalid launch/execution executor input was accepted")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorArtifactsAbsent(t, value.root)
}

func assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorOpenFailsWithoutReceipt(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture) {
	t.Helper()
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutor(value.root, value.expected); err == nil {
		t.Fatal("invalid launch/execution executor artifact was accepted")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorArtifactAbsent(t, value.receiptPath)
}

func assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorArtifactsAbsent(t *testing.T, root string) {
	t.Helper()
	assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorArtifactAbsent(t, filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecordName))
	assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorArtifactAbsent(t, filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceiptName))
}

func assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorArtifactAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("executor unexpectedly published %s", path)
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorOnlyOutputsChanged(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture, before map[string][]byte) {
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
	want := []string{nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorAttemptRecordName, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceiptName}
	sort.Strings(want)
	if !reflect.DeepEqual(changed, want) {
		t.Fatalf("executor changed forbidden predecessor or state artifacts: got %v want %v", changed, want)
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorNarrowEvidence(t *testing.T, receipt NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorReceipt) {
	t.Helper()
	if receipt.Evidence != (NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorEvidence{LocalAttemptMaterialized: true}) {
		t.Fatal("executor widened or omitted its sole local-attempt evidence")
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"task_process":true`, `"task_launch":true`, `"node_execution":true`, `"node_execution_receipt":true`, `"successful_task_outcome":true`, `"graph_progress":true`, `"placement":true`, `"dispatch":true`, `"connector":true`, `"broker":true`, `"provider":true`, `"forgepipe":true`, `"callback":true`, `"external_action":true`, `"network":true`, `"remote_execution":true`, `"validation":true`, `"checkout_mutation":true`, `"git":true`, `"retry":true`, `"repair":true`, `"cancellation":true`, `"publication":true`, `"lifecycle_transition":true`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("executor receipt escalated or implied forbidden activity: %s", forbidden)
		}
	}
}

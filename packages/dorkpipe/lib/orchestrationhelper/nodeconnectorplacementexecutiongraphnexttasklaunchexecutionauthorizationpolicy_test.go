package orchestrationhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

type nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture struct {
	root      string
	executor  *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture
	receipt   NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt
	expected  NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected
	fixture   NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture
	allBefore map[string][]byte
}

func TestNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyApprovedProducesExactRequest(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture(t, "approved")
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(t, value)
	if decision.Decision != "approved" || request == nil || request.RequestID != value.expected.AuthorizationRequestID {
		t.Fatal("approved launch/execution authorization did not produce its exact request")
	}
	if !request.OneTimeRequest || request.AuthorizationConsumed || request.TaskLaunchInvoked || request.NodeExecutionInvoked || request.CallbacksInvoked || request.ExternalActionsInvoked || !request.FixtureOwned || !decision.IndependentlyAuthenticated || decision.ApprovalInferred {
		t.Fatal("launch/execution authorization request was not independent, fixture-owned, and unconsumed")
	}
	expectedBinding := nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyBinding(value.receipt)
	if !reflect.DeepEqual(decision.Binding, expectedBinding) || !reflect.DeepEqual(request.Binding, expectedBinding) || request.DecisionFingerprint != decision.DecisionFingerprint {
		t.Fatal("authorization artifacts omitted the exact scheduling receipt, selected task, scheduled postimage, policy, or authentication binding")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyNarrowAuthority(t, decision, request)
	assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRecordsUnchanged(t, value)
}

func TestNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRejectedProducesNoRequest(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture(t, "rejected")
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(t, value)
	if decision.Decision != "rejected" || request != nil || decision.AuthorizationRequestID != "" || decision.Authority != (NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyAuthority{}) {
		t.Fatal("rejected launch/execution authorization emitted a request or authority")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyArtifactAbsent(t, value.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestName)
	assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRecordsUnchanged(t, value)
}

func TestNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRejectsBindingAndInferenceConflicts(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture(t, "approved")
	tests := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture)
	}{
		{"graph", func(f *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) {
			f.Binding.GraphRunID = "graph-run-conflict-001"
		}},
		{"terminal task", func(f *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) {
			f.Binding.TerminalTaskID = "terminal-task-conflict-001"
		}},
		{"route", func(f *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) {
			f.Binding.Route = "failure_propagation_transition"
		}},
		{"scheduling receipt", func(f *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) {
			f.Binding.SchedulingReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')
		}},
		{"transition receipt", func(f *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) {
			f.Binding.TransitionReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		}},
		{"candidate set", func(f *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) {
			f.Binding.CandidatesFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('3')
		}},
		{"selected task", func(f *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) {
			f.Binding.SelectedTaskID = "dependency-conflict-001"
		}},
		{"released postimage", func(f *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) {
			f.Binding.SelectedReleasedDependencyPostimage.ReleasedPostimageFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('4')
		}},
		{"scheduled record", func(f *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) {
			f.Binding.SchedulingRecordPostimageFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('5')
		}},
		{"scheduled record version", func(f *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) {
			f.Binding.SchedulingRecordPostimageVersion++
		}},
		{"scheduling policy decision", func(f *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) {
			f.Binding.SchedulingPolicyDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('6')
		}},
		{"scheduling policy request", func(f *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) {
			f.Binding.SchedulingPolicyRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('7')
		}},
		{"scheduling authentication", func(f *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) {
			f.Binding.SchedulingPolicyAuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('8')
		}},
		{"decision authentication", func(f *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) {
			f.AuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('9')
		}},
		{"request identity", func(f *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) {
			f.AuthorizationRequestID = "launch-execution-request-conflict-001"
		}},
		{"authority escalation", func(f *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) {
			f.Authority.TaskLaunch = true
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := value.fixture
			test.mutate(&fixture)
			if _, _, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(t, value).Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(t, fixture)); err == nil {
				t.Fatal("mismatched authorization binding was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyArtifactsAbsent(t, value.root)
			assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRecordsUnchanged(t, value)
		})
	}

	for _, source := range []string{"scheduled_state", "dependency_release", "candidate_presence", "ordering", "readiness", "availability", "load", "risk", "cost", "score", "ranking", "recommendation", "matching", "graph_completion", "terminal_state", "lifecycle", "transition_receipt", "connection", "lease", "broker", "provider", "forgepipe", "machine", "capability", "placement", "validation", "execution_receipt"} {
		t.Run("inferred from "+source, func(t *testing.T) {
			fixture := value.fixture
			fixture.ApprovalInferred = true
			fixture.InferenceSource = source
			if _, _, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(t, value).Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(t, fixture)); err == nil {
				t.Fatalf("launch/execution approval was inferred from %s", source)
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyArtifactsAbsent(t, value.root)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRevalidatesSchedulingEvidence(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture(t, "approved")
	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt)
	}{
		{"empty candidates", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) { r.Candidates = nil }},
		{"selected outside candidates", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.SelectedTaskID = "dependency-absent-001"
		}},
		{"duplicate selected candidate", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.Candidates = append(r.Candidates, r.Candidates[0])
		}},
		{"stale released postimage", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.SelectedCandidate.ReleasedPostimageFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('a')
		}},
		{"wrong transition count", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) { r.TransitionCount = 2 }},
		{"wrong write count", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) { r.RecordWriteCount = 2 }},
		{"unconsumed scheduling authorization", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.AuthorizationConsumed = false
		}},
		{"non-scheduled postimage", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
			r.Postimage.State = "dependency_released"
		}},
		{"forbidden launch authority", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
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
		{"forbidden git authority", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) { r.Evidence.Git = true }},
	} {
		t.Run(test.name, func(t *testing.T) {
			receipt := value.receipt
			test.mutate(&receipt)
			receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptFingerprint(receipt)
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.executor.receiptPath, receipt)
			defer mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.executor.receiptPath, value.receipt)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(value.root, value.expected); err == nil {
				t.Fatal("invalid, stale, ambiguous, or authority-escalated scheduling evidence was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyArtifactsAbsent(t, value.root)
		})
	}

	t.Run("missing scheduling receipt", func(t *testing.T) {
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.executor.receiptPath)
		defer mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.executor.receiptPath, value.receipt)
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(value.root, value.expected); err == nil {
			t.Fatal("missing scheduling receipt was accepted")
		}
	})
	t.Run("missing scheduled record", func(t *testing.T) {
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.executor.selectedPath)
		defer mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.executor.selectedPath, value.receipt.Postimage)
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(value.root, value.expected); err == nil {
			t.Fatal("missing scheduled postimage was accepted")
		}
	})
	t.Run("stale scheduled record", func(t *testing.T) {
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.executor.selectedPath, value.executor.preimages[value.executor.request.SelectedTaskID])
		defer mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.executor.selectedPath, value.receipt.Postimage)
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(value.root, value.expected); err == nil {
			t.Fatal("stale non-scheduled record was accepted")
		}
	})
}

func TestNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRevalidatesCompletePredecessorChain(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture(t, "approved")
	tests := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected)
	}{
		{"scheduling receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected) {
			e.SchedulingReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('b')
		}},
		{"scheduling request", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected) {
			e.Executor.PolicyRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('c')
		}},
		{"scheduling decision", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected) {
			e.Executor.PolicyDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('d')
		}},
		{"transition receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected) {
			e.Executor.Policy.TransitionReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('e')
		}},
		{"transition request", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected) {
			e.Executor.Policy.Executor.PolicyRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('f')
		}},
		{"lifecycle receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected) {
			e.Executor.Policy.Executor.Policy.AuditReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')
		}},
		{"projection", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected) {
			e.Executor.Policy.Executor.Policy.Executor.Policy.ProjectionDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		}},
		{"finalization", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected) {
			e.Executor.Policy.Executor.Policy.Executor.Policy.Projection.FinalizationDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('3')
		}},
		{"outcome", func(e *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected) {
			e.Executor.Policy.Executor.Policy.Executor.Policy.Projection.Finalization.Outcomes[0].ArtifactFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('4')
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := value.expected
			test.mutate(&expected)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(value.root, expected); err == nil {
				t.Fatal("changed immutable predecessor binding was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyArtifactsAbsent(t, value.root)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRejectsMalformedUnsafeAndConflictingArtifacts(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture(t, "approved")
	for _, test := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"empty", func([]byte) []byte { return nil }},
		{"malformed", func([]byte) []byte { return []byte("{") }},
		{"unknown", func(raw []byte) []byte { return bytes.Replace(raw, []byte("}"), []byte(",\"unknown\":true}"), 1) }},
		{"trailing", func(raw []byte) []byte { return append(append([]byte(nil), raw...), []byte("{}")...) }},
		{"oversized", func([]byte) []byte {
			return bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionMaxBytes+1)
		}},
		{"noncanonical", func(raw []byte) []byte {
			var decoded any
			_ = json.Unmarshal(raw, &decoded)
			pretty, _ := json.MarshalIndent(decoded, "", "  ")
			return pretty
		}},
	} {
		t.Run("fixture "+test.name, func(t *testing.T) {
			raw := test.mutate(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(t, value.fixture))
			if _, _, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(t, value).Decide(raw); err == nil {
				t.Fatal("malformed, noncanonical, oversized, or ambiguous fixture was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyArtifactsAbsent(t, value.root)
		})
	}

	t.Run("symlinked request", func(t *testing.T) {
		_, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(t, value)
		path := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestName)
		target := path + ".target"
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, target, request)
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, path)
		if err := os.Symlink(target, path); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(value.root, value.expected); err == nil {
			t.Fatal("symlinked authorization request was accepted")
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, path, request)
	})

	t.Run("orphaned request", func(t *testing.T) {
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionName))
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(value.root, value.expected); err == nil {
			t.Fatal("orphaned authorization request was accepted")
		}
	})
}

func TestNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRejectsConsumedOrEscalatedRequests(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture(t, "approved")
	_, acceptedRequest := mustDecideNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(t, value)
	requestPath := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestName)
	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest)
	}{
		{"authorization consumed", func(r *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
			r.AuthorizationConsumed = true
		}},
		{"task launch invoked", func(r *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
			r.TaskLaunchInvoked = true
		}},
		{"node execution invoked", func(r *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
			r.NodeExecutionInvoked = true
		}},
		{"callback invoked", func(r *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
			r.CallbacksInvoked = true
		}},
		{"external action invoked", func(r *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
			r.ExternalActionsInvoked = true
		}},
		{"authority escalated", func(r *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
			r.Authority.Broker = true
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := *acceptedRequest
			test.mutate(&request)
			request.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestFingerprint(request)
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, requestPath, request)
			defer mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, requestPath, *acceptedRequest)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(value.root, value.expected); err == nil {
				t.Fatal("consumed, already-invoked, or authority-escalated request was accepted")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyReplayRestartConcurrencyAndConflicts(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture(t, "approved")
	raw := mustMarshalNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(t, value.fixture)
	const callers = 12
	instances := make([]*NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies, callers)
	for index := range instances {
		instances[index] = mustOpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(t, value)
	}
	var wait sync.WaitGroup
	type result struct {
		decision NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision
		request  *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest
		err      error
	}
	results := make(chan result, callers)
	for _, policies := range instances {
		wait.Add(1)
		go func(policies *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies) {
			defer wait.Done()
			decision, request, err := policies.Decide(raw)
			results <- result{decision: decision, request: request, err: err}
		}(policies)
	}
	wait.Wait()
	close(results)
	var firstDecision NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision
	var firstRequest *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest
	for current := range results {
		if current.err != nil {
			t.Fatal(current.err)
		}
		if firstRequest == nil {
			firstDecision, firstRequest = current.decision, current.request
		} else if !reflect.DeepEqual(firstDecision, current.decision) || !reflect.DeepEqual(firstRequest, current.request) {
			t.Fatal("independently opened concurrent identical attempts produced different authorization artifacts")
		}
	}
	policies := instances[0]
	secondDecision, secondRequest := mustDecideNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyWithPolicies(t, policies, value.fixture)
	restartedDecision, restartedRequest := mustDecideNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyWithPolicies(t, mustOpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(t, value), value.fixture)
	if !reflect.DeepEqual(firstDecision, secondDecision) || !reflect.DeepEqual(firstDecision, restartedDecision) || !reflect.DeepEqual(firstRequest, secondRequest) || !reflect.DeepEqual(firstRequest, restartedRequest) {
		t.Fatal("exact replay, restart, or pre-existing identical artifacts changed authorization")
	}

	conflict := value.fixture
	conflict.DecisionID = "launch-execution-decision-conflict-001"
	var conflictWait sync.WaitGroup
	conflictErrs := make(chan error, 2)
	conflictWait.Add(2)
	go func() { defer conflictWait.Done(); _, _, err := policies.Decide(raw); conflictErrs <- err }()
	go func() {
		defer conflictWait.Done()
		_, _, err := policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(t, conflict))
		conflictErrs <- err
	}()
	conflictWait.Wait()
	close(conflictErrs)
	successes, failures := 0, 0
	for err := range conflictErrs {
		if err == nil {
			successes++
		} else {
			failures++
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("concurrent conflicting decisions produced successes=%d failures=%d", successes, failures)
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionThenRequestRecovery(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture(t, "approved")
	original := nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyWriteRequestAtomic
	nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyWriteRequestAtomic = func(string, any) error { return errors.New("injected request publication failure") }
	t.Cleanup(func() {
		nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyWriteRequestAtomic = original
	})
	if _, _, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(t, value).Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(t, value.fixture)); err == nil {
		t.Fatal("request publication failure was accepted")
	}
	if _, err := os.Lstat(filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionName)); err != nil {
		t.Fatal("durable authorization decision was lost")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyArtifactAbsent(t, value.root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestName)
	nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyWriteRequestAtomic = original
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyWithPolicies(t, mustOpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(t, value), value.fixture)
	assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyNarrowAuthority(t, decision, request)
	assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRecordsUnchanged(t, value)
}

func newNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture(t *testing.T, decision string) *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture {
	t.Helper()
	executor := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t)
	receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(t, executor)
	expected := NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyExpected{
		Executor: executor.expected, SchedulingReceiptFingerprint: receipt.ReceiptFingerprint,
		DecisionAuthenticationID: "launch-execution-authentication-001", DecisionAuthenticationDigest: "sha256:abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		AuthorizationRequestID: "launch-execution-request-001",
	}
	fixture := NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture{
		Schema:     NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixtureSchema,
		DecisionID: "launch-execution-decision-001", ReplayIdentity: "launch-execution-replay-001",
		AuthenticationID: expected.DecisionAuthenticationID, AuthenticationDigest: expected.DecisionAuthenticationDigest,
		Decision: decision, Binding: nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyBinding(receipt),
		Provenance: "fixture_only_local_task_launch_new_node_execution_authorization_policy_decision",
	}
	if decision == "approved" {
		fixture.AuthorizationRequestID = expected.AuthorizationRequestID
		fixture.Authority.TaskLaunchNewNodeExecutionExecutorAttempt = true
	}
	value := &nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture{root: executor.root, executor: executor, receipt: receipt, expected: expected, fixture: fixture, allBefore: make(map[string][]byte)}
	for _, path := range executor.recordPaths {
		value.allBefore[path] = mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path)
	}
	return value
}

func mustOpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture) *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies {
	t.Helper()
	policies, err := OpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return policies
}

func mustDecideNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture) (NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
	t.Helper()
	return mustDecideNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyWithPolicies(t, mustOpenNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies(t, value), value.fixture)
}

func mustDecideNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyWithPolicies(t *testing.T, policies *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicies, fixture NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) (NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
	t.Helper()
	decision, request, err := policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}

func mustMarshalNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicy(t *testing.T, value NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionFixture) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyArtifactsAbsent(t *testing.T, root string) {
	t.Helper()
	assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyArtifactAbsent(t, root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecisionName)
	assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyArtifactAbsent(t, root, nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequestName)
}

func assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyArtifactAbsent(t *testing.T, root, name string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
		t.Fatalf("authorization policy unexpectedly published %s", name)
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRecordsUnchanged(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture) {
	t.Helper()
	for path, before := range value.allBefore {
		if !bytes.Equal(before, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path)) {
			t.Fatalf("authorization policy mutated scheduling state %s", path)
		}
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyNarrowAuthority(t *testing.T, decision NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision, request *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest) {
	t.Helper()
	if decision.Authority != (NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyAuthority{}) || request == nil || request.Authority != (NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyAuthority{TaskLaunchNewNodeExecutionExecutorAttempt: true}) {
		t.Fatal("authorization policy widened or omitted its sole future executor-attempt authority")
	}
	raw, err := json.Marshal(struct {
		Decision NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyDecision `json:"decision"`
		Request  *NodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyRequest `json:"request"`
	}{decision, request})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"scheduling_mutation":true`, `"task_launch":true`, `"node_execution":true`, `"placement":true`, `"dispatch":true`, `"connector":true`, `"broker":true`, `"provider":true`, `"forgepipe":true`, `"retry":true`, `"repair":true`, `"cancellation":true`, `"publication":true`, `"callback":true`, `"external_action":true`, `"remote_execution":true`, `"network":true`, `"validation":true`, `"checkout":true`, `"git":true`, `"commit":true`, `"push":true`, `"authorization_consumed":true`, `"task_launch_invoked":true`, `"node_execution_invoked":true`, `"callbacks_invoked":true`, `"external_actions_invoked":true`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("authorization policy escalated launch, execution, callback, or external authority: %s", forbidden)
		}
	}
}

func testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint(value byte) string {
	return "sha256:" + string(bytes.Repeat([]byte{value}, 64))
}

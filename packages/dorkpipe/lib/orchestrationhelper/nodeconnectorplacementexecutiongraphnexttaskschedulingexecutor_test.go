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

type nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture struct {
	root         string
	policy       *nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture
	decision     NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision
	request      NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest
	expected     NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorExpected
	recordPaths  map[string]string
	preimages    map[string]NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord
	selectedPath string
	receiptPath  string
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorSchedulesOnlyExactSelectedTask(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t)
	dependencyBefore := snapshotNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRecords(t, value.policy)
	unselectedBefore := snapshotNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorUnselected(t, value)
	receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(t, value)
	selected := mustLoadNodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord(t, value.root, value.selectedPath)
	if selected.State != "scheduled" || selected.TaskID != value.request.SelectedTaskID || selected.SchedulingRequestID != value.request.RequestID || selected.SchedulingRequestFingerprint != value.request.RequestFingerprint {
		t.Fatal("executor did not perform the exact selected dependency_released-to-scheduled transition")
	}
	if receipt.SelectedTaskID != value.request.SelectedTaskID || receipt.SelectedCandidate.ReleasedPostimageFingerprint != selected.ReleasedDependencyPostimageFingerprint || receipt.SelectedCandidate.ReleasedPostimageVersion != selected.ReleasedDependencyPostimageVersion || receipt.SchedulingTransition != "dependency_released_to_scheduled" {
		t.Fatal("scheduling evidence omitted the exact selected task or released dependency postimage")
	}
	if receipt.TransitionCount != 1 || receipt.RecordWriteCount != 1 || !receipt.AuthorizationConsumed || !receipt.FixtureOwned || receipt.PreimageVersion+1 != receipt.PostimageVersion {
		t.Fatal("scheduling evidence did not record one exact consumed local transition and write")
	}
	if !reflect.DeepEqual(receipt.Transitions, value.request.Binding.Transitions) || receipt.TransitionsFingerprint != value.request.Binding.TransitionsFingerprint || !reflect.DeepEqual(receipt.Candidates, value.request.Candidates) || receipt.CandidatesFingerprint != value.request.CandidatesFingerprint {
		t.Fatal("scheduling evidence omitted the complete transition postimages or candidate set")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorUnselectedEqual(t, value, unselectedBefore)
	assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRecordsEqual(t, value.policy, dependencyBefore)
	assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorNarrowEvidence(t, receipt)
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorRejectsMissingRejectedAndFailurePolicy(t *testing.T) {
	t.Run("missing request", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t)
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestName))
		assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorOpenFailsWithoutTransition(t, value)
	})
	for _, test := range []struct{ name, terminal string }{{"rejected decision", "succeeded"}, {"failure propagation", "failed"}} {
		t.Run(test.name, func(t *testing.T) {
			policy := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture(t, test.terminal, "rejected")
			decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(t, policy)
			if request != nil {
				t.Fatal("rejected or failure policy unexpectedly emitted a scheduling request")
			}
			expected := NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorExpected{Policy: policy.expected, PolicyDecisionFingerprint: decision.DecisionFingerprint}
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(policy.root, expected); err == nil {
				t.Fatal("rejected decision or failure-propagation route was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptAbsent(t, policy.root)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorRejectsConsumedEscalatedAndMismatchedRequests(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t)

	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest)
	}{
		{"authorization consumed", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) {
			r.AuthorizationConsumed = true
		}},
		{"scheduling already invoked", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) {
			r.SchedulingInvoked = true
		}},
		{"task already launched", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) { r.TaskLaunched = true }},
		{"callbacks already invoked", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) {
			r.CallbacksInvoked = true
		}},
		{"external actions already invoked", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) {
			r.ExternalActionsInvoked = true
		}},
		{"not one time", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) { r.OneTimeRequest = false }},
		{"launch authority", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) {
			r.Authority.TaskLaunch = true
		}},
		{"selected outside candidates", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) {
			r.SelectedTaskID = "task-outside-candidates-001"
		}},
		{"empty candidate set", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) {
			r.Candidates = nil
			r.CandidatesFingerprint, _ = nodeExecutionFingerprintValue(r.Candidates)
		}},
		{"changed authentication", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) {
			r.AuthenticationDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{"changed route", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) {
			r.Binding.Route = "failure_propagation_transition"
		}},
		{"changed transition receipt", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) {
			r.Binding.TransitionReceiptFingerprint = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
		{"changed dependency postimage", func(r *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) {
			r.Candidates[0].ReleasedPostimageVersion++
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			caseValue := *value
			request := cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest(value.request)
			test.mutate(&request)
			request.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestFingerprint(request)
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestName), request)
			caseValue.expected.PolicyRequestFingerprint = request.RequestFingerprint
			assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorOpenFailsWithoutTransition(t, &caseValue)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorDoesNotInferSelectionOrAuthority(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t)

	for _, source := range []string{"dependency release", "candidate presence", "ordering", "readiness", "availability", "load", "risk", "cost", "ranking", "matching", "connection", "provider", "broker", "forgepipe", "lifecycle", "receipt"} {
		t.Run(source, func(t *testing.T) {
			caseValue := *value
			decision := cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision(value.decision)
			decision.ApprovalInferred = true
			decision.InferenceSource = source
			decision.DecisionFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFingerprint(decision)
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionName), decision)
			caseValue.expected.PolicyDecisionFingerprint = decision.DecisionFingerprint
			assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorOpenFailsWithoutTransition(t, &caseValue)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorRevalidatesPredecessorsAndReleasedPostimages(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t)
	expectedRaw, err := json.Marshal(value.expected)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		mutate func(*nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture)
	}{
		{"policy decision fingerprint", func(v *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) {
			v.expected.PolicyDecisionFingerprint = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
		}},
		{"policy request fingerprint", func(v *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) {
			v.expected.PolicyRequestFingerprint = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
		}},
		{"transition receipt fingerprint", func(v *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) {
			v.expected.Policy.TransitionReceiptFingerprint = "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		}},
		{"transition request fingerprint", func(v *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) {
			v.expected.Policy.Executor.PolicyRequestFingerprint = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		}},
		{"lifecycle receipt fingerprint", func(v *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) {
			v.expected.Policy.Executor.Policy.AuditReceiptFingerprint = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
		}},
		{"terminal task", func(v *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) {
			v.expected.Policy.TerminalTaskID = "terminal-task-conflict-001"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			caseValue := *value
			if err := json.Unmarshal(expectedRaw, &caseValue.expected); err != nil {
				t.Fatal(err)
			}
			test.mutate(&caseValue)
			assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorOpenFailsWithoutTransition(t, &caseValue)
		})
	}

	t.Run("stale dependency postimage", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.policy.recordPaths[0], value.policy.executor.preimages[0])
		assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorOpenFailsWithoutTransition(t, value)
	})
	t.Run("tampered transition receipt", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t)
		path := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptName)
		raw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path)
		mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path, bytes.Replace(raw, []byte(`"route": "dependency_release_transition"`), []byte(`"route": "failure_propagation_transition"`), 1))
		assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorOpenFailsWithoutTransition(t, value)
	})
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorRejectsInvalidSchedulingRecords(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t)

	for _, test := range []struct {
		name   string
		mutate func(*testing.T, *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture)
	}{
		{"missing", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) {
			mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, v.selectedPath)
		}},
		{"malformed", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) {
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, v.selectedPath, []byte("{"))
		}},
		{"noncanonical", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) {
			raw, _ := json.Marshal(v.preimages[v.request.SelectedTaskID])
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, v.selectedPath, raw)
		}},
		{"oversized", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) {
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, v.selectedPath, bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifactMaxBytes+1))
		}},
		{"tampered fingerprint", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) {
			record := v.preimages[v.request.SelectedTaskID]
			record.ReleasedDependencyPostimageFingerprint = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
			record.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskSchedulingRecordFingerprint(record)
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, v.selectedPath, record)
		}},
		{"conflicting scheduled output", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) {
			record := v.preimages[v.request.SelectedTaskID]
			record.State = "scheduled"
			record.Version++
			record.PreviousRecordFingerprint = record.RecordFingerprint
			record.SchedulingRequestID = "scheduling-request-conflict-001"
			record.SchedulingRequestFingerprint = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
			record.RecordFingerprint = ""
			record.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskSchedulingRecordFingerprint(record)
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, v.selectedPath, record)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.selectedPath, value.preimages[value.request.SelectedTaskID])
			test.mutate(t, value)
			assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorOpenFailsWithoutTransition(t, value)
		})
	}

	t.Run("symlink", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t)
		target := value.selectedPath + ".target"
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, target, value.preimages[value.request.SelectedTaskID])
		mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.selectedPath)
		if err := os.Symlink(target, value.selectedPath); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorOpenFailsWithoutTransition(t, value)
	})
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReplayRestartConcurrencyAndExistingOutput(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t)
	executor := mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(t, value)
	first, err := executor.Execute()
	if err != nil {
		t.Fatal(err)
	}
	second, err := executor.Execute()
	if err != nil {
		t.Fatal(err)
	}
	restarted := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(t, value)
	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(first, restarted) {
		t.Fatal("exact replay or restart changed durable scheduling evidence")
	}

	const callers = 12
	var wait sync.WaitGroup
	results := make(chan NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt, callers)
	errs := make(chan error, callers)
	for index := 0; index < callers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			receipt, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorNoFatal(value).Execute()
			results <- receipt
			errs <- err
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
			t.Fatal("concurrent identical attempt produced different evidence")
		}
	}

	t.Run("pre-existing identical postimage", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t)
		_, postimage, err := deriveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorRecords(value.request, mustSelectedNodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate(t, value.request))
		if err != nil {
			t.Fatal(err)
		}
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.selectedPath, postimage)
		receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(t, value)
		if receipt.PostimageFingerprint != postimage.RecordFingerprint {
			t.Fatal("pre-existing identical same-request output was not recovered")
		}
	})
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorConcurrentConflictFailsClosed(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t)
	valid := mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorNoFatal(value)
	conflictingExpected := value.expected
	conflictingExpected.PolicyRequestFingerprint = "sha256:4444444444444444444444444444444444444444444444444444444444444444"
	var wait sync.WaitGroup
	errs := make(chan error, 2)
	wait.Add(2)
	go func() { defer wait.Done(); _, err := valid.Execute(); errs <- err }()
	go func() {
		defer wait.Done()
		executor, err := OpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(value.root, conflictingExpected)
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

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorAtomicFailureRecoveryAndConflictingReceipt(t *testing.T) {
	t.Run("record write failure", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t)
		before := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.selectedPath)
		original := nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorWriteRecordAtomic
		nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorWriteRecordAtomic = func(string, any) error { return errors.New("injected record failure") }
		t.Cleanup(func() { nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorWriteRecordAtomic = original })
		if _, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(t, value).Execute(); err == nil {
			t.Fatal("record write failure was accepted")
		}
		if !bytes.Equal(before, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.selectedPath)) {
			t.Fatal("failed scheduling write changed the preimage")
		}
		assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptAbsent(t, value.root)
	})

	t.Run("receipt publication recovery", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t)
		var writes atomic.Int32
		recordWriter := nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorWriteRecordAtomic
		receiptWriter := nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorWriteReceiptAtomic
		nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorWriteRecordAtomic = func(path string, payload any) error { writes.Add(1); return recordWriter(path, payload) }
		nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorWriteReceiptAtomic = func(string, any) error { return errors.New("injected receipt failure") }
		t.Cleanup(func() {
			nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorWriteRecordAtomic = recordWriter
			nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorWriteReceiptAtomic = receiptWriter
		})
		if _, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(t, value).Execute(); err == nil {
			t.Fatal("receipt publication failure was accepted")
		}
		if writes.Load() != 1 {
			t.Fatal("scheduling transition was not written exactly once")
		}
		nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorWriteReceiptAtomic = receiptWriter
		mustExecuteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(t, value)
		if writes.Load() != 1 {
			t.Fatal("restart repeated a completed scheduling transition")
		}
	})

	t.Run("conflicting receipt", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t)
		mustExecuteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(t, value)
		raw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.receiptPath)
		mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.receiptPath, bytes.Replace(raw, []byte(`"record_write_count": 1`), []byte(`"record_write_count": 2`), 1))
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(value.root, value.expected); err == nil {
			t.Fatal("conflicting durable scheduling receipt was accepted")
		}
	})
}

func newNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture(t *testing.T) *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture {
	t.Helper()
	policy := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture(t, "succeeded", "approved")
	decision, requestPointer := mustDecideNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(t, policy)
	if requestPointer == nil {
		t.Fatal("approved scheduling policy did not produce a request")
	}
	request := *requestPointer
	value := &nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture{
		root: policy.root, policy: policy, decision: decision, request: request,
		expected:    NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorExpected{Policy: policy.expected, PolicyDecisionFingerprint: decision.DecisionFingerprint, PolicyRequestFingerprint: request.RequestFingerprint},
		recordPaths: make(map[string]string), preimages: make(map[string]NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord),
		receiptPath: filepath.Join(policy.root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptName),
	}
	for _, candidate := range request.Candidates {
		candidateRequest := request
		candidateRequest.SelectedTaskID = candidate.TaskID
		preimage, _, err := deriveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorRecords(candidateRequest, candidate)
		if err != nil {
			t.Fatal(err)
		}
		path, err := nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorRecordPath(value.root, request.Binding.PolicyBinding.GraphStoreID, candidate.DependencyRecordID)
		if err != nil {
			t.Fatal(err)
		}
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, path, preimage)
		value.recordPaths[candidate.TaskID], value.preimages[candidate.TaskID] = path, preimage
		if candidate.TaskID == request.SelectedTaskID {
			value.selectedPath = path
		}
	}
	return value
}

func mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor {
	t.Helper()
	executor, err := OpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return executor
}

func mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorNoFatal(value *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) *NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor {
	executor, _ := OpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(value.root, value.expected)
	return executor
}

func mustExecuteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt {
	t.Helper()
	receipt, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(t, value).Execute()
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func mustSelectedNodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate(t *testing.T, request NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate {
	t.Helper()
	for _, candidate := range request.Candidates {
		if candidate.TaskID == request.SelectedTaskID {
			return candidate
		}
	}
	t.Fatal("selected candidate missing")
	return NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate{}
}

func mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t *testing.T, path string, value any) {
	t.Helper()
	if err := writeJSONFileAtomic(path, value); err != nil {
		t.Fatal(err)
	}
}

func mustLoadNodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord(t *testing.T, root, path string) NodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord {
	t.Helper()
	record, err := loadNodeConnectorPlacementExecutionGraphNextTaskSchedulingRecord(root, path)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func snapshotNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorUnselected(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) map[string][]byte {
	t.Helper()
	result := make(map[string][]byte)
	for taskID, path := range value.recordPaths {
		if taskID != value.request.SelectedTaskID {
			result[path] = mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path)
		}
	}
	return result
}

func assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorUnselectedEqual(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture, expected map[string][]byte) {
	t.Helper()
	for path, raw := range expected {
		if !bytes.Equal(raw, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path)) {
			t.Fatalf("executor changed unselected scheduling record %s", path)
		}
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorOpenFailsWithoutTransition(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture) {
	t.Helper()
	before, err := os.ReadFile(value.selectedPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutor(value.root, value.expected); err == nil {
		t.Fatal("invalid scheduling executor input was accepted")
	}
	after, afterErr := os.ReadFile(value.selectedPath)
	if err == nil && (afterErr != nil || !bytes.Equal(before, after)) {
		t.Fatal("failed-open attempt changed the selected scheduling record")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptAbsent(t, value.root)
}

func assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptAbsent(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceiptName)); !os.IsNotExist(err) {
		t.Fatal("failed scheduling attempt emitted a receipt")
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorNarrowEvidence(t *testing.T, receipt NodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorReceipt) {
	t.Helper()
	if receipt.Evidence != (NodeConnectorPlacementExecutionGraphNextTaskSchedulingEvidenceAuthority{LocalSchedulingTransitionPerformed: true}) {
		t.Fatal("scheduling receipt widened or omitted its sole evidence claim")
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"task_launch":true`, `"node_execution":true`, `"retry":true`, `"repair":true`, `"cancellation":true`, `"publication":true`, `"callback":true`, `"validation":true`, `"network":true`, `"broker":true`, `"provider":true`, `"forgepipe":true`, `"remote_execution":true`, `"checkout":true`, `"git":true`, `"commit":true`, `"push":true`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("scheduling evidence escalated adjacent authority: %s", forbidden)
		}
	}
}

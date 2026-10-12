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

type nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture struct {
	root        string
	policy      *nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture
	expected    NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected
	decision    NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision
	request     *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest
	preimages   []NodeConnectorPlacementExecutionGraphDependencyRecord
	recordPaths []string
	receiptPath string
}

func TestNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRoutesAreDistinct(t *testing.T) {
	receipts := make(map[string]NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt)
	for _, test := range []struct {
		terminal string
		route    string
		state    string
	}{
		{terminal: "succeeded", route: "dependency_release_transition", state: "dependency_released"},
		{terminal: "failed", route: "failure_propagation_transition", state: "failure_propagated"},
	} {
		t.Run(test.terminal, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t, test.terminal, "approved", true)
			receipt := mustExecuteNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(t, value)
			if receipt.Route != test.route || receipt.TransitionCount != uint64(len(value.recordPaths)) || receipt.RecordWriteCount != uint64(len(value.recordPaths)) || !receipt.AuthorizationConsumed || !receipt.FixtureOwned {
				t.Fatal("executor receipt omitted the exact route, counts, consumed authorization, or fixture ownership")
			}
			if !reflect.DeepEqual(receipt.PolicyBinding, value.request.Binding) || receipt.PolicyDecisionFingerprint != value.decision.DecisionFingerprint || receipt.PolicyRequestFingerprint != value.request.RequestFingerprint || !reflect.DeepEqual(receipt.DependencyTargets, value.request.DependencyTargets) {
				t.Fatal("executor receipt omitted an immutable predecessor, policy, request, or target binding")
			}
			for index, path := range value.recordPaths {
				record := mustLoadNodeConnectorPlacementExecutionGraphDependencyRecord(t, value.root, path)
				if record.State != test.state || record.Version != value.preimages[index].Version+1 || record.PreviousRecordFingerprint != value.preimages[index].RecordFingerprint || record.Route != test.route || record.TransitionRequestFingerprint != value.request.RequestFingerprint {
					t.Fatal("executor did not persist the exact route-specific successor")
				}
				if !reflect.DeepEqual(receipt.Transitions[index].Preimage, value.preimages[index]) || !reflect.DeepEqual(receipt.Transitions[index].Postimage, record) || receipt.Transitions[index].Target.DependencyID != value.request.DependencyTargets[index].DependencyID {
					t.Fatal("receipt omitted an exact target preimage or postimage")
				}
			}
			assertNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorNarrowEvidence(t, receipt)
			receipts[test.terminal] = receipt
		})
	}
	if receipts["succeeded"].Evidence.FailurePropagationPerformed || !receipts["succeeded"].Evidence.DependencyReleasePerformed || receipts["failed"].Evidence.DependencyReleasePerformed || !receipts["failed"].Evidence.FailurePropagationPerformed || receipts["succeeded"].ReceiptFingerprint == receipts["failed"].ReceiptFingerprint {
		t.Fatal("release and failure-propagation routes were collapsed or combined")
	}
}

func TestNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRequiresExactAuthorization(t *testing.T) {
	for _, test := range []struct {
		name          string
		decision      string
		publishPolicy bool
		mutate        func(*nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture)
	}{
		{name: "rejected", decision: "rejected", publishPolicy: true},
		{name: "absent", decision: "approved", publishPolicy: false},
		{name: "mismatched request", decision: "approved", publishPolicy: true, mutate: func(v *nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture) {
			v.expected.PolicyRequestFingerprint = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{name: "mismatched decision", decision: "approved", publishPolicy: true, mutate: func(v *nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture) {
			v.expected.PolicyDecisionFingerprint = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t, "succeeded", test.decision, test.publishPolicy)
			if test.mutate != nil {
				test.mutate(value)
			}
			before := snapshotNodeConnectorPlacementExecutionGraphDependencyRecords(t, value)
			if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(value.root, value.expected); err == nil {
				t.Fatal("absent, rejected, or mismatched authorization was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphDependencyRecordsEqual(t, value, before)
			assertNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptAbsent(t, value)
		})
	}

	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest)
	}{
		{name: "consumed", mutate: func(v *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest) {
			v.AuthorizationConsumed = true
		}},
		{name: "transition already invoked", mutate: func(v *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest) {
			v.TransitionInvoked = true
		}},
		{name: "callback already invoked", mutate: func(v *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest) {
			v.CallbacksInvoked = true
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t, "succeeded", "approved", true)
			request := *value.request
			test.mutate(&request)
			request.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestFingerprint(request)
			mustWriteNodeConnectorPlacementExecutionGraphLifecycleExecutorArtifact(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestName), request)
			if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(value.root, value.expected); err == nil {
				t.Fatal("consumed or inferred route authorization was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptAbsent(t, value)
		})
	}

	t.Run("inferred decision", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t, "succeeded", "approved", true)
		decision := value.decision
		decision.ApprovalInferred = true
		decision.DecisionFingerprint, _ = nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFingerprint(decision)
		mustWriteNodeConnectorPlacementExecutionGraphLifecycleExecutorArtifact(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionName), decision)
		if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(value.root, value.expected); err == nil {
			t.Fatal("inferred authorization was accepted")
		}
		assertNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptAbsent(t, value)
	})

	t.Run("terminal route mismatch", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t, "succeeded", "approved", true)
		request := *value.request
		request.Route = "failure_propagation_transition"
		request.Authority = NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority{FailurePropagationTransitionAttempt: true}
		request.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestFingerprint(request)
		mustWriteNodeConnectorPlacementExecutionGraphLifecycleExecutorArtifact(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestName), request)
		if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(value.root, value.expected); err == nil {
			t.Fatal("terminal graph state and route mismatch was accepted")
		}
		assertNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptAbsent(t, value)
	})
}

func TestNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRejectsTargetSetConflicts(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture)
	}{
		{name: "missing", mutate: func(v *nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture) {
			mustRemoveNodeConnectorPlacementExecutionGraphLifecycleExecutorPath(t, v.recordPaths[1])
		}},
		{name: "stale version", mutate: func(v *nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture) {
			record := v.preimages[1]
			record.Version++
			record.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphDependencyRecordFingerprint(record)
			mustWriteNodeConnectorPlacementExecutionGraphLifecycleExecutorArtifact(t, v.recordPaths[1], record)
		}},
		{name: "changed state", mutate: func(v *nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture) {
			record := v.preimages[1]
			record.State = "dependency_released"
			record.PreviousRecordFingerprint = v.preimages[1].RecordFingerprint
			record.TransitionRequestID = "unrelated-request-001"
			record.TransitionRequestFingerprint = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
			record.Route = "dependency_release_transition"
			record.Version++
			record.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphDependencyRecordFingerprint(record)
			mustWriteNodeConnectorPlacementExecutionGraphLifecycleExecutorArtifact(t, v.recordPaths[1], record)
		}},
		{name: "substituted identity", mutate: func(v *nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture) {
			record := v.preimages[1]
			record.DependencyID = "dependency-substituted-001"
			record.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphDependencyRecordFingerprint(record)
			mustWriteNodeConnectorPlacementExecutionGraphLifecycleExecutorArtifact(t, v.recordPaths[1], record)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t, "succeeded", "approved", true)
			firstBefore := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPaths[0])
			test.mutate(value)
			if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(value.root, value.expected); err == nil {
				t.Fatal("missing, stale, changed, or substituted target was accepted")
			}
			if !bytes.Equal(firstBefore, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPaths[0])) {
				t.Fatal("executor mutated an earlier target before complete-set validation")
			}
			assertNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptAbsent(t, value)
		})
	}

	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected)
	}{
		{name: "duplicated", mutate: func(v *NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected) {
			v.Policy.DependencyTargets[1] = v.Policy.DependencyTargets[0]
		}},
		{name: "unsorted", mutate: func(v *NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected) {
			v.Policy.DependencyTargets[0], v.Policy.DependencyTargets[1] = v.Policy.DependencyTargets[1], v.Policy.DependencyTargets[0]
		}},
		{name: "changed target", mutate: func(v *NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected) {
			v.Policy.DependencyTargets[1].DependencyRecordID = "dependency-record-substituted-001"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t, "succeeded", "approved", true)
			test.mutate(&value.expected)
			if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(value.root, value.expected); err == nil {
				t.Fatal("duplicated, reordered, or substituted target set was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptAbsent(t, value)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReplayRestartAndConcurrency(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t, "succeeded", "approved", true)
	executor := mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(t, value)
	var writes atomic.Int32
	original := nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteRecordAtomic
	nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteRecordAtomic = func(path string, payload any) error { writes.Add(1); return original(path, payload) }
	t.Cleanup(func() { nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteRecordAtomic = original })
	const callers = 12
	var wait sync.WaitGroup
	receipts := make(chan NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt, callers)
	errs := make(chan error, callers)
	for index := 0; index < callers; index++ {
		wait.Add(1)
		go func() { defer wait.Done(); receipt, err := executor.Execute(); receipts <- receipt; errs <- err }()
	}
	wait.Wait()
	close(receipts)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first *NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt
	for receipt := range receipts {
		if first == nil {
			cloned := receipt
			first = &cloned
		} else if !reflect.DeepEqual(*first, receipt) {
			t.Fatal("same-request concurrency returned conflicting receipts")
		}
	}
	if writes.Load() != int32(len(value.recordPaths)) {
		t.Fatalf("same-request concurrency performed %d writes, want %d", writes.Load(), len(value.recordPaths))
	}
	restarted := mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(t, value)
	replayed, err := restarted.Execute()
	if err != nil || first == nil || !reflect.DeepEqual(*first, replayed) || writes.Load() != int32(len(value.recordPaths)) {
		t.Fatal("exact replay or restart repeated transitions or changed durable evidence")
	}

	conflict := value.expected
	conflict.PolicyRequestFingerprint = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(value.root, conflict); err == nil {
		t.Fatal("conflicting request concurrency was accepted")
	}
}

func TestNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorFailureRecovery(t *testing.T) {
	t.Run("failure before replacement", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t, "succeeded", "approved", true)
		before := snapshotNodeConnectorPlacementExecutionGraphDependencyRecords(t, value)
		original := nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteRecordAtomic
		nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteRecordAtomic = func(string, any) error { return errors.New("injected replacement failure") }
		t.Cleanup(func() { nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteRecordAtomic = original })
		if _, err := mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(t, value).Execute(); err == nil {
			t.Fatal("target replacement failure was accepted")
		}
		assertNodeConnectorPlacementExecutionGraphDependencyRecordsEqual(t, value, before)
		assertNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptAbsent(t, value)
	})

	t.Run("partial replacement recovers exact same request", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t, "failed", "approved", true)
		original := nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteRecordAtomic
		var calls atomic.Int32
		nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteRecordAtomic = func(path string, payload any) error {
			if calls.Add(1) == 2 {
				return errors.New("injected second replacement failure")
			}
			return original(path, payload)
		}
		t.Cleanup(func() { nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteRecordAtomic = original })
		if _, err := mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(t, value).Execute(); err == nil {
			t.Fatal("partial replacement failure was accepted")
		}
		assertNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptAbsent(t, value)
		firstPost := mustLoadNodeConnectorPlacementExecutionGraphDependencyRecord(t, value.root, value.recordPaths[0])
		if firstPost.State != "failure_propagated" {
			t.Fatal("first exact same-request postimage was not durable")
		}
		nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteRecordAtomic = original
		receipt := mustExecuteNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(t, value)
		if receipt.Transitions[0].Postimage.RecordFingerprint != firstPost.RecordFingerprint {
			t.Fatal("partial recovery repeated or replaced the completed target")
		}
	})

	t.Run("receipt publication recovers without repeated transitions", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t, "succeeded", "approved", true)
		recordWriter := nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteRecordAtomic
		receiptWriter := nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteReceiptAtomic
		var writes atomic.Int32
		nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteRecordAtomic = func(path string, payload any) error { writes.Add(1); return recordWriter(path, payload) }
		nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteReceiptAtomic = func(string, any) error { return errors.New("injected receipt failure") }
		t.Cleanup(func() {
			nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteRecordAtomic = recordWriter
			nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteReceiptAtomic = receiptWriter
		})
		if _, err := mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(t, value).Execute(); err == nil {
			t.Fatal("receipt publication failure was accepted")
		}
		if writes.Load() != int32(len(value.recordPaths)) {
			t.Fatal("receipt failure did not leave only exact postimages")
		}
		nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorWriteReceiptAtomic = receiptWriter
		receipt := mustExecuteNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(t, value)
		if writes.Load() != int32(len(value.recordPaths)) || receipt.TransitionCount != uint64(len(value.recordPaths)) {
			t.Fatal("receipt recovery repeated target transitions")
		}
	})
}

func TestNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRejectsMalformedAndUnsafeArtifacts(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{name: "malformed", mutate: func([]byte) []byte { return []byte("{") }},
		{name: "noncanonical", mutate: func(raw []byte) []byte {
			var value any
			_ = json.Unmarshal(raw, &value)
			compact, _ := json.Marshal(value)
			return compact
		}},
		{name: "unknown field", mutate: func(raw []byte) []byte {
			return bytes.Replace(raw, []byte("\n}"), []byte(",\n  \"unknown\": true\n}"), 1)
		}},
		{name: "trailing", mutate: func(raw []byte) []byte { return append(append([]byte(nil), raw...), []byte("{}")...) }},
		{name: "oversized", mutate: func([]byte) []byte {
			return bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorArtifactMaxBytes+1)
		}},
	} {
		t.Run("target "+test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t, "succeeded", "approved", true)
			raw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPaths[1])
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPaths[1], test.mutate(raw))
			if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(value.root, value.expected); err == nil {
				t.Fatal("malformed or noncanonical target was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptAbsent(t, value)
		})
		t.Run("receipt "+test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t, "succeeded", "approved", true)
			mustExecuteNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(t, value)
			raw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.receiptPath)
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.receiptPath, test.mutate(raw))
			if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(value.root, value.expected); err == nil {
				t.Fatal("malformed or noncanonical receipt was accepted")
			}
		})
	}

	t.Run("symlink target", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t, "succeeded", "approved", true)
		realPath := value.recordPaths[1] + ".real"
		if err := os.Rename(value.recordPaths[1], realPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(realPath, value.recordPaths[1]); err != nil {
			t.Skipf("symlink creation unavailable: %v", err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(value.root, value.expected); err == nil {
			t.Fatal("symlink target was accepted")
		}
		assertNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptAbsent(t, value)
	})
}

func TestNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRevalidatesImmutableBindings(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected)
	}{
		{name: "lifecycle receipt", mutate: func(v *NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected) {
			v.Policy.AuditReceiptFingerprint = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
		}},
		{name: "lifecycle policy", mutate: func(v *NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected) {
			v.Policy.Executor.PolicyDecisionFingerprint = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
		}},
		{name: "projection", mutate: func(v *NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected) {
			v.Policy.Executor.Policy.ProjectionDecisionFingerprint = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
		}},
		{name: "finalization", mutate: func(v *NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected) {
			v.Policy.Executor.Policy.Projection.FinalizationDecisionFingerprint = "sha256:4444444444444444444444444444444444444444444444444444444444444444"
		}},
		{name: "outcome", mutate: func(v *NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected) {
			v.Policy.Executor.Policy.Projection.Finalization.Outcomes[0].ArtifactFingerprint = "sha256:5555555555555555555555555555555555555555555555555555555555555555"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t, "succeeded", "approved", true)
			test.mutate(&value.expected)
			if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(value.root, value.expected); err == nil {
				t.Fatal("changed immutable predecessor binding was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptAbsent(t, value)
		})
	}
}

func newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t *testing.T, terminal, decision string, publishPolicy bool) *nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture {
	t.Helper()
	policy := newNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture(t, terminal, decision)
	preimages := make([]NodeConnectorPlacementExecutionGraphDependencyRecord, len(policy.expected.DependencyTargets))
	recordPaths := make([]string, len(preimages))
	for index, target := range policy.expected.DependencyTargets {
		preimage := NodeConnectorPlacementExecutionGraphDependencyRecord{Schema: NodeConnectorPlacementExecutionGraphDependencyRecordSchema, GraphRunID: policy.receipt.GraphRunID, DependencyID: target.DependencyID, DependencyRecordID: target.DependencyRecordID, State: "blocked", Version: target.ExpectedPreimageVersion}
		preimage.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphDependencyRecordFingerprint(preimage)
		policy.expected.DependencyTargets[index].ExpectedPreimageFingerprint = preimage.RecordFingerprint
		policy.fixture.DependencyTargets[index].ExpectedPreimageFingerprint = preimage.RecordFingerprint
		path, err := nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorRecordPath(policy.root, policy.receipt.GraphStoreID, target.DependencyRecordID)
		if err != nil {
			t.Fatal(err)
		}
		mustWriteNodeConnectorPlacementExecutionGraphLifecycleExecutorArtifact(t, path, preimage)
		preimages[index] = preimage
		recordPaths[index] = path
	}
	policy.fixture.DependencyTargetsFingerprint, _ = nodeExecutionFingerprintValue(policy.fixture.DependencyTargets)
	value := &nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture{root: policy.root, policy: policy, preimages: preimages, recordPaths: recordPaths, receiptPath: filepath.Join(policy.root, nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptName)}
	if publishPolicy {
		decisionValue, request := mustDecideNodeConnectorPlacementExecutionGraphDependencyTransitionPolicy(t, policy)
		value.decision = decisionValue
		value.request = request
		value.expected = NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected{Policy: policy.expected, PolicyDecisionFingerprint: decisionValue.DecisionFingerprint}
		if request != nil {
			value.expected.PolicyRequestFingerprint = request.RequestFingerprint
		}
	} else {
		value.expected = NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorExpected{Policy: policy.expected}
	}
	return value
}

func mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture) *NodeConnectorPlacementExecutionGraphDependencyTransitionExecutor {
	t.Helper()
	executor, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return executor
}

func mustExecuteNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture) NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt {
	t.Helper()
	receipt, err := mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(t, value).Execute()
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func mustLoadNodeConnectorPlacementExecutionGraphDependencyRecord(t *testing.T, root, path string) NodeConnectorPlacementExecutionGraphDependencyRecord {
	t.Helper()
	record, err := loadNodeConnectorPlacementExecutionGraphDependencyRecord(root, path)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func snapshotNodeConnectorPlacementExecutionGraphDependencyRecords(t *testing.T, value *nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture) [][]byte {
	t.Helper()
	result := make([][]byte, len(value.recordPaths))
	for index, path := range value.recordPaths {
		result[index] = mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path)
	}
	return result
}

func assertNodeConnectorPlacementExecutionGraphDependencyRecordsEqual(t *testing.T, value *nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture, expected [][]byte) {
	t.Helper()
	for index, path := range value.recordPaths {
		if !bytes.Equal(expected[index], mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path)) {
			t.Fatalf("dependency record %d changed before complete authorization and validation", index)
		}
	}
}

func assertNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptAbsent(t *testing.T, value *nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture) {
	t.Helper()
	if _, err := os.Lstat(value.receiptPath); !os.IsNotExist(err) {
		t.Fatal("dependency-transition executor emitted a receipt before completing every target")
	}
}

func assertNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorNarrowEvidence(t *testing.T, receipt NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt) {
	t.Helper()
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"next_task_scheduling":true`, `"new_execution":true`, `"retry":true`, `"repair":true`, `"cancellation":true`, `"callback":true`, `"validation":true`, `"publication":true`, `"network":true`, `"broker":true`, `"provider":true`, `"forgepipe":true`, `"checkout":true`, `"git":true`, `"commit":true`, `"push":true`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("durable evidence escalated adjacent authority: %s", forbidden)
		}
	}
}

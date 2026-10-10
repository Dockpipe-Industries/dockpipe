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

type nodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture struct {
	root           string
	policy         *nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture
	expected       NodeConnectorPlacementExecutionGraphLifecycleExecutorExpected
	policyDecision NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision
	policyRequest  *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest
	preimage       NodeConnectorPlacementExecutionGraphLifecycleRecord
	recordPath     string
	receiptPath    string
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorProjectsExactSuccessAndFailure(t *testing.T) {
	var receipts = make(map[string]NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt)
	for _, terminal := range []string{"succeeded", "failed"} {
		t.Run(terminal, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture(t, terminal, "approved", true)
			before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
			receipt := mustExecuteNodeConnectorPlacementExecutionGraphLifecycleExecutor(t, value)
			record := mustLoadNodeConnectorPlacementExecutionGraphLifecycleRecord(t, value.recordPath)
			if record.LifecycleState != terminal || record.Version != value.preimage.Version+1 || record.PreviousRecordFingerprint != value.preimage.RecordFingerprint {
				t.Fatal("executor did not persist the exact projected terminal successor")
			}
			if receipt.ProjectedTerminalPostState != terminal || !reflect.DeepEqual(receipt.Preimage, value.preimage) || !reflect.DeepEqual(receipt.Postimage, record) || receipt.PolicyDecisionFingerprint != value.policyDecision.DecisionFingerprint || receipt.PolicyRequestFingerprint != value.policyRequest.RequestFingerprint {
				t.Fatal("audit receipt omitted the exact CAS or policy bindings")
			}
			if !reflect.DeepEqual(receipt.TaskBindings, value.policyRequest.TaskBindings) || receipt.ProjectionDecisionFingerprint != value.policyRequest.ProjectionDecisionFingerprint || receipt.ProjectionRequestFingerprint != value.policyRequest.ProjectionRequestFingerprint || receipt.FinalizationDecisionFingerprint != value.policyRequest.FinalizationDecisionFingerprint || receipt.FinalizationRequestFingerprint != value.policyRequest.FinalizationRequestFingerprint {
				t.Fatal("audit receipt omitted an immutable graph-run, run, task, operation, receipt, outcome, projection, or finalization binding")
			}
			assertNodeConnectorPlacementExecutionGraphLifecycleExecutorNarrowAuthority(t, receipt)
			assertNodeConnectorPlacementExecutionGraphLifecycleExecutorOnlyRecordAndReceiptChanged(t, value, before)
			receipts[terminal] = receipt
		})
	}
	if receipts["succeeded"].Postimage.LifecycleState == receipts["failed"].Postimage.LifecycleState || receipts["succeeded"].ReceiptFingerprint == receipts["failed"].ReceiptFingerprint {
		t.Fatal("successful and failed terminal projections were collapsed")
	}
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorRequiresApprovedPolicy(t *testing.T) {
	for _, test := range []struct {
		name          string
		policy        string
		publishPolicy bool
	}{
		{name: "rejected policy", policy: "rejected", publishPolicy: true},
		{name: "projection authority only", policy: "approved", publishPolicy: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture(t, "succeeded", test.policy, test.publishPolicy)
			before := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)
			if _, err := OpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(value.root, value.expected); err == nil {
				t.Fatal("absent or rejected executor policy authorized a graph transition")
			}
			after := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)
			if !bytes.Equal(before, after) {
				t.Fatal("absent or rejected policy changed the graph record")
			}
			if _, err := os.Lstat(value.receiptPath); !os.IsNotExist(err) {
				t.Fatal("absent or rejected policy emitted an audit receipt")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorReplayRestartAndConcurrencyAreIdempotent(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture(t, "succeeded", "approved", true)
	executor := mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(t, value)
	var writes atomic.Int32
	original := nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteRecordAtomic
	nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteRecordAtomic = func(path string, payload any) error {
		writes.Add(1)
		return original(path, payload)
	}
	t.Cleanup(func() { nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteRecordAtomic = original })

	const callers = 12
	receipts := make(chan NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt, callers)
	errs := make(chan error, callers)
	var wait sync.WaitGroup
	for i := 0; i < callers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			receipt, err := executor.Execute()
			receipts <- receipt
			errs <- err
		}()
	}
	wait.Wait()
	close(receipts)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first *NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt
	for receipt := range receipts {
		if first == nil {
			cloned := receipt
			first = &cloned
		} else if !reflect.DeepEqual(*first, receipt) {
			t.Fatal("same-request concurrency returned different durable receipts")
		}
	}
	if writes.Load() != 1 {
		t.Fatalf("same-request concurrency performed %d record transitions, want 1", writes.Load())
	}
	restarted := mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(t, value)
	replayed, err := restarted.Execute()
	if err != nil || first == nil || !reflect.DeepEqual(*first, replayed) || writes.Load() != 1 {
		t.Fatal("exact replay or restart changed the receipt or repeated the transition")
	}
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorRejectsStaleOrChangedRecords(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphLifecycleRecord)
	}{
		{name: "store identity", mutate: func(record *NodeConnectorPlacementExecutionGraphLifecycleRecord) {
			record.GraphStoreID = "local-graph-store-conflict-001"
		}},
		{name: "record identity", mutate: func(record *NodeConnectorPlacementExecutionGraphLifecycleRecord) {
			record.GraphRecordID = "graph-record-conflict-001"
		}},
		{name: "graph run identity", mutate: func(record *NodeConnectorPlacementExecutionGraphLifecycleRecord) {
			record.GraphRunID = "graph-run-conflict-001"
		}},
		{name: "stale version", mutate: func(record *NodeConnectorPlacementExecutionGraphLifecycleRecord) { record.Version++ }},
		{name: "unrelated postimage", mutate: func(record *NodeConnectorPlacementExecutionGraphLifecycleRecord) {
			record.LifecycleState = "failed"
			record.Version++
			record.PreviousRecordFingerprint = record.RecordFingerprint
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture(t, "succeeded", "approved", true)
			record := value.preimage
			test.mutate(&record)
			record.RecordFingerprint = ""
			record.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphLifecycleRecordFingerprint(record)
			mustWriteNodeConnectorPlacementExecutionGraphLifecycleExecutorArtifact(t, value.recordPath, record)
			before := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)
			if _, err := OpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(value.root, value.expected); err == nil {
				t.Fatal("stale fingerprint/version, changed identity, or unrelated postimage was accepted")
			}
			if !bytes.Equal(before, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)) {
				t.Fatal("failed compare-and-swap changed the stale record")
			}
			if _, err := os.Lstat(value.receiptPath); !os.IsNotExist(err) {
				t.Fatal("failed compare-and-swap emitted a receipt")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorRejectsMissingTamperedAndConflictingEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *nodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture)
	}{
		{name: "missing policy request", mutate: func(t *testing.T, value *nodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture) {
			mustRemoveNodeConnectorPlacementExecutionGraphLifecycleExecutorPath(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequestName))
		}},
		{name: "orphaned policy request", mutate: func(t *testing.T, value *nodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture) {
			mustRemoveNodeConnectorPlacementExecutionGraphLifecycleExecutorPath(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionName))
		}},
		{name: "tampered projection request", mutate: func(t *testing.T, value *nodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture) {
			path := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphFinalStateProjectionRequestName)
			raw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path)
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path, bytes.Replace(raw, []byte(`"final_state": "succeeded"`), []byte(`"final_state": "failed"`), 1))
		}},
		{name: "conflicting expected policy", mutate: func(_ *testing.T, value *nodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture) {
			value.expected.PolicyRequestFingerprint = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture(t, "succeeded", "approved", true)
			before := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)
			test.mutate(t, value)
			if _, err := OpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(value.root, value.expected); err == nil {
				t.Fatal("missing, orphaned, tampered, or conflicting predecessor evidence was accepted")
			}
			if !bytes.Equal(before, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)) {
				t.Fatal("invalid predecessor evidence changed the graph record")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorRejectsMalformedNoncanonicalUnknownTrailingAndOversizedRecords(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture(t, "succeeded", "approved", true)
	canonical := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)
	var generic any
	if err := json.Unmarshal(canonical, &generic); err != nil {
		t.Fatal(err)
	}
	compact, _ := json.Marshal(generic)
	unknown := bytes.Replace(canonical, []byte("\n}"), []byte(",\n  \"unknown\": true\n}"), 1)
	tests := [][]byte{nil, compact, append(append([]byte(nil), canonical...), []byte("{}")...), unknown, bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphLifecycleExecutorArtifactMaxBytes+1)}
	for index, raw := range tests {
		t.Run(string(rune('a'+index)), func(t *testing.T) {
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath, raw)
			if _, err := OpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(value.root, value.expected); err == nil {
				t.Fatal("malformed, noncanonical, unknown-field, trailing, or oversized record was accepted")
			}
			if _, err := os.Lstat(value.receiptPath); !os.IsNotExist(err) {
				t.Fatal("invalid record emitted a receipt")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorDoesNotCreateMissingRecord(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture(t, "succeeded", "approved", true)
	mustRemoveNodeConnectorPlacementExecutionGraphLifecycleExecutorPath(t, value.recordPath)
	if _, err := OpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(value.root, value.expected); err == nil {
		t.Fatal("missing bound graph record was created or accepted")
	}
	if _, err := os.Lstat(value.recordPath); !os.IsNotExist(err) {
		t.Fatal("executor created the missing bound graph record")
	}
	if _, err := os.Lstat(value.receiptPath); !os.IsNotExist(err) {
		t.Fatal("missing bound graph record emitted a receipt")
	}
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorAtomicFailureAndRestartRecovery(t *testing.T) {
	t.Run("failure before replacement", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture(t, "succeeded", "approved", true)
		before := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)
		original := nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteRecordAtomic
		nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteRecordAtomic = func(string, any) error { return errors.New("injected record replacement failure") }
		t.Cleanup(func() { nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteRecordAtomic = original })
		if _, err := mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(t, value).Execute(); err == nil {
			t.Fatal("record replacement failure was accepted")
		}
		if !bytes.Equal(before, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)) {
			t.Fatal("failure before replacement did not preserve the preimage")
		}
		if _, err := os.Lstat(value.receiptPath); !os.IsNotExist(err) {
			t.Fatal("failure before replacement emitted a receipt")
		}
	})

	t.Run("failure after replacement recovers", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture(t, "failed", "approved", true)
		var recordWrites atomic.Int32
		recordWriter := nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteRecordAtomic
		receiptWriter := nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteReceiptAtomic
		nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteRecordAtomic = func(path string, payload any) error { recordWrites.Add(1); return recordWriter(path, payload) }
		nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteReceiptAtomic = func(string, any) error { return errors.New("injected receipt publication failure") }
		t.Cleanup(func() {
			nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteRecordAtomic = recordWriter
			nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteReceiptAtomic = receiptWriter
		})
		if _, err := mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(t, value).Execute(); err == nil {
			t.Fatal("receipt publication failure was accepted")
		}
		postimage := mustLoadNodeConnectorPlacementExecutionGraphLifecycleRecord(t, value.recordPath)
		if postimage.LifecycleState != "failed" || recordWrites.Load() != 1 {
			t.Fatal("post-replacement failure lost or repeated the exact transition")
		}
		nodeConnectorPlacementExecutionGraphLifecycleExecutorWriteReceiptAtomic = receiptWriter
		receipt := mustExecuteNodeConnectorPlacementExecutionGraphLifecycleExecutor(t, value)
		if receipt.Postimage.RecordFingerprint != postimage.RecordFingerprint || recordWrites.Load() != 1 {
			t.Fatal("restart did not finish the same receipt without another transition")
		}
	})
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorRejectsStaleOrConflictingReceiptState(t *testing.T) {
	t.Run("missing record", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture(t, "succeeded", "approved", true)
		mustExecuteNodeConnectorPlacementExecutionGraphLifecycleExecutor(t, value)
		mustRemoveNodeConnectorPlacementExecutionGraphLifecycleExecutorPath(t, value.recordPath)
		if _, err := OpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(value.root, value.expected); err == nil {
			t.Fatal("receipt with a missing record was accepted")
		}
	})

	t.Run("stale record", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture(t, "succeeded", "approved", true)
		mustExecuteNodeConnectorPlacementExecutionGraphLifecycleExecutor(t, value)
		mustWriteNodeConnectorPlacementExecutionGraphLifecycleExecutorArtifact(t, value.recordPath, value.preimage)
		if _, err := OpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(value.root, value.expected); err == nil {
			t.Fatal("receipt with a stale preimage was accepted")
		}
	})

	t.Run("conflicting receipt", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture(t, "succeeded", "approved", true)
		mustExecuteNodeConnectorPlacementExecutionGraphLifecycleExecutor(t, value)
		raw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.receiptPath)
		mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.receiptPath, bytes.Replace(raw, []byte(`"projected_terminal_post_state": "succeeded"`), []byte(`"projected_terminal_post_state": "failed"`), 1))
		if _, err := OpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(value.root, value.expected); err == nil {
			t.Fatal("conflicting durable audit receipt was accepted")
		}
	})
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorRejectsMalformedNoncanonicalUnknownTrailingAndOversizedReceipts(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{name: "empty", mutate: func([]byte) []byte { return nil }},
		{name: "noncanonical", mutate: func(raw []byte) []byte {
			var generic any
			_ = json.Unmarshal(raw, &generic)
			compact, _ := json.Marshal(generic)
			return compact
		}},
		{name: "unknown field", mutate: func(raw []byte) []byte {
			return bytes.Replace(raw, []byte("\n}"), []byte(",\n  \"unknown\": true\n}"), 1)
		}},
		{name: "trailing data", mutate: func(raw []byte) []byte { return append(append([]byte(nil), raw...), []byte("{}")...) }},
		{name: "oversized", mutate: func([]byte) []byte {
			return bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphLifecycleExecutorArtifactMaxBytes+1)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture(t, "succeeded", "approved", true)
			receipt := mustExecuteNodeConnectorPlacementExecutionGraphLifecycleExecutor(t, value)
			postimage := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)
			raw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.receiptPath)
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.receiptPath, test.mutate(raw))
			if _, err := OpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(value.root, value.expected); err == nil {
				t.Fatal("malformed, noncanonical, unknown-field, trailing, or oversized receipt was accepted")
			}
			if !bytes.Equal(postimage, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)) || receipt.Postimage.RecordFingerprint == "" {
				t.Fatal("invalid receipt changed or obscured the exact postimage")
			}
		})
	}
}

func newNodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture(t *testing.T, terminal, decision string, publishPolicy bool) *nodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture {
	t.Helper()
	policy := newNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture(t, terminal, decision)
	precondition := policy.expected.StorePrecondition
	preimage := NodeConnectorPlacementExecutionGraphLifecycleRecord{
		Schema: NodeConnectorPlacementExecutionGraphLifecycleRecordSchema, GraphStoreID: precondition.GraphStoreID, GraphRecordID: precondition.GraphRecordID,
		GraphRunID: policy.projectionRequest.GraphRunID, LifecycleState: "running", Version: precondition.ExpectedPreimageVersion,
	}
	preimage.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphLifecycleRecordFingerprint(preimage)
	policy.expected.StorePrecondition.ExpectedPreimageFingerprint = preimage.RecordFingerprint
	policy.fixture.StorePrecondition.ExpectedPreimageFingerprint = preimage.RecordFingerprint
	recordPath := nodeConnectorPlacementExecutionGraphLifecycleExecutorRecordPath(policy.root, policy.expected.StorePrecondition)
	mustWriteNodeConnectorPlacementExecutionGraphLifecycleExecutorArtifact(t, recordPath, preimage)
	value := &nodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture{
		root: policy.root, policy: policy, preimage: preimage, recordPath: recordPath,
		receiptPath: filepath.Join(policy.root, nodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceiptName),
	}
	if publishPolicy {
		policyDecision, policyRequest := mustDecideNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t, mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(t, policy), policy.fixture)
		value.policyDecision = policyDecision
		value.policyRequest = policyRequest
		value.expected = NodeConnectorPlacementExecutionGraphLifecycleExecutorExpected{Policy: policy.expected, PolicyDecisionFingerprint: policyDecision.DecisionFingerprint}
		if policyRequest != nil {
			value.expected.PolicyRequestFingerprint = policyRequest.RequestFingerprint
		}
	} else {
		value.expected = NodeConnectorPlacementExecutionGraphLifecycleExecutorExpected{Policy: policy.expected}
	}
	return value
}

func mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture) *NodeConnectorPlacementExecutionGraphLifecycleExecutor {
	t.Helper()
	executor, err := OpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return executor
}

func mustExecuteNodeConnectorPlacementExecutionGraphLifecycleExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture) NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt {
	t.Helper()
	receipt, err := mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutor(t, value).Execute()
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func mustWriteNodeConnectorPlacementExecutionGraphLifecycleExecutorArtifact(t *testing.T, path string, value any) {
	t.Helper()
	if err := writeJSONFileAtomic(path, value); err != nil {
		t.Fatal(err)
	}
}

func mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustLoadNodeConnectorPlacementExecutionGraphLifecycleRecord(t *testing.T, path string) NodeConnectorPlacementExecutionGraphLifecycleRecord {
	t.Helper()
	record, err := loadNodeConnectorPlacementExecutionGraphLifecycleRecord(path)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func mustRemoveNodeConnectorPlacementExecutionGraphLifecycleExecutorPath(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = raw
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertNodeConnectorPlacementExecutionGraphLifecycleExecutorOnlyRecordAndReceiptChanged(t *testing.T, value *nodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture, before map[string][]byte) {
	t.Helper()
	after := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
	recordRelative, _ := filepath.Rel(value.root, value.recordPath)
	receiptRelative, _ := filepath.Rel(value.root, value.receiptPath)
	recordRelative = filepath.ToSlash(recordRelative)
	receiptRelative = filepath.ToSlash(receiptRelative)
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
	expected := []string{receiptRelative, recordRelative}
	sort.Strings(expected)
	if !reflect.DeepEqual(changed, expected) {
		t.Fatalf("executor changed forbidden files: got %v want %v", changed, expected)
	}
}

func assertNodeConnectorPlacementExecutionGraphLifecycleExecutorNarrowAuthority(t *testing.T, receipt NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt) {
	t.Helper()
	if receipt.Authority != (NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditAuthority{LocalGraphRecordStateProjection: true}) || !receipt.CompareAndSwapMatched || receipt.RecordWriteCount != 1 || !receipt.AuthorizationConsumed || !receipt.FixtureOwned {
		t.Fatal("executor receipt did not preserve the exact narrow local projection authority")
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"dependency_release":true`, `"next_task":true`, `"retry":true`, `"repair":true`, `"cancellation":true`, `"execution":true`, `"broker":true`, `"forgepipe":true`, `"provider":true`, `"network":true`, `"validation":true`, `"checkout":true`, `"git":true`, `"commit":true`, `"push":true`, `"publication":true`, `"lifecycle":true`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("forbidden callback or lifecycle authority appeared: %s", forbidden)
		}
	}
}

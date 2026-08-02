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

type nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture struct {
	root        string
	executor    *nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture
	receipt     NodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceipt
	expected    NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected
	fixture     NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture
	recordPaths []string
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyApprovedReleaseProducesExactRequest(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture(t, "succeeded", "approved")
	before := snapshotNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRecords(t, value)
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(t, value)
	if decision.Decision != "approved" || request == nil || request.SelectedTaskID != value.fixture.SelectedTaskID || request.RequestID != value.expected.SchedulingRequestID {
		t.Fatal("approved release decision did not produce the exact selected next-task request")
	}
	if !request.OneTimeRequest || request.AuthorizationConsumed || request.SchedulingInvoked || request.TaskLaunched || request.CallbacksInvoked || request.ExternalActionsInvoked || !request.FixtureOwned || !decision.IndependentlyAuthenticated || decision.ApprovalInferred {
		t.Fatal("scheduling request was not an independent deterministic unconsumed policy artifact")
	}
	expectedBinding, err := nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyBinding(value.expected, value.receipt)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(request.Binding, expectedBinding) || !reflect.DeepEqual(request.Candidates, value.expected.Candidates) || request.DecisionFingerprint != decision.DecisionFingerprint {
		t.Fatal("scheduling request omitted the exact graph, terminal task, transition receipt, postimages, candidates, or decision binding")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyNarrowAuthority(t, decision, request)
	assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRecordsEqual(t, value, before)
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRejectedAndFailureRoutesProduceNoRequest(t *testing.T) {
	for _, test := range []struct {
		name     string
		terminal string
	}{
		{name: "rejected release", terminal: "succeeded"},
		{name: "failure propagation", terminal: "failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture(t, test.terminal, "rejected")
			assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyArtifactsAbsent(t, value.root)
			before := snapshotNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRecords(t, value)
			decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(t, value)
			if decision.Decision != "rejected" || request != nil || decision.SelectedTaskID != "" || decision.Authority != (NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyAuthority{}) {
				t.Fatal("rejected or failure-propagation evidence emitted scheduling authority")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyArtifactAbsent(t, value.root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestName)
			assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRecordsEqual(t, value, before)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRejectsCandidateAndInferenceConflicts(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture(t, "succeeded", "approved")
	policies := mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(t, value)

	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture)
	}{
		{name: "empty candidates", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.Candidates = nil
		}},
		{name: "selected task missing", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.SelectedTaskID = ""
		}},
		{name: "selected task outside set", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.SelectedTaskID = "dependency-absent-001"
		}},
		{name: "duplicate candidate", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.Candidates[1] = f.Candidates[0]
		}},
		{name: "unsorted candidates", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.Candidates[0], f.Candidates[1] = f.Candidates[1], f.Candidates[0]
		}},
		{name: "changed record", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.Candidates[0].DependencyRecordID = "dependency-record-conflict-001"
		}},
		{name: "changed postimage", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.Candidates[0].ReleasedPostimageFingerprint = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
		}},
		{name: "changed postimage version", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.Candidates[0].ReleasedPostimageVersion++
		}},
		{name: "changed candidate fingerprint", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.CandidatesFingerprint = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := value.fixture
			fixture.Candidates = cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidates(value.fixture.Candidates)
			test.mutate(&fixture)
			if _, _, err := deriveNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(policies.expected, policies.receipt, fixture); err == nil {
				t.Fatal("missing, outside, reordered, duplicated, stale, or mismatched candidate evidence was accepted")
			}
		})
	}

	for _, source := range []string{"released_records", "ordering", "readiness", "availability", "load", "risk", "cost", "ranking", "matching", "connection", "provider", "broker", "forgepipe"} {
		t.Run("inferred from "+source, func(t *testing.T) {
			fixture := value.fixture
			fixture.Candidates = cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidates(value.fixture.Candidates)
			fixture.ApprovalInferred = true
			fixture.InferenceSource = source
			if _, _, err := deriveNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(policies.expected, policies.receipt, fixture); err == nil {
				t.Fatalf("scheduling approval inferred from %s evidence", source)
			}
		})
	}

	assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyArtifactsAbsent(t, value.root)
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRejectsMismatchedFixtureBindings(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture(t, "succeeded", "approved")
	policies := mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(t, value)

	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture)
	}{
		{name: "receipt id", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.TransitionReceiptID = "transition-receipt-conflict-001"
		}},
		{name: "receipt fingerprint", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.TransitionReceiptFingerprint = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
		}},
		{name: "graph", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.GraphRunID = "graph-run-conflict-001"
		}},
		{name: "terminal task", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.TerminalTaskID = "terminal-task-conflict-001"
		}},
		{name: "route", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.Route = "failure_propagation_transition"
		}},
		{name: "request id", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.SchedulingRequestID = "scheduling-request-conflict-001"
		}},
		{name: "decision authentication", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.AuthenticationDigest = "sha256:4444444444444444444444444444444444444444444444444444444444444444"
		}},
		{name: "escalated authority", mutate: func(f *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) {
			f.Authority.TaskLaunch = true
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := value.fixture
			fixture.Candidates = cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidates(value.fixture.Candidates)
			test.mutate(&fixture)
			if _, _, err := deriveNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(policies.expected, policies.receipt, fixture); err == nil {
				t.Fatal("mismatched graph, terminal task, route, receipt, selected task, decision, or authority was accepted")
			}
		})
	}

	assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyArtifactsAbsent(t, value.root)
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRevalidatesCompletePredecessorChain(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture(t, "succeeded", "approved")
	expectedRaw, err := json.Marshal(value.expected)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected)
	}{
		{name: "transition receipt", mutate: func(e *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected) {
			e.TransitionReceiptFingerprint = "sha256:5555555555555555555555555555555555555555555555555555555555555555"
		}},
		{name: "transition policy request", mutate: func(e *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected) {
			e.Executor.PolicyRequestFingerprint = "sha256:6666666666666666666666666666666666666666666666666666666666666666"
		}},
		{name: "transition policy decision", mutate: func(e *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected) {
			e.Executor.PolicyDecisionFingerprint = "sha256:7777777777777777777777777777777777777777777777777777777777777777"
		}},
		{name: "lifecycle receipt", mutate: func(e *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected) {
			e.Executor.Policy.AuditReceiptFingerprint = "sha256:8888888888888888888888888888888888888888888888888888888888888888"
		}},
		{name: "projection", mutate: func(e *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected) {
			e.Executor.Policy.Executor.Policy.ProjectionDecisionFingerprint = "sha256:9999999999999999999999999999999999999999999999999999999999999999"
		}},
		{name: "finalization", mutate: func(e *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected) {
			e.Executor.Policy.Executor.Policy.Projection.FinalizationDecisionFingerprint = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{name: "outcome", mutate: func(e *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected) {
			e.Executor.Policy.Executor.Policy.Projection.Finalization.Outcomes[0].ArtifactFingerprint = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
		{name: "terminal task expected", mutate: func(e *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected) {
			e.TerminalTaskID = "terminal-task-absent-001"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var expected NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected
			if err := json.Unmarshal(expectedRaw, &expected); err != nil {
				t.Fatal(err)
			}
			test.mutate(&expected)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(value.root, expected); err == nil {
				t.Fatal("stale or changed immutable predecessor binding was accepted")
			}
		})
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyArtifactsAbsent(t, value.root)

	t.Run("missing receipt", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture(t, "succeeded", "approved")
		if err := os.Remove(filepath.Join(value.root, nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptName)); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(value.root, value.expected); err == nil {
			t.Fatal("missing transition receipt was accepted")
		}
	})

	t.Run("stale released postimage", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture(t, "succeeded", "approved")
		mustWriteNodeConnectorPlacementExecutionGraphLifecycleExecutorArtifact(t, value.recordPaths[0], value.executor.preimages[0])
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(value.root, value.expected); err == nil {
			t.Fatal("receipt without every exact persisted postimage was accepted")
		}
	})

	t.Run("tampered transition receipt", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture(t, "succeeded", "approved")
		path := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphDependencyTransitionExecutorReceiptName)
		raw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path)
		mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path, bytes.Replace(raw, []byte(`"route": "dependency_release_transition"`), []byte(`"route": "failure_propagation_transition"`), 1))
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(value.root, value.expected); err == nil {
			t.Fatal("tampered transition receipt was accepted")
		}
	})
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyReplayRestartConcurrencyAndExistingOutputs(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture(t, "succeeded", "approved")
	policies := mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(t, value)
	firstDecision, firstRequest := mustDecideNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyWithPolicies(t, policies, value.fixture)
	secondDecision, secondRequest := mustDecideNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyWithPolicies(t, policies, value.fixture)
	restarted := mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(t, value)
	thirdDecision, thirdRequest := mustDecideNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyWithPolicies(t, restarted, value.fixture)
	if !reflect.DeepEqual(firstDecision, secondDecision) || !reflect.DeepEqual(firstDecision, thirdDecision) || !reflect.DeepEqual(firstRequest, secondRequest) || !reflect.DeepEqual(firstRequest, thirdRequest) {
		t.Fatal("exact replay, restart, or pre-existing identical output changed the scheduling request")
	}

	raw := mustMarshalNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(t, value.fixture)
	const callers = 12
	var wait sync.WaitGroup
	errs := make(chan error, callers)
	for index := 0; index < callers; index++ {
		wait.Add(1)
		go func() { defer wait.Done(); _, _, err := policies.Decide(raw); errs <- err }()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	conflict := value.fixture
	conflict.DecisionID = "scheduling-decision-conflict-001"
	if _, _, err := policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(t, conflict)); err == nil {
		t.Fatal("duplicate or replayed request under a conflicting decision was accepted")
	}

	requestPath := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestName)
	conflictingRequest := *firstRequest
	conflictingRequest.SelectedTaskID = conflictingRequest.Candidates[1].TaskID
	conflictingRequest.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestFingerprint(conflictingRequest)
	mustWriteNodeConnectorPlacementExecutionGraphLifecycleExecutorArtifact(t, requestPath, conflictingRequest)
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(value.root, value.expected); err == nil {
		t.Fatal("pre-existing conflicting scheduling output was accepted")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyConcurrentConflictingAttemptsFailClosed(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture(t, "succeeded", "approved")
	policies := mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(t, value)
	conflict := value.fixture
	conflict.DecisionID = "scheduling-decision-conflict-001"
	inputs := [][]byte{mustMarshalNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(t, value.fixture), mustMarshalNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(t, conflict)}
	var wait sync.WaitGroup
	errs := make(chan error, len(inputs))
	for _, input := range inputs {
		wait.Add(1)
		go func(raw []byte) { defer wait.Done(); _, _, err := policies.Decide(raw); errs <- err }(input)
	}
	wait.Wait()
	close(errs)
	successes := 0
	failures := 0
	for err := range errs {
		if err == nil {
			successes++
		} else {
			failures++
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("concurrent conflicts produced successes=%d failures=%d, want one of each", successes, failures)
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRejectsMalformedFixturesAndArtifacts(t *testing.T) {
	fixtureValue := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture(t, "succeeded", "approved")
	fixturePolicies := mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(t, fixtureValue)
	fixtureRaw := mustMarshalNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(t, fixtureValue.fixture)

	for _, test := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{name: "empty", mutate: func([]byte) []byte { return nil }},
		{name: "malformed", mutate: func([]byte) []byte { return []byte("{") }},
		{name: "unknown", mutate: func(raw []byte) []byte { return bytes.Replace(raw, []byte("}"), []byte(",\"unknown\":true}"), 1) }},
		{name: "trailing", mutate: func(raw []byte) []byte { return append(append([]byte(nil), raw...), []byte("{}")...) }},
		{name: "oversized", mutate: func([]byte) []byte {
			return bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionMaxBytes+1)
		}},
		{name: "noncanonical", mutate: func(raw []byte) []byte {
			var decoded any
			_ = json.Unmarshal(raw, &decoded)
			pretty, _ := json.MarshalIndent(decoded, "", "  ")
			return pretty
		}},
	} {
		t.Run("fixture "+test.name, func(t *testing.T) {
			raw := test.mutate(fixtureRaw)
			if _, _, err := fixturePolicies.Decide(raw); err == nil {
				t.Fatal("malformed, unknown-field, trailing, oversized, or noncanonical fixture was accepted")
			}
		})
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyArtifactsAbsent(t, fixtureValue.root)

	requestValue := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture(t, "succeeded", "approved")
	mustDecideNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(t, requestValue)
	requestPath := filepath.Join(requestValue.root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestName)
	requestRaw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, requestPath)

	for _, test := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{name: "malformed", mutate: func([]byte) []byte { return []byte("{") }},
		{name: "unknown", mutate: func(raw []byte) []byte {
			return bytes.Replace(raw, []byte("\n}"), []byte(",\n  \"unknown\": true\n}"), 1)
		}},
		{name: "trailing", mutate: func(raw []byte) []byte { return append(append([]byte(nil), raw...), []byte("{}")...) }},
		{name: "oversized", mutate: func([]byte) []byte {
			return bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyArtifactMaxBytes+1)
		}},
		{name: "noncanonical", mutate: func(raw []byte) []byte {
			var decoded any
			_ = json.Unmarshal(raw, &decoded)
			compact, _ := json.Marshal(decoded)
			return compact
		}},
	} {
		t.Run("request "+test.name, func(t *testing.T) {
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, requestPath, test.mutate(requestRaw))
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(requestValue.root, requestValue.expected); err == nil {
				t.Fatal("malformed, unknown-field, trailing, oversized, or noncanonical request was accepted")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyAtomicRecoveryAndNoActions(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture(t, "succeeded", "approved")
	before := snapshotNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRecords(t, value)
	policies := mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(t, value)
	original := nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyWriteRequestAtomic
	nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyWriteRequestAtomic = func(string, any) error { return errors.New("injected scheduling request failure") }
	t.Cleanup(func() { nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyWriteRequestAtomic = original })
	if _, _, err := policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(t, value.fixture)); err == nil {
		t.Fatal("scheduling request publication failure was accepted")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyArtifactAbsent(t, value.root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestName)
	assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRecordsEqual(t, value, before)
	nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyWriteRequestAtomic = original
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyWithPolicies(t, mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(t, value), value.fixture)
	assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyNarrowAuthority(t, decision, request)
	assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRecordsEqual(t, value, before)
}

func newNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture(t *testing.T, terminal, decision string) *nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture {
	t.Helper()
	executor := newNodeConnectorPlacementExecutionGraphDependencyTransitionExecutorTestFixture(t, terminal, "approved", true)
	receipt := mustExecuteNodeConnectorPlacementExecutionGraphDependencyTransitionExecutor(t, executor)
	terminalTaskID := receipt.PolicyBinding.TaskBindings[0].TaskID
	var candidates []NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate
	if terminal == "succeeded" {
		candidates = make([]NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate, 0, len(receipt.Transitions))
		for _, transition := range receipt.Transitions {
			candidates = append(candidates, NodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidate{TaskID: transition.Target.DependencyID, DependencyRecordID: transition.Target.DependencyRecordID, ReleasedPostimageFingerprint: transition.PostimageFingerprint, ReleasedPostimageVersion: transition.PostimageVersion})
		}
	}
	candidateFingerprint, err := nodeExecutionFingerprintValue(candidates)
	if err != nil {
		t.Fatal(err)
	}
	expected := NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyExpected{
		Executor: executor.expected, TransitionReceiptFingerprint: receipt.ReceiptFingerprint, TerminalTaskID: terminalTaskID, Candidates: candidates,
		DecisionAuthenticationID: "scheduling-authentication-001", DecisionAuthenticationDigest: "sha256:abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
	}
	fixture := NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture{
		Schema: NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixtureSchema, DecisionID: "scheduling-decision-001", ReplayIdentity: "scheduling-replay-001",
		AuthenticationID: expected.DecisionAuthenticationID, AuthenticationDigest: expected.DecisionAuthenticationDigest, Decision: decision,
		TransitionReceiptID: receipt.TransitionReceiptID, TransitionReceiptFingerprint: receipt.ReceiptFingerprint, GraphRunID: receipt.PolicyBinding.GraphRunID,
		TerminalTaskID: terminalTaskID, Route: receipt.Route, Candidates: cloneNodeConnectorPlacementExecutionGraphNextTaskSchedulingCandidates(candidates), CandidatesFingerprint: candidateFingerprint,
		Provenance: "fixture_only_forgepipe_local_graph_next_task_scheduling_policy_decision",
	}
	if terminal == "succeeded" {
		expected.SchedulingRequestID = "scheduling-request-001"
		if decision == "approved" {
			fixture.SelectedTaskID = candidates[0].TaskID
			fixture.SchedulingRequestID = expected.SchedulingRequestID
			fixture.Authority.NextTaskSchedulingExecutorAttempt = true
		}
	}
	return &nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture{root: executor.root, executor: executor, receipt: receipt, expected: expected, fixture: fixture, recordPaths: executor.recordPaths}
}

func mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture) *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies {
	t.Helper()
	policies, err := OpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return policies
}

func mustDecideNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture) (NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) {
	t.Helper()
	return mustDecideNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyWithPolicies(t, mustOpenNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies(t, value), value.fixture)
}

func mustDecideNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyWithPolicies(t *testing.T, policies *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicies, fixture NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) (NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) {
	t.Helper()
	decision, request, err := policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}

func mustMarshalNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicy(t *testing.T, value NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionFixture) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func snapshotNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRecords(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture) [][]byte {
	t.Helper()
	result := make([][]byte, len(value.recordPaths))
	for index, path := range value.recordPaths {
		result[index] = mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path)
	}
	return result
}

func assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRecordsEqual(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyTestFixture, expected [][]byte) {
	t.Helper()
	for index, path := range value.recordPaths {
		if !bytes.Equal(expected[index], mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path)) {
			t.Fatalf("next-task scheduling policy mutated dependency record %d", index)
		}
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyArtifactsAbsent(t *testing.T, root string) {
	t.Helper()
	assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyArtifactAbsent(t, root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecisionName)
	assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyArtifactAbsent(t, root, nodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequestName)
}

func assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyArtifactAbsent(t *testing.T, root, name string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
		t.Fatalf("next-task scheduling policy unexpectedly published %s", name)
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyNarrowAuthority(t *testing.T, decision NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision, request *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest) {
	t.Helper()
	if decision.Authority != (NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyAuthority{}) || request == nil || request.Authority != (NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyAuthority{NextTaskSchedulingExecutorAttempt: true}) {
		t.Fatal("next-task scheduling policy widened or omitted its sole future executor-attempt authority")
	}
	raw, err := json.Marshal(struct {
		Decision NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyDecision `json:"decision"`
		Request  *NodeConnectorPlacementExecutionGraphNextTaskSchedulingPolicyRequest `json:"request"`
	}{decision, request})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"queue_mutation":true`, `"scheduling_mutation":true`, `"task_launch":true`, `"node_execution":true`, `"retry":true`, `"repair":true`, `"cancellation":true`, `"publication":true`, `"broker":true`, `"provider":true`, `"forgepipe":true`, `"remote_execution":true`, `"network":true`, `"validation":true`, `"checkout":true`, `"git":true`, `"commit":true`, `"push":true`, `"authorization_consumed":true`, `"scheduling_invoked":true`, `"task_launched":true`, `"callbacks_invoked":true`, `"external_actions_invoked":true`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("scheduling policy escalated mutation, launch, callback, or external authority: %s", forbidden)
		}
	}
}

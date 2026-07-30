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
	"testing"
)

type nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture struct {
	root               string
	expected           NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected
	fixture            NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture
	projectionDecision NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision
	projectionRequest  NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequiresIndependentSuccessOrFailureAuthority(t *testing.T) {
	for _, terminal := range []string{"succeeded", "failed"} {
		t.Run(terminal, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture(t, terminal, "approved")
			before := mustListNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRoot(t, value.root)
			decision, request := mustDecideNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t, mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(t, value), value.fixture)
			if decision.ProjectedTerminalPostState != terminal || request == nil || request.ProjectedTerminalPostState != terminal {
				t.Fatal("executor policy did not preserve the exact projected terminal post-state")
			}
			if decision.Authority != (NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyAuthority{}) || request.Authority != (NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyAuthority{LocalGraphStateProjectionExecutorAttempt: true}) || request.AuthorizationConsumed || request.ExecutorInvoked {
				t.Fatal("executor policy did not preserve its narrow future one-time authority")
			}
			if !reflect.DeepEqual(request.StorePrecondition, value.expected.StorePrecondition) || !reflect.DeepEqual(request.TaskBindings, value.projectionRequest.TaskBindings) || request.ProjectionDecisionFingerprint != value.projectionDecision.DecisionFingerprint || request.ProjectionRequestFingerprint != value.projectionRequest.RequestFingerprint {
				t.Fatal("executor policy request omitted a store, terminal, or immutable predecessor binding")
			}
			if request.Requirements != nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequiredGuarantees() || !request.OneTimeRequest || !request.FixtureOwned {
				t.Fatal("executor policy request omitted compare-and-swap, atomicity, replay, recovery, or audit requirements")
			}
			assertNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyNoSideEffects(t, decision, request)
			assertNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyOnlyArtifactsAdded(t, value.root, before, []string{nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionName, nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequestName})
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRejectedAndProjectionOnlyEmitNoRequest(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture(t, "succeeded", "rejected")
	before := mustListNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRoot(t, value.root)
	policies := mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(t, value)
	assertNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyOnlyArtifactsAdded(t, value.root, before, nil)
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t, policies, value.fixture)
	if decision.Decision != "rejected" || decision.ProjectedTerminalPostState != "succeeded" || request != nil {
		t.Fatal("rejected executor policy granted a request or lost its exact proposed terminal state")
	}
	assertNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyNoSideEffects(t, decision, request)
	assertNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyOnlyArtifactsAdded(t, value.root, before, []string{nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionName})
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyReplayRestartConcurrencyAndConflictsFailClosed(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture(t, "succeeded", "approved")
	policies := mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(t, value)
	first, firstRequest := mustDecideNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t, policies, value.fixture)
	second, secondRequest := mustDecideNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t, policies, value.fixture)
	third, thirdRequest := mustDecideNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t, mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(t, value), value.fixture)
	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(first, third) || !reflect.DeepEqual(firstRequest, secondRequest) || !reflect.DeepEqual(firstRequest, thirdRequest) {
		t.Fatal("exact replay or restart changed executor policy authority")
	}

	raw := mustMarshalNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t, value.fixture)
	const callers = 12
	var wait sync.WaitGroup
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, _, err := policies.Decide(raw)
			errs <- err
		}()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	conflicting := value.fixture
	conflicting.DecisionID = "graph-executor-policy-decision-conflict-001"
	if _, _, err := policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t, conflicting)); err == nil {
		t.Fatal("conflicting executor policy decision was accepted")
	}
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRejectsChangedBindingsAndStalePreimages(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture)
	}{
		{name: "terminal state", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.ProjectedTerminalPostState = "failed"
		}},
		{name: "graph store", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.StorePrecondition.GraphStoreID = "local-graph-store-conflict-001"
		}},
		{name: "graph record", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.StorePrecondition.GraphRecordID = "graph-record-conflict-001"
		}},
		{name: "preimage fingerprint", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.StorePrecondition.ExpectedPreimageFingerprint = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{name: "preimage version", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.StorePrecondition.ExpectedPreimageVersion++
		}},
		{name: "graph run", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.GraphRunID = "graph-run-conflict-001"
		}},
		{name: "run", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.TaskBindings[0].RunID = "run-conflict-001"
		}},
		{name: "task", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.TaskBindings[0].TaskID = "task-conflict-001"
		}},
		{name: "operation", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.TaskBindings[0].OperationID = "operation-conflict-001"
		}},
		{name: "receipt", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.TaskBindings[0].ReceiptID = "receipt-conflict-001"
		}},
		{name: "receipt fingerprint", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.TaskBindings[0].ReceiptFingerprint = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
		{name: "outcome", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.TaskBindings[0].TaskOutcome = "failed"
		}},
		{name: "outcome fingerprint", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.TaskBindings[0].OutcomeFingerprint = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
		}},
		{name: "finalization decision", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.FinalizationDecisionFingerprint = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
		}},
		{name: "finalization request", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.FinalizationRequestFingerprint = "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		}},
		{name: "projection decision", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.ProjectionDecisionFingerprint = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		}},
		{name: "projection request", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.ProjectionRequestFingerprint = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
		}},
		{name: "atomicity requirement", mutate: func(f *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) {
			f.Requirements.OneRecordAtomicityRequired = false
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture(t, "succeeded", "approved")
			test.mutate(&value.fixture)
			if _, _, err := mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(t, value).Decide(mustMarshalNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t, value.fixture)); err == nil {
				t.Fatal("changed store, preimage, terminal, predecessor, or execution requirement was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyAbsent(t, value.root)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRejectsMissingTamperedAndOrphanedEvidence(t *testing.T) {
	t.Run("missing projection request", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture(t, "succeeded", "approved")
		if err := os.Remove(filepath.Join(value.root, nodeConnectorPlacementExecutionGraphFinalStateProjectionRequestName)); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(value.root, value.expected); err == nil {
			t.Fatal("missing accepted projection request was ignored")
		}
	})

	t.Run("tampered projection request", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture(t, "succeeded", "approved")
		path := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphFinalStateProjectionRequestName)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, bytes.Replace(raw, []byte(`"final_state": "succeeded"`), []byte(`"final_state": "failed"`), 1), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(value.root, value.expected); err == nil {
			t.Fatal("tampered accepted projection request was ignored")
		}
	})

	t.Run("orphaned policy request", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture(t, "succeeded", "approved")
		mustDecideNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t, mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(t, value), value.fixture)
		if err := os.Remove(filepath.Join(value.root, nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionName)); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(value.root, value.expected); err == nil {
			t.Fatal("orphaned executor policy request was accepted")
		}
	})
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRejectsMalformedNoncanonicalOversizedAndTamperedArtifacts(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture(t, "succeeded", "approved")
	policies := mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(t, value)
	raw := mustMarshalNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t, value.fixture)
	var expanded any
	if err := json.Unmarshal(raw, &expanded); err != nil {
		t.Fatal(err)
	}
	pretty, err := json.MarshalIndent(expanded, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	unknown := append(bytes.TrimSuffix(append([]byte(nil), raw...), []byte("}")), []byte(`,"unknown":true}`)...)
	for _, invalid := range [][]byte{nil, pretty, append(append([]byte(nil), raw...), '\n'), unknown, bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionMaxBytes+1)} {
		if _, _, err := policies.Decide(invalid); err == nil {
			t.Fatal("malformed, noncanonical, trailing, unknown-field, or oversized policy input was accepted")
		}
	}
	assertNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyAbsent(t, value.root)

	_, request := mustDecideNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t, policies, value.fixture)
	if request == nil {
		t.Fatal("approved executor policy omitted its request")
	}
	path := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequestName)
	durable, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Replace(durable, []byte(`"expected_preimage_version": 17`), []byte(`"expected_preimage_version": 18`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(value.root, value.expected); err == nil {
		t.Fatal("tampered durable executor policy request was accepted")
	}
}

func TestNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyAtomicPublicationRecoversWithoutExecution(t *testing.T) {
	t.Run("decision failure", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture(t, "succeeded", "approved")
		policies := mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(t, value)
		original := nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyWriteDecisionAtomic
		nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyWriteDecisionAtomic = func(string, any) error { return errors.New("injected policy decision write failure") }
		t.Cleanup(func() { nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyWriteDecisionAtomic = original })
		if _, _, err := policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t, value.fixture)); err == nil {
			t.Fatal("executor policy decision write failure was accepted")
		}
		assertNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyAbsent(t, value.root)
	})

	t.Run("request failure and restart recovery", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture(t, "succeeded", "approved")
		policies := mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(t, value)
		original := nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyWriteRequestAtomic
		nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyWriteRequestAtomic = func(string, any) error { return errors.New("injected policy request write failure") }
		t.Cleanup(func() { nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyWriteRequestAtomic = original })
		if _, _, err := policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t, value.fixture)); err == nil {
			t.Fatal("executor policy request write failure was accepted")
		}
		if _, err := os.Lstat(filepath.Join(value.root, nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionName)); err != nil {
			t.Fatal("durable executor policy decision was lost after request publication failure")
		}
		if _, err := os.Lstat(filepath.Join(value.root, nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequestName)); !os.IsNotExist(err) {
			t.Fatal("executor policy request publication failure left a partial request")
		}
		nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyWriteRequestAtomic = original
		decision, request := mustDecideNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t, mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(t, value), value.fixture)
		assertNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyNoSideEffects(t, decision, request)
	})
}

func newNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture(t *testing.T, terminal, decision string) *nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture {
	t.Helper()
	projection := newNodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture(t, terminal, "approved")
	projectionDecision, projectionRequestPointer := mustDecideNodeConnectorPlacementExecutionGraphFinalStateProjection(t, mustOpenNodeConnectorPlacementExecutionGraphFinalStateProjections(t, projection), projection.fixture)
	projectionRequest := *projectionRequestPointer
	precondition := NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyStorePrecondition{
		GraphStoreID: "local-graph-store-001", GraphRecordID: "graph-record-001",
		ExpectedPreimageFingerprint: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", ExpectedPreimageVersion: 17,
	}
	expected := NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyExpected{
		Projection: projection.expected, ProjectionDecisionFingerprint: projectionDecision.DecisionFingerprint,
		ProjectionRequestFingerprint: projectionRequest.RequestFingerprint, StorePrecondition: precondition,
	}
	fixture := NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture{
		Schema: NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixtureSchema, DecisionID: "graph-executor-policy-decision-001", ReplayIdentity: "graph-executor-policy-replay-001", Decision: decision,
		ProjectedTerminalPostState: projectionRequest.FinalState, StorePrecondition: precondition, GraphRunID: projectionRequest.GraphRunID,
		TaskBindings:           cloneNodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBindings(projectionRequest.TaskBindings),
		FinalizationDecisionID: projectionRequest.FinalizationDecisionID, FinalizationDecisionFingerprint: projectionRequest.FinalizationDecisionFingerprint, FinalizationRequestID: projectionRequest.FinalizationRequestID, FinalizationRequestFingerprint: projectionRequest.FinalizationRequestFingerprint,
		ProjectionDecisionID: projectionDecision.DecisionID, ProjectionDecisionFingerprint: projectionDecision.DecisionFingerprint, ProjectionRequestID: projectionRequest.RequestID, ProjectionRequestFingerprint: projectionRequest.RequestFingerprint,
		Requirements: nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequiredGuarantees(), Provenance: "fixture_only_forgepipe_local_graph_lifecycle_executor_policy_decision",
	}
	if decision == "approved" {
		fixture.ExecutorRequestID = "graph-executor-policy-request-001"
	}
	return &nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture{root: projection.root, expected: expected, fixture: fixture, projectionDecision: projectionDecision, projectionRequest: projectionRequest}
}

func mustOpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(t *testing.T, value *nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyTestFixture) *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies {
	t.Helper()
	policies, err := OpenNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return policies
}

func mustMarshalNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t *testing.T, value NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustDecideNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t *testing.T, policies *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicies, fixture NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionFixture) (NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision, *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest) {
	t.Helper()
	decision, request, err := policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicy(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}

func assertNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyAbsent(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecisionName, nodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequestName} {
		if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("rejected graph lifecycle executor policy published %s", name)
		}
	}
}

func assertNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyNoSideEffects(t *testing.T, decision NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision, request *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest) {
	t.Helper()
	raw, err := json.Marshal(struct {
		Decision NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyDecision `json:"decision"`
		Request  *NodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRequest `json:"request"`
	}{decision, request})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"graph_mutation":true`, `"graph_completion":true`, `"graph_failure":true`, `"dependency_release":true`, `"next_task":true`, `"retry":true`, `"repair":true`, `"cancellation":true`, `"execution":true`, `"broker":true`, `"forgepipe":true`, `"provider":true`, `"validation":true`, `"checkout":true`, `"git":true`, `"publication":true`, `"lifecycle":true`, `"authorization_consumed":true`, `"executor_invoked":true`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("forbidden executor or lifecycle effect appeared: %s", forbidden)
		}
	}
}

func mustListNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRoot(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	sort.Strings(names)
	return names
}

func assertNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyOnlyArtifactsAdded(t *testing.T, root string, before, expectedAdded []string) {
	t.Helper()
	after := mustListNodeConnectorPlacementExecutionGraphLifecycleExecutorPolicyRoot(t, root)
	seen := make(map[string]bool, len(before))
	for _, name := range before {
		seen[name] = true
	}
	added := make([]string, 0, len(expectedAdded))
	for _, name := range after {
		if !seen[name] {
			added = append(added, name)
		}
	}
	sort.Strings(expectedAdded)
	if len(added) != len(expectedAdded) {
		t.Fatalf("executor policy created forbidden graph or lifecycle artifacts: got %v want %v", added, expectedAdded)
	}
	for i := range added {
		if added[i] != expectedAdded[i] {
			t.Fatalf("executor policy created forbidden graph or lifecycle artifacts: got %v want %v", added, expectedAdded)
		}
	}
}

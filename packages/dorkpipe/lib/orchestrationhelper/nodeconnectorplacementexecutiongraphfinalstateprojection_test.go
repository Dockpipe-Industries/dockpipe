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

type nodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture struct {
	root                 string
	expected             NodeConnectorPlacementExecutionGraphFinalStateProjectionExpected
	fixture              NodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionFixture
	finalizationDecision NodeConnectorPlacementExecutionGraphFinalizationDecision
	finalizationRequest  NodeConnectorPlacementExecutionGraphFinalizationRequest
}

func TestNodeConnectorPlacementExecutionGraphFinalStateProjectionRequiresExplicitSuccessOrFailureAuthority(t *testing.T) {
	for _, terminal := range []string{"succeeded", "failed"} {
		t.Run(terminal, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture(t, terminal, "approved")
			before := mustListNodeConnectorPlacementExecutionGraphFinalStateProjectionRoot(t, value.root)
			decision, request := mustDecideNodeConnectorPlacementExecutionGraphFinalStateProjection(t, mustOpenNodeConnectorPlacementExecutionGraphFinalStateProjections(t, value), value.fixture)
			if decision.FinalState != terminal || request == nil || request.FinalState != terminal || decision.FinalizationAuthority != (NodeConnectorPlacementExecutionGraphFinalizationAuthority{LocalGraphFinalization: true}) || !decision.FinalizationAuthorityAccepted || decision.Authority != (NodeConnectorPlacementExecutionGraphFinalStateProjectionAuthority{}) || request.Authority != (NodeConnectorPlacementExecutionGraphFinalStateProjectionAuthority{LocalFinalStateProjection: true}) {
				t.Fatal("explicit graph final-state projection did not preserve the accepted terminal state and narrow authority")
			}
			if request.FinalizationDecisionFingerprint != value.finalizationDecision.DecisionFingerprint || request.FinalizationRequestFingerprint != value.finalizationRequest.RequestFingerprint || !reflect.DeepEqual(request.TaskBindings, nodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBindings(value.expected.Finalization.Outcomes)) {
				t.Fatal("projection request omitted or changed immutable graph, run, task, operation, outcome, or prior-authority bindings")
			}
			assertNodeConnectorPlacementExecutionGraphFinalStateProjectionNoSideEffects(t, decision, request)
			assertNodeConnectorPlacementExecutionGraphFinalStateProjectionOnlyArtifactsAdded(t, value.root, before, []string{nodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionName, nodeConnectorPlacementExecutionGraphFinalStateProjectionRequestName})
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphFinalStateProjectionRejectedDecisionEmitsNoRequest(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture(t, "succeeded", "rejected")
	before := mustListNodeConnectorPlacementExecutionGraphFinalStateProjectionRoot(t, value.root)
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphFinalStateProjection(t, mustOpenNodeConnectorPlacementExecutionGraphFinalStateProjections(t, value), value.fixture)
	if decision.Decision != "rejected" || decision.FinalState != "" || request != nil {
		t.Fatal("rejected projection decision granted a request or final state")
	}
	assertNodeConnectorPlacementExecutionGraphFinalStateProjectionNoSideEffects(t, decision, request)
	assertNodeConnectorPlacementExecutionGraphFinalStateProjectionOnlyArtifactsAdded(t, value.root, before, []string{nodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionName})
}

func TestNodeConnectorPlacementExecutionGraphFinalStateProjectionReplayRestartConcurrencyAndConflictsFailClosed(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture(t, "succeeded", "approved")
	projections := mustOpenNodeConnectorPlacementExecutionGraphFinalStateProjections(t, value)
	first, firstRequest := mustDecideNodeConnectorPlacementExecutionGraphFinalStateProjection(t, projections, value.fixture)
	second, secondRequest := mustDecideNodeConnectorPlacementExecutionGraphFinalStateProjection(t, projections, value.fixture)
	third, thirdRequest := mustDecideNodeConnectorPlacementExecutionGraphFinalStateProjection(t, mustOpenNodeConnectorPlacementExecutionGraphFinalStateProjections(t, value), value.fixture)
	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(first, third) || !reflect.DeepEqual(firstRequest, secondRequest) || !reflect.DeepEqual(firstRequest, thirdRequest) {
		t.Fatal("exact replay or restart changed graph final-state projection authority")
	}

	raw := mustMarshalNodeConnectorPlacementExecutionGraphFinalStateProjection(t, value.fixture)
	const callers = 12
	var wait sync.WaitGroup
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, _, err := projections.Decide(raw)
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
	conflicting.DecisionID = "graph-projection-decision-conflict-001"
	if _, _, err := projections.Decide(mustMarshalNodeConnectorPlacementExecutionGraphFinalStateProjection(t, conflicting)); err == nil {
		t.Fatal("conflicting durable projection decision was accepted")
	}
}

func TestNodeConnectorPlacementExecutionGraphFinalStateProjectionRejectsMissingTamperedAndChangedEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*nodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture)
	}{
		{name: "mismatched terminal state", mutate: func(v *nodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture) {
			v.fixture.FinalState = "failed"
		}},
		{name: "changed graph run", mutate: func(v *nodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture) {
			v.fixture.GraphRunID = "graph-run-projection-conflict-001"
		}},
		{name: "changed task", mutate: func(v *nodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture) {
			v.fixture.TaskBindings[0].TaskID = "task-projection-conflict-001"
		}},
		{name: "changed operation", mutate: func(v *nodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture) {
			v.fixture.TaskBindings[0].OperationID = "operation-projection-conflict-001"
		}},
		{name: "changed outcome", mutate: func(v *nodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture) {
			v.fixture.TaskBindings[0].TaskOutcome = "failed"
		}},
		{name: "changed finalization decision", mutate: func(v *nodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture) {
			v.fixture.FinalizationDecisionFingerprint = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{name: "changed finalization request", mutate: func(v *nodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture) {
			v.fixture.FinalizationRequestFingerprint = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture(t, "succeeded", "approved")
			test.mutate(value)
			if _, _, err := mustOpenNodeConnectorPlacementExecutionGraphFinalStateProjections(t, value).Decide(mustMarshalNodeConnectorPlacementExecutionGraphFinalStateProjection(t, value.fixture)); err == nil {
				t.Fatal("changed identity, outcome, terminal state, or prior authority was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphFinalStateProjectionAbsent(t, value.root)
		})
	}

	t.Run("missing prior request", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture(t, "succeeded", "approved")
		if err := os.Remove(filepath.Join(value.root, nodeConnectorPlacementExecutionGraphFinalizationRequestName)); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphFinalStateProjections(value.root, value.expected); err == nil {
			t.Fatal("missing accepted graph-finalization request was ignored")
		}
	})

	t.Run("tampered prior request", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture(t, "succeeded", "approved")
		path := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphFinalizationRequestName)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, bytes.Replace(raw, []byte(`"finalization": "succeeded"`), []byte(`"finalization": "failed"`), 1), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphFinalStateProjections(value.root, value.expected); err == nil {
			t.Fatal("tampered accepted graph-finalization request was ignored")
		}
	})
}

func TestNodeConnectorPlacementExecutionGraphFinalStateProjectionRejectsMalformedNoncanonicalAndTamperedArtifacts(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture(t, "succeeded", "approved")
	projections := mustOpenNodeConnectorPlacementExecutionGraphFinalStateProjections(t, value)
	raw := mustMarshalNodeConnectorPlacementExecutionGraphFinalStateProjection(t, value.fixture)
	var expanded any
	if err := json.Unmarshal(raw, &expanded); err != nil {
		t.Fatal(err)
	}
	pretty, err := json.MarshalIndent(expanded, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range [][]byte{nil, pretty, append(bytes.TrimSuffix(raw, []byte("}")), []byte(`,"unknown":true}`)...)} {
		if _, _, err := projections.Decide(malformed); err == nil {
			t.Fatal("missing, malformed, noncanonical, or unknown fixture input was accepted")
		}
	}
	assertNodeConnectorPlacementExecutionGraphFinalStateProjectionAbsent(t, value.root)

	_, request := mustDecideNodeConnectorPlacementExecutionGraphFinalStateProjection(t, projections, value.fixture)
	path := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphFinalStateProjectionRequestName)
	durable, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Replace(durable, []byte(`"final_state": "succeeded"`), []byte(`"final_state": "failed"`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if request == nil {
		t.Fatal("approved projection omitted its request")
	}
	if _, err := OpenNodeConnectorPlacementExecutionGraphFinalStateProjections(value.root, value.expected); err == nil {
		t.Fatal("tampered durable projection request was accepted")
	}
}

func TestNodeConnectorPlacementExecutionGraphFinalStateProjectionAtomicRequestFailureRecoversWithoutLifecycleEffects(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture(t, "succeeded", "approved")
	projections := mustOpenNodeConnectorPlacementExecutionGraphFinalStateProjections(t, value)
	original := nodeConnectorPlacementExecutionGraphFinalStateProjectionWriteRequestAtomic
	nodeConnectorPlacementExecutionGraphFinalStateProjectionWriteRequestAtomic = func(string, any) error { return errors.New("injected projection request write failure") }
	t.Cleanup(func() { nodeConnectorPlacementExecutionGraphFinalStateProjectionWriteRequestAtomic = original })
	if _, _, err := projections.Decide(mustMarshalNodeConnectorPlacementExecutionGraphFinalStateProjection(t, value.fixture)); err == nil {
		t.Fatal("projection request write failure was accepted")
	}
	if _, err := os.Lstat(filepath.Join(value.root, nodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionName)); err != nil {
		t.Fatal("durable projection decision was lost after request publication failure")
	}
	if _, err := os.Lstat(filepath.Join(value.root, nodeConnectorPlacementExecutionGraphFinalStateProjectionRequestName)); !os.IsNotExist(err) {
		t.Fatal("projection request publication failure left a partial request")
	}
	nodeConnectorPlacementExecutionGraphFinalStateProjectionWriteRequestAtomic = original
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphFinalStateProjection(t, mustOpenNodeConnectorPlacementExecutionGraphFinalStateProjections(t, value), value.fixture)
	assertNodeConnectorPlacementExecutionGraphFinalStateProjectionNoSideEffects(t, decision, request)
}

func newNodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture(t *testing.T, terminal, decision string) *nodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture {
	t.Helper()
	finalization := newNodeConnectorPlacementExecutionGraphFinalizationTestFixture(t, terminal, "approved")
	finalizationDecision, finalizationRequestPointer := mustDecideNodeConnectorPlacementExecutionGraphFinalization(t, mustOpenNodeConnectorPlacementExecutionGraphFinalizations(t, finalization), finalization.fixture)
	finalizationRequest := *finalizationRequestPointer
	expected := NodeConnectorPlacementExecutionGraphFinalStateProjectionExpected{
		Finalization: finalization.expected, FinalizationDecisionFingerprint: finalizationDecision.DecisionFingerprint, FinalizationRequestFingerprint: finalizationRequest.RequestFingerprint,
	}
	fixture := NodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionFixture{
		Schema: NodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionFixtureSchema, DecisionID: "graph-projection-decision-001", ReplayIdentity: "graph-projection-replay-001", Decision: decision,
		GraphRunID: finalization.expected.GraphRunID, TaskBindings: nodeConnectorPlacementExecutionGraphFinalStateProjectionTaskBindings(finalization.expected.Outcomes),
		FinalizationDecisionID: finalizationDecision.DecisionID, FinalizationDecisionFingerprint: finalizationDecision.DecisionFingerprint, FinalizationRequestID: finalizationRequest.RequestID, FinalizationRequestFingerprint: finalizationRequest.RequestFingerprint,
		Provenance: "fixture_only_forgepipe_local_graph_final_state_projection_decision",
	}
	if decision == "approved" {
		fixture.FinalState = finalizationRequest.Finalization
		fixture.ProjectionRequestID = "graph-projection-request-001"
	}
	return &nodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture{root: finalization.root, expected: expected, fixture: fixture, finalizationDecision: finalizationDecision, finalizationRequest: finalizationRequest}
}

func mustOpenNodeConnectorPlacementExecutionGraphFinalStateProjections(t *testing.T, value *nodeConnectorPlacementExecutionGraphFinalStateProjectionTestFixture) *NodeConnectorPlacementExecutionGraphFinalStateProjections {
	t.Helper()
	projections, err := OpenNodeConnectorPlacementExecutionGraphFinalStateProjections(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return projections
}

func mustMarshalNodeConnectorPlacementExecutionGraphFinalStateProjection(t *testing.T, value NodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionFixture) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustDecideNodeConnectorPlacementExecutionGraphFinalStateProjection(t *testing.T, projections *NodeConnectorPlacementExecutionGraphFinalStateProjections, fixture NodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionFixture) (NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision, *NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest) {
	t.Helper()
	decision, request, err := projections.Decide(mustMarshalNodeConnectorPlacementExecutionGraphFinalStateProjection(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}

func assertNodeConnectorPlacementExecutionGraphFinalStateProjectionAbsent(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{nodeConnectorPlacementExecutionGraphFinalStateProjectionDecisionName, nodeConnectorPlacementExecutionGraphFinalStateProjectionRequestName} {
		if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("rejected graph final-state projection published %s", name)
		}
	}
}

func assertNodeConnectorPlacementExecutionGraphFinalStateProjectionNoSideEffects(t *testing.T, decision NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision, request *NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest) {
	t.Helper()
	raw, err := json.Marshal(struct {
		Decision NodeConnectorPlacementExecutionGraphFinalStateProjectionDecision `json:"decision"`
		Request  *NodeConnectorPlacementExecutionGraphFinalStateProjectionRequest `json:"request"`
	}{decision, request})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"graph_completion":true`, `"graph_failure":true`, `"dependency_release":true`, `"next_task":true`, `"retry":true`, `"repair":true`, `"cancellation":true`, `"execution":true`, `"broker":true`, `"forgepipe":true`, `"provider":true`, `"validation":true`, `"mutation":true`, `"git":true`, `"publication":true`, `"lifecycle":true`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("forbidden lifecycle authority appeared: %s", forbidden)
		}
	}
}

func mustListNodeConnectorPlacementExecutionGraphFinalStateProjectionRoot(t *testing.T, root string) []string {
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

func assertNodeConnectorPlacementExecutionGraphFinalStateProjectionOnlyArtifactsAdded(t *testing.T, root string, before, expectedAdded []string) {
	t.Helper()
	after := mustListNodeConnectorPlacementExecutionGraphFinalStateProjectionRoot(t, root)
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
	if !reflect.DeepEqual(added, expectedAdded) {
		t.Fatalf("projection created forbidden lifecycle artifacts: got %v want %v", added, expectedAdded)
	}
}

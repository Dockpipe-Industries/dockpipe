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

type nodeConnectorPlacementExecutionGraphFinalizationTestFixture struct {
	root     string
	expected NodeConnectorPlacementExecutionGraphFinalizationExpected
	fixture  NodeConnectorPlacementExecutionGraphFinalizationDecisionFixture
}

func TestNodeConnectorPlacementExecutionGraphFinalizationRequiresExplicitAuthorityAndPreservesTerminalFailure(t *testing.T) {
	for _, terminal := range []string{"succeeded", "failed"} {
		t.Run(terminal, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphFinalizationTestFixture(t, terminal, "approved")
			decision, request := mustDecideNodeConnectorPlacementExecutionGraphFinalization(t, mustOpenNodeConnectorPlacementExecutionGraphFinalizations(t, value), value.fixture)
			want := "succeeded"
			if terminal != "succeeded" {
				want = "failed"
			}
			if decision.Finalization != want || request == nil || request.Finalization != want || request.Authority != (NodeConnectorPlacementExecutionGraphFinalizationAuthority{LocalGraphFinalization: true}) || decision.Authority != (NodeConnectorPlacementExecutionGraphFinalizationAuthority{}) {
				t.Fatal("explicit graph finalization did not preserve the exact terminal outcome and narrow authority")
			}
			assertNodeConnectorPlacementExecutionGraphFinalizationNoSideEffects(t, decision, request)
		})
	}

	t.Run("rejected", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphFinalizationTestFixture(t, "succeeded", "rejected")
		decision, request := mustDecideNodeConnectorPlacementExecutionGraphFinalization(t, mustOpenNodeConnectorPlacementExecutionGraphFinalizations(t, value), value.fixture)
		if decision.Decision != "rejected" || request != nil {
			t.Fatal("rejected local finalization granted a request")
		}
	})
}

func TestNodeConnectorPlacementExecutionGraphFinalizationAggregatesOrdinalTaskOutcomes(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphFinalizationTestFixture(t, "succeeded", "approved")
	second := cloneNodeConnectorPlacementExecutionGraphReconciliation(value.expected.Outcomes[0])
	second.TaskID = "task-finalization-second-001"
	second.OperationID = "operation-finalization-second-001"
	second.TaskOutcome = "failed"
	second.TerminalResult = "failed"
	fingerprint, err := nodeConnectorPlacementExecutionGraphReconciliationFingerprint(second)
	if err != nil {
		t.Fatal(err)
	}
	second.ArtifactFingerprint = fingerprint
	value.expected.Outcomes = []NodeConnectorPlacementExecutionGraphReconciliation{second, value.expected.Outcomes[0]}
	value.fixture.Outcomes = cloneNodeConnectorPlacementExecutionGraphFinalizationOutcomes(value.expected.Outcomes)
	value.fixture.Finalization = "failed"
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphFinalization(t, mustOpenNodeConnectorPlacementExecutionGraphFinalizations(t, value), value.fixture)
	if decision.Finalization != "failed" || request == nil || request.Finalization != "failed" {
		t.Fatal("a mixed ordinal task-outcome set did not deterministically finalize as failed")
	}

	unsorted := newNodeConnectorPlacementExecutionGraphFinalizationTestFixture(t, "succeeded", "approved")
	unsorted.expected.Outcomes = []NodeConnectorPlacementExecutionGraphReconciliation{unsorted.expected.Outcomes[0], second}
	if _, err := OpenNodeConnectorPlacementExecutionGraphFinalizations(unsorted.root, unsorted.expected); err == nil {
		t.Fatal("unordered task outcomes were accepted")
	}
}

func TestNodeConnectorPlacementExecutionGraphFinalizationRejectsInferenceAndIdentityConflicts(t *testing.T) {
	for _, mutation := range []func(*NodeConnectorPlacementExecutionGraphFinalizationDecisionFixture){
		func(f *NodeConnectorPlacementExecutionGraphFinalizationDecisionFixture) { f.Finalization = "failed" },
		func(f *NodeConnectorPlacementExecutionGraphFinalizationDecisionFixture) {
			f.GraphRunID = "graph-run-conflict-001"
		},
		func(f *NodeConnectorPlacementExecutionGraphFinalizationDecisionFixture) {
			f.Outcomes[0].TaskID = "task-conflict-001"
		},
		func(f *NodeConnectorPlacementExecutionGraphFinalizationDecisionFixture) {
			f.ReplayIdentity = f.DecisionID
		},
	} {
		value := newNodeConnectorPlacementExecutionGraphFinalizationTestFixture(t, "succeeded", "approved")
		mutation(&value.fixture)
		raw := mustMarshalNodeConnectorPlacementExecutionGraphFinalization(t, value.fixture)
		if _, _, err := mustOpenNodeConnectorPlacementExecutionGraphFinalizations(t, value).Decide(raw); err == nil {
			t.Fatal("inferred, conflicting, or substituted finalization was accepted")
		}
		assertNodeConnectorPlacementExecutionGraphFinalizationAbsent(t, value.root)
	}
}

func TestNodeConnectorPlacementExecutionGraphFinalizationReplayRestartConcurrencyAndTamperFailClosed(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphFinalizationTestFixture(t, "succeeded", "approved")
	finalizations := mustOpenNodeConnectorPlacementExecutionGraphFinalizations(t, value)
	first, firstRequest := mustDecideNodeConnectorPlacementExecutionGraphFinalization(t, finalizations, value.fixture)
	second, secondRequest := mustDecideNodeConnectorPlacementExecutionGraphFinalization(t, finalizations, value.fixture)
	restarted := mustOpenNodeConnectorPlacementExecutionGraphFinalizations(t, value)
	third, thirdRequest := mustDecideNodeConnectorPlacementExecutionGraphFinalization(t, restarted, value.fixture)
	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(first, third) || !reflect.DeepEqual(firstRequest, secondRequest) || !reflect.DeepEqual(firstRequest, thirdRequest) {
		t.Fatal("replay or restart changed finalization evidence")
	}

	const callers = 12
	var wait sync.WaitGroup
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, _, err := finalizations.Decide(mustMarshalNodeConnectorPlacementExecutionGraphFinalization(t, value.fixture))
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

	path := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphFinalizationRequestName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Replace(raw, []byte(`"finalization": "succeeded"`), []byte(`"finalization": "failed"`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorPlacementExecutionGraphFinalizations(value.root, value.expected); err == nil {
		t.Fatal("tampered finalization request was accepted")
	}
}

func TestNodeConnectorPlacementExecutionGraphFinalizationAtomicRecoveryAndMalformedArtifactsFailClosed(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphFinalizationTestFixture(t, "succeeded", "approved")
	finalizations := mustOpenNodeConnectorPlacementExecutionGraphFinalizations(t, value)
	decisionWriter, requestWriter := nodeConnectorPlacementExecutionGraphFinalizationWriteDecisionAtomic, nodeConnectorPlacementExecutionGraphFinalizationWriteRequestAtomic
	t.Cleanup(func() {
		nodeConnectorPlacementExecutionGraphFinalizationWriteDecisionAtomic = decisionWriter
		nodeConnectorPlacementExecutionGraphFinalizationWriteRequestAtomic = requestWriter
	})
	nodeConnectorPlacementExecutionGraphFinalizationWriteDecisionAtomic = func(string, any) error { return errors.New("injected decision write failure") }
	if _, _, err := finalizations.Decide(mustMarshalNodeConnectorPlacementExecutionGraphFinalization(t, value.fixture)); err == nil {
		t.Fatal("decision write failure was accepted")
	}
	assertNodeConnectorPlacementExecutionGraphFinalizationAbsent(t, value.root)

	nodeConnectorPlacementExecutionGraphFinalizationWriteDecisionAtomic = decisionWriter
	nodeConnectorPlacementExecutionGraphFinalizationWriteRequestAtomic = func(string, any) error { return errors.New("injected request write failure") }
	if _, _, err := finalizations.Decide(mustMarshalNodeConnectorPlacementExecutionGraphFinalization(t, value.fixture)); err == nil {
		t.Fatal("request write failure was accepted")
	}
	if _, err := os.Lstat(filepath.Join(value.root, nodeConnectorPlacementExecutionGraphFinalizationDecisionName)); err != nil {
		t.Fatal("durable decision was lost after request publication failure")
	}
	if _, err := os.Lstat(filepath.Join(value.root, nodeConnectorPlacementExecutionGraphFinalizationRequestName)); !os.IsNotExist(err) {
		t.Fatal("request publication failure left a partial request")
	}

	nodeConnectorPlacementExecutionGraphFinalizationWriteRequestAtomic = requestWriter
	if _, request := mustDecideNodeConnectorPlacementExecutionGraphFinalization(t, mustOpenNodeConnectorPlacementExecutionGraphFinalizations(t, value), value.fixture); request == nil {
		t.Fatal("restart did not recover the exact request from the durable decision")
	}
	path := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphFinalizationRequestName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(bytes.TrimSuffix(raw, []byte("}\n")), []byte(",\n  \"unknown\": true\n}\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenNodeConnectorPlacementExecutionGraphFinalizations(value.root, value.expected); err == nil {
		t.Fatal("noncanonical or unknown durable request artifact was accepted")
	}
}

func newNodeConnectorPlacementExecutionGraphFinalizationTestFixture(t *testing.T, terminal, decision string) *nodeConnectorPlacementExecutionGraphFinalizationTestFixture {
	t.Helper()
	graph := newNodeConnectorPlacementExecutionGraphReconciliationTestFixture(t, terminal)
	outcome := mustReconcileNodeConnectorPlacementExecutionGraph(t, mustOpenNodeConnectorPlacementExecutionGraphReconciliations(t, graph))
	expected := NodeConnectorPlacementExecutionGraphFinalizationExpected{GraphRunID: outcome.GraphRunID, Outcomes: []NodeConnectorPlacementExecutionGraphReconciliation{outcome}}
	fixture := NodeConnectorPlacementExecutionGraphFinalizationDecisionFixture{Schema: NodeConnectorPlacementExecutionGraphFinalizationDecisionFixtureSchema, DecisionID: "graph-finalization-decision-001", ReplayIdentity: "graph-finalization-replay-001", Decision: decision, GraphRunID: outcome.GraphRunID, Outcomes: cloneNodeConnectorPlacementExecutionGraphFinalizationOutcomes(expected.Outcomes), Provenance: "fixture_only_local_graph_finalization_decision"}
	if decision == "approved" {
		fixture.Finalization = nodeConnectorPlacementExecutionGraphFinalizationOutcome(expected.Outcomes)
		fixture.RequestID = "graph-finalization-request-001"
	}
	return &nodeConnectorPlacementExecutionGraphFinalizationTestFixture{root: graph.reconciliation.deliveryValue.handoff.base.root, expected: expected, fixture: fixture}
}

func mustOpenNodeConnectorPlacementExecutionGraphFinalizations(t *testing.T, value *nodeConnectorPlacementExecutionGraphFinalizationTestFixture) *NodeConnectorPlacementExecutionGraphFinalizations {
	t.Helper()
	finalizations, err := OpenNodeConnectorPlacementExecutionGraphFinalizations(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return finalizations
}
func mustMarshalNodeConnectorPlacementExecutionGraphFinalization(t *testing.T, value NodeConnectorPlacementExecutionGraphFinalizationDecisionFixture) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func mustDecideNodeConnectorPlacementExecutionGraphFinalization(t *testing.T, finalizations *NodeConnectorPlacementExecutionGraphFinalizations, fixture NodeConnectorPlacementExecutionGraphFinalizationDecisionFixture) (NodeConnectorPlacementExecutionGraphFinalizationDecision, *NodeConnectorPlacementExecutionGraphFinalizationRequest) {
	t.Helper()
	decision, request, err := finalizations.Decide(mustMarshalNodeConnectorPlacementExecutionGraphFinalization(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}
func assertNodeConnectorPlacementExecutionGraphFinalizationAbsent(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{nodeConnectorPlacementExecutionGraphFinalizationDecisionName, nodeConnectorPlacementExecutionGraphFinalizationRequestName} {
		if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("rejected finalization published %s", name)
		}
	}
}
func assertNodeConnectorPlacementExecutionGraphFinalizationNoSideEffects(t *testing.T, decision NodeConnectorPlacementExecutionGraphFinalizationDecision, request *NodeConnectorPlacementExecutionGraphFinalizationRequest) {
	t.Helper()
	raw, err := json.Marshal(struct {
		Decision NodeConnectorPlacementExecutionGraphFinalizationDecision `json:"decision"`
		Request  *NodeConnectorPlacementExecutionGraphFinalizationRequest `json:"request"`
	}{decision, request})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"graph_completion":true`, `"graph_failure":true`, `"dependency_release":true`, `"next_task":true`, `"retry":true`, `"repair":true`, `"cancellation":true`, `"execution":true`, `"broker":true`, `"provider":true`, `"publication":true`, `"lifecycle":true`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("forbidden lifecycle authority appeared: %s", forbidden)
		}
	}
}

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

type nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture struct {
	root       string
	executor   *nodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture
	receipt    NodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceipt
	expected   NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected
	fixture    NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture
	recordPath string
}

func TestNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRoutesTerminalStatesIndependently(t *testing.T) {
	tests := []struct {
		terminal  string
		route     string
		authority NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority
	}{
		{terminal: "succeeded", route: "dependency_release_transition", authority: NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority{DependencyReleaseTransitionAttempt: true}},
		{terminal: "failed", route: "failure_propagation_transition", authority: NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority{FailurePropagationTransitionAttempt: true}},
	}
	for _, test := range tests {
		t.Run(test.terminal, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture(t, test.terminal, "approved")
			beforeRecord := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)
			decision, request := mustDecideNodeConnectorPlacementExecutionGraphDependencyTransitionPolicy(t, value)
			if decision.Decision != "approved" || decision.Route != test.route || decision.Authority != (NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority{}) || request == nil || request.Route != test.route || request.Authority != test.authority {
				t.Fatal("terminal state did not route through one structurally distinct narrow transition request")
			}
			if !request.OneTimeRequest || request.AuthorizationConsumed || request.TransitionInvoked || request.CallbacksInvoked || !request.FixtureOwned || !decision.IndependentlyAuthenticated || decision.ApprovalInferred {
				t.Fatal("transition request was not an independently authenticated unconsumed fixture request")
			}
			expectedBinding, err := nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyBinding(value.expected, value.receipt)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(request.Binding, expectedBinding) || !reflect.DeepEqual(request.DependencyTargets, value.expected.DependencyTargets) {
				t.Fatal("transition request omitted an executor, predecessor, postimage, or target precondition binding")
			}
			assertNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyNarrowAuthority(t, decision, request)
			if !bytes.Equal(beforeRecord, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)) {
				t.Fatal("dependency-transition policy mutated the graph lifecycle record")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRejectedReceiptAndTerminalAloneEmitNoRequest(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture(t, "succeeded", "rejected")
	policies := mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(t, value)
	assertNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyArtifactAbsent(t, value.root, nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionName)
	assertNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyArtifactAbsent(t, value.root, nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestName)
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyWithPolicies(t, policies, value.fixture)
	if decision.Decision != "rejected" || request != nil || decision.Route != "" || decision.Authority != (NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAuthority{}) {
		t.Fatal("rejected decision emitted transition authority")
	}
	assertNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyArtifactAbsent(t, value.root, nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestName)

	terminalOnly := newNodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture(t, "succeeded", "approved", true)
	targets := nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTargets()
	expected := NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected{Executor: terminalOnly.expected, AuditReceiptFingerprint: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", DependencyTargets: targets, DecisionAuthenticationID: "dependency-transition-authentication-001", DecisionAuthenticationDigest: "sha256:abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd", TransitionRequestID: "dependency-transition-request-001"}
	if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(terminalOnly.root, expected); err == nil {
		t.Fatal("terminal state without the durable audit receipt was accepted")
	}
	assertNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyArtifactAbsent(t, terminalOnly.root, nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestName)
}

func TestNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyReplayRestartConcurrencyAndConflicts(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture(t, "succeeded", "approved")
	policies := mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(t, value)
	first, firstRequest := mustDecideNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyWithPolicies(t, policies, value.fixture)
	second, secondRequest := mustDecideNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyWithPolicies(t, policies, value.fixture)
	restarted := mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(t, value)
	third, thirdRequest := mustDecideNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyWithPolicies(t, restarted, value.fixture)
	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(first, third) || !reflect.DeepEqual(firstRequest, secondRequest) || !reflect.DeepEqual(firstRequest, thirdRequest) {
		t.Fatal("exact replay or restart changed transition request identity or authority")
	}

	raw := mustMarshalNodeConnectorPlacementExecutionGraphDependencyTransitionPolicy(t, value.fixture)
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

	conflict := value.fixture
	conflict.DecisionID = "dependency-transition-decision-conflict-001"
	if _, _, err := policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphDependencyTransitionPolicy(t, conflict)); err == nil {
		t.Fatal("conflicting decision for the same execution was accepted")
	}
}

func TestNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRejectsRouteAuthorityAndTargetConflicts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture)
	}{
		{name: "wrong route", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.Route = "failure_propagation_transition"
		}},
		{name: "both route authorities", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.Authority.FailurePropagationTransitionAttempt = true
		}},
		{name: "actual dependency release", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.Authority.DependencyRelease = true
		}},
		{name: "scheduling", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.Authority.NextTaskScheduling = true
		}},
		{name: "empty targets", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.DependencyTargets = nil
		}},
		{name: "duplicate targets", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.DependencyTargets[1] = f.DependencyTargets[0]
		}},
		{name: "unsorted targets", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.DependencyTargets[0], f.DependencyTargets[1] = f.DependencyTargets[1], f.DependencyTargets[0]
		}},
		{name: "changed target", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.DependencyTargets[0].DependencyRecordID = "dependency-record-changed-001"
		}},
		{name: "target fingerprint", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.DependencyTargetsFingerprint = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture(t, "succeeded", "approved")
			test.mutate(&value.fixture)
			if _, _, err := mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(t, value).Decide(mustMarshalNodeConnectorPlacementExecutionGraphDependencyTransitionPolicy(t, value.fixture)); err == nil {
				t.Fatal("conflicting route, authority, or target encoding was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyArtifactsAbsent(t, value.root)
		})
	}

	for _, mutate := range []func(*NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected){
		func(e *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected) {
			e.DependencyTargets = nil
		},
		func(e *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected) {
			e.DependencyTargets[1] = e.DependencyTargets[0]
		},
		func(e *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected) {
			e.DependencyTargets[0], e.DependencyTargets[1] = e.DependencyTargets[1], e.DependencyTargets[0]
		},
	} {
		value := newNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture(t, "succeeded", "approved")
		mutate(&value.expected)
		if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(value.root, value.expected); err == nil {
			t.Fatal("invalid caller-authored target set was normalized instead of rejected")
		}
	}
}

func TestNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRejectsChangedImmutableBindings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture)
	}{
		{name: "audit receipt identity", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.AuditReceiptID = "audit-receipt-conflict-001"
		}},
		{name: "audit receipt fingerprint", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.AuditReceiptFingerprint = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
		}},
		{name: "store", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.GraphStoreID = "graph-store-conflict-001"
		}},
		{name: "record", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.GraphRecordID = "graph-record-conflict-001"
		}},
		{name: "graph run", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.GraphRunID = "graph-run-conflict-001"
		}},
		{name: "postimage", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.PostimageFingerprint = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
		}},
		{name: "version", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.PostimageVersion++
		}},
		{name: "terminal state", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.TerminalGraphState = "failed"
		}},
		{name: "request identity", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.TransitionRequestID = "transition-request-conflict-001"
		}},
		{name: "authentication identity", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.AuthenticationID = "authentication-conflict-001"
		}},
		{name: "authentication digest", mutate: func(f *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) {
			f.AuthenticationDigest = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture(t, "succeeded", "approved")
			test.mutate(&value.fixture)
			if _, _, err := mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(t, value).Decide(mustMarshalNodeConnectorPlacementExecutionGraphDependencyTransitionPolicy(t, value.fixture)); err == nil {
				t.Fatal("changed receipt, record, terminal, request, or authentication binding was accepted")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRejectsChangedPredecessorExpectedBindings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected)
	}{
		{name: "executor audit receipt", mutate: func(e *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected) {
			e.AuditReceiptFingerprint = "sha256:6666666666666666666666666666666666666666666666666666666666666666"
		}},
		{name: "lifecycle policy decision", mutate: func(e *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected) {
			e.Executor.PolicyDecisionFingerprint = "sha256:7777777777777777777777777777777777777777777777777777777777777777"
		}},
		{name: "lifecycle transition request", mutate: func(e *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected) {
			e.Executor.PolicyRequestFingerprint = "sha256:8888888888888888888888888888888888888888888888888888888888888888"
		}},
		{name: "projection decision", mutate: func(e *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected) {
			e.Executor.Policy.ProjectionDecisionFingerprint = "sha256:9999999999999999999999999999999999999999999999999999999999999999"
		}},
		{name: "projection request", mutate: func(e *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected) {
			e.Executor.Policy.ProjectionRequestFingerprint = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{name: "finalization decision", mutate: func(e *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected) {
			e.Executor.Policy.Projection.FinalizationDecisionFingerprint = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		}},
		{name: "finalization request", mutate: func(e *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected) {
			e.Executor.Policy.Projection.FinalizationRequestFingerprint = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
		}},
		{name: "reconciliation decision", mutate: func(e *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected) {
			e.Executor.Policy.Projection.Finalization.Outcomes[0].ReconciliationDecisionFingerprint = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
		}},
		{name: "outcome", mutate: func(e *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected) {
			e.Executor.Policy.Projection.Finalization.Outcomes[0].ArtifactFingerprint = "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
		}},
		{name: "preimage", mutate: func(e *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected) {
			e.Executor.Policy.StorePrecondition.ExpectedPreimageFingerprint = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture(t, "succeeded", "approved")
			test.mutate(&value.expected)
			if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(value.root, value.expected); err == nil {
				t.Fatal("changed lifecycle-policy, projection, finalization, reconciliation, outcome, transition, or execution identity was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyArtifactsAbsent(t, value.root)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRejectsMalformedDecisionFixtures(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{name: "empty", mutate: func([]byte) []byte { return nil }},
		{name: "malformed", mutate: func([]byte) []byte { return []byte("{") }},
		{name: "unknown field", mutate: func(raw []byte) []byte { return bytes.Replace(raw, []byte("}"), []byte(",\"unknown\":true}"), 1) }},
		{name: "trailing data", mutate: func(raw []byte) []byte { return append(append([]byte(nil), raw...), []byte("{}")...) }},
		{name: "oversized", mutate: func([]byte) []byte {
			return bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionMaxBytes+1)
		}},
		{name: "noncanonical", mutate: func(raw []byte) []byte {
			var value any
			_ = json.Unmarshal(raw, &value)
			pretty, _ := json.MarshalIndent(value, "", "  ")
			return pretty
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture(t, "succeeded", "approved")
			raw := test.mutate(mustMarshalNodeConnectorPlacementExecutionGraphDependencyTransitionPolicy(t, value.fixture))
			if _, _, err := mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(t, value).Decide(raw); err == nil {
				t.Fatal("malformed, unknown-field, trailing, oversized, or noncanonical decision fixture was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyArtifactsAbsent(t, value.root)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRejectsMissingStaleConflictingAndMalformedEvidence(t *testing.T) {
	t.Run("missing receipt", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture(t, "succeeded", "approved")
		if err := os.Remove(filepath.Join(value.root, nodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceiptName)); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(value.root, value.expected); err == nil {
			t.Fatal("missing executor receipt was accepted")
		}
	})

	t.Run("stale persisted record", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture(t, "succeeded", "approved")
		mustWriteNodeConnectorPlacementExecutionGraphLifecycleExecutorArtifact(t, value.recordPath, value.executor.preimage)
		if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(value.root, value.expected); err == nil {
			t.Fatal("receipt whose persisted postimage is stale was accepted")
		}
	})

	t.Run("conflicting receipt", func(t *testing.T) {
		value := newNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture(t, "succeeded", "approved")
		path := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphLifecycleExecutorAuditReceiptName)
		raw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path)
		mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path, bytes.Replace(raw, []byte(`"projected_terminal_post_state": "succeeded"`), []byte(`"projected_terminal_post_state": "failed"`), 1))
		if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(value.root, value.expected); err == nil {
			t.Fatal("conflicting executor receipt was accepted")
		}
	})

	for _, test := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{name: "malformed", mutate: func([]byte) []byte { return []byte("{") }},
		{name: "unknown field", mutate: func(raw []byte) []byte {
			return bytes.Replace(raw, []byte("\n}"), []byte(",\n  \"unknown\": true\n}"), 1)
		}},
		{name: "trailing data", mutate: func(raw []byte) []byte { return append(append([]byte(nil), raw...), []byte("{}")...) }},
		{name: "oversized", mutate: func([]byte) []byte {
			return bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyArtifactMaxBytes+1)
		}},
		{name: "noncanonical", mutate: func(raw []byte) []byte {
			var value any
			_ = json.Unmarshal(raw, &value)
			compact, _ := json.Marshal(value)
			return compact
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := newNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture(t, "succeeded", "approved")
			mustDecideNodeConnectorPlacementExecutionGraphDependencyTransitionPolicy(t, value)
			path := filepath.Join(value.root, nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestName)
			raw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path)
			mustWriteRawNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path, test.mutate(raw))
			if _, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(value.root, value.expected); err == nil {
				t.Fatal("malformed, unknown-field, trailing, oversized, or noncanonical artifact was accepted")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyAtomicFailureRecoveryAndNoCallbacks(t *testing.T) {
	value := newNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture(t, "failed", "approved")
	policies := mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(t, value)
	recordBefore := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)
	original := nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyWriteRequestAtomic
	nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyWriteRequestAtomic = func(string, any) error { return errors.New("injected transition request failure") }
	t.Cleanup(func() { nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyWriteRequestAtomic = original })
	if _, _, err := policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphDependencyTransitionPolicy(t, value.fixture)); err == nil {
		t.Fatal("transition request publication failure was accepted")
	}
	assertNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyArtifactAbsent(t, value.root, nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestName)
	nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyWriteRequestAtomic = original
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyWithPolicies(t, mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(t, value), value.fixture)
	assertNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyNarrowAuthority(t, decision, request)
	if !bytes.Equal(recordBefore, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)) {
		t.Fatal("policy publication invoked a dependency mutation or other callback")
	}
}

func newNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture(t *testing.T, terminal, decision string) *nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture {
	t.Helper()
	executor := newNodeConnectorPlacementExecutionGraphLifecycleExecutorTestFixture(t, terminal, "approved", true)
	receipt := mustExecuteNodeConnectorPlacementExecutionGraphLifecycleExecutor(t, executor)
	targets := nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTargets()
	targetFingerprint, err := nodeExecutionFingerprintValue(targets)
	if err != nil {
		t.Fatal(err)
	}
	expected := NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyExpected{
		Executor: executor.expected, AuditReceiptFingerprint: receipt.ReceiptFingerprint, DependencyTargets: targets,
		DecisionAuthenticationID: "dependency-transition-authentication-001", DecisionAuthenticationDigest: "sha256:abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		TransitionRequestID: "dependency-transition-request-001",
	}
	fixture := NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture{
		Schema: NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixtureSchema, DecisionID: "dependency-transition-decision-001", ReplayIdentity: "dependency-transition-replay-001",
		AuthenticationID: "dependency-transition-authentication-001", AuthenticationDigest: "sha256:abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Decision: decision, AuditReceiptID: receipt.AuditReceiptID, AuditReceiptFingerprint: receipt.ReceiptFingerprint, GraphStoreID: receipt.GraphStoreID, GraphRecordID: receipt.GraphRecordID,
		GraphRunID: receipt.GraphRunID, TerminalGraphState: receipt.ProjectedTerminalPostState, PostimageFingerprint: receipt.PostimageFingerprint, PostimageVersion: receipt.PostimageVersion,
		DependencyTargets: cloneNodeConnectorPlacementExecutionGraphDependencyTransitionTargets(targets), DependencyTargetsFingerprint: targetFingerprint,
		Provenance: "fixture_only_forgepipe_local_graph_dependency_transition_policy_decision",
	}
	if decision == "approved" {
		fixture.TransitionRequestID = "dependency-transition-request-001"
		if terminal == "succeeded" {
			fixture.Route = "dependency_release_transition"
			fixture.Authority.DependencyReleaseTransitionAttempt = true
		} else {
			fixture.Route = "failure_propagation_transition"
			fixture.Authority.FailurePropagationTransitionAttempt = true
		}
	}
	return &nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture{root: executor.root, executor: executor, receipt: receipt, expected: expected, fixture: fixture, recordPath: executor.recordPath}
}

func nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTargets() []NodeConnectorPlacementExecutionGraphDependencyTransitionTarget {
	return []NodeConnectorPlacementExecutionGraphDependencyTransitionTarget{
		{DependencyID: "dependency-001", DependencyRecordID: "dependency-record-001", ExpectedPreimageFingerprint: "sha256:4444444444444444444444444444444444444444444444444444444444444444", ExpectedPreimageVersion: 7},
		{DependencyID: "dependency-002", DependencyRecordID: "dependency-record-002", ExpectedPreimageFingerprint: "sha256:5555555555555555555555555555555555555555555555555555555555555555", ExpectedPreimageVersion: 11},
	}
}

func mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(t *testing.T, value *nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture) *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicies {
	t.Helper()
	policies, err := OpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return policies
}

func mustDecideNodeConnectorPlacementExecutionGraphDependencyTransitionPolicy(t *testing.T, value *nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyTestFixture) (NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision, *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest) {
	t.Helper()
	return mustDecideNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyWithPolicies(t, mustOpenNodeConnectorPlacementExecutionGraphDependencyTransitionPolicies(t, value), value.fixture)
}

func mustDecideNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyWithPolicies(t *testing.T, policies *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicies, fixture NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) (NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision, *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest) {
	t.Helper()
	decision, request, err := policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphDependencyTransitionPolicy(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}

func mustMarshalNodeConnectorPlacementExecutionGraphDependencyTransitionPolicy(t *testing.T, value NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionFixture) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyArtifactsAbsent(t *testing.T, root string) {
	t.Helper()
	assertNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyArtifactAbsent(t, root, nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecisionName)
	assertNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyArtifactAbsent(t, root, nodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequestName)
}

func assertNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyArtifactAbsent(t *testing.T, root, name string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
		t.Fatalf("graph dependency-transition policy unexpectedly published %s", name)
	}
}

func assertNodeConnectorPlacementExecutionGraphDependencyTransitionPolicyNarrowAuthority(t *testing.T, decision NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision, request *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest) {
	t.Helper()
	raw, err := json.Marshal(struct {
		Decision NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyDecision `json:"decision"`
		Request  *NodeConnectorPlacementExecutionGraphDependencyTransitionPolicyRequest `json:"request"`
	}{decision, request})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"dependency_release":true`, `"failure_propagation":true`, `"next_task_scheduling":true`, `"new_graph_execution":true`, `"retry":true`, `"repair":true`, `"cancellation":true`, `"validation":true`, `"checkout_mutation":true`, `"git":true`, `"commit":true`, `"push":true`, `"publication":true`, `"broker":true`, `"provider":true`, `"forgepipe":true`, `"remote":true`, `"authorization_consumed":true`, `"transition_invoked":true`, `"callbacks_invoked":true`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("forbidden dependency, scheduling, callback, or external authority appeared: %s", forbidden)
		}
	}
}

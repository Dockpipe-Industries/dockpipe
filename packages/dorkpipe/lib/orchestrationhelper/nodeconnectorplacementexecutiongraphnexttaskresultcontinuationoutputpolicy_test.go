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

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture struct {
	root         string
	executor     *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture
	transition   NodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord
	receipt      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceipt
	expected     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected
	decision     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture
	decisionPath string
	requestPath  string
}

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestTemplate struct {
	once    sync.Once
	fixture nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture
	files   map[string][]byte
}

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestTemplates = map[string]*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestTemplate{
	"succeeded\x00approved\x00" + NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute: {},
	"succeeded\x00rejected\x00": {},
	"succeeded\x00approved\x00" + NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute: {},
	"failed\x00approved\x00" + NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute:        {},
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExactRoutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, result, route, output string
		authority                   NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority
	}{
		{"continuation handoff", "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffOutput, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority{GraphContinuationHandoffAttempt: true}},
		{"successful terminal result", "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority{SuccessfulTerminalGraphResultMaterializationAttempt: true}},
		{"failed terminal result", "failed", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationMaterialization, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority{FailedTerminalGraphResultMaterializationAttempt: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture(t, test.result, "approved", test.route)
			before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
			decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t, value)
			if decision.Decision != "approved" || decision.Route != test.route || decision.OutputType != test.output || request == nil || request.Route != test.route || request.OutputType != test.output || request.Authority != test.authority || decision.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority{}) {
				t.Fatal("approved decision did not publish the exact route-compatible output request")
			}
			if !request.OneTimeRequest || request.AuthorizationConsumed || request.ContinuationHandoffInvoked || request.TerminalGraphResultMaterializationInvoked || request.CallbacksInvoked || request.ExternalActionsInvoked || !request.FixtureOwned {
				t.Fatal("output policy performed an output, callback, external action, or consumed future authority")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExactBindings(t, value, decision, request)
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyOnlyOutputsChanged(t, value, before, true)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRejectedProducesNoRequest(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture(t, "succeeded", "rejected", "")
	before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t, value)
	if decision.Decision != "rejected" || decision.Route != "" || decision.OutputType != "" || decision.OutputRequestID != "" || request != nil {
		t.Fatal("rejected decision emitted or named future output authority")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyOnlyOutputsChanged(t, value, before, false)

	approved := value.decision
	approved.Decision = "approved"
	approved.Route = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute
	approved.OutputType = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffOutput
	approved.OutputRequestID = value.expected.OutputRequestID
	approved.Authority = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyAuthority{GraphContinuationHandoffAttempt: true}
	_, conflictingRequest, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(value.expected, value.transition, value.receipt, approved)
	if err != nil {
		t.Fatal(err)
	}
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.requestPath, *conflictingRequest)
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(value.root, value.expected); err == nil {
		t.Fatal("rejected decision with an orphaned request was accepted")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyCompatibilityAuthenticationAndNoInference(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture(t, "succeeded", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture)
	}{
		{"wrong route", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.Route = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute
		}},
		{"wrong output", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.OutputType = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization
		}},
		{"wrong post-state", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.Binding.PostState = "succeeded"
		}},
		{"wrong effect", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.Binding.RouteSpecificEffect = "result_presence"
		}},
		{"wrong transition", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.Binding.TransitionRecordFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')
		}},
		{"wrong receipt", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.Binding.TransitionExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		}},
		{"wrong graph", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.Binding.ExecutorBinding.GraphRunID = "graph-run-wrong-001"
		}},
		{"wrong terminal task", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.Binding.ExecutorBinding.TerminalTaskID = "terminal-task-wrong-001"
		}},
		{"wrong selected task", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.Binding.ExecutorBinding.SelectedTaskID = "selected-task-wrong-001"
		}},
		{"wrong candidate set", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.Binding.ExecutorBinding.CandidatesFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('3')
		}},
		{"wrong accepted result", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.Binding.ExecutorBinding.AcceptedResultFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('4')
		}},
		{"wrong outcome", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.Binding.ExecutorBinding.TaskOutcome = "failed"
		}},
		{"wrong prior authentication", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.Binding.ExecutorBinding.PolicyAuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('5')
		}},
		{"wrong authentication identity", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.AuthenticationID = "output-authentication-wrong-001"
		}},
		{"wrong authentication digest", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.AuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('6')
		}},
		{"reused prior authentication", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.AuthenticationID = v.Binding.ExecutorBinding.PolicyAuthenticationID
			v.AuthenticationDigest = v.Binding.ExecutorBinding.PolicyAuthenticationDigest
		}},
		{"consumed decision", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.DecisionConsumed = true
		}},
		{"not one time", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.OneTimeDecision = false
		}},
		{"not deterministic", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.Deterministic = false
		}},
		{"provider owned", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.Provenance = "provider"
		}},
		{"authority escalation", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.Authority.Callback = true
		}},
		{"colliding decision", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.DecisionID = v.Binding.TransitionExecutorReceiptID
		}},
		{"colliding replay", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) {
			v.ReplayIdentity = v.Binding.ExecutorBinding.AcceptedResultID
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture(value.decision)
			test.mutate(&changed)
			if _, _, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(value.expected, value.transition, value.receipt, changed); err == nil {
				t.Fatal("incompatible, unauthenticated, consumed, or authority-escalated decision was accepted")
			}
		})
	}

	for _, source := range []string{"result", "outcome", "graph", "transition", "post_state", "scheduling", "availability", "connection", "lease", "broker", "provider", "forgepipe", "ranking", "cost", "risk", "receipt"} {
		t.Run("inference "+source, func(t *testing.T) {
			changed := value.decision
			changed.ApprovalInferred, changed.RouteInferred, changed.OutputTypeInferred, changed.AuthorityInferred, changed.InferenceSource = true, true, true, true, source
			if _, _, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(value.expected, value.transition, value.receipt, changed); err == nil {
				t.Fatal("adjacent evidence inferred approval, route, output type, or authority")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRevalidatesCompleteChain(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture(t, "succeeded", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	raw, _ := json.Marshal(value.expected)
	tests := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected)
	}{
		{"transition receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) {
			e.ExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')
		}},
		{"transition record", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) {
			e.TransitionRecordFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		}},
		{"continuation policy decision", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) {
			e.Executor.PolicyDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('3')
		}},
		{"continuation policy request", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) {
			e.Executor.PolicyRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('4')
		}},
		{"reconciliation receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) {
			e.Executor.Policy.ReconciliationReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('5')
		}},
		{"accepted result", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) {
			e.Executor.Policy.AcceptedResultFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('6')
		}},
		{"result observation", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) {
			e.Executor.Policy.Reconciliation.AuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('7')
		}},
		{"attempt receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) {
			e.Executor.Policy.Reconciliation.ExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('8')
		}},
		{"launch authorization", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) {
			e.Executor.Policy.Reconciliation.Executor.AuthorizationRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('9')
		}},
		{"scheduling receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) {
			e.Executor.Policy.Reconciliation.Executor.Policy.SchedulingReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('a')
		}},
		{"scheduling policy", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) {
			e.Executor.Policy.Reconciliation.Executor.Policy.Executor.PolicyDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('b')
		}},
		{"dependency transition", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) {
			e.Executor.Policy.Reconciliation.Executor.Policy.Executor.Policy.TransitionReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('c')
		}},
		{"lifecycle receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) {
			e.Executor.Policy.Reconciliation.Executor.Policy.Executor.Policy.Executor.Policy.AuditReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('d')
		}},
		{"graph projection", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) {
			e.Executor.Policy.Reconciliation.Executor.Policy.Executor.Policy.Executor.Policy.Executor.Policy.ProjectionDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('e')
		}},
		{"graph outcome", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected) {
			e.Executor.Policy.Reconciliation.Executor.Policy.Executor.Policy.Executor.Policy.Executor.Policy.Projection.Finalization.Outcomes[0].ArtifactFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('f')
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var changed NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected
			_ = json.Unmarshal(raw, &changed)
			test.mutate(&changed)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(value.root, changed); err == nil {
				t.Fatal("changed immutable predecessor chain was accepted")
			}
		})
	}

	opened := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(t, value)
	changed := value.transition
	changed.PostState = "succeeded"
	changed.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecordFingerprint(changed)
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.executor.transitionPath, changed)
	if _, _, err := opened.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t, value.decision)); err == nil {
		t.Fatal("decision-time revalidation accepted a changed transition predecessor")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyReplayRestartConcurrencyAndRecovery(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture(t, "succeeded", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	raw := mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t, value.decision)
	firstDecision, firstRequest, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(t, value).Decide(raw)
	if err != nil {
		t.Fatal(err)
	}
	secondDecision, secondRequest, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(t, value).Decide(raw)
	if err != nil || !reflect.DeepEqual(firstDecision, secondDecision) || !reflect.DeepEqual(firstRequest, secondRequest) {
		t.Fatal("exact replay, restart, or identical existing artifacts changed output")
	}
	const callers = 6
	var group sync.WaitGroup
	errs := make(chan error, callers)
	for index := 0; index < callers; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			policies, openErr := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(value.root, value.expected)
			if openErr == nil {
				decision, request, decideErr := policies.Decide(raw)
				if decideErr == nil && (!reflect.DeepEqual(decision, firstDecision) || !reflect.DeepEqual(request, firstRequest)) {
					decideErr = errors.New("identical concurrency changed output")
				}
				openErr = decideErr
			}
			errs <- openErr
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	conflict := value.decision
	conflict.DecisionID, conflict.ReplayIdentity = "output-decision-002", "output-replay-002"
	if _, _, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(t, value).Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t, conflict)); err == nil {
		t.Fatal("conflicting decision was accepted")
	}

	recovery := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture(t, "failed", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute)
	policies := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(t, recovery)
	decisionWriter := policies.writeDecision
	var decisionWrites atomic.Int32
	policies.writeDecision = func(path string, payload any) error { decisionWrites.Add(1); return decisionWriter(path, payload) }
	policies.writeRequest = func(string, any) error { return errors.New("injected request failure") }
	if _, _, err := policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t, recovery.decision)); err == nil || decisionWrites.Load() != 1 {
		t.Fatal("decision-before-request failure did not preserve exactly one decision")
	}
	if _, err := os.Lstat(recovery.requestPath); !os.IsNotExist(err) {
		t.Fatal("request failure left partial output")
	}
	restarted := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(t, recovery)
	restarted.writeDecision = func(path string, payload any) error { decisionWrites.Add(1); return decisionWriter(path, payload) }
	decision, request, err := restarted.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t, recovery.decision))
	if err != nil || request == nil || decisionWrites.Load() != 1 || decision.DecisionFingerprint != request.DecisionFingerprint {
		t.Fatal("restart did not recover only the exact missing request")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRejectsMalformedUnsafeTamperedAndEscalatedArtifacts(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture(t, "succeeded", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	policies := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(t, value)
	for _, test := range []struct {
		name string
		raw  []byte
	}{
		{"empty", nil},
		{"malformed", []byte("{")},
		{"trailing", append(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t, value.decision), []byte("{}")...)},
		{"unknown field", append(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t, value.decision)[:len(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t, value.decision))-1], []byte(`,"unknown":true}`)...)},
		{"noncanonical", mustMarshalIndentedNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t, value.decision)},
		{"oversized", bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionMaxBytes+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := policies.Decide(test.raw); err == nil {
				t.Fatal("malformed, noncanonical, unknown-field, trailing, or oversized fixture was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyArtifactsAbsent(t, value)
		})
	}

	t.Run("symlinked decision", func(t *testing.T) {
		target := value.decisionPath + ".target"
		defer os.Remove(value.decisionPath)
		defer os.Remove(target)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, target, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{})
		if err := os.Symlink(target, value.decisionPath); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(value.root, value.expected); err == nil {
			t.Fatal("symlinked policy artifact was accepted")
		}
	})

	for _, test := range []struct {
		name string
		raw  []byte
	}{
		{"partial", []byte("{")},
		{"unknown", []byte("{\"unknown\":true}\n")},
		{"trailing", []byte("{}\n{}\n")},
		{"noncanonical", []byte("{}")},
		{"oversized", bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyArtifactMaxBytes+1)},
	} {
		t.Run("existing "+test.name, func(t *testing.T) {
			defer os.Remove(value.decisionPath)
			if err := os.WriteFile(value.decisionPath, test.raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(value.root, value.expected); err == nil {
				t.Fatal("partial, unknown, trailing, noncanonical, or oversized existing artifact was accepted")
			}
		})
	}

	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyCanonicalArtifact(value.root, filepath.Join(value.root, "..", "unsafe-output.json"), &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision{}, true); err == nil {
		t.Fatal("unsafe output path was accepted")
	}

	decision, request, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(value.expected, value.transition, value.receipt, value.decision)
	if err != nil || request == nil {
		t.Fatal(err)
	}
	request.AuthorizationConsumed = true
	request.Authority.Callback = true
	request.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequestFingerprint(*request)
	if validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest(*request, value.expected, value.transition, value.receipt, decision) == nil {
		t.Fatal("consumed or authority-escalated output request was accepted")
	}

	orphan := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture(t, value)
	_, orphanRequest, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(orphan.expected, orphan.transition, orphan.receipt, orphan.decision)
	if err != nil {
		t.Fatal(err)
	}
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, orphan.requestPath, *orphanRequest)
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(orphan.root, orphan.expected); err == nil {
		t.Fatal("request without its exact decision was accepted")
	}
}

func newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture(t *testing.T, terminalResult, decision, route string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture {
	t.Helper()
	template, ok := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestTemplates[terminalResult+"\x00"+decision+"\x00"+route]
	if !ok {
		t.Fatalf("unsupported output-policy test route %q/%q/%q", terminalResult, decision, route)
	}
	template.once.Do(func() {
		value := buildNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture(t, terminalResult, decision, route)
		template.fixture = *value
		template.fixture.executor = nil
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
	value := template.fixture
	value.root = root
	value.executor = &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture{
		root:           root,
		transitionPath: filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecordName),
		receiptPath:    filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorReceiptName),
	}
	value.decisionPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionName)
	value.requestPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequestName)
	return &value
}

func buildNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture(t *testing.T, terminalResult, decision, route string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture {
	t.Helper()
	executorRoute := route
	if executorRoute == "" {
		executorRoute = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute
		if terminalResult == "failed" {
			executorRoute = NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute
		}
	}
	executor := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutorTestFixture(t, terminalResult, executorRoute)
	receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationExecutor(t, executor)
	transition := mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationTransitionRecord(t, executor)
	expected := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExpected{
		Executor: executor.expected, ExecutorReceiptFingerprint: receipt.ReceiptFingerprint, TransitionRecordFingerprint: transition.RecordFingerprint,
		DecisionAuthenticationID: "output-authentication-001", DecisionAuthenticationDigest: testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('d'), OutputRequestID: "output-request-001",
	}
	outputType, authority, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRouteAuthority(transition.Route, transition.PostState, transition.Effect)
	fixture := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture{
		Schema:     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixtureSchema,
		DecisionID: "output-decision-001", ReplayIdentity: "output-replay-001", AuthenticationID: expected.DecisionAuthenticationID, AuthenticationDigest: expected.DecisionAuthenticationDigest,
		Decision: decision, Binding: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyBinding(transition, receipt), Deterministic: true, OneTimeDecision: true,
		Provenance: "fixture_only_post_transition_graph_output_policy_decision",
	}
	if decision == "approved" {
		fixture.Route, fixture.OutputType, fixture.OutputRequestID, fixture.Authority = route, outputType, expected.OutputRequestID, authority
	}
	return &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture{
		root: executor.root, executor: executor, transition: transition, receipt: receipt, expected: expected, decision: fixture,
		decisionPath: filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionName),
		requestPath:  filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequestName),
	}
}

func mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture) *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies {
	t.Helper()
	policies, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return policies
}

func mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest) {
	t.Helper()
	decision, request, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicies(t, value).Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t, value.decision))
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}

func mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t *testing.T, value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustMarshalIndentedNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicy(t *testing.T, value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) []byte {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionFixture
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture {
	t.Helper()
	root := t.TempDir()
	for relative, raw := range mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root) {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture{
		root: root, transition: value.transition, receipt: value.receipt, expected: value.expected, decision: value.decision,
		decisionPath: filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionName),
		requestPath:  filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequestName),
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyExactBindings(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture, decision NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecision, request *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequest) {
	t.Helper()
	want := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyBinding(value.transition, value.receipt)
	if !reflect.DeepEqual(decision.Binding, want) || request == nil || !reflect.DeepEqual(request.Binding, want) || request.DecisionID != decision.DecisionID || request.DecisionReplayIdentity != decision.ReplayIdentity || request.DecisionFingerprint != decision.DecisionFingerprint || request.AuthenticationID != decision.AuthenticationID || request.AuthenticationDigest != decision.AuthenticationDigest {
		t.Fatal("decision or request omitted the executor receipt, transition, route, post-state, effect, graph, task, candidate, result, outcome, or prior-policy authentication binding")
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyOnlyOutputsChanged(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture, before map[string][]byte, requestExpected bool) {
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
	want := []string{nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyDecisionName}
	if requestExpected {
		want = append(want, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyRequestName)
	}
	sort.Strings(want)
	if !reflect.DeepEqual(changed, want) {
		t.Fatalf("output policy changed a predecessor or adjacent artifact: got %v want %v", changed, want)
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyArtifactsAbsent(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputPolicyTestFixture) {
	t.Helper()
	for _, path := range []string{value.decisionPath, value.requestPath} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("failed output policy unexpectedly published %s", path)
		}
	}
}

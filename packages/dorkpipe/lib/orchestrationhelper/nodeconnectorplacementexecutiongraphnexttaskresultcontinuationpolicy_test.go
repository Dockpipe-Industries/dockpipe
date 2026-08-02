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

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture struct {
	root           string
	reconciliation *nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture
	accepted       NodeConnectorPlacementExecutionGraphNextTaskAcceptedResult
	receipt        NodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceipt
	expected       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected
	decision       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture
	decisionPath   string
	requestPath    string
}

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestTemplate struct {
	once             sync.Once
	fixture          nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture
	reconciliation   nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture
	selectedRelative string
	files            map[string][]byte
}

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestTemplates = map[string]*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestTemplate{
	"succeeded\x00approved\x00" + NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute: {},
	"succeeded\x00rejected\x00": {},
	"succeeded\x00approved\x00" + NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute: {},
	"failed\x00approved\x00" + NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute:        {},
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyApprovedRoutesAreExact(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, result, route string
		want                NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority
	}{
		{"passed continuation", "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority{GraphContinuationExecutorAttempt: true}},
		{"passed successful finalization", "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority{SuccessfulGraphFinalizationExecutorAttempt: true}},
		{"failed failed finalization", "failed", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority{FailedGraphFinalizationExecutorAttempt: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture(t, test.result, "approved", test.route)
			before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
			decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(t, value)
			if decision.Decision != "approved" || decision.Route != test.route || request == nil || request.Route != test.route || request.Authority != test.want || decision.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority{}) {
				t.Fatal("approved route did not preserve its exact narrow decision and request authority")
			}
			if request.AuthorizationConsumed || request.GraphContinuationInvoked || request.GraphFinalizationInvoked || request.CallbacksInvoked || request.ExternalActionsInvoked || !request.OneTimeRequest || !request.FixtureOwned {
				t.Fatal("policy request performed or recorded a forbidden action")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExactBindings(t, value, decision, request)
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyOnlyOutputsChanged(t, value, before, true)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRejectedEmitsNoRequest(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture(t, "succeeded", "rejected", "")
	before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(t, value)
	if decision.Decision != "rejected" || decision.Route != "" || decision.ContinuationRequestID != "" || request != nil {
		t.Fatal("rejected decision emitted or named a continuation/finalization request")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyOnlyOutputsChanged(t, value, before, false)
	approved := value.decision
	approved.Decision = "approved"
	approved.Route = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute
	approved.ContinuationRequestID = value.expected.ContinuationRequestID
	approved.Authority = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority{GraphContinuationExecutorAttempt: true}
	_, conflictingRequest, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(value.expected, value.accepted, value.receipt, approved)
	if err != nil {
		t.Fatal(err)
	}
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.requestPath, *conflictingRequest)
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(value.root, value.expected); err == nil {
		t.Fatal("rejected decision with a request was accepted")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRejectsOutcomeRouteMismatchAndInference(t *testing.T) {
	t.Parallel()
	passed := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture(t, "succeeded", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	failed := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture(t, "failed", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute)
	for _, test := range []struct{ result, route string }{
		{"failed", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute},
		{"failed", NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute},
		{"succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute},
		{"succeeded", ""},
		{"succeeded", "unknown_route"},
		{"failed", "graph_continuation|failed_graph_finalization"},
	} {
		value := passed
		if test.result == "failed" {
			value = failed
		}
		fixture := value.decision
		fixture.Route = test.route
		fixture.Authority, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRouteAuthority(value.receipt.TaskOutcome, test.route)
		if _, _, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(value.expected, value.accepted, value.receipt, fixture); err == nil {
			t.Fatalf("accepted outcome/route mismatch %s/%s", test.result, test.route)
		}
	}

	for _, source := range []string{"result_observation", "accepted_result", "task_outcome", "terminal_result", "attempt", "executor_receipt", "authorization_consumption", "scheduled_state", "dependency_release", "graph_state", "lifecycle", "transition", "projection", "finalization", "process_exit", "candidate_presence", "availability", "load", "risk", "cost", "score", "ordering", "ranking", "recommendation", "matching", "connection", "connector", "lease", "broker", "provider", "forgepipe", "machine", "capability", "validation", "receipt"} {
		t.Run("inference "+source, func(t *testing.T) {
			fixture := passed.decision
			fixture.ApprovalInferred, fixture.RouteInferred, fixture.InferenceSource = true, true, source
			if _, _, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(passed.expected, passed.accepted, passed.receipt, fixture); err == nil {
				t.Fatal("inferred approval or route was accepted")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRejectsDecisionAndBindingMatrix(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture(t, "succeeded", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	tests := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture)
	}{
		{"empty decision", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Decision = ""
		}},
		{"ambiguous decision", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Decision = "approved|rejected"
		}},
		{"wrong authentication identity", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.AuthenticationID = "continuation-authentication-wrong-001"
		}},
		{"wrong authentication digest", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.AuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('f')
		}},
		{"not deterministic", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Deterministic = false
		}},
		{"not one time", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.OneTimeDecision = false
		}},
		{"consumed", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.DecisionConsumed = true
		}},
		{"non fixture owned", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Provenance = "provider"
		}},
		{"wrong reconciliation", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Binding.ReconciliationReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')
		}},
		{"wrong accepted result", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Binding.AcceptedResultFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		}},
		{"wrong observation", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Binding.ObservationFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('3')
		}},
		{"wrong executor receipt", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Binding.ExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('4')
		}},
		{"wrong attempt", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Binding.AttemptRecordFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('5')
		}},
		{"wrong graph", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Binding.GraphRunID = "graph-run-wrong-001"
		}},
		{"wrong terminal task", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Binding.TerminalTaskID = "terminal-task-wrong-001"
		}},
		{"wrong selected task", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Binding.SelectedTaskID = "selected-task-wrong-001"
		}},
		{"wrong candidates", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Binding.CandidatesFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('6')
		}},
		{"wrong released postimage", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Binding.SelectedReleasedDependencyPostimage.ReleasedPostimageVersion++
		}},
		{"wrong scheduled record", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Binding.ScheduledRecordFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('7')
		}},
		{"wrong scheduled version", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Binding.ScheduledRecordVersion++
		}},
		{"wrong terminal result", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Binding.TerminalResult = "failed"
		}},
		{"wrong task outcome", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Binding.TaskOutcome = "failed"
		}},
		{"authority escalated", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.Authority.GraphContinuation = true
		}},
		{"colliding decision", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.DecisionID = v.Binding.AcceptedResultID
		}},
		{"colliding replay", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) {
			v.ReplayIdentity = v.Binding.AttemptID
		}},
	}
	base, _ := json.Marshal(value.decision)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var changed NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture
			_ = json.Unmarshal(base, &changed)
			test.mutate(&changed)
			if _, _, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(value.expected, value.accepted, value.receipt, changed); err == nil {
				t.Fatal("invalid decision or exact binding was accepted")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRevalidatesCompletePredecessorChain(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture(t, "succeeded", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	raw, _ := json.Marshal(value.expected)
	tests := []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected)
	}{
		{"reconciliation receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected) {
			e.ReconciliationReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')
		}},
		{"accepted result", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected) {
			e.AcceptedResultFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		}},
		{"result observation", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected) {
			e.Reconciliation.AuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('3')
		}},
		{"executor receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected) {
			e.Reconciliation.ExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('4')
		}},
		{"launch authorization", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected) {
			e.Reconciliation.Executor.AuthorizationRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('5')
		}},
		{"scheduling receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected) {
			e.Reconciliation.Executor.Policy.SchedulingReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('6')
		}},
		{"scheduling policy", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected) {
			e.Reconciliation.Executor.Policy.Executor.PolicyDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('7')
		}},
		{"dependency transition", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected) {
			e.Reconciliation.Executor.Policy.Executor.Policy.TransitionReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('8')
		}},
		{"lifecycle execution", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected) {
			e.Reconciliation.Executor.Policy.Executor.Policy.Executor.Policy.AuditReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('9')
		}},
		{"graph projection", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected) {
			e.Reconciliation.Executor.Policy.Executor.Policy.Executor.Policy.Executor.Policy.ProjectionDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('a')
		}},
		{"task outcome", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected) {
			e.Reconciliation.Executor.Policy.Executor.Policy.Executor.Policy.Executor.Policy.Projection.Finalization.Outcomes[0].ArtifactFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('b')
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var expected NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected
			_ = json.Unmarshal(raw, &expected)
			test.mutate(&expected)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(value.root, expected); err == nil {
				t.Fatal("changed immutable predecessor binding was accepted")
			}
		})
	}

	opened := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(t, value)
	changed := value.accepted
	changed.TerminalResult = "failed"
	changed.AcceptedResultFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskAcceptedResultFingerprint(changed)
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.reconciliation.acceptedPath, changed)
	if _, _, err := opened.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(t, value.decision)); err == nil {
		t.Fatal("policy did not revalidate changed durable predecessor evidence at decision time")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyReplayRestartConcurrencyAndConflicts(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture(t, "succeeded", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	raw := mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(t, value.decision)
	firstDecision, firstRequest, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(t, value).Decide(raw)
	if err != nil {
		t.Fatal(err)
	}
	secondDecision, secondRequest, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(t, value).Decide(raw)
	if err != nil || !reflect.DeepEqual(firstDecision, secondDecision) || !reflect.DeepEqual(firstRequest, secondRequest) {
		t.Fatal("exact replay or restart changed canonical policy artifacts")
	}
	const callers = 8
	var wait sync.WaitGroup
	errs := make(chan error, callers)
	for index := 0; index < callers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			policies, openErr := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(value.root, value.expected)
			if openErr == nil {
				decision, request, decideErr := policies.Decide(raw)
				if decideErr == nil && (!reflect.DeepEqual(decision, firstDecision) || !reflect.DeepEqual(request, firstRequest)) {
					decideErr = errors.New("concurrent identical decision changed output")
				}
				openErr = decideErr
			}
			errs <- openErr
		}()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Run("concurrent conflicting routes", func(t *testing.T) {
		conflict := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture(t, "succeeded", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
		first := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(t, conflict)
		second := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(t, conflict)
		other := conflict.decision
		other.DecisionID, other.ReplayIdentity = "continuation-decision-002", "continuation-replay-002"
		other.Route = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute
		other.Authority = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority{SuccessfulGraphFinalizationExecutorAttempt: true}
		var group sync.WaitGroup
		results := make(chan error, 2)
		group.Add(2)
		go func() {
			defer group.Done()
			_, _, err := first.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(t, conflict.decision))
			results <- err
		}()
		go func() {
			defer group.Done()
			_, _, err := second.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(t, other))
			results <- err
		}()
		group.Wait()
		close(results)
		successes, failures := 0, 0
		for err := range results {
			if err == nil {
				successes++
			} else {
				failures++
			}
		}
		if successes != 1 || failures != 1 {
			t.Fatalf("conflicting routes produced successes=%d failures=%d", successes, failures)
		}
		_, inputs, err := normalizeNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected(conflict.root, conflict.expected)
		if err != nil {
			t.Fatal(err)
		}
		acceptedDecision, exists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision(conflict.root, conflict.expected, inputs)
		if err != nil || !exists {
			t.Fatal("accepted conflicting decision was not durable")
		}
		acceptedRequest, exists, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest(conflict.root, conflict.expected, inputs, acceptedDecision, true)
		if err != nil || !exists {
			t.Fatal("accepted conflicting request was not durable")
		}
		changed := acceptedRequest
		if changed.Route == NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute {
			changed.Route = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute
			changed.Authority = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority{SuccessfulGraphFinalizationExecutorAttempt: true}
		} else {
			changed.Route = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute
			changed.Authority = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyAuthority{GraphContinuationExecutorAttempt: true}
		}
		changed.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequestFingerprint(changed)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, conflict.requestPath, changed)
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(conflict.root, conflict.expected); err == nil {
			t.Fatal("conflicting pre-existing request was accepted")
		}
	})

	mustRemoveNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorPath(t, value.decisionPath)
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(value.root, value.expected); err == nil {
		t.Fatal("request without its exact decision was accepted")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionBeforeRequestRecovery(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture(t, "failed", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute)
	policies := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(t, value)
	decisionWriter := policies.writeDecision
	var decisionWrites atomic.Int32
	policies.writeDecision = func(path string, payload any) error { decisionWrites.Add(1); return decisionWriter(path, payload) }
	policies.writeRequest = func(string, any) error { return errors.New("injected request publication failure") }
	if _, _, err := policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(t, value.decision)); err == nil || decisionWrites.Load() != 1 {
		t.Fatal("request failure did not preserve exactly one durable decision")
	}
	if _, err := os.Lstat(value.requestPath); !os.IsNotExist(err) {
		t.Fatal("request publication failure left a partial request")
	}
	restarted := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(t, value)
	restarted.writeDecision = func(path string, payload any) error { decisionWrites.Add(1); return decisionWriter(path, payload) }
	decision, request, err := restarted.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(t, value.decision))
	if err != nil || request == nil || decisionWrites.Load() != 1 || decision.DecisionFingerprint != request.DecisionFingerprint {
		t.Fatal("restart did not recover only the exact request")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRejectsMalformedUnsafeAndPartialArtifacts(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture(t, "succeeded", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	policies := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(t, value)
	for _, test := range []struct {
		name string
		raw  func(*testing.T, *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture) []byte
	}{
		{"empty", func(*testing.T, *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture) []byte {
			return nil
		}},
		{"malformed", func(*testing.T, *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture) []byte {
			return []byte("{")
		}},
		{"trailing", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture) []byte {
			return append(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(t, v.decision), []byte("{}")...)
		}},
		{"unknown field", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture) []byte {
			raw := mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(t, v.decision)
			return append(raw[:len(raw)-1], []byte(`,"unknown":true}`)...)
		}},
		{"noncanonical", func(t *testing.T, v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture) []byte {
			raw, _ := json.MarshalIndent(v.decision, "", "  ")
			return raw
		}},
		{"oversized", func(*testing.T, *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture) []byte {
			return bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionMaxBytes+1)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := policies.Decide(test.raw(t, value)); err == nil {
				t.Fatal("malformed, noncanonical, trailing, unknown-field, or oversized fixture was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyArtifactsAbsent(t, value)
		})
	}

	t.Run("symlinked decision", func(t *testing.T) {
		target := value.decisionPath + ".target"
		defer os.Remove(value.decisionPath)
		defer os.Remove(target)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, target, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision{})
		if err := os.Symlink(target, value.decisionPath); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(value.root, value.expected); err == nil {
			t.Fatal("symlinked decision artifact was accepted")
		}
	})

	t.Run("partial decision artifact", func(t *testing.T) {
		defer os.Remove(value.decisionPath)
		if err := os.WriteFile(value.decisionPath, []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(value.root, value.expected); err == nil {
			t.Fatal("partial decision artifact was accepted")
		}
	})
}

func newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture(t *testing.T, terminalResult, decision, route string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture {
	t.Helper()
	template, ok := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestTemplates[terminalResult+"\x00"+decision+"\x00"+route]
	if !ok {
		t.Fatalf("unsupported continuation-policy test route %q/%q/%q", terminalResult, decision, route)
	}
	template.once.Do(func() {
		value := buildNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture(t, terminalResult, decision, route)
		selectedRelative, err := filepath.Rel(value.root, value.reconciliation.executor.policy.executor.selectedPath)
		if err != nil {
			t.Fatal(err)
		}
		template.selectedRelative = filepath.ToSlash(selectedRelative)
		template.reconciliation = *value.reconciliation
		template.reconciliation.executor = nil
		template.fixture = *value
		template.fixture.reconciliation = &template.reconciliation
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
	reconciliation := template.reconciliation
	reconciliation.root = root
	reconciliation.observationPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultObservationName)
	reconciliation.acceptedPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskAcceptedResultName)
	reconciliation.receiptPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultReconciliationReceiptName)
	scheduling := &nodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorTestFixture{root: root, selectedPath: filepath.Join(root, filepath.FromSlash(template.selectedRelative))}
	authorization := &nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationPolicyTestFixture{root: root, executor: scheduling}
	reconciliation.executor = &nodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionExecutorTestFixture{root: root, policy: authorization}
	value := template.fixture
	value.root, value.reconciliation = root, &reconciliation
	value.decisionPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionName)
	value.requestPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequestName)
	return &value
}

func buildNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture(t *testing.T, terminalResult, decision, route string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture {
	t.Helper()
	reconciliation := newNodeConnectorPlacementExecutionGraphNextTaskResultReconciliationTestFixture(t, terminalResult)
	accepted, receipt := mustReconcileNodeConnectorPlacementExecutionGraphNextTaskResult(t, reconciliation)
	expected := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExpected{
		Reconciliation: reconciliation.expected, ReconciliationReceiptFingerprint: receipt.ReceiptFingerprint,
		AcceptedResultFingerprint: accepted.AcceptedResultFingerprint,
		DecisionAuthenticationID:  "continuation-authentication-001", DecisionAuthenticationDigest: testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('e'),
		ContinuationRequestID: "continuation-request-001",
	}
	binding := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyBinding(accepted, receipt)
	fixture := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture{
		Schema:     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixtureSchema,
		DecisionID: "continuation-decision-001", ReplayIdentity: "continuation-replay-001",
		AuthenticationID: expected.DecisionAuthenticationID, AuthenticationDigest: expected.DecisionAuthenticationDigest,
		Decision: decision, Route: route, Binding: binding, Deterministic: true, OneTimeDecision: true,
		Provenance: "fixture_only_post_reconciliation_graph_continuation_finalization_policy_decision",
	}
	if decision == "approved" {
		fixture.ContinuationRequestID = expected.ContinuationRequestID
		fixture.Authority, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRouteAuthority(receipt.TaskOutcome, route)
	}
	return &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture{
		root: reconciliation.root, reconciliation: reconciliation, accepted: accepted, receipt: receipt, expected: expected, decision: fixture,
		decisionPath: filepath.Join(reconciliation.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionName),
		requestPath:  filepath.Join(reconciliation.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequestName),
	}
}

func mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture) *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies {
	t.Helper()
	policies, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return policies
}

func mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
	t.Helper()
	decision, request, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicies(t, value).Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(t, value.decision))
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}

func mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicy(t *testing.T, value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionFixture) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyExactBindings(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture, decision NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecision, request *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequest) {
	t.Helper()
	want := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyBinding(value.accepted, value.receipt)
	if !reflect.DeepEqual(decision.Binding, want) || request == nil || !reflect.DeepEqual(request.Binding, want) || request.DecisionID != decision.DecisionID || request.DecisionReplayIdentity != decision.ReplayIdentity || request.DecisionFingerprint != decision.DecisionFingerprint || request.AuthenticationID != decision.AuthenticationID || request.AuthenticationDigest != decision.AuthenticationDigest {
		t.Fatal("decision or request omitted an exact reconciliation, result, observation, attempt, authorization, scheduling, graph, candidate, record, result, outcome, replay, or authentication binding")
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyOnlyOutputsChanged(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture, before map[string][]byte, requestExpected bool) {
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
	want := []string{nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyDecisionName}
	if requestExpected {
		want = append(want, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyRequestName)
	}
	sort.Strings(want)
	if !reflect.DeepEqual(changed, want) {
		t.Fatalf("policy changed immutable predecessor or persisted records: got %v want %v", changed, want)
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyArtifactsAbsent(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationPolicyTestFixture) {
	t.Helper()
	for _, path := range []string{value.decisionPath, value.requestPath} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("failed policy unexpectedly published %s", path)
		}
	}
}

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

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture struct {
	root         string
	executor     *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture
	output       NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord
	receipt      NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceipt
	expected     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected
	decision     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture
	decisionPath string
	requestPath  string
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExactRoutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, result, route, output, delivery string
		authority                             NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority
	}{
		{"continuation handoff", "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffOutput, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffDelivery, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{ContinuationHandoffDeliveryAttempt: true}},
		{"successful terminal result", "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization, NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationDelivery, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{SuccessfulTerminalGraphResultDeliveryAttempt: true}},
		{"failed terminal result", "failed", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute, NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationMaterialization, NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationDelivery, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{FailedTerminalGraphResultDeliveryAttempt: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t, test.result, "approved", test.route)
			before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
			decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(t, value)
			if decision.Decision != "approved" || decision.Route != test.route || decision.OutputType != test.output || decision.DeliveryType != test.delivery || request == nil || request.Route != test.route || request.OutputType != test.output || request.DeliveryType != test.delivery || request.Authority != test.authority || decision.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{}) {
				t.Fatal("approved decision did not publish the exact route-compatible delivery request")
			}
			if !request.OneTimeRequest || request.AuthorizationConsumed || request.DeliveryPerformed || request.ConsumerInvoked || request.AcknowledgementReceived || request.CallbackInvoked || request.LifecycleActionTriggered || request.PublicationPerformed || request.ExternalActionPerformed || !request.FixtureOwned {
				t.Fatal("delivery policy performed or implied delivery, acknowledgement, lifecycle, publication, or external action")
			}
			if decision.AuthenticationID == value.executor.request.AuthenticationID || decision.AuthenticationDigest == value.executor.request.AuthenticationDigest || decision.AuthenticationID == value.output.Binding.PriorPolicyAuthenticationID || decision.AuthenticationDigest == value.output.Binding.PriorPolicyAuthenticationDigest {
				t.Fatal("delivery policy reused prior authentication")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExactBindings(t, value, decision, request)
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyNoActivity(t, decision, request)
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyOnlyOutputsChanged(t, value, before, true)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRejectedProducesNoRequest(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t, "succeeded", "rejected", "")
	before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(t, value)
	if decision.Decision != "rejected" || decision.Route != "" || decision.OutputType != "" || decision.DeliveryType != "" || decision.DeliveryRequestID != "" || decision.ConsumerID != "" || decision.ConsumerContractFingerprint != "" || request != nil {
		t.Fatal("rejected decision named or emitted delivery authority")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyOnlyOutputsChanged(t, value, before, false)
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyCompatibilityConsumerAuthenticationAndNoInference(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t, "succeeded", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture)
	}{
		{"wrong route", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Route = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute
		}},
		{"terminal delivery for continuation", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.DeliveryType = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationDelivery
		}},
		{"wrong output", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.OutputType = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization
		}},
		{"wrong post-state", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Binding.PostState = "succeeded"
		}},
		{"wrong effect", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Binding.RouteSpecificEffect = "availability"
		}},
		{"wrong outcome", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Binding.TaskOutcome = "failed"
		}},
		{"wrong terminal result", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Binding.TerminalResult = "failed"
		}},
		{"wrong output record", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Binding.OutputRecordFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')
		}},
		{"wrong output receipt", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Binding.OutputExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		}},
		{"wrong output policy", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Binding.OutputPolicyRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('3')
		}},
		{"wrong transition", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Binding.TransitionRecordFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('4')
		}},
		{"wrong graph", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Binding.GraphRunID = "graph-run-wrong-001"
		}},
		{"wrong terminal task", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Binding.TerminalTaskID = "terminal-task-wrong-001"
		}},
		{"wrong selected task", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Binding.SelectedTaskID = "selected-task-wrong-001"
		}},
		{"wrong candidate set", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Binding.CandidatesFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('5')
		}},
		{"wrong accepted result", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Binding.AcceptedResultFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('6')
		}},
		{"wrong reconciliation", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Binding.ReconciliationReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('7')
		}},
		{"wrong authentication identity", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.AuthenticationID = "delivery-authentication-wrong-001"
		}},
		{"wrong authentication digest", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.AuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('8')
		}},
		{"wrong consumer", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.ConsumerID = "consumer-wrong-001"
		}},
		{"wrong consumer contract", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.ConsumerContractFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('9')
		}},
		{"consumed decision", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.DecisionConsumed = true
		}},
		{"replayed decision", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.OneTimeDecision = false
		}},
		{"inferred decision", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.ApprovalInferred = true
		}},
		{"unauthenticated decision", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.AuthenticationDigest = ""
		}},
		{"non fixture owned", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Provenance = "provider"
		}},
		{"authority escalation", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Authority.Acknowledgement = true
		}},
		{"ambiguous authority", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.Authority.SuccessfulTerminalGraphResultDeliveryAttempt = true
		}},
		{"colliding decision", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.DecisionID = v.Binding.OutputExecutorReceiptID
		}},
		{"colliding replay", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) {
			v.ReplayIdentity = v.ConsumerID
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture(value.decision)
			test.mutate(&changed)
			if _, _, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(value.expected, value.output, value.receipt, changed); err == nil {
				t.Fatal("incompatible, ambiguous, unauthenticated, consumed, or escalated delivery decision was accepted")
			}
		})
	}

	for _, source := range []string{"output", "result", "graph", "transition", "scheduling", "availability", "connection", "lease", "provider", "broker", "forgepipe", "ranking", "cost", "risk", "validation", "receipt"} {
		t.Run("inference "+source, func(t *testing.T) {
			changed := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture(value.decision)
			changed.ApprovalInferred, changed.RouteInferred, changed.OutputInferred, changed.DeliveryTypeInferred, changed.ConsumerInferred, changed.AuthorityInferred, changed.InferenceSource = true, true, true, true, true, true, source
			if _, _, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(value.expected, value.output, value.receipt, changed); err == nil {
				t.Fatal("adjacent evidence inferred approval, route, output, delivery, consumer, or authority")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicySeparatesTerminalRoutes(t *testing.T) {
	t.Parallel()
	success := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t, "succeeded", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute)
	changed := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture(success.decision)
	changed.DeliveryType = NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationDelivery
	changed.Authority = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{FailedTerminalGraphResultDeliveryAttempt: true}
	if _, _, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(success.expected, success.output, success.receipt, changed); err == nil {
		t.Fatal("successful terminal output authorized failed terminal delivery")
	}
	failure := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t, "failed", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute)
	changed = cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture(failure.decision)
	changed.DeliveryType = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationDelivery
	changed.Authority = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyAuthority{SuccessfulTerminalGraphResultDeliveryAttempt: true}
	if _, _, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(failure.expected, failure.output, failure.receipt, changed); err == nil {
		t.Fatal("failed terminal output authorized successful terminal delivery")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRejectsEveryPriorAuthentication(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t, "succeeded", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	reconciliation := value.expected.Executor.Policy.Executor.Policy.Reconciliation
	for _, test := range []struct {
		name, id, digest string
	}{
		{"output policy", value.executor.request.AuthenticationID, value.executor.request.AuthenticationDigest},
		{"continuation policy", value.output.Binding.PriorPolicyAuthenticationID, value.output.Binding.PriorPolicyAuthenticationDigest},
		{"result observation", reconciliation.AuthenticationID, reconciliation.AuthenticationDigest},
		{"launch authorization", reconciliation.Executor.Policy.DecisionAuthenticationID, reconciliation.Executor.Policy.DecisionAuthenticationDigest},
		{"scheduling policy", reconciliation.Executor.Policy.Executor.Policy.DecisionAuthenticationID, reconciliation.Executor.Policy.Executor.Policy.DecisionAuthenticationDigest},
		{"dependency transition policy", reconciliation.Executor.Policy.Executor.Policy.Executor.Policy.DecisionAuthenticationID, reconciliation.Executor.Policy.Executor.Policy.Executor.Policy.DecisionAuthenticationDigest},
	} {
		t.Run(test.name+" identity", func(t *testing.T) {
			changed := value.expected
			changed.DecisionAuthenticationID = test.id
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(value.root, changed); err == nil {
				t.Fatal("delivery policy reused a prior authentication identity")
			}
		})
		t.Run(test.name+" digest", func(t *testing.T) {
			changed := value.expected
			changed.DecisionAuthenticationDigest = test.digest
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(value.root, changed); err == nil {
				t.Fatal("delivery policy reused a prior authentication digest")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRevalidatesCompleteChainAndDurableOutput(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t, "succeeded", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	raw, _ := json.Marshal(value.expected)
	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected)
	}{
		{"output record", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) {
			e.OutputRecordFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')
		}},
		{"output receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) {
			e.OutputExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		}},
		{"output policy decision", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) {
			e.Executor.PolicyDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('3')
		}},
		{"output policy request", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) {
			e.Executor.PolicyRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('4')
		}},
		{"transition receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) {
			e.Executor.Policy.ExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('5')
		}},
		{"transition record", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) {
			e.Executor.Policy.TransitionRecordFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('6')
		}},
		{"continuation policy", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) {
			e.Executor.Policy.Executor.PolicyDecisionFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('7')
		}},
		{"reconciliation receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) {
			e.Executor.Policy.Executor.Policy.ReconciliationReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('8')
		}},
		{"accepted result", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) {
			e.Executor.Policy.Executor.Policy.AcceptedResultFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('9')
		}},
		{"observation authentication", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) {
			e.Executor.Policy.Executor.Policy.Reconciliation.AuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('0')
		}},
		{"launch receipt", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) {
			e.Executor.Policy.Executor.Policy.Reconciliation.ExecutorReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('b')
		}},
		{"launch authorization", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) {
			e.Executor.Policy.Executor.Policy.Reconciliation.Executor.AuthorizationRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('c')
		}},
		{"scheduling", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) {
			e.Executor.Policy.Executor.Policy.Reconciliation.Executor.Policy.SchedulingReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('d')
		}},
		{"dependency transition", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) {
			e.Executor.Policy.Executor.Policy.Reconciliation.Executor.Policy.Executor.Policy.TransitionReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('e')
		}},
		{"lifecycle", func(e *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected) {
			e.Executor.Policy.Executor.Policy.Reconciliation.Executor.Policy.Executor.Policy.Executor.Policy.AuditReceiptFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('f')
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var changed NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected
			_ = json.Unmarshal(raw, &changed)
			test.mutate(&changed)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(value.root, changed); err == nil {
				t.Fatal("changed immutable predecessor chain was accepted")
			}
		})
	}

	for _, test := range []struct {
		name   string
		mutate func(*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture)
	}{
		{"missing output record", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture) {
			_ = os.Remove(v.executor.outputPath)
		}},
		{"missing output receipt", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture) {
			_ = os.Remove(v.executor.receiptPath)
		}},
		{"wrong action count", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture) {
			v.receipt.OutputActionCount = 2
			v.receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptFingerprint(v.receipt)
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, v.executor.receiptPath, v.receipt)
		}},
		{"wrong write count", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture) {
			v.receipt.OutputRecordWriteCount = 2
			v.receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptFingerprint(v.receipt)
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, v.executor.receiptPath, v.receipt)
		}},
		{"unconsumed output policy", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture) {
			v.receipt.AuthorizationConsumed = false
			v.receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptFingerprint(v.receipt)
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, v.executor.receiptPath, v.receipt)
		}},
		{"wrong version", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture) {
			v.output.Version = 2
			v.output.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordFingerprint(v.output)
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, v.executor.outputPath, v.output)
		}},
		{"non fixture output", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture) {
			v.output.FixtureOwned = false
			v.output.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordFingerprint(v.output)
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, v.executor.outputPath, v.output)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t, value)
			test.mutate(changed)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(changed.root, changed.expected); err == nil {
				t.Fatal("missing, orphaned, wrong-count, wrong-version, or non-fixture output evidence was accepted")
			}
		})
	}

	opened := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(t, value)
	changedOutput := value.output
	changedOutput.Binding.GraphRunID = "graph-run-tampered-001"
	changedOutput.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordFingerprint(changedOutput)
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, value.executor.outputPath, changedOutput)
	if _, _, err := opened.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(t, value.decision)); err == nil {
		t.Fatal("decision-time revalidation accepted changed durable output")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyReplayRestartConcurrencyRecoveryAndConflicts(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t, "succeeded", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	raw := mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(t, value.decision)
	firstDecision, firstRequest, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(t, value).Decide(raw)
	if err != nil {
		t.Fatal(err)
	}
	secondDecision, secondRequest, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(t, value).Decide(raw)
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
			policies, openErr := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(value.root, value.expected)
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
	conflict := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture(value.decision)
	conflict.DecisionID, conflict.ReplayIdentity = "delivery-decision-002", "delivery-replay-002"
	if _, _, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(t, value).Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(t, conflict)); err == nil {
		t.Fatal("conflicting decision concurrency was accepted")
	}

	recovery := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t, "failed", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute)
	policies := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(t, recovery)
	decisionWriter := policies.writeDecision
	var decisionWrites atomic.Int32
	policies.writeDecision = func(path string, payload any) error { decisionWrites.Add(1); return decisionWriter(path, payload) }
	policies.writeRequest = func(string, any) error { return errors.New("injected request failure") }
	if _, _, err := policies.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(t, recovery.decision)); err == nil || decisionWrites.Load() != 1 {
		t.Fatal("decision-before-request failure did not preserve exactly one decision")
	}
	if _, err := os.Lstat(recovery.requestPath); !os.IsNotExist(err) {
		t.Fatal("request failure left partial output")
	}
	restarted := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(t, recovery)
	restarted.writeDecision = func(path string, payload any) error { decisionWrites.Add(1); return decisionWriter(path, payload) }
	decision, request, err := restarted.Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(t, recovery.decision))
	if err != nil || request == nil || decisionWrites.Load() != 1 || decision.DecisionFingerprint != request.DecisionFingerprint {
		t.Fatal("restart did not recover only the exact missing request")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRejectsMalformedUnsafePartialAndOrphanedArtifacts(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t, "succeeded", "approved", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	policies := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(t, value)
	canonical := mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(t, value.decision)
	for _, test := range []struct {
		name string
		raw  []byte
	}{
		{"empty", nil},
		{"malformed", []byte("{")},
		{"trailing", append(append([]byte{}, canonical...), []byte("{}")...)},
		{"unknown field", append(append([]byte{}, canonical[:len(canonical)-1]...), []byte(`,"unknown":true}`)...)},
		{"noncanonical", mustMarshalIndentedNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(t, value.decision)},
		{"oversized", bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryDecisionMax+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := policies.Decide(test.raw); err == nil {
				t.Fatal("malformed, noncanonical, unknown-field, trailing, or oversized fixture was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyArtifactsAbsent(t, value)
		})
	}

	for _, test := range []struct {
		name string
		raw  []byte
	}{
		{"partial", []byte("{")},
		{"unknown", []byte("{\"unknown\":true}\n")},
		{"trailing", []byte("{}\n{}\n")},
		{"noncanonical", []byte("{}")},
		{"oversized", bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryArtifactMax+1)},
	} {
		t.Run("existing "+test.name, func(t *testing.T) {
			changed := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t, value)
			if err := os.WriteFile(changed.decisionPath, test.raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(changed.root, changed.expected); err == nil {
				t.Fatal("partial, unknown, trailing, noncanonical, or oversized artifact was accepted")
			}
		})
	}

	t.Run("symlinked decision", func(t *testing.T) {
		changed := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t, value)
		target := changed.decisionPath + ".target"
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, target, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{})
		if err := os.Symlink(target, changed.decisionPath); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(changed.root, changed.expected); err == nil {
			t.Fatal("symlinked policy artifact was accepted")
		}
	})

	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyCanonicalArtifact(value.root, filepath.Join(value.root, "..", "unsafe-delivery.json"), &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision{}, true); err == nil {
		t.Fatal("unsafe output path was accepted")
	}

	orphan := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t, value)
	decision, orphanRequest, err := deriveNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(orphan.expected, orphan.output, orphan.receipt, orphan.decision)
	if err != nil || orphanRequest == nil {
		t.Fatal(err)
	}
	mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, orphan.requestPath, *orphanRequest)
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(orphan.root, orphan.expected); err == nil {
		t.Fatal("request without its exact decision was accepted")
	}

	escalated := *orphanRequest
	escalated.AuthorizationConsumed, escalated.DeliveryPerformed, escalated.ConsumerInvoked, escalated.AcknowledgementReceived, escalated.LifecycleActionTriggered, escalated.PublicationPerformed = true, true, true, true, true, true
	escalated.Authority.Acknowledgement = true
	escalated.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequestFingerprint(escalated)
	if validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest(escalated, value.expected, value.output, value.receipt, decision) == nil {
		t.Fatal("consumed, performed, acknowledged, lifecycle-escalated, or published request was accepted")
	}
}

func newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t *testing.T, terminalResult, decision, route string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture {
	t.Helper()
	executorRoute := route
	if executorRoute == "" {
		executorRoute = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute
		if terminalResult == "failed" {
			executorRoute = NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute
		}
	}
	executor := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture(t, terminalResult, executorRoute)
	receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutor(t, executor)
	output := mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecord(t, executor)
	expected := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExpected{Executor: executor.expected, OutputRecordFingerprint: output.RecordFingerprint, OutputExecutorReceiptFingerprint: receipt.ReceiptFingerprint, DecisionAuthenticationID: "delivery-authentication-001", DecisionAuthenticationDigest: "sha256:fc891d002159c849bd6ce4a2857fa99d5203d2751f77ec5ad3a9a732e2c78344", DeliveryRequestID: "delivery-request-001", ConsumerID: "downstream-consumer-001", ConsumerContractFingerprint: "sha256:eea6e6c19dbd0091a4060dc49d9e9940f2c14f761aa751b4289b055da0483567"}
	deliveryType, authority, _ := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRouteAuthority(output, receipt)
	fixture := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture{Schema: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixtureSchema, DecisionID: "delivery-decision-001", ReplayIdentity: "delivery-replay-001", AuthenticationID: expected.DecisionAuthenticationID, AuthenticationDigest: expected.DecisionAuthenticationDigest, Decision: decision, Binding: nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyBinding(output, receipt), Deterministic: true, OneTimeDecision: true, Provenance: "fixture_only_graph_output_delivery_policy_decision"}
	if decision == "approved" {
		fixture.Route, fixture.OutputType, fixture.DeliveryType, fixture.DeliveryRequestID, fixture.ConsumerID, fixture.ConsumerContractFingerprint, fixture.Authority = output.Binding.Route, output.Binding.OutputType, deliveryType, expected.DeliveryRequestID, expected.ConsumerID, expected.ConsumerContractFingerprint, authority
	}
	return &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture{root: executor.root, executor: executor, output: output, receipt: receipt, expected: expected, decision: fixture, decisionPath: filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryDecisionName), requestPath: filepath.Join(executor.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryRequestName)}
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture {
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
	executor := &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorTestFixture{root: root, expected: value.executor.expected, outputPath: filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordName), receiptPath: filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputExecutorReceiptName)}
	return &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture{root: root, executor: executor, output: value.output, receipt: value.receipt, expected: value.expected, decision: value.decision, decisionPath: filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryDecisionName), requestPath: filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryRequestName)}
}

func mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture) *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies {
	t.Helper()
	policies, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return policies
}

func mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture) (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision, *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest) {
	t.Helper()
	decision, request, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicies(t, value).Decide(mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(t, value.decision))
	if err != nil {
		t.Fatal(err)
	}
	return decision, request
}

func mustMarshalNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(t *testing.T, value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustMarshalIndentedNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicy(t *testing.T, value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) []byte {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture(value NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture {
	raw, _ := json.Marshal(value)
	var cloned NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecisionFixture
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyExactBindings(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture, decision NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision, request *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest) {
	t.Helper()
	want := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyBinding(value.output, value.receipt)
	if !reflect.DeepEqual(decision.Binding, want) || request == nil || !reflect.DeepEqual(request.Binding, want) || request.DecisionID != decision.DecisionID || request.DecisionReplayIdentity != decision.ReplayIdentity || request.DecisionFingerprint != decision.DecisionFingerprint || request.AuthenticationID != decision.AuthenticationID || request.AuthenticationDigest != decision.AuthenticationDigest || request.ConsumerID != value.expected.ConsumerID || request.ConsumerContractFingerprint != value.expected.ConsumerContractFingerprint {
		t.Fatal("decision or request omitted an exact output, executor, policy, transition, graph, task, candidate, result, reconciliation, authentication, consumer, or contract binding")
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyNoActivity(t *testing.T, decision NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision, request *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest) {
	t.Helper()
	raw, err := json.Marshal(struct {
		Decision NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyDecision
		Request  *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyRequest
	}{decision, request})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"delivery":true`, `"consumption":true`, `"acknowledgement":true`, `"receiver_invocation":true`, `"lifecycle_advancement":true`, `"graph_mutation":true`, `"dependency_release":true`, `"failure_propagation":true`, `"scheduling":true`, `"execution":true`, `"retry":true`, `"repair":true`, `"cancellation":true`, `"callback":true`, `"publication":true`, `"provider":true`, `"connector":true`, `"broker":true`, `"forgepipe":true`, `"network":true`, `"remote_execution":true`, `"validation":true`, `"checkout_mutation":true`, `"git":true`, `"checkpoint":true`, `"commit":true`, `"push":true`, `"external_action":true`, `"delivery_performed":true`, `"consumer_invoked":true`, `"acknowledgement_received":true`, `"callback_invoked":true`, `"lifecycle_action_triggered":true`, `"publication_performed":true`, `"external_action_performed":true`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("delivery policy performed or granted forbidden activity: %s", forbidden)
		}
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyOnlyOutputsChanged(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture, before map[string][]byte, requestExpected bool) {
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
	want := []string{nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryDecisionName}
	if requestExpected {
		want = append(want, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryRequestName)
	}
	sort.Strings(want)
	if !reflect.DeepEqual(changed, want) {
		t.Fatalf("delivery policy changed a predecessor or adjacent artifact: got %v want %v", changed, want)
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyArtifactsAbsent(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryPolicyTestFixture) {
	t.Helper()
	for _, path := range []string{value.decisionPath, value.requestPath} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("failed delivery policy unexpectedly published %s", path)
		}
	}
}

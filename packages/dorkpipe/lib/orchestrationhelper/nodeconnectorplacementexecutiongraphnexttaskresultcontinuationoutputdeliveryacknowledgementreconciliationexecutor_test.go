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

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture struct {
	root        string
	policy      *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture
	decision    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision
	request     NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest
	expected    NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorExpected
	recordPath  string
	receiptPath string
}

type nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestTemplate struct {
	once    sync.Once
	fixture nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture
	policy  nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture
	files   map[string][]byte
}

var nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestTemplates = map[string]*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestTemplate{
	"succeeded\x00" + NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute:           {},
	"succeeded\x00" + NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute: {},
	"failed\x00" + NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute:        {},
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorExactRoutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, result, route, postState, outputType, deliveryType, outcome string
		consumed                                                          NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority
	}{
		{"continuation handoff acknowledgement", "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute, "continued", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffOutput, NodeConnectorPlacementExecutionGraphNextTaskResultContinuationHandoffDelivery, "passed", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{ContinuationHandoffAcknowledgementReconciliationAttempt: true}},
		{"successful terminal acknowledgement", "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationRoute, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization, NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationDelivery, "passed", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{SuccessfulTerminalGraphResultAcknowledgementReconciliationAttempt: true}},
		{"failed terminal acknowledgement", "failed", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationRoute, "failed", NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationMaterialization, NodeConnectorPlacementExecutionGraphNextTaskResultFailedFinalizationDelivery, "failed", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{FailedTerminalGraphResultAcknowledgementReconciliationAttempt: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture(t, test.result, test.route)
			before := mustSnapshotNodeConnectorPlacementExecutionGraphLifecycleExecutorRoot(t, value.root)
			requestBefore := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.requestPath)
			acknowledgementBefore := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementName))
			deliveryReceiptBefore := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptName))

			receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(t, value)
			record := mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord(t, value)
			if record.Version != 1 || record.AcknowledgementReconciliationCount != 1 || !record.FixtureOwned || record.Binding.Route != test.route || record.Binding.PostState != test.postState || record.Binding.OutputType != test.outputType || record.Binding.DeliveryType != test.deliveryType || record.Binding.TaskOutcome != test.outcome {
				t.Fatal("executor did not materialize the exact route-compatible reconciliation record")
			}
			if receipt.ReconciliationRecordID != record.ReconciliationRecordID || receipt.ReconciliationRecordFingerprint != record.RecordFingerprint || receipt.ReconciliationRecordVersion != record.Version || receipt.Route != test.route || receipt.ExactPostState != test.postState || receipt.OutputType != test.outputType || receipt.DeliveryType != test.deliveryType || receipt.ConsumedAuthority != test.consumed {
				t.Fatal("receipt did not bind the exact record or mutually exclusive consumed route authority")
			}
			if receipt.LogicalReconciliationAttemptCount != 1 || receipt.ReconciliationRecordWriteCount != 1 || receipt.ExecutorReceiptWriteCount != 1 || !receipt.AuthorizationConsumed || !receipt.CompleteImmutablePredecessorChainRevalidated || !receipt.NoConsumerReinvocation || !receipt.NoDuplicateReconciliation || !receipt.FixtureOwned {
				t.Fatal("receipt did not prove one durable reconciliation without duplicate or consumer reinvocation")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorExactBinding(t, value, record, receipt)
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorNoAuthority(t, record, receipt)
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorOnlyOutputsChanged(t, value, before)
			if !bytes.Equal(requestBefore, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.policy.requestPath)) || !bytes.Equal(acknowledgementBefore, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementName))) || !bytes.Equal(deliveryReceiptBefore, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, filepath.Join(value.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptName))) {
				t.Fatal("executor mutated the immutable request, acknowledgement, delivery receipt, or predecessor chain")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorRequiresExactIndependentAuthority(t *testing.T) {
	t.Parallel()
	base := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	originalRequest := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, base.policy.requestPath)

	for _, test := range []struct {
		name   string
		mutate func(*NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest)
	}{
		{"consumed request", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest) {
			v.AuthorizationConsumed = true
		}},
		{"replayed request", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest) {
			v.AcknowledgementReconciled = true
		}},
		{"unauthenticated request", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest) {
			v.AuthenticationDigest = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('1')
		}},
		{"non fixture request", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest) {
			v.FixtureOwned = false
		}},
		{"mixed authority", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest) {
			v.Authority.SuccessfulTerminalGraphResultAcknowledgementReconciliationAttempt = true
		}},
		{"authority escalation", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest) {
			v.Authority.Publication = true
		}},
		{"wrong acknowledgement", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest) {
			v.AcknowledgementID = "acknowledgement-wrong-001"
		}},
		{"wrong operation key", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest) {
			v.OperationKey.RequestID = "delivery-request-wrong-001"
		}},
		{"wrong consumer", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest) {
			v.ConsumerID = "consumer-wrong-001"
		}},
		{"wrong contract", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest) {
			v.ConsumerContractFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('2')
		}},
		{"terminal authority on continuation", func(v *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequest) {
			v.Authority = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{SuccessfulTerminalGraphResultAcknowledgementReconciliationAttempt: true}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if err := os.WriteFile(base.policy.requestPath, originalRequest, 0o600); err != nil {
					t.Errorf("restore acknowledgement-reconciliation request: %v", err)
				}
			}()
			request := base.request
			test.mutate(&request)
			request.RequestFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyRequestFingerprint(request)
			expected := base.expected
			expected.PolicyRequestFingerprint = request.RequestFingerprint
			mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, base.policy.requestPath, request)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(base.root, expected); err == nil {
				t.Fatal("consumed, replayed, unauthenticated, non-fixture-owned, misbound, mixed, or authority-escalated request was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorArtifactsAbsent(t, base)
		})
	}

	for _, source := range []string{"acknowledgement_presence", "delivery_receipt_presence", "consumer_acceptance", "delivery_policy_approval", "authorization_consumption", "output", "result", "graph", "transition", "route", "state", "terminal", "scheduling", "candidate", "task", "dependency", "lifecycle", "attempt", "execution", "availability", "connection", "health", "load", "lease", "placement", "ranking", "cost", "risk", "recommendation", "validation", "provider", "connector", "broker", "forgepipe", "machine", "capability", "network"} {
		t.Run("no inference from "+source, func(t *testing.T) {
			decision := base.decision
			decision.ApprovalInferred, decision.RouteInferred, decision.AcknowledgementInferred, decision.ConsumerInferred, decision.ReconciliationInferred, decision.AuthorityInferred, decision.InferenceSource = true, true, true, true, true, true, source
			decision.DecisionFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecisionFingerprint(decision)
			if validateNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyDecision(decision, base.policy.expected, base.policy.acknowledgement, base.policy.receipt) == nil {
				t.Fatal("adjacent evidence inferred acknowledgement reconciliation authority")
			}
		})
	}

	rejectedPolicy := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture(t, base.policy)
	if err := os.Remove(rejectedPolicy.decisionPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(rejectedPolicy.requestPath); err != nil {
		t.Fatal(err)
	}
	rejectedPolicy.fixture.Decision = "rejected"
	rejectedPolicy.fixture.Route = ""
	rejectedPolicy.fixture.PostState = ""
	rejectedPolicy.fixture.RouteSpecificEffect = ""
	rejectedPolicy.fixture.OutputType = ""
	rejectedPolicy.fixture.DeliveryType = ""
	rejectedPolicy.fixture.TerminalResult = ""
	rejectedPolicy.fixture.TaskOutcome = ""
	rejectedPolicy.fixture.ReconciliationRequestID = ""
	rejectedPolicy.fixture.ConsumerID = ""
	rejectedPolicy.fixture.ConsumerContractFingerprint = ""
	rejectedPolicy.fixture.Authority = NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyAuthority{}
	decision, request := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t, rejectedPolicy)
	if request != nil {
		t.Fatal("rejected policy emitted a reconciliation request")
	}
	expected := NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorExpected{Policy: rejectedPolicy.expected, PolicyDecisionFingerprint: decision.DecisionFingerprint}
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(rejectedPolicy.root, expected); err == nil {
		t.Fatal("rejected policy decision was accepted")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorRevalidatesAcknowledgementReceiptAndCompleteChain(t *testing.T) {
	t.Parallel()
	base := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	acknowledgementPath := filepath.Join(base.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementName)
	deliveryReceiptPath := filepath.Join(base.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptName)
	originalAcknowledgement := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, acknowledgementPath)
	originalDeliveryReceipt := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, deliveryReceiptPath)
	for _, test := range []struct {
		name   string
		mutate func(*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture)
	}{
		{"missing acknowledgement", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) {
			_ = os.Remove(filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementName))
		}},
		{"missing delivery receipt", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) {
			_ = os.Remove(filepath.Join(v.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptName))
		}},
		{"unaccepted acknowledgement", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) {
			v.policy.acknowledgement.Accepted = false
			rewriteAcknowledgementReconciliationPolicyAcknowledgement(t, v.policy)
		}},
		{"acknowledgement authority", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) {
			v.policy.acknowledgement.Authority.LifecycleAdvancement = true
			rewriteAcknowledgementReconciliationPolicyAcknowledgement(t, v.policy)
		}},
		{"wrong delivery attempt", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) {
			v.policy.receipt.LogicalDeliveryAttemptCount = 2
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v.policy)
		}},
		{"wrong consumer invocation", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) {
			v.policy.receipt.ConsumerInvocationCount = 0
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v.policy)
		}},
		{"wrong acknowledgement count", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) {
			v.policy.receipt.AcceptedAcknowledgementCount = 0
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v.policy)
		}},
		{"wrong acknowledgement write", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) {
			v.policy.receipt.AcknowledgementArtifactWriteCount = 0
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v.policy)
		}},
		{"wrong receipt write", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) {
			v.policy.receipt.ExecutorReceiptWriteCount = 0
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v.policy)
		}},
		{"delivery authorization not consumed", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) {
			v.policy.receipt.AuthorizationConsumed = false
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v.policy)
		}},
		{"chain not revalidated", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) {
			v.policy.receipt.CompleteImmutablePredecessorChainRevalidated = false
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v.policy)
		}},
		{"duplicate delivery", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) {
			v.policy.receipt.NoDuplicateDelivery = false
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v.policy)
		}},
		{"consumer reinvoked", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) {
			v.policy.receipt.ConsumerReinvoked = true
			rewriteAcknowledgementReconciliationPolicyReceipt(t, v.policy)
		}},
		{"changed predecessor chain", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) {
			v.expected.Policy.Executor.PolicyRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('3')
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				for path, raw := range map[string][]byte{acknowledgementPath: originalAcknowledgement, deliveryReceiptPath: originalDeliveryReceipt} {
					if err := os.WriteFile(path, raw, 0o600); err != nil {
						t.Errorf("restore immutable acknowledgement-reconciliation input %s: %v", filepath.Base(path), err)
					}
				}
			}()
			value := *base
			policy := *base.policy
			value.policy = &policy
			test.mutate(&value)
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(value.root, value.expected); err == nil {
				t.Fatal("missing, changed, unauthoritative, or incomplete acknowledgement, receipt, consumer, or predecessor chain was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorArtifactsAbsent(t, &value)
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorRouteCompatibility(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs)
	}{
		{"empty route", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) {
			v.request.Route = ""
		}},
		{"unknown route", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) {
			v.request.Route = "unknown_route"
		}},
		{"terminal acknowledgement on continuation", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) {
			v.request.DeliveryType = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationDelivery
		}},
		{"terminal output on continuation", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) {
			v.request.OutputType = NodeConnectorPlacementExecutionGraphNextTaskResultSuccessfulFinalizationMaterialization
		}},
		{"wrong state", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) {
			v.request.PostState = "succeeded"
		}},
		{"wrong effect", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) {
			v.request.RouteSpecificEffect = "receipt_presence"
		}},
		{"wrong outcome", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) {
			v.request.TaskOutcome = "failed"
		}},
		{"wrong result", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) {
			v.request.TerminalResult = "failed"
		}},
		{"ambiguous authority", func(v *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) {
			v.request.Authority.SuccessfulTerminalGraphResultAcknowledgementReconciliationAttempt = true
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs(inputs)
			test.mutate(&changed)
			if _, ok := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationConsumedAuthorityFor(changed.request, changed.acknowledgement, changed.deliveryReceipt); ok {
				t.Fatal("empty, unknown, mixed, state-, effect-, output-, delivery-, outcome-, result-, or authority-incompatible route was accepted")
			}
		})
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReplayConcurrencyAndRecovery(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	concurrent := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture(t, value)
	recovery := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture(t, value)
	first := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(t, value)
	firstRecord := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)
	firstReceipt := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.receiptPath)
	second := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(t, value)
	if !reflect.DeepEqual(first, second) || !bytes.Equal(firstRecord, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.recordPath)) || !bytes.Equal(firstReceipt, mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, value.receiptPath)) {
		t.Fatal("exact replay, restart, or identical existing artifacts changed reconciliation evidence")
	}

	const callers = 6
	var group sync.WaitGroup
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			executor, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(concurrent.root, concurrent.expected)
			if err == nil {
				_, err = executor.Execute()
			}
			errs <- err
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal("identical concurrency did not converge", err)
		}
	}

	conflicting := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture(t, concurrent)
	conflicting.expected.PolicyRequestFingerprint = testNodeConnectorPlacementExecutionGraphNextTaskLaunchExecutionAuthorizationFingerprint('4')
	if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(conflicting.root, conflicting.expected); err == nil {
		t.Fatal("conflicting concurrent expectation was accepted")
	}

	executor := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(t, recovery)
	var recordWrites atomic.Int32
	executor.writeRecordAtomic = func(path string, artifact any) error { recordWrites.Add(1); return writeJSONFileAtomic(path, artifact) }
	executor.writeReceiptAtomic = func(string, any) error { return errors.New("injected receipt failure") }
	if _, err := executor.Execute(); err == nil || recordWrites.Load() != 1 {
		t.Fatal("record-before-receipt failure did not preserve exactly one record")
	}
	assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorArtifactAbsent(t, recovery.receiptPath)
	recovered := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(t, recovery)
	if recordWrites.Load() != 1 || recovered.ReconciliationRecordWriteCount != 1 || recovered.ExecutorReceiptWriteCount != 1 {
		t.Fatal("recovery repeated reconciliation instead of publishing only the missing receipt")
	}
}

func TestNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorRejectsOrphansTamperingAndUnsafeState(t *testing.T) {
	t.Parallel()
	value := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture(t, "succeeded", NodeConnectorPlacementExecutionGraphNextTaskResultContinuationRoute)
	completed := cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture(t, value)
	_ = mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(t, completed)
	originalRecord := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, completed.recordPath)
	originalReceipt := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, completed.receiptPath)
	restore := func(t *testing.T, path string, raw []byte) {
		t.Helper()
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Errorf("restore acknowledgement-reconciliation artifact %s: %v", filepath.Base(path), err)
		}
	}

	t.Run("receipt without record", func(t *testing.T) {
		defer restore(t, completed.recordPath, originalRecord)
		if err := os.Remove(completed.recordPath); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(completed.root, completed.expected); err == nil {
			t.Fatal("receipt without its exact reconciliation record was accepted")
		}
	})

	for _, test := range []struct {
		name string
		raw  []byte
	}{
		{"empty", nil}, {"malformed", []byte("{")}, {"unknown field", []byte("{\"unknown\":true}\n")}, {"trailing", []byte("{}\n{}\n")}, {"noncanonical", []byte("{}")}, {"partial", []byte("{\"schema\":\"dorkpipe.node-placement-execution-graph-next-task-result-continuation-output-delivery-acknowledgement-reconciliation-record/v1\"}\n")}, {"oversized", bytes.Repeat([]byte("x"), nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorMaxBytes+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if err := os.Remove(value.recordPath); err != nil && !os.IsNotExist(err) {
					t.Errorf("remove malformed acknowledgement-reconciliation record: %v", err)
				}
			}()
			if err := os.WriteFile(value.recordPath, test.raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(value.root, value.expected); err == nil {
				t.Fatal("malformed, noncanonical, unknown-field, trailing, partial, empty, or oversized record was accepted")
			}
			assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorArtifactAbsent(t, value.receiptPath)
		})
	}

	t.Run("tampered record", func(t *testing.T) {
		defer restore(t, completed.recordPath, originalRecord)
		record := mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord(t, completed)
		record.Binding.ConsumerID = "consumer-tampered-001"
		record.RecordFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecordFingerprint(record)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, completed.recordPath, record)
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(completed.root, completed.expected); err == nil {
			t.Fatal("tampered record was accepted")
		}
	})

	t.Run("tampered receipt", func(t *testing.T) {
		defer restore(t, completed.receiptPath, originalReceipt)
		receipt := mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(t, completed)
		receipt.LogicalReconciliationAttemptCount = 2
		receipt.ReceiptFingerprint, _ = nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceiptFingerprint(receipt)
		mustWriteNodeConnectorPlacementExecutionGraphNextTaskSchedulingExecutorArtifact(t, completed.receiptPath, receipt)
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(completed.root, completed.expected); err == nil {
			t.Fatal("tampered receipt was accepted")
		}
	})

	t.Run("record without exact predecessor", func(t *testing.T) {
		for _, predecessor := range []string{nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRequestName, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementName, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryExecutorReceiptName, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputRecordName} {
			t.Run(predecessor, func(t *testing.T) {
				path := filepath.Join(completed.root, predecessor)
				raw := mustReadNodeConnectorPlacementExecutionGraphLifecycleExecutorFile(t, path)
				defer restore(t, path, raw)
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(completed.root, completed.expected); err == nil {
					t.Fatalf("record without %s was accepted", predecessor)
				}
			})
		}
	})

	t.Run("symlinked record", func(t *testing.T) {
		target := filepath.Join(value.root, "reconciliation-target.json")
		defer func() {
			_ = os.Remove(value.recordPath)
			_ = os.Remove(target)
		}()
		if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, value.recordPath); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(value.root, value.expected); err == nil {
			t.Fatal("symlinked record was accepted")
		}
	})

	if err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorCanonicalArtifact(value.root, filepath.Join(value.root, "..", "unsafe-reconciliation.json"), &NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord{}, true); err == nil {
		t.Fatal("unsafe reconciliation path was accepted")
	}
}

func newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture(t *testing.T, terminalResult, route string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture {
	t.Helper()
	template, ok := nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestTemplates[terminalResult+"\x00"+route]
	if !ok {
		t.Fatalf("unsupported acknowledgement-reconciliation executor test route %q/%q", terminalResult, route)
	}
	template.once.Do(func() {
		value := buildNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture(t, terminalResult, route)
		template.policy = *value.policy
		template.fixture = *value
		template.fixture.policy = &template.policy
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
	policy := template.policy
	policy.root = root
	policy.decisionPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationDecisionName)
	policy.requestPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRequestName)
	value := template.fixture
	value.root, value.policy = root, &policy
	value.recordPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecordName)
	value.receiptPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceiptName)
	return &value
}

func buildNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture(t *testing.T, terminalResult, route string) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture {
	t.Helper()
	policy := newNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicyTestFixture(t, terminalResult, route, "approved")
	decision, requestPointer := mustDecideNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationPolicy(t, policy)
	if requestPointer == nil {
		t.Fatal("approved acknowledgement reconciliation policy did not emit a request")
	}
	request := *requestPointer
	return &nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture{root: policy.root, policy: policy, decision: decision, request: request, expected: NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorExpected{Policy: policy.expected, PolicyDecisionFingerprint: decision.DecisionFingerprint, PolicyRequestFingerprint: request.RequestFingerprint}, recordPath: filepath.Join(policy.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecordName), receiptPath: filepath.Join(policy.root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceiptName)}
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture {
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
	policy := *value.policy
	policy.root = root
	policy.decisionPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationDecisionName)
	policy.requestPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRequestName)
	cloned := *value
	cloned.root, cloned.policy = root, &policy
	cloned.recordPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecordName)
	cloned.receiptPath = filepath.Join(root, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceiptName)
	return &cloned
}

func cloneNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs(value nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs) nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs {
	raw, _ := json.Marshal(value)
	var cloned nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs
	_ = json.Unmarshal(raw, &cloned)
	cloned = value
	return cloned
}

func mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) *NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor {
	t.Helper()
	executor, err := OpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	return executor
}

func mustExecuteNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt {
	t.Helper()
	receipt, err := mustOpenNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutor(t, value).Execute()
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func mustLoadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord {
	t.Helper()
	inputs, err := loadNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorInputs(value.root, value.expected)
	if err != nil {
		t.Fatal(err)
	}
	if !inputs.recordExists {
		t.Fatal("reconciliation record missing")
	}
	return inputs.record
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorExactBinding(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture, record NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt) {
	t.Helper()
	if !reflect.DeepEqual(record.Binding, receipt.Binding) {
		t.Fatal("record and receipt bindings differ")
	}
	b := record.Binding
	pb := value.request.Binding
	db := pb.DeliveryExecutorBinding
	if b.ReconciliationPolicyDecisionID != value.decision.DecisionID || b.ReconciliationPolicyDecisionFingerprint != value.decision.DecisionFingerprint || b.ReconciliationPolicyRequestID != value.request.RequestID || b.ReconciliationPolicyRequestFingerprint != value.request.RequestFingerprint || b.DecisionReplayIdentity != value.request.DecisionReplayIdentity || b.DecisionAuthenticationID != value.request.AuthenticationID || b.DecisionAuthenticationDigest != value.request.AuthenticationDigest || b.AcknowledgementID != value.policy.acknowledgement.AcknowledgementID || b.AcknowledgementFingerprint != value.policy.acknowledgement.AcknowledgementFingerprint || b.OperationKey != value.policy.acknowledgement.OperationKey || b.DeliveryExecutorReceiptID != value.policy.receipt.ExecutorReceiptID || b.DeliveryExecutorReceiptFingerprint != value.policy.receipt.ReceiptFingerprint || b.Route != pb.Route || b.PostState != pb.PostState || b.RouteSpecificEffect != pb.RouteSpecificEffect || b.OutputType != pb.OutputType || b.DeliveryType != pb.DeliveryType || b.ConsumerID != pb.ConsumerID || b.ConsumerContractFingerprint != pb.ConsumerContractFingerprint || b.GraphRunID != db.GraphRunID || b.TerminalTaskID != db.TerminalTaskID || b.SelectedTaskID != db.SelectedTaskID || b.CandidatesFingerprint != db.CandidatesFingerprint || b.AcceptedResultID != db.AcceptedResultID || b.AcceptedResultFingerprint != db.AcceptedResultFingerprint || b.PriorReconciliationReceiptID != db.ReconciliationReceiptID || b.PriorReconciliationReceiptFingerprint != db.ReconciliationReceiptFingerprint || b.TransitionExecutorReceiptID != db.TransitionExecutorReceiptID || b.TransitionRecordID != db.TransitionRecordID || b.OutputPolicyDecisionID != db.OutputPolicyDecisionID || b.OutputPolicyRequestID != db.OutputPolicyRequestID || b.OutputRecordID != db.OutputRecordID || b.OutputExecutorReceiptID != db.OutputExecutorReceiptID || b.DeliveryPolicyDecisionID != db.DeliveryPolicyDecisionID || b.DeliveryPolicyRequestID != db.DeliveryPolicyRequestID || b.TerminalResult != pb.TerminalResult || b.TaskOutcome != pb.TaskOutcome || !reflect.DeepEqual(b.PolicyBinding, pb) {
		t.Fatal("reconciliation evidence omitted an exact policy, authentication, acknowledgement, receipt, consumer, route, graph, task, candidate, result, transition, output, delivery, or predecessor binding")
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorNoAuthority(t *testing.T, record NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord, receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt) {
	t.Helper()
	if record.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationAuthority{}) || receipt.Authority != (NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationAuthority{}) {
		t.Fatal("reconciliation evidence granted adjacent authority")
	}
	raw, err := json.Marshal(struct {
		Record  NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecord
		Receipt NodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceipt
	}{record, receipt})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"lifecycle_advancement":true`, `"graph_mutation":true`, `"dependency_work":true`, `"dependency_release":true`, `"failure_propagation":true`, `"candidate_discovery":true`, `"candidate_selection":true`, `"scheduling":true`, `"execution":true`, `"node_execution":true`, `"result_collection":true`, `"delivery":true`, `"redelivery":true`, `"consumer_invocation":true`, `"consumer_reinvocation":true`, `"retry":true`, `"repair":true`, `"cancellation":true`, `"queue_processing":true`, `"callback":true`, `"publication":true`, `"provider":true`, `"connector":true`, `"broker":true`, `"forgepipe":true`, `"process":true`, `"network":true`, `"remote_execution":true`, `"validation":true`, `"checkout_mutation":true`, `"git":true`, `"checkpoint":true`, `"commit":true`, `"push":true`, `"external_action":true`, `"downstream_authority":true`} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("reconciliation evidence escalated forbidden activity: %s", forbidden)
		}
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorOnlyOutputsChanged(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture, before map[string][]byte) {
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
	want := []string{nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationRecordName, nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorReceiptName}
	sort.Strings(want)
	if !reflect.DeepEqual(changed, want) {
		t.Fatalf("executor changed forbidden predecessor or adjacent state: got %v want %v", changed, want)
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorArtifactsAbsent(t *testing.T, value *nodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorTestFixture) {
	t.Helper()
	for _, path := range []string{value.recordPath, value.receiptPath} {
		assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorArtifactAbsent(t, path)
	}
}

func assertNodeConnectorPlacementExecutionGraphNextTaskResultContinuationOutputDeliveryAcknowledgementReconciliationExecutorArtifactAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("executor unexpectedly published %s", path)
	}
}
